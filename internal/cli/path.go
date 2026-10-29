package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

func (e *env) path(args []string) int {
	pos, f, _ := parse(args, "all", "json", "stat", "shortstat")
	warnUnknownFlags(e.errw, f, allowedFlags("stat", "shortstat", "json", "all"))
	// A switch never takes a value; saying --json=x is a mistake worth naming
	// instead of dropping x (or swallowing the positional that follows).
	if code := rejectSwitchValues(e.errw, f, "all", "json", "stat", "shortstat"); code != 0 {
		return code
	}
	// Batch is decided by what the caller asked for, before repeats are folded
	// away: `path a a --json` is still a batch request, it just names one issue.
	batch := len(pos) > 1
	pos = dedupeStrings(pos)
	all := f.has("all")
	if len(pos) == 0 && !all {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln path <issue> [<issue>…] | --all [--stat|--shortstat|--json]"))
		return 2
	}
	if all && len(pos) > 0 {
		fmt.Fprintln(e.errw, i18n.T("--all 与问题参数不能同用"))
		return 2
	}
	// Batch is JSON-only: the text view is a reading layout (one path, stats
	// nested under each action), while the batch exists to feed a consumer.
	// Refusing beats silently rendering only the first argument, which is what
	// this command used to do.
	if (all || batch) && !f.has("json") {
		fmt.Fprintln(e.errw, i18n.T("批量取路径只支持 --json（--all 或多条问题）"))
		return 2
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	mode := statMode(f)
	if all || batch {
		ids := pos
		if all {
			ids = engine.IDs(repo)
		}
		return e.pathBatch(repo, ids, mode)
	}
	p, ok := repo.Issues[pos[0]]
	if !ok {
		fmt.Fprint(e.errw, i18n.T("未找到问题: %s\n", pos[0]))
		return 1
	}
	if f.has("json") {
		return e.pathJSON(repo, p, mode)
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	fmt.Fprint(e.out, i18n.T("%s  %s（当前 %s）\n", p.ID, e.progressWord(p), stateWord(p)))
	for i, a := range p.Actions {
		line := fmt.Sprintf("  %d. %s %s %s", i+1, pad(a.Kind, 12), pad(shortID(a.ID), 8), a.Subject)
		if mode == statShort {
			stat, err := backend.ShowStat(a.ID, true)
			if err != nil {
				fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
				return 1
			}
			if s := strings.TrimSpace(stat); s != "" {
				line += "  (" + s + ")"
			}
		}
		fmt.Fprintln(e.out, line)
		if mode == statBlock {
			stat, err := backend.ShowStat(a.ID, false)
			if err != nil {
				fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
				return 1
			}
			fmt.Fprint(e.out, indentStat(stat))
		}
	}
	if lines := engine.PathLines(repo, p, e.rel()); len(lines) > 0 {
		fmt.Fprintln(e.out, i18n.T("  关系:"))
		for _, l := range lines {
			fmt.Fprintln(e.out, "    "+l)
		}
	}
	return 0
}

// pathDoc is `kiln path --json` for one issue: the whole research path as one
// object. Actions keep their 1-based ordinal and full commit hash, so a caller
// can address `kiln show` without parsing the text view. A batch wraps these in
// pathBatchDoc unchanged, so one path reads the same either way.
type pathDoc struct {
	Issue     string       `json:"issue"`
	Action    string       `json:"action"`
	Done      bool         `json:"done"`
	Actions   []pathAction `json:"actions"`
	Relations []graphEdge  `json:"relations"`
}

// pathBatchDoc is `kiln path --all --json` / `kiln path <id> <id> --json`: one
// process, every requested path. issues is always an array (empty, never null);
// a request that could not be answered is named in errors instead of aborting
// the whole document.
type pathBatchDoc struct {
	Issues []pathDoc        `json:"issues"`
	Errors []pathBatchError `json:"errors,omitempty"`
}

// pathBatchError codes. unknown-issue means the id was never opened; stat-failed
// means the backend could not produce that action's diffstat.
const (
	errUnknownIssue = "unknown-issue"
	errStatFailed   = "stat-failed"
)

// pathAction is one action in the path: what it did, when, under which hash,
// and — with --stat / --shortstat — the diffstat that belongs to it. Tags are
// the action's own `tags:` trailer (commit order, duplicates kept); they are not
// the issue-level union, which is what `graph --json` and `list --json` carry.
type pathAction struct {
	Ordinal int      `json:"ordinal"`
	Action  string   `json:"action"`
	Commit  string   `json:"commit"`
	Date    string   `json:"date"`
	Title   string   `json:"title,omitempty"`
	Refs    []string `json:"refs,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Stat    string   `json:"stat,omitempty"`
}

// pathJSON renders the path as one document. Relations reuse the graph's edge
// shape, restricted to the edges touching this issue, so a caller that already
// understands `graph --json` needs nothing new. With --stat / --shortstat the
// diffstat travels in each action's stat field instead of replacing the view.
func (e *env) pathJSON(repo *model.Repo, p *model.Issue, mode statDisplay) int {
	backend, code := e.statBackend(mode)
	if code != 0 {
		return code
	}
	doc, err := e.pathDocOf(p, mode, backend, engine.FoldEdges(repo, e.rel()), nil)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取变更失败: %v\n", err))
		return 1
	}
	return e.writeJSON(doc)
}

// pathBatch renders several paths as one document. --all follows engine.IDs
// (sorted by id); explicitly named ids keep the order they were given. A single
// bad id does not sink the document — it lands in errors, the rest still print,
// and the exit code stays non-zero so a script can notice.
//
// With --stat / --shortstat it fetches every action's diffstat in one backend
// pass when the backend offers StatLog (see batchStats), so the batch does not
// pay one process per action. A failure there is not fatal: the per-action
// fallback in pathDocOf keeps today's behavior exactly.
func (e *env) pathBatch(repo *model.Repo, ids []string, mode statDisplay) int {
	backend, code := e.statBackend(mode)
	if code != 0 {
		return code
	}
	stats := e.batchStats(repo, backend, ids, mode)
	edges := engine.FoldEdges(repo, e.rel())
	w := newPathBatchWriter(e.out)
	var errs []pathBatchError
	for _, id := range ids {
		p, ok := repo.Issues[id]
		if !ok {
			errs = append(errs, pathBatchError{Issue: id, Error: errUnknownIssue})
			continue
		}
		d, err := e.pathDocOf(p, mode, backend, edges, stats)
		if err != nil {
			errs = append(errs, pathBatchError{Issue: id, Error: errStatFailed})
			continue
		}
		w.issue(d)
		if w.err != nil {
			break
		}
	}
	if err := w.finish(errs); err != nil {
		fmt.Fprint(e.errw, i18n.T("输出 JSON 失败: %v\n", err))
		return 1
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}

// pathBatchWriter streams the batch document one issue at a time, so a large
// `path --all` never holds the whole document — at 100k actions it is 35 MB of
// JSON and one pathDoc per action — before the first byte leaves. The bytes are
// exactly what writeJSON emits for pathBatchDoc: same key order, 2-space indent,
// no HTML escaping, trailing newline.
type pathBatchWriter struct {
	buf     *bufio.Writer
	enc     *json.Encoder
	scratch bytes.Buffer
	first   bool
	err     error
}

func newPathBatchWriter(out io.Writer) *pathBatchWriter {
	w := &pathBatchWriter{buf: bufio.NewWriterSize(out, 64*1024), first: true}
	w.enc = json.NewEncoder(&w.scratch)
	w.enc.SetEscapeHTML(false)
	w.enc.SetIndent("    ", "  ")
	_, w.err = fmt.Fprint(w.buf, "{\n  \"issues\": [")
	return w
}

// encode renders v at the current nesting level. One encoder and one scratch
// buffer are reused for every issue: 100k allocations of both is measurable.
func (w *pathBatchWriter) encode(v any) ([]byte, error) {
	w.scratch.Reset()
	if err := w.enc.Encode(v); err != nil {
		return nil, err
	}
	body := bytes.TrimSuffix(w.scratch.Bytes(), []byte("\n"))
	out := make([]byte, 0, 4+len(body))
	out = append(out, "    "...)
	out = append(out, body...)
	return out, nil
}

// issue writes one element of the issues array.
func (w *pathBatchWriter) issue(doc pathDoc) {
	if w.err != nil {
		return
	}
	body, err := w.encode(doc)
	if err != nil {
		w.err = err
		return
	}
	if w.first {
		w.first = false
		_, w.err = fmt.Fprint(w.buf, "\n")
	} else {
		_, w.err = fmt.Fprint(w.buf, ",\n")
	}
	if w.err == nil {
		_, w.err = w.buf.Write(body)
	}
}

// finish closes the issues array, adds the errors array when there is one, and
// closes the document.
func (w *pathBatchWriter) finish(errs []pathBatchError) error {
	if w.err != nil {
		return w.err
	}
	if w.first {
		if _, err := fmt.Fprint(w.buf, "]"); err != nil {
			return err
		}
	} else if _, err := fmt.Fprint(w.buf, "\n  ]"); err != nil {
		return err
	}
	if len(errs) > 0 {
		if _, err := fmt.Fprint(w.buf, ",\n  \"errors\": ["); err != nil {
			return err
		}
		for i, e := range errs {
			body, err := w.encode(e)
			if err != nil {
				return err
			}
			if i == 0 {
				_, err = fmt.Fprint(w.buf, "\n")
			} else {
				_, err = fmt.Fprint(w.buf, ",\n")
			}
			if err != nil {
				return err
			}
			if _, err := w.buf.Write(body); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w.buf, "\n  ]"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w.buf, "\n}\n"); err != nil {
		return err
	}
	return w.buf.Flush()
}

// batchStats fetches the diffstats of every action in the batch in one call. It
// returns nil when no stats were asked for, when the backend has no batch form
// (jj), or when the batch call failed — nil means "let pathDocOf ask per
// action", which is the pre-batch behavior and the safety net for both cases.
func (e *env) batchStats(repo *model.Repo, backend vcs.Repo, ids []string, mode statDisplay) map[string]string {
	if mode == statNone || backend == nil {
		return nil
	}
	sl, ok := backend.(vcs.StatLog)
	if !ok {
		return nil
	}
	want := make(map[string]bool)
	for _, id := range ids {
		p, ok := repo.Issues[id]
		if !ok {
			continue
		}
		for _, a := range p.Actions {
			want[a.ID] = true
		}
	}
	if len(want) == 0 {
		return nil
	}
	stats, err := sl.LogStats(mode == statShort, func(id string) bool { return want[id] })
	if err != nil {
		return nil
	}
	return stats
}

// pathDocOf fills one path document without writing it. The folded edges are
// passed in so a batch folds them once instead of once per issue; backend may be
// nil when no stats were asked for. stats is the batch lookup from batchStats:
// a hit renders without another backend call, a miss (a merge commit, or no
// batch at all) still asks ShowStat, so no action can silently lose its stat.
func (e *env) pathDocOf(p *model.Issue, mode statDisplay, backend vcs.Repo, edges []engine.Edge, stats map[string]string) (pathDoc, error) {
	doc := pathDoc{
		Issue:     p.ID,
		Action:    p.State(),
		Done:      e.isDone(p),
		Actions:   make([]pathAction, 0, len(p.Actions)),
		Relations: incidentEdges(edges, e.rel(), p.ID),
	}
	for i, a := range p.Actions {
		item := pathAction{
			Ordinal: i + 1,
			Action:  a.Kind,
			Commit:  a.ID,
			Date:    a.Date.Format(time.RFC3339),
			Title:   a.Title,
			Refs:    a.Refs,
			Tags:    a.Tags,
		}
		if mode != statNone {
			stat, ok := stats[a.ID]
			if !ok {
				var err error
				stat, err = backend.ShowStat(a.ID, mode == statShort)
				if err != nil {
					return pathDoc{}, err
				}
			}
			item.Stat = strings.TrimSpace(stat)
		}
		doc.Actions = append(doc.Actions, item)
	}
	return doc, nil
}

// statBackend opens the backend a stat-carrying view needs. A view without stats
// works straight from the aggregated history and opens nothing.
func (e *env) statBackend(mode statDisplay) (vcs.Repo, int) {
	if mode == statNone {
		return nil, 0
	}
	b, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return nil, 1
	}
	return b, 0
}

// incidentEdges returns the folded edges touching one issue, declared arrow and
// ordering included — the same shape `graph --json` writes.
func incidentEdges(edges []engine.Edge, rel *model.RelationSet, id string) []graphEdge {
	out := make([]graphEdge, 0, 2)
	for _, ed := range edges {
		if ed.From != id && ed.To != id {
			continue
		}
		out = append(out, graphEdge{
			From:     ed.From,
			To:       ed.To,
			Name:     ed.Name,
			Directed: rel.Inverse(ed.Name) != "",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// statDisplay selects how `path` annotates each action with its diffstat.
type statDisplay int

const (
	statNone statDisplay = iota
	statBlock
	statShort
)

// statMode reads the stat flags; the histogram (--stat) wins over --shortstat.
func statMode(f flags) statDisplay {
	switch {
	case f.has("stat"):
		return statBlock
	case f.has("shortstat"):
		return statShort
	}
	return statNone
}

// statIndent nests a diffstat block under the action line it belongs to.
const statIndent = "        "

// indentStat indents every line of a git diffstat block. Empty input yields
// nothing, so an empty diff never leaves a stray indented blank line.
func indentStat(stat string) string {
	stat = trimNewlines(stat)
	if stat == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(stat, "\n") {
		b.WriteString(statIndent)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// trimNewlines drops leading and trailing blank lines while preserving the
// interior layout of a rendered block.
func trimNewlines(s string) string { return strings.Trim(s, "\n") }

// pathBatchError is one id that did not make it into issues. The code is a
// language-neutral token, not a translated sentence: JSON is data, so the same
// repository must produce the same bytes whatever --lang says.
type pathBatchError struct {
	Issue string `json:"issue"`
	Error string `json:"error"`
}
