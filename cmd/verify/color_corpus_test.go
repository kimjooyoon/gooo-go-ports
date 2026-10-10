package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestColorCorpusCoverageAndBatchBudget(t *testing.T) {
	added, err := colorCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 3 || added.PortReferenceCases != 1300 || added.ConsumerReferenceCases != 637 || added.PortRejectionCases != 56 || added.ConsumerRejectionCases != 24 {
		t.Fatalf("unexpected color coverage: %+v", added)
	}
	for index, activity := range []string{"RGBToCMYK", "CMYKToRGB", "InspectRGB"} {
		s := added.Suites[index]
		if s.Activity != activity {
			t.Fatalf("wrong suite: %s", s.Activity)
		}
		seen := map[string]bool{}
		for _, c := range s.Cases {
			key := string(mustJSON(t, c.Inputs))
			if seen[key] {
				t.Fatalf("duplicate %s input %s", activity, key)
			}
			seen[key] = true
			inDomain := true
			for _, input := range c.Inputs {
				n := input.(int64)
				inDomain = inDomain && n >= 0 && n <= 255
			}
			if index == 2 && c.Expected.(map[string]any)["in_byte_domain"] != inDomain {
				t.Fatalf("wrong domain classification for %s", key)
			}
		}
		// All 256 gray ramp inputs are present, in addition to the finite grid.
		for n := int64(0); n < 256; n++ {
			input := []any{n, n, n}
			if index == 1 {
				input = []any{int64(0), int64(0), int64(0), n}
			}
			if !seen[string(mustJSON(t, input))] {
				t.Fatalf("%s missing gray %d", activity, n)
			}
		}
		arity := 3
		rejected := rejectedCMYK()
		if index == 1 {
			arity, rejected = 4, rejectedRGB()
		} else if index == 2 {
			rejected = rejectedColorInspection()
		}
		for _, n := range colorRejectedInputs() {
			for position := 0; position < arity; position++ {
				input := []any{int64(12), int64(34), int64(56), int64(78)}[:arity]
				input[position] = n
				checkConsumerCase(t, s, input, rejected)
			}
		}
		batchSize := colorPortBatchSize
		if index == 2 {
			batchSize = colorInspectorBatchSize
		}
		maxBytes := 0
		for start := 0; start < len(s.Cases); start += batchSize {
			batch := s
			batch.Cases = s.Cases[start:min(start+batchSize, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatalf("%s batch exceeds 32KiB: %d bytes, %v", activity, len(raw)+1, err)
			}
			maxBytes = max(maxBytes, len(raw)+1)
			// Read the exact saved representation with UseNumber. This catches
			// narrowing, Number precision loss and unexpected input key shape.
			var document struct {
				Cases []struct {
					Inputs map[string]json.RawMessage `json:"inputs"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			for i, row := range document.Cases {
				if len(row.Inputs) != arity {
					t.Fatal("serialized input arity changed")
				}
				for j, n := range batch.Cases[i].Inputs {
					key := fmt.Sprintf("%s:%s.input%d", s.Package, s.Activity, j)
					if !equalValue(row.Inputs[key], n) {
						t.Fatalf("serialized input lost shape or precision: %s %v", key, n)
					}
				}
			}
		}
		t.Logf("%s: %d cases, maximum saved batch %d bytes", activity, len(s.Cases), maxBytes)
	}
	t.Logf("color references ports=%d consumer=%d; rejections ports=%d consumer=%d", added.PortReferenceCases, added.ConsumerReferenceCases, added.PortRejectionCases, added.ConsumerRejectionCases)
	t.Logf("Go oracle InspectRGB(142,83,202): %s", mustJSON(t, inspectRGBOracle([]uint8{142, 83, 202})))
}

func TestColorDoesNotChangeExistingCorpus(t *testing.T) {
	base, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	unicode, err := unicodeInspectorCorpus(base)
	if err != nil {
		t.Fatal(err)
	}
	before := mustJSON(t, append(base, unicode.Suites...))
	// Frozen from the unchanged 092c19a corpus before adding color suites.
	const existing = "b93a415a470aa68c770762c0facd4a955376ba7b43fdc751bbb6aa1f2c774ea7"
	if got := fmt.Sprintf("%x", sha256.Sum256(before)); got != existing {
		t.Fatalf("existing 3493-case corpus changed: %s", got)
	}
	if _, err := colorCorpus(); err != nil {
		t.Fatal(err)
	}
	afterBase, _ := corpus()
	afterUnicode, _ := unicodeInspectorCorpus(afterBase)
	if !reflect.DeepEqual(before, mustJSON(t, append(afterBase, afterUnicode.Suites...))) {
		t.Fatal("color construction changed existing inputs or expectations")
	}
	t.Logf("existing 3493-case corpus SHA256: %x", sha256.Sum256(before))
}

func TestColorTuplesRejectBeforeAnyConversion(t *testing.T) {
	for _, arity := range []int{3, 4} {
		valid := []any{int64(0), int64(1), int64(254), int64(255)}[:arity]
		invalid := [][]any{nil, {}, valid[:arity-1], append(append([]any(nil), valid...), int64(0))}
		for position := 0; position < arity; position++ {
			for _, n := range []any{int64(-1), int64(256), int64(math.MinInt64), int64(math.MaxInt64), int64(257), int64(4294967296), int64(9007199254740993), float64(1), "1", json.Number("1"), uint8(1), nil} {
				row := append([]any(nil), valid...)
				row[position] = n
				invalid = append(invalid, row)
			}
		}
		invalid = append(invalid, valid) // duplicate after an otherwise valid row
		for _, bad := range invalid {
			calls := 0
			_, err := byteTupleSuite("test", "color", arity, [][]any{valid, bad}, func([]uint8) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatalf("invalid tuple reached oracle: %v calls=%d err=%v", bad, calls, err)
			}
		}
		var received []uint8
		_, err := byteTupleSuite("test", "color", arity, [][]any{valid}, func(in []uint8) any { received = in; return nil })
		if err != nil || !reflect.DeepEqual(received, []uint8{0, 1, 254, 255}[:arity]) {
			t.Fatalf("byte endpoints changed: %v %v", received, err)
		}
	}
	for _, arity := range []int{0, 1, 2, 5} {
		if _, err := byteTupleSuite("test", "color", arity, [][]any{{int64(1)}}, func([]uint8) any { t.Fatal("invalid shape called oracle"); return nil }); err == nil {
			t.Fatal("accepted unsupported arity")
		}
	}
}

func TestColorExpectedNamedSemantics(t *testing.T) {
	added, err := colorCorpus()
	if err != nil {
		t.Fatal(err)
	}
	checkConsumerCase(t, added.Suites[0], []any{int64(0), int64(0), int64(0)}, map[string]any{"cyan": 0, "magenta": 0, "yellow": 0, "black": 255})
	checkConsumerCase(t, added.Suites[0], []any{int64(255), int64(255), int64(255)}, map[string]any{"cyan": 0, "magenta": 0, "yellow": 0, "black": 0})
	checkConsumerCase(t, added.Suites[1], []any{int64(0), int64(0), int64(0), int64(255)}, map[string]any{"red": 0, "green": 0, "blue": 0})
	checkConsumerCase(t, added.Suites[1], []any{int64(0), int64(0), int64(0), int64(0)}, map[string]any{"red": 255, "green": 255, "blue": 255})
	checkConsumerCase(t, added.Suites[2], []any{int64(142), int64(83), int64(202)}, map[string]any{
		"in_byte_domain": true, "cyan": 75, "magenta": 150, "yellow": 0, "black": 53,
		"red": 143, "green": 83, "blue": 202, "red_delta": 1, "green_delta": 0, "blue_delta": 0, "exact": false,
	})
	for _, c := range added.Suites[2].Cases {
		if c.Expected.(map[string]any)["in_byte_domain"] != true {
			continue
		}
		v := c.Expected.(map[string]any)
		exact := true
		for i, field := range []string{"red", "green", "blue"} {
			delta := int64(v[field].(uint8)) - c.Inputs[i].(int64)
			if v[field+"_delta"] != delta {
				t.Fatal("delta direction or integer width changed")
			}
			exact = exact && delta == 0
		}
		if v["exact"] != exact {
			t.Fatal("round-trip flag disagrees with differences")
		}
	}
}
