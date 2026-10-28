// Package git adapts the `git` binary to the vcs.Repo interface. The engine
// never shells out directly; it consumes the plain structs produced here.
package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/vcs"
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

// streamLogFormat is the streaming form of logFormat: three NUL-separated fields,
// with -z adding one more NUL after each record. A reader that has seen three
// terminators therefore owns exactly one complete commit — no guessing where a
// record ends, and no "the last one has no terminator" special case. NUL is git's
// own machine separator; unlike the unit/record separators logFormat uses, a
// commit body can never make it ambiguous.
const streamLogFormat = "%H%x00%aI%x00%B"

// LogEach streams commits newest-first to fn, stopping as soon as fn returns
// false. See vcs.LogEach.
//
// History is never materialized: the first record is handed over as soon as git
// prints it, so a large repository pays its first-commit latency, not its
// whole-history latency. When fn stops early the child is killed — a stopped
// reader leaves git blocked on a full pipe, and Wait would then never return.
func (r ExecRunner) LogEach(fn func(vcs.Commit) bool) error {
	cmd := exec.Command("git", "log", "-z", "--author-date-order", "--format="+streamLogFormat)
	cmd.Dir = r.Dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// Git for Windows' git.exe is a shim that spawns the real git and waits, so
	// killing the direct child can leave the real one holding the inherited
	// pipe. Wait would then block on the stderr copier until that process ends;
	// the delay is the backstop that keeps a stop from becoming a hang.
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git log: %w", err)
	}
	br := bufio.NewReaderSize(stdout, 64*1024)
	stopped := false
	var readErr error
	for {
		id, err := readNULField(br)
		if err == io.EOF {
			break // clean end between records
		}
		if err != nil {
			readErr = err
			break
		}
		date, err := readNULField(br)
		if err != nil {
			readErr = truncated(err)
			break
		}
		msg, err := readNULField(br)
		if err != nil {
			readErr = truncated(err)
			break
		}
		when, _ := time.Parse(time.RFC3339, strings.TrimSpace(date))
		c := vcs.Commit{
			ID:      strings.TrimSpace(id),
			Date:    when,
			Message: strings.TrimRight(msg, "\r\n"),
		}
		if !fn(c) {
			stopped = true
			break
		}
	}
	if stopped || readErr != nil {
		// Stop the child, then close our end of the pipe: killing the shim alone
		// leaves the real git blocked on a write nobody reads, and closing the
		// pipe is what lets it notice and exit.
		_ = cmd.Process.Kill()
		_ = stdout.Close()
	}
	waitErr := cmd.Wait()
	if stopped {
		return nil
	}
	if readErr != nil {
		return readErr
	}
	if waitErr == nil {
		return nil
	}
	// An empty repository is not a failure: `git log` exits 128 with "does not
	// have any commits yet" before printing a byte. Probe HEAD and treat it as
	// an empty stream; every other failure stays a failure.
	if r.noHistory() {
		return nil
	}
	return fmt.Errorf("git log: %s", failureText(stderr.String(), waitErr))
}

// readNULField reads one NUL-terminated field. io.EOF means "the stream ended
// cleanly between records"; a record that ends mid-field is reported as
// unexpected EOF instead of being silently dropped.
func readNULField(br *bufio.Reader) (string, error) {
	b, err := br.ReadBytes(0)
	if err == io.EOF {
		if len(b) == 0 {
			return "", io.EOF
		}
		return "", io.ErrUnexpectedEOF
	}
	if err != nil {
		return "", err
	}
	return string(b[:len(b)-1]), nil
}

// truncated turns "the stream ended after one or two fields" into an unexpected
// EOF: a half record is a broken read, not the end of history.
func truncated(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// noHistory reports whether HEAD has no commit yet — the one git log failure that
// is really an empty answer. It runs only on the failure path, so the happy path
// still spawns a single process, and it reads nothing but the exit code: a
// translated git message cannot change the verdict.
func (r ExecRunner) noHistory() bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD")
	cmd.Dir = r.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return false
	}
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// failureText picks the one-line reason for a failed git call: git's stderr when
// it wrote one, else the exit status — the same rule runIn follows.
func failureText(stderr string, err error) string {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return msg
	}
	return err.Error()
}

// statLogFormat brackets each commit id with NULs so the diffstat block git
// prints after the format can be split off verbatim; NUL cannot appear in a
// commit message. git also writes two blank lines between the format line and
// the stat block, which the caller trims exactly as it trims ShowStat.
const statLogFormat = "%x00%H%x00"

// LogStats renders every commit's diffstat in one `git log` pass — the batch
// form of ShowStat, and the reason `path --all --stat` no longer spawns one
// process per action. short selects --shortstat; want keeps only the ids the
// caller asked for.
func (r ExecRunner) LogStats(short bool, want func(string) bool) (map[string]string, error) {
	flag := "--stat"
	if short {
		flag = "--shortstat"
	}
	out, err := r.run("log", flag, "--format="+statLogFormat)
	if err != nil {
		return nil, err
	}
	return parseStatLog(out, want), nil
}

