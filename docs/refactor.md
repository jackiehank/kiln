# issue: refactor — 全面重构与性能优化

- **domain**：design
- **关联**：relates-to `perf-audit`（性能边界与三次低风险优化）；relates-to `commit-cache`（缓存落地后，"git 还是不是大头"这个问题换了答案）

## 背景

用户提出：kiln 需要一次全面的重构，同时把性能再推一档；性能侧先回答一个具体问题——**现在主要的耗时是不是在调用 git 上**。

`perf-audit` 的结论是"无索引 + 压掉进程"：真仓读命令 0.10–0.17 s，`--stat` 从 14.5 s 降到 0.31 s。`commit-cache` 之后，热读路径变成"一次 `git log --format=%H` + 一次本地层解码"，于是原来那句"一次 `git log` 是地板"不再自动成立：剩下的一次 git 进程仍按进程计价，而 kiln 自己开始解码 7 MB 级的层、建 100k 条记录。**谁是大头，取决于规模，得测。**

代码侧的现状（本仓 `c325656`）：

| 维度 | 数字 |
| --- | --- |
| 非测试代码 | 18 个文件 / 8,638 行 |
| 测试代码 | 11 个文件 / 7,311 行 |
| 最大单文件 | `internal/cli/cli.go` 3,471 行 / 117 个顶层函数 / 52 个 `(*env)` 方法 |
| 后端 | `vcs/git` 852 行 / 38 个函数；`vcs/jj` 496 行 / 24 个函数 |
| 覆盖率 | cli 86.3%、cache 82.7%、engine 80.9%、config 91.7%、trailer 94.9%、git 45.0%、jj 28.2%、model 33.3% |

`cli.go` 同时承担参数解析、命令编排、渲染（文本 / JSON / ascii / mermaid）与缓存协调；两个后端在 `Log` / `Show` / `Action` / `Changes` 等处各写一遍语义。

## 要回答的问题

1. **耗时归属**：热读 / 冷读的墙上时间里，进程创建、kiln 自身 CPU、一次 `git` 进程各占多少？随提交数怎么变？"git 是不是大头"在哪个规模上反转？
2. **自身 CPU 的热点**：解码层、建 map、建模型、折边、排序、渲染各多少？有没有同一个文件扫两遍这类重复劳动？
3. **不变成本**：一次进程创建（空 Go 程序）在本机多少毫秒？kiln 的二进制（5.5 MB）比它多付多少？二进制大小与初始化能不能压？
4. **结构性代价**：`cli.go` 3,471 行是不是真障碍——改一个命令要碰几处？渲染与编排能不能分开？两个后端重复的语义值不值得抽公共层？覆盖率最低的 `model` / `jj` 是"没测"还是"难测"？
5. **重构边界**：哪些是移动代码、契约为零风险的安全区；哪些必须和性能一起改；哪些（索引 / daemon / 并发 / 新依赖）明确不做？

## 非目标

- 不改 CLI 契约：命令、flag、文本 / JSON 输出逐字节不变；
- 不为性能引入常驻进程、索引或第三方依赖；
- 不重写已被验证的解析器与缓存格式（`trailer` / `cache` 的编码）；
- 不在测量前排并发，不为"整齐"做无收益的搬迁；
- 不把"重构"当目的——每条改动要说清它买到了什么。

## 验收（collect 完成时）

- 热读 × 真仓 + 合成规模的耗时分解表（进程创建 / `git` 子进程 / 自身 CPU）；
- 自身 CPU 的分阶段明细，含 100k 上的行级证据（谁在扫两遍、谁在排序）；
- 结构性盘点：文件与函数规模、后端重复面、覆盖率缺口；
- 明确回答"git 是不是主要耗时"，并给出反转点；
- 候选方向清单（必做 / 可选 / 不做），留给 hypothesize 定性。

## 采集（2026-10-29）：耗时到底在哪

### 口径：三层分开量

绝对耗时一直在被"测量工具本身"污染，所以这次分三层，各量各的：

1. **进程内**：临时探针（独立 module，`replace` 指回本仓）直接调 `cli.Run(argv, io.Discard, os.Stderr)`，只计 `Run` 内部的墙钟；输出丢给 `io.Discard`，不掺终端 / 磁盘。这是"kiln 自己花了多少"的**唯一可信口径**。
2. **git 子进程**：PATH 前置一个 `git.exe` 垫片，转发给真 git 并记录每次调用的存活时间与参数。它回答"调了几次 git、每次多久"，也证明"调 git"是不是瓶颈。
3. **端到端**：.NET `ProcessStartInfo` 起进程、读管道、`WaitForExit`；同一个 harness 里跑一个空 Go 程序做地板。它回答"用户等多久"，其中差额就是"Run 之外的成本"（进程创建、二进制加载）。

