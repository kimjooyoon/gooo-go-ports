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

	"github.com/google/uuid"
)

const uuidParseManifest = "uuid-parse-canonical.workspace.json"
const uuidParseInputs = "examples/uuid-parse-canonical/inputs.json"
const uuidParseInputKey = "go/github.com/google/uuid/text:ParseCanonical"
const uuidParseEntry = "gooo://github.com/google/uuid/text/parse-canonical"
const uuidParseResult = "gooo://github.com/google/uuid/text/parse-canonical-result"

var uuidParseSourceFiles = []string{"ports/google/uuid/quote.gooo", "ports/google/uuid/validate.gooo", "ports/google/uuid/text/parse_canonical.gooo"}
var uuidParseNames = []string{"parsed", "canonical_uuid", "error", "invalid_length"}
var uuidParseFieldIDs = []string{"parsed", "canonical-uuid", "error", "invalid-length"}

// These hashes bind the unchanged Google v1.6.0 sources compiled into the host oracle.
var uuidParseOriginalHashes = map[string]string{
	"uuid.go": "0edec8e34c6b6fe0db31b71a29069a09ed832e3fd04ee0175916b58f2b60e5c1",
	"util.go": "755f50afafb13dd89ae6177da7eb6dc31ae650ece2f9ba52821021e9f8de044e",
	"LICENSE": "0a8d61ed3cbfd5312326e8126c31ce9c627a283adc99131b56896d29ada04b2d",
}

func uuidParseCheckOriginalSources(documents map[string][]byte) error {
	compiled := uuid.PinnedSourceSHA256()
	if len(compiled) != len(uuidParseOriginalHashes) {
		return fmt.Errorf("UUID compiled oracle source roster changed")
	}
	for name, want := range uuidParseOriginalHashes {
		if compiled[name] != want || digest(documents["third_party/google-uuid/"+name]) != want {
			return fmt.Errorf("UUID original source mismatch: %s", name)
		}
	}
	var upstream struct {
		ExternalSources []struct {
			Module        string            `json:"module"`
			Version       string            `json:"version"`
			Revision      string            `json:"revision"`
			Target        string            `json:"target"`
			ModuleH1      string            `json:"module_h1"`
			SourceSHA     string            `json:"source_sha256"`
			SupportingSHA map[string]string `json:"supporting_source_sha256"`
			LicenseSHA    string            `json:"license_sha256"`
		} `json:"external_sources"`
	}
	if err := json.Unmarshal(documents["upstream.json"], &upstream); err != nil {
		return err
	}
	matched := 0
	for _, port := range upstream.ExternalSources {
		if port.Target == "ports/google/uuid/text/parse_canonical.gooo" {
			if port.Module != "github.com/google/uuid" || port.Version != "v1.6.0" || port.Revision != "0f11ee6918f41a04c201eceeadf612a377bc7fbc" || port.ModuleH1 != "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=" || port.SourceSHA != uuidParseOriginalHashes["uuid.go"] || len(port.SupportingSHA) != 1 || port.SupportingSHA["util.go"] != uuidParseOriginalHashes["util.go"] || port.LicenseSHA != uuidParseOriginalHashes["LICENSE"] {
				return fmt.Errorf("UUID upstream source pin changed")
			}
			matched++
		}
	}
	if matched != 1 {
		return fmt.Errorf("UUID upstream parse pin missing or duplicated")
	}
	return nil
}

//go:embed uuid_parse_bridge.go.txt
var uuidParseBridge []byte

type uuidParseOutput struct {
	Parsed        bool   `json:"parsed"`
	CanonicalUUID string `json:"canonical_uuid"`
	Error         string `json:"error"`
	InvalidLength bool   `json:"invalid_length"`
}
type uuidParseCase struct {
	Name       string          `json:"name"`
	Input      string          `json:"input"`
	InputUTF16 []uint16        `json:"input_utf16"`
	Expected   uuidParseOutput `json:"expected"`
	inputJSON  json.RawMessage
}
type uuidParseRejection struct {
	Name       string   `json:"name"`
	InputUTF16 []uint16 `json:"input_utf16"`
}
type uuidParseFixtures struct {
	Schema           string               `json:"schema"`
	CompilerRevision string               `json:"compiler_revision"`
	GoVersion        string               `json:"go_version"`
	Cases            []uuidParseCase      `json:"cases"`
	Rejections       []uuidParseRejection `json:"rejections"`
}

