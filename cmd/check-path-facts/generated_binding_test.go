package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Synthetic metadata fixtures are not Gooo execution or original-Go oracle
// observations. They exercise the same production ABI and typed-row gates.
func pathBindingFixture(t *testing.T, entry string) ([]byte, []byte, string, identity) {
	t.Helper()
	if err := configure(entry); err != nil {
		t.Fatal(err)
	}
	id := identity{Manifest: digest([]byte("fixture-manifest")), Inputs: digest([]byte("fixture-inputs")), GoTool: digest([]byte("fixture-go-tool")), Bodies: map[string]string{"go/path:Ext": digest([]byte("ext-body")), "go/path:IsAbs": digest([]byte("abs-body")), reportPackage + ":Report": digest([]byte("report-body"))}}
	abi := pathGeneratedABI{Function: "GoooComposedActivity0"}
	output, outputType := "string", "Text"
	entity := textEntity
	lowered := "GoooPackage0ActivityExt"
	source := "package main\nfunc " + abi.Function + "(input string) string { return input }\n"
	if entry == "isabs" {
		output, outputType, entity, lowered = "bool", "Boolean", booleanEntity, "GoooPackage1ActivityIsAbs"
		source = "package main\nfunc " + abi.Function + "(input string) bool { return len(input)>0 }\n"
	}
	var record map[string]any
	if entry == "report" {
		lowered = "GoooPackage2ActivityReport"
		outputType, entity = "PathFacts", resultEntity
		abi.Record = pathRecordSymbol(resultEntity)
		output = abi.Record
		abi.Helpers = [2]string{abi.Function + "Call0", abi.Function + "Call1"}
		var fields []any
		var declaration, values strings.Builder
		for i, name := range pathFieldNames {
			abi.Fields[i] = pathFieldSymbol(resultEntity + "/" + pathFieldIDs[i])
			fields = append(fields, map[string]any{"name": name, "id": resultEntity + "/" + pathFieldIDs[i], "type_id": "urn:gooo:type:" + pathFieldEntities[i], "go_name": abi.Fields[i]})
			fmt.Fprintf(&declaration, "%s %s `json:%q`\n", abi.Fields[i], pathFieldTypes[i], name)
		}
		fieldValues := []string{"input", "extension", "absolute", "int64(len(input))", "int64(len(extension))", "len(extension)>0"}
		for i, value := range fieldValues {
			if i > 0 {
				values.WriteString(",")
			}
			fmt.Fprintf(&values, "%s:%s", abi.Fields[i], value)
		}
		source = "package main\ntype " + abi.Record + " struct {\n" + declaration.String() + "}\nfunc " + abi.Function + "(input string) " + output + " { var extension=" + abi.Helpers[0] + "(input);var absolute=" + abi.Helpers[1] + "(input);return " + abi.Record + "{" + values.String() + "} }\nfunc " + abi.Helpers[0] + "(input string) string { return input }\nfunc " + abi.Helpers[1] + "(input string) bool { return len(input)>0 }\n"
		record = map[string]any{"name": "PathFacts", "id": resultEntity, "go_name": abi.Record, "fields": fields}
	}
	plan := map[string]any{"schema": "gooo/body-composition-plan/v1", "typed_plan_sha256": digest([]byte("fixture-plan")), "semantic_fingerprint": strings.TrimPrefix(digest([]byte("fixture-semantic")), "sha256:"), "activities": []any{map[string]any{"name": lowered, "id": activityID, "input_type": "Text", "output_type": outputType, "input_entity_id": textEntity, "output_entity_id": entity, "go_function": abi.Function, "input_from": -1, "assembling": false}}, "edges": []any{}}
	genSource := strings.Replace(source, "package main", "package gooo_workspace", 1)
	if entry == "report" {
		plan["entry_activity"], plan["record_types"] = lowered, []any{record}
		genSource = strings.NewReplacer(abi.Helpers[0], "GoooPackage0ActivityExt", abi.Helpers[1], "GoooPackage1ActivityIsAbs", abi.Function, lowered).Replace(genSource)
	} else {
		genSource = strings.ReplaceAll(genSource, abi.Function, lowered)
	}
	gr := map[string]any{"generated_digest": digest([]byte(genSource)), "replay_digest": digest([]byte(genSource)), "activity": lowered, "activity_id": activityID, "compiler_source_sha": compilerRevision}
	p := map[string]any{}
	if entry == "report" {
		var activities []any
		for _, item := range []struct{ pkg, name, lowered, source string }{{reportPackage, "Report", lowered, "examples/path-facts/main.gooo"}, {"go/path", "Ext", "GoooPackage0ActivityExt", "ports/path/path.gooo"}, {"go/path", "IsAbs", "GoooPackage1ActivityIsAbs", "ports/path/path.gooo"}} {
			activities = append(activities, map[string]any{"activity": map[string]any{"package_path": item.pkg, "activity": item.name, "lowered_name": item.lowered}, "source": item.source, "source_body_sha256": id.Bodies[item.pkg+":"+item.name]})
		}
		p["pure_calls"] = map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": []any{map[string]any{"caller": reportPackage + ":Report", "callee": "go/path:Ext", "surface": "computes"}, map[string]any{"caller": reportPackage + ":Report", "callee": "go/path:IsAbs", "surface": "computes"}}}
		gr["call_closure"] = map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": []any{map[string]any{"name": "GoooPackage0ActivityExt", "activity_id": "gooo://go/path/Ext", "program_sha256": id.Bodies["go/path:Ext"]}, map[string]any{"name": "GoooPackage1ActivityIsAbs", "activity_id": "gooo://go/path/IsAbs", "program_sha256": id.Bodies["go/path:IsAbs"]}}, "edges": []any{map[string]any{"caller_id": activityID, "callee_id": "gooo://go/path/Ext", "ordinal": 0}, map[string]any{"caller_id": activityID, "callee_id": "gooo://go/path/IsAbs", "ordinal": 1}}, "max_calls_per_invocation": 2}
	}
	c := map[string]any{"source": source, "generated_sha256": digest([]byte(source)), "stage": "COMPLETE", "plan": plan, "steps": []any{map[string]any{"generation": map[string]any{"source": genSource, "report": gr}}}}
	e := map[string]any{"result": map[string]any{"composition": c, "program": p}}
	execution, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	e["replayed_from_sha256"] = digest(execution)
	replay, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return execution, replay, digest([]byte(source)), id
}

