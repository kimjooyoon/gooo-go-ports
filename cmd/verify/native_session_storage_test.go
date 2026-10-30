package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSessionStoragePlainReferenceUnchanged(t *testing.T) {
	raw := []byte(" {\"number\":1e0} \n")
	path := filepath.Join(t.TempDir(), "000001.jsonl")
	ref, err := writeNativeSessionStoredEvidence(path, raw, nativeSessionEvidencePlain)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(ref)
	legacy, legacyErr := json.Marshal(struct {
		Path   string `json:"path"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}{path, len(raw), nativeSessionReference(path, raw).SHA256})
	if err != nil || legacyErr != nil || !bytes.Equal(encoded, legacy) {
		t.Fatal("plain reference JSON changed", string(encoded), err, legacyErr)
	}
	decoded, err := readNativeSessionStoredEvidence(ref)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("plain original bytes changed", err)
	}
}

func TestNativeSessionStorageGzipBlobAndLogicalIdentity(t *testing.T) {
	raw := []byte(" {\"number\":1e0,\"text\":\"<&>界\"} \n")
	path := filepath.Join(t.TempDir(), "000001.jsonl")
	ref, err := writeNativeSessionStoredEvidence(path, raw, nativeSessionEvidenceGzip)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	logical, err := nativeSessionLogicalPath(ref)
	if err != nil || logical != path || ref.Path != path+".gz" || ref.Encoding != "gzip" || ref.Bytes != len(raw) || ref.SHA256 != nativeSessionReference(path, raw).SHA256 || ref.StoredBytes != int64(len(stored)) || ref.StoredSHA256 != nativeSessionReference(path, stored).SHA256 || ref.StoredSHA256 == ref.SHA256 {
		t.Fatal("stored gzip identity replaced original logical bytes", ref, err)
	}
	decoded, err := readNativeSessionStoredEvidence(ref)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("gzip reserialized original JSON", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("direct gzip frame storage wrote an extra plain file", err)
	}
	changed := append([]byte(nil), stored...)
	changed[9] ^= 1 // gzip OS header changes stored identity, not decoded JSON.
	if decoded, err := decodeNativeSessionGzip(changed, int64(len(raw))); err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("control must retain decoded bytes", err)
	}
	if err := os.WriteFile(ref.Path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readNativeSessionStoredEvidence(ref); err == nil {
		t.Fatal("stored-byte drift hidden behind unchanged logical identity")
	}
}

func TestNativeSessionStorageGzipRefusesCorruptionAndExpansion(t *testing.T) {
	raw := []byte(" {\"result\":false} \n")
	packed := ciShardTestGzip(t, raw)
	corrupt := append([]byte(nil), packed...)
	corrupt[len(corrupt)-5] ^= 1
	for name, stored := range map[string][]byte{"crc": corrupt, "truncated": packed[:len(packed)-1], "trailing": append(append([]byte(nil), packed...), 0), "multiple": append(append([]byte(nil), packed...), packed...), "empty": ciShardTestGzip(t, nil)} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeNativeSessionGzip(stored, 1024); err == nil {
				t.Fatal("invalid gzip accepted")
			}
		})
	}
	if _, err := decodeNativeSessionGzip(packed, int64(len(raw)-1)); err == nil {
		t.Fatal("decoded length cap bypassed")
	}
	bomb := ciShardTestGzip(t, bytes.Repeat([]byte("a"), nativeSessionEvidenceDecodedLimit+1))
	if _, err := decodeNativeSessionGzip(bomb, nativeSessionEvidenceDecodedLimit); err == nil {
		t.Fatal("session raw expansion cap bypassed")
	}
}

func TestNativeSessionStorageDecoderKeepsOriginalWireAllowance(t *testing.T) {
	raw := []byte("{\"padding\":\"" + strings.Repeat("a", ciShardStoredReceiptLimit) + "\"}\n")
	var packed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&packed, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if packed.Len() <= ciShardStoredReceiptLimit || len(raw) > nativeSessionEvidenceDecodedLimit {
		t.Fatal("control does not distinguish storage profiles")
	}
	decoded, err := decodeNativeSessionGzip(packed.Bytes(), int64(len(raw)))
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("shard8MiB bound silently shrank native16MiB wire", err)
	}
}

func TestNativeSessionStorageRejectsEncodingAndMetadataMixups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	ref, err := writeNativeSessionStoredEvidence(path, []byte("{}\n"), nativeSessionEvidenceGzip)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*nativeSessionFileReference){
		"unknown":       func(r *nativeSessionFileReference) { r.Encoding = "other" },
		"stored-length": func(r *nativeSessionFileReference) { r.StoredBytes++ },
		"stored-hash":   func(r *nativeSessionFileReference) { r.StoredSHA256 = strings.Repeat("0", 64) },
		"raw-length":    func(r *nativeSessionFileReference) { r.Bytes-- },
		"raw-hash":      func(r *nativeSessionFileReference) { r.SHA256 = strings.Repeat("0", 64) },
		"plain-label":   func(r *nativeSessionFileReference) { r.Encoding = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := ref
			mutate(&changed)
			if _, err := readNativeSessionStoredEvidence(changed); err == nil {
				t.Fatal("reference identity mixup accepted")
			}
		})
	}
}
