package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

// These fixtures contain five tiny synthetic suites, not the production corpus.
// All runtime callbacks are fake: no compiler, generated program or private
// native build cache is created by the executor in these tests.
func ciShardExecutorFixture(t *testing.T) (ciShardPlan, ciShardNativeInventory, ciShardNativeRuntime) {
	t.Helper()
	plan := ciShardTestPlan(t, 1)
	inventory := ciShardNativeInventory{Lock: upstream{CompilerRevision: plan.Run.CompilerRevision, CompilerVersion: "0.6.37-dev", GoVersion: plan.Run.GoVersion}}
	// Profiles require the currently admitted pin, even in synthetic fixtures.
	plan.Run.CompilerRevision = uuidCompactCompilerRevision
	var err error
	plan.PlanSHA256, err = ciShardPlanDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	inventory.Lock.CompilerRevision = plan.Run.CompilerRevision
	for _, entry := range plan.Suites {
		inventory.Suites = append(inventory.Suites, suite{Package: entry.Package, Activity: entry.Activity, Cases: make([]testCase, int(entry.Cases))})
		inventory.Workspaces = append(inventory.Workspaces, suiteWorkspace{Names: append([]string(nil), entry.SourcePaths...)})
	}
	records := ciShardTestPartials(t, plan)[0].Suites
	cache := &nativeBuildCache{}
	native := ciShardNativeRuntime{
		identify: func(_ context.Context, _ string, path string, args ...string) ([]byte, error) {
			if !reflect.DeepEqual(args, []string{"version", "--build", "--json"}) {
				return nil, fmt.Errorf("wrong compiler identity command")
			}
			raw := ciShardExecutorIdentity(t, inventory.Lock)
			return raw, os.WriteFile(path, raw, 0600)
		},
		newCache: func(string) (*nativeBuildCache, error) { return cache, nil },
		closeCache: func(got *nativeBuildCache) error {
			if got != cache {
				return fmt.Errorf("foreign cache")
			}
			return nil
		},
		verify: func(_ context.Context, original int, _ nativeSuiteVerifier, s suite, _ suiteWorkspace) (nativeSuiteResult, error) {
			return ciShardExecutorResult(plan, original, s), nil
		},
		inspect: func(_ ciShardPlan, _ int, original int, _ ciShardNativeInventory, _ string, reported ciShardSuiteRecord) (ciShardSuiteRecord, error) {
			actual := records[original]
			actual.State, actual.Error = reported.State, reported.Error
			return actual, nil
		},
	}
	return plan, inventory, native
}

func ciShardExecutorIdentity(t *testing.T, lock upstream) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"schema": "gooo/build-identity/v1", "version": lock.CompilerVersion,
		"compiler_source_sha": lock.CompilerRevision, "vcs_revision": lock.CompilerRevision,
		"source_status": "CLEAN_VCS", "vcs_modified": "false", "go_version": lock.GoVersion, "native_go_required": lock.GoVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func ciShardExecutorResult(plan ciShardPlan, original int, s suite) nativeSuiteResult {
	entry := plan.Suites[original]
	return nativeSuiteResult{Observation: observation{Package: s.Package, Activity: s.Activity, Cases: len(s.Cases), Passed: len(s.Cases), Batches: int(entry.Batches), ReplayEqual: true}}
}

