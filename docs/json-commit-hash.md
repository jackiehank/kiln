# issue: json-commit-hash — 哪些 `--json` 输出该带 commit hash

- **domain**：cli
- **关联**：caused-by `issue-list-scaling`（搜索 JSON 的 `hit` 讨论引出）；relates-to `json-fields`、relates-to `machine-readable-output`、relates-to `read-json`

## 背景

做 `list --search` 时用户提出：搜索的 `--json` 能不能顺带给出提交 hash，好用 git 定位到具体 commit。核对现状后发现不一致：

| 命令 | 现有键 | 含义 |
| --- | --- | --- |
| `path --json` | `actions[].id` | **完整 40 位 commit hash** |
| `show --json` | `id` | **完整 commit hash** |
| `list --json` | `id` | issue id（不是 hash） |
| `status --json` | `id` | issue id |
| `graph --json` | `nodes[].id` | issue id |
| `check --json` | `issue` / `action` | 无 hash |
| `worktree list --json` | — | 无 hash |

两个问题：

1. **同键异义**：`id` 在 `path` / `show` 里是 commit hash，在 `list` / `status` / `graph` 里是 issue id——同一个键名两种含义；
2. **缺地址**：除 `path` / `show` 外，消费方拿到一行（或一条 finding）无法直接 `git show` 对应提交，只能再跑一次 `path` 或自己查 git。

## 要回答的问题

1. **哪些出口确实需要** hash：`list` 行、`status` 行、`graph` 节点、`check` finding？还是只补 `list` / 搜索？
2. **键名**：`commit`（候选，避开 `id`）？还是 `hash` / `sha` / `revision`？
3. **形态**：完整 40 位（与 `path` / `show` 一致）还是短 hash？——倾向前者，短 hash 由消费方自己截。
4. **行上的 hash 指哪条提交**：**末条动作**（与 `action` / `done` / `title` 同源）？还是"最后一条能解释这条行的提交"？
5. **搜索的 `hit`**：保持字符串，还是升级为对象带上 `{ordinal, action, id, subject}`，让命中可直接定位？
6. **空值约定**：hash 恒写（issue 至少一个动作）还是 `omitempty`？没有动作的 issue 本来就不存在，倾向恒写。
7. **`status` 与 `list` 共享行契约**：加就两边一起加，还是只在 `list`？
8. **稳定性怎么写**：hash 随 rebase / amend 变，文档要写清"快照地址，不是主键"，主键仍是 issue id + 动作序号。
9. **`check --json` 的 finding** 要不要带 hash（指到出问题的动作）？这是独立缺口还是本 issue 一并做？

## 候选（待讨论）

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只给 list / 搜索加 `commit`（末条动作） | 行多一个键 | 命中早期动作时拿不到那条的 hash |
| **B** A + `hit` 升级为对象 `{ordinal, action, id, subject}` | 搜索命中可直接定位 | 改 `hit` 形态（刚加，无兼容负担） |
| **C** A + 平铺 `hit-commit` / `hit-ordinal` | 不破 `hit` 字符串 | 键更多、不如对象清楚 |
| **D** 全部出口（status / graph / check 也加） | 一致 | 面大，需逐条论证每个出口的用途 |

## 验收（若做）

- 每个确定的出口加上统一键 `commit`（完整 hash），空值约定写清；
- `path` / `show` 的既有 `id` 含义不动（或另起键并说明），不破坏现有消费方；
- JSON 契约只在**新增键**上变化；文本视图不变；
- 文档（`machine-readable-output.md` 各节 + README）同步；
- 测试覆盖"该有 hash 的行都有、不该有的不变"。

## 非目标

- 不改 `path` / `show` 的 `id` 键名（避免破坏消费方；真要统一另开 issue）；
- 不引入短 hash / 前缀解析；
- 不解决"按 hash 寻址"的命令面（`show <hash>` 之类）；
- 不动文本视图。
