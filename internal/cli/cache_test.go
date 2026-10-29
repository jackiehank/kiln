package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackiehank/kiln/internal/cache"
)

// These tests are the acceptance condition the design puts first: a read command
// must print the same bytes whether it answers from the cache, from a full scan,
// or with the cache disabled. They run against a real git repository.

func cacheTestRepo(t *testing.T, enabled bool) string {
	t.Helper()
	dir := t.TempDir()
	cacheGit(t, dir, nil, "", "init", "-q")
	if enabled {
		body := "{\n  // 缓存\n  \"cache\": { \"enabled\": true }\n}\n"
		if err := os.WriteFile(filepath.Join(dir, ".kilnconfig.jsonc"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cacheGit(t *testing.T, dir string, env []string, stdin string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// commitAction writes one kiln commit with a fixed author date, so the history —
// and therefore the read output — is reproducible.
func commitAction(t *testing.T, dir, issue, kind, title, when string) {
	t.Helper()
	msg := fmt.Sprintf("%s\n\nissue: %s\naction: %s\n", title, issue, kind)
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=" + when, "GIT_COMMITTER_DATE=" + when,
	}
	cacheGit(t, dir, env, msg, "commit", "--allow-empty", "-q", "-F", "-")
}

func cacheOutput(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := Run(args, &out, &errb); code != 0 {
		t.Fatalf("kiln %v: exit %d, stderr %q", args, code, errb.String())
	}
	return out.String()
}

func recordsPath(dir string) string { return filepath.Join(dir, ".git", "kiln", "records") }

func layerCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(recordsPath(dir))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "layer-") && strings.HasSuffix(e.Name(), ".rec") {
			n++
		}
	}
	return n
}

func TestCacheReadsPrintIdenticalBytes(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "beta", "ask", "乙单", "2026-10-01T11:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	commitAction(t, dir, "beta", "apply", "落地乙", "2026-10-02T10:00:00+08:00")
	t.Chdir(dir)

	cold := cacheOutput(t, "graph", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("a cold read must write exactly one layer, got %d", n)
	}

	hit := cacheOutput(t, "graph", "--json")
	if hit != cold {
		t.Fatalf("cache hit changed the output:\n--- cold ---\n%s\n--- hit ---\n%s", cold, hit)
	}
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("a hit must not write another layer, got %d", n)
	}

	// A new action makes the next read miss exactly that commit; it must still
	// print what a full scan prints, and leave a second layer behind. path --all
	// is the command that actually shows another action.
	before := cacheOutput(t, "path", "--all", "--json")
	commitAction(t, dir, "alpha", "apply", "追加", "2026-10-03T09:00:00+08:00")
	miss := cacheOutput(t, "path", "--all", "--json")
	if miss == before {
		t.Fatal("output must change after a new action")
	}
	if plainMiss := cacheOutput(t, "path", "--all", "--json", "--no-cache"); plainMiss != miss {
		t.Fatalf("the gap fill changed the output:\n--- miss ---\n%s\n--- full scan ---\n%s", miss, plainMiss)
	}
	if n := layerCount(t, dir); n != 2 {
		t.Fatalf("the gap fill must add one layer, got %d", n)
	}

	// Remove every layer: the next read is cold again and must print the same.
	if err := os.RemoveAll(recordsPath(dir)); err != nil {
		t.Fatal(err)
	}
	rebuilt := cacheOutput(t, "path", "--all", "--json")
	if rebuilt != miss {
		t.Fatalf("rebuilding the cache changed the output:\n--- miss ---\n%s\n--- rebuilt ---\n%s", miss, rebuilt)
	}

	// --no-cache must answer exactly like the cached path.
	off := cacheOutput(t, "path", "--all", "--json", "--no-cache")
	if off != miss {
		t.Fatalf("--no-cache changed the output:\n--- cached ---\n%s\n--- no-cache ---\n%s", miss, off)
	}
}

