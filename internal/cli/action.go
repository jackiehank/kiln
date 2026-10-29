package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/trailer"
	"github.com/jackiehank/kiln/internal/vcs"
)

// action implements the read-only `action` group. Only `list` exists today;
// `log` will join it later.
func (e *env) action(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			if text, ok := commandUsageText(e.rel(), "action"); ok {
				fmt.Fprint(e.out, text)
				return 0
			}
		}
	}
	// Parse first so flags may precede the subcommand (`action --json list`),
	// the same shape `worktree` uses.
	pos, f, missing := parse(args, "json", "jsonl")
	sub := ""
	rest := pos
	if len(pos) > 0 {
		sub, rest = pos[0], pos[1:]
	}
	switch sub {
	case "list":
		if code := rejectExtra(e.errw, rest, 0, "kiln action list [--json]"); code != 0 {
			return code
		}
		// The two subcommands share one flag namespace after parsing, so each
		// refuses the other's flags instead of dropping them on the floor.
		if f.has("jsonl") {
			fmt.Fprintln(e.errw, i18n.T("action list 用 --json，不用 --jsonl"))
			return 2
		}
		if f.has("limit") {
			fmt.Fprintln(e.errw, i18n.T("--limit 只服务 action log"))
			return 2
		}
		if code := rejectSwitchValues(e.errw, f, "json"); code != 0 {
			return code
		}
		warnUnknownFlags(e.errw, f, allowedFlags("json"))
		return e.actionList(f)
	case "log":
		if code := rejectExtra(e.errw, rest, 0, "kiln action log [--jsonl] [--limit N]"); code != 0 {
			return code
		}
		allowed := allowedFlags("jsonl", "limit")
		if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
			return code
		}
		if code := rejectSwitchValues(e.errw, f, "jsonl"); code != 0 {
			return code
		}
		if code := rejectRepeats(e.errw, f, "limit"); code != 0 {
			return code
		}
		if f.has("json") {
			fmt.Fprintln(e.errw, i18n.T("action log 用 --jsonl（一行一个动作），不用 --json"))
			return 2
		}
		warnUnknownFlags(e.errw, f, allowed)
		return e.actionLog(f)
	default:
		fmt.Fprintln(e.errw, i18n.T("用法: kiln action list [--json] | kiln action log [--jsonl] [--limit N]"))
		return 2
	}
}

// actionList prints the effective vocabulary, not the history: the text view is
// one action per line with optional start/terminal markers, and --json mirrors
// the config's actions block plus where it came from.
func (e *env) actionList(f flags) int {
	if f.has("json") {
		source := e.configSource
		if source == "" {
			source = config.SourceDefault
		}
		doc := actionListDoc{
			Actions: actionSetDoc{
				Names:    e.actions.Names(),
				Start:    e.actions.Start(),
				Terminal: terminalNames(e.actions),
			},
			Source: source,
		}
		if source == config.SourceFile && e.configPath != "" {
			doc.Config = e.configPath
		}
		return e.writeJSON(doc)
	}
	names := e.actions.Names()
	width := 0
	for _, name := range names {
		if w := displayWidth(name); w > width {
			width = w
		}
	}
	for _, name := range names {
		line := pad(name, width)
		var markers []string
		if e.actions.IsStart(name) {
			markers = append(markers, "start")
		}
		if e.actions.IsTerminal(name) {
			markers = append(markers, "terminal")
		}
		if len(markers) > 0 {
			line += "  " + strings.Join(markers, ", ")
		}
		fmt.Fprintln(e.out, strings.TrimRight(line, " "))
	}
	return 0
}

// actionLog renders the global action stream: one row per commit that carries
// both `issue:` and `action:`, newest author date first. It never builds the
// issue model — one pass, one row at a time — so the first line arrives with the
// first commit instead of after the whole history, and memory stays flat however
// large the repository is.
//
// --limit and a consumer that went away both stop the scan through the sink's
// false return; the backend kills its child at that point.
func (e *env) actionLog(f flags) int {
	limit := 0
	if f.has("limit") {
		n, err := strconv.Atoi(strings.TrimSpace(f.get("limit")))
		if err != nil || n < 1 {
			fmt.Fprint(e.errw, i18n.T("--limit 需要 ≥ 1 的整数（当前: %q）\n", f.get("limit")))
			return 2
		}
		limit = n
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	stream, ok := backend.(vcs.LogEach)
	if !ok {
		fmt.Fprintln(e.errw, i18n.T("当前后端不支持 action log 的流式读取"))
		return 1
	}
	jsonl := f.has("jsonl")
	var scratch bytes.Buffer
	enc := json.NewEncoder(&scratch)
	enc.SetEscapeHTML(false)
	var writeErr error
	emitted := 0
	// emit writes one row and reports whether the commit was an action at all.
	emit := func(c vcs.Commit) bool {
		fields, ok := trailer.Parse(c.Message, e.rel())
		if !ok {
			return false
		}
		if err := e.writeActionLogRow(c.ID, c.Date, fields, engine.Subject(c.Message), jsonl, &scratch, enc); err != nil {
			writeErr = err
		}
		return true
	}
	// No --limit means the whole history is going to be printed: on a warm cache
	// that is an enumeration plus lookups, without printing 35 MB of messages.
	if limit == 0 {
		if n, handled, cerr := e.actionLogCached(backend, jsonl, &scratch, enc); handled {
			if cerr != nil {
				if isBrokenPipe(cerr) {
					return 0
				}
				fmt.Fprint(e.errw, i18n.T("输出失败: %v\n", cerr))
				return 1
			}
			if n == 0 {
				fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
			}
			return 0
		}
	}
	err = stream.LogEach(func(c vcs.Commit) bool {
		kept := emit(c)
		if writeErr != nil {
			return false
		}
		if kept {
			emitted++
		}
		return limit == 0 || emitted < limit
	})
	if writeErr != nil {
		if isBrokenPipe(writeErr) {
			return 0 // the consumer closed the pipe: not a kiln failure
		}
		fmt.Fprint(e.errw, i18n.T("输出失败: %v\n", writeErr))
		return 1
	}
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取历史失败: %v\n", err))
		return 1
	}
	if emitted == 0 {
		fmt.Fprint(e.errw, i18n.T("仓库里还没有任何动作（用 kiln do <issue> ask --title <文本> 开一条）\n"))
	}
	return 0
}

