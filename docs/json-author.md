# issue: json-author — `--json` 暴露 git 的 author（写侧不加字段）

- **domain**：cli
- **关联**：relates-to `author-attribution`（写侧"不加字段"的结论仍然成立；本条是它的**读侧**续集）；relates-to `machine-readable-output`（JSON 契约与"缺省即省略"的约定）；relates-to `json-fields`（字段命名与形状的先例）

## 背景

`author-attribution` 的结论是**不做**：不给动作加 `author:` trailer——没有消费场景、手写字段会漂移、真要区分身份 git 本身够用（`%an/%ae`）。这个结论**今天仍然成立，写侧不需要任何新字段**。

但读侧一直没开这个口子：`path --json` / `show --json` / `list --json` / `status --json` / `action log --jsonl` 全都只有 `date`，没有作者。今天所有提交共用同一个 git 身份（人 / agent 糊在一起），所以加了也是常量；可身份一旦分开（给 agent 配独立 `GIT_AUTHOR_NAME`），**现有输出测不出这件事**——没有字段可读，就没有观测点。

## 用户裁定（2026-11-02）

- **提交时不用写 `author` 字段**：kiln 用 git 后端，作者信息 git 本来就有，写侧保持零新增 trailer（与 `author-attribution` 一致）。
- **`--json` 输出可以带**：读侧把 git 已有的 author 投影进机器可读输出。

**为什么单开一条、不并回 `author-attribution`**：那条的结论"写侧不加字段"要原样保留；本条的主语是**读侧投影 + JSON 契约 + 缓存成本**，改的是另一层（`internal/cache` 的记录格式、`internal/cli` 的 DTO），domain 也从 `design` 变成 `cli`。`author-attribution` 的重启条件写的是"渲染要区分人 vs agent"（**文本**视图）与"出现第二个行动者"，本条都不是——是它没预料到的第三个消费面。

## 要回答的问题

1. **覆盖哪些命令**：`path --json`（每个 action）、`action log --jsonl`（每行）是刚需；`show --json`（那次提交）顺理成章；`list --json` / `status --json` 每个 issue 一行——写"**末条动作**的作者"，还是 `authors` 数组？`graph --json` / `check --json` 大概不涉及。
2. **字段形状**：`author` 一个字符串（`"Jackie Han"`），还是对象 `{"name":…,"email":…}`？jsonl 每条一行、逐行解析的场景，两个平铺键（`author` + `email`）是不是更省？
3. **author 还是 committer**：git 能表达双角色（author = 想的人，committer = 提交的人），这正是 `author-attribution` 点过的"归属里最值钱的一条"。只出 author，还是两个都出？倾向**先只出 author**：committer 有真实消费场景再加。
4. **成本**：
   - `path` / `show` 读的是 `git log --format=%H%x00%P%x00%aI%x00%B` 流（`internal/vcs/git/git.go:50`）和 raw commit（`splitRawCommit` 已经解析过 `author ` 行）——加 name/email **几乎零额外 git 调用**；
   - `list` / `status` / `action log` 走缓存记录层（`internal/cache/cache.go` 的 `Record` 只有 `Date` / `TZ`）——加字段 = **缓存格式变更**，升级后要重建一次；要给出本仓与 100k 合成仓的实测增量、缓存占用变化。
5. **共用身份时字段恒等**：现在人 + agent 共用同一个 `user.name`，输出全是同一串。这**不是不做的理由**（先把观测点立起来，成本低），但 README / schema 文档不能把它写成"能区分人机"的卖点——要区分还得先配身份。
6. **契约与兼容**：新字段是**增量**还是破坏性变更？按 `machine-readable-output` 的"缺省即省略"，取不到时（老提交 / 解析失败 / 后端无值）是**省略**还是空串？要不要提 schema 版本、改哪些 golden 测试？
7. **jj 后端口径**：`author.name()` / `author.email()` 在模板里现成（`internal/vcs/jj/jj.go:39` 只取了 `author.timestamp()`）；两后端要不要字段同名同形（jj 的 author 与 committer 也能同时取）。

## 非目标

