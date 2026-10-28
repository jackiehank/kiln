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
