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

## hypothesize（续）：把「action 是原子动作」写进 README，并钉清「终点之后还能提交」

> 用户裁定：本轮**主要解决文档问题**：README 要写明 **action 是原子动作**（只有起点确定、终点负责标记完成，其余自由组合）；并且先**确认**终点动作之后还能不能继续提交。

### 1. 先确认事实（沙盒实测，不是推断）

临时 git 仓库（`/tmp/kiln-sandbox`，`worktree: off`，真跑 `kiln` 二进制）：

| # | 实验 | 结果 |
| --- | --- | --- |
| A | `ask → conclude` | `path` = 「已完成（当前 conclude）」；`check` = `✓ 完整`；`status` 打 `✓` |
| B | 在 A 之后 `kiln commit demo test` | **提交成功，没有拒绝**（确认行 `已提交 f51506a · test · demo`）；随即 `path` = 「进行中（当前 test）」、`check` = `⚠ 缺少终点动作（当前 test）`、`status` 的 `✓` 消失 |
| C | `collect → ask → conclude` | **`check` 判 `✓ 完整（collect, ask, conclude）`** —— 起点不在第一位也过 |
| D | `ask → conclude → apply` | `✓ 完整（当前 apply）` —— 可以有多个终点，完成看最后一个 |

**结论：能提交。** kiln 没有“关闭”这个状态——“完成”是 **“最后一个动作是终点”的函数**（`status` 的 `✓` 与 `check` 的第 2 条规则同一个判据：`isDone = 最后一个动作是终点`）。所以终点之后补动作**不是错误**，只是让这条 issue 回到进行中；想再收尾就再补一个终点动作。

另外 C 还泄露一件事：结构纪律是「**含**起点动作 + **以**终点动作收尾」——起点只要存在，不要求排第一（但如今 `ask` 是新建的唯一入口，实际路径总是它先）。

### 2. README 要改成什么样（§动作词表，草稿）

```md
**每个 action 是一条原子动作**——一次提问、一次采集、一次验证、一条结论，不是流水线上的一个阶段。

- **只有两端是结构**：每个 issue **至少一个起点动作**（默认 `ask`）、**最后一个动作必须是终点动作**（默认 `conclude` / `apply`）。
- **中间完全自由**：任意组合、任意次序、**可以重复**。没有“必须先 hypothesize 再 test”这类流水线要求。
- **终点之后还能继续提交**：kiln 没有“关闭”这个状态——**“完成”= 最后一个动作是终点**（`status` 的 `✓` 与 `check` 第 2 条同源）。
  所以 `ask → conclude → test` 之后这条 issue 会**回到进行中**，`check` 也会报 `缺少终点动作`：那是判定，不是错误。
- **`path` 里的 `1.` `2.` `3.` 是动作出现的次序**（按时间），是索引，**不是阶段号**：`1. collect 2. ask 3. conclude` 也合法。
```

保留原有的 “`apply` 与 `conclude` 的区别” 与 “词表每个仓库可以不同” 两句。

### 3. 顺手修一处**已被实现推翻的规格**

[mvp-plan](mvp-plan.md) §2.3 还写着：「同一 issue ID 下再出现 `ask`，实际属于另一个 issue（**该识别留待后续迭代**）」——但 `issue-uniqueness` 已把它定为**写侧拒绝**（除非该 issue 还没有起点动作：那是修复而不是分叉）。不改就是 spec 与代码相反。

### 4. 附带发现（同族，很小，建议一起做）

**`kiln commit` 失败时看不到原因。** 沙盒里忘 `git add` → `提交失败: git commit: exit status 1`，而 git 自己的解释（`On branch master / Changes not staged for commit / use "git add"`）**被吞了**。

诊断（已验证）：git 把这段写在 **stdout**，而 [`git.runIn`](../internal/vcs/git/git.go) 失败时**只取 stderr**，取空就退回 `err.Error()`，于是只剩 `exit status 1`。

修法（几行）：失败时按 **stderr → stdout → err.Error()** 三级兜底，且只取**第一行非空**（避免把大段输出塞进错误）；jj 侧对称处理。
→ 它与刚做的「`-h` 补提交范围」是同一件事的两半：**规矩说了，但犯规时的报错不说原因**。

