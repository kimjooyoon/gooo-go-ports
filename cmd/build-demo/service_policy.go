package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const servicePolicyModule = "examples/agentmesh-service-policy"
const servicePolicyManifestSchema = "gooo/browser-service-policy/v1"

// Display the relevant unmodified sources, provenance and licenses. Native
// SourceBindings still covers all 89 original assets, including retained
// evidence; no extra oracle or application host is linked into this bridge.
var servicePolicySourceFiles = []string{
	"matcher.gooo", "impact.gooo", "refinement.gooo", "domain.gooo",
	"checked_matcher.gooo", "checked_impact.gooo", "checked_refinement.gooo",
	"model.go", "domain.go", "input.go", "bridge.go", "bindings.go", "adapter_generated.go",
	"internal/checked-matcher/generated.go", "internal/checked-impact/generated.go", "internal/checked-refinement/generated.go",
	"matcher-workspace.json", "impact-workspace.json", "refinement-workspace.json",
	"generation.json", "compiler.json", "source-pin.json", "go.mod",
	"upstream/LICENSE", "upstream/api/tar.go.txt", "upstream/api/network.go.txt", "upstream/api/tar_test.go.txt",
	"LICENSE-APACHE-2.0", "NOTICE.md",
	"cmd/browser/bridge.go", "cmd/browser/main_js.go", "cmd/browser/main_native.go",
}

type servicePolicyBindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	GenerationSHA256  string            `json:"generation_sha256"`
	GenerationOrigin  string            `json:"generation_origin"`
	Assets            map[string]string `json:"assets"`
}

type servicePolicyGeneration struct {
	Schema        string            `json:"schema"`
	Origin        string            `json:"origin"`
	Assets        map[string]string `json:"assets"`
	PublicSources map[string]string `json:"public_sources"`
}

