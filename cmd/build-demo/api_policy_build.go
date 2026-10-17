package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const apiPolicyCurrentSourceFiles = 82

type apiPolicyBuild struct {
	site, dir, root     string
	lock                lockfile
	documents, controls map[string][]byte
	source              apiPolicySource
	checkout            apiPolicyCheckout

	goPath, runtimePath      string
	goRaw, runtimeRaw        []byte
	asset, evidence, work    string
	consumer, module, native string
	runner                   *apiPolicyRunner
	finalSources             map[string]apiPolicyFile

	fixture, wasm            []byte
	fixtureProcesses         []apiPolicyFixtureProcess
	nativeFile, runtimeFile  apiPolicyFile
	fixtureFile, receiptFile apiPolicyFile
	uiFiles, published       map[string]apiPolicyFile
	acceptance, goIdentity   map[string]any
}

func buildAPIRevisionDemo(ctx context.Context, site, dir string, lock lockfile) (resultErr error) {
	b := &apiPolicyBuild{site: site, dir: dir, lock: lock}
	if err := b.prepare(ctx); err != nil {
		return err
	}
	if err := b.prepareWork(); err != nil {
		return err
	}
	defer func() { resultErr = b.finish(ctx, resultErr) }()
	for _, stage := range []func(context.Context) error{b.prepareConsumer, b.build, b.verify} {
		if err := stage(ctx); err != nil {
			return err
		}
	}
	for _, stage := range []func() error{b.publishAssets, b.publishSources, b.publish} {
		if err := stage(); err != nil {
			return err
		}
	}
	return nil
}

func (b *apiPolicyBuild) prepare(ctx context.Context) error {
	var err error
	if runtime.Version() != b.lock.GoVersion || b.lock.GoVersion != "go1.27.2" {
		return fmt.Errorf("API policy browser requires the installed pinned Go1.27.2")
	}
	b.root, err = filepath.Abs(".")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(b.dir, b.site)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("API policy site must be beneath its build directory")
	}
	if real, err := filepath.EvalSymlinks(b.dir); err != nil || real != b.dir {
		return fmt.Errorf("API policy build directory cannot contain symlinks")
	}
	b.documents, err = apiPolicyReadDocumentsAt(b.root)
	if err != nil {
		return err
	}
	b.source, err = apiPolicyCheckDocuments(b.documents, b.lock)
	if err != nil {
		return err
	}
	b.checkout, err = apiPolicyObserveCheckout(ctx, b.root)
	if err != nil {
		return err
	}
	if err := b.checkTracked(ctx); err != nil {
		return err
	}
	b.controls = map[string][]byte{}
	controlNames := []string{"upstream.json", "cmd/build-demo/main.go", "cmd/build-demo/api_policy.go", "cmd/build-demo/api_policy_build.go", "tools/api-policy-browser-bridge.go.txt", "site/api-policy.html", "site/api-policy.js", "site/api-policy.css", "site/api-policy-worker.js", "LICENSE", "NOTICE", "third_party/GO-LICENSE"}
	for _, name := range controlNames {
		raw, err := apiPolicyReadFile(b.root, name, false)
		if err != nil {
			return err
		}
		b.controls[name] = raw
	}
	b.goPath = filepath.Join(runtime.GOROOT(), "bin", "go")
	b.goRaw, err = os.ReadFile(b.goPath)
	if err != nil {
		return err
	}
	b.runtimePath = filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js")
	b.runtimeRaw, err = os.ReadFile(b.runtimePath)
	if err != nil {
		return err
	}
	return nil
}

func (b *apiPolicyBuild) checkTracked(ctx context.Context) error {
	// Only current index paths are inspected; shallow CI needs no old Git objects.
	tracked := exec.CommandContext(ctx, "git", "-C", b.root, "ls-files", "-z", "--", apiPolicyModule)
	tracked.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	trackedRaw, err := tracked.Output()
	if err != nil {
		return err
	}
	trackedNames := strings.Split(strings.TrimSuffix(string(trackedRaw), "\x00"), "\x00")
	if len(trackedNames) != apiPolicyCurrentSourceFiles {
		return fmt.Errorf("API policy tracked source roster changed")
	}
	for _, name := range trackedNames {
		if _, exists := b.documents[name]; !exists {
			return fmt.Errorf("API policy tracked source missing")
		}
	}
	return nil
}

