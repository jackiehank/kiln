# issue: machine-readable-output — 给 `status` 等读命令加 `--json` / `--toon` 结构化输出

- **domain**：cli
- **关联**：derived-from `cli-necessity`

## 背景

kiln 的**读命令**（`status` / `list` / `path` / `check` / `graph` / `show` / `worktree list`）目前**只有人读文本一种形态**：

- `status`：按 domain 分组、`✓ ⚠ ■` 记号、全角对齐的目录视图；
- `list`：等宽列（issue / action / domain=…）；
- `path` / `show`：缩进的编号步骤 + 关系摘要 + 委托 git 的 stat 块；
- `check`：`✓ / ⚠ / ✗` 三档 + 人类措辞的判据；
- `graph`：`ascii` 树（唯一格式）。

agent / 脚本要拿数据，只能**解析这些为"给人看"而排版的文本**——README §与 agent 协作 把这写成优点（"稳定文本，agent 不必解析自由格式"），但"稳定"靠的是**约定**，不是**格式**：字宽、符号、缩进一变，解析器就碎。

这一条**已经被点名过**。`cli-necessity` 对 jj 的调研里三处指出同一件事：

- §"四条命令层面的观察" 第 4 条：jj 有 `-T` 模板，kiln MVP 无 `--json`，对 agent / 脚本不友好；
- §"面向 agent 的 UX"：短板是"**输出偏人读、无 `--json`**"；
- §"对 cli-necessity 的启示" 第 5 条：**"若 agent 是主要用户，这比'命令重叠'更该先补"**。

`docs/mvp-plan.md` 一侧的现状：§6 把 `--format=json` 明确**排除出 MVP**；§10 只留了一条 `kiln check --format=json`（CI 集成与 PR 评论）——覆盖**一个**命令面，且没有回答"用哪种格式"。

## 要回答的问题

1. **要不要做？** 现有文本已"够稳定"，补机器可读输出的边际价值？先例是 git 的 `--porcelain`、jj 的 `-T`：**为机器单开一条输出契约**，与人读渲染解耦。
2. **覆盖哪些命令？** 只 `check`（对齐 §10 既有条目）？还是所有读命令一起上？`do` 的输出**不能**碰（它必须可直接喂 `git commit -m`）。
3. **`--json` 还是 `--toon`？** 这是本 issue 的核心分岔：
   - **JSON**：模型先验最熟、stdlib 可编解码（本仓 **stdlib-only**，零成本）、生态通用（CI / jq / schema 校验）；
   - **TOON**（Token-Oriented Object Notation）：**省 token**、面向 LLM 的新格式——但 agent **先验弱**、要**自写**编码器、无生态、非标准。
   两条判据要摆明：**省 token 的收益 vs 模型不熟 / 自造轮子的成本**；并对照 mvp-plan §1「**LLM 先验优先**：只组合模型最熟的原语，不为优雅引入长尾 DSL」。
4. **单格式还是多格式？** 两个独立 flag（`--json` + `--toon`）／统一 `--format=json|toon|text`（对齐 `graph` 已有的 `--format=`）？后者是一族枚举，前者是布尔，**默认值**各是什么。
5. **人读文本还留吗？** 默认保持文本（人 + agent 共用一条路径），还是要求机器消费方显式 opt-in？（现状 stderr 提示已保证 **stdout 纯净可 pipe**，这条要不要写成契约。）
6. **结构从哪来？** 输出是"领域模型的直接序列化"（`[]Issue` / `[]Action`）还是"渲染层再排一遍"？要不要一份**稳定的 schema 文档** / 版本字段？字段名是否直接复用 trailer 名（`issue` / `action` / `derived-from`）以少一次翻译。
7. **`check` 的退出码与结构化输出的关系**：现在"只有 `✗` 让 `check` 非零退出"。json 模式下，判据（哪条规则、哪个 issue、error/warn）要不要逐条结构化给出——这是 CI 集成（§10 原条目）真正需要的东西。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 不做 | 维持"稳定文本" | agent 继续自写解析器；`cli-necessity` 点名的短板不补 |
| **B** 只补 `check` | `kiln check --format=json`（= §10 现成条目） | 面上最小；`status` / `path` / `list` 仍只能解析文本 |
| **C** 全读命令 `--json` | 各命令加 `--json` | 改动面最大；每个命令都要定 schema |
| **D** `--json` + `--toon` 双格式 | 每命令两个 flag | **格式 × 命令**组合翻倍；toon 要自写编码器、agent 先验弱 |
| **E** 统一 `--format=json\|toon\|text` | 一个 flag 家族 | 要先定枚举、默认值与未知值的报错口径 |

