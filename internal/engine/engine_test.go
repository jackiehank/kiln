package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"kiln/internal/model"
	"kiln/internal/vcs"
)

var testRel = model.DefaultRelationSet()

func mkCommit(hash, msg string, day int) vcs.Commit {
	return vcs.Commit{
		ID:      hash,
		Date:    time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC),
		Message: msg,
	}
}

// newest first, mirroring `git log`
func sampleCommits() []vcs.Commit {
	return []vcs.Commit{
		mkCommit("c3", "得出结论\nissue: auth/session-expiry\naction: conclude", 3),
		mkCommit("c2", "验证假设\nissue: auth/session-expiry\naction: test", 2),
		mkCommit("c1", "提出问题\nissue: auth/session-expiry\naction: ask\nobject: q1", 1),
		mkCommit("c0", "fix: 无关提交\n\n普通正文", 0),
	}
}

func TestActionsFromCommitsFiltersAndOrders(t *testing.T) {
	actions := ActionsFromCommits(sampleCommits(), testRel)
	if len(actions) != 3 {
		t.Fatalf("expected 3 kiln actions, got %d", len(actions))
	}
	got := []string{actions[0].Kind, actions[1].Kind, actions[2].Kind}
	want := []string{"ask", "test", "conclude"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chronological order = %v, want %v", got, want)
	}
}

func TestBuildAggregatesIssue(t *testing.T) {
	repo := Build(ActionsFromCommits(sampleCommits(), testRel), testRel)
	if len(repo.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(repo.Issues))
	}
	p := repo.Issues["auth/session-expiry"]
	if p == nil || len(p.Actions) != 3 {
		t.Fatalf("unexpected issue: %+v", p)
	}
	if !model.DefaultActionSet().IsTerminal(p.State()) {
		t.Fatalf("expected terminal state, got %q", p.State())
	}
}

func TestCheckCompleteIssueIsOK(t *testing.T) {
	repo := Build(ActionsFromCommits(sampleCommits(), testRel), testRel)
	diags := Check(repo, model.DefaultActionSet())
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("expected a single OK diagnostic, got %+v", diags)
	}
	if !strings.HasPrefix(diags[0].Message, "完整（ask, test, conclude）") {
		t.Fatalf("unexpected summary: %q", diags[0].Message)
	}
}

