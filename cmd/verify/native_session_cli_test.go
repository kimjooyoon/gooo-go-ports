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
	"testing"
	"unicode/utf8"
)

type nativeSessionCLIStubOwner struct {
	closed        bool
	closeCalls    int
	closeErr      error
	remaining     bool
	reportedError string
}

func (o *nativeSessionCLIStubOwner) Close() error {
	o.closeCalls++
	o.closed = true
	return o.closeErr
}

func (o *nativeSessionCLIStubOwner) Summary() map[string]any {
	entries := []string{}
	if o.remaining {
		entries = append(entries, "synthetic-retained-child")
	}
	summary := map[string]any{"schema": "gooo/native-session-transport-summary/v1", "closed": o.closed, "waited": o.closed,
		"streams_joined": o.closed, "process_group_remaining": false, "exit_code": 0, "error": nil,
		"owned_entries_after_close": entries, "close_request_sent": o.closed, "close_response_received": o.closed,
		"waited_process_resources_available": false, "waited_process_user_ns": nil, "waited_process_system_ns": nil,
		"resource_scope": "Synthetic lifecycle control only; no process, native computation or OS wait measurement"}
	if o.reportedError != "" {
		summary["error"] = o.reportedError
	}
	return summary
}

func nativeSessionCLITestRunner(t *testing.T) *nativeSessionSuiteRunner {
	t.Helper()
	if !nativeSessionPlatformSupported() {
		t.Skip("Unix session controller controls require Linux or macOS; unsupported stub remains compile-only")
	}
	compiler, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner, err := newNativeSessionSuiteRunner(workerTestContext(t), compiler, etagGoToolPath(), t.TempDir(), upstream{CompilerRevision: uuidCompactCompilerRevision, GoVersion: nativeSessionProfileGoVersion}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

// Receipts are source-bound synthetic test documents, never compiler output.
// The original verifier still performs case/source/oracle/replay inspection.
func nativeSessionCLIInstallStub(t *testing.T, r *nativeSessionSuiteRunner, s suite, raw []byte) (*fakeNativeCommand, *nativeSessionCLIStubOwner) {
	t.Helper()
	fake := &fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(r.root, s): raw}, calls: map[string][]string{}}
	owner := &nativeSessionCLIStubOwner{}
	r.newProfile = func(commands context.Context, compiler, goTool, ownerRoot string, lock upstream, cache *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
		if commands != r.commands || ownerRoot != suiteEvidenceDirectory(r.root, s) || goTool != etagGoToolPath() {
			return nativeSuiteVerifier{}, nil, errors.New("controller changed parent context or owner/tool identity")
		}
		v := newNativeSuiteVerifier(commands, compiler, ownerRoot, lock, cache)
		v.compilerRevisionOverride, v.inspectExecution, v.execute = nativeSessionCompilerRevision, inspectSessionWithRuntime, fake.execute
		return v, owner, nil
	}
	return fake, owner
}

