package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func profileFixture(t *testing.T, dir, file string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	p, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(source), "../../examples", dir, file))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func profileRaw(t *testing.T, dir, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(profileFixture(t, dir, file))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestPublicPrefixPairCorporaAndOriginal18(t *testing.T) {
	prefix, err := decodePrefixRows(profileRaw(t, "ipv4-prefix-probe", "corpus-18.json"))
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := decodePairRows(profileRaw(t, "ipv4-rule-pair", "corpus-112.json"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodePairRows(profileRaw(t, "ipv4-rule-pair", "corpus-18.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) != 18 || len(pairs) != 112 || len(original) != 18 || !reflect.DeepEqual(pairs[:18], original) {
		t.Fatal("frozen ordered populations/original18 differ")
	}
	counts := make(map[int64]int)
	lengths := make(map[int64]int)
	for i, row := range pairs {
		if row.Index != i {
			t.Fatal("case order changed")
		}
		counts[row.Expected.Reason]++
		if i >= 18 && i < 84 {
			lengths[row.Inputs.Left.Bits]++
		}
	}
	if !reflect.DeepEqual(counts, map[int64]int{0: 33, 1: 1, 2: 1, 3: 35, 4: 2, 5: 40}) {
		t.Fatalf("reason partition differs: %v", counts)
	}
	for bits := int64(0); bits <= 32; bits++ {
		if lengths[bits] != 2 {
			t.Fatalf("missing two boundary inputs for /%d", bits)
		}
	}
	// /0's second, different host is still inside the same valid default prefix.
	if pairs[19].Inputs.Left.Bits != 0 || !pairs[19].Expected.Overlap || !pairs[19].Expected.Conflict {
		t.Fatal("/0 other host misclassified")
	}
	for i, row := range prefix {
		if row.Index != i {
			t.Fatal("prefix order lost")
		}
	}
}
func TestPrefixFactsQuestionsAreIndependent(t *testing.T) {
	cases := []struct {
		in   prefixProfileInputs
		want prefixFacts
	}{
		{prefixProfileInputs{networkPrefix{255, 255, 255, 255, 0}, octets{0, 0, 0, 0}, networkPrefix{0, 0, 0, 0, 32}}, prefixFacts{true, true}},
		{prefixProfileInputs{networkPrefix{0, 0, 0, 0, 32}, octets{0, 0, 0, 0}, networkPrefix{0, 0, 0, 0, 32}}, prefixFacts{true, true}},
		{prefixProfileInputs{networkPrefix{10, 1, 2, 3, 8}, octets{10, -1 << 63, 0, 1}, networkPrefix{10, 20, 30, 40, 16}}, prefixFacts{false, true}},
		{prefixProfileInputs{networkPrefix{10, 1, 2, 3, 8}, octets{10, 99, 0, 1}, networkPrefix{10, 0, 0, 0, 1<<63 - 1}}, prefixFacts{true, false}},
		{prefixProfileInputs{networkPrefix{10, 1, 2, 3, -1}, octets{10, 99, 0, 1}, networkPrefix{10, 0, 0, 0, 8}}, prefixFacts{}},
	}
	for _, c := range cases {
		if got := assessPrefix(c.in); got != c.want {
			t.Fatalf("independent prefix questions differ: %#v", got)
		}
	}
}
func TestPairReasonPrecedenceContextAndWitness(t *testing.T) {
	left := pairRule{10, 1, 2, 3, 8, 1, 42, -1 << 63}
	right := pairRule{10, 20, 30, 40, 16, 0, 42, -1 << 63}
	cases := []struct {
		name                                 string
		l, r                                 pairRule
		reason                               int64
		valid, comparable, overlap, opposing bool
	}{
		{"conflict", left, right, 5, true, true, true, true},
		{"scope-first", left, pairRule{10, 20, 30, 40, 16, 0, 43, 1<<63 - 1}, 1, true, false, true, true},
		{"priority", left, pairRule{10, 20, 30, 40, 16, 0, 42, 1<<63 - 1}, 2, true, false, true, true},
		{"disjoint-before-same-effect", left, pairRule{11, 20, 30, 40, 16, 1, 42, -1 << 63}, 3, true, true, false, false},
		{"same-effect", left, pairRule{10, 20, 30, 40, 16, 1, 42, -1 << 63}, 4, true, true, true, false},
		{"invalid-before-context", pairRule{-1 << 63, 0, 0, 0, 8, 1, 42, -1 << 63}, pairRule{10, 0, 0, 0, 8, 0, 43, 1<<63 - 1}, 0, false, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := assessPair(c.l, c.r)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reason != c.reason || got.Valid != c.valid || got.Comparable != c.comparable || got.Overlap != c.overlap || got.OpposingEffect != c.opposing {
				t.Fatalf("contract facts differ: %#v", got)
			}
			if got.Conflict != (c.reason == 5) || got.WitnessValid != got.Conflict {
				t.Fatal("false conflict/witness")
			}
			if got.Conflict && [4]int64{got.WitnessA, got.WitnessB, got.WitnessC, got.WitnessD} != [4]int64{10, 20, 0, 0} {
				t.Fatal("more-specific canonical witness lost")
			}
			if !got.Conflict && [4]int64{got.WitnessA, got.WitnessB, got.WitnessC, got.WitnessD} != [4]int64{} {
				t.Fatal("non-conflict witness is nonzero")
			}
		})
	}
}
func TestPairAll33LengthsActualMembershipAndSwap(t *testing.T) {
	for bits := int64(0); bits <= 32; bits++ {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			left := pairRule{198, 18, 73, 165, bits, 1, 1, 1<<63 - 1}
			p := netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 18, 73, 165}), int(bits))
			a := p.Masked().Addr().As4()
			right := pairRule{int64(a[0]), int64(a[1]), int64(a[2]), int64(a[3]), 32, 0, 1, 1<<63 - 1}
			got, err := assessPair(left, right)
			if err != nil {
				t.Fatal(err)
			}
			swapped, err := assessPair(right, left)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Conflict || !got.WitnessValid || got.Reason != 5 || got != swapped {
				t.Fatalf("nested/swap /%d: %#v %#v", bits, got, swapped)
			}
			want := [4]int64{int64(a[0]), int64(a[1]), int64(a[2]), int64(a[3])}
			if [4]int64{got.WitnessA, got.WitnessB, got.WitnessC, got.WitnessD} != want {
				t.Fatal("witness field/order/masking differs")
			}
			if !p.Contains(netip.AddrFrom4(a)) {
				t.Fatal("test positive host is not contained")
			}
			outside := [4]byte{198 ^ 128, 18, 73, 165}
			right.A, right.B, right.C, right.D = int64(outside[0]), 18, 73, 165
			other, err := assessPair(left, right)
			if err != nil {
				t.Fatal(err)
			}
			if bits == 0 {
				if !other.Overlap || !other.Conflict {
					t.Fatal("/0 rejected another valid host")
				}
			} else if other.Overlap || other.Conflict || other.Reason != 3 {
				t.Fatal("outside leading bit overlapped")
			}
		})
	}
	zero := pairRule{0, 0, 0, 0, 32, 0, 7, 0}
	def := pairRule{255, 255, 255, 255, 0, 1, 7, 0}
	r, err := assessPair(zero, def)
	if err != nil || !r.WitnessValid || r.Reason != 5 || [4]int64{r.WitnessA, r.WitnessB, r.WitnessC, r.WitnessD} != [4]int64{} {
		t.Fatal("valid zero witness conflated with no witness")
	}
}
func TestPairEverySingleInvalidDomainFieldBeforeCast(t *testing.T) {
	baseLeft := pairRule{203, 0, 113, 73, 24, 1, 9, 0}
	baseRight := pairRule{203, 0, 113, 201, 32, 0, 9, 0}
	positive, err := assessPair(baseLeft, baseRight)
	if err != nil || !positive.Conflict {
		t.Fatal("invalid-mutation base is not a conflict")
	}
	for side := range 2 {
		for field := range 7 {
			bad := []int64{-1, 256, -1 << 63, 1<<63 - 1}
			if field == 4 {
				bad = []int64{-1, 33, -1 << 63, 1<<63 - 1}
			}
			if field == 5 {
				bad = []int64{-1, 2, -1 << 63, 1<<63 - 1}
			}
			if field == 6 {
				bad = []int64{0, -1, -1 << 63}
			}
			for _, value := range bad {
				left, right := baseLeft, baseRight
				target := &left
				if side == 1 {
					target = &right
				}
				fields := []*int64{&target.A, &target.B, &target.C, &target.D, &target.Bits, &target.Effect, &target.Scope}
				*fields[field] = value
				got, err := assessPair(left, right)
				if err != nil || got != (pairFacts{}) {
					t.Fatalf("side%d field%d int64%d narrowed/classified: %#v %v", side, field, value, got, err)
				}
			}
		}
	}
	for _, priority := range []int64{-1 << 63, -1, 0, 1<<63 - 1} {
		left, right := baseLeft, baseRight
		left.Priority, right.Priority = priority, priority
		got, err := assessPair(left, right)
		if err != nil || !got.Valid || !got.Conflict {
			t.Fatalf("legal priority%d refused", priority)
		}
	}
}

