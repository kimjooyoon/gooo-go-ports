package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStringByteCorpusAllByteNeedlesAndExactExtensions(t *testing.T) {
	added, err := stringByteCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 2 || added.PortReferenceCases != 1536 || added.ConsumerReferenceCases != 1536 || added.PortExtensionCases != 16 || added.ConsumerExtensionCases != 16 {
		t.Fatal("reviewed reference/extension scope changed", added)
	}
	texts := stringByteTexts()
	if len(texts) != 6 || len(texts[5]) != 1024 {
		t.Fatal("six-text scope or maximum native Text length changed")
	}
	for _, s := range added.Suites {
		if len(s.Cases) != 1552 {
			t.Fatal("suite length", s.Package)
		}
		seen := map[string]map[int64]bool{}
		for i, row := range s.Cases {
			text, needle, err := readStringByteInput(row.Inputs)
			if err != nil || !utf8.ValidString(text) {
				t.Fatal("invalid original Text or Integer", err)
			}
			if i < 1536 {
				if text != texts[i/256] || needle != int64(i%256) {
					t.Fatal("missing/reordered Cartesian byte case", i)
				}
				if seen[text] == nil {
					seen[text] = map[int64]bool{}
				}
				if seen[text][needle] {
					t.Fatal("duplicate reference tuple")
				}
				seen[text][needle] = true
			} else {
				if needle != stringByteInvalidNeedles()[i-1536] || (needle >= 0 && needle <= 255) {
					t.Fatal("extension narrowed or moved into original byte domain")
				}
				want := any(-1)
				if s.Package == stringByteConsumerPackage {
					want = map[string]any{"offset": -1, "found": false, "in_byte_domain": false, "byte_length": len(text)}
				}
				if !equalValue(mustJSON(t, row.Expected), want) {
					t.Fatal("invalid needle extension changed", row)
				}
			}
		}
		for _, needles := range seen {
			if len(needles) != 256 {
				t.Fatal("not all byte needles covered")
			}
		}
	}
}

func TestStringByteManualFirstMatchAndByteOffsetAnchors(t *testing.T) {
	for _, row := range []struct {
		text   string
		needle byte
		offset int
	}{
		{"", 0, -1}, {"ababa", 'a', 0}, {"ababa", 'b', 1}, {"ababa", 'z', -1},
		{"A\x00B\x00", 0, 1}, {"é가🙂xé", 0xa9, 1}, {"é가🙂xé", 0x80, 4},
		{"é가🙂xé", 0xb0, 3}, {"é가🙂xé", 0xf0, 5}, {"é가🙂xé", 'x', 9},
		{strings.Repeat("a", 1023) + "Z", 'Z', 1023},
	} {
		inputs := [][]any{{row.text, int64(row.needle)}}
		port, err := stringByteReferenceSuite(stringBytePackage, "IndexByte", inputs, func(text string, needle byte) any { return strings.IndexByte(text, needle) })
		if err != nil || !equalValue(mustJSON(t, port.Cases[0].Expected), row.offset) {
			t.Fatal("manual byte-offset anchor", row, err)
		}
		want := map[string]any{"offset": row.offset, "found": row.offset >= 0, "in_byte_domain": true, "byte_length": len(row.text)}
		if !reflect.DeepEqual(stringByteSearchOracle(row.text, row.needle), want) {
			t.Fatal("consumer oracle changed byte length or first-match semantics", row)
		}
	}
}

func TestStringByteReferenceValidationPrecedesOracleAndNarrowing(t *testing.T) {
	valid := []any{"é\x00", int64(169)}
	for name, rows := range map[string][][]any{
		"empty": nil, "arity": {{"a"}}, "non-Text": {{[]byte("a"), int64(97)}},
		"invalid UTF-8": {{string([]byte{0xff}), int64(255)}}, "oversized Text": {{strings.Repeat("a", 1025), int64(0)}},
		"oversized multibyte Text": {{strings.Repeat("é", 513), int64(0)}},
		"float needle":             {{"a", float64(97)}}, "JSON number needle": {{"a", json.Number("97")}},
		"int needle": {{"a", int(97)}}, "string needle": {{"a", "97"}},
		"negative": {{"a", int64(-1)}}, "overflow byte": {{"a", int64(256)}},
		"precision boundary": {{"a", int64(9007199254740993)}}, "int64 minimum": {{"a", int64(math.MinInt64)}},
		"duplicate": {valid, valid}, "bad later row": {valid, {"a", int64(256)}},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			if _, err := stringByteReferenceSuite(stringBytePackage, "IndexByte", rows, func(string, byte) any { calls++; return -1 }); err == nil || calls != 0 {
				t.Fatal("invalid roster reached independent oracle or byte narrowing", calls, err)
			}
		})
	}
}
