# issue: cache-hardening — 缓存能不能信到删掉 `--no-cache`：两个会静默出错的洞

- **domain**：design
- **关联**：relates-to `commit-cache`；follows `refactor-structure`

## 结论

**不能删。** 回退设计成立（任何损坏只会变慢），但有两处会静默给错或静默变慢；`--no-cache` 同时是黄金网的唯一对照，保留。

## 背景

`refactor-structure` 收尾后，缓存是后台行为，`--no-cache` / `KILN_NO_CACHE` 是唯一例外。问题：例外还要不要留？如果缓存足够可靠，用户面可以再小一点。

先量再答：设计上"任何损坏只会变慢"（内容寻址、SHA-1 自校验、坏层忽略、写失败静默），但"只会变慢"不等于"永不算错"。

## 实测：四组注入

| 场景 | 结果 |
| --- | --- |
| 翻转 layer 一字节 / 放垃圾 layer / 截断枚举 | 输出不变，坏文件删除或重建 |
| 删除整个 `.git/kiln` | 输出不变，自动重建 |
| 60 层后 12 轮 × 4 并发读，其中两轮并发 `cache gc`（触发 64 层合并） | 48/48 逐字节一致，无 tmp 残留 |
| 空仓 / 游离 HEAD / 切分支 / 切到侧分支 | 输出一致 |

回退设计成立。但继续往下压，找到两个洞。

### 洞 1：packed replace ref 之后，枚举快照过期

```text
git replace --graft HEAD     # 历史从 3 条变 1 条
git pack-refs --all          # refs/replace 收进 packed-refs，目录空掉
kiln action log              # 3 行 —— 旧历史
kiln action log --no-cache   # 1 行 —— git 的答案
```

根因：[`enumInvariant`](../internal/cli/enum.go) 只 `ReadDir(refs/replace)`。`pack-refs` 之后目录为空，指纹与替换前完全相同，于是命中了替换前的快照。replace ref 不改变 HEAD、只改变 `git log` 的内容——正是快照失效判据必须覆盖的那类事实。

### 洞 2：坏掉的枚举文件永不自愈

`storeEnum` 对已存在的文件只比**大小**。位翻转（大小不变、CRC 不对）让 `loadEnum` 永远失败：每次读多起一个 `git log`，永久变慢。输出仍正确。

### 附带：`Sweep` 是死代码

`cache.Sweep` 会清理崩溃留下的 `layer-*.rec.tmp-<pid>`，但没有任何调用方。

## 验收条件

1. replace + pack 之后，暖读输出等于 `--no-cache`。
2. 位翻转的枚举：输出不变，且文件被重写为可解码（不是绕过）。
3. 陈旧 tmp 在 `kiln cache gc` 后消失。
4. `go test ./...` 绿；真仓九变体逐字节一致不变。

## 决策：`--no-cache` 留下

它不是"用户要理解的开关"，而是：

1. 黄金网 60 份录制的唯一对照（每个用例都要暖读与 `--no-cache` 同值）；
2. 现场唯一能在一秒内区分"缓存错"和"kiln 错"的探针；
3. [`refactor.md`](refactor.md) 的触发条件"输出与 `--no-cache` 不一致 → 删 `.git/kiln`"依赖它。

代价是一行帮助文案。删掉省不了什么，却会同时删掉验收口径与排查手段。

## apply

三处改动，都在 `internal/cli`。

### 洞 1：把 packed-refs 纳入失效判据

`replaceRefs(common)` 先看 `refs/replace/` 目录（原逻辑），再看 `packed-refs` 里有没有 `refs/replace/` 行；读文件失败（除不存在）返回 `ok=false`，同样不信任快照。`enumInvariant` 在 grafts 检查之后调用它。

为什么不是哈希 `packed-refs` 的 mtime：同一秒内的两次改写会让 mtime 相同，而精确性正是这个判据存在的理由；读一个 `packed-refs` 在常见仓里是几十 KB，代价可忽略。大仓里它是 MB 级，但那类仓每次读本来就在几百 ms 量级。

### 洞 2：坏快照当场丢弃、按内容比较重写

- `loadEnum`：解码失败就删文件，这次 miss 会重建它。
- `storeEnum`：不再只比大小，改成 `bytes.Equal(data, enumEncode(...))`——大小相同但校验错的文件会被覆盖。

### 附带：`Sweep` 接进 `cache gc`

`cacheGC` 收尾调一次 `cache.Sweep(dir, time.Now())`，清掉崩溃留下的 `layer-*.rec.tmp-<pid>`（超过 1 小时才算陈旧，正在写的进程不受影响）。

### 测试

新增三条：

| 测试 | 修的洞 |
| --- | --- |
| `TestSnapshotFallsBackOnPackedReplaceRefs` | 洞 1：replace + pack 后暖读必须等于 `--no-cache` |
| `TestCorruptSnapshotIsRebuilt` | 洞 2：位翻转后输出不变，且文件必须可解码 |
| `TestCacheGCSweepsTornTempFiles` | Sweep：陈旧 tmp 清掉、新鲜 tmp 保留 |

前两条先对着旧代码跑过：`PackedReplaceRefs` 复现 3 行 vs 1 行，`CorruptSnapshotIsRebuilt` 报"没被重写"。改完转绿。

### 验证

- `go test ./...` 绿（cli 21.6 s，含黄金网 60 份录制）。
- 真仓上重跑 replace 探针：packed 之后四命令 `cache==nocache` 全 True；`action log` 的 `cache==staleWarm` 从 True 变 False——它重新问了 git，而不是命中旧快照。

## 结论

1. 洞 1 修好：replace ref 无论 loose 还是 packed，快照都不再被信任。
2. 洞 2 修好：坏快照在一次读里自愈。
3. `Sweep` 不再是死代码。
4. `--no-cache` 保留（理由见 ask 末节）。

没做的：

- **git 版本没纳入快照键**：换 git 后 tie-break 变化仍可能让旧快照过期；触发条件见 [`refactor.md`](refactor.md)。
- **jj 后端仍无缓存**：`jj.ExecRunner` 不实现 `CacheBackend`，缓存对它整体不生效。
- **没有"重建缓存"命令**：`cache gc` 只合并已有记录、不重新解析；解析器修 bug 后要么删 `.git/kiln`，要么换 magic。这是下一次的触发条件。
- **仍是单机结论**：Windows + git 2.55，没有 CI；Linux/macOS、只读 FS、reftable、git <2.31 未验。


## 追记（2026-10-29，用户重新考虑）

用户指出"保留 `--no-cache`"是把实现需要当成了用户需要：用户要的是恢复命令，不是关缓存开关。结论撤回——`--no-cache` / `KILN_NO_CACHE` 已删除，缓存关断降为包内开关（只给测试），用户面改为 `kiln cache clear`。推演、设计与实现见 [`cache-recovery.md`](cache-recovery.md)。本文件其余内容（两个洞的复现与修法）仍然有效。