type servicePolicyFile struct {
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type servicePolicyManifest struct {
	Schema           string                       `json:"schema"`
	CompilerRevision string                       `json:"compiler_revision"`
	GoVersion        string                       `json:"go_version"`
	SourceRevision   string                       `json:"source_revision"`
	SourceDirty      bool                         `json:"source_dirty"`
	GenerationSHA256 string                       `json:"generation_sha256"`
	Bindings         servicePolicyBindings        `json:"bindings"`
	WasmSHA256       string                       `json:"wasm_sha256"`
	WasmBytes        int                          `json:"wasm_bytes"`
	WasmExecSHA256   string                       `json:"wasm_exec_sha256"`
	WasmExecBytes    int                          `json:"wasm_exec_bytes"`
	Files            map[string]servicePolicyFile `json:"files"`
}

func servicePolicyDecode(raw []byte, target any) error {
	// Reject duplicate keys as well as fields unknown to the typed manifest.
	if _, err := cropDecode(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing service-policy JSON")
	}
	return nil
}

func servicePolicyLocalPath(name string) bool {
	return name != "." && filepath.IsLocal(name) && filepath.ToSlash(filepath.Clean(name)) == name && !strings.Contains(name, "\\")
}

func servicePolicyReadDocuments() (map[string][]byte, error) {
	documents := map[string][]byte{}
	read := func(name string) error {
		if !servicePolicyLocalPath(name) {
			return fmt.Errorf("non-local service-policy source: %s", name)
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		documents[name] = raw
		return nil
	}
	if err := read(servicePolicyModule + "/generation.json"); err != nil {
		return nil, err
	}
	var generation servicePolicyGeneration
	if err := servicePolicyDecode(documents[servicePolicyModule+"/generation.json"], &generation); err != nil {
		return nil, err
	}
	if len(generation.Assets) != 89 || len(generation.PublicSources) != 16 {
		return nil, fmt.Errorf("service-policy native asset roster changed")
	}
	for name := range generation.Assets {
		if !servicePolicyLocalPath(name) {
			return nil, fmt.Errorf("invalid native asset name")
		}
		if err := read(servicePolicyModule + "/" + name); err != nil {
			return nil, err
		}
	}
	for name := range generation.PublicSources {
		if err := read(name); err != nil {
			return nil, err
		}
	}
	for _, name := range servicePolicySourceFiles {
		if err := read(servicePolicyModule + "/" + name); err != nil {
			return nil, err
		}
	}
	return documents, nil
}

// Construct the expected native object from verified source bytes, without
// building a second executable or changing the pinned generation. The worker
// must compare this entire object to SourceBindings from the running Wasm.
func servicePolicyCheckDocuments(documents map[string][]byte, lock lockfile) (servicePolicyBindings, error) {
	bad := func() (servicePolicyBindings, error) {
		return servicePolicyBindings{}, fmt.Errorf("service-policy source or compiler binding differs")
	}
	raw := documents[servicePolicyModule+"/generation.json"]
	var generation servicePolicyGeneration
	if servicePolicyDecode(raw, &generation) != nil || generation.Schema != "gooo/agentmesh-native-generation/v1" || generation.Origin != "fresh_pinned_generation" || len(generation.Assets) != 89 || len(generation.PublicSources) != 16 {
		return bad()
	}
	for name, want := range generation.Assets {
		data := documents[servicePolicyModule+"/"+name]
		if !servicePolicyLocalPath(name) || len(data) == 0 || want != "sha256:"+digest(data) {
			return bad()
		}
	}
	for name, want := range generation.PublicSources {
		data := documents[name]
		if !servicePolicyLocalPath(name) || len(data) == 0 || want != "sha256:"+digest(data) {
			return bad()
		}
	}
	var compiler struct {
		Schema   string `json:"schema"`
		Source   string `json:"compiler_source_sha"`
		Revision string `json:"vcs_revision"`
		Status   string `json:"source_status"`
		Modified string `json:"vcs_modified"`
		Version  string `json:"version"`
		Go       string `json:"go_version"`
		NativeGo string `json:"native_go_required"`
	}
	if json.Unmarshal(documents[servicePolicyModule+"/compiler.json"], &compiler) != nil || compiler.Schema != "gooo/build-identity/v1" || compiler.Source != lock.CompilerRevision || compiler.Revision != lock.CompilerRevision || compiler.Status != "CLEAN_VCS" || compiler.Modified != "false" || compiler.Version != lock.CompilerVersion || compiler.Go != lock.GoVersion || compiler.NativeGo != lock.GoVersion {
		return bad()
	}
	return servicePolicyBindings{CompilerSourceSHA: compiler.Source, GenerationSHA256: "sha256:" + digest(raw), GenerationOrigin: generation.Origin, Assets: generation.Assets}, nil
}

func buildServicePolicyDemo(ctx context.Context, site, dir string, lock lockfile) error {
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("service-policy browser requires %s, got %s", lock.GoVersion, runtime.Version())
	}
	rel, err := filepath.Rel(dir, site)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("service-policy site must be beneath its build directory")
	}
	documents, err := servicePolicyReadDocuments()
	if err != nil {
		return err
	}
	bindings, err := servicePolicyCheckDocuments(documents, lock)
	if err != nil {
		return err
	}
	asset := filepath.Join(site, "service-policy")
	if err := os.MkdirAll(site, 0755); err != nil {
		return err
	}
	if err := os.Mkdir(asset, 0755); err != nil {
		return err
	}
	files := map[string]servicePolicyFile{}
	put := func(name string, raw []byte) error {
		if !servicePolicyLocalPath(name) || len(raw) == 0 {
			return fmt.Errorf("invalid service-policy output asset: %s", name)
		}
		if err := write(filepath.Join(asset, name), raw); err != nil {
			return err
		}
		files[name] = servicePolicyFile{SHA256: digest(raw), Bytes: len(raw)}
		return nil
	}
	for _, name := range servicePolicySourceFiles {
		if err := put("source/"+name, documents[servicePolicyModule+"/"+name]); err != nil {
			return err
		}
	}
	wasmRuntime, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "lib/wasm/wasm_exec.js"))
	if err != nil {
		return err
	}
	if err := put("wasm_exec.js", wasmRuntime); err != nil {
		return err
	}
	module, err := filepath.Abs(servicePolicyModule)
	if err != nil {
		return err
	}
	wasmPath, err := filepath.Abs(filepath.Join(asset, "program.wasm"))
	if err != nil {
		return err
	}
	_, err = command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0", "GOMAXPROCS=2", "GOPROXY=off", "GOSUMDB=off"}, "go", "-C", module, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", wasmPath, "./cmd/browser")
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		return err
	}
	if len(wasm) < 8 || !bytes.Equal(wasm[:8], []byte{0, 'a', 's', 'm', 1, 0, 0, 0}) {
		return fmt.Errorf("service-policy build did not produce WebAssembly")
	}
	files["program.wasm"] = servicePolicyFile{SHA256: digest(wasm), Bytes: len(wasm)}
	// Refuse concurrent source changes between the captured display/manifest
	// inputs and compilation. No clean-checkout claim is inferred from a hash.
	for name, original := range documents {
		current, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(current, original) {
			return fmt.Errorf("service-policy source changed during build: %s", name)
		}
	}
	for _, name := range []string{"service-policy.html", "service-policy.js", "service-policy.css", "service-policy-protocol.js", "service-policy-worker.js"} {
		if err := copyFile(filepath.Join("site", name), filepath.Join(site, name)); err != nil {
			return err
		}
	}
	revision, err := command(ctx, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	status, err := command(ctx, nil, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	manifest := servicePolicyManifest{
		Schema: servicePolicyManifestSchema, CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion,
		SourceRevision: strings.TrimSpace(string(revision)), SourceDirty: len(status) != 0,
		GenerationSHA256: bindings.GenerationSHA256, Bindings: bindings,
		WasmSHA256: files["program.wasm"].SHA256, WasmBytes: files["program.wasm"].Bytes,
		WasmExecSHA256: files["wasm_exec.js"].SHA256, WasmExecBytes: files["wasm_exec.js"].Bytes, Files: files,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	// This manifest describes build bytes, not a native execution receipt, an
	// authenticated observation, or a current authorization decision.
	return write(filepath.Join(asset, "manifest.json"), append(raw, '\n'))
}
