# issue: perf-audit — kiln 的性能现状与优化边界

- **domain**：cli
- **关联**：relates-to `issue-list-scaling`、relates-to `cli-consistency`、relates-to `path-batch`

## 背景

kiln 没有索引：每条读命令都从提交历史现算模型——一次 `git log`，加上按命令需要的少量 git 子进程（`show`、`--stat`、worktree 状态）。仓库小的时候这换来“零状态、永远和 git 一致”；issue 多了以后，代价才开始显形：`issue-list-scaling` 的 collect 实测 60 issue / 260 动作时 `path --all` 121 KB / 102 ms；此后 `path-batch` 把批量做成一次读全仓，`cli-consistency` 又加宽了路径（多 id 去重、重复 flag 拒绝、帮助等）。

用户提出讨论性能与优化。**先测，再谈优化**：本 issue 的 collect 要给出真实数字、增长曲线与热点，hypothesize 再决定改什么、不改什么。

## 要回答的问题

1. **绝对耗时**：7 个读命令（`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list`）在本仓（62 issue / 280 动作）各多少？`do` / `commit` 的固定开销多少？进程启动占其中多少？
2. **增长曲线**：动作数 250 / 500 / 1000 / 2000 / 4000 时，`status` / `list` / `check` / `graph` / `path --all` 怎么变？线性、超线性，还是某个命令先拐弯？
3. **热点**：时间花在进程启动、`git log`、模型构建、解析 trailer、渲染，还是每个动作的额外 git 子进程？
4. **子进程模型**：每条命令实际 spawn 几个 git 进程？`path --all --stat` 是不是每个动作一个？`worktree list` 每个区几个？
5. **输出体量**：文本 / JSON 大小随 issue 数怎么长？`path --all` 什么时候大到不适合一次读？
6. **重复工作**：同一命令内是否重复读配置、重复 `git log`、重复排序、重复判定 `done`？有没有 O(n²)？
7. **优化点与收益**：一次遍历合并、批量 `--stat`、减少 alloc / 字符串拼接、并发 git——各自预期省多少？哪些与“无索引”设计冲突？
8. **预算**：要不要定一个“≤1000 动作 p95 < X ms”的目标，还是只保证“线性 + 千级可用”？

## 候选（测完再选）

- **A 不做优化**：只把数字与上限写清楚，承诺“千级动作可接受”；
- **B 低风险热修**：`--stat` 批量化、去掉重复遍历 / 大分配，不改契约；
- **C 结构性改动**：索引 / 缓存 / 并发 / 常驻进程——与无索引设计冲突，需单独立项。

## 非目标

- 不引入索引、数据库、daemon 或常驻进程；
- 不为性能改 JSON 契约或文本视图；
- 不在测量前先做并发；
- 不优化没有被测量证明的路径。

## 验收（collect 完成时）

- 真仓每个读命令 ≥5 次运行的 min / median；
- 合成仓 5 个规模的增长表（动作数 vs 耗时）；
- 每个命令的 git 子进程数与各阶段占比；
- 文本 / JSON 体量表；
- 热点排序 + 候选（必须改 / 可选 / 不做），留给 hypothesize 定性。

## 采集（2026-10-27）：本机实测

### 方法与口径

- **机器**：Windows（本机会话所在机器），Go 1.27，kiln 由 `e0afdce` 的源码构建（`go build -o kiln.exe ./cmd/kiln`）。
- **计时**：每条命令先跑一次预热，再跑 7 次取 min / median（`path --all --stat` 只跑一次）；PowerShell `Stopwatch` 包住整个进程，包含 PowerShell 的 spawn 与 kiln 的启动。
- **增长曲线**：用 `git fast-import` 造合成仓，**1 提交 = 1 issue = 1 action**（`issue: perf-N` + `action: ask`），N = 250 / 500 / 1000 / 2000 / 4000；每点 3 次取中位数。
- **子进程数**：`GIT_TRACE2_EVENT` 记录每次 `git` 进程，数 `"event":"start"`。
- **体量**：stdout 重定向到文件后取字节数。
- **口径警告**：这台机器的**进程创建很贵**——空 Go 程序 `noop.exe` 约 21–32 ms，`git --version` 约 80–105 ms，`kiln --version` 约 92–105 ms。绝对数字是 Windows + 本机 Defender 环境的，结构性结论（谁按动作 spawn、谁是常数）不依赖它；Linux/macOS 上固定成本会小一个量级。

### 1. 真仓绝对耗时（62 issue / 280 动作 / 3 domain）

