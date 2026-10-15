// Fixed manual16 native/saved-replay observer. No policy evaluator is linked here.
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
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const compilerRevision = "83928fa4538245abe33b08549da47f8cbdc50ed2"
const compilerVersion = "0.6.32-dev"
const goVersion = "go1.27.2"
const appPackage = "app/automatic-redelivery"
const activityName = "AssessRedelivery"
const activityID = "gooo://automatic-redelivery/assess/v1"
const resultEntity = "gooo://automatic-redelivery/decision/v1"
const textEntity = "gooo://std/text"
const booleanEntity = "gooo://std/boolean"
const integerEntity = "gooo://std/integer"
const sourceName = "examples/automatic-redelivery/main.gooo"
const manifestName = "automatic-redelivery.workspace.json"
const contractSHA = "e9dadc2b2bfe776648b216477ffd9cfe7b23ee0deaf2b22bd3ab27c268acdac0"
const candidateSHA = "5ade10c8187e282dc14eef5f47d0042b5b0541d7a55ba95238eb420860ef4833"
const manifestSHA = "e2a93a075c41fc8212b03ac5c14f2ad4256a929df3c9e4a9054c85a5b8443d65"
const nativeInputsSHA = "042e71cd5a7635999f8d6c56da27cf26beb2f57739c7ec0238cfd995cad20e39"
const witnessSHA = "29e4e321caaaf6a2c17e1ab899c8268aa8912de16a9896c1b3770a78c3c14e16"
const loweredName = "GoooPackage0ActivityAssessRedelivery"
const generatedName = "GoooComposedActivity0"

var fieldNames = []string{"method", "delivery_state", "body_replayable", "idempotency_key_present", "server_repeat_contract_verified", "request_binding_current", "current_access_allowed", "attempts_remaining", "provider_retry_guidance", "domain_status", "action", "reason", "eligible_to_claim", "attempts_if_claimed", "dedup_required"}
var fieldTypes = []string{"string", "string", "bool", "bool", "bool", "bool", "bool", "int64", "string", "string", "string", "string", "bool", "int64", "bool"}
var inputEntities = []string{textEntity, textEntity, booleanEntity, booleanEntity, booleanEntity, booleanEntity, booleanEntity, integerEntity, textEntity}
var inputTypes = []string{"Text", "Text", "Boolean", "Boolean", "Boolean", "Boolean", "Boolean", "Integer", "Text"}
var caseNames = []string{"invalid-enum-before-access", "allow-does-not-bypass-access", "allow-does-not-bypass-binding", "applied-veto-is-lookup-only", "veto-before-budget-and-body", "allow-does-not-bypass-budget", "body-before-unverified-guidance", "unknown-guidance-blocks-unsent", "terminal-whole-operation-nonapplication", "allow-does-not-prove-server-contract", "verified-read-repeat-without-key", "allow-does-not-supply-bound-key", "verified-write-dedup", "signed-min-echo-no-arithmetic", "signed-max-echo-no-arithmetic", "verified-unsent-automatic-claim"}

type sourceIdentity struct {
	Package string `json:"package"`
	Digest  string `json:"source_digest"`
}
type identity struct {
	Manifest, Inputs, GoTool, Body string
	Sources                        map[string]sourceIdentity
}
type checked struct {
	Generated, Driver, TypedPlan string
	Results                      []any
}
type replayBinding struct{ Receipt, Result string }
type oracleRow struct {
	Index            int
	Name             string
	Inputs, Expected map[string]any
}

