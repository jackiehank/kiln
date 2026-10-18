# issue: part-of — 分解族（part-of/has-part）的命名，与「母等全部子」语义靠什么落地

- **domain**：design
- **关联**：derived-from `relation-types`、relates-to `relation-vocabulary` / `silent-flag-drop`

## 背景

`relation-types` 定了三族，其中「**分解**」是新增的一族：母问题拆成若干子问题，**母要等全部子完成**（AND-join，由 WBS 100% rule 推得）。`relation-vocabulary` 随后把名字定稿为 `part-of` / `has-part`。

但有两件事没做完：

1. **命名**：`part-of` 是**本体论 / 需求工程**的术语；主流 tracker 用的是另一套（`sub-issue` / `parent-child` / `subtask`）。到底用哪套，先做 collect。
2. **语义传达**：`relation-vocabulary` 的加难版实验暴露一个缺口——**8/8 agent 拒绝从名字或一句简释里推出「母等全部子」**，困惑点一致落在 `part-of` 那一行。也就是：**这层语义，名字传不动**，得另找载体。

另外 `part-of` 至今**没有落地**（见 `silent-flag-drop`）。

## 要回答的问题

1. **命名**：业内（tracker / 需求工程 / 本体论）对「分解/层级」这一族一般怎么称呼？（本轮 collect）
2. **「母等全部子」靠什么传**：名字？释义/Skill？渲染（进度）？`check`/状态规则？还是像 Jira 那样交给"工作流配置"？
3. **与 `blocks` 的边界**：`A part-of B` 是否**蕴含** `A blocks B`（母等子 ⇒ 子阻塞母）？若蕴含，两者是同槽的强度档（`relation-revision` 已倾向如此），还是各存一条？
4. **要不要进度 / 100% rule**：母问题是否显示"X/Y 子完成"？`check` 要不要"子未到终点就拦母到终点"？
5. **落地**：`part-of` 何时进代码、`has-part` 何时能写。

## 非目标

- 不改机制（键 `(对,槽)`、槽内强度、取强）——归 `relation-revision`。
- 不重开 `relation-vocabulary` 已定的名字（除非本轮 collect 给出强反证）。
- 不在本 issue 里定渲染的**全部**规格——那归 `relation-rendering`（已 conclude）。

## 证据

- `relation-types`：分解 = AND-join（母等全部子），来源 WBS 100% rule（ISO 21511）。
- `relation-vocabulary` 加难版实验：8/8 agent 对「P3 与 P9 谁先完成」（part-of）答"无 / 无法确定"，且困惑点一致落在 `P3 part-of P9`；名字与简释都不携带"母等全部子"。
- `silent-flag-drop`：`part-of` 尚未落地。

## collect：「分解」这一族，业内怎么称呼、AND-join 靠什么表达（2026-10-18）

> 一轮联网调研。看两件事：① 业内对「分解 / 层级」这一族的**用词**；② 「母等全部子」（AND-join）在业内**靠什么承载**。

### 1. 结论先说

1. **命名**：主流 issue tracker 用 **parent/child** 或 **sub-issue/subtask** 命名这一族；**`part-of` 是本体论/需求工程术语**（meronymy，part–whole），**不是 tracker 惯用词**。
2. **「母等全部子」在业内是靠“工具行为 / 工作流”表达的，不是靠名字**——正好印证我们的实验（名字传不动它）。

### 2. 命名：两套语系

| 语系 | 代表 | 用词 |
| --- | --- | --- |
| **tracker** | GitHub / GitLab / Linear / Azure Boards / Redmine / YouTrack / Jira | `sub-issue(s)`、`parent` / `child`、`subtask of` / `parent for`、`is a child of` |
| **本体论** | OWL / BFO / RO / Schema.org / Dublin Core | `part_of` / `has_part`（meronymy，part–whole） |
| **需求工程** | SysML / RE | containment、composition（注意：`«deriveReqt»` 是“派生”不是“分解”） |

