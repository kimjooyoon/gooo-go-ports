package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type tutorialInput struct {
	Score int64
	Label string
}

func tutorialRows(raw []byte) (rows []tutorialInput, err error) {
	defer func() {
		if recover() != nil {
			rows = nil
			err = fmt.Errorf("invalid tutorial inputs")
		}
	}()
	v, err := cropDecode(raw)
	if err != nil {
		return nil, err
	}
	m := cropMap(v)
	if len(m) != 2 || m["schema"] != "gooo/body-composition-inputs/v1" {
		return nil, fmt.Errorf("invalid tutorial input schema")
	}
	for _, value := range cropList(m["inputs"]) {
		r := cropMap(value)
		if len(r) != 2 {
			return nil, fmt.Errorf("invalid tutorial input fields")
		}
		n, ok := r["app/report:Describe.input0"].(json.Number)
		if !ok {
			return nil, fmt.Errorf("integer input required")
		}
		score, err := n.Int64()
		if err != nil {
			return nil, err
		}
		label, ok := r["app/report:Describe.input1"].(string)
		if !ok || len(label) > 1024 {
			return nil, fmt.Errorf("native Text required")
		}
		rows = append(rows, tutorialInput{score, label})
	}
	if len(rows) == 0 || len(rows) > 128 {
		return nil, fmt.Errorf("invalid input row count")
	}
	return rows, nil
}
func tutorialFields(v map[string]any) []any {
	fields := make([]any, 4)
	for i, name := range tutorialNames {
		fields[i] = map[string]any{"id": tutorialIDs[i], "name": name, "value": v[name]}
	}
	return fields
}
func tutorialLayout() map[string]any {
	fields := make([]any, 4)
	for i, name := range tutorialNames {
		kind := "integer"
		if i == 2 {
			kind = "boolean"
		}
		if i == 3 {
			kind = "string"
		}
		fields[i] = map[string]any{"name": name, "id": tutorialIDs[i], "type_id": "urn:gooo:type:" + kind, "go_name": cropGoName("Field", tutorialIDs[i])}
	}
	return map[string]any{"name": "Summary", "id": tutorialResult, "go_name": cropGoName("Record", tutorialResult), "fields": fields}
}
func inspectTutorialReceipt(raw []byte, lock lockfile, manifestRaw, inputsRaw []byte, sources map[string][]byte, prior string) (source string, err error) {
	defer func() {
		if recover() != nil {
			source = ""
			err = fmt.Errorf("invalid tutorial receipt shape")
		}
	}()
	fail := func(reason string) (string, error) { return "", fmt.Errorf("invalid tutorial evidence: %s", reason) }
	value, err := cropDecode(raw)
	if err != nil {
		return "", err
	}
	e := cropMap(value)
	rows, err := tutorialRows(inputsRaw)
	if err != nil {
		return "", err
	}
	if e["schema"] != "gooo/workspace-body-execution-receipt/v1" || e["decision"] != "OBSERVED" || e["error"] != nil && e["error"] != "" || e["manifest_digest"] != "sha256:"+digest(manifestRaw) || e["inputs_digest"] != "sha256:"+digest(inputsRaw) || e["cases_digest"] != nil {
		return fail("receipt identity or input binding")
	}
	if prior == "" {
		if e["replayed_from_sha256"] != nil {
			return fail("unexpected replay predecessor")
		}
	} else if e["replayed_from_sha256"] != prior {
		return fail("replay predecessor")
	}
	r := cropMap(e["result"])
	p := cropMap(r["program"])
	c := cropMap(r["composition"])
	run := cropMap(r["runtime"])
	entry := cropMap(p["entry"])
	lowered, ok := entry["lowered_name"].(string)
	plan := cropMap(c["plan"])
	if entry["package_path"] != "app/report" || entry["activity"] != "Describe" || !ok || !token.IsIdentifier(lowered) || plan["entry_activity"] != lowered {
		return fail("entry identity")
	}
	activities := cropList(plan["activities"])
	if len(activities) != 1 {
		return fail("entry roster")
	}
	activity := cropMap(activities[0])
	if activity["name"] != lowered || activity["id"] != tutorialActivity || activity["go_function"] != "GoooComposedActivity0" || activity["output_entity_id"] != tutorialResult {
		return fail("lowered entry binding")
	}
	expectedSlots := []any{map[string]any{"port": "input0", "type": "Integer", "entity_id": "urn:gooo:type:integer", "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": "urn:gooo:type:string", "from": -1}}
	if !cropEqual(activity["inputs"], expectedSlots) || !cropEqual(plan["record_types"], []any{tutorialLayout()}) {
		return fail("input or record layout")
	}
	packages := cropList(cropMap(p["workspace_image"])["packages"])
	if len(packages) != 2 {
		return fail("package roster")
	}
	seen := map[string]bool{}
	for _, value := range packages {
		pkg := cropMap(value)
		path := pkg["path"].(string)
		filename, ok := tutorialSources[path]
		if !ok || seen[path] {
			return fail("package identity")
		}
		seen[path] = true
		srcs := cropList(pkg["sources"])
		if len(srcs) != 1 {
			return fail("source roster")
		}
		src := cropMap(srcs[0])
		encoded, _ := json.Marshal(string(sources[filename]))
		if len(sources[filename]) == 0 || src["filename"] != filename || src["source_digest"] != "sha256:"+digest(encoded) {
			return fail("source binding")
		}
		if path == "app/report" {
			matched := 0
			for _, x := range cropList(pkg["exports"]) {
				ex := cropMap(x)
				if ex["name"] != "Describe" && ex["id"] != tutorialActivity {
					continue
				}
				matched++
				if ex["name"] != "Describe" || ex["kind"] != "activity" || ex["id"] != tutorialActivity || ex["output_type"] != tutorialResult || !cropEqual(ex["input_types"], []string{"urn:gooo:type:integer", "urn:gooo:type:string"}) {
					return fail("entry export signature")
				}
			}
			if matched != 1 {
				return fail("entry export roster")
			}
		}
	}
	source, ok = c["source"].(string)
	if !ok || source == "" || c["stage"] != "COMPLETE" || c["generated_sha256"] != "sha256:"+digest([]byte(source)) || run["generated_sha256"] != c["generated_sha256"] {
		return fail("generated source binding")
	}
	loweredSource, ok := p["lowered_gooo_source"].(string)
	if !ok || loweredSource == "" || loweredSource != c["gooo_source"] {
		return fail("lowered source mismatch")
	}
	loweredDigest := "sha256:" + digest([]byte(loweredSource))
	for _, key := range []string{"original_source_sha256", "selected_source_sha256"} {
		if c[key] != loweredDigest || run[key] != loweredDigest {
			return fail("lowered source digest binding")
		}
	}
	steps := cropList(c["steps"])
	if len(steps) != 1 {
		return fail("generation roster")
	}
	report := cropMap(cropMap(cropMap(steps[0])["generation"])["report"])
	if report["activity_id"] != tutorialActivity || report["compiler_source_sha"] != lock.CompilerRevision {
		return fail("generation identity")
	}
	version, ok := run["go_version"].(string)
	parts := strings.Fields(version)
	if !ok || len(parts) != 4 || parts[0] != "go" || parts[1] != "version" || parts[2] != lock.GoVersion || run["schema"] != "gooo/body-composition-runtime/v1" || run["stage"] != "COMPLETE" || run["failure"] != nil || run["producer_source_sha"] != lock.CompilerRevision || run["projection_replayed"] != true || run["runtime_replayed"] != true {
		return fail("runtime identity or completion")
	}
	for _, key := range []string{"finite_total", "finite_passed", "model_calls"} {
		if run[key] != json.Number("0") {
			return fail("missing or nonzero runtime counter")
		}
	}
	if prior == "" {
		if r["replay"] != nil {
			return fail("unexpected replay metadata")
		}
	} else if cropMap(r["replay"])["model_calls"] != json.Number("0") {
		return fail("replay model calls")
	}
	traces := cropList(run["traces"])
	if len(traces) != len(rows) {
		return fail("trace count")
	}
	for i, value := range traces {
		trace := cropMap(value)
		ds := cropList(trace["deliveries"])
		if trace["case_index"] != json.Number(strconv.Itoa(i)) || len(ds) != 1 {
			return fail("trace order or deliveries")
		}
		d := cropMap(ds[0])
		inputs := []any{map[string]any{"port": "input0", "entity_id": "urn:gooo:type:integer", "value": rows[i].Score}, map[string]any{"port": "input1", "entity_id": "urn:gooo:type:string", "value": rows[i].Label}}
		want := tutorialOracle(rows[i].Score, rows[i].Label)
		if len(d) != 4 || d["activity_id"] != tutorialActivity || !cropEqual(d["inputs"], inputs) || !cropEqual(d["actual"], want) || !cropEqual(d["actual_fields"], tutorialFields(want)) {
			return fail("original input, output or field identity")
		}
	}
	if err := inspectTutorialGoABI(source); err != nil {
		return "", err
	}
	return source, nil
}
func inspectTutorialGoABI(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil || f.Name.Name != "main" {
		return fmt.Errorf("invalid tutorial generated Go: %v", err)
	}
	record, entry := false, false
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "GoooComposedActivity0" {
			if entry || fn.Recv != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 2 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("tutorial entry signature changed")
			}
			for i, field := range fn.Type.Params.List {
				typ, ok := field.Type.(*ast.Ident)
				want := []string{"int64", "string"}[i]
				if !ok || typ.Name != want || len(field.Names) != 1 || field.Names[0].Name != fmt.Sprintf("input%d", i) {
					return fmt.Errorf("tutorial input type changed")
				}
			}
			result, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || result.Name != cropGoName("Record", tutorialResult) {
				return fmt.Errorf("tutorial result type changed")
			}
			entry = true
		}
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, v := range g.Specs {
			t := v.(*ast.TypeSpec)
			if t.Name.Name != cropGoName("Record", tutorialResult) {
				continue
			}
			s, ok := t.Type.(*ast.StructType)
			if !ok || record || len(s.Fields.List) != 4 {
				return fmt.Errorf("tutorial record changed")
			}
			record = true
			for i, field := range s.Fields.List {
				typ, ok := field.Type.(*ast.Ident)
				if !ok || typ.Name != []string{"int64", "int64", "bool", "string"}[i] || len(field.Names) != 1 || field.Names[0].Name != cropGoName("Field", tutorialIDs[i]) || field.Tag == nil {
					return fmt.Errorf("tutorial field ABI changed")
				}
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil || tag != `json:"`+tutorialNames[i]+`"` {
					return fmt.Errorf("tutorial JSON field changed")
				}
			}
		}
	}
	if !record || !entry {
		return fmt.Errorf("tutorial ABI incomplete")
	}
	return nil
}
func tutorialRejectedCommand(ctx context.Context, input string) ([]byte, error) {
	c := exec.CommandContext(ctx, "./.gooo", "package", "execute", "--json", "--inputs", input, tutorialManifest)
	c.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "PATH="+filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	return c.Output()
}
func inspectTutorialRejection(raw, manifest, inputs []byte, name string, lock lockfile, generated string) error {
	var e struct {
		Schema, Decision, Error string
		Manifest                string `json:"manifest_digest"`
		Inputs                  string `json:"inputs_digest"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return fmt.Errorf("native %s rejection did not produce JSON receipt: %w", name, err)
	}
	if e.Schema != "gooo/workspace-body-execution-receipt/v1" || e.Decision != "FAIL_CLOSED" || e.Manifest != "sha256:"+digest(manifest) || e.Inputs != "sha256:"+digest(inputs) || !strings.Contains(e.Error, "Text requires a string of at most 1024 UTF-8 bytes") {
		return fmt.Errorf("native %s rejected for unexpected reason: %s", name, e.Error)
	}
	var decoded map[string]any
	value, err := cropDecode(raw)
	if err != nil {
		return err
	}
	decoded, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid rejection envelope")
	}
	if name != "input-1025" {
		result, ok := decoded["result"].(map[string]any)
		if !ok {
			return fmt.Errorf("missing rejected runtime")
		}
		run, ok := result["runtime"].(map[string]any)
		if !ok || run["stage"] != "EXECUTE_1" || run["producer_source_sha"] != lock.CompilerRevision || run["generated_sha256"] != "sha256:"+digest([]byte(generated)) || run["model_calls"] != json.Number("0") {
			return fmt.Errorf("invalid rejected runtime identity")
		}
	} else if decoded["result"] != nil {
		return fmt.Errorf("input rejection unexpectedly executed")
	}
	if name == "input-1025" {
		if strings.Contains(e.Error, "runtime output") {
			return fmt.Errorf("input limit rejection occurred at output")
		}
	} else if !strings.Contains(e.Error, `runtime output: record "Summary" field "label"`) {
		return fmt.Errorf("output limit rejected before generated output")
	}
	return nil
}