func run() error {
	snapshot := flag.String("snapshot", "", "current public repository root")
	contract := flag.String("contract", "", "public original contract")
	oracle := flag.String("oracle", "", "fresh independent Go manual16 output")
	origin := flag.String("oracle-origin", "", "fresh independent Go origin bindings")
	inputs := flag.String("inputs", "", "input-only exact batch of eight")
	offset := flag.Int("offset", -1, "fixed interval start: 0 or 8")
	count := flag.Int("count", 0, "fixed interval length: 8")
	execution := flag.String("execution", "", "fresh native receipt")
	replay := flag.String("replay", "", "fresh saved-replay receipt")
	build := flag.String("compiler-build", "", "pinned compiler build identity")
	goTool := flag.String("go-tool", "", "explicit pinned Go executable")
	profile := flag.String("result-profile", "", "shared pinned839 Result profile")
	checkInput := flag.String("check-input", "", "strict native wire preflight only")
	format := flag.String("format", "", "native preflight format")
	out := flag.String("out", "", "new checked JSON file")
	flag.Parse()
	if *profile == "" {
		return fmt.Errorf("require --result-profile")
	}
	if err := loadResultProfile(*profile); err != nil {
		return err
	}
	if *checkInput != "" {
		if *format != "native" || *snapshot != "" || *contract != "" || *oracle != "" || *origin != "" || *inputs != "" || *execution != "" || *replay != "" || *build != "" || *goTool != "" || *out != "" || *offset != -1 || *count != 0 || flag.NArg() != 0 {
			return fmt.Errorf("require only --check-input --format native --result-profile")
		}
		raw, err := readLimited(*checkInput, 1<<20)
		if err != nil {
			return err
		}
		rows, err := checkNativeWire(raw)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "gooo/redelivery-native-inputs-checked/v1", "count": len(rows), "required_input_fields": 9})
	}
	if *format != "" {
		return fmt.Errorf("format only supported with --check-input")
	}
	if *snapshot == "" || *contract == "" || *oracle == "" || *origin == "" || *inputs == "" || *execution == "" || *replay == "" || *build == "" || *goTool == "" || *out == "" || (*offset != 0 && *offset != 8) || *count != 8 || flag.NArg() != 0 {
		return fmt.Errorf("require all named bindings, offset 0 or 8, and count 8")
	}
	contractRaw, err := readBound(*contract, contractSHA)
	if err != nil {
		return err
	}
	if _, err = decode(contractRaw); err != nil {
		return err
	}
	toolRef, err := checkToolchain(*goTool, *build)
	if err != nil {
		return err
	}
	inputRaw, err := readLimited(*inputs, 1<<20)
	if err != nil {
		return err
	}
	if len(inputRaw) > 8192 {
		return fmt.Errorf("manual batch raw wire cap")
	}
	rows, err := readInputs(inputRaw, 8)
	if err != nil {
		return err
	}
	allRaw, err := readBound(filepath.Join(*snapshot, "examples/automatic-redelivery/manual-inputs.json"), nativeInputsSHA)
	if err != nil {
		return err
	}
	allInputs, err := readInputs(allRaw, 16)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(rows, allInputs[*offset:*offset+8]) {
		return fmt.Errorf("batch input interval/order differs")
	}
	inputRef, err := currentRef(*inputs, 1<<20)
	if err != nil {
		return err
	}
	originRef, err := currentRef(*origin, 1<<20)
	if err != nil {
		return err
	}
	buildRef, err := currentRef(*build, 1<<20)
	if err != nil {
		return err
	}
	profileRef, err := currentRef(*profile, 1<<20)
	if err != nil {
		return err
	}
	originRefs, err := checkOracleOrigin(*origin, *oracle, *contract, toolRef, *snapshot, profileRef, originRef)
	if err != nil {
		return fmt.Errorf("oracle origin: %w", err)
	}
	oracleRaw, err := readCurrentBytes(originRefs.Oracle, 1<<20)
	if err != nil {
		return err
	}
	witnessesRaw, err := readBound(originRefs.Witness.Path, witnessSHA)
	if err != nil {
		return err
	}
	oracleRows, err := loadOracle(oracleRaw, witnessesRaw, allInputs)
	if err != nil {
		return err
	}
	selected := oracleRows[*offset : *offset+8]
	id, err := snapshotIdentity(*snapshot, inputRaw, "sha256:"+toolRef.SHA256)
	if err != nil {
		return err
	}
	exRaw, err := readLimited(*execution, 1<<20)
	if err != nil {
		return err
	}
	reRaw, err := readLimited(*replay, 1<<20)
	if err != nil {
		return err
	}
	exRef, err := currentRef(*execution, 1<<20)
	if err != nil {
		return err
	}
	reRef, err := currentRef(*replay, 1<<20)
	if err != nil {
		return err
	}
	if "sha256:"+exRef.SHA256 != digest(exRaw) || "sha256:"+reRef.SHA256 != digest(reRaw) || "sha256:"+inputRef.SHA256 != digest(inputRaw) {
		return fmt.Errorf("receipt/input changed between reads")
	}
	ex, err := inspect(exRaw, rows, selected, id, replayBinding{})
	if err != nil {
		return fmt.Errorf("execution: %w", err)
	}
	prior, err := pinnedResultDigest(exRaw)
	if err != nil {
		return fmt.Errorf("original pinned result: %w", err)
	}
	re, err := inspect(reRaw, rows, selected, id, replayBinding{digest(exRaw), prior})
	if err != nil {
		return fmt.Errorf("saved replay: %w", err)
	}
	if ex.Generated != re.Generated || ex.Driver != re.Driver || ex.TypedPlan != re.TypedPlan || !reflect.DeepEqual(ex.Results, re.Results) {
		return fmt.Errorf("saved replay changed typed plan, source, driver or values")
	}
	// Rehash current oracle/source/tool/process/STARTED refs after receipt inspection.
	postOrigin, err := checkOracleOrigin(*origin, *oracle, *contract, toolRef, *snapshot, profileRef, originRef)
	if err != nil {
		return fmt.Errorf("oracle origin postcheck: %w", err)
	}
	if !reflect.DeepEqual(postOrigin, originRefs) {
		return fmt.Errorf("oracle origin changed during inspection")
	}
	post, err := snapshotIdentity(*snapshot, inputRaw, "sha256:"+toolRef.SHA256)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(post, id) {
		return fmt.Errorf("candidate snapshot changed during inspection")
	}
	postTool, err := checkToolchain(*goTool, *build)
	if err != nil {
		return err
	}
	if postTool != toolRef {
		return fmt.Errorf("Go tool changed during inspection")
	}
	for _, want := range []rawRef{inputRef, originRef, buildRef, profileRef, exRef, reRef} {
		got, err := currentRef(want.Path, 1<<20)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("evidence input changed during inspection: %s", want.Path)
		}
	}
	result := map[string]any{"schema": "gooo/redelivery-manual-batch-checked/v1", "decision": "FIXED_MANUAL16_SLICE_NATIVE_AND_SAVED_REPLAY_PASS", "profile": "gooo/automatic-redelivery-manual16/v1", "entry": appPackage + ":" + activityName, "activity_id": activityID, "offset": *offset, "count": 8, "manual_total": 16, "required_output_fields": 15, "native_observed_cases": 8, "saved_replay_observed_cases": 8, "independent_go_oracle_cases": 8, "typed_fields_compared_per_phase": 120, "echo_values_compared_per_phase": 72, "native_receipt_expectation_cases": 0, "full_native_cases": 0, "full_CLI_cases": 0, "full_saved_replay_cases": 0, "compiler_revision": compilerRevision, "compiler_version": compilerVersion, "go_version": goVersion, "contract_sha256": "sha256:" + contractSHA, "manifest_digest": id.Manifest, "inputs_digest": id.Inputs, "source_bindings": id.Sources, "source_body_sha256": id.Body, "execution_sha256": digest(exRaw), "replay_sha256": digest(reRaw), "prior_result_sha256": prior, "generated_sha256": ex.Generated, "driver_sha256": ex.Driver, "typed_plan_sha256": ex.TypedPlan, "oracle_origin": originRefs, "oracle_origin_ref": originRef, "result_profile": profileRef, "compiler_build": buildRef, "repository_root": *snapshot, "source_freeze": originRefs.SourceFreeze, "results": ex.Results, "scope": "Only the selected fixed eight of the original sixteen manual witnesses were freshly compared in native execution and saved replay. All nine scalar echoes and fifteen output fields are checked per row against fresh independent Go output. Input-only receipts have no embedded expectations. No full12288 run, generated harness, host authority, network effect, automatic-delivery safety or performance claim. Oracle producer binary is checked while present in owned work; the outer runner owns later cleanup and retention accounting."}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func checkToolchain(goPath, buildPath string) (rawRef, error) {
	if runtime.Version() != goVersion {
		return rawRef{}, fmt.Errorf("observer must run under %s", goVersion)
	}
	expected, err := os.Stat(filepath.Join(runtime.GOROOT(), "bin/go"))
	if err != nil {
		return rawRef{}, err
	}
	current, err := os.Stat(goPath)
	if err != nil || !os.SameFile(expected, current) {
		return rawRef{}, fmt.Errorf("explicit Go must be observer GOROOT/bin/go")
	}
	tool, err := currentRef(goPath, 128<<20)
	if err != nil {
		return rawRef{}, err
	}
	raw, err := readLimited(buildPath, 1<<20)
	if err != nil {
		return rawRef{}, err
	}
	v, err := decode(raw)
	if err != nil {
		return rawRef{}, err
	}
	b := object(v)
	if b["schema"] != "gooo/build-identity/v1" || b["version"] != compilerVersion || b["compiler_source_sha"] != compilerRevision || b["vcs_revision"] != compilerRevision || b["vcs_modified"] != "false" || b["source_status"] != "CLEAN_VCS" || b["go_version"] != goVersion || b["native_go_required"] != goVersion {
		return rawRef{}, fmt.Errorf("compiler clean839 identity differs")
	}
	return tool, nil
}

