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
