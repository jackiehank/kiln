package cli

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackiehank/kiln/internal/vcs"
)

// Enumeration snapshots.
//
// A warm read needs one thing from git beyond the record layer: the ordered list
// of commit ids `git log` would print. That list is a pure function of HEAD (and
// of the few features that rewrite history), so it is cached verbatim: the
// snapshot is git's own output, which keeps ordering byte-identical instead of
// re-deriving git's walk. A hit costs one file read and zero processes; a miss
// runs git exactly as before and stores what it printed.
//
// Snapshots are content-addressed by (version, kind, HEAD, invalidation factors):
// a new HEAD is a new file, so there is nothing to invalidate — the old file is
// simply never looked up again and is pruned later.
const (
	enumMagic   = "KILNEN"
	enumVersion = 1
	// enumKeep is how many snapshots survive a write, both orders included. A
	// repository gets a new HEAD on every commit; keeping a handful covers the
	// worktree that is being used without letting 2 MB files pile up, and an
	// evicted snapshot costs one git log to rebuild.
	enumKeep = 4
)

// enumKind distinguishes the two orders kiln asks git for.
type enumKind byte

const (
	// enumDefault is `git log --format=%H` (commit-date order), what every read
	// command builds its model from.
	enumDefault enumKind = 1
	// enumAuthorDate is `git log --author-date-order --format=%H`, the order
	// `action log` prints in.
	enumAuthorDate enumKind = 2
	// enumBoth marks one list that both orders produce byte for byte. It is only
	// written when the walked history is linear (no commit with two parents):
	// every step then has exactly one candidate, so the ordering comparator never
	// gets a choice and both git commands print the same sequence. Readers look
	// for their own kind first and fall back to this one.
	enumBoth enumKind = 3
)

// enumKey is everything a snapshot depends on. head is the tip whose ancestors
// the list contains; inv is a digest of the features that can change that list
// without changing HEAD (shallow boundary, replace refs, grafts).
type enumKey struct {
	head [20]byte
	inv  [20]byte
}

// gitDirFromFS walks up from dir to the git directory that belongs to this
// worktree (the one holding HEAD), without running git. fromDir distinguishes a
// plain checkout (`.git` is a directory) from a linked worktree (`.git` is a
// file naming the per-worktree directory).
func gitDirFromFS(dir string) (string, bool, bool) {
	start, err := filepath.Abs(dir)
	if err != nil {
		start = filepath.Clean(dir)
	}
	for d := start; ; {
		gitPath := filepath.Join(d, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				return gitPath, true, true
			}
			data, rerr := os.ReadFile(gitPath)
			if rerr != nil {
				return "", false, false
			}
			line := strings.TrimSpace(string(data))
			const prefix = "gitdir:"
			if !strings.HasPrefix(line, prefix) {
				return "", false, false
			}
			gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(d, gitDir)
			}
			return filepath.Clean(gitDir), false, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false, false
		}
		d = parent
	}
}

// headFromFS resolves HEAD to a commit id by reading refs. Anything unusual —
// an unborn branch, a symbolic ref that is not a plain branch, a reftable store,
// git's own directory environment variables — comes back not-ok, and the caller
// falls back to git rather than guessing.
func headFromFS(dir string) ([20]byte, bool) {
	if os.Getenv("GIT_DIR") != "" || os.Getenv("GIT_WORK_TREE") != "" || os.Getenv("GIT_COMMON_DIR") != "" {
		return [20]byte{}, false
	}
	gitDir, _, ok := gitDirFromFS(dir)
	if !ok {
		return [20]byte{}, false
	}
	common := gitDir
	if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		c := strings.TrimSpace(string(data))
		if !filepath.IsAbs(c) {
			c = filepath.Join(gitDir, c)
		}
		common = filepath.Clean(c)
	}
	// A reftable repository keeps refs in a binary store; reading it is not this
	// package's job.
	if _, err := os.Stat(filepath.Join(common, "reftable")); err == nil {
		return [20]byte{}, false
	}
	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return [20]byte{}, false
	}
	line := strings.TrimSpace(string(raw))
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		return refFromFS(gitDir, common, ref)
	}
	return parseOIDText(line)
}

