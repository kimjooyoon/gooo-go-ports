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

func etagFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func etagFixtureValue(t *testing.T, name string) map[string]any {
	t.Helper()
	v, err := decode(etagFixtureBytes(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}

func etagEncodeFixture(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestETagFrozen247InputsAndLiteralAnchors(t *testing.T) {
	raw, err := decimalPayloadReadFixture("../..", etagCorpusFile, etagCorpusSHA)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := etagInputs(raw)
	if err != nil {
		t.Fatal(err)
	}
	manual, err := decimalPayloadReadFixture("../..", etagManualFile, etagManualSHA)
	if err != nil {
		t.Fatal(err)
	}
	anchors, err := etagManualInputs(manual)
	if err != nil {
		t.Fatal(err)
	}
	if err := etagCheckManual(rows, anchors); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 247 || len(anchors) != 247 || rows[191].Kind != "scanner" || rows[192].Kind != "strong_match" || rows[222].Kind != "strong_match" || rows[223].Kind != "facts" {
		t.Fatal("original192/31/24 partition changed")
	}
	// Repeated scanner inputs are distinct original occurrences, not padding.
	occurrences := map[string]int{}
	for _, row := range rows[:192] {
		occurrences[row.Inputs[0]]++
	}
	if len(occurrences) != 190 || occurrences[`"""`] != 2 || occurrences[`"*"`] != 2 {
		t.Fatal("original scanner duplicates lost", occurrences)
	}
	if len(rows[240].Inputs[0]) != 1024 || rows[240].ID != "full1024-backslashes-first-native-gate" {
		t.Fatal("original full1024 witness changed")
	}
}

func TestETagThreeSuitesPreserveEveryOriginalOccurrence(t *testing.T) {
	suites, err := etagCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := etagInputs(etagFixtureBytes(t, etagCorpusFile))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		packagePath, activity string
		count                 int
	}{
		{etagPortPackage, etagScanActivity, 192}, {etagPortPackage, etagStrongActivity, 31}, {etagReportPackage, etagReportActivity, 24},
	}
	if len(suites) != len(want) {
		t.Fatal("exact3 suites required")
	}
	ordinal := 0
	fields := 0
	for at, s := range suites {
		if s.Package != want[at].packagePath || s.Activity != want[at].activity || len(s.Cases) != want[at].count {
			t.Fatal("suite identity/count", at)
		}
		for _, c := range s.Cases {
			row := rows[ordinal]
			inputs := make([]any, len(row.Inputs))
			for port, text := range row.Inputs {
				inputs[port] = text
			}
			if !reflect.DeepEqual(c.Inputs, inputs) || !reflect.DeepEqual(c.Expected, etagReference(row)) {
				t.Fatal("original input order/independent expected changed", ordinal)
			}
			if at == 0 {
				fields += 2
			} else if at == 1 {
				fields++
			} else {
				fields += 13
			}
			ordinal++
		}
	}
	if ordinal != 247 || fields != 727 {
		t.Fatal("full finite row/returned-field population changed")
	}
}

func TestETagInputGuardsAndOrderedAnchorBinding(t *testing.T) {
	manual, err := etagManualInputs(etagFixtureBytes(t, etagManualFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"late-bool-text", "late-text1025", "extra-expected", "missing-row", "duplicate-id", "within-scan-reorder"} {
		t.Run(mutation, func(t *testing.T) {
			v := etagFixtureValue(t, etagCorpusFile)
			cases := v["cases"].([]any)
			last := cases[len(cases)-1].(map[string]any)
			switch mutation {
			case "late-bool-text":
				last["inputs"] = []any{false}
			case "late-text1025":
				last["inputs"] = []any{strings.Repeat("a", 1025)}
			case "extra-expected":
				last["expected"] = false
			case "missing-row":
				v["cases"] = cases[:len(cases)-1]
			case "duplicate-id":
				last["id"] = cases[len(cases)-2].(map[string]any)["id"]
			case "within-scan-reorder":
				cases[8], cases[9] = cases[9], cases[8]
			}
			rows, parseErr := etagInputs(etagEncodeFixture(t, v))
			if parseErr == nil && etagCheckManual(rows, manual) == nil {
				t.Fatal("input/anchor drift accepted", mutation)
			}
		})
	}
	raw := etagFixtureBytes(t, etagCorpusFile)
	duplicate := bytes.Replace(raw, []byte(`"schema": "gooo/etag-input-corpus/v1"`), []byte(`"schema": "gooo/etag-input-corpus/v1", "\u0073chema": "gooo/etag-input-corpus/v1"`), 1)
	surrogate := bytes.Replace(raw, []byte(`"original-go-1-weak"`), []byte(`"\ud800"`), 1)
	invalidUTF8 := bytes.Replace(raw, []byte("original-go-1-weak"), []byte{0xff}, 1)
	for _, malformed := range [][]byte{duplicate, surrogate, invalidUTF8, append(append([]byte{}, raw...), []byte(" false")...)} {
		if _, err := etagInputs(malformed); err == nil {
			t.Fatal("duplicate/malformed Unicode/trailing fixture accepted")
		}
	}
}

func TestETagLiteralTypeAndSemanticDriftRefused(t *testing.T) {
	rows, err := etagInputs(etagFixtureBytes(t, etagCorpusFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"scan-remainder", "facts-bool-as-int", "facts-integer-as-float", "raw-strong-value", "late-input-order"} {
		t.Run(mutation, func(t *testing.T) {
			v := etagFixtureValue(t, etagManualFile)
			cases := v["cases"].([]any)
			switch mutation {
			case "scan-remainder":
				cases[0].(map[string]any)["expected"].(map[string]any)["remain"] = "changed"
			case "facts-bool-as-int":
				cases[223].(map[string]any)["expected"].(map[string]any)["header_valid"] = json.Number("1")
			case "facts-integer-as-float":
				cases[223].(map[string]any)["expected"].(map[string]any)["input_bytes"] = json.Number("0.0")
			case "raw-strong-value":
				cases[192].(map[string]any)["expected"] = true
			case "late-input-order":
				cases[245], cases[246] = cases[246], cases[245]
			}
			anchors, parseErr := etagManualInputs(etagEncodeFixture(t, v))
			if parseErr == nil && etagCheckManual(rows, anchors) == nil {
				t.Fatal("literal oracle/type/input drift accepted", mutation)
			}
		})
	}
}

func TestETagRepositoryRelativeFixtureByteDriftRefused(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{etagCorpusFile, etagManualFile} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, etagFixtureBytes(t, name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := etagCorpus(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{etagCorpusFile, etagManualFile} {
		path := filepath.Join(root, name)
		raw := etagFixtureBytes(t, name)
		if err := os.WriteFile(path, append(append([]byte{}, raw...), '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := etagCorpus(root); err == nil {
			t.Fatal("raw fixture byte drift accepted", name)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
