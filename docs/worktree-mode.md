# issue: worktree-mode — 要不要把「开不开工作区（worktree）模式」做成配置项

- **状态**：已完成（conclude）
- **domain**：design

## 背景

kiln 现在**强制**"**主干只读 / 只合并，动作只在专用区提交**"：`kiln commit` 在主干会被**结构性拒绝**（`refuseInMain` → exit 1），唯一豁免是显式的 `--force`。这条护栏是 `cli-necessity` 的 U 方案产物——目的正是让并行 agent 的改动**结构上**无法互相污染。

护栏的判定也**写死在代码里**：后端各自实现 `IsDedicated()`（git 看 `.git` 是文件否；jj 看是否非 `default` 工作区），`commit` 里**没有任何读配置的分支**。

而配置刚刚定稿为**分组式、预留扩展**的 `.kilnconfig.jsonc`（`actions` 组 + 顶层 `vcs`），`config-home` 明确写着"以后还会长出别的配置"。"开不开工作区模式"正是这样一个候选：**它该不该从"恒开"变成"可配"？**

> 摩擦实例：在本仓连着开几个 issue 时，每个动作都得先 `kiln workspace new` 再 `cd` 进去提交；想"就在主干改一下"的人只能记着加 `--force`。

## 要回答的问题

1. **要不要做**：把"开不开工作区模式"做成配置项，还是维持"恒开"？关掉之后，kiln 在并行场景"结构上杜绝污染"这一卖点还剩多少？
2. **语义强度**：关掉 = **允许在主干提交**，还是只**降级为警告**？两态（`on` / `off`）还是三态（`off` / `warn` / `error`）？
3. **键名与位置**：`worktree`（本 issue 的措辞）/ `workspace`（对齐 `kiln workspace` 命令语汇）/ `isolation`（直指"隔离"）？放**顶层**（与 `vcs` 平级）还是**成组**（为将来留位）？
4. **取值形态**：字符串枚举（如 `auto` / `require` / `off`，对齐 `vcs` 的风格）还是布尔？将来若要加 `warn` 会不会逼着改类型？
5. **与 `--force` 的关系**：配置是"把 `--force` 常态化"，还是与它**互补**（配置管默认、`--force` 管一次性豁免）？
6. **默认值**：新仓默认**开**（安全优先）——是否无争议？（对齐"宽容输入、严格输出"与 U 方案的立场。）
7. **作用域**：只管 `commit` 的护栏，还是也影响 `workspace new` / 播种（`.worktreeinclude`）/ `status` 的呈现？
8. **后端一致**：git 与 jj 的 `IsDedicated` 判定机制不同，配置项要在两后端**同语义**吗？

## 候选

### 要不要做

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 维持恒开 | 护栏写死，`--force` 是唯一豁免 | 零新增面；"就在主干干"的仓库每次要 `--force` |
| **B** 两态开关 | 新增一个键，默认"开" | 护栏变可选——关掉即自弃"结构上杜绝污染"这一核心卖点 |
| **C** 三态（off / warn / error） | 关 / 只警告 / 拒绝 | 最灵活；多一档语义与测试面 |

### 键名与位置

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **P** 顶层 `worktree` | 与 `vcs` 平级，单键 | 键名与 CLI 的 `workspace` 语汇不一致 |
| **Q** 顶层 `workspace` | 对齐 `kiln workspace` 命令语汇 | 名字指向"物件"（区）而非"行为"（是否强制隔离），语义略偏 |
| **R** 成组 `workspace.{mode}` | 顺着分组 schema，为将来留位 | 单键也开一组，略重 |

### 取值

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **X** 字符串枚举 `auto` / `require` / `off` | 对齐 `vcs` 的字符串风格 | 三值语义要逐个定义 |
| **Y** 布尔 `true` / `false` | 简单直白 | 将来要加 `warn` 就得改类型（破坏性） |

## 待定

- **键名 / 位置 / 取值**三者的组合定稿（P / Q / R × X / Y）。
- **关掉后 `workspace new` 是否仍可用**：倾向**保留**——工具不变，只变默认护栏强度；即"想隔离的人仍可隔离，只是不再强制"。
- 与 **`--force`** 的关系（见问题 5）。
- 是否要在 `kiln -h` 与 `config path` 播种的默认配置里**体现**这个键（可发现性）。
- 若做三态，`warn` 档的出口（stderr 一行？还是输出里带标记？）。

## 非目标

- 不改工作区的**布局**（`.kiln/ws/<issue>`）与**播种**（`.worktreeinclude`）——那是 `cli-necessity` / `worktree-include` 的定论。
- 不新增 VCS 后端；不动 `vcs` 的 `auto | git | jj` 语义。
- 本轮不改代码：只开题、定清问题与候选。

## collect：别的工具怎么处理「隔离要不要强制」与「护栏强度」

> 一轮联网调研（厂商官方文档为主）。目的：看有没有工具把"隔离"做成**可关**的配置、以及"护栏强度"在行业里怎么表达，再看本 issue 的候选该怎么收敛。来源见本节末。

### 1. 有——而且直接就是「默认隔离 + 可关」

**Claude Code 的 `worktree.bgIsolation`**（v2.1.143+）是**与本 issue 同形**的先例：

| 取值 | 语义 |
| --- | --- |
| `"worktree"`（**默认**） | 背景会话**禁止改主干**（Edit/Write 被挡），直到 `EnterWorktree` |
| `"none"` | **允许直接改工作副本**，不再先进 worktree |

官方原文的理由句几乎与 kiln 的场景重合："**To turn off worktree isolation for a repository where git worktrees are impractical**, set `worktree.bgIsolation` to `"none"`"——而且这个键写在**项目设置**里（随仓库、随版本），不是命令行开关。

