# issue: perf-profile — 缓存之后，kiln 的卡点在哪

- **domain**：cli
- **关联**：relates-to `perf-audit`、relates-to `cache-perf`、relates-to `issue-list-scaling`

## 背景

2026-10-27 的 `perf-audit` 是在“没有索引、每条读命令现读 git log”的设计下做的：结论是读命令的地板是进程启动（约 100 ms）+ 一次 `git log`（约 50 ms），模型构建与渲染只占几十毫秒，规模到 4000 动作仍然近似常数。

此后设计变了两次：`commit-cache` 给读路径加了内容寻址的记录层与枚举快照，`cache-perf` 又把热读路径压到了 100k 动作 185 ms。但那次只测了缓存相关命令，且以“warm 读”为中心；`commit` / `show` / `path --stat` / 缓存维护这些路径没有重新测。`perf-audit` 里的数字因此**部分过期**（比如 `path --all --json` 的 223 ms、`status` 的 302 ms 都是缓存之前的口径）。

用户要求重新测一遍：**主要卡点在哪，怎么优化**。先测，再谈改什么。

## 要回答的问题

1. **绝对耗时**：9 条读命令（`status` / `list` / `check` / `graph` / `path` / `show` / `action log` / `worktree list` / `do` 预览）在真仓、1k / 10k / 100k 合成仓各多少？冷、热分开。
2. **地板**：进程启动（`--version`、空 Go 程序）占多少？剩下的时间里，读缓存、建模、渲染 / JSON 编码各占多少？
3. **子进程账本**：每条命令（冷 / 热 / `--stat` / `--json`）实际 spawn 几个 git 进程？哪些路径还在走冷路径？提交之后第一次读要付多少？
4. **写路径**：`commit` 的固定开销是多少、由哪几个进程构成？`do` 与 `commit` 的差在哪？
5. **维护成本**：层的写入、合并（≥64 层）、快照写入与淘汰、`cache gc` / `cache clear`，各是多少毫秒？会不会成为新的地板？
6. **大输出**：100k 时 `list --json` 十几 MB，编码 / 写 pipe / 分配是否已经取代解析成为瓶颈？
7. **优化候选**：按“收益 ÷ 风险”排序；哪些与设计约束冲突（无索引、输出逐字节不变、失败静默）；哪些是这轮就该做的。

## 候选（测完再选）

- **A 只记录**：数字写清楚，不做改动；
- **B 低风险热修**：去掉重复工作 / 大分配，不改契约；
- **C 结构性改动**：需要单独立项（格式 v2、并发、常驻进程等）。

## 非目标

- 不为性能改文本视图 / JSON 契约 / 排序；
- 不引入索引、数据库、daemon；
- 不在测量前先做并发；
- 不优化没有被测量证明的路径。

## 验收（collect 完成时）

- 真仓与三个合成仓的冷 / 热耗时表（≥5 次，中位数）；
- 每条命令的 git 子进程数与冷 / 热分解；
- 100k 热读的 pprof 阶段占比（解码 / 组装 / 渲染 / 编码 / 启动）；
- 写路径与缓存维护的成本表；
- 热点排序 + 候选（必须改 / 可选 / 不做）。

## 采集（2026-10-30）：本机实测

### 方法与口径

- **机器**：Windows 11，13th Gen i5-13600K，Go 1.27，git 2.55.0.windows.3。这台机器的**进程创建与 git 启动都很贵**（空 Go 程序 17 ms，`git --version` 约 60 ms），绝对数字带本机 Defender 的印记；结构性结论（谁随规模长、谁固定）不依赖它。
- **计时**：`nulrun`（一个 20 行的小启动器）把子进程的 stdout/stderr 接到 NUL 再计时。**报的是 kiln 自己的墙钟**，不含调用方读 pipe 的成本。上一轮 `cache-perf` 用的是 PowerShell 读 pipe 的口径，同样命令会多 15–80 ms（5–21 MB 输出时更多），两组数不可直接相减。
- **预热**：每条命令先跑一次再取 5 次中位数；冷路径每次删 `.git/kiln` 再跑，取 3 次中位数。
- **合成仓**：`git fast-import` 造，1 提交 = 1 issue 的一条动作，每 4 条有 1 条非 kiln 提交；1k / 10k / 100k 提交分别 1000 / 10000 / 100003。
- **分解**：这台 Go 发行版**没有 `go tool pprof`**（与 covdata 一同被裁）。阶段占比来自两处：临时构建里按 `KILN_PHASES` 打的毫秒标记（收集完即删除，未提交），以及 `go test -bench` 直接调 `loadCached` / `cache.Scan` / `Run`。两者与整进程数字能对上（±10 ms）。
- **噪声**：本机有 qemu 虚拟机常驻，单次读数波动 ±10–25 %，只信中位数与重复出现的差值。

### 1. 地板：一次调用花在哪

| 程序 | 中位数 (ms) | 说明 |
| --- | --- | --- |
| 空 Go 程序（1.8 MB） | 17 | 纯进程创建 + runtime |
| `kiln help` | 27 | 再加配置读取、i18n、usage（无 git） |
| `kiln do <issue> collect` | 27 | 同上，预览不读历史 |
| `kiln --version` | 47 | **+20 ms = `time.Local` 初始化**（见下） |
| `kiln`（`-s -w` 构建，3.86 MB） | 23 | 只剥符号/DWARF，行为不变 |

- 地板的大头是**加载 5.7 MB 的镜像**（同尺寸的 Go 程序地板 29 ms；1.8 MB 的只要 17 ms）。`-s -w` 把体积压到 68 %、地板降 4 ms（23 vs 27），其余不变。
- `--version` 的 +20 ms 不是 `ReadBuildInfo`：阶段标记显示它落在 `st.ModTime().Format(...)` 这一行——**Go 在 Windows 上第一次用 `time.Local` 会枚举注册表的时区表**（`zoneinfo_windows.go`），约 20 ms。`TZ=UTC` 无效（该实现不看 `TZ`）。
- 同一笔钱还要在 `action log` 的文本视图付一次：`when.Local()`。所以文本比 `--jsonl` 稳定多 20 ms（真仓 50 vs 30，1k 53 vs 30，10k 64 vs 44），而 `--jsonl` 不碰 `Local`。

### 2. 真仓（79 issue / 426 动作）：缓存已经把它压到地板

| 命令 | 热 (ms) | 冷 (ms) |
| --- | --- | --- |
| `status` | 29 | 113 |
| `status --json` | 31 | 112 |
| `list` | 29 | 111 |
| `list --json` | 29 | 111 |
| `list --search=zzz` | 30 | 113 |
| `check` | 29 | 112 |
| `check --json` | 30 | 113 |
| `graph` | 29 | 112 |
| `graph --json` | 29 | 114 |
| `path --all --json` | 30 | 113 |
| `show <id> 1` | 82 | 164 |
| `show <id> --json` | 89 | 170 |
| `action log --limit 3` | 52 | 97 |
| `action log --jsonl` | 30 | 107 |
| `action log` | 50 | 107 |
| `worktree list --json` | 76 | 77 |
| `do <issue> collect` | 28 | 27 |

- **热读 = 地板**：29–31 ms，其中真正的读只有 1–3 ms。剩下的可变量全是 git 子进程（`show` +53、`worktree list` +48）。
- **冷读 111 ms**：地板 27 + 一次 `git log`（带 body，约 55）+ 解析 / 写层 / 建模（约 28）。缓存替真仓省下 **约 80 ms / 次**。
- `action log --limit 3` 冷 = 97：它不读全史，流式取到前 3 条就杀进程。

### 3. 规模：1k / 10k / 100k

热（中位数 ms）：

| 命令 | 1k | 10k | 100k |
| --- | --- | --- | --- |
| `status` | 31 | 39 | 153 |
| `status --json` | 30 | 40 | 164 |
| `list` | 29 | 41 | 146 |
| `list --json` | 30 | 41 | 158 |
| `list --search=zzz` | 29 | 40 | 144 |
| `check` | 29 | 39 | 152 |
| `check --json` | 30 | 41 | 166 |
| `graph` | 30 | 40 | 144 |
| `graph --json` | 29 | 41 | 163 |
| `path --all --json` | 31 | 48 | 248 |
| `show <id> 1` | 81 | 89 | 185 |
| `show <id> --json` | 82 | 91 | 179 |
| `action log --limit 3` | 51 | 135 | 133 |
| `action log --jsonl` | 30 | 44 | 182 |
| `action log` | 53 | 64 | 179 |
| `worktree list --json` | 78 | 78 | 78 |
| `do <issue> collect` | 28 | 27 | 27 |

