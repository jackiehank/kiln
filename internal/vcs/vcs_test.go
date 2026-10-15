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

// A dedicated area lives beside the main checkout, named after the issue, so
// the name alone binds the two with no extra bookkeeping.
func TestSiblingAreaNamesAreaAfterIssue(t *testing.T) {
	root := filepath.Join("w", "kiln")
	want := filepath.Join("w", "kiln--cli-necessity")
	if got := SiblingArea(root, "cli-necessity"); got != want {
		t.Fatalf("SiblingArea = %q, want %q", got, want)
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