| 命令 | min (ms) | median (ms) | 说明 |
| --- | --- | --- | --- |
| `--version`（启动地板） | 89.7 | 105.1 | 不读仓库、不 spawn git |
| `status` | 274.9 | 301.8 | log + worktree list |
| `status --json` | 287.0 | 322.6 | 同上 |
| `list` | 191.1 | 218.3 | log |
| `list --json` | 194.3 | 203.3 | 同上 |
| `list --search=zzz` | 193.9 | 206.6 | 同上（0 命中） |
| `check` | 194.8 | 209.7 | log |
| `check --json` | 200.1 | 216.0 | 同上 |
| `graph` | 197.9 | 220.3 | log |
| `graph --json` | 205.1 | 223.0 | 同上 |
| `path --all --json` | 210.7 | 223.8 | log，输出 134 KB |
| `path <id> --json` | 205.7 | 220.2 | 同上 |
| `show <id> 1` | 286.8 | 322.4 | log + show |
| `show <id> 1 --json` | 298.6 | 315.8 | 同上 |
| `worktree list --json` | 129.4 | 133.8 | **不读历史**，只有 worktree list |
| `path --all --json --stat` | — | **14,396**（单次） | 283 个 git 进程：1 log + 282 show --stat |
| `path <id> --json --stat`（6 动作） | 484 | 998 | 与动作数成比例 |

**读命令的公共地板**：约 100 ms 启动 + 约 50–60 ms 一次 `git log`。去掉地板后，`list` / `check` / `graph` / `path` 的模型构建 + 渲染只有几十毫秒。

### 2. 增长曲线（合成仓，1 提交 = 1 issue）

| 动作数 | status | list --json | check --json | graph --json | path --all --json | list --search | show | git log |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 250 | 291 | 232 | 206 | 214 | 296 | 202 | 318 | 60 |
| 500 | 313 | 209 | 193 | 211 | 210 | 207 | 297 | 46 |
| 1000 | 316 | 213 | 211 | 220 | 228 | 224 | 330 | 47 |
| 2000 | 309 | 231 | 236 | 223 | 254 | 213 | 333 | 44 |
| 4000 | 345 | 264 | 276 | 277 | 275 | 260 | 349 | 43 |

- **从 250 到 4000（16×），所有读命令只涨 30–60 ms**；总时间基本是启动 + 一次 `git log` + 渲染的常数。
- `git log` 自身 43–60 ms，**与提交数几乎无关**（2000 条空提交的 log 比 280 条真实提交还快）——瓶颈是进程与消息量，不是遍历。
- 模型构建约 **10–15 µs/提交**（4000 条比 250 条多出的部分），排序与渲染没有出现超线性。
- `--stat` 不在表里：它按动作 spawn，合成仓 4000 动作按 51 ms/动作外推约 **200 s**（真仓 282 动作实测 14.4 s）。

### 3. 子进程账本（实测）

| 命令 | git 进程数 | 都是什么 |
| --- | --- | --- |
| `list` / `check` / `graph` / `path --all` | 1 | `git log --format=…` |
| `status` | 2 | log + `worktree list --porcelain` |
| `show` | 2 | log + `show <id>` |
| `worktree list` | 1 | `worktree list --porcelain` |
| `do` | 1 | log（起点动作重名检查） |
| `commit` | 7 | log、`rev-parse --show-toplevel`、`status --porcelain -z`、`status --porcelain`、`add -A`、`commit`、`rev-parse HEAD`（外加 git 自己的 `maintenance run`） |
| `path … --stat/--shortstat` | 1 + 动作数 | log + 每个动作一次 `show --stat` |
| `path --all --stat`（真仓） | **283** | 1 log + 282 show |

`commit` 实测约 **830–860 ms**（8 个进程），是单条 issue 上最贵的写操作；其中 `status` 读了两次（一次 `-z` 取变更清单、一次判 dirty），`rev-parse --show-toplevel` 一次。

### 4. 体量

| 动作数 | status 文本 | status JSON | list JSON | check JSON | graph JSON | path --all JSON |
| --- | --- | --- | --- | --- | --- | --- |
| 250 | 4.5 KB | 35 KB | 43 KB | 41 KB | 28 KB | 89 KB |
| 1000 | 18 KB | 138 KB | 169 KB | 160 KB | 109 KB | 348 KB |
| 4000 | 72 KB | 558 KB | 682 KB | 643 KB | 442 KB | 1.34 MB |

