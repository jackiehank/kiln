package cli

import (
	"fmt"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

func (e *env) check(args []string) int {
	pos, f, _ := parse(args, "json", "verbose")
	warnUnknownFlags(e.errw, f, allowedFlags("json", "verbose"))
	if code := rejectSwitchValues(e.errw, f, "json", "verbose"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln check [--verbose] [--json]"); code != 0 {
		return code
	}
	verbose := len(f["verbose"]) > 0
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	if f.has("json") {
		return e.checkJSON(repo)
	}
	if len(repo.Issues) == 0 {
		fmt.Fprintln(e.out, i18n.T("没有 kiln 动作（检查提交信息是否同时带了 issue: 与 action:）"))
		return 0
	}
	failed := false
	for _, d := range engine.Check(repo, e.actions) {
		fmt.Fprintf(e.out, "%s %s %s\n", d.Symbol(), pad(d.Issue, 30), d.Message)
		if d.Level == engine.LevelErr {
			failed = true
		}
		if verbose {
			if p, ok := repo.Issues[d.Issue]; ok {
				for _, a := range p.Actions {
					fmt.Fprintf(e.out, "      %s %s\n", pad(a.Kind, 12), shortID(a.ID))
				}
			}
		}
	}
	if failed {
		return 1
	}
	return 0
}

// checkDoc is the `kiln check --json` document. passed mirrors the exit code —
// false exactly when an error was found — so a caller that ignores exit codes
// still gets the answer CI cares about. findings counts diagnostics (one issue
// can yield several), which is why it is not called "summary": the level names
// ok/warn/error inside it count findings, not issues.
type checkDoc struct {
	Passed   bool          `json:"passed"`
	Findings checkSummary  `json:"findings"`
	Results  []checkResult `json:"results"`
}

// checkSummary counts findings, not issues: one issue can fail twice.
type checkSummary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Err  int `json:"error"`
}

// checkResult is one finding, named by rule code rather than described in
// prose. The sentence stays where it is read by a human.
type checkResult struct {
	Issue    string   `json:"issue"`
	Level    string   `json:"level"`
	Rule     string   `json:"rule"`
	Action   string   `json:"action,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Relation string   `json:"relation,omitempty"`
	Actions  []string `json:"actions"`
}

// checkJSON renders every finding with its rule code. The verbose flag changes
// nothing here: the action list is always reported, so the document keeps one
// shape and a caller needs one decoder whatever flags were passed.
//
// The document is written first and the exit code decided after: leaving with a
// non-zero code must not cost the caller the report they ran the command for.
func (e *env) checkJSON(repo *model.Repo) int {
	diags := engine.Check(repo, e.actions)
	doc := checkDoc{Passed: true, Results: make([]checkResult, 0, len(diags))}
	for _, d := range diags {
		switch d.Level {
		case engine.LevelWarn:
			doc.Findings.Warn++
		case engine.LevelErr:
			doc.Findings.Err++
			doc.Passed = false
		default:
			doc.Findings.OK++
		}
		doc.Results = append(doc.Results, checkResult{
			Issue:    d.Issue,
			Level:    levelWord(d.Level),
			Rule:     d.Rule,
			Action:   d.Action,
			Ref:      d.Ref,
			Relation: d.Relation,
			Actions:  kindsOf(repo.Issues[d.Issue]),
		})
	}
	if code := e.writeJSON(doc); code != 0 {
		return code
	}
	if !doc.Passed {
		return 1
	}
	return 0
}

// levelWord renders a diagnostic level as the word the machine view reports. The
// internal ordering (LevelOK < LevelWarn < LevelErr) is an implementation
// detail and must not leak into the contract.
func levelWord(l engine.Level) string {
	switch l {
	case engine.LevelWarn:
		return "warn"
	case engine.LevelErr:
		return "error"
	default:
		return "ok"
	}
}

// kindsOf lists an issue's action kinds in path order.
func kindsOf(p *model.Issue) []string {
	out := make([]string, 0, len(p.Actions))
	for _, a := range p.Actions {
		out = append(out, a.Kind)
	}
	return out
}
