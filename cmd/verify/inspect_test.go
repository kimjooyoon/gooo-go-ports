package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Reduced from an actual pinned-compiler receipt. Only inspector fields remain.
func fixture(t *testing.T) ([]byte, suite, expectedIdentity) {
	t.Helper()
	raw, err := os.ReadFile("testdata/minimal-receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var e envelope
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	s := suite{Package: "go/unicode/utf16", Activity: "DecodeRune", Cases: []testCase{{Inputs: []any{int64(-1), int64(-1)}, Expected: int64(65533)}}}
	identity := expectedIdentity{CompilerRevision: "366ee6ef8f9d2b9b57331dfd26b6f908c0289851", GoVersion: "go1.27.2", ManifestDigest: e.ManifestDigest, CasesDigest: e.CasesDigest}
	return raw, s, identity
}
func TestInspectRejectsMisleadingEvidence(t *testing.T) {
	raw, s, identity := fixture(t)
	if _, err := inspect(raw, s, false, identity); err != nil {
		t.Fatalf("valid fixture: %v", err)
	}
	tests := map[string]func(*envelope){
		"wrong actual despite PASS": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Actual = json.RawMessage(`0`) },
		"wrong expectation":         func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Expected = json.RawMessage(`0`) },
		"wrong input":               func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = json.RawMessage(`55296`) },
		"reordered ports": func(e *envelope) {
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.Inputs[0], d.Inputs[1] = d.Inputs[1], d.Inputs[0]
		},
		"missing passed": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Passed = nil },
		"missing counts": func(e *envelope) { e.Result.Runtime.Total = nil },
		"wrong count":    func(e *envelope) { n := 2; e.Result.Runtime.Total = &n },
		"missing trace":  func(e *envelope) { e.Result.Runtime.Traces = nil },
		"duplicate trace": func(e *envelope) {
			e.Result.Runtime.Traces = append(e.Result.Runtime.Traces, e.Result.Runtime.Traces[0])
		},
		"wrong case index":        func(e *envelope) { e.Result.Runtime.Traces[0].Index = 1 },
		"wrong activity":          func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ID = "another" },
		"wrong entry":             func(e *envelope) { e.Result.Program.Entry.Activity = "EncodeRune" },
		"wrong compiler":          func(e *envelope) { e.Result.Runtime.Producer = "different" },
		"wrong toolchain":         func(e *envelope) { e.Result.Runtime.GoVersion = "go version go1.26.5 darwin/arm64" },
		"incomplete runtime":      func(e *envelope) { e.Result.Runtime.Stage = "FAILED" },
		"runtime failure":         func(e *envelope) { e.Result.Runtime.Failure = json.RawMessage(`{"reason":"panic"}`) },
		"wrong manifest":          func(e *envelope) { e.ManifestDigest = "different" },
		"wrong cases":             func(e *envelope) { e.CasesDigest = "different" },
		"wrong generated program": func(e *envelope) { e.Result.Runtime.Generated = "different" },
		"no runtime replay":       func(e *envelope) { e.Result.Runtime.Replayed = false },
		"model called":            func(e *envelope) { n := 1; e.Result.Runtime.Calls = &n },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			var e envelope
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			change(&e)
			mutated, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = inspect(mutated, s, false, identity); err == nil {
				t.Fatal("accepted invalid evidence")
			}
		})
	}
	if _, err := inspect(raw, s, true, identity); err == nil {
		t.Fatal("accepted execution as saved replay")
	}
}
func TestNumericComparisonPreservesInt64(t *testing.T) {
	if equalValue(json.RawMessage(`9007199254740993`), int64(9007199254740992)) {
		t.Fatal("rounded distinct integers")
	}
	if !equalValue(json.RawMessage(`9223372036854775807`), int64(9223372036854775807)) {
		t.Fatal("lost int64 maximum")
	}
	if equalValue(json.RawMessage(`1 2`), 1) {
		t.Fatal("accepted trailing JSON")
	}
}
func TestCorpusDomainsAndCoverage(t *testing.T) {
	all, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 15 {
		t.Fatal("expected thirteen port functions and two consumers")
	}
	total, originalTotal := 0, 0
	for _, s := range all {
		total += len(s.Cases)
		if s.Package != "go/math/bits" && s.Package != "app/byte-summary" {
			originalTotal += len(s.Cases)
		}
		for _, c := range s.Cases {
			for _, v := range c.Inputs {
				n := v.(int64)
				if s.Package == "go/unicode/utf16" || (s.Package == "go/unicode/utf8" && s.Activity != "RuneStart") {
					if n < -2147483648 || n > 2147483647 {
						t.Fatal("rune oracle would truncate", n)
					}
				}
			}
		}
		if s.Activity == "RuneStart" {
			seen := map[int64]bool{}
			for _, c := range s.Cases {
				n := c.Inputs[0].(int64)
				if n < 0 || n > 255 || seen[n] {
					t.Fatal("invalid byte corpus", n)
				}
				seen[n] = true
			}
			if len(seen) != 256 {
				t.Fatal("byte domain is not exhaustive")
			}
		}
	}
	if originalTotal != 1861 {
		t.Fatalf("changed original corpus count: %d", originalTotal)
	}
	if total != 3147 {
		t.Fatalf("changed corpus count: %d", total)
	}
}
