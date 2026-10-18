# issue: relations-impl — 落地关系机制：可配置词表 + 槽/强度取强 + 反向推导 + unrelate

- **domain**：design
- **关联**：derived-from `relation-revision`、relates-to `silent-flag-drop` / `help-relation-flags`

## 背景

`relation-revision` 已 conclude 机制（键 `(无序对,槽)` + 槽内强度格取强 + 读侧 LWW + `unrelate` 墓碑 + 词表可配置、结构引擎固定）。本 issue 把它**落地成代码**。

当前实现是**硬编码 5 键**（`derived-from`/`spawns`/`relates-to`/`blocks`/`supersedes`），且：

- 新名 `causes`/`caused-by`、墓碑 `unrelate` **一律被静默丢弃**（`relation-revision` 的最小仓实测）。
- 聚合是 `appendUnique` 并集（只增不减），没有槽 / 强度 / 取强。
- 没有反向推导 / 入边索引。
- `-h`、flag 解析、摘要输出三处硬编码。

## 落地清单（分阶段）

| 阶段 | 内容 | 主要文件 |
| --- | --- | --- |
| **P0 配置** | 新增 `relations.{slots,names,aliases}`；默认 = 9 名 + 2 别名 | `config.go` |
| **P1 trailer** | 配置驱动解析/输出；识别 `unrelate`；未知键不再静默（接 `silent-flag-drop`） | `trailer.go` |
| **P2 model** | 5 个固定字段 → 通用表示；按 `(对,槽)` 单值 | `model.go` |
| **P3 engine** | 折**取强**；**入边索引** + 反向推导；`relationLines` 按槽；`check` 扩展 | `engine.go` |
| **P4 cli** | flag 与摘要改为配置驱动（`-h` 归 `help-relation-flags`） | `cli.go` |

## 验收标准

1. 新名 `causes`/`caused-by` 可写、可读；旧名 `derived-from`/`spawns` 经 alias **仍可读**（本仓 16 条旧边）。
2. 同一 `(无序对,槽)` 取强：`relates-to` 被 `causes` 升档；不产生重复边。
3. `unrelate: X` 作废该对全槽；同动作内声明优先。
4. `path` 默认只显当前态；反向边（X 为目标的边）能被渲染。
5. `check` 对全部写侧名 + `unrelate` 的目标做存在性校验（⚠）。
6. `go build` / `go test ./...` 通过。

## 非目标

- 不改词表本身（`relation-vocabulary` 已定稿）。
- 不改渲染规格（`relation-rendering` 已定稿）。
- `-h` 的分组/动态生成（归 `help-relation-flags`）。
- 不做 YouTrack 式自动关/开父项（`part-of` 已定：只做 `check` 软警告）。

## 证据

- `relation-revision` 的 conclude（机制）+ 两个 test（改动面、最小仓静默丢弃）。
- `internal/{model,trailer,engine,cli,config}` 的硬编码盘点。
