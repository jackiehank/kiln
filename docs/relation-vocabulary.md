# issue: relation-vocabulary — 9 个关系会不会太多、够不够清晰、Agent 好不好读

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

## 决策树（草案）

**只写"可写名"，按顺序问下来，第一个"是"就是答案**（括号里是写在**哪一条** issue 上）：

1. 这两条是「A 由 B **拆解**而来」吗？（母问题必须等**全部**子问题完成才继续；满足 100% rule）
   是 → **`part-of`**（写在**子** issue 上）
2. 其中一条**必须等**另一条完成才能开始/继续吗？
   是 → **`blocks`**（写在**阻塞者**上）
3. 一条是另一条**引发**的，但两者可各自处理、**无先后**吗？
   是 → **`derived-from`**（写在**后出现**的那条上）
4. 新的结论 / 方案**取代**了旧的？
   是 → **`supersedes`**（写在**新** issue 上）
5. 以上都不成立，只是**相关** → **`relates-to`**

顺序即优先级：**先判"拆解"，再判"等待"，最后才退到"引发"与"相关"**——这正是三处重叠风险的解药。

## 命名替换候选

| 现名 | 方向 | 读写 | 候选替换 | 理由 |
| --- | --- | --- | --- | --- |
| `spawns` | 父→子 | 只读 | `branches-into` / `derived` / `spawned` | `spawns` 不是 tracker 术语 |
| `has-part` | 父→子 | 只读 | `has-sub-issue` / `contains` / `parts` | 主流用的是 sub-issue / parent-child |
| `blocked-by` | 被阻塞→阻塞 | 只读 | （保留） | Jira/Linear/GitHub 通用 |
| `superseded-by` | 旧→新 | 只读 | （保留） | 与 `supersedes` 机械对称，好推 |

注意：这四个都是**只读名**（由 kiln 推导，Agent 不写），所以替换成本主要落在"读起来的清晰度"上。

## 实验设计：用 subagent 代理"用户"

**要验证两件事**：

- **A. 决策树到底有没有用？**（值不值得写进 Skill、花掉那点 token）
- **B. 名字好不好读？**（含上表的替换候选）

**方法**：以 subagent 作为代理用户，分两臂、并行跑：

- **对照组**：只给 5 个可写名的名字 + 一句话释义（模拟一份"最小 Skill"）
- **实验组**：同样内容 **+ 上面的决策树**

两臂各 2 个独立 subagent，回答同一套题：

- **场景题（12 道）**：给一段自然语言描述，问"该写哪个键、写在哪条 issue 上"
- **命名题（9 个名字 + 4 个候选）**：给出名字，要求一句话解释含义并自评困惑度

**指标**：场景选对率（分臂、分题型）、命名解释正确率、Agent 自报的困惑点。

**已知局限**：subagent 与本体同源（同一模型族），**不等于**多样本用户；结果只能作方向性参考，不能当定论。

### 场景题库（含预期答案，供复核）

| # | 场景 | 预期键 |
| --- | --- | --- |
| 1 | 把「登录模块重构」拆成会话存储 / 鉴权中间件 / 错误处理三块，三块都做完才能继续推进重构 | `part-of` |
| 2 | 读一篇论文引出一个**完全独立**的疑问「缓存淘汰能不能用 LFU」，两边可各自研究 | `derived-from` |
| 3 | 「数据库迁移」必须先完成，才能开始「数据回填」 | `blocks` |
| 4 | 新方案「改用事件驱动」取代了旧的「轮询方案」 | `supersedes` |
| 5 | 「性能调优」和「日志瘦身」会碰同一段代码，但互不阻塞、也非谁引发 | `relates-to` |
| 6 | 母问题「架构评估」拆出四个子问题，四个都得出结论才能给总评 | `part-of` |
| 7 | 做「权限模型」时顺带发现一个独立小问题「Token 刷新竞态」，它不影响权限模型推进 | `derived-from` |
| 8 | 「依赖升级」不做完，「灰度发布」就上不了 | `blocks` |
| 9 | 旧 issue「V1 方案」已被「V2 方案」取代 | `supersedes` |
| 10 | 两个 issue 涉及同一份配置，但既非拆分、非阻塞、非引发 | `relates-to` |
| 11 | 「API 设计」拆成「错误码规范」和「分页规范」，两个都做完才算 API 设计完成 | `part-of` |
| 12 | 要跑「压测」，先得把「测试环境搭建」弄好——两件**独立**的事 | `blocks` |

