package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
)

const simpleFoldFixtureSource = "package main\ntype Record struct { X int64 }\nfunc Fold(x int64) int64 { return x }\n"

func simpleFoldFixture(source string) map[string]any {
	return map[string]any{"result": map[string]any{"composition": map[string]any{
		"stage": "COMPLETE", "source": source, "generated_sha256": contentDigest([]byte(source)),
		"plan": map[string]any{"schema": "gooo/body-composition-plan/v1", "activities": []any{map[string]any{
			"id": "gooo://go/unicode/SimpleFold", "go_function": "Fold",
			"input_entity_id": integerEntity, "output_entity_id": integerEntity,
		}}},
	}}}
}

func TestSimpleFoldGeneratedBinding(t *testing.T) {
	want := contentDigest([]byte(simpleFoldFixtureSource))
	raw := mustJSON(t, simpleFoldFixture(simpleFoldFixtureSource))
	if source, name, err := inspectSimpleFoldGenerated(raw, want); err != nil || source != simpleFoldFixtureSource || name != "Fold" {
		t.Fatalf("valid single entry rejected: %q %q %v", source, name, err)
	}
	for name, mutate := range map[string]func(map[string]any, map[string]any, map[string]any){
		"wrong source digest":   func(c, p, a map[string]any) { c["generated_sha256"] = contentDigest([]byte("other")) },
		"changed source":        func(c, p, a map[string]any) { c["source"] = simpleFoldFixtureSource + "\n" },
		"wrong stage":           func(c, p, a map[string]any) { c["stage"] = "INCOMPLETE" },
		"wrong plan schema":     func(c, p, a map[string]any) { p["schema"] = "other" },
		"empty activity roster": func(c, p, a map[string]any) { p["activities"] = []any{} },
		"extra activity":        func(c, p, a map[string]any) { p["activities"] = []any{a, a} },
		"wrong activity ID":     func(c, p, a map[string]any) { a["id"] = "gooo://go/unicode/Other" },
		"wrong input entity":    func(c, p, a map[string]any) { a["input_entity_id"] = "gooo://std/boolean" },
		"wrong output entity":   func(c, p, a map[string]any) { a["output_entity_id"] = "gooo://std/boolean" },
		"missing function":      func(c, p, a map[string]any) { a["go_function"] = "Missing" },
		"function expression":   func(c, p, a map[string]any) { a["go_function"] = "unicode.SimpleFold" },
		"blank function":        func(c, p, a map[string]any) { a["go_function"] = "_" },
		"keyword function":      func(c, p, a map[string]any) { a["go_function"] = "return" },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := simpleFoldFixture(simpleFoldFixtureSource)
			c := fixture["result"].(map[string]any)["composition"].(map[string]any)
			p := c["plan"].(map[string]any)
			a := p["activities"].([]any)[0].(map[string]any)
			mutate(c, p, a)
			if _, _, err := inspectSimpleFoldGenerated(mustJSON(t, fixture), want); err == nil {
				t.Fatal("accepted altered source or entry binding")
			}
		})
	}
	for _, digest := range []string{"", "not-a-hash", contentDigest([]byte("different validated program"))} {
		if _, _, err := inspectSimpleFoldGenerated(raw, digest); err == nil {
			t.Fatal("accepted wrong checked.Generated")
		}
	}
}

