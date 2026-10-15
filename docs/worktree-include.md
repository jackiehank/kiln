# issue: worktree-include — 要不要把「被忽略的本地文件」带进专用区

- **状态**：collect（进行中）
- **domain**：design

## 背景

来源：`cli-necessity` 的 `## test（再续 3）`——**区 = 一次检出**，所以**被 gitignore 的本地文件不进区**（本仓的 `kiln.exe` 就是活例），于是在区里干活之前得先 build / 装依赖；而且那次**踩出了一个静默失败**（二进制不存在，退出码又被前一条命令的 `0` 覆盖，看起来像提交成功）。

社区对应物：**`.worktreeinclude`**——把**被忽略、但工作树要用的文件**（`.env`、构建产物、本地配置）按清单**复制**进新工作树（Claude Code 等工具的做法）。若 kiln 也做，那是与 `kiln config path` 播种配置**同构**的一种"**播种**"。

本 issue 要定的，正是 `cli-necessity` 里留作候选的 **B（复制清单）**：把"要不要做"从倾向变成决定，并定下清单的位置与键名。

## 要回答的问题

1. **要不要做**：只提示（A）/ 复制清单（B）/ 建区钩子（C）/ 干脆不做（D）——先定一条；乃至"让区可用"是否**本来就该由项目自己**（构建脚本、文档约定）承担，kiln 不该管。
2. **清单放哪**：**独立文件**（如根下 `.kilninclude`，与 `.gitignore` 同族、零解析负担）还是**配置里的一节**（`.kilnconfig.jsonc` 新增 `include` 组，顺着刚定下的 JSONC + 分组 schema）？
3. **键长什么样**：`include` / `include-paths` / `carry` / `seed`？取值是**glob 模式**（照 `.gitignore` 语法）还是**显式路径列表**？
4. **复制的语义**：只带**被忽略**的文件（照 `.worktreeinclude` 的约定），还是也允许带**未提交的改动**（那会和 `--adopt` 撞车）？目标已存在时覆盖还是跳过？**方向**是单向进区，还是还要回写？
5. **后端中立**：git（worktree）与 jj（workspace）要做同一件事——各自复制，还是委托后端？（注意 jj 的 `--sparse-patterns` 是"稀疏检出"的语义，**不是同一件事**，别混。）

## 候选

### 做不做

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只提示 | `kiln workspace new` 打一行"区里没有构建产物，先 build（或借主干二进制）" | 零成本；靠人或 agent 记得，**静默失败仍在** |
| **B** 复制清单 | 按清单把被忽略的本地文件复制进区 | 新增配置面；"工具替用户搬本地状态"，有越界嫌疑 |
| **C** 建区钩子 | 建区后跑用户声明的命令（如 `go build`） | 最省事；但把**编排**塞进 kiln，与 U 的"编排在外"冲突 |
| **D** 不做 | 只把"区 = 检出、先让区可用"写进文档 | 零成本、零越界；agent 仍可能踩静默失败 |

### 清单放哪 / 键名

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **P** 独立文件 `.kilninclude` | 照 `.gitignore` 的位置与语法（一行一模式），零解析负担 | 仓库根再添一个文件；与"配置集中在一处"的取向相悖 |
| **Q** 配置里一节：`.kilnconfig.jsonc` 的 `include` | 复用现成的 JSONC + 分组 schema | 要新增键与校验；好处是配置只有一处 |
| **R** 复用 `.gitignore` 反向标记 | 不新增面：约定某个前缀表示"要带的" | 语义错位、隐蔽，倾向否 |

## 待定

- 键的最终命名与取值语法（glob vs 路径）。
- 与 **`--adopt`**（搬**未提交改动**）的边界要划清：**"本地忽略文件"（本 issue）与"未提交改动"（adopt）是两件事**，别混成一个机制。
- 若做 B，要不要配套一个"反向"操作（把区里的本地文件清掉 / 回写主干）。
- 触发时机：只在 `workspace new`，还是也支持"已有区补播种"。

## 非目标

