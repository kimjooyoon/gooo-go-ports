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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"text/template"
	"unicode"
)

//go:embed simple_fold_exhaustive_test.go.txt
var simpleFoldExhaustiveTemplate string

type simpleFoldExhaustiveReport struct {
	Schema                           string  `json:"schema"`
	GoVersion                        string  `json:"go_version"`
	UnicodeVersion                   string  `json:"unicode_version"`
	GeneratedSHA256                  string  `json:"generated_sha256"`
	UnicodeRangeInputs               int     `json:"unicode_range_inputs"`
	UnicodeRangeComparisons          int     `json:"unicode_range_comparisons"`
	OutsideUnicodeInt32Inputs        []int64 `json:"outside_unicode_int32_inputs"`
	OutsideUnicodeInt32Comparisons   int     `json:"outside_unicode_int32_comparisons"`
	OutsideInt32ExtensionInputs      []int64 `json:"outside_int32_extension_inputs"`
	OutsideInt32ExtensionComparisons int     `json:"outside_int32_extension_comparisons"`
	Mismatches                       int     `json:"mismatches"`
}

func simpleFoldOutsideUnicodeInputs() []int64 {
	return []int64{-2147483648, -2147483647, -65536, -65461, -1, 0x110000, 0x11004b, 2147483646, 2147483647}
}

func simpleFoldExtensionInputs() []int64 {
	return []int64{-9223372036854775808, -9007199254740993, -4294967371, -4294967221, -2147483649,
		2147483648, 4294967296, 4294967371, 9007199254740993, 9223372036854775806, 9223372036854775807}
}

func expectedSimpleFoldExhaustive(generated string) simpleFoldExhaustiveReport {
	outside, extensions := simpleFoldOutsideUnicodeInputs(), simpleFoldExtensionInputs()
	return simpleFoldExhaustiveReport{
		Schema: "gooo/simple-fold-exhaustive-native/v1", GoVersion: "go1.27.2", UnicodeVersion: "17.0.0",
		GeneratedSHA256: generated, UnicodeRangeInputs: 1114112, UnicodeRangeComparisons: 1114112,
		OutsideUnicodeInt32Inputs: outside, OutsideUnicodeInt32Comparisons: len(outside),
		OutsideInt32ExtensionInputs: extensions, OutsideInt32ExtensionComparisons: len(extensions),
	}
}

// checkSimpleFoldExhaustive is an internal continuation of the finite verifier.
// Its caller MUST first validate execute and saved replay with inspect and
// inspectReplayBinding, and pass checked.Generated from that validation. This
// helper binds the single planned entry to its unchanged source; it deliberately
// does not duplicate the receipt/source-workspace/producer validator.
// The Unicode range is exhaustive, including surrogates. All other observations
// are finite samples. This does not check every signed rune or consumer pair.
func checkSimpleFoldExhaustive(ctx context.Context, dir string, raw []byte, expectedGenerated string, cache *nativeBuildCache) (simpleFoldExhaustiveReport, error) {
	var zero simpleFoldExhaustiveReport
	if runtime.Version() != "go1.27.2" || unicode.Version != "17.0.0" {
		return zero, fmt.Errorf("SimpleFold exhaustive check requires Go 1.27.2 / Unicode 17.0.0")
	}
	source, function, err := inspectSimpleFoldGenerated(raw, expectedGenerated)
	if err != nil {
		return zero, err
	}
	harness, err := makeSimpleFoldExhaustiveHarness(function, expectedGenerated)
	if err != nil {
		return zero, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return zero, err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return zero, fmt.Errorf("SimpleFold requires a fresh evidence directory: %w", err)
	}
	native := filepath.Join(dir, "native")
	cacheDir, err := cache.directory(filepath.Join(dir, "cache"))
	if err != nil {
		return zero, err
	}
	paths := []string{native, filepath.Join(dir, "tmp"), filepath.Join(dir, "gopath")}
	if cache == nil {
		paths = append(paths, cacheDir)
	}
	for _, path := range paths {
		if err := os.Mkdir(path, 0755); err != nil {
			return zero, err
		}
	}
	for name, content := range map[string][]byte{
		"go.mod":       []byte("module gooo-simple-fold-exhaustive\n\ngo 1.27.2\n"),
		"generated.go": []byte(source), "exhaustive_test.go": harness,
	} {
		if err := os.WriteFile(filepath.Join(native, name), content, 0644); err != nil {
			return zero, err
		}
	}
	generatedPath := filepath.Join(native, "generated.go")
	if err := checkSimpleFoldSourceFile(generatedPath, expectedGenerated); err != nil {
		return zero, err
	}
	reportPath := filepath.Join(dir, "exhaustive.json")
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-timeout=90s", "-run", "^TestSimpleFoldExhaustive$", "-v", ".")
	cmd.Dir, cmd.Env = native, simpleFoldGoEnvironment(dir, reportPath, cacheDir)
	output, runErr := cmd.CombinedOutput()
	if err := os.WriteFile(filepath.Join(dir, "native-test.txt"), output, 0644); err != nil {
		return zero, err
	}
	if err := checkSimpleFoldSourceFile(generatedPath, expectedGenerated); err != nil {
		return zero, err
	}
	if runErr != nil {
		return zero, fmt.Errorf("SimpleFold native exhaustive test: %w: %s", runErr, output)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return zero, fmt.Errorf("SimpleFold exhaustive report: %w", err)
	}
	return inspectSimpleFoldExhaustiveReport(report, expectedGenerated)
}

