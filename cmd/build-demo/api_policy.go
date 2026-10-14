package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	apiPolicyModule             = "examples/api-policy-revision"
	apiPolicyOriginalModule     = "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
	apiPolicyRevisionModule     = "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	apiPolicyOriginalVersion    = "v0.0.0-20261014100709-8cddcc4e65af"
	apiPolicyOriginalGeneration = "79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46"
	apiPolicyRevisionGeneration = "5ce3d883db76b9b2cca6f53602eff6b7b3dff1870b3e19a882281dcff91bdda6"
	apiPolicyCompilerRevision   = "83928fa4538245abe33b08549da47f8cbdc50ed2"
)

var apiPolicyOriginalPackaging = map[string]string{
	"go.mod":                  "sha256:7ec289b29bda8fa28b1004af40489834f6173cb511931f4fa8c4f5e23fcdc6e3",
	"NOTICE.md":               "sha256:61ae61f840ce0a0ff98ea79c94368bb3666207bfff6c66c361bb1102a11f59aa",
	"LICENSE-APACHE-2.0":      "sha256:4cdb9af102dfbb0ca03d87d6f650a505df098646a4080f4665b389ad9c6caa02",
	"oracle/upstream/LICENSE": "sha256:cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30",
}

type apiPolicyFile struct {
	Path   string `json:"path,omitempty"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode,omitempty"`
}

type apiPolicyBindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	GenerationSHA256  string            `json:"generation_sha256"`
	Assets            map[string]string `json:"assets"`
}

type apiPolicySource struct {
	Original apiPolicyBindings
	Revision apiPolicyBindings
	Files    map[string]apiPolicyFile
}

func apiPolicyDescribe(raw []byte) apiPolicyFile {
	return apiPolicyFile{Bytes: len(raw), SHA256: digest(raw)}
}

func apiPolicyLocalPath(name string) bool {
	return name != "." && utf8.ValidString(name) && filepath.IsLocal(name) && filepath.ToSlash(filepath.Clean(name)) == name && !strings.ContainsAny(name, "\\:\x00")
}

func apiPolicyDecode(raw []byte, target any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("API policy JSON is not exact UTF-8")
	}
	if _, err := cropDecode(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing API policy JSON")
	}
	return nil
}

// The existing CLI promises json.Marshal followed by one LF. Never TrimSpace:
// the raw execution stdout and the bridge's canonical JSON have distinct hashes.
func apiPolicyCanonicalCLI(raw []byte) ([]byte, error) {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || !utf8.Valid(raw) {
		return nil, fmt.Errorf("native API CLI output must end in exactly one LF")
	}
	canonical := raw[:len(raw)-1]
	if _, err := cropDecode(canonical); err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if json.Compact(&compact, canonical) != nil || !bytes.Equal(compact.Bytes(), canonical) {
		return nil, fmt.Errorf("native API CLI body is not compact JSON")
	}
	return bytes.Clone(canonical), nil
}

// Inspect only paths beneath the caller's canonical repository root. No
// symlink is traversed inside that root, including a same-byte source alias.
func apiPolicyReadFile(root, name string, exactSourceMode bool) ([]byte, error) {
	if !apiPolicyLocalPath(name) {
		return nil, fmt.Errorf("non-local API policy path: %s", name)
	}
	parts := strings.Split(name, "/")
	path := root
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("symlink/non-directory API source path: %s", name)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() || exactSourceMode && info.Mode().Perm() != 0644 {
				return nil, fmt.Errorf("non-regular or changed API source mode: %s", name)
			}
			if info.Size() > 4<<20 {
				return nil, fmt.Errorf("API source exceeds inspection bound: %s", name)
			}
		}
	}
	return os.ReadFile(path)
}

func apiPolicyReadDocuments() (map[string][]byte, error) {
	root, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	return apiPolicyReadDocumentsAt(root)
}

// root is the repository root, not the nested module. Resolving its ambient
// OS alias (e.g. macOS /var) does not permit aliases inside the source tree.
func apiPolicyReadDocumentsAt(root string) (map[string][]byte, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	documents := map[string][]byte{}
	module := filepath.Join(root, apiPolicyModule)
	err = filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("API source symlink: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		raw, err := apiPolicyReadFile(root, name, true)
		if err != nil {
			return err
		}
		documents[name] = raw
		return nil
	})
	return documents, err
}