func pathFixtureMutation(t *testing.T, execution, replay []byte, mutate func(map[string]any, map[string]any)) ([]byte, []byte) {
	t.Helper()
	e, err := decode(execution)
	if err != nil {
		t.Fatal(err)
	}
	r, err := decode(replay)
	if err != nil {
		t.Fatal(err)
	}
	mutate(object(e), object(r))
	execution, err = json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	object(r)["replayed_from_sha256"] = digest(execution)
	replay, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return execution, replay
}

func TestPathGeneratedBindingPositive(t *testing.T) {
	for _, entry := range []string{"ext", "isabs", "report"} {
		t.Run(entry, func(t *testing.T) {
			e, r, expected, id := pathBindingFixture(t, entry)
			source, abi, err := inspectPathGenerated(e, r, expected, id)
			if err != nil {
				t.Fatal(err)
			}
			if source == "" || abi.Function != "GoooComposedActivity0" {
				t.Fatal("positive ABI extraction differs")
			}
			if _, err := makePathHarness(abi, expected, e, r, id); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPathGeneratedBindingMutations(t *testing.T) {
	mutations := []struct {
		name   string
		change func(map[string]any, map[string]any)
	}{
		{"entry-id", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				object(array(object(c["plan"])["activities"])[0])["id"] = "gooo://wrong"
			}
		}},
		{"input-null", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				object(array(object(c["plan"])["activities"])[0])["input_from"] = nil
			}
		}},
		{"input-bool", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				object(array(object(c["plan"])["activities"])[0])["input_from"] = false
			}
		}},
		{"edges", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				object(c["plan"])["edges"] = []any{map[string]any{}}
			}
		}},
		{"field-id", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				record := object(array(object(c["plan"])["record_types"])[0])
				object(array(record["fields"])[0])["id"] = "wrong"
			}
		}},
		{"field-kind", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				record := object(array(object(c["plan"])["record_types"])[0])
				object(array(record["fields"])[2])["type_id"] = "urn:gooo:type:string"
			}
		}},
		{"field-order", func(e, r map[string]any) {
			for _, v := range []map[string]any{e, r} {
				c := object(object(v["result"])["composition"])
				record := object(array(object(c["plan"])["record_types"])[0])
				fields := array(record["fields"])
				fields[0], fields[1] = fields[1], fields[0]
			}
		}},
		{"purecalls-kind", func(e, r map[string]any) { object(r["result"])["program"] = map[string]any{"pure_calls": []any{}} }},
		{"helper-target", func(e, r map[string]any) {
			c := object(object(r["result"])["composition"])
			gr := object(object(object(array(c["steps"])[0])["generation"])["report"])
			object(array(object(gr["call_closure"])["edges"])[0])["callee_id"] = "gooo://go/path/IsAbs"
		}},
		{"helper-body", func(e, r map[string]any) {
			c := object(object(r["result"])["composition"])
			gr := object(object(object(array(c["steps"])[0])["generation"])["report"])
			object(array(object(gr["call_closure"])["activities"])[0])["program_sha256"] = digest([]byte("different-body"))
		}},
		{"source-digest", func(e, r map[string]any) {
			object(object(r["result"])["composition"])["generated_sha256"] = digest([]byte("old-source"))
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			e, r, expected, id := pathBindingFixture(t, "report")
			e, r = pathFixtureMutation(t, e, r, mutation.change)
			if _, _, err := inspectPathGenerated(e, r, expected, id); err == nil {
				t.Fatal("mutated binding accepted")
			}
		})
	}
	e, r, expected, id := pathBindingFixture(t, "ext")
	if _, _, err := inspectPathGenerated(e, r, digest([]byte("old-generated")), id); err == nil {
		t.Fatal("wrong checked digest accepted")
	}
	if _, _, err := inspectPathGenerated(e, e, expected, id); err == nil {
		t.Fatal("same raw receipt accepted")
	}
	if _, _, err := inspectPathGenerated(e, append(r, []byte("{}")...), expected, id); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	e, r, _, id = pathBindingFixture(t, "ext")
	e, r = pathFixtureMutation(t, e, r, func(e, r map[string]any) {
		for _, v := range []map[string]any{e, r} {
			c := object(object(v["result"])["composition"])
			changed := strings.Replace(str(c["source"]), "return input", `return ""`, 1)
			c["source"], c["generated_sha256"] = changed, digest([]byte(changed))
			expected = digest([]byte(changed))
		}
	})
	if _, _, err := inspectPathGenerated(e, r, expected, id); err == nil {
		t.Fatal("composition body changed independently from generation accepted")
	}
}

