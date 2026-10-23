# issue: json-fields — `status` 行补 `title`、`check` 的关系诊断补 `relation`

- **domain**：cli
- **关联**：relates-to `machine-readable-output`、relates-to `graph-export`

## 背景

agent 访谈（2026-10-23）里发现两处既有 JSON 的小缺口，同属"字段少了，机器得回头再查文本"：

1. `status --json` 的行只有 `id` / `domain` / `action` / `done`，agent 知道一条 issue 走到哪一步，却不知道它是**什么事**；而 `graph --json` 的节点已经带 `title`——同一个概念在两条命令里不一致。
2. `check --json` 的 `relation-target` finding 只有 `ref`（缺失的靶 id），没有关系名。同一条 issue 对同一个靶声明了两条不同关系（比如 `blocks` 与 `relates-to`）时，调用方分不清坏的是哪一条。`machine-readable-output` 的结论已把它记为"留档缺口"。

## 问题

1. `status` 行的 `title` 取哪个动作的标题？决策：**最后一个动作**的 `--title`，与 `graph --json` 节点同一来源、同一时点；不发明"issue 的名字"这种第二语义。
2. `check` 的 finding 加什么键——`relation`（声明名）还是 `slot`？决策：`relation`，与 trailer / 关系词表同名；只在 `relation-target` 出现（`omitempty`），其它规则不带。
3. 兼容性：两处都是**加键**，缺键=无的既有约定下旧调用方不受影响；`level` / `ref` 语义不变。

## 验收

- **E1**：`kiln status --json` 每行带 `title`（该 issue 最后一个动作的标题；无动作为空则省略）。
- **E2**：`kiln check --json` 的 `relation-target` finding 带 `relation`（声明名），`ref` 仍是被指向的 id。
- **E3**：其它 rule（`start-action` / `terminal-action` / `unknown-trailer` / `complete`）不带 `relation`。
- **E4**：两命令的文本视图**逐字节不变**；既有 JSON 行的键序与其它字段不变。
- **E5**：`go build ./...` / `go test ./...` 全过；真仓 `status --json` / `check --json` 冒烟。

## 非目标

- 不给 `list --json` 加 `title`（`status` 与 `list` 的公共行 `issueCore` 保持不变，`title` 只属于 `status` 的行）；
- 不动 `check` 的 rule 码集合与退出码；
- 不把 `slot` 暴露出来。