- 都是**线性**：约 18 B/issue（status 文本）、139 B（status JSON）、170 B（list JSON）、349 B（path --all JSON）。
- 真仓 62 issue / 280 动作：`path --all --json` 134 KB、`list --json` 20 KB、`status --json` 18 KB。
- 1.3 MB 的 JSON 在 275 ms 内写完，编码不是瓶颈；真正会放大的是 `--stat`（每个动作的 diffstat 再叠进文档）。

### 5. 热点排序（实测，不是候选）

1. **`--stat` 的 O(动作数) 子进程**：51 ms/动作，282 动作 = 14.4 s；4000 动作外推约 200 s。这是唯一随规模线性变慢、且已经不可用的路径。
2. **固定进程成本 ≈ 100 ms/命令**：占每个读命令的 30–50%。本机 `git --version` 自己就要 ~100 ms，所以“少 spawn”本身就是最大的优化。
3. **`status` 每次多一次 `worktree list`**（约 40–60 ms），即使 `worktree=off`、0 个区；它换来 `has-worktree` 与“已开 N 个”的诚实计数。
4. **`show` 付两次进程**（log + show）；没有索引时先读全史再定位是设计使然。
5. **`commit` 付 7 个进程 ≈ 840 ms**；两次 `status` 与一次 `rev-parse --show-toplevel` 有合并空间。
6. 输出编码、排序、`status` 分组在千级动作都**没有**成为热点。

### 6. 没测 / 边界

- 未测内存峰值、冷启动（文件缓存已热）、Linux/macOS、jj 后端、并发。
- 合成仓是空树提交，`git log` 的耗时与真实提交不同；但两组数据都显示 log 成本由进程 + 消息量主导，不由提交数主导。
- `--stat` 的 4000 动作数字是**外推**（51 ms/动作 × 4000），没有真跑。
- 计时包含 PowerShell 的 spawn 开销（约 20–30 ms），所以各命令之间的差值比绝对值更可靠。

### 7. 观察（留给 hypothesize 定性）

- 无索引设计在**千级动作仍可用**：读命令 0.2–0.35 s，其中 0.1 s 是进程启动；真仓 62 issue 是 0.2 s 级。
- 唯一会爆的是 `--stat`：按动作 spawn。一次 `git log --stat` 批量取全部 diffstat，理论上把 14.4 s 降到 0.3 s 级，是收益最大的单点。
- `status` 的第二次 git 调用可以省掉或改成惰性，但要接受“不列区就不显示 has-worktree/已开数”或缓存。
- `commit` 的 7 个进程里至少两个可合并（两次 status、toplevel）。
- 预算（待定）：读命令 ≤ 500 ms @1000 动作（本机实测 0.32 s）；`--stat` 批量后 ≤ 1 s @1000 动作。

## hypothesize（2026-10-27）：`--stat` 改成一次 `git log --stat`

### 主张

**批量 `--stat` / `--shortstat`（`path --all`、`path <id> <id> …`）不再每个动作 spawn 一次 `git show`，改成一次 `git log --stat`（或 `--shortstat`）取回全部动作的 diffstat，按 commit id 查表。**

- 单条 `path <id> --stat` 与 `show --stat` 保持现状（动作少，批量反而多扫历史）；
- jj 或批量调用失败时，回退到现有的逐动作 `ShowStat`，行为不变；
- **文本与 JSON 输出逐字节不变**。

### 证据（collect + 本轮补测）

真仓 296 commits / 281 actions：

| 调用 | 实测 |
| --- | --- |
| 现状：282 次 `git show --stat` | **14,396 ms** / 283 进程 |
| `git log --stat --format=…`（一次） | **384 ms** / 1 进程 |
| `git log --shortstat --format=…`（一次） | **383 ms** / 1 进程 |
| `git show --stat <296 ids>`（一次） | 613 ms / 1 进程 |
| 分块 `git show --stat`（256/块，合成仓 4000 空提交） | **4,993 ms** / 16 进程 |
| `git log --stat`（合成仓 4000 空提交） | **139 ms** / 1 进程 |

**等价性**：对 3 个提交做原始字节比对，`git log --stat` 的块与 `git show --stat` 的块在去掉前后换行后**完全相同**（148 B / 142 B / 113 B 三例）。`git log` 的格式行会多两个前导换行，`strings.TrimSpace` 即消——现有 `pathDocOf` 已经这么做了。

**为什么不用分块 `show`**：`git show` 按 id 逐个建立提交上下文，4000 个 id 分 16 块要 5.0 s，是 `git log --stat` 的 36 倍；而且一次传 4000 个 id 会超过 Windows 命令行上限（实测启动失败）。

