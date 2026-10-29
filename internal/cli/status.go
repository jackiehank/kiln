package cli

import (
	"fmt"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

func (e *env) status(args []string) int {
	pos, f, _ := parse(args, "json")
	warnUnknownFlags(e.errw, f, allowedFlags("json"))
	if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln status [--json]"); code != 0 {
		return code
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	areas := e.knownAreas()
	s := engine.Summarize(repo, e.actions)
	if f.has("json") {
		return e.statusJSON(repo, s, areas)
	}
	if s.Total == 0 {
		fmt.Fprint(e.out, i18n.T("问题总数: 0（完成 0 / 进行中 0）｜%s\n", worktreeSegment(e.worktree, areasOpen(areas))))
		fmt.Fprint(e.out, i18n.T("kiln 动作数: %d\n", s.Actions))
		fmt.Fprintln(e.out, i18n.T("提示: 用 `kiln do <issue> <action> --title <文本>` 生成规范提交信息"))
		e.renderWorktreeLegend(areas)
		return 0
	}
	groups := engine.GroupByDomain(repo)
	fmt.Fprint(e.out, i18n.T("问题总数: %d（完成 %d / 进行中 %d）｜domain %d 个｜%s\n", s.Total, s.Complete, s.InProgress, engine.CountDomains(groups), worktreeSegment(e.worktree, areasOpen(areas))))
	fmt.Fprint(e.out, i18n.T("kiln 动作数: %d\n\n", s.Actions))
	e.renderStatus(e.statusRows(groups, areaIssues(areas)))
	e.renderWorktreeLegend(areas)
	return 0
}

// statusRow is one issue in the domain view, in exactly the order the view
// prints it. Both renderings — the text rows and the JSON array — come out of
// this one walk, so their issue rows can never disagree about order, which is
// what lets a test compare them row for row.
type statusRow struct {
	issue *model.Issue
	head  string // the block header to print above this row; "" inside a block
	area  bool   // this issue has a worktree open
}

// statusRows lays the domain blocks out as a flat, ordered list of rows: one row
// per issue, the first row of each block carrying that block's header. The header
// is text only — the JSON rows carry their own `domain` — but it comes out of this
// same walk, so neither view can disagree about where a block starts.
func (e *env) statusRows(groups []engine.DomainGroup, areas map[string]bool) []statusRow {
	var rows []statusRow
	for _, g := range groups {
		for i, p := range g.Issues {
			head := ""
			if i == 0 {
				head = g.Domain
			}
			rows = append(rows, statusRow{issue: p, head: head, area: areas[p.ID]})
		}
	}
	return rows
}

// renderStatus prints the rows aligned by display width, with a blank line
// renderStatus prints the domain blocks: a header line naming the block, then one
// row per issue, the rows aligned by display width across every block. Headers
// stay out of that width calculation — one long domain name must not push the
// whole table right — and a header opens its block, so it takes the place of the
// blank line that used to sit between blocks.
func (e *env) renderStatus(rows []statusRow) {
	labels := make([]string, len(rows))
	width := 0
	for i, r := range rows {
		labels[i] = "  " + r.issue.ID
		if w := displayWidth(labels[i]); w > width {
			width = w
		}
	}
	for i, r := range rows {
		if r.head != "" {
			if i > 0 {
				fmt.Fprintln(e.out)
			}
			fmt.Fprintln(e.out, i18n.T(r.head))
		}
		mark := " "
		if e.isDone(r.issue) {
			mark = "✓"
		}
		fmt.Fprintf(e.out, "%s %s %s%s\n", pad(labels[i], width), mark, stateWord(r.issue), areaMark(r.area))
	}
}

// statusDoc is the `kiln status --json` document: the three things the text view
// states — repository counters, the isolation state, and the issues.
type statusDoc struct {
	Summary  statusSummary `json:"summary"`
	Worktree worktreeState `json:"worktree"`
	Issues   []statusIssue `json:"issues"`
}

type statusSummary struct {
	Total   int `json:"total"`
	Done    int `json:"done"`
	Open    int `json:"open"`
	Actions int `json:"actions"`
	Domains int `json:"domains"`
}

type worktreeState struct {
	Policy  string `json:"policy"`
	Open    int    `json:"open"`
	Current string `json:"current,omitempty"`
}

// statusIssue is one flat row of the domain view. It carries no action list —
// that is `path`'s job — which keeps the document O(issues), not O(commits).
type statusIssue struct {
	issueCore
	Title       string `json:"title,omitempty"`
	HasWorktree bool   `json:"has-worktree"`
}

// statusJSON renders the domain view as a flat issue list, in the very order the
// text view prints its rows — both renderings come from one walk, so their issue
// rows line up one for one. The text view's block headers have no counterpart
// here: every row already carries its `domain`, and a heading would be a second
// way to say the same thing.
func (e *env) statusJSON(repo *model.Repo, s engine.Stats, areas []vcs.WorkspaceInfo) int {
	groups := engine.GroupByDomain(repo)
	doc := statusDoc{
		Summary: statusSummary{
			Total:   s.Total,
			Done:    s.Complete,
			Open:    s.InProgress,
			Actions: s.Actions,
			Domains: engine.CountDomains(groups),
		},
		Worktree: e.worktreeState(areas),
		Issues:   make([]statusIssue, 0, s.Total),
	}
	for _, r := range e.statusRows(groups, areaIssues(areas)) {
		title := ""
		if last, ok := r.issue.Last(); ok {
			title = last.Title
		}
		doc.Issues = append(doc.Issues, statusIssue{
			issueCore: issueCore{
				ID:     r.issue.ID,
				Domain: r.issue.Domain,
				Action: r.issue.State(),
				Done:   e.isDone(r.issue),
			},
			Title:       title,
			HasWorktree: r.area,
		})
	}
	return e.writeJSON(doc)
}

// worktreeState states the repository-level isolation mode and how many areas are
// open, plus the area the caller stands in when there is one.
func (e *env) worktreeState(areas []vcs.WorkspaceInfo) worktreeState {
	ws := worktreeState{Policy: worktreeWord(e.worktree), Open: areasOpen(areas)}
	if here, ok := vcs.AreaAt(areas, "."); ok && !here.Main {
		ws.Current = here.Issue
	}
	return ws
}
