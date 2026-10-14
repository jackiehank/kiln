# issue: relation-types — 关系类型应该有哪些

- **状态**：ask（进行中）
- **domain**：design
- **关联**：derived-from `object-semantics`

## 背景

`object-semantics` 定下了"**issue 是原子的，组合靠关系**"。于是"关系表达力"就从边角料升成了核心问题：现在词表里只有 5 个关系键——`derived-from` / `spawns` / `relates-to` / `blocks` / `supersedes`——够不够？类型该怎么分？

## 需要覆盖的语义

从讨论里归纳出至少三种：

1. **引发（顺序无关）**：一个问题**引发**了另一个问题，两者可以各自处理，谁先谁后无所谓。
   → 现在由 `derived-from`（子→父）/ `spawns`（父→子）承担。
2. **分解 + 依赖（顺序相关）**：一个问题**很复杂**，拆成好几个问题；这些子问题**都处理完**，母问题才能继续推进。
   → 这与 (1) 是**不同的语义**：*拆解 ≠ 引发*。拆出来的子问题被母问题依赖，母问题要等它们。
3. **相关 / 取代**：`relates-to`（双向相关）、`supersedes`（新取代旧）。

## 缺口

- `derived-from` / `spawns` **把两种语义混在一起**了：既表示"由它引发"（无序），又被当成"由它拆解"（有依赖）。分不开。
- 缺"**分解**"类型：需要类似 `part-of` / `decomposes-into` 的东西，表达"母问题拆成这些子问题"。
- 缺"**依赖方向**"的直白说法：现有 `blocks:` 是"我阻塞了谁"（阻塞者→被阻塞者）；从被阻塞者视角看就是"我依赖谁"，但命名是从阻塞者出发的，读起来要绕一下。
- 缺"**组合语义**"：母问题拆成子问题后，是**全部完成（and）**才算推进，还是**任一完成（or）**就够了？现在无法表达。

## 关键细化：依赖的粒度

这是必须回答的分叉——**依赖挂在 issue 上，还是挂在 action 上？**

- **issue 级（粗）**：`issue B depends-on issue A` → A 没到终点，B 就不该推进。
- **action 级（细）**：`B` 的某个**具体动作**依赖 `A` 的某个**具体动作** → 精确到"哪一步等哪一步"。

| | issue 级 | action 级 |
| --- | --- | --- |
| 表达力 | 粗：说"B 等 A 完成" | 细：说"B 的 apply 等 A 的 conclude" |
| 复杂度 | 一个键就够 | 关系值里得指明具体动作 |
| 对原子模型的冲击 | 天然契合 | 需要给 action 一个**可寻址的标识** |

**难点在这里**：action 级依赖要求 action 能被寻址。目前 action 唯一能标识自己的只有 **commit hash**——而 hash 会随历史重写而变，天生不稳定。更微妙的是，刚退役的 `object:`（`q1`/`a1`）**本来就是一种 action 标识**；如果要做 action 级依赖，是不是等于要把 `object` 以别的形式请回来？这是本 issue 必须一并回答的连锁问题。

## 候选关系清单（草案）

| 键 | 语义 | 方向 | 顺序 |
| --- | --- | --- | --- |
| `derived-from` / `spawns` | 引发（可分头处理） | 子→父 / 父→子 | 无关 |
| `part-of` / `decomposes-into` | 分解（母等子） | 子→父 / 父→子 | 相关（依赖） |
| `depends-on` / `blocked-by` | 依赖 | 依→被依 | 相关 |
| `relates-to` | 相关 | 双向 | 无关 |
| `supersedes` | 取代 | 新→旧 | — |
| （组合算子？） | 全部完成(and) / 任一完成(or) | — | — |

## 业内实践调研（collect）

> 一次联网调研的整理，来源 URL 见本节末尾。目的：看成熟系统怎么划分关系、怎么处理依赖与分解。

### 1. issue tracker 的关系类型：高度收敛为三族