// refFromFS resolves one ref: the loose file (worktree directory first, then the
// shared one, which is where refs live in a linked worktree), then packed-refs.
func refFromFS(gitDir, common, ref string) ([20]byte, bool) {
	for _, base := range []string{gitDir, common} {
		if data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(ref))); err == nil {
			return parseOIDText(strings.TrimSpace(string(data)))
		}
	}
	data, err := os.ReadFile(filepath.Join(common, "packed-refs"))
	if err != nil {
		return [20]byte{}, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}
		if f := strings.Fields(line); len(f) == 2 && f[1] == ref {
			return parseOIDText(f[0])
		}
	}
	return [20]byte{}, false
}

func parseOIDText(s string) ([20]byte, bool) {
	var oid [20]byte
	if len(s) != 40 {
		return oid, false
	}
	if _, err := hex.Decode(oid[:], []byte(s)); err != nil {
		return oid, false
	}
	return oid, true
}

// enumInvariant digests the state that can change what `git log` prints for a
// fixed HEAD. ok=false means "do not trust a snapshot": a feature we cannot see
// in the digest is in play.
func enumInvariant(common string) ([20]byte, bool) {
	for _, key := range []string{"GIT_REPLACE_REF_BASE", "GIT_NO_REPLACE_OBJECTS", "GIT_GRAFT_FILE"} {
		if os.Getenv(key) != "" {
			return [20]byte{}, false
		}
	}
	if _, err := os.Stat(filepath.Join(common, "info", "grafts")); err == nil {
		return [20]byte{}, false
	}
	if found, ok := replaceRefs(common); !ok || found {
		return [20]byte{}, false
	}
	h := sha1.New()
	h.Write([]byte("kiln-enum-invariant\x00"))
	// A shallow boundary is part of the history git will show.
	if data, err := os.ReadFile(filepath.Join(common, "shallow")); err == nil {
		h.Write(data)
	} else if !os.IsNotExist(err) {
		return [20]byte{}, false
	}
	var sum [20]byte
	copy(sum[:], h.Sum(nil))
	return sum, true
}

// replaceRefs reports whether the repository has any replace ref, loose or
// packed. git applies them while walking history, so a snapshot taken without
// one is wrong the moment one appears — and pack-refs moves the loose file into
// packed-refs, where a directory listing cannot see it. ok=false means the state
// could not be read.
func replaceRefs(common string) (found, ok bool) {
	entries, err := os.ReadDir(filepath.Join(common, "refs", "replace"))
	switch {
	case err == nil && len(entries) > 0:
		return true, true
	case err == nil, os.IsNotExist(err):
	default:
		return false, false
	}
	data, err := os.ReadFile(filepath.Join(common, "packed-refs"))
	if err != nil {
		return false, os.IsNotExist(err)
	}
	// A ref name cannot contain a space, so the bare substring is precise enough
	// and avoids splitting a file that can hold every tag in the repository.
	return bytes.Contains(data, []byte("refs/replace/")), true
}

// enumSite resolves where snapshots live and what one for this worktree must be
// keyed by. ok=false means the filesystem could not name HEAD confidently, in
// which case every read keeps asking git.
func (e *env) enumSite() (string, enumKey, bool) {
	if cacheOff {
		return "", enumKey{}, false
	}
	common, ok := commonDirFromFS(e.dir)
	if !ok {
		return "", enumKey{}, false
	}
	head, ok := headFromFS(e.dir)
	if !ok {
		return "", enumKey{}, false
	}
	inv, ok := enumInvariant(common)
	if !ok {
		return "", enumKey{}, false
	}
	return filepath.Join(common, "kiln", "enums"), enumKey{head: head, inv: inv}, true
}

// enumPath names one snapshot: a digest of the key, so two worktrees (or two
// HEDs of one worktree) never collide and never need invalidating.
func enumPath(dir string, kind enumKind, key enumKey) string {
	h := sha1.New()
	h.Write([]byte(enumMagic))
	h.Write([]byte{enumVersion, byte(kind)})
	h.Write(key.head[:])
	h.Write(key.inv[:])
	return filepath.Join(dir, "enum-"+hex.EncodeToString(h.Sum(nil))+".oid")
}

var enumCRC = crc32.MakeTable(crc32.Castagnoli)

// enumEncode lays one snapshot out: header, ids, checksum. The checksum is what
// keeps a truncated or bit-rotted file from being read as a shorter history.
func enumEncode(kind enumKind, key enumKey, oids [][20]byte) []byte {
	buf := make([]byte, 0, 52+len(oids)*20+4)
	buf = append(buf, enumMagic...)
	buf = append(buf, enumVersion, byte(kind))
	buf = append(buf, key.head[:]...)
	buf = append(buf, key.inv[:]...)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(oids)))
	buf = append(buf, n[:]...)
	for _, o := range oids {
		buf = append(buf, o[:]...)
	}
	var c [4]byte
	binary.BigEndian.PutUint32(c[:], crc32.Checksum(buf, enumCRC))
	buf = append(buf, c[:]...)
	return buf
}