> 倾向（**待验证**）：**先落 C 的一个子集**——优先 `status` / `list` / `check`（结构最规整、agent 最常问），格式先做 **JSON**（零依赖、先验最熟），**TOON 作为待评估的第二格式**，不当第一枪。`path` / `show` 含委托 git 的原文块，序列化口径需单独设计。

## 非目标

- **不改默认输出**：人读文本保留，机器可读是**旁路**而非替换。
- **不引第三方依赖**：本仓 `go.mod` 无 `require`；选 TOON 就意味着**自写编码器**，成本要算进方案 D/E 里。
- **不碰 `do`**：它的 stdout 必须能直接 `$(kiln do …)` 进 `git commit -m`。
- **不做 MCP server**：那是 §10 的另一条（"把结构化工具暴露给 Agent"），与本 issue 相邻但不同——本 issue 只在 **CLI 的输出侧**。
- 不改 stderr 提示策略（提示只进 stderr、`exit code` 不变），但 json 模式下 stdout 必须仍是**单一可解析文档**。

## 备注

- 来源：用户在会话中直接提出——"考虑要不要把 status 等输出信息的命令加上 `--json` 输出或者 `--toon` 输出来优化 agent 交互"。
- 由 `cli-necessity` 派生：其 §启示 5 已把"结构化输出"列为 agent 场景下**优先级高于"命令重叠"**的短板；本 issue 是那一条的**落地评估**。
- 与既有条目合并：mvp-plan §10 的 `kiln check --format=json`（CI 集成与 PR 评论）并入本问——命令面从一个扩到一族。
- 若落地，README §与 agent 协作 里"结构化输出……agent 不必解析自由格式"那句要重写：**"稳定文本"与"机器可读格式"是两回事**，不能继续拿前者当后者卖。

## hypothesize：`status` / `list` / `check` 的 `--json` schema

> 结论先行：本轮只做**三个命令 + 一种格式**——`--json` 布尔 flag，`status` / `list` / `check` 各返回**一个顶层对象**（绝不放裸数组）。
> 三条命令的 schema 都由**同一套约定**生成：**key 复用 trailer 词**、**枚举用字符串**、**缺省即省略**、**机器视图不出 prose**、**排序与文本视图同源**。
> **JSON 是渲染层的事**：DTO 写在 `internal/cli`，不反射 `model.Issue`；唯一要动 `engine` 的是给 `Diagnostic` 补一个 **rule 码**（现在只有中文 prose）。
> 落点仅此三命令；`path` / `show` / `worktree list` 与 `--toon` 明确**留到下一轮**。

### 1. flag 形态：只加 `--json`

- **只加 `--json`**（布尔，`f.has("json")`），**不加** `--format=json|toon|text`：一个格式时，枚举只是把"有没有"包装成"选哪个"，是**为尚不存在的第二种格式付的复杂度**。
- 但**语义上记一笔**：`--json` ≡ `--format=json`。将来真上 `--toon` 时再加 `--format=`，`--json` **保留为别名**——已写下的调用不被破坏。
- 默认（不加 `--json`）**输出完全不变**：人读文本是默认视图，机器可读是**旁路**。

### 2. `kiln status --json`

> **修订（本轮）**：本节的 `groups[] → entries[] → children[]` 三层嵌套**已被用户反馈推翻**——改为顶层扁平 `issues[]` + 可选 `parent`，见下方「hypothesize（续）」。下面的示例与字段表**原样保留作过程记录**；现行 schema 以（续）为准。

```json
{
  "summary": { "total": 29, "complete": 21, "in-progress": 8, "actions": 107, "domains": 2 },
  "worktree": { "mode": "off", "open": 0 },
  "groups": [
    {
      "domain": "cli",
      "entries": [
        {
          "id": "action-diff",
          "domain": "cli",
          "action": "conclude",
          "done": true,
          "has-worktree": false,
          "children": [
            { "id": "target-honesty", "domain": "design", "action": "ask", "done": false, "has-worktree": false }
          ]
        }
      ]
    }
  ]
}
```

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| `summary.total` / `complete` / `in-progress` / `actions` | `engine.Summarize` | 对应文本头两行的四个计数 |
| `summary.domains` | `engine.CountDomains` | **不含**未分类块，与文本"domain 2 个"一致 |
| `worktree.mode` | 配置 + `worktreeSegment` | 报**生效值** `on` / `off`（`""` 即 `on`），不报"未设" |
| `worktree.open` | `areasOpen` | 专用区数量，**main 不计**（与文本"已开 N 个"一致） |
| `worktree.current` | `vcs.AreaAt(areas, ".")` | 当前站在哪个区；不在区内**省略** |
| `groups[].domain` | `engine.GroupByDomain` | **省略 = 未分类块**——`(未分类)` 是人读标记，不进 schema |
| `groups[].entries[]` → `children[]` | `engine.Entry` | **递归**嵌套；用**数组嵌套**表达深度，**不序列化** `├──` 字形 |
| `entries[].id` / `domain` | `model.Issue` | issue 行的**域内全名**由 `domain` + `id` 拼出，不存 `cli/action-diff` 这种合体串 |
| `entries[].action` / `done` | `p.State()` + `IsTerminal` | `action` = **最后一个动作的类型**；`done` 是视图**自己的判定**（终点集可配），消费方不必懂词表 |
| `entries[].has-worktree` | `▲■` 那一列 | 谓词式命名，专指"这个 issue 有开着的区" |

