# issue: relation-revision — 关系写错了怎么改：改历史、追加撤销，还是把关系移出动作历史

- **domain**：design
- **关联**：derived-from `relation-types`、relates-to `evidence-fluidity`

## 背景

`kiln` 的关系是**跨该 issue 全部动作 commit 的并集**（`internal/engine/engine.go:71-75`）：

```go
p.DerivedFrom = appendUnique(p.DerivedFrom, a.DerivedFrom...)
p.RelatesTo   = appendUnique(p.RelatesTo, a.RelatesTo...)
p.Blocks      = appendUnique(p.Blocks, a.Blocks...)
```

→ **只增不减**。一条关系一旦写进任何一次动作，就永久留在 `path` 的关系摘要里；`relation-types`
的非目标也写明「不重写已有提交：现有的 derived-from / relates-to 用法继续有效」，README 的取向是
「不改写历史——动作就是提交，撤销 / 回滚 / 跨分支复用全用 git 自己的本事」（README:284）。

真实代价（wsl-agent 的 issue 梳理，2026-10-17）：先用 `relates-to` 记了两条弱关联，随后发现真正的
语义是 `derived-from` + `blocks`；因为不能删，**只能追加精确的边，让冗余的 `relates-to` 永久留着**。
同一次还撞上两个相关缺口：

1. **反向推导没有实现**。`relation-types` 的结论是「声明一侧、读侧自动推导反向」，但代码里没有：
   `relationLines`（`engine.go:302-313`）只画本 issue 自己存的那三族。→ 要让两侧视图都完整，就得
   **写两遍**（本次确实每条边都写了两次：`A spawns B` + `B derived-from A`）；日后要修正也是两遍。
2. **写错目标无人管**。`check` 只校验 `derived-from` 的父存在、`spawns` 的子存在；`blocks` /
   `relates-to` / `supersedes` 指向不存在的 issue 也不报。

## 要回答的问题

1. **取向先定**：关系是**证据**（不可改，与 `evidence-fluidity` 同题），还是**当前事实的描述**
   （可改，且改它不该碰动作内容）？
   > 我的判断是后者：关系不记录"某一步发生了什么"，只描述"现在这两条 issue 是什么关系"。
   > 这也正是它与 `evidence-fluidity` 的**边界**：动作内容（diff、标题、证据）可以不可改，
   > 关系却应该可改——两者不该被同一条约定绑在一起。
2. **若不改历史，机制是什么**：
   - **追加型撤销**：新增 trailer（如 `unrelates-to:` / `revises-relation:`），读侧按最新动作覆盖；
   - **关系清单**：把关系移出动作历史，改成 issue 级、可编辑的元数据（文件或 `kiln relate` 命令），
     动作 commit 只负责内容；
   - **同键整组后写覆盖**（last-wins）：语义变化大——`path` 里"每一步看到的关系"会不同，而
     `path` 现在呈现的是并集。
3. **反向推导要不要实现**：实现后声明一侧就够、修正也只改一侧（`relation-types` 已定"读侧推导"，
   但**没落地**）。不实现就得双侧维护，且两侧可能不一致（现在就没有任何一致性校验）。
4. **历史改写允不允许**：README 说"用 git 自己的本事"——那就意味着"关系写错 → rebase/amend"是
   官方答案。但这与"关系是长期资产"的期望冲突吗？jj 用户（本仓就是 jj）改历史是日常操作，
   git 用户则危险得多。**要不要对两侧给不同建议？**
5. **检出**：关系若可撤销 / 修订，`path` 要不要显示"这条边后来被撤销过"（以及谁撤的）？
6. **与 `check` 的互动**：关系可改之后，"目标存在"校验要不要补齐到 5 个键（现在只查两族）？

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只立约定 | 关系写错就改历史（`jj describe` / `git rebase`） | 零机制；对 jj 是日常，对 git 危险；且违背"不重写"的默认叙事 |
| **B** 追加型撤销 | `unrelates-to:` / `revises-relation:`，读侧覆盖 | 字段增殖；"撤销的撤销"要定义 |
| **C** 关系移出动作历史 | issue 级关系清单（文件或 `kiln relate`），可改 | 最大改动：关系不再随动作留痕，与 trailer 模型分道 |
| **D** last-wins | 同键后写的动作整组覆盖 | 语义变化大；与 `path` 的"每步并集"呈现冲突 |
| **E** 维持 | 现状：并集、只增不减 | 零成本；冗余边永久留存（已在真实项目里发生） |

> 倾向（待验证）：**先答 1 的取向，再在 B / C 里选**。B 最贴现有模型（trailer 驱动、可审计、
> 与 `action-diff` 同族），C 最干净但动"关系从哪来"的根；A 依赖 `evidence-fluidity` 的结论，
> 且与 README 的"不改写历史"叙事直接相关。

## 非目标

- 不讨论动作内容（diff / 标题 / 证据）能不能改——那是 `evidence-fluidity`。
- 不改关系词表与渲染（分别是 `relation-vocabulary` / `relation-rendering`）。
- 不在本 issue 里决定 `part-of` 的落地（`relation-types` 的遗留，见 `silent-flag-drop`）。

## 证据

- `internal/engine/engine.go:71-75`：`appendUnique` 的并集语义（只增不减）。
- `internal/engine/engine.go:302-313`：`relationLines` 只画本 issue 存的三族——**反向推导确实没实现**。
- `internal/engine/engine.go` `checkIssue`：只校验 derived-from / spawns 的目标存在。
- `docs/relation-types.md` 结论 1（"声明一侧、读侧推导反向"）与非目标（"不迁移历史"）。
- `README.md:284`：不改写历史，撤销 / 回滚交给 git / jj。
- wsl-agent 实例（2026-10-17）：`relates-to` 无法退场，只能追加 `derived-from` / `blocks`；
  每条边都要两侧各写一次（`A spawns B` + `B derived-from A`）。

## collect：同类工具怎么「改一条已经写下的关系」——六种机制与逐条出处

> 一轮联网调研（2026-10-18）：检索 git-trailer 驱动 / git 内嵌的追踪与协作工具，只看一件具体的事——**一条已经落进不可变历史的关系/字段，后来写错了，它们怎么办**。承重结论都核过一手文档；转述/二手已标注。

### 0. 筛选口径

「同类」＝把结构化元数据写进 **git 提交信息 / git 对象 / git ref**，并把 git 当同步介质的工具。中心化 DB 型（GitHub / Jira / Linear 的 issue-link 表）只作反例：它们的关系在服务端数据库里，本来就可改，**不构成这个问题**。

入选：git-bug、git-dit、git-appraise、Fossil tickets、git-issue、Radicle Heartwood COBs；外加两个「trailer 关系」的现实参照——Gerrit `Depends-On:` 与内核邮件流的 `Fixes:` / `PATCH vN`。**没有一个是「并集 + 只增不减」**，这点本身是本次调研最硬的结论。

### 1. 六种机制（先结论，后证据）

| 机制 | 一句话 | 代表实现 | 对应本 issue 候选 |
| --- | --- | --- | --- |
| **M1** 追加「删除/撤销」操作 | 不改历史，新增一条**显式删除 op**，读侧重放到当前态 | git-bug `LabelChangeOperation{Added, Removed}`；Fossil 字段 replace；CRDT redact / tombstone | **B** |
| **M2** 读侧 last-wins | 日志全留，**读数时后写覆盖前写** | Fossil 按时间重放 ticket change；git-appraise 取最后一条 request | **D** |
| **M3** 可变覆盖层（独立 ref） | 不可变提交 + 另一个**可改可删的 ref** 承载元数据 | git notes；`git interpret-trailers --if-exists=replace`；`.mailmap` | **B/C 之间** |
| **M4** 元数据移出历史 | 当前态是**工作树里的文件**（独立对象），历史交给 VCS | git-issue 的 `.issues/` 文件树；`.mailmap`；`git replace` | **C** |
| **M5** 重写历史 / 发新版本 | 直接 amend/rebase；或「旧版留着，发一版新的」 | Gerrit 新 patchset + abandon；邮件流 `PATCH v2` | **A** |
| **M6** 只增不删 | 不纠错，只加新事实，旧的永久留存 | 内核 `Fixes:` 可多条；GitHub cross-ref 累积 | **= kiln 现状（E）** |