// Original Google code is an independent build-host oracle, never a bridge dependency.
func uuidParseReference(text string) uuidParseOutput {
	value, err := uuid.Parse(text)
	message := ""
	if err != nil {
		message = err.Error()
	}
	return uuidParseOutput{err == nil, value.String(), message, uuid.IsInvalidLengthError(err)}
}
func uuidParseNewCase(name, text string) uuidParseCase {
	raw, _ := json.Marshal(text)
	return uuidParseCase{name, text, append([]uint16{}, utf16.Encode([]rune(text))...), uuidParseReference(text), raw}
}
func uuidParseRows(raw []byte) ([]uuidParseCase, error) {
	v, err := cropDecode(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := v.(map[string]any)
	if !ok || len(doc) != 2 || doc["schema"] != "gooo/body-composition-inputs/v1" {
		return nil, fmt.Errorf("invalid UUID input schema")
	}
	rows, ok := doc["inputs"].([]any)
	if !ok || len(rows) == 0 || len(rows) > 128 {
		return nil, fmt.Errorf("invalid UUID input roster")
	}
	result := []uuidParseCase{}
	texts := make([]string, len(rows))
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 1 {
			return nil, fmt.Errorf("invalid UUID input fields")
		}
		text, ok := row[uuidParseInputKey].(string)
		if !ok || !utf8.ValidString(text) || len(text) > 1024 {
			return nil, fmt.Errorf("one decoded valid UTF-8 Text <=1024 bytes required")
		}
		texts[i] = text
	}
	for i, text := range texts {
		result = append(result, uuidParseNewCase(fmt.Sprintf("supplied-%02d", i), text))
	}
	return result, nil
}
func uuidParseBoundaryCases() []uuidParseCase {
	const standard = "00112233-4455-6677-8899-aabbccddeeff"
	const raw = "00112233445566778899aabbccddeeff"
	rows := []uuidParseCase{}
	for _, p := range []struct{ name, text string }{
		{"zero-valid", "00000000-0000-0000-0000-000000000000"},
		{"raw-invalid-high-first", "g1" + raw[2:]},
		{"raw-invalid-low-first", "0g" + raw[2:]},
		{"standard-invalid-high-first", "g0" + standard[2:]},
		{"standard-invalid-low-first", "0g" + standard[2:]},
		{"raw-invalid-high-middle", raw[:14] + "g0" + raw[16:]},
		{"standard-invalid-high-middle", standard[:16] + "g0" + standard[18:]},
		{"hyphen-priority-before-decode", "g0" + standard[2:8] + "_" + standard[9:]},
		{"whitespace-delimiters-accepted", " " + standard + " "},
		{"leading-bom-preserved", "\ufeff" + standard},
		{"controls-preserved", "\x00\r\n"},
		{"urn-quoted-controls", "a\x00\r\n<>&\"x" + standard},
		{"urn-byte9-rune-cut", "12345678가" + strings.Repeat("a", 34)},
		{"length-31", strings.Repeat("a", 31)},
		{"length-37", standard + "x"},
		{"text-1023", strings.Repeat("a", 1023)},
		{"text-1024", strings.Repeat("a", 1024)},
		{"multibyte-1024", strings.Repeat("🙂", 256)},
		{"combining-preserved", "e\u0301"},
		{"html-line-separators", "<>&\u2028\u2029"},
	} {
		rows = append(rows, uuidParseNewCase(p.name, p.text))
	}
	for _, p := range []struct {
		name, text string
		units      []uint16
	}{
		{"lone-high", `"\ud800x"`, []uint16{0xd800, 'x'}},
		{"lone-low", `"x\udfff"`, []uint16{'x', 0xdfff}},
		{"mixed-surrogates", `"\ud800x\udfff"`, []uint16{0xd800, 'x', 0xdfff}},
		{"high-high-low", `"\ud800\ud83d\ude00"`, []uint16{0xd800, 0xd83d, 0xde00}},
	} {
		var text string
		_ = json.Unmarshal([]byte(p.text), &text)
		row := uuidParseNewCase(p.name, text)
		// Preserve browser units; uuidParseNewCase already encoded bound scalar Text.
		row.InputUTF16 = p.units
		rows = append(rows, row)
	}
	return rows
}
func uuidParseInputDocument(rows []uuidParseCase) ([]byte, error) {
	values := make([]map[string]json.RawMessage, len(rows))
	for i, row := range rows {
		values[i] = map[string]json.RawMessage{uuidParseInputKey: row.inputJSON}
	}
	raw, err := json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": values}, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(rows) == 0 || len(rows) > 128 || len(raw) > 32768 {
		return nil, fmt.Errorf("UUID input envelope exceeds native limits")
	}
	return raw, nil
}
func uuidParseCheckABI(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil {
		return err
	}
	recordName := "GoooRecord" + digest([]byte(uuidParseResult))
	entries, records := 0, 0
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "GoooComposedActivity0" {
			if fn.Recv != nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("UUID entry signature changed")
			}
			p := fn.Type.Params.List[0]
			t, ok := p.Type.(*ast.Ident)
			if !ok || t.Name != "string" || len(p.Names) != 1 || p.Names[0].Name != "input" {
				return fmt.Errorf("UUID Text ABI changed")
			}
			out := fn.Type.Results.List[0]
			t, ok = out.Type.(*ast.Ident)
			if !ok || t.Name != recordName || len(out.Names) != 0 {
				return fmt.Errorf("UUID result ABI changed")
			}
			entries++
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
			if !ok || len(s.Fields.List) != 4 {
				return fmt.Errorf("UUID result record changed")
			}
			for i, p := range s.Fields.List {
				kind := "string"
				if i == 0 || i == 3 {
					kind = "bool"
				}
				ft, ok := p.Type.(*ast.Ident)
				if !ok || ft.Name != kind || len(p.Names) != 1 || p.Names[0].Name != "GoooField"+digest([]byte(uuidParseResult+"/"+uuidParseFieldIDs[i])) || p.Tag == nil {
					return fmt.Errorf("UUID result field ABI changed")
				}
				tag, err := strconv.Unquote(p.Tag.Value)
				if err != nil || tag != fmt.Sprintf("json:%q", uuidParseNames[i]) {
					return fmt.Errorf("UUID result JSON field changed")
				}
			}
			records++
		}
	}
	if entries != 1 || records != 1 {
		return fmt.Errorf("UUID generated ABI incomplete or duplicated")
	}
	return nil
}

