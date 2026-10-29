package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackiehank/kiln/internal/i18n"
)

// writeConfig puts body at <root>/.kilnconfig.jsonc.
func writeConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkRepo returns a directory that looks like a repository root.
func mkRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Comments are blanked out, but a `//` inside a string literal is data, not a
// comment — and the bytes that remain must still parse as strict JSON.
func TestStripCommentsLeavesStringsAlone(t *testing.T) {
	src := []byte("{\n  // a line comment\n  \"vcs\": \"auto // not a comment\",\n" +
		"  /* block\n     comment */\n  \"note\": \"/* also data */\"\n}\n")
	got, err := stripComments(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(src) {
		t.Fatalf("length changed: %d -> %d (offsets must stay aligned)", len(src), len(got))
	}
	if !strings.Contains(string(got), `"vcs": "auto // not a comment"`) {
		t.Fatalf("string literal was damaged:\n%s", got)
	}
	if !strings.Contains(string(got), `"/* also data */"`) {
		t.Fatalf("comment-looking string was damaged:\n%s", got)
	}
	// Newlines survive so error positions keep their line numbers.
	if n := strings.Count(string(got), "\n"); n != strings.Count(string(src), "\n") {
		t.Fatalf("newlines = %d, want %d", n, strings.Count(string(src), "\n"))
	}
}

func TestStripCommentsRejectsUnterminatedBlock(t *testing.T) {
	_, err := stripComments([]byte("{\n  /* never closed\n}\n"))
	if err == nil {
		t.Fatal("expected an error for an unterminated block comment")
	}
	if !strings.Contains(err.Error(), "第 2 行") {
		t.Fatalf("error should name the line: %v", err)
	}
}

// Outside a repository there is no config to read, and kiln falls back to the
// built-in vocabulary so `do` keeps working.
func TestReadFallsBackToDefaultsOutsideRepo(t *testing.T) {
	cfg, err := Read(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != "" || cfg.VCS != "auto" || cfg.Worktree != WorktreeOff {
		t.Fatalf("cfg = %+v, want no path, auto, and isolation off", cfg)
	}
	if cfg.Source != SourceDefault {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceDefault)
	}
	if cfg.Actions.Start() != "ask" || !cfg.Actions.IsTerminal("apply") {
		t.Fatalf("unexpected defaults: %v", cfg.Actions.Names())
	}
}

// A repository without a config file still gets the defaults, but it must say
// so: the candidate path exists for `config path` to seed later, yet no file
// was read.
func TestReadReportsDefaultSourceWhenFileIsMissing(t *testing.T) {
	root := mkRepo(t)
	cfg, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != SourceDefault {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceDefault)
	}
	if want := filepath.Join(root, FileName); cfg.Path != want {
		t.Fatalf("Path = %q, want %q", cfg.Path, want)
	}
	if cfg.Actions.Start() != "ask" {
		t.Fatalf("defaults not applied: %v", cfg.Actions.Names())
	}
}

// A file with no actions block is still the source of the defaults: Source says
// which file was read, not whether it changed the vocabulary.
func TestReadKeepsFileSourceWithoutActionsBlock(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"vcs": "git"}`)
	cfg, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != SourceFile {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceFile)
	}
	if cfg.Actions.Start() != "ask" || cfg.Actions.IsValid("probe") {
		t.Fatalf("actions should stay default: %v", cfg.Actions.Names())
	}
}

func TestReadParsesGroupedJSONC(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{
  // 词表
  "actions": { "names": ["ask", "probe", "done"], "start": "ask", "terminal": ["done"] },
  "vcs": "jj" // 后端
}`)
	cfg, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Actions.IsValid("probe") || cfg.Actions.IsValid("hypothesize") {
		t.Fatalf("vocabulary not replaced: %v", cfg.Actions.Names())
	}
	if cfg.Actions.IsTerminal("probe") || !cfg.Actions.IsTerminal("done") {
		t.Fatalf("terminal not replaced: %v", cfg.Actions.Names())
	}
	if cfg.VCS != "jj" {
		t.Fatalf("VCS = %q, want jj", cfg.VCS)
	}
	if cfg.Source != SourceFile {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceFile)
	}
	if want := filepath.Join(root, FileName); cfg.Path != want {
		t.Fatalf("Path = %q, want %q", cfg.Path, want)
	}
}

func TestReadDerivesStartAndTerminal(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"actions": {"names": ["ask", "plan", "do", "done"]}}`)
	cfg, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Actions.Start() != "ask" {
		t.Fatalf("start = %q", cfg.Actions.Start())
	}
	if !cfg.Actions.IsTerminal("done") || cfg.Actions.IsTerminal("plan") {
		t.Fatalf("terminal not derived from the last entry: %v", cfg.Actions.Names())
	}
	if cfg.VCS != "auto" {
		t.Fatalf("VCS = %q, want auto", cfg.VCS)
	}
}

// A misspelled key is an error rather than a silently ignored setting.
func TestReadRejectsUnknownKey(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"actions": {"nams": ["ask", "done"]}}`)
	if _, err := Read(root); err == nil {
		t.Fatal("expected an error for an unknown field")
	}
}