func TestCacheCoversEveryReadCommand(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	commitAction(t, dir, "beta", "ask", "乙单", "2026-10-02T10:00:00+08:00")
	t.Chdir(dir)

	for _, args := range [][]string{
		{"list", "--json"},
		{"check", "--json"},
		{"path", "--all", "--json"},
		{"status"},
		{"graph", "--json"},
		{"show", "alpha", "1", "--json"},
	} {
		plain := cacheOutput(t, append(args, "--no-cache")...)
		_ = cacheOutput(t, args...)
		cached := cacheOutput(t, args...)
		if plain != cached {
			t.Fatalf("%v: cached output differs from --no-cache\n--- plain ---\n%s\n--- cached ---\n%s", args, plain, cached)
		}
	}
}

func TestCacheStaysOffUnlessConfigured(t *testing.T) {
	dir := cacheTestRepo(t, false)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	cacheOutput(t, "list", "--json")
	if _, err := os.Stat(recordsPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("a file without the cache key must not create state: %v", err)
	}
}

func TestNoCacheFlagLeavesNoState(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	cacheOutput(t, "list", "--json", "--no-cache")
	if _, err := os.Stat(recordsPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("--no-cache must not create state: %v", err)
	}
}

func TestCacheIgnoresCorruptLayer(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)

	want := cacheOutput(t, "list", "--json")
	entries, err := os.ReadDir(recordsPath(dir))
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected a layer: %v", err)
	}
	layer := filepath.Join(recordsPath(dir), entries[0].Name())
	data, err := os.ReadFile(layer)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 0xff
	if err := os.WriteFile(layer, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("a corrupt layer must fall back to a full scan, output changed:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestActionLogCachedMatchesTheStream(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "beta", "ask", "乙单", "2026-10-01T11:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	commitAction(t, dir, "beta", "collect", "调研", "2026-10-02T10:00:00+08:00")
	t.Chdir(dir)

	// A cold action log streams and leaves no layer behind; it must still print
	// exactly what the stream prints.
	coldText := cacheOutput(t, "action", "log")
	coldJSON := cacheOutput(t, "action", "log", "--jsonl")
	if n := layerCount(t, dir); n != 0 {
		t.Fatalf("a cold action log must not write a layer, got %d", n)
	}

	// A full read of another command populates the layer; the cached action log
	// must produce the same bytes.
	_ = cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("warm-up must write one layer, got %d", n)
	}
	if got := cacheOutput(t, "action", "log"); got != coldText {
		t.Fatalf("cached action log text differs:\n--- stream ---\n%s\n--- cache ---\n%s", coldText, got)
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != coldJSON {
		t.Fatalf("cached action log jsonl differs:\n--- stream ---\n%s\n--- cache ---\n%s", coldJSON, got)
	}

	// --limit keeps streaming (a first screen is cheaper than decoding a layer).
	if got, want := cacheOutput(t, "action", "log", "--limit", "2"), cacheOutput(t, "action", "log", "--limit", "2", "--no-cache"); got != want {
		t.Fatalf("limited action log differs:\n--- cached ---\n%s\n--- no-cache ---\n%s", got, want)
	}
}

func TestActionLogGapFillMatchesTheStream(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "action", "log")
	_ = cacheOutput(t, "list", "--json")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	if got := cacheOutput(t, "action", "log"); got == want {
		t.Fatal("the new action must show up")
	}
	if got, want2 := cacheOutput(t, "action", "log"), cacheOutput(t, "action", "log", "--no-cache"); got != want2 {
		t.Fatalf("gap fill changed the stream:\n--- cached ---\n%s\n--- no-cache ---\n%s", got, want2)
	}
}

func totalRecords(t *testing.T, dir string) int {
	t.Helper()
	layers, err := cache.Scan(recordsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range layers {
		n += len(l.Records)
	}
	return n
}

func TestCacheSyncFillsTheLayerAndIsIdempotent(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	cacheOutput(t, "cache", "sync")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("sync must write one layer, got %d", n)
	}
	if recs := totalRecords(t, dir); recs != 1 {
		t.Fatalf("sync must record every commit, got %d", recs)
	}
	cacheOutput(t, "cache", "sync")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("a second sync must be a no-op, got %d layers", n)
	}
}

