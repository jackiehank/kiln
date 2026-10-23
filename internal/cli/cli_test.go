package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	code := Run([]string{"do", "bad/id", "ask"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(errb.String(), "不符合") {
		t.Fatalf("expected a naming warning, stderr = %q", errb.String())
	}
	if !strings.Contains(out.String(), "issue: bad/id") {
		t.Fatalf("expected output to still be produced, got %q", out.String())
	}
}

// Case, `_` and doubled `-` are folded on the way in — on **every** action, not
// just the start — so the same issue cannot be split in two by spelling. The fold
// is announced, and the trailer carries the folded name.
func TestDoNormalizesTheIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), out: &out, errw: &errb}
	if code := e.write([]string{"Session__Expiry", "collect"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), `"Session__Expiry" → "session-expiry"`) {
		t.Fatalf("the fold must be announced, stderr = %q", errb.String())
	}
	if !strings.Contains(out.String(), "issue: session-expiry") {
		t.Fatalf("the trailer must carry the folded name, stdout = %q", out.String())
	}
	if strings.Contains(errb.String(), "不符合") {
		t.Fatalf("a folded ID is a valid ID: %q", errb.String())
	}
}

// A second start action on a name that already exists is refused: same name, same
// issue, so there is nothing to open. The message points at the way forward
// instead of leaving the caller to guess.
func TestDoRefusesASecondStart(t *testing.T) {
	var out, errb bytes.Buffer
	e := newJSONEnv(&out, &errb)
	if code := e.write([]string{"parent", "ask", "--title", "又开一次"}, false); code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr %q)", code, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("a refused action must print no message: %q", out.String())
	}
	for _, want := range []string{"拒绝", "已经有起点动作", "kiln commit parent", "hypothesize"} {
		if !strings.Contains(errb.String(), want) {
			t.Fatalf("refusal missing %q:\n%s", want, errb.String())
		}
	}
}

// The fold runs before the refusal, so a second start written in another spelling
// is refused too — otherwise `Session-Expiry` would open a twin of `session-expiry`.
func TestRefusalSeesThroughTheSpelling(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: fakeRunner{commits: []vcs.Commit{
		{ID: "c1", Date: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC), Message: "起点\n\nissue: demo-one\naction: ask\ntitle: 起点"},
	}}, out: &out, errw: &errb}
	if code := e.write([]string{"Demo__One", "ask"}, false); code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr %q)", code, errb.String())
	}
}

// An issue that exists **without** a start action can still take one: that is a
// repair — `check` reports the missing start — not a new issue.
func TestStartIsAllowedToRepairAnIssueWithoutOne(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: fakeRunner{commits: []vcs.Commit{
		{ID: "c1", Date: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC), Message: "先调研\n\nissue: orphan\naction: collect\ntitle: 先调研"},
	}}, out: &out, errw: &errb}
	if code := e.write([]string{"orphan", "ask"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "issue: orphan") {
		t.Fatalf("the repair must produce a message: %q", out.String())
	}
}

// `do` prints and must keep working outside a repository, so the duplicate check
// stays silent when the history cannot be read at all.
func TestDoOutsideARepositoryStillPrints(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), out: &out, errw: &errb}
	if code := e.write([]string{"nowhere", "ask"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "issue: nowhere") {
		t.Fatalf("stdout = %q", out.String())
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

// Domain blocks render as a header line naming the domain, then one aligned row
// per issue carrying the bare ID — the name you can paste straight back into a
// command. Nothing nests, and no row carries a domain prefix: relations belong to
// `kiln graph`.
func TestStatusRendersDomainBlocks(t *testing.T) {
	issue := func(id, domain, kind string) *model.Issue {
		return &model.Issue{ID: id, Domain: domain, Actions: []model.Action{{Kind: kind}}}
	}
	groups := []engine.DomainGroup{
		{Domain: "cli", Issues: []*model.Issue{issue("help-alignment", "cli", "apply")}},
		{Domain: "design", Issues: []*model.Issue{
			issue("action-config", "design", "apply"),
			issue("target-honesty", "design", "ask"),
		}},
	}
	var buf bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), out: &buf}
	e.renderStatus(e.statusRows(groups, nil))
	got := buf.String()

	if !strings.HasPrefix(got, "cli\n") {
		t.Fatalf("the first block must open with its domain:\n%s", got)
	}
	for _, want := range []string{"  help-alignment", "\ndesign\n", "  target-honesty"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"cli/", "design/", "└──", "├──"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("the domain view must not carry %q any more:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "\n\n") {
		t.Fatalf("expected a blank line between domain blocks:\n%s", got)
	}
}

// `status` states the repository-level worktree mode and how many areas are open
// — the "did this repository ask for isolation" question — marks the issues that
// have one, and explains the mark once at the end rather than on every row.
func TestStatusShowsWorktreeModeAndAreas(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: workspaceRunner{fakeRunner: demoRunner(), areas: []vcs.WorkspaceInfo{
			{Path: "C:/w/kiln", Main: true},
			{Path: "C:/w/kiln/.kiln/ws/demo", Issue: "demo"},
		}},
		out: &out, errw: &errb,
	}
	if code := e.status(nil); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{
		"worktree on（已开 1 个）",
		"■ = 已开 worktree——各自状态见 kiln worktree list",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q:\n%s", want, got)
		}
	}
	marked := false
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "demo ") {
			marked = strings.HasSuffix(line, AreaMark)
		}
	}
	if !marked {
		t.Fatalf("the issue with an area must carry the mark:\n%s", got)
	}
}

