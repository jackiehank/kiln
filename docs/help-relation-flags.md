# issue: help-relation-flags — `-h` 里关系 flag 与普通 flag 混排，且硬编码

- **domain**：cli
- **关联**：derived-from `relation-revision`、relates-to `help-alignment` / `help-self-contained`

## 背景

`kiln -h` 的 `do/commit 的 flags:` 一段把两类东西混在一起：

```
  --title  --refs  --target  --scope  --domain  --tag
  --derived-from  --spawns  --relates-to  --blocks  --supersedes
```

- 第一行是**元数据 / 归类 flag**（标题、引用、目标、范围、domain、tag）；
- 第二行是**关系 flag**，而且这 5 个名是**硬编码**在 `usageFooter`（`internal/cli/cli.go:34-36`）里的静态字符串。

两个问题叠在一起：

1. **分组不清**：关系 flag 没有自己的小节 / 说明，读者看不出它和前一行是两类。
2. **硬编码脱节**：关系词表已定要**可配置**（`relation-revision`），定稿名是 `causes` / `caused-by` 等，还多了 `unrelate`；`-h` 若仍列旧的 5 个名，就会显示不存在的能力、并漏掉新能力。

## 要回答的问题

1. **怎么分组**：`do/commit 的 flags` 是否拆成「通用 flag」+「关系 flag」两段？关系段要不要一句说明（如"关系：写一侧、读侧推导反向"）？
2. **动态还是静态**：关系 flag 列表是否从 `relations.names`（配置）**生成**？若是，`usageText()` 需要拿到配置。
3. **`unrelate` 放哪**：tombstone 作为关系段的一员，还是单独一行（“撤销关系”）？
4. **与其它帮助内容的一致性**：action 词表已经是**从配置读**的（`-h` 里 "action 词表默认: …"）——关系段应否照同一模式？
5. **自定义词表**：项目自定义关系名不同时，`-h` 是否反映？

## 候选（草案）

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只分组 | 关系 flag 挪到独立小节，仍静态 | 最小；仍与配置脱节 |
| **B** 分组 + 动态 | 关系段从 `relations.names` 生成 | 要打通配置到 `usageText` |
| **C** 不列具体名 | 只写"关系 flag：见 `relations` 配置 / `kiln config`" | 帮助不会过期，但可发现性差 |

## 非目标

- 不改关系机制 / 词表本身（归 `relation-revision` / `relation-vocabulary`）。
- 不改 `-h` 其它段落（除非为一致性）——那是 `help-alignment` 的地盘。

## 证据

- `internal/cli/cli.go:34-36`：`usageFooter` 硬编码 5 个关系 flag。
- `internal/cli/cli.go:323-327`（flag 解析）、`:1282-1286`（摘要输出）：同源硬编码。
- `relation-revision` 的 test（2026-10-18）：新名 `causes:` / `unrelate:` 当前被**静默丢弃**；词表将可配置。

## collect：CLI 怎么暴露「多种关系参数」，以及帮助分组（2026-10-18）

> 起因（用户）：本 issue 不只是 `-h` 分组——关系有好几类，`do`/`commit` 该不该给关系参数做**分组 / 专门设计**，也一并想。先看同行。

### 0. 本仓现状（`relations-impl` 落地后）

- `do`/`commit` 的 flag **已经是从词表生成**的（`e.rel().Names()`）——新名 `--causes` / `--caused-by` / `--part-of` … 已可用，不再是 5 个硬编码。
- 但 `-h` 的 `usageFooter` 仍是**静态字符串**（手工列了 10 个关系 flag），**与配置可能脱节**。
- → 本 issue 现在只剩两问：**帮助怎么组织** + **参数形态要不要改**。

### 1. 业内暴露关系的三种形态

