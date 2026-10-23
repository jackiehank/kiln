# issue: empty-commit-hint — `kiln commit` 在无改动时提示空提交

- **domain**：cli
- **关联**：relates-to `cli-necessity`（收口结论记下了 jj 允许空动作）；relates-to `target-honesty`（真正的逐动作 diff 审计归它）

## 背景

`cli-necessity` 收口时实测：jj 后端在工作区为空时 `jj commit --message` 仍 **exit 0**，生成一次空提交，新 `@` 也是空的；git 后端 `git commit` 没有内容会失败，且 kiln 从不传 `--allow-empty`。也就是说，**"动作 = 一次有内容的提交"不再由 VCS 强制**——这个自由度只存在于 jj 后端。

后果是静默失败：agent 以为改了文件，实际路径写错或编辑没落盘时，jj 上动作照常记录、输出"已提交"，看起来成功。事后的材料只有 `path --stat` 和 `path --json` 的 `stat` 字段（空即空提交），但要有人主动去查。

用户裁定（2026-10-23）：提示放在 `kiln commit`，**不进 `check`**。理由：`check` 只读提交消息；空动作本身合法（`conclude` 可以只记"决定不改"）；若做成 `check` 规则，就得为每条动作扫一次 diff，且 jj/git 两边不对称。

## 问题

1. **提示时机**：放在 `backend.Action(msg)` **之前**。动作一旦落库就无法撤回，提示必须在可能产生空提交之前发出。
2. **判定依据**：`vcs.Workspace.Dirty()`——jj 实现为 `jj diff --summary` 非空（与动作是否会产生空提交完全同构）；git 实现为未暂存/未跟踪改动（偏"宽"）。不新增接口方法，不额外跑一次 diff。
3. **措辞**：用"没有检测到改动"而不是"一定会是空提交"——git 上工作区可能只因未暂存而显脏，真正的提交失败会由 git 自己的 `nothing to commit` 报出。提示只写 stderr，不拦截、不改退出码、不要求 `--force`。
4. **哪些命令不提示**：`kiln do` 只打印 trailer、不提交，不提示。

## 验收

- **E1**：干净工作区 `kiln commit <issue> ask` → exit 0、动作照常记录，stderr 恰好一行 `提示: …`，stdout 与无提示时逐字节相同。
- **E2**：工作区有改动时 → 不出现该提示。
- **E3**：`kiln do` 在干净工作区 → 不出现该提示。
- **E4**：提示只走 stderr；不改退出码、不改提交内容；`--force` 的语义不变。
- **E5**：`go build ./...` / `go vet ./...` / `go test ./...` 全过；`docs/cli-necessity.md` 的空提交段加一句指向本 issue，作为运行期提示的补充。

## 非目标

- 不做 `check` 规则（见"背景"）：空动作合法，且 `check` 不读 diff；
- 不拦截空提交、不引入新的拒绝分支；
- 不改 `status` / `path` 的文本输出（`path --stat`、`path --json` 已经暴露空 stat）；
- 不做逐动作 diff 审计——那是 `target-honesty` 的范围。
