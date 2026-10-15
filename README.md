# kiln

**git trailer 驱动的问题追踪与研究路径重建。**

一句话：**kiln 是过滤器，不是注册器。** 它不建数据库、不建索引、不替你建记录文件，只做两件事：

- **写**：把一次探究动作（提问 / 假设 / 采集 / 验证 / 结论 / 落地）编码成一段**带语义 trailer 的规范提交信息**；
- **读**：从 Git 历史里按约定**过滤**出这些动作，重建某个 issue 的**研究路径**、关系网与完整性。

所有状态都住在**提交信息**里——`git clone` 到哪，研究历史就跟到哪；没有服务、没有存储、没有隐式状态。底层是你（和 agent）最熟的 `git`或者 `jj`，kiln 只在顶层补上结构化语义。

## 安装

```sh
go build -o kiln ./cmd/kiln        # 需要 Go 1.27+（见 go.mod）
```

单文件 CLI，**零外部依赖**（`go.mod` 里没有 `require`）。Windows 下产物是 `kiln.exe`，放进 `PATH` 即可。

## 五分钟上手

```sh
# 1) 开题：ask 是起点动作
kiln commit session-expiry ask --title "X 并发可能出错" --domain auth

# 2) 中间动作：顺序不是约束，用到哪个写哪个
kiln commit session-expiry hypothesize --title "假设锁竞争导致" --refs q1
kiln commit session-expiry collect     --title "查了两家的实现" --refs "doc-a, doc-b"
kiln commit session-expiry test        --title "压测复现" --target internal/session/lock.go

# 3) 收尾：终点动作（conclude / apply）
kiln commit session-expiry conclude --title "结论：调锁粒度"

# 4) 看这条研究路径
kiln path session-expiry
```

```
$ kiln path session-expiry
session-expiry  已完成（当前 conclude）
  1. ask          a1b2c3d  发现 X 并发可能出错
  2. hypothesize  e4f5g6h  假设锁竞争导致
  3. collect      k1l2m3n  查了两家的实现
  4. test         i7j8k9l  压测复现
  5. conclude     o4p5q6r  结论：调锁粒度
```

**只想拿提交信息、不想让 kiln 提交？** 用 `kiln do` —— 它**只打印，不写文件**：

```sh
git commit -m "$(kiln do session-expiry test --title '压测复现')"
# 或（jj 后端）
jj commit -m "$(kiln do session-expiry test --title '压测复现')"
```

> 提交前要自己暂存改动：**git 后端需要 `git add`**（kiln 不碰暂存区）；**jj 后端不需要**——它没有暂存区，`kiln commit` 直接固定**整个工作副本**。

## 命令一览

| 命令 | 作用 |
| --- | --- |
| `kiln do <issue> <action> [flags]` | 生成规范提交信息（**只打印，不提交**） |
| `kiln commit <issue> <action> [flags]` | 用规范信息直接提交 |
| `kiln path <issue>` | 重建该 issue 的研究路径；`--stat` 逐动作附文件统计、`--shortstat` 每动作一行总计 |
| `kiln show <issue> [n]` | 展开某个动作的完整变更（默认最后一个；`n` 即 `path` 里的序号）；`--stat` 只给文件统计 |
| `kiln status [--json]` | 全库概览：按 domain 分组的目录视图 |
| `kiln check [--verbose] [--json]` | 语义完整性校验 |
| `kiln list [--domain=] [--tag=] [--action=] [--json]` | 按条件过滤 issue |
| `kiln graph [--format=ascii]` | 关系图（当前仅 `ascii`） |
| `kiln worktree new <issue>` | 建一个专用工作区（详见「工作区模式」） |
| `kiln worktree list` | 列出全部工作区及各自状态 |
| `kiln worktree remove <issue> [--force]` | 删除工作区（还有工作的默认拒绝，`--force` 强删） |
| `kiln config path` | 打印配置路径（缺失时写入一份带注释的默认配置） |
| `kiln help` / `kiln -h` | 帮助 |

`do` / `commit` 的 flags：`--title --refs --target --scope --domain --tag --derived-from --spawns --relates-to --blocks --supersedes`
另：`commit --force`（一次性豁免，见「工作区模式」）。
`status` / `list` / `check` 的 flags：`--json`（机器可读输出，见「机器可读输出」）。

## 动作词表

默认：`ask` | `hypothesize` | `collect` | `test` | `conclude` | `apply`
默认起点：`ask`；默认终点：`conclude`、`apply`。