冷（中位数 ms）：

| 命令 | 1k | 10k | 100k |
| --- | --- | --- | --- |
| 读命令（`status`/`list`/`check`/`graph`） | 114–120 | 198–210 | 1071–1112 |
| `path --all --json` | 118 | 210 | 1151 |
| `show <id> 1` | 166 | 253 | 1113 |
| `action log --jsonl` | 106 | 159 | 1052 |
| `action log --limit 3` | 103 | 134 | 97 |

- **10k 是分水岭**：1k 的热读还在地板上，10k 开始 +10 ms，100k 是地板的 5 倍。
- `path --all --json` 100k 的 248 ms 里有约 83 ms 是**输出本身**（20.9 MB），见第 4 节。
- `worktree list` 与 `do` 与规模无关（78 / 27）。
- **10k 的 `action log --limit 3` = 135 ms**：这个仓没有 author-date 快照，`--limit` 回退为流式全走（1 个 git 进程 ~85 ms）+ 时区 20 ms。跑一次完整的 `action log` 之后它会降到 44（10k）。

### 4. 100k 热读：127 ms 里是什么

`list --json`：进程总 158 ms = 地板 27 + Run 127（阶段标记口径），输出 5.3 MB。

| 阶段 | ms | 怎么来的 |
| --- | --- | --- |
| 读层 + 解码（`cache.Scan`） | 18 | 12 MB / 10 万条记录 |
| 组装模型（`gaps` + `assemble` + `Builder`） | 65 | 10 万提交 → 7.5 万动作 / 2.5 万 issue |
| 渲染 / JSON 编码 | 25 | 5.3 MB |
| 其它（枚举查表、settle） | ~19 | 快照命中无 git |
| **Run 合计** | **127** | |

- 进程内的 `Run` 是 108 ms（bench，10 次平均），新进程的 Run 是 127——多出的 ~20 ms 是首次触页与 GC 从零开始；`GOGC=400` / `off` 都不改变这个数（110–122 ms），说明是分配量不是 GC 调参。
- 一次调用分配 **175 MB / 54 万次**（`list --json`）；其中层解码 48 MB，建模 66 MB，编码 61 MB。
- 同类命令：`status --json` 113、`graph --json` 111、`path --all --json` 166、`action log --jsonl` 108、`action log` 89、`action log --limit 3` 26（只解码 + 取前 N 条，不建模）。
- **输出体量**（100k）：`list --json` 5.3 MB、`graph --json` 3.8 MB、`status --json` 4.5 MB、`path --all --json` 20.9 MB、`action log --jsonl` 13.5 MB、`list` 文本 1.4 MB。调用方读 pipe 的成本按体积另算（上一轮口径里 20.9 MB 要多 ~50 ms）。

### 5. 100k 冷读：1.08 s 里 70 % 是 git

| 阶段 | ms |
| --- | --- |
| `git log`（带 body，100 003 提交） | 756 |
| 解析 trailer + 生成 records | 57 |
| 写层（12 MB）+ 写枚举快照（2.1 MB）+ 合并检查 | 206 |
| 组装模型 | 60 |
| **合计 Run** | **1079** |

- 直接测 git：`git log --format=%H` 166、带 body 的格式化 log 716、`git cat-file --batch` 启动 50、`rev-list HEAD` ~150。
- 所以冷路径**没有多少自留地**：想再快只能少读 git，而那正是缓存在做的事。写层 206 ms 里含 12 MB 编码 + SHA-1 + 落盘，可做成异步，但冷路径本来就慢，优先级低。

### 6. 写路径：`do` 很快，`commit` 是进程税，`ask` 例外

| 操作 | 1k | 100k | 说明 |
| --- | --- | --- | --- |
| `do <issue> collect` | 28 | 27 | 不读历史 |
| `do <issue> ask` | 105 | **861** | 起点重复检查走 `findIssue` → 全量 `git log` |
| `commit <issue> collect` | 287 | 287 | 4 个 git 进程，与仓库规模无关 |
| `commit <issue> ask` | 408 | **1138** | 上面 + 全量 `git log` |

- `commit` 的进程账本（trace2 实测）：`status --porcelain -z`、`add -A`、`commit --allow-empty -m`、`rev-parse HEAD`，外加 git 自己 detached 的 `maintenance run --auto`。**287 ms 基本就是 4 × 50 ms 的进程税 + commit 自身**。
- `do ask` / `commit ask` 的 861 / 1138 ms 是**纯浪费**：`refuseDuplicateStart` 为了回答“这条 issue 是否已有起点动作”，直接 `backend.Log()` 读了全史（100k 时 756 ms），而缓存里本来就有答案。

### 7. 维护与“提交后第一次读”

| 操作 | 1k | 100k |
| --- | --- | --- |
| `cache gc` | 86 | 503 |
| 提交后第一次读（`list --json`） | 137 | 439 |
| 提交后第二次读 | 30 | 158 |
| `cache clear` | < 5 | < 5（删文件） |

- **每次提交都会让枚举快照失效**（快照按 HEAD 寻址），下一条读命令要重新 `git log --format=%H`（1k ~75、100k 166）并补读新提交（`cat-file`，~50）。于是 1k 多付 ~107 ms、100k 多付 ~280 ms。真仓按同样两个分量估 ~100 ms（52 + 48）。
- `cache gc` 503 ms@100k：可达性（`rev-list`）+ 解码所有层 + 扫目录。低频手动命令，可以接受。

### 8. 子进程账本（trace2 实测）

| 路径 | git 进程 | 是什么 |
| --- | --- | --- |
| 热读 `list` / `status` / `check` / `graph` / `path --all` | 0 | 快照 + 层 |
| `show` / `show --stat` | 1 | `git show` |
| `path --all --stat` / `path <id> --stat` | 1 | `git log --stat` 一次过（`perf-audit` 的批量化仍然成立，**不是**每动作一个） |
| `worktree list` | 1 | `worktree list --porcelain` |
| `status`（worktree=on 且已有区） | +1 | 每个区一次 |
| `do collect` | 0 | |
| `do ask` / `commit ask` | +1 | `git log` 全量（重复检查） |
| `commit` | 4 | status / add / commit / rev-parse（+ detached maintenance） |
| `action log --limit`（无快照） | 1 | `git log --author-date-order` 流式 |
| `cache gc` | 1–2 | `rev-list` |

### 9. 卡点排序

1. **冷路径的 `git log`**：100k 占 70 %（756 / 1079），真仓 ~55 / 111。这是“无索引”的结构成本，只能靠缓存摊薄；缓存 miss 时它一次付清。
2. **可修浪费 A — `do`/`commit` 的起点检查绕开缓存**：100k 861 / 1138 ms，1k 105 / 408，真仓 106。缓存明明有答案（热读路径 127 ms 就能建出全模型），却去读了 756 ms 的原始历史。
3. **可修浪费 B — 每次提交后枚举快照失效**：100k +280 ms，1k +107 ms，真仓 ~+100 ms。这是“提交 → 读一眼”的工作流里每一步都要付的钱。
4. **可修浪费 C — `action log --limit` 无 author-date 快照时回退流式**：10k 135 ms vs 有快照时 44；真仓冷 97。修 B 的同一机制可以顺带覆盖它（提交时把两种顺序都长出新的快照）。
5. **100k 热读的 127 ms**：建模 65 + 编码 25 + 解码 18。再往下压要么改格式（v2 偏移表，让 `--limit` 只解码用得到的那几块），要么把关系折叠惰性化（`list`/`status`/`check` 不展示关系图）。
6. **固定成本**：地板 27（`-s -w` 可到 23）、Windows 时区 20（只影响 `action log` 文本与 `--version`，没有不改变输出的修法）。
7. **低频**：`cache gc` 503 ms@100k；`commit` 的 4 个进程（287 ms）是进程税，合并到 2–3 个需要改后端接口，收益有限。

