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

> 这张表是 `ask` 时的初始清单，保留作过程记录；下文「采集」逐条实测并扩展为 36 条，同时修正表里两处笔误：`path.actions[].id` 已由 `b5e6a14` 改名 `commit`；`←` 出现在 `list --search`，不是 `path`。

## 采集（2026-10-27，HEAD `b5e6a14`）

### 口径与范围

**什么叫“歧义/不统一”**：同一个用户意图（问状态、取 hash、按条件筛、看某动作、空结果），在不同命令或不同出口得到**不同形状 / 不同退出码 / 不同措辞**，且没有一处成文解释为什么不同。

**实测环境**：本仓 `b5e6a14` 构建的 `kiln.exe`，62 issues / 274 actions（53 done / 9 open / 3 domains）；另在 `%TEMP%` 建空 git 仓（`git init` + 一条空提交）测空态；错误路径全部用 `--lang=zh` / `--lang=en` 双跑；JSON 双语言输出做 SHA-256 比对。**未覆盖**：jj 后端、`worktree=on` 的提交流程、真实的非 main worktree 行、配置损坏、`--toon`（非目标）。

### 实测规模（同一仓，同日）

| 命令 | 文本 | `--json` |
| --- | --- | --- |
| `status` | 2,625 B | 18,038 B |
| `list` | 3,516 B | 19,877 B |
| `check` | 5,221 B | 12,862 B |
| `graph` | 6,797 B | 27,317 B |
| `path --all` | —（只支持 JSON） | 129,974 B |
| `show cli-consistency 1` | 4,399 B | 4,633 B |
| `worktree list` | 53 B | 178 B |

### 1. JSON 键名与语义

**C01 `mode` 一键两义**
- 实测：`status --json` → `"worktree": {"mode": "off", "open": 0}`；`worktree list --json` → `"mode": "git"`。
- 问题：前者是隔离策略（`off`/`on`），后者是 VCS 后端（`git`/`jj`）；两个枚举无交集，但键名相同，读者必须先知道自己在读哪份文档。
- 影响：JSON；两个文档的消费方。
- 候选：`status.worktree.policy` / `worktreeList.backend`；或保留并各自写明（值域不重叠，误读概率低）。

**C02 issue id 的键分裂：行里 `id`，根/部分行 `issue`**
- 实测：`list.issues[].id`、`status.issues[].id`、`graph.nodes[].id`；但 `path.issue`、`show.issue`、`check.results[].issue`、`worktree.worktrees[].issue`、`pathBatch.errors[].issue`。
- 问题：`json-commit-hash` 只统一了 hash 那一半（`id`→`commit`），issue id 这一半仍是两套键；而且不是“根用 `issue`、行用 `id`”的整齐划分——`check` 与 `worktree list` 是**行级**却用 `issue`。
- 影响：JSON + 测试 + 文档 + 已知消费方（dsh-kiln 读 `graph.nodes[].id` 与 `path.issue`）。
- 候选 A（小）：行级 `issue` → `id`（`check` / `worktree` / `pathBatch`），根级保留 `issue` 表示“本文档主体”；候选 B（大）：全改 `issue`，`id` 退役。

**C03 `ok` 同键两类型**
- 实测：`check --json` → 顶层 `"ok": true`（布尔，整体是否通过）与 `"summary": {"ok": 53, ...}`（整数，通过条数）。
- 问题：同一文档同一键名两种类型，解码器写 `ok` 路径时得先分辨层级；`summary.ok` 是“条数”而顶层是“结论”。
- 影响：仅 JSON。
- 候选：`summary` 改 `{"passed","warn","error"}`，或顶层改 `"passed"`。

**C04 `body` 一字段两义**
- 实测：`show --json` 的 `body` 是 `git show` 全文（含提交头、trailer、diff）；`show --json --stat` 的 `body` 是 `docs/cli-consistency.md | 48 +++…` 的 diffstat。
- 问题：同一个键随 flag 变成另一种东西；而 `path --json` 里 diffstat 另有 `stat` 键——同一数据在两条命令里两个键名。
- 影响：JSON；`show` 的消费方。
- 候选：`body` 只放提交正文，diffstat 用 `stat`（与 `path` 对齐）；或加 `body-kind`。

**C05 `title` / `commit` 的作用域随层级变**
- 实测：`list` / `status` / `graph` 行的 `title` 是**issue 标题**（= 末条动作 title，缺省回退 subject），`commit` 是**末条动作**；`path.actions[].title` / `.commit` 是**该动作**的。
- 问题：同名键在不同嵌套层指不同粒度；`list` 行同时有 issue 级 `title` 与 `commit`，读者容易当成同一个动作。
- 影响：JSON + 文档。
- 候选：文档明确“issue 行的 title/commit 指末条动作”；行级改名 `last-commit` 代价大、收益小，倾向只文档化。

**C06 “空”的三种写法没有成文规则**
- 实测：无过滤时 `"filter": {}` 仍写出；无关系时 `"relations": []`；空仓 `"issues": []`；而 `errors` / `missing` / `hit` / `tags` / `title` / `commit` 缺失时**整个键省略**。
- 问题：`read-json` 的约定只写了“缺键=无、不用 `null`”，没写“哪些键恒写、哪些省略”；`filter: {}` 与 `hit` 省略是同一份文档里的两种“没有”。
- 影响：契约文档 + 所有 JSON 测试。
- 候选：定一条——“容器恒为数组（空写 `[]`）、可选标量/对象省略、`filter` 恒写（回显请求）”，写进 `read-json` / `json-fields`。

**C07 同一个“issue 行”有四种形状**
- 实测键集：`status` 行 `{id, domain, action, done, title, has-worktree}`；`list` 行 `{id, domain, action, done, title, commit, tags, hit}`；`graph` 节点 `{id, domain, action, done, title, tags}`；`path` 根 `{issue, action, done, actions, relations}`。
- 问题：四份“同一概念”的行，字段覆盖各不同（`status` 缺 `tags`/`commit` 但多 `has-worktree`；`graph` 缺 `commit`；`path` 无 domain/title/tags）。
- 影响：JSON + 文档；已有 `issueCore` 共享结构（代码层已收一半）。
- 候选：文档列一张“哪个视图带哪些字段”的矩阵；不强行统一（各视图用途不同），但 `has-worktree` 是否该只留在 `worktree list` 值得议。

**C08 `check.results[].actions` 与 `path` 的寻址不对齐**
- 实测：`check --json` 的 `actions` 是动作名数组（可重复，如 `["test","test","conclude"]`），没有序号、没有 commit；`path` 用 `ordinal` 寻址。要定位被检查出的动作还得再跑 `path`。
- 影响：JSON（`check` 消费方）。
- 候选：`actions` 改对象数组 `{ordinal, action, commit}`；或只在文档里说明“check 判完整性，定位用 path”。

### 2. 同一概念在不同出口的漂移