// The segment is a statement of state, so it keeps its shape — off, and zero —
// instead of coming and going; with no areas there is nothing to mark or explain.
func TestStatusStatesWorktreeOffWithNoAreas(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{
		actions:  model.DefaultActionSet(),
		repo:     workspaceRunner{fakeRunner: demoRunner()},
		worktree: config.WorktreeOff,
		out:      &out, errw: &errb,
	}
	if code := e.status(nil); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "worktree off（已开 0 个）") {
		t.Fatalf("status must still state the mode:\n%s", got)
	}
	if strings.Contains(got, AreaMark) {
		t.Fatalf("no area, no mark and no legend:\n%s", got)
	}
}

// The legend names the area the caller is standing in — cheap to know, and the
// question whoever is about to commit actually has.
func TestStatusLegendNamesTheCurrentArea(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: workspaceRunner{fakeRunner: demoRunner(), areas: []vcs.WorkspaceInfo{
			{Path: cwd, Issue: "demo"},
		}},
		out: &out, errw: &errb,
	}
	if code := e.status(nil); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "｜当前：worktree demo") {
		t.Fatalf("legend must name the area we stand in:\n%s", out.String())
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
	text := usageText(model.DefaultRelationSet())
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

// The help text has to stand alone. kiln ships as a tool of its own, so its own
// help cannot point at documents that live in one particular repository — the
// reader may not have them.
func TestUsageTextCitesNoRepositoryDocument(t *testing.T) {
	text := usageText(model.DefaultRelationSet())
	for _, unwanted := range []string{"docs/", ".md"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("help must not cite a document (%q):\n%s", unwanted, text)
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

func (f fakeRunner) Log() ([]vcs.Commit, error)    { return f.commits, nil }
func (f fakeRunner) Action(string) (string, error) { return fakeCommitID, nil }

// fakeCommitID is what the fake backends report as the commit they just wrote; it
// is long enough to exercise the short form the confirmation prints.
const fakeCommitID = "abc123def4567890"

// A commit says what it wrote: the message alone is what `do` already printed, so
// without the confirmation the caller has to run a second command to learn whether
// anything landed.
func TestCommitConfirmsWhatItWrote(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).write([]string{"demo", "collect", "--title", "调研"}, true); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "已提交 abc123d · collect · demo") {
		t.Fatalf("stdout must name the commit it wrote:\n%s", out.String())
	}
}

// An action name selects the last one of that kind — the same "newest match" rule
// git's :/<text> uses. Repeats are the norm here, so when there is a choice the
// hint says how many matched instead of silently picking one.
func TestShowSelectsByActionName(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "collect"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if got := out.String(); !strings.Contains(got, "demo · 2/2 collect · bbbbbbb · 调研") {
		t.Fatalf("show collect header wrong:\n%s", got)
	}
	if strings.Contains(errb.String(), "提示") {
		t.Fatalf("a unique match needs no hint: %q", errb.String())
	}
}

func TestShowTakesTheLastOfRepeatedActions(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: repeatedRunner(), out: &out, errw: &errb}
	if code := e.show([]string{"rep", "test"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if got := out.String(); !strings.Contains(got, "rep · 4/4 test · t4 · 三测") {
		t.Fatalf("show test must take the last one:\n%s", got)
	}
	if !strings.Contains(errb.String(), "有 3 个 test（序号 2, 3, 4）；已显示 4/4（最后一个）") {
		t.Fatalf("repeats must be announced with their addresses: %q", errb.String())
	}
	if !strings.Contains(errb.String(), "看别的：kiln show rep 2") {
		t.Fatalf("the hint must carry a paste-ready command: %q", errb.String())
	}
}

// A name that matches nothing is an answer, not a dead end: the error carries the
// actions this issue actually has, addressed the way the caller would use them.
func TestShowListsActionsWhenTheNameMatchesNone(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "test"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, want := range []string{`没有 "test" 动作`, "1.ask 2.collect"} {
		if !strings.Contains(errb.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errb.String())
		}
	}
}

func TestShowRejectsAnUnknownSelector(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newDemoEnv(&out, &errb).show([]string{"demo", "nope"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "无法识别的选择器") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// repeatedRunner models what a real issue looks like after a few rounds: the same
// action several times, so "show me that test" has to mean something.
func repeatedRunner() fakeRunner {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	return fakeRunner{
		commits: []vcs.Commit{
			{ID: "t1", Date: base, Message: "起点\n\nissue: rep\naction: ask\ntitle: 起点"},
			{ID: "t2", Date: base.Add(time.Hour), Message: "一测\n\nissue: rep\naction: test\ntitle: 一测"},
			{ID: "t3", Date: base.Add(2 * time.Hour), Message: "二测\n\nissue: rep\naction: test\ntitle: 二测"},
			{ID: "t4", Date: base.Add(3 * time.Hour), Message: "三测\n\nissue: rep\naction: test\ntitle: 三测"},
		},
		show: map[string]string{"t4": "+third line\n"},
	}
}
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
	dirty     bool
	areas     []vcs.WorkspaceInfo
	state     vcs.AreaState // what every dedicated area still holds
}

func (w workspaceRunner) IsDedicated() (bool, error) { return w.dedicated, nil }
func (w workspaceRunner) Dirty() (bool, error)       { return w.dirty, nil }
func (w workspaceRunner) NewWorkspace(issue string) (string, error) {
	return "/ws/repo--" + issue, nil
}
func (w workspaceRunner) ListWorkspaces() ([]vcs.WorkspaceInfo, error) { return w.areas, nil }
func (w workspaceRunner) AreaState(area vcs.WorkspaceInfo) (vcs.AreaState, error) {
	if area.Main {
		return vcs.AreaState{}, nil
	}
	return w.state, nil
}
func (w workspaceRunner) RemoveWorkspace(string, bool) error { return nil }

// newWorkspaceEnv builds an env whose backend reports the given layout, with
// isolation on: these tests exercise the guard a repository asks for by name,
// which is now opt-in rather than the zero-value default.
func newWorkspaceEnv(out, errw *bytes.Buffer, dedicated bool) *env {
	return &env{
		actions:  model.DefaultActionSet(),
		repo:     workspaceRunner{fakeRunner: demoRunner(), dedicated: dedicated},
		worktree: config.WorktreeOn,
		out:      out,
		errw:     errw,
	}
}

// Recording an action in the main area is refused: main is read/merge-only,
// which is what makes silent cross-agent contamination structurally impossible.
func TestCommitInMainIsRefused(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, false).write([]string{"demo", "collect"}, true); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "当前是 main") {
		t.Fatalf("stderr = %q, want the main-area refusal", errb.String())
	}
	if out.String() != "" {
		t.Fatalf("nothing was committed, yet stdout = %q", out.String())
	}
}