**顺序不是约束**：`ask` 只负责**开启**一个 issue，其后是一个**无序集合**——任意组合、任意次序都合法，没有"必须先 hypothesize 再 test"这类流水线要求。唯一的结构纪律是：**每个 issue 至少一个起点动作，且以终点动作收尾**（`kiln check` 就查这个）。

`apply` 与 `conclude` 的区别是语义上的：`conclude` = 结论成立，`apply` = 结论**已落地到代码**。

词表**每个仓库可以不同**——它写在配置里，见下节。

## 提交格式（trailer）

**识别规则（唯一的签名）：一个提交是 kiln 动作，当且仅当它同时带 `issue:` 与 `action:`。** 只带其中一个 → 忽略（普通提交里恰好写 `issue: 性能瓶颈` 不会被误收）。

**必需**

| Trailer | 含义 |
| --- | --- |
| `issue:` | issue ID（单段 kebab-case slug，如 `session-expiry`） |
| `action:` | 动作类型（上表词表里的一员） |

**可选（描述）**：`title:` 人类可读标题 ｜ `refs:` 引用的对象 ｜ `target:` 涉及的文件 ｜ `scope:` 描述性范围 ｜ `domain:` 领域分类 ｜ `tags:` 标签

**可选（关系）**

| Trailer | 含义 | 方向 |
| --- | --- | --- |
| `derived-from:` | 这个 issue 从哪来 | 子 → 父 |
| `spawns:` | 这个 issue 引出了什么 | 父 → 子 |
| `relates-to:` | 相关但不派生 | 双向 |
| `blocks:` | 阻塞了什么 | 阻塞者 → 被阻塞者 |
| `supersedes:` | 取代了什么 | 新 → 旧 |

**标题与 trailer 之间必须留一个空行**，否则 `git log --oneline` 与 PR 标题会把整段 trailer 挤进同一行。

关系键的两端**永远是 issue 名**（没有"指向某条动作"的键）；反向名不存在，方向由渲染时的箭头/措辞体现。

`kiln do` 会帮你把这一切排成规范格式，平时不必手写：

```sh
kiln do session-expiry test --title "压测复现" --target internal/session/lock.go --blocks cli-necessity
```

## 配置

配置文件是**仓库根**的 `.kilnconfig.jsonc`（JSONC：JSON + `//` 与 `/* */` 注释，**不支持尾逗号**）：

```jsonc
{
  // action 词表：每个项目可以不同——这是 kiln 的自定义点
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS 后端：auto（.jj 优先，否则 .git）/ git / jj
  "vcs": "auto",
  // worktree 模式：on（默认，动作只在 worktree 提交）| off（允许在 main 提交）
  "worktree": "on"
}
```

- **位置**：`kiln config path` 打印路径；文件缺失时**自动播种一份带注释的默认配置**（幂等，不覆盖已有文件）。定位方式是**从当前目录向上找仓库根**，所以在子目录里跑也一样。
- **严格校验**：词表为空 / `start`、`terminal` 不在 `names` 里 / **键拼错** → 启动即报错，不静默忽略。
- **最小配置**：`{"actions": {"names": ["ask", "probe", "done"]}}` → `start=ask`、`terminal=done`。

## 工作区模式（隔离）

并行干活的 agent 会互相污染：同一个工作副本里，两个 agent 的未提交改动会被混进同一条提交，动作归属就错了。kiln 的对策是**结构性**的——**main（主干）只读 / 只合并，动作只在专用工作区提交**：

```sh
kiln worktree new cli-necessity      # 建区（<仓库根>/.kiln/ws/cli-necessity）
cd .kiln/ws/cli-necessity            # 进去干活
kiln commit cli-necessity test ...   # 在这里提交
```

