package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-go-ports/internal/cropbridge"
)

const cropManifest = "crop-planner.workspace.json"
const cropInputs = "examples/crop-planner/inputs.json"
const cropEntry = "example/crop-planner:CropPlan"
const cropActivityID = "gooo://example/crop-planner/crop-plan"
const cropRectangleID = "gooo://go/image/rectangle"
const cropResultID = "gooo://example/crop-planner/result"

var cropSources = map[string]string{
	"go/image":             "ports/image/geometry.gooo",
	"example/crop-planner": "examples/crop-planner/crop_planner.gooo",
}
var cropCoordinates = []string{"min_x", "min_y", "max_x", "max_y"}

// Source lists use the pinned workspace image's filename sort order.
// Each consumer keeps its own unmodified composition and Wasm module. The Go
// reference calculation runs only while building fixtures, never in a bridge.
type cropProgram struct {
	asset, manifest, inputs, activity, activityID, global, guide string
	sources                                                      map[string][]string
	canonicalSelection                                           bool
}

var originalCropProgram = cropProgram{
	asset: "crop", manifest: cropManifest, inputs: cropInputs,
	activity: "CropPlan", activityID: cropActivityID, global: "goooCrop", guide: "examples/crop-planner/README.md",
	sources: map[string][]string{
		"go/image":             {"ports/image/geometry.gooo"},
		"example/crop-planner": {"examples/crop-planner/crop_planner.gooo"},
	},
}
var dragCropProgram = cropProgram{
	asset: "drag-crop", manifest: "drag-crop.workspace.json", inputs: "examples/crop-planner/drag_inputs.json",
	activity: "DragCropPlan", activityID: "gooo://example/crop-planner/drag-crop-plan", global: "goooDragCrop", guide: "docs/drag-crop.md",
	sources: map[string][]string{
		"go/image":             {"ports/image/canon.gooo", "ports/image/geometry.gooo"},
		"example/crop-planner": {"examples/crop-planner/crop_planner.gooo", "examples/crop-planner/drag_crop.gooo"},
	},
	canonicalSelection: true,
}

func (program cropProgram) entry() string { return "example/crop-planner:" + program.activity }

type cropCase struct {
	Name     string            `json:"name"`
	Inputs   []string          `json:"inputs"`
	Expected cropbridge.Output `json:"expected"`
}

type cropRejection struct {
	Name   string   `json:"name"`
	Inputs []string `json:"inputs"`
}

type cropFixtures struct {
	Schema           string            `json:"schema"`
	GoVersion        string            `json:"go_version"`
	CompilerRevision string            `json:"compiler_revision"`
	Cases            []cropCase        `json:"cases"`
	Rejections       []cropRejection   `json:"rejections"`
	GoSourceSHA      map[string]string `json:"go_source_sha256"`
}

// buildCropDemo retains the original two-record entry. Its generated source is
// never edited: only the separate JS bridge adapts eight strings to Go records.
func buildCropDemo(ctx context.Context, site, dir string, lock lockfile) error {
	for _, program := range []cropProgram{originalCropProgram, dragCropProgram} {
		if err := buildCropConsumer(ctx, site, dir, lock, program); err != nil {
			return fmt.Errorf("%s: %w", program.activity, err)
		}
	}
	return nil
}

