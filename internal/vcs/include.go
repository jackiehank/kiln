package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IncludeFile is the manifest naming the local, gitignored files a fresh area
// needs. The name is deliberately the industry standard: Claude Code and Codex
// both read a .worktreeinclude at the repository root, so one file serves every
// tool that creates worktrees — kiln included.
const IncludeFile = ".worktreeinclude"

// offLimits are the root entries never carried into an area: the VCS stores and
// kiln's own state root. Git reports .jj and .kiln as ignored content, so
// without this guard a broad manifest pattern could copy repository metadata
// into an area.
var offLimits = map[string]bool{".git": true, ".jj": true, StateDir: true}

// SeedArea carries the repository's declared local files into a freshly created
// area and returns the relative paths it copied.
//
// The manifest is <mainRoot>/.worktreeinclude: one .gitignore-style pattern per
// line, with '#' comments and blank lines dropped. Copying follows the same
// rules Claude Code and Codex apply, so a manifest written for them behaves
// identically here:
//
//   - only git-ignored files travel — a tracked file is never duplicated;
//   - a destination that already exists is left alone, never overwritten;
//   - symlinks are skipped.
//
// git resolves both halves of "ignored and matching" in one call, and it is the
// ignore engine for either backend: gitignore semantics are git's own, and jj
// reads the very same .gitignore files. jj repositories are colocated by
// default, so .git is present and the call works unchanged.
func SeedArea(mainRoot, areaRoot string) ([]string, error) {
	patterns, err := readInclude(filepath.Join(mainRoot, IncludeFile))
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return nil, nil
	}
	entries, err := ignoredMatching(mainRoot, patterns)
	if err != nil {
		return nil, err
	}
	var copied []string
	for _, rel := range entries {
		if !seedable(rel) {
			continue
		}
		ok, err := copyEntry(mainRoot, areaRoot, rel)
		if err != nil {
			return copied, err
		}
		if ok {
			copied = append(copied, rel)
		}
	}
	return copied, nil
}

// readInclude reads the manifest, dropping comments and blank lines. A missing
// manifest is not an error: it simply declares nothing to carry.
func readInclude(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, nil
}

// seedable rejects entries under the VCS stores and kiln's state root.
func seedable(rel string) bool {
	root, _, _ := strings.Cut(rel, "/")
	return !offLimits[root]
}

// ignoredMatching asks git which ignored, untracked entries match the manifest
// patterns. Git answers both halves in one call: --others with --ignored and
// --exclude-standard selects gitignored content, and the patterns ride along as
// pathspecs, so a plain path or a glob resolves to what it names.
func ignoredMatching(mainRoot string, patterns []string) ([]string, error) {
	args := append([]string{
		"-C", mainRoot, "ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--",
	}, patterns...)
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git ls-files: %s", msg)
	}
	var entries []string
	for _, e := range strings.Split(stdout.String(), "\x00") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// copyEntry copies one entry from the main checkout into the area, returning
// false when it deliberately does nothing: the source is gone or is not a
// regular file (a symlink or a directory), or the destination already exists.
func copyEntry(mainRoot, areaRoot, rel string) (bool, error) {
	src := filepath.Join(mainRoot, filepath.FromSlash(rel))
	info, err := os.Lstat(src)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	dst := filepath.Join(areaRoot, filepath.FromSlash(rel))
	if _, err := os.Lstat(dst); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	return true, copyFile(src, dst, info.Mode().Perm())
}

// copyFile writes src to dst with perm, creating dst exclusively so a concurrent
// writer cannot be clobbered.
func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