它的形状同时落中本 issue 的三个候选：**键名 `worktree`** → 候选 **P**；**成组**（同组还有 `worktree.baseRef` = `"fresh"` / `"head"`）→ 候选 **R**；**字符串枚举**（`"worktree"` / `"none"`）→ 候选 **X**。

> 一句话：**"隔离默认开、给不需要的仓库一条显式出路"**——这套形态已经被主流 agent 工具落地，kiln 的候选 B/C **不是空想**。

### 2. 「护栏强度」的行业惯例 = 分级

| 工具 | 级别 | 放哪 | 关键语义 |
| --- | --- | --- | --- |
| **ESLint** | `off` / `warn` / `error`（或 `0`/`1`/`2`） | 配置里的 `rules` | **`warn` 不影响退出码，`error` 影响**——正是"只警告"与"拒绝"的分野 |
| **rustc / Cargo** | `allow` / `expect` / `warn` / `force-warn` / `deny` / `forbid` | `Cargo.toml` 的 `[lints.<tool>]` 表 | 六级；**警告与拒绝本就是相邻两档** |
| **git** | 布尔 | `advice.*`，**默认 true** | "不需要提示就设 false"——**纯提示型**的开关用布尔 |

三点收敛：

1. **"只警告 vs 拒绝"是真实且普遍的诉求**（ESLint 的 warn/error、rustc 的 warn/deny 都在同一根轴上）→ kiln 若做，**三态（C）比两态（B）更贴行业**，但多一档语义与测试面。
2. 这类强度**惯例写在配置里**（ESLint `rules` / Cargo `[lints]`），**不是命令行 flag**——支撑"做成配置项"这个大方向。
3. **多级用枚举、纯提示用布尔**：`warn` 若要加，**布尔（候选 Y）就得改类型**（破坏性）→ 偏 **X**。

### 3. 默认值：行业普遍"宽松默认、严格可选"——kiln 的"恒开"是反的

- TypeScript `strict` **默认关**；ESLint 规则**默认关**；rustc 只有少数 correctness lint 默认 `deny`、**多数是 `warn`**；git `advice.*` 默认 true，但那是**提示**不是**拦截**。
- → **惯例是"少拦，让你自己 opt-in 严格"。** kiln 的护栏（恒开、拒主干提交）**反其道**。
- **但本例站得住**：kiln 拦的是**"污染 / 归属错乱"这类静默正确性故障**（`cli-necessity` / `evidence-fluidity` 的关切）——这正是"**默认安全**"该开的地方，与"lint 严格"的取舍不同。所以**默认值"开"（问题 6）无争议**，与 Claude Code 的 `bgIsolation` 默认 `"worktree"` 一致；要给的只是那条**显式出路**。

### 4. opt-in vs 强制：各工具的立场

| 工具 | 立场 |
| --- | --- |
| **git / jj** | worktree / workspace **纯 opt-in**，从不强制（也不替你看管"只在 worktree 干活"） |
| **Claude Code** | 交互式 `--worktree` **opt-in**（每会话一选）；**背景会话 / 子代理默认隔离**，可关 |
| **Codex** | managed worktree 是**聊天默认**（"By default, chats use a Codex-managed worktree"） |

→ kiln 现在（`workspace new` 显式建区 + `commit` 结构性拒主干）**严于 git/jj**，**硬于** Claude Code 的"默认隔离"——因为它把"提醒"升级成了"**拒绝**"。

### 5. 对候选的收敛

| 问题 | 收敛 | 依据 |
| --- | --- | --- |
| **做不做（A/B/C）** | **倾向 B 或 C，而非 A** | 主流已落地"隔离默认开 + 可关"（`worktree.bgIsolation`）；A（恒开）不是行业共识，且让"就在主干干"的仓库每次手记 `--force` |
| **两态 vs 三态** | **C（off / warn / error）更贴行业**，代价是多一档 | ESLint warn/error、rustc warn/deny 证明"警告 vs 拒绝"是普遍需求 |
| **键名** | **`worktree`（P）有直接先例** | Claude Code 的 `worktree.*`；`workspace`（Q）是 kiln 自有语汇但指向"物件"而非"行为" |
| **位置** | **成组（R）有直接先例**；单键平铺（同 `vcs`）更省但不留位 | `worktree.baseRef` + `worktree.bgIsolation` 同组 |
| **取值** | **字符串枚举（X）** | Claude Code 用 `"worktree"` / `"none"`；布尔只适"纯提示型"，且加档即改类型 |
| **默认值** | **默认"开"（隔离）——无争议** | 与 `bgIsolation: "worktree"` 默认一致；拦的是静默正确性故障 |
| **与 `--force`** | **互补**：配置管默认强度，`--force` 管一次性豁免（保留） | Claude Code 只有设置、无 per-命令豁免；kiln 已有 `--force`，两者不冲突 |