func (b *apiPolicyBuild) prepareWork() error {
	var err error
	b.asset, b.evidence = filepath.Join(b.site, "api-policy"), filepath.Join(b.dir, "api-policy-evidence")
	if err := os.MkdirAll(b.site, 0755); err != nil {
		return err
	}
	for _, path := range []string{b.asset, b.evidence} {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	}
	b.work, err = os.MkdirTemp(b.dir, "api-policy-build-")
	if err != nil {
		return err
	}
	b.runner = &apiPolicyRunner{Evidence: b.evidence, Env: append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off", "GOTELEMETRY=off", "CGO_ENABLED=0", "GOFLAGS=", "GOEXPERIMENT=")}
	return nil
}

func (b *apiPolicyBuild) finish(ctx context.Context, resultErr error) error {
	// CommandContext bounds the direct child. On cancellation, do not claim
	// that compiler descendants were observed stopped or remove their work.
	// The failure record leaves this one owned directory for later inspection.
	var cleanupErr error
	if ctx.Err() != nil {
		cleanupErr = fmt.Errorf("cancelled API build work retained; descendant termination was not independently verified")
	} else {
		cleanupErr = os.RemoveAll(b.work)
	}
	if cleanupErr != nil && resultErr == nil {
		resultErr = cleanupErr
	}
	record := map[string]any{"schema": "gooo/api-policy-browser-private-build-log/v1", "processes": b.runner.Steps, "checkout": b.checkout, "source_before": b.source.Files, "source_after": b.finalSources, "native_executable": b.nativeFile, "environment": apiPolicyRecordedEnvironment(b.runner.Env), "temporary_root": b.work, "temporary_removed": cleanupErr == nil, "temporary_retained_after_cancellation": ctx.Err() != nil, "descendant_termination_verified": false, "cache_reclaimed_bytes": nil, "go_executable": b.goPath, "GOROOT": runtime.GOROOT(), "error": ""}
	if resultErr != nil {
		record["error"] = resultErr.Error()
	}
	raw, marshalErr := json.MarshalIndent(record, "", "  ")
	if marshalErr == nil {
		marshalErr = os.WriteFile(filepath.Join(b.evidence, "commands.json"), append(raw, '\n'), 0600)
	}
	if marshalErr != nil && resultErr == nil {
		resultErr = marshalErr
	}
	return resultErr
}

func (b *apiPolicyBuild) prepareConsumer(ctx context.Context) error {
	var err error
	// Reuse the caller's normal Go build cache. Module resolution and all source,
	// native executable and temporary work files belong to this one build only.
	goEnv, _, err := b.runner.run(ctx, "selected-go-env", b.root, nil, nil, b.goPath, "env", "-json", "GOVERSION", "GOROOT", "GOCACHE")
	if err != nil {
		return err
	}
	var observedGo struct{ GOVERSION, GOROOT, GOCACHE string }
	if apiPolicyDecode(goEnv, &observedGo) != nil || observedGo.GOVERSION != b.lock.GoVersion || filepath.Clean(observedGo.GOROOT) != filepath.Clean(runtime.GOROOT()) || !filepath.IsAbs(observedGo.GOCACHE) {
		return fmt.Errorf("selected Go executable identity differs from builder runtime")
	}
	for _, path := range []string{"home", "tmp", "gotmp", "module-cache"} {
		if err := os.Mkdir(filepath.Join(b.work, path), 0700); err != nil {
			return err
		}
	}
	b.runner.Env = append(b.runner.Env, "HOME="+filepath.Join(b.work, "home"), "TMPDIR="+filepath.Join(b.work, "tmp"), "GOTMPDIR="+filepath.Join(b.work, "gotmp"), "GOPATH="+filepath.Join(b.work, "gopath"), "GOMODCACHE="+filepath.Join(b.work, "module-cache"), "GOCACHE="+observedGo.GOCACHE)
	goVersion, _, err := b.runner.run(ctx, "selected-go-version", b.root, nil, nil, b.goPath, "version")
	if err != nil || string(goVersion) != "go version "+b.lock.GoVersion+" "+runtime.GOOS+"/"+runtime.GOARCH+"\n" {
		return fmt.Errorf("selected Go version/host mismatch")
	}
	b.consumer = filepath.Join(b.work, "consumer")
	if err := os.Mkdir(b.consumer, 0700); err != nil {
		return err
	}
	revisionRelative, err := filepath.Rel(b.consumer, filepath.Join(b.root, apiPolicyModule))
	if err != nil {
		return err
	}
	originalRelative, err := filepath.Rel(b.consumer, filepath.Join(b.root, apiPolicyModule, "profiles", "original"))
	if err != nil {
		return err
	}
	b.module = fmt.Sprintf("module github.com/kimjooyoon/gooo-go-ports/browser-api-policy-build\n\ngo 1.27.2\n\nrequire (\n\t%s v0.0.0\n\t%s %s\n)\n\nreplace %s => %q\nreplace %s => %q\n", apiPolicyRevisionModule, apiPolicyOriginalModule, apiPolicyOriginalVersion, apiPolicyRevisionModule, filepath.ToSlash(revisionRelative), apiPolicyOriginalModule, filepath.ToSlash(originalRelative))
	if err := os.WriteFile(filepath.Join(b.consumer, "go.mod"), []byte(b.module), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.consumer, "main.go"), b.controls["tools/api-policy-browser-bridge.go.txt"], 0600); err != nil {
		return err
	}
	return nil
}

