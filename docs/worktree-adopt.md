# issue: worktree-adopt — 要不要做 `worktree new --adopt`（把 main 的未提交改动搬进新区）

- **domain**：design
- **关联**：derived-from `cli-necessity`

## 背景

**它是什么**：`cli-necessity` 的 U 方案里留的一条候选——**建区时把 main（主干）当前的未提交改动一并带进新区**：

> 更好：**`kiln workspace new <issue> --adopt`** 把主干**当前未提交改动**带进新专用区（git：stash → 在 worktree 里 apply；jj：把主干的 `@` squash / rebase 过去）。——`cli-necessity.md` §U 方案

**摩擦现场**（护栏挡下之后没有出路）：护栏只负责拒绝，**不负责迁移**——

> **`--adopt` 未实现**（§7 待定项）："已在主干改了文件才想提交"目前只有护栏报错 + 提示，**没有把改动迁进专用区的命令**。——`cli-necessity.md` §test

**边界早就钉死了**：`worktree-include` 反复声明两者**不是一件事**——`.worktreeinclude` 管**被忽略的本地文件**（层①），`--adopt` 管**未提交改动**；调研里也没有任何工具把两者合成一个机制。

**它现在悬空**：`worktree-include` 结论 §遗留 3 与 `worktree-mode` 取舍 5 / 遗留 5 都把它记成「独立缺口 / 另开 issue」，但那两个 issue 都已 conclude，**没有一条未关闭的 issue 承载它**（只有 README 的「已知边界」点了名）。

## 要回答的问题

> **问题 1 是唯一的前置问题：这件事值不值得做。** 后面的每一问都只在问题 1 答"做"之后才需要答。

### 1. **有没有必要做？**（第一个问题，先答它）

- **真场景是什么**：人在 main 改了几个文件、之后才想起"这该在专用区干"？还是 agent 起手就在 main 动了手、被护栏挡下？两者频率各多少？（本仓自举实测里，**agent 确实踩过**：直接从 main 开始改，被护栏挡下后才建区。）
- **现成的替代路径够不够用**（三条，全都不需要新机制）：
  1. **当次 `--force`**：改动**留在 main**，就在 main 提交（`--force` 已是一次性豁免）；
  2. **手工两步**：`git stash` → `kiln worktree new <issue>` → 进区 `git stash pop`（jj 侧无暂存区，"改动挂在 `@` 上"，更简单：直接 `jj workspace add` 就好）；
  3. **重做 / 复制粘贴**：改动量小时，进区重写一遍。
- **不做的代价能压到多低**：护栏的那句拒绝语**补一行指引**（"要保留改动就先 `stash`，去区里再 `pop`；或当次 `--force`"）——零机制、零新面。问题变成：**这一行提示够不够，还是必须由 kiln 代跑？**
- **与 kiln 的立场冲突吗**：U 方案的立场是「**编排在外、护栏在结构上**」，`worktree-mode` 也立过「提示只说**项目无关的事实**，怎么做留给项目与人」。`--adopt` 引的是**"迁移"这种编排语义**（"替你把本地状态搬到别处"）——它比 `.worktreeinclude` 的"播种"更进一步（播种是"把清单里的东西复制过去"，adopt 是"**搬走**并让 main 变干净"）。**这一步越不越界，是问题 1 的另一半。**
- **判据（建议）**：若"`--force` + `stash` 指引"能覆盖绝大多数场景，且手工两步**不易错**，则**不做**；只有当"手工两步太容易踩（顺序、遗忘、跨后端差异）**且**这类场景高频"时，才值得做。

### 若做，再答（以下全部从属于问题 1）

2. **搬什么**：工作树改动 / 暂存区改动 / **未跟踪文件**（`git stash -u` 级别）？（**被忽略的文件不归它**——那是 `.worktreeinclude` 的层①，别混。）
3. **方向与前提**：`adopt` 是"**迁走**"（main 留下干净树）还是"**复制**"（main 照旧）？要不要**前置要求 main 干净**（否则迁走的语义不清）？
4. **冲突策略**：目标区已存在同名改动时怎么办？`.worktreeinclude` 的先例是"**已存在则跳过、不覆盖**"——adopt 沿用，还是直接覆盖 / 报错？
5. **后端两条路**：git = `stash`（含 `-u`）→ 区内 `apply`；jj = 把 main 的 `@`（改动天然挂在上面）rebase / squash 过去。**两后端语义要对齐，机制必然不同**（与 `worktree-include` 的"枚举交 git、复制逻辑共用"是同一种分派思路）。
6. **形态**：`worktree new --adopt` 的一枚 flag，还是独立子命令 `worktree adopt`？与非 adopt 的 `new` 是否要有行为差异（复用已存在的区怎么办）？
7. **回滚**：搬完 main 变干净，**这是不可逆的**（stash 已消费）——要不要两步式（先建区、再显式 adopt），或给 `--dry-run`？

