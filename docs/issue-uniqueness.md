# issue: issue-uniqueness — 去掉 namespace 后 issue ID 的唯一性怎么保证

- **domain**：design
- **关联**：derived-from `status-by-domain`

## 背景

[README](../README.md) 的提交格式把 `issue:` 定义成**单段 kebab-case slug**，[mvp-plan](mvp-plan.md) §2.4 补齐了约束（小写、只允许字母数字与 `-`、≤ 50、**不允许 `/`**）并留了沿革：早期用过 `<namespace>/<slug>` 降低重名概率，后因"单仓库单人使用时 namespace 恒等于项目名、零信息量"而**去掉**。

于是唯一性只剩一句话：**靠命名约定 + `kiln check` 检测 + 人工确认**，范围是**单个仓库**（mvp-plan §2.4）。§10 把它记成待办：*"issue ID 唯一性：去掉 namespace 后靠什么避免重名——命名约定 + `kiln check` 检测是否够用，是否需要跨仓库引用前缀"*。

邻居 `qualified-issue-id` 已定稿：**不引入限定名输入**（`issue:` 保持单段 slug），歧义在 `status` 的显示侧消除。→ 本条因此**只剩唯一性这一件事**，不再与显示纠缠。

## 要回答的问题

1. **约定 + `check` + 人工确认够不够**？"够"的判据是什么（什么算"重名"、什么时候必须拒绝）？
2. 需不需要**跨仓库引用前缀**？它解决的是"本仓重名"还是"引用别处的 issue"？
3. `check` 该查什么：归一化后相等？同 slug 多个 `ask`？`title` 相似？
4. 拒绝的**时机**：`check` 是事后读，要不要变成 hook / CI 里的拒绝？
5. 要不要一把"不随改名变"的键（哈希 / 序号），还是继续只有 `issue:` 这一个值？
6. `kiln rename`（mvp-plan §10 另一条）在无数据库的前提下，旧名怎么处理？

## hypothesize：写侧归一化 + 起点动作重名拒绝（个人使用：同名即同一条）

> 用户裁定：**个人使用场景下，同名 issue 视为同一条**。所以只做两件事：① 新建 issue 时**归一化命名并提示**；② 新建 issue **重名则拒绝**（不开命名空间、不换派生键）。

### 1. 为什么这两件就够

- 唯一性在业界只能靠「派生」（哈希/序号）或「约定 + 拒绝」两路上分（见 collect §7）。要“同名即同一条”，**拒绝**就是那个零成本的强制点（Gerrit 式：在写侧就报错，而不是留给事后读的 `check`）。
- 个人仓里没有「两个团队各建一个 `session-expiry`」这种事；真要撞，合并来的历史里自然会有第二个 `ask`——拦的是**写侧**，不是历史。

### 2. 三个必须钉死的细节（否则会自己造坑）

**2.1 归一化必须落在写侧单点（`do` / `commit` / `worktree new`）**

只给 `ask` 归一化是不够的：`ask Foo-Bar` 写入 `Foo-Bar`、`collect foo-bar` 另建一条——**静默分裂成两条 issue**，正是本条要治的病。归一化要发生在**写 trailer 值的那一处**（`env.write`），worktree 的目录/分支名也要跟同一份归一化走，否则“区名对不上 issue 名”，`status` 的 `■` 会挂错行。

**2.2 拒绝条件收窄为“该 issue 已有起点动作”，而不是“ID 已存在”**

否则会把**修复路径**堵死：若有人先 `collect foo`（`check` 会报“缺少起点动作”），之后他再也无法补 `ask foo` 了——那是修复而不是新建。→ 判据：`repo.Issues[issue]` 存在**且**它已经有 `e.actions.Start()` 这个动作。

**2.3 提示进 stderr、不改 exit code；拒绝用 exit 1**

沿用 [worktree-hint](worktree-hint.md) 已定的规矩（“提示**只进 stderr**、exit code 不变”）。拒绝是状态冲突（不是用法错误），用 `1`；用法错误才用 `2`。

### 3. 归一化规则（只治“有无歧义等价写法”）

