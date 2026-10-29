// Package engine turns a stream of raw commits into a repository of issues and
// answers the questions the CLI asks: what is the path of this issue, is the
// repository semantically complete, how do issues relate.
package engine

import (
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/cache"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/trailer"
	"github.com/jackiehank/kiln/internal/vcs"
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
			Subject:   Subject(c.Message),
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

			UnknownTrailerKeys: trailer.UnknownKeys(c.Message, rel),
		})
	}
	return out
}

// Subject returns the first non-empty line of a commit message: the one-line
// label a listing shows when the commit carries no `title:` trailer. It is
// exported because `action log` needs exactly this fallback for the same reason
// `list` does.
func Subject(message string) string {
	for _, line := range strings.Split(message, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}

// recordFrom turns one already-parsed commit into the record the cache stores.
// Parsing is passed in so the cold path parses each message exactly once.
func recordFrom(c vcs.Commit, f trailer.Fields, ok bool, rel *model.RelationSet) cache.Record {
	var rec cache.Record
	if oid, err := hex.DecodeString(c.ID); err == nil && len(oid) == len(rec.OID) {
		copy(rec.OID[:], oid)
	}
	rec.Date = c.Date.Unix()
	if _, offset := c.Date.Zone(); offset != 0 {
		rec.TZ = int16(offset / 60)
	}
	rec.Action = ok
	rec.Issue = f.Issue
	rec.Kind = f.Action
	rec.Title = f.Title
	rec.Subject = Subject(c.Message)
	rec.Domain = f.Domain
	rec.Scope = f.Scope
	rec.Target = f.Target
	rec.Refs = f.Refs
	rec.Tags = f.Tags
	rec.Unrelate = f.Unrelate
	if ok {
		// Unknown keys are a `check` diagnostic and only actions are checked; not
		// scanning every non-action message keeps the cold path cheaper.
		rec.Unknown = trailer.UnknownKeys(c.Message, rel)
	}
	for _, r := range f.Relations {
		rec.Relations = append(rec.Relations, cache.Rel{Name: r.Name, Target: r.Target})
	}
	return rec
}

// RecordFromCommit turns one commit into the record the cache stores: the fields
// ActionsFromCommits derives plus the author date, so an Action can be rebuilt
// without the message.
func RecordFromCommit(c vcs.Commit, rel *model.RelationSet) cache.Record {
	f, ok := trailer.Parse(c.Message, rel)
	return recordFrom(c, f, ok, rel)
}

// ActionsAndRecordsFromCommits parses the history once and returns both the
// actions and the records the cache stores, so a cold read pays one parse rather
// than two. The Action is built from the commit itself (id and date verbatim), so
// a full scan keeps producing exactly what it produced before the cache existed.
func ActionsAndRecordsFromCommits(commits []vcs.Commit, rel *model.RelationSet) ([]model.Action, []cache.Record) {
	actions := make([]model.Action, 0, len(commits))
	records := make([]cache.Record, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		f, ok := trailer.Parse(c.Message, rel)
		rec := recordFrom(c, f, ok, rel)
		records = append(records, rec)
		if ok {
			actions = append(actions, actionFrom(c.ID, c.Date, Subject(c.Message), f, rec.Unknown))
		}
	}
	return actions, records
}

// ActionsFromRecords is the cache-side twin of ActionsFromCommits: same fields,
// same order (records are in newest-first enumeration order, actions are emitted
// oldest first), so a cache hit and a full scan produce identical actions.
func ActionsFromRecords(recs []cache.Record) []model.Action {
	var out []model.Action
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		if !r.Action {
			continue
		}
		out = append(out, actionFrom(
			hex.EncodeToString(r.OID[:]),
			time.Unix(r.Date, 0).In(time.FixedZone("", int(r.TZ)*60)),
			r.Subject,
			FieldsFromRecord(r),
			r.Unknown,
		))
	}
	return out
}

// FieldsFromRecord rebuilds the trailer fields the action needs from a record.
func FieldsFromRecord(r cache.Record) trailer.Fields {
	f := trailer.Fields{
		Issue: r.Issue, Action: r.Kind, Title: r.Title, Refs: r.Refs,
		Target: r.Target, Scope: r.Scope, Domain: r.Domain, Tags: r.Tags,
		Unrelate: r.Unrelate,
	}
	for _, rel := range r.Relations {
		f.Relations = append(f.Relations, model.Rel{Name: rel.Name, Target: rel.Target})
	}
	return f
}

