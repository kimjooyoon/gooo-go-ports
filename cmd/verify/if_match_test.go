package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestIfMatchOriginalGoEnumAndThreeLookupSubstitutions(t *testing.T) {
	original, err := os.ReadFile("../../third_party/go/net/http/fs.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile("if_match_reference.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(original), "// condResult is the result")
	if start < 0 {
		t.Fatal("missing original enum")
	}
	end := strings.Index(string(original)[start:], "\nfunc checkIfUnmodifiedSince(")
	if end < 0 {
		t.Fatal("original Go excerpt bounds")
	}
	want := string(original)[start : start+end]
	rs := strings.Index(string(reference), "// condResult is the result")
	if rs < 0 {
		t.Fatal("missing reference enum")
	}
	re := strings.Index(string(reference)[rs:], "\n// Only the independent Go verifier")
	if re < 0 {
		t.Fatal("adapted original excerpt bounds")
	}
	actual := strings.TrimSpace(string(reference)[rs : rs+re])
	substitutions := [][2]string{{"func checkIfMatch(selectedIfMatchHeader, currentETag string) condResult {", "func checkIfMatch(w ResponseWriter, r *Request) condResult {"}, {"im := selectedIfMatchHeader", `im := r.Header.Get("If-Match")`}, {"etagStrongMatch(etag, currentETag)", `etagStrongMatch(etag, w.Header().get("Etag"))`}}
	for _, pair := range substitutions {
		if strings.Count(actual, pair[0]) != 1 {
			t.Fatal("exact adapter substitution", pair[0])
		}
		actual = strings.Replace(actual, pair[0], pair[1], 1)
	}
	if actual != strings.TrimSpace(want) {
		t.Fatal("original enum/loop changed beyond three explicit substitutions")
	}
	for _, row := range []struct {
		selected, current string
		want              int64
	}{
		{"", `"x"`, 0}, {" \t\r\n", `"x"`, 2}, {",,,", `"x"`, 2}, {"*suffix", "", 1},
		{`"z""a"`, `"a"`, 1}, {`"a" invalid`, `"a"`, 1}, {`"a" invalid`, `"b"`, 2},
		{`W/"a"`, `"a"`, 2}, {`"a"`, ` "a"`, 2}, {"\r\n\"a\"\t", `"a"`, 1},
		{"\x00", "\x00", 2}, {`"a\b"`, `"a\b"`, 1}, {`"café 🧪"`, `"café 🧪"`, 2},
	} {
		if got := ifMatchReference(row.selected, row.current)["condition"]; got != row.want {
			t.Fatalf("literal original Go anchor %q/%q got%v want%d", row.selected, row.current, got, row.want)
		}
	}
}

