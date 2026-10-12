package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

const goldenPriorResult = "sha256:9283884c6026ae620152b2ac6353162acd5c6491e12737855ea6e1859bfa1ab1"

func goldenReceipt(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/pinned-original-execution.json")
	if err != nil {
		t.Fatal(err)
	}
	if digest(raw) != "sha256:1bb2b3420f28836452e93a0bcda271d36ebebbfa41bd0e173f61b307c6a02e60" {
		t.Fatal("original pilot fixture changed")
	}
	return raw
}

func rawAt(t *testing.T, raw []byte, keys ...string) []byte {
	t.Helper()
	for _, key := range keys {
		if key == "0" {
			var a []json.RawMessage
			if err := json.Unmarshal(raw, &a); err != nil || len(a) == 0 {
				t.Fatal("missing fixture array", err)
			}
			raw = a[0]
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil || m[key] == nil {
			t.Fatal("missing fixture key", key, err)
		}
		raw = m[key]
	}
	return raw
}

// Reorder opaque generic maps only. Typed object order must remain untouched.
func reverseObjectKeys(v any) []byte {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		var out bytes.Buffer
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			encoded, _ := json.Marshal(key)
			out.Write(encoded)
			out.WriteByte(':')
			out.Write(reverseObjectKeys(x[key]))
		}
		out.WriteByte('}')
		return out.Bytes()
	case []any:
		var items []json.RawMessage
		for _, value := range x {
			items = append(items, reverseObjectKeys(value))
		}
		raw, _ := json.Marshal(items)
		return raw
	default:
		raw, _ := json.Marshal(x)
		return raw
	}
}

func TestPinnedOriginalTypedResultDigest(t *testing.T) {
	raw := goldenReceipt(t)
	encoded, err := pinnedResultBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 95075 || digest(encoded) != goldenPriorResult {
		t.Fatalf("typed original result mismatch: bytes=%d digest=%s", len(encoded), digest(encoded))
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, rawAt(t, raw, "result")); err != nil {
		t.Fatal(err)
	}
	if digest(compact.Bytes()) == goldenPriorResult || digest(raw) == goldenPriorResult {
		t.Fatal("envelope/compact digest unexpectedly substitutes for typed result")
	}

	for _, field := range []string{"scope", "status_counts"} {
		t.Run("reordered generic "+field, func(t *testing.T) {
			originalMap := rawAt(t, raw, "result", "composition", "steps", "0", "generation", "report", "completeness_receipt", field)
			value, err := decode(originalMap)
			if err != nil {
				t.Fatal(err)
			}
			reordered := bytes.Replace(raw, originalMap, reverseObjectKeys(value), 1)
			got, err := pinnedResultDigest(reordered)
			if err != nil || got != goldenPriorResult {
				t.Fatal("generic key order changed typed digest", got, err)
			}
			object(value)["different_value"] = json.Number("9007199254740993")
			changed := bytes.Replace(raw, originalMap, reverseObjectKeys(value), 1)
			got, err = pinnedResultDigest(changed)
			if err != nil || got == goldenPriorResult {
				t.Fatal("generic value change did not change digest", got, err)
			}
		})
	}
}

