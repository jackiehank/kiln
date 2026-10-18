# issue: silent-flag-drop — 未知 flag 与「规格里已定但未落地」的 `part-of` 被静默忽略

- **domain**：cli
- **关联**：derived-from `relation-vocabulary`、relates-to `target-honesty`

## 背景

`kiln do` 对不认识的输入**不报错、不提示、exit 0**。本仓实测（2026-10-17）：

| 输入 | 结果 |
| --- | --- |
| `kiln do dummy ask --title x --bogus 1` | 正常输出，`--bogus 1` 消失，exit 0 |
| `kiln do dummy ask --title x --part-of other --blocked-by other` | 正常输出，**两个键都不见了** |
| `kiln do dummy ask --title x --derived-from no-such-issue --spawns no-such-issue` | 原样写出（目标存在性交给 `check`，这条是设计如此） |
| `kiln do dummy ask --title x --blocks ""` | 空值被丢掉（与 `--refs ""` 是同一族问题） |

危险不在于"不知道就报错"，而在于**使用者以为自己声明了**。而且这里不只是"拼错 flag"，
是**规格与实现不一致，并且不一致被静默吞掉**：

- `relation-vocabulary` 定稿的词表是 **5 个写侧名字**：`derived-from` / **`part-of`** /
  `blocks` / `relates-to` / `supersedes`（`spawns` 随"取消反向名"一起不复存在）。
- 代码里能写的仍是 `internal/trailer/trailer.go` 的 `knownKeys`：`derived-from` / **`spawns`** /
  `relates-to` / `blocks` / `supersedes`。
- 两个集合的交集只有 4 个：**`part-of` 被定稿却写不进去（静默丢弃）**；`spawns` 被定稿取消却仍
  能写、且本仓正在用（`multi-project-isolation`、`sandbox-strategy`，以及 wsl-agent 刚写的那几条）。
- README 同时写着两件事：关系表里有 `spawns:`（README:133），已知边界里又写「`part-of`
  （分解）**尚未落地**」（README:281）——**读文档的人知道，写 flag 的人不知道**。

`check` 这边也不兜：它只校验 derived-from 的父存在、spawns 的子存在，**不校验 `blocks` /
`relates-to` / `supersedes` 的目标是否存在**，所以"关系写错了/丢了"在 `check` 里同样无声。

## 要回答的问题

1. **未知 flag**：报错退出（像 `git` 的 `unknown option`）、还是警告继续？是否存在"宽容未知
   flag"的正当用途（脚本向前兼容、把 `kiln do` 当转发层）？——需要用实际用法证伪或证实。
2. **规格已定但未落地的键**：给一句专门提示（"`part-of` 已设计、尚未实现，本次忽略"）？
   还是与未知 flag 同待遇？`-h` 要不要把"已知未实现"的名字列出来？
3. **词表与实现怎么收口**：实现 `part-of`（分解族）还是把规格改成"以实现为准"？——这属于
   `relation-types` / `relation-vocabulary` 的遗留，本 issue **只要求"不一致不能静默"**。
   注意 `spawns` 的处理更微妙：它已被词表取消但正在被使用，静默报错会让现有提交变得"非法"。
4. **关系目标不存在**：`do` 当场校验，还是维持"交给 `check`"？若维持，`check` 要不要把
   `blocks` / `relates-to` / `supersedes` 的存在性也补上（现在只查两族）？
5. **机器可读面**：`do` "丢掉了什么"要不要体现在输出里（面向脚本的 `--json` 或 stderr 摘要）？
   `do` 的 stdout 是要直接当提交信息用的，所以提示必须走 stderr 或单独一行。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 未知 flag 报错 | 未识别即退出并列出可用 flag | 若确有"宽容"用法会打断既有脚本；要先确认 |
| **B** 只警告 | stderr 一行 `ignored unknown flag --part-of` | 最轻；要保证不污染提交信息 |
| **C** 只针对关系键 | 把"已定稿但未实现"的名字点出来（`part-of`） | 窄，但正中文档与实现的落差 |
| **D** 维持 | 什么都不做 | 零成本；静默丢失继续发生 |

> 倾向（待验证）：**A（未知 flag 报错）+ C（已定稿未实现的名字单独提示）**，并把 `check` 的
> 关系目标校验补齐到 5 个键。原则与 `target-honesty` 同一条：**工具必须诚实说明它做了什么、
> 没做什么**。

## 非目标

- 不实现 `part-of` / `blocked-by` / `has-part` / `superseded-by`（`relation-types` 的遗留，另论）。
- 不改 `blocks` / `supersedes` 的渲染（那是 `relation-rendering`）。
- 不改关系词表本身（那是 `relation-vocabulary`）。

## 证据

