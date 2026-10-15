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

## 待定

- **前置问题（collect 催生）**：action 词表算"项目**共享**"（随版本）还是"工具本地偏好"？——它决定位置那题怎么答。
- 新旧路径的优先级与弃用时间窗。
- 是否需要"仓库根 vs 当前目录"的显式开关（不引入开关更简，但会改变既有行为）。
- 注释性配置是否值得——若选 A/E，就默认放弃在配置里写"为什么"。

## 非目标

- 本轮不改代码、不动 `.kiln.json` 的读取位置；只把问题与候选记下来。
- 不评估"配置放到仓库之外"（与本项目 in-repo 的取向冲突）。
- 不动 action 词表的**语义**（顺序非约束、起点/终点规则），只谈文件的位置与形态。
