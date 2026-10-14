# issue: relation-vocabulary — 9 个关系会不会太多、够不够清晰、Agent 好不好读

- **状态**：ask（进行中）
- **domain**：design
- **关联**：derived-from `relation-types`

## 背景

`relation-types` 刚定下 **9 个关系名**（4 条有向关系各两名 + 1 条无向）。三个疑问待查：① 数量会不会太多；② 语义是否彼此清晰、不重叠；③ 对 Agent（LLM）的可读性如何。

## 一、9 个会不会太多？先拆口径

- 9 是**名字**数，不是**关系**数。实际只有 **5 条关系**：引发 / 分解 / 依赖 / 弱关联 / 取代。
- 其中 **3 条是核心**（引发、分解、依赖），2 条是弱关系（关联、取代）。
- 参照：**Jira 默认 link types 也是 9 个名字**（`blocks` / `is blocked by`、`clones` / `is cloned by`、`duplicates` / `is duplicated by`、`causes` / `is caused by`、`relates to`）——9 正落在主流 tracker 里最富集的那一档，**不算超标**。
- 所以真正该问的不是"9 算不算多"，而是"这 **5 条**关系，Agent 分得清吗"。

## 二、语义是否清晰？三处重叠风险

1. **引发 vs 分解（最大风险）**：都在说"A 来自 B"，区别只在**要不要等**。
   - 分解 = 母**要等全部子**（AND）→ `part-of`
   - 引发 = **不等**、可分头处理 → `derived-from`
   Agent 很可能随手选错。
   **消歧句**：*"母问题是不是必须等这些子问题做完才能继续？" 是 → 分解；否 → 引发。*
2. **分解 vs 依赖**：都"有序"。分解边天然包含一个等待（母等子），**看起来也像依赖**。
   **消歧句**：*依赖是两个独立 issue 的先后（`blocks`）；分解是一个 issue 被拆成若干部分（`part-of`，且满足 100% rule）。*
3. **`relates-to` 变垃圾桶**：一旦分不清，Agent 会一律写 `relates-to`，弱关联成了默认值，整张图就失去信息量。
   **消歧句**：*先试"引发 / 分解 / 依赖"，都不成立才用 `relates-to`。*

## 三、Agent 可读性

- 9 个名字都由极常见的英文词构成（derived / from、part-of、blocks、relates-to、supersedes…），训练语料里都高频出现，**整体不存在"生僻词"问题**。
- 但有两个**先验偏弱**的名字：
  - `spawns` —— 不是 tracker 术语（更像游戏 / 生物词），Jira / GitHub / Linear 都不用
  - `has-part` —— 主流 tracker 用的是 parent / child / sub-issue，没有人用 "has-part"
- 巧合的是：这两个弱先验名**恰好都是反向名**（父→子方向）。

## 四、一个可能有效的收窄：把 9 拆成「可写 5 + 只读 4」

既然 kiln 是**读侧推导反向**，那反向名其实**根本不需要用户 / Agent 去写**——它们只在读 kiln 输出时出现。于是可以规定：

- **可写名（5 个，供声明）**：`derived-from` / `part-of` / `blocks` / `relates-to` / `supersedes`
- **只读名（4 个，kiln 推导出的反向）**：`spawns` / `has-part` / `blocked-by` / `superseded-by`

好处：Agent 需要**掌握并书写**的从 9 降到 **5**，且这 5 个都是主流熟悉的词；两个弱先验名（`spawns`、`has-part`）退到"只读"，Agent 不必主动产出。

代价：如果 Agent 就是想从父方视角写（"这个父问题拆出了 X"），它得把 `part-of` 写在**子**那条 issue 上——多一步反身思考。

## 待探讨

1. 采纳"**可写 5 + 只读 4**"吗？还是坚持双侧皆可写？
2. 要不要把上面的**消歧句**写进 Agent 的使用约定（Skill）？——这是"提示词税收"的老问题：规则写清楚要花 token，不写 Agent 就会选错。
3. `relates-to` 要不要加一条"禁止滥用"的 `check` 提示？
4. 数量还有压缩空间吗？比如把"取代"并入弱关联（那就变成 4 条关系 / 8 个名字）。
5. 名字要不要再筛一轮？`spawns` 是否换成更 tracker 味的词（如 `spawned` / `branches`）？

## 非目标

- 只讨论**词表规模与清晰度**，不推翻三族分类本身（`relation-types` 已 conclude）。
- 不改代码、不迁移历史。
