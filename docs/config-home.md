# issue: config-home — 配置文件收进 `.kiln/`，并定其格式

- **状态**：ask（进行中）
- **domain**：design

## 背景

配置现在的形态（`docs/mvp-plan.md` §2.3 是它的成文规格，实现见 `internal/config`）：

- **位置**：仓库根下的 `.kiln.json`，**从"当前目录"读**（`config.Load(".")` / `config.VCS(".")`）。
- **格式**：JSON。
- **键**：`actions` / `start` / `terminal`（action 词表）+ 后来加上的 `vcs`（后端选择）。
- 它是**可选**的：文件不存在就用内置默认（本仓自己就没有 `.kiln.json`）。

上一轮把**状态根**定成了仓库内的 `.kiln/`（整目录自忽略，`.kiln/.gitignore` = `/*`）。于是出现一处不协调：**同前缀不同物**——`.kiln/` 是目录（kiln 的状态），`.kiln.json` 是根下的文件（kiln 的配置）。既然已经有"家"，配置理应收进去。

**但这不是搬文件那么简单**：`.kiln/` 现在整目录自忽略，配置一进去就**不再受版本控制**——而配置（词表、后端选择）恰恰是"跟着仓库走"的东西，有些仓库会把它提交进版本库。

## 要回答的问题

1. **位置**：搬成 `.kiln/config.<ext>`？若搬，**忽略规则怎么改**才能既保住"区被忽略"、又让配置**可版本化**（单点忽略 `/ws` vs `/*` + `!config.*` 例外）？
2. **格式**：JSON（现状）/ TOML / YAML / 极简 DSL / JSONC？判据：**能否写注释**（配置里"为什么这么设"值不值得留在文件里）、**手写与 agent 生成的可靠性**、**是否引入第三方依赖**（本仓目前 **stdlib-only**，`go.mod` 无任何 require）、**工具生态**（编辑器补全 / schema）。
3. **读的锚点**：继续跟"当前目录"（在子目录里跑 kiln 就找不到配置、**静默回落默认值**），还是锚定**仓库根**（与工作区/状态根一致）？
4. **迁移**：已有仓库里的 `.kiln.json` 怎么办——只认新路径 / 旧路径兼容一段时间 / 报错并引导迁移？

## 候选

### 格式

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** JSON（现状） | `{"actions":["ask",…],"vcs":"auto"}` | 零依赖（stdlib `encoding/json`）；**不能写注释**；手写易错（引号 / 逗号 / 尾逗号） |
| **B** TOML | `actions = ["ask", …]` + `# 注释` | 可注释、手写友好、分区清晰；**要引第三方库** → 打破 stdlib-only |
| **C** YAML | 缩进式 | 表达力最强、可注释；**要引第三方库**，且缩进敏感 + `no` / `on` 会被读成布尔（经典陷阱） |
| **D** 极简行式 DSL | `actions = ask, hypothesize, …` | 零依赖、可注释、解析器极小；**自造语言**——没有生态，schema 得自己维护 |
| **E** JSONC（JSON + 注释） | JSON 里允许 `//` | 可注释、仍是 JSON 心智；**非标准**，要自己剥注释，外部工具可能不认 |

### 位置与忽略

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **P** 留在根下 `.kiln.json` | 不动 | "同前缀不同物"继续存在；"家"不完整 |
| **Q** `.kiln/config.<ext>`，忽略改 `/ws` | 配置天然**可版本化**，区仍被忽略 | `.kiln/.gitignore` 自身变成需要提交的受控文件；且**主干不能提交**（U 布局的护栏）→ 首次得在专用区里提交它 |
| **R** `.kiln/config.<ext>`，忽略 `/*` + `!config.*` | 状态根仍整体自忽略 | 例外规则要维护；git 会把配置显式报成未跟踪（需有人提交）——即"可版本化"得靠一条例外规则兜着 |

## collect：配置"放哪 + 什么格式"的行业惯例

> 一次调研 + 本机实测。目的：看主流工具**各自怎么处理这两件事**，再看 kiln 的候选该怎么收敛。来源见节末。

