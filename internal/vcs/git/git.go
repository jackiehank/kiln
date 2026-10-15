// Package git adapts the `git` binary to the vcs.Repo interface. The engine
// never shells out directly; it consumes the plain structs produced here.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kiln/internal/vcs"
)

// ExecRunner runs the real `git` binary in Dir.
type ExecRunner struct {
	Dir string
}

// logFormat uses unit (0x1f) and record (0x1e) separators so that commit
// messages containing newlines survive the round trip.
const logFormat = "%H%x1f%aI%x1f%B%x1e"

// Log returns every commit, newest first.
func (r ExecRunner) Log() ([]vcs.Commit, error) {
	out, err := r.run("log", "--format="+logFormat)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

// Action records one action. The caller is expected to have staged the changes;
// message is passed as a single argv element so newlines are safe.
func (r ExecRunner) Action(message string) error {
	_, err := r.run("commit", "-m", message)
	return err
}

// Show returns `git show <id>` verbatim: the commit header (author, date,
// message and trailers) followed by the patch.
func (r ExecRunner) Show(id string) (string, error) {
	return r.run("show", id)
}

// ShowStat returns the diffstat git renders for id. short selects the one-line
// `--shortstat` form, otherwise the file histogram (`--stat`). The commit header
// is suppressed with an empty --format so only the stat block is returned.
func (r ExecRunner) ShowStat(id string, short bool) (string, error) {
	flag := "--stat"
	if short {
		flag = "--shortstat"
	}
	return r.run("show", flag, "--format=", id)
}

// IsDedicated reports whether the current checkout is a linked worktree. The
// main checkout keeps `.git` as a directory; a linked worktree replaces it with
// a file pointing back at the main repository.
func (r ExecRunner) IsDedicated() (bool, error) {
	root, err := r.root()
	if err != nil {
		return false, err
	}
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

// NewWorkspace creates, or reuses, the linked worktree bound to issue on branch
// kiln/<issue> and returns its path. The worktree lives under the repository's
// state root, so it is anchored on the main checkout rather than on Dir.
func (r ExecRunner) NewWorkspace(issue string) (string, error) {
	areas, err := r.ListWorkspaces()
	if err != nil {
		return "", err
	}
	mainRoot := areas[0].Path
	path := vcs.AreaPath(mainRoot, issue)
	for _, a := range areas {
		if vcs.SamePath(a.Path, path) {
			return path, nil // an area for this issue already exists
		}
	}
	if err := vcs.PrepareAreas(mainRoot); err != nil {
		return "", err
	}
	if _, err := r.run("worktree", "add", path, "-b", vcs.BranchPrefix+issue); err != nil {
		return "", err
	}
	return path, nil
}

// ListWorkspaces returns the main checkout followed by every linked worktree,
// as `git worktree list` orders them. Only areas on a kiln/<issue> branch carry
// an issue.
func (r ExecRunner) ListWorkspaces() ([]vcs.WorkspaceInfo, error) {
	out, err := r.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

// Dirty reports whether the current checkout still holds uncommitted or
// untracked changes — read wherever the caller stands, since the walk-up works
// from any subdirectory of the working tree.
func (r ExecRunner) Dirty() (bool, error) {
	dirty, _, err := r.dirty(r.Dir)
	return dirty, err
}

// AreaState reports what the linked worktree still holds. Every half mirrors
// what RemoveWorkspace refuses on, so a listing and a removal can never disagree
// about whether an area is safe to let go. The main checkout carries no issue's
// work and a detached worktree names no branch, so both report nothing.
func (r ExecRunner) AreaState(area vcs.WorkspaceInfo) (vcs.AreaState, error) {
	if area.Main || area.Issue == "" {
		return vcs.AreaState{}, nil
	}
	merged, err := r.branchMerged(vcs.BranchPrefix + area.Issue)
	if err != nil {
		return vcs.AreaState{}, err
	}
	state := vcs.AreaState{Unmerged: !merged}
	dirty, gone, err := r.dirty(area.Path)
	if err != nil {
		return vcs.AreaState{}, err
	}
	state.Dirty, state.Gone = dirty, gone
	return state, nil
}

// dirty reports whether the worktree at root still holds uncommitted or
// untracked changes — the reading behind git's own refusal to drop a dirty
// worktree — and whether root is gone altogether. Gone is a state of its own,
// not a failure: git keeps a worktree registered after its directory is deleted
// by hand, and a listing has to be able to say so instead of dying on the read.
func (r ExecRunner) dirty(root string) (dirty, gone bool, err error) {
	out, err := r.runIn(root, "status", "--porcelain")
	if err != nil {
		if !dirExists(root) {
			return false, true, nil
		}
		return false, false, err
	}
	return strings.TrimSpace(out) != "", false, nil
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// RemoveWorkspace deletes the linked worktree bound to issue and the branch that
// carried it. Both halves matter: a worktree removed while its kiln/<issue>
// branch survives makes the next NewWorkspace trip over `-b`. "Still holds work"
// is read from AreaState — the same answer `worktree list` shows — and the check
// comes before anything is deleted, because dropping the worktree first would
// strand an unmerged branch. force loses both, and git's own dirty-worktree
// refusal stays on as a backstop.
func (r ExecRunner) RemoveWorkspace(issue string, force bool) error {
	areas, err := r.ListWorkspaces()
	if err != nil {
		return err
	}
	path, ok := vcs.AreaFor(areas, issue)
	if !ok {
		return fmt.Errorf("没有 issue 为 %q 的 worktree", issue)
	}
	branch := vcs.BranchPrefix + issue
	if !force {
		state, err := r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: issue})
		if err != nil {
			return err
		}
		if state.Dirty {
			return fmt.Errorf("worktree 还有未提交的改动: %s（要丢弃请加 --force）", issue)
		}
		if state.Unmerged {
			return fmt.Errorf("分支 %s 还有未合并的提交（要丢弃请加 --force）", branch)
		}
	}
	remove := []string{"worktree", "remove", path}
	drop := "-d"
	if force {
		remove = append(remove, "--force")
		drop = "-D"
	}
	if _, err := r.run(remove...); err != nil {
		return err
	}
	_, err = r.run("branch", drop, branch)
	return err
}

// branchMerged reports whether branch is already reachable from the current
// checkout's HEAD — a branch git can drop without losing commits.
func (r ExecRunner) branchMerged(branch string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", branch, "HEAD")
	cmd.Dir = r.Dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil // exit 1 is git's "not an ancestor", not a failure
	}
	return false, fmt.Errorf("git merge-base: %w", err)
}

// root returns the working directory of the current checkout.
func (r ExecRunner) root() (string, error) {
	out, err := r.run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// parseWorktrees reads `git worktree list --porcelain`: blank-line separated
// records, each led by its root path and naming its checked-out branch. The
// first record is the main checkout.
func parseWorktrees(out string) []vcs.WorkspaceInfo {
	var areas []vcs.WorkspaceInfo
	for _, block := range strings.Split(out, "\n\n") {
		var area vcs.WorkspaceInfo
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				area.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "branch "):
				ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
				if issue, ok := strings.CutPrefix(ref, "refs/heads/"+vcs.BranchPrefix); ok {
					area.Issue = issue
				}
			}
		}
		if area.Path == "" {
			continue
		}
		area.Main = len(areas) == 0
		areas = append(areas, area)
	}
	return areas
}

// run executes git in the runner's own directory.
func (r ExecRunner) run(args ...string) (string, error) { return r.runIn(r.Dir, args...) }

// runIn executes git in dir and returns stdout; on failure the error carries
// git's stderr so the caller can surface a useful message.
func (r ExecRunner) runIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return stdout.String(), nil
}

func parseLog(out string) []vcs.Commit {
	var commits []vcs.Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\r\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		var date time.Time
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[1])); err == nil {
			date = t
		}
		commits = append(commits, vcs.Commit{
			ID:      strings.TrimSpace(parts[0]),
			Date:    date,
			Message: strings.TrimRight(parts[2], "\r\n"),
		})
	}
	return commits
}
