package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func mkdir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
}

// In a colocated jj workspace both markers exist; jj must win, or kiln would
// silently read git's view and miss jj.
func TestDetectAutoPrefersJJ(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, dir, ".git")
	mkdir(t, dir, ".jj")
	if kind, err := Detect(dir, "auto"); err != nil || kind != JJ {
		t.Fatalf("Detect = %q, %v; want jj", kind, err)
	}
}

func TestDetectAutoFallsBackToGit(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, dir, ".git")
	if kind, err := Detect(dir, ""); err != nil || kind != Git {
		t.Fatalf("Detect = %q, %v; want git", kind, err)
	}
}

func TestDetectHonoursExplicitPreference(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, dir, ".git")
	mkdir(t, dir, ".jj")
	if kind, err := Detect(dir, "git"); err != nil || kind != Git {
		t.Fatalf("prefer=git gave %q, %v", kind, err)
	}
}

func TestDetectErrorsWithoutRepo(t *testing.T) {
	if _, err := Detect(t.TempDir(), "auto"); err == nil {
		t.Fatal("expected an error when neither .git nor .jj exists")
	}
}

func TestDetectRejectsUnknownPreference(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, dir, ".git")
	if _, err := Detect(dir, "svn"); err == nil {
		t.Fatal("expected an error for an unknown preference")
	}
}

// The repository root is found by walking up — the same anchoring config.Path
// uses — so a subdirectory of a repository is never mistaken for "no repository".
func TestDetectSearchesUpward(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, ".git")
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if kind, err := Detect(sub, "auto"); err != nil || kind != Git {
		t.Fatalf("Detect(subdir) = %q, %v; want git", kind, err)
	}
	if _, err := Detect(sub, "jj"); err == nil {
		t.Fatal("prefer=jj must still fail when only .git exists up the tree")
	}
}

