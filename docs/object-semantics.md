# issue: object-semantics — `object:`（q1/h1/t1/c1/a1）到底用来干什么

- **状态**：ask（进行中）
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

## 非目标

- 不改动作词表；不引入强制校验（除非第 1 条决定要给它行为）。
- 不重写已有提交——`object:` 已经写进历史，任何改动都要考虑向后兼容。
