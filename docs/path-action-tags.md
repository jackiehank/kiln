# issue: path-action-tags — `path --json` 的动作行补 `tags`

- **domain**：cli
- **关联**：relates-to `json-fields`、relates-to `machine-readable-output`、relates-to `graph-query`

## 背景

`tags:` 是**每个提交**的 trailer（动作级），但现有 JSON 出口只到 issue 级：

- `graph --json` 的节点 `tags` 是 engine 从**每一个**动作累加出来的**并集**
  （`docs/agent-ux-review.md` §294："从每一个动作累加 `Domain`…… / `Tags`"）；
- `list --json` / `status --json` 的 `issues[].tags` 同样来自 `model.Issue.Tags`（并集）；
- **`path --json` 的动作行没有 `tags`**：`internal/cli/cli.go` 的 `pathAction`
  只给了 `ordinal / action / id / date / title? / refs? / stat?`，
  而 `internal/model/model.go` 的 `Action` 结构上本来就有 `Tags []string`
  —— 数据已在内存里，只是输出层没带出来。

实测（v0.1.1，本仓的一个 issue）：

```text
kiln path kiln-flow-focus-ui --json    # 下游仓里的用例
# 每个 actions[] 都没有 tags 键；同一 issue 的五个动作在 git 里各带各的标签：
#   ask: flow,focus,ui   collect: flow,focus,research   test: flow,test
#   test: flow,test      conclude: flow,focus
# 而 graph --json 的节点回 ["flow","focus","ui","research","test"] —— 并集。
```

### 谁需要它

下游有真实消费者：DSH 的 kiln 视图（`dsh-kiln` 插件）要在时间线上按标签筛**动作**。
现在只有两条路，都不通：

1. **用 issue 级并集** → 张冠李戴：某条 `conclude` 没带 `test`，却因为同 issue 的 `test` 动作带了而被筛出来；
2. **用 `show --json` 逐个动作取 body 再解析 trailer** → 一个动作一次子进程；
   实测两个仓分别是 104 与 256 个动作，视图不可能这么取数。

也就是说：动作级标签的真相在 git 与 `model.Action` 里都有，唯独没有一条**能一次拿全**的读出口。

## 要回答的问题

1. **加不加**：动作级 `tags` 该不该进 `path --json`？
   （本 issue 的主张：该加——数据已在 `model.Action` 上，纯输出层补齐，不引入新语义、
   不新增解析路径、不改任何既有字段。）
2. **键名与空值**：`tags`（与 trailer、`graph --json` 节点、`list --json` 同名）＋ `omitempty`
   （与既有"缺键=无"约定一致）？还是空数组必给（下游不必判 `undefined`）？两种都可用，
   但要写进 `docs/machine-readable-output.md` 的字段表——别让下游靠猜。
3. **顺带补不补同级字段**：`model.Action` 上还有 `domain` / `scope` / `target`，
   `pathAction` 现在一个都没给。本 issue 只要求 `tags`；其余是否同批做留给 hypothesize
   （一起做是"一次补齐动作元数据"，分开做是"只补有下游需求的"）。
4. **并集语义是否受影响**：`graph --json` 节点的 `tags`、`list --tag=` 的过滤都保持
   issue 级并集，本 issue **不改**它们——只让动作级的真相可见。
5. **人读文本要不要跟着打**：`path`（文本视图）是否在动作行后面补 `tags=`？
   现状是"非空才打"，保持一致即可；这条得先定，否则 `--json` 与文本会各说一套。

## 验收（若做）

- **E1**：`kiln path <issue> --json` 的每个动作行带 `tags`，取自该提交 trailer，
  顺序与去重规则与 `model.Action.Tags` 一致。
- **E2**：无标签的动作按问题 2 的决定处理（省略 or `[]`），两种情形都有测试覆盖。
- **E3**：`graph --json` 节点、`list --json` 的 `issues[].tags`、`list --tag=` 的过滤结果
  **逐字节不变**。
- **E4**：文本视图按问题 5 的决定处理；无论哪种，`--json` 之外的命令输出不变。
- **E5**：`docs/machine-readable-output.md` 的 `path` 字段表补上该键（写清空值约定），
  有 golden 文档一并更新。
- **E6**：`go build ./...` / `go vet ./...` / `go test ./...` 全过；真仓冒烟至少覆盖
  "同一 issue 的不同动作 tags 各不相同"这一情形。

## 非目标

- 不改 `graph --json` 的并集语义，也不新增"按 tag 查动作"的命令。
- 不动 `list --tag=` 的过滤口径（仍是"任一动作带过"）。
- 不扩展 `show --json`（它已有整段 body，需要的人可自行解析）。
- 不引入新的 trailer 键，也不改 `tags:` 的写法。

## 线索（实现位置）

| 位置 | 现状 |
| --- | --- |
| `internal/cli/cli.go` `pathAction`（约 1136 行） | 缺 `Tags` 字段 |
| `internal/cli/cli.go` 构造处（约 1200 行） | 只拷了 `Ordinal/Action/ID/Date/Title/Refs` |
| `internal/model/model.go` `Action`（约 110 行） | 已有 `Tags []string` |
| `internal/cli/cli_test.go`（`TestPathJSONRendersThePath` 起） | 补一条"动作 tags 各不相同"的断言 |
| `docs/machine-readable-output.md` | `path` 的字段表要加一行 |
