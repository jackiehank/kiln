# issue: worktree-hint — worktree=on 时，把「约束」提示放在 kiln 的哪个调用点

- **domain**：cli
- **关联**：derived-from `worktree-adopt`

## 背景

**起因**：仓库设了 `worktree: on`，agent **不开区、直接在 main 编辑**，等到 `kiln commit` 被护栏拒绝时才发现——那时工作已经做在 main 上了。

`worktree-adopt` 已裁定：**不做"把改动搬进区"那条路（A）**，因为痛点是**"得知约束的时机太晚"**，不是"改动搬不动"。接力的是**只给提示与配方、零机制**的方案。

**关键前提（要写清）**：kiln **只看得见自己的调用点**，看不见"编辑文件"这个动作。所以目标不是"保证拦住"，而是**最大化拦截概率**——落点要按"agent 何时会调用 kiln"来排。

## 已定（本次会话裁定）

| 项 | 决定 |
| --- | --- |
| **落点** | **`kiln do` + 定位类读命令**：`do` 补"提交时刻"的暗道（`git commit -m "$(kiln do …)"` 今天**完全不经过** `commit` 的护栏，一声不响）；读命令（`status` / `path`）给**动手前**的机会，且 `path` 能带上**具体 issue 名** |
| **触发条件** | `worktree=on` **且** 当前**不在**专用区（`IsDedicated` 为假）——否则不提示 |
| **按后端分派** | kiln 本来就检测得出用哪个后端（`vcs.Detect`；`auto` 下 `.jj` 优先）：**jj → jj 配方，git → git 配方** |
| **拒绝语一并做** | `commit` 的拒绝语按 **main 脏 / 净**分派（见下「第二个交付物」） |
| **四条硬约束** | ① 提示**只进 stderr**，**绝不进 stdout**（`do` 的 stdout 必须是一字不差的提交信息；读视图的 stdout 要保持可解析）；② **exit code 一律不变**（提示不是失败）；③ **按后端分派**配方；④ **诚实标注边界**（见下） |

## 要回答的问题

### 1. **这句提示落在哪些调用点**（第一问）

已定 **`do` + 定位类读命令**；仍未定的是**"定位类读命令"到底是哪几条**：

- `kiln path <issue>`（能带具体 issue，最具体）？
- `kiln status`（全库概览，agent 接手时最常跑）？
- `kiln list [--domain=]` / `kiln graph` 要不要也算？
- `kiln check`（收尾时跑，**太晚**——还值得吗）？

### 2. 措辞与"脏 / 净"

读命令**也能**查到 main 脏不脏（照 `AreaState` 的读法），那么读视图上的提示要不要**也分两态**？

- **净** → 一句动作指引：`提示: 本仓要求动作在 worktree 提交——先 kiln worktree new <issue>`
- **脏** → 追加配方（后端分派）：`… 你已有未提交改动：git 用 stash -u → 建区 → 区内 pop；jj 用 workspace add -r @ + 主干 jj edit @-`

### 3. 噪音的取舍（要**写明**，不是默认）

- 每调一次就提示一次 → **在 main 里只读的人**（如只看 `status`）也会看到。接受这个噪音，还是收窄（只在 `path <issue>` 打、`status` 不打）？
- 单次会话只提示一次？——**注意**：kiln 的设计原则是"**无隐式状态**"（不落本地状态），所以这需要引入一个小状态文件（`.kiln/` 下的本地状态），**与原则有张力**，要明写取舍。

### 4. **没有仓库时提示什么**（用户点名要处理）

- `kiln do` **不依赖仓库**（它只打印提交信息）——那里该提示什么？倾向**不打**（没有仓库，就没有"隔离"可言）；但此时 `vcs.Detect` 会失败，要**静默跳过**而不是把错误放大。
- 读命令在没有仓库时**本来就报错**（`打开仓库失败: …没有 .git 也没有 .jj`）——那已经说清了，**不必再加提示**。
- 还有一种："有仓库但 `worktree=off`" → **不提示**（用户已明确选择不做隔离）。

### 5. **第二个交付物：`commit` 拒绝语按 main 脏 / 净分派**

**现状是个会误导人的缺陷**：拒绝语说「先 `kiln worktree new <issue>`」，但**新建的区是干净检出**（collect 实测）——照做会把改动**晾在 main**；jj 侧甚至可能落一条**空提交**（jj 允许空提交；**待实现轮验证**）。

| main 状态 | 拒绝语该给什么 |
| --- | --- |
| **净** | 现状即可（先建区 → cd 进去提交；或 `--force`） |
| **脏** | **加后端配方**（不要再只说"先建区"）：git = `stash -u` → 建区 → 区内 `pop`；jj = 建区（`-r @`）→ 主干 `jj edit @-`（**别** `jj restore`，会级联 rebase 连坐新区） |

- 判定可行性：有现成先例——本会话刚落的 `AreaState`（`git status --porcelain` / `jj diff --summary`），在 **main** 上照跑即可。
- 这两条配方是 **VCS 级**（不是项目级做法）→ 不违反 `worktree-mode` test（三续）立的「工具中立」硬约束。

### 6. 实现落点与代价

