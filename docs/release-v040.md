# issue: release-v040 — 发布 v0.4.0：先修 README 的缓存说明、命令表与版本示例

## 背景

- v0.3.0 于 2026-10-28 23:21 发布；此后 95 个提交：提交记录缓存（层 / 枚举快照 / GC）、
  `action log --jsonl` 的关系字段、内部 refactor 与性能收口。
- 发布就绪检查（2026-10-30）：`go test ./...` 全过，`go vet` / `gofmt -l` 干净，
  `kiln check` 80/81 完整（唯一进行中的 `doc-search` 是停放的 ask），工作区干净。
- CLI 面相对 v0.3.0 只多两条命令：`kiln cache gc`、`kiln cache clear`；`action log --jsonl`
  的 `refs` / `target` / `scope` / `relations` / `unrelate` 是纯增量字段；无破坏性变更
  （`--no-cache` / `KILN_NO_CACHE` 从未进过 release）。版本定 **v0.4.0**（minor）。
- 但 README 还差三处，发版前要修。

## 要做的

1. `README.en.md:210` 仍写 `--no-cache` / `KILN_NO_CACHE=1` 可按命令绕过缓存——两个开关已在
   `cache-recovery` 删除，中文版已改对；改成与中文版一致。
2. 两份 README 的命令表补 `kiln cache gc` / `kiln cache clear` 两行（本次 release 的新命令）。
3. 两份 README 的版本示例 4 处 → `v0.4.0`。
4. 刷新两份 README 的实测段：本仓 455 提交（原文 408），数字换成 2026-10-30 实测：
   本仓 `list` 151 → 25 ms、`action log --jsonl` 112 → 27 ms；100k：`graph --json` 1,022 → 144、
   `list` 1,035 → 146、`action log --jsonl` 1,173 → 166、`path --all --json` 1,084 → 203。

## 非目标

- 不改代码、不写 CHANGELOG（release notes 由 release workflow 从提交生成）、不批量改历史版本号。
- 本 issue 不 tag、不 push：发布动作由用户确认后单独执行。

## 验收

- `rg -n 'v0\.3\.0' README.md README.en.md` 无输出；4 处示例是 `v0.4.0`。
- `rg -n -- '--no-cache|KILN_NO_CACHE' README.md README.en.md` 无输出。
- 两份命令表都含 `kiln cache gc` / `kiln cache clear`。
- 实测段数字与 2026-10-30 的测量一致。
- `go test ./...` 仍全过（只改文档）。

## apply（2026-10-30）：README 四处更新

- `README.en.md`：删掉失效的 `--no-cache` / `KILN_NO_CACHE=1` 说明，改成与中文版一致（没有单命令开关；
  `kiln cache clear` 复位，`kiln cache gc` 日常回收）；「第一次读一次性多花几十毫秒」→ 本仓约 0.1 秒、100k 约 1 秒。
- 两份 README 的命令表各补 `kiln cache gc` / `kiln cache clear` 两行（`worktree remove` 与 `config path` 之间）。
- 版本示例 4 处 → `v0.4.0`（`README.md:45/59`、`README.en.md:43/57`）。
- 实测段换成 2026-10-30 的测量（本仓 455 提交）：`list` 151 → 25 ms、`action log --jsonl` 112 → 27 ms；
  100k：`graph --json` 1,022 → 144、`list` 1,035 → 146、`action log --jsonl` 1,173 → 166、`path --all --json` 1,084 → 203。
- 验证：`v0.3.0` 与 `--no-cache` / `KILN_NO_CACHE` 在 README 里无残留；两份命令表都含两条 cache 命令；
  `go test ./...` 全过（只改文档）；未 tag、未 push。