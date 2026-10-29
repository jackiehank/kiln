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

## apply（2026-10-28）：落地 `action log`，含两处实现期修正

### 1. 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | 新增可选接口 `LogEach(fn func(Commit) bool) error`；文档写明“可选但不回退” |
| `internal/vcs/git/git.go` | `LogEach`：`git log -z --author-date-order --format="%H%x00%aI%x00%B"` + `bufio.Reader` 三段解析；早停 Kill + 关管道；空仓探 `rev-parse --verify --quiet HEAD` |
| `internal/engine/engine.go` | `subject` → 导出 `Subject`，供 `action log` 复用同一条“首行兜底”规则 |
| `internal/cli/cli.go` | `action` 分派加 `log`；`actionLog`；`actionLogDoc`；组用法与 `usageCommands` 行；`--json` / `--jsonl` / `--limit` 的互相拒绝 |
| `internal/cli/pipe_windows.go` / `pipe_other.go` | `isBrokenPipe` 分平台实现 |
| `internal/i18n/catalog.go` | 8 条新文案的英文 |
| `internal/cli/cli_test.go` | 9 个 `TestActionLog*` 用例 + `streamRunner` / `brokenPipeWriter` 两个测试替身 |
| `internal/vcs/git/git_test.go` | 3 个 `TestLogEach*` 用例 + `commitEmpty` 辅助 |
| `README.md` / `README.en.md` / `docs/machine-readable-output.md` | 命令表 + NDJSON 契约 |

### 2. 实现期发现并修正的两件事

**（a）Git for Windows 的 `git.exe` 是 shim。** 只 `Process.Kill()` 直接子进程时，真正的 git 仍活着并持有继承来的管道；`cmd.Wait()` 会卡在 stderr 复制 goroutine 上，`--limit` 直接挂死（实测：命令挂 120s，`Get-Process` 里还有一个 git）。修法两条并用：

- 停的时候**关掉我们自己这一端的 stdout 管道**——真 git 下一次写就失败并退出；
- 给 `cmd.WaitDelay` 一个 3s 上限，兜住任何仍被占用的 I/O。

修后 `action log --limit 3` 与 `--jsonl --limit 1` 都在 ~117ms 返回（含 git 启动）。

**（b）空结果提示在英文模式下一直没翻译。** `list` 那句用的是 `fmt.Fprintln` + 不带 `\n` 的 key，而 catalog 的 key 带 `\n`，查找永远落空。改成 `fmt.Fprint` + 带 `\n` 的 key（两处），英文空仓现在输出 `no actions in this repository yet (...)`。

### 3. 验证

- `gofmt -l internal cmd` 干净；`go vet ./...` 无输出；`go test ./...` 全过（cli 2.9s / git 11.8s）；
- 真仓：`action log` 339 行 = `status --json` 的 339 个动作；`--limit 3` 只出 3 行且 ~117ms；`--jsonl --limit 2` 两行都是合法 JSON，`commit` 全长、`date` 带 `+08:00`、未声明的 `domain`/`tags` 不出现；
- 拒绝：`--json`（exit 2，文案指向 `--jsonl`）、`--limit 0`（exit 2）、`action list --jsonl`（exit 2）；
- 空仓（临时 `git init`）：exit 0、stdout 空、stderr 一行提示；jj 仓（`jj git init --colocate`）：exit 1、`action log 暂不支持 jj（当前只实现 git 的流式读取）`；
- 破管道：用 Go 父进程读一行后关管道，kiln exit 0 且 stderr 为空（PowerShell 的 `StandardOutput.Close()` 不真正关句柄，不能拿它当破管道测——那次“挂起”是写满缓冲的正常背压，不是缺陷）；
- 英文：`--lang en action log --json`、`--limit 0`、空仓提示、`action -h` 全部无中文。

### 4. 遗留（与 conclude 一致，未变）

- jj 的流式读取（`json(self)` 一行一提交，已实测可行）；
- Unix 上 `| head` 可能以 141 收场；需要统一 0 时再评估 `signal.Ignore(SIGPIPE)`；
- `domain` / `tags` 只有 per-commit 值，按当前分类筛要走 `list` / `status`；
- 无终止记录，截断只能靠退出码与行数判断。

## collect（2026-10-28，第三轮）：jj 到底能不能流式读

> 触发问题：jj 的兼容模式（colocated）本来就用 git，能不能直接读 `.git`？原生 `jj log` 能不能像 git 那样逐条吐？
> 实测环境：jj 0.45.1、git 2.55.0.windows.3。