func TestCIShardExecutorThreeWorkersNamespacesJoinAndFreshOutputs(t *testing.T) {
	plan, inventory, native := ciShardExecutorFixture(t)
	out := filepath.Join(t.TempDir(), "evidence")
	ctx := workerTestContext(t)
	started, release, finished := make(chan int, 5), make(chan struct{}), make(chan struct{})
	var active, peak, closed atomic.Int32
	var mu sync.Mutex
	calls := map[int]int{}
	native.verify = func(worker context.Context, original int, v nativeSuiteVerifier, s suite, workspace suiteWorkspace) (nativeSuiteResult, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		if closed.Load() != 0 || v.root != ciShardSuiteDirectory(out, 0, original) || v.batchDirectory(0) != ciShardBatchDirectory(out, 0, original, 0) || v.commands != ctx || v.caseProfile != nativeCaseProfile(plan.Profiles.Case) || !reflect.DeepEqual(workspace.Names, plan.Suites[original].SourcePaths) {
			return nativeSuiteResult{}, fmt.Errorf("shared cache/context/source or namespace changed")
		}
		mu.Lock()
		calls[original]++
		mu.Unlock()
		started <- original
		select {
		case <-release:
		case <-worker.Done():
			return nativeSuiteResult{}, worker.Err()
		}
		return ciShardExecutorResult(plan, original, s), nil
	}
	native.closeCache = func(*nativeBuildCache) error {
		if active.Load() != 0 {
			return fmt.Errorf("cache closed before join")
		}
		closed.Add(1)
		return nil
	}
	inspect := native.inspect
	native.inspect = func(p ciShardPlan, shard, original int, in ciShardNativeInventory, root string, record ciShardSuiteRecord) (ciShardSuiteRecord, error) {
		if closed.Load() != 1 || active.Load() != 0 {
			return record, fmt.Errorf("reader ran before joined cache close")
		}
		return inspect(p, shard, original, in, root, record)
	}
	var partial ciShardPartial
	var err error
	go func() {
		partial, err = executeCIShardNativeWith(ctx, "unused", out, plan, 0, inventory, native)
		close(finished)
	}()
	for range 3 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("three workers did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("started above three workers")
	default:
	}
	close(release)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("executor did not join")
	}
	if err != nil || !partial.Joined || partial.State != "SUCCEEDED" || active.Load() != 0 || peak.Load() != 3 || closed.Load() != 1 || len(calls) != 5 {
		t.Fatal(partial, err, active.Load(), peak.Load(), closed.Load(), calls)
	}
	for _, record := range partial.Suites {
		if record.State != "VERIFIED" || calls[record.OriginalIndex] != 1 {
			t.Fatal("suite dropped, duplicated or reordered", record, calls)
		}
	}
	if _, err := reduceCIShardPartials(plan, []ciShardPartial{partial}); err != nil {
		t.Fatal("returned metadata contract is invalid", err)
	}
	if _, err := os.Stat(filepath.Join(out, "partial.json")); !os.IsNotExist(err) {
		t.Fatal("executor claimed ownership of caller partial writer")
	}
	for name := range partial.Sources {
		delete(partial.Sources, name)
	}
	if len(plan.Sources) == 0 {
		t.Fatal("partial aliases immutable plan sources")
	}
	if again, err := executeCIShardNativeWith(ctx, "unused", out, plan, 0, inventory, native); err == nil || again.Schema != "" {
		t.Fatal("existing evidence reused", again, err)
	}
}