要点：

- **文本里的 `state` 列在 JSON 里叫 `action`**：同一个值，就是"最后一个动作的类型"。文本的 `✓`/`未知` 是**渲染态**，不进 schema（`未知` 实际也到不了——issue 必然由动作产生）。
- **`status` 只给行，不给动作明细**：动作清单是 `path` 的活。这样 `status --json` 的大小是 O(issue)，不是 O(commit)——**索引与明细分层**正是 `path`/`show` 已定的那条分工。
- **嵌套保留**：跨域派生被嵌到父下，是 `status` 唯一视图的**信息**，不是排版。平铺 + `parent` 字段能表达同样的边，但要消费方再算一次"谁是本块的顶层项"——留给下一轮评估（见 §7）。

### 3. `kiln list --json`

```json
{
  "filter": { "search": "worktree" },
  "count": 12,
  "issues": [
    { "id": "worktree-default", "domain": "cli", "action": "apply", "done": true, "rank": 0, "title": "落地：worktree 默认翻为 off——默认值与零值语义一起翻面，播种模板 / -h footer / README 同步，5 个护栏用例改为显式 on", "commit": "3f1597873d1bfd5541941fd24de2413436dec089" },
    { "id": "perf-audit", "domain": "cli", "action": "apply", "done": true, "rank": 3, "title": "apply：--only 只校验一次——CheckedAction 免掉 Action 的二次 status，3 路径 530→372ms（10→7 进程）；输出与拒绝语义不变", "commit": "73a780c4626700f100b322a22af02b386f504667", "hit": { "ordinal": 5, "action": "apply", "commit": "e80c092d43603c17b4e08cd52553c8deffb00c79", "subject": "apply：status/commit 固定开销——status 154→106ms（少一次 worktree list）、commit 空动作 501→337ms（少 toplevel 与重复 status）；顺带修 0 area 图例", "total": 9, "date": "2026-10-27T22:43:24+08:00" } }
  ]
}
```

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| `filter.*` | 命令行 `--domain/--tag/--action/--status/--search` | **原样回显**已给的条件（给 CI 日志定位用）；未给则**省略该键**——省略即"没按这条过滤" |
| `count` | `len(ids)` | 命中数；与 `issues` 长度恒等，但省得消费方自己数 |
| `issues[].id` / `domain` / `action` / `done` | 同 §2 的 issue 行 | **与 `status` 共用同一个行 schema**（`children` / `has-worktree` 除外） |
| `issues[].title` | 该 issue **最后一个动作**的 `title`；缺则回退该动作的提交首行 | 空则省略；筛出来的行要能回答"是什么事"（`issue-list-scaling` 的 H4，推翻了 `json-fields` 当时"不给 list 加 title"的非目标） |
| `issues[].commit` | **最后一个动作**的完整 40 位 hash | 行的 `action` / `done` / `title` 都取这条动作；`id` 仍是 issue id，两者不混（`json-commit-hash`） |
| `issues[].rank` | 搜索分桶 | **只在 `--search` 时出现**：`0` = id 以查询首词开头（其余词可在标题/动作），`1` = id 含全词，`2` = 当前标题含全词，`3` = 一条动作 subject 含全词，`4` = 词分散；`issues[]` 按 rank 排序。不带 `--search` 时该键省略，行仍按 id 升序 |
| `issues[].hit` | 命中动作的地址 | **只在 id + 标题解释不了命中时出现**：`{ordinal, action, commit, subject, total, date}`；`ordinal` 可直接喂 `kiln show`，`commit` 给 git，`total` 是该 issue 的动作总数（`ordinal` 的上界），`date` 是命中动作的 author date（RFC3339，与 `path.actions[].date` 同源）；词分散在多条动作、没有单条全含时省略 |
| `issues[].tags` | `model.Issue.Tags` | 空则省略（文本只在非空时打 `tags=`） |

要点：