- `kiln do` **目前不打开仓库**（只在 `commit` 时才 `backend()`）。加提示意味着 `do` 也要**尽力而为地** detect 后端 + 查 `IsDedicated` → **detect 失败要静默跳过**，不能让一个纯打印命令因为"没仓库"而报错。
- 读命令要加"当前是否专用区"的判定 = 每次多跑一条后端命令（jj 侧 `workspace list`）——与 `worktree list` 的 X 决定同类代价，可接受但要记明。

## 非目标

- **不做 `--adopt` / 不代跑迁移**（`worktree-adopt` 已裁定 A）。
- **不改护栏语义**：`worktree` 开关、`--force` 一次性豁免不动；提示不改变任何命令的**成功 / 失败与退出码**。
- **不装 git hook**（`mvp-plan` §9 的第三条一直是"可选、非 MVP"；hook 能兜住绕过 kiln 的提交，但它会往用户仓库里塞文件，与"过滤器不是注册器"的定位有张力——**留作后续候选**）。
- **本轮只开题**：不改代码。

## 备注

- 来源：`worktree-adopt` 的 ask / collect / 结论（含两条后端实测配方）；`worktree-mode` test（三续）立的「工具中立」；`mvp-plan` §9「Agent 集成」（`kiln do` 是钦定的 agent 集成点）。
- 用户原话：*"既然用户已经配置了，kiln 是有这个信息的，kiln 可以在 agent 调用时添加提示。那么就要考虑这句提示加在哪里合适。"* 以及 *"如果用 jj 就给 jj 的提示，用 git 给 git 的提示，还得考虑目录没有 git 和 jj 仓库提示什么。"*

## hypothesize：提示收窄到 `do`；`-h` 走措辞升格；「没有仓库」先修判定

### 0. 本轮用户已定

| 项 | 决定 |
| --- | --- |
| **`do` 加提示** | 合理，**采纳** |
| **读命令加提示** | **不加**（倾向）→ 采纳 |
| **没有仓库** | kiln 要**明确提示"没有仓库"** |

### 1. 落点收缩：只剩 `do`（+ `commit` 的拒绝语）

- **代价要写明**：`do` 在**提交时刻**才出声——所以它是「**兜底 + 教育**」，**不是拦截**；agent 若整段工作都不跑 `do`，就一次也看不到。
- **换来的是**：零噪音、零状态。ask §3 的"噪音取舍"**随之关闭**——不再需要"单次会话只提示一次"那种本地状态，也就没有与"**无隐式状态**"原则的张力。
- ask §1 的"定位类读命令取哪几条"也**随之关闭**（一条都不取）。

### 2. `-h` 要不要加？（用户的开放问题）→ 建议：**不加提示块，只升格一行措辞**

- help 里**已经有两处**在说这件事：命令表里 `kiln worktree new <issue>` 的描述「建 worktree（main 只读，动作只在 worktree 提交）」、footer「`worktree 默认: on（强制 worktree）| off（允许在 main 提交）`」。缺的**不是"没说过"，而是"说的是配置，不是动作"**。
- help 是**静态**文本 → 做不到状态感知（本仓 on/off、在不在专用区、main 脏不净）——**而这些恰恰是 `do` 能给、`-h` 给不了的**。
- 往 help 里塞配方会让它变长，且要在两后端各写一套，与 help 的"工具中立、简洁"不合。
- **建议（成本≈0）**：只把 footer 那行从"配置陈述"升格为"动作指引"：
  `worktree 默认: on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）| off（允许在 main 提交）`
- **候选但倾向否**：让 `-h` 读本仓配置、打印"本仓当前 `worktree: on`"——帮助输出会随环境变化，还得在无配置 / 无仓库时回退；"当前值"本该由 `kiln status` 之类回答。

### 3. 「没有仓库」：**先修判定，再定措辞**（本轮实测撞出一个缺陷）

干净复测（本机）：

| 场景 | `config path` | `status` | `do` |
| --- | --- | --- | --- |
| 仓库根 | ✓ 打印配置路径 | ✓ | ✓（exit 0） |
| **子目录 `docs/`** | **✓** 打印 `…/.kilnconfig.jsonc`（**向上找生效**） | **✗ `打开仓库失败: . 下既没有 .git 也没有 .jj`** ← **误报！它在仓库里** | ✓（exit 0） |
| 不在任何仓库（TEMP） | ✗ `不在仓库里，定位不到 .kilnconfig.jsonc` | ✗ 同上"既没有 .git 也没有 .jj" | ✓（exit 0，**静默**） |

**根因**：**配置向上找仓库根，后端只看当前目录**（`vcs.Detect(".")` 只查 `./.git` 与 `./.jj`）——两套判定不一致，于是"没有仓库"这句话在**子目录里是假话**。

→ 所以你要的那句提示，**必须建在"向上找"的判定上**。建议：后端探测与配置**共用同一套锚定**（向上找到仓库根；找不到才说"没有仓库"）。附带收益：**kiln 在子目录里终于可用**——今天 `status` / `path` / `commit` 在子目录里一律失败，而 `config` 却能向上找到，自相矛盾。

→ 措辞直接沿用 config 侧已有的说法：**「不在仓库里」**（config 的报错就是"不在仓库里，定位不到 .kilnconfig.jsonc"），**一个概念一个词**。三类命令的出法：

