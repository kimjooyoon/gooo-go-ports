package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func checkABI(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil {
		return err
	}
	recordName := "GoooRecord" + strings.TrimPrefix(digest([]byte(resultEntity)), "sha256:")
	entry, record := false, false
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "GoooComposedActivity0" {
			if entry || fn.Recv != nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("UUID entry signature changed")
			}
			p := fn.Type.Params.List[0]
			t, ok := p.Type.(*ast.Ident)
			result, rok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || t.Name != "string" || len(p.Names) != 1 || p.Names[0].Name != "input" || !rok || result.Name != recordName {
				return fmt.Errorf("UUID unary Text/result ABI changed")
			}
			entry = true
		}
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			t, ok := spec.(*ast.TypeSpec)
			if !ok || t.Name.Name != recordName {
				continue
			}
			s, ok := t.Type.(*ast.StructType)
			if record || !ok || len(s.Fields.List) != 4 {
				return fmt.Errorf("UUID result record changed")
			}
			for i, p := range s.Fields.List {
				kind := "string"
				if i == 0 || i == 3 {
					kind = "bool"
				}
				fieldID := resultEntity + "/" + resultIDs[i]
				want := "GoooField" + strings.TrimPrefix(digest([]byte(fieldID)), "sha256:")
				ft, ok := p.Type.(*ast.Ident)
				if !ok || ft.Name != kind || len(p.Names) != 1 || p.Names[0].Name != want || p.Tag == nil {
					return fmt.Errorf("UUID result field ABI changed")
				}
				tag, err := strconv.Unquote(p.Tag.Value)
				if err != nil || tag != fmt.Sprintf("json:%q", resultNames[i]) {
					return fmt.Errorf("UUID result JSON field changed")
				}
			}
			record = true
		}
	}
	if !entry || !record {
		return fmt.Errorf("UUID generated ABI incomplete")
	}
	return nil
}

func inspectPlan(plan map[string]any, lowered string) error {
	if !keys(plan, "schema", "entry_activity", "typed_plan_sha256", "semantic_fingerprint", "activities", "edges", "record_types") || plan["schema"] != "gooo/body-composition-plan/v1" || plan["entry_activity"] != lowered || !validDigest(plan["typed_plan_sha256"]) || !validDigest("sha256:"+str(plan["semantic_fingerprint"])) || !equal(plan["edges"], []any{}) {
		return fmt.Errorf("typed plan profile/binding")
	}
	want := map[string]any{"name": lowered, "id": activityID, "input_type": "Text", "output_type": "ParseCanonicalResult", "input_entity_id": textEntity, "output_entity_id": resultEntity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	if !equal(plan["activities"], []any{want}) {
		return fmt.Errorf("typed unary activity contract")
	}
	records := array(plan["record_types"])
	if len(records) != 2 {
		return fmt.Errorf("typed record roster")
	}
	seen := map[string]bool{}
	for _, value := range records {
		r := object(value)
		entity := str(r["id"])
		names, ids, kinds := resultNames, resultIDs, []string{"boolean", "string", "string", "boolean"}
		name := "ParseCanonicalResult"
		if entity == "gooo://github.com/google/uuid/validation-result" {
			name = "ValidationResult"
			names = []string{"valid", "error", "invalid_length"}
			ids = []string{"valid", "error", "invalid-length"}
			kinds = []string{"boolean", "string", "boolean"}
		} else if entity != resultEntity {
			return fmt.Errorf("unknown typed record")
		}
		if seen[entity] || !keys(r, "name", "id", "go_name", "fields") || r["name"] != name || r["go_name"] != "GoooRecord"+strings.TrimPrefix(digest([]byte(entity)), "sha256:") {
			return fmt.Errorf("typed record identity")
		}
		seen[entity] = true
		fields := array(r["fields"])
		if len(fields) != len(names) {
			return fmt.Errorf("typed record fields")
		}
		for i, value := range fields {
			fieldID := entity + "/" + ids[i]
			want := map[string]string{"name": names[i], "id": fieldID, "type_id": "urn:gooo:type:" + kinds[i], "go_name": "GoooField" + strings.TrimPrefix(digest([]byte(fieldID)), "sha256:")}
			if !equal(value, want) {
				return fmt.Errorf("typed record field identity/order/type")
			}
		}
	}
	return nil
}
