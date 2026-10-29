# issue: action-log-relations — 一次扫描把关系也流式吐出来（先记账）

- **domain**：cli
- **关联**：relates-to `action-log`（本 issue 是它的字段扩展）；relates-to `relation-revision`（"声明 vs 当前态"的折叠规则由它定）；relates-to `relations-impl`（词表与 `unrelate` 的落地）；relates-to `read-json`（JSON 形状的既有约定：`path.relations` 复用 `FoldEdges`）；relates-to `machine-readable-output`（NDJSON 例外已在那里成文）

## 背景

`kiln action log` 就是"扫一遍提交、一条动作一行"：git 后端走 `git log -z --author-date-order --format=%H%x00%aI%x00%B`（新→旧），jj 后端走 `jj log` + `json(self)`（拓扑序）。每读到一条提交，`emit` 调一次 `trailer.Parse`（`internal/cli/cli.go:1374`），拿到的是**完整的 `trailer.Fields`**——`Issue`、`Action`、`Title`、`Refs`、`Target`、`Scope`、`Domain`、`Tags`、`Relations`、`Unrelate`。

然后只把其中 7 个字段写进 `actionLogDoc`，其余全部丢掉。所以：

> **关系不需要第二次扫描。** 它已经在手上，只是被扔了。这不是性能问题，只是 schema 问题。

本仓库实测（350 条动作，136 条非动作提交）：

| trailer | 出现在多少条动作 | `action log` 是否输出 |
| --- | --- | --- |
| issue / action | 350 / 350 | 是 |
| title | 349 | 是 |
| domain | 234 | 是 |
| scope | 161 | **否** |
| target | 49 | **否** |
| refs | 11 | **否** |
| tags | 1 | 是 |
| 关系行 | 96 行 → **137 条声明**，分布在 84 条动作（24%） | **否** |
| unrelate | 0 | **否** |

这张表已经说明现在的取舍站不住：留着只出现 1 次的 `tags`，却丢掉 161 条的 `scope`。那 7 个字段是照着"下游先拼 issue 列表"的用例裁的，不是照着"一次扫描吐全部"裁的。

另一头：折叠后 `kiln graph --json` 是 110 条边（`relates-to` 88、`caused-by` 21、`blocks` 1）。137 → 110 的差额来自按 (无序对, 槽) 合并与槽内取强，不是数据缺失。

量级（同一份仓库）：`action log --jsonl` 现在 350 行 / 100,768 B，平均 288 B、最长 474 B。137 条声明按 `{"name","target"}` 塞进去约 +6～8 KB（**≈8%**）；按 `graphEdge` 的 `{from,to,name,directed}` 大约翻倍。100k 动作时需要复核，但在这个量级上不构成决策依据。

## 要回答的问题

1. **吐"声明"还是"当前态"？** 这是分水岭。
   - 声明（`这次动作说：blocks X`）：忠实、免费、可截断（`--limit` 只是少了尾部），下游要自己折叠才知道"现在谁阻塞谁"。
   - 当前态（`这次动作之后，X 阻塞 Y`）：下游直接可用，但折叠是**正向**的（旧→新），而流是**新→旧**的——单遍反向扫描算不出正向折叠；`--limit` 还会让"当前态"名不副实（截断点之后的变化没被应用）。
   - 倾向声明；当前态继续由 `graph` / `path` 负责。
2. **顺序不能保证可回放。** git 是 author date 倒序；jj 是拓扑序（线性历史恰好等于倒序，分叉不等）；同一时间戳的提交，`FoldEdges` 用 issue id 做 tie-break，而流里只有提交顺序。所以"下游重放 log 会得到和 `kiln graph` 一模一样的图"**不能承诺**。要不要把它写成显式限制，而不是留给下游踩？
3. **形状**：`[{"name":"blocks","target":"x"}]`（声明形，最贴 `Fields.Relations`）／`{from,to,name,directed}`（复用 `graphEdge`，与 `graph`/`path --json` 同构）／`{"blocks":["x"]}`（map，短，但键序与重复难规范）。已有先例是 `graphEdge`，但它是**折叠后**的边，硬套到单条声明上会把"方向"提前归一——见下一条。
4. **逆名怎么归一？** 一条动作可能写的是 `has-part: X`（"我属于 X"的反向）。声明形里它是 `name=has-part, target=X`；`graphEdge` 里必须变成 `from=X, to=me, name=part-of`。后者要求输出前查词表并翻转，信息更多，但等于把可配置的词表变成命令输出的先决条件。要不要两条都留（原样名 + 归一 `from/to`）？
5. **词表版本 ≠ 字节回放。** `trailer.Parse` 会把别名 canonicalize（`derived-from` → `caused-by`），也接受大小写与空格变体。所以 log 展示的是"按**当前**词表重新解释过的历史"。可接受吗？要不要加 `--raw` 逃生口，整条 trailer 原文照抄？
6. **`unrelate` 怎么带？** 单列 `{"unrelate":["x"]}`？还是混进 `relations` 数组（`{"name":"unrelate","target":"x"}`）？它语义上不是一条关系，而是"撤销关于 x 的全部声明"；不属于任何 slot，也不该被折叠。
7. **其他 trailer 一起吐吗？** `scope`(161) / `target`(49) / `refs`(11)。吐，命令就从"活动流"变成"提交元数据的 NDJSON 投影"；不吐，至少把 `scope` 这个明显的漏项补上（`path --json` 也只带 `refs`/`tags`）。
8. **文本模式动不动？** 现契约是"一条动作一行，无表头"。关系塞进去会撑长行、破坏对齐。选项：文本不动、关系只走 `--jsonl`（两模不对称，但与 `action list` 只有 `--json` 的做法一致）；或只在有关系时追加 `← blocks:x, relates-to:y`。
9. **要不要 `--reverse`？** 若目标是"可回放的事件流"，旧→新才自然。但 git `--reverse` 必须先读完历史才吐第一行；jj 本来就不是增量的（200 条实测首行 141ms ≈ 全量 152ms），`reverse()` 又要在内存里排序。**流式承诺会当场失效**——明确拒绝，还是承认它只是"批量模式"？
10. **谁真的要用？** 现在想得到三类：下游插件维护自己的图/看板；`kiln graph` 自举（用日志重建图再和 `FoldEdges` 对账，当自检）；agent 一次扫到底，免掉 `path --all` 的 35 MB 文档。缺一个具体消费方，这条就还只是"免费顺手"。
11. **排序与去重**（形状定了才有意义）：一条动作内多条关系按词表序 + 未知名排序？同 (name,target) 去重？
12. **验收怎么写？** 能不能写成可执行的断言——"`action log --jsonl` 的声明按 `FoldEdges` 规则重算 == `graph --json`"？做不到的话，把做不到的原因写进文档（顺序、tie-break、截断），别让下游以为能对账。

## 非目标

- 不改 `graph` / `path` 的输入输出；
- 不做增量/订阅（`watch`、`tail -f` 是另一个命令）；
- 不改 `--limit` 与 broken pipe 的既有语义；
- 不做按关系过滤（`--blocks-of x`）——那是过滤器，另开一条。

## 记录

- 2026-10-29：从"扫一次提交、action/issue/relation 一起吐"的观察开的条目。当前决定：**只记账**（ask），等有具体消费方或第二次需求再 hypothesize。
