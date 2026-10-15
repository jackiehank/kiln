# issue: qualified-issue-id — 命令输入要不要接受 domain/id 限定名（变相引入命名空间，可减少重名）

- **domain**：cli
- **关联**：relates-to `issue-uniqueness` / `status-by-domain` / `relation-rendering`

## 背景

`kiln status` 的文本视图打印 `<domain>/<id>` 形态的名字：

```
cli/action-diff                 ✓ conclude
└── design/target-honesty         ask
```

而命令输入只吃裸 `id`——`kiln path cli/action-diff` 得到的是「未找到问题: cli/action-diff」。

- 造出这个 label 的是 `issueLabel`（[internal/cli/cli.go](../internal/cli/cli.go)）：`p.Domain + "/" + p.ID`；它在全仓**只有一个调用点**（`statusLabel`）。同一份数据的其它出口都给裸 `id`——`status --json` 是 `"id"` + 独立的 `"domain"` 字段，`list` / `check` / `graph` 也是裸 `id`。
- 输入侧是 map 直查（`repo.Issues[pos[0]]`）：查不到就报「未找到问题」，**没有回退、没有提示**。
- 规范本身没写错：[README](../README.md)「提交格式」一节写的是「`issue:` — issue ID（**单段 kebab-case slug**）」；[mvp-plan](mvp-plan.md) §2.4 更明确「小写；只允许字母、数字、`-`；长度 ≤ 50；**不允许 `/`**」，并留了沿革：早年用过 `<namespace>/<slug>` 来降低重名概率，后因"单仓库、单人使用时 namespace 恒等于项目名、零信息量"而**去掉**。

所以这不是"文档没写"，而是**工具自己的输出与输入不自洽**：视图给出一个带命名空间的路径，输入不认它。

## 要回答的问题

1. **接受还是拒绝** `<domain>/<id>`：接受 = 输入侧把早年删掉的 namespace 请回来；拒绝 = 保持单段 slug 不变量，只补提示。
2. **若接受，落在哪一层**：只做输入宽容（显示 `cli/x`、落库 `x`），还是 `issue:` trailer 的值也变成两段（显示名 = 落库名）？
3. **既有数据**：仓库里已有的裸 slug 怎么办——迁移、并存，还是"两种写法都收"？
4. **唯一性边界**：加 domain 是"**减少**重名"还是"**让重名无害**"？`kiln check` 的查重范围随之怎么变？
5. **视图与输入要不要强制一致**：`status` 打出来的名字必须能粘回命令吗？
6. **与 `issue-uniqueness` 的分工**：那条问"唯一性怎么保证"，本条问"输入侧要不要用限定名"，互为前提。

## hypothesize：这是 `status` 的**显示**问题——domain 由分组头显示一次，行内只留裸 id

> 方向定稿：**不引入命名空间**（`issue:` trailer 不动、输入解析不动），只改**显示**。理由与代价见下；三件必须一起定的事见 §3。

### 1. 方向

`status` 从「每行自带 `domain/id` 前缀」改成「**块头写一次 domain，行内只写裸 id**」：

```
cli
  action-diff                        ✓ conclude
  └── target-honesty  (design)         ask
  qualified-issue-id                   ask

design
  action-config                      ✓ apply
```

（缩进与树形符号的具体画法留给 `test`；本轮的取舍只有一条：**domain 一块出现一次**。）

### 2. 为什么这个方向赢

1. **根治原始误解**：显示出来的名字**就是**能粘回命令的名字（裸 id）——`path` / `show` / `worktree new` 直接吃。不再有「看得见、粘不回」的名字，这正是本条 issue 的开题动因。
2. **不引入命名空间**：`issue:` trailer 仍是单段 slug，`engine` 的键空间、`check` 的唯一性检测、`--domain` 筛选全都不动。`issue-uniqueness` 的结论不再被本条绑架——两者的关系从「互为前提」降级为**背景相关**（trailer 保持 `relates-to`，但不再是前置依赖）。
3. **顺手消掉两个隐性陷阱**：① domain 是「最后一个非空 `domain:` 胜出」的**可选** trailer，把它当前缀等于让**地址随动作变化**，而没声明 domain 的 issue **没有前缀**（同一视图两套写法）；② 跨域嵌套行会让**同一个块里出现两个不同的 domain 前缀**（本仓：`cli/action-diff` 下挂 `design/target-honesty`），读者得额外学一条「前缀是行自己的、块是别人的」的规则。

