package main

import (
	"fmt"
	"reflect"
	"runtime"
)

func inspect(raw []byte, rows []row, id identity, priorReceipt string) (checked, error) {
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
	envelopeKeys := []string{"schema", "decision", "manifest", "manifest_digest", "inputs_digest", "result"}
	if priorReceipt != "" {
		envelopeKeys = append(envelopeKeys, "replayed_from_sha256")
	}
	if !keys(e, envelopeKeys...) || e["schema"] != "gooo/workspace-body-execution-receipt/v1" || e["decision"] != "OBSERVED" || e["manifest"] != manifestName || e["manifest_digest"] != id.Manifest || e["inputs_digest"] != id.Inputs {
		return fail("input-only receipt or file binding")
	}
	d := id.Declarations[appPackage+":ParseCanonical"]
	entry := map[string]string{"package_path": appPackage, "activity": "ParseCanonical", "lowered_name": d.Lowered}
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" || !equal(p["entry"], entry) || !equal(p["activities"], []map[string]string{entry}) {
		return fail("program entry")
	}
	for _, name := range []string{"body_fills", "assembly_policy", "assembly_policy_history", "continuation"} {
		if _, ok := r[name]; ok {
			return fail("unsupported body fill/policy/continuation profile")
		}
	}
	if err := inspectWorkspace(object(p["workspace_image"]), id); err != nil {
		return fail(err.Error())
	}
	if err := inspectCalls(object(p["pure_calls"]), id); err != nil {
		return fail(err.Error())
	}
	source := str(p["lowered_gooo_source"])
	sourceDigest := digest([]byte(source))
	if source == "" || r["execution_source_sha256"] != sourceDigest || c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["original_source_sha256"] != sourceDigest || c["selected_source_sha256"] != sourceDigest || c["gooo_source"] != source {
		return fail("lowered source or composition binding")
	}
	for _, name := range []string{"failure", "fill_failure", "record_failure", "model", "fill_model", "continuation", "preparations", "search_validations", "fill_validations"} {
		if _, ok := c[name]; ok {
			return fail("unsupported composition profile")
		}
	}
	steps := array(c["steps"])
	if len(steps) != 1 {
		return fail("generation step roster")
	}
	step := object(steps[0])
	generation := object(step["generation"])
	g := object(generation["report"])
	stepSource := str(generation["source"])
	stepDigest := digest([]byte(stepSource))
	if step["input_source_sha256"] != sourceDigest || stepSource == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != d.Lowered || g["activity_id"] != activityID || g["input_type"] != "string" || g["output_type"] != "ParseCanonicalResult" || g["compiler_source_sha"] != compilerRevision || g["source_digest"] != sourceDigest || g["generated_digest"] != stepDigest || g["replay_digest"] != stepDigest || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !isInt(g["repository_writes"], 0) {
		return fail("generation provenance/source/typecheck/replay binding")
	}
	generated, driver := str(c["source"]), str(c["driver_source"])
	if generated == "" || driver == "" || c["generated_sha256"] != digest([]byte(generated)) || c["driver_sha256"] != digest([]byte(driver)) {
		return fail("generated source or driver digest")
	}
	if err := checkABI(generated); err != nil {
		return fail(err.Error())
	}
	plan := object(c["plan"])
	if err := inspectPlan(plan, d.Lowered); err != nil {
		return fail(err.Error())
	}
	if rt["schema"] != "gooo/body-composition-runtime/v1" || rt["stage"] != "COMPLETE" || rt["failure"] != nil || rt["producer_source_sha"] != compilerRevision || rt["go_version"] != "go version "+goVersion+" "+runtime.GOOS+"/"+runtime.GOARCH || rt["go_tool_sha256"] != id.GoTool || rt["go_tool_selection"] != "explicit" {
		return fail("native producer or explicit toolchain")
	}
	module := object(rt["producer_module"])
	if module["path"] != "github.com/kimjooyoon/meta-ontology-go" || module["version"] != "v0.0.0-20261011201109-83928fa45382" {
		return fail("native producer module")
	}
	if rt["generated_sha256"] != c["generated_sha256"] || rt["driver_sha256"] != c["driver_sha256"] || rt["original_source_sha256"] != sourceDigest || rt["selected_source_sha256"] != sourceDigest || !validDigest(rt["executable_sha256"]) || !validDigest(rt["runtime_suite_sha256"]) || rt["typed_plan_sha256"] != plan["typed_plan_sha256"] {
		return fail("native source/plan/executable binding")
	}
	if !isInt(rt["model_calls"], 0) || !isInt(rt["finite_total"], 0) || !isInt(rt["finite_passed"], 0) || rt["projection_replayed"] != true || rt["runtime_replayed"] != true {
		return fail("input-only counters or native replay flags")
	}
	if !completeProcess(object(rt["toolchain"])) || !completeProcess(object(rt["build"])) {
		return fail("toolchain/build process incomplete")
	}
	runs := array(rt["runs"])
	if len(runs) != 2 || !completeProcess(object(runs[0])) || !completeProcess(object(runs[1])) || object(runs[0])["stdout_sha256"] != object(runs[1])["stdout_sha256"] {
		return fail("two fresh native executions")
	}
	if priorReceipt == "" {
		if _, ok := r["replay"]; ok {
			return fail("unexpected saved replay metadata")
		}
	} else {
		replay := object(r["replay"])
		// Presence and syntax do not verify this inner typed Result hash. The
		// checked report explicitly labels that stronger check NOT_CHECKED.
		if !validDigest(priorReceipt) || e["replayed_from_sha256"] != priorReceipt || !keys(replay, "schema", "prior_result_sha256", "body_fills_replayed", "model_calls", "scope") || replay["schema"] != "gooo/workspace-body-replay/v1" || !isInt(replay["model_calls"], 0) || !isInt(replay["body_fills_replayed"], 0) || !validDigest(replay["prior_result_sha256"]) {
			return fail("outer saved receipt replay binding")
		}
	}
	traces := array(rt["traces"])
	if len(traces) != len(rows) {
		return fail("trace roster")
	}
	result := checked{Generated: str(c["generated_sha256"]), Driver: str(c["driver_sha256"]), Program: p, Plan: plan, Composition: c}
	for i, item := range traces {
		trace := object(item)
		deliveries := array(trace["deliveries"])
		if !keys(trace, "case_index", "deliveries") || !isInt(trace["case_index"], i) || len(deliveries) != 1 {
			return fail("case index or delivery roster")
		}
		delivery := object(deliveries[0])
		if !keys(delivery, "activity_id", "input", "actual", "actual_fields") || delivery["activity_id"] != activityID || delivery["input"] != rows[i].Text {
			return fail(fmt.Sprintf("unary input binding or input-only delivery at row %d", i))
		}
		want := reference(rows[i])
		if !equal(delivery["actual"], want) {
			return fail(fmt.Sprintf("independent original Google UUID oracle differs at row %d", i))
		}
		if err := inspectFields(array(delivery["actual_fields"]), want); err != nil {
			return fail(fmt.Sprintf("row %d: %s", i, err))
		}
		result.Results = append(result.Results, delivery["actual"])
	}
	return result, nil
}

