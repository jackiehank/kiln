// Package config resolves kiln's action vocabulary from an optional .kiln.json
// in the current directory, falling back to the built-in defaults.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kiln/internal/model"
)

// FileName is the optional config file kiln looks for in the current directory.
const FileName = ".kiln.json"

// file mirrors the on-disk schema.
type file struct {
	Actions  []string `json:"actions"`
	Start    string   `json:"start"`
	Terminal []string `json:"terminal"`
}

// VCS returns the configured backend preference from dir/FileName. It is
// "auto" when the file is absent or the key is unset.
func VCS(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return "auto", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 %s: %w", FileName, err)
	}
	var f struct {
		VCS string `json:"vcs"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("解析 %s: %w", FileName, err)
	}
	if strings.TrimSpace(f.VCS) == "" {
		return "auto", nil
	}
	return f.VCS, nil
}

// Load returns the action vocabulary from dir/FileName, or the built-in defaults
// when the file is absent. A present-but-invalid file is an error.
//
// When `start` or `terminal` are omitted they default to the first and the last
// entry of `actions`, so a minimal file only needs to list the vocabulary.
func Load(dir string) (*model.ActionSet, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return model.DefaultActionSet(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", FileName, err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", FileName, err)
	}
	if len(f.Actions) == 0 {
		return model.DefaultActionSet(), nil
	}
	if f.Start == "" {
		f.Start = f.Actions[0]
	}
	if len(f.Terminal) == 0 {
		f.Terminal = []string{f.Actions[len(f.Actions)-1]}
	}
	set, err := model.NewActionSet(f.Actions, f.Start, f.Terminal)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return set, nil
}
