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
	"unicode"
)

func caseMappingFixtureSource(operation string) string {
	if operation == "To" {
		return "package main\ntype Record struct { X int64 }\nfunc Map(c int64, x int64) int64 { return x }\n"
	}
	cases, _ := caseMappingValidCases(operation)
	return fmt.Sprintf("package main\nfunc Map(x int64) int64 { return MapCall0(%d, x) }\nfunc MapCall0(c int64, x int64) int64 { return x }\n", cases[0])
}

func caseMappingFixture(source, operation string) map[string]any {
	a := map[string]any{
		"id": "gooo://go/unicode/" + operation, "go_function": "Map", "input_type": "Integer", "output_type": "Integer",
		"input_entity_id": integerEntity, "output_entity_id": integerEntity, "input_from": -1, "assembling": false,
	}
	if operation == "To" {
		a["inputs"] = []any{
			map[string]any{"port": "input0", "type": "Integer", "entity_id": integerEntity, "from": -1},
			map[string]any{"port": "input1", "type": "Integer", "entity_id": integerEntity, "from": -1},
		}
	}
	return map[string]any{"result": map[string]any{"composition": map[string]any{
		"stage": "COMPLETE", "source": source, "generated_sha256": contentDigest([]byte(source)),
		"plan": map[string]any{"schema": "gooo/body-composition-plan/v1", "activities": []any{a}, "edges": []any{}},
	}}}
}

func TestCaseMappingGeneratedBinding(t *testing.T) {
	for _, operation := range []string{"To", "ToUpper", "ToLower", "ToTitle"} {
		source := caseMappingFixtureSource(operation)
		digest := contentDigest([]byte(source))
		raw := mustJSON(t, caseMappingFixture(source, operation))
		if got, name, err := inspectCaseMappingGenerated(raw, digest, operation); err != nil || got != source || name != "Map" {
			t.Fatalf("valid %s source rejected: %s %v", operation, name, err)
		}
		for name, mutate := range map[string]func(map[string]any, map[string]any, map[string]any){
			"source digest":       func(c, p, a map[string]any) { c["generated_sha256"] = contentDigest([]byte("different")) },
			"source bytes":        func(c, p, a map[string]any) { c["source"] = source + "\n" },
			"stage":               func(c, p, a map[string]any) { c["stage"] = "INCOMPLETE" },
			"schema":              func(c, p, a map[string]any) { p["schema"] = "other" },
			"activities":          func(c, p, a map[string]any) { p["activities"] = []any{a, a} },
			"edges":               func(c, p, a map[string]any) { p["edges"] = []any{map[string]any{}} },
			"preparations":        func(c, p, a map[string]any) { p["preparations"] = []any{map[string]any{}} },
			"entry":               func(c, p, a map[string]any) { a["id"] = "gooo://go/unicode/SimpleFold" },
			"input entity":        func(c, p, a map[string]any) { a["input_entity_id"] = "other" },
			"output entity":       func(c, p, a map[string]any) { a["output_entity_id"] = "other" },
			"input type":          func(c, p, a map[string]any) { a["input_type"] = "Boolean" },
			"output type":         func(c, p, a map[string]any) { a["output_type"] = "Boolean" },
			"input from":          func(c, p, a map[string]any) { a["input_from"] = 0 },
			"assembly":            func(c, p, a map[string]any) { a["assembling"] = true },
			"function":            func(c, p, a map[string]any) { a["go_function"] = "Missing" },
			"function expression": func(c, p, a map[string]any) { a["go_function"] = "unicode.To" },
			"harness collision":   func(c, p, a map[string]any) { a["go_function"] = "TestCaseMappingExhaustive" },
			"input roster":        func(c, p, a map[string]any) { a["inputs"] = []any{map[string]any{}} },
		} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				fixture := caseMappingFixture(source, operation)
				c := fixture["result"].(map[string]any)["composition"].(map[string]any)
				p := c["plan"].(map[string]any)
				a := p["activities"].([]any)[0].(map[string]any)
				mutate(c, p, a)
				if _, _, err := inspectCaseMappingGenerated(mustJSON(t, fixture), digest, operation); err == nil {
					t.Fatal("accepted altered generated-source/entry binding")
				}
			})
		}
		for _, wrong := range []string{"", "bad", contentDigest([]byte("other validated source"))} {
			if _, _, err := inspectCaseMappingGenerated(raw, wrong, operation); err == nil {
				t.Fatal("accepted wrong checked.Generated")
			}
		}
	}
	for _, field := range []string{"port", "type", "entity_id", "from"} {
		fixture := caseMappingFixture(caseMappingFixtureSource("To"), "To")
		c := fixture["result"].(map[string]any)["composition"].(map[string]any)
		a := c["plan"].(map[string]any)["activities"].([]any)[0].(map[string]any)
		input := a["inputs"].([]any)[1].(map[string]any)
		if field == "from" {
			input[field] = 0
		} else {
			input[field] = "other"
		}
		if _, _, err := inspectCaseMappingGenerated(mustJSON(t, fixture), c["generated_sha256"].(string), "To"); err == nil {
			t.Fatal("accepted wrong second input", field)
		}
	}
}