| 形态 | 代表 | 例子 | 优点 | 代价 |
| --- | --- | --- | --- | --- |
| **每类一个 flag**（flat） | `gh issue` v2.94.0 | `--parent` / `--set-parent` / `--remove-parent`；`--blocked-by` / `--blocking` / `--add-*` / `--remove-*` | 可发现、有 shell 补全 | flag 数随词表增长；帮助臃肿 |
| **通用带名 flag** | `git commit --trailer`；Jira CLI `--link`；`--type` | `--trailer "<token>[(=|:)<value>]"`（可重复）；`jira --action linkIssue --link "Relates"` | **与词表解耦**：自定义名直接可用 | 无逐名发现 / 补全；需自己校验 |
| **子命令** | `jira issue link`；`jtk links create/list/delete/types` | `jira issue link A --to B --type Blocks` | 命名空间干净 | 与 kiln 的动作提交模型不合（关系得写进历史） |

### 2. 帮助分组是成熟做法

- **Cobra flag groups**：用 flag annotation 分组，帮助按组渲染（例：`entireio/cli` 把 flag 分成 Scope / Navigation / Filtering & Sorting / Formatting，且规定“Scope 永远最先”）。— 一手。
- GitHub CLI 的帮助也按主题组织。

### 3. 三条对 kiln 的观察（不下结论）

1. **子命令不合适**：kiln 的关系是**动作提交里的 trailer**（写一处、读侧推导）；`kiln relate ...` 要么得写进历史、要么变成第二条边，与 `relation-revision` 的**单值槽**冲突。→ 参数仍应挂在 `do`/`commit` 上。
2. **通用 `--relate name=target` 值得考虑**：它就是 `git --trailer` 的形态，且**天然适配“词表可配置”**（自定义名不用改 CLI）；代价是可发现性差、要额外校验。
3. **flat 逐名 flag + 帮助分组**：保留 `--causes` 这类逐名 flag（可发现 / 可补全），但在 `-h` 里给它们一个**独立小节**（Cobra 式分组），并**从词表动态生成**；`unrelate` 放进关系小节。

→ 可能的组合：**flat 逐名 flag（词表生成）+ 通用 `--relate`（逃生口）+ `-h` 关系小节（动态）**。

### 4. 来源

