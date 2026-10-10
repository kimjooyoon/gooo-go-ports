package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestUnicodeInspectorPreservesLegacyCorpus(t *testing.T) {
	base, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	before := mustJSON(t, base)
	// Recorded from a58b55b before adding the consumer: all 3,147 inputs,
	// expectations, suite identities and domains, not merely the case count.
	const legacy = "57b1c397041b05dc49627ea11f974037dae5261ca89702e662d1ca144ae2bd1d"
	if got := fmt.Sprintf("%x", sha256.Sum256(before)); got != legacy {
		t.Fatalf("legacy corpus changed: %s", got)
	}
	added, err := unicodeInspectorCorpus(base)
	if err != nil {
		t.Fatal(err)
	}
	if added.ReferenceCases != 334 || added.RejectionCases != 12 || len(added.Suites) != 2 {
		t.Fatalf("wrong consumer coverage: %d Go references, %d rejections, %d activities", added.ReferenceCases, added.RejectionCases, len(added.Suites))
	}
	if !reflect.DeepEqual(before, mustJSON(t, base)) {
		t.Fatal("consumer construction mutated the legacy corpus")
	}
	for i, activity := range []string{"InspectScalar", "InspectPair"} {
		s := added.Suites[i]
		if s.Package != "app/unicode-inspector" || s.Activity != activity {
			t.Fatalf("incorrect consumer identity: %s:%s", s.Package, s.Activity)
		}
		seen := map[string]bool{}
		for _, c := range s.Cases {
			key := string(mustJSON(t, c.Inputs))
			if seen[key] {
				t.Fatalf("duplicate consumer input: %s %s", activity, key)
			}
			seen[key] = true
			inDomain := true
			for _, input := range c.Inputs {
				n := input.(int64)
				inDomain = inDomain && n >= math.MinInt32 && n <= math.MaxInt32
			}
			if c.Expected.(map[string]any)["in_rune_domain"] != inDomain {
				t.Fatalf("wrong domain result for original input: %s %s", activity, key)
			}
		}
		for start := 0; start < len(s.Cases); start += unicodeInspectorBatchSize {
			batch := s
			batch.Cases = s.Cases[start:min(start+unicodeInspectorBatchSize, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatalf("%s batch exceeds pinned runtime case-file budget: %d bytes, %v", activity, len(raw)+1, err)
			}
		}
	}
}

func TestUnicodeInspectorReplacementAndPairSemantics(t *testing.T) {
	base, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	added, err := unicodeInspectorCorpus(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input                   int64
		valid, surrogate, pair  bool
		bytes, units, high, low int
	}{
		{65, true, false, false, 1, 1, 65533, 65533},
		{65533, true, false, false, 3, 1, 65533, 65533},
		{65535, true, false, false, 3, 1, 65533, 65533},
		{55296, false, true, false, -1, -1, 65533, 65533},
		{57343, false, true, false, -1, -1, 65533, 65533},
		{65536, true, false, true, 4, 2, 55296, 56320},
		{1114111, true, false, true, 4, 2, 56319, 57343},
		{math.MinInt32, false, false, false, -1, -1, 65533, 65533},
		{math.MaxInt32, false, false, false, -1, -1, 65533, 65533},
	} {
		want := map[string]any{"in_rune_domain": true, "valid_scalar": test.valid, "is_surrogate": test.surrogate,
			"needs_pair": test.pair, "utf8_bytes": test.bytes, "utf16_units": test.units, "high": test.high, "low": test.low}
		checkConsumerCase(t, added.Suites[0], []any{test.input}, want)
	}
	for _, test := range []struct {
		high, low             int64
		valid                 bool
		decoded, bytes, units int
	}{
		{55296, 56320, true, 65536, 4, 2},
		{56319, 57343, true, 1114111, 4, 2},
		{55357, 56832, true, 128512, 4, 2},
		{65533, 65533, false, 65533, 3, 1},
		{56320, 55296, false, 65533, 3, 1},
		{55296, 56319, false, 65533, 3, 1},
		{56320, 57343, false, 65533, 3, 1},
		{55295, 56320, false, 65533, 3, 1},
		{55296, 57344, false, 65533, 3, 1},
		{0x1D800, 0xDC00, false, 65533, 3, 1},
		{0xD800, 0x1DC00, false, 65533, 3, 1},
		{math.MinInt32, 0xDC00, false, 65533, 3, 1},
		{0xD800, math.MinInt32, false, 65533, 3, 1},
	} {
		want := map[string]any{"in_rune_domain": true, "valid_pair": test.valid, "decoded_scalar": test.decoded,
			"decoded_utf8_bytes": test.bytes, "decoded_utf16_units": test.units}
		checkConsumerCase(t, added.Suites[1], []any{test.high, test.low}, want)
	}
	// Every out-of-domain input is retained as int64 in native cases. In
	// particular +/-2^63 must not become zero or -1 before reaching Gooo.
	for _, n := range []int64{math.MinInt64, math.MinInt32 - 1, math.MaxInt32 + 1, math.MaxInt64} {
		checkConsumerCase(t, added.Suites[0], []any{n}, rejectedScalar())
		checkConsumerCase(t, added.Suites[1], []any{n, int64(0xDC00)}, rejectedPair())
		checkConsumerCase(t, added.Suites[1], []any{int64(0xD800), n}, rejectedPair())
	}
}

func checkConsumerCase(t *testing.T, s suite, input []any, want any) {
	t.Helper()
	for _, c := range s.Cases {
		if reflect.DeepEqual(c.Inputs, input) {
			if !equalValue(mustJSON(t, c.Expected), want) {
				t.Fatalf("%s(%v): got %v; want %v", s.Activity, input, c.Expected, want)
			}
			return
		}
	}
	t.Fatalf("missing %s input %v", s.Activity, input)
}

func TestRuneConsumerRejectsBeforeOracleConversion(t *testing.T) {
	for _, invalid := range [][]any{
		{}, {int64(0), int64(1)}, {int64(math.MinInt64)}, {int64(math.MaxInt64)},
		{int64(math.MinInt32 - 1)}, {int64(math.MaxInt32 + 1)}, {float64(65)}, {"65"}, {json.Number("65")},
	} {
		calls := 0
		_, err := runeConsumerSuite("InspectScalar", 1, [][]any{{int64(65)}, invalid}, func([]rune) any {
			calls++
			return nil
		})
		if err == nil || calls != 0 {
			t.Fatalf("invalid corpus %v reached Go oracle: calls=%d err=%v", invalid, calls, err)
		}
	}
	for position := 0; position < 2; position++ {
		input := []any{int64(55296), int64(56320)}
		input[position] = int64(math.MaxInt32 + 1)
		calls := 0
		_, err := runeConsumerSuite("InspectPair", 2, [][]any{input}, func([]rune) any { calls++; return nil })
		if err == nil || calls != 0 {
			t.Fatalf("pair input%d was narrowed before rejection", position)
		}
	}
	var received []rune
	_, err := runeConsumerSuite("InspectScalar", 1, [][]any{{int64(math.MinInt32)}, {int64(math.MaxInt32)}}, func(input []rune) any {
		received = append(received, input[0])
		return nil
	})
	if err != nil || !reflect.DeepEqual(received, []rune{math.MinInt32, math.MaxInt32}) {
		t.Fatalf("rune endpoints changed: %v, %v", received, err)
	}
}
