// Package vcs defines the backend-neutral repository abstraction kiln reads
// from. A backend supplies the commit list (id, date, message), a way to record
// one action, and the rendering of a single change. The model itself lives in
// the commit messages (trailers), so it is backend-agnostic by construction.
package vcs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/i18n"
)

// Kind names a supported backend.
type Kind string

const (
	Git Kind = "git"
	JJ  Kind = "jj"
)

// Commit is one commit as read from a backend. ID is opaque to kiln: both
// backends currently use commit hashes, but nothing here depends on that.
type Commit struct {
	ID      string
	Date    time.Time
	Message string
}

// LastNonEmptyLine returns the last line of out that is not blank.
//
// It exists for one job: turning a failed command's output into a one-line error.
// A backend writes the *reason* for a failure to stderr, but several failures
// ("nothing staged" above all) are advice on stdout — and that advice is shaped
// context-first, conclusion-last, so the line worth reporting is the final one
// (`no changes added to commit (use "git add" ...)`) rather than the opening
// (`On branch master`).
func LastNonEmptyLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}

// Repo is the version-control-agnostic surface kiln needs. Implementations
// shell out to a specific tool; the engine never touches exec.
type Repo interface {
	// Log returns every commit, newest first.
	Log() ([]Commit, error)
	// Show returns one commit's change verbatim, as rendered by the backend.
	Show(id string) (string, error)
	// ShowStat returns the diffstat for one commit; short selects the one-line
	// summary form.
	ShowStat(id string, short bool) (string, error)
	// Action records one action as a commit carrying message, and returns the new
	// commit's id in the same shape Log reports — so a caller can name what it just
	// wrote instead of sending the reader off to look it up.
	//
	// paths scopes what the action records: empty means the whole area's changes —
	// the default, backend-neutral snapshot — and non-empty means only those paths.
	// Every listed path must carry a change, or the action is refused before
	// anything is staged: jj records an empty action silently, so the check is what
	// keeps "a path that changed nothing" from landing as a commit.
	Action(message string, paths []string) (string, error)
	// Changes lists the paths the next action would record, relative to the working
	// copy root and sorted. It is the inventory a caller prints so the reader can see
	// what an action contains before it lands.
	Changes() ([]string, error)
	// Unchanged returns the subset of paths that carry no change to record. It is
	// the check Action runs, exposed so a caller can name a bad path before anything
	// is staged or committed.
	Unchanged(paths []string) ([]string, error)
}

// StatLog is the batch form of Repo.ShowStat: a backend that can render every
// commit's diffstat in one pass implements it, so `path --all --stat` is one
// call instead of one call per action. short selects the one-line summary form.
// want filters which commits to keep, so a huge history does not have to be held
// in memory; a nil want means "every commit".
//
// It is optional: a caller falls back to ShowStat when a backend does not
// implement it (the jj adapter does not, yet), and a LogStats error leaves the
// caller free to fall back too.
type StatLog interface {
	LogStats(short bool, want func(id string) bool) (map[string]string, error)
}

// AreaHint is an optional cheap pre-check for ListWorkspaces: false means the
// backend provably has no linked areas, so a caller may treat the listing as
// empty without spawning anything. true means "maybe" — the caller still asks
// the backend, so the hint can never hide an area, only save a call.
type AreaHint interface {
	MaybeHasAreas() bool
}

// CheckedAction is the optional fast path for a caller that already ran
// Unchanged over paths: it records the action without reading the working tree
// again, so `commit --only` does not pay a second status per path. The caller
// carries the guarantee; Repo.Action keeps the check for every other path, so
// the safety net (jj records a no-op fileset silently) still exists.
type CheckedAction interface {
	ActionChecked(message string, paths []string) (string, error)
}

// BranchPrefix namespaces the branches of kiln's dedicated areas in git.
const BranchPrefix = "kiln/"

// StateDir is the in-repo state root: everything kiln keeps for itself — and
// nothing else — lives here. It is local state, not project content, so the
// whole directory hides itself with its own .gitignore (see PrepareAreas) — the
// same idiom jj uses for .jj. Project-shared files, the config above all, live
// at the repository root instead.
const StateDir = ".kiln"

