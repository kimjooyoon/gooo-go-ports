package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These tests use source-bound synthetic receipts and the production inspection
// and saved-replay checks. They run no compiler, generated program or network.
func timingIntegrationRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func timingIntegrationRows(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal("invalid timing frame", err)
		}
		rows = append(rows, row)
	}
	return rows
}

func timingIntegrationEvidence(t *testing.T, root, timingPath string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			t.Fatal("unexpected synthetic-evidence symlink")
		}
		if path == timingPath {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[name], err = os.ReadFile(path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestTimingPreservesOriginalReceiptsSourcesAndBatchExecution(t *testing.T) {
	raw, original, identity := fixture(t)
	s := repeatNativeCase(original, 129)
	workspace := fakeNativeWorkspace(t, s, identity)
	var evidence [2]map[string][]byte
	var observations [2][]nativeSuiteResult
	for mode := range 2 {
		root := timingIntegrationRoot(t)
		var timing *nativeTiming
		var path string
		if mode == 1 {
			timing, path = timingRecorderForTest(t, root, []suite{s})
		}
		fake := fakeNativeCommand{
			templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw},
			calls:     map[string][]string{},
		}
		ctx := workerTestContext(t)
		verifier := newNativeSuiteVerifier(ctx, "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
		verifier.execute, verifier.timing = fake.execute, timing
		results, err := runNativeSuites(ctx, []suite{s}, 1, func(ctx context.Context, index int, current suite) (nativeSuiteResult, error) {
			span := timing.beginSuite(index, current)
			result, err := verifier.verify(ctx, current, workspace)
			span.finish(result, err, ctx.Err())
			return result, err
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := save(filepath.Join(root, "summary.json"), results); err != nil {
			t.Fatal(err)
		}
		if err := timing.close(nil, ctx.Err()); err != nil {
			t.Fatal(err)
		}
		observations[mode] = results
		for batch := range 2 {
			dir := filepath.Join(suiteEvidenceDirectory(root, s), []string{"000", "001"}[batch])
			if !reflect.DeepEqual(fake.calls[dir], []string{"execute", "replay"}) {
				t.Fatal("timing changed fresh execute/replay calls", mode, batch, fake.calls[dir])
			}
		}
		if len(fake.calls) != 2 {
			t.Fatal("timing changed original batch boundaries", mode, fake.calls)
		}
		if mode == 1 {
			rows := timingIntegrationRows(t, path)
			last := rows[len(rows)-1]
			if last["kind"] != "run_end" || last["status"] != "COMPLETE" || last["verified_batches"] != float64(2) || last["verified_cases"] != float64(129) {
				t.Fatal("full inspected coverage did not yield complete timing", last)
			}
		}
		evidence[mode] = timingIntegrationEvidence(t, root, path)
	}
	if !reflect.DeepEqual(observations[0], observations[1]) || !reflect.DeepEqual(evidence[0], evidence[1]) {
		t.Fatal("optional timing changed original observations or saved evidence bytes")
	}
}

func TestTimingCannotTurnAnUnboundReplayIntoSuccess(t *testing.T) {
	raw, s, identity := fixture(t)
	root := timingIntegrationRoot(t)
	timing, path := timingRecorderForTest(t, root, []suite{s})
	fake := fakeNativeCommand{
		templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw},
		calls:     map[string][]string{},
		mutate: func(action string, receipt map[string]any) {
			if action == "replay" {
				receipt["replayed_from_sha256"] = "sha256:" + strings.Repeat("0", 64)
			}
		},
	}
	ctx := workerTestContext(t)
	verifier := newNativeSuiteVerifier(ctx, "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
	verifier.execute, verifier.timing = fake.execute, timing
	span := timing.beginSuite(0, s)
	result, originalErr := verifier.verify(ctx, s, fakeNativeWorkspace(t, s, identity))
	span.finish(result, originalErr, ctx.Err())
	if originalErr == nil || result.Observation.Passed != 0 {
		t.Fatal("altered original receipt was accepted", result, originalErr)
	}
	if err := timing.close(originalErr, ctx.Err()); err == nil {
		t.Fatal("invalid saved replay received a successful timing completion")
	}
	rows := timingIntegrationRows(t, path)
	if last := rows[len(rows)-1]; last["status"] == "COMPLETE" || last["verified_batches"] != float64(0) {
		t.Fatal("unbound replay became successful coverage", last)
	}
}

func TestTimingPreservesActiveBatchCancellationAndParentContext(t *testing.T) {
	raw, original, identity := fixture(t)
	s := repeatNativeCase(original, 129)
	root := timingIntegrationRoot(t)
	timing, path := timingRecorderForTest(t, root, []suite{s})
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
	verifier.execute, verifier.timing = fake.execute, timing
	span := timing.beginSuite(0, s)
	result, originalErr := verifier.verify(worker, s, fakeNativeWorkspace(t, s, identity))
	span.finish(result, originalErr, worker.Err())
	first := filepath.Join(suiteEvidenceDirectory(root, s), "000")
	if !errors.Is(originalErr, context.Canceled) || parent.Err() != nil || len(fake.calls) != 1 || !reflect.DeepEqual(fake.calls[first], []string{"execute", "replay"}) {
		t.Fatal("timing changed batch-boundary cancellation or parent-owned commands", originalErr, parent.Err(), fake.calls)
	}
	if err := timing.close(originalErr, worker.Err()); err == nil {
		t.Fatal("canceled suite became timing success")
	}
	rows := timingIntegrationRows(t, path)
	if last := rows[len(rows)-1]; last["status"] == "COMPLETE" || last["successful_suites"] != float64(0) {
		t.Fatal("canceled work was counted as a successful duration sample", last)
	}
}

func TestTimingDoesNotRecoverNativePanicOrWriteCompleteFooter(t *testing.T) {
	raw, s, identity := fixture(t)
	root := timingIntegrationRoot(t)
	timing, path := timingRecorderForTest(t, root, []suite{s})
	fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
	panicValue := errors.New("synthetic command panic")
	fake.before = func(context.Context, string, string) error { panic(panicValue) }
	ctx := workerTestContext(t)
	verifier := newNativeSuiteVerifier(ctx, "unused", root, upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}, nil)
	verifier.execute, verifier.timing = fake.execute, timing
	timing.beginSuite(0, s)
	var observed any
	func() {
		defer func() { observed = recover() }()
		_, _ = verifier.verify(ctx, s, fakeNativeWorkspace(t, s, identity))
	}()
	if observed != panicValue {
		t.Fatal("instrumentation recovered or changed the native panic", observed)
	}
	timing.abort()
	if err := timing.close(nil, nil); err == nil {
		t.Fatal("interrupted verifier became timing success")
	}
	rows := timingIntegrationRows(t, path)
	if last := rows[len(rows)-1]; last["status"] == "COMPLETE" || last["successful_suites"] != float64(0) {
		t.Fatal("panic produced a complete timing footer", last)
	}
}
