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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const literalManifest = "literal-text-report.workspace.json"
const literalInputs = "examples/literal-text-report/inputs.json"
const literalEntry = "gooo://go-ports/examples/literal-text-report/report"
const literalResult = "gooo://go-ports/examples/literal-text-report/result"

var literalSourceFiles = []string{"ports/strings/search.gooo", "ports/strings/literal.gooo", "examples/literal-text-report/main.gooo"}
var literalNames = []string{"text", "needle", "first_byte_offset", "found", "non_overlapping_count", "text_byte_length", "needle_byte_length"}
var literalFieldIDs = []string{"text", "needle", "first-byte-offset", "found", "non-overlapping-count", "text-byte-length", "needle-byte-length"}

//go:embed literal_search_bridge.go.txt
var literalBridge []byte

type literalOutput struct {
	Text                string `json:"text"`
	Needle              string `json:"needle"`
	FirstByteOffset     string `json:"first_byte_offset"`
	Found               bool   `json:"found"`
	NonOverlappingCount string `json:"non_overlapping_count"`
	TextByteLength      string `json:"text_byte_length"`
	NeedleByteLength    string `json:"needle_byte_length"`
}
type literalCase struct {
	Name                 string        `json:"name"`
	Inputs               [2]string     `json:"inputs"`
	TextUTF16            []uint16      `json:"text_utf16"`
	NeedleUTF16          []uint16      `json:"needle_utf16"`
	Expected             literalOutput `json:"expected"`
	textJSON, needleJSON json.RawMessage
}
type literalRejection struct {
	Name        string   `json:"name"`
	TextUTF16   []uint16 `json:"text_utf16"`
	NeedleUTF16 []uint16 `json:"needle_utf16"`
}
type literalFixtures struct {
	Schema           string             `json:"schema"`
	CompilerRevision string             `json:"compiler_revision"`
	GoVersion        string             `json:"go_version"`
	Cases            []literalCase      `json:"cases"`
	Rejections       []literalRejection `json:"rejections"`
}

