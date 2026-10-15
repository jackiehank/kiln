package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiln/internal/config"
	"kiln/internal/engine"
	"kiln/internal/model"
	"kiln/internal/vcs"
)

func TestDoPrintsCanonicalMessage(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "auth/session-expiry", "ask", "--title", "X 并发可能出错"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	want := "X 并发可能出错\n\nissue: auth/session-expiry\naction: ask\ntitle: X 并发可能出错\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

// The third positional (the retired `object`) is rejected instead of silently
// ignored, so a stray `q1` cannot slip through.
func TestDoRejectsRetiredObjectPositional(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "x", "ask", "q1"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "object 已退役") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestDoRejectsUnknownAction(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "auth/x", "frobnicate"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "非法 action") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestDoWarnsOnLooseIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "Bad_ID", "ask"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(errb.String(), "不符合") {
		t.Fatalf("expected a naming warning, stderr = %q", errb.String())
	}
	if !strings.Contains(out.String(), "issue: Bad_ID") {
		t.Fatalf("expected output to still be produced, got %q", out.String())
	}
}

// List flags accept both `--refs a,b` and `--refs a --refs b`; both forms must
// render the same canonical `refs: a, b`.
func TestDoListFlagsUseCanonicalForm(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"comma string", []string{"--refs", "a,b"}, "refs: a, b"},
		{"repeated flags", []string{"--refs", "a", "--refs", "b"}, "refs: a, b"},
		{"comma spaced tags", []string{"--tag", "security, performance"}, "tags: security, performance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			args := append([]string{"do", "smoke-test", "ask"}, tc.args...)
			if code := Run(args, &out, &errb); code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output = %q, want it to contain %q", out.String(), tc.want)
			}
		})
	}
}

func TestDoDropsEmptyListFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"do", "smoke-test", "ask", "--refs", ""}, &out, &errb); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if strings.Contains(out.String(), "refs:") {
		t.Fatalf("output = %q, want no refs line", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"nope"}, &out, &errb); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// Domain blocks render as `<domain>/<id>` lines, separated by blank lines, with
// cross-domain children nested under their parent.
func TestStatusRendersDomainBlocks(t *testing.T) {
	issue := func(id, domain, kind string) *model.Issue {
		return &model.Issue{ID: id, Domain: domain, Actions: []model.Action{{Kind: kind}}}
	}
	groups := []engine.DomainGroup{
		{Domain: "cli", Entries: []engine.Entry{{Issue: issue("help-alignment", "cli", "apply")}}},
		{Domain: "design", Entries: []engine.Entry{
			{Issue: issue("action-config", "design", "apply")},
			{Issue: issue("status-by-domain", "design", "ask"), Children: []engine.Entry{
				{Issue: issue("child", "cli", "apply")},
			}},
		}},
	}
	var buf bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), out: &buf}
	e.renderStatus(e.statusLines(groups))
	got := buf.String()

	for _, want := range []string{"cli/help-alignment", "design/action-config", "└── cli/child"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "\n\n") {
		t.Fatalf("expected a blank line between domain blocks:\n%s", got)
	}
}