| 族 | 代表实现（原文名） |
| --- | --- |
| **有向依赖** | Jira `blocks` / `is blocked by`；GitHub `blocked by` / `blocking`；Linear `blocks` / `blocked-by`；Azure Boards `Predecessor` / `Successor`；Redmine `blocks` / `blocked by`、`precedes` / `follows`；YouTrack `depends on` / `is required for` |
| **层级分解** | GitHub *sub-issues*（parent/child，可多级、父项带进度条）、tasklist 的 "tracked in"；Linear parent / sub-issue；Jira `is a child of`（sub-task）；Azure Boards `Parent` / `Child`；Redmine 子任务；YouTrack `Subtask` |
| **弱关联 / 去重** | Jira `relates to`、`duplicates` / `is duplicated by`、`clones`、`causes`；Linear `related` / `duplicate`；Azure Boards `Related`；Redmine `related` / `duplicates`；YouTrack `Relates` / `Duplicate` |

要点：① 依赖**有向**，且正反名成对；② 分解是**聚合**类关系，与依赖分开；③ kiln 把"引发 / 分解 / 依赖"混在 `derived-from` / `spawns` / `blocks` 里，**缺"分解"这一类**。

### 2. 排程视角：依赖类型，以及"分解 = AND"

- **CPM/PDM 四种依赖**：`Finish-to-Start (FS)`、`Start-to-Start (SS)`、`Finish-to-Finish (FF)`、`Start-to-Finish (SF)`，可带 lag。我们只用到 FS 这一档。
- **WBS 100% rule**（ISO 21511）：子层必须代表父层 100% 的工作 → **直接推出：分解的组合语义天然是 AND**（母问题要等**全部**子问题完成）。
- **AND-join / OR-join**（BPMN 术语）：AND-join = 等全部分支；OR / XOR-join = 任一（或活跃）分支即可。"全部完成 / 任一完成"在流程建模里有成熟词汇，不必自造。

### 3. 依赖的粒度：主流挂在"工作项"级，不做步骤级

- **MS Project**：依赖挂在最低层的 task 上；缩进生成的 **summary task 只做聚合、不建依赖**。PMI 明确指出"把两个摘要任务互连"是常见错误——那等于要求前者的**所有**子任务完成才能开始后者的**任何**子任务。
- **Asana**：`blocking` / `blocked by`，并支持 FF（子任务没完成则母任务不能完成）。
- **"动作 / 步骤级依赖"的成熟业界词表：未找到可靠来源**——实践停在 work item 级。

### 4. 工程实现

- **环检测**：有向图无环 = DFS 无 back edge；或 Tarjan 求强连通分量，**写入时拒绝成环边**。
- **传递闭包**：按需预计算（Warshall）换 O(1) 可达查询；是否要"间接阻塞"可见性决定算不算。
- **`blocks` 与 `depends-on` 是同一件事的两个视角**：YouTrack 的做法是**存单向边 + 自动生成反向名**（加了 `depends on`，对方自动挂 `is required for`）。

### 5. 对本设计最相关的 5 条

1. **关系分三族**：依赖 / 分解 / 弱关联。kiln 必须**另立"分解"类**（如 `part-of` / `sub-issue`），不能拿 `blocks` 顶替。
2. **"引发" ≠ "分解"**：引发无序、不阻塞；分解因 100% rule 天然是 **AND**——母问题等全部子问题。
3. **`blocks` 与 `depends-on` 是同一件事**：存单向键、系统自动生成反向名（YouTrack 模式），别让用户两头都写。
4. **依赖粒度挂 issue / 工作项级即可**：主流排程工具都明确"依赖只挂最底层项、摘要层不建依赖"，且**没有**步骤级依赖的成熟实践——这基本上**否掉了"action 级依赖"**。
5. **工程**：写入时用 DFS / Tarjan 检测环；传递闭包按需预计算。

> 这几条直接收窄了下面的「待探讨」：第 2 条回答了粒度问题（走 issue 级），第 3、4 条分别回答了类型划分与依赖方向。

