# issue: cache-perf — 缓存性能复核：warm 读的钱花在哪，优化到哪

- **domain**：tooling
- **关联**：relates-to `commit-cache`、`cache-gc`、`cache-gc-auto`

## 结论

**≤10k 提交的仓本来就在进程地板附近（20 ms 启动 + 十几毫秒读），没有可优化的空间；100k 提交的仓 warm 读从 250 ms 降到 185 ms（−26%），进程内工作从 146 ms 降到 89 ms（−39%），分配从 313 MB / 254 万次降到 114 MB / 54 万次。**

四处改动（都已落地）：

| 改动 | 位置 | 收益（100k） |
| --- | --- | --- |
| 解码器改成字节游标 + 原地填记录 | `internal/cache` | 45.5 → 15.4 ms |
| 索引存「层 + 位置」而不是记录本身 | `internal/cli/cache.go` | 省掉一次 28 MB 的记录拷贝 |
| 建模并成一趟（`engine.Builder`），去掉 `[]Action` 中转 | `internal/engine` | `assemble + Build` 73 → ~40 ms |
| `action log --limit` 有快照时走缓存 | `internal/cli` | 1k/10k 约 −50%，100k −18%（少一个 git 进程） |

`kiln cache gc` / `cache clear` / 冷路径 / 层格式都没动；30 个命令 × 3 个仓的输出与旧二进制逐字节一致。

## 端到端（中位数，ms；同一台机器交替测量旧/新二进制）

| 场景 | 旧 | 新 | 变化 |
| --- | --- | --- | --- |
| 100k `list` | 262.8 | 185.2 | −30% |
| 100k `list --json` | 251.3 | 185.1 | −26% |
| 100k `status` | 253.9 | 183.4 | −28% |
| 100k `show ISS-00001` | 264.0 | 192.4 | −27% |
| 100k `action log --jsonl` | 305.6 | 263.9 | −14% |
| 100k `action log --limit 3` | 101.5 | 83.2 | −18% |
| 10k `list --json` | 51.5 | 43.8 | −15% |
| 10k `action log --limit 3` | 131.6 | 54.1 | −59% |
| 1k `action log --limit 3` | 98.7 | 49.2 | −50% |
| 本仓（438 提交）`action log --limit 3` | 98.9 | 49.1 | −50% |

冷路径（`cache clear` 后第一次读，含全量 `git log`、解析与写层）：100k **1095 ms**、10k 221 ms、1k 124 ms——与改前持平，因为它走的是解析器不是解码器。

## 钱原来花在哪（100k warm，进程内）

改前 146 ms，改后 89 ms：

| 阶段 | 改前 | 改后 | 说明 |
| --- | --- | --- | --- |
| `cache.Scan` | 53 | ~26 | 读 12 MB（4.2）+ SHA-1 校验（5.2）+ 解码 |
| 枚举快照 | 1 | 1 | 读 2 MB 快照 + CRC |
| `gaps` | 19 | ~6 | 建索引 + 查缺口（改成位置索引后不再拷记录） |
| 取记录 + 建模 | 31 + 42 | ~55 | 合成一趟，直接喂 `engine.Builder` |
| 合计 | 146 | 89 | |

端到端 185 − 进程内 89 − 地板 20 ≈ 76 ms 仍在「全新进程的堆增长 / 内存触碰」上：`GOGC=400` 与默认无差别（252.7 vs 256.0），说明瓶颈是分配**总量**；改完 GC 次数从 10 次降到 5 次、峰值堆从 174 MB 降到 120 MB。

## 改了什么

### 1. 解码器（`internal/cache`）

旧的 `Decode` 用 `bytes.Reader` + `io.ReadFull`：每个 u16/u32 都把栈上数组逃逸成一次分配（每次读 210 万次分配里 33 万来自这两个函数），每条记录再从 `readRecord` 里逃逸一次。

新的 `reader` 是纯偏移游标：`take(n)` 越界返回 false 而不是 panic，`str()` 用 `string(body[pos:pos+n])` 一次分配，`record()` 直接把字段写进 `recs[i]`。同一份 12 MB / 10 万条的层：

| | 旧 | 新 |
| --- | --- | --- |
| 耗时 | 45.5 ms | **15.4 ms** |
| 分配 | 2.10M 次 / 76.6 MB | **0.20M 次 / 36.1 MB** |

`Scan` 原先在 `Decode` 之后又调 `Name(data)` 重算一遍 SHA-1（12 MB 再 5 ms），现在 `DecodeWithSum` 把已经算过的摘要返回，`NameOfSum` 直接用它校验文件名。

