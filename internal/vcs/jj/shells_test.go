package jj

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackiehank/kiln/internal/vcs"
)

// The shells are one line each — build argv, call run, hand back what came out.
// They were the untested half of this package only because testing them used to
// require a jj binary; runFn removes that requirement, so what is pinned here is
// the jj CLI contract itself: the exact argv each shell sends.

// recorder answers run calls in order and remembers them.
type recorder struct {
	calls [][]string
	outs  []string
	errs  []error
}

func (rec *recorder) run(args ...string) (string, error) {
	rec.calls = append(rec.calls, append([]string(nil), args...))
	i := len(rec.calls) - 1
	var out string
	var err error
	if i < len(rec.outs) {
		out = rec.outs[i]
	}
	if i < len(rec.errs) {
		err = rec.errs[i]
	}
	return out, err
}

func (rec *recorder) runner(dir string) ExecRunner {
	return ExecRunner{Dir: dir, runFn: rec.run}
}

func (rec *recorder) want(t *testing.T, i int, want ...string) {
	t.Helper()
	if len(rec.calls) <= i {
		t.Fatalf("call %d never happened; calls = %v", i, rec.calls)
	}
	got := rec.calls[i]
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("call %d argv = %q, want %q", i, got, want)
	}
}

func TestLogAsksForTheLogTemplate(t *testing.T) {
	rec := &recorder{outs: []string{"abc\n2026-10-01 10:00:00.000 +00:00\n标题\n" + recordSep}}
	commits, err := rec.runner("C:/repo").Log()
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "log", "--no-graph", "-r", revset, "-T", template)
	if len(commits) != 1 || commits[0].ID != "abc" {
		t.Fatalf("commits = %+v", commits)
	}
}

func TestChangesListsSummaryPaths(t *testing.T) {
	rec := &recorder{outs: []string{"M a.txt\nD b.txt\n"}}
	got, err := rec.runner("C:/repo").Changes()
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "diff", "--summary")
	if strings.Join(got, ",") != "a.txt,b.txt" {
		t.Fatalf("changes = %v", got)
	}
}

func TestUnchangedAsksPerPath(t *testing.T) {
	rec := &recorder{outs: []string{"M a\n", ""}}
	bad, err := rec.runner("C:/repo").Unchanged([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "diff", "--summary", "--", "a")
	rec.want(t, 1, "diff", "--summary", "--", "b")
	if len(bad) != 1 || bad[0] != "b" {
		t.Fatalf("bad = %v", bad)
	}
}

func TestShowAsksForTheRevision(t *testing.T) {
	rec := &recorder{outs: []string{"diff --git …"}}
	out, err := rec.runner("C:/repo").Show("c1")
	if err != nil || out != "diff --git …" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	rec.want(t, 0, "show", "-r", "c1")
}

func TestShowStatShortKeepsTheSummaryLine(t *testing.T) {
	rec := &recorder{outs: []string{"a.txt | 1 +\n1 file changed, 1 insertion(+)\n"}}
	got, err := rec.runner("C:/repo").ShowStat("c1", true)
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "diff", "-r", "c1", "--stat")
	if got != "1 file changed, 1 insertion(+)" {
		t.Fatalf("short stat = %q", got)
	}
}

func TestActionCommitsAndReadsTheNewID(t *testing.T) {
	rec := &recorder{outs: []string{"", "  newid  \n"}}
	id, err := rec.runner("C:/repo").Action("msg", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "commit", "--message", "msg")
	rec.want(t, 1, "log", "--no-graph", "-r", "@-", "-T", "commit_id")
	if id != "newid" {
		t.Fatalf("id = %q", id)
	}
}

func TestActionWithPathsPassesFilesets(t *testing.T) {
	rec := &recorder{outs: []string{"M a\n", "", "id\n"}}
	// Action checks the paths first (one diff per path), then commits.
	if _, err := rec.runner("C:/repo").Action("msg", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "diff", "--summary", "--", "a")
	rec.want(t, 1, "commit", "--message", "msg", "--", "a")
}

