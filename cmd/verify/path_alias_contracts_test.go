package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Synthetic source-bound metadata exercises the real inspector and fresh Go
// expected values. It is never executed or represented as a native receipt.
func pathAliasReceiptFixture(t *testing.T, original suite, row int) (map[string]any, suite, expectedIdentity) {
	t.Helper()
	s := original
	s.Cases = []testCase{original.Cases[row]}
	catalog, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := catalog.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	id := expectedIdentity{CompilerRevision: "83928fa4538245abe33b08549da47f8cbdc50ed2", GoVersion: "go1.27.3", ManifestDigest: contentDigest(mustJSON(t, selected.Manifest)), CasesDigest: contentDigest(mustJSON(t, s.Cases)), Sources: selected.Identities, SourceBytes: selected.Sources}
	definitions, err := pathAliasBodies(s, id)
	if err != nil {
		t.Fatal(err)
	}
	consumer := s.Package == pathAliasPackage
	root := definitions[s.Activity]
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return definitions[names[i]].Lowered < definitions[names[j]].Lowered })
	var source strings.Builder
	source.WriteString("package gooo_workspace\nnamespace gooo_workspace\n")
	for _, name := range names {
		d := definitions[name]
		fmt.Fprintf(&source, "activity %s(%s) -> %s id %q computes %q\n", d.Lowered, strings.Join(d.Declaration.Inputs, ", "), d.Declaration.Output, d.Declaration.ID, pathAliasLowerBody(d.Declaration.Body, definitions))
	}
	text := source.String()
	sourceSHA := contentDigest([]byte(text))
	contract, _ := contractFor(s)
	rootRef := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	packages := []any{}
	initOrder := []string{}
	for _, pkg := range selected.Manifest.Packages {
		files := []any{}
		exports := []any{}
		for _, file := range pkg.Sources {
			files = append(files, map[string]any{"filename": file, "source_digest": selected.Identities[file].Digest})
		}
		for _, name := range names {
			d := definitions[name]
			if d.Package != pkg.Path {
				continue
			}
			entities := []string{}
			for _, kind := range d.Declaration.Inputs {
				entities = append(entities, map[string]string{"Text": textEntity, "Integer": integerEntity, "Boolean": "gooo://std/boolean"}[kind])
			}
			out := map[string]string{"Text": textEntity, "Boolean": "gooo://std/boolean", "CleanState": pathCleanStateEntity, "PathAliasFacts": pathAliasResultEntity}[d.Declaration.Output]
			exports = append(exports, map[string]any{"name": name, "kind": "activity", "id": d.Declaration.ID, "input_types": entities, "output_type": out})
		}
		packages = append(packages, map[string]any{"path": pkg.Path, "sources": files, "exports": exports})
		initOrder = append(initOrder, pkg.Path)
	}
	image := map[string]any{"schema": "gooo/package-runtime-image/v1", "entry": map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}, "init_order": initOrder, "packages": packages}
	activity := map[string]any{"name": root.Lowered, "id": contract.ID, "input_type": "Text", "output_type": root.Declaration.Output, "input_entity_id": textEntity, "output_entity_id": pathAliasResultContract(s).Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	if consumer {
		activity["inputs"] = []any{map[string]any{"port": "input0", "type": "Text", "entity_id": textEntity, "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": textEntity, "from": -1}}
	}
	records := pathAliasRecords(consumer)
	plan := map[string]any{"schema": "gooo/body-composition-plan/v1", "entry_activity": root.Lowered, "typed_plan_sha256": contentDigest([]byte("synthetic plan, not recomputed")), "activities": []any{activity}, "record_types": records, "edges": []any{}}
	generated := "package main\n// synthetic inspector consistency fixture; not executed\n"
	generatedSHA := contentDigest([]byte(generated))
	driver := "synthetic driver"
	inputType, outputType := "string", "string"
	if consumer {
		inputType, outputType = "(string,string)", "PathAliasFacts"
	}
	generation := map[string]any{"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity": root.Lowered, "activity_id": contract.ID, "compiler_source_sha": id.CompilerRevision, "source_digest": sourceSHA, "program_digest": contentDigest([]byte(pathAliasLowerBody(root.Declaration.Body, definitions))), "generated_digest": generatedSHA, "replay_digest": generatedSHA, "input_type": inputType, "output_type": outputType, "record_types": records, "call_closure": pathAliasCallClosure(definitions, consumer), "typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0}
	if consumer {
		generation["input_parameters"] = []any{map[string]any{"name": "input0", "type": "string"}, map[string]any{"name": "input1", "type": "string"}}
	}
	actual := s.Cases[0].Expected
	delivery := map[string]any{"activity_id": contract.ID, "actual": actual, "expected": actual, "passed": true}
	if consumer {
		ports := []any{}
		for i, value := range s.Cases[0].Inputs {
			ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "entity_id": textEntity, "value": value})
		}
		delivery["inputs"] = ports
		fields := []any{}
		for _, field := range pathAliasResultContract(s).Fields {
			fields = append(fields, map[string]any{"id": field.ID, "name": field.Name, "value": actual.(map[string]any)[field.Name]})
		}
		delivery["actual_fields"] = fields
	} else {
		delivery["input"] = s.Cases[0].Inputs[0]
	}
	m := map[string]any{"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "PASS", "manifest_digest": id.ManifestDigest, "cases_digest": id.CasesDigest, "result": map[string]any{
		"execution_source_sha256": sourceSHA,
		"program":                 map[string]any{"workspace_image": image, "entry": rootRef, "activities": []any{rootRef}, "lowered_gooo_source": text, "pure_calls": pathAliasPureCalls(definitions)},
		"composition":             map[string]any{"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "gooo_source": text, "original_source_sha256": sourceSHA, "selected_source_sha256": sourceSHA, "generated_sha256": generatedSHA, "source": generated, "driver_source": driver, "driver_sha256": contentDigest([]byte(driver)), "plan": plan, "steps": []any{map[string]any{"input_source_sha256": sourceSHA, "generation": map[string]any{"source": generated, "report": generation}}}},
		"runtime":                 map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": id.CompilerRevision, "go_version": "go version go1.27.3 linux/amd64", "generated_sha256": generatedSHA, "driver_sha256": contentDigest([]byte(driver)), "typed_plan_sha256": plan["typed_plan_sha256"], "original_source_sha256": sourceSHA, "selected_source_sha256": sourceSHA, "model_calls": 0, "finite_passed": 1, "finite_total": 1, "projection_replayed": true, "runtime_replayed": true, "traces": []any{map[string]any{"case_index": 0, "deliveries": []any{delivery}}}},
	}}
	return pathAliasTestClone(t, m), s, id
}

