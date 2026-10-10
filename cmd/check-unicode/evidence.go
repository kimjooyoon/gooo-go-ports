package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type activityExport struct {
	Name, Kind, ID string
	Inputs         []string `json:"input_types"`
	Output         string   `json:"output_type"`
}
type recordField struct {
	Name, ID string
	Type     string `json:"type_id"`
	GoName   string `json:"go_name"`
}
type recordType struct {
	ID     string
	GoName string `json:"go_name"`
	Fields []recordField
}
type callClosure struct {
	Activities []struct {
		ID string `json:"activity_id"`
	} `json:"activities"`
	Edges []struct {
		Caller  string `json:"caller_id"`
		Callee  string `json:"callee_id"`
		Ordinal int    `json:"ordinal"`
	} `json:"edges"`
}
type composition struct {
	Stage, Source string
	Digest        string `json:"generated_sha256"`
	Plan          struct {
		Activities []struct {
			ID       string
			Function string `json:"go_function"`
			Input    string `json:"input_entity_id"`
			Output   string `json:"output_entity_id"`
		}
		Records []recordType `json:"record_types"`
	} `json:"plan"`
	Steps []struct {
		Generation struct {
			Report struct {
				ID       string      `json:"activity_id"`
				Producer string      `json:"compiler_source_sha"`
				Closure  callClosure `json:"call_closure"`
			} `json:"report"`
		} `json:"generation"`
	} `json:"steps"`
}
type receipt struct {
	Schema, Decision, Error string
	ManifestDigest          string `json:"manifest_digest"`
	CasesDigest             string `json:"cases_digest"`
	ReplayedFrom            string `json:"replayed_from_sha256"`
	Result                  struct {
		Program struct {
			Entry struct {
				Package  string `json:"package_path"`
				Activity string
			} `json:"entry"`
			Image struct {
				Packages []struct {
					Path    string
					Exports []activityExport
					Sources []struct {
						Filename string
						Digest   string `json:"source_digest"`
					}
				}
			} `json:"workspace_image"`
		} `json:"program"`
		Composition composition `json:"composition"`
		Replay      *struct {
			Calls *int `json:"model_calls"`
		} `json:"replay"`
		Runtime struct {
			Stage      string
			Failure    json.RawMessage
			Producer   string `json:"producer_source_sha"`
			Go         string `json:"go_version"`
			Digest     string `json:"generated_sha256"`
			Calls      *int   `json:"model_calls"`
			Passed     *int   `json:"finite_passed"`
			Total      *int   `json:"finite_total"`
			Projection bool   `json:"projection_replayed"`
			Replayed   bool   `json:"runtime_replayed"`
			Traces     []struct {
				Index      int `json:"case_index"`
				Deliveries []struct {
					ID     string `json:"activity_id"`
					Passed bool
					Input  json.RawMessage
					Inputs []struct {
						Port   string
						Entity string `json:"entity_id"`
						Value  json.RawMessage
					}
					Fields []struct {
						ID, Name string
						Value    json.RawMessage
					} `json:"actual_fields"`
					Actual, Expected json.RawMessage
				}
			}
		} `json:"runtime"`
	} `json:"result"`
}
type generatedNames struct{ Entry, Letter, Digit, Record, InRuneDomain, CanStart, CanContinue, IsDecimalDigit string }

