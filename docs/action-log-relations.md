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

## 目标形态：一条总流，而不是几条分命令（2026-10-29 追加）

要的不是"给 `action log` 加一个 `relations` 键"，而是：**kiln 应该有一个总的流式输出，一次扫完，把所有信息全给出去**。分几个命令各自流一遍，效率上不如一遍。

这个说法在本仓库里是站得住的，因为今天下游要凑齐一份"动作 + 关系 + 改动"至少跑三到四次，而它们读的是同一批提交、同一份 message：

| 命令 | 结果 | 实测耗时 / 大小（本仓 366 提交，三次取最小） | 是否真流式 |
| --- | --- | --- | --- |
| `action log --jsonl` | 350 行动作 | 117 ms / 101 KB | **是**（不建模型） |
| `graph --json` | 115 条折叠边 | 127 ms / 34.5 KB | 否（先 `engine.Load`） |
| `path --all --json` | 每 issue 的路径文档 | 169 ms / 170 KB | 半（流式写文档，但先装模型） |
| `list --json` | issue 列表 | 116 ms / 23 KB | 否 |

四个加起来 ≈ 0.53 s。在这个规模上主要是进程启动 + 一次 `git log`；到 100k 动作时遍历本身成为主项（此前在 100k 合成仓测得 `path --all` 全量 954 ms / 35.6 MB）。

更关键的结构事实：**今天只有 `action log` 是不建模型的流式原语**，`list` / `status` / `graph` / `path` / `check` 都是先 `engine.Load` 把整段历史读进内存再渲染。所以"总流"不是要发明新原语，而是把唯一那个流式原语做全——代价比想象中小，边界也比想象中清楚。

## 一遍扫描到底能带什么（分层）

| 层 | 内容 | 一遍扫描能带吗 | 代价 / 边界 |
| --- | --- | --- | --- |
| T1 | trailer 全字段（`relations` / `unrelate` / `scope` / `target` / `refs` …） | **能**，已经解析出来 | 0，今天就在付 |
| T2 | 改动文件 + 增删行数 | **能**，同一次 `git log` | git：`-z --numstat`，本仓字节 +14%、时间不变（见下）；jj：`diff.summary()` / `diff.stat()`，只有文本 |
| T3 | 折叠态：graph 边、issue 当前标题、动作序号 | **不能** | 折叠是正向（旧→新），流是反向（新→旧）；且需要全模型 |
| T4 | diff 正文（`show`） | 不能 | O(历史 diff 总字节)，必须按需 |
| T5 | 派生诊断（`check`） | 不能 | 要模型 + 规则 |

所以"把所有信息全给出去"要分层答应：**T1 + T2 一次扫描全给**，这是总流；**T3 仍是派生视图**（`graph` / `path` 已负责）；**T4 / T5 永远按需**。若硬把 T3 塞进流里，就得先全量读入再吐——那和现在的 `path --all` 没区别，流式承诺当场失效。

## 分帧与后端的实测（2026-10-29）

- **git 的 T2 代价**：本仓 `git log -z --author-date-order --format=%H%x00%aI%x00%B` 44.8 ms / 155,901 B；加 `--numstat` 44.8 ms / 178,262 B（时间不变、字节 +14%）；加 `--raw` 46.4 ms（+4%）；`--name-only` 44.4 ms。**比例不可外推**——diff 成本随树大小走，本仓只有几十个文件。
- **git 的分帧**：实测 `-z --numstat` 的布局是 `body<NUL>\n<numstat 行><NUL>…<下一条 header>`。numstat 条目数未知，所以 record 边界不能靠"NUL 数"数出来；现有 `readNULField`（读三个字段就完事）要升级成状态机：带 tab 的算 numstat 行、40 位 hex 的算下一条 header。rename 的 `-z` 形态（`add\tdel\t` 后跟旧/新路径分两个 NUL）还要单独探针。
- **jj 的 T2**：`json(self)` 不含 diff；但模板支持 `diff.summary()`（`M a.txt` / `A b.txt`）和 `diff.stat()`（`a.txt | 2 +-` / `2 files changed…`）；`json(diff)` 直接报 `Expected expression of type Serialize, but actual type is TreeDiff`。所以 jj 能在同一遍里给出文件级摘要，但**只有文本、没有结构化 JSON**，行数得自己解析。jj 0.45.1 本机实测。
- **两后端的 T2 形状因此不同**：git 是机器友好的三元组，jj 是给人看的文本。要么在 kiln 里解析 jj 的 stat 文本（脆弱），要么接受"总流在 jj 上 T2 降级为 summary"。

## 追加的问题

13. **命令形态**：新开 `kiln stream`（T1+T2 的 NDJSON 总流），还是 `action log --everything`？前者让总流有独立名字、不被 `action log` 的既有契约（一行一动作、文本模式、`--limit` 数动作）绑住；后者复用出口但语义会涨。
14. **一条动作一行，还是动作行 + 文件行**？T2 进流后有两个形状：`files` 内嵌数组（守住"一行一动作"，行变长）；或动作行 + 文件行（更贴 git 原生流，但 NDJSON 要多一个 `type` discriminator）。
15. **issue id 要不要改**：本 id 叫 `action-log-relations`，范围已经扩到"总流"。改名要新 id + `supersedes` 边；我倾向先不改——id 是提交里的稳定句柄，等 hypothesize 定了形态再决定，标题先承载范围。
16. **和 `path --all --json` 的关系**：它已经在流式写一个大文档（`pathBatchWriter`）。总流取代它，还是并存（一个 NDJSON 事件流、一个按 issue 分组的文档）？
17. **100k 复核**：今天只在本仓测了 T2 的边际成本，比例不能外推。定案前要在 100k 合成仓上再测一次 plain vs `--numstat`，否则"总流更高效"只是在小仓上成立。

