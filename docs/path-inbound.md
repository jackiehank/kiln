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

## hypothesize：test 设计——怎么确认 path/graph 边界

### 1. 判定对象：把矩阵写成不变式

边界不是「看起来对」，而是五条可断言的性质（I1–I5）：

| # | 不变式 | 含义 |
| --- | --- | --- |
| **I1** | graph 边唯一 | 全图里每条 fold 后的边（无序对, 槽, 值）**恰好出现一次**，且落在**声明方**的块里 |
| **I2** | graph 无入边/无反名 | graph 的标签 ⊆ 声明名集合；不出现「因 X 是目标才换出来的」反名 |
| **I3** | path = 出 ∪ 入 | path X 的行 = X 的出边（声明名）+ 入边（`inverse(name)`），按 (对, 槽) 去重 |
| **I4** | 互补不丢边 | graph 表示的边集 == 所有 path 页表示的边集；graph 不重复、path 不遗漏 |
| **I5** | 对称回落 | `relates-to` 无 inverse，两端同号出现（不换名） |

**I4 是「边界正确」的硬标准**：删掉 graph 的入边不会丢信息，因为那条边在对方 issue 的 path 页里。

### 2. 三层手段（从便宜到贵）

1. **golden 表驱动单测（主力）**：构造内存 `model.Repo` + `DefaultRelationSet()`，对 fixture 断言 `Graph(repo)` 与新的 `perspectiveLines(repo, X)` 的**精确文本**。确定性、无需 git、毫秒级。
2. **不变式单测（抗文案变动）**：不写死字符串，直接断言 I1–I5；以后改箭头符号 / 标注文案不会误报。
3. **最小仓冒烟（只证 CLI 接线）**：真 git 仓跑 `kiln path X` / `kiln graph`，把 1/2 的逻辑再走一遍端到端。

> **前置重构**：path 的渲染现在在 `cli.go` 的 `relationSummary`（CLI 里，不好单测）。test 前应把它下移到 engine，变成一个纯函数（出边 + 入边 + inverse + 标注），与 `Graph` 共用同一份 fold 数据。

### 3. 为什么不需要 subagent 实验

- 「视角化会不会迷惑人」已由 `relation-rendering` 的 4-agent 实验回答（H1–H3 过）。
- 本 issue 要确认的是**边界是否按矩阵实现**——确定性契约，golden + 不变式即可。
- 只有当要改**反名标注的文案**时，才值得再跑一次可读性实验（判据沿用 relation-rendering 预登记那套）。

### 4. fixture 必须覆盖的陷阱

| # | 场景 | 期望 |
| --- | --- | --- |
| 1 | X 只有出边 | graph X == path X 的出边部分 |
| 2 | X 只有入边 | graph X 无关系行；path X 显示反名（**本 bug 的核心**） |
| 3 | 同一对双槽（`A blocks B` + `A causes B`） | 两视图都不折叠（不同槽各一行） |
| 4 | 同一对反向视图（`B caused-by A`） | 与 `A causes B` 是**同一条边**，不成两条 |
| 5 | 对称 `relates-to` | 两端各一行、同号 |
| 6 | 双侧冗余声明（`A causes B` 且 `B caused-by A`） | path 去重；graph 的形态待定（子问题 #2） |
| 7 | 纯被指 issue | path 只入边；graph 该块无关系行 |
| 8 | 链 A→B→C | 每边只在声明方块出现一次 |
| 9 | dangling 目标 | `stateOf` 显示 `missing`，不 panic |

### 5. 阻塞项：三个待定子问题先冻结

golden 断言要冻结文本，所以这三项必须先定（否则测不了）：

1. `relates-to` 在 graph 的形状（有向箭头 vs 无向连线）。
2. 双侧冗余在 graph 的形状（两条反向箭头 vs 归一）。
3. 反名标注的文案与「首次出现」范围。

### 6. 判据

- I1–I5 全过；
- fixture 的精确输出匹配 golden；
- `go build ./...` / `go test ./...` 全过；
- 最小仓 `kiln path` / `kiln graph` 冒烟与单测一致。

### 7. 顺序

先冻结第 5 节三项（可并入本 issue 的 `conclude`，或再补一个 `hypothesize`）→ 下移 path 渲染到 engine → 写 fixture + golden 与不变式 → 最小仓冒烟。

## hypothesize：冻结三项待定（用户裁定）

### 1. 无向关系不画箭头

`relates-to` 无 `inverse`（对称），在 graph 里**不画箭头**，用无向连线/平铺表示；**只有有向关系才画** `──name──▶`。

### 2. 双侧冗余 = last-wins

同一 `(无序对, 槽)` 被两侧各声明一次（如 A `causes` B 且 B `caused-by` A）时，**以最后一条声明为准**展示（同 `relation-revision` 的读侧 LWW），全图只出一形。

> **连带项**：这要求 fold 的键是 **`(无序对, 槽)`（全局）**；而现在的 `foldRelations` 是**每个 issue 自己的 `(target, 槽)`**，两条声明都留着。所以 #2 不只是 graph 渲染——要**补一个全局折**。这恰好补上 `relation-revision` 结论里「键 = (无序对, 槽)」在本实现里的缺口。

### 3. 反名标注：沿用 relation-rendering 的定稿

之前已讨论（`docs/relation-rendering.md`）：

- 预登记 mitigation #2（`:203`）：`spawns ＝ 我引发的（derived-from 的反面）`。
- conclude 定稿（`:295` / `:378`）：**反名首次出现时带一句轻量标注**，例 `P1 spawns P2（= P2 由 P1 引发）`。
- 相邻：`docs/help-relation-flags.md:260` 已为 `-h` 预留 `relations.names.gloss`。