func TestCIShardExecutorFailurePreservesOriginalOrderAndJoinsCanceledWork(t *testing.T) {
	plan, inventory, native := ciShardExecutorFixture(t)
	out, ctx := filepath.Join(t.TempDir(), "failure"), workerTestContext(t)
	started, release, higherFailed := make(chan int, 3), make(chan struct{}), make(chan struct{})
	var active, closed atomic.Int32
	native.verify = func(worker context.Context, original int, v nativeSuiteVerifier, s suite, _ suiteWorkspace) (nativeSuiteResult, error) {
		active.Add(1)
		defer active.Add(-1)
		started <- original
		select {
		case <-release:
		case <-worker.Done():
			return nativeSuiteResult{}, worker.Err()
		}
		switch original {
		case 0:
			<-higherFailed
			return nativeSuiteResult{}, errors.New("first-original-failure")
		case 1:
			<-worker.Done()
			if v.commands.Err() != nil {
				return nativeSuiteResult{}, errors.New("local cancellation killed parent commands")
			}
			return nativeSuiteResult{}, worker.Err()
		case 2:
			close(higherFailed)
			return nativeSuiteResult{}, errors.New("higher-original-failure")
		default:
			return ciShardExecutorResult(plan, original, s), nil
		}
	}
	native.closeCache = func(*nativeBuildCache) error {
		if active.Load() != 0 {
			return errors.New("cache closed early")
		}
		closed.Add(1)
		return nil
	}
	inspect := native.inspect
	native.inspect = func(p ciShardPlan, shard, original int, in ciShardNativeInventory, root string, record ciShardSuiteRecord) (ciShardSuiteRecord, error) {
		actual, err := inspect(p, shard, original, in, root, record)
		if original == 0 {
			return actual, errors.New("incomplete-tail-preserved")
		}
		return actual, err
	}
	finished := make(chan struct{})
	var partial ciShardPartial
	var err error
	go func() {
		partial, err = executeCIShardNativeWith(ctx, "unused", out, plan, 0, inventory, native)
		close(finished)
	}()
	for range 3 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not start")
		}
	}
	close(release)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("failure did not join")
	}
	if err == nil || !strings.Contains(err.Error(), "first-original-failure") || !partial.Joined || partial.State != "FAILED" || active.Load() != 0 || closed.Load() != 1 {
		t.Fatal(partial, err)
	}
	if partial.Suites[0].State != "FAILED" || !strings.Contains(partial.Suites[0].Error, "first-original-failure") || !strings.Contains(partial.Suites[0].Error, "incomplete-tail-preserved") || partial.Suites[1].State != "CANCELED" || partial.Suites[3].State != "NOT_STARTED" || partial.Suites[4].State != "NOT_STARTED" {
		t.Fatal("failure or joined-prefix state lost", partial.Suites)
	}
	_, reducedErr := reduceCIShardPartials(plan, []ciShardPartial{partial})
	var failure *ciShardFailure
	if !errors.As(reducedErr, &failure) || failure.OriginalIndex != 0 {
		t.Fatal("earliest original error lost", reducedErr)
	}
	if raw, readErr := os.ReadFile(filepath.Join(out, "executor-error.txt")); readErr != nil || !bytes.Contains(raw, []byte("first-original-failure")) {
		t.Fatal("first error evidence not preserved", string(raw), readErr)
	}
}

func TestCIShardExecutorReaderDowngradeRetainsCompletedPrefix(t *testing.T) {
	for _, producerState := range []string{"VERIFIED", "CANCELED"} {
		t.Run(producerState, func(t *testing.T) {
			plan, inventory, native := ciShardExecutorFixture(t)
			verify, inspect := native.verify, native.inspect
			native.verify = func(ctx context.Context, original int, v nativeSuiteVerifier, s suite, w suiteWorkspace) (nativeSuiteResult, error) {
				if original == 1 && producerState == "CANCELED" {
					return nativeSuiteResult{}, context.Canceled
				}
				return verify(ctx, original, v, s, w)
			}
			native.inspect = func(p ciShardPlan, shard, original int, in ciShardNativeInventory, root string, r ciShardSuiteRecord) (ciShardSuiteRecord, error) {
				actual, err := inspect(p, shard, original, in, root, r)
				if original == 1 {
					actual.State, actual.Error = "FAILED", "second batch replay missing"
					actual.Batches = actual.Batches[:1]
					return actual, errors.New(actual.Error)
				}
				return actual, err
			}
			partial, err := executeCIShardNativeWith(workerTestContext(t), "unused", filepath.Join(t.TempDir(), "prefix"), plan, 0, inventory, native)
			got := partial.Suites[1]
			want := ciShardTestPartials(t, plan)[0].Suites[1].Batches[0]
			if err == nil || !partial.Joined || partial.State != "FAILED" || got.State != "FAILED" || !strings.Contains(got.Error, "second batch replay missing") || len(got.Batches) != 1 || !reflect.DeepEqual(got.Batches[0], want) {
				t.Fatal("reader downgrade lost completed prefix or inflated success", partial, err)
			}
			if err := validateCIShardSuite(plan, 0, got); err != nil {
				t.Fatal("retained failure prefix is invalid", err)
			}
		})
	}
}

