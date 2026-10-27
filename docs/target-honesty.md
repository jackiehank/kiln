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

## hypothesize（2026-10-28）：`check` 只核声明侧——新增 `target-unchanged`（warn）

### 0. 主张

选 **B（松）**：`check` 新增一条 warn 规则——**动作声明了 `target:`，但声明里的某路径不在该动作的实际文件清单里**，就报一条诊断。

- 只核**声明侧**（声明了却没改）；「改了却没声明」**不算**。
- `target` 按**逗号切分**、trim、去空后比较。
- 空动作 + `target` → 所有声明路径都缺失（warn）。
- 没写 `target` → 不检查（合格）。
- 精确路径匹配；不改 `--target` 格式；warn 级，不影响退出码。

### 1. 为什么是这个形状

- collect 数据：34 个声明、32 个子集、**2 个吹牛**、**5 个漏报**；5 个漏报全是「target = doc 重点，同提交还改代码/测试/README」——即 **target 是摘要，不是 manifest**。
- 若核「漏报」（C/D）：会命中 5/34 的**正常提交**，而且「哪个文件是重点」无法自动判定。
- 若核「相等」（D）：27/34 通过，7 条 warn，其中 5 条是摘要语义。
- B：命中 2/34，**全部是异常**。
- 级别用 warn：与 `terminal-action` 同档；用 error 会让历史存量（2 条）把 `check` 变红，而 target 不实并不阻塞写侧。

### 2. 精确契约

| 输入 | 结果 |
| --- | --- |
| 无 `target` | 不查 |
| `target` 全部在文件清单里 | 通过 |
| `target` 部分缺失 | warn，**每个缺失路径一条** |
| 空动作 + `target` | 全部缺失 → warn |
| `target:` 空值 | 不查（非空才查） |
| `target: a.go, b.go`（改了两者） | 通过（切分后逐个比较） |

诊断形状：`Rule = target-unchanged`；`Action` = 该动作名；`Ref` = 缺失路径（一条一个）；级别 **warn**，退出码不变。文本形如 `⚠ <issue> target 声明了但未改动: <path>`。

匹配口径：**精确字符串相等**（git 输出仓库相对路径，target 也是；本仓 34 条均如此）。不做 `./` 归一、大小写、目录前缀、glob、rename——遇到真实样本再议。

### 3. 历史存量（要接受的噪声）

- 2 条吹牛会立刻变成 warn：`0597592`（`agent-ux-review`，缺 2 个路径）、`0bf3490`（`qualified-issue-id`，ask 且**空动作**，缺 2 个）。
- 两条 issue 现在都是 ✓，会变 ⚠ → `check` 的 warn issue 数 **3 → 5**（诊断数 +4）。
- 接受：它们确实不实，warn 不阻断；若嫌历史噪声，后续可加「只查最近 N 条」——本批不做。

### 4. 实现落点

- `vcs` 新增可选接口 `FileLog { LogFiles(want func(id string) bool) (map[string][]string, error) }`：
  - git：一次 `git log --name-only --format=%x00%H%x00`，按 `want`（带 `target` 的动作）筛；
  - jj：逐条 `jj diff --name-only -r <commit_id>`（只查带 `target` 的动作），失败**降级跳过**。
- `engine.Check` 增加一份可选的文件清单（`nil` = 不做 target 校验）；cli 在 `check` 前调用可选接口，出错就跳过本规则——**不因基础设施失败阻断 check**。
- 成本：git +1 个进程；jj +N（N = 带 target 的动作数，当前 34）。
- 测试：engine 单测（通过 / 缺失 / 空动作 / 逗号 / 无 target）；git 侧 `LogFiles` 批格式单测；cli 端到端一条（`check` 出 warn、`check --json` 带 rule 码）。
- 文档：`machine-readable-output` 的 rule 码表补 `target-unchanged`。

### 5. 检验办法（预测）

- 本仓：`check` 3 warn → 5 warn；新增 4 条诊断（`0597592` 缺 2、`0bf3490` 缺 2），其余 32 条声明零新 warn。
- 造用例：`target: a.go` 但只改 `b.go` → 1 条 warn；`target: a.go, b.go` 改两者 → 0；`target: ""` → 0；空动作带 target → warn。
- `check --json` 的 `rules` 文档与码表一致。

### 6. 反方（保留）

1. **历史 warn 变多（3→5）**：接受；两例确实不实，且 warn 不阻断。
2. **「target 是摘要，为什么只核声明侧」**：摘要的承诺就是「这些文件变了」；没提的不在承诺里。
3. **`--target` 为什么不改成 list**：改格式（本 issue 非目标）；`check` 侧切逗号即可，写入侧不受影响。
4. **精确匹配太窄**（前缀 / rename / 大小写）：本仓 34 条全是精确文件路径；先窄后宽。
5. **jj 逐条慢**：只查带 `target` 的；出错降级跳过。
6. **规则会鼓励少写 `target`**：缺声明不查，所以少写确实能绕过——但 `target` 是可选元数据，绕过只是丢信息，不是造假。

### 7. 不做

- 不做漏报 / 相等（C/D）；不改 `--target` 格式；不要求缺失声明；不做前缀 / glob / rename / 大小写匹配；不改 `path` / `show`；不加「只查最近 N 条」。

## hypothesize（续，2026-10-28）：改成写侧校验——`--target` 必须 ⊆ 本次提交的文件

### 0. 裁决（取代上一节的 check 方案）

