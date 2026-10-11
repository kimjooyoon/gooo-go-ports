package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

func pinnedTableSource(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src/unicode/tables.go"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func pinnedTables(t *testing.T) foldTables {
	t.Helper()
	tables, err := parseTables(pinnedTableSource(t), unicode.CaseRanges)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestPinnedTablesReadCompleteLiteralData(t *testing.T) {
	tables := pinnedTables(t)
	if len(tables.ascii) != 128 || len(tables.orbit) != 94 || len(tables.ranges) != 336 {
		t.Fatal("pinned table counts changed", len(tables.ascii), len(tables.orbit), len(tables.ranges))
	}
	if tables.ascii['k'] != 0x212a || tables.ascii['s'] != 0x17f {
		t.Fatal("ASCII special orbits were replaced by ordinary lower/upper conversion")
	}
	for _, r := range tables.ranges {
		if r.lo == 0x139 {
			if r.delta != [3]int64{unicode.UpperLower, unicode.UpperLower, unicode.UpperLower} {
				t.Fatal("odd-origin UpperLower range changed")
			}
			return
		}
	}
	t.Fatal("missing odd-origin UpperLower range")
}

func TestTableParserRejectsMissingOrChangedASTShapes(t *testing.T) {
	source := string(pinnedTableSource(t))
	for _, test := range []struct{ name, before, after string }{
		{"missing ASCII", "var asciiFold =", "var hiddenFold ="},
		{"ASCII element type", "[MaxASCII + 1]uint16", "[MaxASCII + 1]int32"},
		{"ASCII length", "[MaxASCII + 1]uint16", "[MaxASCII]uint16"},
		{"ASCII keyed entry", "[MaxASCII + 1]uint16{\n\t0x0000,", "[MaxASCII + 1]uint16{\n\t0: 0x0000,"},
		{"ASCII computed entry", "[MaxASCII + 1]uint16{\n\t0x0000,", "[MaxASCII + 1]uint16{\n\tcompute(),"},
		{"ASCII overflow", "[MaxASCII + 1]uint16{\n\t0x0000,", "[MaxASCII + 1]uint16{\n\t0x10000,"},
		{"orbit element type", "var caseOrbit = []foldPair{", "var caseOrbit = []otherPair{"},
		{"orbit overflow", "var caseOrbit = []foldPair{\n\t{0x004B,", "var caseOrbit = []foldPair{\n\t{0x10000,"},
		{"duplicate orbit key", "var caseOrbit = []foldPair{", "var caseOrbit = []foldPair{\n\t{0x004B, 0x006B},"},
		{"CaseRanges alias", "var CaseRanges = _CaseRanges", "var CaseRanges = otherRanges"},
		{"case delta expression", "{0x0041, 0x005A, d{0, 32, 0}}", "{0x0041, 0x005A, d{0, compute(), 0}}"},
		{"case literal shape", "{0x0041, 0x005A, d{0, 32, 0}}", "{Lo: 0x0041, Hi: 0x005A, Delta: d{0, 32, 0}}"},
		{"Unicode version", `const Version = "17.0.0"`, `const Version = "16.0.0"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !strings.Contains(source, test.before) {
				t.Fatal("mutation no longer matches pinned source")
			}
			changed := strings.Replace(source, test.before, test.after, 1)
			if _, err := parseTables([]byte(changed), unicode.CaseRanges); err == nil {
				t.Fatal("changed table shape accepted")
			}
		})
	}
	if _, err := parseTables([]byte(source+"\nvar caseOrbit = []foldPair{}\n"), unicode.CaseRanges); err == nil {
		t.Fatal("duplicate declaration accepted")
	}
	changed := append([]unicode.CaseRange(nil), unicode.CaseRanges...)
	changed[0].Delta[unicode.LowerCase]++
	if _, err := parseTables([]byte(source), changed); err == nil {
		t.Fatal("exported CaseRanges/source mismatch accepted")
	}
}

func TestTableValidationRejectsUnsafeIntervalsAndAlternation(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*foldTables)
	}{
		{"missing ASCII", func(t *foldTables) { t.ascii = t.ascii[:127] }},
		{"empty orbit", func(t *foldTables) { t.orbit = nil }},
		{"empty ranges", func(t *foldTables) { t.ranges = nil }},
		{"reversed interval", func(t *foldTables) { t.ranges[0].hi = 0 }},
		{"negative Lo", func(t *foldTables) { t.ranges[0].lo = -1 }},
		{"overlapping ranges", func(t *foldTables) { t.ranges[1].lo = t.ranges[0].hi }},
		{"outside Unicode", func(t *foldTables) { t.ranges[len(t.ranges)-1].hi = unicode.MaxRune + 1 }},
		{"mixed UpperLower sentinel", func(t *foldTables) { t.ranges[8].delta[0] = 0 }},
		{"incomplete UpperLower pair", func(t *foldTables) { t.ranges[8].hi-- }},
		{"mapping below zero", func(t *foldTables) { t.ranges[0].delta[0] = -1000 }},
		{"mapping above Unicode", func(t *foldTables) { t.ranges[0].delta[0] = unicode.MaxRune }},
		{"unordered orbit", func(t *foldTables) { t.orbit[0], t.orbit[1] = t.orbit[1], t.orbit[0] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			tables := pinnedTables(t)
			test.edit(&tables)
			if _, err := generate(upstream{}, tables); err == nil {
				t.Fatal("unsafe table accepted")
			}
		})
	}
}
