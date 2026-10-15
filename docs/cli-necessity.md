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

## hypothesize（续）：后端选 git 还是 jj？——jj 分两种模式

> 说明：按 jj 官方把 jj 分**colocated** 与**非 colocated（独立）**两种模式来比（`jj git colocation status` / `--colocate` 佐证）。结论先行：**colocated 是"成本最低、先支持"的模式**（kiln 的 git 适配器直接可用）；**独立模式才真正需要 jj 适配器**。

### 0. 先厘清"两种模式"

| 模式 | 工作区里有什么 | 本地 git 命令能用吗 |
| --- | --- | --- |
| **jj · colocated** | `.jj/` **+** `.git/`（并存） | **能**——git/jj 共用同一工作区，是"安全、可逆的迁移路径" |
| **jj · 非 colocated（独立）** | 只有 `.jj/`（git 仓库在 jj 内部存储里） | **不能**——工作区没有可用的 `.git` |

### 1. 三种后端 × 关键维度

| 维度 | **git** | **jj · colocated** | **jj · 独立** |
| --- | --- | --- | --- |
| kiln 如何探测 | `.git/` | `.jj/`+`.git/`（**须 `.jj` 优先**） | `.jj/`（无 `.git`） |
| kiln 能否用 **git 适配器** | ✅ | ✅ | ❌ |
| kiln 能否用 **jj 适配器** | ❌ | ✅ | ✅ |
| 动作标识 | commit hash（**改写即漂移**） | change id（稳定）+ commit id | 同左 |
| 历史形态 | 追加为主，改写是显式 / 危险操作 | **流体**：改写自由、后代自动 rebase | 同左 |
| 造提交 | 先暂存 + `commit` | 工作区即变更，`jj commit -m` | 同左 |
| 渲染 | `git show` / `--stat` | `jj show` / `jj diff --stat`（格式不同） | 同左 |
| git 生态（IDE / CI / 托管 / `gh`） | ✅ 全通 | ✅ **全通**（工作区有真 `.git`） | ⚠ 本地 git 工具不可用（push 仍是标准 git） |
| 对 kiln 模型的保真 | trailer 原样保留 | 同左 | 同左 |

### 2. 优势 / 不足

**git 后端**

- **优势**：通用（IDE / CI / GitHub / `gh` 全认）；零额外依赖；历史**追加为主** → 与 kiln"动作历史 = 证据"的取向**天然一致**；适配器已实现。
- **不足**：hash 随改写漂移（kiln 靠序号绕过）；**改写历史危险**（rebase / amend）→ agent 容易写坏；**无一等公民的撤销**。

**jj 后端（两模式共有）**

- **优势**：**change id 稳定**（跨改写）→ 比 hash 更适合当标识；**无暂存区 + 操作日志 + 可 undo + 冲突可提交** → **对 agent 明显更安全**；revset / 模板 → 可精确查询；正是收集到的"**jj 与 kiln 可叠加**"。
- **不足**：**历史是流体** → 与 kiln"证据"取向**有张力**（记录随时可被改写）；渲染格式与 git 不同 → 测试 / 示例要**中立化**；多一个二进制依赖；生态较新。

**jj · colocated 独有**

- **优势**：**兼容甜点**——kiln 的 **git 适配器照常可用**（连 P1 都不必做），同时用户享受 jj 的 UX；IDE / CI / 托管**全认**；"安全可逆"（随时能退回纯 git）。
- **不足**：`.jj` 与 `.git` 并存 → **探测歧义**（不显式偏好就会**静默**读成 git 视图）；两边同时操作可能互扰。

**jj · 独立 独有**

- **优势**：jj 的完整能力，无 git 工作区牵绊（更"纯"）。
- **不足**：**kiln 只能用 jj 适配器**（没有退路）；**本地 git 工具 / IDE / CI 的本地部分失效**（远端托管不受影响，因为 push 仍是标准 git）。

### 3. 对 kiln 的结论（倾向，待 `test`）

1. **colocated 是"先支持、成本最低"的模式**：它让 **git 适配器直接可用** → **P1（抽 `vcs` 包）即可覆盖**，无需 jj 适配器。建议把 colocated 作为**推荐给 kiln 用户的 jj 用法**。
2. **独立模式才真正需要 `vcs/jj`**（P2）。
3. **jj 的 change id 更适合 kiln 的标识需求**（稳定），但 kiln 已用**序号**寻址 → 收益**边际**，除非将来 `path` / `show` 要显示稳定 id。
4. **最大张力**：**jj 把历史当流体，kiln 把历史当证据**。序号寻址是对冲（改写后重算但仍自洽），但"**证据可被改写**"这点**适配器解决不了**——是**取向层面的取舍**，必须写清楚。
5. **探测规则**（三后端）：**`.jj` 存在 → jj**（是 colocated 还是独立，看工作区**是否有 `.git`**）；否则 `.git` → git；都无 → 报错。`vcs.Open` 应返回**后端 + 模式**，而不只是后端名。

### 4. 一个顺带的设计钩子

jj 的 **operation log** 能告诉你"**历史被改写过**"。→ kiln 将来可据此**提示"这条动作的证据已被重写"**——这是 git 给不了、而 jj 独有的信息。留作候选，不在本轮。

### 5. 风险 / 非目标

- **风险**：两模式让探测与行为分支变多；`vcs.Open` 的返回要带上**模式**。
- **非目标**：本轮不改代码；不引入 jj 依赖。

