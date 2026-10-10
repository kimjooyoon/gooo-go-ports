package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"unicode"
)

func TestWhitespaceCoverageAndExactInputs(t *testing.T) {
	added, err := whitespaceCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 2 || added.PortReferenceCases != 506 || added.ConsumerReferenceCases != 506 || added.PortRejectionCases != 8 || added.ConsumerRejectionCases != 8 {
		t.Fatalf("coverage changed: %+v", added)
	}
	for index, s := range added.Suites {
		seen := map[int64]bool{}
		for i, c := range s.Cases {
			n := c.Inputs[0].(int64)
			if seen[n] {
				t.Fatalf("duplicate original input %d", n)
			}
			seen[n] = true
			if !reflect.DeepEqual(c.Inputs, added.Suites[0].Cases[i].Inputs) {
				t.Fatal("consumer changed original input roster")
			}
		}
		for n := int64(0); n <= 255; n++ {
			if !seen[n] {
				t.Fatalf("missing Latin-1 value %d", n)
			}
		}
		for _, n := range whitespaceMembers() {
			for delta := int64(-1); delta <= 1; delta++ {
				if !seen[n+delta] {
					t.Fatalf("missing member/neighbor %d", n+delta)
				}
			}
		}
		for _, n := range whitespaceBoundaryInputs() {
			if !seen[n] {
				t.Fatalf("missing rune boundary %d", n)
			}
		}
		want := any(false)
		size := whitespacePortBatchSize
		if index == 1 {
			want = rejectedWhitespaceInspection()
			size = whitespaceInspectorBatchSize
		}
		for _, n := range whitespaceRejectedInputs() {
			checkConsumerCase(t, s, []any{n}, want)
		}
		maxBytes, batches := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 || len(batch.Cases) > 128 {
				t.Fatalf("compiler batch budget exceeded: %d %v", len(raw)+1, err)
			}
			maxBytes = max(maxBytes, len(raw)+1)
			batches++
			var doc struct {
				Cases []struct {
					Inputs map[string]json.RawMessage `json:"inputs"`
				} `json:"cases"`
			}
			if err = json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			for i, row := range doc.Cases {
				if len(row.Inputs) != 1 || !equalValue(row.Inputs[s.Package+":"+s.Activity], batch.Cases[i].Inputs[0]) {
					t.Fatal("serialized original rune changed")
				}
			}
		}
		t.Logf("%s:%s: reference=%d rejection=8 total=%d batches=%d maxbytes=%d", s.Package, s.Activity, added.PortReferenceCases, len(s.Cases), batches, maxBytes)
	}
}

func TestWhitespacePinnedMembersAndNamedDistinctions(t *testing.T) {
	if unicode.Version != "17.0.0" {
		t.Fatalf("Unicode oracle version changed: %s", unicode.Version)
	}
	var members []int64
	for _, r := range unicode.White_Space.R16 {
		for n := uint32(r.Lo); n <= uint32(r.Hi); n += uint32(r.Stride) {
			members = append(members, int64(n))
		}
	}
	if len(unicode.White_Space.R32) != 0 || !reflect.DeepEqual(members, whitespaceMembers()) {
		t.Fatalf("coverage misses original White_Space members: %v", members)
	}
	added, err := whitespaceCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range whitespaceMembers() {
		checkConsumerCase(t, added.Suites[0], []any{n}, true)
	}
	for _, n := range []int64{0x001c, 0x001d, 0x001e, 0x001f, 0x180e, 0x200b, 0x200e, 0x200f, 0x2060, 0x2800, 0xfeff, -65527, -65504, -65403, 0x10009, 0x10020, 0x10085, 0xd800, 0xdfff, 0x110000, math.MinInt32, math.MaxInt32} {
		checkConsumerCase(t, added.Suites[0], []any{n}, false)
	}
	for _, n := range []int64{9, 32, 133, 160, 5760, 8199, 8239, 12288} {
		v := whitespaceInspectionOracle(rune(n)).(map[string]any)
		if v["in_rune_domain"] != true || v["valid_scalar"] != true || v["is_go_space"] != true || v["is_ascii_space"] != (n <= 127) {
			t.Fatalf("wrong whitespace consumer semantics: %v", v)
		}
	}
	for _, n := range []int64{-1, 0xd800, 0x110000, math.MinInt32, math.MaxInt32} {
		v := whitespaceInspectionOracle(rune(n)).(map[string]any)
		if v["in_rune_domain"] != true || v["valid_scalar"] != false || v["is_go_space"] != false || v["utf8_bytes"] != -1 || v["utf16_units"] != -1 {
			t.Fatalf("invalid Unicode confused with outside-rune extension: %v", v)
		}
	}
}

func TestWhitespaceValidatesWholeRosterBeforeOracle(t *testing.T) {
	valid := []any{int64(32)}
	bad := [][]any{nil, {}, {int64(32), int64(0)}, valid}
	for _, n := range whitespaceRejectedInputs() {
		bad = append(bad, []any{n})
	}
	for _, v := range []any{int32(32), 32, float64(32), "32", json.Number("32"), nil} {
		bad = append(bad, []any{v})
	}
	for _, input := range bad {
		calls := 0
		_, err := whitespaceSuite("test", "test", [][]any{valid, input}, func(rune) any { calls++; return false })
		if err == nil || calls != 0 {
			t.Fatalf("invalid input reached oracle: %v calls=%d", input, calls)
		}
	}
	if _, err := whitespaceSuite("test", "test", nil, func(rune) any { t.Fatal("empty oracle call"); return nil }); err == nil {
		t.Fatal("empty roster accepted")
	}
	inputs := [][]any{{int64(math.MinInt32)}, {int64(math.MaxInt32)}}
	var observed []int64
	s, err := whitespaceSuite("test", "test", inputs, func(r rune) any { observed = append(observed, int64(r)); return false })
	if err != nil || !reflect.DeepEqual(observed, []int64{math.MinInt32, math.MaxInt32}) {
		t.Fatal("original int32 endpoints changed", err)
	}
	inputs[0][0] = int64(0)
	if s.Cases[0].Inputs[0] != int64(math.MinInt32) {
		t.Fatal("saved input aliases caller storage")
	}
}

func whitespaceLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	s, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	u, err := unicodeInspectorCorpus(s)
	if err != nil {
		t.Fatal(err)
	}
	c, err := colorCorpus()
	if err != nil {
		t.Fatal(err)
	}
	r, err := rotationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	d, err := durationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(append(append(append(s, u.Suites...), c.Suites...), r.Suites...), d.Suites...)
}

func TestWhitespacePreservesExisting14093Cases(t *testing.T) {
	before := whitespaceLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	if count != 14093 {
		t.Fatalf("old corpus count changed: %d", count)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from unchanged 5ba8b38 suites before adding the whitespace corpus.
	const existing = "431004e621d6cbf668edb4ffec2cbc7f1d0950ee02abbe7977b8164d85020796"
	if hash != existing {
		t.Fatalf("existing 14093-case corpus changed: %s", hash)
	}
	if _, err := whitespaceCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, whitespaceLegacyCorpus(t)) {
		t.Fatal("new construction mutated old corpus")
	}
}
