package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestRotationCoverageAndExactBatchInputs(t *testing.T) {
	added, err := rotationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 2 || added.PortReferenceCases != 2472 || added.PortRejectionCases != 36 || added.ConsumerReferenceCases != 672 || added.ConsumerRejectionCases != 36 {
		t.Fatalf("unexpected rotation coverage: %+v", added)
	}
	for index, s := range added.Suites {
		seen := map[[2]int64]bool{}
		for _, c := range s.Cases {
			key := [2]int64{c.Inputs[0].(int64), c.Inputs[1].(int64)}
			if seen[key] {
				t.Fatalf("duplicate %s input %v", s.Activity, key)
			}
			seen[key] = true
		}
		for x := int64(0); x < 256; x++ {
			for k := int64(0); k < 8; k++ {
				if index == 1 && k != 1 {
					continue
				}
				if !seen[[2]int64{x, k}] {
					t.Fatalf("%s missing byte/rotation pair (%d,%d)", s.Activity, x, k)
				}
			}
		}
		for _, x := range rotationBoundaryBytes() {
			for _, k := range rotationBoundaryCounts() {
				if !seen[[2]int64{x, k}] {
					t.Fatalf("%s missing exact original boundary (%d,%d)", s.Activity, x, k)
				}
			}
		}
		rejected := any(-1)
		batchSize := rotationPortBatchSize
		if index == 1 {
			rejected, batchSize = rejectedRotationInspection(), rotationInspectorBatchSize
		}
		for _, x := range rotationRejectedBytes() {
			for _, k := range []int64{0, -1, math.MinInt64, math.MaxInt64} {
				checkConsumerCase(t, s, []any{x, k}, rejected)
			}
		}
		maxBytes := 0
		for start := 0; start < len(s.Cases); start += batchSize {
			batch := s
			batch.Cases = s.Cases[start:min(start+batchSize, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(batch.Cases) > 128 || len(raw)+1 > 32<<10 {
				t.Fatalf("%s batch exceeds compiler budget: %d cases, %d bytes, %v", s.Activity, len(batch.Cases), len(raw)+1, err)
			}
			maxBytes = max(maxBytes, len(raw)+1)
			var document struct {
				Cases []struct {
					Inputs map[string]json.RawMessage `json:"inputs"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			for i, row := range document.Cases {
				if len(row.Inputs) != 2 {
					t.Fatal("serialized rotation input arity changed")
				}
				for j, n := range batch.Cases[i].Inputs {
					key := fmt.Sprintf("%s:%s.input%d", s.Package, s.Activity, j)
					if !equalValue(row.Inputs[key], n) {
						t.Fatalf("input normalized, narrowed or rounded before receipt: %s %v", key, n)
					}
				}
			}
		}
		t.Logf("%s: %d cases, maximum saved batch %d bytes", s.Activity, len(s.Cases), maxBytes)
	}
}

func TestRotationRejectsBeforeAnyOracleCall(t *testing.T) {
	valid := []any{int64(128), int64(math.MaxInt64)}
	invalid := [][]any{nil, {}, valid[:1], {int64(128), int64(0), int64(1)}, valid}
	for _, x := range []any{int64(-1), int64(256), int64(math.MinInt64), int64(math.MaxInt64), int64(9007199254740993), uint8(128), float64(128), "128", json.Number("128"), nil} {
		invalid = append(invalid, []any{x, int64(0)})
	}
	for _, k := range []any{int(1), float64(1), "1", json.Number("1"), uint64(math.MaxUint64), nil} {
		invalid = append(invalid, []any{int64(128), k})
	}
	for _, bad := range invalid {
		calls := 0
		_, err := rotationSuite("test", "rotation", [][]any{valid, bad}, func(uint8, int) any { calls++; return nil })
		if err == nil || calls != 0 {
			t.Fatalf("invalid roster reached oracle: %v calls=%d err=%v", bad, calls, err)
		}
	}
	if _, err := rotationSuite("test", "rotation", nil, func(uint8, int) any { t.Fatal("empty input called oracle"); return nil }); err == nil {
		t.Fatal("empty corpus accepted")
	}
	inputs := [][]any{{int64(0), int64(math.MinInt64)}, {int64(255), int64(math.MaxInt64)}, {int64(128), int64(9007199254740993)}, {int64(1), int64(-9007199254740993)}}
	var observed [][]any
	s, err := rotationSuite("test", "rotation", inputs, func(x uint8, k int) any {
		observed = append(observed, []any{int64(x), int64(k)})
		return nil
	})
	if err != nil || !reflect.DeepEqual(observed, inputs) {
		t.Fatalf("accepted domain changed before oracle: %v %v", observed, err)
	}
	inputs[0][1] = int64(0)
	if s.Cases[0].Inputs[1] != int64(math.MinInt64) {
		t.Fatal("saved original input aliases caller storage")
	}
}

func TestRotationNamedResultsAndConsumerProperties(t *testing.T) {
	added, err := rotationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ x, k, want int64 }{
		{128, 1, 1}, {1, -1, 128}, {85, 1, 170}, {170, -1, 85},
		{128, math.MinInt64, 128}, {128, math.MaxInt64, 64},
		{128, 9007199254740993, 1}, {128, -9007199254740993, 64},
	} {
		checkConsumerCase(t, added.Suites[0], []any{c.x, c.k}, c.want)
	}
	checkConsumerCase(t, added.Suites[1], []any{int64(128), int64(-9007199254740993)}, map[string]any{
		"in_byte_domain": true, "normalized_rotation": 7, "rotated": 64, "restored": 128, "ones_before": 1, "ones_after": 1,
	})
	for _, c := range added.Suites[1].Cases {
		v := c.Expected.(map[string]any)
		if v["in_byte_domain"] != true {
			continue
		}
		if int64(v["restored"].(uint8)) != c.Inputs[0].(int64) || v["ones_before"] != v["ones_after"] || v["normalized_rotation"].(int) < 0 || v["normalized_rotation"].(int) > 7 {
			t.Fatalf("independent Go consumer violates inverse/population count property: %v", c)
		}
	}
}

func TestRotationPreservesExisting5510Cases(t *testing.T) {
	legacy := func() []suite {
		base, err := corpus()
		if err != nil {
			t.Fatal(err)
		}
		unicode, err := unicodeInspectorCorpus(base)
		if err != nil {
			t.Fatal(err)
		}
		colors, err := colorCorpus()
		if err != nil {
			t.Fatal(err)
		}
		return append(append(base, unicode.Suites...), colors.Suites...)
	}
	before := legacy()
	total := 0
	for _, s := range before {
		total += len(s.Cases)
	}
	if total != 5510 {
		t.Fatalf("existing case count changed: %d", total)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from the unchanged e383513 corpus before adding rotation suites.
	const existing = "2fcd28282b79f82108fe25bdb968b6280f9e28de427a906e691a5f2f3d7a6552"
	if hash != existing {
		t.Fatalf("existing 5510-case corpus changed: %s", hash)
	}
	if _, err := rotationCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, legacy()) {
		t.Fatal("rotation construction mutated old corpus")
	}
}