**→ 双方各站一边**：kiln 现在是本体论语系（`part-of`），而**所有 tracker 都是层级语系**。`relation-types` 当时已列两个候选（`part-of`/`has-part` vs `sub-issue-of`/`has-sub-issue`），未选。

### 3. AND-join 靠什么承载（关键）

| 工具 | 进度 / AND 的表达 | 谁承载 |
| --- | --- | --- |
| **GitHub sub-issues** | 父项可看进度；支持嵌套（最多 **8 层**、每父最多 **100** 子）；**默认不自动关闭父项**，社区用自定义 workflow（“Sub-Issue Closer”）在全部子项完成时关父项 | 渲染 + 自定义自动化 |
| **Jira** | 子任务原生；**默认可以带未完成子任务关闭父项**（JRASERVER-8936）；要“母等全部子”需在 workflow 的 Close 转换上加 **Sub-Task Blocking Condition** | 工作流配置 |
| **YouTrack** | 内置 **Subtask 工作流**：全部子项 resolved → 父项 Fixed；任一子项 reopen → 父项 reopen | 工作流（可逆） |
| **WBS（PM）** | **100% rule**：子层代表父层 100% 的工作 → AND | 方法论/标准 |

**→ 没有一家把这个语义放在关系名里**。要么**渲染**（进度条），要么**拦截 / 自动化**（工作流、校验）。

### 4. 本体论的额外提醒

- `part_of` 在本体论里通常声明为 **transitive**（OWL `owl:TransitiveProperty`；BFO `bfo:part_of`；RO `ro:part_of`），且要与 `member-of`（成员）、`contained-in`/`located-in`（空间包含）区分开。
- → 若 kiln 用 `part-of`，读者可能带进本体论先验：**传递、但不一定是“依赖/先后”**。这恰好解释了实验里 agent 不肯把它读成依赖。用 `sub-issue-of` / `parent-child` 则先验更贴 tracker。

### 5. 观察（不下结论）

- kiln 现在选的是**本体论语系的名字**（短、对称、含介词），但**语义却要靠 tracker 那套机制表达**（进度/拦截）——有点“名字与载体错位”。
- 若实体换成 `sub-issue-of`/`has-sub-issue`，**名字本身就会携带层级先验**（但仍不携带 AND）。
- AND-join 三个候选载体：① Skill 释义里写死“母等全部子”；② `check`/状态规则（仿 Jira 的 Sub-Task Blocking Condition）；③ 渲染进度（仿 GitHub）。三者可叠加。

### 6. 来源

