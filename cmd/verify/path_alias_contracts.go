package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const pathCleanSource = "ports/path/clean.gooo"
const pathOriginalSource = "ports/path/path.gooo"
const pathAliasSource = "examples/path-alias-facts/main.gooo"
const pathCleanStateEntity = "gooo://go-ports/path/internal/clean-state/v1"

func pathAliasResultContract(s suite) resultContract {
	if s.Package == pathCleanPackage && s.Activity == pathCleanActivity {
		return resultContract{Entity: textEntity}
	}
	if s.Package != pathAliasPackage || s.Activity != pathAliasActivity {
		return resultContract{}
	}
	const e = "gooo://go-ports/examples/path-alias-facts/result/"
	return resultContract{Entity: pathAliasResultEntity, Fields: []recordFieldContract{
		{"left", e + "left"}, {"right", e + "right"}, {"left_clean", e + "left-clean"}, {"right_clean", e + "right-clean"},
		{"left_is_abs", e + "left-is-abs"}, {"right_is_abs", e + "right-is-abs"},
		{"left_changed", e + "left-changed"}, {"right_changed", e + "right-changed"},
		{"exact_same", e + "exact-same"}, {"same_clean", e + "same-clean"}, {"lexical_alias", e + "lexical-alias"},
	}}
}

type pathAliasBody struct {
	Package, Source, Namespace, Lowered string
	Declaration                         decimalPayloadBody
}

// This concrete source closure includes the unchanged path source's unused Ext
// declaration when assigning ordinals, while only reachable bodies are lowered.
func pathAliasBodies(s suite, identity expectedIdentity) (map[string]pathAliasBody, error) {
	consumer := s.Package == pathAliasPackage
	files := []string{pathCleanSource, pathOriginalSource}
	if consumer {
		files = append(files, pathAliasSource)
	}
	if len(identity.SourceBytes) != len(files) || len(identity.Sources) != len(files) {
		return nil, fmt.Errorf("path alias frozen source roster")
	}
	result := map[string]pathAliasBody{}
	ordinal := 0
	for _, file := range files {
		raw, exists := identity.SourceBytes[file]
		encoded, err := json.Marshal(string(raw))
		pkg, namespace, expected := pathCleanPackage, "gooo_ports_path", []string{"CleanPart", "Clean"}
		if file == pathOriginalSource {
			expected = []string{"Ext", "IsAbs"}
		}
		if file == pathAliasSource {
			pkg, namespace, expected = pathAliasPackage, "gooo_ports_examples_path_alias_facts", []string{"Report"}
		}
		if !exists || err != nil || identity.Sources[file] != (sourceIdentity{pkg, contentDigest(encoded)}) {
			return nil, fmt.Errorf("path alias current source bytes")
		}
		declared, err := decimalPayloadBodies(raw)
		if err != nil || len(declared) != len(expected) {
			return nil, fmt.Errorf("path alias source declaration roster")
		}
		matches := decimalPayloadDeclaration.FindAllSubmatch(raw, -1)
		if len(matches) != len(expected) {
			return nil, fmt.Errorf("path alias declaration order")
		}
		for i, name := range expected {
			if string(matches[i][1]) != name || declared[name].Name != name {
				return nil, fmt.Errorf("path alias original declaration order")
			}
			d := declared[name]
			ins, out, id := []string{"Text"}, "Text", "gooo://go/path/"+name
			if name == "IsAbs" {
				out = "Boolean"
			}
			if name == "CleanPart" {
				ins, out, id = []string{"Text", "Integer", "Boolean", "Text"}, "CleanState", "gooo://go-ports/path/internal/clean-part/v1"
			}
			if name == "Report" {
				ins, out, id = []string{"Text", "Text"}, "PathAliasFacts", "gooo://go-ports/examples/path-alias-facts/report/v1"
			}
			if d.ID != id || d.Output != out || !reflect.DeepEqual(d.Inputs, ins) {
				return nil, fmt.Errorf("path alias original source signature")
			}
			if name != "Ext" {
				result[name] = pathAliasBody{pkg, file, namespace, fmt.Sprintf("GoooPackage%dActivity%s", ordinal, name), d}
			}
			ordinal++
		}
	}
	return result, nil
}

