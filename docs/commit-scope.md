# issue: commit-scope — 动作提交的内容边界：jj 收全副本、git 只收索引

- **domain**：design
- **关联**：relates-to `commit-order`（曾裁定不做 `--only`，改纪律）；relates-to `cli-necessity`（两后端写侧纪律相反）

## 背景

`commit-order` 收口时（commit `2e1828d`）的裁定是：**不做 `--only`/路径提交**，改用"先开单再落代码"的顺序纪律——因为 jj 后端 `jj commit` 提交整个工作副本，反过来做才需要挪文件。当时把 `--only` 当成"jj 无法稳定支持"而否掉了。

用户 2026-10-23 指出这个摩擦是**对称的**：使用 git 后端时，摩擦点是"必须先 `git add`"——kiln 的 git 后端 `Action()` 是裸 `git commit -m`，从不暂存；调用方忘了 add 就 `nothing to commit` 失败。也就是说两个后端把"选什么进这次动作"推给了相反方向的默认值：

- **jj**：全收。要排除无关文件，只能靠调用方自律或挪文件；
- **git**：只收索引。要纳入改动，只能靠调用方先 add。

本 issue 要回答的是：一个动作的提交内容边界由谁决定——后端默认、调用方显式路径，还是区域（worktree）结构？以及 `--only` 是否值得做、做了语义如何统一。

## 问题

1. **可行性**：两个后端是否都原生支持"只提交指定路径"？（最新实测见 collect 段——jj 有 `[FILESETS]...`，git 有 `git commit -- <paths>`。）
2. **语义统一**：`--only a.md b.md` 在两后端应产生一致的可观察结果——只有这两个文件进提交，其余改动原样保留（jj 留在 `@`；git 留在工作区/索引）。
3. **未跟踪文件**：git 上 `git commit -- u.md` 对未跟踪文件报 `pathspec 'u.md' did not match any file(s) known to git`；jj 自动跟踪所以没有这个问题。`--only` 在 git 上是否代 `git add`？代做的索引副作用要不要写进文档？
4. **静默空提交**：jj 对不存在/无改动的路径是 `Warning: …` + **exit 0 + 空提交**。`--only` 若照搬会放大 `empty-commit-hint` 刚提示过的问题，必须前置校验。
5. **接口成本**：`vcs.Repo.Action(message string)` 需要扩成携带路径集；两个后端实现与 `cli_test.go` 的 fakeRunner 都要改。值与不值？
6. **替代解是否足够**：默认隔离区（worktree 默认开、全副本=该 issue 的工作）、`target:` 声明 + `target-honesty` 审计、顺序纪律，能否替代 `--only`？

## 候选

- **A. 现状 + 纪律**（commit-order 的裁定）：零成本，但摩擦真实存在，用户已经指出对称的另一面；单独作为结论显得回避。
- **B. `--only <path>...`**：jj `jj commit -m msg <paths>`；git `git add -- <paths>` 后 `git commit -m msg -- <paths>`；前置校验路径存在且有改动，坏路径 exit 1 且不产生提交。默认（无 `--only`）行为不变。
- **C. 默认隔离区**：把 worktree 默认打开，让"全副本=该 issue 的工作"成为结构性事实；`--only` 降级为逃生口。与 B 可组合，但改了默认工作流，单独也解不掉"专属区里仍有无关改动"。
- **D. 声明式**：不限制提交内容，靠 `target:` trailer 声明 + `target-honesty` 的 check 审计"声明与 diff 不符"。把摩擦从"选择"变成"可审计的声明"，零接口改动，但只能事后发现、不能阻止混入。
- **E. 两步式 `kiln add` / staging 概念**：最像 git，但 jj 没有暂存区，语义会分裂；否。
- **F. 组合**：B + 文档化 C 的隔离区建议 + 沿用 empty-commit-hint 的提示。

## 验收

