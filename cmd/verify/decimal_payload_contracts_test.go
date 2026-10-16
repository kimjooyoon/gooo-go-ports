package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Synthetic receipts test the actual public inspector. Their values come from
// the separate Go oracle; their metadata is built from current source bytes.
// No synthetic generated source is executed or presented as native evidence.
func decimalPayloadReceiptFixture(t *testing.T, original suite, row int) (map[string]any, suite, expectedIdentity) {
	t.Helper()
	s := original
	s.Cases = []testCase{original.Cases[row]}
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := frozen.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	identity := expectedIdentity{CompilerRevision: "83928fa4538245abe33b08549da47f8cbdc50ed2", GoVersion: "go1.27.2", ManifestDigest: contentDigest(mustJSON(t, selected.Manifest)), CasesDigest: contentDigest(mustJSON(t, s.Cases)), Sources: selected.Identities, SourceBytes: selected.Sources}
	consumer := s.Package == payloadAdmissionPackage
	contract, _ := contractFor(s)
	parserSuite := suite{Package: decimalPackage, Activity: decimalActivity}
	parserContract, _ := contractFor(parserSuite)
	parserBodies, err := decimalPayloadBodies(selected.Sources[parserContract.Source])
	if err != nil {
		t.Fatal(err)
	}
	parser := parserBodies[decimalActivity]
	lower := "GoooPackage0ActivityParseIntDecimal64"
	names, kinds, output := []string{"Text"}, []string{"string"}, "DecimalInt64Result"
	rootBody := parser.Body
	loweredSource := fmt.Sprintf("activity %s(Text) -> DecimalInt64Result id %q computes %q\n", lower, parserContract.ID, parser.Body)
	var pure any
	if consumer {
		bodyMap, err := decimalPayloadBodies(selected.Sources[contract.Source])
		if err != nil {
			t.Fatal(err)
		}
		body := bodyMap[s.Activity].Body
		lower = "GoooPackage1ActivityReport"
		names = []string{"Text", "Integer", "Integer", "Integer", "Boolean", "Boolean", "Boolean"}
		kinds = []string{"string", "int64", "int64", "int64", "bool", "bool", "bool"}
		output = "PayloadAdmissionFacts"
		rootBody = strings.Replace(body, "decimal.ParseIntDecimal64", "GoooPackage0ActivityParseIntDecimal64", 1)
		loweredSource += fmt.Sprintf("activity %s(%s) -> %s id %q computes %q\n", lower, strings.Join(names, ", "), output, contract.ID, rootBody)
		identities := []any{}
		for _, item := range []struct{ pkg, name, lower, namespace, source, body string }{
			{s.Package, s.Activity, lower, "gooo_ports_examples_payload_admission", contract.Source, body},
			{decimalPackage, decimalActivity, "GoooPackage0ActivityParseIntDecimal64", "gooo_ports_strconv", parserContract.Source, parser.Body},
		} {
			identities = append(identities, map[string]any{"activity": map[string]any{"package_path": item.pkg, "activity": item.name, "lowered_name": item.lower}, "activity_id": decimalPayloadDerivedID(item.namespace, item.name), "lowered_activity_id": decimalPayloadDerivedID("gooo_workspace", item.lower), "source": item.source, "source_body_sha256": contentDigest([]byte(item.body))})
		}
		at := strings.Index(body, "decimal.ParseIntDecimal64(input0)")
		pure = map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": identities, "sites": []any{map[string]any{"caller": s.Package + ":" + s.Activity, "callee": decimalPackage + ":" + decimalActivity, "surface": "computes", "start_byte": at, "end_byte": at + len("decimal.ParseIntDecimal64")}}}
	}
	root := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": lower}
	imagePackages := []any{}
	initOrder := []string{}
	for _, pkg := range selected.Manifest.Packages {
		sourceRows := []any{}
		for _, file := range pkg.Sources {
			sourceRows = append(sourceRows, map[string]any{"filename": file, "source_digest": selected.Identities[file].Digest})
		}
		which := parserSuite
		if pkg.Path == s.Package {
			which = s
		}
		exportContract, _ := contractFor(which)
		imagePackages = append(imagePackages, map[string]any{"path": pkg.Path, "sources": sourceRows, "exports": []any{map[string]any{"name": which.Activity, "kind": "activity", "id": exportContract.ID, "input_types": exportContract.InputEntities, "output_type": whitespaceResultContract(which).Entity}}})
		initOrder = append(initOrder, pkg.Path)
	}
	image := map[string]any{"schema": "gooo/package-runtime-image/v1", "entry": map[string]any{"package_path": s.Package, "source": contract.Source, "activity": s.Activity, "inputs": names, "output": output}, "init_order": initOrder, "packages": imagePackages}
	sourceSHA := contentDigest([]byte(loweredSource))
	generated := "package main\n// synthetic inspector consistency fixture only\n"
	generatedSHA := contentDigest([]byte(generated))
	driver := "synthetic driver bytes"
	records := decimalPayloadRecords(consumer)
	activity := map[string]any{"name": lower, "id": contract.ID, "input_type": "Text", "output_type": output, "input_entity_id": textEntity, "output_entity_id": whitespaceResultContract(s).Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	plan := map[string]any{"schema": "gooo/body-composition-plan/v1", "typed_plan_sha256": contentDigest([]byte("synthetic plan")), "activities": []any{activity}, "record_types": records, "edges": []any{}}
	inputType := kinds[0]
	generation := map[string]any{"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity": lower, "activity_id": contract.ID, "compiler_source_sha": identity.CompilerRevision, "source_digest": sourceSHA, "program_digest": contentDigest([]byte(rootBody)), "generated_digest": generatedSHA, "replay_digest": generatedSHA, "input_type": inputType, "output_type": output, "record_types": records, "typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0}
	if consumer {
		ports, parameters := []any{}, []any{}
		for i, name := range names {
			ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "type": name, "entity_id": contract.InputEntities[i], "from": -1})
			parameters = append(parameters, map[string]any{"name": fmt.Sprintf("input%d", i), "type": kinds[i]})
		}
		activity["inputs"] = ports
		plan["entry_activity"] = lower
		generation["input_type"] = "(" + strings.Join(kinds, ",") + ")"
		generation["input_parameters"] = parameters
		generation["call_closure"] = map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": []any{map[string]any{"name": "GoooPackage0ActivityParseIntDecimal64", "activity_id": parserContract.ID, "program_sha256": contentDigest([]byte(parser.Body))}}, "edges": []any{map[string]any{"caller_id": contract.ID, "callee_id": parserContract.ID, "ordinal": 0}}, "max_calls_per_invocation": 1}
	}
	expected := s.Cases[0].Expected.(map[string]any)
	fields := []any{}
	for _, field := range whitespaceResultContract(s).Fields {
		fields = append(fields, map[string]any{"id": field.ID, "name": field.Name, "value": expected[field.Name]})
	}
	delivery := map[string]any{"activity_id": contract.ID, "actual": expected, "expected": expected, "actual_fields": fields, "passed": true}
	if consumer {
		ports := []any{}
		for i, value := range s.Cases[0].Inputs {
			ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "entity_id": contract.InputEntities[i], "value": value})
		}
		delivery["inputs"] = ports
	} else {
		delivery["input"] = s.Cases[0].Inputs[0]
	}
	receipt := map[string]any{"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "PASS", "manifest_digest": identity.ManifestDigest, "cases_digest": identity.CasesDigest, "result": map[string]any{
		"execution_source_sha256": sourceSHA,
		"program":                 map[string]any{"workspace_image": image, "entry": root, "activities": []any{root}, "lowered_gooo_source": loweredSource, "pure_calls": pure},
		"composition":             map[string]any{"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "gooo_source": loweredSource, "original_source_sha256": sourceSHA, "selected_source_sha256": sourceSHA, "generated_sha256": generatedSHA, "source": generated, "driver_source": driver, "driver_sha256": contentDigest([]byte(driver)), "plan": plan, "steps": []any{map[string]any{"input_source_sha256": sourceSHA, "generation": map[string]any{"source": generated, "report": generation}}}},
		"runtime":                 map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": identity.CompilerRevision, "go_version": "go version go1.27.2 linux/amd64", "generated_sha256": generatedSHA, "driver_sha256": contentDigest([]byte(driver)), "typed_plan_sha256": plan["typed_plan_sha256"], "original_source_sha256": sourceSHA, "selected_source_sha256": sourceSHA, "model_calls": 0, "finite_passed": 1, "finite_total": 1, "projection_replayed": true, "runtime_replayed": true, "traces": []any{map[string]any{"case_index": 0, "deliveries": []any{delivery}}}},
	}}
	// Normalize fixture integers to the same exact json.Number domain as raw
	// receipt inspection; retain the independently computed suite as the oracle.
	return decimalPayloadTestClone(t, receipt), s, identity
}

