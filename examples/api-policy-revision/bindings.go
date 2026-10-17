package revisionpolicy

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"reflect"
	"strings"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

const CompilerPin = "83928fa4538245abe33b08549da47f8cbdc50ed2"
const originalGeneration = "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46"

// Hashes establish unsigned consistency inside a trusted executable. They are
// not signatures, current authorization, or evidence that supplied facts are true.
// The separately compiled original module embeds its own 28 assets. Go cannot
// embed across its nested go.mod: generate.py checks the frozen distribution,
// including its go.mod and licenses; this runtime performs no physical-file reads.
//
//go:embed canonical receipts internal readonly.gooo compare.gooo candidate.workspace.json differences.workspace.json summary.workspace.json profiles.json compiler.json adapter_generated.go model.go compare.go input.go bindings.go go.mod go.sum cmd/compare/main.go cmd/compare/diagnostics.go generate.py generation.json testdata/classifier-expected.json
var revisionAssets embed.FS

var revisionNames = []string{
	"testdata/classifier-expected.json", "readonly.gooo", "compare.gooo", "candidate.workspace.json", "differences.workspace.json", "summary.workspace.json", "profiles.json", "compiler.json", "adapter_generated.go", "model.go", "compare.go", "input.go", "bindings.go", "go.mod", "go.sum", "cmd/compare/main.go", "cmd/compare/diagnostics.go", "generate.py",
	"canonical/main.gooo", "canonical/scalar_access.gooo",
	"receipts/candidate-seed-inputs.json", "receipts/candidate-execution.json", "receipts/candidate-replay.json", "receipts/candidate-lowered.gooo", "receipts/candidate-body-codegen.json", "internal/candidate/generated.go",
	"receipts/flags-seed-inputs.json", "receipts/flags-execution.json", "receipts/flags-replay.json", "receipts/flags-lowered.gooo", "receipts/flags-body-codegen.json", "internal/flags/generated.go",
	"receipts/summary-seed-inputs.json", "receipts/summary-execution.json", "receipts/summary-replay.json", "receipts/summary-lowered.gooo", "receipts/summary-body-codegen.json", "internal/summary/generated.go",
}

type revisionConfig struct{ prefix, workspace, pkg, activity, id, source, input, output string }

var revisionConfigs = []revisionConfig{
	{"candidate", "candidate.workspace.json", "app/api-policy-readonly", "ReadOnlyAPI", "gooo://api-revision/readonly/v1", "readonly.gooo", "(bool,string,bool,string,bool,string,string,string,string)", "ApiDecision"},
	{"flags", "differences.workspace.json", "gooo/api-policy-revision/classifier", "DifferenceFlags", "gooo://api-policy-revision/difference-flags", "compare.gooo", "(ApiDecision,ApiDecision)", "ApiDifferenceFlags"},
	{"summary", "summary.workspace.json", "gooo/api-policy-revision/classifier", "ChangeSummary", "gooo://api-policy-revision/change-summary", "compare.gooo", "(ApiDecision,ApiDecision)", "ApiChangeSummary"},
}

// RevisionBindings validates the full embedded roster before native consumption.
func RevisionBindings() (original.Bindings, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(revisionAssets, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || name == "generation.json" {
			return nil
		}
		raw, err := revisionAssets.ReadFile(name)
		if err == nil {
			files[name] = raw
		}
		return err
	})
	if err != nil {
		return original.Bindings{}, err
	}
	manifest, err := revisionAssets.ReadFile("generation.json")
	if err != nil {
		return original.Bindings{}, err
	}
	return checkRevisionBindings(files, manifest)
}

