package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

// The index is an optimization, not a second answer: for every issue it must
// return exactly what the scan returns — ordering, direction and self-relations
// included.
func TestIncidentIndexMatchesScan(t *testing.T) {
	rel := model.DefaultRelationSet()
	edges := []engine.Edge{
		{From: "alpha", To: "beta", Name: "blocks", Slot: "ordering"},
		{From: "beta", To: "alpha", Name: "blocked-by", Slot: "ordering"},
		{From: "gamma", To: "alpha", Name: "caused-by", Slot: "lineage"},
		{From: "alpha", To: "alpha", Name: "relates-to", Slot: "lineage"},
		{From: "delta", To: "delta", Name: "blocks", Slot: "ordering"},
	}
	idx := newIncidentIndex(edges, rel)
	for _, id := range []string{"alpha", "beta", "gamma", "delta", "missing"} {
		got := idx.incident(id)
		want := incidentEdges(edges, rel, id)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: index = %+v, scan = %+v", id, got, want)
		}
	}
}

// A batch with relations must be the very same documents the single-issue view
// prints: the adjacency index must not change order, direction, or list a
// self-relation twice.
func TestPathAllWithRelationsMatchesSingleDocs(t *testing.T) {
	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	commits := []vcs.Commit{
		{ID: "r1", Date: base, Message: "乙起点\n\nissue: beta\naction: ask\ntitle: 乙起点"},
		{ID: "r2", Date: base.Add(time.Hour), Message: "甲起点\n\nissue: alpha\naction: ask\ntitle: 甲起点"},
		{ID: "r3", Date: base.Add(2 * time.Hour), Message: "甲阻塞乙\n\nissue: alpha\naction: collect\ntitle: 甲阻塞乙\nblocks: beta"},
		{ID: "r4", Date: base.Add(3 * time.Hour), Message: "丙自关联并关联甲\n\nissue: gamma\naction: ask\ntitle: 丙\nrelates-to: gamma, alpha"},
	}
	envFor := func(out, errw *bytes.Buffer) *env {
		return &env{actions: model.DefaultActionSet(), repo: fakeRunner{commits: commits}, out: out, errw: errw}
	}

	var out, errb bytes.Buffer
	if code := envFor(&out, &errb).path([]string{"--all", "--json"}); code != 0 {
		t.Fatalf("batch exit = %d, stderr = %q", code, errb.String())
	}
	var batch pathBatchDoc
	if err := json.Unmarshal(out.Bytes(), &batch); err != nil {
		t.Fatalf("batch json: %v\n%s", err, out.String())
	}
	if len(batch.Issues) != 3 {
		t.Fatalf("issues = %d, want 3", len(batch.Issues))
	}

	for _, b := range batch.Issues {
		var single, singleErr bytes.Buffer
		if code := envFor(&single, &singleErr).path([]string{b.Issue, "--json"}); code != 0 {
			t.Fatalf("%s single exit = %d, stderr = %q", b.Issue, code, singleErr.String())
		}
		var doc pathDoc
		if err := json.Unmarshal(single.Bytes(), &doc); err != nil {
			t.Fatalf("%s single json: %v", b.Issue, err)
		}
		if !reflect.DeepEqual(doc, b) {
			t.Fatalf("%s: batch %+v != single %+v", b.Issue, b, doc)
		}
	}

	// gamma relates to itself and to alpha: two edges, the self one exactly once.
	gamma := batch.Issues[2]
	if gamma.Issue != "gamma" || len(gamma.Relations) != 2 {
		t.Fatalf("gamma relations = %+v", gamma.Relations)
	}
}