func TestIfMatchAll284InputsRemainOrderedAndDomainMalformedRefuses(t *testing.T) {
	raw := etagFixtureBytes(t, ifMatchCorpusFile)
	rows, err := ifMatchInputs(raw)
	if err != nil || len(rows) != 284 {
		t.Fatal(err)
	}
	suites, err := ifMatchCorpus("../..")
	if err != nil || len(suites) != 1 || len(suites[0].Cases) != 284 {
		t.Fatal(err)
	}
	for i, row := range rows {
		c := suites[0].Cases[i]
		if !reflect.DeepEqual(c.Inputs, []any{row.Inputs[0], row.Inputs[1]}) || !reflect.DeepEqual(c.Expected, ifMatchReference(row.Inputs[0], row.Inputs[1])) {
			t.Fatal("original two Texts/fresh Go value changed", i)
		}
	}
	for _, mutation := range []func(map[string]any){
		func(v map[string]any) { v["rows"] = v["rows"].([]any)[:283] },
		func(v map[string]any) { v["input_order"] = []any{"currentETag", "selectedIfMatchHeader"} },
		func(v map[string]any) {
			v["rows"].([]any)[1].(map[string]any)["id"] = v["rows"].([]any)[0].(map[string]any)["id"]
		},
		func(v map[string]any) { v["rows"].([]any)[0].(map[string]any)["inputs"] = []any{""} },
		func(v map[string]any) { v["rows"].([]any)[0].(map[string]any)["inputs"] = []any{true, ""} },
		func(v map[string]any) {
			v["rows"].([]any)[0].(map[string]any)["inputs"] = []any{"", strings.Repeat("a", 1025)}
		},
		func(v map[string]any) {
			v["rows"].([]any)[0].(map[string]any)["inputs"] = []any{strings.Repeat("a", 1025), ""}
		},
		func(v map[string]any) {
			v["rows"].([]any)[0].(map[string]any)["expected"] = map[string]any{"condition": 1}
		},
	} {
		value, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		mutation(value.(map[string]any))
		bad, _ := json.Marshal(value)
		if _, err := ifMatchInputs(bad); err == nil {
			t.Fatal("malformed input corpus accepted")
		}
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(`upstream-ifmatch_matches`), []byte{0xff}, 1), bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"wrong","schema":`), 1), bytes.Replace(raw, []byte(`upstream-ifmatch_matches`), []byte(`\ud800`), 1)} {
		if bytes.Equal(bad, raw) {
			continue
		}
		if _, err := ifMatchInputs(bad); err == nil {
			t.Fatal("invalid raw UTF8/surrogate/duplicate accepted")
		}
	}
}

func TestIfMatchSharedABIRequiresTwoTextsAndOneOrderedInt64Field(t *testing.T) {
	s := suite{Package: ifMatchPackage, Activity: ifMatchActivity}
	defs := mustETagDefinitions(t, s)
	if len(defs) != 1 || !reflect.DeepEqual(defs[ifMatchPackage+":"+ifMatchActivity].Declaration.Inputs, []string{"Text", "Text"}) {
		t.Fatal("one exact two Text activity")
	}
	registered, err := contractFor(s)
	if err != nil || registered.ID != ifMatchActivityID || !reflect.DeepEqual(registered.InputEntities, []string{textEntity, textEntity}) {
		t.Fatal("registered original input ABI", err)
	}
	c := whitespaceResultContract(s)
	if c.Entity != ifMatchEntity || !reflect.DeepEqual(c.Fields, []recordFieldContract{{"condition", ifMatchEntity + "/condition"}}) {
		t.Fatal("integer record result contract")
	}
	lowered, _ := etagTestLowered(t, s, true)
	bound, records, err := etagBindLowered(lowered, s, defs)
	if err != nil || len(records) != 1 {
		t.Fatal(err)
	}
	if err := etagInspectImage(etagTestImage(t, s, defs), s, defs); err != nil {
		t.Fatal(err)
	}
	for _, composed := range []bool{false, true} {
		source, _ := etagTestNative(t, s, bound, records, composed)
		if err := inspectETagNativeABI(source, bound, s, records, composed, "SelectedRoot"); err != nil {
			t.Fatal(err)
		}
		for _, pair := range [][2]string{{"input1 string", "input1 bool"}, {" int64 `json:", " bool `json:"}, {`json:"condition"`, `json:"other"`}} {
			bad := strings.Replace(source, pair[0], pair[1], 1)
			if bad == source {
				t.Fatal("missing ABI mutation", pair[0])
			}
			if err := inspectETagNativeABI(bad, bound, s, records, composed, "SelectedRoot"); err == nil {
				t.Fatal("changed ABI accepted", pair)
			}
		}
	}
	for _, pair := range [][2]string{{`/condition" type integer`, `/condition" type boolean`}, {"let state = 0", "let state = 1"}, {"(Text, Text)", "(Text)"}} {
		bad := strings.Replace(lowered, pair[0], pair[1], 1)
		if bad == lowered {
			t.Fatal("missing source mutation", pair[0])
		}
		if _, _, err := etagBindLowered(bad, s, defs); err == nil {
			t.Fatal("changed source/record/arity accepted")
		}
	}
	s.Cases = []testCase{{Inputs: []any{"", ""}, Expected: map[string]any{"condition": int64(0)}}}
	stdout, err := etagExpectedStdout(s)
	if err != nil || string(stdout) != "[[{\"condition\":0}]]\n" {
		t.Fatal("integer raw matrix", string(stdout), err)
	}
}

func TestIfMatchFiveLosslessPartitionsAndOriginal79WorkspacePrefix(t *testing.T) {
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	added, err := ifMatchCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := added[0]
	w, err := f.workspaceForSuite(s)
	if err != nil || !reflect.DeepEqual(w.Names, []string{ifMatchSource}) {
		t.Fatal("one current source closure", err)
	}
	p, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	ends := []int{}
	covered := []testCase{}
	for start := 0; start < len(s.Cases); {
		prepared, err := p.prepare(s, start)
		if err != nil {
			t.Fatal(err)
		}
		batch := s
		batch.Cases = s.Cases[start:prepared.End]
		assertETagTwoInputKeys(t, batch, prepared.Raw)
		covered = append(covered, batch.Cases...)
		ends = append(ends, prepared.End)
		start = prepared.End
	}
	if !reflect.DeepEqual(ends, []int{101, 201, 266, 270, 284}) || !reflect.DeepEqual(covered, s.Cases) {
		t.Fatal("original284/5 ordered partition changed", ends)
	}
	for port := 0; port < 2; port++ {
		for _, bad := range []any{nil, true, 1.5, []any{}, map[string]any{}, string([]byte{0xff}), strings.Repeat("a", 1025)} {
			c := s
			c.Cases = []testCase{{Inputs: []any{"", ""}, Expected: map[string]any{"condition": int64(0)}}}
			c.Cases[0].Inputs[port] = bad
			if _, err := etagBatchPolicy(c, w, caseProfileTestLock()); err == nil {
				t.Fatal("unsupported input accepted", port, bad)
			}
		}
	}
	old := etagOriginal76(t)
	etags, err := etagCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	old = append(old, etags...)
	prior := f
	prior.Catalog.Packages = nil
	for _, pkg := range f.Catalog.Packages {
		if pkg.Path != ifMatchPackage {
			prior.Catalog.Packages = append(prior.Catalog.Packages, pkg)
		}
	}
	cases := 0
	for _, s := range old {
		cases += len(s.Cases)
		beforeLegacy, err := prior.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		afterLegacy, err := f.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range []nativeWorkspaceProfile{"", dependencyClosureWorkspaceProfile} {
			before, err := profile.workspaceForSelected(prior, s, beforeLegacy)
			if err != nil {
				t.Fatal(err)
			}
			after, err := profile.workspaceForSelected(f, s, afterLegacy)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("IfMatch changed original79 selected workspace", s.Package, s.Activity, err)
			}
			if _, ok := after.Sources[ifMatchSource]; ok {
				t.Fatal("new source leaked into old79")
			}
			for _, cp := range []nativeCaseProfile{"", uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
				a, err := cp.policy(s, before, caseProfileTestLock())
				if err != nil {
					t.Fatal(err)
				}
				b, err := cp.policy(s, after, caseProfileTestLock())
				if err != nil || a != b {
					t.Fatal("original79 case policy changed", err)
				}
			}
		}
	}
	if len(old) != 79 || cases != 173582 {
		t.Fatal("original79 prefix population changed", len(old), cases)
	}
}