### 10. 结论

- **真仓已经到底**：热读 29 ms，其中 kiln 自己的活只有 1–3 ms；地板是镜像加载，压不动。
- **小仓（1k）也到底**：热读 29–30 ms；冷读 115 ms 是一次性重建。
- **100k 的热读从“现算模型”换成了“解码 + 重建模型”**：127 ms，与动作数线性，但没有超线性。
- 真正还能拿走的三笔，都在**绕开缓存**的地方：`do/commit ask` 的全量 log（A）、提交后的枚举失效（B）、`--limit` 的流式回退（C）。B 和 C 是同一个机制的两面。

## 结论（2026-10-30）：只记录，不动代码

决定：把数字与排序留在文档里，代码留到有明确需求时再立项。理由：B 要新增“提交时增量写快照”的不变量（涉及改写 / 非线型历史），C 依附 B，都属于需要单独证明的改动；A 虽然低风险，但 `ask` 每条 issue 只发生一次，没有证据表明它正在造成实际困扰。

**真仓补充实测**（`8d5a7c0` 提交后立刻测，同一口径，各 5 次）：

- 提交后第一次 `list --json` = **140 ms**（热读 29 ms），多付 111 ms；
- 没有 author-date 快照的 `action log --limit 3` = **101–105 ms**（有快照 52 ms）。

**如果以后要做**，按这个顺序，各自的目标与验收：

1. **A — 起点检查走缓存**：`findIssue` 改用 `loadCached`，只在缓存不可用时回退 `Log()`；输出与拒绝语义不变。目标：100k `do ask` 861 → 约 150、`commit ask` 1138 → 约 420；冷路径不变。
2. **B — 提交时增量写快照**：提交前后从文件系统读 HEAD，新提交前插进两种顺序的快照；只在线性历史成立，改写 / 非线型一律回退。目标：真仓提交后首读 140 → 约 40，100k 439 → 约 160。验收要求逐字节等价 + 回退用例。
3. **C — `--limit` 搭 B 的便车**：快照命中后直接取行；仍无快照时，可给流式加 `-n` 提示，但必须先证明输出不变。

**顺带确认 / 修正的旧结论**：

- `path --all --stat` 已是 1 个 git 进程（`perf-audit` 的批量化成立），不再按动作 spawn；
- `perf-audit` 里的绝对数字（`status` 302、`path --all --json` 223）是缓存之前的口径，已过期；
- 上一轮 `cache-perf` 报的 100k `list --json` 185 ms 含 PowerShell 读 pipe 的成本；同一命令本口径是 158 ms，两组数不可相减。

## apply（2026-10-30）：A — 起点检查走缓存

这一条推翻了上节结论里的 A：收集完成后要求先修它，B / C 仍按结论保留。

### 改了什么

`findIssue`（`do` / `commit` 的起点重复检查的唯一调用点）先问记录层：`loadCached` 能服务就用它建出的模型回答“这条 issue 有没有起点动作”；缓存不可用（冷、损坏、测试里被关断、后端不支持）时才回退到原来的 `backend.Log()` 全量扫描。没有新接口，也没有改“best effort”的契约——读不到仍然是“没有意见”，从不报错。

### 等价性

- 新增 `TestDuplicateStartCheckAnswersFromTheCache`：暖好层与枚举后把 `PATH` 指向空目录（git 不可达），必须仍然拒绝重复起点、并放行没有起点动作的 issue。**旧代码上这个测试先失败**（`duplicate start: exit 0, stderr ""`）：检查绕开缓存去 spawn git，失败后静默返回“没有意见”。
- 真仓 `do cache-perf ask`：暖缓存与 `cache clear` 之后逐字节相同（同一段拒绝文本、exit 1）。
- 全量 `go test ./...` 绿；仓库外 `do foo ask` 仍打印“不在仓库里”的提示。

### 性能（同机、同口径，中位数）

| 路径 | 改前 | 改后 |
| --- | --- | --- |
| `do <issue> ask` 真仓 | 106 | **29** |
| `do <issue> ask` 1k | 105 | **28** |
| `do <issue> ask` 100k | 861 | **122** |
| `commit <新 issue> ask` 1k | 408 | **280** |
| `commit <新 issue> ask` 100k | 1138 | **373** |

100k 的 `do ask` 现在就是一次热读的成本（地板 27 + `loadCached` 约 85 + 一行输出）；`commit ask` 剩下的是 4 个 git 进程的进程税（约 280 ms），与仓库规模无关。

### 边界

- **冷缓存仍走 `Log()`**：`cache clear` 之后第一次 `do ask` 与改前一样，并且不会顺手写缓存（`findIssue` 的回退不 persist）。
- `loadCached` 自己会 `settle`（补 gap 层、按需合并），所以暖缓存下的 `do` / `commit` 也可能顺手写一层——与读命令的行为一致。
- 拒绝语义、提示文本、退出码都没有变；变的只是答案从哪来。

### 验收

- [x] 暖缓存 + 无 git 仍能回答重复起点（新测试，先证伪）；
- [x] 真仓冷 / 暖输出逐字节一致；
- [x] 全量测试绿、`gofmt` / `go vet` 干净；
- [x] 100k 收益 ≥ 5×（861 → 122），真仓回到地板（106 → 29）。

## collect（续，2026-10-30）：哪些读取不顺手写缓存

A 的边界里有一句“`findIssue` 的回退不 persist”。顺着它把问题放大：**哪些读历史的路径读完不留缓存**，代价有多大？答案是 4 条读历史的路径里有 3 条不写，而它们正好是“看动作 / 开单”的日常路径。

### 方法

- 1k 合成仓上做对照：清空 `.git/kiln` → 跑一条命令 → 数层（`layer-*.rec`）与枚举快照（`enum-*.oid`）；
- 真仓复现“反复调用也不变暖”的序列；
- 100k 上量冷 / 热两态（同机、`nulrun` 口径、中位数）。

### 1. 谁在写缓存（1k，每次先清空）

| 命令 | 读什么 | 留下的层 | 留下的快照 |
| --- | --- | --- | --- |
| `list` / `status` / `check` / `graph` / `path --all` / `show` | 全史（`git log` 带 body） | 1 | 1 |
| `action log`（文本） | 全史（`git log --author-date-order` 流式） | **0** | **0** |
| `action log --jsonl` | 同上 | **0** | **0** |
| `action log --limit N` | 同上，取够就杀进程 | **0** | **0** |
| `do` / `commit` 的起点检查（冷） | 全史（`Log()`） | **0** | **0** |
| `do` / `commit` 的非起点动作 | 不读史 | 0 | 0 |
| `worktree list` | 不读史 | 0 | 0 |

**只有“建模型”的那条路径顺手写缓存。** 流式读（`action log` 两种形状）和起点检查的回退都不写；读历史的 4 条路径里 3 条如此。

### 2. 不写的代价：反复调用也不会变暖

真仓，`cache clear` 之后连续调用：

| 序列 | 第 1 次 (ms) | 第 2 次 (ms) |
| --- | --- | --- |
| `action log --jsonl` ×2 | 105 | 109 |
| `do cache-perf ask` ×2 | 108 | 107 |
| 中间插一次 `list`（写缓存），再 `do ask` | 118（list，冷） | **28** |

100k，`do ask` 连跑两次，两次之间都清空缓存以证明它确实没写：

| 序列 | 第 1 次 | 第 2 次 | 留下的层 |
| --- | --- | --- | --- |
| `do ISS-99997 ask` | 864 | 864 | 0 |

### 3. 冷 / 热对照（中位数 ms）

| 路径 | 1k 冷 / 热 | 10k 冷 / 热 | 100k 冷 / 热 |
| --- | --- | --- | --- |
| `action log --jsonl` | 106 / 30 | 159 / 44 | **1026 / 174** |
| `action log`（文本） | 104 / 53 | 159 / 64 | **1035 / 179** |
| `do <issue> ask` | 110 / 28 | 183 / 36 | **864 / 122** |
| `action log --limit 3` | 103 / 51 | 134 / 135¹ | 97 / 84 |

¹ 10k 仓当时没有 author-date 快照，`--limit` 的“热”也走流式；跑过一次完整 `action log` 后会降到 44。