- **`--status=open|done`**：`done` = 最后一个动作属于 `actions.terminal`（默认 `conclude` / `apply`），与行里的 `done` **同一谓词**；`open` 是它的补集。非法值 exit 2，不静默回全量。**`done` 不是"永久关闭"**——终点动作之后仍可继续，再提交一个非终点动作就会回到 `open`（见 README「动作词表」与 `-h` 的 do/commit 说明）。
- **`--search <query>`**：空白分词、**词间 AND**、**字面量**、**大小写不敏感**；每个词都要出现在 `id + 当前标题 + 所有动作 subject` 里（可分散在不同字段）。**不搜正文**（提交正文只有 trailer，没有散文）。空串 / 纯空白 query exit 2；**无命中是正常结果**：`count: 0`、`issues: []`、exit 0，并在 **stderr** 给一行 `没有匹配 "<query>" 的问题`（stdout 仍只有一个文档；措辞在 `cli-consistency` 批 C 从“issue”改为“问题”）。
- **`--search` 的排序**：按 `rank` 升序（0–4 见上表）；同 rank 按末条动作日期**降序**（按时间比较，不按字符串），日期相同或缺失时按 id 升序。文本视图与 `issues[]` 同序；不带 `--search` 的 `list` 仍按 id 升序——**只有搜索会改变顺序**。
- **`id` 与 `commit` 的分工（全局）**：`id` 只指 issue，`commit` 只指提交 hash（完整 40 位）——`path.actions[]` / `show` / `list` 行都叫 `commit`。hash 是**取数时的快照地址**，rebase / amend 会变，不是主键；稳定寻址用 issue id + 动作序号。
- **`list` 不报 `worktree`**：`■` 与 worktree 段**只在 `status` 出现**，这是 `worktree-hint` 已定的"读命令不加 worktree 噪声"；为它给 `list` 多起一次后端查询不值得。
- **行 schema 与 `status` 的重叠部分是契约**（`id` / `domain` / `action` / `done` / `title`）：同一套字段名，消费方一份解码器覆盖两个命令。

### 4. `kiln check --json`

```json
{
  "ok": true,
  "summary": { "ok": 21, "warn": 7, "error": 0 },
  "results": [
    { "issue": "action-granularity", "level": "warn", "rule": "terminal-action", "action": "ask" },
    { "issue": "action-config", "level": "ok", "rule": "complete", "actions": ["ask", "apply"] }
  ]
}
```

关系靶不存在时（示例，warn）：`{ "issue": "broken-ref", "level": "warn", "rule": "relation-target", "ref": "missing-parent" }`；trailer 里出现不认识的键时：`{ "issue": "a", "level": "warn", "rule": "unknown-trailer", "action": "ask", "ref": "foo" }`

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| `ok` | 无 `level == "error"` | **与退出码同义**：`ok=false` ⟺ exit 1。CI 判据一眼可见 |
| `summary.ok/warn/error` | `engine.Check` 结果计数 | **计的是"条目数"不是"issue 数"**（一个 issue 可同时缺起点与父） |
| `results[].level` | `engine.Level` | **字符串枚举** `ok` / `warn` / `error`——不吐内部的 0/1/2 序号 |
| `results[].rule` | **新增**（见 §6） | 机器码：`start-action` / `terminal-action` / `relation-target` / `unknown-trailer` / `complete`（原 `derived-from` / `spawns` 已并入 `relation-target`，见文末结论） |
| `results[].action` | `p.Last()` | 仅 `terminal-action` / `unknown-trailer` 用：前者是停在哪个动作，后者是坏键落在哪个动作 |
| `results[].ref` | 关系目标 / 键名 | `relation-target` 用：靶 issue；`unknown-trailer` 用：不认识的键名 |
| `results[].actions` | `kinds(p)` | 路径里全部动作类型，**按序**；每一条结果都带 |
| 标题 `actions` | | 恒在，**与 `--verbose` 无关**（见 §5 第 6 条） |

要点：

- **不给 `message`**：`check` 的中文判据（"缺少 ask：问题没有起点动作"）是**文本视图的文案**。机器视图给 `rule` + `ref` / `action`，消费方按码分支，**不解析中文**——这正是把它做成 JSON 的全部理由。
- **关系码是泛化的**：任一写侧关系名（`causes` / `part-of` / …，见 `relations-impl` 的词表）的靶不存在，都报 `relation-target`（warn），**不按关系名分码**；坏 trailer 键报 `unknown-trailer`。
- **空仓**：文本打一句提示并 exit 0；JSON 给 `{ "ok": true, "summary": 全零, "results": [] }`，**不出提示句**（prose 不进 schema）。
- **stdlib 编码**：`encoding/json` 默认转义 `<` `>` `&`；issue ID 与关系名都是 kebab-slug，不受影响，不必设 `SetEscapeHTML(false)`。

