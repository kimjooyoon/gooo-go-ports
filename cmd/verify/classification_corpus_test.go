package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"unicode"
)

func classificationCasesByInput(t *testing.T, s suite) map[int64]testCase {
	t.Helper()
	result := map[int64]testCase{}
	for _, c := range s.Cases {
		if len(c.Inputs) != 1 {
			t.Fatalf("%s: wrong input arity", s.Activity)
		}
		n, ok := c.Inputs[0].(int64)
		if _, exists := result[n]; !ok || exists {
			t.Fatalf("%s: duplicate or inexact original input %v", s.Activity, c.Inputs)
		}
		result[n] = c
	}
	return result
}

func TestClassificationRangeCoverageAndConsumerUnion(t *testing.T) {
	added, err := classificationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 3 {
		t.Fatalf("expected two ports and one consumer: %+v", added)
	}
	if len(added.Suites[0].Cases) != 4140 || len(added.Suites[1].Cases) != 927 || added.PortReferenceCases != 5043 || added.ConsumerReferenceCases != 4570 {
		t.Fatalf("pinned finite classification coverage changed: %+v", added)
	}
	consumer := classificationCasesByInput(t, added.Suites[2])
	union := map[int64]bool{}
	portCases := 0
	for i, table := range []*unicode.RangeTable{unicode.Letter, unicode.Digit} {
		s := added.Suites[i]
		if s.Package != "go/unicode" || s.Activity != []string{"IsLetter", "IsDigit"}[i] {
			t.Fatalf("wrong port identity: %s:%s", s.Package, s.Activity)
		}
		seen := classificationCasesByInput(t, s)
		portCases += len(seen)
		require := func(n int64) {
			t.Helper()
			if _, ok := seen[n]; !ok {
				t.Fatalf("%s missing coverage target U+%04X", s.Activity, n)
			}
		}
		for n := int64(0); n < 256; n++ {
			require(n)
		}
		checkRange := func(lo, hi, stride int64) {
			for _, edge := range []int64{lo, hi} {
				for delta := int64(-1); delta <= 1; delta++ {
					require(edge + delta)
				}
			}
			if seen[lo].Expected != true || seen[hi].Expected != true {
				t.Fatalf("%s range endpoints must be members: %X..%X", s.Activity, lo, hi)
			}
			if lo+stride <= hi {
				require(lo + stride)
				if seen[lo+stride].Expected != true {
					t.Fatalf("%s lost next stride member: %X", s.Activity, lo+stride)
				}
			}
			if stride > 1 {
				for _, hole := range []int64{lo + 1, hi - 1} {
					if hole > lo && hole < hi && seen[hole].Expected != false {
						t.Fatalf("%s filled a stride hole: %X", s.Activity, hole)
					}
				}
			}
		}
		for _, r := range table.R16 {
			checkRange(int64(r.Lo), int64(r.Hi), int64(r.Stride))
		}
		for _, r := range table.R32 {
			checkRange(int64(r.Lo), int64(r.Hi), int64(r.Stride))
		}
		for _, n := range classificationBoundaryInputs() {
			require(n)
		}
		for n := range seen {
			union[n] = true
			if _, ok := consumer[n]; !ok {
				t.Fatalf("consumer omitted %s input %d", s.Activity, n)
			}
		}
	}
	if added.Suites[2].Package != "app/identifier-rune" || added.Suites[2].Activity != "Classify" || len(consumer) != len(union) {
		t.Fatal("consumer does not contain exactly the port-input union")
	}
	if added.PortReferenceCases+added.PortRejectionCases != portCases || added.ConsumerReferenceCases+added.ConsumerRejectionCases != len(consumer) || added.PortRejectionCases != 24 || added.ConsumerRejectionCases != 12 {
		t.Fatalf("reference/rejection accounting differs: %+v", added)
	}
	again, err := classificationCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("classification corpus is nondeterministic", err)
	}
	t.Logf("IsLetter reference=%d; IsDigit reference=%d; port reference=%d rejection=%d; consumer reference=%d rejection=%d", len(added.Suites[0].Cases)-12, len(added.Suites[1].Cases)-12, added.PortReferenceCases, added.PortRejectionCases, added.ConsumerReferenceCases, added.ConsumerRejectionCases)
}

