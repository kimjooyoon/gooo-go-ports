package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"unicode"
)

func simpleFoldCasesByInput(t *testing.T, s suite) map[[2]int64]testCase {
	t.Helper()
	result := map[[2]int64]testCase{}
	for _, c := range s.Cases {
		if len(c.Inputs) < 1 || len(c.Inputs) > 2 {
			t.Fatal("wrong SimpleFold input arity")
		}
		var key [2]int64
		for i, value := range c.Inputs {
			n, ok := value.(int64)
			if !ok {
				t.Fatal("inexact SimpleFold input", value)
			}
			key[i] = n
		}
		if _, ok := result[key]; ok {
			t.Fatal("duplicate SimpleFold input tuple", key)
		}
		result[key] = c
	}
	return result
}

func TestSimpleFoldCoverageAndCycleBound(t *testing.T) {
	added, err := simpleFoldCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 2 || added.Suites[0].Package != "go/unicode" || added.Suites[0].Activity != "SimpleFold" || added.Suites[1].Package != "app/simple-fold" || added.Suites[1].Activity != "Compare" {
		t.Fatal("wrong SimpleFold suite roster")
	}
	port := simpleFoldCasesByInput(t, added.Suites[0])
	consumer := simpleFoldCasesByInput(t, added.Suites[1])
	require := func(n int64) {
		t.Helper()
		if _, ok := port[[2]int64{n, 0}]; !ok {
			t.Fatalf("missing SimpleFold input %d (U+%04X)", n, n)
		}
	}
	for n := int64(0); n <= 255; n++ {
		require(n)
	}
	changed := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) != r {
			require(int64(r))
			changed++
		}
	}
	for _, cr := range unicode.CaseRanges {
		for _, edge := range []int64{int64(cr.Lo), int64(cr.Hi)} {
			for delta := int64(-1); delta <= 1; delta++ {
				require(edge + delta)
			}
		}
	}
	for _, n := range simpleFoldRuneBoundaries() {
		require(n)
	}
	cycles, allChanged, maximum, err := simpleFoldCycles()
	if err != nil || maximum != 4 || len(allChanged) != changed {
		t.Fatal("whole Unicode range cycle analysis failed", maximum, err)
	}
	cycleSizes := map[int]int{}
	for _, cycle := range cycles {
		cycleSizes[len(cycle)]++
		if len(cycle) > 2 {
			for _, left := range cycle {
				for _, right := range cycle {
					c, ok := consumer[[2]int64{int64(left), int64(right)}]
					if !ok || c.Expected.(map[string]any)["equivalent"] != true {
						t.Fatalf("consumer missing true member pair %X/%X", left, right)
					}
				}
			}
		}
	}
	if cycleSizes[2] == 0 || cycleSizes[3] == 0 || cycleSizes[4] == 0 {
		t.Fatal("lost required cycle lengths", cycleSizes)
	}
	if len(port) != added.PortReferenceCases+added.PortExtensionCases || len(consumer) != added.ConsumerReferenceCases+added.ConsumerRejectionCases || added.PortExtensionCases != 11 || added.ConsumerRejectionCases != 34 {
		t.Fatal("SimpleFold reference/extension counts changed", added)
	}
	if added.PortReferenceCases != 3505 || added.ConsumerReferenceCases != 497 || changed != 2994 || !reflect.DeepEqual(cycleSizes, map[int]int{2: 1455, 3: 24, 4: 3}) {
		t.Fatal("pinned SimpleFold finite coverage or cycle roster changed")
	}
	again, err := simpleFoldCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("nondeterministic SimpleFold corpus", err)
	}
	t.Logf("changed code points=%d; nontrivial cycles=%v; port reference=%d extension=%d; consumer reference=%d rejection=%d", changed, cycleSizes, added.PortReferenceCases, added.PortExtensionCases, added.ConsumerReferenceCases, added.ConsumerRejectionCases)
}