### 契约（落地的验收标准）

- `path --all --json --stat` / `--shortstat` 的输出与改动前**逐字节相同**；
- `path <id> --stat` 文本、`show --stat` 文本与 JSON 不变；
- 失败语义不变：某个 action 的 stat 取不到 → 该 issue 进 `errors`（`stat-failed`），其余照常，exit 1；
- `stat` 仍是后端原始 diffstat 文本，不自己重绘；
- 不支持批量接口的后端（jj）或批量调用出错 → 逐动作回退，行为与现在完全一致。

### 落点

1. `internal/vcs/vcs.go`：新增**可选**接口
   ```go
   // StatLog is implemented by backends that can render every commit's diffstat
   // in one pass. want filters which commits to keep, so a huge history does not
   // have to be held in memory.
   type StatLog interface {
       LogStats(short bool, want func(id string) bool) (map[string]string, error)
   }
   ```
2. `internal/vcs/git/git.go`：`LogStats` 跑
   `git log --stat --format=%x00%H%x00`（`short` 时换成 `--shortstat`），按 NUL 切分：
   `["", id1, block1, id2, block2, …]`，`want(id)` 为真才存；块原样返回，由调用方 `TrimSpace`。
3. `internal/cli/cli.go`：`pathBatch` 在 `mode != statNone` 且 backend 实现 `StatLog` 时先取一次 map（`want` = 本次所有 issue 的 action id），把 map 传给 `pathDocOf`；`pathDocOf` 增加可选 `stats map[string]string`：命中用 map，未命中（合并提交等）回退 `ShowStat`。单条 `path` 与 `show` 不传 map。
4. `internal/vcs/jj`：不实现 `StatLog`，走回退路径（行为不变）。
5. 测试：`fakeRunner` 实现 `LogStats` 并计数；新增
   - 批量 `--stat` 只调一次 `LogStats`、零次 `ShowStat`；
   - map 未命中的 action 回退 `ShowStat`；
   - 后端不实现 `StatLog` 时走逐动作回退；
   - 输出与既有 `TestPathAllJSONCarriesStats` 一致。

### 预期收益

- 真仓：**14.4 s → 约 0.5 s**（启动 0.10 + `git log` 0.05 + `git log --stat` 0.38 + 渲染）；
- 4000 动作（空提交）：约 0.3–0.4 s；
- 代价：`git log --stat` 会为**所有**提交算 diff，混合仓（大量非 action 提交）的成本随总提交数增长。接受这个代价——它仍是一个进程，且比现状的 N 个进程低一个量级；真到不行再考虑按 action 集合收窄（`show` 分块已证明更慢）。

### 检验办法

1. 改动前保存 `path --all --json --stat` 与 `path --all --json --shortstat` 的输出，改动后逐字节对比；
2. 真仓计时 3 次取中位；合成仓 4000 计时；
3. 单 issue `path <id> --stat` 与 `show <id> --stat` 输出不变；
4. jj 用 fake 后端跑回退（本机无 jj 环境）；
5. `gofmt` / `go vet` / `go test ./...` 全过；`kiln check` 仍全绿。

### 反方（保留）

- **`git log --stat` 扫全史**：如果 action 只占提交的很小一部分，会做无用功；分块 `show` 在 4000 id 时慢 36 倍，但那是空提交、每块 256 的最坏测量。若将来出现“10 万提交 / 500 action”的仓，可再按 action 比例在两种策略间选择——先记录触发条件，不预做。
- **两进程**：`path --all --stat` 仍要 `git log`（建模型）+ `git log --stat`（取 diffstat），约多 50 ms。要省可以把两者合并成一次带 stat 的 log，但那要动 `engine` 的模型来源，收益 10%，不在本批。
- **合并提交**：`git log --stat` 默认不给 merge 的 stat；action 提交由 kiln 生成、不是 merge，且未命中会回退 `ShowStat`，风险可控。

### 其他收集发现的处置（不在本批）

- `status` 的第二次 `worktree list`（约 40–60 ms）：保持——`has-worktree` 与“已开 N 个”必须诚实；要省得先接受字段可缺失，另立项。
- `commit` 的 7 个 git 进程（约 840 ms）：两次 `status` 与 `rev-parse --show-toplevel` 有合并空间（约 100–150 ms），另立项。
- 预算：本批落地后写进文档——**读命令 ≤ 500 ms @1000 动作（本机）**；`path --all --stat` ≤ 1 s @1000 动作。是否入 README 留到 conclude。

