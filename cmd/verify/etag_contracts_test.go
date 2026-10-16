package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func etagTestSuites() []suite {
	return []suite{{Package: etagPortPackage, Activity: etagScanActivity}, {Package: etagPortPackage, Activity: etagStrongActivity}, {Package: etagReportPackage, Activity: etagReportActivity}}
}
func etagTestIdentity(t *testing.T, s suite) expectedIdentity {
	t.Helper()
	id := expectedIdentity{CompilerRevision: uuidCompactCompilerRevision, GoVersion: "go1.27.2", Sources: map[string]sourceIdentity{}, SourceBytes: map[string][]byte{}}
	for _, spec := range etagSpecs(s) {
		raw, err := os.ReadFile(filepath.Join("../..", spec.source))
		if err != nil {
			t.Fatal(err)
		}
		image, err := json.Marshal(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		id.SourceBytes[spec.source] = raw
		id.Sources[spec.source] = sourceIdentity{spec.path, contentDigest(image)}
	}
	return id
}
func TestETagThreeEntryContractsAndFrozenSourceClosure(t *testing.T) {
	for i, s := range etagTestSuites() {
		defs, err := etagDefinitions(s, etagTestIdentity(t, s))
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if i == 2 {
			want = 5
		}
		if len(defs) != want {
			t.Fatalf("declared%d want%d", len(defs), want)
		}
		c := etagResultContract(s)
		fields := []int{2, 0, 13}[i]
		if len(c.Fields) != fields {
			t.Fatalf("resultfields%d", len(c.Fields))
		}
		inputs := []int{1, 2, 1}[i]
		if len(defs[s.Package+":"+s.Activity].Declaration.Inputs) != inputs {
			t.Fatal("input ABI")
		}
		registered, e := contractFor(s)
		if e != nil || registered.ID != defs[s.Package+":"+s.Activity].Declaration.ID || len(registered.InputEntities) != inputs {
			t.Fatal("activity routing missing", e)
		}
		if !reflect.DeepEqual(whitespaceResultContract(s), c) {
			t.Fatal("generic typed result routing missing")
		}
	}
	if got := etagResultContract(suite{Package: etagPortPackage, Activity: "Unknown"}); got.Entity != "" {
		t.Fatal("unknown entry admitted")
	}
	for _, s := range etagTestSuites() {
		id := etagTestIdentity(t, s)
		id.SourceBytes[etagPortSource] = append(id.SourceBytes[etagPortSource], []byte("\n// different current source\n")...)
		encoded, _ := json.Marshal(string(id.SourceBytes[etagPortSource]))
		id.Sources[etagPortSource] = sourceIdentity{etagPortPackage, contentDigest(encoded)}
		if _, err := etagDefinitions(s, id); err == nil {
			t.Fatal("changed rawsource accepted despite changed image")
		}
	}
}

func etagTestLowered(t *testing.T, s suite, includeUnusedRecord bool) (string, map[string]etagDefinition) {
	t.Helper()
	defs, err := etagDefinitions(s, etagTestIdentity(t, s))
	if err != nil {
		t.Fatal(err)
	}
	keys := etagReachable(s)
	bound := map[string]etagDefinition{}
	for i, key := range keys {
		d := defs[key]
		d.Lowered = fmt.Sprintf("GoooPackage%dActivity%s", 901-i*47, d.Declaration.Name)
		bound[key] = d
	}
	var b strings.Builder
	b.WriteString("package gooo_workspace\nnamespace gooo_workspace\nentity Text id \"gooo://std/text\"\nentity Boolean id \"gooo://std/boolean\"\n")
	if includeUnusedRecord || s.Activity != etagStrongActivity {
		for _, spec := range etagSpecs(s) {
			raw := etagTestIdentity(t, s).SourceBytes[spec.source]
			for _, m := range headerAdditionRecordPattern.FindAll(raw, -1) {
				b.Write(m)
				b.WriteByte('\n')
			}
		}
	}
	// Reverse declaration order and unrelated ordinals deliberately: IDs decide roles.
	for i := len(keys) - 1; i >= 0; i-- {
		d := bound[keys[i]]
		fmt.Fprintf(&b, "activity %s(%s) -> %s id %q computes %s\n", d.Lowered, strings.Join(d.Declaration.Inputs, ", "), d.Declaration.Output, d.Declaration.ID, strconv.Quote(etagLowerBody(d, bound, s)))
	}
	return b.String(), bound
}
func TestETagLoweredRolesDoNotAssumeNumericOrDeclarationOrder(t *testing.T) {
	for _, s := range etagTestSuites() {
		text, want := etagTestLowered(t, s, true)
		got, records, err := etagBindLowered(text, s, mustETagDefinitions(t, s))
		if err != nil {
			t.Fatal(err)
		}
		for key, d := range want {
			if got[key].Lowered != d.Lowered {
				t.Fatal("lowered name guessed")
			}
		}
		if etagReportConsumer(s) && len(records) != 2 {
			t.Fatal("two required records")
		}
	}
	s := etagTestSuites()[1]
	text, _ := etagTestLowered(t, s, false)
	_, records, err := etagBindLowered(text, s, mustETagDefinitions(t, s))
	if err != nil || len(records) != 0 {
		t.Fatal("unused source record metadata was assumed", err)
	}
}
func mustETagDefinitions(t *testing.T, s suite) map[string]etagDefinition {
	t.Helper()
	d, e := etagDefinitions(s, etagTestIdentity(t, s))
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestETagLoweredAndRecordDriftRefused(t *testing.T) {
	s := etagTestSuites()[2]
	text, _ := etagTestLowered(t, s, true)
	for _, change := range [][2]string{{etagReportActivityID, etagReportActivityID + "/wrong"}, {"let parsed =", "let modified ="}, {`/tag" type string`, `/tag" type boolean`}, {`/raw" type string`, `/other" type string`}} {
		bad := strings.Replace(text, change[0], change[1], 1)
		if bad == text {
			t.Fatal("mutation target missing", change[0])
		}
		if _, _, err := etagBindLowered(bad, s, mustETagDefinitions(t, s)); err == nil {
			t.Fatal("changed lowered contract accepted", change)
		}
	}
}

func etagTestImage(t *testing.T, s suite, defs map[string]etagDefinition) map[string]any {
	t.Helper()
	packages := []any{}
	order := []any{}
	for _, spec := range etagSpecs(s) {
		order = append(order, spec.path)
		exports := []any{}
		for _, a := range spec.activities {
			inputs := []string{}
			for _, k := range a.Inputs {
				inputs = append(inputs, spec.entities[k])
			}
			exports = append(exports, map[string]any{"name": a.Name, "kind": "activity", "id": a.ID, "input_types": inputs, "output_type": spec.entities[a.Output]})
		}
		for name, id := range spec.entities {
			exports = append(exports, map[string]any{"name": name, "kind": "entity", "id": id})
		}
		var imports any
		if spec.path == etagReportPackage {
			imports = []any{headerPortPackage, etagPortPackage}
		}
		packages = append(packages, map[string]any{"path": spec.path, "name": spec.name, "namespace": spec.namespace, "imports": imports, "exports": exports})
	}
	root := defs[s.Package+":"+s.Activity]
	fixture := map[string]any{"schema": "gooo/package-runtime-image/v1", "init_order": order, "packages": packages, "entry": map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return normalized.(map[string]any)
}
func TestETagImageHasAllExportsButOnlyDependencyInitConstraint(t *testing.T) {
	s := etagTestSuites()[2]
	defs := mustETagDefinitions(t, s)
	image := etagTestImage(t, s, defs)
	if err := etagInspectImage(image, s, defs); err != nil {
		t.Fatal(err)
	}
	image["init_order"] = []any{headerPortPackage, etagPortPackage, etagReportPackage}
	image["packages"] = []any{image["packages"].([]any)[2], image["packages"].([]any)[1], image["packages"].([]any)[0]}
	if err := etagInspectImage(image, s, defs); err != nil {
		t.Fatal("independent leaves/order assumed", err)
	}
	image["init_order"] = []any{etagReportPackage, headerPortPackage, etagPortPackage}
	if err := etagInspectImage(image, s, defs); err == nil {
		t.Fatal("dependency afterReport accepted")
	}
	image = etagTestImage(t, s, defs)
	header := image["packages"].([]any)[1].(map[string]any)
	exports := header["exports"].([]any)
	header["exports"] = exports[1:]
	if err := etagInspectImage(image, s, defs); err == nil {
		t.Fatal("unused declaredheader export omitted")
	}
}

func etagTestNative(t *testing.T, s suite, defs map[string]etagDefinition, records []any, composed bool) (string, map[string]string) {
	t.Helper()
	kinds := map[string]string{"Text": "string", "Boolean": "bool"}
	var b strings.Builder
	b.WriteString("package main\n")
	for _, v := range records {
		r := v.(map[string]any)
		kind := r["go_name"].(string)
		kinds[r["name"].(string)] = kind
		fmt.Fprintf(&b, "type %s struct {\n", kind)
		for _, v := range r["fields"].([]any) {
			f := v.(map[string]any)
			k := map[string]string{"urn:gooo:type:string": "string", "urn:gooo:type:boolean": "bool", "urn:gooo:type:integer": "int64"}[f["type_id"].(string)]
			fmt.Fprintf(&b, "%s %s `json:%q`\n", f["go_name"], k, f["name"])
		}
		b.WriteString("}\n")
	}
	names := map[string]string{}
	root := s.Package + ":" + s.Activity
	for i, key := range etagReachable(s) {
		names[key] = defs[key].Lowered
		if composed {
			names[key] = fmt.Sprintf("IndependentLeaf%d", i)
			if key == root {
				names[key] = "SelectedRoot"
			}
		}
	}
	for _, key := range etagReachable(s) {
		d := defs[key]
		params := []string{}
		for i, k := range d.Declaration.Inputs {
			name := "input"
			if len(d.Declaration.Inputs) > 1 {
				name = fmt.Sprintf("input%d", i)
			}
			params = append(params, name+" "+kinds[k])
		}
		fmt.Fprintf(&b, "func %s(%s) %s {\n", names[key], strings.Join(params, ","), kinds[d.Declaration.Output])
		if key == root {
			for _, callee := range etagCallees(s) {
				args := "input"
				if callee == etagPortPackage+":"+etagStrongActivity {
					args = "input,input"
				}
				fmt.Fprintf(&b, "_ = %s(%s)\n", names[callee], args)
			}
		}
		zero := "false"
		if d.Declaration.Output != "Boolean" {
			zero = kinds[d.Declaration.Output] + "{}"
		}
		fmt.Fprintf(&b, "return %s\n}\n", zero)
	}
	return b.String(), names
}
func TestETagNativeTypedRolesFieldsAndCallOrder(t *testing.T) {
	for _, s := range etagTestSuites() {
		text, _ := etagTestLowered(t, s, true)
		defs, records, err := etagBindLowered(text, s, mustETagDefinitions(t, s))
		if err != nil {
			t.Fatal(err)
		}
		for _, composed := range []bool{false, true} {
			source, _ := etagTestNative(t, s, defs, records, composed)
			if err := inspectETagNativeABI(source, defs, s, records, composed, "SelectedRoot"); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := etagTestSuites()[2]
	text, _ := etagTestLowered(t, s, true)
	defs, records, _ := etagBindLowered(text, s, mustETagDefinitions(t, s))
	source, names := etagTestNative(t, s, defs, records, true)
	a, b := names[etagPortPackage+":"+etagScanActivity], names[headerPortPackage+":"+headerValueActivity]
	for _, change := range [][2]string{{"input1 string", "input1 bool"}, {"_ = " + a + "(input)\n_ = " + b + "(input)", "_ = " + b + "(input)\n_ = " + a + "(input)"}, {"return false", "return unexpected(input)"}, {"json:\"raw\"", "json:\"other\""}} {
		bad := strings.Replace(source, change[0], change[1], 1)
		if bad == source {
			t.Fatal("native mutation missing", change[0])
		}
		if err := inspectETagNativeABI(bad, defs, s, records, true, "SelectedRoot"); err == nil {
			t.Fatal("native ABI mutation accepted", change)
		}
	}
}
func TestETagRawGoMatrixUsesSourceRecordOrder(t *testing.T) {
	s := suite{Package: etagPortPackage, Activity: etagScanActivity, Cases: []testCase{{Expected: map[string]any{"remain": ",x", "etag": "\"a\""}}}}
	raw, err := etagExpectedStdout(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[[{\"etag\":\"\\\"a\\\"\",\"remain\":\",x\"}]]\n" {
		t.Fatalf("recordorder %s", raw)
	}
	s = suite{Package: etagPortPackage, Activity: etagStrongActivity, Cases: []testCase{{Expected: false}}}
	raw, err = etagExpectedStdout(s)
	if err != nil || string(raw) != "[[false]]\n" {
		t.Fatal("Boolean matrix", string(raw), err)
	}
	if !reflect.DeepEqual(etagFactsNames, []string{"raw", "tag", "remainder", "header_valid", "present", "weak", "complete", "strong", "single_strong", "input_bytes", "tag_bytes", "remainder_bytes", "reason"}) {
		t.Fatal("facts order drift")
	}
}

// The observed codegen profile has only record types and functions. Imports,
// variables (including initializers) and constants must not bypass call checks.
func TestETagNativeRejectsOtherTopLevelDeclarations(t *testing.T) {
	s := etagTestSuites()[2]
	text, _ := etagTestLowered(t, s, true)
	defs, records, err := etagBindLowered(text, s, mustETagDefinitions(t, s))
	if err != nil {
		t.Fatal(err)
	}
	for _, composed := range []bool{false, true} {
		source, _ := etagTestNative(t, s, defs, records, composed)
		for _, injected := range []string{`import "unsafe"`, `var injected = unexpected()`, `const injected = 1`} {
			bad := strings.Replace(source, "package main\n", "package main\n"+injected+"\n", 1)
			if bad == source {
				t.Fatal("top-level mutation did not apply")
			}
			if err := inspectETagNativeABI(bad, defs, s, records, composed, "SelectedRoot"); err == nil {
				t.Fatalf("unexpected top-level declaration accepted: %s", injected)
			}
		}
	}
}