**C09 `done` 的值一致，词有七套**
- 实测：五处数值全对得上（`status.summary.complete=53` / `in-progress=9`、`list --status=done` 53 / `open` 9、`status` 行 `done=true` 53、`path --all` 里 `done` 53、`check.summary.ok=53` / `warn=9`）。
- 问题：JSON `done`、`status.summary.complete` / `in-progress`、`check` 的 `terminal-action` 规则、帮助里的“终点动作”、`--status=done`、中文“已完成/进行中”、英文“done/in progress”——同一状态七种叫法。
- 影响：JSON 键 + 帮助 + 文档。
- 候选：JSON 统一 `done`（`status.summary` 改 `done` / `open` 或 `done` / `not-done`），文本保持中文；或只补一张术语映射表。

**C10 计数键四种**
- 实测：`list.count`（过滤后行数）、`status.summary.total`（全量）、`graph.summary.issues`（节点数）、`check.summary.ok/warn/error`（**诊断条数，不是 issue 数**——源码注释明说一个 issue 可失败两次）。
- 问题：`summary` 这个键在 `status` / `check` / `graph` 里结构三套，`check.summary` 的计数单位还不同（条数 vs 个数）。
- 影响：JSON + 文档。
- 候选：`check.summary` 改名 `findings`；其余文档化。

**C11 文本里“状态”有五种渲染**
- 实测：`status` 行 `action-diff               ✓ conclude`（✓ 只给 done）；`list` 行 `action-config  apply  domain=design`（无 done 标记）；`graph` 行 `action-config (apply)`；`path` 首行 `cli-consistency  进行中（当前 ask）`；`show` 首行 `cli-consistency · 1/1 ask · …`。
- 问题：读者要知道 done 得先背终点动作词表（`list`/`graph`），而 `status`/`path` 直接给结论；同一事实五种排版。
- 影响：文本（改则动 golden 测试）。
- 候选：`list` / `graph` 文本加一个与 `status` 同款标记；或文档说明“文本里状态靠动作名推断，结构化看 JSON”。

### 3. 命令与 flag 形态

**C12 开关型 flag 会把紧跟的位置参数吞掉**
- 实测：
  - `show --json cli-consistency` → exit 2，`用法: kiln show <issue> [n] [--stat] [--json]`（id 变成了 `--json` 的值）
  - `show --stat cli-consistency` → 同上
  - `config --json path` → exit 2，`警告: 未知 flag --json` + `用法: kiln config path`
  - `path --json cli-consistency` → exit 2，`flag --json 不接受值: cli-consistency`（**唯一精确报错**）
  - `do --force x ask` → exit 2，`用法: kiln do <issue> <action> [flags]`（`force` 吞了 `x`；`commit` 共用解析器）
- 问题：同一类误用三种表现：精确报错（`path`）/ 泛用法错（`show`、`do`、`commit`）/ 警告 + 用法（`config`）；而正确写法（flag 放后面）没有任何提示。
- 影响：解析器 + 帮助；不改则用户只能靠试。
- 候选：给每个命令声明“开关型 flag”，`parse` 遇到它们不吞下一个 token（`path` 已手工实现同一逻辑）；或统一为 `path` 式报错。

**C13 没有子命令帮助**
- 实测：`kiln -h` / `kiln help` → 完整帮助 exit 0；`list -h` → 单横线当位置参数**静默忽略**，照常输出全量（exit 0）；`show -h` → `未找到问题: -h`（exit 1）；`commit -h` → 用法（exit 2）；`list --help` → `警告: 未知 flag --help` 后继续。
- 问题：最常见的“看这个命令怎么用”在 8 个命令里得到 5 种结果，且没有一处成功。
- 影响：帮助 + 退出码；`TestUsageTextCitesNoRepositoryDocument` 禁止帮助里引用仓库文档。
- 候选：分发前识别 `<cmd> -h|--help`，打印该命令的用法与 flag（可从现有 `usageCommands` + `usage*Flags` 生成）。

**C14 已知 flag 放错命令：警告后继续，且不区分“拼错”与“属于别的命令”**
- 实测：`status --status=done` → 警告 + 全量状态；`graph --domain=nope` → 警告 + 全图；`check --domain=cli` → 警告 + 全检；而 `list --status=closed` → exit 2。
- 背景：未知 flag 警告是 `silent-flag-drop` 的既定契约（`TestUnknownFlagWarnsOnEveryCommand`），**不是**本轮要推翻的。
- 问题：`--status` 在 `list` 合法、在 `status` 非法；`--domain` 在 `list`/`commit` 合法、在 `graph`/`check` 非法（而 `graph` 明明按 domain 组织节点）。用户拿到的是“未知 flag”，不是“它在 list 里”。
- 影响：帮助 + 错误措辞。
- 候选：保留不拦，但把措辞改成“`--domain` 只在 list/do/commit 生效”；或让 `graph`/`check`/`status` 接受 `--domain` 过滤（那是新功能，另立项）。

**C15 重复 flag 三种语义**
- 实测：`do … --title=one --title=two` → 最终 `title: two`（后者胜，静默）；`do … --tag=a --tag=b` → `tags: a, b`（累加）；`list --domain=cli --domain=design` → `count: 29`（= design 单域 29；cli 单域 32）——只留最后一个；`list --search=json --search=worktree` → `filter.search: "worktree"`、`count: 10`（json 单搜 13）——前一个查询被静默丢弃。
- 问题：`parse` 注释说“repeated `--key` all accumulate”，但 `get()` 取最后一个；哪些 flag 走 `list()`（累加）、哪些走 `get()`（覆盖）没有成文规则，且 `list` 的覆盖会让用户以为查了两个词。
- 影响：行为 + 文档。
- 候选：定规则“列表型累加、标量型后者胜”，并让 `list` 的 `--domain` / `--tag` / `--action` / `--search` 累加或明确拒绝重复。

**C16 同一个 `--tag` 两种文法**
- 实测：`do demo ask --tag=a,b` → `tags: a, b`（两个标签）；`list --tag=json` → 1 条，`list --tag="json,path"` → **0 条**（整个串当字面量）。
- 问题：帮助在 do 段写 `--tag <标签>[,<标签>]`、在全局写“均可重复、逗号分隔”，而 `list --tag` 既不切分也不重复；同名 flag 两种语法。
- 影响：行为 + 帮助。
- 候选：`list` 也按逗号切分（与 do 一致）；或帮助里明写 `list` 只接受单个标签。

**C17 过滤值校验不一致**
- 实测：`list --status=closed` → exit 2；`list --search=`（空）→ exit 2；`list --action=nope` / `--domain=nope` / `--tag=nope` → exit 0、stdout 0 B、stderr 0 B。
- 问题：`--status` 是受控枚举所以校验；`--action` 同属受控词表（`do` 会拒绝非法动作）却不校验；同类输入两类后果。
- 影响：退出码 + 帮助。
- 候选：`--action` 校验动作词表（与 `do` 同源）；或全部静默 + 空结果提示（见 C22）。

