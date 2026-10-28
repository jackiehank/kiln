# issue: search-result-contract — 搜索行要能被 UI 直接渲染（rank + hit 的 total/date）

- **domain**：cli
- **关联**：relates-to `issue-list-scaling`（`list --search` 在这里落地）；relates-to `json-commit-hash`（`hit` 对象形态）；relates-to `machine-readable-output`（JSON 契约）

## 背景

`kiln list --search` 已经能按 id / 当前标题 / 动作 subject 做字面量 AND 搜索，`--json` 也给 `hit` 对象。下游 `dsh-kiln` 插件正在把搜索从「浏览器内存匹配」换成「调 `kiln list --search --json` 后只渲染」（B 方案；220ms 的端到端延迟已实测可接受）。

渲染需要三样数据，现在没有：

1. **相关度顺序**：插件现有排序是 id 首词前缀 > id 命中 > 标题命中 > 单条动作命中 > 词分散，再按最近动作；kiln 现在固定 id 字典序。UI 若自己再排一次，就又有两套算法。
2. **`hit.total`**：结果行要显示「命中第 n/m 条」；`hit` 只有 ordinal，没有 m。
3. **`hit.date`**：结果预览要显示命中动作的日期；现在只能再查 `path`。

本 issue 只补这三样与它们的顺序契约；不改匹配语义，也不做 UI 的高亮/交互。

## 需求

1. **rank**：`list --search ... --json` 的每个 `issues[]` 行必须有整数 `rank`（0–4），且 `issues[]` 按 rank 排序。不带 `--search` 的 `list` 不输出 `rank`（缺省即省略），顺序仍按 id。
2. **rank 的定义**（与 dsh-kiln 现有搜索一致；词序按查询串）：
   - `0`：issue id（不区分大小写）以**第一个词**开头；
   - `1`：issue id 含**全部**词；
   - `2`：当前标题（`issues[].title` 的来源）含全部词；
   - `3`：至少一条动作 subject 含全部词；
   - `4`：整体命中，但以上都不满足（词分散在多个字段/多条动作）。
   0 优先于 1；桶内再按 tie-break。
3. **tie-break**：同 rank 按末条动作日期降序；日期相同或缺失时按 issue id 升序。日期按时间比较，不按字符串字典序。
4. **hit 元数据**：`hit` 继续只在「id + 标题解释不了命中，且某一条动作含全部词」时出现；对象保持 `{ordinal, action, commit, subject}`，新增：
   - `total`：该 issue 的动作总数（整数，≥1）；
   - `date`：命中动作的日期，格式与 `path --json` 的 `actions[].date` 一致。
5. **文本与 JSON 同序**：`list --search` 的文本视图按同样的 rank/tie-break 排序；除顺序外文本布局不变（仍带 `← subject`）。
6. **不变的部分**：匹配字段/分词/AND/字面量/大小写、`hit` 的触发规则、全仓范围、与 `--domain/--tag/--action/--status` 的 AND 叠加、空结果 exit 0 + stderr、空查询 exit 2、`count`/`filter` 回显、非搜索的 `list`/`status`/`graph`/`path`/`show` 输出。
7. **文档**：`docs/machine-readable-output.md` 的 `list` 节与 README 的 `--search` 说明补 rank、顺序与 hit 新字段；说明「只有 `--search` 改变顺序，普通 list 仍按 id」。

## 测试

只列必须覆盖的用例，具体测试写法由实现方决定：

