# issue: commit-scope — 动作提交的内容边界：jj 收全副本、git 只收索引

- **domain**：design
- **关联**：relates-to `commit-order`（曾裁定不做 `--only`，改纪律）；relates-to `cli-necessity`（两后端写侧纪律相反）

## 背景

`commit-order` 收口时（commit `2e1828d`）的裁定是：**不做 `--only`/路径提交**，改用"先开单再落代码"的顺序纪律——因为 jj 后端 `jj commit` 提交整个工作副本，反过来做才需要挪文件。当时把 `--only` 当成"jj 无法稳定支持"而否掉了。

用户 2026-10-23 指出这个摩擦是**对称的**：使用 git 后端时，摩擦点是"必须先 `git add`"——kiln 的 git 后端 `Action()` 是裸 `git commit -m`，从不暂存；调用方忘了 add 就 `nothing to commit` 失败。也就是说两个后端把"选什么进这次动作"推给了相反方向的默认值：

- **jj**：全收。要排除无关文件，只能靠调用方自律或挪文件；
- **git**：只收索引。要纳入改动，只能靠调用方先 add。

本 issue 要回答的是：一个动作的提交内容边界由谁决定——后端默认、调用方显式路径，还是区域（worktree）结构？以及 `--only` 是否值得做、做了语义如何统一。

## 问题

1. **可行性**：两个后端是否都原生支持"只提交指定路径"？（最新实测见 collect 段——jj 有 `[FILESETS]...`，git 有 `git commit -- <paths>`。）
2. **语义统一**：`--only a.md b.md` 在两后端应产生一致的可观察结果——只有这两个文件进提交，其余改动原样保留（jj 留在 `@`；git 留在工作区/索引）。
3. **未跟踪文件**：git 上 `git commit -- u.md` 对未跟踪文件报 `pathspec 'u.md' did not match any file(s) known to git`；jj 自动跟踪所以没有这个问题。`--only` 在 git 上是否代 `git add`？代做的索引副作用要不要写进文档？
4. **静默空提交**：jj 对不存在/无改动的路径是 `Warning: …` + **exit 0 + 空提交**。`--only` 若照搬会放大 `empty-commit-hint` 刚提示过的问题，必须前置校验。
5. **接口成本**：`vcs.Repo.Action(message string)` 需要扩成携带路径集；两个后端实现与 `cli_test.go` 的 fakeRunner 都要改。值与不值？
6. **替代解是否足够**：默认隔离区（worktree 默认开、全副本=该 issue 的工作）、`target:` 声明 + `target-honesty` 审计、顺序纪律，能否替代 `--only`？

## 候选

- **A. 现状 + 纪律**（commit-order 的裁定）：零成本，但摩擦真实存在，用户已经指出对称的另一面；单独作为结论显得回避。
- **B. `--only <path>...`**：jj `jj commit -m msg <paths>`；git `git add -- <paths>` 后 `git commit -m msg -- <paths>`；前置校验路径存在且有改动，坏路径 exit 1 且不产生提交。默认（无 `--only`）行为不变。
- **C. 默认隔离区**：把 worktree 默认打开，让"全副本=该 issue 的工作"成为结构性事实；`--only` 降级为逃生口。与 B 可组合，但改了默认工作流，单独也解不掉"专属区里仍有无关改动"。
- **D. 声明式**：不限制提交内容，靠 `target:` trailer 声明 + `target-honesty` 的 check 审计"声明与 diff 不符"。把摩擦从"选择"变成"可审计的声明"，零接口改动，但只能事后发现、不能阻止混入。
- **E. 两步式 `kiln add` / staging 概念**：最像 git，但 jj 没有暂存区，语义会分裂；否。
- **F. 组合**：B + 文档化 C 的隔离区建议 + 沿用 empty-commit-hint 的提示。

## 验收

- **E1**：两后端 `kiln commit <issue> <action> --only a.md b.md` → 提交里只有这两个文件；其余改动原样保留（jj 在 `@`，git 在工作区/索引）。
- **E2**：git 后端对未跟踪路径可用（自动 `git add -- <列出的路径>` 或明确要求先 add，二选一并在文档写死）；索引副作用写进文档。
- **E3**：路径不存在或无改动 → exit 1，**不产生任何提交**（jj 不允许落到空提交）；错误信息点名坏路径。
- **E4**：无 `--only` 时行为与现在逐字节相同（既有测试守住）；`do` 是否接受 `--only` 要明确（建议拒收，`do` 只打印 trailer）。
- **E5**：`--only` 与 `commit` 其余 flag（`--domain`/`--title`/`--relates-to`/`--force` 等）组合正常；参数为 0 个路径时按 `--only` 缺参报错。
- **E6**：测试：两后端各有用例（fakeRunner 断言 argv、真仓/临时仓冒烟）、坏的路径用例、无 `--only` 回归；`go build`/`go vet`/`go test ./...` 全绿。

## 非目标

- 不做交互式 `split`、不做 hunk 级部分暂存；
- 不改 `Action` 的默认语义（无 `--only` = 今天的整副本/索引行为）；
- 不做逐动作 diff 审计——那是 `target-honesty` 的范围；
- 不改 `check` 规则。