// In main with uncommitted work the refusal must change its advice: a new area is
// a clean checkout, so "先建区" alone would leave that work behind on main.
func TestCommitRefusalCarriesTheRecipeWhenMainIsDirty(t *testing.T) {
	var out, errb bytes.Buffer
	e := newWorkspaceEnv(&out, &errb, false)
	e.repo = workspaceRunner{fakeRunner: demoRunner(), dirty: true}
	e.kind = vcs.Git
	if code := e.write([]string{"demo", "collect"}, true); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	got := errb.String()
	if !strings.Contains(got, "你已有未提交改动") || !strings.Contains(got, "git stash -u") {
		t.Fatalf("refusal must hand over the recipe:\n%s", got)
	}
	if strings.Contains(got, "先 `kiln worktree new demo`") {
		t.Fatalf("with a dirty main the plain advice is the wrong one:\n%s", got)
	}
	// --force must not be offered as the cheaper alternative here: it releases the
	// guard, it does not move the work.
	if strings.Contains(got, "确要在 main 提交时加 --force") {
		t.Fatalf("the dirty branch must not offer --force as a parallel option:\n%s", got)
	}
	if !strings.Contains(got, "只放行护栏、不把改动搬进区") {
		t.Fatalf("the caveat must say what --force does not do:\n%s", got)
	}
	if strings.Contains(got, "设为 off") {
		t.Fatalf("a runtime message must not invite editing the repo config:\n%s", got)
	}
}

// A clean main needs no recipe: "先建区" is exactly right there.
func TestCommitRefusalPointsAtAnAreaWhenMainIsClean(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, false).write([]string{"demo", "collect"}, true); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	got := errb.String()
	if !strings.Contains(got, "先 `kiln worktree new demo`") {
		t.Fatalf("refusal must name the area to create:\n%s", got)
	}
	if strings.Contains(got, "你已有未提交改动") {
		t.Fatalf("nothing is dirty, so no warning belongs here:\n%s", got)
	}
	if strings.Contains(got, "设为 off") {
		t.Fatalf("a runtime message must not invite editing the repo config:\n%s", got)
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

// `do` is the one place a caller is guaranteed to pass through before committing,
// so it tells them what the repository asks of them — on stderr, leaving stdout
// the bare commit message and the exit code untouched.
func TestDoTipsWhenIsolationIsRequired(t *testing.T) {
	var out, errb bytes.Buffer
	e := newWorkspaceEnv(&out, &errb, false) // standing in main
	if code := e.write([]string{"demo", "collect"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "开工前先 kiln worktree new demo") {
		t.Fatalf("stderr = %q, want the isolation tip", errb.String())
	}
	if strings.Contains(out.String(), "提示") {
		t.Fatalf("stdout must stay the bare commit message:\n%s", out.String())
	}
}

// With uncommitted work in main the tip carries the backend's own recipe: "先建区"
// alone would leave that work behind, since a new area is a clean checkout.
func TestDoTipCarriesTheBackendRecipe(t *testing.T) {
	cases := []struct {
		name string
		kind vcs.Kind
		want string
	}{
		{"git", vcs.Git, "git stash -u"},
		{"jj", vcs.JJ, "jj edit @-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			e := &env{
				actions:  model.DefaultActionSet(),
				repo:     workspaceRunner{fakeRunner: demoRunner(), dirty: true},
				worktree: config.WorktreeOn,
				kind:     tc.kind,
				out:      &out, errw: &errb,
			}
			if code := e.write([]string{"demo", "test"}, false); code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(errb.String(), "你已有未提交改动") || !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("stderr = %q, want the %s recipe", errb.String(), tc.name)
			}
		})
	}
}

