# issue: worktree-list-status — `kiln worktree list` 要不要带状态列（哪个区能安全删）

- **状态**：ask（进行中）
- **domain**：cli
- **关联**：derived-from `worktree-mode`

## 背景

来源：`worktree-mode` 的 `## test（再续）`——那轮自查把"在区里干活"的摩擦列了 7 条，其中第 6 条是：

> **`list` 不告诉你哪个区能安全删**——脏 / 未合并只在 `remove` 被拒时才暴露。

同轮的「余项」把它记成候选：**`worktree list` 增状态列（干净 / 有改动 / 分支未合并）**。

现状是 `list` 只回答"区在哪、叫什么"：

```
<repo>              main（只读/只合并）
<repo>/.kiln/ws/t1  worktree t1
```

**它不回答"这个区现在什么状态"。** 于是：

- 想知道能不能删，只能**先试删**——`remove` 的护栏在**拒绝那一刻**才把真相打出来（"还有未提交的改动" / "分支 kiln/t3 还有未合并的提交"）；
- 收尾（`merge` → `remove`，顺序敏感）的判断**全在脑子里**，工具不给材料。这与 `worktree-mode` 自查里"工具的职责是给材料、不是替人编排"一致。

## 要回答的问题

1. **要哪几档状态**：干净 / 有改动 / 分支未合并——够了吗？要不要第四档（区目录已删但注册还在、jj 侧 `jj workspace list` 未列出）？多档之间是**互斥一列**还是**并列多个标记**（一个区可以既脏又未合并）？
2. **判定要与 `remove` 同源吗**：`remove` 的护栏判据已分别落在两个后端（git = `worktree remove` 自带的脏区检查 **+** 我们前置的"分支未合并"检查——`git rev-list --count kiln/<issue> --not <main>`；jj = `jj -R <区> diff --summary` 非空即脏）。状态列若各写一遍，会不会出现"`list` 说干净、`remove` 却拒"的**判据漂移**？要不要收进 `vcs.Workspace` 一个共享方法？
3. **代价**：探状态意味着**每个区各跑一次后端命令**（N 个区 × 1~2 次调用），`list` 从"读注册表"变成"要跑 git / jj"。默认全探、还是 `--status` 显式开启、还是默认探但给 `--no-status` 关掉？
4. **输出形态**：两列（路径 + 状态）/ 记号后缀（仿 `git worktree list` 的 `locked`、`prunable`）/ 状态词内联在 issue 名后（`worktree t1（有改动，未合并）`）？要不要更紧凑的两字符记号（仿 `git status --short`）？
5. **main 那一行给什么状态**：main 是"只读 / 只合并"，没有 issue 分支可判"未合并"——留空、写 `—`、还是换成别的词？
6. **后端中立**：jj 没有"分支"这个概念（工作区挂在各自的 commit 上，`kiln/<issue>` 分支是 git 侧的构造）——jj 侧是**少一档**，还是换一个等价表达（如"尚未合回 main"）？两后端的状态列要同语义吗？
7. **作用域**：只改 `worktree list`，还是 `kiln status`（全库概览）也要带上？

## 候选

### 判定归属

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 与 `remove` 同源 | 把"区状态"抽成一个 `vcs.Workspace` 查询方法，`list` 与 `remove` 共用 | 判据一处、无漂移；要把两个后端现有的零散检查收拢成一件事 |
| **B** `list` 自己判 | 各自再读 `git status --porcelain` / `jj diff --summary` | 快；但判据与 `remove` 分家，容易漂移出"两套真相" |

### 探测时机

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **X** 默认全探 | `list` 直接给状态 | 最有用；"只是看一眼区在哪"的调用也变重（N 个区 N 次后端调用） |
| **Y** `--status` 显式开启 | 默认只列表，加 flag 才探 | 零额外代价；但状态变成"要记得加参数"——正是本 issue 要治的那条摩擦的翻版 |
| **Z** 默认探、`--no-status` 可关 | 折中 | 多一个 flag 与一路分支 |

### 输出形态

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **P** 两列（路径 + 状态） | 顺着现输出的 `<path>  worktree <issue>` 尾上加一列 | 与现输出同构，改动最小 |
| **Q** 记号后缀 | 仿 `git worktree list` 的 `locked` / `prunable`；或仿 `git status --short` 的两字符 | 紧凑、易对齐；要定一张记号表 |
| **R** 状态词内联 | `worktree t1（有改动，未合并）` | 人读最顺；列对齐与脚本解析变难 |

## 待定

- 档位集合（三档 / 四档）与**互斥一列还是并列标记**。
- 判定要不要**收进 `vcs.Workspace`**（候选 A），即一个后端方法同时服务 `list` 与 `remove`。
- 探测时机（X / Y / Z）。
- jj 侧"未合并"的**表达**（少一档，还是等价词）。
- main 一行的状态怎么写。
- 是否与 `kiln status` 联动。

## 非目标

- 不改 `remove` 的护栏语义与 `--force` 一次性豁免——那是 `worktree-mode` 的定论。
- 不改区布局（`.kiln/ws/<issue>`）与播种（`.worktreeinclude`）——`cli-necessity` / `worktree-include` 的定论。
- 不新增 VCS 后端；不动 `vcs` 的 `auto | git | jj` 语义。
- **本轮不改代码**：只开题、定清问题与候选。
