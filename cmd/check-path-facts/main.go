// Strict manual24 CLI/replay observer, adapted from the accepted pinned839
// path pilot. Full9841 native generated-function values are a separate layer.
// The Go path oracle is not linked into generated candidate source.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const compilerRevision = "205896a413e5dea92366309c35ae85e542d055f6"
const compilerVersion = "0.6.37-dev"
const goVersion = "go1.27.3"
const reportPackage = "app/path-facts"
const resultEntity = "gooo://go-ports/examples/path-facts/result"
const textEntity = "gooo://std/text"
const booleanEntity = "gooo://std/boolean"

var entryName, appPackage, activityName, activityID string
var goSources = map[string]string{
	"src/path/path.go":         "0a5024c364387eeea9ab5fe0b285f0d62355e8cd181cdbddfdfa227f6916d94c",
	"src/unicode/utf8/utf8.go": "3c145ee25fd7631ebbc670fd1236e8c163ca30ae8913c796489efbf3f267ad54",
}

type report struct {
	Text           string `json:"text"`
	Ext            string `json:"ext"`
	IsAbs          bool   `json:"is_abs"`
	TextByteLength int64  `json:"text_byte_length"`
	ExtByteLength  int64  `json:"ext_byte_length"`
	HasExtension   bool   `json:"has_extension"`
}
type oracleRow struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected report `json:"expected"`
}
type sourceIdentity struct {
	Package string `json:"package"`
	Digest  string `json:"source_digest"`
}
type identity struct {
	Manifest, Inputs, GoTool string
	Sources                  map[string]sourceIdentity
	Bodies                   map[string]string
}
type checked struct {
	Generated, Driver string
	Results           []any
}
type replayBinding struct{ Receipt, Result string }

var oracleReports []report

func configure(name string) error {
	entryName = name
	switch name {
	case "ext":
		appPackage, activityName, activityID = "go/path", "Ext", "gooo://go/path/Ext"
	case "isabs":
		appPackage, activityName, activityID = "go/path", "IsAbs", "gooo://go/path/IsAbs"
	case "report":
		appPackage, activityName, activityID = reportPackage, "Report", "gooo://go-ports/examples/path-facts/report"
	default:
		return fmt.Errorf("unknown path pilot entry")
	}
	return nil
}

func expectedValue(r report) any {
	switch entryName {
	case "ext":
		return r.Ext
	case "isabs":
		return r.IsAbs
	default:
		return r
	}
}

