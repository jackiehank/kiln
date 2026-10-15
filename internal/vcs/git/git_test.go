package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kiln/internal/vcs"
)

// A failed commit must report *why*, and git puts several reasons on stdout
// ("nothing staged" above all). Reporting the first line would answer "On branch
// master" to the question "why did this fail?" — so the contract is the last
// line, which is where git puts the conclusion and the hint.
func TestActionReportsGitsReason(t *testing.T) {
	dir := newRepo(t) // clean tree: nothing staged
	_, err := ExecRunner{Dir: dir}.Action("issue: x\naction: ask")
	if err == nil {
		t.Fatal("committing with nothing staged must fail")
	}
	if !strings.Contains(err.Error(), "nothing to commit") {
		t.Fatalf("error must carry git's reason, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "On branch") {
		t.Fatalf("error must not report the context line, got %q", err.Error())
	}
}

// gitRun runs git in dir and fails the test with its combined output on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// commit records a commit in dir under a throwaway identity.
func commit(t *testing.T, dir, msg string) {
	t.Helper()
	gitRun(t, dir, "-c", "user.name=kiln", "-c", "user.email=kiln@example.com", "commit", "-qm", msg)
}

// write puts body at root/rel, creating parent directories first.
func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo makes a repository with one commit, so a worktree has something to
// branch from, and returns its root.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	write(t, root, "a.txt", "a")
	gitRun(t, root, "add", "a.txt")
	commit(t, root, "init")
	return root
}

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

// `AreaState` and `RemoveWorkspace` answer the same question — is this area safe
// to drop? — so a listing can never call an area safe while the removal refuses
// it. One area per ground: nothing held, uncommitted work, work main never took.
func TestAreaStateMatchesWhyRemoveRefuses(t *testing.T) {
	r := ExecRunner{Dir: newRepo(t)}

	// Nothing held: the state is empty, and the removal goes through.
	path, err := r.NewWorkspace("clean")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: "clean"}); err != nil || got != (vcs.AreaState{}) {
		t.Fatalf("clean area state = %+v, %v; want the zero value", got, err)
	}
	if err := r.RemoveWorkspace("clean", false); err != nil {
		t.Fatalf("a clean, merged area must be removable: %v", err)
	}

	// Uncommitted work is one ground: remove refuses, --force loses it.
	path, err = r.NewWorkspace("dirty")
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, "b.txt", "b")
	got, err := r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: "dirty"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Dirty || got.Unmerged {
		t.Fatalf("dirty area state = %+v; want Dirty alone", got)
	}
	if err := r.RemoveWorkspace("dirty", false); err == nil {
		t.Fatal("remove must refuse an area that still holds uncommitted changes")
	}
	if err := r.RemoveWorkspace("dirty", true); err != nil {
		t.Fatalf("--force must drop it: %v", err)
	}

	// A commit main never took is the other ground, and it stands alone: the area
	// itself is clean, so only the branch can refuse.
	path, err = r.NewWorkspace("unmerged")
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, "c.txt", "c")
	gitRun(t, path, "add", "c.txt")
	commit(t, path, "work")
	got, err = r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: "unmerged"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Dirty || !got.Unmerged {
		t.Fatalf("unmerged area state = %+v; want Unmerged alone", got)
	}
	if err := r.RemoveWorkspace("unmerged", false); err == nil {
		t.Fatal("remove must refuse a branch that was never merged")
	}
	if err := r.RemoveWorkspace("unmerged", true); err != nil {
		t.Fatalf("--force must drop it: %v", err)
	}

	// An area whose directory was deleted by hand stays registered: git calls it
	// prunable and keeps listing it, so the state must report it rather than fail
	// the whole listing. A plain `git clean -xdf` spares the areas — git skips a
	// nested worktree as a nested repository — but a hand delete or `-xffd` does not.
	path, err = r.NewWorkspace("gone")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	got, err = r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: "gone"})
	if err != nil {
		t.Fatalf("a missing directory must be reported, not raised: %v", err)
	}
	if !got.Gone || got.Dirty {
		t.Fatalf("gone area state = %+v; want Gone alone", got)
	}

	// The main checkout holds no issue's work, so it reports nothing.
	if got, err := r.AreaState(vcs.WorkspaceInfo{Main: true}); err != nil || got != (vcs.AreaState{}) {
		t.Fatalf("main state = %+v, %v; want the zero value", got, err)
	}
}
