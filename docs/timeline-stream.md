# issue: timeline-stream — 逐条流式输出 timeline（NDJSON、author-date 倒序、首行不等全仓）

- **domain**：cli
- **关联**：relates-to `path-batch`（流式输出先例）；relates-to `perf-audit`（O(commits) 与首次输出）；relates-to `machine-readable-output`（JSON 契约）

## 背景

下游 `dsh-kiln` 的首屏 timeline 需要“最近动作”。现在只能 `path` 按 issue 取，或者 `path --all` 取全量；
两者都要先 `engine.Build` 扫完整段历史，100k 动作时首字节要等 1–2s。`path --all` 的流式只流 JSON 编码，
不流 git 扫描，所以首屏没有改善。

需要一条新的读命令：把**动作流**按 author date 倒序逐条发出，第一行在读到第一个命中提交后即可出现；
消费方可以边收边渲染，也可以读够 N 行就断开。它是 timeline 专用，不是 issue 列表分页。

## 需求

1. **命令**：`kiln timeline --jsonl`。stdout 是 NDJSON：一行一个动作对象，UTF-8，每行以 `\n` 结束；
   没有数组包装、没有 legend/prose；报错只走 stderr。
2. **字段**（与 `path` 的动作字段同源）：
   ```json
   {"issue":"...","action":"...","commit":"...","date":"2026-10-28T...","title":"...","domain":"...","tags":["..."]}
   ```
   - `issue`：归一化后的 issue id（与 `path` 一致）；
   - `action`：动作词表里的动作名（与 `path` 一致）；
   - `commit`：完整提交 id（git 40/64 位；jj 用其 commit id）；
   - `date`：**author date**，RFC3339（与 `path.actions[].date` 同格式/同源）；
   - `title`：该动作的 subject（不是整段 body）；
   - `domain` / `tags`：取自该提交自己的 trailer；空则省略（沿用“缺省即省略”）。
3. **过滤**：只输出**同时带 `issue:` + `action:`** 的提交；普通 chore 提交跳过；
   与 `path` 的动作一一对应——同一个提交只出现一次，不能重复/漏。
4. **顺序**：author date 倒序。必须用 git 的 author-date 顺序（`--author-date-order` 语义），
   不是 commit date；同一时间保持 git 给出的相对顺序。文档要写明：它是**拓扑约束下的 author-date 倒序**，
   merge 多的仓不保证严格全局时间序。
5. **流式**：第一行必须在读到**第一个命中提交**后立即输出；不得等全仓聚合、全量排序或完整文档生成。
   后续逐条输出。100k 动作的全程仍是一次 git 历史扫描，但首行与总量解耦。
6. **`--limit N`（要求支持）**：最多输出 N 行后正常退出（exit 0）；`N < 1` / 缺值 / 非整数拒绝（exit 2）；
   没有 `--limit` 时输出全部。实现可以在够 N 行后提前停止扫描/关闭 git。
7. **退出 / 中断**：成功 0；任何错误非 0；stdout 只有 NDJSON。消费者中途关闭 stdout 时，
   kiln 要在合理时间内退出（broken pipe 不挂住），不能继续跑完整扫描。
8. **jj 后端**：要么按 author timestamp 给出等价流；要么明确 exit 非 0 + stderr「timeline --jsonl 暂不支持 jj」。
   **不能**静默空输出/exit 0。
9. **不变**：`path` / `list` / `graph` / `status` / `check` / `show` 的 JSON 与文本输出字节不变；
   新命令独立，不改既有契约。
10. **文档**：README、`kiln -h` / `kiln timeline -h`、`docs/machine-readable-output.md` 写清
    字段、顺序、`--limit`、NDJSON（一行一对象）、author-date 的拓扑约束与 jj 行为。
    本轮只承诺 `--jsonl`；文本 timeline 视图不做。

## 测试

- **小 fixture**：混合 issue/action 提交、普通 chore 提交、没有 domain/tags 的提交、相同 author date 的提交；
  断言行数、字段、跳过规则、顺序。
- **与 path 对拍**：同一小仓，`timeline --jsonl` 的行集合（`issue/action/commit/date/title`）与
  `path --all --json` 的动作集合一致（排序可不同）；每个动作恰好出现一次。
- **`--limit`**：`0` / 负数 / 缺值 / 非整数拒绝；`--limit 5` 恰好 5 行；`--limit` 大于总数输出全部。
- **流式首行**：100k 合成仓上计时第一行到达；`--limit 50` 计时全程；不持有 35MB 文档/全仓模型。
- **中断**：读几行后关闭管道，kiln 及时退出、无挂起。
- **错误**：非 kiln 仓、空仓、坏 trailer、jj 不支持路径；退出码/stderr 正确。
- **回归**：`path` / `list` / `graph` 的输出对拍（改前/改后逐字节）；`go test ./...`、`go vet`、`gofmt`。
- **真仓 dogfood**：本仓跑一次，行序/字段正确；与 `path --all` 的动作集合对拍。

## 验收

- **100k 合成仓**：冷启动第一行 ≤250ms（目标 ≤200ms）；`--limit 50` 全程 ≤500ms；
  不带 limit 全程 ≤2s；峰值内存 ≤64MB（不持整份文档/模型）。
- **10k / 64 条**：第一行分别 ≤200ms / ≤150ms；行数正确。
- 每行可独立 `JSON.parse`；字段、顺序、过滤、`--limit`、退出码符合上文。
- 消费者中途断开时 kiln ≤1s 退出；jj 行为明确（支持或非 0 报错）。
- `path` / `list` / `graph` 输出不变；`go test ./...` 全过；文档同步。
- 下游能只读前 N 行渲染首屏，不需要等完整流；不需要缓存/分页配合。

## 非目标

- 不改 `list` / `graph` / `path` 的既有输出或语义；
- 不做 issue 列表分页 / offset / 全局严格 author-date 排序；
- 不做文本 timeline 视图（本轮只 `--jsonl`）；
- 不做 `--domain/--tag/--action` 过滤（消费方自己筛；需要再开 issue）；
- 不做缓存（消费方决定缓存首屏）；
- 不建索引/数据库/常驻进程。

## 备注

- 需求来源：`dsh-kiln` 大仓首屏 timeline。它需要“最近动作先出现”，而不是等 `path --all`。
- 排序必须与 `path` 一致用 **author date**，不是 commit date；后者会被 rebase/amend 改写。
- `--limit` 是消费方“只读首屏”的廉价止损：够 N 行就停，不必为 35MB 付完。
- 与 `issue-list-scaling` 的“分页”不同：这是**动作流**，不是 issue 列表分页；
  但同样可以按“消费方需要前 N 条”这一触发条件来评估。
