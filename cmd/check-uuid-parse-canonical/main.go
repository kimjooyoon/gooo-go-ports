// check-uuid-parse-canonical compares input-only observations with the original
// Google UUID oracle. It does not supply UUID behavior to the compiled program.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const (
	compilerRevision = "205896a413e5dea92366309c35ae85e542d055f6"
	compilerVersion  = "0.6.37-dev"
	goVersion        = "go1.27.3"
	manifestName     = "uuid-parse-canonical.workspace.json"
	appPackage       = "go/github.com/google/uuid/text"
	uuidPackage      = "go/github.com/google/uuid"
	quotePackage     = "go/github.com/google/uuid/internal/quote"
	activityID       = "gooo://github.com/google/uuid/text/parse-canonical"
	resultEntity     = "gooo://github.com/google/uuid/text/parse-canonical-result"
	textEntity       = "gooo://std/text"
	uuidRevision     = "0f11ee6918f41a04c201eceeadf612a377bc7fbc"
	uuidModuleH1     = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0="
)

var oracleSources = map[string]string{
	"uuid.go": "0edec8e34c6b6fe0db31b71a29069a09ed832e3fd04ee0175916b58f2b60e5c1",
	"util.go": "755f50afafb13dd89ae6177da7eb6dc31ae650ece2f9ba52821021e9f8de044e",
	"LICENSE": "0a8d61ed3cbfd5312326e8126c31ce9c627a283adc99131b56896d29ada04b2d",
}
var sourceRoster = map[string]string{
	"ports/google/uuid/quote.gooo":                quotePackage,
	"ports/google/uuid/validate.gooo":             uuidPackage,
	"ports/google/uuid/text/parse_canonical.gooo": appPackage,
}
var resultNames = []string{"parsed", "canonical_uuid", "error", "invalid_length"}
var resultIDs = []string{"parsed", "canonical-uuid", "error", "invalid-length"}

