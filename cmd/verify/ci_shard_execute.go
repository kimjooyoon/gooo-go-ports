package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Per-call dependencies let tiny tests exercise joins, cancellation and error
// paths without a compiler, generated program or new native build cache.
type ciShardNativeRuntime struct {
	identify   func(context.Context, string, string, ...string) ([]byte, error)
	newCache   func(string) (*nativeBuildCache, error)
	closeCache func(*nativeBuildCache) error
	verify     func(context.Context, int, nativeSuiteVerifier, suite, suiteWorkspace) (nativeSuiteResult, error)
	inspect    func(ciShardPlan, int, int, ciShardNativeInventory, string, ciShardSuiteRecord) (ciShardSuiteRecord, error)
}

func executeCIShardNative(ctx context.Context, compiler, out string, plan ciShardPlan, shardIndex int, inventory ciShardNativeInventory) (ciShardPartial, error) {
	return executeObservedCIShardNative(ctx, compiler, out, plan, shardIndex, inventory)
}

func validateCIShardCompilerIdentity(raw []byte, lock upstream) error {
	if len(raw) == 0 || len(raw) > MaxContractBytes {
		return fmt.Errorf("bounded compiler build identity required")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return err
	}
	value, err := decode(raw)
	if err != nil {
		return err
	}
	build, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("compiler build identity must be an object")
	}
	for key, expected := range map[string]string{
		"schema": "gooo/build-identity/v1", "version": lock.CompilerVersion,
		"compiler_source_sha": lock.CompilerRevision, "vcs_revision": lock.CompilerRevision,
		"source_status": "CLEAN_VCS", "vcs_modified": "false",
		"go_version": lock.GoVersion, "native_go_required": lock.GoVersion,
	} {
		if expected == "" || build[key] != expected {
			return fmt.Errorf("compiler build identity differs from pinned inventory: %s", key)
		}
	}
	return nil
}

func ciShardExecutionDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToValidUTF8(err.Error(), "\ufffd")
	if len(text) > 4096 {
		text = text[:4096]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	if text == "" {
		return "native shard operation failed"
	}
	return text
}

func ciShardEmptySuiteRecord(plan ciShardPlan, original int) ciShardSuiteRecord {
	s := plan.Suites[original]
	return ciShardSuiteRecord{OriginalIndex: original, Package: s.Package, Activity: s.Activity,
		State: "NOT_STARTED", Batches: []ciShardBatch{}, Exhaustive: []ciShardExhaustive{}}
}