- `git commit --trailer <token>[(=|:)<value>]`（可重复、自由 token）：[git-commit](https://git-scm.com/docs/git-commit)（一手）。
- GitHub CLI v2.94.0 的 issue 层级 / 依赖 flag：`gh issue --parent / --blocked-by / --blocking`（[GitHub Changelog](https://github.blog/changelog/2026-06-10-manage-sub-issues-types-and-dependencies-from-github-cli/)）（一手）。
- Jira CLI 通用 `--link`：`jira --action linkIssue --issue X --toIssue Y --link "Relates"`（[Appfire JCLI linkIssue](https://appfire.atlassian.net/wiki/spaces/SUPPORT/pages/89147359/How+to+use+linkIssue)）（一手 / 厂商文档）。
- 子命令式关系管理：`jira issue link`（[matcra587/jira-cli](https://pkg.go.dev/github.com/matcra587/jira-cli@v0.9.1/internal/cli/issue)）、`jtk links create ... --type`（[atlassian-cli #148](https://github.com/open-cli-collective/atlassian-cli/issues/148)）（二手 / 提案）。
- Cobra flag groups： [entireio/cli flaggroups.go](https://github.com/entireio/cli/blob/v0.11.3/cmd/entire/cli/flaggroups.go)（一手）。

## hypothesize：参数形态 + 帮助分组设计（采纳方案 3）

> 用户倾向方案 3（flat 逐名 flag + 通用逃生口 + `-h` 关系小节）。本节把它设计到可实现。

### 0. 先修一个既有缺陷（实测）

`relations-impl` 之后，`write` 只从 `e.rel().Names()`（canonical 名）取 flag，于是：

| 命令 | 现状 |
| --- | --- |
| `kiln do i1 ask --caused-by i2` | 输出 `caused-by: i2` ✓ |
| `kiln do i1 ask --derived-from i2` | **静默丢弃**（无 `caused-by`）✗ |
| `kiln do i1 ask --bogus i2` | 静默丢弃 |

→ 本设计必须同时解决：**alias flag 兼容** + **未知 flag 不再沉默**。

### 1. 参数形态（三层）

1. **flat 逐名 flag（主，词表生成）**——canonical 名 + **别名也接受**（归一）：
   `--causes` / `--caused-by` / `--part-of` / `--has-part` / `--blocks` / `--blocked-by` / `--supersedes` / `--superseded-by` / `--relates-to`；
   别名（仅写侧兼容，不出现在 `-h`）：`--derived-from`→`caused-by`、`--spawns`→`causes`。
   可重复 + 逗号分隔（与 `refs`/`tag` 一致）。
2. **`--unrelate T`**：墓碑。
3. **通用逃生口 `--relate NAME=TARGET`**（可重复；也接受 `NAME:TARGET`，贴 `git --trailer`）：
   - 用途：① 词表外的自定义名；② 脚本 / 一次性。
   - 解析时过 `rel.Resolve(NAME)`；未知名**不沉默**（见 §2）。

### 2. 校验与“不沉默”（与 `silent-flag-drop` 的接口）

- **未知 `--<key>` flag**：现在全静默。设计：`do`/`commit` 对不在 `{固定 flag} ∪ {词表名 ∪ 别名} ∪ {relate, unrelate}` 的 `--key` 给 **⚠ 提示**（可能拼错），但**不拦**（宽容输入）。
- **`--relate` 未知名**：同上 ⚠。
- 具体“报错还是提示”与 `silent-flag-drop` 保持一致（该 issue 正在 `ask`）。

### 3. 帮助分组（`-h`，动态生成）

```
do/commit 的 flags:
  元数据:  --title  --refs  --target  --scope  --domain  --tag
  关系:    --caused-by  --causes  --part-of  --has-part  --blocks  --blocked-by
           --supersedes  --superseded-by  --relates-to
           --unrelate X            撤销与该 issue 的关系
           --relate NAME=TARGET    通用关系（词表外的名走这条）
```

- **关系行由 `rel.Names()` 生成**（按槽 / rank 排序）；**别名不列出**。
- `usageText()` → `usageText(rel *model.RelationSet)`；调用点传 `e.rel()`（无仓库时用默认词表）。
- 可选：再按**槽**分两组（阻塞 / 谱系），贴 Cobra flag groups。

### 4. 实现落点

| 位置 | 改动 |
| --- | --- |
| `cli.go write` | rels 来源 = canonical 名 + 别名（`Resolve` 归一） + `--relate` |
| `cli.go usageFooter` / `usageText` | 关系小节**动态生成**；`usageText` 加 `rel` 参数 |
| `cli.go` 调用点（`-h` / 未知命令） | 传 `e.rel()` 或默认词表 |
| `do`/`commit` 校验 | 未知 flag / 未知名 → ⚠（§2） |

### 5. 取舍

- **flat + 别名都写**：兼容旧脚本（`--derived-from`）；但 `-h` 只列 canonical → **不教别名**（符合 `relation-vocabulary` 的“别名只读”）。
- **`--relate` 用 `=`（也收 `:`）**：贴 git，且 shell 友好（不用引号）。
- 不给 `--relate` 短形式（无必要）。

### 6. 验收

1. `--caused-by X`、`--causes X`、`--part-of X`、`--relates-to X` 可写。
2. **`--derived-from X` → 输出 `caused-by: X`**（alias 兼容）。
3. `--relate part-of=X` → `part-of: X`；`--relate nonsense=X` → ⚠。
4. 未知 `--bogus` → ⚠。
5. `-h` 关系小节从词表生成（改 `relations.names` 会变）。
6. 全在 `do`（不提交）下即可验。

### 7. 非目标

- 不做子命令 `kiln relate ...`（collect 已否）。
- 不改关系机制 / 词表本身。
- 不动 `-h` 其它段落（除非为分组）。

### 结论（设计，待确认）

- 形态：**flat 逐名（词表生成，别名兼容）+ `--relate` 逃生口 + `--unrelate`**。
- 帮助：**动态关系小节**（`usageText(rel)`）。
- 必须一并修：**alias flag 兼容** + **未知 flag ⚠**。

## hypothesize（续）：取消 `--relate` 逃生口 + 核实 `--target`

> 用户指示（2026-10-18）：① 取消逃生口，防止乱改；② 核实 `--target` 现状。

### 1. 取消 `--relate`（逃生口）

- **理由**：逃生口允许写**词表外的关系名**，而这些名没有槽 / 强度定义 → 读侧只能当“未知关系”存着，等于开了个绕过**结构层**的后门。
- **后果**：写侧关系名的**唯一来源**是词表（canonical + 别名）；词表外的名**无法写入**。需要新关系名就改 `relations.names`（配置），而不是临时 flag。
- 前文的 `--relate` 相关项（§1.3 / §2 / §3 / §4 / §5 / §6）**作废**。最终形态：
  1. **flat 逐名 flag**（词表生成；接受别名，归一）；
  2. **`--unrelate T`**。
- **未知 `--<key>` flag → ⚠** 仍保留（防拼错）；原“`--relate` 未知名 ⚠”项删去。

### 2. `--target` 核实（现状）

- **语义**：`target:` = “本动作**涉及的文件**”，**不是关系**（README 例：`--target internal/session/lock.go`）。
- **形态：标量**（`f.get("target")` + `writeScalar`）。实测：
  - `--target a.go,b.go,c.go` → `target: a.go,b.go,c.go`（**整串，不拆**）
  - 对照 `--refs a,b,c` → `refs: a, b, c`（拆）
- **校验：无**。`check` 不核对；渲染也不出错。
- **已在跟的 issue**：
  - `target-honesty`（ask）：用 diff 反查 `target:` 是否属实；它 的 Q5 正是“单值还是列表”。
  - `refs-flag-splitting`：flag 形态（标量 vs 列表）——`--target` 是该族一员。
  - `agent-ux-review`：已点名“`--target` 标量 vs `--refs`/`--tag` 列表却在同一行，flag 形状不自证”。
- **对本 issue 的影响**：
  - `--target` 仍归**元数据**组（不是关系）。
  - 本 issue 可顺手在帮助里**标注元数**（`--target FILE`（单个）vs `--refs LIST`）——这是“帮助形状”的事；但“要不要让 `target` 变列表”归 `refs-flag-splitting` / `target-honesty`，**本 issue 不动**。

### 3. 更新后的最终形态

```
do/commit 的 flags:
  元数据:  --title <文本>  --refs <列表>  --target <单个文件>  --scope <文本>  --domain <文本>  --tag <列表>
  关系:    --caused-by <issue>  --causes <issue>  ...（词表生成，按槽排序）
           --unrelate <issue>          撤销与该 issue 的关系
```

（`--relate` 已删；元数据的“单个 / 列表”标注待定，可与 `agent-ux-review` 对齐。）

### 4. 验收（更新）

1. `--caused-by X` / `--part-of X` / `--relates-to X` 可写。
2. **`--derived-from X` → `caused-by: X`**（alias 兼容）。
3. **未知 `--bogus` → ⚠**。
4. **词表外的关系名无法写**（无 `--relate`）。
5. `-h` 关系小节从词表生成。
6. `--target` 行为**不变**（仍标量、不拆、不校验）。

## hypothesize（再续）：帮助里写清“值形状”——列表要说明接受什么

> 用户（2026-10-18）：接受列表的帮助信息，是不是详细写清接受什么列表比较好？

### 1. 同意，且现状更糟

- 现状 `-h` 的 `do/commit 的 flags` **只列 flag 名，完全没有参数形状**：看不出 `--title` 要一个值、`--refs` 要几个、逗号拆不拆。
- `agent-ux-review` 已点名同族问题（“flag 形状不自证：可选 / 必填 / 可变”），并拿 `--target` 标量 vs `--refs` 列表排同一行当实例。
- 而代码里的真实规则是：`list()` = **逗号分隔 + 可重复**；`get()` = **标量取最后一个**。这些差异现在只存于源码。

### 2. 提议：三件事一起写

1. **值类型**（占位符）：`<文本>` / `<文件>` / `<标签>` / `<条目>` / `<issue>` / `<名字>`。
2. **列表 vs 单个**：列表写 `<项>[,<项>]`；单个写 `<项>`。
3. **分隔与重复**：一句图例“`[,<…>]` = 可重复、逗号分隔”。

### 3. 应用后的样子（草案）

```
do/commit 的 flags:
  元数据:
    --title <文本>
    --refs <条目>[,<条目>]     引用
    --target <文件>            单个文件
    --scope <文本>
    --domain <名字>
    --tag <标签>[,<标签>]      标签
  关系:                          （均可重复、逗号分隔）
    --caused-by <issue>        本条 issue 由某 issue 引发
    ...
    --unrelate <issue>         撤销与该 issue 的关系

  说明: < > 是占位；[,<…>] 表示可重复、逗号分隔。
```

### 4. 边界

- `--target` 仍是**单个**（是否变列表归 `refs-flag-splitting` / `target-honesty`）——本 issue 只**如实标注**。
- “形状自证”的完整方案（必填 / 可选 / 可变）归 `agent-ux-review`；本 issue 只覆盖 `do/commit` 的 flags 小节。
- 关系名的**释义**（如“本条由某 issue 引发”）可选：若 `relations.names` 带 `gloss`，`-h` 可一并渲染；否则只给 `<issue>` 占位。

### 5. 取舍

- 详细标注会让 `-h` 变长；但 `do`/`commit` 是最常用的命令，**值形状是高频踩坑点**（`agent-ux-review` 实证：把 9 个文件名传给 `--target` 毫无提示）。
- 折中：**元数据给紧凑的 shape 标注 + 一行图例**；关系小节**同名同形**（均 `<issue>[,<issue>]`），只在**小节头**写一次“可重复、逗号分隔”。

### 6. 验收（补）

7. `-h` 里每个 flag 带**占位符**；列表型带 `[,<…>]`；有一行图例说明。

## hypothesize（三续）：help 形态——分命令帮助 + `help` 命令 + `--json`

> 用户（2026-10-18）：考虑优化 help 形态，如 `kiln do -h` / `kiln do help`，或 `--json` 格式的 help。

### 0. 现状

| 入口 | 行为 |
| --- | --- |
| `kiln -h` / `--help` / `help` | 全局 usage → **stdout, exit 0** |
| `kiln`（无参） | 全局 usage → **stderr, exit 2** |
| **`kiln do -h`** | **不是帮助**——落到 `用法: kiln do <issue> <action> [flags]`（stderr, exit 2） |
| `kiln help <cmd>` | **不存在** |
| `--json` help | **不存在** |

- 帮助有**三处来源**：`usageHeader` / `usageCommands` / `usageFooter`（全局） + 各命令的 `用法:` 串（局部） → **会漂移**（`usageFooter` 与生成的关系 flag 已经是一次实例）。

### 1. 提议

1. **分命令帮助**：`kiln <cmd> -h` / `--help` 在**位置参数之前拦截** → 该命令的帮助。
2. **`kiln help [cmd]`**：`help` 无参 = 全局；`help do` = 分命令。
   - **`kiln do help` 不采纳**：`help` 会被当成 issue 名（`do` 要 `<issue> <action>`）；`kiln do help` 不应有第二种含义。
3. **`--json` help**：`kiln help --json` / `kiln <cmd> -h --json` → **一个 JSON 文档**（与 `status`/`list`/`check` 的 `--json` 契约一致）。

### 2. 单一真源（关铪）

- 把命令与 flag 建成**结构化表**：命令（名/用法）、flag（名/值类型/是否列表/可重复/所属组/释义/是否关系）。
- **文本帮助与 JSON 帮助都从它渲染**；关系 flag 由 `RelationSet` 生成（承接本 issue 已有结论）。
- → 消掉“三处来源”的漂移（`usageFooter` 与生成 flag 不一致就是当前的病）。

### 3. JSON 形状（草案）

```json
{
  "commands": [
    {"name": "do", "usage": "kiln do <issue> <action> [flags]",
     "flags": [{"name": "title", "value": "text"},
               {"name": "refs", "value": "list", "list": true, "repeatable": true},
               {"name": "caused-by", "value": "issue", "list": true, "relation": true}]}
  ],
  "relations": {
    "slots": [{"name": "ordering", "blocking": true}],
    "names": [{"name": "caused-by", "slot": "lineage", "rank": 2, "directed": true, "inverse": "causes"}],
    "aliases": {"derived-from": "caused-by"}
  },
  "actions": {"names": ["ask"], "start": "ask", "terminal": ["conclude", "apply"]}
}
```

### 4. 触发与退出码

| 入口 | 输出 | 退出 |
| --- | --- | --- |
| `kiln <cmd> -h` / `kiln help <cmd>` | stdout | 0 |
| `kiln help` / `-h` / `--help` | stdout | 0 |
| `kiln`（无参） | stderr | 2 |
| 加 `--json` | stdout **只放一个 JSON 文档** | 0 |

### 5. 边界（建议）

- 分命令 / JSON help 是**通用 help** 的事，不止关系 flag；与 `help-alignment` / `help-self-contained` 相邻。
- **建议**：本 issue 只定 **`do`/`commit` 的小节内容与值形状**；**分命令 / JSON help 的框架**另开一个 issue（或并入 `help-alignment`），以免本 issue 膨胀。
- 不实现 shell 补全（可后续）。

### 6. 取舍

- 分命令 + JSON help 正是 agent 想要的（可自省、无散文）；代价是要把帮助重构成**单一真源**。
- `kiln do help` 不采纳（与 issue 名冲突）。

## hypothesize（四续）：`--json` help 暂缓

> 用户（2026-10-18）：`--json` help 先不做，再想。

- **处置**：三续里的**第 3 条（`--json` help）标为暂缓、不在本轮**；其余三条（分命令帮助 `kiln <cmd> -h`、`kiln help [cmd]`、单一真源）保留。
- **理由 / 影响**：`--json` help 要与 `status`/`list`/`check` 的 `--json` 契约对齐，并且会**把“命令 / flag 结构化表”的 schema 过早冻结**；先不定，保留重构余地。
- **仍待定**：分命令 / 单一真源的**框架**是否另开一个 issue（还是并入 `help-alignment`）。

## apply：落地与验证（2026-10-18）

### 0. 先确认：本 issue 不依赖分命令帮助

- 各命令的 flag 小节**本来就在全局 `-h`**（`usageFooter`）；→ 改 `do/commit` 小节**不需要** `kiln do -h`。
- 唯一小耦合：`usageText` 需要词表→ 已把 `config.Read` 提到 help 分支**之前**，且**容错**（坏配置不让 `-h` 失败，回退默认词表）。

### 1. 改了什么

| 文件 | 改动 |
| --- | --- |
| `model/relation.go` | 新增 `RelationSet.Aliases()`（排序） |
| `cli.go` 帮助 | `usageText(rel)`；`usageFooter` 拆成 `usagePrologue` + `usageOtherFlags`；`do/commit 的 flags` 改为**分组 + 从词表生成 + 值形状**（`flagHelp` / `doCommitFlagGroups`）；行尾去空 |
| `cli.go` `Run` | 先读配置（容错），help/未知命令都用 `rel` |
| `cli.go` `write` | 新增 `relationsFromFlags`：canonical 名 + **别名（归一）**；未知 `--key` → **⚠** |
| 测试 | `usageText(model.DefaultRelationSet())` 调用点同步 |

### 2. 验收（实测）

| # | 项 | 结果 |
| --- | --- | --- |
| 1 | `--causes` / `--part-of` / `--relates-to` 可写 | ✓ |
| 2 | **`--derived-from X` → `caused-by: X`**（alias 兼容） | ✓ |
| 3 | 未知 `--bogus` → ⚠ | ✓ |
| 4 | `--relate`（已删）→ ⚠ | ✓ |
| 5 | `-h` 关系小节从词表生成、带值形状 | ✓ |
| 6 | `--target` 行为不变 | ✓ |

- `go build ./...` / `go test ./...` 全过。

### 3. 未做（框架期）

- **分命令帮助** `kiln <cmd> -h` / `kiln help [cmd]`；
- **`--json` help**（暂缓）；
- **单一真源**（命令 / flag 结构化表，文本与 JSON 同源）。

→ 建议另开一个 issue（或并入 `help-alignment`）承接这三项。
