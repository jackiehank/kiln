# issue: worktree-list-status — `kiln worktree list` 要不要带状态列（哪个区能安全删）

- **状态**：已完成（conclude）
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
| **档位** | **`干净` / `有改动` / `未合并`**；两者同时成立写 **`有改动，未合并`**（一列里并列，不拆多列）。**分支后补第四档 `目录已删`**，见 test（续） | 三档正好是"能不能安全删"的三个答案；一格里写完，读表不会看错列 |
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
4. **main 工作树的行尾仍是历史遗留**：`.gitattributes`（`* text=auto eol=lf`）只保证**新检出的区**是 LF；main 里早年检出的文件（`internal/vcs/vcs.go`、`git_test.go`、`vcs_test.go`、`config_test.go`、`include.go`、`jj_test.go` 等，共 13 只）在**工作树里仍是 CRLF**，于是 `gofmt -l` 在 main 会误报这几只——本轮改动的提交内容仍被 git 归一为 LF，**不影响仓库**。（→ 已在 test（再续）收口。）
5. **main 一行的 `—` 是刻意的**：不写就等于"干净"，而 main 根本不在"能不能删"的讨论里。

### 6. 未做 / 待定（留给 conclude）

- **`--status` / `--no-status` 开关**（候选 Y / Z）本轮未做——X 已定。
- **与 `kiln status`（全库概览）联动**：未做。
- **`jj` 的"未合回 main"若要再补一档**：需先有"jj 侧丢不丢"的正例，现在没有。
- **状态列的颜色 / 记号**（候选 Q 的紧凑记号）未做。
- ~~**main 工作树的行尾刷不刷**~~ → **已刷新为 LF**，见 test（再续）。

## test（续）：目录被删的区（prunable）——补第四档「目录已删」

> 起因：A（判定与 `remove` 同源）让 `list` **第一次真的去读每个区**，于是撞出一个 `ask` 问题 1 里列过、上一轮先按三档答掉的边角：**注册还在、目录没了**。

### 1. 撞到的现场

`kiln worktree new a1` 之后**手删区目录** `.kiln/ws/a1`，git 仍登记着它：

```
$ git worktree list
<repo>              ...
<repo>/.kiln/ws/a1  ... [kiln/a1] prunable

$ kiln worktree list          # 三档版本
读取 worktree 状态失败: git status: chdir <区>: The system cannot find the file specified.
                                                                              exit 1
```

**整条 `list` 挂掉**——而"区还在不在"恰恰是这时最想知道的事。三档版本把"读不到"一刀切成"失败"，撞在这里就过头了：它是一个**已知事实**（目录没了），不是读失败。

### 2. 顺带证伪一条旧结论：`git clean -xdf` 并不会删掉区

`cli-necessity` 的取舍里写过"区放在 `.kiln/` 下 → 主干跑 `git clean -xdf` 会把全部区一起删掉"。**本机实测不成立**：

```
$ git clean -xdf -n
Would skip repository .kiln/ws/a1      ← git 把区当嵌套仓库，跳过
Would remove .kiln/.gitignore
Would remove .kilnconfig.jsonc

$ git clean -xffd -n
Would remove .kiln/                    ← 要 -ff 才会连区一起抹
```

于是"目录被删"的真实入口是**手删**（或 `-xffd`）；而三档版本偏偏在这个入口上把整条命令弄挂了。（附带更正：`-xdf` 会删掉 `.kiln/.gitignore`——不过 `worktree new` 下次会把它写回，自愈。**旧结论该改**。）

### 3. 补丁：第四档 `目录已删`

- **`AreaState` 加 `Gone`**：git 侧 `status --porcelain` 失败**且目录确实不存在** → 报 `Gone`；**读得出错但目录还在**仍照旧报错（上表"答不出就退 1"不变）。
- **渲染**：`目录已删`；与未合并并存写 **`目录已删，未合并`**（分支照样在，删它仍会丢提交）。
- **jj 侧天然没有这一档**：区目录没了之后 `jj workspace list` 干脆不再列它（`-T` 取 `root` 为空 → 该记录被跳过），故 jj 不受影响。
- **`remove` 不动**：这种区本来就删不掉（`git worktree remove` 拒绝一个目录已不在的 worktree），三档版本同样失败，**非本轮引入**；要"能清掉它"得让 `remove` 先跑 `git worktree prune`，记在未做。

### 4. 真机验证（新二进制）

