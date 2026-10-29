# kiln

中文 ｜ [English](README.en.md)

**git trailer 驱动的问题追踪与研究路径重建。**

一句话：**kiln 是过滤器，不是注册器。** 它不建数据库、不建索引、不替你建记录文件，只做两件事：

- **写**：把一次探究动作（提问 / 假设 / 采集 / 验证 / 结论 / 落地）编码成一段**带语义 trailer 的规范提交信息**；
- **读**：从 Git 历史里按约定**过滤**出这些动作，重建某个 issue 的**研究路径**、关系网与完整性。

所有状态都住在**提交信息**里——`git clone` 到哪，研究历史就跟到哪；没有服务、没有存储、没有隐式状态。底层是你（和 agent）最熟的 `git`或者 `jj`，kiln 只在顶层补上结构化语义。

## 设计：两个原子 + 涌现

> **本节是摘要。** 权威版本（**当前快照，会随结论性改动被改写，不是契约**）见 [docs/design.md](docs/design.md)——含两个原子、七条张力裁定与 5 条判据。

kiln 把两样东西都拆成**原子**，再让复杂性从**组合**里长出来——而不是预先设计一套流程去套：

| 拆什么 | 怎么拆 | 靠什么组合 |
| --- | --- | --- |
| **原子 issue** | 一个复杂问题**不**在内部再分小线索，而是拆成**多条独立 issue**（各自 `ask`、各自 `domain`、各自过 `check`） | 类型化**关系**：`blocks` / `part-of` / `relates-to` / `caused-by` / `supersedes`（有向的各有反名） |
| **原子 action** | 一条 issue 的处理流程**没有固定阶段**，拆成**原子动作**（提问 / 假设 / 采集 / 验证 / 结论 / 落地），每个动作就是一次提交 | 自由**组合**：任意次序、可重复、可按需自定义词表 |

于是**复杂性是涌现出来的，不是设计出来的**：

- 需要多细的粒度，就写多少个动作——同一个 `test` 出现六次是正常的；
- 需要多复杂的分解，就挂多少条关系——母问题不用"变大"，它只是**多了几条边**；
- 新流程不需要新功能：**新词**（`.kilnconfig.jsonc` 里的 action 词表）就够了。

两条拆法的边界恰好互补：**issue 之间靠关系组合，issue 之内靠动作组合**。而 action 是**过程里的步子**，不是需要被引用的实体——所以早年那个给单个产物挂标签的 `object:` 退役了。

> 理念出处：**Unix 哲学**（小工具、单一职责、用组合表达复杂），以及更根本的一条信念——**复杂性与优秀产品是涌现出来的，不是设计出来的**。详见 [docs/object-semantics.md](docs/object-semantics.md)（issue 原子化）与 [docs/evidence-fluidity.md](docs/evidence-fluidity.md)（三条理念原话）。

## 安装

三种方式任选其一。

**1. `go install`（需要 Go 1.27+，见 [go.mod](go.mod)）**

```sh
go install github.com/jackiehank/kiln/cmd/kiln@latest
```

二进制装到 `$GOBIN`（默认 `$(go env GOPATH)/bin`，Windows 上是 `%USERPROFILE%\go\bin`）；确保它在 `PATH` 里。也可以用 `@v0.3.0` 之类的版本写法固定版本。

**2. 自己构建（单文件、零外部依赖）**

```sh
git clone https://github.com/jackiehank/kiln.git
cd kiln
go build -o kiln ./cmd/kiln
```

**3. 下载预编译二进制**