func readInputs(raw []byte, count int) ([]map[string]any, error) {
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	m := object(v)
	if !keys(m, "schema", "inputs") || m["schema"] != "gooo/body-composition-inputs/v1" || len(array(m["inputs"])) != count {
		return nil, fmt.Errorf("input-only row count/shape differs")
	}
	rows := make([]map[string]any, count)
	for i, v := range array(m["inputs"]) {
		item := object(v)
		if len(item) != 9 {
			return nil, fmt.Errorf("input row %d roster", i)
		}
		row := map[string]any{}
		for j := 0; j < 9; j++ {
			key := appPackage + ":" + activityName + ".input" + strconv.Itoa(j)
			value, present := item[key]
			if !present || !scalar(value, fieldTypes[j]) {
				return nil, fmt.Errorf("input row %d port %d kind/int64", i, j)
			}
			row[fieldNames[j]] = value
		}
		rows[i] = row
	}
	return rows, nil
}

func loadOracle(raw, witnessRaw []byte, inputs []map[string]any) ([]oracleRow, error) {
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	m := object(v)
	w, err := decode(witnessRaw)
	if err != nil {
		return nil, err
	}
	wm := object(w)
	if !keys(m, "schema", "contract_sha256", "cases") || m["schema"] != "gooo/redelivery-independent-manual-oracle/v1" || m["contract_sha256"] != contractSHA || len(array(m["cases"])) != 16 || wm["schema"] != "gooo/redelivery-independent-manual-inputs/v1" || wm["contract_sha256"] != contractSHA || len(array(wm["cases"])) != 16 || len(inputs) != 16 {
		return nil, fmt.Errorf("fresh oracle or frozen witness roster")
	}
	rows := make([]oracleRow, 16)
	for i, v := range array(m["cases"]) {
		r := object(v)
		wr := object(array(wm["cases"])[i])
		ins := object(r["inputs"])
		expected := object(r["expected"])
		if !keys(r, "index", "name", "inputs", "expected") || !isInt(r["index"], i) || r["name"] != caseNames[i] || !isInt(wr["index"], i) || wr["name"] != caseNames[i] || !reflect.DeepEqual(ins, inputs[i]) || !reflect.DeepEqual(ins, object(wr["inputs"])) {
			return nil, fmt.Errorf("oracle original witness interval/order/input %d", i)
		}
		if err := validateRecord(expected, ins); err != nil {
			return nil, fmt.Errorf("oracle row %d: %w", i, err)
		}
		rows[i] = oracleRow{Index: i, Name: caseNames[i], Inputs: ins, Expected: expected}
	}
	return rows, nil
}

