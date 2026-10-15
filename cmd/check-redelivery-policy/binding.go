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

func recordSymbol() string {
	return "GoooRecord" + strings.TrimPrefix(digest([]byte(resultEntity)), "sha256:")
}
func fieldSymbol(i int) string {
	return "GoooField" + strings.TrimPrefix(digest([]byte(resultEntity+"/"+fieldNames[i])), "sha256:")
}
func typeID(i int) string {
	t := fieldTypes[i]
	if t == "bool" {
		t = "boolean"
	}
	if t == "int64" {
		t = "integer"
	}
	return "urn:gooo:type:" + t
}
func bodyDigest(source string) string {
	// Pinned syntax.Format writes computes as a quoted escaped string even when
	// the original author used raw backticks. Decode its scalar body, then hash
	// exactly the preserved ValueProgram bytes; do not hash the quoted spelling.
	pattern := "(?s)\\bcomputes\\s+(\"(?:\\\\.|[^\"\\\\])*\"|`[^`]*`)"
	matches := regexp.MustCompile(pattern).FindAllStringSubmatch(source, -1)
	if len(matches) != 1 {
		return ""
	}
	body, err := strconv.Unquote(matches[0][1])
	if err != nil {
		return ""
	}
	return digest([]byte(body))
}

func inspectRecordABI(value any) error {
	records := array(value)
	if len(records) != 1 {
		return fmt.Errorf("one required record type")
	}
	r := object(records[0])
	fields := array(r["fields"])
	if !keys(r, "name", "id", "go_name", "fields") || r["name"] != "RedeliveryDecision" || r["id"] != resultEntity || r["go_name"] != recordSymbol() || len(fields) != 15 {
		return fmt.Errorf("record stable identity/required field roster")
	}
	for i, v := range fields {
		f := object(v)
		if !keys(f, "name", "id", "type_id", "go_name") || f["name"] != fieldNames[i] || f["id"] != resultEntity+"/"+fieldNames[i] || f["type_id"] != typeID(i) || f["go_name"] != fieldSymbol(i) {
			return fmt.Errorf("record field %d stable identity/type/order", i)
		}
	}
	return nil
}

func inspectGenerationABI(gr map[string]any) error {
	if gr["input_type"] != "(string,string,bool,bool,bool,bool,bool,int64,string)" || gr["output_type"] != "RedeliveryDecision" || len(array(gr["input_parameters"])) != 9 {
		return fmt.Errorf("generation nine-scalar signature")
	}
	for i, v := range array(gr["input_parameters"]) {
		p := object(v)
		if !keys(p, "name", "type") || p["name"] != "input"+strconv.Itoa(i) || p["type"] != fieldTypes[i] {
			return fmt.Errorf("generation parameter %d type/order", i)
		}
	}
	return inspectRecordABI(gr["record_types"])
}

func inspectPlan(plan map[string]any) error {
	if !keys(plan, "schema", "typed_plan_sha256", "semantic_fingerprint", "activities", "edges", "record_types") || plan["schema"] != "gooo/body-composition-plan/v1" || !validDigest(plan["typed_plan_sha256"]) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(str(plan["semantic_fingerprint"])) || !equal(plan["edges"], []any{}) || len(array(plan["activities"])) != 1 {
		return fmt.Errorf("one direct typed plan, no entry override/edges/helpers")
	}
	a := object(array(plan["activities"])[0])
	ports := array(a["inputs"])
	if !keys(a, "name", "id", "input_type", "output_type", "input_entity_id", "output_entity_id", "go_function", "input_from", "assembling", "inputs") || a["name"] != loweredName || a["id"] != activityID || a["input_type"] != "Text" || a["input_entity_id"] != textEntity || a["output_type"] != "RedeliveryDecision" || a["output_entity_id"] != resultEntity || a["go_function"] != generatedName || !isInt(a["input_from"], -1) || a["assembling"] != false || len(ports) != 9 {
		return fmt.Errorf("typed activity nine-input ABI")
	}
	for i, v := range ports {
		p := object(v)
		if !keys(p, "port", "type", "entity_id", "from") || p["port"] != "input"+strconv.Itoa(i) || p["type"] != inputTypes[i] || p["entity_id"] != inputEntities[i] || !isInt(p["from"], -1) {
			return fmt.Errorf("typed input port %d kind/identity/order", i)
		}
	}
	return inspectRecordABI(plan["record_types"])
}

func inspectGeneratedPair(generated, step string) error {
	actual, err := parseBindingSource(generated, "main", generatedName)
	if err != nil {
		return fmt.Errorf("composition generated ABI: %w", err)
	}
	prior, err := parseBindingSource(step, "gooo_workspace", loweredName)
	if err != nil {
		return fmt.Errorf("generation generated ABI: %w", err)
	}
	prior.Name.Name = generatedName
	if astFingerprint(prior) != astFingerprint(actual) {
		return fmt.Errorf("composition function differs from checked generation")
	}
	return nil
}

