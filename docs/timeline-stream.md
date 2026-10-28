# issue: timeline-stream — 动作时间线的流式输出（扩 `path --all`，不新增顶层命令）

- **domain**：cli
- **关联**：relates-to `path-batch`（流式输出先例）；relates-to `perf-audit`（O(commits) 与首次输出）；relates-to `machine-readable-output`（JSON 契约）

## 背景

下游 `dsh-kiln` 的首屏 timeline 需要“最近动作”。先前想新增一个 `timeline` 命令，但这不该是默认答案：
先把现有命令面核一遍，确认到底缺的是“数据”还是“形态/时序”。

结论：**数据在 `path --all --json` 里已经全有，但没有一种现有形态能在首个命中提交后就输出**。
`path --all` 仍要 `engine.Build` 全史建模，按 issue id 输出、动作按时间正序、没有 limit；
`list` 只有 issue 行没有动作；`show` 一次只能看一条、无法枚举全局顺序。所以需要的是
**给 `path --all` 加一个流式动作模式**，而不是新造一个顶层命令。

## 现有命令核查（2026-10-28）

| 现有入口 | 能拿到全部动作？ | 能满足首行早/limit？ | 结论 |
| --- | --- | --- | --- |
| `path --all --json` | 是：issue + actions 全量 | 否：先全史 `engine.Build`，id 序、动作正序，无 limit；100k 35MB | 数据够，时序/流式不够 |
| `path <id> --json` | 单 issue | 否：要先知道 id，且逐条进程 | 详情可用，不能当全局流 |
| `list --json` / `--search` | 否：只有 issue 行/命中行 | 否 | 不能还原动作时间线 |
| `show <issue> <n|action>` | 单动作 | 否：需要先枚举 | 只能做详情 |
| `graph` / `status` / `check` | 否：无动作历史 | 否 | 不相关 |
| `git log` 直读 | 是 | 是 | 不是 kiln 命令；违反“数据只走 kiln --json” |

因此：**不新增 `kiln timeline` 命令**；在 `path` 上增加一个显式的流式时间线模式。
如果上游认为 `path` 的语义不适合承载平铺动作流，再单独讨论命令面——但默认先复用 `path`。

## 需求

1. **入口**：`kiln path --all --timeline --jsonl [--limit N]`。
   - `--timeline`：输出平铺动作流，而不是 `pathDoc` 列表；**只能与 `--all` 同用**；
   - `--jsonl`：NDJSON，一行一个动作对象，UTF-8，每行 `\n`；没有数组包装/legend/prose；
   - `--limit N`：最多 N 行后正常退出；只允许与 `--timeline` 同用；
   - 缺 `--all`、缺 `--jsonl`、`--timeline` 与 `--stat/--shortstat` 同用、`N<1/缺值/非整数` → exit 2。
2. **字段**（与 `path.actions[]` 同源）：
   ```json
   {"issue":"...","action":"...","commit":"...","date":"2026-10-28T...","title":"...","domain":"...","tags":["..."]}
   ```
   - `issue`：归一化 issue id；`action`：动作词表名；`commit`：完整提交 id；
   - `date`：**author date**，RFC3339，与 `path.actions[].date` 同源；
   - `title`：动作 subject；`domain`/`tags`：取自该提交 trailer，空则省略。
3. **过滤**：只输出同时带 `issue:` + `action:` 的提交；普通 chore 跳过；与 `path` 的动作一一对应，
   同一提交只出现一次，不能重复/漏。
4. **顺序**：author date 倒序。用 author-date 语义（不是 commit date）；同一时间保持 git 相对顺序。
   文档写明：这是**拓扑约束下的 author-date 倒序**，merge 多的仓不保证严格全局时间序。
5. **流式**：第一行必须在读到第一个命中提交后立即输出；不得等全仓聚合/全量排序/完整文档生成；
   100k 动作全程仍是一次 git 历史扫描，但首行与总量解耦。
6. **退出 / 中断**：成功 0；错误非 0；stdout 只有 NDJSON。消费者关闭 stdout 时 kiln 要及时退出
   （broken pipe 不挂住），不能继续跑完整扫描。
7. **jj 后端**：要么按 author timestamp 给出等价流；要么明确 exit 非 0 + stderr「path --all --timeline 暂不支持 jj」。
   **不能**静默空输出/exit 0。
8. **不变**：`path <id> --json`、`path a b --json`、`path --all --json` 的输出与文本视图**字节不变**；
   `--timeline` 只新增模式，不改变既有 `path` 行为。
9. **文档**：README、`kiln path -h`、`docs/machine-readable-output.md` 写清 `--timeline --jsonl`、
   字段、顺序、`--limit`、NDJSON、author-date 拓扑约束与 jj 行为；本轮不做文本 timeline 视图。

## 测试

- **现有入口对拍**：`path --all --json`（改前/改后）、`path <id> --json`、`path a b --json`
  逐字节不变；`--timeline` 不影响它们。