#### M1 追加「删除/撤销」操作

**git-bug**（操作日志式）：实体不存最终态，存一串 edit `Operation`；当前态靠**按序重放**算出。操作类型里删除是**一等槽位**——标签变更就是 `LabelChangeOperation{Added []Label, Removed []Label}`，标题变更带 `SetTitleOperation{Title, Was}`（**旧值一起记**，可审计）。→ 字段的「改」＝追加一条新 op；字段的「删」＝追加一条带 `Removed` 的 op。「改一条关系」的答案 = **再加一条 op**，历史不动。

> 一手：[git-bug data-model.md](https://github.com/git-bug/git-bug/blob/trunk/doc/design/data-model.md)（Operation / OperationPack / Lamport 时钟 / 并发按 op-pack 哈希定序）｜ [pkg.go.dev: bug package](https://pkg.go.dev/github.com/MichaelMure/git-bug/entities/bug)（`LabelChangeOperation`、`SetTitleOperation.Was`、`EditCommentOperation`）。
> 注：git-bug 的操作表里**没有跨 issue 的「关系」类型**——它只解决了「同一实体的字段可改」，没解决 issue 间连边。

**Fossil tickets**（非 git，但机制最同构）：ticket 的当前态是把该 ticket 的**所有 change artifact 按时间重放**的结果；每个 key/value 对带一个标志，决定它是 **replace** 还是 **append** 前值。当前态落在 `TICKET` 表、历史落在 `TICKETCHNG` 表。→ 一条「关系字段」写错，就再发一个 change artifact 把它 replace 掉。

> 一手：[The Fossil Ticket System（artifact 717ff5a4）](https://www2.fossil-scm.org/index.html/artifact/717ff5a436834bf0)；[Fossil tickets.wiki](https://fossil-scm.org/home/doc/trunk/www/tickets.wiki)（TICKET 当前态 + TICKETCHNG 历史）。

#### M2 读侧 last-wins

**Fossil**：见上——追加 change，但**读侧按时间取最后值**，语义就是字段级 LWW。

**git-appraise**：评审数据不写进提交，而写进 **git notes**（`refs/notes/devtools/reviews`）。读 summary 时把该 commit 的 request notes **按时间排序、取最后一条**（`requests[len(requests)-1]`）；评论则按 notes 的日志顺序解析成线程。

> 一手：[git-appraise review/review.go](https://raw.githubusercontent.com/google/git-appraise/master/review/review.go)（`getSummaryFromNotes` 取最后一条 request）、[review/request/request.go](https://raw.githubusercontent.com/google/git-appraise/master/review/request/request.go)（`const Ref = "refs/notes/devtools/reviews"`）。

#### M3 可变覆盖层（独立 ref / 单条消息内替换）

- **git notes**：与提交分离的独立 ref，可 `add` / `append` / `edit` / `remove`；连「重写历史时 note 怎么搬」都有既定语义——`notes.rewriteMode` ∈ `overwrite` / `concatenate` / `cat_sort_uniq` / `ignore`，配 `GIT_NOTES_REWRITE_REF` 选哪些 ref 的 note 跟着搬。→ **git 原生就提供了一个「可改、可删、且知道怎么跨 rewrite 迁移」的元数据层**。
- **`.mailmap`**：仓库根一个可编辑的小文件，把 author/committer 映射到规范名/邮箱，**完全不碰历史**。
- **`git replace`**：创建 replace ref，让某个对象在**读取时**被另一个替换，不改写历史。
- **`git interpret-trailers --if-exists=<action>`**：控制**同键 trailer 已存在**时的行为（`replace` 即就地替换）；也可由 `trailer.<token>.ifexists` 配置。→ 但只作用于**一条**提交信息内部，不跨历史。

> 一手：[git-config（notes.rewriteMode / GIT_NOTES_REWRITE_REF）](https://git-scm.com/docs/git-config)｜ [gitmailmap](https://git-scm.com/docs/gitmailmap)｜ [git-replace](https://git-scm.com/docs/git-replace)｜ [git-interpret-trailers](https://git-scm.com/docs/git-interpret-trailers)。

#### M4 元数据移出历史

**git-issue（dspinellis）**：每个 issue 是 `.issues/`（一个**独立的 git 仓库**）里的**一个文件目录**；每次修改直接改文件并产生一次 git commit；tag / milestone / weight / duedate 都可用 `-r` 移除。→ 关系/元数据不是「动作历史里的 trailer」，而是**可编辑的工作树**，历史由这棵树的 commit 提供。

> 一手/二手：[dspinellis/git-issue README](https://github.com/dspinellis/git-issue)（`git issue edit` / `git issue tag -r` 等命令面）｜ [DeepWiki: Issue Storage](https://deepwiki.com/dspinellis/git-issue/3.1-issue-storage)、[Issue Lifecycle](https://deepwiki.com/dspinellis/git-issue/3.2-issue-lifecycle)（二手，存储目录与「每次修改一个 commit」）。

#### M5 重写历史 / 发新版本

- **Gerrit**：change 间依赖写成提交信息里的 `Depends-On: <Change-Id>` trailer，由服务端 / CI 解析成 related changes（限制：只支持单条、只用最新 patchset、依赖方必须 open）。→ 关系仍住在提交信息里，**改它 = 改提交信息 = 发新 patchset**；或 abandon 旧的、开新 change。
- **内核邮件流**：补丁改版就是重发 `[PATCH v2]`。而 LKML 的 `SubmittingPatches` 反过来还劝你**别把新旧版本用 In-Reply-To 串成一棵树**（「multiple versions of the patch don't become an unmanageable forest of references」），要链接旧版就在 cover letter 里放一个 `lkml.kernel.org` 的 URL。

> 一手（邮件流）：[LKML: [PATCH v2] Documentation/SubmittingPatches: discuss In-Reply-To](https://lkml.org/lkml/2015/11/5/651)（已合入 `d7ac8d85`）。
> Gerrit（二手/示例实现）：[gerrit.nordix 的 change-dependency 实现说明](https://gerrit.nordix.org/gitweb?p=infra/cicd.git;a=commit;h=750a20fde6dd15e1cd02fe176d2422ceb06929c8)（`Depends-On:` 的解析规则）、[SO: 用 REST `/related` 取依赖](https://stackoverflow.com/questions/59271325)（说明该关系在服务端也有一份可查的映射）。

#### M6 只增不删（kiln 现状）

内核的 `Fixes:` 可以一条修一个 bug、于是同一提交上挂多条 `Fixes:`；GitHub 的 cross-reference 在时间线里**只累积、不消失**。→ 与 kiln 的 `appendUnique` 并集语义（`engine.go:71-75`）是同一文化的两端：**把「关系只增」当成特性而非缺陷**。代价也同构——冗余边永久留存。

### 2. 把这六种机制对回本 issue 的候选

| 本 issue 候选 | 外部是否有人这么做 | 现成证据 |
| --- | --- | --- |
| **A** 只立约定（改历史） | 有 | Gerrit 新 patchset；邮件流 `PATCH v2`；jj/git 用户的 amend/rebase |
| **B** 追加型撤销 | **最多人这么做** | git-bug `Removed`；Fossil change artifact replace；CRDT redact |
| **C** 关系移出动作历史 | 有 | git-issue 的 `.issues/` 文件树；git notes 作为独立元数据层；`.mailmap` |
| **D** last-wins | 有 | Fossil 按时间重放；git-appraise 取最后一条 request |
| **E** 维持并集 | 有（但少） | 内核 `Fixes:` 多条；GitHub cross-ref 累积 |
| **反向推导** | **没找到同类先例** | git-dit 用**额外 parent** 显式跨 issue 引用，也不是「读侧自动推反向」 |

两条对 kiln 最有用的「现成分层」：

1. **git-dit 的分层**——「消息**不可变**（`Like emails, messages are immutable once released to the public`），head **指针可变**」；元数据由**从 head 沿 first-parent 走到初始消息**累积得到，跨 issue 引用靠**额外 parent**，多余的 leaf ref 可直接删（GC）。→ 这正是本 issue 取向 1 想要的形状：**「说过的话」不可变，「当前关系」是可变的另一层**，两层不互相绑死。
   > 一手：[libgitdit datamodel.md](https://docs.rs/crate/libgitdit/latest/source/doc/datamodel.md)、[libgitdit lib.rs](https://docs.rs/libgitdit/0.5.0/src/libgitdit/lib.rs.html)。
2. **Radicle COBs 的完备性**——issue/patch 是**签名的、只追加的 DAG**，同步时**取并集**合并、再按拓扑序**确定性归约**成当前态；`radicle-crdt` 为此提供 **LWW Set/Map、LWW Register、`Redactable`、Lamport 时钟**，而 `radicle-cob` 的接口是 **`create/get/list/update`——没有 delete**（删除语义由 CRDT 的 redact/LWW 承担，而不是删历史）。→ 给「并集语义」补上**删除维 + 冲突确定序**，正是 kiln 现在缺的两块。
   > 一手：[radicle-cob 0.17.0 docs（CRU Interface (No Delete)）](https://docs.rs/crate/radicle-cob/0.17.0)｜ 二手：[Radworks 2022-11 客户端更新（CRDT 列表：LWW Set/Map、LWW Register、Redactable、Lamport）](https://community.radworks.org/t/clients-community-update-november-2022/3141)、[DeepWiki: Radicle COBs（append-only DAG + CRDT 归约）](https://deepwiki.com/radicle-dev/heartwood/6.1-collaborative-objects-(cobs))。

### 3. 对 kiln 的直接启示（候选，不下裁定——裁定属 `conclude`）

1. **现状 E 是少数派**：同类工具里没有一个是「并集 + 只增不减 + 不做反向推导」；最接近的 M6 也只用于**天然只增**的关系（`Fixes:` 可以多条），而不是用于「写错了要改」的场景。
2. **最小增量的两条路**：
   - **B′（M1+M2）**：新增 `unrelates-to:` / `revises-relation:` 之类 trailer，读侧对同族关系做**后写覆盖**；这与 git-bug 的 `Removed`、Fossil 的字段 replace 同族，且「动作仍是提交、仍可审计」。
   - **C′（M3+M4）**：关系移进一个**独立、可编辑的 ref / 文件**（对齐 git notes 与 git-issue），动作提交只管内容。
3. **`path` 的呈现被迫要选边**：一旦关系可改，「每一步看到的关系」就不再等于「当前关系」。Fossil（当前态 + 历史表两份）与 git-dit（当前 = head 累积）都选择了**两者都给**——这对 issue 问题 5「`path` 要不要显示某条边后来被撤销过」是一个现成参照。
4. **反向推导没有先例**：git-dit 用显式 parent 连跨 issue 引用，Radicle 用 DAG + 归约——**都不是「声明一侧、读侧自动推反向」**。`relation-types` 的结论 1 若要落地，是**自创**，得自己担一致性成本（问题 3 里的「两侧可能不一致」在同类工具里靠**单一存储**规避，不靠推导）。
5. **可实测的下一步（留给 `test`）**：拿 B′ 或 C′ 各做一次最小实验——写一条关系、再用新机制把它撤掉，检查 `path` / `status` / `check` 三处的呈现与 `engine.go` 的 `appendUnique` / `relationLines` / `checkIssue` 改动量。

### 来源清单

- git-bug：[data-model.md](https://github.com/git-bug/git-bug/blob/trunk/doc/design/data-model.md)（一手）｜ [pkg.go.dev bug package](https://pkg.go.dev/github.com/MichaelMure/git-bug/entities/bug)（一手）。
- git-dit：[datamodel.md](https://docs.rs/crate/libgitdit/latest/source/doc/datamodel.md)（一手）｜ [lib.rs（messages immutable / metadata as trailers）](https://docs.rs/libgitdit/0.5.0/src/libgitdit/lib.rs.html)（一手）。
- git-appraise：[review.go（notes、取最后一条 request）](https://raw.githubusercontent.com/google/git-appraise/master/review/review.go)（一手）｜ [request.go（`refs/notes/devtools/reviews`）](https://raw.githubusercontent.com/google/git-appraise/master/review/request/request.go)（一手）。
- Fossil：[The Fossil Ticket System](https://www2.fossil-scm.org/index.html/artifact/717ff5a436834bf0)（一手）｜ [tickets.wiki](https://fossil-scm.org/home/doc/trunk/www/tickets.wiki)（一手）。
- git-issue：[README](https://github.com/dspinellis/git-issue)（一手）｜ [DeepWiki Issue Storage / Lifecycle](https://deepwiki.com/dspinellis/git-issue/3.1-issue-storage)（二手）。
- Radicle：[radicle-cob docs](https://docs.rs/crate/radicle-cob/0.17.0)（一手）｜ [Radworks 客户端更新](https://community.radworks.org/t/clients-community-update-november-2022/3141)（官方公告，二手转述）｜ [DeepWiki COBs](https://deepwiki.com/radicle-dev/heartwood/6.1-collaborative-objects-(cobs))（二手）。
- git 原生： [git-config（notes.rewriteMode）](https://git-scm.com/docs/git-config)｜ [gitmailmap](https://git-scm.com/docs/gitmailmap)｜ [git-replace](https://git-scm.com/docs/git-replace)｜ [git-interpret-trailers](https://git-scm.com/docs/git-interpret-trailers)（均一手手册）。
- 参照：[LKML SubmittingPatches: In-Reply-To](https://lkml.org/lkml/2015/11/5/651)（一手）｜ [gerrit.nordix Depends-On 实现](https://gerrit.nordix.org/gitweb?p=infra/cicd.git;a=commit;h=750a20fde6dd15e1cd02fe176d2422ceb06929c8)（二手/示例）｜ [SO: Gerrit related changes REST](https://stackoverflow.com/questions/59271325)（二手）。

## hypothesize：图论推演——「两个 issue 之间只有一种关系吗」，以及它如何决定 last-wins vs 追加撤销

> 一轮纯推演（2026-10-18），不引入新外部证据；把问题翻译成图论，看「一种 / 多种」到底是个什么量，以及它是否真的决定选型。

### 0. 先把问题翻译成图论

- 顶点 `V` = issue；**关系 = 带标签的有向边** `e = (u, t, v)`，`t ∈ T` = 5 个可写名（`relation-vocabulary` 已取消反向名，故 `|T| = 5`：`derived-from` / `part-of` / `blocks` / `relates-to` / `supersedes`）。
- **边的载荷为空**（除标签外没有别的信息）→「同起、同终、同标签」的两条边**无法区分** → 永远可折叠成一条。所以 kiln 的图**一定是简单图**（无平行边），这段没有选择空间。
- 于是唯一的自由度是：**同一对顶点之间，能同时挂多少条不同标签的边**。所谓「一种还是多种」，说的就是这个**纤维的大小**。

### 1. 两个有意义的极端（和中间的变体）

| 键 | 一对顶点之间最多几条边 | 图模型 | 代价 |
| --- | --- | --- | --- |
| **K1 无序对 `{u,v}`** | **1** | 一条边带一个标签 | 「来源 + 依赖」这类跨标签事实被压扁（反例见 §3） |
| **K3 `(无序对, 标签)`** | **5**（每标签一条） | 边着色的简单图 | 需防出现 2-环（`A→B` 与 `B→A` 不同标签可共存）；纤维稍大 |
| （K2 有序对 `(u,v)`，变体） | ≤ 2 | 一方向一槽 | 贴序但次自然；无向标签还需归一化，不单独讨论 |

两者都是**简单图**，区别只在纤维大小。再往上（真正的**多重边**：同一 `(u,v,t)` 存多条）对 kiln 没意义——边没有载荷，多重边不可区分。**所以 last-wins vs 追加撤销，并不在「一种 / 多种」这个轴上分叉，而在下面这个轴上分叉。**

### 2. 核心命题：last-wins 的「键」就是边的身份

- **last-wins** 需要两样东西：**键**（谁是「同一条边」）+ **全序**（谁更晚）。
- **追加撤销（B）** 需要：**边的可寻址身份**（撤哪条）+ 「撤销的撤销」的语义 + 「撤到空」的表示。
- → 只要**同一键上不允许平行边**，`(键, 值)` 就足以表达状态，**last-wins 就够，追加撤销是多余的机制**。键定下来了，机制就定下来了：**键的粒度 = 边身份的粒度 = 修改机制的复杂度**。
- 追加撤销真正不可替代的只剩两种情况：
  1. 同一键上要并存多条**带载荷**的边 → kiln 用不上（边无载荷）；
  2. 关系被当成**事件证据**，每条声明本身就是要留痕的事实，不能被「后来的写」盖掉 → 见 §5，即使要证据也可用**读侧 last-wins + 保留日志**同时满足（Fossil 的当前态表 + 历史表），不必发明撤销 trailer。

### 3. 那「两个 issue 之间只有一种关系」成立吗

逐个检验 5 个标签能否共存：

| 组合 | 能共存吗 | 依据 |
| --- | --- | --- |
| `relates-to` × 任何强类型 | **不能** | 它的定义就是「相关但**不派生**」，是兜底/垃圾桶（`relation-vocabulary` 决策树最后才轮到它） |
| `derived-from` × `part-of` | **不能** | 判据互斥：母等**全部**子（分解）vs **不等**（引发） |
| `derived-from` × `blocks` | **能，且常见** | 反例见下 |
| `supersedes` × `derived-from` | 可能 | 「新方案由旧方案引发，并取代它」 |

**关键反例（K1 的漏洞）**：

> `B` =「排查间歇性失败」，`A` =「加 retry」。事实是两条：
> 1. **`B blocks A`**（排查没完，修不了）——方向 阻塞者→被阻塞者，`B→A`
> 2. **`A derived-from B`**（修复由排查引出）——方向 被引发者→来源，`A→B`
>
> 两条**方向相反、同一对顶点**。K1（无序对）让它们撞成一条槽 → **必然丢一条**。

所以「两个 issue 之间只有一种关系」**在 K1 下是人为约束，不是自然事实**：它会压扁「来源 + 依赖」这个很常见的组合。

**但要公平地看**：`relation-vocabulary` 的决策树是「按顺序问，第一个『是』就是答案」——它**一次只给一个标签**，即现有语义其实**已经按 K1 在设计**。用户提的方向与决策树自洽，只是需要意识到：K1 主动**放弃了跨标签共存的表达力**，换取「一键一槽」的简单。

### 4. 选型

| 键 | 一对之间可共存 | last-wins 够吗 | 额外代价 |
| --- | --- | --- | --- |
| **K1** `{u,v}` | 1 | **够，最简** | 丢反向/跨标签事实；但**免费**拿到 2-环防护（`A→B` 与 `B→A` 同槽） |
| **K3** `(u,v,标签)` | ≤5 | **够**（同标签重写 = 改类型） | 保留反例与跨标签共存；2-环需**显式**检测；需接受「每标签至多一条」 |

**推演倾向 K3**，理由是它把两种不同的修正**分开**了：

- 「**同一标签改类型**」（`relates-to` → `derived-from`，或 `derived-from` → `part-of`）→ 交给 **last-wins**（这不是两条事实，是前一条写错了）；
- 「**两条不同的事实**」（来源 + 依赖）→ 留给**两个标签**。

K1 把这两者一起压扁，为了省一个键维度，损失了真实表达力。而 K3 的键 `(对, 标签)` 恰好就是「纤维按标签分层」，与取消反向名后的 5 个可写名一一对应。

**但这是推演，不是裁定**：如果实测发现跨标签共存很少，K1 的简单就值回票价（见 §6）。

### 5. last-wins 不是零成本：三处依赖 + 一个 tombstone

写侧 last-wins 在 kiln 里**不可能**——提交不可变，删不掉旧 trailer。所以 last-wins **只能发生在读侧**（派生视图）。由此带出四条硬约束：

1. **必须单侧声明**：键只会出现在**声明方**那条 issue 的提交史里。若两侧都写，同一对有两个声明点，两段提交史之间**没有全序**，last-wins 无从比较（这正是 git-bug 要 Lamport 时钟 + op-pack 哈希的原因）。→ 选 last-wins **等价于必须落地反向推导**（`relation-types` 已定「声明一侧、读侧推导反向」，但代码里没有）。
2. **必须有「删到空」的写法**：last-wins 只解决「改类型」，不解决「这对顶点从此没有关系」。需要一个 tombstone（`unrelates-to` / 空值 / `--clear`）。**这一小块正是追加撤销里唯一不可省的部分**，其余都能被 last-wins 吸收。
3. **必须有确定序**：kiln 每个 issue 的动作提交本就是一条（近）线性史，`git log` 给得出顺序，比分布式 CRDT 简单得多；只有并行分支编辑同一 issue 时才分叉，需要 tiebreak（git-bug 用 op-pack 哈希）——单 agent 场景可先不处理。
4. **tombstone 与「曾撤销过」的呈现**：读侧覆盖后，`path` 若还想显示「这条边后来被撤过」，必须**保留日志并额外标注**。→ **「当前事实」与「留痕」并不矛盾**：last-wins 是读侧投影，日志原样留着。

### 6. 与取向 1 的关系，以及可实测的下一步

- 「一槽 / 每标签一槽」＝关系是**当前状态的描述**（键有确定的值）——与本 issue 取向 1（关系描述现在，不记录某一步发生了什么）**同构**。
- 反过来，若坚持关系是**证据**（每条声明都是事件），机制上仍是「事件日志 + 读侧 last-wins 投影」，只是**呈现**要把被覆盖的边也列出来；所以取向 1 不直接决定 K1/K3，但**决定了 last-wins 是否可接受**——可接受，因为提交不可变，覆盖只发生在读侧。
- **可实测（留给 `test`）**：
  1. 造最小仓：同一对顶点 (a) 先 `relates-to` 再 `derived-from`；(b) 反向共存 `blocks` + `derived-from`。看 `path` / `graph` / `check` 在 K1 与 K3 下的输出。
  2. 量改动面：`relationLines`（`engine.go:302`，只画本 issue 三族）要改几个分支；`appendUnique`（`engine.go:71`）要不要换成「键 → 值」的 map；`path` 的「每步并集」要不要改成「当前态」。
  3. 扫全库 41 个 issue：**同一对顶点已存在多条边**的有几例？（若为 0，K1 的表达力损失在本仓还没发生——它仍是可选项。）

### 结论（推演，未定论——裁定属 `conclude`）

1. **用户的方向对**：只要不允许多重平行边，**last-wins 就够，追加撤销是超集**。
2. **但分野不在「一种 / 多种」，而在「键取多细」**：K1 是从「决策树只给一个标签」读出的最强约束；K3 更贴图的自然身份（每标签一条）。两者都能用 last-wins。
3. **推演倾向 K3**：同标签 last-wins（改类型）+ 跨标签共存（来源 + 依赖），代价是要认定「每标签至多一条」并显式防 2-环。
4. **无论 K1 还是 K3，选 last-wins 都强制两件事先落地**：**反向推导**（单侧声明）与 **删到空 tombstone**。
5. 真正需要证据的是：**跨标签 / 反向共存在本仓到底有没有发生**——这是 `test` 的活，不是推演能定的。

## hypothesize（续）：关系可降到两条结构轴（有向/无向 × 阻塞/非阻塞）

> 起因（用户，2026-10-18）：关系到底多少类、叫什么名字，**还不成熟**；但总的来说可以分成**有向 / 无向**与**阻塞 / 非阻塞**。
> 本续只取这两条**结构轴**，看它对「修改机制」意味着什么。**不动词表与命名**——那是 `relation-types` / `relation-vocabulary` 的地盘，也是本 issue 的非目标。

### 0. 把 5 个现名投影到两轴

| 名 | 有向 / 无向 | 阻塞 / 非阻塞 | 依据 |
| --- | --- | --- | --- |
| `derived-from` | 有向 | 非阻塞 | 引发，可分头处理（`relation-types`） |
| `part-of` | 有向 | **阻塞** | 分解 = AND-join：母等**全部**子 |
| `blocks` | 有向 | **阻塞** | FS 依赖 |
| `supersedes` | 有向 | 非阻塞 | `relation-types` 定「无顺序」 |
| `relates-to` | **无向** | 非阻塞 | 双向相关 |

→ 2×2：

| | 阻塞 | 非阻塞 |
| --- | --- | --- |
| **有向** | `part-of` `blocks` | `derived-from` `supersedes` |
| **无向** | **∅** | `relates-to` |

### 1. 两个发现

**发现 A：无向×阻塞 结构性为空。** 互相阻塞 = 死锁，是**错误态**，不是一种关系；若有人想要「互斥 / 冲突」，那其实是「非阻塞的关联」（两条不能并存），不属阻塞象限。→ 象限实际只有 **3 个**。（若真需要「互斥」，是词表留白。）

**发现 B：两条轴各自恰好决定修改机制的一维。**

- **有向 / 无向 → 键的对称性**：有向键 = 有序对 `(u,v)`；无向键 = 无序对 `{u,v}`（字典序归一化）。这决定「同一条边」怎么判定，也决定无向关系不需要反向推导、有向需要。
- **阻塞 / 非阻塞 → 环约束**：阻塞边构成**偏序** → 图必须无环（写入时 DFS / Tarjan 拒成环，`relation-types` 已调研）；非阻塞边只是关联 → **允许环**（`relates-to` 成环很正常）。

### 2. 比「5 个名字」更稳的键：K4 = (对, 象限)

上一轮给了 K1（对，一槽）/ K3（对，标签）。两轴给出中间那档：

| 键 | 槽数 | 绑定到词表？ | 表达力 |
| --- | --- | --- | --- |
| **K1** `{u,v}` | 1 | 否 | 太粗（压掉反例） |
| **K4** `({u,v}, 象限)` | **3** | **否** | 中 |
| **K3** `({u,v}, 标签)` | 5 | **是** | 满 |

K4 的收益，恰好回应「名字还不成熟」：

1. **抗词表 churn**：改名 / 合并 / 新增标签都**不动键**；机制不被尚未定稿的命名绑架。
2. **上一轮的反例自动解决**：`B blocks A`（有向阻塞）与 `A derived-from B`（有向非阻塞）落**不同象限** → 可共存；K1 会压扁。
3. **免费 2-环防护正好落在该落的地方**：`A blocks B` 与 `B blocks A` 同属阻塞象限 → 同槽折叠 → 结构上自动无 2-环；非阻塞象限允许环；**长环（≥3）**仍需显式检测。
4. **与决策树同构**：决策树第一层问「要不要等」（= 阻塞 / 非阻塞），第二层才是「哪种」（= 象限内的标签）。象限就是决策树的第一层。

### 3. 但两轴不够：`relates-to` 的「兜底」是第三条轴

这是本续的关键修正。`relates-to` 在词表里的定义是**最后手段**（「先试其他四个，都不成立才用」）——即它是**跨象限的兜底 / 默认**，不是与别人并列的一个象限。而「兜底」意味着**被更强的关系取代**：

> 真实痛点（本 issue 的 wsl-agent 案）：先写了 `relates-to`，后来判定真语义是 `derived-from` + `blocks`——但 **K4 不会让它退场**，因为 `relates-to`（无向非阻塞）与 `derived-from`（有向非阻塞）在**不同象限**，结构上是两条可共存的边。

所以修改机制还差一条**语义规则**（非结构）：

- **兜底规则 F**：`relates-to` 是同一条边上「没有更强关系」时的默认值；只要该对顶点在任一**强象限**（有向阻塞 / 有向非阻塞）有边，`relates-to` 即视为**作废**（读侧投影时不呈现）。

把 F 加进来，得到本续的综合键：

> **K4+F**：强关系按 `({u,v}, 象限)` 分槽、槽内 LWW；`relates-to` 不占槽，是「强槽全空」时的默认。

三件事同时成立：

- wsl-agent 案：写 `derived-from` → 强槽非空 → `relates-to` 自动退场 ✓
- 反例：`blocks` 与 `derived-from` 分属两强槽 → 共存 ✓
- 词表 churn：强槽仍是结构象限，改名不动 ✓

### 4. 遗留与待验证（留给 `test` / `conclude`）

1. **有向非阻塞象限里 `derived-from` 与 `supersedes` 会撞**：K4 下它们同槽、至多一条。三种处置：(a) 承认「新由旧引发并取代旧」时 `derived-from` 就够（`supersedes` 冗余）；(b) 象限内再分槽 → 退回 K3；(c) 认为 `supersedes` 其实是阻塞（旧的要停）→ 挪进阻塞象限。**这条必须定**，否则 K4 的槽划不干净。
2. **无向×阻塞象限为空**是否绝对：若要「互斥 / 冲突」，它会落在这个象限（或造一条新的无向阻塞）——目前是词表留白。
3. **F 的边界**：`relates-to` 被「作废」后**还要不要留痕**（`path` 里显示「曾是 relates-to」）？——与 §5 的 tombstone 同题：读侧覆盖，日志留着。
4. **可实测**：把 41 个 issue 的现有边按象限归类，统计每个 `(对, 象限)` 的重数是否都 ≤1；特别看有向非阻塞象限里 `derived-from` 与 `supersedes` 撞过没有、以及有多少对是「`relates-to` 与强关系并存」（= F 该作废的）。

### 结论（推演，未定论）

1. **两轴是对的骨架**：有向 / 无向 定**键的对称性**，阻塞 / 非阻塞 定**环约束**；象限给出一个**抗词表 churn** 的键 `(对, 象限)`。
2. **但两轴不足以决定修改机制**：`relates-to` 的**兜底**是跨象限的第三条（语义）轴，必须显式立规则 **F**（或改用 K1，把兜底结构化成「一槽」）。
3. **综合 K4+F** 同时解决四件事：跨象限共存、兜底退场、免费 2-环防护、词表无关。
4. **仍需先定**：有向非阻塞象限内 `derived-from` / `supersedes` 的槽怎么分——否则 K4 不成立。

## hypothesize（再续）：`derived-from` 与 `supersedes` 的区别——「覆盖/接替」是第三维，建议同槽不同档

> 用户追问（2026-10-18）：这两个关系的区别是什么？取代 = 新 issue 范围**覆盖**旧 issue；衍生 = 两者**没有**这个覆盖关系。且名字/类数也未完全定。

### 1. 区别一句话：只问「旧的那个还要不要做」

| | 断言的是 | B 还要不要做 | 判据问题 |
| --- | --- | --- | --- |
| `derived-from`（A→B） | **因果 / 谱系**：A 因 B 而生 | **要做**（A、B 各自独立） | 「B 还要不要做？」→ 要 |
| `supersedes`（A→B） | **范围覆盖**：A 顶替 B | **不用做**（B 作废） | 「B 还要不要做？」→ 不做 |

**关键性质**：这个判据问题**只有一个答案**——同一对顶点上，B 不可能既「要做」又「不用做」。所以**应该同槽，而不是两个槽**：

> `supersedes` = `derived-from` **+ 覆盖** —— 它不是另一个维度的事实，而是同一件事的**强化档**。

### 2. 它们能不能同时为真？

逻辑上有三种情形：

| 情形 | 能否发生 |
| --- | --- |
| 只派生、不覆盖 | 能（引发但都要做） |
| 只覆盖、不派生 | 能（别人独立开了个 A 顶替 B） |
| **两者同时** | 能（A 由 B 引出，且 A 顶替 B）——最常见 |

第三种「同时」看起来像要求两个槽。但它其实**不需要**：一旦 A 顶替 B，「A 由 B 引出」要么是显然的（接替本身就是最强的引发），要么对修 bug 没用——`supersedes` 已经把「A 接替 B」说完了。所以**同槽、取强档**就够了。

### 3. 建议：槽内是一个「强度格」，不是一个平铺枚举

把每对顶点的每个象限想成一个**有强度序的槽**：

**非阻塞槽**：`relates-to` ≺ `derived-from` ≺ `supersedes`
- `relates-to`：有关联，不派生，无方向（最弱，兜底）
- `derived-from`：有派生，带方向
- `supersedes`：派生 **+ 覆盖**（旧作废）

**阻塞槽**：`blocks` ≺ `part-of`
- `blocks`：A 阻塞 B（先后 / FS）
- `part-of`：A 是 B 的子，母等**全部**子（AND-join）——含 `blocks` 的信息，又多一层「分解结构」

> 两个强度格都有一个好听的性质：**强档蕴含弱档的语义**（`supersedes` 蕴含派生，`part-of` 蕴含阻塞），所以「升级档位」永远不会丢失那个弱档要说的东西。

### 4. 这一步把上两节的三件事统一了

1. **兜底规则 F 不用另立**：`relates-to` 就是非阻塞槽的**最弱档**——写 `derived-from` 就是把它升级（覆盖），wsl-agent 的痛点自然解决。F 从一条外挂规则变成了**强度格的下界**。
2. **反例自然解决**：`B blocks A`（阻塞槽）与 `A derived-from B`（非阻塞槽）不同槽 → 共存。
3. **「写错了怎么改」有了统一解释**：同一槽上**用更强（或更晚）的档覆盖**；跨槽则共存。两个轴（方向 / 阻塞）定槽，强度序定档。

### 5. 对「名字未定」的回答：机制绑「语义角色 + 强度序」，不绑名字

槽应该钉在**关系对工作项生命周期的作用**上，名字可换：

| 槽 | 语义角色 | 强度序（弱→强） |
| --- | --- | --- |
| **阻塞槽** | 约束工作顺序 / 结构 | 依赖 → 分解 |
| **非阻塞槽** | 谱系 / 接替 | 关联 → 引发 → 取代 |
| （无向×阻塞） | **∅**（互阻塞=死锁，不是关系） | —— |

只要「角色 + 强度序」不变，`derived-from` 改名成什么都动不到机制；新增一个「比引发强、比取代弱」的关系（如 `duplicates`）也只是往格子里插一档。→ 正回应你说的「名字没完全定」。

### 6. 代价与约定（必须写下来）

1. **同槽取强 = 约定「接替蕴含派生」。** 若真存在「无派生的接替」（A 与 B 无谱系关系，只是范围碰巧覆盖），那 `supersedes` 对该派生断言就是**有损**的。要么接受这条无损假设，要么承认它是有损约定。
2. **`derived-from` + `supersedes` 双双要留痕时无解**：同槽只能存一档。若真要两条事实都展出来，就得在槽内再开子槽（退回 K3）。**但本推演认为不需要**。
3. **`part-of` 归阻塞槽的另一面**：`part-of` 还带「分解结构 / AND」，比单纯的阻塞多一个**结构**维度——如果以后要给子问题做进度条 / 100% rule 校验，这一维会被用到。目前先按强度档处理。
4. **阻塞 / 非阻塞 两槽会不会互相覆盖？** 不会——它们键不同（不同象限），这就是 K4 的本意；只有**同一个** (对, 槽) 才 LWW / 取强。

### 7. 可实测（留给 `test`）

1. 扫全库：有没有哪一对 issue **同时**写了 `derived-from` 与 `supersedes`（或 `blocks` 与 `part-of`）？若无 → 同槽取强在本仓还未丢过东西。
2. 实现最小原型时，`engine.go` 里把 `appendUnique` 换成 `map[键]值`，看 `relationLines`（`engine.go:302`）按槽分组渲染要改多少。
3. 试造一个「无派生的接替」（A 与 B 无文字谱系，只 A 顶替 B），看同槽取强会不会让 `path` 读起来别扭。

### 结论（推演，未定论）

1. **区别**：`derived-from` = 派生（旧的**还要做**）；`supersedes` = 派生 **+ 覆盖**（旧的**作废**）。判据一个问题定音，所以**同槽**。
2. **建议**：槽内是**强度格**，不是平铺枚举——非阻塞 `relates-to ≺ derived-from ≺ supersedes`；阻塞 `blocks ≺ part-of`。
3. **收益**：兜底 F、跨象限共存、写错退场、**词表 churn 免疫**——四件事用同一套解释说完。
4. **代价**：同槽取强 = 约定「接替蕴含派生」；不接受就退回两槽 / K3。
5. 由此，上一节遗留的「有向非阻塞象限的槽怎么分」有了答案：**不分，同一槽内分档**。

## hypothesize（三续）：relation 词表要不要像 action 词表一样可自定义

> 用户推演（2026-10-18）：action 是原子、可自由组合；issue 也原子、由不同关系组合。relation 要不要保留同样的灵活性——**分了类，但具体名称不绑定，可自定义**？

### 0. 先把类比钉死：action 的「可配置」到底配置了什么

`.kilnconfig.jsonc` 里 action 词表可改，但**不是什么都可改**：

| 层 | 谁定 | 内容 |
| --- | --- | --- |
| **结构层** | 引擎固定 | 必须有**起点**动作；必须有≥1 个**终点**动作；动作组成有序 path |
| **名字层** | 项目配置 | `names`、数量、哪一个是 start、哪些是 terminal |

→ 「可配置」= **名字自由、角色映射可配；但引擎用来推理的「角色」是固定的**。

### 1. 平移到 relation：完全同构，而且刚好补上了「名字未定」

| 层 | 谁定 | relation 里的内容 |
| --- | --- | --- |
| **结构层** | 引擎固定 | 键 = `(无序对, 槽)`；槽的 `blocking`（→ DAG 检测）；槽内**强度全序**（→ LWW 取强）；`directed`（→ 反向推导） |
| **名字层** | 项目配置 | 每个名的（名字、释义、所属槽、槽内 rank、是否 directed）、槽表 |

**答案：该保留灵活性，但只在「名字层」——结构层绝不能可配**，否则环检测/取强/反向推导都无处落脚（引擎逻辑被交给用户）。

### 2. 配置形态（草图，贴 `actions` 的风格）

```jsonc
"relations": {
  // 槽：结构层，引擎只认 blocking 这一个布尔
  "slots": {
    "ordering": { "blocking": true },   // 约束顺序/结构
    "lineage":  { "blocking": false }   // 谱系/接替
  },
  // 名字层：映射到槽 + 槽内强度序（1 最弱）；directed 决定反向推导
  "names": {
    "blocks":       { "slot": "ordering", "rank": 1, "directed": true },
    "part-of":      { "slot": "ordering", "rank": 2, "directed": true },
    "relates-to":   { "slot": "lineage",  "rank": 1, "directed": false },
    "derived-from": { "slot": "lineage",  "rank": 2, "directed": true },
    "supersedes":   { "slot": "lineage",  "rank": 3, "directed": true }
  },
  // 历史别名：提交不可改，旧名必须仍能解析
  "aliases": { "spawns": { "as": "derived-from", "flip": true } }
}
```

引擎侧：key = `(无序对, slot)`；value = `(name, direction)`；同槽取 **rank 最大**（平手按时间）。

### 3. 为什么这是对的（四条收益）

1. **与 action 对称**：kiln 就有两处「词表即自定义点」，理念一致——这是「原子 + 自由组合」在**边**上的自然延伸。
2. **直接回应「名字未定」**：改名 / 翻译 / 增删 / 调强度，全在配置里，引擎一行不改。
3. **引擎保持通用**：只认 `slot / blocking / rank / directed`，不认任何具体名字——这正是上一节「机制不绑名字」的落地形态。
4. **释义可跟配置走**：`relation-vocabulary` 的结论是「释义本身就是规则，Skill 只要 5 个键 + 一句话释义」。名字可配后，**这句释义就该从配置里读**，`-h` / Skill 可**数据化生成**，不用再两处维护。

### 4. 绝不不能再可配的（引擎的落脚点）

| 不可配 | 为什么 |
| --- | --- |
| 键 = `(无序对, 槽)` | 一整张图的存/读方式；可配就没法比较、没法反向 |
| `slot.blocking` | 决定要不要跑 DFS / Tarjan 防环 |
| 槽内**全序** | LWW「取强」靠它；无全序就无法定谁盖谁 |
| `directed` | 反向推导靠它 |

> **一条设计校验规则**：槽内必须是**全序**；两个关系若**不可比**（谁都不蕴含谁），就必须**分属不同槽**。这条规则使「自定义」安全：不论用户怎么加名字，引擎的推理都有落脚点。

### 5. 一并要解决的坑

1. **历史别名**：`derived-from` / `blocks` 等已写进提交，改名不改历史 → `aliases` +（必要时）方向 `flip` 是必需品（与 action 改名同题）。
2. **`check` 对未知 trailer**：配置里没声明的键，报错还是提示？— 连到 `silent-flag-drop`。
3. **默认词表 = 现有 5 名**：保住 `relation-vocabulary` 已验证过的 LLM 可读性；自定义是 **opt-in**，不是“什么都自己起名”。
4. **跨项目 / 桥接**：两仓词表不同时需要 glossary 映射；kiln 单仓单项目，暂可不管。
5. **无向×阻塞 槽**：应视为**空槽**而不是不允许存在——配置里允许它，但 `blocking + !directed` 给出警告（互阻塞=死锁）。

### 6. 可实测（留给 `test`）

1. 把 `internal/model` 里的五个固定字段（`DerivedFrom` / `Spawns` / `RelatesTo` / `Blocks` / `Supersedes`）换成 `map[(对,槽)] → (name,dir)`，看 `engine.go` 的 `appendUnique` / `relationLines` / `checkIssue` 改动面。
2. 写一个只改了名字（如 `derived-from` → `stems-from`）+ aliases 的仓库，看 `path` / `graph` / `check` 是否仍能读旧提交。

### 结论（推演，未定论）

1. **该保留灵活性**——但要同 action 一样：**自由度给「名字」，结构由引擎固定**。
2. **结构层三项必须固定**：键 `(对,槽)`、槽的 `blocking`、槽内**强度全序**（+ `directed`）。
3. **名字层全部可配**：名字、数量、释义、槽归属、槽内 rank、槽表。
4. 形态上就是 `actions` 的翻版：**`relations.slots` + `relations.names` + `aliases`**；默认词表就是现有 5 名。
5. 一个硬约束：**槽内必须全序；不可比者分槽**——这是自定义安全性的根。

## hypothesize（四续）：写侧方向与语态——「后→先」该降为「声明方→被引用方」

> 用户推演（2026-10-18）：写关系一般是在**第二个 issue 出现后**，写明它跟已有 issue 的关系，所以有先后。**正向关系应定为后 issue 对先 issue 的关系**，据此再定主动/被动语态。问合理吗。

### 0. 直接回答：合理，但要把「后→先」从**语义方向**降为**声明方向**

两个东西必须分开：

| | 含义 | 谁决定 |
| --- | --- | --- |
| **声明方向** | 这条 trailer 写在谁的提交里、指向谁 | 写的人（声明方→被引用方） |
| **语义方向** | 谁因谁而生 / 谁等谁 / 谁接替谁 | 关系本身 |

你说的实际是前者：**声明方向恒为 声明方 → 被引用方**，而声明方**通常**就是新 issue。这不等同于语义方向。

### 1. 为什么「后→先」不能当语义方向（反例）

| 语义 | 语义是否天然随时间 | 「后→先」成立吗 |
| --- | --- | --- |
| `derived-from` | 是（来源更早） | ✓ |
| `part-of` | 是（母题更早） | ✓ |
| `supersedes` | 是（旧版更早） | ✓ |
| **`blocks`** | **否** | ✗ |

> `blocks` 的反例：B 是 A 的前置、B 先建 → 新 issue A 写 `A blocked-by B`；但也可以是 A 先建，后来才发现要先做 B → **新建 B** 去阻塞 A，则声明方 B 是后来的。**同一条语义，声明方可能是先的也可能后的。**

→ 规则应写成「**声明方→被引用方**」，而不是「后→先」；后者只是通常情形。

### 2. 由此推出的命名规则（回答语态）

**名字的主语应当是声明方。** 于是每条**非对称**关系需要两个名字（声明方扮演哪个角色就用哪个），按角色选语态：

| 语义 | 声明方角色 | 名字 | 语态 |
| --- | --- | --- | --- |
| 引发 | 被引发者 / 来源 | `derived-from` / `spawns` | 被动 / 主动 |
| 分解 | 子 / 母 | `part-of` / `has-part` | 介词 / 主动 |
| 依赖 | 等待方 / 阻塞者 | `depends-on`（或 `blocked-by`）/ `blocks` | 主动 / 主动 |
| 取代 | 新 / 旧 | `supersedes` / `superseded-by` | 主动 / 被动 |
| 关联 | — | `relates-to` | 对称 |

**「主语=声明方」这条不变式比「主动/被动」更本质**：语态只是副产品（`supersedes` 主动、`derived-from` 被动，但主语都是声明方）。若你偏好**语态统一**（全主动、主语=新），则依赖族主名取 `depends-on`（YouTrack 风）而不是 `blocks`。

### 3. 这会与 `relation-vocabulary` 的「取消反向名」冲突吗？不——那是**渲染层**的决定

- `relation-vocabulary` 取消反向名，理由是**渲染**：同一条边无论从哪侧声明只画一种箭头（实验二 2/2 agent 自己发现「8 条声明实际只有 4 条渲染」）。—— 对。
- 但**写侧**如果只有 5 个名，当声明方处在**反向角色**时就写不出来：创建 A 发现被 B 阻塞，只有 `blocks` 就只能去改 B。—— 不行。
- → 调和：**写侧 9 个名（按角色选），读侧只渲染一种箭头**（目标 issue 的视角用反向名渲染，不存第二份）。

### 4. 主名（primary）选哪一侧

既然常见声明场景是「新 issue 写向已有 issue」，**主名应当是「声明方=新/被引发/子/等待方」那一侧**：

- 引发 → `derived-from`（新是被引发者）
- 分解 → `part-of`（新是子）
- 依赖 → `depends-on` / `blocked-by`（**新是等待方**）
- 取代 → `supersedes`（新是新版）
- 关联 → `relates-to`

→ 这**直接回答**了 `relation-types` 的遗留待探讨「依赖以哪一侧为正名」：**取等待方视角**（`depends-on` / `blocked-by`），而不是主流 Jira/Linear 的 `blocks`。代价是偏离主流用词，收益是贴合「写关系时手在哪个 issue 上」。

### 5. 对 relation-revision 机制的影响：几乎为零

- **槽键 = (无序对, 槽)**，所以 `blocks` 与 `blocked-by` / `depends-on` 落**同一槽同一 rank** → 不会因写了哪个名而重复；LWW / 取强不受影响。
- **反向推导** = 目标 issue 的读侧**换名渲染**，不需要存第二份边。
- → 反向名与「9 个可写名」不冲突：两个名映射到同一个 `(对, 槽)`，正是上一节「9 个名字不该绑架机制」的具体体现。

### 6. 建议（结论）

1. **修正规则**：正向不是「后→先」，而是「**声明方→被引用方**」（通常后→先，但 `blocks` 是反例）。
2. **不变式**：关系的**主语恒为声明方**；由此每条非对称关系需要**正反两个可写名**（9 个），语态随角色。
3. **写 9 / 渲 1**：写侧 9 名，读侧只渲染一种箭头形态（把「取消反向名」限定在渲染层）。
4. **依赖族主名改为等待方视角**（`depends-on` / `blocked-by`），`blocks` 降为反名 / 读侧视图。—— 这会改动 `relation-vocabulary` 的现有 5 名结论，应回写那里。
5. **默认/模板的取向**：主名一律选「主语 = 新 issue」那一侧；这条规则可直接用来**审校**默认词表。
6. **机制无关**：不管最终 5 名还是 9 名，槽/rank/键都不动——这正是「名字可自定义」设计的好处。

## test：扫全库真实关系——检验「每 (对,槽) 只有一个值」与 K4 / 强度格（2026-10-18）

> 本 issue 的首个实测：解析本仓 `git log` 的全部 trailer，看真实关系分布是否支持设计假设。脚本：Python 解析 `issue/action/derived-from/spawns/relates-to/blocks/supersedes/part-of/has-part` 八个 trailer。

### 1. 数据

- **42 个 issue、44 条关系声明、48 条边**（有的声明多目标）。
- 按键：`derived-from` **16**、`relates-to` **27**、`blocks` **1**；**没有** `spawns` / `supersedes` / `part-of` / `has-part`。
- **dangling 目标：0**（所有目目标都存在）。

### 2. (无序对, 槽) 重数 >1：仅 3 例

**例 1** — `(cli-necessity, config-home)` 非阻塞槽：
`config-home relates-to cli-necessity` 在 config-home 的 **4 个动作里重复写了 4 次**。
→ `appendUnique` 在**同一 issue 内**会去重，模型层只 1 条；但这说明**同一条边被反复声明**是常态。

**例 2** — `(derived-from-direction, relation-vocabulary)` 非阻塞槽：
`relation-vocabulary relates-to derived-from-direction` **且** `derived-from-direction derived-from relation-vocabulary` —— **同对、同槽、两族**。
→ 这正是 wsl-agent 那个“`relates-to` 无法退场”案的**真实复现**；强度格（`relates-to` 弱于 `derived-from`）会折叠为 `derived-from`，**正确**。

**例 3** — `(relation-rendering, relation-vocabulary)` 非阻塞槽：
**两侧都写了 `relates-to`**（`relation-vocabulary → relation-rendering` 与反向）——**单侧声明约定被人工违反了**；无序对键去重即可。

**反向共存：2 对**，正是例 2 / 例 3。

### 3. 结论

1. **K4 被真实数据支持**：3 例重数**全部**是「同一条边重复」或「应被强度格折叠」，**没有一例是“两条必须共存的同槽边”**。→ **单值槽 + 强度格够用**，不需要多重边。
2. **强度格不是空转**：例 2 就是它要解决的（`relates-to` 被 `derived-from` 升档）。
3. **单侧声明靠不住**（例 3 双侧都写）→ **反向推导 + 无序对去重是必需品**，不是优化。
4. **dangling = 0**：`check` 的“目标存在”缺口暂无实害，但仍应对齐 5 键。
5. **词表用量高度集中**：`relates-to` 占 56%、`derived-from` 33%、`blocks` 2%——强度格实际主要服务 relates-to ↔ derived-from。

### 4. 局限

- 只 42 issue / 48 边；`part-of`/`causes` 等新名未落地，无法测。
- 静态解析，未覆盖运行时的入边索引/渲染行为。

### 5. 剩余可实测项（未做）

- **量改动面**：`internal/model` 的五个固定字段（`DerivedFrom`/`Spawns`/`RelatesTo`/`Blocks`/`Supersedes`）→ `map[(对,槽)]`；`engine.go` 的 `appendUnique` → 取强；`relationLines`（`engine.go:302`）→ 按槽分组。
- **最小仓实验**：造只有 2-3 个 issue 的仓，验证“同槽改族（relates-to→derived-from）”与“删到空 tombstone”在 `path`/`graph`/`check` 三处的行为。

## hypothesize（五续）：收尾细节——tombstone 写法、历史改写政策、「曾撤销」呈现、check 补齐

> 把 ask 里 Q4/Q5/Q6 与 test 例 3 暴露的四个尾巴一次收定。

### 1. tombstone（删到空）怎么写

机制前提：关系是 `(对, 槽)` 的**单值**，读侧 LWW。所以“删到空”= 往该槽写一个**墓碑值**。

**提案**：

- **主形式 `unrelate: X`**：作废 `{本条 issue, X}` 的**全部槽**（常见情形：“这条关系写错了，去掉”）。
- **同动作内优先级**：**声明优先于 `unrelate`**。→ 可写 `unrelate: X` + `blocks: X` 表示“清掉重写”（清空后只留 `blocks`）。
- 支持逗号分隔多目标，与其它 trailer 一致。
- **不**新增 `unrelate-*` 一族名（避免词表翻倍）。

**为什么不按槽细分**：实测（test 第 2 节）里**没有一例同一对需要两个槽**；全槽作废几乎总等于目标槽作废。真需要“只删一个槽、留另一个”时，用“`unrelate` + 同行重声明”即可。

### 2. 历史改写政策（ask Q4）

- **官方答案 = 追加 `unrelate`，不改写历史。**
- 理由：墓碑消除了“必须改历史”的**唯一场景**（写错了要改）；而 README 的“用 git/jj 自己的本事”依旧是用户自由，但不再是**修关系的官方方式**。
- → 与“不重写历史”的默认叙事一致，且机制上**不需 rebase/amend**。

### 3. “曾撤销”的呈现（ask Q5）

- **默认：读侧 LWW 覆盖，只显当前态**（与取向 1 一致；与 `evidence-fluidity` 的“一般视图呈现当前态”一致）。
- **不丢历史**：被覆盖 / 被撤销的边仍是提交内容，可由 `kiln show <issue> <action>` 看到。
- `path` **默认不显示**“曾被撤销”；需要时另加 `--history`（是否做留待实现）。

### 4. `check` 补齐（ask Q6）

现状：`checkIssue` 只校验 `derived-from` 父存在、`spawns` 子存在。

**提案（分批）**：

| # | 规则 | 严重度 |
| --- | --- | --- |
| 1 | **全键目标存在**：9 个写侧名 + `unrelate` 的目标都要存在 | ⚠ 警告（不 error） |
| 2 | **双侧声明同一槽**（test 例 3）→ 冗余 | ⚠ 提示 |
| 3 | **`part-of` 的 AND**：父项已有终点动作、但有子项未到终点 | ⚠（见 `part-of` 结论） |

- 第 1 条是最小闭环，可直接落地；2/3 可分批。
- 与 `mvp-plan §10`（blocks 状态一致性）相邻但不同，不合并。
- **不报**“同槽反向共存”：那是强度格该折叠的，不是错误（否则会把正常升级报成错）。

### 5. 单侧声明被违反怎么办（test 例 3）

- kiln 只读 git，**无法阻止**双侧声明 → 只能：① 读侧**去重**（无序对 + 强度格）；② `check` 给 ⚠（§4 第 2 条）。
- → 反向推导依旧是**必需**（否则单侧声明时目标侧根本看不到入边）。

### 结论（hypothesize）

1. **tombstone**：`unrelate: X`（作废全槽；同动作内声明优先；不新增一族名）。
2. **历史改写**：不需；官方答案 = 追加 `unrelate`。
3. **检出**：默认只显当前态；历史由 `show`（未来 `--history`）看。
4. **check**：先补齐“全键目标存在”，再分批加“双侧声明冗余”与“part-of AND”两条 ⚠。
5. 至此本 issue 的机制设计无未定项；剩下的都是 `test`（量改动面 / 最小仓）与 `conclude`。