// AreasDir is the StateDir child holding one area per issue. The name is
// deliberately short: an area nests a real working tree, and this segment is
// paid for on every path below it.
const AreasDir = "ws"

// ignoreBody is what kiln writes into <StateDir>/.gitignore: the state root is
// local state, so it hides itself and never shows up as untracked content.
const ignoreBody = "/*\n"

// WorkspaceInfo describes one working area a backend knows about.
type WorkspaceInfo struct {
	Path  string // path of the area's root directory
	Issue string // issue the area is bound to; empty for the main area
	Main  bool   // the main checkout, which kiln keeps read/merge-only
}

// AreaState is what an area still holds: the reasons a backend refuses to drop
// it without force, plus the case of an area whose directory is gone while the
// backend still registers it. A listing and a removal read this one answer, so
// they can never disagree about whether an area is safe to let go.
type AreaState struct {
	Gone     bool // the directory is missing while the backend still lists the area
	Dirty    bool // the area holds uncommitted or untracked changes
	Unmerged bool // its commits are not reachable from the main checkout
}

// Workspace is the working-area surface, implemented by backends that can
// isolate one area per issue. It is separate from Repo so a backend may serve
// reads and writes without offering isolation.
type Workspace interface {
	// IsDedicated reports whether the current area is a dedicated (non-main)
	// one. Only dedicated areas may record actions.
	IsDedicated() (bool, error)
	// Dirty reports whether the current checkout holds uncommitted or untracked
	// changes — the "净 / 脏" question the isolation hints ask about where the
	// caller is standing. Per-area state stays AreaState's job.
	Dirty() (bool, error)
	// NewWorkspace creates the dedicated area bound to issue, reusing it when it
	// already exists, and returns its path.
	NewWorkspace(issue string) (string, error)
	// ListWorkspaces lists every known area, the main checkout first.
	ListWorkspaces() ([]WorkspaceInfo, error)
	// AreaState reports what an area still holds, in exactly the terms
	// RemoveWorkspace refuses on. main is not a dedicated area and holds no
	// issue's work, so it reports the zero value.
	AreaState(area WorkspaceInfo) (AreaState, error)
	// RemoveWorkspace deletes the dedicated area bound to issue. force lets an
	// area that still holds work go — uncommitted changes, or a branch that was
	// never merged; without it a backend refuses rather than losing work, on the
	// grounds AreaState already reports.
	RemoveWorkspace(issue string, force bool) error
}

// AreaPath returns the dedicated area for issue: <mainRoot>/.kiln/ws/<issue>.
// Anchoring on the main checkout's root — never the caller's directory — keeps
// an area created from inside another area from nesting inside it.
func AreaPath(mainRoot, issue string) string {
	return filepath.Join(filepath.Clean(mainRoot), StateDir, AreasDir, issue)
}

// AreaFor returns the dedicated area bound to issue from a backend's area list.
// The binding is the name itself, so this is a search of the backend's own
// registry rather than a lookup in a stored index.
func AreaFor(areas []WorkspaceInfo, issue string) (path string, ok bool) {
	if len(areas) == 0 {
		return "", false
	}
	want := AreaPath(areas[0].Path, issue)
	for _, a := range areas {
		if SamePath(a.Path, want) {
			return a.Path, true
		}
	}
	return "", false
}

