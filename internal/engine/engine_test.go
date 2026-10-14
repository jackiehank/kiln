package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"kiln/internal/git"
	"kiln/internal/model"
)

func mkCommit(hash, msg string, day int) git.Commit {
	return git.Commit{
		Hash:    hash,
		Date:    time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC),
		Message: msg,
	}
}

// newest first, mirroring `git log`
func sampleCommits() []git.Commit {
	return []git.Commit{
		mkCommit("c3", "得出结论\nissue: auth/session-expiry\naction: conclude", 3),
		mkCommit("c2", "验证假设\nissue: auth/session-expiry\naction: test", 2),
		mkCommit("c1", "提出问题\nissue: auth/session-expiry\naction: ask\nobject: q1", 1),
		mkCommit("c0", "fix: 无关提交\n\n普通正文", 0),
	}
}

func TestActionsFromCommitsFiltersAndOrders(t *testing.T) {
	actions := ActionsFromCommits(sampleCommits())
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
	repo := Build(ActionsFromCommits(sampleCommits()))
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
	repo := Build(ActionsFromCommits(sampleCommits()))
	diags := Check(repo, model.DefaultActionSet())
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("expected a single OK diagnostic, got %+v", diags)
	}
	if !strings.HasPrefix(diags[0].Message, "完整（ask, test, conclude）") {
		t.Fatalf("unexpected summary: %q", diags[0].Message)
	}
}

func TestCheckRules(t *testing.T) {
	commits := []git.Commit{
		mkCommit("d3", "没有 ask，也没闭环\nissue: sub/no-ask\naction: test", 4),
		mkCommit("d2", "父不存在\nissue: sub/bad-parent\naction: ask\nderived-from: missing/parent", 3),
		mkCommit("d1", "子不存在\nissue: sub/bad-child\naction: ask\nspawns: missing/child", 2),
	}
	repo := Build(ActionsFromCommits(commits))
	diags := Check(repo, model.DefaultActionSet())

	if !hasDiag(diags, "sub/no-ask", LevelErr, "缺少 ask") {
		t.Fatalf("rule 1 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/no-ask", LevelWarn, "缺少终点动作") {
		t.Fatalf("rule 2 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-parent", LevelErr, "父问题不存在") {
		t.Fatalf("rule 3 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-child", LevelErr, "子问题不存在") {
		t.Fatalf("rule 4 not reported: %+v", diags)
	}
}

// Actions after the start are an unordered combination: the path below runs
// ask -> test -> collect -> conclude, which is not the "canonical" order, and
// must still count as complete.
func TestCheckDoesNotEnforceOrder(t *testing.T) {
	commits := []git.Commit{
		mkCommit("h4", "结论\nissue: o/r\naction: conclude", 4),
		mkCommit("h3", "采集\nissue: o/r\naction: collect", 3),
		mkCommit("h2", "验证\nissue: o/r\naction: test", 2),
		mkCommit("h1", "提问\nissue: o/r\naction: ask", 1),
	}
	repo := Build(ActionsFromCommits(commits))
	diags := Check(repo, model.DefaultActionSet())
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("order must not be enforced, got %+v", diags)
	}
}

func TestCheckHonoursCustomVocabulary(t *testing.T) {
	commits := []git.Commit{
		mkCommit("g2", "收尾\nissue: x/y\naction: done", 2),
		mkCommit("g1", "开始\nissue: x/y\naction: begin", 1),
	}
	repo := Build(ActionsFromCommits(commits))
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
	commits := []git.Commit{
		mkCommit("f1", "问题一\nissue: auth/token\naction: conclude\ndomain: auth\ntags: [security]", 3),
		mkCommit("f2", "问题二\nissue: perf/slow\naction: ask\ndomain: perf\ntags: [performance]", 2),
	}
	repo := Build(ActionsFromCommits(commits))

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
	repo := Build(ActionsFromCommits(commitList))
	s := Summarize(repo, model.DefaultActionSet())
	if s.Total != 2 || s.Complete != 1 || s.InProgress != 1 || s.Actions != 4 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

// A cross-domain child is relocated under its parent and leaves its own block.
func TestGroupByDomainRelocatesCrossDomainChild(t *testing.T) {
	commits := []git.Commit{
		mkCommit("k3", "子\nissue: child\naction: ask\ndomain: cli\nderived-from: parent", 3),
		mkCommit("k2", "父\nissue: parent\naction: conclude\ndomain: design", 2),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits)))
	if len(groups) != 1 || groups[0].Domain != "design" {
		t.Fatalf("expected one design block, got %+v", groups)
	}
	entries := groups[0].Entries
	if len(entries) != 1 || entries[0].Issue.ID != "parent" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if len(entries[0].Children) != 1 || entries[0].Children[0].Issue.ID != "child" {
		t.Fatalf("child not nested: %+v", entries[0].Children)
	}
}

// Relations between issues of the same domain must not nest.
func TestGroupByDomainKeepsSameDomainFlat(t *testing.T) {
	commits := []git.Commit{
		mkCommit("m2", "二\nissue: second\naction: ask\ndomain: design\nspawns: first", 2),
		mkCommit("m1", "一\nissue: first\naction: ask\ndomain: design", 1),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits)))
	if len(groups) != 1 || len(groups[0].Entries) != 2 {
		t.Fatalf("same-domain relations must stay flat: %+v", groups)
	}
	for _, en := range groups[0].Entries {
		if len(en.Children) != 0 {
			t.Fatalf("same-domain entry should not nest: %+v", en)
		}
	}
}

func TestGroupByDomainOrdersAndBucketsUndomained(t *testing.T) {
	commits := []git.Commit{
		mkCommit("n3", "无域\nissue: zeta\naction: ask", 3),
		mkCommit("n2", "域二\nissue: beta\naction: ask\ndomain: zzz", 2),
		mkCommit("n1", "域一\nissue: alpha\naction: ask\ndomain: aaa", 1),
	}
	groups := GroupByDomain(Build(ActionsFromCommits(commits)))
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

func hasDiag(diags []Diagnostic, issue string, level Level, substr string) bool {
	for _, d := range diags {
		if d.Issue == issue && d.Level == level && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}
