// Package cache stores the parsed form of every commit in an immutable,
// content-addressed layer file, so a read command can answer from it instead of
// walking every commit message again.
//
// A layer is a pure function of the records it holds and the tips it covers:
// records are sorted by OID and the string table by its bytes, so the same input
// always produces the same bytes and therefore the same file name. That is what
// makes concurrent writers idempotent and lets a merge be a plain union.
package cache

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackiehank/kiln/internal/model"
)

// Magic and Layout identify the on-disk format. Changing either means a new
// Magic: old layers are then ignored rather than migrated.
const (
	Magic  = "KILNREC1"
	Layout = 1

	headerFixed = 8 + 2 + 20 + 2 // magic, layout, vocabulary fingerprint, tip count
	trailerSize = sha1.Size
	maxTips     = 1 << 14
	maxStrings  = 1 << 24
)

// Rel is one relation declaration as it was parsed and canonicalised.
type Rel struct {
	Name   string
	Target string
}

// Record is the parsed content of one commit.
type Record struct {
	OID    [20]byte
	Date   int64 // author date, unix seconds
	TZ     int16 // author timezone, minutes east of UTC
	Action bool  // the commit carries both `issue:` and `action:`

	Issue   string
	Kind    string
	Title   string
	Subject string
	Domain  string
	Scope   string
	Target  string

	Refs      []string
	Tags      []string
	Unrelate  []string
	Unknown   []string
	Relations []Rel
}

// Buffer helpers: every string in a layer goes through the shared table, so a
// record only stores indices.
type table struct {
	idx  map[string]uint32
	list []string
}

func (t *table) add(s string) {
	if _, ok := t.idx[s]; ok {
		return
	}
	t.idx[s] = 0 // placeholder; indices are assigned after sorting
	t.list = append(t.list, s)
}

func (t *table) freeze() {
	sort.Strings(t.list)
	for i, s := range t.list {
		t.idx[s] = uint32(i)
	}
}

// Encode serialises records into one layer file. Records are sorted by OID so
// the encoding does not depend on the caller's order.
func Encode(records []Record, tips [][20]byte, fingerprint [20]byte) []byte {
	recs := sortRecords(records)
	tab := &table{idx: map[string]uint32{}}
	for _, r := range recs {
		tab.add(r.Issue)
		tab.add(r.Kind)
		tab.add(r.Title)
		tab.add(r.Subject)
		tab.add(r.Domain)
		tab.add(r.Scope)
		tab.add(r.Target)
		for _, s := range r.Refs {
			tab.add(s)
		}
		for _, s := range r.Tags {
			tab.add(s)
		}
		for _, s := range r.Unrelate {
			tab.add(s)
		}
		for _, s := range r.Unknown {
			tab.add(s)
		}
		for _, rel := range r.Relations {
			tab.add(rel.Name)
			tab.add(rel.Target)
		}
	}
	tab.freeze()

	sortedTips := sortTips(tips)
	var b bytes.Buffer
	b.WriteString(Magic)
	putU16(&b, uint16(Layout))
	b.Write(fingerprint[:])
	putU16(&b, uint16(len(sortedTips)))
	for _, tip := range sortedTips {
		b.Write(tip[:])
	}
	putU32(&b, uint32(len(tab.list)))
	for _, s := range tab.list {
		putStr(&b, s)
	}
	putU32(&b, uint32(len(recs)))
	for _, r := range recs {
		b.Write(r.OID[:])
		putI64(&b, r.Date)
		putI16(&b, r.TZ)
		flags := byte(0)
		if r.Action {
			flags |= 1
		}
		if r.Subject == r.Title {
			flags |= 2
		}
		b.WriteByte(flags)
		for _, s := range []string{r.Issue, r.Kind, r.Title, r.Subject, r.Domain, r.Scope, r.Target} {
			putU32(&b, tab.idx[s])
		}
		putStringIndices(&b, r.Refs, tab)
		putStringIndices(&b, r.Tags, tab)
		putU16(&b, uint16(len(r.Relations)))
		for _, rel := range r.Relations {
			putU32(&b, tab.idx[rel.Name])
			putU32(&b, tab.idx[rel.Target])
		}
		putStringIndices(&b, r.Unrelate, tab)
		putStringIndices(&b, r.Unknown, tab)
	}
	sum := sha1.Sum(b.Bytes())
	b.Write(sum[:])
	return b.Bytes()
}