func TestCIShardExecutorReaderCannotUpgradeFailedProducer(t *testing.T) {
	plan, inventory, native := ciShardExecutorFixture(t)
	verify, inspect := native.verify, native.inspect
	native.verify = func(ctx context.Context, original int, v nativeSuiteVerifier, s suite, w suiteWorkspace) (nativeSuiteResult, error) {
		if original == 1 {
			return nativeSuiteResult{}, errors.New("producer failed")
		}
		return verify(ctx, original, v, s, w)
	}
	native.inspect = func(p ciShardPlan, shard, original int, in ciShardNativeInventory, root string, r ciShardSuiteRecord) (ciShardSuiteRecord, error) {
		actual, err := inspect(p, shard, original, in, root, r)
		if original == 1 {
			actual.State, actual.Error = "VERIFIED", ""
		}
		return actual, err
	}
	partial, err := executeCIShardNativeWith(workerTestContext(t), "unused", filepath.Join(t.TempDir(), "upgrade"), plan, 0, inventory, native)
	if err == nil || !partial.Joined || partial.State != "FAILED" || partial.Suites[1].State != "FAILED" || len(partial.Suites[1].Batches) != 0 || !strings.Contains(err.Error(), "foreign suite identity/state") {
		t.Fatal("reader upgraded a failed producer", partial, err)
	}
}

func TestCIShardExecutorParentCancellationJoinsWithoutSuccess(t *testing.T) {
	plan, inventory, native := ciShardExecutorFixture(t)
	ctx, cancel := context.WithCancel(workerTestContext(t))
	defer cancel()
	started, finished := make(chan struct{}, 3), make(chan struct{})
	var active, closed atomic.Int32
	native.verify = func(worker context.Context, _ int, _ nativeSuiteVerifier, _ suite, _ suiteWorkspace) (nativeSuiteResult, error) {
		active.Add(1)
		defer active.Add(-1)
		started <- struct{}{}
		<-worker.Done()
		return nativeSuiteResult{}, worker.Err()
	}
	native.closeCache = func(*nativeBuildCache) error {
		if active.Load() != 0 {
			return errors.New("cache closed before parent join")
		}
		closed.Add(1)
		return nil
	}
	var partial ciShardPartial
	var err error
	go func() {
		partial, err = executeCIShardNativeWith(ctx, "unused", filepath.Join(t.TempDir(), "canceled"), plan, 0, inventory, native)
		close(finished)
	}()
	for range 3 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not start")
		}
	}
	cancel()
	<-finished
	if !errors.Is(err, context.Canceled) || !partial.Joined || partial.State != "INCOMPLETE" || active.Load() != 0 || closed.Load() != 1 {
		t.Fatal(partial, err)
	}
}

func TestCIShardExecutorCompilerIdentityRejectsBeforeCacheOrSuites(t *testing.T) {
	for _, field := range []string{"schema", "version", "compiler_source_sha", "vcs_revision", "source_status", "vcs_modified", "go_version", "native_go_required"} {
		t.Run(field, func(t *testing.T) {
			plan, inventory, native := ciShardExecutorFixture(t)
			out := filepath.Join(t.TempDir(), "rejected")
			var build map[string]string
			if err := json.Unmarshal(ciShardExecutorIdentity(t, inventory.Lock), &build); err != nil {
				t.Fatal(err)
			}
			build[field] = "foreign-or-dirty"
			raw, _ := json.Marshal(build)
			native.identify = func(_ context.Context, _ string, path string, _ ...string) ([]byte, error) {
				return raw, os.WriteFile(path, raw, 0600)
			}
			native.newCache = func(string) (*nativeBuildCache, error) {
				t.Error("created cache for bad compiler")
				return nil, errors.New("must not run")
			}
			native.verify = func(context.Context, int, nativeSuiteVerifier, suite, suiteWorkspace) (nativeSuiteResult, error) {
				t.Error("started suite for bad compiler")
				return nativeSuiteResult{}, nil
			}
			partial, err := executeCIShardNativeWith(workerTestContext(t), "unused", out, plan, 0, inventory, native)
			if err == nil || partial.Joined || partial.State != "INCOMPLETE" {
				t.Fatal(partial, err)
			}
			if got, err := os.ReadFile(filepath.Join(out, "compiler.json")); err != nil || !bytes.Equal(got, raw) {
				t.Fatal("bad compiler first evidence lost", err)
			}
		})
	}
}

