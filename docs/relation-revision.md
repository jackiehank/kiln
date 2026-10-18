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
