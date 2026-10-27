// Package jj adapts the `jj` (Jujutsu) binary to the vcs.Repo interface.
//
// kiln's model lives in commit messages, and jj preserves messages verbatim, so
// a jj backend needs no model changes. The one semantic difference is the
// absence of a staging area: "recording an action" finalizes the whole
// working-copy change, whereas git records only what the caller staged.
package jj

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/vcs"
)

// ExecRunner runs the real `jj` binary in Dir.
type ExecRunner struct{ Dir string }

// revset and template define the log output. The revset drops the virtual root
// and the working-copy commit (@) so the result matches git's "committed actions
// only, newest first" view: in jj the working copy is itself a commit, so
// without ~@ an uncommitted action would be counted.
const (
	revset    = "all() ~ root() ~ @"
	template  = `commit_id ++ "\n" ++ author.timestamp() ++ "\n" ++ description ++ "\n@@KILN@@\n"`
	recordSep = "@@KILN@@"
)

// Log returns every committed record, newest first.
func (r ExecRunner) Log() ([]vcs.Commit, error) {
	out, err := r.run("log", "--no-graph", "-r", revset, "-T", template)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

// Action records one kiln action and returns the new commit's id. Unlike git there
// is no staging area: the working-copy change is finalized as a commit carrying
// the message. `jj commit` also starts a fresh working-copy commit, so the action
// just recorded is `@-` — the commit Log's revset leaves out for that same reason.
//
// Empty paths records the whole working-copy change (the default snapshot);
// non-empty paths records only their changes, via jj's native FILESETS. Each path
// is checked first: jj would otherwise accept a fileset that matches nothing and
// record an empty action in silence.
func (r ExecRunner) Action(message string, paths []string) (string, error) {
	return r.action(message, paths, true)
}

// ActionChecked records a path-scoped action whose paths the caller already
// validated with Unchanged: the second check is skipped. See vcs.CheckedAction.
func (r ExecRunner) ActionChecked(message string, paths []string) (string, error) {
	return r.action(message, paths, false)
}

func (r ExecRunner) action(message string, paths []string, check bool) (string, error) {
	if len(paths) > 0 && check {
		if bad, err := r.Unchanged(paths); err != nil {
			return "", err
		} else if len(bad) > 0 {
			return "", i18n.E("没有改动可记录: %s", strings.Join(bad, ", "))
		}
	}
	args := []string{"commit", "--message", message}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	if _, err := r.run(args...); err != nil {
		return "", err
	}
	// Reading the new id back is a nicety: the action is already recorded, so a
	// failure here must not turn a successful commit into a reported failure.
	out, err := r.run("log", "--no-graph", "-r", "@-", "-T", "commit_id")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// Changes lists the paths the next action would record, from `jj diff --summary`.
// jj renders OS-native separators, so the listing is normalized to slashes: the
// commit confirmation and `--only` paths are the same surface, and it should not
// change shape with the platform.
func (r ExecRunner) Changes() ([]string, error) {
	out, err := r.run("diff", "--summary")
	if err != nil {
		return nil, err
	}
	return parseSummary(out), nil
}

// Unchanged reports which of paths hold no change in the working copy. On this
// backend the check is load-bearing, not defensive: `jj commit <fileset>` matches
// nothing without complaint and still records the action.
func (r ExecRunner) Unchanged(paths []string) ([]string, error) {
	var bad []string
	for _, p := range paths {
		out, err := r.run("diff", "--summary", "--", p)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(out) == "" {
			bad = append(bad, p)
		}
	}
	return bad, nil
}

// Show returns `jj show <id>` verbatim: the change header, message and diff, as
// rendered by jj.
func (r ExecRunner) Show(id string) (string, error) {
	return r.run("show", "-r", id)
}

// ShowStat returns the diffstat jj renders for id. jj has no --shortstat, so the
// one-line form is recovered from the summary line at the end of `jj diff --stat`.
func (r ExecRunner) ShowStat(id string, short bool) (string, error) {
	out, err := r.run("diff", "-r", id, "--stat")
	if err != nil {
		return "", err
	}
	if short {
		return vcs.LastNonEmptyLine(out), nil
	}
	return out, nil
}

// workspaceFormat renders one workspace per record: name, root, record
// separator. jj's template language emits the two control bytes verbatim.
const workspaceFormat = `name ++ "\x1f" ++ root ++ "\x1e"`

// mainWorkspace is jj's name for the workspace created at init; kiln treats it
// as the main checkout.
const mainWorkspace = "default"

// IsDedicated reports whether the current workspace is a dedicated one: any
// workspace other than the main "default" one.
func (r ExecRunner) IsDedicated() (bool, error) {
	current, err := r.workspaceRoot()
	if err != nil {
		return false, err
	}
	areas, err := r.ListWorkspaces()
	if err != nil {
		return false, err
	}
	for _, a := range areas {
		if vcs.SamePath(a.Path, current) {
			return !a.Main, nil
		}
	}
	return false, i18n.E("jj workspace list 未列出当前 worktree: %s", current)
}

// NewWorkspace creates, or reuses, the jj workspace bound to issue and returns
// its path. The workspace name is the issue, so naming alone binds it; it lives
// under the repository's state root, anchored on the main workspace rather than
// on Dir.
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
	if err := vcs.PrepareAreas(mainRoot); err != nil { // jj builds no parent dirs itself
		return "", err
	}
	if _, err := r.run("workspace", "add", "--name", issue, path); err != nil {
		return "", err
	}
	return path, nil
}

// ListWorkspaces returns the main workspace followed by the dedicated ones.
func (r ExecRunner) ListWorkspaces() ([]vcs.WorkspaceInfo, error) {
	out, err := r.run("workspace", "list", "-T", workspaceFormat)
	if err != nil {
		return nil, err
	}
	areas := parseWorkspaces(out)
	sort.SliceStable(areas, func(i, j int) bool {
		if areas[i].Main != areas[j].Main {
			return areas[i].Main
		}
		return areas[i].Path < areas[j].Path
	})
	return areas, nil
}

// Dirty reports whether the current workspace holds working-copy changes — the
// same reading `workspace forget` is refused on, asked of where the caller stands.
func (r ExecRunner) Dirty() (bool, error) { return r.hasChanges(r.Dir) }

// AreaState reports what the workspace still holds. Only the uncommitted half
// exists on this backend: `workspace forget` asks no questions and loses no
// commits — the workspace's commit stays in the repository — so there is no
// unmerged-branch ground to refuse on. The main workspace reports nothing.
func (r ExecRunner) AreaState(area vcs.WorkspaceInfo) (vcs.AreaState, error) {
	if area.Main {
		return vcs.AreaState{}, nil
	}
	dirty, err := r.hasChanges(area.Path)
	if err != nil {
		return vcs.AreaState{}, err
	}
	return vcs.AreaState{Dirty: dirty}, nil
}

// RemoveWorkspace forgets the workspace bound to issue and deletes its
// directory. jj's `workspace forget` only unregisters the workspace — the files
// stay on disk — and it asks no questions about them, so an area that still
// holds changes is refused here, on the ground AreaState already reports, and
// the directory is removed explicitly.
func (r ExecRunner) RemoveWorkspace(issue string, force bool) error {
	areas, err := r.ListWorkspaces()
	if err != nil {
		return err
	}
	path, ok := vcs.AreaFor(areas, issue)
	if !ok {
		return i18n.E("没有 issue 为 %q 的 worktree", issue)
	}
	if !force {
		state, err := r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: issue})
		if err != nil {
			return err
		}
		if state.Dirty {
			return i18n.E("worktree 还有未提交的改动: %s（要丢弃请加 --force）", issue)
		}
	}
	if _, err := r.run("workspace", "forget", issue); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// hasChanges reports whether the workspace rooted at root holds working-copy
