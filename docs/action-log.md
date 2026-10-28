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
5. `domain` / `tags`：单条声明还是 issue 级？**更正**：上一版这里写成“issue 级 `domain` 可在 newest-first 单遍里确定”，是错的——只有“该 issue 最新一条自己声明了 domain”时才成立；若最新一条没声明、声明在更老的提交上，单遍读到最新一条时还没见过它，早已输出过。所以严格流式下 `domain` 也只能是单条；`tags` 并集同理不可得。
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

## hypothesize（2026-10-28）：`LogEach` 单遍流式；`-z` 三字段分帧；每条一次写；破管道静默 0

> 结论先行：`vcs` 增加可选接口 `LogEach(fn func(Commit) bool) error`，git 用 `git log -z --author-date-order --format="%H%x00%aI%x00%B"` 逐条读，jj 不实现并显式报错（**不回退**到 `Log()`）；命令层每写一条就落地、不缓冲，`--limit` 与破管道都靠 sink 返回 `false` 早停；`--jsonl` 是 NDJSON，字段与 ask 一致，`title` 用“trailer title → subject 兜底”，`domain`/`tags` 只给本条提交自己的声明；空仓映射成空流 exit 0；Windows 上破管道静默 exit 0，Unix 保持 SIGPIPE 的平台惯例。

### 0. 先更正一条事实

collect 第一轮说“issue 级 `domain` 可以在 newest-first 单遍里确定”，**这是错的**：若最新一条没声明 `domain:`、声明在更老的提交上，读到最新一条时还没见过那个声明。严格流式下 `domain` 只能是**本条提交自己的声明**；`tags` 并集同理不可得。下面的方案按更正后的事实走。

### 1. 接口：`vcs.LogEach`，可选但**不回退**

```go
// LogEach streams commits newest-first to fn and stops as soon as fn returns
// false. It is the streaming form of Repo.Log: a backend that can read history
// incrementally implements it, so a large repository never has to be held in
// memory before the first row is printed.
//
// It is optional, but there is no fallback: Log() would turn the stream back
// into a buffer, which is the one thing the caller asked not to happen. A
// backend that does not implement it is reported as unsupported.
type LogEach interface {
	LogEach(fn func(Commit) bool) error
}
```

- 与 `StatLog` 同一种“可选接口 + 类型断言”的形状，但**故意没有回退**（`StatLog` 的回退是等价语义，这里的回退是相反的语义）；
- sink 用 `bool` 而不是 `error`：写入失败/破管道由闭包记在局部 `err` 里，返回 `false` 让后端干净地停；后端不需要为“正常停止”发明哨兵错误；
- `false` 必须让后端**杀掉子进程**，不能只是不再读（否则 git 阻塞在写满的管道上）。

### 2. git 实现：`-z` + 三个 NUL 字段

```go
const streamLogFormat = "%H%x00%aI%x00%B" // 记录之间由 -z 补 NUL
```

- 命令：`git log -z --author-date-order --format=…`，`StdoutPipe` + `bufio.Reader`（**不用 `Scanner`**，64KB 上限）；
- 解析：按 NUL 读三段（hash / 日期 / 正文），第三段的结尾 NUL 就是 `-z` 的终止符；实测 1 条提交 = 3 个 NUL、末字节 `00`，所以“读到三段就是一个完整记录”，没有“最后一条没有终止符”的特例；
- 正文沿用 `Log()` 的 `TrimRight(…, "\r\n")`，两条路径对同一条提交给出同样的 message；
- 为什么换 `-z`/`%x00`：`\x1e`/`\x1f` 是“实践中不出现”，NUL 是 git 自己的机器分隔符（`-z` 文档 + `cat-file --batch` 同族），且**不要求正文没有换行**；
- 为什么不用 `cat-file --batch`：那是“先有 id 再取详情”的请求/响应，`action log` 只需要一遍顺序扫描，`log` 更薄；
- 早停：sink 返回 `false` → 关 reader、`Process.Kill()`、`Wait()` 并**吞掉**被杀错误，返回 `nil`；
- 空仓：`git log` 在空仓退出 128。只在**一条记录都没读到**的失败路径上补一次 `git rev-parse --verify --quiet HEAD`（空仓 exit 1、非空 exit 0，实测）：exit 1 → 当作空流返回 `nil`；否则把原错误往上抛。happy path 仍然只有一个进程。

### 3. jj：显式不支持，门留着