### 1. 先纠正前提：colocated 现在是 jj 的**默认**

- `jj git init --help` 原文：`--colocate` “**This is the default**”，除非 `git.colocate=false`；`--no-colocate` 才把 git 仓库藏进 `.jj/repo/store/git`。
- 所以主流 jj 仓库都有 `.git`，git 确实能读——用户的直觉对了一半。
- 没 colocate 也不是没有 git：`.jj/repo/store/git` 就是一个真 git 仓库，`git --git-dir=.jj/repo/store/git log --all` 实测可读（2 条用户提交 + 4 条空 message 的内部提交，靠 `refs/jj/keep/*` 保活）。

### 2. colocated 里 git 看到的是什么（实测）

| 场景 | `jj log` | `git log`（HEAD） | `git log --all` |
| --- | --- | --- | --- |
| 两个 jj 提交 | 2 条 | 2 条，一致 | 更多（keep refs） |
| 改写（`jj describe -r @-`）后 | 只有编辑后的 | 只有编辑后的 | **旧提交还在**（`second: probe` 8534639）+ 空 message 内部提交 |
| 空仓 | 0 条，exit 0 | 无提交 | — |

- jj 把被改写的提交**藏起来**，但仍用 `refs/jj/keep/<hash>` 保活；`--all` 会把它们翻出来，等于给同一次动作多一行。
- HEAD 链接近 jj 的可见集，但 HEAD 只有一条链；`all()` 覆盖所有可见 head（多 bookmark / 多工作区），HEAD 链会漏。
- jj 改写会换 `commit_id`（`ade098d` → `3a4d880`），这本身就是两套模型的分歧；读 git 会让一次动作以两个 id 出现。

### 3. 原生 `jj log` **不是流式的**（实测）

| 仓规模 | 命令 | 首行 | 全量 |
| --- | --- | --- | --- |
| 200 提交 | `jj log -T 'json(self) ++ "\n"'` | **141ms** | 152ms |
| 300 提交 | 同上 | — | 163ms |
| 300 提交 | 同上 + `-n 3` | — | 116ms |
| 300 提交 | 同上 + `-n 50` | — | 121ms |
| 300 提交 | `git log --author-date-order` | — | 71ms |

- 200 提交时**首行 ≈ 全量**：jj 先把 revset/渲染算完再吐第一字节，与 git 的“首行 64ms / 全量 1.35s（100k）”是两种曲线。
- `-n 3` 仍要 116ms：`--limit` 下推给 jj 只省渲染和输出，省不了扫描。
- 结论：jj 上没有“首行早于全量”的免费午餐；`action log` 在 jj 上若要成立，交付契约得从“首行流式”退成“输出与内存流式”。

### 4. 记录框架与字段（实测）

- `-T 'json(self)'` **不带换行时 200 个对象挤在一行**（`Measure-Object -Line` 得 1）；必须 `json(self) ++ "\n"`。JSON 会把正文换行转义成 `\n`，所以“一条提交一行”成立。
- `json(self)` 的 `author.timestamp` 是 RFC3339 带 offset（`2026-10-28T22:18:36+08:00`）；模板里直接 `author.timestamp()` 得到的是 `2026-10-28 22:47:04.000 +08:00`（空格 + 毫秒），不能直接当 RFC3339 解析。
- `description` 带尾部 `\n`；正文就是 commit message，`trailer.Parse` 原样可用，不需要新的解析器。
- **顺序是拓扑序**：`jj log --help` 写的是 “ordered with children before parents”；revsets 没有 `sort()`，`--reversed` 只翻转。要 author-date 只能自己缓存排序（O(n) 内存），与本 issue 的卖点直接冲突。
- 空仓免费：`all() ~ root() ~ @` 为 0 条、exit 0，不像 git 那样 128。
- 分页：管道下把 `PAGER` / `JJ_PAGER` 设成不存在的命令也不影响（实测）；另有 `--no-pager`。

### 5. 四条可选路线

| 路线 | 读法 | 优点 | 代价 |
| --- | --- | --- | --- |
| A 原生 jj | `jj log -T 'json(self) ++ "\n"'` | 语义正确（隐藏被改写提交、覆盖所有 head）、空仓免费、一条一行 | **非流式**（首行=全量）；顺序是拓扑序 |
| B 读 colocated `.git` | 复用 git 的 `LogEach` | 增量、按 author-date、代码几乎零新增 | `--all` 有幽灵（keep refs / 内部提交）、HEAD 链漏其它 head、colocate 可被配置关掉 |
| C 读 `.jj/repo/store/git` | 同上，指向内部 store | 非 colocated 也能读 | 依赖 jj 内部路径（文档明说“hidden inside .jj”）、幽灵问题同 B |
| D 维持不支持 | 现状 | 契约不退化 | jj 用户拿不到 |