**C18 子命令前不能放 flag**
- 实测：`worktree --json list` → exit 2，`用法: kiln worktree new <issue> | …`；`worktree list --json` 正常。`config` 反过来先 `parse`，所以 `config --json path` 是“警告 + 用法”（C12）。
- 问题：两个带子命令的命令用了两种做法（先 parse / 先取子命令），用户得记住 flag 放在子命令之后。
- 影响：行为 + 帮助。
- 候选：`worktree` 也先 `parse` 再取子命令（与 `config` 对齐）。

**C19 多余位置参数静默忽略**
- 实测：`status extra`、`list extra`、`check extra`、`graph extra`、`worktree list extra`、`show <id> 1 2` 全部 exit 0；只有 `do`/`commit` 对第三个位置参数报错（`object 已退役`）。
- 背景：`silent-flag-drop` 明确把“未知位置参数”排除在范围外，所以这是**已知未决**。
- 问题：`show cli-consistency 1 2` 里 `2` 被丢掉，用户无法知道命令少读了参数。
- 影响：行为 + 退出码。
- 候选：读命令的多余位置参数 → exit 2 或警告；`path` 例外（多 id 是合法批量）。

**C20 `path` 的 usage 行没写多 id 形态**
- 实测：`kiln path cli-consistency json-commit-hash --json` 合法（批量、保序），但 `-h` 的用法行只写 `kiln path [<issue>|--all]`；只有后面的“path 额外 flag”一行提到“也可直接给多条 issue”。重复 id 会输出两份（`path a a --json` → 2 条，不去重）。
- 影响：帮助 + 文档。
- 候选：用法行写 `kiln path <issue> [<issue>…] | --all`；重复 id 去重或文档说明保留。

### 4. 退出码与错误通道

**C21 矩阵（实测）**

| 情形 | 退出码 | 通道 |
| --- | --- | --- |
| 成功；无命中；空仓 | 0 | stdout（无命中仅 `--search` 有 stderr 提示） |
| 未知 issue；后端失败；`check` 有 `error`；批量部分失败 | 1 | stderr（批量例外：stdout 整份文档 + `errors[]`） |
| 用法错；非法枚举；冲突 flag；未知命令；`--lang` 非法 | 2 | stderr |

**C22 空结果提示只覆盖 `--search`**
- 实测：`list --search=zzzznotfound` → exit 0 + stderr `没有匹配 "zzzznotfound" 的 issue`；`list --domain=nope` → exit 0、stdout 0 B、stderr 0 B；空仓 `list` 同样 0 B/0 B。
- 问题：同一个“过滤后为空”的用户疑问，只有一条路径有回答；其余看起来像命令坏了。
- 影响：stderr 提示（不影响 stdout 纯净）。
- 候选：所有过滤与空仓都给一行 stderr 提示（`没有匹配 <条件> 的 issue`），JSON 文档不变。

**C23 `--json` 的错误通道：单条与批量两套**
- 实测：`path no-such-issue --json` → exit 1、stdout 0 B、stderr `未找到问题: no-such-issue`（有测试 `TestReadJSONErrorsPrintNothing` 钉住）；`path cli-consistency no-such --json` → exit 1、stdout 完整文档、`errors: [{"issue":"no-such","error":"unknown-issue"}]`、stderr 0 B。
- 问题：同为“拿不到”，一个 stdout 空、一个 stdout 有文档；`path --all` 在两种模式间切换，消费方得写两条分支。
- 影响：契约文档（`path-batch` 已写形状，但没写这条取舍）。
- 候选：保持（批量本来就是为了不中断），但在 `read-json` / `path-batch` 里写死“单条失败 stdout 0；批量失败文档 + `errors` + 1”。

**C24 同一失败两种表示：token vs prose**
- 实测：批量 `"error": "unknown-issue"`（语言中立 token，`--lang=en` 不变）；单条 `未找到问题: x` / `no such issue: x`（随语言变）。
- 问题：机器要读单条错误只能解析 prose 或忽略；JSON 不出 prose 的原则只在批量里落实。
- 影响：错误契约。
- 候选：保持（单条错误目前没有机器消费方），列入 `machine-readable-output` 的“错误 prose 结构化”遗留。

**C25 `check` 的 warn 不失败，但文本打 ⚠**
- 实测：本仓 `warn=9`、`ok:true`、exit 0；文本却逐条打 `⚠`。
- 问题：`⚠` 与退出码 0 的组合没有成文解释（“只有 `error` 让 check 非零”）；CI 若以 ⚠ 为准会把 9 个进行中 issue 当失败。
- 影响：文档 + 帮助。
- 候选：写进 `check` 的结论与 `-h`（“warn 是提醒，不影响退出码”）。

### 5. 文本与 JSON 对齐

**C26 hash 长度：文本 7 位，JSON 40 位**
- 实测：`path` 文本 `2d03af2`（`shortID` 取 7 位，再按 8 列对齐）；`path --json` `"commit": "2d03af264f7c5b846b3a94e68e30f6ba277cce81"`；`show` 首行与 `commit` 确认行同样是 7 位。
- 问题：同一动作两个地址；从文本复制 hash 去 `git show` 能用（7 位可解析），但和 JSON 对不上时难以判断是否同一提交。
- 影响：文本（改则动 golden）。
- 候选：保持（短 hash 是给人扫的）+ 文档写明“文本 7 位前缀、JSON 完整 40 位”。

**C27 `list` 文本两种布局，JSON 一种**
- 实测：普通 `action-config  apply  domain=design`；`--search` 时 `help-relation-flags  apply  <当前标题>  ← <命中的动作 subject>`。JSON 一份文档：`{id, domain, action, done, title, commit, tags, hit}`。
- 问题：普通文本没有 title/commit、search 文本没有 tags；`←` 只在一种布局出现（帮助里也未提）；`tagsSuffix` 这个函数名还同时渲染 `domain=`（代码命名与内容不符）。
- 影响：文本 + 帮助。
- 候选：文档化两种布局（保持），或普通布局补 title。

**C28 `status` 文本没有 title / `has-worktree`，JSON 有**
- 实测：文本行 `  action-diff               ✓ conclude`；JSON 行含 `title`、`has-worktree`。
- 问题：JSON 比文本多两列事实，且 `has-worktree` 与 `worktree list` 重复表达。
- 影响：文本/JSON 对齐。
- 候选：保持（文本是概览），文档写明 JSON 多出哪些键。

**C29 关系名：文本用反名，JSON 用声明名**
- 实测：`path cli-consistency` 文本对入边显示 `causes json-commit-hash（causes ＝ 我引发了谁）`；`path json-commit-hash --json` 对同一条边给 `{"from":"cli-consistency","to":"json-commit-hash","name":"caused-by","directed":true}`。
- 问题：文本读者与 JSON 读者看到两个名字；`read-json` 已写明“入边不换反名”，但这是**唯一**一处文本/JSON 语义不同名的地方。
- 影响：文档（已写）。
- 候选：保持（文本为可读性换名，JSON 为可判定用声明名）。