func TestClassificationNamedSemanticsAndRejections(t *testing.T) {
	added, err := classificationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		input                         int64
		letter, digit, start, proceed bool
	}{
		{'_', false, false, true, true},
		{'A', true, false, true, true}, {'z', true, false, true, true},
		{0xaa, true, false, true, true}, {0xb5, true, false, true, true},
		{0xab, false, false, false, false},
		{0x905, true, false, true, true}, {0x4e2d, true, false, true, true},
		{0xac00, true, false, true, true}, {0x10400, true, false, true, true},
		{0x20000, true, false, true, true},
		{'0', false, true, false, true}, {'9', false, true, false, true},
		{0x660, false, true, false, true}, {0x967, false, true, false, true},
		{0xe50, false, true, false, true}, {0xff10, false, true, false, true},
		{0x104a0, false, true, false, true}, {0x11f50, false, true, false, true},
		{0x1d7d8, false, true, false, true}, {0x1e4f0, false, true, false, true},
		// Numeric categories Nl/No are not decimal digits (Nd) or letters (L).
		{0x216b, false, false, false, false}, {0xb2, false, false, false, false},
		{0xbd, false, false, false, false}, {0x2460, false, false, false, false},
		// Combining marks and join controls are outside this Go-style rune policy.
		{0x301, false, false, false, false}, {0x200c, false, false, false, false},
		{0x200d, false, false, false, false}, {0x1f600, false, false, false, false},
	} {
		input := []any{example.input}
		checkConsumerCase(t, added.Suites[0], input, example.letter)
		checkConsumerCase(t, added.Suites[1], input, example.digit)
		checkConsumerCase(t, added.Suites[2], input, map[string]any{"in_rune_domain": true, "can_start": example.start, "can_continue": example.proceed, "is_decimal_digit": example.digit})
	}
	for _, n := range []int64{math.MinInt32, -65488, -65471, -65441, -1, 0xd800, 0xdbff, 0xdc00, 0xdfff, 0x110000, 0x110030, 0x110041, 0x11005f, math.MaxInt32} {
		for _, s := range added.Suites[:2] {
			checkConsumerCase(t, s, []any{n}, false)
		}
		checkConsumerCase(t, added.Suites[2], []any{n}, map[string]any{"in_rune_domain": true, "can_start": false, "can_continue": false, "is_decimal_digit": false})
	}
	for _, n := range classificationRejectedInputs() {
		if n >= math.MinInt32 && n <= math.MaxInt32 {
			t.Fatalf("rejection input is in the Go domain: %d", n)
		}
		for _, s := range added.Suites[:2] {
			checkConsumerCase(t, s, []any{n}, false)
		}
		checkConsumerCase(t, added.Suites[2], []any{n}, rejectedIdentifierRune())
	}
}

func TestClassificationBatchesPreserveExactJSONInputsAndOutputs(t *testing.T) {
	added, err := classificationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		size := classificationPortBatchSize
		if s.Package == "app/identifier-rune" {
			size = classificationConsumerBatchSize
		}
		maxBytes, batches := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 || len(batch.Cases) > 128 {
				t.Fatalf("%s compiler batch budget exceeded: %d %v", s.Activity, len(raw)+1, err)
			}
			var doc struct {
				Cases []struct {
					Inputs   map[string]json.RawMessage `json:"inputs"`
					Expected map[string]json.RawMessage `json:"expected"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Cases) != len(batch.Cases) {
				t.Fatal("serialized case roster changed")
			}
			key := s.Package + ":" + s.Activity
			for i, row := range doc.Cases {
				if len(row.Inputs) != 1 || !equalValue(row.Inputs[key], batch.Cases[i].Inputs[0]) || len(row.Expected) != 1 || !equalValue(row.Expected[key], batch.Cases[i].Expected) {
					t.Fatal("serialized original input or expectation changed")
				}
				if s.Package == "app/identifier-rune" {
					var fields map[string]bool
					if err := json.Unmarshal(row.Expected[key], &fields); err != nil || len(fields) != 4 {
						t.Fatalf("consumer must have exactly four Boolean fields: %s", row.Expected[key])
					}
					for _, name := range []string{"in_rune_domain", "can_start", "can_continue", "is_decimal_digit"} {
						if _, ok := fields[name]; !ok {
							t.Fatalf("missing consumer field %q", name)
						}
					}
				}
			}
			maxBytes = max(maxBytes, len(raw)+1)
			batches++
		}
		t.Logf("%s:%s: cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maxBytes)
	}
}

func TestClassificationValidatesWholeRosterBeforeOracle(t *testing.T) {
	valid := []any{int64('A')}
	bad := [][]any{nil, {}, {int64('A'), int64(0)}, valid}
	for _, n := range classificationRejectedInputs() {
		bad = append(bad, []any{n})
	}
	for _, value := range []any{int32('A'), int('A'), float64(65), "65", json.Number("65"), nil} {
		bad = append(bad, []any{value})
	}
	for _, input := range bad {
		calls := 0
		_, err := classificationSuite("test", "test", [][]any{valid, input}, func(rune) any { calls++; return false })
		if err == nil || calls != 0 {
			t.Fatalf("invalid original roster reached oracle: %v, calls=%d", input, calls)
		}
	}
	if _, err := classificationSuite("test", "test", nil, func(rune) any { t.Fatal("empty oracle call"); return nil }); err == nil {
		t.Fatal("empty roster accepted")
	}
	inputs := [][]any{{int64(math.MinInt32)}, {int64(math.MaxInt32)}}
	var observed []int64
	s, err := classificationSuite("test", "test", inputs, func(r rune) any { observed = append(observed, int64(r)); return false })
	if err != nil || !reflect.DeepEqual(observed, []int64{math.MinInt32, math.MaxInt32}) {
		t.Fatal("original int32 endpoints changed", err)
	}
	inputs[0][0] = int64(0)
	if s.Cases[0].Inputs[0] != int64(math.MinInt32) {
		t.Fatal("saved input aliases caller storage")
	}
}

func classificationLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	legacy := whitespaceLegacyCorpus(t)
	whitespace, err := whitespaceCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(legacy, whitespace.Suites...)
}

func TestClassificationPreservesExisting15121Cases(t *testing.T) {
	before := classificationLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	if count != 15121 {
		t.Fatalf("existing corpus count changed: %d", count)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from the unchanged f1e002c corpus before classification integration.
	const existing = "de28d188677016479ec921a92f5d6b6e76dd57ef8562e4492b105bb59c70acda"
	if hash != existing {
		t.Fatalf("existing 15121-case corpus changed: %s", hash)
	}
	if _, err := classificationCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, classificationLegacyCorpus(t)) {
		t.Fatal("classification construction mutated existing cases")
	}
}
