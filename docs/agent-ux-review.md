# issue: agent-ux-review — agent 对 kiln 的评价与改进建议

- **domain**：cli
- **关联**：relates-to `refs-flag-splitting` / `relation-rendering`

## 背景

一名 agent 在**一个完整会话**里用 kiln 做完了 6 条 issue（`qualified-issue-id`、`issue-uniqueness`、`status` 视图改造等），期间高频使用了 `path` / `show` / `status` / `check` / `list` / `do` / `commit` / `-h`。用户要求它做一次**使用回访**：哪些高效、哪些反直觉、哪些试了多次才成。

本 collect 就是那次回访的一手记录。**它同时是证据**：每条都带可复现的命令与观察，供后续 `hypothesize` 挑可落地项，而不是一篇印象派感想。

**覆盖面的诚实说明**：`graph` 只用过一次；`worktree new/list/remove` 因本仓 `worktree: off` **没有真实使用**（只读代码与测试）；`--json` 用在 `status` / `list` / `check` 三个命令上。所以下面的意见对这三块是"空白"，不是"满意"。

## collect：agent 视角的使用回访

> 来源类型：**本仓的一手使用记录**（命令 + 输出 + 本会话踩坑），非联网调研。凡"已修"指本会话内已落地并有提交；"仍在"指访谈当时仍然成立。

### 1. 高效且符合直觉的（8 条）

1. **`-h` 一屏就能开工。** 命令名是动词、语义直白；读完 `-h` 不必翻 README 就能发出第一条命令——学习成本属于最低一档。
2. **`kiln do` 只打印、不落库，是全工具最有价值的单点设计。** 每次 `commit` 前都先用 `do` 零副作用地试 flags：
   `kiln do qualified-issue-id ask --title "…" --domain cli --relates-to issue-uniqueness`
   把"生成"与"落库"分开，对 agent 尤其划算。
3. **`kiln path <issue>` 是主力命令。** 一次给到「序号 + 动作名 + hash + 标题 + 关系」，一次会话里用了几十次。
4. **`--stat` / `--shortstat` 委托 git 渲染**，输出与 `git show --stat` 同形——工具没有自创展示语言，学习成本为零。
5. **`check` 的输出形状好。** `✓ action-diff 完整（ask, collect, hypothesize, test, conclude）`：逐 issue 报动作序列，审计友好；warn 不影响退出码、`--json` 的 `ok` 与退出码同义，语义干净。
6. **状态全在提交信息里 → 没被工具锁住。** 全程用 `jj st`（干净）+ `check`（全绿）当"这轮做完了"的双信号，随时可 `jj log` / `git show` 交叉验证；没有只在自家 TUI 里看得见的黑盒。
7. **拒绝语给可复制配方。** worktree 那套提示（"开工前先 `kiln worktree new <issue>`"、cd 到该区）是全场最好的错误处理；后来写新拒绝语时直接照抄了这种风格（`提示: 要继续它，用 kiln commit <id> <action>`）。
8. **文档约定可推断。** 一 issue 一 doc、`背景 / 要回答的问题 / collect / hypothesize / test / 结论` 分节、末尾"来源"一行——看 3 个文件就推出来了，之后几十次写入都没犹豫过放哪。

### 2. 反直觉 / 需要试探或读源码的（10 条）

