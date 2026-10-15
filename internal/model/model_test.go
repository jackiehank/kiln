package model

import "testing"

func TestNewActionSetRejectsBadInput(t *testing.T) {
	cases := []struct {
		name     string
		names    []string
		start    string
		terminal []string
	}{
		{"empty vocabulary", nil, "ask", []string{"done"}},
		{"blank name", []string{"ask", "  "}, "ask", []string{"ask"}},
		{"duplicate name", []string{"ask", "Ask"}, "ask", []string{"ask"}},
		{"start not in set", []string{"ask", "done"}, "zzz", []string{"done"}},
		{"empty terminal", []string{"ask", "done"}, "ask", nil},
		{"terminal not in set", []string{"ask", "done"}, "ask", []string{"nope"}},
	}
	for _, c := range cases {
		if _, err := NewActionSet(c.names, c.start, c.terminal); err == nil {
			t.Fatalf("%s: expected an error", c.name)
		}
	}
}

func TestActionSetNormalizesNames(t *testing.T) {
	set, err := NewActionSet([]string{" Ask ", "Done"}, "ASK", []string{"DONE"})
	if err != nil {
		t.Fatal(err)
	}
	if !set.IsValid("ask") || !set.IsStart("ask") {
		t.Fatalf("start not normalized: %v", set.Names())
	}
	if !set.IsTerminal("done") || set.IsTerminal("ask") {
		t.Fatalf("terminal not normalized: %v", set.Names())
	}
}

func TestValidIssueID(t *testing.T) {
	for _, id := range []string{"action-config", "status-by-domain", "a", "issue1", "x-1-2"} {
		if !ValidIssueID(id) {
			t.Fatalf("%q should be valid", id)
		}
	}
	for _, id := range []string{"", "Bad", "has space", "a/b", "UPPER", "中文", "with_underscore"} {
		if ValidIssueID(id) {
			t.Fatalf("%q should be invalid", id)
		}
	}
}

func TestDefaultActionSet(t *testing.T) {
	set := DefaultActionSet()
	if len(set.Names()) != 6 {
		t.Fatalf("names = %v", set.Names())
	}
	if set.Start() != Ask {
		t.Fatalf("start = %q", set.Start())
	}
	if !set.IsTerminal(Conclude) || !set.IsTerminal(Apply) || set.IsTerminal(Collect) {
		t.Fatalf("terminal set wrong for %v", set.Names())
	}
}

func TestNormalizeIssueID(t *testing.T) {
	cases := map[string]string{
		"action-config":    "action-config",
		"Session-Expiry":   "session-expiry",
		"session_expiry":   "session-expiry",
		"Session__Expiry":  "session-expiry",
		"a--b":             "a-b",
		"-lead-and-trail-": "lead-and-trail",
		"  padded  ":       "padded",
		"a_ -_b":           "a- -b",  // the space has no single answer: not folded
		"bad/id":           "bad/id", // no single answer: left for ValidIssueID
		"has space":        "has space",
		"中文":               "中文",
		"":                 "",
	}
	for in, want := range cases {
		if got := NormalizeIssueID(in); got != want {
			t.Fatalf("NormalizeIssueID(%q) = %q, want %q", in, got, want)
		}
	}
	// Folded spellings must land on a valid ID when the input was otherwise legal.
	for _, in := range []string{"Session_Expiry", "a--b-", "UPPER"} {
		if got := NormalizeIssueID(in); !ValidIssueID(got) {
			t.Fatalf("NormalizeIssueID(%q) = %q, which is not a valid ID", in, got)
		}
	}
}