- `jj.ExecRunner` 不实现 `LogEach`，命令报 `action log 暂不支持 jj（当前只实现 git 的流式读取）`，exit 1；
- 不用 `Log()` 兜底：那会静默变成“全量读 + 排序”，与本命令的交付承诺相反；
- 未来实现成本很低：实测 `jj log -T 'json(self)'` 就是一条提交一行紧凑 JSON（含 `commit_id` / `author.timestamp` / `description`），**不需要改接口**，只需在 jj 侧实现 `LogEach`。这一句写进文档，避免后来者以为要重设计。

### 4. 命令与交付

**文本**（一行一条，无表头/legend）：

```text
2026-10-28 22:17  action-list             collect      80d1a9c  collect：把问题写进契约文档
```

- 日期 `2006-01-02 15:04`，**本地时区**（文本给人看；JSON 保留原 offset）；
- issue 按 `displayWidth` 补到 24、action 补到 12（同 `path` 的 12）、hash 取 7（`shortID`），超过宽度的不截断、只是该行右移；
- 末尾 `TrimRight`，不留尾随空格；`kiln action log | wc -l` 就是动作数。

**`--jsonl`**（NDJSON，字段顺序固定）：

```json
{"issue":"action-log","action":"collect","commit":"edbe9be…","date":"2026-10-28T22:17:33+08:00","title":"collect 第二轮：…","domain":"cli"}
```

- 字段：`issue / action / commit / date / title / domain / tags`，与 ask 一致；
- `commit` 全长 hash；`date` 用 `time.RFC3339`（保留提交原始 offset），与 `path --json` / `show --json` 同；
- `title` / `domain` / `tags` 为空时**省略键**（缺省即没有）；`tags` 是本条提交的 `tags:`；
- 一行一个对象、以 `\n` 结尾；用 `Encoder` + `SetEscapeHTML(false)` 写进 scratch 再输出，保证与 `writeJSON` 同样的转义习惯。

**写入**：每条记录直接写 `e.out`（一行一次 `Write`），**不套 `bufio`**——`pathBatchWriter` 的 64KB 缓冲正是首行延迟的敌人，而流式场景每条本来就要落地。10 万行 ≈ 10 万次写，相对 1.6s 的全史扫描可以忽略。

**空结果**：exit 0；stderr 复用 `list` 已有的一句提示（`仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）`），中文与英文条目都在 catalog 里，不新增文案。

### 5. 字段语义（三条要写进契约）

1. **`title` = `title:` trailer，缺省时取提交首行**。和 `list --json` 的 `issueTitle` 同一条规则；`path --json` 的 `actions[].title` 仍是严格 trailer。ask 里文本写的是 subject——这里统一成同一个 label（没有 `title:` 时它就是 subject），因为同一行在两个视图里必须同名，否则“这条动作叫什么”取决于你问谁。kiln 自己写的提交 `subject == title`，两者只在手写/早期提交上分叉。
2. **`issue` 是 trailer 原值，不 normalize**。`path` / `list` 也用它当键；`do` 写前才 normalize，所以老提交可能不规范，视图保持原样。
3. **`domain` / `tags` 只给本条提交的声明**，不给 issue 级（见 §0）。当前分类去 `list --json` / `status --json` 取；这条差异写进 `machine-readable-output`，不让消费方猜。

另外：**不按当前词表过滤**。不在词表里的历史动作照样出现——这是与 `path`/`list`/`check` 一致的定义；合法性是 `check` 的事，不是读路径的事。

### 6. broken pipe：静默退出

- Windows：写 `e.out` 报错且 `isBrokenPipe(err)` 为真 → 不再读、exit **0**、不打印；
- `isBrokenPipe` 用两个构建标签文件放：`pipe_windows.go` 认 `io.ErrClosedPipe` / `syscall.EPIPE` / `ERROR_BROKEN_PIPE` / `ERROR_NO_DATA`，`pipe_other.go` 认前两个；
- Unix：不覆盖 Go runtime 的 SIGPIPE 行为（写 fd 1 收到 EPIPE 会直接终止进程），这与 git / rg 一致；文档写明 `kiln action log | head` 在 Unix 上可能是 141 收场，shell 通常看到的是 `head` 的 0；
- 实测依据：消费者读一行就关管道时 git 与 rg **都 exit 0 且无 stderr**；kiln 现在是打印错误 + exit 1，属于要修的那一类。

### 7. 参数与帮助

