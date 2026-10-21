package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const headerAdditionPackage = "app/diagnostic-header-addition"
const headerAdditionActivity = "AssessAddition"
const headerAdditionSource = "examples/diagnostic-header-addition/main.gooo"
const headerAdditionInputEntity = "gooo://diagnostic-header-addition/request/v1"
const headerAdditionResultEntity = "gooo://diagnostic-header-addition/result/v1"
const headerAdditionActivityID = "gooo://diagnostic-header-addition/assess/v1"

var headerAdditionInputNames = []string{"content_length_occurrences", "observed_body_bytes", "maximum_payload_bytes", "transfer_encoding_present", "body_observation_bound_to_request", "current_access", "candidate_name_occurrences", "header_set_bound_to_request", "policy_revision_current", "allowed_header_name", "expected_request_revision", "observed_request_revision"}
var headerAdditionInputKinds = []string{"integer", "integer", "integer", "boolean", "boolean", "boolean", "integer", "boolean", "boolean", "string", "integer", "integer"}
var headerAdditionResultNames = []string{"candidate_name", "candidate_value", "status", "reason", "payload_eligible", "payload_reason", "header_name_valid", "header_value_valid", "name_invalid_offset", "name_invalid_octet", "value_invalid_offset", "value_invalid_octet", "value_has_obs_text", "value_edge_whitespace", "request_revision", "eligible"}
var headerAdditionResultKinds = []string{"string", "string", "string", "string", "boolean", "integer", "boolean", "boolean", "integer", "integer", "integer", "integer", "boolean", "boolean", "integer", "boolean"}

func isHeaderAdditionSuite(s suite) bool {
	return s.Package == headerAdditionPackage && s.Activity == headerAdditionActivity
}
func headerAdditionResultContract(s suite) resultContract {
	if !isHeaderAdditionSuite(s) {
		return resultContract{}
	}
	c := resultContract{Entity: headerAdditionResultEntity}
	for _, name := range headerAdditionResultNames {
		c.Fields = append(c.Fields, recordFieldContract{name, c.Entity + "/" + name})
	}
	return c
}
func headerAdditionOwnRecord(name, id string, names, kinds []string) any {
	fields := []any{}
	for i, n := range names {
		fid := id + "/" + n
		fields = append(fields, map[string]any{"name": n, "id": fid, "type_id": "urn:gooo:type:" + kinds[i], "go_name": "GoooField" + strings.TrimPrefix(contentDigest([]byte(fid)), "sha256:")})
	}
	return map[string]any{"name": name, "id": id, "go_name": "GoooRecord" + strings.TrimPrefix(contentDigest([]byte(id)), "sha256:"), "fields": fields}
}
func headerAdditionRecords() []any {
	records := []any{headerAdditionOwnRecord("RequestFacts", headerAdditionInputEntity, headerAdditionInputNames, headerAdditionInputKinds), headerAdditionOwnRecord("HeaderAdditionDecision", headerAdditionResultEntity, headerAdditionResultNames, headerAdditionResultKinds)}
	records = append(records, headerRecords(true)...)
	return append(records, decimalPayloadRecords(true)...)
}
func inspectHeaderAdditionInputFields(entity string, fields []recordFieldValue, expected any) error {
	if entity != headerAdditionInputEntity {
		if len(fields) != 0 {
			return fmt.Errorf("addition scalar input has record fields")
		}
		return nil
	}
	want, ok := expected.(map[string]any)
	if !ok || len(want) != 12 || len(fields) != 12 {
		return fmt.Errorf("addition original12 input record")
	}
	for i, name := range headerAdditionInputNames {
		value, exists := want[name]
		if !exists || fields[i].Name != name || fields[i].ID != entity+"/"+name || !equalValue(fields[i].Value, value) {
			return fmt.Errorf("addition input field identity/type/order/value")
		}
	}
	return nil
}

type headerAdditionDefinition struct {
	Package, Source, Namespace, Lowered string
	Declaration                         decimalPayloadBody
}

