# issue: cli-necessity — agent 手握 git 时，kiln 的 CLI 还有多少必要性

- **状态**：ask（进行中）
- **domain**：design

## 背景

本会话里，**agent（我）频繁直接用 `git`，而不是 `kiln`**。最典型的例子：要查一个 issue 的提交里到底写了什么，我敲的是

```
git log -1 --format=%B e98c23f
git show e98c23f --stat --format=""
```

而不是 `kiln show author-attribution 1`。这逼出一个根本问题：**如果 agent（和人）手里都有 git，kiln 这套 CLI 到底凭什么存在？**

## 为什么当时用了 git（自述，供解剖）

1. **要逐字原文**：`kiln path` 给的是摘要（kind / hash / subject）；我要确认提交里**到底写了哪些 trailer**（例如那条已退役的 `object: q1`），只有 `git log --format=%B` 是逐字真值。
2. **手上有 hash**：直接 `git show <hash>` 比"先想 issue id、再想序号"短。
3. **惯性**：agent 默认抓**最通用**的工具；专用包装（kiln）需要"主动想起来用"。
4. **对 kiln 的覆盖没底**：当时不确定 `kiln show` 是否给到逐字 trailer、值不值得为此多敲一次 id + 序号。

> 诚实地说：第 1 条是**真需求**（逐字真值），第 2–4 条是**惯性 + 信任缺口**。后者正是本 issue 要盯的——**如果 agent 没理由想起 kiln，它就不存在。**

## 要回答的问题

1. **增量价值在哪**：agent 有 `git` + 一份**约定文档**，就能自己 `git log --grep` / `git show` / 手写 trailer。kiln 的命令里，**哪些是 git 做不到的**，哪些只是"方便包装"？
2. **命令逐个体检**：
   - `check`（语义完整性校验）——git 没有"issue / action"模型，**git 做不到**？
   - `status` / `graph`（按 domain 聚合、关系图）——git 无此综合？
   - `path`（把一个 issue 的动作重建成有序路径）——git 无此重建？
   - `show` 及各类读取——**纯 git 包装**？
   - `do` / `commit`（规范化写）——对一个能自己拼 trailer 的 agent，还需要吗？价值是否只在**减少格式漂移**？
3. **必要性锚点**：kiln 的价值是不是**只**落在 git 表达不了的四件事——**可执行的模型（不变量）、校验、综合视图、类型化接口**？读 / 单提交展示是否该完全让给 git？
4. **方向**：CLI 该**变薄**（退成"spec + `check`"，读全交给 git）、**变厚**（把结构化能力封成 **MCP**，让 agent 不拼 git 串），还是维持现状？

## 事实记录（本会话的用法分布）

| 用途 | 实际用了 |
| --- | --- |
| 规范写一条动作 | `kiln commit`（始终用 kiln） |
| 概览 / 校验 / 关系图 | `kiln status`、`kiln check`、`kiln graph` |
| 看某 issue 的动作路径 | `kiln path` |
| **取提交逐字原文 / 某提交的 diff** | **`git log --format=%B`、`git show`** |
| 文件操作 / 构建 / 测试 | shell、`go` |

> 现成结论：**kiln 被用到的时候，都是"它独有的视图或规范化写"**；一旦只想拿**逐字真值**，我立刻退回 git。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 维持 | 现有命令不动 | 继续与 git 重合；agent 可能仍绕过 kiln |
| **B** 变薄 | kiln ≈ spec + `check`（+ 少量综合视图）；读 / 单提交展示全让给 git | 承认 CLI 很窄；需要一份权威 spec 文档 |
| **C** 变厚 | 把结构化能力封成 **MCP**（typed tools），agent 不再拼 git 串 | 投入最大；MVP 明确排除过 MCP |
| **D** 分层 | "git 做不到的"留 kiln（`check` / `status` / `graph` / `path` + 规范写），"git 能读的"统一委托 git | 需划清边界 |

## 非目标

- 本轮不改代码、不新增 / 删除命令。
- 不评估 MCP 的实现细节。

## 备注

由用户在会话中直接提出：看到 agent 用 `git log --format=%B` 而非 `kiln path`，追问"基于什么考虑；如果 agent 都在用 git，kiln 的 CLI 有多少必要性"。这是迄今最**根本**的一个问题——它反过来质疑 kiln 这个项目的立足点。
