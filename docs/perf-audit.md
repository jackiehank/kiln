# issue: perf-audit — kiln 的性能现状与优化边界

- **domain**：cli
- **关联**：relates-to `issue-list-scaling`、relates-to `cli-consistency`、relates-to `path-batch`

## 背景

kiln 没有索引：每条读命令都从提交历史现算模型——一次 `git log`，加上按命令需要的少量 git 子进程（`show`、`--stat`、worktree 状态）。仓库小的时候这换来“零状态、永远和 git 一致”；issue 多了以后，代价才开始显形：`issue-list-scaling` 的 collect 实测 60 issue / 260 动作时 `path --all` 121 KB / 102 ms；此后 `path-batch` 把批量做成一次读全仓，`cli-consistency` 又加宽了路径（多 id 去重、重复 flag 拒绝、帮助等）。

用户提出讨论性能与优化。**先测，再谈优化**：本 issue 的 collect 要给出真实数字、增长曲线与热点，hypothesize 再决定改什么、不改什么。

## 要回答的问题

1. **绝对耗时**：7 个读命令（`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list`）在本仓（62 issue / 280 动作）各多少？`do` / `commit` 的固定开销多少？进程启动占其中多少？
2. **增长曲线**：动作数 250 / 500 / 1000 / 2000 / 4000 时，`status` / `list` / `check` / `graph` / `path --all` 怎么变？线性、超线性，还是某个命令先拐弯？
3. **热点**：时间花在进程启动、`git log`、模型构建、解析 trailer、渲染，还是每个动作的额外 git 子进程？
4. **子进程模型**：每条命令实际 spawn 几个 git 进程？`path --all --stat` 是不是每个动作一个？`worktree list` 每个区几个？
5. **输出体量**：文本 / JSON 大小随 issue 数怎么长？`path --all` 什么时候大到不适合一次读？
6. **重复工作**：同一命令内是否重复读配置、重复 `git log`、重复排序、重复判定 `done`？有没有 O(n²)？
7. **优化点与收益**：一次遍历合并、批量 `--stat`、减少 alloc / 字符串拼接、并发 git——各自预期省多少？哪些与“无索引”设计冲突？
8. **预算**：要不要定一个“≤1000 动作 p95 < X ms”的目标，还是只保证“线性 + 千级可用”？

## 候选（测完再选）

- **A 不做优化**：只把数字与上限写清楚，承诺“千级动作可接受”；
- **B 低风险热修**：`--stat` 批量化、去掉重复遍历 / 大分配，不改契约；
- **C 结构性改动**：索引 / 缓存 / 并发 / 常驻进程——与无索引设计冲突，需单独立项。

## 非目标

- 不引入索引、数据库、daemon 或常驻进程；
- 不为性能改 JSON 契约或文本视图；
- 不在测量前先做并发；
- 不优化没有被测量证明的路径。

## 验收（collect 完成时）

- 真仓每个读命令 ≥5 次运行的 min / median；
- 合成仓 5 个规模的增长表（动作数 vs 耗时）；
- 每个命令的 git 子进程数与各阶段占比；
- 文本 / JSON 体量表；
- 热点排序 + 候选（必须改 / 可选 / 不做），留给 hypothesize 定性。
