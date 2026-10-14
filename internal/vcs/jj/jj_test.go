package jj

import (
	"testing"
	"time"
)

func TestParseLogSplitsRecords(t *testing.T) {
	out := "id1\n2026-10-14 21:30:47.000 +08:00\nfirst\n\nissue: demo\naction: ask\n@@KILN@@\n" +
		"id2\n2026-10-14 22:00:00.000 +08:00\nsecond\n@@KILN@@\n"
	got := parseLog(out)
	if len(got) != 2 {
		t.Fatalf("got %d commits, want 2", len(got))
	}
	if got[0].ID != "id1" || got[1].ID != "id2" {
		t.Fatalf("ids = %q, %q", got[0].ID, got[1].ID)
	}
	if want := "first\n\nissue: demo\naction: ask"; got[0].Message != want {
		t.Fatalf("message = %q, want %q", got[0].Message, want)
	}
	want := time.Date(2026, 10, 14, 21, 30, 47, 0, time.FixedZone("", 8*3600))
	if !got[0].Date.Equal(want) {
		t.Fatalf("date = %v, want %v", got[0].Date, want)
	}
}

func TestParseLogSkipsEmptyRecords(t *testing.T) {
	if got := parseLog("\n@@KILN@@\n"); len(got) != 0 {
		t.Fatalf("got %d commits, want 0", len(got))
	}
}

func TestLastLine(t *testing.T) {
	in := "a.md | 1 +\n1 file changed, 1 insertion(+), 0 deletions(-)\n"
	if got := lastLine(in); got != "1 file changed, 1 insertion(+), 0 deletions(-)" {
		t.Fatalf("lastLine = %q", got)
	}
	if got := lastLine("\n\n"); got != "" {
		t.Fatalf("lastLine of blanks = %q", got)
	}
}