| # | 现象 | 证据 | 状态 |
| --- | --- | --- | --- |
| 1 | `status` 打 `<domain>/<id>`，但输入只吃裸 id | `kiln path design/worktree-mode` → 「未找到问题」（会话第二条命令就踩） | **已修**（`qualified-issue-id`：块头显示 domain、行内裸 id） |
| 2 | **`show <issue> <n>` 的 `n` 是位置序号，随动作增加而漂移，且选错不报错** | 想取 test，`kiln show qualified-issue-id 4` 返回的是第二条 `hypothesize`；只有 header `4/6 hypothesize` 提示了错误，而注意力在下面的 diffstat 上（本会话踩了 **2 次**） | 仍在 |
| 3 | **`--target` 是标量，`--refs` / `--tag` 是列表**，但 `-h` 把它们排在同一行 | 给 `--target` 传 9 个逗号分隔的文件名，落成**一个字符串**（`writeScalar` + `f.get("target")`）；而 `--refs a,b` 会拆成两项。读了源码才确定 | 仍在（与 `refs-flag-splitting` 同族） |
| 4 | **关系是单向可见的**：`relates-to` 文档写"双向"，渲染却只从声明侧出 | `kiln path qualified-issue-id` → `关系: relates-to issue-uniqueness, …`；`kiln path issue-uniqueness` → `关系: derived-from status-by-domain`（**完全看不到有人 relate 它**）→ 反查"谁和我相关"只能靠 `graph` 或人工搜 | 仍在 |
| 5 | **`--domain` 看起来是"issue 的属性"，实际是"最后写入者胜的可选 trailer"** | 读 `engine.go` 才知道：后续任一动作都能改写它，且可以没有。而它同时影响 `status` 的显示与 `list --domain=` 的筛选 | 语义没变（本轮已写进 `mvp-plan` §2.4） |
| 6 | **`commit` 没有确认行** | 提交成功只把消息原样打印一遍，没有 `已提交 <hash>`；为确认"到底提没提"额外跑了 `path` + `jj log` 数次。对比 `worktree new` 会打印 `worktree: <path>`——同一工具两种确认风格 | 仍在 |
| 7 | **`-h` 不提「提交范围 / 暂存」** | 最危险的语义（jj 下 `commit` 固定**整个工作副本**；git 下必须先 `git add`）只在 README 里。agent 的默认动作是先读 `-h`，这次恰好先读了 README 才没出事 | 仍在 |
| 8 | 小不对称：flag 是 `--tag`（单数）、trailer 是 `tags:`（复数）；历史里还有 `object:`（现役 `do` 不产出） | `object` 只从一条旧提交 + "object 已退役"的用法报错里知道——**报错处理得算好的那类** | 仍在（措辞/兼容性小事） |
| 9 | 动作词表**无序**，但 `path` 的 `1..N` 编号像在描述时间线 | 文档两处明说"顺序不是约束"，使用者仍会把它读成"流程第 3 步"。编号作为索引有用，同时暗示了一条不存在的流水线 | 设计取舍，供参考 |
| 10 | 过时措辞把人引向不存在的功能 | `警告: issue ID "…" 不符合 <namespace>/<slug> 约定`，而 `-h`/README 里 namespace 早已删掉 | **已修**（本会话顺手改成单段 kebab-case slug） |

### 3. 失败 / 需要多次尝试的清单

| 命令 | 结果 | 次数 |
| --- | --- | --- |
| `kiln path design/worktree-mode` | 未找到问题（假设了 namespace）→ 改裸 id 成功 | 2 |
| `kiln show worktree-mode 2 --stat` | 动作序号超出范围（猜 2 个动作，实际 10 个） | 2 |
| `kiln show qualified-issue-id 4 --stat` | **不报错**，静默给了另一条动作 | 1（靠 header 发现） |
| 查"有没有重名" | `check` 不查这个 → 自己拼 `list --json \| grep id \| uniq -d` | 1 |

### 4. 至今不确定的

- **`target:` 该装"一个文件"还是"一串文件"**：传了 9 个，落成 1 个字符串，**没有任何地方说这是错的**（`check` 不校验、渲染看不出）。将来 `target-honesty`（用 diff 反查 `target:` 声明是否属实）会直接撞上这个——它得先决定怎么切分。
- **`status --json` 的 `domains` 在被搬空的组上的历史语义**：本轮修掉了，但是**读测试才发现**曾经不直观。

### 5. 如果只改三件事（按高频用户收益排序）

1. **`commit` 加一行确认**：`已提交 830a0cd · test · qualified-issue-id`。零成本，直接消掉最多的"再跑一条命令确认"。
2. **让 `show` 的错位可见或可避**：允许按**动作名**选（`kiln show <issue> test` 取最后一次该动作？），或至少把 `4/6` 提到与标题同一行。序号漂移是设计选择，"选错不报错"不是。
3. **`-h` 补一行「提交范围」**：jj＝固定整个工作副本、git＝需先 `add`、`--force` 是什么。这是唯一"错了会污染历史"的语义，不该只藏在 README。

> 收敛成一个主题：**flag 与输出的"形状"应当自证**——标量 vs 列表、可选 vs 必填、可变 vs 不可变、序号是否稳定。这四类信息里，目前只有"必填"在 `-h` 上写清楚了，其余都得读源码或试错。

