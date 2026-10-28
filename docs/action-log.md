# issue: action-log — `kiln action log`：全局动作流

- **domain**：cli
- **关联**：relates-to `action-view`（命名与形态已 conclude）；relates-to `action-list`（同组的词表命令）；relates-to `timeline-stream`（视图必要性、test 与验收分档）；relates-to `machine-readable-output`（NDJSON 例外与字段）

## 背景

`action-view` 已 conclude：`action` 组下，`action list` 列词表、`action log` 列动作。`action log` 是全局 action 视图：只留 `issue:` + `action:` 的提交，按 author-date 倒序，默认文本、`--jsonl` 给机器、`--limit` 首屏止损。

本 issue 只做 `action log`，不碰 `action list`。

## 要做的

```text
kiln action log [--jsonl] [--limit N]
```

- 全局动作流；不需要 `--all`；
- 默认文本：一行一条，含日期 / issue / action / 短 hash / subject；
- `--jsonl`：NDJSON，一行一个对象，字段 `issue / action / commit / date / title / domain / tags`；
- `--limit N`：最多 N 条动作后正常退出；N ≥ 1；
- git 流式；jj 显式非 0「暂不支持」，不静默空输出；
- 空仓/无动作 exit 0；broken pipe 及时退出。

## 要回答的问题

1. 流式后端接口：`vcs` 加可选的 `LogEach` / `LogStream`，还是别的形状？怎么复用 `StatLog` 的“可选接口”先例？
2. 文本布局与日期格式；是否要表头/legend；空结果是否给 stderr 提示。
3. `--jsonl` 的字段细节：`title` 取 subject 还是 trailer title；`domain` 是 per-commit 还是 issue 级；`tags` 是否带。
4. `--limit` 与 broken pipe 的退出码；Windows/Unix 差异怎么测。
5. jj 的行为与文案；未来要支持时的接口预留。
6. NDJSON 如何写进“一份 JSON 文档”规则；commit-graph 分档验收怎么写。
7. 与 `path --all --json`、`git log`、`list --search` 的边界与文档。

## 非目标

- 不做 `action list`；
- 不做 `path --timeline` / `path --all --jsonl`；
- 不做 `--all`、`--json`、`--stat`、过滤；
- 不做 `action show` / `action search`；
- 不做文本富 timeline（高亮/分组/交互）。

## collect（2026-10-28）：实现 `action log` 需要的现状与缺口

> 只收事实与缺口，不选形状；留给 hypothesize。实测日期 2026-10-28，git 2.55.0.windows.3。

### 1. 后端读历史只有一个全量接口

