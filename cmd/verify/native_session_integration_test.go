//go:build linux || darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Four accepted-engine requests have60s ceilings each; two identity probes
// have15s ceilings each. Leave cleanup time before the caller's7-minute test
// limit. This pilot bound does not change the default verifier's30-minute limit.
const nativeSessionPilotTimeout = 5 * time.Minute

// This opt-in harness selects one unchanged original corpus suite. It uses the
// production scheduler, batch writer, receipt/oracle inspector and own-parent
// replay binding. It does not select the CLI/shard/full82 acceptance paths.
func TestNativeSessionActualRuneStartPilot(t *testing.T) {
	compiler, goTool, output := os.Getenv("GOOO_NATIVE_SESSION_COMPILER"), os.Getenv("GOOO_NATIVE_SESSION_GO"), os.Getenv("GOOO_NATIVE_SESSION_OUT")
	if os.Getenv("GOOO_NATIVE_SESSION_PILOT") != "1" || compiler == "" || goTool == "" || output == "" {
		t.Skip("set GOOO_NATIVE_SESSION_PILOT=1 and explicit GOOO_NATIVE_SESSION_COMPILER, GOOO_NATIVE_SESSION_GO, GOOO_NATIVE_SESSION_OUT for one original RuneStart session pilot")
	}
	if !filepath.IsAbs(compiler) || !filepath.IsAbs(goTool) || !filepath.IsAbs(output) {
		t.Fatal("explicit absolute compiler, Go and fresh output paths required")
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal("fresh persistent pilot output required", err)
	}
	report := map[string]any{
		"schema": "gooo/experimental-native-verifier-session-pilot/v1", "state": "STARTED",
		"scope":           "one original RuneStart suite through nativeSuiteVerifier.verify; no full82, compact-profile, shard or CI acceptance",
		"full82_accepted": false, "shard_accepted": false, "producer_authenticated": false,
		"compiler": compiler, "go_tool": goTool, "original_catalog_index": 3,
		"case_profile": "legacy", "workspace_profile": "legacy", "suite_workers": 1,
		"per_request_wait_resources":   map[string]any{"status": "NOT_OBSERVED", "reason": "nativeTiming and commandObserver are nil; persistent process has not been waited per request", "user_cpu_ns": nil, "system_cpu_ns": nil, "peak_rss_bytes": nil},
		"native_runtime_metrics_scope": "unsigned compiler-reported current-request diagnostics; artifact.source_build is historical",
	}
	defer func() {
		if t.Failed() {
			report["state"] = "FAILED_FIRST_OBSERVATION_PRESERVED"
		}
		storage, err := nativeSessionPilotStorage(output)
		report["retained_storage_before_result_json"] = storage
		if err != nil {
			report["state"] = "FAILED_FIRST_OBSERVATION_PRESERVED"
			t.Errorf("pilot evidence inventory: %v", err)
		}
		if err := nativeSessionProfileWriteJSON(filepath.Join(output, "result.json"), report); err != nil {
			t.Errorf("preserve pilot result: %v", err)
		}
	}()
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var lock upstream
	lockRaw, err := os.ReadFile(filepath.Join(repository, "upstream.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := nativeSessionProfileWriteNew(filepath.Join(output, "upstream.json"), lockRaw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lockRaw, &lock); err != nil {
		t.Fatal(err)
	}
	if runtime.Version() != lock.GoVersion || lock.GoVersion != nativeSessionProfileGoVersion || lock.CompilerRevision != uuidCompactCompilerRevision {
		t.Fatal("original205 lock and pinned Go1.27.3 oracle runtime required")
	}
	oracleHashes := map[string]string{}
	for path, want := range lock.SourceSHA {
		got, err := digestFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil || got != want {
			t.Fatalf("original Go oracle source differs: %s: %v", path, err)
		}
		oracleHashes[path] = "sha256:" + got
	}
	if err := verifyExternalOracles(repository, lock.ExternalSources); err != nil {
		t.Fatal(err)
	}
	report["oracle_go_version"], report["oracle_source_sha256"] = runtime.Version(), oracleHashes
	report["upstream_lock_sha256"], report["original_locked_compiler"] = contentDigest(lockRaw), lock.CompilerRevision
	report["experimental_execution_compiler"] = nativeSessionCompilerRevision
	suites, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(suites) <= 3 {
		t.Fatal("original corpus lacks RuneStart index3")
	}
	s := suites[3]
	if s.Package != "go/unicode/utf8" || s.Activity != "RuneStart" || len(s.Cases) != 256 || nativeBatchSize(s) != 128 {
		t.Fatal("pilot must retain the original RuneStart256 suite and128-row partition")
	}
	for index, c := range s.Cases {
		if len(c.Inputs) != 1 || !reflect.DeepEqual(c.Inputs[0], int64(index)) {
			t.Fatal("pilot must retain the original ordered int64 byte inputs")
		}
	}
	contract, err := contractFor(s)
	if err != nil || contract.Source != "ports/unicode/utf8.gooo" || contract.ID != "gooo://go/unicode/utf8/RuneStart" {
		t.Fatal("original RuneStart activity contract required", err)
	}
	frozen, err := freezeVerificationCatalog(filepath.Join(repository, "verification-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := nativeWorkspaceProfile("").workspaceForSuite(frozen, s)
	if err != nil {
		t.Fatal(err)
	}
	original, err := frozen.workspaceForSuite(s)
	if err != nil || !reflect.DeepEqual(selected, original) {
		t.Fatal("session pilot requires the unchanged original legacy workspace", err)
	}
	if err := nativeSessionProfileWriteNew(filepath.Join(output, "verification-catalog.json"), frozen.Raw); err != nil {
		t.Fatal(err)
	}
	report["catalog_sha256"], report["catalog_source_hashes"] = contentDigest(frozen.Raw), frozen.Hashes
	report["selected_source_paths"], report["selected_source_identities"] = selected.Names, selected.Identities
	sourceInventory, err := nativeSessionPilotSourceInventory(repository)
	if err != nil {
		t.Fatal(err)
	}
	report["experimental_source_inventory"] = sourceInventory
	report["experimental_sources_covered_by_old293_roster"] = false
	if err := validateSuiteEvidenceDirectories(output, []suite{s}); err != nil {
		t.Fatal(err)
	}
	cache, err := newNativeBuildCache(output)
	if err != nil {
		t.Fatal(err)
	}
	cacheClosed := false
	defer func() {
		if !cacheClosed {
			if err := cache.close(); err != nil {
				t.Errorf("close original Go build cache: %v", err)
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), nativeSessionPilotTimeout)
	defer cancel()
	var requests []map[string]any
	callbacks, freshRuns, builds, reuses, modelCalls := 0, 0, 0, 0, 0
	misses := map[string]int{}
	results, runErr := runNativeSuites(ctx, []suite{s}, 1, func(suiteCtx context.Context, _ int, originalSuite suite) (result nativeSuiteResult, returnedErr error) {
		callbacks++
		// One suite callback owns one session. Active complete batches retain the
		// parent command context; suite cancellation is observed between batches.
		verifier, client, err := newExperimentalNativeSessionVerifier(ctx, compiler, goTool, output, lock, cache)
		if err != nil {
			return result, err
		}
		defer func() {
			closeErr := client.Close()
			summary := client.Summary()
			report["session_lifetime_after_close"] = summary
			if closeErr == nil {
				closeErr = nativeSessionPilotCleanClose(summary)
			}
			returnedErr = errors.Join(returnedErr, closeErr)
		}()
		if verifier.caseProfile != "" || verifier.timing != nil || verifier.commandObserver != nil || !reflect.DeepEqual(verifier.lock, lock) {
			return result, fmt.Errorf("pilot must preserve legacy cases, original lock and separately scoped lifetime diagnostics")
		}
		verifier.inspectExecution = func(raw []byte, batch suite, replay bool, identity expectedIdentity, reported *nativeReportedRuntime) (checked, error) {
			var metrics nativeReportedRuntime
			check, err := inspectSessionWithRuntime(raw, batch, replay, identity, &metrics)
			if err != nil {
				return check, err
			}
			if reported != nil {
				*reported = metrics
			}
			var receipt struct {
				Result struct {
					Runtime struct {
						Build struct {
							Started bool `json:"started"`
						} `json:"build"`
						Runs []struct {
							Started, Completed bool
						} `json:"runs"`
						Artifact   nativeSessionArtifact `json:"artifact"`
						ModelCalls int                   `json:"model_calls"`
					} `json:"runtime"`
				} `json:"result"`
			}
			if err := json.Unmarshal(raw, &receipt); err != nil {
				return check, err
			}
			runtime := receipt.Result.Runtime
			for _, run := range runtime.Runs {
				if run.Started && run.Completed {
					freshRuns++
				}
			}
			if runtime.Build.Started {
				builds++
			}
			if runtime.Artifact.Reused {
				reuses++
			} else {
				misses[runtime.Artifact.MissReason]++
			}
			modelCalls += runtime.ModelCalls
			requests = append(requests, map[string]any{"replay": replay, "cases": len(batch.Cases), "receipt_sha256": contentDigest(raw), "generated_sha256": check.Generated,
				"replayed_from_sha256": check.ReplayedFrom, "current_request_reported_runtime": metrics,
				"artifact_observation": runtime.Artifact, "current_build_started": runtime.Build.Started, "current_native_runs": len(runtime.Runs)})
			return check, nil
		}
		return verifier.verify(suiteCtx, originalSuite, selected)
	})
	report["suite_callbacks"], report["inspected_requests"] = callbacks, requests
	report["fresh_native_runs"], report["current_native_builds"], report["artifact_reuses"] = freshRuns, builds, reuses
	report["artifact_miss_reasons"], report["model_calls"] = misses, modelCalls
	if err := cache.close(); err != nil {
		t.Fatal("close original Go build cache after callbacks join", err)
	}
	cacheClosed = true
	_, cacheErr := os.Lstat(cache.dir)
	report["go_build_cache_removed_after_callbacks_join"] = os.IsNotExist(cacheErr)
	if runErr != nil {
		report["verification_error"] = runErr.Error()
		t.Fatal(runErr)
	}
	if callbacks != 1 || len(results) != 1 || len(requests) != 4 || freshRuns != 8 || builds+reuses != 4 || modelCalls != 0 || !os.IsNotExist(cacheErr) {
		t.Fatal("pilot callback/request/current-run/cleanup denominator differs")
	}
	observed := results[0].Observation
	report["observation"] = observed
	if observed.Package != s.Package || observed.Activity != s.Activity || observed.Cases != 256 || observed.Passed != 256 || observed.Batches != 2 || !observed.ReplayEqual {
		t.Fatal("original RuneStart verifier observation differs")
	}
	var batches []map[string]any
	for index := 0; index < 2; index++ {
		dir := filepath.Join(suiteEvidenceDirectory(output, s), fmt.Sprintf("%03d", index))
		files := map[string]string{}
		for _, name := range []string{"gooo.workspace.json", "cases.json", "execution.json", "replay.json"} {
			digest, err := digestFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			files[name] = "sha256:" + digest
		}
		if requests[index*2]["receipt_sha256"] != files["execution.json"] || requests[index*2+1]["receipt_sha256"] != files["replay.json"] || requests[index*2+1]["replayed_from_sha256"] != files["execution.json"] {
			t.Fatal("saved own-batch receipt/hash binding differs")
		}
		batches = append(batches, map[string]any{"directory": dir, "case_range": []int{index * 128, (index + 1) * 128}, "sha256": files})
	}
	report["batches"] = batches
	report["state"] = "ORIGINAL_RUNESTART256_SESSION_VERIFIED"
}

func nativeSessionPilotCleanClose(summary map[string]any) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	value, err := decode(raw)
	if err != nil {
		return err
	}
	fields, ok := value.(map[string]any)
	if !ok || fields["schema"] != "gooo/native-session-transport-summary/v1" || fields["error"] != nil ||
		fields["close_request_sent"] != true || fields["close_response_received"] != true ||
		fields["closed"] != true || fields["waited"] != true || fields["streams_joined"] != true || fields["process_group_remaining"] != false || fields["exit_code"] != json.Number("0") {
		return fmt.Errorf("session close/wait/join/process-group result is incomplete")
	}
	entries, ok := fields["owned_entries_after_close"].([]any)
	if !ok || len(entries) != 0 {
		return fmt.Errorf("session-owned storage remains after close")
	}
	return nil
}

func nativeSessionPilotSourceInventory(repository string) (map[string]any, error) {
	paths, err := filepath.Glob(filepath.Join(repository, "cmd/verify/native_session*.go"))
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"cmd/verify/native_suite.go", "cmd/verify/inspect.go", "cmd/verify/corpus.go", "cmd/verify/workspace_catalog.go", "cmd/verify/case_profile.go", "cmd/verify/workspace_profile.go", "cmd/verify/batch_sizes.go", "cmd/verify/contracts.go", "cmd/verify/testdata/native-verification-contract-v1.json", "go.mod"} {
		paths = append(paths, filepath.Join(repository, name))
	}
	inventory := map[string]any{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(repository, path)
		if err != nil {
			return nil, err
		}
		inventory[filepath.ToSlash(relative)] = map[string]any{"bytes": len(raw), "sha256": contentDigest(raw)}
	}
	return inventory, nil
}

func nativeSessionPilotStorage(root string) (map[string]any, error) {
	var files, bytes, allocated int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected symlink in pilot evidence: %s", path)
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			files++
			bytes += info.Size()
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				allocated += stat.Blocks * 512
			}
		}
		return nil
	})
	return map[string]any{"regular_files": files, "logical_bytes": bytes, "allocated_bytes": allocated, "scope": "retained evidence before writing result.json; session temporary storage is separately checked after Close"}, err
}

