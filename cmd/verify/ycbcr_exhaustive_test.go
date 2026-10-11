package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func ycbcrFixtureSource(operation, mutation string) string {
	contract, err := ycbcrOperationContract(operation)
	if err != nil {
		panic(err)
	}
	var text strings.Builder
	text.WriteString("package main\ntype Unrelated struct { Value bool }\ntype Result struct {\n")
	for i, name := range contract.Fields {
		fmt.Fprintf(&text, "Component%d int64 `json:%q`\n", i, name)
	}
	text.WriteString("}\nfunc Convert(a int64, b int64, c int64) Result {\n")
	// Synthetic checker fixtures, not the Gooo port. The arithmetic constants
	// are derived from Go's image/color/ycbcr.go (Copyright 2011 The Go Authors;
	// BSD license retained in third_party/GO-LICENSE). Native tests still invoke the Go
	// image/color oracle independently for every tuple.
	if operation == "RGBToYCbCr" {
		text.WriteString("x := (19595*a + 38470*b + 7471*c + 32768) >> 16\ny := (-11056*a - 21712*b + 32768*c + 8421376) >> 16\nz := (32768*a - 27440*b - 5328*c + 8421376) >> 16\n")
	} else {
		text.WriteString("yy := a * 65793\ncb, cr := b-128, c-128\nx := (yy + 91881*cr) >> 16\ny := (yy - 22554*cb - 46802*cr) >> 16\nz := (yy + 116130*cb) >> 16\n")
	}
	text.WriteString("if x < 0 { x = 0 }; if x > 255 { x = 255 }\nif y < 0 { y = 0 }; if y > 255 { y = 255 }\nif z < 0 { z = 0 }; if z > 255 { z = 255 }\n")
	text.WriteString(mutation)
	text.WriteString("return Result{Component0:x, Component1:y, Component2:z}\n}\n")
	return text.String()
}

func ycbcrFixture(source, operation string) map[string]any {
	contract, err := ycbcrOperationContract(operation)
	if err != nil {
		panic(err)
	}
	fields := []any{}
	inputs := []any{}
	for i, name := range contract.Fields {
		fields = append(fields, map[string]any{"name": name, "id": contract.Entity + "/" + name, "type_id": "urn:gooo:type:integer", "go_name": fmt.Sprintf("Component%d", i)})
		inputs = append(inputs, map[string]any{"port": fmt.Sprintf("input%d", i), "type": "Integer", "entity_id": integerEntity, "from": -1})
	}
	return map[string]any{"result": map[string]any{"composition": map[string]any{
		"stage": "COMPLETE", "source": source, "generated_sha256": contentDigest([]byte(source)),
		"plan": map[string]any{"schema": "gooo/body-composition-plan/v1", "activities": []any{map[string]any{
			"id": "gooo://go/image/color/" + operation, "go_function": "Convert",
			"input_entity_id": integerEntity, "output_entity_id": contract.Entity,
			"input_from": -1, "assembling": false, "inputs": inputs,
		}}, "edges": []any{}, "record_types": []any{map[string]any{"id": contract.Entity, "go_name": "Result", "fields": fields}}},
	}}}
}