- **`worktree` 键默认 `on`**：此时在 main 上直接 `kiln commit` 会被**结构性拒绝**，并提示你先建区。想"就这一次在 main 提交"→ 加 `--force`（**一次性豁免**，不改配置、不留状态）；想长期允许 → 把配置里的 `worktree` 设为 `off`。
- **动手前就会被提醒**：`kiln do` 会先看你站在哪——不在仓库、已关隔离（`off`）、**已经站在区内**，它都闭嘴；在 main 就提醒你"先建区"，而 **main 还有未提交改动时，它直接给出把改动搬进区的现成命令**（git 与 jj 各有一条）。提示只走 stderr，**不会弄脏提交信息**，退出码也不变。
- **看状态**：`kiln status` 头行报 `｜worktree on（已开 N 个）`；有区的 issue 行尾带 `■`；有区时末行 legend 说明记号，并告诉你**当前站在哪个 worktree**。
- **被拦下时它给的是出路不是墙**：拒绝语按 main **脏 / 净**分派——净了就只说"先建区"；脏了会先提醒"新区是干净检出，直接建区会把改动留在 main"，再给后端配方。
- **`kiln worktree list`** 列出全部工作区，并给每个区一个**状态**：

  ```
  <repo>              main（只读/只合并）  —
  <repo>/.kiln/ws/t1  worktree t1          干净
  <repo>/.kiln/ws/t2  worktree t2          有改动，未合并
  ```

  状态有四档：`干净` / `有改动`（未提交改动） / `未合并`（它的提交还没进 main） / `目录已删`（目录没了但还登记着）。**main 一行报 `—`**——它不是专用区，没有可判的工作。这份状态与 `worktree remove` 的拒绝判据**同源**：`list` 报什么，`remove` 就按什么拒。
- **`kiln worktree remove <issue>`** 删除区**连同它的分支**；区里还有工作（脏 / 未合并）时**默认拒绝**，`--force` 才丢。
- **区是一次性检出**：新区只含**受版本控制**的文件——被 gitignore 的构建产物与依赖**不在其中**，进去之前要自行 build / 装依赖。
- **播种**：如果仓库根有一份 **`.worktreeinclude`**（一行一个模式，`.gitignore` 语法，`#` 注释），建区时会把**"匹配且被忽略"的本地小文件**（如 `.env`、本地覆盖配置）带进新区。已存在的跳过、只进不回、不搬 `.git`/`.jj`/`.kiln` 元数据。依赖与构建产物**不归它管**（那是"重建"，不是"复制"）。

## 后端：git 与 jj

kiln 有可插拔的后端层，**git 与 jj 都能用**。`vcs: auto`（默认）时：**`.jj` 优先于 `.git`**——colocated 仓里两者都在，读 git 会漏掉 jj 的视角。

| | git | jj |
| --- | --- | --- |
| 提交动作 | 需要你先 `git add`（kiln 不碰暂存区） | **不需要**——`kiln commit` 固定整个工作副本 |
| 动作历史 | 只算**已提交**的 | 排除工作副本提交（`@`），同样是"只算已提交的" |
| 隔离单位的后端叫法 | `worktree` | `workspace`（kiln 的命令面对两者都叫 `kiln worktree`） |
| 专用区判定 | 当前检出是不是 linked worktree | 当前工作区是不是非 `default` |

**colocated 仓（同时有 `.jj` 与 `.git`）下的注意**：

- jj 的提交会**自动导出进 git 的 refs**，因此 `git log` / `jj log` 两个视图不会分叉。
- jj 用 `.jj/.gitignore`（内容 `/*`）自我忽略，好让 git 别把 `.jj` 报成未跟踪；但 `git clean -xdf` 的语义是"**连同被忽略的一起清**"——**裸跑它会连 `.jj/` 一起抹掉**。要清就用 `git clean -xdf -e .jj -e .kiln`，或只清具体路径。
- 本仓（kiln 自己的仓库）就是 colocated 的，上一条对在本仓干活同样成立。

## 校验：`kiln check`

**每个 issue 必须由起点动作开启、由终点动作收尾**（中间动作不设顺序约束）。目前的规则：

1. 存在性：至少一个起点动作（默认 `ask`）
2. 终点：最后一个动作是终点动作（默认 `conclude` / `apply`）——**只警告，不阻断**
3. 父存在：声明 `derived-from` 时父 issue 必须存在
4. 子存在：声明 `spawns` 时子 issue 必须存在

```
$ kiln check
✓ session-expiry   完整（ask, hypothesize, test, conclude）
⚠ deep-dive        缺少终点动作（当前 test）
✗ broken-ref       父 issue 不存在：derived-from 指向 missing-parent
```

约定：**只有 `✗`（错误）让 `check` 返回非零退出码**，`⚠` 只提示——所以"进行中的 issue"不会卡住 CI。

## 机器可读输出（`--json`）

`status` / `list` / `check` 可以吐机器可读的 JSON（加 `--json`），给 agent 与脚本用。**不加这个 flag 时，输出一字不变。**

```sh
kiln status --json
kiln list --json --domain=design
kiln check --json
```