func buildUUIDParseDemo(ctx context.Context, site, dir string, lock lockfile) error {
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("UUID parser requires pinned Go")
	}
	asset, work := filepath.Join(site, "uuid-parse"), filepath.Join(dir, "uuid-parse-build")
	for _, p := range []string{asset, work} {
		if err := os.Mkdir(p, 0755); err != nil {
			return err
		}
	}
	documents := map[string][]byte{}
	for _, name := range []string{"LICENSE", "NOTICE", "upstream.json", "third_party/GO-LICENSE", "third_party/google-uuid/LICENSE", "third_party/google-uuid/uuid.go", "third_party/google-uuid/util.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		documents[name] = raw
	}
	if err := uuidParseCheckOriginalSources(documents); err != nil {
		return err
	}
	manifestRaw, err := os.ReadFile(uuidParseManifest)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(uuidParseInputs)
	if err != nil {
		return err
	}
	rows, err := uuidParseRows(original)
	if err != nil {
		return err
	}
	boundary := uuidParseBoundaryCases()
	boundaryRaw, err := uuidParseInputDocument(boundary)
	if err != nil {
		return err
	}
	sourceHashes := map[string]string{}
	for _, name := range uuidParseSourceFiles {
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
		rows []uuidParseCase
	}{{"", original, rows}, {"boundary-", boundaryRaw, boundary}} {
		input, execution, replay := filepath.Join(asset, batch.stem+"inputs.json"), filepath.Join(asset, batch.stem+"execution.json"), filepath.Join(asset, batch.stem+"replay.json")
		if err = put(batch.stem+"inputs.json", batch.raw); err != nil {
			return err
		}
		goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
		execRaw, err := command(ctx, nil, "./.gooo", "package", "execute", "--json", "--go", goTool, "--inputs", input, uuidParseManifest)
		if err != nil {
			return err
		}
		if err = put(batch.stem+"execution.json", execRaw); err != nil {
			return err
		}
		replayRaw, err := command(ctx, nil, "./.gooo", "package", "replay", "--json", "--go", goTool, "--receipt", execution, "--inputs", input, uuidParseManifest)
		if err != nil {
			return err
		}
		if err = put(batch.stem+"replay.json", replayRaw); err != nil {
			return err
		}
		checked, err := command(ctx, nil, "go", "run", "./cmd/check-uuid-parse-canonical", "--snapshot", ".", "--inputs", input, "--execution", execution, "--replay", replay, "--compiler-build", filepath.Join(asset, "compiler.json"))
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
			return fmt.Errorf("UUID generated source changed across batches")
		}
		if err = uuidParseCheckABI(source); err != nil {
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
		if proof.Schema != "gooo/uuid-parse-canonical-checked/v1" || proof.Decision != "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY" || proof.Generated != "sha256:"+digest([]byte(source)) || proof.Execution != "sha256:"+digest(execRaw) || proof.Replay != "sha256:"+digest(replayRaw) || proof.Cases != len(batch.rows) || len(proof.Results) != len(batch.rows) {
			return fmt.Errorf("UUID checked report binding changed")
		}
	}
	for name, want := range sourceHashes {
		raw, err := os.ReadFile(name)
		if err != nil || digest(raw) != want {
			return fmt.Errorf("UUID source changed during build: %s", name)
		}
	}
	currentManifest, err := os.ReadFile(uuidParseManifest)
	if err != nil || !bytes.Equal(currentManifest, manifestRaw) {
		return fmt.Errorf("UUID manifest changed during build")
	}
	if err = put("generated.go", []byte(generated)); err != nil {
		return err
	}
	if err = put("bridge.go", uuidParseBridge); err != nil {
		return err
	}
	if err = write(filepath.Join(work, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	if err = write(filepath.Join(work, "bridge.go"), uuidParseBridge); err != nil {
		return err
	}
	if _, err = command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}, "go", "-C", dir, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", filepath.Join(asset, "program.wasm"), "./uuid-parse-build"); err != nil {
		return err
	}
	fixture := uuidParseFixtures{Schema: "gooo/uuid-parse-browser-fixtures/v1", CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion, Cases: append(rows, boundary...)}
	for _, p := range []struct{ name, text string }{{"text-1025", strings.Repeat("a", 1025)}, {"text-multibyte-1026", strings.Repeat("가", 342)}} {
		fixture.Rejections = append(fixture.Rejections, uuidParseRejection{p.name, append([]uint16{}, utf16.Encode([]rune(p.text))...)})
	}
	fixture.Rejections = append(fixture.Rejections, uuidParseRejection{"text-surrogate-1026", repeatLiteralUnit(0xd800, 342)})
	fixtureRaw, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	if err = put("fixtures.json", append(fixtureRaw, '\n')); err != nil {
		return err
	}
	for name, raw := range documents {
		current, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(current, raw) {
			return fmt.Errorf("UUID published source/license changed during build: %s", name)
		}
		if err = put(name, raw); err != nil {
			return err
		}
	}
	currentInputs, err := os.ReadFile(uuidParseInputs)
	if err != nil || !bytes.Equal(currentInputs, original) {
		return fmt.Errorf("UUID original inputs changed during build")
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
	metadata := map[string]any{"schema": "gooo/browser-uuid-parse/v1", "entry_id": uuidParseEntry, "result_entity_id": uuidParseResult, "input_entity_ids": []string{"gooo://std/text"}, "compiler_revision": lock.CompilerRevision, "compiler_version": lock.CompilerVersion, "go_version": lock.GoVersion, "source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0, "source_sha256": sourceHashes, "generated_sha256": hashes["generated.go"], "wasm_sha256": digest(wasm), "wasm_bytes": len(wasm), "execution_sha256": hashes["execution.json"], "replay_sha256": hashes["replay.json"], "fixtures_sha256": hashes["fixtures.json"], "files_sha256": hashes, "fixture_cases": len(fixture.Cases), "fixture_rejections": len(fixture.Rejections), "native_cases": len(fixture.Cases), "native_receipt_expectation_cases": 0, "scope": "Unchanged Gooo ParseCanonical activity, adapting Google UUID Parse and returned UUID.String to four observations including partial values on error. Finite independent original Go comparisons and checked input-only native OBSERVED receipts with zero expectations and saved replay. Exact checker scope is recorded in checked-report.json; unsupported inner bindings are NOT_CHECKED. Actual browser calls are unsigned browser observations, not receipts or attestations. Decoded valid UTF-8 Text <=1024 bytes; JS lone UTF-16 surrogates become U+FFFD. No binary UUID API, arbitrary-byte strings, full-library, security or performance claim."}
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(asset, "manifest.json"), append(raw, '\n'))
}
