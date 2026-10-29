# issue: refactor-structure — 结构性重构：把 cli.go 拆开、后端抽公共层、补最薄的那几块测试

- **domain**：design
- **关联**：relates-to `refactor`（性能那一半已经收尾，这里接手它的结构性一半；数字与"为什么现在做"都来自它）；relates-to `commit-cache`（`cli/cache.go` + `cli/enum.go` 是这次拆分的第一个候选）

## 背景

`refactor` 把性能做完了：热读 0 次 git 进程，真仓 `130 → 42.5 ms`（其中 39 ms 是进程地板，kiln 自己的活约 3 ms），100k `graph` 1,185 → 241 ms。结论里明确写了：**性能上已经没有结构重构的位置**，剩下的债是维护性的。

现状（`e24bfc8`）：

| 维度 | 数字 |
| --- | --- |
| 非测试代码 | 19 个文件 / 9,150 行 |
| 测试代码 | 12 个文件 / 7,563 行 |
| 最大单文件 | `internal/cli/cli.go` 3,475 行 / 117 个顶层函数 / 52 个 `(*env)` 方法 |
| 紧随其后 | `vcs/git` 852、`engine` 843、`vcs/jj` 496、`cli/cache.go` 475、`cli/enum.go` 467 |
| 后端重复 | `git` 与 `jj` 有 **17 个同名方法**（`Log` / `LogEach` / `Action` / `ActionChecked` / `action` / `Changes` / `Unchanged` / `Show` / `ShowStat` / `IsDedicated` / `NewWorkspace` / `ListWorkspaces` / `Dirty` / `AreaState` / `dirty` / `RemoveWorkspace` / `run`） |
| 覆盖率 | cli 86.5%、trailer 94.9%、i18n 92.3%、config 91.7%、cache 82.7%、engine 81.1%、vcs 77.5%、**git 45.0%**、**model 33.3%**、**jj 28.2%** |

`cli.go` 同时承担参数解析、命令编排、渲染（文本 / JSON / ascii / mermaid）与缓存协调；`loadCached` 一个函数兼五件事（扫层、算缺口、补拉、判重写、建模）。这个形状让"改一个命令要碰几处"一直很高，而覆盖率最薄的三块（`model` / `jj` / `git`）恰好是最想动的地方。

## 要回答的问题

1. **耦合到底长什么样**：每个命令碰哪些文件、哪些函数？`cli.go` 里的 117 个函数按职责能分成几簇，簇之间怎么依赖？有没有"一个改动必须同时改三处"的具体例子？
2. **先拆哪一块**：渲染（纯函数、无 I/O）是不是最安全的第一刀？拆分顺序怎么定，每步的验收是什么（输出逐字节不变 + 该步不引入新依赖）？
3. **后端那 17 个同名方法值不值得抽**：抽成公共 helper、模板方法，还是维持两个实现？`vcs.Repo` 契约要不要动？（不动契约是硬约束。）
4. **覆盖率 33% / 28% / 45% 是"没测"还是"难测"**：哪些是纯函数（好补），哪些必须靠 git/jj 真进程（难补）？补到多少才敢搬代码？
5. **怎么保证搬代码不改行为**：现有 23 条缓存/快照测试 + 逐字节矩阵够不够？要不要先加一层"快照式"回归（把命令输出存成 golden）再动手？

## 非目标

- **不改 CLI 契约**：命令、flag、文本 / JSON 输出逐字节不变；
- **不追求性能**（性能已收尾，见 `refactor` 结论）；
- 不动 `trailer` / `cache` / 枚举快照的**编码格式**；
- 不引入第三方依赖（仍是零依赖单二进制）；
- 不做大爆炸重写：每一步都要能单独提交、单独回滚；
- 不为"整齐"搬没被测试覆盖的代码——先测再动。

## 验收（collect 完成时）

- `cli.go` 的函数簇清单 + 每个命令的依赖落点（哪些命令碰了哪些簇）；
- 拆分路线图：每步"改什么 / 验收 / 风险"，第一步小到可以一次提交；
- 后端重复面的判断：抽 or 不抽，以及抽的边界（接口 / helper / 复制）；
- 覆盖率补测清单：先补哪几个文件、补到什么程度、为什么是它们；
- 一条"搬代码前的回归网"方案（golden 输出 / 现有测试的组织方式）；
- 明确说明哪些结构问题**不值得动**（比如已验证的解析器、格式编码）。