## 候选

### 对问题 1

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A 不做** | 护栏拒绝语**补一行指引**（`stash` → 建区 → `pop`；或当次 `--force`），机制零新增 | 把"迁移"留给 git / jj 自己的原语；代价是人工两步，顺序反了要自救 |
| **B 做 `--adopt`** | 建区时把 main 未提交改动搬进区 | 新机制 + 两后端各一套实现；**引入"迁移"编排语义**，与"编排在外"有张力；冲突 / 回滚语义都要定 |
| **C 只做提示里的可复制命令** | 拒绝语里直接打印**一条现成的、可复制粘贴**的 recipe（按后端各写一条），kiln **不代跑** | 半机制：零实现风险、零新面，控制权仍在人手里；代价是提示变长、且要维护两条命令 |

## 非目标

- **不重复 `.worktreeinclude`**：被忽略的本地文件由播种负责，本 issue 只管**未提交改动**（两者严格两件事）。
- **不改护栏本身**：`worktree` 开关、`--force` 一次性豁免的语义不动。
- 不新增后端、不动区布局（`.kiln/ws/<issue>`）。
- **本轮只开题**：不改代码。

## 备注

出处分布（供核实）：`cli-necessity.md` §U 方案（首次提出）｜§待定 ｜§test（落地时明确不做）｜§非目标；`worktree-include.md` §要回答的问题 4 / 待定 / collect 收敛表 / 结论 / 遗留 3；`worktree-mode.md` 取舍 5 / 遗留 5；`README.md`「已知边界」。

## collect：别的工具怎么处理「把未提交改动带进新工作区」

> 一轮调研。**以本地权威源 + 真机实测为骨干**（git / jj 自带的用法说明与真实行为），厂商文档用来回答"这件事到底有没有人做"。目的不是选方案，而是**给问题 1 备料**。

### 1. 先说结论：厂商没把这个做成功能

| 工具 | 有没有"带未提交改动进新 worktree"的能力 |
| --- | --- |
| **git** | **没有**。`git worktree add` 的选项只有 `-f / --detach / --checkout / --lock / --orphan / (-b\|-B) / --reason / --track / --guess-remote / --relative-paths`，**没有任何"携带改动"的开关**；它对脏工作区也**不拒绝**，只是**当没看见**（新区是目标提交的干净检出）。man page 无相关指引。 |
| **jj** | **能，但它是"选父提交"的副作用**：`jj workspace add -r <revset>`。**不给 `-r`** 时，新工作区的 working-copy commit 建在当前 `@` 的**父**上（共享父 → **不含**你的改动）；**给 `-r @`** 就"如同执行了 `jj new @`" → **含**你的改动。文档**不把它称作**"携带改动"。 |
| **VS Code** | 文档明说 worktree **不含**主工作区的未提交改动。 |
| **Claude Code** | worktree 是 fresh checkout；只有 `.worktreeinclude`（管**被忽略**的文件），未记载携带未提交改动。 |
| **JetBrains** | New Worktree 对话框只有 From branch，无此选项。 |
| **Codex** | 调研**未找到**官方 worktree 文档（not found）。 |
| **第三方 worktree 管理器** | 调研里**唯一一例**：`gwt switch --carry`——**已逐源核对**，见 §4。 |

> 一句话：**"把未提交改动带进新工作区"在主流工具里不是一项功能**——共识是"你自己的本地状态自己管"（stash / apply / 重做）。唯一把它做成开关的是一支第三方脚本，而且叫 **`--carry`**，不叫 `--adopt`。

### 2. git 侧：机制与坑（真机实测）

**官方语义先垫底**：`refs/` 下的引用**跨 worktree 共享**（`git worktree` man 的 REFS 节；例外只有 `refs/bisect` / `refs/worktree` / `refs/rewritten`）。`refs/stash` 属 `refs/` ⇒ **stash 是仓库级的**，任何链接工作区都看得见、都能 `pop`。

