# issue: graph-export — `graph` 增加 `--json` / `--mermaid` 中间产物

- **domain**：cli
- **关联**：relates-to `graph-normalize`、relates-to `machine-readable-output`

## 背景

`graph` 归一化（`graph-normalize`）之后，全图只剩一份中间表示：`FoldEdges` 产出的 `Edge{From, To, Name, Slot}` 列表，配上 issue 的 `id / domain / state`。文本树 `ascii` 是给人看的投影；一旦要给**能画图的软件**吃，缺一个稳定的中间产物。

用户诉求（2026-10-23）：**「加个 `--json`、`--mermaid` 选项就可以」**——不追求一步到位的格式矩阵，先给两种最常用的出口：

- `--json`：通用中间产物（d3 / cytoscape / Graphviz 转换 / 自己的脚本）；
- `--mermaid`：文本转图，GitHub / Markdown / mermaid.live 原生可渲染，agent 也能直接读。

`machine-readable-output` 的结论里留了「真有第二种格式时再评估 `--format=`」——本条就是那个时刻，但只承接 graph，不扩散到其它命令。

## 问题

1. `--json` 的 schema：节点要不要带 `domain`？边要不要带 `slot` / `symmetric`？——目标是"拿到就能画"。
2. `--mermaid` 的形态：节点 id 怎么消毒（issue id 含 `/` 等字符）？状态放节点 label 还是注释？对称边用 `---`？
3. 与既有 `--format=ascii` 的关系：保留 `--format` 并接受新值（`--format=json` ≡ `--json`），还是只加布尔 flag？
4. 多选冲突：`--json --mermaid` 同时给时报错还是后者胜？

## 候选

| 方案 | 形态 | 评价 |
| --- | --- | --- |
| **A** 只用 `--format=` | `--format=ascii\|json\|mermaid` | 一致但多一层枚举；与 `--json` 家族不一致 |
| **B** 只加布尔 flag | `--json`、`--mermaid` | 与 `status` / `list` / `check` 的 `--json` 一致；`--format` 仅留 `ascii` |
| **C** B + `--format` 别名 | 同上，且 `--format=json` ≡ `--json` | 兼容旧调用；实现上多一行归一 |

**倾向 C**：`--json` 是本仓既有约定，`--format` 已存在、留作别名即可。

## 验收（定稿，2026-10-23）

- **E1**：`kiln graph --json` → 单 JSON 对象 `{"summary":{...},"nodes":[...],"missing":[...],"edges":[...]}`（`missing` 为空时省略）；边**已折**（与 ascii 同一份 `FoldEdges`），每条恰好一次。
- **E2**：节点 `id` / `domain` / `action` / `done` / `title`；边 `from` / `to` / `name` / `directed`；`missing` 列 `to` 指向未开 issue 的 id。**不输出** `slot` / `symmetric`（理由见下节）。
- **E3**：`kiln graph --mermaid` → 可直接被 mermaid 渲染：有向 `a -- name --> b`，对称 `a --- b`；node id 只含 `[A-Za-z0-9_]` 且无碰撞。（已实现）
- **E4**：默认 `ascii` 输出**逐字节不变**；`--format=json` ≡ `--json`。
- **E5**：`--json --mermaid`、`--json --format=ascii` → 明确报错（exit 2），不静默选一个。
- **E6**：`go build ./...` / `go test ./...` 全过；最小仓三种格式冒烟；**空 issue 仓**（有历史、无 `issue:` trailer）输出合法 JSON 而不是文本 `（空）`。

## 非目标

- 不引依赖、不做布局 / 不做渲染（mermaid 与 json 都只是文本）；
- 不做 `dot` / Graphviz 格式（真要时再加，`Edge` 已是现成中间层）；
- 不改 `ascii` 的形态与默认值；
- 不扩展到 `path` / `show`。

## apply（2026-10-23：先落 `--mermaid`）

用户要求**先出一条 `--mermaid` 看效果**，所以本轮只落 E3，`--json`（E1/E2）与双选报错（E5）留待下一步。

- `internal/engine/engine.go`：新增 `Mermaid(repo, rel) string`、`sanitizeNodeID`、`quoteLabel`。输出 `flowchart LR`：每个 issue 一个 `i_<消毒 id>["<id> (<state>)"]`；被声明却没被任何提交开过的目标补 `(missing)` 节点，和 ascii 的缺失目标显示对齐；边直接复用 `FoldEdges`，按 `(from, to, name)` 排序，有向 `-- name -->`、对称（`rel.Inverse(name) == ""`）`---`。id 消毒到 `[A-Za-z0-9_]`，碰撞依次加 `_2`、`_3`；节点与边都排序，同仓每次同文本。
- `internal/cli/cli.go`：`graph` 接受 `--mermaid`，并把 `--format=mermaid` 归一为同一选项（`--format=ascii` 仍是默认）；未知格式的报错扩成 `ascii|mermaid`；`kiln -h` 的命令行同步为 `graph [--format=ascii|mermaid] [--mermaid]`。
- 测试：`engine_test.go` `TestMermaidRendersTheFoldedGraph`（消毒碰撞 `a.b`/`a/b`、`(missing)` 节点、有向与对称各一条，逐字节断言）、`TestMermaidIsStable`；`cli_test.go` `TestGraphMermaidFlagMatchesFormatAlias`、`TestGraphRejectsUnknownOrConflictingFormats`。
- 复核：真仓 `kiln graph --mermaid` 111 行 = 48 节点 + 62 边；`--mermaid` 与 `--format=mermaid` 逐字节相同；`--format=json` 与 `--mermaid --format=json` 均 exit 2；`go build`/`go vet`/`go test ./...` 全过；ascii 分支代码未动。
- 预览：`.kiln/graph-preview.{mmd,md,html}`（`.kiln/` 被忽略，不入库）；`graph-preview.html` 用本地 `mermaid.min.js`，离线可开。