func TestNativeSessionCompilerIdentityRejectsUnqualifiedProducer(t *testing.T) {
	qualified := map[string]any{"schema": "gooo/build-identity/v1", "compiler_source_sha": nativeSessionCompilerRevision, "vcs_revision": nativeSessionCompilerRevision,
		"source_status": "CLEAN_VCS", "vcs_modified": "false", "go_version": nativeSessionProfileGoVersion, "native_go_required": nativeSessionProfileGoVersion, "version": "descriptive"}
	raw, err := json.Marshal(qualified)
	if err != nil || validateNativeSessionCompilerIdentity(raw) != nil {
		t.Fatal("qualified immutable identity must be accepted", err)
	}
	for _, key := range []string{"schema", "compiler_source_sha", "vcs_revision", "source_status", "vcs_modified", "go_version", "native_go_required"} {
		t.Run(key, func(t *testing.T) {
			changed := map[string]any{}
			for name, value := range qualified {
				changed[name] = value
			}
			changed[key] = "different"
			raw, err := json.Marshal(changed)
			if err != nil || validateNativeSessionCompilerIdentity(raw) == nil {
				t.Fatal("unqualified compiler identity accepted", err)
			}
		})
	}
	duplicate := []byte(`{"compiler_source_sha":"` + nativeSessionCompilerRevision + `",` + string(raw[1:]))
	for _, malformed := range [][]byte{nil, duplicate, append(append([]byte{}, raw...), []byte(`{}`)...), append(append([]byte{}, raw...), []byte(strings.Repeat(" ", nativeSessionProfileProbeLimit))...)} {
		if validateNativeSessionCompilerIdentity(malformed) == nil {
			t.Fatal("missing, duplicate, trailing or oversized compiler identity accepted")
		}
	}
}