- 校验发生在**写侧**（`do` / `commit`）：`--target` 里的每个路径（按逗号切分）都必须在**本次提交的文件**里。
- 不满足 → **exit 1**，不打印消息、不落提交（`do` 与 `commit` 同判）。
- 集合口径：`commit --only` 时以 `--only` 为准；否则以全量改动为准。`do` 以当前工作区改动为准。
- 空动作 + `--target` → 拒绝；没写 `--target` → 不校验。
- **不做** `check` 规则、`FileLog`、历史审计——上一节的 `target-unchanged` **作废**。

### 1. 为什么

- 写侧有现成集合：`commit` 本来就要算 `files`（`--only` 或全量）；`do` 在仓库里本来就会打开后端（`worktreeTip`），只在给了 `--target` 时多一次 `git status`。
- 历史审计是永久的：诊断不会消失，除非改写提交——与 `evidence-fluidity` 的「非审计」冲突。
- 写侧能拦住 2 条吹牛，对 5 条漏报**零误伤**（target ⊆ 改动）。

### 2. 精确契约

| 输入 | 结果 |
| --- | --- |
| 改动含 `docs/a.md`，`--target docs/a.md` | 通过 |
| 改动不含 `docs/a.md`，`--target docs/a.md` | exit 1：`以下 target 没有改动可记录: docs/a.md` |
| `--only docs/a.md --target internal/x.go` | exit 1（`x.go` 不在这次提交） |
| `--target a.go,b.go`，两者都在改动里 | 通过 |
| 同上，缺其一 | exit 1，只列缺失的那个 |
| 空动作 + `--target` | exit 1 |
| 无 `--target` | 通过 |
| `do`，同判；不在仓库里 | 跳过校验（无法知道），照常打印 |

匹配：归一化 `\`→`/`、去掉 `./`；精确路径；目标以 `/` 结尾（或作为目录前缀）时覆盖其下文件；改动里的 `sub/`（未跟踪目录折叠）也覆盖其下文件。

### 3. 落点

- 全部在 `write`（`internal/cli/cli.go`）：
  - `commit`：`files` 算好之后、打印清单与提交之前校验；
  - `do`：格式化后、打印前校验（仅当给了 `--target`；仓库外跳过）。
- 不新增接口，不改 `trailer` / `engine` / `check`。
- 测试：缺 target 拒绝（无提交）、`--only` 交互、逗号列表、空动作、`do` 同判、仓库外跳过、目录前缀。

### 4. 反方（保留）

1. **git-first 流程可漂移**：`git commit -m "$(kiln do …)"` 只在 `do` 时校验；之后改动 / amend 绕过。接受——写侧护栏，不是不变量。
2. **目录 / rename 仍是近似**：先精确 + 前缀；rename 未特判。
3. **与 `--only` 语义相近**：一个拒绝「路径没改动」，一个拒绝「声明没改动」；都 exit 1，消息不同。

### 5. 不做

- 不做 `check` 规则 / `FileLog` / 历史审计；不改 `--target` 格式（仍标量，比较时按逗号切）；不做 rename / glob / 大小写匹配；不改 `path` / `show`。

## apply（2026-10-28）：写侧校验 `--target`——必须 ⊆ 本次提交的文件

### 改了什么

- `write`（`internal/cli/cli.go`）：
  - `targets := f.list("target")`：按逗号切分、trim、去空。
  - `commit`：`files` 算好之后（`--only` 时是那组路径，否则全量改动）、打印清单与提交之前，用 `reportMissingTargets` 校验；缺任一 → **exit 1**，不打印、不提交。
  - `do`：打印消息之前用 `checkTargets` 比对 `Changes()`；仓库外（打开后端失败）或取不到改动时**跳过**，照常打印。
  - `targetsNotIn`：路径归一化（`\`→`/`、去掉 `./`）；精确匹配；目标作为目录前缀时覆盖其下文件；改动里的 `sub/`（未跟踪目录折叠）覆盖其下文件。
- 帮助：`--target` 的词条改成 `<文件>[,<文件>]` +「涉及的文件（必须在本次改动里）」。
- i18n：`以下 target 没有改动可记录: %s`（en 同步）。
- **不做**：`check` 规则 / `FileLog` / 历史审计（上一节作废）；`--target` 格式不变（仍标量，可写逗号）。

### 契约（实测）

| 场景 | 结果 |
| --- | --- |
| 真仓 `do … --target internal/cli/cli.go`（已改） | exit 0 |
| 真仓 `do … --target README.md`（未改） | exit 1：`以下 target 没有改动可记录: README.md` |
| 临时仓 `commit … --target b.txt`（未改） | exit 1，commits 1→1（无提交） |
| 临时仓 `commit … --target a.txt`（已改） | 通过，commits 1→2 |
| `--only docs/a.md --target docs/b.md` | exit 1（b 不在这次提交） |
| 空动作 + `--target` | exit 1 |
| 无 `--target` | 不校验 |

### 测试

- 5 个新测试：拒绝（缺文件 / 列表缺一 / 空动作）、接受（精确 / 目录 / 折叠目录）、`--only` 收窄、`do` 同判、仓库外跳过。
- `go test ./...` 全绿；`gofmt -l` 无输出。

### 边界

- 只保护 **kiln 自己写的提交**：`git commit -m "$(kiln do …)"` 只在 `do` 时校验，之后 `amend` / `rebase` 绕过。
- rename 未特判；大小写按精确比较。
- 历史那 2 条不实**不会被追**——没有审计，符合 `evidence-fluidity`。
