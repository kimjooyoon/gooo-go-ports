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
	execute     func(context.Context, string, string, ...string) ([]byte, error)
	simpleFold  func(context.Context, string, []byte, string, *nativeBuildCache) (simpleFoldExhaustiveReport, error)
	ycbcr       func(context.Context, string, []byte, string, string, *nativeBuildCache) (ycbcrExhaustiveReport, error)
	caseMapping func(context.Context, string, []byte, string, string, *nativeBuildCache) (caseMappingExhaustiveReport, error)
}

func newNativeSuiteVerifier(commands context.Context, compiler, root string, lock upstream, cache *nativeBuildCache) nativeSuiteVerifier {
	return nativeSuiteVerifier{commands: commands, compiler: compiler, root: root, lock: lock, cache: cache,
		execute: execute, simpleFold: checkSimpleFoldExhaustive,
		ycbcr: checkYCbCrExhaustive, caseMapping: checkCaseMappingExhaustive}
}

func (v nativeSuiteVerifier) verify(ctx context.Context, s suite, selected suiteWorkspace) (result nativeSuiteResult, returnedErr error) {
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
		identity := expectedIdentity{CompilerRevision: v.lock.CompilerRevision, GoVersion: v.lock.GoVersion, ManifestDigest: "sha256:" + manifestSHA, CasesDigest: "sha256:" + casesSHA, Sources: selected.Identities}
		receipt := filepath.Join(dir, "execution.json")
		pendingTiming.phase("execute")
		raw, err := v.execute(v.commands, v.compiler, receipt, "package", "execute", "--json", "--cases", cases, workspace)
		if err != nil {
			return result, err
		}
		pendingTiming.phase("inspect_execution")
		original, err := inspect(raw, batch, false, identity)
		if err != nil {
			return result, fmt.Errorf("%s: %w", receipt, err)
		}
		replayPath := filepath.Join(dir, "replay.json")
		pendingTiming.phase("replay")
		raw, err = v.execute(v.commands, v.compiler, replayPath, "package", "replay", "--json", "--receipt", receipt, "--cases", cases, workspace)
		if err != nil {
			return result, err
		}
		pendingTiming.phase("inspect_replay")
		replay, err := inspect(raw, batch, true, identity)
		if err != nil {
			return result, fmt.Errorf("%s: %w", replayPath, err)
		}
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
