# issue: cli-necessity — agent 手握 git 时，kiln 的 CLI 还有多少必要性

- **状态**：ask（进行中）
- **domain**：design

## 背景

本会话里，**agent（我）频繁直接用 `git`，而不是 `kiln`**。最典型的例子：要查一个 issue 的提交里到底写了什么，我敲的是

```
git log -1 --format=%B e98c23f
git show e98c23f --stat --format=""
```

而不是 `kiln show author-attribution 1`。这逼出一个根本问题：**如果 agent（和人）手里都有 git，kiln 这套 CLI 到底凭什么存在？**

## 为什么当时用了 git（自述，供解剖）

1. **要逐字原文**：`kiln path` 给的是摘要（kind / hash / subject）；我要确认提交里**到底写了哪些 trailer**（例如那条已退役的 `object: q1`），只有 `git log --format=%B` 是逐字真值。
2. **手上有 hash**：直接 `git show <hash>` 比"先想 issue id、再想序号"短。
3. **惯性**：agent 默认抓**最通用**的工具；专用包装（kiln）需要"主动想起来用"。
4. **对 kiln 的覆盖没底**：当时不确定 `kiln show` 是否给到逐字 trailer、值不值得为此多敲一次 id + 序号。

> 诚实地说：第 1 条是**真需求**（逐字真值），第 2–4 条是**惯性 + 信任缺口**。后者正是本 issue 要盯的——**如果 agent 没理由想起 kiln，它就不存在。**

## 要回答的问题

1. **增量价值在哪**：agent 有 `git` + 一份**约定文档**，就能自己 `git log --grep` / `git show` / 手写 trailer。kiln 的命令里，**哪些是 git 做不到的**，哪些只是"方便包装"？
2. **命令逐个体检**：
   - `check`（语义完整性校验）——git 没有"issue / action"模型，**git 做不到**？
   - `status` / `graph`（按 domain 聚合、关系图）——git 无此综合？
   - `path`（把一个 issue 的动作重建成有序路径）——git 无此重建？
   - `show` 及各类读取——**纯 git 包装**？
   - `do` / `commit`（规范化写）——对一个能自己拼 trailer 的 agent，还需要吗？价值是否只在**减少格式漂移**？
3. **必要性锚点**：kiln 的价值是不是**只**落在 git 表达不了的四件事——**可执行的模型（不变量）、校验、综合视图、类型化接口**？读 / 单提交展示是否该完全让给 git？
4. **方向**：CLI 该**变薄**（退成"spec + `check`"，读全交给 git）、**变厚**（把结构化能力封成 **MCP**，让 agent 不拼 git 串），还是维持现状？

## 事实记录（本会话的用法分布）

| 用途 | 实际用了 |
| --- | --- |
| 规范写一条动作 | `kiln commit`（始终用 kiln） |
| 概览 / 校验 / 关系图 | `kiln status`、`kiln check`、`kiln graph` |
| 看某 issue 的动作路径 | `kiln path` |
| **取提交逐字原文 / 某提交的 diff** | **`git log --format=%B`、`git show`** |
| 文件操作 / 构建 / 测试 | shell、`go` |

> 现成结论：**kiln 被用到的时候，都是"它独有的视图或规范化写"**；一旦只想拿**逐字真值**，我立刻退回 git。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 维持 | 现有命令不动 | 继续与 git 重合；agent 可能仍绕过 kiln |
| **B** 变薄 | kiln ≈ spec + `check`（+ 少量综合视图）；读 / 单提交展示全让给 git | 承认 CLI 很窄；需要一份权威 spec 文档 |
| **C** 变厚 | 把结构化能力封成 **MCP**（typed tools），agent 不再拼 git 串 | 投入最大；MVP 明确排除过 MCP |
| **D** 分层 | "git 做不到的"留 kiln（`check` / `status` / `graph` / `path` + 规范写），"git 能读的"统一委托 git | 需划清边界 |

## 业内实践调研（collect）