### 3. 必须一起定的三件事（本轮的增量）

#### 3.1 跨域嵌套行——现设计的**理由句**被抽掉了

[mvp-plan](mvp-plan.md) §11 现在写的是：

> 每行写作 `<domain>/<issue-id>`；**不设目录标题行**，块之间用空行分隔——**域名由每行的前缀携带，所以跨域嵌套行的域名依然一眼可见**。

加了块头之后，这个理由就不成立了：`cli` 块下挂着的 `design/target-honesty` 会被读成 cli 的 issue。候选：

| | 做法 | 代价 |
| --- | --- | --- |
| **A** | 跨域行带**例外标记**（如行尾 `(design)`），其余行纯裸 id | 仍有一个非裸名，但只在例外行出现 |
| **B** | 接受丢失：跨域行也只写裸 id，域去看 `status --json` / `list` | 视图里丢掉一个现在有的事实 |
| **C** | **不再跨域嵌套**：跨域子留在自己的 domain 块，跨域关系交给 `graph` | 显示层不再有「外来的行」→ 前缀彻底不需要；代价是推翻 §11.1 的落地，丢掉状态视图里唯一的跨域线索 |

> **已被「hypothesize（续）」取代**：裁定为候选 **C**，并且**连树形缩进一起取消**（不只是“跨域不嵌套”，而是 `status` 整个视图不再画树）。§4 的非目标与 §5 的落地清单也一并以那节为准。

#### 3.2 「JSON 行 = 文本行」的不变量：**顺序**一致，不是**行号**相等

- 硬事实：[cli_test.go](../internal/cli/cli_test.go) 的注释把它定义为「`status --json` 是**一层平铺行**（没有分组、没有子），**其顺序就是文本视图打印的顺序**」——**顺序**不变量。所以**加块头行不会破这个测试**。
- 但 [README](../README.md) 的措辞更强：「行序与文本视图**逐行一致**：第 N 行就是同一个 issue」——加块头后这句必须改写成「**issue 行的相对顺序**一致（块头行不参与）」，并明说**块头不入 JSON**（`domain` 仍是每行的独立字段，JSON 侧零信息损失）。
- 待定：`status --json` 要不要给每行加分组标识（如 `"block": 0`）？倾向**不加**——`domain` 已能重建分组，加了就是第二个真相。

#### 3.3 对齐与空行

- 现状：列宽 = **全部行**显示宽度的最大值（跨块统一对齐）；块之间打**一个空行**（`renderStatus` 的 `r.block`）。
- 要定：① 块头行是否参与列宽计算——**倾向不参与**（否则一个长域名把全表推宽）；② 块界表达是「块头 + 空行」（两行开销）还是**只用块头**（省一行/块，方向与 `issue-list-scaling` 同向）——**倾向只用块头**。

### 4. 非目标

- **不引入限定名输入**：`kiln path cli/x` 依然报错——本方向认为「让显示名能粘回去」比「让两种写法都能输入」更便宜（则 `hypothesize` 的候选 C「`issue:` trailer 也带前缀」**出局**）。
- 不动 `issue:` trailer / `engine` 键空间 / `check` / `list --domain=` / `--json` 的字段集合。
- 不动跨域**归属**的判定（`GroupByDomain` 的落位规则不变），只动「怎么写出来」。
- **不关闭**未来「限定名输入」的可能性：那条若还要做，属于 `issue-uniqueness` 一侧，不该由 `status` 的显示来夹带。

### 5. 落地时要一起改的地方（现状盘点）

| 位置 | 现状 | 要动什么 |
| --- | --- | --- |
| [`issueLabel`](../internal/cli/cli.go) | `p.Domain + "/" + p.ID`，全仓唯一调用点是 `statusLabel` | 拆成「块头」+「裸 id」两条渲染路径 |
| [mvp-plan](mvp-plan.md) §11 | 「不设目录标题行……域名由每行的前缀携带」 | 改成「块头携带域名」，并回答 §3.1 |
| [domain-status.md](domain-status.md) | 7 行示例，含跨域嵌套行 | 示例按新画法重画 |
| [README](../README.md) 机器可读输出 | 「行序与文本视图逐行一致：第 N 行就是同一个 issue」 | 改成「issue 行的相对顺序一致」 |
| [`cli_test.go`](../internal/cli/cli_test.go) | `TestStatusRendersDomainBlocks` 断言 `<domain>/<id>` 行 | 断言改为新画法；JSON 的顺序测试**不受影响** |