```
$ git worktree list                     # 手删 a1 / a2 的目录后
<repo>/.kiln/ws/a1  ... [kiln/a1] prunable
<repo>/.kiln/ws/a2  ... [kiln/a2] prunable

$ kiln worktree list                    # 不再整条挂掉
<repo>              main（只读/只合并）  —
<repo>/.kiln/ws/a1  worktree a1          目录已删
<repo>/.kiln/ws/a2  worktree a2          目录已删，未合并
                                                                              exit 0
```

（a2 是"目录已删"与"未合并"并存的活例：目录没了，可 `kiln/a2` 分支仍在、还带着 main 没拿到的提交。）

### 5. 测试

- **`internal/vcs/git/git_test.go`**：`TestAreaStateMatchesWhyRemoveRefuses` 追加一段——建区、`os.RemoveAll(区)`，断言 `AreaState` 报 `Gone` 而**不报错**。
- **`internal/cli/cli_test.go`**：状态列表补 `目录已删` 与 `目录已删，未合并` 两行。
- `go vet ./...` / `go test ./...` **全绿**；`gofmt -l` 空（LF 镜像上验）。

### 6. 未做（留给 conclude）

- **`remove` 清不掉这种区**：目录已删的区要清掉，得先 `git worktree prune`（我们目前只"按 issue 找区 → `worktree remove`"，对 prunable 直接报错）。要不要让 `remove` 顺手 prune，留给 conclude。
- **`.kiln/.gitignore` 被 `-xdf` 删掉后的自愈**（第 2 节）未做验证以外的事——`worktree new` 会重写，够用。

## test（再续）：行尾收口——Windows 例外规则 + 把 main 工作树刷成 LF

> 收掉 test（一）取舍 4 的那笔遗留：13 个文件的**工作树副本**是 CRLF（提交内容本就 LF），`gofmt -l` 在 main 因此误报。

### 1. `.gitattributes` 补齐 Windows 例外（用户定）

```
* text=auto eol=lf                        # 默认：一律 LF
*.bat  *.cmd             → text eol=crlf  # label / goto 一类构造在纯 LF 下会出错
*.ps1  *.psm1  *.psd1    → text eol=crlf  # PowerShell 自己吃得下 LF，但 Windows 工具链普遍只认 CRLF
```

**后写的规则覆盖先写的**，所以这几行只从上面那条通配里"捞出"脚本后缀。本仓现有 39 个跟踪文件里**没有任何脚本后缀**（只有 `.go` / `.md` / `.jsonc` / `.mod` / 属性文件本身），这几行是**前瞻**——真出现 `.ps1` / `.bat` 时不至于再补一次单独的行尾修正。

`git check-attr -a` 验证：`demo.ps1` / `demo.bat` / `demo.cmd` / `demo.psm1` / `demo.psd1` → `text: set, eol: crlf`；`internal/vcs/vcs.go` / `docs/x.md` → `text: auto, eol: lf`。

### 2. 把 main 工作树刷成 LF

那 13 只（`.gitignore`、`go.mod`、5 个旧 `docs/*.md`、6 个旧 `.go`）是**属性加上之前**检出的，工作树副本才留成 CRLF；索引里的 blob 一直就是 LF。修法是把文件删掉再 `git checkout --` 一次（内容逐字节相同，只差行尾）：

```
$ git ls-files --eol | grep w/crlf     → （无，全部 w/lf）
$ git status --short                   → 干净（行尾变化被属性归一，不算改动）
$ gofmt -l .                           → 空    ← 此前在 main 会误报那几只
```

**代价**：一次性重写 13 个文件；**不进提交**（提交内容前后一致，故工作区反而是干净）。

### 3. 校验

- `git ls-files --eol` 全 `w/lf`；`git check-attr` 脚本后缀 `eol=crlf`、其余 `eol=lf`。
- `go vet ./...` / `go test ./...` **全绿**；`gofmt -l` **在 main 直接为空**——此前每轮都得借一只把 CRLF 归一过的 LF 镜像来验格式，现在不必了。

## 结论（已定）

**定了：`kiln worktree list` 尾列各区状态——`干净` / `有改动` / `未合并` / `目录已删`（可并列），`main` 一行报 `—`；判定收在 `vcs.Workspace.AreaState` 一处，`list` 与 `remove` 读同一份答案。**

### 1. 逐条裁定（对 ask 的 7 个问题）

