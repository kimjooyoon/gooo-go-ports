package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type pathGeneratedABI struct {
	Function, Record string
	Helpers          [2]string
	Fields           [6]string
}

var pathFieldNames = [6]string{"text", "ext", "is_abs", "text_byte_length", "ext_byte_length", "has_extension"}
var pathFieldIDs = [6]string{"text", "ext", "is-abs", "text-byte-length", "ext-byte-length", "has-extension"}
var pathFieldTypes = [6]string{"string", "string", "bool", "int64", "int64", "bool"}
var pathFieldEntities = [6]string{"string", "string", "boolean", "integer", "integer", "boolean"}

func pathRecordSymbol(id string) string {
	return "GoooRecord" + strings.TrimPrefix(digest([]byte(id)), "sha256:")
}
func pathFieldSymbol(id string) string {
	return "GoooField" + strings.TrimPrefix(digest([]byte(id)), "sha256:")
}

// This is a continuation of inspect, not a receipt acceptance shortcut. Both
// receipts must already have passed the finite observer, including inner result
// replay, source/workspace/tool and imported-body provenance checks.
func inspectPathGenerated(executionRaw, replayRaw []byte, expected string, id identity) (string, pathGeneratedABI, error) {
	var zero pathGeneratedABI
	if !validDigest(expected) || bytes.Equal(executionRaw, replayRaw) {
		return "", zero, fmt.Errorf("require distinct fresh checked execute/replay receipts")
	}
	e, err := decode(executionRaw)
	if err != nil {
		return "", zero, err
	}
	r, err := decode(replayRaw)
	if err != nil {
		return "", zero, err
	}
	ec := object(object(object(e)["result"])["composition"])
	rr := object(object(r)["result"])
	c := object(rr["composition"])
	if object(r)["replayed_from_sha256"] != digest(executionRaw) || digest(executionRaw) == digest(replayRaw) {
		return "", zero, fmt.Errorf("raw execute/replay binding differs")
	}
	source, ok := c["source"].(string)
	if !ok || source == "" || len(source) > 65536 || c["stage"] != "COMPLETE" || c["generated_sha256"] != expected || digest([]byte(source)) != expected || ec["source"] != source || ec["generated_sha256"] != expected {
		return "", zero, fmt.Errorf("fresh checked generated source differs")
	}
	plan := object(c["plan"])
	wantPlanFields := 5
	if entryName == "report" {
		wantPlanFields = 7
	}
	if len(plan) != wantPlanFields || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(str(plan["semantic_fingerprint"])) {
		return "", zero, fmt.Errorf("typed plan field roster/fingerprint differs")
	}
	if !reflect.DeepEqual(ec["plan"], c["plan"]) || plan["schema"] != "gooo/body-composition-plan/v1" || !validDigest(plan["typed_plan_sha256"]) || len(array(plan["activities"])) != 1 || !equal(plan["edges"], []any{}) || (plan["preparations"] != nil && !equal(plan["preparations"], []any{})) {
		return "", zero, fmt.Errorf("require one independent typed activity")
	}
	a := object(array(plan["activities"])[0])
	outEntity, outType := textEntity, "Text"
	switch entryName {
	case "ext":
	case "isabs":
		outEntity, outType = booleanEntity, "Boolean"
	case "report":
		outEntity, outType = resultEntity, "PathFacts"
	default:
		return "", zero, fmt.Errorf("unknown path entry")
	}
	lowered := str(a["name"])
	if len(a) != 9 || !regexp.MustCompile(`^GoooPackage[0-9]+Activity`+activityName+`$`).MatchString(lowered) || a["id"] != activityID || a["input_type"] != "Text" || a["input_entity_id"] != textEntity || a["output_type"] != outType || a["output_entity_id"] != outEntity || a["go_function"] != "GoooComposedActivity0" || !isInt(a["input_from"], -1) || a["assembling"] != false {
		return "", zero, fmt.Errorf("planned Text activity identity or ABI differs")
	}
	abi := pathGeneratedABI{Function: "GoooComposedActivity0"}
	if entryName == "report" {
		if plan["entry_activity"] != lowered || len(array(plan["record_types"])) != 1 {
			return "", zero, fmt.Errorf("report record or root differs")
		}
		record := object(array(plan["record_types"])[0])
		abi.Record = pathRecordSymbol(resultEntity)
		if len(record) != 4 || record["id"] != resultEntity || record["name"] != "PathFacts" || record["go_name"] != abi.Record || len(array(record["fields"])) != 6 {
			return "", zero, fmt.Errorf("report entity ABI differs")
		}
		for i, value := range array(record["fields"]) {
			field := object(value)
			abi.Fields[i] = pathFieldSymbol(resultEntity + "/" + pathFieldIDs[i])
			if len(field) != 4 || field["name"] != pathFieldNames[i] || field["id"] != resultEntity+"/"+pathFieldIDs[i] || field["type_id"] != "urn:gooo:type:"+pathFieldEntities[i] || field["go_name"] != abi.Fields[i] {
				return "", zero, fmt.Errorf("report field identity/type/order differs at %d", i)
			}
		}
		abi.Helpers = [2]string{abi.Function + "Call0", abi.Function + "Call1"}
	} else if plan["record_types"] != nil || plan["entry_activity"] != nil {
		return "", zero, fmt.Errorf("scalar entry contains record/composition metadata")
	}
	actualFuncs, err := parsePathBindingSource(source, "main", abi)
	if err != nil {
		return "", zero, err
	}
	steps := array(c["steps"])
	if len(steps) != 1 {
		return "", zero, fmt.Errorf("generation step roster differs")
	}
	gen := object(object(steps[0])["generation"])
	gr := object(gen["report"])
	genSource := str(gen["source"])
	if genSource == "" || digest([]byte(genSource)) != gr["generated_digest"] || gr["replay_digest"] != gr["generated_digest"] || gr["activity"] != lowered || gr["activity_id"] != activityID || gr["compiler_source_sha"] != compilerRevision {
		return "", zero, fmt.Errorf("generation function/source identity differs")
	}
	genABI := abi
	genABI.Function = lowered
	renames := map[string]string{lowered: abi.Function}
	if entryName == "report" {
		calls, ok := object(rr["program"])["pure_calls"].(map[string]any)
		if !ok {
			return "", zero, fmt.Errorf("report pure calls must be an object")
		}
		if err := inspectCalls(calls, id.Bodies); err != nil {
			return "", zero, err
		}
		byKey := map[string]string{}
		for _, value := range array(calls["activities"]) {
			ca := object(object(value)["activity"])
			key := str(ca["package_path"]) + ":" + str(ca["activity"])
			name := str(ca["lowered_name"])
			if !regexp.MustCompile(`^GoooPackage[0-9]+Activity`+str(ca["activity"])+`$`).MatchString(name) || byKey[key] != "" {
				return "", zero, fmt.Errorf("imported lowered symbol differs")
			}
			byKey[key] = name
		}
		if byKey[appPackage+":"+activityName] != lowered {
			return "", zero, fmt.Errorf("report lowered source target differs")
		}
		closure := object(gr["call_closure"])
		if closure["schema"] != "gooo/pure-activity-call-closure/v1" || !isInt(closure["max_calls_per_invocation"], 2) || len(array(closure["activities"])) != 2 || len(array(closure["edges"])) != 2 {
			return "", zero, fmt.Errorf("imported generation closure differs")
		}
		for i, name := range []string{"Ext", "IsAbs"} {
			key := "go/path:" + name
			helper := byKey[key]
			ca := object(array(closure["activities"])[i])
			edge := object(array(closure["edges"])[i])
			if helper == "" || ca["name"] != helper || ca["activity_id"] != "gooo://go/path/"+name || ca["program_sha256"] != id.Bodies[key] || edge["caller_id"] != activityID || edge["callee_id"] != "gooo://go/path/"+name || !isInt(edge["ordinal"], i) {
				return "", zero, fmt.Errorf("source-bound helper target differs")
			}
			genABI.Helpers[i] = helper
			renames[helper] = abi.Helpers[i]
		}
	}
	genFuncs, err := parsePathBindingSource(genSource, "gooo_workspace", genABI)
	if err != nil {
		return "", zero, fmt.Errorf("generation ABI: %w", err)
	}
	for name, generated := range genFuncs {
		ast.Inspect(generated, func(n ast.Node) bool {
			if identifier, ok := n.(*ast.Ident); ok {
				if replacement, ok := renames[identifier.Name]; ok {
					identifier.Name = replacement
				}
			}
			return true
		})
		target := actualFuncs[renames[name]]
		if target == nil || pathASTFingerprint(generated) != pathASTFingerprint(target) {
			return "", zero, fmt.Errorf("composition body differs from source-bound generation for %s", name)
		}
	}
	return source, abi, nil
}