func headerAdditionDefinitions(identity expectedIdentity) ([]headerAdditionDefinition, error) {
	specs := []struct {
		pkg, file, namespace, packageName string
		names, outputs, ids               []string
		inputs                            [][]string
	}{
		{headerPortPackage, headerPortSource, "gooo_ports_x_net_httpguts", "httpguts", []string{headerNameActivity, headerValueActivity}, []string{"Boolean", "Boolean"}, []string{"gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldName", "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldValue"}, [][]string{{"Text"}, {"Text"}}},
		{headerReportPackage, headerReportSource, "gooo_ports_examples_header_field_facts", "header_field_facts", []string{"Report"}, []string{"HeaderFacts"}, []string{"gooo://go-ports/examples/header-field-facts/report/v1"}, [][]string{{"Text", "Text"}}},
		{decimalPackage, "ports/strconv/decimal.gooo", "gooo_ports_strconv", "strconv", []string{decimalActivity}, []string{"DecimalInt64Result"}, []string{"gooo://go/strconv/parse-int-decimal64/v1"}, [][]string{{"Text"}}},
		{payloadAdmissionPackage, "examples/payload-admission/main.gooo", "gooo_ports_examples_payload_admission", "payload_admission", []string{"Report"}, []string{"PayloadAdmissionFacts"}, []string{"gooo://go-ports/examples/payload-admission/report/v1"}, [][]string{{"Text", "Integer", "Integer", "Integer", "Boolean", "Boolean", "Boolean"}}},
		{headerAdditionPackage, headerAdditionSource, "gooo_original_diagnostic_header_addition", "diagnostic_header_addition", []string{headerAdditionActivity}, []string{"HeaderAdditionDecision"}, []string{headerAdditionActivityID}, [][]string{{"Text", "Text", "Text", "RequestFacts"}}},
	}
	if len(identity.SourceBytes) != 5 || len(identity.Sources) != 5 {
		return nil, fmt.Errorf("addition five original source files required")
	}
	result := []headerAdditionDefinition{}
	for _, spec := range specs {
		raw, ok := identity.SourceBytes[spec.file]
		encoded, err := json.Marshal(string(raw))
		if !ok || err != nil || identity.Sources[spec.file] != (sourceIdentity{spec.pkg, contentDigest(encoded)}) {
			return nil, fmt.Errorf("addition frozen source identity")
		}
		if !strings.Contains(string(raw), "namespace "+spec.namespace+"\n") || !strings.Contains(string(raw), "package "+spec.packageName+"\n") {
			return nil, fmt.Errorf("addition package/namespace")
		}
		declared, err := decimalPayloadBodies(raw)
		matches := decimalPayloadDeclaration.FindAllSubmatch(raw, -1)
		if err != nil || len(declared) != len(spec.names) || len(matches) != len(spec.names) {
			return nil, fmt.Errorf("addition source activity roster")
		}
		for i, name := range spec.names {
			d := declared[name]
			if string(matches[i][1]) != name || d.Name != name || d.ID != spec.ids[i] || d.Output != spec.outputs[i] || !reflect.DeepEqual(d.Inputs, spec.inputs[i]) {
				return nil, fmt.Errorf("addition source activity identity/signature/order")
			}
			result = append(result, headerAdditionDefinition{spec.pkg, spec.file, spec.namespace, fmt.Sprintf("GoooPackage%dActivity%s", len(result), name), d})
		}
	}
	// The six functions include two independent Reports. Always key by ordinal
	// and full package identity, never by the unqualified activity name.
	if err := headerAdditionSourceRecords(identity.SourceBytes); err != nil {
		return nil, err
	}
	return result, nil
}

var headerAdditionRecordPattern = regexp.MustCompile(`(?ms)^entity[\t ]+([A-Za-z_][A-Za-z_0-9]*)[\t ]+id[\t ]+"([^"\n]+)"[\t ]+fields[\t ]*\{([^}]*)\}`)

