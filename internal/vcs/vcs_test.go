package vcs

import (
	"os"
	"path/filepath"
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

// An area lives inside the repository's state root, named after the issue, so
// the name alone binds the two with no extra bookkeeping.
func TestAreaPathNamesAreaAfterIssue(t *testing.T) {
	root := filepath.Join("w", "kiln")
	want := filepath.Join("w", "kiln", ".kiln", "ws", "cli-necessity")
	if got := AreaPath(root, "cli-necessity"); got != want {
		t.Fatalf("AreaPath = %q, want %q", got, want)
	}
}

// The state root must exist, and ignore its areas, before the first area does:
// jj creates no intermediate directories, and an unignored area would show up
// as untracked content in the main checkout. Only the areas are ignored — the
// config sitting beside them is project content.
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
	if string(body) != "/ws\n" {
		t.Fatalf(".gitignore = %q, want only the areas dir ignored", body)
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
