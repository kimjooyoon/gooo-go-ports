package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These small synthetic receipts test evidence consistency, not native execution.
// The formatting is deliberately preserved so the extraction test detects edits.
const generatedFixture = "package main\n\n// Preserve whitespace and UTF-8: 한글  \nfunc GoooComposedActivity0(input0 string, input1, input2, input3, input4 int64) int64 { return input1 }\n\n"

func evidenceFixture(t *testing.T) ([]byte, lockfile, map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	lock := lockfile{CompilerRevision: strings.Repeat("a", 40), CompilerVersion: "0.6.24-dev", GoVersion: "go1.27.2"}
	// Workspace source_digest hashes JSON-encoded source text, unlike the raw
	// generated_sha256. Keep this fixture aligned with the pinned compiler.
	sources := map[string]string{
		"ports/math/bits.gooo":            digest(testJSON(t, "package bits\n")),
		"examples/browser-demo/main.gooo": digest(testJSON(t, "package browser_demo\n")),
	}
	manifestRaw := []byte(`{"schema":"gooo/package-workspace-manifest/v1","entry":{"package_path":"app/browser-demo","activity":"Dispatch"},"packages":[{"path":"go/math/bits","sources":["ports/math/bits.gooo"]},{"path":"app/browser-demo","sources":["examples/browser-demo/main.gooo"]}]}`)
	if err := os.WriteFile(manifest, manifestRaw, 0644); err != nil {
		t.Fatal(err)
	}
	const key = "app/browser-demo:Dispatch"
	rows := []any{}
	traces := []any{}
	for i, n := range []int64{9007199254740993, 1} {
		output := map[string]any{"text": "", "first": n, "flag": false, "second": int64(0), "third": int64(0), "fourth": int64(0)}
		rows = append(rows, map[string]any{
			"inputs":   map[string]any{key + ".input0": "fixture.Identity", key + ".input1": n, key + ".input2": -n, key + ".input3": int64(0), key + ".input4": int64(255)},
			"expected": map[string]any{key: output},
		})
		traces = append(traces, map[string]any{"case_index": i, "deliveries": []any{map[string]any{
			"activity_id": "gooo://go-ports/examples/browser-demo/dispatch/v2",
			"inputs": []any{
				map[string]any{"port": "input0", "entity_id": "gooo://std/text", "value": "fixture.Identity"},
				map[string]any{"port": "input1", "entity_id": "gooo://std/integer", "value": n},
				map[string]any{"port": "input2", "entity_id": "gooo://std/integer", "value": -n},
				map[string]any{"port": "input3", "entity_id": "gooo://std/integer", "value": int64(0)},
				map[string]any{"port": "input4", "entity_id": "gooo://std/integer", "value": int64(255)},
			},
			"actual": output, "expected": output, "passed": true,
		}}})
	}
	casesRaw := testJSON(t, map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": rows})
	if err := os.MkdirAll(filepath.Dir(casesPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesPath, casesRaw, 0644); err != nil {
		t.Fatal(err)
	}
	packages := []any{}
	for _, pkg := range []struct{ path, source string }{
		{"go/math/bits", "ports/math/bits.gooo"},
		{"app/browser-demo", "examples/browser-demo/main.gooo"},
	} {
		entry := map[string]any{"path": pkg.path, "sources": []any{map[string]any{
			"filename": pkg.source, "source_digest": "sha256:" + sources[pkg.source],
		}}}
		if pkg.path == "app/browser-demo" {
			entry["exports"] = []any{map[string]any{"name": "Reply", "kind": "entity", "id": "gooo://go-ports/examples/browser-demo/reply/v2"}, map[string]any{
				"name": "Dispatch", "kind": "activity", "id": "gooo://go-ports/examples/browser-demo/dispatch/v2",
				"input_types": []string{"gooo://std/text", "gooo://std/integer", "gooo://std/integer", "gooo://std/integer", "gooo://std/integer"},
				"output_type": "gooo://go-ports/examples/browser-demo/reply/v2",
			}}
		}
		packages = append(packages, entry)
	}
	generatedSHA := "sha256:" + digest([]byte(generatedFixture))
	raw := testJSON(t, map[string]any{
		"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "PASS",
		"manifest_digest": "sha256:" + digest(manifestRaw), "cases_digest": "sha256:" + digest(casesRaw),
		"result": map[string]any{
			"program": map[string]any{
				"entry":           map[string]any{"package_path": "app/browser-demo", "activity": "Dispatch"},
				"workspace_image": map[string]any{"packages": packages},
			},
			"composition": map[string]any{"stage": "COMPLETE", "source": generatedFixture, "generated_sha256": generatedSHA,
				"steps": []any{map[string]any{"generation": map[string]any{"report": map[string]any{
					"activity_id": "gooo://go-ports/examples/browser-demo/dispatch/v2",
				}}}},
			},
			"runtime": map[string]any{
				"stage": "COMPLETE", "producer_source_sha": lock.CompilerRevision, "go_version": "go version go1.27.2 darwin/arm64",
				"generated_sha256": generatedSHA, "model_calls": 0, "finite_passed": len(rows), "finite_total": len(rows),
				"projection_replayed": true, "runtime_replayed": true, "traces": traces,
			},
		},
	})
	return raw, lock, sources
}

func testJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decodeReceipt(t *testing.T, raw []byte) receipt {
	t.Helper()
	var e receipt
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func testInt(n int) *int { return &n }

func TestInspectPreservesGeneratedSourceBytes(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	got, err := inspectReceipt(raw, lock, sources, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != generatedFixture {
		t.Fatal("generated source was reformatted, trimmed or otherwise changed")
	}
	if digest([]byte(got)) != digest([]byte(generatedFixture)) {
		t.Fatal("generated source byte digest changed")
	}
}

func TestInspectRejectsTamperedEvidence(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	tests := map[string]func(*receipt){
		"receipt schema":  func(e *receipt) { e.Schema = "wrong" },
		"failed decision": func(e *receipt) { e.Decision = "FAIL_CLOSED" },
		"compiler error":  func(e *receipt) { e.Error = "failed" },
		"manifest hash":   func(e *receipt) { e.ManifestDigest = "sha256:" + strings.Repeat("0", 64) },
		"cases hash":      func(e *receipt) { e.CasesDigest = "sha256:" + strings.Repeat("0", 64) },
		"entry package":   func(e *receipt) { e.Result.Program.Entry.Package = "wrong" },
		"entry activity":  func(e *receipt) { e.Result.Program.Entry.Activity = "Wrong" },
		"source contents": func(e *receipt) {
			e.Result.Program.Image.Packages[0].Sources[0].Digest = "sha256:" + strings.Repeat("0", 64)
		},
		"source digest without algorithm": func(e *receipt) {
			s := &e.Result.Program.Image.Packages[0].Sources[0]
			s.Digest = strings.TrimPrefix(s.Digest, "sha256:")
		},
		"source filename": func(e *receipt) { e.Result.Program.Image.Packages[0].Sources[0].Filename = "another.gooo" },
		"missing source": func(e *receipt) {
			e.Result.Program.Image.Packages[0].Sources = nil
		},
		"duplicate source": func(e *receipt) {
			p := &e.Result.Program.Image.Packages[0]
			p.Sources = append(p.Sources, p.Sources[0])
		},
		"generated text edited": func(e *receipt) { e.Result.Composition.Source += "// edited\n" },
		"generated text rehashed but runtime unchanged": func(e *receipt) {
			c := &e.Result.Composition
			c.Source += "// edited\n"
			c.Digest = "sha256:" + digest([]byte(c.Source))
		},
		"generated hash":      func(e *receipt) { e.Result.Composition.Digest = "sha256:" + strings.Repeat("0", 64) },
		"runtime source hash": func(e *receipt) { e.Result.Runtime.Digest = "sha256:" + strings.Repeat("0", 64) },
		"incomplete generation": func(e *receipt) {
			e.Result.Composition.Stage = "BODY_PREFLIGHT"
		},
		"wrong compiler":      func(e *receipt) { e.Result.Runtime.Producer = strings.Repeat("b", 40) },
		"wrong Go version":    func(e *receipt) { e.Result.Runtime.Go = "go version go1.26.5 darwin/arm64" },
		"incomplete runtime":  func(e *receipt) { e.Result.Runtime.Stage = "EXECUTE_1" },
		"runtime failure":     func(e *receipt) { e.Result.Runtime.Failure = json.RawMessage(`"panic"`) },
		"missing model count": func(e *receipt) { e.Result.Runtime.Calls = nil },
		"model invocation":    func(e *receipt) { e.Result.Runtime.Calls = testInt(1) },
		"missing total":       func(e *receipt) { e.Result.Runtime.Total = nil },
		"false total":         func(e *receipt) { e.Result.Runtime.Total = testInt(1) },
		"missing pass count":  func(e *receipt) { e.Result.Runtime.Passed = nil },
		"false pass count":    func(e *receipt) { e.Result.Runtime.Passed = testInt(1) },
		"projection not replayed": func(e *receipt) {
			e.Result.Runtime.Projection = false
		},
		"runtime not replayed": func(e *receipt) { e.Result.Runtime.Replayed = false },
		"missing trace":        func(e *receipt) { e.Result.Runtime.Traces = e.Result.Runtime.Traces[:1] },
		"wrong case index":     func(e *receipt) { e.Result.Runtime.Traces[0].Index = 1 },
		"extra delivery": func(e *receipt) {
			tr := &e.Result.Runtime.Traces[0]
			tr.Deliveries = append(tr.Deliveries, tr.Deliveries[0])
		},
		"wrong stable ID": func(e *receipt) { e.Result.Runtime.Traces[0].Deliveries[0].ID = "other" },
		"case failed":     func(e *receipt) { e.Result.Runtime.Traces[0].Deliveries[0].Passed = false },
		"missing argument": func(e *receipt) {
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.Inputs = d.Inputs[:2]
		},
		"input order": func(e *receipt) {
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.Inputs[1], d.Inputs[2] = d.Inputs[2], d.Inputs[1]
		},
		"rounded int64 input": func(e *receipt) {
			e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = json.RawMessage(`9007199254740992`)
		},
		"changed operation": func(e *receipt) {
			e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = json.RawMessage(`"other"`)
		},
		"rounded int64 output": func(e *receipt) {
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.Actual = json.RawMessage(strings.Replace(string(d.Actual), "9007199254740993", "9007199254740992", 1))
		},
		"invented expectation": func(e *receipt) {
			e.Result.Runtime.Traces[0].Deliveries[0].Expected = json.RawMessage(`{}`)
		},
		"unexpected replay binding": func(e *receipt) { e.ReplayedFrom = "sha256:" + strings.Repeat("0", 64) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			e := decodeReceipt(t, raw)
			mutate(&e)
			if _, err := inspectReceipt(testJSON(t, e), lock, sources, ""); err == nil {
				t.Fatal("accepted tampered evidence")
			}
		})
	}
}

func TestInspectSavedReplayBinding(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	prior := "sha256:" + digest(raw)
	e := decodeReceipt(t, raw)
	e.ReplayedFrom = prior
	e.Result.Replay = &struct {
		ModelCalls *int `json:"model_calls"`
	}{ModelCalls: testInt(0)}
	replay := testJSON(t, e)
	if got, err := inspectReceipt(replay, lock, sources, prior); err != nil || got != generatedFixture {
		t.Fatalf("valid saved replay: %q, %v", got, err)
	}
	for name, check := range map[string]struct {
		raw  []byte
		from string
	}{
		"unrelated original receipt": {replay, "sha256:" + strings.Repeat("0", 64)},
		"execute claimed as replay":  {raw, prior},
		"replay claimed as execute":  {replay, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := inspectReceipt(check.raw, lock, sources, check.from); err == nil {
				t.Fatal("accepted wrong saved replay binding")
			}
		})
	}
	for _, calls := range []*int{nil, testInt(1)} {
		e.Result.Replay.ModelCalls = calls
		if _, err := inspectReceipt(testJSON(t, e), lock, sources, prior); err == nil {
			t.Fatal("accepted missing or nonzero replay model calls")
		}
	}
	e.Result.Replay = nil
	if _, err := inspectReceipt(testJSON(t, e), lock, sources, prior); err == nil {
		t.Fatal("accepted missing replay metadata")
	}
}

func TestInspectBindsFilesAtInspection(t *testing.T) {
	for _, path := range []string{manifest, casesPath} {
		t.Run(path, func(t *testing.T) {
			raw, lock, sources := evidenceFixture(t)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := inspectReceipt(raw, lock, sources, ""); err == nil {
				t.Fatal("accepted stale receipt after local file changed")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := inspectReceipt(raw, lock, sources, ""); err == nil {
				t.Fatal("accepted receipt without bound input file")
			}
		})
	}
}

func TestSameJSONPreservesIntegerPrecisionAndRejectsTrailingData(t *testing.T) {
	for _, value := range []string{"9007199254740993", "9223372036854775807", "-9223372036854775808"} {
		if !sameJSON([]byte(value), []byte(value)) {
			t.Fatalf("lost exact integer %s", value)
		}
	}
	if sameJSON([]byte(`9007199254740993`), []byte(`9007199254740992`)) {
		t.Fatal("rounded distinct integers")
	}
	if !sameJSON([]byte(`{"a":1,"b":false}`), []byte(`{"b":false,"a":1}`)) {
		t.Fatal("object order changed value identity")
	}
	for _, invalid := range []string{"", "1 2", `{} {}`, "1 garbage"} {
		if sameJSON([]byte(invalid), []byte("1")) || sameJSON([]byte("1"), []byte(invalid)) {
			t.Fatalf("accepted invalid/trailing JSON %q", invalid)
		}
	}
}
