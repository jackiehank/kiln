# issue: graph-normalize — `graph` 按规范化箭头收口（去入边 / 对称无箭头 / 双侧冗余 last-wins）

- **domain**：cli
- **关联**：caused-by `relation-rendering`、relates-to `path-inbound`

## 背景

从 `path-inbound` 移出（用户裁定：`path-inbound` 只做 path，graph 另处理）。

`relation-rendering` 的矩阵定：`graph` = **声明源箭头**（箭尾 = 声明方，标签 = 声明的名，不换名、无入边）。现状 `relationLines`（`engine.go:349`，全仓只被 `Graph` 调用）却给被指向方也画了一条换名的入边（`child └── caused-by demo`）——那是 `path` 的**句子**形态混进了 graph。

## 待办（收口）

1. **去掉入边**：graph 每个 issue 的块只列它**自己声明**的出边（声明源箭头），不在目标块里重复。
2. **对称关系不画箭头**：`relates-to` 无 `inverse`，用**无向连线 / 平铺**，不画 `──▶`（用户裁定）。
3. **双侧冗余 last-wins**：同一 `(无序对, 槽)` 被两侧各声明一次时，以**最后一条声明**为准，全图只出一形。这要求补一个**全局 `(无序对, 槽)` 折**——现 `foldRelations` 是按 issue 的 `(target, 槽)`，两条都留。补上即闭合 `relation-revision`「键 = (无序对, 槽)」在实现里的缺口。
4. graph **不需要入边索引**（只投出边）。

## 与 path 的边界（回顾）

| 维度 | `path X` | `graph` |
| --- | --- | --- |
| 视角 | X 为中心 | 全局无视角 |
| 形态 | 句子 | 箭头（对称 → 无向） |
| 入边 | 渲染（换名 + 释义） | 不渲染 |
| 每条边出现次数 | 两端各一次 | 全图一次 |

## 验收

- **G1**：每条 fold 后的边在 graph 里**恰好一次**，落在声明方的块里。
- **G2**：graph 标签 ⊆ 声明名集合（不出现反名）。
- **G3**：对称关系无箭头；有向关系为 `──name──▶`。
- **G4**：同一 `(无序对, 槽)` 至多一条（last-wins）。
- **G5**：`go build ./...` / `go test ./...` 全过；最小仓 `kiln graph` 冒烟。

## 非目标

- 不动 `foldRelations` 的**出边**语义（只补全局折的键）。
- 不改 `status`（维持不渲染关系）。

## apply：`FoldEdges` + 声明源箭头

- `internal/engine/engine.go` 新增 `Edge{From, To, Name, Slot}` 与 `FoldEdges(repo, rel)`：**全局**按 `(无序对, 槽)` 折，动作按日期排序、同侧仍守强度链（rank 大者胜、平手后写），`unrelate` 清整对。
- `Graph` 改为只取 `From == 本 issue` 的边；`relationLines`（含入边换名）删除。
- 形态：有向 `──name──▶ target (state)`；对称（`Inverse(name) == ""`）`──name── target (state)`。
- 测试：`TestGraphDrawsDeclaredArrowsOnce` / `TestGraphDropsArrowForSymmetricRelation` / `TestGraphLastWinsAcrossBothSides` / `TestGraphFoldKeepsStrongerNameWithinOneIssue` / `TestGraphFoldUnrelateClearsThePair`。
- README「已知边界」里「graph 只画 spawns/relates-to/derived-from」的旧说法一并更新。

