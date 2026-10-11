// Build a static demo from an unmodified, verified Gooo composition.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const manifest = "browser-demo.workspace.json"
const casesPath = "examples/browser-demo/cases.json"

type lockfile struct {
	CompilerRevision string `json:"compiler_revision"`
	CompilerVersion  string `json:"compiler_version"`
	GoVersion        string `json:"go_version"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "new build directory beneath this repository")
	compilerRoot := flag.String("compiler-root", ".compiler", "clean pinned compiler checkout used only by the browser codegen overlay")
	codegenOnly := flag.Bool("codegen-only", false, "build and verify only the editable browser body-codegen page")
	flag.Parse()
	if *out == "" {
		return fmt.Errorf("provide --out for a new build directory")
	}
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("output must be beneath repository root")
	}
	var lock lockfile
	if err := readJSON("upstream.json", &lock); err != nil {
		return err
	}
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("require %s, got %s", lock.GoVersion, runtime.Version())
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	// Isolate generated Go from the parent repository's native ./... checks.
	parent, err := filepath.Rel(dir, root)
	if err != nil {
		return err
	}
	// Absolute replacement paths survive -trimpath in Go build information.
	module := fmt.Sprintf("module github.com/kimjooyoon/gooo-go-ports/browserbuild\n\ngo %s\n\nrequire github.com/kimjooyoon/gooo-go-ports v0.0.0\nreplace github.com/kimjooyoon/gooo-go-ports => %q\n", strings.TrimPrefix(lock.GoVersion, "go"), filepath.ToSlash(parent))
	if err := write(filepath.Join(dir, "go.mod"), []byte(module)); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	compilerRaw, err := command(ctx, nil, "./.gooo", "version", "--build", "--json")
	if err != nil {
		return err
	}
	var build struct {
		Revision string `json:"compiler_source_sha"`
		Version  string `json:"version"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
		Modified string `json:"vcs_modified"`
		Status   string `json:"source_status"`
	}
	if err := json.Unmarshal(compilerRaw, &build); err != nil {
		return err
	}
	if build.Revision != lock.CompilerRevision || build.Version != lock.CompilerVersion || build.Go != lock.GoVersion || build.Native != lock.GoVersion || build.Modified != "false" || build.Status != "CLEAN_VCS" {
		return fmt.Errorf("compiler identity does not match upstream.json")
	}
	if err := os.WriteFile(filepath.Join(dir, "compiler.json"), compilerRaw, 0644); err != nil {
		return err
	}
	site := filepath.Join(dir, "site")
	work := filepath.Join(dir, "build")
	for _, p := range []string{site, work} {
		if err := os.Mkdir(p, 0755); err != nil {
			return err
		}
	}
	if *codegenOnly {
		if err := buildCodegenDemo(ctx, site, dir, lock, *compilerRoot); err != nil {
			return err
		}
		fmt.Printf("Built %s/codegen; actual compiler-library code generation, no generated-Go execution\n", site)
		return nil
	}
	var ws struct {
		Packages []struct {
			Sources []string `json:"sources"`
		} `json:"packages"`
	}
	if err := readJSON(manifest, &ws); err != nil {
		return err
	}
	sources := map[string]string{}
	bindings := map[string]string{}
	for _, p := range ws.Packages {
		for _, name := range p.Sources {
			if !filepath.IsLocal(name) {
				return fmt.Errorf("non-local source %s", name)
			}
			content, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			sources[name] = digest(content)
			// The pinned package image hashes a JSON string, not raw source bytes.
			encoded, err := json.Marshal(string(content))
			if err != nil {
				return err
			}
			bindings[name] = digest(encoded)
			if err := write(filepath.Join(site, "source", name), content); err != nil {
				return err
			}
		}
	}
	generated, evidence, err := executeBrowserBatches(ctx, site, lock, bindings)
	if err != nil {
		return err
	}
	// These bytes are copied exactly. Never edit a generated function or package clause.
	if err := write(filepath.Join(work, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	if err := write(filepath.Join(site, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	if err := copyFile("tools/browser-bridge.go.txt", filepath.Join(work, "bridge.go")); err != nil {
		return err
	}
	_, err = command(ctx, []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}, "go", "-C", dir, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", filepath.Join(site, "demo.wasm"), "./build")
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(runtime.GOROOT(), "lib/wasm/wasm_exec.js"), filepath.Join(site, "wasm_exec.js")); err != nil {
		return err
	}
	if err := buildCropDemo(ctx, site, dir, lock); err != nil {
		return err
	}
	if err := buildTutorialDemo(ctx, site, dir, lock); err != nil {
		return err
	}
	if err := buildCodegenDemo(ctx, site, dir, lock, *compilerRoot); err != nil {
		return err
	}
	for _, name := range []string{"google878ae9014add7065.html", "sitemap.xml", "index.html", "fixed-point-bounds.html", "app.js", "share.js", "codepoint.js", "style.css", "text-inspector.html", "text-inspector.js", "text-inspector.css", "text-input.js", "text-inspector-worker.js", "text-inspector-protocol.js", "text-inspection-report.js", "crop-planner.html", "crop-planner.js", "crop-planner.css", "crop-planner-protocol.js", "tutorial.html", "tutorial.css", "tutorial.js", "tutorial-protocol.js", "tutorial-worker.js", "source-view.html", "source-view.css", "source-view.js", "source-view.json"} {
		if err := copyFile(filepath.Join("site", name), filepath.Join(site, name)); err != nil {
			return err
		}
	}
	for _, name := range []string{"LICENSE", "NOTICE", "third_party/GO-LICENSE", "upstream.json", manifest} {
		if err := copyFile(name, filepath.Join(site, name)); err != nil {
			return err
		}
	}
	_, err = command(ctx, nil, "go", "run", "./cmd/verify", "--browser-fixtures", filepath.Join(site, "fixtures.json"))
	if err != nil {
		return err
	}
	_, err = command(ctx, nil, "go", "run", "./cmd/text-inspector-fixtures", "--out", filepath.Join(site, "text-inspector-fixtures.json"))
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
	wasm, err := os.ReadFile(filepath.Join(site, "demo.wasm"))
	if err != nil {
		return err
	}
	metadata := map[string]any{
		"schema": "gooo/browser-demo/v2", "compiler_revision": lock.CompilerRevision,
		"compiler_version": lock.CompilerVersion, "go_version": lock.GoVersion,
		"source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(status) > 0,
		"source_sha256": sources, "generated_sha256": digest([]byte(generated)),
		"wasm_sha256": digest(wasm), "wasm_bytes": len(wasm), "execution_sha256": evidence.Batches[0].ExecutionSHA,
		"native_cases": evidence.Cases, "native_batches": len(evidence.Batches),
		"entry_id": browserDispatchID, "reply_entity_id": browserReplyID,
		"input_entity_ids": browserInputEntities[:],
	}
	indexRaw, err := os.ReadFile(filepath.Join(site, "evidence-index.json"))
	if err != nil {
		return err
	}
	metadata["evidence_index_sha256"] = digest(indexRaw)
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := write(filepath.Join(site, "metadata.json"), append(b, '\n')); err != nil {
		return err
	}
	fmt.Printf("Built %s (%d bytes of Wasm); generated Go preserved byte for byte\n", site, len(wasm))
	return nil
}

func command(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	if name == "go" {
		name = filepath.Join(runtime.GOROOT(), "bin", "go")
	}
	c := exec.CommandContext(ctx, name, args...)
	c.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "PATH="+filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	c.Env = append(c.Env, env...)
	var stderr strings.Builder
	c.Stderr = &stderr
	b, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s\n%s", name, args, err, stderr.String(), b)
	}
	return b, nil
}
func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func readJSON(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
func write(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return write(to, b)
}
