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
	"os/exec"
	"strings"
	"time"

	"kiln/internal/vcs"
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

// Action records one kiln action. Unlike git there is no staging area: the
// working-copy change is finalized as a commit carrying the message.
func (r ExecRunner) Action(message string) error {
	_, err := r.run("commit", "--message", message)
	return err
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
		return lastLine(out), nil
	}
	return out, nil
}

// run executes jj with args and returns stdout; on failure the error carries
// jj's stderr so the caller can surface a useful message.
func (r ExecRunner) run(args ...string) (string, error) {
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
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

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
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