- **E1**：两后端 `kiln commit <issue> <action> --only a.md b.md` → 提交里只有这两个文件；其余改动原样保留（jj 在 `@`，git 在工作区/索引）。
- **E2**：git 后端对未跟踪路径可用（自动 `git add -- <列出的路径>` 或明确要求先 add，二选一并在文档写死）；索引副作用写进文档。
- **E3**：路径不存在或无改动 → exit 1，**不产生任何提交**（jj 不允许落到空提交）；错误信息点名坏路径。
- **E4**：无 `--only` 时行为与现在逐字节相同（既有测试守住）；`do` 是否接受 `--only` 要明确（建议拒收，`do` 只打印 trailer）。
- **E5**：`--only` 与 `commit` 其余 flag（`--domain`/`--title`/`--relates-to`/`--force` 等）组合正常；参数为 0 个路径时按 `--only` 缺参报错。
- **E6**：测试：两后端各有用例（fakeRunner 断言 argv、真仓/临时仓冒烟）、坏的路径用例、无 `--only` 回归；`go build`/`go vet`/`go test ./...` 全绿。

## 非目标

- 不做交互式 `split`、不做 hunk 级部分暂存；
- 不改 `Action` 的默认语义（无 `--only` = 今天的整副本/索引行为）；
- 不做逐动作 diff 审计——那是 `target-honesty` 的范围；
- 不改 `check` 规则。

## collect（2026-10-23）

两个后端各自在临时仓里实测。环境：jj 0.45.1、git（Windows）、kiln 临时二进制。

### jj 0.45.1

| 探针 | 命令 | 结果 |
| --- | --- | --- |
| help | `jj commit --help` | `Usage: jj commit [OPTIONS] [FILESETS]...`；"Put these paths in the current commit" |
| 只提交一个路径 | `jj commit -m "probe: a only" a.md`（`a.md`、`b.md` 均为新文件） | exit 0；`@-` 的 stat 只有 `a.md \| 1 +`；新 `@` 留 `A b.md` |
| 路径不存在 | `jj commit -m "probe: missing" nope.md` | **exit 0**；stderr `Warning: The given paths do not match any file: nope.md`；仍生成 `(empty)` 空提交 |
| 路径存在但无改动 | `jj commit -m "probe: unchanged keep" keep.md`（`keep.md` 已提交、无改动） | exit 0；生成空提交 |

### git

| 探针 | 命令 | 结果 |
| --- | --- | --- |
| 未跟踪路径 | `git commit -m "probe: untracked path" -- u.md`（`u.md` 未跟踪） | exit 1；`error: pathspec 'u.md' did not match any file(s) known to git` |
| 先 add 再路径提交 | `git add u.md` 后 `git commit -m "probe: u only" -- u.md` | exit 0；提交只含 `u.md`（`create mode 100644`）；`base.md` 的改动留在工作区（`M base.md`） |
| 与已暂存改动并存 | 先 `git add other.md`，再 `git commit -m "probe: base only" -- base.md` | exit 0；提交只含 `base.md`；`other.md` 仍是已暂存（`M  other.md`） |
| 路径无改动 | `git commit -m "probe: unchanged" -- base.md`（无改动） | exit 1；`no changes added to commit (use "git add" and/or "git commit -a")` |

### 含义

1. **两后端都原生支持路径限定提交**：jj 用 fileset，git 用 `-- <paths>`；jj 零额外步骤，git 对未跟踪文件需要 add（kiln 可代加，只加显式列出的路径）。
2. **jj 的坏路径是静默空提交**：`Warning` 只上 stderr、exit 0。`--only` 必须前置校验（路径存在 + 有改动），否则会把 `empty-commit-hint` 刚提示过的坑放大成常规事故。
3. **git 路径提交绕过索引**：提交内容 = 列出的路径的工作区内容；其余改动（含已暂存的其他文件）原样保留。`--only` 的 git 语义因此干净。
4. **接口成本小**：`vcs.Repo.Action(message string)` 只被 `cli.write()` 调用；扩成 `Action(message string, paths []string)` 需改 git/jj 两实现、fakeRunner 与调用点，改动面可控。
5. **与 `do` 的关系**：`do` 只打印 trailer、不提交，`--only` 对它没有意义，应当拒收。

## hypothesize（2026-10-23）

**裁定：做 `--only`（候选 B + F），默认行为不变。**

理由：摩擦是真实且对称的，而两个后端都原生支持路径限定；成本集中在一次接口扩参，不需要引入暂存区概念（候选 E 否）。`target-honesty`（D）是事后审计，与本 issue 互补而不是替代；默认隔离区（C）作为文档建议保留，不在此轮改默认工作流。

