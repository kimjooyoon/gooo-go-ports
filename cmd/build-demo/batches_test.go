package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSplitBrowserCasesPreservesOrderPrecisionAndActualByteLimit(t *testing.T) {
	for _, tc := range []struct{ count, textBytes int }{{257, 0}, {97, 600}} {
		source := caseDocument{Schema: "gooo/body-composition-cases/v1"}
		for i := 0; i < tc.count; i++ {
			source.Cases = append(source.Cases, testJSON(t, map[string]any{"index": i, "input": int64(9007199254740993), "text": strings.Repeat("한", tc.textBytes)}))
		}
		batches, err := splitBrowserCases(testJSON(t, source))
		if err != nil {
			t.Fatal(err)
		}
		if len(batches) < 2 {
			t.Fatal("fixture did not exercise splitting")
		}
		position := 0
		for _, raw := range batches {
			if len(raw) > maxCaseBytes || raw[len(raw)-1] != '\n' {
				t.Fatal("serialized byte limit or newline lost")
			}
			var batch caseDocument
			if err := json.Unmarshal(raw, &batch); err != nil {
				t.Fatal(err)
			}
			if batch.Schema != source.Schema || len(batch.Cases) == 0 || len(batch.Cases) > 128 {
				t.Fatal("case envelope changed")
			}
			for _, row := range batch.Cases {
				if position >= len(source.Cases) || !sameJSON(row, source.Cases[position]) {
					t.Fatal("input changed, dropped, duplicated or reordered")
				}
				position++
			}
		}
		if position != len(source.Cases) {
			t.Fatal("cases lost")
		}
	}
}

func TestSplitBrowserCasesRejectsEmptyMalformedAndOversizedCase(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"schema":"gooo/body-composition-cases/v1","cases":[]}`),
		[]byte(`{"schema":"other","cases":[{}]}`),
		[]byte(`{} {}`),
		testJSON(t, caseDocument{Schema: "gooo/body-composition-cases/v1", Cases: []json.RawMessage{testJSON(t, strings.Repeat("x", maxCaseBytes))}}),
	} {
		if _, err := splitBrowserCases(raw); err == nil {
			t.Fatal("accepted invalid source cases")
		}
	}
}

func TestInspectBatchBindsTheSuppliedCaseFile(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	original, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("batch.json", original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesPath, []byte(`{"cases":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectBatchReceipt(raw, lock, sources, "", "batch.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectBatchReceipt(raw, lock, sources, "", casesPath); err == nil {
		t.Fatal("accepted another batch's case file")
	}
}
