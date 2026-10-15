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
