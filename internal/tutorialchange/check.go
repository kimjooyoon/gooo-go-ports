// Package tutorialchange checks the finite learn-gooo threshold-edit profile.
// It reads observations; it neither executes Gooo nor authenticates their origin.
package tutorialchange

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const (
	MaxReceiptBytes       = 256 << 10
	MaxCompilerBuildBytes = 16 << 10
	compilerRevision      = "83928fa4538245abe33b08549da47f8cbdc50ed2"
)

// These are public, synthetic, retained observations, not a newly executed test.
// Only the explicitly listed process/accounting coordinates vary at runtime.
//
//go:embed testdata/before.json testdata/after.json testdata/replay.json testdata/compiler-build.json testdata/main-before.gooo testdata/main-after.gooo testdata/limits.gooo testdata/workspace.json testdata/inputs.json
var profileFiles embed.FS

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var platformPattern = regexp.MustCompile(`^(darwin|linux)/(amd64|arm64)$`)

// Check verifies three original CLI records and separately observed compiler
// BuildInfo for exactly the four-input, threshold-60-to-80 tutorial. It does not
// accept an arbitrary program, compiler revision, input corpus or expected result.
func Check(before, after, replay, compilerBuild []byte) (Summary, error) {
	var out Summary
	profile, err := selectEvidenceProfile(compilerBuild)
	if err != nil {
		return out, err
	}
	raws := [][]byte{before, after, replay}
	labels := []string{"before", "after", "replay"}
	records := make([]map[string]any, 3)
	for i, raw := range raws {
		v, err := decode(raw)
		if err != nil {
			return out, fmt.Errorf("%s: %w", labels[i], err)
		}
		records[i] = object(v)
		ref, err := profile.record(labels[i] + ".json")
		if err != nil {
			return out, err
		}
		want, err := decode(ref)
		if err != nil {
			return out, fmt.Errorf("embedded tutorial profile: %w", err)
		}
		version := stringAt(records[i], "result", "runtime", "go_version")
		platform := strings.TrimPrefix(version, "go version "+profile.goVersion+" ")
		if version != "go version "+profile.goVersion+" "+platform || !platformPattern.MatchString(platform) {
			return out, fmt.Errorf("%s: unsupported Go tool version/platform", labels[i])
		}
		if err := matchProfile(v, want, "$", platform, profile.goVersion, 0); err != nil {
			return out, fmt.Errorf("%s: %w", labels[i], err)
		}
		if err := checkBindings(raw, records[i]); err != nil {
			return out, fmt.Errorf("%s: %w", labels[i], err)
		}
	}
	if len(compilerBuild) == 0 || len(compilerBuild) > MaxCompilerBuildBytes {
		return out, fmt.Errorf("compiler-build: size outside finite profile")
	}
	build, err := decode(compilerBuild)
	if err != nil {
		return out, fmt.Errorf("compiler-build: %w", err)
	}
	buildRef, err := profile.record("compiler-build.json")
	if err != nil {
		return out, err
	}
	wantBuild, err := decode(buildRef)
	if err != nil || !reflect.DeepEqual(build, wantBuild) {
		return out, fmt.Errorf("compiler-build: exact clean pinned compiler BuildInfo differs")
	}
	for i := 1; i < 3; i++ {
		for _, key := range []string{"go_version", "go_tool_sha256", "producer_source_sha"} {
			if stringAt(records[i], "result", "runtime", key) != stringAt(records[0], "result", "runtime", key) {
				return out, fmt.Errorf("%s: actual tool/compiler identity differs", labels[i])
			}
		}
	}
	if err := checkSourceFiles(records); err != nil {
		return out, err
	}
	if records[2]["replayed_from_sha256"] != digest(after) {
		return out, fmt.Errorf("replay: raw parent does not identify the edited execution")
	}
	priorResult, err := pinnedResultDigestForProfile(after, profile.resultProfile, profile.compiler)
	if err != nil {
		return out, fmt.Errorf("edited typed Result: %w", err)
	}
	if stringAt(records[2], "result", "replay", "prior_result_sha256") != priorResult {
		return out, fmt.Errorf("replay: pinned typed Result parent differs")
	}
	summary := summarize(raws, compilerBuild, records, object(build), priorResult)
	summary.Profile = profile.name
	summary.Limits[1] = "Exactly the selected embedded " + profile.name + " tutorial profile, four original inputs and the sole threshold60-to80 edit; not a general receipt validator or all-input correctness proof."
	return summary, nil
}

