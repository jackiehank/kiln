# issue: issue-list-scaling — `status` 等列出 issue 的命令要不要全量输出

- **domain**：cli
- **关联**：relates-to `machine-readable-output`

## 背景

列出 issue 的三个命令——`status`、`list`、`graph`——现在**都是全量**：没有上限、没有分页、也没有"只要摘要"的模式。

- `status`：头两行给计数（问题总数 / 完成 / 进行中 / 动作数），正文按 domain 分组**逐条列出全部**——"总量"与"逐条"都出。
- `list`：只有显式加 `--domain` / `--tag` / `--action` 才变短，否则全量。
- `graph`：每个 issue 一块，全量。

现在 30 条还读得动。**几十上百条时**，压过来的是三件事：

1. **文本视图刷屏**：一屏看不到头，"扫一眼"这件事失效——而"扫一眼"正是 `status` 存在的理由。
2. **agent 的 token 成本线性**：一次 `status` 就得吞下整份输出，跟规模一起涨。`--json` 也没解决这件事——`machine-readable-output` 只把体量从 O(commit) 压到 O(issue)，**没有封顶**。
3. **读命令本身的代价线性**：每次 `status` / `list` 都要 `engine.Build` **聚合整段历史**（issue 上百 → 动作上千）。而 kiln 的一条立场是**不建索引、不建数据库**（状态只住在提交信息里）——规模化和这条立场之间是有张力的，得正面回答，而不是等它自己爆。

还有两处相邻的既有讨论，别绕开：

- `machine-readable-output` 刚把 `status` 从"分组嵌套"改成**平铺的一层行**（行数 = issue 数）。那次简化**放大**了"行数会不会失控"这个问题——从三层嵌套里的一层，变成了整份文档的主体。
- `cli-necessity` 提过"kiln 若要更强的查询能力，可参考 revset"。但那是**查询语言**的问题，与本 issue 不同：这里问的是**默认给多少**。

## 要回答的问题

1. **默认要不要全量？** `status` 的语义就是"全库概览"，而**截断会静默破坏它**——读者（尤其 agent）会以为看全了，这是最危险的一种"少输出"。备选：全量 / 只列进行中 / 只列"有变化的" / 只给计数。
2. **截断的判据是什么？** 阈值（前 N 条）？按 domain？按状态（进行中优先）？按最后动作的时间？——判据不同，丢的东西不同，得先定**顺序**，否则"截断丢什么"就是随机的。
3. **文本与 JSON 要不要一致？** 还是**文本摘要、JSON 全量**？这是分层的一个天然位置：人读视图怕长，机器视图不怕（消费方自己会筛）。若两者不一致，必须写明**以哪份为准**，否则又是一个静默分歧。
4. **"少输出"该靠筛还是靠截？** 复用并扩展过滤（`list` 已有 `--domain/--tag/--action`，可加 `--status=进行中`），还是引入 `--limit` / `--offset` / 游标？**筛**不丢信息、**截**才需要"还有 N 条"的补救提示。
5. **要不要"只要数不要列"的模式？** `--count` / `--summary` 之类——CI 只想问"有没有进行中的"时，不必拉全表。
6. **默认值与可配性**：阈值写死、放 `.kilnconfig.jsonc`、还是只走 flag？注意 `worktree-hint` 立下的那条边界的精神——**运行时的显示偏好，不该是"改仓库级配置"的事**，更像每次调用的事。
7. **三个视图的"看不全程度"要不要统一？** 若 `status` 截断而 `check` 不截断（它本来就逐 issue 一行），同一个仓库里"哪份是全集"就说不清了。
8. **成本怎么摊？** 维持"每次重算、无索引"，把规模化交给筛选？还是承认需要缓存 / 增量读取？（**不建索引**是立场，还是可讨论项——这一条要单独表态。）

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 不做 | 维持全量 | 语义最简单；规模化后刷屏、token 线性 |
| **B** 文本截断 + 提示 | 文本给前 N，尾注"还有 M 条（`--limit` / `--json` 看全）" | 需先定 N 与**排序**；"概览 = 看全"的直觉被打破 |
| **C** 文本摘要 + JSON 全量 | 文本只给计数 + 进行中；`--json` 全量 | 两个视图不一致，要写明以哪份为准 |
| **D** 默认只给计数 | `status` 默认只出汇总，逐条列表明示 opt-in | "扫一眼"最快；但失去一屏总览的能力 |
| **E** 靠筛不靠截 | 默认改成"进行中优先 / 只进行中"，全量走 flag | 不改输出机制、不丢信息；但仍要面对"全量时照样刷屏" |
| **F** 分页 | `--limit` / `--offset` | 多一组状态要管；agent 得循环拉取 |

> 倾向（**待验证**）：**先把"人读"与"机器读"分成两条路**——文本视图默认**摘要化**（计数 + 进行中，全量明示）；`--json` **保持全量**（体量 O(issue) 尚可，本来就是给程序筛的）。阈值**不写死进配置**，先用 flag 探手感。无论选哪条，**顺序与判据要先定**：不知道"谁排前面"，截断丢什么就是随机的。

## 非目标

- 不引入索引 / 数据库 / 缓存——那是**另一个决定**，要单列（本 issue 只在问题 8 里点一下张力）。
- 不改 `check` 的逐条输出（它的粒度本来就是"每条一行"）。
- 不管 `path` / `show`：单 issue，天然不长。

## 备注