> 一次联网调研的整理，来源见本节末。目的：看"git 之上的工具"各自**加了 git 没有的什么**，从而定位 kiln 的必要性。范围：git 原生 / 近 git 的工具 + **Jira**（作为"语义层在 git 之外"的对照组）。

### 1. 三种"git 之上的工具"，按「数据放哪」分

**A. 数据在 git 里（仓内语义层）**

- **git-bug**：issue 存成 `refs/bugs/<id>` 下的**操作日志**，与代码历史**分离**；离线 / 分布式；CLI 优先（`git bug`）；可桥接到 GitHub/GitLab。加的是 **issue 数据模型 + 免服务器的同步**。
- **git-appraise**：代码评审存进 git notes；分布式评审。加的是**评审工作流数据**（Git Rev News 至今仍在列它——常青但小众）。
- **Radicle**：p2p git 托管；issue / patch 存成仓内 **Collaborative Objects（COB）**，随代码一起复制。加的是**去中心化托管 + 仓内社交产物**。
- **Fossil**：把 issue tracker + wiki + forum + technotes + Web UI **打包进版本控制本身**（"why add forum/wiki/web to your DVCS"）。加的是**全家桶**——但它在 **SCM 层面**统一，不是"在 git 之上"。

**B. 数据也在 git 里，但加的是「人的体验」（不是新数据）**

- **Jujutsu (jj)**：git 兼容；工作区**自动提交**、**无暂存区**、无当前分支、冲突可提交、后代自动 rebase、**操作日志（可 undo）**。加的是 **UX**——数据仍是 git 的对象。**且已被封装成面向 agent 的技能**（"version control for agent workflows"）。
- **StGit / Graphite**：把 commit 当**补丁栈** / 堆叠 PR 来管理。加的是**堆叠式工作流的顺手度**。

**C. 数据在 git 外（服务端语义层 / 平台）**

- **`gh` / `glab`**：包的是 GitHub/GitLab 的 **API，不是 git**（官方原文："gh 专门用于 GitHub……优先用 CLI 而非浏览器"）。加的是**认证、分页、结构化 / 可过滤输出**。
- **Jira**：追踪器 + 工作流 + 报表；**与 SCM 无关**，git 只是它**外链**的一个来源——靠 **smart commit**（提交信息里写 issue key）关联，再把 commits / PR / build 拉进 **development panel**。加的是**工作流模型 + 跨工具报表**；代价是 **issue 数据与仓库分离**（在服务端，要靠集成）。

### 2. 必要性拆成一句话：这些工具靠「加一样东西」活着

- **加新数据模型**（git 没有 issue / review / patch）：git-bug、git-appraise、Radicle、Fossil、Gerrit、Jira。
- **同一份数据、更好的 UX**：jj、StGit、Graphite——这类**与 git 高度重叠**，靠"更好用"而非"git 做不到"存活。
- **访问 git 之外的数据**：gh / glab / Jira——必要性来自"数据不在本地"。

> 对照到 kiln：kiln 属**第①类（新数据模型）**，模型 = **issue / action / relation + 不变量**。它的必要性**只**能落在这四个词上：**模型、校验、综合、规范化写**；凡"git 已经能读 / 能显示的"，都落进"UX 包装"区——和 jj 一样，只能靠"更好用"立足。

### 3. kiln 占的那个格子：调研里**没人占**

把"数据模型在 git 里"再细分一层——**issue 的历史，用不用代码提交本身**：

| 工具 | issue 历史存哪 |
| --- | --- |
| git-bug | **独立**的 `refs/bugs` 操作日志（与代码分离） |
| Radicle / git-appraise | 仓内的独立对象（COB / notes） |
| Fossil | SCM 自带表（非提交 trailer） |
| Jira / gh / glab | **仓库之外**（服务端数据库） |
| **kiln** | **就是代码提交本身**（`issue:` / `action:` trailer） |

