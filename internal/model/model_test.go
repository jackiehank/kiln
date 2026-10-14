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