func validateRecord(record, inputs map[string]any) error {
	if len(record) != 15 || len(inputs) != 9 {
		return fmt.Errorf("record or echo roster")
	}
	for i, name := range fieldNames {
		v, present := record[name]
		if !present || !scalar(v, fieldTypes[i]) {
			return fmt.Errorf("record field %s kind/int64", name)
		}
		if i < 9 && !reflect.DeepEqual(v, inputs[name]) {
			return fmt.Errorf("record input echo %s", name)
		}
	}
	return nil
}

func snapshotIdentity(root string, inputs []byte, goTool string) (identity, error) {
	raw, err := readBound(filepath.Join(root, manifestName), manifestSHA)
	if err != nil {
		return identity{}, err
	}
	v, err := decode(raw)
	if err != nil {
		return identity{}, err
	}
	m := object(v)
	packages := array(m["packages"])
	entry := object(m["entry"])
	if !keys(m, "schema", "entry", "packages") || m["schema"] != "gooo/package-workspace-manifest/v1" || !keys(entry, "package_path", "activity") || entry["package_path"] != appPackage || entry["activity"] != activityName || len(packages) != 1 {
		return identity{}, fmt.Errorf("manifest entry/package contract")
	}
	pkg := object(packages[0])
	if !keys(pkg, "path", "sources") || pkg["path"] != appPackage || !equal(pkg["sources"], []string{sourceName}) {
		return identity{}, fmt.Errorf("manifest one-source roster")
	}
	source, err := readBound(filepath.Join(root, sourceName), candidateSHA)
	if err != nil {
		return identity{}, err
	}
	body := regexp.MustCompile("(?s)activity\\s+AssessRedelivery[^`]* computes `(.*?)`").FindSubmatch(source)
	if len(body) != 2 || len(body[1]) == 0 {
		return identity{}, fmt.Errorf("frozen candidate computes body missing")
	}
	encoded, err := json.Marshal(string(source))
	if err != nil {
		return identity{}, err
	}
	return identity{Manifest: digest(raw), Inputs: digest(inputs), GoTool: goTool, Body: digest(body[1]), Sources: map[string]sourceIdentity{sourceName: {appPackage, digest(encoded)}}}, nil
}