func checkSimpleFoldSourceFile(path, want string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if contentDigest(raw) != want {
		return fmt.Errorf("SimpleFold generated source changed on disk")
	}
	return nil
}

func inspectSimpleFoldGenerated(raw []byte, expectedGenerated string) (string, string, error) {
	var receipt struct {
		Result struct {
			Composition struct {
				Stage, Source string
				Generated     string `json:"generated_sha256"`
				Plan          struct {
					Schema     string
					Activities []struct {
						ID       string
						Function string `json:"go_function"`
						Input    string `json:"input_entity_id"`
						Output   string `json:"output_entity_id"`
					} `json:"activities"`
				} `json:"plan"`
			} `json:"composition"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return "", "", err
	}
	c := receipt.Result.Composition
	if !validDigest(expectedGenerated) || c.Stage != "COMPLETE" || c.Generated != expectedGenerated || c.Source == "" || contentDigest([]byte(c.Source)) != expectedGenerated {
		return "", "", fmt.Errorf("SimpleFold checked generated-source binding differs")
	}
	if c.Plan.Schema != "gooo/body-composition-plan/v1" || len(c.Plan.Activities) != 1 {
		return "", "", fmt.Errorf("SimpleFold requires exactly one planned activity")
	}
	a := c.Plan.Activities[0]
	if a.ID != "gooo://go/unicode/SimpleFold" || a.Input != integerEntity || a.Output != integerEntity || !token.IsIdentifier(a.Function) || a.Function == "_" || a.Function == "main" || a.Function == "init" {
		return "", "", fmt.Errorf("SimpleFold planned activity identity or ABI differs")
	}
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", c.Source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", "", err
	}
	if f.Name.Name != "main" || len(f.Imports) != 0 {
		return "", "", fmt.Errorf("SimpleFold generated source must be package main without imports")
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:") || strings.HasPrefix(comment.Text, "//line ") || strings.HasPrefix(comment.Text, "/*line ") {
				return "", "", fmt.Errorf("SimpleFold generated source contains compiler directives")
			}
		}
	}
	functions := 0
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			functions++
			if d.Name.Name != a.Function || d.Recv != nil || d.Body == nil || d.Type.TypeParams != nil || !simpleFoldInt64Slot(d.Type.Params, true) || !simpleFoldInt64Slot(d.Type.Results, false) {
				return "", "", fmt.Errorf("SimpleFold generated function must have the exact int64 -> int64 signature")
			}
		case *ast.GenDecl:
			// Pinned composition may emit unrelated record declarations. Accept
			// only inert structs: no imports, aliases, globals or initializers.
			if d.Tok != token.TYPE {
				return "", "", fmt.Errorf("SimpleFold generated source contains a non-record declaration")
			}
			for _, spec := range d.Specs {
				t, ok := spec.(*ast.TypeSpec)
				if !ok || t.Assign.IsValid() || t.TypeParams != nil || !ast.IsExported(t.Name.Name) {
					return "", "", fmt.Errorf("SimpleFold generated record declaration differs")
				}
				if _, ok := t.Type.(*ast.StructType); !ok {
					return "", "", fmt.Errorf("SimpleFold generated type is not a record")
				}
			}
		default:
			return "", "", fmt.Errorf("SimpleFold generated declaration differs")
		}
	}
	if functions != 1 {
		return "", "", fmt.Errorf("SimpleFold generated function roster differs")
	}
	return c.Source, a.Function, nil
}

func simpleFoldInt64Slot(fields *ast.FieldList, named bool) bool {
	if fields == nil || len(fields.List) != 1 {
		return false
	}
	f := fields.List[0]
	t, ok := f.Type.(*ast.Ident)
	return ok && t.Name == "int64" && len(f.Names) <= 1 && (!named || (len(f.Names) == 1 && f.Names[0].Name != "_"))
}

func makeSimpleFoldExhaustiveHarness(function, generated string) ([]byte, error) {
	t, err := template.New("simple-fold").Parse(simpleFoldExhaustiveTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, struct {
		Function, Generated string
		Outside, Extensions []int64
	}{function, generated, simpleFoldOutsideUnicodeInputs(), simpleFoldExtensionInputs()})
	return out.Bytes(), err
}

func simpleFoldGoEnvironment(dir, report, cacheDir string) []string {
	// Do not inherit GOFLAGS/-toolexec, GOENV configuration, alternate toolchain,
	// loader injection or any provider credentials from the caller's environment.
	return []string{
		"PATH=" + filepath.Join(runtime.GOROOT(), "bin") + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
		"GOROOT=" + runtime.GOROOT(), "GOTOOLCHAIN=local", "GOENV=off", "GOWORK=off", "GOFLAGS=",
		"GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off",
		"GOCACHE=" + cacheDir, "GOPATH=" + filepath.Join(dir, "gopath"),
		"GOTMPDIR=" + filepath.Join(dir, "tmp"), "TMPDIR=" + filepath.Join(dir, "tmp"),
		"GOOO_SIMPLE_FOLD_REPORT=" + report,
	}
}

func inspectSimpleFoldExhaustiveReport(raw []byte, generated string) (simpleFoldExhaustiveReport, error) {
	var got simpleFoldExhaustiveReport
	want := expectedSimpleFoldExhaustive(generated)
	wantRaw, _ := json.Marshal(want)
	var required map[string]json.RawMessage
	_ = json.Unmarshal(wantRaw, &required)
	// Require every field, including zero mismatches, exactly once. A zero-value
	// struct alone would incorrectly accept absent or null evidence counters.
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return got, fmt.Errorf("SimpleFold report must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return got, err
		}
		name, ok := key.(string)
		if !ok || required[name] == nil || seen[name] {
			return got, fmt.Errorf("SimpleFold report field roster differs")
		}
		seen[name] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return got, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return got, fmt.Errorf("SimpleFold report field is null")
		}
	}
	if _, err := d.Token(); err != nil {
		return got, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF || len(seen) != len(required) {
		return got, fmt.Errorf("SimpleFold report is incomplete or has trailing data")
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return got, err
	}
	if !validDigest(generated) || !reflect.DeepEqual(got, want) {
		return got, fmt.Errorf("SimpleFold report identity, input roster or counters differ")
	}
	return got, nil
}
