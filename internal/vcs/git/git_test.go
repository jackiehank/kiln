package git

import "testing"

// `git worktree list --porcelain` records are blank-line separated; the first
// one is the main checkout, and only a kiln/<issue> branch carries an issue.
func TestParseWorktreesReadsAreas(t *testing.T) {
	out := "worktree C:/w/kiln\nHEAD abc\nbranch refs/heads/master\n\n" +
		"worktree C:/w/kiln--cli-necessity\nHEAD def\nbranch refs/heads/kiln/cli-necessity\n\n" +
		"worktree C:/w/kiln-detached\nHEAD 0123\ndetached\n\n"
	got := parseWorktrees(out)
	if len(got) != 3 {
		t.Fatalf("got %d areas, want 3", len(got))
	}
	if !got[0].Main || got[0].Issue != "" || got[0].Path != "C:/w/kiln" {
		t.Fatalf("main area = %+v", got[0])
	}
	if got[1].Main || got[1].Issue != "cli-necessity" {
		t.Fatalf("dedicated area = %+v", got[1])
	}
	if got[2].Issue != "" {
		t.Fatalf("a detached worktree carries no issue: %+v", got[2])
	}
}

func TestParseWorktreesIgnoresEmptyOutput(t *testing.T) {
	if got := parseWorktrees("\n\n"); len(got) != 0 {
		t.Fatalf("got %d areas, want 0", len(got))
	}
}