func TestYCbCrGeneratedBinding(t *testing.T) {
	for _, operation := range []string{"RGBToYCbCr", "YCbCrToRGB"} {
		t.Run(operation, func(t *testing.T) {
			source := ycbcrFixtureSource(operation, "")
			digest := contentDigest([]byte(source))
			got, abi, err := inspectYCbCrGenerated(mustJSON(t, ycbcrFixture(source, operation)), digest, operation)
			if err != nil || got != source || abi.Function != "Convert" || abi.Record != "Result" || abi.Fields != [3]string{"Component0", "Component1", "Component2"} {
				t.Fatalf("valid generated ABI rejected: %+v %v", abi, err)
			}
		})
	}
	source := ycbcrFixtureSource("RGBToYCbCr", "")
	for name, mutate := range map[string]func(map[string]any, map[string]any, map[string]any, map[string]any){
		"digest":          func(c, p, a, r map[string]any) { c["generated_sha256"] = contentDigest([]byte("other")) },
		"changed source":  func(c, p, a, r map[string]any) { c["source"] = source + "\n" },
		"stage":           func(c, p, a, r map[string]any) { c["stage"] = "INCOMPLETE" },
		"plan schema":     func(c, p, a, r map[string]any) { p["schema"] = "other" },
		"no activity":     func(c, p, a, r map[string]any) { p["activities"] = []any{} },
		"extra activity":  func(c, p, a, r map[string]any) { p["activities"] = []any{a, a} },
		"dependency":      func(c, p, a, r map[string]any) { p["edges"] = []any{map[string]any{}} },
		"activity ID":     func(c, p, a, r map[string]any) { a["id"] = "gooo://go/image/color/YCbCrToRGB" },
		"output entity":   func(c, p, a, r map[string]any) { a["output_entity_id"] = "gooo://go/image/color/rgb" },
		"input entity":    func(c, p, a, r map[string]any) { a["input_entity_id"] = "gooo://std/boolean" },
		"input from":      func(c, p, a, r map[string]any) { a["input_from"] = 0 },
		"assembling":      func(c, p, a, r map[string]any) { a["assembling"] = true },
		"expression":      func(c, p, a, r map[string]any) { a["go_function"] = "color.RGBToYCbCr" },
		"reserved symbol": func(c, p, a, r map[string]any) { a["go_function"] = "TestYCbCrExhaustive" },
		"blank symbol":    func(c, p, a, r map[string]any) { a["go_function"] = "_" },
		"keyword":         func(c, p, a, r map[string]any) { a["go_function"] = "return" },
		"missing symbol":  func(c, p, a, r map[string]any) { a["go_function"] = "Missing" },
		"input count":     func(c, p, a, r map[string]any) { a["inputs"] = a["inputs"].([]any)[:2] },
		"input reorder": func(c, p, a, r map[string]any) {
			x := a["inputs"].([]any)
			x[0], x[1] = x[1], x[0]
		},
		"input type": func(c, p, a, r map[string]any) { a["inputs"].([]any)[2].(map[string]any)["type"] = "Boolean" },
		"input entity secondary": func(c, p, a, r map[string]any) {
			a["inputs"].([]any)[2].(map[string]any)["entity_id"] = "gooo://std/boolean"
		},
		"input dependency": func(c, p, a, r map[string]any) { a["inputs"].([]any)[2].(map[string]any)["from"] = 0 },
		"missing record":   func(c, p, a, r map[string]any) { p["record_types"] = []any{} },
		"duplicate record": func(c, p, a, r map[string]any) { p["record_types"] = []any{r, r} },
		"record symbol":    func(c, p, a, r map[string]any) { r["go_name"] = "Result.Other" },
		"record identity":  func(c, p, a, r map[string]any) { r["id"] = "gooo://go/image/color/rgb" },
		"field count":      func(c, p, a, r map[string]any) { r["fields"] = r["fields"].([]any)[:2] },
		"duplicate field":  func(c, p, a, r map[string]any) { r["fields"].([]any)[1] = r["fields"].([]any)[0] },
		"field type": func(c, p, a, r map[string]any) {
			r["fields"].([]any)[0].(map[string]any)["type_id"] = "urn:gooo:type:boolean"
		},
		"field identity":    func(c, p, a, r map[string]any) { r["fields"].([]any)[0].(map[string]any)["id"] = "other" },
		"field symbol swap": func(c, p, a, r map[string]any) { r["fields"].([]any)[0].(map[string]any)["go_name"] = "Component1" },
		"unknown field": func(c, p, a, r map[string]any) {
			f := r["fields"].([]any)[0].(map[string]any)
			f["name"], f["id"] = "other", "gooo://go/image/color/ycbcr/other"
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := ycbcrFixture(source, "RGBToYCbCr")
			c := fixture["result"].(map[string]any)["composition"].(map[string]any)
			p := c["plan"].(map[string]any)
			mutate(c, p, p["activities"].([]any)[0].(map[string]any), p["record_types"].([]any)[0].(map[string]any))
			if _, _, err := inspectYCbCrGenerated(mustJSON(t, fixture), contentDigest([]byte(source)), "RGBToYCbCr"); err == nil {
				t.Fatal("accepted altered source or entry binding")
			}
		})
	}
	for _, digest := range []string{"", "bad-digest", contentDigest([]byte("other"))} {
		if _, _, err := inspectYCbCrGenerated(mustJSON(t, ycbcrFixture(source, "RGBToYCbCr")), digest, "RGBToYCbCr"); err == nil {
			t.Fatal("accepted wrong checked.Generated")
		}
	}
	if _, _, err := inspectYCbCrGenerated(mustJSON(t, ycbcrFixture(source, "RGBToYCbCr")), contentDigest([]byte(source)), "RGBToCMYK"); err == nil {
		t.Fatal("accepted unsupported operation")
	}
}