**没有一个是"issue 的生命周期 = 那串代码提交"**。kiln 的格子是空的：比 git-bug / Radicle 少一套存储（issue 历史就是正常提交，clone 即有）；比 Jira 少一个服务端。**代价**：kiln **只能读**，不能像 Jira / git-bug 那样**脱开代码去改 issue**——改 issue 就得再提交一步。

### 4. 对"必要性"最相关的 6 条

1. **必要性 = "git 表达不了的东西"**：调研里所有工具都符合——要么加**数据模型**、要么加 **UX**、要么加**对外部数据的访问**。kiln 只有押在第①类上才站得住。
2. **能立住的四件事**：`check`（不变量校验）、`status`/`graph`/`path`（模型之上的综合视图）、`do`/`commit`（规范化写）——**git 都不会做**。
3. **会失守的地方**：`show` 与 `path --stat` 落在"UX 包装"区——与 `git show` / `git log --stat` 重叠，**只能靠更好用来赢**（正如 jj 之于 git）。
4. **kiln 的格子是空的**（§3）："issue 生命周期 = 代码提交"这一定位，调研的十来个工具里**没人占**。这是它相对 Jira（要服务器）与 git-bug（要另一套存储）的真实差异点。
5. **反例的启示——jj 已盯上 agent**：jj 用**同一份 git 数据**做出更好 UX，并已封成**面向 agent 的技能**。→ 若 kiln 只做"git 的 UX 包装"，就是在和 jj 正面竞争，且必须**明显更好**。
6. **Jira 的教训**：语义层放在 git **之外**，换来工作流 / 报表，代价是**数据与仓库分离、要靠集成**。kiln 走 in-repo 是**反向选择**——其存在理由正是"**不要服务端**"（同 git-bug / Radicle / Fossil）。

