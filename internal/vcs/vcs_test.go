package vcs

import (
	"os"
	"path/filepath"
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
