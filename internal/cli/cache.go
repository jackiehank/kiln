package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackiehank/kiln/internal/i18n"
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
var maxLayers = 64

// cacheSite resolves where the cache lives and which vocabulary it belongs to.
// ok is false when the cache is off (config, --no-cache, KILN_NO_CACHE), the
// backend cannot serve it (jj), or the directory cannot be resolved.
func (e *env) cacheSite(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
	if !e.cacheEnabled || e.noCache {
		return nil, "", [20]byte{}, false
	}
	return e.cacheDir(backend)
}

// cacheDir resolves the cache directory and fingerprint without asking whether
// the cache is enabled: gc is also the escape hatch for a directory left behind.
func (e *env) cacheDir(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
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
	e.maybeMerge(cb, dir, fp, len(oids))
	return engine.Build(engine.ActionsFromRecords(records), e.rel()), true
}

// persistRecords writes the records of a full scan. It is the cold path: the
// command already parsed every message, so the layer costs an encode and a write.
func (e *env) persistRecords(backend vcs.Repo, records []cache.Record, tip string) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok || len(records) == 0 {
		return
	}
	e.persist(dir, fp, records, tip)
	e.maybeMerge(cb, dir, fp, len(records))
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
func (e *env) maybeMerge(cb vcs.CacheBackend, dir string, fp [20]byte, visited int) {
	layers, err := cache.Scan(dir)
	if err != nil {
		return
	}
	layers = usable(layers, fp)
	if len(layers) < maxLayers {
		return
	}
	records, tips := cache.Merge(layers)
	// The GC ladder's first rung: a merge that sees more records than the
	// enumeration visited has candidates. Only then is reachability worth its
	// 200 ms. A record outside the reachable set costs nothing but space, so a
	// failed `rev-list --all` simply skips the pruning.
	if visited > 0 && len(records) > visited {
		if reachable, rerr := cb.Reachable(); rerr == nil {
			kept := make([]cache.Record, 0, len(records))
			for _, r := range records {
				if reachable[r.OID] {
					kept = append(kept, r)
				}
			}
			records = kept
			live := make([][20]byte, 0, len(tips))
			for _, tip := range tips {
				if reachable[tip] {
					live = append(live, tip)
				}
			}
			tips = live
		}
	}
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

// actionLogCached prints the whole action stream from the record layer. It
// returns handled=false when the cache cannot serve the request (off, cold,
// damaged), in which case the caller streams from git as before.
//
// Enumeration is streamed, not materialised: git walks and we render in the same
// pass, which is what makes the cache a win here at all. A miss (a commit newer
// than the last layer) is fetched on the spot with one `git cat-file` process —
// rare enough that a batch is not worth losing the overlap.
//
// --limit never comes here: a five-row first screen is cheaper straight from git
// (46 ms) than decoding a layer for it.
func (e *env) actionLogCached(backend vcs.Repo, jsonl bool, scratch *bytes.Buffer, enc *json.Encoder) (int, bool, error) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok {
		return 0, false, nil
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		return 0, false, nil
	}
	layers = usable(layers, fp)
	if len(layers) == 0 {
		return 0, false, nil
	}
	head, _ := cb.HeadOID()
	index := make(map[[20]byte]cache.Record, 0)
	for _, l := range layers {
		for _, r := range l.Records {
			index[r.OID] = r
		}
	}
	emitted := 0
	var fresh []cache.Record
	var writeErr error
	err = cb.EachOIDAuthorDate(func(id string) bool {
		oid, ok := decodeOID(id)
		if !ok {
			return false
		}
		rec, hit := index[oid]
		if !hit {
			raw, ferr := cb.Commits([]string{id})
			if ferr != nil || raw[id].Date.IsZero() {
				return false
			}
			rec = engine.RecordFromCommit(vcs.Commit{ID: id, Date: raw[id].Date, Message: raw[id].Message}, e.rel())
			fresh = append(fresh, rec)
		}
		if !rec.Action {
			return true
		}
		when := time.Unix(rec.Date, 0).In(time.FixedZone("", int(rec.TZ)*60))
		if werr := e.writeActionLogRow(hex.EncodeToString(rec.OID[:]), when, engine.FieldsFromRecord(rec), rec.Subject, jsonl, scratch, enc); werr != nil {
			writeErr = werr
			return false
		}
		emitted++
		return true
	})
	if writeErr != nil {
		return emitted, true, writeErr
	}
	if err != nil {
		return 0, false, nil
	}
	e.persist(dir, fp, fresh, head)
	e.maybeMerge(cb, dir, fp, emitted)
	return emitted, true, nil
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

// cacheCmd is `kiln cache <sync|gc>`: sync prewarms the layer, gc forces a
// reachability clean-up. Reads populate the cache on their own, so neither is
// part of the normal path.
func (e *env) cacheCmd(args []string) int {
	pos, _, missing := parse(args)
	if code := rejectMissingValues(e.errw, missing, allowedFlags()); code != 0 {
		return code
	}
	if len(pos) != 1 || (pos[0] != "sync" && pos[0] != "gc") {
		fmt.Fprint(e.errw, i18n.T("用法: kiln cache <sync|gc>\n"))
		return 2
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	if pos[0] == "sync" {
		return e.cacheSync(backend)
	}
	return e.cacheGC(backend)
}

func (e *env) cacheSync(backend vcs.Repo) int {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok {
		fmt.Fprint(e.errw, i18n.T("缓存未启用：在 .kilnconfig.jsonc 里设置 \"cache\": { \"enabled\": true }\n"))
		return 1
	}
	head, err := cb.HeadOID()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取 HEAD 失败: %v\n", err))
		return 1
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取缓存失败: %v\n", err))
		return 1
	}
	layers = usable(layers, fp)
	if headOID, ok := decodeOID(head); ok && covered(layers, headOID) {
		fmt.Fprintln(e.out, i18n.T("已是最新（无需同步）"))
		return 0
	}
	oids, err := cb.LogOIDs()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取历史失败: %v\n", err))
		return 1
	}
	index := make(map[[20]byte]cache.Record)
	for _, l := range layers {
		for _, r := range l.Records {
			index[r.OID] = r
		}
	}
	var missing []string
	for _, id := range oids {
		oid, ok := decodeOID(id)
		if !ok {
			continue
		}
		if _, hit := index[oid]; !hit {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		fmt.Fprintln(e.out, i18n.T("已是最新（无需同步）"))
		return 0
	}
	fetched, err := cb.Commits(missing)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取提交失败: %v\n", err))
		return 1
	}
	records := make([]cache.Record, 0, len(missing))
	for _, id := range missing {
		raw, ok := fetched[id]
		if !ok {
			continue
		}
		records = append(records, engine.RecordFromCommit(vcs.Commit{ID: id, Date: raw.Date, Message: raw.Message}, e.rel()))
	}
	e.persist(dir, fp, records, head)
	e.maybeMerge(cb, dir, fp, len(oids))
	fmt.Fprintln(e.out, i18n.T("已同步 %d 条提交记录", len(records)))
	return 0
}