func pathAliasTestClone(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	v, err := decode(mustJSON(t, m))
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}
func pathAliasTestMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	v, err := decimalPayloadReceiptObject(m, keys...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func pathAliasTestDelivery(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	return pathAliasTestMap(t, m, "result", "runtime")["traces"].([]any)[0].(map[string]any)["deliveries"].([]any)[0].(map[string]any)
}
func pathAliasTestGeneration(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	return pathAliasTestMap(t, m, "result", "composition")["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)["report"].(map[string]any)
}

func TestPathAliasExactResultContractsAndInternalRecordKinds(t *testing.T) {
	clean := pathAliasResultContract(suite{Package: pathCleanPackage, Activity: pathCleanActivity})
	if clean.Entity != textEntity || len(clean.Fields) != 0 {
		t.Fatal("Clean scalar output contract")
	}
	report := pathAliasResultContract(suite{Package: pathAliasPackage, Activity: pathAliasActivity})
	want := []string{"left", "right", "left_clean", "right_clean", "left_is_abs", "right_is_abs", "left_changed", "right_changed", "exact_same", "same_clean", "lexical_alias"}
	if report.Entity != pathAliasResultEntity || len(report.Fields) != 11 {
		t.Fatal("eleven required facts")
	}
	for i, field := range report.Fields {
		if field.Name != want[i] || field.ID != "gooo://go-ports/examples/path-alias-facts/result/"+strings.ReplaceAll(want[i], "_", "-") {
			t.Fatal("fixed declared field identity/order", field)
		}
	}
	if pathAliasResultContract(suite{Package: pathCleanPackage, Activity: "Ext"}).Entity != "" {
		t.Fatal("old path entry must remain outside new contract")
	}
	state := pathAliasRecords(false)[0].(map[string]any)
	if state["id"] != pathCleanStateEntity || state["fields"].([]any)[0].(map[string]any)["type_id"] != "urn:gooo:type:string" || state["fields"].([]any)[1].(map[string]any)["type_id"] != "urn:gooo:type:integer" {
		t.Fatal("internal CleanState shape")
	}
}

func TestPathAliasInspectorRejectsCurrentSourceCallAndRecordTampering(t *testing.T) {
	suites, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range suites {
		t.Run(original.Package, func(t *testing.T) {
			base, s, id := pathAliasReceiptFixture(t, original, 0)
			first, err := inspect(mustJSON(t, base), s, false, id)
			if err != nil {
				t.Fatal("positive synthetic current-source fixture", err)
			}
			replayed := pathAliasTestClone(t, base)
			replayed["replayed_from_sha256"] = contentDigest(mustJSON(t, base))
			pathAliasTestMap(t, replayed, "result")["replay"] = map[string]any{"model_calls": 0}
			second, err := inspect(mustJSON(t, replayed), s, true, id)
			if err != nil {
				t.Fatal("positive saved fixture", err)
			}
			if err := inspectReplayBinding(first, second, contentDigest(mustJSON(t, base))); err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(first, second, contentDigest([]byte("foreign parent"))); err == nil {
				t.Fatal("foreign raw receipt parent")
			}
			mutations := map[string]func(map[string]any){
				"selected_entry": func(m map[string]any) {
					pathAliasTestMap(t, m, "result", "program", "entry")["lowered_name"] = "GoooPackage9ActivityFake"
				},
				"image_entry": func(m map[string]any) {
					pathAliasTestMap(t, m, "result", "program", "workspace_image", "entry")["source"] = "foreign.gooo"
				},
				"pure_body": func(m map[string]any) {
					p := pathAliasTestMap(t, m, "result", "program", "pure_calls")
					p["activities"].([]any)[0].(map[string]any)["source_body_sha256"] = contentDigest([]byte("foreign body"))
				},
				"pure_site": func(m map[string]any) {
					p := pathAliasTestMap(t, m, "result", "program", "pure_calls")
					p["sites"].([]any)[0].(map[string]any)["start_byte"] = json.Number("0")
				},
				"plan_function": func(m map[string]any) {
					p := pathAliasTestMap(t, m, "result", "composition", "plan")
					p["activities"].([]any)[0].(map[string]any)["go_function"] = "WrongFunction"
				},
				"callee_program": func(m map[string]any) {
					g := pathAliasTestGeneration(t, m)
					g["call_closure"].(map[string]any)["activities"].([]any)[0].(map[string]any)["program_sha256"] = contentDigest([]byte("wrong helper"))
				},
				"callee_edge": func(m map[string]any) {
					g := pathAliasTestGeneration(t, m)
					g["call_closure"].(map[string]any)["edges"].([]any)[0].(map[string]any)["ordinal"] = json.Number("0")
				},
				"generated_activity": func(m map[string]any) { pathAliasTestGeneration(t, m)["activity"] = "WrongFunction" },
				"driver": func(m map[string]any) {
					pathAliasTestMap(t, m, "result", "composition")["driver_source"] = "changed driver"
				},
				"plan_binding": func(m map[string]any) {
					pathAliasTestMap(t, m, "result", "runtime")["typed_plan_sha256"] = contentDigest([]byte("other plan"))
				},
				"coherent_lowered_mutant": func(m map[string]any) {
					p := pathAliasTestMap(t, m, "result", "program")
					text := p["lowered_gooo_source"].(string)
					changed := strings.Replace(text, "let cut = int64(len(input0)) - 1", "let cut = int64(len(input0)) - 2", 1)
					if changed == text {
						t.Fatal("mutation must change helper body")
					}
					p["lowered_gooo_source"] = changed
					digest := contentDigest([]byte(changed))
					r := pathAliasTestMap(t, m, "result")
					r["execution_source_sha256"] = digest
					c := pathAliasTestMap(t, m, "result", "composition")
					c["gooo_source"], c["original_source_sha256"], c["selected_source_sha256"] = changed, digest, digest
					c["steps"].([]any)[0].(map[string]any)["input_source_sha256"] = digest
					pathAliasTestGeneration(t, m)["source_digest"] = digest
					rt := pathAliasTestMap(t, m, "result", "runtime")
					rt["original_source_sha256"], rt["selected_source_sha256"] = digest, digest
				},
			}
			for name, mutate := range mutations {
				changed := pathAliasTestClone(t, base)
				mutate(changed)
				if _, err := inspect(mustJSON(t, changed), s, false, id); err == nil {
					t.Fatal("tampered current-source ABI admitted", name)
				}
			}
			missing := id
			missing.SourceBytes = nil
			if _, err := inspect(mustJSON(t, base), s, false, missing); err == nil {
				t.Fatal("missing current source byte roster")
			}
			changed := id
			changed.SourceBytes = map[string][]byte{}
			for name, raw := range id.SourceBytes {
				changed.SourceBytes[name] = append([]byte(nil), raw...)
			}
			changed.SourceBytes[pathCleanSource] = append(changed.SourceBytes[pathCleanSource], []byte("\n// changed")...)
			if _, err := inspect(mustJSON(t, base), s, false, changed); err == nil {
				t.Fatal("source-byte/snapshot mismatch")
			}
		})
	}
}

func TestPathAliasEveryResultFieldAndScalarPortMustRetainExactKinds(t *testing.T) {
	suites, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range suites {
		base, s, id := pathAliasReceiptFixture(t, original, 0)
		if _, err := inspect(mustJSON(t, base), s, false, id); err != nil {
			t.Fatal("positive typed base", err)
		}
		if s.Package == pathCleanPackage {
			for _, wrong := range []any{nil, false, json.Number("1"), "wrong Clean value"} {
				m := pathAliasTestClone(t, base)
				pathAliasTestDelivery(t, m)["actual"] = wrong
				if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
					t.Fatal("wrong scalar Clean value/kind")
				}
			}
			for _, wrong := range []any{nil, false, json.Number("0")} {
				m := pathAliasTestClone(t, base)
				pathAliasTestDelivery(t, m)["input"] = wrong
				if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
					t.Fatal("wrong unary Text input kind")
				}
			}
			m := pathAliasTestClone(t, base)
			pathAliasTestDelivery(t, m)["input_fields"] = []any{map[string]any{"name": "fake", "id": "fake", "value": ""}}
			if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
				t.Fatal("scalar Clean input record fields")
			}
			continue
		}
		for i, field := range pathAliasResultContract(s).Fields {
			m := pathAliasTestClone(t, base)
			d := pathAliasTestDelivery(t, m)
			actual := d["actual"].(map[string]any)
			var wrong any
			if i < 4 {
				wrong = "changed original/clean Text"
			} else {
				wrong = !actual[field.Name].(bool)
			}
			actual[field.Name] = wrong
			d["actual_fields"].([]any)[i].(map[string]any)["value"] = wrong
			if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
				t.Fatal("coherent wrong actual field bypassed Go comparison", field.Name)
			}
			m = pathAliasTestClone(t, base)
			g := pathAliasTestGeneration(t, m)
			g["record_types"].([]any)[0].(map[string]any)["fields"].([]any)[i].(map[string]any)["type_id"] = "urn:gooo:type:integer"
			if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
				t.Fatal("declared returned field kind weakened", field.Name)
			}
			m = pathAliasTestClone(t, base)
			pathAliasTestDelivery(t, m)["actual_fields"].([]any)[i].(map[string]any)["id"] = "foreign-field"
			if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
				t.Fatal("field identity changed", field.Name)
			}
		}
		for port := 0; port < 2; port++ {
			for _, wrong := range []any{nil, false, json.Number("1")} {
				m := pathAliasTestClone(t, base)
				pathAliasTestDelivery(t, m)["inputs"].([]any)[port].(map[string]any)["value"] = wrong
				if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
					t.Fatal("Text port coerced", port)
				}
			}
			m := pathAliasTestClone(t, base)
			pathAliasTestDelivery(t, m)["inputs"].([]any)[port].(map[string]any)["fields"] = []any{map[string]any{"name": "fake", "id": "fake", "value": ""}}
			if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
				t.Fatal("scalar Text port has record fields", port)
			}
		}
	}
}