- 本轮不改代码：只开题、定清问题与候选。
- 不评估 **C（建区钩子）** 的细节——它与"编排在外"的立场冲突，除非第 1 问翻案。
- 不评估"让区可用"该不该由**项目自身**（Makefile / 构建脚本）承担——先把 kiln 该不该管这问清楚。

## collect：`.worktreeinclude` 的确切语义，与「新工作区可用性」的行业分层

> 一轮联网调研（厂商官方文档为主 + 社区脚本 + 构建工具文档）。目的：把 `.worktreeinclude` 从"社区对应物"变成**可照抄的规格**，并看别人怎么处理"新工作树要能用"——好判断 kiln 该做哪一层。来源见本节末。

### 1. `.worktreeinclude` 的确定语义（两家 agent 厂商已把它标准化）

| 维度 | 事实 |
| --- | --- |
| **位置** | **仓库根**（Claude Code："project root"；Codex："repository root"），与 `.gitignore` 同级 |
| **语法** | **`.gitignore` 语法**（一行一模式），支持路径与 glob |
| **带哪些** | **只带被忽略的文件**——文件必须**同时**「匹配清单里某模式」**且**「被 git 忽略」；**被跟踪的文件永不复制**（就不会与已检出内容重复） |
| **冲突** | **已存在于目标区的文件，跳过、不覆盖**（Codex 明说 "won't overwrite files that already exist"；社区脚本同）；Codex 还**跳过源软链接**（不跟随 symlink） |
| **方向** | **单向进区**：只从主干（main checkout）复制进新工作树，**没有回写** |
| **触发** | **只在工具自己建工作树时**（Claude Code 的 `--worktree` / `EnterWorktree` / 子代理隔离；Codex 的 managed worktree）。**手动 `git worktree add` 不生效**；Codex 明确"**不适用** remote worktree 或**你自己从命令行建的** worktree" |
| **后端** | Claude Code：`.worktreeinclude` **在非 git VCS 下不处理**——要改用 `WorktreeCreate` hook 替代。即它**绑 git worktree**；扩到别的 VCS 得靠 hook |

一句话：**`.worktreeinclude` = 一张"从主干搬被忽略的小文件进新树"的选择清单，只在工具建树时跑一次，只进不回、只补不覆盖。**

### 2. "让新工作区可用"其实有四层——复制清单只解决其中一层

调研里最有用的一条：**"新工作树能不能干活"≠"复制几个本地文件"**。它至少含四层，而**各层的行业解法不同**：

| 层 | 典型对象 | 行业解法 | 是"复制清单"该管吗？ |
| --- | --- | --- | --- |
| **① 本地小文件** | `.env` / `.env.local` / `config/secrets.json` / 本地覆盖配置 | **复制清单**（`.worktreeinclude`；已存在则跳过） | ✅ **就是它** |
| **② 依赖** | `node_modules` / `.venv` | **各自重建 + 共享缓存**（pnpm/Bun 的 content-addressable store + hardlink；uv 共享 cache，每区 `uv sync` 出自己 venv）；**明确反对**各区 symlink 同一个 `node_modules`（见下） | ❌ 不该复制——重装更便宜、更安全 |
| **③ 构建产物** | `target/` / 编译二进制 | **共享 / 重定向 build 目录**（Cargo `CARGO_TARGET_DIR` / `build.target-dir` 指向共享目录加速重建），或干脆重建 | ❌ 不复制 |
| **④ 运行态资源** | 端口 / 数据库 / OAuth 回调 | **按区分配**（如一台区一个 `.worktree-port` 端口块、一个独立 DB 名、注册 per-port 回调）；**复制救不了**——这是"共享资源"，不是"文件" | ❌ 不复制 |

两条关键洞察：

- **②③ 都倾向"重建 + 共享缓存 / 目录"，而非"复制"**：Cargo 用 `CARGO_TARGET_DIR` 让多个工作树共享一个 `target/`；pnpm/Bun 用 hardlink 共享 store；uv 共享 cache。→ **复制清单不是"让区可用"的通用解，只是①那一格。**（且 `node_modules` **不该 symlink 共享**：dev.to 那篇明确——一个 agent `npm install` 会写进主干的 `node_modules`，把其他 agent 的环境**在脚下改掉**。）
- **④ 是"共享资源"，不是"文件"**：端口 / DB / 回调**在文件夹之外**，复制清单（在文件夹之内）**结构上碰不到**。→ 印证"区解决文件隔离，不解决资源隔离"。

