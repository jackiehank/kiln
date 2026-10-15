// Package vcs defines the backend-neutral repository abstraction kiln reads
// from. A backend supplies the commit list (id, date, message), a way to record
// one action, and the rendering of a single change. The model itself lives in
// the commit messages (trailers), so it is backend-agnostic by construction.
package vcs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	// Action records one kiln action carrying the given commit message.
	Action(message string) error
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

// AreaState is what an area still holds: the two reasons a backend refuses to
// drop it without force, and nothing else. A listing and a removal read this one
// answer, so they can never disagree about whether an area is safe to let go.
type AreaState struct {
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

// Detect picks a backend for dir. prefer is "", "auto", "git" or "jj". In auto
// mode a jj repo wins over git: a colocated jj workspace contains both .jj and
// .git, and reading through git would silently miss jj's view.
func Detect(dir, prefer string) (Kind, error) {
	hasJJ := exists(filepath.Join(dir, ".jj"))
	hasGit := exists(filepath.Join(dir, ".git"))
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
		return "", fmt.Errorf("vcs=git，但 %s 下没有 .git", dir)
	case string(JJ):
		if hasJJ {
			return JJ, nil
		}
		return "", fmt.Errorf("vcs=jj，但 %s 下没有 .jj", dir)
	default:
		return "", fmt.Errorf("未知 vcs: %q（可选 auto / git / jj）", prefer)
	}
	return "", fmt.Errorf("%s 下既没有 .git 也没有 .jj", dir)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