func run() error {
	entry := flag.String("entry", "", "ext, isabs or report")
	snapshot := flag.String("snapshot", ".", "frozen repository source root")
	inputs := flag.String("inputs", "", "exact original input-only file")
	execution := flag.String("execution", "", "actual native execution JSON")
	replay := flag.String("replay", "", "actual saved replay JSON")
	build := flag.String("compiler-build", "", "actual pinned version --build --json output")
	oracle := flag.String("oracle", "", "actual frozen independent original-Go output")
	manual := flag.String("manual", "", "frozen24 manual contract")
	out := flag.String("out", "", "new checked report, stdout when empty")
	fullOut := flag.String("full-out", "", "new generated-native full9841 evidence directory")
	flag.Parse()
	if err := configure(*entry); err != nil {
		return err
	}
	if *fullOut == "" || *inputs == "" || *execution == "" || *replay == "" || *build == "" || *oracle == "" || *manual == "" || flag.NArg() != 0 {
		return fmt.Errorf("require full-out, inputs, execution, replay, compiler-build, oracle and manual")
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
	if err := loadFrozenOracle(*oracle, *manual, rows); err != nil {
		return err
	}
	id, err := snapshotIdentity(*snapshot, publicManifest(entryName), inputRaw)
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
	priorResult, err := pinnedResultDigest(executionRaw)
	if err != nil {
		return fmt.Errorf("original result format: %w", err)
	}
	original, err := inspect(executionRaw, rows, id, replayBinding{})
	if err != nil {
		return fmt.Errorf("execution: %w", err)
	}
	repeated, err := inspect(replayRaw, rows, id, replayBinding{digest(executionRaw), priorResult})
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	if original.Generated != repeated.Generated || original.Driver != repeated.Driver || !reflect.DeepEqual(original.Results, repeated.Results) {
		return fmt.Errorf("saved replay changed source, driver or typed outputs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	full, err := checkGeneratedCorpus(ctx, *snapshot, *fullOut, executionRaw, replayRaw, original.Generated, id)
	if err != nil {
		return fmt.Errorf("full generated-native corpus: %w", err)
	}
	result := map[string]any{"schema": "gooo/path-facts-checked/v1", "decision": "MANUAL24_CLI_REPLAY_AND_FULL9841_GENERATED_NATIVE_PASS",
		"full_generated_corpus": full, "full_generated_native_outputs": 9841, "full_CLI_outputs": 0, "full_saved_replay_outputs": 0,
		"entry": entryName, "compiler_revision": compilerRevision, "compiler_version": compilerVersion, "go_version": goVersion,
		"native_observed_cases": len(rows), "independent_go_oracle_cases": len(rows), "saved_replay_observed_cases": len(rows),
		"native_receipt_expectation_cases": 0, "manifest_digest": id.Manifest, "inputs_digest": id.Inputs, "source_bindings": id.Sources,
		"execution_sha256": digest(executionRaw), "replay_sha256": digest(replayRaw), "prior_result_sha256": priorResult,
		"generated_sha256": original.Generated, "driver_sha256": original.Driver, "results": original.Results,
		"scope": "Fresh24 manual decoded validUTF8 Text inputs<=1024 bytes are independently compared through actualCLI andsavedreplay; finite_total remains0. Separately all9841 alphabet strings for this entry are evaluated by unchanged source-bound generated native function and compared to originalGo; these are not9841 individual CLI/savedreplay observations. No fullText-domain, filesystem authorization, library compatibility, performance or safety claim."}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if *out == "" {
		_, err = os.Stdout.Write(raw)
		return err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
		return fmt.Errorf("source compiler/Go pin changed")
	}
	for path, want := range goSources {
		raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil {
			return err
		}
		if digest(raw) != "sha256:"+want || object(lock["source_sha256"])[path] != want {
			return fmt.Errorf("independent Go source binding differs: %s", path)
		}
	}
	raw, err := os.ReadFile(buildPath)
	if err != nil {
		return err
	}
	v, err = decode(raw)
	if err != nil {
		return err
	}
	build := object(v)
	if build["schema"] != "gooo/build-identity/v1" || build["version"] != compilerVersion || build["compiler_source_sha"] != compilerRevision || build["vcs_revision"] != compilerRevision || build["vcs_modified"] != "false" || build["source_status"] != "CLEAN_VCS" || build["go_version"] != goVersion || build["native_go_required"] != goVersion {
		return fmt.Errorf("compiler build identity differs from clean839 source")
	}
	return nil
}

func readInputs(raw []byte) ([]string, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("input raw bytes are not UTF8")
	}
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	input := object(v)
	if len(input) != 2 || input["schema"] != "gooo/body-composition-inputs/v1" || len(array(input["inputs"])) != 24 {
		return nil, fmt.Errorf("require exactly24 input-only rows")
	}
	var rows []string
	for i, value := range array(input["inputs"]) {
		item := object(value)
		text, ok := item[appPackage+":"+activityName].(string)
		if len(item) != 1 || !ok || !utf8.ValidString(text) || len(text) > 1024 {
			return nil, fmt.Errorf("input row %d requires original valid UTF8 Text<=1024", i)
		}
		rows = append(rows, text)
	}
	return rows, nil
}

func loadFrozenOracle(oraclePath, manualPath string, rows []string) error {
	raw, err := os.ReadFile(oraclePath)
	if err != nil {
		return err
	}
	if _, err := decode(raw); err != nil {
		return err
	}
	manual, err := os.ReadFile(manualPath)
	if err != nil {
		return err
	}
	if _, err := decode(manual); err != nil {
		return err
	}
	var frozen struct {
		Schema string      `json:"schema"`
		Status string      `json:"status"`
		Cases  []oracleRow `json:"cases"`
	}
	var observed struct {
		Schema    string      `json:"schema"`
		Status    string      `json:"status"`
		GoVersion string      `json:"go_version"`
		Source    string      `json:"go_path_source_sha256"`
		Corpus    string      `json:"corpus"`
		ManualSHA string      `json:"manual_contract_sha256"`
		Cases     int         `json:"cases"`
		Results   []oracleRow `json:"results"`
	}
	if err := json.Unmarshal(manual, &frozen); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &observed); err != nil {
		return err
	}
	if frozen.Schema != "gooo/path-facts-manual-contract/v1" || frozen.Status != "FROZEN_MANUAL24_UNEXECUTED" || len(frozen.Cases) != 24 ||
		observed.Schema != "gooo/path-facts-original-go-oracle/v1" || observed.Status != "ORIGINAL_GO_EXPECTATIONS_ONLY_NO_GOOO_EXECUTION" || observed.GoVersion != goVersion || observed.Source != goSources["src/path/path.go"] || observed.Corpus != "manual" || observed.ManualSHA != digest(manual) || observed.Cases != 24 || !reflect.DeepEqual(observed.Results, frozen.Cases) {
		return fmt.Errorf("actual original-Go oracle/frozen manual binding differs")
	}
	for i, row := range observed.Results {
		if row.Name == "" || row.Input != rows[i] || row.Expected.Text != rows[i] {
			return fmt.Errorf("frozen oracle original input/order at row %d", i)
		}
		oracleReports = append(oracleReports, row.Expected)
	}
	return nil
}

func snapshotIdentity(root, manifest string, inputs []byte) (identity, error) {
	raw, err := os.ReadFile(filepath.Join(root, manifest))
	if err != nil {
		return identity{}, err
	}
	value, err := decode(raw)
	if err != nil {
		return identity{}, err
	}
	m := object(value)
	if m["schema"] != "gooo/package-workspace-manifest/v1" || object(m["entry"])["package_path"] != appPackage || object(m["entry"])["activity"] != activityName {
		return identity{}, fmt.Errorf("manifest entry differs")
	}
	roster := map[string]string{"ports/path/path.gooo": "go/path"}
	if entryName == "report" {
		roster["examples/path-facts/main.gooo"] = reportPackage
	}
	id := identity{Manifest: digest(raw), Inputs: digest(inputs), Sources: map[string]sourceIdentity{}, Bodies: map[string]string{}}
	seen := map[string]bool{}
	for _, value := range array(m["packages"]) {
		pkg := object(value)
		path := str(pkg["path"])
		if (path != "go/path" && !(entryName == "report" && path == reportPackage)) || seen[path] {
			return identity{}, fmt.Errorf("manifest package roster differs")
		}
		seen[path] = true
		for _, item := range array(pkg["sources"]) {
			name := str(item)
			if roster[name] != path || id.Sources[name].Package != "" {
				return identity{}, fmt.Errorf("manifest source roster differs")
			}
			source, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				return identity{}, err
			}
			encoded, _ := json.Marshal(string(source))
			id.Sources[name] = sourceIdentity{path, digest(encoded)}
			for _, body := range regexp.MustCompile("(?s)activity\\s+(\\w+)[^`]* computes `(.*?)`").FindAllStringSubmatch(string(source), -1) {
				id.Bodies[path+":"+body[1]] = digest([]byte(body[2]))
			}
		}
	}
	wantPackages := 1
	if entryName == "report" {
		wantPackages = 2
	}
	if len(seen) != wantPackages || len(id.Sources) != len(roster) {
		return identity{}, fmt.Errorf("manifest source roster incomplete")
	}
	goTool, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "bin/go"))
	if err != nil {
		return identity{}, err
	}
	id.GoTool = digest(goTool)
	return id, nil
}

func decode(raw []byte) (any, error) {
	if err := jsonkeys.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value, extra any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	return value, nil
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func str(v any) string            { s, _ := v.(string); return s }
func isInt(v any, want int) bool {
	n, ok := v.(json.Number)
	return ok && n.String() == fmt.Sprint(want)
}
func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func validDigest(v any) bool {
	s := str(v)
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
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
func publicManifest(name string) string {
	switch name {
	case "ext":
		return "path-ext.workspace.json"
	case "isabs":
		return "path-isabs.workspace.json"
	default:
		return "path-facts.workspace.json"
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