## 采集（2026-10-29）：cli.go 的 15 个函数簇、每个命令的依赖落点、拆分顺序与回归网

### 0. 一句话

`cli.go` 的问题不是 3,475 行太长，而是**每个读命令都要穿过同一段 727 行的缓存装配层**（30 个函数，7 个读命令每个摸到 25 个）；同时 `args`（12 函数 / 129 行）与 `render`（8 / 61）这两簇**没有任何出边**——它们是叶子，是能立刻搬走的部分。所以第一刀不是后端那 17 个同名方法（它们只是契约形状，不共享逻辑），而是：先建逐字节回归网，再搬叶子簇，最后把 `loadCached`（85 行 / 21 分支）拆成命名步骤。

口径（本轮全部数字）：函数体行数含注释与空行；分支 = `if`/`for`/`range`/`case`/`&&`/`||` 计数（gocyclo 风格）；"触及" = 同包调用图的传递闭包，不是运行期覆盖（原因见 §7）。

### 1. 耦合长什么样

包内 **149 个函数 = 76 个自由函数 + 73 个方法**（`(*env)` 52 个、`flags` 3 个、`pathBatchWriter` 3 个，其余 15 个）。`cli.go` 一个文件里有 117 个（52 个 `(*env)` + 59 个自由函数 + 6 个其它类型方法），2,565 行函数体，其余是 import、16 个 JSON 文档类型与注释。

按职责切成 15 个簇（每一行都是机械可搬的边界）：

| 簇 | 函数 | 行 | 分支 | 出边 | 建议落位 |
| --- | --- | --- | --- | --- | --- |
| entry | 13 | 336 | 100 | 每个命令簇各 1（`Run` 的分发） | `run.go`(170) + `help.go`(166) |
| args | 12 | 129 | 29 | **0** | `args.go` |
| load | 9 | 134 | 29 | cache 4、args 2 | `load.go` |
| cache | 30 | 727 | 187 | load 4、args 3、action 1 | `cache.go` + `enum.go`（已就位） |
| write | 11 | 297 | 63 | load 9、args 8、render 2、area 1 | `write.go` |
| area | 17 | 266 | 57 | args 7、load 2、entry 2、write 1、render 1 | `area.go`(100) + `workspace.go`(166) |
| action | 6 | 260 | 58 | args 14、entry 4、load 4、render 2、cache 1 | `action.go` |
| path | 14 | 328 | 73 | args 7、load 7、render 4、entry 1、list 1 | `path.go` |
| show | 2 | 114 | 23 | args 6、load 3、list 3、render 2、path 2 | `show.go` |
| status | 4 | 99 | 16 | area 8、args 6、entry 2、list 2、render 2、load 1 | `status.go` |
| list | 14 | 261 | 58 | args 9、render 8、load 1、entry 1 | `list.go` + `ordinals.go` |
| check | 4 | 88 | 21 | args 6、render 2、load 1、entry 1 | `check.go` |
| graph | 4 | 185 | 49 | args 11、load 3、list 1、render 1 | `graph.go` |
| config | 1 | 25 | 5 | args 4 | `config.go` |
| render | 8 | 61 | 11 | **0** | `render.go` |

（"出边"是该簇调用其它簇的边数，来自 `go/types` 解析后的调用图；15 簇函数数合计 149，行数合计 3,310。）

**每个命令的依赖落点**（传递闭包；"行"是触及的函数体行数）：

