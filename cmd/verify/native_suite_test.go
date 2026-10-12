package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type fakeNativeCommand struct {
	templates map[string][]byte
	before    func(context.Context, string, string) error
	mutate    func(string, map[string]any)
	mu        sync.Mutex
	calls     map[string][]string
}

// The fake command saves source-bound synthetic receipts. No compiler or
// generated Go executes; the production inspection and saved-replay binding
// still run for every batch in nativeSuiteVerifier.verify.
func (f *fakeNativeCommand) execute(ctx context.Context, _ string, path string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, action := filepath.Dir(path), args[1]
	if f.before != nil {
		if err := f.before(ctx, dir, action); err != nil {
			return nil, err
		}
	}
	var receipt map[string]any
	if err := json.Unmarshal(f.templates[filepath.Dir(dir)], &receipt); err != nil {
		return nil, err
	}
	manifest, err := digestFile(filepath.Join(dir, "gooo.workspace.json"))
	if err != nil {
		return nil, err
	}
	cases, err := digestFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		return nil, err
	}
	receipt["manifest_digest"], receipt["cases_digest"] = "sha256:"+manifest, "sha256:"+cases
	var document struct {
		Cases []json.RawMessage `json:"cases"`
	}
	if err := readJSON(filepath.Join(dir, "cases.json"), &document); err != nil {
		return nil, err
	}
	result := receipt["result"].(map[string]any)
	runtime := result["runtime"].(map[string]any)
	first, err := json.Marshal(runtime["traces"].([]any)[0])
	if err != nil {
		return nil, err
	}
	traces := make([]any, len(document.Cases))
	for index := range traces {
		var trace map[string]any
		if err := json.Unmarshal(first, &trace); err != nil {
			return nil, err
		}
		trace["case_index"] = index
		traces[index] = trace
	}
	runtime["traces"], runtime["finite_total"], runtime["finite_passed"] = traces, len(traces), len(traces)
	if action == "replay" {
		original, err := digestFile(filepath.Join(dir, "execution.json"))
		if err != nil {
			return nil, err
		}
		receipt["replayed_from_sha256"] = "sha256:" + original
		result["replay"] = map[string]any{"model_calls": 0}
	}
	if f.mutate != nil {
		f.mutate(action, receipt)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.calls[dir] = append(f.calls[dir], action)
	f.mu.Unlock()
	return raw, nil
}

func fakeNativeWorkspace(t *testing.T, s suite, identity expectedIdentity) suiteWorkspace {
	t.Helper()
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../" + contract.Source)
	if err != nil {
		t.Fatal(err)
	}
	w := suiteWorkspace{Names: []string{contract.Source}, Sources: map[string][]byte{contract.Source: source}, Identities: identity.Sources}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	w.Manifest.Packages = []sourcePackage{{s.Package, []string{contract.Source}}}
	return w
}

func repeatNativeCase(s suite, count int) suite {
	first := s.Cases[0]
	s.Cases = make([]testCase, count)
	for index := range s.Cases {
		s.Cases[index] = first
	}
	return s
}

func TestParallelNativeSuitesKeepBatchPathsSourcesAndReplayIsolated(t *testing.T) {
	root := t.TempDir()
	raw, decode, identity := fixture(t)
	encoded, encode, encodeIdentity := contractFixture(t, suite{Package: "go/unicode/utf16", Activity: "EncodeRune", Cases: []testCase{{Inputs: []any{int64(0)}, Expected: map[string]any{"high": int64(65533), "low": int64(65533)}}}})
	suites := []suite{repeatNativeCase(decode, 129), encode}
	workspaces := []suiteWorkspace{fakeNativeWorkspace(t, decode, identity), fakeNativeWorkspace(t, encode, encodeIdentity)}
	if err := validateSuiteEvidenceDirectories(root, suites); err != nil {
		t.Fatal(err)
	}
	fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, decode): raw, suiteEvidenceDirectory(root, encode): encoded}, calls: map[string][]string{}}
	started, release := make(chan struct{}, 2), make(chan struct{})
	fake.before = func(ctx context.Context, dir, action string) error {
		if action == "execute" && filepath.Base(dir) == "000" {
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	verifier := newNativeSuiteVerifier(workerTestContext(t), "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
	verifier.execute = fake.execute
	ctx := workerTestContext(t)
	finished := make(chan struct{})
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuites(ctx, suites, 2, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
			return verifier.verify(ctx, s, workspaces[index])
		})
		close(finished)
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("suite callbacks did not overlap")
		}
	}
	close(release)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("suite callbacks did not join")
	}
	if err != nil || len(results) != 2 || results[0].Observation.Passed != 129 || results[0].Observation.Batches != 2 || results[1].Observation.Passed != 1 {
		t.Fatal("batch coverage changed", results, err)
	}
	for index, s := range suites {
		for batch := range results[index].Observation.Batches {
			dir := filepath.Join(suiteEvidenceDirectory(root, s), fmt.Sprintf("%03d", batch))
			if !reflect.DeepEqual(fake.calls[dir], []string{"execute", "replay"}) {
				t.Fatal("batch execution/replay order or paths changed", dir, fake.calls[dir])
			}
			for name, original := range workspaces[index].Sources {
				copied, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(copied) != string(original) {
					t.Fatal("frozen source changed", name, err)
				}
			}
		}
	}
}

