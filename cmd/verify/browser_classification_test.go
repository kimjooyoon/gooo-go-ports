package main

import (
	"strconv"
	"testing"
	"unicode"
)

func TestClassificationBrowserFixturesKeepFullRuneDomainAndGoResults(t *testing.T) {
	suites, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := makeBrowserFixtures(suites, upstream{})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []struct {
		name   string
		oracle func(rune) bool
		count  int
	}{{"unicode.IsLetter", unicode.IsLetter, 4128}, {"unicode.IsDigit", unicode.IsDigit, 915}} {
		seen := map[int64]bool{}
		for _, row := range doc.Cases {
			if row.Operation != op.name {
				continue
			}
			if len(row.Inputs) != 2 || row.Inputs[1] != "0" {
				t.Fatalf("changed unary ABI: %+v", row)
			}
			n, err := strconv.ParseInt(row.Inputs[0], 10, 32)
			if err != nil || seen[n] {
				t.Fatalf("narrowed or duplicate original rune: %+v", row)
			}
			seen[n] = true
			if row.Expected.Type != "boolean" || row.Expected.Value != op.oracle(rune(n)) {
				t.Fatalf("not original Go classification: %+v", row)
			}
		}
		if len(seen) != op.count {
			t.Fatalf("%s: changed reference roster %d", op.name, len(seen))
		}
		for n := int64(0); n <= 255; n++ {
			if !seen[n] {
				t.Fatalf("missing Latin-1 %d", n)
			}
		}
		for _, n := range classificationBoundaryInputs() {
			if !seen[n] {
				t.Fatalf("missing original classification boundary %d", n)
			}
		}
		rejected := map[string]bool{}
		for _, row := range doc.Rejections {
			if row.Operation == op.name && len(row.Inputs) == 2 && row.Inputs[1] == "0" {
				rejected[row.Inputs[0]] = true
			}
		}
		for _, n := range classificationRejectedInputs() {
			if !rejected[strconv.FormatInt(n, 10)] {
				t.Fatalf("missing original rejection integer %d", n)
			}
			if _, err := browserRow(op.name, testCase{Inputs: []any{n}, Expected: false}); err == nil {
				t.Fatalf("outside-int32 value presented as Go-reference case: %d", n)
			}
		}
	}
}