实测（一步不差；主干一个已跟踪改动 + 一个未跟踪文件）：

| # | 动作 | 结果 |
| --- | --- | --- |
| 1 | `git worktree add <path> -b feat` | **建区成功、不报错**；新区内容 =**旧内容**、**没有**那个未跟踪文件 → 改动**一点没跟过去** |
| 2 | 主干 `git stash -u` | 主干**干净** |
| 3 | 在**区内** `git stash list` | **看得见**那条 stash（印证"仓库级共享"） |
| 4 | 区内 `git stash pop` | 已跟踪改动 **+ 未跟踪文件**都落进区里；主干仍干净 |

**坑**：未跟踪要 `-u`、被忽略的还要 `-a`（**默认只存已跟踪的改动**）；顺序不能反（先 `pop` 回主干再建区 = 没搬）；`pop` 冲突时会保留 stash 但要人收拾；这条路**"迁走"得很彻底**（主干变干净），代价是**不可逆**（stash 已消费）。

### 3. jj 侧：天然能"注入"，但"迁走"踩中流体历史的坑（真机实测）

jj 把"未提交改动"就当成**一条提交**（工作副本 `@`），于是：

| # | 动作 | 结果 |
| --- | --- | --- |
| 1 | 主干有改动，`jj workspace add --name w <path>`（**不加 `-r`**） | 新区与主干 `@` **共享父** → 新区是**旧内容**（改动留在主干） |
| 2 | 改加 `-r @` | 新区是**改后内容** ✓ —— 且**主干也仍然持有那份改动**：`-r @` 是**复制 / 注入**，不是搬迁 |
| 3 | 主干想变干净——**错法**：`jj restore base.txt` | 主干 `@` 被**改写** → jj 打出 `Rebased 1 descendant commits.`，**新区的工作副本随即 stale**；`jj workspace update-stale` 之后真相是：**区里的改动没了**（回到旧内容）。**磁盘上那个"改后内容"是 stale 造成的假象。** |
| 4 | 主干想变干净——**对法**：`jj edit @-` | **不改写任何提交**（无 rebase 提示）；主干工作副本退回旧内容；**区里的改后内容原封不动** |

→ **jj 侧正解 = `-r @`（把改动交给新区的父提交）+ 主干 `jj edit @-`（把工作副本挪开）**：两步都**原生、都不改写历史**。一旦用 `restore` 这类"改内容"的动作，就会顺着 jj 的自动 rebase **把新区连坐**。

> 这正面呼应 `evidence-fluidity`：**在 jj 上，"把改动搬走"的难点不在搬运，而在"别顺手改写历史"。**

### 4. 唯一的先例（已逐源核对）：`gwt switch --carry`

第三方 bash 脚本 `gwt`（worktree 管理器）的 `switch` 有 `--carry`：

> `--carry  Copy working state (staged, unstaged, untracked) to new worktree (requires --create)`

实现（读源码）：`git diff --cached --binary`（暂存）+ `git diff --binary`（未暂存）→ 新区里 `git apply --cached` / `git apply`；未跟踪文件按 **`git ls-files --others --exclude-standard`** 枚举再 `cp -a`。

三点可借鉴：
1. **名字是 `--carry`**（携带），不是 `--adopt`；
2. **是"复制"不是"搬迁"**（源工作区不动）；
3. **不含被忽略的文件**（枚举用了 `--exclude-standard`）——正好落在与 `.worktreeinclude` 的**分界线**上（层①归播种，它只管"未提交"）。

### 5. 给问题 1 的三点材料

1. **没有厂商先例**：主流工具一律"不代管未提交状态"，唯一例外是一支第三方脚本 → "做"这件事**没有行业共识可当理由**（也就没有"别人都有、我们也得有"的压力）。
2. **机制代价不对称**：jj 侧是 `-r @` + `jj edit @-` 两枚**原生**命令（kiln 只是转发，**谈不上编排**）；git 侧却要 kiln 自己串 `stash -u` → `pop`——**那才是真正的"编排"**，与 U 方案的"编排在外"有张力。→ 若做，**两侧的"越界程度"并不相同**。
3. **名字的先验在别处**：调研里 `adopt` 在其它工具一律指"**收编既有对象**"（GNU Stow `--adopt`、grove / vde-worktree 的 `wt adopt`），**没有**"携带未提交改动"的含义——若真做，`--carry`（或 `--with-changes`）比 `--adopt` 更贴既有语汇。

