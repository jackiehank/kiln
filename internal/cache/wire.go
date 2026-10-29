package cache

import (
	"bytes"
	"encoding/binary"
	"sort"
)

func sortRecords(recs []Record) []Record {
	out := append([]Record(nil), recs...)
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].OID[:], out[j].OID[:]) < 0 })
	return out
}

func sortTips(tips [][20]byte) [][20]byte {
	out := append([][20]byte(nil), tips...)
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i][:], out[j][:]) < 0 })
	uniq := out[:0]
	for _, t := range out {
		if len(uniq) == 0 || !bytes.Equal(uniq[len(uniq)-1][:], t[:]) {
			uniq = append(uniq, t)
		}
	}
	return uniq
}

func putU16(b *bytes.Buffer, v uint16) {
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], v)
	b.Write(tmp[:])
}

func putU32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}

func putI16(b *bytes.Buffer, v int16) {
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], uint16(v))
	b.Write(tmp[:])
}

func putI64(b *bytes.Buffer, v int64) {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], uint64(v))
	b.Write(tmp[:])
}

func putStr(b *bytes.Buffer, s string) {
	putU16(b, uint16(len(s)))
	b.WriteString(s)
}

func putStringIndices(b *bytes.Buffer, ss []string, tab *table) {
	putU16(b, uint16(len(ss)))
	for _, s := range ss {
		putU32(b, tab.idx[s])
	}
}

// reader walks a layer body by offset. Every read reports failure instead of
// panicking or allocating, so a torn or truncated file is rejected at the first
// byte that does not fit — the same contract the old bytes.Reader carried, at a
// third of the allocations.
type reader struct {
	b   []byte
	pos int
}

// take returns the next n bytes, or false when the body ends first.
func (r *reader) take(n int) ([]byte, bool) {
	if n < 0 || r.pos+n > len(r.b) {
		return nil, false
	}
	b := r.b[r.pos : r.pos+n]
	r.pos += n
	return b, true
}

func (r *reader) u16() (uint16, bool) {
	b, ok := r.take(2)
	if !ok {
		return 0, false
	}
	return binary.LittleEndian.Uint16(b), true
}

func (r *reader) u32() (uint32, bool) {
	b, ok := r.take(4)
	if !ok {
		return 0, false
	}
	return binary.LittleEndian.Uint32(b), true
}

// str reads a length-prefixed string with exactly one allocation.
func (r *reader) str() (string, bool) {
	n, ok := r.u16()
	if !ok {
		return "", false
	}
	b, ok := r.take(int(n))
	if !ok {
		return "", false
	}
	return string(b), true
}

// ref reads an index into the shared string table.
func (r *reader) ref(strs []string) (string, bool) {
	i, ok := r.u32()
	if !ok || int(i) >= len(strs) {
		return "", false
	}
	return strs[i], true
}

// refs reads a count followed by that many string-table indices.
func (r *reader) refs(strs []string) ([]string, bool) {
	n, ok := r.u16()
	if !ok {
		return nil, false
	}
	if n == 0 {
		return nil, true
	}
	out := make([]string, 0, n)
	for i := 0; i < int(n); i++ {
		s, ok := r.ref(strs)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// record decodes one record into dst, in place: the caller hands out a slot of
// the output slice, so a record never has to escape to the heap on its own.
func (r *reader) record(dst *Record, strs []string) bool {
	head, ok := r.take(20 + 8 + 2 + 1)
	if !ok {
		return false
	}
	copy(dst.OID[:], head)
	dst.Date = int64(binary.LittleEndian.Uint64(head[20:]))
	dst.TZ = int16(binary.LittleEndian.Uint16(head[28:]))
	flags := head[30]
	dst.Action = flags&1 != 0
	sameSubject := flags&2 != 0
	fields := []*string{&dst.Issue, &dst.Kind, &dst.Title, &dst.Subject, &dst.Domain, &dst.Scope, &dst.Target}
	for _, f := range fields {
		s, ok := r.ref(strs)
		if !ok {
			return false
		}
		*f = s
	}
	if sameSubject {
		dst.Subject = dst.Title
	}
	if dst.Refs, ok = r.refs(strs); !ok {
		return false
	}
	if dst.Tags, ok = r.refs(strs); !ok {
		return false
	}
	n, ok := r.u16()
	if !ok {
		return false
	}
	if n > 0 {
		dst.Relations = make([]Rel, 0, n)
		for i := 0; i < int(n); i++ {
			name, ok := r.ref(strs)
			if !ok {
				return false
			}
			target, ok := r.ref(strs)
			if !ok {
				return false
			}
			dst.Relations = append(dst.Relations, Rel{Name: name, Target: target})
		}
	}
	if dst.Unrelate, ok = r.refs(strs); !ok {
		return false
	}
	if dst.Unknown, ok = r.refs(strs); !ok {
		return false
	}
	return true
}