### 5. 非目标

- 不改 `check` 的规则（“最后一个动作是终点”已够用，且它是 **warn 不阻断**）；**不加**“终点之后不许再有动作”的新规则——那会把实验 D 那种合法写法判死。
- 不改 `isDone` 判定、不改 `path` 的编号（索引有用）。
- 不动 `status` / `-h` 的结构：`-h` 已有 `action 词表默认: …`，且 help 带“不得引用文档”的测试约束；本轮只改 README 与 spec。

### 6. 落地清单（供 test）

| 位置 | 改动 |
| --- | --- |
| README §动作词表 | 按 §2 草稿重写（原子动作 + 两端是结构 + 中间自由 + 终点之后可继续 + 编号是索引） |
| [mvp-plan](mvp-plan.md) §2.3 | `ask` 那句改成“写侧拒绝 + 例外（补起点是修复）” |
| [`git`](../internal/vcs/git/git.go) / [`jj`](../internal/vcs/jj/jj.go) 的 `run` | 失败信息三级兜底（stderr → stdout → err.Error()），取首行非空 |
| 测试 | 文字改动无单测；适配器兜底加一条真仓测试（干净仓 `commit` 无暂存 → 错误里应含 git 的解释，而不只是 `exit status 1`） |

## hypothesize（再续）：起点要不要「固定在第一个」——先理理念，再提一个二选一

> 用户裁定：**原子 action 的设计是精妙的**（流程可自由组合）；但「起点是否必须第一个」值得继续探讨。本轮**只理思路、列选项、给建议**，不动代码。

### 1. 文档里的「理念」原话（先对齐口径）

**同一套文档里有两句互相拉着的话：**

- [evidence-fluidity](evidence-fluidity.md) 记下了用户给定的三条理念：① **issue 原子化**（复杂 issue 由不同 issue 以不同类型的关系组合表达）；② **action 原子化**——“一个 issue 的解决**没有固定流程**，从提出后可**按不同顺序**进行，甚至自定义 action”；③ 依据是 **Unix 哲学**，更根本的是相信“**复杂性 / 优秀产品是涌现出来的，不是设计出来的**”。
- [object-semantics](object-semantics.md)：**“issue 是原子的；组合靠关系，不靠 issue 内部再分线索。”**——每条原子 issue“各自过 `check`、各自有 domain、各自可再派生”。
- [mvp-plan](mvp-plan.md) §2.1（命名沿革）却说：issue 的**本质**是“**以 `ask` 开启**、经若干动作、**以终点动作收尾**”的一条**探究线**。

→ **张力就在②与 §2.1 之间**：前者是**集合**语义（顺序无关），后者是**叙事**语义（有开端与收束）。本条的真正问题因此可以写得更锋利：**“开启”到底是“位置”，还是“角色”？**

### 2. 数据模型怎么说（读代码得到的事实）

- `engine.Build` 从**每一个**动作累加 `Domain`（最后一个非空者胜）/ `Tags` / `DerivedFrom` / `Spawns` / `RelatesTo` / `Blocks` / `Supersedes`——**没有任何字段是“只从起点取”的**。
- `p.State()` = **最后一个**动作的 kind；`isDone` = 最后一个是不是终点。
- `check` 的两条结构规则：① 起点**存在**（Rule 1）；② **最后一个**动作是终点（Rule 2）。

→ 结构上的事实：**只有“尾部”参与判定（位置语义）；“头部”只在存在性里出现一次**。今天这个模型里，**终点是位置角色，起点只是存在角色**。

### 3. 这个不对称是偶然的吗？不是——它是「只追加」的推论

- “**最后一个**动作是终点”是**尾部性质**：不满足 → **追加一个终点**就修好了 → 可强制、可修复。
- “**第一个**动作是起点”是**头部性质**：不满足 → **无法修补**（写入路径只能往尾部追加；git 侧无法把提交插到最前，jj 侧要改写历史、等于换掉全部坐标）→ 只能**预防**，不能修复。

→ 于是关键裁决点浮出来，**二选一且不可兼得**：