### 6. 留给下一轮的问题

1. **接受“jj 非流式”吗？** 若接受，`action log` 在 jj 上首行=全量，`--limit` 只省输出；文档要把“首行延迟”从契约里拿掉，改成“内存与输出流式”。
2. **顺序**：jj 只有拓扑序。接受拓扑序 + 文档说明，还是为 author-date 缓存排序（破坏流式），还是继续不支持？
3. **B/C 的语义坑**（幽灵提交、漏 head、内部路径）能不能接受？不能的话 A 是唯一正确路线。
4. 若走 A：`json(self)` 直接解析（简单、字段多、吃 jj 版本）还是自定义模板 + 分隔符（要选一个正文里不会出现的标记，现有 `@@KILN@@` 就是这种思路）。
5. `--limit` 要不要下推 `jj log -n`？实测省不了 116ms 固定成本，但能省 jj 侧渲染与输出。
6. 若仍不支持，文案从“暂不支持 jj”改成“jj 暂不支持（可先用 `kiln path --all` 看全量）”，并把原因写成“交付曲线不同”，别写成“做不到”。

## hypothesize（2026-10-28，第四轮）：jj 走原生读取，接受非增量，顺序用 jj 的拓扑序

> 结论先行：实现 jj 的 `LogEach`（原生 `jj log -T 'json(self) ++ "\n"'`），**放弃“首行早于全量”在 jj 上的承诺**，保留“内存与输出流式”；顺序沿用 jj 的拓扑序并写进文档；不做 git 回退（幽灵提交、漏 head）；`--limit` 向 jj 下推 `-n`，为此给 `LogEach` 加一个 limit 提示参数。

### 0. 为什么这次不是“继续不支持”

ask 里“jj 显式非 0”的前提是“jj 不能流式”。第三轮 collect 证明的是**不能增量**，不是**不能交付**：

- 原生 `jj log` 给出的是 jj 真正可见的提交集（改写过的隐藏、所有 head 覆盖、空仓免费）；
- jj 是 kiln 的一等后端（`jj.ExecRunner` 实现了 Repo 与 Workspace 的全部方法），留一个命令空白，插件侧会读成“这是 git-only 产品”；
- 拒绝的实际代价是让 agent 退到 `path --all --json`（本仓 35MB 量级），比让 jj 自己扫一遍更糟。

所以：**把承诺改对，而不是把功能砍掉**。

### 1. 为什么不是 B/C（读 git）

| 反对理由 | 证据 |
| --- | --- |
| `--all` 有幽灵 | 改写后旧提交仍被 `refs/jj/keep/*` 保活，`git log --all` 能看到；还有一批空 message 的内部提交 |
| HEAD 链会漏 | HEAD 只有一条链，jj 的 `all()` 覆盖所有可见 head |
| colocate 可关 | `git.colocate=false` 或 `--no-colocate` 时 `.git` 不存在 |
| 内部路径不可靠 | jj 文档明说非 colocated 的 git 仓库“hidden inside `.jj`” |

一句话：**读 git 能读到更多行，但读到的不是 jj 认为存在的东西**。

### 2. 接口：`LogEach` 加一个 limit 提示

```go
LogEach(limit int, fn func(Commit) bool) error
```

- `limit <= 0` 表示“不限”；
- 它只是**提示**：调用方仍然用 `fn` 返回 `false` 来停止，后端可以用它避免做无用功（jj 用它拼 `-n`）；
- 为什么值得改签名：300 提交实测全量 163ms、`-n 3` 116ms，差 47ms 全花在渲染 JSON 上（≈0.16ms/条）；100k 外推约 16s。对 `--limit 20` 这种首屏用法，下推不是优化而是可用性；
- 同时把契约说清楚：**后端可能不是增量的**；kiln 侧内存仍是 O(1)，首行时间由后端决定。

### 3. jj 实现

```go
const streamTemplate = `json(self) ++ "\n"` // 记录必须以换行结尾

func (r ExecRunner) LogEach(limit int, fn func(vcs.Commit) bool) error {
	args := []string{"log", "--no-graph", "-r", revset, "-T", streamTemplate}
	if limit > 0 {
		args = append(args, "-n", strconv.Itoa(limit))
	}
	// …StdoutPipe + stderr buffer + Start…
	stopped, err := streamRecords(br, fn)
	// 早停：Kill + 关 stdout + Wait（吞错，与 git 同）
}
```