| 命令 | 函数 | 行 | 触及的簇 |
| --- | --- | --- | --- |
| `list` | 57 | 1,028 | entry 3、args 9、load 4、**cache 25**、list 11、render 5 |
| `path` | 58 | 1,088 | entry 3、args 7、load 4、**cache 25**、path 14、list 1、render 4 |
| `action log` | 53 | 1,055 | entry 8、args 11、load 3、**cache 23**、action 6、render 2 |
| `status` | 54 | 912 | entry 3、args 7、load 4、**cache 25**、area 8、status 4、list 1、render 2 |
| `graph` | 44 | 900 | args 9、load 4、**cache 25**、graph 4、list 1、render 1 |
| `show` | 44 | 832 | args 7、load 4、**cache 25**、show 2、list 3、path 1、render 2 |
| `check` | 45 | 828 | entry 3、args 7、load 4、**cache 25**、check 4、render 2 |
| `do` / `commit` | 31 | 534 | args 9、load 8、write 11、area 1、render 2（**不碰 cache**） |
| `worktree` | 23 | 361 | entry 3、args 7、load 2、area 9、write 1、render 1 |
| `cache gc` | 12 | 258 | args 3、load 3、cache 6 |
| `config` | 5 | 86 | args 4、config 1 |

三条读出来的事实：

1. **缓存不是"某个命令用的模块"，是每条读命令的必经之路**：30 个函数里 25 个落在每个读命令的闭包里；而 `loadCached` 一个函数同时做扫层、算缺口、补拉、判重写、建模五件事（85 行、21 分支）。这是"改一个读命令要碰几处"的主要来源——不是命令本身复杂，而是它站在一个 727 行的黑盒后面。
2. **写路径完全不碰缓存**（`do`/`commit` 闭包里 cache = 0）：它要的是写之前的历史，直接 `backend.Log()`。这与 `refactor` 结论里"提交后首次读 149 ms"对得上；但把写路径并进 loader 不会自动消掉那 149 ms（新提交在写完之后才存在），所以本轮不动。
3. **两个叶子簇**：`args` 与 `render` 出边为 0，`render` 还被 8 个命令共用（`writeJSON` 8 个调用者，`stateWord`/`shortID`/`issueTitle` 跨 status/list/path/show）。叶子 + 高复用 = 最安全的搬运对象，也解释了为什么现在改一个词要改多处。

**枢纽**（被 ≥8 个命令触及）：`parse` 与 `allowedFlags` 11，`env.backend` 与 `openRepo` 10，`warnUnknownFlags` 10，`env.rel`/`flags.get`/`flags.has`/`rejectSwitchValues` 9，`commonDirFromFS`/`cacheDir`/`writeJSON`/`usable`/`rejectExtra` 8。前五个是"每个命令都要解析参数 + 打开仓库"的必然代价；后面几个说明缓存层已经渗进参数层（`stripNoCache` 在 `cache.go`，`usable` 也是）。

**三个"改一处要动多处"的实例**：

| 改动 | 现在要动 | 证据 |
| --- | --- | --- |
| 给 `list` 加一个 flag | `cli.go` 4 处（用法表 L71、帮助表 L52、`rejectExtra` 的 usage 串 L2717、错误文案 L2742）+ README 中英各 2 处 + 测试 | `--search` 在 `internal/cli/*.go` 出现 4 次 |
| 给 issue 的 JSON 加一个字段 | 16 个 doc 类型（`cli.go` 3213–3373，约 160 行）+ 每个命令各一个 builder（`statusJSON`/`pathDocOf`/`showJSON`/`checkJSON`/`graphJSON`/`list` 内联）+ `issueCore` 的 3 处字面量（L2790/L2818/L3393） | `issueCore` 被 `statusIssue`/`listIssue` 内嵌 |
| 改一个状态词 | `render` 簇一处即可（`stateWord`/`modeName`/`progressWord`） | 反例是 `--no-cache`：全局剥离（`stripNoCache` 一处），正是"先抽公共层"的现成先例 |

### 2. 先拆哪一块

`render` 是最安全的第一刀，**但单独拆它不够**：`show` 依赖 `list` 簇里的 `ordinalList`/`matchingOrdinals`/`ordinalNumbers`，`path`/`check` 依赖 `shortID`/`stateWord`。所以第一刀是"叶子 + 被跨命令复用的词汇"，顺序按"出边为 0 → 出边少"排。