- 来源：用户在会话中提出——"status 等列出 issue 的命令，输出时要不要全量输出，以后 issue 几十条上百条怎么办？"。用户明确要求**先记、稍后再处理**。
- 与 `machine-readable-output` 相邻：那一轮把 `status` 定成平铺的一层行、把 `list` 定成"过滤但全量"，本 issue 接着问"行数失控怎么办"。
- 与 `cli-necessity` 的"查询能力"一条相邻但不同：那是**查询语言**，这里是**默认输出多少**。

## 收集（2026-10-27）

本轮先核实"现有信息"，不裁决。**问题被重新校准了一句**：原文问"要不要全量"，而实际痛点是"issue 多了以后**找不到**某条信息"；用户在下游 `dsh-kiln` 插件里用的办法是**筛选 + 搜索**。所以下面核实的是：现有筛选到什么程度、有没有搜索、缺什么。

### 1. 现仓基线（本仓 60 issue / 260 动作 / 3 domain）

| 出口 | 体量 | 说明 |
| --- | --- | --- |
| `status`（文本） | 2,552 B / **70 行** | 60 条 issue 全量 + 3 个块头 |
| `status --json` | 17,365 B | 行有 `id/domain/action/done/title/has-worktree` |
| `list --json` | 6,933 B | 行**只有** `id/domain/action/done`——**没有 title** |
| `graph --json` | 25,659 B | 60 节点（`title` 有则给）、78 边 |
| `path --all --json` | 121,008 B / 260 动作 | 单进程；单条 path 平均 ~2 KB、最大 7.3 KB（`relation-vocabulary`，18 个动作） |
| 域分布 | cli 30 / design 29 / tooling 1 | 域筛选最多只砍一半 |

耗时（三次取最快）：`--version` 45 ms（启动地板）、`list` 99 ms、`graph` 100 ms、`path --all --json` 102 ms、`status` 150 ms。**读一次全史 ≈ 100–150 ms**，搜索/筛选若在同一份历史里做，不额外读盘。

### 2. kiln 现有筛选（核实结果）

- **按 issue 名取路径**：`kiln path <id> --json` 可用，但是**精确 id**，没有前缀/模糊匹配；单条平均 ~2 KB。
- **域筛选**：`kiln list --domain=<d>` 可用（cli 30 / design 29 / tooling 1），但三个短板：
  1. `list` 的行**不带 title**（`status --json` 才带），筛完还剩"这 id 是什么事"要再查；
  2. **没有 `--status`**（进行中/完成）；
  3. 未知 flag **只警告不报错**：`kiln list --status=open --json` 打一行警告后仍**全量 60 条**（exit 0）——看起来筛了，其实没筛。
- **带标题的出口**只有 `graph --json`（节点有 `title/domain/tags`）与 `status --json`；但 `graph` 只支持 `--issue/--depth` 收窄**邻域**，不能按 domain 筛；`status` 没有任何过滤 flag。
- 因此今天**没有任何一条命令能"按 domain 筛 + 带标题"**：要么 `list --domain`（无标题），要么 `graph --json` 全量后客户端筛（下游插件正是这么做的），要么逐条 `path`。
- 组合筛选是 AND（`--domain` × `--tag` × `--action`）；`--tag` 是 issue 级并集（`list --tag=json` 命中 1 条）。

### 3. dsh-kiln 现状（下游真实消费者）

- **取数**：一次快照 = 1 次 `kiln graph --json` + **每条 issue 一次** `kiln path <id> --json`（N ≤ 80、并发 8）；冷启动实测 **1.29 s / 59 条**（dsh-kiln `c58e166` 记录），HEAD 令牌增量约 0.2 s。
- **筛选**：issue 页按 **domain / 状态**，时间线页按 **动作 / domain**；组内 OR、组间 AND；**纯客户端**（不调 `kiln list`）；tag 刻意不作为筛选维度（动作级 trailer vs issue 级并集）。
- **搜索**：页头常驻输入框，输入即搜。检索面 = **issue id + 当前标题 + 所有动作标题**；空白切词、大小写不敏感、词间 AND；排序 **id 前缀 > id 命中 > 标题命中 > 动作标题命中**；命中片段高亮，回车按命中类型跳 issue 页或时间线。**不含** tag / domain / 关系名 / 提交正文。
- **它记的上游缺口已补**：插件 README 仍写"`kiln path a b --json` 静默只回第一个，要再快只能等上游给批量接口"——本仓刚落地 `path --all --json`（一次进程 60 条 / 260 动作 / 182 ms；见 `docs/issues/path-batch.md`）。插件**尚未迁移**到批量，仍逐条取。

### 4. 搜索的空白（留给下一步，不在本轮定）

1. **归属**：只留在插件 = 两套语义（插件能搜，CLI/agent 不能；agent 现在要搜只能自己读 `graph --json` 再在内存里匹）；进 kiln = 要定匹配面、排序、输出形态。
2. **检索面**：与插件同口径（id + 标题 + 动作标题）最省；要不要加 `tags/domain/refs/scope/target`，或加**提交正文**？正文本来就在 `Log()` 读进来的消息里，**不加进程、不加 IO**，但噪声（trailer 行也会命中）与"搜到的是不是用户想找的"要定。
3. **形态**：新 `kiln search <词…>`，还是 `list --match <词…>` 与现有 `--domain/--tag/--action` 做 AND？后者不引入新排序，前者可做 id 前缀优先。
4. **顺带缺口**：`--status`（进行中/完成）与 `--limit/--offset` 都还没有；但"未知 flag 只警告"意味着加这些之前，误用会静默回全量。