type row struct{ Text string }
type report struct {
	Parsed        bool   `json:"parsed"`
	CanonicalUUID string `json:"canonical_uuid"`
	Error         string `json:"error"`
	InvalidLength bool   `json:"invalid_length"`
}
type sourceIdentity struct {
	Package string `json:"package"`
	Digest  string `json:"source_digest"`
}
type identity struct {
	Manifest, Inputs, GoTool string
	Sources                  map[string]sourceIdentity
	Declarations             map[string]declaration
	Calls                    []callSite
}
type checked struct {
	Generated, Driver          string
	Program, Plan, Composition any
	Results                    []any
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	snapshot := flag.String("snapshot", ".", "frozen source root")
	manifest := flag.String("manifest", manifestName, "fixed UUID manifest relative to snapshot")
	inputs := flag.String("inputs", "examples/uuid-parse-canonical/inputs.json", "original input-only file")
	execution := flag.String("execution", "", "input-only native execution JSON")
	replay := flag.String("replay", "", "saved native replay JSON")
	build := flag.String("compiler-build", "", "pinned compiler version --build --json output")
	out := flag.String("out", "", "exclusive new report file; defaults to stdout")
	flag.Parse()
	if *manifest != manifestName || *execution == "" || *replay == "" || *build == "" || flag.NArg() != 0 {
		return fmt.Errorf("require fixed UUID manifest, --execution, --replay and --compiler-build")
	}
	if err := checkOracle(*snapshot, *build); err != nil {
		return err
	}
	inputRaw, err := os.ReadFile(*inputs)
	if err != nil {
		return err
	}
	rows, err := readInputs(inputRaw)
	if err != nil {
		return err
	}
	id, err := snapshotIdentity(*snapshot, inputRaw)
	if err != nil {
		return err
	}
	executionRaw, err := os.ReadFile(*execution)
	if err != nil {
		return err
	}
	replayRaw, err := os.ReadFile(*replay)
	if err != nil {
		return err
	}
	original, err := inspect(executionRaw, rows, id, "")
	if err != nil {
		return fmt.Errorf("execution: %w", err)
	}
	repeated, err := inspect(replayRaw, rows, id, digest(executionRaw))
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	if err := compareReplay(original, repeated); err != nil {
		return err
	}
	result := checkedReport(rows, id, original, executionRaw, replayRaw)
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if *out == "" {
		_, err = os.Stdout.Write(raw)
		return err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func checkedReport(rows []row, id identity, original checked, executionRaw, replayRaw []byte) map[string]any {
	return map[string]any{
		"schema": "gooo/uuid-parse-canonical-checked/v1", "decision": "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY",
		"compiler_revision": compilerRevision, "compiler_version": compilerVersion, "go_version": goVersion,
		"oracle_module": "github.com/google/uuid", "oracle_version": "v1.6.0", "oracle_revision": uuidRevision,
		"oracle_source_sha256": oracleSources, "manifest_digest": id.Manifest, "inputs_digest": id.Inputs,
		"source_bindings": id.Sources, "execution_sha256": digest(executionRaw), "replay_sha256": digest(replayRaw),
		"generated_sha256": original.Generated, "driver_sha256": original.Driver,
		"native_observed_cases": len(rows), "independent_go_oracle_cases": len(rows), "saved_replay_observed_cases": len(rows),
		"native_receipt_expectation_cases": 0, "results": original.Results,
		"verification_scope": map[string]string{
			"input_source_entry_and_compiler_bindings":                       "CHECKED",
			"independent_original_google_uuid_oracle":                        "CHECKED",
			"ordered_result_fields_and_types":                                "CHECKED",
			"pure_call_source_bodies_and_sites":                              "CHECKED",
			"generation_provenance_source_and_generated_hashes":              "CHECKED",
			"generation_typecheck_and_deterministic_replay_flags":            "CHECKED",
			"runtime_to_typed_plan_digest_binding":                           "CHECKED",
			"outer_saved_receipt_digest":                                     "CHECKED",
			"identical_saved_program_composition_and_outputs":                "CHECKED",
			"prior_typed_result_sha256":                                      "NOT_CHECKED",
			"typed_plan_digest_recomputation":                                "NOT_CHECKED",
			"compiler_generation_or_native_execution_reperformed_by_checker": "NOT_CHECKED",
			"unmodeled_receipt_fields_and_typed_serialization":               "NOT_CHECKED",
		},
		"scope": "Finite observations for decoded valid UTF-8 Text <=1024 bytes. Input-only receipts remain OBSERVED with finite_total=0. Original Google uuid.Parse, returned UUID.String, exact error and IsInvalidLengthError are independent host references. The inner replay prior_result_sha256 is present and syntactically validated but is NOT_CHECKED against a decoded typed Result; typed-plan digest recomputation and compiler projection reconstruction are NOT_CHECKED. No arbitrary-byte ParseBytes, binary UUID method set, exhaustive/full-module compatibility, adoption, performance or signed-attestation claim.",
	}
}

func checkOracle(snapshot, buildPath string) error {
	if runtime.Version() != goVersion {
		return fmt.Errorf("oracle toolchain %s, require %s", runtime.Version(), goVersion)
	}
	lockRaw, err := os.ReadFile(filepath.Join(snapshot, "upstream.json"))
	if err != nil {
		return err
	}
	v, err := decode(lockRaw)
	if err != nil {
		return err
	}
	lock := object(v)
	if lock["go_version"] != goVersion || lock["compiler_revision"] != compilerRevision || lock["compiler_version"] != compilerVersion {
		return fmt.Errorf("upstream pin differs")
	}
	if !reflect.DeepEqual(uuid.PinnedSourceSHA256(), oracleSources) {
		return fmt.Errorf("compiled original UUID oracle source differs")
	}
	for name, want := range oracleSources {
		raw, err := os.ReadFile(filepath.Join(snapshot, "third_party/google-uuid", name))
		if err != nil {
			return err
		}
		if digest(raw) != "sha256:"+want {
			return fmt.Errorf("original UUID source differs: %s", name)
		}
	}
	matched := 0
	for _, value := range array(lock["external_sources"]) {
		s := object(value)
		if s["target"] != "ports/google/uuid/text/parse_canonical.gooo" {
			continue
		}
		matched++
		if s["module"] != "github.com/google/uuid" || s["repository"] != "https://github.com/google/uuid" || s["version"] != "v1.6.0" || s["revision"] != uuidRevision || s["module_h1"] != uuidModuleH1 || s["source"] != "uuid.go" || s["source_sha256"] != oracleSources["uuid.go"] || s["oracle_copy"] != "third_party/google-uuid/uuid.go" || s["license"] != "third_party/google-uuid/LICENSE" || s["license_sha256"] != oracleSources["LICENSE"] || !equal(s["supporting_sources"], []string{"util.go"}) || !equal(s["supporting_source_sha256"], map[string]string{"util.go": oracleSources["util.go"]}) || !equal(s["symbols"], []string{"Parse", "UUID.String"}) {
			return fmt.Errorf("UUID oracle provenance differs")
		}
	}
	if matched != 1 {
		return fmt.Errorf("UUID oracle metadata roster differs")
	}
	buildRaw, err := os.ReadFile(buildPath)
	if err != nil {
		return err
	}
	v, err = decode(buildRaw)
	if err != nil {
		return err
	}
	b := object(v)
	if b["schema"] != "gooo/build-identity/v1" || b["version"] != compilerVersion || b["compiler_source_sha"] != compilerRevision || b["vcs_revision"] != compilerRevision || b["vcs_modified"] != "false" || b["source_status"] != "CLEAN_VCS" || b["go_version"] != goVersion || b["native_go_required"] != goVersion {
		return fmt.Errorf("compiler build identity differs from pinned clean source")
	}
	return nil
}

func readInputs(raw []byte) ([]row, error) {
	if len(raw) > 32768 {
		return nil, fmt.Errorf("input envelope exceeds 32768 bytes")
	}
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	input := object(v)
	values := array(input["inputs"])
	if !keys(input, "schema", "inputs") || input["schema"] != "gooo/body-composition-inputs/v1" || len(values) == 0 || len(values) > 128 {
		return nil, fmt.Errorf("require 1..128 input-only cases")
	}
	rows := make([]row, len(values))
	for i, value := range values {
		item := object(value)
		text, ok := item[appPackage+":ParseCanonical"].(string)
		if !keys(item, appPackage+":ParseCanonical") || !ok || !utf8.ValidString(text) || len(text) > 1024 {
			return nil, fmt.Errorf("input %d requires decoded valid UTF-8 Text <=1024 bytes", i)
		}
		rows[i] = row{text}
	}
	return rows, nil
}

func reference(r row) report {
	value, err := uuid.Parse(r.Text)
	message := ""
	if err != nil {
		message = err.Error()
	}
	return report{err == nil, value.String(), message, uuid.IsInvalidLengthError(err)}
}

func snapshotIdentity(root string, inputs []byte) (identity, error) {
	raw, err := os.ReadFile(filepath.Join(root, manifestName))
	if err != nil {
		return identity{}, err
	}
	v, err := decode(raw)
	if err != nil {
		return identity{}, err
	}
	m := object(v)
	if !keys(m, "schema", "entry", "packages") || m["schema"] != "gooo/package-workspace-manifest/v1" || !equal(m["entry"], map[string]string{"package_path": appPackage, "activity": "ParseCanonical"}) {
		return identity{}, fmt.Errorf("manifest entry differs")
	}
	id := identity{Manifest: digest(raw), Inputs: digest(inputs), Sources: map[string]sourceIdentity{}, Declarations: map[string]declaration{}}
	seen := map[string]bool{}
	for _, value := range array(m["packages"]) {
		pkg := object(value)
		pkgPath := str(pkg["path"])
		if !keys(pkg, "path", "sources") || !knownPackage(pkgPath) || seen[pkgPath] || len(array(pkg["sources"])) != 1 {
			return identity{}, fmt.Errorf("manifest package roster differs")
		}
		seen[pkgPath] = true
		for _, item := range array(pkg["sources"]) {
			name := str(item)
			if sourceRoster[name] != pkgPath || id.Sources[name].Package != "" {
				return identity{}, fmt.Errorf("manifest source roster differs")
			}
			source, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				return identity{}, err
			}
			if !utf8.Valid(source) {
				return identity{}, fmt.Errorf("invalid UTF-8 source")
			}
			encoded, _ := json.Marshal(string(source))
			id.Sources[name] = sourceIdentity{pkgPath, digest(encoded)}
			if err := indexDeclarations(name, pkgPath, string(source), id.Declarations); err != nil {
				return identity{}, err
			}
		}
	}
	if len(seen) != 3 || len(id.Sources) != 3 {
		return identity{}, fmt.Errorf("incomplete manifest source roster")
	}
	id.Calls, err = sourceCalls(id.Declarations)
	if err != nil {
		return identity{}, err
	}
	goTool, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "bin", "go"))
	if err != nil {
		return identity{}, err
	}
	id.GoTool = digest(goTool)
	return id, nil
}

func decode(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > 32<<20 || !utf8.Valid(raw) {
		return nil, fmt.Errorf("JSON exceeds supported size or UTF-8 domain")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := checkDepth(d, 0); err != nil {
		return nil, err
	}
	if err := jsonkeys.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v, extra any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return v, nil
}
func checkDepth(d *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON exceeds 64 nested levels")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	for d.More() {
		if delim == '{' {
			if _, err := d.Token(); err != nil {
				return err
			}
		}
		if err := checkDepth(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func str(v any) string            { s, _ := v.(string); return s }
func keys(m map[string]any, names ...string) bool {
	if m == nil || len(m) != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := m[name]; !ok {
			return false
		}
	}
	return true
}
func knownPackage(p string) bool { return p == appPackage || p == uuidPackage || p == quotePackage }
func isInt(v any, want int) bool {
	n, ok := v.(json.Number)
	return ok && n.String() == fmt.Sprint(want)
}
func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func validDigest(v any) bool {
	s := str(v)
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}
func equal(v, want any) bool {
	raw, err := json.Marshal(want)
	if err != nil {
		return false
	}
	decoded, err := decode(raw)
	return err == nil && reflect.DeepEqual(v, decoded)
}