| 输入 | 归一化为 | 理由 |
| --- | --- | --- |
| `Linux-Kernel` | `linux-kernel` | 大小写折算（PyPI/npm 同款第一步） |
| `foo_bar` | `foo-bar` | `_` 与 `-` 等价（crates.io RFC 940「Hyphens Considered Harmful」同款） |
| `foo--bar` / `-foo-` | `foo-bar` / `foo` | 连续与首尾 `-` 折叠（PyPI 的 `[-_.]+` → `-` 同款） |
| `foo bar` / `foo.zh` | **不动，仅警告** | 没有唯一等价物——**不猜**（猜错了比报错贵） |

### 4. 非目标

- **读侧不宽容**：`kiln path Foo--Bar` 仍报「未找到问题」——写侧归一化后这种仓不会再新增，而读侧宽容要先回答“仓里同时有 `foo--bar` 与 `foo-bar` 时用哪个”（老数据才可能，存量无价值）。
- **不加 `--force`**：个人使用下没有“我就是要两条同名 issue”的正当场景；真要分叉，合入的历史本来也不会被拦——**拒绝是写侧护栏，不是不变量**。
- 不换派生键（哈希/序号）——那会让人**不再能读名字**，与“slug 就是人读的名字”直接冲突。
- 不做跨仓库引用前缀（那解决的是“引用别处”，不是“本仓重名”，见 collect §7）。
- `kiln rename` 仍留待 mvp-plan §10（本方向只保证**不需要为显示名改名**）。

## collect：业内怎么保证「扁平人类可读 ID」的唯一性

> 一轮联网调研：初稿由一个 **pi 子代理**（`pi -p`，独立 session `kiln-issue-uniqueness-collect`）检索，随后由本仓**逐条复核**（凡本节能引"官方原文"的都是复核过的；标 **[二手]** 的为非官方）。来源见节末。

### 1. 五种机制：谁在保证唯一

| 机制 | 代表 | 做法 | 代价 |
| --- | --- | --- | --- |
| **命名约定** | kiln 现状、内核 trailer、各种 slug 约定 | 强制字符集 + 长度 + 人工确认 | 只有"事后发现"；无强制力 |
| **中心注册表分配** | PyPI / npm / crates.io / Jira project key / Docker namespace | 注册时占坑，名字带所有权 | 需要中心、需要服务；抢注成为问题 |
| **平台强约束** | Gerrit `receive.requireChangeId`、PyPI 名字格式 | push/发布时**拒绝**不符合者 | 只有"格式"能约束，"语义重复"不能 |
| **内容哈希派生** | git OID、git-bug、Fossil ticket、Gerrit `Change-Id` | 从内容算出 ID，**无需协调**即唯一 | 不可读；只能当主键，不能当名字 |
| **全局递增序号** | PEP、RFC、Debian BTS、Jira/Linear 的 `-123` | 单序列只增不改 | 需要分配者；跨仓库就撞 |

一句话：**人读的名字很难天然唯一，业界的做法是"把唯一性交给机器键，把人类名字降级为标签"。**

### 2. 没有注册表时：用"派生"换"协调"

- **Gerrit `Change-Id`**：官方明确**不承诺**实例内唯一——"a Change-Id is **not necessarily unique** within a Gerrit instance. It can be reused among different repositories or branches"；用户指南则把实际范围说成"unique for a branch of a project"。它由 `commit-msg` hook 生成进 commit footer，而**缺失会被拒绝**（`receive.requireChangeId`，官方报错原文 `missing Change-Id in commit message footer`）。→ **唯一性范围被显式声明得比"名字看起来的范围"更小**，冲突留给 (project, branch) 这一层去兜。
- **git-bug**（同样是"把 tracker 放进 git"）：ID = **SHA-256 的 64 位 hex**（"Every ID is the SHA-256 of bytes stored in git"），人只用 **7 字符前缀**寻址；**前缀撞上多个 ID 是错误**——"A prefix resolves if it matches exactly one known ID; **a prefix matching several IDs is an error**"。操作 ID 的约束也只在局部声明："Operation IDs **must be unique within an entity**"。
- **beads**（`gastownhall/beads`，agent 向的 git 原生 tracker）README 原文：**"Zero Conflict: Hash-based IDs (`bd-a1b2`) prevent merge collisions in multi-agent/multi-branch workflows"** → 用哈希**逃掉**冲突，而不是检测冲突。
- **Fossil**：ticket id 是随机十六进制（官方文档自陈"look like SHA1, but are not the hash of any artifact"）——同样是"指派一个机器键，人读的是标题"。