// writeActionLogRow renders one action's row. Both read paths — the stream and the
// cache — call it, so their bytes cannot drift apart.
func (e *env) writeActionLogRow(id string, when time.Time, f trailer.Fields, subject string, jsonl bool, scratch *bytes.Buffer, enc *json.Encoder) error {
	label := f.Title
	if label == "" {
		label = subject
	}
	if jsonl {
		scratch.Reset()
		if err := enc.Encode(actionLogDoc{
			Issue:     f.Issue,
			Action:    f.Action,
			Commit:    id,
			Date:      when.Format(time.RFC3339),
			Title:     label,
			Domain:    f.Domain,
			Tags:      f.Tags,
			Refs:      dedupeStrings(f.Refs),
			Target:    dedupeStrings(trailer.SplitList(f.Target)),
			Scope:     f.Scope,
			Relations: actionLogRelations(e.rel(), f.Relations),
			Unrelate:  dedupeStrings(f.Unrelate),
		}); err != nil {
			return err
		}
		_, err := e.out.Write(scratch.Bytes())
		return err
	}
	line := fmt.Sprintf("%s  %s  %s  %s  %s",
		when.Local().Format("2006-01-02 15:04"),
		pad(f.Issue, 24),
		pad(f.Action, 12),
		shortID(id),
		label)
	_, err := fmt.Fprintln(e.out, strings.TrimRight(line, " "))
	return err
}

// actionLogRelations lays an action's declarations out in a fixed order: the
// vocabulary's own order first (so the same repository always prints the same
// bytes), then each name's targets in the order the action wrote them, with
// exact repeats dropped. Parse has already canonicalized aliases, and names
// outside the vocabulary never reach Fields, but the sorted fallback keeps the
// output total (and therefore deterministic) if that ever changes.
func actionLogRelations(rel *model.RelationSet, decls []model.Rel) []actionLogRelation {
	if len(decls) == 0 {
		return nil
	}
	byName := make(map[string][]string, len(decls))
	for _, d := range decls {
		byName[d.Name] = append(byName[d.Name], d.Target)
	}
	for name, targets := range byName {
		byName[name] = dedupeStrings(targets)
	}
	out := make([]actionLogRelation, 0, len(decls))
	emit := func(name string) {
		for _, t := range byName[name] {
			out = append(out, actionLogRelation{Name: name, Target: t})
		}
		delete(byName, name)
	}
	for _, name := range rel.Names() {
		emit(name)
	}
	extra := make([]string, 0, len(byName))
	for name := range byName {
		extra = append(extra, name)
	}
	sort.Strings(extra)
	for _, name := range extra {
		emit(name)
	}
	return out
}

// terminalNames lists the terminal actions in declaration order. ActionSet has
// no direct accessor, and deriving keeps the order the config declares.
func terminalNames(set *model.ActionSet) []string {
	names := set.Names()
	terminal := make([]string, 0, len(names))
	for _, name := range names {
		if set.IsTerminal(name) {
			terminal = append(terminal, name)
		}
	}
	return terminal
}

// actionListDoc is `kiln action list --json`: the effective vocabulary, mirroring
// the config's actions block, plus where it came from. Config is omitted when
// the defaults are in force; Source is always present.
type actionListDoc struct {
	Actions actionSetDoc `json:"actions"`
	Source  string       `json:"source"`
	Config  string       `json:"config,omitempty"`
}

type actionSetDoc struct {
	Names    []string `json:"names"`
	Start    string   `json:"start"`
	Terminal []string `json:"terminal"`
}

// actionLogDoc is one NDJSON record of `kiln action log --jsonl`. There is no
// envelope because every line is the same kind of record; the four identity keys
// are always written, and the descriptive ones are omitted when absent — an
// absent key means "none", never an empty string. date keeps the commit's own
// offset so a consumer never has to guess the timezone.
//
// Relations are the action's own declarations, not the folded graph: `has-part:
// X` stays {name: has-part, target: X} instead of being flipped into an edge.
// Folding is `graph` / `path`'s job, and doing it here would need the vocabulary
// at emit time and would silently drop the "this action said so" half. Names are
// canonical (aliases resolved at parse time) and unknown keys never reach this
// field — they are `check`'s unknown-trailer finding.
type actionLogDoc struct {
	Issue     string              `json:"issue"`
	Action    string              `json:"action"`
	Commit    string              `json:"commit"`
	Date      string              `json:"date"`
	Title     string              `json:"title,omitempty"`
	Domain    string              `json:"domain,omitempty"`
	Tags      []string            `json:"tags,omitempty"`
	Refs      []string            `json:"refs,omitempty"`
	Target    []string            `json:"target,omitempty"`
	Scope     string              `json:"scope,omitempty"`
	Relations []actionLogRelation `json:"relations,omitempty"`
	Unrelate  []string            `json:"unrelate,omitempty"`
}

// actionLogRelation is one declared relation: the canonical name and its target
// issue, exactly as this action wrote them.
type actionLogRelation struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}
