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
