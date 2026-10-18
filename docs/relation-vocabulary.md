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

## hypothesize：对比实验设计——`derived-from`/`spawns` vs `causes`/`caused-by`（含 `blocks`/`blocked-by` 对照）

> 用户要求（2026-10-18）：设计一个对比实验，比“现状名”与 `causes`/`caused-by` 这一对；并把 `blocks`/`blocked-by` 也放进实验作对照。本节点只**设计**，不跑（跑是下一个 `test`）。

### 1. 要回答的问题

- **主问题**：引发族的名字，`derived-from`/`spawns` 与 `caused-by`/`causes`，哪个对 agent **更不易错、更清晰**？
- **对照问题**：已定的 `blocks`/`blocked-by` 在两臂里表现是否一致（若不一致，说明实验有偏）。

### 2. 变量隔离

| 项 | 处理 |
| --- | --- |
| 只变 | **引发族**一对名（N/C 两臂不同） |
| 固定作对照 | **依赖族** `blocks`/`blocked-by`（两臂相同） |
| 完全固定 | `part-of`/`has-part`、`supersedes`/`superseded-by`、`relates-to` |
| 渲染形态 | 用 `relation-rendering` 已 conclude 的**视角化句子（主语=X）** |
| 命名原则 | 两臂都遵守“含介词”+“主语=声明方” |

### 3. 臂与条件

| 臂 | 引发族（效果→来源 / 来源→效果） | 依赖族（对照） |
| --- | --- | --- |
| **N（现状）** | `derived-from` / `spawns` | `blocks` / `blocked-by` |
| **C（causes）** | `caused-by` / `causes` | `blocks` / `blocked-by` |
| （可选 **R（RE）**） | `derives-from` / `gives-rise-to` | `blocks` / `blocked-by` |

两种条件：

- **条件 1（带释义）**：每个名给一句 **主语=声明方** 的释义（模拟 Skill）。两臂释义力度相同，以隔离“名字本身的可读性”。
- **条件 2（裸名字）**：不给释义，只问方向（测**先验**；复现 `relation-vocabulary` 第三轮）。

### 4. 实验材料（只含引发 + 依赖，隔离变量）

声明侧（写在哪个 issue 里）：

| 边 | 声明内容 |
| --- | --- |
| e1 | P2 声明 `derived-from: P1`（P1 引发 P2） |
| e2 | P3 声明 `derived-from: P1` |
| e3 | P1 声明 `blocks: P4` |
| e4 | P5 声明 `blocks: P1` |

两个观察页（两臂的渲染）：

| 事实 | N 臂在 P1 页 | C 臂在 P1 页 |
| --- | --- | --- |
| P1 引发 P2 | `P1 spawns P2` | `P1 causes P2` |
| P1 引发 P3 | `P1 spawns P3` | `P1 causes P3` |
| P1 阻塞 P4 | `P1 blocks P4` | `P1 blocks P4` |
| P5 阻塞 P1 | `P1 blocked-by P5` | `P1 blocked-by P5` |

额外观测页 **P2**（用来展示“效果侧”主名）：N 臂 `P2 derived-from P1` / C 臂 `P2 caused-by P1`。

→ 变量只剩一处：**当 X 是“来源”时，X 页上那个名是 `spawns` 还是 `causes`**；以及 P2 页上是 `derived-from` 还是 `caused-by`。

### 5. 题库（含预期答案）

**读向（D1–D4）**
| # | 页 | 题 | 预期 |
| --- | --- | --- | --- |
| D1 | P1 | P1 与 P2 谁引发了谁？ | P1 引发 P2 |
| D2 | P2 | 同上（换一页再问同一事实） | P1 引发 P2 |
| D3 | P1 | P1 与 P4 谁先完成？ | P1（P1 blocks P4） |
| D4 | P1 | P1 与 P5 谁先完成？ | P5（P5 blocks P1） |

**写侧（W1–W4）** — 都是“新建 issue，引用已有 issue”
| # | 场景 | N 臂预期 | C 臂预期 |
| --- | --- | --- | --- |
| W1 | 新建 Q，Q 由已有 R 引发 | `derived-from: R` | `caused-by: R` |
| W2 | 新建 Q，Q 引发了已有 R | `spawns: R` | `causes: R` |
| W3 | 新建 Q，Q 阻塞已有 R | `blocks: R` | `blocks: R` |
| W4 | 新建 Q，Q 被已有 R 阻塞 | `blocked-by: R` | `blocked-by: R` |