> 对 kiln 的直接含义：**B（复制清单）该做、且只能做的，是①**——"被忽略的本地小文件"。**②③** 属于"先 build / 装依赖"（A 式**提示**的领地），**④** 属于用户 / 项目的编排。这把 issue 里"B 是不是在做越界的全包"这一疑虑**收敛**掉了：B 的语义天然就窄。

### 3. 别人的具体做法（一览）

| 工具 | 清单形态 | 语义要点 |
| --- | --- | --- |
| **Claude Code** | 仓库根 `.worktreeinclude` | `.gitignore` 语法；只"匹配**且**被忽略"的；建 `--worktree` 时复制；非 git VCS 用 hook 替代 |
| **Codex** | 仓库根 `.worktreeinclude` | 同上；**不覆盖已存在**、**跳过 symlink**；**不适用**命令行自建的 worktree |
| **VS Code** | 设置项 `git.worktreeIncludeFiles`（实验性） | 同样是"建 worktree 时带上被忽略文件"的 glob 列表——**证明"清单式带上"是编辑器层的既定需求** |
| **社区 post-checkout 脚本** | 读同一份 `.worktreeinclude` | 为**兼容两家厂商**而读同名文件；但**只支持纯路径**（含 `*` 的跳过）；`git check-ignore` 二次确认"确实被忽略"；已存在则跳过；**手动 `git worktree add` 也生效**（厂商做不到，hook 可以） |
| **"直接拿 `.gitignore` 当清单"** | 不加新文件 | `git ls-files --others --ignored --exclude-standard --directory` 枚举全部被忽略项再复制——**零配置**，但等于"**带全部**被忽略文件"（含 `node_modules`），无法选择性排除 |
| **jj（colocated）** | 同 git 命令 | 同一枚举命令可用，但**必须 `:(exclude).jj`**：否则 Git 会把 jj 自有的 `.jj/` 当被忽略目录**复制进新区，损坏该区** |

### 4. 对候选的收敛

