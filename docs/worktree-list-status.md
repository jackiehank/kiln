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

## test：落地——`worktree list` 尾列状态（判定与 `remove` 同源）

> 用户点了候选 **A / X / P**：判定抽成 `vcs.Workspace` 的一个共享查询（`list` 与 `remove` 共用）、默认全探、输出在现两列后补**状态一列**。

### 1. 本轮定下的细节（A / X / P 之外的留白）

| # | 决定 | 依据 |
| --- | --- | --- |
| **档位** | **`干净` / `有改动` / `未合并`**；两者同时成立写 **`有改动，未合并`**（一列里并列，不拆多列） | 三档正好是"能不能安全删"的三个答案；一格里写完，读表不会看错列 |
| **main 一行** | **`—`** | main 不是专用区、没有 issue 分支可判；**留空就会被读成"干净"** |
| **jj 侧** | **少一档**——jj 只会出 `干净` / `有改动` | `workspace forget` 既不问、也不丢提交（工作副本的提交留在同一对象库里），"未合并"在 jj 侧**不是"会丢东西"的理由**，报出来反而误导 |
| **输出形态（P）** | 三列 `<路径>  <角色>  <状态>`，路径与角色各自按显示宽度对齐 | 顺着现输出尾上加一列，改动最小 |
| **探测时机（X）** | `list` **恒探**，不加开关 | 状态列的意义就是"不必先试删"；做成 `--status` 等于把摩擦换个地方 |
| **答不出时** | 后端读状态失败 → **报错、exit 1**，不静默省略 | 省略会被读作"干净"——这正是本 issue 要消灭的误读 |

### 2. 落地内容

- **`internal/vcs/vcs.go`**：新增 `AreaState{Dirty, Unmerged}` 与 `Workspace.AreaState(area) (AreaState, error)`——**只有这一处判据**，`RemoveWorkspace` 与 `worktree list` 都读它。
- **`internal/vcs/git/git.go`**：`AreaState` = 在**该区里**跑 `git status --porcelain`（为此把 `run` 拆出 `runIn(dir, …)`）**＋** `merge-base --is-ancestor kiln/<issue> HEAD`；`RemoveWorkspace` 的两道前置拒绝改读同一个 `AreaState`。
- **`internal/vcs/jj/jj.go`**：`AreaState` = `jj -R <区> diff --summary` 非空即脏；`Unmerged` 恒假（见上表）；`RemoveWorkspace` 同源。
- **`internal/cli/cli.go`**：`workspaceList` 逐区取 `AreaState`，尾列渲染（`areaStateWord`）；`usageCommands` 的 `worktree list` 描述改为"列出全部 worktree（含 main）及各自状态"。

### 3. 真机验证（临时仓，新二进制）

**git**——一轮走完四种状态：

```
$ kiln worktree new a1 && kiln worktree list
<repo>              main（只读/只合并）  —
<repo>/.kiln/ws/a1  worktree a1          干净

$（区内落一个未提交文件）kiln worktree list            # → 有改动
$ kiln worktree remove a1
删除 worktree 失败: worktree 还有未提交的改动: a1（要丢弃请加 --force）      exit 1

$（区内 git commit 一个 main 没有的提交）kiln worktree list   # → 未合并（区本身干净）
$ kiln worktree remove a1
删除 worktree 失败: 分支 kiln/a1 还有未合并的提交（要丢弃请加 --force）      exit 1

$ git merge --ff-only kiln/a1 && kiln worktree list     # → 干净
$ kiln worktree remove a1                                                  exit 0
```

**jj（colocated）**：同序跑一遍——`干净` → `有改动`（remove 同样拒）→ **区内 `jj commit` 后仍是 `干净`，`remove` 直接成功**（提交不丢，故无"未合并"档）。

两后端共同印证了 A 的目的：**`list` 报什么，`remove` 就按什么拒**——上面四次拒绝的措辞，与它前一行 `list` 给出的状态逐一对上。

### 4. 测试

- **`internal/vcs/git/git_test.go`**：`TestAreaStateMatchesWhyRemoveRefuses`——真 git 仓里造三个区（干净 / 有改动 / 有未合并提交），断言 `AreaState` 的取值**与 `RemoveWorkspace` 的接受或拒绝逐一对齐**，并断言 main 报零值。
- **`internal/cli/cli_test.go`**：假后端补 `AreaState`；`TestWorktreeListShowsAreaState`（四态表驱动）、`TestWorktreeListLeavesMainUnjudged`（main 行须以 `—` 收尾且不得出现"干净"）、`TestWorktreeListFailsWhenStateIsUnknown`（答不出 → exit 1 且 stderr 点明状态读取失败）。
- `go vet ./...` / `go test ./...` **全绿**；`gofmt -l` 空（在把行尾归一为 LF 的镜像上验的，见取舍 4）。

### 5. 发现 / 取舍

1. **`remove` 的"脏区"拒绝语换了主人**：以前 git 侧那句是 git 自己的英文（`contains modified or untracked files, use --force to delete it`），现在由 kiln 先判先拒，两后端**同一句话**。git 自带的拒绝**保留为兜底**——先行检查遗漏的边角仍会被它拦住。
2. **判据只剩一条**：`AreaState` 是 `list` 与 `remove` 的唯一来源，问题 2 担心的"`list` 说干净、`remove` 却拒"从结构上不可能出现。
3. **`list` 现在真的在读盘**：从"读注册表"变成"N 个区各跑 1~2 条后端命令"（git：`status --porcelain` + `merge-base`；jj：`diff --summary`）。X 是用户点的，这笔代价是明账。
4. **main 工作树的行尾仍是历史遗留**：`.gitattributes`（`* text=auto eol=lf`）只保证**新检出的区**是 LF；main 里早年检出的文件（`internal/vcs/vcs.go`、`git_test.go`、`vcs_test.go`、`config_test.go`、`include.go`、`jj_test.go`）在**工作树里仍是 CRLF**，于是 `gofmt -l` 在 main 会误报这几只——本轮改动的提交内容仍被 git 归一为 LF，**不影响仓库**。要不要一次性重检出把工作树刷成 LF，留给 conclude（或另开 issue）。
5. **main 一行的 `—` 是刻意的**：不写就等于"干净"，而 main 根本不在"能不能删"的讨论里。

### 6. 未做 / 待定（留给 conclude）

- **`--status` / `--no-status` 开关**（候选 Y / Z）本轮未做——X 已定。
- **与 `kiln status`（全库概览）联动**：未做。
- **`jj` 的"未合回 main"若要成为第四档**：需先有"jj 侧丢不丢"的正例，现在没有。
- **状态列的颜色 / 记号**（候选 Q 的紧凑记号）未做。
- **main 工作树的行尾刷不刷**（见取舍 4）——需 conclude 定。
