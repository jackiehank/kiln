package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
// The cache is background behaviour: it is on unless --no-cache (or
// KILN_NO_CACHE) turns it off, and it stays off for a backend that cannot serve
// it (jj) or a directory that cannot be resolved.
func (e *env) cacheSite(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
	if e.noCache {
		return nil, "", [20]byte{}, false
	}
	return e.cacheDir(backend)
}

// cacheDir resolves the cache directory and fingerprint without checking
// --no-cache: gc is the explicit escape hatch and must work regardless.
func (e *env) cacheDir(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
	cb, ok := backend.(vcs.CacheBackend)
	if !ok {
		return nil, "", [20]byte{}, false
	}
	fp := cache.Fingerprint(e.rel())
	// Resolving the shared git dir from the filesystem costs nothing; asking git
	// costs a process (~45 ms on Windows) on every read, which is the whole
	// budget of a small repository. Fall back to git for the shapes the walk
	// cannot read (bare repository, GIT_DIR pointing elsewhere).
	if dir, ok := commonDirFromFS(e.dir); ok {
		return cb, filepath.Join(dir, "kiln", recordsDir), fp, true
	}
	dir, err := cb.CommonDir()
	if err != nil {
		return nil, "", [20]byte{}, false
	}
	return cb, filepath.Join(dir, "kiln", recordsDir), fp, true
}

// commonDirFromFS finds the shared git directory by walking up from dir, without
// spawning git. A plain checkout has `.git/`; a linked worktree has a `.git` file
// naming its gitdir, which holds a `commondir` file pointing back at the shared
// directory. Anything else is left to git.
func commonDirFromFS(dir string) (string, bool) {
	start, err := filepath.Abs(dir)
	if err != nil {
		start = filepath.Clean(dir)
	}
	for d := start; ; {
		gitPath := filepath.Join(d, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				return gitPath, true
			}
			data, rerr := os.ReadFile(gitPath)
			if rerr != nil {
				return "", false
			}
			line := strings.TrimSpace(string(data))
			const prefix = "gitdir:"
			if !strings.HasPrefix(line, prefix) {
				return "", false
			}
			gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(d, gitDir)
			}
			if data, rerr := os.ReadFile(filepath.Join(gitDir, "commondir")); rerr == nil {
				common := strings.TrimSpace(string(data))
				if !filepath.IsAbs(common) {
					common = filepath.Join(gitDir, common)
				}
				return filepath.Clean(common), true
			}
			return "", false
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
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
	// The enumeration is the one thing the record layer cannot answer: it comes
	// from a snapshot of git's own output, so a hit costs a file read and zero
	// processes. A miss runs git once and stores what it printed.
	oids, ok := e.enumerateDefault(cb)
	if !ok || len(oids) == 0 {
		return nil, false
	}
	// The record count is the map's size hint: without one, 100k [20]byte keys
	// grow the map through every power of two and copy the whole table each time.
	index := make(map[[20]byte]cache.Record, len(oids))
	for _, l := range layers {
		for _, r := range l.Records {
			index[r.OID] = r
		}
	}
	var missing [][20]byte
	for _, oid := range oids {
		if _, hit := index[oid]; !hit {
			missing = append(missing, oid)
		}
	}
	var fetched map[[20]byte]vcs.RawCommit
	if len(missing) > 0 {
		ids := make([]string, len(missing))
		for i, oid := range missing {
			ids[i] = hex.EncodeToString(oid[:])
		}
		byID, err := cb.Commits(ids)
		if err != nil {
			return nil, false
		}
		fetched = make(map[[20]byte]vcs.RawCommit, len(byID))
		for id, raw := range byID {
			if oid, ok := decodeOID(id); ok {
				fetched[oid] = raw
			}
		}
	}
	records := make([]cache.Record, 0, len(oids))
	var fresh []cache.Record
	for _, oid := range oids {
		if r, hit := index[oid]; hit {
			records = append(records, r)
			continue
		}
		raw, ok := fetched[oid]
		if !ok {
			return nil, false
		}
		rec := engine.RecordFromCommit(vcs.Commit{ID: hex.EncodeToString(oid[:]), Date: raw.Date, Message: raw.Message}, e.rel())
		records = append(records, rec)
		fresh = append(fresh, rec)
	}
	// Persist what this read had to compute anyway; the enumeration covered the
	// whole history, so the layer may claim HEAD. The layer just written joins the
	// set in memory: re-reading the directory to find it is what made every read
	// decode the same bytes twice.
	if l := e.persist(dir, fp, fresh, oids[0]); l != nil {
		layers = append(layers, *l)
	}
	if len(fresh) > 0 && len(fresh) == len(oids) {
		// Every enumerated commit missed while layers exist: the history was
		// rewritten (rebase / amend). Merge (and prune) now instead of waiting for
		// the layer count to reach maxLayers.
		if len(layers) > 1 {
			e.mergeLayers(cb, layers, dir, fp, len(oids))
			return engine.Build(engine.ActionsFromRecords(records), e.rel()), true
		}
	}
	e.maybeMerge(cb, layers, dir, fp, len(oids))
	return engine.Build(engine.ActionsFromRecords(records), e.rel()), true
}