// covered reports whether some layer already claims this exact tip: if it does,
// everything reachable from it has a record.
func covered(layers []cache.Layer, tip [20]byte) bool {
	for _, l := range layers {
		for _, t := range l.Tips {
			if t == tip {
				return true
			}
		}
	}
	return false
}

func (e *env) cacheGC(backend vcs.Repo) int {
	cb, dir, fp, ok := e.cacheDir(backend)
	if !ok {
		fmt.Fprint(e.errw, i18n.T("当前后端不支持缓存\n"))
		return 1
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取缓存失败: %v\n", err))
		return 1
	}
	if len(layers) == 0 {
		fmt.Fprintln(e.out, i18n.T("没有可清理的层"))
		return 0
	}
	keep := usable(layers, fp)
	reachable, err := cb.Reachable()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("读取可达集失败: %v\n", err))
		return 1
	}
	records, _ := cache.Merge(keep)
	kept := make([]cache.Record, 0, len(records))
	for _, r := range records {
		if reachable[r.OID] {
			kept = append(kept, r)
		}
	}
	var tips [][20]byte
	for _, l := range keep {
		for _, t := range l.Tips {
			if reachable[t] {
				tips = append(tips, t)
			}
		}
	}
	name, err := cache.Write(dir, kept, tips, fp)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("写入缓存失败: %v\n", err))
		return 1
	}
	removed := 0
	for _, l := range layers {
		if l.Name != name {
			if cache.Remove(l) == nil {
				removed++
			}
		}
	}
	fmt.Fprintln(e.out, i18n.T("已清理：保留 %d 条记录，丢弃 %d 条，层 %d → %d",
		len(kept), len(records)-len(kept), len(layers), len(layers)-removed))
	return 0
}
