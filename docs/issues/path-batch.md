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