（#7 是「引发 vs 分解」的陷阱题， #12 是「依赖 vs 分解」的陷阱题。）

## 实验结果（subagent 代理用户）

**样本**：4 个独立 subagent，2 臂 × 2（**对照** = 只给 5 个键 + 一句话释义；**实验** = 再加决策树），同一套题。

### A. 场景题：4/4 全对，两臂**没有任何差异**

| 组别 | 12 题正确数 |
| --- | --- |
| 对照 C1 / C2 | 12 / 12 |
| 实验 T1 / T2 | 12 / 12 |

连两道陷阱题（#7 引发 vs 分解、#12 依赖 vs 分解）都全对。

**结论：决策树没有边际价值。** 但要注意口径——我给的"一句话释义"里**本身就含判据词**（"两者无先后、可各自处理" / "母问题要等**全部**子部分完成"）。所以准确表述是：**起作用的是释义里的判据词，不是决策树**；决策树只是同一信息换了个排布。→ **决策树不必写进 Skill，省掉那点 token。**

### B. 命名题：两个弱名被数据点名

"一眼看懂"平均分（4 个 agent）：

| 键 | 均分 | 备注 |
| --- | --- | --- |
| `blocks` / `blocked-by` / `relates-to` | **5.0** | 全票满分 |
| `part-of` | 4.5 | |
| `derived-from` / `superseded-by` | 4.25 | |
| `supersedes` | 4.0 | |
| **`has-part`** | **3.0** | 次差 |
| **`spawns`** | **2.25** | 最差 |

预判的两个弱先验名被完全证实，而且它们恰好都是**只读反向名**。

替换选择**四票一致**：

- `spawns` → **`branches-into`**（4/4）
- `has-part` → **`has-sub-issue`**（4/4）

### C. 自由反馈：一个被 4/4 独立提到的点

四位 agent 都自发指出：**只读反向名的"方向"难记**——谁正谁反、谁是只读，靠猜。

> "spawns 与 derived-from 是同一关系的两端，父→子还是子→父容易记反"
> "四个只读反向名的方向靠猜，读的人常分不清正反"

其余零星反馈集中在 `derived-from` 与 `blocks` / `relates-to` 的界线（"必须先有 A 才有 B" 到底算引发还是阻塞）。

### 由数据得出的三条

1. **决策树不写进 Skill**：一句话释义（含判据词）已经够用。
2. **`spawns` → `branches-into`、`has-part` → `has-sub-issue`**：四票一致，直接采纳。
3. **只读反向名的方向问题要单独解**：它们由 kiln 推导，与其让人记方向，不如给一条**机械可预测**的规则（如统一 `-by` 后缀），或干脆**不把它们当"名字"暴露**，只在渲染时用带箭头的固定句式。

### 局限（必须记住）

- subagent 与本体**同源**（同一模型族），不是多样本用户；
- 每臂只有 2 个样本，且两臂的差异本就不该出现——**这个实验能证明"决策树多余"，但证明不了"名字对更弱的模型也够清楚"**；
- 释义里已含判据词，所以**"裸名字"条件下 Agent 会不会选错，本实验没有覆盖**。

## 实验二：取消反向名、改用箭头（test）

**假设**：实验一里 4/4 都抱怨"反向名方向难记"，那就干脆**不给反向命名**——每条有向关系只留一个名字，方向由**箭头**表示（`A ──键──▶ B`）。

**样本**：4 个 subagent，2 臂 × 2，同一批 8 行输出（其实是 **4 条边各写两遍**：前 4 行从**正向**侧声明，后 4 行从**反向**侧声明）：