func pathASTFingerprint(node ast.Node) string {
	var out bytes.Buffer
	_ = ast.Fprint(&out, nil, node, func(name string, value reflect.Value) bool {
		return value.IsValid() && value.Type() != reflect.TypeOf(token.Pos(0)) && name != "Obj" && name != "Scope" && name != "Unresolved" && name != "Doc" && name != "Comment"
	})
	return out.String()
}

func parsePathBindingSource(source, pkg string, abi pathGeneratedABI) (map[string]*ast.FuncDecl, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	if f.Name.Name != pkg || len(f.Imports) != 0 {
		return nil, fmt.Errorf("generated package/import roster differs")
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(comment.Text)
			if strings.HasPrefix(text, "//go:") || constraint.IsPlusBuild(text) || strings.HasPrefix(text, "//line") || strings.HasPrefix(text, "/*line") || strings.Contains(text, "#cgo") {
				return nil, fmt.Errorf("generated compiler directive refused")
			}
		}
	}
	want := map[string]string{abi.Function: "string"}
	if entryName == "isabs" {
		want[abi.Function] = "bool"
	}
	if entryName == "report" {
		want[abi.Function] = abi.Record
		want[abi.Helpers[0]] = "string"
		want[abi.Helpers[1]] = "bool"
	}
	functions := map[string]*ast.FuncDecl{}
	records := 0
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			result, ok := want[d.Name.Name]
			if !ok || functions[d.Name.Name] != nil || d.Recv != nil || d.Body == nil || d.Type.TypeParams != nil || d.Type.Params == nil || len(d.Type.Params.List) != 1 || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				return nil, fmt.Errorf("generated function roster/signature differs")
			}
			p, r := d.Type.Params.List[0], d.Type.Results.List[0]
			pt, pok := p.Type.(*ast.Ident)
			rt, rok := r.Type.(*ast.Ident)
			if !pok || pt.Name != "string" || len(p.Names) != 1 || p.Names[0].Name != "input" || !rok || rt.Name != result || len(r.Names) != 0 {
				return nil, fmt.Errorf("generated unary Text ABI differs")
			}
			if err := validatePathBody(d, abi); err != nil {
				return nil, err
			}
			functions[d.Name.Name] = d
		case *ast.GenDecl:
			if entryName != "report" || d.Tok != token.TYPE || len(d.Specs) != 1 {
				return nil, fmt.Errorf("generated global/extra declaration refused")
			}
			t, ok := d.Specs[0].(*ast.TypeSpec)
			if !ok || t.Name.Name != abi.Record || t.Assign.IsValid() || t.TypeParams != nil {
				return nil, fmt.Errorf("generated record declaration differs")
			}
			s, ok := t.Type.(*ast.StructType)
			if !ok || s.Fields == nil || len(s.Fields.List) != 6 {
				return nil, fmt.Errorf("generated record must have six required primitive fields")
			}
			for i, field := range s.Fields.List {
				typ, ok := field.Type.(*ast.Ident)
				if !ok || typ.Name != pathFieldTypes[i] || len(field.Names) != 1 || field.Names[0].Name != abi.Fields[i] || field.Tag == nil {
					return nil, fmt.Errorf("generated record field type/order differs")
				}
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil || tag != `json:"`+pathFieldNames[i]+`"` {
					return nil, fmt.Errorf("generated record JSON tag differs")
				}
			}
			records++
		default:
			return nil, fmt.Errorf("unexpected generated declaration")
		}
	}
	wantRecords := 0
	if entryName == "report" {
		wantRecords = 1
	}
	if len(functions) != len(want) || records != wantRecords {
		return nil, fmt.Errorf("generated complete function/record roster differs")
	}
	return functions, nil
}

