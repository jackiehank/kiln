package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/i18n"
	"github.com/jackiehank/kiln/internal/model"
)

func (e *env) graph(args []string) int {
	pos, f, missing := parse(args, "json", "mermaid")
	allowed := allowedFlags("format", "mermaid", "json", "issue", "depth")
	if code := rejectMissingValues(e.errw, missing, allowed); code != 0 {
		return code
	}
	warnUnknownFlags(e.errw, f, allowed)
	if code := rejectSwitchValues(e.errw, f, "json", "mermaid"); code != 0 {
		return code
	}
	if code := rejectRepeats(e.errw, f, "format", "issue", "depth"); code != 0 {
		return code
	}
	if code := rejectExtra(e.errw, pos, 0, "kiln graph [--format=ascii|mermaid|json] [--mermaid] [--json] [--issue <id>] [--depth N]"); code != 0 {
		return code
	}
	format := f.get("format")
	if f.has("json") && f.has("mermaid") {
		fmt.Fprintln(e.errw, i18n.T("--json 与 --mermaid 冲突"))
		return 2
	}
	if f.has("json") && format != "" && format != "json" {
		fmt.Fprint(e.errw, i18n.T("--json 与 --format=%s 冲突\n", format))
		return 2
	}
	if f.has("mermaid") {
		if format != "" && format != "mermaid" && format != "ascii" {
			fmt.Fprint(e.errw, i18n.T("--mermaid 与 --format=%s 冲突\n", format))
			return 2
		}
		format = "mermaid"
	}
	if f.has("json") {
		format = "json"
	}
	if format == "" {
		format = "ascii"
	}
	if format != "ascii" && format != "mermaid" && format != "json" {
		fmt.Fprint(e.errw, i18n.T("仅支持 --format=ascii|mermaid|json（当前: %s）\n", format))
		return 2
	}
	focus, depth, qcode := e.graphQuery(f, format)
	if qcode != 0 {
		return qcode
	}
	repo, code := e.loadRepo()
	if repo == nil {
		return code
	}
	if format == "json" {
		return e.graphJSON(repo, focus, depth)
	}
	if len(repo.Issues) == 0 {
		fmt.Fprintln(e.out, i18n.T("（空）"))
		return 0
	}
	if format == "mermaid" {
		fmt.Fprintln(e.out, engine.Mermaid(repo, e.rel()))
		return 0
	}
	fmt.Fprintln(e.out, engine.Graph(repo, e.rel()))
	return 0
}

// graphQuery validates the neighbourhood flags. They only apply to the JSON
// view (other formats say so instead of silently printing everything), a
// depth needs an issue to be a depth from, and the depth is a non-negative
// integer with a default of 1. A zero code means "no filter".
func (e *env) graphQuery(f flags, format string) (focus string, depth int, code int) {
	if !f.has("issue") && !f.has("depth") {
		return "", 0, 0
	}
	if format != "json" {
		fmt.Fprintln(e.errw, i18n.T("--issue/--depth 只服务 --json"))
		return "", 0, 2
	}
	if !f.has("issue") {
		fmt.Fprintln(e.errw, i18n.T("--depth 需要 --issue"))
		return "", 0, 2
	}
	depth = 1
	if f.has("depth") {
		n, err := strconv.Atoi(f.get("depth"))
		if err != nil || n < 0 {
			fmt.Fprintln(e.errw, i18n.T("--depth 需要非负整数"))
			return "", 0, 2
		}
		depth = n
	}
	return f.get("issue"), depth, 0
}

// graphDoc is the `kiln graph --json` document: the whole folded graph as one
// object. Nodes are always real issues; a target that was declared but never
// opened is named in missing, so a caller checks membership instead of decoding
// a per-node flag.
type graphDoc struct {
	Summary graphSummary `json:"summary"`
	Nodes   []graphNode  `json:"nodes"`
	Missing []string     `json:"missing,omitempty"`
	Edges   []graphEdge  `json:"edges"`
}