1. **`do`**：本不依赖仓库 → 只打一行 **stderr 提示**，**exit 0 不变**（实测它在任何地方都是 exit 0）；
2. **读命令 / 提交命令**：那是**真失败** → 保持 exit 1，只把**起因**说准（"不在仓库里"，而不是含糊的 `.`）；
3. **`-h`**：帮助不该依赖仓库 → **不提示**。

### 4. `do` 的提示：触发条件与四态措辞

触发条件：**在仓库里** ∧ `worktree=on` ∧ **当前不在专用区**。否则不打。

| 情形 | `do` 的行为 |
| --- | --- |
| 不在仓库里 | `提示: 不在仓库里（向上找也没有 .git / .jj）`——**不再打** worktree 提示；exit 0 |
| 在仓库里、`worktree=off` | **不打**（用户已明确选择不隔离） |
| 在仓库里、on、**已在专用区** | **不打** |
| 在仓库里、on、在 main、main **净** | `提示: 本仓要求动作在 worktree 里提交——开工前先 kiln worktree new <issue>`（issue 名从 `do` 的参数里就有） |
| 在仓库里、on、在 main、main **脏** | 上面那句 + **后端配方**：git = `git stash -u` →（建区）→ 区内 `git stash pop`；jj =（建区）→ 主干 `jj edit @-`（**别** `jj restore`） |

**四条硬约束重申**：**只进 stderr**（`do` 的 stdout 必须是一字不差的提交信息）；**exit code 一律不变**；**按后端分派**；**诚实标注边界**——kiln 看不见"编辑文件"本身，只在自己的调用点出声。

### 5. 第二交付物（不变）：`commit` 拒绝语按 main 脏 / 净分派

同 ask §5；措辞与配方与 §4 同源，判定用 `AreaState` 的读法在 **main** 上跑（`git status --porcelain` / `jj diff --summary`）。

### 6. 留给 test 轮

1. **实现**：`do` 要"**尽力而为**"地探测（detect + 读配置）；**任何失败都降级为提示，绝不改 exit code**。
2. **先确认 §3 的分叉**（后端探测改向上找 vs 只改措辞），`do` 与读命令**一起对齐**。
3. **真机跑五态 × 两后端**（git / jj 临时仓：不在仓库、off、已在区内、净、脏），并核对 **stdout 未被污染**——把 `do` 的输出**原样喂给** `git commit -m` / `jj commit -m` 跑通。
4. `-h` 那行措辞改动（§2）顺带落地并核对帮助排版。

## hypothesize（续）：`status` 里怎么显示 worktree 状态

### 0. 用户的新想法

> *"`status` 本身表示状态，用户有没有要求开 worktree、现在哪个 issue 开了 worktree，其实也是一种状态。好好设计一下，希望能做到既展示信息，又简要。"*

### 1. 现状基线（实测）

仓库里有一个区（`demo`）时，`status` **只字不提**：

```
问题总数: 1（完成 0 / 进行中 1）｜domain 1 个
kiln 动作数: 2

design/demo   test
```

对照 `worktree list`：`<path>  worktree demo  干净`。→ 在 `status` 里补"有没有 / 在哪"这一层，是**纯增量**信息。

### 2. 三层「worktree 状态」，成本差一个量级（设计的地基）

| 层 | 内容 | 成本 | 来源 |
| --- | --- | --- | --- |
| **① 仓库级开关** | `worktree on / off` | **零**——配置启动时本就读了，`e.worktree` 现成 | `.kilnconfig.jsonc` |
| **② 哪些 issue 开了区** | 有 / 无 | **1 次注册表读取**（`ListWorkspaces()`：git `worktree list --porcelain` / jj `workspace list -T`），且 `WorkspaceInfo.Issue` **直接给出映射**，不用猜 | 后端注册表 |
| **③ 每个区的状态** | 干净 / 有改动 / 未合并 / 目录已删 | **N 次探盘**（每区 1~2 条命令） | `AreaState`——即 `worktree list` 的 X 代价 |

→ **`status` 只吃 ①②，坚决不碰 ③**。③ 归 `worktree list`——这正是本仓已有的分工：`status` = domain 视图 / `graph` = 关系视图 / `path` = 过程视图 / **`worktree list` = 区视图**。把 ③ 塞进 `status`，等于让**最常跑的命令**背上 N 次探盘。

### 3. 推荐形态（既展示信息，又简要）

```
$ kiln status
问题总数: 26（完成 19 / 进行中 7）｜domain 2 个｜worktree on
kiln 动作数: 93

cli/action-diff              ✓ conclude
…
design/cli-necessity           test  ▣
…

▣ = 已开 worktree（1 个）——各自状态见 kiln worktree list
```

三条规则：

1. **头行尾加一段**（`｜worktree on` / `｜worktree off`）——回答"**用户有没有要求开**"。**零成本、零新增行**，沿用现有的 `｜` 分段习惯。
2. **有区的 issue 行尾加一个记号**（`▣`）——回答"**哪个**"。没有区的行**一个字都不动**（记号列对它们只是空白，噪声≈0）。
3. **有区时才追加一行 legend**——记号含义 + 计数 + **指路**到 `worktree list`。这就是"既展示又简要"的落点：**解释一次，不在每行重复**。

