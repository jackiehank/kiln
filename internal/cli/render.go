package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

func modeName(commit bool) string {
	if commit {
		return "commit"
	}
	return "do"
}

func stateWord(p *model.Issue) string {
	if s := p.State(); s != "" {
		return s
	}
	return i18n.T("未知")
}

func (e *env) progressWord(p *model.Issue) string {
	if last, ok := p.Last(); ok && e.actions.IsTerminal(last.Kind) {
		return i18n.T("已完成")
	}
	return i18n.T("进行中")
}

func tagsSuffix(p *model.Issue) string {
	var parts []string
	if p.Domain != "" {
		parts = append(parts, "domain="+p.Domain)
	}
	if len(p.Tags) > 0 {
		parts = append(parts, "tags="+strings.Join(p.Tags, ","))
	}
	return strings.Join(parts, " ")
}

// issueTitle is the one line that names an issue in a listing: the last action's
// title, falling back to that action's subject when the commit carried no title
// trailer (a handful of early actions did not). It never invents a second
// notion of "the issue's name" — status and graph read the same last action.
func issueTitle(p *model.Issue) string {
	last, ok := p.Last()
	if !ok {
		return ""
	}
	if last.Title != "" {
		return last.Title
	}
	return last.Subject
}

// issueCommit is the commit the row's action/done/title describe: the last
// action's full hash. Rows are only built for issues that have actions, so it is
// empty only in theory; the key is omitted rather than guessed.
func issueCommit(p *model.Issue) string {
	last, ok := p.Last()
	if !ok {
		return ""
	}
	return last.ID
}

func shortID(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// writeJSON emits one document on stdout, which is the whole machine-readable
// contract: stdout carries the document and nothing else, every failure stays on
// stderr, and the exit code says what the text view says.
func (e *env) writeJSON(v any) int {
	enc := json.NewEncoder(e.out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprint(e.errw, i18n.T("输出 JSON 失败: %v\n", err))
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Machine-readable output (`--json`)
//
// The text views are the default; `--json` is a side road for agents and
// scripts, whose schema lives in docs/machine-readable-output.md. Four rules
// hold for all of it. (1) The document is always one object, never a bare
// array, so a field can be added later without changing the type a caller
// decodes. (2) Keys reuse the words kiln already speaks — trailer names where
// the thing is the trailer's, view words (`parent`, `has-worktree`) where the
// concept belongs to the view. (3) Nothing addressed to a human gets in: no
// hints, no legends, no prose — a caller that wants those reads the text view.
// (4) An absent key means "none", never `null`; booleans are the exception, since
// `false` is a finding rather than an absence and is always written.
// ---------------------------------------------------------------------------

// issueCore is the row `status` and `list` share, so one decoder covers the same
// four fields in both and each command only adds what belongs to it.
type issueCore struct {
	ID     string `json:"id"`
	Domain string `json:"domain,omitempty"`
	Action string `json:"action"`
	Done   bool   `json:"done"`
}