func TestCIShardExecutorReaderAndCleanupFailuresCannotSucceed(t *testing.T) {
	for _, kind := range []string{"observation", "reader", "foreign-reader", "cache-close"} {
		t.Run(kind, func(t *testing.T) {
			plan, inventory, native := ciShardExecutorFixture(t)
			switch kind {
			case "observation":
				native.verify = func(context.Context, int, nativeSuiteVerifier, suite, suiteWorkspace) (nativeSuiteResult, error) {
					return nativeSuiteResult{}, nil
				}
			case "reader":
				native.inspect = func(_ ciShardPlan, _ int, _ int, _ ciShardNativeInventory, _ string, r ciShardSuiteRecord) (ciShardSuiteRecord, error) {
					return r, errors.New("raw receipt rejected")
				}
			case "foreign-reader":
				native.inspect = func(_ ciShardPlan, _ int, _ int, _ ciShardNativeInventory, _ string, r ciShardSuiteRecord) (ciShardSuiteRecord, error) {
					r.OriginalIndex = 999
					return r, nil
				}
			case "cache-close":
				native.closeCache = func(*nativeBuildCache) error { return errors.New("ownership changed") }
			}
			partial, err := executeCIShardNativeWith(workerTestContext(t), "unused", filepath.Join(t.TempDir(), kind), plan, 0, inventory, native)
			if err == nil || !partial.Joined || partial.State == "SUCCEEDED" {
				t.Fatal("incomplete evidence succeeded", partial, err)
			}
		})
	}
}

func TestCIShardExecutorInvalidInventoryNeverCreatesOutputOrStartsCommands(t *testing.T) {
	for _, kind := range []string{"foreign-index", "foreign-plan", "workspace-count", "suite-identity", "case-count", "compiler-pin", "canceled-context", "cache-unavailable"} {
		t.Run(kind, func(t *testing.T) {
			plan, inventory, native := ciShardExecutorFixture(t)
			out, shard := filepath.Join(t.TempDir(), "not-created"), 0
			ctx := workerTestContext(t)
			var identifies, callbacks atomic.Int32
			identify := native.identify
			native.identify = func(ctx context.Context, compiler, path string, args ...string) ([]byte, error) {
				identifies.Add(1)
				return identify(ctx, compiler, path, args...)
			}
			native.verify = func(context.Context, int, nativeSuiteVerifier, suite, suiteWorkspace) (nativeSuiteResult, error) {
				callbacks.Add(1)
				return nativeSuiteResult{}, errors.New("unexpected suite callback")
			}
			switch kind {
			case "foreign-index":
				shard = 999
			case "foreign-plan":
				plan.Shards[0].OriginalIndices[0] = 999
				plan.PlanSHA256, _ = ciShardPlanDigest(plan)
			case "workspace-count":
				inventory.Workspaces = inventory.Workspaces[:1]
			case "suite-identity":
				inventory.Suites[0].Package = "foreign/package"
			case "case-count":
				inventory.Suites[0].Cases = nil
			case "compiler-pin":
				inventory.Lock.CompilerRevision = strings.Repeat("f", 40)
			case "canceled-context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "cache-unavailable":
				native.newCache = func(string) (*nativeBuildCache, error) { return nil, errors.New("private cache unavailable fixture") }
			}
			partial, err := executeCIShardNativeWith(ctx, "unused", out, plan, shard, inventory, native)
			if err == nil || partial.Joined || callbacks.Load() != 0 {
				t.Fatal("invalid input started work or claimed a joined run", partial, err, callbacks.Load())
			}
			if kind == "cache-unavailable" {
				if identifies.Load() != 1 || partial.State != "INCOMPLETE" {
					t.Fatal("cache setup failure lost compiler-first diagnostic state", partial, err)
				}
				if _, err := os.Stat(filepath.Join(out, "executor-error.txt")); err != nil {
					t.Fatal("setup failure diagnosis lost", err)
				}
			} else if identifies.Load() != 0 {
				t.Fatal("invalid input ran compiler identity command")
			} else if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("invalid input created output", err)
			}
		})
	}
}