func decode(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > MaxReceiptBytes || !utf8.Valid(raw) {
		return nil, fmt.Errorf("JSON size or raw UTF-8 outside finite profile")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := pinnedJSONDepth(d, 0); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	if err := jsonkeys.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func digest(raw []byte) string    { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func stringAt(v map[string]any, keys ...string) string {
	var x any = v
	for _, key := range keys {
		x = object(x)[key]
	}
	s, _ := x.(string)
	return s
}

// Strict structure/static-value comparison also covers the codec's deliberately
// opaque RawMessage and generic-map fields. An added or missing key is rejected
// at every level, including provider, trace, field, ABI and source-image metadata.
func matchProfile(v, want any, path, platform, goVersion string, depth int) error {
	if depth > 64 {
		return fmt.Errorf("profile depth exceeded")
	}
	if handled, ok := observedLeaf(v, path, platform, goVersion); handled {
		if !ok {
			return fmt.Errorf("invalid observed value at %s", path)
		}
		return nil
	}
	switch w := want.(type) {
	case map[string]any:
		m, ok := v.(map[string]any)
		if !ok || len(m) != len(w) {
			return fmt.Errorf("unsupported object shape at %s", path)
		}
		for key, expected := range w {
			value, exists := m[key]
			if !exists {
				return fmt.Errorf("missing field at %s.%s", path, key)
			}
			if err := matchProfile(value, expected, path+"."+key, platform, goVersion, depth+1); err != nil {
				return err
			}
		}
	case []any:
		a, ok := v.([]any)
		if !ok || len(a) != len(w) {
			return fmt.Errorf("unsupported array roster at %s", path)
		}
		for i := range w {
			if err := matchProfile(a[i], w[i], path+".*", platform, goVersion, depth+1); err != nil {
				return err
			}
		}
	default:
		if !reflect.DeepEqual(v, want) {
			return fmt.Errorf("tutorial value/type differs at %s", path)
		}
	}
	return nil
}

func observedLeaf(v any, path, platform, goVersion string) (bool, bool) {
	if path == "$.result.runtime.go_version" {
		return true, v == "go version "+goVersion+" "+platform
	}
	if path == "$.result.runtime.go_tool_selection" {
		// These are the pin839 selectGoTool source tags. The selection route
		// may differ while the actual Go version and binary hash remain equal.
		return true, v == "explicit" || v == "path" || v == "compiler_goroot" ||
			v == "toolchain_cache" || v == "path_fallback"
	}
	if path == "$.result.composition.steps.*.generation.report.completeness_receipt.scope.execution_environment" {
		return true, v == platform
	}
	if path == "$.replayed_from_sha256" || path == "$.result.replay.prior_result_sha256" ||
		path == "$.result.runtime.go_tool_sha256" || path == "$.result.runtime.executable_sha256" ||
		path == "$.result.runtime.composition_sha256" {
		s, ok := v.(string)
		return true, ok && digestPattern.MatchString(s)
	}
	if path == "$.result.runtime.elapsed_ns" || path == "$.result.composition.elapsed_ns" {
		return true, nonnegativeInteger(v)
	}
	for _, prefix := range []string{"$.result.runtime.toolchain.", "$.result.runtime.build.", "$.result.runtime.runs.*."} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		key := strings.TrimPrefix(path, prefix)
		// The version command has platform-specific output. This finite
		// tutorial's build and both driver stdout hashes remain source/input
		// bound static profile values, rather than arbitrary matching digests.
		if prefix == "$.result.runtime.toolchain." && key == "stdout_sha256" {
			s, ok := v.(string)
			return true, ok && digestPattern.MatchString(s)
		}
		if key == "wall_ns" || key == "user_ns" || key == "system_ns" || key == "peak_rss_bytes" ||
			strings.HasPrefix(key, "timing.") && key != "timing.context_at_start_return" {
			return true, nonnegativeInteger(v)
		}
	}
	return false, false
}

func nonnegativeInteger(v any) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	x, err := n.Int64()
	return err == nil && x >= 0 && n.String() == strconv.FormatInt(x, 10)
}

func checkBindings(raw []byte, e map[string]any) error {
	r := object(e["result"])
	c := object(r["composition"])
	rt := object(r["runtime"])
	p := object(r["program"])
	source := stringAt(p, "lowered_gooo_source")
	sd := digest([]byte(source))
	if r["execution_source_sha256"] != sd || c["gooo_source"] != source || c["original_source_sha256"] != sd ||
		c["selected_source_sha256"] != sd || rt["original_source_sha256"] != sd || rt["selected_source_sha256"] != sd {
		return fmt.Errorf("lowered source binding differs")
	}
	for _, pair := range [][2]string{{"source", "generated_sha256"}, {"driver_source", "driver_sha256"}} {
		text, _ := c[pair[0]].(string)
		want := digest([]byte(text))
		if c[pair[1]] != want || rt[pair[1]] != want {
			return fmt.Errorf("generated source/driver binding differs")
		}
	}
	for _, step := range array(c["steps"]) {
		s := object(step)
		gen := object(s["generation"])
		report := object(gen["report"])
		text, _ := gen["source"].(string)
		if s["input_source_sha256"] != sd || report["source_digest"] != sd || report["generated_digest"] != digest([]byte(text)) ||
			report["replay_digest"] != digest([]byte(text)) {
			return fmt.Errorf("generation source/replay binding differs")
		}
	}
	if rt["typed_plan_sha256"] != object(c["plan"])["typed_plan_sha256"] {
		return fmt.Errorf("typed plan binding differs")
	}
	runs := array(rt["runs"])
	if len(runs) != 2 || object(runs[0])["stdout_sha256"] != object(runs[1])["stdout_sha256"] {
		return fmt.Errorf("two native run output hashes differ")
	}
	if object(rt["toolchain"])["stdout_sha256"] != digest([]byte(stringAt(rt, "go_version")+"\n")) {
		return fmt.Errorf("Go version process output binding differs")
	}
	// Composition uses its own producer-serialized subtree. Unlike prior typed
	// Result replay hashing, preserve original member order inside generic maps.
	var envelope, result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if err := json.Unmarshal(envelope["result"], &result); err != nil {
		return err
	}
	encoded, err := json.Marshal(result["composition"])
	if err != nil || rt["composition_sha256"] != digest(encoded) {
		return fmt.Errorf("own serialized composition binding differs")
	}
	return nil
}
