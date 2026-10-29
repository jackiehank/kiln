package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The golden net is the acceptance tool the structure refactor is built on:
// every later step only moves code, so the whole point is that the bytes a
// command prints do not change. A fragment assertion (the 172 `strings.Contains`
// calls elsewhere in this package) cannot see a dropped newline or a reordered
// column; a recorded whole output can.
//
// Each case is run twice — once with the cache, once with the cache switched off
// internally — and the two must be identical. That is the same invariant the
// refactor settled on, kept here so no later step can trade one path for the other.

// updateGolden rewrites testdata/golden from the current output. It is the only
// way those files change, so a diff there always means someone ran -update.
var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

type goldenCase struct {
	name string
	args []string
}

// goldenCases is the read-only surface: nothing here writes a commit, adds a
// worktree or touches the cache directory, so every case can run against the
// same fixture in any order.
func goldenCases() []goldenCase {
	return []goldenCase{
		{"list", []string{"list"}},
		{"list_json", []string{"list", "--json"}},
		{"list_domain", []string{"list", "--domain=cli"}},
		{"list_tag", []string{"list", "--tag=perf"}},
		{"list_done", []string{"list", "--status=done"}},
		{"list_search", []string{"list", "--search", "快照"}},
		{"graph", []string{"graph"}},
		{"graph_json", []string{"graph", "--json"}},
		{"graph_mermaid", []string{"graph", "--mermaid"}},
		{"graph_focus", []string{"graph", "--issue", "cache", "--depth", "1"}},
		{"path_cache", []string{"path", "cache"}},
		{"path_all_json", []string{"path", "--all", "--json"}},
		{"path_stat", []string{"path", "cache", "--stat"}},
		{"show", []string{"show", "cache"}},
		{"show_action", []string{"show", "cache", "apply"}},
		{"show_json", []string{"show", "cache", "--json"}},
		{"show_stat", []string{"show", "cache", "--stat"}},
		{"status", []string{"status"}},
		{"status_json", []string{"status", "--json"}},
		{"check", []string{"check"}},
		{"check_verbose", []string{"check", "--verbose"}},
		{"check_json", []string{"check", "--json"}},
		{"action_list", []string{"action", "list"}},
		{"action_log", []string{"action", "log"}},
		{"action_log_limit", []string{"action", "log", "--limit", "2"}},
		{"action_log_jsonl", []string{"action", "log", "--jsonl"}},
		{"worktree_list", []string{"worktree", "list"}},
		{"worktree_list_json", []string{"worktree", "list", "--json"}},
		{"config_path", []string{"config", "path"}},
		{"help", []string{"help"}},
	}
}

// TestGolden records and checks the whole output of every read command.
//
// Run `go test ./internal/cli -run TestGolden -update` to re-record after an
// intentional output change; review that diff like any other contract change.
func TestGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("golden runs a real git history per fixture")
	}
	graders := []struct {
		name  string
		build func(*testing.T) string
	}{
		{"basic", goldenBasicRepo},
		{"area", goldenAreaRepo},
	}
	for _, g := range graders {
		g := g
		t.Run(g.name, func(t *testing.T) {
			dir := g.build(t)
			t.Chdir(dir)
			// One warm read so the record layer and the enumeration snapshot exist:
			// the cases below then exercise the cache path, not the cold one.
			cacheOutput(t, "list")
			for _, c := range goldenCases() {
				c := c
				t.Run(c.name, func(t *testing.T) {
					warm := runGolden(t, c.args...)
					cold := runColdGolden(t, c.args...)
					if warm != cold {
						t.Fatalf("cache and the cold path disagree for %v:\ncache:\n%s\ncold:\n%s",
							c.args, warm.render(dir, c.args), cold.render(dir, c.args))
					}
					want := warm.render(dir, c.args)
					file := goldenFile(g.name, c.name+".txt")
					if *updateGolden {
						if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file, []byte(want), 0o644); err != nil {
							t.Fatal(err)
						}
						return
					}
					got, err := os.ReadFile(file)
					if err != nil {
						t.Fatalf("missing golden (run with -update): %v", err)
					}
					if string(got) != want {
						t.Fatalf("output drifted from %s:\n%s", file, firstDiff(string(got), want))
					}
				})
			}
			if !*updateGolden {
				checkNoStaleGoldens(t, g.name)
			}
		})
	}
}

type goldenRun struct {
	code   int
	stdout string
	stderr string
}

func runGolden(t *testing.T, args ...string) goldenRun {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, &out, &errb)
	return goldenRun{code: code, stdout: out.String(), stderr: errb.String()}
}

// runColdGolden answers the same command with the record cache switched off: the
// oracle a warm read is checked against. The switch is internal — the CLI itself
// has no way to run uncached, because a user repairs a cache with
// `kiln cache clear`.
func runColdGolden(t *testing.T, args ...string) goldenRun {
	t.Helper()
	cacheOff = true
	defer func() { cacheOff = false }()
	return runGolden(t, args...)
}

// render is the recorded form: the argv names the case inside the file, and the
// byte counts make a missing trailing newline visible instead of invisible.
func (g goldenRun) render(root string, args []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "argv: kiln %s\n", strings.Join(args, " "))
	fmt.Fprintf(&b, "exit: %d\n", g.code)
	out := normalizeGolden(root, g.stdout)
	errOut := normalizeGolden(root, g.stderr)
	// The counts are of the normalized body: the fixture path itself changes length
	// between runs, so counting the raw bytes would make the manifest unstable.
	fmt.Fprintf(&b, "stdout[%d]:\n%s\n", len(out), out)
	fmt.Fprintf(&b, "stderr[%d]:\n%s\n", len(errOut), errOut)
	return b.String()
}