- **R 臂（名字）**：反向声明用反向名 —— `登录重构: has-sub-issue 会话存储`、`数据回填: blocked-by 数据库迁移`…
- **A 臂（箭头）**：只用正向名 + 箭头 —— `会话存储 ──part-of──▶ 登录重构`…

### 结果

| 指标 | R 臂（名字） | A 臂（箭头） |
| --- | --- | --- |
| 方向判读正确 | 8/8、8/8（全对） | 8/8、8/8（全对） |
| 一眼看懂均分（全 16 行） | **4.13** | **4.63** |
| 一眼看懂均分（**反向声明**的那 8 行） | **4.00** | **4.63** |

**准确率两臂都满分**——给了对照表之后，名字和箭头都读得对。差别在**清晰度**：箭头更高，尤其在"从反向侧声明"的那些行上（4.63 vs 4.00）。

### 最有价值的意外：反向名被实证为"冗余"

**A 臂 2/2 都自发指出**：8 行里后 4 行与前 4 行"逐字重复"——**8 条声明实际只有 4 条不同的渲染**。

> "第 5–8 行与第 1–4 行是完全逐字重复的同一批边……8 行实际只有 4 条不同的关系"

这就是"反向名冗余"的直接证据：同一条边无论从哪侧声明，箭头渲染只有一形态。→ **反向名可以整个去掉，而不是换名字。**

### 遗留热点

- **`derived-from` 是唯一的争议名**（R 臂 2/2、A 臂 1/2 点名它）：它读起来像"来源 → 衍生"，而约定方向是"被引发者 → 来源"，反直觉（箭头能钉死方向，但名字本身仍别扭）。
- 次热点：`part-of`（A 臂 1/2 觉得"子→母"与"母→子"的直觉相反）。

### 由实验二得出的四条

1. **取消反向名**：词表从 **9 个名字收到 5 个**（`derived-from` / `part-of` / `blocks` / `relates-to` / `supersedes`）。
2. **"可写 5 + 只读 4"这个问题随之消失**——因为没有"只读名"了：5 个名字都既能**写**（各有固定方向约定）又能**读**（箭头给方向）。
3. **实验一的"替换弱名"也不再必要**：`spawns` / `has-part` 直接**不复存在**，而不是换成 `branches-into` / `has-sub-issue`。
4. 遗留：`derived-from` 的方向约定必须在 Skill 里**显式写明**，或再议名字。

## `derived-from` 的替换候选与第三轮测试

`derived-from` 被 3/4 个 agent 点名：它读起来像"来源 → 衍生"，而约定方向是"**被引发者 → 来源**"。**方向已被箭头钉死之后，"名字的默认读法"就成了唯一的歧义来源。**

### 候选（6 个）

| 候选 | 字面读法 | 方向是否顺 | 备注 |
| --- | --- | --- | --- |
| `derived-from` | "X 由 Y 派生" | 顺 | 现名，但被指认为**易反读** |
| `inspired-by` | "X 受 Y 启发" | 顺 | 常见、语气轻（不暗示依赖） |
| `stems-from` | "X 源自 Y" | 顺 | 方向感强 |
| `based-on` | "X 基于 Y" | 顺 | 最熟悉，但语义偏强（像依赖） |
| `spawned-by` | "X 由 Y 生出" | 顺 | 与 `spawns` 机械配对；但 `spawn` 先验弱 |
| `origin` | "X 的来源是 Y" | 顺 | 名词式，最直白；与动词式键不同风格 |

### 测试设计（第三轮：裸名字）

这轮**刻意不给任何方向约定**——只给语义（"两者无先后、可各自处理，一条由另一条引发"），然后问：

> 看到 `X: <候选> Y`（X 写了这个键、指向 Y），你**默认**认为 **X 是引发者，还是被引发者**？

正确答案是"**X 是被引发者**"（约定方向：被引发者 → 来源）。**若某个候选让人答成"X 是引发者"，就说明它的默认读法与约定相反，是坏名字。**

