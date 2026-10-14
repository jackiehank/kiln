package trailer

import (
	"reflect"
	"testing"
)

func TestParseDetectsKilnAction(t *testing.T) {
	msg := "发现 X 并发可能出错\nproblem: auth/session-expiry\naction: ask\nobject: q1\n"
	f, ok := Parse(msg)
	if !ok {
		t.Fatalf("expected a kiln action")
	}
	if f.Problem != "auth/session-expiry" || f.Action != "ask" || f.Object != "q1" {
		t.Fatalf("unexpected fields: %+v", f)
	}
}

func TestParseIgnoresLoneTrailers(t *testing.T) {
	cases := map[string]string{
		"plain commit":     "fix: 修复登录超时\n\n问题出在 session 过期时间配置错误",
		"problem only":     "problem: 性能瓶颈在数据库\n",
		"action only":      "action: ask\n",
		"unknown key":      "note: problem: 不算\n",
		"empty value":      "problem:\naction:\n",
		"subject mentions": "problem: 和 action: 是 kiln 约定的 trailer",
	}
	for name, msg := range cases {
		if _, ok := Parse(msg); ok {
			t.Fatalf("%s: expected non-kiln commit for %q", name, msg)
		}
	}
}

func TestParseIsLenient(t *testing.T) {
	msg := "Problem:My/Thing\nACTION : Ask\ntags: [security, performance]\nrefs: q1,h1\n"
	f, ok := Parse(msg)
	if !ok {
		t.Fatalf("expected a kiln action despite messy formatting")
	}
	if f.Problem != "My/Thing" {
		t.Fatalf("problem = %q", f.Problem)
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

func TestParseRelationLists(t *testing.T) {
	msg := "problem: a/x\naction: ask\nderived-from: b/y\nspawns: c/z, d/w\n"
	f, _ := Parse(msg)
	if !reflect.DeepEqual(f.DerivedFrom, []string{"b/y"}) {
		t.Fatalf("derived-from = %+v", f.DerivedFrom)
	}
	if !reflect.DeepEqual(f.Spawns, []string{"c/z", "d/w"}) {
		t.Fatalf("spawns = %+v", f.Spawns)
	}
}

func TestFormatRoundTrip(t *testing.T) {
	in := Fields{
		Problem:     "auth/session-expiry",
		Action:      "ask",
		Object:      "q1",
		Title:       "X 并发可能出错",
		Tags:        []string{"security", "performance"},
		Refs:        []string{"q1", "h1"},
		DerivedFrom: []string{"perf/deep-dive"},
	}
	msg := Format(in)
	back, ok := Parse(msg)
	if !ok {
		t.Fatalf("round trip lost detection:\n%s", msg)
	}
	if back.Problem != in.Problem || back.Action != in.Action || back.Object != in.Object || back.Title != in.Title {
		t.Fatalf("round trip scalar mismatch:\n%s\n%+v", msg, back)
	}
	if !reflect.DeepEqual(back.Tags, in.Tags) || !reflect.DeepEqual(back.Refs, in.Refs) {
		t.Fatalf("round trip list mismatch: %+v", back)
	}
	if !reflect.DeepEqual(back.DerivedFrom, in.DerivedFrom) {
		t.Fatalf("round trip relation mismatch: %+v", back.DerivedFrom)
	}
	// Format is strict: canonical key casing and spacing.
	if msg != "X 并发可能出错\nproblem: auth/session-expiry\naction: ask\nobject: q1\ntitle: X 并发可能出错\nrefs: q1, h1\ntags: security, performance\nderived-from: perf/deep-dive" {
		t.Fatalf("unexpected canonical output:\n%s", msg)
	}
}