func headerAdditionSourceRecords(sources map[string][]byte) error {
	expected := map[string]map[string]any{}
	for _, v := range headerAdditionRecords() {
		r := v.(map[string]any)
		expected[r["name"].(string)] = r
	}
	count := 0
	for _, raw := range sources {
		for _, m := range headerAdditionRecordPattern.FindAllSubmatch(raw, -1) {
			r, ok := expected[string(m[1])]
			if !ok || r["id"] != string(m[2]) {
				return fmt.Errorf("addition source record identity")
			}
			delete(expected, string(m[1]))
			count++
			fields := r["fields"].([]any)
			actual := headerFieldDeclaration.FindAllSubmatch(m[3], -1)
			if len(actual) != len(fields) || len(headerAnyFieldDeclaration.FindAll(m[3], -1)) != len(fields) {
				return fmt.Errorf("addition source record arity")
			}
			for i, v := range fields {
				f := v.(map[string]any)
				if string(actual[i][1]) != f["name"] || string(actual[i][2]) != f["id"] || "urn:gooo:type:"+string(actual[i][3]) != f["type_id"] {
					return fmt.Errorf("addition source record field identity/type/order")
				}
			}
		}
	}
	if count != 5 || len(expected) != 0 {
		return fmt.Errorf("addition five source records required")
	}
	return nil
}

// Original call sites in source order; ordinals belong to pinned839's typed
// call plan, not to case indices or a private receipt's success label.
var headerAdditionEdges = []struct {
	caller, callee, ordinal int
	token                   string
}{{5, 2, 0, "headers.Report"}, {5, 4, 1, "payload.Report"}, {2, 0, 8, "headers.ValidHeaderFieldName"}, {2, 1, 9, "headers.ValidHeaderFieldValue"}, {4, 3, 0, "decimal.ParseIntDecimal64"}}

func headerAdditionLowerBody(index int, defs []headerAdditionDefinition) string {
	body := defs[index].Declaration.Body
	for _, e := range headerAdditionEdges {
		if e.caller == index {
			body = strings.ReplaceAll(body, e.token, defs[e.callee].Lowered)
		}
	}
	return body
}

// Preserve the historical839 synthetic fixture expectation. Current inspection
// supplies an independently selected compiler profile to WithIdentity.
func headerAdditionPureCalls(defs []headerAdditionDefinition) (map[string]any, error) {
	return headerAdditionPureCallsWithIdentity(defs, legacyPureCallIdentityProfile)
}