func TestCacheGCDropsUnreachableRecords(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")
	if recs := totalRecords(t, dir); recs != 2 {
		t.Fatalf("warm-up must record two commits, got %d", recs)
	}

	// Amend the last commit: its old OID becomes unreachable, but its record stays.
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-10-03T09:00:00+08:00", "GIT_COMMITTER_DATE=2026-10-03T09:00:00+08:00",
	}
	cacheGit(t, dir, env, "落地改\n\nissue: alpha\naction: apply\n", "commit", "--amend", "--allow-empty", "-q", "-F", "-")
	afterAmend := cacheOutput(t, "list", "--json") // the read fills the gap
	if recs := totalRecords(t, dir); recs != 3 {
		t.Fatalf("the gap fill must record the amended commit, got %d", recs)
	}
	if afterAmend == want {
		t.Fatal("the amended action must change the output")
	}

	cacheOutput(t, "cache", "gc")
	if recs := totalRecords(t, dir); recs != 2 {
		t.Fatalf("gc must drop the unreachable record, got %d", recs)
	}
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("gc must merge into one layer, got %d", n)
	}
	if got := cacheOutput(t, "list", "--json"); got != afterAmend {
		t.Fatalf("gc changed the output:\n--- before ---\n%s\n--- after ---\n%s", afterAmend, got)
	}
}

func TestMergePrunesUnreachableRecords(t *testing.T) {
	old := maxLayers
	maxLayers = 2
	defer func() { maxLayers = old }()

	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")

	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-10-03T09:00:00+08:00", "GIT_COMMITTER_DATE=2026-10-03T09:00:00+08:00",
	}
	cacheGit(t, dir, env, "落地改\n\nissue: alpha\naction: apply\n", "commit", "--amend", "--allow-empty", "-q", "-F", "-")
	afterAmend := cacheOutput(t, "list", "--json")
	if afterAmend == want {
		t.Fatal("the amended action must change the output")
	}
	// Two layers now, so the next read merges; the merge sees three records for
	// two reachable commits and must drop the orphan without a gc.
	_ = cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("the merge must leave one layer, got %d", n)
	}
	if recs := totalRecords(t, dir); recs != 2 {
		t.Fatalf("the merge must prune the unreachable record, got %d", recs)
	}
	if got := cacheOutput(t, "list", "--json"); got != afterAmend {
		t.Fatalf("pruning changed the output:\n--- before ---\n%s\n--- after ---\n%s", afterAmend, got)
	}
}

func TestCacheIsSharedAcrossWorktrees(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("main worktree must write one layer, got %d", n)
	}

	wt := filepath.Join(t.TempDir(), "probe-wt")
	cacheGit(t, dir, nil, "", "worktree", "add", "-q", "-b", "probe-cache", wt)
	t.Chdir(wt)

	// The linked worktree reads the same history; a cached answer must be the
	// same as a full scan there too.
	if got, plain := cacheOutput(t, "list", "--json"), cacheOutput(t, "list", "--json", "--no-cache"); got != plain {
		t.Fatalf("worktree cache hit differs from a full scan:\n--- cached ---\n%s\n--- plain ---\n%s", got, plain)
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != cacheOutput(t, "action", "log", "--jsonl", "--no-cache") {
		t.Fatal("worktree action log differs from a full scan")
	}
	// One cache for the whole clone: nothing is written under the worktree's own
	// git dir (a file in a linked worktree, so a directory there would be wrong).
	if _, err := os.Stat(filepath.Join(wt, ".git", "kiln")); !os.IsNotExist(err) {
		t.Fatalf("a linked worktree must share the main cache: %v", err)
	}
	_ = want
}

func TestCacheWriteFailureIsSilent(t *testing.T) {
	dir := cacheTestRepo(t, true)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	// A file where the cache directory should go makes every write fail the way a
	// read-only checkout does. The command must not notice.
	if err := os.WriteFile(filepath.Join(dir, ".git", "kiln"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	plain := cacheOutput(t, "list", "--json", "--no-cache")
	if got := cacheOutput(t, "list", "--json"); got != plain {
		t.Fatalf("a failed write must not change the output:\n--- plain ---\n%s\n--- got ---\n%s", plain, got)
	}
	if got := cacheOutput(t, "list", "--json"); got != plain {
		t.Fatal("the second read must stay correct too")
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != cacheOutput(t, "action", "log", "--jsonl", "--no-cache") {
		t.Fatal("action log must stay correct when the cache cannot be written")
	}
}