- **rank 分档**：一个 fixture 覆盖 0/1/2/3/4 各至少一条；断言每行 `rank` 与 `issues[]` 顺序。
- **tie-break**：同 rank 的两条（日期不同；日期相同）→ 日期降序、再 id 升序；日期跨时区仍按时间。
- **hit**：命中早期动作时 `ordinal/action/commit/subject/total/date` 全对；`ordinal` 从 1 起；`total == 该 issue 动作数`。
- **hit 抑制**：id+标题已含全部词 → 无 `hit`；词分散在多条动作 → 无 `hit`（保持现状）。
- **文本/JSON 同序**：同一 query 抓文本行序与 JSON `issues[]`，逐条相等。
- **非搜索不变**：`list --json`（无 `--search`）与改前逐字节相同；`list` 文本逐行相同；`rank` 不出现。
- **组合**：`--search` + `--domain/--tag/--action/--status`，断言在过滤后的集合内按 rank 排序。
- **边界**：大小写、多词 AND、字面量、超长 subject（>200 字符）、无结果、空查询。
- **真仓冒烟**：在本仓跑一个已知多档的 query，记录 JSON 与文本顺序；再跑一个 `list --json` 对拍。

## 验收

- `kiln list --search <q> --json`：每行有 `rank`，`hit` 有 `total`/`date`；`issues[]` 按契约排序。
- `kiln list --search <q>`：文本顺序与 JSON 相同，`← subject` 保留。
- `kiln list --json`（无 search）输出与现状逐字节相同；`list` 文本相同。
- `gofmt` / `go vet` / `go test ./...` 全过；新增测试覆盖上面的用例。
- README 与 `docs/machine-readable-output.md` 同步。
- 真仓 dogfood：搜索排序符合 rank 契约；非搜索输出无 diff。

## 非目标

- 不改 `--search` 的匹配语义与 `hit` 触发规则；
- 不输出高亮区间、不实现 UI 的 filter/search 切换；
- 不加分页 / `--limit` / `--offset`；
- 不新增独立 `search` 命令；
- 不改 `status` / `graph` / `path` / `show` 的输出。

## 备注

- 需求来源：下游 `dsh-kiln` 插件的搜索接入（B 方案）。插件只保留高亮与交互，匹配/顺序/hit 全部以 kiln 为准；本 issue 是「一套算法」的前置。
- `issue-list-scaling` 是 `list --search` 的落地 issue；`json-commit-hash` 定了 `hit` 的对象形态；本 issue 只增量补 UI 渲染需要的元数据与顺序。
- 排序变化只发生在 `--search`；普通 `list` 的 id 序不变，避免影响现有 CI/golden。

## hypothesize（2026-10-28）：一次行构造同时给出 rank、顺序与 hit 元数据

> 结论先行：三条诉求都能在 `internal/cli` 的 `list` 内闭环，**不改 engine、模型、匹配语义与非搜索输出**。`rank` 只在 `--search` 时以 `*int` 出现（0 是合法值，`omitempty` 会吞掉 `int` 的 0）；`searchRank` 是纯函数，按需求 2 的 0→4 顺序判定；过滤后构造一次 `searchRow{p, rank, hit}`，按 rank → 末条动作日期降序 → id 升序排序，文本与 JSON 遍历同一个切片，顺序不可能分叉；`hit` 只补 `total`/`date`，触发规则不动。

### 1. 主张与证伪条件

**主张**：`model.Issue` 已经带齐排序所需的一切——`Actions` 在 `engine.Build` 里按 `Date` 升序稳定排序（`engine.go:73-77`），所以 `Last()` 就是末条动作、`len(Actions)` 就是 `hit.total`；`issueTitle` 就是“当前标题”；`Action.Date.Format(time.RFC3339)` 就是 `path --json` 已在用的日期格式（`cli.go:1784`）。因此不需要新增后端调用，也不需要 engine 暴露“命中位置”。

**证伪条件**（任一成立，本方案就要改）：

1. `rank` 需要知道词命中在哪个字段，而 `issueMatches` 的 AND 语义给不出——不成立：`issueMatches` 本来就分别拥有 id / title / subject 三段（`cli.go:2241-2251`），rank 按段重判即可，每个 issue 的代价是几次 `containsAll`，仍是 O(文本长度)。
2. 文本与 JSON 的顺序无法保证一致——不成立：两者改为遍历同一个 `[]searchRow`。
3. 非搜索输出会变——不成立：`rank` 用 `*int` + `omitempty`，不搜索时为 nil（键整个省略）；现有逐字节 golden `TestListJSONSharesTheIssueRow`（`cli_test.go:2048-2072`）原样通过就是证据。

