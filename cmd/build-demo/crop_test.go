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

func cropTestRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", path))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Synthetic receipt tests assert evidence validation, not executable geometry.
func cropEvidenceFixture(t *testing.T) ([]byte, lockfile, []byte, []byte, map[string][]byte) {
	return cropProgramEvidenceFixture(t, originalCropProgram)
}

func cropProgramEvidenceFixture(t *testing.T, program cropProgram) ([]byte, lockfile, []byte, []byte, map[string][]byte) {
	t.Helper()
	lock := lockfile{CompilerRevision: strings.Repeat("a", 40), GoVersion: "go1.27.2"}
	manifestRaw, inputsRaw := cropTestRead(t, program.manifest), cropTestRead(t, program.inputs)
	rows, err := cropProgramInputRows(inputsRaw, program)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	packages := []any{}
	for pkg, names := range program.sources {
		sourceRows := []any{}
		for _, name := range names {
			raw := cropTestRead(t, name)
			sources[name] = raw
			encoded, _ := json.Marshal(string(raw))
			sourceRows = append(sourceRows, map[string]any{"filename": name, "source_digest": "sha256:" + digest(encoded)})
		}
		p := map[string]any{"path": pkg, "sources": sourceRows}
		if pkg == "example/crop-planner" {
			p["exports"] = []any{map[string]any{"name": program.activity, "id": program.activityID, "kind": "activity", "input_types": []string{cropRectangleID, cropRectangleID}, "output_type": cropResultID}}
		}
		packages = append(packages, p)
	}
	resultNames := append(append([]string(nil), cropCoordinates...), "has_pixels")
	layouts := []any{cropRecordLayout("CropResult", cropResultID, resultNames), cropRecordLayout("Rectangle", cropRectangleID, cropCoordinates)}
	source := "package main\n// Preserve exact fixture bytes: 한글  \n"
	for _, v := range layouts {
		layout := cropMap(v)
		source += "type " + layout["go_name"].(string) + " struct {\n"
		for _, v := range cropList(layout["fields"]) {
			field := cropMap(v)
			typ := "int64"
			if field["name"] == "has_pixels" {
				typ = "bool"
			}
			source += fmt.Sprintf("%s %s `json:%q`\n", field["go_name"], typ, field["name"])
		}
		source += "}\n"
	}
	source += fmt.Sprintf("func GoooComposedActivity0(input0 %s, input1 %s) %s { return %s{} }\n", cropGoName("Record", cropRectangleID), cropGoName("Record", cropRectangleID), cropGoName("Record", cropResultID), cropGoName("Record", cropResultID))
	traces := []any{}
	for i, row := range rows {
		inputs := []any{}
		for j := range 2 {
			record := map[string]any{}
			for k, name := range cropCoordinates {
				record[name] = row[4*j+k]
			}
			inputs = append(inputs, map[string]any{"port": fmt.Sprintf("input%d", j), "entity_id": cropRectangleID, "value": record, "fields": cropFields(cropRectangleID, cropCoordinates, record)})
		}
		want := cropProgramOracle(row, program)
		traces = append(traces, map[string]any{"case_index": i, "deliveries": []any{map[string]any{"activity_id": program.activityID, "inputs": inputs, "actual": want, "actual_fields": cropFields(cropResultID, resultNames, want)}}})
	}
	sha := "sha256:" + digest([]byte(source))
	raw := testJSON(t, map[string]any{
		"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest_digest": "sha256:" + digest(manifestRaw), "inputs_digest": "sha256:" + digest(inputsRaw),
		"result": map[string]any{
			"program":     map[string]any{"entry": map[string]any{"package_path": "example/crop-planner", "activity": program.activity, "lowered_name": "GoooPackage4ActivityCropPlan"}, "workspace_image": map[string]any{"packages": packages}},
			"composition": map[string]any{"stage": "COMPLETE", "source": source, "generated_sha256": sha, "plan": map[string]any{"record_types": layouts, "entry_activity": "GoooPackage4ActivityCropPlan", "activities": []any{map[string]any{"name": "GoooPackage4ActivityCropPlan", "id": program.activityID, "go_function": "GoooComposedActivity0"}}}, "steps": []any{map[string]any{"generation": map[string]any{"report": map[string]any{"activity_id": program.activityID, "compiler_source_sha": lock.CompilerRevision}}}}},
			"runtime":     map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": lock.CompilerRevision, "go_version": "go version go1.27.2 darwin/arm64", "generated_sha256": sha, "model_calls": 0, "finite_total": 0, "finite_passed": 0, "projection_replayed": true, "runtime_replayed": true, "traces": traces},
		},
	})
	return raw, lock, manifestRaw, inputsRaw, sources
}

