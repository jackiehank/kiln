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