```json
{
  "ok": true,
  "summary": { "ok": 21, "warn": 9, "error": 0 },
  "results": [
    { "issue": "action-config", "level": "ok", "rule": "complete", "actions": ["ask", "apply"] }
  ]
}
```

规矩（完整 schema 见 [docs/machine-readable-output.md](docs/machine-readable-output.md)）：

- **顶层永远是一个对象**，不放裸数组；`issues` / `results` 恒为数组——空仓是 `[]`，不是 `null`。
- **`status` 是平铺的一层行**：`id` / `domain` / `action` / `done`，外加 `has-worktree`。**没有分组、没有嵌套**——文本视图用一行**块头**标出 domain，JSON 里每个 issue 一行、自带 `domain`，所以两边**issue 行的相对顺序一致**（块头行不参与）。
- **`check` 用 rule 码说话**：`start-action` / `terminal-action` / `derived-from` / `spawns` / `complete`——不吐中文判据；`ok` **与退出码同义**（有 `✗` 才 `false` / 退出 1）。
- **缺省即省略**（省略 = 没有），**布尔恒显**（`false` 是判定结果，不是"没有"）。
- **stdout 只放一个 JSON 文档**：提示、legend、报错一律在 stderr，退出码与文本视图一致。

## 与 agent 协作

kiln 的模型天生适合 agent：

- **约定只一条**：做研究动作时用 `kiln commit`（或 `git commit -m "$(kiln do …)"`）。告诉 agent 这一句就够了。
- **结构化输出**：`status` / `list` / `check` 加 `--json` 后有**真正的**机器可读格式；其余命令（`path` / `show` / `graph` / `worktree list`）仍是稳定文本——"稳定"靠的是约定，不等于机器可读，见「机器可读输出」。
- **诚实**：`check` 会把"缺终点动作""父 issue 不存在"这类结构问题直接指出来（`--json` 下给的是 rule 码，不是中文）。
- **隔离**：并行 agent 用 `kiln worktree new` 各占一个区，从**结构上**消除互相污染。

## 它不做什么

- 不建文件（`q1.md` 这类记录由人或 agent 自己建）；不看 diff、不区分变更类型；
- 不建中央注册表 / 索引 / 数据库——状态全在提交信息里；
- 不做模式切换（没有"纯变更模式 / 记录模式"之分，一切靠 trailer 约定）；
- 不编排你的工作流（不跑构建、不装依赖、不钉 cwd）；
- 不改写历史——动作就是提交，撤销 / 回滚 / 跨分支复用全用 git 自己的本事。

## 已知边界

- `kiln graph` 目前只画 `spawns` / `relates-to` / `derived-from`，**`blocks` 与 `supersedes` 还没渲染**（`path` 里是有的）——见 [docs/relation-rendering.md](docs/relation-rendering.md)。
- 关系词表规格里还留了 `part-of`（分解），**尚未落地**。
- `worktree remove` 清不掉"目录已删"的区（那种区得先 `git worktree prune`）。
- 新区的未提交改动**不会**被搬过去——**刻意为之，不是待办**（kiln 不代管本地状态，与 `.worktreeinclude` 只管被忽略文件是同一条边界，见 [docs/worktree-adopt.md](docs/worktree-adopt.md)）。要带过去得手工：git 用 `stash -u` → 建区 → 区内 `pop`；jj 用 `workspace add -r @` + 主干 `jj edit @-`（**别**用 `jj restore`，那会级联 rebase 把新区连坐）。
- **机器可读输出只落在三个命令**：`status` / `list` / `check` 有 `--json`，`path` / `show` / `worktree list` 还没有，`--toon` 也尚未评估——见 [docs/machine-readable-output.md](docs/machine-readable-output.md)。

## 延伸阅读

- [docs/mvp-plan.md](docs/mvp-plan.md) —— **规格**：核心约定、trailer 规范、命令面、`check` 规则、设计原则。改 kiln 之前先读它。
- [docs/](docs/) —— 每个设计决定一份记录（`worktree-mode.md` / `worktree-include.md` / `config-home.md` …）：为什么长成这样、当时试过什么、放弃了什么。
- 模块地图：`cmd/kiln`（入口）｜`internal/model`（领域类型）｜`internal/config`（配置）｜`internal/trailer`（提交信息解析）｜`internal/vcs`（后端接口 + `git` / `jj` 两个适配器 + 播种）｜`internal/engine`（聚合：path / check / status / list / graph）｜`internal/cli`（子命令与渲染）。分层约束：`cli → engine → (vcs, model)`，引擎不直接 exec。
