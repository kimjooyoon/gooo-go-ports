package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func canonicalHeaderFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func canonicalHeaderMutableFixture(t *testing.T) map[string]any {
	t.Helper()
	value, err := decode(canonicalHeaderFixture(t, canonicalHeaderCorpusFile))
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)
}

func TestCanonicalHeaderFrozenPopulationKeepsOrderRangesAndDuplicates(t *testing.T) {
	raw := canonicalHeaderFixture(t, canonicalHeaderCorpusFile)
	in, err := canonicalHeaderInputs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Rows) != 1059 || in.UniqueKeys != 1054 || in.WithinDuplicates != 0 || in.CrossDuplicates != 5 || len(in.Partitions) != 13 {
		t.Fatal("raw occurrences or exact duplicate accounting changed", in)
	}
	spans := [][2]int{{0, 20}, {20, 148}, {148, 276}, {276, 404}, {404, 532}, {532, 660}, {660, 788}, {788, 821}, {821, 857}, {857, 867}, {867, 931}, {931, 995}, {995, 1059}}
	for at, part := range in.Partitions {
		if part.StartInclusive != spans[at][0] || part.EndExclusive != spans[at][1] || part.RawRows != part.EndExclusive-part.StartInclusive || part.UniqueKeys != part.RawRows {
			t.Fatal("fixed half-open partition changed", at, part)
		}
	}
	byKey := map[string][]string{}
	for _, row := range in.Rows {
		byKey[row.Key] = append(byKey[row.Key], row.ID)
	}
	for key, want := range map[string][]string{"": {"empty", "named/00", "seed-unicode/009"}, "\x14": {"ascii-single/020", "seed-unicode/047"}, "-": {"ascii-single/045", "named/01"}, "é": {"named/31", "unicode-boundaries/027"}} {
		if !reflect.DeepEqual(byKey[key], want) {
			t.Fatalf("original duplicate occurrences lost for%q: %v", key, byKey[key])
		}
	}
	if len(byKey["e\u0301"]) != 1 || reflect.DeepEqual(byKey["e\u0301"], byKey["é"]) {
		t.Fatal("NFC/NFD originals collapsed")
	}
	anchors, err := canonicalHeaderManualInputs(canonicalHeaderFixture(t, canonicalHeaderManualFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := canonicalHeaderCheckManual(in, anchors); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalHeaderTwoSuitesKeepEveryOccurrenceAndSeparateResults(t *testing.T) {
	suites, err := canonicalHeaderCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(suites) != 2 || suites[0].Package != canonicalHeaderPackage || suites[0].Activity != canonicalHeaderActivity || suites[1].Package != canonicalHeaderReportPackage || suites[1].Activity != canonicalHeaderReportActivity {
		t.Fatal("exact scalar then Report suite order")
	}
	in, err := canonicalHeaderInputs(canonicalHeaderFixture(t, canonicalHeaderCorpusFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range suites {
		if len(s.Cases) != 1059 {
			t.Fatal("partial or deduplicated suite", s.Package, len(s.Cases))
		}
		for at, row := range s.Cases {
			if len(row.Inputs) != 1 || row.Inputs[0] != in.Rows[at].Key {
				t.Fatal("original ordered input changed", at)
			}
		}
	}
	first := suites[1].Cases[0].Expected.(map[string]any)
	first["canonical_key"] = "changed-only-in-this-map"
	if suites[1].Cases[788].Expected.(map[string]any)["canonical_key"] != "" || suites[0].Cases[0].Expected != "" {
		t.Fatal("repeated input occurrences or scalar expected alias Report map")
	}
	fresh, err := canonicalHeaderCorpus("../..")
	if err != nil || fresh[1].Cases[0].Expected.(map[string]any)["canonical_key"] != "" {
		t.Fatal("expected state leaked across corpus calls", err)
	}
}

func TestCanonicalHeaderIndependentReferenceManualTransitionsAndWitnesses(t *testing.T) {
	for _, test := range []struct {
		key, canonical string
		valid          bool
		offset, octet  int64
	}{
		{"", "", false, -1, -1},
		{"Content-Type", "Content-Type", true, -1, -1},
		{"content-type", "Content-Type", true, -1, -1},
		{"!FOO", "!foo", true, -1, -1},
		{"1FOO", "1foo", true, -1, -1},
		{"X-!FOO", "X-!foo", true, -1, -1},
		{"x-HEADER:TAIL", "x-HEADER:TAIL", false, 8, 58},
		{"aé", "aé", false, 1, 195},
		{"e\u0301", "e\u0301", false, 1, 204},
		{"x\x00TAIL", "x\x00TAIL", false, 1, 0},
		{"x\r\nTAIL", "x\r\nTAIL", false, 1, 13},
		{"x" + strings.Repeat("A", 1023), "X" + strings.Repeat("a", 1023), true, -1, -1},
		{strings.Repeat("a", 1022) + "é", strings.Repeat("a", 1022) + "é", false, 1022, 195},
	} {
		want := map[string]any{"key": test.key, "canonical_key": test.canonical, "changed": test.key != test.canonical, "name_valid": test.valid, "input_bytes": int64(len(test.key)), "canonical_bytes": int64(len(test.canonical)), "name_invalid_offset": test.offset, "name_invalid_octet": test.octet}
		if got := canonicalHeaderReference(test.key); !reflect.DeepEqual(got, want) {
			t.Fatalf("independent scalar/8typed fields for%q: got%v want%v", test.key, got, want)
		}
	}
}

func TestCanonicalHeaderInputRefusesLateMalformedRowsAndChangedPopulation(t *testing.T) {
	for _, kind := range []string{"missing-key", "null-key", "number-key", "bool-key", "array-key", "object-key", "key1025", "wrong-ID", "null-ID", "oversized-ID", "extra-field", "case-alias", "duplicate-ID", "wrong-schema", "missing-case", "order", "extra-top", "changed-duplicate-accounting"} {
		t.Run(kind, func(t *testing.T) {
			value := canonicalHeaderMutableFixture(t)
			rows := value["cases"].([]any)
			last := rows[len(rows)-1].(map[string]any)
			switch kind {
			case "missing-key":
				delete(last, "key")
			case "null-key":
				last["key"] = nil
			case "number-key":
				last["key"] = 7
			case "bool-key":
				last["key"] = false
			case "array-key":
				last["key"] = []any{}
			case "object-key":
				last["key"] = map[string]any{}
			case "key1025":
				last["key"] = strings.Repeat("a", 1025)
			case "wrong-ID":
				last["id"] = "seed-unicode/999"
			case "null-ID":
				last["id"] = nil
			case "oversized-ID":
				last["id"] = strings.Repeat("x", 129)
			case "extra-field":
				last["expected"] = "must-not-be-an-oracle"
			case "case-alias":
				last["KEY"] = last["key"]
				delete(last, "key")
			case "duplicate-ID":
				last["id"] = rows[len(rows)-2].(map[string]any)["id"]
			case "wrong-schema":
				value["schema"] = "gooo/other/v1"
			case "missing-case":
				value["cases"] = rows[:len(rows)-1]
			case "order":
				rows[0], rows[1] = rows[1], rows[0]
			case "extra-top":
				value["expected"] = "no-output-fixtures"
			case "changed-duplicate-accounting":
				counts := map[string]int{}
				for _, row := range rows {
					counts[row.(map[string]any)["key"].(string)]++
				}
				changed := false
				for at := len(rows) - 1; at >= 995; at-- {
					row := rows[at].(map[string]any)
					if counts[row["key"].(string)] == 1 {
						row["key"] = ""
						changed = true
						break
					}
				}
				if !changed {
					t.Fatal("fixture has no unique tail witness")
				}
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			got, err := canonicalHeaderInputs(raw)
			if err == nil || len(got.Rows) != 0 {
				t.Fatal("late malformed input returned trusted rows", kind, err)
			}
		})
	}
}

func TestCanonicalHeaderRawUnicodeAndDuplicateWireRefusals(t *testing.T) {
	raw := canonicalHeaderFixture(t, canonicalHeaderCorpusFile)
	needle := []byte(`"key": ""`)
	if !bytes.Contains(raw, needle) {
		t.Fatal("frozen explicit empty source spelling")
	}
	for _, replacement := range [][]byte{
		[]byte("\"key\": \"" + string([]byte{0xff}) + "\""),
		[]byte(`"key": "\ud800"`), []byte(`"key": "\udfff"`),
		[]byte(`"key":"","\u006bey":""`),
	} {
		bad := bytes.Replace(raw, needle, replacement, 1)
		if _, err := canonicalHeaderInputs(bad); err == nil {
			t.Fatal("invalid Unicode/decoded duplicate was replacement-decoded")
		}
	}
	for _, bad := range [][]byte{append(append([]byte{}, raw...), []byte("{}")...), raw[:len(raw)-2], bytes.Repeat([]byte{' '}, (4<<20)+1)} {
		if _, err := canonicalHeaderInputs(bad); err == nil {
			t.Fatal("trailing/malformed/oversized raw input accepted")
		}
	}
	pair := bytes.Replace(raw, []byte("🙂"), []byte(`\ud83d\ude42`), 1)
	if bytes.Equal(pair, raw) {
		t.Fatal("fixture missing valid pair counterpart")
	}
	before, err := canonicalHeaderInputs(raw)
	if err != nil {
		t.Fatal(err)
	}
	after, err := canonicalHeaderInputs(pair)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("valid surrogate pair changed original decoded population", err)
	}
}

func TestCanonicalHeaderManualTruthAndRawFixturePinsAreIndependent(t *testing.T) {
	in, err := canonicalHeaderInputs(canonicalHeaderFixture(t, canonicalHeaderCorpusFile))
	if err != nil {
		t.Fatal(err)
	}
	anchors, err := canonicalHeaderManualInputs(canonicalHeaderFixture(t, canonicalHeaderManualFile))
	if err != nil {
		t.Fatal(err)
	}
	wrong := append([]canonicalHeaderManualRow{}, anchors...)
	wrong[1].Expected = "wrong-manual-answer"
	if err := canonicalHeaderCheckManual(in, wrong); err == nil {
		t.Fatal("manual values substituted for actual Go truth")
	}
	wrong = append([]canonicalHeaderManualRow{}, anchors...)
	wrong[19].Key = "different-original"
	if err := canonicalHeaderCheckManual(in, wrong); err == nil {
		t.Fatal("manual prefix changed")
	}
	root := t.TempDir()
	for _, name := range []string{canonicalHeaderCorpusFile, canonicalHeaderManualFile} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, canonicalHeaderFixture(t, name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := canonicalHeaderCorpus(root); err != nil {
		t.Fatal("frozen source-only fixture copy", err)
	}
	for _, name := range []string{canonicalHeaderCorpusFile, canonicalHeaderManualFile} {
		original := canonicalHeaderFixture(t, name)
		if err := os.WriteFile(filepath.Join(root, name), append(append([]byte{}, original...), '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := canonicalHeaderCorpus(root); err == nil || !strings.Contains(err.Error(), "frozen fixture bytes differ") {
			t.Fatal("raw source SHA not enforced", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), original, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
