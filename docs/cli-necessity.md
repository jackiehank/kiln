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

## 非目标

- 本轮不改代码、不新增 / 删除命令。
- 不评估 MCP 的实现细节。

## 备注

由用户在会话中直接提出：看到 agent 用 `git log --format=%B` 而非 `kiln path`，追问"基于什么考虑；如果 agent 都在用 git，kiln 的 CLI 有多少必要性"。这是迄今最**根本**的一个问题——它反过来质疑 kiln 这个项目的立足点。