var pathAliasCall = regexp.MustCompile(`(^|[^A-Za-z0-9_.])((?:paths\.)?(?:CleanPart|Clean|IsAbs))\(`)

func pathAliasLowerBody(body string, definitions map[string]pathAliasBody) string {
	var output strings.Builder
	last := 0
	for _, match := range pathAliasCall.FindAllStringSubmatchIndex(body, -1) {
		token := body[match[4]:match[5]]
		name := strings.TrimPrefix(token, "paths.")
		output.WriteString(body[last:match[4]])
		output.WriteString(definitions[name].Lowered)
		last = match[5]
	}
	output.WriteString(body[last:])
	return output.String()
}

// Preserve the historical839 synthetic fixture expectation. Current inspection
// supplies an independently selected compiler profile to WithIdentity.
func pathAliasPureCalls(definitions map[string]pathAliasBody) map[string]any {
	return pathAliasPureCallsWithIdentity(definitions, legacyPureCallIdentityProfile)
}

func pathAliasPureCallsWithIdentity(definitions map[string]pathAliasBody, ids pureCallIdentityProfile) map[string]any {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := definitions[names[i]], definitions[names[j]]
		return a.Package+":"+a.Declaration.Name < b.Package+":"+b.Declaration.Name
	})
	activities, sites := []any{}, []any{}
	for _, name := range names {
		d := definitions[name]
		activities = append(activities, map[string]any{"activity": map[string]any{"package_path": d.Package, "activity": name, "lowered_name": d.Lowered}, "activity_id": ids.original(d.Namespace, name, d.Declaration.ID), "lowered_activity_id": ids.lowered(d.Lowered, d.Declaration.ID), "source": d.Source, "source_body_sha256": contentDigest([]byte(d.Declaration.Body))})
		for _, match := range pathAliasCall.FindAllStringSubmatchIndex(d.Declaration.Body, -1) {
			token := d.Declaration.Body[match[4]:match[5]]
			callee := strings.TrimPrefix(token, "paths.")
			sites = append(sites, map[string]any{"caller": d.Package + ":" + name, "callee": pathCleanPackage + ":" + callee, "surface": "computes", "start_byte": match[4], "end_byte": match[5]})
		}
	}
	return map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": sites}
}

func pathAliasRecords(consumer bool) []any {
	makeRecord := func(name, entity string, fields []recordFieldContract, kinds []string) any {
		rows := []any{}
		for i, f := range fields {
			rows = append(rows, map[string]any{"name": f.Name, "id": f.ID, "type_id": "urn:gooo:type:" + kinds[i], "go_name": "GoooField" + strings.TrimPrefix(contentDigest([]byte(f.ID)), "sha256:")})
		}
		return map[string]any{"name": name, "id": entity, "go_name": "GoooRecord" + strings.TrimPrefix(contentDigest([]byte(entity)), "sha256:"), "fields": rows}
	}
	const base = "gooo://go-ports/path/internal/clean-state/"
	state := makeRecord("CleanState", pathCleanStateEntity, []recordFieldContract{{"path", base + "path/v1"}, {"dotdot", base + "dotdot/v1"}}, []string{"string", "integer"})
	if !consumer {
		return []any{state}
	}
	contract := pathAliasResultContract(suite{Package: pathAliasPackage, Activity: pathAliasActivity})
	report := makeRecord("PathAliasFacts", contract.Entity, contract.Fields, []string{"string", "string", "string", "string", "boolean", "boolean", "boolean", "boolean", "boolean", "boolean", "boolean"})
	return []any{report, state}
}

func pathAliasCallClosure(definitions map[string]pathAliasBody, consumer bool) map[string]any {
	activities, edges := []any{}, []any{}
	for _, name := range []string{"IsAbs", "CleanPart"} {
		d := definitions[name]
		activities = append(activities, map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(pathAliasLowerBody(d.Declaration.Body, definitions)))})
	}
	for i, name := range []string{"IsAbs", "CleanPart", "CleanPart"} {
		edges = append(edges, map[string]any{"caller_id": definitions["Clean"].Declaration.ID, "callee_id": definitions[name].Declaration.ID, "ordinal": i + 2})
	}
	maximum := 1026
	if consumer {
		d := definitions["Clean"]
		activities = append(activities, map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(pathAliasLowerBody(d.Declaration.Body, definitions)))})
		for i, name := range []string{"Clean", "Clean", "IsAbs", "IsAbs"} {
			edges = append(edges, map[string]any{"caller_id": definitions["Report"].Declaration.ID, "callee_id": definitions[name].Declaration.ID, "ordinal": i})
		}
		maximum = 2056
	}
	return map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": activities, "edges": edges, "max_calls_per_invocation": maximum}
}

