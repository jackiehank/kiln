# issue: cli-consistency — 命令面与输出的歧义/不统一盘点

- **domain**：cli
- **关联**：caused-by `json-commit-hash`；relates-to `machine-readable-output`、relates-to `agent-ux-review`、relates-to `silent-flag-drop`

## 背景

`json-commit-hash` 发现 `id` 在 `path` / `show` 里是提交 hash、在 `list` / `status` / `graph` 里是 issue id——同一个键两种含义。这类"看起来一致、其实不一致"的地方不止一处；用户要求系统盘一遍，**稍后统一处理**。

本 issue **只盘点，不改行为**：把歧义/不统一列清楚、给出候选与影响面，修由后续 issue 或本 issue 的 apply 分批做。

## 盘点维度

1. **JSON 键名与语义**：`id` / `commit` / `action` / `done` / `status` / `filter` / `hit` / `search` / `tags` / `title`；单复数、kebab-case、`omitempty` vs 恒写、布尔恒写。
2. **同一概念在不同出口的漂移**：`status` 行 vs `list` 行 vs `graph` 节点 vs `path` / `show` 动作；`done` 与 `check` 的 terminal 判定。
3. **命令与 flag 形态**：`status` vs `list`；`path` vs `show`；`--status=open|done` 的取值；`--search` vs `--match`；`--all` / `--only` / `--force`；未知 flag 的 warn vs error。
4. **退出码与错误通道**：无命中 0 vs grep 1；非法值 2；stderr prose vs JSON rule 码；批量部分失败（`path --all` 的 errors + exit 1）。
5. **文本与 JSON 对齐**：`title` / `commit` 是否只在 JSON；空结果提示；`path` 文本里的 `←` 只在 `--search` 出现。
6. **术语**：done / terminal / 完成 / 进行中；worktree vs workspace（历史命令已改名）；problem / object（历史）；"issue" 与"问题"。
7. **空态与提示**：空仓、无命中、缺 worktree、缺配置、未知 domain。

## 已知样例（本轮及近几轮发现的）

| 位置 | 现象 | 现状 / 候选 |
| --- | --- | --- |
| `path.actions[].id` vs `list.id` | 同键异义 | 正由 `json-commit-hash` 统一：`id` = issue、`commit` = hash |
| `--status=open|done` | `done` = 末条是终点，不是永久关闭 | 文档已说明；命名是否够清楚待评 |
| 未知 flag | 警告后继续 → 可能静默回全量 | 是否对读命令改 error（至少带新过滤 flag 的命令） |
| 无命中 | `list` 家族 exit 0 + stderr 提示；grep 家族 exit 1 | 是否统一、写进契约 |
| `hit` / `match` / `search` | gh 的 `--match` 是字段收窄、`--search` 是查询 | 已避开；继续观察 |
| `status` 文本无 `title`，`status --json` 有 | 文本 / JSON 信息面不同 | 是否有意为之，写清 |
| `path` 文本的 `←` 只在 `--search` | 同一命令两种布局 | 文档化或统一 |
| 批量部分失败 | `path --all` 单条失败进 `errors`、exit 1 | 与 `list` 空结果 exit 0 的口径如何对齐 |

## 验收（盘点完成时）

- 每个维度至少一条**实测样例**（命令 + 现在的输出 + 为什么有歧义/不统一）；
- 每条给出候选与影响面（改 JSON / 改文本 / 改退出码 / 只改文档）；
- 分类：**必须改 / 可改可不改 / 保持**；
- 不在这条 issue 里改任何行为；修复分批进行，每批一个动作。

## 非目标

- 不重命名命令（除非盘点后单独立项）；
- 不引入查询语言 / 搜索语义；
- 不改 `check` 的规则集；
- 不新增输出格式（TOON 等）；
- 不动 `status` 的领域分组语义。