func TestDisplayWidthCountsWideRunes(t *testing.T) {
	cases := map[string]int{"abc": 3, "中文": 4, "a中": 3, "": 0}
	for s, want := range cases {
		if got := displayWidth(s); got != want {
			t.Fatalf("displayWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestPadUsesDisplayWidth(t *testing.T) {
	if got := pad("中文", 6); got != "中文  " {
		t.Fatalf("pad cjk = %q", got)
	}
	if got := pad("ab", 4); got != "ab  " {
		t.Fatalf("pad ascii = %q", got)
	}
	if got := pad("abcdef", 3); got != "abcdef" {
		t.Fatalf("pad must not truncate: %q", got)
	}
}

// Every command line must place its description at the same display column,
// even though the commands differ in length.
func TestUsageTextAlignsDescriptions(t *testing.T) {
	text := usageText()
	col := -1
	for _, c := range usageCommands {
		i := strings.Index(text, "  "+c[0])
		if i < 0 {
			t.Fatalf("command %q missing from usage text", c[0])
		}
		line := text[i:]
		if j := strings.Index(line, "\n"); j >= 0 {
			line = line[:j]
		}
		if got := displayWidth(line) - displayWidth(c[1]); col == -1 {
			col = got
		} else if got != col {
			t.Fatalf("description column %d != %d for %q", got, col, c[0])
		}
	}
}

// fakeRunner supplies hand-built history and stat/show output so the CLI tests
// never touch a real repository.
type fakeRunner struct {
	commits []vcs.Commit
	show    map[string]string
	stat    map[string]string
	short   map[string]string
}

func (f fakeRunner) Log() ([]vcs.Commit, error) { return f.commits, nil }
func (f fakeRunner) Action(string) error        { return nil }
func (f fakeRunner) Show(id string) (string, error) {
	return f.show[id], nil
}
func (f fakeRunner) ShowStat(id string, short bool) (string, error) {
	if short {
		return f.short[id], nil
	}
	return f.stat[id], nil
}

// demoRunner models a two-action issue (ask then collect) with per-action stats.
func demoRunner() fakeRunner {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	return fakeRunner{
		commits: []vcs.Commit{
			{ID: "aaaaaaa1", Date: base, Message: "起点\n\nissue: demo\naction: ask\ntitle: 起点"},
			{ID: "bbbbbbb2", Date: base.Add(time.Hour), Message: "调研\n\nissue: demo\naction: collect\ntitle: 调研"},
		},
		stat: map[string]string{
			"aaaaaaa1": " docs/a.md | 2 ++\n 1 file changed, 2 insertions(+)\n",
			"bbbbbbb2": " docs/b.md | 3 +++\n 1 file changed, 3 insertions(+)\n",
		},
		short: map[string]string{
			"aaaaaaa1": " 1 file changed, 2 insertions(+)\n",
			"bbbbbbb2": " 1 file changed, 3 insertions(+)\n",
		},
		show: map[string]string{
			"aaaaaaa1": "commit aaaaaaa1\n\n起点\n\nissue: demo\naction: ask\n",
			"bbbbbbb2": "commit bbbbbbb2\n\n调研\n\nissue: demo\naction: collect\n\n+new line\n",
		},
	}
}

func newDemoEnv(out, errw *bytes.Buffer) *env {
	return &env{actions: model.DefaultActionSet(), repo: demoRunner(), out: out, errw: errw}
}

// `path --stat` keeps the action list and nests git's diffstat block under each
// action, mirroring `git log --stat`.
func TestPathStatIndentsEachAction(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).path([]string{"demo", "--stat"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{
		"  1. ask",
		"  2. collect",
		"        docs/a.md | 2 ++",
		"        docs/b.md | 3 +++",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("path --stat missing %q:\n%s", want, got)
		}
	}
}

// `path --shortstat` keeps each summary on its own action line.
func TestPathShortstatStaysOnActionLine(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).path([]string{"demo", "--shortstat"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	found := false
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "  2. ") {
			found = true
			if !strings.Contains(line, "(1 file changed, 3 insertions(+))") {
				t.Fatalf("shortstat not inline on the action line: %q", line)
			}
		}
	}
	if !found {
		t.Fatalf("action line 2 missing:\n%s", out.String())
	}
}

// Without a flag, `path` must stay the terse view it always was.
func TestPathWithoutFlagHasNoStat(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).path([]string{"demo"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if got := out.String(); strings.Contains(got, "files changed") || strings.Contains(got, "docs/") {
		t.Fatalf("plain path should not include stats:\n%s", got)
	}
}

// `show` with no ordinal expands the last action and delegates to git's show.
func TestShowDefaultsToLastAction(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "demo · 2/2 collect · bbbbbbb · 调研") {
		t.Fatalf("show header wrong:\n%s", got)
	}
	if !strings.Contains(got, "+new line") {
		t.Fatalf("show should print git's patch:\n%s", got)
	}
}

// An explicit ordinal selects that action.
func TestShowSelectsOrdinal(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "1"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if got := out.String(); !strings.Contains(got, "demo · 1/2 ask · aaaaaaa · 起点") {
		t.Fatalf("show 1 header wrong:\n%s", got)
	}
}

// `show --stat` gives only the stat block, not git's commit header.
func TestShowStatOmitsCommitHeader(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "2", "--stat"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "docs/b.md | 3 +++") {
		t.Fatalf("show --stat missing the stat block:\n%s", got)
	}
	if strings.Contains(got, "commit bbbbbbb2") {
		t.Fatalf("show --stat should not print git's commit header:\n%s", got)
	}
}

// An ordinal outside 1..N is a usage error, not a silent fallback.
func TestShowRejectsOutOfRangeOrdinal(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "9"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "超出范围") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// workspaceRunner is a fakeRunner that also stands in for a backend offering
// workspace isolation.
type workspaceRunner struct {
	fakeRunner
	dedicated bool
	areas     []vcs.WorkspaceInfo
}

func (w workspaceRunner) IsDedicated() (bool, error) { return w.dedicated, nil }
func (w workspaceRunner) NewWorkspace(issue string) (string, error) {
	return "/ws/repo--" + issue, nil
}
func (w workspaceRunner) ListWorkspaces() ([]vcs.WorkspaceInfo, error) { return w.areas, nil }
func (w workspaceRunner) RemoveWorkspace(string, bool) error           { return nil }

// newWorkspaceEnv builds an env whose backend reports the given layout.
func newWorkspaceEnv(out, errw *bytes.Buffer, dedicated bool) *env {
	return &env{
		actions: model.DefaultActionSet(),
		repo:    workspaceRunner{fakeRunner: demoRunner(), dedicated: dedicated},
		out:     out,
		errw:    errw,
	}
}

// Recording an action in the main area is refused: main is read/merge-only,
// which is what makes silent cross-agent contamination structurally impossible.
func TestCommitInMainIsRefused(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, false).write([]string{"demo", "collect"}, true); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "主干") {
		t.Fatalf("stderr = %q, want the main-area refusal", errb.String())
	}
	if out.String() != "" {
		t.Fatalf("nothing was committed, yet stdout = %q", out.String())
	}
}