func TestPathGeneratedASTMutations(t *testing.T) {
	e, r, expected, id := pathBindingFixture(t, "report")
	source, abi, err := inspectPathGenerated(e, r, expected, id)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"import":                  strings.Replace(source, "package main", "package main\nimport \"path\"", 1),
		"global":                  source + "\nvar global=1\n",
		"init":                    source + "\nfunc init() {}\n",
		"method":                  source + "\nfunc (x " + abi.Record + ") Extra() {}\n",
		"directive":               "//go:build linux\n" + source,
		"legacy-build":            "// +build linux\n\n" + source,
		"legacy-build-no-space":   "//+build linux\n\n" + source,
		"legacy-build-two-spaces": "//  +build linux\n\n" + source,
		"legacy-build-tab":        "//\t+build linux\n\n" + source,
		"line-directive":          "//line wrong.go:1\n" + source,
		"oracle-call":             strings.Replace(source, "len(input)", "path.Ext(input)", 1),
		"unknown-call":            strings.Replace(source, "len(input)", "pathCorpusDigest(input)", 1),
		"recursive":               strings.Replace(source, abi.Helpers[0]+"(input)", abi.Function+"(input)", 1),
		"helper-swapped":          strings.Replace(source, "var extension="+abi.Helpers[0]+"(input)", "var extension="+abi.Helpers[1]+"(input)", 1),
		"tag-omitempty":           strings.Replace(source, `json:"text"`, `json:"text,omitempty"`, 1),
		"type-changed":            strings.Replace(source, abi.Fields[0]+" string", abi.Fields[0]+" bool", 1),
		"extra-field":             strings.Replace(source, "struct {", "struct { Extra string;", 1),
		"alias":                   strings.Replace(source, "type "+abi.Record+" struct", "type "+abi.Record+" = struct", 1),
		"generics":                strings.Replace(source, "func "+abi.Function+"(", "func "+abi.Function+"[T any](", 1),
		"main":                    strings.Replace(source, "func "+abi.Function+"(", "func main(", 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePathBindingSource(mutated, "main", abi); err == nil {
				t.Fatal("mutated generated AST accepted")
			} else if strings.HasPrefix(name, "legacy-build") && err.Error() != "generated compiler directive refused" {
				t.Fatalf("legacy build must reach directive refusal: %v", err)
			}
		})
	}
	_ = e
	_ = r
}

