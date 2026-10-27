// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
package sdkcatalogrefresh

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	visibility "github.com/kimjooyoon/gooo-go-ports/examples/sdk-catalog-refresh/internal/rowvisibility"
)

type mapping struct {
	State     string `json:"state"`
	Reason    string `json:"reason"`
	WireCount int64  `json:"wire_count"`
	SDKCount  int64  `json:"sdk_count"`
	First     int64  `json:"first_difference_index"`
	WireName  string `json:"wire_name"`
	SDKName   string `json:"sdk_name"`
	Equal     bool   `json:"mapping_equal"`
	Scope     string `json:"scope"`
}

func generatedMapping(t *testing.T, wire, sdk []string) mapping {
	t.Helper()
	actual := visibility.Evaluate(visibility.R_RowNames{F_names: wire}, visibility.R_RowNames{F_names: sdk})
	raw, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 9 {
		t.Fatalf("nine-field ABI lost: %s", raw)
	}
	var result mapping
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// The reference uses standard-library equality and binary search over unequal
// prefixes, independently of the sequential generated Gooo activity.
func reference(wire, sdk []string) mapping {
	r := mapping{State: "MAPPED", Reason: "SAME_ORDERED_NAMES", WireCount: int64(len(wire)), SDKCount: int64(len(sdk)), First: -1, Equal: true, Scope: "ORDERED_NAME_CORRESPONDENCE"}
	invalid := func(s string) bool { return s == "" || len(s) > 1024 }
	if len(wire) > 256 || len(sdk) > 256 {
		r.State, r.Reason, r.Equal = "INVALID_INPUT", "ROW_LIMIT", false
		return r
	}
	if index := slices.IndexFunc(wire, invalid); index >= 0 {
		r.State, r.Reason, r.First, r.Equal = "INVALID_INPUT", "INVALID_WIRE_NAME", int64(index), false
		return r
	}
	if index := slices.IndexFunc(sdk, invalid); index >= 0 {
		r.State, r.Reason, r.First, r.Equal = "INVALID_INPUT", "INVALID_SDK_NAME", int64(index), false
		return r
	}
	if slices.Equal(wire, sdk) {
		return r
	}
	r.State, r.Equal = "MISMATCH", false
	limit := min(len(wire), len(sdk))
	first := sort.Search(limit, func(i int) bool { return !slices.Equal(wire[:i+1], sdk[:i+1]) })
	r.First = int64(first)
	if first < limit {
		r.Reason, r.WireName, r.SDKName = "NAME_DIFFERENCE", wire[first], sdk[first]
	} else if len(wire) > len(sdk) {
		r.Reason, r.WireName = "SDK_ROW_MISSING", wire[first]
	} else {
		r.Reason, r.SDKName = "SDK_ROW_UNEXPECTED", sdk[first]
	}
	return r
}

func TestLiteralNativeCasesAgainstGeneratedLibrary(t *testing.T) {
	raw, err := os.ReadFile("row-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]visibility.R_RowNames `json:"inputs"`
			Expected map[string]mapping               `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil || len(corpus.Cases) != 8 {
		t.Fatal("eight independent literal expectations required", err)
	}
	entry := "gooo/sdk-catalog/row-visibility:CompareSDKRows"
	for i, c := range corpus.Cases {
		got := generatedMapping(t, c.Inputs[entry+".input0"].F_names, c.Inputs[entry+".input1"].F_names)
		if got != c.Expected[entry] {
			t.Fatalf("literal %d: got%+v, want%+v", i, got, c.Expected[entry])
		}
	}
}

func TestFiniteOrderedNamesMatrix(t *testing.T) {
	values := [][]string{{}}
	last := [][]string{{}}
	for length := 1; length <= 3; length++ {
		next := [][]string{}
		for _, prior := range last {
			for _, name := range []string{"alpha", "beta", ""} {
				v := append(slices.Clone(prior), name)
				next = append(next, v)
				values = append(values, v)
			}
		}
		last = next
	}
	if len(values) != 40 {
		t.Fatal(len(values))
	}
	for _, wire := range values {
		for _, sdk := range values {
			if got, want := generatedMapping(t, wire, sdk), reference(wire, sdk); got != want {
				t.Fatalf("wire%q sdk%q: got%+v want%+v", wire, sdk, got, want)
			}
		}
	}
	t.Log("1600 finite ordered-name pairs matched independent Go reference")
}

func TestDirectFunctionBoundariesAndByteSpelling(t *testing.T) {
	rows := func(n int) []string { return slices.Repeat([]string{"a"}, n) }
	cases := []struct{ wire, sdk []string }{
		{rows(256), rows(256)}, {rows(257), rows(1)}, {rows(1), rows(257)},
		{[]string{strings.Repeat("a", 1024)}, []string{strings.Repeat("a", 1024)}},
		{[]string{strings.Repeat("a", 1025)}, []string{"a"}},
		{[]string{"a"}, []string{strings.Repeat("a", 1025)}},
		{[]string{"é"}, []string{"e\u0301"}},
		{[]string{"é", "😀"}, []string{"é", "😀"}},
	}
	for i, c := range cases {
		if got, want := generatedMapping(t, c.wire, c.sdk), reference(c.wire, c.sdk); got != want {
			t.Fatalf("boundary%d got%+v want%+v", i, got, want)
		}
	}
	t.Log("8 direct-Go boundary pairs; oversized inputs are not native-input acceptance claims")
}
