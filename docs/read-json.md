# issue: read-json — `path` / `show` / `worktree list` 补 `--json`

- **domain**：cli
- **关联**：relates-to `machine-readable-output`

## 背景

`machine-readable-output` 收尾时把 `path` / `show` / `worktree list` 三个读命令留在了 `--json` 之外，README 的「已知边界」也还写着这一句。agent 访谈（2026-10-23）里这条被列为 P1：`path` 的动作列表、日期、标题、引用、关系本来就是结构化数据，现在只能解析稳定文本；`show` 的 diff 与 `worktree list` 的行也一样。

`--json` 的既有约定（`machine-readable-output` 结论）不重复：一个对象；键复用 kiln 的词；不写给人看的话；缺键=无、不用 `null`，布尔永远写出。

## 问题

1. `path --json` 的 schema：每个动作带什么（`kind` / `date` / `title` / `refs` / `relations` / `stat`）？`--stat` / `--shortstat` 在 JSON 下如何共处——把 stat 作为字段收进文档，还是要求 `--json` 与 stat 二选一？
2. `show --json` 的内容：diff 是文本，JSON 里放原文（`"diff": "..."`）还是只放元数据 + `stat`？agent 真需要 diff 原文吗？
3. `worktree list --json` 的行：`issue` / `open` / `path` / `main`？取自 `vcs.WorkspaceInfo`。
4. 三个命令的地址错误（未知 issue、动作越界）与 `--json` 的关系：错误走 stderr + 非零退出，stdout 不写半截文档（与既有 `--json` 一致）。

## 验收（草案）

- **E1**：`kiln path --json` → 单对象 `{"issue":"…","action":"…","done":false,"actions":[…],"relations":[…]}`；动作按 path 顺序，每项带 `kind` / `date` / `title` / `refs`（空省略）。
- **E2**：`--json` 与 `--stat` 并存时 stat 进文档（`stat` 字段），文本视图不变；`--json --shortstat` 同理。
- **E3**：`kiln show --json` → 单对象，含被选动作的 `issue` / `ordinal` / `kind` / `date` / `title` / `refs` / `stat`；diff 原文以 `diff` 字段给出（选择靠后决定，避免把大 diff 塞进每个调用）。
- **E4**：`kiln worktree list --json` → 单对象 `{"mode":"…","worktrees":[{issue?,path,open,main}]}`（`main` 行不带 `issue`）。
- **E5**：三命令的文本视图**逐字节不变**；错误路径 stdout 0 字节、exit 非零。
- **E6**：`go build ./...` / `go test ./...` 全过；真仓与空仓各冒烟一次。

## 非目标

- 不做 `--toon`（`machine-readable-output` 留的另一项，继续不评估）；
- 不把 `graph` 的邻域查询搬进 `path`（见新 issue `graph-query`）；
- 不改 `show` 的 diff 呈现本身。
