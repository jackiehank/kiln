# issue: cache-recovery — 用户要的不是"关缓存"，是"清缓存"：`kiln cache clear` + 内部开关

- **domain**：cli
- **关联**：supersedes `cache-hardening` 的"保留 `--no-cache`"；relates-to `commit-cache`

## 结论

`--no-cache` 与 `KILN_NO_CACHE` 从命令行、帮助、配置注释里消失，缓存关断降级为包内开关（只给测试用）；用户面新增 `kiln cache clear`——出错或升级后一条命令回到干净状态，下一次读用当前代码重建。

## 重新推演

上一轮的结论是"保留 `--no-cache`，因为它是黄金网的对照，也是现场唯一的探针"。用户指出：这是把**实现需要**当成了**用户需要**。

1. 用户不需要"不读缓存地跑一次"。他要的是"结果不对时，回到可信状态"——前者是诊断手段，后者是恢复手段。
2. 诊断探针可以内部化。60 份黄金录制与缓存测试对照的是**冷路径**，那是测试自己的需要；包里一个开关就够，不必暴露成 flag。
3. `--no-cache` 是负向开关：它让工具"少做一件事"，用户得先理解"缓存是什么"才能用它。`clear` 是正向动作：清掉、重建，不需要理解内部。
4. 恢复命令必须覆盖"解析器变了"这一类：`cache gc` 只合并已有记录、不重新解析，旧层会一直遮住升级后的修复；只有让下一次读走冷路径（当前代码全扫）才修得掉。

## 设计

`kiln cache clear`：

- 删除 `<git-common-dir>/kiln/records/` 与 `kiln/enums/`（层 + 枚举快照）。缓存本来不存在也算成功，输出"缓存是空的"。
- **不立即重建**：下一次读就是重建——冷路径与缓存出现之前的行为逐字节相同，黄金网盯着。不额外提供 `cache rebuild`，那只是 `clear` + 一次全扫，输出没人消费。
- **并发安全**：删的是目录；正在读的进程已经把层读进内存，它在删除之后写出的层是合法的重建。同一个克隆的所有 worktree 共享这份缓存，一起被清。
- **与 `gc` 的分工**：`gc` 是日常回收（按可达集裁剪、合并层）；`clear` 是恢复（忘掉一切，用当前代码重建）。

内部开关：

- 包级 `cacheOff`，只有测试会置位；不是 flag，不读环境变量。
- `--no-cache` 从帮助文本、flag 表、配置注释、`lineNamesCommand` 的全局行豁免里删除。
- 顺带修一个已存在的 i18n 缺陷：配置种子的英文键在加入缓存注释后就对不上了（`KILN_LANG=en` 写出的 `.kilnconfig.jsonc` 仍是中文），本次改种子文本时一并修正，并加一条回归测试。

## 验收条件

1. `kiln cache clear` 清空后，下一次读输出与清空前逐字节相同，层与快照各重建一份。
2. `kiln cache clear` 在空缓存上是成功且安静的（不报错、退出 0）。
3. `--no-cache` / `KILN_NO_CACHE` 不再被接受：帮助里没有、`kiln list --no-cache` 按未知 flag 处理。
4. 黄金网仍做暖读/冷读对照（内部开关），60 份录制不改。
5. 英文模式下 `config path` 写出的种子是英文（新增回归）。
6. `go test ./...` 绿。

## apply

三块改动。

### 命令面

- `cacheCmd` 接两个子命令：`gc`（原样）与 `clear`。`kiln cache -h` 自动列出两条（帮助表加了 `kiln cache clear` 一行）。
- `cacheClear`：`cache.Scan` 数层、`ReadDir` 数快照 → `RemoveAll` 两个目录。空缓存输出"缓存是空的"并 exit 0；成功输出"已清空缓存：层 N、快照 M；下一次读会用当前版本重建"。
- 配置种子注释改成"缓存出错或升级后行为仍旧，用 kiln cache clear 清空"。

### 关断降级为内部开关

- `internal/cli/cache.go` 新增包级 `cacheOff`（带注释：只给测试，用户恢复靠 clear）；`cacheSite` 与 `enumSite` 读它。
- 删除：`env.noCache` 字段、`stripNoCache`、`KILN_NO_CACHE` 读取、帮助里的一行、`lineNamesCommand` 的 `--no-cache` 全局豁免、i18n 的两条映射。
- `retiredCommands` 加 `no-cache → cache clear`：`kiln no-cache` 会提示新名字。

### 测试与 i18n

- 28 处 `--no-cache` 调用点换成内部开关：`cache_test.go` 用 `uncached(t, …)`，`golden_test.go` 用 `runColdGolden`。黄金网仍是"暖读 vs 冷读逐字节相同"，只是冷读不再经过命令行。
- 新增三条：`TestCacheClearRebuildsFromScratch`（清空 → 重建 → 输出不变；空缓存二次清空）、`TestNoCacheFlagIsRejected`（只警告、不生效）、`TestDefaultFileIsTranslated`（英文种子无 CJK）。
- 黄金录制只改 `help.txt` 两份：多一行 clear、少一行 `--no-cache`。

### 验证

- `go test ./...` 绿（cli 21.6 s）。
- 手工：`cache clear` → "已清空缓存：层 1、快照 1"；再清 → "缓存是空的"；下一次读重建 1 层且输出不变；`kiln list --no-cache` → "警告: 未知 flag --no-cache"；`KILN_NO_CACHE=1` 不再生效（读仍写层）；`KILN_LANG=en` 的种子已是英文。

## 结论

- 用户面只有两条缓存命令：`cache clear`（恢复）、`cache gc`（回收）。没有"关缓存"开关。
- 内部 `cacheOff` 只给测试，黄金网的暖/冷对照因此保留；CLI 本身无法再关断。
- 顺带修好配置种子的英文回归（长期存在，之前无人发现）。

没做的：

- `cache rebuild`：`clear` 后的下一次读就是重建，单独命令只是重复，且清空后立刻重建会把"下一读"的成本提前，没换来别的东西。
- 快照键不含 git 版本、jj 无缓存、无 CI/跨平台验证：沿用 `cache-hardening` 的遗留清单，触发条件不变。