**词对（P1–P2）**
| # | 题 | 预期 |
| --- | --- | --- |
| P1 | 名 `<效果侧>` 与 `<来源侧>` 是什么关系？ | 同一关系的正/反两面 |
| P2 | `blocks` 与 `blocked-by` 呢？ | 同上 |

**跨页同一（I1）**
| # | 题 | 预期 |
| --- | --- | --- |
| I1 | P1 页的 `<来源侧名> P2` 与 P2 页的 `<效果侧名> P1`，几条关系？ | **1** |

最后一行自由文本：`困惑: …`（哪一行最不确定）。

### 6. 假设

| # | 假设 |
| --- | --- |
| **H1** | C 臂的读向正确率 ≥ N 臂（`causes`/`caused-by` 英语因果常识强） |
| **H2** | C 臂的写侧正确率 ≥ N 臂（同词根主动/被动，对称好推） |
| **H3** | 两臂对 `blocks`/`blocked-by` 的表现一致（对照组，证明实验无偏） |
| **H4** | 裸名字条件下，`derived-from` 仍读得对（复现第三轮 2/2） |

### 7. 预登记判据（先定，避免事后挑数）

1. **否决项**：某臂读向 D1–D4 正确率 < 3/4 → 该臂淘汰（不管写侧多好）。
2. **主判据**：写侧 W1–W4 正确率高者胜；**差 ≤1 题视为平手**。
3. **平手裁定顺序**：① 与 `blocks`/`blocked-by` 的**形态一致性**（`causes`/`caused-by` 与 `blocks`/`blocked-by` 同为“同词根·主动/被动”）；② 自报困惑总条数；③ 业内地位（Jira 用 `causes`；RE 用 `derived-from`）。
4. **控制失败**：若 H3 不成立（两臂对 `blocks`/`blocked-by` 差异 ≥2 题）→ 判实验设计有问题，本轮作废。

### 8. 样本与隔离

- **2 臂 × 2 agent × 2 条件 = 8 个 subagent**（可选 R 臂则 +4）。
- 隔离按 `relation-rendering` 已实测定稿的配方：`--no-tools` + 中性临时目录 + `-nc -ns -np -na` + `--no-session`；两臂 prompt 分开放。
- 隔离探针（先跑一次关键词探针）确认无泄漏后再开。

### 9. 预期陷阱

1. **`causes` 太通用**：英语里“causes”日常就是“导致”，可能被读成泛因果而丢掉“可各自处理”的语义（与 `blocks` 混）。→ 由 D3/D4 与 P2 题监控。
2. **`derived-from` 太“需求工程”腔**：可能让 agent 觉得它很正式而过度谨慎（多停一秒）。→ 由自报困惑与裸名字条件监控。
3. **两臂对“引发”的熟悉度不同**（`causes` 先验强）——这正是要测的，不做中和。

### 10. 与既有实验的关系

- 第三轮已证 `derived-from` **裸名字读得对**（2/2、5/5）；本实验补上：**写侧对比**、与 `causes` 的正面比较、以及 `blocks` 对照。

### 结论（实验方案，待执行）

- 2 臂（N/C，可选 R）× 2 条件 × 2 agent，12 题（读 4 / 写 4 / 词对 2 / 跨页 1 / 困惑 1），成本与既有几轮同量级。
- 先定判据（§7）**再跑**。下一个动作：`test`。

## test：对比实验结果——`derived-from`/`spawns` vs `caused-by`/`causes`（2026-10-18 执行）

> 按上节方案执行（**不加 R 臂**，两条件都跑）。工具：`pi -p --no-tools --no-session -nc -ns -np -na`，中性临时目录 `/tmp/tmp.otluHLfTUt`。**先跑隔离探针**：上下文里除了框架性 system prompt 块与 cwd，无任何项目指令/技能；`kiln`/`relation`/`实验`/`derived-from` 只出现在用户消息本身。确认无泄漏后开跑。

### 1. 逐 agent 得分

条件 1（带释义）12 题；条件 2（裸名字）无写侧题，共 7 题。