func apiPolicyCheckDocuments(documents map[string][]byte, lock lockfile) (apiPolicySource, error) {
	bad := func(message string) (apiPolicySource, error) {
		return apiPolicySource{}, fmt.Errorf("API policy source refused: %s", message)
	}
	if lock.CompilerRevision != apiPolicyCompilerRevision || lock.CompilerVersion != "0.6.32-dev" || lock.GoVersion != "go1.27.2" || len(documents) != 78 {
		return bad("pinned toolchain or complete source roster changed")
	}
	files := map[string]apiPolicyFile{}
	for name, raw := range documents {
		if !apiPolicyLocalPath(name) || !strings.HasPrefix(name, apiPolicyModule+"/") || len(raw) == 0 {
			return bad("non-local, empty or unrelated source")
		}
		file := apiPolicyDescribe(raw)
		file.Mode = "100644"
		files[name] = file
	}
	var revision struct {
		Schema            string            `json:"schema"`
		Assets            map[string]string `json:"assets"`
		CompilerSourceSHA string            `json:"compiler_source_sha"`
	}
	var original struct {
		Schema string            `json:"schema"`
		Assets map[string]string `json:"assets"`
	}
	revisionRaw := documents[apiPolicyModule+"/generation.json"]
	originalRoot := apiPolicyModule + "/profiles/original/"
	originalRaw := documents[originalRoot+"generation.json"]
	if apiPolicyDecode(revisionRaw, &revision) != nil || digest(revisionRaw) != apiPolicyRevisionGeneration || revision.Schema != "gooo/api-policy-revision-generation/v1" || revision.CompilerSourceSHA != apiPolicyCompilerRevision || len(revision.Assets) != 36 {
		return bad("revision36/raw generation changed")
	}
	if apiPolicyDecode(originalRaw, &original) != nil || digest(originalRaw) != apiPolicyOriginalGeneration || original.Schema != "gooo/cel-public-generation/v1" || len(original.Assets) != 28 {
		return bad("original28/raw generation changed")
	}
	for prefix, assets := range map[string]map[string]string{apiPolicyModule + "/": revision.Assets, originalRoot: original.Assets} {
		for name, expected := range assets {
			if !apiPolicyLocalPath(name) || len(documents[prefix+name]) == 0 || expected != "sha256:"+digest(documents[prefix+name]) {
				return bad("sealed source bytes changed: " + prefix + name)
			}
		}
	}
	var profiles struct {
		Schema    string `json:"schema"`
		Candidate struct {
			Compiler   string `json:"compiler_source_sha"`
			Generation string `json:"generation_file"`
			ID         string `json:"profile_id"`
			Commit     string `json:"source_commit"`
		} `json:"candidate"`
		Original struct {
			Assets     map[string]string `json:"assets"`
			Generation string            `json:"generation_sha256"`
			Module     string            `json:"module"`
			Packaging  map[string]string `json:"packaging_assets"`
			ID         string            `json:"profile_id"`
			Commit     string            `json:"source_commit"`
		} `json:"original"`
	}
	if apiPolicyDecode(documents[apiPolicyModule+"/profiles.json"], &profiles) != nil || profiles.Schema != "gooo/api-policy-two-profiles/v1" || profiles.Candidate.Compiler != apiPolicyCompilerRevision || profiles.Candidate.Generation != "generation.json" || profiles.Candidate.ID != "cel-api-readonly-v1" || profiles.Candidate.Commit != "defined_by_this_generation" || profiles.Original.Generation != "sha256:"+apiPolicyOriginalGeneration || profiles.Original.Module != apiPolicyOriginalModule || profiles.Original.ID != "cel-api-original-v1" || profiles.Original.Commit != "8cddcc4e65afb42431f8f42a86f29859ae2f4d69" || !reflect.DeepEqual(profiles.Original.Assets, original.Assets) || !reflect.DeepEqual(profiles.Original.Packaging, apiPolicyOriginalPackaging) {
		return bad("two-profile/frozen packaging contract changed")
	}
	frozen := map[string]bool{"generation.json": true}
	for name := range original.Assets {
		frozen[name] = true
	}
	for name, expected := range apiPolicyOriginalPackaging {
		if expected != "sha256:"+digest(documents[originalRoot+name]) {
			return bad("original packaging byte changed: " + name)
		}
		frozen[name] = true
	}
	frozenCount := 0
	for name := range documents {
		if strings.HasPrefix(name, originalRoot) {
			frozenCount++
			if !frozen[strings.TrimPrefix(name, originalRoot)] {
				return bad("extra frozen original file")
			}
		} else if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			if _, exists := revision.Assets[strings.TrimPrefix(name, apiPolicyModule+"/")]; !exists {
				return bad("unsealed compiled Go source")
			}
		}
	}
	if frozenCount != 33 || len(frozen) != 33 {
		return bad("frozen original file roster changed")
	}
	for _, prefix := range []string{apiPolicyModule + "/", originalRoot} {
		var compiler map[string]any
		if apiPolicyDecode(documents[prefix+"compiler.json"], &compiler) != nil || compiler["schema"] != "gooo/build-identity/v1" || compiler["compiler_source_sha"] != lock.CompilerRevision || compiler["vcs_revision"] != lock.CompilerRevision || compiler["source_status"] != "CLEAN_VCS" || compiler["vcs_modified"] != "false" || compiler["version"] != lock.CompilerVersion || compiler["go_version"] != lock.GoVersion || compiler["native_go_required"] != lock.GoVersion || compiler["build_info_observed"] != true {
			return bad("sealed compiler identity changed")
		}
	}
	for _, name := range []string{"main.gooo", "scalar_access.gooo"} {
		if !bytes.Equal(documents[apiPolicyModule+"/canonical/"+name], documents[originalRoot+"canonical/"+name]) {
			return bad("canonical original source changed")
		}
	}
	return apiPolicySource{Original: apiPolicyBindings{lock.CompilerRevision, "sha256:" + apiPolicyOriginalGeneration, original.Assets}, Revision: apiPolicyBindings{lock.CompilerRevision, "sha256:" + apiPolicyRevisionGeneration, revision.Assets}, Files: files}, nil
}

