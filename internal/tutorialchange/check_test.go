package tutorialchange

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func originals(t *testing.T) [4][]byte {
	t.Helper()
	var raw [4][]byte
	for i, name := range []string{"before.json", "after.json", "replay.json", "compiler-build.json"} {
		var err error
		raw[i], err = profileFiles.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
	}
	return raw
}

func TestRetainedOriginalObservations(t *testing.T) {
	raw := originals(t)
	summary, err := Check(raw[0], raw[1], raw[2], raw[3])
	if err != nil {
		t.Fatal(err)
	}
	if summary.RepositoryRevision != nil || summary.RepositoryRevisionStatus != "NOT_CAPTURED" {
		t.Fatal("invented repository revision")
	}
	if len(summary.Cases) != 4 || len(summary.Sources) != 2 || summary.Sources[0].Changed || !summary.Sources[1].Changed {
		t.Fatal("source/case roster")
	}
	changed := 0
	for _, row := range summary.Cases {
		if len(row.Fields) != 4 {
			t.Fatal("four fields required")
		}
		for _, f := range row.Fields {
			if !equalValue(f.After, f.Replay) {
				t.Fatal("fresh replay differs")
			}
			if f.Changed {
				changed++
				if row.Index != 2 || f.Name != "label" || f.Before != "sensor: high" || f.After != "sensor: low" {
					t.Fatal("wrong finite diff")
				}
			}
		}
	}
	if changed != 1 {
		t.Fatal("exactly one changed field")
	}
	if summary.Sources[1].AfterRawSHA256 == summary.Sources[1].AfterReceiptDigest {
		t.Fatal("raw file and JSON-string digest conflated")
	}
}

type documents [4]map[string]any

func decoded(t *testing.T) documents {
	t.Helper()
	raw := originals(t)
	var result documents
	for i := range raw {
		v, err := decode(raw[i])
		if err != nil {
			t.Fatal(err)
		}
		result[i] = object(v)
	}
	return result
}

func runtimeDoc(d map[string]any) map[string]any   { return object(object(d["result"])["runtime"]) }
func composition(d map[string]any) map[string]any  { return object(object(d["result"])["composition"]) }
func trace(d map[string]any, n int) map[string]any { return object(array(runtimeDoc(d)["traces"])[n]) }
func delivery(d map[string]any, n int) map[string]any {
	return object(array(trace(d, n)["deliveries"])[0])
}

