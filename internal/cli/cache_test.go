package cli

import (
	"bytes"
	"encoding/hex"
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

// TestActionLogColdSeedsTheCache pins the seeding behaviour: a cold action log
// reads every message, so it leaves the records behind and stores the author-date
// enumeration. The next call must be answerable without git, with the same bytes.
func TestActionLogColdSeedsTheCache(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitPlain(t, dir, "不是动作", "2026-10-01T12:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)

	cold := cacheOutput(t, "action", "log", "--jsonl")
	if n := layerCount(t, dir); n == 0 {
		t.Fatal("a cold action log must seed the cache")
	}
	if n := len(enumFiles(t, dir)); n != 1 {
		t.Fatalf("a complete stream must store one snapshot, got %d", n)
	}

	// With git unreachable only the cache can answer; the bytes must not move.
	t.Setenv("PATH", t.TempDir())
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != cold {
		t.Fatalf("cached stream differs:\n--- stream ---\n%s\n--- cache ---\n%s", cold, got)
	}
}

// TestActionLogEarlyStopWritesNoSnapshot pins the other half: a peek that stopped
// early may keep the prefix (those records are true), but it must not claim a
// complete enumeration it never walked.
func TestActionLogEarlyStopWritesNoSnapshot(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)

	_ = cacheOutput(t, "action", "log", "--limit", "1")
	if n := len(enumFiles(t, dir)); n != 0 {
		t.Fatalf("an early stop must not store a snapshot, got %d", n)
	}
	if n := layerCount(t, dir); n == 0 {
		t.Fatal("an early stop must keep the prefix it read")
	}

	// A second peek still streams: nothing pretends the enumeration is known.
	_ = cacheOutput(t, "action", "log", "--limit", "1")
	if n := len(enumFiles(t, dir)); n != 0 {
		t.Fatalf("a peek must not create a snapshot, got %d", n)
	}
}

// TestChunkedLayersReadIdentical pins the chunking boundary: when one read has to
// write more than one layer, the model it produces is still the full scan's.
func TestChunkedLayersReadIdentical(t *testing.T) {
	old := layerChunkRecords
	layerChunkRecords = 2
	defer func() { layerChunkRecords = old }()

	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "beta", "ask", "乙单", "2026-10-01T11:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	commitAction(t, dir, "beta", "collect", "调研", "2026-10-02T10:00:00+08:00")
	t.Chdir(dir)

	want := cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n < 2 {
		t.Fatalf("a 2-record chunk must split 4 commits into more than one layer, got %d", n)
	}
	if got := uncached(t, "list", "--json"); got != want {
		t.Fatalf("chunked read differs from the full scan:\n--- full scan ---\n%s\n--- chunked ---\n%s", got, want)
	}
	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("hot read differs:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

// TestStreamedRecordsCoverTheHistory pins that the stream records every commit,
// not only the actions: the enumeration names all of them, so a layer that skipped
// plain commits would leave permanent gaps.
func TestStreamedRecordsCoverTheHistory(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitPlain(t, dir, "普通提交", "2026-10-01T12:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)

	_ = cacheOutput(t, "action", "log", "--jsonl")
	if got, want := totalRecords(t, dir), 3; got != want {
		t.Fatalf("the stream must record every commit, got %d want %d", got, want)
	}

	before := layerCount(t, dir)
	want := cacheOutput(t, "list", "--json")
	if after := layerCount(t, dir); after != before {
		t.Fatalf("the model read must reuse the seeded records: %d → %d layers", before, after)
	}
	if got := uncached(t, "list", "--json"); got != want {
		t.Fatalf("seeded records changed the model:\n--- full scan ---\n%s\n--- seeded ---\n%s", got, want)
	}
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

	// A cold action log streams and seeds the cache: it must print exactly what
	// the stream prints, and leave the records behind for the next read.
	coldText := cacheOutput(t, "action", "log")
	coldJSON := cacheOutput(t, "action", "log", "--jsonl")
	if n := layerCount(t, dir); n == 0 {
		t.Fatal("a cold action log must seed the cache")
	}

	// The same cache now answers both shapes from the seeded records; the bytes
	// must not have moved.
	if got := cacheOutput(t, "action", "log"); got != coldText {
		t.Fatalf("cached action log text differs:\n--- stream ---\n%s\n--- cache ---\n%s", coldText, got)
	}
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != coldJSON {
		t.Fatalf("cached action log jsonl differs:\n--- stream ---\n%s\n--- cache ---\n%s", coldJSON, got)
	}

	// A full model read reuses the seeded records: it may add the default
	// enumeration snapshot, but it must not write another layer.
	_ = cacheOutput(t, "list", "--json")
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("the seeded records must be reused, got %d layers", n)
	}

	// --limit takes the cached author-date enumeration, since the full walk it
	// replaces is the expensive part on a big repository; the rows must still be
	// the ones the stream prints.
	if got, want := cacheOutput(t, "action", "log", "--limit", "2"), uncached(t, "action", "log", "--limit", "2"); got != want {
		t.Fatalf("limited action log differs:\n--- cached ---\n%s\n--- cold ---\n%s", got, want)
	}
}

// TestActionLogLimitServesTheFirstScreenWithoutGit pins what the --limit path is
// for: once the author-date snapshot and the layer exist, the first screen comes
// from the cache, so it works with no git process at all. Before the snapshot was
// consulted, --limit always streamed, which needed a git child per invocation.
func TestActionLogLimitServesTheFirstScreenWithoutGit(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "beta", "ask", "乙单", "2026-10-01T11:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-02T09:00:00+08:00")
	t.Chdir(dir)
	// Warm the layer and both enumeration snapshots.
	_ = cacheOutput(t, "list", "--json")
	_ = cacheOutput(t, "action", "log", "--jsonl")
	want := cacheOutput(t, "action", "log", "--limit", "2")

	// With git unreachable, only a cache answer is possible.
	t.Setenv("PATH", t.TempDir())
	if got := cacheOutput(t, "action", "log", "--limit", "2"); got != want {
		t.Fatalf("cached first screen differs:\n--- warm ---\n%s\n--- no git ---\n%s", want, got)
	}
}

// TestDuplicateStartCheckAnswersFromTheCache pins the other read a write command
// used to do: `do`/`commit` with a start action asks "does this issue already
// have one?", and that used to read the whole history (`git log`, every message)
// even when a warm layer already held the answer. With git unreachable, only a
// cache answer is possible — and it must refuse the duplicate while letting an
// issue without a start action through.
func TestDuplicateStartCheckAnswersFromTheCache(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "beta", "collect", "只收", "2026-10-01T11:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json") // warm the layer and the enumeration

	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	if code := Run([]string{"do", "alpha", "ask", "--title", "再来一次"}, &out, &errb); code != 1 {
		t.Fatalf("duplicate start: exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "已经有起点动作") {
		t.Fatalf("duplicate start must be refused: %q", errb.String())
	}

	out.Reset()
	errb.Reset()
	if code := Run([]string{"do", "beta", "ask", "--title", "开新单"}, &out, &errb); code != 0 {
		t.Fatalf("a collect-only issue may open: exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "issue: beta") {
		t.Fatalf("message missing: %q", out.String())
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

// commitPlain writes a commit with no kiln trailers: the record layer still
// holds it, but `action log` renders no row for it.
func commitPlain(t *testing.T, dir, subject, when string) {
	t.Helper()
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=" + when, "GIT_COMMITTER_DATE=" + when,
	}
	cacheGit(t, dir, env, subject+"\n", "commit", "--allow-empty", "-q", "-F", "-")
}

func TestActionLogMergePrunesWithoutRenderingRows(t *testing.T) {
	old := maxLayers
	maxLayers = 2
	defer func() { maxLayers = old }()

	dir := cacheTestRepo(t)
	commitPlain(t, dir, "普通提交一", "2026-10-01T10:00:00+08:00")
	commitPlain(t, dir, "普通提交二", "2026-10-02T10:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json")
	if recs := totalRecords(t, dir); recs != 2 {
		t.Fatalf("warm-up must record two commits, got %d", recs)
	}

	// Amend: the old head becomes unreachable, but its record stays behind.
	cacheGit(t, dir, []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-10-03T09:00:00+08:00", "GIT_COMMITTER_DATE=2026-10-03T09:00:00+08:00",
	}, "改过的普通提交\n", "commit", "--amend", "--allow-empty", "-q", "-F", "-")

	// No commit is an action, so `action log` renders nothing. It still writes a
	// layer and reaches the merge threshold: the ladder must count the
	// enumeration, not the zero rows it printed.
	if got := cacheOutput(t, "action", "log"); strings.TrimSpace(got) != "" {
		t.Fatalf("plain commits must render no rows, got %q", got)
	}
	if n := layerCount(t, dir); n != 1 {
		t.Fatalf("the merge must leave one layer, got %d", n)
	}
	if recs := totalRecords(t, dir); recs != 2 {
		t.Fatalf("the merge must prune the orphan with no rows rendered, got %d records", recs)
	}
}

// cacheGitOut runs git and returns its stdout.
func cacheGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestReadSweepsStaleTempFiles(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json")

	stale := filepath.Join(recordsPath(dir), "layer-"+strings.Repeat("0", 40)+".rec.tmp-1")
	fresh := filepath.Join(recordsPath(dir), "layer-"+strings.Repeat("1", 40)+".rec.tmp-2")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("torn"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	_ = cacheOutput(t, "list", "--json")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("a read must sweep a stale temp file: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("a read must leave a live temp file alone: %v", err)
	}
}

func TestReadDropsForeignLayersOnlyAfterTheGracePeriod(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")

	layers, err := cache.Scan(recordsPath(dir))
	if err != nil || len(layers) != 1 {
		t.Fatalf("expected one layer: %v", err)
	}
	ours := layers[0]
	old := time.Now().Add(-8 * 24 * time.Hour)
	// Our own layer is kept however old it is: age only decides the fate of the
	// layers this binary cannot read.
	if err := os.Chtimes(ours.Path, old, old); err != nil {
		t.Fatal(err)
	}

	foreign := func(seed byte) string {
		var fp [20]byte
		fp[0] = seed
		name, werr := cache.Write(recordsPath(dir), ours.Records, ours.Tips, fp)
		if werr != nil {
			t.Fatal(werr)
		}
		return filepath.Join(recordsPath(dir), name)
	}
	agedPath := foreign(0xAA)
	if err := os.Chtimes(agedPath, old, old); err != nil {
		t.Fatal(err)
	}
	youngPath := foreign(0xBB)

	_ = cacheOutput(t, "list", "--json")

	if _, err := os.Stat(ours.Path); err != nil {
		t.Fatalf("a layer of this vocabulary must survive any age: %v", err)
	}
	if _, err := os.Stat(agedPath); !os.IsNotExist(err) {
		t.Fatalf("a foreign layer older than %v must be dropped: %v", foreignGrace, err)
	}
	if _, err := os.Stat(youngPath); err != nil {
		t.Fatalf("a fresh foreign layer must stay for the other version: %v", err)
	}
	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("pruning changed the output:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestMergeDropsUnreachableSnapshots(t *testing.T) {
	old := maxLayers
	maxLayers = 1
	defer func() { maxLayers = old }()

	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	commitAction(t, dir, "alpha", "collect", "调研", "2026-10-02T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-03T10:00:00+08:00")
	t.Chdir(dir)
	_ = cacheOutput(t, "list", "--json")
	if len(enumFiles(t, dir)) == 0 {
		t.Fatal("the warm read must store a snapshot")
	}

	// Drop the newest commit: its HEAD is no ref's ancestor any more, and the
	// next read merges (maxLayers=1) and computes reachability for the records.
	cacheGit(t, dir, nil, "", "reset", "-q", "--hard", "HEAD~1")
	_ = cacheOutput(t, "list", "--json")

	files := enumFiles(t, dir)
	if len(files) != 1 {
		t.Fatalf("the merge must drop the unreachable snapshot, %d left", len(files))
	}
	data, err := os.ReadFile(files[0])
	if err != nil || len(data) < 28 {
		t.Fatalf("snapshot unreadable: %v", err)
	}
	head := strings.TrimSpace(hex.EncodeToString(data[8:28]))
	if head != strings.TrimSpace(cacheGitOut(t, dir, "rev-parse", "HEAD")) {
		t.Fatalf("the surviving snapshot must belong to HEAD, got %s", head)
	}
}

// The shared snapshot is the scheme under test: a linear history prints both of
// git's orders identically, so one snapshot is honest for both. These tests pin
// the proof, the mechanism and its limit.

// commitAt writes one action with an author date and a committer date that may
// differ: the two git orderings look at different fields, so a history where the
// two dates disagree is where the orders could part ways.
func commitAt(t *testing.T, dir, issue, kind, title, author, committer string) {
	t.Helper()
	msg := fmt.Sprintf("%s\n\nissue: %s\naction: %s\n", title, issue, kind)
	env := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=" + author, "GIT_COMMITTER_DATE=" + committer,
	}
	cacheGit(t, dir, env, msg, "commit", "--allow-empty", "-q", "-F", "-")
}

// mergeRepo builds the shape the shared snapshot must refuse: one merge, with the
// dates arranged so the two orders genuinely disagree.
func mergeRepo(t *testing.T) string {
	t.Helper()
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-01T10:00:00+08:00")
	branch := strings.TrimSpace(cacheGitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	// Main's tip is committed late (10-11) but authored early (10-02).
	commitAt(t, dir, "alpha", "collect", "主线", "2026-10-02T10:00:00+08:00", "2026-10-11T10:00:00+08:00")
	cacheGit(t, dir, nil, "", "checkout", "-q", "-b", "feature", "HEAD~1")
	commitAt(t, dir, "alpha", "test", "分支", "2026-10-09T10:00:00+08:00", "2026-10-09T10:00:00+08:00")
	cacheGit(t, dir, nil, "", "checkout", "-q", branch)
	cacheGit(t, dir, []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-10-12T10:00:00+08:00", "GIT_COMMITTER_DATE=2026-10-12T10:00:00+08:00",
	}, "", "merge", "-q", "--no-ff", "feature", "-m", "merge")
	return dir
}

// snapshotKinds is the kind byte of every stored snapshot.
func snapshotKinds(t *testing.T, dir string) []byte {
	t.Helper()
	var kinds []byte
	for _, p := range enumFiles(t, dir) {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 8 {
			t.Fatalf("snapshot %s is too short", p)
		}
		kinds = append(kinds, data[7])
	}
	return kinds
}

// TestLogOrdersDifferOnlyWithMerges pins the git behaviour the scheme rests on:
// the two orderings are the same sequence exactly while the history is linear.
// A different git would fail here rather than let a shared snapshot lie.
func TestLogOrdersDifferOnlyWithMerges(t *testing.T) {
	lin := cacheTestRepo(t)
	commitAction(t, lin, "alpha", "ask", "开单", "2026-10-03T10:00:00+08:00")
	commitAction(t, lin, "alpha", "collect", "调研", "2026-10-02T10:00:00+08:00")
	commitAction(t, lin, "alpha", "apply", "落地", "2026-10-01T10:00:00+08:00")
	if a, b := cacheGitOut(t, lin, "log", "--format=%H"), cacheGitOut(t, lin, "log", "--author-date-order", "--format=%H"); a != b {
		t.Fatal("a linear history must print both orders identically")
	}

	mer := mergeRepo(t)
	a := cacheGitOut(t, mer, "log", "--format=%H")
	b := cacheGitOut(t, mer, "log", "--author-date-order", "--format=%H")
	if a == b {
		t.Fatal("this merge history is built so the two orders differ; git printed one sequence")
	}
}

// TestSharedSnapshotServesBothOrders starts from the model path: one cold List
// files a single shared snapshot, and the author-date command is then answerable
// with no git process at all.
func TestSharedSnapshotServesBothOrders(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-03T10:00:00+08:00")
	commitAction(t, dir, "alpha", "collect", "调研", "2026-10-02T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	wantLog := uncached(t, "action", "log", "--jsonl")

	_ = cacheOutput(t, "list", "--json")
	if kinds := snapshotKinds(t, dir); len(kinds) != 1 || kinds[0] != byte(enumBoth) {
		t.Fatalf("a linear history must file one shared snapshot, got kinds %v", kinds)
	}

	t.Setenv("PATH", t.TempDir())
	if got := cacheOutput(t, "action", "log", "--jsonl"); got != wantLog {
		t.Fatalf("the shared snapshot must serve the author-date order:\n--- git ---\n%s\n--- cache ---\n%s", wantLog, got)
	}
}

// TestSharedSnapshotSeededByTheStream is the other direction: a cold action log
// files the shared snapshot, and the model read is then answerable without git.
func TestSharedSnapshotSeededByTheStream(t *testing.T) {
	dir := cacheTestRepo(t)
	commitAction(t, dir, "alpha", "ask", "开单", "2026-10-03T10:00:00+08:00")
	commitAction(t, dir, "alpha", "collect", "调研", "2026-10-02T10:00:00+08:00")
	commitAction(t, dir, "alpha", "apply", "落地", "2026-10-01T10:00:00+08:00")
	t.Chdir(dir)
	wantList := uncached(t, "list", "--json")

	_ = cacheOutput(t, "action", "log", "--jsonl")
	if kinds := snapshotKinds(t, dir); len(kinds) != 1 || kinds[0] != byte(enumBoth) {
		t.Fatalf("a linear history must file one shared snapshot, got kinds %v", kinds)
	}

	t.Setenv("PATH", t.TempDir())
	if got := cacheOutput(t, "list", "--json"); got != wantList {
		t.Fatalf("the shared snapshot must serve the model order:\n--- git ---\n%s\n--- cache ---\n%s", wantList, got)
	}
}

// TestMergeRepoDoesNotShareSnapshots is the limit: with a merge in the walked
// history the two orders may differ, so the reader must still go to git.
func TestMergeRepoDoesNotShareSnapshots(t *testing.T) {
	dir := mergeRepo(t)
	t.Chdir(dir)

	_ = cacheOutput(t, "list", "--json")
	if kinds := snapshotKinds(t, dir); len(kinds) != 1 || kinds[0] != byte(enumDefault) {
		t.Fatalf("a merge history must keep its own default-order snapshot, got kinds %v", kinds)
	}

	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	if code := Run([]string{"action", "log", "--jsonl"}, &out, &errb); code == 0 {
		t.Fatal("without a shared snapshot the author-date order must need git")
	}
}