// parseStatLog splits the NUL-delimited stream into (id, block) pairs. The split
// starts with an empty field (the marker before the first commit) and has no
// trailing empty field (the last block ends the stream), so pairs live at
// (1,2), (3,4), ….
func parseStatLog(out string, want func(string) bool) map[string]string {
	parts := strings.Split(out, "\x00")
	stats := make(map[string]string, len(parts)/2)
	for i := 1; i+1 < len(parts); i += 2 {
		id := parts[i]
		if id == "" || (want != nil && !want(id)) {
			continue
		}
		stats[id] = parts[i+1]
	}
	return stats
}

// Action records one action and returns the new commit's id.
//
// Empty paths means the whole area's changes: git has no snapshot of its own, so
// kiln stages everything first. Non-empty paths are staged and committed on their
// own, leaving anything else that happened to be staged for the next action.
// --allow-empty keeps an action with nothing behind it legal on both backends, so
// "empty" means the same thing whichever adapter runs.
func (r ExecRunner) Action(message string, paths []string) (string, error) {
	return r.action(message, paths, true)
}

// ActionChecked records a path-scoped action whose paths the caller already
// validated with Unchanged: the second status per path is skipped. See
// vcs.CheckedAction.
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
	if len(paths) > 0 {
		if _, err := r.run(append([]string{"add", "-A", "--"}, paths...)...); err != nil {
			return "", err
		}
		if _, err := r.run(append([]string{"commit", "--allow-empty", "-m", message, "--"}, paths...)...); err != nil {
			return "", err
		}
	} else {
		if _, err := r.run("add", "-A"); err != nil {
			return "", err
		}
		if _, err := r.run("commit", "--allow-empty", "-m", message); err != nil {
			return "", err
		}
	}
	// Reading the new id back is a nicety: the action is already recorded, so a
	// failure here must not turn a successful commit into a reported failure.
	out, err := r.run("rev-parse", "HEAD")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// Changes lists the paths the next action would record. The listing is taken at
// the caller's directory, but porcelain -z always prints root-relative paths
// (git ignores status.relativePaths under -z), so the inventory keeps the same
// shape `kiln path` and the commit confirmation print — without a rev-parse
// process to find the root first.
func (r ExecRunner) Changes() ([]string, error) {
	out, err := r.run("status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseStatusZ(out), nil
}

// MaybeHasAreas reports whether linked worktrees can exist at all, without
// spawning git. git keeps their metadata under <common-dir>/worktrees; no
// directory there means `git worktree list` would print only the main checkout,
// so status can skip that call. A checkout whose `.git` is a file is a linked
// worktree itself — different layout, unknown state — and answers true, as does
// any unreadable case: the caller then pays for the real listing.
func (r ExecRunner) MaybeHasAreas() bool {
	root, ok := findCheckoutRoot(r.Dir)
	if !ok {
		return true
	}
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil || !info.IsDir() {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(root, ".git", "worktrees"))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	return len(entries) > 0
}

// findCheckoutRoot walks up from start to the first directory holding a .git or
// .jj marker, mirroring where kiln's config lookup stops.
func findCheckoutRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		for _, marker := range []string{".git", ".jj"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Unchanged reports which of paths hold no change relative to HEAD — the check
// Action runs per path. A path that matches nothing (or is ignored) is reported
// here rather than committed: git would fail on it, but only after staging the
// others, and jj would accept it as an empty action.
func (r ExecRunner) Unchanged(paths []string) ([]string, error) {
	var bad []string
	for _, p := range paths {
		out, err := r.run("status", "--porcelain", "-z", "--", p)
		if err != nil {
			return nil, err
		}
		if strings.Trim(out, "\x00\r\n") == "" {
			bad = append(bad, p)
		}
	}
	return bad, nil
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
		return i18n.E("没有 issue 为 %q 的 worktree", issue)
	}
	branch := vcs.BranchPrefix + issue
	if !force {
		state, err := r.AreaState(vcs.WorkspaceInfo{Path: path, Issue: issue})
		if err != nil {
			return err
		}
		if state.Dirty {
			return i18n.E("worktree 还有未提交的改动: %s（要丢弃请加 --force）", issue)
		}
		if state.Unmerged {
			return i18n.E("分支 %s 还有未合并的提交（要丢弃请加 --force）", branch)
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

// parseStatusZ reads `git status --porcelain -z`: NUL-separated records of two
// status letters, one space, and the path. A rename or copy record carries the
// original path in the next field, which is skipped — the new path is the one the
// action records. The result is sorted so the same change set always prints the
// same inventory.
func parseStatusZ(out string) []string {
	fields := strings.Split(out, "\x00")
	paths := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		path := f[3:]
		if f[0] == 'R' || f[0] == 'C' {
			i++ // the next field is the original path, not a second change
		}
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
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
// git's own explanation: stderr when it wrote one, else the last line of stdout
// (some failures, "nothing staged" above all, are advice printed there).
func (r ExecRunner) runIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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
