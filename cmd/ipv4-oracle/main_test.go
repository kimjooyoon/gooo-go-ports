package main

import (
	"bytes"
	"crypto/sha256"
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

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	p, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(filename), "../../examples/ipv4-address-facts", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFrozenPublicCorpusAndOriginalPrefix(t *testing.T) {
	value, err := readCorpus(fixturePath(t, "corpus-202.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(fixturePath(t, "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 12164 || fmt.Sprintf("%x", sha256.Sum256(raw)) != "af414a801669b7f1a3607baca2638b951a2e7a25704bef41ac05ff2ed9aaf89f" {
		t.Fatal("original64 corpus changed")
	}
	original, err := decodeCorpus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(original.Cases) != 64 || len(value.Cases) != 202 || !reflect.DeepEqual(value.Cases[:64], original.Cases) {
		t.Fatal("expanded corpus does not preserve original64 ordered rows")
	}
	got, err := evaluateCorpus(value, pinnedGo)
	if err != nil {
		t.Fatal(err)
	}
	if got.CaseCount != 202 || got.ValidStdlibCases != 48 || got.InvalidExtensionCases != 154 || len(got.Cases) != 202 {
		t.Fatal("expanded population differs")
	}
	for i, row := range got.Cases {
		in := value.Cases[i].Inputs
		if row.Index != i || row.Name != value.Cases[i].Name || row.Cohort != value.Cases[i].Cohort || row.Inputs != (octets{*in.A, *in.B, *in.C, *in.D}) {
			t.Fatalf("original row/order lost at %d", i)
		}
		if row.Expected.A != row.Inputs.A || row.Expected.B != row.Inputs.B || row.Expected.C != row.Inputs.C || row.Expected.D != row.Inputs.D {
			t.Fatalf("original echoes lost at %d", i)
		}
		if row.Cohort == "invalid" && row.Expected != (report{A: row.Inputs.A, B: row.Inputs.B, C: row.Inputs.C, D: row.Inputs.D}) {
			t.Fatalf("invalid extension not allfalse at %d", i)
		}
	}
}

func TestInvalidPositiveAndNarrowingWitnesses(t *testing.T) {
	anchors := []struct {
		name     string
		in       octets
		positive func(netip.Addr) bool
	}{
		{"private10", octets{10, 0, 0, 0}, netip.Addr.IsPrivate},
		{"private172", octets{172, 16, 0, 0}, netip.Addr.IsPrivate},
		{"private192", octets{192, 168, 0, 0}, netip.Addr.IsPrivate},
		{"loopback", octets{127, 0, 0, 0}, netip.Addr.IsLoopback},
		{"unspecified", octets{0, 0, 0, 0}, netip.Addr.IsUnspecified},
		{"multicast", octets{224, 0, 0, 0}, netip.Addr.IsMulticast},
		{"linklocal", octets{169, 254, 0, 0}, netip.Addr.IsLinkLocalUnicast},
		{"global", octets{0, 0, 0, 1}, netip.Addr.IsGlobalUnicast},
	}
	for _, anchor := range anchors {
		t.Run(anchor.name, func(t *testing.T) {
			addr := netip.AddrFrom4([4]byte{byte(anchor.in.A), byte(anchor.in.B), byte(anchor.in.C), byte(anchor.in.D)})
			if !anchor.positive(addr) {
				t.Fatal("test anchor is not otherwise positive in Go")
			}
			for position := range 4 {
				base := [4]int64{anchor.in.A, anchor.in.B, anchor.in.C, anchor.in.D}
				values := []int64{-1, 256, -1 << 63, 1<<63 - 1, base[position] - 256, base[position] + 256}
				for _, invalid := range values {
					changed := base
					changed[position] = invalid
					in := octets{changed[0], changed[1], changed[2], changed[3]}
					want := report{A: in.A, B: in.B, C: in.C, D: in.D}
					if got := classify(in); got != want {
						t.Fatalf("coordinate %d original %d was narrowed or classified: %#v", position, invalid, got)
					}
				}
			}
		})
	}
}

func TestValidMethodMappingAndOverlaps(t *testing.T) {
	inputs := []octets{{10, 0, 0, 1}, {172, 16, 4, 5}, {192, 168, 6, 7}, {127, 0, 0, 1}, {0, 0, 0, 0}, {224, 1, 2, 3}, {169, 254, 4, 5}, {0, 0, 0, 1}, {100, 64, 0, 1}, {192, 0, 2, 1}, {198, 51, 100, 1}, {203, 0, 113, 1}, {255, 0, 0, 1}, {255, 255, 255, 255}}
	for _, in := range inputs {
		a := netip.AddrFrom4([4]byte{byte(in.A), byte(in.B), byte(in.C), byte(in.D)})
		got := classify(in)
		actual := [6]bool{got.IsPrivate, got.IsLoopback, got.IsUnspecified, got.IsMulticast, got.IsLinkLocalUnicast, got.IsGlobalUnicast}
		want := [6]bool{a.IsPrivate(), a.IsLoopback(), a.IsUnspecified(), a.IsMulticast(), a.IsLinkLocalUnicast(), a.IsGlobalUnicast()}
		if !got.Valid || actual != want || (octets{got.A, got.B, got.C, got.D}) != in {
			t.Fatalf("method/field mapping differs for %v", in)
		}
	}
	for _, in := range inputs[:3] {
		got := classify(in)
		if !got.IsPrivate || !got.IsGlobalUnicast {
			t.Fatalf("private/global overlap was lost for %v", in)
		}
	}
	broadcast := classify(octets{255, 255, 255, 255})
	if !broadcast.Valid || broadcast.IsGlobalUnicast {
		t.Fatal("valid limited broadcast confused with invalid extension/global")
	}
}

func oneCase(inputs string) []byte {
	return []byte(`{"schema":"gooo/ipv4-classification-corpus/v1","cases":[{"index":0,"name":"probe","cohort":"boundary","inputs":` + inputs + `}]}`)
}

func TestDecodeCorpusRejectsKindsAndInt64Overflow(t *testing.T) {
	fields := []string{"a", "b", "c", "d"}
	badTokens := []string{"null", "true", `"1"`, "1.0", "1e0", "9223372036854775808", "-9223372036854775809"}
	for _, field := range fields {
		for _, token := range badTokens {
			parts := make([]string, 0, 4)
			for _, key := range fields {
				value := "0"
				if key == field {
					value = token
				}
				parts = append(parts, fmt.Sprintf("%q:%s", key, value))
			}
			if _, err := decodeCorpus(oneCase("{" + strings.Join(parts, ",") + "}")); err == nil {
				t.Fatalf("accepted %s=%s", field, token)
			}
		}
		parts := make([]string, 0, 3)
		for _, key := range fields {
			if key != field {
				parts = append(parts, fmt.Sprintf("%q:0", key))
			}
		}
		if _, err := decodeCorpus(oneCase("{" + strings.Join(parts, ",") + "}")); err == nil {
			t.Fatalf("accepted missing %s", field)
		}
	}
	// Domain-invalid values are still exact int64 tokens, never malformed input.
	if _, err := decodeCorpus(oneCase(`{"a":-9223372036854775808,"b":9223372036854775807,"c":-1,"d":256}`)); err != nil {
		t.Fatalf("exact int64 endpoints refused as transport: %v", err)
	}
}

func TestDecodeCorpusRejectsDuplicateUnknownAndTrailing(t *testing.T) {
	valid := oneCase(`{"a":10,"b":0,"c":0,"d":1}`)
	if _, err := decodeCorpus(valid); err != nil {
		t.Fatal("positive decoder control:", err)
	}
	cases := map[string][]byte{
		"duplicate":         oneCase(`{"a":10,"a":11,"b":0,"c":0,"d":1}`),
		"escaped-duplicate": oneCase(`{"a":10,"\u0061":11,"b":0,"c":0,"d":1}`),
		"unknown":           oneCase(`{"a":10,"b":0,"c":0,"d":1,"extra":0}`),
		"trailing":          append(append([]byte(nil), valid...), []byte(` {}`)...),
		"nonobject":         []byte(`[]`),
		"malformed":         []byte(`{"schema":`),
		"raw-invalid-UTF8":  append(append([]byte(nil), valid...), 0xff),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCorpus(raw); err == nil {
				t.Fatal("malformed corpus accepted")
			}
		})
	}
}

func TestReportEncodingRetainsAllFieldsAndInt64s(t *testing.T) {
	in := octets{-1 << 63, 1<<63 - 1, -1, 256}
	raw, err := json.Marshal(classify(in))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "-9223372036854775808", "b": "9223372036854775807", "c": "-1", "d": "256", "valid": "false", "is_private": "false", "is_loopback": "false", "is_unspecified": "false", "is_multicast": "false", "is_link_local_unicast": "false", "is_global_unicast": "false"}
	if len(fields) != 11 {
		t.Fatal("false or echo fields omitted")
	}
	for key, token := range want {
		if string(fields[key]) != token {
			t.Fatalf("field %s changed JSON kind/value", key)
		}
	}
}