| 步 | 动什么 | 新增文件（行） | 验收 |
| --- | --- | --- | --- |
| 0 | golden 网（见 §5） | `golden_test.go`(~120) + `testdata/golden/`(~26 份) | 新测试过；`-update` 生成的 manifest 与现网输出逐字节一致 |
| 1 | `args` 簇 | `args.go`(~170，含 `flags`/`missingValue` 类型) | `go build`、`go test`、golden；`git diff -M` 显示 100% 相似度 |
| 2 | `help` 簇（entry 的一半） | `help.go`(~166) | 同上 |
| 3 | `run` 簇（entry 的另一半） | `run.go`(~170：`Run`+`extractLang`+`stripNoCache`+`printVersion`) | 同上；`Run` 的 switch 可选换成表驱动，非必需 |
| 4 | `render` + `ordinals` | `render.go`(~250，含 16 个 doc 类型) + `ordinals.go`(~30) | 同上；`show`/`list` 调用点不动（同包） |
| 5 | `load` 簇 | `load.go`(~134) | 同上 |
| 6 | `area` 拆两种职责 | `area.go`(~100，查询/状态) + `workspace.go`(~166，命令) | 同上；`status` 的 8 条 area 边不受影响 |
| 7 | 每命令一文件 | `show.go` 114 → `config.go` 25 → `check.go` 88 → `graph.go` 185 → `action.go` 260 → `status.go` 99 → `write.go` 297 → `path.go` 328 → `list.go` 261 | 每步同上 |
| 8 | `loadCached` 拆 5 步 | `cache.go` 内（85 行 → 扫层/算缺口/补拉/判重写/建模五个命名函数） | 23 条缓存测试 + golden + `--no-cache` 平价 |

风险与理由：

- **同包拆文件不改可见性，所以没有调用点要改**——这是选择"拆文件"而不是"拆子包"的核心理由。拆子包要设计导出 API、要重写测试的包路径，风险高一个量级，本轮不做（只记：将来真要拆，`render` 与 `cache` 是两个天然候选）。
- 唯一真实风险是搬运时漏 import、漏构建标签（`pipe_windows.go` 保持独立，不要并进 `args.go`）与 `gofmt` 漂移；每步 `gofmt -l internal cmd` + `go vet ./...` 是硬门槛。
- `git blame` 会断，但 `git blame -C -C -M` 仍能跟；每步只搬一个簇就是为了可追。
- 搬文件的覆盖率门槛是 0（编译器 + golden 足够）；拆函数（第 8 步）的门槛是 23 条缓存测试 + golden + `--no-cache` 平价，三者都已具备。

### 3. 后端那 17 个同名方法：不抽

把两边实现取出来、剥注释、归一化 receiver 名与包路径后逐行比（`difflib` 行级相似度）：

| 方法 | git 行 | jj 行 | 相似度 | 判断 |
| --- | --- | --- | --- | --- |
| `NewWorkspace` | 20 | 20 | 0.90 | 形状同：都是"拼参数 → `run` → 包错"，参数串完全不同 |
| `Log` | 7 | 7 | 0.86 | 同上，7 行里 3 行是 `run` + `parseLog` |
| `Unchanged` | 13 | 13 | 0.85 | 同上 |
| `Changes` | 7 | 7 | 0.71 | 同上 |
| `action` | 29 | 22 | 0.63 | 同序：暂存 → 校验 → 提交，每步命令与错误文案不同 |
| `LogEach` | 65 | 35 | 0.46 | **唯一真重复**：`StdoutPipe` + `WaitDelay` + 停止即 kill + stderr 摘要，约 20–25 行重合 |
| `Action`/`ActionChecked` | 3 | 3 | 1.00 | 3 行薄壳（`Action` → `ActionChecked` → `action`） |
| `Dirty` | 4 | 1 | 0.00 | git 要跑命令，jj 读一次 `log` 结果 |

结论：