func buildCropConsumer(ctx context.Context, site, dir string, lock lockfile, program cropProgram) error {
	if strconv.IntSize != 64 || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") || runtime.Version() != lock.GoVersion {
		return fmt.Errorf("crop oracle requires pinned Go on amd64/arm64 with 64-bit int")
	}
	asset := filepath.Join(site, program.asset)
	work := filepath.Join(dir, program.asset+"-build")
	for _, path := range []string{asset, work} {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	}
	manifestRaw, err := os.ReadFile(program.manifest)
	if err != nil {
		return err
	}
	inputsRaw, err := os.ReadFile(program.inputs)
	if err != nil {
		return err
	}
	rows, err := cropProgramInputRows(inputsRaw, program)
	if err != nil {
		return err
	}
	fixtures, err := makeCropProgramFixtures(rows, lock, program)
	if err != nil {
		return err
	}
	sources := map[string][]byte{}
	sourceHashes := map[string]string{}
	for _, paths := range program.sources {
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sources[path], sourceHashes[path] = raw, digest(raw)
			if err := write(filepath.Join(asset, "source", path), raw); err != nil {
				return err
			}
		}
	}
	if err := copyFile(program.guide, filepath.Join(asset, "source", program.guide)); err != nil {
		return err
	}
	// Execute this consumer with its original two-record input ABI.
	// It uses the existing 12 input-only examples, then checks their outputs with
	// the independent Go oracle below. OBSERVED receipts remain OBSERVED.
	execution, err := command(ctx, nil, "./.gooo", "package", "execute", "--json", "--inputs", program.inputs, program.manifest)
	if err != nil {
		return err
	}
	generated, err := inspectCropProgramReceipt(execution, lock, manifestRaw, inputsRaw, sources, "", program)
	if err != nil {
		return err
	}
	if err := write(filepath.Join(asset, "execution.json"), execution); err != nil {
		return err
	}
	replay, err := command(ctx, nil, "./.gooo", "package", "replay", "--json", "--receipt", filepath.Join(asset, "execution.json"), "--inputs", program.inputs, program.manifest)
	if err != nil {
		return err
	}
	replayed, err := inspectCropProgramReceipt(replay, lock, manifestRaw, inputsRaw, sources, "sha256:"+digest(execution), program)
	if err != nil || replayed != generated {
		return fmt.Errorf("crop replay source or evidence mismatch: %v", err)
	}
	for path, raw := range map[string][]byte{
		"generated.go": []byte(generated), "replay.json": replay,
		"inputs.json": inputsRaw, program.manifest: manifestRaw,
	} {
		if err := write(filepath.Join(asset, path), raw); err != nil {
			return err
		}
	}
	if err := write(filepath.Join(work, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	bridge, err := os.ReadFile("tools/crop-browser-bridge.go.txt")
	if err != nil {
		return err
	}
	replacements := []string{"__RECTANGLE__", cropGoName("Record", cropRectangleID), "__GLOBAL__", program.global}
	for _, name := range cropCoordinates {
		replacements = append(replacements, "__"+strings.ToUpper(name)+"__", cropGoName("Field", cropFieldID(cropRectangleID, name)))
	}
	bridge = []byte(strings.NewReplacer(replacements...).Replace(string(bridge)))
	if err := write(filepath.Join(work, "bridge.go"), bridge); err != nil {
		return err
	}
	_, err = command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}, "go", "-C", dir, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", filepath.Join(asset, "crop.wasm"), "./"+program.asset+"-build")
	if err != nil {
		return err
	}
	fixtureRaw, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	fixtureRaw = append(fixtureRaw, '\n')
	if err := write(filepath.Join(asset, "fixtures.json"), fixtureRaw); err != nil {
		return err
	}
	for _, name := range []string{"upstream.json", "LICENSE", "third_party/GO-LICENSE"} {
		if err := copyFile(name, filepath.Join(asset, name)); err != nil {
			return err
		}
	}
	revision, err := command(ctx, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	status, err := command(ctx, nil, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(filepath.Join(asset, "crop.wasm"))
	if err != nil {
		return err
	}
	metadata := map[string]any{
		"schema": "gooo/browser-crop-planner/v1", "entry_id": program.activityID,
		"compiler_revision": lock.CompilerRevision, "go_version": lock.GoVersion,
		"source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0,
		"source_sha256": sourceHashes, "generated_sha256": digest([]byte(generated)),
		"wasm_sha256": digest(wasm), "wasm_bytes": len(wasm), "fixtures_sha256": digest(fixtureRaw),
		"execution_sha256": digest(execution), "replay_sha256": digest(replay),
		"manifest_sha256": digest(manifestRaw), "inputs_sha256": digest(inputsRaw),
		"fixture_cases": len(fixtures.Cases), "fixture_rejections": len(fixtures.Rejections),
		"observed_inputs": len(rows), "native_oracle_matches": len(rows),
		"scope": "Finite offline Go-reference fixtures. Coordinates only; no image files are processed. Artifact hashes establish local consistency, not signed provenance.",
	}
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(asset, "metadata.json"), append(raw, '\n'))
}

// The oracle runs only on the pinned 64-bit build host, never in the Wasm
// module. Direct Point fields preserve reversed bounds: image.Rect would not.
func cropOracle(values [8]int64) map[string]any {
	return cropProgramOracle(values, originalCropProgram)
}

func cropProgramOracle(values [8]int64, program cropProgram) map[string]any {
	a := image.Rectangle{Min: image.Point{X: int(values[0]), Y: int(values[1])}, Max: image.Point{X: int(values[2]), Y: int(values[3])}}
	b := image.Rectangle{Min: image.Point{X: int(values[4]), Y: int(values[5])}, Max: image.Point{X: int(values[6]), Y: int(values[7])}}
	if program.canonicalSelection {
		b = b.Canon()
	}
	c := a.Intersect(b)
	return map[string]any{"min_x": int64(c.Min.X), "min_y": int64(c.Min.Y), "max_x": int64(c.Max.X), "max_y": int64(c.Max.Y), "has_pixels": !c.Empty()}
}

func makeCropFixtures(rows [][8]int64, lock lockfile) (cropFixtures, error) {
	return makeCropProgramFixtures(rows, lock, originalCropProgram)
}

func makeCropProgramFixtures(rows [][8]int64, lock lockfile, program cropProgram) (cropFixtures, error) {
	var upstream struct {
		SourceSHA map[string]string `json:"source_sha256"`
	}
	if err := readJSON("upstream.json", &upstream); err != nil {
		return cropFixtures{}, err
	}
	const source = "src/image/geom.go"
	raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), source))
	if err != nil || upstream.SourceSHA[source] == "" || digest(raw) != upstream.SourceSHA[source] {
		return cropFixtures{}, fmt.Errorf("crop oracle Go source hash mismatch: %v", err)
	}
	f := cropFixtures{Schema: "gooo/crop-browser-fixtures/v1", GoVersion: lock.GoVersion, CompilerRevision: lock.CompilerRevision, GoSourceSHA: map[string]string{source: upstream.SourceSHA[source]}}
	add := func(name string, values [8]int64) error {
		raw, err := json.Marshal(cropProgramOracle(values, program))
		if err != nil {
			return err
		}
		out, err := cropbridge.DecodeOutput(raw)
		if err != nil {
			return err
		}
		inputs := make([]string, 8)
		for i, n := range values {
			inputs[i] = strconv.FormatInt(n, 10)
		}
		f.Cases = append(f.Cases, cropCase{name, inputs, out})
		return nil
	}
	for i, values := range rows {
		if err := add(fmt.Sprintf("documented-%02d", i+1), values); err != nil {
			return f, err
		}
	}
	const lo, hi = int64(-1 << 63), int64(1<<63 - 1)
	boundary := [][8]int64{
		{0, 0, 1, 1, 0, 0, 1, 1}, {0, 0, 0, 0, lo, lo, hi, hi},
		{lo, lo, hi, hi, lo, lo, hi, hi}, {hi, hi, lo, lo, lo, lo, hi, hi},
		{0, 0, 10, 10, 10, 10, 20, 20}, {0, 0, 10, 10, -10, -10, 0, 0},
		{0, 0, 10, 10, 2, 2, 2, 8}, {0, 0, 10, 10, 2, 8, 8, 2},
		{lo, -1, lo + 2, 1, lo + 1, -1, hi, 1}, {hi - 2, -1, hi, 1, lo, -1, hi - 1, 1},
		{9007199254740992, 0, 9007199254740994, 2, 9007199254740993, 1, 9007199254740995, 3},
		{-9007199254740995, -3, -9007199254740992, 0, -9007199254740994, -2, -9007199254740993, -1},
	}
	for i, values := range boundary {
		if err := add(fmt.Sprintf("boundary-%02d", i+1), values); err != nil {
			return f, err
		}
	}
	rng := rand.New(rand.NewSource(20261011))
	for i := range 64 {
		var values [8]int64
		for j := range values {
			values[j] = int64(rng.Uint64())
		}
		if i%2 == 0 {
			// Deliberately generate valid overlapping rectangles as well as raw
			// noncanonical records. This generation is not input normalization.
			x, y := rng.Int63n(2000)-1000, rng.Int63n(2000)-1000
			values = [8]int64{x, y, x + 100, y + 100, x + 20, y - 20, x + 120, y + 80}
		}
		if err := add(fmt.Sprintf("seeded-%02d", i+1), values); err != nil {
			return f, err
		}
	}
	for position := range 8 {
		for i, bad := range []string{"", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "0x10", "NaN", "9223372036854775808", "-9223372036854775809"} {
			inputs := []string{"0", "0", "10", "10", "1", "1", "9", "9"}
			inputs[position] = bad
			f.Rejections = append(f.Rejections, cropRejection{fmt.Sprintf("input-%d-invalid-%02d", position+1, i+1), inputs})
		}
	}
	return f, nil
}