> 来源：**本仓的一手使用记录**（agent 在 `qualified-issue-id` / `issue-uniqueness` / `status` 展示改造等 6 条 issue 上的完整会话：命令、输出、失败次数）；其中「已修」两项分别落在 `qualified-issue-id`（`status` 画法）与 `issue-uniqueness`（警告措辞与写侧归一化）的提交里。相关既有追踪：`refs-flag-splitting`（flag 形态，本条 issue 自身尚无 doc）、[relation-rendering](relation-rendering.md)（关系渲染不一致）、[target-honesty](target-honesty.md)（`target:` 声明与实际 diff）、[issue-list-scaling](issue-list-scaling.md)（全量输出与无索引）。

## hypothesize：① `commit` 确认行 ② `-h` 补提交范围 ③ `show` 按动作名（取名×）

> 用户裁定：① `commit` 加确认信息；② `-h` 补提交范围；③ `show` 按动作名展示“确实有必要”，但**先把“同名有好几个”定清楚**。

### 0. 先摆数据：同名动作是常态，不是边角

本仓实测（`kiln check --json` 按动作聚合）：**33 条 issue 里有 8 条带重复动作**；单种动作重复最多的是 `cli-necessity`（17 个动作）——**同一个动作出现 8 次**；`worktree-mode` 6 次、`worktree-hint` 5 次、`relation-vocabulary` 4 次。

→ 所以 `show <issue> test` **必须**定义多匹配语义。“唯一匹配才接受”会让它在最常用的 issue 上直接不可用。

### 1. ① `commit` 加确认行

- **位置**：**stdout**，紧跟在消息之后。理由：`commit` 的输出是**结果**而不是提示（与 `worktree new` 打印的 `worktree: <path>` 同侧），不适用“提示只进 stderr”那条规矩。
- **形态**：`已提交 <hash> · <action> · <issue>`（例：`已提交 830a0cd · test · qualified-issue-id`）。
- **一个待选：带不带 hash。** 倾向 **带**（B）：确认行若不指认对象，就只说“成功了”；而同一个工具的 `worktree new` 给出的是**可复制的值**。代价量过：`vcs.Repo.Action(message) error` 要改成回传新 commit id——git 侧 = 提交后 `rev-parse HEAD`；jj 侧 = 刚落的动作就是 `@-` 的 commit id（多跑一条命令）；另外要改两个后端 + 测试假件，全是机械改动。**零代价的退路是 A**（`已提交 · test · qualified-issue-id`，不动接口），且 A **不封死** B：行格式预留可选字段即可。
- **不改**：仍然打印完整消息（`do` / `commit` 输出同形是现状优势，别动）；不加 `--quiet` 之类开关。

### 2. ② `-h` 补一行「提交范围」

加在命令表下方，与既有的 `action 词表默认: …` / `worktree 默认: …` 同形：

```
提交范围: git 后端只提交已暂存的改动（kiln 不碰 index，先 git add）；
           jj 后端直接固定整个工作副本（无暂存区，无需 add）
```

- **两条现有测试的约束**：① `TestUsageTextCitesNoRepositoryDocument` 禁止 help 里出现 `docs/` 与 `.md`——所以这里只能写语义、不能指文档；② `TestUsageTextAlignsDescriptions` 只算命令表三列的描述列宽，新增段落不影响它。
- 顺带把 `--force`（`worktree: on` 时的一次性豁免）写进同一行——它也是“提交规则”的一部分，而现在同样只在 README 里。

### 3. ③ `show` 按动作名——**取最近一次，并在有歧义时出声**

| | 语义 | 代价 |
| --- | --- | --- |
| **A（推荐）** | `show <issue> <action>` = 该动作**最后一次**出现；**匹配到多个 → stderr 提示**`提示: 有 6 个 test，显示最后一个（2/6）；要看别的用序号`；**没有匹配 → 报错并列出该 issue 的动作**（序号 + 动作名 + 标题） | 小，且与现状默认同构 |
| B | 唯一匹配才接受，多个就报错列候选 | 按 §0 的数据，它在最常用的 issue 上**不可用**；等于把选择推回给使用者 |
| C | 支持 `<action>:<k>`（如 `test:2`） | 新语法要学；与序号并存 = 两套定位方式 |
| D | 只保留序号，但把 `4/6` 写得更显眼 | 不解决“我要看最近那个 test”——而那正是真实诉求 |

