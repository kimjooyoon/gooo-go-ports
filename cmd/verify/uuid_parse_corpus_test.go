package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Literal original-Go observations guard distinctions that a Bool-only parser
// or a Validate acceptance gate would lose. These are not native-run claims.
func TestUUIDParseOraclePreservesReturnedValueAndErrorPriority(t *testing.T) {
	const zero = "00000000-0000-0000-0000-000000000000"
	raw := strings.ReplaceAll(uuidStandardSample, "-", "")
	rows := []struct {
		input, canonical, message string
		parsed, invalidLength     bool
	}{
		{uuidStandardSample, uuidStandardSample, "", true, false},
		{"[" + uuidStandardSample + "]", uuidStandardSample, "", true, false},
		{"!" + uuidStandardSample + "?", uuidStandardSample, "", true, false},
		{"\x00" + uuidStandardSample + " ", uuidStandardSample, "", true, false},
		{raw[:30] + "0g", uuidStandardSample, "invalid UUID format", false, false},
		{uuidStandardSample[:34] + "0g", "00112233-4455-6677-8899-aabbccddee00", "invalid UUID format", false, false},
		{"g0" + raw[2:], "f0000000-0000-0000-0000-000000000000", "invalid UUID format", false, false},
		{"g" + uuidStandardSample[1:23] + "_" + uuidStandardSample[24:], zero, "invalid UUID format", false, false},
		{"not:uuid:" + uuidStandardSample, zero, `invalid urn prefix: "not:uuid:"`, false, false},
		{"12345678가" + strings.Repeat("a", 34), zero, `invalid urn prefix: "12345678\xea"`, false, false},
		{strings.Repeat("a", 1024), zero, "invalid UUID length: 1024", false, true},
		{zero, zero, "", true, false},
		{strings.Repeat("f", 8) + "-ffff-ffff-ffff-" + strings.Repeat("f", 12), "ffffffff-ffff-ffff-ffff-ffffffffffff", "", true, false},
	}
	for _, row := range rows {
		want := map[string]any{"parsed": row.parsed, "canonical_uuid": row.canonical, "error": row.message, "invalid_length": row.invalidLength}
		if got := uuidParseCanonicalOracle(row.input); !reflect.DeepEqual(got, want) {
			t.Fatalf("input %q: got %v, want %v", row.input, got, want)
		}
	}
}

func TestUUIDParseCorpusRetainsLegacyInputsAndParserAnchors(t *testing.T) {
	parsed, err := uuidParseCorpus()
	if err != nil {
		t.Fatal(err)
	}
	legacy := uuidCorpusInputs()
	anchors, err := uuidParseAnchors()
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 21745 || len(anchors) != 136 || parsed.LegacyReferenceInputs != len(legacy) || parsed.ParserAnchorInputs != len(anchors) {
		t.Fatal("established input inventories changed")
	}
	if parsed.Suite.Package != uuidParsePackage || parsed.Suite.Activity != "ParseCanonical" || parsed.ReferenceCases != len(parsed.Suite.Cases) || parsed.AddedReferenceInputs != parsed.ReferenceCases-len(legacy) {
		t.Fatal("parser inventory counters differ")
	}
	seen := map[string]bool{}
	for i, row := range parsed.Suite.Cases {
		text, err := readUUIDInput(row.Inputs)
		if err != nil || seen[text] {
			t.Fatal("duplicate or invalid parser original Text", err)
		}
		seen[text] = true
		if i < len(legacy) && !reflect.DeepEqual(row.Inputs, legacy[i]) {
			t.Fatal("prior UUID input order or bytes changed")
		}
		if !reflect.DeepEqual(row.Expected, uuidParseCanonicalOracle(text)) {
			t.Fatal("parser expectation is not original Go Parse + String")
		}
	}
	counts := map[string]int{}
	for _, anchor := range anchors {
		counts[anchor.Category]++
		if !seen[anchor.Text] {
			t.Fatal("parser-specific input missing", anchor.Name)
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"pilot12": 12, "partial_return": 24, "hyphen_priority": 24, "outer_38_bytes": 24, "length_domain": 8, "prefix_quote_utf8": 24, "uuid_value_patterns": 20}) {
		t.Fatal("known boundary coverage changed", counts)
	}
	for _, inputs := range [][][]any{{{strings.Repeat("x", 1025)}}, {{string([]byte{255})}}, {{uuidStandardSample}, {uuidStandardSample}}} {
		calls := 0
		if _, err := uuidReferenceSuite(uuidParsePackage, "ParseCanonical", inputs, func(string) any { calls++; return nil }); err == nil || calls != 0 {
			t.Fatal("invalid domain reached the original parser oracle")
		}
	}
	batchCount := 0
	for start := 0; start < len(parsed.Suite.Cases); {
		end, err := nativeBatchEnd(parsed.Suite, start, nativeBatchSize(parsed.Suite))
		if err != nil {
			t.Fatal(err)
		}
		batch := parsed.Suite
		batch.Cases = parsed.Suite.Cases[start:end]
		raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
		if err != nil || len(raw)+1 > 30000 || end-start > 128 {
			t.Fatal("serialized parser batch exceeds the actual byte/row cap", err)
		}
		batchCount++
		start = end
	}
	t.Logf("UUID ParseCanonical: legacy=%d anchors=%d added=%d cases=%d batches=%d", parsed.LegacyReferenceInputs, parsed.ParserAnchorInputs, parsed.AddedReferenceInputs, parsed.ReferenceCases, batchCount)
}

func TestUUIDParsePreservesEstablished56SuiteAndBatchCounts(t *testing.T) {
	suites := original57NativeSuites(t)
	if len(suites) != 57 || !isUUIDParseSuite(suites[56]) {
		t.Fatal("parser must be a new final suite")
	}
	total, batches := 0, 0
	for _, s := range suites[:56] {
		total += len(s.Cases)
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil {
				t.Fatal(err)
			}
			batches++
			start = end
		}
	}
	if total != 108835 || batches != 1416 {
		t.Fatal("established56 corpus or batch semantics changed", total, batches)
	}
}
