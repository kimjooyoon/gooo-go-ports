package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func headerAdditionTestIdentity(t *testing.T) expectedIdentity {
	t.Helper()
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.workspaceForSuite(suite{Package: headerAdditionPackage, Activity: headerAdditionActivity})
	if err != nil {
		t.Fatal(err)
	}
	return expectedIdentity{CompilerRevision: uuidCompactCompilerRevision, GoVersion: "go1.27.2", Sources: w.Identities, SourceBytes: w.Sources}
}
func TestHeaderAdditionCurrentFiveSourcesSixActivitiesAndRecordFields(t *testing.T) {
	id := headerAdditionTestIdentity(t)
	defs, err := headerAdditionDefinitions(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 6 || len(headerAdditionRecords()) != 5 || len(headerAdditionResultContract(suite{Package: headerAdditionPackage, Activity: headerAdditionActivity}).Fields) != 16 {
		t.Fatal("ABI population changed")
	}
	calls, err := headerAdditionPureCalls(defs)
	if err != nil || len(calls["activities"].([]any)) != 6 || len(calls["sites"].([]any)) != 5 {
		t.Fatal("source-derived call graph", err)
	}
	// Rebinding a modified source digest must not disguise a schema/signature
	// change as the reviewed original source contract.
	for _, change := range []struct{ before, after string }{
		{`field content_length_occurrences id "` + headerAdditionInputEntity + `/content_length_occurrences" type integer`, `field content_length_occurrences id "` + headerAdditionInputEntity + `/content_length_occurrences" type boolean`},
		{`field request_revision id "` + headerAdditionResultEntity + `/request_revision"`, `field request_revision id "` + headerAdditionResultEntity + `/other_revision"`},
		{`AssessAddition(Text, Text, Text, RequestFacts)`, `AssessAddition(Text, Text, Text, Text)`},
		{`headers.Report(input0, input1)`, `payload.Report(input0, input1)`},
	} {
		identity := headerAdditionTestIdentity(t)
		raw := string(identity.SourceBytes[headerAdditionSource])
		if !strings.Contains(raw, change.before) {
			t.Fatal("mutation anchor absent", change.before)
		}
		identity.SourceBytes[headerAdditionSource] = []byte(strings.Replace(raw, change.before, change.after, 1))
		encoded, e := json.Marshal(string(identity.SourceBytes[headerAdditionSource]))
		if e != nil {
			t.Fatal(e)
		}
		identity.Sources[headerAdditionSource] = sourceIdentity{headerAdditionPackage, contentDigest(encoded)}
		altered, e := headerAdditionDefinitions(identity)
		if e == nil {
			_, e = headerAdditionPureCalls(altered)
		}
		if e == nil {
			t.Fatal("changed source contract/callee accepted", change.before)
		}
	}
}
func TestHeaderAdditionInputFieldIdentityOrderAndExactIntegerValues(t *testing.T) {
	suites, err := headerAdditionCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	want := suites[0].Cases[0].Inputs[3].(map[string]any)
	fields := []recordFieldValue{}
	for _, name := range headerAdditionInputNames {
		raw, e := json.Marshal(want[name])
		if e != nil {
			t.Fatal(e)
		}
		fields = append(fields, recordFieldValue{ID: headerAdditionInputEntity + "/" + name, Name: name, Value: raw})
	}
	if err := inspectHeaderAdditionInputFields(headerAdditionInputEntity, fields, want); err != nil {
		t.Fatal("positive reviewed input fields", err)
	}
	for _, mutate := range []func([]recordFieldValue){func(f []recordFieldValue) { f[0], f[1] = f[1], f[0] }, func(f []recordFieldValue) { f[0].ID += "-other" }, func(f []recordFieldValue) { f[0].Value = json.RawMessage("1.0") }, func(f []recordFieldValue) { f[3].Value = json.RawMessage("1") }} {
		changed := append([]recordFieldValue(nil), fields...)
		mutate(changed)
		if inspectHeaderAdditionInputFields(headerAdditionInputEntity, changed, want) == nil {
			t.Fatal("changed typed input record accepted")
		}
	}
	if inspectHeaderAdditionInputFields(textEntity, fields, want) == nil {
		t.Fatal("record metadata accepted on scalar Text")
	}
}

// This witness only supplies concrete Go syntax for the ABI inspector. Bodies
// make the five reviewed calls and return zero values; they do not implement
// the policy, inherit private generated source, or establish native semantics.
func headerAdditionSyntheticNativeABI(defs []headerAdditionDefinition, composed bool) (string, []string, map[string]string) {
	var source strings.Builder
	source.WriteString("package synthetic\n")
	kinds := map[string]string{"Text": "string", "Integer": "int64", "Boolean": "bool"}
	for _, value := range headerAdditionRecords() {
		record := value.(map[string]any)
		name := record["go_name"].(string)
		kinds[record["name"].(string)] = name
		fmt.Fprintf(&source, "type %s struct {\n", name)
		for _, value := range record["fields"].([]any) {
			field := value.(map[string]any)
			kind := map[string]string{"urn:gooo:type:string": "string", "urn:gooo:type:integer": "int64", "urn:gooo:type:boolean": "bool"}[field["type_id"].(string)]
			fmt.Fprintf(&source, "%s %s `json:%q`\n", field["go_name"], kind, field["name"])
		}
		source.WriteString("}\n")
	}
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Lowered
		if composed {
			names[i] = fmt.Sprintf("GoooComposedActivity0Call%d", i)
			if i == 5 {
				names[i] = "GoooComposedActivity0"
			}
		}
	}
	zero := func(kind string) string {
		switch kind {
		case "string":
			return `""`
		case "int64":
			return "0"
		case "bool":
			return "false"
		}
		return kind + "{}"
	}
	// Keep the witness graph explicit, separate from the inspector's edge table.
	calls := map[int][]int{2: {0, 1}, 4: {3}, 5: {2, 4}}
	for i, d := range defs {
		parameters := []string{}
		for j, kind := range d.Declaration.Inputs {
			name := "input"
			if len(d.Declaration.Inputs) > 1 {
				name = fmt.Sprintf("input%d", j)
			}
			parameters = append(parameters, name+" "+kinds[kind])
		}
		fmt.Fprintf(&source, "func %s(%s) %s {\n", names[i], strings.Join(parameters, ", "), kinds[d.Declaration.Output])
		for _, callee := range calls[i] {
			arguments := []string{}
			for _, kind := range defs[callee].Declaration.Inputs {
				arguments = append(arguments, zero(kinds[kind]))
			}
			fmt.Fprintf(&source, "_ = %s(%s)\n", names[callee], strings.Join(arguments, ", "))
		}
		fmt.Fprintf(&source, "return %s\n}\n", zero(kinds[d.Declaration.Output]))
	}
	return source.String(), names, kinds
}