| # | 问题 | 结论 |
| --- | --- | --- |
| 1 | **要哪几档** | **四档**：`干净` / `有改动` / `未合并` / `目录已删`；并列写在一格（`有改动，未合并`、`目录已删，未合并`），不拆多列。第四档是 test（续）实测补上的——"注册还在、目录没了"是 git 的真实状态（`prunable`） |
| 2 | **判定同源吗** | **同源（候选 A）**：`AreaState` 是唯一来源；两后端各自实现，`list` 与 `remove` 共用 |
| 3 | **代价与时机** | **默认全探（候选 X）**：`list` 每次为 N 个区各跑 1~2 条后端命令；**不加开关** |
| 4 | **输出形态** | **候选 P**：三列 `<路径>  <角色>  <状态>`，路径与角色按显示宽度对齐 |
| 5 | **main 一行** | **`—`**：main 不是专用区、没有 issue 分支可判；留空会被读成"干净" |
| 6 | **后端中立** | **同口径、档位可不同**：git 有"未合并"，jj 没有（`workspace forget` 不丢提交）；`目录已删` 在 jj 侧也不会出现（`jj workspace list` 干脆不再列它） |
| 7 | **作用域** | **只改 `worktree list`**；与 `kiln status`（全库概览）联动**不做** |

### 2. 留下的不变式

**一个区"能不能安全删"，全库只有一个答案——`vcs.Workspace.AreaState`。** `list` 只负责把它显示出来，`remove` 只负责按它拒绝或放行；两者不许各判一遍。

推论有三：① 档位按后端能力裁剪（git 四档、jj 两档），但**口径一致**；② "读不出状态"仍算失败（exit 1），而"已知的没有"（`目录已删`）是**状态**、不是失败；③ 状态列**默认常开**——它的价值正是"不必先试删"，做成 flag 等于把摩擦挪个地方。

### 3. 落地物

- **代码**：[internal/vcs/vcs.go](../internal/vcs/vcs.go)（`AreaState{Gone, Dirty, Unmerged}` + `Workspace.AreaState`）；[internal/vcs/git/git.go](../internal/vcs/git/git.go)（`status --porcelain` 在该区里跑 + `merge-base --is-ancestor`，为此拆出 `runIn(dir, …)`）；[internal/vcs/jj/jj.go](../internal/vcs/jj/jj.go)（`diff --summary`）；[internal/cli/cli.go](../internal/cli/cli.go)（`workspaceList` 尾列 + `areaStateWord`）。
- **一处行为变更**：`remove` 的"脏区"拒绝语不再用 git 的英文，改为 kiln 自己那句（两后端同一句）；git 自带的拒绝**保留为兜底**。
- **仓库约定**：[.gitattributes](../.gitattributes) 分后缀例外（`*.bat` / `*.cmd` / `*.ps1` / `*.psm1` / `*.psd1` 留 CRLF，其余一律 LF），并把 main 工作树里 13 个旧文件的副本刷成 LF。
- **测试**：真 git 仓的 `TestAreaStateMatchesWhyRemoveRefuses`（四态与 `remove` 逐一对齐，`Gone` 不报错）；cli 的 `TestWorktreeListShowsAreaState`（六行表驱动）、`TestWorktreeListLeavesMainUnjudged`、`TestWorktreeListFailsWhenStateIsUnknown`。`gofmt -l` 空、`go vet` / `go test ./...` 全绿。
- **文档**：[docs/mvp-plan.md](mvp-plan.md) §10 加一条；本文件。

### 4. 一并纠正的一条旧结论

`cli-necessity` 的取舍里写过"区放在 `.kiln/` 下 → 主干跑 `git clean -xdf` 会把全部区删掉"。**不成立**：git 跳过嵌套仓库（干跑打印 `Would skip repository .kiln/ws/<issue>`），要 `git clean -xffd` 才连区一起抹。附带：`-xdf` 确实会删掉 `.kiln/.gitignore`，但 `worktree new` 会把它写回，**自愈**。

（`cli-necessity.md` 的**历史正文不改写**——那是当时的记录；更正落在本节，并与项目记忆同步。）

### 5. 未做（要可另开 issue）

1. **`remove` 清不掉"目录已删"的区**——要清得先跑 `git worktree prune`。
2. **状态列的颜色 / 紧凑记号**（候选 Q）。
3. **`--status` / `--no-status` 开关**（候选 Y / Z）——X 已定，不做。
4. **与 `kiln status` 联动**。
5. **jj 侧补"未合回 main"一档**——需先有"jj 会丢东西"的正例，现在没有。