> **要么**保留“孤儿 issue 可以补起点”的**修复路径**（现状：`ask` 追加在后面，位置不在第一）；
> **要么**坚持“起点必须在第一”，那就必须**放弃**这条修复路径——`check` 的 `缺少起点动作` 会变成**永久错误**，因为补救动作永远“位置不对”。

### 4. 实测：位置性质今天已经**事实上**成立

本仓 34 条 issue，**首动作 100% 是 `ask`**（`kiln check --json` 聚合）。原因在写侧入口：`ask` 是新建的唯一入口，且**已有起点的 issue 再 `ask` 会被拒绝**（`issue-uniqueness` 的落地）。

→ “起点在第一个”在**实践**中已由入口保证；“再加一条位置约束去强制它”是给一个已经发生的事付第二道手续。

### 5. 选项

| | 做法 | 代价 | 评价 |
| --- | --- | --- | --- |
| **A**（现状） | 起点只需**存在**（check Rule 1） | 无 | 与②理念一致；“位置”靠入口事实保证 |
| **B**（明说） | 文档 / spec 写清“**新建时第一条动作就是起点**；读侧只强制存在性” | 一句文档 | **推荐**：把已成立的事实写成口径，不新增约束 |
| **C**（读侧告警） | `check` 加 warn：起点不在第一个动作 | 新规则 + 测试 | **不推荐**：这个告警对历史**不可满足**（补了 `ask` 位置还是不对）→ 变成“修不好又一直响”的噪音 |
| **D**（读侧错误） | Rule 1 升级为“第一条必须是起点” | 新规则 + 判死历史 | **不推荐**：放弃修复路径；与②“涌现 / 无固定流程”冲突；本仓 0 例，收益为零 |
| **E**（角色可见） | 位置自由，但**把角色标出来**：`path` 给起点 / 终点加标记（如 `1. ask ← 起点`、`5. conclude ← 终点`） | 改 `path` 渲染 + 测试 | **推荐搭配 B**：把“叙事”从**位置推断**改成**显式标记**，正回应“编号暗示流水线”的原始困扰 |

### 6. 建议（待裁定）

**采 B + E。** B 让口径统一（**起点是角色，不是位置**；它总是第一条，因为它是唯一入口）；E 让输出自证（不再靠“1 就是开头、最后一个就是终点”去猜）。**C / D 出局，理由都是不可操作性**。

留给用户的两个问题：

1. 认可“**起点 = 角色，不绑位置**”这条口径吗？（它同时解释了为何终点可以有位置约束而起点不能。）
2. 若认可，E 的标记形态选哪种：只给**起点**打标、只给**终点**打标（终点其实已有 `✓` 与之对应）、还是两者都打？

### 7. 非目标

- 本轮不动代码、不改 `check`、不改 `path`（E 的形态定了再落地）。
- 不重提“第二个 `ask`”（已由 `issue-uniqueness` 定案：写侧拒绝）。
- 不改 `path` 的 `1..N` 编号本身（它是时间序索引，有用）。

## hypothesize（三续）：E 收窄为「只标起点」——终点行标记会被读成“关闭”

> 用户裁定：**认可 B**；但 E 有问题——“把终点标出来会不会被 agent 或人认为**结束了就不能再加 action**？”

### 1. 这个风险成立，而且是致命的

实测语义恰恰相反：**终点之后仍可提交**，只是会让 issue 回到进行中（见上一节实验 B）。一个标记若把人引向**相反**的理解，它就是负资产——比不标更坏。

### 2. 更根本的理由：“终点”这个角色**已经有地方表达**了

| 已有出口 | 它说了什么 |
| --- | --- |
| `path` 头部 | `已完成（当前 conclude）` / `进行中（当前 test）` |
| `check` | `✓ demo 完整（ask, conclude）` |
| `status` | `✓` 列（与 `check`、`isDone` 同源） |

行内再标一次是**重复**（本仓的规矩：同一个事实只留一处真相）。而反观**起点**：它的角色**没有任何地方表达**——除了“它在第一行”这个位置推断。所以两个角色里，**只有起点有标记价值**。

### 3. 收窄后的 E1

