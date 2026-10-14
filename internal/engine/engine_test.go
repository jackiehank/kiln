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
		mkCommit("c3", "得出结论\nproblem: auth/session-expiry\naction: conclude", 3),
		mkCommit("c2", "验证假设\nproblem: auth/session-expiry\naction: test", 2),
		mkCommit("c1", "提出问题\nproblem: auth/session-expiry\naction: ask\nobject: q1", 1),
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

func TestBuildAggregatesProblem(t *testing.T) {
	repo := Build(ActionsFromCommits(sampleCommits()))
	if len(repo.Problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(repo.Problems))
	}
	p := repo.Problems["auth/session-expiry"]
	if p == nil || len(p.Actions) != 3 {
		t.Fatalf("unexpected problem: %+v", p)
	}
	if !model.IsTerminal(p.State()) {
		t.Fatalf("expected terminal state, got %q", p.State())
	}
}

func TestCheckCompleteProblemIsOK(t *testing.T) {
	repo := Build(ActionsFromCommits(sampleCommits()))
	diags := Check(repo)
	if len(diags) != 1 || diags[0].Level != LevelOK {
		t.Fatalf("expected a single OK diagnostic, got %+v", diags)
	}
	if !strings.HasPrefix(diags[0].Message, "完整（ask → test → conclude）") {
		t.Fatalf("unexpected summary: %q", diags[0].Message)
	}
}

func TestCheckRules(t *testing.T) {
	commits := []git.Commit{
		mkCommit("d3", "没有 ask，也没闭环\nproblem: sub/no-ask\naction: test", 4),
		mkCommit("d2", "父不存在\nproblem: sub/bad-parent\naction: ask\nderived-from: missing/parent", 3),
		mkCommit("d1", "子不存在\nproblem: sub/bad-child\naction: ask\nspawns: missing/child", 2),
	}
	repo := Build(ActionsFromCommits(commits))
	diags := Check(repo)

	if !hasDiag(diags, "sub/no-ask", LevelErr, "缺少 ask") {
		t.Fatalf("rule 1 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/no-ask", LevelWarn, "缺少 conclude/apply") {
		t.Fatalf("rule 2 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-parent", LevelErr, "父问题不存在") {
		t.Fatalf("rule 3 not reported: %+v", diags)
	}
	if !hasDiag(diags, "sub/bad-child", LevelErr, "子问题不存在") {
		t.Fatalf("rule 4 not reported: %+v", diags)
	}
}

func TestFilter(t *testing.T) {
	commits := []git.Commit{
		mkCommit("f1", "问题一\nproblem: auth/token\naction: conclude\ndomain: auth\ntags: [security]", 3),
		mkCommit("f2", "问题二\nproblem: perf/slow\naction: ask\ndomain: perf\ntags: [performance]", 2),
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
		mkCommit("x1", "进行中\nproblem: perf/ongoing\naction: ask", 5))
	repo := Build(ActionsFromCommits(commitList))
	s := Summarize(repo)
	if s.Total != 2 || s.Complete != 1 || s.InProgress != 1 || s.Actions != 4 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

func hasDiag(diags []Diagnostic, problem string, level Level, substr string) bool {
	for _, d := range diags {
		if d.Problem == problem && d.Level == level && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}
