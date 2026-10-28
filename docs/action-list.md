# issue: action-list — `kiln action list`：列出生效的 action 词表

- **domain**：cli
- **关联**：relates-to `action-view`（命名与形态已 conclude）；relates-to `action-log`（同组的事件视图）

## 背景

`action-view` 已 conclude：`action` 组下，`action list` 列词表、`action log` 列动作。action 词表可由仓库根 `.kilnconfig.jsonc` 的 `actions` 覆盖；现在没有专门的“列出当前可用 action”的命令，用户只能读配置文件、看 `-h`（它写死默认词表），或触发 `do` 的“非法 action”错误。

本 issue 只做 `action list`，不碰 `action log`。

## 要做的

```text
kiln action list [--json]
```

- 输出**生效**的词表：`names`、`start`、`terminal`；
- 文本：一行一个 action，标注 `start` / `terminal`；
- `--json`：一份文档，形状与 `.kilnconfig.jsonc` 的 `actions` 组对齐，外加 `source` / `config`；
- 不读 git 历史、不需要仓库；仓外/未初始化时用默认词表，`source=default`；
- 与 `do` / `commit` 共用同一个 `ActionSet`，列出的就是写侧接受的集合。

## 要回答的问题

1. 文本的标记格式：列对齐，`start` / `terminal` 用词还是符号？要不要表头/legend？
2. `--json` 的精确形状：`actions: {names,start,terminal}` 还是平铺？`source` / `config` 如何省略？
3. `source=file|default` 怎么判：`Config.Path` 是否等于“文件存在”？要不要给 `Config` 加字段？
4. 仓外、配置缺失、配置解析失败时分别输出什么？`action list` 是否也要在配置错误时失败？
5. `-h` 现在写死默认词表；本 issue 要不要顺手把帮助里的 action 词表改成生效词表，还是留给后续？
6. 测试、README、`-h`、i18n（zh/en）要覆盖哪些面？

## 非目标

- 不做 `action log`；
- 不做 `action show` / `action search`；
- 不写/生成配置文件（`config path` 已有 seed 行为）；
- 不新增模型字段或校验规则；
- 不改 `ActionSet` 的语义。

## collect（2026-10-28）：现在自定义 action 的实现情况

> 只收事实与缺口，不选文本/JSON 的具体形状；留给 hypothesize。

### 1. 配置面