- **不抽 template method，不动 `vcs.Repo`。** 17 个同名是契约形状（`Repo` 7 个 + 工作区 6 个 + `Show`/`ShowStat`/`LogEach`/`action`），相似度高的是调用壳的形状，不是逻辑；能抽的公共层只有"调 `run` 并包错"，而 `run` 已经是公共层。
- 真重复只有 `LogEach` 的流式子进程外壳（两处约 40 行）。抽成 `vcs.Stream(cmd, read)` 能省 40 行，但动的是全仓最容易挂死的代码（`WaitDelay` 3s、停止即 kill、`Wait` 与 `Close` 的顺序），收益小于风险 → **现在不抽**。触发条件：出现第三种后端，或 `LogEach` 真挂死过一次。
- 两个便宜动作值得做（都不改行为）：① 编译期断言——现在一个 `var _` 都没有，加 `var _ vcs.Repo = git.Git{}` / `var _ vcs.CacheBackend = git.Git{}` / `var _ vcs.Repo = jj.ExecRunner{}`，让"实现跟不上契约"在编译期暴露；② jj 的 `run` 留缝（`ExecRunner` 加一个可注入的 run 字段或抽 `runner` 接口），10 个命令壳立刻可单测（`jj` 28.2% 的直接成因）。

### 4. 覆盖率 33% / 28% / 45%：成因不同，补法不同

| 包 | 数字 | 0% 的函数 | 成因 |
| --- | --- | --- | --- |
| `model` | 33.3% | 12 个（`relation.go` 10 个 + `Last`/`State`） | 本包测试只测 `model.go` 的 `ActionSet`；`RelationSet` 全靠 cli 集成测试跑，per-package 口径看不见 |
| `jj` | 28.2% | 10 个命令壳（`Log`/`action`/`Changes`/`Show`/`workspace*`/`run`） | 10 个测试全是纯解析器（`parseLog`/`parseSummary`/`parseWorkspaces`/`streamRecords`）；本机没有 jj 二进制，壳不可测——**不是难测，是没留缝** |
| `git` | 45.0% | 16 个（`CacheBackend` 5 个 + `Log`/`Show`/`ShowStat`/`LogStats`/`Commits`/`Reachable`/`CommonDir`/`LogOIDs`/`IsDedicated`/`Dirty`/`root`） | `git_test.go` 14 个测试里 5 个是 fixture 助手（`newRepo`/`commit`/`write`…），测试本身大多建真仓，但只盖了"写 + 解析"一半；读的那一半在 cli 集成测试里跑 |

补测顺序（按"搬家会不会踩到"排，不按数字排）：

1. `model/relation.go`（纯函数，半天，33% → 80%+）：搬 `render`/`ordinals` 必然碰 `RelationSet`，这是唯一有直接因果的一条。
2. `jj` 的 run 缝（半天，28% → 60%+）：搬后端或改 `vcs` 之前该有的网。
3. `git` 不补包内单测：端到端已覆盖，补它只是让数字好看；真要补就补 `CacheBackend` 的错路径（损坏层、版本不匹配），而 `cache_test.go` 已经盖了大半。

### 5. 搬代码前的回归网

现状：`cli_test.go` 4,360 行里有 **172 处 `strings.Contains`、只有 23 处 `Equal`**——断言的是片段，不是整体。搬家时把一个 `fmt.Fprintf` 换成 `Fprintln`、多一个空格，现有测试全绿。

方案（`internal/cli/golden_test.go` + `testdata/golden/`）：

- **fixture 由测试现建**：3 个仓库——空仓、3 提交小仓、30 提交中仓（含 relation、area、`--json`、`action log` 的历史）。提交日期用 `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` 锁死，`--lang zh` 固定，避免依赖本机时区与语言。
- **矩阵**：11 个命令 × {热读, `--no-cache`}，再加 `--json` 变体，约 26 份输出。
- **归一化**：`config`/`status`/`worktree list` 的输出含 fixture 绝对路径（`t.TempDir()` 每次不同）→ 比对前把 fixture 根替换成 `<ROOT>`；其余不做任何加工。
- **比对**：全文逐字节；失败打首个不同行（带上下文）；`-update` 重生成。
- **成本**：harness ~120 行，26 份 golden 合计 < 60 KB（fixture 小，`path --all` 也不会到 MB 级）。

这条网同时是第 8 步（拆 `loadCached`）的验收工具：热读与 `--no-cache` 两条路径必须逐字节一致——正是 `refactor` 里用过的"六命令 × 四仓"矩阵，区别只是固化成测试而不是临时脚本。

### 6. 明确不值得动的