func TestCaseMappingRejectsUnexpectedGeneratedGo(t *testing.T) {
	base := caseMappingFixtureSource("To")
	for name, source := range map[string]string{
		"wrong package": strings.Replace(base, "package main", "package other", 1),
		"oracle import": "package main\nimport \"unicode\"\nfunc Map(c int64,x int64) int64 { return int64(unicode.To(int(c),rune(x))) }",
		"main":          base + "func main() {}", "init": base + "func init() {}",
		"unexpected helper": base + "func MapCall0(c int64,x int64) int64 { return x }",
		"duplicate":         base + "func Map(c int64,x int64) int64 { return x }",
		"missing":           "package main\ntype Record struct {}",
		"global":            base + "var x = 1", "constant": base + "const x = 1",
		"alias": base + "type int64 = int32", "record alias": base + "type Other = Record",
		"non-inert record": base + "type Bad struct { X func() }",
		"duplicate record": base + "type Record struct {}",
		"method":           strings.Replace(base, "func Map", "func (r Record) Map", 1),
		"int32 input":      strings.Replace(base, "c int64", "c int32", 1),
		"int32 result":     strings.Replace(base, ") int64", ") int32", 1),
		"extra input":      strings.Replace(base, "c int64, x int64", "c int64, x int64, y int64", 1),
		"extra result":     strings.Replace(base, ") int64", ") (int64,int64)", 1),
		"variadic":         strings.Replace(base, "x int64", "x ...int64", 1),
		"generic":          strings.Replace(base, "Map(", "Map[T any](", 1),
		"directive":        strings.Replace(base, "func Map", "//go:linkname Map syscall.x\nfunc Map", 1),
		"recursive":        strings.Replace(base, "return x", "return Map(c,x)", 1),
		"harness call":     strings.Replace(base, "return x", "TestCaseMappingExhaustive(nil); return x", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := inspectCaseMappingGenerated(mustJSON(t, caseMappingFixture(source, "To")), contentDigest([]byte(source)), "To"); err == nil {
				t.Fatal("accepted unexpected generated Go")
			}
		})
	}
	for name, source := range map[string]string{
		"wrong fixed case": strings.Replace(caseMappingFixtureSource("ToUpper"), "MapCall0(0, x)", "MapCall0(1, x)", 1),
		"changed input":    strings.Replace(caseMappingFixtureSource("ToUpper"), "MapCall0(0, x)", "MapCall0(0, x+1)", 1),
		"wrong helper ABI": strings.Replace(caseMappingFixtureSource("ToUpper"), "c int64", "c int32", 1),
		"renamed helper":   strings.ReplaceAll(caseMappingFixtureSource("ToUpper"), "MapCall0", "MapCall1"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := inspectCaseMappingGenerated(mustJSON(t, caseMappingFixture(source, "ToUpper")), contentDigest([]byte(source)), "ToUpper"); err == nil {
				t.Fatal("accepted wrong wrapper/helper binding")
			}
		})
	}
}

func TestCaseMappingExhaustiveReportBinding(t *testing.T) {
	digest := contentDigest([]byte("bound source"))
	for _, operation := range []string{"To", "ToUpper", "ToLower", "ToTitle"} {
		want, err := expectedCaseMappingExhaustive(digest, operation)
		if err != nil {
			t.Fatal(err)
		}
		raw := mustJSON(t, want)
		if got, err := inspectCaseMappingExhaustiveReport(raw, digest, operation); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("valid %s report rejected: %+v %v", operation, got, err)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		for field := range fields {
			for _, mutation := range []string{"missing", "null", "wrong value"} {
				t.Run(operation+"/"+field+"/"+mutation, func(t *testing.T) {
					var altered map[string]json.RawMessage
					_ = json.Unmarshal(raw, &altered)
					switch mutation {
					case "missing":
						delete(altered, field)
					case "null":
						altered[field] = json.RawMessage("null")
					default:
						var value any
						_ = json.Unmarshal(altered[field], &value)
						switch v := value.(type) {
						case string:
							value = v + "other"
						case float64:
							value = v + 1
						case []any:
							value = append(v, float64(123))
						default:
							t.Fatalf("unhandled field %s", field)
						}
						altered[field] = mustJSON(t, value)
					}
					if _, err := inspectCaseMappingExhaustiveReport(mustJSON(t, altered), digest, operation); err == nil {
						t.Fatal("accepted tampered exhaustive evidence")
					}
				})
			}
		}
		for name, altered := range map[string][]byte{
			"duplicate": []byte(strings.Replace(string(raw), "{", "{\"mismatches\":0,", 1)),
			"unknown":   []byte(strings.Replace(string(raw), "{", "{\"unverified\":true,", 1)),
			"trailing":  append(append([]byte(nil), raw...), []byte(" {}")...), "null": []byte("null"),
		} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				if _, err := inspectCaseMappingExhaustiveReport(altered, digest, operation); err == nil {
					t.Fatal("accepted malformed report")
				}
			})
		}
		if _, err := inspectCaseMappingExhaustiveReport(raw, contentDigest([]byte("other")), operation); err == nil {
			t.Fatal("accepted unrelated generated source")
		}
	}
	if _, err := expectedCaseMappingExhaustive(digest, "SimpleFold"); err == nil {
		t.Fatal("accepted unknown operation")
	}
}

