package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Synthetic receipts exercise the checker contract, not native execution.
// The separate saved pilot receipts supply the real native observations.
func fixture(t *testing.T, rows []row) ([]byte, identity) {
	t.Helper()
	id := identity{
		Manifest: digest([]byte("manifest")), Inputs: digest([]byte("inputs")), GoTool: digest([]byte("go tool")),
		Sources: map[string]sourceIdentity{
			"ports/strings/search.gooo":              {"go/strings", digest([]byte("search"))},
			"ports/strings/literal.gooo":             {"go/strings", digest([]byte("literal"))},
			"examples/literal-text-report/main.gooo": {appPackage, digest([]byte("consumer"))},
		},
		Bodies: map[string]string{appPackage + ":Report": digest([]byte("report body")), "go/strings:Index": digest([]byte("index body")), "go/strings:Count": digest([]byte("count body"))},
	}
	process := map[string]any{"started": true, "completed": true, "canceled": false, "timed_out": false, "output_truncated": false, "diagnostics_bytes": 0, "exit_code": 0, "stdout_sha256": digest(nil), "stderr_sha256": digest(nil)}
	var traces []any
	for i, row := range rows {
		want := reference(row)
		names := []string{"text", "needle", "first_byte_offset", "found", "non_overlapping_count", "text_byte_length", "needle_byte_length"}
		ids := []string{"text", "needle", "first-byte-offset", "found", "non-overlapping-count", "text-byte-length", "needle-byte-length"}
		values := []any{want.Text, want.Needle, want.FirstByteOffset, want.Found, want.NonOverlappingCount, want.TextByteLength, want.NeedleByteLength}
		var fields []any
		for j, name := range names {
			fields = append(fields, map[string]any{"id": resultEntity + "/" + ids[j], "name": name, "value": values[j]})
		}
		traces = append(traces, map[string]any{"case_index": i, "deliveries": []any{map[string]any{
			"activity_id": activityID,
			"inputs":      []any{map[string]any{"port": "input0", "entity_id": textEntity, "value": row.Text}, map[string]any{"port": "input1", "entity_id": textEntity, "value": row.Needle}},
			"actual":      want, "actual_fields": fields,
		}}})
	}
	var packages []any
	for _, path := range []string{"go/strings", appPackage} {
		var sources, exports []any
		for name, source := range id.Sources {
			if source.Package == path {
				sources = append(sources, map[string]any{"filename": name, "source_digest": source.Digest})
			}
		}
		activities := []string{"Index", "Count"}
		if path == appPackage {
			activities = []string{"Report"}
		}
		for _, name := range activities {
			id, output := "gooo://go/strings/"+name, integerEntity
			if path == appPackage {
				id, output = activityID, resultEntity
			}
			exports = append(exports, map[string]any{"name": name, "kind": "activity", "id": id, "input_types": []string{textEntity, textEntity}, "output_type": output})
		}
		packages = append(packages, map[string]any{"path": path, "sources": sources, "exports": exports})
	}
	var calls []any
	for key, body := range id.Bodies {
		parts := strings.Split(key, ":")
		source := "ports/strings/literal.gooo"
		if parts[0] == appPackage {
			source = "examples/literal-text-report/main.gooo"
		}
		calls = append(calls, map[string]any{"activity": map[string]any{"package_path": parts[0], "activity": parts[1]}, "source": source, "source_body_sha256": body})
	}
	source, generated, driver, step := "lowered source", "generated Go", "native driver", "generated step"
	plan := digest([]byte("typed plan"))
	e := map[string]any{
		"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest_digest": id.Manifest, "inputs_digest": id.Inputs,
		"result": map[string]any{
			"schema": "gooo/workspace-body-execution/v1", "execution_source_sha256": digest([]byte(source)),
			"program": map[string]any{
				"schema": "gooo/workspace-body-program/v1", "entry": map[string]any{"package_path": appPackage, "activity": "Report"}, "lowered_gooo_source": source,
				"workspace_image": map[string]any{"schema": "gooo/package-runtime-image/v1", "packages": packages},
				"pure_calls":      map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": calls, "sites": []any{map[string]any{"caller": appPackage + ":Report", "callee": "go/strings:Index", "surface": "computes"}, map[string]any{"caller": appPackage + ":Report", "callee": "go/strings:Count", "surface": "computes"}}},
			},
			"composition": map[string]any{
				"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "original_source_sha256": digest([]byte(source)), "selected_source_sha256": digest([]byte(source)), "gooo_source": source,
				"source": generated, "driver_source": driver, "generated_sha256": digest([]byte(generated)), "driver_sha256": digest([]byte(driver)),
				"plan": map[string]any{"schema": "gooo/body-composition-plan/v1", "typed_plan_sha256": plan},
				"steps": []any{map[string]any{"input_source_sha256": digest([]byte(source)), "generation": map[string]any{
					"source": step, "report": map[string]any{
						"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity_id": activityID,
						"compiler_source_sha": compilerRevision, "source_digest": digest([]byte(source)),
						"generated_digest": digest([]byte(step)), "replay_digest": digest([]byte(step)),
						"typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0,
					},
				}}},
			},
			"runtime": map[string]any{
				"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": compilerRevision,
				"producer_module": map[string]any{"path": "github.com/kimjooyoon/meta-ontology-go", "version": "v0.6.37-dev.0.20261020180505-205896a413e5"},
				"go_version":      "go version " + goVersion + " " + runtimePlatform(), "go_tool_sha256": id.GoTool, "go_tool_selection": "explicit",
				"generated_sha256": digest([]byte(generated)), "driver_sha256": digest([]byte(driver)), "original_source_sha256": digest([]byte(source)), "selected_source_sha256": digest([]byte(source)),
				"executable_sha256": digest([]byte("executable")), "runtime_suite_sha256": digest([]byte("suite")), "typed_plan_sha256": plan,
				"model_calls": 0, "finite_total": 0, "finite_passed": 0, "projection_replayed": true, "runtime_replayed": true,
				"toolchain": process, "build": process, "runs": []any{process, process}, "traces": traces,
			},
		},
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return raw, id
}

func mutateReceipt(t *testing.T, raw []byte, mutation func(map[string]any)) []byte {
	t.Helper()
	v, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	mutation(object(v))
	raw, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func firstDelivery(e map[string]any) map[string]any {
	rt := object(object(e["result"])["runtime"])
	return object(array(object(array(rt["traces"])[0])["deliveries"])[0])
}

func TestReceiptRequiresIndependentValuesAndBindings(t *testing.T) {
	rows := []row{{"가x가", "x"}, {"ababa", "aba"}}
	raw, id := fixture(t, rows)
	if _, err := inspect(raw, rows, id, replayBinding{}); err != nil {
		t.Fatal("valid synthetic fixture rejected", err)
	}
	mutations := map[string]func(map[string]any){
		"PASS label":    func(e map[string]any) { e["decision"] = "PASS" },
		"manifest":      func(e map[string]any) { e["manifest_digest"] = digest([]byte("other")) },
		"inputs file":   func(e map[string]any) { e["inputs_digest"] = digest([]byte("other")) },
		"input value":   func(e map[string]any) { object(array(firstDelivery(e)["inputs"])[0])["value"] = "x가" },
		"input entity":  func(e map[string]any) { object(array(firstDelivery(e)["inputs"])[0])["entity_id"] = integerEntity },
		"input order":   func(e map[string]any) { object(array(firstDelivery(e)["inputs"])[0])["port"] = "input1" },
		"scalar fields": func(e map[string]any) { object(array(firstDelivery(e)["inputs"])[0])["fields"] = "wrong" },
		"rune offset and correlated fields": func(e map[string]any) {
			d := firstDelivery(e)
			object(d["actual"])["first_byte_offset"] = 1
			object(array(d["actual_fields"])[2])["value"] = 1
		},
		"overlap count and correlated fields": func(e map[string]any) {
			rt := object(object(e["result"])["runtime"])
			d := object(array(object(array(rt["traces"])[1])["deliveries"])[0])
			object(d["actual"])["non_overlapping_count"] = 2
			object(array(d["actual_fields"])[4])["value"] = 2
		},
		"input-only expectation": func(e map[string]any) { firstDelivery(e)["expected"] = firstDelivery(e)["actual"] },
		"missing fields":         func(e map[string]any) { delete(firstDelivery(e), "actual_fields") },
		"field identity":         func(e map[string]any) { object(array(firstDelivery(e)["actual_fields"])[0])["id"] = "wrong" },
		"field order": func(e map[string]any) {
			f := array(firstDelivery(e)["actual_fields"])
			f[0], f[1] = f[1], f[0]
		},
		"source": func(e map[string]any) {
			p := object(object(e["result"])["program"])
			pkg := object(array(object(p["workspace_image"])["packages"])[0])
			object(array(pkg["sources"])[0])["source_digest"] = digest([]byte("wrong"))
		},
		"body source": func(e map[string]any) {
			p := object(object(e["result"])["program"])
			object(array(object(p["pure_calls"])["activities"])[0])["source_body_sha256"] = digest([]byte("wrong"))
		},
		"export output entity": func(e map[string]any) {
			p := object(object(e["result"])["program"])
			pkg := object(array(object(p["workspace_image"])["packages"])[1])
			object(array(pkg["exports"])[0])["output_type"] = textEntity
		},
		"generated source": func(e map[string]any) { object(object(e["result"])["composition"])["source"] = "wrong" },
		"producer":         func(e map[string]any) { object(object(e["result"])["runtime"])["producer_source_sha"] = "wrong" },
		"go tool": func(e map[string]any) {
			object(object(e["result"])["runtime"])["go_tool_sha256"] = digest([]byte("wrong"))
		},
		"native build":    func(e map[string]any) { object(object(object(e["result"])["runtime"])["build"])["completed"] = false },
		"native runs":     func(e map[string]any) { object(object(e["result"])["runtime"])["runs"] = []any{} },
		"finite counters": func(e map[string]any) { object(object(e["result"])["runtime"])["finite_total"] = len(rows) },
		"model calls":     func(e map[string]any) { object(object(e["result"])["runtime"])["model_calls"] = 1 },
		"case index": func(e map[string]any) {
			rt := object(object(e["result"])["runtime"])
			object(array(rt["traces"])[0])["case_index"] = 1
		},
	}
	for field, value := range map[string]any{
		"schema": "gooo/body-codegen-report/v2", "decision": "OBSERVED", "compiler_source_sha": strings.Repeat("f", 40),
		"source_digest": digest([]byte("wrong source")), "generated_digest": digest([]byte("wrong generated")),
		"replay_digest": digest([]byte("wrong replay")), "typecheck_passed": false, "deterministic_replay": false, "repository_writes": 1,
	} {
		mutations["body-generation "+field] = func(e map[string]any) {
			c := object(object(e["result"])["composition"])
			object(object(object(array(c["steps"])[0])["generation"])["report"])[field] = value
		}
	}
	mutations["body-generation source"] = func(e map[string]any) {
		c := object(object(e["result"])["composition"])
		object(object(array(c["steps"])[0])["generation"])["source"] = "other generated step"
	}
	mutations["plan schema"] = func(e map[string]any) {
		object(object(object(e["result"])["composition"])["plan"])["schema"] = "wrong"
	}
	mutations["valid composition plan digest"] = func(e map[string]any) {
		object(object(object(e["result"])["composition"])["plan"])["typed_plan_sha256"] = digest([]byte("other plan"))
	}
	mutations["valid runtime plan digest"] = func(e map[string]any) {
		object(object(e["result"])["runtime"])["typed_plan_sha256"] = digest([]byte("other plan"))
	}
	for name, mutation := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := inspect(mutateReceipt(t, raw, mutation), rows, id, replayBinding{}); err == nil {
				t.Fatal("altered receipt accepted")
			}
		})
	}
	duplicate := append([]byte(`{"decision":"OBSERVED",`), raw[1:]...)
	if _, err := inspect(duplicate, rows, id, replayBinding{}); err == nil {
		t.Fatal("duplicate receipt key accepted")
	}
}

