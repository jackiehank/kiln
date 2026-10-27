# path-batch — 一次进程读完多条 issue 的研究路径

## 问题

`kiln path` 一次只认一个 issue。而"把一个仓的动作序列整个读出来"是最常见的一种取数：

- DSH 的 kiln 插件要把 59 条 issue 的 257 个动作全部画成时间线（还要给搜索当检索面），
  只能起 59 个子进程；
- 想"看这个仓最近 20 个动作属于哪些 issue"的 agent，也只能逐个 path。

代价几乎全是**进程启动**。实测（Windows 11，20 核，`~/kiln` 59 条 issue）：

| 操作 | 子进程 | 耗时 |
| --- | --- | --- |
| `kiln --version`（纯启动地板） | 1 | 84 ms |
| `kiln path <id> --json` | 1 | 104–110 ms |
| `kiln graph --json`（一次启动读全图） | 1 | 103–115 ms |
| 59 条 path，并发 8 | 59 | 1290 ms |

即单条 path 里约 80ms 是启动，真正干活只占 20–30ms。并发 8 已经打到 Windows 进程创建的
平台期（8→24 并发只差 20ms 上下），**再优化并发没有意义**。

另外：`kiln path a b --json` 今天**静默忽略第二个位置参数**（exit 0，只回第一个 issue 的文档）。
这不是"不支持批量"，是"看起来支持了却只拿到一半"——比报错更危险。

## 建议

`kiln path --all --json`（或接受多个 id：`kiln path <id> <id> --json`；或 `--issue a,b`）一次
进程输出全部：

```json
{
  "issues": [
    { "issue": "a", "action": "apply", "done": true, "actions": [ … ], "relations": [ … ] },
    { "issue": "b", "action": "ask", "done": false, "actions": [ … ], "relations": [ … ] }
  ]
}
```

实现上就是**一次 `git log`**：`graph` 已经证明"一次启动读全仓"可行（节点、边、标题、标签都在
一份文档里），动作序列是同一次历史的另一半。

## 边界

- 只加批量，不改单条：`kiln path <id> --json` 的输出保持逐字节不变（现有消费者照旧）；
- 顺序写清：按 graph 的节点顺序（或 issue id 字典序），文档里点名；
- 单条失败不拖垮整份：该条给 `null` 或省略，另给一个 `errors` 数组；
- `--stat` / `--shortstat` 要不要支持批量另议，先只做 `--json`；
- 多位置参数不应再静默：要么按批量处理，要么报错（见 `silent-flag-drop` 的处理思路）。

## 验收

- `kiln path --all --json` 在 `~/kiln`（59 条）里一次进程返回 59 条动作序列，不需要 N 次启动；
- 单条 `kiln path <id> --json` 输出不变；
- 传了两个位置参数时，要么回两份数据，要么明确报错，不再静默丢一个；
- 文档（README / -h）写清批量形态与顺序。

## 落地（2026-10-27）

两种批量入口，都是**一次 `git log`、一份文档**：

- `kiln path --all --json`：全部 issue，顺序同 `graph` 的节点（`engine.IDs`，按 id 字典序）；
- `kiln path <id> <id> … --json`：只取点名的几条，**保持给定顺序**；未知 id 不再静默消失。

文档形状：

```json
{
  "issues": [
    { "issue": "a", "action": "apply", "done": true, "actions": [ … ], "relations": [ … ] }
  ],
  "errors": [
    { "issue": "nope", "error": "unknown-issue" }
  ]
}
```

- `issues` 恒为数组（空仓是 `[]`，不是 `null`）；每项与单条 `kiln path <id> --json` **逐字节同形**——批量没改 schema，只是外面套了一层。
- `errors` 缺省即省略；单条失败不拖垮整份（`unknown-issue` / `stat-failed`），其余照出，退出码 1。错误码是语言中立 token：JSON 不随 `--lang` 变。
- 单条路径不变：`kiln path <id> --json` 仍是一个裸对象，文本视图逐字节不变。
- 批量只服务 `--json`：`kiln path a b`（不带 `--json`）从"静默只回第一条"改为 **exit 2** 明确拒绝；`--all` 与 issue 参数同用也 exit 2。
- 顺带堵住同类静默：`--all` / `--json` / `--stat` / `--shortstat` 都是开关，若解析器把它们后面的位置参数吞成值（`path --all alpha`），现在直接报 "flag takes no value"，不再把 `alpha` 悄悄丢掉。

实现（`internal/cli/cli.go`）：`path` 判定批量后走 `pathBatch`；`pathDocOf` 是 `pathJSON` 与批量共用的构造函数；`FoldEdges` 在批量里只折叠一次；`statBackend` 只在要 stat 时才开后端。帮助与 `--lang en` 文案同步（`internal/i18n/catalog.go`）。

测试（`internal/cli/cli_test.go`）：`TestPathAllJSONIsEveryPathInOneDocument`（批量项与单条文档 `reflect.DeepEqual`）、`TestPathJSONKeepsTheGivenOrder`、`TestPathBatchKeepsGoingAfterAnUnknownIssue`、`TestPathRefusesBatchWithoutJSON`、`TestPathAllFlagIsJSONOnlyAndTakesNoIssue`、`TestPathAllJSONCarriesStats`、`TestPathAllJSONOnAnEmptyRepoWritesAnEmptyArray`。

复核：`gofmt` / `go vet` / `go test ./...` 全过；本仓真跑 `path --all --json`——60 条 issue、258 个动作、120 KB、**182 ms 一次进程**（旧法 59 次子进程约 1.3 s，见上文实测表）。

边界：`--all` 暂不吃 `--domain` / `--tag` 过滤（要过滤可先 `list --json` 拿 id 再点名）；批量带 `--stat` / `--shortstat` 时仍按动作逐次 `ShowStat`，省的是进程启动而不是 stat 读取——要快就别带 stat。