`streamRecords` 是纯函数，便于单测（jj 侧现有测试也都不 spawn 真 jj）：

- `ReadBytes('\n')` 取一行；空行跳过；
- `json.Unmarshal` 到 `{commit_id, description, author.timestamp}`；只取这三个字段，**忽略其它键**（`parents`/`change_id` 变不变都不影响）；
- `description` `TrimRight("\n")`，与 `parseLog` 对同一条提交给出的 message 一致；
- 时间用 `time.Parse(time.RFC3339, author.timestamp)`（`json(self)` 给的就是 RFC3339；模板里裸写 `author.timestamp()` 是 `2026-10-28 22:47:04.000 +08:00`，不能直接用）；
- 坏行 → 报错（与 git 半条记录同样处理），不静默跳过。

为什么用 `json(self)` 而不是自定义分隔符：现有 `@@KILN@@` 是“正文里几乎不会出现”的赌注，JSON 把正文换行转义掉，一条记录一行是硬保证；代价是记录变胖（约 400B/条），但 jj 本来就要把这些字节渲染出来。

### 4. 顺序：用 jj 的，不排序

- `jj log` 是拓扑序（“children before parents”），revsets 没有 `sort()`；要 author-date 只能缓存全部行再排序；
- 而 jj 的行本来就“一次到齐”（首行≈全量），排序不会再多付延迟，只会多付内存（100k × ~100B ≈ 10MB 级，且要持有全部行）；
- 取舍：**保留拓扑序 + 下推 `-n`**。线性历史下拓扑序就是 newest-first；合并历史下顺序不严格按日期，但每行都带 `date`，要严格序的消费方可自行排序；
- 文档必须改成“git：author-date 倒序；jj：jj 的拓扑序”，不能再写“一律按 author-date 倒序”。

### 5. 交付表现（jj vs git）

| 项 | git | jj |
| --- | --- | --- |
| 首行 | 第一批提交到达即出（64ms/359ms@100k） | 等 jj 扫描+渲染完成（300 提交 116ms；100k 未测） |
| 内存 | O(1) | O(1)（行在 jj 侧，不在 kiln 侧） |
| `--limit` | 早停杀 git | 下推 `-n`，并在够数时同样杀进程 |
| 顺序 | author-date 倒序 | 拓扑序（子先于父） |
| 空仓 | 探 HEAD → 空流 | 0 行，天然空流 |
| 破管道 | 静默 0（Windows） | 同（关管道 + Kill + WaitDelay） |

### 6. 测试

1. `streamRecords`：正常两行（含转义换行的 description）、`description` 尾换行被 trim、坏 JSON 报错、空行跳过、`fn` 返回 false 后不再读；
2. 日期解析：`2026-10-28T22:18:36+08:00` → 原样 offset；
3. CLI：现有 `streamRunner` 加 limit 形参后，`--limit N` 断言后端收到的 limit 是 N；`--limit` 缺省为 0；
4. 真 jj 冒烟（apply 阶段手工，不进单测，与现有 jj 测试不 spawn 的约定一致）：临时 jj 仓 2 条动作 → `action log` 2 行、`--jsonl` 逐行可解析、改写后旧动作不出现、空仓 exit 0。

### 7. 风险与证伪

- **100k 的 jj 渲染**：16s 是外推，不是实测。falsifier：真仓 100k 全量超过约 5s 且 `--limit` 下推也救不回来 → 回退为“jj 仍不支持”，因为首屏体验不成立。
- **`json(self)` 的格式**：只依赖三个键；若 jj 未来改键名，解析会失败并报错（不会静默错行），届时改字段映射即可。
- **顺序差异**：消费方若默认“行序=日期序”，在合并历史上会错。falsifier：出现按行序取“最近 N 条”的用法 → 加缓冲排序，同时把 `-n` 下推去掉。
- **`-n` 与 `--limit` 的语义**：`-n` 限制的是 jj 输出的提交数，kiln 仍按“动作数”停止；当跳过非动作提交时，`-n N` 可能不够 N 条动作 → 不能只靠 `-n` 收敛输出，必须保留 `fn` 早停（否则 `--limit 3` 可能只出 1 行）。这是实现时最容易写错的一处。
- **jj 的 shim**：`jj.exe` 是 Rust 单体，没有 git 那种 shim 问题；但仍按 git 的收尾方式（Kill + 关管道 + WaitDelay）处理，避免第二个平台差异。

