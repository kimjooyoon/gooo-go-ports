package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLastIndexManualAnchorsAndSeparateEightFieldExample(t *testing.T) {
	anchors := lastIndexAnchors()
	if len(anchors) != 26 {
		t.Fatal("manual LastIndex anchor roster changed")
	}
	var example struct {
		Inputs []map[string]any `json:"inputs"`
	}
	raw, err := os.ReadFile("../../examples/literal-final-marker-report/inputs.json")
	if err != nil || json.Unmarshal(raw, &example) != nil || len(example.Inputs) != len(anchors) {
		t.Fatal("separate v1 example input roster", err)
	}
	for i, a := range anchors {
		if strings.Index(a.Text, a.Needle) != a.First || strings.LastIndex(a.Text, a.Needle) != a.Last || strings.Count(a.Text, a.Needle) != a.Count {
			t.Fatal("pinned Go disagrees with independently stated anchor", a.Name)
		}
		want := map[string]any{"text": a.Text, "needle": a.Needle, "first_byte_offset": a.First, "last_byte_offset": a.Last, "found": a.First >= 0, "non_overlapping_count": a.Count, "text_byte_length": len(a.Text), "needle_byte_length": len(a.Needle)}
		if !reflect.DeepEqual(lastIndexReportOracle(a.Text, a.Needle), want) {
			t.Fatal("eight-field Go oracle disagrees with manual anchor", a.Name)
		}
		row := example.Inputs[i]
		if len(row) != 2 || row[lastIndexConsumerPackage+":Report.input0"] != a.Text || row[lastIndexConsumerPackage+":Report.input1"] != a.Needle {
			t.Fatal("new example must retain every original anchor and its order", a.Name)
		}
	}
	// Independently highlight the two common incorrect reductions.
	if strings.LastIndex("aaaaa", "aa") != 3 || strings.Count("aaaaa", "aa") != 2 || strings.LastIndex("가🙂\x00", "") != 8 {
		t.Fatal("overlap or empty-needle byte boundary changed")
	}
}

func TestLastIndexReferenceValidatesEveryRowBeforeAnyOracle(t *testing.T) {
	valid := []any{"ababa", "aba"}
	for _, entry := range []suite{{Package: literalTextPackage, Activity: "LastIndex"}, {Package: lastIndexConsumerPackage, Activity: "Report"}} {
		for _, bad := range [][]any{nil, {"only-one"}, {"a", "b", "extra"}, {nil, "a"}, {3, "a"}, {"a", false}, {string([]byte{0xff}), "a"}, {"a", string([]byte{0xc0})}, {strings.Repeat("a", 1025), ""}, {"", strings.Repeat("a", 1025)}} {
			calls := 0
			if _, err := lastIndexReferenceSuite(entry.Package, entry.Activity, [][]any{valid, bad}, func(string, string) any { calls++; return 0 }); err == nil || calls != 0 {
				t.Fatal("invalid later row reached oracle or was accepted", entry, bad, err, calls)
			}
		}
		for _, rows := range [][][]any{nil, {valid, valid}} {
			calls := 0
			if _, err := lastIndexReferenceSuite(entry.Package, entry.Activity, rows, func(string, string) any { calls++; return 0 }); err == nil || calls != 0 {
				t.Fatal("empty/duplicate roster reached oracle", entry, err, calls)
			}
		}
		if _, err := lastIndexReferenceSuite(entry.Package, entry.Activity, [][]any{valid}, nil); err == nil {
			t.Fatal("nil oracle accepted")
		}
		for _, pair := range []literalTextPair{{strings.Repeat("a", 1024), ""}, {"", strings.Repeat("a", 1024)}, {strings.Repeat("🙂", 256), ""}} {
			if _, err := lastIndexReferenceSuite(entry.Package, entry.Activity, [][]any{{pair.Text, pair.Needle}}, func(string, string) any { return 0 }); err != nil {
				t.Fatal("valid exact1024 boundary rejected", err)
			}
		}
	}
	for _, entry := range []suite{{Package: literalTextPackage, Activity: "Index"}, {Package: lastIndexConsumerPackage, Activity: "LastIndex"}, {Package: "untrusted", Activity: "LastIndex"}} {
		calls := 0
		if _, err := lastIndexReferenceSuite(entry.Package, entry.Activity, [][]any{valid}, func(string, string) any { calls++; return 0 }); err == nil || calls != 0 {
			t.Fatal("unsupported entry reached oracle", entry)
		}
	}
}

func TestLastIndexFiniteCorpusCoverageOrderAndIndependentOracles(t *testing.T) {
	got, err := lastIndexCorpus()
	if err != nil {
		t.Fatal(err)
	}
	again, err := lastIndexCorpus()
	if err != nil || !reflect.DeepEqual(got, again) || len(got.Suites) != 2 {
		t.Fatal("finite corpus is not deterministic", err)
	}
	if got.PortReferenceCases != len(got.PortPairs) || got.ConsumerReferenceCases != len(got.ConsumerPairs) || got.PortReferenceCases < 1465 || got.PortReferenceCases > 1800 || got.ConsumerReferenceCases < 26 || got.ConsumerReferenceCases > 90 {
		t.Fatal("finite coverage unexpectedly shrank or expanded", got.PortReferenceCases, got.ConsumerReferenceCases)
	}
	seen := map[literalTextPair]bool{}
	for i, pair := range got.PortPairs {
		if seen[pair] {
			t.Fatal("duplicate port pair", i)
		}
		seen[pair] = true
		if _, err := readLiteralTextPair([]any{pair.Text, pair.Needle}); err != nil {
			t.Fatal("out-of-domain generated pair", err)
		}
	}
	short := []string{""}
	for _, a := range []string{"a", "b", "\x00", "é"} {
		short = append(short, a)
		for _, b := range []string{"a", "b", "\x00", "é"} {
			short = append(short, a+b)
		}
	}
	if len(short) != 21 {
		t.Fatal("independent short alphabet roster")
	}
	for _, text := range short {
		for _, needle := range short {
			if !seen[literalTextPair{text, needle}] {
				t.Fatal("missing short Cartesian pair", text, needle)
			}
		}
	}
	for offset := 0; offset < 1024; offset++ {
		pair := literalTextPair{strings.Repeat("a", offset) + "Z" + strings.Repeat("a", 1023-offset), "Z"}
		if !seen[pair] || strings.LastIndex(pair.Text, pair.Needle) != offset {
			t.Fatal("missing original byte offset", offset)
		}
	}
	for k, s := range got.Suites {
		pairs := got.PortPairs
		if k == 1 {
			pairs = got.ConsumerPairs
		}
		if !isLastIndexSuite(s) || len(s.Cases) != len(pairs) {
			t.Fatal("new suite registration or size", k)
		}
		for i, pair := range pairs {
			want := any(strings.LastIndex(pair.Text, pair.Needle))
			if k == 1 {
				want = lastIndexReportOracle(pair.Text, pair.Needle)
			}
			if !seen[pair] || !reflect.DeepEqual(s.Cases[i], testCase{Inputs: []any{pair.Text, pair.Needle}, Expected: want}) {
				t.Fatal("original pair or independent result differs", k, i)
			}
		}
	}
}