- `action log --limit` 是唯一“冷也不太贵”的：它取够行数就杀进程，git 只走了前几百个提交（100k 冷 97，甚至比暖态解整层的 84 接近）。真正的痛点是**完整流**与**起点检查**。
- `--limit` 冷的 97 ms 里仍有进程地板 27 + 时区 20；它不是“没写缓存”的主要受害者。

### 4. 这意味着什么

- 一个只做“看最近动作（`action log --limit`）+ 开单（`commit … ask`）”的会话，**永远不会让缓存变暖**：每次调用都付全额冷价。真仓是 ~105 ms/次（热态 30），100k 是 ~1 s/次（热态 ~0.18 s）。
- 缓存现在实际上是“**读过模型才存在**”的副产品；读历史但不建模型的路径是旁观者。
- 写缓存的成本已经摊在 `loadRepo` 里（面 4：100k 写层 + 快照 206 ms），所以问题不是“写不起”，而是“没接上”。

### 5. 候选（下一轮再选）

- **D — 起点检查写完再答**：`findIssue` 的冷回退已经有 `commits` 与 tip，照 `loadRepo` 补 `persistRecords` + `storeEnumFromCommits` 即可。低风险；100k 第二次 `do ask` 864 → 122，真仓 108 → 28。
- **E — `action log` 冷路径改“先建模型再渲染”**：首行变慢、内存上涨，但 100k 第二次 1026 → 174。
- **F — `action log` 边流边攒 records，结束后写层**：保住首行早出的卖点，代价是 100k 约 48 MB 常驻记录（层本身 12 MB）。
- **G — 不动**：接受“只读动作的会话每次全量读”，把 4 条路径的冷价写进文档。

## hypothesize（2026-10-30）：让“读完历史”自己种缓存

### 主张

缓存现在的存在条件是“有人读过模型”。把条件换成**“有人读完了历史”**：任何一条路径只要为了回答自己把全史读了一遍，就把读到的 records 与枚举写下去。这样“看动作 + 开单”的会话也能在第二次调用变暖，而不需要用户先跑一条 `list`。

主张分两半：

1. **D 先做**：起点检查读完就写。它便宜、契约干净，且它和 `loadRepo` 的冷路径本来就是同一件事，没有理由写两遍。
2. **E / F 是产品取舍**，不是技术问题：`action log` 的流式有两个卖点（首行早、内存平），“顺手写缓存”和这两个卖点直接冲突。要么牺牲首行（E），要么牺牲内存（F）。这需要先明确“谁在等首行”。

### 证据（来自上一节 collect）

- 4 条读历史路径，3 条不写：`action log` 文本、`action log --jsonl`、起点检查；唯一写的是建模型的那条。
- 不写的代价是**每次都付**：100k `action log --jsonl` 1026 ms（热 174）、`do ask` 864 ms（热 122）；真仓 105 / 108（热 30 / 28）。两次连着跑，数字不变，层数还是 0。
- 写缓存的成本已经测过：100k 冷路径写层 + 快照 206 ms；真仓约 3 ms。问题不是写不起。

### 方案对比

| 方案 | 第一次冷（100k） | 第二次（100k） | 首行延迟 | 额外内存 |
| --- | --- | --- | --- | --- |
| **D** 起点检查写 | `do ask` 864 → 约 1050 | **122** | 不变 | 无 |
| **E** action log 先建模型 | `action log` 1026 → 约 1084 | **174** | 50 ms → 1.1 s | 与 `list` 相同 |
| **F** 边流边攒 records | `action log` 1026 → 约 1250（收尾写层） | **174** | 不变 | 约 48 MB 常驻 |
| **G** 不动 | 不变 | 不变 | 不变 | 无 |

D 的“第一次变慢”是 206 ms 的写层成本，换来之后每次省约 740 ms（100k）/ 78 ms（真仓）。对 `ask` 这种每条 issue 只发生一次的动作，这笔账是划算的；`commit ask` 同享（冷 1138 → 第二次约 373）。

F 的 48 MB 需要对照现在的峰值：100k `list --json` 一次调用分配 175 MB、峰值堆约 120 MB；再加 48 MB 常驻不算破例，但**必须在 100k 上实测**，不能只靠推算。

### 契约（如果落 D）

- 输出、退出码、拒绝语义逐字节不变；唯一可观察变化是**冷跑之后缓存里多一层一快照**。
- 冷路径的失败仍然是静默的：写不了就不写，`do` 照常回答。
- `loadRepo` 与 `findIssue` 共用同一个冷路径函数（`loadUncached`），不允许两份“读完全史后写缓存”的代码各自漂移。

### 落点

- `internal/cli/load.go`：把 `loadRepo` 的冷分支抽成 `loadUncached(backend) (*model.Repo, error)`——`Log` → `ActionsAndRecordsFromCommits` → `persistRecords` → `storeEnumFromCommits` → `Build`。
- `internal/cli/write.go`：`findIssue` 的回退改调 `loadUncached`，错误吞掉（保持 best effort）。
- 测试放 `internal/cli/cache_test.go`。

### 检验办法（验收）

1. 新测试：空缓存仓里跑一次 `do <issue> ask`，层数必须从 0 变 1；随后把 `PATH` 指向空目录，第二次 `do ask` **仍能拒绝**——证明第一次把答案写下来了（这个测试比 A 的强：A 是“预热后无 git”，D 是“第一次自己预热”）。
2. 真仓：冷 108 → 第二次 28；100k：冷 864 → 第二次 122（第一次会到约 1050，如实记录）。
3. 全量测试绿；`gofmt` / `go vet` 干净。

### 反方（保留）

- `do` / `commit` 是写命令，突然开始写缓存文件（100k 12 MB），把“预览”变重了。回应：`do` 本来就要读全史做起点检查，读命令也早就这么做了；写失败静默。
- 第一次冷跑变慢 200 ms，可能让超大仓上的“开单”更慢。回应：`commit ask` 冷态本来 1138 ms，200 ms 在噪声下；而第二次省 742 ms。
- E / F 的取舍不该由实现者替用户决定：先做 D，再看真实会话里 `action log` 是不是主路径，再决定 E 还是 F。

## hypothesize（2026-10-30，续）：一个“边读边种”的读取组件

本节细化“共用读取组件、边读边缓存”的方案，取代上一节的 D / E / F：D 变成第 3 步，E / F 变成同一个组件里“什么时候落盘”的参数，不再是对立选项。

### 主张

1. **读历史的消费方式只有四种**（建模型、查一条 issue、逐行渲染、存枚举），但“读完就种缓存”只需要一处实现；
2. **分块写**让“首行早出”与“顺手种缓存”不再冲突——每块是一份合法的不可变层，读者不必等整条流结束；
3. 冷读时**写缓存的墙钟成本≈0**：git 在另一个进程里继续产出，消费者的 parse + encode 总量小于 git 的产出时间。

### 证据

- 100k 冷路径分解（上一节）：`git log` 756、解析 57、写层+快照 206、建模 60。
- 写一个 12 MB 层：`cache.Encode` **121 ms / 92 MB 分配**；`cache.Write`（含 SHA-1 与落盘）**124 ms**。
- 消费者一侧总量 ≈ 57（parse）+ 124（分块 encode/写）+ 60（建模）≈ 240 ms < 756 ms（git）。所以即使**同步**分块写，墙钟也主要被 git 的产出时间盖住；分块还把那次 92 MB 的编码峰值拆成每块约 10 MB。
- 反过来说：现在的 1026 ms 冷 `action log` 里，git 已经占了 756，加 124 的写盘只可能被“pipe 背压”放大——这是本方案第一个要实测的量（见检验）。

当前与目标（100k，中位数）：

| 场景 | 现在 | 目标 |
| --- | --- | --- |
| `action log --jsonl` 冷 | 1026 | ≤ 1100（写盘被 git 盖住） |
| 同上，第二次 | 174 | 174 |
| `list --json` 在 action log 之后首次 | 1084 | 约 300（补一次 166 的默认枚举 + 热读 122） |
| `do ask` 第二次 | 122 | 122 |
| 冷读峰值堆 | 约 174 MB/次调用 | 不高于现在（分块后编码峰值下降） |

### 约束：顺序与折叠（这决定组件能“统一”到什么程度）