func TestSimpleFoldManualInputsAndExpectations(t *testing.T) {
	// Deliberately handwritten next members, not generated from either the Gooo
	// source or the Go oracle. Order matches the portable input-only example.
	manual := []struct {
		left, right, leftNext, rightNext int64
		domain, equivalent               bool
	}{
		{65, 97, 97, 65, true, true},
		{75, 8490, 107, 75, true, true},
		{115, 383, 383, 83, true, true},
		{931, 962, 962, 963, true, true},
		{920, 1012, 952, 920, true, true},
		{837, 8126, 921, 837, true, true},
		{5040, 43904, 43904, 5040, true, true},
		{66560, 66600, 66600, 66560, true, true},
		{304, 73, 304, 105, true, false},
		{305, 105, 305, 73, true, false},
		{223, 7838, 7838, 223, true, true},
		{223, 115, 7838, 383, true, false},
		{233, 101, 201, 69, true, false},
		{32, 32, 32, 32, true, true},
		{65, 66, 97, 98, true, false},
		{-1, -1, -1, -1, true, true},
		{-1, math.MaxInt32, -1, math.MaxInt32, true, false},
		{0xd800, 0xd800, 0xd800, 0xd800, true, true},
		{0xd800, 0xd801, 0xd800, 0xd801, true, false},
		{0x110000, 0x110000, 0x110000, 0x110000, true, true},
		{math.MinInt32, math.MinInt32, math.MinInt32, math.MinInt32, true, true},
		{4294967361, 65, 4294967361, 65, false, false},
		{97, -4294967231, 97, -4294967231, false, false},
		{math.MaxInt64, math.MinInt64, math.MaxInt64, math.MinInt64, false, false},
	}
	added, err := simpleFoldCorpus()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../examples/simple-fold/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema string                       `json:"schema"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Schema != "gooo/body-composition-inputs/v1" || len(doc.Inputs) != len(manual) {
		t.Fatal("invalid manual input-only document", err)
	}
	for i, row := range manual {
		input := []any{row.left, row.right}
		expected := map[string]any{"in_rune_domain": row.domain, "equivalent": row.equivalent, "left_next": row.leftNext, "right_next": row.rightNext}
		checkConsumerCase(t, added.Suites[1], input, expected)
		if len(doc.Inputs[i]) != 2 {
			t.Fatal("unexpected manual input keys")
		}
		for position, original := range input {
			if !equalValue(doc.Inputs[i][fmt.Sprintf("app/simple-fold:Compare.input%d", position)], original) {
				t.Fatalf("manual input%d row%d changed", position, i)
			}
		}
	}
	for _, n := range simpleFoldExtendedInputs() {
		if n >= math.MinInt32 && n <= math.MaxInt32 {
			t.Fatal("extension silently overlaps Go rune domain")
		}
		checkConsumerCase(t, added.Suites[0], []any{n}, n)
		for _, pair := range [][2]int64{{n, 'A'}, {'a', n}, {n, n}} {
			checkConsumerCase(t, added.Suites[1], []any{pair[0], pair[1]}, map[string]any{"in_rune_domain": false, "equivalent": false, "left_next": pair[0], "right_next": pair[1]})
		}
	}
}

func TestSimpleFoldBatchesPreserveExactJSON(t *testing.T) {
	added, err := simpleFoldCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		size := simpleFoldPortBatchSize
		if s.Package == "app/simple-fold" {
			size = simpleFoldConsumerBatchSize
		}
		maxBytes, batches := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 || len(batch.Cases) > 128 {
				t.Fatalf("SimpleFold batch budget exceeded: bytes=%d error=%v", len(raw)+1, err)
			}
			var doc struct {
				Cases []struct {
					Inputs   map[string]json.RawMessage `json:"inputs"`
					Expected map[string]json.RawMessage `json:"expected"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Cases) != len(batch.Cases) {
				t.Fatal("serialized SimpleFold roster changed", err)
			}
			key := s.Package + ":" + s.Activity
			for i, c := range batch.Cases {
				if len(doc.Cases[i].Inputs) != len(c.Inputs) || len(doc.Cases[i].Expected) != 1 || !equalValue(doc.Cases[i].Expected[key], c.Expected) {
					t.Fatal("serialized SimpleFold expectation changed")
				}
				for position, value := range c.Inputs {
					inputKey := key
					if len(c.Inputs) > 1 {
						inputKey = fmt.Sprintf("%s.input%d", key, position)
					}
					if !equalValue(doc.Cases[i].Inputs[inputKey], value) {
						t.Fatal("serialized SimpleFold original input changed")
					}
				}
				if s.Package == "app/simple-fold" {
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(doc.Cases[i].Expected[key], &fields); err != nil || len(fields) != 4 {
						t.Fatal("comparison record must have exactly four fields", err)
					}
					for _, name := range []string{"in_rune_domain", "equivalent"} {
						var flag bool
						if err := json.Unmarshal(fields[name], &flag); err != nil {
							t.Fatal("comparison Boolean field changed", name, err)
						}
					}
					for _, name := range []string{"left_next", "right_next"} {
						var n int64
						if err := json.Unmarshal(fields[name], &n); err != nil {
							t.Fatal("comparison exact int64 field changed", name, err)
						}
					}
				}
			}
			maxBytes = max(maxBytes, len(raw)+1)
			batches++
		}
		t.Logf("%s:%s cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maxBytes)
	}
}