func validatePathBody(fn *ast.FuncDecl, abi pathGeneratedABI) error {
	var failure error
	counts := map[string]int{}
	allowed := map[string]bool{"len": true, "int64": true}
	if entryName == "report" && fn.Name.Name == abi.Function {
		allowed[abi.Helpers[0]], allowed[abi.Helpers[1]] = true, true
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil || failure != nil {
			return false
		}
		switch node := n.(type) {
		case *ast.BlockStmt, *ast.DeclStmt, *ast.ValueSpec, *ast.AssignStmt, *ast.IfStmt, *ast.ForStmt, *ast.ReturnStmt,
			*ast.ExprStmt, *ast.BinaryExpr, *ast.UnaryExpr, *ast.ParenExpr, *ast.BasicLit, *ast.IndexExpr, *ast.SliceExpr, *ast.KeyValueExpr:
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				failure = fmt.Errorf("generated local declaration refused")
			}
		case *ast.CompositeLit:
			t, ok := node.Type.(*ast.Ident)
			if entryName != "report" || !ok || t.Name != abi.Record {
				failure = fmt.Errorf("generated composite literal refused")
			}
		case *ast.CallExpr:
			name, ok := node.Fun.(*ast.Ident)
			if !ok || !allowed[name.Name] || len(node.Args) != 1 || node.Ellipsis.IsValid() {
				failure = fmt.Errorf("generated call target/arity refused")
			} else {
				counts[name.Name]++
			}
		case *ast.Ident:
			if node.Obj == nil {
				known := allowed[node.Name] || node.Name == "string" || node.Name == "bool" || node.Name == "true" || node.Name == "false" || node.Name == abi.Record
				for _, field := range abi.Fields {
					known = known || field != "" && node.Name == field
				}
				if !known {
					failure = fmt.Errorf("generated unresolved identifier %s refused", node.Name)
				}
			} else if (node.Obj.Kind == ast.Var || node.Obj.Kind == ast.Con) && (node.Name == "len" || node.Name == "int64" || node.Name == abi.Function || node.Name == abi.Helpers[0] || node.Name == abi.Helpers[1]) {
				failure = fmt.Errorf("generated builtin/helper shadow refused")
			}
		default:
			failure = fmt.Errorf("generated AST construct %T refused", n)
		}
		return failure == nil
	})
	if failure != nil {
		return failure
	}
	if entryName == "report" && fn.Name.Name == abi.Function && (counts[abi.Helpers[0]] != 1 || counts[abi.Helpers[1]] != 1) {
		return fmt.Errorf("report must call each source-bound helper exactly once")
	}
	return nil
}
