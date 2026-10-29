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

// foreignGrace is how long a layer written by another vocabulary survives before
// a read deletes it. Two kiln versions on one clone are rare but real (a pinned
// CI binary next to the repo build); a week lets them coexist, and a downgrade
// costs one full scan to rebuild.
const foreignGrace = 7 * 24 * time.Hour

// cacheOff turns the record cache off for every read. It exists for the test
// suite, whose oracle is the cold path, and is deliberately not reachable from
// argv or the environment: a user repairs a cache with `kiln cache clear`, not
// by running the tool uncached.
var cacheOff bool

// cacheSite resolves where the cache lives and which vocabulary it belongs to.
// The cache is background behaviour: it is on unless cacheOff says otherwise,
// and it stays off for a backend that cannot serve it (jj) or a directory that
// cannot be resolved.
func (e *env) cacheSite(backend vcs.Repo) (vcs.CacheBackend, string, [20]byte, bool) {
	if cacheOff {
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

// pruneAged drops leftovers no live reader can use: temp files a crashed writer
// left behind, and layers from another vocabulary once they have aged out.
// Deletion is always safe — the losing side rebuilds with one full scan — and it
// never touches a readable layer of this vocabulary, however old.
func (e *env) pruneAged(dir string, fp [20]byte, layers []cache.Layer) {
	now := time.Now()
	for _, l := range layers {
		if l.FP != fp && now.Sub(l.ModTime) > foreignGrace {
			_ = cache.Remove(l)
		}
	}
	cache.RemoveStaleTemps(dir, now)
}

// loadCached answers a read from the record layer. It returns ok=false whenever
// the cache cannot serve the whole answer (cold, damaged, disabled, backend
// without records): the caller then falls back to today's full scan, which also
// repopulates the cache.
//
// Nothing here is allowed to change the command's output: records carry the same
// fields the parser derives, and the enumeration order comes from git.
func (e *env) loadCached(backend vcs.Repo) (*model.Repo, bool) {
	r, ok := e.openReader(backend)
	if !ok {
		return nil, false
	}
	oids, ok := e.enumerateOIDs(r.cb)
	if !ok {
		return nil, false
	}
	fetched, ok := r.fetch(r.missing(oids))
	if !ok {
		return nil, false
	}
	sink := newLayerSink(r.dir, r.fp)
	b := engine.NewBuilder(e.rel())
	// Oldest first, the order the builder wants; the last chunk written then
	// holds the newest records, which is where the coverage claim belongs.
	if !e.walk(r, oids, fetched, true, sink, func(rec cache.Record) bool {
		if rec.Action {
			b.Add(engine.ActionFromRecord(rec))
		}
		return true
	}) {
		return nil, false
	}
	// Every enumerated commit missed while layers exist: the history was
	// rewritten (rebase / amend). Merge now instead of waiting for the ladder.
	rewritten := len(fetched) == len(oids)
	e.settleRead(r.cb, r.dir, r.fp, sink, [][20]byte{oids[0]}, len(oids), rewritten)
	return b.Repo(), true
}

// enumerateOIDs answers "which commits, in git's order": the one thing the record
// layer cannot answer. It comes from a snapshot of git's own output, so a hit
// costs a file read and one git process fewer.
func (e *env) enumerateOIDs(cb vcs.CacheBackend) ([][20]byte, bool) {
	oids, ok := e.enumerateDefault(cb)
	if !ok || len(oids) == 0 {
		return nil, false
	}
	return oids, true
}

// persistRecords writes the records of a full scan. It is the cold path: the
// command already parsed every message, so a layer costs an encode and a write.
// The records are written in chunks (see layerSink), which keeps the encoding
// peak to one chunk instead of the whole history.
func (e *env) persistRecords(backend vcs.Repo, records []cache.Record, tip [20]byte) {
	cb, dir, fp, ok := e.cacheSite(backend)
	if !ok || len(records) == 0 {
		return
	}
	layers, err := cache.Scan(dir)
	if err != nil {
		layers = nil
	}
	e.pruneAged(dir, fp, layers)
	layers = usable(layers, fp)
	sink := newLayerSink(dir, fp)
	for i := range records {
		sink.add(records[i])
	}
	sink.flush([][20]byte{tip})
	e.maybeMergeDir(cb, dir, fp, len(records)+len(layers))
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
			// The reachable set was just paid for, so the snapshots get the same
			// treatment: a snapshot keyed by a HEAD no ref reaches can never be
			// looked up again and only waits for the four-file window to evict it.
			gcEnums(dir, reachable)
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

// actionLogCached prints the action stream from the record layer. It returns
// handled=false when the cache cannot serve the request (off, cold, damaged), in
// which case the caller streams from git and seeds the cache as it reads.
//
// limit==0 prints the whole history: the enumeration comes from its snapshot or,
// on a miss, from git as the oracle. limit>0 is the first screen: it only takes
// a snapshot that already exists, because running git's full author-date walk to
// print three rows is exactly what the streaming path does — and on a big
// repository that walk costs several times a layer decode.
//
// The rows and the records come from the same reader the model commands use, so
// "find the record for this commit" has one implementation.
func (e *env) actionLogCached(backend vcs.Repo, jsonl bool, scratch *bytes.Buffer, enc *json.Encoder, limit int) (int, bool, error) {
	r, ok := e.openReader(backend)
	if !ok {
		return 0, false, nil
	}
	var oids [][20]byte
	if limit > 0 {
		oids, ok = e.cachedEnum(enumAuthorDate)
	} else {
		oids, ok = e.enumerateAuthorDate(r.cb)
	}
	if !ok || len(oids) == 0 {
		return 0, false, nil
	}
	fetched, ok := r.fetch(r.missing(oids))
	if !ok {
		return 0, false, nil
	}
	sink := newLayerSink(r.dir, r.fp)
	emit := func(rec cache.Record) error {
		when := time.Unix(rec.Date, 0).In(time.FixedZone("", int(rec.TZ)*60))
		return e.writeActionLogRow(hex.EncodeToString(rec.OID[:]), when, engine.FieldsFromRecord(rec), rec.Subject, jsonl, scratch, enc)
	}
	if limit > 0 {
		// Resolve the rows before printing any of them: a row that cannot be
		// resolved has to leave the caller free to fall back to git without
		// duplicating what is already on stdout.
		rows := make([]cache.Record, 0, limit)
		for _, oid := range oids {
			rec, fresh, hit := e.recordOf(r, oid, fetched)
			if !hit {
				return 0, false, nil
			}
			if fresh {
				sink.add(rec)
			}
			if !rec.Action {
				continue
			}
			rows = append(rows, rec)
			if len(rows) == limit {
				break
			}
		}
		for _, rec := range rows {
			if werr := emit(rec); werr != nil {
				return 0, true, werr
			}
		}
		// What gets written holds the newest records only, so it claims no tip:
		// claiming HEAD would say it covers a history it only sampled.
		e.settleRead(r.cb, r.dir, r.fp, sink, nil, len(oids), false)
		return len(rows), true, nil
	}
	emitted := 0
	for _, oid := range oids {
		rec, fresh, hit := e.recordOf(r, oid, fetched)
		if !hit {
			return 0, false, nil
		}
		if fresh {
			sink.add(rec)
		}
		if !rec.Action {
			continue
		}
		if werr := emit(rec); werr != nil {
			return emitted, true, werr
		}
		emitted++
	}
	// The first enumerated commit is the newest tip of HEAD's history; claiming it
	// covers a subset of what HEAD covers, which is all the records prove.
	e.settleRead(r.cb, r.dir, r.fp, sink, [][20]byte{oids[0]}, len(oids), false)
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

// cacheCmd is the escape hatch, in two shapes: gc reclaims (reachability, merge,
// broken files), clear forgets everything so the next read rebuilds with the
// code in this binary.
func (e *env) cacheCmd(args []string) int {
	pos, _, missing := parse(args)
	if code := rejectMissingValues(e.errw, missing, allowedFlags()); code != 0 {
		return code
	}
	if len(pos) != 1 || (pos[0] != "gc" && pos[0] != "clear") {
		fmt.Fprint(e.errw, i18n.T("用法: kiln cache gc | kiln cache clear\n"))
		return 2
	}
	backend, err := e.backend()
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("打开仓库失败: %v\n", err))
		return 1
	}
	if pos[0] == "clear" {
		return e.cacheClear(backend)
	}
	return e.cacheGC(backend)
}

// cacheClear deletes every layer and snapshot. A read that was already running
// has its layers in memory and may write a fresh one afterwards; that is a
// rebuild, not a conflict. Clearing is the recovery path: the next read scans
// with the code in this binary, so a parser fix stops being masked by layers
// written by an older version.
func (e *env) cacheClear(backend vcs.Repo) int {
	_, dir, _, ok := e.cacheDir(backend)
	if !ok {
		fmt.Fprint(e.errw, i18n.T("当前后端不支持缓存\n"))
		return 1
	}
	enumsDir := filepath.Join(filepath.Dir(dir), "enums")
	layers, _ := cache.Scan(dir)
	enums, _ := os.ReadDir(enumsDir)
	if len(layers) == 0 && len(enums) == 0 {
		fmt.Fprintln(e.out, i18n.T("缓存是空的"))
		return 0
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprint(e.errw, i18n.T("清空缓存失败: %v\n", err))
		return 1
	}
	if err := os.RemoveAll(enumsDir); err != nil {
		fmt.Fprint(e.errw, i18n.T("清空缓存失败: %v\n", err))
		return 1
	}
	fmt.Fprintln(e.out, i18n.T("已清空缓存：层 %d、快照 %d；下一次读会用当前版本重建", len(layers), len(enums)))
	return 0
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
	// The layer set is settled now; Sweep takes what no read would ever touch
	// again: a torn temp file a crashed writer left behind.
	_, _ = cache.Sweep(dir, time.Now())
	fmt.Fprintln(e.out, i18n.T("已清理：保留 %d 条记录，丢弃 %d 条，层 %d → %d",
		len(kept), len(records)-len(kept), len(layers), len(layers)-removed))
	return 0
}