func headerAdditionPureCallsWithIdentity(defs []headerAdditionDefinition, ids pureCallIdentityProfile) (map[string]any, error) {
	activities, sites := []any{}, []any{}
	for _, i := range []int{5, 2, 4, 0, 1, 3} {
		d := defs[i]
		activities = append(activities, map[string]any{"activity": map[string]any{"package_path": d.Package, "activity": d.Declaration.Name, "lowered_name": d.Lowered}, "activity_id": ids.original(d.Namespace, d.Declaration.Name, d.Declaration.ID), "lowered_activity_id": ids.lowered(d.Lowered, d.Declaration.ID), "source": d.Source, "source_body_sha256": contentDigest([]byte(d.Declaration.Body))})
	}
	for _, e := range headerAdditionEdges {
		caller, callee := defs[e.caller], defs[e.callee]
		at := strings.Index(caller.Declaration.Body, e.token+"(")
		if at < 0 || strings.Count(caller.Declaration.Body, e.token) != 1 {
			return nil, fmt.Errorf("addition original call-site roster")
		}
		sites = append(sites, map[string]any{"caller": caller.Package + ":" + caller.Declaration.Name, "callee": callee.Package + ":" + callee.Declaration.Name, "surface": "computes", "start_byte": at, "end_byte": at + len(e.token)})
	}
	return map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": sites}, nil
}
func headerAdditionCallClosure(defs []headerAdditionDefinition) map[string]any {
	activities, edges := []any{}, []any{}
	for i := 0; i < 5; i++ {
		d := defs[i]
		activities = append(activities, map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(headerAdditionLowerBody(i, defs)))})
	}
	for _, i := range []int{2, 3, 0, 4, 1} {
		e := headerAdditionEdges[i]
		edges = append(edges, map[string]any{"caller_id": defs[e.caller].Declaration.ID, "callee_id": defs[e.callee].Declaration.ID, "ordinal": e.ordinal})
	}
	return map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": activities, "edges": edges, "max_calls_per_invocation": 5}
}
func inspectHeaderAdditionNativeABI(source string, defs []headerAdditionDefinition, composed bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), "header-addition-native.go", source, 0)
	if err != nil {
		return fmt.Errorf("addition native syntax: %w", err)
	}
	kinds := map[string]string{"Text": "string", "Integer": "int64", "Boolean": "bool"}
	records := map[string]map[string]any{}
	for _, v := range headerAdditionRecords() {
		r := v.(map[string]any)
		kinds[r["name"].(string)] = r["go_name"].(string)
		records[r["go_name"].(string)] = r
	}
	names := make([]string, 6)
	for i, d := range defs {
		names[i] = d.Lowered
		if composed {
			names[i] = fmt.Sprintf("GoooComposedActivity0Call%d", i)
			if i == 5 {
				names[i] = "GoooComposedActivity0"
			}
		}
	}
	found := map[string]bool{}
	recordCount := 0
	callCount := 0
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.TYPE {
			for _, v := range d.Specs {
				spec, ok := v.(*ast.TypeSpec)
				if !ok || spec.Assign.IsValid() || spec.TypeParams != nil {
					return fmt.Errorf("addition native type alias")
				}
				r, ok := records[spec.Name.Name]
				if !ok {
					return fmt.Errorf("addition unexpected native record")
				}
				delete(records, spec.Name.Name)
				recordCount++
				structure, ok := spec.Type.(*ast.StructType)
				fields := r["fields"].([]any)
				if !ok || len(structure.Fields.List) != len(fields) {
					return fmt.Errorf("addition native record arity")
				}
				for i, v := range fields {
					f := v.(map[string]any)
					actual := structure.Fields.List[i]
					kind, ok := actual.Type.(*ast.Ident)
					tag := ""
					if actual.Tag != nil {
						tag, err = strconv.Unquote(actual.Tag.Value)
					}
					wantKind := map[string]string{"urn:gooo:type:string": "string", "urn:gooo:type:integer": "int64", "urn:gooo:type:boolean": "bool"}[f["type_id"].(string)]
					if err != nil || !ok || len(actual.Names) != 1 || actual.Names[0].Name != f["go_name"] || kind.Name != wantKind || tag != `json:"`+f["name"].(string)+`"` {
						return fmt.Errorf("addition native field type/ID/order/tag")
					}
				}
			}
		}
		if d, ok := decl.(*ast.FuncDecl); ok {
			index := -1
			for i, n := range names {
				if d.Name.Name == n {
					index = i
				}
			}
			if index < 0 || found[d.Name.Name] {
				return fmt.Errorf("addition native function roster")
			}
			found[d.Name.Name] = true
			original := defs[index].Declaration
			if d.Recv != nil || d.Type.TypeParams != nil || d.Type.Params == nil || len(d.Type.Params.List) != len(original.Inputs) || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				return fmt.Errorf("addition native function arity")
			}
			for i, f := range d.Type.Params.List {
				kind, ok := f.Type.(*ast.Ident)
				name := "input"
				if len(original.Inputs) > 1 {
					name = fmt.Sprintf("input%d", i)
				}
				if !ok || kind.Name != kinds[original.Inputs[i]] || len(f.Names) != 1 || f.Names[0].Name != name {
					return fmt.Errorf("addition native parameter type/name")
				}
			}
			output, ok := d.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || output.Name != kinds[original.Output] || len(d.Type.Results.List[0].Names) != 0 {
				return fmt.Errorf("addition native output type")
			}
			actualCalls := []string{}
			ast.Inspect(d.Body, func(node ast.Node) bool {
				c, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := c.Fun.(*ast.Ident)
				if ok {
					for _, known := range names {
						if name.Name == known {
							actualCalls = append(actualCalls, known)
						}
					}
				}
				return true
			})
			wanted := []string{}
			for _, e := range headerAdditionEdges {
				if e.caller == index {
					wanted = append(wanted, names[e.callee])
				}
			}
			if !reflect.DeepEqual(actualCalls, wanted) {
				return fmt.Errorf("addition native callee call sequence")
			}
			callCount += len(actualCalls)
		}
	}
	if recordCount != 5 || len(records) != 0 || len(found) != 6 || callCount != 5 {
		return fmt.Errorf("addition five records/six functions/five calls")
	}
	return nil
}

