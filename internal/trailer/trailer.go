// Package trailer parses kiln's semantic metadata out of a git commit message.
//
// A commit is a kiln action iff it carries both an `issue:` and an `action:`
// trailer. Parsing is deliberately lenient (case-insensitive keys, optional
// spaces around the colon, `[a, b]` or comma-separated lists) so that agents and
// humans can write naturally; formatting (see Format) is strict.
//
// Relation keys are not hardcoded: the vocabulary comes from the RelationSet, so
// renamed and project-custom names read the same way (and legacy names stay
// readable through aliases).
package trailer

import (
	"sort"
	"strings"

	"github.com/jackiehank/kiln/internal/model"
)

// Fields is the semantic content of one kiln commit message.
type Fields struct {
	Issue  string
	Action string
	Title  string

	Refs   []string
	Target string
	Scope  string
	Domain string
	Tags   []string

	// Relations are canonicalized declarations (aliases resolved). Unrelate
	// lists targets whose relations this action voids.
	Relations []model.Rel
	Unrelate  []string
}

// scalarKeys are the fixed (non-relation) trailers; relation keys come from the
// vocabulary and "unrelate" is always recognized.
var scalarKeys = map[string]bool{
	"issue": true, "action": true, "title": true,
	"refs": true, "target": true, "scope": true, "domain": true, "tags": true,
	"unrelate": true,
}

// retiredKeys are trailers kiln once wrote and deliberately stopped using. They
// are ignored without an "unknown key" warning: retired is not the same as a
// typo.
var retiredKeys = map[string]bool{"object": true}

// UnknownKeys returns the trailer-block keys that are neither fixed trailers nor
// vocabulary names/aliases. The trailer block is the last blank-line-separated
// paragraph, and it only counts when it carries at least one kiln trailer
// (issue:), so a prose line like "注意: …" is not mistaken for a trailer.
func UnknownKeys(message string, rel *model.RelationSet) []string {
	body := strings.TrimRight(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	paras := strings.Split(body, "\n\n")
	block := paras[len(paras)-1]
	hasIssue := false
	var keys []string
	for _, raw := range strings.Split(block, "\n") {
		line := strings.TrimSpace(raw)
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		if strings.TrimSpace(line[i+1:]) == "" {
			continue
		}
		if key == "issue" {
			hasIssue = true
		}
		keys = append(keys, key)
	}
	if !hasIssue {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, key := range keys {
		if scalarKeys[key] || retiredKeys[key] || seen[key] {
			continue
		}
		if _, ok := rel.Resolve(key); ok {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// Parse scans every line of message for known trailers. It returns the parsed
// fields and whether the commit qualifies as a kiln action.
func Parse(message string, rel *model.RelationSet) (Fields, bool) {
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
		if val == "" {
			continue
		}
		if !scalarKeys[key] {
			if n, ok := rel.Resolve(key); ok {
				for _, t := range SplitList(val) {
					f.Relations = append(f.Relations, model.Rel{Name: n.Name, Target: t})
				}
			}
			continue
		}
		switch key {
		case "issue":
			f.Issue = val
		case "action":
			f.Action = val
		case "title":
			f.Title = val
		case "target":
			f.Target = val
		case "scope":
			f.Scope = val
		case "domain":
			f.Domain = val
		case "refs":
			f.Refs = append(f.Refs, SplitList(val)...)
		case "tags":
			f.Tags = append(f.Tags, SplitList(val)...)
		case "unrelate":
			f.Unrelate = append(f.Unrelate, SplitList(val)...)
		}
	}
	f.Action = strings.ToLower(f.Action)
	return f, f.Issue != "" && f.Action != ""
}

// SplitList accepts `[a, b]` or `a, b` and returns the trimmed items. It is
// exported because `target` is the one trailer kept verbatim in Fields (a
// scalar, since the flag is optional and repeated lines are rare); readers that
// want the list shape apply the same rule here instead of inventing a second
// one.
func SplitList(v string) []string {
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

// Format renders fields as a canonical kiln commit message: the title (when
// set) on the subject line, a blank line, then trailers in a fixed order
// (relations grouped by name, in vocabulary order, then unrelate). The blank
// line keeps git from folding the whole message into one "subject" paragraph,
// so `git log --oneline` and PR titles show only the title.
func Format(f Fields, rel *model.RelationSet) string {
	var b strings.Builder
	if f.Title != "" {
		b.WriteString(f.Title)
		b.WriteString("\n\n")
	}
	writeScalar(&b, "issue", f.Issue)
	writeScalar(&b, "action", f.Action)
	writeScalar(&b, "title", f.Title)
	writeList(&b, "refs", f.Refs)
	writeScalar(&b, "target", f.Target)
	writeScalar(&b, "scope", f.Scope)
	writeScalar(&b, "domain", f.Domain)
	writeList(&b, "tags", f.Tags)
	byName := map[string][]string{}
	for _, r := range f.Relations {
		byName[r.Name] = append(byName[r.Name], r.Target)
	}
	for _, name := range rel.Names() {
		if tg := byName[name]; len(tg) > 0 {
			writeList(&b, name, tg)
		}
	}
	for name, tg := range byName {
		if _, ok := rel.Lookup(name); !ok {
			writeList(&b, name, tg)
		}
	}
	writeList(&b, "unrelate", f.Unrelate)
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