## hypothesize（续）：去掉嵌套——`status` 只管 domain 目录，关系归 `graph`

> 用户裁定：**显示 domain 分组名，同时去掉嵌套信息**。理由：后续 issue 之间的关系可能变复杂，**`status` 没必要承担这个职责**。
> → 上节 §3.1 的候选 **C** 胜出，并且**不只是“跨域不嵌套”**：`status` 整个视图不再画树（无缩进、无 `├──`/`└──`）。**本节取代上节的 §3.1 候选表与 §5 落地清单。**

### 1. 效果

```
问题总数: 33（完成 22 / 进行中 11）｜domain 2 个｜worktree off（已开 0 个）
kiln 动作数: 116

cli
  action-diff                    ✓ conclude
  qualified-issue-id               ask

design
  action-config                  ✓ apply
  target-honesty                   ask
```

两个直接后果：① 行内**全部是裸 id**（包括原先被嵌套的 `target-honesty`）→ 每行都能粘回命令，本 issue 的开题动因彻底消失；② `target-honesty` 回到 `design` 块——**块归属 = issue 自己的 domain**，一个 issue 只出现一次、只在一个块里。

### 2. 顺带的收益：**净删代码**

嵌套不是“少显示一点”，而是一整套为它而存的机制：

| 现有机制 | 只服务于嵌套 |
| --- | --- |
| `engine.Entry.Children` + `GroupByDomain` 里的 `parent` 映射 / `loops` / `buildEntry` | 跨域落位 |
| `engine` 的两个测试：`TestGroupByDomainRelocatesCrossDomainChild`、`TestGroupByDomainKeepsSameDomainFlat` | 同上 |
| `cli.statusRow.{depth, isLast, indent, parent}` + `statusWalk` 递归 + `statusLabel` 的 `├──`/`└──` | 文本视图的树 |
| `status --json` 的 `issues[].parent` + 它的文档 | 平铺后仍能表达落位 |

`GroupByDomain` 退化成「**分组 + 排序 + `(未分类)` 兜底**」——正好把 §11.2 里「已排序、**已嵌套**」的“已嵌套”删掉。

### 3. 一个非平凡的连带：`parent` 失去了指代物

- [machine-readable-output](machine-readable-output.md) 上一轮刚刚把 JSON 从 `groups[]→entries[]→children[]` 推翻成「顶层扁平 `issues[]` + 可选 `parent`」，并特意声明 `parent` 是「**落位**词，不是关系词」（“不能叫 `derived-from`，那等于声称一个它并不承载的关系”）。
- 文本视图一旦不落位，`parent` 就没有可指的东西了——它是**从嵌套里长出来的字段**，不是独立事实。→ **该字段连同它的说明一起退场**（[README](../README.md) 那句、machine-readable-output 的 §2.1 表格与 §2 那条“嵌套保留”的论证）。
- 代价可控：这是 `status --json` 的**破坏性**变更，但§11.3 已述本仓无外部消费者；而少一个“永远表述摆法”的词，比留一个失去指代的词干净。
- 附带好处：JSON 与文本的“**顺序**一致”不变量更好守了——两边都是“一个 issue 一行”，块头只存在于文本（JSON 的 `domain` 字段已能重建分组）。

### 4. 与 `relation-rendering` 的关系（新增一条边）

`cli/relation-rendering`（ask）的问题 4 逐字就是：「**`status` 要不要显示关系？**还是坚持 `status` 只管 domain 目录、把关系留给 `graph`？」——本轮裁定等于把它答了：**`status` 不显示关系**（既不画树、也不画边），关系归 `path`（过程）与 `graph`（关系网）。故 `relates-to relation-rendering`。

### 5. 落地清单（取代上节 §5）

