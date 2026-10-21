package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// This is an fresh saved actual205 receipt, not a relabeled839 fixture.
func TestCurrentGo1273TypedResultMatchesSavedOwnReplayParent(t *testing.T) {
	raw, err := os.ReadFile("testdata/current-go1273-execution.json")
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

// The encoder is an independent ordinary Go struct with the source205
// CompositionDelivery Input json.RawMessage tag. These literals do not share
// omission logic with the codec under test.
func TestCurrent205RawMessageOmitemptyPreservesEncodedZero(t *testing.T) {
	const path = "$.runtime.traces.*.deliveries.*"
	shape := map[string]profileShape{path: {Kinds: []string{"object"}, Fields: []profileField{
		{Name: "activity_id"}, {Name: "producer_id", Optional: true, OmitZero: true},
		{Name: "input", Optional: true, OmitZero: true}, {Name: "actual"},
	}}, path + ".activity_id": {Kinds: []string{"string"}}, path + ".producer_id": {Kinds: []string{"string"}}}
	type sourceDelivery struct {
		ActivityID string          `json:"activity_id"`
		ProducerID string          `json:"producer_id,omitempty"`
		Input      json.RawMessage `json:"input,omitempty"`
		Actual     json.RawMessage `json:"actual"`
	}
	normalize := func(raw []byte, current bool) ([]byte, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		return normalizeResultValueForProfile(d, raw, path, shape, 0, current)
	}
	for _, literal := range []string{`""`, `false`, `0`, `null`, `[]`, `{}`} {
		t.Run(literal, func(t *testing.T) {
			want, err := json.Marshal(sourceDelivery{ActivityID: "source-id", Input: json.RawMessage(literal), Actual: json.RawMessage(`false`)})
			if err != nil {
				t.Fatal(err)
			}
			got, err := normalize(want, true)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("encoded RawMessage changed: %s %v", got, err)
			}
			if _, err := normalize(want, false); err == nil {
				t.Fatal("archival omission behavior unexpectedly changed")
			}
		})
	}
	for name, raw := range map[string]string{
		"ordinary empty scalar": `{"activity_id":"source-id","producer_id":"","input":"","actual":false}`,
		"ordinary null scalar":  `{"activity_id":"source-id","producer_id":null,"input":"","actual":false}`,
		"unknown field":         `{"activity_id":"source-id","input":"","extra":false,"actual":false}`,
		"wrong field order":     `{"input":"","activity_id":"source-id","actual":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalize([]byte(raw), true); err == nil {
				t.Fatal("accepted unsupported ordinary omission or field")
			}
		})
	}
}
