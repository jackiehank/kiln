package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The enumeration snapshot is the thing that lets a warm read skip git entirely,
// so it has to be exact: same bytes as git, keyed by everything that can change
// them, and never trusted when it is damaged or ambiguous.

func testKey(head, inv byte) enumKey {
	var k enumKey
	k.head[0] = head
	k.inv[0] = inv
	return k
}

func testOIDs(n int) [][20]byte {
	oids := make([][20]byte, n)
	for i := range oids {
		oids[i][0] = byte(i)
		oids[i][1] = byte(i >> 8)
	}
	return oids
}

func TestEnumEncodeDecodeRoundtrip(t *testing.T) {
	key := testKey(1, 2)
	want := testOIDs(37)
	got, ok := enumDecode(enumEncode(enumDefault, key, want), enumDefault, key)
	if !ok {
		t.Fatal("roundtrip must decode")
	}
	if len(got) != len(want) {
		t.Fatalf("count: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("oid %d: got %x want %x", i, got[i], want[i])
		}
	}
}

func TestEnumDecodeRejectsMismatchAndDamage(t *testing.T) {
	key := testKey(1, 2)
	good := enumEncode(enumDefault, key, testOIDs(5))
	flipped := append([]byte(nil), good...)
	flipped[60] ^= 0xff
	cases := []struct {
		name string
		data []byte
		kind enumKind
		key  enumKey
	}{
		{"other head", good, enumDefault, testKey(9, 2)},
		{"other invariants", good, enumDefault, testKey(1, 9)},
		{"other kind", good, enumAuthorDate, key},
		{"flipped byte", flipped, enumDefault, key},
		{"truncated", good[:len(good)-3], enumDefault, key},
		{"short", good[:20], enumDefault, key},
		{"empty", nil, enumDefault, key},
	}
	for _, c := range cases {
		if _, ok := enumDecode(c.data, c.kind, c.key); ok {
			t.Errorf("%s: must not decode", c.name)
		}
	}
}

