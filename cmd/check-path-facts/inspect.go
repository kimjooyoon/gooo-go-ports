package main

import (
	"fmt"
	"reflect"
)

func inspect(raw []byte, rows []string, id identity, prior replayBinding) (checked, error) {
	v, err := decode(raw)
	if err != nil {
		return checked{}, err
	}
	e := object(v)
	r := object(e["result"])
	p := object(r["program"])
	c := object(r["composition"])
	rt := object(r["runtime"])
	fail := func(reason string) (checked, error) {
		return checked{}, fmt.Errorf("invalid native evidence: %s", reason)
	}
	if e["schema"] != "gooo/workspace-body-execution-receipt/v1" || e["decision"] != "OBSERVED" || e["error"] != nil || e["manifest_digest"] != id.Manifest || e["inputs_digest"] != id.Inputs || e["cases_digest"] != nil {
		return fail("input-only receipt or file binding")
	}
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" || object(p["entry"])["package_path"] != appPackage || object(p["entry"])["activity"] != activityName {
		return fail("program entry")
	}
	if err := inspectWorkspace(object(p["workspace_image"]), id.Sources); err != nil {
		return fail(err.Error())
	}
	// Preserve the raw JSON kind before object coercion, especially on replay.
	rawCalls := p["pure_calls"]
	if entryName != "report" {
		if rawCalls != nil {
			return fail("unexpected pure-call metadata for a direct scalar entry")
		}
	} else if _, ok := rawCalls.(map[string]any); !ok {
		return fail("imported pure-call metadata must be an object")
	}
	if err := inspectCalls(object(rawCalls), id.Bodies); err != nil {
		return fail(err.Error())
	}
	source := str(p["lowered_gooo_source"])
	sourceDigest := digest([]byte(source))
	if source == "" || r["execution_source_sha256"] != sourceDigest || c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["original_source_sha256"] != sourceDigest || c["selected_source_sha256"] != sourceDigest || c["gooo_source"] != source {
		return fail("lowered source or body composition")
	}
	steps := array(c["steps"])
	if len(steps) != 1 || object(steps[0])["input_source_sha256"] != sourceDigest || object(object(object(steps[0])["generation"])["report"])["activity_id"] != activityID {
		return fail("body-generation entry")
	}
	generation := object(object(steps[0])["generation"])
	report := object(generation["report"])
	stepSource := str(generation["source"])
	stepDigest := digest([]byte(stepSource))
	if stepSource == "" || report["schema"] != "gooo/body-codegen-report/v3" || report["decision"] != "PASS" || report["compiler_source_sha"] != compilerRevision || report["source_digest"] != sourceDigest || report["generated_digest"] != stepDigest || report["replay_digest"] != stepDigest || report["typecheck_passed"] != true || report["deterministic_replay"] != true || !isInt(report["repository_writes"], 0) {
		return fail("body-generation provenance, typecheck or deterministic replay")
	}
	generated := str(c["source"])
	driver := str(c["driver_source"])
	if generated == "" || driver == "" || c["generated_sha256"] != digest([]byte(generated)) || c["driver_sha256"] != digest([]byte(driver)) {
		return fail("generated source or driver hash")
	}
	if rt["schema"] != "gooo/body-composition-runtime/v1" || rt["stage"] != "COMPLETE" || rt["failure"] != nil || rt["producer_source_sha"] != compilerRevision || rt["go_version"] != "go version "+goVersion+" "+runtimePlatform() || rt["go_tool_sha256"] != id.GoTool || rt["go_tool_selection"] != "explicit" {
		return fail("native producer or toolchain")
	}
	module := object(rt["producer_module"])
	if module["path"] != "github.com/kimjooyoon/meta-ontology-go" || module["version"] != "v0.6.37-dev.0.20261020180505-205896a413e5" {
		return fail("native producer module")
	}
	if rt["generated_sha256"] != c["generated_sha256"] || rt["driver_sha256"] != c["driver_sha256"] || rt["original_source_sha256"] != sourceDigest || rt["selected_source_sha256"] != sourceDigest || !validDigest(rt["executable_sha256"]) || !validDigest(rt["runtime_suite_sha256"]) {
		return fail("native source/executable binding")
	}
	plan := object(c["plan"])
	if plan["schema"] != "gooo/body-composition-plan/v1" || !validDigest(plan["typed_plan_sha256"]) || rt["typed_plan_sha256"] != plan["typed_plan_sha256"] {
		return fail("native typed-plan binding")
	}
	if !isInt(rt["model_calls"], 0) || !isInt(rt["finite_total"], 0) || !isInt(rt["finite_passed"], 0) || rt["projection_replayed"] != true || rt["runtime_replayed"] != true {
		return fail("native input-only counters or replay flags")
	}
	if !completeProcess(object(rt["toolchain"])) || !completeProcess(object(rt["build"])) {
		return fail("native toolchain/build did not complete")
	}
	runs := array(rt["runs"])
	if len(runs) != 2 || !completeProcess(object(runs[0])) || !completeProcess(object(runs[1])) || object(runs[0])["stdout_sha256"] != object(runs[1])["stdout_sha256"] {
		return fail("two fresh native executions")
	}
	if prior.Receipt == "" && prior.Result == "" {
		if e["replayed_from_sha256"] != nil || r["replay"] != nil {
			return fail("unexpected saved replay metadata")
		}
	} else {
		replay := object(r["replay"])
		if !validDigest(prior.Receipt) || !validDigest(prior.Result) || e["replayed_from_sha256"] != prior.Receipt || replay["schema"] != "gooo/workspace-body-replay/v1" || !isInt(replay["model_calls"], 0) || !isInt(replay["body_fills_replayed"], 0) || replay["prior_result_sha256"] != prior.Result {
			return fail("saved receipt replay binding")
		}
	}
	traces := array(rt["traces"])
	if len(traces) != len(rows) {
		return fail("trace roster")
	}
	result := checked{Generated: str(c["generated_sha256"]), Driver: str(c["driver_sha256"])}
	for i, item := range traces {
		trace := object(item)
		deliveries := array(trace["deliveries"])
		if !isInt(trace["case_index"], i) || len(deliveries) != 1 {
			return fail("case index or delivery count")
		}
		d := object(deliveries[0])
		if d["activity_id"] != activityID || d["input"] != rows[i] || d["input_fields"] != nil || d["inputs"] != nil || d["producer_id"] != nil || d["expected"] != nil || d["passed"] != nil || d["fault"] != nil || d["blocked_by"] != nil {
			return fail(fmt.Sprintf("original single input, activity identity or input-only metadata at row %d", i))
		}
		want := expectedValue(oracleReports[i])
		if !equal(d["actual"], want) {
			return fail(fmt.Sprintf("independent Go oracle differs at row %d", i))
		}
		if entryName == "report" {
			if err := inspectFields(array(d["actual_fields"]), oracleReports[i]); err != nil {
				return fail(fmt.Sprintf("row %d: %s", i, err))
			}
		} else if d["actual_fields"] != nil {
			return fail("scalar output unexpectedly has record fields")
		}

		result.Results = append(result.Results, d["actual"])
	}
	return result, nil
}

