// Package model defines kiln's domain types: a parsed action, an aggregated
// issue, the repository-wide aggregation, and the (configurable) action
// vocabulary. Nothing here touches git or the filesystem, so the whole package
// is trivially testable.
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Built-in action names, used when no config file overrides them.
const (
	Ask         = "ask"
	Hypothesize = "hypothesize"
	Collect     = "collect"
	Test        = "test"
	Conclude    = "conclude"
	Apply       = "apply"
)

// Defaults describe the built-in vocabulary.
var (
	DefaultActions  = []string{Ask, Hypothesize, Collect, Test, Conclude, Apply}
	DefaultStart    = Ask
	DefaultTerminal = []string{Conclude, Apply}
)

// ActionSet is the vocabulary of actions kiln recognises.
//
// Order is deliberately not a constraint. The start action (default `ask`) only
// opens an issue; every action after it belongs to an unordered combination.
// The sole structural rules are "contains the start action" and "ends with a
// terminal action".
type ActionSet struct {
	names    []string
	valid    map[string]bool
	start    string
	terminal map[string]bool
}

// DefaultActionSet returns the built-in vocabulary.
func DefaultActionSet() *ActionSet {
	set, err := NewActionSet(DefaultActions, DefaultStart, DefaultTerminal)
	if err != nil {
		panic(err) // the built-in defaults are valid by construction
	}
	return set
}

// NewActionSet validates a vocabulary and builds a set. Names are trimmed and
// lower-cased; empty names, duplicates, a start outside the set, or an empty or
// out-of-set terminal list are rejected.
func NewActionSet(names []string, start string, terminal []string) (*ActionSet, error) {
	if len(names) == 0 {
		return nil, errors.New("actions 不能为空")
	}
	s := &ActionSet{valid: map[string]bool{}, terminal: map[string]bool{}}
	for _, raw := range names {
		n := normalizeName(raw)
		if n == "" {
			return nil, errors.New("action 名称不能为空")
		}
		if s.valid[n] {
			return nil, fmt.Errorf("action %q 重复", n)
		}
		s.valid[n] = true
		s.names = append(s.names, n)
	}
	s.start = normalizeName(start)
	if s.start == "" {
		return nil, errors.New("start 不能为空")
	}
	if !s.valid[s.start] {
		return nil, fmt.Errorf("start %q 不在 actions 中", s.start)
	}
	if len(terminal) == 0 {
		return nil, errors.New("terminal 不能为空")
	}
	for _, raw := range terminal {
		t := normalizeName(raw)
		if !s.valid[t] {
			return nil, fmt.Errorf("terminal %q 不在 actions 中", t)
		}
		s.terminal[t] = true
	}
	return s, nil
}

func normalizeName(n string) string { return strings.ToLower(strings.TrimSpace(n)) }

// Names returns the vocabulary in declaration order.
func (s *ActionSet) Names() []string { return append([]string(nil), s.names...) }

// Start returns the action that opens an issue.
func (s *ActionSet) Start() string { return s.start }

// IsValid reports whether name is part of the vocabulary.
func (s *ActionSet) IsValid(name string) bool { return s.valid[name] }

// IsStart reports whether name opens an issue.
func (s *ActionSet) IsStart(name string) bool { return name == s.start }

// IsTerminal reports whether name closes a research loop.
func (s *ActionSet) IsTerminal(name string) bool { return s.terminal[name] }

// Action is a single git commit recognised as a kiln action.
type Action struct {
	Hash    string
	Subject string
	Date    time.Time

	Issue  string
	Kind   string
	Object string
	Title  string
	Refs   []string
	Target string
	Scope  string
	Domain string
	Tags   []string

	DerivedFrom []string
	Spawns      []string
	RelatesTo   []string
	Blocks      []string
	Supersedes  []string
}

// Issue aggregates every action that shares one issue ID, plus the union of
// the relationship and classification trailers declared across those actions.
type Issue struct {
	ID      string
	Actions []Action

	Domain string
	Tags   []string

	DerivedFrom []string
	Spawns      []string
	RelatesTo   []string
	Blocks      []string
	Supersedes  []string
}

// Last returns the final action in the path.
func (p *Issue) Last() (Action, bool) {
	if len(p.Actions) == 0 {
		return Action{}, false
	}
	return p.Actions[len(p.Actions)-1], true
}

// State returns the kind of the final action, or "" when the issue has none.
func (p *Issue) State() string {
	if a, ok := p.Last(); ok {
		return a.Kind
	}
	return ""
}

// Repo is the whole-repository aggregation produced by the engine.
type Repo struct {
	Issues map[string]*Issue
}

// ValidIssueID reports whether id follows the `<namespace>/<slug>` convention:
// at least one slash, every segment a non-empty lowercase kebab-case token, and
// the whole ID at most 50 characters.
func ValidIssueID(id string) bool {
	if id == "" || len(id) > 50 {
		return false
	}
	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if !validSegment(p) {
			return false
		}
	}
	return true
}

func validSegment(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