func TestCIShardExecutorKeepsFullErrorBesideBoundedSuiteDiagnostic(t *testing.T) {
	plan, inventory, native := ciShardExecutorFixture(t)
	out := filepath.Join(t.TempDir(), "long-diagnostic")
	message := "original-failure: " + strings.Repeat("흐", 2000) + ":retained-tail"
	native.verify = func(context.Context, int, nativeSuiteVerifier, suite, suiteWorkspace) (nativeSuiteResult, error) {
		return nativeSuiteResult{}, errors.New(message)
	}
	partial, err := executeCIShardNativeWith(workerTestContext(t), "unused", out, plan, 0, inventory, native)
	if err == nil || !partial.Joined || partial.State != "FAILED" {
		t.Fatal(partial, err)
	}
	for _, record := range partial.Suites {
		if record.State == "FAILED" && (len(record.Error) == 0 || len(record.Error) > 4096 || !utf8.ValidString(record.Error)) {
			t.Fatal("suite diagnostic is not bounded UTF-8", len(record.Error))
		}
	}
	full, readErr := os.ReadFile(filepath.Join(out, "executor-error.txt"))
	if readErr != nil || !bytes.Contains(full, []byte(message)) {
		t.Fatal("complete native failure diagnostic was truncated", readErr)
	}
}

func TestCIShardBatchDirectoryHookPreservesDefaultWriterAndInspection(t *testing.T) {
	raw, s, identity := fixture(t)
	workspace := fakeNativeWorkspace(t, s, identity)
	root := t.TempDir()
	oldRoot, newRoot := filepath.Join(root, "default"), filepath.Join(root, "sharded")
	oldDir := filepath.Join(suiteEvidenceDirectory(oldRoot, s), "000")
	newDir := ciShardBatchDirectory(newRoot, 0, 1, 0)
	for _, sharded := range []bool{false, true} {
		base, dir, templateRoot := oldRoot, oldDir, suiteEvidenceDirectory(oldRoot, s)
		if sharded {
			base = ciShardSuiteDirectory(newRoot, 0, 1)
			dir = newDir
			templateRoot = base
		}
		fake := fakeNativeCommand{templates: map[string][]byte{templateRoot: raw}, calls: map[string][]string{}}
		v := newNativeSuiteVerifier(workerTestContext(t), "unused", base, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
		v.execute = fake.execute
		if sharded {
			v.batchDirectory = func(batch int) string { return ciShardBatchDirectory(newRoot, 0, 1, batch) }
		}
		result, err := v.verify(workerTestContext(t), s, workspace)
		if err != nil || result.Observation.Passed != len(s.Cases) || !reflect.DeepEqual(fake.calls[dir], []string{"execute", "replay"}) {
			t.Fatal(result, err, fake.calls)
		}
	}
	for _, name := range append(append([]string{}, workspace.Names...), "gooo.workspace.json", "cases.json") {
		before, err := os.ReadFile(filepath.Join(oldDir, name))
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(newDir, name))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("hook changed original source/manifest/case bytes", name, err)
		}
	}
}
