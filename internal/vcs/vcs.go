// Package vcs defines the backend-neutral repository abstraction kiln reads
// from. A backend supplies the commit list (id, date, message), a way to record
// one action, and the rendering of a single change. The model itself lives in
// the commit messages (trailers), so it is backend-agnostic by construction.
package vcs

import (
	"fmt"
	"os"
	"path/filepath"
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