这一轮同时补上了此前一直没做的那档：**"裸名字"条件下 Agent 的默认直觉**（实验一/二都给了解释，没覆盖这个真实失败模式）。

**样本**：2 个独立 agent；每题再给"这个名字一眼看懂"评分（1–5）；最后一题请它们各自选一个总冠军。

### 第三轮结果（裸名字）：`derived-from` 其实**没坏**

2 个 agent，**均未给方向约定**，仅凭名字作答（正确答案 = `X=被引发者`）：

| 候选 | 方向判读（2 个 agent） | 一眼看懂 |
| --- | --- | --- |
| `derived-from` | ✓ ✓ | **5 / 5** |
| `inspired-by` | ✓ ✓ | 5 / 5 |
| `spawned-by` | ✓ ✓ | 5 / 5 |
| `stems-from` | ✓ ✓ | 4 / 4 |
| `based-on` | ✓ ✓ | 4 / 4 |
| `origin` | **说不准 / 说不准** | **2 / 2** |

**结论：`derived-from` 并不坏。** 两个 agent 在**没有任何方向约定**的情况下都读对了，且清晰度满分；其中一个还把它选为总冠军，理由是"英文里表达来源方向最标准、最无歧义的说法"。

这**推翻了"`derived-from` 不好"的印象**——那个印象来自**实验二的箭头呈现**（agent 看到 `缓存-LFU ──derived-from──▶ 论文笔记` 时，预期箭头应从来源指出去），而不是名字本身的读法。在"裸名字"这个**真实失败模式**下，它读得又快又准。

**唯一失败的是 `origin`**：名词式、不含介词，方向只能靠猜。

**由此得出一条命名原则**：**有向关系的键名必须含介词**（`from` / `by` / `of` / `on`）——`derived-from`、`inspired-by`、`spawned-by`、`based-on`、`part-of`、`blocked-by` 都自带方向；`origin` 这类"光杆名词"方向全靠猜。

**处置建议：`derived-from` 保持不动**（读得对、清晰度满分、且已写进历史，零迁移成本）。

## 第四轮：`relates-to` 加不加提示（对照）

**假设**：`relates-to` 最容易被当垃圾桶，所以加一句提示（"它是**最后手段**，先试其他四个"）能降低误用。

**样本**：4 个 agent，2 臂 × 2，12 个情境（4 个正确答案是 `relates-to`： #1 / #4 / #8 / #11；其余 8 个是干扰项）。

| 组别 | 正确数 |
| --- | --- |
| 不加提示 ×2 | **11 / 12、11 / 12** |
| 加提示 ×2 | **11 / 12、11 / 12** |

**结论一：提示没有任何边际价值**——两臂一模一样，4 个 agent 错的是同一道题（#8）。→ **这条提示不写**，省掉 token。（与实验一同一个结论：释义里的判据词已经足够。）

**结论二：`relates-to` 并没有被滥用。** 在有 8 个干扰项的情况下，两臂都**正确**判出了它们该用 `derived-from` / `part-of` / `blocks` / `supersedes`——"垃圾桶"风险在"给了释义"的前提下**不成立**。

### 唯一的失败，恰好是反方向

4/4 个 agent 在 **#8** 上都答成了 `derived-from`（正确答案是 `relates-to`）：

> #8：两个 issue 由同一次线上故障的记录**引出**，但两条各自独立、无先后

原因很清楚：**场景里写了"引出"这个词**，字面就在暗示"引发"。**这是场景设计的锅，不是词表的问题**——4/4 全踩同一个坑，恰恰说明措辞本身有歧义。

但它顺带暴露了一个**真实缺口**：当两个 issue 有**共同来源、而那个来源不是一个 issue**（如一次线上故障）时，**没有任何键能表达"同源"**，agent 只好硬编一条兄弟间的 `derived-from`。这个缺口值得记下来。

## 结论（已定）

**四轮代理用户测试收敛成一套极简词表；规则不必写进 Skill——释义本身就是规则。**