### 不做

- 不用 `--numstat` 自绘 diffstat（要复刻 git 的列宽/二进制/重命名格式，风险高、收益低）；
- 不分块 `git show`（实测更慢）；
- 不做并发 spawn（进程成本主导，收益不确定，错误语义与顺序复杂）；
- 不引入索引、缓存、daemon。

## apply（2026-10-27）：批量 `--stat` 改成一次 `git log --stat`

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | 新增可选接口 `StatLog { LogStats(short bool, want func(id string) bool) (map[string]string, error) }`——批量取 diffstat，`want` 只为需要的提交保留结果 |
| `internal/vcs/git/git.go` | `LogStats` 跑 `git log --stat --format=%x00%H%x00`（`short` 时 `--shortstat`）；`parseStatLog` 按 NUL 切成 `["", id1, block1, id2, block2, …]`，块原样返回 |
| `internal/cli/cli.go` | 新增 `batchStats`：`pathBatch` 在 `--stat/--shortstat` 且后端实现 `StatLog` 时取一次 map（`want` = 本次所有 action id）；`pathDocOf` 多一个可选 `stats`：命中查表、未命中回退 `ShowStat`；单条 `path` 与 `show` 不传 map |
| `internal/cli/cli_test.go` | `fakeRunner` 实现 `LogStats`（带调用计数/错误注入）；新增 4 个测试：批量只调一次 `LogStats`、无 `StatLog` 时文档相同、批量失败回退、部分未命中回退 |
| `docs/perf-audit.md` | 本文件 |

### 等价性（验收第一条）

改动前用旧二进制保存 6 份输出，改动后逐字节对比，**全部相同**：

| 输出 | 字节 | 相同 |
| --- | --- | --- |
| `path --all --json --stat` | 188,634 | ✅ |
| `path --all --json --shortstat` | 151,546 | ✅ |
| `path --all --json`（无 stat） | 134,734 | ✅ |
| `path <id> --json --stat` | 5,043 | ✅ |
| `path <id> --stat`（文本） | 3,562 | ✅ |
| `show <id> 1 --stat --json` | 4,760 | ✅ |

### 性能（同一会话，旧/新二进制背靠背）

| 调用 | 改前 | 改后 | 倍数 |
| --- | --- | --- | --- |
| `path --all --json --stat` | 14,490 ms | **296 ms** | 49× |
| `path --all --json --shortstat` | 14,446 ms | **299 ms** | 48× |
| `path --all --json`（无 stat） | 110 ms | 110 ms | — |
| 合成仓 4000 动作 `--stat` | （按动作外推约 200 s） | **220 ms** | — |
| git 进程数（真仓 282 动作） | 283 | **2**（log + log --stat） | 141× |

改后的 296 ms ≈ 启动 46 + `git log` 55 + `git log --stat` ~190。`--stat` 相对无 stat 只多约 190 ms，且不再随动作数增长（4000 动作只多 68 ms，因为空提交没有 diff 可算）。

### 回退与边界

- **单条 `path <id> --stat`、`show --stat` 不变**：仍逐动作 `ShowStat`（本仓 6 动作约 470 ms；动作少时批量反而多扫历史）。
- **jj 不实现 `StatLog`**：走回退；有测试证明「无 StatLog 的后端」产出与批量路径**逐字节相同的文档**。
- **批量调用出错或 map 未命中**（如合并提交）：逐动作回退，单条失败仍进 `errors.stat-failed`、exit 1——语义与改动前一致。
- **代价**：`git log --stat` 为所有提交算 diff，混合仓会为 non-action 提交做无用功；接受，仍是单进程。真出现“大历史/少量 action”再按比例选策略。
- **仍有两个进程**（log + log --stat，约多 55 ms）；合并成一次带 stat 的 log 要动模型来源，留给后续。

### 验收

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过；新增 4 个测试覆盖批量/回退。
- 真仓与合成仓计时如上；进程数用 `GIT_TRACE2_EVENT` 实测 2。
- `kiln check`：55 ok / 8 warn / 0 error。
- JSON 契约未变（`stat` 仍是后端原始 diffstat），dsh-kiln 不受影响。