func checkRevisionBindings(files map[string][]byte, manifest []byte) (original.Bindings, error) {
	invalid := errors.New("API revision source bindings invalid")
	var generation struct {
		Schema   string            `json:"schema"`
		Compiler string            `json:"compiler_source_sha"`
		Assets   map[string]string `json:"assets"`
	}
	if json.Unmarshal(manifest, &generation) != nil || generation.Schema != "gooo/api-policy-revision-generation/v1" || generation.Compiler != CompilerPin {
		return original.Bindings{}, invalid
	}
	names := append([]string{}, revisionNames...)
	if len(files) != len(names) || len(generation.Assets) != len(names) {
		return original.Bindings{}, invalid
	}
	for _, name := range names {
		if len(files[name]) == 0 || generation.Assets[name] != revisionDigest(files[name]) {
			return original.Bindings{}, invalid
		}
	}
	compiler, err := revisionJSON(files["compiler.json"])
	if err != nil || compiler["schema"] != "gooo/build-identity/v1" || compiler["compiler_source_sha"] != CompilerPin || compiler["vcs_revision"] != CompilerPin || compiler["source_status"] != "CLEAN_VCS" || compiler["vcs_modified"] != "false" || compiler["go_version"] != "go1.27.2" || compiler["native_go_required"] != "go1.27.2" {
		return original.Bindings{}, invalid
	}
	runtime := revisionObject(compiler["decision_runtime"])
	if runtime["module_path"] != "github.com/kimjooyoon/gooo-decision-runtime" || runtime["version"] != "v0.2.41-experimental" || runtime["replaced"] != false {
		return original.Bindings{}, invalid
	}
	profiles, err := revisionJSON(files["profiles.json"])
	if err != nil || profiles["schema"] != "gooo/api-policy-two-profiles/v1" {
		return original.Bindings{}, invalid
	}
	before, candidate := revisionObject(profiles["original"]), revisionObject(profiles["candidate"])
	if before["profile_id"] != OriginalProfile || before["module"] != "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy" || before["source_commit"] != "8cddcc4e65afb42431f8f42a86f29859ae2f4d69" || before["generation_sha256"] != originalGeneration || candidate["profile_id"] != CandidateProfile || candidate["compiler_source_sha"] != CompilerPin || candidate["generation_file"] != "generation.json" || candidate["source_commit"] != "defined_by_this_generation" {
		return original.Bindings{}, invalid
	}
	frozen, err := original.SourceBindings()
	if err != nil || frozen.CompilerSourceSHA != CompilerPin || frozen.GenerationSHA256 != originalGeneration || len(frozen.Assets) != 28 {
		return original.Bindings{}, invalid
	}
	frozenMap := revisionObject(before["assets"])
	if len(frozenMap) != 28 {
		return original.Bindings{}, invalid
	}
	for name, digest := range frozen.Assets {
		if frozenMap[name] != digest {
			return original.Bindings{}, invalid
		}
	}
	packaging := revisionObject(before["packaging_assets"])
	if len(packaging) != 4 || packaging["go.mod"] != "sha256:7ec289b29bda8fa28b1004af40489834f6173cb511931f4fa8c4f5e23fcdc6e3" || packaging["LICENSE-APACHE-2.0"] != "sha256:4cdb9af102dfbb0ca03d87d6f650a505df098646a4080f4665b389ad9c6caa02" || packaging["NOTICE.md"] != "sha256:61ae61f840ce0a0ff98ea79c94368bb3666207bfff6c66c361bb1102a11f59aa" || packaging["oracle/upstream/LICENSE"] != "sha256:cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30" {
		return original.Bindings{}, invalid
	}
	if revisionDigest(files["canonical/main.gooo"]) != frozen.Assets["canonical/main.gooo"] {
		return original.Bindings{}, invalid
	}
	if revisionDigest(files["canonical/scalar_access.gooo"]) != frozen.Assets["canonical/scalar_access.gooo"] {
		return original.Bindings{}, invalid
	}
	for _, config := range revisionConfigs {
		if !checkRevisionProgram(files, config) {
			return original.Bindings{}, invalid
		}
	}
	return original.Bindings{CompilerSourceSHA: CompilerPin, Assets: generation.Assets, GenerationSHA256: revisionDigest(manifest)}, nil
}

