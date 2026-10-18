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