### 2. 现状核算（读到的代码事实）

| 事实 | 位置 | 对方案的含义 |
| --- | --- | --- |
| ids 先 `engine.Filter`（id 升序），再 `--status`，再 `--search`，全程不重排 | `cli.go:2372-2378` | 搜索排序必须显式加在 `filterSearch` 之后；默认路径保持原循环 |
| 匹配 = id + 当前标题 + 所有动作 subject 的 AND | `cli.go:2241-2251` | rank 0–4 的字段来源已经齐了 |
| `searchHit` 独立再匹配一遍，取最早一条全含词的动作 | `cli.go:2253-2268` | 只需扩字段；同时应只算一次 |
| JSON 行与文本行各自调用 `searchHit` | `cli.go:2404`、`cli.go:2414` | 同一行一次查询算两遍；新结构顺带消除 |
| `Actions` 按 `Date` 升序 | `engine.go:73-77` | `Last()` 即末条动作，tie-break 直接读 |
| 日期序列化先例 | `cli.go:1784` | `hit.date` 照抄 `Format(time.RFC3339)`，格式天然与 `path` 一致 |
| DTO 是纯结构 | `cli.go:2853-2869` | 加字段不会波及其它命令 |
| 非搜索 JSON 有逐字节 golden | `cli_test.go:2048-2072` | 回归锚点，改完必须原样通过 |

真仓基线（2026-10-28，当前 64 issue）：`kiln list --search worktree` 12 条，顺序是 id 字典序（`cli-consistency`、`help-relation-flags`、`issue-uniqueness`、…），与 rank 无关。这正是本 issue 要替换的排序。

### 3. 决策

#### D1 `rank` 用 `*int`，只在 `--search` 时赋值

```go
type listIssue struct {
	issueCore
	Rank   *int     `json:"rank,omitempty"`
	Title  string   `json:"title,omitempty"`
	Commit string   `json:"commit,omitempty"`
	Hit    *listHit `json:"hit,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}
```

- **必须是指针**：`omitempty` 对 `int` 会丢掉 0，而 rank 0 恰恰是最高档；`nil` 才是“没搜索”。
- 备选是自定义 `MarshalJSON` 或拆一个 `searchIssue` 行类型——为省一次分配引入两套 DTO，不值。64 行规模下一次分配可忽略。
- 赋值时用循环内局部变量取地址（`r := row.rank; row.Rank = &r`），不要复用同一个 `int`。

#### D2 rank 判定：纯函数，按需求的 0→4 顺序

```go
// searchRank assumes the issue already matched; rank 4 is the fallback, not a
// second filter. 0 only needs the first term to prefix the id (requirement 2),
// so a multi-term query can rank 0 while the other terms come from the title.
func searchRank(p *model.Issue, terms []string) int {
	if strings.HasPrefix(strings.ToLower(p.ID), strings.ToLower(terms[0])) {
		return 0
	}
	if containsAll(p.ID, terms) {
		return 1
	}
	if containsAll(issueTitle(p), terms) {
		return 2
	}
	for _, a := range p.Actions {
		if containsAll(a.Subject, terms) {
			return 3
		}
	}
	return 4
}
```

- 判定顺序即优先级（`if/return` 链），0 先于 1；不要先算“全词在 id”再回头判前缀。
- rank 0 **不要求其余词也在 id 里**——需求原文只约束“以第一个词开头”。这是刻意保留的“id 首词前缀”桶，用测试钉住，避免实现者顺手加条件。
- 不引入词边界：`a` 命中 `action` 是既有“字面量包含”约定的一部分。
- `terms` 非空才调用（`list` 已在空查询时 exit 2），rank 4 是兜底，不会把未匹配的 issue 带进来。

#### D3 排序：rank → 末条动作日期降序 → id

```go
type searchRow struct {
	p    *model.Issue
	rank int
	hit  *listHit
}

func lastDate(p *model.Issue) time.Time {
	if a, ok := p.Last(); ok {
		return a.Date
	}
	return time.Time{}
}