| agent | 臂 | 条件 | 读向 D | 写侧 W | 词对 P | 跨页 I | 合计 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| N1.1 | N | 带释义 | 4/4 | 4/4 | 2/2 | 1/1 | **12/12** |
| N1.2 | N | 带释义 | 4/4 | 4/4 | 2/2 | 1/1 | **12/12** |
| N0.1 | N | 裸 | 4/4 | — | 2/2 | 1/1 | **7/7** |
| N0.2 | N | 裸 | 4/4 | — | 2/2 | 1/1 | **7/7** |
| C1.1 | C | 带释义 | 4/4 | 4/4 | 2/2 | 1/1 | **12/12** |
| C1.2 | C | 带释义 | 4/4 | 4/4 | 2/2 | 1/1 | **12/12** |
| C0.1 | C | 裸 | 4/4 | — | 2/2 | 1/1 | **7/7** |
| C0.2 | C | 裸 | 4/4 | — | 2/2 | 1/1 | **7/7** |

**全部满分。两臂在所有维度上完全打平。**

### 2. 假设核对

| # | 假设 | 结果 |
| --- | --- | --- |
| **H1** | C 臂读向 ≥ N 臂 | 两者均 100% → **打平** |
| **H2** | C 臂写侧 ≥ N 臂 | 两者均 4/4 → **打平** |
| **H3** | 两臂对 `blocks`/`blocked-by` 表现一致 | **成立**（相关题两臂均 100%）→ 实验无偏 |
| **H4** | 裸名字下 `derived-from` 仍读得对 | **成立**（复现第三轮） |

### 3. 按预登记判据裁定

1. **否决项**（读向 <3/4）：两臂都没触发。
2. **主判据**（写侧高者胜；差 ≤1 = 平手）：**平手**（均 4/4）。
3. **平手裁定顺序**：
   - ① **与 `blocks`/`blocked-by` 的形态一致性** → **C 臂胜**：`causes`/`caused-by` 与 `blocks`/`blocked-by` 同为“词根相同·主动/被动”；而 `derived-from`/`spawns` 是**异构对**（分词+介词 vs 光杆动词）。
   - ② 自报困惑：两臂各 1 条（N0.2 疑 `spawns`/`derived-from` 是否同一条；C0.1 疑 `blocked-by` 方向）→ 打平。
   - ③ 业内地位：Jira 用 `causes`；需求工程用 `derived-from` → 两边都有。
4. **控制失败**：未触发。

→ **预登记规则的结论：写侧平手 → 由①形态一致性判给 C 臂（`causes`/`caused-by`）。**

### 4. 真发现

1. **正确性上无法区分**：两对名字在两条件下**均满分**（含裸名字）——说明 agent 根本不会因为改用 `causes`/`caused-by` 而变差，也不会因为用 `derived-from`/`spawns` 而变差。
2. **天花板效应**：题太容易（带释义 + 场景直白），实验**没有区分力**。它只证明了“两对都过关”，没证明“谁更好”。
3. **唯一结构性差异在“形态”**：`causes`/`caused-by` 与已定的 `blocks`/`blocked-by` 同形；`derived-from`/`spawns` 异构——这正是 `relation-types` 当年就记下的“不对称”患处。
4. **裸名字也没拉开差距**：说明“`derived-from` 靠先验读得对”与“`causes` 靠英语常识读得对”打平——两都不需要释义。

### 5. 建议

- **若以词表形态齐整为先**：改用 `causes`/`caused-by`（与 `blocks`/`blocked-by` 同形，且对齐 Jira）；`derived-from` 作为 **alias** 保留读旧提交（已有历史里写的是 `derived-from:`）。
- **若以零迁移/学术术语为先**：保留 `derived-from`，只把 `spawns` 换成 `gives-rise-to`（RE 规范对名）——但仍是异构对。
- 两条路实验上都不扣分；这是**取舍**，不是正确性问题。

### 6. 局限

- **天花板效应**（见上）——要真正分出高下，需要更难的题：长链、多级引用、无释义、强干扰项（如同时有 `causes` 与 `blocks` 同现）。
- 样本仅 2/臂/条件，同源模型。
- 未测迁移成本与 alias 读取。

### 结论（`test`）

两对名字**正确性上无法区分**；预登记规则下由“形态一致性”判给 **`causes`/`caused-by`**。是否采纳属**取舍**（迁移 vs 齐整），待用户拍板。

## hypothesize：加难版对比实验设计（破解天花板效应）

> 上一版全满分、无区分力。本版针对性地把难度与**分歧点**压在两类题上：**源方向写侧**（`spawns` 的患处）与**引发/阻塞消歧**（`causes` 的惠处/风险）。仍只设计，不跑。

### 1. 为什么上一版太容易（对症下药）