type apiPolicyCheckout struct {
	Head  string `json:"head"`
	Tree  string `json:"tree"`
	Dirty bool   `json:"dirty"`
}

func apiPolicyObserveCheckout(ctx context.Context, root string) (apiPolicyCheckout, error) {
	var result apiPolicyCheckout
	outputs := make([][]byte, 3)
	for i, args := range [][]string{{"rev-parse", "HEAD"}, {"rev-parse", "HEAD^{tree}"}, {"status", "--porcelain=v1", "--untracked-files=normal"}} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		raw, err := cmd.Output()
		if err != nil {
			return result, fmt.Errorf("read current API build checkout: %w", err)
		}
		outputs[i] = raw
	}
	result.Head, result.Tree = strings.TrimSpace(string(outputs[0])), strings.TrimSpace(string(outputs[1]))
	if len(result.Head) != 40 || len(result.Tree) != 40 {
		return result, fmt.Errorf("invalid current Git identity")
	}
	result.Dirty = len(outputs[2]) != 0
	return result, nil
}

type apiPolicyPrivateProcess struct {
	Name        string            `json:"name"`
	Argv        []string          `json:"argv"`
	CWD         string            `json:"cwd"`
	Started     string            `json:"started_at_utc"`
	ElapsedMS   int64             `json:"elapsed_ms"`
	ExitCode    int               `json:"exit_code"`
	TimedOut    bool              `json:"timed_out"`
	Environment map[string]string `json:"environment"`
	Stdin       apiPolicyFile     `json:"stdin"`
	Stdout      apiPolicyFile     `json:"stdout"`
	Stderr      apiPolicyFile     `json:"stderr"`
}

func apiPolicyRecordedEnvironment(environment []string) map[string]string {
	allowed := map[string]bool{}
	for _, key := range strings.Fields("PATH HOME TMPDIR GOTMPDIR GOPATH GOMODCACHE GOCACHE GOTOOLCHAIN GOWORK GOENV GOPROXY GOSUMDB GOVCS GOTELEMETRY CGO_ENABLED GOFLAGS GOEXPERIMENT GOOS GOARCH") {
		allowed[key] = true
	}
	result := map[string]string{}
	for _, value := range environment {
		key, text, ok := strings.Cut(value, "=")
		if ok && allowed[key] {
			result[key] = text
		}
	}
	return result
}

type apiPolicyRunner struct {
	Evidence string
	Env      []string
	Steps    []apiPolicyPrivateProcess
}

func (runner *apiPolicyRunner) run(ctx context.Context, name, cwd string, input []byte, extra []string, executable string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir, command.Env = cwd, append(append([]string{}, runner.Env...), extra...)
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = 2 * time.Second
	stdoutPath, stderrPath := filepath.Join(runner.Evidence, name+".stdout"), filepath.Join(runner.Evidence, name+".stderr")
	stdout, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, nil, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(stderrPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, nil, err
	}
	defer stderr.Close()
	if err := os.WriteFile(filepath.Join(runner.Evidence, name+".stdin"), input, 0600); err != nil {
		return nil, nil, err
	}
	command.Stdout, command.Stderr = stdout, stderr
	started := time.Now()
	runErr := command.Run()
	out, readErr := os.ReadFile(stdoutPath)
	errout, stderrErr := os.ReadFile(stderrPath)
	step := apiPolicyPrivateProcess{Name: name, Argv: append([]string{executable}, args...), CWD: cwd, Started: started.UTC().Format(time.RFC3339Nano), ElapsedMS: time.Since(started).Milliseconds(), ExitCode: -1, TimedOut: ctx.Err() != nil, Environment: apiPolicyRecordedEnvironment(command.Env), Stdin: apiPolicyDescribe(input), Stdout: apiPolicyDescribe(out), Stderr: apiPolicyDescribe(errout)}
	if command.ProcessState != nil {
		step.ExitCode = command.ProcessState.ExitCode()
	}
	runner.Steps = append(runner.Steps, step)
	if runErr != nil || readErr != nil || stderrErr != nil || step.ExitCode != 0 || step.TimedOut {
		return out, errout, fmt.Errorf("API policy %s failed (exit %d): %w", name, step.ExitCode, apiPolicyFirstError(runErr, readErr, stderrErr, ctx.Err()))
	}
	return out, errout, nil
}

