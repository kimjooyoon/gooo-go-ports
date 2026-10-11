package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

//go:embed ycbcr_exhaustive_test.go.txt
var ycbcrExhaustiveTemplate string

type ycbcrExhaustiveReport struct {
	Schema                   string `json:"schema"`
	GoVersion                string `json:"go_version"`
	Operation                string `json:"operation"`
	ActivityID               string `json:"activity_id"`
	OutputEntityID           string `json:"output_entity_id"`
	GeneratedSHA256          string `json:"generated_sha256"`
	InputMin                 int    `json:"input_min"`
	InputMax                 int    `json:"input_max"`
	InputArity               int    `json:"input_arity"`
	InputOrder               string `json:"input_order"`
	InputCount               int    `json:"input_count"`
	ComparisonCount          int    `json:"comparison_count"`
	ComponentComparisonCount int    `json:"component_comparison_count"`
	OutputSHA256             string `json:"output_sha256"`
	OracleSHA256             string `json:"oracle_sha256"`
	Mismatches               int    `json:"mismatches"`
}

type ycbcrOperation struct {
	Operation, Entity, Digest string
	Fields                    [3]string
}

func ycbcrOperationContract(operation string) (ycbcrOperation, error) {
	// The digests cover the three raw output bytes per tuple, ordered by
	// input0 * 65536 + input1 * 256 + input2, not JSON or host byte order.
	switch operation {
	case "RGBToYCbCr":
		return ycbcrOperation{operation, "gooo://go/image/color/ycbcr", "sha256:c7a38f2fd3b7dccee233499f58c95ca122dd4b833fe4e2908f4681386e643098", [3]string{"y", "cb", "cr"}}, nil
	case "YCbCrToRGB":
		return ycbcrOperation{operation, "gooo://go/image/color/rgb", "sha256:ad0d598e5b003c424acbc9e5d8a3cc8fe965d7f22ecc3903c425a929ba742682", [3]string{"red", "green", "blue"}}, nil
	default:
		return ycbcrOperation{}, fmt.Errorf("unsupported YCbCr exhaustive operation %q", operation)
	}
}

func expectedYCbCrExhaustive(generated, operation string) (ycbcrExhaustiveReport, error) {
	c, err := ycbcrOperationContract(operation)
	if err != nil {
		return ycbcrExhaustiveReport{}, err
	}
	return ycbcrExhaustiveReport{
		Schema: "gooo/ycbcr-exhaustive-native/v1", GoVersion: "go1.27.2", Operation: operation,
		ActivityID: "gooo://go/image/color/" + operation, OutputEntityID: c.Entity,
		GeneratedSHA256: generated, InputMin: 0, InputMax: 255, InputArity: 3,
		InputOrder: "input0-major,input1-middle,input2-minor",
		InputCount: 16777216, ComparisonCount: 16777216, ComponentComparisonCount: 50331648,
		OutputSHA256: c.Digest, OracleSHA256: c.Digest,
	}, nil
}

type ycbcrGeneratedABI struct {
	Function, Record string
	Fields           [3]string
}

