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

## 收集（2026-10-27）：谁在消费 JSON，hash 现在/将来有什么用

### 1. 消费者盘点

| 消费者 | 读哪些 JSON | 用 hash 吗 | 备注 |
| --- | --- | --- | --- |
| dsh-kiln 插件 | `graph --json` + `path <id> --json` | **是，但来自 path** | `flow.js` 保留完整 sha（缓存增量失效 + timeline"跳回提交"）与 7 位短哈希（展示）；**不调用** `list` / `status` / `check` |
| kiln 自身 CLI / 测试 | 无（直接读 model） | — | 内部不经过 JSON |
| CI | 无 | — | `.github/workflows` 只有 release.yml，不调 kiln 读命令 |
| listenloop 等其他仓 | 文档只要求 `kiln check` 通过 | 否 | 没有代码从 JSON 取数 |
| agent / 临时脚本 | 任意 | 视查询而定 | `list --search --json` 是新开的口子 |

结论：**已知的下游消费者（dsh-kiln）不从 `list` / `status` / `graph` / `check` 取 hash**；它要的 hash 全部来自 `path`。

### 2. hash 现在已经能一次拿全

- `path <id> --json`：每个动作的 `id` 就是完整 40 位 hash；
- `path --all --json`：61 条 issue / 269 动作 / **125,731 B**，一次进程给出全仓 hash；
- 对照（同一时点）：`list --json` 15,890 B、`status --json` 17,742 B、`graph --json` 26,526 B、`check --json` 12,645 B；
- 行级加一个 40 位 hash 约 **+41 B/行**：61 行 ≈ **+2.5 KB**（`list` 约 +16%，`status` 约 +14%）。

也就是说：加 hash 省的是**取数的调用次数/体量**，不是新能力——`path --all` 已经能一次给全，只是贵 8 倍。

### 3. 既有分层：索引 vs 明细

- `docs/machine-readable-output.md` §3 写过：「`status` 只给行，不给动作明细：动作清单是 `path` 的活。这样 `status --json` 的大小是 O(issue)，不是 O(commit)——索引与明细分层正是 `path`/`show` 已定的那条分工。」
- 一个"末条动作 hash"仍然是 O(issue)，不违背体量；但它是 `path` 已有事实的**第二份拷贝**，schema 面变大。
- `show` 的寻址用**序号/动作名**（不接 hash），所以 hash 的用途是**git 侧定位**，不是 kiln 侧寻址。

### 4. 先例

- `gh issue list --json`：**没有** commit hash（issue 不是提交）；
- `gh pr list --json`：有 `headRefOid` / `baseRefOid` / `mergeCommit`（PR 行对应提交，就带上）；
- 所以"行对应提交就带 OID"是通行做法。kiln 的 issue 行算不算"对应提交"，取决于把哪条动作当作行的事实来源——现在 `action` / `done` / `title` 都取**末条动作**，末条 hash 是最自洽的候选。

### 5. 唯一有真实需求的缺口：搜索的 `hit`

- `list --search` 的 `hit` 指向**早期动作**，而行的 `action` / `title` 是末条动作的事实；
- 若 hash 只给末条，命中早期动作时仍拿不到那条 hash——还得再跑一次 `path`，正好抵消了搜索"一次拿到"的意义；
- 若把 `hit` 升级为 `{ordinal, action, id, subject}`，命中就自带地址：hash 给 git，ordinal 直接喂 `kiln show`。

### 6. 各出口的"需要程度"（证据视角，不裁决）

| 出口 | 现有 hash | 已知需求 | 证据 |
| --- | --- | --- | --- |
| `list` 行（末条） | 无 | 暂无已知消费者；agent 可能 | 搜索场景真正要的是**命中动作**的地址，不是末条 |
| `status` 行 | 无 | 无 | 与 `list` 共行契约；dsh-kiln 不用 |
| `graph` 节点 | 无 | 无 | 节点是 issue 级；插件用 `path` 取 hash |
| `check` finding | 无 | 无 | finding 已给 `issue` + `action`，`path` 可定位 |
| `path` / `show` | **有** | dsh-kiln（缓存失效 + 跳回提交） | 已满足 |

### 7. 由此得到的三条讨论输入

1. **只做搜索**：`hit` 对象化（带 `{ordinal, action, id, subject}`），其余出口不动——证据最强、面最小；
2. **搜索 + list 行末条**：行级 `commit` 给"这条 issue 的最新提交"，`hit` 对象给命中动作——两件事分开、语义清楚；
3. **全加**（status/graph/check 也带）：目前没有消费者，属于为一致性买单，需要逐出口论证。

另外两个事实要带进讨论：hash 随 rebase / amend 变（只能当快照地址，不是主键）；`id` 在 `path`/`show` 是 hash、在 `list`/`status`/`graph` 是 issue id（新键不能叫 `id`）。