func TestCropEvidencePreservesGeneratedBytesAndChecksSavedReplay(t *testing.T) {
	raw, lock, manifest, inputs, sources := cropEvidenceFixture(t)
	got, err := inspectCropReceipt(raw, lock, manifest, inputs, sources, "")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := cropDecode(raw)
	e := cropMap(v)
	result := cropMap(e["result"])
	if got != cropMap(result["composition"])["source"] {
		t.Fatal("generated source changed")
	}
	prior := "sha256:" + digest(raw)
	e["replayed_from_sha256"] = prior
	result["replay"] = map[string]any{"model_calls": 0}
	if saved, err := inspectCropReceipt(testJSON(t, e), lock, manifest, inputs, sources, prior); err != nil || saved != got {
		t.Fatal("valid saved replay rejected", err)
	}
	if _, err := inspectCropReceipt(testJSON(t, e), lock, manifest, inputs, sources, ""); err == nil {
		t.Fatal("replay accepted as fresh execution")
	}
	for _, invalid := range []any{nil, true, "0", json.Number("0.0"), json.Number("1")} {
		result["replay"] = map[string]any{"model_calls": invalid}
		if _, err := inspectCropReceipt(testJSON(t, e), lock, manifest, inputs, sources, prior); err == nil {
			t.Fatal("invalid replay model counter accepted", invalid)
		}
	}
	result["replay"] = map[string]any{}
	if _, err := inspectCropReceipt(testJSON(t, e), lock, manifest, inputs, sources, prior); err == nil {
		t.Fatal("missing replay model count accepted")
	}
}

func TestCropEvidenceRejectsTampering(t *testing.T) {
	raw, lock, manifest, inputs, sources := cropEvidenceFixture(t)
	result := func(e map[string]any) map[string]any { return cropMap(e["result"]) }
	run := func(e map[string]any) map[string]any { return cropMap(result(e)["runtime"]) }
	comp := func(e map[string]any) map[string]any { return cropMap(result(e)["composition"]) }
	delivery := func(e map[string]any, i int) map[string]any {
		return cropMap(cropList(cropMap(cropList(run(e)["traces"])[i])["deliveries"])[0])
	}
	mutations := map[string]func(map[string]any){
		"lowered entry name": func(e map[string]any) {
			cropMap(cropMap(result(e)["program"])["entry"])["lowered_name"] = "OtherActivity"
		},
		"lowered entry function": func(e map[string]any) {
			cropMap(cropList(cropMap(comp(e)["plan"])["activities"])[0])["go_function"] = "OtherFunction"
		},
		"decision":               func(e map[string]any) { e["decision"] = "PASS" },
		"input digest":           func(e map[string]any) { e["inputs_digest"] = "wrong" },
		"manifest digest":        func(e map[string]any) { e["manifest_digest"] = "wrong" },
		"case digest":            func(e map[string]any) { e["cases_digest"] = "invented" },
		"missing model calls":    func(e map[string]any) { delete(run(e), "model_calls") },
		"null model calls":       func(e map[string]any) { run(e)["model_calls"] = nil },
		"Boolean model calls":    func(e map[string]any) { run(e)["model_calls"] = false },
		"fractional model calls": func(e map[string]any) { run(e)["model_calls"] = json.Number("0.0") },
		"wrong compiler":         func(e map[string]any) { run(e)["producer_source_sha"] = "wrong" },
		"wrong toolchain":        func(e map[string]any) { run(e)["go_version"] = "go version go1.26.5 darwin/arm64" },
		"runtime failed":         func(e map[string]any) { run(e)["failure"] = "panic" },
		"not replayed":           func(e map[string]any) { run(e)["runtime_replayed"] = false },
		"unexpected replay":      func(e map[string]any) { result(e)["replay"] = map[string]any{"model_calls": 0} },
		"rounded input": func(e map[string]any) {
			record := cropMap(cropList(delivery(e, 11)["inputs"])[1])
			cropMap(record["value"])["min_x"] = json.Number("9007199254740992")
		},
		"Boolean coordinate": func(e map[string]any) {
			record := cropMap(cropList(delivery(e, 0)["inputs"])[0])
			cropMap(record["value"])["min_x"] = false
		},
		"fractional coordinate": func(e map[string]any) {
			record := cropMap(cropList(delivery(e, 0)["inputs"])[0])
			cropMap(record["value"])["min_x"] = json.Number("0.0")
		},
		"input order":  func(e map[string]any) { d := delivery(e, 0); x := cropList(d["inputs"]); x[0], x[1] = x[1], x[0] },
		"input entity": func(e map[string]any) { cropMap(cropList(delivery(e, 0)["inputs"])[0])["entity_id"] = "wrong" },
		"field identity": func(e map[string]any) {
			x := cropMap(cropList(delivery(e, 0)["inputs"])[0])
			cropMap(cropList(x["fields"])[0])["id"] = "wrong"
		},
		"field order": func(e map[string]any) {
			x := cropMap(cropList(delivery(e, 0)["inputs"])[0])
			fields := cropList(x["fields"])
			fields[0], fields[1] = fields[1], fields[0]
		},
		"rounded output":        func(e map[string]any) { cropMap(delivery(e, 11)["actual"])["min_x"] = json.Number("9007199254740992") },
		"numeric Boolean":       func(e map[string]any) { cropMap(delivery(e, 0)["actual"])["has_pixels"] = json.Number("1") },
		"output field identity": func(e map[string]any) { cropMap(cropList(delivery(e, 0)["actual_fields"])[0])["id"] = "wrong" },
		"extra input field": func(e map[string]any) {
			x := cropMap(cropList(delivery(e, 0)["inputs"])[0])
			cropMap(x["value"])["extra"] = 0
		},
		"unexpected expected": func(e map[string]any) { delivery(e, 0)["expected"] = delivery(e, 0)["actual"] },
		"record type": func(e map[string]any) {
			layouts := cropList(cropMap(comp(e)["plan"])["record_types"])
			cropMap(layouts[1])["id"] = "wrong"
		},
		"generated bytes": func(e map[string]any) { comp(e)["source"] = comp(e)["source"].(string) + "\n" },
		"source binding": func(e map[string]any) {
			p := cropMap(result(e)["program"])
			pkg := cropMap(cropList(cropMap(p["workspace_image"])["packages"])[0])
			cropMap(cropList(pkg["sources"])[0])["source_digest"] = "wrong"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			v, _ := cropDecode(raw)
			e := cropMap(v)
			mutate(e)
			if _, err := inspectCropReceipt(testJSON(t, e), lock, manifest, inputs, sources, ""); err == nil {
				t.Fatal("accepted tampered evidence")
			}
		})
	}
}