```
demo  进行中（当前 test）
  1. ask       32f8e79  开题：X 并发可能出错        ← 起点
  2. conclude  781abdb  结论：调锁粒度
  3. test      f51506a  事后补测
```

三条理由：① 不引入“关闭”暗示（没人会以为“起点”之后不能加动作）；② 它是唯一没被表达过的角色；③ 它直接指向“**位置 ≠ 角色**”——万一哪天首动作不是 `ask`，读者仍能一眼看出哪个是起点。

### 4. 被否掉的形态（过程记录）

| 形态 | 形态举例 | 为何否 |
| --- | --- | --- |
| E2 两个都标 | `5. conclude ← 终点` | 正是用户担心的误读；且与头部重复 |
| E2′ 措辞变体 | `5. conclude ← 收尾（可再续）` | 能消歧，但每行更宽、仍与头部重复——**若将来真需要终端标记**，选这个措辞，而不是“终点” |
| E3 只加页脚说明 | 无行标记，末尾一行“编号是索引，不是阶段” | 违反本仓自己的 legend 规矩（`worktree` 的 `■` legend 只在标记出现时打：“没有标记的 legend 就是噪声”）；且每次 `path` 都付一行 token |

### 5. 顺带指出的一个真相：真正会被读成“结束”的是 `path` **头部那个词**

`已完成（当前 conclude）`——“已完成”才是最强的关闭暗示。但它与 `status` 的 `✓`、`check` 的“完整”**同源**（都是“最后一个动作是终点”这条判定），四处一起改成本太高。
→ 本轮选择：**保留词形，让 B 负责解释**（README / spec 写明“完成是**判定**，不是锁”）。若将来仍有人误读，再单独开一条改状态词。

### 6. 定稿（供 test）

- **B**：README §动作词表 + [mvp-plan](mvp-plan.md) §2.3 写清四条（原子动作 / 两端是结构 / 中间自由 / **终点之后仍可继续**）+ `path` 编号是索引而非阶段。
- **E1**：`path` 只在**起点行**加 `← 起点`（含测试）。
- **不做**：终点行标记、页脚 legend、状态词改动。

## hypothesize（四续）：E 也去掉——**恒真的标记不配占一列**

> 用户裁定：“**甚至 E 里的起点也可以不标注**，因为 `ask` 排第一个，第一个作为起点很合理。”

### 1. 同意，而且理由能用本仓自己的规矩封口

- **“第一个是起点”不是误读，而是正确推断**：写侧入口保证（`ask` 是新建唯一入口，且已有起点的 issue 再 `ask` 被拒），实测 **34/34 条 issue 首动作都是 `ask`**。
- **原始困扰是另一个问题**：把 `1. 2. 3.` 读成“流水线阶段”（暗示“第 3 步之后必须第 4 步”），而那个由 **B** 解决（编号 = 时间序索引，动作自由组合）。E 想解决的是“位置能不能当角色读”——而这个问题上位置读法**本来就是对**的。
- **统一判据**：一个**恒真**（永远出现、永远相同）的标记，跟一个没有对应标记的页脚 legend 一样是噪声（本仓已有先例：`■` 的 legend 只在真有区时才打）。**E2（终点标记）因会误导被否；E1（起点标记）因恒真（34/34）被否——同一个判据的两次应用。**

### 2. 那“首动作不是起点”那个合法例外呢？

- 它只出现在**修复路径**（`collect → ask` 补起点），而那是 `issue-uniqueness` 特意保留的；`check` 会判 `✓ 完整`（结构纪律只要求**含**起点）。
- “为它读侧告警”（上上节的 C）已被否——**不可满足**：补了 `ask` 位置依旧不是第一，会变成“修不好又一直响”的噪告。
- 所以这个例外**按设计就是不可见的**，与“起点是角色不是位置”**不矛盾**：识别起点靠的是**公开词表**（`-h` / README 都写「默认起点：`ask`」）与 B 的措辞，不需要行内标记。

### 3. 定稿（供 test）

