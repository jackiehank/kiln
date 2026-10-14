package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoPrintsCanonicalMessage(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "auth/session-expiry", "ask", "q1", "--title", "X 并发可能出错"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errb.String())
	}
	want := "X 并发可能出错\n\nissue: auth/session-expiry\naction: ask\nobject: q1\ntitle: X 并发可能出错\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestDoRejectsUnknownAction(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "auth/x", "frobnicate"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "非法 action") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestDoWarnsOnLooseIssueID(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"do", "badid", "ask"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(errb.String(), "不符合") {
		t.Fatalf("expected a naming warning, stderr = %q", errb.String())
	}
	if !strings.Contains(out.String(), "issue: badid") {
		t.Fatalf("expected output to still be produced, got %q", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"nope"}, &out, &errb); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestDisplayWidthCountsWideRunes(t *testing.T) {
	cases := map[string]int{"abc": 3, "中文": 4, "a中": 3, "": 0}
	for s, want := range cases {
		if got := displayWidth(s); got != want {
			t.Fatalf("displayWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestPadUsesDisplayWidth(t *testing.T) {
	if got := pad("中文", 6); got != "中文  " {
		t.Fatalf("pad cjk = %q", got)
	}
	if got := pad("ab", 4); got != "ab  " {
		t.Fatalf("pad ascii = %q", got)
	}
	if got := pad("abcdef", 3); got != "abcdef" {
		t.Fatalf("pad must not truncate: %q", got)
	}
}

// Every command line must place its description at the same display column,
// even though the commands differ in length.
func TestUsageTextAlignsDescriptions(t *testing.T) {
	text := usageText()
	col := -1
	for _, c := range usageCommands {
		i := strings.Index(text, "  "+c[0])
		if i < 0 {
			t.Fatalf("command %q missing from usage text", c[0])
		}
		line := text[i:]
		if j := strings.Index(line, "\n"); j >= 0 {
			line = line[:j]
		}
		if got := displayWidth(line) - displayWidth(c[1]); col == -1 {
			col = got
		} else if got != col {
			t.Fatalf("description column %d != %d for %q", got, col, c[0])
		}
	}
}