func decimalPayloadTestClone(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	decoded, err := decode(mustJSON(t, value))
	if err != nil {
		t.Fatal(err)
	}
	return decoded.(map[string]any)
}
func decimalPayloadTestMap(t *testing.T, value map[string]any, path ...string) map[string]any {
	t.Helper()
	got, err := decimalPayloadReceiptObject(value, path...)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func decimalPayloadTestDelivery(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	runtime := decimalPayloadTestMap(t, receipt, "result", "runtime")
	return runtime["traces"].([]any)[0].(map[string]any)["deliveries"].([]any)[0].(map[string]any)
}
func decimalPayloadTestGeneration(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	c := decimalPayloadTestMap(t, receipt, "result", "composition")
	return c["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)["report"].(map[string]any)
}

func TestDecimalPayloadInspectorBindsTwoCurrentSourceClosures(t *testing.T) {
	observations, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range observations.Suites {
		t.Run(original.Package, func(t *testing.T) {
			baseline, s, id := decimalPayloadReceiptFixture(t, original, 0)
			first, err := inspect(mustJSON(t, baseline), s, false, id)
			if err != nil {
				t.Fatal("positive synthetic fixture", err)
			}
			replay := decimalPayloadTestClone(t, baseline)
			replay["replayed_from_sha256"] = contentDigest(mustJSON(t, baseline))
			decimalPayloadTestMap(t, replay, "result")["replay"] = map[string]any{"model_calls": 0}
			checkedReplay, err := inspect(mustJSON(t, replay), s, true, id)
			if err != nil {
				t.Fatal("positive saved fixture", err)
			}
			if err = inspectReplayBinding(first, checkedReplay, contentDigest(mustJSON(t, baseline))); err != nil {
				t.Fatal(err)
			}
			if err = inspectReplayBinding(first, checkedReplay, contentDigest([]byte("foreign parent"))); err == nil {
				t.Fatal("foreign raw parent accepted")
			}
			missing := id
			missing.SourceBytes = nil
			if _, err = inspect(mustJSON(t, baseline), s, false, missing); err == nil {
				t.Fatal("missing current source bytes accepted")
			}
			tests := map[string]func(map[string]any){
				"image_entry": func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "program", "workspace_image", "entry")["source"] = "foreign.gooo"
				},
				"init_order": func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "program", "workspace_image")["init_order"] = []any{}
				},
				"lowered_entry": func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "program", "entry")["lowered_name"] = "GoooPackage9ActivityFake"
				},
				"lowered_body": func(m map[string]any) {
					p := decimalPayloadTestMap(t, m, "result", "program")
					p["lowered_gooo_source"] = strings.Replace(p["lowered_gooo_source"].(string), "let length", "let changed_length", 1)
				},
				"generated_body_hash": func(m map[string]any) {
					decimalPayloadTestGeneration(t, m)["program_digest"] = contentDigest([]byte("wrong body"))
				},
				"record_field_type": func(m map[string]any) {
					r := decimalPayloadTestGeneration(t, m)["record_types"].([]any)[0].(map[string]any)
					r["fields"].([]any)[0].(map[string]any)["type_id"] = "urn:gooo:type:boolean"
				},
				"driver": func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "composition")["driver_source"] = "changed"
				},
				"plan_binding": func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "runtime")["typed_plan_sha256"] = contentDigest([]byte("wrong plan"))
				},
				"field_order": func(m map[string]any) {
					d := decimalPayloadTestDelivery(t, m)
					f := d["actual_fields"].([]any)
					f[0], f[1] = f[1], f[0]
				},
				"extra_field": func(m map[string]any) {
					d := decimalPayloadTestDelivery(t, m)
					d["actual_fields"] = append(d["actual_fields"].([]any), map[string]any{"id": "extra", "name": "extra", "value": 0})
				},
				"missing_field": func(m map[string]any) {
					d := decimalPayloadTestDelivery(t, m)
					d["actual_fields"] = d["actual_fields"].([]any)[1:]
				},
			}
			if s.Package == payloadAdmissionPackage {
				tests["callee_source_body"] = func(m map[string]any) {
					p := decimalPayloadTestMap(t, m, "result", "program", "pure_calls")
					p["activities"].([]any)[1].(map[string]any)["source_body_sha256"] = contentDigest([]byte("wrong parser"))
				}
				tests["callee_call_site"] = func(m map[string]any) {
					p := decimalPayloadTestMap(t, m, "result", "program", "pure_calls")
					p["sites"].([]any)[0].(map[string]any)["start_byte"] = json.Number("1")
				}
				tests["compiled_callee"] = func(m map[string]any) {
					cl := decimalPayloadTestMap(t, decimalPayloadTestGeneration(t, m), "call_closure")
					cl["activities"].([]any)[0].(map[string]any)["program_sha256"] = contentDigest([]byte("wrong compiled parser"))
				}
				tests["callee_edge"] = func(m map[string]any) {
					cl := decimalPayloadTestMap(t, decimalPayloadTestGeneration(t, m), "call_closure")
					cl["edges"].([]any)[0].(map[string]any)["callee_id"] = "foreign"
				}
				tests["callee_export"] = func(m map[string]any) {
					image := decimalPayloadTestMap(t, m, "result", "program", "workspace_image")
					image["packages"].([]any)[0].(map[string]any)["exports"].([]any)[0].(map[string]any)["output_type"] = "foreign"
				}
				tests["input_parameter_kind"] = func(m map[string]any) {
					decimalPayloadTestGeneration(t, m)["input_parameters"].([]any)[4].(map[string]any)["type"] = "int64"
				}
				tests["typed_port_kind"] = func(m map[string]any) {
					p := decimalPayloadTestMap(t, m, "result", "composition", "plan")
					p["activities"].([]any)[0].(map[string]any)["inputs"].([]any)[4].(map[string]any)["entity_id"] = integerEntity
				}
			} else {
				tests["unexpected_call"] = func(m map[string]any) {
					decimalPayloadTestMap(t, m, "result", "program")["pure_calls"] = map[string]any{}
				}
				tests["scalar_fields"] = func(m map[string]any) {
					decimalPayloadTestDelivery(t, m)["input_fields"] = []any{map[string]any{"id": "fake", "name": "fake", "value": 0}}
				}
			}
			for name, mutate := range tests {
				t.Run(name, func(t *testing.T) {
					m := decimalPayloadTestClone(t, baseline)
					mutate(m)
					if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
						t.Fatal("tampered receipt accepted")
					}
				})
			}
			// Every returned field has independently checked identity and value.
			for i, field := range whitespaceResultContract(s).Fields {
				for _, mode := range []string{"id", "name", "value"} {
					t.Run(fmt.Sprintf("field_%s_%s", field.Name, mode), func(t *testing.T) {
						m := decimalPayloadTestClone(t, baseline)
						v := decimalPayloadTestDelivery(t, m)["actual_fields"].([]any)[i].(map[string]any)
						v[mode] = nil
						if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
							t.Fatal("field tamper accepted")
						}
					})
				}
			}
			if s.Package == payloadAdmissionPackage {
				for i := 0; i < 7; i++ {
					t.Run(fmt.Sprintf("scalar_port_%d", i), func(t *testing.T) {
						for _, mode := range []string{"wrong_kind", "record_fields", "wrong_entity"} {
							m := decimalPayloadTestClone(t, baseline)
							port := decimalPayloadTestDelivery(t, m)["inputs"].([]any)[i].(map[string]any)
							switch mode {
							case "wrong_kind":
								if i >= 4 {
									port["value"] = json.Number("0")
								} else {
									port["value"] = false
								}
							case "record_fields":
								port["fields"] = []any{map[string]any{"id": "fake", "name": "fake", "value": 0}}
							case "wrong_entity":
								port["entity_id"] = "gooo://std/foreign"
							}
							if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
								t.Fatal("scalar input tamper accepted", mode)
							}
						}
					})
				}
			}
		})
	}
}

