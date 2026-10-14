# issue: action-diff — 如何追踪一个 issue 的全过程，并查看每个 action 的变更

- **状态**：ask（进行中）
- **domain**：cli
- **关联**：relates-to `status-by-domain`

## 背景

`kiln path` 现在只列 **kind / hash / subject** 三列：

```
relation-vocabulary  已完成（当前 conclude）
  1. ask          fc5b428  9 个关系会不会太多、语义清晰吗、Agent 好不好读
  2. hypothesize  4910435  决策树草案 + …
  …
```

也就是说：**能看到"做了哪些动作"，但看不到每个动作到底改了什么。** 要知道变更，得手动 `git show <hash>` 一条条翻——而这正是"追踪一个 issue 的全过程"最想看的部分。

## 要回答的问题

1. **变更怎么带进 `path`？**
   - 全量 diff 会把整个问题淹掉（一个 issue 8 个动作、每个都几百行）；
   - 只给 `--stat`（文件 + 增删行数）可能刚好够用；
   - 还是干脆另开一条命令？
2. **要不要 `kiln show <issue> <n>`？** 只展开第 n 个动作的完整变更——比在 `path` 里塞 diff 干净，但多一个子命令、还要给动作编号（现在只有 hash，hash 会随历史重写而变）。
3. **和 `graph` / `status` 的分工**：`status` 是 domain 视图、`graph` 是关系视图、`path` 是**过程视图**——变更展示该挂在 `path` 上，还是另有其位？

## 更根本的一层：用 diff 反查元数据的诚实性

现有 `target:` trailer 表示"这个动作涉及的文件"。**它完全可以和真实 diff 对照**：

- 声明了 `target: internal/cli/cli.go`，实际只改了 `docs/` → **声明不实**；
- 改了 5 个文件一个字没提 → 至少值得一个提示。

也就是说，`diff` 不只是"好看"，它能让 kiln 多一条**可校验**的规则（现在 `check` 只校验关系完整性，从不碰"声明 vs 现实"）。这让 `check` 第一次有了"对着真实代码校验"的能力。

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** | `kiln path --stat`：每个动作附一行文件统计 | 轻、够用 |
| **B** | `kiln path --diff`：附完整 diff | 重，终端会被淹 |
| **C** | `kiln show <issue> <n>`：单独展开某个动作的完整变更 | 多一个子命令 + 动作编号问题 |
| **D** | `check` 新增规则：`target:` 声明 vs 实际 diff 一致性 | 让 check 首次"对着真实代码"校验 |

A + C 可以并存（概览用 A、深挖用 C）；D 是独立价值，值得单独评估。

## 非目标

- 不改 `graph` / `status`。
- 不新增存储——diff 全部来自 git 本身，kiln 仍只是"读 + 呈现"。

## 备注

这是本次长会话里**用 `kiln path` 复盘自己工作时**顺手记下的：过程看得到，但证据看不到。