**C30 JSON 与 `--lang` 完全无关（好消息）**
- 实测：7 个 `--json` 输出在 `--lang=zh` / `--lang=en` 下 SHA-256 全部相同；有测试 `TestJSONIgnoresTheLanguage`。
- 问题：无；但这条契约只在测试里，没写进 `read-json`。
- 候选：写进 `read-json`（“JSON 是数据，不随语言变；错误 prose 随语言变”）。

**C31 完成标记只出现在 `status` 文本**
- 实测：`✓` 只在 `status`；`list` / `graph` 文本没有 done 标记（见 C11）。
- 候选：并入 C11。

### 6. 术语

**C32 中文里「问题」与「issue」混用**
- 实测：`问题总数` / `未找到问题: x` / `问题没有动作: x` / `问题关系图`；同一工具又说 `没有匹配 %q 的 issue` / `没有 issue %q` / `拒绝: issue %q 已经有起点动作` / `警告: issue ID %q` / `提示: issue ID 已归一化`。英文全部统一为 issues。
- 问题：中文用户会在两条相邻命令里看到两个词指同一对象；没有一处说“问题 = issue”。
- 影响：i18n 词条 + 文档。
- 候选：中文统一用「issue」（与 JSON 键、帮助里的 `issue:` 一致），或统一用「问题」并只在 JSON 键保留 `issue`；不能继续混。

**C33 `worktree` / `workspace` / `area` 三层词**
- 实测：用户面只出现 `worktree`（命令、JSON 键、文本）；代码里是 `workspace()`、`workspaceBackend`、`seedArea`、`areaIssues`、`vcs.Workspace`；`retiredCommands` 只把 `workspace` 映射到 `worktree`。
- 问题：用户面一致，代码面三套词；不影响 CLI，但新人读代码要翻译一次。
- 影响：仅代码/文档命名。
- 候选：保持（用户面正确），代码注释里点明三者同义。

**C34 `--status` 的 `open` / `done` 与 JSON 的 `done`**
- 实测：`--status=open`（帮助写“未收尾”）/ `done`（“末条动作是终点动作”）；JSON 是 `done` 布尔；`status.summary` 是 `complete` / `in-progress`。
- 候选：并入 C09（术语映射表）。

### 7. 空态与提示

**C35 空仓（`git init` + 一条空提交）逐命令实测**

| 命令 | 输出 | 退出码 |
| --- | --- | --- |
| `status` | 计数 0 + `提示: 用 kiln do …` + worktree 图例 | 0 |
| `check` | `没有 kiln 动作（检查提交信息是否同时带了 issue: 与 action:）` | 0 |
| `graph` | `（空）` | 0 |
| `list` | **0 B**（stdout / stderr 都空） | 0 |
| `path --all --json` | `{"issues": []}` | 0 |
| `worktree list` | main 一行 | 0 |
| `config path` | 写入并打印 `.kilnconfig.jsonc` | 0 |
| `show` / `path <id>` | `未找到问题` | 1 |

- 问题：8 个出口里 5 个回答“为什么是空的”，`list` 完全不回答（与 C22 同根）；空仓 `list` 与“过滤后为空”看不出区别。
- 影响：stderr 提示。
- 候选：`list` 空结果统一提示（见 C22），并让空仓提示说清“还没有任何 issue 动作”。

**C36 未知 domain 在三个命令三种后果**
- 实测：`list --domain=nope` 静默空；`graph --domain=nope` 警告“未知 flag”后画全图；`check --domain=nope` 警告后全检；`status --domain=nope` 警告后全量。
- 问题：同一“按 domain 收窄”的意图，一个给空、三个给全量（且警告说的是 flag 不认识，不是 domain 不存在）。
- 影响：行为 + 措辞。
- 候选：并入 C14（已知 flag 放错命令）；若将来给 `graph`/`check` 加 domain 过滤，需单独立项。

### 已由测试钉住（候选=“保持”的依据）

- 未知 flag：警告后继续、exit 0（`TestUnknownFlagWarnsOnEveryCommand`、`TestDoAcceptsForceWithoutAWarning`）；
- `--json` 忽略 `--verbose`、形状只有一份（`TestJSONKeepsOneShapeRegardlessOfVerbose`）；
- JSON 与 `--lang` 无关（`TestJSONIgnoresTheLanguage`）；
- JSON 错误 stdout 0 B（`TestReadJSONErrorsPrintNothing`）；
- 空容器是 `[]` 不是 `null`（`TestStatusJSONEmitsAnEmptyArrayNotNil`、`TestPathAllJSONOnAnEmptyRepoWritesAnEmptyArray`）；
- 搜索 0 命中 exit 0 + stderr 提示（`TestListSearchNoMatchIsNotAnError`）；
- 文本视图逐字节（`TestStatusRendersDomainBlocks`、`TestPathStatIndentsEachAction`、`TestListSearchTextShowsTitleAndHit` 等 golden）。

**分类（必须改 / 可改可不改 / 保持）留待 hypothesize**；本节只给实测、影响面与候选，不改任何行为。

## hypothesize（2026-10-27）：分类与四批

### 结论先行

36 条里 **16 条必须改 / 3 条可改可不改 / 17 条保持**，拆成四批，每批一次 `apply`：

| 批 | 主题 | 条数 | 风险 |
| --- | --- | --- | --- |
| **A** | JSON 契约（同键异义 / 词表统一 / 文档纠错） | 5（C01 C03 C04 C09 C10） | 低：无已知消费方读这些键 |
| **B** | 解析器与帮助（开关 flag、子命令帮助、重复与多余参数） | 6（C12 C13 C15 C18 C19 C20） | 高：每命令都过解析器，测试先行 |
| **C** | 过滤语义与空态提示 | 4（C16 C17 C22 C35） | 低：只动 `list` |
| **D** | 中文术语统一 | 1（C32） | 低：只动 zh 词条 |

顺序有理由：先定数据契约（A 决定 B/C 之后 JSON 长什么样），再修入口（B 是用户每天会踩的），再修语义（C），最后动措辞（D）。**文本视图除帮助外逐字节不动**；每一批都能单独回退。

### 判定标准

- **必须改**：同一输入两种后果（C12 C15 C19）、同一键两种含义（C01 C03 C04）、静默给出错误答案（C16 C17）、日常入口失败或帮助与行为不符（C13 C18 C20）、同一状态两套词且值已一致（C09 C10）、空结果零反馈（C22 C35）、用户可见的术语混用（C32）。
- **可改可不改**：有歧义但没有消费方，或只是措辞级收益（C08 C14 C36）。
- **保持**：有意为之、已有测试钉住，或改动成本明显大于收益（其余 17 条）。

### 分类表