func TestDecimalPayloadInspectorRejectsSelfReportedWrongOracleValues(t *testing.T) {
	observations, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range observations.Suites {
		// Check all error/result kinds across the small new corpus, without
		// regenerating the historical66 suites or executing a compiler.
		seen := map[string]bool{}
		for row, item := range original.Cases {
			expected := item.Expected.(map[string]any)
			key := fmt.Sprint(expected["error_kind"], expected["parse_error_kind"], expected["reason"])
			if seen[key] {
				continue
			}
			seen[key] = true
			baseline, s, id := decimalPayloadReceiptFixture(t, original, row)
			if _, err := inspect(mustJSON(t, baseline), s, false, id); err != nil {
				t.Fatal("positive oracle class", err)
			}
			for _, field := range whitespaceResultContract(s).Fields {
				t.Run(fmt.Sprintf("%s_%d_%s", s.Activity, row, field.Name), func(t *testing.T) {
					m := decimalPayloadTestClone(t, baseline)
					d := decimalPayloadTestDelivery(t, m)
					wrong := any(nil)
					switch value := expected[field.Name].(type) {
					case int64:
						wrong = value ^ 1
					case bool:
						wrong = !value
					case string:
						wrong = value + "!"
					default:
						t.Fatalf("unexpected Go oracle kind %T", value)
					}
					d["actual"].(map[string]any)[field.Name] = wrong
					d["expected"].(map[string]any)[field.Name] = wrong
					for _, v := range d["actual_fields"].([]any) {
						f := v.(map[string]any)
						if f["name"] == field.Name {
							f["value"] = wrong
						}
					}
					if _, err := inspect(mustJSON(t, m), s, false, id); err == nil {
						t.Fatal("self-consistent wrong Go oracle value accepted")
					}
				})
			}
		}
	}
}

