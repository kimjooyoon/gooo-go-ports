package main

import (
	"fmt"
	"reflect"
	"strconv"
)

func inspect(raw []byte, rows []map[string]any, oracle []oracleRow, id identity, prior replayBinding) (checked, error) {
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
	if len(rows) != 8 || len(oracle) != 8 {
		return fail("fixed batch roster")
	}
	if e["schema"] != "gooo/workspace-body-execution-receipt/v1" || e["decision"] != "OBSERVED" || e["error"] != nil || e["manifest_digest"] != id.Manifest || e["inputs_digest"] != id.Inputs || e["cases_digest"] != nil {
		return fail("input-only receipt/file binding")
	}
	wantEnvelope := []string{"schema", "decision", "manifest", "manifest_digest", "inputs_digest", "result"}
	if prior.Receipt != "" {
		wantEnvelope = append(wantEnvelope, "replayed_from_sha256")
	}
	if !keys(e, wantEnvelope...) {
		return fail("exact input-only envelope roster")
	}
	entry := object(p["entry"])
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" || !keys(entry, "package_path", "activity", "lowered_name") || entry["package_path"] != appPackage || entry["activity"] != activityName || entry["lowered_name"] != loweredName || !equal(p["activities"], []any{entry}) {
		return fail("one original program activity/entry")
	}
	// Do not coerce raw pure_calls to map: false, array, empty string and object
	// are all forbidden for this exact direct nine-input activity. Only nil is
	// accepted, representing absent or JSON null as emitted by pinned839.
	if p["pure_calls"] != nil || p["entity_aliases"] != nil {
		return fail("unexpected pure-call/entity-alias metadata")
	}
	if err := inspectWorkspace(object(p["workspace_image"]), id.Sources); err != nil {
		return fail(err.Error())
	}
	source := str(p["lowered_gooo_source"])
	sourceDigest := digest([]byte(source))
	if source == "" || len(source) > 32768 || r["execution_source_sha256"] != sourceDigest || c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["original_source_sha256"] != sourceDigest || c["selected_source_sha256"] != sourceDigest || c["gooo_source"] != source {
		return fail("lowered source/composition")
	}
	if bodyDigest(source) != id.Body {
		return fail("lowered computes differs from frozen original body")
	}
	steps := array(c["steps"])
	if len(steps) != 1 || object(steps[0])["input_source_sha256"] != sourceDigest {
		return fail("single generation step")
	}
	generation := object(object(steps[0])["generation"])
	gr := object(generation["report"])
	stepSource := str(generation["source"])
	stepDigest := digest([]byte(stepSource))
	if stepSource == "" || gr["schema"] != "gooo/body-codegen-report/v3" || gr["decision"] != "PASS" || gr["activity"] != loweredName || gr["activity_id"] != activityID || gr["compiler_source_sha"] != compilerRevision || gr["source_digest"] != sourceDigest || gr["program_digest"] != id.Body || gr["generated_digest"] != stepDigest || gr["replay_digest"] != stepDigest || gr["typecheck_passed"] != true || gr["deterministic_replay"] != true || !isInt(gr["repository_writes"], 0) || gr["call_closure"] != nil {
		return fail("generation source, body, typecheck or replay provenance")
	}
	if err := inspectGenerationABI(gr); err != nil {
		return fail(err.Error())
	}
	generated := str(c["source"])
	driver := str(c["driver_source"])
	if generated == "" || driver == "" || len(generated) > 65536 || len(driver) > 65536 || c["generated_sha256"] != digest([]byte(generated)) || c["driver_sha256"] != digest([]byte(driver)) {
		return fail("generated/driver raw hashes")
	}
	if rt["schema"] != "gooo/body-composition-runtime/v1" || rt["stage"] != "COMPLETE" || rt["failure"] != nil || rt["producer_source_sha"] != compilerRevision || rt["go_version"] != "go version "+goVersion+" "+runtimePlatform() || rt["go_tool_sha256"] != id.GoTool || rt["go_tool_selection"] != "explicit" {
		return fail("native producer/toolchain")
	}
	module := object(rt["producer_module"])
	if module["path"] != "github.com/kimjooyoon/meta-ontology-go" || module["version"] != "v0.6.37-dev.0.20261020180505-205896a413e5" {
		return fail("native producer module")
	}
	if rt["generated_sha256"] != c["generated_sha256"] || rt["driver_sha256"] != c["driver_sha256"] || rt["original_source_sha256"] != sourceDigest || rt["selected_source_sha256"] != sourceDigest || !validDigest(rt["executable_sha256"]) || !validDigest(rt["runtime_suite_sha256"]) {
		return fail("native source/executable bindings")
	}
	plan := object(c["plan"])
	if err := inspectPlan(plan); err != nil {
		return fail(err.Error())
	}
	if rt["typed_plan_sha256"] != plan["typed_plan_sha256"] {
		return fail("native typed plan hash")
	}
	if err := inspectGeneratedPair(generated, stepSource); err != nil {
		return fail(err.Error())
	}
	if !isInt(rt["model_calls"], 0) || !isInt(rt["finite_total"], 0) || !isInt(rt["finite_passed"], 0) || rt["projection_replayed"] != true || rt["runtime_replayed"] != true {
		return fail("input-only counters/replay flags")
	}
	if !completeProcess(object(rt["toolchain"])) || !completeProcess(object(rt["build"])) {
		return fail("native toolchain/build incomplete")
	}
	runs := array(rt["runs"])
	if len(runs) != 2 || !completeProcess(object(runs[0])) || !completeProcess(object(runs[1])) || object(runs[0])["stdout_sha256"] != object(runs[1])["stdout_sha256"] {
		return fail("two fresh native executions")
	}
	if prior.Receipt == "" && prior.Result == "" {
		if r["replay"] != nil {
			return fail("unexpected saved replay metadata")
		}
	} else {
		rr := object(r["replay"])
		if !validDigest(prior.Receipt) || !validDigest(prior.Result) || e["replayed_from_sha256"] != prior.Receipt || !keys(rr, "schema", "model_calls", "body_fills_replayed", "prior_result_sha256", "scope") || rr["schema"] != "gooo/workspace-body-replay/v1" || !isInt(rr["model_calls"], 0) || !isInt(rr["body_fills_replayed"], 0) || rr["prior_result_sha256"] != prior.Result {
			return fail("raw receipt and inner pinned result replay binding")
		}
	}
	results, err := inspectTraces(rt["traces"], rows, oracle)
	if err != nil {
		return fail(err.Error())
	}
	return checked{Generated: str(c["generated_sha256"]), Driver: str(c["driver_sha256"]), TypedPlan: str(plan["typed_plan_sha256"]), Results: results}, nil
}