## 非目标

- 不改 `graph` / `path` 的输入输出；
- 不做增量/订阅（`watch`、`tail -f` 是另一个命令）；
- 不改 `--limit` 与 broken pipe 的既有语义；
- 不做按关系过滤（`--blocks-of x`）——那是过滤器，另开一条。

## 命名：先不改，但要认下"名字跟着记录单位走"（2026-10-29）

用户问：总流还叫 `kiln action log` 合适吗，要不要改成 `kiln log`？

**结论：这次不改，沿用 `action log`。** 这不是新问题——`action-view` 已经辩过一轮（`e526dde`，2026-10-28），`kiln log` 是被明确否掉的：

| 候选 | 当时的裁决 | 理由 |
| --- | --- | --- |
| `kiln action log` | **选** | `action` 作用域限定；`list` 列 schema、`log` 列 events，成组；"log 有作用域限定，不是顶层 `kiln log`" |
| `kiln log` | 不选 | 撞 `git log` 的心智；`path` 已占"单 issue 的 log"生态位；与已否的 `log --rewrites` 重名 |

现在的新问题是"总流还是不是动作流"，答案是**是**：进流的仍然只有带 `issue:` + `action:` 的提交，T2 带的是这条动作所在提交的改动，**记录单位还是动作**（一行一个动作）。所以名字仍然准确，没有失真。

名字真正失真的条件是下面两条之一，而且两条都该**新开命令**、不是改名：

1. 行形状变成"动作行 + 文件行"多类型（那时一行不再是一个动作，`--jsonl` 的"一行一动作"契约也一起破）；
2. 流里开始收非动作提交（那时它就是 `kiln log`，但那是另一个命令，语义是"仓库的提交流"，kiln 明确不做 `git log` 的包装）。

不改的成本考量：v0.3.0 才发一天，`action log` 已写进双语 README、`machine-readable-output` 与 900 行 `action-log` 文档，下游插件按 `--jsonl` 接入；改名要么并存双份名字（双份 usage / 文档 / 弃用提示），要么直接破坏。为 7 个字符付这个账不值。

唯一承认的结构性别扭：`action` 组里只剩 `list` 与 `log`，若哪天 `log` 搬走，`action` 组就只剩一个子命令，那时组本身该退成 `kiln actions`——但这属于"结构性改动"，别和总流混在一起做。

### 更强的理由：这个名字是按粒度命名，不是按字段命名

用户补了一句解释，比上面所有惯例论证都强：**kiln 的基本粒度是 action，relation 等信息都是跟着提交走的，所以 `action log` 就是所有 kiln 信息的集合。**

这是结构性的，不是习惯性的：

- 存储里没有第二种东西——每个事实都是某条提交上的一条 trailer；
- 每条提交只有一种类型判断（是不是动作：带 `issue:` + `action:`），所以**信息的粒度 = 动作**；
- 于是加字段（relations / files / numstat）是**给同一行加属性**，不是引入第二种记录；一行的语义永远是"某条动作 + 它的全部属性"；
- 结论：**按粒度命名的名字，不会因为字段变多而失真。** 名字的射程 = 存储层（T1 + T2），恰好等于可缓存的那一层；T3–T5 是计算出来的视图，本来就不属于"信息集合"。

严格说"所有信息"要收窄成"所有**已存储**信息"：派生态（issue 当前标题、动作序号、graph 边、check 诊断）不是存储，是计算。但这个收窄正好和分层一致——总流就是存储层，视图就是视图。

因此要守住的不变量是**"行 = 动作"**，不是名字本身。这一条把前面几个决定串成同一根线：为什么吐声明而不是当前态（行里装的是这条动作**说的话**）、为什么 `files` 要内嵌（保住一行一动作）、为什么把解析外包给 `%(trailers)` 危险（"动作"的定义在 kiln 里，不在 git 的 trailer 规则里）。

## 记录

- 2026-10-29：从"扫一次提交、action/issue/relation 一起吐"的观察开的条目。当前决定：**只记账**（ask），等有具体消费方或第二次需求再 hypothesize。
- 2026-10-29（同日晚）：用户把目标从"加一个键"推到"一条总流，一次扫完，全部给出去"，理由是效率。补充了四层切分（T1/T2 可一遍带、T3 派生、T4/T5 按需）、两后端的 T2 实测与分帧细节，并把命令形态/id 改名/100k 复核留作待定。仍未 hypothesize：要先确认 T3 不进流这条边界是否接受。
- 2026-10-29（同日晚）：命名问题当场裁决——**沿用 `action log`**，依据 `action-view`（`e526dde`）已否掉 `kiln log`，且总流的记录单位仍是动作；名字失真的两种条件（多类型行、收非动作提交）都指向"新开命令"而非改名。
- 2026-10-29（同日晚）：补上更强的命名论据——**按粒度命名**：存储里只有提交，每条提交的元数据都是那条动作的属性，所以 `action log` 天生是"全部已存储信息"的集合，加字段不改名；要守的不变量是"行 = 动作"，T3–T5 仍是视图。