### 5. 统一约定（三个命令共用，也是将来扩命令的模板）

1. **顶层永远是对象，不放裸数组**——将来加字段（计数、过滤器、schema 版本）不破坏已有消费方；裸数组一加字段就得改类型。
2. **key 复用 trailer / 已有词**：`domain` / `tags` / `derived-from` / `spawns` / `action`；多词坚持 **kebab-case**（与 trailer 逐字同形），**一个概念一个词**，不给机器再学一套拼写。
3. **机器视图不出 prose**：JSON 里没有一句"给人读的话"——`提示:` / legend / `(未分类)` / `未知` / 中文判据一律不进。要 prose 就用默认文本视图。
4. **缺省即省略**（`omitempty`），**不用 `null` 占位**：省略 = "没有"。规则唯一且好记，也省得消费方判两种空。**修订：布尔除外**——`done` / `has-worktree` **恒显**，`false` 是**判定结果**而不是"没有"（见 hypothesize（续）§5）。
5. **枚举用字符串**：`"ok" / "warn" / "error"`、`"on" / "off"`。数字序号是内部实现（`Level` 的 iota），不该成为契约。
6. **schema 与 flag 无关**：同一命令的 JSON 形状**不随 flag 变**——`check --json --verbose` 与 `check --json` 字段集相同。否则消费方要为每个 flag 组合写一套解码器，flag 越多越碎。
7. **排序与文本视图同源**：domain 按名、未分类最后、issue 按 ID、诊断按 issue 顺序。→ 同一仓库状态**逐字节可复现**，可做 golden 测试，也可 diff。
8. **stdout 只放一个 JSON 文档**：提示 / legend / 报错全部留在 **stderr**（现状已如此，本轮把它**写成契约**）；退出码不变（`status`/`list` 0；`check` 0/1 且与 `ok` 同义）。
9. **缩进 2 空格**，不做紧凑变体：JSON 选它图的是**先验与生态**，省 token 是 TOON 的活，别在这里混。

### 6. 落点与测试

