package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTutorialManualAndTransportBoundaries(t *testing.T) {
	f := tutorialFixtureSet(lockfile{})
	want := []tutorialOutput{{"0", "100", false, "sensor: low"}, {"42", "58", true, "sensor: low"}, {"75", "25", true, "sensor: high"}, {"100", "0", false, "sensor: high"}}
	for i, w := range want {
		if f.Cases[i].Expected != w {
			t.Fatalf("manual row %d", i)
		}
	}
	raw, err := tutorialInputDocument(f.Cases[4:])
	if err != nil || len(raw) > 32<<10 {
		t.Fatal("boundary envelope", err, len(raw))
	}
	rows, err := tutorialRows(raw)
	if err != nil || len(rows) != len(f.Cases)-4 {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	scores := map[string]bool{}
	for i, row := range f.Cases {
		if seen[row.Name] {
			t.Fatal("duplicate case name")
		}
		seen[row.Name] = true
		scores[row.Inputs[0]] = true
		if row.LabelUTF16 == nil {
			t.Fatal("UTF-16 units must be an array")
		}
		if len(row.Expected.Label) > 1024 {
			t.Fatal("successful output exceeds native limit")
		}
		if i >= 4 && (rows[i-4].Label != row.Inputs[1] || strconv.FormatInt(rows[i-4].Score, 10) != row.Inputs[0]) {
			t.Fatal("native canonical input differs")
		}
	}
	for _, name := range []string{"lone-high", "lone-low", "pair", "mixed-surrogates", "high-high-low", "high-text-1018"} {
		if !seen[name] {
			t.Fatal("missing", name)
		}
	}
	for _, score := range []string{"-9223372036854775808", "9223372036854775807", "9007199254740993", "-9007199254740993", "0", "59", "60", "100", "101"} {
		if !scores[score] {
			t.Fatal("missing integer", score)
		}
	}
	if !strings.Contains(string(raw), `\ud800`) {
		t.Fatal("raw escaped native surrogate lost")
	}
	documented, err := tutorialRows(cropTestRead(t, tutorialInputs))
	if err != nil || len(documented) != 4 {
		t.Fatal(err)
	}
	for i, row := range documented {
		if row.Score != []int64{-5, 42, 75, 105}[i] || row.Label != "sensor" {
			t.Fatal("manual sample changed")
		}
	}
	// The low 1019-byte input succeeds, while a high 1019-byte input fails only
	// after the real generated label exceeds the runtime's output Text limit.
	low := strings.Repeat("a", 1019)
	if len(tutorialExpected(42, low).Label) != 1024 || len(tutorialExpected(60, low).Label) != 1025 {
		t.Fatal("asymmetric output boundary lost")
	}
}
func TestTutorialRowsRejectMalformedBindings(t *testing.T) {
	cases := []string{`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/report:Describe.input0":9223372036854775808,"app/report:Describe.input1":"x"}]}`, `{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/report:Describe.input0":1.0,"app/report:Describe.input1":"x"}]}`, `{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/report:Describe.input0":1,"app/report:Describe.input1":null}]}`, `{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/report:Describe.input0":1,"app/report:Describe.input0":2,"app/report:Describe.input1":"x"}]}`, `{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/report:Describe.input0":1}]}`}
	for _, raw := range cases {
		if _, err := tutorialRows([]byte(raw)); err == nil {
			t.Fatal("accepted malformed inputs", raw)
		}
	}
}

// Synthetic evidence checks are negative tests only; tools/tutorial-browser-
// smoke.cjs additionally checks the actual native receipts and shipped Wasm.
func tutorialEvidenceFixture(t *testing.T) ([]byte, lockfile, []byte, []byte, map[string][]byte) {
	t.Helper()
	lock := lockfile{CompilerRevision: strings.Repeat("a", 40), GoVersion: "go1.27.2"}
	manifest, inputs := cropTestRead(t, tutorialManifest), cropTestRead(t, tutorialInputs)
	rows, err := tutorialRows(inputs)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	packages := []any{}
	for pkg, name := range tutorialSources {
		raw := cropTestRead(t, name)
		sources[name] = raw
		encoded, _ := json.Marshal(string(raw))
		p := map[string]any{"path": pkg, "sources": []any{map[string]any{"filename": name, "source_digest": "sha256:" + digest(encoded)}}}
		if pkg == "app/report" {
			p["exports"] = []any{map[string]any{"name": "Describe", "id": tutorialActivity, "kind": "activity", "input_types": []string{"urn:gooo:type:integer", "urn:gooo:type:string"}, "output_type": tutorialResult}}
		}
		packages = append(packages, p)
	}
	source := "package main\n// preserve bytes: 한글  \n"
	layout := tutorialLayout()
	source += "type " + layout["go_name"].(string) + " struct {\n"
	for i, v := range cropList(layout["fields"]) {
		field := cropMap(v)
		source += fmt.Sprintf("%s %s `json:%q`\n", field["go_name"], []string{"int64", "int64", "bool", "string"}[i], field["name"])
	}
	source += "}\n"
	source += fmt.Sprintf("func GoooComposedActivity0(input0 int64,input1 string)%s{return %s{}}\n", layout["go_name"], layout["go_name"])
	traces := []any{}
	for i, row := range rows {
		want := tutorialOracle(row.Score, row.Label)
		traces = append(traces, map[string]any{"case_index": i, "deliveries": []any{map[string]any{"activity_id": tutorialActivity, "inputs": []any{map[string]any{"port": "input0", "entity_id": "urn:gooo:type:integer", "value": row.Score}, map[string]any{"port": "input1", "entity_id": "urn:gooo:type:string", "value": row.Label}}, "actual": want, "actual_fields": tutorialFields(want)}}})
	}
	sha := "sha256:" + digest([]byte(source))
	lowered := "GoooPackage2ActivityDescribe"
	raw := testJSON(t, map[string]any{"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest_digest": "sha256:" + digest(manifest), "inputs_digest": "sha256:" + digest(inputs), "result": map[string]any{
		"program":     map[string]any{"entry": map[string]any{"package_path": "app/report", "activity": "Describe", "lowered_name": lowered}, "workspace_image": map[string]any{"packages": packages}, "lowered_gooo_source": "fixture lowered source"},
		"composition": map[string]any{"stage": "COMPLETE", "gooo_source": "fixture lowered source", "original_source_sha256": "sha256:" + digest([]byte("fixture lowered source")), "selected_source_sha256": "sha256:" + digest([]byte("fixture lowered source")), "source": source, "generated_sha256": sha, "plan": map[string]any{"entry_activity": lowered, "record_types": []any{layout}, "activities": []any{map[string]any{"name": lowered, "id": tutorialActivity, "go_function": "GoooComposedActivity0", "output_entity_id": tutorialResult, "inputs": []any{map[string]any{"port": "input0", "type": "Integer", "entity_id": "urn:gooo:type:integer", "from": -1}, map[string]any{"port": "input1", "type": "Text", "entity_id": "urn:gooo:type:string", "from": -1}}}}}, "steps": []any{map[string]any{"generation": map[string]any{"report": map[string]any{"activity_id": tutorialActivity, "compiler_source_sha": lock.CompilerRevision}}}}},
		"runtime":     map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "original_source_sha256": "sha256:" + digest([]byte("fixture lowered source")), "selected_source_sha256": "sha256:" + digest([]byte("fixture lowered source")), "producer_source_sha": lock.CompilerRevision, "go_version": "go version go1.27.2 darwin/arm64", "generated_sha256": sha, "model_calls": 0, "finite_total": 0, "finite_passed": 0, "projection_replayed": true, "runtime_replayed": true, "traces": traces},
	}})
	return raw, lock, manifest, inputs, sources
}
func TestTutorialEvidenceOriginalAndSavedReplay(t *testing.T) {
	raw, lock, manifest, inputs, sources := tutorialEvidenceFixture(t)
	got, err := inspectTutorialReceipt(raw, lock, manifest, inputs, sources, "")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := cropDecode(raw)
	e := cropMap(v)
	r := cropMap(e["result"])
	if got != cropMap(r["composition"])["source"] {
		t.Fatal("source edited")
	}
	prior := "sha256:" + digest(raw)
	e["replayed_from_sha256"] = prior
	r["replay"] = map[string]any{"model_calls": 0}
	replayed, err := inspectTutorialReceipt(testJSON(t, e), lock, manifest, inputs, sources, prior)
	if err != nil || replayed != got {
		t.Fatal(err)
	}
}
func TestTutorialEvidenceRejectsTampering(t *testing.T) {
	for _, change := range []string{"input", "input-type", "input-order", "label", "field-id", "field-name", "field-order", "actual", "source", "entry", "export", "producer", "generation", "generated", "missing-calls", "null-calls", "nonzero-calls", "fresh-replay", "trace-index", "missing-field", "lowered", "missing-lowered", "lowered-digest", "ABI", "replay-prior", "replay-missing-calls"} {
		t.Run(change, func(t *testing.T) {
			raw, lock, manifest, inputs, sources := tutorialEvidenceFixture(t)
			v, _ := cropDecode(raw)
			e := cropMap(v)
			r := cropMap(e["result"])
			run := cropMap(r["runtime"])
			p := cropMap(r["program"])
			c := cropMap(r["composition"])
			tr := cropMap(cropList(run["traces"])[0])
			d := cropMap(cropList(tr["deliveries"])[0])
			in := cropList(d["inputs"])
			fields := cropList(d["actual_fields"])
			prior := ""
			switch change {
			case "input":
				cropMap(in[0])["value"] = json.Number("-4")
			case "input-type":
				cropMap(in[0])["entity_id"] = "wrong"
			case "input-order":
				in[0], in[1] = in[1], in[0]
			case "label":
				cropMap(in[1])["value"] = " sensor"
			case "field-id":
				cropMap(fields[3])["id"] = "wrong"
			case "field-name":
				cropMap(fields[3])["name"] = "wrong"
			case "field-order":
				fields[0], fields[1] = fields[1], fields[0]
			case "actual":
				cropMap(d["actual"])["label"] = "sensor: high"
			case "source":
				pkg := cropMap(cropList(cropMap(p["workspace_image"])["packages"])[0])
				cropMap(cropList(pkg["sources"])[0])["source_digest"] = "sha256:" + strings.Repeat("0", 64)
			case "entry":
				cropMap(p["entry"])["activity"] = "Other"
			case "export":
				for _, pkg := range cropList(cropMap(p["workspace_image"])["packages"]) {
					m := cropMap(pkg)
					if m["path"] == "app/report" {
						cropMap(cropList(m["exports"])[0])["input_types"] = []string{"urn:gooo:type:string", "urn:gooo:type:integer"}
					}
				}
			case "producer":
				run["producer_source_sha"] = "wrong"
			case "generation":
				cropMap(cropMap(cropMap(cropList(c["steps"])[0])["generation"])["report"])["compiler_source_sha"] = "wrong"
			case "generated":
				c["source"] = c["source"].(string) + "\n"
			case "missing-calls":
				delete(run, "model_calls")
			case "null-calls":
				run["model_calls"] = nil
			case "nonzero-calls":
				run["model_calls"] = json.Number("1")
			case "fresh-replay":
				r["replay"] = map[string]any{"model_calls": 0}
			case "trace-index":
				tr["case_index"] = json.Number("1")
			case "missing-field":
				d["actual_fields"] = fields[:3]
			case "lowered":
				p["lowered_gooo_source"] = "different"
			case "missing-lowered":
				delete(p, "lowered_gooo_source")
				delete(c, "gooo_source")
			case "lowered-digest":
				run["selected_source_sha256"] = "wrong"
			case "ABI":
				src := strings.Replace(c["source"].(string), "input0 int64", "input0 int32", 1)
				c["source"] = src
				c["generated_sha256"] = "sha256:" + digest([]byte(src))
				run["generated_sha256"] = c["generated_sha256"]
			case "replay-prior":
				prior = "sha256:" + digest(raw)
				e["replayed_from_sha256"] = "wrong"
				r["replay"] = map[string]any{"model_calls": 0}
			case "replay-missing-calls":
				prior = "sha256:" + digest(raw)
				e["replayed_from_sha256"] = prior
				r["replay"] = map[string]any{}
			}
			if _, err := inspectTutorialReceipt(testJSON(t, e), lock, manifest, inputs, sources, prior); err == nil {
				t.Fatal("accepted tamper")
			}
		})
	}
}
func TestTutorialSourceDownloadIdentity(t *testing.T) {
	// The two manifest layouts differ only by paths; every portable source maps
	// to the exact same checked-in bytes that the native root workspace binds.
	raw := cropTestRead(t, tutorialManifest)
	portable := cropTestRead(t, "examples/learn-gooo/gooo.workspace.json")
	var a, b any
	_ = json.Unmarshal(raw, &a)
	_ = json.Unmarshal(portable, &b)
	normalized := strings.ReplaceAll(string(raw), "examples/learn-gooo/", "")
	_ = json.Unmarshal([]byte(normalized), &a)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("portable workspace changed semantics")
	}
	for _, name := range []string{"limits.gooo", "main.gooo", "inputs.json", "gooo.workspace.json", "NOTICE", "README.md"} {
		if _, err := os.Stat(filepath.Join("../../examples/learn-gooo", name)); err != nil {
			t.Fatal(err)
		}
	}
}