- 不加 `author:` / `agent:` trailer，不动写侧；
- 不给**文本**视图加作者列（今天是常量，还会挤掉行宽；等身份真的分开再议）；
- 不做 `list --author=` 过滤、不做 `check` 的作者校验；
- 不代管 git 身份配置（那是使用者自己的 `user.name` / 环境变量）；
- 不承诺"能区分人 vs agent"——那要先有独立身份，属于 `author-attribution` 重启时的事。

## hypothesize：动作级 + 对象形状（用户裁定）

> 用户裁定（2026-11-02）：① **动作级先上**——`path` / `show` / `action log --jsonl`；`list` / `status` 本轮不动。② **对象形状**——`"author": {"name": …, "email": …}`。

### 1. 覆盖范围

三条都是"一次动作一行"的视图，加作者等于给每一行补一列来源；`list` / `status` 是 issue 级摘要，要先回答"末条动作的作者，还是全部作者"，还会立刻牵动缓存记录层——留到形状稳定后再说。

### 2. 形状

```json
{ "author": { "name": "Jackie Han", "email": "jackiehank@163.com" } }
```

- **对象**而不是平铺：以后加 `role` / `signature` 不破形状；jsonl 多一层嵌套是明确接受的代价。
- **缺省即省略**：取不到作者时整个 `author` 键不出现，不留空对象（与既有约定一致）。
- **email 跟 name 一起出**：`%ae` / jj 的 `author.email()` 本来就取得到；它是 PII，但本仓提交本来就带着它。

### 3. 读取成本

| 视图 | 数据来源 | 加 author 的代价 |
| --- | --- | --- |
| `path` / `show` | `git log` 流、raw commit | 格式串加两列；`splitRawCommit` 本来就解析 `author` 行 → **0 个额外进程** |
| `action log --jsonl` | 流 / 缓存记录 | 记录层多存两个字符串 → **缓存格式升级**（`Magic` 换一版，旧层直接忽略、自动重建） |

### 4. 一致性约束

缓存命中与冷读必须逐字节同形（`cache_test.go` 已经在守这条），所以记录层必须存 author——否则 `action log --jsonl` 的两条路径会给出不同结果。

## apply：落地

- **写侧零改动**：没有新 trailer、没有 `author:` 字段——作者就是提交自己的 `%an` / `%ae`（jj 的 `author`）。
- **读侧三处**：`path --json` 的每个 `actions[]`、`show --json`、`action log --jsonl`。前两者把 `author` 放在 `date` 后；`--jsonl` 按本 issue 定过的键序规则**追加在最后**。
- **形状**：`{"name": …, "email": …}`（`person.go` 的 `personDoc`），缺省省键——后端没给作者时整个 `author` 不出现。
- **数据通路**：git 的 `%H%x1f%P%x1f%aI%x1f%an%x1f%ae%x1f%B`（流式是同一套 NUL 版），`cat-file` 侧解析 `author Name <email> ts zone` 一行；jj 用 `author.name()` / `author.email()`。`path` / `show` **零额外进程**。
- **缓存**：`cache.Record` 加 `AuthorName` / `AuthorEmail`，`Magic` → `KILNREC2`、`Layout` → 2——旧层直接忽略、自动重建。每条记录多 8 B 索引；本仓 476 提交实测记录层 112,748 B ≈ **237 B/提交**（README 原先写的 130 B 是更早的测量，已按今日实测更新）。
- **一致性**：`cache_test.go` 的"缓存命中与冷读逐字节同形"通过——gap fill 走 `RawCommit`，那条路也补了 author（否则新动作那一行会缺键）。
- **测试**：`go test ./...` 全绿；golden（`basic` / `area` 的 `path_all_json` / `show_json` / `action_log_jsonl`）用 `-update` 重录；jj 的模板测试与 cache 的 magic 测试同步更新。
- **文档**：`machine-readable-output` 文末新增契约，`read-json` / `action-log` 各加一条后续，README 中英的 `--jsonl` 字段表与缓存占用同步。

## 记录

- 2026-11-02：由人工提出——写侧不写 author（git 已有），读侧 `--json` 可以带；并确认**单开一条 issue**，不并回 `author-attribution`。