> 来源：[Jira link types](https://support.atlassian.com/jira-software-cloud/docs/link-issues/) ｜ [GitHub issue dependencies](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/creating-issue-dependencies) ｜ [GitHub sub-issues](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/adding-sub-issues) ｜ [Linear parent & sub-issues](https://linear.app/docs/parent-and-sub-issues) ｜ [Azure Boards link types](https://learn.microsoft.com/en-gb/azure/devops/boards/queries/link-type-reference) ｜ [Redmine issues](https://www.redmine.org/projects/redmine/wiki/RedmineIssues/) ｜ [YouTrack link types](https://www.jetbrains.com/help/youtrack/server/link-types.html) ｜ [PMI Scheduling 101](https://www.pmi.org/learning/library/schedule-101-basic-best-practices-6701) ｜ [ISO 21511 (WBS)](https://www.iso.org/obp/ui/#iso:std:iso:21511:ed-1:v1:en) ｜ [BPMN Appendix C (AND-join)](https://www.omg.org/bpmn/Documents/BPMN_Appendix_C.pdf) ｜ [MS Project subtasks & summary tasks](https://support.microsoft.com/en-us/office/create-and-work-with-subtasks-and-summary-tasks-in-project-desktop-b3ff64ce-b121-42cc-905b-cb9b8ce0255f) ｜ [Asana dependency types](https://help.asana.com/s/article/dependency-types) ｜ [arXiv 2102.08485](https://arxiv.org/abs/2102.08485)

## 命名草案

分类已定：**三族**——引发（无序）、分解（有序，母等全部子）、依赖（有序，阻塞）；外加一个**无向**的弱关联。

命名规则：

- 每条**有向**关系有两个名字（正 / 反）。用户只需声明**任意一侧**，kiln 在读侧推导并**自动补全反向关系**（不要求两头都写）。
- **无向**关系（弱关联）只有一个名字，没有反向。
- 风格：kebab-case；贴合主流 tracker 的用词（Jira / GitHub / Linear 的原文名）；望文生义。

| 族 | 正名 | 反名 | 语义 | 现状 |
| --- | --- | --- | --- | --- |
| **引发**（无序） | `derived-from`（子→父） | `spawns`（父→子） | 由它引发，可分头处理 | 已有且在用 |
| **分解**（有序，AND） | `part-of`（子→父） | `has-part`（父→子） | 母问题拆成这些子问题；母要等**全部**子 | **新增** |
| **依赖**（有序） | `blocks`（阻塞者→被阻塞者） | `blocked-by`（被阻塞者→阻塞者） | 谁必须先完成 | 已有 `blocks`，补反名 |
| **弱关联**（无向） | `relates-to` | ——（同一名字，无反向） | 相关但不派生 | 已有 |
| **取代**（有向、弱） | `supersedes`（新→旧） | `superseded-by`（旧→新） | 新的取代旧的 | 已有 `supersedes`，补反名 |

**数的口径**：有向关系 **4 条 × 2 名 = 8 个名字**（引发 / 分解 / 依赖 / 取代），无向 **1 条 = 1 个名字**（弱关联），合计 **9 个名字**。若把"取代"并入弱关联家族，则族数仍是"三族 + 一个无向"。

### 待定的命名选择

**分解**是全新一族，名字有两个候选：

- `part-of` / `has-part` —— 最短最对称；`part-of` 是天然的声明方向
- `sub-issue-of` / `has-sub-issue` —— 贴 GitHub 现行术语，LLM 先验可能更强，但更长

**依赖**以哪一侧为正名：

- `blocks` / `blocked-by` —— Jira / Linear / GitHub 全用这套，且 kiln 已有 `blocks`
- `depends-on` / `required-by` —— YouTrack 风格，站在"等待方"视角写起来更顺（"B 依赖 A"）

**引发**的命名不够对称（`derived-from` 是 "from" 视角、`spawns` 是动作视角）。可改成 `spawned-by` / `spawns` 求对称，但 `derived-from` 已写进历史，**建议保持不动**。

## 待探讨

1. **"引发"与"分解"要不要分成两个类型？** 还是合并进 `derived-from`、再加一个 and/or 标记？
2. **依赖用 issue 级还是 action 级？**（见上表）
3. **若走 action 级，action 如何被寻址？**——用 commit hash（不稳定）？把 `object` 请回来？还是引入新的、与提交解耦的标识？
4. **组合语义（and/or）要不要显式表达？**
5. **与 `check` 的关系**：加了依赖后，要不要新增一致性规则——例如"依赖项还没到终点，就把母问题标成进行中"或直接告警？
6. **成本权衡**：词表膨胀 vs 语义精度。现有风格是"一键一类"，多加键最贴风格，但也最容易失控。

## 非目标

- 本 issue 只定**词汇与语义**，暂不改代码。
- 不重写已有提交：现有的 `derived-from` / `relates-to` 用法继续有效，新类型是**增量**。