func readBound(path, want string) ([]byte, error) {
	raw, err := readLimited(path, 1<<20)
	if err != nil {
		return nil, err
	}
	if digest(raw) != "sha256:"+want {
		return nil, fmt.Errorf("frozen source hash differs: %s", path)
	}
	return raw, nil
}
func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, fmt.Errorf("file kind/size: %s", path)
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("file read bound: %s", path)
	}
	return raw, nil
}
func decode(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > 8<<20 || !utf8.Valid(raw) {
		return nil, fmt.Errorf("JSON size/UTF8")
	}
	limits := json.NewDecoder(bytes.NewReader(raw))
	limits.UseNumber()
	if err := pinnedJSONDepth(limits, 0); err != nil {
		return nil, err
	}
	if err := validEscapedUnicode(raw); err != nil {
		return nil, err
	}
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
func validEscapedUnicode(raw []byte) error {
	// encoding/json replaces unpaired UTF16 surrogates. Refuse before decoding.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		i++
		for i < len(raw) && raw[i] != '"' {
			if raw[i] != '\\' {
				i++
				continue
			}
			i++
			if i >= len(raw) {
				break
			}
			if raw[i] != 'u' {
				i++
				continue
			}
			if i+4 >= len(raw) {
				return fmt.Errorf("invalid Unicode escape")
			}
			n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
			if err != nil {
				return err
			}
			i += 5
			if n >= 0xD800 && n <= 0xDBFF {
				if i+5 >= len(raw) || raw[i] != '\\' || raw[i+1] != 'u' {
					return fmt.Errorf("unpaired high surrogate")
				}
				low, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
				if err != nil || low < 0xDC00 || low > 0xDFFF {
					return fmt.Errorf("invalid surrogate pair")
				}
				i += 6
			} else if n >= 0xDC00 && n <= 0xDFFF {
				return fmt.Errorf("unpaired low surrogate")
			}
		}
	}
	return nil
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func str(v any) string            { s, _ := v.(string); return s }
func isInt(v any, want int) bool {
	n, ok := v.(json.Number)
	return ok && n.String() == strconv.Itoa(want)
}
func scalar(v any, kind string) bool {
	switch kind {
	case "string":
		s, ok := v.(string)
		return ok && utf8.ValidString(s) && len(s) <= 1024
	case "bool":
		_, ok := v.(bool)
		return ok
	case "int64":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		x, err := strconv.ParseInt(n.String(), 10, 64)
		return err == nil && strconv.FormatInt(x, 10) == n.String()
	}
	return false
}
func keys(m map[string]any, names ...string) bool {
	if len(m) != len(names) {
		return false
	}
	for _, n := range names {
		if _, ok := m[n]; !ok {
			return false
		}
	}
	return true
}
func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func validDigest(v any) bool {
	s := str(v)
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && s == strings.ToLower(s)
}
func equal(v, want any) bool {
	raw, err := json.Marshal(want)
	if err != nil {
		return false
	}
	decoded, err := decode(raw)
	return err == nil && reflect.DeepEqual(v, decoded)
}
func runtimePlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