另有一个**阶段探针**：用真实包（`cache.Scan`、`engine.ActionsFromRecords`、`engine.Build`、`engine.FoldEdges`）逐段复刻热读路径，并与进程内总时长对账（账对得上，见 §3）。

- **机器**：Windows，Go 1.27.0，git 2.55.0.windows.3；kiln 由 `c325656` 的源码构建。绝对数字只属于这台机器（进程贵是 Windows + 本机 Defender 的特征），结构性结论不依赖它。
- **合成仓**：`git fast-import`，消息语料逐字取自本仓（关系、标题、body 的真实分布），N = 3k / 10k / 100k 提交。**注意：语料是循环的，100k 提交只有 72 个 issue**——所以折边和渲染被低估；读取与解码不被低估。
- **取数**：每个数字 3–7 次取 min；"端到端"表 7 次。

### 0. 一句话答案

- **本仓规模（406 提交 / 72 issue / 层 94 KB）：是。** `status` 端到端 96.3 ms，其中一次 git 进程 52.5 ms、进程地板 39.3 ms、kiln 自己的逻辑 **1–3 ms**。
- **10k 提交：还是 git，但只剩 69%。** `graph --json` 进程内 92.4 ms，git 63.8 ms、自身 28.6 ms。
- **100k 提交：反转。** `graph` 的 git 占 42%、`list` 52%、`path` 35%，**kiln 自身 CPU 成了大头**（195–392 ms），其中约 138 ms 是测量证明的浪费（见 §3）。
- **例外是 `action log`**：任何规模上 git 都占 84–96%，因为作者日期枚举拿不到缓存（author date 不在 commit-graph 里，git 必须逐个解析提交对象）。

### 1. 地板：一次调用里，进程占多大

用 `-overlay` 把 `cmd/kiln/main.go` 换成一个"判断环境变量就打印 `hi` 并返回"的版本，得到**同尺寸、同链接、什么都不做**的二进制——这样二进制大小与运行时的成本就和命令逻辑分开了。

| 项 | 端到端 min (ms) | 说明 |
| --- | --- | --- |
| 空 Go 程序 `hello.exe`（2.5 MB） | 32.3 | 进程创建 + 测量侧管道 + Go 运行时 |
| 同二进制改走"空分支"（5.5 MB） | 39.3 | 多 7 ms = 二进制 2.5 → 5.5 MB |
| `kiln --version` | 59.0 | 39.3 + **19.7**（见下一行） |
| `--version` 内部那 19.7 ms | — | Go 的 `time.Local` 惰性初始化：`time.Now()` 首次调用本机实测 20.5 ms |
| 一次 `git --version`（PATH 上的 `cmd\git.exe`） | 43.7 | Git for Windows 的 launcher |
| 直呼真身 `mingw64\bin\git.exe --version` | 30.8 | **launcher 链本身 ≈13 ms/次** |

两件事值得记住：

- **`--version` 的 20 ms 花在打印本地时间上**，不在命令逻辑里；`cli.Run` 内部实测把 `--version` 的第二、三次调用压到 0 µs，说明是一次性初始化。读命令不付这笔钱（提交日期带显式时区偏移，不碰 `time.Local`）。
- **kiln 的每次 git 调用是 2 个进程**：`exec.Command("git")` → PATH 上的 `cmd\git.exe` → 再 exec `mingw64\bin\git.exe`。`git.go` 里 `WaitDelay` 的注释已经记了这件事（杀 launcher 不等于杀真身），但没人量过它 ≈13 ms。

### 2. 热读分解：git 子进程 vs kiln 自身

进程内口径；"git 子进程"取自阶段探针的 `LogOIDs` 实际耗时，"自身"= 进程内总时长 − git。

| 仓 / 规模 | 命令 | `cli.Run` 内 (ms) | git 子进程 (ms) | kiln 自身 (ms) | git 占比 |
| --- | --- | --- | --- | --- | --- |
| 本仓 406 提交 / 层 94 KB | `status` | 53.6 | 52.5 | **1.1** | 98% |
| 同上 | `graph --json` | 55.1 | 52.5 | 2.6 | 95% |
| 同上 | `action log --jsonl` | 68.9 | 66 † | 2.9 | 96% |
| 3k 提交 | `graph --json` | 62.0 | 52.2 | 9.8 | 84% |
| 10k 提交 | `graph --json` | 92.4 | 63.8 | 28.6 | 69% |
| 100k 提交 | `graph --json` | 507.0 | 212.5 | **294.5** | 42% |
| 100k 提交 | `list --json` | 406.7 | 212.5 | 194.2 | 52% |
| 100k 提交 | `path --all --json` | 604.6 | 212.5 | 392.1 | 35% |
| 100k 提交 | `action log --jsonl` | 844.7 | 707 † | 137.7 | 84% |