// The relation vocabulary is fixed (design-philosophy's conclusion): a
// `relations` section is an unknown key, not a supported customisation point.
func TestReadRejectsRelationsSection(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"relations": {"aliases": {"foo": "relates-to"}}}`)
	_, err := Read(root)
	if err == nil {
		t.Fatal("expected an error for a relations section")
	}
	if !strings.Contains(err.Error(), "relations") {
		t.Fatalf("the error should name the unknown field: %v", err)
	}
}

func TestReadRejectsInconsistentConfig(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"actions": {"names": ["a", "b"], "start": "zzz", "terminal": ["b"]}}`)
	if _, err := Read(root); err == nil {
		t.Fatal("expected an error for a start outside the vocabulary")
	}
}

func TestReadRejectsMalformedJSON(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"actions":`)
	if _, err := Read(root); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

// The config is anchored on the repository root: running from a subdirectory
// must find the same file rather than silently falling back to defaults.
func TestReadFindsConfigFromSubdirectory(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"vcs": "jj"}`)
	sub := filepath.Join(root, "internal", "cli")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := Read(sub)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VCS != "jj" || cfg.Path != filepath.Join(root, FileName) {
		t.Fatalf("cfg = %+v, want the repo-root file", cfg)
	}
}

func TestEnsureWritesCommentedDefaults(t *testing.T) {
	root := mkRepo(t)
	path, created, err := Ensure(root)
	if err != nil || !created {
		t.Fatalf("Ensure = %q, %v, %v; want a fresh file", path, created, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "//") {
		t.Fatalf("seeded config carries no comments:\n%s", body)
	}
	// The seed must be a valid config that reproduces the built-in defaults —
	// otherwise editing it would change behaviour before the user touches it.
	cfg, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Actions.Start() != "ask" || !cfg.Actions.IsTerminal("conclude") || !cfg.Actions.IsTerminal("apply") {
		t.Fatalf("seeded vocabulary is not the default: %v", cfg.Actions.Names())
	}
	if cfg.VCS != "auto" {
		t.Fatalf("seeded VCS = %q, want auto", cfg.VCS)
	}
	// Every configurable setting is spelled out, so the seeded file is a complete
	// declaration of the repository's configuration rather than a partial one.
	if !strings.Contains(string(body), `"worktree"`) || cfg.Worktree != WorktreeOff || cfg.Isolated() {
		t.Fatalf("seeded config must declare worktree=off :\n%s", body)
	}
}

func TestEnsureKeepsAnExistingFile(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"vcs": "jj"}`)
	path, created, err := Ensure(root)
	if err != nil || created {
		t.Fatalf("Ensure = %q, %v, %v; want it left alone", path, created, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"vcs": "jj"}` {
		t.Fatalf("file was clobbered: %q", body)
	}
}

func TestEnsureOutsideRepoFails(t *testing.T) {
	if _, _, err := Ensure(t.TempDir()); err == nil {
		t.Fatal("expected an error when there is no repository root to write to")
	}
}

// The isolation mode is a two-state knob: off when unset, and a repository can
// turn it on by name; case and spacing are tolerated.
func TestReadParsesWorktreeMode(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"worktree": "off"}`, WorktreeOff},
		{`{"worktree": "on"}`, WorktreeOn},
		{`{"worktree": " OFF "}`, WorktreeOff},
		{`{}`, WorktreeOff},
	}
	for _, tc := range cases {
		root := mkRepo(t)
		writeConfig(t, root, tc.body)
		cfg, err := Read(root)
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		if cfg.Worktree != tc.want {
			t.Fatalf("%s: Worktree = %q, want %q", tc.body, cfg.Worktree, tc.want)
		}
		if got := cfg.Isolated(); got != (tc.want == WorktreeOn) {
			t.Fatalf("%s: Isolated = %v", tc.body, got)
		}
	}
}

// An unknown mode is an error rather than a silent fallback to the default.
func TestReadRejectsUnknownWorktreeMode(t *testing.T) {
	root := mkRepo(t)
	writeConfig(t, root, `{"worktree": "sometimes"}`)
	if _, err := Read(root); err == nil {
		t.Fatal("expected an error for an unknown worktree mode")
	}
}

// TestDefaultFileIsTranslated pins the seed to the catalog. A stale key means an
// English checkout gets a Chinese config, which is exactly how this drifted when
// the cache comment was added without touching the catalog.
func TestDefaultFileIsTranslated(t *testing.T) {
	i18n.Set(i18n.En)
	defer i18n.Set(i18n.Zh)
	en := i18n.T(defaultFile)
	if en == defaultFile {
		t.Fatal("the seed has no English entry: the catalog key drifted from defaultFile")
	}
	if regexp.MustCompile(`[\x{4e00}-\x{9fff}]`).MatchString(en) {
		t.Fatalf("the English seed still has CJK:\n%s", en)
	}
}