### 1. 位置：惯例是「分成两层」，且方向相反

| 工具 | 工具**自有**的配置 | 项目**共享**的配置（随版本） |
| --- | --- | --- |
| **git** | 三层：系统 `…/etc/gitconfig`、用户 `~/.gitconfig`、**仓库 `.git/config`**（在状态目录内，**不进工作树、不随版本**） | 仓库根的 `.gitignore` / `.gitattributes`（**提交进版本**） |
| **jj** | 三层：用户 `%APPDATA%\jj\config.toml`、**repo `%APPDATA%\jj\repos\<hash>\config.toml`**、**workspace `%APPDATA%\jj\workspaces\<hash>\config.toml`**——**全在仓库之外** | 与 git 同理：忽略类文件在仓库根、随版本 |
| **Cargo / Ruff / pyproject / golangci-lint / tsconfig** | — | **项目根**：`Cargo.toml` / `ruff.toml` / `pyproject.toml` / `.golangci.yml` / `tsconfig.json`，**全部随版本** |

**一句话**：**"工具自己的东西"惯例放状态目录（或仓库外）、不随版本；"项目共享的东西"惯例放项目根、随版本。**

→ 对 kiln 的直接含义：`.kiln.json` 装的是 **action 词表**——**协作者该用同一套词**，属"项目共享"。它现在"随版本"是**符合惯例的**；搬进 `.kiln/`（默认整目录自忽略）等于把它**降级为"只随仓库、不随版本"**。所以位置问题的**前置问题**是：**词表算项目共享，还是算工具本地？**

> 副产物：jj 的 repo / workspace 配置**实测在仓库之外**（旧文档说的 `.jj/repo/config.toml` 在 0.45.1 上并不存在）——可见"仓库级配置放哪"连成熟工具都会**随版本漂移**。因此 kiln 值得有个**自述命令**（如 `kiln config path`，jj 就有 `jj config path`），别让用户靠猜。

### 2. 格式：惯例按「给谁读」分

| 格式 | 谁在用 | 为什么 |
| --- | --- | --- |
| **严格 JSON** | `package.json`、各类**交换 / 产物** | 机器产出、无注释需求，解析器必须严格 |
| **JSONC（JSON + 注释）** | **`tsconfig.json`、`.vscode/settings.json`** | 微软推广的 JSONC 规格；TypeScript 1.8 专门加注释，原文理由是「**总是很乐意能给配置写说明**」，编译器自带 JSONC reader。tsconfig 能有注释而 `package.json` 不能，正因为后者是**发布产物** |
| **TOML** | **jj**（文件头带 `#:schema`）、`Cargo.toml`、`pyproject.toml`、Ruff | 主打"给人读、语义明显、**无缩进陷阱**、支持注释"；社区共识是**手写的扁平配置用 TOML**，深层嵌套 / 生态所需的才上 YAML（K8s、CI） |
| **多格式并存** | **golangci-lint**（`yml` / `yaml` / `toml` / `json` 都支持） | "不选"的解法：自由给用户，代价是维护多条解析路径；配套 **JSON Schema 校验**与 `migrate` 迁移（**迁移前先备份旧文件**） |
| **YAML** | CI / K8s / 部分 lint | 深层嵌套与生态使然；缩进敏感是代价 |

**两条可抄的习惯**：① **schema 校验**——jj 在配置头写 `#:schema <url>`（本机那份用户配置就有这一行），golangci-lint 用 JSON Schema 校验配置；② **迁移向导**——golangci-lint 的 `migrate` 先备份再改。

### 3. 对候选的收敛

- **位置**：先答"词表算不算**项目共享**"。
  - **算**（协作者必须同词表）→ 惯例支持**放项目根、随版本**；若仍要收进 `.kiln/`，就必须选 **Q** 或 **R**（改忽略规则把配置放出来）——"**可版本化**"这条不能靠忽略规则偷偷丢掉。
  - **不算**（视作本地偏好）→ **P** 或 **R** 都行，随版本与否不再重要。
