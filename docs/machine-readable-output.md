# issue: machine-readable-output — 给 `status` 等读命令加 `--json` / `--toon` 结构化输出

- **domain**：cli
- **关联**：derived-from `cli-necessity`

## 背景

kiln 的**读命令**（`status` / `list` / `path` / `check` / `graph` / `show` / `worktree list`）目前**只有人读文本一种形态**：

- `status`：按 domain 分组、`✓ ⚠ ■` 记号、全角对齐的目录视图；
- `list`：等宽列（issue / action / domain=…）；
- `path` / `show`：缩进的编号步骤 + 关系摘要 + 委托 git 的 stat 块；
- `check`：`✓ / ⚠ / ✗` 三档 + 人类措辞的判据；
- `graph`：`ascii` 树（唯一格式）。

agent / 脚本要拿数据，只能**解析这些为"给人看"而排版的文本**——README §与 agent 协作 把这写成优点（"稳定文本，agent 不必解析自由格式"），但"稳定"靠的是**约定**，不是**格式**：字宽、符号、缩进一变，解析器就碎。

这一条**已经被点名过**。`cli-necessity` 对 jj 的调研里三处指出同一件事：

- §"四条命令层面的观察" 第 4 条：jj 有 `-T` 模板，kiln MVP 无 `--json`，对 agent / 脚本不友好；
- §"面向 agent 的 UX"：短板是"**输出偏人读、无 `--json`**"；
- §"对 cli-necessity 的启示" 第 5 条：**"若 agent 是主要用户，这比'命令重叠'更该先补"**。

`docs/mvp-plan.md` 一侧的现状：§6 把 `--format=json` 明确**排除出 MVP**；§10 只留了一条 `kiln check --format=json`（CI 集成与 PR 评论）——覆盖**一个**命令面，且没有回答"用哪种格式"。

## 要回答的问题

1. **要不要做？** 现有文本已"够稳定"，补机器可读输出的边际价值？先例是 git 的 `--porcelain`、jj 的 `-T`：**为机器单开一条输出契约**，与人读渲染解耦。
2. **覆盖哪些命令？** 只 `check`（对齐 §10 既有条目）？还是所有读命令一起上？`do` 的输出**不能**碰（它必须可直接喂 `git commit -m`）。
3. **`--json` 还是 `--toon`？** 这是本 issue 的核心分岔：
   - **JSON**：模型先验最熟、stdlib 可编解码（本仓 **stdlib-only**，零成本）、生态通用（CI / jq / schema 校验）；
   - **TOON**（Token-Oriented Object Notation）：**省 token**、面向 LLM 的新格式——但 agent **先验弱**、要**自写**编码器、无生态、非标准。
   两条判据要摆明：**省 token 的收益 vs 模型不熟 / 自造轮子的成本**；并对照 mvp-plan §1「**LLM 先验优先**：只组合模型最熟的原语，不为优雅引入长尾 DSL」。
4. **单格式还是多格式？** 两个独立 flag（`--json` + `--toon`）／统一 `--format=json|toon|text`（对齐 `graph` 已有的 `--format=`）？后者是一族枚举，前者是布尔，**默认值**各是什么。
5. **人读文本还留吗？** 默认保持文本（人 + agent 共用一条路径），还是要求机器消费方显式 opt-in？（现状 stderr 提示已保证 **stdout 纯净可 pipe**，这条要不要写成契约。）
6. **结构从哪来？** 输出是"领域模型的直接序列化"（`[]Issue` / `[]Action`）还是"渲染层再排一遍"？要不要一份**稳定的 schema 文档** / 版本字段？字段名是否直接复用 trailer 名（`issue` / `action` / `derived-from`）以少一次翻译。
7. **`check` 的退出码与结构化输出的关系**：现在"只有 `✗` 让 `check` 非零退出"。json 模式下，判据（哪条规则、哪个 issue、error/warn）要不要逐条结构化给出——这是 CI 集成（§10 原条目）真正需要的东西。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 不做 | 维持"稳定文本" | agent 继续自写解析器；`cli-necessity` 点名的短板不补 |
| **B** 只补 `check` | `kiln check --format=json`（= §10 现成条目） | 面上最小；`status` / `path` / `list` 仍只能解析文本 |
| **C** 全读命令 `--json` | 各命令加 `--json` | 改动面最大；每个命令都要定 schema |
| **D** `--json` + `--toon` 双格式 | 每命令两个 flag | **格式 × 命令**组合翻倍；toon 要自写编码器、agent 先验弱 |
| **E** 统一 `--format=json\|toon\|text` | 一个 flag 家族 | 要先定枚举、默认值与未知值的报错口径 |

> 倾向（**待验证**）：**先落 C 的一个子集**——优先 `status` / `list` / `check`（结构最规整、agent 最常问），格式先做 **JSON**（零依赖、先验最熟），**TOON 作为待评估的第二格式**，不当第一枪。`path` / `show` 含委托 git 的原文块，序列化口径需单独设计。

## 非目标

- **不改默认输出**：人读文本保留，机器可读是**旁路**而非替换。
- **不引第三方依赖**：本仓 `go.mod` 无 `require`；选 TOON 就意味着**自写编码器**，成本要算进方案 D/E 里。
- **不碰 `do`**：它的 stdout 必须能直接 `$(kiln do …)` 进 `git commit -m`。
- **不做 MCP server**：那是 §10 的另一条（"把结构化工具暴露给 Agent"），与本 issue 相邻但不同——本 issue 只在 **CLI 的输出侧**。
- 不改 stderr 提示策略（提示只进 stderr、`exit code` 不变），但 json 模式下 stdout 必须仍是**单一可解析文档**。

## 备注

- 来源：用户在会话中直接提出——"考虑要不要把 status 等输出信息的命令加上 `--json` 输出或者 `--toon` 输出来优化 agent 交互"。
- 由 `cli-necessity` 派生：其 §启示 5 已把"结构化输出"列为 agent 场景下**优先级高于"命令重叠"**的短板；本 issue 是那一条的**落地评估**。
- 与既有条目合并：mvp-plan §10 的 `kiln check --format=json`（CI 集成与 PR 评论）并入本问——命令面从一个扩到一族。
- 若落地，README §与 agent 协作 里"结构化输出……agent 不必解析自由格式"那句要重写：**"稳定文本"与"机器可读格式"是两回事**，不能继续拿前者当后者卖。