func TestCheckRules(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("d3", "没有 ask，也没闭环\nissue: sub/no-ask\naction: test", 4),
		mkCommit("d2", "父不存在\nissue: sub/bad-parent\naction: ask\nderived-from: missing/parent", 3),
		mkCommit("d1", "子不存在\nissue: sub/bad-child\naction: ask\nspawns: missing/child", 2),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	diags := Check(repo, model.DefaultActionSet())

	if !hasDiag(diags, "sub/no-ask", LevelErr, "缺少 ask") {
		t.Fatalf("rule 1 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/no-ask", LevelWarn, "缺少终点动作") {
		t.Fatalf("rule 2 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-parent", LevelWarn, "关系目标不存在") {
		t.Fatalf("rule 3 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-child", LevelWarn, "关系目标不存在") {
		t.Fatalf("rule 4 not reported: %+v", diags)
	}
}

// Actions after the start are an unordered combination: the path below runs
// ask -> test -> collect -> conclude, which is not the "canonical" order, and
// must still count as complete.
func TestCheckDoesNotEnforceOrder(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("h4", "结论\nissue: o/r\naction: conclude", 4),
		mkCommit("h3", "采集\nissue: o/r\naction: collect", 3),
		mkCommit("h2", "验证\nissue: o/r\naction: test", 2),
		mkCommit("h1", "提问\nissue: o/r\naction: ask", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	diags := Check(repo, model.DefaultActionSet())
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("order must not be enforced, got %+v", diags)
	}
}

func TestCheckHonoursCustomVocabulary(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("g2", "收尾\nissue: x/y\naction: done", 2),
		mkCommit("g1", "开始\nissue: x/y\naction: begin", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	set, err := model.NewActionSet([]string{"begin", "probe", "done"}, "begin", []string{"done"})
	if err != nil {
		t.Fatal(err)
	}
	diags := Check(repo, set)
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("custom vocabulary not honoured, got %+v", diags)
	}
}

func TestFilter(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("f1", "问题一\nissue: auth/token\naction: conclude\ndomain: auth\ntags: [security]", 3),
		mkCommit("f2", "问题二\nissue: perf/slow\naction: ask\ndomain: perf\ntags: [performance]", 2),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)

	if got := Filter(repo, FilterOptions{Domain: "auth"}); !reflect.DeepEqual(got, []string{"auth/token"}) {
		t.Fatalf("domain filter = %v", got)
	}
	if got := Filter(repo, FilterOptions{Tag: "performance"}); !reflect.DeepEqual(got, []string{"perf/slow"}) {
		t.Fatalf("tag filter = %v", got)
	}
	if got := Filter(repo, FilterOptions{Action: "ask"}); !reflect.DeepEqual(got, []string{"perf/slow"}) {
		t.Fatalf("action filter = %v", got)
	}
}

func TestSummarize(t *testing.T) {
	commitList := append(sampleCommits(),
		mkCommit("x1", "进行中\nissue: perf/ongoing\naction: ask", 5))
	repo := Build(ActionsFromCommits(commitList, testRel), testRel)
	s := Summarize(repo, model.DefaultActionSet())
	if s.Total != 2 || s.Complete != 1 || s.InProgress != 1 || s.Actions != 4 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

// A cross-domain child stays in its own domain's block: the domain view is a
// directory, not a relation view — cross-domain edges belong to `kiln graph`.
func TestGroupByDomainKeepsCrossDomainFlat(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("k3", "子\nissue: child\naction: ask\ndomain: cli\nderived-from: parent", 3),
		mkCommit("k2", "父\nissue: parent\naction: conclude\ndomain: design", 2),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits, testRel), testRel))
	if len(groups) != 2 || groups[0].Domain != "cli" || groups[1].Domain != "design" {
		t.Fatalf("expected a cli block then a design block, got %+v", groups)
	}
	if len(groups[0].Issues) != 1 || groups[0].Issues[0].ID != "child" {
		t.Fatalf("the child must stay in its own block: %+v", groups[0].Issues)
	}
	if len(groups[1].Issues) != 1 || groups[1].Issues[0].ID != "parent" {
		t.Fatalf("unexpected design block: %+v", groups[1].Issues)
	}
}

// An issue belongs to exactly one block — its own domain — so a relation that
// points outside the block cannot move or duplicate the row.
func TestGroupByDomainSortsIssuesInABlockByID(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("m2", "二\nissue: second\naction: ask\ndomain: design\nspawns: first", 2),
		mkCommit("m1", "一\nissue: first\naction: ask\ndomain: design", 1),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits, testRel), testRel))
	if len(groups) != 1 || len(groups[0].Issues) != 2 {
		t.Fatalf("one block with two issues: %+v", groups)
	}
	if groups[0].Issues[0].ID != "first" || groups[0].Issues[1].ID != "second" {
		t.Fatalf("issues must be sorted by ID: %+v", groups[0].Issues)
	}
}

func TestGroupByDomainOrdersAndBucketsUndomained(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("n3", "无域\nissue: zeta\naction: ask", 3),
		mkCommit("n2", "域二\nissue: beta\naction: ask\ndomain: zzz", 2),
		mkCommit("n1", "域一\nissue: alpha\naction: ask\ndomain: aaa", 1),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits, testRel), testRel))
	if len(groups) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(groups))
	}
	if groups[0].Domain != "aaa" || groups[1].Domain != "zzz" || groups[2].Domain != NoDomain {
		t.Fatalf("unexpected order: %q, %q, %q", groups[0].Domain, groups[1].Domain, groups[2].Domain)
	}
	if CountDomains(groups) != 2 {
		t.Fatalf("CountDomains = %d, want 2", CountDomains(groups))
	}
}