**术语合规**：记号是**符号**，legend 里用的是 **`worktree`** 这个词——不引入"区"这个已被废掉的中文别名（`worktree-mode` test 四续立的规矩：用户可见面一个概念一个词）。

### 4. 候选与被我否掉的

| 方案 | 形态 | 判断 |
| --- | --- | --- |
| **A（推荐）** | 头行开关 + issue 行记号 + 有区才出现的 legend | ✅ 展示"有没有 + 哪个"，成本 ①② |
| **B** | 只在头行报汇总（`worktree on（1 个）`），不标具体 issue | ✗ 答不了"**哪个**" |
| **C** | 末尾**内嵌一个区区块**（照 `worktree list` 的样子列出来） | ✗ 与 `worktree list` 重复，还把 ③ 的成本拉进来 |
| **D** | 每个区的状态（干净/脏/未合并）也进 `status` | ✗ N 次探盘 + 与 `worktree list` 分工冲突 |
| **E** | 用文字列 `worktree` 代替记号 | ✗ 每行多 8 个字符，与"简要"冲突；**记号 + legend 更省** |
| **F** | 头行再报「**你站在哪个区**」（`当前：worktree demo`） | **候选（倾向加）**：回答"我此刻在哪"，成本≈0（区列表已在手 + 一次 `IsDedicated`）；待看头行会不会太长 |

### 5. 待定（留给下一轮 / test）

1. **记号用哪个符号**：`▣` / `●` / `+`——要一眼懂且窄；legend 兜底解释。
2. **`worktree off` 时**：头行仍显示开关（它回答"有没有要求"）；**有区照样标**（区存在是事实，与开关无关）。
3. **计数放哪**：头行尾（`worktree on（1 个）`）还是 legend 里（`（1 个）`）——倾向 legend（头行短），但头行更早可见。
4. **要不要为它多开一次后端调用**：① 零成本；② 一次注册表读取（与 `worktree list` 同笔，可接受）。可选优化：**只在 `worktree=on` 时才读**（off 且无区时连这次也省掉）。
5. **无仓库 / 子目录**：与第一论 §3 的分叉绑在一起（同一个"向上找"的判定）——`status` 本来就要求仓库；子目录误报那条要一起修。

### 6. 范围提醒

这已是本 issue 的**第三个交付物**（`do` 提示 / `commit` 拒绝语 / `status` 展示）。三者同源（都在说"worktree 这件事"），**留着一起做是合理的**；若你更想让 status 这条独立成线（如 `status-worktree`），**现在拆最省**（一行代码都还没动）。

## collect：记号符号的字体 / 终端兼容性（本机字形表实测）

> 用户要求：*"符号用 `▣` 的话，看看一般字体或终端的兼容性如何，不兼容就换其他兼容性更好的。"*
> 方法：用 .NET/WPF 的 `GlyphTypeface.CharacterToGlyphMap` **直接查本机各字体的字形表**（不是猜、也不是渲染截图糊在一起看），并拿**仓库已经在打印的符号**当基准（`✓`、`├──`、`│`、`—`）。

| 字体 | U+25A3 `▣` | U+25A0 `■` | U+25A1 `□` | U+25CF `●` | U+25CB `○` | U+2713 `✓` | U+251C `├` | U+2502 `│` | U+2014 `—` | U+002B `+` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Consolas | **✗** | ✓ | ✓ | ✓ | ✓ | **✗** | ✓ | ✓ | ✓ | ✓ |
| Cascadia Mono | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Cascadia Code | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Lucida Console | **✗** | ✓ | **✗** | **✗** | ✓ | **✗** | ✓ | ✓ | ✓ | ✓ |
| Courier New | **✗** | ✓ | ✓ | ✓ | ✓ | **✗** | ✓ | ✓ | ✓ | ✓ |
| MS Gothic | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| SimSun | **✗** | ✓ | ✓ | ✓ | ✓ | **✗** | ✓ | ✓ | ✓ | ✓ |
| Segoe UI | **✗** | ✓ | ✓ | ✓ | ✓ | **✗** | ✗ | ✗ | ✓ | ✓ |
| DejaVu Sans Mono（本机这份连盒线都没有，大概是某包 vendored 的裁剪版） | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ |

三条读数：

1. **`▣`（U+25A3）覆盖很差**：9 个字体里只有 Cascadia（Mono / Code）与 MS Gothic 有；Consolas / Lucida Console / Courier New / SimSun / Segoe UI **全缺**。
2. **有意思的对照**：仓库**已经在打印的 `✓`（U+2713）覆盖一样差**（同样缺 Consolas / Lucida / Courier New / SimSun / Segoe UI）——即**今天这份输出在这些字体下本来就缺字形**。
3. **`■`（U+25A0）是候选中覆盖最好的**：9 个字体里 8 个有（唯一例外是那份来路不明的 DejaVu）。`□` / `○` 次之，`●` 又次（Lucida 缺）。

→ **结论：`▣` 换 `■`。** 不只是"数量多"：**`✓` 缺失的那 5 个字体里，`■` 全都还在**——所以新加的这列**比整份输出里已有的记号更稳**，不会成为最脆弱的一环。语义由那行 legend 兜底（`■ = 已开 worktree`）。

