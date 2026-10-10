package main

import (
	"encoding/json"
	"os"
	"testing"
	"unicode"
)

func TestWhitespaceBrowserSeedsKeepAllMembersAndIndexedBatches(t *testing.T) {
	raw, err := os.ReadFile("../../examples/browser-demo/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	batches, err := splitBrowserCases(raw)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Inputs   map[string]json.RawMessage `json:"inputs"`
			Expected map[string]struct {
				Flag bool `json:"flag"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	positive := 0
	for _, row := range doc.Cases {
		var op string
		if err = json.Unmarshal(row.Inputs["app/browser-demo:Dispatch.input0"], &op); err != nil {
			t.Fatal(err)
		}
		if op != "unicode.IsSpace" {
			continue
		}
		var n int64
		if err = json.Unmarshal(row.Inputs["app/browser-demo:Dispatch.input1"], &n); err != nil {
			t.Fatal(err)
		}
		if n < -1<<31 || n > 1<<31-1 || seen[n] {
			t.Fatalf("bad seed rune %d", n)
		}
		seen[n] = true
		want := unicode.IsSpace(rune(n))
		if want {
			positive++
		}
		if row.Expected["app/browser-demo:Dispatch"].Flag != want {
			t.Fatalf("seed is not Go reference for %d", n)
		}
	}
	if len(doc.Cases) != 193 || len(seen) != 40 || positive != 25 {
		t.Fatalf("unexpected seeds %d, whitespace %d, members %d", len(doc.Cases), len(seen), positive)
	}
	t.Logf("%d browser seeds preserved across %d indexed batches", len(doc.Cases), len(batches))
}