### 5. 对原问题的一句话校准

按实测，**"少输出"不是首要矛盾**：100–150 ms、几十 KB、70 行，都还读得动；矛盾是"**找得到**"。默认截断会破坏"全库概览 = 看全"的语义，而筛选/搜索是**无损**路线——这与候选 E 同向。但这只是收集，**是否加、加在哪、搜什么**尚未决定。

## hypothesize（2026-10-27）：搜索以 `list` 的过滤条件落地，不截断

> **主张**：kiln 要的是**过滤式搜索**，不是搜索命令。具体是 `list --match <文本>`（可重复、AND、大小写不敏感、无排名），检索面 = **id + 当前标题 + 每次动作的 subject**（即 `path` / `graph` 里看得见的字）；`list` 行同时补 `title`，让筛出来的结果能自解释。**不搜提交正文**（本仓 261 条动作提交的正文里没有散文），**不截断、不分页**；逐键交互式搜索继续留在 dsh-kiln。

### 1. 为什么"正文检索"不是一条路（本轮最强的事实）

对本仓 274 条提交逐条数过：

| 事实 | 数字 |
| --- | --- |
| 带 `action:` trailer 的动作提交 | **261**（kiln 的 260 条动作 + 本文前一条 `collect`） |
| 其中 `title` == 提交首行（subject） | **260** |
| 只有首行、没有 `title` 的动作 | **1**（`f8413d8`，`help-relation-flags` 的 hypothesize） |
| 动作提交里"trailer 之外的正文行" | **0** |
| 有正文散文的提交 | **3**，全部是**未带 `action:` 的早期提交**（`f25ed6a` / `829d406` / `f703bb9`） |

也就是说：kiln 的提交正文**就是 trailer**，能搜的散文只有标题（= 首行）。`Log()` 已经把消息读进来了，所以加"正文检索"不增加 IOC，但也**搜不到任何标题之外的东西**——它只会命中 trailer 行（`issue:` / `action:` / `refs:` 之类）和那 3 条历史提交。结论：检索面不必包含正文，**"标题 + 动作 subject" 已是全部语料**。

### 2. 主张与证伪条件

| # | 主张 | 证伪条件 |
| --- | --- | --- |
| **H1** | 不截断、不分页：`status` / `list` / `graph` 维持全量 | 单仓到数百 issue 时，全量文本的阅读成本超过"删信息"的收益（要带 N 与行数实测） |
| **H2** | 搜索只做 `list --match`，不加 `search` 子命令 | `--match` 的命中理由无法在 `listDoc` 里表达（必须换行型 / 多行结果）时，独立命令更合适 |
| **H3** | 检索面 = id + 当前标题 + 动作 subject；不搜正文 | 提交约定改成正文写散文（"why" 段落）且占比可观（>10%） |
| **H4** | 过滤结果必须自解释：`list` 行补 `title`；命中动作时带 `match` | 若真实查询都命中 issue 标题本身（行内已显示），`match` 可省 |
| **H5** | 逐键搜索留插件；kiln 只做一次性、可组合的过滤 | 出现"CLI 里也必须按键即搜"的真实场景（目前是 UI 需求，不是 CLI 需求） |

**H4 会推翻一条既有决定**：`json-fields` 的非目标写过"不给 `list --json` 加 `title`，公共行 `issueCore` 保持不变"。现在有新证据：`list` 是唯一按属性过滤的出口，行里却只有 `id/domain/action/done`——筛完还得逐条 `path` 才知道"是什么事"，正是本 issue 要消掉的 N+1。

### 3. 对原 issue 八问的回答

| 原问题 | 本假设的回答 |
| --- | --- |
| 1 默认全量？ | **是**，三个视图都维持全量 |
| 2 截断判据？ | **不截断**，所以不需要判据 |
| 3 文本与 JSON 一致？ | 一致：都全量；`--match` 时两边同源（同一份行集） |
| 4 少输出靠筛还是靠截？ | **靠筛**；`--match` 只是又一个筛选条件 |
| 5 只要计数？ | 暂不做（`status` 头两行已是计数）；无证据要 `--count` |
| 6 默认值可配性？ | 不写配置，全是每次调用的 flag（沿用 `worktree-hint` 的边界） |
| 7 三视图看不全程度统一？ | 统一：谁都不截 |
| 8 成本怎么摊？ | 不建索引；读全史 100–150 ms，匹配在已读入的内存里做，不额外 IO |

### 4. 为什么不是别的位置

- **不给 `status` 加过滤**：`status` 是"看全"的概览（分组 + 计数 + JSON 行有 title），`list` 才是"筛"。两边都加会分裂同一套过滤语义；`list` 补 `title` 后，"筛 + 认出是什么"就齐了。
- **不把交互式搜索搬进 CLI**：插件是**每次按键**就搜（快照在内存）；CLI 每次调用 = 一次进程（100–150 ms），只能做一次性查询。搬进来不会减少插件的实现，只会多一套匹配语义。
- **不新增 `search` 命令**：见 H2。真出现"多行结果 + 排序 + 命中理由"的需求再升级，不一步到位。

### 5. 若成立，落点（按依赖顺序，分动作）