func TestSimpleFoldRejectsUnexpectedGeneratedGo(t *testing.T) {
	for name, source := range map[string]string{
		"wrong package":      "package other\nfunc Fold(x int64) int64 { return x }",
		"import":             "package main\nimport \"unicode\"\nfunc Fold(x int64) int64 { return int64(unicode.SimpleFold(rune(x))) }",
		"main":               simpleFoldFixtureSource + "func main() {}",
		"initializer":        simpleFoldFixtureSource + "func init() {}",
		"extra helper":       simpleFoldFixtureSource + "func Other(x int64) int64 { return x }",
		"duplicate function": simpleFoldFixtureSource + "func Fold(x int64) int64 { return x }",
		"missing function":   "package main\ntype Record struct { X int64 }",
		"global variable":    simpleFoldFixtureSource + "var state = 1",
		"global constant":    simpleFoldFixtureSource + "const state = 1",
		"shadow int64":       "package main\ntype int64 = int32\nfunc Fold(x int64) int64 { return x }",
		"method":             "package main\ntype Record struct {}\nfunc (r Record) Fold(x int64) int64 { return x }",
		"int32 input":        "package main\nfunc Fold(x int32) int64 { return int64(x) }",
		"int32 output":       "package main\nfunc Fold(x int64) int32 { return int32(x) }",
		"extra input":        "package main\nfunc Fold(x, y int64) int64 { return x }",
		"extra output":       "package main\nfunc Fold(x int64) (int64, int64) { return x, x }",
		"variadic":           "package main\nfunc Fold(x ...int64) int64 { return x[0] }",
		"generic":            "package main\nfunc Fold[T any](x int64) int64 { return x }",
		"no body":            "package main\nfunc Fold(x int64) int64",
		"compiler directive": "package main\n//go:linkname Fold syscall.somewhere\nfunc Fold(x int64) int64 { return x }",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := inspectSimpleFoldGenerated(mustJSON(t, simpleFoldFixture(source)), contentDigest([]byte(source))); err == nil {
				t.Fatal("accepted unexpected generated Go")
			}
		})
	}
}

func TestSimpleFoldExhaustiveReportBinding(t *testing.T) {
	digest := contentDigest([]byte(simpleFoldFixtureSource))
	want := expectedSimpleFoldExhaustive(digest)
	raw := mustJSON(t, want)
	if got, err := inspectSimpleFoldExhaustiveReport(raw, digest); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("valid report rejected: %+v %v", got, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for field := range fields {
		t.Run("missing "+field, func(t *testing.T) {
			var altered map[string]json.RawMessage
			_ = json.Unmarshal(raw, &altered)
			delete(altered, field)
			if _, err := inspectSimpleFoldExhaustiveReport(mustJSON(t, altered), digest); err == nil {
				t.Fatal("accepted omitted report field")
			}
		})
	}
	for name, mutate := range map[string]func(*simpleFoldExhaustiveReport){
		"schema":                   func(r *simpleFoldExhaustiveReport) { r.Schema = "other" },
		"toolchain":                func(r *simpleFoldExhaustiveReport) { r.GoVersion = "go1.27.1" },
		"Unicode":                  func(r *simpleFoldExhaustiveReport) { r.UnicodeVersion = "16.0.0" },
		"source":                   func(r *simpleFoldExhaustiveReport) { r.GeneratedSHA256 = contentDigest([]byte("other")) },
		"range coverage":           func(r *simpleFoldExhaustiveReport) { r.UnicodeRangeInputs-- },
		"comparison coverage":      func(r *simpleFoldExhaustiveReport) { r.UnicodeRangeComparisons-- },
		"outside reference roster": func(r *simpleFoldExhaustiveReport) { r.OutsideUnicodeInt32Inputs[0] = 0 },
		"extension roster narrowed": func(r *simpleFoldExhaustiveReport) {
			r.OutsideInt32ExtensionInputs[2] = int64(int32(r.OutsideInt32ExtensionInputs[2]))
		},
		"outside counter":   func(r *simpleFoldExhaustiveReport) { r.OutsideUnicodeInt32Comparisons++ },
		"extension counter": func(r *simpleFoldExhaustiveReport) { r.OutsideInt32ExtensionComparisons++ },
		"mismatch":          func(r *simpleFoldExhaustiveReport) { r.Mismatches = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			altered := expectedSimpleFoldExhaustive(digest)
			mutate(&altered)
			if _, err := inspectSimpleFoldExhaustiveReport(mustJSON(t, altered), digest); err == nil {
				t.Fatal("accepted altered exhaustive evidence")
			}
		})
	}
	for name, altered := range map[string][]byte{
		"duplicate zero": []byte(strings.Replace(string(raw), "{", "{\"mismatches\":0,", 1)),
		"null zero":      []byte(strings.Replace(string(raw), "\"mismatches\":0", "\"mismatches\":null", 1)),
		"extra field":    []byte(strings.Replace(string(raw), "{", "{\"unverified\":true,", 1)),
		"trailing JSON":  append(append([]byte(nil), raw...), []byte(" {}")...),
		"null report":    []byte("null"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := inspectSimpleFoldExhaustiveReport(altered, digest); err == nil {
				t.Fatal("accepted malformed exhaustive evidence")
			}
		})
	}
}

