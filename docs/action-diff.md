# issue: action-diff — 如何追踪一个 issue 的全过程，并查看每个 action 的变更

- **状态**：ask（进行中）
- **domain**：cli
- **关联**：relates-to `status-by-domain`

## 背景

`kiln path` 现在只列 **kind / hash / subject** 三列：

```
relation-vocabulary  已完成（当前 conclude）
  1. ask          fc5b428  9 个关系会不会太多、语义清晰吗、Agent 好不好读
  2. hypothesize  4910435  决策树草案 + …
  …
```

也就是说：**能看到"做了哪些动作"，但看不到每个动作到底改了什么。** 要知道变更，得手动 `git show <hash>` 一条条翻——而这正是"追踪一个 issue 的全过程"最想看的部分。

## 要回答的问题

1. **变更怎么带进 `path`？**
   - 全量 diff 会把整个问题淹掉（一个 issue 8 个动作、每个都几百行）；
   - 只给 `--stat`（文件 + 增删行数）可能刚好够用；
   - 还是干脆另开一条命令？
2. **要不要 `kiln show <issue> <n>`？** 只展开第 n 个动作的完整变更——比在 `path` 里塞 diff 干净，但多一个子命令、还要给动作编号（现在只有 hash，hash 会随历史重写而变）。
3. **和 `graph` / `status` 的分工**：`status` 是 domain 视图、`graph` 是关系视图、`path` 是**过程视图**——变更展示该挂在 `path` 上，还是另有其位？

## 更根本的一层：用 diff 反查元数据的诚实性

现有 `target:` trailer 表示"这个动作涉及的文件"。**它完全可以和真实 diff 对照**：

- 声明了 `target: internal/cli/cli.go`，实际只改了 `docs/` → **声明不实**；
- 改了 5 个文件一个字没提 → 至少值得一个提示。

也就是说，`diff` 不只是"好看"，它能让 kiln 多一条**可校验**的规则（现在 `check` 只校验关系完整性，从不碰"声明 vs 现实"）。这让 `check` 第一次有了"对着真实代码校验"的能力。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** | `kiln path --stat`：每个动作附一行文件统计 | 轻、够用 |
| **B** | `kiln path --diff`：附完整 diff | 重，终端会被淹 |
| **C** | `kiln show <issue> <n>`：单独展开某个动作的完整变更 | 多一个子命令 + 动作编号问题 |
| **D** | `check` 新增规则：`target:` 声明 vs 实际 diff 一致性 | 让 check 首次"对着真实代码"校验 |

A + C 可以并存（概览用 A、深挖用 C）；D 是独立价值，值得单独评估。

## 业内实践调研（collect）

> 一次联网调研的整理，来源 URL 见本节末尾。目的：看成熟系统怎么把「变更证据」挂到 issue 的生命周期上——即 `path` 该不该/该怎么带 diff。

### 1. 两条主线：外挂式关联 vs 内嵌式存储

- **外挂（主流）**：GitHub / Jira / Linear——在 commit message 里写关键词（`Closes #10`、Jira 的 `PROJ-123`），工具在**外侧**建立 commit↔issue 关联；issue 页上因此出现「development panel」（分支 / 提交 / PR / 构建），并可下钻看 **files changed** 与 **code diff**。
- **内嵌（小众）**：**git-bug** 把 issue 存成本仓库 `refs/bugs/<id>` 下的**操作日志**，与代码历史**完全分离**（`git log` 主分支看不到任何 bug commit），因而可离线、可分布式、可像代码一样 push/fetch。

> 这个对照对 kiln 很关键：git-bug 把 issue 历史**独立**存；kiln 反其道，**直接拿代码提交当动作历史**。所以对 kiln 而言"diff 就在这些提交里"——`path` 带变更几乎是**顺手**的事，这是 kiln 相对 git-bug 的**结构性优势**。

### 2. 变更展示：默认「摘要」，下钻才「全文」

- **git 原生就是两档**：`git log --stat / --shortstat / --name-status` 是摘要；`--patch / -p` 才是全文。这正是"概览 vs 深挖"的原始范式 → 直接对应候选 **A / C**。
- **GitHub PR "Files changed"**：先给一张 diffstat 汇总，再逐文件展开补丁。
- **Jira dev panel（GitKraken 等插件）**：issue 上先给**活动 rollup**（提交列表），再给 **files changed + code diffs**。

> 结论：**摘要与全文分两层**是行规，没人把整段 diff 塞进列表视图。→ 支持 **A 概览 + C 深挖并存**。

### 3. 动作的稳定寻址：hash 不稳定是通病，主流靠「提交内容里的稳定 ID」

- **Gerrit `Change-Id`**：在 commit footer 写一个 `Change-Id:` trailer，给一个 change **稳定身份**；即使经过 rebase / cherry-pick，新版本仍关联回同一次评审，一串修订叫 **patch sets**。
- **`git range-diff`**：比较**同一补丁序列的两个版本**（v1 vs v2），能自动找出「对应提交对」——即"历史重写后仍可对照每个动作"。

> kiln 现在给 action 的唯一标识就是 **commit hash**，而 hash 会随历史重写漂移（本仓库确实重写过历史）。→ `show <issue> <n>` 的 `n` 应当是**"该 issue 内第 n 个动作"的相对序号**（相对 `path` 的顺序、不随 hash 变），而不是把 hash 当稳定主键。

