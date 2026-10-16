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

const headerPortPackage = "go/golang.org/x/net/http/httpguts"
const headerReportPackage = "app/header-field-facts"
const headerNameActivity = "ValidHeaderFieldName"
const headerValueActivity = "ValidHeaderFieldValue"
const headerReportActivity = "Report"
const headerPortSource = "ports/net/httpguts/headers.gooo"
const headerReportSource = "examples/header-field-facts/main.gooo"
const headerFactsEntity = "gooo://go-ports/examples/header-field-facts/result/v1"

func isHeaderSuite(s suite) bool {
	return s.Package == headerPortPackage && (s.Activity == headerNameActivity || s.Activity == headerValueActivity) || s.Package == headerReportPackage && s.Activity == headerReportActivity
}

func headerResultContract(s suite) resultContract {
	if !isHeaderSuite(s) {
		return resultContract{}
	}
	if s.Package == headerPortPackage {
		return resultContract{Entity: "gooo://std/boolean"}
	}
	c := resultContract{Entity: headerFactsEntity}
	for _, name := range []string{"name", "value", "name_valid", "value_valid", "name_empty", "name_has_uppercase", "value_has_obs_text", "value_edge_whitespace", "name_invalid_offset", "name_invalid_octet", "value_invalid_offset", "value_invalid_octet"} {
		c.Fields = append(c.Fields, recordFieldContract{name, "gooo://go-ports/examples/header-field-facts/" + strings.ReplaceAll(name, "_", "-")})
	}
	return c
}

func headerFieldKind(i int) string {
	if i < 2 {
		return "string"
	}
	if i < 8 {
		return "boolean"
	}
	return "integer"
}

func headerRecords(consumer bool) []any {
	if !consumer {
		return nil
	}
	c := headerResultContract(suite{Package: headerReportPackage, Activity: headerReportActivity})
	fields := []any{}
	for i, f := range c.Fields {
		fields = append(fields, map[string]any{"name": f.Name, "id": f.ID, "type_id": "urn:gooo:type:" + headerFieldKind(i), "go_name": "GoooField" + strings.TrimPrefix(contentDigest([]byte(f.ID)), "sha256:")})
	}
	return []any{map[string]any{"name": "HeaderFacts", "id": c.Entity, "go_name": "GoooRecord" + strings.TrimPrefix(contentDigest([]byte(c.Entity)), "sha256:"), "fields": fields}}
}

var headerFieldDeclaration = regexp.MustCompile(`(?m)^[\t ]*field[\t ]+([A-Za-z_][A-Za-z_0-9]*)[\t ]+id[\t ]+"([^"\n]+)"[\t ]+type[\t ]+(string|boolean|integer)[\t ]+required[\t ]+one[\t ]*$`)
var headerAnyFieldDeclaration = regexp.MustCompile(`(?m)^[\t ]*field[\t ]+`)

func headerSourceRecord(raw []byte) error {
	c := headerResultContract(suite{Package: headerReportPackage, Activity: headerReportActivity})
	rows := headerFieldDeclaration.FindAllSubmatch(raw, -1)
	if len(rows) != len(c.Fields) || len(headerAnyFieldDeclaration.FindAll(raw, -1)) != len(c.Fields) || !strings.Contains(string(raw), `entity HeaderFacts id "`+c.Entity+`" fields {`) {
		return fmt.Errorf("header original required record declaration")
	}
	for i, row := range rows {
		if string(row[1]) != c.Fields[i].Name || string(row[2]) != c.Fields[i].ID || string(row[3]) != headerFieldKind(i) {
			return fmt.Errorf("header original record identity/type/order")
		}
	}
	return nil
}

type headerBody struct {
	Package, Source, Namespace, Lowered string
	Declaration                         decimalPayloadBody
}