func nativeSessionCLIReadSummary(t *testing.T, r *nativeSessionSuiteRunner) map[string]any {
	t.Helper()
	path := filepath.Join(r.root, "session-summary.json")
	if err := r.WriteSummary(path); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := readJSON(path, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestNativeSessionCLIConstructorDoesNotStartProcesses(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	entries, err := os.ReadDir(r.root)
	if err != nil || len(entries) != 0 || len(r.records) != 0 {
		t.Fatal("constructor started owner/probe evidence", entries, err)
	}
	if _, err := newNativeSessionSuiteRunner(r.commands, r.compiler, filepath.Join(r.root, "other-go"), r.root, r.lock, nil); err == nil {
		t.Fatal("different HTTP Go tool path accepted")
	}
	changed := r.lock
	changed.CompilerRevision = nativeSessionCompilerRevision
	if _, err := newNativeSessionSuiteRunner(r.commands, r.compiler, r.goTool, r.root, changed, nil); err == nil {
		t.Fatal("experimental producer replaced original oracle lock")
	}
}

func TestNativeSessionCLIAlreadyCancelledCallbackStartsNoOwner(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	_, s, identity := nativeSessionInspectFixture(t, false)
	called := false
	r.newProfile = func(context.Context, string, string, string, upstream, *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
		called = true
		return nativeSuiteVerifier{}, nil, errors.New("cancelled callback must not launch profile")
	}
	ctx, cancel := context.WithCancel(r.commands)
	cancel()
	_, err := r.Verify(ctx, s, fakeNativeWorkspace(t, s, identity))
	entries, readErr := os.ReadDir(r.root)
	if !errors.Is(err, context.Canceled) || called || readErr != nil || len(entries) != 0 || len(r.records) != 0 {
		t.Fatal("already-cancelled callback started or recorded an owner", err, called, entries, readErr)
	}
}

func TestNativeSessionCLICompactErrorCappedRawOwnerPreserved(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	raw, s, identity := nativeSessionInspectFixture(t, false)
	_, owner := nativeSessionCLIInstallStub(t, r, s, raw)
	owner.reportedError = "synthetic refusal: " + strings.Repeat("界", 3000)
	if _, err := r.Verify(r.commands, s, fakeNativeWorkspace(t, s, identity)); err == nil {
		t.Fatal("owner refusal became verified success")
	}
	summary := nativeSessionCLIReadSummary(t, r)
	row := summary["owners"].([]any)[0].(map[string]any)
	compact := row["lifecycle"].(map[string]any)["error"].(string)
	if len(compact) > 4096 || !utf8.ValidString(compact) || !strings.HasPrefix(owner.reportedError, compact) {
		t.Fatal("compact lifecycle error bypassed bounded UTF-8 diagnostic", len(compact))
	}
	var retained map[string]any
	if err := readJSON(row["lifecycle_path"].(string), &retained); err != nil || retained["error"] != owner.reportedError {
		t.Fatal("raw session owner error was shortened", err)
	}
}

func TestNativeSessionCLIRunnerOriginalPathsReplayAndOwnerJoin(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	raw, s, identity := nativeSessionInspectFixture(t, false)
	s = repeatNativeCase(s, 129)
	fake, owner := nativeSessionCLIInstallStub(t, r, s, raw)
	result, err := r.Verify(workerTestContext(t), s, fakeNativeWorkspace(t, s, identity))
	if err != nil || result.Observation.Passed != 129 || result.Observation.Batches != 2 || owner.closeCalls != 1 || !owner.closed {
		t.Fatal("controller changed production verification or returned before owner Close", result, owner, err)
	}
	for batch := range 2 {
		dir := filepath.Join(suiteEvidenceDirectory(r.root, s), fmt.Sprintf("%03d", batch))
		if !reflect.DeepEqual(fake.calls[dir], []string{"execute", "replay"}) {
			t.Fatal("same-batch execute/replay order or original path changed", fake.calls)
		}
	}
	nested := suiteEvidenceDirectory(suiteEvidenceDirectory(r.root, s), s)
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Fatal("controller nested the suite evidence directory", err)
	}
	if _, err := r.Verify(workerTestContext(t), s, fakeNativeWorkspace(t, s, identity)); err == nil {
		t.Fatal("duplicate owner attempted")
	}
	summary := nativeSessionCLIReadSummary(t, r)
	if summary["attempted_suites"] != float64(1) || summary["verified_suites"] != float64(1) || summary["full82_accepted"] != false {
		t.Fatal("controller summary changed denominator or scope", summary)
	}
	row := summary["owners"].([]any)[0].(map[string]any)
	path := row["lifecycle_path"].(string)
	bytes, err := os.ReadFile(path)
	if err != nil || contentDigest(bytes) != row["lifecycle_sha256"] || row["state"] != "SUITE_VERIFIED_OWNER_CLOSED" {
		t.Fatal("actual stub Close evidence not retained", row, err)
	}
	if err := r.WriteSummary(filepath.Join(r.root, "another-summary.json")); err == nil {
		t.Fatal("sealed summary rewritten")
	}
}

func TestNativeSessionCLIRunnerPreservesFailedOwnerEvidence(t *testing.T) {
	for _, mode := range []string{"constructor", "receipt", "close", "retained-storage", "evidence", "panic"} {
		t.Run(mode, func(t *testing.T) {
			r := nativeSessionCLITestRunner(t)
			raw, s, identity := nativeSessionInspectFixture(t, false)
			fake, owner := nativeSessionCLIInstallStub(t, r, s, raw)
			baseFactory := r.newProfile
			sentinel := errors.New("synthetic owner Close refusal")
			switch mode {
			case "constructor":
				r.newProfile = func(context.Context, string, string, string, upstream, *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
					return nativeSuiteVerifier{}, nil, errors.New("synthetic constructor refusal")
				}
			case "receipt":
				fake.mutate = func(_ string, document map[string]any) { document["decision"] = "FAIL" }
			case "close":
				owner.closeErr = sentinel
			case "retained-storage":
				owner.remaining = true
			case "evidence":
				r.newProfile = func(ctx context.Context, compiler, goTool, root string, lock upstream, cache *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
					if err := os.WriteFile(filepath.Join(root, "session-owner.json"), []byte("retained original"), 0600); err != nil {
						return nativeSuiteVerifier{}, nil, err
					}
					return baseFactory(ctx, compiler, goTool, root, lock, cache)
				}
			case "panic":
				fake.before = func(context.Context, string, string) error { panic("synthetic interrupted callback") }
			}
			panicked := false
			var returned error
			func() {
				defer func() { panicked = recover() != nil }()
				_, returned = r.Verify(workerTestContext(t), s, fakeNativeWorkspace(t, s, identity))
			}()
			if mode == "panic" {
				if !panicked {
					t.Fatal("panic unexpectedly became ordinary success")
				}
			} else if returned == nil {
				t.Fatal("failed controller became success")
			}
			if mode != "constructor" && owner.closeCalls != 1 {
				t.Fatal("failed callback did not close/join its owner", owner)
			}
			if mode == "close" && !errors.Is(returned, sentinel) {
				t.Fatal("original Close error was lost", returned)
			}
			summary := nativeSessionCLIReadSummary(t, r)
			if summary["attempted_suites"] != float64(1) || summary["verified_suites"] != float64(0) || summary["failed_suites"] != float64(1) {
				t.Fatal("partial failure was not retained", summary)
			}
			row := summary["owners"].([]any)[0].(map[string]any)
			if mode == "panic" && row["state"] != "INTERRUPTED_CALLBACK_INCOMPLETE" {
				t.Fatal("interruption gained verified status", row)
			}
			if mode == "evidence" {
				bytes, err := os.ReadFile(filepath.Join(suiteEvidenceDirectory(r.root, s), "session-owner.json"))
				if err != nil || string(bytes) != "retained original" || row["evidence_error"] == "" {
					t.Fatal("existing evidence overwritten or refusal omitted", row, err)
				}
			}
		})
	}
}

func TestNativeSessionCLIWorkerCancellationFinishesActiveBatch(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	raw, s, identity := nativeSessionInspectFixture(t, false)
	s = repeatNativeCase(s, 129)
	fake, owner := nativeSessionCLIInstallStub(t, r, s, raw)
	worker, cancel := context.WithCancel(r.commands)
	defer cancel()
	fake.before = func(commands context.Context, _ string, action string) error {
		if action == "execute" {
			cancel()
		}
		if commands.Err() != nil {
			return errors.New("worker cancellation replaced parent command context")
		}
		return nil
	}
	_, err := r.Verify(worker, s, fakeNativeWorkspace(t, s, identity))
	if !errors.Is(err, context.Canceled) || owner.closeCalls != 1 {
		t.Fatal("worker cancellation or lifecycle changed", err, owner)
	}
	first := filepath.Join(suiteEvidenceDirectory(r.root, s), "000")
	if !reflect.DeepEqual(fake.calls[first], []string{"execute", "replay"}) || len(fake.calls) != 1 {
		t.Fatal("active batch was interrupted or another batch started", fake.calls)
	}
	nativeSessionCLIReadSummary(t, r)
}

func TestNativeSessionCLIWriteSummaryRefusesActiveCallbacks(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	raw, s, identity := nativeSessionInspectFixture(t, false)
	fake, owner := nativeSessionCLIInstallStub(t, r, s, raw)
	started, release := make(chan struct{}), make(chan struct{})
	fake.before = func(ctx context.Context, _ string, action string) error {
		if action == "execute" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	done := make(chan error, 1)
	workspace := fakeNativeWorkspace(t, s, identity)
	go func() { _, err := r.Verify(r.commands, s, workspace); done <- err }()
	select {
	case <-started:
	case <-r.commands.Done():
		t.Fatal("callback did not start")
	}
	path := filepath.Join(r.root, "too-early.json")
	if err := r.WriteSummary(path); err == nil {
		t.Fatal("active callback became completed summary")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("refused summary still wrote evidence")
	}
	close(release)
	if err := <-done; err != nil || owner.closeCalls != 1 {
		t.Fatal("callback did not close before joining", err)
	}
	nativeSessionCLIReadSummary(t, r)
}

func TestNativeSessionCLIRunnerKeepsExhaustiveResultAndSortedOwners(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	secondRaw, second, secondIdentity := nativeSessionInspectFixture(t, false)
	firstRaw, first, firstIdentity := contractFixture(t, suite{Package: "go/unicode", Activity: "SimpleFold", Cases: []testCase{{Inputs: []any{int64(65)}, Expected: int64(97)}}})
	var firstDocument, sessionDocument map[string]any
	if json.Unmarshal(firstRaw, &firstDocument) != nil || json.Unmarshal(secondRaw, &sessionDocument) != nil {
		t.Fatal("synthetic fixture decode")
	}
	firstRuntime := firstDocument["result"].(map[string]any)["runtime"].(map[string]any)
	sessionRuntime := sessionDocument["result"].(map[string]any)["runtime"].(map[string]any)
	for _, key := range []string{"schema", "producer_source_sha", "elapsed_ns", "toolchain", "build", "runs", "artifact", "executable_sha256", "runtime_suite_sha256"} {
		firstRuntime[key] = sessionRuntime[key]
	}
	firstIdentity.CompilerRevision = nativeSessionCompilerRevision
	firstRaw = mustJSON(t, firstDocument)
	fake := &fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(r.root, first): firstRaw, suiteEvidenceDirectory(r.root, second): secondRaw}, calls: map[string][]string{}}
	owners := map[string]*nativeSessionCLIStubOwner{}
	exhaustivePath := ""
	r.newProfile = func(ctx context.Context, compiler, _ string, ownerRoot string, lock upstream, cache *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
		owner := &nativeSessionCLIStubOwner{}
		owners[ownerRoot] = owner
		v := newNativeSuiteVerifier(ctx, compiler, ownerRoot, lock, cache)
		v.compilerRevisionOverride, v.inspectExecution, v.execute = nativeSessionCompilerRevision, inspectSessionWithRuntime, fake.execute
		v.simpleFold = func(_ context.Context, path string, _ []byte, _ string, _ *nativeBuildCache) (simpleFoldExhaustiveReport, error) {
			exhaustivePath = path
			return simpleFoldExhaustiveReport{Schema: "synthetic-controller-result-only"}, nil
		}
		return v, owner, nil
	}
	if _, err := r.Verify(r.commands, second, fakeNativeWorkspace(t, second, secondIdentity)); err != nil {
		t.Fatal(err)
	}
	result, err := r.Verify(r.commands, first, fakeNativeWorkspace(t, first, firstIdentity))
	if err != nil || result.SimpleFold == nil || result.SimpleFold.Schema != "synthetic-controller-result-only" || exhaustivePath != filepath.Join(r.root, "simple-fold-exhaustive") {
		t.Fatal("controller discarded exhaustive report or relocated its original path", result, exhaustivePath, err)
	}
	for _, owner := range owners {
		if owner.closeCalls != 1 {
			t.Fatal("owner not joined once", owner)
		}
	}
	summary := nativeSessionCLIReadSummary(t, r)
	rows := summary["owners"].([]any)
	if len(rows) != 2 || summary["verified_suites"] != float64(2) || rows[0].(map[string]any)["owner_root"].(string) >= rows[1].(map[string]any)["owner_root"].(string) {
		t.Fatal("finite owner rows are not sorted", summary)
	}
}