func (b *apiPolicyBuild) build(ctx context.Context) error {
	var err error
	b.native = filepath.Join(b.work, "compare-native")
	_, _, err = b.runner.run(ctx, "native-build", b.consumer, nil, []string{"GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH}, b.goPath, "build", "-trimpath", "-buildvcs=false", "-mod=readonly", "-o", b.native, apiPolicyRevisionModule+"/cmd/compare")
	if err != nil {
		return err
	}
	nativeRaw, err := os.ReadFile(b.native)
	if err != nil {
		return err
	}
	b.nativeFile = apiPolicyDescribe(nativeRaw)
	b.fixture, b.fixtureProcesses, err = apiPolicyNativeFixtures(ctx, b.runner, b.work, b.native, b.source)
	if err != nil {
		return err
	}
	if afterNative, err := os.ReadFile(b.native); err != nil || !bytes.Equal(afterNative, nativeRaw) {
		return fmt.Errorf("native API producer bytes changed during fixture generation")
	}
	_, _, err = b.runner.run(ctx, "wasm-build", b.consumer, nil, []string{"GOOS=js", "GOARCH=wasm"}, b.goPath, "build", "-trimpath", "-buildvcs=false", "-mod=readonly", "-o", filepath.Join(b.asset, "program.wasm"), ".")
	if err != nil {
		return err
	}
	b.wasm, err = os.ReadFile(filepath.Join(b.asset, "program.wasm"))
	if err != nil || len(b.wasm) < 8 || !bytes.Equal(b.wasm[:8], []byte{0, 'a', 's', 'm', 1, 0, 0, 0}) {
		return fmt.Errorf("API policy output is not WebAssembly")
	}
	if err := apiPolicyPublicBytes(b.wasm, b.root); err != nil {
		return err
	}
	return nil
}

func (b *apiPolicyBuild) verify(ctx context.Context) error {
	after, err := apiPolicyReadDocumentsAt(b.root)
	if err != nil || !reflect.DeepEqual(b.documents, after) {
		return fmt.Errorf("API policy sources changed during build")
	}
	checkedAfter, err := apiPolicyCheckDocuments(after, b.lock)
	if err != nil {
		return err
	}
	b.finalSources = checkedAfter.Files
	for name, before := range b.controls {
		after, err := apiPolicyReadFile(b.root, name, false)
		if err != nil || !bytes.Equal(before, after) {
			return fmt.Errorf("API policy build control changed: %s", name)
		}
	}
	for path, before := range map[string][]byte{b.goPath: b.goRaw, b.runtimePath: b.runtimeRaw} {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			return fmt.Errorf("selected Go tool/runtime changed during build")
		}
	}
	finalCheckout, err := apiPolicyObserveCheckout(ctx, b.root)
	if err != nil || finalCheckout != b.checkout {
		return fmt.Errorf("API policy checkout identity/dirty observation changed during build")
	}
	// Remove the native executable/module before minting a successful public
	// package. Normal shared build cache stays in place and is never copied.
	if err := os.RemoveAll(b.work); err != nil {
		return err
	}
	if _, err := os.Lstat(b.work); !os.IsNotExist(err) {
		return fmt.Errorf("owned API build work was not removed")
	}
	return nil
}

func (b *apiPolicyBuild) put(name string, raw []byte) (apiPolicyFile, error) {
	if !apiPolicyLocalPath(name) || len(raw) == 0 {
		return apiPolicyFile{}, fmt.Errorf("invalid API policy package path")
	}
	if name != "program.wasm" {
		if err := apiPolicyPublicBytes(raw, b.root); err != nil {
			return apiPolicyFile{}, err
		}
	}
	if err := write(filepath.Join(b.asset, name), raw); err != nil {
		return apiPolicyFile{}, err
	}
	file := apiPolicyDescribe(raw)
	file.Path = "./" + name
	return file, nil
}

func (b *apiPolicyBuild) publishAssets() error {
	var err error
	b.runtimeFile, err = b.put("wasm_exec.js", b.runtimeRaw)
	if err != nil {
		return err
	}
	b.fixtureFile, err = b.put("native-fixtures.json", b.fixture)
	if err != nil {
		return err
	}
	b.acceptance = map[string]any{"cases": 2, "facts": 9, "decision_fields_per_side": 15, "difference_flags": 15, "summary_fields": 2, "full_bindings_exact": true, "raw_stdout_one_LF": true, "replay_raw_bytes_exact": true}
	nativeSourcesRaw, err := json.Marshal(b.source.Files)
	if err != nil {
		return err
	}
	b.goIdentity = map[string]any{"version": b.lock.GoVersion, "executable": apiPolicyDescribe(b.goRaw), "runtime": apiPolicyDescribe(b.runtimeRaw)}
	fixtureReceipt := map[string]any{"schema": "gooo/api-policy-browser-native-fixtures-receipt/v1", "status": "FRESH_NATIVE_CLI_FIXTURES_PASS", "scope": "source_local_two_explicit_replacements_not_public_noReplace", "source": map[string]any{"head": b.checkout.Head, "tree": b.checkout.Tree, "dirty": b.checkout.Dirty, "native_sources_sha256": digest(nativeSourcesRaw), "original_generation": b.source.Original.GenerationSHA256, "revision_generation": b.source.Revision.GenerationSHA256}, "go": b.goIdentity, "fixture": b.fixtureFile, "processes": b.fixtureProcesses, "acceptance": b.acceptance, "limits": []string{"Finite source-local native read/write fixtures; not public no-replace resolution, an independent CEL evaluator or browser execution.", "Unsigned historical/hypothetical comparison only; no current authorization or external effect."}}
	receiptRaw, err := apiPolicyPutJSON(filepath.Join(b.asset, "native-fixtures-receipt.json"), fixtureReceipt, b.root)
	if err != nil {
		return err
	}
	b.receiptFile = apiPolicyDescribe(receiptRaw)
	b.receiptFile.Path = "./native-fixtures-receipt.json"
	return nil
}

func (b *apiPolicyBuild) publishSources() error {
	b.uiFiles = map[string]apiPolicyFile{}
	for _, name := range []string{"api-policy.html", "api-policy.js", "api-policy.css", "api-policy-worker.js"} {
		raw := b.controls["site/"+name]
		if err := apiPolicyPublicBytes(raw, b.root); err != nil {
			return err
		}
		if err := write(filepath.Join(b.site, name), raw); err != nil {
			return err
		}
		b.uiFiles[name] = apiPolicyDescribe(raw)
	}
	b.published = map[string]apiPolicyFile{}
	publish := func(name string, raw []byte) error {
		file, err := b.put(name, raw)
		if err == nil {
			file.Path = ""
			b.published[name] = file
		}
		return err
	}
	for _, name := range apiPolicyPublishedRevision {
		if err := publish("source/revision/"+name, b.documents[apiPolicyModule+"/"+name]); err != nil {
			return err
		}
	}
	for _, name := range apiPolicyPublishedOriginal {
		if err := publish("source/original/"+name, b.documents[apiPolicyModule+"/profiles/original/"+name]); err != nil {
			return err
		}
	}
	for name, sourceName := range map[string]string{"source/api-policy-browser-bridge.go.txt": "tools/api-policy-browser-bridge.go.txt", "source/LICENSE": "LICENSE", "source/NOTICE": "NOTICE", "source/GO-LICENSE": "third_party/GO-LICENSE"} {
		if err := publish(name, b.controls[sourceName]); err != nil {
			return err
		}
	}
	for _, name := range []string{"LICENSE", "NOTICE", "third_party/GO-LICENSE"} {
		if err := write(filepath.Join(b.site, name), b.controls[name]); err != nil {
			return err
		}
	}
	return nil
}

func (b *apiPolicyBuild) publish() error {
	controlFiles := map[string]apiPolicyFile{}
	for name, raw := range b.controls {
		controlFiles[name] = apiPolicyDescribe(raw)
	}
	wasmFile := apiPolicyDescribe(b.wasm)
	wasmFile.Path = "./program.wasm"
	buildRecord := map[string]any{"schema": "gooo/browser-api-policy-build/v1", "status": "NATIVE_FIXTURES_AND_WASM_BUILD_PASS", "scope": "source_local_generated_Gooo_native_to_Wasm_build_not_browser_execution", "source": map[string]any{"head": b.checkout.Head, "tree": b.checkout.Tree, "dirty": b.checkout.Dirty, "native_sources": b.source.Files, "control_sources": controlFiles, "source_before_after_equal": true, "historical_git_objects_checked": false}, "go": b.goIdentity, "native_executable": b.nativeFile, "temporary_main_module_sha256": digest([]byte(b.module)), "explicit_local_replacements": 2, "temporary_sources_native_binary_module_cache_removed": true, "normal_build_cache_retained_not_copied": true, "wasm": wasmFile, "runtime": b.runtimeFile, "native_fixtures": b.fixtureFile, "native_fixture_receipt": b.receiptFile, "acceptance": b.acceptance, "published_source_subset": b.published, "verbose_process_logs_published": false, "limits": []string{"Build and fresh native literal controls only; actual Wasm/Worker/browser tests remain separate mandatory validation.", "Full original28/revision37 binding maps do not claim every sealed source was fetched by the browser.", "Current checkout head/tree/dirty observed; frozen33 verified from fixed byte contracts without historical Git objects."}}
	buildRaw, err := apiPolicyPutJSON(filepath.Join(b.asset, "build.json"), buildRecord, b.root)
	if err != nil {
		return err
	}
	buildFile := apiPolicyDescribe(buildRaw)
	buildFile.Path = "./build.json"
	bridgeFile := apiPolicyDescribe(b.controls["tools/api-policy-browser-bridge.go.txt"])
	bridgeFile.Path = "./source/api-policy-browser-bridge.go.txt"
	workerFile := b.uiFiles["api-policy-worker.js"]
	workerFile.Path = "../api-policy-worker.js"
	packageManifest := map[string]any{"schema": "gooo/browser-api-policy-revision/v1", "scope": "unsigned_historical_hypothetical_comparison", "source": map[string]any{"head": b.checkout.Head, "tree": b.checkout.Tree, "dirty": b.checkout.Dirty, "tracked_file_count": apiPolicyCurrentSourceFiles, "native_sources": b.source.Files, "ui_files": b.uiFiles, "native_fixtures": b.fixtureFile, "native_fixture_receipt": b.receiptFile, "published_sources": b.published, "historical_git_objects_checked": false}, "bridge": bridgeFile, "build_verification": buildFile, "files": map[string]apiPolicyFile{"worker.js": workerFile, "program.wasm": wasmFile, "wasm_exec.js": b.runtimeFile}, "expected_info": apiPolicyExpectedInfo(b.source), "limits": map[string]any{"request_deadline_ms": 120000, "manifest_bytes": 512 << 10, "worker_source_bytes": 128 << 10, "note": "Operational deadline only; cancellation/timeout gives no semantic result."}}
	_, err = apiPolicyPutJSON(filepath.Join(b.asset, "manifest.json"), packageManifest, b.root)
	return err
}
