//go:build gooo_shard_native_smoke

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

const ciShardSmokeCacheBudget int64 = 512 << 20

type ciShardSmokeCache struct {
	Path              string `json:"path"`
	PeakObservedBytes int64  `json:"peak_observed_bytes"`
	Samples           int    `json:"samples"`
	ClosedAndRemoved  bool   `json:"closed_and_removed"`
	owner             os.FileInfo
}

// Only the fresh test-owned TMPDIR is observed. The production executor owns
// creation and removal of its native cache; the test never scans global temp or
// cache directories, and cancels the run if the observed budget is exceeded.
func ciShardSmokeObserveCache(ctx context.Context, cancel context.CancelFunc, directory string, stop <-chan struct{}, done chan<- struct{}, observed *[]ciShardSmokeCache, failure *error) {
	defer close(done)
	indices := map[string]int{}
	scan := func() error {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "gooo-native-build-cache-") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("invalid owned runner cache: %s", path)
			}
			index, found := indices[path]
			if !found {
				index = len(*observed)
				indices[path] = index
				*observed = append(*observed, ciShardSmokeCache{Path: path, owner: info})
			} else if !os.SameFile((*observed)[index].owner, info) {
				return fmt.Errorf("owned runner cache identity changed")
			}
			var size int64
			err = filepath.WalkDir(path, func(name string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("symlink in owned cache")
				}
				if !entry.IsDir() {
					info, err := entry.Info()
					if err != nil {
						return err
					}
					size += info.Size()
				}
				if size > ciShardSmokeCacheBudget {
					return fmt.Errorf("runner cache exceeds %d-byte smoke budget", ciShardSmokeCacheBudget)
				}
				return nil
			})
			if os.IsNotExist(err) {
				continue
			} // Production close can race this observation.
			if err != nil {
				return err
			}
			cache := &(*observed)[index]
			cache.Samples++
			if size > cache.PeakObservedBytes {
				cache.PeakObservedBytes = size
			}
		}
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := scan(); err != nil {
			*failure = err
			cancel()
			return
		}
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func ciShardSmokeFixture(t *testing.T, repository string) (ciShardPlan, ciShardNativeInventory, []int64) {
	t.Helper()
	var lock upstream
	if err := readJSON(filepath.Join(repository, "upstream.json"), &lock); err != nil {
		t.Fatal(err)
	}
	if runtime.Version() != lock.GoVersion || lock.CompilerRevision != uuidCompactCompilerRevision {
		t.Fatal("smoke requires pinned Go/compiler839")
	}
	oracle, err := digestFile(filepath.Join(runtime.GOROOT(), "src/net/http/status.go"))
	if err != nil || oracle != lock.SourceSHA["src/net/http/status.go"] {
		t.Fatal("independent Go HTTP oracle source differs", err)
	}
	codes := []int64{-1, 0, 99, 100, 103, 200, 204, 301, 418, 429, 500, 511, math.MaxInt64}
	s := suite{Package: "go/net/http", Activity: "StatusText", Domain: "13 independent Go net/http.StatusText values; local native connection smoke only"}
	for _, code := range codes {
		s.Cases = append(s.Cases, testCase{Inputs: []any{code}, Expected: http.StatusText(int(code))})
	}
	const sourceName = "ports/net/http.gooo"
	source, err := os.ReadFile(filepath.Join(repository, sourceName))
	if err != nil {
		t.Fatal(err)
	}
	packages := []sourcePackage{{Path: s.Package, Sources: []string{sourceName}}}
	identities, err := snapshotSourceIdentities(packages, map[string][]byte{sourceName: source})
	if err != nil {
		t.Fatal(err)
	}
	workspace := suiteWorkspace{Names: []string{sourceName}, Sources: map[string][]byte{sourceName: source}, Identities: identities}
	workspace.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	workspace.Manifest.Entry.Package = s.Package
	workspace.Manifest.Entry.Activity = s.Activity
	workspace.Manifest.Packages = packages
	manifest, err := json.MarshalIndent(workspace.Manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, '\n')
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	anchors := []ciShardSourceAnchor{{Name: sourceName, RawSHA256: contentDigest(source), Identity: identities[sourceName]}}
	anchorSHA, err := canonicalContractDigest(anchors)
	if err != nil {
		t.Fatal(err)
	}
	c := &NativeVerificationContract{Schema: "gooo/local-native-smoke-ledger/v1", Sources: []ContractSource{{Path: sourceName, Mode: "100644"}}, Suites: []ContractSuite{{Index: 0, Package: s.Package, Activity: s.Activity, Cases: int64(len(codes)), Batches: 1, SourcePaths: []string{sourceName}}}, TotalCases: int64(len(codes)), TotalBatches: 1}
	modes, err := ciShardExpectedSourceModes(c)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]ciShardSource{}
	for name, mode := range modes {
		raw, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = ciShardSource{Mode: mode, Bytes: int64(len(raw)), SHA256: contentDigest(raw)}
	}
	// Deliberately synthetic run/tree identity: this fixture never asserts that
	// a current clean GitHub/CLI checkout or the full 82-suite ledger was tested.
	run := ciShardRun{Repository: "local/native-shard-smoke", RunID: "1", Attempt: 1, TestedCommit: strings.Repeat("a", 40), TestedTree: strings.Repeat("b", 40), CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion, Workers: 3}
	anchor := ciShardWorkspace{OriginalIndex: 0, ManifestSHA256: contentDigest(manifest), SourceSetSHA256: anchorSHA, EntryProducer: contract.ID, Sources: anchors}
	plan, err := newCIShardPlan(c, contentDigest([]byte("explicit local 13-input StatusText smoke ledger; not the production full ledger")), run, 1, sources, []ciShardWorkspace{anchor})
	if err != nil {
		t.Fatal(err)
	}
	catalog := verificationCatalog{Schema: verificationCatalogSchema, Packages: packages}
	catalogRaw, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	inventory := ciShardNativeInventory{Suites: []suite{s}, Lock: lock, Workspaces: []suiteWorkspace{workspace}, Catalog: frozenVerificationCatalog{Catalog: catalog, Raw: catalogRaw, Sources: workspace.Sources, Hashes: map[string]string{sourceName: strings.TrimPrefix(contentDigest(source), "sha256:")}}}
	return plan, inventory, codes
}