（附注：`+` 之类纯 ASCII 兼容性当然最好，但与 `status` 已有的记号风格（`✓`、`├──`）不成体系；`■` 在**风格一致**与**稳健**之间取平衡。另：以上是**本机字形表**的实测，不外推到别人的机器——但"**比已在用的符号更稳**"这个判据是可移植的。）

### 收敛（用户两条指示落定）

| # | 项 | 定 |
| --- | --- | --- |
| 1 | **记号符号** | **`■`（U+25A0）** 取代 `▣`（依据见上）——收掉原「待定 1」 |
| 2 | **`worktree off` 时** | 头行**仍显示**开关（它回答"用户有没有要求开"）；**有区照样标**——收掉原「待定 2」 |
| 3 | **区数量** | **上头行**（不只在 legend）——收掉原「待定 3」 |
| 4 | **有区的 issue** | 行尾 `■` |
| 5 | **legend** | 仍**只在有区时**出现；计数已上头行，legend 不再重复：`■ = 已开 worktree——各自状态见 kiln worktree list` |

### 更新后的形态

```
$ kiln status
问题总数: 26（完成 19 / 进行中 7）｜domain 2 个｜worktree on（已开 1 个）
kiln 动作数: 93

cli/action-diff              ✓ conclude
…
design/cli-necessity           test  ■
…

■ = 已开 worktree——各自状态见 kiln worktree list
```

`off` 时同一位置写 `｜worktree off（已开 0 个）`。

### 已定（用户本轮指示，落地见 §test）

- ~~`（已开 0 个）` 在 off 且 0 个时要不要省略~~ → **保留**：用户指示"数量也在头行显示"，格式固定才好读。
- ~~「你站在哪个区」放哪~~ → **放 legend**（用户指定），形如 `｜当前：worktree demo`。

## test：落地（切片一）——探测向上找 + `status` 的 worktree 展示

> 本轮落两块：**① 后端探测改「向上找仓库根」**（修子目录误报，为「不在仓库里」那句措辞打底）；**② `status` 显示 worktree 状态**（头行开关与区数量 / issue 行记号 / legend 含"当前在哪"）。
> 切片二（`do` 提示、`-h` 措辞、`commit` 拒绝语按脏净分派）见文末。

### 1. 落地内容

- **`internal/vcs`**：`Detect` 先做 `repoRoot()` —— 走祖先链找第一个带 `.jj` / `.git` 的目录，**与 `config.Path` 同一套锚定**；新增 `notARepository()`，**一个说法**「不在仓库里（从 <绝对路径> 向上也没有 .git / .jj）」，且**报出搜索起点**（不再是含糊的 `.`）；新增 `AreaAt()`（按路径包含找"我站在哪个区"，**最深匹配**）与 `within()`。
- **`internal/cli`**：`status` 头行尾加 `｜worktree on（已开 N 个）`（`areasOpen()` 只数**专用区**，main 不算）；有区的 issue 行尾加 `■`（`areaIssues()` / `areaMark()`）；有区时末行 legend，站在区内时带 `｜当前：worktree <issue>`；`knownAreas()` 是 **best effort**——读不到就当 0，**绝不让 status 失败**。

### 2. 真机验证

```
# ① 子目录（原来误报"没有仓库"）
$ cd docs && kiln status
问题总数: 28（完成 20 / 进行中 8）｜domain 2 个｜worktree off（已开 0 个）     ← 原来是 打开仓库失败: . 下既没有 .git 也没有 .jj

# ② 真不在任何仓库
$ cd <temp> && kiln status
打开仓库失败: 不在仓库里（从 C:\...\kiln-norepo3 向上也没有 .git / .jj）

# ③ 有区的仓库（main 上跑）
问题总数: 1（完成 0 / 进行中 1）｜domain 1 个｜worktree on（已开 1 个）
kiln 动作数: 2

design/demo   test  ■

■ = 已开 worktree——各自状态见 kiln worktree list

# ④ 从区内跑
■ = 已开 worktree——各自状态见 kiln worktree list｜当前：worktree demo
```

### 3. 测试

- **`internal/vcs`**：`TestDetectSearchesUpward`（子目录 → 找到父级仓库；`prefer=jj` 仍按存在的标记报错）、`TestDetectDoesNotReportARepositoryOutsideOne`（措辞 + 报出起点）、`TestAreaAtFindsTheContainingArea`（区内深处 / main / 无匹配 / 空表）。
- **`internal/cli`**：`TestStatusShowsWorktreeModeAndAreas`、`TestStatusStatesWorktreeOffWithNoAreas`、`TestStatusLegendNamesTheCurrentArea`；`TestStatusRendersDomainBlocks` 随签名更新。
- `gofmt -l` 空、`go vet` / `go test ./...` **全绿**。

### 4. 实现中撞到的两个坑（都被单测当场逮住）

1. **`AreaAt` 会被 main 抢先命中**：区就嵌在 main **里面**（`.kiln/ws/<issue>`），"第一段包含"匹配时 main 总是先中，于是"当前在哪"会答成 main。→ 改成**最深匹配**（路径最长者胜）。
2. **计数把 main 算成"已开的一个"**：`len(areas)` 连 main 一起数了。→ 加 `areasOpen()`，只数专用区。

