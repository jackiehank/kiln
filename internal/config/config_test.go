package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFallsBackToDefaults(t *testing.T) {
	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if set.Start() != "ask" || !set.IsValid("hypothesize") || !set.IsTerminal("apply") {
		t.Fatalf("unexpected defaults: %v", set.Names())
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"actions":["ask","probe","done"],"start":"ask","terminal":["done"]}`)
	set, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !set.IsValid("probe") || set.IsValid("hypothesize") {
		t.Fatalf("vocabulary not replaced: %v", set.Names())
	}
	if set.IsTerminal("probe") || !set.IsTerminal("done") {
		t.Fatalf("terminal not replaced: %v", set.Names())
	}
}

func TestLoadDerivesStartAndTerminal(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"actions":["ask","plan","do","done"]}`)
	set, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if set.Start() != "ask" {
		t.Fatalf("start = %q", set.Start())
	}
	if !set.IsTerminal("done") || set.IsTerminal("plan") {
		t.Fatalf("terminal not derived from the last entry: %v", set.Names())
	}
}

func TestLoadRejectsInconsistentConfig(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"actions":["a","b"],"start":"zzz","terminal":["b"]}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected an error for a start outside the vocabulary")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"actions":`)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}