sort.Slice(rows, func(i, j int) bool {
	a, b := rows[i], rows[j]
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	ad, bd := lastDate(a.p), lastDate(b.p)
	switch {
	case ad.IsZero() != bd.IsZero():
		return bd.IsZero() // 有日期的一律在前；零值视为最旧
	case !ad.Equal(bd):
		return ad.After(bd) // 按时间，不按字符串
	default:
		return a.p.ID < b.p.ID
	}
})
```

- 日期比较用 `time.Time`，不用 `Format` 后的字符串。
- 零值日期：git 后端每条 commit 都解析 `%aI`（`git.go:27`、`git.go:478`），零值只在解析失败时出现。需求里的“缺失”按“两边都缺失”理解；一边缺失、一边有值时视为缺失方最旧——否则“任一缺失就按 id”会让比较器不满足传递性，`sort.Slice` 的结果不可预测。真实仓库里这条分支不可达，测试可钉一个零值行排最后。
- 比较器以唯一 id 收尾，是全域全序，`sort.Slice` 即可，不需要 `SliceStable`。

#### D4 过滤后构造一次 rows，两个视图共用

```go
if len(terms) > 0 {
	rows := make([]searchRow, 0, len(ids))
	for _, id := range ids {
		p := repo.Issues[id]
		rows = append(rows, searchRow{p: p, rank: searchRank(p, terms), hit: searchHit(p, terms)})
	}
	sort.Slice(rows, lessSearchRow)
	// JSON: 用 rows 填 doc.Issues；文本: 用 rows 打印
	// 两处都用 row.hit，不再各调一次 searchHit
	return ...
}
// 非搜索的 JSON 块与文本循环原样保留
```

- 搜索分支独立返回，非搜索路径的代码**一行不挪**——这是“逐字节不变”最便宜的保证。
- `count` 仍等于 `len(ids)`（= `len(rows)`），`filter` 回显逻辑不变。
- 文本的行格式不变，只是遍历顺序变了；`← subject` 仍来自 `row.hit`。

#### D5 `hit` 只补 `total`/`date`

```go
type listHit struct {
	Ordinal int    `json:"ordinal"`
	Action  string `json:"action"`
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	Total   int    `json:"total"`
	Date    string `json:"date"`
}
```

- `searchHit` 命中时填 `Total: len(p.Actions)`、`Date: a.Date.Format(time.RFC3339)`；`Ordinal` 仍从 1 起。
- 触发规则完全不动：id+标题能解释 → `nil`；词分散、没有单条动作全含 → `nil`。因此 rank 4 必然无 `hit`，rank 3 必然有（除非 id+标题已经解释，但那时 rank 会 ≤2）。
- 新字段追加在末尾；JSON 键序只在搜索行变化，对象顺序不是契约，现有解码器不受影响。

#### D6 文档同步

- `README.md:109` 的 `--search` 单元格补：结果按 rank 0–4 排序、同档按末条动作日期降序再按 id、`--json` 搜索行带 `rank`、`hit` 带 `total`/`date`。
- `docs/machine-readable-output.md` §3（`kiln list --json`，约 126–155 行）：示例行加 `rank`，`hit` 示例加 `total`/`date`，字段表补三行，搜索要点补排序契约；§7 第 276 行“list 按 ID”改为“默认按 ID；`--search` 按 rank”。
- `-h` 与 `internal/i18n/catalog.go:236` **不动**：需求 7 只点名 README 与机器可读文档；改 help 会牵动 catalog 与 help 测试，收益低。若以后要在 `-h` 露出排序，另开 issue。

#### D7 明确不做

- 不改 `engine.Filter` 的签名/返回（避免波及 `status` / `graph`）。
- 不加 `--sort` / `--limit` / `--offset`，不做高亮/交互。
- 不改匹配字段、`hit` 触发规则、空查询/无命中的退出码与 stderr。
- 不改非搜索的 `list` / `status` / `path` / `show` 输出。

### 4. 落点与实施顺序

| 步 | 改动 | 验证 |
| --- | --- | --- |
| 0 | 记录基线：`list`、`list --json`、`status --json`、`path --json` 的哈希与 `list --search worktree` 的 12 行顺序 | 改前快照留档 |
| 1 | `listIssue` 加 `Rank *int`；`listHit` 加 `Total`/`Date`；加 `searchRank`、`lastDate` | `go build`；非搜索 golden 仍过 |
| 2 | `list` 的搜索分支建 `rows`、排序，JSON/文本改用 `rows` | 手动跑 rank fixture + 真仓 |
| 3 | 测试：rank 分档、tie-break、hit total/date、文本/JSON 同序、无搜索省略 rank、组合过滤 | `go test ./...` |
| 4 | README 与 `docs/machine-readable-output.md` | 对照需求 7 |
| 5 | 真仓 dogfood 与 `kiln check` | 非搜索输出哈希不变；本 issue 仍 open（apply 前） |

提交建议拆两个 `apply`：代码+测试一个，文档一个；`kiln check` 在两步之间仍应通过。

### 5. 测试计划

**rank fixture**（查询 `cache lru`；日期故意与 rank 反向，证明 rank 优先于“最近动作”）：

| issue | 命中面 | 期望 rank |
| --- | --- | --- |
| `cache-lru` | id 以首词 `cache` 开头 | 0 |
| `lru-cache` | id 含全部词，但非前缀 | 1 |
| `title-only` | 当前标题含全部词 | 2 |
| `action-only` | 早期动作 subject 含全部词 | 3 |
| `spread` | 标题只有 `cache`、早期动作只有 `lru` | 4 |

**tie fixture**（查询 `cache`，三条都是 rank 1）：`a-cache` 日期 T+1、`b-cache` T+2、`c-cache` T+2 → 期望 `b-cache`、`c-cache`、`a-cache`。

| # | 用例 | 断言 |
| --- | --- | --- |
| 1 | `TestListSearchRanksAndOrdersRows` | JSON 的 `rank` 为 0..4，`issues[]` 顺序同上表 |
| 2 | `TestListSearchTieBreaksOnLastDateThenID` | tie fixture 的顺序；日期相同按 id 升序 |
| 3 | `TestListSearchHitCarriesTotalAndDate` | 扩展现有表：`ordinal`/`total=3`/`date=RFC3339`，两次命中分别指 s1、s2 |
| 4 | `TestListSearchRankPrefixOnlyNeedsFirstTerm` | id `cache-x` + 标题 `lru`，查询 `cache lru` → rank 0 |
| 5 | `TestListSearchTextOrderMatchesJSON` | 文本首列 id 序列 == JSON `issues[].id` |
| 6 | `TestListJSONWithoutSearchOmitsRank` | 无搜索时 `Rank == nil`，原始 stdout 无 `"rank"`；golden 逐字节不变 |
| 7 | `TestListTextWithoutSearchKeepsIDOrder` | 无搜索文本仍按 id 升序，且不出现 rank |
| 8 | `TestListSearchRankComposesWithFilters` | `--domain` 过滤后的子集仍按 rank 排序 |
| 9 | 既有用例 | 大小写、多词 AND、分散 hit 抑制、无命中、空查询、`--status` 组合全部原样通过 |

真仓 dogfood：`list --search worktree` 的 12 条应变成 `worktree-mode` 先（rank 0），随后 id 含 `worktree` 的 rank 1；文本与 JSON 顺序逐条相等；非搜索 `list` / `list --json` 与基线哈希相同。

### 6. 验收对照（需求 1–7）

| 需求 | 由什么满足 |
| --- | --- |
| 1 rank 0–4、按 rank 排序、非搜索省略 | D1/D2/D3/D4 + 用例 1/6 |
| 2 rank 定义与优先级 | D2 + 用例 1/4 |
| 3 tie-break 日期降序、再 id | D3 + 用例 2 |
| 4 `hit.total` / `hit.date` | D5 + 用例 3 |
| 5 文本与 JSON 同序、布局不变 | D4 + 用例 5/7 |
| 6 匹配/`hit` 规则/过滤/空结果/非搜索输出不变 | 非搜索分支不挪 + golden + 既有用例 + dogfood |
| 7 README 与 `docs/machine-readable-output.md` | D6 |

### 7. 风险与边界

- **rank 0 的语义**：只要求首词前缀，不要求全词——按需求字面实现；若下游实际要“全词都在 id 且首词前缀”，只改 `searchRank` 第一个分支并改用例 4。
- **指针别名**：循环里必须给每行一个独立的 `int`，否则所有行指向最后一个 rank。
- **零值日期**：见 D3；真实 git 数据不会触发，测试只钉“不 panic、排最后”。
- **JSON 键序**：搜索行多了 `rank`，对象键序变化；契约按字段名，不按字节序，不视为破坏。
- **性能**：每个命中 issue 多几次 `containsAll` + 一次 O(k log k) 排序；无后端调用，64 issue 规模可忽略。
- **i18n**：不新增用户可见文案；错误/提示字串不变。

### 8. 备选与取舍

- **在 engine 里排序**：`Filter` 只负责“留哪些”，搜索词与 rank 属于 CLI 层；改 engine 会波及其它调用方。
- **插件侧排序**：正是本 issue 要消除的两套算法。
- **拆独立 `search` 命令或加 `--sort`**：非目标，且会扩大契约面。
- **自定义 `MarshalJSON` 省指针**：代码更多、更易错；指针方案一眼能看出“缺席”与“0”。

### 9. 待确认（apply 前请 dsh-kiln 侧回复）

下面三点会改变排序/`hit` 的具体行为。**未回复时按默认口径落地**；回复后若与默认不同，只改对应的一处函数或比较器，并同步一个用例。请尽量逐条给出 `Q1=A` 这样的结论；能贴插件现有比较器/排序代码，或一个真实 query 的排序结果，最好。

#### Q1：rank 0 只认“首词前缀”，还是要求“全词都在 id 且首词前缀”？

- **A（本方案默认，按需求 2 字面）**：`id` 以查询的第一个词开头即 rank 0；其余词可以来自标题或动作 subject。
  - 例：query `cache lru`，issue `cache-basics`（id 只有 `cache`）+ 标题 `LRU 策略` → **rank 0**。
- **B（更严格）**：`id` 含全部词，且以第一个词开头，才 rank 0；否则落到 rank 1。
  - 同一例在 B 下 → rank 1（id 不含 `lru`）。
- **为什么要问**：需求原文只写“id 以第一个词开头”，但插件现有排序的“id 首词前缀”桶如果隐含“全词都在 id”，两边会差一档。A 会让“首词命中 id、其余词命中标题”的 issue 拿到最高档，可能比插件现在的排序更激进。
- **请回复**：`Q1=A` 或 `Q1=B`；若选 B，请说明是“全词在 id”还是“全词在 id+标题”。

#### Q2：末条动作日期为零值（日期缺失）时怎么排？

- **A（本方案默认）**：有日期的排在无日期的前面；都无日期时按 id 升序。
- **B（字面“缺失就按 id”）**：只要有一边日期缺失，就按 id 升序。
- **为什么不建议 B**：B 会让比较器不满足传递性（A 缺失、B 有日期、C 有日期时，A-B 按 id、B-C 按日期、A-C 又按 id），`sort.Slice` 的结果依赖初始顺序，同一仓库可能排出不同结果。A 是全序且确定。
- **实际影响**：git 后端每条 commit 都解析 `%aI`（`git.go:27`、`git.go:478`），零值只在日期解析失败时出现，正常仓库不可达，插件侧大概率永远看不到这个分支。
- **请回复**：`Q2=A` 或 `Q2=B`；若选 B，请给出一个必须区分“缺失”的真实数据来源。

#### Q3（低风险，默认沿用）：`hit.date` 用 author date 还是 committer date？

- 本方案用 author date，与 `path --json` 的 `actions[].date` 同源、同格式（`time.RFC3339`）。
- 若插件预览需要的是 committer date，请回复 `Q3=committer`；否则 `Q3=author`（默认）。
- 注意：若选 committer，`path --json` 的日期口径也会与本 issue 不一致，需要另开 issue 统一，不能只改 `hit.date`。

#### 回复模板

```text
Q1=A | B
Q2=A | B
Q3=author | committer
备注：（可选：插件现有排序代码 / 真实 query 的排序结果）
```

收到回复后，本 issue 会加一条 `collect` 动作记录结论；若与默认不同，只改 `searchRank` 的 Q1 分支、排序比较器的 Q2 分支或 `searchHit` 的 Q3 字段，再进 `apply`。

### 10. 其它开放问题（不阻塞）

- `-h` 是否露出排序：本轮不做；触发条件是有用户仅看 `-h` 就误解顺序。
- `rank` 是否也进文本视图：不进；文本只变顺序，布局与列不变。

## 回复（2026-10-28，dsh-kiln 侧）

Q1=A、Q2=A、Q3=author。D1（`*int`）与 D4（一次 `searchRows` 两个视图共用）没意见。

### Q1=A：rank 0 只认首词前缀

插件 `plugin/lib/client.js` 的 `searchIssues`（第 1017–1023 行）：

```js
const idHit = hasAll(id)
const titleHit = hasAll(title)
const actionHit = titleHit
  ? null
  : (actions.find((action) => hasAll(String(action?.title ?? ''))) ?? null)
