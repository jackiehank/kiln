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

## hypothesize（2026-10-27）：只给 `list` 行加 `commit`，`hit` 升级为动作地址

> 主张：**`list --json` 的行加 `commit`（末条动作的完整 hash）；`hit` 从字符串升级为动作地址 `{ordinal, action, commit, subject}`；`path` / `show` 的 `id` 也统一改叫 `commit`（§7 修订）。`status` / `graph` / `check` 暂不加。** 理由不是"一致性"，而是 collect 里唯一有真实需求的那条：搜索/筛选 → 下一步是**看提交**。

### 1. 主张

| # | 主张 | 证伪条件 |
| --- | --- | --- |
| **H1** | `list --json` 每行加 `commit` = **末条动作**的完整 40 位 hash（与行上的 `action` / `done` / `title` 同源） | 真实使用中，行级 `commit` 没人用（大家只从 `hit` 取地址），且 +2.5 KB 被嫌弃 |
| **H2** | `--search` 命中早期动作时，`hit` 升级为 `{ordinal, action, commit, subject}`：`commit` 是完整 hash、`ordinal` 可直接喂 `kiln show` | 消费者更愿意保持 `hit` 字符串、自己再跑 `path`；或 `hit` 对象带来的嵌套让解码变麻烦 |
| **H3** | `status` / `graph` / `check` **本轮不加** hash | 出现真实消费者（脚本/插件/agent）需要从这三个出口直接定位提交 |
| **H4** | 统一键名：**`id` 只指 issue、`commit` 只指提交 hash（完整 40 位）**；`path` / `show` 也改（§7） | 下游（dsh-kiln）来不及迁移，或改动成本超出预期 |
| **H5** | hash 恒写（issue 至少一个动作），`hit` 缺省即省略；文本视图不变 | 空动作 issue 出现（目前模型上不可能） |

### 2. 依据（来自 collect）

- **唯一已知消费者 dsh-kiln 不从这些出口取 hash**：它读 `graph` + `path`，hash 全部来自 `path`；`list` / `status` / `check` 它根本不调。所以"全加"没有消费者支撑。
- **hash 已能一次拿全**：`path --all --json` 125.7 KB 给出全仓；`list` 加 hash 后 ≈18.4 KB——省的是 8 倍的取数体量，不是新能力。
- **搜索是唯一"行的事实来源 ≠ 命中来源"的出口**：`hit` 指早期动作，行上的 `action`/`title` 是末条。因此行级 `commit`（末条）与 `hit.commit`（命中）**必须分开**，各回答一个问题：
  - "这条 issue 的最新提交是哪一次？" → 行级 `commit`
  - "为什么这条命中、那次提交在哪？" → `hit`
- **先例**：`gh issue list --json` 无 commit；`gh pr list --json` 有 `headRefOid` / `baseRefOid` / `mergeCommit`——**行对应提交就带 OID**。kiln 的 `list` 行现在 `action`/`done`/`title` 都取末条动作，带末条 hash 与这条先例同向。
- **`show` 的寻址是序号/动作名，不接 hash**：所以 `hit.ordinal` 是 kiln 侧地址、`hit.commit` 是 git 侧地址，两个都要。

### 3. 若成立，契约

行（`list --json`）：

```json
{
  "id": "help-relation-flags",
  "domain": "cli",
  "action": "apply",
  "done": true,
  "title": "图例去掉冗余的必填/选填：<> 只是值占位；必填信息只留「do/commit 标题（位置参数 vs flag）」与「--force 条件必填」两处",
  "commit": "e41ac6a8f1bdcb441f8917d0755d5c8653bbb975",
  "hit": {
    "ordinal": 9,
    "action": "apply",
    "commit": "53c8fd12edd73db804bb35e8475ac37a1eb6867a",
    "subject": "help 标注必填/选填：do/commit 必填=位置参数 issue+action、flag 均选填；--force 标为条件必填（worktree=on 在 main 提交）；图例补 <> 必填占位 / [] 选填"
  }
}
```

（这是真仓 `list --search=worktree` 命中 `help-relation-flags` 的实际数据：`commit` 是末条 `apply`，`hit` 指向第 9 条也含 "worktree" 的 `apply`。）