func caseMappingSyntheticSource(operation, extra, fallback, invalidPrefix string) string {
	// Checker fixture, not port code: materialize pinned oracle answers so each
	// real subprocess exercises the entire unchanged-source harness independently.
	var s strings.Builder
	s.WriteString("package main\n")
	name := "Map"
	if operation != "To" {
		cases, _ := caseMappingValidCases(operation)
		fmt.Fprintf(&s, "func Map(x int64) int64 { return MapCall0(%d,x) }\n", cases[0])
		name = "MapCall0"
	}
	fmt.Fprintf(&s, "func %s(c int64,x int64) int64 { %s; if c<0 || c>2 { return 65533 }; %s; switch c {\n", name, invalidPrefix, extra)
	for c := 0; c < 3; c++ {
		fmt.Fprintf(&s, "case %d: switch x {\n", c)
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if next := unicode.To(c, r); next != r {
				fmt.Fprintf(&s, "case %d: return %d\n", r, next)
			}
		}
		s.WriteString("}\n")
	}
	s.WriteString("}; return " + fallback + " }\n")
	return s.String()
}

func TestCaseMappingNativeExhaustiveRunsUnchangedSource(t *testing.T) {
	cache, err := newNativeBuildCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cache.close(); err != nil {
			t.Error(err)
		}
	})
	// Host poisoning must not enter the isolated, stdlib-only verifier module.
	t.Setenv("GOFLAGS", "-toolexec=/does-not-exist")
	t.Setenv("GOTOOLCHAIN", "not-a-toolchain")
	t.Setenv("GOOS", "invalid-target")
	t.Setenv("GOWORK", "/does-not-exist")
	t.Setenv("GOCACHE", "/must-not-use-caller-cache")
	t.Setenv("GOCACHEPROG", "/must-not-use-caller-cache-program")
	t.Setenv("GOOO_CASE_MAPPING_REPORT", "/must-not-write-here")
	for _, operation := range []string{"To", "ToUpper", "ToLower", "ToTitle"} {
		t.Run(operation, func(t *testing.T) {
			source := caseMappingSyntheticSource(operation, "", "x", "")
			digest := contentDigest([]byte(source))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dir := filepath.Join(t.TempDir(), "check")
			raw := mustJSON(t, caseMappingFixture(source, operation))
			got, err := checkCaseMappingExhaustive(ctx, dir, raw, digest, operation, cache)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := expectedCaseMappingExhaustive(digest, operation)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected native report: %+v", got)
			}
			for _, name := range []string{"native-test.txt", "exhaustive.json"} {
				if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || len(b) == 0 {
					t.Fatal("missing native evidence", name, err)
				}
			}
			path := filepath.Join(dir, "native", "generated.go")
			if b, err := os.ReadFile(path); err != nil || string(b) != source {
				t.Fatal("source not preserved byte-for-byte", err)
			}
			if _, err := checkCaseMappingExhaustive(ctx, dir, raw, digest, operation, cache); err == nil {
				t.Fatal("reused existing evidence directory")
			}
			if err := os.WriteFile(path, []byte(source+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := checkSimpleFoldSourceFile(path, digest); err == nil {
				t.Fatal("accepted changed on-disk source")
			}
		})
	}
}

func TestCaseMappingNativeExhaustiveRejectsSemanticTampering(t *testing.T) {
	for _, test := range []struct{ name, extra, fallback, prefix, message string }{
		{"last title comparison", "if c==2 && x==1114111 { return 0 }", "x", "", "case=2 rune=1114111"},
		{"narrowed extension", "", "int64(int32(x))", "", "int64 extension To case=0 rune=-9223372036854775808"},
		{"narrowed case", "", "x", "c = int64(int32(c))", "invalid case original rune To case=-9223372036854775808"},
		{"invalid case precedence", "", "x", "if x < -2147483648 || x > 2147483647 { return x }", "invalid case int64 extension To case=-9223372036854775808"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := caseMappingSyntheticSource("To", test.extra, test.fallback, test.prefix)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dir := filepath.Join(t.TempDir(), "check")
			_, err := checkCaseMappingExhaustive(ctx, dir, mustJSON(t, caseMappingFixture(source, "To")), contentDigest([]byte(source)), "To", nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatal("did not reject semantic tampering at expected comparison", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "exhaustive.json")); !os.IsNotExist(err) {
				t.Fatal("failed run wrote success report", err)
			}
			if b, err := os.ReadFile(filepath.Join(dir, "native-test.txt")); err != nil || !strings.Contains(string(b), test.message) {
				t.Fatal("failure stdout not preserved", err)
			}
		})
	}
}