- 上表四条命令与输出：本仓实测 2026-10-17（`kiln do`，未提交）。
- `internal/trailer/trailer.go`：`knownKeys` 只含 5 个关系键；未知键在 `Parse` 里被跳过。
- `internal/engine/engine.go` `checkIssue`：Rule 3/4 只覆盖 derived-from / spawns 的存在性。
- `README.md:133`（关系表含 `spawns:`）与 `README.md:281`（`part-of` 尚未落地）并存。
- `docs/relation-vocabulary.md` 结论 1 / 5：词表定稿 5 个名字、反向名取消。

## test：现状重测——大半已解决，剩三个面（2026-10-18）

> 本 issue 源于 2026-10-17；其后 `relations-impl` / `help-relation-flags` 已改变大部分前提。用新编译的 kiln 重测。

### 1. 已解决（原问题清单逐条核对）

| 原问题 | 现状 |
| --- | --- |
| `part-of` 定稿却写不进（静默丢弃） | ✅ `kiln do zz ask --part-of other` → 正常写出 `part-of: other` |
| 未知 flag 静默 | ✅ `kiln do zz ask --bogus 1` → `警告: 未知 flag --bogus…`（exit 0） |
| `check` 只查两族关系目标 | ✅ 改为**全键目标存在**（⚠） |
| 词表 5 名 vs 实现 5 键只交集 4 | ✅ trailer 解析改为**词表驱动**（9 名 + 别名） |

### 2. 仍未解决（实测）

| 输入 | 结果 |
| --- | --- |
| `kiln path <issue> --bogus` | **静默忽略**（无警告） |
| `kiln status --bogus` / `kiln list --bogus x` | **静默忽略** |
| 提交里带 `typo-key: i1` / `caused-byy: i1` | **静默忽略**：`path` 不显示关系，`check` 也不报 |
| `kiln do <i> ask --force`（非 commit） | ⚠，但**措辞误导**：`--force` 是**已知**的 commit flag，被当成“未知” |

### 3. 剩下的三个面

- **A 未知 trailer 键**（最危险：使用者以为声明了，但进不了模型）；
- **B 其它命令的未知 flag**（`path`/`status`/`list`/`show`/`graph`/`check`/`worktree`/`config`）；
- **C `--force` 在 `do` 下被误报为未知**（措辞问题）。

### 4. 结论

- 原 issue 的“词表 / 实现不一致”已消；剩下的是**“不识别的输入是否报”**的通用问题。
- 下一步：`hypothesize` 定三个面的处置（注意 A 的**假阳性**风险：正文里的 `中文: 东西` 不能当 trailer）。

## hypothesize：收口三个残留面（2026-10-18）

### 1. 面 C（最简单，先修）

- `--force` 是 `commit` 的**已知** flag；在 `do` 下不该被叫“未知”。
- **处置**：把 `force` 加入固定 flag（不再 ⚠）；帮助里已经把它写在“commit 额外 flag”。

### 2. 面 B：所有命令的未知 flag → ⚠

- 现状仅 `do`/`commit` 做（`relationsFromFlags`）。扩展到 `path` / `show` / `status` / `list` / `graph` / `check` / `worktree` / `config`。
- **做法**：每个命令声明自己的 flag 白名单；`parse` 后统一比对，不在白名单 → **⚠（不拦）**。
- 白名单（草案）：

| 命令 | 允许的 flag |
| --- | --- |
| do / commit | title, refs, target, scope, domain, tag, unrelate, force + 词表名 + 别名 |
| path | stat, shortstat |
| show | stat |
| status / check | json |
| list | json, domain, tag, action |
| graph | format |
| worktree | （子命令各异：remove 有 force） |
| config | —（无） |

### 3. 面 A：未知 trailer 键 → `check` ⚠（防假阳性）

- **难点**：`trailer.Parse` 逐行宽松扫描，正文里的 `中文: 东西` 会被当键。
- **处置**：只在 **trailer 块** 里判定。trailer 块 = 消息**最后一个空行之后**的段，且该段**至少含一个已知 trailer**（`issue:` / `action:`）；在该段内、键不属于 `{固定键 ∪ 词表名 ∪ 别名}` 的行 → ⚠ `未知 trailer 键: X`。
- **只在 `check` 报**（不拦提交）；`do`/`commit` 不需要（写入本来就是白名单驱动）。
- 效果：`typo-key: i1` / `caused-byy: i1` 被点名；正文里的 `注意: …` 不误报（不在 trailer 块 / 块内无已知 trailer）。

### 4. 不做

- 不实现新关系名（已落地）。
- 不改 `do` 的 stdout（提示一律走 **stderr**，不污染提交信息）。

### 验收

1. **C**：`kiln do X ask --force` 不再报“未知”；`kiln commit … --force` 正常。
2. **B**：`kiln path X --bogus` / `kiln status --bogus` → ⚠；合法 flag 不受影响。
3. **A**：临时仓提交带 `caused-byy:` / `typo-key:` → `kiln check` ⚠ 点名；正文含 `注意: xxx` 不误报。
4. `go build` / `go test ./...` 过。
