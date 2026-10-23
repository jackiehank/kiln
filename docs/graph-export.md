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

## 验收（草案）

- **E1**：`kiln graph --json` → 单 JSON 文档，`{"nodes":[{...}],"edges":[{...}]}`；边**已折**（与 ascii 同一份 `FoldEdges`），每条恰好一次。
- **E2**：节点 `id` / `domain` / `state`；边 `from` / `to` / `name` / `slot` / `symmetric`。
- **E3**：`kiln graph --mermaid` → 可直接被 mermaid 渲染：有向 `a -- name --> b`，对称 `a --- b`；node id 只含 `[A-Za-z0-9_]` 且无碰撞。
- **E4**：默认 `ascii` 输出**逐字节不变**；`--format=json` ≡ `--json`。
- **E5**：`--json --mermaid` 同时给 → 明确报错（exit 2），不静默选一个。
- **E6**：`go build ./...` / `go test ./...` 全过；最小仓三种格式冒烟。

## 非目标

- 不引依赖、不做布局 / 不做渲染（mermaid 与 json 都只是文本）；
- 不做 `dot` / Graphviz 格式（真要时再加，`Edge` 已是现成中间层）；
- 不改 `ascii` 的形态与默认值；
- 不扩展到 `path` / `show`。