- GitHub：[Adding sub-issues（含 8 层嵌套、100 子上限、parent/sub-issue）](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/adding-sub-issues)（一手）。
- GitHub 不自动关闭父项（社区 workflow 反证）：[gh-aw sub-issue-closer](https://github.com/github/gh-aw/blob/main/.github/workflows/sub-issue-closer.md)、[reinhardt-web PR #1900](https://github.com/kent8192/reinhardt-web/pull/1900)（二手/社区）。
- Jira：[How to prevent closing issues with open sub-tasks（Sub-Task Blocking Condition）](https://confluence.atlassian.com/jirakb/preventing-issues-from-being-closed-while-the-sub-tasks-are-still-open-724928276.html)（一手）｜ [JRASERVER-8936：默认能带未完成子任务关父项](https://jira.atlassian.com/browse/JRASERVER-8936)（一手）。
- YouTrack：[Subtasks workflow（全部子 resolved → 父 Fixed；子 reopen → 父 reopen）](https://www.jetbrains.com/help/youtrack/devportal/Workflow-Subtasks.html)（一手）。
- 本体论 `part_of`/`has_part` 与 BFO/RO/Schema.org/dcterms 对照：[Semantic relations 映射表](https://systems-analysis.ru/eng/Semantic_relations)（二手整理）；[W3C OntEngPatternsNotes（partOf 的各式 part-whole）](https://www.w3.org/wiki/index.php?title=OntEngPatternsNotes)（一手/社区）。
- WBS 100% rule：[ISO 21511](https://www.iso.org/obp/ui/#iso:std:iso:21511:ed-1:v1:en)（一手标准）。
- 前序：`relation-types` 的 collect（tracker 层级分解族对照表）；`relation-vocabulary` 加难版实验（`part-of` 语义未传达）。

## hypothesize：命名对比实验设计——`part-of`/`has-part` vs `sub-issue-of`/`has-sub-issue`

> 用户要求（2026-10-18）：设计一个**命名对比**实验。本节点只设计，不跑。定位：只测**名字**（不测 AND 载体）；AND 的载体是另一个决定。

### 0. 背景约束（决定实验该怎么设计）

- 前两轮命名实验（`derived-from` vs `causes`）**都天花板**——LLM 对两对名字都满分。
- collect 已表明：**名字不携带 AND**（业内都不把它放在名里）。
- → 本实验的可行性目标是 **① 先验（裸名字）判读差异** 与 **② 头对头偏好**，而不是“能不能推出依赖”。

### 1. 臂

| 臂 | 正名 | 反名 | 语系 |
| --- | --- | --- | --- |
| **P** | `part-of` | `has-part` | 本体论 / 需求工程 |
| **S** | `sub-issue-of` | `has-sub-issue` | tracker（GitHub 风） |

（可选第三臂 `parent-of`/`has-child`——但不是介词式，与“含介词”原则冲突，默认不加。）

### 2. 分两部分

- **Part 1（分臂理解）**：条件 **裸名字** 与 **带释义**，2 臂 × 2 条件 × 2 agent = **8 个 subagent**。
- **Part 2（头对头偏好）**：同时展示两套名字，请 agent 逐项选择更贴切的——**2 个 subagent**。

### 3. 题库

#### Part 1 — 裸名字（不给释义，测先验）

| # | 题 | 预期 |
| --- | --- | --- |
| ★B1 | 一条关系写作 `Q <正名> R`。Q 与 R 谁属于谁/谁包含谁？ | **Q 属于 R**（Q 是子项） |
| ★B2 | 一条关系写作 `Q <反名> R`。谁属于谁？ | **R 属于 Q** |
| B3 | 这条关系更接近哪一类？（结构包含 / 因果引发 / 先后依赖 / 弱关联） | **结构包含** |
| B4 | 这个关系是否意味着“R 必须等 Q 全部完成”？（是 / 否 / 不确定） | 预期两臂都 **否 / 不确定**（名字不携带 AND） |

#### Part 1 — 带释义（释义只写“是某 issue 的一部分”/“子项”，**不写 AND**）

| # | 题 | P 预期 | S 预期 |
| --- | --- | --- | --- |
| W1 | 新建 Q，Q 是已有 R 的一部分/子项 | `part-of: R` | `sub-issue-of: R` |
| W2 | 新建 Q，Q 包含已有 R（Q 是母） | `has-part: R` | `has-sub-issue: R` |
| ★D1 | 新建 Q，Q 由已有 R 引发（**非**包含） | `caused-by: R`（**不是** part-of） | `caused-by: R`（**不是** sub-issue-of） |
| ★D2 | 新建 Q，Q 必须等 R 完成、但 Q **不是** R 的一部分 | `blocked-by: R`（**不是** part-of） | `blocked-by: R`（**不是** sub-issue-of） |
| A1 | 只有 `X <正名> Y` 这一条关系：Y 能在 X 未完成时就当作完成吗？ | 否 / 不确定 | 否 / 不确定 |

#### Part 2 — 头对头偏好（两套名字都给）

对下面每个描述，选 A（`part-of`/`has-part`）或 B（`sub-issue-of`/`has-sub-issue`），各给一句理由：

1. Q 是 R **拆出来的一个子任务**
2. Q 是 R 的**组成部分**
3. 用词**最通用、最少歧义**
4. **最不容易与“依赖/阻塞”混淆**
5. **最不容易与“引发/派生”混淆**
6. 在 issue 追踪工具里**最自然**

### 4. 假设

| # | 假设 |
| --- | --- |
| **H1** | 裸名字条件下，S 臂先验方向判读（B1/B2）正确率 ≥ P 臂（tracker 词先验更强） |
| **H2** | 两臂都**不能**从名字推出 AND（B4/A1 均否/不确定）——**确认名字不携带 AND** |
| **H3** | 消歧题 D1/D2 两臂误用率无显著差异 |
| **H4** | Part 2 中 agent **一致**偏好某一套（若一致 → 强信号；若分歧 → 无偏好） |

### 5. 预登记判据

1. **主判据**：★ 先验题（B1/B2/D1/D2）正确率高者胜；**差 ≤1 = 平手**。
2. **AND 确认**：若 B4 与 A1 两臂均非“是”→ 采信“**名字不携带 AND**”，作为独立结论（与 collect 一致）。
3. **偏好判据**：Part 2 若 ≥75% 题项指向同一方案 → 记为“有偏好”；否则“无偏好”。
4. **天花板检查**：若 ★ 题两臂均 ≥90% **且** Part 2 无偏好 → **判无区分力**，不选边；命名改按**设计偏好**定（本体论：短、对称、可传递；tracker：先验准、贴主流）。

### 6. 样本与隔离

- 2 臂 × 2 条件 × 2 agent + 2 偏好 = **10 个 subagent**。
- 隔离按 `relation-rendering` 的配方（`--no-tools` + 中性目录 + `-nc -ns -np -na` + `--no-session`）；**先跑关键词探针**。
- Part 2 的 prompt 不暴露 P/S 哪一套是现状（标注为 A/B，并随机正反排一次以消位置偏）。

### 7. 预期与陷阱

- **最可能的结果**：与上两轮一样天花板（两臂先验都高）——那就直接按设计偏好定。
- **真正可能拉开的点**：B1/B2（裸名字方向）与 Part 2 的“最通用/最不容易歧义”。
- **陷阱**：`part-of` 在本体论里是 **transitive**——agent 可能由此把“子-母-祖”链推成“传递依赖”，反而误判 A1。
- **已知局限**：同源模型、样本小；只能作方向性参考。

### 结论（实验方案，待执行）

- 10 个 subagent，分“分臂理解”与“头对头偏好”两部分；预登记含 AND 确认题与天花板检查。
- 下一个动作：`test`。

## test：命名对比实验结果（10 agent，2026-10-18 执行）

> 按上节方案执行。隔离探针先跑过（无泄漏）。

### 1. Part 1 — 裸名字（B1–B4）

| agent | 臂 | B1 谁属于谁 | B2 谁属于谁 | B3 哪一类 | B4 母是否等子 |
| --- | --- | --- | --- | --- | --- |
| P_bare.1 | P | Q∈R ✓ | R∈Q ✓ | 结构包含 ✓ | **是 ✗** |
| P_bare.2 | P | ✓ | ✓ | ✓ | 否 ✓ |
| S_bare.1 | S | ✓ | ✓ | ✓ | 否 ✓ |
| S_bare.2 | S | ✓ | ✓ | ✓ | 否 ✓ |

→ 先验方向/归类：**两臂均 3/3 全对**（Q∈R、R∈Q、结构包含）。

### 2. Part 1 — 带释义（W1–W2 / D1–D2 / A1）

| agent | 臂 | W1 | W2 | D1 引发 | D2 依赖 | A1 母是否等子 |
| --- | --- | --- | --- | --- | --- | --- |
| P_gloss.1 | P | `part-of: R` ✓ | `has-part: R` ✓ | `caused-by: R` ✓ | `blocked-by: R` ✓ | 否 ✓ |
| P_gloss.2 | P | ✓ | ✓ | ✓ | ✓ | **是 ✗** |
| S_gloss.1 | S | `sub-issue-of: R` ✓ | `has-sub-issue: R` ✓ | `caused-by: R` ✓ | `blocked-by: R` ✓ | **是 ✗** |
| S_gloss.2 | S | ✓ | ✓ | ✓ | ✓ | **是 ✗** |

→ 写侧与**消歧题（D1/D2）两臂均 4/4、全对**：没有人把“引发”或“依赖”误写成 `part-of`/`sub-issue-of`。

### 3. ★ 主判据（B1/B2/D1/D2）

**两臂、两条件、全部 agent 均 4/4。→ 主判据：平手。**

### 4. 意外：AND 语义的结果与预期**相反**，且随条件翻转

| | 裸名字（B4） | 带释义（A1） |
| --- | --- | --- |
| P 臂答“是” | 1/2 | 1/2 |
| S 臂答“是” | 0/2 | **2/2** |
| **合计“是”** | **1/4** | **3/4** |

- 预登记预期：两臂都答“否/不确定”。
- 实际：**裸条件下 3/4 答“否”**（与预期一致）；**带释义条件下 3/4 答“是”**（与预期相反）。
- → 不能干净地采信“名字不携带 AND”。更准确的表述是：**名字单独出现时不携带 AND；但一旦处于“结构化关系词表”的语境（带释义），agent 反而倾向推出“母等全部子”**。
- 另一条：我的报告里之前那句“8/8 agent 拒绍推断”**需收窄**——那是**问“谁先完成”（排序）**的措辞；本实验问“母是否等子全部完成”（完成条件），问法不同结果就不同。

### 5. Part 2 — 头对头偏好（两 agent，A/B 已互换位序）

两个 agent 对 A/B 互换后**逐项答案一致**（无位置偏）：

| # | 描述 | 胜出 |
| --- | --- | --- |
| 1 | Q 是 R 拆出来的子任务 | **`sub-issue-of`** |
| 2 | Q 是 R 的组成部分 | **`part-of`** |
| 3 | 最通用、最少歧义 | **`part-of`** |
| 4 | 最不易与“依赖/阻塞”混淆 | **`sub-issue-of`** |
| 5 | 最不易与“引发/派生”混淆 | **`part-of`** |
| 6 | 在 issue 工具里最自然 | **`sub-issue-of`** |
| 7 | 只能保留一套 | **`sub-issue-of`** |

合计：**`sub-issue-of` 4 项 / `part-of` 3 项**（=57%<75%）→ 预登记判为“**无偏好**”。

但两条定性理由值得记下（两 agent 一致）：

- `part-of` **更易被读成依赖**：H2H_b.1 原话“sub-issue-of 明示父子层级，不易像 part-of 那样被读成‘完成 R 需要 Q’的依赖”。→ 正好命中预设陷阱（`part_of` 的传递先验）。
- `part-of` **更通用**（跨领域部分-整体词），但不够贴 tracker。

### 6. 预登记判据裁定

1. 主判据：**平手**。
2. AND 确认：**未成立**（结果翻转，见 §4）——作为“需收窄的结论”记下。
3. 偏好：4/7=57% <75% → **无偏好**。
4. **天花板检查触发**（★ 两臂均 100%，Part 2 无偏好）→ **判无区分力、不选边**。

→ **与前两轮命名实验一致：名字在正确性上分不出高下。**

### 7. 真发现（可用的信号）

1. **正确性无关命名**：两套名字在理解与写侧均满分（第三轮了）。
2. **`sub-issue-of` 在“强制二选一”上两票全取**，且在“最不易与依赖混淆”“工具最自然”两项胜出——与 collect 的“所有 tracker 都用层级词”一致。
3. **`part-of` 反而更易被读成依赖**——这解释了它在加难版实验里被当依赖/不当依赖的两面性，且与本体论 transitive 先验有关。
4. **AND 随语境翻转**（§4）——这是本轮最意料之外的发现，值得单独跟。

### 8. 结论（`test`）

- 按预登记：**无区分力，不靠本实验选名**；命名改按设计偏好定。
- 偏好信号（非判决）：`sub-issue-of`/`has-sub-issue` 在 tracker 语境、与依赖消歧、以及强制二选一上占优；`part-of`/`has-part` 在通用性、与引发消歧、短对称上占优。
- **AND 载体的结论要改**：不能再说“名字完全不携带 AND”——它在带释义时会浮现，但不可靠（仍不能当语义保证）→ 仍需显式载体（Skill/`check`/进度）。
- 下一个动作：`hypothesize`——定「名字 + AND 载体」方案（可参 collect 三家做法）。