func TestYCbCrRejectsUnexpectedGeneratedGo(t *testing.T) {
	base := ycbcrFixtureSource("RGBToYCbCr", "")
	for name, source := range map[string]string{
		"package":              strings.Replace(base, "package main", "package other", 1),
		"import":               strings.Replace(base, "package main", "package main\nimport \"image/color\"", 1),
		"main":                 base + "func main() {}",
		"init":                 base + "func init() {}",
		"helper":               base + "func Other() {}",
		"global":               base + "var hidden = 1",
		"constant":             base + "const hidden = 1",
		"shadow":               base + "type int64 = int32",
		"duplicate record":     base + "type Result struct {}",
		"duplicate field":      strings.Replace(base, "Component1", "Component0", 1),
		"non-primitive record": base + "type Extra struct { Callback func() }",
		"embedded field":       base + "type Extra struct { Result }",
		"input type":           strings.Replace(base, "a int64", "a int32", 1),
		"input arity":          strings.Replace(base, "a int64, ", "", 1),
		"grouped parameters":   strings.Replace(base, "a int64, b int64, c int64", "a, b, c int64", 1),
		"variadic":             strings.Replace(base, "c int64", "c ...int64", 1),
		"blank input":          strings.Replace(base, "a int64", "_ int64", 1),
		"generic":              strings.Replace(base, "Convert(", "Convert[T any](", 1),
		"wrong output type":    strings.Replace(base, ") Result {", ") Unrelated {", 1),
		"pointer output":       strings.Replace(base, ") Result {", ") *Result {", 1),
		"named output":         strings.Replace(base, ") Result {", ") (value Result) {", 1),
		"component type":       strings.Replace(base, "Component0 int64", "Component0 int32", 1),
		"component tag":        strings.Replace(base, "json:\"y\"", "json:\"cb\"", 1),
		"omitempty tag":        strings.Replace(base, "json:\"y\"", "json:\"y,omitempty\"", 1),
		"compiler directive":   strings.Replace(base, "func Convert", "//go:noinline\nfunc Convert", 1),
		"line directive":       strings.Replace(base, "func Convert", "//line other.go:1\nfunc Convert", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := inspectYCbCrGenerated(mustJSON(t, ycbcrFixture(source, "RGBToYCbCr")), contentDigest([]byte(source)), "RGBToYCbCr"); err == nil {
				t.Fatal("accepted unexpected generated Go")
			}
		})
	}
}