// --force is the explicit escape hatch for committing in the main area.
func TestCommitInMainWithForce(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, false).write([]string{"demo", "collect", "--force"}, true); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "issue: demo") {
		t.Fatalf("stdout = %q", out.String())
	}
}

// A dedicated area records actions without any guard.
func TestCommitInDedicatedArea(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).write([]string{"demo", "collect"}, true); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
}

// `do` only prints, so the layout guard does not apply to it.
func TestDoIsUnaffectedByTheGuard(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, false).write([]string{"demo", "collect"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
}

// `workspace list` labels the main area and names each dedicated one.
func TestWorkspaceListLabelsAreas(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: workspaceRunner{areas: []vcs.WorkspaceInfo{
			{Path: "C:/w/kiln", Main: true},
			{Path: "C:/w/kiln--cli-necessity", Issue: "cli-necessity"},
		}},
		out:  &out,
		errw: &errb,
	}
	if code := e.workspace([]string{"list"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{"C:/w/kiln ", "主干", "C:/w/kiln--cli-necessity", "专用 cli-necessity"} {
		if !strings.Contains(got, want) {
			t.Fatalf("workspace list missing %q:\n%s", want, got)
		}
	}
}

// `workspace new` derives the area from the issue and points the user at it.
func TestWorkspaceNewReportsPath(t *testing.T) {
	var out, errb bytes.Buffer
	e := newWorkspaceEnv(&out, &errb, true)
	if code := e.workspace([]string{"new", "cli-necessity"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{"/ws/repo--cli-necessity", "kiln commit cli-necessity", "区内没有 kiln 二进制"} {
		if !strings.Contains(got, want) {
			t.Fatalf("workspace new missing %q:\n%s", want, got)
		}
	}
}

// An issue ID that could not name a directory is rejected before anything is
// created, since the name is what binds the area to the issue.
func TestWorkspaceNewRejectsLooseIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).workspace([]string{"new", "Bad_ID"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "非法 issue ID") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// A missing or unknown subcommand is a usage error.
func TestWorkspaceUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).workspace(nil); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// `config path` names the repository's state root and seeds a commented default
// there, so a fresh repository has something to edit.
func TestConfigPathSeedsTheFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), dir: root, out: &out, errw: &errb}
	if code := e.config([]string{"path"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	want := filepath.Join(root, config.FileName)
	if !strings.Contains(out.String(), want) {
		t.Fatalf("stdout = %q, want it to name %s", out.String(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("config was not seeded: %v", err)
	}
}

// The second run reports the same path without rewriting anything.
func TestConfigPathKeepsAnExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), dir: root, out: &out, errw: &errb}
	if code := e.config([]string{"path"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if strings.Contains(out.String(), "原本不存在") {
		t.Fatalf("second run claimed to seed: %q", out.String())
	}
}

// A missing or unknown subcommand is a usage error.
func TestConfigUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), dir: ".", out: &out, errw: &errb}
	if code := e.config(nil); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// A repository that turned isolation off records an action in the main area
// without any guard — no --force needed.
func TestCommitInMainAllowedWhenWorktreeOff(t *testing.T) {
	var out, errb bytes.Buffer
	e := newWorkspaceEnv(&out, &errb, false)
	e.worktree = config.WorktreeOff
	if code := e.write([]string{"demo", "collect"}, true); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "issue: demo") {
		t.Fatalf("stdout = %q", out.String())
	}
}