> 来源：[git-bug / git-appraise（Git Rev News 126）](https://git.github.io/rev_news/2025/08/31/edition-126/) ｜ [Radicle（P2P 替代 GitHub）](https://laoutaris.org/blog/radicle/) ｜ [Radicle（arXiv 2609.13555）](https://arxiv.org/pdf/2609.13555) ｜ [Fossil：Why Add Forum, Wiki, and Web Software To Your DVCS](https://www.fossil-scm.org/home/doc/edfa01d9/www/whyallinone.md) ｜ [Fossil：Bug-Tracking In Fossil](https://fossil-scm.org/home/doc/trunk/www/bugtheory.wiki) ｜ [Gerrit：Changes / patch sets](https://review.opendev.org/Documentation/concept-changes.html) ｜ [Jujutsu：Git comparison](https://docs.jj-vcs.dev/latest/git-comparison/) ｜ [StGit](https://stacked-git.github.io/) ｜ [Graphite CLI](https://www.graphite.com/docs/cli-overview) ｜ [GitHub CLI：vs Git](https://docs.github.com/en/enterprise-server@3.13/github-cli/github-cli/about-github-cli) ｜ [Jira：Configure development tools](https://support.atlassian.com/jira-software-cloud/docs/configure-development-tools/) ｜ [Jira：Reference work items in your development work](https://support.atlassian.com/jira-software-cloud/docs/reference-issues-in-your-development-work/)

## collect（续）：jj（Jujutsu）vs kiln

> 第二轮调研，专比一个对象：**jj**。三个角度——设计哲学、命令层面、面向 agent 的 UX。来源见本节末。

### 一句话

**jj 改的是「怎么动历史」，kiln 加的是「git 缺的模型」——两者正交，可叠加。**

### 1. 设计哲学

| | **jj** | **kiln** |
| --- | --- | --- |
| 一句话 | 给同一批 git 对象一个更好的**操作界面** | 给同一批 git 提交一个 git 没有的**语义模型** |
| 抽象单元 | change / commit（**历史编辑**） | issue / action（**问题求解路径**） |
| 对 git 的态度 | **重做界面**（可替代 git 做日常操作） | **叠加语义**（读写都委托 git） |
| 数据 | 仍是 git 对象（另存 change id / op log） | 仍是 git 提交（+ trailer） |
| 朝向 | 让"改历史"顺手、可撤销 | 让"看 / 校验问题路径"成为可能 |
| **共同根** | **一切皆 commit、无隐藏状态**（都反 Jira 式外部库） | 同左 |

分歧在**对历史的态度**：jj 视历史为**流体**（随时改写、后代自动 rebase）；kiln 视动作历史为**问题的记录 / 证据**。

> 关键交叉点：jj 的流体历史会让 **hash 漂移**，它的解法是 **change ID「跨改写稳定」**；kiln 的解法（本会话）是 **用序号寻址、不用 hash**。→ **jj 的存在反而印证了 kiln 的两个选择**，且 jj 的 change-id 与 Gerrit 的 `Change-Id` 是同一思路（"提交内容里的稳定 id"）。

### 2. 命令层面

| 关注点 | **jj** | **kiln** |
| --- | --- | --- |
| 看一段历史 | `jj log -r <revset>`（**查询语言**） | `path <issue>`（按 issue 聚合，非查询语言） |
| 看单个变更 | `jj show <rev>` / `jj diff -r <rev>`（自己渲染） | `show <issue> n`（**委托 `git show`**） |
| 写 | `jj describe` / `new` / `commit` / `squash` / `split` | `do` / `commit`（**生成规范化 trailer**） |
| 校验 | **无此层**（只保证仓库一致性） | `check`（**模型不变量**） |
| 寻址 | **revset**（`::@`、`diff_lines()`、模板 `-T`） | **序号**（弱、但稳且免费） |
| 撤销 | `jj undo` / `jj op log` / `jj op restore` | 无（交给 git） |
| 状态 | `jj st`（**工作区**状态） | `status`（**全库 issue** 概览，语义不同） |

四条命令层面的观察：

1. **jj 有 revset 查询语言，kiln 没有**——kiln 的选择很粗（issue id + 序号）；`path <issue>` 相当于 `jj log -r "issue(...)"` 的一个特例。若 kiln 将来要更强的查询，**可参考 revset，不必自造**。
2. **jj 是强 writer**（改写历史是核心能力）；**kiln 是 reader + 规范 writer**——面向"读 + 校验"的 kiln 在"改"上刻意退让给 git。
3. **`jj show` 与 `kiln show` 同名同义**（一个对象 + 它的变更）——反过来印证 `action-diff` 里选 `show` 而非 `diff` 是对的。
4. **结构化输出是 kiln 的短板**：jj 有模板（`jj log -T '<template>'`）；kiln MVP 无 `--json`——对 agent / 脚本不友好。

### 3. 面向 agent 的 UX

jj 正被大量"AI Agent Skills"包装（社区已成气候），卖点是**让 agent 安全地写 / 改仓库**：

- **无暂存区**（工作区自动成为提交）→ agent 不会留下"半暂存"的坏状态；
- **每步操作自动快照 + `jj undo` / `op log`** → 敢试错、可回滚；
- **冲突可提交、命令不中断** → agent 不被冲突卡死；
- **change ID 跨改写稳定** → agent 的引用不因 rebase 失效；
- **revset + 模板** → 精确查询 + 可解析输出。

→ 这批特性解决的是「**仓库被写坏**」这一类失败模式。

kiln 面向 agent 的 UX：

- `do` / `commit` 产出**规范化消息** → 减少 agent 自由拼字符串出错；
- `check` 是**护栏**：agent 能拿它校验自己的产物并自纠——**真正的 agent-UX 价值**；
- 短板：输出偏人读、无 `--json`；模型词汇（issue / action）需 agent 额外学。

> **结论：二者在 agent 场景解决的失败模式不同**——**jj 防"仓库写坏"，kiln 防"语义写错 / 读错"**。因此**可叠加**：agent 用 jj 安全地写提交，用 kiln 校验语义 + 复盘路径。

### 4. 对 cli-necessity 的启示

1. **jj 证明"纯 UX 层"也能独立成立**——但它靠**明显更好**，且**不提供 git 没有的模型**。kiln 若也走 UX 路线，就要和 jj 比"更好"；kiln 的护城河应在**模型 + 校验**（jj 没有）。
2. **"稳定标识"是行业共识**：Gerrit `Change-Id`、jj change ID 都在对抗 hash 漂移。kiln 用**序号**是同一动机的另一解——更弱但免费。
3. **`check` 是 jj 明确没有的一层**——坐实了收集到的结论：**不变量校验是 kiln 最不可替代的部分**。
4. **revset 值得借鉴、不必自造**：kiln 若需查询能力，可把选择器做得像 revset；否则维持"粗选择 + 精确校验"。
5. **结构化输出是面向 agent 的真实短板**：jj 有 `-T` 模板，kiln 无 `--json`。若 agent 是主要用户，这比"命令重叠"更该先补。

> 来源（官方 jj 文档为主；"面向 agent"的包装来自社区技能页，标 ★）：[jj：Git comparison](https://docs.jj-vcs.dev/latest/git-comparison/) ｜ [jj：Git command table](https://docs.jj-vcs.dev/latest/git-command-table/) ｜ [jj：Revsets](https://docs.jj-vcs.dev/v0.40.0/revsets/) ｜ ★ [jj 技能：change ID 跨改写稳定](https://skills.cat/skills/dashed/claude-marketplace/jj) ｜ ★ [onevcat-jj（面向 agent 的 jj）](https://www.skill4agent.com/en/skill/onevcat-skills/onevcat-jj) ｜ ★ [hivebook：jj with stable change IDs](https://hivebook.wiki/wiki/jujutsu-jj-git-compatible-vcs-with-stable-change-ids) ｜ [15 Years of Git, Then Jujutsu](https://dev.to/oukayuka/15-years-of-git-then-jujutsu-why-i-cant-go-back-2o6n)

## hypothesize：VCS adapter（git 可选、可接 jj）

> 结论先行：**把"git 硬编码"提升为"可插拔的仓库后端"**——接口原型其实**已经存在**（`internal/git.Runner`），工作只是①把它提升为后端中立的 `vcs.Repo`、②加一个选择/探测机制、③（将来真用 jj 时）补一个 jj 适配器。渲染继续**委托给后端**（现在已是如此）。
> 分阶段：**P1 = 只做抽象与选择（行为不变，便宜）**；**P2 = jj 适配器（真用 jj 再上，避免投机）**。

### 1. 现状：seam 已经存在，只是名字里带 git

- [`internal/git`](file:///c:/Users/jacki/kiln/internal/git/git.go) 里已有 `Runner` 接口：`Log / Commit / Show / ShowStat`，外加 `ExecRunner`（shell 出 git）。
- [`cli.env`](file:///c:/Users/jacki/kiln/internal/cli/cli.go#L115-L123) 已把 `git.Runner` 当**接口**持有——**这正是 adapter 的雏形**，只是包名/类型名把它和 git 绑死了。
- 引擎只消费 `[]git.Commit`（纯数据），**根本不碰 exec**。→ 后端替换对引擎**零影响**。

### 2. 目标抽象：`vcs.Repo`（后端中立）

```go
// package vcs：后端中立的仓库抽象
type Commit struct {
    ID      string    // 后端给的不透明标识（git: hash；jj: change id / commit id）
    Date    time.Time
    Message string
}

type Repo interface {
    Log() ([]Commit, error)                          // 全部提交：id + 时间 + 消息
    Show(id string) (string, error)                  // 单条变更原文（后端渲染）
    ShowStat(id string, short bool) (string, error)  // 单条变更的统计（后端渲染）
    Action(message string) error                     // 记录一条 kiln 动作
}

// Open 按偏好选择后端；prefer = "auto" | "git" | "jj"
func Open(dir, prefer string) (Repo, error)
```

- `Commit(message)` **改名 `Action(message)`**：既中立，又正好对上 kiln 的词汇（"记录一条动作"）。
- `ID` 保持**不透明**——kiln 对外仍用**序号**寻址（`action-diff` 的决定），所以不依赖任何后端的 id 稳定性。

### 3. 方法映射（预期；**待 `test` 用真实 jj 验证**）

| `Repo` 方法 | git | jj（预期） |
| --- | --- | --- |
| `Log` | `git log --format=%H%x1f%aI%x1f%B%x1e` | `jj log -r 'all()' --no-graph -T '<id/date/desc 模板>'` |
| `Show(id)` | `git show <hash>` | `jj show <rev>` |
| `ShowStat(id, short)` | `git show --stat` / `--shortstat --format=` | `jj diff -r <rev> --stat`（形态待验证） |
| `Action(msg)` | 暂存后 `git commit -m` | `jj commit -m`（把工作区变更**终结**成一条带该消息的提交） |

### 4. 三处"硬分歧"（抽象真正要吞掉的东西）

1. **造提交的语义**（最难）：
   - git：**先暂存、再 `commit`**；kiln 的 `commit` 只提交**已暂存**的内容。
   - jj：**无暂存区**，工作区**本身就是**当前变更；`jj commit -m` 会把**整个工作区变更**终结成提交。
   - → 契约要定义成"**用这条消息记录当前变更**"，并把差异**写进文档**，而不是假装两者一致。
2. **历史范围**：git `log` 默认给全量；jj `log` 默认只给一个 revset。→ 适配器**必须显式传 `all()`/`::`** 才能与 git 对齐。
3. **仓库探测**：jj 常用 **colocated** 仓（`.jj/` 与 `.git/` **并存**）。→ 自动探测必须**`.jj` 优先于 `.git`**，否则会**静默**读成 git 的视图（看到不同的 commit 集合、看不到 jj 的 change id）。

### 5. 为什么模型能"免费"移植

kiln 的模型**全在 commit message 里**（`issue:` / `action:` 等 trailer），不在对象存储里。两个后端都**原样保留提交消息** → **模型天然与后端无关**，换后端不用迁数据。

> 这也把 cli-necessity 的结论落地成一句架构话：**kiln 的产品是「模型 + 校验 + 综合」，VCS 只是可换的底座。**

### 6. 选择机制

- `.kiln.json` 增 `"vcs": "auto" | "git" | "jj"`（默认 `auto`）；
- `auto`：有 `.jj/` → jj，否则有 `.git/` → git，都无 → 报错；
- 可选：`kiln <cmd> --vcs=jj` 命令行覆盖。

### 7. 成本 / 风险 / 非目标

- **风险**：造提交语义可能"泄漏"（适配器吞不干净）→ 这是本设计最脆的一处。
- **风险**：jj 渲染格式与 git 不同 → `show`/`path --stat` 的示例与测试**绑定 git 形态**的地方要**中立化**（把正文当**不透明文本**）。
- **风险**：维护两个后端；`path` 里现在印"短 hash"，jj 下要印"短 id"。
- **反方**：若**只用 colocated 仓**，现有 git 适配器**已能工作**；jj 原生适配器的净收益是**非 colocated 仓 + 稳定 change id + 原生渲染**。→ 所以 **P2 应等真用 jj 再上**。
- **非目标**：本轮不改代码；不引入 jj 依赖；不动引擎与模型。

### 8. 与 cli-necessity 的呼应

- 该 issue 的结论是"kiln 的价值 = 模型 / 校验 / 综合 / 规范写"。**adapter 让这句话的边界更清楚**：这四件事与后端无关，git/jj 只是 substrate。
- 收集到的"jj 与 kiln 可叠加"也在此具象化：jj 提供**安全地写**（无暂存、可 undo、change id 稳定），kiln 提供**语义校验与复盘**。

## 非目标

- 本轮不改代码、不新增 / 删除命令。
- 不评估 MCP 的实现细节。

## 备注

由用户在会话中直接提出：看到 agent 用 `git log --format=%B` 而非 `kiln path`，追问"基于什么考虑；如果 agent 都在用 git，kiln 的 CLI 有多少必要性"。这是迄今最**根本**的一个问题——它反过来质疑 kiln 这个项目的立足点。
