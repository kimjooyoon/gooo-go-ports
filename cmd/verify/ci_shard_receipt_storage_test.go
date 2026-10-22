package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ciShardTestGzip(t *testing.T, raw []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestCIShardCompressedReceiptsKeepRawProofAndStoredIdentity(t *testing.T) {
	plan, inventory, root, partial := ciShardReferenceFixture(t)
	dir := ciShardBatchDirectory(root, 0, 0, 0)
	original, err := os.ReadFile(filepath.Join(dir, "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := compactCIShardReceipts(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "execution.json")); !os.IsNotExist(err) {
		t.Fatal("plain receipt retained after successful pair compaction")
	}
	record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, root, partial.Suites[0])
	if err != nil {
		t.Fatal(err)
	}
	if !equalCIShardSuiteRecords(record, partial.Suites[0]) {
		t.Fatal("original receipt semantics or raw hashes changed")
	}
	index, err := buildCIShardNativeReference(plan, inventory, []ciShardPartial{partial}, map[int]string{0: filepath.Join(root, "partial.json")}, map[int]string{0: root})
	if err != nil {
		t.Fatal(err)
	}
	b := index.Suites[0].Batches[0]
	packed, err := os.ReadFile(filepath.Join(dir, "execution.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if b.Execution.Encoding != "gzip" || !strings.HasSuffix(b.Execution.Path, "/execution.json.gz") || b.Execution.SHA256 != contentDigest(original) || b.Execution.Bytes != int64(len(original)) || b.Execution.StoredBytes != int64(len(packed)) || b.Execution.StoredSHA256 != contentDigest(packed) || b.Batch.ReplayedFrom != contentDigest(original) {
		t.Fatal("stored and original JSON identities conflated", b)
	}
	if index.NativeAcceptance || index.ProducerAuthenticated {
		t.Fatal("storage cannot add authority")
	}
}
func TestCIShardGzipStrictStreamAndRawLimits(t *testing.T) {
	raw := []byte(" {\n\t\"ok\":false} \n")
	packed := ciShardTestGzip(t, raw)
	decoded, err := decodeCIShardGzip(packed, int64(len(raw)))
	if err != nil || !bytes.Equal(raw, decoded) {
		t.Fatal("exact bytes changed", err)
	}
	corrupt := append([]byte(nil), packed...)
	corrupt[len(corrupt)-5] ^= 1
	for name, b := range map[string][]byte{"crc": corrupt, "truncated": packed[:len(packed)-1], "trailing": append(append([]byte(nil), packed...), 0), "multiple": append(append([]byte(nil), packed...), packed...), "empty": ciShardTestGzip(t, nil)} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCIShardGzip(b, 1024); err == nil {
				t.Fatal("accepted invalid gzip")
			}
		})
	}
	if _, err := decodeCIShardGzip(packed, int64(len(raw)-1)); err == nil {
		t.Fatal("accepted decoded overflow")
	}
}
func TestCIShardReceiptRejectsAmbiguousAndStoredByteMutation(t *testing.T) {
	root := t.TempDir()
	raw := []byte("{\"result\":false}\n")
	packed := ciShardTestGzip(t, raw)
	p := filepath.Join(root, "execution.json.gz")
	if err := os.WriteFile(p, packed, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := newCIShardEvidenceReader(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.receipt("execution.json"); err != nil {
		t.Fatal(err)
	}
	changed := append([]byte(nil), packed...)
	changed[9] ^= 1 // OS metadata changes stored bytes but not original JSON.
	if err := os.WriteFile(p, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.finish(); err == nil {
		t.Fatal("stored gzip drift hidden by same decoded JSON")
	}
	if err := os.WriteFile(filepath.Join(root, "execution.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	r, _ = newCIShardEvidenceReader(root)
	if _, _, err := r.receipt("execution.json"); err == nil {
		t.Fatal("ambiguous representations accepted")
	}
}
func TestCIShardCompactionFailurePreservesBothOriginals(t *testing.T) {
	root := t.TempDir()
	raw := []byte("{\"fixture\":true}\n")
	for _, n := range []string{"execution.json", "replay.json", "replay.json.gz"} {
		if err := os.WriteFile(filepath.Join(root, n), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := compactCIShardReceipts(root); err == nil {
		t.Fatal("overwrote pre-existing destination")
	}
	for _, n := range []string{"execution.json", "replay.json"} {
		b, err := os.ReadFile(filepath.Join(root, n))
		if err != nil || !bytes.Equal(b, raw) {
			t.Fatal("lost original on compaction failure", err)
		}
	}
}
