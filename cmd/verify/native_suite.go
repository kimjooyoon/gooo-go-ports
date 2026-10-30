package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Configuration and frozen workspaces are read-only once workers start.
// Each callback owns its observation, receipts and exhaustive reports. The
// private Go build cache is safe for same-machine concurrent Go processes and
// remains alive until every callback has returned. Worker failure cancellation
// is observed between complete batches; active commands use the parent context
// so a later-suite failure does not kill a CLI and strand its native children.
type nativeSuiteVerifier struct {
	commands    context.Context
	compiler    string
	root        string
	lock        upstream
	cache       *nativeBuildCache
	timing      *nativeTiming
	caseProfile nativeCaseProfile
	// nil retains the original full-verifier evidence layout exactly.
	batchDirectory func(int) string
	// Experimental at-rest compaction after all per-batch/exhaustive checks.
	// nil preserves every original full-mode file and byte unchanged.
	compactBatch func(string) error
	// Optional waited-command accounting for an explicit shard. nil preserves
	// the original full timing path and its context exactly.
	commandObserver func(int, string, nativeCommandResources)
	execute         func(context.Context, string, string, ...string) ([]byte, error)
	// Explicit experimental transport/inspection profile; the zero value keeps
	// the immutable upstream producer and stateless runtime/v1 inspection.
	compilerRevisionOverride string
	inspectExecution         func([]byte, suite, bool, expectedIdentity, *nativeReportedRuntime) (checked, error)
	simpleFold               func(context.Context, string, []byte, string, *nativeBuildCache) (simpleFoldExhaustiveReport, error)
	ycbcr                    func(context.Context, string, []byte, string, string, *nativeBuildCache) (ycbcrExhaustiveReport, error)
	caseMapping              func(context.Context, string, []byte, string, string, *nativeBuildCache) (caseMappingExhaustiveReport, error)
}

func newNativeSuiteVerifier(commands context.Context, compiler, root string, lock upstream, cache *nativeBuildCache) nativeSuiteVerifier {
	return nativeSuiteVerifier{commands: commands, compiler: compiler, root: root, lock: lock, cache: cache,
		execute: execute, simpleFold: checkSimpleFoldExhaustive,
		ycbcr: checkYCbCrExhaustive, caseMapping: checkCaseMappingExhaustive}
}

func (v nativeSuiteVerifier) commandContext(parent context.Context, timing *nativeBatchTiming, batch int, phase string) context.Context {
	observed := timing.commandContext(parent, phase)
	if v.commandObserver == nil {
		return observed
	}
	previous, _ := observed.Value(nativeCommandResourcesContextKey{}).(func(nativeCommandResources))
	return withNativeCommandResources(observed, func(resources nativeCommandResources) {
		if previous != nil {
			previous(resources)
		}
		v.commandObserver(batch, phase, resources)
	})
}