// normalizeGolden removes the one thing that cannot be reproduced across runs:
// the temporary directory the fixture lives in, which several commands print.
// A separator directly after <ROOT> is rendered as `/`, so a golden recorded on
// Windows (where `config path` prints `\`) also matches on a Linux runner.
func normalizeGolden(root, s string) string {
	slash := filepath.ToSlash(root)
	escaped := strings.ReplaceAll(root, `\`, `\\`)
	for _, form := range []string{escaped, root, slash} {
		if form != "" {
			s = strings.ReplaceAll(s, form, "<ROOT>")
		}
	}
	return strings.ReplaceAll(s, "<ROOT>"+`\`, "<ROOT>/")
}

// firstDiff reports the first differing line, with one line of context each side.
func firstDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) && i < len(g); i++ {
		if w[i] != g[i] {
			lo := max(0, i-1)
			hi := min(len(w), i+2)
			return fmt.Sprintf("first difference at line %d:\nrecorded:\n%s\ncurrent:\n%s",
				i+1, strings.Join(w[lo:hi], "\n"), strings.Join(g[lo:min(len(g), hi)], "\n"))
		}
	}
	if len(w) != len(g) {
		return fmt.Sprintf("recorded %d lines, current %d", len(w), len(g))
	}
	return "output differs only by trailing bytes"
}

// checkNoStaleGoldens fails when a recorded file no longer has a case, so
// removing a command from the matrix is not silently forgotten.
func checkNoStaleGoldens(t *testing.T, fixture string) {
	t.Helper()
	names := map[string]bool{}
	for _, c := range goldenCases() {
		names[c.name+".txt"] = true
	}
	entries, err := os.ReadDir(goldenFile(fixture))
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	for _, e := range entries {
		if !names[e.Name()] {
			t.Errorf("stale golden without a case: %s/%s", fixture, e.Name())
		}
	}
}

// goldenFile resolves a recorded output next to this test file, not next to the
// working directory: every case chdirs into the fixture before it runs.
func goldenFile(parts ...string) string {
	_, file, _, _ := runtime.Caller(1)
	return filepath.Join(append([]string{filepath.Dir(file), "testdata", "golden"}, parts...)...)
}

// goldenBasicRepo is the fixture every case runs against: four issues with a
// start, a hypothesis, a conclusion, relations and tags, plus one issue that has
// no start (so `check` has something to report). Dates are fixed and the test
// pins TZ=UTC, so the rendered timestamps are the same on every machine.
func goldenBasicRepo(t *testing.T) string {
	t.Helper()
	dir := cacheTestRepo(t)
	goldenCommit(t, dir, "2026-10-01T10:00:00+00:00", "读命令要跑几次 git",
		"issue: cache", "action: ask", "domain: tooling")
	goldenCommit(t, dir, "2026-10-01T11:00:00+00:00", "热读可以用快照",
		"issue: cache", "action: hypothesize", "domain: tooling")
	goldenCommit(t, dir, "2026-10-01T12:00:00+00:00", "落地枚举快照",
		"issue: cache", "action: apply", "domain: tooling", "tags: perf")
	goldenCommit(t, dir, "2026-10-02T09:00:00+00:00", "图形输出要能收窄",
		"issue: graph", "action: ask", "domain: design", "relates-to: cache")
	goldenCommit(t, dir, "2026-10-02T10:00:00+00:00", "图查询按邻域收窄",
		"issue: graph", "action: conclude", "domain: design", "relates-to: cache")
	goldenCommit(t, dir, "2026-10-03T09:00:00+00:00", "命令表面要一致",
		"issue: cli-consistency", "action: ask", "domain: cli", "part-of: cache")
	goldenCommit(t, dir, "2026-10-03T10:00:00+00:00", "统一 help 与错误文案",
		"issue: cli-consistency", "action: apply", "domain: cli", "part-of: cache", "tags: docs")
	goldenCommit(t, dir, "2026-10-04T09:00:00+00:00", "文档搜索还没做",
		"issue: doc-search", "action: ask", "domain: design", "blocks: cli-consistency")
	goldenCommit(t, dir, "2026-10-04T10:00:00+00:00", "找到三条可选路径",
		"issue: doc-search", "action: collect", "domain: design")
	goldenCommit(t, dir, "2026-10-05T09:00:00+00:00", "没有起点的问题",
		"issue: late-idea", "action: collect", "domain: tooling")
	// `config path` writes a default config when none exists; create it up front so
	// the case records the path and not the one-time creation notice.
	t.Chdir(dir)
	cacheOutput(t, "config", "path")
	return dir
}

// goldenAreaRepo adds one worktree, so `worktree list` and the area marks in
// `status` are recorded too. The area lives next to the repository so its path
// normalizes with the same <ROOT> replacement.
func goldenAreaRepo(t *testing.T) string {
	t.Helper()
	dir := goldenBasicRepo(t)
	area := dir + "-area"
	cacheGit(t, dir, nil, "", "worktree", "add", "-q", "-b", "kiln/area", area, "HEAD")
	return dir
}

// goldenCommit writes one commit with a fixed author and date, so the history —
// and therefore every rendered timestamp and ordering — is reproducible.
func goldenCommit(t *testing.T, dir, when, title string, trailers ...string) {
	t.Helper()
	msg := title + "\n\n" + strings.Join(trailers, "\n") + "\n"
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=" + when, "GIT_COMMITTER_DATE=" + when,
	}
	cacheGit(t, dir, env, msg, "commit", "--allow-empty", "-q", "-F", "-")
}