| # | 判定 | 批 | 理由 |
| --- | --- | --- | --- |
| C01 `mode` 双义 | 必须改 | A | `off/on` 与 `git/jj` 共用一键，改两处键名即消失 |
| C02 `id` / `issue` | 保持 | — | 规则其实成立：`id` = 行主体就是 issue；`issue` = 文档主体或非 issue 行。补文档即可 |
| C03 `ok` bool/int | 必须改 | A | 同文档同键两类型，对类型化解码器不友好 |
| C04 `body` 双义 | 必须改 | A | `--stat` 不再替换 `body`；同时纠正 `read-json` 里“不含 diff”的错误描述 |
| C05 title/commit 作用域 | 保持 | — | 层内一致（行=末条动作，path 动作=该动作）；文档写明 |
| C06 空容器规则 | 保持 | — | 容器恒写、可选省略、`filter` 回显；写进契约，不改形状 |
| C07 四种行形状 | 保持 | — | 四视图用途不同；文档给字段矩阵，不强行统一 |
| C08 `check.actions` 无序号 | 可改可不改 | — | 没有消费方需要从 check 直接定位动作；要做时再加 `ordinals` |
| C09 done 七套词 | 必须改 | A | 数值已全一致，只统一 `status.summary` 的键为 `done`/`open` |
| C10 `check.summary` | 必须改 | A | 它数的是 finding 不是 issue，改名 `findings` 即止 |
| C11 文本五种状态渲染 | 保持 | — | 文本稳定优先；JSON 已有 `done`，文档说明即可 |
| C12 开关 flag 吞位置参数 | 必须改 | B | 用户每天会踩；`path` 已有修法，推广到全命令 |
| C13 无子命令帮助 | 必须改 | B | 八个命令五种结果且没有一种成功 |
| C14 已知 flag 放错命令 | 可改可不改 | — | 警告契约保留；“该 flag 属于 list”的措辞改进留待用户抱怨 |
| C15 重复 flag 三种语义 | 必须改 | B | 静默丢弃查询/域是数据丢失 |
| C16 `--tag` 两种文法 | 必须改 | C | 逗号静默返回 0；先明确拒绝，多标签过滤留待有人需要 |
| C17 过滤值校验不一 | 必须改 | C | `--action` 与 `--status` 同为受控词表，却一个校验一个不校验 |
| C18 子命令前 flag | 必须改 | B | 与 `config` 对齐的成本很低 |
| C19 多余位置参数静默 | 必须改 | B | “工具必须诚实”；silent-flag-drop 只豁免了 flag，这次补位置参数 |
| C20 path usage / 重复 id | 必须改 | B | 帮助与行为不符；重复输出两份没有意义 |
| C21 退出码矩阵 | 保持 | — | 0/1/2 的分工合理且已稳定；写进文档 |
| C22 空结果提示只覆盖 search | 必须改 | C | 同一个疑问只有一条路径有回答 |
| C23 单条/批量错误通道 | 保持 | — | 批量不中断是设计目的；写死“单条 stdout 0、批量文档+errors+1” |
| C24 token vs prose | 保持 | — | 单条错误无机器消费方；归 `machine-readable-output` 的遗留 |
| C25 check warn 不失败 | 保持 | — | 行为正确；补一句“只有 error 让 check 非零” |
| C26 hash 7 vs 40 | 保持 | — | 短 hash 给人扫，JSON 给机器；文档写明 |
| C27 list 两种文本布局 | 保持 | — | 文本稳定；文档补 `←` 与两种布局 |
| C28 status 文本少 title | 保持 | — | 概览视图有意精简；文档写 JSON 多出的键 |
| C29 关系名文本/JSON 不同 | 保持 | — | 已文档化；文本为可读性换反名 |
| C30 JSON 与语言无关 | 保持 | — | 已实现且有测试；写进 `read-json` 成为契约 |
| C31 ✓ 只在 status | 保持 | — | 与 C11 同一条：文本稳定优先 |
| C32 问题/issue 混用 | 必须改 | D | 同一工具两条相邻命令两个词 |
| C33 worktree/workspace/area | 保持 | — | 用户面已统一；代码注释点明同义即可 |
| C34 `--status` 与 done | 保持 | — | 值域命名合理；归入 C09 的术语映射表即可 |
| C35 空仓提示 5/8 | 必须改 | C | 只有 `list` 静默；其余出口已有回答，保持 |
| C36 未知 domain 三种后果 | 可改可不改 | — | 与 C14 同一条：警告契约保留，措辞改进留待需求 |

### 批次 A：JSON 契约

**改什么**（只动 JSON 键与文档，不动文本）：

| 文档 | 现在 | 改为 |
| --- | --- | --- |
| `status --json` | `"summary": {"complete": 53, "in-progress": 9, …}` | `"summary": {"done": 53, "open": 9, …}` |
| `status --json` | `"worktree": {"mode": "off", "open": 0}` | `"worktree": {"policy": "off", "open": 0}` |
| `worktree list --json` | `"mode": "git"` | `"backend": "git"` |
| `check --json` | `"ok": true, "summary": {"ok": 53, …}` | `"passed": true, "findings": {"ok": 53, …}` |
| `show --json --stat` | `"body": "<diffstat>"` | `"body": "<提交全文>", "stat": "<diffstat>"` |

**同时纠错与补契约（不改实现）**：

- `read-json` 说 `show.body` “不含 diff 原文”，实测 `body` 就是 `git show` 全文、**包含 diff**——按实现改正，并把“是否拆出 `diff` 字段”记为以后的独立决定；
- `read-json` / `json-fields` 补四条成文规则：`id` vs `issue`（C02）；容器恒写 vs 可选省略；`filter` 回显（C06）；JSON 不随 `--lang` 变（C30）。

**为什么先做**：这 5 条都是“同一份文档里自相矛盾”，与命令面无关；键名定下来，B/C 批的测试与文档才能一次写完。

**验收**：`gofmt` / `go vet` / `go test ./...`；`status` / `check` / `worktree list` / `show --stat --json` 冒烟；`--lang=zh` / `en` 输出仍逐字节相同；`docs/read-json.md`、`docs/json-fields.md` 与新形状一致；dsh-kiln 的 `action.commit` 迁移（见“跨批约束”）。

**不做**：不给 `check` 加 `commit`（仍无消费方）；不改 `path` / `list` / `graph` 的键；不把 `body` 换成“正文不含 diff”（要新后端方法，收益未证）。

### 批次 B：解析器与帮助

**改什么**：

1. `parse(args, switches...)`：`--name` 后面跟非 `--` token 时，**只有非开关 flag 才消耗它**；开关带显式值（`--json=x`）统一 exit 2，消息与 `path` 现在的“flag --json 不接受值: x”一致。
   - 开关表：do/commit `{force}`；path `{all,json,stat,shortstat}`；show `{stat,json}`；status `{json}`；check `{json,verbose}`；list `{json}`；graph `{json,mermaid}`；worktree `{json,force}`；config 无。
   - 效果：`show --json cli-consistency`、`show --stat cli-consistency`、`commit --force <issue> <action>`、`worktree --json list` 全部按“flag + 位置参数”解析，不再吞参数。
