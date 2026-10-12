package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLiteralTextManualSemanticsAndSupplied24Pairs(t *testing.T) {
	anchors := literalTextPilotAnchors()
	if len(anchors) != 24 {
		t.Fatal("supplied anchor roster changed")
	}
	var input struct {
		Schema string              `json:"schema"`
		Inputs []map[string]string `json:"inputs"`
	}
	raw, err := os.ReadFile("../../examples/literal-text-report/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if input.Schema != "gooo/body-composition-inputs/v1" || len(input.Inputs) != len(anchors) {
		t.Fatal("supplied input-only roster")
	}
	for i, a := range anchors {
		if strings.Index(a.Text, a.Needle) != a.Index || strings.Count(a.Text, a.Needle) != a.Count {
			t.Fatal("manual expected byte offset/non-overlap count", a.Name)
		}
		if len(input.Inputs[i]) != 2 || input.Inputs[i][literalTextConsumerPackage+":Report.input0"] != a.Text || input.Inputs[i][literalTextConsumerPackage+":Report.input1"] != a.Needle {
			t.Fatal("supplied original pair changed", i)
		}
	}
	for _, a := range []literalTextAnchor{
		{"five-as", literalTextPair{"aaaaa", "aa"}, 0, 2},
		{"combining-empty", literalTextPair{"e\u0301", ""}, 0, 3},
		{"zwj-empty", literalTextPair{"👩\u200d💻", ""}, 0, 4},
		{"serializer-offset", literalTextPair{"\"\\<>&\u2028\u2029", "\u2029"}, 8, 1},
		{"last-multibyte", literalTextPair{strings.Repeat("a", 1020) + "🙂", "🙂"}, 1020, 1},
	} {
		if strings.Index(a.Text, a.Needle) != a.Index || strings.Count(a.Text, a.Needle) != a.Count {
			t.Fatal("manual semantic anchor", a.Name)
		}
	}
}

func TestLiteralTextRejectsCompleteBadRosterBeforeOracle(t *testing.T) {
	valid := []any{"é🙂", "🙂"}
	bad := []any{nil, false, true, 0, int64(1), []any{}, map[string]any{}, []byte("text"), string([]byte{0xff}), strings.Repeat("a", 1025), strings.Repeat("🙂", 256) + "a"}
	for position := 0; position < 2; position++ {
		for _, value := range bad {
			row := append([]any(nil), valid...)
			row[position] = value
			calls := 0
			_, err := literalTextReferenceSuite(literalTextPackage, "Index", [][]any{valid, row}, func(string, string) any { calls++; return 0 })
			if err == nil || calls != 0 {
				t.Fatal("bad pair reached Go oracle", position, value, calls)
			}
		}
	}
	for _, rows := range [][][]any{nil, {nil}, {{"one"}}, {{"a", "b", "extra"}}, {valid, valid}} {
		calls := 0
		if _, err := literalTextReferenceSuite(literalTextPackage, "Count", rows, func(string, string) any { calls++; return 0 }); err == nil || calls != 0 {
			t.Fatal("empty/arity/duplicate roster reached oracle", calls)
		}
	}
	for _, pair := range []literalTextPair{{"", ""}, {strings.Repeat("🙂", 256), ""}, {"", strings.Repeat("a", 1024)}, {strings.Repeat("a", 1024), strings.Repeat("🙂", 256)}} {
		if got, err := readLiteralTextPair([]any{pair.Text, pair.Needle}); err != nil || got != pair {
			t.Fatal("valid decoded Text boundary rejected", err)
		}
	}
}