- `action` 组解析：switches 加 `jsonl`；allowed 按子命令给——`list` = `{json}`，`log` = `{jsonl, limit}`；
- `action log --json`（或 `action --json log`）**明确拒绝**：`action log 用 --jsonl（一行一个动作），不用 --json`，exit 2；
- `--limit`：`Atoi`，`< 1` 报 `--limit 需要 ≥ 1 的整数（当前: %q）`，exit 2；重复给按 `rejectRepeats` 拒绝；
- 多余位置参数 `rejectExtra`，exit 2；
- 组用法改成 `用法: kiln action list [--json] | kiln action log [--jsonl] [--limit N]`；
- `usageCommands` 加一行：`kiln action log [--jsonl] [--limit N]` / `全局动作流（一条动作一行；--jsonl 给 agent 读；--limit 只看最近 N 条）`；
- 不读 issue 模型：`action log` 只走 `e.backend()` + `trailer.Parse`，不调 `loadRepo`，因此坏掉的历史（不合规 issue id、未知 action 名）不影响它。

### 8. 实现落点

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | `LogEach` 接口与文档；不改 `Repo` |
| `internal/vcs/git/git.go` | `LogEach` 实现：`-z` 流式、三段解析、早停 Kill、空仓探 HEAD |
| `internal/vcs/jj/jj.go` | 不改（不实现 `LogEach`） |
| `internal/cli/cli.go` | `action` 分派加 `log`；`actionLog`；`actionLogDoc`；组用法；`usageCommands` 行 |
| `internal/cli/pipe_windows.go` / `pipe_other.go` | `isBrokenPipe` |
| `internal/i18n/catalog.go` | 新文案的英文 |
| `README.md` / `README.en.md` | 命令表加 `action log` |
| `docs/machine-readable-output.md` | NDJSON 例外小节 + 字段语义（title 兜底、per-commit domain/tags、退出码） |
| `internal/cli/cli_test.go`、`internal/vcs/git/git_test.go` | 下表 |

### 9. 测试清单

1. **文本**：3 条 fixture（newest-first）→ 3 行；顺序、日期格式、7 位 hash、`title` 兜底都对。
2. **JSONL**：2 行、逐行 `json.Unmarshal` 成功；`commit` 全长、`date` 带 offset；`title/domain/tags` 缺失时**没有**这些键。
3. **`--limit` 早停**：5 条 fixture + `--limit 2` → 2 行，且 fake 记录“只被要求了 2 条”（证明不是读完再切）。
4. **`--limit` 非法**：`0` / `-1` / 非数字 → exit 2。
5. **空历史**：fake 直接返回零条 → exit 0、stdout 空、stderr 有“还没有任何动作”提示。
6. **破管道**：writer 第 2 次返回 `io.ErrClosedPipe` → exit 0、无 stderr、fake 早停（跨平台；不依赖真实管道）。
7. **jj / 不支持的后端**：只实现 `Repo` 的 fake → exit 1 + 文案。
8. **`--json` 被拒**：exit 2，文案指向 `--jsonl`。
9. **`--jsonl --limit` 组合**：2 行、2 个对象。
10. **`title` 兜底**：无 `title:` trailer 的提交 → 用首行；文本与 JSONL 同值。
11. **原值保留**：`issue: Foo_Bar` → 输出 `Foo_Bar`（不 normalize）。
12. **词表外动作**：`action: retired` 仍出现。
13. **真仓冒烟**：临时 git 仓 3 提交（2 个动作、1 个非动作）→ 文本 2 行；空仓 → exit 0。
14. **help**：`usageText` 含两条 `action` 用法；现有对齐测试仍过。

### 10. 风险与证伪

- **首行延迟**：无 commit-graph 的 100k 仓若 > 400ms，“流式”就只剩内存收益；falsifier 是实测超标 → 文档降级为“不保证首行上限”，先别把 ≤250ms 写死。
- **`-z` 与 `%B` 的假设**：如果某些 git 版本在 `-z` 下给记录补的不是一个 NUL，三段解析会错位；用“读到 EOF 视为结束、半条记录报错”兜底，并保留真仓冒烟。
- **拒绝 jj**：jj 用户会拿到 exit 1；falsifier 是有人真的要用 → 按 §3 用 `json(self)` 实现，不改接口。
- **per-commit `domain` 大多为空**（多数 issue 只在 ask 声明一次）：若消费方要“按当前分类筛”，只能 join `list`；falsifier 出现就考虑加一个显式非流式的取法，而不是偷偷在流里补。
- **Unix 141**：脚本 `set -o pipefail` 会把 `| head` 当失败；若真成为痛点，再评估 `signal.Ignore(SIGPIPE)` 统一成 0（Windows 不受影响）。
- **文本列宽**：issue 超过 24 时该行右移；改用固定不补白也可，先按 `path` 的习惯。
- **NDJSON 例外**：想要“一份文档”的消费者会失望；拒绝 `--json` 时已指到 `path --all --json` / `--jsonl`，不再提供第二条 `--json` 路径。