type graphSummary struct {
	Issues  int `json:"issues"`
	Edges   int `json:"edges"`
	Missing int `json:"missing"`
}

// graphNode is one issue as the graph sees it: what it is called, where it
// stands, and what its latest action said it was doing.
type graphNode struct {
	ID     string   `json:"id"`
	Domain string   `json:"domain,omitempty"`
	Action string   `json:"action"`
	Done   bool     `json:"done"`
	Title  string   `json:"title,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// graphEdge is one folded relationship, named the way it was declared. directed
// is always written: true is an arrow, false is a symmetric line — so a caller
// never needs the relation vocabulary to read the arrow.
type graphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Name     string `json:"name"`
	Directed bool   `json:"directed"`
}

// graphJSON renders the folded graph as one document. Edges come from the same
// FoldEdges the ascii and mermaid views draw, so the three renderings agree.
// Everything is ordered by id or by (from, to, name): the same repository always
// prints the same document.
func (e *env) graphJSON(repo *model.Repo, focus string, depth int) int {
	rel := e.rel()
	ids := engine.IDs(repo)
	edges := engine.FoldEdges(repo, rel)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Name < edges[j].Name
	})
	if focus != "" {
		if _, ok := repo.Issues[focus]; !ok {
			fmt.Fprint(e.errw, i18n.T("未找到问题: %s\n", focus))
			return 1
		}
		ids, edges = neighbourhood(ids, edges, focus, depth)
	}

	doc := graphDoc{
		Nodes: make([]graphNode, 0, len(ids)),
		Edges: make([]graphEdge, 0, len(edges)),
	}
	for _, id := range ids {
		p := repo.Issues[id]
		title := ""
		if last, ok := p.Last(); ok {
			title = last.Title
		}
		doc.Nodes = append(doc.Nodes, graphNode{
			ID:     id,
			Domain: p.Domain,
			Action: p.State(),
			Done:   e.isDone(p),
			Title:  title,
			Tags:   p.Tags,
		})
	}

	missing := map[string]bool{}
	for _, ed := range edges {
		if _, ok := repo.Issues[ed.To]; !ok {
			missing[ed.To] = true
		}
		doc.Edges = append(doc.Edges, graphEdge{
			From:     ed.From,
			To:       ed.To,
			Name:     ed.Name,
			Directed: rel.Inverse(ed.Name) != "",
		})
	}
	for id := range missing {
		doc.Missing = append(doc.Missing, id)
	}
	sort.Strings(doc.Missing)
	doc.Summary = graphSummary{Issues: len(doc.Nodes), Edges: len(doc.Edges), Missing: len(doc.Missing)}
	return e.writeJSON(doc)
}

// neighbourhood cuts the graph down to the issues within depth undirected hops
// of focus. Neighbours are found through both ends of every edge: a relation's
// arrow says who declared it, not which side is relevant. The ids keep their
// original order; an edge survives when both of its ends are inside, so an
// unopened target that is reachable stays visible and lands in missing exactly
// as it does in the full document.
func neighbourhood(ids []string, edges []engine.Edge, focus string, depth int) ([]string, []engine.Edge) {
	adj := map[string][]string{}
	for _, ed := range edges {
		adj[ed.From] = append(adj[ed.From], ed.To)
		adj[ed.To] = append(adj[ed.To], ed.From)
	}
	dist := map[string]int{focus: 0}
	queue := []string{focus}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if dist[cur] >= depth {
			continue
		}
		for _, next := range adj[cur] {
			if _, ok := dist[next]; ok {
				continue
			}
			dist[next] = dist[cur] + 1
			queue = append(queue, next)
		}
	}
	keptIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := dist[id]; ok {
			keptIDs = append(keptIDs, id)
		}
	}
	keptEdges := make([]engine.Edge, 0, len(edges))
	for _, ed := range edges {
		_, fromOK := dist[ed.From]
		_, toOK := dist[ed.To]
		if fromOK && toOK {
			keptEdges = append(keptEdges, ed)
		}
	}
	return keptIDs, keptEdges
}
