package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

//go:embed case_exhaustive_test.go.txt
var caseMappingExhaustiveTemplate string

type caseMappingExhaustiveReport struct {
	Schema                             string   `json:"schema"`
	GoVersion                          string   `json:"go_version"`
	UnicodeVersion                     string   `json:"unicode_version"`
	Operation                          string   `json:"operation"`
	ActivityID                         string   `json:"activity_id"`
	InputEntityIDs                     []string `json:"input_entity_ids"`
	OutputEntityID                     string   `json:"output_entity_id"`
	GeneratedSHA256                    string   `json:"generated_sha256"`
	ValidCases                         []int64  `json:"valid_cases"`
	UnicodeRangeInputs                 int      `json:"unicode_range_inputs"`
	UnicodeRangeComparisons            int      `json:"unicode_range_comparisons"`
	OutsideUnicodeInt32Inputs          []int64  `json:"outside_unicode_int32_inputs"`
	OutsideUnicodeInt32Comparisons     int      `json:"outside_unicode_int32_comparisons"`
	OutsideInt32ExtensionInputs        []int64  `json:"outside_int32_extension_inputs"`
	OutsideInt32ExtensionComparisons   int      `json:"outside_int32_extension_comparisons"`
	InvalidCases                       []int64  `json:"invalid_cases"`
	InvalidCaseOriginalRuneInputs      []int64  `json:"invalid_case_original_rune_inputs"`
	InvalidCaseOriginalRuneComparisons int      `json:"invalid_case_original_rune_comparisons"`
	InvalidCaseExtensionComparisons    int      `json:"invalid_case_extension_comparisons"`
	Mismatches                         int      `json:"mismatches"`
}

func caseMappingValidCases(operation string) ([]int64, error) {
	switch operation {
	case "To":
		return []int64{0, 1, 2}, nil
	case "ToUpper":
		return []int64{0}, nil
	case "ToLower":
		return []int64{1}, nil
	case "ToTitle":
		return []int64{2}, nil
	default:
		return nil, fmt.Errorf("unsupported case mapping operation %q", operation)
	}
}

func caseMappingInvalidCases() []int64 {
	return []int64{-9223372036854775808, -9007199254740993, -4294967296, -2147483649,
		-1, 3, 4, 2147483648, 4294967296, 9007199254740993, 9223372036854775807}
}

func caseMappingInvalidCaseRunes() []int64 {
	return []int64{-2147483648, -1, 0, 65, 97, 0x1c5, 0xd800, 0x10ffff, 0x110000, 2147483647}
}

func expectedCaseMappingExhaustive(generated, operation string) (caseMappingExhaustiveReport, error) {
	cases, err := caseMappingValidCases(operation)
	if err != nil {
		return caseMappingExhaustiveReport{}, err
	}
	// Use the same finite original-rune and extension boundary roster as the
	// independent SimpleFold check, without conflating its different semantics.
	outside, extensions := simpleFoldOutsideUnicodeInputs(), simpleFoldExtensionInputs()
	want := caseMappingExhaustiveReport{
		Schema: "gooo/case-mapping-exhaustive-native/v1", GoVersion: "go1.27.2", UnicodeVersion: "17.0.0",
		Operation: operation, ActivityID: "gooo://go/unicode/" + operation,
		InputEntityIDs: []string{integerEntity}, OutputEntityID: integerEntity, GeneratedSHA256: generated,
		ValidCases: cases, UnicodeRangeInputs: 1114112, UnicodeRangeComparisons: 1114112 * len(cases),
		OutsideUnicodeInt32Inputs: outside, OutsideUnicodeInt32Comparisons: len(outside) * len(cases),
		OutsideInt32ExtensionInputs: extensions, OutsideInt32ExtensionComparisons: len(extensions) * len(cases),
		InvalidCases: []int64{}, InvalidCaseOriginalRuneInputs: []int64{},
	}
	if operation == "To" {
		want.InputEntityIDs = []string{integerEntity, integerEntity}
		want.InvalidCases, want.InvalidCaseOriginalRuneInputs = caseMappingInvalidCases(), caseMappingInvalidCaseRunes()
		want.InvalidCaseOriginalRuneComparisons = len(want.InvalidCases) * len(want.InvalidCaseOriginalRuneInputs)
		want.InvalidCaseExtensionComparisons = len(want.InvalidCases) * len(extensions)
	}
	return want, nil
}