// Avoid putting detailed host command errors into any public record.
func apiPolicyFirstError(values ...error) error {
	for _, err := range values {
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("unexpected process state")
}

type apiPolicyFacts struct {
	GroupPresent     bool   `json:"group_present"`
	GroupValue       string `json:"group_value"`
	PrincipalPresent bool   `json:"principal_present"`
	PrincipalValue   string `json:"principal_value"`
	Authenticated    bool   `json:"authenticated"`
	CallerTenant     string `json:"caller_tenant"`
	ResourceTenant   string `json:"resource_tenant"`
	RequestedAction  string `json:"requested_action"`
	AllowedAction    string `json:"allowed_action"`
}

func apiPolicyLiteralFacts(action string) apiPolicyFacts {
	return apiPolicyFacts{true, "admin", true, "synthetic:user", true, "tenant-a", "tenant-a", action, action}
}

func apiPolicyLiteralReport(action string, source apiPolicySource) map[string]any {
	decision := func() map[string]any {
		return map[string]any{"allowed": true, "reason": "ALLOW", "authenticated": true, "tenant_present": true, "tenant_matched": true, "action_present": true, "action_matched": true, "access_allowed": true, "access_error": false, "access_result": "ALLOW", "group_matched": true, "principal_matched": false, "group_missing": false, "principal_defaulted": false, "effective_principal": "synthetic:user"}
	}
	before, after := decision(), decision()
	flags := map[string]bool{}
	for name := range before {
		flags[name] = false
	}
	summary := map[string]any{"classification": "UNCHANGED", "changed_count": 0}
	if action == "write" {
		after["allowed"], after["reason"] = false, "ACTION_NOT_READ_ONLY"
		flags["allowed"], flags["reason"] = true, true
		summary["classification"], summary["changed_count"] = "NEWLY_DENIED", 2
	}
	return map[string]any{"schema": "gooo/api-policy-revision-comparison/v1", "status": "COMPARED", "scope": "same_saved_facts_two_sealed_profiles", "before_profile": "cel-api-original-v1", "candidate_profile": "cel-api-readonly-v1", "before": map[string]any{"facts": apiPolicyLiteralFacts(action), "decision": before, "bindings": source.Original}, "after": map[string]any{"facts": apiPolicyLiteralFacts(action), "decision": after, "bindings": source.Revision}, "differences": flags, "summary": summary, "classifier_bindings": source.Revision}
}

type apiPolicyFixtureProcess struct {
	Name      string        `json:"name"`
	Operation string        `json:"operation"`
	Action    string        `json:"action"`
	ExitCode  int           `json:"exit_code"`
	Stdin     apiPolicyFile `json:"stdin"`
	Stdout    apiPolicyFile `json:"stdout"`
	Stderr    apiPolicyFile `json:"stderr"`
	Canonical apiPolicyFile `json:"canonical_stdout"`
}

func apiPolicyNativeFixtures(ctx context.Context, runner *apiPolicyRunner, work, binary string, source apiPolicySource) ([]byte, []apiPolicyFixtureProcess, error) {
	book := struct {
		Schema string `json:"schema"`
		Scope  string `json:"scope"`
		Cases  []struct {
			Action string          `json:"action"`
			Report json.RawMessage `json:"report"`
		} `json:"cases"`
	}{Schema: "gooo/api-policy-consumer-observation/v1", Scope: "synthetic_unsigned_historical_hypothetical_readonly"}
	var processes []apiPolicyFixtureProcess
	for _, action := range []string{"read", "write"} {
		facts, err := json.Marshal(apiPolicyLiteralFacts(action))
		if err != nil {
			return nil, nil, err
		}
		expected := apiPolicyLiteralReport(action, source)
		wantReport, err := json.Marshal(expected)
		if err != nil {
			return nil, nil, err
		}
		wantRequest, err := json.Marshal(map[string]any{"before_profile": "cel-api-original-v1", "candidate_profile": "cel-api-readonly-v1", "before": expected["before"]})
		if err != nil {
			return nil, nil, err
		}
		input := facts
		var comparison, comparisonRaw []byte
		for _, operation := range []string{"baseline", "compare", "replay"} {
			name := action + "-" + operation
			cwd := filepath.Join(work, "run-"+name)
			if err := os.Mkdir(cwd, 0700); err != nil {
				return nil, nil, err
			}
			stdout, stderr, err := runner.run(ctx, name, cwd, input, []string{"PATH="}, binary, "--mode", operation)
			if err != nil {
				return nil, nil, err
			}
			canonical, err := apiPolicyCanonicalCLI(stdout)
			if err != nil || len(stderr) != 0 {
				return nil, nil, fmt.Errorf("API policy %s did not return one complete quiet canonical record", name)
			}
			wanted := wantReport
			if operation == "baseline" {
				wanted = wantRequest
			}
			if !sameJSON(canonical, wanted) {
				return nil, nil, fmt.Errorf("API policy %s differs from independent complete literal expectations", name)
			}
			if operation == "compare" {
				comparison, comparisonRaw = canonical, stdout
			}
			if operation == "replay" && (!bytes.Equal(stdout, comparisonRaw) || !bytes.Equal(canonical, comparison)) {
				return nil, nil, fmt.Errorf("API policy saved replay raw/canonical bytes differ")
			}
			processes = append(processes, apiPolicyFixtureProcess{name, operation, action, 0, apiPolicyDescribe(input), apiPolicyDescribe(stdout), apiPolicyDescribe(stderr), apiPolicyDescribe(canonical)})
			input = stdout // Preserve the CLI's exact final LF in the next input.
		}
		book.Cases = append(book.Cases, struct {
			Action string          `json:"action"`
			Report json.RawMessage `json:"report"`
		}{action, json.RawMessage(comparison)})
	}
	raw, err := json.MarshalIndent(book, "", "  ")
	return append(raw, '\n'), processes, err
}

var apiPolicyPublishedRevision = []string{"readonly.gooo", "compare.gooo", "candidate.workspace.json", "differences.workspace.json", "summary.workspace.json", "profiles.json", "model.go", "compare.go", "input.go", "bindings.go", "adapter_generated.go", "go.mod", "generation.json", "compiler.json", "canonical/main.gooo", "canonical/scalar_access.gooo", "internal/candidate/generated.go", "internal/flags/generated.go", "internal/summary/generated.go", "cmd/compare/main.go"}
var apiPolicyPublishedOriginal = []string{"adapter_generated.go", "model.go", "bridge.go", "bindings.go", "input.go", "proof_input.go", "replay.go", "internal/api/generated.go", "internal/scalar/generated.go", "canonical/main.gooo", "canonical/scalar_access.gooo", "canonical/cel-api-policy.workspace.json", "canonical/cel-scalar.workspace.json", "canonical/source-pin.json", "compiler.json", "generation.json", "go.mod", "LICENSE-APACHE-2.0", "NOTICE.md", "oracle/upstream/LICENSE"}

func apiPolicyExpectedInfo(source apiPolicySource) map[string]any {
	return map[string]any{"schema": "gooo/api-policy-browser-bridge-info/v1", "scope": "unsigned_historical_hypothetical_comparison", "modes": []string{"baseline", "compare", "replay"}, "profiles": map[string]string{"original": "cel-api-original-v1", "candidate": "cel-api-readonly-v1"}, "limits": map[string]int{"text_utf8_bytes": 65536, "baseline_input_bytes": 4 << 20, "compare_input_bytes": 4 << 20, "replay_input_bytes": 16 << 20, "original_proof_bytes": 4 << 20, "native_facts_raw_bytes": 3539232, "output_bytes": 64 << 20}, "bindings": map[string]apiPolicyBindings{"original": source.Original, "revision": source.Revision}}
}

func apiPolicyPublicBytes(raw []byte, root string) error {
	for _, forbidden := range []string{"/Users/", "/home/", root} {
		if forbidden != "" && bytes.Contains(raw, []byte(forbidden)) {
			return fmt.Errorf("private absolute path in API policy public payload")
		}
	}
	return nil
}

func apiPolicyPutJSON(path string, value any, root string) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if err := apiPolicyPublicBytes(raw, root); err != nil {
		return nil, err
	}
	return raw, write(path, raw)
}

