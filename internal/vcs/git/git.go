// Package git adapts the `git` binary to the vcs.Repo interface. The engine
// never shells out directly; it consumes the plain structs produced here.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
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

// run executes git with args and returns stdout; on failure the error carries
// git's stderr so the caller can surface a useful message.
func (r ExecRunner) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
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
