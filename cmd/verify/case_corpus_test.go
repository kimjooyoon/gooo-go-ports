package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestCaseMappingCorpusCoverageAndOriginalDomains(t *testing.T) {
	observed, err := caseMappingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.Suites) != 5 {
		t.Fatal("suite roster")
	}
	refs, extensions, invalidRefs, invalidExtensions, consumerRefs, consumerExtensions := 0, 0, 0, 0, 0, 0
	for _, s := range observed.Suites {
		seen := map[[2]int64]bool{}
		for _, c := range s.Cases {
			which := int64(0)
			r := c.Inputs[len(c.Inputs)-1].(int64)
			if s.Activity == "To" {
				which = c.Inputs[0].(int64)
			}
			key := [2]int64{which, r}
			if seen[key] {
				t.Fatal("duplicate tuple", s.Activity, key)
			}
			seen[key] = true
			original := r >= math.MinInt32 && r <= math.MaxInt32
			if s.Package == "app/case-inspector" {
				var want map[string]any
				if original {
					consumerRefs++
					v := rune(r)
					want = map[string]any{"in_rune_domain": true, "valid_scalar": utf8.ValidRune(v), "original": r, "upper": int64(unicode.ToUpper(v)), "lower": int64(unicode.ToLower(v)), "title": int64(unicode.ToTitle(v))}
				} else {
					consumerExtensions++
					want = caseMappingExtendedReport(r)
				}
				if !reflect.DeepEqual(c.Expected, want) {
					t.Fatal("consumer oracle differs", r)
				}
				continue
			}
			var want int64
			if original {
				refs++
				switch s.Activity {
				case "To":
					want = int64(unicode.To(int(which), rune(r)))
				case "ToUpper":
					want = int64(unicode.ToUpper(rune(r)))
				case "ToLower":
					want = int64(unicode.ToLower(rune(r)))
				case "ToTitle":
					want = int64(unicode.ToTitle(rune(r)))
				}
			} else {
				extensions++
				want = r
			}
			if s.Activity == "To" && (which < 0 || which > 2) {
				want = int64(unicode.ReplacementChar)
				if original {
					invalidRefs++
				} else {
					invalidExtensions++
				}
			}
			if c.Expected != want {
				t.Fatal("oracle/domain precedence differs", s.Activity, key, c.Expected, want)
			}
		}
		modes := []int{0, 1, 2}
		if s.Activity != "To" && s.Activity != "Describe" {
			modes = []int{caseMappingMode(s.Activity)}
		}
		for _, mode := range modes {
			for r := rune(0); r <= unicode.MaxRune; r++ {
				if unicode.To(mode, r) == r {
					continue
				}
				key := [2]int64{0, int64(r)}
				if s.Activity == "To" {
					key[0] = int64(mode)
				}
				if !seen[key] {
					t.Fatal("missing changing rune", s.Activity, mode, r)
				}
			}
			for r := int64(0); r <= 255; r++ {
				key := [2]int64{0, r}
				if s.Activity == "To" {
					key[0] = int64(mode)
				}
				if !seen[key] {
					t.Fatal("missing Latin-1", s.Activity, mode, r)
				}
			}
			for _, r := range caseMappingBoundaryRunes() {
				key := [2]int64{0, r}
				if s.Activity == "To" {
					key[0] = int64(mode)
				}
				if !seen[key] {
					t.Fatal("missing range/parity/sample boundary", s.Activity, key)
				}
			}
		}
	}
	if refs != observed.PortReferenceCases || extensions != observed.PortExtensionCases || invalidRefs != observed.InvalidCaseReferenceCases || invalidExtensions != observed.InvalidCaseExtensionCases || consumerRefs != observed.ConsumerReferenceCases || consumerExtensions != observed.ConsumerExtensionCases {
		t.Fatal("summary counts differ")
	}
	t.Logf("ports refs=%d extensions=%d (invalidcase subsets refs=%d extensions=%d); consumer refs=%d extensions=%d", refs, extensions, invalidRefs, invalidExtensions, consumerRefs, consumerExtensions)
}