func TestLiteralTextCoverageOriginalInputsAndSerializedBatches(t *testing.T) {
	result, err := literalTextCorpus()
	if err != nil {
		t.Fatal(err)
	}
	roster := result.Roster
	seen := map[literalTextPair]bool{}
	for _, pair := range roster.Pairs {
		if seen[pair] || !utf8.ValidString(pair.Text) || !utf8.ValidString(pair.Needle) || len(pair.Text) > 1024 || len(pair.Needle) > 1024 {
			t.Fatal("invalid/duplicate original pair")
		}
		seen[pair] = true
	}
	if !reflect.DeepEqual(roster, literalTextInputs()) {
		t.Fatal("fixed-seed roster is not deterministic")
	}
	if roster.CategoryCandidates["pilot"] != 24 || roster.CategoryCandidates["short_cartesian"] != 7225 || roster.CategoryCandidates["byte_offsets"] != 1024 || roster.CategoryCandidates["seeded"] != 512 {
		t.Fatal("declared structured coverage changed", roster.CategoryCandidates)
	}
	added := 0
	for _, count := range roster.CategoryAdded {
		added += count
	}
	if added != len(roster.Pairs) {
		t.Fatal("unique pair category accounting")
	}
	short := literalTextShortValues()
	if len(short) != 85 {
		t.Fatal("small Cartesian set")
	}
	for _, text := range short {
		for _, needle := range short {
			if !seen[literalTextPair{text, needle}] {
				t.Fatal("symmetric short pair missing")
			}
		}
	}
	for offset := 0; offset <= 1023; offset++ {
		pair := literalTextPair{strings.Repeat("a", offset) + "Z" + strings.Repeat("a", 1023-offset), "Z"}
		if !seen[pair] || strings.Index(pair.Text, pair.Needle) != offset {
			t.Fatal("supported byte offset missing", offset)
		}
	}
	for length := 0; length <= 1024; length++ {
		text := literalTextOfLength(length)
		if !utf8.ValidString(text) || len(text) != length {
			t.Fatal("exact mixed UTF-8 byte length", length)
		}
	}
	if len(result.Suites) != 3 || result.IndexReferenceCases != len(roster.Pairs) || result.CountReferenceCases != len(roster.Pairs) || result.ReportReferenceCases != len(roster.Pairs) {
		t.Fatal("three same-roster reference counts")
	}
	caseTotal, batchTotal := 0, 0
	for index, s := range result.Suites {
		batches, maxbytes := 0, 0
		for i, row := range s.Cases {
			pair, err := readLiteralTextPair(row.Inputs)
			if err != nil || pair != roster.Pairs[i] {
				t.Fatal("original pair roster changed", s.Activity, i, err)
			}
			var want any
			switch index {
			case 0:
				want = strings.Index(pair.Text, pair.Needle)
			case 1:
				want = strings.Count(pair.Text, pair.Needle)
			case 2:
				want = literalTextReportOracle(pair.Text, pair.Needle)
			}
			if !reflect.DeepEqual(row.Expected, want) {
				t.Fatal("original Go expectation changed", s.Activity, i)
			}
		}
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil {
				t.Fatal(err)
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 30000 || end-start > 128 {
				t.Fatal("serialized literal batch budget", s.Activity, start, end, len(raw)+1, err)
			}
			if end < min(start+128, len(s.Cases)) {
				larger := s
				larger.Cases = s.Cases[start : end+1]
				next, err := json.MarshalIndent(caseDocument(larger), "", "  ")
				if err != nil || len(next)+1 <= 30000 {
					t.Fatal("batch did not select largest fitting prefix")
				}
			}
			var document struct {
				Cases []struct{ Inputs, Expected map[string]json.RawMessage }
			}
			if err = json.Unmarshal(raw, &document); err != nil || len(document.Cases) != end-start {
				t.Fatal("serialized batch roster", err)
			}
			key := s.Package + ":" + s.Activity
			for i, row := range document.Cases {
				if len(row.Inputs) != 2 || len(row.Expected) != 1 {
					t.Fatal("serialized two-Text input/output arity")
				}
				for arg := 0; arg < 2; arg++ {
					port := ".input0"
					if arg == 1 {
						port = ".input1"
					}
					if !equalValue(row.Inputs[key+port], batch.Cases[i].Inputs[arg]) {
						t.Fatal("serialization changed original Text", s.Activity, start+i, arg)
					}
				}
				if !equalValue(row.Expected[key], batch.Cases[i].Expected) {
					t.Fatal("serialized Go expectation changed")
				}
			}
			maxbytes = max(maxbytes, len(raw)+1)
			batches++
			start = end
		}
		t.Logf("%s:%s cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maxbytes)
		caseTotal += len(s.Cases)
		batchTotal += batches
	}
	// Input slices and record maps are independently allocated for each suite.
	result.Suites[0].Cases[0].Inputs[0] = "changed"
	if result.Suites[1].Cases[0].Inputs[0] != "" || result.Suites[2].Cases[0].Inputs[0] != "" {
		t.Fatal("input slices alias across independent suites")
	}
	t.Logf("categories candidates=%v added=%v; total suites=62 cases=%d batches=%d", roster.CategoryCandidates, roster.CategoryAdded, 131842+caseTotal, 1777+batchTotal)
}