2. `kiln <cmd> -h`、`kiln <cmd> --help`、`kiln help <cmd>` 打印该命令的用法行 + 该命令的 flag 段（从 `usageCommands` / `usageOtherFlags` 生成，不新增文案源）；`kiln help` 仍是全量。
3. 重复 flag：开关幂等；**标量值 flag 重复 → exit 2**（`--domain`/`--action`/`--status`/`--search`、do 的 `--title`/`--target`/`--scope`/`--domain`、graph 的 `--format`/`--issue`/`--depth`）；列表型（tag/refs/关系/unrelate/only）继续累加。
4. 多余位置参数：list/status/check/graph 期望 0、show ≤2、worktree new/remove 期望 1、worktree list 0、config 1 → 超出 exit 2；`path` 不限（批量），`do/commit` 第三参仍报“object 已退役”。
5. `worktree` 先 `parse` 再取子命令（与 `config` 对齐）；`path` 的用法行改成 `kiln path <issue> [<issue>…] | --all`，批量 id 去重（保序）。

**为什么第二批**：这是用户每天的第一道门；改完之后，示例与文档都能用“flag 在前”的自然写法，不用绕。

**测试先行**（先写失败的，再改）：`show --json <id>` 成功、`show --json=x` exit 2、`commit --force <id> <action>` 在 fake backend 下成功、`list -h` exit 0 有用法、`kiln help list` 同、`list --search a --search b` exit 2、`list extra` exit 2、`worktree --json list` 成功、`path a a --json` 只出一份。

**风险**：解析器是全局改动。回退单位是整批；`parse` 保持变参形式，调用点机械更新，golden 文本测试提供第二道保险。

### 批次 C：过滤语义与空态

**改什么**（只动 `list`）：

1. `--tag` 含逗号或重复 → exit 2：`--tag 只接受一个标签（当前: "json,path"）`；帮助里写明“多标签过滤暂不支持”。**不**顺手做 AND 语义——`filter.tag` 的 JSON 形状、engine API 都要跟着变，等真有需求再立项。
2. `--action` 按动作词表校验（复用 `do` 的 `非法 action: %q（可选: %s）`），exit 2。
3. 空结果提示：过滤后为 0 时，stderr 给一行
   - 仓里本来没有动作：`仓库里还没有任何动作（用 kiln do <issue> ask 开一条）`；
   - 否则：`没有匹配 domain=cli status=done 的问题`（把实际生效的条件拼进去；`--search` 沿用现有措辞“没有匹配 %q 的 issue”，到 D 批统一成“问题”，避免两种提示叠加）。
   - stdout 仍 0 字节；`--json` 仍是一份 `{filter, count: 0, issues: []}`。
   - 本批新增的文案直接按 D 批的术语规则写（“问题”，不用“issue”）。

**为什么第三批**：B 把输入面修好之后，C 才能保证“用户打的每个条件都被读懂，读不懂就说话”。

**验收**：`list --tag="json,path"` exit 2；`list --action=nope` exit 2；`list --domain=nope` 有 stderr 提示且 stdout 0；空仓 `list` 有提示；`--json` 文档不变。

### 批次 D：中文术语统一

**规则**：**「问题」= 中文叙述里的实体；`issue` 只保留在标识符位置**（`issue:` trailer、`--issue`、JSON `issue` 键、`<issue>` 占位、`issue ID`、`issue 参数`）。

**改这些 zh 词条**（en 只改 key 的映射，不改措辞）：

- `没有匹配 %q 的 issue` → `没有匹配 %q 的问题`
- `没有 issue %q`（graph 邻域）→ `未找到问题: %s`（与 path/show 同一句）
- `拒绝: issue %q 已经有起点动作…` → `拒绝: 问题 %q 已经有起点动作…`
- `没有 issue 为 %q 的 worktree` → `没有属于问题 %q 的 worktree`
- `issue 不会被关闭`（帮助说明）→ `问题不会被关闭`
- `--all 与 issue 参数不能同用` → `--all 与问题参数不能同用`

**保持**：`问题总数`、`问题关系图`、`按条件过滤问题`、`kiln do <issue> <action>` 这类占位写法。

**验收**：zh 下不再出现“问题…issue…”同一句混用；`TestEnglishErrorsSpeakEnglish` 与 i18n 键一致性测试仍过；`--lang=en` 输出不变。

### 跨批约束

- 每批一次 `apply`；中间批落地后 issue 会显示“已完成”，下一批动作会把它翻回进行中——这是 `done` 的既定语义，不是 bug。
- 文本视图（`status` / `list` / `path` / `show` / `graph` / `check` / `worktree list`）除**新增帮助**外逐字节不变；`TestStatusRendersDomainBlocks` 等 golden 是护栏。
- 每批都要跑：`gofmt` / `go vet` / `go test ./...`、真仓 62 issues 冒烟、空仓冒烟、zh/en 各一次；JSON 双语言哈希仍要相同。
- **已知消费方 dsh-kiln**：它读 `graph.nodes[]`（不变）与 `path.issue` / `actions[]`（不变），但 `action.id` 在 `b5e6a14` 已改名 `commit` 未同步——**任何一批开始前都应先修它**，否则时间线会展示错误事实。该文件在沙箱工作区外，需用户批准写入。
- 文档随每批更新：`read-json`、`json-fields`、`machine-readable-output` 的遗留清单、本文件追加 apply 记录。

### 验收（四批全部落地时）

1. 36 条逐条有落点：16 条已改、3 条写明暂不改、17 条写明保持理由；
2. 四批的验收项全部复跑通过；
3. `read-json` / `json-fields` 与实现一致，包括 C04 的 body 描述纠错与 C02/C06/C30 的新规则；
4. dsh-kiln 与 CLI 同步（`action.commit` + 本批键名）；
5. 没有 golden 文本意外变化；帮助新增的 `<cmd> -h` 有测试；
6. `kiln check` 仍全绿（53 ok / 9 warn / 0 error）。

### 本轮明确不做

- `check.actions` 加 ordinal（C08）；
- `graph` / `check` / `status` 的 `--domain` 过滤（C14/C36 的另一半）；
- 文本状态标记统一、`list` 文本补 title（C11/C27）；
- 多标签过滤与 `filter.tag` 形状变更（C16）；
- 单条错误的 JSON 化（C24，归 `machine-readable-output`）；
- 把 `show.body` 里的 diff 拆出去（要新后端方法，另立项）；
- 没有消费方的新键（例如给 `check` / `graph` 加 `commit`——`json-commit-hash` 已裁掉的那类）。

## 验收（盘点完成时）

- 每个维度至少一条**实测样例**（命令 + 现在的输出 + 为什么有歧义/不统一）；
- 每条给出候选与影响面（改 JSON / 改文本 / 改退出码 / 只改文档）；
- 分类：**必须改 / 可改可不改 / 保持**；
- 盘点阶段不改任何行为；分类后按批落地（作为本 issue 的后续动作），每批一个动作。

## 非目标