### 设计

- **命令面**：`kiln commit <issue> <action> --only <path>...`。路径为仓库相对路径（支持目录，交给后端 pathspec/fileset 处理）；`--only` 至少一个路径。`kiln do` 拒绝 `--only`（exit 2）。
- **接口**：`vcs.Repo.Action(message string, paths []string)`；`paths` 为空 = 今天的整副本（jj）/ 索引（git）行为，逐字节不变。
  - git：`paths` 非空 → 先 `git add -- <paths>`（只加列出的），再 `git commit -m msg -- <paths>`；
  - jj：`paths` 非空 → `jj commit --message <msg> -- <paths>`（`--` 分隔，避免路径被当成 revision）。
- **前置校验**（在 `Action` 之前、`empty-commit-hint` 提示之后）：每个路径必须存在，且这些路径整体有改动（jj：`jj diff --summary -- <paths>` 非空；git：`git status --porcelain -- <paths>` 非空）；坏路径 exit 1 并点名，**不产生提交**。
- **索引副作用**：git 上 `--only` 会先把列出的路径加入索引；若提交失败，这些路径保持已暂存。写进文档，不尝试回滚（回滚会误伤调用方原本的暂存意图）。
- **提示**：逻辑不变；`--only` 场景下"无改动"由前置校验拦截，提示只留给默认路径。

### 待 apply 时确认

- 路径是"仓库相对"还是"当前目录相对"（倾向仓库相对，与 `git`/`jj` 一致）；
- 目录路径是否允许（倾向允许，由后端展开）；
- `--only` 与 `--force`、`--domain` 等的组合顺序（`--only` 只影响提交内容，不影响 trailer）。

### 验收映射

E1/E2/E3 由前置校验 + 两后端实现覆盖；E4 由既有逐字节回归测试守住；E5/E6 由 fakeRunner argv 断言与临时仓冒烟覆盖。

## hypothesize（2026-10-23，第二回：jj 后端值不值）

用户提出：`--only` 会让命令更复杂，而 agent 的认知成本随命令数上升；应评估 jj 后端的好处，好处不大就考虑改回只支持 git。

### 实测 1：在 jj 默认的 colocated 仓里用 git 提交会打架

临时仓 `jj git init`（默认 colocate）后只走 git 命令：

```
echo one > a.md; git add -A; git commit -m "kiln-like: a"
```

结果：

- jj 导入 git 变化后，`jj log` 里多出一个**孤儿空提交**（`d77c7f72`，description 空，不在 `@` 链上）；
- `jj status` 仍报 `M a.md`——工作副本快照与 git HEAD 失配；
- 第二次 git commit 前 jj 打 `Reset the working copy parent to the new Git HEAD`，每次提交都留下空提交痕迹。

即：**jj 适配器的独有价值是"在 jj 管理工作副本的仓里保持工作副本一致"**；同一仓里用 git 后端提交，会和 jj 互相踩。

### 实测 2：jj 的其它好处都能在 git 上仿真

| jj 特性 | git 仿真 | 结论 |
| --- | --- | --- |
| 工作区快照（无需 add） | `git add -A` + commit | 语义等价 |
| 允许空动作 | `git commit --allow-empty`（实测 exit 0；不存在的路径仍 `pathspec` 报错 exit 1） | 等价 |
| `change_id` 稳定 ID | 代码 0 处引用 | 无收益 |
| op log / undo | 未暴露给 kiln | 无收益 |
| `jj workspace add` | `git worktree`（已有实现；worktree-* 系列已收口） | 等价 |

### 实测 3：体量

- jj 适配器 329 行 / 4 个测试；git 适配器 456 行 / 4 个测试。
- README + docs 里 `jj` 提及 436 处、21 个文件（多数是 design 历史文档）。
- `vcs: auto` 规则：`.jj` 优先，否则 `.git`。

### 认知成本的真实来源

命令面完全一致：agent 用 `kiln commit/path/status`，不需要知道后端名字。真正要多记的是**语义分歧**：写侧（快照 vs 索引）、空动作（允许 vs 失败）、历史可改写（jj 可 `describe/squash`）。统一这些分歧后，agent 只需记一套；删掉 jj 适配器主要省的是**维护面**，不是认知面。

