package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func canonicalHeaderTestIdentity(t *testing.T, s suite) expectedIdentity {
	t.Helper()
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := frozen.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	return expectedIdentity{CompilerRevision: uuidCompactCompilerRevision, GoVersion: "go1.27.3", Sources: w.Identities, SourceBytes: w.Sources}
}
func canonicalHeaderTestSuites() []suite {
	return []suite{{Package: canonicalHeaderPackage, Activity: canonicalHeaderActivity}, {Package: canonicalHeaderReportPackage, Activity: canonicalHeaderReportActivity}}
}
func TestCanonicalHeaderSourceReachabilityFieldsAndSingleTextContract(t *testing.T) {
	for _, s := range canonicalHeaderTestSuites() {
		consumer := canonicalHeaderConsumer(s)
		identity := canonicalHeaderTestIdentity(t, s)
		defs, err := canonicalHeaderDefinitions(s, identity)
		if err != nil {
			t.Fatal(err)
		}
		wanted := 2
		if consumer {
			wanted = 5
		}
		if len(defs) != wanted {
			t.Fatal("reached bodies", len(defs), wanted)
		}
		calls, err := canonicalHeaderPureCalls(defs, consumer)
		if err != nil {
			t.Fatal(err)
		}
		sites := 1
		if consumer {
			sites = 5
		}
		if len(calls["activities"].([]any)) != wanted || len(calls["sites"].([]any)) != sites {
			t.Fatal("pure source closure")
		}
		root := defs[len(defs)-1]
		if len(root.Declaration.Inputs) != 1 || root.Declaration.Inputs[0] != "Text" {
			t.Fatal("not single scalar Text")
		}
		if !consumer && root.Lowered != "GoooPackage2ActivityCanonicalMIMEHeaderKey" {
			t.Fatal("unused header declaration lost its ordinal")
		}
		result := canonicalHeaderResultContract(s)
		if !consumer && (result.Entity != textEntity || len(result.Fields) != 0) {
			t.Fatal("scalar result contract")
		}
		if consumer && (result.Entity != canonicalHeaderResultEntity || len(result.Fields) != 8 || len(canonicalHeaderRecords(true)) != 2) {
			t.Fatal("Report8 and HeaderFacts12")
		}
		activity, err := contractFor(s)
		if err != nil || activity.Source != root.Source || activity.ID != root.Declaration.ID || len(activity.InputEntities) != 1 || activity.InputEntities[0] != textEntity {
			t.Fatal("registered source contract", err)
		}
	}
	for _, change := range []struct{ source, before, after string }{
		{canonicalHeaderSource, `CanonicalMIMEHeaderKey(Text)`, `CanonicalMIMEHeaderKey(Boolean)`},
		{canonicalHeaderReportSource, `field name_valid id "` + canonicalHeaderResultEntity + `/name_valid" type boolean`, `field name_valid id "` + canonicalHeaderResultEntity + `/name_valid" type integer`},
		{canonicalHeaderReportSource, `/canonical_key" type string`, `/other_key" type string`},
		{canonicalHeaderReportSource, `headers.Report(input, "")`, `textproto.Report(input, "")`},
		{canonicalHeaderReportSource, `import textproto "` + canonicalHeaderPackage + `"`, `import other "` + canonicalHeaderPackage + `"`},
	} {
		s := canonicalHeaderTestSuites()[1]
		identity := canonicalHeaderTestIdentity(t, s)
		raw := string(identity.SourceBytes[change.source])
		if !strings.Contains(raw, change.before) {
			t.Fatal("mutation anchor", change.before)
		}
		identity.SourceBytes[change.source] = []byte(strings.Replace(raw, change.before, change.after, 1))
		encoded, err := json.Marshal(string(identity.SourceBytes[change.source]))
		if err != nil {
			t.Fatal(err)
		}
		identity.Sources[change.source] = sourceIdentity{identity.Sources[change.source].Package, contentDigest(encoded)}
		defs, err := canonicalHeaderDefinitions(s, identity)
		if err == nil {
			_, err = canonicalHeaderPureCalls(defs, true)
		}
		if err == nil {
			t.Fatal("source signature/field/callee/import mutation accepted", change.before)
		}
	}
}