| 问题 | 加难手段 |
| --- | --- |
| 只 4 条边、X 页只 4 行 | 扩到 **7 行/页**，**5 族混排**（引发/阻塞/分解/取代/关联） |
| 带释义 → 方向直接告诉 | 主条件改为**名字无释义**（只给键名列表，不给方向） |
| 场景太直白 | 加**对抗措辞**（反向叙述）与**多跳**（跨页推链） |
| 写侧只考“手在效果侧” | 重点考 **源方向写侧**（手在来源侧） |
| 无干扰项 | 同页混排多族；加“同名字不同边”的陷阱题 |

### 2. 臂与条件

| 臂 | 引发族 | 其余四族（两臂相同） |
| --- | --- | --- |
| **N** | `derived-from` / `spawns` | `blocks`/`blocked-by`、`part-of`/`has-part`、`supersedes`/`superseded-by`、`relates-to` |
| **C** | `caused-by` / `causes` | 同上 |

- **条件 H1（名字无释义，主条件）**：给两张表——“可写的键名列表”（只有名字）+ 渲染页；**不给方向释义**。agent 必须从**名字形态本身 + 渲染页**推断方向。这是**真正的区分条件**。
- **条件 H2（名字+释义，对照）**：同上一版，预期仍高分；用来证明难度来自名字而非任务。

### 3. 实验图（多族、含“同对两族”反例）

声明侧（写在哪个 issue 里）：

| 边 | 声明 | 族 / 槽 |
| --- | --- | --- |
| e1 | P2 `derived-from` P1 | 引发 / 谱系 |
| e2 | P1 `blocks` P2 | 阻塞 |
| e3 | P3 `derived-from` P2 | 引发 / 谱系 |
| e4 | P3 `blocks` P4 | 阻塞 |
| e5 | P8 `blocks` P3 | 阻塞 |
| e6 | P3 `part-of` P9 | 分解 / 阻塞槽 |
| e7 | P6 `supersedes` P3 | 取代 / 谱系槽 |
| e8 | P3 `relates-to` P7 | 关联 / 谱系槽 |
| e9 | P10 `derived-from` P3 | 引发 / 谱系 |

注：`{P1,P2}` 同时有 **e1（引发）与 e2（阻塞）**——同一对、两族、反向，是旧推演里的那个反例；也顺便验“两族不混”。

### 4. 渲染页（视角化句子，主语=当前页）

**P3 页（7 行，所有族）**：

| 事实 | N 臂 | C 臂 |
| --- | --- | --- |
| P3 由 P2 引发 | `P3 derived-from P2` | `P3 caused-by P2` |
| P3 引发了 P10 | `P3 spawns P10` | `P3 causes P10` |
| P6 取代了 P3 | `P3 superseded-by P6` | `P3 superseded-by P6` |
| P3 与 P7 相关 | `P3 relates-to P7` | `P3 relates-to P7` |
| P3 阻塞 P4 | `P3 blocks P4` | `P3 blocks P4` |
| P8 阻塞 P3 | `P3 blocked-by P8` | `P3 blocked-by P8` |
| P3 属于 P9 | `P3 part-of P9` | `P3 part-of P9` |

**P2 页**：N `P2 derived-from P1` / C `P2 caused-by P1`。
**P4 页**：`P4 blocked-by P3`（两臂相同）。

### 5. 题库（20 题；★ = 预登记的“分歧题”）

**D 类 方向（含多跳）**
| # | 题 | 预期 |
| --- | --- | --- |
| D1 | P3 与 P2，谁引发谁？ | P2 引发 P3 |
| ★D2 | P3 与 P10，谁引发谁？ | P3 引发 P10 |
| D3 | P3 与 P4，谁先完成？ | P3 |
| ★D4 | P3 与 P8，谁先完成？ | P8 |
| D5 | P3 与 P9，谁先完成？ | P3（P9 等 P3） |
| D6 | 结合 P2 页：P3 最上游的来源是谁？ | **P1** |

**E 类 消歧（引发 vs 阻塞）**
| # | 题 | 预期 |
| --- | --- | --- |
| ★E1 | P3 与 P4 是“谁等谁”还是“谁由谁引发”？ | 谁等谁（阻塞） |
| ★E2 | P3 与 P2 是“谁等谁”还是“谁由谁引发”？ | 引发 |