- **B 是唯一文档改动面**：README §动作词表 按四句重写（原子动作 / 两端是结构 / 中间自由 / **终点之后仍可继续**）+ [mvp-plan](mvp-plan.md) §2.3 的 `ask` 句改写 + 补一句“`path` 的 `1..N` 是动作次序的**索引**，不是阶段号”。
- **E 全部取消**（E1 / E2 / E2′ / E3 都否，不留任何行内标记）。
- **附带项仍在**（上一节 §4）：`git` / `jj` 的 `run` 失败时三级兜底（stderr → stdout → `err.Error()`，取首行非空）+ 一条真仓测试——它与本条同属“**犯规时的报错要说原因**”。
- 不改渲染、不改 `check`、不改状态词。

> 一句话：**B 已经把“编号是索引、起点是角色、终点是判定”三件事说完；再加标记就是在文档之外把同一件事再说一遍。**

## apply：落地（纯文档）——两个原子的理念成文、词表四句重写、spec 对齐

### 1. 改了哪里

| 位置 | 改动 |
| --- | --- |
| [README](../README.md) 新增 **`## 设计：两个原子 + 涌现`**（放在引言之后、安装之前） | 一张表说清两种拆法——**原子 issue**（复杂问题拆成多条独立 issue，靠类型化关系组合）/ **原子 action**（流程没有固定阶段，拆成可自由组合的原子动作）；三条“复杂性是涌现的”（粒度多细就写多少动作 / 分解多复杂就挂多少关系 / **新流程 = 新词表**）；以及“issue 之间靠关系组合、issue 之内靠动作组合”；末尾给出理念出处（Unix 哲学 + 涌现）与两份既有文档 |
| [README](../README.md) §动作词表 | “顺序不是约束”一段改为**四句**：每条 `action:` 是**原子动作** / 只有两端是结构 / 中间完全自由（可重复）/ **终点之后还能继续提交**（“完成”是判定不是锁）；并补一句 **`path` 的 `1. 2. 3.` 是次序索引、不是阶段号** |
| [mvp-plan](mvp-plan.md) §1 | 「设计原则」表新增三行：**原子 issue** / **原子 action** / **涌现优先**（并链到 `object-semantics`） |
| [mvp-plan](mvp-plan.md) §2.3 | “顺序不是约束”一段重写：结构纪律两条（起点**存在**、最后是终点）+ **终点之后可追加**（指向 §7 第 2 条）+ `path` 编号是索引；**并修掉一句被实现推翻的话**——旧文写“同一 issue ID 下再出现 `ask`…（该识别留待后续迭代）”，现改为**写侧拒绝**（除非该 issue 还没有起点动作，那是修复） |

### 2. 两个落点选择

- **理念放 README 靠前，而不是塞进词表那段**：词表那节只讲“有哪些词、怎么组合”，而“为什么这么组合”是全局设计，应当在读者上手前就出现。
- **spec 落在 §1 的「设计原则」表**：那本来就是“从讨论中提炼的原则”的正式位置（与无模式 / 不建文件 / LLM 先验优先并列），而不是新开一节。
- **用词统一**：用户原话是“原子事务”，文档统一用**原子动作**（与 `evidence-fluidity` 的“action 原子化”和词表一致），但“**复杂性是涌现的**”这条原话保留。

### 3. 验证

- 纯文档改动，**未碰代码**：`gofmt -l internal/` 空、`go vet ./...` 空、`go test ./...` **全绿**（含 `-h` 的两条 usage 测试——“help 不得引用文档”与 README 无关，不受影响）。
- 新增的两个链接目标（`docs/object-semantics.md`、`docs/evidence-fluidity.md`）**存在**且确实含对应论据（前者“issue 是原子的；组合靠关系”，后者四条理念原话）。
- `grep "留待后续迭代"` → **空**：该消失的只是那句被实现推翻的规格。`顺序不是约束` 本身**有意保留**（README 的上手示例注释、mvp-plan §2.3 的段首引语）——它仍然是正确的，错的是它后面那句。

### 4. 本轮不做

- **E 全部形态**（起点/终点行标记、页脚 legend）——上节已裁定取消。
- **附带项**（`git`/`jj` 的 `run` 失败时三级兜底）：它属“报错质量”而非“文档口径”，本轮用户只要文档改动——**仍排队**，待单独一轮或另开 issue。