func normalizeSample(t *testing.T, raw, path string, shape profileShape) []byte {
	t.Helper()
	var profile resultProfile
	if err := json.Unmarshal(resultProfileJSON, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Nodes[path] = shape
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	out, err := normalizeResultValue(d, []byte(raw), path, profile.Nodes, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err = json.Marshal(json.RawMessage(out))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPinnedSourceDeclaredTimingAndFloatVariations(t *testing.T) {
	// These tags/types mirror ProcessTiming at pin 83928fa, independently of the
	// observed values. Zero wait limits omit a scalar; a nonnil deadline retains
	// even a zero pointee. Ordinary same-pin measurements must remain supported.
	type timing struct {
		StartNS             int64  `json:"start_ns"`
		WaitNS              int64  `json:"wait_ns"`
		WaitLimitNS         int64  `json:"wait_limit_ns,omitempty"`
		DeadlineRemainingNS *int64 `json:"deadline_remaining_ns,omitempty"`
		Context             string `json:"context_at_start_return"`
	}
	var profile resultProfile
	if err := json.Unmarshal(resultProfileJSON, &profile); err != nil {
		t.Fatal(err)
	}
	zero, nonzero := int64(0), int64(123456)
	for _, path := range []string{"$.runtime.toolchain.timing", "$.runtime.build.timing", "$.runtime.runs.*.timing"} {
		for i, value := range []timing{
			{Context: "ACTIVE"}, {StartNS: 1, WaitNS: 2, WaitLimitNS: 3, Context: "ACTIVE"},
			{DeadlineRemainingNS: &zero, Context: "ACTIVE"}, {WaitLimitNS: 3, DeadlineRemainingNS: &nonzero, Context: "ACTIVE"},
		} {
			want, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			got := normalizeSample(t, string(want), path, profile.Nodes[path])
			if !bytes.Equal(got, want) {
				t.Fatalf("timing %s case %d: got=%s want=%s", path, i, got, want)
			}
		}
	}
	for _, path := range []string{
		"$.composition.steps.*.generation.report.route_decision_latency_ms",
		"$.composition.steps.*.generation.report.completeness_percent",
		"$.composition.steps.*.generation.report.completeness_receipt.aggregate_completeness_score",
	} {
		for _, number := range []float64{0, 0.125, 99.75, 100, 1e-9, 1e30} {
			want, err := json.Marshal(number)
			if err != nil {
				t.Fatal(err)
			}
			got := normalizeSample(t, string(want), path, profile.Nodes[path])
			if !bytes.Equal(got, want) {
				t.Fatalf("float64 %s: got=%s want=%s", path, got, want)
			}
		}
	}
	for _, raw := range []string{"null", "0", "9007199254740993"} {
		path := "$.runtime.build.peak_rss_bytes"
		if got := normalizeSample(t, raw, path, profile.Nodes[path]); string(got) != raw {
			t.Fatal("nullable int64 changed", string(got))
		}
	}
}

func TestPinnedSerializerEscapingAndOpaqueValues(t *testing.T) {
	text := "<>&\u2028\u2029"
	want := `"\u003c\u003e\u0026\u2028\u2029"`
	if got := normalizeSample(t, string(cliString(text)), "$", profileShape{Kinds: []string{"string"}}); string(got) != want {
		t.Fatalf("typed string escaping: %s", got)
	}
	rawPath := "$.runtime.traces.*.deliveries.*.actual"
	raw := `{"z":9007199254740993,"a":"<>&\u2028\u2029\u0061","values":[2,1]}`
	want = `{"z":9007199254740993,"a":"\u003c\u003e\u0026\u2028\u2029\u0061","values":[2,1]}`
	if got := normalizeSample(t, raw, rawPath, profileShape{}); string(got) != want {
		t.Fatalf("RawMessage lost order, number token or existing escape: %s", got)
	}
	scopePath := "$.composition.steps.*.generation.report.completeness_receipt.scope"
	raw = `{"z":[{"b":9007199254740993,"a":"<>&\u2028\u2029"},2,1],"a":"\u0061"}`
	want = `{"a":"a","z":[{"a":"\u003c\u003e\u0026\u2028\u2029","b":9007199254740993},2,1]}`
	if got := normalizeSample(t, raw, scopePath, profileShape{}); string(got) != want {
		t.Fatalf("generic map lost sorted keys, exact number or array order: %s", got)
	}
}

func TestPinnedResultRejectsUnsupportedFormats(t *testing.T) {
	raw := goldenReceipt(t)
	result := rawAt(t, raw, "result")
	program := rawAt(t, result, "program")
	schema := rawAt(t, result, "schema")
	prefix := append(append(append([]byte(`{"schema":`), schema...), []byte(`,"program":`)...), program...)
	reordered := append(append(append([]byte(`{"program":`), program...), []byte(`,"schema":`)...), schema...)
	route := rawAt(t, raw, "result", "composition", "steps", "0", "generation", "report", "route_decision")
	newRoute := bytes.Replace(route, []byte(`"request_sha256":`), []byte(`"routing":{},"request_sha256":`), 1)
	timing := rawAt(t, raw, "result", "runtime", "toolchain", "timing")
	emptyOptional := bytes.Replace(timing, []byte(`"deadline_remaining_ns":`), []byte(`"wait_limit_ns":0,"deadline_remaining_ns":`), 1)
	mutations := map[string][]byte{
		"typed field reorder":     bytes.Replace(raw, prefix, reordered, 1),
		"case alias":              bytes.Replace(raw, []byte(`"execution_source_sha256":`), []byte(`"Execution_source_sha256":`), 1),
		"unknown typed field":     bytes.Replace(raw, []byte(`"execution_source_sha256":`), []byte(`"new_field":0,"execution_source_sha256":`), 1),
		"optional routing map":    bytes.Replace(raw, route, newRoute, 1),
		"explicit omitempty zero": bytes.Replace(raw, timing, emptyOptional, 1),
		"duplicate":               append([]byte(`{"schema":"other",`), raw[1:]...),
		"unknown envelope":        append([]byte(`{"new_field":0,`), raw[1:]...),
		"trailing data":           append(append([]byte{}, raw...), []byte(` {}`)...),
		"invalid UTF-8":           bytes.Replace(raw, []byte("OBSERVED"), []byte{0xff}, 1),
		"noncanonical integer":    bytes.Replace(raw, []byte(`"finite_total":0`), []byte(`"finite_total":0.0`), 1),
		"typed escaped scalar":    bytes.Replace(raw, []byte(`"OBSERVED"`), []byte(`"\u004fBSERVED"`), 1),
	}
	// The envelope decision is not part of the typed Result. Exercise a typed
	// string instead, where decoding/remarshaling must normalize existing escapes.
	mutations["typed escaped scalar"] = bytes.Replace(raw, []byte(`"gooo/workspace-body-execution/v1"`), []byte(`"\u0067ooo/workspace-body-execution/v1"`), 1)
	for name, changed := range mutations {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, changed) {
				t.Fatal("mutation did not alter fixture")
			}
			if _, err := pinnedResultDigest(changed); err == nil {
				t.Fatal("unsupported format accepted")
			}
		})
	}
	for _, keys := range [][]string{
		{"result", "composition", "steps", "0", "generation", "report", "completeness_receipt", "scope"},
		{"result", "runtime", "traces", "0", "deliveries", "0", "actual"},
	} {
		nested := strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)
		original := rawAt(t, raw, keys...)
		changed := bytes.Replace(raw, original, []byte(`{"deep":`+nested+`}`), 1)
		if _, err := pinnedResultDigest(changed); err == nil {
			t.Fatal("deep opaque data accepted", keys)
		}
	}
}
