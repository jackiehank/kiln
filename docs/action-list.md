# issue: action-list — `kiln action list`：列出生效的 action 词表

- **domain**：cli
- **关联**：relates-to `action-view`（命名与形态已 conclude）；relates-to `action-log`（同组的事件视图）

## 背景

`action-view` 已 conclude：`action` 组下，`action list` 列词表、`action log` 列动作。action 词表可由仓库根 `.kilnconfig.jsonc` 的 `actions` 覆盖；现在没有专门的“列出当前可用 action”的命令，用户只能读配置文件、看 `-h`（它写死默认词表），或触发 `do` 的“非法 action”错误。

本 issue 只做 `action list`，不碰 `action log`。

## 要做的

```text
kiln action list [--json]
```

- 输出**生效**的词表：`names`、`start`、`terminal`；
- 文本：一行一个 action，标注 `start` / `terminal`；
- `--json`：一份文档，形状与 `.kilnconfig.jsonc` 的 `actions` 组对齐，外加 `source` / `config`；
- 不读 git 历史、不需要仓库；仓外/未初始化时用默认词表，`source=default`；
- 与 `do` / `commit` 共用同一个 `ActionSet`，列出的就是写侧接受的集合。

## 要回答的问题

1. 文本的标记格式：列对齐，`start` / `terminal` 用词还是符号？要不要表头/legend？
2. `--json` 的精确形状：`actions: {names,start,terminal}` 还是平铺？`source` / `config` 如何省略？
3. `source=file|default` 怎么判：`Config.Path` 是否等于“文件存在”？要不要给 `Config` 加字段？
4. 仓外、配置缺失、配置解析失败时分别输出什么？`action list` 是否也要在配置错误时失败？
5. `-h` 现在写死默认词表；本 issue 要不要顺手把帮助里的 action 词表改成生效词表，还是留给后续？
6. 测试、README、`-h`、i18n（zh/en）要覆盖哪些面？

## 非目标

- 不做 `action log`；
- 不做 `action show` / `action search`；
- 不写/生成配置文件（`config path` 已有 seed 行为）；
- 不新增模型字段或校验规则；
- 不改 `ActionSet` 的语义。