- git 的**流**只有一种顺序：`LogEach` 固定 `--author-date-order`（新→旧）；默认序（commit-date）只有批量 `Log()` / `LogOIDs()`。
- `Builder` 要**旧→新**：`Domain`/`Tags` 折叠取最后一条，`Repo()` 的稳定排序在日期相同时保留插入序。冷路径正是把 batch 反转后再喂（`ActionsAndRecordsFromCommits` 倒序遍历）。
- 所以模型读取必须缓冲（它本来就要反转），**流式只能服务“新→旧”的 `action log`**。硬把模型路径塞进流式，要么多一次 166 ms 的默认枚举，要么丢掉折叠顺序——两者都不划算。

这条约束把“一个 walk 包打天下”修正为更诚实的版本：**一个缓存 walk + 一个分块 sink + 流式明确分叉**。

### 组件形状

```
// internal/cli/reader.go
type reader struct {
    cb     vcs.CacheBackend
    dir    string
    fp     [20]byte
    kind   enumKind
    layers []cache.Layer
    index  map[[20]byte]recPos
}

// openReader：扫描层、剪掉过期的，返回 nil 表示这条读只能走 git。
func (e *env) openReader(backend vcs.Repo, kind enumKind) (*reader, bool)

// walk：按枚举顺序逐条给 record；缺口用一次 Commits() 补齐；
// 新 record 交给 sink。complete 表示“走完了整个枚举”。
func (r *reader) walk(fn func(cache.Record) bool) (complete bool, err error)

// layerSink：把 record 攒成块，满了就 cache.Write（原子 rename）。
type layerSink struct{ dir string; fp [20]byte; chunk int; buf []cache.Record; wrote []cache.Layer }
func (s *layerSink) add(rec cache.Record)
func (s *layerSink) flush()
```

使用者：

| 调用方 | 顺序 | 缓存能答 | 缓存不能答 |
| --- | --- | --- | --- |
| `loadRepo`（模型） | 旧→新 | reader 倒序取 | `Log()` 批量 + sink（保持现状，只换落盘） |
| `findIssue`（D） | 旧→新 | reader 倒序取 | 同上，错误静默 |
| `action log` | 新→旧 | reader 顺序取 | `LogEach` 流式 + sink，边渲染边落盘 |

### 分块写入规则

- 按**记录数**分块，初值 16384 条（100k 时约 2 MB/块）；块大小是包内常量，不进配置。
- flush 时 `cache.Write`（内容寻址 + 原子 rename）；块**不声称覆盖**（tips=nil）——部分覆盖本来合法，合并时再并。
- 早停（`--limit`）：把手上这块 flush 掉再返回，**不写枚举快照**。
- 与已有层去重：流式时查 `index`，命中就跳过；写入的 OID 记进 `index`（同一流不会重复，但便宜）。
- 合并：整条 walk 结束后调一次 `maybeMerge`，不在流中途合并（否则要重扫目录、还可能改写正在读的层）。
- 写失败一律静默，与现在一致。

### 快照规则

- 只有**完整走完**（git 正常结束、`fn` 没早停）且 `enumKeyHeld`（HEAD 没在 walk 期间变）才 `storeEnum(kind, key, oids)`。
- 早停不写；半块可以留（它只证明“这些 OID 有 record”，是 gaps 能用的事实）。
- 模型冷路径维持现状：`Log()` 拿到默认序 + 写完 records 后存默认快照，因为反正要缓冲来反转。

### 参数与实测纪律

- 16384 条/块：100k 冷读约 6 块；合并线 64 层 → 约 10 次冷读触发一次合并（合并 ≈ decode 18 + encode 121 + 写 ≈ 160 ms，落在某次读命令里）。
- 待测矩阵（100k）：块 8192 / 16384 / 32768 下的冷读墙钟、层数、峰值堆、第二次读耗时；用数据定块大小，不凭推算。

### 落地顺序（V1 三步，每步可单独停）

1. **抽组件，行为不变**：`reader` 承接 `loadCached`；`actionLogCached` 改成复用它。不加任何写入。验收：全量测试 + `uncached` 字节对比。
2. **流式落盘**：`layerSink` + `action log` 流式分支种 records 与作者日序快照；模型冷路径的 `persistRecords` 也改走 sink（等价，只降峰值内存）。验收见下。
3. **接 `findIssue`**：走 `openReader`，冷回退 `Log()` + sink —— D 在这里自然完成。

V2（本轮不做）：给 `LogEach` 加 order 参数，让模型冷读也流式。收益约 57 ms 的解析重叠 + 去掉 `[]vcs.Commit` 切片；代价是改 `vcs` 接口与 jj 实现，且上面那条折叠约束仍然要求缓冲 actions，收益有限。

### 检验办法

新增测试：

1. `TestActionLogColdSeedsTheCache`：空缓存仓（2 个动作 + 1 个普通提交）跑一次 `action log --jsonl` → 必须出现层 + 作者日序快照；随后 `PATH` 指向空目录，第二次输出逐字节相同。
2. `TestActionLogEarlyStopWritesNoSnapshot`：空缓存仓跑 `action log --limit 1` → 可以有半块层，但枚举目录必须为空；再跑一次仍走流式。
3. `TestChunkedLayersReadIdentical`：测试里把块常量调小（包内变量）→ 冷跑 `list --json` 产生多块；与 `uncached` 输出逐字节相同；再跑一次热读仍相同。
4. `TestStreamedRecordsCoverTheHistory`：`action log --jsonl` 冷跑后，层里的 record 总数 == 提交数（普通提交也种了）；随后 `list --json` 不再写新层（records 被复用）。
5. 既有字节对比与 `TestDuplicateStartCheckAnswersFromTheCache` 保持绿；`gofmt` / `go vet` 干净。

### 反方

- **一周内第三次读路径重构**：`loadCached` 刚在 `cache-perf` 定型。回应：这次是把 `loadCached` 与 `actionLogCached` **合并**，净删代码（约 −100 行），且第 1 步就是纯搬运，可单独回退。
- **分块让层数涨、合并更频繁**：合并要 decode + encode 12 MB。回应：块大小与合并线都可调，先测矩阵再定。
- **流式落盘可能通过 pipe 背压拖慢 git**：我们写盘时 git 阻塞在管道上。回应：这是要测的第一件事；若冷读墙钟变差超过 50 ms，就退回窄版本——只种作者日序快照（records 由下一次模型读补），仍能让第二次 `action log` 变暖。
- **抽象过度**：三条路径的顺序与完整性不同，未必撑得起一个接口。回应：V1 只统一“缓存 walk + sink”，流式明确分叉；不把模型路径硬塞进流式。

### 不做

- 不改层格式（v2 偏移表 / 可流式格式）；
- 不并发写、不开 goroutine（先靠 git 并行）；
- 不缓存 `show` 的 body 与 `--stat` 的 diffstat；
- 不为 `action log --limit` 预生成完整快照（它读完就杀，没有完整枚举）。

## apply（2026-10-30）：读取组件 + 边读边种

### 改了什么

- **新增 `internal/cli/reader.go`**：`reader`（扫描层、建 OID 索引、缺口一次 `commits` 补读、正/反向遍历）、`layerSink`（按 8192 条分块写层，写完就丢记录）、`layerFileCount` / `settleRead` / `maybeMergeDir`（先数文件、到阈值才重新解码合并）。
- **`loadCached` 与 `actionLogCached` 合并到同一套读取**：`actionLogCached` 不再自己 scan/index/lookup/persist，两处“找这条提交的 record”只剩一份实现。
- **流式 `action log` 边读边种**：每个提交都构造成 record 落盘；完整走完且 HEAD 未变时写作者日序快照；早停只留半块、不写快照。行仍由同一段 record→row 代码渲染。
- **`findIssue` / `loadRepo` 共用 `loadUncached`**：冷回退读全史后顺手落盘 + 写默认序快照，D 在这里完成。
- **冷模型路径的 `persistRecords` 也走 sink**：一次 92 MB 的编码峰值被拆成每块约 10 MB。

### 等价性

- **新旧二进制逐字节对比**：真仓与 1k 各 21 条命令（`list` / `status` / `check` / `graph` / `path` / `show`×3 / `action log`×3 / `worktree list` / `do`×3），**0 组不同**；冷缓存下 `action log --jsonl`、`list --json`、`show` 在 1k 与 100k 也逐字节相同。
- **4 条新测试全部先对旧代码（`d319518`）验证失败**：