### 2. 位置索引（`internal/cli/cache.go`）

`gaps` 原来返回 `map[[20]byte]cache.Record`：每次查找拷出整个 280 B 的记录，`assemble` 再把它们拷进 `[]Record`。现在索引是 `map[[20]byte]recPos{layer, rec}`，记录只有解码层里那一份，取用时按下标读。

### 3. 建模并成一趟（`internal/engine`）

原来记录要走三遍：`assemble` 造 `[]Record`（28 MB）→ `ActionsFromRecords` 造 `[]Action`（23 MB）→ `Build` 再拷进每个 issue 的动作切片。

`engine.Builder`（`NewBuilder` / `Add` / `Repo`）把「哪个 issue、什么顺序、怎么折叠关系」收在一处；`assemble` 从枚举末尾往前走（最旧在前，正是 `Builder` 要的顺序），查一条喂一条。`Build(actions, rel)` 变成 `Builder` 的薄封装，冷路径与缓存路径仍然只有一套语义。`ActionFromRecord` 顶替了 `ActionsFromRecords`。

### 4. `action log --limit` 走缓存

旧注释的理由是「5 行首屏走 git 更便宜（46 ms）」。实测首屏的流式路径在 1k 就要 ~99 ms、10k ~132 ms，且**必须 spawn 一个 git**；作者日序枚举本身在 100k 要 630–680 ms，流式读靠 kill 提前停，kill/回收仍要 ~50 ms。

现在的规则：`--limit N` 先找**已经存在的**作者日序快照，有就解码层、按枚举顺序取前 N 条动作（不跑 git）；没有就退回流式。行先解析完再打印，这样任何中途失败都能干净地回落，不会打一半又重打一遍。这类部分枚举层**不声明覆盖**（`tips` 为空），与设计文档里「部分枚举只贡献记录」的约定一致。

验证：`TestActionLogLimitServesTheFirstScreenWithoutGit` 在 PATH 里去掉 git 后仍能输出首屏——旧代码在同一条测试里报 `exec: "git": executable file not found`。

## 验证

1. **逐字节一致**：用 HEAD 构建的旧二进制与新二进制，在 100k 合成仓、1k 合成仓、本仓三个仓库上跑 `list`、`list --json`、`status`、`check`、`graph --json`、`action log`、`action log --jsonl`、`action log --limit 5`、`path --all --json`、`show`，共 30 组，全部相同。
2. **解码器的拒绝面**：`TestDecodeRejectsEveryTruncationAndFlip` 穷举每一个截断点和每一个位翻转，全部必须被拒绝（旧实现只有两处抽查）。
3. **全测试 + 黄金网**：`go test ./...` 绿，golden 未重录。
4. **新测试先证伪**：`TestActionLogLimitServesTheFirstScreenWithoutGit` 对旧代码跑过、确认失败；解码器与索引改动由既有的「缓存/非缓存逐字节对照」测试兜底。
5. 真仓复查：`kiln list`、`kiln action log --limit 3`、`kiln cache gc` 均正常。

## 还不做（重估后仍不划算）

| 方向 | 为什么 |
| --- | --- |
| 预分配每个 issue 的动作切片 | 要多扫一遍 10 万条（~4 ms）换掉 ~15 MB 的增长拷贝；全新进程里可能值 5–10 ms，但收益不稳，留作后手 |
| 层格式 v2（OID 偏移表） | 只有「只要几条记录」的命令需要，省的是解码；换格式版本与迁移，现在解码已降到 15 ms |
| mmap 层文件 | 省 ~4 ms 读文件；Go 标准库无 mmap，要平台代码 |
| 并行解码 | 记录变长、无偏移表，等价于格式 v2 |
| `git rev-list` 代替 `git log` 做枚举 | 两轮测量互相矛盾（221/277 vs 234/310），差异在噪声内，不碰 |

## 剩余观察

- 100k 时 warm 读 185 ms 里仍有 ~76 ms 是全新进程的堆成本（分配量决定），继续要压就得动「模型本身占多大」，不是缓存格式的问题。
- 提交后第一次读仍要付一次 `git log --format=%H`（100k 约 230 ms）；快照按 HEAD 寻址，这是设计上的取舍，没动。

## 复现方法

合成仓用 `git fast-import` 生成 1k / 10k / 100k 提交（每 4 个提交 1 个无 trailer，每 4 个动作一个 issue）；端到端用 `Process.Start` 计时、每项 8–10 次取中位数；进程内用 `go test -bench` + `pprof`。