func TestDecimalPayloadWorkspacesIsolateAllPrior66Sources(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	baseline := frozen
	baseline.Catalog.Packages = nil
	for _, p := range frozen.Catalog.Packages {
		if p.Path != decimalPackage && p.Path != payloadAdmissionPackage && p.Path != pathCleanPackage && p.Path != pathAliasPackage {
			baseline.Catalog.Packages = append(baseline.Catalog.Packages, p)
		}
	}
	old := 0
	for key := range nativeActivityContracts() {
		parts := strings.SplitN(key, ":", 2)
		s := suite{Package: parts[0], Activity: parts[1]}
		// Keep this regression scoped to the original 66 entries. The later
		// path addition has its own regression over the preceding 68 entries.
		if isDecimalPayloadSuite(s) || isPathAliasSuite(s) || isHeaderSuite(s) || isHeaderAdditionSuite(s) || isCanonicalHeaderSuite(s) || isETagSuite(s) {
			continue
		}
		old++
		got, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(key, err)
		}
		before, err := baseline.workspaceForSuite(s)
		if err != nil {
			t.Fatal(key, err)
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatal("new parser changed original workspace", key)
		}
		if _, ok := got.Sources["ports/strconv/decimal.gooo"]; ok {
			t.Fatal("new parser leaked", key)
		}
		if _, ok := got.Sources["examples/payload-admission/main.gooo"]; ok {
			t.Fatal("new consumer leaked", key)
		}
	}
	if old != 66 {
		t.Fatal("original activity roster differs", old)
	}
	for _, s := range []suite{{Package: decimalPackage, Activity: decimalActivity}, {Package: payloadAdmissionPackage, Activity: payloadAdmissionActivity}} {
		selected, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if s.Package == payloadAdmissionPackage {
			want = 2
		}
		if len(selected.Sources) != want || len(selected.Manifest.Packages) != want {
			t.Fatal("new closure not minimal", s.Package)
		}
		// A catalog source change after freeze must fail before a compiler runs.
		changed := frozen
		changed.Sources = map[string][]byte{}
		for name, raw := range frozen.Sources {
			changed.Sources[name] = append([]byte(nil), raw...)
		}
		changed.Sources["ports/strconv/decimal.gooo"] = append(changed.Sources["ports/strconv/decimal.gooo"], '\n')
		if _, err := changed.workspaceForSuite(s); err == nil {
			t.Fatal("changed frozen parser source accepted")
		}
		extra := frozen
		extra.Catalog.Packages = append([]sourcePackage(nil), frozen.Catalog.Packages...)
		for i, p := range extra.Catalog.Packages {
			if p.Path == decimalPackage {
				extra.Catalog.Packages[i].Sources = []string{"ports/strconv/decimal.gooo", "ports/strconv/extra.gooo"}
			}
		}
		if _, err := extra.workspaceForSuite(s); err == nil {
			t.Fatal("extra parser source accepted")
		}
	}
}