const rank = id.toLowerCase().startsWith(terms[0]) ? 0 : idHit ? 1 : titleHit ? 2 : actionHit !== null ? 3 : 4
```

`hasAll(text)` 是「全部词都在该字段里」，但 rank 0 只看 `startsWith(terms[0])`，**不要求其余词在 id**——其余词可以来自标题或动作。这正是插件现在对用户的行为，选 A 才能直接对齐；选 B 会让插件为了对齐改排序，又回到两套算法。

本仓真实例子：query `kiln v0.2.0`，issue `kiln-v020-adapter` 的 id 以 `kiln` 开头但不含 `v0.2.0`（含在标题/动作里），插件排 rank 0。kiln 按 A 应同档；按 B 会落到 rank 1。

### Q2=A：有日期在前，零值排最后

插件 `byRecency`（第 849–854 行）：

```js
const leftDate = left.last?.date ?? ''
const rightDate = right.last?.date ?? ''
if (leftDate === rightDate) return left.id.localeCompare(right.id)
return leftDate < rightDate ? 1 : -1
```

缺日期的空串自然排最后；两边都缺按 id。与 A 一致。B 会让比较器不满足传递性，`sort.Slice` 依赖初始顺序，不接受。git 后端正常数据不会出现零值，A 只是契约兜底。

补充：插件现在按 ISO 字符串比较，跨时区会与「按时间」不一致；插件侧会改成按时间比较，kiln 按 D3 的时间比较即可，不需要迁就旧的字符串序。

### Q3=author：author date

插件展示的日期来自 `kiln path --json` 的 `actions[].date`；kiln 的 git 后端用 `%aI`（author date）填它（`internal/vcs/git/git.go:27`：`const logFormat = "%H%x1f%aI%x1f%B%x1e"`）。`hit.date` 必须同源，否则同一行在搜索结果预览与 `path` 里日期不一致。选 author。

### 备注

- 插件侧还有一处会对齐 kiln：`hit` 的抑制规则从「当前标题能解释」改成「id + 标题能解释」——这是插件改，不需要 kiln 动；kiln 保持现状即可。
- `total` 用该 issue 的全部动作数；`date` 用命中动作的 author date，格式 `time.RFC3339`；两者与 `ordinal`/`action`/`commit`/`subject` 放进同一个 `hit` 对象。
- 以上确认后，插件按需求 1–7 接 `list --search --json`；apply 后我们再做宿主路由与渲染，不再保留内存匹配的第二套排序。