### 5. 顺带观察（不是本轮引入，但值得记住）

**在区内跑 `status`，读的是那个区的分支历史**：实测在区内看到 `design/demo   ask`，而 main 上是 `test`——因为区自建区那一刻起就是**独立分支**，`test` 那笔在 main 上、还没进区。→ 这是"区 = 一次检出 / 独立分支"的必然。**区内的 `status` 可能落后于 main。**

### 6. 切片二（未做）

`do` 提示（五态 + 后端配方）、`-h` footer 措辞升格、`commit` 拒绝语按 main 脏 / 净分派（需要一个新的 `Workspace.Dirty()` ——`AreaState` 刻意不报 main）。

## test（切片二 A）：`do` 的五态提示 + `-h` 措辞升格

### 1. 落地内容

- **`internal/vcs`**：新增 `ErrNoRepository` 哨兵——把"不在仓库里"从"打不开仓库"里**独立出来**，好让 `do` 认出并说出这一句；`Workspace` 加 **`Dirty()`**（git = `status --porcelain`、jj = `diff --summary`）：**`AreaState` 刻意不报 main**，所以需要另一个只问"我站的地方脏不脏"的方法。
- **`internal/cli`**：`do` 打完消息后在 **stderr** 给一行提示（`worktreeTip`），**stdout 一字不动、exit code 不变**；配方按后端分派（`carryRecipe`，用新记下的 `env.kind` 路由——由 `openRepo` 在开仓时记录）；`-h` 的 footer 那行从"配置陈述"升格为"**动作指引**"（`worktree 默认: on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）| off（…）`）。

### 2. 五态真机（git / jj 两个临时仓）

| 态 | 输出 |
| --- | --- |
| 不在仓库里 | `提示: 不在仓库里（从 <绝对路径> 向上也没有 .git / .jj）`，**exit 0** |
| `worktree off` / 已在专用区 | **静默**（stderr 0 字节） |
| main + 净 + on | `提示: 本仓要求动作在 worktree 里提交——开工前先 kiln worktree new <issue>` |
| main + 脏（git） | 上句改为 `…；你已有未提交改动——git：git stash -u → kiln worktree new <issue> → 进区后 git stash pop` |
| main + 脏（jj） | `…；你已有未提交改动——jj：先在仓库根 mkdir .kiln/ws，再 jj workspace add --name <issue> <区路径> -r @（新区即带上它），最后在 main 跑 jj edit @-` |

### 3. stdout 纯度（硬约束的实测）

把 `kiln do` 的 **stdout 原样**喂给 `git commit -m`：

```
$ git commit -q -m "$(kiln do demo ask --title 起点)"
$ git log -1 --format=%B      → 只有那条消息，没有"提示"
```

`do` 的 exit code 仍是 **0**——提示不改退出码。

### 4. 两条配方都**照着手跑通**了（不是只打印）

- **git**：`git stash -u` → `kiln worktree new demo` → 区内 `git stash pop` → **区内 a.txt=b、main a.txt=a** ✓（干净地"迁走"）
- **jj**：`mkdir .kiln/ws` → `jj workspace add --name demo <区> -r @` → main `jj edit @-` → **区内 b、main a** ✓

> **实现中逮住的一个真 bug**：最初的 jj 配方直接写 `jj workspace add …`——**跑不通**（`os error 3`）：**jj 不建中间目录**，而 `.kiln/ws` 那时还不存在（这正是 kiln 自己 `PrepareAreas` 先做的事）。→ 配方补上 `mkdir .kiln/ws`。**少了这一步，打印出来的配方就是个陷阱**——这条只有照着手跑才会暴露。

### 5. 测试

- **`internal/cli`**：`TestDoTipsWhenIsolationIsRequired`（stderr 有提示、stdout 无提示）、`TestDoTipCarriesTheBackendRecipe`（git / jj 两 case）、`TestDoStaysQuietWhenThereIsNothingToAsk`（off / 已在区内）、`TestDoSaysWhenThereIsNoRepository`（不在仓库仍打印消息、exit 0）。假后端补 `Dirty()`。
- `gofmt -l` 空、`go vet` / `go test ./...` **全绿**；`-h` 那行措辞已核。

### 6. 仍未做（切片二 B）

`commit` 的**拒绝语按 main 脏 / 净分派**（含同样那两条配方）——`refuseInMain` 目前不分态，所以"我已经在 main 改了文件"时它仍只会说"先建区"。

## test（切片二 B）：`commit` 拒绝语按 main 脏 / 净分派

### 1. 为什么改

现状的拒绝语在「我已经在 main 改了文件」这个场景里**是误导的**：它让你「先 `kiln worktree new <issue>`」，而新区是**干净检出**（collect 实测过），照做会把改动**晾在 main**。

### 2. 落地

`refuseInMain` 现在按 `ws.Dirty()` 分派，并接受 `issue` 参数（配方要用它）：