// Numbering includes both original unary declarations even when only one is
// reachable. The Report includes those same source-resolved pure callees.
func headerBodies(s suite, identity expectedIdentity) (map[string]headerBody, error) {
	consumer := s.Package == headerReportPackage
	files := []string{headerPortSource}
	if consumer {
		files = append(files, headerReportSource)
	}
	if len(identity.SourceBytes) != len(files) || len(identity.Sources) != len(files) {
		return nil, fmt.Errorf("header frozen source roster")
	}
	result := map[string]headerBody{}
	ordinal := 0
	for _, file := range files {
		pkg, namespace, names := headerPortPackage, "gooo_ports_x_net_httpguts", []string{headerNameActivity, headerValueActivity}
		if file == headerReportSource {
			pkg, namespace, names = headerReportPackage, "gooo_ports_examples_header_field_facts", []string{headerReportActivity}
		}
		raw, exists := identity.SourceBytes[file]
		encoded, err := json.Marshal(string(raw))
		if !exists || err != nil || identity.Sources[file] != (sourceIdentity{pkg, contentDigest(encoded)}) {
			return nil, fmt.Errorf("header current source bytes")
		}
		packageName := "httpguts"
		if file == headerReportSource {
			packageName = "header_field_facts"
		}
		if !strings.Contains(string(raw), "namespace "+namespace+"\n") || !strings.Contains(string(raw), "package "+packageName+"\n") {
			return nil, fmt.Errorf("header source package/namespace")
		}
		if file == headerReportSource {
			if err := headerSourceRecord(raw); err != nil {
				return nil, err
			}
			if !strings.Contains(string(raw), `import headers "`+headerPortPackage+`"`) {
				return nil, fmt.Errorf("header source import")
			}
		}
		declared, err := decimalPayloadBodies(raw)
		matches := decimalPayloadDeclaration.FindAllSubmatch(raw, -1)
		if err != nil || len(declared) != len(names) || len(matches) != len(names) {
			return nil, fmt.Errorf("header source activity roster")
		}
		for i, name := range names {
			d := declared[name]
			inputs, output, id := []string{"Text"}, "Boolean", "gooo://golang.org/x/net/http/httpguts/"+name
			if name == headerReportActivity {
				inputs, output, id = []string{"Text", "Text"}, "HeaderFacts", "gooo://go-ports/examples/header-field-facts/report/v1"
			}
			if string(matches[i][1]) != name || d.Name != name || d.ID != id || d.Output != output || !reflect.DeepEqual(d.Inputs, inputs) {
				return nil, fmt.Errorf("header original activity signature/order")
			}
			result[name] = headerBody{pkg, file, namespace, fmt.Sprintf("GoooPackage%dActivity%s", ordinal, name), d}
			ordinal++
		}
	}
	return result, nil
}

func headerLowerBody(d headerBody, definitions map[string]headerBody) string {
	body := d.Declaration.Body
	if d.Package == headerReportPackage {
		for _, name := range []string{headerNameActivity, headerValueActivity} {
			body = strings.ReplaceAll(body, "headers."+name, definitions[name].Lowered)
		}
	}
	return body
}

func headerPureCalls(definitions map[string]headerBody) (map[string]any, error) {
	activities, sites := []any{}, []any{}
	root := definitions[headerReportActivity]
	for _, name := range []string{headerReportActivity, headerNameActivity, headerValueActivity} {
		d := definitions[name]
		activities = append(activities, map[string]any{"activity": map[string]any{"package_path": d.Package, "activity": name, "lowered_name": d.Lowered}, "activity_id": decimalPayloadDerivedID(d.Namespace, name), "lowered_activity_id": decimalPayloadDerivedID("gooo_workspace", d.Lowered), "source": d.Source, "source_body_sha256": contentDigest([]byte(d.Declaration.Body))})
	}
	for _, name := range []string{headerNameActivity, headerValueActivity} {
		token := "headers." + name
		at := strings.Index(root.Declaration.Body, token+"(")
		if at < 0 || strings.Count(root.Declaration.Body, token) != 1 {
			return nil, fmt.Errorf("header original callee site")
		}
		sites = append(sites, map[string]any{"caller": headerReportPackage + ":" + headerReportActivity, "callee": headerPortPackage + ":" + name, "surface": "computes", "start_byte": at, "end_byte": at + len(token)})
	}
	return map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": sites}, nil
}

func headerCallClosure(definitions map[string]headerBody) map[string]any {
	activities, edges := []any{}, []any{}
	for i, name := range []string{headerNameActivity, headerValueActivity} {
		d := definitions[name]
		activities = append(activities, map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(d.Declaration.Body))})
		// These are the pinned839 Report's reviewed typed call ordinals,
		// independently observed in the original receipts, not case indices.
		edges = append(edges, map[string]any{"caller_id": definitions[headerReportActivity].Declaration.ID, "callee_id": d.Declaration.ID, "ordinal": 8 + i})
	}
	return map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": activities, "edges": edges, "max_calls_per_invocation": 2}
}