1. `list` 行补 `title`（最后一个动作的 title；**缺则回退 subject**——`f8413d8` 那种动作的 `title` 就是空的（`path --json` 现在给它空标题），issue 级虽然还没出现"末条动作无 title"，逻辑上要兜住）；
2. `list --status=open|done`（值名待定）；
3. `list --match <文本>`（可重复；**不按逗号切**，否则短语会被拆碎）；命中动作 subject 时 JSON 行带 `match`，文本行至少显示 id + 标题；
4. `list` 把**未知 flag 改为报错**——否则 `--matc` 打错就静默回全量（collect 里 `--status` 已复现这一现象）；
5. dsh-kiln：N 次 `path` → 1 次 `path --all`（另一仓，消费侧）。

### 6. 可跑的检验

1. **召回对拍**：12 条查询（3 条 id 片段、3 条 issue 标题词、3 条动作标题词、3 条只在 3 条早期提交正文里的词）→ `list --match --json` 与插件 `searchIssues` 的 id 集应一致，正文那 3 条应 0 命中；不一致先定契约（谁为准）。
2. **输出体量**：按当前语料，`--match=worktree` 10 条、`=json` 12 条、`=关系` 11 条、`=path` 11 条、`=搜索` 1 条、`=i18n` 1 条 → 期望 1–12 行、约 0.2–2 KB，对比 `graph --json` 25.7 KB、`path --all` 121 KB。宽查询（`=a`）接近全量是查询质量问题，不是命令缺陷。
3. **成本**：`list --match` 最优三次与 `list` 同量级（±20 ms）；若 >2× 则重估。
4. **可解释性**：统计"命中动作标题但不是 issue 标题"的比例；若 >30%，`match` 字段是必需。
5. **使用频率**：落地后观察 agent 是否用它替代 `path --all`；若没人用，说明"在内存里筛"已够，H2 可回退。

### 7. 不做

- 不加 `search` 子命令；不搜正文；不做相关性排序（排序是 UI 的事，插件已有）；不加 `--limit/--offset`（没有证据）；不建索引；不改 `status` / `graph` 的默认输出。

### 8. 反方（保留）

- **`graph --json` 已够 agent 用**：25.7 KB 里就有 id/title/domain/done；若 agent 只按 issue 级找，`--match` 只是省 token，不是新能力。先做 H4 的 title，再观察是否真的需要 match。
- **两套匹配**：插件与 CLI 各一份"包含"逻辑；不做 §6.1 的对拍，语义会慢慢分叉——要先声明谁是契约（倾向 kiln 定义、插件对齐）。
- **正文约定会变**：一旦要求动作提交写"为什么"段落，H3 失效，检索面必须重议。

## apply（2026-10-27，第一落点：`list` 行补 `title`）

只落 H4 的第一项；`--status` 的**取值名**（"done/完成"与"`conclude`、`apply` 之后仍可继续"这件事有歧义，用户点名要再探讨）与 `--match` 都还没定。

- `internal/cli/cli.go`：`listIssue` 增 `Title string`（`json:"title,omitempty"`）；`list --json` 经 `issueTitle(p)` 取——**最后一个动作的 `title`，缺则回退该动作的提交首行**（`f8413d8` 是这种：首行就是 `issue: help-relation-flags`，`title` 为空）。`issueCore` 未动。
- **文本视图不动**：`list` 文本仍只有 id / 状态 / domain；标题进文本的时机留给 `--match`（那里才有"命中理由"要显示）。
- 文档：`docs/machine-readable-output.md` §3 的 list 字段表、示例与共享契约补 `title`；`docs/json-fields.md` 记下其非目标被推翻的原因。
- 测试：`TestListJSONCarriesTheIssueTitle`（三行 title 与最后动作一致）、`TestListJSONFallsBackToSubjectWithoutTitle`（末条动作无 title → 首行）；既有 `TestListJSONSharesTheIssueRow` 的 golden 补 `title`。
- 复核：`gofmt` / `go vet` / `go test ./...` 全过；真仓 `list --json --domain=cli` 冒烟（30 行、每行带 title）。

**这不是完成**：本条按 `apply` 记（`kiln status` 会显示已完成），但 `--status` 取值名与 `--match` 仍待议；下一次讨论会以新动作把它拉回"进行中"。

## apply（2026-10-27，第二落点：`list --status=open|done`）

命名讨论的结论：**展示层不改**（`status` 照旧用 `✓` / 已完成 / 进行中），筛选层用 `--status=open|done`；`done` 就是行里那个 `done`（末条动作属于 `actions.terminal`），`open` 是补集。唯一要教给用户的一句是"终点动作之后仍可继续"，按用户的位置决定，写在 **`-h` 的 do/commit 说明里**（读命令的 `list` 帮助只写取值含义，不展开生命周期）——`list` 是读，`do`/`commit` 才是写入侧。

- `internal/cli/cli.go`：`list` 接受 `--status`，非法值（含 `--status` 不带值）**exit 2**、stdout 0 字节；合法则先走 `engine.Filter`（domain/tag/action），再用 `e.filterStatus` 按 `isDone` 过滤——**与行里的 `done` 同一谓词**，筛选与展示不可能分叉。
- `filterQuery` 增 `Status string`（`json:"status,omitempty"`），`--status` 原样回显在 `filter` 里。
- 帮助：`usageCommands` 的 list 行加 `[--status=open|done]`；`usageOtherFlags` 增一行 `list 的 --status: open（未收尾）| done（末条动作是终点动作）`；`do/commit` 的「说明」后补一句 **"终点动作（默认 conclude / apply）之后仍可继续接动作——issue 不会被关闭。"**（中英同步）。
- 文档：`docs/machine-readable-output.md` §3 的 filter 字段、示例与要点补 `--status`（含"done ≠ 永久关闭"）；README 双语命令表的 list 行补该 flag。
- 测试：`TestListStatusFiltersOpenAndDone`（done 只留 parent；open 只留 child/stray；`open ∩ domain=cli` 只剩 child；文本视图同口径）、`TestListRejectsAnUnknownStatus`（`--status=closed` → exit 2、stdout 空、stderr 指 `open` / `done`）。
- 复核：`gofmt` / `go vet` / `go test ./...` 全过；真仓 `list --status=open --json` 冒烟（60 条里 7 条未收尾，与 `status` 的"进行中 7"一致）。