- 不重命名命令（除非盘点后单独立项）；
- 不引入查询语言 / 搜索语义；
- 不改 `check` 的规则集；
- 不新增输出格式（TOON 等）；
- 不动 `status` 的领域分组语义。

## apply（批 A，2026-10-27）：JSON 契约

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/cli/cli.go` | `statusSummary`：`complete`/`in-progress` → `Done`/`Open`（`json:"done"/"open"`）；`worktreeState.Mode` → `Policy`；`worktreeListDoc.Mode` → `Backend`；`checkDoc.OK`/`Summary` → `Passed`/`Findings`；`showDoc` 增 `Stat`，`showJSON` 先 `Show` 再按需 `ShowStat`，`body` 不再被替换 |
| `internal/cli/cli_test.go` | 更新 status / check / worktree-list / show 的 JSON 断言；`show --stat` 现在断言 `body` 仍是提交文本、`stat` 是 diffstat |
| `docs/read-json.md` | 结论改成新键；新增「批 A 的键名修订」：5 个改名、`body` 纠错、四条成文规则（`id` vs `issue`、容器/省略、`title`/`commit` 粒度、JSON 不随语言变） |
| `docs/machine-readable-output.md` | 文末指向新契约；旧示例保留作过程记录 |
| `README.md` / `README.en.md` | `check` 示例改 `passed`/`findings`，并写明 `findings` 数诊断条数 |

### 验收（实测）

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过。
- 真仓：`status --json` → `{"total":62,"done":53,"open":9,"actions":276,"domains":3}`、`"worktree":{"policy":"off","open":0}`；`worktree list --json` → `"backend":"git"`；`check --json` → `"passed":true`、`"findings":{"ok":53,"warn":9,"error":0}`；`show cli-consistency 1 --stat --json` → `body` 为提交全文、`stat` 为 `docs/cli-consistency.md | 48 …`。
- `--lang=zh` / `en` 四份 JSON 逐字节相同（status / check / worktree list / show --stat）。
- 文本视图未动：`status` 仍「问题总数…（完成 53 / 进行中 9）」；`show --stat` 文本仍是提交头 + diffstat；golden 测试全绿。

### 没做什么

- **只写规则、不改形状**：C02（`id` vs `issue`）、C06（容器恒写）、C30（JSON 不随语言变）都只进文档——现形状本来就成立，改名只会给唯一消费方添麻烦。
- **`show.body` 的 diff 没拆**：实测 `body` 一直是 `git show` 全文（含 diff），与旧文档“不含 diff”矛盾；本批按实现改正文档、让 `body` 语义稳定，拆 `diff` 字段留作独立决定。
- **dsh-kiln 不在这条 issue 的范围**：用户决定自行迁移消费方，本批不跟。

### 下一批

批 B（解析器与帮助）：开关 flag 不再吞位置参数、`kiln <cmd> -h`、重复标量 flag 报错、多余位置参数报错、`worktree` 先 parse、`path` usage/去重。测试先行。

## apply（批 B，2026-10-27）：解析器与帮助

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/cli/cli.go` | `parse(args, switches...)`：开关不再吞后一个 token；新增 `rejectSwitchValues`（`--json=x` → exit 2）、`rejectRepeats`（标量 flag 重复 → exit 2）、`rejectExtra`（多余位置参数 → exit 2）、`dedupeStrings`；`usageText` 的 do/commit flag 段抽成 `doCommitFlagText`，新增 `commandUsageText` / `flagLinesFor` / `lineNamesCommand` 给 `<cmd> -h` 用；各命令声明自己的开关表并接入三类校验；`workspace` 先 parse 再取子命令；`path` 去重、批量判定按“用户给了几个 id”而非去重后的数量 |
| `internal/i18n/catalog.go` | 新增三条：`flag --%s 只能给一次…`、`多余的参数: %s`、`用法: %s`；`path` 的用法串改成多 id 形态 |
| `internal/cli/cli_test.go` | 新增 9 个测试（parse 开关、`show --json <id>`、显式值拒绝、子命令帮助、重复标量、多余参数、`worktree --json list`、`path` 去重、`commit --force` 前置） |
| `README.md` / `README.en.md` | 帮助一行加 `<cmd> -h` |

### 行为对照

| 输入 | 以前 | 现在 |
| --- | --- | --- |
| `kiln show --json cli-consistency` | exit 2，`用法: kiln show …`（id 被 `--json` 吞掉） | 正常输出 JSON；`show --stat <id>` 同理 |
| `kiln commit --force <issue> <action>` | 参数被吞，`用法` 错 | 正常提交（fake backend 测试覆盖） |
| `kiln worktree --json list` | exit 2，worktree 用法 | 正常输出 worktree JSON |
| `kiln list -h` / `show -h` / `commit -h` | 静默全量 / 当 issue 查 / 用法错 | 三者都是该命令的用法 + flags，exit 0 |
| `kiln list --search=a --search=b` | 静默只留 `b` | exit 2：`只能给一次（当前给了 2 次）` |
| `kiln list extra` / `status extra` / `show a 1 2` | 静默忽略 | exit 2：`多余的参数: …` + 用法 |
| `kiln path --json=x` | exit 2（`path` 独有的拦截） | exit 2（所有命令同一句 `flag --json 不接受值: x`） |
| `kiln path a a --json` | 两份重复文档 | 一份（保序去重），仍是批量形态 |
| `kiln path --all a --json` | `flag --all 不接受值: a` | `--all 与 issue 参数不能同用`（更接近真实错误） |

### 验收（实测）

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过。
- 真仓逐条冒烟上表九行：`show --json cli-consistency`（ordinal 4）、`show --stat` 前置、`list -h`、`help show`、`commit -h`、`worktree --json list`、`path` 重复 id（1 份）、`list --search` 重复（exit 2）、`list extra`（exit 2）、`graph --depth` 重复（exit 2）。
- 文本视图：除帮助（新增每命令帮助、`path` 用法行改成多 id 形态）外未动；`status` / `list` / `path` / `show` / `check` / `graph` 的 golden 全绿。
- 全量 `kiln -h` 仍是一份完整帮助，只有 `path` 行按 C20 改了形状。

### 与 hypothesize 的两处小偏差

- **`--tag` 的重复检查提前到本批**：`list` 是标量语义，重复本来就和 `--domain` 一样该拒；C 批只剩“含逗号”一条 → `--tag 只接受一个标签`。
- **开关值拒绝从 `path` 推广到所有命令**：`--json=x` 不再只有 `path` 会精确报错，统一成同一句。

### 下一批

批 C（过滤与空态）：`--tag` 含逗号 → exit 2、`--action` 按词表校验、空结果/空仓给 stderr 提示。

