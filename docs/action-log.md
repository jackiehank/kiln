# issue: action-log — `kiln action log`：全局动作流

- **domain**：cli
- **关联**：relates-to `action-view`（命名与形态已 conclude）；relates-to `action-list`（同组的词表命令）；relates-to `timeline-stream`（视图必要性、test 与验收分档）；relates-to `machine-readable-output`（NDJSON 例外与字段）

## 背景

`action-view` 已 conclude：`action` 组下，`action list` 列词表、`action log` 列动作。`action log` 是全局 action 视图：只留 `issue:` + `action:` 的提交，按 author-date 倒序，默认文本、`--jsonl` 给机器、`--limit` 首屏止损。

本 issue 只做 `action log`，不碰 `action list`。

## 要做的

```text
kiln action log [--jsonl] [--limit N]
```

- 全局动作流；不需要 `--all`；
- 默认文本：一行一条，含日期 / issue / action / 短 hash / subject；
- `--jsonl`：NDJSON，一行一个对象，字段 `issue / action / commit / date / title / domain / tags`；
- `--limit N`：最多 N 条动作后正常退出；N ≥ 1；
- git 流式；jj 显式非 0「暂不支持」，不静默空输出；
- 空仓/无动作 exit 0；broken pipe 及时退出。

## 要回答的问题

1. 流式后端接口：`vcs` 加可选的 `LogEach` / `LogStream`，还是别的形状？怎么复用 `StatLog` 的“可选接口”先例？
2. 文本布局与日期格式；是否要表头/legend；空结果是否给 stderr 提示。
3. `--jsonl` 的字段细节：`title` 取 subject 还是 trailer title；`domain` 是 per-commit 还是 issue 级；`tags` 是否带。
4. `--limit` 与 broken pipe 的退出码；Windows/Unix 差异怎么测。
5. jj 的行为与文案；未来要支持时的接口预留。
6. NDJSON 如何写进“一份 JSON 文档”规则；commit-graph 分档验收怎么写。
7. 与 `path --all --json`、`git log`、`list --search` 的边界与文档。

## 非目标

- 不做 `action list`；
- 不做 `path --timeline` / `path --all --jsonl`；
- 不做 `--all`、`--json`、`--stat`、过滤；
- 不做 `action show` / `action search`；
- 不做文本富 timeline（高亮/分组/交互）。