- **小 fixture**：混合 issue/action、普通 chore、空 domain/tags、相同 author date；断言行数/字段/跳过/顺序。
- **与 path 动作集合对拍**：同一小仓，`path --all --timeline --jsonl` 的行集合
  （`issue/action/commit/date/title`）与 `path --all --json` 的动作集合一致（排序可不同），每个动作恰好一次。
- **flag 矩阵**：`--timeline` 缺 `--all` / 缺 `--jsonl` / 与 `--stat` 同用 → exit 2；
  `--limit` 无 `--timeline` → exit 2；`--limit 0/负/缺值/非整数` → exit 2；`--limit 5` 恰好 5 行；
  `--limit` 大于总数输出全部。
- **流式首行**：100k 合成仓计时第一行；`--limit 50` 计时全程；不持有 35MB 文档/全仓模型。
- **中断**：读几行后关管道，kiln 及时退出、无挂起。
- **错误**：非 kiln 仓、空仓、坏 trailer、jj 不支持；退出码/stderr 正确。
- **回归**：`list` / `graph` 输出不变；`go test ./...`、`go vet`、`gofmt`。
- **真仓 dogfood**：本仓跑一次；与 `path --all` 的动作集合对拍。

## 验收

- **100k 合成仓**：冷启动第一行 ≤250ms（目标 ≤200ms）；`--limit 50` 全程 ≤500ms；
  不带 limit 全程 ≤2s；峰值内存 ≤64MB（不持整份文档/模型）。
- **10k / 64 条**：第一行分别 ≤200ms / ≤150ms；行数正确。
- 每行可独立 `JSON.parse`；字段、顺序、过滤、`--limit`、flag 矩阵、退出码符合。
- 消费者中途断开时 kiln ≤1s 退出；jj 行为明确。
- `path --all --json` / `path <id> --json` / `list` / `graph` 输出不变；`go test ./...` 全过；文档同步。
- 下游能只读前 N 行渲染首屏；不需要缓存/分页配合。

## 非目标

- **不新增顶层命令**（`timeline` 不做）；只扩 `path` 的显式模式；
- 不改 `path` / `list` / `graph` 的既有输出或语义；
- 不做 issue 列表分页 / offset / 全局严格 author-date 排序；
- 不做文本 timeline 视图（本轮只 `--jsonl`）；
- 不做 `--domain/--tag/--action` 过滤（消费方自己筛；需要再开 issue）；
- 不做缓存（消费方决定缓存首屏）；
- 不建索引/数据库/常驻进程。

## 备注

- **2026-10-28 修订**：初版 ask 要求新增 `kiln timeline` 命令；核查现有命令面后改为
  `path --all --timeline --jsonl`，不新增顶层命令。`--timeline` 是显式模式，避免改变既有
  `path --all --json` 的形状。
- 排序用 author date（与 `path` 一致），不是 commit date；后者会被 rebase/amend 改写。
- `--limit` 是消费方“只读首屏”的止损：够 N 行就停，不必为 35MB 付完。
- 与 `issue-list-scaling` 的“分页”不同：这是动作流，不是 issue 列表分页；但同样命中
  “消费方需要前 N 条”的触发条件。

## collect（2026-10-28）：timeline 与 `git log` / `path` 的关系，以及入口候选

> 本节只收事实与候选，不裁决入口，也不判必要性；这两件事留给下节 hypothesize。

### 1. timeline 的形状：像 `git log`，但单位是 action

| 维度 | `git log` | `path` | timeline（提案） |
| --- | --- | --- | --- |
| 范围 | 所有 commit | **一个 issue** 的动作 | **所有 issue** 的动作 |
| 单位 | commit | pathDoc（issue + actions） | 一条 action 事件 |
| 过滤 | 无 | 只留 issue + action | 只留 issue + action |
| 顺序 | commit-date 默认，可 `--author-date-order` | 单 issue 内时间正序；`--all` 按 id | 全局 author-date 倒序 |
| 输出 | 文本 / `--format` | 一份 JSON 文档 | NDJSON 流 |
| 关系 / stat | 无 | 有 | 无 |
| 首行 / limit | 原生支持 | 不支持（先建全史） | 要求首行早出 + `--limit` |

结论：timeline 更像“**只保留 kiln 动作、解析成语义字段、按 author-date 流式输出**的 `git log`”，不是原始提交日志。它和 `path` 只共享底层 git 历史，unit / grouping / order / output / timing 都不是同一个视图。

### 2. 既有命令面里“log”的生态位

