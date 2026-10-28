# issue: action-view — 给“全局 action 索引”定命令名与入口

- **domain**：cli
- **关联**：relates-to `timeline-stream`（上一轮 conclude：需要 action 级全局视图）；relates-to `machine-readable-output`（NDJSON / 字段契约）；relates-to `path-batch`（批量读取先例）

## 背景

`timeline-stream` 已 conclude：kiln 需要一个 action 粒度的全局视图；它不是 `git log` 包装，而是 `list`（issue 索引）在 action 维度上的对称物。上一轮只定了“要这个视图”，没有定命令名与入口。

用户倾向 `action list` 之类的命名；本 issue 先收集类似 CLI 的子命令命名方式与 kiln 已有命令，再 hypothesize 选名，最后 apply。

## 要定的问题

1. **命令形态**：`kiln action list`（名词组 + 动词）、`kiln actions`（复数名词组）、`kiln log`（顶层动词）、`kiln timeline`（视图名）还是别的？
2. **单复数**：kiln 现有名词组用单数（`worktree`、`config`）；`actions` 复数是否值得为了短而破例？
3. **动词**：`list` 还是 `ls`？kiln 已有命令全用全词；`gh` / `docker` 的 `ls` 是别名而非主名。
4. **分组边界**：如果叫 `kiln action list`，`action` 组以后会不会长出 `show` / `search`？它们与现有 `show <issue> n`、`list --search` 的边界是什么？
5. **与 `list` 的区分**：顶层 `list` 是 issue 列表；`action list` 是动作列表。名字要能一眼区分，而不是让 `list` 兼职。
6. **交付契约**：`--jsonl`、`--limit`、字段、NDJSON 例外、jj、broken pipe / 空仓——留给后续 apply；本 issue 只定名与入口。

## 非目标

- 不改代码；不决定“现在做还是等 webui/tui”；
- 不新增模型字段 / 校验；
- 不讨论 `path --timeline` 之外的入口（`timeline-stream` 已排除 `path` 模式）。
