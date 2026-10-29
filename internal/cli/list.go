package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

func (e *env) isDone(p *model.Issue) bool {
	last, ok := p.Last()
	return ok && e.actions.IsTerminal(last.Kind)
}

// filterStatus keeps the issues whose current step is (`done`) or is not
// (`open`) a terminal action. It reads the same predicate as the row's `done`,
// so a filter and the rows it returns can never disagree.
func (e *env) filterStatus(repo *model.Repo, ids []string, done bool) []string {
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if e.isDone(repo.Issues[id]) == done {
			kept = append(kept, id)
		}
	}
	return kept
}

// filterSearch keeps the issues whose id, current title or any action subject
// contains every term, case-insensitively. The query is an AND of literal
// fragments, not a regular expression: that is the convention for issue/record
// search (gh, git-bug), while regex stays opt-in elsewhere.
func (e *env) filterSearch(repo *model.Repo, ids []string, terms []string) []string {
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if issueMatches(repo.Issues[id], terms) {
			kept = append(kept, id)
		}
	}
	return kept
}

// issueMatches joins the searchable text of one issue — id, current title and
// every action subject, the same words the views show — and asks whether every
// term occurs somewhere in it. A separator keeps matching from crossing fields.
func issueMatches(p *model.Issue, terms []string) bool {
	fields := make([]string, 0, len(p.Actions)+2)
	fields = append(fields, p.ID, issueTitle(p))
	for _, a := range p.Actions {
		fields = append(fields, a.Subject)
	}
	return containsAll(strings.Join(fields, "\x00"), terms)
}

// searchHit names the action that justifies a hit the row does not already
// show. When id + title contain every term the row explains itself and hit is
// nil; otherwise the earliest action subject holding all terms is returned. If
// the terms are spread over several actions, no single action explains the
// query and hit is nil too — better silent than pointing at the wrong line.
func searchHit(p *model.Issue, terms []string) *listHit {
	if len(terms) == 0 || containsAll(p.ID+"\x00"+issueTitle(p), terms) {
		return nil
	}
	for i, a := range p.Actions {
		if containsAll(a.Subject, terms) {
			return &listHit{
				Ordinal: i + 1,
				Action:  a.Kind,
				Commit:  a.ID,
				Subject: a.Subject,
				Total:   len(p.Actions),
				Date:    a.Date.Format(time.RFC3339),
			}
		}
	}
	return nil
}

// searchRank buckets a match the way the caller orders search results: 0 is an
// id that starts with the first term (the other terms may live in the title or
// an action), 1 an id holding every term, 2 the current title, 3 one action
// subject, and 4 a match spread over several fields. It assumes the issue
// already passed filterSearch; rank 4 is a fallback, not a second filter.
func searchRank(p *model.Issue, terms []string) int {
	if strings.HasPrefix(strings.ToLower(p.ID), strings.ToLower(terms[0])) {
		return 0
	}
	if containsAll(p.ID, terms) {
		return 1
	}
	if containsAll(issueTitle(p), terms) {
		return 2
	}
	for _, a := range p.Actions {
		if containsAll(a.Subject, terms) {
			return 3
		}
	}
	return 4
}

// searchRow is one issue in a search result: the issue, its rank, and the hit
// that explains the match. Both the JSON and the text view walk one slice of
// these, so the two orders cannot drift apart.
type searchRow struct {
	p    *model.Issue
	rank int
	hit  *listHit
}

// lastDate is the issue's most recent action date, the search tie-break key.
func lastDate(p *model.Issue) time.Time {
	if a, ok := p.Last(); ok {
		return a.Date
	}
	return time.Time{}
}

// lessSearchRow orders search results by rank, then newest last action first,
// then id. Dates compare as times, not strings; a missing date sorts after every
// known one so the order stays total.
func lessSearchRow(a, b searchRow) bool {
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	ad, bd := lastDate(a.p), lastDate(b.p)
	switch {
	case ad.IsZero() != bd.IsZero():
		return bd.IsZero()
	case !ad.Equal(bd):
		return ad.After(bd)
	default:
		return a.p.ID < b.p.ID
	}
}

// containsAll reports whether every term occurs in haystack, case-insensitively.
func containsAll(haystack string, terms []string) bool {
	h := strings.ToLower(haystack)
	for _, term := range terms {
		if !strings.Contains(h, strings.ToLower(term)) {
			return false
		}
	}
	return true
}