1. **词表（草案，未定稿）5 个名字**：`derived-from` / `part-of` / `blocks` / `relates-to` / `supersedes`。（**2026-10-18**：此“定稿”已降为**待定**——具体名字仍有多项悬而未决，见文末「命名待决」。）
   - **取消反向名**（实验二）：同一条边无论从哪侧声明，箭头渲染只有一种形态；A 臂 2/2 自发发现"8 条声明实际只有 4 条渲染"。（**2026-10-18 修订**：此条限定在**渲染层**；写侧依「主语一致」原则需正反两名——见文末「命名原则（补）」。）
   - **方向改由箭头渲染**：写侧每个名字有固定方向约定，读侧一律 `A ──键──▶ B`。
2. **`derived-from` 保留**（第三轮裸名字测试：2/2 读对、清晰度 5/5，且零迁移成本）。原先"它不好"的印象来自实验二的**箭头语境**，不是名字本身。
3. **命名原则：有向关系的键名必须含介词**（`from` / `by` / `of` / `on`）。反例 `origin`（光杆名词）2/2 答"说不准"。
4. **不加决策树、不加 `relates-to` 提示**（实验一 12/12 vs 12/12；第四轮 11/12 vs 11/12）。**释义里的判据词就是规则本身**，再加规则不产生边际收益——所以 **Skill 只需要"5 个键 + 一句话释义"**。
5. **作废的待办**："可写 5 + 只读 4"的拆分、"替换 `spawns` / `has-part`"——反向名取消后这两个问题都不存在了。

**遗留**（另开 issue）：两个 issue **同源**、而来源不是 issue 时（如共同源自一次线上故障），没有任何键能表达——见 `common-origin`。

**非目标**：本 issue 只定**词表与渲染规格**，不改代码；实现走 [docs/relation-types.md](relation-types.md) 的落地项。

## 待探讨

1. 采纳"**可写 5 + 只读 4**"吗？还是坚持双侧皆可写？
2. 要不要把上面的**消歧句**写进 Agent 的使用约定（Skill）？——这是"提示词税收"的老问题：规则写清楚要花 token，不写 Agent 就会选错。
3. `relates-to` 要不要加一条"禁止滥用"的 `check` 提示？
4. 数量还有压缩空间吗？比如把"取代"并入弱关联（那就变成 4 条关系 / 8 个名字）。
5. 名字要不要再筛一轮？`spawns` 是否换成更 tracker 味的词（如 `spawned` / `branches`）？

## 非目标

- 只讨论**词表规模与清晰度**，不推翻三族分类本身（`relation-types` 已 conclude）。
- 不改代码、不迁移历史。

## 命名原则（补）：主语一致——名字的主语恒为声明方

> 来源：`relation-revision` 的写侧方向推演（2026-10-18）。本条是**命名原则**，接在「有向关系的键名必须含介词」之后，并**修订**上面结论 #1 的「取消反向名」。属于 `hypothesize`（推演所得，尚无实测）。

### 原则

**每条关系名的主语，恒为声明方**——即这条 trailer 写在谁的提交里，谁就是主语。

- 声明方通常就是**新 issue**（写关系一般发生在第二个 issue 出现、指向已有 issue 时），但**它不等于语义方向**：`derived-from` / `part-of` / `supersedes` 的语义天然随时间（来源/母题/旧版更早），而 `blocks` 不是——阻塞者的创建时间与阻塞方向无关（B 可先建，也可在 A 之后才新建去阻塞 A）。所以规则是「声明方→被引用方」，不是「后→先」。
- **语态只是副产品**：`supersedes` 主动、`derived-from` 被动，但主语都是声明方。
- 主名要按「声明方在语义里扮哪个角色」选；这正好回答 `relation-types` 的遗留「依赖以哪一侧为正名」：**等待方视角**（`depends-on` / `blocked-by`），因为写关系时手通常在等待的那个新 issue 上。

### 推论 1：写侧需正反两名（9 个），读侧仍只渲染一种箭头