→ 规律：**无注册表时，唯一性靠"派生"（哈希/内容）而不是"约定"；人类可读的名字从来不是主键。**

### 3. 有注册表时：防的是**近似**重名，靠**归一化 + 相似度审查**

| 平台 | 具体规则（官方） |
| --- | --- |
| **PyPI** | 名字规范化：`re.sub(r"[-_.]+", "-", name).lower()`——**大小写折叠 + `.`/`-`/`_` 连续串折成一个 `-`**；规范化后才允许比较与查找 |
| **npm** | **去标点后比较**："we remove punctuation from its name and compare it to existing package names. If the names are identical without punctuation, we do not allow the package to be created"（并建议改用 **scope**）；新包**不得含大写**；scope 的意义正是"you don't have to worry about someone taking your package name ahead of you"；名字规范另外要求"不要与已有包拼写相似" |
| **crates.io** | RFC 3463 政策：**crate 不可删除**（"Crate deletion by their owners is not possible to keep the registry as immutable as possible"）、**squatting 会被删**；更早的 RFC 940「Hyphens Considered Harmful」统一了 `-` 与 `_` |
| **Go module path** | 路径首段必须是**带点的域名**（`example.com/foo`）→ 命名空间 = **所有权证明**；无点首段不可下载 |
| **Maven** | `groupId`（反写域名）+ `artifactId`，两级，同样把命名空间绑到域名所有权 |
| **Docker** | `namespace/repository`；省略 namespace 时默认 **`library`**（"the namespace reserved for Docker Official Images"）→ 短名字被官方独占 |
| **DNS** | LDH label（RFC 5890）+ 大小写不敏感 + 层级委派；同形字问题单独治理 |

→ 两条规律：① **归一化**是防重名的第一手段（人可以有多种写法，机器只认归一后的一个）；② **命名空间在本行业几乎都绑定"所有权"**（域名/账号/组织），不是随便一层分类。

### 4. 改名/迁移：旧名必须被**某处**记住

| 平台 | 旧名的命运 |
| --- | --- |
| **GitHub** 仓库/用户名 rename | **自动重定向**（2013 官方公告）；但**旧名立刻可被他人占用**——"the new owner of your old username creates a repository with the same name … that will override the redirect entry and **your redirect will stop working**"（官方用户名变更文档） |
| **crates.io** | 名字**不可复用**（政策），只能转让所有权或等 `available_at` 冷却 |
| **PyPI** | PEP 541 允许把废弃/被抢注的名字**重新分配**给正当使用者 |
| **Jira** | 改 key 后旧 key 的链接/JQL/REST **仍可用**；旧 key 不能再给新项目；可改回；删项目后 key 被释放可复用 |
| **YouTrack** | 旧 ID 存进 **Historical IDs**，外部链接**重定向**到新 ID |
| **Linear** | 移动 team → **新 identifier**；旧 identifier 仍可搜索、URL 重定向，但正文里手写的旧引用**不会被改写** |

→ 规律：**"旧名→新名"的映射必须被某处记住**（重定向表 / historical IDs / historical name 字段）。没有这层，改名就等于断链。

### 5. 反面教材：扁平名字跨出容器之后的真实事故

- **dependency confusion**（Birsan 2021）：内部包名在**公共**注册表被抢注，解析方无法辨别真假 → 35+ 公司受影响、$130k 赏金。
- **npm / PyPI typosquatting**：npm 的 moniker rules 就是一起 typosquatting 事件后的应对。
- **Docker Hub 短名混淆**：省略 namespace 落到 `library`，用户以为拉的是官方镜像——**名字少了那一段，"谁发布的"就没人能看出来**（规则出自官方 image tag 文档）。
- **IDN 同形字**：ICANN 关于 IDN homograph 的专门说明。

