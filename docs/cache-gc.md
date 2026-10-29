# issue: cache-gc — 自动 GC 的五档与两处口径：`visited` 该用枚举数，`pruneEnums` 该说真话

- **domain**：tooling
- **关联**：follows `cache-recovery`、`cache-hardening`；relates-to `commit-cache`

## 结论

自动 GC 的档位本身是对的——只删可再生成的东西、失败方向是"少删"、量有界；但两处口径与实现不一致：`action log` 触发的合并把**渲染出的动作行数**当成 `visited`，而设计说的是"本次枚举的提交数"；`pruneEnums` 声明"按 kind 保留 4 份"，实际是**总数 4 份**。前者改代码，后者改接口（去掉没用到的 `kind`）并如实写注释。

## 自动 GC 的五档

| 时机 | 触发 | 删除什么 | 成本 |
| --- | --- | --- | --- |
| 每次读扫描 | 层或快照校验失败 | 坏文件当场删 | 0 |
| 读命中后层数 ≥64 | `maybeMerge` | 合并成 1 层（纯本地） | 100k ≈ 80 ms，摊 64 次读 |
| 合并时记录数 > 本次枚举数 | `mergeLayers` | `rev-list --all` 过滤不可达记录 | 只在有候选时跑，100k ≈ 200 ms |
| 全史重写（枚举 100% 未命中且已有层） | `settle` | 立刻合并 + 过滤，不等 64 层 | 同上，重写后一次 |
| 每次写快照 | `pruneEnums` | 只留最近 4 份 | 0 |

不自动回收的三类（必须 `kiln cache gc` / `cache clear`）：旧词表指纹的层、不可达 HEAD 的枚举快照、崩溃留下的 tmp 文件。

## 两处口径

### 1. `actionLogCached` 把 `emitted` 当 `visited`

[`cache.go`](../internal/cli/cache.go) 里 `maybeMerge(…, emitted)`：`emitted` 是渲染出的**动作**行数（无 trailer 的提交不计）。后果：

- 有动作时 `records > emitted` 几乎总成立，于是每次合并都多跑一次 `rev-list --all`（每 64 次读一次，摊薄后约 3 ms/读）；
- **零动作仓**（只有普通提交）`emitted == 0`，`visited > 0` 不成立，这条路径**永不剪枝**——与设计相反。

正确口径是 `len(oids)`：本次枚举的提交数，与 `settle` / `persistRecords` 两处一致。

### 2. `pruneEnums` 声明 per-kind，实现 per-dir

[`enum.go`](../internal/cli/enum.go) 的注释说"每种 kind 保留 4 份"，`enumKeep` 的注释写"of one kind"，但函数体只按文件名过滤，`kind` 参数**没被使用**。实际是总数 4 份，两种顺序共享窗口。

保持"总数 4 份"是对的（快照 20 B/提交，100k 下 4 份约 8 MB；被挤掉的快照一次 `git log` 就能重建），所以改接口而不是改行为：

- `pruneEnums(dir, keep)` 去掉 `kind`；
- `enumKeep` 与 `pruneEnums` 的注释改成"总数、两种顺序共享窗口、被挤出只是多一次 git log"。

## 验收条件

1. 零动作仓：两次普通提交 warm 后 amend 一次 → `action log` 触发合并 → 记录 3 → 2（旧代码不剪），层数 1。
2. `pruneEnums` 不再有 `kind` 参数；`TestEnumStoreLoadAndPrune` 仍绿。
3. `go test ./...` 绿。
4. 文档：`commit-cache.md` 的"保留最近 4 份"与实现一致（总数）。

## apply

两处都改。

### `visited` = 本次枚举的提交数

`actionLogCached` 的 `maybeMerge(…, len(oids))`。注释写清：比较的是枚举，不是渲染行数——普通提交也算访问过的历史，而且仓库可能一个动作都没有。

回归测试 `TestActionLogMergePrunesWithoutRenderingRows`：两条普通提交 warm 后 amend 一次，`action log` 渲染 0 行、但仍写一个层并触发合并；旧代码留下 3 条记录，新代码剪到 2 条。先对旧代码跑过（FAIL: got 3 records），再转绿。

### `pruneEnums` 说真话

- 去掉没用到的 `kind` 参数：`pruneEnums(dir, keep)`；
- `enumKeep` 与 `pruneEnums` 的注释改成"总数 4 份、两种顺序共享窗口、被挤出只是多一次 git log"；
- 行为不变（本来就按 mtime 取最新 4 个文件），`TestEnumStoreLoadAndPrune` 照旧。

### 验证

- `go test ./...` 绿（cli 21.7 s）。
- 回归测试先对旧代码验证失败。

## 结论

- 合并门槛的口径统一：`settle` / `persistRecords` / `actionLogCached` 三处都用"本次枚举的提交数"。
- 枚举保留策略的注释与实现一致：总数 4 份。
- 自动 GC 仍不覆盖旧指纹层、不可达快照、tmp 文件——归 `kiln cache gc`，与本文的"五档"一致。

没做的：没有为多 worktree 提高保留份数（10 万提交下每份约 2 MB，多留几份换命中率不划算）；也没有把 `cache gc` 挂到 `commit` 之后（提交路径不该承担读路径的回收）。