> 来源：[Claude Code：Settings（`worktree.bgIsolation` / `worktree.baseRef`）](https://code.claude.com/docs/en/settings) ｜ [Claude Code：Agent view（How file edits are isolated——关掉 worktree 隔离）](https://code.claude.com/docs/en/agent-view) ｜ [Claude Code：Run parallel sessions with worktrees](https://code.claude.com/docs/en/worktrees) ｜ [ESLint：Configure Rules（规则严重性 off / warn / error）](https://eslint.org/docs/latest/use/configure/rules) ｜ [rustc：Lint Levels（allow…forbid）](https://doc.rust-lang.org/stable/rustc/lints/levels.html) ｜ [Cargo：The Manifest Format（`[lints]` 表）](https://doc.rust-lang.org/cargo/reference/manifest.html) ｜ [Cargo：Configuration（lint 级别）](https://doc.rust-lang.org/stable/cargo/reference/config.html) ｜ [git：git-config（`advice.*` 默认 true）](https://git-scm.com/docs/git-config)

## hypothesize：默认隔离、可关闭；命令统一为 `worktree`；`--force` = 一次性豁免（按用户决策定稿）

> 一句话：**配置里加一个顶层 `worktree` 开关（默认 `"on"`），命令面把 `workspace` 统一改名为 `worktree`，`--force` 明确定义为"一次性豁免"。**

### 已定的三条（用户）

1. **默认隔离，可关闭** —— 隔离模式**默认开**；仓库可在配置里**关掉**它（关掉后主干可直接提交）。
2. **命令名统一为 `worktree`** —— `kiln workspace` → **`kiln worktree`**，让**命令 / 配置键 / git 的 `worktree` 说法三处统一**。
3. **`--force` = 一次性豁免** —— 隔离开着时，`--force` 只豁免**当次**命令，不改配置、不留持久状态；与"配置管默认"**互补**。

### 由这三条推出的子决策（需确认）

#### a. 键位与取值：顶层 `worktree`，取值 `"on" | "off"`，缺省 `"on"`

```jsonc
{
  "actions": { "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
               "start": "ask", "terminal": ["conclude", "apply"] },
  "vcs": "auto",
  "worktree": "on"   // "on"（默认，强制专用区）| "off"（允许在主干提交）
}
```

- **键位**：先做**顶层单键**，与 `vcs` 同款；将来若长出 `worktree.baseRef` 一类子项，再收成组（collect 里 `worktree.*` 的成组先例留作扩展点）。
- **取值**：字符串 `"on" | "off"`（对齐 `vcs` 的字符串风格）；**缺省 `"on"`**。
- **不用布尔**：① 与 `vcs` 的字符串风格不一致；② 将来若要加 `"warn"` 一档（collect 里有 ESLint / rustc 的先例）就得**改类型**。
- **严格校验**：非法值 / 拼错的键 → **报错**（沿用 `DisallowUnknownFields` 的严格取向）。
- **需确认**：取值用 `"on" | "off"`，还是 `"require" | "off"`，或照 Claude Code 的 `"worktree" | "none"`。

#### b. 关掉后的行为：不拒绝、也不提示

- `worktree: "off"` → `commit` 在主干**直接提交**（不再走 `refuseInMain`）。
- **是否仍打一行提示**：倾向**不打**（"关"就关干净）；"只提示（`warn`）"这一档**不在本轮取值集合内**。
- **需确认**：关掉后要不要保留一行提示。

#### c. 改名的作用域：只改「用户可见面」，内部标识符与目录名不动

- **改**：命令 `kiln workspace new | list` → `kiln worktree new | list`；帮助文本、`usageCommands`、输出文案与错误提示中的命令名。
- **不动**：内部标识符（`vcs.Workspace` 接口、`WorkspaceInfo`、`NewWorkspace` / `ListWorkspaces`）、目录 `<repo>/.kiln/ws/<issue>`（`ws` 是**路径长度**考量，见 `cli-necessity`）。
- 理由：改名的目的是让 **命令 / 配置 / git 三处说法统一**（用户语汇），不是重命名整个代码库。
- **需确认**：是否连内部符号一起改名（那会牵动 `vcs` / `cli` 的多处命名与文档）。

#### d. `--force` 保留原名，语义写明「一次性豁免」

- 名字**不改**（`--force` 已是 `commit` 的既有 flag；collect 也确认"配置管默认、`--force` 管一次性"）。
- 语义：**只作用于当次命令**；`worktree: "off"` 时 `--force` **无实际作用**（已放行）但仍**接受且不报错**（幂等）。
- **需确认**：是否要改成更直白的 `--allow-main` / `--once`。

#### e. `kiln worktree new` 仍可用；播种的默认配置带上新键

- 关掉隔离**不删除**建区能力——"想隔离的人仍可隔离，只是不再强制"（ask 的待定倾向，保留）。
- `kiln config path` 播种的**默认配置**要含 `worktree` 键（**带注释**），让新仓可见、可发现。
- 本仓 `.kilnconfig.jsonc` 是否**显式**写上 `worktree: "on"`：**可选**（缺省即 `"on"`）；倾向**写上并加注释**，当作"这就是默认行为"的自述。**需确认**。

#### f. 旧命令名不留别名（硬切）

- 倾向**不留** `workspace` 别名：与 `.kiln.json` → `.kilnconfig.jsonc` 的**硬切**取向一致，避免两套语汇长期并存。
- **需确认**：是否留一个过渡别名。

### 影响面（留给 test 落地）

- **`internal/config`**：新增顶层 `worktree` 的解析与校验（缺省 `"on"`；非法值报错）；`Ensure` 播种的默认内容加一行注释与键。
- **`internal/cli`**：`refuseInMain` 改为**读配置**（`"off"` 放行，`--force` 仍豁免当次）；命令面改名 `workspace` → `worktree`（`new` / `list` + 帮助 + 文案）。
- **`kiln -h`**：命令表与配置说明同步（`worktree` 键 + `worktree new | list`）。
- **文档**：`docs/mvp-plan.md` §2.3 配置示例加 `worktree` 键、命令面同步；§10 条目在 conclude 时改"已落地"。
- **历史文档里的 `kiln workspace new`**（`cli-necessity.md` / `worktree-include.md` / `mvp-plan` 等）：改成 `kiln worktree new`，或加一行"旧名 → 新名"的注记。**需确认**改写方式。
- **单测**：`config` 新键的解析 / 缺省 / 非法值；`cli` 的 `worktree ∈ {on, off}` × `--force ∈ {有, 无}` 四组合 + 命令面改名后的用法与报错。

## test：落地——`worktree` 配置键 + 命令统一为 `worktree` + `--force` 一次性豁免

> 本轮把 hypothesize 的定稿（用户确认 a–e）**落成代码并真机验证**。

### 1. 用户已确认的子决策（a–e；f 按倾向）

| # | 决定 |
| --- | --- |
| **a** | 顶层 `worktree`，取值 `"on" \| "off"`，缺省 `"on"` |
| **b** | 关闭后**不拒绝、也不提示** |
| **c** | **只改用户可见面**；内部标识符（`vcs.Workspace` 等）与目录 `.kiln/ws/<issue>` 不动 |
| **d** | **保留 `--force`** 原名，语义写明"一次性豁免" |
| **e** | `kiln worktree new` 仍可用；**播种的默认配置列出全部可配置项**（声明式，一眼看清配置状态） |
| **f** | （未点名）按倾向：**不留旧名别名**（硬切） |

### 2. 落地内容

- **`internal/config`**：新增顶层 `worktree`（`file.Worktree` / `Config.Worktree`）；`worktreeMode()` 归一化（容忍大小写与空白）并**严格校验**（非法值报错）；`Config.Isolated()` 以"非 `off` 即隔离"（零值 = 默认开）；`defaultFile` 播种内容**补齐 `worktree` 键**——默认配置现在把 `actions` / `vcs` / `worktree` **全部列出**。
- **`internal/cli`**：`refuseInMain` 先看 `e.isolated()`，`worktree=off` 直接放行；`--force` 仍是"跳过护栏"的一次性豁免；命令面 **`workspace` → `worktree`**（允许列表 / 分发 / `usageCommands` / 用法与拒绝文案）；`-h` 增 `worktree 默认: on | off` 一行。
- **本仓**：`.kilnconfig.jsonc` 显式写上 `"worktree": "on"`。

### 3. 真机验证（临时 git 仓，新二进制）

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | 无 `worktree` 键 → 主干 `kiln commit` | **拒绝**（exit 1）；提示改为"先 `kiln worktree new <issue>` … 加 `--force`（一次性豁免）" |
| 2 | 同上 + `--force` | **放行**（exit 0）——一次性豁免生效 |
| 3 | 配置 `worktree: "off"` → 主干 `kiln commit` | **放行**（exit 0），**无需 `--force`** |
| 4 | 配置 `worktree: "sometimes"` | `配置错误: ... 非法 worktree: "sometimes"（可选 on / off）`，exit 1 |
| 5 | `kiln worktree new t9` / `kiln worktree list` | 建区成功；list 显示主干 + `专用 t9` |
| 6 | 旧名 `kiln workspace list` | **`未知命令: workspace`**，exit **2** |
| 7 | `kiln help` | 命令表为 `kiln worktree new / list`；含 `worktree 默认: on \| off` 一行 |

### 4. 测试

- **单测**：`config` 新增 2 例（`TestReadParsesWorktreeMode`：`off` / `on` / 带空白大小写的 ` OFF ` / 缺省 on；`TestReadRejectsUnknownWorktreeMode`），并在 `TestEnsureWritesCommentedDefaults` 断言播种含 `worktree` 且读回为 on、`TestReadFallsBackToDefaultsOutsideRepo` 断言缺省 on。`cli` 新增 3 例（`worktree=off` 时主干提交放行；usage 只提 `worktree`、不提 `workspace`；旧名 `workspace` 报未知命令 exit 2）。`gofmt -l` 空、`go vet` / `go test ./...` **全绿**。

### 5. 发现 / 取舍

1. **`--force` 与配置互补落地**：护栏的两条出路各司其职——配置管**默认**（`off` 长期放行）、`--force` 管**当次**（`on` 时的一次性豁免）；`off` 下 `--force` 依然接受且无副作用。
2. **"声明式配置"按用户要求做足**：播种的默认配置现在**列出每个可配置项**（`actions` / `vcs` / `worktree`），读一眼即知仓库配置状态；本仓也照此显式写了 `worktree: "on"`。
3. **改名落点在用户可见面**：内部 `vcs.Workspace` / `WorkspaceInfo` / `.kiln/ws/<issue>` **一律未动**——改名目标是"命令 / 配置 / git 三处说法统一"，不是重命名代码库。**代价**：源码里 `e.workspace*` 方法名仍叫 workspace（用户不可见，可接受）。
4. **历史文档不改写**：本 issue 之前各文档（`cli-necessity` / `config-home` / `worktree-include` 等）里的 `kiln workspace …` 是**当时的真实记录**，保留原样；**living spec（`docs/mvp-plan.md`）已更新为新名**。→ 换言之，**`kiln workspace` 现在是旧名**，会报未知命令。
5. **硬切、无别名**：`workspace` 不再被识别（exit 2），与 `.kiln.json` → `.kilnconfig.jsonc` 的硬切一致。

### 未做 / 待定

- **`warn` 中间档未采纳**（collect 里的 ESLint / rustc 先例）——用户定的是两态"可关闭"。
- **历史文档里的旧命令名未改写**（见取舍 4）；若要统一，可在 conclude 决定。

## test（续）：`worktree` 命令补上 `remove`——三件套齐了

> 用户问"worktree 命令是不是还缺一个 remove"→ 确认缺，并**并入本 issue**（同一"命令面统一"议题）。

### 1. 缺口与现状

- 原命令面只有 `new | list`——**没有任何在 kiln 里拆区的动作**；每轮收尾都退回**后端方言**手工两步（`git worktree remove …` + `git branch -d kiln/<issue>`；jj 则 `jj workspace forget`）。
- 这正是 `cli-necessity` 说"该由 `vcs.Workspace` 吞掉"的东西——`new` / `list` 已吞，`remove` 没吞。
- **顺带堵坑**：[cli-necessity.md](cli-necessity.md) 记的「区删了而 `kiln/<issue>` 分支尚存 → 下次 `new` 因 `-b` 冲突失败」——**连分支一起删的 `remove` 正是这条坑的解**。

### 2. 设计（本轮定，可再调）

| 决定 | 取值 |
| --- | --- |
| **命名** | **`remove`**——对齐 git 与既有措辞（不为 `rm` / `delete` 另起名） |
| **寻址** | **按 issue**（`kiln worktree remove <issue>`），不是按路径——"命名即绑定" |
| **接口** | `vcs.Workspace` 增 `RemoveWorkspace(issue, force)`；共用 `vcs.AreaFor(areas, issue)` 查找（**查后端注册表，不是存索引**） |
| **git 的两半** | `worktree remove` **且** `branch -d` / `-D` `kiln/<issue>`——只删区不删分支会留下 `-b` 冲突坑 |
| **jj 的两半** | `jj workspace forget <issue>` **且** `os.RemoveAll(<区目录>)`——实测 `forget` **只注销、不删目录**（区里文件仍在） |
| **护栏（"还有工作"）** | **默认拒绝，`--force` 才丢**：git 侧 = `worktree remove` 自带的脏区检查 **+** 我们前置的**分支未合并**检查；jj 侧 = 我们查 `jj -R <区> diff --summary`（非空即脏），因为 `forget` 自己**不问任何问题** |
| **前置检查（git）** | 未合并检查**放在删之前**：先删区再删分支会**把未合并分支搁浅**（正是上面那条坑），所以不 `--force` 时**先拒、不删** |

### 3. 真机验证

| 后端 | 实验 | 结果 |
| --- | --- | --- |
| **git** | 干净且已合并 → `remove t1` | exit 0；**区没了、`kiln/t1` 也没了** |
| **git** | 区里有未提交改动 → `remove t2` | **拒绝**（git 原话 "contains modified or untracked files, use --force to delete it"），区**完好** |
| **git** | 同上 + `--force` | exit 0，区已删 |
| **git** | 分支**未合并**（`git rev-list --count kiln/t3 --not master` = 1）→ `remove t3` | **拒绝**：`分支 kiln/t3 还有未合并的提交（要丢弃请加 --force）`，区**完好**（未搁浅） |
| **git** | 同上 + `--force` | exit 0，区与分支**都**清掉 |
| **git** | 不存在的 issue → `remove nope` | exit 1：`没有 issue 为 "nope" 的专用区` |
| **jj（colocated）** | 干净 → `remove t1` | exit 0；区目录已删、`jj workspace list` **不再列出 t1** |
| **jj** | 区里有改动 → `remove t2` | **拒绝**：`专用区还有未提交的改动: t2（要丢弃请加 --force）`，目录完好 |
| **jj** | 同上 + `--force` | exit 0，目录已删 |

### 4. 测试

- **单测**（`internal/cli/cli_test.go`）：`worktree remove` 把 issue 交给后端并报告（`TestWorktreeRemoveDeletesTheArea`）；**`--force` 确实抵达后端**（`TestWorktreeRemoveForwardsForce`，用一个"force 前必拒"的假后端）；缺 issue 是用法错误（`TestWorktreeRemoveRequiresAnIssue`）；`usage` 断言补上 `kiln worktree remove`；假后端 `workspaceRunner` 补上新接口方法。`gofmt -l` 空、`go vet` / `go test ./...` **全绿**。

### 5. 取舍

1. **`--force` 一词两用**：`commit --force`（一次性豁免主干护栏）与 `worktree remove --force`（丢弃区里的工作）——都是"我知道后果、放行当次"，语义一致，故**同名、不另造**。
2. **护栏的两半各自归属**：git 的脏区护栏**白拿**（git 自己有），未合并护栏**必须我们前置**；jj 则**两半都要我们做**（`forget` 既不问、也不删）。→ 正是 `vcs.Workspace` 该吸收的差异。
3. **仍未做**：`worktree new --adopt`（搬未提交改动）——`cli-necessity` 提过、一直未落地，仍是独立缺口。

## test（再续）：worktree 工作流的手感——`.gitattributes` + 两条零成本提示

> 起因：用户问"每次开区、收区，整个过程还有什么不顺畅的"。以下是一份**基于本会话实际跑过 8 轮**的自查，本轮落其中最疼的三项，余项记在末尾。

### 1. 自查：跑这一圈的摩擦（按代价排）

| # | 摩擦 | 证据（本会话） |
| --- | --- | --- |
| 1 | **站在区里，却必须用主干那份二进制**，且**无处提示** | 本轮**真踩**：主干二进制比区内源码旧一轮，新加的配置键让它直接 **`配置错误: … unknown field "worktree"`**，只能改用区里临时 build 的那份 |
| 2 | 每轮一段**行尾归一化**脚本 | 跑了约 **8 次**（`core.autocrlf=true` → 新区检出 CRLF → `gofmt -l` **误报整棵树**） |
| 3 | 收尾**手敲 + 顺序敏感** | `merge` → `remove` → `branch -d`；顺序反了会**搁浅未合并分支** |
| 4 | git 后端提交前**必须 `git add -A`** | jj 不需要；忘了就 "nothing to commit" |
| 5 | **旧名硬切、零引导** | 本轮**自己踩**：`kiln workspace new` → 未知命令 |
| 6 | `list` 不告诉你**哪个区能安全删** | 脏 / 未合并只在 `remove` 被拒时才暴露 |
| 7 | 跨目录 **cwd** 管理 | 每轮 `Push-Location` / `Pop-Location` |

> 一句话：kiln 管住了**区的生命周期**（`new` / `list` / `remove`），但**"在区里干活"的手感**仍全靠外部约定——cwd、用哪个二进制、要不要暂存、行尾。

### 2. 本轮落地（用户挑的 1 / 2 / 3）

- **仓库根 `.gitattributes`（`* text=auto eol=lf`）**——一次加、永久消灭第 2 条：**新区检出直接是 LF**，不再需要手工归一化；且实测**不会把主干现有的 CRLF 工作树判成 modified**（`eol=lf` 让 git 对工作树行尾不再敏感）。
- **`kiln worktree new` 输出补一句**——把**正在运行的二进制绝对路径**打出来（`os.Executable`），并写明"区内没有 kiln 二进制，用主干那份（改了 Go 源码需先重建它）"→ 治第 1 条。**该提示已撤回**，见 test（三续）。
- **旧名引导**——`未知命令: workspace（已改名为 `worktree`）`（`retiredCommands` 一张小表；普通未知命令照旧只报名字）→ 治第 5 条。

### 3. 验证

| 实验 | 结果 |
| --- | --- |
| 临时仓 `kiln worktree new t1` | 输出含 `提交提示: 区内没有 kiln 二进制，用主干那份（改了 Go 源码需先重建它）: <绝对路径>`（**该行已撤回**，见 test（三续）） |
| `kiln workspace list` | `未知命令: workspace（已改名为 `worktree`）`；exit 2 |
| `kiln nope` | 仍只是 `未知命令: nope`（旧名引导不外溢） |
| **合并后在本仓新建区、不归一化直接 `gofmt -l`** | **空**（此前会误报整棵树） |

单测：`TestWorkspaceNewReportsPath` 追加输出提示断言、`TestRetiredWorkspaceCommandIsRejected` 追加"已改名为"断言；`gofmt -l` 空、`go vet` / `go test ./...` **全绿**。

### 4. 余项（本轮未做；要可另开 issue）

- **`kiln worktree land <issue>`**：在主干一步收尾（主干干净检查 → `merge` → `remove`），消掉第 3 条的**顺序敏感**。
- **`worktree list` 增状态列**（干净 / 有改动 / 分支未合并）→ 治第 6 条。
- **`kiln commit` 在 git 后端先检查暂存并明说**（或加 `--all`）→ 治第 4 条。
- **`kiln commit --in <area>`**（免 cd）→ 治第 1 / 7 条的 cd 部分。

## test（三续）：撤回自举专属提示，改为项目中立提示

> 起因：用户读那条提示后指出——"现在是在开发 kiln 可以这么做，等 kiln 开发好了用在其他项目的话，不能这样"。**判断成立。**

### 1. 为什么那条提示越界

它把「开发 kiln 时的**自举约定**」泄漏进了通用 CLI 输出，硬编码了三个**只在本仓成立**的假设：

1. **仓库根有 kiln 二进制**（普通项目里 kiln 是装到 PATH 的，**区里本来就不需要它**）；
2. **这是个 Go 项目**（"改了 Go 源码需先重建"）；
3. **"主干那份"有语义**（只对 kiln 自举——用本仓源码构建自己——才成立）。

→ 用在别的项目上，这行话**无意义、甚至误导**。「陈旧二进制」这个坑本质是 **dogfooding 的产物**，归宿是**本仓的开发约定**，不是工具输出——这正呼应 worktree-include 的立场：**让区可用是项目自己的事**。

### 2. 撤回与替换

- **撤回**：`提交提示: …` 那行、`selfPath()`、以及为它引入的 `os` 导入。测试里对它的断言删掉，改成**反向断言**（输出不得出现"二进制"）。
- **替换为项目中立的一句**（**措辞经用户复议定稿**）：

  ```
  提示: worktree 是一次性检出——被忽略的构建产物与依赖不在其中，请自行 build / 装依赖
  ```

  不含路径、不提语言、不提任何具体工具。它也正是 `worktree-include` 结论里留的「**A 式提示覆盖层②③**」——至此那条留白落地。

  **措辞修订记录**（用户复议）：名词从内部速记「区」改为**用户可见术语 `worktree`**（与命令 `kiln worktree`、配置键 `worktree`、usage 页 `worktree 默认: on` 统一）；「一次检出」→「**一次性检出**」，更达意（只在建区时检出一次、之后不再同步）；`worktree` 后留一个空格，对齐 usage 页排版。
- **自举约定归本仓文档**（就是本节）：在 kiln 本仓干活时，站在区里提交请**用主干那份二进制**（区里没有它），并**在改了 Go 源码后先重建它**；或等价地在区内自行 `go build`。

### 3. 保留（不受影响）

- **`.gitattributes`**：它是**仓库自己的文件**，项目无关，通用。
- **旧名引导**（`未知命令: workspace（已改名为 worktree）`）：通用行为，项目无关。

### 4. 验证

| 实验 | 结果 |
| --- | --- |
| 临时仓 `kiln worktree new t1` | 输出为 `专用工作区: … / 下一步: … / 提示: worktree 是一次性检出——… / 已写入默认配置: …`——**无路径、无语言、无二进制建议** |
| 单测 | `TestWorkspaceNewReportsPath` 断言改为"worktree 是一次性检出"，并**反向断言输出不含"二进制"** |
| `gofmt -l` / `go vet` / `go test ./...` | 空 / 0 / **全绿** |
| **建区不手工归一化** | 区里 **0 个 CR 的 .go**、`gofmt -l` **空**——上一轮 `.gitattributes` 的收益实测成立 |

### 5. 取舍

**"工具中立"是硬约束**：CLI 的输出只说**项目无关的事实**（"worktree 是一次性检出"），项目相关的做法（用哪份二进制、怎么 build、装什么依赖）一律留给**项目自己的文档与约定**。这与 kiln 的定位一致——它是过滤器 / 规范器，不是项目工作流的编排者。

### 6. 待决（留 conclude）

1. ~~**用户可见名词是否全面统一为 `worktree`**~~ → **已定**（用户："统一术语"）：全部用户可见名词统一为 **`worktree` / `main`**，见 test（四续）。
2. **这句提示要不要顺带指向 `.worktreeinclude`**：播种是 kiln 自己的功能（项目中立、可发现性好），提一句能让人知道"有清单可用"；代价是这行变长。

## test（四续）：术语统一——用户可见名词全部改为 `worktree` / `main`

> 用户："统一术语"。把 test（三续）里"只改了一行"的事**做全**。

### 1. 映射

| 旧 | 新 |
| --- | --- |
| 专用工作区 / 专用区 / 工作区 | **`worktree`** |
| 主干 | **`main`** |

### 2. 覆盖面（`grep 专用|主干|工作区` 命中 27 处，全查全改）

- **CLI 输出与文案**：`worktree new` 首行（`worktree: <path>`）、`worktree list` 的两行角色（`main（只读/只合并）` / `worktree <issue>`）、`worktree remove` 的 `已删除 worktree: <issue>`、`refuseInMain` 的三行拒绝语。
- **帮助 / usage**：三条 `worktree` 命令描述、`worktree 默认: on（强制 worktree）| off（允许在 main 提交）`、两句 `--force` 说明。
- **报错**：`无法判定 worktree 类型`、`当前后端不支持 worktree`、`建立 / 读取 / 删除 worktree 失败`、`后端未列出 main`，以及**两个后端适配器**里的 `没有 issue 为 %q 的 worktree`、jj 的 `worktree 还有未提交的改动`、`jj workspace list 未列出当前 worktree`。
- **配置面**（用户会读、会改）：**播种的默认配置注释**（`config.defaultFile`）、本仓 `.kilnconfig.jsonc` 的注释、[docs/mvp-plan.md](mvp-plan.md) §2.3 配置示例的注释——**三处改成同一句**，免得 spec 与种子说法不一。

### 3. 不覆盖（有意）

- **文档的说明性散文**：历史 issue 文档是**当时的记录**、不改写；它们用的"专用区 / 主干"是**中文自述**，与 CLI 术语的映射就记在本节。
- **代码里的英文标识符**：`vcs.Workspace`、`WorkspaceInfo`、`.kiln/ws/<issue>`——按本 issue 早前的决定（"只改用户可见面"）**不动**。

### 4. 验证（临时 git 仓，真机整套输出）

```
$ kiln worktree new t1
worktree: <repo>/.kiln/ws/t1
下一步: cd 到该目录，再用 `kiln commit t1 <action>` 记录动作
提示: worktree 是一次性检出——被忽略的构建产物与依赖不在其中，请自行 build / 装依赖
已写入默认配置: <repo>/.kilnconfig.jsonc

$ kiln worktree list
<repo>              main（只读/只合并）
<repo>/.kiln/ws/t1  worktree t1

$ kiln commit t2 ask            # 在 main
拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。
先 `kiln worktree new <issue>`，再 cd 到 worktree 提交；确要在 main 提交时加 --force（一次性豁免）。
（想长期允许 main 提交，把配置里的 worktree 设为 off。）

$ kiln worktree remove nope
删除 worktree 失败: 没有 issue 为 "nope" 的 worktree

$ kiln worktree remove t1
已删除 worktree: t1
```

`kiln help` 里三条命令描述、`worktree 默认: …` 行与两句 `--force` 说明均已是新术语。单测断言同步（`当前是 main`、`main（只读/只合并）`、`worktree cli-necessity`、假后端的 `worktree 还有未提交的改动`）；`gofmt -l` 空、`go vet` / `go test ./...` **全绿**。

### 5. 取舍

- **`main` 而非「主干」**：与 `worktree` 同源的英文术语，且 `git worktree list` 的第一行就是主检出；中文散文里继续可用"主干"。
- **别名一并消失**：`workspace`（命令）已硬切，`专用区` / `主干` 这两个**中文别名**也在用户可见面消失——**一个概念，一个词**。

## 结论（已定）

**定了：加顶层 `worktree` 开关（字符串 `"on" | "off"`，缺省 `"on"`）；命令面统一为 `kiln worktree`（`new` / `list` / `remove`）；`--force` 定义为一次性豁免。关闭后 main 可直接提交，不拒绝、也不提示。**

### 1. 逐条裁定

| 问题 | 结论 | 依据 |
| --- | --- | --- |
| **要不要做** | **做**（否 A「恒开」）；形态取**两态** | 主流已把「隔离默认开 + 显式可关」做进项目设置（Claude Code `worktree.bgIsolation`）；A 不是行业共识，且让"就在 main 干"的仓库每次手记 `--force` |
| **语义强度** | 关闭 = **允许在 main 提交**；**不打提示** | 用户定两态；collect 里的 `warn` 档（ESLint / rustc）**明确不采纳**（见遗留 6） |
| **键名 / 位置** | **顶层 `worktree`（P）**，与 `vcs` 平级 | 键名有直接先例（`worktree.*`）；先平铺单键，将来长出 `worktree.baseRef` 一类子项再收成组（collect 的 R 留作扩展点） |
| **取值** | **字符串 `"on" \| "off"`**，缺省 `"on"` | 对齐 `vcs` 的字符串风格；**不用布尔**——将来加档不必改类型（破坏性） |
| **与 `--force` 的关系** | **互补**：配置管默认强度、`--force` 管当次豁免 | 两者从不互相改写状态；`"off"` 时 `--force` 仍接受、无副作用（幂等） |
| **默认值** | **`"on"`（隔离默认开）** | 与 `bgIsolation: "worktree"` 一致；拦的是「污染 / 归属错乱」这类**静默正确性故障**——正是"默认安全"该开的地方，行业"宽松默认"的惯例在此不适用 |
| **作用域** | 只管 **`commit` 的护栏** | `worktree new` 能力**保留**（想隔离的人仍可隔离，只是不再强制）；播种与 `status` 呈现不受影响 |
| **后端一致** | git 与 jj **同语义**（`on` 即拒 main 提交），**判定各写各的** | 判定机制本就不同（`.git` 是文件否 vs jj 非 `default` 工作区）；语义对齐由配置层收口 |
| **旧命令名** | **硬切、不留别名**（`workspace` → 未知命令 exit 2） | 与 `.kiln.json` → `.kilnconfig.jsonc` 的硬切取向一致，避免两套语汇长期并存 |
| **命令集** | `new` / `list` / **`remove`** 三件套 | 收尾不再退回后端方言（`git worktree remove` + `branch -d` / `jj workspace forget` + 删目录） |

### 2. 留下的不变式

**`worktree` 是这个仓的「默认护栏强度」，`--force` 是它的「当次出口」——一条管默认、一条管例外，谁也不改写谁。**

推论有三：① `"off"` 只是"允许"，不是"取消隔离能力"——建区命令照旧；② 护栏强度**随仓库、随版本**（写在 `.kilnconfig.jsonc`），不是命令行开关；③ kiln 拦的是**静默正确性故障**，故"默认开"站得住。

### 3. 落地物

- **配置**：[internal/config](../internal/config/)——顶层 `worktree`（`file.Worktree` / `Config.Worktree`）；归一化容忍空白与大小写并**严格校验**（非法值报错）；`Config.Isolated()` 取「非 `off` 即隔离」（零值 = 默认开）；`defaultFile` 播种内容把 `actions` / `vcs` / `worktree` **全部列出**（声明式：读一眼即知配置状态）。
- **CLI**：[internal/cli/cli.go](../internal/cli/cli.go)——`refuseInMain` 先看 `e.isolated()`，`"off"` 直接放行；`--force` 仍是当次豁免；命令面 `workspace` → `worktree`（`new` / `list` / `remove`、用法、拒绝文案、旧名引导一律改口）；`-h` 增 `worktree 默认: on（强制 worktree）| off（允许在 main 提交）`。
- **后端**：[internal/vcs](../internal/vcs/)——`vcs.Workspace` 增 `RemoveWorkspace(issue, force)`：git = `worktree remove` **且** `branch -d / -D kiln/<issue>`（未合并检查**前置**，避免把未合并分支搁浅）；jj = `workspace forget` **且** `os.RemoveAll`（实测 `forget` 只注销、不删目录）并查 `jj -R <区> diff --summary` 判脏。
- **仓库自己**：`.kilnconfig.jsonc` 显式写 `"worktree": "on"`；`.gitattributes`（`* text=auto eol=lf`）让新区检出直接是 LF，免去每轮行尾归一化。
- **文档**：本文件；[docs/mvp-plan.md](mvp-plan.md) §2.3 配置示例与 §10「已落地」条目。
- **测试**：`config` 新键的解析 / 缺省 / 非法值 + 播种含键；`cli` 的 `worktree ∈ {on, off}` × `--force ∈ {有, 无}`、用法只提 `worktree`、旧名 exit 2、`worktree remove` 三例。`gofmt -l` 空、`go vet` / `go test ./...` 全绿。

### 4. 本次一并裁定的两处留白

1. **`worktree new` 的提示不指向 `.worktreeinclude`**（用户裁定）。理由：那句话管**层②③**（依赖 / 构建产物，"请自行 build / 装依赖"），清单管**层①**（被忽略的本地小文件，已由播种自动处理）——并进一句会糊掉 `worktree-include` 立的分层；且清单不存在时指向一个不存在的文件是噪音（清单存在时，播种行已自报文件名）。**重启条件**：若清单成为普遍必读项（如将来默认生成一份），再考虑指向。
2. **历史文档里的旧命令名不改写**：`cli-necessity` / `config-home` / `worktree-include` 等文档里的 `kiln workspace …` 是**当时的真实记录**，保留原样；**living spec（`docs/mvp-plan.md`）已更新为新名**。代价是读者可能在旧文里撞见已废弃的命令名——由 `retiredCommands` 的旧名引导（`未知命令: workspace（已改名为 worktree）`）在**运行期**兜住。

### 5. 遗留（未做；若要做，另开 issue，不拖住本结论）

1. **`kiln worktree land <issue>`**——在 main 一步收尾（干净检查 → `merge` → `remove`），消掉收尾的**顺序敏感**（顺序反了会搁浅未合并分支）。
2. **`worktree list` 增状态列**（干净 / 有改动 / 分支未合并）——现在"哪个区能安全删"只在 `remove` 被拒时才暴露。
3. **`kiln commit` 在 git 后端先检查暂存并明说**（或加 `--all`）——git 提交前必须 `git add -A`（jj 不需要），忘了就 "nothing to commit"。
4. **`kiln commit --in <area>`**（免 `cd`）——治跨目录 `cwd` 管理。
5. **`worktree new --adopt`**（搬未提交改动）——`cli-necessity` 提过、一直未落地，是**独立缺口**（与 `.worktreeinclude` 的层①严格两件事）。
6. **`warn` 中间档**——重启条件：出现"只想提示、不想拦截"的真实需求时再议；届时取值是可扩展的字符串，不必改类型。
