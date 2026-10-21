package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const canonicalHeaderPackage = "go/net/textproto"
const canonicalHeaderActivity = "CanonicalMIMEHeaderKey"
const canonicalHeaderSource = "ports/net/textproto/canonical.gooo"
const canonicalHeaderActivityID = "gooo://go/net/textproto/CanonicalMIMEHeaderKey"
const canonicalHeaderReportPackage = "app/canonical-header-key"
const canonicalHeaderReportActivity = "Report"
const canonicalHeaderReportSource = "examples/canonical-header-key/main.gooo"
const canonicalHeaderReportActivityID = "gooo://canonical-header-key/report/v1"
const canonicalHeaderResultEntity = "gooo://canonical-header-key/report-result/v1"

var canonicalHeaderResultNames = []string{"key", "canonical_key", "changed", "name_valid", "input_bytes", "canonical_bytes", "name_invalid_offset", "name_invalid_octet"}
var canonicalHeaderResultKinds = []string{"string", "string", "boolean", "boolean", "integer", "integer", "integer", "integer"}

func isCanonicalHeaderSuite(s suite) bool {
	return s.Package == canonicalHeaderPackage && s.Activity == canonicalHeaderActivity || s.Package == canonicalHeaderReportPackage && s.Activity == canonicalHeaderReportActivity
}
func canonicalHeaderConsumer(s suite) bool { return s.Package == canonicalHeaderReportPackage }
func readCanonicalHeaderInput(values []any) (string, error) {
	if len(values) != 1 {
		return "", fmt.Errorf("canonical header requires one original Text")
	}
	key, ok := values[0].(string)
	if !ok || !utf8.ValidString(key) || len(key) > 1024 {
		return "", fmt.Errorf("canonical header original validUTF8 Text0..1024bytes required")
	}
	return key, nil
}
func canonicalHeaderResultContract(s suite) resultContract {
	if !isCanonicalHeaderSuite(s) {
		return resultContract{}
	}
	if !canonicalHeaderConsumer(s) {
		return resultContract{Entity: textEntity}
	}
	result := resultContract{Entity: canonicalHeaderResultEntity}
	for _, name := range canonicalHeaderResultNames {
		result.Fields = append(result.Fields, recordFieldContract{name, canonicalHeaderResultEntity + "/" + name})
	}
	return result
}
func canonicalHeaderRecords(consumer bool) []any {
	if !consumer {
		return nil
	}
	return append([]any{headerAdditionOwnRecord("CanonicalHeaderKeyFacts", canonicalHeaderResultEntity, canonicalHeaderResultNames, canonicalHeaderResultKinds)}, headerRecords(true)...)
}

type canonicalHeaderDefinition struct {
	Package, Source, Namespace, Lowered string
	Declaration                         decimalPayloadBody
}