func pathRowsFixture(t *testing.T, entry string) []byte {
	t.Helper()
	if err := configure(entry); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	for i, text := range pathCorpusInputs() {
		want := expectedValue(pathOracle(text))
		raw, err := json.Marshal(struct {
			Index  int    `json:"index"`
			Input  string `json:"input"`
			Actual any    `json:"actual"`
			Oracle any    `json:"oracle"`
		}{i, text, want, want})
		if err != nil {
			t.Fatal(err)
		}
		out.Write(raw)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func TestPathGeneratedRowsPositive(t *testing.T) {
	inputs := pathCorpusInputs()
	if len(inputs) != 9841 || !reflect.DeepEqual(inputs[:4], []string{"", "a", ".", "/"}) || inputs[3279] != "///////" || inputs[3280] != "aaaaaaaa" || inputs[9840] != "////////" {
		t.Fatal("independent corpus anchors differ")
	}
	for _, entry := range []string{"ext", "isabs", "report"} {
		t.Run(entry, func(t *testing.T) {
			raw := pathRowsFixture(t, entry)
			e, err := inspectPathRows(raw)
			if err != nil {
				t.Fatal(err)
			}
			comparisons := 9841
			if entry == "report" {
				comparisons *= 6
			}
			if e.Rows != 9841 || e.ScalarComparisons != comparisons || e.SHA256 != digest(raw) || e.OutputSHA256 != e.OracleSHA256 {
				t.Fatal("positive typed-row counters/digests differ")
			}
		})
	}
}

func TestPathGeneratedRowsMutations(t *testing.T) {
	raw := pathRowsFixture(t, "report")
	lines := bytes.Split(raw, []byte{'\n'})
	mutations := map[string][]byte{
		"omission":          bytes.Join(append(append([][]byte{}, lines[:1]...), lines[2:]...), []byte{'\n'}),
		"duplicate":         bytes.Join(append(append([][]byte{}, lines[:2]...), lines[1:]...), []byte{'\n'}),
		"missing-lf":        raw[:len(raw)-1],
		"null":              bytes.Replace(raw, []byte(`"actual":{`), []byte(`"actual":null,"ignored":{`), 1),
		"length-float":      bytes.Replace(raw, []byte(`"text_byte_length":0`), []byte(`"text_byte_length":0.0`), 1),
		"length-bool":       bytes.Replace(raw, []byte(`"text_byte_length":0`), []byte(`"text_byte_length":false`), 1),
		"boolean-string":    bytes.Replace(raw, []byte(`"is_abs":false`), []byte(`"is_abs":"false"`), 1),
		"index-bool":        bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":false`), 1),
		"extra-field":       bytes.Replace(raw, []byte(`"index":0`), []byte(`"extra":0,"index":0`), 1),
		"duplicate-key":     bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":0,"index":0`), 1),
		"wrong-oracle":      bytes.Replace(raw, []byte(`"oracle":{"text":""`), []byte(`"oracle":{"text":"a"`), 1),
		"trailing-row-json": bytes.Replace(raw, []byte{'\n'}, []byte("{}\n"), 1),
	}
	reordered := append([][]byte{}, lines...)
	reordered[1], reordered[2] = reordered[2], reordered[1]
	mutations["order"] = bytes.Join(reordered, []byte{'\n'})
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, mutated) {
				t.Fatal("mutation did not alter fixture")
			}
			if _, err := inspectPathRows(mutated); err == nil {
				t.Fatal("mutated typed rows accepted")
			}
		})
	}
	if _, err := inspectPathRows([]byte(`{"decision":"PASS","input_count":9841}` + "\n")); err == nil {
		t.Fatal("report-only PASS accepted as full rows")
	}
}

