// Package cli wires the command surface: argument parsing, git access and text
// rendering. All output goes through the writers passed to Run, so the package
// stays testable.
package cli

import (
	"io"

	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

// env carries the resolved action vocabulary, the repository backend and the
// output streams. The backend is an interface so tests can substitute a fake
// instead of touching a real repository; it is opened lazily, so commands that
// need no repo (do) still work outside one.
type env struct {
	actions      *model.ActionSet
	relations    *model.RelationSet
	dir          string
	vcsPref      string
	worktree     string
	configPath   string
	configSource string
	noCache      bool // --no-cache / KILN_NO_CACHE turns the record cache off
	repo         vcs.Repo
	kind         vcs.Kind // the backend that was opened, recorded for backend-specific wording
	out          io.Writer
	errw         io.Writer
}

// fixedFlags are the do/commit flags that are not relation names.
var fixedFlags = []string{"title", "refs", "target", "scope", "domain", "tag", "unrelate", "force"}
