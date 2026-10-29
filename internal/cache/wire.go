package cache

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
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

func readU16(r *bytes.Reader) (uint16, error) {
	var tmp [2]byte
	if _, err := io.ReadFull(r, tmp[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(tmp[:]), nil
}

func readU32(r *bytes.Reader) (uint32, error) {
	var tmp [4]byte
	if _, err := io.ReadFull(r, tmp[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(tmp[:]), nil
}

func readStr(r *bytes.Reader) (string, error) {
	n, err := readU16(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readIndex(r *bytes.Reader, strs []string) (string, error) {
	i, err := readU32(r)
	if err != nil {
		return "", err
	}
	if int(i) >= len(strs) {
		return "", errors.New("layer string index out of range")
	}
	return strs[i], nil
}

func readIndices(r *bytes.Reader, strs []string) ([]string, error) {
	n, err := readU16(r)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]string, n)
	for i := range out {
		if out[i], err = readIndex(r, strs); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readRecord(r *bytes.Reader, strs []string) (Record, error) {
	var rec Record
	if _, err := io.ReadFull(r, rec.OID[:]); err != nil {
		return rec, err
	}
	var tmp8 [8]byte
	if _, err := io.ReadFull(r, tmp8[:]); err != nil {
		return rec, err
	}
	rec.Date = int64(binary.LittleEndian.Uint64(tmp8[:]))
	var tmp2 [2]byte
	if _, err := io.ReadFull(r, tmp2[:]); err != nil {
		return rec, err
	}
	rec.TZ = int16(binary.LittleEndian.Uint16(tmp2[:]))
	flags, err := r.ReadByte()
	if err != nil {
		return rec, err
	}
	rec.Action = flags&1 != 0
	sameSubject := flags&2 != 0

	fields := []*string{&rec.Issue, &rec.Kind, &rec.Title, &rec.Subject, &rec.Domain, &rec.Scope, &rec.Target}
	for _, f := range fields {
		if *f, err = readIndex(r, strs); err != nil {
			return rec, err
		}
	}
	if sameSubject {
		rec.Subject = rec.Title
	}
	if rec.Refs, err = readIndices(r, strs); err != nil {
		return rec, err
	}
	if rec.Tags, err = readIndices(r, strs); err != nil {
		return rec, err
	}
	n, err := readU16(r)
	if err != nil {
		return rec, err
	}
	for i := uint16(0); i < n; i++ {
		name, err := readIndex(r, strs)
		if err != nil {
			return rec, err
		}
		target, err := readIndex(r, strs)
		if err != nil {
			return rec, err
		}
		rec.Relations = append(rec.Relations, Rel{Name: name, Target: target})
	}
	if rec.Unrelate, err = readIndices(r, strs); err != nil {
		return rec, err
	}
	if rec.Unknown, err = readIndices(r, strs); err != nil {
		return rec, err
	}
	return rec, nil
}