### 4. 提交消息 = 机器可读元数据：trailer + 校验器

- **`git interpret-trailers`**：commit message 末尾的结构化 `key: value` 块（trailer block 必须**前置空行**）。kiln 的 `issue:` / `action:` 就是这一格式——本仓库刚修过的"标题与 trailer 间缺空行"问题，正是该规范的一条硬要求。
- **`git notes`**：**不改 commit**、在对象上外挂注释，`git log` 可一并显示。→ 另一种"给已有提交补元数据"的机制（对"追加证据而不改写历史"有价值）。
- **commitlint**：内置 `trailer-exists` 规则、`issuePrefixes`、`noteKeywords`，在 **commit-msg hook / CI** 校验结构。→ 对应 kiln `check` 从"关系完整性"扩展到"**结构化与声明**校验"。

### 5. 「声明 vs 现实」的校验：可追溯性（traceability）有成熟传统

- **需求可追溯性 / 双向追溯**（NASA SWE-052 *Bidirectional Traceability*）：要求需求→设计→代码→测试**双向链接**，变更时做 **impact analysis**，并定期做 **traceability audit**。
- **ReqToCode**（arXiv 2603.13999）：把 traceable 做成代码里的**强引用**，由构建工具**自动校验**（引用的 traceable 被删，则依赖代码**编译失败**）。

> "让声明可被真实产物校验"是有几十年传统的（可追溯 / 审计）。但"**用某条提交的 diff 反查该提交 trailer 的自述**"没有现成标准做法——若做候选 **D**，等于把 traceability 的精神搬进 **trailer 层**，属于**新意**，不是抄现成方案。

### 6. 对本设计最相关的 6 条

1. **变更展示走两层**：`path` 默认给 stat（A），深挖用单独命令给全 diff（C）——git 自身、GitHub PR、Jira panel 都是这么分档的。**A + C 并存有充分依据**。
2. **动作需要稳定寻址，别只用 hash**：Gerrit 用提交内 `Change-Id` 提供跨改写稳定 ID；`range-diff` 支持版本间对照。→ `show` 的编号应是**issue 内相对序号**。
3. **trailer 是主流且必须有校验器**：`interpret-trailers` 定格式、commitlint 的 `trailer-exists` + CI 做校验。→ **D 的"校验"部分与行规同源**，只是校验对象从"格式/必填"升级到"对真实 diff"。
4. **声明可校验 = 有传统的可追溯性**；但"trailer 声明 vs 真实 diff"**没有现成实现**，是 kiln 可发力的**空白区**。
5. **git-bug 是反向参照**：它把 issue 历史与代码历史**分离**；kiln 用代码提交**本身**当动作史，所以 diff 天然在手。→ 这是 kiln 做"**证据内建**"的先天优势，不用另建存储。
6. **不改 graph / status 的判断得到支持**：所有成熟系统的"变更证据"都挂在**单个工作项的视图**上（issue 页 / change 页），不挂全局图。→ 变更展示**归 `path`（过程视图）**，不越界。

> 来源：[GitHub：Closing issues using keywords](https://docs.github.com/en/enterprise-server@3.13/get-started/writing-on-github/working-with-advanced-formatting/using-keywords-in-issues-and-pull-requests) ｜ [Jira：View development information](https://support.atlassian.com/jira-software-cloud/docs/view-development-information-for-an-issue/) ｜ [Jira × GitHub 集成（显示 files changed / diffs）](https://www.gitkraken.com/git-integration-for-jira) ｜ [git-bug：Git as a Database](https://dev.to/eme_gug_0821b41b948be6516/git-as-a-database-how-git-bug-stores-issues-inside-your-repo-and-how-to-build-your-own-39mi) ｜ [git-bug 简介（Git Rev News 43）](https://git.github.io/rev_news/2018/09/19/edition-43/) ｜ [git-notes](https://git-scm.com/docs/git-notes) ｜ [git-interpret-trailers](https://git-scm.com/docs/git-interpret-trailers) ｜ [git-log（--stat / --patch）](https://git-scm.com/docs/git-log/zh_HANS-CN.html) ｜ [git-range-diff](https://git-scm.com/docs/git-range-diff) ｜ [Gerrit Change-Ids](https://review.opendev.org/Documentation/user-changeid.html) ｜ [Gerrit Changes / patch sets](https://review.opendev.org/Documentation/concept-changes.html) ｜ [commitlint Rules（trailer-exists）](https://commitlint.js.org/reference/rules) ｜ [commitlint CI Setup](https://commitlint.js.org/guides/ci-setup) ｜ [NASA SWE-052 Bidirectional Traceability](https://swehb.nasa.gov/display/SWEHBVD/SWE-052+-+Bidirectional+Traceability) ｜ [ReqToCode（arXiv 2603.13999）](https://arxiv.org/pdf/2603.13999v1) ｜ [Visure：Requirements Traceability Links](https://visuresolutions.com/alm-guide/requirements-traceability-links)

## 非目标

- 不改 `graph` / `status`。
- 不新增存储——diff 全部来自 git 本身，kiln 仍只是"读 + 呈现"。

## 备注

这是本次长会话里**用 `kiln path` 复盘自己工作时**顺手记下的：过程看得到，但证据看不到。