// PrepareAreas makes the state root ready for a new area: it creates
// <mainRoot>/.kiln/ws and, when absent, writes the self-ignoring .gitignore.
// Both halves are load-bearing: jj creates no intermediate directories, and the
// ignore entry has to exist before the first area does, or git reports the area
// as untracked content in the main checkout.
func PrepareAreas(mainRoot string) error {
	root := filepath.Clean(mainRoot)
	if err := os.MkdirAll(filepath.Join(root, StateDir, AreasDir), 0o755); err != nil {
		return err
	}
	ignore := filepath.Join(root, StateDir, ".gitignore")
	if _, err := os.Stat(ignore); err == nil {
		return nil // leave an existing file alone rather than clobber an edit
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(ignore, []byte(ignoreBody), 0o644)
}

// SamePath reports whether two paths name the same location, tolerating the
// separator and case differences the platforms introduce.
func SamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Detect picks a backend for dir, looking for the repository root first: the walk
// goes up through dir's ancestors and stops at the first one holding a marker,
// the same anchoring config.Path uses, so kiln behaves the same from any
// subdirectory. prefer is "", "auto", "git" or "jj". In auto mode a jj repo wins
// over git: a colocated jj workspace contains both .jj and .git, and reading
// through git would silently miss jj's view.
func Detect(dir, prefer string) (Kind, error) {
	start := absDir(dir)
	root, hasJJ, hasGit := repoRoot(start)
	switch prefer {
	case "", "auto":
		if hasJJ {
			return JJ, nil
		}
		if hasGit {
			return Git, nil
		}
	case string(Git):
		if hasGit {
			return Git, nil
		}
		if root == "" {
			return "", notARepository(start)
		}
		return "", i18n.E("vcs=git，但 %s 下没有 .git", root)
	case string(JJ):
		if hasJJ {
			return JJ, nil
		}
		if root == "" {
			return "", notARepository(start)
		}
		return "", i18n.E("vcs=jj，但 %s 下没有 .jj", root)
	default:
		return "", i18n.E("未知 vcs: %q（可选 auto / git / jj）", prefer)
	}
	return "", notARepository(start)
}

// repoRoot walks dir and its ancestors, returning the first one that holds a
// marker — .jj or .git — and which of the two it holds. root is "" once the walk
// reaches the filesystem root, i.e. dir is in no repository at all.
func repoRoot(dir string) (root string, hasJJ, hasGit bool) {
	for {
		hasJJ, hasGit = exists(filepath.Join(dir, ".jj")), exists(filepath.Join(dir, ".git"))
		if hasJJ || hasGit {
			return dir, hasJJ, hasGit
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, false
		}
		dir = parent
	}
}

// ErrNoRepository marks the "not in a repository at all" case, so a caller can
// tell it apart from a repository that merely failed to open. `kiln do` runs
// outside a repository by design, and it wants to say exactly this and nothing
// more.
var ErrNoRepository = errors.New("不在仓库里")

// notARepository is the one wording kiln uses for "no repository here", naming
// where the search started so the message is never the bare "." the caller typed.
// It unwraps to ErrNoRepository so errors.Is keeps working while the text can
// follow the active language.
func notARepository(start string) error {
	return &noRepoError{start: start}
}

// noRepoError carries the start directory; its text is resolved when printed so
// a language switch after detection still shows the right wording.
type noRepoError struct{ start string }

func (e *noRepoError) Error() string {
	return i18n.T("%s（从 %s 向上也没有 .git / .jj）", i18n.T(ErrNoRepository.Error()), e.start)
}

func (e *noRepoError) Unwrap() error { return ErrNoRepository }

// absDir makes dir absolute, falling back to a cleaned relative path.
func absDir(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

// AreaAt returns the known area that contains dir — the one the caller is
// standing in, main included. Areas nest inside the main checkout, so the
// *deepest* match wins; ok is false when dir is outside every area.
func AreaAt(areas []WorkspaceInfo, dir string) (WorkspaceInfo, bool) {
	abs := absDir(dir)
	var best WorkspaceInfo
	found := false
	for _, a := range areas {
		if !within(abs, a.Path) {
			continue
		}
		if !found || len(filepath.Clean(a.Path)) > len(filepath.Clean(best.Path)) {
			best, found = a, true
		}
	}
	return best, found
}

// within reports whether dir sits at or under root, tolerating the separator and
// case differences the platforms introduce.
func within(dir, root string) bool {
	dir, root = filepath.Clean(dir), filepath.Clean(absDir(root))
	if strings.EqualFold(dir, root) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(dir), strings.ToLower(root)+string(filepath.Separator))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
