package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeWireAllEighteenSourceFixturesAndValidBase(t *testing.T) {
	root := filepath.Join("testdata", "wire")
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	value, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	m := object(value)
	if !keys(m, "schema", "count", "base_named", "base_native", "cases") || m["schema"] != "gooo/redelivery-wire-fixtures/v1" || !isInt(m["count"], 36) {
		t.Fatal("wire manifest schema/roster")
	}
	base := []byte(str(m["base_native"]))
	rows, err := checkNativeWire(base)
	if err != nil || len(rows) != 1 {
		t.Fatal("positive native fixture must reach policy boundary", err)
	}
	names := []string{"missing", "unknown", "null-text", "null-boolean", "null-integer", "bool-as-integer", "number-as-boolean", "fractional-integer", "out-of-int64", "duplicate-exact", "duplicate-escaped", "raw-invalid-utf8", "unpaired-high", "unpaired-low", "text-1025", "trailing", "malformed", "nonobject"}
	codes := []string{"MISSING_FIELD", "UNKNOWN_FIELD", "TEXT_TYPE", "BOOLEAN_TYPE", "INTEGER_TOKEN", "INTEGER_TOKEN", "BOOLEAN_TYPE", "INTEGER_TOKEN", "INTEGER_RANGE", "DUPLICATE_FIELD", "DUPLICATE_FIELD", "INVALID_RAW_UTF8", "INVALID_UNICODE_SCALAR", "INVALID_UNICODE_SCALAR", "TEXT_BYTE_BOUND", "TRAILING_JSON", "MALFORMED_JSON", "OBJECT_REQUIRED"}
	cases := array(m["cases"])
	if len(cases) != 36 {
		t.Fatal("exact36 wire sources")
	}
	seen := map[string]bool{}
	for i, v := range cases {
		c := object(v)
		format := "named"
		if i >= 18 {
			format = "native"
		}
		name := format + "-" + names[i%18]
		file := str(c["file"])
		if !keys(c, "name", "format", "file", "bytes", "sha256", "exit_code", "stderr") || c["name"] != name || c["format"] != format || file != name+".json" || filepath.Base(file) != file || seen[file] || !isInt(c["exit_code"], 1) || c["stderr"] != "WIRE_REFUSED: "+codes[i%18] {
			t.Fatal("wire source exact order/diagnostic", i)
		}
		seen[file] = true
		body, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if !isInt(c["bytes"], len(body)) || c["sha256"] != strings.TrimPrefix(digest(body), "sha256:") {
			t.Fatal("wire source bytes/hash", name)
		}
		if format == "native" {
			t.Run(name, func(t *testing.T) {
				_, err := checkNativeWire(body)
				if err == nil || err.Error() != c["stderr"] {
					t.Fatalf("wrong refusal gate: %v want%s", err, c["stderr"])
				}
			})
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 37 {
		t.Fatal("unexpected fixture entry", err)
	}
	for _, e := range entries {
		if e.IsDir() || (e.Name() != "manifest.json" && !seen[e.Name()]) {
			t.Fatal("unbound fixture", e.Name())
		}
	}
}
func TestNativeWireSemanticInvalidAndBatchBounds(t *testing.T) {
	row := map[string]any{}
	for i := 0; i < 9; i++ {
		row[appPackage+":"+activityName+".input"+string(rune('0'+i))] = fixtureInputs()[fieldNames[i]]
	}
	for _, value := range []json.Number{"-9223372036854775808", "9223372036854775807", "-1", "3"} {
		row[appPackage+":"+activityName+".input7"] = value
		row[appPackage+":"+activityName+".input0"] = "not-a-method"
		raw, _ := json.Marshal(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": []any{row}})
		rows, err := checkNativeWire(raw)
		if err != nil || rows[0]["attempts_remaining"] != value || rows[0]["method"] != "not-a-method" {
			t.Fatal("well-typed semantic-invalid facts must be preserved", err)
		}
	}
	for _, count := range []int{0, 1, 8, 9} {
		rows := []any{}
		for i := 0; i < count; i++ {
			rows = append(rows, row)
		}
		raw, _ := json.Marshal(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": rows})
		got, err := checkNativeWire(raw)
		if count == 1 || count == 8 {
			if err != nil || len(got) != count {
				t.Fatal("valid fixed batch", count, err)
			}
		} else if err == nil {
			t.Fatal("unbounded batch admitted", count)
		}
	}
	for _, size := range []int{1024, 1025} {
		row[appPackage+":"+activityName+".input0"] = strings.Repeat("x", size)
		raw, _ := json.Marshal(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": []any{row}})
		_, err := checkNativeWire(raw)
		if (err == nil) != (size == 1024) {
			t.Fatal("decoded UTF8 byte bound", size, err)
		}
	}
}
func TestCurrentPublicFreezeRejectsForeignDuplicateAndCoercedSources(t *testing.T) {
	repository := canonicalTestTempDir(t)
	path := filepath.Join(repository, "source")
	if err := os.WriteFile(path, []byte("current"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := currentRef(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(canonicalTestTempDir(t), "tool")
	if err := os.WriteFile(outside, []byte("tool"), 0600); err != nil {
		t.Fatal(err)
	}
	tool, err := currentRef(outside, 32)
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"schema": "gooo/redelivery-public-runtime-source-freeze/v1", "repository_root": repository, "files": []any{r}, "tools": []any{tool}, "overall_seconds": 300}
	check := func(v any) error {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		p := filepath.Join(canonicalTestTempDir(t), "freeze.json")
		if err = os.WriteFile(p, raw, 0600); err != nil {
			return err
		}
		ref, err := currentRef(p, 1<<20)
		if err != nil {
			return err
		}
		_, _, _, err = loadRuntimeFreeze(clone(t, ref), repository)
		return err
	}
	if err := check(base); err != nil {
		t.Fatal("positive current-public freeze", err)
	}
	for _, name := range []string{"foreign-source", "duplicate", "empty", "wrong-schema", "foreign-root", "bool-time", "float-time", "too-long", "unknown-field"} {
		t.Run(name, func(t *testing.T) {
			m := object(clone(t, base))
			switch name {
			case "foreign-source":
				m["files"] = []any{tool}
			case "duplicate":
				m["files"] = []any{r, r}
			case "empty":
				m["files"] = []any{}
			case "wrong-schema":
				m["schema"] = "historical-private-freeze"
			case "foreign-root":
				m["repository_root"] = "/synthetic/foreign"
			case "bool-time":
				m["overall_seconds"] = true
			case "float-time":
				m["overall_seconds"] = json.Number("300.0")
			case "too-long":
				m["overall_seconds"] = 301
			case "unknown-field":
				m["private_acceptance"] = true
			}
			if check(m) == nil {
				t.Fatal("invalid runtime source boundary admitted")
			}
		})
	}
}

func TestOracleAndOriginBytesMustMatchTheirBoundReference(t *testing.T) {
	path := filepath.Join(canonicalTestTempDir(t), "evidence.json")
	if err := os.WriteFile(path, []byte(`{"n":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := currentRef(path, 64)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := readCurrentBytes(r, 64)
	if err != nil || string(raw) != `{"n":1}` {
		t.Fatal("positive bound read", err)
	}
	if err := os.WriteFile(path, []byte(`{"n":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCurrentBytes(r, 64); err == nil || !strings.Contains(err.Error(), "bound raw reference") {
		t.Fatal("same-size oracle/origin byte change bypassed binding", err)
	}
}
