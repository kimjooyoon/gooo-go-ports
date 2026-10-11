package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestInspectReceiptRejectsDuplicateKeys(t *testing.T) {
	e, binding := fixture(t)
	raw := marshal(t, e)
	prior := sha(raw)
	e.ReplayedFrom = prior
	zero := 0
	e.Result.Replay = &struct {
		Calls *int `json:"model_calls"`
	}{Calls: &zero}
	replay := marshal(t, e)

	for _, mode := range []struct {
		name, prior string
		raw         []byte
	}{{"execute", "", raw}, {"saved replay", prior, replay}} {
		t.Run(mode.name, func(t *testing.T) {
			t.Run("valid", func(t *testing.T) {
				got, _, err := inspectReceipt(mode.raw, binding, mode.prior)
				if err != nil || got != e.Result.Composition.Source {
					t.Fatalf("valid receipt: source preserved=%v, error=%v", got == e.Result.Composition.Source, err)
				}
			})
			for _, mutation := range []struct{ name, needle, replacement string }{
				{"root decision", `"Decision":"PASS"`, `"Decision":"FAIL","Decision":"PASS"`},
				{"escaped decision", `"Decision":"PASS"`, `"Dec\u0069sion":"FAIL","Decision":"PASS"`},
				{"same decision value", `"Decision":"PASS"`, `"Decision":"PASS","Decision":"PASS"`},
				{"delivery field value", `"Value":false`, `"Value":true,"Value":false`},
				{"delivery metadata", `"Deliveries":[{"activity_id":`, `"Deliveries":[{"activity_id":"wrong","activity_id":`},
				{"delivery unknown metadata", `"Deliveries":[{`, `"Deliveries":[{"audit_note":1,"audit_note":1,`},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					mutated := injectDuplicateKey(t, mode.raw, mutation.needle, mutation.replacement)
					if _, _, err := inspectReceipt(mutated, binding, mode.prior); err == nil {
						t.Fatal("accepted receipt containing duplicate JSON keys")
					}
				})
			}
		})
	}
}

// Inject directly into bytes: unmarshalling and remarshal would erase the
// duplicate key before the inspector sees it.
func injectDuplicateKey(t *testing.T, raw []byte, needle, replacement string) []byte {
	t.Helper()
	if !bytes.Contains(raw, []byte(needle)) {
		t.Fatalf("fixture does not contain mutation target %q", needle)
	}
	mutated := bytes.Replace(raw, []byte(needle), []byte(replacement), 1)
	if !json.Valid(mutated) {
		t.Fatal("mutation produced malformed JSON instead of a duplicate-key object")
	}
	return mutated
}

func TestSameJSONRejectsDuplicateObjectKeys(t *testing.T) {
	for _, test := range []struct{ name, duplicate, unambiguous string }{
		{"different values", `{"n":1,"n":2}`, `{"n":2}`},
		{"same values", `{"n":2,"n":2}`, `{"n":2}`},
		{"escaped key", `{"n":1,"\u006e":2}`, `{"n":2}`},
		{"object inside array", `[{"n":1,"n":2}]`, `[{"n":2}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			duplicate, unambiguous := []byte(test.duplicate), []byte(test.unambiguous)
			if sameJSON(duplicate, unambiguous) || sameJSON(unambiguous, duplicate) || sameJSON(duplicate, duplicate) {
				t.Fatal("accepted an ambiguous object in JSON comparison")
			}
		})
	}
}

func TestSameJSONKeepsNestedInt64Precision(t *testing.T) {
	const exact = `[{"n":9223372036854775807},{"n":-9223372036854775808},{"n":9007199254740993}]`
	const escaped = `[{"\u006e":9223372036854775807},{"n":-9223372036854775808},{"n":9007199254740993}]`
	if !sameJSON([]byte(exact), []byte(escaped)) {
		t.Fatal("lost exact int64 values or rejected the same key in separate objects")
	}
	for _, different := range []string{
		`[{"n":9223372036854775806},{"n":-9223372036854775808},{"n":9007199254740993}]`,
		`[{"n":9223372036854775807},{"n":-9223372036854775807},{"n":9007199254740993}]`,
		`[{"n":9223372036854775807},{"n":-9223372036854775808},{"n":9007199254740992}]`,
	} {
		if sameJSON([]byte(exact), []byte(different)) || sameJSON([]byte(different), []byte(exact)) {
			t.Fatal("collapsed distinct nested int64 values")
		}
	}
}