// path's relation block reads with the issue as the subject: its own
// declarations first, then the inbound edges flipped to the inverse name. A name
// that only shows up because it was flipped carries its glossary once.
func TestPathLinesPutsOutboundBeforeInbound(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("e4", "丙被甲阻塞\nissue: c\naction: ask\nblocked-by: a", 4),
		mkCommit("e3", "乙引发甲\nissue: b\naction: ask\ncauses: a", 3),
		mkCommit("e2", "甲声明\nissue: a\naction: hypothesize\nblocks: b\nrelates-to: d\npart-of: e", 2),
		mkCommit("e1", "甲开题\nissue: a\naction: ask", 1),
		mkCommit("e0", "丁\nissue: d\naction: ask", 1),
		mkCommit("e00", "戊\nissue: e\naction: ask", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	got := PathLines(repo, repo.Issues["a"], testRel)
	want := []string{
		"blocks b",
		"relates-to d",
		"part-of e",
		"caused-by b（caused-by ＝ 谁引发了我）",
		"blocks c",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PathLines(a) = %#v, want %#v", got, want)
	}
}

// A pair declared from both sides is one edge, so path must show it once —
// whichever issue is in view.
func TestPathLinesDedupesBothSides(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("g2", "反向声明\nissue: b\naction: ask\ncaused-by: a", 2),
		mkCommit("g1", "正向声明\nissue: a\naction: ask\ncauses: b", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	if got := PathLines(repo, repo.Issues["a"], testRel); !reflect.DeepEqual(got, []string{"causes b"}) {
		t.Fatalf("PathLines(a) = %#v, want [causes b]", got)
	}
	if got := PathLines(repo, repo.Issues["b"], testRel); !reflect.DeepEqual(got, []string{"caused-by a"}) {
		t.Fatalf("PathLines(b) = %#v, want [caused-by a]", got)
	}
}

// A symmetric relation reads the same from either side, so the inbound view
// keeps its name and needs no gloss.
func TestPathLinesKeepsSymmetricRelationsUnflipped(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("h2", "乙\nissue: b\naction: ask", 2),
		mkCommit("h1", "甲相关乙\nissue: a\naction: ask\nrelates-to: b", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	if got := PathLines(repo, repo.Issues["b"], testRel); !reflect.DeepEqual(got, []string{"relates-to a"}) {
		t.Fatalf("symmetric inbound = %#v, want [relates-to a]", got)
	}
}

// The glossary is for the reader who meets an inverse name for the first time,
// so it lands on the first flipped line only.
func TestPathLinesGlossesEachFlippedNameOnce(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("i3", "丙引发甲\nissue: c\naction: ask\ncauses: a", 3),
		mkCommit("i2", "乙引发甲\nissue: b\naction: ask\ncauses: a", 2),
		mkCommit("i1", "甲开题\nissue: a\naction: ask", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	got := PathLines(repo, repo.Issues["a"], testRel)
	want := []string{"caused-by b（caused-by ＝ 谁引发了我）", "caused-by c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gloss must land on the first flipped name only: %#v", got)
	}
}

// An issue that only ever appears as someone else's target still gets a block:
// that is the bug this view fixes.
func TestPathLinesShowsAnIssueThatOnlyHasInbound(t *testing.T) {
	commits := []vcs.Commit{
		mkCommit("j2", "乙引发甲\nissue: b\naction: ask\ncauses: a", 2),
		mkCommit("j1", "甲开题\nissue: a\naction: ask", 1),
	}
	repo := Build(ActionsFromCommits(commits, testRel), testRel)
	if got := PathLines(repo, repo.Issues["a"], testRel); len(got) != 1 {
		t.Fatalf("an inbound-only issue must still show its edge: %#v", got)
	}
}

func hasDiag(diags []Diagnostic, issue string, level Level, substr string) bool {
	for _, d := range diags {
		if d.Issue == issue && d.Level == level && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}