func inspectWorkspace(image map[string]any, expected map[string]sourceIdentity) error {
	if image["schema"] != "gooo/package-runtime-image/v1" {
		return fmt.Errorf("workspace image schema")
	}
	seenSources := map[string]sourceIdentity{}
	seenPackages := map[string]bool{}
	selected := map[string]bool{}
	outputs := map[string]string{"go/path:Ext": textEntity, "go/path:IsAbs": booleanEntity}
	if entryName == "report" {
		outputs[reportPackage+":Report"] = resultEntity
	}
	for _, value := range array(image["packages"]) {
		pkg := object(value)
		path := str(pkg["path"])
		if (path != "go/path" && !(entryName == "report" && path == reportPackage)) || seenPackages[path] {
			return fmt.Errorf("workspace package roster")
		}
		seenPackages[path] = true
		for _, value := range array(pkg["sources"]) {
			source := object(value)
			name := str(source["filename"])
			got := sourceIdentity{path, str(source["source_digest"])}
			if want, ok := expected[name]; !ok || want != got || seenSources[name].Package != "" {
				return fmt.Errorf("workspace source binding: %s", name)
			}
			seenSources[name] = got
		}
		for _, value := range array(pkg["exports"]) {
			export := object(value)
			name := str(export["name"])
			key := path + ":" + name
			output, chosen := outputs[key]
			if !chosen {
				continue
			}
			id := "gooo://go/path/" + name
			if path == reportPackage {
				id = "gooo://go-ports/examples/path-facts/report"
			}
			if selected[key] || export["kind"] != "activity" || export["id"] != id || export["output_type"] != output || !equal(export["input_types"], []string{textEntity}) {
				return fmt.Errorf("workspace activity export contract")
			}
			selected[key] = true
		}
	}
	wantPackages := 1
	if entryName == "report" {
		wantPackages = 2
	}
	if len(seenPackages) != wantPackages || len(selected) != len(outputs) || !reflect.DeepEqual(seenSources, expected) {
		return fmt.Errorf("workspace source/export roster is incomplete")
	}
	return nil
}

