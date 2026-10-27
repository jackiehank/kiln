# issue: target-honesty — 用 diff 反查 `target:` 声明是否属实

- **domain**：design
- **关联**：derived-from `action-diff`

## 背景

`target:` trailer 表示"这个动作涉及的文件"。它是**自述**——写的人说什么就是什么，`check` 从不核对。但 diff 就在同一条提交里，**声明完全可以和现实对照**：

- 声明 `target: internal/cli/cli.go`，实际只改 `docs/` → **声明不实**；
- 改了 5 个文件一个字没提 → 至少值得一个提示。

这正是 `action-diff` 里被 **候选 D** 命名、但为"先聚焦变更展示"而**未落地**的那部分。它是独立价值：**让 `check` 第一次有了"对着真实代码校验"的能力**（现有 `check` 只校验 trailer 之间的关系完整性，从不碰"声明 vs 现实"）。

## 要回答的问题

1. **要不要做？** `target` 是自述元数据，让 `check` 拿真实 diff 去核，值不值？会不会太啰嗦（几乎每次提交都能挑出点不一致）？
2. **核到什么程度？** 三种严格度：
   - **严**：声明的文件集合必须**恰等于** diff 的文件集合（多一个、少一个都告警）；
   - **中**：声明必须是 diff 的**子集**（"声明了却没改"= 不实；"改了却没声明"= 只提示）；
   - **松**：只在"**声明了却没改**"时告警，漏报（改了没提）不究。
3. **缺失声明怎么办？** 没写 `target:` 的动作算无信息、还是默认合格？多文件动作要不要强制声明？
4. **告警级别**：算 **error（阻断，`check` 非零退出）** 还是 **warn（提示）**？现有 `check` 里"缺终点动作"是 warn、"父问题不存在"是 error——`target` 不实该落哪一档？
5. **匹配口径**：`target:` 是单值还是列表？路径的大小写 / 分隔符 / glob / 目录前缀（写了 `internal/cli/` 算不算覆盖 `internal/cli/cli.go`）怎么算"对上"？重命名（rename）又怎么算？

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 不做 | `target` 维持纯自述 | 丢掉一条可校验规则 |
| **B** 松 | "声明了却没改" → warn | 误报最少，但只抓吹牛、不抓漏报 |
| **C** 中 | 声明 ⊆ diff；多声明 → warn，漏报 → warn | 双向提示，需定义"漏报"的可接受阈值 |
| **D** 严 | 声明 == diff 才算过 | 误报高（无关文件、格式化都会触发） |

> 倾向：**先落 B/C 之间的"warn 级、只核声明侧"**——先抓"吹牛"（声明了却没改），把"漏报"留给后续；级别用 warn，避免阻断正常提交。待验证。

## 与现有 `check` 的关系

现有规则：① 必须含起点动作；② 末尾必须是终点动作（warn）；③ `derived-from` 父存在；④ `spawns` 子存在。本 issue 若落地，将是**第一条"对着真实提交内容"校验的规则**——`check` 要从"只看 trailer 语义"扩到"读 git diff"。这会影响实现落点：`engine.Check` 现在只吃 `[]git.Commit`，很可能需要引入 diff / 文件清单数据。

## 非目标

- 不改 `path` / `show`（变更展示已在 `action-diff` 定稿）。
- 不改 `target:` 的格式本身；只讨论 `check` 是否核、怎么核、多严。

## 备注

来自 `action-diff` 的**候选 D**：该 issue 定稿时明确"先聚焦变更展示"，D 拆出来单独评估。呼应 `action-diff` 的业内调研结论——"用某条提交的 diff 反查该提交 trailer 的自述"**没有现成标准做法**，若做，等于把需求可追溯性（traceability）的精神搬进 trailer 层，属**新意**而非抄现成方案。

## collect（2026-10-28）：diff 能算，但 `target` 是「意图摘要」——34 个带声明的动作实测

### 0. 可行性：怎么算