// Decode parses a layer and verifies its trailer. Any inconsistency — wrong
// magic, wrong layout, truncated body, mismatched checksum — is an error: the
// caller ignores the layer rather than serving part of it.
func Decode(data []byte) ([]Record, [][20]byte, [20]byte, error) {
	recs, tips, fp, _, err := DecodeWithSum(data)
	return recs, tips, fp, err
}

// DecodeWithSum is Decode plus the body digest the trailer carried. Scan needs
// that digest to check the file name, and hashing a multi-megabyte layer twice
// per read is pure waste: the value is exactly what Name would hash again.
func DecodeWithSum(data []byte) ([]Record, [][20]byte, [20]byte, [20]byte, error) {
	var zero, sum [20]byte
	if len(data) < headerFixed+trailerSize {
		return nil, nil, zero, zero, errors.New("layer too short")
	}
	body := data[:len(data)-trailerSize]
	sum = sha1.Sum(body)
	if !bytes.Equal(sum[:], data[len(data)-trailerSize:]) {
		return nil, nil, zero, zero, errors.New("layer checksum mismatch")
	}
	if string(body[:8]) != Magic {
		return nil, nil, zero, zero, fmt.Errorf("layer magic %q, want %q", body[:8], Magic)
	}
	r := &reader{b: body, pos: 8}
	layout, ok := r.u16()
	if !ok {
		return nil, nil, zero, zero, errors.New("layer truncated in header")
	}
	if layout != Layout {
		return nil, nil, zero, zero, fmt.Errorf("layer layout %d, want %d", layout, Layout)
	}
	var fp [20]byte
	rawFP, ok := r.take(20)
	if !ok {
		return nil, nil, zero, zero, errors.New("layer truncated in header")
	}
	copy(fp[:], rawFP)
	tipCount, ok := r.u16()
	if !ok {
		return nil, nil, zero, zero, errors.New("layer truncated in header")
	}
	if int(tipCount) > maxTips || int(tipCount) > len(body)/20 {
		return nil, nil, zero, zero, errors.New("layer tip count out of range")
	}
	tips := make([][20]byte, tipCount)
	for i := range tips {
		b, ok := r.take(20)
		if !ok {
			return nil, nil, zero, zero, errors.New("layer truncated in tips")
		}
		copy(tips[i][:], b)
	}
	strCount, ok := r.u32()
	if !ok {
		return nil, nil, zero, zero, errors.New("layer truncated before string table")
	}
	if strCount > maxStrings || int(strCount) > len(body) {
		return nil, nil, zero, zero, errors.New("layer string count out of range")
	}
	strs := make([]string, strCount)
	for i := range strs {
		s, ok := r.str()
		if !ok {
			return nil, nil, zero, zero, errors.New("layer truncated in string table")
		}
		strs[i] = s
	}
	recCount, ok := r.u32()
	if !ok {
		return nil, nil, zero, zero, errors.New("layer truncated before records")
	}
	// A record is at least its 20-byte id, so this bounds the allocation a
	// corrupt count could ask for without rejecting any real layer.
	if recCount > uint32(len(body)/20) {
		return nil, nil, zero, zero, errors.New("layer record count out of range")
	}
	recs := make([]Record, recCount)
	for i := range recs {
		if !r.record(&recs[i], strs) {
			return nil, nil, zero, zero, errors.New("layer truncated in record")
		}
	}
	if r.pos != len(body) {
		return nil, nil, zero, zero, errors.New("layer has trailing bytes")
	}
	return recs, tips, fp, sum, nil
}

// Fingerprint identifies the vocabulary a layer was parsed with. A layer whose
// fingerprint differs is ignored: relation names, slots and ranks decide how a
// declaration folds, so the parse result is only valid under the same vocabulary.
func Fingerprint(rel *model.RelationSet) [20]byte {
	keys := make([]string, 0, len(rel.Keys()))
	for k := range rel.Keys() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		name, ok := rel.Resolve(k)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%s\x00%s\x00%s\x00%d\x00%t\x00%s\x00%t\n",
			k, name.Name, name.Slot, name.Rank, name.Directed, name.Inverse, rel.IsBlocking(name.Name))
	}
	return sha1.Sum([]byte(b.String()))
}