→ 规律：**扁平名字一旦跨出"你自己的容器"，它同时是"身份"和"寻址"**——抢注与混淆就从"重名"升级成"安全问题"。kiln 目前只在**一个仓库内**，尚未走到这一步；**一旦支持跨仓库引用，威胁模型就是这个形状**。

### 6. 相似/重复检测：只做提示，不做拒绝

- GitHub 2026-06 上线 **Detecting Duplicate Issues**（public preview），官方原话："Duplicate issues are one of the biggest time sinks for maintainers: triaging the same bug filed multiple ways, closing duplicates, and linking back to the original." → 机制是**提交时给候选**，不是拒绝。
- Bugzilla 有 _possible duplicates_ 关键词搜索接口（官方 API）。
- 相似度匹配的假阳性代价是经典问题（VLDB 2002 的记录匹配综述 [二手]）。

→ 规律：**"相似"永远只做建议；拒绝只留给"归一化后完全相等"。**

### 7. 对 kiln 的映射（事实 + 选项，裁定留给下一轮）

1. **唯一性只能靠"派生"或"约定"**：kiln 的 `issue:` 是**唯一一把键**（没有 GitLab `id`/`iid` 那种"机器键 + 人读键"的双键结构）。要"派生唯一"就必须改 `issue:` 的值形态（哈希/序号）——这是唯一的硬开关，代价是**人不再能读它**。
2. **归一化是零成本那一档**：kiln 已经强制小写 + 只允许 `[a-z0-9-]`，等于 PyPI/npm 归一化的前半段；还剩的是**连续 `-`、首尾 `-`（以及可选的 `_` 折叠）**——`check` 能查，成本几乎为零。
3. **"约定 + check + 人工确认"在无注册表场景是行业常态**，差别在**拒绝的时机**：Gerrit 在 push 时就拒绝，git-bug/beads 靠哈希从根上避免，kiln 的 `check` 是**事后读**。要不要把 `check` 接到 hook/CI（把"提示"变成"拒绝"）是本条要定的。
4. **"前缀"要分清是哪一种**：npm scope / Go module 域名 = **所有权命名空间**（为跨仓库而存在）；Jira/Linear 的 `KEY-123` = **序列号命名空间**（同一实例内避免撞号）。→ 在单仓库前提下，**跨仓库前缀解决的是"引用别处的 issue"，不是"减少本仓重名"**。
5. **`kiln rename` 唯一负担得起的做法**：GitHub 式重定向表需要数据库（kiln 不可得）；**beads 那样把旧 id 留在数据里（`external_ref`）**是可在提交信息内实现的等价物——即"改名时把旧名写进新提交的 trailer"。
6. **`check` 可查什么**：① 归一化后相同的两个 ID；② 同一 ID 下出现第二个 `ask`（mvp-plan §10 已有该条）；③ `title` 高度相似（只告警）。前两条是"确定性冲突"，第三条按行业惯例**只提示**。