## apply（2026-10-27，续）：`status` 与 `commit` 的固定开销

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | 新增可选接口 `AreaHint { MaybeHasAreas() bool }`：false = 后端**能证明**没有 linked area，调用方可以不 spawn `ListWorkspaces`；true = 不确定，照旧调用 |
| `internal/vcs/git/git.go` | `MaybeHasAreas` 检查 `<checkout>/.git/worktrees`（向上找 `.git`/`.jj`）：不存在 → false；`.git` 是文件或不可读 → true（保守）。`Changes` 去掉 `rev-parse --show-toplevel`：porcelain `-z` 本身输出 root-relative 路径，从子目录调用也不变 |
| `internal/cli/cli.go` | `knownAreas` 在 hint 为 false 时直接返回空；`write` 的空动作提示改用 `Changes` 的结果判断干净，不再额外 `git status`；`renderWorktreeLegend` 改为 0 个 dedicated area 时不打印 |
| `internal/cli/cli_test.go` | 新增 3 条：hint=false 不调 `ListWorkspaces`、hint=true 仍调、commit 不再调 `Dirty`；`workspaceRunner.Changes` 与 `dirty` 保持一致（dirty 就列出一个路径） |
| `internal/vcs/git/git_test.go` | 新增 2 条：子目录 `Changes` 仍 root-relative、`MaybeHasAreas` 对普通仓/有 worktree/站在 worktree 里三种情况 |

### 顺带修的一个小 bug

`status` 原来只要 `git worktree list` 返回了 main 就会打印图例 `■ = 已开 worktree——…`——等于每个 git 仓末尾都多一行“已开 worktree”，即使一个 area 都没有；而 `TestStatusStatesWorktreeOffWithNoAreas` 的意图正是“没 area 没图例”。现在按 `areasOpen`（非 main）判断，零 area 不再打印。**只影响文本，`status --json` 逐字节不变。**

### 性能（同一会话，旧/新二进制背靠背）

| 调用 | 旧 | 新 | 变化 |
| --- | --- | --- | --- |
| `status` | 154 ms | **106 ms** | −31% |
| `status --json` | 152 ms | **105 ms** | −31% |
| `commit`（空动作） | 501 ms | **337 ms** | −33% |
| `commit`（有改动） | 387 ms | 341 ms | −12% |
| git 进程数（status） | 2 | **1** | −1 |
| git 进程数（commit） | 7（+git 自身 maintenance） | **5**（+maintenance） | −2 |

`status` 少的是 `git worktree list --porcelain`；`commit` 少的是 `rev-parse --show-toplevel` 与空动作分支里的第二次 `status --porcelain`。

### 输出核对

- `status --json` / `list --json` / `check --json` / `graph --json` 与旧二进制**逐字节相同**；
- `status` 文本唯一差异是上面那行图例（0 area 时不再打印）；
- `commit` 的 trailer、清单（`将提交 N 个文件`）、空动作提示与退出码不变；两个全新仓里同样输入提交，stdout 除“已提交 <短 hash>”外完全相同。

### 边界

- `MaybeHasAreas` 只对 git 实现；jj 没有该接口，`status` 仍调用 `jj workspace list`，行为不变。
- 在 linked worktree 里（`.git` 是文件）返回 true，保守调用，不猜。
- `commit` 在 `worktree=on` 时仍会为 `refuseInMain` 调 `IsDedicated`（一次 `rev-parse`），未动。
- 真实仓现在没有 linked area；有 area 时 hint 为 true，走原路径，图例与计数照旧。

### 验收

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过；
- `kiln check`：56 ok / 7 warn / 0 error；
- 旧/新计时与进程数如上。

## collect（续，2026-10-27）：规模上限——8k / 16k / 100k

### 方法

- 合成仓用 `git fast-import`，**1 提交 = 1 issue = 1 action**（空提交），规模 8k / 16k / 100k；
- 另造 mixed 仓：**16k 提交、只有 1000 个 action**（其余是无 trailer 的普通提交），用来分开“总提交数”与“issue 数”；
- 每项 2–3 次取中位；`commit` 用新 issue 名避免起点动作拒绝；`path --all` 的峰值内存用 `Start-Process` 轮询 `PeakWorkingSet64`。

### 增长表

| 规模 | status | list --json | check --json | graph --json | search | show | path --all | path --all --stat |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 8k | 170 | 197 | 188 | 182 | 170 | 212 | 212 | 312 |
| 16k | 241 | 266 | 300 | 307 | 243 | 262 | 336 | 461 |
| 100k | **967** | **1150** | **1317** | **1139** | **840** | **851** | **1570** | **2087** |
| mixed 16k 提交 / 1000 issue | 210 | 198 | 196 | 199 | 196 | 245 | 202 | 323 |

`commit`：16k 约 450–540 ms，100k **1500 ms**。

### 体量与内存（100k）