### 11. 下一步

- 若无异议：`conclude` 定稿后 `apply`，实现顺序按 §8（`vcs.LogEach` → git 实现 → 命令 → 文档）。
- 异议可能集中在三处：`title` 兜底（要不要和 `path` 一样严格）、`domain` 只给单条（要不要牺牲流式换 issue 级）、Unix 破管道是否统一 0。这三处任一改动都不影响其余方案。

## conclude（2026-10-28）：action log 定稿——git 单遍流式，一行一动作，`--jsonl` 逐条

### 0. 裁决

| 项 | 决定 | 理由 |
| --- | --- | --- |
| 入口 | `kiln action log [--jsonl] [--limit N]` | ask 已定；`action` 组只读，与 `action list` 同组 |
| 后端接口 | 可选接口 `vcs.LogEach(fn func(Commit) bool) error`，**无回退** | 回退到 `Log()` 等于把流变回缓冲；与 `StatLog` 的“可选+回退”区别在于回退语义相反 |
| git 读法 | `git log -z --author-date-order --format="%H%x00%aI%x00%B"`，`bufio.Reader` 按 NUL 读三段 | `-z` 给每条记录补 NUL（实测末字节 `00`），不要求正文无换行；NUL 是 git 官方分隔符，比 `\x1e/\x1f` 硬 |
| 顺序 | author-date 倒序（`--author-date-order`） | ask 已定；它是拓扑序 + author-date 打破平局，合并可能让严格倒序不成立——文档写明 |
| 早停 | sink 返回 `false` → 关 reader、`Kill`、`Wait` 吞错 | `--limit` 与破管道共用；不 Kill 会让 git 卡在写满的管道上 |
| 空仓 | 零记录 + `git log` 失败时补 `git rev-parse --verify --quiet HEAD`：exit 1 → 空流 | 空仓 `git log` 退出 128；HEAD 探测只看退出码，不受语言影响，happy path 仍是单进程 |
| jj | 不实现 `LogEach`，显式 exit 1 | 不静默缓冲；将来用 `jj log -T 'json(self)'` 实现，不改接口 |
| 文本 | 一行一条：本地日期到分 / issue / action / 7 位 hash / label | 与 `path` 的列习惯一致；`| wc -l` 即动作数 |
| `--jsonl` | NDJSON，一行一个对象 | 逐条可消费、可截断；`--json` 明确拒绝并指到 `--jsonl` |
| `title` | `title:` trailer，缺省取提交首行 | 与 `list --json` 的 `issueTitle` 同规则；文本与 JSONL 用同一个 label |
| `domain` / `tags` | 只给本条提交的声明 | 严格流式下 issue 级 domain/tags 拿不到（collect 的更正）；当前分类去 `list`/`status` 取 |
| 过滤 | 只认 `issue:` + `action:`，不按当前词表过滤 | 与 `path`/`list`/`show`/`check` 对“动作”的定义一致 |
| 破管道 | Windows 静默 exit 0；Unix 保持 SIGPIPE（可能 141） | 实测 git 与 rg 都静默 exit 0；Unix 的 141 是平台惯例，不覆盖 |
| 空结果 | exit 0 + stderr 复用 `list` 的提示 | 空是答案，stdout 保持干净 |

### 1. 命令契约

```text
$ kiln action log
2026-10-28 22:17  action-list             collect      80d1a9c  collect：把问题写进契约文档
2026-10-28 22:10  action-log              collect      edbe9be  collect 第二轮：同类工具的逐条读 git 与逐条交付做法

$ kiln action log --limit 1
2026-10-28 22:17  action-list             collect      80d1a9c  collect：把问题写进契约文档

$ kiln action log --jsonl
{"issue":"action-list","action":"collect","commit":"80d1a9c…","date":"2026-10-28T22:17:33+08:00","title":"collect：把问题写进契约文档","domain":"cli"}
```

- 文本：日期 `2006-01-02 15:04` **本地时区**；issue 补到 24、action 补到 12（同 `path`）、hash 取 7；超出宽度不截断，该行右移；末尾不留空格；无表头/legend。
- JSONL：字段顺序 `issue / action / commit / date / title / domain / tags`；`commit` 全长；`date` 为 `time.RFC3339` 并保留提交原 offset；`title` / `domain` / `tags` 为空时**省略键**；一行一个对象、`\n` 结尾；`SetEscapeHTML(false)`，与 `writeJSON` 的转义习惯一致。
- `issue` 是 trailer 原值（不 normalize）；`action` 已小写；词表外的历史动作照常出现。