// inventory MUST come from loadCIShardNativeInventory for this independently
// validated plan/checkout. The checks here prevent shape/pin mixups; they do not
// replace independent oracle/source loading. The caller owns writing partial.json.
// The executor owns a fresh out directory and preserves every failed artifact.
func executeCIShardNativeWith(ctx context.Context, compiler, out string, plan ciShardPlan, shardIndex int, inventory ciShardNativeInventory, native ciShardNativeRuntime) (partial ciShardPartial, returnedErr error) {
	if ctx == nil || compiler == "" || out == "" || native.identify == nil || native.newCache == nil || native.closeCache == nil || native.verify == nil || native.inspect == nil {
		return partial, fmt.Errorf("native shard requires context, compiler, fresh output and complete runtime")
	}
	if err := validateCIShardPlanShape(plan); err != nil {
		return partial, err
	}
	if shardIndex < 0 || shardIndex >= len(plan.Shards) {
		return partial, fmt.Errorf("native shard index outside plan")
	}
	if len(inventory.Suites) != len(plan.Suites) || len(inventory.Workspaces) != len(plan.Suites) ||
		inventory.Lock.CompilerRevision != plan.Run.CompilerRevision || inventory.Lock.GoVersion != plan.Run.GoVersion || runtime.Version() != plan.Run.GoVersion {
		return partial, fmt.Errorf("native shard inventory shape/toolchain differs from plan")
	}
	if err := nativeCaseProfile(plan.Profiles.Case).validateCompiler(inventory.Lock); err != nil {
		return partial, err
	}
	if err := nativeWorkspaceProfile(plan.Profiles.Workspace).validateCompiler(inventory.Lock); err != nil {
		return partial, err
	}
	for index, s := range inventory.Suites {
		expected := plan.Suites[index]
		if s.Package != expected.Package || s.Activity != expected.Activity || int64(len(s.Cases)) != expected.Cases {
			return partial, fmt.Errorf("native shard inventory differs at original suite %d", index)
		}
	}
	if err := ctx.Err(); err != nil {
		return partial, err
	}
	root, err := filepath.Abs(out)
	if err != nil {
		return partial, err
	}
	if err := os.Mkdir(root, 0755); err != nil {
		return partial, fmt.Errorf("native shard requires fresh output: %w", err)
	}
	assignment := plan.Shards[shardIndex]
	partial = ciShardPartial{Schema: ciShardPartialSchema, PlanSHA256: plan.PlanSHA256, Run: plan.Run,
		Index: shardIndex, Namespace: assignment.Namespace, Sources: map[string]ciShardSource{}, State: "INCOMPLETE", Suites: []ciShardSuiteRecord{}}
	for name, source := range plan.Sources {
		partial.Sources[name] = source
	}
	indices := append([]int(nil), assignment.OriginalIndices...)
	selected := make([]suite, len(indices))
	for local, original := range indices {
		selected[local] = inventory.Suites[original]
		partial.Suites = append(partial.Suites, ciShardEmptySuiteRecord(plan, original))
	}
	defer func() {
		if returnedErr != nil {
			path := filepath.Join(root, "executor-error.txt")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err == nil {
				// Keep the complete first/reader/cleanup errors here; only the
				// JSON suite diagnostic has the bounded schema representation.
				_, err = file.WriteString(returnedErr.Error() + "\n")
				err = errors.Join(err, file.Close())
			}
			if err != nil {
				returnedErr = errors.Join(returnedErr, fmt.Errorf("preserve executor diagnostic: %w", err))
			}
		}
	}()
	identity, err := native.identify(ctx, compiler, filepath.Join(root, "compiler.json"), "version", "--build", "--json")
	if err != nil {
		return partial, fmt.Errorf("native shard compiler identity command: %w", err)
	}
	if err := validateCIShardCompilerIdentity(identity, inventory.Lock); err != nil {
		return partial, err
	}
	cache, err := native.newCache(root)
	if err != nil || cache == nil {
		if cache != nil {
			err = errors.Join(err, native.closeCache(cache))
		}
		return partial, fmt.Errorf("native shard private build cache: %w", errors.Join(err, errors.New("cache unavailable")))
	}
	// Each slot has exactly one callback writer. Only the joined coordinator's
	// caller reads these attempts, including on its error paths.
	type attempt struct {
		started bool
		err     error
	}
	attempts := make([]attempt, len(indices))
	_, runErr := runNativeSuites(ctx, selected, plan.Run.Workers, func(worker context.Context, local int, s suite) (nativeSuiteResult, error) {
		original := indices[local]
		attempts[local].started = true
		verifier := newNativeSuiteVerifier(ctx, compiler, ciShardSuiteDirectory(root, shardIndex, original), inventory.Lock, cache)
		verifier.caseProfile = nativeCaseProfile(plan.Profiles.Case)
		verifier.compactBatch = compactCIShardReceipts
		verifier.batchDirectory = func(batch int) string { return ciShardBatchDirectory(root, shardIndex, original, batch) }
		result, err := native.verify(worker, original, verifier, s, inventory.Workspaces[original])
		if err == nil {
			observed, expected := result.Observation, plan.Suites[original]
			if observed.Package != s.Package || observed.Activity != s.Activity || int64(observed.Cases) != expected.Cases || int64(observed.Passed) != expected.Cases || int64(observed.Batches) != expected.Batches || !observed.ReplayEqual {
				err = fmt.Errorf("original suite %d observation is incomplete or foreign", original)
			}
		}
		attempts[local].err = err
		return result, err
	})
	// Existing coordinator waits for every started callback, even on failure or
	// parent cancellation. Never close the runner-private cache before this join.
	partial.Joined = true
	cacheErr := native.closeCache(cache)
	var inspectionErrors []error
	failed := false
	allVerified := true
	for local, original := range indices {
		record := partial.Suites[local]
		attempt := attempts[local]
		if attempt.started {
			switch {
			case attempt.err == nil:
				record.State = "VERIFIED"
			case errors.Is(attempt.err, context.Canceled) || errors.Is(attempt.err, context.DeadlineExceeded):
				record.State = "CANCELED"
			default:
				record.State, record.Error = "FAILED", ciShardExecutionDiagnostic(attempt.err)
			}
			actual, inspectErr := native.inspect(plan, shardIndex, original, inventory, root, record)
			// Re-inspection may reject an incomplete tail after the producer
			// returned or was canceled. Preserve its verified prefix while
			// retaining failure; a reader must never upgrade a producer state.
			sameIdentity := actual.OriginalIndex == record.OriginalIndex && actual.Package == record.Package && actual.Activity == record.Activity
			allowedState := actual.State == record.State || (actual.State == "FAILED" && inspectErr != nil && actual.Error != "" && (record.State == "VERIFIED" || record.State == "CANCELED"))
			if !sameIdentity || !allowedState {
				inspectErr = errors.Join(inspectErr, fmt.Errorf("reader returned a foreign suite identity/state"))
			} else {
				record = actual
			}
			if record.State == "FAILED" && record.Error != "" {
				record.Error = ciShardExecutionDiagnostic(errors.New(record.Error))
			}
			if err := validateCIShardSuite(plan, shardIndex, record); err != nil {
				inspectErr = errors.Join(inspectErr, err)
			}
			if inspectErr != nil {
				inspectionErrors = append(inspectionErrors, fmt.Errorf("original suite %d evidence: %w", original, inspectErr))
				if record.State == "VERIFIED" {
					record.State, record.Error = "FAILED", ciShardExecutionDiagnostic(inspectErr)
				} else if record.State == "FAILED" {
					record.Error = ciShardExecutionDiagnostic(errors.Join(attempt.err, inspectErr))
				}
			}
		}
		partial.Suites[local] = record
		failed = failed || record.State == "FAILED"
		allVerified = allVerified && record.State == "VERIFIED"
	}
	returnedErr = errors.Join(runErr, errors.Join(inspectionErrors...), cacheErr, ctx.Err())
	switch {
	case cacheErr != nil:
		partial.State = "INCOMPLETE"
	case failed:
		partial.State = "FAILED"
	case allVerified && returnedErr == nil:
		partial.State = "SUCCEEDED"
	}
	return partial, returnedErr
}