// Whole-package numbering retains the unused httpguts value declaration for
// the scalar entry. Only reachable bodies enter its generated two-function ABI.
func canonicalHeaderDefinitions(s suite, identity expectedIdentity) ([]canonicalHeaderDefinition, error) {
	consumer := canonicalHeaderConsumer(s)
	if !isCanonicalHeaderSuite(s) || len(identity.SourceBytes) != len(canonicalHeaderPackages(s)) || len(identity.Sources) != len(identity.SourceBytes) {
		return nil, fmt.Errorf("canonical header exact original source roster")
	}
	type spec struct {
		pkg, file, namespace, packageName string
		names, outputs, ids               []string
		inputs                            [][]string
		imports                           []string
	}
	specs := []spec{{headerPortPackage, headerPortSource, "gooo_ports_x_net_httpguts", "httpguts", []string{headerNameActivity, headerValueActivity}, []string{"Boolean", "Boolean"}, []string{"gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldName", "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldValue"}, [][]string{{"Text"}, {"Text"}}, nil}}
	if consumer {
		specs = append(specs, spec{headerReportPackage, headerReportSource, "gooo_ports_examples_header_field_facts", "header_field_facts", []string{headerReportActivity}, []string{"HeaderFacts"}, []string{"gooo://go-ports/examples/header-field-facts/report/v1"}, [][]string{{"Text", "Text"}}, []string{`import headers "` + headerPortPackage + `"`}})
	}
	specs = append(specs, spec{canonicalHeaderPackage, canonicalHeaderSource, "gooo_ports_net_textproto", "textproto", []string{canonicalHeaderActivity}, []string{"Text"}, []string{canonicalHeaderActivityID}, [][]string{{"Text"}}, []string{`import headers "` + headerPortPackage + `"`}})
	if consumer {
		specs = append(specs, spec{canonicalHeaderReportPackage, canonicalHeaderReportSource, "gooo_original_canonical_header_key", "canonical_header_key", []string{canonicalHeaderReportActivity}, []string{"CanonicalHeaderKeyFacts"}, []string{canonicalHeaderReportActivityID}, [][]string{{"Text"}}, []string{`import textproto "` + canonicalHeaderPackage + `"`, `import headers "` + headerReportPackage + `"`}})
	}
	result := []canonicalHeaderDefinition{}
	ordinal := 0
	for _, spec := range specs {
		raw, ok := identity.SourceBytes[spec.file]
		encoded, err := json.Marshal(string(raw))
		if !ok || err != nil || identity.Sources[spec.file] != (sourceIdentity{spec.pkg, contentDigest(encoded)}) {
			return nil, fmt.Errorf("canonical header frozen source identity")
		}
		if !strings.Contains(string(raw), "namespace "+spec.namespace+"\n") || !strings.Contains(string(raw), "package "+spec.packageName+"\n") || len(catalogImport.FindAll(raw, -1)) != len(spec.imports) {
			return nil, fmt.Errorf("canonical header package/namespace/import roster")
		}
		previous := -1
		for _, line := range spec.imports {
			at := strings.Index(string(raw), line+"\n")
			if at < 0 || at <= previous {
				return nil, fmt.Errorf("canonical header original import aliases/order")
			}
			previous = at
		}
		declared, err := decimalPayloadBodies(raw)
		matches := decimalPayloadDeclaration.FindAllSubmatch(raw, -1)
		if err != nil || len(declared) != len(spec.names) || len(matches) != len(spec.names) {
			return nil, fmt.Errorf("canonical header source activity roster")
		}
		for i, name := range spec.names {
			body := declared[name]
			if string(matches[i][1]) != name || body.Name != name || body.ID != spec.ids[i] || body.Output != spec.outputs[i] || !reflect.DeepEqual(body.Inputs, spec.inputs[i]) {
				return nil, fmt.Errorf("canonical header source signature/ID/order")
			}
			lowered := fmt.Sprintf("GoooPackage%dActivity%s", ordinal, name)
			ordinal++
			if !consumer && spec.pkg == headerPortPackage && name == headerValueActivity {
				continue
			}
			result = append(result, canonicalHeaderDefinition{spec.pkg, spec.file, spec.namespace, lowered, body})
		}
	}
	if err := canonicalHeaderSourceRecords(identity.SourceBytes, consumer); err != nil {
		return nil, err
	}
	return result, nil
}
func canonicalHeaderSourceRecords(sources map[string][]byte, consumer bool) error {
	expected := map[string]map[string]any{}
	for _, value := range canonicalHeaderRecords(consumer) {
		record := value.(map[string]any)
		expected[record["name"].(string)] = record
	}
	count := 0
	for _, raw := range sources {
		for _, m := range headerAdditionRecordPattern.FindAllSubmatch(raw, -1) {
			record, ok := expected[string(m[1])]
			if !ok || record["id"] != string(m[2]) {
				return fmt.Errorf("canonical header source record identity")
			}
			delete(expected, string(m[1]))
			count++
			fields := record["fields"].([]any)
			actual := headerFieldDeclaration.FindAllSubmatch(m[3], -1)
			if len(actual) != len(fields) || len(headerAnyFieldDeclaration.FindAll(m[3], -1)) != len(fields) {
				return fmt.Errorf("canonical header source record arity")
			}
			for i, value := range fields {
				field := value.(map[string]any)
				if string(actual[i][1]) != field["name"] || string(actual[i][2]) != field["id"] || "urn:gooo:type:"+string(actual[i][3]) != field["type_id"] {
					return fmt.Errorf("canonical header source field ID/type/order")
				}
			}
		}
	}
	if count != len(canonicalHeaderRecords(consumer)) || len(expected) != 0 {
		return fmt.Errorf("canonical header exact source records")
	}
	return nil
}

