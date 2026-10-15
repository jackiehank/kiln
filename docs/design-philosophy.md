# issue: design-philosophy — 抽象层面的两条理念：原子组合解决复杂性 + 诘问法式的方法论

- **domain**：design
- **关联**：relates-to `design-tensions` / `evidence-fluidity`

## 背景

本仓已有的"理念"多以**工程约定**的形态出现（无模式 / 不建文件 / 宽容输入严格输出 / LLM 先验优先），而 [design-tensions](design-tensions.md) 盘的是**理念与实现之间的张力**。两者都不是本 issue 要谈的东西。

本 issue 要谈的是**抽象层**的两条：

1. **原子组合解决复杂性**——不设计流程，而是把流程拆成原子（issue 原子 + action 原子），让复杂性在**组合**里涌现。这条在 README 已成文，但只写了"是什么"，没写"为什么有效、边界在哪"。
2. **诘问法式的方法论**——本工具的动作词表本身就是一次**苏格拉底式诘问**（elenchus）的编码：提出（`ask`/`hypothesize`）→ 取证（`collect`）→ **检验**（`test`）→ 收敛（`conclude`/`apply`）。而它真正区别于"流程管理"的地方是：**结论随时可以被下一步实测推翻**。

第 2 条并非事后附会——**本仓最近几轮的实践就是一连串诘问**（见下表）。

## 实践里的诘问记录（本仓真实发生，非举例）

| 提案（`hypothesize`） | 检验手段 | 结果 → 修正 |
| --- | --- | --- |
| 让 `status` 显示 `domain/id`，输入侧接受限定名（= 引入命名空间） | 实测：`kiln path design/xxx` 报"未找到问题"；用户指出"这其实是显示问题" | 拆掉前缀、改块头显示 domain（`qualified-issue-id`） |
| 保留跨域嵌套（树形） | 用户裁定："关系会变复杂，`status` 没必要承担这个职责" | 嵌套整体废除，`engine` 净删代码 |
| 给起点 / 终点打行标记（E 方案） | 用户反问："终点标记会不会让人以为不能再加 action？" | E 收窄为只标起点 → 再收紧为**不标**（恒真标记=噪声） |
| 报错兜底取 stdout **首行**（A 方案） | **真沙盒跑一次**：输出 `git commit: On branch master`——只有上下文 | 改为取**末行**（A′）：一行给全"原因 + 修法"（`vcs-error-detail`） |
| 唯一性加"读侧告警"（C 方案） | 分析：该告警对历史**不可满足**（补 `ask` 后位置依然不是第一） | 出局，改为写侧拒绝（`issue-uniqueness`） |

上表里，"检验"既有**实测输出**，也有**用户的反例**——两者都是诘问的合法动作。

## 要回答的问题

1. **机制**：「原子组合解决复杂性」能不能讲清复杂性被**推到了哪里、由谁承担**？（倾向：kiln 自己不承担——没有流程引擎、没有状态机、没有内部树；它只保证**两端**（起点存在、最后是终点）与**关系可重建**。）
2. **边界**：什么时候原子化**反而更贵**？候选：需要**全局不变量**的判断（如跨 issue 的唯一性——要读全史）、需要**一次性看全貌**的决策、以及"组合的可见性"（关系一多，谁来读？）。
3. **方法论是不是内核**：词表 + `check`（不变量）+ 写侧守卫，是否构成一套可复述的流程？它与"诘问法"的对应关系是**比喻**还是**事实**？
4. **两条理念的关系**：**原子动作让诘问可执行**（每一步都能独立提交、独立被推翻），**诘问让原子动作有意义**（否则只是一堆碎片）——这个互补说法站得住吗？
5. **"被打回的方案 + 实测输出"要不要当一等记录**：本仓事实上已经在做（`agent-ux-review` 保留了被否的 E 形态，`vcs-error-detail` 保留了 A 字面版的输出），但**没写成规矩**。要写成规矩吗（比如 docs 里固定一个小节）？
6. **落点**：这些抽象理念放哪——沿用 `design-tensions` 的候选 **D**（`docs/design.md` 权威 + README 一屏摘要 + `mvp-plan` §1 只留判据）？