> 来源（官方文档为骨干，其余标注类型）：[Gerrit：Change-Ids](https://gerrit-review.googlesource.com/Documentation/user-changeid.html)（官方文档：Change-Id **不保证**实例内唯一，可在 repository/branch 间复用）｜ [Gerrit：Project Configuration（`receive.requireChangeId`）](https://gerrit-review.googlesource.com/Documentation/config-project-config.html)（官方文档：缺失即拒绝，报错 `missing Change-Id in commit message footer`）｜ [Gerrit User Guide](https://gerrit.cloudera.org/Documentation/intro-user.html)（官方文档：unique for a branch of a project）｜ [git-bug spec：identifiers](https://github.com/git-bug/spec/blob/master/ids.md)（官方规范：ID = SHA-256；7 字符前缀；"a prefix matching several IDs is an error"；"unique within an entity"）｜ [beads（gastownhall/beads）](https://github.com/gastownhall/beads)（开源项目 README："Hash-based IDs (`bd-a1b2`) prevent merge collisions"）｜ [Fossil：Bug Reports and Feature Requests](https://fossil-scm.org/home/doc/trunk/www/bugtheory.wiki)（官方文档：ticket id 是随机十六进制）｜ [PyPI：Names and normalization](https://packaging.python.org/en/latest/specifications/name-normalization)（官方规范：`[-_.]+` → `-` + 小写）｜ [npm：New Package Moniker rules](https://blog.npmjs.org/post/168978377570/new-package-moniker-rules.html)（官方博客：去标点后比较，相同即拒绝，建议用 scope）｜ [npm：package.json（name 规则）](https://docs.npmjs.com/cli/configuring-npm/package-json)（官方文档：新包不得含大写）｜ [npm：Package name guidelines](https://docs.npmjs.com/package-name-guidelines)（官方文档：不要与已有包拼写相似）｜ [npm：Scoped packages](https://docs.npmjs.com/cli/using-npm/scope)（官方文档：scope 内名字不会被抢）｜ [crates.io：RFC 3463 policy update](https://rust-lang.github.io/rfcs/3463-crates-io-policy-update.html)（官方政策：不可删除、squatting 会被删）｜ [Rust RFC 940：Hyphens Considered Harmful](https://rust-lang.github.io/rfcs/0940-hyphens-considered-harmful.html)（官方 RFC：统一 `-` 与 `_`）｜ [Go Modules Reference](https://go.dev/ref/mod)（官方文档：module path 首段为域名）｜ [Maven：Guide to naming conventions](https://maven.apache.org/guides/mini/guide-naming-conventions.html)（官方文档：groupId 反写域名）｜ [Docker：image tag 参考](https://docs.docker.com/reference/cli/docker/image/tag)（官方文档：省略 namespace 默认 `library`，官方镜像保留）｜ [RFC 5890（IDNA 定义 / LDH label）](https://www.rfc-editor.org/rfc/rfc5890)（标准）；[ICANN：IDN homograph 说明](https://www.icann.org/en/blogs/details/idns-and-homograph-attacks-25-2-2018-en)（官方机构，同形字风险）｜ [GitHub：Repository redirects are here!](https://github.blog/news-insights/product-news/repository-redirects-are-here)（官方博客：自动重定向旧位置）｜ [GitHub Docs：Username changes](https://docs.github.com/en/account-and-profile/concepts/username-changes)（官方文档：旧名可被他人占用，会覆盖重定向）｜ [GitHub Docs：Renaming an organization](https://docs.github.com/en/organizations/managing-organization-settings/renaming-an-organization)（官方文档，同上语义）｜ [GitHub Changelog：Detecting Duplicate Issues（2026-06-18，public preview）](https://github.blog/changelog/2026-06-18-duplicate-detection-and-issue-fields-mcp-support-for-github-issues)（官方公告：重复 issue 是维护者最大时间黑洞之一，机制是给候选）｜ [Bugzilla：Bugs API（`possible_duplicates`）](https://bugzilla.readthedocs.io/en/latest/api/core/v1/bug.html)（官方文档）｜ [PyPI：PEP 541 名字重新分配](https://peps.python.org/pep-0541)（官方 PEP）｜ [Atlassian：Editing a project key](https://confluence.atlassian.com/spaces/ADMINJIRASERVER/pages/938847080/Editing+a+project+key)（官方文档：旧 key 仍可用、不可给新项目、可改回、删项目后释放）｜ [YouTrack：Configure a Project](https://www.jetbrains.com/help/youtrack/server/configuring-a-project.html)（官方文档：Historical IDs + 重定向）｜ [Linear Docs：Issues](https://linear.app/docs/creating-issues)（官方文档：旧 identifier 可搜索，正文引用不改写）｜ [Birsan：Dependency Confusion（2021）](https://medium.com/@alex.birsan/dependency-confusion-4a5d60fec610)（一手披露：35+ 公司、$130k）｜ [VLDB 2002：记录匹配综述](https://www.vldb.org/conf/2002/S24P01.pdf)（学术 [二手]：相似匹配的假阳性代价）