| 位置 | 要动什么 | 量 |
| --- | --- | --- |
| `engine.GroupByDomain` | 去掉跨域落位；删 `Entry.Children`；退化成分组 + 排序 + `(未分类)` 兜底 | 删 |
| `engine` 测试 | `RelocatesCrossDomainChild` 删；`KeepsSameDomainFlat` 改成“跨域也平铺”；`OrdersAndBucketsUndomained` 保留 | 删 / 改 |
| `cli` | `issueLabel` 拆成“块头 + 裸 id”；`statusWalk` / `statusLabel` 的 depth / indent / `├──` 去掉；`statusRow` 瘦身 | 净删 |
| `status --json` | 删 `issues[].parent`（DTO 与赋值） | 删 |
| [mvp-plan](mvp-plan.md) §11 / §11.1 | 「每行写作 `<domain>/<issue-id>`、不设目录标题行」→「块头携带 domain、行内裸 id」；§11.1「跨域派生（嵌套）」**整节撤掉**；§11.2「已嵌套」删 | 改 |
| [machine-readable-output](machine-readable-output.md) | §2“嵌套保留”的论证 + §2.1 的 `parent` 行 | 改 |
| [domain-status.md](domain-status.md) | 示例重画（无嵌套、无前缀） | 改 |
| [README](../README.md) | `status` 那条：去掉 `parent`、“逐行一致”→“**issue 行的相对顺序**一致”；提交格式节无需改 | 改 |
| [`cli_test.go`](../internal/cli/cli_test.go) | `TestStatusRendersDomainBlocks` 断言新画法；JSON 期望里的 `"parent"` 删掉 | 改 |

### 6. 非目标（补一条，其余沿用上节 §4）

- **不动 `graph` / `path` 的关系渲染**（那是 `relation-rendering` 的地盘）；本轮只是**把 `status` 从关系渲染里摘出去**。
- **不删 `engine` 的关系数据**（`DerivedFrom` / `Spawns` / … 照旧解析、`check` 照旧校验）：本轮只是不再拿它决定 `status` 的**摆位**。

## collect：业内怎么编号，怎么处理重名

> 一轮联网调研（厂商官方文档为骨干，其余标注类型）。目的：看 issue / 工单 / 想法类工具**有没有命名空间**、**重名怎么处理**，再看本 issue 的候选该怎么收敛。来源见本节末。

### 1. 三种形态

| 形态 | 谁这么做 | 编号的样子 | 唯一性边界 | 跨边界怎么引用 |
| --- | --- | --- | --- | --- |
| **前缀命名空间**（前缀 + 序列号） | Jira、YouTrack、Linear、ServiceNow（按表）、ClickUp（可选开） | `PROJ-123`、`ENG-123`、`INC0012345` | 前缀在**实例/工作区内唯一**；序列号只在前缀内唯一 | 不用——ID 自带命名空间 |
| **全局单序列号** | Bugzilla、Azure DevOps、PEP、RFC、Debian BTS、Launchpad、Phabricator `T` | `#12345`、`T123`、`PEP 8` | **实例/组织内**唯一，且**不复用** | 不用；但跨实例/跨组织就有歧义（`AB#12`） |
| **容器限定**（短号 + 隐含上下文） | GitHub、GitLab `iid`、Canny、Notion | `#123` | **容器内**（仓库 / 项目 / board）唯一 | 升级成完全限定引用：`owner/repo#123` |

### 2. 命名空间不是"防重名"，是"让重名无害"

- **GitHub**：`#1` 在**每个**仓库都存在——全网有上百万个 `#1`，从不冲突；只有跨仓引用时才升级成完全限定形式（官方文档：同仓 `Closes #10`、跨仓 `Fixes octo-org/octo-repo#100`）。
- **GitLab** 自家 issue 的说法最直白：`id` 全局唯一，而 **`iid` 只在 namespace（group / subgroup / project）内唯一**；因此短引用是 `#123`、完全限定是 `group/subgroup/project#123`；epic 走另一套符号 `&123` / `group1/subgroup&123`。
- **代价也被用户点破**：GitLab 有人专门开 issue 要求"全局持久数字 ID"，理由是"我们有多个组件、bug 常被报到错误组件，需要一个能口头念、能贴进表格、跨组件唯一"的标识——**离开容器就得带上下文**，这正是容器限定的账单。
- 本仓实测：33 个 issue 的 `id` **无重复**（`kiln list --json` 里 `id` 无重值）——即当前重名不是现实问题。

→ 命名空间买到的是"**局部唯一就够用**"。它不减少重名**数量**，只是把重名限制在互不可见的容器里。kiln 会担心重名，恰恰因为它只有**一个**容器（仓库）。

### 3. 前缀是可变键：改名之后靠别名/重定向兜