func TestSimpleFoldValidatesWholeRosterBeforeOracle(t *testing.T) {
	for _, arity := range []int{1, 2} {
		valid := []any{int64('A'), int64('a')}[:arity]
		bad := [][]any{nil, {}, {int64('A'), int64('a'), int64('K')}, valid}
		for position := 0; position < arity; position++ {
			for _, value := range []any{int(65), int32(65), float64(65), "65", json.Number("65"), nil, int64(math.MinInt32 - 1), int64(math.MaxInt32 + 1), int64(math.MinInt64), int64(math.MaxInt64)} {
				input := append([]any(nil), valid...)
				input[position] = value
				bad = append(bad, input)
			}
		}
		for _, invalid := range bad {
			calls := 0
			_, err := simpleFoldSuite("test", "test", arity, [][]any{valid, invalid}, func([]rune) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatalf("bad original roster reached oracle: arity=%d input=%v calls=%d", arity, invalid, calls)
			}
		}
	}
	for _, arity := range []int{0, 1, 2, 3} {
		if _, err := simpleFoldSuite("test", "test", arity, nil, func([]rune) any { t.Fatal("empty oracle call"); return nil }); err == nil {
			t.Fatal("empty corpus accepted")
		}
	}
	inputs := [][]any{{int64(math.MinInt32), int64(math.MaxInt32)}}
	s, err := simpleFoldSuite("test", "test", 2, inputs, func(r []rune) any {
		if r[0] != math.MinInt32 || r[1] != math.MaxInt32 {
			t.Fatal("rune endpoints narrowed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs[0][0] = int64(0)
	if s.Cases[0].Inputs[0] != int64(math.MinInt32) {
		t.Fatal("saved original inputs alias caller storage")
	}
}

func simpleFoldLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	legacy := geometryLegacyCorpus(t)
	geometry, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(legacy, geometry.Suites...)
}

func TestSimpleFoldPreservesExisting26967Cases(t *testing.T) {
	before := simpleFoldLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	if count != 26967 {
		t.Fatalf("existing corpus count changed: %d", count)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from the unchanged 44a6a8 legacy corpus generators, before adding
	// SimpleFold suites to the verifier's roster.
	const existing = "623b6d1f970ede6d36fcf0dd0d8101e10128f33a32402f83a331cfe5b8d48add"
	if hash != existing {
		t.Fatalf("existing 26967-case corpus changed: %s", hash)
	}
	if _, err := simpleFoldCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, simpleFoldLegacyCorpus(t)) {
		t.Fatal("SimpleFold construction mutated existing cases")
	}
}