| 问题 | 收敛 | 依据 |
| --- | --- | --- |
| **做不做** | **做 B，语义限定在"层①：被忽略的本地小文件"**；同时保留 A 式提示覆盖②③ | 两家 agent 厂商已把 B 标准化成一等公民——但**那绑的是厂商自己建的 worktree**；kiln 自己的区（`kiln workspace new` 建）**不在**其覆盖范围内，所以 **kiln 不做就没人替它兜**。而 B 的窄语义正好避开"越界替用户搬重物"的疑虑 |
| **C 建区钩子** | **维持不做** | 行业确实有 hook（Claude Code `WorktreeCreate`、Codex setup script），但那是**工具内的编排**；kiln 立场是"编排在外"，不该把 `go build` 塞进 CLI。hook 只作**逃生舱**（厂商正是用它把 `.worktreeinclude` 扩到非 git VCS）——留作未来扩展点，不纳入本轮 |
| **D 不做** | **否** | "清单式带上被忽略文件"已是**编辑器 / agent 层的既定需求**（VS Code 设置项 + 两家厂商 + 社区脚本都在做）；kiln 的区是自建的，退成"只写文档"等于让 agent 在自己的区里踩已知坑 |
| **清单放哪** | **倾向 P（仓库根独立文件）**，**建议沿用厂商键名** | 两家厂商 + VS Code + 社区脚本**全部**用"仓库根独立文件"，且都用 `.gitignore` 语法——这是**最强的 LLM 先验**（设计原则第 6 条）；独立文件还有额外好处：**同一份文件可同时被 Claude Code / Codex 和 kiln 读**。名字可定 `.worktreeinclude`（吃厂商先验）或 `.kilninclude`（kiln 自有）——留 conclude 定 |
| **Q 配置里一节** | **次选** | 好处是"配置只有一处"（顺 `.kilnconfig.jsonc`）；但**新增键**有解析 / 校验成本，且自造键名**没有 LLM 先验**，agent 不会天然想起来写 |
| **R 复用 `.gitignore` 反向标记** | **否** | "零配置枚举全部被忽略"（`git ls-files --others --ignored`）确实可行，但它是"**带全部**"、**无法选择性排除** `node_modules` 一类重物——与"清单"的初衷背道而驰 |
| **键的语法** | **`.gitignore` 语法（一行一模式）**；最小实现可先只支持纯路径 | 与厂商一致；社区兼容脚本进一步证明"纯路径"子集够用 |
| **复制语义** | **只带"匹配 且 被忽略"的；目标已存在 → 跳过不覆盖；单向进区、不回写；跳过 symlink** | 厂商 + 社区脚本**三条全部一致**（唯一分歧：Codex 还跳过 symlink——可采纳） |
| **与 `--adopt` 的边界** | **保持两件事**：本 issue = "被忽略的本地文件"；`--adopt` = "未提交改动" | 调研里**没有任何工具**把两者合成一个机制——`.worktreeinclude` 一律"只被忽略的"，未提交改动另有其路（Codex Handoff / stash 类） |
| **后端中立** | **"解析清单 + 复制 + 跳过"与后端无关、可共用；"枚举被忽略文件 / 校验 ignored"按后端分派** | 关键差异在枚举：git 用 `git ls-files --others --ignored`；jj（colocated）用**同一命令 + 排除 `.jj`**（否则损坏区）。Claude Code 的"非 git VCS 走 hook"也印证**复制清单天然偏 git**，jj 要额外处理 |
| **触发时机** | **计划：只在 `workspace new`**（对齐厂商"只在建树时跑一次"）；"已有区补播种"留作可选后续 | 厂商一致的触发点就是"建树那一刻"；补播种不是刚需 |

### 5. 结论（待 conclude 确认）

1. **做 B**，但**只做层①**（被忽略的本地小文件）；②依赖 / ③构建产物 / ④运行态资源**明确不做**——②③ 是"先 build / 装依赖"（靠提示），④是用户编排。
2. **清单 = 仓库根独立文件 + `.gitignore` 语法**（倾向 P；名字 `.worktreeinclude` vs `.kilninclude` 待定）。
3. **语义照抄厂商**：只带"匹配且被忽略"、已存在则跳过、单向进区、跳过 symlink。
4. **后端分派**只在"枚举被忽略文件"这一步；jj 必须排除 `.jj`。
5. **与 `--adopt` 严划边界**：一个管"被忽略的本地文件"，一个管"未提交改动"。

> 来源（厂商官方文档为骨干，其余标注类型）：[Claude Code：Run parallel sessions with worktrees（`.worktreeinclude`）](https://code.claude.com/docs/en/worktrees#copy-gitignored-files-into-worktrees) ｜ [Claude Code：Common workflows（Copy gitignored files to worktrees）](https://code.claude.com/docs/en/common-workflows) ｜ [Codex：Worktrees（Copy ignored local files into managed worktrees）](https://learn.chatgpt.com/docs/environments/git-worktrees) ｜ [VS Code：Git branches and worktrees（`git.worktreeIncludeFiles`）](https://code.visualstudio.com/docs/sourcecontrol/branches-worktrees) ｜ [Auto-copy files when creating Git worktrees or Jujutsu workspaces（社区脚本）](https://cretezy.com/2026/worktree-copy) ｜ [Git Worktrees for Coding Agents Break on .env, node_modules and Port 3000（社区长文）](https://dev.to/gabrielanhaia/git-worktrees-for-coding-agents-break-on-env-nodemodules-and-port-3000-fix-all-three-3781) ｜ [Cargo：Build Cache（`CARGO_TARGET_DIR`）](https://doc.rust-lang.org/cargo/guide/build-cache.html) ｜ [pnpm：Motivation（content-addressable store）](https://pnpm.io/motivation) ｜ [uv：Caching](https://docs.astral.sh/uv/concepts/cache/)
