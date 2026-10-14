// Package engine turns a stream of raw commits into a repository of issues and
// answers the questions the CLI asks: what is the path of this issue, is the
// repository semantically complete, how do issues relate.
package engine

import (
	"sort"
	"strings"

	"kiln/internal/git"
	"kiln/internal/model"
	"kiln/internal/trailer"
)

// ActionsFromCommits keeps only the kiln commits and returns them in
// chronological order (git log hands them to us newest-first).
func ActionsFromCommits(commits []git.Commit) []model.Action {
	var out []model.Action
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		f, ok := trailer.Parse(c.Message)
		if !ok {
			continue
		}
		out = append(out, model.Action{
			Hash:        c.Hash,
			Subject:     subject(c.Message),
			Date:        c.Date,
			Issue:       f.Issue,
			Kind:        f.Action,
			Object:      f.Object,
			Title:       f.Title,
			Refs:        f.Refs,
			Target:      f.Target,
			Scope:       f.Scope,
			Domain:      f.Domain,
			Tags:        f.Tags,
			DerivedFrom: f.DerivedFrom,
			Spawns:      f.Spawns,
			RelatesTo:   f.RelatesTo,
			Blocks:      f.Blocks,
			Supersedes:  f.Supersedes,
		})
	}
	return out
}

// subject returns the first non-empty line of a commit message.
func subject(message string) string {
	for _, line := range strings.Split(message, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}

// Build aggregates actions into a repository of issues.
func Build(actions []model.Action) *model.Repo {
	repo := &model.Repo{Issues: map[string]*model.Issue{}}
	for _, a := range actions {
		p := repo.Issues[a.Issue]
		if p == nil {
			p = &model.Issue{ID: a.Issue}
			repo.Issues[a.Issue] = p
		}
		p.Actions = append(p.Actions, a)
		if a.Domain != "" {
			p.Domain = a.Domain
		}
		p.Tags = appendUnique(p.Tags, a.Tags...)
		p.DerivedFrom = appendUnique(p.DerivedFrom, a.DerivedFrom...)
		p.Spawns = appendUnique(p.Spawns, a.Spawns...)
		p.RelatesTo = appendUnique(p.RelatesTo, a.RelatesTo...)
		p.Blocks = appendUnique(p.Blocks, a.Blocks...)
		p.Supersedes = appendUnique(p.Supersedes, a.Supersedes...)
	}
	for _, p := range repo.Issues {
		sort.SliceStable(p.Actions, func(i, j int) bool {
			return p.Actions[i].Date.Before(p.Actions[j].Date)
		})
	}
	return repo
}

// IDs returns issue IDs sorted for stable output.
func IDs(repo *model.Repo) []string {
	ids := make([]string, 0, len(repo.Issues))
	for id := range repo.Issues {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func appendUnique(dst []string, vals ...string) []string {
	for _, v := range vals {
		if !containsStr(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func hasKind(p *model.Issue, kind string) bool {
	for _, a := range p.Actions {
		if a.Kind == kind {
			return true
		}
	}
	return false
}

// Level classifies a diagnostic.
type Level int

const (
	LevelOK Level = iota
	LevelWarn
	LevelErr
)

// Diagnostic is one finding from Check.
type Diagnostic struct {
	Level   Level
	Issue   string
	Message string
}

// Symbol returns the ✓ / ⚠ / ✗ marker for the level.
func (d Diagnostic) Symbol() string {
	switch d.Level {
	case LevelOK:
		return "✓"
	case LevelWarn:
		return "⚠"
	default:
		return "✗"
	}
}

// Check applies the MVP completeness rules (plan §7) to every issue. The
// action vocabulary comes from actions so custom configs are honoured.
func Check(repo *model.Repo, actions *model.ActionSet) []Diagnostic {
	var out []Diagnostic
	for _, id := range IDs(repo) {
		out = append(out, checkIssue(repo, repo.Issues[id], actions)...)
	}
	return out
}

func checkIssue(repo *model.Repo, p *model.Issue, actions *model.ActionSet) []Diagnostic {
	var diags []Diagnostic

	// Rule 1: a issue must contain the start action (default ask).
	if !hasKind(p, actions.Start()) {
		diags = append(diags, Diagnostic{LevelErr, p.ID, "缺少 " + actions.Start() + "：问题没有起点动作"})
	}

	// Rule 2: the final action must be terminal. Order is not checked.
	if last, ok := p.Last(); ok && !actions.IsTerminal(last.Kind) {
		diags = append(diags, Diagnostic{LevelWarn, p.ID, "缺少终点动作（当前 " + last.Kind + "）"})
	}

	// Rule 3: every derived-from parent must exist.
	for _, parent := range p.DerivedFrom {
		if _, ok := repo.Issues[parent]; !ok {
			diags = append(diags, Diagnostic{LevelErr, p.ID, "父问题不存在：derived-from 指向 " + parent})
		}
	}

	// Rule 4: every spawned child must exist.
	for _, child := range p.Spawns {
		if _, ok := repo.Issues[child]; !ok {
			diags = append(diags, Diagnostic{LevelErr, p.ID, "子问题不存在：spawns 指向 " + child})
		}
	}

	if len(diags) == 0 {
		diags = append(diags, Diagnostic{LevelOK, p.ID, "完整（" + strings.Join(kinds(p), ", ") + "）"})
	}
	return diags
}

func kinds(p *model.Issue) []string {
	out := make([]string, 0, len(p.Actions))
	for _, a := range p.Actions {
		out = append(out, a.Kind)
	}
	return out
}

// FilterOptions narrows a list of issues.
type FilterOptions struct {
	Domain string
	Tag    string
	Action string
}

// Filter returns issue IDs matching every provided option.
func Filter(repo *model.Repo, o FilterOptions) []string {
	var out []string
	for _, id := range IDs(repo) {
		p := repo.Issues[id]
		if o.Domain != "" && p.Domain != o.Domain {
			continue
		}
		if o.Tag != "" && !containsStr(p.Tags, o.Tag) {
			continue
		}
		if o.Action != "" && !hasKind(p, o.Action) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// Stats summarises the repository.
type Stats struct {
	Total      int
	Complete   int
	InProgress int
	Actions    int
}

// Summarize computes repository-level counters.
func Summarize(repo *model.Repo, actions *model.ActionSet) Stats {
	var s Stats
	s.Total = len(repo.Issues)
	for _, id := range IDs(repo) {
		p := repo.Issues[id]
		s.Actions += len(p.Actions)
		if last, ok := p.Last(); ok && actions.IsTerminal(last.Kind) {
			s.Complete++
		} else {
			s.InProgress++
		}
	}
	return s
}

// Graph renders an ASCII relationship tree: one block per issue, listing the
// issues it spawns, relates to and derives from.
func Graph(repo *model.Repo) string {
	var b strings.Builder
	for i, id := range IDs(repo) {
		if i > 0 {
			b.WriteByte('\n')
		}
		p := repo.Issues[id]
		state := p.State()
		if state == "" {
			state = "?"
		}
		b.WriteString(id + " (" + state + ")\n")
		lines := relationLines(repo, p)
		for j, l := range lines {
			if j == len(lines)-1 {
				b.WriteString("└── " + l + "\n")
			} else {
				b.WriteString("├── " + l + "\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func relationLines(repo *model.Repo, p *model.Issue) []string {
	var lines []string
	for _, c := range p.Spawns {
		lines = append(lines, "spawns "+c+" ("+stateOf(repo, c)+")")
	}
	for _, c := range p.RelatesTo {
		lines = append(lines, "related "+c+" ("+stateOf(repo, c)+")")
	}
	for _, c := range p.DerivedFrom {
		lines = append(lines, "derived-from "+c+" ("+stateOf(repo, c)+")")
	}
	return lines
}

func stateOf(repo *model.Repo, id string) string {
	if p, ok := repo.Issues[id]; ok {
		if s := p.State(); s != "" {
			return s
		}
	}
	return "missing"
}