func TestActionRefusesPathsWithNoChange(t *testing.T) {
	rec := &recorder{outs: []string{""}}
	_, err := rec.runner("C:/repo").Action("msg", []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "没有改动可记录") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("nothing may be committed, calls = %v", rec.calls)
	}
}

// The id read-back is a nicety: the action is already recorded, so a failure here
// must not turn a successful commit into a reported failure.
func TestActionToleratesAFailedReadBack(t *testing.T) {
	rec := &recorder{errs: []error{nil, errFake{}}}
	id, err := rec.runner("C:/repo").Action("msg", nil)
	if err != nil || id != "" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

type errFake struct{}

func (errFake) Error() string { return "boom" }

func TestListWorkspacesPutsMainFirst(t *testing.T) {
	out := "default\x1fC:/repo\x1e" + "issue\x1fC:/repo/.kiln/ws/issue\x1e"
	rec := &recorder{outs: []string{out}}
	areas, err := rec.runner("C:/repo").ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "workspace", "list", "-T", workspaceFormat)
	if len(areas) != 2 || !areas[0].Main || areas[0].Path != "C:/repo" {
		t.Fatalf("areas = %+v", areas)
	}
	if areas[1].Main || areas[1].Issue != "issue" {
		t.Fatalf("second area = %+v", areas[1])
	}
}

func TestDirtyAsksAboutTheRunnersRoot(t *testing.T) {
	rec := &recorder{outs: []string{"M a.txt\n"}}
	dirty, err := rec.runner("C:/repo").Dirty()
	if err != nil || !dirty {
		t.Fatalf("dirty=%v err=%v", dirty, err)
	}
	rec.want(t, 0, "-R", "C:/repo", "diff", "--summary")
}

func TestAreaStateOfMainCallsNothing(t *testing.T) {
	rec := &recorder{}
	state, err := rec.runner("C:/repo").AreaState(vcs.WorkspaceInfo{Main: true})
	if err != nil || state != (vcs.AreaState{}) {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("main is not an area; calls = %v", rec.calls)
	}
}

func TestNewWorkspaceCreatesUnderTheStateRoot(t *testing.T) {
	root := t.TempDir()
	area := vcs.AreaPath(root, "issue")
	rec := &recorder{outs: []string{"default\x1f" + root + "\x1e"}}
	got, err := rec.runner(root).NewWorkspace("issue")
	if err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "workspace", "list", "-T", workspaceFormat)
	rec.want(t, 1, "workspace", "add", "--name", "issue", area)
	if got != area {
		t.Fatalf("path = %q, want %q", got, area)
	}
}

func TestRemoveWorkspaceForgetsAndDeletes(t *testing.T) {
	root := t.TempDir()
	area := vcs.AreaPath(root, "issue")
	if err := os.MkdirAll(filepath.Join(area), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{outs: []string{
		"default\x1f" + root + "\x1e" + "issue\x1f" + area + "\x1e",
		"",
	}}
	if err := rec.runner(root).RemoveWorkspace("issue", true); err != nil {
		t.Fatal(err)
	}
	rec.want(t, 0, "workspace", "list", "-T", workspaceFormat)
	rec.want(t, 1, "workspace", "forget", "issue")
	if _, err := os.Stat(area); !os.IsNotExist(err) {
		t.Fatalf("area directory must be gone, stat err = %v", err)
	}
}

func TestIsDedicatedReadsTheWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	area := vcs.AreaPath(root, "issue")
	rec := &recorder{outs: []string{area + "\n", "default\x1f" + root + "\x1e" + "issue\x1f" + area + "\x1e"}}
	dedicated, err := rec.runner(area).IsDedicated()
	if err != nil || !dedicated {
		t.Fatalf("dedicated=%v err=%v", dedicated, err)
	}
	rec.want(t, 0, "workspace", "root")
}
