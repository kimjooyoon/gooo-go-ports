package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Optional focused real-compiler check. Unlike a representative smoke test,
// this preserves every ordered ETag input. The full verifier is still required.
func TestETagNativeFullCorpus(t *testing.T) {
	compiler, out := os.Getenv("GOOO_ETAG_TEST_COMPILER"), os.Getenv("GOOO_ETAG_TEST_OUT")
	if compiler == "" && out == "" {
		t.Skip("set GOOO_ETAG_TEST_COMPILER and GOOO_ETAG_TEST_OUT for all247 native inputs")
	}
	if !filepath.IsAbs(compiler) || !filepath.IsAbs(out) {
		t.Fatal("explicit absolute compiler and new output paths required")
	}
	if err := os.Mkdir(out, 0755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var lock upstream
	if err := readJSON(filepath.Join(root, "upstream.json"), &lock); err != nil {
		t.Fatal(err)
	}
	if runtime.Version() != lock.GoVersion {
		t.Fatal("independent oracle Go version differs", runtime.Version())
	}
	for name, want := range lock.SourceSHA {
		got, err := digestFile(filepath.Join(runtime.GOROOT(), name))
		if err != nil || got != want {
			t.Fatal("original Go source differs", name, err)
		}
	}
	if err := verifyExternalOracles(root, lock.ExternalSources); err != nil {
		t.Fatal(err)
	}
	if err := verifyHTTPGutsRuntimeDependencies(); err != nil {
		t.Fatal(err)
	}
	added, err := etagCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 3 || len(added[0].Cases) != 192 || len(added[1].Cases) != 31 || len(added[2].Cases) != 24 {
		t.Fatal("all192 scanner,31 strong and24 Report rows required")
	}
	catalog, err := freezeVerificationCatalog(filepath.Join(root, "verification-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	raw, err := execute(ctx, compiler, filepath.Join(out, "compiler.json"), "version", "--build", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var build struct {
		Revision string `json:"compiler_source_sha"`
		Version  string `json:"version"`
		Modified string `json:"vcs_modified"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
	}
	if err := json.Unmarshal(raw, &build); err != nil || build.Revision != lock.CompilerRevision || build.Version != lock.CompilerVersion || build.Modified != "false" || build.Go != lock.GoVersion || build.Native != lock.GoVersion {
		t.Fatal("clean pinned compiler identity required", err)
	}
	profile, err := parseNativeCaseProfile("uuid-and-literal-report-compact-v1", false)
	if err != nil {
		t.Fatal(err)
	}
	workspaceProfile, err := parseNativeWorkspaceProfile("dependency-closure-v1", false)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := newNativeBuildCache(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cache.close(); err != nil {
			t.Error(err)
		}
	}()
	verifier := newNativeSuiteVerifier(ctx, compiler, out, lock, cache)
	verifier.caseProfile = profile
	var observations []observation
	for _, s := range added {
		selected, err := workspaceProfile.workspaceForSuite(catalog, s)
		if err != nil {
			t.Fatal(err)
		}
		result, err := verifier.verify(ctx, s, selected)
		if err != nil {
			t.Fatal(err)
		}
		observations = append(observations, result.Observation)
		t.Logf("%s:%s %d/%d; %d batches; saved replay identical", s.Package, s.Activity, result.Observation.Passed, len(s.Cases), result.Observation.Batches)
	}
	if err := save(filepath.Join(out, "etag-focused-summary.json"), map[string]any{
		"schema": "gooo/etag-focused-native/v1", "runs": observations,
		"cases": 247, "scope": "All247 fixed ETag rows only; does not replace the complete verifier, boundary probes or CI.",
	}); err != nil {
		t.Fatal(err)
	}
}
