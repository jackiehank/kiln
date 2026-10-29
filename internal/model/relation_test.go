package model

import (
	"reflect"
	"strings"
	"testing"
)

// relation.go was the untested half of this package: cli's integration tests
// exercise it hard, but per-package coverage cannot see that, so a change here
// had no local net. These tests are that net — they pin the vocabulary's shape
// (slots, rank order, auto-registered inverses, aliases) rather than any one
// spelling of it.

func TestDefaultRelationSetShape(t *testing.T) {
	rs := DefaultRelationSet()
	want := []string{
		"blocks", "blocked-by",
		"part-of", "has-part",
		"relates-to",
		"caused-by", "causes",
		"supersedes", "superseded-by",
	}
	if got := rs.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestRelationSetResolvesNamesAndAliases(t *testing.T) {
	rs := DefaultRelationSet()
	if got, ok := rs.Resolve("blocks"); !ok || got.Name != "blocks" {
		t.Fatalf("canonical resolve = %q, %v", got.Name, ok)
	}
	// The retired spellings stay readable: they are aliases, not names.
	for alias, canonical := range map[string]string{
		"derived-from": "caused-by",
		"spawns":       "causes",
	} {
		if got, ok := rs.Resolve(alias); !ok || got.Name != canonical {
			t.Fatalf("Resolve(%q) = %q, %v; want %q", alias, got.Name, ok, canonical)
		}
	}
	if _, ok := rs.Resolve("nope"); ok {
		t.Fatal("an unknown key must not resolve")
	}
}

func TestRelationSetInverseAndGloss(t *testing.T) {
	rs := DefaultRelationSet()
	if got := rs.Inverse("blocks"); got != "blocked-by" {
		t.Fatalf("Inverse(blocks) = %q", got)
	}
	// A symmetric name has no opposite, and an unknown one reports the same.
	if got := rs.Inverse("relates-to"); got != "" {
		t.Fatalf("symmetric Inverse = %q", got)
	}
	if got := rs.Inverse("nope"); got != "" {
		t.Fatalf("unknown Inverse = %q", got)
	}
	if got := rs.Gloss("blocks"); got != "我阻塞谁" {
		t.Fatalf("Gloss(blocks) = %q", got)
	}
	// The inverse carries the partner's gloss, auto-registered.
	if got := rs.Gloss("blocked-by"); got != "谁阻塞我" {
		t.Fatalf("Gloss(blocked-by) = %q", got)
	}
	if got := rs.Gloss("relates-to"); got != "无向的关联" {
		t.Fatalf("Gloss(relates-to) = %q", got)
	}
	if got := rs.Gloss("nope"); got != "" {
		t.Fatalf("unknown Gloss = %q", got)
	}
}

func TestRelationSetLookupKeepsTheSlotChain(t *testing.T) {
	rs := DefaultRelationSet()
	part, ok := rs.Lookup("part-of")
	if !ok || part.Slot != "ordering" || part.Rank != 2 || part.Inverse != "has-part" {
		t.Fatalf("part-of = %+v, %v", part, ok)
	}
	// The auto-registered inverse points back and is marked directed.
	inv, ok := rs.Lookup("has-part")
	if !ok || inv.Inverse != "part-of" || !inv.Directed {
		t.Fatalf("has-part = %+v, %v", inv, ok)
	}
	if _, ok := rs.Lookup("nope"); ok {
		t.Fatal("Lookup of an unknown name must fail")
	}
}

func TestRelationSetKeysAndAliases(t *testing.T) {
	rs := DefaultRelationSet()
	keys := rs.Keys()
	for _, k := range rs.Names() {
		if !keys[k] {
			t.Fatalf("Keys() is missing the canonical name %q", k)
		}
	}
	for _, a := range []string{"derived-from", "spawns"} {
		if !keys[a] {
			t.Fatalf("Keys() is missing the alias %q", a)
		}
	}
	if got, want := rs.Aliases(), []string{"derived-from", "spawns"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Aliases() = %v, want %v (sorted)", got, want)
	}
}

func TestRelationSetBlockingSlots(t *testing.T) {
	rs := DefaultRelationSet()
	if !rs.IsBlocking("blocks") {
		t.Fatal("the ordering slot must be blocking")
	}
	if !rs.IsBlocking("blocked-by") {
		t.Fatal("an inverse shares its slot, so it blocks too")
	}
	if rs.IsBlocking("relates-to") || rs.IsBlocking("caused-by") {
		t.Fatal("the lineage slot must not be blocking")
	}
	if rs.IsBlocking("nope") {
		t.Fatal("an unknown name is not blocking")
	}
}

func TestNewRelationSetRejectsBadVocabulary(t *testing.T) {
	slot := []RelationSlot{{Name: "s"}}
	cases := []struct {
		name  string
		slots []RelationSlot
		names []RelationName
		alias map[string]string
		want  string // a fragment that must appear in the error
	}{
		{"empty slot name", []RelationSlot{{Name: ""}}, nil, nil, "name"},
		{"empty relation name", slot, []RelationName{{Name: "", Slot: "s", Rank: 1}}, nil, "name"},
		{"duplicate name", slot, []RelationName{
			{Name: "a", Slot: "s", Rank: 1},
			{Name: "a", Slot: "s", Rank: 2},
		}, nil, "a"},
		{"unknown slot", slot, []RelationName{{Name: "a", Slot: "nope", Rank: 1}}, nil, "nope"},
		{"non-positive rank", slot, []RelationName{{Name: "a", Slot: "s", Rank: 0}}, nil, "a"},
		{"rank clash inside a slot", slot, []RelationName{
			{Name: "a", Slot: "s", Rank: 1},
			{Name: "b", Slot: "s", Rank: 1},
		}, nil, "a"},
		{"alias to an unknown name", slot, []RelationName{{Name: "a", Slot: "s", Rank: 1}},
			map[string]string{"old": "missing"}, "missing"},
	}
	for _, c := range cases {
		_, err := NewRelationSet(c.slots, c.names, c.alias)
		if err == nil {
			t.Fatalf("%s: expected an error", c.name)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: error %q should mention %q", c.name, err, c.want)
		}
	}
}

// A directed pair is one rank, not two: the inverse must not collide with its
// partner, but a third name at the same rank in the same slot must.
func TestRelationSetRankCountsAPairOnce(t *testing.T) {
	if _, err := NewRelationSet(
		[]RelationSlot{{Name: "s"}},
		[]RelationName{{Name: "a", Slot: "s", Rank: 1, Inverse: "b"}},
		nil,
	); err != nil {
		t.Fatalf("a directed pair must build: %v", err)
	}
	_, err := NewRelationSet(
		[]RelationSlot{{Name: "s"}},
		[]RelationName{
			{Name: "a", Slot: "s", Rank: 1, Inverse: "b"},
			{Name: "c", Slot: "s", Rank: 1},
		}, nil)
	if err == nil {
		t.Fatal("a third name at the pair's rank must be refused")
	}
}
