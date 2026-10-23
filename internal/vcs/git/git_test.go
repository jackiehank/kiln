package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackiehank/kiln/internal/vcs"
)

// "Nothing to record" is no longer a failure: the unified semantics make an empty
// action legal on both backends, so kiln stages the whole area and lets git record
// an empty commit. Reporting git's advice text was the old contract.
func TestActionRecordsAnEmptySnapshot(t *testing.T) {
	dir := newRepo(t) // clean tree: no changes at all
	id, err := ExecRunner{Dir: dir}.Action("issue: x\naction: ask", nil)
	if err != nil {
		t.Fatalf("an empty action must land: %v", err)
	}
	if id == "" {
		t.Fatal("Action must return the id it wrote")
	}
	out, err := ExecRunner{Dir: dir}.run("log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "issue: x action: ask" {
		t.Fatalf("last commit subject = %q", out)
	}
}

// A path-scoped action leaves everything else in the area: that is the one
// difference from the default snapshot, and the reason --only exists.
func TestActionWithPathsLeavesTheRestInTheArea(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "changed")
	write(t, dir, "b.txt", "new")
	r := ExecRunner{Dir: dir}
	if _, err := r.Action("only a", []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	changes, err := r.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0] != "b.txt" {
		t.Fatalf("changes after a path-scoped action = %v, want [b.txt]", changes)
	}
	out, err := r.run("show", "--stat", "--format=%s", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.txt") || strings.Contains(out, "b.txt") {
		t.Fatalf("the action must carry a.txt alone:\n%s", out)
	}
}

// A listed path with no change is refused before anything is committed. jj would
// report success and record an empty action, so the check is the same on both
// backends; on git it also keeps a no-op path from landing as a silent commit.
func TestActionRefusesAPathWithNoChange(t *testing.T) {
	dir := newRepo(t)
	_, err := ExecRunner{Dir: dir}.Action("x", []string{"a.txt"})
	if err == nil {
		t.Fatal("a path with no change must be refused")
	}
	if !strings.Contains(err.Error(), "没有改动可记录") {
		t.Fatalf("error = %q", err.Error())
	}
	out, err := ExecRunner{Dir: dir}.run("log", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "x") {
		t.Fatalf("the refused action must not land:\n%s", out)
	}
}

// The inventory is what the caller prints before committing: modified, deleted
// and untracked paths, root-relative, sorted.
func TestChangesReportsTheWholeArea(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "b.txt", "b")
	gitRun(t, dir, "add", "b.txt")
	commit(t, dir, "base")
	write(t, dir, "a.txt", "changed")
	write(t, dir, "c.txt", "new")
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	got, err := ExecRunner{Dir: dir}.Changes()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.txt", "b.txt", "c.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("changes = %v, want %v", got, want)
	}
}

// The status parser reads NUL-separated porcelain: a rename record carries the
// original path in a second field, which must not be counted as a change.
func TestParseStatusZSkipsRenameOrigin(t *testing.T) {
	got := parseStatusZ("R  new.txt\x00old.txt\x00 M a.txt\x00 A c.txt\x00")
	want := []string{"a.txt", "c.txt", "new.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", got, want)
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
	// The adapter commits through git without -c, so the repo needs its own
	// identity: CI runners have no global user.name/user.email.
	gitRun(t, root, "config", "user.name", "kiln")
	gitRun(t, root, "config", "user.email", "kiln@example.com")
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