// Independent reference, used only by the build host and never in the bridge.
func literalReference(text, needle string) literalOutput {
	offset := strings.Index(text, needle)
	return literalOutput{text, needle, strconv.Itoa(offset), offset >= 0, strconv.Itoa(strings.Count(text, needle)), strconv.Itoa(len(text)), strconv.Itoa(len(needle))}
}
func literalRows(raw []byte) ([]literalCase, error) {
	v, err := cropDecode(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := v.(map[string]any)
	if !ok || len(doc) != 2 || doc["schema"] != "gooo/body-composition-inputs/v1" {
		return nil, fmt.Errorf("invalid literal input schema")
	}
	rows, ok := doc["inputs"].([]any)
	if !ok || len(rows) == 0 || len(rows) > 128 {
		return nil, fmt.Errorf("invalid literal input roster")
	}
	result := []literalCase{}
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 2 {
			return nil, fmt.Errorf("invalid literal input fields")
		}
		text, t := row["app/literal-text-report:Report.input0"].(string)
		needle, n := row["app/literal-text-report:Report.input1"].(string)
		if !t || !n || !utf8.ValidString(text) || !utf8.ValidString(needle) || len(text) > 1024 || len(needle) > 1024 {
			return nil, fmt.Errorf("two decoded valid UTF-8 Text values <=1024 bytes required")
		}
		result = append(result, literalNewCase(fmt.Sprintf("supplied-%02d", i), text, needle))
	}
	return result, nil
}
func literalNewCase(name, text, needle string) literalCase {
	a, _ := json.Marshal(text)
	b, _ := json.Marshal(needle)
	return literalCase{name, [2]string{text, needle}, append([]uint16{}, utf16.Encode([]rune(text))...), append([]uint16{}, utf16.Encode([]rune(needle))...), literalReference(text, needle), a, b}
}
func literalBoundaryCases() []literalCase {
	rows := []literalCase{}
	for i, pair := range [][2]string{
		{"가x가", "x"}, {"🙂가🙂", "가"}, {"ée\u0301", "\u0301"}, {"e\u0301", "é"}, {"é", "e\u0301"},
		{"aaaaa", "aa"}, {"ababa", "aba"}, {"a\x00\r\na", "\x00\r\n"}, {"\ufeffA\ufeff", "\ufeff"},
		{"<>&\u2028\u2029", "\u2028"}, {"👩‍💻👩‍💻", "👩‍💻"}, {"가x가", ""},
		{strings.Repeat("a", 1024), ""}, {strings.Repeat("🙂", 256), ""},
		{strings.Repeat("é", 512), ""}, {strings.Repeat("가", 341) + "x", "x"},
		{strings.Repeat("a", 1023) + "b", "b"}, {strings.Repeat("a", 1024), strings.Repeat("a", 1024)},
		{"", strings.Repeat("a", 1024)}, {strings.Repeat("a", 1024), strings.Repeat("b", 1024)},
	} {
		rows = append(rows, literalNewCase(fmt.Sprintf("boundary-%02d", i), pair[0], pair[1]))
	}
	// Original browser UTF-16 units survive for the actual Wasm call. Native
	// comparisons encode the bound scalar Text, not malformed surrogate escapes.
	// The compiler independently refuses those raw JSON spellings.
	for _, p := range []struct {
		name, text, needle string
		tu, nu             []uint16
	}{
		{"lone-high-text", `"\ud800x"`, `"\ufffd"`, []uint16{0xd800, 'x'}, []uint16{0xfffd}},
		{"lone-low-needle", `"x\ufffdy"`, `"\udfff"`, []uint16{'x', 0xfffd, 'y'}, []uint16{0xdfff}},
		{"mixed-surrogates", `"\ud800x\udfff"`, `""`, []uint16{0xd800, 'x', 0xdfff}, []uint16{}},
		{"high-high-low", `"\ud800\ud83d\ude00"`, `"\ud83d\ude00"`, []uint16{0xd800, 0xd83d, 0xde00}, []uint16{0xd83d, 0xde00}},
	} {
		var text, needle string
		_ = json.Unmarshal([]byte(p.text), &text)
		_ = json.Unmarshal([]byte(p.needle), &needle)
		r := literalNewCase(p.name, text, needle)
		r.TextUTF16 = p.tu
		r.NeedleUTF16 = p.nu
		rows = append(rows, r)
	}
	return rows
}
func literalInputDocument(rows []literalCase) ([]byte, error) {
	values := make([]map[string]json.RawMessage, len(rows))
	for i, r := range rows {
		values[i] = map[string]json.RawMessage{"app/literal-text-report:Report.input0": r.textJSON, "app/literal-text-report:Report.input1": r.needleJSON}
	}
	raw, err := json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": values}, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(rows) == 0 || len(rows) > 128 || len(raw) > 32768 {
		return nil, fmt.Errorf("literal input envelope exceeds native limits")
	}
	return raw, nil
}
func literalCheckABI(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil {
		return err
	}
	recordName := "GoooRecord" + digest([]byte(literalResult))
	entry, record := false, false
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "GoooComposedActivity0" {
			if fn.Recv != nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 2 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("literal entry signature changed")
			}
			for i, p := range fn.Type.Params.List {
				t, ok := p.Type.(*ast.Ident)
				if !ok || t.Name != "string" || len(p.Names) != 1 || p.Names[0].Name != fmt.Sprintf("input%d", i) {
					return fmt.Errorf("literal Text ABI changed")
				}
			}
			t, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || t.Name != recordName {
				return fmt.Errorf("literal result ABI changed")
			}
			entry = true
		}
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, sp := range g.Specs {
			t, ok := sp.(*ast.TypeSpec)
			if !ok || t.Name.Name != recordName {
				continue
			}
			s, ok := t.Type.(*ast.StructType)
			if !ok || len(s.Fields.List) != 7 {
				return fmt.Errorf("literal result record changed")
			}
			for i, p := range s.Fields.List {
				kind := "int64"
				if i < 2 {
					kind = "string"
				}
				if i == 3 {
					kind = "bool"
				}
				fieldID := literalResult + "/" + literalFieldIDs[i]
				wantGo := "GoooField" + digest([]byte(fieldID))
				ft, ok := p.Type.(*ast.Ident)
				if !ok || ft.Name != kind || len(p.Names) != 1 || p.Names[0].Name != wantGo || p.Tag == nil {
					return fmt.Errorf("literal result field ABI changed")
				}
				tag, err := strconv.Unquote(p.Tag.Value)
				if err != nil || tag != fmt.Sprintf("json:%q", literalNames[i]) {
					return fmt.Errorf("literal result field JSON changed")
				}
			}
			record = true
		}
	}
	if !entry || !record {
		return fmt.Errorf("literal generated ABI incomplete")
	}
	return nil
}

