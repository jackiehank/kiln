// Package git is the thin adapter around the `git` binary. The engine never
// shells out directly; it consumes the plain structs produced here, which keeps
// the core logic testable with hand-built commits.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Commit is one raw commit as read from `git log`.
type Commit struct {
	Hash    string
	Date    time.Time
	Message string
}

// Runner reads history, creates commits and exposes a single commit's change.
type Runner interface {
	Log() ([]Commit, error)
	Commit(message string) error
	Show(hash string) (string, error)
	ShowStat(hash string, short bool) (string, error)
}

// ExecRunner runs the real `git` binary in Dir.
type ExecRunner struct {
	Dir string
}

// logFormat uses unit (0x1f) and record (0x1e) separators so that commit
// messages containing newlines survive the round trip.
const logFormat = "%H%x1f%aI%x1f%B%x1e"

// Log returns every commit, newest first.
func (r ExecRunner) Log() ([]Commit, error) {
	cmd := exec.Command("git", "log", "--format="+logFormat)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git log: %s", msg)
	}
	return parseLog(stdout.String()), nil
}

// Commit creates a commit from message. The caller is expected to have staged
// changes; Message is passed as a single argv element so newlines are safe.
func (r ExecRunner) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.Dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(buf.String()))
	}
	return nil
}

// Show returns `git show <hash>` verbatim: the commit header (author, date,
// message and trailers) followed by the patch.
func (r ExecRunner) Show(hash string) (string, error) {
	return r.run("show", hash)
}

// ShowStat returns the diffstat git renders for hash. short selects the
// one-line `--shortstat` form, otherwise the file histogram (`--stat`). The
// commit header is suppressed with an empty --format so only the stat block is
// returned.
func (r ExecRunner) ShowStat(hash string, short bool) (string, error) {
	flag := "--stat"
	if short {
		flag = "--shortstat"
	}
	return r.run("show", flag, "--format=", hash)
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

func parseLog(out string) []Commit {
	var commits []Commit
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
		commits = append(commits, Commit{
			Hash:    strings.TrimSpace(parts[0]),
			Date:    date,
			Message: strings.TrimRight(parts[2], "\r\n"),
		})
	}
	return commits
}