| 测试 | 旧代码上的失败 |
| --- | --- |
| `TestActionLogColdSeedsTheCache` | 冷跑后 0 层 |
| `TestActionLogEarlyStopWritesNoSnapshot` | 早停后 0 层（应留前缀） |
| `TestChunkedLayersReadIdentical` | 4 提交只写 1 层（应 ≥2 块） |
| `TestStreamedRecordsCoverTheHistory` | 层里 0 条记录（应 3 条，含普通提交） |

- 全量 `go test ./...` 绿；`gofmt` / `go vet` 干净。

### 性能（100k，中位数）

| 场景 | 改前 | 改后 |
| --- | --- | --- |
| `action log --jsonl` 冷 | 1026 | **1166**（写盘 +140） |
| 同上，第二次 | 174（要先有别的命令种过） | **167** |
| `list --json` 在 action log 之后首次 | 1084（action log 不种，等于冷读） | **385** |
| 同上，再读 | 158 | 149 |
| `do <issue> ask` 冷 | 864 | **983**（+119） |
| 同上，第二次 | 864 | **122** |
| 一次冷读留下的层 / 快照 | 0 / 0 | 13 / 1 |

真仓：

| 场景 | 改前 | 改后 |
| --- | --- | --- |
| `action log --jsonl` 冷 | 105 | 111 |
| 同上，第二次 | 105 | **29** |
| `list` 在 action log 之后首次 | 111 | 84 |
| `do cache-perf ask` 冷 | 106 | 113 |
| 同上，第二次 | 106 | **29** |

1k：`action log --jsonl` 冷 106 → 115，第二次 30 → 28。

**热读的 git 进程数**：`action log --jsonl` / `list --json` / `action log --limit` 在种子就位后都是 **0**（trace2 实测）。

### 内存（诚实记录：没有达到“不高于现在”）

冷流式的峰值工作集：**旧 15 MB → 新 30 MB**（8192 条/块）。这是分块编码的临时表，与历史长度无关（除快照 OID 列表：每 10 万提交约 2 MB）。分块大小实测（100k 冷 `action log --jsonl`）：

| 块大小 | 墙钟 | 峰值 WS | 层数 |
| --- | --- | --- | --- |
| 2048 | 1235 | 21 MB | 49 |
| 4096 | 1226 | 26 MB | 25 |
| **8192** | **1200** | **30 MB** | **13** |
| 16384 | 1167 | 49 MB | 7 |

取 8192：13 层/次冷读，64 层合并线约每 5 次冷读触发一次；墙钟与最小块只差 35 ms。

### 与 hypothesize 的偏差（都要记）

1. **块大小从 16384 改成 8192**：16384 的峰值 49 MB 太高（比旧流式多 34 MB），8192 用 +35 ms 换回 19 MB。
2. **冷流式 +140 ms，超过“变差 50 ms 就退窄版本”的护栏**：护栏防的是 pipe 背压拖成秒级，实际 +14% 且换来之后每次 1026 → 167——正是本方案的目标。若只跑一次 `action log` 就再也不用，这 140 ms 是纯支出；这个取舍写在这里，不再偷偷放宽口径。
3. **早停写半块**（按计划）：重复 peek 同一个前缀时内容寻址让文件名相同，不会无限涨层。
4. `--limit` 的缺口改成一次批量 `commits`（旧代码每条缺口一个 `cat-file`）。
5. **“净删约 100 行”的估计不成立**：`cache.go` 删了 230 行，`reader.go` 新增 234 行（其中约一半是注释），生产代码净增约 100 行。换来的是“找一条提交的 record”只剩一份实现——这是结构收益，不是行数收益。

### 验收

- [x] 输出逐字节不变（真仓/1k 各 21 条 + 冷路径 3 条 × 2 个仓）；
- [x] 4 条新测试先失败后通过；
- [x] 全量测试绿、`gofmt` / `go vet` 干净；
- [x] 100k 冷 `action log` 之后第二次 167；`do ask` 第二次 122；热读 0 个 git 进程；
- [x] 早停不写快照；
- [ ] 冷流式峰值不高于旧值：30 vs 15 MB，未达成（理由与矩阵见上）。

## collect（续，2026-10-30）：两种顺序各一份快照

### 现象

枚举快照按 `(kind, HEAD, 不变式)` 存：`kind=1` 是默认序（模型读要的），`kind=2` 是作者日序（`action log` 要的）。种一次只种一个 kind，于是**换一种顺序就要现走一次 git**。两个方向都测了（清空缓存 → 只让一序种下 → 测另一序第一次与第二次）：

| 规模 | 默认序 walk | 作者日序 walk | 缺默认序的代价 | 缺作者日序的代价 |
| --- | --- | --- | --- | --- |
| 真仓 450 | 52 | 54 | **54**（83 → 29） | **60**（89 → 29） |
| 1k | 50 | 54 | 55（84 → 29） | 59（88 → 29） |
| 10k | 85 | 87 | 112（149 → 37） | 96（137 → 41） |
| 100k | **164** | **477** | **228**（379 → 151） | **624**（821 → 197） |

- “代价” = 只有另一序快照时该命令的耗时 − 两序都热时的耗时；含 walk、10 万行的十六进制解析与写快照。
- 100k 的作者日序 walk 是默认序的 2.9 倍（`--author-date-order` 要带着日期队列做拓扑，10k 时两者还一样快）。
- 现实里的触发点：**每次提交后两份快照都过期**，接下来两条不同顺序的读命令各付一次 walk。

### 关键观察：线性历史里两种顺序逐字节相同

四个仓（真仓 / 1k / 10k / 100k）的 `git log --format=%H` 与 `git log --author-date-order --format=%H` 输出 **sha256 完全相同**。构造函数验证这个结论的边界：

- **线性 + 作者日期倒挂**（c5 的 author date 比 c4 早）：两序仍是 `c5 c4 c3 c2 c1`，相同；
- **带 merge + 日期倒挂**：默认序 `merge main-1 c3 c2 c1 feature-1`，作者日序 `merge feature-1 main-1 c3 c2 c1`，不同。

理由：两种顺序都是在“下一步可展示的候选集”里挑；线性历史每一步候选只有一个，比较器没有发挥空间。所以判据是**HEAD 的祖先里没有 ≥2 个父母的提交**——而且它可以从 walk 本身拿到（`%P`）。

### 方案对比

| 方案 | 做法 | 成本 | 风险 |
| --- | --- | --- | --- |
| **A 共享快照（推荐）** | `enumKind` 加 `shared=3`；walk 判定线性就写 kind=3；读者先找自己的 kind 再找 3 | walk 加 `%P`：100k 默认序 +8 ms、作者日序 +22 ms；≤10k 仓 ≤1 ms | 依赖“线性 ⇒ 同序”这一 git 行为（用测试钉住） |
| B 提交时增量前插 | 提交后把新提交前插进两份快照，不再走 walk | 需要在提交时核实父提交与日期；只覆盖线性提交 | 中等；与 A 正交，可叠加 |
| C 双写 | seed 时把两种顺序都走一遍 | 把成本前移，不省 | 无 |
| D 格式 v2 存图 | 记录里存 parents + committer date，离线推序 | 层体积 +8–28 B/条 | 要复刻 git 的队列与并列规则，字节一致风险最高 |
| E 不动 | 每次换序付上表的钱 | 0 | 无 |

### 预估收益（A）

- 100k：`action log` 种下之后 `list --json` **379 → 约 151**；`list` 种下之后 `action log --jsonl` **821 → 约 197**。
- 真仓：83 → 29、89 → 29。
- 提交循环里两种顺序都用一次的话，每次提交从两次 walk 降到一次（100k 848 → 约 500，真仓 114 → 约 55）。
- 磁盘：一份快照而不是两份（100k 约 2 MB/份）。

### 落地要点