func inspectReceipt(raw []byte, b evidenceBinding, replayedFrom string) (string, generatedNames, error) {
	fail := func(reason string) (string, generatedNames, error) {
		return "", generatedNames{}, fmt.Errorf("invalid evidence: %s", reason)
	}
	var e receipt
	if err := json.Unmarshal(raw, &e); err != nil {
		return fail(err.Error())
	}
	if e.Schema != "gooo/workspace-body-execution-receipt/v1" || e.Decision != "PASS" || e.Error != "" || len(b.Cases) == 0 {
		return fail("schema or decision")
	}
	if e.ManifestDigest != b.ManifestDigest || e.CasesDigest != b.CasesDigest {
		return fail("manifest or cases binding")
	}
	p, c, r := e.Result.Program, e.Result.Composition, e.Result.Runtime
	if p.Entry.Package != "app/identifier-rune" || p.Entry.Activity != "Classify" {
		return fail("entry")
	}
	seenPackages := map[string]bool{}
	seenSources := map[string]sourceBinding{}
	seenExports := map[string]bool{}
	contracts := map[string]struct{ Package, Name, Output string }{entryID: {"app/identifier-rune", "Classify", replyID}, letterID: {"go/unicode", "IsLetter", "gooo://std/boolean"}, digitID: {"go/unicode", "IsDigit", "gooo://std/boolean"}}
	for _, pkg := range p.Image.Packages {
		if !b.Packages[pkg.Path] || seenPackages[pkg.Path] {
			return fail("workspace package roster")
		}
		seenPackages[pkg.Path] = true
		for _, source := range pkg.Sources {
			if _, exists := seenSources[source.Filename]; exists {
				return fail("duplicate source")
			}
			seenSources[source.Filename] = sourceBinding{pkg.Path, source.Digest}
		}
		for _, export := range pkg.Exports {
			for id, want := range contracts {
				if export.ID != id && !(pkg.Path == want.Package && export.Name == want.Name) {
					continue
				}
				if seenExports[id] || pkg.Path != want.Package || export.ID != id || export.Name != want.Name || export.Kind != "activity" || export.Output != want.Output || !reflect.DeepEqual(export.Inputs, []string{"gooo://std/integer"}) {
					return fail("activity export identity")
				}
				seenExports[id] = true
			}
		}
	}
	if !reflect.DeepEqual(seenPackages, b.Packages) || !reflect.DeepEqual(seenSources, b.Sources) || len(seenExports) != 3 {
		return fail("source or export bindings")
	}
	if c.Stage != "COMPLETE" || c.Source == "" || c.Digest != sha([]byte(c.Source)) || r.Digest != c.Digest {
		return fail("generated source digest")
	}
	version := strings.Fields(r.Go)
	if r.Stage != "COMPLETE" || (len(r.Failure) > 0 && string(r.Failure) != "null") || r.Producer != b.Lock.CompilerRevision || len(version) != 4 || version[0] != "go" || version[1] != "version" || version[2] != b.Lock.GoVersion {
		return fail("runtime identity")
	}
	if r.Calls == nil || *r.Calls != 0 || r.Passed == nil || *r.Passed != len(b.Cases) || r.Total == nil || *r.Total != len(b.Cases) || !r.Projection || !r.Replayed || len(r.Traces) != len(b.Cases) {
		return fail("runtime counts or replay")
	}
	if e.ReplayedFrom != replayedFrom {
		return fail("saved receipt binding")
	}
	if replayedFrom != "" {
		if e.Result.Replay == nil || e.Result.Replay.Calls == nil || *e.Result.Replay.Calls != 0 {
			return fail("saved replay calls")
		}
	} else if e.Result.Replay != nil {
		return fail("unexpected replay")
	}
	for i, t := range r.Traces {
		if t.Index != i || len(t.Deliveries) != 1 {
			return fail("trace roster")
		}
		d := t.Deliveries[0]
		if d.ID != entryID || !d.Passed || len(d.Inputs) != 0 {
			return fail("delivery identity")
		}
		input, _ := json.Marshal(b.Cases[i].Inputs[entryKey])
		want, _ := json.Marshal(b.Cases[i].Expected[entryKey])
		if !sameJSON(d.Input, input) || !sameJSON(d.Actual, want) || !sameJSON(d.Expected, want) {
			return fail("trace input or oracle output")
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(want, &output); err != nil {
			return fail("oracle output")
		}
		fieldNames := map[string]string{"in_rune_domain": "in-rune-domain", "can_start": "can-start", "can_continue": "can-continue", "is_decimal_digit": "is-decimal-digit"}
		if len(d.Fields) != 4 {
			return fail("actual field roster")
		}
		seen := map[string]bool{}
		for _, field := range d.Fields {
			if seen[field.Name] || fieldNames[field.Name] == "" || field.ID != replyID+"/"+fieldNames[field.Name] || !sameJSON(field.Value, output[field.Name]) {
				return fail("actual field binding")
			}
			seen[field.Name] = true
		}
	}
	names, err := inspectGenerated(c, b.Lock.CompilerRevision)
	if err != nil {
		return fail(err.Error())
	}
	return c.Source, names, nil
}

func inspectGenerated(c composition, compilerRevision string) (generatedNames, error) {
	var n generatedNames
	bad := func(reason string) (generatedNames, error) {
		return n, fmt.Errorf("generated entry binding: %s", reason)
	}
	if len(c.Plan.Activities) != 1 || len(c.Steps) != 1 || len(c.Plan.Records) != 1 {
		return bad("plan roster")
	}
	activity := c.Plan.Activities[0]
	report := c.Steps[0].Generation.Report
	if activity.ID != entryID || activity.Input != "gooo://std/integer" || activity.Output != replyID || report.ID != entryID || report.Producer != compilerRevision {
		return bad("plan identities")
	}
	n.Entry = activity.Function
	if !token.IsIdentifier(n.Entry) {
		return bad("entry identifier")
	}
	closure := report.Closure
	if len(closure.Activities) != 2 || len(closure.Edges) != 2 {
		return bad("call closure roster")
	}
	ids := map[string]bool{}
	// The pinned compiler names CallN by the Activities array index in
	// internal/bodyexecution/composition_calls.go, not by an edge's ordinal.
	// Edge ordinals instead identify calls in the caller's source traversal.
	for i, a := range closure.Activities {
		if ids[a.ID] || (a.ID != letterID && a.ID != digitID) {
			return bad("call closure activity")
		}
		ids[a.ID] = true
		helper := fmt.Sprintf("%sCall%d", n.Entry, i)
		if a.ID == letterID {
			n.Letter = helper
		} else {
			n.Digit = helper
		}
	}
	// This consumer calls IsLetter first and IsDigit second, once each.
	expectedCallIDs := []string{letterID, digitID}
	ordinals := map[int]bool{}
	for _, edge := range closure.Edges {
		if edge.Caller != entryID || ordinals[edge.Ordinal] || edge.Ordinal < 0 || edge.Ordinal > 1 {
			return bad("call closure edge")
		}
		ordinals[edge.Ordinal] = true
		if edge.Callee != expectedCallIDs[edge.Ordinal] {
			return bad("call closure source ordinal")
		}
	}
	record := c.Plan.Records[0]
	n.Record = record.GoName
	if record.ID != replyID || !token.IsIdentifier(n.Record) || len(record.Fields) != 4 {
		return bad("record")
	}
	fields := map[string]string{}
	suffix := map[string]string{"in_rune_domain": "in-rune-domain", "can_start": "can-start", "can_continue": "can-continue", "is_decimal_digit": "is-decimal-digit"}
	for _, f := range record.Fields {
		if suffix[f.Name] == "" || f.ID != replyID+"/"+suffix[f.Name] || f.Type != "urn:gooo:type:boolean" || !token.IsIdentifier(f.GoName) || fields[f.Name] != "" {
			return bad("record field")
		}
		fields[f.Name] = f.GoName
	}
	n.InRuneDomain, n.CanStart, n.CanContinue, n.IsDecimalDigit = fields["in_rune_domain"], fields["can_start"], fields["can_continue"], fields["is_decimal_digit"]
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", c.Source, parser.ParseComments)
	if err != nil {
		return bad(err.Error())
	}
	if file.Name.Name != "main" || len(file.Imports) != 0 {
		return bad("package or imports")
	}
	functions := map[string]*ast.FuncDecl{}
	records := map[string]*ast.StructType{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if functions[d.Name.Name] != nil {
				return bad("duplicate function")
			}
			functions[d.Name.Name] = d
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				return bad("unexpected declaration")
			}
			for _, s := range d.Specs {
				typ, ok := s.(*ast.TypeSpec)
				if !ok {
					return bad("type declaration")
				}
				rec, ok := typ.Type.(*ast.StructType)
				if !ok || records[typ.Name.Name] != nil {
					return bad("record declaration")
				}
				records[typ.Name.Name] = rec
			}
		default:
			return bad("unexpected declaration")
		}
	}
	if len(functions) != 3 || len(records) != 1 {
		return bad("generated declaration roster")
	}
	for name, result := range map[string]string{n.Entry: n.Record, n.Letter: "bool", n.Digit: "bool"} {
		f := functions[name]
		if f == nil || f.Recv != nil || f.Type.TypeParams != nil || f.Type.Params == nil || len(f.Type.Params.List) != 1 || f.Type.Results == nil || len(f.Type.Results.List) != 1 {
			return bad("function signature")
		}
		in, out := f.Type.Params.List[0], f.Type.Results.List[0]
		if len(in.Names) != 1 || in.Names[0].Name != "input" || ident(in.Type) != "int64" || len(out.Names) != 0 || ident(out.Type) != result {
			return bad("function signature types")
		}
	}
	rec := records[n.Record]
	if rec == nil || len(rec.Fields.List) != 4 {
		return bad("record AST")
	}
	seen := map[string]bool{}
	for _, f := range rec.Fields.List {
		if len(f.Names) != 1 || ident(f.Type) != "bool" || f.Tag == nil {
			return bad("record field AST")
		}
		tag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			return bad("field tag")
		}
		key := reflect.StructTag(tag).Get("json")
		if fields[key] != f.Names[0].Name || seen[key] {
			return bad("field tag identity")
		}
		seen[key] = true
	}
	var called []string
	validArguments := true
	ast.Inspect(functions[n.Entry].Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			called = append(called, ident(call.Fun))
			if call.Ellipsis.IsValid() || len(call.Args) != 1 || ident(call.Args[0]) != "input" {
				validArguments = false
			}
		}
		return true
	})
	if !validArguments || !reflect.DeepEqual(called, []string{n.Letter, n.Digit}) {
		return bad("entry helper call slots or arguments")
	}
	return n, nil
}
func ident(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
func sameJSON(a, b []byte) bool {
	decode := func(raw []byte) (any, error) {
		var v any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		var trailing any
		if err := d.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("trailing JSON")
		}
		return v, nil
	}
	x, err := decode(a)
	if err != nil {
		return false
	}
	y, err := decode(b)
	return err == nil && reflect.DeepEqual(x, y)
}