func cropInputRows(raw []byte) ([][8]int64, error) {
	return cropProgramInputRows(raw, originalCropProgram)
}

func cropProgramInputRows(raw []byte, program cropProgram) ([][8]int64, error) {
	v, err := cropDecode(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := v.(map[string]any)
	if !ok || doc["schema"] != "gooo/body-composition-inputs/v1" || len(doc) != 2 {
		return nil, fmt.Errorf("invalid crop inputs document")
	}
	inputs, ok := doc["inputs"].([]any)
	if !ok || len(inputs) != 12 {
		return nil, fmt.Errorf("require the 12 documented crop examples")
	}
	var rows [][8]int64
	for _, input := range inputs {
		row, ok := input.(map[string]any)
		if !ok || len(row) != 2 {
			return nil, fmt.Errorf("crop example needs two original records")
		}
		var values [8]int64
		for i := range 2 {
			record, ok := row[fmt.Sprintf("%s.input%d", program.entry(), i)].(map[string]any)
			if !ok || len(record) != 4 {
				return nil, fmt.Errorf("crop example has invalid record")
			}
			for j, name := range cropCoordinates {
				n, ok := record[name].(json.Number)
				if !ok {
					return nil, fmt.Errorf("crop example coordinate is not integer")
				}
				value, err := n.Int64()
				if err != nil || strconv.FormatInt(value, 10) != n.String() {
					return nil, fmt.Errorf("crop example coordinate is not exact int64")
				}
				values[4*i+j] = value
			}
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func cropGoName(kind, id string) string { return "Gooo" + kind + digest([]byte(id)) }
func cropFieldID(entity, name string) string {
	return entity + "/" + strings.ReplaceAll(name, "_", "-")
}
func cropMap(v any) map[string]any { return v.(map[string]any) }
func cropList(v any) []any         { return v.([]any) }
func cropEqual(a, b any) bool {
	x, e1 := json.Marshal(a)
	y, e2 := json.Marshal(b)
	return e1 == nil && e2 == nil && sameJSON(x, y)
}

func cropFields(entity string, names []string, value map[string]any) []any {
	fields := make([]any, len(names))
	for i, name := range names {
		fields[i] = map[string]any{"id": cropFieldID(entity, name), "name": name, "value": value[name]}
	}
	return fields
}

func cropRecordLayout(name, id string, names []string) map[string]any {
	fields := make([]any, len(names))
	for i, field := range names {
		typeID := "urn:gooo:type:integer"
		if field == "has_pixels" {
			typeID = "urn:gooo:type:boolean"
		}
		fieldID := cropFieldID(id, field)
		fields[i] = map[string]any{"name": field, "id": fieldID, "type_id": typeID, "go_name": cropGoName("Field", fieldID)}
	}
	return map[string]any{"name": name, "id": id, "go_name": cropGoName("Record", id), "fields": fields}
}

// inspectCropReceipt binds observed inputs and outputs to source/type identities
// and the pinned producer. A successful observation alone is not a Go oracle.
func inspectCropReceipt(raw []byte, lock lockfile, manifestRaw, inputsRaw []byte, sources map[string][]byte, prior string) (string, error) {
	return inspectCropProgramReceipt(raw, lock, manifestRaw, inputsRaw, sources, prior, originalCropProgram)
}

func inspectCropProgramReceipt(raw []byte, lock lockfile, manifestRaw, inputsRaw []byte, sources map[string][]byte, prior string, program cropProgram) (source string, err error) {
	defer func() {
		if recover() != nil {
			source, err = "", fmt.Errorf("invalid crop receipt shape")
		}
	}()
	fail := func(reason string) (string, error) { return "", fmt.Errorf("invalid crop evidence: %s", reason) }
	v, err := cropDecode(raw)
	if err != nil {
		return "", err
	}
	e := cropMap(v)
	rows, err := cropProgramInputRows(inputsRaw, program)
	if err != nil {
		return "", err
	}
	if e["schema"] != "gooo/workspace-body-execution-receipt/v1" || e["decision"] != "OBSERVED" || e["error"] != nil && e["error"] != "" || e["manifest_digest"] != "sha256:"+digest(manifestRaw) || e["inputs_digest"] != "sha256:"+digest(inputsRaw) || e["cases_digest"] != nil {
		return fail("receipt identity or input binding")
	}
	if prior == "" {
		if e["replayed_from_sha256"] != nil {
			return fail("unexpected replay binding")
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
	if entry["package_path"] != "example/crop-planner" || entry["activity"] != program.activity || !ok || !token.IsIdentifier(lowered) || plan["entry_activity"] != lowered {
		return fail("entry")
	}
	activities := cropList(plan["activities"])
	if len(activities) != 1 {
		return fail("entry activity plan")
	}
	activity := cropMap(activities[0])
	if activity["name"] != lowered || activity["id"] != program.activityID || activity["go_function"] != "GoooComposedActivity0" {
		return fail("lowered entry binding")
	}
	packages := cropList(cropMap(p["workspace_image"])["packages"])
	if len(packages) != 2 {
		return fail("package roster")
	}
	seen := map[string]bool{}
	for _, value := range packages {
		pkg := cropMap(value)
		path := pkg["path"].(string)
		filenames, ok := program.sources[path]
		if !ok || seen[path] {
			return fail("package identity")
		}
		seen[path] = true
		sourceRows := cropList(pkg["sources"])
		if len(sourceRows) != len(filenames) {
			return fail("source roster")
		}
		for i, filename := range filenames {
			encoded, _ := json.Marshal(string(sources[filename]))
			src := cropMap(sourceRows[i])
			if len(sources[filename]) == 0 || src["filename"] != filename || src["source_digest"] != "sha256:"+digest(encoded) {
				return fail("source binding")
			}
		}
		if path == "example/crop-planner" {
			matched := 0
			for _, value := range cropList(pkg["exports"]) {
				x := cropMap(value)
				if x["name"] != program.activity && x["id"] != program.activityID {
					continue
				}
				matched++
				if x["name"] != program.activity || x["kind"] != "activity" || x["id"] != program.activityID || x["output_type"] != cropResultID || !cropEqual(x["input_types"], []string{cropRectangleID, cropRectangleID}) {
					return fail("entry export signature")
				}
			}
			if matched != 1 {
				return fail("entry export roster")
			}
		}
	}
	resultNames := append(append([]string(nil), cropCoordinates...), "has_pixels")
	expectedLayouts := []any{cropRecordLayout("CropResult", cropResultID, resultNames), cropRecordLayout("Rectangle", cropRectangleID, cropCoordinates)}
	if !cropEqual(cropMap(c["plan"])["record_types"], expectedLayouts) {
		return fail("record type or field identities")
	}
	source, ok = c["source"].(string)
	if !ok || source == "" || c["stage"] != "COMPLETE" || c["generated_sha256"] != "sha256:"+digest([]byte(source)) || run["generated_sha256"] != c["generated_sha256"] {
		return fail("generated source binding")
	}
	steps := cropList(c["steps"])
	if len(steps) != 1 {
		return fail("generation roster")
	}
	report := cropMap(cropMap(cropMap(steps[0])["generation"])["report"])
	if report["activity_id"] != program.activityID || report["compiler_source_sha"] != lock.CompilerRevision {
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
		deliveries := cropList(trace["deliveries"])
		if trace["case_index"] != json.Number(strconv.Itoa(i)) || len(deliveries) != 1 {
			return fail("trace order or delivery count")
		}
		d := cropMap(deliveries[0])
		inputs := make([]any, 2)
		for j := range 2 {
			record := map[string]any{}
			for k, name := range cropCoordinates {
				record[name] = rows[i][4*j+k]
			}
			inputs[j] = map[string]any{"port": fmt.Sprintf("input%d", j), "entity_id": cropRectangleID, "value": record, "fields": cropFields(cropRectangleID, cropCoordinates, record)}
		}
		want := cropProgramOracle(rows[i], program)
		if len(d) != 4 || d["activity_id"] != program.activityID || !cropEqual(d["inputs"], inputs) || !cropEqual(d["actual"], want) || !cropEqual(d["actual_fields"], cropFields(cropResultID, resultNames, want)) {
			return fail("original input, exact field binding or Go-reference output")
		}
	}
	if err := inspectCropGoABI(source); err != nil {
		return "", err
	}
	return source, nil
}

// Check the compiler-generated ABI before inserting its names in the separate
// bridge. This does not rewrite or trust a caller-supplied Go type spelling.
func inspectCropGoABI(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil || f.Name.Name != "main" {
		return fmt.Errorf("invalid crop generated Go: %v", err)
	}
	records, entry := map[string]bool{}, false
	for _, declaration := range f.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == "GoooComposedActivity0" {
			if entry || fn.Recv != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 2 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("crop generated entry signature changed")
			}
			for i, field := range fn.Type.Params.List {
				name, ok := field.Type.(*ast.Ident)
				if !ok || name.Name != cropGoName("Record", cropRectangleID) || len(field.Names) != 1 || field.Names[0].Name != fmt.Sprintf("input%d", i) {
					return fmt.Errorf("crop generated input type changed")
				}
			}
			result, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || result.Name != cropGoName("Record", cropResultID) {
				return fmt.Errorf("crop generated result type changed")
			}
			entry = true
		}
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, value := range gen.Specs {
			typ := value.(*ast.TypeSpec)
			for _, id := range []string{cropRectangleID, cropResultID} {
				if typ.Name.Name != cropGoName("Record", id) {
					continue
				}
				names := append([]string(nil), cropCoordinates...)
				if id == cropResultID {
					names = append(names, "has_pixels")
				}
				st, ok := typ.Type.(*ast.StructType)
				if !ok || records[id] || len(st.Fields.List) != len(names) {
					return fmt.Errorf("crop generated record changed")
				}
				records[id] = true
				for i, field := range st.Fields.List {
					wantType := "int64"
					if names[i] == "has_pixels" {
						wantType = "bool"
					}
					fieldType, ok := field.Type.(*ast.Ident)
					if !ok || fieldType.Name != wantType || len(field.Names) != 1 || field.Names[0].Name != cropGoName("Field", cropFieldID(id, names[i])) || field.Tag == nil {
						return fmt.Errorf("crop generated field changed")
					}
					tag, err := strconv.Unquote(field.Tag.Value)
					if err != nil || tag != `json:"`+names[i]+`"` {
						return fmt.Errorf("crop generated JSON field changed")
					}
				}
			}
		}
	}
	if !entry || len(records) != 2 {
		return fmt.Errorf("crop generated ABI incomplete")
	}
	return nil
}

// Decode evidence with exact numbers and recursive duplicate-key rejection.
func cropDecode(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func() (any, error)
	read = func() (any, error) {
		t, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if _, exists := m[name]; !ok || exists {
					return nil, fmt.Errorf("duplicate or invalid JSON key")
				}
				value, err := read()
				if err != nil {
					return nil, err
				}
				m[name] = value
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("invalid object")
			}
			return m, nil
		case json.Delim('['):
			v := []any{}
			for d.More() {
				value, err := read()
				if err != nil {
					return nil, err
				}
				v = append(v, value)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("invalid array")
			}
			return v, nil
		default:
			if _, ok := t.(json.Delim); ok {
				return nil, fmt.Errorf("unexpected JSON delimiter")
			}
			return t, nil
		}
	}
	v, err := read()
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON evidence")
	}
	return v, nil
}
