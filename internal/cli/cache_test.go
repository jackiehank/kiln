package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