func inspectHeaderExports(image map[string]any, definitions map[string]headerBody) error {
	packages, ok := image["packages"].([]any)
	if !ok {
		return fmt.Errorf("header package exports")
	}
	for name, d := range definitions {
		found := 0
		for _, value := range packages {
			pkg, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("header package object")
			}
			if pkg["path"] != d.Package {
				continue
			}
			exports, ok := pkg["exports"].([]any)
			if !ok {
				return fmt.Errorf("header exports array")
			}
			for _, value := range exports {
				e, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("header export object")
				}
				if e["name"] != name && e["id"] != d.Declaration.ID {
					continue
				}
				found++
				inputs, out := []string{textEntity}, "gooo://std/boolean"
				if name == headerReportActivity {
					inputs, out = []string{textEntity, textEntity}, headerFactsEntity
				}
				if e["name"] != name || e["kind"] != "activity" || e["id"] != d.Declaration.ID || !decimalPayloadSame(e["input_types"], inputs) || e["output_type"] != out {
					return fmt.Errorf("header selected/callee export ABI")
				}
			}
		}
		if found != 1 {
			return fmt.Errorf("header selected/callee export count")
		}
	}
	return nil
}

// Read concrete native layout/signatures without executing generated code.
func inspectHeaderNativeABI(source, function string, consumer bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), "header-native.go", source, 0)
	if err != nil {
		return fmt.Errorf("header generated Go syntax: %w", err)
	}
	wantOutput, recordName := "bool", "GoooRecord"+strings.TrimPrefix(contentDigest([]byte(headerFactsEntity)), "sha256:")
	if consumer {
		wantOutput = recordName
	}
	functions, records := 0, 0
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.TYPE {
			for _, spec := range d.Specs {
				t, ok := spec.(*ast.TypeSpec)
				if !ok || !consumer || t.Name.Name != recordName || t.Assign.IsValid() || t.TypeParams != nil {
					return fmt.Errorf("header unexpected native type")
				}
				structure, ok := t.Type.(*ast.StructType)
				c := headerResultContract(suite{Package: headerReportPackage, Activity: headerReportActivity})
				if !ok || len(structure.Fields.List) != len(c.Fields) {
					return fmt.Errorf("header native record arity")
				}
				for i, field := range structure.Fields.List {
					kind, ok := field.Type.(*ast.Ident)
					tag := ""
					if field.Tag != nil {
						tag, err = strconv.Unquote(field.Tag.Value)
					}
					goKind := map[string]string{"string": "string", "boolean": "bool", "integer": "int64"}[headerFieldKind(i)]
					if err != nil || !ok || len(field.Names) != 1 || field.Names[0].Name != "GoooField"+strings.TrimPrefix(contentDigest([]byte(c.Fields[i].ID)), "sha256:") || kind.Name != goKind || tag != `json:"`+c.Fields[i].Name+`"` {
						return fmt.Errorf("header native record field identity/type/order/tag")
					}
				}
				records++
			}
		}
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == function {
			functions++
			count := 1
			if consumer {
				count = 2
			}
			if d.Recv != nil || d.Type.TypeParams != nil || d.Type.Params == nil || len(d.Type.Params.List) != count || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				return fmt.Errorf("header native function arity")
			}
			for i, field := range d.Type.Params.List {
				kind, ok := field.Type.(*ast.Ident)
				name := "input"
				if consumer {
					name = fmt.Sprintf("input%d", i)
				}
				if !ok || kind.Name != "string" || len(field.Names) != 1 || field.Names[0].Name != name {
					return fmt.Errorf("header native Text parameter")
				}
			}
			output, ok := d.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || output.Name != wantOutput || len(d.Type.Results.List[0].Names) != 0 {
				return fmt.Errorf("header native result type")
			}
		}
	}
	if functions != 1 || consumer && records != 1 || !consumer && records != 0 {
		return fmt.Errorf("header native function/record roster")
	}
	return nil
}