// actionFrom is the single place an Action is assembled, so the commit path and
// the record path cannot drift apart.
func actionFrom(id string, date time.Time, subject string, f trailer.Fields, unknown []string) model.Action {
	return model.Action{
		ID:                 id,
		Subject:            subject,
		Date:               date,
		Issue:              f.Issue,
		Kind:               f.Action,
		Title:              f.Title,
		Refs:               f.Refs,
		Target:             f.Target,
		Scope:              f.Scope,
		Domain:             f.Domain,
		Tags:               f.Tags,
		Relations:          f.Relations,
		Unrelate:           f.Unrelate,
		UnknownTrailerKeys: unknown,
	}
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
// view binds to these words: a caller branches on `relation-target`, never on
// Chinese prose. The relation code is generic on purpose: every relation name
// reports the same code, because the finding (a target that does not exist) is
// the same.
const (
	RuleStartAction    = "start-action"
	RuleTerminalAction = "terminal-action"
	RuleRelationTarget = "relation-target"
	RuleUnknownTrailer = "unknown-trailer"
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
	// has no such context. Relation names the declared relation behind a
	// relation-target finding, so a caller can tell two edges to the same
	// missing target apart.
	Rule     string
	Action   string
	Ref      string
	Relation string
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
			Message: i18n.T("缺少 %s：问题没有起点动作", actions.Start())})
	}

	// Rule 2: the final action must be terminal. Order is not checked.
	if last, ok := p.Last(); ok && !actions.IsTerminal(last.Kind) {
		diags = append(diags, Diagnostic{Level: LevelWarn, Issue: p.ID, Rule: RuleTerminalAction, Action: last.Kind,
			Message: i18n.T("缺少终点动作（当前 %s）", last.Kind)})
	}

	// Rule 3: every relation target must exist — for every relation name, not
	// just derived-from/spawns. A dangling target is a warning (lenient input).
	for _, r := range p.Relations {
		if _, ok := repo.Issues[r.Target]; !ok {
			diags = append(diags, Diagnostic{Level: LevelWarn, Issue: p.ID, Rule: RuleRelationTarget, Ref: r.Target, Relation: r.Name,
				Message: i18n.T("关系目标不存在：%s 指向 %s", r.Name, r.Target)})
		}
	}

	// Rule 4: unknown trailer keys are surfaced, not silently dropped.
	for _, a := range p.Actions {
		for _, key := range a.UnknownTrailerKeys {
			diags = append(diags, Diagnostic{Level: LevelWarn, Issue: p.ID, Rule: RuleUnknownTrailer, Action: a.Kind, Ref: key,
				Message: i18n.T("未知 trailer 键: %s", key)})
		}
	}

	if len(diags) == 0 {
		diags = append(diags, Diagnostic{Level: LevelOK, Issue: p.ID, Rule: RuleComplete,
			Message: i18n.T("完整（%s）", strings.Join(kinds(p), ", "))})
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

// Edge is one relationship after the whole-repository fold: the declaring
// issue (the arrow's tail), the target, the name it declared, and the slot the
// name lives in.
type Edge struct {
	From string
	To   string
	Name string
	Slot string
}

// FoldEdges folds every declaration in the repository by (unordered pair, slot)
// — last declaration wins, whichever side made it — so a relationship declared
// from both sides shows one shape once. Within one issue the strength chain
// still decides: a later weaker name does not displace a stronger earlier one
// (higher rank wins, a tie goes to the later action), which is exactly
// foldRelations' rule.
func FoldEdges(repo *model.Repo, rel *model.RelationSet) []Edge {
	type item struct {
		issue string
		a     model.Action
	}
	var items []item
	for _, id := range IDs(repo) {
		for _, a := range repo.Issues[id].Actions {
			items = append(items, item{id, a})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].a.Date.Equal(items[j].a.Date) {
			return items[i].a.Date.Before(items[j].a.Date)
		}
		return items[i].issue < items[j].issue
	})

	type key struct{ pair, slot string }
	pair := func(a, b string) string {
		if a > b {
			a, b = b, a
		}
		return a + "\x00" + b
	}
	state := map[key]Edge{}
	for _, it := range items {
		declared := map[string]bool{}
		for _, r := range it.a.Relations {
			declared[r.Target] = true
		}
		// An unrelate clears the pair as a whole: it says the two issues are no
		// longer related, not merely that this side stopped declaring.
		for _, t := range it.a.Unrelate {
			if declared[t] {
				continue
			}
			p := pair(it.issue, t)
			for k := range state {
				if k.pair == p {
					delete(state, k)
				}
			}
		}
		for _, r := range it.a.Relations {
			n, ok := rel.Lookup(r.Name)
			if !ok {
				continue
			}
			k := key{pair(it.issue, r.Target), n.Slot}
			if cur, ok := state[k]; ok && cur.From == it.issue {
				if c, ok := rel.Lookup(cur.Name); ok && c.Rank > n.Rank {
					continue
				}
			}
			state[k] = Edge{From: it.issue, To: r.Target, Name: r.Name, Slot: n.Slot}
		}
	}
	out := make([]Edge, 0, len(state))
	for _, e := range state {
		out = append(out, e)
	}
	return out
}

// Graph renders the normalized relationship graph: one block per issue, and in
// it only the edges whose declaration this issue made — the declared name is the
// arrow's label, never an inverse, and a target never repeats its own block's
// edge back. A symmetric name (one without an inverse) is drawn as a plain line
// instead of an arrow.
func Graph(repo *model.Repo, rel *model.RelationSet) string {
	byFrom := map[string][]Edge{}
	for _, e := range FoldEdges(repo, rel) {
		byFrom[e.From] = append(byFrom[e.From], e)
	}
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
		lines := edgeLines(repo, byFrom[id], rel)
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

// edgeLines renders one issue's edges as arrows, sorted by (target, name).
func edgeLines(repo *model.Repo, edges []Edge, rel *model.RelationSet) []string {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Name < edges[j].Name
	})
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		state := stateOf(repo, e.To)
		if rel.Inverse(e.Name) == "" {
			out = append(out, "──"+e.Name+"── "+e.To+" ("+state+")")
			continue
		}
		out = append(out, "──"+e.Name+"──▶ "+e.To+" ("+state+")")
	}
	return out
}

