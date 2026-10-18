# issue: path-inbound — `kiln path` 缺入边：反向边规格未实现

- **domain**：cli
- **关联**：caused-by `relation-rendering`、relates-to `relations-impl`

## 背景

`relation-rendering` 的结论定了「视图 × 关系」矩阵，其中：

> `path X` | 句子（主语=X）| 全部 5 族，**含反向边**（X 为目标那些）

即 `kiln path X` 应把「别的 issue 指向 X」的边也渲染出来（用 inverse 名，如 `caused-by P`）。但当前实现只画出边。

## 现状证据

- `internal/cli/cli.go` 的 `relationSummary` 只遍历 `p.Relations`（`foldRelations` 产出的**出边**），从不扫描其它 issue 的声明。
- `docs/relation-rendering.md` 的「视图 × 关系」矩阵把这一格标注为 **"现状缺反向边"**。
- 入边渲染逻辑其实已经写好，但只接给了 `graph`：`internal/engine/engine.go` 的 `relationLines`（出边 + 入边换名）→ `Graph`；`path` 没走这条路。
- 60b02b9（`relations-impl` apply）给 `relationSummary` 加了 `rel *model.RelationSet` 参数，**函数体从未使用**——词表递进去了却不读，入边渲染像在接线时被漏掉。

复现：A `--causes B`；`kiln graph` 里 B 会显示 `caused-by A`，但 `kiln path B` 的关系区为空。

## 顺带（同一处渲染牵连）

`graph` 反而偏离矩阵：设计说 `graph` = **规范化箭头**（声明源为箭尾、不换名、单一侧声明 ⇒ 唯一形态），但 `relationLines` 给被指向方也画了一条换名的入边，导致同一条边在两个端点各出现一次。建议与 `path` 一并按矩阵收口。

## 验收

1. `kiln path B` 显示入边（`caused-by A` / `blocked-by A` 等，按 B 为主语换名）。
2. `kiln path X` 仍只显当前态，复用词表的 inverse。
3. `graph` 按设计只画声明源箭头，不再重复渲染入边。
4. 出边已渲染的 `(target, slot)` 不再重复渲染其入边。

## 非目标

- 不动 `foldRelations` / 词表结构。
- 不改「`status` 不渲染关系」的决定。