// This supplements the existing exact value/source-image inspector only for
// the three bounded header entries. It does not recompute compiler semantic
// hashes or the typed inner saved-Result parent digest.
func inspectHeaderABI(raw []byte, s suite, identity expectedIdentity) error {
	if !isHeaderSuite(s) {
		return fmt.Errorf("header entry required")
	}
	definitions, err := headerBodies(s, identity)
	if err != nil {
		return err
	}
	consumer, root := s.Package == headerReportPackage, definitions[s.Activity]
	v, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("header receipt object")
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
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" {
		return fmt.Errorf("header result/program schema")
	}
	initOrder := []string{headerPortPackage}
	if consumer {
		initOrder = append(initOrder, headerReportPackage)
	}
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	imageEntry := map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}
	if image["schema"] != "gooo/package-runtime-image/v1" || !decimalPayloadSame(image["init_order"], initOrder) || !decimalPayloadSame(image["entry"], imageEntry) || !decimalPayloadSame(p["entry"], entry) || !decimalPayloadSame(p["activities"], []any{entry}) || p["entity_aliases"] != nil {
		return fmt.Errorf("header source/lowered entry")
	}
	text, ok := p["lowered_gooo_source"].(string)
	actual, bodyErr := decimalPayloadBodies([]byte(text))
	names := []string{s.Activity}
	if consumer {
		names = []string{headerNameActivity, headerValueActivity, headerReportActivity}
	}
	if !ok || bodyErr != nil || len(actual) != len(names) {
		return fmt.Errorf("header lowered body roster")
	}
	for _, name := range names {
		d := definitions[name]
		a := actual[d.Lowered]
		if a.ID != d.Declaration.ID || a.Output != d.Declaration.Output || !reflect.DeepEqual(a.Inputs, d.Declaration.Inputs) || a.Body != headerLowerBody(d, definitions) {
			return fmt.Errorf("header changed lowered source body/signature")
		}
	}
	if consumer {
		if err := headerSourceRecord([]byte(text)); err != nil {
			return err
		}
		calls, err := headerPureCalls(definitions)
		if err != nil || !decimalPayloadSame(p["pure_calls"], calls) {
			return fmt.Errorf("header original pure-call identities/sites")
		}
	} else if p["pure_calls"] != nil {
		return fmt.Errorf("header unary unexpected pure calls")
	}
	sh := contentDigest([]byte(text))
	if c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["gooo_source"] != text || c["original_source_sha256"] != sh || c["selected_source_sha256"] != sh || r["execution_source_sha256"] != sh {
		return fmt.Errorf("header selected source binding")
	}
	plan, err := decimalPayloadReceiptObject(c, "plan")
	if err != nil {
		return err
	}
	activity := map[string]any{"name": root.Lowered, "id": root.Declaration.ID, "input_type": "Text", "output_type": root.Declaration.Output, "input_entity_id": textEntity, "output_entity_id": headerResultContract(s).Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	if consumer {
		activity["inputs"] = []any{map[string]any{"port": "input0", "type": "Text", "entity_id": textEntity, "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": textEntity, "from": -1}}
	}
	if consumer && plan["entry_activity"] != root.Lowered || !consumer && plan["entry_activity"] != nil || plan["schema"] != "gooo/body-composition-plan/v1" || !decimalPayloadSame(plan["activities"], []any{activity}) || !decimalPayloadSame(plan["edges"], []any{}) || !decimalPayloadSame(plan["record_types"], headerRecords(consumer)) || plan["preparations"] != nil {
		return fmt.Errorf("header typed port/record plan")
	}
	steps, ok := c["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("header generation roster")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sh {
		return fmt.Errorf("header generation source")
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
	in, out := "string", "bool"
	if consumer {
		in, out = "(string,string)", "HeaderFacts"
	}
	if !ok || generated == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != root.Lowered || g["activity_id"] != root.Declaration.ID || g["compiler_source_sha"] != identity.CompilerRevision || g["source_digest"] != sh || g["program_digest"] != contentDigest([]byte(headerLowerBody(root, definitions))) || g["generated_digest"] != contentDigest([]byte(generated)) || g["replay_digest"] != g["generated_digest"] || g["input_type"] != in || g["output_type"] != out || !decimalPayloadSame(g["record_types"], headerRecords(consumer)) || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !decimalPayloadSame(g["repository_writes"], 0) {
		return fmt.Errorf("header generated source/record ABI")
	}
	if consumer {
		parameters := []any{map[string]any{"name": "input0", "type": "string"}, map[string]any{"name": "input1", "type": "string"}}
		if !decimalPayloadSame(g["input_parameters"], parameters) || !decimalPayloadSame(g["call_closure"], headerCallClosure(definitions)) {
			return fmt.Errorf("header generated parameter/callee ABI")
		}
	} else if g["input_parameters"] != nil || g["call_closure"] != nil {
		return fmt.Errorf("header unary unexpected parameter/callee metadata")
	}
	composition, sourceOK := c["source"].(string)
	driver, driverOK := c["driver_source"].(string)
	planSHA, digestOK := plan["typed_plan_sha256"].(string)
	if !sourceOK || composition == "" || !driverOK || driver == "" || c["generated_sha256"] != contentDigest([]byte(composition)) || runtime["generated_sha256"] != c["generated_sha256"] || c["driver_sha256"] != contentDigest([]byte(driver)) || runtime["driver_sha256"] != c["driver_sha256"] || !digestOK || !validDigest(planSHA) || runtime["typed_plan_sha256"] != planSHA || runtime["original_source_sha256"] != sh || runtime["selected_source_sha256"] != sh {
		return fmt.Errorf("header runtime source/driver/plan consistency")
	}
	if err := inspectHeaderNativeABI(generated, root.Lowered, consumer); err != nil {
		return err
	}
	if err := inspectHeaderNativeABI(composition, "GoooComposedActivity0", consumer); err != nil {
		return err
	}
	return inspectHeaderExports(image, definitions)
}