func (v nativeSuiteVerifier) verify(ctx context.Context, s suite, selected suiteWorkspace) (result nativeSuiteResult, returnedErr error) {
	if (v.compilerRevisionOverride != "") != (v.inspectExecution != nil) {
		return result, fmt.Errorf("experimental native transport requires an explicit producer and receipt inspector")
	}
	if v.compilerRevisionOverride != "" && (v.caseProfile != "" || v.timing != nil || v.commandObserver != nil) {
		return result, fmt.Errorf("experimental session requires legacy cases and separately scoped lifecycle diagnostics")
	}
	var pendingTiming *nativeBatchTiming
	defer func() {
		if pendingTiming != nil {
			// A panic leaves returnedErr nil and verified false: diagnostic incomplete, never success.
			pendingTiming.finish(returnedErr, ctx.Err(), false)
		}
	}()
	r := observation{Package: s.Package, Activity: s.Activity, Domain: s.Domain, Cases: len(s.Cases), ReplayEqual: true}
	casePolicy, err := v.caseProfile.policy(s, selected, v.lock)
	if err != nil {
		return result, err
	}
	for start := 0; start < len(s.Cases); {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		prepared, err := casePolicy.prepare(s, start)
		if err != nil {
			return result, err
		}
		end := prepared.End
		pendingTiming = v.timing.beginBatch(s, r.Batches, start, end)
		pendingTiming.phase("prepare")
		batch := s
		batch.Cases = s.Cases[start:end]
		dir := filepath.Join(suiteEvidenceDirectory(v.root, s), fmt.Sprintf("%03d", r.Batches))
		if v.batchDirectory != nil {
			dir = v.batchDirectory(r.Batches)
		}
		if err = os.MkdirAll(dir, 0755); err != nil {
			return result, err
		}
		for _, source := range selected.Names {
			b := selected.Sources[source]
			path := filepath.Join(dir, source)
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return result, err
			}
			if err = os.WriteFile(path, b, 0644); err != nil {
				return result, err
			}
		}
		workspace := filepath.Join(dir, "gooo.workspace.json")
		cases := filepath.Join(dir, "cases.json")
		if err = save(workspace, selected.Manifest); err != nil {
			return result, err
		}
		if err = writeNativeCaseBatch(cases, batch, prepared); err != nil {
			return result, err
		}
		manifestSHA, err := digestFile(workspace)
		if err != nil {
			return result, err
		}
		casesSHA, err := digestFile(cases)
		if err != nil {
			return result, err
		}
		producerRevision := v.lock.CompilerRevision
		if v.compilerRevisionOverride != "" {
			producerRevision = v.compilerRevisionOverride
		}
		identity := expectedIdentity{CompilerRevision: producerRevision, GoVersion: v.lock.GoVersion, ManifestDigest: "sha256:" + manifestSHA, CasesDigest: "sha256:" + casesSHA, Sources: selected.Identities, SourceBytes: selected.Sources}
		receipt := filepath.Join(dir, "execution.json")
		pendingTiming.phase("execute")
		executeArgs := []string{"package", "execute", "--json", "--cases", cases}
		if isHTTPConditionalSuite(s) {
			executeArgs = append(executeArgs, "--go", etagGoToolPath())
		}
		executeArgs = append(executeArgs, workspace)
		raw, err := v.execute(v.commandContext(v.commands, pendingTiming, r.Batches, "execute"), v.compiler, receipt, executeArgs...)
		if err != nil {
			return result, err
		}
		pendingTiming.phase("inspect_execution")
		var executeReported *nativeReportedRuntime
		if pendingTiming != nil {
			executeReported = &nativeReportedRuntime{}
		}
		original, err := v.inspectReceipt(raw, batch, false, identity, executeReported)
		if err != nil {
			return result, fmt.Errorf("%s: %w", receipt, err)
		}
		pendingTiming.recordReportedRuntime("execute", executeReported)
		replayPath := filepath.Join(dir, "replay.json")
		pendingTiming.phase("replay")
		replayArgs := []string{"package", "replay", "--json", "--receipt", receipt, "--cases", cases}
		if isHTTPConditionalSuite(s) {
			replayArgs = append(replayArgs, "--go", etagGoToolPath())
		}
		replayArgs = append(replayArgs, workspace)
		raw, err = v.execute(v.commandContext(v.commands, pendingTiming, r.Batches, "replay"), v.compiler, replayPath, replayArgs...)
		if err != nil {
			return result, err
		}
		pendingTiming.phase("inspect_replay")
		var replayReported *nativeReportedRuntime
		if pendingTiming != nil {
			replayReported = &nativeReportedRuntime{}
		}
		replay, err := v.inspectReceipt(raw, batch, true, identity, replayReported)
		if err != nil {
			return result, fmt.Errorf("%s: %w", replayPath, err)
		}
		pendingTiming.recordReportedRuntime("replay", replayReported)
		pendingTiming.phase("receipt_binding")
		h, err := digestFile(receipt)
		if err != nil {
			return result, err
		}
		if err := inspectReplayBinding(original, replay, "sha256:"+h); err != nil {
			return result, err
		}
		pendingTiming.bind(identity, "sha256:"+h)
		if s.Package == "go/unicode" && s.Activity == "SimpleFold" && r.Batches == 0 {
			pendingTiming.phase("exhaustive")
			exhaustive, err := v.simpleFold(v.commands, filepath.Join(v.root, "simple-fold-exhaustive"), raw, original.Generated, v.cache)
			if err != nil {
				return result, err
			}
			result.SimpleFold = &exhaustive
		}
		if s.Package == "go/image/color" && (s.Activity == "RGBToYCbCr" || s.Activity == "YCbCrToRGB") && r.Batches == 0 {
			pendingTiming.phase("exhaustive")
			exhaustive, err := v.ycbcr(v.commands, filepath.Join(v.root, "ycbcr-exhaustive-"+s.Activity), raw, original.Generated, s.Activity, v.cache)
			if err != nil {
				return result, err
			}
			result.YCbCr = &exhaustive
		}
		if s.Package == "go/unicode" && (s.Activity == "To" || s.Activity == "ToUpper" || s.Activity == "ToLower" || s.Activity == "ToTitle") && r.Batches == 0 {
			pendingTiming.phase("exhaustive")
			exhaustive, err := v.caseMapping(v.commands, filepath.Join(v.root, "case-mapping-exhaustive-"+s.Activity), raw, original.Generated, s.Activity, v.cache)
			if err != nil {
				return result, err
			}
			result.CaseMapping = &exhaustive
		}
		if v.compactBatch != nil {
			if err := v.compactBatch(dir); err != nil {
				return result, err
			}
		}
		r.Batches++
		r.Passed += len(batch.Cases)
		start = end
		pendingTiming.finish(nil, ctx.Err(), true)
		pendingTiming = nil
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Observation = r
	return result, nil
}

func (v nativeSuiteVerifier) inspectReceipt(raw []byte, s suite, replay bool, identity expectedIdentity, reported *nativeReportedRuntime) (checked, error) {
	if v.inspectExecution != nil {
		return v.inspectExecution(raw, s, replay, identity, reported)
	}
	return inspectWithRuntime(raw, s, replay, identity, reported)
}