| 文档 | 事实 | 对 timeline 的含义 |
| --- | --- | --- |
| [action-diff](action-diff.md#L101) | 命令面按基数对齐：`path` ↔ `git log`（一个 issue 的动作串），`show` ↔ `git show`；`path --stat` ↔ `git log --stat` | kiln 早已把“log 的生态位”给了 `path`，只是范围是单 issue |
| [evidence-fluidity](evidence-fluidity.md#L190) | 讨论过 `kiln log --rewrites`，最终[明确不做](evidence-fluidity.md#L226)：改写史溯源回 jj `op log` / git `reflog` | 被否的是“改写史视图”，不是“全局动作流”；`log` 这个名字会撞上这条已否的提案 |
| [cli-necessity](cli-necessity.md#L151) | `path <issue>` 相当于 `jj log -r "issue(...)"` 的一个特例；`path` / `show` 属于“git 的 UX 包装”区 | 全局 log 类视图要过 `cli-necessity` 的必要性尺子，不能只靠“git log 也能看” |
| `mvp-plan` | 命令面只有 `do` / `commit` / `path` / `status` / `check` / `list` / `graph`，没有 `log` | 没有“通用 log 命令”的历史决策可复用；timeline 是新面 |

所以“kiln 讨论过要不要做 log 命令”更准确的说法是：**讨论过 `kiln log --rewrites` 并否决了改写史视图**；通用 `log` 从未立项。timeline 若叫 `log`，会同时继承“为什么不用 git log”和“改写史”两重误解；`timeline` 这个名字更贴近“语义动作流”。

### 3. `path` 是否适合承载

- `path` 的原始单位是一个 issue 的 pathDoc：标题/状态、编号动作、关系；文本视图也是“一个 issue 头 + 编号步骤 + 关系”（[cli.go](../internal/cli/cli.go#L1365)）。
- `path --all --json` 是后来加的**批量包装**：`{"issues":[pathDoc…]}`，仍是一份文档装 N 个 issue 的 path，按 id 排（[path-batch](path-batch.md#L60)）。它的 `--all` 是“所有 issue 的 path”，不是“所有动作的平铺”。
- timeline 的 `issue / action / commit / date / title / domain / tags` 是**动作事件**字段，没有 `ordinal` / `relations` / `stat`；顺序是全局 author-date，不是 id。两者只在“底层数据来自同一份 git 历史”这一点上共享。

`path --all --timeline` 的读法是“所有 issue 的 path，但不是 path”，语义上确实别扭。

### 4. 入口候选

| 方案 | 入口 | 视图显式？ | 编码显式？ | 已知代价 |
| --- | --- | --- | --- | --- |
| A | `path --all --timeline --jsonl` | 是 | 是 | 三个 flag；`--all` 在 timeline 下恒真；`path` 出现第二种形状 |
| B | `path --all --jsonl` | **否** | 是 | `--jsonl` 同时改形状与编码；`path <id> --jsonl` 语义不明 |
| A' | `path --timeline --jsonl` | 是 | 是 | 少一个恒真 flag；仍有 `path` 第二种形状 |
| C | `path --timeline` | 是 | 否（隐含 NDJSON） | flag 最少；未来文本 timeline 会改默认 |
| D | `timeline --jsonl [--limit N]` | 是（命令名） | 是 | 多一个顶层命令 |

契约侧的事实：

- [machine-readable-output 规则 6](machine-readable-output.md#L199) 要求“同一命令的 JSON 形状不随 flag 变”；B 的 `--jsonl` 直接改 unit，A/A' 也仍然是 `path` 的第二种形状。
- [规则 8](machine-readable-output.md#L201) 要求“stdout 只放一个 JSON 文档”；timeline 的 NDJSON 必然要写例外，区别只是例外落在 `path` 还是新命令上。
- [path-batch](path-batch.md#L78) 已把“批量没改 schema，只是外面套了一层”写成契约；timeline 不是这种“套一层”。

### 5. 本机方向性测量（2026-10-28）

| 项 | 值 |
| --- | --- |
| 当前动作数 | 317 |
| `path --all --json` | 150,478 bytes，单次约 188 ms |
| `git log --author-date-order --format=…` 首条 | 约 130 ms（冷启动一次） |
| 同命令全量 | 约 61 ms（热缓存） |

这些只说明量级：当前 317 条动作下，`path --all` 仍是“先建全史再一次输出”；timeline 要成立，必须新增流式后端（现有 `vcs.Repo.Log()` 返回 `[]Commit`，是全量接口）。100k 目标需要自己的合成仓与计时，不能用上表外推。

实现侧已确认的事实：

- `vcs.Repo` 的 `Log()` 是全量 `[]Commit`；`StatLog` 已经开过“可选接口”的先例（`LogStats`），timeline 可照此加一个可选的流式 log 接口，jj 不实现就按需求 7 明确报“不支持”。
- `path` 现在先 `engine.Build` 全史；timeline 必须绕过 `loadRepo()`，否则首行不可能早出。

### 6. 留给 hypothesize 的问题

1. timeline 是 `cli-necessity` 里的“新综合视图”（像 `status` / `graph` / `path`），还是“诚实的 UX 包装”（像 `show` / `path --stat`）？
2. 如果是包装，它是否满足“明显更好”的立项条件（首行、limit、稳定 NDJSON）？
3. 如果成立，入口选 A / A' / C / D 中的哪一个；B 是否应直接排除？
4. 字段与边界：`title` 到底取 subject 还是 trailer title；`domain` 是 per-commit 还是 issue 级；NDJSON 如何写进既有“一份文档”契约；broken pipe / 空仓的退出码。

## hypothesize（2026-10-28）：timeline 过不了严格必要性线，只能按“诚实包装”立项

> 结论先行：按 `cli-necessity` 的四件事逐条量，timeline **不新增模型、校验、规范化写**；“综合”这一项也只是把 `path --all` 已有的动作再投影成全局流，而 `git log --author-date-order` 原生就能流式 + `-n`。所以它**过不了“git 表达不了”的严格必要性线**。它能过的是 `show` / `path --stat` 那条“诚实包装”线：承认可被 git + 约定替代，只在“明显更好”时立项。若按包装立项，入口优先独立 `timeline`（不冻结命令面时）；若要求核心必要性，则不做。

### 1. 尺子：`cli-necessity` 要求什么

该 issue 的[结论](cli-necessity.md#L904)把必要性收在四件事：**模型、校验、综合、规范化写**；`path` 因为“按 issue 重建有序路径”被列为必要，`show` / `path --stat` 则被明确归为**诚实的包装**——“有价值，但可被 git 直接替代”，只能靠更好用来赢（[6 条观察](cli-necessity.md#L103)）。它还要求：读与单提交展示应与后端重合、甚至让给后端，不必假装 CLI 更权威。

因此“能不能过”要分开问：

- **严格必要性**：timeline 是否提供了 git + 约定表达不了的东西？
- **包装立项**：如果不能，它是否至少是“明显更好”的包装，并且不冒充权威？

### 2. 逐条体检

| 尺子 | timeline 的增量 | 判断 |
| --- | --- | --- |
| 模型 | 无新实体/字段；`issue/action/date/domain/tags` 全部来自现有 `Action` 与 trailer | 否 |
| 校验 | 无新规则；不参与 `check` | 否 |
| 综合 | 把 `path --all` 已有的动作按 author-date 展平/重排；没有新增 `path` 的 ordinal/relations、`status` 的 domain 分组、`graph` 的边 | **弱/否** |
| 规范化写 | 只读 | 否 |

唯一新增的是**交付形态**：全局 author-date 顺序、首行早出、`--limit`、稳定 NDJSON。这些是时序/契约，不是模型语义。

对比一下“必要”的邻居为什么站得住：

- `path`：新增了 **issue → 有序动作序列**的重建，带 ordinal、progress、relations；git 给不出。
- `status` / `graph`：新增 **domain 分组 / 关系边** 的综合；git 给不出。
- `show` / `path --stat`：只是单提交展示，`cli-necessity` 已判为包装。
- timeline：没有新增上述任何一种结构；它回答的“全局最近动作”可以从 `path --all --json` 展平得到，而 `git log --author-date-order` 原生就能流式 + limit。

### 3. 反方：它也可以被算作“综合视图”

最强的反方理由是：**“全局动作最近发生了什么”本身是一个模型问题**，只有 kiln 知道可配置的 action 词表与 `issue + action` 的准入规则（写侧归一化也只发生在 kiln）；`path --all` 虽然包含数据，但不是这个问题的直接视图。插件的首屏成本也是真的：当前 317 动作 `path --all --json` 已 150 KB / 188 ms，100k 时约 35–47 MB，不能要求消费方先吃完整文档。

但这条理由落到 `cli-necessity` 的尺子上仍然不够：

1. **数据不是新的，只是投影**：`path --all` 已包含全部动作；展平 + 排序是消费方几行代码的事。`status`/`graph`/`path` 的必要性来自“重建结构”，不是“换个排法”。
2. **流式 + limit 是 git 的原生能力**：`git log --author-date-order --format=… -n N` 已经能首行早出、按 N 停；kiln 的增量只是“按词表过滤 + 解析 trailer + 稳定字段”。这是**约定层**的工作，不是模型层。
3. **如果任何再投影都算综合**，那每个输出格式都能自称“必要视图”，这条尺子就失效了。

所以：**“全局最近动作”是一个合理的问题，但 timeline 只是其中一种交付方式，不是唯一能回答它的视图；它主要是一个把已有模型数据送到消费方眼前的交付层。**

### 4. 裁决

- **严格必要性：不过。** timeline 不新增模型 / 校验 / 规范化写；综合项也只是再投影。按 `cli-necessity` 的结论，它应归入 `show` / `path --stat` 的“诚实包装”，而不是 `path` / `status` / `graph` 的“必要视图”。
- **诚实包装：条件性通过。** 满足以下四条才立项：
  1. **明显更好**：对目标消费者，首行/`--limit`/内存显著优于 `git log` + 脚本和 `path --all --json`；不是“也能用”。
  2. **同源**：字段全部由同一份 trailer 解析与配置词表产生，不引入第二套语义或第二份真相。
  3. **不冒充权威**：文档写明它是流式读视图，不是审计/完整历史契约；不改写、不校验。
  4. **契约完整**：NDJSON、字段、顺序、`--limit`、jj 不支持、broken pipe、空仓的退出码都写死。
- **如果包装这条也不接受**：不做 timeline；插件用 `git log` 或 `path --all --json` 自行展平。这是与“核心只保留四件事”最一致的选择。

### 5. 对入口的含义

既然它是包装，就**不能用“必要性”为新命令辩护**；入口应由契约清晰度和 UX 决定：

- **D（`timeline --jsonl [--limit N]`）**：包装命令，名字直接说视图；NDJSON 例外只属于它；不需要 `--all`；`path` 零改动。代价是多一个命令。
- **A'（`path --timeline --jsonl`）**：不扩命令面时的次优；`--all` 应去掉（timeline 天生全局）；仍然让 `path` 出现第二种形状。
- **A（`path --all --timeline --jsonl`）**：可接受但啰嗦；`--all` 是恒真 flag。
- **B（`path --all --jsonl`）**：应排除。`--jsonl` 同时改形状与编码，撞[规则 6](machine-readable-output.md#L199)（schema 不随 flag 变）和[规则 8](machine-readable-output.md#L201)（stdout 一个文档）；`path <id> --jsonl` 也无定义。
- **C（`path --timeline`，隐含 NDJSON）**：flag 最少，但未来加文本视图会改默认，不如显式 `--jsonl`。

说明：A / A' 也不是零成本——它们同样让 `path` 出现第二种形状，严格说也碰规则 6 的字面；区别是 `--timeline` 明说“换视图”，而 B 让格式 flag 悄悄承担视图变化。D 是唯一不碰既有命令契约的。

**建议**：若接受“诚实包装”，用 **D**；若命令面冻结，用 **A'**；不要 B。若连包装都不接受，则**不要做 timeline**——此时 A' 也会与“核心只保留四件事”的立场自相矛盾。

### 6. 证伪条件与检验

1. **包装可替代性**：写一个 `git log --author-date-order` + trailer 解析的原型。若它以可接受的代码量就能达到首行 / `--limit` / 内存目标，说明 timeline 的增量主要是包装而非能力；此时只有“明显更省事”才立项，否则不做。
2. **`path --all --json` 可承受**：10k / 100k 动作下若消费方能接受先读完整文档（或分块），“流式”的增量消失，退回包装判断。
3. **出现新语义**：若要让每行都有 issue 级 `domain`（而非 per-commit）、或加校验/过滤语义，timeline 就不再是纯投影；那要重新按“综合视图”审必要性，而不是套用本结论。
4. **首行目标不可达**：若流式后端 + `--limit` 仍达不到 250 ms / 500 ms，包装连自己的承诺都不成立。
5. **NDJSON 契约收不了口**：若“一份文档”规则无法干净地写成“timeline 例外”，契约成本高于收益，不做。

### 7. 下一步

- 先把本结论作为**分类**而不是**否决**：timeline 是包装，不是核心视图。
- 若接受包装：入口按 D；先补 `test` 原型（`git log --author-date-order` 流式 + trailer 过滤 + `-n` 首行计时），再决定是否 `apply`。
- `apply` 前必须定的契约：`title` 取 subject 还是 trailer title；`domain` 的 per-commit 语义；NDJSON 与既有“一份文档”规则的关系；broken pipe / 空仓退出码；100k 合成仓的计时口径。
- 若最终不做：把“`git log` 直读”或“`path --all --json` 展平”写进 issue 的替代方案，不留下一个半成品命令面。

## test（2026-10-28）：`git log` 原型能否替代新命令

### 方法

- 合成仓用 `git fast-import`，**1 提交 = 1 issue = 1 action**（空提交，`issue: issue-N` + `action: apply`），
  规模 1k / 100k；author/committer date 每提交 +60s；
- 原型约 60 行 Node：`spawn git log` + 逐记录解析（0x1e 分隔），格式只用需要的字段：
  ```text
  git log [--author-date-order] --format='%H%x1f%aI%x1f%s%x1f%(trailers:key=issue,valueonly)%x1f%(trailers:key=action,valueonly)%x1f%(trailers:key=domain,valueonly)%x1f%(trailers:key=tags,valueonly,separator=%x2C)%x1e'
  ```
- 过滤 issue+action 非空；测第一行、第 50 行、全量；`-n 50` 与“读够 50 行 kill”都试；
- 对照 `kiln path --all --json` 的第一块 stdout/总耗时/字节；
- 另测 `git commit-graph write --reachable` 之后同一条命令（commit-graph 是仓库侧性能前置，不是插件可写的东西）。

### 结果（本机，Windows）

| 场景 | 首行 | 第 50 行 | 全量 | 备注 |
| --- | --- | --- | --- | --- |
| 1k，author-date | 78 ms | 78 ms | 91 ms | 357 条动作的真相在此规模无差别 |
| 100k，default 顺序 | 64 ms | 64 ms | 1,475 ms | 快，但顺序是 commit-date/topology |
| 100k，author-date，无 commit-graph | 359 ms | 360 ms | 1,347 ms | 首行 >250ms 目标 |
| 100k，author-date，有 commit-graph | **64 ms** | 66 ms | 1,681 ms | 首行达标；全量略慢 |
| 100k，author-date + `-n 50`，无 commit-graph | 363 ms | 363 ms | 366 ms | git 仍要先走完约 360ms |
| 100k，author-date + `-n 50`，有 commit-graph | **66 ms** | 66 ms | 70 ms | 提前停止有效 |
| 100k，`kiln path --all --json` | 954 ms | — | 1,117 ms / 35.6 MB | 先 `engine.Build`，首块晚 |

（首行/第 50 行相同，是因为第一次 data chunk 已经含多行；`-n 50` 才真正让 git 提前收尾。）

### 结论

1. **git 命令可以实现**：约 60 行解析 + 一条 `git log --format`，首行/limit/行数都对；
   **没有新增 kiln 命令的必要**。
2. **author-date 首行是否达标取决于 commit-graph**：没有它，100k 首行 ~359ms（仍比
   `path --all` 的 954ms 快 2.6×）；有它 64ms（15×）。commit-graph 是仓库侧前置，
   插件只读、不能替用户写。
3. **default 顺序首行稳定 64ms**，但顺序是 commit-date/topology；`%aI` 仍是 author date，
   所以“值”对、“序”可能和 `path` 的 author-date 排序不同（merge/rebase 才明显）。
4. **全量并不更快**：git 全量 1.35–1.68s，`path --all` 1.12s；流式赢在首行，不在总吞吐。
5. **`-n 50` / 读够 kill 都可行**；没有 commit-graph 时 `-n 50` 仍要 ~360ms，因为 git 首块迟。
6. **插件直读 git 是另一条决定**：它会复制 kiln 的 trailer 过滤/归一化语义，并违反
   dsh-kiln 的“数据只走 kiln --json”约定；本 test 只证明数据路径可行，不裁决插件架构。
7. 因此：**不做 timeline 命令**是对的；若仍要“首行 ≤250ms + author-date 稳定”，
   要么要求仓库有 commit-graph，要么接受 ~360ms，要么退回 default 顺序并写明排序口径。

## hypothesize（2026-10-28）：多客户端改变必要性——timeline 作为共享只读传输契约

> 结论先行：单消费者下前一条 test 的结论不变（不需要命令）；但**如果未来会出现独立的
> kiln-webui / kiln-tui，timeline 的必要性从“模型”转到“接口”**：与其让每个前端各写一份
> `git log` + trailer 解析，不如做一条只读的 `kiln timeline --jsonl`，作为多客户端共享契约。
> 入口选 **D（独立 timeline 命令）**，不选 A/A'（把第二种形状塞进 `path`），B 仍排除。
> 这不是 conclude：是否接受“包装命令”、commit-graph/排序口径、jj 与 NDJSON 例外，留给上游裁决。

### 1. 条件变化：从 1 个消费者到 N 个

当前只有 dsh-kiln；test 已证明它可以直接 `git log`。但计划中的 webui/tui 会带来：

- 2 个以上独立前端（插件退场前后还会并存）；
- 不同语言/进程：浏览器端不能 spawn，必须有宿主/服务端；TUI 可能是 Go；
- 每个前端都要重复：`issue+action` 识别、issue 归一化、action 词表、domain/tags、
  author date、jj 差异、limit、broken pipe。

这正是“一份实现 vs N 份实现”的问题，和单客户端时“git 也能做”不是同一个问题。

### 2. 主张：做 D，`kiln timeline --jsonl [--limit N]`

把 timeline 定义为**多客户端的只读传输契约**，不是新的核心模型视图：

- 命令名直接说视图；NDJSON 例外只属于它；
- `path` 零改动，不出现“第二种形状”；
- webui 后端、TUI、dsh-kiln 都消费同一份行格式；
- 识别/归一化/词表/jj/broken pipe 只在 kiln 内实现一次；
- 性能与直读 git 同级（内部就是流式 `git log`），但契约稳定、测试集中。

### 3. 为什么不是“每个客户端直读 git”

| 成本 | 每个客户端直读 git | 一个 timeline 命令 |
| --- | --- | --- |
| 识别/归一化 | 每个客户端各写一份，容易漂移 | kiln 一份 |
| jj | 每个客户端各处理或各自声明仅 git | kiln 统一（支持或明确报错） |
| 字段/顺序 | 各写各的解析 | 一份 NDJSON 契约 |
| 测试 | N 套 | 一套 |
| 首行 | 64/360ms（取决于 commit-graph） | 同级 |
| 新增面 | 无命令 | 多一个包装命令 |

多客户端下，“多一个包装命令”比“N 份 trailer 解析”便宜。

### 4. 入口比较

| 方案 | 入口 | 多客户端视角 |
| --- | --- | --- |
| **D** | `timeline --jsonl [--limit N]` | **推荐**：名字/契约/例外都独立，`path` 不动 |
| A' | `path --timeline --jsonl` | 可行但 `path` 出现第二种单位；未来契约更模糊 |
| A | `path --all --timeline --jsonl` | `--all` 恒真，啰嗦 |
| C | `path --timeline`（隐含 NDJSON） | 编码不显式，未来文本视图会改默认 |
| B | `path --all --jsonl` | 排除：`--jsonl` 同时改形状与编码，撞既有 JSON 规则 |

上游的 hypothesize 已说明：D 是唯一不碰既有命令契约的入口；单客户端时它显得多余，
多客户端时它反而最合理。

### 5. 验收口径按 test 修正

test 的实测（100k）：

- 有 commit-graph：author-date 首行 **64ms**、`--limit 50` **70ms**、全量 1.68s；
- 无 commit-graph：首行 **359ms**、`--limit 50` 366ms、全量 1.35s；
- `path --all`：首行 954ms、全量 1.12s / 35.6MB；
- default 顺序：首行 64ms，但序是 commit-date/topology。

因此不要把“首行 ≤250ms”写成无条件验收。建议分档：

- 有 commit-graph：首行 ≤250ms、`--limit 50` ≤300ms；
- 无 commit-graph：首行 ≤400ms（仍 2.6× 优于 `path --all`）；
- 全量 ≤2s；不承诺总吞吐优于 `path --all`；
- 排序默认 author-date；default 顺序作为可选/降级需显式写清。

### 6. 契约要点（待上游 conclude 定）

- NDJSON 与“stdout 一份 JSON 文档”规则的关系：作为 timeline 的显式例外，还是另立规则；
- 字段：`title` 取 subject；`domain` 是 per-commit 还是 issue 级；`tags` 是否带；
- `--limit`：是否必填/可选；`-n` 与过滤的语义（N 条动作而非 N 个提交）；
- jj：等价支持，还是明确非 0「暂不支持」；
- broken pipe / 空仓 / 非仓的退出码；
- commit-graph 缺失时的文档承诺（≤400ms 还是提示用户生成）；
- 该命令是否只读、是否承诺长期稳定（多客户端契约意味着不能随意改字段）。

### 7. 反方 / 风险

- **YAGNI**：如果最终只做一个 Go TUI 且直接 import kiln 内部包，命令可以不做；
- **仍是包装**：它不新增模型/校验，按 `cli-necessity` 属于“诚实包装”；多客户端只改变
  “明显更好”的判断，不改变分类；
- **承诺成本**：一旦 webui/tui 依赖它，字段/顺序/NDJSON 就不能随便改；
- **commit-graph 不可控**：插件只读，不能替用户生成；无 graph 的首行 360ms 要接受或写进文档；
- **jj**：若只支持 git，要在契约里明确，不能静默空输出。

### 8. 留给上游 conclude 的问题

1. 是否接受 D（独立 `timeline --jsonl`）作为多客户端只读契约？如果不接受，是否明确
   “各客户端直读 git、kiln 不提供”？
2. 验收是否按 commit-graph 分档（≤250ms / ≤400ms）？
3. NDJSON 如何写进既有“一份文档”规则；`--limit`/jj/broken pipe 的最终语义；
4. 是否现在做，还是等 webui/tui 真正开工再做（YAGNI 的裁决）。

## hypothesize（修订，2026-10-28）：缺的是 action 索引，不是“git log 皮肤”——timeline 按综合视图成立

> 结论先行：上一节（`5c84242`）说“timeline 过不了严格必要性线”，那只适用于**“git log 的格式化包装”**。把 timeline 放回模型的主语——**全局 action 索引**——它通过 `cli-necessity` 的“综合视图”这一关。test 证明的是**交付可行**，不是“视图已经存在”；多客户端（`d8447a2`）是第二条理由，不是必要性的第一来源。流式 / NDJSON / `--limit` 仍是交付层，按 test 的分档验收。

### 1. 触发点：action 是模型原子，但命令面只有 issue 索引

| 问题 | 现有命令 | 状态 |
| --- | --- | --- |
| 有哪些 issue？ | `list` | 有 |
| 一个 issue 的完整路径？ | `path <issue>` | 有 |
| 一个动作的详情？ | `show <issue> n` | 有 |
| 全部 issue 按 domain / 关系？ | `status` / `graph` | 有 |
| **全部 action 按全局时间？** | **没有** | **缺** |

`list` 是 issue 的索引，`path` 是 issue 的详情，`show` 是动作的详情——唯独没有**动作的索引**。如果 action 是模型的最小单位，这个缺口不是“少一个排序”，而是少了一个与 `list` 对称的视图。

准确说：所有 kiln 动作都是一个提交，但不是所有提交都是动作（还有 chore / merge / 普通提交）。timeline 的价值之一就是把非动作提交过滤掉，只留模型里的 action。

### 2. 三个相邻结论各自的边界

| 结论 | 它证明了什么 | 它没证明什么 |
| --- | --- | --- |
| `5c84242`：“严格必要性不过” | 若 timeline 只是 `git log` 的格式化包装，确实可被 git + 约定替代 | 不能推出“全局 action 视图不必要”；它用“数据已在 `path --all`”否定了视图，标准用错了 |
| `58f13b3` test：“git log 原型可行，无需新命令” | 单消费者可以在约 60 行内实现交付；首行/limit 可行 | 不证明视图已经存在；也不证明每个消费者都该自己实现 |
| `d8447a2`：“多客户端使共享契约必要” | N 个前端会重复识别/归一化/词表/jj/契约，D 更划算 | 必要性不只来自客户端数量；单客户端时视图缺口仍在 |

`5c84242` 的推理错在把**数据可得性**当成**视图必要性**。这个标准如果成立，会连坐：

- `status` 可以从 `path --all` 按 `domain` 分组算出来；
- `graph` 可以从关系声明自己折；
- `path` 可以从 `git log` 加 trailer 解析重建。

如果“消费方能自己算”就判定不必要，那 `status` / `graph` / `path` 都不必要。它们之所以必要，是因为**组织原则本身是产品**：按 domain 看、按关系看、按 issue 路径看，各自回答不同问题。

timeline 的组织原则是“**按时间看模型的原子**”，这是现有视图都没有的：

- `path`：单 issue 内的时间序，没有跨 issue 的全局序；
- `list`：有 issue，但只有末条动作，中间动作不出现；
- `status` / `graph`：根本没有时间轴；
- `git log`：有全局序，但单位是 commit，不是 action，也没有 issue/action/domain/tags 语义。

### 3. 裁决：view 必要，delivery 是包装

| 主张 | 判定 |
| --- | --- |
| timeline 作为“全局 action 索引 / 模型日志” | **通过** `cli-necessity` 的“综合”关 |
| timeline 作为“git log 的格式化包装” | 不通过；那只是 UX |
| 流式 / NDJSON / `--limit` | 交付层；不构成必要性，按 test 分档验收 |
| timeline 作为模型 / 校验 / 规范化写 | 不新增；本就不靠这三项 |

关键区分：**view 是必要的，delivery 是包装。** 即使完全不流式，一个“平铺 action 列表”的 JSON 视图仍然是缺失的；流式只是让这个视图满足插件首屏。反过来说，只做流式而不输出模型字段，那才是真正的 `git log` 包装。

### 4. 入口含义

- **独立 `timeline` 命令**：它是 **action 级的 `list`**，补的是模型视图的对称性，不是给 `path` 加排序开关。入口不带 `--all`（timeline 天生全局），`--jsonl` / `--limit` 属交付层。
- **`path --timeline`**：仍不合适。`path` 是 issue 级视图，把全局 action 流塞进去，等于让“issue 路径”兼职“模型日志”。
- **B（`path --all --jsonl`）**：仍排除。`--jsonl` 同时改形状与编码，语义最隐蔽。
- **C（`path --timeline` 隐含 NDJSON）**：不优先；视图和编码分开更清楚。
- 名字：`timeline` 可以；若叫 `log`，必须一开始就定义成“模型日志”，否则会继续和 `git log` / 已否的 `log --rewrites` 混。

### 5. 和 test / 多客户端的关系

- **test 的交付结论接受**：有 commit-graph 首行 64 ms、无 graph 359 ms；`--limit 50` 70/366 ms；不要把“≤250 ms”写成无条件验收。这些数字说明**交付可行**，正好把“必要但不值得做”的问题变成“视图已经缺、交付也不难”的问题。
- **多客户端是加强项**：它把“一份实现 vs N 份实现”的成本摆出来，进一步支持 D；但即使只有 dsh-kiln，action 索引缺口仍在。不要把必要性全押在 webui/tui 一定会做上。
- **YAGNI 仍然有效**：视图缺口不等于“现在必须做命令”。若 webui/tui 还没开工，可以只记录结论、等触发条件；但那是**时机**判断，不是“视图不必要”。

### 6. 证伪条件

1. **不是视图，只是换排法**：如果“全局时间序的 action”不是用户/插件真正要问的问题，`path --all` 的 issue 分组已够用，那它只是投影，退回包装判断。
2. **git log 足够**：如果目标消费者用 `git log --author-date-order` + trailer 过滤就能在可接受代码量内得到同样答案，且不需要稳定字段/契约，timeline 的模型价值不成立。
3. **与 `list` 不对称是伪需求**：如果真实需求是“最近 issue”，正确落点是 `list --recent`（按末条动作日期），不是 action 流；这种情况下 timeline 解决的是伪问题。
4. **输出不是模型**：如果实现只透传 `commit/date/message`，没有 `issue/action/domain/tags` 与动作过滤，那它确实是 git log 包装，应否决。
5. **交付承诺不成立**：若流式首行 / `--limit` / 内存达不到 test 的分档目标，视图可以保留，但“对插件明显更好”的交付理由不成立，需重新评估是否值得做命令。

### 7. 留给上游 conclude 的问题（在 `d8447a2` 四问上补两条）

1. 是否接受分类修正：**timeline 按“全局 action 索引”通过综合视图关；只有“git log 包装”版本不通过**？
2. 是否接受 **D（`timeline --jsonl [--limit N]`）** 作为入口，且不带 `--all`？
3. 是否接受 test 的 commit-graph 分档验收（有 graph ≤250ms / 无 graph ≤400ms），而不是无条件 250ms？
4. NDJSON 与“一份文档”规则、`--limit`/jj/broken pipe、字段（`title` 取 subject；`domain` 的 per-commit 语义）如何定？
5. 现在做，还是等 webui/tui 真正开工再做（YAGNI 的时机裁决）？
6. 若不做命令：是接受“各客户端直读 git + 各自的 trailer 解析”，还是接受“消费方自己展平 `path --all --json`”？这两条都要明确写，不能留空。
