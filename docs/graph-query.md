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