func inspectCalls(calls map[string]any, bodies map[string]string) error {
	// pinned839 workspaceCalls.expand returns nil when there are no call sites.
	if entryName != "report" {
		if calls != nil {
			return fmt.Errorf("unexpected pure-call metadata for a direct scalar entry")
		}
		return nil
	}
	expected := map[string]string{appPackage + ":" + activityName: "ports/path/path.gooo"}
	sites := map[string]bool{}
	if entryName == "report" {
		expected = map[string]string{reportPackage + ":Report": "examples/path-facts/main.gooo", "go/path:Ext": "ports/path/path.gooo", "go/path:IsAbs": "ports/path/path.gooo"}
		sites = map[string]bool{reportPackage + ":Report>go/path:Ext": false, reportPackage + ":Report>go/path:IsAbs": false}
	}
	seen := map[string]bool{}
	if calls["schema"] != "gooo/workspace-pure-calls/v1" || len(array(calls["activities"])) != len(expected) {
		return fmt.Errorf("imported pure activity roster")
	}
	for _, value := range array(calls["activities"]) {
		item := object(value)
		activity := object(item["activity"])
		key := str(activity["package_path"]) + ":" + str(activity["activity"])
		if expected[key] == "" || seen[key] || item["source"] != expected[key] || bodies[key] == "" || item["source_body_sha256"] != bodies[key] {
			return fmt.Errorf("imported body source binding")
		}
		seen[key] = true
	}
	if len(array(calls["sites"])) != len(sites) {
		return fmt.Errorf("imported call sites count")
	}
	for _, value := range array(calls["sites"]) {
		site := object(value)
		key := str(site["caller"]) + ">" + str(site["callee"])
		done, ok := sites[key]
		if !ok || done || site["surface"] != "computes" {
			return fmt.Errorf("imported call site binding")
		}
		sites[key] = true
	}
	return nil
}

func inspectFields(fields []any, want report) error {
	names := []string{"text", "ext", "is_abs", "text_byte_length", "ext_byte_length", "has_extension"}
	ids := []string{"text", "ext", "is-abs", "text-byte-length", "ext-byte-length", "has-extension"}
	values := []any{want.Text, want.Ext, want.IsAbs, want.TextByteLength, want.ExtByteLength, want.HasExtension}
	if len(fields) != len(names) {
		return fmt.Errorf("result field roster")
	}
	for i, value := range fields {
		field := object(value)
		if field["name"] != names[i] || field["id"] != resultEntity+"/"+ids[i] || !equal(field["value"], values[i]) {
			return fmt.Errorf("result field identity/order/value")
		}
	}
	return nil
}

func completeProcess(p map[string]any) bool {
	return p["started"] == true && p["completed"] == true && p["canceled"] == false && p["timed_out"] == false && p["output_truncated"] == false && isInt(p["diagnostics_bytes"], 0) && isInt(p["exit_code"], 0) && validDigest(p["stdout_sha256"]) && p["stderr_sha256"] == digest(nil)
}