func TestCropGeneratedABIRejectsTypeAndFieldDrift(t *testing.T) {
	raw, lock, manifest, inputs, sources := cropEvidenceFixture(t)
	source, err := inspectCropReceipt(raw, lock, manifest, inputs, sources, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(source, "package main", "package other", 1),
		strings.Replace(source, " int64 ", " int32 ", 1),
		strings.Replace(source, `json:"min_x"`, `json:"max_x"`, 1),
		strings.Replace(source, "input1 "+cropGoName("Record", cropRectangleID), "input1 int64", 1),
		strings.Replace(source, cropGoName("Field", cropFieldID(cropResultID, "min_x")), "DifferentField", 1),
	} {
		if err := inspectCropGoABI(bad); err == nil {
			t.Fatal("accepted generated ABI drift")
		}
	}
}

func TestCropDocumentedOracleAndDeterministicFixtures(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("the reference domain requires 64-bit int")
	}
	inputs := cropTestRead(t, cropInputs)
	rows, err := cropInputRows(inputs)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := int64(-1<<63), int64(1<<63-1)
	want := [][4]int64{{0, 40, 200, 480}, {0, 0, 640, 480}, {100, 100, 200, 200}, {}, {}, {}, {}, {}, {-8, -30, 5, 4}, {hi - 1, hi - 1, hi, hi}, {lo, lo, lo + 1, lo + 1}, {9007199254740993, -9007199254740992, 9007199254740994, -9007199254740991}}
	for i, row := range rows {
		actual := cropOracle(row)
		for j, name := range cropCoordinates {
			if actual[name] != want[i][j] {
				t.Fatal("documented coordinate", i, name, actual)
			}
		}
		if actual["has_pixels"] != (i < 3 || i > 7) {
			t.Fatal("documented emptiness", i)
		}
	}
	t.Chdir("../..")
	var lock lockfile
	if err := readJSON("upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	a, err := makeCropFixtures(rows, lock)
	if err != nil {
		t.Fatal(err)
	}
	b, err := makeCropFixtures(rows, lock)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic fixtures", err)
	}
	if len(a.Cases) != 88 || len(a.Rejections) != 96 {
		t.Fatal("fixture roster", len(a.Cases), len(a.Rejections))
	}
}

func TestCropEvidenceJSONRejectsDuplicateAndTrailingValues(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":{"b":0,"b":1}}`, `[] []`} {
		if _, err := cropDecode([]byte(raw)); err == nil {
			t.Fatal("accepted malformed evidence", raw)
		}
	}
}

// Optional saved-artifact regression check. This reads already produced
// evidence; it never invokes the compiler or repeats native execution.
func TestCropSavedReceiptCompatibility(t *testing.T) {
	dir := os.Getenv("CROP_EVIDENCE")
	if dir == "" {
		t.Skip("set CROP_EVIDENCE to an absolute directory containing execution.json and replay.json")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("CROP_EVIDENCE must be an absolute path")
	}
	manifestRaw, inputsRaw := cropTestRead(t, cropManifest), cropTestRead(t, cropInputs)
	sources := map[string][]byte{}
	for _, filename := range cropSources {
		sources[filename] = cropTestRead(t, filename)
	}
	var lock lockfile
	if err := json.Unmarshal(cropTestRead(t, "upstream.json"), &lock); err != nil {
		t.Fatal(err)
	}
	execution, err := os.ReadFile(filepath.Join(dir, "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := inspectCropReceipt(execution, lock, manifestRaw, inputsRaw, sources, "")
	if err != nil {
		t.Fatal("saved execution", err)
	}
	replay, err := os.ReadFile(filepath.Join(dir, "replay.json"))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := inspectCropReceipt(replay, lock, manifestRaw, inputsRaw, sources, "sha256:"+digest(execution))
	if err != nil || source != replayed {
		t.Fatal("saved replay", err)
	}
}