## 设计（`--json`，2026-10-23）

用户（m00870）：`--svg` 不做，"还得自己考虑样式，太重"；`--json` 好好设计——**"如何让 json 输出能让 agent 看明白"**。

既有约定（`machine-readable-output` 结论）：一个对象；键复用 kiln 的词；不写给人看的话（无 hint / legend / prose）；缺键表示"无"、不用 `null`，布尔永远写出。据此定形：

```json
{
  "summary": { "issues": 50, "edges": 62, "missing": 0 },
  "nodes": [
    { "id": "graph-export", "domain": "cli", "action": "design", "done": false,
      "title": "graph 增加 --json / --mermaid 中间产物" }
  ],
  "missing": ["ghost-issue"],
  "edges": [
    { "from": "graph-export", "to": "graph-normalize", "name": "relates-to", "directed": false },
    { "from": "evidence-fluidity", "to": "cli-necessity", "name": "blocks", "directed": true }
  ]
}
```

逐条理由（都回答"agent 凭什么看明白"）：

- `nodes[].title` 取该 issue **最后一个 action** 的 `--title`，与 `action` 同一行同一时点；agent 不必再 `kiln show` 就知道这行是什么。id + 状态文本树已有，标题文本树不显示，所以不算重复。
- `edges[].directed` 是布尔、**永远写出**：`true` = 有向（箭头），`false` = 对称（无箭头）。语义由数据给出，agent 不必先查关系词表（`blocks` 有向、`relates-to` 对称都不言自明）。**不输出 `symmetric`**——它与 `directed` 互补，双写就是两种说法。
- 不输出 `slot`：同（对, 槽）取强是折叠的**过程**，不是图的事实；给 agent 只会多一个要解释的字段。
- `missing` 单独成列：这样 `nodes` 恒等于真实存在过的 issue，`edges[].to ∈ nodes ∪ missing` 一看便知。若塞进 `nodes`，就要在 50 行里各写一个 `"missing": false`，比单列更难读。
- `summary` 三个计数，脚本不必自己数。
- 顺序确定：`nodes` 按 id；`edges` 按 `(from, to, name)`——同仓每次同文本。
- 空仓输出 `{"summary":{"issues":0,"edges":0,"missing":0},"nodes":[],"edges":[]}`：文本视图的 `（空）` 是给人看的，不进 JSON。
- `--json` ≡ `--format=json`；`--json --mermaid`、`--json --format=ascii` 都是 exit 2，不静默择一；`--json --format=json` 同义，允许。

## apply（2026-10-23：落 `--json`）

- `internal/cli/cli.go`：
  - `graph` 现在接受 `--json` 与 `--format=json`（同义）；与 `--mermaid` / `--format=ascii` 同给时 exit 2，报 `--json 与 --mermaid 冲突` / `--json 与 --format=ascii 冲突`；未知格式报 `仅支持 --format=ascii|mermaid|json（当前: …）`；`kiln -h` 的命令行同步。
  - 新增 `graphDoc` / `graphSummary` / `graphNode` / `graphEdge` 与 `graphJSON(repo)`：边取同一份 `engine.FoldEdges`，按 `(from, to, name)` 排序；`nodes` 按 id 排序、每个带最后一个 action 的 `title`；`directed = rel.Inverse(name) != ""`；`missing` 收 `to` 不在 `repo.Issues` 的唯一 id，排序后单列（空则省略）。
  - 空 issue 仓走 JSON 分支（不再打印 `（空）`）。真·零提交仓在读历史时就会失败（`读取历史失败: git log …`，exit 1）——这是 backend 既有行为，与文本视图一致，不在本 issue 内改。
- README：命令表、与 agent 协作一节、已知边界的「机器可读输出只落在四个命令」三处同步。
- 测试：`internal/cli/cli_test.go` 新增 `TestGraphJSONRendersTheFoldedGraph`（双侧声明的 `relates-to` 折成一条、`caused-by` 指向未开 issue 进 `missing`、nodes 排序与 title）、`TestGraphJSONMatchesFormatAlias`、`TestGraphJSONOnEmptyRepoIsStillJSON`；`TestGraphRejectsUnknownOrConflictingFormats` 改用 `--format=yaml` / `--mermaid --json` / `--json --format=ascii`（`--format=json` 现在是合法值）。
- 复核：真仓 `graph --json` → 50 issue / 65 边 / 0 missing；两次运行与 `--format=json` 逐字节相同；ascii 与 mermaid 输出未变；空 issue 仓 → `{"summary":{"issues":0,"edges":0,"missing":0},"nodes":[],"edges":[]}`（exit 0）；`--json --mermaid`、`--json --format=ascii`、`--format=yaml` 均 exit 2 且 stdout 0 字节；带有向边指向未开 issue 的临时仓 → `missing: ["ghost"]`、边 `directed: true`；`go build` / `go vet` / `go test ./...` 全过；`kiln check` ok=true。