## 非目标

- **不写宣传文案**：每条理念都要能指出它在实现里的**对应事实**（或明确指出它是"尚未落地的取向"）。
- 不重复 [design-tensions](design-tensions.md) 的七条实现层张力——本 issue 只谈抽象机制与方法论。
- **不动代码**、不改已 `conclude` 的结论。

## collect：「原子组合解决复杂性」的机制、边界与反方证据

> 一轮联网调研：初稿由 **pi 子代理**（`pi -p`，独立 session `kiln-design-philosophy-collect`）检索；本仓**逐条复核**承重引用——凡写"原文"的都核对过原出处，整理/读书笔记类标 [二手]。来源见节末。

### 1. 先纠正一处常被引错的归属

常被引的"Unix 哲学三条"（do one thing well / work together / text streams）是**后来的概括**。**原始文本是 McIlroy 1978 年给 BSTJ Unix 专号的 foreword，四条**，其中第二条才是"组合"的正身：

> Expect the output of every program to become the input to another, **as yet unknown**, program.

而那句最像本 issue 的话——"**系统的能力更多来自程序之间的关系，而不是程序本身**"——**不是 McIlroy 说的**，出自 Kernighan & Pike《The Unix Programming Environment》(1984)：

> …at its heart is the idea that the power of a system comes more from the relationships among programs than from the programs themselves. Many UNIX programs do quite trivial things in isolation, but, combined with other programs, become general and useful tools.

→ 对 kiln 的意义：它引用的这条理念，**原始表述本身就是"关系 > 个体"**——与"issue 原子 + 关系组合"同构。引依据时要引对地方。

### 2. 机制：为什么"原子 + 组合"能对付复杂性

**2.1 稳定子装配让演化变快（Simon 的制表匠）**

Simon 1962《The Architecture of Complexity》里的寓言：Hora 把表拆成**每约十个元件一组的子装配**，Tempus 从 1000 个零件一次装；同样会被电话打断，

> he had designed them so that he could put together subassemblies of about ten elements each

原文给的数字是：Tempus 每完成一只表要做的尝试是 Hora 的 **2 万倍**。Simon 把它一般化为：层次系统**通常是"近乎可分解的"（nearly decomposable）**——子系统之间的作用弱于内部。

→ 机制一：**原子化把"中断 / 返工"的代价从整块降到一小块**。kiln 的对应事实：一个动作就是一条提交，写坏了只影响那一条。

**2.2 复杂性来自"纠结"，不来自零件数（Hickey）**

Hickey《Simple Made Easy》把 "complect" 拉回来用：

> It means to interleave or entwine or braid.

simple 的词根是"一折"，complex 是"多折 / 编在一起"——**关键在是否被缠在一起，不在零件多少**。

→ 机制二：**原子化的目标是"不纠结"（decomplect）**，让每个单元能被独立理解与替换。对应事实：kiln 的原子动作之间**没有顺序依赖**（除了两端），读一个动作的语义不需要先读完整条流程。

**2.3 分解的判据不是"执行顺序"（Parnas / Ousterhout）——这是对 kiln 最硬的反方**

Parnas 1972 在比较两种分解时说，按信息隐藏的那一种：

> The modules no longer correspond to **steps in the processing**.

Ousterhout 把"按执行顺序拆"命名为 **temporal decomposition**，并给出规则：

> when designing modules focus on the knowledge that's needed to perform each task, **not the order in which tasks occur**.

→ kiln 的动作词表（ask → test → conclude）本质上是**时间序**的，这条质疑必须正面回答。（候选回应见 §5 第 3 条，尚未定论。）

**2.4 日志是首要事实、视图是派生的（Kleppmann）**

> A materialized view is just a cached subset of the log, and you could rebuild it from the log at any time.
> …think of a database as an always-growing collection of immutable facts.

