// Package config resolves kiln's per-repository configuration. The file lives at
// the repository root — <repo>/.kilnconfig.jsonc — beside the other project-shared
// files, because kiln's config carries the project's own vocabulary, which every
// collaborator should read the same way. It is committed project content, not
// local state: kiln's local state (the .kiln/ state root) stays ignored.
//
// The file is JSONC: JSON plus // and /* */ comments, because a config that can
// say why each entry is set is worth more than one that cannot. Only comments
// are added on top of JSON — no trailing commas — so the payload stays strict.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

// FileName is the config file, at the repository root.
const FileName = ".kilnconfig.jsonc"

// file mirrors the on-disk schema. The vocabulary is grouped so later settings
// can sit beside it instead of competing for the top-level keys.
type file struct {
	Actions struct {
		Names    []string `json:"names"`
		Start    string   `json:"start"`
		Terminal []string `json:"terminal"`
	} `json:"actions"`
	VCS      string `json:"vcs"`
	Worktree string `json:"worktree"`
}

// Config sources: where the effective vocabulary came from.
const (
	SourceFile    = "file"    // a config file was read
	SourceDefault = "default" // built-in defaults (no file, or outside a repository)
)

// Config is everything kiln reads from a repository's config file.
type Config struct {
	Path      string             // the candidate file path ("" outside a repository)
	Source    string             // SourceFile or SourceDefault
	Actions   *model.ActionSet   // the vocabulary (built-in defaults when unset)
	Relations *model.RelationSet // the relation vocabulary (built-in defaults when unset)
	VCS       string             // backend preference: auto | git | jj
	Worktree  string             // isolation mode: off (default) | on
}

// Worktree modes answer one question: may an action be recorded in the main
// checkout, or must it go through a dedicated area?
const (
	WorktreeOn  = "on"  // the repository asked for isolation: the main checkout refuses actions
	WorktreeOff = "off" // default: the main checkout accepts actions
)

// Isolated reports whether actions must go through a dedicated area. The zero
// value — an unset config — means off, so isolation is opt-in: the guard engages
// only when a repository asks for it by name.
func (c Config) Isolated() bool { return c.Worktree == WorktreeOn }

// Read resolves and parses the configuration for startDir. A missing file is not
// an error: kiln falls back to the built-in vocabulary and "auto". A file that
// exists but does not parse is an error, so a misspelled key is never silently
// ignored.
func Read(startDir string) (Config, error) {
	cfg := Config{Actions: model.DefaultActionSet(), Relations: model.DefaultRelationSet(), VCS: "auto", Worktree: WorktreeOff, Source: SourceDefault}
	path, ok := Path(startDir)
	if !ok {
		return cfg, nil // outside a repository: do still works, on the defaults
	}
	cfg.Path = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, i18n.E("读取 %s: %w", path, err)
	}
	stripped, err := stripComments(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	var f file
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Config{}, i18n.E("解析 %s: %w", path, err)
	}
	if v := strings.TrimSpace(f.VCS); v != "" {
		cfg.VCS = v
	}
	wt, err := worktreeMode(f.Worktree)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	cfg.Worktree = wt
	set, err := actionSet(f)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	cfg.Actions = set
	cfg.Source = SourceFile
	return cfg, nil
}

// actionSet turns the actions group into a vocabulary, staying lenient about
// what may be omitted: start defaults to the first name, terminal to the last,
// and an empty group means "keep the built-in defaults".
func actionSet(f file) (*model.ActionSet, error) {
	names := f.Actions.Names
	if len(names) == 0 {
		return model.DefaultActionSet(), nil
	}
	start := f.Actions.Start
	if strings.TrimSpace(start) == "" {
		start = names[0]
	}
	terminal := f.Actions.Terminal
	if len(terminal) == 0 {
		terminal = []string{names[len(names)-1]}
	}
	return model.NewActionSet(names, start, terminal)
}

// worktreeMode normalizes the isolation mode, tolerating case and spacing so a
// hand-edited file reads the same as one kiln wrote. An unknown value is an error
// rather than a silent fallback to the default.
func worktreeMode(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", WorktreeOff:
		return WorktreeOff, nil
	case WorktreeOn:
		return WorktreeOn, nil
	}
	return "", i18n.E("非法 worktree: %q（可选 on / off）", v)
}

// Path resolves the config file for startDir by walking up from it: the first
// <dir>/.kilnconfig.jsonc wins, and the nearest repository root — a directory
// holding .git or .jj — stops the walk and names the canonical location when no
// file exists yet. ok is false outside a repository.
func Path(startDir string) (path string, ok bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		dir = filepath.Clean(startDir)
	}
	for {
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		if isRepoRoot(dir) {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// isRepoRoot reports whether dir holds a repository marker: .git (a directory in
// the main checkout, a file in a linked worktree) or .jj.
func isRepoRoot(dir string) bool {
	for _, marker := range []string{".git", ".jj"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Ensure writes the commented default config at the repository root when none
// exists yet, so a fresh repository has something to edit. An existing file is
// never touched. created reports whether it wrote one.
func Ensure(startDir string) (path string, created bool, err error) {
	path, ok := Path(startDir)
	if !ok {
		return "", false, i18n.E("不在仓库里，定位不到 %s", FileName)
	}
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(i18n.T(defaultFile)), 0o644); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// defaultFile is the seeded config: the built-in vocabulary spelled out, with
// the comments JSONC exists for.
const defaultFile = `// kiln 配置 —— 随仓库提交（项目共享），放在仓库根。
// 位置：<仓库根>/.kilnconfig.jsonc（kiln config path 打印路径）
// 格式：JSONC（JSON + 注释）；除注释外是严格 JSON，不支持尾逗号。
{
  // action 词表：每个项目可以不同 —— 这是 kiln 的自定义点。
  // 顺序不是约束；只需含起点动作，并以终点动作收尾。
  "actions": {
    "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
    "start": "ask",
    "terminal": ["conclude", "apply"]
  },
  // VCS 后端：auto（.jj 优先，否则 .git）/ git / jj
  "vcs": "auto",
  // worktree 模式：off（默认，允许在 main 提交）| on（动作只在 worktree 提交——开工前先 kiln worktree new <issue>）
  "worktree": "off"
}
`

// stripComments blanks out // line comments and /* */ block comments while
// leaving string literals untouched. Comment bytes become spaces, so byte
// offsets — and therefore json's error positions — still line up with the file
// the user is looking at.
func stripComments(src []byte) ([]byte, error) {
	out := make([]byte, len(src))
	for i := 0; i < len(src); {
		switch c := src[i]; {
		case c == '"':
			// Copy a whole string literal verbatim, escapes and all.
			j := i + 1
			for j < len(src) && src[j] != '\n' {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == '"' {
					j++
					break
				}
				j++
			}
			if j > len(src) {
				j = len(src)
			}
			copy(out[i:j], src[i:j])
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := i
			for j < len(src) && src[j] != '\n' {
				out[j] = ' '
				j++
			}
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := bytes.Index(src[i+2:], []byte("*/"))
			if end < 0 {
				line := 1 + bytes.Count(src[:i], []byte("\n"))
				return nil, i18n.E("第 %d 行: 块注释没有闭合的 */", line)
			}
			j := i + 2 + end + 2
			for k := i; k < j; k++ {
				if src[k] == '\n' {
					out[k] = '\n' // keep the line count intact
				} else {
					out[k] = ' '
				}
			}
			i = j
		default:
			out[i] = c
			i++
		}
	}
	return out, nil
}