> 来源：[jj：Git compatibility（colocated）](https://docs.jj-vcs.dev/latest/git-compatibility/) ｜ [jj FAQ：Should I colocate my repository?](https://docs.jj-vcs.dev/latest/faq/) ｜ [jj CLI：`--colocate`](https://docs.jj-vcs.dev/v0.29.0/cli-reference/)

## test：落地 VCS adapter，接入 jj（colocated）并验证

### 落地内容

- 新增 **`internal/vcs`**：后端中立抽象——`Commit{ID,Date,Message}` + `Repo{Log, Show, ShowStat, Action}` + `Detect(dir, prefer)`（auto 下 **`.jj` 优先于 `.git`**）。
- **`internal/git` → `internal/vcs/git`**：`Runner` 接口换成 `vcs.Repo`，`Commit()` 改名 `Action()`；行为不变（仍委托 `git show [--stat|--shortstat] [--format=]`）。
- 新增 **`internal/vcs/jj`**：`jj log --no-graph -r 'all() ~ root() ~ @' -T '<id/时间/描述>'` 取历史；`jj show -r`、`jj diff -r --stat`（末行当 shortstat）、`jj commit --message`。
- `env` 改为持有 `vcs.Repo`（**惰性打开**，故 `do` 在仓外仍可用）；`config` 增 `VCS(dir)` 读 `.kiln.json` 的 `"vcs"`（默认 `auto`）。
- 顺带把 `model.Action.Hash` 改名 `ID`（中性）。

### 验证

- `gofmt -l` 干净；`go vet` 通过；`go test ./...` **全绿**（新增 `vcs.Detect` 5 例 + jj 解析 3 例 + `config.VCS` 2 例）。
- **真机 jj 0.45.1**，临时 **colocated** 仓（`.jj` + `.git` 并存）：
  - **auto 探测 → jj**（`.jj` 优先）；`status` / `path` / `path --stat` / `--shortstat` / `show` / `check` 全部正常。
  - **两后端对比**（同一仓、同一提交）：
    - `path --stat`：git → `1 file changed, 1 insertion(+)`；jj → `…, 0 deletions(-)`。
    - `show`：git → `commit/Author/Date` + `diff --git`；jj → `Commit ID/Change ID/Bookmarks/…` + jj 自有 diff 格式。
    - 各自后端**原样渲染**，kiln 只负责编排——正是"渲染委托后端"的预期。
  - **Action 在 jj 上可用**：`kiln commit`（→ `jj commit --message`）产出真实 git 提交 `3a6a9ba`，`git log` 可见；`path` 随即显示 `ask → collect` 两步；jj 的**新工作区提交 `@` 被正确忽略**。
- **真仓回归**：本仓无 `.jj` → auto 仍走 git，`path` / `status` / `check` 行为不变，exit=0。

### 发现 / 对假设的修正

1. **colocated 探测必须 `.jj` 优先**——已证实，否则会静默读成 git 视图；`TestDetectAutoPrefersJJ` 锁定该点。
2. **jj 的工作区提交 `@` 是"额外动作"风险**（jj 与 git 的关键语义差）：用 revset `~ @` 排除，使"只算已提交动作"与 git 对齐——**这是本次最重要的一处适配**。
3. **jj 没有 `--shortstat`**：用 `diff --stat` 的末行近似，输出形态略异（多 `, 0 deletions(-)`）。
4. **造提交差异被证实**：git 只提交**已暂存**内容；jj 终结**整个**工作区变更（契约差异见上段 hypothesize §4.1）。
5. **colocated 下 git 适配器本可直接用**：印证了假设"§3.1"——纯 colocated 其实不"必须" jj 适配器；jj 适配器的净收益是**非 colocated 仓 + 原生渲染**（本次为验证而实现）。

## test（续）：实测 git vs jj 后端差异与 jj 的优势

> 用测试仓实证。实验 1 建**纯 jj 仓**（`--no-colocate`），其余在 **colocated** 仓。

### 实验与结果

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | 纯 jj 仓（`jj git init --no-colocate`） | 工作区只有 `.jj`，`git log` **直接失败**（not a git repository）；**kiln + jj 适配器完整可用**（commit / path / show）→ **jj 适配器是真必要**，不是可选 |
| 1b | `jj git init`（默认） | **jj 0.45 默认就是 colocated**（`.git` + `.jj` 同时建）——与"独立仓需要显式建"的直觉相反 |
| 2 | 造提交（有**未暂存**改动） | **git 后端失败**（`git commit: exit 1`，须先 `git add`）；**jj 后端成功**（`jj commit --message` 终结工作区变更）→ **jj 免掉暂存步** |
| 3 | 改写历史（`jj describe` 改 collect 的标题） | **change_id 不变**、**commit_id 漂移**（`3a6a9ba`→`9b5185c`）；**后代自动 rebase**（test `32a13db`→`062ac26`，"Rebased 2 descendant commits"）；kiln `path` 的短 id 随之漂移，但**序号寻址仍自洽**；`jj op log` 记下"describe …"这次改写 |
| 4 | 删除 + 撤销（`jj abandon` → `jj undo`） | 动作**秒删**，kiln 视图立即少一步；`jj undo` **原样恢复**。abandon 触发冲突时 jj **把冲突记进提交、命令不中断**，kiln 仍能照读 |

### jj 的优势（本次实测支持）

1. **无暂存区** → agent 少一步、少一个失败点（实验 2）。
2. **change_id 跨改写稳定** → 真正的稳定标识；**git 没有等价物**（hash 必漂，实验 3）。
3. **操作日志 + `undo`** → 敢试错、可回滚（实验 4）；op log 还能告诉你"**历史被改写过**"（git 给不了）。
4. **冲突可提交、命令不中断** → agent 不被卡死（实验 4）。
5. **纯 jj 仓也能用** → 不依赖 `.git`（实验 1）。

### jj 的代价（也实测到）

1. **历史是流体** → 动作"证据"可被**任意改写 / 删除**（实验 3/4）——与 kiln"记录"取向的张力**被实证**。
2. **commit_id 漂移** → kiln 现在印的短 id 会变（实验 3）。
3. **渲染形态不同** → `path --stat` / `show` 与 git 不同（见上一段 test）。
4. **abandon 有副作用** → 自动 rebase 会引发冲突（实验 4），"删一个动作"并不总是干净。

### 对 kiln 的结论

- **git 后端**：稳、通用、与"历史 = 证据"取向一致；代价是造动作要先暂存、改写危险。
- **jj 后端**：对 agent 更友好（无暂存、可 undo、change id 稳定、冲突不阻断）；代价是**历史可被轻易改写**。
- **改进候选（实测催生）**：jj 适配器可改用 **change_id** 作 `Commit.ID`（跨改写稳定）→ `path` 的短 id 不再漂移；`show` / `stat` 用 change_id 寻址即可（jj 支持）。代价：ID 语义变成"jj change id"，与 git 的 hash 不再同形。
- **仍成立的张力**：除非把"动作路径"当**可改写草稿**，否则"证据可被改写"要靠 **op log 校验**或**约定不改写**来兜——这是取向取舍，适配器解决不了。

## test（再续）：并发——不同 issue 交给不同 agent，jj vs git

> 同一仓、两个「agent」并行处理**不同 issue**，考察**并发冲突**。jj 用 **workspace**、git 用 **worktree**。

### 场景与结果

| 场景 | **jj**（workspace + `jj new`） | **git**（worktree + `git merge`） |
| --- | --- | --- |
| 初始化 | `jj workspace add w1/w2`：**共享同一 store**，各带自己的**工作区提交 `@`** | `git worktree add a/b -b a -b b`：各带分支与 `HEAD` |
| 两 agent 改**不同文件** | 各自 `kiln commit`（→ `jj commit`）→ 干净 | 各自 commit → 干净 |
| 合并（不同文件） | `jj new <A> <B>` → **exit 0**，合并提交落地，`a.md`+`b.md` 都在，**kiln 同时看到 2 个 issue** | `git merge b` → 自动合并干净 |
| 两 agent 改**同一文件** | `jj new <A> <B>` → **exit 0、不阻断**；冲突**记进合并提交**（`(conflict)`），`jj st` 报 unresolved；**kiln 照读 2 issue / 4 动作** | `git merge b` → **CONFLICT、exit 1、中止合并**；工作区进入 unmerged 态、**无合并提交**，必须手工解决并 `git commit` |
| 机制 | 冲突是**提交的一等内容**（可推迟解决，命令不停） | 冲突是**工作区的临时态**（不解决就没有提交） |
| 次工作区里能用另一套 VCS 吗 | **不能**——w2 里 `git log` 直接 `fatal: not a git repository` | 能（worktree 是 `.git` 文件） |

### 结论

1. **并行能力**：jj 用 **workspace**（共享 store、各自 `@`）天然适合「一仓多 agent」；git 用 **worktree + 分支**。两者都能并行——但…
2. **冲突处理是分水岭**：**jj 不阻断**（冲突进提交、可延后解决，agent 继续干活）；**git 阻断**（merge 中止、必须立即解决才有提交）。→ 对 agent 场景，**jj 明显更安全**（呼应前面 test 的"冲突可提交、命令不中断"）。
3. **对 kiln**：两种情形下 kiln 的**读视图都不受影响**（只要提交存在就能重建）；差别全在**写侧**——jj 让「并行 + 冲突」不成为卡点。
4. **注意**：jj 的次 workspace **不是 git 仓**，那里 `kiln` 走 **jj 后端**（`auto` 探测到 `.jj`）；git 侧 worktree 仍走 git 后端。

### 风险 / 备注

- 本次未测「两 agent 落在**同一个** workspace」——那会争用同一个 `@`，属另一类问题（应为「一 agent 一 workspace」）。
- jj 的冲突标记里带「from/to change-id」，比 git 的 `<<<<<<<` 更能说明**谁改了什么**。

## test（再续 2）：必须手动声明 workspace 吗？+ jj 冲突机制与 agent 处理

### A. 不声明 workspace / worktree 会怎样（实测）

| | **jj**（同一工作区，无 workspace） | **git**（同一工作区，无 worktree） |
| --- | --- | --- |
| 两 agent 改**不同文件** | **互相吞并**：A 的 `kiln commit` 把 B 的**未提交改动一起提交**（实测 alpha 动作同时含 `a.md`+`b.md`）；B 的动作随之变成**空提交**（`f611e26`） | **能隔离**：`git add a.md` 只提交 A 的文件，B 的 `b.md` 仍游离（`?? b.md`） |
| 原因 | **jj 无暂存区** → 任何快照 / 提交都捕获**整个**工作区 | 有**暂存区** → 能只挑自己的文件 |
| 同文件 | 直接并进同一次提交 | stage 整个文件 → 也会混 |
| 结论 | **并行 agent 必须手动 `jj workspace add`——不是可选** | worktree 是"更强隔离"；暂存区提供**部分**隔离 |

> 反直觉点：**jj"无暂存区"对单个 agent 是便利，对同工作区的多 agent 却是隐患**——因为它没有"只提交我这份"的手段。这正是 `jj workspace add` 非用不可的原因。

### B. jj 的冲突机制（补充）

- 冲突**存在提交里**（不是工作区临时态）：合并提交被标 `(conflict)`，命令**从不因冲突失败**；冲突随 rebase / clone 迁移。
- 标记由 `ui.conflict-marker-style` 决定：`diff`（默认：一侧全文 + 另一侧 diff）/ `snapshot`（两侧全文 + base）/ **`git`**（标准 `<<<<<<< ||||||| ======= >>>>>>>`，实测生效）。
- 解决三条路：
  1. **手写文件**到最终内容 → jj 下次命令**自动快照即判定解决**（实测）；
  2. **`jj resolve --tool=:ours | :theirs`** → **非交互选边**（实测 `:theirs` 一条命令解成 side #2）；
  3. `jj resolve`（配 3-way 工具，交互式）。
- `jj resolve --list` → 机器可读的冲突清单（路径 + 元数）。

### C. agent 处理冲突的推荐流程

1. 合并后先 `jj resolve --list`；
2. 逐文件：能选边 → `:ours` / `:theirs`；要真合并 → 读 marker、写回最终内容；
3. 复查 `resolve --list` 为空、`jj st` 无 `(conflict)`；
4. **再** `kiln commit` 落 action——**别把冲突带进动作提交**；
5. 延后可以（冲突在提交里、不阻断），但**结案前必须清干净**（冲突会渗进带 trailer 的子树）。

### D. 对 kiln 的含义

- kiln 的**读视图**在冲突 / 并行下都**不受影响**；风险全在**写侧**。
- **并行 agent 的正确姿势**：**一 agent 一 workspace**（jj）/ 一 worktree（git）。否则——如 A 节实测——"动作 = 提交"会把**别人的改动算进你的动作**，动作归属被静默打乱。

## hypothesize：让 agent 一键拿到隔离区，并让"污染"可校验

### 0. 要解决的具体伤害（实测）

同工作区里 A 的 `kiln commit` 把 B 的未提交文件**一起提交** → **动作归属被静默打乱**。目标两条：**① 让隔离成为默认好走的路；② 万一没隔离，污染要能被发现。**

### 1. 先划边界：谁编排、谁提供原语

- **编排**（起几个 agent、谁做什么）= **外部**（harness / 人）。kiln 不该管。
- **原语**（"给我一个隔离的工作区"）= VCS 的事，但**后端不同、命令不同**（jj `workspace add` vs git `worktree add -b <branch>`）→ **kiln 的 backend 抽象正好该吞掉这个差异**。

### 2. 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只立约定 | 文档写明"一 agent 一 workspace"，命令用原生 | 零；但 agent 会忘 → **静默污染** |
| **B** 薄封装 | `kiln workspace new / list`（后端中立） | 小；从"只读 + 呈现"扩到"轻写" |
| **C** B + issue 绑定 + `check` 规则 | workspace 绑定 issue；`commit` / `check` 能发现"串档" | 更强；更"管"；要设计绑定落点 |
| **D** kiln 自己编排 agent | — | **越界，否** |

> 倾向：**B 起步、C 为增强**。

### 3. 具体形态（B：后端中立的原语）

- 扩 `vcs.Repo`：`NewWorkspace(name) (path, error)` / `ListWorkspaces()`。
  - git → `git worktree add <path> -b <name>`；jj → `jj workspace add <path>`。
- 命令：`kiln workspace new [--for <issue>]` → 建隔离区、打印路径、提示 `cd` 过去；`kiln workspace list`。
- **标签按 issue 绑，而非按 agent 身份**——避免重开 `author-attribution`（那题已判"不做"）。

### 4. 增强（C）：把隔离变成**可校验**

- 在 workspace 内写绑定（如该 workspace 的 `.kiln.json` 记 `issue: <x>`，或仓库外的索引）。
- 于是：
  - `kiln commit` 在**绑定** workspace 里：若 action 的 `issue:` ≠ 绑定 issue → **warn**；若改动触及**不属于该 issue 的文件** → **warn**（正对"被吞并"那类实测伤害）。
  - `kiln check` 增规则：**同一 workspace 出现多个 issue 的动作** = 可疑（污染信号）。
- 效果：把"污染"从**静默**变**可见**。

### 5. 为什么这仍符合 kiln 的取向

- workspace 是**底座**（同提交层），**不是**耐久层；kiln 只做**薄封装 + 校验**，**不做编排**。
- 从"只读 + 呈现"扩到"**轻写**"（建隔离区）——与 `do` / `commit` 同类，不越界。
- 与 `vcs.Repo` 抽象一脉相承：**后端可换，隔离方式跟着换**。

### 6. 待定 / 非目标

- **绑定落点**：workspace 内配置 vs 仓库外索引？
- **命名映射**：git worktree 的分支名、jj workspace 的路径如何与 issue 名对齐？
- **是否强制**（"没有隔离区就不许 commit"）？——倾向**只 warn、不强制**。
- **非目标**：本轮不改代码；**不重开 author-attribution**。

## hypothesize（续）：按 issue 建隔离区（静默化），放弃文件归属检测

> 用户场景：**issue 很多，一个 agent 管一个 issue**（先不考虑依赖）；但 agent **很可能忘记建 workspace**，需要 kiln 兜住；且判定"绑定文件"不靠谱——两个 issue 碰**同一个文件**时，归属无法检测、声明也麻烦。

### 0. 接受判断：**不要检测，要隔离**

- 两个 issue 编辑同一文件时，"这份改动属于谁"**从文件本身无法判断**（内容里没有归属信息），要求用户声明又太重 → **放弃上一轮的 C（绑定文件 + 归属检测）**。
- 换思路：**隔离在「工作区」层做**，问题自动消失——两个 issue **各编辑自己的副本**，永远不会落在同一个工作区里混；真正合并时交给 **VCS 的正常冲突机制**（jj 已验证处理得体）。
- 一句话：**文件"归属"不用判，因为根本不需要它们共处一室。**

### 1. 隔离的单位 = **issue**（不是 agent）

- 一个 issue 一个工作区。**名字 / 路径 / 分支由 issue 派生** → **命名即绑定**，**不需要额外的绑定文件**。
- 于是 `kiln workspace list` 能列出全部隔离区，而**无需任何声明、也无需任何检测**。

### 2. "agent 会忘"怎么办——三层手段

| 手段 | 形态 | 静默程度 | 代价 / 风险 |
| --- | --- | --- | --- |
| **E1** 显式原语 | `kiln workspace new <issue>`（后端中立，敲一条） | 中（还得记得敲） | 小 |
| **E2** 读命令隐式建 | `kiln path <issue>` **首次触达即建**并提示 `cd` | **高**（不必记新的动词） | 违反"读命令无副作用"的取向 |
| **E3** 编排层建 | 起 agent 时就建好、并把它置于其中 | **最高**（agent 无感） | 要 kiln 参与编排 / 依赖 harness 配合 |
| **G** 结构性护栏 | `kiln commit` **在非专用工作区**拒绝 / 告警 | 兜底 | 会误伤"单人正常在主干工作" |
| **UL** 只是提示 | 读命令打一行"并行请先建区" | 低 | 有人不看提示 |

**关键**：G 只判"**我是不是在专用工作区**"（**结构性**），**不判"这些文件属于谁"**（归属性）——正好绕开你指出的难点。

### 3. 倾向

- **E1 + G** 起步：一条命令建区；护栏把上一轮实测的"**提交吞并别人的改动**"挡在**提交之前**（即便文件已被改，至少**动作不会串档**）。
- **E2 最"静默"**（不用记新动词），但要权衡"**读命令有副作用**"是否可接受。
- **E3 才是真·静默的答案**，但它属于**编排**——取决于"谁来起 agent"。若将来有 `kiln run <issue> …` 之类（kiln 参与拉起 agent），顺手把区建好即可。

### 4. 关于 G 的可行性（诚实标注）

- **不能**无差别拒绝主干工作区：**单人 agent 在主干干活是完全正常的**，拒了会砸掉正常流程。
- 所以 G 需要知道"**当前是否处于并行模式**"。两条路：
  - **配置开关**（如 `.kiln.json` 的 `parallel: true`）→ 只有并行模式才护栏；
  - 或**自动**：**已存在多个工作区**即视为并行，此时主干提交才告警。
- 倾向：**默认只 warn**，把"拒绝"留给显式配置。

### 5. 为什么这样更简洁

- **少三样东西**：没有绑定文件、没有归属检测、没有专门的 `check` 规则。
- 两个 issue 碰同一文件**不再是问题**（各改各的副本，合并交给 VCS）。
- 与取向一致：kiln 只提供**原语 + 结构性护栏**，**编排仍在外**。

### 6. 待定 / 非目标

- **命名 / 路径约定**：`../<repo>-<issue>`？分支 `kiln/<issue>`？
- **G 的触发**：配置开关 vs "存在多个工作区即视为并行"。
- **非目标**：本轮不改代码；**不重开 author-attribution**。

## hypothesize（再续）：agent 互不可见——信号必须取自「共享仓库」，不许 agent 自省

> 用户指出硬伤：**不同 agent 之间互不知道；就算问"是否并行"，agent 也没有信息可判断。** → G 不能建立在"agent 判断"上。

### 0. 但仓库是唯一的公共观测点

agent 彼此**看不见**，它们**共享一个仓库**——能"看见彼此"的**不是 agent，而是仓库状态**。两端强度还不同：

- **jj**：workspace **共享同一 store** → `jj log -r 'all()'` 里**能看见别的 workspace 的 `@`（在飞的工作）**（实测：在 w1 里看到了 w2 的 `@` `tsmxrpmu`）；`jj workspace list` 看区。
- **git**：worktree 各自独立 → 别的 worktree 的**未提交改动不可见**；只能靠 `git worktree list` 看到"区存在"。

→ **jj 能"看见别人在干活"，git 只能"看见有人存在"。** 且两者都**看不到**同一工作区里另一个 writer 的未提交改动——所以任何"动态检测并发"都不成立。

### 1. 三类来源（按可靠度）

| 来源 | 形态 | 可靠度 | 备注 |
| --- | --- | --- | --- |
| **D 声明**（推荐） | `.kiln.json` 里的**并行 / 布局标志** | **高** | **所有 agent 读同一份** → 天然解决"互相不知道"；由"谁起 agent"的人声明一次 |
| **O 观测** | `workspace list` > 1；jj 的 `all()` 里有别的 issue 的**在飞动作** | 中 | 启发式；**t=0（还没别人）时会漏**；git 端只能看到"区存在" |
| **U 统一布局**（零判断） | 约定"**动作只在专用工作区提交，主干只读 / 只合并**" | **高（不依赖任何信息）** | G 退化成"在主干提交 → warn"，**根本不用判断是否并行** |

### 2. 结论

- **别让 agent 判断**——它做不到。
- 二选一：**要么把"是否并行"变成一份共享声明**（人能声明，agent 只能读）；**要么干脆不判断**——让"在专用工作区提交"成为**统一约定**，G 只看"我在不在主干"。
- 后者更合"不越界、不猜"：**零检测、零声明，只查结构**。

### 3. 倾向

- **D 为主**：它**直接对症**你指出的问题——并行这件事**由"起 agent 的人"声明进共享配置**，**每个 agent 读同一份**，不需要私有知识。G 的判据 = **结构（我在不在主干）+ 声明（这个仓是不是并行布局）**。
- **U 作为零配置替代**：若不想加配置，就把"动作只在专用区提交"定为**统一约定**，单人用户**同样**走一次 `workspace new`（一条命令），换来**规则统一、无需任何并发判断**。
- **O 只作增强**：`workspace list` 多值 / jj 看到别的 issue 在飞，可用于**额外提示**，但**不能**当唯一依据（会漏）。

### 4. 待定 / 非目标

- 声明键名与取值（`parallel` / `layout`？），以及"谁写"（人 / harness）。
- 是否把 O 接进 `kiln workspace list` 作为"当前疑似并发"展示。
- **非目标**：本轮不改代码；不让 agent 做并发判断。

## hypothesize：U 方案设计——「主干只读，动作只在专用工作区提交」

### 0. 一句话定义

**主干（main checkout）只读 / 只合并；每条动作都在专用工作区提交。**
→ G 退化成 **"我是不是在专用区"**，于是 **零并发判断、零配置、零文件归属**。

### 1. 结构判据：什么算"专用区"

- **git**：**链接工作树**（`.git` 是**文件**，或 `rev-parse --git-dir` ≠ `--git-common-dir`）＝专用；仓库根（`.git` 是**目录**）＝主干。
- **jj**：**非 `default` workspace**＝专用；`default`＝主干。
→ **纯结构**，两端都判得出，**不需要任何 agent 知识**。

### 2. 原语 E1（后端中立）

- **`kiln workspace new <issue>`**：建（或复用）专用区、打印路径 + "`cd` 过去再动手"。
  - git → `git worktree add <path> -b kiln/<issue>`；jj → `jj workspace add <path>`。
  - **命名即绑定**：路径 / 分支由 issue 派生，**无绑定文件**。
  - **区放在仓库之外**（兄弟目录 `<repo>--<issue>`，或统一根 `…/ws/<repo>/<issue>`），避免污染仓库。
  - **幂等**：已存在 → 复用。
- **`kiln workspace list`**：列出全部专用区（+ 由名字推出的 issue）。

### 3. 护栏 G（纯结构）

- **`kiln commit`**：
  - 在**专用区** → 正常；
  - 在**主干** → **拒绝**（"动作只在专用工作区提交；先 `kiln workspace new <issue>`"），`--force` 可显式豁免。
- **读命令**（`path` / `show` / `status` / `check` / `graph` / `list`）**不受限**：主干照样读，且看到的是**全局**（Log 是全仓的）。
- 得到一条干净的不变式：**读是全局的，写是按区分地的。**

### 4. U 真正买到什么

- **静默污染在结构上不可能**：主干的动作提交**根本不存在** → A 无法在共享区吞掉 B 的改动。
- **零判断 / 零配置 / 零归属**：没有 `.kiln.json` 开关、没有文件检测、没有 `check` 新规则。
- **单人 / 并行同一套规则**：无需模式切换。

### 5. U 的代价（诚实标注）

- **单人用户也要走一次"建区 + cd"**——唯一的摩擦。
- **"已在主干改了文件才想提交"怎么办**：护栏在**提交时**触发，而改动**早已落在主干** → 需要**迁移路径**：
  - 最小：报错时**提示**改动仍在主干、需迁移；
  - 更好：**`kiln workspace new <issue> --adopt`** 把主干**当前未提交改动**带进新专用区（git：stash → 在 worktree 里 apply；jj：把主干的 `@` squash / rebase 过去）。
- **拦不住编辑**：编辑不是 kiln 命令 → U 只能**拦住提交**，拦不住"已经在主干编辑"。

### 6. 与 D 对比

| | **U（统一布局）** | **D（声明开关）** |
| --- | --- | --- |
| 配置 | 无 | 需在 `.kiln.json` 声明 |
| 单人用户 | **也要建区**（摩擦） | 不变 |
| 并行 | 结构护栏 | 结构 + 声明 |
| 判断 | **零** | 零（读共享声明） |

### 7. 待定 / 非目标

- 区的位置约定；`--adopt` 的实现；`--force` 是否保留。
- 主干是否也承担"合并"职责（如把 `kiln/<issue>` 分支合回主干）。
- **非目标**：本轮不改代码。

## test：落地 U 方案——workspace 原语 + 主干结构性护栏

> 承「hypothesize：U 方案设计」：把「主干只读 / 只合并，动作只在专用工作区提交」**落地成代码并实测**。范围按早先约定取**核心 U**（`new` / `list` + 护栏 + `--force`），`--adopt` 不做。

### 落地内容

- **`vcs.Workspace`**（新接口，**与 `vcs.Repo` 分开**）：`IsDedicated()` / `NewWorkspace(issue)` / `ListWorkspaces()`；配套 `WorkspaceInfo{Path, Issue, Main}`、`SiblingArea(root, issue)`（兄弟目录 `<repo>--<issue>`）、`SamePath`。分开是为了区分"**后端能读写**"与"**后端能隔离**"。
- **`vcs/git`**：`IsDedicated` = `.git` 是**文件**（链接工作树）还是**目录**（主干）；`NewWorkspace` → `git worktree add <path> -b kiln/<issue>`；`ListWorkspaces` 解析 `git worktree list --porcelain`（首条即主干，分支前缀 `kiln/` 即 issue）。
- **`vcs/jj`**：`IsDedicated` 以 `jj workspace root` 对照 `jj workspace list -T 'name ++ "\x1f" ++ root ++ "\x1e"'`，判"当前区是否 `default`"；`NewWorkspace` → `jj workspace add --name <issue> <path>`；`ListWorkspaces` 解析同一模板。
- **`cli`**：新命令 `kiln workspace new <issue>` / `kiln workspace list`；`kiln commit` 增护栏 G——**纯结构判据：在主干即拒绝**（exit 1），`--force` 显式豁免；**读命令一律不受限**；后端不实现 `vcs.Workspace` 时跳过护栏（不假装有隔离）。`kiln -h` 同步更新。
- **命名即绑定**：区路径 / 分支 / workspace 名全部由 issue 派生，**无绑定文件**——承「按 issue 建隔离区（静默化）」的选择。

### 验证

- `gofmt -l` 干净；`go vet ./...` 通过；`go test ./...` **全绿**。
- 新增单测 14 例：`vcs` 2（`SiblingArea` / `SamePath`）、`vcs/git` 2（worktree porcelain 解析）、`vcs/jj` 2（workspace 列表解析）、`cli` 8（主干拒绝 / `--force` / 专用区放行 / `do` 不受影响 / `list` 标注 / `new` 打印路径 / 非法 issue 拒绝 / 未知子命令）。

### 真机实验（临时仓）

**git 后端**（`gmain`：主干 + 一个专用区）

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | 主干有 `a.md`/`b.md` 未提交改动 → `kiln commit` | **拒绝**，exit 1；提交数不变、两文件仍游离 → **动作根本没落盘** |
| 2 | `kiln workspace new u-test` ×2 | 建 `…/gmain--u-test`（分支 `kiln/u-test`）；**第二次原样复用**（幂等，exit 0） |
| 3 | `kiln workspace list` | 主干行 + `专用 u-test`，按路径宽度对齐 |
| 4 | 结构判据 | 主干：`.git` 是目录、`gitdir == commondir`；区里：`.git` 是**文件**、`gitdir = …/.git/worktrees/gmain--u-test` ≠ `commondir` |
| 5 | 区内 `git add c.md` + `kiln commit` | **成功**（exit 0）；区里 `kiln path` 立即显示该动作 |
| 6 | **主干读区内的动作** | **读不到**（`未找到问题: u-test`，exit 1）→ **git 的分支隔离** |
| 7 | 主干 `--force`（且 `git add -A`） | 成功，但 `git show --stat` = **a.md + b.md 两文件** → **复现"吞并别人的改动"**；对照实验 1，护栏挡的正是这件事 |
| 8 | 主干 `kiln status` | 正常（exit 0）——**读不受限** |
| 9 | 主干 `git merge kiln/u-test` 后再 `kiln path` | **读到 2 步**（区内动作 + 主干强制动作）→ **合并即全局** |

**jj 后端**（`jmain`：colocated，`default` + 一个专用 workspace）

| # | 实验 | 结果 |
| --- | --- | --- |
| 10 | `default` workspace → `kiln commit` | **拒绝**，exit 1 |
| 11 | `kiln workspace new u-test` ×2 + `list` | 建 `…/jmain--u-test`（workspace 名 = issue）；复用幂等；list 显示主干 + 专用 |
| 12 | 区内 `kiln commit`（c.md **未暂存**） | **成功**——jj 终结整个工作区变更，**免暂存** |
| 13 | **`default` 读区内的动作** | **读到**（exit 0）——`all()` 覆盖共享 store → **jj 的读是全局的** |
| 14 | `default` 里 `b.md`/`e.md` 待提交 → `kiln commit` | 拒绝；`jj status` 显示两文件**仍在工作副本**（未提交）→ **吞并没发生** |
| 15 | `default` `--force` | 成功（豁免） |

### 发现（对假设的修正）

1. **"读是全局的"只对 jj 成立，对 git 不成立**。hypothesize（U）§3 写"主干照样读，且看到的是**全局**（Log 是全仓的）"——**实验 6 证伪**：git 的动作提交在 `kiln/<issue>` 分支上，**主干在 merge 前读不到**（§3 的"全仓"实际只是"全分支"）。后果：并行期间**主干的 `status` / `check` 会少报在飞动作**；jj 因 workspace **共享同一 store**（`all()` 覆盖全部）没有这个缺口。→ 应写进方案：**git 布局下"全局视图"要靠周期合并兑现，jj 布局下天然兑现。**
2. **护栏确实挡在"吞并"之前**（实验 1/7、14）：主干**根本没有动作提交** → A 无法把 B 的未提交改动带进自己的动作。"**结构性、零判断**"的承诺成立。
3. **护栏是 fail-closed 的**：判不出"我在哪"（后端不支持 / 命令失败）时报错停手，**不静默放行**——避免"以为有隔离"。
4. **后端中立性成立**：`workspace new` / `list` 两端**同一命令、同一输出形态**；差异（分支名 vs workspace 名、暂存 vs 免暂存）全被适配器吞在内部——**这正是 `vcs` 抽象该吞的东西**。
5. **自举摩擦是真的**（承 §5）：本仓第一次建区**只能用原生 `git worktree add`**（旧二进制没有 `workspace` 命令）——"单人也要走一次建区"在落地时表现为"**新命令得先有二进制**"。→ **首次建区天然需要一条不依赖 kiln 的引导路径**（原生 VCS 命令即可，不必为此设计特殊命令）。
6. **环境坑（记录备查）**：本机 `core.autocrlf=true`，专用区 checkout 出 **CRLF**，使 `gofmt -l` 把**整棵树**报成未格式化（主干是 LF 所以干净）；归一成 LF 后干净，且 `git diff --cached` 证实**未产生伪改动**。**不是 kiln 的问题，但会在"每个新建的工作区"上反复出现**。

### 未做 / 待定

- **`--adopt` 未实现**（§7 待定项）："已在主干改了文件才想提交"目前只有护栏报错 + 提示，没有把改动迁进专用区的命令。
- **残留分支的复用边界**：区被删而 `kiln/<issue>` 分支尚存时，`workspace new` 会因 `-b` 冲突失败（未做二次尝试兜底）。
- **主干承担合并职责**：实验 9 用原生 `git merge` 走通；kiln 不提供合并命令——与"编排在外"一致，维持现状。

## test：区的位置改到仓库内 `.kiln/ws/`

> 承 U 方案 §7 的待定项「区的位置约定」。原倾向是"仓库之外的兄弟目录 `<repo>--<issue>`"；本轮按用户要求改为**仓库内的状态根**，并先把 git / jj 的惯例查清再动。

### 调研：git 与 jj 各自怎么放

| | **git** | **jj** |
| --- | --- | --- |
| 工作树放哪 | **无约定**，路径由调用方给（官方只定义 `<path>`） | **无约定**，同理 |
| 自己的元数据 | 集中在 `$GIT_DIR/worktrees/<id>/`，**扁平、每工作树一目录**：`HEAD`/`index`/`ORIG_HEAD`/`commondir`/`gitdir`（回指文件）/`logs/`/`refs/` | 共享 store 在主 `.jj/repo/`（`store`/`op_store`/`op_heads`/`index`/`workspace_store`/`submodule_store`）；次 workspace 的 `.jj/repo` 是**指向共享 store 的文件**，另有自己的 `.jj/working_copy/{checkout,tree_state,type}` |
| 状态目录的自我忽略 | 无 | **有**：`.jj/.gitignore`，内容 `/*` |
| 建中间目录 | `worktree add` **会建** | **不建**（实测父目录不存在 → `Cannot access … 系统找不到指定的路径`） |
| 区落在仓库内的后果 | 主干 `git status` 冒 `?? .kiln/` → **必须忽略** | 默认**看不见**（`jj status` 无变化、`jj diff -r @` 0 文件）——不走进嵌套 workspace |
| 清理原语 | `worktree remove` | `workspace forget` / `remove`（`remove` 连**被忽略的文件**一起删；主 workspace 不可删） |

社区（面向 agent 的 worktree 工具）惯例同样是「**仓库内目录 + 必须 gitignore**」（`.worktrees/`、`claude/worktrees/`、`.workspaces/<slug>/`），并有工具用 `.worktreeinclude` 把**被忽略的本地文件**复制进新工作树。

### 选定与落地

```
<repo>/                      ← 主干（main checkout，只读/只合并）
  .kiln/
    .gitignore               ← 内容 /*：状态根自我忽略（照 jj 的 .jj/.gitignore）
    ws/
      <issue>/               ← 一个 issue 的专用区 = 工作树根
                                （git 分支 kiln/<issue>；jj workspace 名 = issue）
```

- **两级，且 `ws` 刻意短**：多一层需要一个**稳定命名键**——domain 会变、agent 身份已判"不做"、时间戳是噪声，都不成立；而 git 自己**扁平**（`.git/worktrees/<name>`）、jj **固定两层**（`.jj/repo` 共享 + `.jj/working_copy` 每区），都不按内容再分层。短名是因为**区里每条路径都要摊这个字符数**（Windows 260 预算）。
- **锚点是主干根**（`worktree list` 首条 / `workspace list` 的 `default`），**不是进程 cwd、也不是当前所在区**——避免"区中区"。
- **忽略用自我忽略**（`.kiln/.gitignore` = `/*`），**不动仓库受控文件**——在"主干不能提交"的 U 布局下尤其要紧：kiln 没法替你落一个根 `.gitignore` 的改动。
- **登记仍由 VCS 提供**：git `worktree list` / jj `workspace list` 已是登记处 → 状态根下**不存 kiln 自己的绑定文件或索引**（守住"命名即绑定"）。
- 代码：`SiblingArea` → `AreaPath(mainRoot, issue)` = `<mainRoot>/.kiln/ws/<issue>`，新增 `PrepareAreas(mainRoot)`。CLI 与**识别逻辑未改**（git 仍从分支 `kiln/<issue>` 推 issue，jj 仍用 workspace 名）。

> **修订 → 再修订（config-home）**：本节"忽略用自我忽略（`.kiln/.gitignore` = `/*`）"**一度**被推翻——config-home 先把配置收进 `.kiln/`、忽略规则因此改成 `/ws`；随后该 issue **复议**又把配置挪到**仓库根**（`.kilnconfig.jsonc`），于是忽略规则**回到本节的原决策 `/*`**，只是配置不再住在状态根里。见 `docs/config-home.md`。

### 验证（临时仓，新二进制）

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | git：`kiln workspace new u-test` | 区落在 **`<repo>/.kiln/ws/u-test`**；同时写入 `.kiln/.gitignore` = `/*`；主干 `git status` **空** |
| 2 | git：连跑两次 | **幂等复用**（同路径，exit 0） |
| 3 | git：`workspace list` | 主干行 + 嵌套路径的 `专用 u-test` |
| 4 | git：区内 `git add` + `kiln commit` | **成功**（exit 0） |
| 5 | jj：`kiln workspace new u-test` | **成功**（此前因父目录不存在直接失败）——`PrepareAreas` 先建 `<repo>/.kiln/ws` |
| 6 | jj：主干 `git status` / `jj status` | **都空**（colocated 下 git 也安静） |
| 7 | jj：区内 `kiln commit`（未暂存） | **成功**（exit 0） |
| 8 | 单测 | `gofmt -l` 干净、`go vet` / `go test ./...` 全绿（`vcs` 新增 `AreaPath` + `PrepareAreas` 共 3 例） |

### 两条实现约束（都是实测逼出来的）

1. **必须先建中间目录**：jj 不建（git 建）→ `PrepareAreas` 统一 `MkdirAll(<repo>/.kiln/ws)`。
2. **忽略条目必须先于第一个区存在**，否则 git 立刻把区报成未跟踪内容；`PrepareAreas` 写 `.gitignore`，且**已存在则不动**（不覆盖用户改过的规则）。

### 风险 / 与 U 原决策的取舍

1. **`git clean -xdf` 在主干执行会删掉整个 `.kiln/`（所有区一起没）**——这是"区在仓库内"的真实代价；U §2 原本倾向仓库外兄弟目录，正是为了躲这个。**这是取向交换**，换来"工作区集中在仓库内一处、`ls .kiln/ws` 即见"。
2. **Windows 长路径**：`<repo>/.kiln/ws/<issue>` 再叠构建产物可能逼近 260 上限——故 `ws` 取短名（本机已启用长路径则无碍）。
3. **jj"看不见嵌套 workspace"是 0.45 的隐式行为**，不能当契约——靠显式 `.gitignore` 兜住。
4. **缺一个原语**：jj 有 `workspace remove` / `forget`，kiln 只有 `new` / `list`；区进了仓库内后清理需求更明显，**留作下一轮候选**。
5. 一处不协调：配置是根下的 `.kiln.json`（**文件**），状态根是 `.kiln/`（**目录**）——同前缀不同物；将来可收成 `.kiln/config.json`，本轮不动。

## 非目标

- 不评估 MCP 的实现细节。
- 不实现 `--adopt`；不做"绑定文件 / 文件归属检测"；不让 kiln 编排 agent。

## 备注

由用户在会话中直接提出：看到 agent 用 `git log --format=%B` 而非 `kiln path`，追问"基于什么考虑；如果 agent 都在用 git，kiln 的 CLI 有多少必要性"。这是迄今最**根本**的一个问题——它反过来质疑 kiln 这个项目的立足点。
