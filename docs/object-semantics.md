# issue: object-semantics — `object:`（q1/h1/t1/c1/a1）到底用来干什么

- **domain**：design
- **关联**：relates-to `action-granularity`

## 背景

动作词表里有个可选的 `object:` trailer，约定取值是 `q1` / `h1` / `t1` / `c1` / `a1`（依次对应一次 ask / hypothesize / test / conclude / apply 的产物对象，见 `docs/mvp-plan.md` §2.2）。

问题在于：**它现在完全没有被使用。**

- 本仓库每个 kiln 动作都带了 `object:`（`q1` / `a1`），但 kiln **从不解析、聚合或展示它的语义**：
  - `kiln path` 只把它当一个字符串打印在第三列；
  - `kiln check` / `status` / `list` / `graph` 完全不看它；
  - 没有任何命令能按 `object` 查询，也没有任何校验（`q1` 会不会重复？和 `action` 类型对不对得上？都没管）。
- 结论：`object:` 目前是"写了但没人读"的字段——和 `tags:`、`refs:` 一样只是被动存着，区别是后两者至少还能被 `list` 过滤，`object` 连这个都没有。

## 核心问题：`object:` 与"引出一个新 issue"有什么区别？

这是本 issue 要回答的关键。同一段"问题"，我们有两种表达手段：

**A. 在同一个 issue 下加一个对象**

```
issue: session-expiry
action: hypothesize
object: h1
```

含义：在 `session-expiry` 这条研究路径上，产生了一个中间产物 `h1`（一个假设）。

**B. 派生出一个新的 issue**

```
issue: session-cache
action: ask
derived-from: session-expiry
```

含义：从 `session-expiry` 里**分叉**出一条新的、独立的探究线。

差别看似是"粒度"：

| 维度 | A：同一 issue 下的 `object` | B：派生的子 issue |
| --- | --- | --- |
| 生命周期 | 属于**过程**，随宿主 issue 结束而结束 | 独立成活的探究线，有自己的 `ask` → 终点 |
| 完整性校验 | 不参与 `check` | 参与 `check`（要有起点、以终点收尾） |
| domain | 继承宿主，不单独分类 | 有自己的 domain，可跨域 |
| 关系网 | 无独立关系 | 有 `derived-from` / `spawns`，可再派生 |
| 可见性 | `path` 里的一行文字 | `status` / `list` / `graph` 里的一等公民 |

但边界在执行时**很模糊**：什么时候"这个假设值得单独起一条 issue 追"，什么时候"它只是当前问题的一个中间结论"？现在没有可操作的判据，于是实际操作里大家（包括 Agent）**一律选择 B**——这也是 `object:` 沦为摆设的直接原因。

## 待探讨

1. **`object:` 保留还是砍掉？** 若永远只是"写在第三列的字符串"，砍掉能换来更简单的模型；若要保留，就得给它行为（解析、聚合、展示、校验）。
2. **编号谁来定？** `q1` / `h1` 这种编号是人工约定还是工具生成？现在 `object` 是自由字符串、无唯一性约束，不同 issue 都写 `q1` 也不会冲突——这算优点还是隐患？
3. **要不要与 `action` 解耦？** `q`/`h`/`t`/`c`/`a` 这套"前缀对应动作类型"的约定是否必要？改成可读名字（`object: lock-race-hypothesis`）会不会更好？
4. **可否给出升格判据？** 例如一条可操作的规则：*当这个产物自己需要多个动作、或需要单独分类（跨 domain）、或会被别的 issue 引用时，升格为子 issue；否则留在原地做 `object`。*
5. **可视化**：若保留 `object`，`kiln path` 该不该把对象也树形展开，而不是平铺一行？

## 结论（已定）

**issue 是原子的；组合靠关系，不靠 issue 内部再分线索。**

1. **`object:` 退场。** 它原本是"文件粒度"时代给每个产物（`q1.md` / `h1.md`…）挂的标签。粒度改成提交、issue 变成原子单元之后，`q1 → h1 → t1 → c1` 这条流程**本身就是该 issue 的动作序列**（`ask → hypothesize → test → conclude`），不需要再用 `object` 标一遍。
2. **子问题 = 原子 issue，用关系连接。** 一个复杂母问题下的 q1/q2/q3，就是三条独立 issue（连同母问题共四条），用类型化关系挂到母 issue 上；每条都各自过 `check`、各自有 domain、各自可再派生。
3. **两层同构。** 动作（`ask` / `hypothesize` / …）是原子的、可在流程里组合；issue 也是原子的、可由关系组合——同一个设计思想贯穿两层。
4. **关系保留"可加类型"的口子。** 现有 `derived-from` / `spawns` / `relates-to` / `blocks` / `supersedes` 暂不细分；将来要表达更语义化的类型（如"子问题" vs "后续跟进"），再按现有"一键一类"的风格增键，或给值加类型限定。

这样，原先"什么时候用 `object`、什么时候开 issue"那个模糊判据消失了——**一律开 issue**。剩下的"这条记录值不值得单开一条 issue"，本质与 `action-granularity` 里"一个动作要不要拆成两个提交"是同一类问题。

## 非目标

- **不重写已有提交**：`object:` 退役后，旧提交里那些 `object:` 行会被新解析器忽略（识别只看 `issue:` + `action:`，不受影响），不做历史迁移。
- **不改动作词表**：`ask` / `hypothesize` / `collect` / `test` / `conclude` / `apply` 不变，本次只退役 `object:` 这一个 trailer。
- **暂不新增关系类型、不加校验**：`check` 仍只跑规则 1–4。