| 输出 | 字节 |
| --- | --- |
| `path --all --json` | **35.2 MB**（350 B/action） |
| `list --json` | 17.3 MB（170 B/issue） |
| `status --json` | 14.2 MB（142 B/issue） |

`path --all --json` 的**峰值工作集 329 MB**（含模型与整份文档）。

### 观察

- **全部是线性**，每项每 issue 约 **10–15 µs**；没有超线性项。
- **随总提交数（不是 issue 数）增长**：每条命令的 log+建模（约 10–13 µs/提交）；`show` / `commit` 每次全史扫描（100k 时 851 ms / 1500 ms）；`--stat` 为**所有**提交算 diff（mixed 仓只有 1000 issue、16k 提交仍需 323 ms）。
- **随 issue 数增长**：输出体量（path --all 350 B/action）、`check` 的规则、`graph` 的边、`search` 的 haystack。
- **不增长**：git 进程数（status 1、commit 5、path-all-stat 2）、启动约 46 ms。
- 最先撞到的是**内存与输出体量**：100k 动作 = 35 MB 文档 + 329 MB RSS；时间在 1–2 s 量级，仍是可用范围。

### 边界

- 100k 是空提交；真实提交带 diff 时 `--stat` 更慢，`git log` 的消息体积也更大。
- 峰值内存不含 git 子进程。
- mixed 仓的普通提交没有 trailer，因此不产生 issue/action，但仍在 log 与模型构建里。

### 下一步（待 hypothesize / test）

- 低风险：`path --all` 流式编码，避免整份文档与 stats 常驻内存；
- `search` 的 haystack 不逐 issue 拼大字符串；
- `show` / `commit` 的全史依赖需要索引或缓存，与无索引设计冲突，暂不做。

## test（2026-10-27）：`path --all` 流式 JSON

### 改了什么

- `pathBatch` 不再先构造整份 `pathBatchDoc`，而是逐 issue 编码写出：
  - 新增 `pathBatchWriter`：64 KB `bufio.Writer` + **复用一个** `json.Encoder` 与 scratch buffer（100k 次编码不为每条分配 encoder）；
  - 输出字节与 `writeJSON` 完全一致（键序、2 空格缩进、不转义 HTML、结尾换行）；
  - `errors` 很小，仍缓冲到末尾再写；空 `issues` 写 `[]`。
- 单条 `path`、`show`、`list`、`status`、`check`、`graph` 仍走 `writeJSON`，不动。

### 等价性

用旧二进制保存 5 份输出，新二进制逐字节对比，**全部相同**：

| 输出 | 字节 | 相同 |
| --- | --- | --- |
| 真仓 `path --all --json` | 136,540 | ✅ |
| 真仓 `path --all --json --shortstat` | 153,551 | ✅ |
| `path <id> <id> --json`（重复 id） | 3,503 | ✅ |
| `path nope1 nope2 --json`（只有 errors） | 173 | ✅ |
| 16k 仓 `path --all --json` | 5,610,447 | ✅ |

新增两个测试：流式输出与 `writeJSON` 对同一批文档**逐字节相同**；只有 errors 的 batch 的精确文本。

### 效果

| 规模 | 指标 | 旧 | 新 |
| --- | --- | --- | --- |
| 真仓 62 issue | 时间 | 107 ms | 109 ms（噪声内） |
| 16k issue | 时间 | 339 ms | 336 ms |
| 16k issue `--shortstat` | 时间 | 421 ms | 411 ms |
| **100k issue** | 时间 | 1,055–1,075 ms | **1,114–1,223 ms**（+5–10%） |
| **100k issue** | **峰值内存** | **330 MB** | **183–201 MB（−40%）** |
| 100k `--stat` | 峰值内存 | 265–299 MB | 199 MB |

- 内存下降来自不再持有整份文档：100k 个 `pathDoc` 结构体 + 35 MB JSON 一起消失，只剩当前 issue 与（`--stat` 时的）stats map。
- 时间小幅上升是 **100k 次小 JSON 编码**的固定成本（每次约 1 µs）；bufio 与复用 encoder 已经压到最低。真仓与 16k 上看不出差别。
- 峰值仍随模型增长（模型本身 O(提交数)，无法靠输出层解决）；`--stat` 的 stats map 也仍在。

### 边界

- 只影响 `path --all` / 多 id 的 batch；单条命令仍是原路径。
- 写错误语义不变：写失败打印 `输出 JSON 失败` 并 exit 1（可能已写出半份文档，与旧行为相同）。
- 流式后的输出仍是一份合法 JSON，只是产生方式不同。

