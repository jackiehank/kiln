// Package engine turns a stream of raw commits into a repository of issues and
// answers the questions the CLI asks: what is the path of this issue, is the
// repository semantically complete, how do issues relate.
package engine

import (
	"sort"
	"strings"

	"kiln/internal/model"
	"kiln/internal/trailer"
	"kiln/internal/vcs"
)

// ActionsFromCommits keeps only the kiln commits and returns them in
// chronological order (git log hands them to us newest-first).
func ActionsFromCommits(commits []vcs.Commit, rel *model.RelationSet) []model.Action {
	var out []model.Action
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		f, ok := trailer.Parse(c.Message, rel)
		if !ok {
			continue
		}
		out = append(out, model.Action{
			ID:        c.ID,
			Subject:   subject(c.Message),
			Date:      c.Date,
			Issue:     f.Issue,
			Kind:      f.Action,
			Title:     f.Title,
			Refs:      f.Refs,
			Target:    f.Target,
			Scope:     f.Scope,
			Domain:    f.Domain,
			Tags:      f.Tags,
			Relations: f.Relations,
			Unrelate:  f.Unrelate,
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
func Build(actions []model.Action, rel *model.RelationSet) *model.Repo {
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
	}
	for _, p := range repo.Issues {
		sort.SliceStable(p.Actions, func(i, j int) bool {
			return p.Actions[i].Date.Before(p.Actions[j].Date)
		})
		p.Relations = foldRelations(p, rel)
	}
	return repo
}

// foldRelations computes an issue's outbound view: at most one relation per
// (target, slot), the strongest by rank; ties go to the later action. A
// declaration inside an action outranks that action's unrelate for the same
// target, so `unrelate: X` + `blocks: X` means "clear, then keep blocks".
func foldRelations(p *model.Issue, rel *model.RelationSet) []model.Rel {
	type key struct{ target, slot string }
	state := map[key]model.Rel{}
	for _, a := range p.Actions {
		declared := map[string]bool{}
		for _, r := range a.Relations {
			declared[r.Target] = true
		}
		for _, t := range a.Unrelate {
			if declared[t] {
				continue
			}
			for k := range state {
				if k.target == t {
					delete(state, k)
				}
			}
		}
		for _, r := range a.Relations {
			n, ok := rel.Lookup(r.Name)
			if !ok {
				continue
			}
			k := key{r.Target, n.Slot}
			if cur, ok := state[k]; ok {
				if c, ok := rel.Lookup(cur.Name); ok && c.Rank > n.Rank {
					continue
				}
			}
			state[k] = r
		}
	}
	out := make([]model.Rel, 0, len(state))
	for _, v := range state {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Name < out[j].Name
	})
	return out
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

// Rule codes name the finding a diagnostic carries. They are the contract a
// machine view reports, so the text view keeps its sentence and the machine
// view binds to these words: a caller branches on `derived-from`, never on
// Chinese prose. Each code doubles as the trailer it checks where one exists.
const (
	RuleStartAction    = "start-action"
	RuleTerminalAction = "terminal-action"
	RuleRelationTarget = "relation-target"
	RuleDerivedFrom    = "derived-from"
	RuleSpawns         = "spawns"
	RuleComplete       = "complete"
)

// Diagnostic is one finding from Check.
type Diagnostic struct {
	Level   Level
	Issue   string
	Message string

	// Rule names the finding; Action and Ref carry the little bit of context the
	// rule needs (the last action, the issue that is missing), so a machine view
	// never has to re-derive it from the sentence. Both are empty when the rule
	// has no such context.
	Rule   string
	Action string
	Ref    string
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
		diags = append(diags, Diagnostic{Level: LevelErr, Issue: p.ID, Rule: RuleStartAction,
			Message: "缺少 " + actions.Start() + "：问题没有起点动作"})
	}

	// Rule 2: the final action must be terminal. Order is not checked.
	if last, ok := p.Last(); ok && !actions.IsTerminal(last.Kind) {
		diags = append(diags, Diagnostic{Level: LevelWarn, Issue: p.ID, Rule: RuleTerminalAction, Action: last.Kind,
			Message: "缺少终点动作（当前 " + last.Kind + "）"})
	}

	// Rule 3: every relation target must exist — for every relation name, not
	// just derived-from/spawns. A dangling target is a warning (lenient input).
	for _, r := range p.Relations {
		if _, ok := repo.Issues[r.Target]; !ok {
			diags = append(diags, Diagnostic{Level: LevelWarn, Issue: p.ID, Rule: RuleRelationTarget, Ref: r.Target,
				Message: "关系目标不存在：" + r.Name + " 指向 " + r.Target})
		}
	}

	if len(diags) == 0 {
		diags = append(diags, Diagnostic{Level: LevelOK, Issue: p.ID, Rule: RuleComplete,
			Message: "完整（" + strings.Join(kinds(p), ", ") + "）"})
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
func Graph(repo *model.Repo, rel *model.RelationSet) string {
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
		lines := relationLines(repo, p, rel)
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

func relationLines(repo *model.Repo, p *model.Issue, rel *model.RelationSet) []string {
	var lines []string
	own := map[string]bool{} // target|slot already shown from this issue
	for _, r := range p.Relations {
		if n, ok := rel.Lookup(r.Name); ok {
			own[r.Target+"|"+n.Slot] = true
		}
		lines = append(lines, r.Name+" "+r.Target+" ("+stateOf(repo, r.Target)+")")
	}
	// Inbound: edges other issues declared toward this one. Rendered with the
	// inverse name so the sentence still reads with p as the subject.
	for _, id := range IDs(repo) {
		q := repo.Issues[id]
		if q.ID == p.ID {
			continue
		}
		for _, r := range q.Relations {
			if r.Target != p.ID {
				continue
			}
			if n, ok := rel.Lookup(r.Name); ok && own[q.ID+"|"+n.Slot] {
				continue
			}
			label := rel.Inverse(r.Name)
			if label == "" {
				label = r.Name
			}
			lines = append(lines, label+" "+q.ID+" ("+stateOf(repo, q.ID)+")")
		}
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

// NoDomain is the bucket for issues that never declared a domain.
const NoDomain = "(未分类)"

// DomainGroup is one domain block: a domain name and the issues that named it in
// their own `domain:` trailer, sorted by ID.
type DomainGroup struct {
	Domain string
	Issues []*model.Issue
}

// GroupByDomain arranges the repository as domain blocks for `kiln status`.
//
// Blocks are keyed by each issue's **own** domain and sorted by domain name, with
// NoDomain last; issues inside a block are sorted by ID (IDs is already sorted).
// An issue belongs to exactly one block, and relations never relocate it: the
// domain view is a directory, not a relation view — cross-domain edges belong to
// `kiln graph`. Relocating them here once made the domain column ambiguous (a
// nested row carried a different domain than the block it sat in) and cost this
// function a parent map, a cycle check and a recursive walk.
func GroupByDomain(repo *model.Repo) []DomainGroup {
	groups := map[string]*DomainGroup{}
	for _, id := range IDs(repo) {
		p := repo.Issues[id]
		key := domainKey(p)
		g := groups[key]
		if g == nil {
			g = &DomainGroup{Domain: key}
			groups[key] = g
		}
		g.Issues = append(g.Issues, p)
	}

	out := make([]DomainGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Domain == NoDomain) != (out[j].Domain == NoDomain) {
			return out[j].Domain == NoDomain
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

// CountDomains returns how many groups are real domains (NoDomain excluded).
func CountDomains(groups []DomainGroup) int {
	n := 0
	for _, g := range groups {
		if g.Domain != NoDomain {
			n++
		}
	}
	return n
}

// domainKey returns the grouping key for an issue.
func domainKey(p *model.Issue) string {
	if p.Domain == "" {
		return NoDomain
	}
	return p.Domain
}