// Re-check existing real receipts without rebuilding or rerunning the native
// compiler. The separate Node smoke executes the actual corresponding Wasm.
func TestTutorialExistingNativeArtifacts(t *testing.T) {
	dir := os.Getenv("GOOO_TUTORIAL_EVIDENCE")
	if dir == "" {
		t.Skip("set GOOO_TUTORIAL_EVIDENCE to an existing tutorial asset directory")
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var lock lockfile
	if err := json.Unmarshal(read("upstream.json"), &lock); err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	for _, name := range tutorialSources {
		sources[name] = read(filepath.Join("bound-source", name))
	}
	manifest := read("workspace.json")
	generated := string(read("generated.go"))
	for _, stem := range []string{"", "boundary-"} {
		execution := read(stem + "execution.json")
		inputs := read(stem + "inputs.json")
		for i, raw := range [][]byte{execution, read(stem + "replay.json")} {
			prior := ""
			if i == 1 {
				prior = "sha256:" + digest(execution)
			}
			got, err := inspectTutorialReceipt(raw, lock, manifest, inputs, sources, prior)
			if err != nil || got != generated {
				t.Fatalf("%s receipt%d: %v", stem, i, err)
			}
		}
	}
	index := 0
	for _, row := range tutorialFixtureSet(lock).Rejections {
		if !row.Native {
			continue
		}
		name := fmt.Sprintf("rejected-%02d", index)
		if err := inspectTutorialRejection(read(name+"-execution.json"), manifest, read(name+"-inputs.json"), row.Name, lock, generated); err != nil {
			t.Fatal(err)
		}
		index++
	}
}