func compareReplay(original, repeated checked) error {
	if original.Generated != repeated.Generated || original.Driver != repeated.Driver || !reflect.DeepEqual(original.Program, repeated.Program) || !reflect.DeepEqual(original.Plan, repeated.Plan) || !reflect.DeepEqual(original.Composition, repeated.Composition) || !reflect.DeepEqual(original.Results, repeated.Results) {
		return fmt.Errorf("saved replay changed program, composition, plan, driver or outputs")
	}
	return nil
}

func inspectWorkspace(image map[string]any, id identity) error {
	if image["schema"] != "gooo/package-runtime-image/v1" || !equal(image["init_order"], []string{quotePackage, uuidPackage, appPackage}) || !equal(image["entry"], map[string]any{"package_path": appPackage, "source": "ports/google/uuid/text/parse_canonical.gooo", "activity": "ParseCanonical", "inputs": []string{"Text"}, "output": "ParseCanonicalResult"}) {
		return fmt.Errorf("workspace schema/order/entry")
	}
	seenSources := map[string]sourceIdentity{}
	seenPackages := map[string]bool{}
	seenActivities := map[string]bool{}
	for _, value := range array(image["packages"]) {
		pkg := object(value)
		pkgPath := str(pkg["path"])
		if !knownPackage(pkgPath) || seenPackages[pkgPath] {
			return fmt.Errorf("workspace package roster")
		}
		seenPackages[pkgPath] = true
		for _, value := range array(pkg["sources"]) {
			source := object(value)
			name := str(source["filename"])
			got := sourceIdentity{pkgPath, str(source["source_digest"])}
			if want, ok := id.Sources[name]; !ok || want != got || seenSources[name].Package != "" {
				return fmt.Errorf("workspace source binding")
			}
			seenSources[name] = got
		}
		for _, value := range array(pkg["exports"]) {
			export := object(value)
			if export["kind"] != "activity" {
				continue
			}
			key := pkgPath + ":" + str(export["name"])
			d, ok := id.Declarations[key]
			if !ok || seenActivities[key] || export["id"] != d.ID || !equal(export["input_types"], []string{entityForType(d.Input)}) || export["output_type"] != entityForType(d.Output) {
				return fmt.Errorf("workspace activity export contract")
			}
			seenActivities[key] = true
		}
	}
	if len(seenPackages) != 3 || len(seenActivities) != len(id.Declarations) || !reflect.DeepEqual(seenSources, id.Sources) {
		return fmt.Errorf("incomplete source/export roster")
	}
	return nil
}

func entityForType(name string) string {
	switch name {
	case "Text":
		return textEntity
	case "Integer":
		return "gooo://std/integer"
	case "Boolean":
		return "gooo://std/boolean"
	case "ParseCanonicalResult":
		return resultEntity
	case "ValidationResult":
		return "gooo://github.com/google/uuid/validation-result"
	}
	return "unsupported"
}

func inspectFields(fields []any, want report) error {
	values := []any{want.Parsed, want.CanonicalUUID, want.Error, want.InvalidLength}
	if len(fields) != 4 {
		return fmt.Errorf("four-field result roster")
	}
	for i, value := range fields {
		field := object(value)
		if !keys(field, "id", "name", "value") || field["name"] != resultNames[i] || field["id"] != resultEntity+"/"+resultIDs[i] || !equal(field["value"], values[i]) {
			return fmt.Errorf("result field identity/order/type/value")
		}
	}
	return nil
}

func completeProcess(p map[string]any) bool {
	return p["started"] == true && p["completed"] == true && p["canceled"] == false && p["timed_out"] == false && p["output_truncated"] == false && isInt(p["diagnostics_bytes"], 0) && isInt(p["exit_code"], 0) && validDigest(p["stdout_sha256"]) && p["stderr_sha256"] == digest(nil) && p["failure"] == nil && p["failure_phase"] == nil && p["diagnostics"] == nil
}
