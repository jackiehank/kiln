// Package trailer parses kiln's semantic metadata out of a git commit message.
//
// A commit is a kiln action iff it carries both an `issue:` and an `action:`
// trailer. Parsing is deliberately lenient (case-insensitive keys, optional
// spaces around the colon, `[a, b]` or comma-separated lists) so that agents and
// humans can write naturally; formatting (see Format) is strict.
package trailer

import "strings"

// Fields is the semantic content of one kiln commit message.
type Fields struct {
	Issue  string
	Action string
	Object string
	Title  string

	Refs   []string
	Target string
	Scope  string
	Domain string
	Tags   []string

	DerivedFrom []string
	Spawns      []string
	RelatesTo   []string
	Blocks      []string
	Supersedes  []string
}

var knownKeys = map[string]bool{
	"issue": true, "action": true, "object": true, "title": true,
	"refs": true, "target": true, "scope": true, "domain": true, "tags": true,
	"derived-from": true, "spawns": true, "relates-to": true,
	"blocks": true, "supersedes": true,
}

// Parse scans every line of message for known trailers. It returns the parsed
// fields and whether the commit qualifies as a kiln action.
func Parse(message string) (Fields, bool) {
	var f Fields
	for _, raw := range strings.Split(message, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		val := strings.TrimSpace(line[i+1:])
		if val == "" || !knownKeys[key] {
			continue
		}
		switch key {
		case "issue":
			f.Issue = val
		case "action":
			f.Action = val
		case "object":
			f.Object = val
		case "title":
			f.Title = val
		case "target":
			f.Target = val
		case "scope":
			f.Scope = val
		case "domain":
			f.Domain = val
		case "refs":
			f.Refs = append(f.Refs, splitList(val)...)
		case "tags":
			f.Tags = append(f.Tags, splitList(val)...)
		case "derived-from":
			f.DerivedFrom = append(f.DerivedFrom, splitList(val)...)
		case "spawns":
			f.Spawns = append(f.Spawns, splitList(val)...)
		case "relates-to":
			f.RelatesTo = append(f.RelatesTo, splitList(val)...)
		case "blocks":
			f.Blocks = append(f.Blocks, splitList(val)...)
		case "supersedes":
			f.Supersedes = append(f.Supersedes, splitList(val)...)
		}
	}
	f.Action = strings.ToLower(f.Action)
	return f, f.Issue != "" && f.Action != ""
}

// splitList accepts `[a, b]` or `a, b` and returns the trimmed items.
func splitList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// order is the canonical trailer order used by Format.
var order = []string{
	"issue", "action", "object", "title", "refs", "target", "scope",
	"domain", "tags", "derived-from", "spawns", "relates-to", "blocks", "supersedes",
}

// Format renders fields as a canonical kiln commit message: the title (when
// set) on the subject line, a blank line, then trailers in a fixed order. The
// blank line keeps git from folding the whole message into one "subject"
// paragraph, so `git log --oneline` and PR titles show only the title.
func Format(f Fields) string {
	var b strings.Builder
	if f.Title != "" {
		b.WriteString(f.Title)
		b.WriteString("\n\n")
	}
	for _, k := range order {
		switch k {
		case "issue":
			writeScalar(&b, "issue", f.Issue)
		case "action":
			writeScalar(&b, "action", f.Action)
		case "object":
			writeScalar(&b, "object", f.Object)
		case "title":
			writeScalar(&b, "title", f.Title)
		case "refs":
			writeList(&b, "refs", f.Refs)
		case "target":
			writeScalar(&b, "target", f.Target)
		case "scope":
			writeScalar(&b, "scope", f.Scope)
		case "domain":
			writeScalar(&b, "domain", f.Domain)
		case "tags":
			writeList(&b, "tags", f.Tags)
		case "derived-from":
			writeList(&b, "derived-from", f.DerivedFrom)
		case "spawns":
			writeList(&b, "spawns", f.Spawns)
		case "relates-to":
			writeList(&b, "relates-to", f.RelatesTo)
		case "blocks":
			writeList(&b, "blocks", f.Blocks)
		case "supersedes":
			writeList(&b, "supersedes", f.Supersedes)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeScalar(b *strings.Builder, key, val string) {
	if val == "" {
		return
	}
	b.WriteString(key + ": " + val + "\n")
}

func writeList(b *strings.Builder, key string, vals []string) {
	items := make([]string, 0, len(vals))
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			items = append(items, v)
		}
	}
	if len(items) == 0 {
		return
	}
	b.WriteString(key + ": " + strings.Join(items, ", ") + "\n")
}