实验二观测到的「反向名冗余」是**渲染层**的事实，对；但**写侧**若只有 5 个名，声明方处在反向角色时**写不出来**：

> 新建 A，发现它被已有 B 阻塞——只有 `blocks` 就只能去改 B 的提交；应能写 `A blocked-by B`。

→ **写 9 / 渲 1**：写侧每个非对称关系保留正反两名（按角色选），读侧只渲染一种箭头（目标视角用反名渲染，不存第二份）。

| 语义 | 声明方 = 等待/子/被引发/新（**主名**） | 声明方 = 阻塞/母/来源/旧（反名） |
| --- | --- | --- |
| 引发 | `derived-from` | `spawns` |
| 分解 | `part-of` | `has-part` |
| 依赖 | `depends-on` / `blocked-by` | `blocks` |
| 取代 | `supersedes` | `superseded-by` |
| 关联 | `relates-to`（对称，无正反） | — |

### 推论 2：机制不受影响（与 `relation-revision` 的接口）

`relation-revision` 把关系的键定为 `(无序对, 槽)`：正反两名映射到**同一槽同一 rank** → 写哪个名都不会产生重复边，取强 / 覆盖也不受影响。反向名只是**读侧换名渲染**。

### 本条修订 / 回答了什么

- **修订结论 #1**：「取消反向名」限定在**渲染层**（读侧）；写侧恢复正反两名。
- **回答 `relation-types` 的遗留**：依赖族主名取等待方视角（`depends-on` / `blocked-by`）。
- 与「有向关系的键名必须含介词」不冲突：`derived-from` / `part-of` / `blocked-by` 都既含介词、又以声明方为主语。

### 待办

- 若采纳，`relation-types` 的「9 个名字」应恢复为**写侧 9 名（5 主名 + 4 反名）+ 读侧 1 种渲染**；本 issue 的「词表定稿 5 个名字」相应改为「**主名 5 个**」。
- 需用同样的 subagent 代理实验验证：**主语一致**是否降低 Agent 的写侧方向错误率（当前只有推演，无实测）。

## hypothesize：命名尚未定稿——待决清单

> 用户指出（2026-10-18）：本 issue 里**关系到底叫什么还没确定**。先前那句「词表定稿 5 个名字」已被后续推演部分推翻，且具体名字仍有多项悬而未决。本节点只**记录待决项**，不下结论。

### 1. 为什么不能算定稿

1. **文档自相矛盾**：结论 #1 写「取消反向名」，同文末补遗又写「写侧需正反两名（9 个）」。
2. **依赖族主名未定**：`blocks`（Jira/Linear/GitHub 主流，且已写进历史）vs `blocked-by` / `depends-on`（等待方视角，贴「主语=声明方」）。
3. **被删过的名要请回来，且没重评**：`spawns` / `has-part` 在实验二被整体取消，现在写侧又需要它们；名字本身（`spawns` 是否改 `spawned-by` / `branches-into`，`has-part` 是否改 `has-sub-issue`）未重新评估。
4. **分解一族的名未定**：`part-of` / `has-part` vs `sub-issue-of` / `has-sub-issue`（`relation-types` 已列两个候选，未选）。
5. **`derived-from` 的语态/方向仍别扭**（实验二遗留热点），保留是硬性决定还是可再议，未封。
6. **同源（`common-origin`）无名字**：两 issue 同源、而来源不是 issue 时，无键可表达（已另开 issue）。

### 2. 待决清单（请拍板）