func checkRevisionProgram(files map[string][]byte, c revisionConfig) bool {
	base := "receipts/" + c.prefix + "-"
	execution, err := revisionJSON(files[base+"execution.json"])
	if err != nil {
		return false
	}
	replay, err := revisionJSON(files[base+"replay.json"])
	if err != nil {
		return false
	}
	replayInfo := revisionObject(revisionObject(replay["result"])["replay"])
	if replay["replayed_from_sha256"] != revisionDigest(files[base+"execution.json"]) || replayInfo["schema"] != "gooo/workspace-body-replay/v1" || revisionInteger(replayInfo["model_calls"]) != 0 {
		return false
	}
	p := revisionObject(revisionObject(execution["result"])["program"])
	if len(p) == 0 || !reflect.DeepEqual(p, revisionObject(revisionObject(replay["result"])["program"])) {
		return false
	}
	lowered, ok := p["lowered_gooo_source"].(string)
	if !ok || lowered == "" || !bytes.Equal([]byte(lowered), files[base+"lowered.gooo"]) || p["schema"] != "gooo/workspace-body-program/v1" {
		return false
	}
	entry := revisionObject(p["entry"])
	loweredName, ok := entry["lowered_name"].(string)
	if !ok || loweredName == "" || entry["package_path"] != c.pkg || entry["activity"] != c.activity {
		return false
	}
	for _, receipt := range []map[string]any{execution, replay} {
		result := revisionObject(receipt["result"])
		if receipt["schema"] != "gooo/workspace-body-execution-receipt/v1" || (receipt["decision"] != "OBSERVED" && receipt["decision"] != "PASS") || receipt["manifest_digest"] != revisionDigest(files[c.workspace]) || receipt["inputs_digest"] != revisionDigest(files[base+"seed-inputs.json"]) || result["schema"] != "gooo/workspace-body-execution/v1" || result["execution_source_sha256"] != revisionDigest([]byte(lowered)) || !checkRevisionRuntime(revisionObject(result["runtime"]), lowered) {
			return false
		}
		composition := revisionObject(result["composition"])
		runtime := revisionObject(result["runtime"])
		generated, sourceOK := composition["source"].(string)
		driver, driverOK := composition["driver_source"].(string)
		if !sourceOK || generated == "" || !driverOK || driver == "" || composition["schema"] != "gooo/body-composition/v1" || composition["stage"] != "COMPLETE" || composition["gooo_source"] != lowered || composition["original_source_sha256"] != revisionDigest([]byte(lowered)) || composition["selected_source_sha256"] != revisionDigest([]byte(lowered)) || composition["generated_sha256"] != revisionDigest([]byte(generated)) || composition["driver_sha256"] != revisionDigest([]byte(driver)) || runtime["generated_sha256"] != composition["generated_sha256"] || runtime["driver_sha256"] != composition["driver_sha256"] {
			return false
		}
	}
	if !checkRevisionImage(revisionObject(p["workspace_image"]), files, c) {
		return false
	}
	body, err := revisionJSON(files[base+"body-codegen.json"])
	if err != nil {
		return false
	}
	source, ok := body["source"].(string)
	report := revisionObject(body["report"])
	if !ok || source == "" || report["schema"] != "gooo/body-codegen-report/v3" || report["decision"] != "PASS" || report["activity"] != loweredName || report["activity_id"] != c.id || report["input_type"] != c.input || report["output_type"] != c.output || report["compiler_source_sha"] != CompilerPin || report["source_digest"] != revisionDigest([]byte(lowered)) || report["generated_digest"] != revisionDigest([]byte(source)) || report["replay_digest"] != report["generated_digest"] || report["typecheck_passed"] != true || report["deterministic_replay"] != true || revisionInteger(report["repository_writes"]) != 0 || !revisionNoErrors(report) || !checkRevisionRecords(report, c) {
		return false
	}
	projection, err := format.Source([]byte("// Code generated from compiler-owned workspace lowering; DO NOT EDIT.\n" + source))
	return err == nil && bytes.Equal(projection, files["internal/"+c.prefix+"/generated.go"])
}

func checkRevisionRuntime(r map[string]any, lowered string) bool {
	version, ok := r["go_version"].(string)
	module := revisionObject(r["producer_module"])
	if !ok || !strings.HasPrefix(version, "go version go1.27.2 ") || r["schema"] != "gooo/body-composition-runtime/v1" || r["stage"] != "COMPLETE" || r["producer_source_sha"] != CompilerPin || module["path"] != "github.com/kimjooyoon/meta-ontology-go" || module["version"] != "v0.0.0-20261011201109-83928fa45382" || r["projection_replayed"] != true || r["runtime_replayed"] != true || r["original_source_sha256"] != revisionDigest([]byte(lowered)) || r["selected_source_sha256"] != revisionDigest([]byte(lowered)) || revisionInteger(r["model_calls"]) != 0 || revisionInteger(r["elapsed_ns"]) < 0 {
		return false
	}
	if !checkRevisionProcess(revisionObject(r["toolchain"])) || !checkRevisionProcess(revisionObject(r["build"])) {
		return false
	}
	passed, total := revisionInteger(r["finite_passed"]), revisionInteger(r["finite_total"])
	if passed < 0 || total < 0 || passed > total {
		return false
	}
	runs := revisionArray(r["runs"])
	if len(runs) != 2 {
		return false
	}
	for _, raw := range runs {
		run := revisionObject(raw)
		if !checkRevisionProcess(run) || revisionInteger(revisionObject(run["timing"])["wait_limit_ns"]) < 0 {
			return false
		}
	}
	return revisionObject(runs[0])["stdout_sha256"] == revisionObject(runs[1])["stdout_sha256"] && revisionObject(runs[0])["stderr_sha256"] == revisionObject(runs[1])["stderr_sha256"]
}

