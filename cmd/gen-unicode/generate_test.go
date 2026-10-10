package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

func TestTableIntervalsRejectMalformedRanges(t *testing.T) {
	for name, table := range map[string]*unicode.RangeTable{
		"nil":             nil,
		"empty":           {},
		"zero stride":     {R16: []unicode.Range16{{Lo: 256, Hi: 260, Stride: 0}}},
		"reversed":        {R16: []unicode.Range16{{Lo: 260, Hi: 256, Stride: 1}}},
		"overlap":         {R16: []unicode.Range16{{Lo: 256, Hi: 260, Stride: 1}, {Lo: 260, Hi: 270, Stride: 1}}},
		"unsorted":        {R16: []unicode.Range16{{Lo: 300, Hi: 310, Stride: 1}, {Lo: 256, Hi: 270, Stride: 1}}},
		"LatinOffset":     {R16: []unicode.Range16{{Lo: 65, Hi: 90, Stride: 1}}},
		"R32 low":         {R32: []unicode.Range32{{Lo: 255, Hi: 260, Stride: 1}}},
		"above MaxRune":   {R32: []unicode.Range32{{Lo: 65536, Hi: 1114112, Stride: 1}}},
		"negative offset": {R16: []unicode.Range16{{Lo: 256, Hi: 260, Stride: 1}}, LatinOffset: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tableIntervals(table); err == nil {
				t.Fatal("malformed table accepted")
			}
		})
	}
}

func repositoryPin(t *testing.T) (upstream, []byte) {
	t.Helper()
	raw, err := os.ReadFile("../../upstream.json")
	if err != nil {
		t.Fatal(err)
	}
	var pin upstream
	if err := json.Unmarshal(raw, &pin); err != nil {
		t.Fatal(err)
	}
	return pin, raw
}

func TestDeterministicGenerationAndReadOnlyCheck(t *testing.T) {
	pin, raw := repositoryPin(t)
	one, err := generate(pin, unicode.Letter, unicode.Digit)
	if err != nil {
		t.Fatal(err)
	}
	two, err := generate(pin, unicode.Letter, unicode.Digit)
	if err != nil || !bytes.Equal(one, two) {
		t.Fatal("generation changed with identical inputs", err)
	}
	root := t.TempDir()
	path := filepath.Join(root, targetSource)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "upstream.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "--check"}, io.Discard); err == nil {
		t.Fatal("stale source passed --check")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != "stale source" {
		t.Fatal("--check modified its input", err)
	}
	if err := run([]string{"--root", root}, io.Discard); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(one, written) {
		t.Fatal("written bytes differ from deterministic generation", err)
	}
	if err := run([]string{"--root", root, "--check"}, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedBranchesMatchIndependentUnicodeOracle(t *testing.T) {
	pin, _ := repositoryPin(t)
	source, err := generate(pin, unicode.Letter, unicode.Digit)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name   string
		table  *unicode.RangeTable
		oracle func(rune) bool
	}{{"IsLetter", unicode.Letter, unicode.IsLetter}, {"IsDigit", unicode.Digit, unicode.IsDigit}} {
		body := parseGeneratedBody(t, source, item.name)
		inputs := map[int64]bool{math.MinInt64: true, math.MinInt32: true, -1: true,
			unicode.MaxRune: true, unicode.MaxRune + 1: true, math.MaxInt32: true, math.MaxInt64: true}
		// Includes Go's property-bit fast path independently of its RangeTable search.
		for r := int64(0); r <= 255; r++ {
			inputs[r] = true
		}
		ranges, err := tableIntervals(item.table)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range ranges {
			for _, x := range []int64{r.lo - 1, r.lo, r.lo + 1, r.lo + r.stride - 1,
				r.lo + r.stride, r.lo + r.stride + 1, r.hi - 1, r.hi, r.hi + 1} {
				inputs[x] = true
			}
		}
		for input := range inputs {
			want := input >= math.MinInt32 && input <= math.MaxInt32 && item.oracle(rune(input))
			if got := evaluateGeneratedBody(t, body, input); got != want {
				t.Fatalf("%s(%d) = %v, want %v", item.name, input, got, want)
			}
		}
	}
}