选 A 的五条理由：

1. **与现状同构**：`show <issue>` 不给参数就是“**最后一个动作**”；A 只是把它推广为“最后一个 X”。
2. **有官方先例**：git 的 `:/<text>` 就是“**the youngest matching commit**”（git-rev-parse(1) 原文）——按内容找、默认取最近一次，是版本控制里已被接受的手感。
3. **不静默**（这才是本条的要害）：歧义时提示候选数，且 header 本来就有 `· 4/6 hypothesize ·`，所以“取到的是哪一个”永远看得见。这恰好补上访谈里那个漏洞——“序号取错时**只有 header 提示，而我不看 header**”：换成语义寻址后，**动作名本身就是自证**（你要 `test` 却拿到 `hypothesize`，一眼就错）。
4. **序号仍在一等公民**：要精确定位第 k 个，今天就有 `n`，不需要新语法（所以 C 出局）。
5. **输入类型可判**：动作名永远不会是纯数字（词表可配，但 `ValidIssueID`/词表已限定为字母词），所以“纯数字 = 序号、词表成员 = 动作名”不存在歧义解析。

**非目标**：不改 `path` 的编号（它是索引，有用）；不给 `show` 加 `--first` / `--all` / `--list`（真要再说，别一开始就把面铺大）。

### 4. 不在本轮的三条（仍在 collect 里排队）

`--target` 的标量 vs 列表、flag “形状”自证（可选/必填/可变）、关系单向可见——三条都比这三条贵（前两条要动 `parse` / `trailer` 的语义，第三条动渲染层），且与既有 `refs-flag-splitting` / `relation-rendering` 重叠，留给那两条 issue。

### 5. 落地清单（供 test）

| 位置 | 改动 |
| --- | --- |
| `cli.write` | `commit` 成功后打确认行（stdout；hash 走待选 A/B） |
| `vcs.Repo.Action` + `git` / `jj` 适配器（仅待选 B） | 回传新 commit id |
| `usageText` | 新增「提交范围」段（避开 `docs/` / `.md`） |
| `cli.show` | `pos[1]` 先判：纯数字 = 序号（现状行为不变）；词表成员 = 按名取**最后一次**；无匹配 = 报错 + 列候选；多匹配 = stderr 提示 |
| 测试 | 确认行；`-h` 新段（不破两个既有 usage 测试）；`show` 的四种情形（唯一 / 多 / 无 / 序号仍准） |

> 验证方式：单测 + 真仓跑 `show worktree-mode test`（该 issue 有 6 个 `test`，正好压测“取最近一次 + 提示候选数”）与 `show cli-necessity test`（8 个同名）。

## test：落地——确认行带 hash、`show` 按动作名、`-h` 补提交范围

### 1. 代码

| 位置 | 改动 |
| --- | --- |
| [`vcs.Repo.Action`](../internal/vcs/vcs.go) | `Action(message) error` → `Action(message) (string, error)`：回传新提交 id，形状与 `Log` 一致 |
| [`git`](../internal/vcs/git/git.go) / [`jj`](../internal/vcs/jj/jj.go) 适配器 | git＝提交后 `rev-parse HEAD`；jj＝`@-` 的 `commit_id`（`jj commit` 会把工作副提交定型并开一个新的 `@`，所以刚落的动作就是 `@-`——与 `Log` 的 revset 排除 `@` 同源） |
| `cli.write` | 提交成功后追加一行确认（stdout） |
| `cli.show` | 选择器三分支：纯数字＝序号（行为不变）、词表成员＝按名取**最后一次**（多个→stderr 提示）、其余→报错；**无匹配→exit 1 并列出该 issue 的动作**（`ordinalList`） |
| `usageFooter` / `usageCommands` | 新增「提交范围」段；`show` 行改为 `[n\|action]` |

### 2. 一个实现时冒出来的风险（并已堵上）

“读回新提交 id”是**附赠**动作，而提交已经落下。如果读 id 失败就返回错误，调用方会看到 `提交失败` 但历史里已经多了一条提交——**把成功报成失败**。所以两个后端都改成：**读不到 id 就返回空字符串、不报错**，确认行降级为不带 hash（`已提交 · test · issue`）。

### 3. 真机行为（`show` 与 `-h`）