func TestSavedReplayRequiresExactReceiptBinding(t *testing.T) {
	rows := []row{{"가🙂\x00", ""}}
	raw, id := fixture(t, rows)
	receipt := digest(raw)
	replay := mutateReceipt(t, raw, func(e map[string]any) {
		e["replayed_from_sha256"] = receipt
		object(e["result"])["replay"] = map[string]any{"schema": "gooo/workspace-body-replay/v1", "prior_result_sha256": digest([]byte("prior")), "body_fills_replayed": 0, "model_calls": 0}
	})
	expected := replayBinding{receipt, digest([]byte("prior"))}
	if _, err := inspect(replay, rows, id, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(replay, rows, id, replayBinding{digest([]byte("other receipt")), expected.Result}); err == nil {
		t.Fatal("replay of a different receipt accepted")
	}
	wrongPrior := mutateReceipt(t, replay, func(e map[string]any) {
		object(object(e["result"])["replay"])["prior_result_sha256"] = digest([]byte("different typed result"))
	})
	if _, err := inspect(wrongPrior, rows, id, expected); err == nil {
		t.Fatal("valid-looking but different prior-result digest accepted")
	}
	if _, err := inspect(replay, rows, id, replayBinding{}); err == nil {
		t.Fatal("saved replay accepted as original execution")
	}
}

func TestInputsRequireTwoTextArgumentsWithinByteLimit(t *testing.T) {
	encode := func(text, needle any) []byte {
		raw, err := json.Marshal(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": []any{map[string]any{appPackage + ":Report.input0": text, appPackage + ":Report.input1": needle}}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, r := range []row{{"", ""}, {strings.Repeat("🙂", 256), ""}, {"", strings.Repeat("a", 1024)}} {
		if _, err := readInputs(encode(r.Text, r.Needle)); err != nil {
			t.Fatal("valid boundary rejected", err)
		}
	}
	for _, raw := range [][]byte{encode(strings.Repeat("a", 1025), "a"), encode("a", strings.Repeat("🙂", 257)), encode(42, "x"), encode("x", nil), []byte(`{"schema":"gooo/body-composition-inputs/v1","inputs":[]}`)} {
		if _, err := readInputs(raw); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