### 8. 下一步

- 无异议 → `conclude`：把 §0 的“承诺改对”写进契约（首行不保证、顺序按后端、limit 是提示），然后 `apply`。
- 有异议集中在两处：jj 的顺序要不要靠缓冲换取 author-date；`LogEach` 加 limit 参数值不值得（也可以不改接口，接受 100k 全量渲染）。

## conclude（2026-10-28，第四轮）：jj 支持定稿——原生读取、非增量、拓扑序

### 0. 裁决

| 项 | 决定 | 理由 |
| --- | --- | --- |
| jj 支持 | **实现**，原生 `jj log -T 'json(self) ++ "\n"'` | 语义正确（隐藏被改写提交、覆盖所有 head、空仓免费）；不读 git |
| 不读 git 的原因 | 拒绝 B/C | `--all` 有 keep-ref 幽灵、HEAD 链漏 head、colocate 可关、store 是内部路径 |
| 接口 | `LogEach(limit int, fn func(Commit) bool) error` | jj 需要 `-n` 下推；limit 只是提示，停止仍由 `fn` 决定 |
| 首行承诺 | **只对 git**；jj 首行 = jj 扫描+渲染完成 | 实测 200 提交首行 141ms ≈ 全量 152ms |
| 顺序 | git author-date 倒序；jj 拓扑序（子先于父） | jj 无 `sort()`，排序要缓存全部行；`date` 字段可让消费方自行排 |
| 空仓 | jj 天然 0 行 exit 0 | 无需 HEAD 探测 |
| 破管道 / 早停 | 与 git 同一收尾：Kill + 关管道 + `WaitDelay` | 平台差异只该有一处，不该有第二处 |
| 其余契约 | 不变：只认 `issue:`+`action:`、`title` 兜底、`domain`/`tags` per-commit、退出码 0/1/2 | 同一条命令，字段与语义不按后端分叉 |

### 1. 契约修订（apply 时必须写进文档）

1. **顺序按后端**：git 是 author-date 倒序；jj 是 jj 的拓扑序（“children before parents”，线性历史下即 newest-first）。每行都带 `date`，需要严格序的消费方自行排序。
2. **首行按后端**：git 在第一批提交到达时就出首行；jj 等它扫描+渲染完才一次到齐。`--limit N` 两边都只输出 N 条动作，但 jj 的下推是 `-n`（省渲染，不省扫描）。
3. **其余不变**：只留 `issue:` + `action:` 的提交；`title` = trailer → subject 兜底；`domain`/`tags` 只给本条提交；退出码 0（正常/空/limit/破管道）、1（读失败）、2（参数）；Unix 上 141 是平台惯例。

`machine-readable-output` 的 NDJSON 小节加一段“jj 后端”的说明；README 双语把“jj 暂不支持”换成实际行为。

### 2. 代码改动点

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | `LogEach` 签名加 `limit int`；文档写明“提示、可非增量、内存 O(1) 由 kiln 保证” |
| `internal/vcs/git/git.go` | 签名同步（忽略 limit，仍靠 `fn` 早停）；注释说明为什么不需要 limit |
| `internal/vcs/jj/jj.go` | 实现 `LogEach`；`streamRecords` 纯函数；`streamTemplate = json(self) ++ "\n"` |
| `internal/cli/cli.go` | 调用传 `limit`；删除“暂不支持 jj”分支与文案 |
| `internal/i18n/catalog.go` | 删除已无引用的“action log 暂不支持 jj…”条目 |
| `internal/cli/cli_test.go` | `streamRunner` 签名 + 断言后端收到的 limit；`--limit` 缺省为 0 |
| `internal/vcs/git/git_test.go` | 调用加 `0` |
| `internal/vcs/jj/jj_test.go` | `streamRecords` 用例 |
| `README.md` / `README.en.md` / `docs/machine-readable-output.md` | 顺序与首行的按后端说明 |

### 3. 验收

- `streamRecords`：正常两行、`description` 尾换行被 trim、正文含换行仍是一行、坏 JSON 报错、空行跳过、`fn` 返回 false 后不再读；
- 时间：`2026-10-28T22:18:36+08:00` 解析后保留 offset；
- CLI：`--limit 3` 时后端收到的 limit 是 3，缺省是 0；
- 真 jj 冒烟（手工，不进单测）：2 条动作 → `action log` 2 行；`--jsonl` 每行可解析；改写后旧动作不出现；空仓 exit 0；`--limit 1` 一行；
- git 回归：`go test ./...`、真仓行数 = 动作数、`--limit 3` 仍在 ~120ms 量级、破管道静默 0；
- 文档不再出现“jj 暂不支持”。

