package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// These fixtures are synthetic checker inputs. They do not represent compiler,
// native, Wasm or browser executions and contain no borrowed runtime receipt.
func fixture(t *testing.T, rows []row) ([]byte, identity) {
	t.Helper()
	inputValues := make([]map[string]string, len(rows))
	for i, r := range rows {
		inputValues[i] = map[string]string{appPackage + ":ParseCanonical": r.Text}
	}
	inputRaw := mustJSON(t, map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": inputValues})
	id, err := snapshotIdentity("../..", inputRaw)
	if err != nil {
		t.Fatal(err)
	}
	var packages []any
	for _, pkgPath := range []string{quotePackage, uuidPackage, appPackage} {
		var sources, exports []any
		for name, source := range id.Sources {
			if source.Package == pkgPath {
				sources = append(sources, map[string]any{"filename": name, "source_digest": source.Digest})
			}
		}
		for _, d := range id.Declarations {
			if d.Package == pkgPath {
				exports = append(exports, map[string]any{"name": d.Name, "kind": "activity", "id": d.ID, "input_types": []string{entityForType(d.Input)}, "output_type": entityForType(d.Output)})
			}
		}
		packages = append(packages, map[string]any{"path": pkgPath, "sources": sources, "exports": exports})
	}
	participants := map[string]bool{}
	for _, site := range id.Calls {
		participants[site.Caller] = true
		participants[site.Callee] = true
	}
	var activities []any
	for key := range participants {
		d := id.Declarations[key]
		activities = append(activities, map[string]any{"activity": map[string]string{"package_path": d.Package, "activity": d.Name, "lowered_name": d.Lowered}, "activity_id": namespaceID(d.Namespace, d.Name), "lowered_activity_id": namespaceID("gooo_workspace", d.Lowered), "source": d.Source, "source_body_sha256": digest([]byte(d.Body))})
	}
	entry := map[string]string{"package_path": appPackage, "activity": "ParseCanonical", "lowered_name": id.Declarations[appPackage+":ParseCanonical"].Lowered}
	source, generated, driver, step := "synthetic lowered source", syntheticABI(), "synthetic driver", "synthetic generated step"
	process := map[string]any{"started": true, "completed": true, "canceled": false, "timed_out": false, "diagnostics_bytes": 0, "output_truncated": false, "exit_code": 0, "stdout_sha256": digest(nil), "stderr_sha256": digest(nil)}
	var traces []any
	for i, r := range rows {
		want := reference(r)
		values := []any{want.Parsed, want.CanonicalUUID, want.Error, want.InvalidLength}
		var fields []any
		for j, name := range resultNames {
			fields = append(fields, map[string]any{"id": resultEntity + "/" + resultIDs[j], "name": name, "value": values[j]})
		}
		traces = append(traces, map[string]any{"case_index": i, "deliveries": []any{map[string]any{"activity_id": activityID, "input": r.Text, "actual": want, "actual_fields": fields}}})
	}
	plan := syntheticPlan(entry["lowered_name"])
	receipt := map[string]any{
		"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest": manifestName, "manifest_digest": id.Manifest, "inputs_digest": id.Inputs,
		"result": map[string]any{
			"schema": "gooo/workspace-body-execution/v1", "execution_source_sha256": digest([]byte(source)),
			"program": map[string]any{"schema": "gooo/workspace-body-program/v1", "entry": entry, "activities": []any{entry}, "lowered_gooo_source": source,
				"workspace_image": map[string]any{"schema": "gooo/package-runtime-image/v1", "init_order": []string{quotePackage, uuidPackage, appPackage}, "packages": packages, "entry": map[string]any{"package_path": appPackage, "source": "ports/google/uuid/text/parse_canonical.gooo", "activity": "ParseCanonical", "inputs": []string{"Text"}, "output": "ParseCanonicalResult"}},
				"pure_calls":      map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": id.Calls}},
			"composition": map[string]any{"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "original_source_sha256": digest([]byte(source)), "selected_source_sha256": digest([]byte(source)), "gooo_source": source, "source": generated, "driver_source": driver, "generated_sha256": digest([]byte(generated)), "driver_sha256": digest([]byte(driver)), "plan": plan,
				"steps": []any{map[string]any{"input_source_sha256": digest([]byte(source)), "generation": map[string]any{"source": step, "report": map[string]any{"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity": entry["lowered_name"], "activity_id": activityID, "input_type": "string", "output_type": "ParseCanonicalResult", "compiler_source_sha": compilerRevision, "source_digest": digest([]byte(source)), "generated_digest": digest([]byte(step)), "replay_digest": digest([]byte(step)), "typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0}}}}},
			"runtime": map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": compilerRevision, "producer_module": map[string]string{"path": "github.com/kimjooyoon/meta-ontology-go", "version": "v0.0.0-20261011201109-83928fa45382"}, "go_version": "go version " + goVersion + " " + runtime.GOOS + "/" + runtime.GOARCH, "go_tool_sha256": id.GoTool, "go_tool_selection": "explicit", "generated_sha256": digest([]byte(generated)), "driver_sha256": digest([]byte(driver)), "original_source_sha256": digest([]byte(source)), "selected_source_sha256": digest([]byte(source)), "executable_sha256": digest([]byte("synthetic executable")), "runtime_suite_sha256": digest([]byte("synthetic suite")), "typed_plan_sha256": plan["typed_plan_sha256"], "model_calls": 0, "finite_total": 0, "finite_passed": 0, "projection_replayed": true, "runtime_replayed": true, "toolchain": process, "build": process, "runs": []any{process, process}, "traces": traces},
		},
	}
	return mustJSON(t, receipt), id
}

func syntheticABI() string {
	name := "GoooRecord" + strings.TrimPrefix(digest([]byte(resultEntity)), "sha256:")
	var out strings.Builder
	fmt.Fprintf(&out, "package main\ntype %s struct {\n", name)
	for i, field := range resultNames {
		kind := "string"
		if i == 0 || i == 3 {
			kind = "bool"
		}
		fmt.Fprintf(&out, "GoooField%s %s `json:%q`\n", strings.TrimPrefix(digest([]byte(resultEntity+"/"+resultIDs[i])), "sha256:"), kind, field)
	}
	fmt.Fprintf(&out, "}\nfunc GoooComposedActivity0(input string) %s { return %s{} }\n", name, name)
	return out.String()
}
func syntheticPlan(lowered string) map[string]any {
	var records []any
	for _, entity := range []string{resultEntity, "gooo://github.com/google/uuid/validation-result"} {
		names, ids, kinds := resultNames, resultIDs, []string{"boolean", "string", "string", "boolean"}
		name := "ParseCanonicalResult"
		if entity != resultEntity {
			name = "ValidationResult"
			names = []string{"valid", "error", "invalid_length"}
			ids = []string{"valid", "error", "invalid-length"}
			kinds = []string{"boolean", "string", "boolean"}
		}
		var fields []any
		for i, field := range names {
			fieldID := entity + "/" + ids[i]
			fields = append(fields, map[string]string{"name": field, "id": fieldID, "type_id": "urn:gooo:type:" + kinds[i], "go_name": "GoooField" + strings.TrimPrefix(digest([]byte(fieldID)), "sha256:")})
		}
		records = append(records, map[string]any{"name": name, "id": entity, "go_name": "GoooRecord" + strings.TrimPrefix(digest([]byte(entity)), "sha256:"), "fields": fields})
	}
	return map[string]any{"schema": "gooo/body-composition-plan/v1", "entry_activity": lowered, "typed_plan_sha256": digest([]byte("synthetic plan")), "semantic_fingerprint": strings.TrimPrefix(digest([]byte("synthetic fingerprint")), "sha256:"), "activities": []any{map[string]any{"name": lowered, "id": activityID, "input_type": "Text", "output_type": "ParseCanonicalResult", "input_entity_id": textEntity, "output_entity_id": resultEntity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}}, "edges": []any{}, "record_types": records}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func mutate(t *testing.T, raw []byte, f func(map[string]any)) []byte {
	t.Helper()
	v, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	e := object(v)
	f(e)
	return mustJSON(t, e)
}
func delivery(e map[string]any) map[string]any {
	return object(array(object(array(object(object(e["result"])["runtime"])["traces"])[0])["deliveries"])[0])
}
func addReplay(t *testing.T, raw []byte, parent string) []byte {
	return mutate(t, raw, func(e map[string]any) {
		e["replayed_from_sha256"] = parent
		object(e["result"])["replay"] = map[string]any{"schema": "gooo/workspace-body-replay/v1", "prior_result_sha256": digest([]byte("unverified synthetic typed result")), "body_fills_replayed": 0, "model_calls": 0, "scope": "synthetic saved replay"}
	})
}

func TestIndependentOraclePreservesPartialReturnedUUID(t *testing.T) {
	for _, test := range []struct {
		input string
		want  report
	}{
		{"00112233445566778899aabbccddee0g", report{false, "00112233-4455-6677-8899-aabbccddeeff", "invalid UUID format", false}},
		{"00112233-4455-6677-8899-aabbccddee0g", report{false, "00112233-4455-6677-8899-aabbccddee00", "invalid UUID format", false}},
		{"g1112233445566778899aabbccddeeff", report{false, "f1000000-0000-0000-0000-000000000000", "invalid UUID format", false}},
		{"!00112233-4455-6677-8899-aabbccddeeff?", report{true, "00112233-4455-6677-8899-aabbccddeeff", "", false}},
		{"", report{false, "00000000-0000-0000-0000-000000000000", "invalid UUID length: 0", true}},
	} {
		if got := reference(row{test.input}); got != test.want {
			t.Fatalf("input %q: %#v, want %#v", test.input, got, test.want)
		}
	}
}

func TestReceiptRejectsCoordinatedOracleAndProvenanceTampering(t *testing.T) {
	rows := []row{{"00112233445566778899aabbccddee0g"}, {"[00112233-4455-6677-8899-aabbccddeeff]"}}
	raw, id := fixture(t, rows)
	if _, err := inspect(raw, rows, id, ""); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"input normalized":     func(e map[string]any) { delivery(e)["input"] = rows[0].Text + " " },
		"expectation injected": func(e map[string]any) { delivery(e)["expected"] = delivery(e)["actual"] },
		"unary input replaced by array": func(e map[string]any) {
			d := delivery(e)
			delete(d, "input")
			d["inputs"] = []any{map[string]any{"value": rows[0].Text}}
		},
		"field order": func(e map[string]any) { f := array(delivery(e)["actual_fields"]); f[0], f[1] = f[1], f[0] },
		"source digest": func(e map[string]any) {
			p := object(object(e["result"])["program"])
			s := object(array(object(array(object(p["workspace_image"])["packages"])[0])["sources"])[0])
			s["source_digest"] = digest([]byte("wrong"))
		},
		"call offset": func(e map[string]any) {
			calls := object(object(object(e["result"])["program"])["pure_calls"])
			object(array(calls["sites"])[0])["start_byte"] = 0
		},
		"helper source body": func(e map[string]any) {
			calls := object(object(object(e["result"])["program"])["pure_calls"])
			object(array(calls["activities"])[0])["source_body_sha256"] = digest([]byte("wrong"))
		},
		"producer pin":          func(e map[string]any) { object(object(e["result"])["runtime"])["producer_source_sha"] = "other" },
		"ambient Go":            func(e map[string]any) { object(object(e["result"])["runtime"])["go_tool_selection"] = "ambient" },
		"missing model counter": func(e map[string]any) { delete(object(object(e["result"])["runtime"]), "model_calls") },
		"nonzero finite total":  func(e map[string]any) { object(object(e["result"])["runtime"])["finite_total"] = 1 },
		"failed second run": func(e map[string]any) {
			object(array(object(object(e["result"])["runtime"])["runs"])[1])["completed"] = false
		},
		"generation pin": func(e map[string]any) {
			c := object(object(e["result"])["composition"])
			object(object(object(array(c["steps"])[0])["generation"])["report"])["compiler_source_sha"] = "other"
		},
		"generation typecheck": func(e map[string]any) {
			c := object(object(e["result"])["composition"])
			object(object(object(array(c["steps"])[0])["generation"])["report"])["typecheck_passed"] = false
		},
		"plan/runtime mismatch": func(e map[string]any) {
			object(object(e["result"])["runtime"])["typed_plan_sha256"] = digest([]byte("other"))
		},
		"record type": func(e map[string]any) {
			plan := object(object(object(e["result"])["composition"])["plan"])
			object(array(object(array(plan["record_types"])[0])["fields"])[0])["type_id"] = "urn:gooo:type:string"
		},
	}
	for field, wrong := range map[string]any{"parsed": true, "canonical_uuid": "00000000-0000-0000-0000-000000000000", "error": "", "invalid_length": true} {
		mutations["coordinated "+field] = func(e map[string]any) {
			d := delivery(e)
			object(d["actual"])[field] = wrong
			for _, value := range array(d["actual_fields"]) {
				f := object(value)
				if f["name"] == field {
					f["value"] = wrong
				}
			}
		}
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := inspect(mutate(t, raw, change), rows, id, ""); err == nil {
				t.Fatal("accepted tampered evidence")
			}
		})
	}
}

func TestSavedReplayChecksOuterParentAndFullProgramWithoutClaimingInnerHash(t *testing.T) {
	rows := []row{{"00112233445566778899aabbccddee0g"}}
	raw, id := fixture(t, rows)
	original, err := inspect(raw, rows, id, "")
	if err != nil {
		t.Fatal(err)
	}
	parent := digest(raw)
	replay := addReplay(t, raw, parent)
	repeated, err := inspect(replay, rows, id, parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareReplay(original, repeated); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(replay, rows, id, digest([]byte("unrelated"))); err == nil {
		t.Fatal("accepted unrelated parent")
	}
	changed := mutate(t, replay, func(e map[string]any) { object(object(e["result"])["program"])["scope"] = "changed program metadata" })
	repeated, err = inspect(changed, rows, id, parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareReplay(original, repeated); err == nil {
		t.Fatal("accepted changed saved program")
	}
	changed = mutate(t, replay, func(e map[string]any) {
		object(object(e["result"])["replay"])["prior_result_sha256"] = digest([]byte("different unverified typed result"))
	})
	if _, err := inspect(changed, rows, id, parent); err != nil {
		t.Fatal("inner digest is deliberately outside this checker scope", err)
	}
	scope := checkedReport(rows, id, original, raw, replay)["verification_scope"].(map[string]string)
	if scope["prior_typed_result_sha256"] != "NOT_CHECKED" || scope["typed_plan_digest_recomputation"] != "NOT_CHECKED" {
		t.Fatal("overclaimed verification scope")
	}
	if _, ok := checkedReport(rows, id, original, raw, replay)["prior_result_sha256"]; ok {
		t.Fatal("exported unverified digest as a computed prior Result hash")
	}
}

func TestInputByteBoundaryAndDecoderReplacement(t *testing.T) {
	makeInput := func(v any) []byte {
		return mustJSON(t, map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": []any{map[string]any{appPackage + ":ParseCanonical": v}}})
	}
	for _, text := range []string{strings.Repeat("a", 1024), strings.Repeat("🙂", 256), "\ufeffx"} {
		if _, err := readInputs(makeInput(text)); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []any{strings.Repeat("a", 1025), strings.Repeat("가", 342), nil, true, 1, []any{}, map[string]any{}} {
		if _, err := readInputs(makeInput(v)); err == nil {
			t.Fatal("accepted unsupported input", v)
		}
	}
	raw := []byte(`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"go/github.com/google/uuid/text:ParseCanonical":"\ud800"}]}`)
	rows, err := readInputs(raw)
	if err != nil || !reflect.DeepEqual(rows, []row{{"\ufffd"}}) {
		t.Fatal("decoder replacement binding", rows, err)
	}
	for _, raw := range [][]byte{[]byte(`{"x":0,"x":1}`), []byte(`{"x":0,"\u0078":1}`), []byte(`{} {}`), {0xff}, []byte(strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66))} {
		if _, err := decode(raw); err == nil {
			t.Fatal("accepted unsupported JSON")
		}
	}
}

func TestGeneratedABIRejectsFieldAndArgumentChanges(t *testing.T) {
	source := syntheticABI()
	if err := checkABI(source); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{strings.Replace(source, "input string", "input0 string", 1), strings.Replace(source, "input string", "input int64", 1), strings.Replace(source, "json:\"parsed\"", "json:\"parsed,omitempty\"", 1), strings.Replace(source, " bool ", " string ", 1)} {
		if err := checkABI(changed); err == nil {
			t.Fatal("accepted changed ABI")
		}
	}
}