// Preserve the pinned producer's typed object field order while changing test
// values. encoding/json on a map sorts keys and is not the pin's typed codec.
func marshalLike(t *testing.T, value any, template []byte) []byte {
	t.Helper()
	switch v := value.(type) {
	case string:
		return cliString(v)
	case map[string]any:
		d := json.NewDecoder(bytes.NewReader(template))
		if first, err := d.Token(); err == nil && first == json.Delim('{') {
			var out bytes.Buffer
			out.WriteByte('{')
			count := 0
			seen := map[string]bool{}
			write := func(key string, child []byte) {
				if count != 0 {
					out.WriteByte(',')
				}
				k, _ := json.Marshal(key)
				out.Write(k)
				out.WriteByte(':')
				out.Write(child)
				count++
			}
			for d.More() {
				token, err := d.Token()
				if err != nil {
					t.Fatal(err)
				}
				key := token.(string)
				var child json.RawMessage
				if err := d.Decode(&child); err != nil {
					t.Fatal(err)
				}
				if a, ok := v[key]; ok {
					write(key, marshalLike(t, a, child))
					seen[key] = true
				}
			}
			var extra []string
			for key := range v {
				if !seen[key] {
					extra = append(extra, key)
				}
			}
			sort.Strings(extra)
			for _, key := range extra {
				write(key, marshalLike(t, v[key], nil))
			}
			out.WriteByte('}')
			return out.Bytes()
		}
	case []any:
		var children []json.RawMessage
		_ = json.Unmarshal(template, &children)
		var out bytes.Buffer
		out.WriteByte('[')
		for i, child := range v {
			if i != 0 {
				out.WriteByte(',')
			}
			var ref []byte
			if i < len(children) {
				ref = children[i]
			}
			out.Write(marshalLike(t, child, ref))
		}
		out.WriteByte(']')
		return out.Bytes()
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Rebind the two independently named parent coordinates after semantic mutation.
// A test must reach its intended guard, rather than fail at a stale outer hash.
func encodeLinked(t *testing.T, docs documents) [4][]byte {
	t.Helper()
	var raw [4][]byte
	templates := originals(t)
	for i := 0; i < 3; i++ {
		var envelope, result map[string]json.RawMessage
		if err := json.Unmarshal(templates[i], &envelope); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(envelope["result"], &result); err != nil {
			t.Fatal(err)
		}
		c := marshalLike(t, composition(docs[i]), result["composition"])
		encoded, err := json.Marshal(json.RawMessage(c))
		if err != nil {
			t.Fatal(err)
		}
		runtimeDoc(docs[i])["composition_sha256"] = digest(encoded)
	}
	for i := 0; i < 2; i++ {
		raw[i] = marshalLike(t, docs[i], templates[i])
	}
	docs[2]["replayed_from_sha256"] = digest(raw[1])
	parent, err := pinnedResultDigest(raw[1])
	// Unsupported typed numbers cannot be reserialized by the pin's codec.
	// The profile rejects them before inspecting the parent coordinate.
	if err != nil {
		parent = digest([]byte("unsupported typed mutation"))
	}
	object(object(docs[2]["result"])["replay"])["prior_result_sha256"] = parent
	for i := 2; i < 4; i++ {
		raw[i] = marshalLike(t, docs[i], templates[i])
	}
	return raw
}

func TestLinuxAndDifferentObservedAccounting(t *testing.T) {
	docs := decoded(t)
	for i := 0; i < 3; i++ {
		rt := runtimeDoc(docs[i])
		rt["go_version"] = "go version go1.27.2 linux/amd64"
		rt["go_tool_selection"] = "path"
		rt["go_tool_sha256"] = digest([]byte("another observed Go1.27.2 platform tool"))
		rt["executable_sha256"] = digest([]byte("observed generated binary"))
		rt["elapsed_ns"] = json.Number("77")
		c := composition(docs[i])
		c["elapsed_ns"] = json.Number("99")
		for _, proc := range []map[string]any{object(rt["toolchain"]), object(rt["build"]), object(array(rt["runs"])[0]), object(array(rt["runs"])[1])} {
			for _, key := range []string{"wall_ns", "user_ns", "system_ns", "peak_rss_bytes"} {
				proc[key] = json.Number("123")
			}
		}
		object(rt["toolchain"])["stdout_sha256"] = digest([]byte("go version go1.27.2 linux/amd64\n"))
		gen := object(object(array(c["steps"])[0])["generation"])
		scope := object(object(object(gen["report"])["completeness_receipt"])["scope"])
		scope["execution_environment"] = "linux/amd64"
	}
	raw := encodeLinked(t, docs)
	if _, err := Check(raw[0], raw[1], raw[2], raw[3]); err != nil {
		t.Fatal(err)
	}
}

// Only explicit/path were observed in the retained native executions. The
// other tags are source-defined offline compatibility, not executions of those
// selection routes. Actual Go version and binary hash still match all records.
func TestSupportedToolSelectionMethods(t *testing.T) {
	for _, selection := range []string{"explicit", "path", "compiler_goroot", "toolchain_cache", "path_fallback"} {
		t.Run(selection, func(t *testing.T) {
			docs := decoded(t)
			runtimeDoc(docs[1])["go_tool_selection"] = selection
			runtimeDoc(docs[2])["go_tool_selection"] = "path"
			raw := encodeLinked(t, docs)
			summary, err := Check(raw[0], raw[1], raw[2], raw[3])
			if err != nil {
				t.Fatal(err)
			}
			selections := object(summary.ReceiptCompiler["go_tool_selection_by_record"])
			if selections["before"] != "explicit" || selections["after"] != selection || selections["replay"] != "path" {
				t.Fatal("per-record Go tool selection methods not preserved")
			}
		})
	}
}

func TestReboundSemanticTampering(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(documents)
		target string
	}{
		{"immutable value", func(d documents) { object(delivery(d[1], 2)["actual"])["remaining"] = json.Number("24") }, "tutorial value/type differs"},
		{"integer boolean", func(d documents) { object(delivery(d[1], 2)["actual"])["score"] = true }, "tutorial value/type differs"},
		{"integer float spelling", func(d documents) { object(delivery(d[1], 2)["actual"])["score"] = json.Number("75.0") }, "tutorial value/type differs"},
		{"field URI", func(d documents) { object(array(delivery(d[1], 2)["actual_fields"])[0])["id"] = "urn:other" }, "tutorial value/type differs"},
		{"duplicate case index", func(d documents) { trace(d[1], 2)["case_index"] = json.Number("1") }, "tutorial value/type differs"},
		{"input port order", func(d documents) { a := array(delivery(d[1], 2)["inputs"]); a[0], a[1] = a[1], a[0] }, "tutorial value/type differs"},
		{"missing case", func(d documents) { runtimeDoc(d[1])["traces"] = array(runtimeDoc(d[1])["traces"])[:3] }, "unsupported array roster"},
		{"source identity", func(d documents) {
			image := object(object(object(d[1]["result"])["program"])["workspace_image"])
			object(array(object(array(image["packages"])[1])["sources"])[0])["source_digest"] = digest([]byte("wrong source"))
		}, "tutorial value/type differs"},
		{"compiler revision", func(d documents) { runtimeDoc(d[1])["producer_source_sha"] = "other" }, "tutorial value/type differs"},
		{"compiler module", func(d documents) { object(runtimeDoc(d[1])["producer_module"])["version"] = "other" }, "tutorial value/type differs"},
		{"provider", func(d documents) {
			g := object(object(array(composition(d[1])["steps"])[0])["generation"])
			object(object(g["report"])["route_decision"])["provider"] = "other"
		}, "tutorial value/type differs"},
		{"unknown completeness scope", func(d documents) {
			g := object(object(array(composition(d[1])["steps"])[0])["generation"])
			object(object(object(g["report"])["completeness_receipt"])["scope"])["unknown"] = true
		}, "unsupported object shape"},
		{"record field type URI", func(d documents) {
			plan := object(composition(d[1])["plan"])
			object(array(object(array(plan["record_types"])[0])["fields"])[0])["type_id"] = "urn:gooo:type:boolean"
		}, "tutorial value/type differs"},
		{"Go version", func(d documents) { runtimeDoc(d[1])["go_version"] = "go version go1.27.1 darwin/arm64" }, "unsupported Go tool"},
		{"Go tool cohort", func(d documents) { runtimeDoc(d[1])["go_tool_sha256"] = digest([]byte("other Go binary")) }, "actual tool/compiler identity differs"},
		{"Go tool selection domain", func(d documents) { runtimeDoc(d[1])["go_tool_selection"] = "environment" }, "invalid observed value"},
		{"single route latency", func(d documents) {
			g := object(object(array(composition(d[1])["steps"])[0])["generation"])
			object(g["report"])["route_decision_latency_ms"] = json.Number("0.25")
		}, "tutorial value/type differs"},
		{"process not complete", func(d documents) { object(runtimeDoc(d[1])["build"])["completed"] = false }, "tutorial value/type differs"},
		{"negative process CPU", func(d documents) { object(runtimeDoc(d[1])["build"])["user_ns"] = json.Number("-1") }, "invalid observed value"},
		{"overflow process CPU", func(d documents) { object(runtimeDoc(d[1])["build"])["user_ns"] = json.Number("9223372036854775808") }, "invalid observed value"},
		{"runtime pair differs", func(d documents) {
			object(array(runtimeDoc(d[1])["runs"])[0])["stdout_sha256"] = digest([]byte("other observed output"))
		}, "tutorial value/type differs"},
		{"matching forged runtime pair", func(d documents) {
			for _, run := range array(runtimeDoc(d[1])["runs"]) {
				object(run)["stdout_sha256"] = digest([]byte("same invented output"))
			}
		}, "tutorial value/type differs"},
		{"build stdout differs", func(d documents) {
			object(runtimeDoc(d[1])["build"])["stdout_sha256"] = digest([]byte("invented compiler output"))
		}, "tutorial value/type differs"},
		{"Go version output differs", func(d documents) {
			object(runtimeDoc(d[1])["toolchain"])["stdout_sha256"] = digest([]byte("other version output"))
		}, "Go version process output binding differs"},
		{"separate build dirty", func(d documents) { d[3]["vcs_modified"] = "true" }, "exact clean pinned compiler"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docs := decoded(t)
			test.mutate(docs)
			raw := encodeLinked(t, docs)
			_, err := Check(raw[0], raw[1], raw[2], raw[3])
			if err == nil || !strings.Contains(err.Error(), test.target) {
				t.Fatalf("want %q, got %v", test.target, err)
			}
		})
	}
}

