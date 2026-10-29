package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

// show implements `kiln show <issue> [n] [--stat]`: it expands a single action.
// The action is addressed by its 1-based ordinal in `path` (default: the last
// one), so the selector stays put even when commit hashes are rewritten by a
// rebase. Rendering is delegated to git.
func (e *env) show(args []string) int {
	pos, f, _ := parse(args, "stat", "json")
	warnUnknownFlags(e.errw, f, allowedFlags("stat", "json"))
	if code := rejectSwitchValues(e.errw, f, "stat", "json"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 2, "kiln show <issue> [n|action] [--stat] [--json]"); code != 0 {
		return code
	}
	if len(pos) < 1 {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln show <issue> [n] [--stat] [--json]"))
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	p, ok := repo.Issues[pos[0]]
	if !ok {
		fmt.Fprint(e.errw, i18n.T("未找到问题: %s\n", pos[0]))
		return 1
	}
	if len(p.Actions) == 0 {
		fmt.Fprint(e.errw, i18n.T("问题没有动作: %s\n", pos[0]))
		return 1
	}
	ordinal := len(p.Actions)
	if len(pos) >= 2 {
		n, err := strconv.Atoi(pos[1])
		switch {
		case err == nil && n >= 1 && n <= len(p.Actions):
			ordinal = n
		case err == nil:
			fmt.Fprint(e.errw, i18n.T("动作序号超出范围: %q（可选 1..%d）\n", pos[1], len(p.Actions)))
			return 2
		case e.actions.IsValid(pos[1]):
			// An action name selects the **last** one of that kind — the same "newest
			// match" rule git's :/<text> uses — because that is what "show me that
			// test" means in practice. Repeats are the norm, not a corner case, so
			// ambiguity is announced with the addresses of every match, plus one
			// command the caller can paste, rather than "use an ordinal" (which only
			// sends them back to count lines in `path`).
			ords := matchingOrdinals(p, pos[1])
			if len(ords) == 0 {
				fmt.Fprint(e.errw, i18n.T("没有 %q 动作: %s（它的动作: %s）\n", pos[1], p.ID, ordinalList(p)))
				return 1
			}
			ordinal = ords[len(ords)-1]
			if len(ords) > 1 {
				fmt.Fprint(e.errw, i18n.T("提示: %s 有 %d 个 %s（序号 %s）；已显示 %d/%d（最后一个）\n", p.ID, len(ords), pos[1], ordinalNumbers(ords), ordinal, len(p.Actions)))
				fmt.Fprint(e.errw, i18n.T("  看别的：kiln show %s %d\n", p.ID, ords[0]))
			}
		default:
			fmt.Fprint(e.errw, i18n.T("无法识别的选择器: %q（用序号 1..%d，或动作名: %s）\n", pos[1], len(p.Actions), strings.Join(e.actions.Names(), ", ")))
			return 2
		}
	}
	a := p.Actions[ordinal-1]
	if f.has("json") {
		return e.showJSON(p, ordinal, a, f.has("stat"))
	}
	fmt.Fprintf(e.out, "%s · %d/%d %s · %s · %s\n\n",
		p.ID, ordinal, len(p.Actions), a.Kind, shortID(a.ID), a.Subject)

	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	var body string
	if f.has("stat") {
		body, err = backend.ShowStat(a.ID, false)
	} else {
		body, err = backend.Show(a.ID)
	}
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
		return 1
	}
	fmt.Fprint(e.out, trimNewlines(body))
	fmt.Fprintln(e.out)
	return 0
}

// showDoc is `kiln show --json`: one action's metadata plus its body. body is
// always the backend's own rendering of the commit (which includes the diff —
// the same text the text view prints), so the key never changes meaning; --stat
// only adds the diffstat beside it instead of replacing the body.
type showDoc struct {
	Issue   string `json:"issue"`
	Ordinal int    `json:"ordinal"`
	Total   int    `json:"total"`
	Action  string `json:"action"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Title   string `json:"title,omitempty"`
	Body    string `json:"body,omitempty"`
	Stat    string `json:"stat,omitempty"`
}

// showJSON renders one action as one document: metadata plus body, where body
// is git's text (or the diffstat under --stat) with surrounding blank lines
// trimmed, exactly the text the text view would print.
func (e *env) showJSON(p *model.Issue, ordinal int, a model.Action, stat bool) int {
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	body, err := backend.Show(a.ID)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
		return 1
	}
	statText := ""
	if stat {
		statText, err = backend.ShowStat(a.ID, false)
		if err != nil {
			fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
			return 1
		}
	}
	return e.writeJSON(showDoc{
		Issue:   p.ID,
		Ordinal: ordinal,
		Total:   len(p.Actions),
		Action:  a.Kind,
		Commit:  a.ID,
		Date:    a.Date.Format(time.RFC3339),
		Title:   a.Title,
		Body:    trimNewlines(body),
		Stat:    trimNewlines(statText),
	})
}