func checkRevisionProcess(p map[string]any) bool {
	if p["started"] != true || p["completed"] != true || p["canceled"] != false || p["timed_out"] != false || p["output_truncated"] != false || revisionInteger(p["exit_code"]) != 0 || revisionInteger(p["diagnostics_bytes"]) != 0 {
		return false
	}
	for _, name := range []string{"wall_ns", "user_ns", "system_ns", "peak_rss_bytes"} {
		if revisionInteger(p[name]) < 0 {
			return false
		}
	}
	timing := revisionObject(p["timing"])
	for _, name := range []string{"start_ns", "wait_ns", "deadline_remaining_ns"} {
		if revisionInteger(timing[name]) < 0 {
			return false
		}
	}
	for _, name := range []string{"stdout_sha256", "stderr_sha256"} {
		if !revisionSHA(p[name]) {
			return false
		}
	}
	return timing["context_at_start_return"] == "ACTIVE"
}

func checkRevisionImage(image map[string]any, files map[string][]byte, c revisionConfig) bool {
	inputNames := []string{"ApiDecision", "ApiDecision"}
	if c.prefix == "candidate" {
		inputNames = []string{"Boolean", "Text", "Boolean", "Text", "Boolean", "Text", "Text", "Text", "Text"}
	}
	entry := revisionObject(image["entry"])
	if image["schema"] != "gooo/package-runtime-image/v1" || entry["package_path"] != c.pkg || entry["source"] != c.source || entry["activity"] != c.activity || entry["output"] != c.output || !revisionStrings(entry["inputs"], inputNames) {
		return false
	}
	expectedSources := map[string]string{"go/cel.dev/cel-go/codelab": "canonical/scalar_access.gooo", "app/cel-api-policy": "canonical/main.gooo", c.pkg: c.source}
	packages, seen := revisionArray(image["packages"]), map[string]bool{}
	if len(packages) != 3 {
		return false
	}
	for _, raw := range packages {
		pkg := revisionObject(raw)
		path, ok := pkg["path"].(string)
		filename, exists := expectedSources[path]
		if !ok || !exists || seen[path] {
			return false
		}
		seen[path] = true
		sources := revisionArray(pkg["sources"])
		if len(sources) != 1 {
			return false
		}
		source := revisionObject(sources[0])
		encoded, _ := json.Marshal(string(files[filename]))
		if source["filename"] != filename || source["source_digest"] != revisionDigest(encoded) {
			return false
		}
		expected := map[string]string{}
		inputs := []string{"gooo://std/boolean", "gooo://std/text", "gooo://std/boolean", "gooo://std/text"}
		output := "gooo://cel-example/access-decision/v1"
		if path == "go/cel.dev/cel-go/codelab" {
			expected["ScalarAccess"] = "gooo://cel-example/scalar-access"
		} else if path == "app/cel-api-policy" {
			expected["APIAccess"] = "gooo://api-example/api-access"
			inputs = append(inputs, "gooo://std/boolean", "gooo://std/text", "gooo://std/text", "gooo://std/text", "gooo://std/text")
			output = "gooo://api-example/decision/v1"
		} else if c.prefix == "candidate" {
			expected[c.activity] = c.id
			inputs = append(inputs, "gooo://std/boolean", "gooo://std/text", "gooo://std/text", "gooo://std/text", "gooo://std/text")
			output = "gooo://api-example/decision/v1"
		} else {
			expected["DifferenceFlags"] = "gooo://api-policy-revision/difference-flags"
			expected["ChangeSummary"] = "gooo://api-policy-revision/change-summary"
			inputs = []string{"gooo://api-example/decision/v1", "gooo://api-example/decision/v1"}
		}
		found := map[string]bool{}
		for _, raw := range revisionArray(pkg["exports"]) {
			export := revisionObject(raw)
			if export["kind"] != "activity" {
				continue
			}
			name, ok := export["name"].(string)
			id, exists := expected[name]
			if !ok || !exists || found[name] || export["id"] != id || !revisionStrings(export["input_types"], inputs) {
				return false
			}
			found[name] = true
			if path == c.pkg && c.prefix != "candidate" {
				output = id + "/v1"
			}
			if export["output_type"] != output {
				return false
			}
		}
		if len(found) != len(expected) {
			return false
		}
	}
	return true
}