| 工具 | 改前缀 / 迁移 | 旧引用的命运 |
| --- | --- | --- |
| **Jira** | 改 project key | 旧 key 的链接、JQL、REST 调用**继续可用**（`/rest/api/issue/EXAMPLE-100` 仍能取到）；旧 key 不能再给新项目用，且随时可改回 |
| **YouTrack** | 改 project ID | 旧 ID 存进 **Historical IDs**（只读）；用旧 ID 的**外部链接重定向**到新 ID |
| **Linear** | issue 移到别的 team | 得到**新 identifier**；旧 identifier 仍可搜索、旧 URL 重定向，但正文里手写的 `#ENG-123` **不会被改写**（只在点击时跳转） |
| **GitHub** | `gh issue transfer` 跨仓转移 | 社区 issue 抱怨"转移后 issue ID 变了"，写进 commit message 的 `#N` 链接因此断裂 |
| **beads**（社区，git 原生 tracker） | `--rename-prefix` | 只换前缀段，输出 `prefix_renames` 的 old→new 映射，并把旧 id 存进 `external_ref` |

- **共性：别名/重定向是数据库能力。** 每一个能在改名后不断链的工具，背后都有一张"旧名 → 新名"的表或一个可查询的别名层。
- kiln **没有数据库**：所有状态住在提交信息里。改 ID / 换前缀只能"改写历史里的全部相关 trailer"（[mvp-plan](mvp-plan.md) §10 已把 `kiln rename` 列为后续迭代）——成本高于上表每一家。上表的 `external_ref` 是现成的对照：**要么留一个旧名兜底字段，要么改名就是断链**。
- 另有一条实战翻车值得记：Jira 的**项目 name 可以与另一个项目的 key 撞名**，此时 JQL `project = ABC` 会产生歧义（官方 KB 的结论是"只按 key 查询"）。→ **显示名与寻址键一旦分裂，用显示名寻址就会翻车**——这正是本 issue 的起因。

### 4. 重名的另一种解法：重复是"关系"，不是"命名"

| 工具 | 机制 |
| --- | --- |
| **Bugzilla** | 字段 `dupe_of` + `duplicates` 表；resolution = `DUPLICATE`；还有按关键词"找疑似重复"的接口 |
| **GitHub** | 评论里写 `Duplicate of #N` → 时间线出现 "marked as duplicate" 事件，可 Undo；跨仓同样用 `owner/repo#N` |
| **Jira** | 默认 link type 里就有 `duplicates / is duplicated by`（与 `relates to`、`blocks`、`clones` 并列） |
| **Canny**（想法类） | API 直接给**合并**：`mergeIdeaID` → `intoIdeaID`；post 同时带机器 `id` 与人类可读的 `urlName` |

→ 没有任何一家做"**标题唯一**"约束；重名/重复**照单全收**，用一条关系把它们连起来。唯一性只在**标识**层面强求，从不在**名字**层面强求。

### 5. 主流是"两把键"：可读键会变，内部键不变

| 工具 | 可读键（人会改） | 内部键（不该变） |
| --- | --- | --- |
| GitHub | `number`（仓内） | `node_id` / GraphQL `id`（官方建议持久化它） |
| Jira | issue key `PROJ-123` | 数字 `id` |
| GitLab | `iid`（namespace 内） | `id`（全局） |
| Linear | `identifier` `ENG-123` | UUID |
| Canny | `urlName` | `id` |
| Notion | 标题（URL 里的 slug 段） | 页面 UUID，API 只用 `id` 寻址 |
| **kiln** | `issue:` trailer 的值 | **没有**——`issue:` 的值本身就是唯一键 |

→ kiln 只有**一把键**，没有"内部不可变 ID"可以退守（对比 GitHub 的 `node_id`）。**这把键一旦改名，就得改写所有引用它的提交**——这是与上表全部工具的**结构性差异**，也是本 issue 与 `issue-uniqueness` 共同的约束。

### 6. 单序列全局号：设计/想法类的先例

- **PEP**：「PEP numbers are assigned by the PEP editors, and **once assigned are never changed**」；分配时几乎总是取"下一个可用号"。
- **RFC**：「Each RFC is assigned a unique number and **these numbers are not reused**」；要改就**发一个新号取代旧的**（supersede + obsoletes）。
- **Bugzilla / Azure DevOps / Launchpad / Debian BTS / Phabricator `T`**：单序列、只增、不复用。

→ 当一个项目/生态只有**一个**规范的问题序列时，全局号既不需要命名空间、又比前缀短。**命名空间是"多序列并存"的产物**，不是"更高级的编号"。