| # | 待决项 | 选项 | 备注 |
| --- | --- | --- | --- |
| **D1** | 反向名是否回写侧（写 9） | **已定：回**（写 9） | 实验：不暴露反名会诱发写侧错配 |
| **D2** | 依赖族**主名** | **已定：`blocks` / `blocked-by`** | 保留主流用词；两名均可写（写 9） |
| **D3** | 反名 `spawns` | 保留 / `yields` / `gives-rise-to` / `branches-into` / （换成 `spawned-by`+`spawns` 对） | 与 `derived-from` 对称性 |
| **D4** | 反名 `has-part` | 保留 / 改 `has-sub-issue` / `contains` | 与 `part-of` 对称性 |
| **D5** | `part-of` 一族的名 | `part-of`/`has-part` vs `sub-issue-of`/`has-sub-issue` | `relation-types` 待选 |
| **D6** | `derived-from` 的名字 | 保留 / 改名 | **不在本 issue**——归 `derived-from-direction`：读侧已证清白（2/2），但 wsl-agent owner 提过改名，且「写反了没人管」待验 |
| **D7** | 同源（`common-origin`）新名 | 新造 | 归 `common-origin` |

> **澄清（D3）**：`derive` / `derives-from` **不是** `derived-from` 的反向——英文里两者**同向**（`X derives from Y` ≡ `X is derived from Y`），只是语态不同。反向必须让**来源方当主语**：`spawns` / `yields` / `gives-rise-to` / `produces`。唯一同词根的双向对是 **`spawned-by` / `spawns`**（但那是把 `derived-from` 换掉，有迁移成本，且 `spawned-by` 与 `derived-from` 同向）。→ D3 的候选收窄为 {`spawns`（保留）、`yields`、`gives-rise-to`、`branches-into`} 或整族改用 `spawned-by`/`spawns`。

### 3. 已定、不动的部分

- **两条命名原则**：有向关系的键名必须含**介词**；**主语恒为声明方**。
- **`relates-to` 保留**（第四轮：在有干扰项下未被滥用）。
- **`derived-from` 读侧已证清白**（第三轮裸名字：2/2 读对、清晰度 5/5）——但**名字是否保留归 `derived-from-direction`**（wsl-agent owner 提过改名；写侧方向与“写反了没人管”未验）。
- **数量结构**：非对称关系写侧正反各一；对称关系一名，无反向。

### 4. 影响面（定了之后要回写的）

- **`derived-from` 的命名与写侧方向**归 `derived-from-direction`（后者在本仓已存在：wsl-agent 反馈 + 「写反了没人管」未验）；其非目标里「写侧 5 个名字已定稿」的前提已因本节点过期，需一并更新。
- `relation-rendering` 的结论已假定「写 9 / 句渲 2 / 图渲 1」与反名可渲；**若 D2 改主名，箭头规范方向随之变**。
- `relation-types` 的「9 个名字」需与 D1–D5 同步。
- 默认词表（模板）在 D1–D5 定了之后才能冻结。

## collect：「衍生 / 来源」这一对，业内一般叫什么（2026-10-18）

> 用户裁定 **D1 = 回**（写侧 9 名）、**D2 = `blocks` / `blocked-by`**（已回写入上表）。本轮只看一件事：**「X 由 Y 引发」这一对方向关系，同行怎么命名。**

### 1. 三条结论

1. **主流把「一对方向关系」建模成「一个链接类型 + 正反两个名字」**，你在一侧加、**另一侧自动出现**——这正是 D1=回 的业内形态。
2. 「衍生/来源」这类，**Jira 叫 `causes` / `is caused by`**；**需求工程（SysRE）叫 `derived from` / `derives-from` / `gives-rise-to`**（SysML `«deriveReqt»`、NIST/ISO 29148 术语）。
3. **`spawns` 不是业内词**——没找到任何 tracker 用它。

### 2. 逐工具证据

