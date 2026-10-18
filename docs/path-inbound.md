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

## hypothesize：path 渲入边后，graph 的定位与两者边界

### 一句话区别

- **`path X` = 单点视角（句子）**：X 与谁有什么关系——**出边 + 入边**，入边用 `inverse(name)` 换名并带轻量标注。
- **`graph` = 全局无视角（箭头）**：整张关系图长什么样——每条边**只在「声明方」那块出现一次**，标签 = 声明时写的名。

### 决定

1. **graph 不补入边，反而要删**。现在 `relationLines`（`engine.go:349`，全仓只被 `Graph` 调用）把入边也按反名画了（`child └── caused-by demo`）——那是 path 的**句子**形态混进了 graph。按 `relation-rendering` 矩阵，graph = 声明源箭头，**不换名、不画入边**。
2. **每条边在 graph 里恰好出现一次**（在声明方的块里），不在目标块里重复。
3. **graph 因此不需要入边索引**：它只是每个 issue 自身 fold 出的**出边**的投影。真正的反向推导只服务 path。
4. **path 改为视角化句子**：出边用声明的名，入边沿用 `inverse(name)`，反名首次出现带标注。
5. **两个渲染器拆开**（现在职责错位：`relationLines` 服务 graph 却含入边、`relationSummary` 服务 path 却只有出边）：graph 用「只出边的箭头」渲染，path 用「出+入+换名+标注」渲染。

### 边界表

| 维度 | `path X` | `graph` |
| --- | --- | --- |
| 视角 | X 为中心 | 无（全局） |
| 形态 | 句子（主语 = X） | 箭头（箭尾 = 声明源） |
| 入边 | **渲染**，换 `inverse` 名 + 标注 | **不渲染** |
| 每条边出现次数 | 每个相关 issue 各一次（X 页 + 对方页） | 全图**恰好一次** |
| 反名 | 需要 | 不需要 |
| 入边索引 | **硬依赖** | 不需要 |
| 数据源 | fold 出的关系 + 入边索引 | 每个 issue fold 出的**出边** |

### 为什么这样分

- 两条腿互补：path 答「这条 issue 边上连着谁」（局部、可读句子），graph 答「整个仓库的关系网」（全局、一眼扫完、不重复）。
- **若 graph 也画入边**：一条边出现 2 次（两端各一次）、同一事实两种名字（`causes`/`caused-by`），图会膨胀并诱导「这是两条关系」——正是 `relation-rendering` 实验里 agent 的困惑点。
- **若 path 不画入边**：X 是目标的关系直接丢掉（本 issue 的 bug）；且写侧只见正名会诱发 `blocked-by`→`blocks` 错配（实验 H4 的意外）。

### 待定子问题（留给 test / conclude）

1. **对称关系 `relates-to`** 在 graph 里画成有向箭头（声明源为尾）还是无向连线？实验 B2 曾对 `relates-to` 的箭头方向困惑。
2. **双侧声明冗余**（A `causes` B 且 B `caused-by` A）时，graph 画两条反向箭头（各声明方一条）还是归一成一种形态？`check` 已有「双侧冗余」告警的规划。
3. **反名标注**的确切文案，与「首次出现」的判定范围（全局首次？还是每页首次？）。

### 对验收的修订

- path：出边 + 入边（换名 + 标注）→ 补实现。
- graph：**去掉入边**（回归声明源箭头，每条边一次）→ 是删而非加。
- 入边索引只进 path，不进 graph。