func TestRawParentAndPinnedTypedResultCoordinates(t *testing.T) {
	for _, which := range []string{"raw parent", "typed Result parent", "own composition"} {
		t.Run(which, func(t *testing.T) {
			raw := originals(t)
			v, _ := decode(raw[2])
			e := object(v)
			if which == "raw parent" {
				e["replayed_from_sha256"] = digest(raw[0])
			}
			if which == "typed Result parent" {
				object(object(e["result"])["replay"])["prior_result_sha256"] = digest(raw[1])
			}
			if which == "own composition" {
				runtimeDoc(e)["composition_sha256"] = digest([]byte("other composition"))
			}
			var err error
			raw[2], err = json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			// Re-marshaling changes generic-map order, so repair only the own
			// composition coordinate unless it is the target of this test.
			if which != "own composition" {
				c, _ := json.Marshal(composition(e))
				runtimeDoc(e)["composition_sha256"] = digest(c)
				raw[2], _ = json.Marshal(e)
			}
			_, err = Check(raw[0], raw[1], raw[2], raw[3])
			if err == nil {
				t.Fatal("accepted swapped parent/binding")
			}
			want := map[string]string{"raw parent": "raw parent", "typed Result parent": "typed Result parent", "own composition": "own serialized composition"}[which]
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("want %q guard, got %v", want, err)
			}
		})
	}
}

func TestStrictJSONBoundary(t *testing.T) {
	tests := []struct {
		name   string
		raw    func([]byte) []byte
		target string
	}{
		{"decoded duplicate", func(b []byte) []byte {
			return append([]byte(`{"decision":"OBSERVED","\u0064ecision":"OBSERVED",`), b[1:]...)
		}, "duplicate JSON key"},
		{"unknown envelope", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schema":`), []byte(`"unknown": true, "schema":`), 1)
		}, "unsupported object shape"},
		{"invalid UTF8", func(b []byte) []byte { return append([]byte{0xff}, b...) }, "raw UTF-8"},
		{"oversized", func(b []byte) []byte { return bytes.Repeat([]byte(" "), MaxReceiptBytes+1) }, "JSON size"},
		{"trailing", func(b []byte) []byte { return append(b, []byte(` {}`)...) }, "trailing JSON"},
		{"depth", func(b []byte) []byte { return []byte(strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)) }, "64 nested levels"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := originals(t)
			raw[0] = test.raw(raw[0])
			_, err := Check(raw[0], raw[1], raw[2], raw[3])
			if err == nil || !strings.Contains(err.Error(), test.target) {
				t.Fatalf("want %q got %v", test.target, err)
			}
		})
	}
}