```
$ kiln show worktree-mode test
提示: 有 6 个 test，显示最后一个（9/10）；要看别的用序号
worktree-mode · 9/10 test · f57c709 · 术语统一：用户可见名词全部改为 worktree / main

$ kiln show cli-necessity test        # 8 个同名
提示: 有 8 个 test，显示最后一个（17/17）；要看别的用序号
cli-necessity · 17/17 test · 69d7ec1 · …

$ kiln show worktree-adopt test       # 这个 action 不存在 → exit 1
没有 "test" 动作: worktree-adopt（它的动作: 1.ask 2.collect 3.conclude）

$ kiln show issue-uniqueness nope     # 既不是序号也不是动作名 → exit 2
无法识别的选择器: "nope"（用序号 1..5，或动作名: ask, hypothesize, collect, test, conclude, apply）

$ kiln -h | grep 提交范围
提交范围: git 后端只提交已暂存的改动（kiln 不碰 index，先 git add）| jj 后端固定整个工作副本（无暂存区，无需 add）
```

`commit` 的确认行由**本次 `test` 提交自身**验证：形态为 `已提交 <hash> · <action> · <issue>`，<hash> 是 7 字符短号（与 `path` 里显示的一致）。

### 4. 验证

- `gofmt -l internal/` → 空；`go vet ./...` → 空；`go test ./...` → **全绿**。
- 新增单测 5 个：`TestCommitConfirmsWhatItWrote`；`TestShowSelectsByActionName`（唯一匹配**不**出声）；`TestShowTakesTheLastOfRepeatedActions`（3 个 test → `4/4 test` + 提示 `有 3 个 test`）；`TestShowListsActionsWhenTheNameMatchesNone`（exit 1 + `1.ask 2.collect`）；`TestShowRejectsAnUnknownSelector`（exit 2）。既有 `TestShowDefaultsToLastAction` / `TestShowSelectsOrdinal` / `TestShowRejectsOutOfRangeOrdinal` **行为未变**。
- 两条 usage 测试（列宽对齐、**不得引用 `docs/` 或 `.md`**）继续通过：所以新增段只写语义、不指文档。

### 5. 被现实修正的一条 hypothesize 假设

原计划“顺带把 `--force` 写进提交范围那一行”——实际上 `-h` 的 footer **早就有** `commit 额外 flag: --force（一次性豁免：worktree=on 时放行当次在 main 提交）`，所以本轮只需补“提交范围”本身，**不重复**。

### 6. 修订（用户反馈）：同名提示要“写具体”

第一版提示是：

```
提示: 有 6 个 test，显示最后一个（9/10）；要看别的用序号      ← 被打回
```

用户意见：**“要看别的用序号”等于把活推回给使用者**（还得自己去 `path` 里数第几个才是 test）。修订为：**列出所有同名序号 + 给一条能直接粘的命令**。

```
提示: worktree-mode 有 6 个 test（序号 4, 5, 6, 7, 8, 9）；已显示 9/10（最后一个）
  看别的：kiln show worktree-mode 4
```

三点信息全在一行里：**匹配了几个 / 它们是哪几个 / 已显示哪个**；第二行是 recipe（缩进行格式沿用 `refuseInMain` 里“提示 + 缩进配方”的写法）。

本仓两个真实例子（序号**不是连着的**才看得出这条提示的价值）：

```
$ kiln show worktree-mode test
提示: worktree-mode 有 6 个 test（序号 4, 5, 6, 7, 8, 9）；已显示 9/10（最后一个）
  看别的：kiln show worktree-mode 4

$ kiln show cli-necessity test
提示: cli-necessity 有 8 个 test（序号 6, 7, 8, 9, 14, 15, 16, 17）；已显示 17/17（最后一个）
  看别的：kiln show cli-necessity 6
```

（`cli-necessity` 的 8 个 test 落在 `6,7,8,9` 与 `14,15,16,17` 两段——光说“有 8 个”使用者猜不出地址，列出来才是可操作的。）

- 实现：新增 `matchingOrdinals(p, kind)`（返回 1-based 位置数组）与 `ordinalNumbers(ns)`（渲染成 `4, 5, 6`）；单测 `TestShowTakesTheLastOfRepeatedActions` 改断言新文案 + 断言 recipe 存在。
- 唯一匹配仍然**不出声**（`show worktree-adopt collect` → 只有内容，无提示）。

