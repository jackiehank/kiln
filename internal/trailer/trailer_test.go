package trailer

import (
	"reflect"
	"testing"

	"github.com/jackiehank/kiln/internal/model"
)

var testRel = model.DefaultRelationSet()

func relTargets(f Fields, name string) []string {
	var out []string
	for _, r := range f.Relations {
		if r.Name == name {
			out = append(out, r.Target)
		}
	}
	return out
}

func TestParseDetectsKilnAction(t *testing.T) {
	msg := "发现 X 并发可能出错\nissue: auth/session-expiry\naction: ask\nobject: q1\n"
	f, ok := Parse(msg, testRel)
	if !ok {
		t.Fatalf("expected a kiln action")
	}
	if f.Issue != "auth/session-expiry" || f.Action != "ask" {
		t.Fatalf("unexpected fields: %+v", f)
	}
}

func TestParseIgnoresLoneTrailers(t *testing.T) {
	cases := map[string]string{
		"plain commit":     "fix: 修复登录超时\n\n问题出在 session 过期时间配置错误",
		"issue only":       "issue: 性能瓶颈在数据库\n",
		"action only":      "action: ask\n",
		"unknown key":      "note: issue: 不算\n",
		"empty value":      "issue:\naction:\n",
		"subject mentions": "issue: 和 action: 是 kiln 约定的 trailer",
	}
	for name, msg := range cases {
		if _, ok := Parse(msg, testRel); ok {
			t.Fatalf("%s: expected non-kiln commit for %q", name, msg)
		}
	}
}

func TestParseIsLenient(t *testing.T) {
	msg := "Issue:My/Thing\nACTION : Ask\ntags: [security, performance]\nrefs: q1,h1\n"
	f, ok := Parse(msg, testRel)
	if !ok {
		t.Fatalf("expected a kiln action despite messy formatting")
	}
	if f.Issue != "My/Thing" {
		t.Fatalf("issue = %q", f.Issue)
	}
	if f.Action != "ask" {
		t.Fatalf("action should be lower-cased, got %q", f.Action)
	}
	if !reflect.DeepEqual(f.Tags, []string{"security", "performance"}) {
		t.Fatalf("tags = %+v", f.Tags)
	}
	if !reflect.DeepEqual(f.Refs, []string{"q1", "h1"}) {
		t.Fatalf("refs = %+v", f.Refs)
	}
}

// Legacy names read through aliases: derived-from -> caused-by, spawns -> causes.
func TestParseRelationAliases(t *testing.T) {
	msg := "issue: a/x\naction: ask\nderived-from: b/y\nspawns: c/z, d/w\n"
	f, _ := Parse(msg, testRel)
	if got := relTargets(f, "caused-by"); !reflect.DeepEqual(got, []string{"b/y"}) {
		t.Fatalf("derived-from alias -> caused-by = %+v", got)
	}
	if got := relTargets(f, "causes"); !reflect.DeepEqual(got, []string{"c/z", "d/w"}) {
		t.Fatalf("spawns alias -> causes = %+v", got)
	}
}

func TestFormatRoundTrip(t *testing.T) {
	in := Fields{
		Issue:     "auth/session-expiry",
		Action:    "ask",
		Title:     "X 并发可能出错",
		Tags:      []string{"security", "performance"},
		Refs:      []string{"q1", "h1"},
		Relations: []model.Rel{{Name: "caused-by", Target: "perf/deep-dive"}},
		Unrelate:  []string{"old/thing"},
	}
	msg := Format(in, testRel)
	back, ok := Parse(msg, testRel)
	if !ok {
		t.Fatalf("round trip lost detection:\n%s", msg)
	}
	if back.Issue != in.Issue || back.Action != in.Action || back.Title != in.Title {
		t.Fatalf("round trip scalar mismatch:\n%s\n%+v", msg, back)
	}
	if !reflect.DeepEqual(back.Tags, in.Tags) || !reflect.DeepEqual(back.Refs, in.Refs) {
		t.Fatalf("round trip list mismatch: %+v", back)
	}
	if !reflect.DeepEqual(back.Relations, in.Relations) {
		t.Fatalf("round trip relation mismatch: %+v", back.Relations)
	}
	if !reflect.DeepEqual(back.Unrelate, in.Unrelate) {
		t.Fatalf("round trip unrelate mismatch: %+v", back.Unrelate)
	}
	// Format is strict: canonical key casing and spacing, with a blank line
	// separating the subject from the trailer block, and relations in
	// vocabulary order before unrelate.
	if msg != "X 并发可能出错\n\nissue: auth/session-expiry\naction: ask\ntitle: X 并发可能出错\nrefs: q1, h1\ntags: security, performance\ncaused-by: perf/deep-dive\nunrelate: old/thing" {
		t.Fatalf("unexpected canonical output:\n%s", msg)
	}
}

func TestFormatSkipsBlankListItems(t *testing.T) {
	cases := []struct {
		name string
		refs []string
		want string
	}{
		{"only empty item", []string{""}, "issue: x\naction: ask"},
		{"item that is blank", []string{"a", "  "}, "issue: x\naction: ask\nrefs: a"},
		{"normal items", []string{"a", "b"}, "issue: x\naction: ask\nrefs: a, b"},
	}
	for _, c := range cases {
		got := Format(Fields{Issue: "x", Action: "ask", Refs: c.refs}, testRel)
		if got != c.want {
			t.Errorf("%s: Format(Refs=%+v) =\n%q\nwant:\n%q", c.name, c.refs, got, c.want)
		}
	}
}

// UnknownKeys names the keys that look like trailers but are not: a typo in the
// block would otherwise be parsed away without a word, and the writer would
// think they declared it. The block is only the last paragraph and only when it
// carries `issue:`, so prose that merely contains a colon is not a trailer.
func TestUnknownKeys(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want []string
	}{
		{"typo in the trailer block", "结题\n\nissue: a\naction: conclude\ncaused-byy: b", []string{"caused-byy"}},
		{"several typos, deduped and sorted", "结题\n\nissue: a\naction: conclude\nz: 1\nb: 2\nz: 3", []string{"b", "z"}},
		{"prose colon before the block", "注意: 正文里有冒号\n\nissue: a\naction: ask", nil},
		{"retired key is not a typo", "结题\n\nissue: a\naction: conclude\nobject: q1", nil},
		{"alias is a known key", "结题\n\nissue: a\naction: conclude\nderived-from: b", nil},
		{"no kiln trailer, nothing to judge", "随便\n\nfoo: bar", nil},
		{"empty value is not a declaration", "结题\n\nissue: a\naction: conclude\nfoo:", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnknownKeys(c.msg, testRel)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("UnknownKeys(%q) = %v, want %v", c.msg, got, c.want)
			}
		})
	}
}
