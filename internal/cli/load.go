package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
	"github.com/jackiehank/kiln/internal/vcs/git"
	"github.com/jackiehank/kiln/internal/vcs/jj"
)

// rel returns the relation vocabulary, falling back to the built-in default
// when the env was built without one (e.g. tests).
func (e *env) rel() *model.RelationSet {
	if e.relations == nil {
		return model.DefaultRelationSet()
	}
	return e.relations
}

// relationsFromFlags reads relation declarations from the parsed flags, accepting
// canonical names and aliases (resolved to canonical).
func (e *env) relationsFromFlags(f flags) []model.Rel {
	rel := e.rel()
	var out []model.Rel
	add := func(key string) {
		n, ok := rel.Resolve(key)
		if !ok {
			return
		}
		for _, t := range f.list(key) {
			out = append(out, model.Rel{Name: n.Name, Target: t})
		}
	}
	for _, name := range rel.Names() {
		add(name)
	}
	for _, alias := range rel.Aliases() {
		add(alias)
	}
	return out
}

// doCommitFlags is the do/commit flag allowlist: fixed flags plus the relation
// vocabulary's names and aliases.
func (e *env) doCommitFlags() map[string]bool {
	allowed := allowedFlags(fixedFlags...)
	for _, name := range e.rel().Names() {
		allowed[name] = true
	}
	for _, alias := range e.rel().Aliases() {
		allowed[alias] = true
	}
	return allowed
}

// backend returns the repository backend, opening it from dir/config on first
// use and caching it. Tests inject one via repo.
func (e *env) backend() (vcs.Repo, error) {
	if e.repo != nil {
		return e.repo, nil
	}
	r, kind, err := openRepo(e.dir, e.vcsPref)
	if err != nil {
		return nil, err
	}
	e.repo, e.kind = r, kind
	return r, nil
}

// openRepo picks a backend for dir. In auto mode jj wins over git: a colocated
// workspace contains both .jj and .git, so reading through git would silently
// miss jj's view.
func openRepo(dir, prefer string) (vcs.Repo, vcs.Kind, error) {
	kind, err := vcs.Detect(dir, prefer)
	if err != nil {
		return nil, "", err
	}
	switch kind {
	case vcs.Git:
		return git.ExecRunner{Dir: dir}, kind, nil
	case vcs.JJ:
		return jj.ExecRunner{Dir: dir}, kind, nil
	}
	return nil, "", i18n.E("未知后端: %s", kind)
}

// checkTargets validates --target for `do`: every declared path must be among the
// working changes the next commit would record. Outside a repository there is
// nothing to compare against (the backend fails), so the preview still prints.
func (e *env) checkTargets(targets []string) int {
	backend, err := e.backend()
	if err != nil {
		return 0
	}
	files, cerr := backend.Changes()
	if cerr != nil {
		return 0
	}
	return reportMissingTargets(e.errw, targets, files)
}

// reportMissingTargets refuses a target the file set does not cover; it names the
// missing paths and returns 1, mirroring the --only path refusal.
func reportMissingTargets(errw io.Writer, targets, files []string) int {
	missing := targetsNotIn(targets, files)
	if len(missing) == 0 {
		return 0
	}
	fmt.Fprint(errw, i18n.T("以下 target 没有改动可记录: %s\n", strings.Join(missing, ", ")))
	return 1
}

// targetsNotIn reports the declared targets the file set does not cover. Paths are
// slash-normalised and compared exactly; a target covers a directory when it is a
// prefix of a changed path, and a changed "sub/" (an untracked directory git
// collapsed) covers the files under it.
func targetsNotIn(targets, files []string) []string {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "/")
		return strings.TrimPrefix(s, "./")
	}
	set := make([]string, 0, len(files))
	for _, f := range files {
		if f = norm(f); f != "" {
			set = append(set, f)
		}
	}
	var missing []string
	for _, t := range targets {
		t = norm(t)
		if t == "" {
			continue
		}
		dir := strings.TrimSuffix(t, "/") + "/"
		covered := false
		for _, f := range set {
			if f == t || strings.HasPrefix(f, dir) || (strings.HasSuffix(f, "/") && strings.HasPrefix(t, f)) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, t)
		}
	}
	return missing
}

// loadRepo reads history and aggregates it; returns (nil, exitCode) on failure.
func (e *env) loadRepo() (*model.Repo, int) {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return nil, 1
	}
	if repo, ok := e.loadCached(backend); ok {
		return repo, 0
	}
	repo, err := e.loadUncached(backend)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取历史失败: %v\n", err))
		return nil, 1
	}
	return repo, 0
}

// loadUncached is the cold path every caller shares: read the whole history,
// build the model, and seed the cache with what was read. It returns the error
// instead of printing it, so a caller whose failure is not an error (the
// duplicate-start check) can stay silent.
func (e *env) loadUncached(backend vcs.Repo) (*model.Repo, error) {
	commits, err := backend.Log()
	if err != nil {
		return nil, err
	}
	actions, records := engine.ActionsAndRecordsFromCommits(commits, e.rel())
	if len(commits) > 0 {
		tip, _ := decodeOID(commits[0].ID)
		e.persistRecords(backend, records, tip)
		// The scan already holds every commit in git's order: seed the enumeration
		// snapshot so the next read does not have to ask git for the same list.
		e.storeEnumFromCommits(commits)
	}
	return engine.Build(actions, e.rel()), nil
}