// enumDecode parses a snapshot and checks it belongs to the key we asked for.
func enumDecode(data []byte, kind enumKind, key enumKey) ([][20]byte, bool) {
	const header = 6 + 2 + 20 + 20 + 4
	if len(data) < header+4 {
		return nil, false
	}
	if string(data[:6]) != enumMagic || data[6] != enumVersion || enumKind(data[7]) != kind {
		return nil, false
	}
	if string(data[8:28]) != string(key.head[:]) || string(data[28:48]) != string(key.inv[:]) {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32(data[48:52]))
	if n < 0 || len(data) != header+n*20+4 {
		return nil, false
	}
	want := binary.BigEndian.Uint32(data[len(data)-4:])
	if crc32.Checksum(data[:len(data)-4], enumCRC) != want {
		return nil, false
	}
	oids := make([][20]byte, n)
	for i := 0; i < n; i++ {
		copy(oids[i][:], data[header+i*20:header+i*20+20])
	}
	return oids, true
}

// loadEnum reads the snapshot for this key, if there is a sound one: the order's
// own file first, then the shared one (a linear history prints both orders
// identically, so one file is honest for both).
func loadEnum(dir string, kind enumKind, key enumKey) ([][20]byte, bool) {
	if oids, ok := readEnum(dir, kind, key); ok {
		return oids, true
	}
	if kind != enumBoth {
		if oids, ok := readEnum(dir, enumBoth, key); ok {
			return oids, true
		}
	}
	return nil, false
}

