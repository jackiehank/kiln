package cache

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jackiehank/kiln/internal/model"
)

func oid(n byte) [20]byte {
	var o [20]byte
	o[0] = n
	return o
}

func sampleRecords() []Record {
	return []Record{
		{
			OID: oid(2), Date: 1_700_000_000, TZ: 480, Action: true,
			Issue: "alpha", Kind: "ask", Title: "开单", Subject: "开单",
			Domain: "design", Scope: "", Target: "",
			Refs: []string{"beta"}, Tags: []string{"v1"},
			Relations: []Rel{{Name: "blocks", Target: "beta"}},
		},
		{
			OID: oid(1), Date: 1_700_000_060, TZ: -300, Action: false,
			Issue: "", Kind: "", Title: "", Subject: "无 trailer 的提交",
			Unknown: []string{"foo"},
		},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	fp := Fingerprint(model.DefaultRelationSet())
	recs := sampleRecords()
	tips := [][20]byte{oid(9)}
	data := Encode(recs, tips, fp)

	got, gotTips, gotFP, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if gotFP != fp {
		t.Fatal("fingerprint not preserved")
	}
	if !reflect.DeepEqual(gotTips, tips) {
		t.Fatalf("tips = %x, want %x", gotTips, tips)
	}
	if len(got) != len(recs) {
		t.Fatalf("records = %d, want %d", len(got), len(recs))
	}
	// records come back sorted by OID
	want := []Record{recs[1], recs[0]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	fp := Fingerprint(model.DefaultRelationSet())
	a := Encode(sampleRecords(), [][20]byte{oid(9)}, fp)
	b := Encode(sampleRecords(), [][20]byte{oid(9)}, fp)
	if !bytes.Equal(a, b) {
		t.Fatal("same input must encode to the same bytes")
	}
	shuffled := []Record{sampleRecords()[1], sampleRecords()[0]}
	c := Encode(shuffled, [][20]byte{oid(9)}, fp)
	if !bytes.Equal(a, c) {
		t.Fatal("record order must not affect the encoding")
	}
	if Name(a) != Name(c) {
		t.Fatal("record order must not affect the file name")
	}
}

func TestDecodeRejectsCorruptionAndTruncation(t *testing.T) {
	fp := Fingerprint(model.DefaultRelationSet())
	data := Encode(sampleRecords(), nil, fp)

	flipped := append([]byte(nil), data...)
	flipped[len(flipped)/2] ^= 0xff
	if _, _, _, err := Decode(flipped); err == nil {
		t.Fatal("a flipped byte must be rejected")
	}
	if _, _, _, err := Decode(data[:len(data)/2]); err == nil {
		t.Fatal("a truncated layer must be rejected")
	}
	renamed := append([]byte(nil), data...)
	copy(renamed, "KILNREC2")
	if _, _, _, err := Decode(renamed); err == nil {
		t.Fatal("an unknown magic must be rejected")
	}
}

func TestWriteIsIdempotentAndAtomic(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint(model.DefaultRelationSet())
	first, err := Write(dir, sampleRecords(), [][20]byte{oid(9)}, fp)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(dir, sampleRecords(), [][20]byte{oid(9)}, fp)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("same content must produce the same name: %s vs %s", first, second)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one layer, got %d", len(entries))
	}
	if _, err := Write(dir, nil, nil, fp); err != nil || second == "" {
		t.Fatalf("empty write must be a no-op, got %v", err)
	}
}

func TestScanIgnoresTornFileAndSweepRemovesIt(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint(model.DefaultRelationSet())
	if _, err := Write(dir, sampleRecords(), nil, fp); err != nil {
		t.Fatal(err)
	}
	torn := filepath.Join(dir, FilePrefix+"deadbeef"+FileSuffix)
	if err := os.WriteFile(torn, []byte("not a layer"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, FilePrefix+"cafe"+FileSuffix+TempInfix+"12345")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * StaleTemp)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	layers, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	var valid, invalid int
	for _, l := range layers {
		if l.Err != nil {
			invalid++
		} else {
			valid++
		}
	}
	if valid != 1 || invalid != 1 {
		t.Fatalf("scan: valid=%d invalid=%d, want 1/1", valid, invalid)
	}
	if recs, _ := Merge(layers); len(recs) != 2 {
		t.Fatalf("merge must only use valid layers, got %d records", len(recs))
	}

	removed, err := Sweep(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("sweep removed %v, want the torn layer and the stale temp", removed)
	}
	layers, _ = Scan(dir)
	if len(layers) != 1 || layers[0].Err != nil {
		t.Fatalf("sweep must keep the valid layer: %+v", layers)
	}
}

func TestScanNameMustMatchContents(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint(model.DefaultRelationSet())
	data := Encode(sampleRecords(), nil, fp)
	// A valid layer under a wrong name is as untrustworthy as a corrupt one.
	wrong := filepath.Join(dir, FilePrefix+"0000"+FileSuffix)
	if err := os.WriteFile(wrong, data, 0o644); err != nil {
		t.Fatal(err)
	}
	layers, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 || layers[0].Err == nil {
		t.Fatal("a layer whose name does not match its bytes must be rejected")
	}
}

func TestMergeDedupesAndUnionsTips(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint(model.DefaultRelationSet())
	recs := sampleRecords()
	if _, err := Write(dir, recs[:1], [][20]byte{oid(7)}, fp); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, recs, [][20]byte{oid(8)}, fp); err != nil {
		t.Fatal(err)
	}
	layers, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	merged, tips := Merge(layers)
	if len(merged) != 2 {
		t.Fatalf("merge must dedupe by OID, got %d records", len(merged))
	}
	wantTips := [][20]byte{oid(7), oid(8)}
	if !reflect.DeepEqual(tips, wantTips) {
		t.Fatalf("tips = %x, want %x", tips, wantTips)
	}
}

func TestFingerprintTracksVocabulary(t *testing.T) {
	def := Fingerprint(model.DefaultRelationSet())
	set, err := model.NewRelationSet(
		[]model.RelationSlot{{Name: "blocks"}},
		[]model.RelationName{{Name: "blocks", Slot: "blocks", Rank: 1}},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	custom := Fingerprint(set)
	if def == custom {
		t.Fatal("different vocabularies must produce different fingerprints")
	}
	if again := Fingerprint(model.DefaultRelationSet()); again != def {
		t.Fatal("the same vocabulary must produce the same fingerprint")
	}
}
