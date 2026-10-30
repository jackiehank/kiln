# issue: release-v041 — 发布 v0.4.1：版本示例与实测段

## 背景

- v0.4.0 于 2026-10-30 发布（tag `v0.4.0` → `b11db2b`）；此后 7 个提交，全是性能收口与文档：
  `cache-precompute`（外推 / 原型实测 / apply）与 `path-incident-index`（apply）。
- 代码只落在 `internal/engine`（折叠）与 `internal/cli/path.go`（邻接索引）：`go.mod`、`help.go`、`i18n`
  无 diff，命令面与 flags 不变，20 组命令输出与 B1 版逐字节一致。按 semver 是 **patch**：v0.4.1。
- 缓存格式没动，升级后不需要 `kiln cache clear`。
- README 有两处要跟着走：版本示例 4 处仍是 `v0.4.0`；实测段还是 v0.4.0 的数（本仓 455 提交、
  当时的合成 100k 仓），而 `path --all` 与 `graph` 的二次项已经修掉，数字要重测重写。

## 要做的

1. 两份 README 的版本示例 4 处 → `v0.4.1`（`README.md:45/59`、`README.en.md:43/57`）。
2. 实测段（`README.md:215` / `README.en.md:215`）换成 2026-10-30 的测量，并写清仓的形状：
   - 本仓 469 提交：`list` 114 → 28 ms、`action log --jsonl` 114 → 29 ms；
   - 100k 合成仓（80k 动作 / 16k issue，每动作 1 条关系，≈80k 边）：`list` 978 → 162、`action log --jsonl`
     800 → 220、`graph --json` 1,076 → 236、`path --all --json` 1,155 → 320；
   - 关系密集（每动作 3 条关系 + 每第 4 个动作 unrelate，≈240k 边）：热读 `graph --json` 545、
     `path --all --json` 706——v0.4.0 在同形状仓上是 51 秒 / 16 秒，两个二次项。

## 非目标

- 不改代码、不写 CHANGELOG（release notes 由 release workflow 从提交生成）。
- 不 tag、不 push：发布动作由用户确认后单独执行。

## 验收

- 两份 README 的版本示例 4 处都是 `v0.4.1`（实测段里对 v0.4.0 的一次对比保留）。
- 两份 README 的实测段数字与 2026-10-30 的测量一致（`.bench/readme.ps1` 的 tsv）。
- `go test ./...` 全过、`kiln check` 退出 0（只改文档）。

## apply（2026-10-30）：README 版本示例与实测段

- 版本示例 4 处 → `v0.4.1`（`README.md:45/59`、`README.en.md:43/57`）。
- 实测段换成 2026-10-30 的测量：本仓 469 提交 `list` 114 → 28 ms、`action log --jsonl` 114 → 29 ms；
  100k 合成仓（80k 动作 / 16k issue）冷读 → 热读 `list` 978 → 162（−83%）、`action log --jsonl` 800 → 220（−73%）、
  `graph --json` 1,076 → 236（−78%）、`path --all --json` 1,155 → 320（−72%）。
- 新增一条关系密集仓（≈240k 边）的热读：`graph --json` 545 ms、`path --all --json` 706 ms，并注明
  v0.4.0 在同一形状仓上是 51 秒 / 16 秒——两个二次项已修掉，热读回到线性。
- 测量方式（`.bench/readme.ps1`，不进仓）：本仓 + plain-100k + dense-100k，`timeit` 冷读 3 次 / 热读 5 次取中位数。
- 验证：`go test ./...` 全过，`go vet` / `gofmt -l` 干净，`kiln check` 退出 0；未 tag、未 push。
