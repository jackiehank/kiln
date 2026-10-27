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

## 采集（2026-10-23）

三个文本视图与它们手里已有的数据：

- `path`（`internal/cli/cli.go:862–914`）：首行 `<id>  <已完成|进行中>（当前 <动作名>）`；每动作一行 `  N. <kind 左宽 12> <shortid 8> <subject>`；`--shortstat` 行尾追加 `  (<backend shortstat>)`，`--stat` 再追加缩进的 diffstat 块；末尾 `  关系:` 加 `engine.PathLines`（出边用声明名，入边用反名，symmetric 同名，换名首次出现带 `（反名 ＝ 声明名）` 注解）。数据源：`model.Issue.Actions` 的 `ID/Subject/Date/Kind/Title/Refs`、`e.progressWord`、`e.rel()`、backend `ShowStat`。
- `show`（`cli.go:920–995`）：首行 `<id> · <ordinal>/<total> <kind> · <shortid> · <subject>`、空行、`backend.Show(id)` 的提交全文（trim）；`--stat` 换成 diffstat；选择器为 1-based 序号，或动作名（取该动作的最后一个，重名时 stderr 列出全部序号）。
- `worktree list`（`cli.go:778–807`）：每区一行 `<path>  <role>  <state>`；role = `main（只读/只合并）` 或 `worktree <issue>`；state 由 `vcs.AreaState{Gone, Dirty, Unmerged}` 拼成 `目录已删` / `有改动` / `未合并`（逗号连接，空则 `干净`），main 为 `—`。
- 既有 JSON 基建：`writeJSON`（2 空格缩进、只写 stdout、失败写 stderr 并沿用文本视图的退出码）、`issueCore` 可复用；`check` 的布尔/缺键约定照旧。

## 设计（2026-10-23）

三个命令都加 `--json`，各出一份**单对象**；不加 `--json` 时文本视图逐字节不变。stat 不做二选一：`--json --stat` / `--json --shortstat` 把 stat 文本作为字段收进文档。

- `path --json` → `{"issue","action":<当前动作名>,"done":bool,"actions":[{"ordinal","action","id","date","title","refs"?}],"relations":[{"from","to","name","directed"}]}`。`relations` 取 `FoldEdges` 中与该 issue 相邻的边，与 `graph --json` 共用词表：`name` 是声明名、`from` 说清谁声明；入边不换反名。
- `show --json` → `{"issue","ordinal","total","action","id","date","title","body"}`；`body` 在 `--stat` 时是 diffstat、否则是提交全文；**不放 diff 原文**（大，且文本视图仍能拿），留作以后按需加 `diff`。
- `worktree list --json` → `{"mode":"git|jj","worktrees":[{"path","main","issue"?,"gone","dirty","unmerged"}]}`；布尔永远写出，`issue` 缺省即 main。
- 错误与文本视图同一套：未知 issue / 越界序号 exit 1/2，stdout 0 字节。

验收按上文 E1–E6，其中 E1/E3 的 `relations`、`diff` 按本节调整为“共用 graph 词表的相邻边 / 本轮不放 diff”。

## apply（2026-10-23）

- `internal/cli/cli.go`：
  - `path` / `show` / `worktree list` 的 `allowedFlags` 加 `json`；`--json` 时分别走 `pathJSON` / `showJSON` / `workspaceList(ws, f)`，文本分支原样保留。
  - 新增 `pathDoc` / `pathAction` / `showDoc` / `worktreeListDoc` / `worktreeEntry`；`pathJSON` 的 `relations` 由 `incidentEdges` 给出——取 `FoldEdges` 中与该 issue 相邻的边并按 `(from, to, name)` 排序，`directed` 用 `rel.Inverse(name) != ""` 判定，与 `graph --json` 同义。
  - `--json --stat` / `--json --shortstat`：`path` 把每动作的 diffstat 收进 `stat` 字段；`show` 把 `body` 换成 diffstat。`worktree list` 的 `gone` / `dirty` / `unmerged` 恒写，`issue` 缺省即 main，`mode` 取后端名（`git` / `jj`）。
  - `writeJSON` 增加 `SetEscapeHTML(false)`：`show` 的 body 是提交全文，diff 里的 `<` / `>` / `&` 不再被转义成 `\u003c` 之类。
  - 帮助：`usageCommands` 三条命令补 `--json`，`usageOtherFlags` 按命令列出 `--json`。
- 测试（`internal/cli/cli_test.go`）：`TestPathJSONRendersThePath`、`TestPathJSONCarriesStatInFields`、`TestPathJSONIncludesIncidentRelations`、`TestShowJSONRendersTheSelectedAction`、`TestReadJSONErrorsPrintNothing`、`TestWorktreeListJSONRendersAreaStates`。
- 复核：`gofmt` / `go vet` / `go test ./...` 全过；真仓 `path browser-shot --json`（3 个动作，含完整 hash 与 RFC3339 日期）、`show browser-shot conclude --json`（body 为 jj 的提交全文）、`worktree list --json`（`mode: "jj"`、main 行无 `issue`）各冒烟一次；三条命令的文本视图仍由既有逐字节断言守着。

## 结论（已定）

七个读命令全部有 `--json`。三条新命令的契约：

- `path --json`：`issue` / `action`（当前动作名）/ `done` / `actions[]`（`ordinal`、`action`、`id`、`date`（RFC3339）、`title`、`refs`、`stat`）/ `relations[]`（与 `graph --json` 同形的相邻边）。`actions` 与 text 视图同序，`ordinal` 可直接喂给 `kiln show`。
- `show --json`：`issue` / `ordinal` / `total` / `action` / `id` / `date` / `title` / `body`；`body` 是提交全文（`--stat` 时是 diffstat），**不含 diff 原文**——需要 diff 的调用方仍可用文本视图。
- `worktree list --json`：`mode` / `worktrees[]`（`path`、`main`、`issue`、`gone`、`dirty`、`unmerged`）；三个状态布尔恒写，`issue` 缺省即 main。
- 批量形态（issue `path-batch`）：`kiln path --all --json` / `kiln path <id> <id> --json` → `{"issues":[<上面那条文档>…],"errors"?}`。单条文档形状不变，所以同一个解码器两种形态都读得动；`errors` 是语言中立 token（`unknown-issue` / `stat-failed`），单条失败不拖垮整份。

边界：`relations` 里的靶可能从未开过（ghost），文档不另设 `missing` 列表——`to` 是否在 `issues` 里由调用方自己比对；`show` 本轮不放原始 diff；`--toon` 仍未评估。
