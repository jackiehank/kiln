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