| 事实 | 位置 |
| --- | --- |
| `vcs.Commit` 只有 `ID / Date / Message` 三个字段，`Log()` 文档写的是 “every commit, newest first” | [vcs.go:27-31](../internal/vcs/vcs.go#L27-L31)、[vcs.go:53-56](../internal/vcs/vcs.go#L53-L56) |
| git 实现是一次 `git log --format="%H\x1f%aI\x1f%B\x1e"`，`cmd.Run()` 把全部输出收进 `bytes.Buffer` | [git.go:27-36](../internal/vcs/git/git.go#L27-L36)、[git.go:447-464](../internal/vcs/git/git.go#L447-L464) |
| 解析是 `\x1e` 切记录、`\x1f` 切字段；日期用 `time.Parse(time.RFC3339)` | [git.go:466-487](../internal/vcs/git/git.go#L466-L487) |
| jj 实现是 `jj log --no-graph -r "all() ~ root() ~ @ "` + 自定义模板 | [jj.go:30-42](../internal/vcs/jj/jj.go#L30-L42) |

即：**今天没有任何逐条读取的通道**。`Log()` 返回时，100k 条 message 已经在内存里；`path --all` 的 954ms 首块、35.6MB 全量就是这条路径的量级（[timeline-stream.md](timeline-stream.md) 的 test）。

### 2. 可选接口的既有先例

- `vcs.StatLog`：后端能批量出 diffstat 就实现，调用方类型断言，**不实现就回退**（[vcs.go:81-92](../internal/vcs/vcs.go#L81-L92)、[cli.go:1847-1854](../internal/cli/cli.go#L1847-L1854)）。
- 同一种写法还有 `CheckedAction`（[vcs.go:102-109](../internal/vcs/vcs.go#L102-L109)）与 `AreaHint`（[vcs.go:94-100](../internal/vcs/vcs.go#L94-L100)）。
- 先例的共同点：**可选接口 + 调用方回退**，语义与主接口等价。`action log` 想借这个形状，但 jj 这里没有等价回退（回退成 `Log()` 就是“先全量再排序”，与本 issue 的流式承诺矛盾，且和“jj 显式非 0”冲突）——这是与 `StatLog` 最大的不同，必须在 hypothesize 里裁决。

### 3. 流式输出的唯一先例：`pathBatchWriter`，但它不是逐条 flush

- [pathBatchWriter](../internal/cli/cli.go#L1747-L1841)：64KB `bufio`、与 `writeJSON` 逐字节一致、每次写都记 `w.err`，调用方 `w.err != nil` 时 break（[cli.go:1727-1731](../internal/cli/cli.go#L1727-L1731)）。
- **它只解决“文档流式”**：`finish()` 才 `Flush()`（[cli.go:1840](../internal/cli/cli.go#L1840)），而且第一字节之前 `engine.Build` 已经读完整个历史（[cli.go:2892-2897](../internal/cli/cli.go#L2892-L2897)）。
- 对 `action log` 直接套用会坏事：64KB 缓冲意味着首行要等缓冲满或进程结束，`| head -1` 反而更慢。逐条流式需要**每记录 flush**（或极小缓冲），这是新写法。

### 4. 什么算一条动作：唯一的准入规则

- `trailer.Parse`：逐行扫，key 小写，值 trim，`action` 转小写（[trailer.go:142](../internal/trailer/trailer.go#L142)），**`issue` 与 `action` 都非空才算动作**（[trailer.go:143](../internal/trailer/trailer.go#L143)）。
- `engine.ActionsFromCommits` 用的就是这一条，没有第二条规则（[engine.go:19-46](../internal/engine/engine.go#L19-L46)）。
- **读路径从不用当前词表过滤**：`IsValid` 只出现在输入校验（`do`/`commit` [cli.go:795](../internal/cli/cli.go#L795)、`list --action` [cli.go:2520](../internal/cli/cli.go#L2520)、`show` 选择器 [cli.go:1545](../internal/cli/cli.go#L1545)）；`check` 也没有“action 名不在词表里”这条规则（[engine.go:231-268](../internal/engine/engine.go#L231-L268)）。
- 所以：`action log` 若**不**按词表过滤，就与 `path`/`list`/`show` 对“动作”的定义完全一致；若过滤，就会与它们不一致，并把历史藏起来。本仓 350 个提交里 335 个是动作，15 个不是（2026-10-28）。

### 5. 字段从哪来（都不是新数据）

| 字段 | 现成来源 | 已有先例 |
| --- | --- | --- |
| `issue` | `Fields.Issue`，**原样**（不 normalize）；`path`/`list` 也用它当 issue 键（[model.go:134-136](../internal/model/model.go#L134-L136)） | `do` 写前会 normalize（[cli.go:793](../internal/cli/cli.go#L793)），所以新提交规范、老提交可能不规范 |
| `action` | `Fields.Action`，已小写 | 同上 |
| `commit` | `Commit.ID` 全长 hash | `pathAction.commit`（[cli.go:1637](../internal/cli/cli.go#L1637)）、`show --json` |
| `date` | `Commit.Date`（来自 `%aI`） | JSON 一律 `Format(time.RFC3339)`（[cli.go:1893](../internal/cli/cli.go#L1893)、[cli.go:1983](../internal/cli/cli.go#L1983)）；文本视图今天只有 `--version` 打时间，用的是本地 `2006-01-02 15:04:05`（[cli.go:558](../internal/cli/cli.go#L558)） |
| `title` | 两套：trailer `title:`（`pathAction.title`，`omitempty`，[cli.go:1639](../internal/cli/cli.go#L1639)）或 “trailer title → subject 兜底”（`issueTitle`，[cli.go:2936-2945](../internal/cli/cli.go#L2936-L2945)） | subject = 首行非空（[engine.go:49-56](../internal/engine/engine.go#L49-L56)）；kiln 自己写的提交 `subject == title`（[trailer.go:165-170](../internal/trailer/trailer.go#L165-L170)），两者只在手写提交上分叉 |
| `domain` | 单条：`Fields.Domain`；issue 级：`Issue.Domain` = 时间上最后一个非空值（[engine.go:68-70](../internal/engine/engine.go#L68-L70)） | `list --json`/`graph --json` 用 issue 级，`pathAction` 用单条 |
| `tags` | 单条：`Fields.Tags`；issue 级：跨动作并集（[engine.go:71](../internal/engine/engine.go#L71)、[engine.go:141-148](../internal/engine/engine.go#L141-L148)） | 同上 |

`refs` / `target` / `scope` / 关系不在本 issue 的字段集里，但同一次 `trailer.Parse` 就能拿到，后续要加不必改接口。

### 6. 顺序与时间

- **默认 `git log` 不是 author-date**：默认是 commit-date + 拓扑；要 author-date 必须显式 `--author-date-order`。它是“拓扑约束 + author-date 打破平局”，**合并会让倒序不严格单调**。
- `%aI` 是 author date；`%cI` 是 committer date，rebase/amend 后两者不同。现有 `Commit.Date` 只来自 author date。
- 100k 合成仓实测（[timeline-stream.md](timeline-stream.md) 的 test）：author-date 首行有 commit-graph 64ms / 无 359ms；`--limit 50` 70 / 366ms；全量 1.68s / 1.35s；`path --all` 首块 954ms、35.6MB。
- 因此“首行 ≤250ms”只能在有 commit-graph 时成立；commit-graph 是仓库侧状态，kiln 只读、不能替用户生成。

### 7. 空仓、无动作、仓外

- **空仓实测**：`git log` 退出 128，`fatal: your current branch 'master' does not have any commits yet`；`git rev-parse --verify HEAD` 也是 128。所以“空仓 exit 0”不是免费的，必须把“还没有提交”映射成空流。
- 今天 `loadRepo` 把任何 `Log()` 错误变成 `读取历史失败` exit 1（[cli.go:2892-2895](../internal/cli/cli.go#L2892-L2895)），因此 `path`/`list` 在全新仓里是失败的；`list` 的“仓库里还没有任何动作”提示只覆盖**有提交但没有动作**（[cli.go:2556-2564](../internal/cli/cli.go#L2556-L2564)）。
- 仓外：`backend()` 报 `不在仓库里` exit 1，与 `path`/`list` 同（[cli.go:2886-2891](../internal/cli/cli.go#L2886-L2891)）。
- “空结果是答案 + stderr 一行提示”的唯一先例就是 `list`。

### 8. broken pipe：Windows 上今天会报错并 exit 1

实测（2026-10-28，当前 `kiln.exe`）：启动 `path --all --json`，读一行后关闭管道 →

```text
输出 JSON 失败: write /dev/stdout: The pipe has been ended.
exit=1
```

- Go 对 fd 1/2 的 `EPIPE`：Unix 上 runtime 转成 SIGPIPE 静默结束；Windows 没有 SIGPIPE，写操作返回错误（`ERROR_BROKEN_PIPE` / `ERROR_NO_DATA`），被我们的 `输出 JSON 失败` 接住。
- 也就是说“broken pipe 静默退出”在 Unix 上几乎是白送的，在 Windows 上必须显式识别；两个平台要表现得一样，就得自己做判断（exit 0、不打印）。
- 测试侧：所有 CLI 测试都写 `bytes.Buffer`，**没有任何用例经过关闭的管道**。可移植的替身是返回 `io.ErrClosedPipe` 的 writer（或 `io.Pipe` 关掉读端），而不是真的造管道。

### 9. 早停要动子进程，仓里没有先例

- 现在 git 一律 `cmd.Run()` + 缓冲（[git.go:447-464](../internal/vcs/git/git.go#L447-L464)）：没有 `StdoutPipe`、没有 `Kill`、没有“读一半就停”的路径。
- 逐条读取 + `--limit` / broken pipe 早停需要：(a) `StdoutPipe`；(b) 读到满足条件就停；(c) `Process.Kill()`；(d) `Wait()` 并吞掉被杀产生的错误；(e) 别让 goroutine 卡在写满的管道上。这套机器是全仓新的，也是最容易在测试里挂住的部分。

### 10. 参数、分派与帮助的落点

- `env.action` 在子命令**之前**解析 flag，目前只声明了 `json` 一个 switch（[cli.go:1239-1247](../internal/cli/cli.go#L1239-L1247)）。`--jsonl` 必须在那里声明成 switch，否则会被当成未知 flag（warning）或吞掉下一个 token；`--limit` 是值 flag，可被 `parse` 自动识别，但 `rejectMissingValues` 的 allowed 集要按子命令给。
- allowed 集必须**按子命令**：`list` 只认 `--json`，`log` 认 `--jsonl` / `--limit`；否则 `action log --json` 会被静默接受。
- 组用法现在写死 `用法: kiln action list [--json]`（[cli.go:1256](../internal/cli/cli.go#L1256)），要改成两条。
- `--limit` 的数值先例：`graph --depth` 用 `Atoi`，错误文案 `--depth 需要非负整数`，exit 2（[cli.go:2713-2734](../internal/cli/cli.go#L2713-L2734)）。
- 帮助/文案联动四处：`usageCommands` 行（[cli.go:63-78](../internal/cli/cli.go#L63-L78)）、i18n catalog、`README.md` / `README.en.md` 命令表、`machine-readable-output.md`。

### 11. 测试与假后端

- `fakeRunner` 实现 `Repo` 与 `StatLog`（[cli_test.go:379-392](../internal/cli/cli_test.go#L379-L392)、[cli_test.go:509-528](../internal/cli/cli_test.go#L509-L528)）；它的 `Log()` **原样返回 fixtures**（[cli_test.go:394](../internal/cli/cli_test.go#L394)）——现有测试不在乎顺序，因为 `Build` 会按日期重排（[engine.go:73-77](../internal/engine/engine.go#L73-L77)）。
- 流式命令不能重排，所以：(a) fixture 必须自己按 newest-first 写；(b) 新可选接口要么让 fake 实现，要么命令对 fake 回退到 `Log()`——后者会让测试完全绕开流式路径，等于没测。
- 真 git 只在少数用例里用（`t.Chdir` + 临时仓，如 [cli_test.go:1934](../internal/cli/cli_test.go#L1934)）；broken pipe / 早停更适合对着 fake 测。
- 帮助表的形状有测试约束：`TestUsageTextAlignsDescriptions`、`TestUsageTextCitesNoRepositoryDocument`（[cli_test.go:345](../internal/cli/cli_test.go#L345)、[cli_test.go:368](../internal/cli/cli_test.go#L368)）。
- i18n 没有完整性测试，缺条目只回退中文（[i18n.go:87-96](../internal/i18n/i18n.go#L87-L96)），英文文案要靠断言覆盖。

### 12. 留给 hypothesize 的问题

1. 流式接口形状：`LogEach(fn func(Commit) bool) error`（返回 bool 表示继续）？还是 iterator / channel？怎么在“可选接口 + 回退”的既有先例里表达“这里不回退”？
2. 早停与子进程：`--limit` 与 broken pipe 谁负责 `Kill`、谁负责 `Wait`、被杀的 `git` 错误怎么吞；`Wait` 必须在 reader 停止之后还是之前？
3. 文本：日期用本地时间还是 UTC、精确到分还是秒；列序（日期 / issue / action / hash / title）；对齐与表头；空结果给不给 stderr 提示。
4. `title` 语义：跟 `pathAction` 一样严格取 trailer（手写提交可能没有），还是跟 `issueTitle` 一样 subject 兜底？
5. `domain` / `tags`：单条声明还是 issue 级？注意两者不对称——newest-first 单遍里 issue 级 `domain`（最后一个非空）**可以**在遇到该 issue 最新一条时确定；issue 级 `tags`（并集）**不可**，要全史。
6. 空仓：把 “no commits yet” 当空流，还是让 git 报错？判据是匹配 stderr 文案，还是先探 `HEAD`？
7. broken pipe：两个平台统一成 exit 0 静默？Unix 的 SIGPIPE 默认行为要不要覆盖（默认就会静默死）？Windows 识别哪些错误码？
8. `action log --json` / `action --json log` 怎么处理：明确拒绝并指向 `--jsonl`，还是当未知 flag 警告？
9. NDJSON 在 `machine-readable-output` 里的例外怎么写才诚实——理由不是内存（`pathBatchWriter` 已经证明文档可流式），而是逐条可消费、可截断、`jq` 可直接吃。
10. 验收分档与 commit-graph 前提怎么写；jj 的“暂不支持”用哪条文案与退出码。

## collect（2026-10-28，第二轮）：别人怎么“逐条读 git”，怎么“逐条写给下游”

> 第一轮收的是本仓现状；这一轮收同类工具的既有做法，作为 hypothesize 的参考系。仍是事实，不是选择。
> 证据分三类：**实测**（本机跑出来的，命令与结果都写在正文）、**文档**（`rg --help` / `go help testflag` / git 官方手册）、**二进制字符串**（从已安装的可执行文件里读出的常量）。

### 1. 样本一览

| 工具 | 怎么读 git | 怎么写给下游 | 证据 |
| --- | --- | --- | --- |
| lazygit 0.65.0（本机安装） | `git log --pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%P%x00%m%x00%D%x00%s`：**一条提交一行**，字段用 NUL 隔开，行首 `+` 当记录标记；列表的 format 只取 `%s`（正文另取） | TUI，不是机器格式 | 二进制里的 format 常量 |
| git 自己 | `-z` / `%x00` 是官方机器分隔符；`cat-file --batch` 是长度前缀的请求/响应；`rev-list --stdin` 从 stdin 收版本号 | `git log -z`：每条提交一个 NUL 终止 | 实测 |
| ripgrep（本机） | 不读 git | `--json` 是 **JSON Lines**：一条一个对象，`type`/`data` 信封，末尾 `summary` 记录 | `rg --help` 原文 + 本机输出 |
| `go test` | 不读 git | `-json`：每个事件一行（`Time`/`Action`/`Package`/`Test`/`Output`），最后一条是包级 `pass`/`fail` | `go help testflag` + 本机输出 |
| jj 0.45.1（本机） | `jj log -T 'json(self)'`：**一条提交一行紧凑 JSON** | 同一行即记录 | 实测 |
| tig（本机） | 装上但跑不起来（`couldn't create signal pipe, Win32 error 5`） | — | 实测失败，不作证据 |

结论先说：**样本里没有“先把整段 git log 读进内存”还被称为先进做法的**；差别只在“用哪种记录框架”和“是否逐条 flush”。lazygit 能一行一条，是因为它只取 subject；kiln 要 `%B`（正文），正文里有换行，行框架不够用。

### 2. 读 git：记录框架比接口更要紧

1. **`-z` 的实测语义**：`git log --format="%H%x00%aI%x00%s"` 不加 `-z`，2 条提交得到 4 个 NUL、末字节是 `0a`（换行）；加 `-z` 得到 6 个 NUL、末字节是 `00`。即 **`-z` 给每条记录补一个 NUL 终止符，最后一条也有**。所以解析是“`ReadBytes(0)` 循环”，没有“最后一条没有终止符”的特例。
2. **NUL 比 `\x1e` / `\x1f` 硬**：现有 [logFormat](../internal/vcs/git/git.go#L27) 用 RS/US，理由是“提交信息里不会出现”；但 git 不禁止正文含这两个字节。NUL 是 git 官方 `-z` / `%x00` 的机器分隔符，也是 `cat-file --batch` 之外的通用约定。换成 NUL 不影响 `Log()`，只需要给流式路径一个自己的格式串。
3. **两种官方“逐条”接口，选记录框架而不是自造协议**：
   - `git log -z --format=…`：单向、按提交顺序、天然支持早停（实测 `git log --format=%H | 读一行` 首行 52ms 就返回）。
   - `git cat-file --batch`：**请求/响应**——stdin 写对象名，stdout 出 `<sha> <type> <size>\n` 再跟原文（实测 `HEAD` / `HEAD^{tree}` / `nope` 三条请求都得到响应）。适合“先拿 id 再取详情”，一次进程吃很多请求。
   - `git rev-list --stdin`：版本号从 stdin 进（实测可用），适合“候选集由调用方决定”。
   kiln 的场景是“按提交顺序单遍过滤”，`git log -z` 是三者里最薄的。
4. **分页不会误伤**：子进程 stdout 是管道时 git 不分页——把 `GIT_PAGER` 设成不存在的命令，piped `git log` 仍 exit 0、正常输出（实测）。只有 `--paginate` 或 tty 才触发。现有 [runIn](../internal/vcs/git/git.go#L447-L464) 不设 `GIT_PAGER` 也没问题，但值得写进注释，免得以后有人加 `--paginate`。
5. **首行延迟**：本仓 350 提交，`git log --format=%H` 读到首行 52ms（含 PowerShell 开销）；100k 的 64ms（有 commit-graph）/ 359ms（无）见 [timeline-stream.md](timeline-stream.md)。说明 git 是逐条 flush 的，逐条读能立刻拿到。
6. **`bufio.Scanner` 的 64KB 上限**：默认 token 上限 64KB，提交正文可以更大（一条几千字的 collect 就接近）。流式解析要用 `bufio.Reader` + `ReadBytes(0)`（任意长度），不要用 `Scanner`。
7. **早停必须杀子进程**：不读就走，git 会阻塞在写满的管道上；`Kill` 之后 `Wait` 会返回“被信号杀死”的错误，要吞掉，不能当失败上报。

### 3. 写下游：四种形态，代价不同

| 形态 | 代表 | 记录边界 | 消费者怎么判断“被截断” |
| --- | --- | --- | --- |
| NDJSON / JSON Lines | `rg --json`、`go test -json`、`jj -T json(self)` | 换行（JSON 字符串里的换行会被转义，所以一条记录永远一行） | rg 有末尾 `summary`；go test 有包级 `pass`/`fail`；jj 没有 |
| NUL 分隔 | `git log -z`、`find -print0` | NUL | 只能靠退出码 |
| 单文档 + 流式生成 | kiln `pathBatchWriter` | 文档结构（`]}`） | JSON 解析失败即截断 |
| 长度前缀 | `git cat-file --batch` | 头部声明的字节数 | 读不满即截断 |

### 4. 写下游：四条被反复验证的细节

1. **每条 flush**：rg / `go test -json` / jj 都是“事件发生就出现在 stdout”；kiln 的 [pathBatchWriter](../internal/cli/cli.go#L1747-L1841) 只在 `finish()` flush——对完整文档正确，对流式首行是灾难（64KB 缓冲要先填满）。逐条流式 = 每条 `Flush`（或 1KB 级缓冲）。
2. **stdout 只放记录，提示/错误走 stderr**：rg、git、kiln 都是这个分工。唯一的“反例”是 `go test -json` 把测试输出也包进 JSON 事件——那是为了不让消费者丢上下文，代价是记录变胖。
3. **早停时“静默成功”是常见选择，但不是唯一选择**：实测“消费者读一行就关管道”时，**git 与 rg 都退出 0、都静默无 stderr**（rg 用有命中的输入复测，避免把“无命中 = 1”误读成破管道）；kiln 的 `path --all --json` 则是打印 `输出 JSON 失败: … The pipe has been ended.` 并退出 1。也就是说“截断算不算失败”是产品决定：主流工具选 0，kiln 现在选 1。
4. **终止记录是有意的**：rg 的 `summary`、go test 的包级 `pass`/`fail` 让消费者能分辨“正常读完”和“被截断”。kiln 的字段集（`issue/action/commit/date/title/domain/tags`）没有终止记录——**截断只能靠退出码与行数判断**；要不要加是一条待裁决项（加就超出 ask 已定的字段）。
5. **信封 vs 扁平**：rg 用 `{"type","data"}`，因为它有五种记录类型；`go test -json` 和 jj 是扁平对象，因为只有一种记录。`action log` 只有一种记录 → 扁平即可，不必学 rg 的信封。
6. **非 UTF-8 是真实风险**：rg 用 `text` / `bytes` 两态处理“路径/内容不保证是 UTF-8”；Go 的 `encoding/json` 遇到非法 UTF-8 不会报错，而是**静默替换成 U+FFFD**。git 不校验提交信息编码，`issue`/`title` 里理论上可以有坏字节——要么 sanitize（`strings.ToValidUTF8`），要么接受“改了数据但仍是合法 JSON”。

### 5. 和 kiln 现状的差距（只列缺口）

| 需要 | 现状 | 参考 |
| --- | --- | --- |
| 逐条读 git | 只有 `Log()` 全量 | `git log -z` + `ReadBytes(0)` |
| 记录框架抗正文 | `\x1e`/`\x1f`（非保证） | NUL（git 官方分隔符） |
| 早停 | 无 | `Kill` + `Wait` 吞错 |
| 每条 flush | 只有 `finish()` flush | rg / go test / jj |
| broken pipe 静默 | Windows 打印错误、exit 1 | git 与 rg 均 exit 0、静默 |
| 空仓 exit 0 | `git log` 128 → 现在 exit 1 | 需自己映射空流 |
| NDJSON 先例 | 无 | rg / go test / jj |

### 6. 给 hypothesize 的选项（不是结论）

1. 接口：`LogEach(fn func(Commit) bool) error`，`false` 表示停；git 用 `-z` 流式实现，jj 不实现并显式报错。要不要另外给 `vcs` 一个“该后端不支持流式”的能力查询，还是直接类型断言失败即报错？
2. 分帧：沿用 `\x1e/\x1f`，还是给流式路径单独一个 `-z` + `%x00` 的格式串？后者更硬，代价是两套解析。
3. flush 粒度：每条 flush（最直白），还是 1–4KB 缓冲（少 syscall，首行仍可接受）？
4. 退出码：早停/破管道统一 0；`--limit` 用完 0；jj 不支持用 1 还是 2；空仓 0。
5. 非 UTF-8：sanitize 还是像 rg 那样两态（后者超出字段集，且改变消费方形状）。
6. 终止记录：不加（守 ask 的字段集），还是加一条 `type:"summary"`（就与“一行一条动作”的承诺冲突，需要重新定义）。