// Nothing to say when the repository asked for no isolation, or when the caller is
// already standing in an area.
func TestDoStaysQuietWhenThereIsNothingToAsk(t *testing.T) {
	cases := []struct {
		name string
		env  func(out, errw *bytes.Buffer) *env
	}{
		{"isolation off", func(out, errw *bytes.Buffer) *env {
			e := newWorkspaceEnv(out, errw, false)
			e.worktree = config.WorktreeOff
			return e
		}},
		{"already in an area", func(out, errw *bytes.Buffer) *env {
			return newWorkspaceEnv(out, errw, true)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := tc.env(&out, &errb).write([]string{"demo", "collect"}, false); code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
			}
			if errb.String() != "" {
				t.Fatalf("stderr = %q, want silence", errb.String())
			}
		})
	}
}

// `do` needs no repository — but a caller outside one should hear it, because that
// is the one thing worth saying there.
func TestDoSaysWhenThereIsNoRepository(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), dir: t.TempDir(), out: &out, errw: &errb}
	if code := e.write([]string{"demo", "ask"}, false); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "不在仓库里") {
		t.Fatalf("stderr = %q, want the no-repository tip", errb.String())
	}
	if !strings.Contains(out.String(), "issue: demo") {
		t.Fatalf("the message must still be printed:\n%s", out.String())
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
	for _, want := range []string{"C:/w/kiln ", "main（只读/只合并）", "C:/w/kiln--cli-necessity", "worktree cli-necessity"} {
		if !strings.Contains(got, want) {
			t.Fatalf("workspace list missing %q:\n%s", want, got)
		}
	}
}

// `worktree list` ends every row with what that area still holds — the same
// judgment `remove` refuses on, so the column and the refusal can never differ.
func TestWorktreeListShowsAreaState(t *testing.T) {
	cases := []struct {
		name  string
		state vcs.AreaState
		want  string
	}{
		{"clean", vcs.AreaState{}, "干净"},
		{"dirty", vcs.AreaState{Dirty: true}, "有改动"},
		{"unmerged", vcs.AreaState{Unmerged: true}, "未合并"},
		{"dirty and unmerged", vcs.AreaState{Dirty: true, Unmerged: true}, "有改动，未合并"},
		{"gone", vcs.AreaState{Gone: true}, "目录已删"},
		{"gone and unmerged", vcs.AreaState{Gone: true, Unmerged: true}, "目录已删，未合并"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			e := &env{
				actions: model.DefaultActionSet(),
				repo: workspaceRunner{state: tc.state, areas: []vcs.WorkspaceInfo{
					{Path: "C:/w/kiln", Main: true},
					{Path: "C:/w/kiln--t1", Issue: "t1"},
				}},
				out: &out, errw: &errb,
			}
			if code := e.workspace([]string{"list"}); code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("worktree list missing %q:\n%s", tc.want, out.String())
			}
		})
	}
}

// main is not a dedicated area, so its row reports nothing rather than claiming
// to be clean — the column only ever speaks about an issue's work.
func TestWorktreeListLeavesMainUnjudged(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: workspaceRunner{areas: []vcs.WorkspaceInfo{
			{Path: "C:/w/kiln", Main: true},
			{Path: "C:/w/kiln--t1", Issue: "t1"},
		}},
		out: &out, errw: &errb,
	}
	if code := e.workspace([]string{"list"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.Contains(line, "main（只读/只合并）") {
			continue
		}
		if !strings.HasSuffix(line, "—") {
			t.Fatalf("the main row must report no state: %q", line)
		}
		if strings.Contains(line, "干净") {
			t.Fatalf("main must not be called clean: %q", line)
		}
		return
	}
	t.Fatalf("no main row in:\n%s", out.String())
}

// brokenStateRunner cannot answer "what does this area hold".
type brokenStateRunner struct{ workspaceRunner }

func (brokenStateRunner) AreaState(vcs.WorkspaceInfo) (vcs.AreaState, error) {
	return vcs.AreaState{}, fmt.Errorf("读取失败")
}

// A backend that cannot answer must fail the command: a silently missing state
// would be read as "safe to drop".
func TestWorktreeListFailsWhenStateIsUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: brokenStateRunner{workspaceRunner{areas: []vcs.WorkspaceInfo{
			{Path: "C:/w/kiln", Main: true},
			{Path: "C:/w/kiln--t1", Issue: "t1"},
		}}},
		out: &out, errw: &errb,
	}
	if code := e.workspace([]string{"list"}); code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr = %q)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "状态") {
		t.Fatalf("stderr = %q, want it to name the failed state read", errb.String())
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
	for _, want := range []string{"/ws/repo--cli-necessity", "kiln commit cli-necessity", "worktree 是一次性检出"} {
		if !strings.Contains(got, want) {
			t.Fatalf("workspace new missing %q:\n%s", want, got)
		}
	}
	// The output must stay project-neutral: no advice about kiln's own binary.
	if strings.Contains(got, "二进制") {
		t.Fatalf("workspace new must not carry self-hosting advice:\n%s", got)
	}
}

// An issue ID that could not name a directory is rejected before anything is
// created, since the name is what binds the area to the issue.
// A loose ID is folded before it is judged, so the worktree name and the trailer
// value cannot drift apart.
func TestWorkspaceNewNormalizesTheIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).workspace([]string{"new", "Demo__One"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), `"Demo__One" → "demo-one"`) {
		t.Fatalf("the fold must be announced, stderr = %q", errb.String())
	}
	if !strings.Contains(out.String(), "/ws/repo--demo-one") {
		t.Fatalf("the area must be named after the folded issue: %q", out.String())
	}
}

func TestWorkspaceNewRejectsLooseIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newWorkspaceEnv(&out, &errb, true).workspace([]string{"new", "bad/id"}); code != 2 {
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
	text := usageText(model.DefaultRelationSet())
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
		return fmt.Errorf("worktree 还有未提交的改动: %s（要丢弃请加 --force）", issue)
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

// ---------------------------------------------------------------------------
// Machine-readable output
// ---------------------------------------------------------------------------

// jsonRunner models a cross-domain pair plus an undomained stray: the smallest
// history that exercises every row field — two domains (so the blocks have an
// order to get right), a finished issue, an unfinished one, and an issue with a
// worktree open. Every issue opens with `ask`, so the only findings are the
// missing terminal action.
func jsonRunner() workspaceRunner {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	return workspaceRunner{
		fakeRunner: fakeRunner{commits: []vcs.Commit{
			{ID: "d5", Date: base.Add(5 * time.Hour), Message: "子\n\nissue: child\naction: ask\ndomain: cli\nderived-from: parent\ntitle: 子"},
			{ID: "d4", Date: base.Add(4 * time.Hour), Message: "父落地\n\nissue: parent\naction: apply\ndomain: design\ntitle: 父落地"},
			{ID: "d3", Date: base.Add(3 * time.Hour), Message: "父开题\n\nissue: parent\naction: ask\ndomain: design\ntitle: 父开题"},
			{ID: "d2", Date: base.Add(2 * time.Hour), Message: "游荡验证\n\nissue: stray\naction: test\ntags: [draft]\ntitle: 游荡验证"},
			{ID: "d1", Date: base.Add(time.Hour), Message: "游荡开题\n\nissue: stray\naction: ask\ntags: [draft]\ntitle: 游荡开题"},
		}},
		areas: []vcs.WorkspaceInfo{
			{Path: "C:/w/kiln", Main: true},
			{Path: "C:/w/kiln/.kiln/ws/parent", Issue: "parent"},
		},
	}
}

// newJSONEnv builds an env over jsonRunner with a clean stderr.
func newJSONEnv(out, errw *bytes.Buffer) *env {
	return &env{actions: model.DefaultActionSet(), repo: jsonRunner(), out: out, errw: errw}
}

// `status --json` is one flat list of rows — no groups, no children — whose
// order is the order the text view prints, and whose absent keys mean "none".
func TestStatusJSONIsFlatAndOrdered(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).status([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	want := `{
  "summary": {
    "total": 3,
    "complete": 1,
    "in-progress": 2,
    "actions": 5,
    "domains": 2
  },
  "worktree": {
    "mode": "on",
    "open": 1
  },
  "issues": [
    {
      "id": "child",
      "domain": "cli",
      "action": "ask",
      "done": false,
      "title": "子",
      "has-worktree": false
    },
    {
      "id": "parent",
      "domain": "design",
      "action": "apply",
      "done": true,
      "title": "父落地",
      "has-worktree": true
    },
    {
      "id": "stray",
      "action": "test",
      "done": false,
      "title": "游荡验证",
      "has-worktree": false
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Fatalf("status --json =\n%s\nwant\n%s", got, want)
	}
}

// The document is the whole contract: nothing addressed to a human rides along
// — no marker legend, no hint — even when the repository has an area open.
func TestStatusJSONCarriesNoProse(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).status([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	for _, unwanted := range []string{AreaMark, "提示", "worktree list", "问题总数"} {
		if strings.Contains(out.String(), unwanted) {
			t.Fatalf("the machine view must carry no %q:\n%s", unwanted, out.String())
		}
	}
}

// An empty repository still gets a list, not a null: a decoder should never have
// to tell two kinds of nothing apart.
func TestStatusJSONEmitsAnEmptyArrayNotNil(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: fakeRunner{}, out: &out, errw: &errb}
	if code := e.status([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, `"issues": []`) {
		t.Fatalf("empty repository must still emit a list:\n%s", got)
	}
	if !strings.Contains(got, `"total": 0`) {
		t.Fatalf("counters must still be there:\n%s", got)
	}

	var listOut, listErr bytes.Buffer
	e2 := &env{actions: model.DefaultActionSet(), repo: fakeRunner{}, out: &listOut, errw: &listErr}
	if code := e2.list([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, listErr.String())
	}
	if !strings.Contains(listOut.String(), `"issues": []`) {
		t.Fatalf("list on an empty repository must still emit a list:\n%s", listOut.String())
	}
}

// The row names the area the caller stands in, so "where am I" needs no second
// command.
func TestStatusJSONNamesTheCurrentArea(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	e := &env{
		actions: model.DefaultActionSet(),
		repo: workspaceRunner{
			fakeRunner: jsonRunner().fakeRunner,
			areas:      []vcs.WorkspaceInfo{{Path: cwd, Issue: "parent"}},
		},
		out: &out, errw: &errb,
	}
	if code := e.status([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), `"current": "parent"`) {
		t.Fatalf("the document must name the area we stand in:\n%s", out.String())
	}
}

// `list --json` echoes the conditions it applied and reports the same row core
// `status` reports, so one decoder covers both commands.
func TestListJSONSharesTheIssueRow(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).list([]string{"--json", "--domain", "design"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	want := `{
  "filter": {
    "domain": "design"
  },
  "count": 1,
  "issues": [
    {
      "id": "parent",
      "domain": "design",
      "action": "apply",
      "done": true
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Fatalf("list --json =\n%s\nwant\n%s", got, want)
	}

	var statusOut, statusErr bytes.Buffer
	if code := newJSONEnv(&statusOut, &statusErr).status([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, statusErr.String())
	}
	var fromStatus, fromList struct {
		Issues []issueCore `json:"issues"`
	}
	if err := json.Unmarshal(statusOut.Bytes(), &fromStatus); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &fromList); err != nil {
		t.Fatal(err)
	}
	if len(fromStatus.Issues) == 0 || len(fromList.Issues) == 0 {
		t.Fatalf("both commands must report rows: %d / %d", len(fromStatus.Issues), len(fromList.Issues))
	}
	// The two views order rows differently on purpose — `status` walks domain
	// blocks, `list` sorts by ID — so the shared contract is the row itself, not
	// its position.
	pick := func(rows []issueCore) (issueCore, bool) {
		for _, r := range rows {
			if r.ID == "parent" {
				return r, true
			}
		}
		return issueCore{}, false
	}
	fromStatusRow, okStatus := pick(fromStatus.Issues)
	fromListRow, okList := pick(fromList.Issues)
	if !okStatus || !okList {
		t.Fatalf("both commands must report `parent`: %v / %v", okStatus, okList)
	}
	if !reflect.DeepEqual(fromStatusRow, fromListRow) {
		t.Fatalf("the shared row differs: status %+v vs list %+v", fromStatusRow, fromListRow)
	}
}

// Tags belong to `list`, which is the one view that reports them.
func TestListJSONReportsTagsAndOmitsUndomainedRows(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).list([]string{"--json", "--tag", "draft"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{`"tag": "draft"`, `"id": "stray"`, `"tags": [`, `"draft"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("list --json missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"domain"`) {
		t.Fatalf("an issue without a domain must not claim one:\n%s", got)
	}
}

// `check --json` names the rule rather than describing it, counts findings, and
// keeps OK in step with the exit code.
func TestCheckJSONNamesRulesAndMirrorsTheExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).check([]string{"--json"}); code != 0 {
		t.Fatalf("warnings alone must not fail the check (exit %d, stderr %q)", code, errb.String())
	}
	want := `{
  "ok": true,
  "summary": {
    "ok": 1,
    "warn": 2,
    "error": 0
  },
  "results": [
    {
      "issue": "child",
      "level": "warn",
      "rule": "terminal-action",
      "action": "ask",
      "actions": [
        "ask"
      ]
    },
    {
      "issue": "parent",
      "level": "ok",
      "rule": "complete",
      "actions": [
        "ask",
        "apply"
      ]
    },
    {
      "issue": "stray",
      "level": "warn",
      "rule": "terminal-action",
      "action": "test",
      "actions": [
        "ask",
        "test"
      ]
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Fatalf("check --json =\n%s\nwant\n%s", got, want)
	}
}

// The prose is the text view's; the machine view must not carry a single
// sentence of it.
func TestCheckJSONCarriesNoProse(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newJSONEnv(&out, &errb).check([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	for _, unwanted := range []string{"缺少", "完整（", "✓", "⚠"} {
		if strings.Contains(out.String(), unwanted) {
			t.Fatalf("the machine view must carry no %q:\n%s", unwanted, out.String())
		}
	}
}

// An error is both a finding and a non-zero exit, and OK says so without the
// caller having to look at the code.
func TestCheckJSONReportsErrorAndFails(t *testing.T) {
	runner := fakeRunner{commits: []vcs.Commit{
		{ID: "e1", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Message: "父不存在\n\nissue: broken\naction: conclude\nderived-from: missing\ntitle: 父不存在"},
	}}
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: runner, out: &out, errw: &errb}
	if code := e.check([]string{"--json"}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for an error", code)
	}
	got := out.String()
	for _, want := range []string{`"ok": false`, `"rule": "relation-target"`, `"ref": "missing"`, `"level": "error"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("check --json missing %q:\n%s", want, got)
		}
	}
}