func TestHeaderAdditionNativeABIRejectsConcreteFieldParameterAndCalleeMutations(t *testing.T) {
	defs, err := headerAdditionDefinitions(headerAdditionTestIdentity(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, composed := range []bool{false, true} {
		source, names, kinds := headerAdditionSyntheticNativeABI(defs, composed)
		if err := inspectHeaderAdditionNativeABI(source, defs, composed); err != nil {
			t.Fatal("positive synthetic five-record/six-function/five-call ABI", composed, err)
		}
		first := "GoooField" + strings.TrimPrefix(contentDigest([]byte(headerAdditionInputEntity+"/content_length_occurrences")), "sha256:")
		second := "GoooField" + strings.TrimPrefix(contentDigest([]byte(headerAdditionInputEntity+"/observed_body_bytes")), "sha256:")
		firstLine := first + " int64 `json:\"content_length_occurrences\"`\n"
		secondLine := second + " int64 `json:\"observed_body_bytes\"`\n"
		rootSignature := names[5] + "(input0 string, input1 string, input2 string, input3 " + kinds["RequestFacts"] + ")"
		nameCall := "_ = " + names[0] + "(\"\")\n"
		mutations := []struct{ name, before, after string }{
			{"field-tag", `json:"content_length_occurrences"`, `json:"other_occurrences"`},
			{"field-type", first + " int64", first + " bool"},
			{"field-order", firstLine + secondLine, secondLine + firstLine},
			{"field-removed", firstLine, ""},
			{"record-removed", "type " + kinds["RequestFacts"] + " struct", "type OtherRequestFacts struct"},
			{"parameter-arity", rootSignature, names[5] + "(input0 string, input1 string, input2 string)"},
			{"parameter-record-type", rootSignature, names[5] + "(input0 string, input1 string, input2 string, input3 string)"},
			{"parameter-name", rootSignature, names[5] + "(input0 string, input1 string, changed string, input3 " + kinds["RequestFacts"] + ")"},
			{"callee-removed", nameCall, ""},
			{"callee-extra", nameCall, nameCall + nameCall},
			{"callee-redirected", nameCall, "_ = " + names[1] + "(\"\")\n"},
		}
		for _, mutation := range mutations {
			if !strings.Contains(source, mutation.before) {
				t.Fatal("synthetic mutation anchor absent", mutation.name)
			}
			changed := strings.Replace(source, mutation.before, mutation.after, 1)
			if err := inspectHeaderAdditionNativeABI(changed, defs, composed); err == nil {
				t.Fatal("changed synthetic ABI accepted", composed, mutation.name)
			}
		}
		if err := inspectHeaderAdditionNativeABI(source+"func Unrelated(input string) bool {return false}\n", defs, composed); err == nil {
			t.Fatal("unexpected native function accepted", composed)
		}
	}
}
