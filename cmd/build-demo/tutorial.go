package main

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
)

const tutorialManifest = "learn-gooo.workspace.json"
const tutorialInputs = "examples/learn-gooo/inputs.json"
const tutorialActivity = "urn:learn-gooo:describe"
const tutorialResult = "urn:learn-gooo:summary"

var tutorialSources = map[string]string{"example/limits": "examples/learn-gooo/limits.gooo", "app/report": "examples/learn-gooo/main.gooo"}
var tutorialNames = []string{"score", "remaining", "in_range", "label"}
var tutorialIDs = []string{"urn:learn-gooo:score", "urn:learn-gooo:remaining", "urn:learn-gooo:in-range-field", "urn:learn-gooo:label"}

//go:embed tutorial_bridge.go.txt
var tutorialBridge []byte

type tutorialOutput struct {
	Score     string `json:"score"`
	Remaining string `json:"remaining"`
	InRange   bool   `json:"in_range"`
	Label     string `json:"label"`
}
type tutorialCase struct {
	Name       string         `json:"name"`
	Inputs     [2]string      `json:"inputs"`
	LabelUTF16 []uint16       `json:"label_utf16"`
	Expected   tutorialOutput `json:"expected"`
	labelJSON  json.RawMessage
}
type tutorialRejection struct {
	Name       string    `json:"name"`
	Inputs     [2]string `json:"inputs"`
	LabelUTF16 []uint16  `json:"label_utf16"`
	Native     bool      `json:"native"`
	labelJSON  json.RawMessage
}
type tutorialFixtures struct {
	Schema           string              `json:"schema"`
	GoVersion        string              `json:"go_version"`
	CompilerRevision string              `json:"compiler_revision"`
	Cases            []tutorialCase      `json:"cases"`
	Rejections       []tutorialRejection `json:"rejections"`
}

// This independent reference is used only by the build host, never the bridge.
func tutorialOracle(n int64, label string) map[string]any {
	score := max(int64(0), min(int64(100), n))
	suffix := ": low"
	if score >= 60 {
		suffix = ": high"
	}
	return map[string]any{"score": score, "remaining": 100 - score, "in_range": n >= 0 && n <= 100, "label": label + suffix}
}
func tutorialExpected(n int64, label string) tutorialOutput {
	v := tutorialOracle(n, label)
	return tutorialOutput{strconv.FormatInt(v["score"].(int64), 10), strconv.FormatInt(v["remaining"].(int64), 10), v["in_range"].(bool), v["label"].(string)}
}
func tutorialFixtureSet(lock lockfile) tutorialFixtures {
	f := tutorialFixtures{Schema: "gooo/tutorial-browser-fixtures/v1", GoVersion: lock.GoVersion, CompilerRevision: lock.CompilerRevision}
	add := func(name string, n int64, label string) {
		raw, _ := json.Marshal(label)
		f.Cases = append(f.Cases, tutorialCase{name, [2]string{strconv.FormatInt(n, 10), label}, append([]uint16{}, utf16.Encode([]rune(label))...), tutorialExpected(n, label), raw})
	}
	for i, n := range []int64{-5, 42, 75, 105} {
		add(fmt.Sprintf("documented-%d", i+1), n, "sensor")
	}
	for i, n := range []int64{-1 << 63, -9007199254740993, -1, 0, 1, 59, 60, 61, 99, 100, 101, 9007199254740993, 1<<63 - 1} {
		add(fmt.Sprintf("integer-boundary-%d", i), n, "boundary")
	}
	for i, label := range []string{"", "  spaced  ", "quotes\" slash\\ tab\t NUL\x00 CRLF\r\n", "한글 😀 𐐀 e\u0301", "<script>&\u2028\u2029", "\ufffd", "\ufeffBOM", "é", "e\u0301", strings.Repeat("a", 1019), strings.Repeat("한", 339) + "a"} {
		add(fmt.Sprintf("text-boundary-%d", i), 42, label)
	}
	add("high-text-1018", 60, strings.Repeat("a", 1018))
	// The original JSON spellings are executed natively. The UTF-16 units are
	// supplied to the real Wasm callback, so replacement is checked at both ends.
	for _, x := range []struct {
		name, raw string
		units     []uint16
	}{{"lone-high", `"\ud800"`, []uint16{0xd800}}, {"lone-low", `"\udfff"`, []uint16{0xdfff}}, {"pair", `"\ud83d\ude00"`, []uint16{0xd83d, 0xde00}}, {"mixed-surrogates", `"\ud800x\udfff"`, []uint16{0xd800, 'x', 0xdfff}}, {"high-high-low", `"\ud800\ud83d\ude00"`, []uint16{0xd800, 0xd83d, 0xde00}}} {
		var label string
		_ = json.Unmarshal([]byte(x.raw), &label)
		f.Cases = append(f.Cases, tutorialCase{x.name, [2]string{"60", label}, x.units, tutorialExpected(60, label), json.RawMessage(x.raw)})
	}
	reject := func(name, score, label string, native bool) {
		raw, _ := json.Marshal(label)
		f.Rejections = append(f.Rejections, tutorialRejection{name, [2]string{score, label}, append([]uint16{}, utf16.Encode([]rune(label))...), native, raw})
	}
	for _, x := range []struct{ name, score, label string }{{"output-low-1025", "42", strings.Repeat("a", 1020)}, {"output-high-1025", "60", strings.Repeat("a", 1019)}, {"input-1024-output-too-large", "42", strings.Repeat("a", 1024)}, {"input-1025", "42", strings.Repeat("a", 1025)}} {
		reject(x.name, x.score, x.label, true)
	}
	for _, score := range []string{"", "+1", "01", "-0", "1.0", "1e2", " 1", "1 ", "9223372036854775808", "-9223372036854775809"} {
		reject("integer-transport-"+strconv.Itoa(len(f.Rejections)), score, "sensor", false)
	}
	return f
}
func tutorialInputDocument(rows []tutorialCase) ([]byte, error) {
	values := make([]map[string]json.RawMessage, len(rows))
	for i, row := range rows {
		values[i] = map[string]json.RawMessage{"app/report:Describe.input0": json.RawMessage(row.Inputs[0]), "app/report:Describe.input1": row.labelJSON}
	}
	return json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": values}, "", "  ")
}