**仍未做**：`--match`（hypothesize 的 H2/H3）与"命中动作标题时的 `match` 字段"；`list` 文本仍不带 title。

## collect（2026-10-27）：CLI 搜索的成熟方案盘点

问题：第三落点 `--match` / `search` 该长什么样。先看广泛采用的 CLI 搜索怎么设计——只记录，不裁决。

### 1. 两种形态，而且不互斥

- **独立搜索动词**：`grep` / `rg` / `fd` / `git grep` / `gh search issues` / `brew search`。
- **列表上的过滤/搜索**：`gh issue list --search` / `kubectl get -l` / `docker ps --filter` / `git-bug ls [QUERY]`。
- 同一工具常常两者都有：gh 既提供 `gh search issues`（独立输出、相关度排序），也提供 `gh issue list --search`（复用列表行）。所以"独立命令还是 flag"不是二选一，真正的问题是**要不要独立的输出与排序**。
- **查询串**：gh 支持 `label:bug author:@me` 这类限定符与 OR/AND（布尔要作为独立参数传，不能包在一个多词字符串里）；git-bug 支持 `status:open sort:edit-desc` + 自然语言全文，且能与 flag 混用。
- 连 gh 都把语义检索做成了 opt-in：`--search-type lexical|semantic|hybrid`，**默认仍是 lexical**。

→ 共识：**属性条件用 flag/限定符，自由文本只负责"记得的词"；布尔与语法是专家层。**

### 2. 默认匹配语义（手感的核心）

| 工具 | 默认匹配 | 大小写 | 多词 |
| --- | --- | --- | --- |
| `grep` | 基本正则 | 敏感（`-i` 才忽略） | 多个 `-e` = OR |
| `rg` | 正则 | 敏感（`-S` = smart-case） | 多个 `-e` = OR |
| `fd` | 正则 | **smart-case 为默认**（含大写才敏感） | 单个 pattern |
| `git log --grep` | 基本正则 | 敏感（`-i`） | 多个 `--grep` = OR；`--all-match` 才 AND |
| `gh search` / `gh issue list --search` | 关键词（GitHub 搜索语法） | 不敏感 | **AND** |
| `git-bug ls` | 自然语言全文 + 查询串 | — | 词一起给 |
| `fzf` | 模糊 | 不敏感 | 空格 = AND，`\|` = OR，`!` = 非 |

两条线很清楚：**文件/行搜索器默认正则（grep 家族），issue/记录搜索默认关键词 AND（gh / GitHub / git-bug）**。kiln 的"记得一个词"属于后者——"默认正则"不是搜索的普遍共识，只是 grep 家族的传统；GitHub 的搜索默认也不是正则。

大小写：grep 家族默认敏感但都提供 `-i` / smart-case；issue 类搜索（gh / GitHub）默认不敏感。`fd` 把 smart-case 做成默认，是"手感更好"的代表。字面量兜底也普遍存在（`rg` / `git grep -F`、`git grep -E`、fzf 的 `'` 精确匹配）；否定（`-v` / `!` / `-label:`）是选项，不是默认。

### 3. 检索面与字段收窄

- gh：默认搜 title + body + comments，`--match {title|body|comments}` 收窄；网页语法有 `in:title`。
- `git log --grep` 只搜提交信息；`-S` / `-G` 才搜 diff；`git grep` 搜内容。
- git-bug：自然语言全文 + `--title` 等 flag。
- `rg` / `fd`：内容 / 文件名，`-g` / `-t` 限定范围。

→ 共识：默认面可以宽，但**必须有"只搜某字段"的开关**；"标题优先"是 issue 工具的常见诉求（gh 的 `--match`、git-bug 的 `--title`）。

### 4. 排序、limit 与截断

- 非交互默认稳定序：`git log` 时间倒序、`rg` 文件遍历顺序（`--sort path` 可确定化）、git-bug `--by creation`、`gh issue list` 默认 open + 最近 30。
- 相关度是交互/服务端的活：`gh search` 默认 `--sort best-match`、`fzf` 按模糊分数排；**文件搜索器没有相关度**（rg 按遍历序）。
- limit：gh 两个命令都 `--limit` **默认 30**（文档明确"默认只看最近 30"）；npm search `--searchlimit` 默认 20；kubectl/docker 不截断；`rg -m` 按文件限。
- 截断可见性：gh 用"most recent 30"说明，但没有"还有 N 条"的尾巴——这是它的短板。

→ 共识：**非交互不搞相关度排序**（留给 fzf / 服务端）；**有默认 limit 就必须让人知道被截断**。kiln 现在不截断，反而少一个坑。

### 5. 输出与退出码