- `isWide`（29 行 / 24 分支的东亚宽度判定）：已验证，且 `pad`/`displayWidth` 压在它上面，为了"精简"动它是最差的性价比。
- `trailer` / `cache` / 枚举快照的编码格式：格式动了就有兼容问题，与结构无关。
- `vcs.Repo` 契约与两个 exec 壳：契约稳定（7 + 6 + 2 个可选接口），壳薄且错误文案是产品的一部分。
- `parse`（34 行、11 个命令共用）：换成 `flag` 包的收益小于"同时接受 `--x=v` 与 `--x v`"的代价。

### 7. 更正与局限

- ask 里写"`cli.go` 3,475 行 / 117 个顶层函数 / 52 个 `(*env)` 方法"。复核后：117 是 `cli.go` 里的**函数总数**，其中 52 个是 `(*env)` 方法、59 个自由函数、6 个其它类型方法；包内合计 149 个函数、3,310 行函数体。ask 里的说法是笔误，不是新变化。
- 非测试 9,162 行 / 测试 7,563 行（ask 写 9,150，差 12 行，当时手数误差）。
- 覆盖率数字没变，但"33.3% / 28.2% / 45.0%" 是 per-package 口径：cli 的集成测试跑过的路径不计入 `model`/`git`/`jj` 的分子。三个数字里只有 `jj` 是真正的空白。
- **没测成的**：逐命令覆盖率。`go test -coverpkg=./...` 在这台机器上不写 profile，`go tool` 里也没有 `covdata`（本机 Go 只有 asm/cgo/compile/cover/fix/link/preprofile/vet），所以拿不到"跑一次 `list` 到底执行了哪些函数"的经验值。本文的"触及"是静态调用图的传递闭包，口径上会**高估**（分支里才走的路径也算），但用来排序与定边界足够。

### 结论（采集）

1. 先建 golden 网（§5），再搬 `args`/`help`/`run`/`render`/`ordinals` 五个叶子/复用簇（§2 的 1–5 步），每步一个提交、逐字节验收。
2. `loadCached` 是读路径唯一的结构瓶颈（85 行 / 21 分支，7 个读命令共用），在现有 23 条缓存测试保护下拆成 5 个命名步骤——这是本轮唯一"改逻辑形状"的动作。
3. 后端 17 个同名方法保持两份实现；只加编译期断言，并给 jj 的 `run` 留一个可注入的缝。
4. 覆盖率按"搬家会不会踩到"补，不按数字补：`model/relation.go` → jj 的 run 缝 → 其余不补。

## 假设（2026-10-29）：先上回归网，再拆读路径的瓶颈；文件搬家排在后面，而且可以随时停

### 1. 我押的是什么（三条可证伪的判断）

- **H1 导航成本可测地下降**：改一个读命令要读的行数，从"3,475 行里翻"降到"一个 ≤330 行的文件 + 两个明确的依赖（render/ordinals、load）"。→ 证伪方式：拆完 3 个命令文件后回看，若改一个命令仍要读 3 个以上文件才能看懂，说明词汇/序数没抽干净，H1 不成立，停止长尾。
- **H2 `loadCached` 是唯一值得改逻辑的地方**：把它拆成 6 个命名步骤后，读路径从"一个 85 行 / 21 分支的函数"变成"6 个各 ≤30 行的函数"，且热读与 `--no-cache` 的输出逐字节不变。→ 证伪方式：拆分需要动 `loadCached` 之外的签名（`persist`/`enumerateDefault` 的返回契约），或 golden 红一次且无法解释。
- **H3 回归网必须先于搬家**：没有它，"9 次机械移动没改行为"只能靠人眼；有它，每次搬家是"1 个提交 + 1 条命令"。→ 反证也有用：若 9 次搬家 + 1 次拆函数里网一次没红，说明搬家确实机械、网在空转（那就把网留着当回归，不必再加）；若红了但都是误报，说明归一化没做好，先修网。

### 2. 顺序：把 collect 的第 8 步（拆 `loadCached`）提到第 3 步

collect 把 `loadCached` 排在最后，是"先易后难"的直觉。但 `cache.go` / `enum.go` 本来就是独立文件，`cli.go` 拆不拆都不影响读它们——所以"先拆文件让 `loadCached` 更好读"这个理由不成立。反过来，`loadCached` 是 7 个读命令的共同瓶颈，而保护它的 23 条缓存测试已经就位，收益/风险比最高。改后的顺序：

