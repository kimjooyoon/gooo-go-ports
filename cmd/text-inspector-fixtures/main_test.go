package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

func pinnedLock(t *testing.T) upstream {
	t.Helper()
	raw, err := os.ReadFile("../../upstream.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock upstream
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	return lock
}

func TestOracleRequiresPinnedToolchainAndSourceBytes(t *testing.T) {
	lock := pinnedLock(t)
	if err := checkOracleIdentity(lock, runtime.Version(), runtime.GOROOT()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*upstream){
		"other pinned version":   func(v *upstream) { v.GoVersion = "go1.28.0" },
		"other repository":       func(v *upstream) { v.GoRepository = "https://example.com/go" },
		"missing Go revision":    func(v *upstream) { v.GoRevision = "" },
		"missing property table": func(v *upstream) { delete(v.SourceSHA, "src/unicode/tables.go") },
		"changed oracle source":  func(v *upstream) { v.SourceSHA["src/unicode/graphic.go"] = strings.Repeat("0", 64) },
		"nonlocal source path":   func(v *upstream) { v.SourceSHA["../outside.go"] = strings.Repeat("0", 64) },
		"bad producer identity":  func(v *upstream) { v.CompilerRevision = strings.Repeat("z", 40) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := lock
			changed.SourceSHA = maps.Clone(lock.SourceSHA)
			change(&changed)
			if err := checkOracleIdentity(changed, runtime.Version(), runtime.GOROOT()); err == nil {
				t.Fatal("accepted changed oracle identity")
			}
		})
	}
	if err := checkOracleIdentity(lock, "go1.26.5", runtime.GOROOT()); err == nil {
		t.Fatal("accepted another executing Go toolchain")
	}
}

func TestAuthoredRowsRejectLostUnitsNarrowingAndSplitPairs(t *testing.T) {
	for name, e := range map[string]example{
		"missing arrays":     {},
		"negative point":     {Units: []uint16{0}, Points: []point{{-1, 0, 1}}},
		"outside Unicode":    {Units: []uint16{0}, Points: []point{{0x110000, 0, 1}}},
		"narrowed point":     {Units: []uint16{0x20}, Points: []point{{0x10020, 0, 1}}},
		"lost first unit":    {Units: []uint16{0x41, 0x42}, Points: []point{{0x42, 1, 1}}},
		"lost final unit":    {Units: []uint16{0x41, 0x42}, Points: []point{{0x41, 0, 1}}},
		"changed source":     {Units: []uint16{0x41}, Points: []point{{0x42, 0, 1}}},
		"wrong span":         {Units: []uint16{0xd83d, 0xde00}, Points: []point{{0x1f600, 0, 1}}},
		"split valid pair":   {Units: []uint16{0xd800, 0xdc00}, Points: []point{{0xd800, 0, 1}, {0xdc00, 1, 1}}},
		"replaced surrogate": {Units: []uint16{0xd800}, Points: []point{{0xfffd, 0, 1}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkExample(e); err == nil {
				t.Fatal("accepted malformed independent decoding expectation")
			}
		})
	}
	for _, e := range examples() {
		if err := checkExample(e); err != nil {
			t.Fatalf("valid authored example %s: %v", e.ID, err)
		}
	}
	overPoints := repeatExample("over-points", []uint16{0x41}, 0x41)
	overPoints.Units = append(overPoints.Units, 0x41)
	overPoints.Points = append(overPoints.Points, point{0x41, 256, 1})
	if err := checkExample(overPoints); err == nil {
		t.Fatal("accepted 257 code points")
	}
	overUnits := repeatExample("over-units", []uint16{0xd83d, 0xde00}, 0x1f600)
	overUnits.Units = append(overUnits.Units, 0)
	if err := checkExample(overUnits); err == nil {
		t.Fatal("accepted 513 source units")
	}
}

func TestFixtureRoundTripPreservesOriginalSurrogatesAndCoverage(t *testing.T) {
	doc, err := makeFixtures(pinnedLock(t))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var decoded fixtures
	if err := json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(doc, decoded) {
		t.Fatal("original numeric units or oracle rows changed through JSON", err)
	}
	if !strings.Contains(string(encoded), `"units":[55296]`) || !strings.Contains(string(encoded), `"codePoint":55296`) {
		t.Fatal("lone surrogate was replaced or serialized as text")
	}
	if len(doc.Cases) != 18 {
		t.Fatalf("changed application case roster: %d", len(doc.Cases))
	}
	byID := map[string]fixtureCase{}
	for _, c := range doc.Cases {
		byID[c.ID] = c
	}
	var actualMembers []int
	for _, p := range byID["all-go-white-space"].Expected {
		if !p.GoSpace {
			t.Fatal("authored property member absent from pinned Go", p.CodePoint)
		}
		actualMembers = append(actualMembers, p.CodePoint)
	}
	var tableMembers []int
	for _, r := range unicode.White_Space.R16 {
		for n := int(r.Lo); n <= int(r.Hi); n += int(r.Stride) {
			tableMembers = append(tableMembers, n)
		}
	}
	if len(unicode.White_Space.R32) != 0 || len(actualMembers) != 25 || !reflect.DeepEqual(actualMembers, tableMembers) {
		t.Fatal("not the complete pinned White_Space property")
	}
	for _, name := range []string{"lone-high-surrogate", "lone-low-surrogate", "reversed-surrogates"} {
		for _, p := range byID[name].Expected {
			if p.ValidScalar || p.GoSpace || p.UTF8Bytes != -1 || p.UTF16Units != -1 || p.Units != 1 {
				t.Fatal("lost distinction between raw code-unit span and invalid scalar length", p)
			}
		}
	}
	if len(byID["empty"].Units) != 0 || len(byID["empty"].Expected) != 0 || len(byID["limit-256-ascii"].Expected) != 256 || len(byID["limit-256-supplementary"].Expected) != 256 || len(byID["limit-256-supplementary"].Units) != 512 {
		t.Fatal("empty or exact-limit cases changed")
	}
	bytes, units := 0, 0
	for _, p := range byID["copied-separators"].Expected {
		bytes += p.UTF8Bytes
		units += p.UTF16Units
	}
	if bytes != 13 || units != 8 || len(byID["copied-separators"].Expected) != 7 {
		t.Fatal("copied-text use case lost its original sequence")
	}
}

func TestOutputNeverReplacesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "text-fixtures.json")
	doc, err := makeFixtures(pinnedLock(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFixtures(path, doc); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFixtures(path, fixtures{}); err == nil {
		t.Fatal("overwrote existing fixtures")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatal("existing fixture bytes changed", err)
	}
}