### 选项

- **A. 维持双后端**（现状）：分歧暴露给 agent；cli-necessity 里"纯 jj 仓没有 `.git`"的论据已部分过时——jj 0.45 的 `jj git init` **默认 colocate**，非 colocate 需显式 `--no-colocate` 或配置。
- **B. git-only**：删 jj 适配器；本仓必须停止用 jj（导出后删 `.jj`），否则 jj 工作副本会与 git 提交打架（实测 1）。命令面不变，但 jj-only 工作流失去支持。
- **C. 双后端 + 语义统一**：把 git 后端的动作改成"快照整个区域 + `--allow-empty`"，与 jj 对齐；`--only` 两后端同语义。agent 只记一套，jj 适配器保留、只剩维护成本。
- **D. 显式 `vcs: git|jj`（auto 只认 git）**：能力保留但配置复杂化，不推荐。

### 结论与建议

- jj 的**独有**好处只有两条：**(1) colocated 仓里保持 jj 工作副本一致**（本仓正是）；**(2) 无 git 二进制的环境**（jj 不依赖 git）。其余都能仿真。
- 从"agent 认知成本"看，收益最大的是 **C**：命令面不变、语义统一、agent 不必在意后端；删 jj（B）省的是维护而不是认知，代价是本仓迁回纯 git。
- 若目标明确是"少一个后端、少一套文档"，B 可行；迁移步骤是 `jj git export` 后删除 `.jj`（本仓 git HEAD 已与 jj 同步，`git log` 完整），并接受 jj 用户不可用。
- 本 issue 不扩大范围：`--only` 仍按两后端设计；**语义统一（C）或删后端（B）应另开 issue 裁定**，本 issue 保留该评估作为依据。

## hypothesize（2026-10-23，第三回：统一语义，用户已裁定）

用户裁定（2026-10-23）：

1. **默认内容 = 区域快照**：无 `--only` 时提交当前工作区全部改动（改/删/未跟踪），不再依赖索引。git 后端由 kiln 执行 `git add -A` 后提交；jj 原生快照。
2. **空动作 = 允许**，保留"工作区无改动"的 stderr 提示（数据集：256 个动作提交里 7 个空提交，多为 conclude/design；提示措辞去掉"jj 后端"限定）。
3. **git 上 `--only` 列出的未跟踪文件由 kiln 代 `git add -- <列出的路径>`**——但明确这是**初步方案**，效果要靠 `test` 动作验证（尤其索引副作用与失败残留）。

统一后的语义（两后端应一致）：

| 维度 | 语义 |
| --- | --- |
| 默认（无 `--only`） | 区域快照：改/删/未跟踪全收；索引不再承载语义 |
| `--only p...` | 只提交列出的路径，其余原样保留 |
| 路径校验 | 每个路径必须"相对父提交有改动"（删除也算）；坏路径 exit 1 且不提交 |
| 未跟踪（git） | `--only` 时代 `git add -- <列出的路径>`，只加列出的 |
| 空动作 | 允许；工作区无改动时 stderr 一行提示，不拦截、不改退出码 |
| `do` | 拒收 `--only`（`do` 不提交） |
| 后端检测 | 不变：`.jj` 优先，否则 `.git` |

### 待 test 验证的点

- **T1** git：默认快照能收未跟踪与删除（`add -A` + commit）。
- **T2** git：空工作区 + `--allow-empty` → 空提交 exit 0，与 jj 行为一致。
- **T3** git：`--only u.md`（未跟踪）→ 代 add 后只提交 u.md，其他改动保留；观察索引副作用。
- **T4** git：`--only` 指向已删除的已跟踪文件 → 提交删除。
- **T5** 两后端：坏路径（不存在/无改动）→ exit 1、不产生提交（jj 重点：禁止静默空提交）。
- **T6** jj：`jj commit -m msg <paths>` 对删除路径与目录路径的行为。
- **T7** 端到端：改后的 kiln 二进制在临时 git 仓与临时 jj 仓各跑一遍上述场景。
- **风险**：`git add -A` 会把调用方手工暂存的半成品一并收编（需文档）；`--only` 代 add 成功后若提交失败，索引会残留列出的路径（不自动回滚，需文档）。


