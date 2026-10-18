# issue: relation-rendering — `graph` / `status` 不渲染 `blocks` / `supersedes`，与 `path` 不一致

- **domain**：cli
- **关联**：relates-to `cli-necessity`

## 背景

同一个 `blocks:` 关系，**三个视图看到的程度不一样**：

- `kiln path <issue>`：**显示**——关系摘要里含 derived-from / spawns / relates-to / **blocks** / supersedes（`relationSummary`）。
- `kiln graph`：**不显示**——`relationLines` 只画 spawns / relates-to / derived-from（[engine.go](file:///c:/Users/jacki/kiln/internal/engine/engine.go#L277-L289)）。
- `kiln status`：**不显示**——status 只管 domain 分组与嵌套，不呈现依赖边。

于是：给 `evidence-fluidity` 声明 `blocks: cli-necessity` 后，`path` 能看到这条阻塞（`关系: blocks cli-necessity`），但 `graph` / `status` 里**看不到**——同一个事实，换个视图就消失了。

## 要回答的问题

1. **每个视图该渲染哪些关系？** 定一张"**视图 × 关系**"矩阵：`path`（过程视图）显示哪些、`graph`（关系视图）显示哪些、`status`（目录视图）显示哪些。现状**没有定义**，各视图各写各的。
2. **`graph` 该补 `blocks` / `supersedes` 吗？** 二者都是有向关系，画成箭头最自然；`part-of` 未来落地后同理。
3. **方向怎么画？** `blocks` 是"阻塞者 → 被阻塞者"；`graph` 现有条目是"我 spawns / related / derived-from 谁"的正向写法。反向边（"谁 `blocks` 我"）要不要一并生成？
4. **`status` 要不要显示关系？** 还是坚持"status 只管 domain 目录"、把关系留给 `graph`？（现状偏后者。）
5. **`check` 要不要顺带校验 `blocks` 一致性**（阻塞者未完成、被阻塞者却已推进 → 告警）？——mvp-plan §10 已有"`blocks` 状态一致性"条目，与本 issue 相邻但**不同**（那是校验，这是渲染）。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只补 `graph` | 给 `relationLines` 加 `blocks` / `supersedes` | 最小；`status` 仍不显示关系 |
| **B** 统一渲染 | 先定"视图 × 关系"矩阵，各视图按矩阵补齐 | 要先做设计决定；改动面较大 |
| **C** 全塞进 `graph` | 所有关系都在 `graph` 出现 | `graph` 会变臃肿；方向 / 反向边要处理 |

## 非目标

- 不改关系**词表**（那是 `relation-vocabulary`）。
- 不改 `path` 的关系摘要（它已显示）。

## 备注

由 `cli-necessity` 会话中发现：为 `evidence-fluidity` 声明 `blocks: cli-necessity` 后，`path` 能看到、`graph` / `status` 看不到。用户要求先**记录**、之后再解决。

## hypothesize：视角化渲染——X 为主语，动词按 X 的角色选正名 / 反名

> 用户推演（2026-10-18）：`relation-vocabulary` 刚定下「声明方是主语」；于是有向关系同样有主动/被动两面（`blocks` / `blocked-by`）。**渲染时，看与 X 有关的 issue，应该都能读成「X blocks Y」「X blocked-by Z」——主语始终是 X。**

### 0. 把「主语」拆成两个视角

| 视角 | 谁是主语 | 用在哪 |
| --- | --- | --- |
| **写侧** | 声明方（写这条 trailer 的 issue） | 决定**写哪个名**（按声明方在语义里扮的角色） |
| **读侧** | 被看的 issue X | 决定**渲染哪个名**（按 X 在语义里扮的角色） |

不变式统一为一句：**当前视角的 issue 就是主语**。「声明方是主语」只是写侧的特例。

### 1. 渲染规则（形式化）

每条边存成 `(声明方 D, 目标 T, 名字 n)`，`n` 是「D 相对 T 的角色」。名字两两成对（`blocks` ↔ `blocked-by`），`inverse(n)` 由配置机械给出。从视角 `X ∈ {D, T}` 渲染：

- `X == D` → `X <n> T`
- `X == T` → `X <inverse(n)> D`
- 对称关系（`relates-to`）→ 两侧同名，无正反

### 2. 例子：`blocks` / `blocked-by`

> 存：新建的 A 声明 `blocked-by: B`（A 是等待方）。
> - 看 A 的关系 → `A blocked-by B`
> - 看 B 的关系 → `B blocks A`

同一条边，两页两句，**主语都是当前页的 issue**——与 Jira / GitHub 的关系区的读法一致。两侧名都不需额外存储，一侧声明、另一侧渲染反名即可。

### 3. 由此确认：9 个名不是可选的，它们是**渲染词对**

| 语义 | 正名（声明方=等待/子/被引发/新） | 反名（声明方=阻塞/母/来源/旧） |
| --- | --- | --- |
| 引发 | `derived-from` | `spawns` |
| 分解 | `part-of` | `has-part` |
| 依赖 | `blocks` / `blocked-by` | （另一侧） |
| 取代 | `supersedes` | `superseded-by` |
| 关联 | `relates-to`（对称） | — |

配置里每个关系声明一个 `inverse`（或配对 id），引擎就能在**任意视角**渲染出「X 为主语」的句子。这也直接回答了本 issue 的问题 **3（反向边要不要生成）**：**要**，但不是存第二条边，而是渲染时换名。

### 4. 两种渲染形态，别混

| 形态 | 用在哪 | 方向 / 主语 |
| --- | --- | --- |
| **箭头** | `graph` / 全局图 | **规范化**：一条边只有一种箭头形态（正名方向），与谁声明无关 |
| **句子** | `path X` / X 的关系区 | **视角化**：主语 = X，动词按 X 的角色选正/反名 |

两者同源（同一个 `(对, 槽, 名字)`），只是投影不同；箭头管「图长什么样」，句子管「从 X 看，它跟谁什么关系」。→ 上一轮在 `relation-vocabulary` 写的「写 9 / 渲 1」应修正为「**写 9 / 句渲 2（正反） / 图渲 1（箭头）**」。

### 5. 回答本 issue 的「视图 × 关系」矩阵

| 视图 | 渲染形态 | 应包含的关系 | 备注 |
| --- | --- | --- | --- |
| `path X` | 句子（主语=X） | 全部 5 族，**含反向边**（X 为目标那些） | 现状缺反向边 |
| `graph` | 箭头（规范化） | 全部有向关系（含现在缺的 `blocks` / `supersedes`） | 问题 2 的答案 |
| `status` | 不渲染关系 | — | 维持现状（问题 4） |

### 6. 实现强制项（与 `relation-revision` 的接口）

- **反向推导从「可选优化」升为硬依赖**：X 视角里「X 是目标」的那些边，必须把**别人的声明反着读**。现状 `relationLines`（`engine.go:302`）只画本 issue 自己存的三族，连名都没反。
- 需要 per-issue 的**入边索引**（或全量扫）才能把入边翻出来；否则只能看到出边。
- 反名与配对（`inverse`）要么硬编码、要么进 `relations` 配置——与上一轮「名字可自定义」的设计对齐。

### 7. 待验证

1. **视角化会不会让人迷惑**：同一对在 A 页是 `blocked-by`、在 B 页是 `blocks`——需 subagent 代理实验验证可读性。
2. **箭头规范形式用哪一侧做正名方向**：如 `blocks` 的箭头是「阻塞者→被阻塞者」还是反过来；需与 `relation-vocabulary` 的箭头规格对齐。

### 结论（推演，未定论）

1. **合理且必要**：有向关系的正反名不是冗余，而是**视角渲染的词对**。
2. **命名不变式升级**：写侧主语=声明方，读侧主语=被看方；统一为「**当前视角的 issue 即主语**」。
3. **两形态**：全局图=规范化箭头（1 态）；关系列表=视角化句子（按主语选正/反名）。
4. **反向边要生成**，但只在**渲染时**换名——不存第二条边。
5. 由此，反向推导/入边索引从「可选」变成**硬依赖**。