func TestPathGeneratedChildReportMutations(t *testing.T) {
	e, r, generated, id := pathBindingFixture(t, "ext")
	rows, err := inspectPathRows(pathRowsFixture(t, "ext"))
	if err != nil {
		t.Fatal(err)
	}
	want := pathExpectedChildReport(generated, e, r, id, rows)
	want["harness_sha256"] = digest([]byte("fixture-harness"))
	want["executable_sha256"] = digest([]byte("self-observed-binary"))
	raw, _ := json.Marshal(want)
	decoded, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectPathChildReport(decoded, want); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"input_count", "generated_sha256", "harness_sha256", "rows", "full_saved_replay_outputs", "mismatches"} {
		t.Run(name, func(t *testing.T) {
			d, err := decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			m := object(d)
			if name == "rows" {
				object(m[name])["sha256"] = digest([]byte("wrong-rows"))
			} else {
				m[name] = false
			}
			if err := inspectPathChildReport(m, want); err == nil {
				t.Fatal("mutated child report accepted")
			}
		})
	}
	if err := inspectPathChildReport(map[string]any{"decision": "PASS"}, want); err == nil {
		t.Fatal("report-only PASS accepted")
	}
}

// This regression builds the production harness once, then uses its already
// built test binary for positive and seven deliberately wrong body modes. The
// fixture is explicitly synthetic Go, with test instrumentation outside the
// candidate ABI; it is not accepted as a Gooo receipt or native corpus PASS.
func TestPathHarnessInvokesBoundFunction(t *testing.T) {
	_, _, _, id := pathBindingFixture(t, "report")
	if runtime.Version() != goVersion {
		t.Fatal("harness regression requires pinned Go")
	}
	abi := pathGeneratedABI{Function: "GoooComposedActivity0", Record: pathRecordSymbol(resultEntity)}
	var decl strings.Builder
	for i := range pathFieldNames {
		abi.Fields[i] = pathFieldSymbol(resultEntity + "/" + pathFieldIDs[i])
		fmt.Fprintf(&decl, "%s %s `json:%q`\n", abi.Fields[i], pathFieldTypes[i], pathFieldNames[i])
	}
	fixture := "package main\nimport (\"os\";\"path\")\ntype " + abi.Record + " struct {" + decl.String() + "}\n"
	fixture += `var regressionCalls int
var regressionInputs=func() []string {
 all,level:=[]string{""},[]string{""}
 for length:=1; length<=8; length++ {
  next:=[]string{}; for _,prefix:=range level { for _,letter:=range []string{"a",".","/"} { next=append(next,prefix+letter) } }; all=append(all,next...); level=next
 }; return all
}()
func regressionInput(index int) string { if index>=len(regressionInputs) { panic("unexpected extra candidate call") }; return regressionInputs[index] }
`
	fixture += "func " + abi.Function + "(input string) " + abi.Record + ` {
 if input!=regressionInput(regressionCalls) { panic("candidate original input/order differs") }; regressionCalls++
 extension,absolute:=path.Ext(input),path.IsAbs(input)
 value:=RECORD{FIELD0:input,FIELD1:extension,FIELD2:absolute,FIELD3:int64(len(input)),FIELD4:int64(len(extension)),FIELD5:len(extension)>0}
 switch os.Getenv("GOOO_PATH_REGRESSION_MODE") {
 case "text": value.FIELD0+="!"
 case "ext": value.FIELD1+="!"
 case "is_abs": value.FIELD2=!value.FIELD2
 case "text_byte_length": value.FIELD3++
 case "ext_byte_length": value.FIELD4++
 case "has_extension": value.FIELD5=!value.FIELD5
 case "wrong-entry": return RECORD{}
 }
 return value
}
`
	replacements := []string{"RECORD", abi.Record}
	for i, field := range abi.Fields {
		replacements = append(replacements, fmt.Sprint("FIELD", i), field)
	}
	fixture = strings.NewReplacer(replacements...).Replace(fixture)
	generated := digest([]byte(fixture))
	e, r := []byte("synthetic-execute"), []byte("synthetic-replay")
	tool := filepath.Join(runtime.GOROOT(), "bin/go")
	toolRaw, err := os.ReadFile(tool)
	if err != nil {
		t.Fatal(err)
	}
	id.GoTool = digest(toolRaw)
	harness, err := makePathHarness(abi, generated, e, r, id)
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	work, out := filepath.Join(base, "work"), filepath.Join(base, "evidence")
	native := filepath.Join(out, "native")
	cache, tmp, gopath := filepath.Join(work, "cache"), filepath.Join(work, "tmp"), filepath.Join(work, "gopath")
	for _, dir := range []string{work, out, native, cache, tmp, gopath} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{"generated.go": []byte(fixture), "exhaustive_test.go": harness, "go.mod": []byte("module gooo-path-harness-regression\n\ngo 1.27.2\n"), "regression_test.go": []byte(pathHarnessRegressionDriver)}
	for name, raw := range files {
		if err := pathExclusive(filepath.Join(native, name), raw); err != nil {
			t.Fatal(err)
		}
	}
	_, err = runPathNativeTest(context.Background(), tool, native, tmp, cache, gopath, filepath.Join(out, "unused-rows"), filepath.Join(out, "unused-report"), base, work, out, "^TestPathHarnessRegressionDriver$")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "positive", "rows.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := inspectPathRows(raw)
	if err != nil {
		t.Fatal(err)
	}
	if rows.Rows != 9841 || rows.ScalarComparisons != 59046 {
		t.Fatal("actual harness failed full count/all6 fields")
	}
	child, err := os.ReadFile(filepath.Join(out, "positive", "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decode(child)
	if err != nil {
		t.Fatal(err)
	}
	want := pathExpectedChildReport(generated, e, r, id, rows)
	want["harness_sha256"] = digest(harness)
	if err := inspectPathChildReport(decoded, want); err != nil {
		t.Fatal(err)
	}
}

const pathHarnessRegressionDriver = `package main
import ("context";"fmt";"os";"os/exec";"path/filepath";"strings";"testing";"time")
func TestMain(m *testing.M) {
 code:=m.Run()
 if os.Getenv("GOOO_PATH_REGRESSION_MODE")=="positive" && code==0 && regressionCalls!=9841 { fmt.Fprintln(os.Stderr,"candidate not called9841 times"); code=1 }; os.Exit(code)
}
func TestPathHarnessRegressionDriver(t *testing.T) {
 executable,err:=os.Executable(); if err!=nil { t.Fatal(err) }
 base:=filepath.Dir(os.Getenv("GOOO_PATH_REPORT"))
 for _,mode:=range []string{"positive","text","ext","is_abs","text_byte_length","ext_byte_length","has_extension","wrong-entry"} {
  dir:=filepath.Join(base,mode); if err:=os.Mkdir(dir,0700); err!=nil { t.Fatal(err) }
  ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second)
  cmd:=exec.CommandContext(ctx,executable,"-test.run=^TestPathGeneratedCorpus$","-test.timeout=15s")
  cmd.Env=append(os.Environ(),"GOOO_PATH_REGRESSION_MODE="+mode,"GOOO_PATH_ROWS="+filepath.Join(dir,"rows.jsonl"),"GOOO_PATH_REPORT="+filepath.Join(dir,"report.json"))
  output,runErr:=cmd.CombinedOutput(); contextErr:=ctx.Err(); cancel()
  if contextErr!=nil { t.Fatalf("mode%s timed out: %v",mode,contextErr) }
  if mode=="positive" && runErr!=nil { t.Fatalf("positive harness: %v %s",runErr,output) }
  if mode!="positive" && runErr==nil { t.Fatalf("wrong generated field/body mode%s passed comparator",mode) }
  if mode!="positive" && !strings.Contains(string(output),"original Go mismatch at input index") { t.Fatalf("mode%s failed for another reason: %s",mode,output) }
 }
}
`
