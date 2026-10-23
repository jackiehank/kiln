# issue: derived-from-direction — 名字已定保留，但「写侧方向」与「写反了没人管」还没验

- **domain**：design
- **关联**：derived-from `relation-vocabulary`、relates-to `relation-rendering`

## 背景

`relation-vocabulary` 第三轮（裸名字测试）已给出结论：**`derived-from` 保持不动**——2/2 读对、
清晰度 5/5，并指出「原先'它不好'的印象来自实验二的箭头语境，不是名字本身」。同轮还定下命名
原则：**有向关系的键名必须含介词**（`part-of` / `blocked-by` 也都合规）。

但那三轮验的都是**读**（给一个名字，问方向是哪边）。真实项目里连续写边时，卡住的是**写**：

- 约定方向是「**被引发者 → 来源**」，名字读起来却像「来源 → 衍生」——每写一条都要停一下确认
  （本次写 `image-granularity` / `agent-declaration` / `agent-version-from-host` 三条边时如此）。
- 同一轮里，项目 owner 直接问出了「`derived-from` 要不要换个名字」。
- 更要紧的是：**写反了没有任何东西会拦**。`check` 只校验 derived-from 的目标**存在**
  （`engine.go` Rule 3），不校验方向；`A derived-from B` 与 `B derived-from A` 同时存在也只是
  两条边，不成环、不报错、渲染也不特别提示。

一句话：**「读得对」已经证过，「写得对 / 写反能被发现」没证过。**

## 要回答的问题

1. **真问题还是噪音**：写侧到底有多容易错？可做一轮代理用户实验：
   - 给场景 + 5 个键，看选择正确率（第四轮做过，11/12，但那批场景里没有"反向诱因"）；
   - 给一组**已知写反**的边（A 与 B 互为 derived-from），看 agent / 人能否发现——这是真正的
     失败模式，至今没测。
2. **若不换名，靠什么兜**：
   - **渲染**：`path` / `graph` 强制显示方向（`A ──derived-from──▶ B`），写反肉眼可见；
   - **校验**：`check` 增加「方向自相矛盾」（互为 derived-from → error）或状态一致性规则；
   - **提示**：`do` 的输出把方向展开成一句话（「被引发者 ← 来源」）。
3. **若换名，换成什么**：候选 `arises-from` / `caused-by` / `induced-by` / `stems-from`
   （第三轮里 `stems-from` / `inspired-by` / `spawned-by` 都读对）。代价：历史 trailer 里的旧键
   名要长期兼容读取，且现有名字已合规——**换名解决的是"感觉别扭"，不是"读错"**。
4. **写侧如果只是"多停一秒"**：实验若显示写侧正确率与读侧相当，本 issue 应直接收成
   「不换名 + 加方向渲染」一条记录，而不是留一个开放问题。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只加方向渲染 | `path` / `graph` 一律画箭头；`do` 输出加一句方向释义 | 最小；不解决"没人拦" |
| **B** 渲染 + `check` 方向规则 | 在 A 上加「互为 derived-from → error」等矛盾检测 | 要定义"自相矛盾"的判定面（环？状态先后？） |
| **C** 换名 | 换成 `arises-from` 之类 | 迁移成本；第三轮证据不支持 |
| **D** 不动 | 维持现状 | 零成本；风险留给使用者 |

> 倾向（待验证）：**A 或 B**——第三轮的读侧证据已经否掉了 C 的必要性；真正没覆盖的是
> 「写反了没人管」，那是**工具**问题，不是**词表**问题。

## 非目标

- 不重开 `relation-vocabulary` 的整体词表结论（写侧 5 个名字已定稿）。
- 不改 `derived-from` 的方向约定本身（子 → 父）。

## 证据

- `relation-vocabulary` 第三轮（`d7536c3`）：2/2 读对、清晰度 5/5，「保持不动」。
- `internal/engine/engine.go` `checkIssue` Rule 3：只校验 derived-from 的父**存在**，不校验方向。
- `internal/engine/engine.go` `relationLines`：渲染只有 `spawns` / `related` / `derived-from`
  三族，且**不带箭头**——写反的边在 `graph` 里长得和写对的边一样。
- 真实使用：wsl-agent 的 issue 梳理（2026-10-17）写完三条 `derived-from` 后 owner 提出改名。

## 结论（已定）

**问题已由词表正名解决，issue 收尾。**

- **词表现状**：面向用户的键是 `caused-by`（反向 `causes`）；`derived-from` 只作为**读别名**
  保留（`spawns` → `causes`），历史提交里的旧键名仍可解析（`internal/model/relation.go:125-143`）。
  `relation-vocabulary` 第三轮「`derived-from` 保持不动」的结论已被后续正名取代——方向语义
  （被引发者 → 来源）不变，键名换成了带介词的形态。
- **写侧方向自明**：`caused-by` 自带介词，方向写在名字里（被引发者 caused-by 来源），不再需要
  「每写一条停一下」的心理确认；`do` 的 flag 帮助由词表生成，带 `谁引发了我` 的释义
  （`internal/cli/cli.go:86-88`）。
- **写反可见**：`graph` / `path` 现在按声明源画箭头（`relation-rendering` / `graph-normalize`），
  写反的边方向相反，肉眼可辨。
- **不加 `check` 方向矛盾规则**：互为 `caused-by` 的两条边可能是有意的互引或历史事实，判定面
  （环？状态先后？）模糊、误报代价高于收益；`check` 继续只校验目标存在（warn）。此项留档为
  未来可选项，不在本 issue 范围。
- **非目标不变**：不重开 `relation-vocabulary` 的整体词表结论。