| 阶段 | 内容 | 提交 | 工作量 | 风险 | 收益 |
| --- | --- | --- | --- | --- | --- |
| 0 网 | `golden_test.go` + `testdata/golden/` | 1 | 半天 | 低（可降级） | 后面 10 步全部可验收 |
| 1 叶子 | 搬 `args` / `help` / `run` / `render`+`ordinals` | 4 | ~1 小时 | 无 | `cli.go` 3,475 → ~2,790；"改一个词"只碰 `render.go` |
| 2 瓶颈 | `loadCached` 拆六步（站点/枚举/缺口/取件/组装/落盘合并） | 1–2 | 半天 | **中**（唯一逻辑改动） | 读路径从 85 行一函数变成 6 个命名步骤 |
| 3 长尾 | 9 个命令各一文件 | 9 | 2–3 天 | 无 | `cli.go` 消失；新文件最大 330 行 |
| 4 顺手 | 编译期断言 + jj 的 `run` 缝 | 1 | 半天 | 低 | 契约有断言；`jj` 28% → 60%+ |

为什么阶段 3 可以放最后、也可以做一半：它只降低"读文件"的成本，不改变任何依赖关系；而阶段 2 降低的是"读逻辑"的成本。前者随时可停，后者做完才算这次成功。

### 3. 每步的验收（统一一张表）

| 检查 | 适用 |
| --- | --- |
| `go build ./...` 过 | 全部 |
| `gofmt -l internal cmd` 空、`go vet ./...` 干净 | 全部 |
| `go test ./...` 绿 | 全部 |
| `go test ./internal/cli -run TestGolden` 绿 | 全部 |
| `git log --stat -M -1` 每个新文件 100% 相似度 | 搬家步（阶段 1、3） |
| 六命令 × {热读, `--no-cache`} 哈希一致 | 逻辑步（阶段 2、4） |

中间态必须可发布：阶段 1 结束时 `cli.go` 还有 2,790 行，阶段 3 每搬完一个命令就停一天也不影响仓库。

### 4. 止损（提前写死，避免"拆到一半没法收场"）

- **网**：若 golden 在半天内稳不下来（时区 / 语言 / 绝对路径 / 并发顺序），降级为"只锁 6 个文本命令 + 2 个 JSON"，不追 26 份；网不完美不阻塞后续，但搬家验收降级为"人工对比 + 现有 172 条片段断言"。
- **搬家**：若某一步 `git diff -M` 不是 100% 相似度（说明顺手改了逻辑），回滚该提交，只做移动。
- **瓶颈**：若拆 `loadCached` 需要动 `persist` / `enumerateDefault` 的签名或返回契约，停——那已不是拆分，是设计变更，回到 design。
- **长尾**：拆到第 3 个命令文件时回看 H1；若"改一个命令要读的行数"没降，停掉剩余 6 个，把 2,790 行的 `cli.go` 留着——它并不比 9 个文件差。
- **总预算**：3 天。超了就停，把没拆的部分写成触发条件（下次改到那个命令时顺手拆）。

### 5. 明确不做（本轮）

- 不拆子包。`render` 与 `cache` 是两个天然候选，但要设计导出 API、重写测试路径，风险高一个量级；本轮只做同包拆文件。
- 不动 `vcs.Repo` 契约，不抽后端 template method（只加 `var _` 断言）。
- 不改任何 CLI 输出、flag、JSON 字段、文案。
- 不重排函数顺序：移动时保持原顺序，"只搬不改"让 diff 只看文件边界。
- 不把 `Run` 的 switch 改表驱动（可选、无收益、纯噪音）。
- 不补 `git` 的包内覆盖率；不动 `trailer` / `cache` / 枚举编码、`isWide`、`parse`。

### 6. 如果只能做三件事

1. golden 网（阶段 0）；2. `loadCached` 六步（阶段 2）；3. 四个叶子搬家（阶段 1）。这三件做完，读路径的"黑盒"与"导航"两个问题基本解决；长尾 9 个命令文件是"有空就做"，不做也不算这次失败。