### 4. 非目标与遗留

- 不做缓冲排序（jj 的 author-date 需求出现时再加，并去掉 `-n` 下推）；
- 不做 100k jj 基准（外推 16s 仅作风险记录；apply 时若顺手可测一次）；
- 不动 `path` / `list` / `show`；
- 不处理 `jj` 快照工作副本可能带来的额外延迟（`--ignore-working-copy` 不在本 issue 范围）。

### 5. 下一步

`apply`：`vcs` 签名 → git/jj 实现 → cli 调用 → 测试 → 文档（machine-readable-output、README 双语、i18n 清理）。

## apply（2026-10-28，第五轮）：jj 支持落地；`limit` 提示在实现时被证伪并撤掉

### 1. 改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/vcs/vcs.go` | `LogEach(fn)`；文档写明“故意没有 limit 参数”与“后端可以非增量” |
| `internal/vcs/git/git.go` | 不变（本来就用 `fn` 早停） |
| `internal/vcs/jj/jj.go` | 实现 `LogEach`：`jj log --no-graph -r revset -T 'json(self) ++ "\n"'`；`streamRecords` 纯函数；早停 Kill + 关管道 + `WaitDelay` |
| `internal/cli/cli.go` | 去掉 jj 专用分支，保留“后端不支持流式”的兜底 |
| `internal/i18n/catalog.go` | 删“暂不支持 jj”，加“当前后端不支持 action log 的流式读取” |
| `internal/vcs/jj/jj_test.go` | 4 个 `streamRecords` 用例（逐行解析 / 早停 / 坏行 / 空行） |
| `internal/cli/cli_test.go` | 新增 `TestActionLogLimitCountsActionsNotCommits` |
| `README.md` / `README.en.md` / `docs/machine-readable-output.md` | 顺序与首行按后端；`--limit` 不下推 |

### 2. 撤掉 conclude 的一个设计：`LogEach(limit, fn)` 不成立

conclude 定了 limit 提示、并让 jj 拼 `-n`，理由是 100k 时渲染约 16s。实现时发现这是**错的**：

- `jj log -n N` 数的是**提交**，不是 kiln 动作；
- `--limit 3` 若前 3 条提交里只有 1 条带 trailer，jj 输出 3 条提交就结束，kiln 只打印 1 行、读到 EOF、exit 0 —— **少给行且不报错**；
- 没有任何后端能安全使用这个提示：只有调用方知道哪条提交是动作，后端拿不到这个知识。

于是参数被删掉，接口回到 `LogEach(fn)`；`vcs.LogEach` 与 `jj.LogEach` 的注释都写明了“为什么没有 limit”。诚实记录的代价：**jj 上 `--limit` 不再省扫描/渲染，只省读取与输出**，100k 的渲染风险仍在（16s 依旧是外推，见遗留）。

### 3. jj 实现的要点

- 记录框架：`json(self) ++ "\n"` —— 换行是必需的，否则 jj 把所有对象拼成一行（实测 200 提交 1 行）；
- 只取 `commit_id` / `description` / `author.timestamp` 三个键，其它键（`parents` / `change_id` / `bookmarks`）忽略；
- `description` 去尾换行，与 `Log()` 对同一条提交给出的 message 一致；
- 坏 JSON、坏时间戳都报错，不静默跳过（日期列变成零时间比失败更难查）；
- 空行跳过；早停：Kill + 关 stdout + `WaitDelay`，与 git 同一套收尾。

### 4. 验证

- `gofmt -l` 干净、`go vet ./...` 无输出、`go test ./...` 全过；
- 真 jj 仓（colocated，中间夹一条无 trailer 的提交）：`action log` 2 行、顺序 newest-first；`--jsonl` 两行合法，`date` 带 `+08:00`；`--limit 2` 仍给 2 条动作（证明没有 `-n` 少给）；改写最新动作后旧提交消失、只剩 EDITED；空仓 exit 0 + 一行提示；
- git 仓回归：行数 = `status --json` 的动作数，`--limit 3` 仍在 ~120ms 量级，破管道静默 0。

### 5. 遗留