- grep 三态：**0 有命中 / 1 无命中 / 2 错误**；`-l` / `-c` / `-n`；`rg --json` 是 begin/match/end 事件流；颜色只在 TTY。
- gh：人读表格；`--json <fields>` + `--jq` / `--template` 是机器契约；`--limit`。
- kubectl / docker：空结果是表头 + "No resources found"（stderr）、**exit 0**。
- fzf：没选中就没有 stdout（exit 1 / 130）。

→ 两条约定：**搜索动词遵循 grep 三态**（"没命中"是答案，exit 1）；**列表过滤器 exit 0 + 空表**。kiln 的 `list` 属于后者（`--domain` / `--status` 空结果 exit 0，与 kubectl / docker 一致）；只有独立 `search` 才需要决定要不要学 grep 的 1。

### 6. 交互式与管道

fzf / skim 把"交互式搜索"做成独立工具（TUI、模糊评分、预览、多选），靠管道组合（`rg … | fzf`），不塞进数据 CLI。dsh-kiln 的搜索框已经是这一层，kiln 不需要 TUI。

### 7. 广泛接受的约定（清单）

1. 记录类搜索默认**关键词 / 字面量子串**，正则 `--regex` 是专家开关；文件类才默认正则。
2. **多词默认 AND**（grep 家族的 OR 是例外，git 用 `--all-match` 补上 AND）。
3. 记录类**大小写不敏感**；文件类敏感 + smart-case。
4. 字段收窄有开关（`--match` / `--grep` / `-t`）。
5. 结构化条件用 **flag / 限定符**，不发明自由文本里的魔法词。
6. 非交互用**稳定顺序**，相关度留给交互层。
7. **有 limit 就显示截断**；没有 limit 就别假装有。
8. **搜索动词 grep 三态；列表过滤器空结果 exit 0**。
9. **颜色只在 TTY；JSON 是机器契约**（gh 的 `--json <fields>` 是标杆）。

### 8. 对 kiln 的含义（观察，不是决定）

1. **形态**：gh 的双形态说明 `list --match` 与独立 `search` 都正当；区别在"要不要独立输出 / 排序"。kiln 现在只有一种行（`listDoc`），`--match` 能直接复用，独立 `search` 才需要新输出。
2. **默认语义**：kiln 的语料是标题（无正文散文），"记得一个词"→ **字面量子串 + 多词 AND + 大小写不敏感**是共识；正则应 opt-in（这个默认与 gh 一致，不是与 rg 一致）。
3. **字段**：默认搜 id + 当前标题 + 动作 subject 已覆盖"记得的词"；`--match=title|action` 这类收窄可以晚点加（gh 也是显式字段）。
4. **结构化**：domain / tag / action / status 已有 flag，不需要查询语言，也不要 `label:` 语法。
5. **排序**：`list` 按 id，符合非交互共识；相关度留给 dsh-kiln。
6. **limit**：现在不截断；若加 `--limit`，必须打印"只显示前 N / 共 M"。
7. **退出码**：`list --match` 空结果继承 `list` 的 exit 0；独立 `search` 才考虑 grep 三态。
8. **输出**：`--json` 复用 `listDoc`（命中动作时加 `match` 字段）；文本一行一命中；颜色仅在 TTY（kiln 现在无颜色）。

### 9. 留待 hypothesize / conclude 的问题

- **形态**：`--match` vs `search`（盘点给了 gh 的双形态先例，但没替 kiln 选）。
- **默认**：字面量 AND 已定调；要不要 `--regex` 开关、引号短语怎么切。
- **字段收窄**（`--match=title|action`）一开始要不要。
- **`--limit` 与截断提示、空结果退出码**。

### 来源