```
# main 净
拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。
先 `kiln worktree new demo`，再 cd 到 worktree 提交。
确要在 main 提交时加 --force（一次性豁免）；（想长期允许 main 提交，把配置里的 worktree 设为 off。）

# main 脏（git）
拒绝提交：当前是 main（只读/只合并），动作只在 worktree 里提交。
你已有未提交改动——新区是干净检出，直接建区会把改动留在 main：
git：git stash -u → kiln worktree new demo → 进区后 git stash pop
确要在 main 提交时加 --force（一次性豁免）；（…）

# main 脏（jj）—— 同一位置换成 jj 的配方
jj：先在仓库根 mkdir .kiln/ws，再 jj workspace add --name demo <区路径> -r @（新区即带上它），最后在 main 跑 jj edit @-
```

配方与 `do` 的提示**共用同一个 `carryRecipe`**——一处维护，两处出声。

> 脏态末尾那句 `--force` 的措辞在**切片二 C** 被降级为条件句——见下。

### 3. 真机

三个场景逐条核过（git 净 / git 脏 / jj 脏），措辞与上面一致，**exit 1 不变**，且脏态那条**不再出现**"先建区"的错误指引。

### 4. 测试

`TestCommitRefusalCarriesTheRecipeWhenMainIsDirty`（脏 → 有配方，**且没有**"先建区"那句）、`TestCommitRefusalPointsAtAnAreaWhenMainIsClean`（净 → 有"先建区"、无脏警告）；既有的 `TestCommitInMainIsRefused` / `TestCommitInMainWithForce` 照旧通过。

### 5. 切片二完成

`do` 提示、`-h` 措辞、`commit` 拒绝语——**三件都落地了**。下一步是 conclude。

## test（切片二 C）：脏态那句 `--force` 会诱导直接 force —— 降级为条件句

> 用户提出：*"「确要在 main 提交时加 --force」在 main 脏的情况下提示这句，会不会诱导 agent 直接 force 提交。"*

### 1. 判断：成立，但只在这一条分支上

- **脏态下成立**：拒绝语刚给出"该把改动搬进区 + 三步配方"，紧接着又并列一句"**或者加 `--force`**"。对 agent 而言是**一枚 flag vs 三条命令**——它取便宜的，结果正是护栏要防的：动作落在 main。**那句与配方在竞争，而且更短的那句更容易赢。**
- **净态没有这个毛病**：那里没有"会被搁置的工作"，"先建区"本身就是对的答案，`--force` 只是一个**合法出口**。

### 2. 但**不删**——藏起来更糟

`--force` 是**明面上的出口**。藏起来不会让它消失，只会把人推去 `git commit -m "$(kiln do …)"`（那条暗道刚堵上）：动作照样记在 main，但**连"你违反了本仓约定"这句提示都看不到了**。护栏的目的是让这件事**显式**，不是让它不可能。

### 3. 改法：保留，降级为条件句 + 说清它**不做**什么

```
（--force 只放行护栏、不把改动搬进区——只有你确实要把它留在 main 时才用；想长期允许 main 提交，把 worktree 设为 off。）
```

- **"只放行护栏"**——正面否掉"捷径"那种读法；
- **"只有你确实要…"**——条件句，不是并列选项；
- **净态措辞不动**。

> 这句里的「把 worktree 设为 off」在**切片二 D** 删掉——见下。

### 4. 测试与真机

`TestCommitRefusalCarriesTheRecipeWhenMainIsDirty` 补两条断言：脏态**不得**出现"确要在 main 提交时加 --force"，且**必须**出现"只放行护栏、不把改动搬进区"；净态断言原措辞不变。真机核过 git 脏态与净态两种输出。

## test（切片二 D）：运行时消息不再提「改配置」

> 用户：*"「把 worktree 设为 off」把这样的提示删去，我不希望诱导 agent 去改配置文件。"*

### 1. 为什么删

「想长期允许 main 提交，把配置里的 `worktree` 设为 `off`」这句，与上一轮那句 `--force` 是**同一类毛病，而且更重**：

- `.kilnconfig.jsonc` 是**随仓库提交的共享配置**——改它等于**改这个仓库对全体协作者与 agent 的约定**；
- 而读这句话的人（尤其 agent）正处在**干活中途被拦下**的那一刻，最容易顺手把拦它的东西关掉；
- 于是"拒绝一次提交"可能被升级成"**把本仓的隔离关掉**"。`--force` 至少是**一次性、有范围**的；改配置是**持久且全局**的。

→ **护栏的提示只该说"这一次怎么走通"，不该说"怎么让它永远不拦你"。** 后者是**仓库所有者**的决定，而且 `.kilnconfig.jsonc` 的注释里本来就写着这个键是干什么的。

### 2. 改法（两态都删）

```
# 净态
先 `kiln worktree new demo`，再 cd 到 worktree 提交。
确要在 main 提交时加 --force（一次性豁免）。

# 脏态
你已有未提交改动——新区是干净检出，直接建区会把改动留在 main：
git：git stash -u → kiln worktree new demo → 进区后 git stash pop
（--force 只放行护栏、不把改动搬进区——只有你确实要把它留在 main 时才用。）
```

**只动运行时消息**：`README.md`「工作区模式」那节与 `.kilnconfig.jsonc` 的注释都**保留**这条说明——它们是**给配置仓库的人看的手册**，位置本来就对。

### 3. 测试

两个拒绝语用例各补一条**反向断言**：**不得**出现「设为 off」。真机核过净态与脏态两种输出。