// checkCaseMappingExhaustive continues an already validated finite execution.
// The caller MUST validate execute/replay and workspace/source/producer bindings
// with inspect and inspectReplayBinding, and pass checked.Generated. Generated
// Go is copied byte-for-byte. Only 0..10FFFF is exhaustive; all other int32 rune
// observations, int64 extensions and invalid case values are finite samples.
func checkCaseMappingExhaustive(ctx context.Context, dir string, raw []byte, expectedGenerated, operation string, cache *nativeBuildCache) (caseMappingExhaustiveReport, error) {
	var zero caseMappingExhaustiveReport
	if runtime.Version() != "go1.27.2" || unicode.Version != "17.0.0" || strconv.IntSize != 64 {
		return zero, fmt.Errorf("case mapping exhaustive check requires 64-bit Go 1.27.2 / Unicode 17.0.0")
	}
	source, function, err := inspectCaseMappingGenerated(raw, expectedGenerated, operation)
	if err != nil {
		return zero, err
	}
	harness, err := makeCaseMappingExhaustiveHarness(function, expectedGenerated, operation)
	if err != nil {
		return zero, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return zero, err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return zero, fmt.Errorf("case mapping requires a fresh evidence directory: %w", err)
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
		"go.mod":       []byte("module gooo-case-mapping-exhaustive\n\ngo 1.27.2\n"),
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
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-timeout=90s", "-run", "^TestCaseMappingExhaustive$", "-v", ".")
	cmd.Dir, cmd.Env = native, caseMappingGoEnvironment(dir, reportPath, cacheDir)
	output, runErr := cmd.CombinedOutput()
	if err := os.WriteFile(filepath.Join(dir, "native-test.txt"), output, 0644); err != nil {
		return zero, err
	}
	if err := checkSimpleFoldSourceFile(generatedPath, expectedGenerated); err != nil {
		return zero, err
	}
	if runErr != nil {
		return zero, fmt.Errorf("%s native exhaustive test: %w: %s", operation, runErr, output)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return zero, fmt.Errorf("case mapping exhaustive report: %w", err)
	}
	return inspectCaseMappingExhaustiveReport(report, expectedGenerated, operation)
}

func makeCaseMappingExhaustiveHarness(function, generated, operation string) ([]byte, error) {
	want, err := expectedCaseMappingExhaustive(generated, operation)
	if err != nil {
		return nil, err
	}
	t, err := template.New("case-mapping").Parse(caseMappingExhaustiveTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, struct {
		caseMappingExhaustiveReport
		Function string
		Binary   bool
	}{want, function, operation == "To"})
	return out.Bytes(), err
}

func caseMappingGoEnvironment(dir, report, cacheDir string) []string {
	// Reuse the existing hermetic stdlib-only Go environment, with a distinct
	// report variable so this evidence cannot be confused with SimpleFold.
	env := []string{}
	for _, entry := range simpleFoldGoEnvironment(dir, "", cacheDir) {
		if !strings.HasPrefix(entry, "GOOO_SIMPLE_FOLD_REPORT=") {
			env = append(env, entry)
		}
	}
	return append(env, "GOOO_CASE_MAPPING_REPORT="+report)
}

func inspectCaseMappingExhaustiveReport(raw []byte, generated, operation string) (caseMappingExhaustiveReport, error) {
	var got caseMappingExhaustiveReport
	want, err := expectedCaseMappingExhaustive(generated, operation)
	if err != nil {
		return got, err
	}
	wantRaw, _ := json.Marshal(want)
	var required map[string]json.RawMessage
	_ = json.Unmarshal(wantRaw, &required)
	// Every field is required once, including empty rosters and zero counters.
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return got, fmt.Errorf("case mapping report must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return got, err
		}
		name, ok := key.(string)
		if !ok || required[name] == nil || seen[name] {
			return got, fmt.Errorf("case mapping report field roster differs")
		}
		seen[name] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return got, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return got, fmt.Errorf("case mapping report field is null")
		}
	}
	if _, err := d.Token(); err != nil {
		return got, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF || len(seen) != len(required) {
		return got, fmt.Errorf("case mapping report is incomplete or has trailing data")
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return got, err
	}
	if !validDigest(generated) || !reflect.DeepEqual(got, want) {
		return got, fmt.Errorf("case mapping report identity, input roster or counters differ")
	}
	return got, nil
}