type canonicalHeaderEdge struct {
	caller, callee, ordinal int
	token                   string
}

func canonicalHeaderEdges(consumer bool) []canonicalHeaderEdge {
	if !consumer {
		return []canonicalHeaderEdge{{1, 0, 0, "headers.ValidHeaderFieldName"}}
	}
	return []canonicalHeaderEdge{{4, 2, 0, "headers.Report"}, {4, 3, 1, "textproto.CanonicalMIMEHeaderKey"}, {2, 0, 8, "headers.ValidHeaderFieldName"}, {2, 1, 9, "headers.ValidHeaderFieldValue"}, {3, 0, 0, "headers.ValidHeaderFieldName"}}
}
func canonicalHeaderLowerBody(index int, defs []canonicalHeaderDefinition, consumer bool) string {
	body := defs[index].Declaration.Body
	for _, edge := range canonicalHeaderEdges(consumer) {
		if edge.caller == index {
			body = strings.ReplaceAll(body, edge.token, defs[edge.callee].Lowered)
		}
	}
	return body
}

// Preserve the historical839 synthetic fixture expectation. Current inspection
// supplies an independently selected compiler profile to WithIdentity.
func canonicalHeaderPureCalls(defs []canonicalHeaderDefinition, consumer bool) (map[string]any, error) {
	return canonicalHeaderPureCallsWithIdentity(defs, consumer, legacyPureCallIdentityProfile)
}

func canonicalHeaderPureCallsWithIdentity(defs []canonicalHeaderDefinition, consumer bool, ids pureCallIdentityProfile) (map[string]any, error) {
	activities, sites := []any{}, []any{}
	order := make([]int, len(defs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := defs[order[i]], defs[order[j]]
		return a.Package+":"+a.Declaration.Name < b.Package+":"+b.Declaration.Name
	})
	for _, i := range order {
		d := defs[i]
		activities = append(activities, map[string]any{"activity": map[string]any{"package_path": d.Package, "activity": d.Declaration.Name, "lowered_name": d.Lowered}, "activity_id": ids.original(d.Namespace, d.Declaration.Name, d.Declaration.ID), "lowered_activity_id": ids.lowered(d.Lowered, d.Declaration.ID), "source": d.Source, "source_body_sha256": contentDigest([]byte(d.Declaration.Body))})
	}
	for _, edge := range canonicalHeaderEdges(consumer) {
		caller, callee := defs[edge.caller], defs[edge.callee]
		at := strings.Index(caller.Declaration.Body, edge.token+"(")
		if at < 0 || strings.Count(caller.Declaration.Body, edge.token) != 1 {
			return nil, fmt.Errorf("canonical header original callee site")
		}
		sites = append(sites, map[string]any{"caller": caller.Package + ":" + caller.Declaration.Name, "callee": callee.Package + ":" + callee.Declaration.Name, "surface": "computes", "start_byte": at, "end_byte": at + len(edge.token)})
	}
	return map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": sites}, nil
}
func canonicalHeaderCallClosure(defs []canonicalHeaderDefinition, consumer bool) map[string]any {
	activities, edges := []any{}, []any{}
	for i := 0; i < len(defs)-1; i++ {
		d := defs[i]
		activities = append(activities, map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(canonicalHeaderLowerBody(i, defs, consumer)))})
	}
	order := []int{0}
	if consumer {
		order = []int{2, 3, 0, 4, 1}
	}
	sourceEdges := canonicalHeaderEdges(consumer)
	for _, i := range order {
		edge := sourceEdges[i]
		edges = append(edges, map[string]any{"caller_id": defs[edge.caller].Declaration.ID, "callee_id": defs[edge.callee].Declaration.ID, "ordinal": edge.ordinal})
	}
	return map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": activities, "edges": edges, "max_calls_per_invocation": len(sourceEdges)}
}

