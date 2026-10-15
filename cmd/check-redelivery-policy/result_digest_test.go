package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Resolve shared public fixtures from this source, never the caller's cwd.
func codecSibling(t *testing.T, name string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(source) {
		t.Fatal("absolute codec test source path required")
	}
	return filepath.Join(filepath.Dir(source), "..", name)
}
func codecProfile(t *testing.T) resultProfile {
	t.Helper()
	if err := loadResultProfile(codecSibling(t, "check-path-facts/result-profile-83928fa.json")); err != nil {
		t.Fatal(err)
	}
	var p resultProfile
	if err := json.Unmarshal(resultProfileJSON, &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func codecGolden(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(codecSibling(t, "check-literal-text-report/testdata/pinned-original-execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(b) != "sha256:1bb2b3420f28836452e93a0bcda271d36ebebbfa41bd0e173f61b307c6a02e60" {
		t.Fatal("shared original fixture changed")
	}
	return b
}
func codecAt(t *testing.T, raw []byte, keys ...string) []byte {
	t.Helper()
	for _, key := range keys {
		if key == "0" {
			var a []json.RawMessage
			if json.Unmarshal(raw, &a) != nil || len(a) == 0 {
				t.Fatal("fixture array")
			}
			raw = a[0]
		} else {
			var m map[string]json.RawMessage
			if json.Unmarshal(raw, &m) != nil || m[key] == nil {
				t.Fatal("fixture key", key)
			}
			raw = m[key]
		}
	}
	return raw
}
func codecValue(raw, path string, p map[string]profileShape) ([]byte, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	b, err := normalizeResultValue(d, []byte(raw), path, p, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(json.RawMessage(b))
}

func TestResultCodecProfileBinding(t *testing.T) {
	prior := resultProfileJSON
	t.Cleanup(func() { resultProfileJSON = prior })
	resultProfileJSON = nil
	if _, err := pinnedResultBytes([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "not loaded") {
		t.Fatal("uninitialized profile accepted", err)
	}
	path := codecSibling(t, "check-path-facts/result-profile-83928fa.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := append([]byte{}, good...)
	mutated[len(mutated)-2] ^= 1
	wrongCompiler := bytes.Replace(good, []byte("83928fa"), []byte("93928fa"), 1)
	for i, bad := range [][]byte{nil, good[:len(good)-1], append(append([]byte{}, good...), '\n'), mutated, wrongCompiler} {
		if err := loadResultProfile(path); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(file, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if err := loadResultProfile(file); err == nil || len(resultProfileJSON) != 0 {
			t.Fatal("invalid load retained usable state", i, err)
		}
	}
	for _, bad := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		if err := loadResultProfile(path); err != nil {
			t.Fatal(err)
		}
		if err := loadResultProfile(bad); err == nil || len(resultProfileJSON) != 0 {
			t.Fatal("failed acquisition reused profile", err)
		}
	}
	codecProfile(t)
	if len(resultProfileJSON) != expectedResultProfileBytes || !bytes.Equal(resultProfileJSON, good) {
		t.Fatal("exact shared bytes not loaded")
	}
}

func TestResultCodecGoldenAndRefusals(t *testing.T) {
	codecProfile(t)
	raw := codecGolden(t)
	const want = "sha256:9283884c6026ae620152b2ac6353162acd5c6491e12737855ea6e1859bfa1ab1"
	b, err := pinnedResultBytes(raw)
	if err != nil || len(b) != 95075 || digest(b) != want {
		t.Fatal("typed original result differs", len(b), err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, codecAt(t, raw, "result")); err != nil {
		t.Fatal(err)
	}
	if digest(compact.Bytes()) == want || digest(raw) == want {
		t.Fatal("envelope/compact substituted for typed result")
	}
	for _, field := range []string{"scope", "status_counts"} {
		old := codecAt(t, raw, "result", "composition", "steps", "0", "generation", "report", "completeness_receipt", field)
		x := bytes.Replace(raw, old, []byte(`{"b":9007199254740993,"a":1}`), 1)
		y := bytes.Replace(raw, old, []byte(`{"a":1,"b":9007199254740993}`), 1)
		dx, ex := pinnedResultDigest(x)
		dy, ey := pinnedResultDigest(y)
		if ex != nil || ey != nil || dx != dy {
			t.Fatal("generic key sorting changed digest", field, ex, ey)
		}
		z := bytes.Replace(raw, old, []byte(`{"b":9007199254740994,"a":1}`), 1)
		dz, ez := pinnedResultDigest(z)
		if ez != nil || dz == dx {
			t.Fatal("generic number change lost", field, ez)
		}
	}
	result := codecAt(t, raw, "result")
	schema := codecAt(t, result, "schema")
	program := codecAt(t, result, "program")
	first := append(append(append([]byte(`{"schema":`), schema...), []byte(`,"program":`)...), program...)
	swapped := append(append(append([]byte(`{"program":`), program...), []byte(`,"schema":`)...), schema...)
	route := codecAt(t, raw, "result", "composition", "steps", "0", "generation", "report", "route_decision")
	timing := codecAt(t, raw, "result", "runtime", "toolchain", "timing")
	bad := map[string][]byte{
		"order":                bytes.Replace(raw, first, swapped, 1),
		"case":                 bytes.Replace(raw, []byte(`"execution_source_sha256":`), []byte(`"Execution_source_sha256":`), 1),
		"typed field":          bytes.Replace(raw, []byte(`"execution_source_sha256":`), []byte(`"extra":0,"execution_source_sha256":`), 1),
		"optional map":         bytes.Replace(raw, route, bytes.Replace(route, []byte(`"request_sha256":`), []byte(`"routing":{},"request_sha256":`), 1), 1),
		"omitempty zero":       bytes.Replace(raw, timing, bytes.Replace(timing, []byte(`"deadline_remaining_ns":`), []byte(`"wait_limit_ns":0,"deadline_remaining_ns":`), 1), 1),
		"duplicate":            append([]byte(`{"schema":"other",`), raw[1:]...),
		"envelope":             append([]byte(`{"extra":0,`), raw[1:]...),
		"trailing":             append(append([]byte{}, raw...), []byte(` {}`)...),
		"UTF8":                 bytes.Replace(raw, []byte("OBSERVED"), []byte{0xff}, 1),
		"integer":              bytes.Replace(raw, []byte(`"finite_total":0`), []byte(`"finite_total":0.0`), 1),
		"escaped typed scalar": bytes.Replace(raw, []byte(`"gooo/workspace-body-execution/v1"`), []byte(`"\u0067ooo/workspace-body-execution/v1"`), 1),
	}
	for name, value := range bad {
		if bytes.Equal(value, raw) {
			t.Fatal("ineffective mutation", name)
		}
		if _, err := pinnedResultDigest(value); err == nil {
			t.Fatal("unsupported form accepted", name)
		}
	}
	for _, keys := range [][]string{{"result", "composition", "steps", "0", "generation", "report", "completeness_receipt", "scope"}, {"result", "runtime", "traces", "0", "deliveries", "0", "actual"}} {
		deep := []byte(`{"deep":` + strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66) + `}`)
		if _, err := pinnedResultDigest(bytes.Replace(raw, codecAt(t, raw, keys...), deep, 1)); err == nil {
			t.Fatal("deep opaque input accepted", keys)
		}
	}
}

func TestResultCodecTimingFloatAndOpaqueValues(t *testing.T) {
	p := codecProfile(t)
	type timing struct {
		StartNS             int64  `json:"start_ns"`
		WaitNS              int64  `json:"wait_ns"`
		WaitLimitNS         int64  `json:"wait_limit_ns,omitempty"`
		DeadlineRemainingNS *int64 `json:"deadline_remaining_ns,omitempty"`
		Context             string `json:"context_at_start_return"`
	}
	zero, nonzero := int64(0), int64(123456)
	for _, path := range []string{"$.runtime.toolchain.timing", "$.runtime.build.timing", "$.runtime.runs.*.timing"} {
		for _, value := range []timing{{Context: "ACTIVE"}, {StartNS: 1, WaitNS: 2, WaitLimitNS: 3, Context: "ACTIVE"}, {DeadlineRemainingNS: &zero, Context: "ACTIVE"}, {WaitLimitNS: 3, DeadlineRemainingNS: &nonzero, Context: "ACTIVE"}} {
			want, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			got, err := codecValue(string(want), path, p.Nodes)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatal("timing optional/pointer value", path, string(got), err)
			}
		}
	}
	for _, path := range []string{"$.composition.steps.*.generation.report.route_decision_latency_ms", "$.composition.steps.*.generation.report.completeness_percent", "$.composition.steps.*.generation.report.completeness_receipt.aggregate_completeness_score"} {
		for _, value := range []float64{0, 0.125, 99.75, 100, 1e-9, 1e30} {
			want, _ := json.Marshal(value)
			got, err := codecValue(string(want), path, p.Nodes)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatal("float64", path, string(got), err)
			}
		}
	}
	for _, raw := range []string{"null", "0", "9007199254740993"} {
		got, err := codecValue(raw, "$.runtime.build.peak_rss_bytes", p.Nodes)
		if err != nil || string(got) != raw {
			t.Fatal("nullable int64", err)
		}
	}
	p.Nodes["$"] = profileShape{Kinds: []string{"string"}}
	samples := []struct{ raw, path, want string }{
		{string(cliString("<>&\u2028\u2029")), "$", `"\u003c\u003e\u0026\u2028\u2029"`},
		{`{"z":9007199254740993,"a":"<>&\u2028\u2029\u0061","values":[2,1]}`, "$.runtime.traces.*.deliveries.*.actual", `{"z":9007199254740993,"a":"\u003c\u003e\u0026\u2028\u2029\u0061","values":[2,1]}`},
		{`{"z":[{"b":9007199254740993,"a":"<>&\u2028\u2029"},2,1],"a":"\u0061"}`, "$.composition.steps.*.generation.report.completeness_receipt.scope", `{"a":"a","z":[{"a":"\u003c\u003e\u0026\u2028\u2029","b":9007199254740993},2,1]}`},
	}
	for _, sample := range samples {
		got, err := codecValue(sample.raw, sample.path, p.Nodes)
		if err != nil || string(got) != sample.want {
			t.Fatal("escaping/raw/sorted map", sample.path, string(got), err)
		}
	}
	for _, raw := range []string{`""`, "0", "false", "1", "1.0", "true"} {
		got, err := codecValue(raw, "$.runtime.traces.*.deliveries.*.actual", p.Nodes)
		if err != nil || string(got) != raw {
			t.Fatal("raw scalar kind/spelling changed", raw, string(got), err)
		}
	}
	for _, raw := range []string{`{"x":1.0}`, `{"x":true}`, `{"x":-0}`, `{"x":9223372036854775808}`} {
		if _, err := codecValue(raw, "$.composition.steps.*.generation.report.completeness_receipt.status_counts", p.Nodes); err == nil {
			t.Fatal("status count accepted", raw)
		}
	}
}
