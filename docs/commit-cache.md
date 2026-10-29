# issue: commit-cache — 把一次扫描的结果落成缓存文件（先记账，暂不建）

- **domain**：design
- **关联**：relates-to `perf-audit`（它把索引/缓存列为明确不做，但留了"再单独立项"的口子，并给了触发条件——本 issue 就是那个立项）；relates-to `action-log-relations`（总流是"顺手落盘"的那个进程）；relates-to `action-log`（现成的流式原语）；relates-to `issue-list-scaling`（规模论证的前置）

## 背景

`perf-audit` 的结论是"无索引"：每条读命令都从提交历史现算模型，换来"零状态、永远和 git 一致"。它同时留了一句："如果将来长历史上确实疼，再单独立项"，并给了触发条件（真实仓 1k+ issue，或单次 `path --all` > 10 MB）。

本 issue 就是那个"单独立项"。触发还没有到——本仓现在 69 issue / 367 提交——所以**现在不建**，先把设计记下来，免得真要建时重新推一遍。

新增的论点是这一条：**缓存不必是"额外一遍"**。`action-log-relations` 里的总流本来就要遍历每条提交、用 `trailer.Parse` 解析全部字段；把解析结果顺手落盘，缓存就是总流的副产物。这把缓存从"多付一次全史扫描"变成"零边际成本产出"——原来挡着它的成本理由消失了，剩下的是正确性和写策略。

## 今天的账：缓存一定是净亏

本仓 366 提交，三次取最小：

| 命令 | 耗时 | 其中 |
| --- | --- | --- |
| `kiln --version` | 62 ms（`cmd /c` 包一层；`perf-audit` 记录纯启动 46 ms） | 进程启动地板 |
| `action log --jsonl` | 117 ms | 地板 + 一次 `git log` + 解析 |
| `graph --json` | 127 ms | 地板 + 同上 + 建模折叠（≈10 ms） |
| `path --all --json` | 169 ms | 地板 + 同上 + 渲染 170 KB |

`graph` 只比 `action log` 多 10 ms——**折叠和建模不是瓶颈，`git log` 的传输与解析才是**。而在 366 提交上，省掉 log 最多省 20 来毫秒，缓存要多付一次读盘 + 一整套失效/损坏/并发的正确性支出。不划算。

## 为什么相信它在规模上会赢

`perf-audit` 与 `timeline-stream` 已经测过 100k：

- 全史 `git log` **1.3–1.7 s**，`path --all` 峰值 **190 MB**，`show` 851 ms，`commit` 1.5 s；
- 首行 64 ms（有 commit-graph）/ 359 ms（无）。

瓶颈是 **body 的字节数与解析**（100k 条 × 约 450 B ≈ 45 MB），而缓存恰好能整段省掉它：只存解析后的记录（hash、日期、issue、action、title、domain、tags、relations），约 100–200 B/条 → 100k 时 10–20 MB，顺序读 + 零解析，量级上是几十毫秒对一秒。

换算下来，`graph` / `check` / `list` 在 100k 上大概能快 10–30×。**`path --all` 是个例外**：它 35 MB 的输出本身就是大头，缓存救不了它；`--stat` 的 diff 计算也是另一层（每提交确定，可缓存，但体积大一个量级）。

## 真要建时的清单（现在只记，不写代码）

1. **键 = commit hash，只增不改。** 同一个 hash 的消息、日期、父提交永不变化 → 没有失效逻辑，只有 GC。这是整个方案能成立的前提，也是它比"缓存派生模型"安全得多的原因。
2. **但解析依赖配置。** 关系别名 → 规范名由词表决定；缓存里应存**原文键值**、读时再解析，或以配置指纹做命名空间。否则改一次 `.kilnconfig.jsonc` 就会读到旧词表下的规范名。
3. **jj 不一样。** 重写会换 commit id、弃用提交留垃圾；colocated 仓里 git hash 与 jj change id 是两套身份。缓存必须按后端分键，jj 可能得二期。
4. **写策略才是真难点。** 读命令要不要写盘（只读仓、CI 并发、`--no-cache` / `KILN_NO_CACHE`）；放哪（`.git/kiln/cache/` 每克隆一份、worktree 天然共享、不进版本控制；还是 XDG 用户缓存，跨克隆但 key 要自己算）；原子写（temp + rename）；**半写损坏必须退化为全扫，绝不给错结果**。
5. **格式版本化**：magic + version 头，不匹配就当没有，别做迁移。
6. **GC**：按可达提交裁剪；手动 `kiln cache gc` 还是自动，取决于体积。
7. **验收硬条件**：命中与不命中的输出**逐字节一致**。这条比"快多少"重要——`perf-audit` 的三批优化全是这么验收的。
8. **与"只写 stdout"的契约冲突**：`machine-readable-output` 的约定是命令只产出文档；一个后台落盘行为要不要在 `-h` 与文档里显式声明？

## 先做便宜的那两步（都测过，零/低成本）

