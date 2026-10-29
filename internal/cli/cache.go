package cli

import (
	"encoding/hex"
	"path/filepath"

	"github.com/jackiehank/kiln/internal/cache"
	"github.com/jackiehank/kiln/internal/engine"
	"github.com/jackiehank/kiln/internal/model"
	"github.com/jackiehank/kiln/internal/vcs"
)

// recordsDir is the cache subdirectory under the shared git directory.
const recordsDir = "records"

// maxLayers is the point where a read merges the layers into one. It is a knob,
// not a contract: it decides how often the merge runs (and therefore how often
// one read pays for it) and how many files a read opens.
const maxLayers = 64

// cacheSite resolves where the cache lives and which vocabulary it belongs to.
// ok is false when the cache is off (config, --no-cache, KILN_NO_CACHE), the
// backend cannot serve it (jj), or the directory cannot be resolved.
func (e *env) cacheSite(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
	if !e.cacheEnabled || e.noCache {
		return nil, "", [20]byte{}, false
	}
	cb, ok := backend.(vcs.CacheBackend)
	if !ok {
		return nil, "", [20]byte{}, false
	}
	dir, err := cb.CommonDir()
	if err != nil {
		return nil, "", [20]byte{}, false
	}
	return cb, filepath.Join(dir, "kiln", recordsDir), cache.Fingerprint(e.rel()), true
}

// usable keeps the layers this vocabulary can read: intact and fingerprint-matched.
// A layer that does not decode is removed on sight: it can never be used again.
func usable(layers []cache.Layer, fp [20]byte) []cache.Layer {
	var out []cache.Layer
	for _, l := range layers {
		if l.Err != nil {
			_ = cache.Remove(l)
			continue
		}
		if l.FP != fp {
			continue
		}
		out = append(out, l)
	}
	return out
}

// loadCached answers a read from the record layer. It returns ok=false whenever
// the cache cannot serve the whole answer (cold, damaged, disabled, backend
// without records): the caller then falls back to today's full scan, which also
// repopulates the cache.
//
// Nothing here is allowed to change the command's output: records carry the same
// fields the parser derives, and the enumeration order comes from git.
func (e *env) loadCached(backend vcs.Repo) (*model.Repo, bool) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok {
		return nil, false
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		return nil, false
	}
	layers = usable(layers, fp)
	if len(layers) == 0 {
		return nil, false // cold: full scan + persist in the caller
	}
	oids, err := cb.LogOIDs()
	if err != nil || len(oids) == 0 {
		return nil, false
	}
	index := make(map[[20]byte]cache.Record, 0)
	for _, l := range layers {
		for _, r := range l.Records {
			index[r.OID] = r
		}
	}
	var missing []string
	for _, id := range oids {
		oid, ok := decodeOID(id)
		if !ok {
			return nil, false
		}
		if _, hit := index[oid]; !hit {
			missing = append(missing, id)
		}
	}
	var fetched map[string]vcs.RawCommit
	if len(missing) > 0 {
		if fetched, err = cb.Commits(missing); err != nil {
			return nil, false
		}
	}
	records := make([]cache.Record, 0, len(oids))
	var fresh []cache.Record
	for _, id := range oids {
		oid, _ := decodeOID(id)
		if r, hit := index[oid]; hit {
			records = append(records, r)
			continue
		}
		raw, ok := fetched[id]
		if !ok {
			return nil, false
		}
		rec := engine.RecordFromCommit(vcs.Commit{ID: id, Date: raw.Date, Message: raw.Message}, e.rel())
		records = append(records, rec)
		fresh = append(fresh, rec)
	}
	// Persist what this read had to compute anyway; the enumeration covered the
	// whole history, so the layer may claim HEAD.
	e.persist(dir, fp, fresh, oids[0])
	e.maybeMerge(dir, fp)
	return engine.Build(engine.ActionsFromRecords(records), e.rel()), true
}

// persistRecords writes the records of a full scan. It is the cold path: the
// command already parsed every message, so the layer costs an encode and a write.
func (e *env) persistRecords(backend vcs.Repo, records []cache.Record, tip string) {
	_, dir, fp, ok := e.cacheSite(backend)
	if !ok || len(records) == 0 {
		return
	}
	e.persist(dir, fp, records, tip)
	e.maybeMerge(dir, fp)
}

// persist writes one layer and swallows every error: a cache that cannot be
// written must not change what the command prints or what it exits with.
func (e *env) persist(dir string, fp [20]byte, records []cache.Record, tip string) {
	if len(records) == 0 {
		return
	}
	var tips [][20]byte
	if oid, ok := decodeOID(tip); ok {
		tips = append(tips, oid)
	}
	_, _ = cache.Write(dir, records, tips, fp)
}

// maybeMerge folds the layers back into one when the count reaches maxLayers.
// The merge is a local rewrite: no git process, no ordering, and it is idempotent
// because encoding is deterministic.
func (e *env) maybeMerge(dir string, fp [20]byte) {
	layers, err := cache.Scan(dir)
	if err != nil {
		return
	}
	layers = usable(layers, fp)
	if len(layers) < maxLayers {
		return
	}
	records, tips := cache.Merge(layers)
	name, err := cache.Write(dir, records, tips, fp)
	if err != nil {
		return
	}
	for _, l := range layers {
		if l.Name != name {
			_ = cache.Remove(l)
		}
	}
}

// stripNoCache removes the global --no-cache switch from the argument list.
func stripNoCache(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == "--no-cache" {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}

func decodeOID(id string) ([20]byte, bool) {
	var oid [20]byte
	if len(id) != 40 {
		return oid, false
	}
	if _, err := hex.Decode(oid[:], []byte(id)); err != nil {
		return oid, false
	}
	return oid, true
}