- **`internal/cli`**：新增三个 DTO（带 `json` tag）+ 三个命令的 `--json` 分支；解析复用现有 `parse`。**不反射 `model.Issue`**——领域类型不带 json tag，输出契约不该长在领域模型上；DTO 的另一半好处是"**默认只出想要的字段**"（`status` 不要 `Actions` 明细，靠 DTO 天然拿不到）。
- **`internal/engine`（唯一的改动）**：[Diagnostic](file:///c:/Users/jacki/kiln/internal/engine/engine.go#L132-L136) 现在只有 `Level + Issue + Message`（中文 prose）。JSON 要 `rule` 码与结构化字段 → 补 `Rule` / `Ref` / `Action`。**`Message` 原样保留**给文本视图，`check` 的文本输出与退出码**一字不变**。
- **测试**：三个命令各一组 golden（用 [engine_test](file:///c:/Users/jacki/kiln/internal/engine/engine_test.go) 那类假提交集构造 repo，比对 JSON 字节）；再在真仓跑一遍与文本视图对齐。

### 7. 与候选表的映射 / 遗留

- **落 C 的一个子集**（`status` / `list` / `check`）+ **只用 JSON**；**B** 被 C 覆盖（`check` 在其中）；**D**（双格式）与 **E**（`--format=`）**推迟到真有第二种格式时**再评估。
- 遗留（明确不在本轮）：
  - `path` / `show` / `worktree list` 的 JSON——`path`/`show` 含**委托 git 的原文块**（patch / stat），口径要单设计（贴原文？还是给文件清单？）；
  - `--toon`；
  - 紧凑输出 / `--compact`；
  - **错误路径的结构化**：`打开仓库失败` 这类仍走 stderr prose，本轮不动（要动则全命令一起改）；
  - ~~`status` 嵌套是否改为**平铺 + `parent`**~~ ——**已销**：用户反馈直接定案平铺，见 hypothesize（续）；
  - `check` 结果里是否带**提交 id**（现只给动作类型；要追溯交给 `path`）。

## hypothesize（续）：按用户反馈定稿平铺——`status` 去掉三层嵌套

> 用户反馈：**"status --json 的输出嵌套太多，能不能简单点"**。
> 结论：**顶层直接给一个扁平 `issues[]`**，跨域落位只留一个可选字段 `parent`；§2 的 `groups[] → entries[] → children[]` **整体撤掉**。
> 这一改**预先回答了 §7 那条遗留**（"嵌套是否改为平铺 + `parent`"）——不是留待将来，而是现在就定案。

### 1. 新形状

```json
{
  "summary": { "total": 30, "complete": 21, "in-progress": 9, "actions": 109, "domains": 2 },
  "worktree": { "mode": "off", "open": 0 },
  "issues": [
    { "id": "action-diff", "domain": "cli", "action": "conclude", "done": true, "has-worktree": false },
    { "id": "target-honesty", "domain": "design", "action": "ask", "done": false, "has-worktree": false, "parent": "action-diff" }
  ]
}
```

**三层 → 一层**：`summary` 与 `worktree` **原样保留**（它们本来就只有一层，不是抱怨点）；递归的 `groups[].entries[].children[]` 换成平铺的 `issues[]`。

| 字段 | 说明 |
| --- | --- |
| `issues[].id` / `domain` / `action` / `done` | 同 §2，职责不变（`done` **恒显**） |
| `issues[].has-worktree` | 同 §2（`status` 独有） |

**修订（issue `qualified-issue-id`）**：`parent` 已**退场**。它是从“跨域嵌套”里长出来的字段——文本视图不再落位（见下 §2），它就没有可指的东西了。

顺带少了一条判定：原来 `groups[].domain` 省略还要单独解释成"未分类块"，现在只剩行级一条——**`domain` 省略 = 没声明 domain**。

### 2. ~~`parent` 是什么~~——已退役

它原本是“**落位**词，不是关系词”：值由 `GroupByDomain` 的跨域落位给出，量的是**视图怎么摆**，不是 `derived-from` 本身（那条边还可能来自父上的 `spawns`）。

**现已整体取消**（issue `qualified-issue-id`）：`status` 不再跨域嵌套、不再画树、每行也不再带 domain 前缀——**块归属 = issue 自己的 `domain`**，一个 issue 只出现一次。关系一律留给 `graph` / `path`。这也顺手答了 `cli/relation-rendering` 的“`status` 要不要显示关系”：**不显示**。

对 §5 第 2 条的影响：`parent` 退出后，视图自造词只剩 `has-worktree`，而它是“**就是那个东西**”（那个 issue 有开着的区），与 `domain` / `tags` 同类——“视图自己的概念另立其词”这条分叉不再有实例。

### 3. 原来的反对理由为什么不成立了

§2 记下过："平铺 + `parent` 能表达同样的边，但要消费方再算一次'谁是本块的顶层项'"。回头看，这条**反了**：

- **嵌套里的"顶层项"是隐式的**，且消费方仍要按 `children` **递归**才能枚举全部 issue——并**没有更省**；
- 平铺里"谁是顶层项" ⟺ **`parent` 缺失**：判一个字段的空，比递归更省。

### 4. 行序：与文本视图**同源**

`issues[]` 的次序 = 文本视图的 **issue 行**序（domain 块按名升序、未分类最后；块内按 ID 升序）。文本视图多一层**块头行**，JSON 不多——两者**相对顺序**一致即可；否则同一份信息两个视图各排各的，golden 测试就得写两套。

### 5. 对 §5 的修订

九条约定**照旧**，只有第 4 条补一个例外（已就地注在 §5）：**布尔恒显**——`done` / `has-worktree` 的 `false` **是判定结果**，不是"没有"；省掉它反而要消费方把"缺席"当 `false`，而这正是 `done` 存在的理由（省得消费方去懂可配的终点词表）。

### 6. 连带

- **§7 的遗留"嵌套 vs 平铺"已销**（已就地标注）。
- 这条取舍对 `path` / `show` 同样成立：**数据层一律平铺，视图（缩进 / 树线 / 分组标题）归视图**——将来给它们加 `--json` 时不必再议同一个问题。
- `status` 与 `list` 的行 schema 从“重叠”变成**同一份契约**：共享 `id` / `domain` / `action` / `done` / `title`（`title` 于 `issue-list-scaling` 补进 `list`）；`status` 多 `has-worktree`，`list` 多 `commit` / `hit` / `tags`。一份解码器覆盖两个命令。两个命令的**排序**不同是刻意的（`status` 走 domain 块，`list` 默认按 ID、`--search` 时按 rank），所以共享的是**行本身**，测试按 ID 取行比对。

## test：落地三个命令的 `--json`（含真仓端到端）

> 按（续）定稿的 schema 落地：`engine` 只补给 `Diagnostic` 一个 rule 码，`cli` 加三组 DTO 与三个 `--json` 分支。
> 默认文本视图**逐字节未变**——用改动前后的两个二进制对拍过（见验证表第 2 行）。

### 落地内容

| 文件 | 改动 |
| --- | --- |
| [internal/engine](file:///c:/Users/jacki/kiln/internal/engine/engine.go#L131-L156) | `Diagnostic` 增 `Rule` / `Action` / `Ref`，新增 5 个 rule 码常量；`Message` 一字未动 |
| [internal/cli](file:///c:/Users/jacki/kiln/internal/cli/cli.go#L832-L952) | `status` 的遍历重构成**一次走、两种渲染**（`statusRow` + `statusWalk`）；`worktreeWord` 抽出，文本段与 JSON 共用同一处判定 |
| [internal/cli](file:///c:/Users/jacki/kiln/internal/cli/cli.go#L1220-L1430) | 三个命令各加 `--json` 分支 + `writeJSON`（`encoding/json`，缩进 2 空格）；DTO：`issueCore`（两命令共用）+ `statusDoc` / `listDoc` / `checkDoc`，**不反射 `model.Issue`** |
| [cli_test.go](file:///c:/Users/jacki/kiln/internal/cli/cli_test.go#L926-L1243) | 新增 9 个用例：三份 golden + 空仓 / prose 隔离 / 布尔恒显 / `parent` / 共享行 / verbose 不变形 / 退出码 |
| [README.md](file:///c:/Users/jacki/kiln/README.md#L209-L235) | 新增「机器可读输出」一节；三条命令补 `[--json]`；改写「与 agent 协作」里"稳定文本就够了"那句；「已知边界」补一条 |
| `kiln -h` | footer 补一行 `status / list / check 的 flags: --json …` |
| [mvp-plan.md](file:///c:/Users/jacki/kiln/docs/mvp-plan.md#L312) §10 | 该条改记为"**已落地（三个命令）**"，其余命令与 `--toon` 标为仍待定 |

### 验证

| # | 做法 | 结果 |
| --- | --- | --- |
| 1 | `go vet ./...` + `go test ./...` | 全绿 |
| 2 | **改前 / 改后二进制对拍**：`status`、`list`、`check` 的文本输出 | **逐行相同**——默认视图确实一字未变 |
| 3 | 真仓 `status --json` | 30 行平铺；5 条跨域落位带 `parent`（`target-honesty ← action-diff` 等）；`mode=off`、`open=0` |
| 4 | 真仓 `list --json --action hypothesize` | `filter` 回显、`count=9`、行与 `status` 同形 |
| 5 | 真仓 `check --json` 与 `check` | 退出码同为 0；`ok=true`、`warn=9`、`error=0` |
| 6 | `status --json 1>j.json 2>j.err` | **stderr 0 字节**——stdout 只有那一个文档 |
| 7 | 空仓（无任何 kiln 动作） | `"issues": []` / `"results": []`（**不是 `null`**）；`check --json` 不吐文本视图那句"没有 kiln 动作"提示 |
| 8 | `check --json --verbose` vs `check --json` | **逐字节相同**——schema 与 flag 无关 |
| 9 | `--json` 与既有 flag 混用（`--domain` / `--tag` / `--action`） | 直接复用既有 `parse`，无需新的参数处理 |

### 落地时才暴露的两个点

1. **`check --json` 的退出码一开始漏了。** 第一版 `checkJSON` 直接 `return e.writeJSON(doc)`，于是**有 error 也退 0**——`ok` 字段与退出码当场脱钩，"同义"变成空话。是 golden 用例（期望 exit 1）抓到的。修法：**先写文档、再按 `ok` 决定退出码**——非零退出不能换来"报告丢了"。这条已写进函数的注释，因为它不是实现细节，是契约。
2. **"行序同源"得是构造出来的，不能是约定出来的。** 文本视图原来把 `├──` / `│` 前缀直接拼进 `left`，JSON 里没有这层字符串。若两边各写一次遍历，迟早分叉。所以抽成 `statusRow` + 一次 `statusWalk`：**两种渲染都从同一份有序行读出**——文本渲染器按 `depth / isLast / indent` 拼前缀，JSON 只读 `parent`。golden 与文本用例因此互相锁住。
3. （顺带）`-h` 的旗标说明走了既有的 footer 分行风格，**没有**往命令列里塞 `[--json]`：命令列里有 `kiln commit <issue> <action> [flags]` 这种长货，再拉宽会让整张表的说明列跟着后移。

### 结论

三个命令的 `--json` 与（续）的 schema 一致；**默认文本视图、`check` 的文本输出与退出码均未受影响**（对拍 + `Message` 原样保留）。`--json` 目前是**旁路**：它不改任何既有行为的默认值。

### 遗留（与 §7 合并，未动）

- `path` / `show` / `worktree list` 的 `--json`——原文块（patch / stat）口径待定；
- `--toon`、紧凑输出；
- 错误路径仍走 stderr prose（`打开仓库失败` 这类），未结构化；
- `check` 结果里是否带提交 id。

## 结论（已定）

2026-10-23 按 §7 的清单把三个命令复核了一遍：**实现未改一行**；唯一动手的是**把文档对齐到已交付的行为**（本质上是原 §4 写于落地当时，随后被 `relations-impl` / `silent-flag-drop` 的落地改掉了）。

### 复核（真仓 + 最小仓）

| # | 检查 | 结果 |
| --- | --- | --- |
| 1 | 真仓 `status --json` | 47 条 issue、顶层 `summary` / `worktree` / `issues`；行键 `id`/`domain`/`action`/`done`/`has-worktree`，无 `parent` |
| 2 | 真仓 `list --json --action hypothesize` | `filter` 回显、`count` 与 `results` 各 19 |
| 3 | 真仓 `check --json` | `ok=true`、`summary` 34/13/0、`results` 47 行；exit 0；stderr 0 字节 |
| 4 | `check --json --verbose` | 与不带 verbose **逐字节相同** |
| 5 | 空仓 | `issues: []`、`results: []`，exit 0，不出提示句 |
| 6 | 错误仓（父靶不存在） | `"ok": false`、`rule: relation-target`、`ref` 为靶、exit 1 |
| 7 | 坏 trailer 键 | `rule: unknown-trailer`、`ref` 为键名、`action` 为所在动作 |
| 8 | `go build` / `go vet` / `go test ./...` | 全绿 |

### 原文块的出人（以本文 §4 为准）

| 原文 §4 | 现状 | 依据 |
| --- | --- | --- |
| `derived-from` / `spawns` 两个码，`ref` 指父 | 统一成一个 `relation-target`（**warn**），覆盖全部写侧关系名的靶存在性 | `relations-impl` 验收 5、60b02b9 |
| 示例把靶缺失写成 `"level": "error"` | 现为 **warn**（`ok` 仍 `true`，不拉退出码） | 同上 |
| 未提 `unknown-trailer` | 新增码；`ref` 是**键名**、`action` 是该动作 | `silent-flag-drop`（处处报，不静默） |

`ref` 的用途由"仅 derived-from / spawns"扩成 `relation-target`（靶 issue）/ `unknown-trailer`（键名）；`action` 由"仅 terminal-action"扩成 `terminal-action` / `unknown-trailer`。死常量 `RuleDerivedFrom` / `RuleSpawns` 已无引用，随本次对齐删除。

### 已知缺口（留档，未做）

`relation-target` 的 finding 里**不带关系名**：`Diagnostic` 只有 `ref`，同一 issue 的多条坏边分不出是哪一条。补 `relation` 字段会动 JSON schema，属于读侧契约的扩字段，本轮不动；若消费方需要，单独开 issue。

### 遗留（承接 §7）

- `path` / `show` / `worktree list` 的 `--json`；
- `--toon`、紧凑输出；
- 错误路径仍走 stderr prose；
- `check` 结果里是否带提交 id；
- 上述「关系名缺失」。

## 后续（2026-10-27）：`cli-consistency` 批 A 的键名修订

本文件的 `status` / `check` 示例（hypothesize / apply / 复核各节）保留了 2026-10-23 的键名（`complete` / `in-progress` / `mode` / `ok` / `summary`），作过程记录。`cli-consistency` 的采集发现这些键与其它文档同名异义，批 A 已改名：

- `status.summary.complete` → `done`，`in-progress` → `open`；
- `status.worktree.mode` → `policy`，`worktree list.mode` → `backend`；
- `check.ok` → `passed`，`check.summary` → `findings`；
- `show --stat` 不再把 `body` 换成 diffstat，而是追加 `stat`。

契约与四条成文规则（`id` vs `issue`、容器恒写、`title`/`commit` 粒度、JSON 不随语言变）见 `docs/read-json.md` 文末「批 A 的键名修订」。本文件的「遗留」清单不变。

## 后续（2026-10-28）：`kiln action list --json`

`action list` 是词表命令，不读历史、不需要仓库。`--json` 输出一份对象：

```json
{
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  "source": "file",
  "config": "C:\\Users\\jacki\\kiln\\.kilnconfig.jsonc"
}
```

- `actions` 与 `.kilnconfig.jsonc` 的 `actions` 组同形；`names` 保持声明顺序；
- `source`：`file`（读取了配置文件）或 `default`（缺文件 / 仓外）；`default` 时 `config` 省略；
- `config` 是**当前 OS 的本地绝对路径**：Windows 用 `\`（JSON 源码里转义为 `\\`），POSIX 用 `/`；消费方解码后可直接 `os.Stat` / 打开，不要照抄源码里的转义；
- 不做 `--jsonl`；空仓/仓外输出默认词表并 exit 0；配置解析失败 exit 1；
- 不创建配置文件：`config path` 才会 seed，`action list` 只读。