| 领域 / 工具 | 「衍生 → 来源」叫什么 | 反向叫什么 | 备注 |
| --- | --- | --- | --- |
| **Jira** | `is caused by` | `causes` | 链接类型名 “Problem/Incident”（outward=`causes`, inward=`is caused by`）；**在默认链接类型列表里** |
| **YouTrack** | ——（默认无） | —— | 方向模型分 **Directed / Undirected / Aggregation**；名称成对（inward/outward）；`causes`/`is caused by` 可作链接类型加（Jira 导入时出现） |
| **Azure Boards** | ——（**无「引发」类**） | —— | 只有 Related / Predecessor-Successor / Parent-Child / Duplicate / Affects-Affected by |
| **需求工程（NIST / ISO 29148）** | **`derived from`**（“derived requirement”：由高层需求隐含或转换而来） | —— | 术语地位：`parent requirement` / `child requirement`、upward/downward traceability |
| **SysML / OMG** | **`«deriveReqt»`**（client requirement derived from supplier） | —— | 箭头从 derived（client）→ supplier；属性 `/derived`、`/derivedFrom` |
| **RE trace-link 分类学** | **`derives-from`**（Requirement → Business Goal） | **`gives-rise-to`**（Business Goal → Requirement） | 明确把反名定为 `gives-rise-to` |
| **Jira / GreenHopper** | `split from` | `split to` | 拆分场景（`SplitIssueLinkTypeService`：把一个 issue 链到“从它拆出来”的 issues） |

### 3. 关键结构发现（比名字更重要）

1. **一个链接类型 = inward/outward 两个名字**：Jira 管理端直接把链接类型定义为「Name + Outward description + Inward description」（例：Problem/Incident → `causes` / `is caused by`）；YouTrack 同样“Directed 与 Aggregation 用不同的 inward/outward 名”。→ **写侧 9 名（正反各一）是业内标准形态**。
2. **写一侧、另一侧自动出现**：YouTrack：“When you add the is required for link to issue A, a depends on link is automatically added to issue B.” → 正好是 kiln 的「单侧声明 + 读侧推导反向」。
3. **方向分三类**：YouTrack 明确分 **Directed（依赖）/ Undirected（关联）/ Aggregation（分解/去重）** → 与 kiln 的「阻塞/非阻塞 + 有向/无向」两轴同构，且**分解被单列**。
4. **名字多为带介词的短语**（`is blocked by` / `is caused by` / `derived from` / `subtask of`）→ 与「有向关系名必须含介词」一致。

### 4. 对 D3 的启示

- **`derived-from` 有业内地位**（RE / SysML / NIST）——保留。
- **`spawns` 无业内对应**。若要与行业对齐，反名候选：`causes`（Jira）/ `gives-rise-to`（RE）/ `yields`。
- 注意 `causes` 是“因→果”框架：`X is caused by Y` ≈ `X derived-from Y`，两者方向一致；`causes` 就是反名。

### 5. 来源

- Jira 链接类型列表：[Link work items（Jira Cloud）](https://support.atlassian.com/jira-software-cloud/docs/link-issues/)（含 `causes` / `is caused by`）。
- Jira 链接类型定义（outward/inward descriptions）：[Configure work item linking](https://support.atlassian.com/jira-cloud-administration/docs/configure-issue-linking/)（一手）。
- YouTrack：[Link Issues（YouTrack Server 2026.2）](https://www.jetbrains.com/help/youtrack/server/link-issues.html)（一手；Directed/Undirected/Aggregation、inward/outward 名、自动补反向）。
- Azure Boards：[Link Types Reference](https://learn.microsoft.com/en-us/azure/devops/boards/queries/link-type-reference)（一手；无“引发”类）。
- SysML `«deriveReqt»`：[OMG SysML 1.3/1.6 §16.3.2.2](https://www.omg.org/spec/SysML/1.3/Beta1/PDF/changebar)（一手）。
- 需求工程术语：[NIST CSRC「derived requirement」（引 ISO/IEC/IEEE 29148:2018）](https://csrc.nist.gov/glossary/term/derived_requirement)（一手）。
- Jira split：[SplitIssueLinkTypeServiceImpl](https://docs.atlassian.com/jira-software/10.0.0/com/atlassian/greenhopper/service/issuelink/SplitIssueLinkTypeServiceImpl.html)（一手/API 文档）。
- RE trace-link 分类学（`derives-from`/`gives-rise-to`）：[chwezi-sdlc-documentation trace-link-types](https://github.com/peterbamuhigire/chwezi-sdlc-documentation/blob/main/02-requirements-engineering/fundamentals/after/09-traceability-engineering/references/trace-link-types.md)（二手，社区整理）。
