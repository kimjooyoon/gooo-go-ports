package main

import (
	"bufio"
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-go-ports/internal/jsonkeys"
)

const codegenFiles = "tools/browser-codegen"

type codegenRequest struct {
	Schema   string `json:"schema"`
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Source   string `json:"source"`
	Activity string `json:"activity"`
}

type codegenFixture struct {
	ID               string         `json:"id"`
	Origin           string         `json:"origin"`
	OriginKind       string         `json:"origin_kind"`
	OriginPath       string         `json:"origin_path"`
	OriginRevision   string         `json:"origin_revision"`
	OriginSHA256     string         `json:"origin_sha256"`
	SourceFile       string         `json:"source_file"`
	License          string         `json:"license"`
	Request          codegenRequest `json:"request"`
	NativeComparison bool           `json:"native_comparison"`
}

type codegenFixtureBook struct {
	Schema      string           `json:"schema"`
	CompilerPin string           `json:"compiler_pin"`
	Cases       []codegenFixture `json:"cases"`
}

type codegenHostResult struct {
	Schema             string          `json:"schema"`
	ID                 string          `json:"id"`
	SourceSHA256       string          `json:"source_sha256"`
	Activity           string          `json:"activity"`
	OK                 bool            `json:"ok"`
	Source             string          `json:"source"`
	GoooSource         string          `json:"gooo_source"`
	Report             json.RawMessage `json:"report"`
	Error              string          `json:"error"`
	CompilerPin        string          `json:"compiler_pin"`
	BridgeSourceSHA256 string          `json:"bridge_source_sha256"`
	NativeExecution    *bool           `json:"native_execution"`
	ModelCalls         *bool           `json:"model_calls"`
	Diagnostic         struct {
		Scope string `json:"scope"`
	} `json:"diagnostic"`
}

type codegenNativeResult struct {
	ID       string          `json:"id"`
	ExitCode int             `json:"exit_code"`
	Result   json.RawMessage `json:"result"`
}

