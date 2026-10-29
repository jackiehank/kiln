# issue: cache-gc-auto — 三类"只能手动清"的残留：按年龄自动收，快照搭合并顺带清

- **domain**：tooling
- **关联**：follows `cache-gc`、`cache-recovery`；relates-to `commit-cache`

## 结论

都用年龄阈值 + 下次读顺手扫：tmp 文件超 **1 小时**、旧指纹层超 **7 天** 自动删；不可达快照不新增扫描，搭合并本来就要算的 `rev-list --all` 顺带清。删除永远只是让某一方多一次全扫，不会算错。

## 先看别人的宽限期

| 工具 | 自动清理的阈值 | 依据 |
| --- | --- | --- |
| Go build cache | 超过 5 天未用就 trim | Go 的 trim "drops only those unused for five days" |
| git gc | 不可达对象默认 2 周，`gc.pruneExpire` 可调，`--prune=now` 立即 | `git-gc(1)` |
| npm/cacache | 维护脚本示例按 30 天 | cacache Cache Maintenance |

共同模式：**不因"版本不同"立刻删，按年龄删；同时保留"立刻清"的显式入口**。kiln 的立即入口已经有两个：`cache gc`（立刻、无年龄限制）、`cache clear`（全清），所以自动路径只需要年龄阈值。

## 三类方案

### 1. tmp 文件（`layer-*.rec.tmp-<pid>`）

- 每次读扫描目录时，超龄（1h，复用 `cache.StaleTemp`）就删。
- 活着的写者只持有 tmp 毫秒级（写完即 rename），只有崩溃会留下。
- 成本：一次 `ReadDir` + 命中时 `Remove`，通常 0 个文件。

### 2. 旧指纹层

- 读扫描时 `fp != 当前` 且 mtime 超过 **7 天** → 删。
- 为什么不是立刻：两个版本混用（PATH 与仓库各一个、CI 固定旧版）会互删，每次读退化全扫；宽限期让两版共存时各自命中。
- 为什么不是永远：旧 fp 对这个二进制永远不可读；100k 一代约 13 MB。
- 兜底：`cache gc` 立刻删（不受年龄限制）。
- 成本：`cache.Layer` 加 `ModTime`（`Scan` 的 `DirEntry` 本来就有），零额外 I/O。

### 3. 不可达快照

- 现状已自动有界：写时只留总数 4 份，最多 4 个文件（100k 每个约 2 MB）。
- 单独按可达集扫要一次 `rev-list --all`（100k ≈ 200 ms），为几 MB 不值。
- 方案：`mergeLayers` 在"记录数 > 枚举数"时本来就跑 `rev-list --all`，顺手调 `gcEnums`——重写信号与 64 层合并都会走到，成本 +0。
- 其余等 4 份窗口挤出。
- 代价：linked worktree 的 detached HEAD 不在 `rev-list --all` 里，其快照会被删——下一次读多一次 `git log`，不会错。

## 落地形状

```text
internal/cache/store.go   Layer 加 ModTime；新增 RemoveStaleTemps(dir, now)（只 ReadDir）
internal/cli/cache.go     pruneAged(dir, fp, layers)：超龄 tmp + 超龄异指纹层
                          openLayers / actionLogCached / persistRecords 在 Scan 后各调一次
                          mergeLayers 算完 reachable 后调 gcEnums
常量                       StaleTemp = 1h（已有）；foreignGrace = 7*24h（新）
```

一次读的额外开销：若干 `stat`（DirEntry.Info 已在字段里）+ 一次 `ReadDir`，无 git 进程。

## 验收

1. 2 小时前的 tmp：一次普通读后消失；刚写的保留。
2. 异指纹层：8 天前 → 读后删；1 天前 → 保留；当前指纹的层即使 8 天前也保留。
3. `maxLayers=1` + `reset --hard`：合并后被改写掉的 HEAD 快照消失，当前 HEAD 的快照保留。
4. 黄金网 60 份录制不变；`go test ./...` 绿。

## 风险与不做

- **时钟回拨**：mtime 在未来 → 不删（保守方向）。
- **并发**：删除一律尽力而为，失败忽略；读方已把层读进内存。
- **不做**按大小配额（100k 才 13 MB），不做定时器/后台进程。
- **混用两版**：宽限期内互不干扰；超过之后旧版一次全扫重建，可接受。

## apply

三处改动。

### 1. 层带上 mtime，tmp 清理变便宜

`cache.Layer` 加 `ModTime`（`Scan` 从 `DirEntry.Info` 填，零额外 I/O）；新增 `cache.RemoveStaleTemps(dir, now)`：只 `ReadDir` + `stat`，不解码，读路径可以每次调。

### 2. 读时顺手清：tmp 1 小时、旧指纹层 7 天

`cli.pruneAged(dir, fp, layers)`：删超龄 tmp（`StaleTemp = 1h`）+ 超龄异指纹层（`foreignGrace = 7*24h`）；在 `openLayers`、`persistRecords`、`actionLogCached` 三处 Scan 之后各调一次。**当前指纹的层无论多老都不删**——年龄只决定这个二进制读不了的层。

### 3. 不可达快照搭合并

`mergeLayers` 在 `rev-list --all` 成功之后调 `gcEnums(dir, reachable)`：重写信号与 64 层合并都会走到这里，成本 +0。其余情况仍由"写时保留 4 份"封顶。

## 测试

三条新测试，先对旧行为跑过、确认失败：

| 测试 | 旧行为下的失败 |
| --- | --- |
| `TestReadSweepsStaleTempFiles` | 2 小时前的 tmp 还在 |
| `TestReadDropsForeignLayersOnlyAfterTheGracePeriod` | 8 天前的异指纹层还在 |
| `TestMergeDropsUnreachableSnapshots` | 合并后剩 2 个快照（被 `reset` 掉的 HEAD 还在） |

`TestReadDropsForeign…` 同时钉住两条边界：当前指纹的层即使 8 天前也保留；1 天前的异指纹层保留（宽限期内）。

## 验证

- `go test ./...` 绿（cli 24.9 s），黄金网 60 份录制未动。
- `gofmt -l`、`go vet` 干净。

## 结论

- 三类残留现在都能自动收：tmp 1 小时、旧指纹层 7 天、不可达快照搭合并。
- 手动入口仍在：`kiln cache gc` 立即清（不受年龄限制）、`kiln cache clear` 全清。
- 没做配额、定时器、后台进程；时钟回拨（mtime 在未来）不删——保守方向。