| 事实 | 位置 |
| --- | --- |
| 配置文件是仓库根 `.kilnconfig.jsonc`，JSONC（注释可写、无尾逗号） | [.kilnconfig.jsonc](../.kilnconfig.jsonc) |
| `actions` 组字段：`names` / `start` / `terminal`；`vcs`、`worktree` 与它平级 | [config.go:30-38](../internal/config/config.go#L30-L38) |
| 缺 `start` 取 `names[0]`；缺 `terminal` 取 `names[last]`；`names` 为空则整组回退内置默认 | [config.go:108-121](../internal/config/config.go#L108-L121) |
| `NewActionSet` 规范化（trim/lower）、去重校验；start/terminal 必须在集合内 | [model.go:56-90](../internal/model/model.go#L56-L90) |
| 缺文件不是错误：`Read` 返回内置默认；文件存在但解析失败则报错 | [config.go:65-103](../internal/config/config.go#L65-L103) |
| 仓外 `Read` 也返回默认词表；`Config.Path` 为空 | [config.go:65-70](../internal/config/config.go#L65-L70) |
| `config.Path` 从 cwd 向上找，命中文件或最近仓库根；文件不存在时仍返回候选路径 | [config.go:141-171](../internal/config/config.go#L141-L171) |
| `config.Ensure` 会在文件缺失时写入一份默认配置；`config path` 用它 | [config.go:176-191](../internal/config/config.go#L176-L191)、[cli.go:1183-1209](../internal/cli/cli.go#L1183-L1209) |

### 2. 生效词表怎么进命令

- [Run](../internal/cli/cli.go#L420-L465)：先 `config.Read(".")`；成功则把 `cfg.Actions` 放进 `env.actions`；失败则在 dispatch 前直接 `配置错误` exit 1（`-h` 走默认关系回退，但 action 文案仍写死）。
- `do` / `commit`：`e.actions.IsValid(action)`；不合法时报 `非法 action: ...（可选: <names>）`（[cli.go:779-782](../internal/cli/cli.go#L779-L782)）。
- `list --action`：同样校验，错误里列 `e.actions.Names()`（[cli.go:2410-2412](../internal/cli/cli.go#L2410-L2412)）。
- `show <issue> <action>`：动作名选择器也走 `IsValid`（[cli.go:1436](../internal/cli/cli.go#L1436)）；非法名最终报“无法识别的选择器”，并列出可控动作名。
- `check`：用 `actions.Start()` / `IsTerminal()` 判起点/终点（[engine.go:223-241](../internal/engine/engine.go#L223-L241)）；没有“当前 action 名是否合法”的规则。
- `status` / `graph` / `path` / `list`：只消费已解析的 action kind，不直接列词表。

### 3. 用户现在怎么“看到” action

| 入口 | 是否反映自定义词表 | 问题 |
| --- | --- | --- |
| `kiln -h` 的 `action 词表默认:` 行 | **否** | 写死默认 `ask | hypothesize | collect | test | conclude | apply`（[cli.go:37](../internal/cli/cli.go#L37)） |
| `do` / `commit` 的非法 action 错误 | 是 | 只有触发错误才看得到；不是可发现的查询 |
| `list --action` 的非法 action 错误 | 是 | 同上 |
| `show` 的动作名选择错误 | 是（列出该 issue 的动作） | 不是全局词表 |
| `config path` | 只给路径 | 不列内容；还会在缺失时写默认文件 |
| `action list` | — | **不存在** |

结论：**自定义词表缺少一个可发现、可脚本化的读取入口**；`-h` 在自定义仓里还会给出错误信息（写死默认）。

### 4. 数据模型与实现缺口

- `model.ActionSet` 有 `Names` / `Start` / `IsValid` / `IsTerminal`，**没有** `TerminalNames()`；列 terminal 需要遍历 `Names()` + `IsTerminal`，或新增一个只读方法（实现选择）。
- `config.Config` 有 `Path`、`Actions`，**没有** `Source`；而 `Read` 在文件缺失时也会把 `cfg.Path` 设为候选路径（[config.go:67-75](../internal/config/config.go#L67-L75)），所以“`Path` 非空”不等于“文件存在”。要输出 `source=file|default`，要么在 `Read` 里加字段，要么在命令里 `os.Stat(cfg.Path)`。
- `env` 只有 `actions`，没有 `configPath` / `configSource`；`action list` 要么：
  - 在 `Run` 里把 `cfg.Path`/source 存进 `env`；或
  - 命令内再 `config.Read(".")`（重复读文件/可能重复报错）；或
  - 只用 `e.actions` 输出词表，`source` 由 `os.Stat(config.Path("."))` 另判。
- 仓外：`config.Read` 返回默认词表、`Path` 为空；`action list` 不读 VCS，应该能直接输出默认词表 + `source=default`。
- `config.Ensure` 会写文件；`action list` **不能**用它（只读命令不应产生副作用）。
- `usageText` 只接收 `RelationSet`，不接收 `ActionSet`（[cli.go:101-121](../internal/cli/cli.go#L101-L121)）；要让 `-h` 显示生效词表，需要改签名/多个调用点，牵动 help 测试。`action list` 可以先把“可发现入口”补上，`-h` 是否同步另议。

### 5. 测试现状

- `internal/config/config_test.go` 已覆盖：仓外默认、解析自定义 `actions`、start/terminal 派生、未知键、非法 start、缺失文件、向上查找、seed 默认（[config_test.go:64-110](../internal/config/config_test.go#L64-L110)）。
- `internal/model/model_test.go` 覆盖 `NewActionSet` 的拒绝/规范化/默认（[model_test.go:5-61](../internal/model/model_test.go#L5-L61)）。
- CLI 侧只有 `do` 非法 action、`list --action` 校验、`show` 选择器等用例；**没有** `action list` / “列出词表”的测试。
- i18n：帮助里的默认 action 行在 [catalog.go:223](../internal/i18n/catalog.go#L223) 写死；新命令需要 zh/en 文案。
- `usageText` 测试只断言命令表齐全与对齐（[cli_test.go:345-368](../internal/cli/cli_test.go#L345-L368)），不校验 action 词表是否来自配置。

### 6. 留给 hypothesize 的问题

1. 文本格式：一行一个 action；`start` / `terminal` 用列、括号还是符号？要不要表头/legend？如何保证 `| wc -l` 干净？
2. `--json` 形状：`actions: {names,start,terminal}` + `source`/`config`，还是平铺？`source=default` 时 `config` 省略还是给空？
3. `source` 判定：加 `Config.Source`，还是命令内 stat？`Config.Path` 的“候选路径”语义要不要顺手改清楚？
4. 仓外/缺配置/配置错误：`action list` 是否也在配置错误时 exit 1？仓外是否输出默认词表并 exit 0？
5. `-h` 写死默认词表：本 issue 是否顺手同步？如果同步，`usageText` 签名与 help 测试怎么改；如果不同步，是否在 README/`action list -h` 里说明“`-h` 显示默认，生效词表用 `action list`”？
6. 实现落点：新增 `action` 命令组 + `list` 子命令；DTO；`env` 是否加 `configPath`/`source`；测试与 i18n 的最小集。

## hypothesize（2026-10-28）：文本一行一个 + 英文 `start`/`terminal`；JSON 镜像配置；`source` 由 `Config` 提供

> 结论先行：文本保持“一行一个 action + 可选的 `start` / `terminal` 列”，不加表头/legend，保证 `| wc -l` 干净；`--json` 的 `actions` 组与 `.kilnconfig.jsonc` 同形，外加 `source` / `config`；`source` 不靠命令内 `stat`，而是在 `config.Read` 里加一个字段；`action list` 不读历史、不 seed、仓外可用。`-h` 不改成“生效词表”（它写的是“默认”），只加一行指到 `action list`。

### 1. 文本格式

```text
ask           start
hypothesize
collect
test
conclude      terminal
apply         terminal
```

- 一行一个 action，顺序 = 配置 `names` 的顺序；
- `start` / `terminal` 用英文列，与配置/JSON 字段同词；两者都成立时写 `start, terminal`；
- 列宽按 `displayWidth` 对齐；没有标记的行不补尾随空格；
- 不加表头、不加 legend；`kiln action list | wc -l` 就是 action 数；
- 不在文本里输出 `source` / `config`：文本是“给人扫一眼的词表”，来源信息交给 `--json` 或 `config path`。

为什么用英文标记而不是“起点/终点”：action 名、配置键、JSON 字段都是英文；文本里再用一套中文标记，会让 `action list` 与配置文件对不上。中文解释放在 help/README 即可。

### 2. `--json` 形状

```json
{
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  "source": "file",
  "config": "C:\\Users\\jacki\\kiln\\.kilnconfig.jsonc"
}
```

- `actions` 与 `.kilnconfig.jsonc` 的 `actions` 组**同形**：`names` / `start` / `terminal`；
- `source` 恒显：`file`（文件存在且读取成功）或 `default`（缺文件 / 仓外 / 未自定义）；
- `config` 只在 `source=file` 时出现；`default` 时省略（缺省即省略）；
- 顶层永远是一个对象；`--json` 只此一种文档，不做 `--jsonl`；
- `actions.names` 保持配置顺序；`terminal` 由 `Names()` + `IsTerminal` 派生，顺序同 `names`。

### 3. `source` 判定：给 `config.Config` 加字段，不在命令里 stat

- `config.Config` 增加 `Source string`（`file` / `default`），由 `Read` 在成功读完文件后设 `file`；缺文件/仓外保持 `default`；
- `config.Path` 的语义不变（“找到的候选路径”），不拿它当“文件存在”的判据；
- `env` 增加 `configPath` / `configSource`，`Run` 从 `cfg` 填入；`action list` 直接使用，不重复 `config.Read`；
- 测试里手工构造 `env` 时这两个字段可能为空：`action list` 把空 `configSource` 当 `default`，空 `configPath` 不输出 `config`；
- `action list` **不调用 `config.Ensure`**：只读命令不能写文件；
- 配置解析失败时，`Run` 已在 dispatch 前 `配置错误` exit 1；`action list` 不需要自己吞错并显示默认词表（静默回退会掩盖配置错误）。

### 4. 仓外 / 缺文件 / 自定义

| 场景 | 输出 |
| --- | --- |
| 仓外（无 `.git` / `.jj`） | 默认词表；`source=default`；无 `config`；exit 0 |
| 仓内但没有 `.kilnconfig.jsonc` | 默认词表；`source=default`；无 `config`；不创建文件 |
| 仓内有配置文件，`actions` 完整 | 文件里的词表；`source=file`；`config` = 文件路径 |
| 仓内有配置文件，但没有 `actions` 组 | 默认词表；`source=file`（文件确实被读了）；`config` = 文件路径 |
| 配置文件解析失败 | `Run` 报配置错误，exit 1；`action list` 不输出 |

“文件存在但 `actions` 省略”时 `source=file` 是刻意的：source 说的是“读的是文件还是内置默认”，不是“actions 组是否被自定义”。如果以后需要区分，再加字段。

### 5. `-h` 怎么处理

- `kiln -h` 的 `action 词表默认: ...` 行本来就写着“默认”，不是“生效”；本 issue **不**把 `usageText` 改成接收 `ActionSet`（那会牵动签名、所有调用点与 help 测试）；
- 在该行末尾补一句：`（生效词表见 kiln action list）`，让自定义仓的用户知道去哪看；
- `usageCommands` 增加 `kiln action list [--json]` 一行；
- `kiln action` 不带子命令 → 打印 `action` 组用法并 exit 2（与 `worktree` 一致）；`action list -h` 走组 help，具体在 apply 定；
- `action list` 的 `--json` 开关值、额外位置参数按现有 parser 规则拒绝（exit 2）。

### 6. 实现落点

| 文件 | 改动 |
| --- | --- |
| `internal/config/config.go` | `Config` 加 `Source`；`Read` 成功读文件后设 `file`，缺文件/仓外保持 `default`；补测试 |
| `internal/cli/cli.go` | `Run` 支持 `action`；`env` 加 `configPath` / `configSource`；新增 `action` 分派与 `actionList`；DTO；`usageCommands` 与 `usageOtherFlags` 行；`action` 组用法 |
| `internal/i18n/catalog.go` | 新命令描述、组用法、`action list` 的 help 行；英文对应 |
| `README.md` / `README.en.md` | 命令表加 `action list` |
| `docs/machine-readable-output.md` | 补 `action list --json` 的小节（一份对象；`actions` / `source` / `config`） |
| `internal/cli/cli_test.go` | 文本/JSON/仓外/缺文件/自定义/不创建文件/非法参数/help |

实现顺序建议：先 `config.Source` + 测试，再 CLI 命令 + 测试，最后文档。

### 7. 测试清单

1. **文本**：自定义词表 `ask/probe/done`（start=ask，terminal=done）；`ask` 标 start，`done` 标 terminal，`probe` 无标记；行数 = 3；CJK action 名的对齐（若允许）。
2. **start+terminal 同一 action**：临时 ActionSet 自定义 `ask` 同时 start/terminal，文本写 `start, terminal`。
3. **JSON 文件源**：临时仓 `.kilnconfig.jsonc` 自定义；`source=file`，`config` 为绝对路径，`actions` 同形；golden 逐字节。
4. **JSON 默认源**：仓外/无文件；`source=default`，无 `config`；golden。
5. **`actions` 省略**：文件只有其他键；`source=file`，词表默认。
6. **不创建文件**：仓内无配置，跑 `action list` 后 `os.Stat(.kilnconfig.jsonc)` 仍不存在。
7. **错误**：配置解析失败 → `Run` exit 1；`action list --json=x` / 多余位置参数 → exit 2。
8. **help**：`usageText` 含 `kiln action list` 与“生效词表见”提示；`action` 无子命令 exit 2。
9. **不读历史**：`action list` 在 `e.repo == nil` 的 env 上也能跑；不调用 backend。
10. **真仓冒烟**：本仓 `action list` / `--json`；临时自定义仓各一次。

### 8. 风险与证伪

- **英文标记**：如果中文用户觉得 `start`/`terminal` 突兀，可改成 i18n 的“起点/终点”；但那会让文本与配置键不同词，默认先按英文。
- **`source=file` 但 `actions` 省略**：有人会认为应该是 `default`；本方案选择“source 指读取来源”，若实践反对再改。
- **`-h` 仍不显示生效词表**：如果用户只看 `-h` 仍被默认列表误导，下一步再改 `usageText` 接收 `ActionSet`；本 issue 先用一行提示兜底。
- **JSON 形状**：如果消费者更想要平铺 `names/start/terminal`，改 DTO 即可；本方案选镜像配置，便于与文件对照。
- **配置错误静默回退**：不做；否则 `action list` 会掩盖坏配置，与 `Read` 的严格性冲突。

### 9. 下一步

- 若认可：本 issue 可 `conclude` 后进入 `apply`；实现顺序按 §6。
- 若对英文标记/`source` 语义有异议，先改这两处，再 conclude。
- `action log` 仍在自己的 issue 里等实现，不受本 issue 影响。

## conclude（2026-10-28）：action list 定稿——文本一行一个；JSON 镜像配置；`config` 用当前 OS 原生路径

### 0. 裁决

| 项 | 决定 |
| --- | --- |
| 命令 | `kiln action list [--json]` |
| 文本 | 一行一个 action；`start` / `terminal` 英文列；无表头 / legend |
| JSON | 一份对象；`actions: {names,start,terminal}` + `source` + `config?` |
| `source` | `file` / `default`；由 `config.Config.Source` 提供，不靠命令内 `stat` |
| `config` | **当前 OS 的本地绝对路径**；Windows 用 `\`，POSIX 用 `/`；不强制 `ToSlash` |
| 仓外 / 缺文件 | 默认词表，`source=default`，无 `config`，exit 0 |
| 配置错误 | `Run` 先报错 exit 1；`action list` 不静默回退 |
| 副作用 | 不读历史、不开后端、不 `config.Ensure`、不创建配置文件 |
| `-h` | 默认词表行末尾加“生效词表见 `kiln action list`”；不改 `usageText` 签名 |

### 1. 文本定稿

```text
ask           start
hypothesize
collect
test
conclude      terminal
apply         terminal
```

- 顺序 = 配置 `names`；`start` / `terminal` 同时成立写 `start, terminal`；
- 按 `displayWidth` 对齐；无标记行不补尾随空格；无表头 / legend；
- `kiln action list | wc -l` 等于 action 数；
- 文本不输出 `config` 路径；路径只在 `--json` 与 `config path` 里出现。

### 2. JSON 与路径口径（本轮新增的硬要求）

```json
{
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  "source": "file",
  "config": "C:\\Users\\jacki\\kiln\\.kilnconfig.jsonc"
}
```

- `config` 必须是**当前操作系统可直接访问的本地绝对路径**：
  - Windows：`C:\Users\jacki\kiln\.kilnconfig.jsonc`（JSON 里反斜杠转义为 `\\`）；
  - Linux / macOS：`/home/jacki/kiln/.kilnconfig.jsonc`。
- 实现直接使用 `config.Path`（`filepath.Abs` / `filepath.Join` 已产出 OS 原生分隔符）；**不要**为了好看调用 `filepath.ToSlash`，也不要把路径写死成 `/` 或 `\`。
- JSON 字符串中的 `\\` 是编码结果；消费方解码后的字符串就是原生路径，可直接 `os.Stat` / 打开 / 传给子进程。文档要说明“用解码后的值，不要照抄 JSON 源码里的转义”。
- `source=default` 时省略 `config`；`source=file` 时 `config` 为绝对路径。
- 测试不要硬编码 `C:\` 或 `/`：用 `filepath.Join(root, ".kilnconfig.jsonc")` 构造期望值，再 `os.Stat(doc.Config)` 验证可访问；这样 Windows 与 POSIX CI 同一份测试都能过。
- 文档示例分平台写两条（Windows / POSIX），不要把某一条当成唯一契约。

### 3. `source` / 仓外 / 配置错误

- `config.Config` 加 `Source string`：成功读文件后 `file`；缺文件 / 仓外保持 `default`；
- `Config.Path` 仍是“候选路径”，不拿它当“文件存在”的判据；
- `env` 加 `configPath` / `configSource`，`Run` 填入；测试手工 env 为空时按 `default` 处理，空 path 不输出 `config`；
- 仓外：默认词表、`source=default`、无 `config`、exit 0；
- 仓内缺文件：同上，且**不创建** `.kilnconfig.jsonc`；
- 文件存在但 `actions` 省略：`source=file`、`config` 有值、词表默认；
- 配置解析失败：`Run` 在 dispatch 前 exit 1，`action list` 不输出、不回退默认。

### 4. `-h`

- `usageCommands` 增加 `kiln action list [--json]`；
- `usageOtherFlags` 的 `action 词表默认:` 行末尾加“（生效词表见 `kiln action list`）”；
- 不改 `usageText` 签名，不把 `-h` 改成生效词表；`-h` 行明确写“默认”，`action list` 才是生效视图；
- `kiln action` 无子命令 → 组用法 exit 2；`action list` 的额外参数 / `--json=x` 按现有 parser 规则 exit 2；
- `action` 组后续可加 `log`，本 issue 不实现。

### 5. 实现与测试

| 文件 | 改动 |
| --- | --- |
| `internal/config/config.go` | `Config.Source`；`Read` 设置 `file`/`default`；测试 |
| `internal/cli/cli.go` | `Run` 支持 `action`；`env` 加 config 字段；`actionList` + DTO；文本/JSON；help 行 |
| `internal/i18n/catalog.go` | 命令描述、组用法、默认词表提示；zh/en |
| `README.md` / `README.en.md` | 命令表加 `action list` |
| `docs/machine-readable-output.md` | `action list --json` 小节；分平台路径示例 |
| `internal/cli/cli_test.go` | 文本 / JSON / 仓外 / 缺文件 / 自定义 / 不创建文件 / 路径可访问 / 参数 / help |

必测：

1. 自定义词表文本：`ask/probe/done`，start/terminal 标记正确，行数 = 3；
2. start+terminal 同一 action → `start, terminal`；
3. JSON 文件源：`source=file`、`config` 等于 `filepath.Join(root, ".kilnconfig.jsonc")`、`os.Stat` 成功；golden 不含硬编码分隔符；
4. JSON 默认源：`source=default`、无 `config`；
5. 文件存在但 `actions` 省略：`source=file`、默认词表；
6. 仓内无文件跑 `action list`：不创建文件；
7. 配置错误：exit 1；`--json=x` / 多余参数：exit 2；
8. `-h` 含新命令与“生效词表见”提示；`action` 无子命令 exit 2；
9. `e.repo == nil` 时也能跑，不调用 backend；
10. 真仓 + 临时自定义仓各跑一次，`--json` 的 `config` 在本机可访问。

### 6. 验收

- 文本、JSON 与上述裁决一致；`config` 在 Windows/POSIX 各自是原生绝对路径，解码后 `os.Stat` 成功；
- 不读历史、不创建文件、不改现有命令输出；
- `go test ./...`、`go vet`、`gofmt` 全过；
- README / `machine-readable-output` 文档同步；`action log` 不受影响。
