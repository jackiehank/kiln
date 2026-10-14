// Package model defines kiln's domain types: a parsed action, an aggregated
// problem, and the repository-wide aggregation. Nothing here touches git or the
// filesystem, so the whole package is trivially testable.
package model

import (
	"strings"
	"time"
)

// Action kinds, roughly in the order a research loop flows.
const (
	Ask         = "ask"
	Hypothesize = "hypothesize"
	Collect     = "collect"
	Test        = "test"
	Conclude    = "conclude"
	Apply       = "apply"
)

// ValidActions lists every legal action kind.
var ValidActions = []string{Ask, Hypothesize, Collect, Test, Conclude, Apply}

// IsValidAction reports whether kind is a known action kind.
func IsValidAction(kind string) bool {
	for _, a := range ValidActions {
		if a == kind {
			return true
		}
	}
	return false
}

// IsTerminal reports whether kind closes a research loop.
func IsTerminal(kind string) bool {
	return kind == Conclude || kind == Apply
}

// Action is a single git commit recognised as a kiln action.
type Action struct {
	Hash    string
	Subject string
	Date    time.Time

	Problem string
	Kind    string
	Object  string
	Title   string
	Refs    []string
	Target  string
	Scope   string
	Domain  string
	Tags    []string

	DerivedFrom []string
	Spawns      []string
	RelatesTo   []string
	Blocks      []string
	Supersedes  []string
}

// Problem aggregates every action that shares one problem ID, plus the union of
// the relationship and classification trailers declared across those actions.
type Problem struct {
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
func (p *Problem) Last() (Action, bool) {
	if len(p.Actions) == 0 {
		return Action{}, false
	}
	return p.Actions[len(p.Actions)-1], true
}

// State returns the kind of the final action, or "" when the problem has none.
func (p *Problem) State() string {
	if a, ok := p.Last(); ok {
		return a.Kind
	}
	return ""
}

// Repo is the whole-repository aggregation produced by the engine.
type Repo struct {
	Problems map[string]*Problem
}

// ValidProblemID reports whether id follows the `<namespace>/<slug>` convention:
// at least one slash, every segment a non-empty lowercase kebab-case token, and
// the whole ID at most 50 characters.
func ValidProblemID(id string) bool {
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