// readEnum reads and verifies one snapshot file.
func readEnum(dir string, kind enumKind, key enumKey) ([][20]byte, bool) {
	path := enumPath(dir, kind, key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	oids, ok := enumDecode(data, kind, key)
	if !ok {
		// A torn or bit-rotted snapshot can never be used again. Dropping it lets
		// this miss rebuild it, instead of every later read paying a git process
		// for a file that will never decode.
		_ = os.Remove(path)
	}
	return oids, ok
}

// storeEnum writes a snapshot (own order or shared) and prunes the old window.
// failure is silent: a cache that cannot be written must not change what the
// command prints or what it exits with.
func storeEnum(dir string, kind enumKind, key enumKey, oids [][20]byte) {
	if len(oids) == 0 {
		return
	}
	path := enumPath(dir, kind, key)
	// Compare the bytes, not just the size: a file of the right size with a
	// broken checksum would otherwise be left in place and miss forever.
	if data, err := os.ReadFile(path); err == nil && bytes.Equal(data, enumEncode(kind, key, oids)) {
		return // identical content is already there
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmp, enumEncode(kind, key, oids), 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return
	}
	pruneEnums(dir, path)
}

// pruneEnums keeps the newest snapshots, whatever order they cache: the orders
// share one four-file window, which is enough for the worktree in use and
// costs one git log to refill when a snapshot is evicted. Deleting an old one is
// always safe — a reader that loses the race falls back to git.
func pruneEnums(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type aged struct {
		path string
		mod  int64
	}
	var files []aged
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasPrefix(name, "enum-") || !strings.HasSuffix(name, ".oid") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		files = append(files, aged{filepath.Join(dir, name), info.ModTime().UnixNano()})
	}
	if len(files) <= enumKeep {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	for i := enumKeep; i < len(files); i++ {
		if files[i].path == keep {
			continue
		}
		_ = os.Remove(files[i].path)
	}
}

// cachedEnum answers from a snapshot that already exists, without running git.
// The first-screen path (`action log --limit`) uses it so a peek never pays for
// the full walk it is trying to avoid.
func (e *env) cachedEnum(kind enumKind) ([][20]byte, bool) {
	dir, key, ok := e.enumSite()
	if !ok {
		return nil, false
	}
	return loadEnum(dir, kind, key)
}

// enumerateDefault answers "which commits, in git's default order" from the
// snapshot, running git only on a miss — and then storing what git printed.
func (e *env) enumerateDefault(cb vcs.CacheBackend) ([][20]byte, bool) {
	if oids, ok := e.cachedEnum(enumDefault); ok {
		return oids, true
	}
	lines, err := cb.LogOIDs()
	if err != nil || len(lines) == 0 {
		return nil, false
	}
	oids := make([][20]byte, 0, len(lines))
	linear := true
	for _, line := range lines {
		oid, ok := decodeOID(line.ID)
		if !ok {
			return nil, false
		}
		if unlinear(line.Parents, line.ParentsKnown) {
			linear = false
		}
		oids = append(oids, oid)
	}
	if dir, key, siteOK := e.enumSite(); siteOK && e.enumKeyHeld(dir, key) {
		kind := enumDefault
		if linear {
			kind = enumBoth
		}
		storeEnum(dir, kind, key, oids)
	}
	return oids, true
}

// enumerateAuthorDate is the same for `action log`'s order.
func (e *env) enumerateAuthorDate(cb vcs.CacheBackend) ([][20]byte, bool) {
	if oids, ok := e.cachedEnum(enumAuthorDate); ok {
		return oids, true
	}
	var oids [][20]byte
	linear := true
	err := cb.EachOIDAuthorDate(func(line vcs.Line) bool {
		oid, ok := decodeOID(line.ID)
		if !ok {
			return false
		}
		if unlinear(line.Parents, line.ParentsKnown) {
			linear = false
		}
		oids = append(oids, oid)
		return true
	})
	if err != nil || len(oids) == 0 {
		return nil, false
	}
	if dir, key, siteOK := e.enumSite(); siteOK && e.enumKeyHeld(dir, key) {
		kind := enumAuthorDate
		if linear {
			kind = enumBoth
		}
		storeEnum(dir, kind, key, oids)
	}
	return oids, true
}

// enumKeyHeld reports whether the site still names the same HEAD and invariants
// after git ran. A commit landing mid-enumeration must not leave a snapshot that
// claims the new HEAD with the old list; on a disagreement the list is returned
// as-is and simply not stored.
func (e *env) enumKeyHeld(dir string, key enumKey) bool {
	dir2, key2, ok := e.enumSite()
	return ok && dir2 == dir && key2 == key
}

// gcEnums drops snapshots whose HEAD no ref can reach: they describe a history
// that no longer exists, and rebuilding one would only reproduce a HEAD that is
// gone. A snapshot that cannot be parsed is dropped too — it can never be used.
func gcEnums(recordsDir string, reachable map[[20]byte]bool) {
	dir := filepath.Join(filepath.Dir(recordsDir), "enums")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasPrefix(name, "enum-") || !strings.HasSuffix(name, ".oid") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) < 28 {
			_ = os.Remove(path)
			continue
		}
		var head [20]byte
		copy(head[:], data[8:28])
		if !reachable[head] {
			_ = os.Remove(path)
		}
	}
}

// storeEnumFromCommits seeds the default-order snapshot from a full scan: the
// scan already holds every commit in git's order, so a cold read leaves the next
// one with nothing to ask git for. It is filed only when HEAD is the tip the scan
// started from, so a commit landing mid-scan cannot be mislabelled as covered.
func (e *env) storeEnumFromCommits(commits []vcs.Commit) {
	if len(commits) == 0 {
		return
	}
	dir, key, ok := e.enumSite()
	if !ok {
		return
	}
	tip, ok := decodeOID(commits[0].ID)
	if !ok || tip != key.head {
		return
	}
	oids := make([][20]byte, 0, len(commits))
	for _, c := range commits {
		oid, ok := decodeOID(c.ID)
		if !ok {
			return
		}
		oids = append(oids, oid)
	}
	kind := enumDefault
	if linearHistory(commits) {
		kind = enumBoth
	}
	storeEnum(dir, kind, key, oids)
}

// unlinear reports whether one walked commit rules out a shared snapshot: it has
// two parents (a merge, so the two orderings may part ways) or the backend did
// not say what its parents are (nothing can be concluded). The rule lives in one
// place because every walk that may file a snapshot applies it.
func unlinear(parents []string, known bool) bool {
	return !known || len(parents) > 1
}

// linearHistory reports whether the walked history has no merge: every commit
// lists at most one parent. A backend that does not report parents cannot prove
// it, so it answers "no". A linear walk means both of git's orderings have
// exactly one candidate at every step, so their output is the same sequence and
// one snapshot can honestly serve both.
func linearHistory(commits []vcs.Commit) bool {
	if len(commits) == 0 {
		return false
	}
	for _, c := range commits {
		if unlinear(c.Parents, c.ParentsKnown) {
			return false
		}
	}
	return true
}