### 判定（留给 conclude）

- **建议保留**：100k 时内存是更硬的约束（330 → 190 MB），时间代价 5% 且只在极端规模出现；真仓无差别。
- 若以后更在意大 batch 的延迟，可加“超过阈值才流式”的双路径——代价是两条路径都要测试，暂不做。

## 结论（2026-10-27）

### 一句话

不引入索引；把 **“每次都要 spawn 的固定成本”** 与 **“按动作 spawn”** 这两处常数压掉之后，真仓（63 issue / 288 动作）读命令回到 **0.10–0.17 s**、`path --all --stat` 从 **14.5 s 降到 0.31 s**；100k 动作仍在 **1–2 s / 190 MB** 量级。**不设硬性性能预算**——只记录规模拐点、设计边界与再评估的触发条件。

### 最终数字（真仓 63 issue / 288 动作）

| 命令 | 现在 |
| --- | --- |
| `status` / `status --json` | 106 ms |
| `list --json` | 106 ms |
| `check --json` | 106 ms |
| `graph --json` | 107 ms |
| `path --all --json` | 114 ms（136 KB） |
| `show <id> <n>` | 168 ms（2 个 git 进程） |
| `path --all --json --stat` | **313 ms**（2 个 git 进程） |
| `commit`（空动作 / 有改动） | 337 ms / 341 ms |

100k 动作（合成仓）：读命令 0.84–1.32 s，`path --all --json` 1.14 s / 35 MB / 峰值 190 MB，`path --all --stat` 2.1 s，`commit` 1.5 s。

### 三批落地

| 提交 | 内容 | 关键收益 |
| --- | --- | --- |
| `611c272` | 批量 `--stat` 改成一次 `git log --stat` | 14.5 s → 0.30 s（49×），283 → 2 个进程 |
| `e80c092` | `status` 跳过无 area 时的 worktree list；`commit` 去掉 toplevel 与重复 status | status 154 → 106 ms；commit 501 → 337 ms |
| `2afb028` | `path --all` 流式 JSON（bufio + 复用 encoder） | 100k 峰值 330 → 190 MB（−40%），输出字节不变 |

### 仍然随规模增长的（观测，不是预算）

1. **O(总提交数)**：每条命令的 `git log` + 解析 + 建模，约 10–13 µs/提交；`show` 与 `commit` 每次都要全史扫描（100k 时 0.85 s / 1.5 s）；`--stat` 为所有提交算 diff（mixed 仓 16k 提交 / 仅 1k issue 也要 0.32 s）。
2. **O(issue/动作数)**：输出体量（`path --all` 约 350 B/动作、`list` 约 170 B/issue），以及 `check` 的规则、`graph` 的边、`search` 的 haystack；100k 时分别是 35 MB / 17 MB 文档。
3. **不增长**：git 进程数（status 1、commit 5、path-all-stat 2）、启动约 46 ms。

### 设计边界（接受的代价）

- **无索引**：`show` / `commit` 的全史成本是设计带来的，不打算用索引/缓存/daemon 换；如果将来长历史上确实疼，再单独立项。
- **`--stat` 批量**：以“为所有提交算 diff”换“一次进程”。混合仓会为 non-action 提交做无用功，但仍是单进程，比按动作 spawn 低一个量级。
- **流式只覆盖 batch**：时间换内存的取舍（100k 时 +5–10% 时间、−40% 内存）；单条命令仍是 `writeJSON`。
- **status 图例**：0 个 dedicated area 时不再打印（原先只要列表含 main 就打，属顺带修正）；`status --json` 不变。

### 明确不做

索引 / 缓存 / daemon / 常驻进程；分页与截断（`issue-list-scaling` 已论证否决）；“超过阈值才流式”的双路径；把 `git log` 与 `git log --stat` 合并成一次（收益约 10%）；并发 spawn。

### 再评估的触发条件（不是预算）

- 真实仓 **issue 数到 1k+**，或单次 `path --all` 输出 **>10 MB**；
- 有人抱怨 `show` / `commit` 在长历史上慢（先确认历史量级）；
- 消费方需要“前 N 条”/分页——那时再谈 `--limit` 与相关度，不推翻现有语义。

### 验收

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过；
- `kiln check`：55 ok / 8 warn / 0 error；
- 所有优化都验证过输出字节不变（`--stat` 六份、batch 四份、流式五份）；
- 真仓与合成仓计时、进程数、峰值内存均已记录在上文各节。
