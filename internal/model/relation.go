package model

import (
	"fmt"
	"sort"
)

// Rel is one relation declaration: a canonical relation name and its target
// issue. The name is always canonical (aliases are resolved at parse time).
type Rel struct {
	Name   string
	Target string
}

// RelationSlot is one structural slot. Blocking slots carry an ordering and must
// stay acyclic; names inside a slot form a strength chain (rank).
type RelationSlot struct {
	Name     string
	Blocking bool
}

// RelationName is one writable relation name. A name with an Inverse is
// directed; the inverse is auto-registered as a writable name too, so both sides
// can be declared.
type RelationName struct {
	Name     string
	Slot     string
	Rank     int
	Directed bool
	Inverse  string

	// Gloss is the one-line meaning of the name. `kiln path` attaches it to an
	// inverse name the first time it shows, and `-h` prints it beside the flag.
	Gloss string
	// InverseGloss is the gloss handed to the auto-registered inverse name.
	InverseGloss string
}

// RelationSet is the relation vocabulary. The structure — slots and the
// strength order inside each — is owned here and not configurable; only the
// names are (see config). Aliases keep legacy names readable.
type RelationSet struct {
	slots  map[string]RelationSlot
	names  map[string]RelationName
	order  []string
	alias  map[string]string
	bySlot map[string][]string
}

// NewRelationSet builds a vocabulary. Each directed name with an Inverse spawns
// its inverse entry automatically (same slot and rank, inverse pointing back).
// Within a slot every rank must be unique: the slot is a strength chain, so two
// incomparable names belong in different slots.
func NewRelationSet(slots []RelationSlot, names []RelationName, aliases map[string]string) (*RelationSet, error) {
	rs := &RelationSet{
		slots:  map[string]RelationSlot{},
		names:  map[string]RelationName{},
		alias:  map[string]string{},
		bySlot: map[string][]string{},
	}
	for _, s := range slots {
		if s.Name == "" {
			return nil, fmt.Errorf("relation slot 缺 name")
		}
		rs.slots[s.Name] = s
	}
	add := func(n RelationName) error {
		if n.Name == "" {
			return fmt.Errorf("relation name 为空")
		}
		if _, ok := rs.names[n.Name]; ok {
			return fmt.Errorf("relation 重名: %s", n.Name)
		}
		if _, ok := rs.slots[n.Slot]; !ok {
			return fmt.Errorf("relation %s 的 slot 不存在: %s", n.Name, n.Slot)
		}
		if n.Rank <= 0 {
			return fmt.Errorf("relation %s 的 rank 必须为正", n.Name)
		}
		rs.names[n.Name] = n
		rs.order = append(rs.order, n.Name)
		rs.bySlot[n.Slot] = append(rs.bySlot[n.Slot], n.Name)
		return nil
	}
	for _, n := range names {
		if err := add(n); err != nil {
			return nil, err
		}
		if n.Inverse != "" {
			if _, ok := rs.names[n.Inverse]; !ok {
				inv := RelationName{Name: n.Inverse, Slot: n.Slot, Rank: n.Rank, Directed: true, Inverse: n.Name, Gloss: n.InverseGloss}
				if err := add(inv); err != nil {
					return nil, err
				}
			}
		}
	}
	for a, c := range aliases {
		if _, ok := rs.names[c]; !ok {
			return nil, fmt.Errorf("alias %s 指向未知的 relation %s", a, c)
		}
		rs.alias[a] = c
	}
	// Within a slot, ranks must be unique across *families* (a directed pair and
	// its inverse share a rank, so count each pair once).
	seen := map[string]map[int]string{}
	for _, n := range rs.names {
		if n.Inverse != "" && n.Name > n.Inverse {
			continue // the reverse view of a pair already counted
		}
		if seen[n.Slot] == nil {
			seen[n.Slot] = map[int]string{}
		}
		if other, ok := seen[n.Slot][n.Rank]; ok {
			return nil, fmt.Errorf("slot %s 内 rank %d 被 %s 与 %s 共用（槽内全序）", n.Slot, n.Rank, other, n.Name)
		}
		seen[n.Slot][n.Rank] = n.Name
	}
	for s := range rs.bySlot {
		sort.Slice(rs.bySlot[s], func(i, j int) bool { return rs.names[rs.bySlot[s][i]].Rank < rs.names[rs.bySlot[s][j]].Rank })
	}
	return rs, nil
}

// DefaultRelationSet is kiln's built-in vocabulary, matching relation-vocabulary:
// 4 directed pairs + 1 symmetric name, plus aliases for the retired names.
func DefaultRelationSet() *RelationSet {
	rs, err := NewRelationSet(
		[]RelationSlot{{Name: "ordering", Blocking: true}, {Name: "lineage", Blocking: false}},
		[]RelationName{
			{Name: "blocks", Slot: "ordering", Rank: 1, Inverse: "blocked-by", Gloss: "我阻塞谁", InverseGloss: "谁阻塞我"},
			{Name: "part-of", Slot: "ordering", Rank: 2, Inverse: "has-part", Gloss: "我属于谁", InverseGloss: "谁属于我"},
			{Name: "relates-to", Slot: "lineage", Rank: 1, Gloss: "无向的关联"},
			{Name: "caused-by", Slot: "lineage", Rank: 2, Inverse: "causes", Gloss: "谁引发了我", InverseGloss: "我引发了谁"},
			{Name: "supersedes", Slot: "lineage", Rank: 3, Inverse: "superseded-by", Gloss: "我取代谁", InverseGloss: "谁取代了我"},
		},
		map[string]string{"derived-from": "caused-by", "spawns": "causes"},
	)
	if err != nil {
		panic(err)
	}
	return rs
}

// Resolve maps a written key to its canonical name, following aliases.
func (rs *RelationSet) Resolve(key string) (RelationName, bool) {
	if c, ok := rs.alias[key]; ok {
		key = c
	}
	n, ok := rs.names[key]
	return n, ok
}

// Lookup returns the canonical name's definition.
func (rs *RelationSet) Lookup(name string) (RelationName, bool) {
	n, ok := rs.names[name]
	return n, ok
}

// Inverse returns the opposite name for a directed relation, or "" when the
// name is symmetric or unknown.
func (rs *RelationSet) Inverse(name string) string {
	if n, ok := rs.names[name]; ok {
		return n.Inverse
	}
	return ""
}

// Gloss returns the one-line meaning of a canonical name, or "" when the name is
// unknown or carries none.
func (rs *RelationSet) Gloss(name string) string {
	if n, ok := rs.names[name]; ok {
		return n.Gloss
	}
	return ""
}

// Names lists the canonical writable names in declaration order.
func (rs *RelationSet) Names() []string {
	out := append([]string(nil), rs.order...)
	return out
}

// Keys lists every accepted trailer key: canonical names plus aliases. Used by
// the trailer parser so legacy names still read.
func (rs *RelationSet) Keys() map[string]bool {
	out := map[string]bool{}
	for k := range rs.names {
		out[k] = true
	}
	for k := range rs.alias {
		out[k] = true
	}
	return out
}

// Aliases lists the read-compat aliases in sorted order.
func (rs *RelationSet) Aliases() []string {
	out := make([]string, 0, len(rs.alias))
	for k := range rs.alias {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsBlocking reports whether the name's slot carries an ordering (must be
// acyclic).
func (rs *RelationSet) IsBlocking(name string) bool {
	if n, ok := rs.names[name]; ok {
		return rs.slots[n.Slot].Blocking
	}
	return false
}