// This entry-specific finite inspector supplements the existing independent
// oracle, source-image, value/type and raw saved-parent checks. It does not
// recompute compiler semantic hashes or inner canonical typed Result digests.
func inspectHeaderAdditionABI(raw []byte, s suite, identity expectedIdentity) error {
	if !isHeaderAdditionSuite(s) {
		return fmt.Errorf("addition entry required")
	}
	defs, err := headerAdditionDefinitions(identity)
	if err != nil {
		return err
	}
	root := defs[5]
	node, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := node.(map[string]any)
	if !ok {
		return fmt.Errorf("addition receipt object")
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
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	imageEntry := map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" || !decimalPayloadSame(p["entry"], entry) || !decimalPayloadSame(p["activities"], []any{entry}) || p["entity_aliases"] != nil || image["schema"] != "gooo/package-runtime-image/v1" || !decimalPayloadSame(image["entry"], imageEntry) || !decimalPayloadSame(image["init_order"], []string{headerPortPackage, headerReportPackage, decimalPackage, payloadAdmissionPackage, headerAdditionPackage}) {
		return fmt.Errorf("addition original entry/source initialization")
	}
	text, ok := p["lowered_gooo_source"].(string)
	lowered, bodyErr := decimalPayloadBodies([]byte(text))
	if !ok || bodyErr != nil || len(lowered) != 6 {
		return fmt.Errorf("addition lowered six-body roster")
	}
	for i, d := range defs {
		actual := lowered[d.Lowered]
		if actual.ID != d.Declaration.ID || actual.Output != d.Declaration.Output || !reflect.DeepEqual(actual.Inputs, d.Declaration.Inputs) || actual.Body != headerAdditionLowerBody(i, defs) {
			return fmt.Errorf("addition exact source-derived lowered body/signature")
		}
	}
	ids, err := selectPureCallIdentityProfile(identity.CompilerRevision)
	if err != nil {
		return err
	}
	calls, err := headerAdditionPureCallsWithIdentity(defs, ids)
	if err != nil || !decimalPayloadSame(p["pure_calls"], calls) {
		return fmt.Errorf("addition six pure identities/five source call sites")
	}
	// Validate both record definitions in the lowered original program as well as
	// every imported record; no receipt-supplied field contract is trusted.
	if err := headerAdditionSourceRecords(map[string][]byte{"lowered": []byte(text)}); err != nil {
		return err
	}
	sh := contentDigest([]byte(text))
	if c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["gooo_source"] != text || c["original_source_sha256"] != sh || c["selected_source_sha256"] != sh || r["execution_source_sha256"] != sh {
		return fmt.Errorf("addition selected source binding")
	}
	plan, err := decimalPayloadReceiptObject(c, "plan")
	if err != nil {
		return err
	}
	ports := []any{}
	entities := []string{textEntity, textEntity, textEntity, headerAdditionInputEntity}
	for i, n := range root.Declaration.Inputs {
		ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "type": n, "entity_id": entities[i], "from": -1})
	}
	activity := map[string]any{"name": root.Lowered, "id": root.Declaration.ID, "input_type": "Text", "output_type": "HeaderAdditionDecision", "input_entity_id": textEntity, "output_entity_id": headerAdditionResultEntity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false, "inputs": ports}
	records := headerAdditionRecords()
	if plan["schema"] != "gooo/body-composition-plan/v1" || plan["entry_activity"] != root.Lowered || !decimalPayloadSame(plan["activities"], []any{activity}) || !decimalPayloadSame(plan["record_types"], records) || !decimalPayloadSame(plan["edges"], []any{}) || plan["preparations"] != nil {
		return fmt.Errorf("addition typed12 input/16 output/five-record plan")
	}
	steps, ok := c["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("addition generation step roster")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sh {
		return fmt.Errorf("addition generation source")
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
	parameters := []any{map[string]any{"name": "input0", "type": "string"}, map[string]any{"name": "input1", "type": "string"}, map[string]any{"name": "input2", "type": "string"}, map[string]any{"name": "input3", "type": "RequestFacts"}}
	if !ok || generated == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != root.Lowered || g["activity_id"] != root.Declaration.ID || g["compiler_source_sha"] != identity.CompilerRevision || g["source_digest"] != sh || g["program_digest"] != contentDigest([]byte(headerAdditionLowerBody(5, defs))) || g["generated_digest"] != contentDigest([]byte(generated)) || g["replay_digest"] != g["generated_digest"] || g["input_type"] != "(string,string,string,RequestFacts)" || g["output_type"] != "HeaderAdditionDecision" || !decimalPayloadSame(g["record_types"], records) || !decimalPayloadSame(g["input_parameters"], parameters) || !decimalPayloadSame(g["call_closure"], headerAdditionCallClosure(defs)) || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !decimalPayloadSame(g["repository_writes"], 0) {
		return fmt.Errorf("addition source-derived generation/ABI/callee closure")
	}
	composition, sourceOK := c["source"].(string)
	driver, driverOK := c["driver_source"].(string)
	planSHA, digestOK := plan["typed_plan_sha256"].(string)
	if !sourceOK || composition == "" || !driverOK || driver == "" || c["generated_sha256"] != contentDigest([]byte(composition)) || runtime["generated_sha256"] != c["generated_sha256"] || c["driver_sha256"] != contentDigest([]byte(driver)) || runtime["driver_sha256"] != c["driver_sha256"] || !digestOK || !validDigest(planSHA) || runtime["typed_plan_sha256"] != planSHA || runtime["original_source_sha256"] != sh || runtime["selected_source_sha256"] != sh {
		return fmt.Errorf("addition runtime source/plan/driver consistency")
	}
	if err := inspectHeaderAdditionNativeABI(generated, defs, false); err != nil {
		return err
	}
	if err := inspectHeaderAdditionNativeABI(composition, defs, true); err != nil {
		return err
	}
	// Source image and original selected/callee exports are checked independently
	// from the root plan, including both Reports with their separate entities.
	packages, ok := image["packages"].([]any)
	if !ok {
		return fmt.Errorf("addition export packages")
	}
	for _, d := range defs {
		count := 0
		for _, pv := range packages {
			pkg, ok := pv.(map[string]any)
			if !ok {
				return fmt.Errorf("addition package object")
			}
			if pkg["path"] != d.Package {
				continue
			}
			exports, ok := pkg["exports"].([]any)
			if !ok {
				return fmt.Errorf("addition exports")
			}
			for _, ev := range exports {
				e, ok := ev.(map[string]any)
				if !ok {
					return fmt.Errorf("addition export object")
				}
				if e["name"] != d.Declaration.Name && e["id"] != d.Declaration.ID {
					continue
				}
				count++
				in := []string{}
				for _, n := range d.Declaration.Inputs {
					id := map[string]string{"Text": textEntity, "Integer": integerEntity, "Boolean": "gooo://std/boolean", "RequestFacts": headerAdditionInputEntity}[n]
					in = append(in, id)
				}
				out := map[string]string{"Boolean": "gooo://std/boolean", "HeaderFacts": headerFactsEntity, "DecimalInt64Result": decimalResultEntity, "PayloadAdmissionFacts": payloadAdmissionResultEntity, "HeaderAdditionDecision": headerAdditionResultEntity}[d.Declaration.Output]
				if e["name"] != d.Declaration.Name || e["kind"] != "activity" || e["id"] != d.Declaration.ID || !decimalPayloadSame(e["input_types"], in) || e["output_type"] != out {
					return fmt.Errorf("addition selected/callee export ABI")
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("addition exact selected/callee export count")
		}
	}
	return nil
}