- **git**：`git show --name-only --format= <hash>`（或 `git diff-tree --no-commit-id --name-only -r <hash>`；要看 rename 用 `--name-status`）。空动作 → 空列表。
- **jj**：`jj diff --name-only -r <commit_id>`；能否批取未验证（模板函数待确认），先按动作取。
- **kiln 侧**：`StatLog` 已经示范「一次 `git log` 批量取 diffstat」；文件清单可以照做（`git log --name-only --format=%x00%H%x00`）。`engine.Check` 目前只吃 `[]Commit`，要把文件清单带进去——新增可选接口，或在 cli 侧预取后传入。

### 1. 实测（HEAD `bcd56cd`，300 个动作）

| 指标 | 数值 |
| --- | --- |
| 声明了 `target` | **34 / 300（11%）** |
| 单值 / 逗号列表 | 26 / **8** |
| 声明集合 == diff 集合 | 27 |
| 声明 ⊆ diff（没吹牛） | 32 |
| **声明了却没改（吹牛）** | **2** |
| **改了却没声明（漏报）** | **5** |

- **吹牛 2 例**：`0597592` 声明 `README.md`、`docs/mvp-plan.md`、`docs/agent-ux-review.md`，实际只改了 `agent-ux-review.md`（疑似历史改写/拆分后的残留，不深究）；`0bf3490` 是**空动作**却声明 `internal/cli/cli.go`、`README.md`。
- **漏报 5 例**：都是「target = 该 issue 的 doc，同一提交还改了代码 / README / 配置」——`f42f114`（还改了 relation-revision 等 3 篇 + 一个测试）、`ac5de27`（还改了 README×2、cli.go、cli_test.go、catalog.go）、`5b37df0`（还改了 mvp-plan）、`8d80eff`（还改了 cli/engine/model 与测试）、`3f15978`（还改了 .kilnconfig、README、config 与测试）。

### 2. 口径：diff 是 manifest，`target` 是摘要

- diff 回答「**到底改了哪些文件**」；`target` 回答「**作者认为这一步的重点是哪个**」——多文件动作里，哪个是重点 git 不知道。
- 因此「改了却没声明」是**常态**（最近 5 个结论/落地提交都这样：doc 是重点，代码/测试/README 顺带），而「声明了却没改」才是异常。
- 这是本轮最重要的结论：**只核声明侧（松）**才有语义；核「漏报」会把正常提交判成不实。

### 3. 格式：逗号列表是事实格式

- 34 个声明里有 **8 个逗号列表**（例：`target: README.md, docs/mvp-plan.md, docs/agent-ux-review.md`）。
- 但 `--target` 是**标量**、不切逗号（`f.get` 不是 `f.list`），help 也写「单个」——所以那些是整串写入的，不是解析出来的列表。
- 校验若按列表比较，需要决定：**在 `check` 里切逗号**（不改写入格式），还是把 `--target` 改成 list flag（会改格式，本 issue 的非目标）。

### 4. 边界与未观测

- 目录前缀 / glob / 大小写 / rename：34 个声明里**都没出现**（全是文件路径；空动作 1 个）。
- 缺声明：266 个动作没有 `target`；默认应算**合格**，否则等于要求全仓补声明。
- 空动作带 `target`：`0bf3490` 一例，应算吹牛。
- jj 侧批取与 git 的 rename 语义未实测。

### 5. 留给 hypothesize

- **B（松）**：只 warn「声明了却没改」，按逗号切分比较；空动作带 `target` 也 warn；级别 warn（与「缺终点」同档，不阻断）。
- **C / D**：会命中 5/34 的漏报（包括最近自己的提交），噪声高，且与「target 是摘要」的语义冲突。
- 实现落点：`engine.Check` + 文件清单数据源（批取优先，jj 回退逐条）；缺声明不要求。
- 格式：默认在 `check` 侧切逗号，写入侧不动（`--target` 仍是标量）。