// This opt-in layer checks real compiler/receipt compatibility for one bounded
// injected fixture. It is excluded from default tests and adds no observer skip.
// It is not the current full CLI, full corpus, CI acceptance, or a speed test.
func TestCIShardActualNativeStatusTextSmoke(t *testing.T) {
	compiler, output := os.Getenv("GOOO_SHARD_SMOKE_COMPILER"), os.Getenv("GOOO_SHARD_SMOKE_OUT")
	if compiler == "" || output == "" {
		t.Fatal("explicit GOOO_SHARD_SMOKE_COMPILER and fresh GOOO_SHARD_SMOKE_OUT are required; smoke never skips")
	}
	compiler, err := filepath.Abs(compiler)
	if err != nil {
		t.Fatal(err)
	}
	output, err = filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal("fresh smoke output required", err)
	}
	outputOwner, err := os.Lstat(output)
	if err != nil {
		t.Fatal(err)
	}
	receipt := map[string]any{"schema": "gooo/local-native-shard-smoke/v1", "state": "STARTED", "scope": "one injected StatusText fixture only; not current full CLI/full82/CI acceptance or performance", "synthetic_run_identity": true, "current_cli_clean_source_proven": false, "native_acceptance": false, "producer_authenticated": false, "full_corpus_consumed": false, "compiler": compiler}
	defer func() {
		if t.Failed() {
			receipt["state"] = "FAILED_FIRST_OBSERVATION_PRESERVED"
		}
		raw, e := json.MarshalIndent(receipt, "", "  ")
		if e == nil {
			e = os.WriteFile(filepath.Join(output, "result.json"), append(raw, '\n'), 0600)
		}
		if e != nil {
			t.Errorf("preserve smoke result: %v", e)
		}
	}()
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	plan, inventory, codes := ciShardSmokeFixture(t, repository)
	receipt["input_codes"], receipt["cases"], receipt["batches"], receipt["suites"] = codes, 13, 1, 1
	receipt["plan_sha256"], receipt["context"] = plan.PlanSHA256, plan.Run
	if err := writeNewCIShardJSON(filepath.Join(output, "plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(output, "native-tmp")
	if err := os.Mkdir(tmp, 0700); err != nil {
		t.Fatal(err)
	}
	tmpOwner, err := os.Lstat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	stop, done := make(chan struct{}), make(chan struct{})
	var observed []ciShardSmokeCache
	var cacheErr error
	go ciShardSmokeObserveCache(ctx, cancel, tmp, stop, done, &observed, &cacheErr)
	evidence := filepath.Join(output, "evidence")
	partial, executionErr := executeCIShardNative(ctx, compiler, evidence, plan, 0, inventory)
	close(stop)
	<-done // All executor callbacks and the cache observer are joined.
	for i := range observed {
		_, err := os.Lstat(observed[i].Path)
		observed[i].ClosedAndRemoved = os.IsNotExist(err)
	}
	receipt["runner_caches"], receipt["runner_cache_budget_bytes"], receipt["joined"] = observed, ciShardSmokeCacheBudget, partial.Joined
	if partial.Schema != "" {
		if err := writeNewCIShardJSON(filepath.Join(evidence, "partial.json"), partial); err != nil {
			t.Fatal(err)
		}
	}
	if executionErr != nil || cacheErr != nil {
		receipt["execution_error"] = fmt.Sprint(executionErr)
		receipt["cache_error"] = fmt.Sprint(cacheErr)
		t.Fatal("first actual native smoke failed", executionErr, cacheErr)
	}
	if !partial.Joined || partial.State != "SUCCEEDED" || len(partial.Suites) != 1 || len(partial.Suites[0].Batches) != 1 {
		t.Fatal("actual native shard did not complete bounded fixture", partial.State)
	}
	if len(observed) != 1 || observed[0].Samples == 0 || !observed[0].ClosedAndRemoved || observed[0].PeakObservedBytes > ciShardSmokeCacheBudget {
		t.Fatal("runner cache bound/close was not observed", observed)
	}
	compilerRaw, err := os.ReadFile(filepath.Join(evidence, "compiler.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCIShardCompilerIdentity(compilerRaw, inventory.Lock); err != nil {
		t.Fatal(err)
	}
	partialRaw, err := readCIShardFile(filepath.Join(evidence, "partial.json"), maxCIShardJSONBytes)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip ciShardPartial
	if err := decodeCIShardJSON(partialRaw, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(partial, roundtrip) {
		t.Fatal("saved actual partial changed on JSON round trip")
	}
	reduced, err := reduceCIShardPartials(plan, []ciShardPartial{roundtrip})
	if err != nil {
		t.Fatal(err)
	}
	if reduced.NativeAcceptance || reduced.RawReceiptsInspected || reduced.State != "CONTRACT_COMPLETE_NATIVE_UNCHECKED" {
		t.Fatal("metadata reducer inflated acceptance scope")
	}
	if err := writeNewCIShardJSON(filepath.Join(output, "metadata-reduction.json"), reduced); err != nil {
		t.Fatal(err)
	}
	actual, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, evidence, roundtrip.Suites[0])
	if err != nil || !reflect.DeepEqual(actual, roundtrip.Suites[0]) {
		t.Fatal("saved actual native evidence differs", err)
	}
	dir := ciShardBatchDirectory(evidence, 0, 0, 0)
	cases, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.MarshalIndent(caseDocument(inventory.Suites[0]), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	expected = append(expected, '\n')
	if !bytes.Equal(cases, expected) {
		t.Fatal("actual native smoke changed independent original case bytes")
	}
	execution, err := os.ReadFile(filepath.Join(dir, "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := os.ReadFile(filepath.Join(dir, "replay.json"))
	if err != nil {
		t.Fatal(err)
	}
	var original, rerun envelope
	if err := json.Unmarshal(execution, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replay, &rerun); err != nil {
		t.Fatal(err)
	}
	if rerun.ReplayedFrom != contentDigest(execution) || len(original.Result.Runtime.Traces) != 13 || len(rerun.Result.Runtime.Traces) != 13 {
		t.Fatal("saved replay parent or original row population changed")
	}
	receipt["execution_receipt_sha256"], receipt["replay_receipt_sha256"], receipt["cases_sha256"], receipt["compiler_identity_sha256"] = contentDigest(execution), contentDigest(replay), contentDigest(cases), contentDigest(compilerRaw)
	receipt["compiler_execute_commands"], receipt["compiler_replay_commands"] = 1, 1
	receipt["native_runtime_contract"] = "each execute/replay receipt checks two fresh value-graph executions through the pinned engine; no separate OS-process counter is claimed"
	receipt["raw_receipts_inspected"], receipt["partial_roundtrip_equal"] = true, true
	// The executor has already removed its observed cache after joining. Remove
	// only this fresh test-owned TMPDIR after retaining its remaining file sizes.
	var remainingBytes int64
	remainingFiles := 0
	if err := filepath.WalkDir(tmp, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected link in owned smoke TMPDIR")
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			remainingBytes += info.Size()
			remainingFiles++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	currentOutput, err := os.Lstat(output)
	if err != nil || !os.SameFile(outputOwner, currentOutput) {
		t.Fatal("smoke output ownership changed", err)
	}
	currentTmp, err := os.Lstat(tmp)
	if err != nil || !os.SameFile(tmpOwner, currentTmp) {
		t.Fatal("smoke TMPDIR ownership changed", err)
	}
	if err := os.RemoveAll(tmp); err != nil {
		t.Fatal("remove joined owned smoke TMPDIR", err)
	}
	receipt["owned_tmp_cleanup"] = map[string]any{"path": tmp, "remaining_bytes_before": remainingBytes, "remaining_files_before": remainingFiles, "removed_after_all_joins": true}
	receipt["state"] = "LOCAL_NATIVE_FIXTURE_CHECKED_NOT_CI_ACCEPTANCE"
}