// checkYCbCrExhaustive continues an already validated finite execution. Its
// caller MUST first validate execute/replay, workspace/source and producer
// bindings with inspect and inspectReplayBinding, then supply checked.Generated.
// This covers all 256^3 byte triples for one conversion, not images, codecs,
// out-of-domain int64 inputs, browser execution or lossless round trips.
func checkYCbCrExhaustive(ctx context.Context, dir string, raw []byte, expectedGenerated, operation string) (ycbcrExhaustiveReport, error) {
	var zero ycbcrExhaustiveReport
	if runtime.Version() != "go1.27.2" {
		return zero, fmt.Errorf("YCbCr exhaustive check requires Go 1.27.2")
	}
	source, abi, err := inspectYCbCrGenerated(raw, expectedGenerated, operation)
	if err != nil {
		return zero, err
	}
	harness, err := makeYCbCrExhaustiveHarness(abi, expectedGenerated, operation)
	if err != nil {
		return zero, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return zero, err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return zero, fmt.Errorf("YCbCr requires a fresh evidence directory: %w", err)
	}
	native := filepath.Join(dir, "native")
	for _, path := range []string{native, filepath.Join(dir, "cache"), filepath.Join(dir, "tmp"), filepath.Join(dir, "gopath")} {
		if err := os.Mkdir(path, 0755); err != nil {
			return zero, err
		}
	}
	for name, content := range map[string][]byte{
		"go.mod":       []byte("module gooo-ycbcr-exhaustive\n\ngo 1.27.2\n"),
		"generated.go": []byte(source), "exhaustive_test.go": harness,
	} {
		if err := os.WriteFile(filepath.Join(native, name), content, 0644); err != nil {
			return zero, err
		}
	}
	generatedPath := filepath.Join(native, "generated.go")
	if err := checkYCbCrSourceFile(generatedPath, expectedGenerated); err != nil {
		return zero, err
	}
	reportPath := filepath.Join(dir, "exhaustive.json")
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-timeout=90s", "-run", "^TestYCbCrExhaustive$", "-v", ".")
	cmd.Dir, cmd.Env = native, ycbcrGoEnvironment(dir, reportPath)
	output, runErr := cmd.CombinedOutput()
	if err := os.WriteFile(filepath.Join(dir, "native-test.txt"), output, 0644); err != nil {
		return zero, err
	}
	if err := checkYCbCrSourceFile(generatedPath, expectedGenerated); err != nil {
		return zero, err
	}
	if runErr != nil {
		return zero, fmt.Errorf("%s native exhaustive test: %w: %s", operation, runErr, output)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return zero, fmt.Errorf("YCbCr exhaustive report: %w", err)
	}
	return inspectYCbCrExhaustiveReport(report, expectedGenerated, operation)
}

func checkYCbCrSourceFile(path, want string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if contentDigest(raw) != want {
		return fmt.Errorf("YCbCr generated source changed on disk")
	}
	return nil
}