→ 机制四：**只要日志是唯一真相，就消掉了"同步状态"那一整类复杂性**。kiln 的对应极准：提交历史是 log，`status` / `path` / `check` / `graph` 全是**派生视图**（每次都从头算，不留缓存）。

### 3. 边界：它削的是哪一种复杂性

- Brooks《No Silver Bullet》：「**The complexity of software is an essential property, not an accidental one.**」→ 分解法只能削掉 **accidental** 的那部分；**essential 部分不会因为拆细而消失**。
- Moseley & Marks《Out of the Tar Pit》：复杂性归给 **state / control / volume** 三项；对治是"避免 + 分离"。
- Anderson《More Is Different》(1972)：整篇不含 "emergence" 这个词，但说清了集体行为：

> the whole becomes not only more than but **very different from** the sum of its parts.

→ 所以"复杂性在组合中涌现"有**两面**：
  - **乐观面（Anderson）**：组合产生低层没有的**新性质**；
  - **代价面（Brooks）**：组合也产生**新的、不可约的**复杂性。
  本仓文档目前只写了前一面。

### 4. 反方与边界清单（原子化的代价，逐条配证据）

| # | 代价 | 证据 |
| --- | --- | --- |
| 1 | **集成 / 分布税** | Fowler《MicroservicePremium》：「don't even consider microservices unless you have a system that's too complex to manage as a monolith. **The majority of software systems should be built as a single monolithic application.**」（配《MonolithFirst》） |
| 2 | **真实回退案例** | Prime Video 官方博客《Scaling up the Prime Video audio/video monitoring service and reducing costs by 90%》：分布式 serverless 方案「hit a hard scaling limit at around **5% of the expected load**」，改单体后「reduced our infrastructure cost by **over 90%**」（原文页在本环境未能直接取到，引文据多份二手转述一致） |
| 3 | **拆错边界比不拆更糟** | Parnas 的判据是**信息隐藏**；Foote & Yoder《Big Ball of Mud》描述"边界只存在于白板上"的系统 |
| 4 | **"独立"是假的** | Waldo et al. 1994《A Note on Distributed Computing》：远程调用**不是**本地调用——延迟与**部分失败**无法隐藏 |
| 5 | **粒度无人能事先定** | Zettelkasten 的做法可抄：原子性「**不是铁律，而是一个方向**」、是"**持续趋近的 desired outcome**"——而不是"创建前必须先决定粒度" |
| 6 | **术语陷阱** | **DB 的 atomic（ACID）＝全有或全无**；而"原子动作 / 原子笔记"指的是**不可再分的粒度**。两者不是一个概念——kiln 的原子动作**没有事务语义**（一条动作失败不会被回滚）。ACID 这个词是 Härder & Reuter 1983 造的；Gray 1981 已提出 atomicity/consistency/durability |

### 5. 对 kiln 的映射（机制 + 边界，只给候选，不下裁定）

1. **机制的可复述版**：kiln 不承担复杂性，而是把复杂性**推给"记录"与"组合"**——(a) 原子动作让中断/返工只损失一小步（Simon）；(b) 动作之间不纠结，只保证两端结构（Hickey）；(c) 历史是 log、视图全派生（Kleppmann）；(d) issue 之间用关系组合，语义在**边上**而不在内部（Kernighan & Pike 的"关系 > 个体"）。
2. **边界的诚实版**：它削的是 **accidental**（记录 / 流程 / 状态同步），**不是 essential**（Brooks）——一个困难问题被拆成 20 条 issue 之后，**问题本身没变简单**。
3. **对 temporal decomposition 质疑的回应（待定）**：辩护候选是「**kiln 拆的是记录边界，不是模块边界——它不隐藏知识，而是把过程暴露出来**」；但这条需要单独论证（`design-tensions` 的张力 1 正是它的影子）。
4. **可引入的做法**：Zettelkasten 的「原子性是方向而非规则」——与 `action-granularity` 那条 open issue 同源，可作为"粒度由谁定"的答案候选：**由作者持续趋近，工具只给两端约束**。
5. **必须说清的一条**：kiln 的"原子"**不是事务**（无 ACID 语义）——用户口语里的"原子事务"应当纠正为**原子动作**。