// A relation-target finding names the broken relation, not just the issue it
// points at: two edges from one issue to the same missing target stay
// distinguishable, and the other rules never grow the field.
func TestCheckJSONNamesTheBrokenRelation(t *testing.T) {
	runner := fakeRunner{commits: []vcs.Commit{
		{ID: "r1", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Message: "缺靶\n\nissue: broken\naction: ask\nblocks: ghost\ntitle: 缺靶"},
	}}
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: runner, out: &out, errw: &errb}
	if code := e.check([]string{"--json"}); code != 0 {
		t.Fatalf("warnings alone must not fail the check (exit %d, stderr %q)", code, errb.String())
	}
	got := out.String()
	for _, want := range []string{`"rule": "relation-target"`, `"ref": "ghost"`, `"relation": "blocks"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("check --json missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, `"relation":`); n != 1 {
		t.Fatalf("only the relation-target finding carries a relation, got %d:\n%s", n, got)
	}
}

// A repository with no actions has nothing to report, and says only that — the
// text view's tip is not part of the document.
func TestCheckJSONOnAnEmptyRepository(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: fakeRunner{}, out: &out, errw: &errb}
	if code := e.check([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, `"ok": true`) || !strings.Contains(got, `"results": []`) {
		t.Fatalf("an empty repository is trivially fine:\n%s", got)
	}
	if strings.Contains(got, "没有 kiln 动作") {
		t.Fatalf("the tip belongs to the text view:\n%s", got)
	}
}

// One shape whatever the flags: --verbose changes the text view only, so a
// caller never has to write a second decoder.
func TestJSONKeepsOneShapeRegardlessOfVerbose(t *testing.T) {
	var plain, plainErr, verbose, verboseErr bytes.Buffer
	if code := newJSONEnv(&plain, &plainErr).check([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, plainErr.String())
	}
	if code := newJSONEnv(&verbose, &verboseErr).check([]string{"--json", "--verbose"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, verboseErr.String())
	}
	if plain.String() != verbose.String() {
		t.Fatalf("--verbose changed the document:\n%s\nvs\n%s", plain.String(), verbose.String())
	}
}

// `path` must show the edges that point at the issue, not just the ones it
// declared — that is the whole point of the inbound view.
func TestPathShowsInboundRelations(t *testing.T) {
	var out, errb bytes.Buffer
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	runner := fakeRunner{commits: []vcs.Commit{
		{ID: "in2", Date: base.Add(time.Hour), Message: "乙\n\nissue: b\naction: ask\ncauses: a"},
		{ID: "in1", Date: base, Message: "甲\n\nissue: a\naction: ask"},
	}}
	e := &env{actions: model.DefaultActionSet(), repo: runner, out: &out, errw: &errb}
	if code := e.path([]string{"a"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	for _, want := range []string{"  关系:", "    caused-by b（caused-by ＝ 谁引发了我）"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("path must show the inbound edge %q:\n%s", want, out.String())
		}
	}
}

// The help reuses the vocabulary's glosses, so a relation name is explained
// where it is offered.
func TestUsageGlossesRelationFlags(t *testing.T) {
	text := usageText(model.DefaultRelationSet())
	for _, want := range []string{"--blocked-by", "谁阻塞我", "--caused-by", "谁引发了我"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage must carry the gloss %q:\n%s", want, text)
		}
	}
}

// Every command that takes flags names the ones it accepts; a stray flag is
// warned about and then ignored (exit 0), so no command swallows a typo quietly.
func TestUnknownFlagWarnsOnEveryCommand(t *testing.T) {
	t.Run("path", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := newDemoEnv(&out, &errb).path([]string{"demo", "--bogus"}); code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
		}
		if !strings.Contains(errb.String(), "未知 flag --bogus") {
			t.Fatalf("path did not warn: %q", errb.String())
		}
		if !strings.Contains(out.String(), "demo") {
			t.Fatalf("path must still print: %q", out.String())
		}
	})
	t.Run("worktree list", func(t *testing.T) {
		var out, errb bytes.Buffer
		e := &env{
			actions: model.DefaultActionSet(),
			repo:    workspaceRunner{areas: []vcs.WorkspaceInfo{{Path: "C:/w/kiln", Main: true}}},
			out:     &out, errw: &errb,
		}
		if code := e.workspace([]string{"list", "--bogus"}); code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
		}
		if !strings.Contains(errb.String(), "未知 flag --bogus") {
			t.Fatalf("worktree list did not warn: %q", errb.String())
		}
	})
	t.Run("config", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		e := &env{actions: model.DefaultActionSet(), dir: root, out: &out, errw: &errb}
		if code := e.config([]string{"path", "--bogus"}); code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
		}
		if !strings.Contains(errb.String(), "未知 flag --bogus") {
			t.Fatalf("config did not warn: %q", errb.String())
		}
		if !strings.Contains(out.String(), config.FileName) {
			t.Fatalf("config must still print the path: %q", out.String())
		}
	})
}

// --force is a commit flag, and `do` prints the same message with the same
// flag set; it must not be reported as unknown when nobody commits.
func TestDoAcceptsForceWithoutAWarning(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"do", "x", "ask", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if strings.Contains(errb.String(), "未知 flag") {
		t.Fatalf("--force is a known flag: %q", errb.String())
	}
}

// graph --mermaid is the same document as --format=mermaid, the option's older
// spelling; both print a Mermaid flowchart and nothing but it.
func TestGraphMermaidFlagMatchesFormatAlias(t *testing.T) {
	var byFlag, byFormat, flagErr, formatErr bytes.Buffer
	if code := newDemoEnv(&byFlag, &flagErr).graph([]string{"--mermaid"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, flagErr.String())
	}
	if code := newDemoEnv(&byFormat, &formatErr).graph([]string{"--format=mermaid"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, formatErr.String())
	}
	if byFlag.String() != byFormat.String() {
		t.Fatalf("--mermaid and --format=mermaid must agree:\n%s\nvs\n%s", byFlag.String(), byFormat.String())
	}
	if !strings.HasPrefix(byFlag.String(), "flowchart LR\n") {
		t.Fatalf("mermaid document must start with its header:\n%s", byFlag.String())
	}
	if strings.Contains(byFlag.String(), "──") {
		t.Fatalf("mermaid must not reuse the ascii drawing:\n%s", byFlag.String())
	}
}

// A format other than ascii/mermaid/json is refused, and so is asking for two
// formats at once.
func TestGraphRejectsUnknownOrConflictingFormats(t *testing.T) {
	for _, args := range [][]string{
		{"--format=yaml"},
		{"--mermaid", "--json"},
		{"--json", "--format=ascii"},
	} {
		var out, errb bytes.Buffer
		if code := newDemoEnv(&out, &errb).graph(args); code != 2 {
			t.Fatalf("%v: exit code = %d, stderr = %q", args, code, errb.String())
		}
		if out.Len() != 0 {
			t.Fatalf("%v: a refused format must print no document: %q", args, out.String())
		}
	}
}

// graph --json is one document: nodes are issues, edges name the declaring
// side, and a target that was never opened lands in missing rather than nodes.
func TestGraphJSONRendersTheFoldedGraph(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newRelationEnv(&out, &errb).graph([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	var doc graphDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("graph --json is not one JSON document: %v\n%s", err, out.String())
	}
	if doc.Summary.Issues != 3 || doc.Summary.Edges != 2 || doc.Summary.Missing != 1 {
		t.Fatalf("summary = %+v, want 3 issues / 2 edges / 1 missing", doc.Summary)
	}
	var ids []string
	for _, n := range doc.Nodes {
		ids = append(ids, n.ID)
	}
	if !reflect.DeepEqual(ids, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("nodes must be sorted by id: %v", ids)
	}
	if got := doc.Nodes[2].Title; got != "依赖" {
		t.Fatalf("a node carries its latest action's title, got %q", got)
	}
	if got := doc.Nodes[0].Action; got != "ask" {
		t.Fatalf("node action = %q, want ask", got)
	}
	want := []graphEdge{
		{From: "beta", To: "alpha", Name: "relates-to", Directed: false},
		{From: "gamma", To: "ghost", Name: "caused-by", Directed: true},
	}
	if !reflect.DeepEqual(doc.Edges, want) {
		t.Fatalf("edges = %+v, want %+v", doc.Edges, want)
	}
	if !reflect.DeepEqual(doc.Missing, []string{"ghost"}) {
		t.Fatalf("missing = %v, want [ghost]", doc.Missing)
	}
}

// --json is the same document as --format=json, the older spelling.
func TestGraphJSONMatchesFormatAlias(t *testing.T) {
	var byFlag, byFormat, flagErr, formatErr bytes.Buffer
	if code := newDemoEnv(&byFlag, &flagErr).graph([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, flagErr.String())
	}
	if code := newDemoEnv(&byFormat, &formatErr).graph([]string{"--format=json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, formatErr.String())
	}
	if byFlag.String() != byFormat.String() {
		t.Fatalf("--json and --format=json must agree:\n%s\nvs\n%s", byFlag.String(), byFormat.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(byFlag.String()), "{") {
		t.Fatalf("json document must be one object:\n%s", byFlag.String())
	}
}

// An empty repository prints an empty document, not the text view's （空）.
func TestGraphJSONOnEmptyRepoIsStillJSON(t *testing.T) {
	var out, errb bytes.Buffer
	e := &env{actions: model.DefaultActionSet(), repo: fakeRunner{}, out: &out, errw: &errb}
	if code := e.graph([]string{"--json"}); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	var doc graphDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("empty repo must still be JSON: %v\n%s", err, out.String())
	}
	if doc.Summary.Issues != 0 || doc.Summary.Edges != 0 || doc.Summary.Missing != 0 {
		t.Fatalf("empty summary = %+v", doc.Summary)
	}
	if strings.Contains(out.String(), "（空）") {
		t.Fatalf("the text view's word for empty must not enter the document: %q", out.String())
	}
}

// relationRunner holds three issues: two declare the same symmetric relation
// from both sides, and one points a directed relation at an issue that was
// never opened.
func relationRunner() fakeRunner {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	return fakeRunner{commits: []vcs.Commit{
		{ID: "a1", Date: base, Message: "起点\n\nissue: alpha\naction: ask\ntitle: 起点\nrelates-to: beta"},
		{ID: "b1", Date: base.Add(time.Hour), Message: "另起\n\nissue: beta\naction: ask\ntitle: 另起\nrelates-to: alpha"},
		{ID: "c1", Date: base.Add(2 * time.Hour), Message: "依赖\n\nissue: gamma\naction: ask\ntitle: 依赖\ncaused-by: ghost"},
	}}
}

func newRelationEnv(out, errw *bytes.Buffer) *env {
	return &env{actions: model.DefaultActionSet(), repo: relationRunner(), out: out, errw: errw}
}

// The version command reports the three facts that decide "is this the binary I
// just built?", and its three spellings are one command.
func TestVersionPrintsTheBuildFacts(t *testing.T) {
	var first string
	for _, args := range [][]string{{"--version"}, {"-V"}, {"version"}} {
		var out, errb bytes.Buffer
		if code := Run(args, &out, &errb); code != 0 {
			t.Fatalf("%v: exit code = %d, stderr = %q", args, code, errb.String())
		}
		if errb.Len() != 0 {
			t.Fatalf("%v: version must not write stderr: %q", args, errb.String())
		}
		got := out.String()
		for _, want := range []string{"kiln ", "可执行文件: ", "构建时间: ", runtime.Version()} {
			if !strings.Contains(got, want) {
				t.Fatalf("%v: missing %q in:\n%s", args, want, got)
			}
		}
		if first == "" {
			first = got
		} else if got != first {
			t.Fatalf("%v and --version must print the same bytes:\n%s\nvs\n%s", args, first, got)
		}
	}
}

// A broken config does not make the version unanswerable: it reads neither the
// config nor any repository.
func TestVersionNeedsNoConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".kilnconfig.jsonc"), []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Run([]string{"--version"}, &out, &errb); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "kiln ") {
		t.Fatalf("version output missing: %q", out.String())
	}
}
