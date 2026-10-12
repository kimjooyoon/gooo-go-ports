package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLiteralSearchByteAndNonOverlapAnchors(t *testing.T) {
	for _, c := range []struct{ text, needle, offset, count, bytes string }{
		{"가x가", "x", "3", "1", "7"}, {"aaaaa", "aa", "0", "2", "5"},
		{"ée\u0301", "\u0301", "3", "1", "5"}, {"e\u0301", "é", "-1", "0", "3"},
		{"가x가", "", "0", "4", "7"}, {"", "", "0", "1", "0"},
		{"🙂가🙂", "가", "4", "1", "11"}, {"a\x00\r\na", "\x00\r\n", "1", "1", "5"},
	} {
		r := literalReference(c.text, c.needle)
		if r.FirstByteOffset != c.offset || r.NonOverlappingCount != c.count || r.TextByteLength != c.bytes || r.Found != (c.offset != "-1") {
			t.Fatalf("wrong independent fixture: %+v", r)
		}
	}
}
func TestLiteralSearchBoundaryTransportRoundTrip(t *testing.T) {
	rows := literalBoundaryCases()
	raw, err := literalInputDocument(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 24 || len(raw) > 32768 || !strings.Contains(string(raw), `\ud800`) {
		t.Fatalf("lost native input boundary: %d rows %d bytes", len(rows), len(raw))
	}
	decoded, err := literalRows(raw)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, row := range rows {
		if seen[row.Name] || row.TextUTF16 == nil || row.NeedleUTF16 == nil {
			t.Fatal("fixture identity/UTF-16 units missing")
		}
		seen[row.Name] = true
		if !reflect.DeepEqual(row.Inputs, decoded[i].Inputs) || row.Expected != decoded[i].Expected {
			t.Fatal("native decoder and independent reference differ", row.Name)
		}
		if len(row.Inputs[0]) > 1024 || len(row.Inputs[1]) > 1024 {
			t.Fatal("successful fixture is outside domain")
		}
	}
	if rows[20].Inputs[0] != "�x" || !reflect.DeepEqual(rows[20].TextUTF16, []uint16{0xd800, 'x'}) {
		t.Fatal("original lone surrogate or decoded replacement lost")
	}
	original, err := os.ReadFile("../../" + literalInputs)
	if err != nil {
		t.Fatal(err)
	}
	supplied, err := literalRows(original)
	if err != nil || len(supplied) != 24 {
		t.Fatal("original24 missing", err)
	}
}
func TestLiteralRowsRejectAmbiguousAndOutOfDomainInputs(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":"x"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":null,"app/literal-text-report:Report.input1":"x"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":"x","app/literal-text-report:Report.input1":1}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":"x","app/literal-text-report:Report.input0":"y","app/literal-text-report:Report.input1":"x"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":"` + strings.Repeat("a", 1025) + `","app/literal-text-report:Report.input1":""}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/literal-text-report:Report.input0":"","app/literal-text-report:Report.input1":"` + strings.Repeat("가", 342) + `"}]}`,
	} {
		if _, err := literalRows([]byte(raw)); err == nil {
			t.Fatal("accepted malformed native input", raw[:min(100, len(raw))])
		}
	}
}
func literalABIForTest() string {
	record := "GoooRecord" + digest([]byte(literalResult))
	source := "package main\ntype " + record + " struct {\n"
	for i, name := range literalNames {
		kind := "int64"
		if i < 2 {
			kind = "string"
		}
		if i == 3 {
			kind = "bool"
		}
		source += fmt.Sprintf("GoooField%s %s `json:%q`\n", digest([]byte(literalResult+"/"+literalFieldIDs[i])), kind, name)
	}
	return source + "}\nfunc GoooComposedActivity0(input0 string,input1 string) " + record + " {return " + record + "{}}\n"
}
func TestLiteralSearchGeneratedABIRejectsDrift(t *testing.T) {
	source := literalABIForTest()
	if err := literalCheckABI(source); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"input1 string", "input1 int64"}, {"input0 string,input1 string", "input0 string"},
		{"input0 string", "input0 *string"}, {"input1 string", "input1 ...string"},
		{"json:\"first_byte_offset\"", "json:\"character_offset\""},
		{" int64 ", " float64 "}, {" bool ", " string "},
		{"func GoooComposedActivity0(", "func AlteredEntry("},
		{"GoooField" + digest([]byte(literalResult+"/text")), "AlteredField"},
	} {
		changed := strings.Replace(source, pair[0], pair[1], 1)
		if changed == source {
			t.Fatal("ineffective mutation", pair)
		}
		if err := literalCheckABI(changed); err == nil {
			t.Fatal("accepted ABI drift", pair)
		}
	}
}