// Concrete zero-return syntax is an ABI witness only. It is not generated
// native output, a policy implementation, an expected oracle or a PASS receipt.
func canonicalHeaderSyntheticABI(defs []canonicalHeaderDefinition, consumer, composed bool) (string, []string, map[string]string) {
	var source strings.Builder
	source.WriteString("package synthetic\n")
	kinds := map[string]string{"Text": "string", "Integer": "int64", "Boolean": "bool"}
	for _, value := range canonicalHeaderRecords(consumer) {
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
			if i == len(defs)-1 {
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
	// Explicit independent graph; do not copy the inspector's edge table.
	graph := map[int][]int{1: {0}}
	if consumer {
		graph = map[int][]int{2: {0, 1}, 3: {0}, 4: {2, 3}}
	}
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
		for _, callee := range graph[i] {
			args := []string{}
			for _, kind := range defs[callee].Declaration.Inputs {
				args = append(args, zero(kinds[kind]))
			}
			fmt.Fprintf(&source, "_ = %s(%s)\n", names[callee], strings.Join(args, ", "))
		}
		fmt.Fprintf(&source, "return %s\n}\n", zero(kinds[d.Declaration.Output]))
	}
	return source.String(), names, kinds
}
func TestCanonicalHeaderNativeABIRefusesFieldParameterAndCalleeChanges(t *testing.T) {
	for _, s := range canonicalHeaderTestSuites() {
		consumer := canonicalHeaderConsumer(s)
		defs, err := canonicalHeaderDefinitions(s, canonicalHeaderTestIdentity(t, s))
		if err != nil {
			t.Fatal(err)
		}
		for _, composed := range []bool{false, true} {
			source, names, _ := canonicalHeaderSyntheticABI(defs, consumer, composed)
			if err := inspectCanonicalHeaderNativeABI(source, defs, consumer, composed); err != nil {
				t.Fatal("positive synthetic ABI", consumer, composed, err)
			}
			root := names[len(names)-1] + "(input string)"
			call := "_ = " + names[0] + "(\"\")\n"
			mutations := []struct{ name, before, after string }{{"input-name", root, names[len(names)-1] + "(renamed string)"}, {"input-type", root, names[len(names)-1] + "(input bool)"}, {"input-arity", root, names[len(names)-1] + "()"}, {"callee-removed", call, ""}, {"callee-extra", call, call + call}}
			if consumer {
				first := "GoooField" + strings.TrimPrefix(contentDigest([]byte(canonicalHeaderResultEntity+"/key")), "sha256:")
				second := "GoooField" + strings.TrimPrefix(contentDigest([]byte(canonicalHeaderResultEntity+"/canonical_key")), "sha256:")
				a := first + " string `json:\"key\"`\n"
				b := second + " string `json:\"canonical_key\"`\n"
				mutations = append(mutations, struct{ name, before, after string }{"field-tag", `json:"key"`, `json:"other_key"`}, struct{ name, before, after string }{"field-type", first + " string", first + " bool"}, struct{ name, before, after string }{"field-order", a + b, b + a}, struct{ name, before, after string }{"field-removed", a, ""}, struct{ name, before, after string }{"callee-redirected", call, "_ = " + names[1] + "(\"\")\n"})
			}
			for _, m := range mutations {
				if !strings.Contains(source, m.before) {
					t.Fatal("synthetic mutation anchor", m.name)
				}
				if inspectCanonicalHeaderNativeABI(strings.Replace(source, m.before, m.after, 1), defs, consumer, composed) == nil {
					t.Fatal("changed synthetic ABI accepted", consumer, composed, m.name)
				}
			}
			if inspectCanonicalHeaderNativeABI(source+"func Unrelated(input string) bool {return false}\n", defs, consumer, composed) == nil {
				t.Fatal("unexpected function accepted")
			}
		}
	}
}