func inspectTraces(value any, rows []map[string]any, oracle []oracleRow) ([]any, error) {
	traces := array(value)
	if len(traces) != 8 || len(rows) != 8 || len(oracle) != 8 {
		return nil, fmt.Errorf("trace roster requires eight")
	}
	results := make([]any, 8)
	for i, v := range traces {
		trace := object(v)
		deliveries := array(trace["deliveries"])
		if !keys(trace, "case_index", "deliveries") || !isInt(trace["case_index"], i) || len(deliveries) != 1 {
			return nil, fmt.Errorf("trace %d local index/order/delivery roster", i)
		}
		d := object(deliveries[0])
		if !keys(d, "activity_id", "inputs", "actual", "actual_fields") || d["activity_id"] != activityID {
			return nil, fmt.Errorf("trace %d direct input-only delivery", i)
		}
		ports := array(d["inputs"])
		if len(ports) != 9 {
			return nil, fmt.Errorf("trace %d nine scalar ports", i)
		}
		for j, v := range ports {
			port := object(v)
			if !keys(port, "port", "entity_id", "value") || port["port"] != "input"+strconv.Itoa(j) || port["entity_id"] != inputEntities[j] || !scalar(port["value"], fieldTypes[j]) || !reflect.DeepEqual(port["value"], rows[i][fieldNames[j]]) {
				return nil, fmt.Errorf("trace %d port %d original typed input", i, j)
			}
		}
		actual := object(d["actual"])
		if err := validateRecord(actual, rows[i]); err != nil {
			return nil, fmt.Errorf("trace %d: %w", i, err)
		}
		if !reflect.DeepEqual(actual, oracle[i].Expected) {
			return nil, fmt.Errorf("trace %d independent Go output differs", i)
		}
		if err := inspectFields(d["actual_fields"], actual); err != nil {
			return nil, fmt.Errorf("trace %d: %w", i, err)
		}
		results[i] = actual
	}
	return results, nil
}

func inspectFields(value any, want map[string]any) error {
	fields := array(value)
	if len(fields) != 15 {
		return fmt.Errorf("fifteen actual field roster")
	}
	for i, v := range fields {
		f := object(v)
		if !keys(f, "name", "id", "value") || f["name"] != fieldNames[i] || f["id"] != resultEntity+"/"+fieldNames[i] || !scalar(f["value"], fieldTypes[i]) || !reflect.DeepEqual(f["value"], want[fieldNames[i]]) {
			return fmt.Errorf("field %d identity/type/order/value", i)
		}
	}
	return nil
}

func inspectWorkspace(image map[string]any, expected map[string]sourceIdentity) error {
	if image["schema"] != "gooo/package-runtime-image/v1" || len(array(image["packages"])) != 1 {
		return fmt.Errorf("one workspace package image")
	}
	pkg := object(array(image["packages"])[0])
	sources := array(pkg["sources"])
	if pkg["path"] != appPackage || len(sources) != 1 {
		return fmt.Errorf("workspace package/source roster")
	}
	src := object(sources[0])
	want, ok := expected[sourceName]
	if !ok || len(expected) != 1 || src["filename"] != sourceName || src["source_digest"] != want.Digest || want.Package != appPackage {
		return fmt.Errorf("workspace original source image hash")
	}
	expectedIDs := map[string]string{"Text": textEntity, "Boolean": booleanEntity, "Integer": integerEntity, "RedeliveryDecision": resultEntity, "AssessRedelivery": activityID}
	seen := map[string]bool{}
	exports := array(pkg["exports"])
	if len(exports) != 5 {
		return fmt.Errorf("workspace export roster")
	}
	for _, v := range exports {
		x := object(v)
		name := str(x["name"])
		id, exists := expectedIDs[name]
		if !exists || seen[name] || x["id"] != id {
			return fmt.Errorf("workspace export identity")
		}
		seen[name] = true
		if name == activityName {
			if x["kind"] != "activity" || x["output_type"] != resultEntity || !equal(x["input_types"], inputEntities) {
				return fmt.Errorf("workspace nine-input activity export")
			}
		} else if x["kind"] != "entity" {
			return fmt.Errorf("workspace entity export kind")
		}
	}
	return nil
}

func completeProcess(p map[string]any) bool {
	return p["started"] == true && p["completed"] == true && p["canceled"] == false && p["timed_out"] == false && p["output_truncated"] == false && isInt(p["diagnostics_bytes"], 0) && isInt(p["exit_code"], 0) && validDigest(p["stdout_sha256"]) && p["stderr_sha256"] == digest(nil)
}