// Outside any repository the error says exactly that, and names where the search
// started instead of the bare "." the caller typed.
func TestDetectDoesNotReportARepositoryOutsideOne(t *testing.T) {
	dir := t.TempDir()
	_, err := Detect(dir, "auto")
	if err == nil {
		t.Fatal("expected an error outside any repository")
	}
	if !strings.Contains(err.Error(), "不在仓库里") {
		t.Fatalf("err = %v, want the no-repository wording", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(dir)) {
		t.Fatalf("err = %v, want it to name the starting point %s", err, dir)
	}
}

// AreaAt finds the area a directory sits in — main included — so a view can say
// where the caller is standing, from any depth.
func TestAreaAtFindsTheContainingArea(t *testing.T) {
	main := filepath.Join("w", "kiln")
	area := filepath.Join(main, StateDir, AreasDir, "t1")
	areas := []WorkspaceInfo{{Path: main, Main: true}, {Path: area, Issue: "t1"}}

	if got, ok := AreaAt(areas, filepath.Join(area, "internal")); !ok || got.Issue != "t1" {
		t.Fatalf("AreaAt(inside the area) = %+v, %v; want t1", got, ok)
	}
	if got, ok := AreaAt(areas, filepath.Join(main, "docs")); !ok || !got.Main {
		t.Fatalf("AreaAt(main) = %+v, %v; want the main checkout", got, ok)
	}
	if _, ok := AreaAt(areas, filepath.Join("w", "elsewhere")); ok {
		t.Fatal("a directory outside every area must not match")
	}
	if _, ok := AreaAt(nil, main); ok {
		t.Fatal("an empty area list matches nothing")
	}
}

// An area lives inside the repository's state root, named after the issue, so
// the name alone binds the two with no extra bookkeeping.
func TestAreaPathNamesAreaAfterIssue(t *testing.T) {
	root := filepath.Join("w", "kiln")
	want := filepath.Join("w", "kiln", ".kiln", "ws", "cli-necessity")
	if got := AreaPath(root, "cli-necessity"); got != want {
		t.Fatalf("AreaPath = %q, want %q", got, want)
	}
}

// The state root must exist, and hide itself, before the first area does: jj
// creates no intermediate directories, and an unignored area would show up as
// untracked content in the main checkout. Hiding the whole state root is what
// keeps it clear of every checkout.
func TestPrepareAreasCreatesStateRootAndIgnoreFile(t *testing.T) {
	root := t.TempDir()
	if err := PrepareAreas(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, AreasDir)); err != nil {
		t.Fatalf("areas directory missing: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, StateDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "/*\n" {
		t.Fatalf(".gitignore = %q, want the state root to hide itself", body)
	}
}

// An existing .gitignore is kept rather than clobbered.
func TestPrepareAreasKeepsExistingIgnoreFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignore, []byte("!keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareAreas(root); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "!keep\n" {
		t.Fatalf(".gitignore = %q, want the original untouched", body)
	}
}

// Paths that differ only in case name the same location on the platforms kiln
// runs on; genuinely different directories must not.
func TestSamePathComparesLocations(t *testing.T) {
	root := filepath.Join("w", "kiln")
	if !SamePath(root, strings.ToUpper(root)) {
		t.Fatal("case differences must not distinguish two paths")
	}
	if SamePath(root, root+"--x") {
		t.Fatal("different directories must not compare equal")
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

// writeFile writes body at root/rel, creating parent directories first.
func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The manifest drops comments and blank lines, trims each entry, and a missing
// manifest declares nothing rather than failing.
func TestReadIncludeDropsCommentsAndBlanks(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".worktreeinclude")
	writeFile(t, filepath.Dir(path), ".worktreeinclude", "# 本地文件\n\n.env\n  .env.local  \n#注释\nsecret.txt\n")
	got, err := readInclude(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".env", ".env.local", "secret.txt"}; !slices.Equal(got, want) {
		t.Fatalf("readInclude = %q, want %q", got, want)
	}
	if got, err := readInclude(filepath.Join(t.TempDir(), "missing")); err != nil || len(got) != 0 {
		t.Fatalf("a missing manifest must declare nothing: %q, %v", got, err)
	}
}

// The VCS stores and kiln's state root are never carried into an area, however
// broad a manifest pattern is.
func TestSeedableRejectsRepositoryMetadata(t *testing.T) {
	for _, rel := range []string{".git/config", ".jj/working_copy/type", ".kiln/ws/x", ".kiln"} {
		if seedable(rel) {
			t.Fatalf("seedable(%q) = true, want false", rel)
		}
	}
	for _, rel := range []string{".env", "config/secrets.json", "build/out.bin", ".kiln2/x"} {
		if !seedable(rel) {
			t.Fatalf("seedable(%q) = false, want true", rel)
		}
	}
}

// Seeding carries only the ignored files the manifest names: a tracked file is
// never duplicated, an untracked-but-unignored file is not touched, and a file
// the area already has is left alone.
func TestSeedAreaCarriesOnlyNamedIgnoredFiles(t *testing.T) {
	main := t.TempDir()
	gitRun(t, main, "init", "-q")
	writeFile(t, main, ".gitignore", "*.local\nsecret.txt\nbuild/\n")
	writeFile(t, main, "keep.txt", "tracked")
	writeFile(t, main, "a.local", "a")
	writeFile(t, main, "secret.txt", "s")
	writeFile(t, main, "loose.txt", "not ignored")
	writeFile(t, main, "build/out.bin", "bin")
	writeFile(t, main, ".worktreeinclude", "a.local\nsecret.txt\nbuild\nkeep.txt\n")
	gitRun(t, main, "add", "keep.txt", ".gitignore", ".worktreeinclude")
	gitRun(t, main, "-c", "user.name=kiln", "-c", "user.email=kiln@example.com", "commit", "-qm", "init")

	area := filepath.Join(main, StateDir, AreasDir, "t1")
	if err := os.MkdirAll(area, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, area, "secret.txt", "area's own") // must survive, never overwritten

	copied, err := SeedArea(main, area)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 2 || !slices.Contains(copied, "a.local") || !slices.Contains(copied, "build/out.bin") {
		t.Fatalf("copied = %q", copied)
	}
	if got := readFile(t, area, "a.local"); got != "a" {
		t.Fatalf("a.local = %q", got)
	}
	if got := readFile(t, area, "build/out.bin"); got != "bin" {
		t.Fatalf("build/out.bin = %q", got)
	}
	if got := readFile(t, area, "secret.txt"); got != "area's own" {
		t.Fatalf("an existing file was overwritten: %q", got)
	}
	for _, rel := range []string{"keep.txt", "loose.txt"} {
		if _, err := os.Stat(filepath.Join(area, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s must not reach the area", rel)
		}
	}
}

// A manifest listing nothing ignored copies nothing, and no manifest copies
// nothing either.
func TestSeedAreaWithoutAManifestIsANoOp(t *testing.T) {
	main := t.TempDir()
	gitRun(t, main, "init", "-q")
	writeFile(t, main, ".gitignore", "*.local\n")
	writeFile(t, main, "a.local", "a")
	area := t.TempDir()
	if copied, err := SeedArea(main, area); err != nil || len(copied) != 0 {
		t.Fatalf("no manifest: copied = %q, %v", copied, err)
	}
	writeFile(t, main, ".worktreeinclude", "# 只有注释\n")
	if copied, err := SeedArea(main, area); err != nil || len(copied) != 0 {
		t.Fatalf("comment-only manifest: copied = %q, %v", copied, err)
	}
}

// A symlink is never followed into an area, and a directory is not a regular
// file to copy.
func TestCopyEntrySkipsSymlinksAndDirectories(t *testing.T) {
	main, area := t.TempDir(), t.TempDir()
	writeFile(t, main, "real.txt", "x")
	if err := os.Mkdir(filepath.Join(main, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ok, err := copyEntry(main, area, "adir"); err != nil || ok {
		t.Fatalf("copyEntry(dir) = %v, %v; want false", ok, err)
	}
	if err := os.Symlink(filepath.Join(main, "real.txt"), filepath.Join(main, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable on this host: %v", err)
	}
	if ok, err := copyEntry(main, area, "link.txt"); err != nil || ok {
		t.Fatalf("copyEntry(symlink) = %v, %v; want false", ok, err)
	}
}