- **jj 100k 渲染未测**（16s 为外推）。若要解决，正确方向不是 `-n`（会少给行），而是让调用方先给候选集、或把“哪条提交是动作”下沉给后端——两者都超出本 issue；
- jj 顺序是拓扑序；要 author-date 时加缓冲排序（并接受 O(n) 内存）；
- jj 会先快照工作副本，脏工作区可能拖慢首行；`--ignore-working-copy` 未采用（会改变语义），另议。

## conclude（2026-10-28，第六轮）：下游接入定稿——`--jsonl` 是唯一机器入口

> 这一节是写给消费者（插件 / agent）的，不是写给实现者的。文本视图只给人看，**不要解析**；机器入口只有 `--jsonl`。

### 1. 记录形状

```json
{"issue":"action-log","action":"apply","commit":"7b808426a5a93ba5b4c5c374f87ea71b934352a2","date":"2026-10-28T23:06:28+08:00","title":"apply 第五轮：文档按后端写清顺序与首行，--limit 不下推"}
```

| 字段 | 恒有 | 类型 | 含义 |
| --- | --- | --- | --- |
| `issue` | 是 | string | `issue:` trailer 的**原值**，未 normalize（老提交可能是 `Foo_Bar`） |
| `action` | 是 | string | `action:` trailer，已小写 |
| `commit` | 是 | string | 全长 40 位 hash（不是 7 位短 hash） |
| `date` | 是 | string | RFC3339，保留提交自己的 offset（`+08:00` / `Z`） |
| `title` | 否 | string | `title:` trailer；缺省时是提交首行（与 `list --json` 的 `title` 同规则） |
| `domain` | 否 | string | **本条提交**声明的分类，不是 issue 级 |
| `tags` | 否 | string[] | **本条提交**声明的标签，不是 issue 级并集 |

缺省规则：**键直接省略**，不会出现 `null`、`""` 或 `[]`。消费方必须按“缺省即没有”处理，不能假设键存在。

### 2. 调用契约

- 只输出同时带 `issue:` 与 `action:` 的提交；**不按当前 action 词表过滤**（词表外的历史动作也会出现）；
- 顺序按后端：git 是 author-date 倒序；jj 是 jj 的拓扑序（线性历史下即 newest-first）；
- `--limit N` 数的是**动作**，不是提交（中间夹无 trailer 的提交不占配额）；
- 退出码：`0` 正常（含空结果、`--limit` 用尽、Windows 上消费者提前关管道）；`1` 读历史失败；`2` 参数错误；
- 空仓 / 无动作：`exit 0`、stdout 为空、stderr 一行提示——**stdout 永远只有记录**；
- `--json` 会被拒绝（exit 2）并提示用 `--jsonl`，不会静默降级；
- Unix 上若消费方提前关闭管道，进程可能以 `141`（SIGPIPE）结束；这是平台惯例，按“我主动关的”处理即可。

### 3. 两个接值班点

1. **没有终止记录**：NDJSON 没有 `summary` 收尾行，EOF 就是结束。判断“读完”靠退出码与行数，不要假设最后一行特殊。
2. **没有增量游标**：目前没有 `--since` / `--after`。只想看新动作时，读到已知 commit 就自己停（配 `--limit` 给上界），或按 `date` 过滤。

### 4. 最小接入示例（Node）

```js
const { spawn } = require('node:child_process');

export function tailActions(repoDir, limit, onAction, onDone) {
  const p = spawn('kiln', ['action', 'log', '--jsonl', '--limit', String(limit)], { cwd: repoDir });
  let buf = '';
  p.stdout.setEncoding('utf8');
  p.stdout.on('data', (b) => {
    buf += b;
    for (let i; (i = buf.indexOf('\n')) >= 0; ) {
      const line = buf.slice(0, i).trim();
      buf = buf.slice(i + 1);
      if (line) onAction(JSON.parse(line));
    }
  });
  p.on('close', (code) => onDone(code, buf.trim())); // 0 = 正常，含空结果
}
```

要点：按行切、不按字节切（一条记录可能跨 chunk）；`JSON.parse` 失败说明契约被破坏，不要吞掉；`--limit` 传数字字符串。`kiln` 不在 PATH 时写绝对路径（Windows 上是 `kiln.exe`）。

### 5. 最小接入示例（PowerShell）

```powershell
.\kiln.exe action log --jsonl --limit 5 |
  ForEach-Object { $_ | ConvertFrom-Json } |
  Select-Object issue, action, date, title
```

`ConvertFrom-Json` 会把 `date` 转成 `DateTime`（显示成本地时间）；需要原始 RFC3339 字符串就直接读 `$_` 文本再解析。