- **格式**：惯例把候选压成三条路——
  - **L1 严格 JSON + 工具侧解释**：文件仍是 **A**（零依赖），把"为什么这么设"从文件**搬到工具**（`kiln config explain` / schema 文档）→ 零依赖 + 注释需求另解；
  - **L2 JSONC（E）**：零依赖 + 文件内可注释，代价是**非标准**（外部工具可能不认，需自写剥注释）；
  - **L3 TOML（B）**：手写体验与生态最好，代价是**打破 stdlib-only**（本仓首个第三方依赖）。
  - **C（YAML）与 D（自造 DSL）本轮基本可排除**：YAML 与"手写少量扁平键"的场景不合且陷阱多；自造 DSL 没生态——**调研里也没有主流工具自造配置语言**。
- **两件几乎无争议的**：**schema 校验**（与 `check` 的取向一致）、**`kiln config path` 自述命令**（对齐 `jj config path`）。

> 来源：[git-config（多层配置与语法）](https://git-scm.com/docs/git-config) ｜ [jj Settings（三层配置）](https://docs.jj-vcs.dev/v0.36.0/config/) ｜ [jj CLI reference（`jj config path`）](https://docs.jj-vcs.dev/v0.36.0/cli-reference/) ｜ [JSONC 规格（微软为 VS Code 引入）](https://jsonc.org/) ｜ [TypeScript 1.8：Allow comments in tsconfig](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-1-8.html) ｜ [为什么 tsconfig 能注释而 package.json 不能](https://zeroutil.com/blog/tsconfig-json-comments-jsonc/) ｜ [TOML 官网（以人为先）](https://toml.io/cn/) ｜ [TOML vs YAML vs JSON 对比](https://codeutil.dev/blog/toml-vs-yaml-vs-json-config-formats) ｜ [golangci-lint：配置文件（多格式 + JSON Schema）](https://golangci-lint.run/docs/configuration/file/) ｜ [golangci-lint：迁移指南（先备份）](https://golangci-lint.run/product/migration-guide/)

## hypothesize：位置、格式与分组（按用户决策定稿）

### 已定的三条（用户）

1. **词表算「项目共享」**——每个项目的 action 可以不同，这正是自定义的价值 → 配置**必须随版本**（提交进仓库）。
2. **格式用 JSONC**——为了能在配置里写"为什么"。
3. **先分组**——action 只是配置的一部分，以后还会长出别的配置 → 把 action 收成一组。

### 由这三条推出的子决策（需确认）

#### a. 位置与忽略：从「`/*` 自忽略」改成「只忽略区」

- 文件：**`.kiln/config.jsonc`**（唯一配置文件；`.jsonc` 扩展名让编辑器按"JSON with comments"处理）。
- 既然要随版本，就**不能再待在自忽略的状态根里**（collect §1 留的前置问题现在有答案了）→ 忽略规则改**单点**：`.kiln/.gitignore` = **`/ws`**。
- 语义随之清爽：**`.kiln/` 下 `ws/` 是本地状态（忽略），其余是项目共享（随版本）**。
- **这是对上一轮布局决策（cli-necessity）的部分修订**：当时选"自我忽略 `/*`"，理由是"零改动受控文件"；决策 1 把这个理由推翻了。值得记一笔。
- 代价：`.kiln/.gitignore` 从"自我隐藏"变成**需提交的受控文件**；而主干不能提交（U 布局的护栏），所以它得在**专用区里提交一次**（一次性的引导动作）。
- 备选 **R**（`/*` + `!config.jsonc`）被弃：每长出一个配置文件就要加一条例外，与决策 3 冲突。

#### b. JSONC 只支持注释

- 支持 `//` 行注释与 `/* */` 块注释，**字符串感知**（不误剥 `"a//b"`、不误伤转义引号）。
- **不引入尾逗号**（那是 JSON5 的另一套扩展）：除注释外仍是严格 JSON，错误可以定位。
- 实现：`internal/config` 内约 30 行的 `stripComments`，**仍然零依赖**（stdlib）。

#### c. 分组形态

```jsonc
{
  // 词表：每个项目可以不同——这是 kiln 的自定义点
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  "vcs": "auto"
}
```

- 旧扁平键 → 新分组键：`actions`（数组）变 `actions.names`，`start` / `terminal` 移入 `actions` 组。
- **只认一种形态**（不做"扁平与分组都认"）——避免 schema 分叉；迁移靠改名，不靠兼容层。
- 宽松规则保留：`start` 缺省取 `names` 首项、`terminal` 缺省取末项、`names` 为空则整体回落内置默认。
- `vcs` 暂留顶层（单键不值得再开一组）；将来多了再收。

#### d. 读的锚点：锚定仓库根（向上搜索）

- 现在跟**当前目录**（子目录里跑 kiln 会**静默回落默认值**）——与"项目共享"的定位不符。
- 新规则：从 cwd **向上**找 `.kiln/config.jsonc`（找不到再找旧的根 `.kiln.json`），命中的即**仓库根的配置**。
- 好处：`do` 在仓外仍可用；在子目录里跑 kiln 读到的是同一份。

#### e. 迁移：兼容旧 `.kiln.json` + 一行提示

- 优先 `.kiln/config.jsonc`；不存在而目录里还有 `.kiln.json` → 读它，并向 stderr 打一行"配置已迁移到 .kiln/config.jsonc"。
- **不做硬切**（会让现有仓库突然失配），也**不长期双读**（留一个明确窗口，写进文档）。

### 影响面（留给 test 落地）

- `internal/config`：注释剥离 + 新路径/分组解析 + 旧路径 fallback；
- `vcs.PrepareAreas`：`.kiln/.gitignore` 内容 `/*` → `/ws`（仍不覆盖已存在）；
- 本仓自身：`.kiln/.gitignore` 从 `/*` 改成 `/ws` 并**提交**（引导动作需在专用区做）；
- 成文规格与帮助文本同步：`docs/mvp-plan.md` §2.3、`kiln -h` 里"可用 `.kiln.json` 覆盖"那句。

## test：落地配置——`.kiln/config.jsonc`（JSONC + 分组 + 锚定仓库根 + `config path`）

### 落地内容

- **`internal/config` 重写**：
  - **位置**：`<仓库根>/.kiln/config.jsonc`，**向上搜索**定位——第一个 `<dir>/.kiln/config.jsonc` 命中；最近的仓库根（`.git` / `.jj`）既终止搜索，又给出"还没有文件时该写哪"。
  - **格式**：**JSONC**——自写 `stripComments`（约 40 行，**零依赖**）：剥 `//` 与 `/* */`，**字符串感知**（`"auto // not a comment"` 是数据）；注释字节**替换成空格**，使字节偏移与行号与原文对齐，json 的报错位置可直接对着文件读。只加注释，**不引入尾逗号**。
  - **schema 分组**：`actions.{names,start,terminal}` + 顶层 `vcs`；**只认这一种形态**。
  - **`DisallowUnknownFields`**：键拼错**报错**，不再静默忽略。
  - **旧 `.kiln.json` 已移除**：硬切，无回落、无兼容层。
  - `Ensure(startDir)`：缺失时在仓库根写入**带注释的默认配置**（内容即内置默认，可原样编辑）。
- **`kiln config path`**（新命令）：打印配置路径；不存在则播种并明说。
- **`kiln workspace new` 也会播种**：写命令把仓库的"家"布置好（幂等、不覆盖已有配置）。
- **`vcs` 的忽略规则 `/*` → `/ws`**：`.kiln/` 语义随之清爽——**`ws/` 是本地状态（忽略），其余是项目共享（随版本）**。
- 帮助文本与成文规格同步：`kiln -h` 的配置说明、`docs/mvp-plan.md` §2.3。

### 验证

- `gofmt -l` 干净、`go vet` / `go test ./...` **全绿**。
- 新增单测 **15 例**：`config` 12 例（剥注释 2、解析 6、锚定 1、`Ensure` 3）+ `cli` 3 例（`config path` 播种 / 二次不重写 / 未知子命令）。

真机（临时仓，新二进制）：

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | 干净 git 仓跑 `kiln config path` | 打印 `<repo>/.kiln/config.jsonc`，并写入**带注释的默认配置**（头几行就是注释） |
| 2 | 再跑一次 | 只打印路径，**不再播种** |
| 3 | `kiln workspace new u-test` | 建区（`.kiln/ws/u-test`）+ 顺带"已写入默认配置"；`.kiln/.gitignore` = **`/ws`** |
| 4 | `git check-ignore -v .kiln/ws/u-test` | `.kiln/.gitignore:1:/ws` → **区被忽略**；而 `config.jsonc` 与 `.gitignore` 在 `git status` 里**可见**（待提交的项目文件） |
| 5 | 写入**带注释 + 自定义词表**（`["ask","probe","done"]`） | `kiln do u-test probe` 正常；`kiln do u-test hypothesize` → **非法 action**（exit 2）→ 词表确实被替换 |
| 6 | 子目录 `sub/deep` 里 `kiln config path` | 仍解析到**仓库根那一份** → 修掉了"子目录里静默回落默认值" |
| 7 | 把键拼错（`"action"`） | `配置错误: … unknown field "action"`，exit 1 → **不静默** |
| 8 | 只放旧 `.kiln.json`（老扁平 schema，含 `zzz`） | `zzz` → **非法 action（列出的是内置默认词表）**，而 `hypothesize` 正常 → **旧文件已失效** |
| 9 | colocated jj 仓：`config path` + `workspace new` | 播种在 `<repo>/.kiln/config.jsonc`；`git status` 只列两个项目文件，**区不出现** |

### 发现 / 取舍

1. **`/*` → `/ws` 是对 cli-necessity 布局决策的修订**：那条当时以"零改动受控文件"为由选了自我忽略；"词表算项目共享"一旦成立，自忽略就必须让位——配置得进版本。已回填一行修订注记到 `docs/cli-necessity.md`。
2. **播种用仓库根，读用最先命中的那份**：因此从某个区里跑，读到的是**该区自己检出出来的那份**（随版本的文件天然如此）——不再有第二个真相源。
3. **"想要注释"与"stdlib-only"不再冲突**：JSONC 的代价只是一段剥注释代码，不是一个第三方库。
4. **播种是 `config path` 的既定语义**（"给我一份可编辑的配置"），因此**读命令依旧无副作用**：`status` / `path` 不会悄悄写文件。
5. 环境提醒（沿用上轮）：`core.autocrlf=true` 让新区 checkout 出 CRLF → `gofmt -l` 会误报整棵树；归一为 LF 后干净，且 `git diff --cached` 证实**无伪改动**。

### 未做 / 待定

- 旧 `.kiln.json` 的**硬切**是否要在用户可见处（如启动时的 stderr）再提醒一次——现在只在 `docs/mvp-plan.md` 说明新位置。
- 配置的只读视图（如 `config list` / `--json`）本轮未做，只做了 `config path`。
- 区里那份配置与主干那份**理论上可分叉**（各自是检出副本，且被忽略的 `.kiln/ws` 不进区）；目前靠"配置随版本"自然收敛，没有额外同步机制。

## test（复议）：「配置放 `.kiln/` 还是根」——改到仓库根 `.kilnconfig.jsonc`

> 上一轮已按"收进 `.kiln/`"落地；本轮是用户**复议**后的反转。记下 A/B 对照、三笔实测成本与落地。

### 复议：A/B 对照

| 维度 | A：`.kiln/config.jsonc`（上一轮） | B：根下 `.kilnconfig.jsonc`（本轮） |
| --- | --- | --- |
| 仓库根条目 | 不变（`.kiln/` 已因 `ws/` 存在） | **+1** |
| "一个家" | 全在 `.kiln/` | 配置在根、状态在 `.kiln/` |
| `.kiln/` 的语义 | **混合**（本地状态 + 项目内容） | **单一**：纯本地状态 |
| 忽略规则 | 被迫 `/*` → `/ws`，`.kiln/.gitignore` 变成**要提交的受控文件** | `/*` 自我忽略（同 jj 的 `.jj`，**无需提交**） |
| 实测摩擦 | ① 修订了 cli-necessity 的原决策；② 主干不能提交 → 得在专用区提交那个 `.gitignore`；③ **实际挡过一次快进合并**（未跟踪文件被 git 拒绝覆盖） | 三条**全部消失** |
| `git status` 观感 | `?? .kiln/`（看不出区是否真被忽略，要 `check-ignore` 确认） | `?? .kilnconfig.jsonc`（单文件；`.kiln/` **永不出现**） |
| 惯例对照 | 工具**成族**配置用点目录（`.github/workflows`、`.vscode`） | **项目共享**的单文件配置用根（`.editorconfig`、`.golangci.yml`；git 自己也把共享的 `.gitignore`/`.gitattributes` 放根，只把本地的 `.git/config` 放进状态目录） |
| 决策 3（会长） | 加配置不动根 | 每个新配置文件 +1 根条目 |

**定论的分层视角**：**共享进根，本地进 `.kiln/`**——正好与 git / jj 的双层惯例对齐。

### 落地内容

- `config.FileName` = **`.kilnconfig.jsonc`**，位置 = **仓库根**；`Path` 的向上搜索改为直接找 `<dir>/.kilnconfig.jsonc`（最近的 `.git` / `.jj` 仍是"还没有文件时该写哪"的规范落点）。
- **`internal/config` 不再依赖 `internal/vcs`**（不必再引用状态根常量）——依赖方向更干净。
- **`vcs.ignoreBody` 回到 `/*`**：`.kiln/.gitignore` 重新"自我隐藏"、**不需要提交**；`PrepareAreas` 的注释同步回"状态根自我隐藏"。
- 播种内容的头行改为"放在仓库根 / 位置：`<仓库根>/.kilnconfig.jsonc`"。
- 本仓自身：删除上一轮提交的 `.kiln/config.jsonc` 与 `.kiln/.gitignore`，新建根下 `.kilnconfig.jsonc`。
- 文档同步：`docs/mvp-plan.md` §2.3（位置与文件名）、`docs/cli-necessity.md` 的修订注记（改回 `/*`）。

### 验证（临时仓，新二进制）

| # | 实验 | 结果 |
| --- | --- | --- |
| 1 | 干净 git 仓 `kiln config path` | 打印 **`<repo>/.kilnconfig.jsonc`** 并播种；头行即"放在仓库根" |
| 2 | 播种后 `git status` | 只有 **`?? .kilnconfig.jsonc`** |
| 3 | `kiln workspace new u-test` | 建区（`.kiln/ws/u-test`）；`.kiln/.gitignore` = **`/*`**；`git status` **仍只有那一行**（`.kiln/` 完全不出现） |
| 4 | `git check-ignore -v .kiln/ws/u-test` | `.kiln/.gitignore:1:/*` → 区被忽略 |
| 5 | 带注释 + 自定义词表；子目录里 `config path` | 词表生效（`hypothesize` 非法）；子目录仍解析到**仓库根那份** |
| 6 | colocated jj 仓 | 同样落在 `<repo>/.kilnconfig.jsonc`；`git status` 只有 `A .kilnconfig.jsonc` |
| 7 | 单测 | `gofmt -l` 干净、`go vet` / `go test ./...` 全绿（改的是路径与断言，测试数不变） |

### 结论

- **这个 issue 的前提被自己的调研推翻**：原始命题是"把配置收进 `.kiln/`"，而 collect 的惯例（项目共享 → 根）与上一轮落地实测的三笔成本，共同指向"配置该在根"。**"问了 → 查了 → 做了 → 复议后改前提"**，正是这个 issue 最有价值的部分。
- `.kiln/` 从此**只装本地状态**（自我忽略、无需提交）；将来若出现"工具本地、不随版本"的设置（缓存、个人偏好），那里仍是它们的家。
- 你要的三条行为**全部保留**：JSONC、分组 schema、锚定仓库根、`config path` 播种。

## 非目标

- 不评估"配置放到仓库之外"（与本项目 in-repo 的取向冲突）。
- 不动 action 词表的**语义**（顺序非约束、起点/终点规则），只谈文件的位置与形态。
- 暂无用户级 / 系统级配置层——只有"仓库这一层"。