- GitHub CLI：`gh search issues`、`gh issue list` —— [gh_search_issues](https://cli.github.com/manual/gh_search_issues)、[gh_issue_list](https://cli.github.com/manual/gh_issue_list)
- ripgrep：[GUIDE.md](https://github.com/BurntSushi/ripgrep/blob/master/GUIDE.md)、[rg(1)](https://man.archlinux.org/man/rg.1.en)
- fd：[fd(1)](https://man.archlinux.org/man/fd.1.en)（smart-case 默认）
- fzf：[fzf(1)](https://manpages.ubuntu.com/manpages/jammy/man1/fzf.1.html)（extended search）
- git：[git-log(1)](https://git-scm.com/docs/git-log)、[git-grep(1)](https://git-scm.com/docs/git-grep)
- git-bug：[git-bug-ls(1)](https://man.freebsd.org/cgi/man.cgi?query=git-bug-ls&sektion=1)
- kubectl：[Labels and Selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
- docker：[docker ps](https://docs.docker.com/engine/reference/commandline/ps/)
- Homebrew：[brew(1) Manpage](https://docs.brew.sh/Manpage)（substring；`/re/` 才按正则）
- npm：[npm-search](https://docs.npmjs.com/cli/v10/commands/npm-search)（`--searchlimit` 默认 20）
- GNU grep：[grep(1)](https://www.gnu.org/software/grep/manual/grep.html)

## hypothesize（2026-10-27，二版）：`list --search`，字面量 AND；不做独立 `search`

> 一版的 `--match` 是暂名。collect 发现 **gh 已经把这组词用掉了**：查询串是 `--search`，`--match` 是"限定搜哪个字段"。kiln 跟这套词——查询叫 `--search`，`--match` 留给将来的字段收窄。

### 1. 主张

| # | 主张 | 证伪条件 |
| --- | --- | --- |
| **H6** | 形态：加在 `list` 上的 `--search <query>`，**不新增独立 `search` 命令** | 出现需要独立输出/排序的硬需求（best-match 排序、分页、跨仓搜索） |
| **H7** | 默认：空格切词、**词间 AND**、**字面量子串**、**大小写不敏感**；`--regex` 本轮不做 | 真实查询大量依赖正则或模糊（"记得形状记不得字"），字面量召回明显不够 |
| **H8** | 检索面 = **id + 当前标题 + 每个动作的 subject**；命中动作时行带命中来源（候选字段名 `hit`） | 字段收窄成为常态（只想搜标题），或"命中动作却无法指向单条"的比例 >30% |
| **H9** | 非交互：**id 序、无相关度、无默认 limit、空结果 exit 0** | 结果集常态 >50 条（需要 limit + 截断提示）；或脚本明确依赖 grep 的"无命中 exit 1" |
| **H10** | 交互（逐键、模糊、高亮、相关度）**留给 dsh-kiln**；kiln 不做 TUI | 出现"CLI 里也必须即按即搜"的真实场景 |

### 2. 依据（来自 collect）

- **形态**：gh 同时有 `gh search issues` 与 `gh issue list --search`，说明两种都正当；kiln 只有一种行（`listDoc`），`--search` 是"复用列表输出"的那半，先选它不需要新 schema。
- **命名**：gh 的查询串叫 `--search`、字段收窄叫 `--match`（`{title|body|comments}`）。kiln 现在只需要"查询串"，所以 `--search`；`--match` 留着，等真有"只搜标题"的需求再用，两个词不抢。
- **默认语义**：issue/记录类工具的共识是**关键词 + 多词 AND + 不区分大小写**（gh / GitHub / git-bug）；默认正则只是 grep 家族的传统。kiln 的用户"记得一个词"，不是"记得一个正则"。
- **字段收窄**：gh 的 `--match`、git-bug 的 `--title` 都说明"默认宽、收窄可选"是对的；我们默认就落在最窄的语料上（id + 标题 + 动作 subject），连收窄都不急。
- **退出码**：kubectl / docker 这类"列表过滤器"空结果 exit 0；grep 三态属于"搜索动词"。`list` 是前者，别混。
- **limit**：gh 默认 30 是服务端分页的产物，且只写"最近 30"、不提示还有多少；kiln 的语料是本地全量，**不设默认 limit** 反而更诚实。
- **交互**：fzf / skim 证明交互层是独立工具；dsh-kiln 的搜索框已经是这一层。

### 3. 若成立，契约（落地按这个做）

- **命令**：`kiln list [--search <query>] [--domain=] [--tag=] [--action=] [--status=] [--json]`；`--search` 只收一个串，**空串或纯空白 → exit 2**（不静默当没给）。
- **匹配**：`strings.ToLower` 后逐词 `Contains`；词按空白切分；**所有词都要在 `id + 当前标题 + 所有动作 subject` 的并集里出现**（可分散在不同字段），不要求同一字段。
- **命中来源**：若某个动作的 subject **包含全部词**，行带 `hit`（该 subject；多条符合取最早的一条）；若词分散在多个动作、没有单条全含，则 `hit` 省略（行仍保留）——宁可少解释，也不错指。
- **输出**：JSON 复用 `listDoc`——`filter.search` 回显查询串、`issues[].hit` 仅在命中动作时出现；文本行给 **id + title**（命中动作时行尾 `← <subject>`），`--search` 之外的 `list` 文本维持现状。
- **排序 / 截断 / 退出码**：id 序（`engine.Filter` 的输出顺序），无相关度；不设默认 limit；0 = 正常（含无命中），2 = 用法错误。
- **不做**：`--regex`、模糊匹配、查询语言（`label:` 之类）、分页、TUI、颜色。

### 4. 与一版假设的关系

- 一版 H2 的**形态结论不变**（`list` 的过滤条件，不是新命令），**只把暂名 `--match` 改成 `--search`**；
- 一版 H3（检索面 = id + 标题 + 动作标题）不变，此处把"动作标题"精确为 **subject**（`path` 文本里看到的那一行）；
- 一版 H4（`list` 行补 `title`）已落地；`hit` 是它的补充：title 回答"是什么事"，hit 回答"为什么这条在结果里"；
- 一版 H5（交互留插件）不变；"不搜正文"仍成立（collect 已核实语料只有标题）。

### 5. 检验办法

1. **命名**：拿 gh 用户的直觉对照——`--search` 放查询、`--match` 留字段，两者不抢词；如果试用时有人把 `--search` 当字段选择，说明这套词没对齐，再改。
2. **召回对拍**：12 条查询（id / 当前标题 / 动作 subject / 只在历史正文里的词各 3 条）→ 与 dsh-kiln 的 `searchIssues` 比 id 集；唯一允许的差异来自 subject vs title（`f8413d8` 这种只有 subject 的动作）。
3. **体量与成本**：`worktree` 10 条、`json` 12 条、`关系` 11 条、`搜索` 1 条、`i18n` 1 条 → 1–12 行、约 0.2–2 KB；一次 `list --search` 与 `list` 同量级（±20 ms）。
4. **可解释性**：统计 `hit` 省略比例；>30% 就要么只搜标题、要么允许多行命中来源，别让结果"看起来没理由"。
5. **空结果**：先按 exit 0 落；若真实脚本依赖"无命中 exit 1"，再讨论是不是该升级成独立 `search`（那时 grep 三态才适用）。

### 6. 反方（保留）

- **独立 `search` 的输出空间更大**：可以做相关度、分页、`--json` 字段选择；`list --search` 被 `listDoc` 绑死。若 agent 需要"最好的前 5 条"，现在的假设会挡路——但那也该先有需求。
- **`--search` 与 `--status` 的组合语义**：`list --search=foo --status=open` 是 AND，够用；但有人会期望"搜索时忽略状态"——文档要写清是 AND，不搞隐式放宽。
- **字面量的上限**：拼写记错、词序记错就搜不到；fzf 的模糊与 smart-case 是更好的手感，但那属于交互层。若 CLI 侧频繁抱怨"搜不到"，H7 会被推翻。

## test（2026-10-27）：`list --search` 落地与验证

按二版假设实现，动作记 `test`（还不是 apply）。

### 实现

- `internal/cli/cli.go`：
  - `list` 的 `allowedFlags` 加 `search`；`--search` 收一个串，**空串 / 纯空白 exit 2**（`--search 需要一个非空查询串`）。
  - `filterSearch` / `issueMatches`：`strings.Fields` 切词，每个词都要在 **id + `issueTitle(p)` + 所有动作 `subject`** 的并集里出现（`strings.ToLower` 后 `Contains`），字段间用 `\x00` 隔开防止跨字段误配。
  - `searchHit`：**仅当 id + 标题解释不了命中时**，取最早一条"包含全部词"的动作 `subject` 作为 `hit`；词分散在多条动作、没有单条全含 → 省略。这比假设原文更克制（原文说"某条动作含全部词就给 hit"），避免标题已命中还挂一条箭头。
  - 输出：JSON 行加 `hit`（`omitempty`），`filter.search` 回显原串；文本视图在 `--search` 时改为 `id + state + title`（命中动作时行尾 `  ← <subject>`），其余 `list` 文本不动。
  - 帮助：list 行加 `[--search=]`，`usageOtherFlags` 加 `list 的 --search` 一行；中英同步。
- `internal/i18n/catalog.go`：新错误串与帮助行的英文。
- 文档：`machine-readable-output.md` §3 的 filter 字段、`hit` 字段、示例与要点；README 双语命令表。

### 测试（`internal/cli/cli_test.go`）

- `TestListSearchFindsIdTitleAndActionSubjects`：id（`cache`）、标题（`淘汰`）、id+标题分词（`cache 淘汰`）→ 无 `hit`；早期动作（`LFU`）→ `hit`；多词同条动作且大小写不敏感（`lru 命中率`）→ `hit`；
- `TestListSearchLeavesHitEmptyWhenTermsSplitAcrossActions`：`LRU 淘汰` 分散在两条动作 → 命中但 `hit` 为空；
- `TestListSearchNoMatchIsNotAnError`：`zzz` → `count: 0` / `issues: []` / exit 0，文本零输出；
- `TestListSearchRejectsAnEmptyQuery`：`--search "   "` 与 `--search`（无值）→ exit 2、stdout 空、stderr 指"非空"；
- `TestListSearchComposesWithStatus`：`--search=淘汰 --status=done` = 1，`--status=open` = 0；
- `TestListSearchTextShowsTitleAndHit`：文本含 id、标题与 `← <subject>`。

`gofmt` / `go vet` / `go test ./...` 全过。

### 真仓冒烟（60 issue / 262 动作）

| 查询 | 命中 | 其中带 `hit` | 体量 |
| --- | --- | --- | --- |
| `worktree` | 10 | 2 | 2.9 KB |
| `json` | 13 | 3 | 4.1 KB |
| `搜索` | 1 | 0（标题已解释） | 0.4 KB |
| `i18n` | 1 | 0 | 0.4 KB |
| `zzz` | 0 | 0 | 72 B，exit 0 |

`--search=worktree --status=open` = 0（命中的都已收尾），AND 语义符合预期；文本行形如 `help-relation-flags  apply  <标题>  ← <命中的动作>`。

### 与假设的差异 / 仍未做

- 唯一的实现取舍：**`hit` 只在 id + 标题解释不了时出现**（见上）。
- 未做：`--regex`、模糊、查询语言、`--match` 字段收窄、`--limit`、独立 `search`。
- 下一步是 `apply`（把这套契约定性）还是先观察使用手感，待定。

## test（2026-10-27，续）：0 命中提示与 `--json` 复核

- **0 命中**：文本与 `--json` **都**在 **stderr** 打一行 `没有匹配 "<query>" 的 issue`，exit 0；JSON 的 stdout 仍只有一个文档（`count: 0` / `issues: []`）。之前是纯静默，终端里像"命令没执行"。
- **`--json` 字段复核**：`filter.search` 回显查询串、`issues[].hit` 只在 id + 标题解释不了命中时出现——这两项在 `cb5730a` 已随实现落地；本轮只补测试把契约钉住，没有新增字段。
- 测试：`TestListSearchNoMatchIsNotAnError` 扩成"两种模式都断言 stderr 提示"；其余 6 条不变。
- 文档：`docs/machine-readable-output.md` 的 `--search` 要点补 0 命中约定；README 双语机器可读示例加 `kiln list --json --search=worktree`。
- 复核：`gofmt` / `go vet` / `go test ./...` 全过；真仓 `list --search=zzz` → stderr 提示、stdout 空、exit 0；`list --search=zzz --json` → stdout 一份文档 + 同一行 stderr 提示。