func inspectCanonicalHeaderNativeABI(source string, defs []canonicalHeaderDefinition, consumer, composed bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), "canonical-header-native.go", source, 0)
	if err != nil {
		return fmt.Errorf("canonical header native syntax: %w", err)
	}
	kinds := map[string]string{"Text": "string", "Integer": "int64", "Boolean": "bool"}
	records := map[string]map[string]any{}
	for _, v := range canonicalHeaderRecords(consumer) {
		r := v.(map[string]any)
		kinds[r["name"].(string)] = r["go_name"].(string)
		records[r["go_name"].(string)] = r
	}
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Lowered
		if composed {
			names[i] = fmt.Sprintf("GoooComposedActivity0Call%d", i)
			if i == len(defs)-1 {
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
					return fmt.Errorf("canonical header native type alias")
				}
				r, ok := records[spec.Name.Name]
				if !ok {
					return fmt.Errorf("canonical header unexpected native record")
				}
				delete(records, spec.Name.Name)
				recordCount++
				structure, ok := spec.Type.(*ast.StructType)
				fields := r["fields"].([]any)
				if !ok || len(structure.Fields.List) != len(fields) {
					return fmt.Errorf("canonical header native record arity")
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
						return fmt.Errorf("canonical header native field type/ID/order/tag")
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
				return fmt.Errorf("canonical header native function roster")
			}
			found[d.Name.Name] = true
			original := defs[index].Declaration
			if d.Recv != nil || d.Type.TypeParams != nil || d.Type.Params == nil || len(d.Type.Params.List) != len(original.Inputs) || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				return fmt.Errorf("canonical header native function arity")
			}
			for i, f := range d.Type.Params.List {
				kind, ok := f.Type.(*ast.Ident)
				name := "input"
				if len(original.Inputs) > 1 {
					name = fmt.Sprintf("input%d", i)
				}
				if !ok || kind.Name != kinds[original.Inputs[i]] || len(f.Names) != 1 || f.Names[0].Name != name {
					return fmt.Errorf("canonical header native parameter type/name")
				}
			}
			output, ok := d.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || output.Name != kinds[original.Output] || len(d.Type.Results.List[0].Names) != 0 {
				return fmt.Errorf("canonical header native output type")
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
			for _, e := range canonicalHeaderEdges(consumer) {
				if e.caller == index {
					wanted = append(wanted, names[e.callee])
				}
			}
			if !reflect.DeepEqual(actualCalls, wanted) {
				return fmt.Errorf("canonical header native callee call sequence")
			}
			callCount += len(actualCalls)
		}
	}
	if recordCount != len(canonicalHeaderRecords(consumer)) || len(records) != 0 || len(found) != len(defs) || callCount != len(canonicalHeaderEdges(consumer)) {
		return fmt.Errorf("canonical header exact reached functions/records/calls")
	}
	return nil
}