// Mermaid renders the same folded graph as a Mermaid flowchart, so the plain
// terminal view can be handed to a renderer unchanged: one node per issue
// carrying its state, one labelled arrow per directed edge, a plain line per
// symmetric edge. Node ids are kept to [A-Za-z0-9_] and made unique, labels are
// quoted, and both lists are sorted so the same repository always prints the
// same text.
func Mermaid(repo *model.Repo, rel *model.RelationSet) string {
	edges := FoldEdges(repo, rel)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Name < edges[j].Name
	})

	var b strings.Builder
	b.WriteString("flowchart LR\n")
	name := map[string]string{}
	taken := map[string]bool{}
	node := func(id, state string) {
		base := "i_" + sanitizeNodeID(id)
		n := base
		for i := 2; taken[n]; i++ {
			n = base + "_" + strconv.Itoa(i)
		}
		taken[n] = true
		name[id] = n
		b.WriteString("  " + n + "[\"" + quoteLabel(id+" ("+state+")") + "\"]\n")
	}

	ids := make([]string, 0, len(repo.Issues))
	for id := range repo.Issues {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		state := repo.Issues[id].State()
		if state == "" {
			state = "?"
		}
		node(id, state)
	}

	// A declaration may point at an issue no commit ever opened: the text view
	// draws it as (missing), so the flowchart needs a node for it too.
	missing := map[string]bool{}
	for _, e := range edges {
		if _, ok := repo.Issues[e.To]; !ok {
			missing[e.To] = true
		}
	}
	targets := make([]string, 0, len(missing))
	for id := range missing {
		targets = append(targets, id)
	}
	sort.Strings(targets)
	for _, id := range targets {
		node(id, "missing")
	}

	for _, e := range edges {
		if rel.Inverse(e.Name) == "" {
			b.WriteString("  " + name[e.From] + " --- " + name[e.To] + "\n")
			continue
		}
		b.WriteString("  " + name[e.From] + " -- " + e.Name + " --> " + name[e.To] + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// sanitizeNodeID keeps only [A-Za-z0-9_] so an id never needs quoting.
func sanitizeNodeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

// quoteLabel escapes a quoted Mermaid label; the only character Mermaid reads
// specially inside one is the double quote.
func quoteLabel(s string) string {
	return strings.ReplaceAll(s, "\"", "#quot;")
}

// PathLines renders issue p's relation block for `kiln path`: p's own
// declarations first, under the name it declared, then the inbound edges — the
// edges other issues declared toward p — flipped to the inverse name so every
// line still reads with p as the subject. One relation per (target, slot), out
// before in, each sorted by (target, name). A name that only appears because it
// was flipped carries its glossary the first time it shows, so the reader learns
// the inverse once without the block turning into a chart.
func PathLines(repo *model.Repo, p *model.Issue, rel *model.RelationSet) []string {
	type entry struct {
		name    string
		target  string
		flipped bool
	}
	var out, in []entry
	seen := map[string]bool{} // target|slot already shown from p's own declarations
	for _, r := range p.Relations {
		n, ok := rel.Lookup(r.Name)
		if !ok {
			continue
		}
		seen[r.Target+"|"+n.Slot] = true
		out = append(out, entry{name: r.Name, target: r.Target})
	}
	for _, id := range IDs(repo) {
		q := repo.Issues[id]
		if q.ID == p.ID {
			continue
		}
		for _, r := range q.Relations {
			if r.Target != p.ID {
				continue
			}
			n, ok := rel.Lookup(r.Name)
			if !ok || seen[q.ID+"|"+n.Slot] {
				continue
			}
			label := rel.Inverse(r.Name)
			flipped := label != ""
			if !flipped {
				label = r.Name // symmetric: the name reads the same from either side
			}
			in = append(in, entry{name: label, target: q.ID, flipped: flipped})
		}
	}
	sortEntries := func(es []entry) {
		sort.Slice(es, func(i, j int) bool {
			if es[i].target != es[j].target {
				return es[i].target < es[j].target
			}
			return es[i].name < es[j].name
		})
	}
	sortEntries(out)
	sortEntries(in)

	var lines []string
	glossed := map[string]bool{}
	for _, e := range out {
		lines = append(lines, e.name+" "+e.target)
		glossed[e.name] = true // a name the issue declared needs no introduction
	}
	for _, e := range in {
		line := e.name + " " + e.target
		if e.flipped && !glossed[e.name] {
			if g := rel.Gloss(e.name); g != "" {
				line += i18n.T("（%s ＝ %s）", e.name, i18n.T(g))
			}
		}
		glossed[e.name] = true
		lines = append(lines, line)
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