func TestNativeSuiteRejectsFailedUnsupportedAndUnboundReplay(t *testing.T) {
	raw, s, identity := fixture(t)
	for _, decision := range []string{"FAIL", "UNSUPPORTED", "PASS-with-wrong-replay"} {
		t.Run(decision, func(t *testing.T) {
			root := t.TempDir()
			fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
			fake.mutate = func(action string, receipt map[string]any) {
				if decision == "PASS-with-wrong-replay" {
					if action == "replay" {
						receipt["replayed_from_sha256"] = "sha256:" + strings.Repeat("0", 64)
					}
				} else {
					receipt["decision"] = decision
				}
			}
			verifier := newNativeSuiteVerifier(workerTestContext(t), "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
			verifier.execute = fake.execute
			result, err := verifier.verify(workerTestContext(t), s, fakeNativeWorkspace(t, s, identity))
			if err == nil || result.Observation.Passed != 0 {
				t.Fatal("invalid receipt became a passed observation", result, err)
			}
		})
	}
}

func TestNativeSuiteFailureCancellationFinishesActiveBatch(t *testing.T) {
	raw, s, identity := fixture(t)
	s = repeatNativeCase(s, 129)
	root := t.TempDir()
	parent := workerTestContext(t)
	worker, cancel := context.WithCancel(parent)
	defer cancel()
	fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
	fake.before = func(ctx context.Context, _ string, action string) error {
		if action == "execute" {
			cancel()
		}
		return ctx.Err()
	}
	verifier := newNativeSuiteVerifier(parent, "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
	verifier.execute = fake.execute
	result, err := verifier.verify(worker, s, fakeNativeWorkspace(t, s, identity))
	first := filepath.Join(suiteEvidenceDirectory(root, s), "000")
	if !errors.Is(err, context.Canceled) || result.Observation.Passed != 0 || !reflect.DeepEqual(fake.calls[first], []string{"execute", "replay"}) || len(fake.calls) != 1 {
		t.Fatal("cancelled suite interrupted its active batch or started a new one", result, err, fake.calls)
	}
	if parent.Err() != nil {
		t.Fatal("worker cancellation affected parent commands", parent.Err())
	}
}

func TestParallelNativeSuitesRetainEveryExhaustiveCheckOnce(t *testing.T) {
	root := t.TempDir()
	ctx := workerTestContext(t)
	fake := fakeNativeCommand{templates: map[string][]byte{}, calls: map[string][]string{}}
	var suites []suite
	var workspaces []suiteWorkspace
	var lock upstream
	for _, original := range allNativeSuites(t) {
		if !((original.Package == "go/unicode" && (original.Activity == "SimpleFold" || original.Activity == "To" || original.Activity == "ToUpper" || original.Activity == "ToLower" || original.Activity == "ToTitle")) ||
			(original.Package == "go/image/color" && (original.Activity == "RGBToYCbCr" || original.Activity == "YCbCrToRGB"))) {
			continue
		}
		raw, s, identity := contractFixture(t, original)
		fake.templates[suiteEvidenceDirectory(root, s)] = raw
		workspaces = append(workspaces, fakeNativeWorkspace(t, s, identity))
		suites = append(suites, repeatNativeCase(s, 129))
		lock.CompilerRevision, lock.GoVersion = identity.CompilerRevision, identity.GoVersion
	}
	cache, err := newNativeBuildCache(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cache.close(); err != nil {
			t.Error(err)
		}
	})
	verifier := newNativeSuiteVerifier(ctx, "unused", root, lock, cache)
	verifier.execute = fake.execute
	var mu sync.Mutex
	checks := map[string]int{}
	record := func(name, dir string, raw []byte, gotCache *nativeBuildCache) error {
		if gotCache != cache || len(raw) == 0 || filepath.Dir(dir) != root {
			return fmt.Errorf("exhaustive context/cache/evidence binding changed")
		}
		if _, err := gotCache.directory(""); err != nil {
			return err
		}
		mu.Lock()
		checks[name]++
		mu.Unlock()
		return nil
	}
	verifier.simpleFold = func(_ context.Context, dir string, raw []byte, generated string, cache *nativeBuildCache) (simpleFoldExhaustiveReport, error) {
		return expectedSimpleFoldExhaustive(generated), record("SimpleFold", dir, raw, cache)
	}
	verifier.ycbcr = func(_ context.Context, dir string, raw []byte, generated, operation string, cache *nativeBuildCache) (ycbcrExhaustiveReport, error) {
		if err := record(operation, dir, raw, cache); err != nil {
			return ycbcrExhaustiveReport{}, err
		}
		return expectedYCbCrExhaustive(generated, operation)
	}
	verifier.caseMapping = func(_ context.Context, dir string, raw []byte, generated, operation string, cache *nativeBuildCache) (caseMappingExhaustiveReport, error) {
		if err := record(operation, dir, raw, cache); err != nil {
			return caseMappingExhaustiveReport{}, err
		}
		return expectedCaseMappingExhaustive(generated, operation)
	}
	results, err := runNativeSuites(ctx, suites, 2, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
		return verifier.verify(ctx, s, workspaces[index])
	})
	if err != nil || len(results) != 7 || len(checks) != 7 {
		t.Fatal("exhaustive roster changed", err, len(results), checks)
	}
	for index, result := range results {
		if checks[suites[index].Activity] != 1 || result.Observation.Passed != 129 || result.Observation.Batches < 2 ||
			(result.SimpleFold == nil && result.YCbCr == nil && result.CaseMapping == nil) {
			t.Fatal("exhaustive check repeated, skipped, or lost from result", suites[index].Activity, result)
		}
	}
}