† `action log` 走 `--author-date-order`，阶段探针没有对应实现，用垫片量（含垫片自身 ≈5 ms）。

**真仓的账能对上**：端到端 96.3 ≈ 39.3（地板）+ 52.5（git）+ 4.5（自身 + 输出）。

### 3. 自身 CPU 的分段（100k，`graph --json`）

阶段探针 4 次取 min，并用进程内 507.0 ms 对账：

| 阶段 | ms | 占比 | 备注 |
| --- | --- | --- | --- |
| `git log --format=%H` | 212.5 | 44.6% | 4.1 MB OID + 一次进程 |
| `cache.Scan` | 31.4 | 6.6% | 读 7.0 MB + SHA-1 + 解码 100k 条 |
| 建 index：`make(map, 0)` | 51.8 | 10.9% | 没有容量提示，反复扩容 |
| —— 同一件事、预分配 `len(oids)` | **9.2** | 1.9% | **差 42.6 ms** |
| `engine.Build` | 47.6 | 10.0% | |
| 折边 + 排序 | 97.7 | 20.5% | `sort.SliceStable` 占其中 ~65%（pprof） |
| 再扫一遍层（`maybeMerge`） | 35.4 | 7.4% | 同一次读里第二次 `cache.Scan` |
| 渲染（只有 72 个 issue） | ≈0 | | 语料低估了这项 |
| 合计 | 476.4 | | 进程内 507.0；差 ≈30 = 记录装配 + i18n |

三笔**测量证明**的浪费（合计 ≈138 ms，占 100k 热读的 27%）：

1. **每次热读把层读两遍。** pprof 的 `cache.Scan` 调用方：54% 来自 `maybeMerge`、46% 来自 `loadCached`。`loadCached` 已经解出了 `layers`，但它把 `records` 交给下游后，`maybeMerge` 又 `cache.Scan(dir)` 一遍来数层数——7 MB 读两遍、SHA-1 两遍、解码两遍（31.4 + 35.4 ms）。
2. **index map 没有容量提示。** `make(map[[20]byte]cache.Record, 0)` 插入 10 万条：[20]byte 键 + `Record` 值（含 4 个 slice 头）体量大，扩容与搬迁把 9.2 ms 的活干成 51.8 ms。
3. **折边把所有动作全量排序。** `FoldEdges` 为了"按时间顺序折叠声明"，把 100k 个 `model.Action` 装进 `[]item` 再用 `sort.SliceStable` 按 `time.Time` 比较——100k 条、每比较一次 `Equal`+`Before`；pprof 行级显示排序 400/620 ms（5 次运行），即 ≈65%。

### 4. 冷读：建层要付多少

| 仓 | 冷 (ms) | 热 (ms) | 多付 (ms) | 层大小 |
| --- | --- | --- | --- | --- |
| 本仓（`status`） | 95.3 | 56.6 | +38.7 | 94 KB |
| 10k（`graph --json`） | 201.3 | 89.5 | +111.8 | 766 KB |
| 100k（`graph --json`） | 1358.2 | 525.9 | +832.3 | 7.0 MB |

100k 上多付的 832 ms 里，约 700 ms 是"带 body 的全史 `git log`"（912 − 212），其余是解析 100k 条消息 + 编码 + 写 7 MB。

### 5. 命令形态：git 调了几次

垫片实测（真仓）：

| 命令 | git 调用 |
| --- | --- |
| `status` / `list` / `check` / `graph` / `path` | 1 × `log --format=%H` |
| `action log` | 1 × `log --author-date-order --format=%H`（流式，可提前停） |
| `show` | 2 ×（`log --format=%H` + `show <hash>`） |
| `worktree list` | 1 × `worktree list --porcelain`（不读历史） |
| `--no-cache` 变体 | 同样 1 次，但换成带 body 的 `log --format=%H%x1f%aI%x1f%B%x1e` |

体量：100k 上 `path --all --json` 输出 31.5 MB、`action log --jsonl` 31.8 MB——渲染与写盘本身也是一笔账（`path --all --json` 写文件 903 ms vs 写 `Discard` 604 ms）。

### 6. 结论