// The command surface speaks one word for isolation: worktree.
func TestUsageNamesTheWorktreeCommand(t *testing.T) {
	text := usageText()
	for _, want := range []string{"kiln worktree new", "kiln worktree list", "kiln worktree remove"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage must name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "kiln workspace") {
		t.Fatalf("the retired command name must be gone from usage:\n%s", text)
	}
}

// The rename is a hard cut: the old command name is rejected, not aliased.
func TestRetiredWorkspaceCommandIsRejected(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"workspace", "list"}, &out, &errb); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	// A hard cut still points the caller at the new name.
	if !strings.Contains(errb.String(), "未知命令") || !strings.Contains(errb.String(), "已改名为 `worktree`") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// needsForceRunner is a fake whose RemoveWorkspace refuses until force is set, so
// a test can prove the flag actually reaches the backend.
type needsForceRunner struct{ workspaceRunner }

func (n needsForceRunner) RemoveWorkspace(issue string, force bool) error {
	if !force {
		return fmt.Errorf("专用区还有未提交的改动: %s（要丢弃请加 --force）", issue)
	}
	return nil
}

// `worktree remove` names the issue to the backend and reports the deletion.
func TestWorktreeRemoveDeletesTheArea(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: workspaceRunner{}, out: &out, errw: &errb}
	if code := e.workspace([]string{"remove", "cli-necessity"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "cli-necessity") {
		t.Fatalf("stdout = %q", out.String())
	}
}

// A backend refusal becomes a non-zero exit carrying its message, and --force is
// the way through it.
func TestWorktreeRemoveForwardsForce(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: needsForceRunner{}, out: &out, errw: &errb}
	if code := e.workspace([]string{"remove", "x"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "未提交的改动") {
		t.Fatalf("stderr = %q", errb.String())
	}

	var out2, errb2 bytes.Buffer
	e2 := &env{actions: model.DefaultActionSet(), repo: needsForceRunner{}, out: &out2, errw: &errb2}
	if code := e2.workspace([]string{"remove", "x", "--force"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb2.String())
	}
}

// Removing without naming an issue is a usage error.
func TestWorktreeRemoveRequiresAnIssue(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: workspaceRunner{}, out: &out, errw: &errb}
	if code := e.workspace([]string{"remove"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