func inspectYCbCrGenerated(raw []byte, expectedGenerated, operation string) (string, ycbcrGeneratedABI, error) {
	var zero ycbcrGeneratedABI
	contract, err := ycbcrOperationContract(operation)
	if err != nil {
		return "", zero, err
	}
	var receipt struct {
		Result struct {
			Composition struct {
				Stage, Source string
				Generated     string `json:"generated_sha256"`
				Plan          struct {
					Schema     string
					Activities []struct {
						ID         string
						Function   string `json:"go_function"`
						Input      string `json:"input_entity_id"`
						Output     string `json:"output_entity_id"`
						InputFrom  int    `json:"input_from"`
						Assembling bool
						Inputs     []struct {
							Port, Type string
							Entity     string `json:"entity_id"`
							From       int
						}
					}
					Edges       []json.RawMessage
					RecordTypes []struct {
						ID     string
						GoName string `json:"go_name"`
						Fields []struct {
							Name, ID string
							TypeID   string `json:"type_id"`
							GoName   string `json:"go_name"`
						}
					} `json:"record_types"`
				} `json:"plan"`
			} `json:"composition"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return "", zero, err
	}
	c := receipt.Result.Composition
	if !validDigest(expectedGenerated) || c.Stage != "COMPLETE" || c.Generated != expectedGenerated || c.Source == "" || contentDigest([]byte(c.Source)) != expectedGenerated {
		return "", zero, fmt.Errorf("YCbCr checked generated-source binding differs")
	}
	if c.Plan.Schema != "gooo/body-composition-plan/v1" || len(c.Plan.Activities) != 1 || len(c.Plan.Edges) != 0 {
		return "", zero, fmt.Errorf("YCbCr requires one independent planned activity")
	}
	a := c.Plan.Activities[0]
	if a.ID != "gooo://go/image/color/"+operation || a.Input != integerEntity || a.Output != contract.Entity || !ycbcrSymbol(a.Function) || a.Function == "main" || a.Function == "init" || a.Function == "TestYCbCrExhaustive" || a.InputFrom != -1 || a.Assembling || len(a.Inputs) != 3 {
		return "", zero, fmt.Errorf("YCbCr planned activity identity or ABI differs")
	}
	for i, input := range a.Inputs {
		if input.Port != fmt.Sprintf("input%d", i) || input.Type != "Integer" || input.Entity != integerEntity || input.From != -1 {
			return "", zero, fmt.Errorf("YCbCr planned integer input binding differs")
		}
	}
	abi := ycbcrGeneratedABI{Function: a.Function}
	seenRecords, seenNames := map[string]bool{}, map[string]bool{}
	for _, record := range c.Plan.RecordTypes {
		if record.ID == "" || seenRecords[record.ID] || !ycbcrExportedSymbol(record.GoName) || seenNames[record.GoName] {
			return "", zero, fmt.Errorf("YCbCr record roster contains missing or duplicated identity")
		}
		seenRecords[record.ID], seenNames[record.GoName] = true, true
		if record.ID != contract.Entity {
			continue
		}
		abi.Record = record.GoName
		if len(record.Fields) != 3 {
			return "", zero, fmt.Errorf("YCbCr output record must contain exactly three components")
		}
		seenFields, seenSymbols := map[string]bool{}, map[string]bool{}
		for _, field := range record.Fields {
			if seenFields[field.Name] || seenSymbols[field.GoName] || !ycbcrExportedSymbol(field.GoName) || field.TypeID != "urn:gooo:type:integer" || field.ID != contract.Entity+"/"+field.Name {
				return "", zero, fmt.Errorf("YCbCr output component identity, type or symbol differs")
			}
			seenFields[field.Name], seenSymbols[field.GoName] = true, true
			found := false
			for i, name := range contract.Fields {
				if field.Name == name {
					abi.Fields[i], found = field.GoName, true
				}
			}
			if !found {
				return "", zero, fmt.Errorf("YCbCr output field roster differs")
			}
		}
	}
	if abi.Record == "" {
		return "", zero, fmt.Errorf("YCbCr output record binding is missing")
	}
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", c.Source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", zero, err
	}
	if f.Name.Name != "main" || len(f.Imports) != 0 {
		return "", zero, fmt.Errorf("YCbCr generated source must be package main without imports")
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:") || strings.HasPrefix(comment.Text, "//line ") || strings.HasPrefix(comment.Text, "/*line ") {
				return "", zero, fmt.Errorf("YCbCr generated source contains compiler directives")
			}
		}
	}
	functions, targetRecords := 0, 0
	declarations := map[string]bool{}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			functions++
			if d.Name.Name != abi.Function || d.Recv != nil || d.Body == nil || d.Type.TypeParams != nil || !ycbcrThreeInt64Inputs(d.Type.Params) || !ycbcrRecordResult(d.Type.Results, abi.Record) || declarations[d.Name.Name] {
				return "", zero, fmt.Errorf("YCbCr generated function must have the exact three-int64 to record signature")
			}
			declarations[d.Name.Name] = true
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				return "", zero, fmt.Errorf("YCbCr generated source contains a non-record declaration")
			}
			for _, spec := range d.Specs {
				t, ok := spec.(*ast.TypeSpec)
				if !ok || t.Assign.IsValid() || t.TypeParams != nil || !ycbcrExportedSymbol(t.Name.Name) || declarations[t.Name.Name] {
					return "", zero, fmt.Errorf("YCbCr generated record declaration differs")
				}
				declarations[t.Name.Name] = true
				s, ok := t.Type.(*ast.StructType)
				if !ok || !ycbcrInertRecord(s) {
					return "", zero, fmt.Errorf("YCbCr generated type is not an inert primitive record")
				}
				if t.Name.Name == abi.Record {
					targetRecords++
					if !ycbcrOutputRecord(s, abi, contract) {
						return "", zero, fmt.Errorf("YCbCr generated output record fields, types or JSON tags differ")
					}
				}
			}
		default:
			return "", zero, fmt.Errorf("YCbCr generated declaration differs")
		}
	}
	if functions != 1 || targetRecords != 1 {
		return "", zero, fmt.Errorf("YCbCr generated function or output record roster differs")
	}
	return c.Source, abi, nil
}

func ycbcrSymbol(s string) bool         { return token.IsIdentifier(s) && s != "_" }
func ycbcrExportedSymbol(s string) bool { return ycbcrSymbol(s) && ast.IsExported(s) }

func ycbcrThreeInt64Inputs(fields *ast.FieldList) bool {
	if fields == nil || len(fields.List) != 3 {
		return false
	}
	seen := map[string]bool{}
	for _, field := range fields.List {
		t, ok := field.Type.(*ast.Ident)
		if !ok || t.Name != "int64" || len(field.Names) != 1 || !ycbcrSymbol(field.Names[0].Name) || seen[field.Names[0].Name] {
			return false
		}
		seen[field.Names[0].Name] = true
	}
	return true
}

func ycbcrRecordResult(fields *ast.FieldList, name string) bool {
	if fields == nil || len(fields.List) != 1 || len(fields.List[0].Names) != 0 {
		return false
	}
	t, ok := fields.List[0].Type.(*ast.Ident)
	return ok && t.Name == name
}

func ycbcrInertRecord(s *ast.StructType) bool {
	if s.Fields == nil {
		return false
	}
	seen := map[string]bool{}
	for _, field := range s.Fields.List {
		t, ok := field.Type.(*ast.Ident)
		if !ok || (t.Name != "int64" && t.Name != "bool" && t.Name != "string") || len(field.Names) != 1 || !ycbcrExportedSymbol(field.Names[0].Name) || seen[field.Names[0].Name] {
			return false
		}
		seen[field.Names[0].Name] = true
	}
	return true
}

func ycbcrOutputRecord(s *ast.StructType, abi ycbcrGeneratedABI, c ycbcrOperation) bool {
	if len(s.Fields.List) != 3 {
		return false
	}
	for _, field := range s.Fields.List {
		if field.Type.(*ast.Ident).Name != "int64" || field.Tag == nil {
			return false
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return false
		}
		found := false
		for i, name := range abi.Fields {
			if field.Names[0].Name == name && tag == `json:"`+c.Fields[i]+`"` {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func makeYCbCrExhaustiveHarness(abi ycbcrGeneratedABI, generated, operation string) ([]byte, error) {
	contract, err := ycbcrOperationContract(operation)
	if err != nil {
		return nil, err
	}
	t, err := template.New("ycbcr").Parse(ycbcrExhaustiveTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, struct {
		ycbcrGeneratedABI
		Operation, Entity, Generated string
	}{abi, operation, contract.Entity, generated})
	return out.Bytes(), err
}

func ycbcrGoEnvironment(dir, report string) []string {
	// Reuse the hermetic Go environment, replacing its unrelated report variable.
	env := []string{}
	for _, entry := range simpleFoldGoEnvironment(dir, "") {
		if !strings.HasPrefix(entry, "GOOO_SIMPLE_FOLD_REPORT=") {
			env = append(env, entry)
		}
	}
	return append(env, "GOOO_YCBCR_REPORT="+report)
}

func inspectYCbCrExhaustiveReport(raw []byte, generated, operation string) (ycbcrExhaustiveReport, error) {
	var got ycbcrExhaustiveReport
	want, err := expectedYCbCrExhaustive(generated, operation)
	if err != nil {
		return got, err
	}
	wantRaw, _ := json.Marshal(want)
	var required map[string]json.RawMessage
	_ = json.Unmarshal(wantRaw, &required)
	// Every field is mandatory, including zero counters; reject duplicate keys,
	// absent/null values and unknown claims rather than trusting a PASS label.
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return got, fmt.Errorf("YCbCr report must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return got, err
		}
		name, ok := key.(string)
		if !ok || required[name] == nil || seen[name] {
			return got, fmt.Errorf("YCbCr report field roster differs")
		}
		seen[name] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return got, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return got, fmt.Errorf("YCbCr report field is null")
		}
	}
	if _, err := d.Token(); err != nil {
		return got, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF || len(seen) != len(required) {
		return got, fmt.Errorf("YCbCr report is incomplete or has trailing data")
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return got, err
	}
	if !validDigest(generated) || !reflect.DeepEqual(got, want) {
		return got, fmt.Errorf("YCbCr report identity, coverage, output digests or counters differ")
	}
	return got, nil
}