func integerWire(keys []string, values []int64) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	for i, k := range keys {
		m[k] = json.RawMessage(fmt.Sprint(values[i]))
	}
	return m
}
func pairWire(t *testing.T, left, right map[string]json.RawMessage) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"schema": "gooo/ipv4-rule-pair-inputs/v1", "cases": []any{map[string]any{"name": "probe", "left": left, "right": right}}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestPairStrictAll16IntegerTokens(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "bits", "effect", "scope", "priority"}
	lv := []int64{10, 1, 2, 3, 8, 1, 42, -1 << 63}
	rv := []int64{10, 20, 30, 40, 16, 0, 42, -1 << 63}
	if rows, err := decodePairRows(pairWire(t, integerWire(keys, lv), integerWire(keys, rv))); err != nil || len(rows) != 1 || !rows[0].Expected.Conflict {
		t.Fatal("positive parser control", err)
	}
	for side := range 2 {
		for _, key := range keys {
			for _, token := range []string{"null", "true", `"1"`, "1.0", "1e0", "9223372036854775808", "-9223372036854775809"} {
				l, r := integerWire(keys, lv), integerWire(keys, rv)
				target := l
				if side == 1 {
					target = r
				}
				target[key] = json.RawMessage(token)
				if _, err := decodePairRows(pairWire(t, l, r)); err == nil {
					t.Fatalf("accepted side%d %s=%s", side, key, token)
				}
			}
			l, r := integerWire(keys, lv), integerWire(keys, rv)
			target := l
			if side == 1 {
				target = r
			}
			delete(target, key)
			if _, err := decodePairRows(pairWire(t, l, r)); err == nil {
				t.Fatalf("missing side%d %s accepted", side, key)
			}
		}
	}
}
func TestPrefixStrictAll14IntegerTokens(t *testing.T) {
	keys := [][]string{{"a", "b", "c", "d", "bits"}, {"a", "b", "c", "d"}, {"a", "b", "c", "d", "bits"}}
	names := []string{"prefix", "address", "other_prefix"}
	values := [][]int64{{10, 1, 2, 3, 8}, {10, 99, 0, 1}, {10, 20, 30, 40, 16}}
	makeRaw := func(record, field int, token string) []byte {
		row := map[string]any{"name": "probe"}
		for j, name := range names {
			m := integerWire(keys[j], values[j])
			if j == record {
				if token == "missing" {
					delete(m, keys[j][field])
				} else {
					m[keys[j][field]] = json.RawMessage(token)
				}
			}
			row[name] = m
		}
		b, err := json.Marshal(map[string]any{"schema": "gooo/ipv4-prefix-pilot-inputs/v1", "cases": []any{row}})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if rows, err := decodePrefixRows(makeRaw(-1, 0, "")); err != nil || len(rows) != 1 || rows[0].Expected != (prefixFacts{true, true}) {
		t.Fatal("positive prefix parser control", err)
	}
	for j := range names {
		for k := range keys[j] {
			for _, token := range []string{"null", "false", `"0"`, "0.0", "0e0", "9223372036854775808", "-9223372036854775809", "missing"} {
				if _, err := decodePrefixRows(makeRaw(j, k, token)); err == nil {
					t.Fatalf("accepted %s.%s=%s", names[j], keys[j][k], token)
				}
			}
		}
	}
}
func TestProfileDuplicateUnknownAndUnicodeRefusals(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "bits", "effect", "scope", "priority"}
	valid := pairWire(t, integerWire(keys, []int64{10, 1, 2, 3, 8, 1, 42, 0}), integerWire(keys, []int64{10, 20, 30, 40, 16, 0, 42, 0}))
	if _, err := decodePairRows(valid); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]byte{
		"duplicate":         []byte(strings.Replace(string(valid), `"a":10`, `"a":10,"a":11`, 1)),
		"escaped-duplicate": []byte(strings.Replace(string(valid), `"a":10`, `"a":10,"\u0061":11`, 1)),
		"uppercase-key":     []byte(strings.Replace(string(valid), `"a":10`, `"A":10`, 1)),
		"unknown":           []byte(strings.Replace(string(valid), `"a":10`, `"extra":0,"a":10`, 1)),
		"high-surrogate":    []byte(strings.Replace(string(valid), `"probe"`, `"\ud800"`, 1)),
		"low-surrogate":     []byte(strings.Replace(string(valid), `"probe"`, `"\udc00"`, 1)),
		"wrong-pair":        []byte(strings.Replace(string(valid), `"probe"`, `"\ud800\u0041"`, 1)),
		"trailing":          append(append([]byte{}, valid...), []byte(` {}`)...),
		"raw-UTF8":          append(append([]byte{}, valid...), 0xff),
		"array":             []byte(`[]`), "null": []byte(`null`), "malformed": []byte(`{"schema":`),
	}
	for name, raw := range bad {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, valid) {
				t.Fatal("mutation did not change base")
			}
			if _, err := decodePairRows(raw); err == nil {
				t.Fatal("malformed transport accepted")
			}
		})
	}
	for _, name := range []string{`"pair-\ud83d\ude00"`, `"literal-\\ud800"`, `"replacement-\ufffd"`} {
		if _, err := decodePairRows([]byte(strings.Replace(string(valid), `"probe"`, name, 1))); err != nil {
			t.Fatalf("valid string escape refused: %s %v", name, err)
		}
	}
}
func TestPairFactEncodingAndOriginalInputInt64s(t *testing.T) {
	raw, err := json.Marshal(pairFacts{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	bools := []string{"valid", "comparable", "overlap", "opposing_effect", "conflict", "witness_valid"}
	ints := []string{"witness_a", "witness_b", "witness_c", "witness_d", "reason"}
	if len(fields) != 11 {
		t.Fatal("required fields omitted")
	}
	for _, k := range bools {
		if string(fields[k]) != "false" {
			t.Fatal("false kind lost", k)
		}
	}
	for _, k := range ints {
		if string(fields[k]) != "0" {
			t.Fatal("zero integer kind lost", k)
		}
	}
	in := pairProfileInputs{pairRule{-1 << 63, 0, 0, 0, 8, 1, 1, 1<<63 - 1}, pairRule{10, 0, 0, 0, 8, 0, 1, -1 << 63}}
	facts, err := assessPair(in.Left, in.Right)
	if err != nil || facts != (pairFacts{}) {
		t.Fatal("invalid extension")
	}
	encoded, err := json.Marshal(pairReference{0, "invalid", in, facts})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"a":-9223372036854775808`)) || !bytes.Contains(encoded, []byte(`"priority":9223372036854775807`)) {
		t.Fatal("input endpoints lost")
	}
	var got pairReference
	if err := json.Unmarshal(encoded, &got); err != nil || got.Inputs != in {
		t.Fatal("original16inputvalues changed")
	}
}
func TestProfileFilePinsAliasesAndChangedBytes(t *testing.T) {
	path := profileFixture(t, "ipv4-rule-pair", "corpus-112.json")
	raw, ref, err := readProfileFile(path, 49496, pairCorpusSHA)
	if err != nil || len(raw) != 49496 || ref.SHA256 != pairCorpusSHA {
		t.Fatal("positive read control", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]byte{}, raw...)
	changed[1] = ' '
	target := filepath.Join(dir, "changed.json")
	if err := os.WriteFile(target, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readProfileFile(target, 49496, pairCorpusSHA); err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatal("same-length hash mutation accepted", err)
	}
	alias := filepath.Join(dir, "alias.json")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readProfileFile(alias, 49496, pairCorpusSHA); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatal("symlink alias accepted", err)
	}
}
func TestCombinedProfileCLIAndOldDefaultBytes(t *testing.T) {
	args := []string{"--profile", "prefix-rule-pair", "--prefix-corpus", profileFixture(t, "ipv4-prefix-probe", "corpus-18.json"), "--corpus", profileFixture(t, "ipv4-rule-pair", "corpus-112.json"), "--contract", profileFixture(t, "ipv4-rule-pair", "contract.json")}
	var out bytes.Buffer
	if err := run(args, &out, pinnedGo); err != nil {
		t.Fatal(err)
	}
	var result prefixPairOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != profileOutputSchema || result.GoVersion != pinnedGo || result.Contract.SHA256 != profileContractSHA || result.PrefixCaseCount != 18 || result.PairCaseCount != 112 || len(result.PrefixCases) != 18 || len(result.PairCases) != 112 {
		t.Fatal("combined CLI contract differs")
	}
	for i, row := range result.PairCases {
		if row.Index != i {
			t.Fatal("output order lost")
		}
	}
	for _, bad := range [][]string{{"--profile", "typo"}, {"--profile", "prefix-rule-pair"}, append(append([]string{}, args...), "extra"), append(append([]string{}, args...), "--unknown")} {
		out.Reset()
		if err := run(bad, &out, pinnedGo); err == nil || out.Len() != 0 {
			t.Fatal("invalid profile CLI emitted result")
		}
	}
	out.Reset()
	if err := run(args, &out, "go1.27.1"); err == nil || out.Len() != 0 {
		t.Fatal("wrong Go identity accepted")
	}
	oldPath := fixturePath(t, "corpus-202.json")
	inputs, err := readCorpus(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := evaluateCorpus(inputs, pinnedGo)
	if err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	encoder := json.NewEncoder(&original)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(want); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"--corpus", oldPath}, &out, pinnedGo); err != nil || !bytes.Equal(out.Bytes(), original.Bytes()) {
		t.Fatal("old default202 serialization changed", err)
	}
}
