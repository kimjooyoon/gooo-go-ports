package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"unicode"
)

func TestClassificationBrowserSeedsRetainOldCasesAndObserveGoSemantics(t *testing.T) {
	raw, err := os.ReadFile("../../examples/browser-demo/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc caseDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 261 {
		t.Fatalf("changed seed roster: %d", len(doc.Cases))
	}
	legacy, err := json.Marshal(doc.Cases[:193])
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(legacy)) != "5f7b609fbc4119a968ceed088e7683a330b73e030d381c75796dee5a8cb0391d" {
		t.Fatal("old browser observations changed")
	}
	counts := map[string]int{}
	for _, raw := range doc.Cases[193:] {
		var row struct {
			Inputs   map[string]json.RawMessage `json:"inputs"`
			Expected map[string]struct {
				Flag bool `json:"flag"`
			} `json:"expected"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		var op string
		var n int64
		if err := json.Unmarshal(row.Inputs["app/browser-demo:Dispatch.input0"], &op); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(row.Inputs["app/browser-demo:Dispatch.input1"], &n); err != nil || n < -1<<31 || n > 1<<31-1 {
			t.Fatal("invalid original seed rune")
		}
		var want bool
		switch op {
		case "unicode.IsLetter":
			want = unicode.IsLetter(rune(n))
		case "unicode.IsDigit":
			want = unicode.IsDigit(rune(n))
		default:
			t.Fatalf("unexpected appended operation %s", op)
		}
		if row.Expected["app/browser-demo:Dispatch"].Flag != want {
			t.Fatalf("not Go reference: %s(%d)", op, n)
		}
		counts[op]++
	}
	if counts["unicode.IsLetter"] != 34 || counts["unicode.IsDigit"] != 34 {
		t.Fatalf("missing classification seed coverage: %v", counts)
	}
}