func (e *env) list(args []string) int {
	pos, f, missing := parse(args, "json")
	allowed := allowedFlags("json", "domain", "tag", "action", "status", "search")
	if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
		return code
	}
	warnUnknownFlags(e.errw, f, allowed)
	if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
		return code
	}
	if code := rejectRepeats(e.errw, f, "domain", "tag", "action", "status", "search"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln list [--domain=] [--tag=] [--action=] [--status=open|done] [--search=] [--json]"); code != 0 {
		return code
	}
	status := f.get("status")
	if f.has("status") && status != "open" && status != "done" {
		fmt.Fprint(e.errw, i18n.T("--status 只接受 open 或 done（当前: %s）\n", status))
		return 2
	}
	// The action vocabulary is the same closed set `do` validates against, so a
	// typo here is a refusal, not an empty result.
	action := strings.ToLower(strings.TrimSpace(f.get("action")))
	if action != "" && !e.actions.IsValid(action) {
		fmt.Fprint(e.errw, i18n.T("非法 action: %q（可选: %s）\n", action, strings.Join(e.actions.Names(), ", ")))
		return 2
	}
	// A tag is one literal value here; `do` splits commas into several tags, and
	// silently treating "a,b" as one tag returns zero rows that look like a miss.
	tag := f.get("tag")
	if strings.Contains(tag, ",") {
		fmt.Fprint(e.errw, i18n.T("--tag 只接受一个标签（当前: %q）；多个标签暂不支持\n", tag))
		return 2
	}
	query := f.get("search")
	terms := strings.Fields(query)
	if f.has("search") && len(terms) == 0 {
		fmt.Fprintln(e.errw, i18n.T("--search 需要一个非空查询串"))
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	opts := engine.FilterOptions{
		Domain: f.get("domain"),
		Tag:    tag,
		Action: action,
	}
	ids := engine.Filter(repo, opts)
	if status != "" {
		ids = e.filterStatus(repo, ids, status == "done")
	}
	if len(terms) > 0 {
		ids = e.filterSearch(repo, ids, terms)
	}
	// An empty result is a valid answer (exit 0), but silence reads like a broken
	// command. The hint goes to stderr in both views, so a JSON consumer still
	// gets exactly one document on stdout.
	if len(ids) == 0 {
		switch {
		case len(terms) > 0:
			fmt.Fprint(e.errw, i18n.T("没有匹配 %q 的问题\n", query))
		case len(repo.Issues) == 0:
			fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
		default:
			fmt.Fprint(e.errw, i18n.T("没有匹配 %s 的问题\n", filterSummary(opts, status)))
		}
	}
	filter := filterQuery{Domain: opts.Domain, Tag: opts.Tag, Action: opts.Action, Status: status, Search: query}
	// Search is the only view that reorders: rank first, then the newest last
	// action, then id. Rows are built once so the text and JSON views below walk
	// the same order and share the same hit.
	if len(terms) > 0 {
		rows := make([]searchRow, 0, len(ids))
		for _, id := range ids {
			p := repo.Issues[id]
			rows = append(rows, searchRow{p: p, rank: searchRank(p, terms), hit: searchHit(p, terms)})
		}
		sort.Slice(rows, func(i, j int) bool { return lessSearchRow(rows[i], rows[j]) })
		if f.has("json") {
			doc := listDoc{Filter: filter, Count: len(rows), Issues: make([]listIssue, 0, len(rows))}
			for _, r := range rows {
				rank := r.rank
				doc.Issues = append(doc.Issues, listIssue{
					issueCore: issueCore{ID: r.p.ID, Domain: r.p.Domain, Action: r.p.State(), Done: e.isDone(r.p)},
					Rank:      &rank,
					Title:     issueTitle(r.p),
					Commit:    issueCommit(r.p),
					Hit:       r.hit,
					Tags:      r.p.Tags,
				})
			}
			return e.writeJSON(doc)
		}
		for _, r := range rows {
			line := fmt.Sprintf("%s %s %s", pad(r.p.ID, 30), pad(stateWord(r.p), 12), issueTitle(r.p))
			if r.hit != nil {
				line += "  ← " + r.hit.Subject
			}
			fmt.Fprintln(e.out, line)
		}
		return 0
	}
	if f.has("json") {
		doc := listDoc{
			Filter: filter,
			Count:  len(ids),
			Issues: make([]listIssue, 0, len(ids)),
		}
		for _, id := range ids {
			p := repo.Issues[id]
			doc.Issues = append(doc.Issues, listIssue{
				issueCore: issueCore{ID: p.ID, Domain: p.Domain, Action: p.State(), Done: e.isDone(p)},
				Title:     issueTitle(p),
				Commit:    issueCommit(p),
				Tags:      p.Tags,
			})
		}
		return e.writeJSON(doc)
	}
	for _, id := range ids {
		p := repo.Issues[id]
		fmt.Fprintf(e.out, "%s %s %s\n", pad(p.ID, 30), pad(stateWord(p), 12), tagsSuffix(p))
	}
	return 0
}

// filterSummary names the conditions actually in force, in the help's order, so
// an empty result says what matched nothing instead of nothing at all.
func filterSummary(o engine.FilterOptions, status string) string {
	var parts []string
	if o.Domain != "" {
		parts = append(parts, "domain="+o.Domain)
	}
	if o.Tag != "" {
		parts = append(parts, "tag="+o.Tag)
	}
	if o.Action != "" {
		parts = append(parts, "action="+o.Action)
	}
	if status != "" {
		parts = append(parts, "status="+status)
	}
	return strings.Join(parts, " ")
}

// listDoc is the `kiln list --json` document.
type listDoc struct {
	Filter filterQuery `json:"filter"`
	Count  int         `json:"count"`
	Issues []listIssue `json:"issues"`
}

// filterQuery echoes the conditions that were asked for; an absent key means
// that condition was not applied.
type filterQuery struct {
	Domain string `json:"domain,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Action string `json:"action,omitempty"`
	Status string `json:"status,omitempty"`
	Search string `json:"search,omitempty"`
}

type listIssue struct {
	issueCore
	Rank   *int     `json:"rank,omitempty"`
	Title  string   `json:"title,omitempty"`
	Commit string   `json:"commit,omitempty"`
	Hit    *listHit `json:"hit,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// listHit is the action a search matched when the row's id and title cannot
// explain it: the ordinal addresses it in `kiln show`, the commit addresses it
// in git. It is a pointer so an unexplained search omits the key entirely.
type listHit struct {
	Ordinal int    `json:"ordinal"`
	Action  string `json:"action"`
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	Total   int    `json:"total"`
	Date    string `json:"date"`
}