### 7. 对 kiln 的事实映射（先摆事实，不下结论）

| 事实 | 出处 | 对本 issue 的含义 |
| --- | --- | --- |
| 唯一边界 = **单个仓库** | mvp-plan §2.4「唯一性范围：单个仓库内」 | "加 domain 前缀"不是新增容器，而是在**现有唯一容器**上再叠一个前缀 |
| **domain 不是容器**，它是动作上的可选 trailer | `p.Domain` 由**每个**带 `domain:` 的动作覆写（[engine.go](../internal/engine/engine.go)）；`issueLabel` 对空 domain 回退到裸 `id` | ①任意后续动作都能**改写**这个前缀 → 前缀是**可变地址**；②没声明 domain 的 issue **没有前缀** → 一旦接受限定名就是"两套写法并存" |
| 视图**已经**有一层 namespace，输入侧没有 | `issueLabel`（`status` 独有）vs `repo.Issues[...]` 直查 | 分裂已经存在；要么补输入侧，要么拆视图——**保持现状等于长期留一个"看得见、粘不回"的名字** |
| 展示的 `domain/` 前缀**不一定**等于 issue 自己的 domain | `GroupByDomain` 会把跨域子 issue **挪到父的块下**（`status --json` 用 `parent` 表达这点） | 块的位置是"视图摆法"，issue 自己的前缀是另一个事实——两者已经在同一个视图里同时出现 |
| 没有数据库，也没有第二把键 | 状态全在提交信息里 | 别名/重定向**不可得**；改名的代价高于本节全部工具 |
| 重名的检测点是**现成的** | `kiln check` 已是完整性校验的落点（**目前不查重**，只查起点/终点等不变量） | kiln 有"先允许重名、再由 `check` 报错"这条路——有 DB 的工具是**在建表时**就用约束挡掉，没有这个自由度 |

