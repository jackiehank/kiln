# issue: readme — 为 kiln 写一份面向使用者的使用说明

- **domain**：cli

## 背景

kiln 的开发**基本完善**，也已经**不再是 MVP 阶段**：命令面齐了（`do` / `commit` / `path` / `show` / `status` / `check` / `list` / `graph` / `worktree new|list|remove` / `config path`），两个后端（git / jj）都在真机跑通，工作区模式与播种落地，本仓也已完成自举（dogfooding：它自己就用 kiln 追踪自己的设计）。

**但仓库根没有 README**：现在要了解它，只能读 `docs/mvp-plan.md`（那是**规格**，写给"这工具该怎么长"）或一堆 `docs/*.md` 设计记录（那是**过程**，写给"为什么长成这样"）。缺的是给**使用者**的那一份——它是什么、怎么装、怎么开始、每天怎么用。

## 要回答的问题

1. **面向谁**：只用 kiln 的人（要能直接照做），还是也给改 kiln 的人（要有模块地图）？
2. **写到多深**：只做"快速上手 + 命令速查"，还是把 trailer 规范、配置、后端差异、`check` 规则都收进来？
3. **和现有文档怎么分工**：`README.md` 与 `docs/mvp-plan.md`（规格）边界划在哪，避免两处说法不一。
4. **语言**：中文（与仓库现有文档一致）还是英文。
5. **要不要写"为什么"**：把"过滤器不是注册器"这类定位与设计原则讲多少。

## 范围（本轮定）

| 项 | 决定 |
| --- | --- |
| 面向谁 | **使用者优先**；"改 kiln"的部分只给一行模块地图 + 指向 `docs/mvp-plan.md` |
| 深度 | 收齐：定位 / 安装 / 快速上手 / 命令一览 / trailer 规范 / 配置 / worktree 模式 / 后端差异 / `check` / 与 agent 协作 / 非目标 |
| 分工 | README 是**使用手册**（当前怎么做）；`mvp-plan.md` 是**规格**（约定与理由）；`docs/<issue>.md` 是**设计记录**（历史过程）。README 只留"现在是什么" |
| 语言 | **中文**（与仓库其余文档一致；英文版另说） |
| 定位与理由 | **只写一段**（过滤器不是注册器），理由留给 `mvp-plan.md` §1 |

## 非目标

- **不把 `mvp-plan.md` 搬进 README**（规格仍是唯一权威）。
- **不做变更日志 / 路线图**（那是 `docs/` 里 issue 的事）。
- **不改代码、不改命令面**——纯文档。

## apply：落地

- **[README.md](../README.md) 写出来了**（13 节 / 233 行）：定位 → 安装 → 五分钟上手 → 命令一览 → 动作词表 → trailer 规范 → 配置 → 工作区模式 → 后端差异 → `check` → 与 agent 协作 → 它不做什么 → 已知边界 → 延伸阅读。
- **只写当前态**：命令面照**今天的** `kiln -h` 写（`worktree new|list|remove`、`config path`、`show`、`worktree list` 的状态列），不带历史命令名与"待办"语气。
- **如实标边界**（「已知边界」一节）：`graph` 未渲染 `blocks` / `supersedes`、`part-of` 未落地、`remove` 清不掉"目录已删"的区、`--adopt` 未做——各给出处（如 [docs/relation-rendering.md](relation-rendering.md)）。使用手册不替未落地的事背书。
- **分工照旧**：README = 使用手册（**怎么做**）；`mvp-plan.md` = 规格（**约定与理由**）；`docs/<issue>.md` = 设计记录（**为什么长成这样**）。
- **顺带把一条说法变成了实测**：本轮开题提交里那个 `docs/readme.md` **没有 `git add`**，jj 后端把它整个工作副本固定进了提交（`git show --stat` 里是 `docs/readme.md | 33 +`）。README「后端」一节里"jj 不需要暂存"由此不是转述。

### 非目标（重申）

- 不搬 `mvp-plan.md` 进 README；**不做英文版**（要则另开 issue）。