func TestReadCorpusRefusesAliasAndChangedBytes(t *testing.T) {
	fixture := fixturePath(t, "corpus-202.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(dir, "corpus.json")
	changed := append([]byte(nil), raw...)
	changed[1] = ' '
	if err := os.WriteFile(copyPath, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCorpus(copyPath); err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("same-length changed bytes missed hash guard: %v", err)
	}
	link := filepath.Join(dir, "alias.json")
	if err := os.Symlink(fixture, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readCorpus(link); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("corpus alias accepted: %v", err)
	}
}

func TestEvaluateCorpusRejectsWrongRoster(t *testing.T) {
	for _, mutation := range []string{"count", "index", "name", "cohort", "tuple"} {
		t.Run(mutation, func(t *testing.T) {
			value, err := readCorpus(fixturePath(t, "corpus-202.json"))
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "count":
				value.Cases = value.Cases[:201]
			case "index":
				*value.Cases[1].Index = 0
			case "name":
				value.Cases[1].Name = value.Cases[0].Name
			case "cohort":
				value.Cases[64].Cohort = "boundary"
			case "tuple":
				value.Cases[1].Inputs = value.Cases[0].Inputs
			}
			if _, err := evaluateCorpus(value, pinnedGo); err == nil {
				t.Fatal("wrong roster accepted")
			}
		})
	}
}

func TestCLIContractAndOrderedOutput(t *testing.T) {
	path := fixturePath(t, "corpus-202.json")
	var stdout bytes.Buffer
	if err := run([]string{"--corpus", path}, &stdout, pinnedGo); err != nil {
		t.Fatal(err)
	}
	var got output
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != outputSchema || got.GoVersion != pinnedGo || got.CorpusSHA256 != corpusSHA256 || got.CorpusBytes != corpusBytes || got.CaseCount != 202 || got.RequiredReportFields != 11 || got.OriginalEchoes != 4 || len(got.Cases) != 202 {
		t.Fatal("CLI output contract changed")
	}
	for i, row := range got.Cases {
		if row.Index != i {
			t.Fatal("CLI row order changed")
		}
	}
	for _, args := range [][]string{nil, {"--corpus", path, "extra"}, {"--unknown"}} {
		stdout.Reset()
		if err := run(args, &stdout, pinnedGo); err == nil || stdout.Len() != 0 {
			t.Fatal("invalid CLI emitted a result")
		}
	}
	stdout.Reset()
	if err := run([]string{"--corpus", path}, &stdout, "go1.27.1"); err == nil || stdout.Len() != 0 {
		t.Fatal("wrong Go identity accepted")
	}
}
