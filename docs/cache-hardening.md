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