> 来源（厂商官方文档为骨干，其余标注类型）：[Atlassian：Editing a project key](https://confluence.atlassian.com/spaces/ADMINJIRASERVER/pages/938847080/Editing+a+project+key)（官方文档）｜ [Atlassian：Configuring issue linking](https://confluence.atlassian.com/spaces/ADMINJIRASERVER/pages/938847862/Configuring+issue+linking)（官方文档，默认 link type 含 `duplicates / is duplicated by`）｜ [Atlassian KB：Sprint 无法管理 / JQL 项目名与项目 key 撞名](https://confluence.atlassian.com/spaces/JIRAKB/pages/1125881102/Some+Jira+users+can+t+manage+a+Sprint+create+start+complete+edit+due+to+lack+of+permissions)（官方 KB）｜ [Atlassian KB：JCMA 迁移预检的项目名 / 项目键冲突](https://support.atlassian.com/atlassian-cloud/kb/project-name-and-project-key-conflict-when-migrating-using-jira-cloud-migration-assistant-jcma)（官方 KB）｜ [GitHub Docs：Linking a pull request to an issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/linking-a-pull-request-to-an-issue)（官方文档）｜ [GitHub Docs：Using global node IDs](https://docs.github.com/en/graphql/guides/using-global-node-ids)（官方文档）｜ [GitHub Docs：Marking issues or pull requests as a duplicate](https://docs.github.com/en/issues/tracking-your-work-with-issues/administering-issues/marking-issues-or-pull-requests-as-a-duplicate)（官方文档）｜ [GitHub Docs：Using keywords in issues and pull requests](https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/using-keywords-in-issues-and-pull-requests)（官方文档）｜ [GitHub Docs：Transferring an issue to another repository](https://docs.github.com/en/issues/tracking-your-work-with-issues/administering-issues/transferring-an-issue-to-another-repository)（官方文档）｜ [cli/cli#8686](https://github.com/cli/cli/issues/8686)（社区 issue：转移后 issue ID 变 / commit 里的链接断）｜ [GitLab Docs：Issues API](https://docs.gitlab.com/api/issues)（官方文档：`id` / `iid` / `closed_as_duplicate_of`）｜ [gitlab-org/gitlab#482027](https://gitlab.com/gitlab-org/gitlab/-/issues/482027)（GitLab 自家 issue：`id` 全局唯一、`iid` 只在 namespace 内唯一；短引用 vs 完全限定；epic `&123`）｜ [gitlab-org/gitlab#24821](https://gitlab.com/gitlab-org/gitlab/-/issues/24821)（GitLab 自家 issue：要求全局持久数字 ID，跨组件歧义）｜ [Microsoft Learn：Key concepts, objects, and terms（Azure DevOps）](https://learn.microsoft.com/en-us/azure/devops/project/navigation/glossary?view=azure-devops)（官方文档：work item ID 在组织 / 项目集合内唯一）｜ [Microsoft Learn：Connect Azure Boards to GitHub](https://learn.microsoft.com/en-us/azure/devops/boards/github/connect-to-github?view=azure-devops)（官方文档：ID 跨组织不唯一 → `AB#12` 歧义）｜ [Bugzilla：Bugs API（`dupe_of`）](https://bugzilla.readthedocs.io/en/latest/api/core/v1/bug.html)（官方文档）｜ [Bugzilla Schema 5.2（`duplicates` 表 / resolution `DUPLICATE`）](https://schema.bugzilla.org?action=single&version=5.2&view=View+schema)（官方文档）｜ [Linear Docs：Issues](https://linear.app/docs/creating-issues)（官方文档：移到别的 team → 新 identifier、旧 identifier 仍可搜索、旧引用不改写）｜ [Linear Docs：Projects（`P-TEAM-123`）](https://linear.app/docs/projects)（官方文档）｜ [YouTrack Docs：Configure a Project](https://www.jetbrains.com/help/youtrack/server/configuring-a-project.html)（官方文档：project ID 唯一、作 issue 前缀、Historical IDs 重定向）｜ [Canny API Reference](https://developers.canny.io/api-reference)（官方文档：post `id` + `urlName`、`mergeIdeaID` → `intoIdeaID`）｜ [Notion API：Page](https://developers.notion.com/reference/page)（官方文档：页面 `id` 为 UUID，`url` 另给）｜ [PEP 0](https://peps.python.org)（官方文档：编号由编辑分配、一旦分配永不改）｜ [PEP 1](https://peps.python.org/pep-0001)（官方文档：几乎总取下一个可用号）｜ [draft-eastlake-test-rfc-numbers](https://www.ietf.org/archive/id/draft-eastlake-test-rfc-numbers-01.html)（IETF 草案：每个 RFC 唯一编号、不复用）｜ [Wikipedia：Request for Comments](https://en.wikipedia.org/wiki/Request_for_Comments)（二手：被取代即发新号）｜ [Gerrit：Change-Ids](https://gerrit-review.googlesource.com/Documentation/user-changeid.html)（官方文档：Change-Id **不保证**实例内唯一，可在不同 repository / branch 间复用）｜ [Gerrit：User Guide](https://gerrit.cloudera.org/Documentation/intro-user.html)（官方文档：Change-Ids are unique for a branch of a project）｜ [ClickUp Help：Create Custom Task IDs](https://help.clickup.com/hc/en-us/articles/6304361756951-Create-Custom-Task-IDs)（官方帮助页；正文被 Cloudflare 拦，仅标题可核）｜ [ClickUp 反馈：search by custom task id](https://feedback.clickup.com/feature-requests/p/search-by-custom-task-id-in-list-view)（社区：必须带完整前缀才搜得到，用户抱怨 `[DEV-1232]` 要手打进标题）｜ [ServiceNow Community：改 INC 前缀](https://www.servicenow.com/community/developer-forum/change-my-incident-numbers-from-starts-inc-to-incq/m-p/2851113)（官方社区：Number Maintenance 按表设前缀，**改前缀不回溯已有记录**）｜ [ServiceNow 观察记：sys_number 与 INC 编号规则](https://note.com/u_sn_notes/n/n40cfb8c4d5d2)（二手：编号 = prefix + 补零序号，一表一行规则）｜ [beads_rust README](https://github.com/Dicklesworthstone/beads_rust/blob/main/README.md)（社区 / 开源，git 原生 tracker：前缀式 ID、`--rename-prefix` 输出 old→new 映射、旧 id 存 `external_ref`）｜ [git-bug](https://github.com/git-bug/git-bug)（开源，git 内嵌 tracker；其 ID 为内容哈希、按"最短唯一前缀"寻址，见[社区实操文章](https://dev.to/jamilxt/this-bug-tracker-lives-inside-your-git-repo-a-hands-on-guide-to-git-bug-3bjp)与 [HN 讨论](https://news.ycombinator.com/item?id=43971620)：用户想要的正是 `ABC-123` 这种人手写的 key）
