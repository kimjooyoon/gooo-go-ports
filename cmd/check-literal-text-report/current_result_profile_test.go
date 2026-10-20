package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// This is an fresh saved actual205 receipt, not a relabeled839 fixture.
func TestCurrentGo1273TypedResultMatchesSavedOwnReplayParent(t *testing.T) {
	raw, err := os.ReadFile("../check-path-facts/testdata/current-go1273-execution.json")
	if err != nil {
		t.Fatal(err)
	}
	if digest(raw) != "sha256:8eb0e7c0c7eac774423ef2b74d4d8228478ba659748d98f3c5e83c6cd58ae277" {
		t.Fatal("actual current receipt raw bytes changed")
	}
	encoded, err := pinnedResultBytes(raw)
	if err != nil || len(encoded) != 85954 || digest(encoded) != "sha256:abd836ff53ff153e779c706f27d4b4350260c20b85a9d69bfd7bc9f09cdebe41" {
		t.Fatal("source-backed current typed Result differs from its saved own replay parent", len(encoded), err)
	}
	for name, mutated := range map[string][]byte{
		"unknown current typed field": bytes.Replace(raw, []byte(`"activity_origins":`), []byte(`"untrusted_origin":0,"activity_origins":`), 1),
		"origin integer type":         bytes.Replace(raw, []byte(`"start_byte":721`), []byte(`"start_byte":721.0`), 1),
	} {
		if bytes.Equal(mutated, raw) {
			t.Fatal("ineffective current-profile mutation", name)
		}
		if _, err := pinnedResultBytes(mutated); err == nil {
			t.Fatal("current profile accepted malformed source-derived field", name)
		}
	}
	historical, err := os.ReadFile("../check-literal-text-report/testdata/pinned-original-execution.json")
	if err != nil {
		t.Fatal(err)
	}
	if digest(historical) != "sha256:1bb2b3420f28836452e93a0bcda271d36ebebbfa41bd0e173f61b307c6a02e60" {
		t.Fatal("actual historical receipt changed")
	}
	if _, err := pinnedResultBytes(historical); err == nil {
		t.Fatal("current caller accepted a historical producer")
	}
	if _, err := legacy839ResultBytes(raw); err == nil {
		t.Fatal("archival caller accepted a current producer")
	}
}

// Only source-declared finite shapes are supported; these are synthetic codec
// controls, not new native observations or proof that provenance is authentic.
func TestCurrentGo1273ReadAndLocalMetadataStaysFinite(t *testing.T) {
	var profile resultProfile
	if err := json.Unmarshal(resultProfileJSON, &profile); err != nil {
		t.Fatal(err)
	}
	readPath := "$.composition.steps.*.generation.report.body_reads"
	good := `[{"activity_id":"gooo://go/strings/Count","kind":"index","native_ordinal":0,"body_sha256":"sha256:source","decoded_start":313,"decoded_end":327,"expression":"input0[offset]"}]`
	want := `[{"activity_id":"gooo://go/strings/Count","kind":"index","native_ordinal":0,"body_sha256":"sha256:source","decoded_start":313,"decoded_end":327,"expression":"input0[offset]"}]`
	encode := func(path, raw string) ([]byte, error) {
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		return normalizeResultValue(decoder, []byte(raw), path, profile.Nodes, 0)
	}
	encoded, err := encode(readPath, good)
	if err != nil || string(encoded) != want {
		t.Fatalf("source-declared read order/kinds: %s %v", encoded, err)
	}
	for _, value := range []struct{ path, raw string }{
		{"$.runtime.traces.*.local_uses.*.reason", `"UNINITIALIZED"`},
		{"$.runtime.traces.*.local_write_dropped", `1`},
		{"$.runtime.traces.*.local_use_dropped", `1`},
	} {
		if _, err := encode(value.path, value.raw); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]struct{ path, raw string }{
		"wrong typed field order":           {readPath, `[{"expression":"input0[offset]","decoded_end":327,"decoded_start":313,"body_sha256":"sha256:source","native_ordinal":0,"kind":"index","activity_id":"gooo://go/strings/Count"}]`},
		"unknown read field":                {readPath, strings.Replace(good, `"expression":`, `"untrusted":0,"expression":`, 1)},
		"decimal read ordinal":              {readPath, strings.Replace(good, `"native_ordinal":0`, `"native_ordinal":0.0`, 1)},
		"string read offset":                {readPath, strings.Replace(good, `"decoded_start":313`, `"decoded_start":"313"`, 1)},
		"null read expression":              {readPath, strings.Replace(good, `"expression":"input0[offset]"`, `"expression":null`, 1)},
		"read array replaced by object":     {readPath, `{}`},
		"reason type":                       {"$.runtime.traces.*.local_uses.*.reason", `1`},
		"decimal local-use dropped count":   {"$.runtime.traces.*.local_use_dropped", `1.0`},
		"string local-use dropped count":    {"$.runtime.traces.*.local_use_dropped", `"1"`},
		"decimal dropped count":             {"$.runtime.traces.*.local_write_dropped", `1.0`},
		"string dropped count":              {"$.runtime.traces.*.local_write_dropped", `"1"`},
		"adjacent unsupported read payload": {"$.runtime.traces.*.reads", `[{}]`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := encode(value.path, value.raw); err == nil {
				t.Fatal("accepted unsupported shape")
			}
		})
	}
}
