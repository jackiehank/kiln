package jj

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"github.com/jackiehank/kiln/internal/vcs"
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

// `jj workspace list -T` yields name/root records. The "default" workspace is
// the main checkout; every other one is dedicated and named after its issue.
// `jj diff --summary` is one status letter and a path per line; Windows renders
// the separator natively, so the inventory normalizes to slashes and sorts.
func TestParseSummaryNormalizesAndSorts(t *testing.T) {
	out := "M a.md\nD b.md\nA dir\\x.md\n"
	got := parseSummary(out)
	want := []string{"a.md", "b.md", "dir/x.md"}
	if len(got) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseSummarySkipsBlankAndMalformedLines(t *testing.T) {
	if got := parseSummary("\n  \nM\n"); len(got) != 0 {
		t.Fatalf("got %v, want no paths", got)
	}
}

func TestParseWorkspacesReadsNameAndRoot(t *testing.T) {
	out := "default\x1fC:/w/repo\x1e" +
		"cli-necessity\x1fC:/w/repo--cli-necessity\x1e"
	got := parseWorkspaces(out)
	if len(got) != 2 {
		t.Fatalf("got %d areas, want 2", len(got))
	}
	if !got[0].Main || got[0].Issue != "" || got[0].Path != "C:/w/repo" {
		t.Fatalf("default workspace = %+v; want the main area with no issue", got[0])
	}
	if got[1].Main || got[1].Issue != "cli-necessity" {
		t.Fatalf("dedicated workspace = %+v; want issue cli-necessity", got[1])
	}
}

func TestParseWorkspacesSkipsEmptyRecords(t *testing.T) {
	if got := parseWorkspaces("\n\x1e\n"); len(got) != 0 {
		t.Fatalf("got %d areas, want 0", len(got))
	}
}

// One json(self) object per line is one commit: identity, RFC3339 date and the
// description as the message (its trailing newline trimmed so it matches Log).
func TestStreamRecordsParsesOnePerLine(t *testing.T) {
	in := `{"commit_id":"aaa111","parents":[],"change_id":"x","description":"first: probe\n\nissue: demo\naction: ask\n","author":{"name":"p","email":"p@e","timestamp":"2026-10-28T22:18:36+08:00"},"committer":{}}` + "\n" +
		`{"commit_id":"bbb222","description":"second\n","author":{"timestamp":"2026-10-28T22:20:00+08:00"}}` + "\n"
	var got []vcs.Commit
	stopped, err := streamRecords(bufio.NewReader(strings.NewReader(in)), func(c vcs.Commit) bool {
		got = append(got, c)
		return true
	})
	if err != nil || stopped {
		t.Fatalf("stopped=%v err=%v", stopped, err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d commits, want 2", len(got))
	}
	if got[0].ID != "aaa111" || got[1].ID != "bbb222" {
		t.Fatalf("ids = %q, %q", got[0].ID, got[1].ID)
	}
	if want := "first: probe\n\nissue: demo\naction: ask"; got[0].Message != want {
		t.Fatalf("message = %q, want %q", got[0].Message, want)
	}
	if want := "2026-10-28T22:18:36+08:00"; got[0].Date.Format(time.RFC3339) != want {
		t.Fatalf("date = %q, want %q", got[0].Date.Format(time.RFC3339), want)
	}
	if got[1].Message != "second" {
		t.Fatalf("trailing newline must be trimmed, got %q", got[1].Message)
	}
}

// A sink that says stop ends the read; the remaining lines are not visited.
func TestStreamRecordsStopsWhenTheSinkSaysSo(t *testing.T) {
	in := `{"commit_id":"a","description":"x","author":{"timestamp":"2026-10-28T22:00:00+08:00"}}` + "\n" +
		`{"commit_id":"b","description":"y","author":{"timestamp":"2026-10-28T22:00:01+08:00"}}` + "\n"
	calls := 0
	stopped, err := streamRecords(bufio.NewReader(strings.NewReader(in)), func(vcs.Commit) bool {
		calls++
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if !stopped || calls != 1 {
		t.Fatalf("stopped=%v calls=%d, want true and 1", stopped, calls)
	}
}

// A malformed record is an error, never a silently dropped row.
func TestStreamRecordsRejectsBadRecords(t *testing.T) {
	cases := []string{
		"not json\n",
		`{"commit_id":"a","description":"x","author":{"timestamp":"not-a-time"}}` + "\n",
	}
	for _, in := range cases {
		if _, err := streamRecords(bufio.NewReader(strings.NewReader(in)), func(vcs.Commit) bool { return true }); err == nil {
			t.Fatalf("input %q must fail", in)
		}
	}
}

// Blank lines (a trailing newline, an empty render) are skipped, not parsed.
func TestStreamRecordsSkipsBlankLines(t *testing.T) {
	in := "\n" + `{"commit_id":"a","description":"x","author":{"timestamp":"2026-10-28T22:00:00+08:00"}}` + "\n\n"
	calls := 0
	if _, err := streamRecords(bufio.NewReader(strings.NewReader(in)), func(vcs.Commit) bool {
		calls++
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("sink called %d times, want 1", calls)
	}
}