## 结论（已定）

**定了：`worktree=on` 的约束靠三层"出声"落地——`do` 的状态感知提示、`commit` 的脏 / 净分派拒绝语、`status` 的 worktree 状态段；而"把改动搬进区"那条路（`--adopt`）不做：提示只给当次出路，绝不邀请改共享配置。**

### 1. 逐条裁定（对 ask 的 6 问）

| # | 问题 | 结论 |
| --- | --- | --- |
| 1 | **提示落在哪些调用点** | **`do`（唯一）+ `commit` 拒绝语**；**读命令不加**（用户判定：噪音不值）→ 落点收缩，也顺带关掉了"单次会话只提示一次"那种本地状态——与"**无隐式状态**"的张力随之消失 |
| 2 | **措辞与脏 / 净** | `do` 按**五态**出声；`commit` 拒绝语按 main **脏 / 净**分派（脏 → 后端配方） |
| 3 | **噪音取舍** | **关闭**：读命令不提示；`do` 只在 `worktree=on` ∧ **不在专用区**时出声 |
| 4 | **没有仓库时** | 说一句「**不在仓库里**（从 <绝对路径> 向上也没有 .git / .jj）」；`do` 只提示、**exit 0 不变**；读命令 / 提交命令保持 exit 1，只把**起因**说准 |
| 5 | **拒绝语按脏 / 净** | **做**（第二交付物）：脏态给配方，且**不再**说"先建区" |
| 6 | **实现落点与代价** | `do` "**尽力而为**"地探测（任何失败降级为提示、**绝不改退出码**）；`status` 为此多一次**注册表读取**（不是 N 次探盘） |

### 2. 三个交付物的最终形态

| # | 交付物 | 形态 |
| --- | --- | --- |
| **A** | **`do` 的提示** | 五态：不在仓库 / `off` / 已在区内（**闭嘴**） / main 净（一句指引） / main 脏（指引 + **后端配方**）。**只进 stderr**，stdout 可原样喂给 `git commit -m` |
| **B** | **`-h` footer** | 从"配置陈述"升格为"**动作指引**"：`worktree 默认: on（动作只在 worktree 里提交——开工前先 kiln worktree new <issue>）| off（允许在 main 提交）` |
| **C** | **`status` 的 worktree 状态段** | 头行尾 `｜worktree on（已开 N 个）`（`off` 与 0 也照显，格式固定）；有区的 issue 行尾 `■`；有区时末行 legend，站在区内时带 `｜当前：worktree <issue>` |

### 3. 一并修掉的既有缺陷

1. **后端探测只看当前目录** → 子目录会误报"没有仓库"（而 `config` 明明向上找得到，自相矛盾）→ 改成**向上找仓库根**，与 `config.Path` 共用同一套锚定；顺带修好"kiln 在子目录里根本不能用"。
2. **`commit` 拒绝语在脏态下是错的**——它让人"先建区"，而新区是**干净检出**，照做会把改动晾在 main → 按脏 / 净分派，脏态给配方。
3. **运行时劝人改配置** ——"把 `worktree` 设为 `off`"这类句子被删：护栏提示不该邀请 agent 去动**共享配置**。

### 4. 三条贯穿始终的不变式

1. **提示只进 stderr、绝不进 stdout；exit code 一律不变**——`do` 的 stdout 必须能**原样**喂给 `git commit -m`（已实测）。
2. **护栏提示只说"这一次怎么走通"，不说"怎么让它永远不拦你"**；`--force` 是**条件句**（"只有你确实要…时才用"），不与配方并列。
3. **配方必须"照着手能跑通"**——两条都真机跑过；jj 那条正因如此才发现"**漏了 `mkdir .kiln/ws` 根本跑不通**"（jj 不建中间目录）。

### 5. 落地物

- **代码**：[internal/vcs/vcs.go](../internal/vcs/vcs.go)（`Detect` 向上找 + `repoRoot` + `ErrNoRepository` + `AreaAt` + `Workspace.Dirty()`）；[internal/cli/cli.go](../internal/cli/cli.go)（`worktreeTip` / `carryRecipe` / `refuseInMain` 分派 / `status` 的段与记号 / `-h` footer / `env.kind`）；两个后端各补 `Dirty()`。
- **测试**：`internal/vcs` 3 例（向上找、不在仓库的措辞、`AreaAt` 四种情形）；`internal/cli` 6 例（`status` 三例、`do` 四态、拒绝语两态，含两条**反向断言**）。`gofmt -l` 空、`go vet` / `go test ./...` 全绿。
- **文档**：本文件；[docs/mvp-plan.md](mvp-plan.md) §10 加条目；`README.md`「工作区模式」补两句。

### 6. 未做（留作后续，要则另开 issue）

- **`--status` / `--no-status` 开关**（候选 Y / Z）——X 已定，不做。
- **与其它视图联动**（`list` / `graph` 也报 worktree 状态）。
- **git hook 兜底**（`mvp-plan` §9 的第三条）——能兜住绕过 kiln 的提交，但要往用户仓库塞文件，仍是候选。
- **净态那句 `--force` 是否也再弱化**——本轮按用户判断只处理了脏态与配置劝告。
