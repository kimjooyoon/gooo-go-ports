package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func headerTestMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	v, err := decimalPayloadReceiptObject(m, keys...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func headerTestClone(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	v, err := decode(mustJSON(t, m))
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}

func headerTestGeneration(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	return headerTestMap(t, m, "result", "composition")["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)
}

func headerTestNative(function string, consumer bool) string {
	var out strings.Builder
	out.WriteString("package main\n")
	result := "bool"
	if consumer {
		result = "GoooRecord" + strings.TrimPrefix(contentDigest([]byte(headerFactsEntity)), "sha256:")
		fmt.Fprintf(&out, "type %s struct {\n", result)
		for i, f := range headerResultContract(suite{Package: headerReportPackage, Activity: headerReportActivity}).Fields {
			kind := map[string]string{"string": "string", "boolean": "bool", "integer": "int64"}[headerFieldKind(i)]
			fmt.Fprintf(&out, "%s %s `json:%q`\n", "GoooField"+strings.TrimPrefix(contentDigest([]byte(f.ID)), "sha256:"), kind, f.Name)
		}
		out.WriteString("}\n")
		fmt.Fprintf(&out, "func %s(input0 string, input1 string) %s { return %s{} }\n", function, result, result)
	} else {
		fmt.Fprintf(&out, "func %s(input string) bool { return false }\n", function)
	}
	return out.String()
}

// Synthetic metadata tests the inspector's boundaries, never native behavior.
// Its zero-value Go bodies are not executed or presented as porting evidence.
func headerReceiptFixture(t *testing.T, s suite) (map[string]any, suite, expectedIdentity) {
	t.Helper()
	consumer := s.Package == headerReportPackage
	packages := []sourcePackage{{headerPortPackage, []string{headerPortSource}}}
	if consumer {
		packages = append(packages, sourcePackage{headerReportPackage, []string{headerReportSource}})
	}
	bytes := map[string][]byte{}
	for _, pkg := range packages {
		for _, file := range pkg.Sources {
			raw, err := os.ReadFile(filepath.Join("../..", file))
			if err != nil {
				t.Fatal(err)
			}
			bytes[file] = raw
		}
	}
	identities, err := snapshotSourceIdentities(packages, bytes)
	if err != nil {
		t.Fatal(err)
	}
	id := expectedIdentity{CompilerRevision: "83928fa4538245abe33b08549da47f8cbdc50ed2", GoVersion: "go1.27.2", ManifestDigest: contentDigest([]byte("synthetic manifest")), CasesDigest: contentDigest([]byte("synthetic case")), Sources: identities, SourceBytes: bytes}
	definitions, err := headerBodies(s, id)
	if err != nil {
		t.Fatal(err)
	}
	root := definitions[s.Activity]
	var expected any = true
	inputs := []any{"X-Test"}
	if consumer {
		inputs = []any{"X-Test", "ok"}
		expected = map[string]any{"name": "X-Test", "value": "ok", "name_valid": true, "value_valid": true, "name_empty": false, "name_has_uppercase": true, "value_has_obs_text": false, "value_edge_whitespace": false, "name_invalid_offset": int64(-1), "name_invalid_octet": int64(-1), "value_invalid_offset": int64(-1), "value_invalid_octet": int64(-1)}
	}
	s.Cases = []testCase{{Inputs: inputs, Expected: expected}}
	var source strings.Builder
	source.WriteString("package gooo_workspace\nnamespace gooo_workspace\nentity Text id \"gooo://std/text\"\nentity Boolean id \"gooo://std/boolean\"\n")
	if consumer {
		fmt.Fprintf(&source, "entity HeaderFacts id %q fields {\n", headerFactsEntity)
		for i, f := range headerResultContract(s).Fields {
			fmt.Fprintf(&source, "    field %s id %q type %s required one\n", f.Name, f.ID, headerFieldKind(i))
		}
		source.WriteString("}\n")
	}
	names := []string{s.Activity}
	if consumer {
		names = []string{headerNameActivity, headerValueActivity, headerReportActivity}
	}
	for _, name := range names {
		d := definitions[name]
		fmt.Fprintf(&source, "activity %s(%s) -> %s id %q computes %q\n", d.Lowered, strings.Join(d.Declaration.Inputs, ", "), d.Declaration.Output, d.Declaration.ID, headerLowerBody(d, definitions))
	}
	text, records := source.String(), headerRecords(consumer)
	sh := contentDigest([]byte(text))
	imagePackages := []any{}
	for _, pkg := range packages {
		files, exports := []any{}, []any{}
		for _, file := range pkg.Sources {
			files = append(files, map[string]any{"filename": file, "source_digest": identities[file].Digest})
		}
		for _, name := range []string{headerNameActivity, headerValueActivity, headerReportActivity} {
			d, present := definitions[name]
			if !present || d.Package != pkg.Path {
				continue
			}
			in, out := []string{textEntity}, "gooo://std/boolean"
			if name == headerReportActivity {
				in, out = []string{textEntity, textEntity}, headerFactsEntity
			}
			exports = append(exports, map[string]any{"name": name, "kind": "activity", "id": d.Declaration.ID, "input_types": in, "output_type": out})
		}
		imagePackages = append(imagePackages, map[string]any{"path": pkg.Path, "sources": files, "exports": exports})
	}
	order := []string{headerPortPackage}
	if consumer {
		order = append(order, headerReportPackage)
	}
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	image := map[string]any{"schema": "gooo/package-runtime-image/v1", "packages": imagePackages, "init_order": order, "entry": map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}}
	activity := map[string]any{"name": root.Lowered, "id": root.Declaration.ID, "input_type": "Text", "output_type": root.Declaration.Output, "input_entity_id": textEntity, "output_entity_id": headerResultContract(s).Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	plan := map[string]any{"schema": "gooo/body-composition-plan/v1", "typed_plan_sha256": contentDigest([]byte("synthetic typed plan")), "activities": []any{activity}, "edges": []any{}, "record_types": records}
	if consumer {
		plan["entry_activity"] = root.Lowered
		activity["inputs"] = []any{map[string]any{"port": "input0", "type": "Text", "entity_id": textEntity, "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": textEntity, "from": -1}}
	}
	generated, composed, driver := headerTestNative(root.Lowered, consumer), headerTestNative("GoooComposedActivity0", consumer), "synthetic driver"
	in, out := "string", "bool"
	if consumer {
		in, out = "(string,string)", "HeaderFacts"
	}
	g := map[string]any{"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity": root.Lowered, "activity_id": root.Declaration.ID, "compiler_source_sha": id.CompilerRevision, "source_digest": sh, "program_digest": contentDigest([]byte(headerLowerBody(root, definitions))), "generated_digest": contentDigest([]byte(generated)), "replay_digest": contentDigest([]byte(generated)), "input_type": in, "output_type": out, "record_types": records, "typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0}
	program := map[string]any{"schema": "gooo/workspace-body-program/v1", "workspace_image": image, "entry": entry, "activities": []any{entry}, "lowered_gooo_source": text}
	delivery := map[string]any{"activity_id": root.Declaration.ID, "actual": expected, "expected": expected, "passed": true}
	if consumer {
		calls, err := headerPureCalls(definitions)
		if err != nil {
			t.Fatal(err)
		}
		program["pure_calls"] = calls
		g["input_parameters"] = []any{map[string]any{"name": "input0", "type": "string"}, map[string]any{"name": "input1", "type": "string"}}
		g["call_closure"] = headerCallClosure(definitions)
		delivery["inputs"] = []any{map[string]any{"port": "input0", "entity_id": textEntity, "value": inputs[0]}, map[string]any{"port": "input1", "entity_id": textEntity, "value": inputs[1]}}
		values := []any{}
		for _, f := range headerResultContract(s).Fields {
			values = append(values, map[string]any{"id": f.ID, "name": f.Name, "value": expected.(map[string]any)[f.Name]})
		}
		delivery["actual_fields"] = values
	} else {
		delivery["input"] = inputs[0]
	}
	m := map[string]any{"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "PASS", "manifest_digest": id.ManifestDigest, "cases_digest": id.CasesDigest, "result": map[string]any{"schema": "gooo/workspace-body-execution/v1", "program": program, "execution_source_sha256": sh,
		"composition": map[string]any{"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "gooo_source": text, "original_source_sha256": sh, "selected_source_sha256": sh, "source": composed, "generated_sha256": contentDigest([]byte(composed)), "driver_source": driver, "driver_sha256": contentDigest([]byte(driver)), "plan": plan, "steps": []any{map[string]any{"input_source_sha256": sh, "generation": map[string]any{"source": generated, "report": g}}}},
		"runtime":     map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": id.CompilerRevision, "go_version": "go version go1.27.2 linux/amd64", "generated_sha256": contentDigest([]byte(composed)), "driver_sha256": contentDigest([]byte(driver)), "typed_plan_sha256": plan["typed_plan_sha256"], "original_source_sha256": sh, "selected_source_sha256": sh, "model_calls": 0, "finite_passed": 1, "finite_total": 1, "projection_replayed": true, "runtime_replayed": true, "traces": []any{map[string]any{"case_index": 0, "deliveries": []any{delivery}}}},
	}}
	return headerTestClone(t, m), s, id
}

func TestHeaderInspectorRejectsABIAndSourceSubstitution(t *testing.T) {
	for _, original := range []suite{{Package: headerPortPackage, Activity: headerNameActivity}, {Package: headerPortPackage, Activity: headerValueActivity}, {Package: headerReportPackage, Activity: headerReportActivity}} {
		t.Run(original.Activity, func(t *testing.T) {
			base, s, id := headerReceiptFixture(t, original)
			first, err := inspect(mustJSON(t, base), s, false, id)
			if err != nil {
				t.Fatal("positive synthetic inspector fixture", err)
			}
			replayed := headerTestClone(t, base)
			replayed["replayed_from_sha256"] = contentDigest(mustJSON(t, base))
			headerTestMap(t, replayed, "result")["replay"] = map[string]any{"model_calls": 0}
			second, err := inspect(mustJSON(t, replayed), s, true, id)
			if err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(first, second, contentDigest(mustJSON(t, base))); err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(first, second, contentDigest([]byte("foreign raw saved receipt"))); err == nil {
				t.Fatal("accepted foreign saved parent")
			}
			mutations := map[string]func(map[string]any){
				"source_owner": func(m map[string]any) {
					headerTestMap(t, m, "result", "program", "workspace_image")["packages"].([]any)[0].(map[string]any)["path"] = "foreign"
				},
				"callee_export": func(m map[string]any) {
					headerTestMap(t, m, "result", "program", "workspace_image")["packages"].([]any)[0].(map[string]any)["exports"].([]any)[0].(map[string]any)["output_type"] = textEntity
				},
				"tuple_shape": func(m map[string]any) {
					headerTestMap(t, m, "result", "composition", "plan")["activities"].([]any)[0].(map[string]any)["input_entity_id"] = integerEntity
				},
				"coherent_body_mutant": func(m map[string]any) {
					p := headerTestMap(t, m, "result", "program")
					text := p["lowered_gooo_source"].(string)
					changed := strings.Replace(text, "let size = int64(len(input))", "let size = int64(len(input)) + 1", 1)
					if changed == text {
						t.Fatal("mutation did not change body")
					}
					digest := contentDigest([]byte(changed))
					p["lowered_gooo_source"] = changed
					headerTestMap(t, m, "result")["execution_source_sha256"] = digest
					c := headerTestMap(t, m, "result", "composition")
					c["gooo_source"], c["original_source_sha256"], c["selected_source_sha256"] = changed, digest, digest
					c["steps"].([]any)[0].(map[string]any)["input_source_sha256"] = digest
					headerTestGeneration(t, m)["report"].(map[string]any)["source_digest"] = digest
					rt := headerTestMap(t, m, "result", "runtime")
					rt["original_source_sha256"], rt["selected_source_sha256"] = digest, digest
				},
				"coherent_native_abi": func(m map[string]any) {
					g := headerTestGeneration(t, m)
					src := g["source"].(string)
					src = strings.Replace(src, "input string", "input int64", 1)
					if s.Package == headerReportPackage {
						src = strings.Replace(src, "input0 string", "input0 int64", 1)
					}
					g["source"] = src
					report := g["report"].(map[string]any)
					report["generated_digest"], report["replay_digest"] = contentDigest([]byte(src)), contentDigest([]byte(src))
				},
			}
			if s.Package == headerReportPackage {
				mutations["coherent_native_record_type"] = func(m map[string]any) {
					g := headerTestGeneration(t, m)
					src := strings.Replace(g["source"].(string), " string `json:", " int64 `json:", 1)
					g["source"] = src
					report := g["report"].(map[string]any)
					report["generated_digest"], report["replay_digest"] = contentDigest([]byte(src)), contentDigest([]byte(src))
				}
				mutations["field_kind"] = func(m map[string]any) {
					headerTestMap(t, m, "result", "composition", "plan")["record_types"].([]any)[0].(map[string]any)["fields"].([]any)[2].(map[string]any)["type_id"] = "urn:gooo:type:integer"
				}
				mutations["callee_ordinal"] = func(m map[string]any) {
					headerTestGeneration(t, m)["report"].(map[string]any)["call_closure"].(map[string]any)["edges"].([]any)[0].(map[string]any)["ordinal"] = json.Number("0")
				}
				mutations["field_order"] = func(m map[string]any) {
					d := headerTestMap(t, m, "result", "runtime")["traces"].([]any)[0].(map[string]any)["deliveries"].([]any)[0].(map[string]any)
					fields := d["actual_fields"].([]any)
					fields[0], fields[1] = fields[1], fields[0]
				}
				mutations["boolean_as_integer"] = func(m map[string]any) {
					d := headerTestMap(t, m, "result", "runtime")["traces"].([]any)[0].(map[string]any)["deliveries"].([]any)[0].(map[string]any)
					d["actual"].(map[string]any)["name_valid"] = json.Number("1")
				}
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					m := headerTestClone(t, base)
					mutate(m)
					if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
						t.Fatal("accepted substituted header evidence")
					}
				})
			}
		})
	}
}

func TestHeaderContractScopeAndRequiredFieldTypes(t *testing.T) {
	for _, s := range []suite{{Package: headerPortPackage, Activity: "Other"}, {Package: headerReportPackage, Activity: headerNameActivity}, {Package: "go/net/http", Activity: "StatusText"}} {
		if isHeaderSuite(s) || headerResultContract(s).Entity != "" {
			t.Fatal("old/unknown activity entered header contract")
		}
	}
	c := headerResultContract(suite{Package: headerReportPackage, Activity: headerReportActivity})
	if len(c.Fields) != 12 || c.Entity != headerFactsEntity {
		t.Fatal("required mixed12-field contract")
	}
	for _, s := range []suite{{Package: headerPortPackage, Activity: headerNameActivity}, {Package: headerPortPackage, Activity: headerValueActivity}, {Package: headerReportPackage, Activity: headerReportActivity}} {
		c, err := contractFor(s)
		if err != nil || len(c.InputEntities) == 0 {
			t.Fatal("missing header native dispatch", err)
		}
	}
}
