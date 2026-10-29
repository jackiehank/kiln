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
