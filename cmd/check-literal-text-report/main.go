// check-literal-text-report checks an input-only native receipt and its saved
// replay against independent Go strings.Index/Count references. It never
// substitutes a Go implementation for the Gooo program's returned values.
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
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const (
	compilerRevision = "83928fa4538245abe33b08549da47f8cbdc50ed2"
	compilerVersion  = "0.6.32-dev"
	goVersion        = "go1.27.2"
	appPackage       = "app/literal-text-report"
	activityID       = "gooo://go-ports/examples/literal-text-report/report"
	resultEntity     = "gooo://go-ports/examples/literal-text-report/result"
	textEntity       = "gooo://std/text"
	integerEntity    = "gooo://std/integer"
)

var goSources = map[string]string{
	"src/strings/strings.go":              "a81bdf73a308277c322e35d0c602e3813f57b6f769c091f8f8962c8884a45ea3",
	"src/internal/stringslite/strings.go": "3ed7df2eeabb5427d44b95e6cfe66ef03e13f1c969ad08050a4a0e89d43b8af2",
	"src/unicode/utf8/utf8.go":            "3c145ee25fd7631ebbc670fd1236e8c163ca30ae8913c796489efbf3f267ad54",
}

type row struct{ Text, Needle string }
type report struct {
	Text                string `json:"text"`
	Needle              string `json:"needle"`
	FirstByteOffset     int    `json:"first_byte_offset"`
	Found               bool   `json:"found"`
	NonOverlappingCount int    `json:"non_overlapping_count"`
	TextByteLength      int    `json:"text_byte_length"`
	NeedleByteLength    int    `json:"needle_byte_length"`
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	snapshot := flag.String("snapshot", ".", "frozen source root with upstream.json and the manifest")
	manifest := flag.String("manifest", "literal-text-report.workspace.json", "manifest path relative to snapshot")
	inputs := flag.String("inputs", "examples/literal-text-report/inputs.json", "original supplied input file")
	execution := flag.String("execution", "", "input-only native execution JSON")
	replay := flag.String("replay", "", "saved native replay JSON")
	build := flag.String("compiler-build", "", "pinned compiler version --build --json output")
	out := flag.String("out", "", "new checked report JSON; defaults to stdout")
	flag.Parse()
	if *execution == "" || *replay == "" || *build == "" || flag.NArg() != 0 {
		return fmt.Errorf("require --execution, --replay and --compiler-build")
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
	id, err := snapshotIdentity(*snapshot, *manifest, inputRaw)
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
		return fmt.Errorf("saved replay changed the generated program, driver or outputs")
	}
	result := map[string]any{
		"schema": "gooo/literal-text-report-checked/v1", "decision": "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY",
		"compiler_revision": compilerRevision, "compiler_version": compilerVersion, "go_version": goVersion,
		"go_source_sha256": goSources, "manifest_digest": id.Manifest, "inputs_digest": id.Inputs,
		"source_bindings":  id.Sources,
		"execution_sha256": digest(executionRaw), "replay_sha256": digest(replayRaw),
		"prior_result_sha256": priorResult,
		"generated_sha256":    original.Generated, "driver_sha256": original.Driver,
		"native_observed_cases": len(rows), "independent_go_oracle_cases": len(rows), "saved_replay_observed_cases": len(rows),
		"native_receipt_expectation_cases": 0, "results": original.Results,
		"scope": "Finite importing-consumer observations for decoded valid UTF-8 Text <=1024 bytes per argument. Input-only receipts remain OBSERVED with finite_total=0; this separate checker compares returned values with independent Go strings.Index/Count. No full-input, whole-package, customer-demand, adoption or performance claim.",
	}
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

func checkOracle(snapshot, buildPath string) error {
	if runtime.Version() != goVersion {
		return fmt.Errorf("oracle toolchain %s, require %s", runtime.Version(), goVersion)
	}
	lockRaw, err := os.ReadFile(filepath.Join(snapshot, "upstream.json"))
	if err != nil {
		return err
	}
	lock, err := decode(lockRaw)
	if err != nil {
		return err
	}
	l := object(lock)
	if l["go_version"] != goVersion || l["compiler_revision"] != compilerRevision || l["compiler_version"] != compilerVersion {
		return fmt.Errorf("upstream pin differs from this pilot")
	}
	for path, want := range goSources {
		actual, err := os.ReadFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil {
			return err
		}
		if object(l["source_sha256"])[path] != want || digest(actual) != "sha256:"+want {
			return fmt.Errorf("independent Go source binding differs: %s", path)
		}
	}
	buildRaw, err := os.ReadFile(buildPath)
	if err != nil {
		return err
	}
	v, err := decode(buildRaw)
	if err != nil {
		return err
	}
	b := object(v)
	if b["schema"] != "gooo/build-identity/v1" || b["version"] != compilerVersion || b["compiler_source_sha"] != compilerRevision || b["vcs_revision"] != compilerRevision || b["vcs_modified"] != "false" || b["source_status"] != "CLEAN_VCS" || b["go_version"] != goVersion || b["native_go_required"] != goVersion {
		return fmt.Errorf("compiler build identity differs from the pinned clean source")
	}
	return nil
}

func readInputs(raw []byte) ([]row, error) {
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	input := object(v)
	if len(input) != 2 || input["schema"] != "gooo/body-composition-inputs/v1" || len(array(input["inputs"])) == 0 {
		return nil, fmt.Errorf("require a nonempty input-only file")
	}
	var rows []row
	for i, value := range array(input["inputs"]) {
		item := object(value)
		text, textOK := item[appPackage+":Report.input0"].(string)
		needle, needleOK := item[appPackage+":Report.input1"].(string)
		if len(item) != 2 || !textOK || !needleOK || !utf8.ValidString(text) || !utf8.ValidString(needle) || len(text) > 1024 || len(needle) > 1024 {
			return nil, fmt.Errorf("input %d requires two valid UTF-8 Text values <=1024 bytes", i)
		}
		rows = append(rows, row{text, needle})
	}
	return rows, nil
}

func reference(r row) report {
	offset := strings.Index(r.Text, r.Needle)
	return report{r.Text, r.Needle, offset, offset >= 0, strings.Count(r.Text, r.Needle), len(r.Text), len(r.Needle)}
}

func snapshotIdentity(root, manifest string, inputs []byte) (identity, error) {
	raw, err := os.ReadFile(filepath.Join(root, manifest))
	if err != nil {
		return identity{}, err
	}
	v, err := decode(raw)
	if err != nil {
		return identity{}, err
	}
	m := object(v)
	if m["schema"] != "gooo/package-workspace-manifest/v1" || object(m["entry"])["package_path"] != appPackage || object(m["entry"])["activity"] != "Report" {
		return identity{}, fmt.Errorf("manifest entry differs")
	}
	roster := map[string]string{
		"ports/strings/search.gooo": "go/strings", "ports/strings/literal.gooo": "go/strings",
		"examples/literal-text-report/main.gooo": appPackage,
	}
	id := identity{Manifest: digest(raw), Inputs: digest(inputs), Sources: map[string]sourceIdentity{}, Bodies: map[string]string{}}
	seen := map[string]bool{}
	for _, value := range array(m["packages"]) {
		pkg := object(value)
		path := str(pkg["path"])
		if (path != "go/strings" && path != appPackage) || seen[path] {
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
	if len(seen) != 2 || len(id.Sources) != len(roster) {
		return identity{}, fmt.Errorf("manifest source roster is incomplete")
	}
	goTool, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "bin", "go"))
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
	var v, extra any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return v, nil
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
func equal(v any, want any) bool {
	raw, err := json.Marshal(want)
	if err != nil {
		return false
	}
	decoded, err := decode(raw)
	return err == nil && reflect.DeepEqual(v, decoded)
}
