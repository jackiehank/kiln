# issue: graph-query — `graph --json` 的邻域查询与节点标签

- **domain**：cli
- **关联**：relates-to `graph-export`

## 背景

`graph-export` 落地 `graph --json` 之后，agent 访谈（2026-10-23）实测：全量文档 21KB，问"`X` 连着谁"要先把整图拉出来自己过滤，噪声大；节点也缺 `tags`（`list --json` 有）。图是 agent 最需要"按点查"的接口，全量只是它的一个特例。

## 问题

1. 过滤形态：`--issue X --depth N`（邻域）还是 `--subgraph`？默认深度多少？
2. 邻域沿哪些边走：只沿声明方向，还是双向？关系语义上有向边（`blocks`）反向也相关，邻域应该是**无向**的。
3. 过滤后 `summary` / `nodes` / `missing` / `edges` 的语义：同 schema、计数收窄，还是换一套文档？
4. 未知 issue、`--depth` 不带 `--issue`、负数深度：报错退出码各是什么？
5. 节点要不要顺带补 `tags`（`omitempty`）？

## 候选

| 方案 | 形态 | 评价 |
| --- | --- | --- |
| **A** 全量 + `--issue` | 只过滤 `nodes` / `edges` | 简单，但 agent 仍要读 summary 才知道收窄了多少 |
| **B** 同一文档、按邻域收窄 | `summary` / `nodes` / `missing` / `edges` 全部是过滤后的诱导子图 | schema 不变、一个解码器；正则表达式与 `graph --json` 完全一致 |
| **C** 换一套子图文档 | 单独 `subgraph` 字段 | 两套 schema，调用方要分支 |

## 验收（草案）

- **E1**：`kiln graph --json --issue X --depth 1` → 同一 schema，`nodes` = `X` 的一跳邻居（双向）与 `X` 自身；`edges` = 两端都在这个集合里的已折边；`summary` 是收窄后的计数。
- **E2**：`--depth 0` → 只有 `X`、`edges: []`；`--depth` 省略时等于 1。
- **E3**：`--issue` 未知 → 明确报错、exit 1、stdout 0 字节；`--depth` 不带 `--issue` → exit 2；负深度 → exit 2。
- **E4**：不带 `--issue` 时输出与现在**逐字节相同**（全量文档是邻域的特例）。
- **E5**：节点带 `tags`（`omitempty`）；与 `list --json` 的 `tags` 同源。
- **E6**：`go build ./...` / `go test ./...` 全过；真仓冒烟。

## 非目标

- 不做任意路径查询（`kiln path` 已有）；
- 不做子图布局 / 排序策略的改动（排序仍按 id、`(from, to, name)`）；
- 不改 mermaid / ascii 的过滤（本轮 `--issue` / `--depth` 只服务 `--json`；其它格式给了就明确报错，不静默忽略）。

## 设计（2026-10-23）

采用 **B**：同一个 `graph --json` 文档，收窄成邻域诱导子图——`summary` / `nodes` / `missing` / `edges` 全部按过滤后的集合计数与排列，schema 与解码器都不变；全量文档就是"不传 `--issue`"的特例。

- **邻域是无向的**：边 `blocks` 声明方向只表示"谁声明"，反向到达的 issue 同样相关；BFS 双向扩散。
- `--depth` 默认 **1**；`--depth 0` 只留 `X` 自身、`edges: []`。
- 过滤集合包含**未开过的靶**（ghost）：它若在深度内，边照旧写出并进 `missing`，与全量文档的表示一致——`nodes` 仍只有真实 issue。
- 错误：`--issue` 未知 → exit 1、stdout 0 字节；`--depth` 无 `--issue` → exit 2；负深度 / 非整数 → exit 2；`--issue` / `--depth` 与 `--json` 之外的格式同给 → exit 2（不静默忽略）。
- `nodes[]` 补 `tags`（`omitempty`），与 `list --json` 同源。

## apply（2026-10-23）

- `internal/cli/cli.go`：
  - `graph` 的 `allowedFlags` 增加 `issue` / `depth`；`--issue` / `--depth` 只允许与 `--json` 同给，否则 `--issue/--depth 只服务 --json`、exit 2；`--depth` 无 `--issue` → `--depth 需要 --issue`、exit 2；深度非法 → `--depth 需要非负整数`、exit 2。
  - `graphJSON(repo, focus, depth)`：`focus` 非空时先查 issue 存在（否则 `没有 issue "<id>"`、exit 1），再 `neighbourhood` 双向 BFS 出 `dist`，保留真实 issue 的 id（原序）与两端都在 `dist` 里的边；随后照旧算 `missing` / `summary`。`graphNode` 增加 `Tags []string`（`json:"tags,omitempty"`），从 `p.Tags` 抄。
- 测试（`internal/cli/cli_test.go`）：`TestGraphJSONFiltersToANeighbourhood`（`beta` 深度 1 → `alpha,beta` 一条边；深度 0 → 只剩 `beta`）、`TestGraphJSONNeighbourhoodKeepsGhostTargets`（`gamma` → `ghost` 未开，边保留、`missing: [ghost]`）、`TestGraphQueryFlagErrors`（未知 issue / 缺 `--issue` 的 `--depth` / 负深度 / 与 mermaid 同给 四条，stdout 均为空）。
- 复核：`go build` / `go vet` / `go test ./...` 全过；真仓 `graph --json --issue graph-query --depth 1` 得到同 schema 的收窄文档，不带 `--issue` 时与旧输出逐字节相同。

## 结论（已定）

`kiln graph --json` 支持按点查：`--issue <id> [--depth N]`（默认 1），返回**同一 schema** 的邻域诱导子图；`--depth 0` 只有该点本身。邻域按无向边扩散（`blocks` 的反向也算相邻），未开过的靶在深度内会以边 + `missing` 出现，而 `nodes` 恒为真实 issue。`nodes[]` 现在带 `tags`，与 `list --json` 一致。

边界：

- `--issue` / `--depth` 只服务 `--json`；给 ascii / mermaid 会明确报错（exit 2），不静默回退全量；
- 不传 `--issue` 时输出与旧版逐字节相同——全量是特例，不是另一条代码路径；
- 排序仍由 id 与 `(from, to, name)` 决定，邻域不改变顺序，只决定取舍。
