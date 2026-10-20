package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUUIDParseErrorValueAnchors(t *testing.T) {
	for _, c := range []struct {
		input string
		want  uuidParseOutput
	}{
		{"00112233-4455-6677-8899-aabbccddeeff", uuidParseOutput{true, "00112233-4455-6677-8899-aabbccddeeff", "", false}},
		{"00112233445566778899aabbccddee0g", uuidParseOutput{false, "00112233-4455-6677-8899-aabbccddeeff", "invalid UUID format", false}},
		{"00112233-4455-6677-8899-aabbccddee0g", uuidParseOutput{false, "00112233-4455-6677-8899-aabbccddee00", "invalid UUID format", false}},
		{"[00112233-4455-6677-8899-aabbccddeeff]", uuidParseOutput{true, "00112233-4455-6677-8899-aabbccddeeff", "", false}},
		{"", uuidParseOutput{false, "00000000-0000-0000-0000-000000000000", "invalid UUID length: 0", true}},
		{"00000000-0000-0000-0000-000000000000", uuidParseOutput{true, "00000000-0000-0000-0000-000000000000", "", false}},
	} {
		if got := uuidParseReference(c.input); got != c.want {
			t.Fatalf("original oracle anchor changed for %q: %+v", c.input, got)
		}
	}
}
func TestUUIDParseBoundaryTransportPreservesPartialValuesAndDecoderUnits(t *testing.T) {
	rows := uuidParseBoundaryCases()
	raw, err := uuidParseInputDocument(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 24 || len(raw) > 32768 || strings.Contains(string(raw), `\ud800`) || strings.Contains(string(raw), `\udfff`) {
		t.Fatalf("boundary roster or JSON spelling changed: %d/%d", len(rows), len(raw))
	}
	decoded, err := uuidParseRows(raw)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	byName := map[string]uuidParseCase{}
	for i, row := range rows {
		if names[row.Name] || row.InputUTF16 == nil || len(row.Input) > 1024 {
			t.Fatal("invalid fixture identity/domain", row.Name)
		}
		names[row.Name] = true
		byName[row.Name] = row
		if decoded[i].Input != row.Input || decoded[i].Expected != row.Expected {
			t.Fatal("native decoded input differs from fixture", row.Name)
		}
	}
	if c := byName["raw-invalid-high-first"]; c.Expected.CanonicalUUID != "f1000000-0000-0000-0000-000000000000" || c.Expected.Parsed {
		t.Fatal("raw failing high nibble lost", c)
	}
	if c := byName["raw-invalid-low-first"]; c.Expected.CanonicalUUID != "ff000000-0000-0000-0000-000000000000" || c.Expected.Parsed {
		t.Fatal("raw failing low nibble lost", c)
	}
	if c := byName["hyphen-priority-before-decode"]; c.Expected.CanonicalUUID != "00000000-0000-0000-0000-000000000000" || c.Expected.Parsed {
		t.Fatal("hyphens no longer precede decoding", c)
	}
	if c := byName["urn-byte9-rune-cut"]; c.Expected.Error != `invalid urn prefix: "12345678\xea"` {
		t.Fatal("byte9 cut not quoted as upstream", c.Expected.Error)
	}
	if c := byName["lone-high"]; c.Input != "�x" || !reflect.DeepEqual(c.InputUTF16, []uint16{0xd800, 'x'}) {
		t.Fatal("lone surrogate capture lost", c)
	}
	if c := byName["text-1024"]; c.Expected.Error != "invalid UUID length: 1024" || !c.Expected.InvalidLength {
		t.Fatal("accepted browser domain confused with successful parse", c)
	}
	original, err := os.ReadFile("../../" + uuidParseInputs)
	if err != nil {
		t.Fatal(err)
	}
	supplied, err := uuidParseRows(original)
	if err != nil || len(supplied) != 12 {
		t.Fatal("original twelve inputs missing", err)
	}
	inputs := map[string]bool{}
	for _, row := range append(supplied, rows...) {
		if inputs[row.Input] {
			t.Fatal("duplicate decoded fixture input", row.Name)
		}
		inputs[row.Input] = true
	}
	if len(inputs) != 36 {
		t.Fatal("distinct input coverage changed", len(inputs))
	}
}
func TestUUIDParseRowsRejectMalformedAndDomainInputs(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":null}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":1}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":"x","other":"x"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":"x","go/github.com/google/uuid/text:ParseCanonical":"y"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":"` + strings.Repeat("a", 1025) + `"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":"` + strings.Repeat("가", 342) + `"}]}`,
	} {
		if _, err := uuidParseRows([]byte(raw)); err == nil {
			t.Fatal("accepted malformed UUID inputs", raw[:min(100, len(raw))])
		}
	}
}
func uuidParseABIForTest() string {
	record := "GoooRecord" + digest([]byte(uuidParseResult))
	source := "package main\ntype " + record + " struct {\n"
	for i, name := range uuidParseNames {
		kind := "string"
		if i == 0 || i == 3 {
			kind = "bool"
		}
		source += fmt.Sprintf("GoooField%s %s `json:%q`\n", digest([]byte(uuidParseResult+"/"+uuidParseFieldIDs[i])), kind, name)
	}
	return source + "}\nfunc GoooComposedActivity0(input string) " + record + " {return " + record + "{}}\n"
}
func TestUUIDParseGeneratedABIRejectsDrift(t *testing.T) {
	source := uuidParseABIForTest()
	if err := uuidParseCheckABI(source); err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]string{
		{"input string", "input int64"}, {"input string", "input0 string"}, {"input string", "input *string"}, {"input string", "input ...string"},
		{"input string", "input string, extra string"}, {"json:\"canonical_uuid\"", "json:\"uuid\""}, {" bool ", " string "}, {" string ", " []byte "},
		{"func GoooComposedActivity0(", "func AlteredEntry("}, {"GoooField" + digest([]byte(uuidParseResult+"/parsed")), "AlteredField"},
	} {
		changed := strings.Replace(source, p[0], p[1], 1)
		if changed == source {
			t.Fatal("ineffective mutation", p)
		}
		if err := uuidParseCheckABI(changed); err == nil {
			t.Fatal("accepted UUID ABI drift", p)
		}
	}
	if err := uuidParseCheckABI(source + strings.TrimPrefix(source, "package main\n")); err == nil {
		t.Fatal("accepted duplicate entry/record")
	}
}
func TestUUIDParseOriginalSourceBindings(t *testing.T) {
	docs := map[string][]byte{}
	for _, name := range []string{"upstream.json", "third_party/google-uuid/uuid.go", "third_party/google-uuid/util.go", "third_party/google-uuid/LICENSE"} {
		raw, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		docs[name] = raw
	}
	if err := uuidParseCheckOriginalSources(docs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"third_party/google-uuid/uuid.go", "third_party/google-uuid/util.go", "third_party/google-uuid/LICENSE", "upstream.json"} {
		original := docs[name]
		docs[name] = append(append([]byte{}, original...), byte('x'))
		if err := uuidParseCheckOriginalSources(docs); err == nil {
			t.Fatal("accepted original source/pin tampering", name)
		}
		docs[name] = original
	}
}