> 来源。**本地权威源 + 实测**：`git worktree add -h` / `git stash -h` / `jj workspace add --help`（本机 jj 0.45.1）；两个后端的临时仓实测（见 §2 / §3 的表格）。
> **厂商文档**：[git-worktree（REFS：`refs/` 跨 worktree 共享）](https://git-scm.com/docs/git-worktree) ｜ [git-stash（`-u` / `-a`）](https://git-scm.com/docs/git-stash) ｜ [Claude Code：worktrees](https://code.claude.com/docs/en/worktrees) ｜ [JetBrains：Use Git worktrees](https://www.jetbrains.com/help/idea/use-git-worktrees.html)。
> **第三方（已逐源核对）**：[gwt（`switch --carry` 的源码与实现）](https://raw.githubusercontent.com/omerhadari/gwt/main/gwt)。
> **二手 / 未逐字复核**（标注类型）：VS Code"worktree 不含未提交改动"那句话的原文（[delegate-two-tasks](https://code.visualstudio.com/docs/agents/guides/delegate-two-tasks)）、Codex 侧"not found"——均由调研代理报告。

## 结论（已定）

**定了：不做（方案 A）。`--adopt` 不做——真正的痛点是「agent 得知约束的时机太晚」，不是「改动搬不动」。接力的是只给提示与配方、零机制的方案 C，落在新 issue `worktree-hint`。**

### 1. 逐条裁定（对 ask 的 7 个问题）

| # | 问题 | 结论 |
| --- | --- | --- |
| **1** | **有没有必要做**（唯一前置） | **不做**。理由见 §2：三条零机制出路够用，且 collect 证明**厂商没把这个做成功能**（唯一一例是第三方 `gwt switch --carry`） |
| 2–7 | 搬什么 / 方向与前提 / 冲突 / 两后端 / 形态 / 回滚 | **随问题 1 一并作废**——不做，就不必定这些。但 §3 把实测结论留档，因为它们**直接变成"配方"** |

### 2. 为什么不做（用户重新框定 + collect 材料）

**痛点的根因是"发现时机"，不是"搬运机制"**：

> 用户：*"这个问题的起因是用户设置了 worktree 为 on 时，agent 不开 worktree 直接编辑，提交时才发现。"*

- 即：**agent 动手前不知道约束**（或不知道），等到 `kiln commit` 被护栏拒绝，工作**已经做在 main 上**了。
- "把改动搬进区"只是**事后补救**的一种，而它要引一整套**迁移 + 冲突 + 回滚**语义（问题 2–7）——**为一个"本不该发生"的场景加机制**。
- 对照之下，**三条零机制出路**已在：① 当次 `--force`；② 手工 `stash` 两步；③ 改得少就重做。加之 collect 的结论：**主流工具一律不代管未提交状态**——"做"没有行业先例可当理由。
- → 值得做的是**把约束提前说清**（提示 / 配方），而不是代跑迁移。

### 3. 留档：实测出来的两条"配方"

不做 `--adopt` ≠ 这些白测了——它们正是"提示"要说清的东西：

| 后端 | 把 main 的未提交改动搬到新区 | **要避开的错法** |
| --- | --- | --- |
| **git** | `git stash -u`（未跟踪也要）→ `kiln worktree new <issue>` → 进区 `git stash pop` | **先建区再想搬**——新区是**干净检出**，改动会**晾在 main**；而 stash 一旦 pop 出去就**不可逆** |
| **jj** | `kiln worktree new <issue>`（底层 `-r @`）→ 主干 `jj edit @-` | **`jj restore`**：会改写 main 的 `@` → **级联 rebase 把新区连坐**，而且现场用 **stale 的假象**骗你（磁盘上看着还在，`update-stale` 之后已没了） |

两条都是**后端级**配方（不是项目级做法），与 `worktree-mode` test（三续）立的「工具中立」不冲突。

### 4. 接力：新 issue `worktree-hint`

"提示 / 配方"交给 [docs/worktree-hint.md](worktree-hint.md)（`derived-from worktree-adopt`）：**第一问就是这句提示落在哪个调用点**，并带上四条硬约束（只进 stderr / exit code 不变 / 按后端分派 / 诚实标注边界）。

### 5. README 同步

`README.md`「已知边界」那条原写作「新区的未提交改动**不会**被搬过去（`--adopt` 未做）」——**读起来像 TODO**。已改成"**刻意为之，不是待办**"，并给出两条手工出路。