func checkRevisionRecords(report map[string]any, c revisionConfig) bool {
	names := []string{"ApiDecision", "ApiDifferenceFlags"}
	if c.prefix == "candidate" {
		names = []string{"ApiDecision", "AccessDecision"}
	}
	if c.prefix == "summary" {
		names = append(names, "ApiChangeSummary", "AccessDecision")
	}
	records, seen := revisionArray(report["record_types"]), map[string]bool{}
	if len(records) != len(names) {
		return false
	}
	for _, raw := range records {
		record := revisionObject(raw)
		name, ok := record["name"].(string)
		if !ok || seen[name] || !checkRevisionRecord(record) {
			return false
		}
		seen[name] = true
	}
	for _, name := range names {
		if !seen[name] {
			return false
		}
	}
	parameters := revisionArray(report["input_parameters"])
	inputs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(c.input, "("), ")"), ",")
	if len(parameters) != len(inputs) {
		return false
	}
	for i, input := range inputs {
		if revisionObject(parameters[i])["name"] != fmt.Sprintf("input%d", i) || revisionObject(parameters[i])["type"] != input {
			return false
		}
	}
	return true
}

func checkRevisionRecord(record map[string]any) bool {
	name := record["name"]
	base := "gooo://api-example/decision/"
	fields := []string{"allowed", "reason", "authenticated", "tenant_present", "tenant_matched", "action_present", "action_matched", "access_allowed", "access_error", "access_result", "group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal"}
	text := map[string]bool{"reason": true, "access_result": true, "effective_principal": true}
	if name == "AccessDecision" {
		base = "gooo://cel-example/access-decision/"
		fields = []string{"allowed", "evaluation_error", "result", "group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal"}
		text = map[string]bool{"result": true, "effective_principal": true}
	} else if name == "ApiDifferenceFlags" {
		base = "gooo://api-policy-revision/difference-flags/"
		text = nil
	} else if name == "ApiChangeSummary" {
		base = "gooo://api-policy-revision/change-summary/"
		fields = []string{"classification", "changed_count"}
		text = map[string]bool{"classification": true}
	} else if name != "ApiDecision" {
		return false
	}
	actual := revisionArray(record["fields"])
	if record["id"] != base+"v1" || len(actual) != len(fields) {
		return false
	}
	for i, name := range fields {
		field := revisionObject(actual[i])
		typ := "urn:gooo:type:boolean"
		if text[name] {
			typ = "urn:gooo:type:string"
		}
		if name == "changed_count" {
			typ = "urn:gooo:type:integer"
		}
		if field["name"] != name || field["id"] != base+strings.ReplaceAll(name, "_", "-") || field["type_id"] != typ {
			return false
		}
	}
	return true
}

func revisionJSON(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("trailing JSON data")
	}
	return value, nil
}
func revisionObject(value any) map[string]any { result, _ := value.(map[string]any); return result }
func revisionArray(value any) []any           { result, _ := value.([]any); return result }
func revisionNoErrors(report map[string]any) bool {
	value, ok := report["errors"]
	if !ok {
		// The pinned v3 Report has no errors member; success is explicit above.
		return true
	}
	if value == nil {
		return true
	}
	list, ok := value.([]any)
	return ok && len(list) == 0
}
func revisionInteger(value any) int64 {
	number, ok := value.(json.Number)
	if !ok {
		return -1
	}
	result, err := number.Int64()
	if err != nil {
		return -1
	}
	return result
}
func revisionStrings(value any, expected []string) bool {
	actual := revisionArray(value)
	if len(actual) != len(expected) {
		return false
	}
	for i, item := range expected {
		if actual[i] != item {
			return false
		}
	}
	return true
}
func revisionDigest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func revisionSHA(value any) bool {
	text, ok := value.(string)
	if !ok || len(text) != 71 || !strings.HasPrefix(text, "sha256:") {
		return false
	}
	for _, c := range text[7:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