**W 类 写侧（角色/方向陷阱）** — 均“新建 Q，引用已有 R”
| # | 场景 | N 预期 | C 预期 |
| --- | --- | --- | --- |
| W1 | Q 由 R 引发 | `derived-from: R` | `caused-by: R` |
| ★W2 | 已有 R 由 Q 引发（Q 是来源） | `spawns: R` | `causes: R` |
| W3 | Q 阻塞 R | `blocks: R` | `blocks: R` |
| W4 | Q 被 R 阻塞 | `blocked-by: R` | `blocked-by: R` |
| W5 | Q 是 R 的一部分 | `part-of: R` | `part-of: R` |
| W6 | Q 取代 R | `supersedes: R` | `supersedes: R` |
| ★W7 | “已有 R 是因为新建的 Q 才出现的”（反向叙述） | `spawns: R` | `causes: R` |

**X 类 跨页/陷阱**
| # | 题 | 预期 |
| --- | --- | --- |
| ★X1 | P3 页 `P3 <效果侧名> P2` 与 P2 页 `P2 <效果侧名> P1`，是同一条关系吗？ | **不是**（不同对） |
| ★X2 | P2 页会不会出现 `P2 <来源侧名> P3`？ | **会**（同一边的反向视图） |
| X3 | P4 页的 `P4 blocked-by P3` 与 P3 页的 `P3 blocks P4`，几条关系？ | 1 |

**T 类 干扰下选择**
| # | 题 | 预期 |
| --- | --- | --- |
| T1 | P3 页里哪些行表示“别的 issue 必须等 P3”？ | `P3 blocks P4`、`P3 part-of P9` |
| T2 | P3 页里哪行表示“P3 必须等别的 issue”？ | `P3 blocked-by P8` |

最后一行：`困惑: …`。

### 6. 假设

| # | 假设 | 方向 |
| --- | --- | --- |
| **H1** | 无释义下，C 臂在**源方向写侧**（W2/W7）正确率 > N 臂 | 利 C（`causes`/`caused-by` 对称，`spawns` 靠额外知识） |
| **H2** | 无释义下，C 臂在**消歧题**（E1/E2）正确率 ≥ N 臂 | **双刃**：`causes` 可能被读成“阻塞/顺序”→ 若如此则 C 输 |
| **H3** | N 臂在 `derived-from` 读向（D1/D2）仍稳 | 复现第三轮 |
| **H4** | 两臂在 `blocks`/`blocked-by` 对照题（D3/D4/W3/W4/T1/T2）一致 | 对照无偏 |

### 7. 预登记判据（先定）

1. **主判据**：在 **H1 条件（名字无释义）** 下，**★ 分歧题**（D2、D4、E1、E2、W2、W7、X1、X2 共 8 题）的正确率高者胜；**差 ≤1 题 = 平手**。
2. **否决项**：某臂在 ★ 分歧题上 < 50%（≤4/8）→ 淘汰。
3. **天花板检查**：若 H1 条件下两臂均 ★ ≥ 7/8，**判本轮仍无区分力**，不选边（不靠噪声选名），转下一手段（更长的链 / 更多页 / 真实仓回放）。
4. **平手裁定**（同上：① 与 `blocks` 同形 ② 困惑数 ③ 业内地位）。
5. **控制失败**：若 H4 不成立（对照题差异 ≥2）→ 判设计有偏，本轮作废。

### 8. 样本与隔离

- 2 臂 × 2 条件 × 2 agent = **8 个 subagent**。
- 隔离按 `relation-rendering` 的配方（`--no-tools` + 中性目录 + `-nc -ns -np -na` + `--no-session`），**先跑关键词探针**确认无泄漏。
- 条件 H1 的 prompt 中，“可写键名列表”**只列名字、不写方向释义**；两臂列表形式一致（同列 9 名）。

### 9. 预期与风险

- **可能仍打平**：两份名字对 LLM 而言都足够清楚——若如此，结论应是“名字不是瓶颈，别再折腾改名”，这本身是有价值的结论。
- **最可能拉开的点**：W2/W7（源方向写侧）与 X1/X2（跨页/陷阱）。
- **`causes` 的特定风险**：在 E1/E2 上被读成“阻塞/顺序”（因“导致”常隐含先后）——若 H2 反向成立，会直接推翻 C 臂的优势。

### 结论（实验方案，待执行）

- 20 题、2 臂×2 条件×2 agent；预登记 8 题分歧子集与**天花板检查**。
- 下一个动作：`test`。