func buildLiteralSearchDemo(ctx context.Context, site, dir string, lock lockfile) error {
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("literal search requires pinned Go")
	}
	asset, work := filepath.Join(site, "literal-search"), filepath.Join(dir, "literal-search-build")
	for _, p := range []string{asset, work} {
		if err := os.Mkdir(p, 0755); err != nil {
			return err
		}
	}
	manifestRaw, err := os.ReadFile(literalManifest)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(literalInputs)
	if err != nil {
		return err
	}
	rows, err := literalRows(original)
	if err != nil {
		return err
	}
	boundary := literalBoundaryCases()
	boundaryRaw, err := literalInputDocument(boundary)
	if err != nil {
		return err
	}
	sourceHashes := map[string]string{}
	for _, name := range literalSourceFiles {
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		sourceHashes[name] = digest(raw)
		if err = write(filepath.Join(asset, "bound-source", name), raw); err != nil {
			return err
		}
	}
	hashes := map[string]string{}
	put := func(name string, raw []byte) error {
		if err := write(filepath.Join(asset, name), raw); err != nil {
			return err
		}
		hashes[name] = digest(raw)
		return nil
	}
	if err = put("workspace.json", manifestRaw); err != nil {
		return err
	}
	compiler, err := command(ctx, nil, "./.gooo", "version", "--build", "--json")
	if err != nil {
		return err
	}
	if err = put("compiler.json", compiler); err != nil {
		return err
	}
	var generated string
	for _, batch := range []struct {
		stem string
		raw  []byte
		rows []literalCase
	}{{"", original, rows}, {"boundary-", boundaryRaw, boundary}} {
		input := filepath.Join(asset, batch.stem+"inputs.json")
		execution := filepath.Join(asset, batch.stem+"execution.json")
		replay := filepath.Join(asset, batch.stem+"replay.json")
		if err = put(batch.stem+"inputs.json", batch.raw); err != nil {
			return err
		}
		// Explicit matching Go path, including on Linux CI; never ambient fallback.
		goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
		execRaw, err := command(ctx, nil, "./.gooo", "package", "execute", "--json", "--go", goTool, "--inputs", input, literalManifest)
		if err != nil {
			return err
		}
		if err = put(batch.stem+"execution.json", execRaw); err != nil {
			return err
		}
		replayRaw, err := command(ctx, nil, "./.gooo", "package", "replay", "--json", "--go", goTool, "--receipt", execution, "--inputs", input, literalManifest)
		if err != nil {
			return err
		}
		if err = put(batch.stem+"replay.json", replayRaw); err != nil {
			return err
		}
		checked, err := command(ctx, nil, "go", "run", "./cmd/check-literal-text-report", "--snapshot", ".", "--inputs", input, "--execution", execution, "--replay", replay, "--compiler-build", filepath.Join(asset, "compiler.json"))
		if err != nil {
			return err
		}
		if err = put(batch.stem+"checked-report.json", checked); err != nil {
			return err
		}
		var receipt struct {
			Result struct {
				Composition struct {
					Source string `json:"source"`
				}
			}
		}
		if err = json.Unmarshal(execRaw, &receipt); err != nil {
			return err
		}
		source := receipt.Result.Composition.Source
		if source == "" || generated != "" && source != generated {
			return fmt.Errorf("literal generated source changed across batches")
		}
		if err = literalCheckABI(source); err != nil {
			return err
		}
		generated = source
		var proof struct {
			Schema    string
			Decision  string
			Generated string `json:"generated_sha256"`
			Execution string `json:"execution_sha256"`
			Replay    string `json:"replay_sha256"`
			Cases     int    `json:"independent_go_oracle_cases"`
			Results   []json.RawMessage
		}
		if err = json.Unmarshal(checked, &proof); err != nil {
			return err
		}
		if proof.Schema != "gooo/literal-text-report-checked/v1" || proof.Decision != "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY" || proof.Generated != "sha256:"+digest([]byte(source)) || proof.Execution != "sha256:"+digest(execRaw) || proof.Replay != "sha256:"+digest(replayRaw) || proof.Cases != len(batch.rows) || len(proof.Results) != len(batch.rows) {
			return fmt.Errorf("literal strong checked report binding changed")
		}
	}
	// Freeze check: later edits cannot be published with earlier receipts.
	for name, want := range sourceHashes {
		raw, err := os.ReadFile(name)
		if err != nil || digest(raw) != want {
			return fmt.Errorf("literal source changed during build: %s", name)
		}
	}
	currentManifest, err := os.ReadFile(literalManifest)
	if err != nil || !bytes.Equal(currentManifest, manifestRaw) {
		return fmt.Errorf("literal manifest changed during build")
	}
	if err = put("generated.go", []byte(generated)); err != nil {
		return err
	}
	if err = put("bridge.go", literalBridge); err != nil {
		return err
	}
	if err = write(filepath.Join(work, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	if err = write(filepath.Join(work, "bridge.go"), literalBridge); err != nil {
		return err
	}
	if _, err = command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}, "go", "-C", dir, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", filepath.Join(asset, "program.wasm"), "./literal-search-build"); err != nil {
		return err
	}
	fixture := literalFixtures{Schema: "gooo/literal-search-browser-fixtures/v1", CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion, Cases: append(rows, boundary...)}
	for _, p := range []struct{ name, text, needle string }{{"text-1025", strings.Repeat("a", 1025), ""}, {"needle-1025", "", strings.Repeat("a", 1025)}, {"text-multibyte-1026", strings.Repeat("가", 342), ""}, {"needle-multibyte-1026", "", strings.Repeat("가", 342)}} {
		fixture.Rejections = append(fixture.Rejections, literalRejection{p.name, append([]uint16{}, utf16.Encode([]rune(p.text))...), append([]uint16{}, utf16.Encode([]rune(p.needle))...)})
	}
	fixture.Rejections = append(fixture.Rejections, literalRejection{"text-surrogate-1026", repeatLiteralUnit(0xd800, 342), []uint16{}}, literalRejection{"needle-surrogate-1026", []uint16{}, repeatLiteralUnit(0xdfff, 342)})
	fixtureRaw, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	if err = put("fixtures.json", append(fixtureRaw, '\n')); err != nil {
		return err
	}
	for _, name := range []string{"LICENSE", "NOTICE", "upstream.json", "third_party/GO-LICENSE"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err = put(name, raw); err != nil {
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
	wasm, err := os.ReadFile(filepath.Join(asset, "program.wasm"))
	if err != nil {
		return err
	}
	hashes["program.wasm"] = digest(wasm)
	metadata := map[string]any{"schema": "gooo/browser-literal-search/v1", "entry_id": literalEntry, "result_entity_id": literalResult, "input_entity_ids": []string{"gooo://std/text", "gooo://std/text"}, "compiler_revision": lock.CompilerRevision, "compiler_version": lock.CompilerVersion, "go_version": lock.GoVersion, "source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0, "source_sha256": sourceHashes, "generated_sha256": hashes["generated.go"], "wasm_sha256": digest(wasm), "wasm_bytes": len(wasm), "execution_sha256": hashes["execution.json"], "replay_sha256": hashes["replay.json"], "fixtures_sha256": hashes["fixtures.json"], "files_sha256": hashes, "fixture_cases": len(fixture.Cases), "fixture_rejections": len(fixture.Rejections), "native_cases": len(fixture.Cases), "native_receipt_expectation_cases": 0, "scope": "Unchanged importing Gooo Report. Finite independent Go comparisons and input-only native OBSERVED receipts with zero expectations, strongly checked saved native replay; actual browser calls are unsigned browser observations, not receipts or attestations. Decoded valid UTF-8 Text <=1024 bytes per argument; JS lone UTF-16 surrogates become U+FFFD. Literal byte search, no normalization, regex or whole-library/performance claim."}
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(asset, "manifest.json"), append(raw, '\n'))
}
func repeatLiteralUnit(unit uint16, n int) []uint16 {
	v := make([]uint16, n)
	for i := range v {
		v[i] = unit
	}
	return v
}