从 [Releases](https://github.com/jackiehank/kiln/releases) 下载对应平台的产物，放进 `PATH` 即可（Windows 是 `kiln.exe`）。每个 `v*` tag 由 GitHub Actions 构建五个产物：Windows amd64、macOS arm64/amd64、Linux amd64/arm64，并附 `checksums.txt`；Termux 等 arm64 Linux 环境取 `kiln_linux_arm64`。tag 需是 `v0.x` / `v1.x`——模块路径没有 `/v2`，更高的主版本会被 Go 的版本戳忽略。

单文件 CLI，`go.mod` 里没有 `require`。`kiln --version` 可确认装的是哪个构建（release 产物显示 tag 名，如 `v0.3.0`）。

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

> `kiln commit` 自己处理暂存：默认收**整个工作区的改动**（快照，git 侧由 kiln 全量 `add`）；只想提交列出的路径时用 `--only <paths>`（两后端同义：git 只 add 这些路径，jj 用原生 fileset）。提交前 stdout 会打一行 `将提交 N 个文件: …`；空动作允许，只给一条 stderr 提示。

## 命令一览

| 命令 | 作用 |
| --- | --- |
| `kiln do <issue> <action> --title <文本> [flags]` | 生成规范提交信息（**只打印，不提交**）；`--title` 必填（trim 后非空）——它就是 git 的 subject |
| `kiln commit <issue> <action> --title <文本> [--only <paths>] [flags]` | 用规范信息直接提交；`--title` 必填，其余 flag 选填；默认提交整个工作区（快照），`--only` 只提交列出的路径；stdout 先报 `将提交 N 个文件: …`，末尾报 `已提交 <hash> · <action> · <issue>` |
| `kiln path <issue>` | 重建该 issue 的研究路径；`--stat` 逐动作附文件统计、`--shortstat` 每动作一行总计；`--json` 出单对象，`--all --json`（或给多条 issue）一次进程读全仓 |
| `kiln show <issue> [n\|action]` | 展开某个动作的完整变更（默认最后一个；`n` 是 `path` 里的序号，**也可给动作名**——取该动作**最后一次**，有多个会在 stderr 提示，没有则列出可控序号）；`--stat` 只给文件统计 |
| `kiln status [--json]` | 全库概览：按 domain 分组的目录视图 |
| `kiln check [--verbose] [--json]` | 语义完整性校验 |
| `kiln list [--domain=] [--tag=] [--action=] [--status=open\|done] [--search=] [--json]` | 按条件过滤 issue；`--action` 必须命中动作词表；`--tag` 只接受单个标签（不切逗号）；`--status=open` 只留未收尾的（末条动作不是终点）；`--search` 空白分词、词间 AND、字面量、大小写不敏感（搜 id / 标题 / 动作 subject），结果按 rank 0–4 排序（id 首词前缀 / id 全词 / 标题全词 / 单条动作全词 / 词分散），同档按末条动作日期降序、再按 id；`--json` 的搜索行带 `rank`，`hit` 带 `total`/`date`；空结果会在 stderr 说明是哪些条件没匹配 |
| `kiln action list [--json]` | 列出当前生效的 action 类型（词表）及起点/终点；`--json` 输出 `actions` / `source` / `config`，其中 `config` 是当前 OS 原生绝对路径；不读历史、不需要仓库、不创建配置文件 |
| `kiln action log [--jsonl] [--limit N]` | 全局动作流：只留带 `issue:` + `action:` 的提交，一条一行（本地日期 / issue / action / 7 位 hash / 标题）；`--jsonl` 逐行一个 JSON 对象（NDJSON），带**全部 trailer**（`refs` / `target` / `scope` / `relations` / `unrelate`）——关系是**这条动作的声明**，不是折叠后的边（要当前图用 `graph --json`），不含改动文件（用 `show --stat` / `path --all --stat`）；`--limit N` 只看最近 N 条，取够即停；空仓 exit 0，消费者提前关管道静默 exit 0。顺序与首行按后端：**git** 按 author-date 倒序、首行随第一批提交即出；**jj** 用 jj 自己的拓扑序（线性历史下即 newest-first）、等 jj 扫完才一次到齐，所以 jj 上 `--limit` 省的是读取与输出，不是扫描 |
| `kiln graph [--format=ascii\|mermaid\|json] [--mermaid] [--json] [--issue <id>] [--depth N]` | 关系图（`--mermaid` 出流程图文本；`--json` 出给 agent 读的一份文档，`--issue`/`--depth` 收窄成无向邻域，默认 `ascii`） |
| `kiln worktree new <issue>` | 建一个专用工作区（详见「工作区模式」） |
| `kiln worktree list` | 列出全部工作区及各自状态 |
| `kiln worktree remove <issue> [--force]` | 删除工作区（还有工作的默认拒绝，`--force` 强删） |
| `kiln config path` | 打印配置路径（缺失时写入一份带注释的默认配置） |
| `kiln --version` | 打印版本、可执行文件路径、构建时间与 go 平台（`-V` / `version` 同义） |
| `kiln help` / `kiln -h` / `kiln <cmd> -h` | 帮助（`<cmd> -h` 只列该命令的用法与 flags） |

`do` / `commit` 的 flags：`--title --refs --target --scope --domain --tag`；关系：`--blocks --blocked-by --part-of --has-part --relates-to --caused-by --causes --supersedes --superseded-by --unrelate`（均可重复、逗号分隔；别名 `--derived-from` / `--spawns` 同样接受）。
`--title` **必填**（trim 后非空，`do` 与 `commit` 同判）；**值以 `--` 开头时写 `--flag=值`**——空格形式会被当成缺值而拒绝（`--title --force` 报错，不会把 `--force` 吃掉）。
`--target` 声明的每个路径都必须在**本次提交的文件**里（`--only` 时以 `--only` 为准），否则 `do` / `commit` 直接拒绝（exit 1）——写侧护栏，不是事后审计。
另：`commit --force`（一次性豁免，见「工作区模式」）、`commit --only <paths>`（只提交列出的路径，逗号分隔或重复给）。
`--json`：`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` 都支持（机器可读输出，见「机器可读输出」）；`path` 可加 `--all` 一次读全仓（或直接给多条 issue），仅与 `--json` 同用。

## 动作词表

默认：`ask` | `hypothesize` | `collect` | `test` | `conclude` | `apply`
默认起点：`ask`；默认终点：`conclude`、`apply`。

**每个 `action:` 是一条原子动作**——一次提问、一次采集、一次验证、一条结论，不是流水线上的一个阶段。所以：

- **只有两端是结构**：每个 issue **至少一个起点动作**（默认 `ask`）、**最后一个动作必须是终点动作**（默认 `conclude` / `apply`）。
- **中间完全自由**：任意组合、任意次序、**可以重复**（同一个 `test` 出现六次很正常）。没有"必须先 hypothesize 再 test"这类流水线要求。
- **终点之后还能继续提交**：kiln 没有"关闭"这个状态——**"完成"是"最后一个动作是终点"的函数**（`status` 的 `✓` 与 `check` 的第 2 条规则同源）。所以 `ask → conclude → test` 之后这条 issue 会**回到进行中**，`check` 也会报 `缺少终点动作`：那是**判定**，不是错误；想收尾就再补一个终点动作。
- **`path` 里的 `1.` `2.` `3.` 是动作出现的次序**（按时间），是**索引**，不是阶段号——`1. collect 2. ask 3. conclude` 也合法（起点动作只要求"存在"，不要求排第一）。

`apply` 与 `conclude` 的区别是语义上的：`conclude` = 结论成立，`apply` = 结论**已落地到代码**。

词表**每个仓库可以不同**——它写在配置里，见下节。

## 提交格式（trailer）

**识别规则（唯一的签名）：一个提交是 kiln 动作，当且仅当它同时带 `issue:` 与 `action:`。** 只带其中一个 → 忽略（普通提交里恰好写 `issue: 性能瓶颈` 不会被误收）。

**必需**

| Trailer | 含义 |
| --- | --- |
| `issue:` | issue ID（单段 kebab-case slug，如 `session-expiry`）；**写侧会归一化**（大小写 / `_` / 连续 `-`），对**已有起点动作**的 issue 再 `ask` 会被拒绝（见 [docs/mvp-plan.md](docs/mvp-plan.md) §2.4） |
| `action:` | 动作类型（上表词表里的一员） |

**可选（描述）**：`title:` 人类可读标题 ｜ `refs:` 引用的对象 ｜ `target:` 涉及的文件（**必须在本次改动里**；可逗号写多个） ｜ `scope:` 描述性范围 ｜ `domain:` 领域分类 ｜ `tags:` 标签

**可选（关系）**

| Trailer | 含义 | 方向 |
| --- | --- | --- |
| `blocks:` | 我阻塞谁 | 阻塞者 → 被阻塞者 |
| `blocked-by:` | 谁阻塞我 | 被阻塞者 → 阻塞者 |
| `part-of:` | 我属于谁 | 部分 → 整体 |
| `has-part:` | 谁属于我 | 整体 → 部分 |
| `relates-to:` | 相关 | 无向 |
| `caused-by:` | 谁引发了我 | 结果 → 原因 |
| `causes:` | 我引发了谁 | 原因 → 结果 |
| `supersedes:` | 我取代谁 | 新 → 旧 |
| `superseded-by:` | 谁取代了我 | 旧 → 新 |
| `unrelate:` | 撤销与该 issue 的全部关系（同一条里再声明则声明优先） | — |

别名 `derived-from` / `spawns` 仍可读可写，落在 `caused-by` / `causes` 上。

**标题与 trailer 之间必须留一个空行**，否则 `git log --oneline` 与 PR 标题会把整段 trailer 挤进同一行。

关系键的两端**永远是 issue 名**（没有"指向某条动作"的键）；正名与反名都是可写键——`blocks:` 与 `blocked-by:` 是同一对边的两个视角，方向由名字本身决定。

`kiln do` 会帮你把这一切排成规范格式，平时不必手写：

```sh
kiln do session-expiry test --title "压测复现" --target internal/session/lock.go --blocks cli-necessity
```

## 配置

配置文件是**仓库根**的 `.kilnconfig.jsonc`（JSONC：JSON + `//` 与 `/* */` 注释，**不支持尾逗号**）：

```jsonc
{
  // action 词表：每个项目可以不同——这是 kiln 唯一可配的词表（relation 词表固定 9 名）
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS 后端：auto（.jj 优先，否则 .git）/ git / jj
  "vcs": "auto",
  // worktree 模式：off（默认，允许在 main 提交）| on（动作只在 worktree 提交，开工前先 kiln worktree new <issue>）
  "worktree": "off"
}
```

- **位置**：`kiln config path` 打印路径；文件缺失时**自动播种一份带注释的默认配置**（幂等，不覆盖已有文件）。定位方式是**从当前目录向上找仓库根**，所以在子目录里跑也一样。
- **严格校验**：词表为空 / `start`、`terminal` 不在 `names` 里 / **键拼错** → 启动即报错，不静默忽略。
- **最小配置**：`{"actions": {"names": ["ask", "probe", "done"]}}` → `start=ask`、`terminal=done`。

## 缓存（后台行为，默认开启）

每条读命令都要两样东西：一份"提交列表"，和每条提交解析后的字段。两样都落盘——列表按 HEAD 存成**枚举快照**（`.git/kiln/enums/`，约 20 B/提交），字段存成**记录层**（`.git/kiln/records/`，每克隆一份、所有 worktree 共享）。**热读不启动 git**，输出逐字节不变：快照就是 git 自己的输出，层里存的就是解析后的同一份字段。

- 第一次读顺手写进去（一次性多花几十毫秒），之后命中。HEAD 前进就是一份新快照，旧的自动收敛（写时保留最近 4 份，`kiln cache gc` 清掉不可达的）；层数到 64 自动合并，合并时顺手剪掉不可达记录。
- 单个命令可以 `--no-cache`（或 `KILN_NO_CACHE=1`）绕过，输出不变。
- 只读仓 / CI 挂载：写失败静默跳过，读照常（退化成每次问 git）。
- 仓库里若出现 `refs/replace`、`info/grafts`、`shallow`、reftable，或设了 `GIT_DIR` 一类变量，kiln 不猜——直接按老办法每次问 git。
- 实测（本仓 408 提交）：读命令 130 → 42 ms（−68%）；100k 提交：`graph --json` 1,185 → 246 ms（−79%）、`list` 1,162 → 219（−81%）、`action log --jsonl` 1,381 → 405（−71%）、`path --all --json` 1,308 → 360（−73%）。首次读一次性多花约 100 ms；磁盘约 20 B/提交（快照）+ 约 130 B/提交（层，真实仓）。

设计细节见 [docs/commit-cache.md](docs/commit-cache.md)；"热读不起 git"的取舍与实验数据见 [docs/refactor.md](docs/refactor.md)。

## 工作区模式（隔离）

并行干活的 agent 会互相污染：同一个工作副本里，两个 agent 的未提交改动会被混进同一条提交，动作归属就错了。kiln 的对策是**结构性**的——**main（主干）只读 / 只合并，动作只在专用工作区提交**：

```sh
kiln worktree new cli-necessity      # 建区（<仓库根>/.kiln/ws/cli-necessity）
cd .kiln/ws/cli-necessity            # 进去干活
kiln commit cli-necessity test --title "…" ...   # 在这里提交
```

- **`worktree` 键默认 `off`**：开箱即用，在 main 上直接 `kiln commit` 就是允许的。要隔离就在配置里显式写 `"worktree": "on"`：此时在 main 上直接 `kiln commit` 会被**结构性拒绝**，并提示你先建区；想“就这一次在 main 提交”→ 加 `--force`（**一次性豁免**，不改配置、不留状态）。
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
| 提交动作 | 默认由 kiln 全量 `add` 后提交快照；`--only` 只 add 列出的路径 | 默认固定整个工作副本；`--only` 用原生 fileset |
| 空动作 | 允许（`--allow-empty`） | 允许 |
| 动作历史 | 只算**已提交**的 | 排除工作副本提交（`@`），同样是"只算已提交的" |
| 隔离单位的后端叫法 | `worktree` | `workspace`（kiln 的命令面对两者都叫 `kiln worktree`） |
| 专用区判定 | 当前检出是不是 linked worktree | 当前工作区是不是非 `default` |

**colocated 仓（同时有 `.jj` 与 `.git`）下的注意**：

- jj 的提交会**自动导出进 git 的 refs**，因此 `git log` / `jj log` 两个视图不会分叉。
- jj 用 `.jj/.gitignore`（内容 `/*`）自我忽略，好让 git 别把 `.jj` 报成未跟踪；但 `git clean -xdf` 的语义是"**连同被忽略的一起清**"——**裸跑它会连 `.jj/` 一起抹掉**。要清就用 `git clean -xdf -e .jj -e .kiln`，或只清具体路径。
- 本仓（kiln 自己的仓库）就是 colocated 的，上一条对在本仓干活同样成立。

## 校验：`kiln check`

**每个 issue 必须由起点动作开启、由终点动作收尾**（中间动作不设顺序约束）。目前的规则：

1. 起点：至少一个起点动作（默认 `ask`）——缺失报 `start-action`（✗）
2. 终点：最后一个动作是终点动作（默认 `conclude` / `apply`）——不符报 `terminal-action`（⚠，只警告不阻断）
3. 关系：每条声明的关系目标都必须存在——悬空报 `relation-target`（⚠，带关系名与目标 issue）
4. 未知键：不认识的 trailer 键会被点名——`unknown-trailer`（⚠）
5. 以上都通过：`complete`（✓）

```
$ kiln check
✓ session-expiry   完整（ask, hypothesize, test, conclude）
⚠ deep-dive        缺少终点动作（当前 test）
⚠ broken-ref       关系目标不存在：caused-by 指向 missing-parent
```

约定：**只有 `✗`（错误）让 `check` 返回非零退出码**，`⚠` 只提示——所以"进行中的 issue"不会卡住 CI。

## 机器可读输出（`--json`）

`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` 都能吐机器可读的 JSON（加 `--json`），给 agent 与脚本用。**不加这个 flag 时，输出一字不变。**

```sh
kiln status --json
kiln list --json --domain=design
kiln list --json --search=worktree
kiln check --json
kiln path --all --json
```

```json
{
  "passed": true,
  "findings": { "ok": 21, "warn": 9, "error": 0 },
  "results": [
    { "issue": "action-config", "level": "ok", "rule": "complete", "actions": ["ask", "apply"] }
  ]
}
```

规矩（完整 schema 见 [docs/machine-readable-output.md](docs/machine-readable-output.md)）：

- **顶层永远是一个对象**，不放裸数组；`issues` / `results` 恒为数组——空仓是 `[]`，不是 `null`。
- **`status` 是平铺的一层行**：`id` / `domain` / `action` / `done` / `title`，外加 `has-worktree`。**没有分组、没有嵌套**——文本视图用一行**块头**标出 domain，JSON 里每个 issue 一行、自带 `domain`，所以两边**issue 行的相对顺序一致**（块头行不参与）。
- **`check` 用 rule 码说话**：`start-action` / `terminal-action` / `relation-target` / `unknown-trailer` / `complete`——不吐中文判据；`passed` **与退出码同义**（有 `✗` 才 `false` / 退出 1），`findings` 里的 `ok` / `warn` / `error` 数的是**诊断条数**，不是一个 issue 一个。
- **缺省即省略**（省略 = 没有），**布尔恒显**（`false` 是判定结果，不是"没有"）。
- **stdout 只放一个 JSON 文档**：提示、legend、报错一律在 stderr，退出码与文本视图一致。
- **`path --all --json` 一次进程读全仓**：`{"issues":[<单条 path 文档>…]}`，每项与 `kiln path <id> --json` 逐字节同形。显式给多条 issue 时保持给定顺序，`--all` 按 id 字典序；单条取不到不拖垮整份——该条进 `errors`（`unknown-issue` / `stat-failed`），其余照出，退出码 1。批量只服务 `--json`（文本视图一次只展开一条）。
- **例外：`action log --jsonl` 是 NDJSON**，不是"一份文档"——一行一个动作、一个独立对象，没有信封也没有终止记录；字段为 `issue` / `action` / `commit` / `date` / `title` / `domain` / `tags` / `refs` / `target` / `scope` / `relations` / `unrelate`，为空省键。`relations` 是**这条动作的声明**（`has-part: X` 原样保留，不翻成边），要当前关系图请用 `graph --json` / `path --json`。见 [docs/action-log.md](docs/action-log.md)。

## 与 agent 协作

kiln 的模型天生适合 agent：

- **约定只一条**：做研究动作时用 `kiln commit`（或 `git commit -m "$(kiln do …)"`）。告诉 agent 这一句就够了。
- **先开单再落代码**：`ask` 动作里只放 issue doc，提交后再写实现。按顺序做不需要任何额外能力；确实要反过来做时，用 `kiln commit … --only docs/<issue>.md` 只提交 doc（两后端一致），不必把代码挪开。见 [docs/commit-order.md](docs/commit-order.md)、[docs/commit-scope.md](docs/commit-scope.md)。
- **结构化输出**：`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` 加 `--json` 后都是**真正的**机器可读格式，stdout 只放一个 JSON 文档；`path --all --json` 一次进程给全部研究路径（省掉 N 次启动），`graph` 另有 `--mermaid`（流程图文本），`graph --json --issue <id> --depth N` 只出该 issue 的无向邻域。见「机器可读输出」。
- **诚实**：`check` 会把"缺终点动作""关系目标不存在""不认识的 trailer 键"这类结构问题直接指出来（`--json` 下给的是 rule 码，不是中文）。
- **隔离**：并行 agent 用 `kiln worktree new` 各占一个区，从**结构上**消除互相污染。

## 它不做什么

- 不建文件（`q1.md` 这类记录由人或 agent 自己建）；**不区分变更类型**——变更展示委托 git（`--stat` / `show`），写侧只为 `--target` 看一眼本次的文件清单；
- 不建中央注册表 / 索引 / 数据库——状态全在提交信息里；
- 不做模式切换（没有"纯变更模式 / 记录模式"之分，一切靠 trailer 约定）；
- 不编排你的工作流（不跑构建、不装依赖、不钉 cwd）；
- 不改写历史——动作就是提交，撤销 / 回滚 / 跨分支复用全用 git 自己的本事。

## 已知边界

- `kiln graph` 按「声明源箭头」画：每条关系只出现在**声明方**的块里（不换成反名、目标块不重复），对称关系（`relates-to`）不画箭头，有向关系是 `──name──▶`；同一 `(无序对, 槽)` 被两侧声明时**后写者胜**，全图只出一形——见 [docs/graph-normalize.md](docs/graph-normalize.md)。
- `worktree remove` 清不掉"目录已删"的区（那种区得先 `git worktree prune`）。
- 新区的未提交改动**不会**被搬过去——**刻意为之，不是待办**（kiln 不代管本地状态，与 `.worktreeinclude` 只管被忽略文件是同一条边界，见 [docs/worktree-adopt.md](docs/worktree-adopt.md)）。要带过去得手工：git 用 `stash -u` → 建区 → 区内 `pop`；jj 用 `workspace add -r @` + 主干 `jj edit @-`（**别**用 `jj restore`，那会级联 rebase 把新区连坐）。
- **机器可读输出覆盖七个命令**：`status` / `list` / `check` / `graph` / `path` / `show` / `worktree list` 都有 `--json`；`--toon` 尚未评估——见 [docs/machine-readable-output.md](docs/machine-readable-output.md)、[docs/read-json.md](docs/read-json.md)。

## 延伸阅读

- [docs/design.md](docs/design.md) —— **理念**：两个原子、七条张力裁定、5 条判据；**当前快照，不是契约**。
- [docs/mvp-plan.md](docs/mvp-plan.md) —— **规格**：核心约定、trailer 规范、命令面、`check` 规则、设计原则。改 kiln 之前先读它。
- [docs/](docs/) —— 每个设计决定一份记录（`worktree-mode.md` / `worktree-include.md` / `config-home.md` …）：为什么长成这样、当时试过什么、放弃了什么。
- 模块地图：`cmd/kiln`（入口）｜`internal/model`（领域类型）｜`internal/config`（配置）｜`internal/trailer`（提交信息解析）｜`internal/vcs`（后端接口 + `git` / `jj` 两个适配器 + 播种）｜`internal/engine`（聚合：path / check / status / list / graph）｜`internal/cli`（子命令与渲染）。分层约束：`cli → engine → (vcs, model)`，引擎不直接 exec。

## 许可证

MIT，见 [LICENSE](LICENSE)。