- `commit`：**恒写**，末条动作的完整 hash；行级 `id` 仍是 issue id，两者不混。
- `hit`：仅当 id + 标题解释不了命中、且**某一条动作**含全部查询词时出现；字段 `ordinal`（1-based，`path` 序号）/ `action`（动作名）/ `commit`（完整 hash）/ `subject`（该动作首行）。词分散在多条动作时仍省略。
- 文本视图不变：仍是 `id + state + title`，命中动作时 `← <subject>`。
- `status` / `graph` / `check` 的 JSON **一个键都不加**；`path` / `show` 的 `id` 改名 `commit`（见 §7）。
- 文档写清：hash 是**取数时的快照地址**（rebase / amend 会变），不是主键；稳定寻址用 issue id + 动作序号。

`path --json` 的动作（改名后）：

```json
{ "ordinal": 9, "action": "apply", "commit": "53c8fd12…（40 位）", "date": "…", "title": "…", "refs": ["…"] }
```

`show --json`（改名后）：

```json
{ "issue": "help-relation-flags", "ordinal": 9, "total": 10, "action": "apply", "commit": "53c8fd12…", "date": "…", "title": "…", "body": "…" }
```

### 4. 为什么不加 `status` / `graph` / `check`

- `status` 的职责是"全库概览"（分组 + 计数），不是"找到某条去看提交"；它与 `list` 共享的行契约是 `id/domain/action/done/title`，多出来的字段本来就不要求两边一致（`list` 多 `tags`）。
- `graph` 的节点是 issue 级图谱；要提交就该去 `path`（插件正是这么做的）。
- `check` 的 finding 已经给 `issue` + `action`，要定位提交跑一次 `path` 就够；给 finding 加 hash 属于"验证输出"的另一个决定。
- 一句话：**没有消费者，就先不制造第二份事实**。等真有人从这三个出口要提交，再按同一个键名加。

### 5. 检验办法

1. **体量**：`list --json` 从 15.9 KB → 约 18.4 KB（61 行 × ~41 B）；`hit` 对象只在搜索命中的少数行出现（真仓 `worktree` 10 条里 2 条）。
2. **可用性**：拿一条 `hit` 行，`git show <hit.commit>` 应打开命中的动作；`kiln show <id> <hit.ordinal>` 应打开同一条（两者互证）。
3. **一致性**：行级 `commit` 必须等于 `path <id> --json` 的 `actions[-1].commit`；加一条测试钉住。
4. **不越界**：断言 `status` / `graph` / `check` 的 JSON 里没有 `commit` 键（防止以后顺手加）。
5. **命名**：`hit.commit`、`path.actions[].commit`、`show.commit` 同义（都指提交 hash）；`id` 只出现在 issue 级——用一条文档 + 一个契约测试钉住。

### 6. 反方（保留）

- **`status` 也会被 agent 当入口**：如果以后 `status --json` 成为"先看全库，再点开某条"的默认入口，行级 `commit` 在那里同样有用——那时 H3 被推翻，按同一键名补上即可。
- **`hit` 对象 vs 字符串**：对象更完整，但让"只想取一行文本"的消费方多走一层；如果实测发现大家只读 `subject`，可以再考虑加一个平铺的 `hit-subject`，但不建议现在就双写。
- **hash 的时效**：任何人把 `commit` 缓存下来跨 rebase 使用都会失效；文档只能提醒，挡不住误用。

### 7. 修订（2026-10-27，用户决定）：`path` / `show` 也统一为 `commit`

- **推翻 ask 的非目标**（原文"不改 `path` / `show` 的 `id` 键名"）：kiln 尚未正式发布，现在改代价最小；统一后规则只剩一条——**`id` 只指 issue，`commit` 只指提交 hash（完整 40 位）**，所有出口一致。
- 改动面：
  - `path --json`：`actions[].id` → `commit`；
  - `show --json`：`id` → `commit`；
  - `list --json`：行级 `commit`（末条动作）+ `hit.commit`（命中动作）；
  - `status` / `graph` / `check`：仍不加 hash（H3 不变）。
- **下游迁移**：dsh-kiln 的 `flow.js` `mapActions` 现在读 `action.id`（完整 sha）与 `sha.slice(0,7)`，需改成 `action.commit`；这是同一轮消费侧更新，不阻塞本仓落地（kiln 未发布，插件是本地目录）。
- 测试影响：所有断言 `path` / `show` 的 `id` 的用例改成 `commit`；新增一条契约测试——"`id` 只出现在 issue 级、`commit` 只出现在提交级"（扫 JSON 键，不是逐条比对）。
- 文档：`read-json.md`、`machine-readable-output.md`、README、`-h` 里的示例全部同步；`show` 的示例也改。
- 风险：这是一次**破坏性改名**，但无正式版本、唯一已知消费者可控；`hit` 本身也还没发布，一起改成本最低。