// persistRecords writes the records of a full scan. It is the cold path: the
// command already parsed every message, so the layer costs an encode and a write.
func (e *env) persistRecords(backend vcs.Repo, records []cache.Record, tip [20]byte) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok || len(records) == 0 {
		return
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		layers = nil
	}
	layers = usable(layers, fp)
	if l := e.persist(dir, fp, records, tip); l != nil {
		layers = append(layers, *l)
	}
	e.maybeMerge(cb, layers, dir, fp, len(records))
}

// persist writes one layer and returns it, so a caller that just wrote a layer
// can count it without re-reading the directory. Every error is swallowed: a
// cache that cannot be written must not change what the command prints or what
// it exits with.
func (e *env) persist(dir string, fp [20]byte, records []cache.Record, tip [20]byte) *cache.Layer {
	if len(records) == 0 {
		return nil
	}
	tips := [][20]byte{tip}
	name, err := cache.Write(dir, records, tips, fp)
	if err != nil || name == "" {
		return nil
	}
	return &cache.Layer{
		Name:    name,
		Path:    filepath.Join(dir, name),
		Records: records,
		Tips:    tips,
		FP:      fp,
	}
}

// maybeMerge folds the layers back into one when the count reaches maxLayers.
// The caller passes the layers it already holds — reading the directory again
// here would decode every layer a second time in the same command.
func (e *env) maybeMerge(cb vcs.CacheBackend, layers []cache.Layer, dir string, fp [20]byte, visited int) {
	if len(layers) < maxLayers {
		return
	}
	e.mergeLayers(cb, layers, dir, fp, visited)
}

// mergeLayers rewrites the given layers into one, pruning records no ref can
// reach when the counts say there may be some.
func (e *env) mergeLayers(cb vcs.CacheBackend, layers []cache.Layer, dir string, fp [20]byte, visited int) {
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
	cached := 0
	for _, l := range layers {
		cached += len(l.Records)
	}
	index := make(map[[20]byte]cache.Record, cached)
	for _, l := range layers {
		for _, r := range l.Records {
			index[r.OID] = r
		}
	}
	emitted := 0
	var fresh []cache.Record
	oids, ok := e.enumerateAuthorDate(cb)
	if !ok || len(oids) == 0 {
		return 0, false, nil
	}
	for _, oid := range oids {
		rec, hit := index[oid]
		if !hit {
			id := hex.EncodeToString(oid[:])
			raw, ferr := cb.Commits([]string{id})
			if ferr != nil || raw[id].Date.IsZero() {
				return 0, false, nil
			}
			rec = engine.RecordFromCommit(vcs.Commit{ID: id, Date: raw[id].Date, Message: raw[id].Message}, e.rel())
			fresh = append(fresh, rec)
		}
		if !rec.Action {
			continue
		}
		when := time.Unix(rec.Date, 0).In(time.FixedZone("", int(rec.TZ)*60))
		if werr := e.writeActionLogRow(hex.EncodeToString(rec.OID[:]), when, engine.FieldsFromRecord(rec), rec.Subject, jsonl, scratch, enc); werr != nil {
			return emitted, true, werr
		}
		emitted++
	}
	// The first enumerated commit is the newest tip of HEAD's history; claiming it
	// covers a subset of what HEAD covers, which is all the records prove.
	if l := e.persist(dir, fp, fresh, oids[0]); l != nil {
		layers = append(layers, *l)
	}
	e.maybeMerge(cb, layers, dir, fp, emitted)
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

// cacheCmd is `kiln cache gc`: the explicit escape hatch. Reads write the cache,
// merge it and prune unreachable records on their own, so the only job left for a
// command is forcing a full reachability clean-up (old records, foreign-fingerprint
// layers, broken files).
func (e *env) cacheCmd(args []string) int {
	pos, _, missing := parse(args)
	if code := rejectMissingValues(e.errw, missing, allowedFlags()); code != 0 {
		return code
	}
	if len(pos) != 1 || pos[0] != "gc" {
		fmt.Fprint(e.errw, i18n.T("用法: kiln cache gc\n"))
		return 2
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	return e.cacheGC(backend)
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
	// Snapshots are keyed by HEAD, so one per commit piles up; the write path keeps
	// the newest few, and a forced gc drops the ones no ref can reach any more.
	gcEnums(dir, reachable)
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