// Build the compiler library through a separate, byte-bound Go overlay. Neither
// compiler sources nor compiler VCS identities are rewritten by this adapter.
func buildCodegenDemo(ctx context.Context, site, dir string, lock lockfile, compilerRoot string) error {
	core, err := filepath.Abs(compilerRoot)
	if err != nil {
		return err
	}
	core, err = filepath.EvalSymlinks(core)
	if err != nil {
		return fmt.Errorf("browser codegen compiler checkout: %w", err)
	}
	coreTree, err := checkCodegenCompiler(ctx, core, lock.CompilerRevision)
	if err != nil {
		return err
	}
	fixturesRaw, fixtures, err := readCodegenFixtures(core, lock)
	if err != nil {
		return err
	}
	asset := filepath.Join(site, "codegen")
	evidence := filepath.Join(dir, "codegen-evidence")
	for _, folder := range []string{asset, evidence} {
		if err := os.Mkdir(folder, 0755); err != nil {
			return err
		}
	}
	bridgeHashes := make(map[string]string)
	replacements := make(map[string]string)
	for _, name := range []string{"adapter.go", "entry_host.go", "entry_js.go"} {
		source, err := filepath.Abs(filepath.Join(codegenFiles, "bridge", name+".txt"))
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		formatted, err := format.Source(raw)
		if err != nil || !bytes.Equal(raw, formatted) {
			return fmt.Errorf("browser codegen bridge %s must be gofmt formatted", name)
		}
		bridgeHashes[name] = digest(raw)
		replacements[filepath.Join(core, "cmd", "gooo-browser-probe", name)] = source
		if err := write(filepath.Join(asset, "bridge", name), raw); err != nil {
			return err
		}
	}
	bridgeEncoded, err := json.Marshal(bridgeHashes)
	if err != nil {
		return err
	}
	bridgeSHA := digest(bridgeEncoded)
	overlay, err := json.MarshalIndent(map[string]any{"Replace": replacements}, "", "  ")
	if err != nil {
		return err
	}
	overlayPath := filepath.Join(evidence, "overlay.json")
	if err := write(overlayPath, append(overlay, '\n')); err != nil {
		return err
	}
	hostPath := filepath.Join(evidence, "codegen-host")
	wasmPath := filepath.Join(asset, "codegen.wasm")
	for _, target := range []struct {
		name, path string
		env        []string
	}{{"host", hostPath, nil}, {"wasm", wasmPath, []string{"GOOS=js", "GOARCH=wasm"}}} {
		args := []string{"build", "-overlay", overlayPath, "-mod=readonly", "-buildvcs=false", "-trimpath", "-ldflags", "-s -w -X main.bridgeSourceSHA=" + bridgeSHA, "-o", target.path, "./cmd/gooo-browser-probe"}
		stdout, stderr, exitCode, err := codegenCommand(ctx, core, target.env, nil, filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
		if saveErr := saveCodegenCommand(evidence, "build-"+target.name, stdout, stderr); saveErr != nil {
			return saveErr
		}
		if err != nil {
			return fmt.Errorf("build browser codegen %s: %w", target.name, err)
		}
		if exitCode != 0 {
			return fmt.Errorf("build browser codegen %s failed with exit %d: %s", target.name, exitCode, stderr)
		}
	}
	if tree, err := checkCodegenCompiler(ctx, core, lock.CompilerRevision); err != nil || tree != coreTree {
		return fmt.Errorf("compiler checkout changed during overlay build: %v", err)
	}
	dependencies, err := bindCodegenLicenses(ctx, core, hostPath, evidence, lock)
	if err != nil {
		return err
	}
	for _, name := range []string{"index.html", "probe.css", "probe.js", "probe-worker.js"} {
		if err := copyFile(filepath.Join("site", "codegen", name), filepath.Join(asset, name)); err != nil {
			return err
		}
	}
	if err := copyFile(filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"), filepath.Join(asset, "wasm_exec.js")); err != nil {
		return err
	}
	for _, name := range []string{"LICENSE-COMPILER", "LICENSE-DECISION-RUNTIME"} {
		if err := copyFile(filepath.Join(codegenFiles, name), filepath.Join(asset, name)); err != nil {
			return err
		}
	}
	// The served notice uses local license links; repository notice keeps its
	// original relative links. License bytes are copied without editing.
	notice, err := os.ReadFile(filepath.Join(codegenFiles, "NOTICE"))
	if err != nil {
		return err
	}
	notice = bytes.ReplaceAll(notice, []byte("bridge/adapter.go.txt"), []byte("bridge/adapter.go"))
	notice = bytes.ReplaceAll(notice, []byte("../../third_party/GO-LICENSE"), []byte("GO-LICENSE"))
	notice = bytes.ReplaceAll(notice, []byte("../../LICENSE"), []byte("LICENSE"))
	if err := write(filepath.Join(asset, "NOTICE"), notice); err != nil {
		return err
	}
	for from, to := range map[string]string{"LICENSE": "LICENSE", "third_party/GO-LICENSE": "GO-LICENSE"} {
		if err := copyFile(from, filepath.Join(asset, to)); err != nil {
			return err
		}
	}
	if err := write(filepath.Join(asset, "fixtures.json"), fixturesRaw); err != nil {
		return err
	}
	for _, fixture := range fixtures.Cases {
		if err := write(filepath.Join(asset, "fixtures", fixture.ID+".gooo"), []byte(fixture.Request.Source)); err != nil {
			return err
		}
	}
	if err := verifyCodegenNative(ctx, evidence, hostPath, fixtures, lock, bridgeSHA); err != nil {
		return err
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		return err
	}
	host, err := os.ReadFile(hostPath)
	if err != nil {
		return err
	}
	revision, err := command(ctx, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	status, err := command(ctx, nil, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	filesSHA := make(map[string]string)
	for _, name := range []string{"index.html", "probe.css", "probe.js", "probe-worker.js", "wasm_exec.js", "fixtures.json", "LICENSE-COMPILER", "LICENSE-DECISION-RUNTIME", "LICENSE", "GO-LICENSE", "NOTICE"} {
		raw, err := os.ReadFile(filepath.Join(asset, name))
		if err != nil {
			return err
		}
		filesSHA[name] = digest(raw)
	}
	build := map[string]any{
		"schema": "gooo/browser-codegen-build/v1", "compiler_pin": lock.CompilerRevision,
		"compiler_version": lock.CompilerVersion, "compiler_tree": coreTree,
		"compiler_tracked_source_edits": 0, "core_checkout_still_clean": true,
		"bridge_files_sha256": bridgeHashes, "bridge_source_sha256": bridgeSHA,
		"go_version": runtime.Version(), "execution_environment": runtime.GOOS + "/" + runtime.GOARCH,
		"source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0,
		"vcs_embedding": false, "new_dependency_downloads_allowed": false,
		"native_execution": false, "model_calls": false,
		"scope":        "Actual compiler-library code generation through a separate overlay bridge. Generated Go is not executed; full CLI, package imports, model files and providers are outside this browser adapter's scope. Compiler report provenance remains UNBOUND_LOCAL_SOURCE.",
		"host":         map[string]any{"bytes": len(host), "sha256": digest(host)},
		"wasm":         map[string]any{"bytes": len(wasm), "sha256": digest(wasm)},
		"files_sha256": filesSHA, "fixture_cases": len(fixtures.Cases), "native_comparison_cases": 15,
		"dependency_versions": dependencies,
	}
	raw, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(asset, "build-manifest.json"), append(raw, '\n'))
}

func bindCodegenLicenses(ctx context.Context, core, hostPath, evidence string, lock lockfile) (map[string]string, error) {
	info, err := buildinfo.ReadFile(hostPath)
	if err != nil || info.GoVersion != lock.GoVersion || info.Path != "github.com/kimjooyoon/meta-ontology-go/cmd/gooo-browser-probe" {
		return nil, fmt.Errorf("browser codegen host toolchain/module identity mismatch: %v", err)
	}
	dependencies := make(map[string]string)
	for _, module := range info.Deps {
		if module.Path != "github.com/kimjooyoon/gooo-decision-runtime" || module.Version != "v0.2.41-experimental" || module.Replace != nil {
			return nil, fmt.Errorf("browser codegen contains an unexpected or replaced linked module %s", module.Path)
		}
		dependencies[module.Path] = module.Version
	}
	if len(dependencies) != 1 {
		return nil, fmt.Errorf("browser codegen linked dependency identity differs")
	}
	stdout, stderr, exitCode, err := codegenCommand(ctx, core, nil, nil, filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", "github.com/kimjooyoon/gooo-decision-runtime")
	if err != nil || exitCode != 0 || len(stderr) != 0 {
		return nil, fmt.Errorf("browser codegen cannot locate the already-cached linked runtime: %v", err)
	}
	runtimeRoot := strings.TrimSpace(string(stdout))
	for _, pair := range []struct{ copy, original string }{
		{filepath.Join(codegenFiles, "LICENSE-COMPILER"), filepath.Join(core, "LICENSE")},
		{filepath.Join(codegenFiles, "LICENSE-DECISION-RUNTIME"), filepath.Join(runtimeRoot, "LICENSE")},
		{"third_party/GO-LICENSE", filepath.Join(runtime.GOROOT(), "LICENSE")},
	} {
		copied, err := os.ReadFile(pair.copy)
		if err != nil {
			return nil, err
		}
		original, err := os.ReadFile(pair.original)
		if err != nil || !bytes.Equal(copied, original) {
			return nil, fmt.Errorf("browser codegen license %s differs from the actual pinned source: %v", pair.copy, err)
		}
	}
	if err := writeCodegenJSON(filepath.Join(evidence, "host-linked-modules.json"), map[string]any{"go_version": info.GoVersion, "module_path": info.Path, "dependency_versions": dependencies}); err != nil {
		return nil, err
	}
	return dependencies, nil
}

func checkCodegenCompiler(ctx context.Context, core, revision string) (string, error) {
	for _, item := range []struct {
		args []string
		want string
	}{{[]string{"-C", core, "rev-parse", "--show-toplevel"}, core}, {[]string{"-C", core, "rev-parse", "HEAD"}, revision}, {[]string{"-C", core, "status", "--porcelain", "--untracked-files=all", "--ignored"}, ""}} {
		result, err := command(ctx, nil, "git", item.args...)
		if err != nil || strings.TrimSpace(string(result)) != item.want {
			return "", fmt.Errorf("browser codegen requires its own clean pinned compiler checkout: %v", err)
		}
	}
	tree, err := command(ctx, nil, "git", "-C", core, "rev-parse", "HEAD^{tree}")
	return strings.TrimSpace(string(tree)), err
}

func readCodegenFixtures(core string, lock lockfile) ([]byte, codegenFixtureBook, error) {
	var book codegenFixtureBook
	raw, err := os.ReadFile(filepath.Join(codegenFiles, "fixtures.json"))
	if err != nil {
		return nil, book, err
	}
	if err := jsonkeys.RejectDuplicates(raw); err != nil {
		return nil, book, err
	}
	if err := json.Unmarshal(raw, &book); err != nil {
		return nil, book, err
	}
	ids := []string{"clamp", "changed", "double", "invalid-body", "malformed", "text", "multiple-inputs", "record", "ir-fill", "ir-search", "record-assembly", "typed-assembly", "pure-callee", "missing-activity", "invalid-import", "type-error"}
	if book.Schema != "gooo/browser-codegen-probe-fixtures/v1" || book.CompilerPin != lock.CompilerRevision || len(book.Cases) != len(ids) {
		return nil, book, fmt.Errorf("browser codegen fixture catalog differs from the pinned 16-case scope")
	}
	for i, row := range book.Cases {
		request := row.Request
		if row.ID != ids[i] || request.ID != row.ID || request.Schema != "gooo/browser-body-codegen-request/v1" || request.Filename != row.ID+".gooo" || request.Activity == "" || request.Source == "" || row.SourceFile != filepath.ToSlash(filepath.Join(codegenFiles, "fixtures", request.Filename)) || row.NativeComparison != (row.ID != "invalid-import") {
			return nil, book, fmt.Errorf("browser codegen fixture %s identity or scope mismatch", ids[i])
		}
		copied, err := os.ReadFile(row.SourceFile)
		if err != nil || string(copied) != request.Source || digest(copied) != row.OriginSHA256 {
			return nil, book, fmt.Errorf("browser codegen fixture %s source bytes or digest mismatch: %v", row.ID, err)
		}
		if !filepath.IsLocal(row.OriginPath) || strings.Contains(row.Origin, "/Users/") {
			return nil, book, fmt.Errorf("browser codegen fixture %s must retain a public or repository-local origin", row.ID)
		}
		originRoot := "."
		switch row.OriginKind {
		case "compiler":
			originRoot = core
			if row.OriginRevision != lock.CompilerRevision || row.Origin != "https://github.com/kimjooyoon/meta-ontology-go/blob/"+lock.CompilerRevision+"/"+row.OriginPath || row.License != "LICENSE-COMPILER" {
				return nil, book, fmt.Errorf("browser codegen fixture %s compiler origin mismatch", row.ID)
			}
		case "ports":
			if row.OriginRevision != "09861502085012fe41ab5993a220e3fde7f0a87d" || row.Origin != "https://github.com/kimjooyoon/gooo-go-ports/blob/"+row.OriginRevision+"/"+row.OriginPath || !strings.HasPrefix(row.OriginPath, "tools/compiler-explorer/fixtures/") {
				return nil, book, fmt.Errorf("browser codegen fixture %s port origin mismatch", row.ID)
			}
		case "local":
			if row.Origin != row.SourceFile || row.OriginPath != row.SourceFile || row.OriginRevision != "" {
				return nil, book, fmt.Errorf("browser codegen fixture %s authored origin mismatch", row.ID)
			}
		default:
			return nil, book, fmt.Errorf("browser codegen fixture %s unknown origin kind", row.ID)
		}
		original, err := os.ReadFile(filepath.Join(originRoot, row.OriginPath))
		if err != nil || !bytes.Equal(copied, original) {
			return nil, book, fmt.Errorf("browser codegen fixture %s differs from its original source: %v", row.ID, err)
		}
	}
	return raw, book, nil
}

func codegenCommand(ctx context.Context, dir string, env []string, input []byte, executable string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=0", "GOPROXY=off", "GOFLAGS=", "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=", "PATH="+filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.Bytes(), stderr.Bytes(), -1, ctx.Err()
	}
	if err == nil {
		return stdout.Bytes(), stderr.Bytes(), 0, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.Bytes(), stderr.Bytes(), exit.ExitCode(), nil
	}
	return stdout.Bytes(), stderr.Bytes(), -1, err
}

func saveCodegenCommand(dir, name string, stdout, stderr []byte) error {
	if err := write(filepath.Join(dir, name+".stdout"), stdout); err != nil {
		return err
	}
	return write(filepath.Join(dir, name+".stderr"), stderr)
}

func verifyCodegenNative(ctx context.Context, evidence, hostPath string, fixtures codegenFixtureBook, lock lockfile, bridgeSHA string) error {
	var requests bytes.Buffer
	for _, row := range fixtures.Cases {
		raw, err := json.Marshal(row.Request)
		if err != nil {
			return err
		}
		requests.Write(raw)
		requests.WriteByte('\n')
	}
	if err := write(filepath.Join(evidence, "host-requests.ndjson"), requests.Bytes()); err != nil {
		return err
	}
	caseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	stdout, stderr, exitCode, err := codegenCommand(caseCtx, evidence, nil, requests.Bytes(), hostPath)
	cancel()
	if saveErr := saveCodegenCommand(evidence, "host", stdout, stderr); saveErr != nil {
		return saveErr
	}
	if err != nil || exitCode != 0 || len(stderr) != 0 {
		return fmt.Errorf("browser codegen host probe failed (exit %d): %v: %s", exitCode, err, stderr)
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	var hostRaw []json.RawMessage
	var hosts []codegenHostResult
	for scanner.Scan() {
		raw := append(json.RawMessage(nil), scanner.Bytes()...)
		if err := jsonkeys.RejectDuplicates(raw); err != nil {
			return err
		}
		var result codegenHostResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return err
		}
		hostRaw, hosts = append(hostRaw, raw), append(hosts, result)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(hosts) != len(fixtures.Cases) {
		return fmt.Errorf("browser codegen host result roster mismatch")
	}
	if err := writeCodegenJSON(filepath.Join(evidence, "host-results.json"), hostRaw); err != nil {
		return err
	}
	nativeCLI, err := filepath.Abs(".gooo")
	if err != nil {
		return err
	}
	var natives []codegenNativeResult
	accepted := 0
	for i, row := range fixtures.Cases {
		host := hosts[i]
		if host.Schema != "gooo/browser-body-codegen-response/v1" || host.ID != row.ID || host.SourceSHA256 != "sha256:"+row.OriginSHA256 || host.Activity != row.Request.Activity || host.CompilerPin != lock.CompilerRevision || host.BridgeSourceSHA256 != bridgeSHA || host.NativeExecution == nil || *host.NativeExecution || host.ModelCalls == nil || *host.ModelCalls {
			return fmt.Errorf("browser codegen host fixture %s response identity mismatch", row.ID)
		}
		if !host.OK && (host.Source != "" || host.GoooSource != "" || string(host.Report) != "null" || host.Error == "") {
			return fmt.Errorf("browser codegen host fixture %s retained output on rejection", row.ID)
		}
		if !row.NativeComparison {
			if host.OK || host.Diagnostic.Scope != "bridge_scope" {
				return fmt.Errorf("browser codegen fixture %s must be explicitly outside bridge scope", row.ID)
			}
			continue
		}
		inputs := filepath.Join(evidence, "inputs", row.ID)
		if err := write(filepath.Join(inputs, row.Request.Filename), []byte(row.Request.Source)); err != nil {
			return err
		}
		caseCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		stdout, stderr, exitCode, err := codegenCommand(caseCtx, inputs, nil, nil, nativeCLI, "body-codegen", "--json", "--activity", row.Request.Activity, row.Request.Filename)
		cancel()
		if saveErr := saveCodegenCommand(evidence, "cli-"+row.ID, stdout, stderr); saveErr != nil {
			return saveErr
		}
		if err != nil || (exitCode != 0 && exitCode != 1) || host.OK != (exitCode == 0) {
			return fmt.Errorf("browser codegen fixture %s native/host acceptance mismatch (exit %d): %v", row.ID, exitCode, err)
		}
		native := codegenNativeResult{ID: row.ID, ExitCode: exitCode}
		if exitCode == 0 {
			if err := jsonkeys.RejectDuplicates(stdout); err != nil {
				return err
			}
			var generated struct {
				Source     string          `json:"source"`
				GoooSource string          `json:"gooo_source"`
				Report     json.RawMessage `json:"report"`
			}
			if err := json.Unmarshal(stdout, &generated); err != nil || len(stderr) != 0 || host.Source == "" || host.Source != generated.Source || host.GoooSource != generated.GoooSource {
				return fmt.Errorf("browser codegen fixture %s native generated bytes mismatch: %v", row.ID, err)
			}
			var sourceDigest string
			for _, item := range []struct {
				raw json.RawMessage
				pin string
			}{{host.Report, "UNBOUND_LOCAL_SOURCE"}, {generated.Report, lock.CompilerRevision}} {
				var report struct {
					CompilerSourceSHA string `json:"compiler_source_sha"`
					SourceDigest      string `json:"source_digest"`
					GeneratedDigest   string `json:"generated_digest"`
					BodyFill          *struct {
						OriginalSourceDigest string `json:"original_source_digest"`
					} `json:"body_fill"`
					BodySearch *struct {
						OriginalSourceDigest string `json:"original_source_digest"`
					} `json:"body_search"`
				}
				if err := json.Unmarshal(item.raw, &report); err != nil || report.CompilerSourceSHA != item.pin || report.SourceDigest == "" || report.GeneratedDigest != "sha256:"+digest([]byte(host.Source)) {
					return fmt.Errorf("browser codegen fixture %s native/bridge provenance mismatch: %v", row.ID, err)
				}
				// Source-owned assembly can bind source_digest to a materialized
				// source rather than the editor input. Preserve that distinction.
				if sourceDigest != "" && sourceDigest != report.SourceDigest {
					return fmt.Errorf("browser codegen fixture %s native/bridge materialized source mismatch", row.ID)
				}
				sourceDigest = report.SourceDigest
				for _, original := range []*struct {
					OriginalSourceDigest string `json:"original_source_digest"`
				}{report.BodyFill, report.BodySearch} {
					if original != nil && original.OriginalSourceDigest != host.SourceSHA256 {
						return fmt.Errorf("browser codegen fixture %s original source binding mismatch", row.ID)
					}
				}
			}
			native.Result = append(json.RawMessage(nil), bytes.TrimSpace(stdout)...)
			accepted++
		} else {
			// --json returns a fail-closed report on stdout, even when the
			// command exits 1. Preserve this original diagnostic receipt.
			if err := jsonkeys.RejectDuplicates(stdout); err != nil {
				return fmt.Errorf("browser codegen fixture %s invalid native rejection JSON: %w", row.ID, err)
			}
			var rejected struct {
				Schema            string `json:"schema"`
				Decision          string `json:"decision"`
				Source            string `json:"source"`
				Error             string `json:"error"`
				CompilerSourceSHA string `json:"compiler_source_sha"`
				RepositoryWrites  *int   `json:"repository_writes"`
			}
			if err := json.Unmarshal(stdout, &rejected); err != nil || len(stderr) != 0 ||
				rejected.Schema != "gooo/body-codegen-report/v3" || rejected.Decision != "FAIL_CLOSED" ||
				rejected.Source != "" || rejected.Error == "" || rejected.CompilerSourceSHA != lock.CompilerRevision ||
				rejected.RepositoryWrites == nil || *rejected.RepositoryWrites != 0 {
				return fmt.Errorf("browser codegen fixture %s invalid native rejection envelope: %v", row.ID, err)
			}
			if host.Diagnostic.Scope != "compiler_syntax" && rejected.Error != host.Error {
				return fmt.Errorf("browser codegen fixture %s native/bridge diagnostic mismatch", row.ID)
			}
			native.Result = append(json.RawMessage(nil), bytes.TrimSpace(stdout)...)
		}
		natives = append(natives, native)
	}
	if accepted != 11 || len(natives) != 15 {
		return fmt.Errorf("browser codegen finite native baseline differs: %d accepted, %d compared", accepted, len(natives))
	}
	if err := writeCodegenJSON(filepath.Join(evidence, "native-cli-results.json"), natives); err != nil {
		return err
	}
	return writeCodegenJSON(filepath.Join(evidence, "native-host-summary.json"), map[string]any{
		"schema": "gooo/browser-codegen-native-host-check/v1", "status": "PASS",
		"host_cases": len(hosts), "native_cli_cases": len(natives), "accepted": accepted,
		"compiler_pin": lock.CompilerRevision, "bridge_source_sha256": bridgeSHA,
		"generated_source_matches_native_cli": true, "selected_gooo_matches_native_cli": true,
		"generated_go_executed": false, "model_calls_performed": false,
		"scope": "Finite native CLI and native overlay-host source generation observations; Wasm/receipt structural checks run separately and no all-source or runtime-execution claim is made.",
	})
}

func writeCodegenJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return write(path, append(raw, '\n'))
}
