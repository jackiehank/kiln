package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackiehank/kiln/internal/cache"
)

// These tests are the acceptance condition the design puts first: a read command
// must print the same bytes whether it answers from the cache, from a full scan,
// or with the cache disabled. They run against a real git repository.

// cacheTestRepo builds a repository. The cache is background behaviour now, so
// there is nothing to switch on.
func cacheTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cacheGit(t, dir, nil, "", "init", "-q")
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

// uncached answers args with the record cache switched off: the oracle every
// cached assertion is checked against. The switch is internal — a user repairs a
// cache with `kiln cache clear`, not by running the tool uncached.
func uncached(t *testing.T, args ...string) string {
	t.Helper()
	cacheOff = true
	defer func() { cacheOff = false }()
	return cacheOutput(t, args...)
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
	dir := cacheTestRepo(t)
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
	if plainMiss := uncached(t, "path", "--all", "--json"); plainMiss != miss {
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

	// The uncached answer must be the same one the cache gave.
	off := uncached(t, "path", "--all", "--json")
	if off != miss {
		t.Fatalf("the cold path changed the output:\n--- cached ---\n%s\n--- cold ---\n%s", miss, off)
	}
}

func TestCacheCoversEveryReadCommand(t *testing.T) {
	dir := cacheTestRepo(t)
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
		plain := uncached(t, args...)
		_ = cacheOutput(t, args...)
		cached := cacheOutput(t, args...)
		if plain != cached {
			t.Fatalf("%v: cached output differs from the cold path\n--- plain ---\n%s\n--- cached ---\n%s", args, plain, cached)
		}
	}
}

func TestColdReadLeavesNoState(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	_ = uncached(t, "list", "--json")
	if _, err := os.Stat(recordsPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("a cold read must not create state: %v", err)
	}
}

func TestCacheIgnoresCorruptLayer(t *testing.T) {
	dir := cacheTestRepo(t)
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
	dir := cacheTestRepo(t)
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
	if got, want := cacheOutput(t, "action", "log", "--limit", "2"), uncached(t, "action", "log", "--limit", "2"); got != want {
		t.Fatalf("limited action log differs:\n--- cached ---\n%s\n--- cold ---\n%s", got, want)
	}
}

func TestActionLogGapFillMatchesTheStream(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "action", "log")
	_ = cacheOutput(t, "list", "--json")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	if got := cacheOutput(t, "action", "log"); got == want {
		t.Fatal("the new action must show up")
	}
	if got, want2 := cacheOutput(t, "action", "log"), uncached(t, "action", "log"); got != want2 {
		t.Fatalf("gap fill changed the stream:\n--- cached ---\n%s\n--- cold ---\n%s", got, want2)
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

func TestCacheGCDropsUnreachableRecords(t *testing.T) {
	dir := cacheTestRepo(t)
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

func TestCacheGCSweepsTornTempFiles(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json")

	torn := filepath.Join(recordsPath(dir), "layer-"+strings.Repeat("0", 40)+".rec.tmp-4242")
	if err := os.WriteFile(torn, []byte("half a layer"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(torn, old, old); err != nil {
		t.Fatal(err)
	}
	// A temp file a writer might still be using must survive the sweep.
	live := filepath.Join(recordsPath(dir), "layer-"+strings.Repeat("1", 40)+".rec.tmp-4243")
	if err := os.WriteFile(live, []byte("in flight"), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheOutput(t, "cache", "gc")
	if _, err := os.Stat(torn); !os.IsNotExist(err) {
		t.Fatalf("gc must sweep a stale temp file: %v", err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("gc must not touch a fresh temp file: %v", err)
	}
}

func TestMergePrunesUnreachableRecords(t *testing.T) {
	old := maxLayers
	maxLayers = 2
	defer func() { maxLayers = old }()

	dir := cacheTestRepo(t)
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
	dir := cacheTestRepo(t)
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
	if got, plain := cacheOutput(t, "list", "--json"), uncached(t, "list", "--json"); got != plain {
		t.Fatalf("worktree cache hit differs from a full scan:\n--- cached ---\n%s\n--- plain ---\n%s", got, plain)
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != uncached(t, "action", "log", "--jsonl") {
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
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	// A file where the cache directory should go makes every write fail the way a
	// read-only checkout does. The command must not notice.
	if err := os.WriteFile(filepath.Join(dir, ".git", "kiln"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	plain := uncached(t, "list", "--json")
	if got := cacheOutput(t, "list", "--json"); got != plain {
		t.Fatalf("a failed write must not change the output:\n--- plain ---\n%s\n--- got ---\n%s", plain, got)
	}
	if got := cacheOutput(t, "list", "--json"); got != plain {
		t.Fatal("the second read must stay correct too")
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != uncached(t, "action", "log", "--jsonl") {
		t.Fatal("action log must stay correct when the cache cannot be written")
	}
}

func TestRewriteSignalCleansWithoutWaitingForTheLayerCount(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json")
	if recs := totalRecords(t, dir); recs != 1 {
		t.Fatalf("warm-up must record one commit, got %d", recs)
	}

	// Rewrite the only commit: every enumerated OID then misses, which is the
	// free "history was rewritten" signal. The read must merge and prune on the
	// spot, without waiting for maxLayers layers to pile up.
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-10-02T09:00:00+08:00", "GIT_COMMITTER_DATE=2026-10-02T09:00:00+08:00",
	}
	cacheGit(t, dir, env, "改过的开单\n\nissue: alpha\naction: ask\n", "commit", "--amend", "--allow-empty", "-q", "-F", "-")
	want := cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("the rewrite signal must leave one layer, got %d", n)
	}
	if recs := totalRecords(t, dir); recs != 1 {
		t.Fatalf("the rewrite signal must prune the old record, got %d", recs)
	}
	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("cleanup changed the output:\n--- before ---\n%s\n--- after ---\n%s", want, got)
	}
}

func TestCacheClearRebuildsFromScratch(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("warm-up must write one layer, got %d", n)
	}

	if got := cacheOutput(t, "cache", "clear"); !strings.Contains(got, "已清空缓存") {
		t.Fatalf("clear must say what it did: %q", got)
	}
	if n := layerCount(t, dir); n != 0 {
		t.Fatalf("clear must delete every layer, %d left", n)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, ".git", "kiln", "enums")); err == nil && len(entries) != 0 {
		t.Fatalf("clear must delete every snapshot, %d left", len(entries))
	}

	// Clearing is not a state: the next read rebuilds with the same bytes.
	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("a rebuild changed the output:\n--- before ---\n%s\n--- after ---\n%s", want, got)
	}
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("the next read must rebuild one layer, got %d", n)
	}
	if len(enumFiles(t, dir)) == 0 {
		t.Fatal("the rebuild must leave a snapshot")
	}

	// Clearing twice is not an error: the second one reports an empty cache.
	_ = cacheOutput(t, "cache", "clear")
	if got := cacheOutput(t, "cache", "clear"); !strings.Contains(got, "缓存是空的") {
		t.Fatalf("clearing an empty cache must be quiet: %q", got)
	}
	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatal("clearing an empty cache must not change the output")
	}
}

func TestNoCacheFlagIsRejected(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)

	var out, errb bytes.Buffer
	if code := Run([]string{"list", "--no-cache"}, &out, &errb); code != 0 {
		t.Fatalf("an unknown flag must only warn, got exit %d", code)
	}
	if !strings.Contains(errb.String(), "未知 flag --no-cache") {
		t.Fatalf("--no-cache must be reported as unknown: %q", errb.String())
	}
}