**「首次出现」的含义（澄清）**：kiln 无状态、每次 `path X` 只渲一页，所以范围 = **本次输出这一页**——同一反名在该页**只标注第一行**，后续同名的行不再重复；没有跨调用记忆。graph 不用反名，故无标注。

**残余待定**：标注内容是「**静态名释义**」（per-name，可挂 `names.gloss`，与「首次」天然契合）还是「**行内展开**」（`X 反名 P（= P 正名 X）`，per-line）？**倾向静态名释义**。另：反名取**规范名**（`caused-by` / `blocked-by` / `has-part` / `superseded-by`），示例里的 `spawns` / `derived-from` 是改名前的旧名。

### 对不变式的补充

- **I5 修订**：对称关系在 graph **无箭头**（无向）。
- **新增 I6**：同一 `(无序对, 槽)` 在 graph 里**至多一条**（last-wins）。

## conclude：范围收窄到 path，反名标注取 A（2026-10-18）

### 0. 范围裁定

本 issue **只做 `path`**。一切 `graph` 的收口（去入边、对称不画箭头、双侧冗余 last-wins + 全局折）**移出**，另开 `graph-normalize`。因此前几节里关于 graph 的 I1 / I2 / I6 **不再是本 issue 的验收项**。

### 1. path 渲染规格（定稿）

- 形态：**视角化句子**，主语 = X。
- 内容：X 的**出边**（用声明的名）+ **入边**（用 `inverse(名)`）；按 `(目标, 槽)` 去重，一行一条。
- 反名：取**规范名**（`caused-by` / `blocked-by` / `has-part` / `superseded-by`）；`relates-to` 无 inverse，回落同号。
- **反名标注 = A 静态名释义**：`<名> ＝ <一句释义>`（如 `caused-by ＝ 谁引发了我（causes 的反面）`）。挂在反名**在本页首次出现**的那一行，同页后续同名不重复。释义属于**词表**（每规范名一句，内置默认，配置可覆盖），`-h` 复用同一份。
- 顺序：**先出边、后入边**，各自按 `(目标, 名)` 稳定排序。

### 2. path 不变式（本 issue 验收）

- **P1**：path X 的行 = 出边（声明名）∪ 入边（inverse 规范名），按 (对, 槽) 去重，无重复。
- **P2**：对称关系（`relates-to`）在两端同号出现。
- **P3**：反名在本页首次出现时附静态释义，后续同名不重复。
- **P4**：顺序稳定（先出后入，各按 (目标, 名) 排）。

> 原 I3 微调为 P1；I4 的「graph 不丢边」一面移入 `graph-normalize`；I5 收敛为 P2；I1 / I2 / I6 移出。

### 3. test 收敛

- 主力：path 的 golden 单测（对 fixture 断言 `path X` 精确文本）+ P1–P4 不变式单测。
- fixture 保留 path 相关陷阱：只有出边 / 只有入边（本 bug 核心）/ 同对双槽 / 同对反向视图 / 对称 relates-to / 双侧冗余（path 侧去重）/ 纯被指 / 链 / dangling。
- **前置重构**：path 渲染从 `cli.go:relationSummary` 下移到 engine 纯函数。
- graph 的 fixture 与断言 → `graph-normalize`。
- 最小仓冒烟 `kiln path X`。

### 4. 落地清单（本 issue）

1. 词表加 `gloss`（9 名 + 反名各一句；内置默认，配置可覆盖）。
2. path 渲染下移到 engine：出边 + 入边（inverse）+ 去重 + 顺序 + 首次释义。
3. `-h` 复用同一份 gloss（与 `help-relation-flags` 的预留位对齐）。
4. 单测：golden + P1–P4。

### 5. 移出（→ `graph-normalize`）

- graph 去掉入边（只画声明源箭头）。
- 对称关系不画箭头。
- 双侧冗余 last-wins + **全局 `(无序对, 槽)` 折**（补 `relation-revision` 的键缺口）。

## apply：落地与验证（2026-10-18）

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/model/relation.go` | `RelationName` 加 `Gloss` / `InverseGloss`；`RelationSet.Gloss(name)`；`DefaultRelationSet` 给 9 名各一句释义（反名经 `InverseGloss` 自动继承） |
| `internal/engine/engine.go` | 新增 `PathLines(repo, p, rel)`：出边（声明名）在前、入边（`inverse` 名）在后，各按 `(目标, 名)` 排序；按 `(目标, 槽)` 去重；翻转名在本页首次出现附 `（名 ＝ 释义）` |
| `internal/cli/cli.go` | `path` 改用 `engine.PathLines`，关系区多行渲染；删除只画出边的 `relationSummary`；`-h` 的关系 flag 从词表带出 gloss |
| 测试 | `engine_test`：5 个 `PathLines` 用例（出先入后 / 双侧去重 / 对称不翻转 / 释义一次 / 纯入边）；`cli_test`：path 入边 + `-h` 释义 |

### 渲染形状

```
  关系:
    blocks b
    relates-to d
    caused-by b（caused-by ＝ 谁引发了我）
    blocks c
```

出边在前；翻转名首次出现才附释义；已作为出边出现过的名字不再解释。

### 验证

- `go build ./...`、`go vet ./...`、`go test ./...` 全过。
- 最小仓端到端：`b causes a` / `c blocked-by a` / `a blocks b relates-to c` → `kiln path a` 显示 `caused-by b（…）` 与 `blocks c`；`kiln path b` 显示入边 `blocked-by a（…）`。
- 本仓 dogfood：`kiln path relation-revision` 出边 + 入边（`causes relations-impl`）正常；`-h` 关系小节带释义。

### 未做

- graph 的收口（去入边、对称无箭头、last-wins 全局折）→ `graph-normalize`。