> 来源（经典原文为骨干，其余标注类型）：[McIlroy 1978 foreword（四条 maxims，含 "as yet unknown"）](https://vedang.me/doug-mcilroy-unix-philosophy)（一手转录）｜ [同一 foreword 的引用与书目](https://www.textbookoflinux.com/references/mcilroy1978.html)（二手）｜ [Kernighan & Pike《The Unix Programming Environment》(1984) 原句 ①](https://en.wikipedia.org/wiki/The_Unix_Programming_Environment)（二手引原文）｜ [Simon 1962《The Architecture of Complexity》PDF](https://faculty.sites.iastate.edu/tesfatsi/archive/tesfatsi/ArchitectureOfComplexity.HSimon1962.pdf)（一手，制表匠与 2 万倍）｜ [同文要点（nearly decomposable）](https://www.infraculture.org/2020-09-10-text-notes-architecture-of-complexity)（二手）｜ [Brooks《No Silver Bullet》PDF](https://worrydream.com/refs/Brooks_1986_-_No_Silver_Bullet.pdf)（一手，essential vs accidental）｜ [同文引用与页码](https://jaidevd.com/posts/no-silver-bullet)（二手）｜ [Out of the Tar Pit（state/control/volume）](https://github.com/papers-we-love/papers-we-love/blob/main/design/out-of-the-tar-pit.pdf)（一手论文）｜ [Ousterhout：temporal decomposition 与"关注知识而非顺序"](https://github.com/alysivji/notes/blob/main/software-engineering/philosophy_of_software_design.md)（读书笔记）｜ [同书要点：信息泄漏](https://bagerbach.com/books/a-philosophy-of-software-design)（读书笔记）｜ [Hickey《Simple Made Easy》transcript 引文（complect）](https://blakecrosley.com/blog/engineering-philosophy-rich-hickey)（二手引 transcript）｜ [Kleppmann《Turning the database inside-out》](https://martin.kleppmann.com/2015/03/04/turning-the-database-inside-out.html)（一手作者博客）｜ [Anderson《More Is Different》PDF](https://www.tkm.kit.edu/downloads/TKM1_2011_more_is_different_PWA.pdf)（一手）｜ [Parnas 1972 原文 PDF](https://prl.khoury.northeastern.edu/img/p-tr-1971.pdf)（一手，"no longer correspond to steps in the processing"）｜ [Fowler《MicroservicePremium》](https://martinfowler.com/bliki/MicroservicePremium.html)（一手）｜ [Fowler《MonolithFirst》](https://martinfowler.com/bliki/MonolithFirst.html)（一手）｜ [Prime Video 官方博客](https://www.primevideotech.com/video-streaming/scaling-up-the-prime-video-audio-video-monitoring-service-and-reduces-costs-by-90)（一手，本环境未能取到正文）＋ [二手转述（含两段引文）](https://adventures.nodeland.dev/archive/prime-video-migrates-off-aws-lambda-saving-90-of)、[ByteByteGo 图解](https://bytebytego.com/guides/amazon-prime-video-monitoring-service)｜ [Foote & Yoder《Big Ball of Mud》](https://www.laputan.org/mud/mud.html)（一手）｜ [Waldo et al.《A Note on Distributed Computing》](https://scholar.harvard.edu/waldo/publications/note-distributed-computing)（一手）｜ [Zettelkasten：principle of atomicity 是方向而非铁律](https://zettelkasten.de/posts/principle-of-atomicity-difference-between-principle-and-implementation)（方法论文档）＋ [Zettelkasten 导论（一张卡一个想法）](https://zettelkasten.de/introduction)｜ [ACID 由 Härder & Reuter 1983 造词](https://en.wikipedia.org/wiki/ACID)（二手，含书目）