1. **"git 是不是主要耗时"取决于规模，且在 10k–100k 之间反转。** 真仓 98% 是那一次 git 进程；10k 时降到 69%；100k 时 `graph` / `list` / `path` 都跌破 52%，kiln 自身成为大头。
2. **小规模上最大的单项不是代码，是进程。** 每次调用 ≈39 ms 地板（含 7 ms 二进制尺寸），一次 git ≈50 ms（其中 ≈13 ms 是 launcher 链），kiln 自己的逻辑 1–3 ms。**在真仓规模上继续"优化代码"没有意义**——除非动进程形态（不现实）或干脆不调 git。
3. **大规模上最大的单项在 kiln 自己这边**，而且不是算法复杂度问题，是三处"顺手写"的浪费：扫两遍层、map 不预分配、全量稳定排序。
4. **`action log` 是唯一没被缓存救到的命令**（作者日期不在 commit-graph 里），它的 84–96% 永远是 git；要动它只能动枚举方式，不是动 kiln 的 CPU。
5. **冷读的代价与层大小成正比**，100k 一次性 +832 ms；只要层还在读路径上，这就是"第一次用"的地板。

### 7. 结构性盘点（重构侧）

| 项 | 现状 | 观察 |
| --- | --- | --- |
| `internal/cli/cli.go` | 3,471 行 / 117 函数 / 52 个 `(*env)` 方法 | 占非测试代码 40%；解析、编排、渲染、缓存协调全在一处 |
| `loadCached` | 1 个函数里做完：扫层、算缺失、补拉、判重写、判合并、建模 | 70 行、6 个职责；"扫两遍层"就长在这里 |
| 两个后端 | `git` 38 函数 / `jj` 24 函数，**17 个同名方法**（`Log`/`LogEach`/`Action`/`ActionChecked`/`action`/`Changes`/`Unchanged`/`Show`/`ShowStat`/`IsDedicated`/`NewWorkspace`/`ListWorkspaces`/`Dirty`/`AreaState`/`dirty`/`RemoveWorkspace`/`run`） | 语义各写一遍；jj 不实现 `CacheBackend`，分叉是设计使然，但重复的模板可以抽 |
| 渲染 | 文本在 `cli`、ascii/mermaid 在 `engine`、JSON 在 `cli` | 同一条数据三种出口，没有共同的渲染接口，测试只能走 `cli.Run` |
| 覆盖率 | `model` 33.3%、`jj` 28.2%、`git` 45.0% | 重构最想动的地方恰好是测得最薄的地方 |
| 测试规模 | 11 文件 / 7,311 行，`internal/cli` 单包 13.1 s | 反馈不慢，但端到端测试占主导 |

### 8. 候选方向（留给 hypothesize 定性，本 collect 不决定）

**有测量证据、可预期收益**：

1. 热读**只扫一遍层**：`loadCached` 把已解出的 `layers` 传给 `maybeMerge`，别让它再 `Scan`。（100k：−35 ms）
2. **预分配** index map 与 `missing`：`make(map[[20]byte]cache.Record, len(oids))`。（100k：−43 ms）
3. **折边去掉全量稳定排序**：先按 `(Date, issue)` 生成 `int64` 键 + `sort.Slice`；或者干脆不排——每个 issue 的 actions 本身已是时间序，按 issue 归并即可。（100k：−60 ms 量级）
4. （可选）**缓存命中时省掉一次哈希往返**：层内记录本来就按 OID 有序，把 `oids` 排序后与记录做归并，也许比建 map 更快——未测，需 hypothesize 里定性。

**结构性（重构）候选**：

5. 把 `cli.go` 按"解析 / 编排 / 渲染"拆包，先拆渲染（可脱离 `cli.Run` 测试）；
6. 给两个后端抽一层公共语义（模板方法或 helper），不动 `vcs.Repo` 契约；
7. 补 `model` / `jj` 的测试到与 `cli` 同档（50%–80%），再谈搬迁；
8. `--version` 的 20 ms：改印 UTC，或把 `构建时间` 挪出 `time.Local`——只影响诊断命令。

**明确不做**：

- daemon / 常驻进程 / 索引 / 并发读——与"无状态、永远和 git 一致"的设计冲突，且本 collect 没测出它们能赢过"一次 git 进程 + 本地层"；
- 二进制瘦身（`-s -w` 实测只省 ≈2 ms，不值）；
- `show` / `--stat` 路径的进一步批量——`perf-audit` 已经做过，且不在这个问题的热点里。

### 9. 没测到的 / 口径警告

- **只有 Windows**：Linux/macOS 上进程创建便宜一个量级，"地板"结构不变但占比会小很多；本 collect 的结论里"进程是大头"只对这台机器成立。
- **合成语料只有 72 个 issue**：折边与渲染被低估；真实"多 issue"仓的 `graph` / `path` 会更贵，`FoldEdges` 那笔可能更显著。
- `show` / `--stat` / `commit` 只量了调用数，没做分解。
- 并发读（两个进程同时写层）没测；`go test -race` 在本机不可用（CGO 关闭）。
- 5.5 MB 二进制的 +7 ms 无法进一步拆分（链接布局 vs Defender 扫描）。