### 7. 结算方式（提前定义，免得事后各说各话）

结构重构没法用"快了多少"结算，所以把收益定义成四条可验收的事实：

| 指标 | 现在 | 做完 |
| --- | --- | --- |
| `cli.go` | 3,475 行 / 117 函数 | 不存在 |
| 最大文件（不含 `cache.go`/`enum.go`） | 3,475 | ≤ 330（`path.go`） |
| 一个读命令的可读范围 | 1 个 3,475 行文件 + 2 个缓存文件 | 1 个 ≤330 行的命令文件 + `render`/`ordinals` + `load` |
| `loadCached` | 85 行 / 21 分支 | 6 个函数，各 ≤30 行 |

诚实的提醒：`loadCached` 拆完分支总数不变（21 → 21，只是分布开），这是**命名，不是简化**——不要期待复杂度下降；`cache.go`（456 行）与 `enum.go`（443 行）仍是最大的两个文件，本轮不动它们，所以"最大文件 ≤330"只对新拆出的命令文件成立。

四条全满足就算做完；不满意的部分，写明为什么没做，而不是"以后再说"。

## 落地（阶段 0，2026-10-29）：golden 回归网

### 1. 形状

- `internal/cli/golden_test.go`（约 300 行）+ `internal/cli/testdata/golden/{basic,area}/`：30 个用例 × 2 个 fixture = **60 份录制，45.6 KB**，最大的一份 `help.txt` 5.6 KB。
- 两个 fixture 都由测试现建。`basic`：10 个提交、4 个 issue（其中 `late-idea` 故意没有起点）、`blocks`/`part-of`/`relates-to` 三种关系、`tags`、两个 domain。`area` 在 `basic` 之上多一个 `git worktree`，所以 `status` 的 `已开 1 个` 与 `worktree list` 都被录进去。
- 每个用例跑两遍：热读一次、`--no-cache` 一次，**两者必须逐字节相同**（`refactor` 的不变式固化进测试），然后再与录制文件比。
- 记录格式：`argv` / `exit` / `stdout[n]` + 正文 / `stderr[n]`。n 是**归一化后**的字节数——fixture 路径本身长度每次运行不同，用原始长度会让 manifest 不稳定（第一版就是这么红的）。
- 归一化：把 fixture 根替换成 `<ROOT>`，同时覆盖 JSON 转义（`\\`）与正斜杠（`/`）两种写法。
- 用例只选只读命令（不含 `do`/`commit`/`worktree new|remove`/`cache gc`，它们会改 HEAD 或缓存目录），所以 30 个用例可以在同一个 fixture 上任意顺序跑。

### 2. 确定性从哪来

- `TestMain` 里加 `TZ=UTC`：`action log` 与日期列都用 `time.Local` 渲染，不钉住的话 UTC+8 录的 golden 在 CI（UTC）上必红。
- 提交日期写死（`GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE`，全部 `+00:00`），`KILN_LANG=zh` 早已固定。
- `config path` 的用例在 fixture 里预先建好配置文件，否则第一次运行会多打印一行"已写入一份带注释的默认配置"——这是第二处踩到的坑。

### 3. 一次"网会红"的验证

故意在 `status` 表头加一个空格 → `TestGolden/basic/status` 立刻红，并打出首个不同行（`recorded` / `current` 各三行上下文）→ 还原 → 绿。这是 H3 的正面证据：搬家时一个空格的漂移就够触发。

### 4. 成本

- 全量 `go test ./internal/cli`：**21.5 s**（其中 golden 约 7.7 s）。
- 钉住 `TZ` 后全套测试仍绿（本机验证；没有 CI 可复验，所以只记"本机绿"）。

### 5. 遗留

- 矩阵里没有 `do`/`commit`：写路径的输出也值得锁，但它们改 HEAD，得换成"每个用例克隆一份 fixture"，成本高一些，等阶段 2（拆 `loadCached`）需要时再补。
- `graph --mermaid`、`path --stat` 等变体已经覆盖，但没有 `--shortstat`、`graph --format=ascii` 的显式重复（二者与已录用例等价）。