func astFingerprint(node ast.Node) string {
	var out bytes.Buffer
	_ = ast.Fprint(&out, nil, node, func(name string, value reflect.Value) bool {
		return value.IsValid() && value.Type() != reflect.TypeOf(token.Pos(0)) && name != "Obj" && name != "Scope" && name != "Unresolved" && name != "Doc" && name != "Comment"
	})
	return out.String()
}

func parseBindingSource(source, pkg, name string) (*ast.FuncDecl, error) {
	if len(source) == 0 || len(source) > 65536 {
		return nil, fmt.Errorf("generated source size")
	}
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	if f.Name.Name != pkg || len(f.Imports) != 0 || len(f.Decls) != 2 {
		return nil, fmt.Errorf("package/declaration/import roster")
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			t := strings.TrimSpace(c.Text)
			if strings.HasPrefix(t, "//go:") || constraint.IsPlusBuild(t) || strings.HasPrefix(t, "//line") || strings.HasPrefix(t, "/*line") || strings.Contains(t, "#cgo") {
				return nil, fmt.Errorf("generated compiler directive refused")
			}
		}
	}
	var fn *ast.FuncDecl
	records := 0
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if fn != nil || d.Name.Name != name || d.Recv != nil || d.Type.TypeParams != nil || d.Body == nil || d.Type.Params == nil || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				return nil, fmt.Errorf("one concrete generated function")
			}
			result := d.Type.Results.List[0]
			rt, ok := result.Type.(*ast.Ident)
			if !ok || rt.Name != recordSymbol() || len(result.Names) != 0 {
				return nil, fmt.Errorf("concrete record result ABI")
			}
			arg := 0
			for _, p := range d.Type.Params.List {
				pt, ok := p.Type.(*ast.Ident)
				if !ok || len(p.Names) == 0 {
					return nil, fmt.Errorf("primitive named parameters required")
				}
				for _, n := range p.Names {
					if arg >= 9 || n.Name != "input"+strconv.Itoa(arg) || pt.Name != fieldTypes[arg] {
						return nil, fmt.Errorf("parameter %d type/order", arg)
					}
					arg++
				}
			}
			if arg != 9 {
				return nil, fmt.Errorf("exactly nine parameters required")
			}
			if err := validateGeneratedBody(d); err != nil {
				return nil, err
			}
			fn = d
		case *ast.GenDecl:
			if d.Tok != token.TYPE || len(d.Specs) != 1 {
				return nil, fmt.Errorf("globals/extra declarations refused")
			}
			t, ok := d.Specs[0].(*ast.TypeSpec)
			if !ok || t.Name.Name != recordSymbol() || t.Assign.IsValid() || t.TypeParams != nil {
				return nil, fmt.Errorf("stable record symbol required")
			}
			s, ok := t.Type.(*ast.StructType)
			if !ok || s.Fields == nil || len(s.Fields.List) != 15 {
				return nil, fmt.Errorf("fifteen required record fields")
			}
			for i, field := range s.Fields.List {
				ft, ok := field.Type.(*ast.Ident)
				if !ok || ft.Name != fieldTypes[i] || len(field.Names) != 1 || field.Names[0].Name != fieldSymbol(i) || field.Tag == nil {
					return nil, fmt.Errorf("record field %d type/symbol/order", i)
				}
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil || tag != `json:"`+fieldNames[i]+`"` {
					return nil, fmt.Errorf("record field %d JSON tag", i)
				}
			}
			records++
		default:
			return nil, fmt.Errorf("unexpected generated declaration")
		}
	}
	if fn == nil || records != 1 {
		return nil, fmt.Errorf("complete generated function/record roster")
	}
	return fn, nil
}

func validateGeneratedBody(fn *ast.FuncDecl) error {
	var failure error
	returns := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil || failure != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.BlockStmt, *ast.DeclStmt, *ast.ValueSpec, *ast.AssignStmt, *ast.IfStmt, *ast.BinaryExpr, *ast.UnaryExpr, *ast.ParenExpr, *ast.BasicLit, *ast.KeyValueExpr, *ast.Ident:
		case *ast.GenDecl:
			if x.Tok != token.VAR || len(x.Specs) != 1 {
				failure = fmt.Errorf("only scalar local var declarations")
			}
		case *ast.ReturnStmt:
			if len(x.Results) != 1 {
				failure = fmt.Errorf("one concrete record return")
			}
			returns++
		case *ast.CompositeLit:
			typ, ok := x.Type.(*ast.Ident)
			if !ok || typ.Name != recordSymbol() || len(x.Elts) != 15 {
				failure = fmt.Errorf("complete stable record constructor")
				break
			}
			for i, v := range x.Elts {
				kv, ok := v.(*ast.KeyValueExpr)
				if !ok {
					failure = fmt.Errorf("named record fields required")
					break
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != fieldSymbol(i) {
					failure = fmt.Errorf("constructor field identity/order")
					break
				}
			}
		default:
			failure = fmt.Errorf("generated body construct refused: %T", n)
		}
		return failure == nil
	})
	if failure != nil {
		return failure
	}
	if returns != 1 {
		return fmt.Errorf("one direct record return required")
	}
	return nil
}