func TestYCbCrExhaustiveReportBinding(t *testing.T) {
	digest := contentDigest([]byte("checked source"))
	for _, operation := range []string{"RGBToYCbCr", "YCbCrToRGB"} {
		want, err := expectedYCbCrExhaustive(digest, operation)
		if err != nil {
			t.Fatal(err)
		}
		raw := mustJSON(t, want)
		if got, err := inspectYCbCrExhaustiveReport(raw, digest, operation); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("valid report rejected: %+v %v", got, err)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		for field := range fields {
			t.Run(operation+" missing "+field, func(t *testing.T) {
				var altered map[string]json.RawMessage
				_ = json.Unmarshal(raw, &altered)
				delete(altered, field)
				if _, err := inspectYCbCrExhaustiveReport(mustJSON(t, altered), digest, operation); err == nil {
					t.Fatal("accepted omitted report field")
				}
			})
			t.Run(operation+" altered "+field, func(t *testing.T) {
				var altered map[string]json.RawMessage
				_ = json.Unmarshal(raw, &altered)
				if fields[field][0] == '"' {
					altered[field] = json.RawMessage(`"changed"`)
				} else {
					altered[field] = json.RawMessage("-1")
				}
				if _, err := inspectYCbCrExhaustiveReport(mustJSON(t, altered), digest, operation); err == nil {
					t.Fatal("accepted altered report field")
				}
			})
		}
		for name, altered := range map[string][]byte{
			"duplicate zero": []byte(strings.Replace(string(raw), "{", "{\"mismatches\":0,", 1)),
			"null zero":      []byte(strings.Replace(string(raw), "\"mismatches\":0", "\"mismatches\":null", 1)),
			"extra pass":     []byte(strings.Replace(string(raw), "{", "{\"PASS\":true,", 1)),
			"trailing JSON":  append(append([]byte(nil), raw...), []byte(" {}")...),
			"null":           []byte("null"),
		} {
			t.Run(operation+" "+name, func(t *testing.T) {
				if _, err := inspectYCbCrExhaustiveReport(altered, digest, operation); err == nil {
					t.Fatal("accepted malformed exhaustive evidence")
				}
			})
		}
	}
}

func TestYCbCrNativeExhaustiveRunsUnchangedSource(t *testing.T) {
	// These must not escape into the isolated compiler or execution environment.
	t.Setenv("GOFLAGS", "-toolexec=/does-not-exist")
	t.Setenv("GOTOOLCHAIN", "not-a-toolchain")
	t.Setenv("GOOS", "invalid-target")
	t.Setenv("GOWORK", "/does-not-exist")
	t.Setenv("GOOO_YCBCR_REPORT", "/must-not-write-here")
	for _, operation := range []string{"RGBToYCbCr", "YCbCrToRGB"} {
		t.Run(operation, func(t *testing.T) {
			source := ycbcrFixtureSource(operation, "")
			digest := contentDigest([]byte(source))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dir := filepath.Join(t.TempDir(), "check")
			raw := mustJSON(t, ycbcrFixture(source, operation))
			got, err := checkYCbCrExhaustive(ctx, dir, raw, digest, operation)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := expectedYCbCrExhaustive(digest, operation)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected native report: %+v", got)
			}
			if actual, err := os.ReadFile(filepath.Join(dir, "native", "generated.go")); err != nil || string(actual) != source {
				t.Fatal("source was not preserved byte for byte", err)
			}
			if proof, err := os.ReadFile(filepath.Join(dir, "native-test.txt")); err != nil || len(proof) == 0 {
				t.Fatal("native process proof was not saved", err)
			}
			if _, err := checkYCbCrExhaustive(ctx, dir, raw, digest, operation); err == nil {
				t.Fatal("reused existing evidence directory")
			}
			if err := os.WriteFile(filepath.Join(dir, "native", "generated.go"), []byte(source+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := checkYCbCrSourceFile(filepath.Join(dir, "native", "generated.go"), digest); err == nil {
				t.Fatal("accepted altered on-disk source")
			}
		})
	}
}

func TestYCbCrNativeExhaustiveRejectsLastTupleMismatch(t *testing.T) {
	for _, operation := range []string{"RGBToYCbCr", "YCbCrToRGB"} {
		t.Run(operation, func(t *testing.T) {
			// A +256 error has identical narrowed output bytes. Exact int64
			// comparison must catch it even at the final tuple of the traversal.
			source := ycbcrFixtureSource(operation, "if a == 255 && b == 255 && c == 255 { z += 256 }\n")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dir := filepath.Join(t.TempDir(), "check")
			_, err := checkYCbCrExhaustive(ctx, dir, mustJSON(t, ycbcrFixture(source, operation)), contentDigest([]byte(source)), operation)
			if err == nil || !strings.Contains(err.Error(), operation+"(255,255,255)") {
				t.Fatal("failed to reject final tuple before narrowing output", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "exhaustive.json")); !os.IsNotExist(err) {
				t.Fatal("failed execution produced a success report", err)
			}
		})
	}
}
