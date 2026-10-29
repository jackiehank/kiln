package cli

import (
	"encoding/hex"
	"os"
	"strings"

	"github.com/jackiehank/kiln/internal/cache"
	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/vcs"
)

// layerChunkRecords is how many records one layer holds when a read seeds the
// cache. It is a knob, not a contract: it trades the number of files (more
// layers mean a more frequent merge) against the encoding peak (a chunk is
// encoded in one buffer). 16384 records is roughly 2 MB on a 100k-commit
// repository, and small repositories still write a single layer.
var layerChunkRecords = 8192

// recPos locates one record: which layer holds it, and where inside that layer.
// Storing a position instead of the record keeps the index small and lets the
// decoded layers stay the only copy of the records.
type recPos struct{ layer, rec int32 }

// reader is the shared read path: the layers this vocabulary can use, an OID
// index over them, and the place records get written back. The model commands
// and action log both answer from it, so "which record does this commit have"
// has one implementation instead of one per command.
type reader struct {
	cb     vcs.CacheBackend
	dir    string
	fp     [20]byte
	layers []cache.Layer
	index  map[[20]byte]recPos
}

// openReader scans the record directory and keeps the layers this vocabulary can
// use. ok=false means the cache cannot answer: the caller scans with git and
// seeds through a layerSink.
func (e *env) openReader(backend vcs.Repo) (*reader, bool) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok {
		return nil, false
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		return nil, false
	}
	e.pruneAged(dir, fp, layers)
	layers = usable(layers, fp)
	if len(layers) == 0 {
		return nil, false // cold: full scan + persist in the caller
	}
	r := &reader{cb: cb, dir: dir, fp: fp, layers: layers, index: make(map[[20]byte]recPos)}
	for li := range layers {
		recs := layers[li].Records
		for ri := range recs {
			if _, dup := r.index[recs[ri].OID]; dup {
				continue
			}
			r.index[recs[ri].OID] = recPos{layer: int32(li), rec: int32(ri)}
		}
	}
	return r, true
}

// missing names the commits the layers do not hold yet, in enumeration order.
func (r *reader) missing(oids [][20]byte) [][20]byte {
	var out [][20]byte
	for _, oid := range oids {
		if _, hit := r.index[oid]; !hit {
			out = append(out, oid)
		}
	}
	return out
}

// fetch reads the messages of the gaps in one process.
func (r *reader) fetch(missing [][20]byte) (map[[20]byte]vcs.RawCommit, bool) {
	if len(missing) == 0 {
		return nil, true
	}
	ids := make([]string, len(missing))
	for i, oid := range missing {
		ids[i] = hex.EncodeToString(oid[:])
	}
	byID, err := r.cb.Commits(ids)
	if err != nil {
		return nil, false
	}
	fetched := make(map[[20]byte]vcs.RawCommit, len(byID))
	for id, raw := range byID {
		if oid, ok := decodeOID(id); ok {
			fetched[oid] = raw
		}
	}
	return fetched, true
}

// recordOf answers one commit: from a layer when it is there, otherwise from
// what git handed over. fresh reports that the record came from git, so a walk
// knows what still has to be written.
func (e *env) recordOf(r *reader, oid [20]byte, fetched map[[20]byte]vcs.RawCommit) (rec cache.Record, fresh, ok bool) {
	if pos, hit := r.index[oid]; hit {
		return r.layers[pos.layer].Records[pos.rec], false, true
	}
	raw, hit := fetched[oid]
	if !hit {
		return cache.Record{}, false, false
	}
	return engine.RecordFromCommit(vcs.Commit{ID: hex.EncodeToString(oid[:]), Date: raw.Date, Message: raw.Message}, e.rel()), true, true
}

// walk hands the records of oids to fn in enumeration order, or backwards when
// back is set (the model builder wants oldest first). Records git had to hand
// over are given to the sink, so the walk that paid for them seeds the cache.
func (e *env) walk(r *reader, oids [][20]byte, fetched map[[20]byte]vcs.RawCommit, back bool, sink *layerSink, fn func(cache.Record) bool) bool {
	for i := range oids {
		oid := oids[i]
		if back {
			oid = oids[len(oids)-1-i]
		}
		rec, fresh, ok := e.recordOf(r, oid, fetched)
		if !ok {
			return false
		}
		if fresh && sink != nil {
			sink.add(rec)
		}
		if !fn(rec) {
			return false
		}
	}
	return true
}

// layerSink accumulates the records a read computed anyway and writes them as
// immutable layers of at most layerChunkRecords records. Chunking is what makes
// "seed while reading" free: the writer never holds more than one chunk, and a
// reader that stops early still leaves the prefix it saw behind (without a
// coverage claim, which a partial walk cannot honestly make).
type layerSink struct {
	dir   string
	fp    [20]byte
	limit int
	buf   []cache.Record
	wrote int
}

func newLayerSink(dir string, fp [20]byte) *layerSink {
	return &layerSink{dir: dir, fp: fp, limit: layerChunkRecords}
}

// add buffers one record, flushing a full chunk. A zero limit means the whole
// buffer is written by flush.
func (s *layerSink) add(rec cache.Record) {
	s.buf = append(s.buf, rec)
	if s.limit > 0 && len(s.buf) >= s.limit {
		s.flush(nil)
	}
}

// flush writes what is buffered and returns how many layers it wrote. Every
// error is swallowed: a cache that cannot be written must not change what the
// command prints or what it exits with. tips is the coverage claim for this
// chunk; a partial read passes nil.
func (s *layerSink) flush(tips [][20]byte) int {
	if len(s.buf) == 0 {
		return 0
	}
	recs := s.buf
	s.buf = make([]cache.Record, 0, s.limit)
	if _, err := cache.Write(s.dir, recs, tips, s.fp); err != nil {
		return 0
	}
	s.wrote++
	return 1
}

func (s *layerSink) count() int { return s.wrote }

// layerFileCount counts layer files without decoding them, so the merge ladder
// can decide whether the (expensive) scan is worth it.
func layerFileCount(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, cache.FilePrefix) && strings.HasSuffix(name, cache.FileSuffix) {
			n++
		}
	}
	return n
}

// settleRead is the last step of a read: write what the walk paid for, then
// merge when the ladder says so (or immediately, when the whole history had to
// be re-read because it was rewritten).
func (e *env) settleRead(cb vcs.CacheBackend, dir string, fp [20]byte, sink *layerSink, tips [][20]byte, visited int, rewritten bool) {
	sink.flush(tips)
	if !rewritten && layerFileCount(dir) < maxLayers {
		return
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		return
	}
	layers = usable(layers, fp)
	if len(layers) == 0 {
		return
	}
	e.mergeLayers(cb, layers, dir, fp, visited)
}

// maybeMergeDir merges when the directory holds enough layers; it re-scans only
// at that point, so a read that wrote a chunk does not pay for decoding the
// layers it just wrote.
func (e *env) maybeMergeDir(cb vcs.CacheBackend, dir string, fp [20]byte, visited int) {
	if layerFileCount(dir) < maxLayers {
		return
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		return
	}
	layers = usable(layers, fp)
	if len(layers) == 0 {
		return
	}
	e.mergeLayers(cb, layers, dir, fp, visited)
}