func TestEnumStoreLoadAndPrune(t *testing.T) {
	dir := t.TempDir()
	// One snapshot per key: six heads, so the oldest must be pruned.
	for i := 0; i < 6; i++ {
		key := testKey(byte(i), 2)
		oids := testOIDs(3)
		storeEnum(dir, enumDefault, key, oids)
		if got, ok := loadEnum(dir, enumDefault, key); !ok || len(got) != 3 {
			t.Fatalf("snapshot %d must load: ok=%v len=%d", i, ok, len(got))
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != enumKeep {
		t.Fatalf("prune must keep %d snapshots, kept %d", enumKeep, len(entries))
	}
	// The newest key survives; the oldest does not.
	if _, ok := loadEnum(dir, enumDefault, testKey(5, 2)); !ok {
		t.Fatal("newest snapshot must survive")
	}
	if _, ok := loadEnum(dir, enumDefault, testKey(0, 2)); ok {
		t.Fatal("oldest snapshot must be pruned")
	}
}

func TestEnumDecodeKindAndHeadAreIndependent(t *testing.T) {
	dir := t.TempDir()
	key := testKey(7, 8)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The same head with a different order is a different file, not a stale hit.
	storeEnum(dir, enumDefault, key, testOIDs(4))
	storeEnum(dir, enumAuthorDate, key, testOIDs(5))
	if got, ok := loadEnum(dir, enumDefault, key); !ok || len(got) != 4 {
		t.Fatalf("default order: ok=%v len=%d", ok, len(got))
	}
	if got, ok := loadEnum(dir, enumAuthorDate, key); !ok || len(got) != 5 {
		t.Fatalf("author-date order: ok=%v len=%d", ok, len(got))
	}
}

// snapshotTestRepo builds a small history and returns the directory.
func snapshotTestRepo(t *testing.T, commits int) string {
	t.Helper()
	dir := cacheTestRepo(t)
	for i := 0; i < commits; i++ {
		issue := "s1"
		if i%2 == 1 {
			issue = "s2"
		}
		commitAction(t, dir, issue, "ask", "提交", "2026-10-0"+string(rune('1'+i))+"T10:00:00+08:00")
	}
	return dir
}

// noGit makes every later git invocation impossible: the PATH points at an empty
// directory. A command that still succeeds therefore did not start git.
func noGit(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func TestSnapshotAnswersWithoutGit(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	warm := cacheOutput(t, "graph", "--json") // cold read: scans, stores layer + snapshot
	again := cacheOutput(t, "graph", "--json")
	if warm != again {
		t.Fatal("cached read must not change the output")
	}
	noGit(t)
	got := cacheOutput(t, "graph", "--json")
	if got != warm {
		t.Fatal("a snapshot hit must print the same bytes as the git read")
	}
	// And the snapshot is what made that possible: without the cache the same
	// environment cannot answer at all.
	cacheOff = true
	defer func() { cacheOff = false }()
	var out, errb bytes.Buffer
	if code := Run([]string{"graph", "--json"}, &out, &errb); code == 0 {
		t.Fatal("a cold read must need git, so it must fail without it")
	}
}

func TestSnapshotFollowsHead(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	check := func(label string) {
		t.Helper()
		warm := cacheOutput(t, "graph", "--json")
		plain := uncached(t, "graph", "--json")
		if warm != plain {
			t.Fatalf("%s: snapshot output differs from the full scan", label)
		}
	}
	check("warm")
	commitAction(t, dir, "s1", "collect", "新提交", "2026-10-09T10:00:00+08:00")
	check("after a new commit")
	// A runner without a global user.name/user.email must still be able to amend,
	// so the identity travels with the command.
	cacheGit(t, dir, []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	}, "", "commit", "--amend", "--allow-empty", "-q", "-m", "改写\n\nissue: s1\naction: test\n")
	check("after amend")
	cacheGit(t, dir, nil, "", "reset", "-q", "--hard", "HEAD~1")
	check("after reset")
}

func TestSnapshotFallsBackOnReplaceRefs(t *testing.T) {
	dir := snapshotTestRepo(t, 2)
	t.Chdir(dir)
	cacheOutput(t, "graph", "--json") // stores a snapshot for this HEAD
	// A replace ref can change what git reports without changing HEAD, so a
	// snapshot must not be trusted while one exists — even a bogus file.
	refs := filepath.Join(dir, ".git", "refs", "replace")
	if err := os.MkdirAll(refs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refs, "deadbeef"), []byte("0000000000000000000000000000000000000000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noGit(t)
	var out, errb bytes.Buffer
	if code := Run([]string{"graph", "--json"}, &out, &errb); code == 0 {
		t.Fatal("with replace refs present the read must ask git, so it must fail without it")
	}
}

func TestSnapshotFallsBackOnPackedReplaceRefs(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	// Warm both orders: pack-refs alone does not change HEAD, so only the
	// invalidation digest can tell that a replace ref appeared.
	_ = cacheOutput(t, "list", "--json")
	_ = cacheOutput(t, "action", "log")
	cacheGit(t, dir, nil, "", "replace", "--graft", "HEAD")
	cacheGit(t, dir, nil, "", "pack-refs", "--all")
	want := uncached(t, "action", "log")
	if got := cacheOutput(t, "action", "log"); got != want {
		t.Fatalf("a packed replace ref must invalidate the snapshot:\n--- cached ---\n%s\n--- git ---\n%s", got, want)
	}
}

func TestCorruptSnapshotIsRebuilt(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	want := cacheOutput(t, "list", "--json")
	enums := enumFiles(t, dir)
	if len(enums) != 1 {
		t.Fatalf("a warm read must store one snapshot, got %d", len(enums))
	}
	data, err := os.ReadFile(enums[0])
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 0xff // same size, broken checksum
	if err := os.WriteFile(enums[0], data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := cacheOutput(t, "list", "--json"); got != want {
		t.Fatalf("a corrupt snapshot must fall back to git, output changed:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
	fixed, err := os.ReadFile(enums[0])
	if err != nil {
		t.Fatal(err)
	}
	var key enumKey
	copy(key.head[:], fixed[8:28])
	copy(key.inv[:], fixed[28:48])
	if _, ok := enumDecode(fixed, enumKind(fixed[7]), key); !ok {
		t.Fatal("the snapshot must be rewritten, not left undecodable forever")
	}
}

// enumFiles lists the snapshots a repository has stored.
func enumFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".git", "kiln", "enums"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "enum-") && strings.HasSuffix(e.Name(), ".oid") {
			out = append(out, filepath.Join(dir, ".git", "kiln", "enums", e.Name()))
		}
	}
	return out
}

func TestSnapshotSilentWithoutWritableCache(t *testing.T) {
	dir := snapshotTestRepo(t, 2)
	t.Chdir(dir)
	// A read-only cache (simulated by making the cache directory a file) must not
	// change what the command prints; it just keeps asking git.
	if err := os.WriteFile(filepath.Join(dir, ".git", "kiln"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	warm := cacheOutput(t, "graph", "--json")
	plain := uncached(t, "graph", "--json")
	if warm != plain {
		t.Fatal("an unwritable cache must not change the output")
	}
}

// The snapshot is keyed by a HEAD resolved by reading files, so every ref shape
// git can use has to resolve the same way - and a shape kiln cannot read must
// fall back to git instead of guessing.
func TestSnapshotPackedRefs(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	cacheOutput(t, "graph", "--json")
	cacheGit(t, dir, nil, "", "pack-refs", "--all", "--prune")
	want := uncached(t, "graph", "--json")
	noGit(t)
	if got := cacheOutput(t, "graph", "--json"); got != want {
		t.Fatal("a packed ref must still resolve from the filesystem")
	}
}

func TestSnapshotDetachedHead(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	cacheGit(t, dir, nil, "", "checkout", "-q", "--detach", "HEAD~1")
	warm := cacheOutput(t, "graph", "--json")
	plain := uncached(t, "graph", "--json")
	if warm != plain {
		t.Fatal("a detached HEAD must read the same as a full scan")
	}
}

func TestSnapshotSharedByWorktrees(t *testing.T) {
	dir := snapshotTestRepo(t, 3)
	t.Chdir(dir)
	cacheOutput(t, "graph", "--json")
	wt := filepath.Join(t.TempDir(), "ws")
	cacheGit(t, dir, nil, "", "worktree", "add", "-q", "-b", "enum-wt", wt, "HEAD~1")
	t.Chdir(wt)
	warm := cacheOutput(t, "graph", "--json")
	plain := uncached(t, "graph", "--json")
	if warm != plain {
		t.Fatal("a linked worktree must read the same as a full scan")
	}
}
