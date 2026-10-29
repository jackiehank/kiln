package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FilePrefix / FileSuffix name a layer file; the middle is the hex of its own
// SHA-1, so a file can be verified without trusting anything else.
const (
	FilePrefix = "layer-"
	FileSuffix = ".rec"
	TempInfix  = ".tmp-"
)

// StaleTemp is how old an orphaned temporary file must be before Sweep removes
// it: old enough that no live writer can still be using the name.
const StaleTemp = time.Hour

// Layer is one scanned layer file. Err is non-nil when the file is unusable
// (truncated, corrupt, wrong magic): readers ignore it, Sweep deletes it.
// ModTime is when the file was written; reads use its age to decide whether a
// layer from another vocabulary is worth keeping around.
type Layer struct {
	Name    string
	Path    string
	Records []Record
	Tips    [][20]byte
	FP      [20]byte
	ModTime time.Time
	Err     error
}

// Name returns the canonical file name for the encoded bytes: the SHA-1 of
// everything except the trailer, in hex.
func Name(data []byte) string {
	if len(data) < trailerSize {
		return ""
	}
	sum := sha1.Sum(data[:len(data)-trailerSize])
	return FilePrefix + hex.EncodeToString(sum[:]) + FileSuffix
}

// Scan reads every layer in dir. A missing directory is not an error (no cache
// yet). Files that do not decode are returned with Err set, so the caller can
// ignore them and Sweep can remove them.
func Scan(dir string) ([]Layer, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Layer
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}
		path := filepath.Join(dir, name)
		layer := Layer{Name: name, Path: path}
		if info, ierr := e.Info(); ierr == nil {
			layer.ModTime = info.ModTime()
		}
		data, err := os.ReadFile(path)
		if err != nil {
			layer.Err = err
			out = append(out, layer)
			continue
		}
		recs, tips, fp, err := Decode(data)
		if err == nil && Name(data) != name {
			err = fmt.Errorf("layer name %q does not match its contents", name)
		}
		layer.Records, layer.Tips, layer.FP, layer.Err = recs, tips, fp, err
		out = append(out, layer)
	}
	return out, nil
}

// RemoveStaleTemps deletes the torn temporary files a crashed writer leaves
// behind: a live writer only holds its temp for the duration of a write and a
// rename, so anything older than StaleTemp is dead. It reads directory entries
// only — no decode, no git — so a read path can afford to call it every time.
func RemoveStaleTemps(dir string, now time.Time) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var removed []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, FilePrefix) || !strings.Contains(name, TempInfix) {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil || now.Sub(info.ModTime()) < StaleTemp {
			continue
		}
		if os.Remove(filepath.Join(dir, name)) == nil {
			removed = append(removed, name)
		}
	}
	return removed
}

// Write stores records as a new layer and returns its file name. The write is
// atomic (temporary file then rename) and idempotent: the same records plus tips
// produce the same name, so a concurrent writer lands on the same file and
// "rename onto an existing identical file" is treated as success.
func Write(dir string, records []Record, tips [][20]byte, fingerprint [20]byte) (string, error) {
	if len(records) == 0 {
		return "", nil
	}
	data := Encode(records, tips, fingerprint)
	name := Name(data)
	if name == "" {
		return "", errors.New("encoded layer too short")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, name)
	if _, err := os.Stat(final); err == nil {
		return name, nil // identical content is already there
	}
	tmp := final + TempInfix + fmt.Sprint(os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		// A concurrent writer may have won the race with identical bytes.
		if _, statErr := os.Stat(final); statErr == nil {
			_ = os.Remove(tmp)
			return name, nil
		}
		_ = os.Remove(tmp)
		return "", err
	}
	return name, nil
}

// Remove deletes one layer file. Missing is fine: a concurrent merge may have
// already removed it.
func Remove(l Layer) error {
	if err := os.Remove(l.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Merge unions the valid layers: one record per OID (duplicates are identical by
// construction) and the union of their covered tips.
func Merge(layers []Layer) ([]Record, [][20]byte) {
	seen := make(map[[20]byte]bool)
	var recs []Record
	var tips [][20]byte
	for _, l := range layers {
		if l.Err != nil {
			continue
		}
		for _, r := range l.Records {
			if seen[r.OID] {
				continue
			}
			seen[r.OID] = true
			recs = append(recs, r)
		}
		tips = append(tips, l.Tips...)
	}
	return recs, sortTips(tips)
}

// Sweep removes unusable layer files and stale temporary files, and reports what
// it deleted. It never touches a readable layer: dropping records is the job of
// the caller (reachability), not of housekeeping.
func Sweep(dir string, now time.Time) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(dir, name)
		switch {
		case strings.HasPrefix(name, FilePrefix) && strings.Contains(name, TempInfix):
			info, err := e.Info()
			if err != nil || now.Sub(info.ModTime()) < StaleTemp {
				continue
			}
			if err := os.Remove(path); err == nil {
				removed = append(removed, name)
			}
		case strings.HasPrefix(name, FilePrefix) && strings.HasSuffix(name, FileSuffix):
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if _, _, _, decErr := Decode(data); decErr != nil || Name(data) != name {
				if err := os.Remove(path); err == nil {
					removed = append(removed, name)
				}
			}
		}
	}
	return removed, nil
}