func TestNativeSessionCLICompilerProbeRetainsRawBeforeRefusal(t *testing.T) {
	if !nativeSessionPlatformSupported() {
		t.Skip("local shell identity stub requires Linux or macOS")
	}
	for _, clean := range []bool{true, false} {
		t.Run(fmt.Sprintf("clean=%t", clean), func(t *testing.T) {
			identity := map[string]any{"schema": "gooo/build-identity/v1", "compiler_source_sha": nativeSessionCompilerRevision, "vcs_revision": nativeSessionCompilerRevision,
				"source_status": "CLEAN_VCS", "vcs_modified": "false", "go_version": nativeSessionProfileGoVersion, "native_go_required": nativeSessionProfileGoVersion}
			if !clean {
				identity["vcs_modified"] = "true"
			}
			raw, err := json.Marshal(identity)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			root := t.TempDir()
			compiler := filepath.Join(root, "local-identity-stub")
			script := "#!/bin/sh\ncat <<'GOOO_TEST_IDENTITY'\n" + strings.TrimSuffix(string(raw), "\n") + "\nGOOO_TEST_IDENTITY\n"
			if err := os.WriteFile(compiler, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			got, probeErr := readNativeSessionCLICompilerIdentity(workerTestContext(t), compiler, root)
			retained, err := os.ReadFile(filepath.Join(root, "compiler.json"))
			if err != nil || string(retained) != string(raw) || (probeErr == nil) != clean {
				t.Fatal("bounded compiler identity was not preserved before refusal", probeErr, err)
			}
			if clean && string(got) != string(raw) {
				t.Fatal("identity wrapper changed original bytes")
			}
			if _, err := os.Stat(filepath.Join(root, "compiler-probe.json")); err != nil {
				t.Fatal("probe status absent", err)
			}
		})
	}
}
