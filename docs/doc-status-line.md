# issue: doc-status-line — issue 文档头部不再写「状态」行

- **domain**：design

（本文档即按本 issue 的结论写：**头部不写「状态」行**——状态由 kiln 从提交重建。）

## 背景

每个 issue 文档的头部一直有三行元数据：

```
- **状态**：ask（进行中）
- **domain**：design
- **关联**：derived-from `worktree-mode`
```

其中**「状态」是唯一会动的一行**：它按轮次推进（`ask` → `hypothesize` / `collect` → `test` → `conclude`），而写它的时机**只有开题那一次**，之后没人回来改。于是它**必然漂移**——实测（2026-10-15）：

| 文档 | 头部写的 | `kiln status` 的真相 |
| --- | --- | --- |
| `relation-vocabulary.md` | ask（进行中） | ✓ conclude |
| `relation-types.md` | ask（进行中） | ✓ conclude |
| `action-diff.md` | ask（进行中） | ✓ conclude |
| `object-semantics.md` | ask（进行中） | ✓ conclude |
| `cli-necessity.md` | ask（进行中） | test |
| `evidence-fluidity.md` | ask（进行中） | hypothesize |

而**状态本来就是 kiln 自己记的**：每个动作的提交带 `issue:` + `action:` trailer，`kiln status` / `kiln path` 从**提交**重建当前态（还有 `kiln check` 校验完整性）。在文档里再手写一行，等于**给同一个事实留两处真相**，且**注定错的那处写在最显眼的位置**（文件第 3 行）。

> 用户判断：*"我们是通过 kiln 来规范 git 提交来记录状态，在文档写一行没必要。"*

## 要回答的问题

1. **删不删**：删掉「状态」行，还是改成别的写法（只写起点动作？干脆不写？）。
2. **删到哪**：只删**未完成**的文档，还是**全部**（已 conclude 的历史文档也一并删）？
3. **与「domain」「关联」的关系**：那两行同样"可从 kiln 派生"（`domain:` trailer、关系 trailer），但它们**不随轮次变**——该不该一起删？（同一个"文档里要不要重复元数据"的问题。）
4. **约定记在哪**：只靠本 issue + 项目记忆，还是要写进 living spec（`docs/mvp-plan.md`），免得后来者又添上？
5. **替代物**：要不要在文档里留一句**指向 kiln 的提示**（如"状态见 `kiln path <issue>`"），还是彻底不留？

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只删「状态」 | 头部留 domain / 关联 | 最小改动，对症（只有它会漂移）；静态元数据保留，读者不必跑 kiln |
| **B** 删「状态」+「关联」 | 关系也算可派生 | 少一行冗余；但读文档时看不到它连着什么 |
| **C** 三行全删 | 头部只剩标题 | 最干净；但 domain 是对外归类提示，删了要看 `kiln list --domain=` |

## 非目标

- **不改 `kiln` 的任何行为**——纯文档约定，不动代码、不动 `check`。
- **不改文档正文**：历史记录照旧（`cli-necessity` / `worktree-*` 等文里的实测与取舍**一律保留原样**），本 issue 只动头部那一行。