### 2. 参数与错误文案（apply 照抄）

| 情形 | 输出 | 退出码 |
| --- | --- | --- |
| `--limit N`，N ≥ 1 | 最多输出 N 条后正常结束 | 0 |
| `--limit 0` / 负数 / 非整数 | `--limit 需要 ≥ 1 的整数（当前: %q）` | 2 |
| 重复 `--limit` | `flag --limit 只能给一次（当前给了 %d 次）`（`rejectRepeats`） | 2 |
| `--json`（含 `action --json log`） | `action log 用 --jsonl（一行一个动作），不用 --json` | 2 |
| 多余位置参数 | `rejectExtra` 的标准文案 | 2 |
| 无子命令 / 未知子命令 | 组用法 `用法: kiln action list [--json] \| kiln action log [--jsonl] [--limit N]` | 2 |
| jj 后端 | `action log 暂不支持 jj（当前只实现 git 的流式读取）` | 1 |
| 读历史失败 | `读取历史失败: %v`（与 `loadRepo` 同文案） | 1 |
| 空仓 / 无动作 | stderr `仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）` | 0 |

`--limit` 数的是**输出的动作数**，不是扫描的提交数；够数即停，不读剩余历史。

### 3. 交付与内存

- 每条记录一次 `Write` 到 `e.out`，不套 `bufio`（首行不等待缓冲）；`pathBatchWriter` 那套 64KB 缓冲不适用于逐条流。
- 内存：O(1)（一条记录 + `bufio.Reader` 的 64KB 缓冲），不再持有全史。
- 不调 `loadRepo`：只走 `e.backend()` + `trailer.Parse`，坏掉的历史（不合规 issue id、未知 action 名）不影响本命令。

### 4. 契约要写进 `machine-readable-output`

新增一节，明确三件事：

1. **NDJSON 是“一份文档”规则的显式例外**：stdout 是行分隔的 JSON 流，不是单个文档；理由不是内存（`pathBatchWriter` 已证明文档可流式），而是**逐条可消费、可截断、`jq`/`awk` 直接吃**；空流（0 行）是合法结果。
2. **字段语义**：`title` 是“trailer title → subject 兜底”；`domain`/`tags` 是**本条提交**的声明，不是 issue 级；issue 级分类去 `list --json` / `status --json`。
3. **退出码**：0 = 正常（含空结果、`--limit` 用尽、Windows 破管道）；1 = 后端不支持 / 读取失败；2 = 参数错误；Unix 上被 SIGPIPE 终止时 shell 看到 141，属平台惯例。

### 5. 验收

- `vcs.LogEach` 在 git 后端实现，jj 不实现；
- 14 条测试全过（hypothesize §9）：文本顺序与格式、JSONL 逐行可解析、`--limit` 早停且**后端只被要求 N 条**、非法 `--limit`、空历史、破管道（`io.ErrClosedPipe`）静默 0、jj 不支持、`--json` 被拒、`--jsonl --limit`、`title` 兜底、issue 原值、词表外动作、真仓冒烟、help；
- `go test ./...`、`go vet ./...`、`gofmt -l` 干净；
- 性能按 test 分档：有 commit-graph 首行 ≤250ms、无 ≤400ms（100k 合成仓），全量 ≤2s；**不写“无条件 ≤250ms”**；
- 本仓冒烟：`action log` 行数 = 当前动作总数（含本次新提交），`--jsonl` 逐行可解析，`--limit 3` 只出 3 行。

### 6. 非目标与遗留

- 不做 `--json`、`--all`、`--stat`、`--domain/--tag/--action` 过滤、`action show/search`；
- 不改 `path` / `list` / `show` 的任何行为；
- 遗留 1：jj 的流式读取（`json(self)` 已可行，另开 issue 或在本 issue apply 后补）；
- 遗留 2：Unix 上 `| head` 可能 141，若成为痛点再评估 `signal.Ignore(SIGPIPE)`；
- 遗留 3：`domain` 只有 per-commit 值，按当前分类筛需求出现时再议（不要偷偷在流里补）；
- 遗留 4：无终止记录，截断只能靠退出码/行数判断；加 `summary` 会与“一行一条动作”冲突，暂不加。

### 7. 下一步

`apply`：按 `vcs.LogEach` + git 实现 → `actionLog` + 分派 + 管道判断 → 测试 → 文档（machine-readable-output、README 双语、help/i18n）的顺序落地。