// Finite entry-specific checks supplement the unchanged independent oracle,
// typed trace, source image and raw saved-parent checks. Inner canonical receipt
// hash protocols and universal source semantics are not recomputed here.
func inspectCanonicalHeaderABI(raw []byte, s suite, identity expectedIdentity) error {
	consumer := canonicalHeaderConsumer(s)
	defs, err := canonicalHeaderDefinitions(s, identity)
	if err != nil {
		return err
	}
	root := defs[len(defs)-1]
	node, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := node.(map[string]any)
	if !ok {
		return fmt.Errorf("canonical header receipt object")
	}
	result, err := decimalPayloadReceiptObject(top, "result")
	if err != nil {
		return err
	}
	program, err := decimalPayloadReceiptObject(result, "program")
	if err != nil {
		return err
	}
	composition, err := decimalPayloadReceiptObject(result, "composition")
	if err != nil {
		return err
	}
	runtime, err := decimalPayloadReceiptObject(result, "runtime")
	if err != nil {
		return err
	}
	image, err := decimalPayloadReceiptObject(program, "workspace_image")
	if err != nil {
		return err
	}
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	imageEntry := map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": []string{"Text"}, "output": root.Declaration.Output}
	packageOrder := []string{}
	for _, pkg := range canonicalHeaderPackages(s) {
		packageOrder = append(packageOrder, pkg.Path)
	}
	if result["schema"] != "gooo/workspace-body-execution/v1" || program["schema"] != "gooo/workspace-body-program/v1" || !decimalPayloadSame(program["entry"], entry) || !decimalPayloadSame(program["activities"], []any{entry}) || program["entity_aliases"] != nil || image["schema"] != "gooo/package-runtime-image/v1" || !decimalPayloadSame(image["entry"], imageEntry) || !decimalPayloadSame(image["init_order"], packageOrder) {
		return fmt.Errorf("canonical header exact original entry/source initialization")
	}
	text, ok := program["lowered_gooo_source"].(string)
	lowered, bodyErr := decimalPayloadBodies([]byte(text))
	if !ok || bodyErr != nil || len(lowered) != len(defs) {
		return fmt.Errorf("canonical header reached body roster")
	}
	for i, d := range defs {
		actual := lowered[d.Lowered]
		if actual.ID != d.Declaration.ID || actual.Output != d.Declaration.Output || !reflect.DeepEqual(actual.Inputs, d.Declaration.Inputs) || actual.Body != canonicalHeaderLowerBody(i, defs, consumer) {
			return fmt.Errorf("canonical header source-derived lowered body/signature")
		}
	}
	ids, err := selectPureCallIdentityProfile(identity.CompilerRevision)
	if err != nil {
		return err
	}
	calls, err := canonicalHeaderPureCallsWithIdentity(defs, consumer, ids)
	if err != nil || !decimalPayloadSame(program["pure_calls"], calls) {
		return fmt.Errorf("canonical header source pure participants/call sites")
	}
	if err := canonicalHeaderSourceRecords(map[string][]byte{"lowered": []byte(text)}, consumer); err != nil {
		return err
	}
	sourceSHA := contentDigest([]byte(text))
	if composition["schema"] != "gooo/body-composition/v1" || composition["stage"] != "COMPLETE" || composition["gooo_source"] != text || composition["original_source_sha256"] != sourceSHA || composition["selected_source_sha256"] != sourceSHA || result["execution_source_sha256"] != sourceSHA {
		return fmt.Errorf("canonical header selected original source binding")
	}
	plan, err := decimalPayloadReceiptObject(composition, "plan")
	if err != nil {
		return err
	}
	output := canonicalHeaderResultContract(s).Entity
	activity := map[string]any{"name": root.Lowered, "id": root.Declaration.ID, "input_type": "Text", "output_type": root.Declaration.Output, "input_entity_id": textEntity, "output_entity_id": output, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	records := canonicalHeaderRecords(consumer)
	if plan["schema"] != "gooo/body-composition-plan/v1" || plan["entry_activity"] != root.Lowered || !decimalPayloadSame(plan["activities"], []any{activity}) || !decimalPayloadSame(plan["record_types"], records) || !decimalPayloadSame(plan["edges"], []any{}) || plan["preparations"] != nil {
		return fmt.Errorf("canonical header unary Text and exact record plan")
	}
	steps, ok := composition["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("canonical header generation step roster")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sourceSHA {
		return fmt.Errorf("canonical header generation source")
	}
	generation, err := decimalPayloadReceiptObject(step, "generation")
	if err != nil {
		return err
	}
	report, err := decimalPayloadReceiptObject(generation, "report")
	if err != nil {
		return err
	}
	generated, ok := generation["source"].(string)
	outputType := "string"
	if consumer {
		outputType = "CanonicalHeaderKeyFacts"
	}
	if !ok || generated == "" || report["schema"] != "gooo/body-codegen-report/v3" || report["decision"] != "PASS" || report["activity"] != root.Lowered || report["activity_id"] != root.Declaration.ID || report["compiler_source_sha"] != identity.CompilerRevision || report["source_digest"] != sourceSHA || report["program_digest"] != contentDigest([]byte(canonicalHeaderLowerBody(len(defs)-1, defs, consumer))) || report["generated_digest"] != contentDigest([]byte(generated)) || report["replay_digest"] != report["generated_digest"] || report["input_type"] != "string" || report["output_type"] != outputType || !decimalPayloadSame(report["record_types"], records) || report["input_parameters"] != nil || !decimalPayloadSame(report["call_closure"], canonicalHeaderCallClosure(defs, consumer)) || report["typecheck_passed"] != true || report["deterministic_replay"] != true || !decimalPayloadSame(report["repository_writes"], 0) {
		return fmt.Errorf("canonical header generation/source/record/callee ABI")
	}
	composed, sourceOK := composition["source"].(string)
	driver, driverOK := composition["driver_source"].(string)
	planSHA, digestOK := plan["typed_plan_sha256"].(string)
	if !sourceOK || composed == "" || !driverOK || driver == "" || composition["generated_sha256"] != contentDigest([]byte(composed)) || runtime["generated_sha256"] != composition["generated_sha256"] || composition["driver_sha256"] != contentDigest([]byte(driver)) || runtime["driver_sha256"] != composition["driver_sha256"] || !digestOK || !validDigest(planSHA) || runtime["typed_plan_sha256"] != planSHA || runtime["original_source_sha256"] != sourceSHA || runtime["selected_source_sha256"] != sourceSHA {
		return fmt.Errorf("canonical header runtime source/typed plan/driver consistency")
	}
	if err := inspectCanonicalHeaderNativeABI(generated, defs, consumer, false); err != nil {
		return err
	}
	if err := inspectCanonicalHeaderNativeABI(composed, defs, consumer, true); err != nil {
		return err
	}
	packages, ok := image["packages"].([]any)
	if !ok {
		return fmt.Errorf("canonical header export package roster")
	}
	for _, d := range defs {
		count := 0
		for _, pv := range packages {
			pkg, ok := pv.(map[string]any)
			if !ok {
				return fmt.Errorf("canonical header package object")
			}
			if pkg["path"] != d.Package {
				continue
			}
			exports, ok := pkg["exports"].([]any)
			if !ok {
				return fmt.Errorf("canonical header exports")
			}
			for _, ev := range exports {
				export, ok := ev.(map[string]any)
				if !ok {
					return fmt.Errorf("canonical header export object")
				}
				if export["name"] != d.Declaration.Name && export["id"] != d.Declaration.ID {
					continue
				}
				count++
				inputs := []string{}
				for _, name := range d.Declaration.Inputs {
					inputs = append(inputs, map[string]string{"Text": textEntity}[name])
				}
				out := map[string]string{"Text": textEntity, "Boolean": "gooo://std/boolean", "HeaderFacts": headerFactsEntity, "CanonicalHeaderKeyFacts": canonicalHeaderResultEntity}[d.Declaration.Output]
				if export["name"] != d.Declaration.Name || export["kind"] != "activity" || export["id"] != d.Declaration.ID || !decimalPayloadSame(export["input_types"], inputs) || export["output_type"] != out {
					return fmt.Errorf("canonical header selected/callee typed export")
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("canonical header exact selected/callee export count")
		}
	}
	return nil
}