- **git 自己的 commit-graph**：100k 上首行 359 → 64 ms（`timeline-stream` 实测）。这是 git 的持久化缓存，维护成本为零。kiln 该不该在文档里建议、或在读路径上可选地 `git commit-graph write`？几行代码，先于自己造缓存。
- **`%(trailers)` 只读 trailer 块**：本仓 `%B` 156,223 B → `%(trailers)` 107,018 B（**−31%**），时间在噪声内。但**兼容性有洞**：本仓 352 条动作里 git 认不出 4 条——
  - `f8413d8`：整条消息只有 trailer，没有标题行；
  - `c12f6eb`、`8a0fe99`、`e0ff712`：标题与 trailer 之间没有空行（正是 `commit-format` 那条 issue 的历史产物）。

  所以它只能当"快路径 + 回退"，不能直接换掉 `%B`。反过来看，这个洞恰好支持本 issue 的立场：**解析语义应该留在 kiln 里**，缓存存的应该是 kiln 的解析结果，而不是把解析外包给 git 的 `%(trailers)`。

## collect（2026-10-29）：git 怎么区分"存储"与"计算"

> 依据：[git-commit-graph](https://git-scm.com/docs/git-commit-graph)、[commit-graph 格式](https://git-scm.com/docs/commit-graph-format)、[racy-git](https://git-scm.com/docs/racy-git)、[bitmap-format](https://git-scm.com/docs/bitmap-format)。本机 git 2.55.0。

### 1. 两个平面

| 平面 | git 里的东西 | 特点 |
| --- | --- | --- |
| 真相 | object db（commit = tree + parents + author/committer + message）、refs、index 里的 path→OID | 内容寻址、不可变；只被新建覆盖，不被"更新" |
| 加速 | commit-graph、reachability bitmap、multi-pack-index、`.rev`、index 的 stat 缓存、untracked cache、fsmonitor | 全可删可重建；删掉只变慢，不改变任何答案 |

index 是两栖的：`path→OID` 是真相（暂存区），`stat` 信息是加速。

### 2. commit-graph：把"提交自带的事实"和"算出来的事实"放进同一个文件

每条提交存：OID、父提交（positional 下标）、root tree OID、提交时间、generation number（拓扑层级），可选再加 corrected commit date（GDA2）、**changed-paths Bloom filter**（BIDX/BDAT）、以及分层用的 BASE。也就是说：

- **父提交 / 树 / 时间**是"提交里本来就有的"，复制过来只为加速遍历；
- **generation number / corrected commit date / Bloom filter** 是**算出来的**，git 照样缓存——这正是"计算的那部分也可以缓存"的先例。

关键在**键**：这些派生数据以 commit OID 为键，而 OID 不可变 → **永远不会过期**。bitmap 同理，以 tip 提交为键：给某个 tip 算的可达性集合对该 tip 恒真，tip 一变就不用它。

### 3. 失效机制（可以直接抄的五条）

1. **内容寻址**：键不可变 → 不需要失效逻辑，只需要 GC。
2. **缺失即正常**：commit-graph 不记录"覆盖到哪"，它就是"没有更新的提交"。实测：写完 graph 再提交，`git rev-list --count HEAD` 仍 4、`git log` 仍 4 行、`git commit-graph verify` 仍 exit 0——**陈旧只变慢，不变错**，缺的提交回落到对象库按需读。
3. **自校验文件名**：split 层的文件名 = 去掉尾部 trailer 的 SHA-1 = 它自己的 trailer。实测本仓三层全部成立（`8a169f2…` / `8cc7b5f…` / `eaf71be…`），所以内容被截断或篡改可以靠名字发现。
4. **结构版本靠换 id，不迁移**：GDA2 / GDO2 之所以带 "2"，是因为老版本写过错的 GDAT / GDOV；新 git 直接忽略旧 chunk 重写。对 kiln 的含义：格式一变就换 magic / version，旧文件当没有。
5. **元数据 + 可疑才验**（index / racy-git）：先比 stat（mode、mtime、ctime、uid、gid、ino、size），不一致才看内容；写入时若条目"racy"（mtime 不早于 index 自身）就把 size 截成 0，强制下次重验。经典的"便宜判断 + 贵兜底"。

还有一条**边界**：replace object / graft 一出现，git 直接关掉 commit-graph 的读写——语义被外部改写时缓存自动退场。kiln 的类比是**改关系词表**：槽位与强度变了，折叠就不再是同一个函数，所以缓存必须带词表指纹。

### 4. 分层的工程策略（本仓正在用）

`--split` 把 commit-graph 写成**不可变的一层层**：新提交进新的 tip 层，`commit-graph-chain` 列出层哈希；tip 超过前一层 2 倍（`--size-multiple=2` 默认）或超过 `--max-commits` 就合并；`--expire-time` 删旧层；`--split=no-merge` / `replace` 是极端档。本仓现在正好三层（7,164 / 27,392 / 14,104 字节）。

### 5. jj 是同一形态

jj 也把索引存成内容寻址的不可变片段：`.jj/repo/index/segments/<hash>`（本机 jj 0.45.1 实测），外加 `op_links/`。名字不同、语义相同：**追加不可变片段，由读取方归并**。jj 明说内部格式会变，所以只能学形态，不能读它的文件。

### 6. 对 kiln 的直接推论

1. **枚举与取属性分开**。枚举只要 OID 列表——`git rev-list HEAD` 只走拓扑、不解消息；属性靠 `OID → record` 查缓存，缺的才去读 body。实测本仓：`rev-list` 15,211 B / 66.2 ms，`--format=%H%x00%aI` 24,857 B，带 body 157,633 B（**10×**）。这样"新鲜度"问题自然消失：**枚举永远来自 rev-list，缓存只回答属性**，rebase / amend 留下的不可达记录根本不会被枚举到，最多占地方等 GC。
2. **缓存文件自校验**：文件名或头部存内容 hash，对不上就当没有（照抄 commit-graph 的 trailer 约定）。
3. **派生结果可以缓存，但键要装下全部输入**。某 issue 的折叠路径 = f(它的动作 OID 序列, 词表指纹)；键一变就是新条目，旧条目自然作废——不需要"失效逻辑"，只需要 GC。generation number / bitmap 就是这么活的。
4. **但默认先不缓存派生结果**。折叠只要 ~10 ms（本仓），而派生缓存会把"命中与否字节一致"从天然成立变成需要证明。等派生本身变贵（100k 上的 `check` 规则之类）再说。
5. **合并策略直接借 `size-multiple=2`**：记录层按"新增超过已有 2 倍就合并"来管，比"最多留 N 个文件"好推理。

### 7. 由此新增的问题

10. **枚举也交给 git 吗**：`rev-list` 在"有缓存"和"没缓存"两条路径上都要跑（它便宜但非零）。这会不会让"一个提取层两个数据源"变成三处？还是把它当成提取层的第一步，所有路径共用？
11. **派生缓存做不做**：做的话键 = (动作 OID 序列, 词表指纹)；不做的话，100k 上折叠的真实耗时得先量出来，别凭"本仓 10 ms"外推。
12. **层合并策略**：照抄 `size-multiple=2` + expire，还是简化成"最多 N 层、超过就全量重写"？
13. **还没量到的**：`git rev-list` 在 100k 的耗时（本仓 66 ms 里 ~60 ms 是进程启动，比例不可外推）；逐条 `git log -1 --format=%B <oid>` 补缺口 vs 一次 `git cat-file --batch`；jj 侧没有 `rev-list` 等价物（枚举要靠 `jj log --no-graph -r 'all()'`，而它本来就不增量）。

## 要回答的问题

1. **触发条件**：照 `perf-audit` 的（1k+ issue 或 `path --all` >10 MB），还是提前到"有人抱怨慢"？谁来认领这个判断？
2. **缓存哪一层**：只 T1（每提交解析记录），还是也含 T2（每动作 diffstat）？T2 也是每提交确定的、可缓存，但体积大一个量级。
3. **位置**：`.git/kiln/cache/`（每克隆、worktree 共享、天然不进 VCS）还是 XDG（跨克隆，但要自己算 key、还要处理仓库移动）？
4. **读命令允许写盘吗**？需要一个开关吗？只读挂载、`GIT_DIR` 只读、CI 并行怎么退化？
5. **并发**：两个 kiln 同时写（worktree + 多 agent 是真实场景）——原子 rename 够不够？要不要锁文件？锁坏了怎么办？
6. **"赢"怎么量**：同一 100k 合成仓，`graph` / `check` / `list` 冷/热缓存各多少 ms、内存峰值多少？没有这组数就不该合入。
7. **与总流的关系**：写缓存的是 `action log`/`stream` 那个进程吗？如果是，它就从纯读变成"读 + 落盘"——要不要只在显式开关下写？
8. **daemon 仍然不做**吧？（`perf-audit` 已否决；这里只谈文件缓存。）
9. **只对 git 生效**可以接受吗？

## 非目标

- 不做 daemon / 常驻进程 / 跨进程内存共享；
- 不做跨机器共享缓存；
- 不缓存 diff 正文（T4 永远按需）；
- 不改任何命令的输出字节（命中与否必须字节一致）；
- 不在本 issue 里动 `path --all` 的 35 MB 输出（那是 `path-batch` / `issue-list-scaling` 的地盘）。

## 记录

- 2026-10-29：由"总流扫的时候顺手落缓存"的想法开的条目。**现在不建**——今天缓存是净亏（进程启动 46–62 ms，log 边际约 20 ms），触发条件沿用 `perf-audit`（1k+ issue 或 `path --all` >10 MB）。先记下设计前提（hash 键、只增不改、损坏退化为全扫）与两个更便宜的先行项（commit-graph、`%(trailers)` 快路径）。
- 2026-10-29（同日）：收集 git 自己的做法（§collect）。要点：git 用"内容寻址 + 缺失即正常 + 自校验文件名 + 换 chunk id 不迁移 + stat 可疑才验"这五条做到**陈旧只变慢不变错**；它确实缓存计算出来的东西（generation number、corrected commit date、changed-paths Bloom filter、bitmap），前提是键装下全部输入且缺失就回退。对 kiln 的直接结论：**枚举走 `rev-list`、缓存只回答属性**，于是不需要"新鲜度"判断；缓存文件自校验；派生结果可缓存但默认先不缓存。