func simpleFoldSyntheticSource(extraCase, fallback string) string {
	// Synthetic checker fixture only, not a port implementation: materialize
	// known answers to exercise the subprocess and the entire harness loop.
	var source strings.Builder
	source.WriteString("package main\nfunc Fold(x int64) int64 { switch x {\n")
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if next := unicode.SimpleFold(r); next != r {
			fmt.Fprintf(&source, "case %d: return %d\n", r, next)
		}
	}
	source.WriteString(extraCase)
	source.WriteString("}; return " + fallback + " }\n")
	return source.String()
}

func TestSimpleFoldNativeExhaustiveRunsUnchangedSource(t *testing.T) {
	if runtime.Version() != "go1.27.2" || unicode.Version != "17.0.0" {
		t.Fatal("test requires the pinned toolchain")
	}
	text := simpleFoldSyntheticSource("", "x")
	digest := contentDigest([]byte(text))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// These would break compilation or poison execution if inherited.
	t.Setenv("GOFLAGS", "-toolexec=/does-not-exist")
	t.Setenv("GOTOOLCHAIN", "not-a-toolchain")
	t.Setenv("GOOS", "invalid-target")
	t.Setenv("GOWORK", "/does-not-exist")
	t.Setenv("GOOO_SIMPLE_FOLD_REPORT", "/must-not-write-here")
	dir := filepath.Join(t.TempDir(), "check")
	got, err := checkSimpleFoldExhaustive(ctx, dir, mustJSON(t, simpleFoldFixture(text)), digest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expectedSimpleFoldExhaustive(digest)) {
		t.Fatalf("unexpected native report: %+v", got)
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "native", "generated.go")); err != nil || string(raw) != text {
		t.Fatal("source was not copied byte for byte", err)
	}
	if _, err := checkSimpleFoldExhaustive(ctx, dir, mustJSON(t, simpleFoldFixture(text)), digest, nil); err == nil {
		t.Fatal("reused existing evidence directory")
	}
	if err := os.WriteFile(filepath.Join(dir, "native", "generated.go"), []byte(text+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkSimpleFoldSourceFile(filepath.Join(dir, "native", "generated.go"), digest); err == nil {
		t.Fatal("accepted changed on-disk composition")
	}
}

func TestSimpleFoldNativeExhaustiveRejectsLateMismatch(t *testing.T) {
	// A zero mismatch counter or PASS text cannot substitute for execution.
	// Reach the last Unicode-range input before failing the independent oracle.
	text := simpleFoldSyntheticSource("case 1114111: return 0\n", "x")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := filepath.Join(t.TempDir(), "check")
	if _, err := checkSimpleFoldExhaustive(ctx, dir, mustJSON(t, simpleFoldFixture(text)), contentDigest([]byte(text)), nil); err == nil || !strings.Contains(err.Error(), "SimpleFold(1114111)") {
		t.Fatal("did not reject final-range mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "exhaustive.json")); !os.IsNotExist(err) {
		t.Fatal("failed run produced a successful report", err)
	}
}

func TestSimpleFoldNativeExhaustiveRejectsNarrowedExtension(t *testing.T) {
	text := simpleFoldSyntheticSource("", "int64(int32(x))")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := filepath.Join(t.TempDir(), "check")
	if _, err := checkSimpleFoldExhaustive(ctx, dir, mustJSON(t, simpleFoldFixture(text)), contentDigest([]byte(text)), nil); err == nil || !strings.Contains(err.Error(), "int64 extension SimpleFold(-9223372036854775808)") {
		t.Fatal("did not reject narrowed extension after all original rune checks", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "exhaustive.json")); !os.IsNotExist(err) {
		t.Fatal("failed extension produced a successful report", err)
	}
}
