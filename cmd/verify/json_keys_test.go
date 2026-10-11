package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInspectRejectsDuplicateJSONKeys(t *testing.T) {
	raw, s, identity := fixture(t)
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	raw = compact.Bytes()
	if _, err := inspect(raw, s, false, identity); err != nil {
		t.Fatalf("valid fixture: %v", err)
	}
	mutations := []struct{ name, from, to string }{
		{"conflicting decision", `"decision":"PASS"`, `"decision":"FAIL_CLOSED","decision":"PASS"`},
		{"escaped decision", `"decision":"PASS"`, `"\u0064ecision":"FAIL_CLOSED","decision":"PASS"`},
		{"identical decision", `"decision":"PASS"`, `"decision":"PASS","decision":"PASS"`},
		{"nested counter", `"finite_passed":1`, `"finite_passed":0,"finite_passed":1`},
		{"delivery output", `"actual":65533`, `"actual":0,"actual":65533`},
		{"input value", `"value":-1`, `"value":0,"value":-1`},
		{"ignored extension", `"decision":"PASS"`, `"extension":{"x":0,"x":1},"decision":"PASS"`},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			if !bytes.Contains(raw, []byte(m.from)) {
				t.Fatal("mutation anchor missing")
			}
			mutated := bytes.Replace(raw, []byte(m.from), []byte(m.to), 1)
			if !json.Valid(mutated) {
				t.Fatal("mutation must remain syntactically valid JSON")
			}
			if _, err := inspect(mutated, s, false, identity); err == nil {
				t.Fatal("accepted receipt with duplicate JSON keys")
			} else if !strings.Contains(err.Error(), "duplicate JSON key") {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

func TestInspectRejectsDuplicateKeysInSavedReplay(t *testing.T) {
	raw, s, identity := fixture(t)
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	zero := 0
	e.Result.Replay = &struct {
		Calls *int `json:"model_calls"`
	}{Calls: &zero}
	e.ReplayedFrom = "sha256:" + strings.Repeat("a", 64)
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(raw, s, true, identity); err != nil {
		t.Fatalf("valid replay fixture: %v", err)
	}
	mutated := append([]byte(`{"replayed_from_sha256":"wrong",`), raw[1:]...)
	if _, err := inspect(mutated, s, true, identity); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
		t.Fatalf("expected duplicate replay binding rejection, got %v", err)
	}
}

func TestValueComparisonRejectsDuplicateKeys(t *testing.T) {
	want := map[string]any{"x": int64(9007199254740993)}
	for _, raw := range []string{
		`{"x":0,"x":9007199254740993}`,
		`{"\u0078":0,"x":9007199254740993}`,
		`{"x":9007199254740993,"x":9007199254740993}`,
	} {
		if equalValue(json.RawMessage(raw), want) {
			t.Fatalf("accepted duplicate-key value: %s", raw)
		}
	}
	if !equalValue(json.RawMessage(`{"x":9007199254740993}`), want) {
		t.Fatal("lost exact integer value")
	}
}

func TestInspectStillAllowsUniqueUnknownFields(t *testing.T) {
	raw, s, identity := fixture(t)
	// Unrecognized metadata stays accepted; uniqueness is per object, not global.
	mutated := append([]byte(`{"extension":[{"x":1},{"x":2}],`), raw[1:]...)
	if _, err := inspect(mutated, s, false, identity); err != nil {
		t.Fatalf("changed unknown-field policy: %v", err)
	}
}