- `vcs.Commit` 增 `Parents []string`；`Log` / `LogEach` 的 format 加 `%P`（冷路径本来就在读，只是多解析一段）。
- `CacheBackend.LogOIDs` / `EachOIDAuthorDate` 要把 parents 带出来（只有 `internal/cli/enum.go` 与 git 实现用到，改动面小）。
- `enumKind` 加 `shared`；`loadEnum(kind)` 先找 kind，再找 shared；`storeEnum` 在判定线性时写 shared。
- 兼容：旧二进制看不到 kind=3，会 miss 一次并重建自己那份；新二进制仍能读旧的 kind=1/2；`pruneEnums` 的“保留 4 份”不变（文件更少）。

### 风险与检验

- **“线性 ⇒ 同序”是 git 的实现行为，不是文档承诺**。防线是把假设钉进测试：线性 + 日期倒挂的仓断言两序相同；带 merge + 倒挂的仓断言两序不同。git 一旦改排序，测试先红，而不是快照悄悄给出错序。
- 判定只看**实际走到的集合**：浅克隆、替换 refs、graft 都由 walk 与快照不变式覆盖（边界提交的父提交不在集合里时，队列同样只有一个候选）。
- 新测试：线性仓种一次后，另一序在 `PATH` 空时也能逐字节复现；merge 仓不许假装线性（另一序 `PATH` 空时必须失败/回退，证明它仍要走 git）；`--limit` 命中 shared；非线性的 `--limit` 仍流式且不写快照。

## collect（续二，2026-10-30）：两份快照到底差在哪，怎么共享

### 字节上到底差多少

快照格式（`enum.go`）：magic(6) + version + kind + key(head 20 + 不变式 20 + count 4) + 有序 OID（每条 20 B）+ CRC32。

真仓同一个 HEAD 的两份快照（9056 字节）逐字节比对：**只有 5 个字节不同**——offset 7 的 kind，和末尾 4 字节 CRC；450×20 的 payload 完全相同。所以“其他信息”确实一样，两份的差别就是 kind 与（在非线性历史里才可能出现的）顺序。

### 把“集合”和“顺序”拆开，能省时间吗

- 拆法：一份“集合”（HEAD 可达的 OID，2 MB @100k）+ 每种顺序一份排列（uint32 下标，0.4 MB/份）。
- 磁盘：100k 从 4 MB 降到 2.8 MB。
- **时间省不了**：集合与顺序都是 git walk 的产物；拆开之后仍然要为每种顺序各走一次。读侧也省不了——多解码 1.2 MB 是几毫秒，而一次 walk 是 164–477 ms。
- 结论：这是格式工程，不是性能工程。能共享的只有“一次 walk 产出的那份列表在两种顺序下都成立”的情形。

### 为什么线性历史里就是同一份

- 四个仓（真仓/1k/10k/100k）的 merge 数都是 **0**；两种顺序的 sha256 相同。
- 构造实验：线性 + 作者日期倒挂 → 两序相同；带 merge + 倒挂 → 两序不同。
- 理由：两种顺序都在“下一步可展示的候选集”里挑；线性历史每一步候选只有一个。判据是 HEAD 祖先里没有 ≥2 个父母的提交，`%P` 就能给。

### 共享的最小实现（kind=3，payload 不变）

- `enumKind` 加 `shared=3`：文件内容与 kind=1/2 完全同构，只把 kind 字节写成 3，表示“两种顺序都成立”。
- 写：walk 的 format 加 `%P`（100k 默认序 +8 ms、作者日序 +22 ms；≤10k 仓 ≤1 ms），判定线性就写 kind=3。
- 读：`loadEnum(kind)` 先找自己的 kind，再找 3。旧二进制看不到 3，会 miss 一次并重建自己那份；新二进制仍能读旧 kind=1/2。
- 预期：100k `list` 379→约 151、`action log` 821→约 197；真仓 83→29、89→29。

### 顺带试了 git 自己的办法：commit-graph 没用

`git commit-graph write --reachable` 之后测同样的 walk（顺序不变，sha256 相同）：

| 仓 | 默认序 | 作者日序 |
| --- | --- | --- |
| 10k | 84 → 58 | 86 → 86 |
| 100k | 165 → 170 | 468 → 486 |

`--format=%H` 的 walk 不需要解析提交对象，commit-graph 的收益落不到这里；作者日序的开销在拓扑队列，不在读对象。

### 想把非线性的两序也省掉，只有 D（v2 存图）

记录里加 parents + committer date，就能在内存里从 HEAD 推出可达集合，再按 git 的两种队列规则排序；枚举 walk 与快照都可以不要。代价：层 +28 B/条（100k +2.8 MB），并且必须逐字节复刻 git 的排序（默认序是 commit-date 队列、作者日序是拓扑 + 日期队列，并列规则也要对上）。风险远高于收益，不做。

### 小结

- 两份快照的信息完全一样，差的只是 kind 和顺序；
- 拆分集合/顺序省磁盘不省时间；
- 线性判据让“一份快照服务两种顺序”成立，而四个仓的 merge 数都是 0；
- 所以选 A：共享快照（kind=3），非线性时退回两份。

## test（2026-10-30）：共享快照的原型验证

### 改了什么（原型）

- `vcs.Commit` 增 `Parents []string` + `ParentsKnown bool`；git 的 `Log` / `LogEach` 的 format 各加 `%P` 并解析（根提交的空字段也算“报告过”）。
- `CacheBackend.LogOIDs` / `EachOIDAuthorDate` 改成携带 parents（新类型 `vcs.Line`），这样两条纯枚举路径也能判定线性。
- `enumKind` 加 `shared = 3`；`loadEnum` 先找自己的 kind 再找 shared；走完一次 walk 后，四条路径（模型 `Log`、流式 `LogEach`、两条枚举）都在“线性”时写 shared。
- 判据集中在一处：`linearHistory` / 流式里的 `linear` —— **每个提交都报告了 parents，且没有一个有 ≥2 个父母**。

### 等价性

- 21 条命令 × 2 个仓（真仓、1k）与上一版二进制（`0ffcd48`）**逐字节相同**；旧二进制读不到 kind=3，会 miss 一次再重建自己那份，混合状态的输出也一致（兼容方向已验证）。
- 新增 4 条测试，其中两条**先在旧代码上失败过**：

| 测试 | 作用 | 旧代码 |
| --- | --- | --- |
| `TestLogOrdersDifferOnlyWithMerges` | 把“线性 ⇒ 同序、merge ⇒ 可能异序”钉在 git 上 | 通过（不变量） |
| `TestSharedSnapshotServesBothOrders` | 模型种一次，作者日序在 `PATH` 空时可答 | **失败** `kinds [1]` |
| `TestSharedSnapshotSeededByTheStream` | 流式种一次，模型在 `PATH` 空时可答 | **失败** `kinds [2]` |
| `TestMergeRepoDoesNotShareSnapshots` | merge 仓不许假装共享，另一序仍需 git | 通过（守卫） |

- 全量 `go test ./...` 绿；`gofmt` / `go vet` 干净。

### 效果（中位数，`nulrun` 口径）

| 场景 | 改前 | 改后 |
| --- | --- | --- |
| 真仓：`action log` 种下后 `list --json` | 83 | **30** |
| 真仓：`list` 种下后 `action log --jsonl` | 89 | **30** |
| 100k：`action log` 种下后 `list --json` | 379 | **158** |
| 100k：`list` 种下后 `action log --jsonl` | 821 | **172** |
| 100k 冷 `list --json`（种下去程） | 1011 | 1024（+13，带 parents） |
| 100k 冷 `action log --jsonl` | 1166 | 1174（+8） |

种子写入之后枚举目录里只有**一个** kind=3 文件（真仓与 100k 都一样）。

### 边界与代价

- 枚举文本变长：`git log --format=%H%x1f%P` 在 100k 从 4.0 MB 到 8.1 MB；walk 本身 +8 ms（默认序）/+22 ms（作者日序），≤10k 仓 ≤1 ms。
- 浅克隆 / 替换 refs：`%P` 报的是提交对象里的真实父母，边界上的 merge 会判成非线性——只会更保守，不会误共享。
- **非线性的两种顺序仍各走一次**，本方案不改变这一情形。
- 旧二进制看不到 kind=3：miss 一次、重建自己那份 kind=1/2，之后照旧——降级安全，只多付一次 walk。

### 判定（留给 conclude / apply）