// changes — the jj-side answer to "is this area still holding work".
func (r ExecRunner) hasChanges(root string) (bool, error) {
	out, err := r.run("-R", root, "diff", "--summary")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// workspaceRoot returns the root of the workspace the runner operates in.
func (r ExecRunner) workspaceRoot() (string, error) {
	out, err := r.run("workspace", "root")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// parseSummary reads `jj diff --summary`: one line per changed path, led by a
// status letter and a space. A rename or copy line carries the pair in braces,
// which is kept whole — it is a display inventory, and the new name is inside it.
// Paths are normalized to slashes and sorted so the inventory is deterministic.
func parseSummary(out string) []string {
	paths := make([]string, 0, 8)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, path, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if path = strings.TrimSpace(path); path != "" {
			// jj prints Windows paths with backslashes; the summary is a
			// portable view, so normalize both separators regardless of
			// the host we happen to be running on.
			paths = append(paths, strings.ReplaceAll(path, `\`, "/"))
		}
	}
	sort.Strings(paths)
	return paths
}

// parseWorkspaces splits the workspace-list template output into areas. Each
// record is a name and a root separated by a unit separator.
func parseWorkspaces(out string) []vcs.WorkspaceInfo {
	var areas []vcs.WorkspaceInfo
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.Trim(rec, "\r\n")
		if rec == "" {
			continue
		}
		name, root, ok := strings.Cut(rec, "\x1f")
		if !ok {
			continue
		}
		name, root = strings.TrimSpace(name), strings.TrimSpace(root)
		if root == "" {
			continue
		}
		area := vcs.WorkspaceInfo{Path: root, Main: name == mainWorkspace}
		if !area.Main {
			area.Issue = name
		}
		areas = append(areas, area)
	}
	return areas
}

// run executes jj with args and returns stdout; on failure the error carries
// jj's stderr so the caller can surface a useful message.
// run executes jj in Dir; on failure the error carries jj's own explanation:
// stderr when it wrote one, else the last line of stdout.
func (r ExecRunner) run(args ...string) (string, error) {
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = vcs.LastNonEmptyLine(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("jj %s: %s", args[0], msg)
	}
	return stdout.String(), nil
}

// parseLog splits the template output into commits. Each record is three
// sections — id, author timestamp, description — terminated by recordSep.
func parseLog(out string) []vcs.Commit {
	var commits []vcs.Commit
	for _, rec := range strings.Split(out, recordSep) {
		lines := strings.Split(strings.Trim(rec, "\r\n"), "\n")
		if len(lines) < 2 {
			continue
		}
		id := strings.TrimSpace(lines[0])
		if id == "" {
			continue
		}
		msg := ""
		if len(lines) > 2 {
			msg = strings.TrimRight(strings.Join(lines[2:], "\n"), "\n")
		}
		commits = append(commits, vcs.Commit{
			ID:      id,
			Date:    parseTime(strings.TrimSpace(lines[1])),
			Message: msg,
		})
	}
	return commits
}

// parseTime parses jj's `author.timestamp()` rendering, e.g.
// "2026-10-14 21:30:47.000 +08:00".
func parseTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05.000 -07:00",
		"2006-01-02 15:04:05 -07:00",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