func buildAPIRevisionDemo(ctx context.Context, site, dir string, lock lockfile) (resultErr error) {
	if runtime.Version() != lock.GoVersion || lock.GoVersion != "go1.27.2" {
		return fmt.Errorf("API policy browser requires the installed pinned Go1.27.2")
	}
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(dir, site)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("API policy site must be beneath its build directory")
	}
	if real, err := filepath.EvalSymlinks(dir); err != nil || real != dir {
		return fmt.Errorf("API policy build directory cannot contain symlinks")
	}
	documents, err := apiPolicyReadDocumentsAt(root)
	if err != nil {
		return err
	}
	source, err := apiPolicyCheckDocuments(documents, lock)
	if err != nil {
		return err
	}
	checkout, err := apiPolicyObserveCheckout(ctx, root)
	if err != nil {
		return err
	}
	// Only current index paths are inspected; shallow CI needs no old Git objects.
	tracked := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--", apiPolicyModule)
	tracked.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	trackedRaw, err := tracked.Output()
	if err != nil {
		return err
	}
	trackedNames := strings.Split(strings.TrimSuffix(string(trackedRaw), "\x00"), "\x00")
	if len(trackedNames) != 78 {
		return fmt.Errorf("API policy tracked source roster changed")
	}
	for _, name := range trackedNames {
		if _, exists := documents[name]; !exists {
			return fmt.Errorf("API policy tracked source missing")
		}
	}
	controls := map[string][]byte{}
	controlNames := []string{"upstream.json", "cmd/build-demo/main.go", "cmd/build-demo/api_policy.go", "tools/api-policy-browser-bridge.go.txt", "site/api-policy.html", "site/api-policy.js", "site/api-policy.css", "site/api-policy-worker.js", "LICENSE", "NOTICE", "third_party/GO-LICENSE"}
	for _, name := range controlNames {
		raw, err := apiPolicyReadFile(root, name, false)
		if err != nil {
			return err
		}
		controls[name] = raw
	}
	goPath := filepath.Join(runtime.GOROOT(), "bin", "go")
	goRaw, err := os.ReadFile(goPath)
	if err != nil {
		return err
	}
	runtimePath := filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js")
	runtimeRaw, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	asset, evidence := filepath.Join(site, "api-policy"), filepath.Join(dir, "api-policy-evidence")
	if err := os.MkdirAll(site, 0755); err != nil {
		return err
	}
	for _, path := range []string{asset, evidence} {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	}
	work, err := os.MkdirTemp(dir, "api-policy-build-")
	if err != nil {
		return err
	}
	runner := &apiPolicyRunner{Evidence: evidence, Env: append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off", "GOTELEMETRY=off", "CGO_ENABLED=0", "GOFLAGS=", "GOEXPERIMENT=")}
	var nativeFile apiPolicyFile
	var finalSources map[string]apiPolicyFile
	defer func() {
		// CommandContext bounds the direct child. On cancellation, do not claim
		// that compiler descendants were observed stopped or remove their work.
		// The failure record leaves this one owned directory for later inspection.
		var cleanupErr error
		if ctx.Err() != nil {
			cleanupErr = fmt.Errorf("cancelled API build work retained; descendant termination was not independently verified")
		} else {
			cleanupErr = os.RemoveAll(work)
		}
		if cleanupErr != nil && resultErr == nil {
			resultErr = cleanupErr
		}
		record := map[string]any{"schema": "gooo/api-policy-browser-private-build-log/v1", "processes": runner.Steps, "checkout": checkout, "source_before": source.Files, "source_after": finalSources, "native_executable": nativeFile, "environment": apiPolicyRecordedEnvironment(runner.Env), "temporary_root": work, "temporary_removed": cleanupErr == nil, "temporary_retained_after_cancellation": ctx.Err() != nil, "descendant_termination_verified": false, "cache_reclaimed_bytes": nil, "go_executable": goPath, "GOROOT": runtime.GOROOT(), "error": ""}
		if resultErr != nil {
			record["error"] = resultErr.Error()
		}
		raw, marshalErr := json.MarshalIndent(record, "", "  ")
		if marshalErr == nil {
			marshalErr = os.WriteFile(filepath.Join(evidence, "commands.json"), append(raw, '\n'), 0600)
		}
		if marshalErr != nil && resultErr == nil {
			resultErr = marshalErr
		}
	}()
	// Reuse the caller's normal Go build cache. Module resolution and all source,
	// native executable and temporary work files belong to this one build only.
	goEnv, _, err := runner.run(ctx, "selected-go-env", root, nil, nil, goPath, "env", "-json", "GOVERSION", "GOROOT", "GOCACHE")
	if err != nil {
		return err
	}
	var observedGo struct{ GOVERSION, GOROOT, GOCACHE string }
	if apiPolicyDecode(goEnv, &observedGo) != nil || observedGo.GOVERSION != lock.GoVersion || filepath.Clean(observedGo.GOROOT) != filepath.Clean(runtime.GOROOT()) || !filepath.IsAbs(observedGo.GOCACHE) {
		return fmt.Errorf("selected Go executable identity differs from builder runtime")
	}
	for _, path := range []string{"home", "tmp", "gotmp", "module-cache"} {
		if err := os.Mkdir(filepath.Join(work, path), 0700); err != nil {
			return err
		}
	}
	runner.Env = append(runner.Env, "HOME="+filepath.Join(work, "home"), "TMPDIR="+filepath.Join(work, "tmp"), "GOTMPDIR="+filepath.Join(work, "gotmp"), "GOPATH="+filepath.Join(work, "gopath"), "GOMODCACHE="+filepath.Join(work, "module-cache"), "GOCACHE="+observedGo.GOCACHE)
	goVersion, _, err := runner.run(ctx, "selected-go-version", root, nil, nil, goPath, "version")
	if err != nil || string(goVersion) != "go version "+lock.GoVersion+" "+runtime.GOOS+"/"+runtime.GOARCH+"\n" {
		return fmt.Errorf("selected Go version/host mismatch")
	}
	consumer := filepath.Join(work, "consumer")
	if err := os.Mkdir(consumer, 0700); err != nil {
		return err
	}
	revisionRelative, err := filepath.Rel(consumer, filepath.Join(root, apiPolicyModule))
	if err != nil {
		return err
	}
	originalRelative, err := filepath.Rel(consumer, filepath.Join(root, apiPolicyModule, "profiles", "original"))
	if err != nil {
		return err
	}
	module := fmt.Sprintf("module github.com/kimjooyoon/gooo-go-ports/browser-api-policy-build\n\ngo 1.27.2\n\nrequire (\n\t%s v0.0.0\n\t%s %s\n)\n\nreplace %s => %q\nreplace %s => %q\n", apiPolicyRevisionModule, apiPolicyOriginalModule, apiPolicyOriginalVersion, apiPolicyRevisionModule, filepath.ToSlash(revisionRelative), apiPolicyOriginalModule, filepath.ToSlash(originalRelative))
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(module), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(consumer, "main.go"), controls["tools/api-policy-browser-bridge.go.txt"], 0600); err != nil {
		return err
	}
	native := filepath.Join(work, "compare-native")
	_, _, err = runner.run(ctx, "native-build", consumer, nil, []string{"GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH}, goPath, "build", "-trimpath", "-buildvcs=false", "-mod=readonly", "-o", native, apiPolicyRevisionModule+"/cmd/compare")
	if err != nil {
		return err
	}
	nativeRaw, err := os.ReadFile(native)
	if err != nil {
		return err
	}
	nativeFile = apiPolicyDescribe(nativeRaw)
	fixture, fixtureProcesses, err := apiPolicyNativeFixtures(ctx, runner, work, native, source)
	if err != nil {
		return err
	}
	if afterNative, err := os.ReadFile(native); err != nil || !bytes.Equal(afterNative, nativeRaw) {
		return fmt.Errorf("native API producer bytes changed during fixture generation")
	}
	_, _, err = runner.run(ctx, "wasm-build", consumer, nil, []string{"GOOS=js", "GOARCH=wasm"}, goPath, "build", "-trimpath", "-buildvcs=false", "-mod=readonly", "-o", filepath.Join(asset, "program.wasm"), ".")
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(filepath.Join(asset, "program.wasm"))
	if err != nil || len(wasm) < 8 || !bytes.Equal(wasm[:8], []byte{0, 'a', 's', 'm', 1, 0, 0, 0}) {
		return fmt.Errorf("API policy output is not WebAssembly")
	}
	if err := apiPolicyPublicBytes(wasm, root); err != nil {
		return err
	}
	after, err := apiPolicyReadDocumentsAt(root)
	if err != nil || !reflect.DeepEqual(documents, after) {
		return fmt.Errorf("API policy sources changed during build")
	}
	checkedAfter, err := apiPolicyCheckDocuments(after, lock)
	if err != nil {
		return err
	}
	finalSources = checkedAfter.Files
	for name, before := range controls {
		after, err := apiPolicyReadFile(root, name, false)
		if err != nil || !bytes.Equal(before, after) {
			return fmt.Errorf("API policy build control changed: %s", name)
		}
	}
	for path, before := range map[string][]byte{goPath: goRaw, runtimePath: runtimeRaw} {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			return fmt.Errorf("selected Go tool/runtime changed during build")
		}
	}
	finalCheckout, err := apiPolicyObserveCheckout(ctx, root)
	if err != nil || finalCheckout != checkout {
		return fmt.Errorf("API policy checkout identity/dirty observation changed during build")
	}
	// Remove the native executable/module before minting a successful public
	// package. Normal shared build cache stays in place and is never copied.
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if _, err := os.Lstat(work); !os.IsNotExist(err) {
		return fmt.Errorf("owned API build work was not removed")
	}
	put := func(name string, raw []byte) (apiPolicyFile, error) {
		if !apiPolicyLocalPath(name) || len(raw) == 0 {
			return apiPolicyFile{}, fmt.Errorf("invalid API policy package path")
		}
		if name != "program.wasm" {
			if err := apiPolicyPublicBytes(raw, root); err != nil {
				return apiPolicyFile{}, err
			}
		}
		if err := write(filepath.Join(asset, name), raw); err != nil {
			return apiPolicyFile{}, err
		}
		file := apiPolicyDescribe(raw)
		file.Path = "./" + name
		return file, nil
	}
	runtimeFile, err := put("wasm_exec.js", runtimeRaw)
	if err != nil {
		return err
	}
	fixtureFile, err := put("native-fixtures.json", fixture)
	if err != nil {
		return err
	}
	acceptance := map[string]any{"cases": 2, "facts": 9, "decision_fields_per_side": 15, "difference_flags": 15, "summary_fields": 2, "full_bindings_exact": true, "raw_stdout_one_LF": true, "replay_raw_bytes_exact": true}
	nativeSourcesRaw, err := json.Marshal(source.Files)
	if err != nil {
		return err
	}
	goIdentity := map[string]any{"version": lock.GoVersion, "executable": apiPolicyDescribe(goRaw), "runtime": apiPolicyDescribe(runtimeRaw)}
	fixtureReceipt := map[string]any{"schema": "gooo/api-policy-browser-native-fixtures-receipt/v1", "status": "FRESH_NATIVE_CLI_FIXTURES_PASS", "scope": "source_local_two_explicit_replacements_not_public_noReplace", "source": map[string]any{"head": checkout.Head, "tree": checkout.Tree, "dirty": checkout.Dirty, "native_sources_sha256": digest(nativeSourcesRaw), "original_generation": source.Original.GenerationSHA256, "revision_generation": source.Revision.GenerationSHA256}, "go": goIdentity, "fixture": fixtureFile, "processes": fixtureProcesses, "acceptance": acceptance, "limits": []string{"Finite source-local native read/write fixtures; not public no-replace resolution, an independent CEL evaluator or browser execution.", "Unsigned historical/hypothetical comparison only; no current authorization or external effect."}}
	receiptRaw, err := apiPolicyPutJSON(filepath.Join(asset, "native-fixtures-receipt.json"), fixtureReceipt, root)
	if err != nil {
		return err
	}
	receiptFile := apiPolicyDescribe(receiptRaw)
	receiptFile.Path = "./native-fixtures-receipt.json"
	uiFiles := map[string]apiPolicyFile{}
	for _, name := range []string{"api-policy.html", "api-policy.js", "api-policy.css", "api-policy-worker.js"} {
		raw := controls["site/"+name]
		if err := apiPolicyPublicBytes(raw, root); err != nil {
			return err
		}
		if err := write(filepath.Join(site, name), raw); err != nil {
			return err
		}
		uiFiles[name] = apiPolicyDescribe(raw)
	}
	published := map[string]apiPolicyFile{}
	publish := func(name string, raw []byte) error {
		file, err := put(name, raw)
		if err == nil {
			file.Path = ""
			published[name] = file
		}
		return err
	}
	for _, name := range apiPolicyPublishedRevision {
		if err := publish("source/revision/"+name, documents[apiPolicyModule+"/"+name]); err != nil {
			return err
		}
	}
	for _, name := range apiPolicyPublishedOriginal {
		if err := publish("source/original/"+name, documents[apiPolicyModule+"/profiles/original/"+name]); err != nil {
			return err
		}
	}
	for name, sourceName := range map[string]string{"source/api-policy-browser-bridge.go.txt": "tools/api-policy-browser-bridge.go.txt", "source/LICENSE": "LICENSE", "source/NOTICE": "NOTICE", "source/GO-LICENSE": "third_party/GO-LICENSE"} {
		if err := publish(name, controls[sourceName]); err != nil {
			return err
		}
	}
	for _, name := range []string{"LICENSE", "NOTICE", "third_party/GO-LICENSE"} {
		if err := write(filepath.Join(site, name), controls[name]); err != nil {
			return err
		}
	}
	controlFiles := map[string]apiPolicyFile{}
	for name, raw := range controls {
		controlFiles[name] = apiPolicyDescribe(raw)
	}
	wasmFile := apiPolicyDescribe(wasm)
	wasmFile.Path = "./program.wasm"
	buildRecord := map[string]any{"schema": "gooo/browser-api-policy-build/v1", "status": "NATIVE_FIXTURES_AND_WASM_BUILD_PASS", "scope": "source_local_generated_Gooo_native_to_Wasm_build_not_browser_execution", "source": map[string]any{"head": checkout.Head, "tree": checkout.Tree, "dirty": checkout.Dirty, "native_sources": source.Files, "control_sources": controlFiles, "source_before_after_equal": true, "historical_git_objects_checked": false}, "go": goIdentity, "native_executable": nativeFile, "temporary_main_module_sha256": digest([]byte(module)), "explicit_local_replacements": 2, "temporary_sources_native_binary_module_cache_removed": true, "normal_build_cache_retained_not_copied": true, "wasm": wasmFile, "runtime": runtimeFile, "native_fixtures": fixtureFile, "native_fixture_receipt": receiptFile, "acceptance": acceptance, "published_source_subset": published, "verbose_process_logs_published": false, "limits": []string{"Build and fresh native literal controls only; actual Wasm/Worker/browser tests remain separate mandatory validation.", "Full original28/revision36 binding maps do not claim every sealed source was fetched by the browser.", "Current checkout head/tree/dirty observed; frozen33 verified from fixed byte contracts without historical Git objects."}}
	buildRaw, err := apiPolicyPutJSON(filepath.Join(asset, "build.json"), buildRecord, root)
	if err != nil {
		return err
	}
	buildFile := apiPolicyDescribe(buildRaw)
	buildFile.Path = "./build.json"
	bridgeFile := apiPolicyDescribe(controls["tools/api-policy-browser-bridge.go.txt"])
	bridgeFile.Path = "./source/api-policy-browser-bridge.go.txt"
	workerFile := uiFiles["api-policy-worker.js"]
	workerFile.Path = "../api-policy-worker.js"
	packageManifest := map[string]any{"schema": "gooo/browser-api-policy-revision/v1", "scope": "unsigned_historical_hypothetical_comparison", "source": map[string]any{"head": checkout.Head, "tree": checkout.Tree, "dirty": checkout.Dirty, "tracked_file_count": 78, "native_sources": source.Files, "ui_files": uiFiles, "native_fixtures": fixtureFile, "native_fixture_receipt": receiptFile, "published_sources": published, "historical_git_objects_checked": false}, "bridge": bridgeFile, "build_verification": buildFile, "files": map[string]apiPolicyFile{"worker.js": workerFile, "program.wasm": wasmFile, "wasm_exec.js": runtimeFile}, "expected_info": apiPolicyExpectedInfo(source), "limits": map[string]any{"request_deadline_ms": 120000, "manifest_bytes": 512 << 10, "worker_source_bytes": 128 << 10, "note": "Operational deadline only; cancellation/timeout gives no semantic result."}}
	_, err = apiPolicyPutJSON(filepath.Join(asset, "manifest.json"), packageManifest, root)
	return err
}