## apply（批 C，2026-10-27）：过滤语义与空态

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/cli/cli.go` | `list`：`--action` 按动作词表校验并统一小写（与 `do` 同一句错误）；`--tag` 含逗号 → exit 2；空结果按三种情形给 stderr 提示（search 命中 0 / 空仓 / 过滤条件），新增 `filterSummary` 列出实际生效的条件；`<cmd> -h` 的英文行过滤修正（`extra list flag:` 前缀、`… 的 flags` 后缀） |
| `internal/i18n/catalog.go` | 新增 `--tag 只接受一个标签…`、`没有匹配 %s 的问题`、`仓库里还没有任何动作…`；`没有匹配 %q 的 issue` 改为 `…的问题`；新增 `用法:\n`，修掉英文 `<cmd> -h` 回退中文的标题 |
| `internal/cli/cli_test.go` | 新增 5 个测试（逗号 tag、action 词表与小写归一、空结果条件回显、空仓提示、英文子命令帮助）；search 0 命中断言的措辞同步为“问题” |
| `README.md` / `README.en.md` | 过滤一行补 `--action` 校验、`--tag` 单标签、空结果提示 |

### 行为对照

| 输入 | 以前 | 现在 |
| --- | --- | --- |
| `kiln list --tag="json,path"` | exit 0、0 行、无提示（像“没有这个标签”） | exit 2：`--tag 只接受一个标签（当前: "json,path"）；多个标签暂不支持` |
| `kiln list --tag=json` | 1 行 | 不变（单标签仍可用） |
| `kiln list --action=nope` | exit 0、0 行 | exit 2：`非法 action: "nope"（可选: ask, …）` |
| `kiln list --action=ASK` | 0 行（大小写敏感） | 按 `ask` 过滤，`filter.action` 回显 `"ask"` |
| `kiln list --domain=nope` | exit 0、0 B / 0 B | exit 0、stdout 0 B、stderr `没有匹配 domain=nope 的问题` |
| `kiln list --domain=nope --status=done` | 同上静默 | stderr `没有匹配 domain=nope status=done 的问题` |
| 空仓 `kiln list` | exit 0、0 B / 0 B | stderr `仓库里还没有任何动作（用 kiln do <issue> ask 开一条）` |
| `kiln list --search=zzz` | `没有匹配 "zzz" 的 issue` | `没有匹配 "zzz" 的问题`（D 批术语规则提前用在这里） |
| `kiln --lang=en list -h` | 英文块的行用 `extra list flag:`，过滤不到，且标题是“用法:” | 完整英文：`usage:` + 四条 list flag 行 |

### 验收（实测）

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过。
- 真仓：`--tag=json,path` exit 2；`--tag=json` → 1 行（`path-action-tags`）；`--action=nope` exit 2；`--action=ASK` → 62 行、`filter.action="ask"`；`--domain=nope` → stderr 提示 + stdout 0 B；`--domain=nope --status=done` 提示带两个条件；`--search=zzzz` 提示“问题”；无过滤输出 3,516 B 未变。
- 空仓：`list` → 0 B stdout + 空仓提示；`list --json` → 49 B 文档 + 同一提示。
- 英文子命令帮助：`--lang=en list -h` 全英文且含 `--status/--tag/--search` 三条说明。

### 与 hypothesize 的偏差

- **`--search` 的 0 命中措辞在本批就改成“问题”**：否则同一条命令里会出现“问题”与“issue”并存；D 批只剩 graph 邻域、拒绝新建、worktree 关系、帮助说明等词条。
- **英文子命令帮助的过滤 bug 在本批修**：它是 B 批新功能的缺口，不修就等于 `--lang=en` 下 `<cmd> -h` 悄悄退回全量帮助。

### 下一批

批 D（中文术语统一）：把 prose 位置的 `issue` 改成「问题」，`issue` 只留标识符位置；另外把 graph 的 `没有 issue %q` 统一成 `未找到问题: %s`。

## apply（批 D，2026-10-27）：中文术语统一

### 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/cli/cli.go` | 帮助说明 `issue 不会被关闭` → `问题不会被关闭`；`--all 与 issue 参数` → `问题参数`；拒绝新建的 `issue %q … 同一条 issue` → `问题 %q … 同一条问题`；`没有 issue %q`（graph 邻域）改用现成的 `未找到问题: %s`；批量提示 `多条 issue` → `多条问题` |
| `internal/i18n/catalog.go` | 上述五处 zh key 同步；删除两个已无引用的条目（`没有 issue %q`、`没有 issue 为 %q 的 worktree`）；en 值不变 |
| `internal/cli/cli_test.go` | 新增 `TestChineseProseSaysWenTiNotIssue`：拒绝新建、graph 邻域、`--all` 冲突、帮助说明四处都断言用「问题」 |

规则：**「问题」是中文叙述里的实体；`issue` 只保留在标识符位置**——`issue:` trailer、`--issue`、JSON `issue` 键、`<issue>` 占位、`issue ID`、`kiln do <issue> <action>`。

### 验收（实测）

- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全过。
- 真仓：`kiln do cli-consistency ask` → `拒绝: 问题 "cli-consistency" 已经有起点动作 "ask"…同名即同一条问题…`；`graph --issue nope --json` → `未找到问题: nope`；`path --all <id> --json` → `--all 与问题参数不能同用`；`path a b`（无 `--json`）→ `…多条问题`；`-h` 的说明是“问题不会被关闭”。
- `--lang=en` 对应输出不变（`refused: issue …` 等），中英各自的措辞完整。
- zh 错误路径不再出现 `问题…issue…` 同句混用（新测试 + 人工抽查）。

## 结论（2026-10-27）

`cli-consistency` 收尾。36 条实测发现全部有落点：**16 条已改 / 3 条写明暂不改 / 17 条写明保持理由**，分四批落地：

| 批 | 提交 | 内容 |
| --- | --- | --- |
| A | `09b9045` | JSON 契约：`status done/open`、`worktree policy/backend`、`check passed/findings`、`show --stat` 追加 `stat`；四条成文规则（`id` vs `issue`、容器/省略、`title`/`commit` 粒度、JSON 不随语言变） |
| B | `986d49d` | 解析器与帮助：开关不再吞位置参数、`<cmd> -h`、重复标量/多余参数报错、`worktree` 先 parse、`path` 多 id 去重 |
| C | `adc307b` | 过滤与空态：`--action` 校验、`--tag` 拒逗号、空结果/空仓在 stderr 说明条件；修英文子命令帮助 |
| D | 本次 | 中文术语：prose 用「问题」，`issue` 只留标识符位置 |

**明确不做的**（都有理由，不是遗漏）：`check.actions` 加 ordinal（无消费方）；`graph` / `check` / `status` 的 `--domain` 过滤（新功能，另立项）；文本状态标记统一与 `list` 文本补 title（文本稳定优先）；多标签过滤与 `filter.tag` 形状变更（等需求）；单条错误的 JSON 化（归 `machine-readable-output` 遗留）；把 `show.body` 的 diff 拆出去（要新后端方法）。**dsh-kiln 的 `action.commit` 迁移由用户自行处理**，不在本 issue 范围。

最终状态：`kiln check` 53 ok / 9 warn / 0 error；七个读命令的 JSON 在同一仓库下 `--lang=zh` / `en` 逐字节相同；文本视图除帮助外未动。