原型证明了原理可行：一次 walk 的快照被两种顺序直接服用，输出逐字节不变，非线性仓不会误共享。接下来的 apply 只需要做“收敛”而不是再验证：决定 kind=3 的命名与注释、把 `--limit` 与合并路径过一遍、清理原型里为验证加的东西（如果有多余的）。

## apply（2026-10-30）：共享快照落地

### 改了什么

原型原样收敛，没有新增机制：

- `vcs.Commit` 增 `Parents` / `ParentsKnown`；git 的 `Log`、`LogEach` format 各加 `%P`，空字段（根提交）也算“报告过”。
- `CacheBackend.LogOIDs` / `EachOIDAuthorDate` 改带 parents（`vcs.Line` 同样有 `ParentsKnown`），两条纯枚举路径也能判定线性。
- `enumBoth = 3`：一份列表，两种顺序通用。`loadEnum` 先找自己的 kind，再找共享；四条 walk（模型 `Log`、流式 `LogEach`、两条枚举）在“线性”时写共享。
- 判据收成一个函数：`unlinear(parents, known)` —— 有 ≥2 个父母，或后端没说父母，都不许共享。只有一处定义，四处调用。
- `docs/commit-cache.md` 的“现状”与“读顺手写”两节同步了 kind 与判定；changelog 补了条目。

### 等价性

- 21 条命令 × 2 个仓（真仓、1k）与上一版二进制（`0ffcd48`）**逐字节相同**；kind 3/1/2 并存的混合状态也一致。
- 4 条测试：

| 测试 | 作用 | 旧代码 |
| --- | --- | --- |
| `TestLogOrdersDifferOnlyWithMerges` | 把“线性 ⇒ 同序、merge ⇒ 可能异序”钉在 git 上 | 通过（不变量） |
| `TestSharedSnapshotServesBothOrders` | 模型种一次，作者日序在 `PATH` 空时可答 | **失败**（kinds `[1]`） |
| `TestSharedSnapshotSeededByTheStream` | 流式种一次，模型在 `PATH` 空时可答 | **失败**（kinds `[2]`） |
| `TestMergeRepoDoesNotShareSnapshots` | merge 仓不许假装共享，另一序仍需 git | 通过（守卫） |

- 全量 `go test ./...` 绿；`gofmt` / `go vet` 干净。

### 性能（中位数，`nulrun` 口径）

| 场景 | 改前 | 改后 |
| --- | --- | --- |
| 真仓：`action log` 后 `list --json` | 83 | **30** |
| 真仓：`list` 后 `action log --jsonl` | 89 | **30** |
| 100k：`action log` 后 `list --json` | 379 | **158** |
| 100k：`list` 后 `action log --jsonl` | 821 | **172** |
| 100k 冷 `list --json`（种下去程） | 1011 | 1024（+13） |
| 100k 冷 `action log --jsonl`（种下去程） | 1166 | 1174（+8） |

### 边界与代价

- 枚举文本 100k 从 4.0 MB 到 8.1 MB；walk 本身 +8 ms（默认序）/ +22 ms（作者日序），≤10k 仓 ≤1 ms。
- **非线性的两种顺序仍各走一次**；方案不给它们新进程，也不改任何输出。
- 父母不可知（`ParentsKnown=false`）、浅克隆边界、替换 refs、octopus merge：一律判非线性——保守方向，最多多走一次 walk。
- 早停（`--limit`）不写快照、旧二进制不认 kind=3（miss 一次重建自己那份）：都与上一版一致。

### 验收

- [x] 42 组命令逐字节一致（含混合 kind 状态）；
- [x] 线性仓一次 walk 服务两种顺序（`PATH` 空可答，两条测试先对旧代码失败）；
- [x] merge 仓不共享（守卫测试）；
- [x] 全量测试绿、`gofmt` / `go vet` 干净；
- [x] 100k 换序 379→158、821→172；真仓 83/89→30。

## conclude（2026-10-30）：结论

### 一句话

缓存之后，kiln 一条读命令的时间只有三项：**进程地板**（本机 27 ms，其中镜像加载占大头）、**git 子进程**（每次约 50 ms，`show` / `worktree` / 提交都是这个价）、以及 **100k 上的解码 + 建模 + 编码**（约 18 + 65 + 25 ms）。这个 issue 最后落下的三件事，都没有碰这三项的算法，而是把“重复付”的部分拿掉：读组件让每个读命令共用同一套“找 record”的实现，边读边种让“读完历史”的路径都留下缓存，共享快照让一次 walk 同时服务两种顺序。

### 最后再测一次（同机、同状态，旧/新二进制各 3 次中位数）

旧 = `0ffcd48` 构建；新 = 本次 `apply` 后的二进制。每轮都先删 `.git/kiln`，“冷 list”与“冷 action log”各自从空缓存开始。

| 场景 | 真仓 旧 → 新 | 1k 旧 → 新 | 100k 旧 → 新 |
| --- | --- | --- | --- |
| 冷 `list --json`（种下枚举） | 118 → 115 | 116 → 119 | 996 → 1017 |
| `list` 之后 `action log --jsonl`（换序） | 89 → **29** | 88 → **30** | 769 → **174** |
| 再 `list --json` | 29 → 29 | 30 → 29 | 151 → 158 |
| 冷 `action log --jsonl`（种下作者日序） | 116 → 113 | 117 → 120 | 1169 → 1171 |
| `action log` 之后 `list --json`（换序） | 86 → **29** | 86 → **29** | 379 → **151** |
| 一次“两序都读”的会话（冷 list + 另一序 action log） | 207 → **144** | 204 → **149** | 1765 → **1191** |
| 反向会话（冷 action log + 另一序 list） | 202 → **142** | 203 → **149** | 1548 → **1322** |

- 冷读本身基本不变（100k +21 ms 是 walk 里多读 `%P`、冷路径多写共享快照的代价）。
- 换序的第二次 walk 在真仓 / 1k 从约 60 ms 降到 0（直接读共享快照），在 100k 从 228 / 624 ms 降到 0。
- 一次“两序都读一遍”的会话，真仓 −30%、1k −27%、100k −33%；反向 −30% / −27% / −15%。

### 这个 issue 的最终状态

- **读组件**（`reader` + `layerSink`）：`loadCached` 与 `actionLogCached` 共用一套“找 record”；冷路径按 8192 条分块写层，写完即丢记录。
- **边读边种**：模型读、流式 `action log`、起点检查（`findIssue`，与 `loadRepo` 共用 `loadUncached`）都会把读到的记录留下；流式早停只留半块、不写快照。
- **共享快照**：kind=3 + 线性判据（`%P` 无 merge）；非线性仓仍按顺序各存一份，重复读同样 0 进程。
- 全程没有改输出字节（真仓 / 1k × 21 条命令 + 冷路径 3 条 × 2 个仓逐字节一致）、没有新进程、没有格式版本、没有新配置。

### 仍然接受的代价（有数字的）

- **冷读的 `git log`**：100k 756 / 1079 ms，真仓约 55 / 111 ms——无索引设计的结构成本，缓存只能摊薄。
- **冷流式 +140 ms**（100k），峰值工作集 15 → 30 MB（分块编码）。
- **每次 HEAD 变化、每种顺序一次 walk**（非线性仓；真仓 52–54 ms、100k 164 / 477 ms），提交后的首读同样付一次。
- **`show` 的 body、`--stat` 的 diffstat** 仍不进缓存。
- v2“记录存图、离线推序”没做（用 28 B/条与排序复刻风险换掉最后几次 walk，判断为不划算）。

### 验收

- [x] 输出逐字节不变（多轮、含混合 kind 状态）；
- [x] 新测试全部先对旧代码验证失败（`--limit` 无 git、共享快照两条、分块、流式覆盖）；
- [x] 全量测试绿、`gofmt` / `go vet` 干净；
- [x] 热读 0 个 git 进程（trace2）；
- [x] 两序会话实测 −15% ~ −33%。

### 下一次性能复核的触发条件

- 出现“提交循环里每次读都嫌慢”的实际抱怨 → 做 B（提交时增量前插），它同时覆盖非线性仓与提交后首读；
- 100k 以上的仓成为常态、且热读仍是瓶颈 → 再评估 v2 存图；
- 有人开始把 `show` / `--stat` 当热路径用 → 单独设计体量更大的 diffstat 缓存。