func TestCaseMappingRejectsWholeRosterBeforeAnyOracleCast(t *testing.T) {
	for _, activity := range []string{"To", "ToUpper", "ToLower", "ToTitle", "Describe"} {
		good := []any{int64('A')}
		if activity == "To" {
			good = []any{int64(0), int64('A')}
		}
		bad := [][]any{nil, {int64(0), int64(1), int64(2)}, append([]any(nil), good...)}
		for _, value := range []any{float64(65), json.Number("65"), "65", int(65), nil, int64(math.MinInt32 - 1), int64(math.MaxInt32 + 1), int64(4294967361), int64(math.MaxInt64)} {
			row := append([]any(nil), good...)
			row[len(row)-1] = value
			bad = append(bad, row)
		}
		if activity == "To" {
			for _, value := range []any{nil, "0", float64(0), json.Number("0"), int(0)} {
				bad = append(bad, []any{value, int64('A')})
			}
		}
		for _, row := range bad {
			calls := 0
			_, err := caseMappingReferenceSuite("go/unicode", activity, [][]any{good, row}, func(int, rune) any { calls++; return 0 })
			if err == nil || calls != 0 {
				t.Fatal("invalid later input reached rune conversion/oracle", activity, row, err, calls)
			}
		}
	}
}

func TestCaseMappingNamedSemanticsAndPresetCoverage(t *testing.T) {
	observed, err := caseMappingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	consumer := observed.Suites[4]
	byRune := map[int64]map[string]any{}
	for _, c := range consumer.Cases {
		byRune[c.Inputs[0].(int64)] = c.Expected.(map[string]any)
	}
	for _, test := range []struct{ r, upper, lower, title int64 }{
		{0x1c6, 0x1c4, 0x1c6, 0x1c5}, {0x1c5, 0x1c4, 0x1c6, 0x1c5}, {0x345, 0x399, 0x345, 0x399},
		{0xdf, 0xdf, 0xdf, 0xdf}, {0x130, 0x130, 'i', 0x130}, {0x131, 'I', 0x131, 'I'},
		{0x212a, 0x212a, 'k', 0x212a}, {0xd800, 0xd800, 0xd800, 0xd800}, {-1, -1, -1, -1},
	} {
		row, ok := byRune[test.r]
		if !ok || row["upper"] != test.upper || row["lower"] != test.lower || row["title"] != test.title {
			t.Fatal("named one-rune semantics differ", test, row)
		}
	}
	raw, err := os.ReadFile("../../examples/case-inspector/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var presets struct {
		Inputs []map[string]int64 `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &presets); err != nil {
		t.Fatal(err)
	}
	for _, input := range presets.Inputs {
		r, ok := input["app/case-inspector:Describe"]
		if !ok || len(input) != 1 || byRune[r] == nil {
			t.Fatal("example preset absent from independent finite corpus", input)
		}
	}
}

func TestCaseMappingActualSerializedBatchesAndBaselinePreserved(t *testing.T) {
	all := allNativeSuites(t)
	if len(all) != 57 {
		t.Fatal("native suite roster")
	}
	documents := []any{}
	oldCases := 0
	for _, s := range all[:45] {
		documents = append(documents, caseDocument(s))
		oldCases += len(s.Cases)
	}
	if oldCases != 37232 || contentDigest(mustJSON(t, documents)) != "sha256:2814c5e1ea3ff03ea156633a9a0563f2a9df45ca1b9297982f10ebe499a27369" {
		t.Fatal("existing 45 suites/37,232 original inputs or expectations changed")
	}
	maximum, addedCases, addedBatches := 0, 0, 0
	for _, s := range all[45:50] {
		size := caseMappingPortBatchSize
		if s.Package == "app/case-inspector" {
			size = caseMappingConsumerBatchSize
		}
		batches := (len(s.Cases) + size - 1) / size
		addedCases += len(s.Cases)
		addedBatches += batches
		t.Logf("%s:%s cases=%d batches=%d", s.Package, s.Activity, len(s.Cases), batches)
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			maximum = max(maximum, len(raw))
			if len(raw) > 32768 {
				t.Fatal("actual saved JSON exceeds 32KiB", s.Activity, len(raw))
			}
			var value any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&value); err != nil {
				t.Fatal(err)
			}
			// Re-encoding exact JSON tokens preserves signed int64 boundary values.
			roundtrip, err := json.MarshalIndent(value, "", "  ")
			if err != nil || !reflect.DeepEqual(append(roundtrip, '\n'), raw) {
				t.Fatal("input token or shape changed in JSON roundtrip")
			}
		}
	}
	t.Logf("added cases=%d batches=%d; aggregate cases=%d batches=%d; maximum actual case JSON bytes=%d", addedCases, addedBatches, oldCases+addedCases, 531+addedBatches, maximum)
}