func buildTutorialDemo(ctx context.Context, site, dir string, lock lockfile) error {
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("tutorial requires pinned Go")
	}
	asset, work := filepath.Join(site, "tutorial"), filepath.Join(dir, "tutorial-build")
	for _, p := range []string{asset, work} {
		if err := os.Mkdir(p, 0755); err != nil {
			return err
		}
	}
	manifestRaw, err := os.ReadFile(tutorialManifest)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(tutorialInputs)
	if err != nil {
		return err
	}
	sources := map[string][]byte{}
	sourceHashes := map[string]string{}
	for _, name := range tutorialSources {
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		sources[name] = raw
		sourceHashes[name] = digest(raw)
		if err := write(filepath.Join(asset, "bound-source", name), raw); err != nil {
			return err
		}
	}
	sampleHashes := map[string]string{}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, name := range []string{"limits.gooo", "main.gooo", "inputs.json", "gooo.workspace.json", "NOTICE", "README.md"} {
		raw, err := os.ReadFile(filepath.Join("examples/learn-gooo", name))
		if err != nil {
			return err
		}
		sampleHashes[name] = digest(raw)
		if err := write(filepath.Join(asset, "source", name), raw); err != nil {
			return err
		}
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		if _, err = w.Write(raw); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	if err := write(filepath.Join(asset, "source.zip"), archive.Bytes()); err != nil {
		return err
	}
	f := tutorialFixtureSet(lock)
	boundary, err := tutorialInputDocument(f.Cases[4:])
	if err != nil {
		return err
	}
	if len(boundary) > 32<<10 {
		return fmt.Errorf("tutorial boundary input envelope exceeds32KiB")
	}
	var generated string
	hashes := map[string]string{}
	for _, batch := range []struct {
		stem string
		raw  []byte
	}{{"", original}, {"boundary-", boundary}} {
		inputPath := filepath.Join(asset, batch.stem+"inputs.json")
		if err := write(inputPath, batch.raw); err != nil {
			return err
		}
		execution, err := command(ctx, nil, "./.gooo", "package", "execute", "--json", "--inputs", inputPath, tutorialManifest)
		if err != nil {
			return err
		}
		source, err := inspectTutorialReceipt(execution, lock, manifestRaw, batch.raw, sources, "")
		if err != nil {
			return err
		}
		if generated != "" && source != generated {
			return fmt.Errorf("tutorial generated source changed between input batches")
		}
		generated = source
		executionPath := filepath.Join(asset, batch.stem+"execution.json")
		if err := write(executionPath, execution); err != nil {
			return err
		}
		replay, err := command(ctx, nil, "./.gooo", "package", "replay", "--json", "--receipt", executionPath, "--inputs", inputPath, tutorialManifest)
		if err != nil {
			return err
		}
		replaySource, err := inspectTutorialReceipt(replay, lock, manifestRaw, batch.raw, sources, "sha256:"+digest(execution))
		if err != nil {
			return err
		}
		if replaySource != generated {
			return fmt.Errorf("tutorial replay source mismatch")
		}
		if err := write(filepath.Join(asset, batch.stem+"replay.json"), replay); err != nil {
			return err
		}
		for name, raw := range map[string][]byte{batch.stem + "inputs.json": batch.raw, batch.stem + "execution.json": execution, batch.stem + "replay.json": replay} {
			hashes[name] = digest(raw)
		}
	}
	// Rejection envelopes are executed by the real compiler too, rather than
	// inferring its Text input/output limits from the browser implementation.
	nativeRejected := 0
	for _, row := range f.Rejections {
		if !row.Native {
			continue
		}
		raw, err := tutorialInputDocument([]tutorialCase{{Inputs: row.Inputs, labelJSON: row.labelJSON}})
		if err != nil {
			return err
		}
		name := fmt.Sprintf("rejected-%02d", nativeRejected)
		p := filepath.Join(asset, name+"-inputs.json")
		if err := write(p, raw); err != nil {
			return err
		}
		observation, commandErr := tutorialRejectedCommand(ctx, p)
		if commandErr == nil {
			return fmt.Errorf("native tutorial unexpectedly accepted %s", row.Name)
		}
		// JSON stdout is retained even on nonzero exit; plain process errors
		// cannot count as an observed native rejection.
		if err := inspectTutorialRejection(observation, manifestRaw, raw, row.Name, lock, generated); err != nil {
			return err
		}
		if err := write(filepath.Join(asset, name+"-execution.json"), observation); err != nil {
			return err
		}
		hashes[name+"-inputs.json"] = digest(raw)
		hashes[name+"-execution.json"] = digest(observation)
		nativeRejected++
	}
	for name, raw := range map[string][]byte{"generated.go": []byte(generated), "workspace.json": manifestRaw} {
		if err := write(filepath.Join(asset, name), raw); err != nil {
			return err
		}
		hashes[name] = digest(raw)
	}
	if err := write(filepath.Join(work, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	if err := write(filepath.Join(work, "bridge.go"), tutorialBridge); err != nil {
		return err
	}
	if _, err := command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}, "go", "-C", dir, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", filepath.Join(asset, "program.wasm"), "./tutorial-build"); err != nil {
		return err
	}
	fixtureRaw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	fixtureRaw = append(fixtureRaw, '\n')
	if err := write(filepath.Join(asset, "fixtures.json"), fixtureRaw); err != nil {
		return err
	}
	hashes["fixtures.json"] = digest(fixtureRaw)
	for _, name := range []string{"LICENSE", "upstream.json"} {
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
	wasm, err := os.ReadFile(filepath.Join(asset, "program.wasm"))
	if err != nil {
		return err
	}
	metadata := map[string]any{"schema": "gooo/browser-tutorial/v1", "entry_id": tutorialActivity, "result_entity_id": tutorialResult, "input_entity_ids": []string{"urn:gooo:type:integer", "urn:gooo:type:string"}, "compiler_revision": lock.CompilerRevision, "compiler_version": lock.CompilerVersion, "go_version": lock.GoVersion, "source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0, "source_sha256": sourceHashes, "sample_sha256": sampleHashes, "source_zip_sha256": digest(archive.Bytes()), "source_zip_bytes": archive.Len(), "generated_sha256": digest([]byte(generated)), "wasm_sha256": digest(wasm), "wasm_bytes": len(wasm), "execution_sha256": hashes["execution.json"], "replay_sha256": hashes["replay.json"], "fixtures_sha256": hashes["fixtures.json"], "files_sha256": hashes, "fixture_cases": len(f.Cases), "fixture_rejections": len(f.Rejections), "native_cases": len(f.Cases), "native_rejections": nativeRejected, "scope": "Original tutorial Gooo compiled without edits. Four manual examples, finite build-host reference checks and saved native replay; browser calls are local observations, not native receipts. Text follows native JSON decoding and 1024 UTF-8-byte input/output limits. Hashes establish local consistency, not signatures."}
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(asset, "manifest.json"), append(raw, '\n'))
}