// Scope is only these two current-source entries. This checks a concrete ABI and
// plan/source consistency; it does not recompute typed_plan_sha256 or the typed
// inner Result replay-parent digest, and does not execute generated source.
func inspectPathAliasABI(raw []byte, s suite, identity expectedIdentity) error {
	if !isPathAliasSuite(s) {
		return fmt.Errorf("path alias entry required")
	}
	definitions, err := pathAliasBodies(s, identity)
	if err != nil {
		return err
	}
	consumer := s.Package == pathAliasPackage
	root := definitions[s.Activity]
	decoded, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("path alias receipt object")
	}
	r, err := decimalPayloadReceiptObject(top, "result")
	if err != nil {
		return err
	}
	p, err := decimalPayloadReceiptObject(r, "program")
	if err != nil {
		return err
	}
	c, err := decimalPayloadReceiptObject(r, "composition")
	if err != nil {
		return err
	}
	runtime, err := decimalPayloadReceiptObject(r, "runtime")
	if err != nil {
		return err
	}
	image, err := decimalPayloadReceiptObject(p, "workspace_image")
	if err != nil {
		return err
	}
	initOrder := []string{pathCleanPackage}
	if consumer {
		initOrder = append(initOrder, pathAliasPackage)
	}
	imageEntry := map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	if image["schema"] != "gooo/package-runtime-image/v1" || !decimalPayloadSame(image["init_order"], initOrder) || !decimalPayloadSame(image["entry"], imageEntry) || !decimalPayloadSame(p["entry"], entry) || !decimalPayloadSame(p["activities"], []any{entry}) || p["entity_aliases"] != nil {
		return fmt.Errorf("path alias source/lowered entry")
	}
	text, ok := p["lowered_gooo_source"].(string)
	actual, bodyErr := decimalPayloadBodies([]byte(text))
	if !ok || bodyErr != nil || len(actual) != len(definitions) {
		return fmt.Errorf("path alias lowered body roster")
	}
	for _, d := range definitions {
		a := actual[d.Lowered]
		if a.ID != d.Declaration.ID || a.Output != d.Declaration.Output || !reflect.DeepEqual(a.Inputs, d.Declaration.Inputs) || a.Body != pathAliasLowerBody(d.Declaration.Body, definitions) {
			return fmt.Errorf("path alias changed lowered source body/signature")
		}
	}
	ids, err := selectPureCallIdentityProfile(identity.CompilerRevision)
	if err != nil {
		return err
	}
	if !decimalPayloadSame(p["pure_calls"], pathAliasPureCallsWithIdentity(definitions, ids)) {
		return fmt.Errorf("path alias original pure-call identities/sites")
	}
	sourceSHA := contentDigest([]byte(text))
	if c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["gooo_source"] != text || c["original_source_sha256"] != sourceSHA || c["selected_source_sha256"] != sourceSHA || r["execution_source_sha256"] != sourceSHA {
		return fmt.Errorf("path alias selected source binding")
	}
	plan, err := decimalPayloadReceiptObject(c, "plan")
	if err != nil {
		return err
	}
	result := pathAliasResultContract(s)
	activity := map[string]any{"name": root.Lowered, "id": root.Declaration.ID, "input_type": "Text", "output_type": root.Declaration.Output, "input_entity_id": textEntity, "output_entity_id": result.Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	if consumer {
		activity["inputs"] = []any{map[string]any{"port": "input0", "type": "Text", "entity_id": textEntity, "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": textEntity, "from": -1}}
	}
	records := pathAliasRecords(consumer)
	if plan["schema"] != "gooo/body-composition-plan/v1" || plan["entry_activity"] != root.Lowered || !decimalPayloadSame(plan["activities"], []any{activity}) || !decimalPayloadSame(plan["edges"], []any{}) || !decimalPayloadSame(plan["record_types"], records) || plan["preparations"] != nil {
		return fmt.Errorf("path alias typed port/record plan")
	}
	steps, ok := c["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("path alias generation roster")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sourceSHA {
		return fmt.Errorf("path alias generation source")
	}
	generation, err := decimalPayloadReceiptObject(step, "generation")
	if err != nil {
		return err
	}
	g, err := decimalPayloadReceiptObject(generation, "report")
	if err != nil {
		return err
	}
	generated, ok := generation["source"].(string)
	inputType, outputType := "string", "string"
	if consumer {
		inputType, outputType = "(string,string)", "PathAliasFacts"
	}
	if !ok || generated == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != root.Lowered || g["activity_id"] != root.Declaration.ID || g["compiler_source_sha"] != identity.CompilerRevision || g["source_digest"] != sourceSHA || g["program_digest"] != contentDigest([]byte(pathAliasLowerBody(root.Declaration.Body, definitions))) || g["generated_digest"] != contentDigest([]byte(generated)) || g["replay_digest"] != g["generated_digest"] || g["input_type"] != inputType || g["output_type"] != outputType || !decimalPayloadSame(g["record_types"], records) || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !decimalPayloadSame(g["repository_writes"], 0) {
		return fmt.Errorf("path alias generated source/record ABI")
	}
	if !decimalPayloadSame(g["call_closure"], pathAliasCallClosure(definitions, consumer)) {
		return fmt.Errorf("path alias compiled callee ABI")
	}
	if consumer {
		if !decimalPayloadSame(g["input_parameters"], []any{map[string]any{"name": "input0", "type": "string"}, map[string]any{"name": "input1", "type": "string"}}) {
			return fmt.Errorf("path alias generated input parameter ABI")
		}
	} else if g["input_parameters"] != nil {
		return fmt.Errorf("Clean unexpected explicit parameter tuple")
	}
	driver, ok := c["driver_source"].(string)
	compositionSource, sourceOK := c["source"].(string)
	planSHA, digestOK := plan["typed_plan_sha256"].(string)
	if !ok || driver == "" || !sourceOK || compositionSource == "" || c["generated_sha256"] != contentDigest([]byte(compositionSource)) || runtime["generated_sha256"] != c["generated_sha256"] || c["driver_sha256"] != contentDigest([]byte(driver)) || runtime["driver_sha256"] != c["driver_sha256"] || runtime["typed_plan_sha256"] != planSHA || !digestOK || !validDigest(planSHA) || runtime["original_source_sha256"] != sourceSHA || runtime["selected_source_sha256"] != sourceSHA {
		return fmt.Errorf("path alias runtime source/driver/plan consistency")
	}
	packages, ok := image["packages"].([]any)
	if !ok {
		return fmt.Errorf("path alias source exports")
	}
	for name, d := range definitions {
		found := 0
		for _, value := range packages {
			pkg, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("path alias package object")
			}
			if pkg["path"] != d.Package {
				continue
			}
			exports, ok := pkg["exports"].([]any)
			if !ok {
				return fmt.Errorf("path alias exports array")
			}
			for _, value := range exports {
				e, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("path alias export object")
				}
				if e["name"] != name && e["id"] != d.Declaration.ID {
					continue
				}
				found++
				entities := make([]string, len(d.Declaration.Inputs))
				for i, kind := range d.Declaration.Inputs {
					entities[i] = map[string]string{"Text": textEntity, "Integer": integerEntity, "Boolean": "gooo://std/boolean"}[kind]
				}
				out := map[string]string{"Text": textEntity, "Boolean": "gooo://std/boolean", "CleanState": pathCleanStateEntity, "PathAliasFacts": pathAliasResultEntity}[d.Declaration.Output]
				if e["name"] != name || e["kind"] != "activity" || e["id"] != d.Declaration.ID || !decimalPayloadSame(e["input_types"], entities) || e["output_type"] != out {
					return fmt.Errorf("path alias source callee export ABI")
				}
			}
		}
		if found != 1 {
			return fmt.Errorf("path alias selected/callee export count")
		}
	}
	return nil
}