### 6. 不在接入面里的东西

`--json`、`--all`、`--domain/--tag/--action` 过滤、`--since/--after`、`--stat`、终止记录、`action show/search` —— 都要另开 issue，不要在当前契约上假设。

### 7. 接入验收（本仓实测）

- `action log --jsonl --limit 3` 三行，逐行 `ConvertFrom-Json` 成功；`commit` 全长、`date` 带 `+08:00`、未声明的 `domain`/`tags` 不出现；
- 空结果时 stdout 为空、exit 0、提示只在 stderr；
- `--limit 0` exit 2；
- jj 仓同样可用（顺序为拓扑序，首行等 jj 扫完）。

## apply（2026-10-29）：`--jsonl` 补齐 T1 字段（关系等），文本视图不动

按 `action-log-relations` 的 hypothesize（`00f4ec5`）落了第一批：**只加 T1**，即"提交里已经写着、不需要任何计算"的字段。文本模式一行没改。

### 新增字段（追加在 `tags` 之后，已有键位置不动）

| key | 类型 | 来源 |
| --- | --- | --- |
| `refs` | `[]string` | `refs:` trailer，按逗号拆 |
| `target` | `[]string` | `target:` trailer，按逗号拆（与 `refs`/`tags` 同规则） |
| `scope` | `string` | `scope:` trailer，原样 |
| `relations` | `[{name,target}]` | 关系 trailer 的**声明**，规范名 |
| `unrelate` | `[]string` | `unrelate:` trailer 的靶 |

### 三个实现细节

1. **声明，不是折叠**。`has-part: X` 输出 `{name:"has-part", target:"X"}`，不会翻成 `from=X, to=me, name=part-of`。翻转、取强、合并都是 `graph` / `path` 的事；在流里做等于把派生逻辑塞进"这条动作说了什么"。
2. **顺序确定**：`relations` 按词表声明序输出（`blocks` 在 `caused-by` 前），同一 name 下按书写序，重复的 `(name,target)` 去重。`refs` / `target` / `unrelate` 同样去重保序。同一份仓永远同样的字节。
3. **词表外的键不出现**：`trailer.Parse` 只认词表与别名，未知名归 `check` 的 `unknown-trailer`。别名会解析成规范名（`derived-from` → `caused-by`）。

### 改动面

| 文件 | 改动 |
| --- | --- |
| `internal/trailer/trailer.go` | `splitList` 导出为 `SplitList`（`target` 是 `Fields` 里唯一保持标量的列表类 trailer，读侧复用同一条拆分规则） |
| `internal/cli/cli.go` | `actionLogDoc` 加 5 个键；`actionLogRelations` 负责词表序 + 去重；`emit` 里填字段 |
| `internal/cli/cli_test.go` | `TestActionLogJSONLCarriesEveryStoredTrailer`：全字段、别名解析、去重、词表序、未知键不泄漏、空记录不出现新键 |
| `docs/machine-readable-output.md` | 字段表与 `relations` 语义 |

### 验收

- `gofmt -l` 无输出；`go vet` / `go test ./...` 全过；
- `action log --jsonl` 只多键，已有键的值不变；文本模式字节不变；
- 本仓冒烟：99 行关系 / 147 条声明 / 87 行带关系；把全部声明按 `FoldEdges` 规则重算，覆盖 `graph --json` 的 120 条边（每条边都能在某条动作的声明里找到对侧或本侧的原文）；
- 空记录（只声明了 `issue`/`action`/`title`）不出现 `refs`/`target`/`scope`/`relations`/`unrelate` 任何一个键。

### 不做：`files`（T2，改动文件 + 增删行数）

`action log` 保持"只吐存储层的字节"。`files` 不是提交里写着的字段，而是拿提交树和父树 diff 出来的**派生数据**，所以不进流：

- 下游要 join 时已经有 `commit` 全 hash，一条 `git show --numstat <hash>` 就能拼；
- 逐动作要文件列表，`kiln show <issue> <n> --stat`（单条）或 `path --all --stat`（批量，283 动作 0.31 s）都现成；
- 它是唯一需要额外计算（每条提交一次 tree diff）的字段，默认路径不该付这个钱；
- 两后端还不对称：git 给机器友好的三元组，jj 只有 `diff.summary()` 的文本。

形状已经设计好（`{path,add,del}` / `{binary:true}` / `{path,from}`），真有消费方时再上——那时做成 opt-in `--files`，默认路径一行都不变。