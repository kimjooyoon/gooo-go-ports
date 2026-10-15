package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Synthetic receipts exercise the production writer, inspector and saved replay
// binding. These tests execute no compiler or generated program and establish no
// native capacity, performance or fresh hosted CI acceptance.
func caseProfileIntegrationFixture(t *testing.T, entry suite, count int) ([]byte, suite, expectedIdentity, suiteWorkspace) {
	t.Helper()
	var expected any
	switch entry.Package + ":" + entry.Activity {
	case uuidPackage + ":Validate":
		expected = uuidValidationOracle(uuidStandardSample)
	case uuidConsumerPackage + ":Inspect":
		expected = uuidInspectorOracle(uuidStandardSample)
	case uuidParsePackage + ":ParseCanonical":
		expected = uuidParseCanonicalOracle(uuidStandardSample)
	default:
		t.Fatal("integration fixture requires a reviewed UUID entry")
	}
	entry.Cases = []testCase{{Inputs: []any{uuidStandardSample}, Expected: expected}}
	raw, one, identity := contractFixture(t, entry)
	var receipt envelope
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	// The original generic fixture uses an older synthetic producer. Keep its
	// independent source/field contracts while binding this fixture to the one
	// compiler revision admitted by the explicit compact profile.
	identity.CompilerRevision = uuidCompactCompilerRevision
	receipt.Result.Runtime.Producer = identity.CompilerRevision
	raw = mustJSON(t, receipt)
	if _, err := inspect(raw, one, false, identity); err != nil {
		t.Fatal("invalid pinned synthetic fixture", err)
	}
	s := repeatNativeCase(one, count)
	return raw, s, identity, fakeNativeWorkspace(t, s, identity)
}

type caseProfileIntegrationBatch struct {
	end int
	raw []byte
}

func caseProfileIntegrationPlan(t *testing.T, profile nativeCaseProfile, s suite, workspace suiteWorkspace, lock upstream) []caseProfileIntegrationBatch {
	t.Helper()
	policy, err := profile.policy(s, workspace, lock)
	if err != nil {
		t.Fatal(err)
	}
	var planned []caseProfileIntegrationBatch
	for start := 0; start < len(s.Cases); {
		prepared, err := policy.prepare(s, start)
		if err != nil || prepared.End <= start || prepared.End > len(s.Cases) {
			t.Fatal("invalid prepared batch", start, prepared.End, err)
		}
		batch := s
		batch.Cases = s.Cases[start:prepared.End]
		var expected []byte
		if profile == "" {
			if prepared.Raw != nil {
				t.Fatal("legacy profile replaced the original save path")
			}
			expected, err = json.MarshalIndent(caseDocument(batch), "", "  ")
		} else {
			if prepared.Raw == nil {
				t.Fatal("compact UUID batch has no sole writer bytes")
			}
			expected, err = json.Marshal(caseDocument(batch))
		}
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, '\n')
		if profile != "" && !bytes.Equal(prepared.Raw, expected) {
			t.Fatal("prepared compact bytes differ from the specified wire format")
		}
		planned = append(planned, caseProfileIntegrationBatch{end: prepared.End, raw: expected})
		start = prepared.End
	}
	return planned
}

func caseProfileIntegrationCommand(t *testing.T, root string, s suite, planned []caseProfileIntegrationBatch, fake *fakeNativeCommand) func(context.Context, string, string, ...string) ([]byte, error) {
	t.Helper()
	byDirectory := map[string][]byte{}
	for index, batch := range planned {
		dir := filepath.Join(suiteEvidenceDirectory(root, s), fmt.Sprintf("%03d", index))
		byDirectory[dir] = batch.raw
	}
	return func(ctx context.Context, compiler, path string, args ...string) ([]byte, error) {
		if len(args) < 2 {
			t.Fatal("missing native command action")
		}
		dir := filepath.Dir(path)
		wantRaw, ok := byDirectory[dir]
		if !ok {
			t.Fatal("command used an unplanned batch directory", dir)
		}
		cases := filepath.Join(dir, "cases.json")
		workspace := filepath.Join(dir, "gooo.workspace.json")
		var want []string
		switch args[1] {
		case "execute":
			want = []string{"package", "execute", "--json", "--cases", cases, workspace}
			if path != filepath.Join(dir, "execution.json") {
				t.Fatal("execution receipt path changed")
			}
		case "replay":
			want = []string{"package", "replay", "--json", "--receipt", filepath.Join(dir, "execution.json"), "--cases", cases, workspace}
			if path != filepath.Join(dir, "replay.json") {
				t.Fatal("saved replay output path changed")
			}
		default:
			t.Fatal("unexpected native action", args[1])
		}
		if !reflect.DeepEqual(args, want) {
			t.Fatal("native invocation lost the exact saved cases/workspace/receipt arguments", args, want)
		}
		raw, err := os.ReadFile(cases)
		if err != nil || !bytes.Equal(raw, wantRaw) {
			t.Fatal("writer changed sized case bytes before native execution or replay", err)
		}
		return fake.execute(ctx, compiler, path, args...)
	}
}

func caseProfileIntegrationTiming(t *testing.T, root string, s suite, lock upstream, profile nativeCaseProfile) (*nativeTiming, string) {
	t.Helper()
	cfg := nativeTimingTestConfig(t, root, []suite{s})
	cfg.CaseProfile = profile
	cfg.Lock.CompilerRevision, cfg.Lock.GoVersion = lock.CompilerRevision, lock.GoVersion
	// Keep the production source/pin observer. Only the pre-existing test binary
	// observer uses an inert file; no actual host executable is launched.
	if err := os.WriteFile(filepath.Join(cfg.RepositoryRoot, "upstream.json"), mustJSON(t, cfg.Lock), 0644); err != nil {
		t.Fatal(err)
	}
	cfg.CompilerIdentity = mustJSON(t, map[string]any{
		"version": cfg.Lock.CompilerVersion, "compiler_source_sha": cfg.Lock.CompilerRevision,
		"go_version": cfg.Lock.GoVersion, "native_go_required": cfg.Lock.GoVersion, "vcs_modified": "false",
	})
	recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recorder.close(fmt.Errorf("synthetic profile integration cleanup"), nil) })
	return recorder, cfg.Output
}

func TestUUIDCompactCaseProfileWritesExactSavedCasesAndBindsEveryFreshReplay(t *testing.T) {
	for _, entry := range []suite{
		{Package: uuidPackage, Activity: "Validate"},
		{Package: uuidConsumerPackage, Activity: "Inspect"},
		{Package: uuidParsePackage, Activity: "ParseCanonical"},
	} {
		t.Run(entry.Package+":"+entry.Activity, func(t *testing.T) {
			raw, s, identity, workspace := caseProfileIntegrationFixture(t, entry, 129)
			lock := upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}
			planned := caseProfileIntegrationPlan(t, uuidCompactCaseProfile, s, workspace, lock)
			if len(planned) < 2 || planned[len(planned)-1].end != len(s.Cases) {
				t.Fatal("129 original cases were not completely partitioned under the row cap")
			}
			root := t.TempDir()
			fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
			ctx := workerTestContext(t)
			verifier := newNativeSuiteVerifier(ctx, "unused", root, lock, nil)
			verifier.caseProfile = uuidCompactCaseProfile
			verifier.execute = caseProfileIntegrationCommand(t, root, s, planned, &fake)
			result, err := verifier.verify(ctx, s, workspace)
			if err != nil || result.Observation.Cases != len(s.Cases) || result.Observation.Passed != len(s.Cases) || result.Observation.Batches != len(planned) || !result.Observation.ReplayEqual {
				t.Fatal("compact profile lost inspected case/replay coverage", result, err)
			}
			if len(fake.calls) != len(planned) {
				t.Fatal("native commands skipped or added a batch", fake.calls)
			}
			for index, batch := range planned {
				dir := filepath.Join(suiteEvidenceDirectory(root, s), fmt.Sprintf("%03d", index))
				if !reflect.DeepEqual(fake.calls[dir], []string{"execute", "replay"}) {
					t.Fatal("every batch must execute and replay its saved receipt exactly once", fake.calls[dir])
				}
				executionRaw, err := os.ReadFile(filepath.Join(dir, "execution.json"))
				if err != nil {
					t.Fatal(err)
				}
				var execution, replay envelope
				if err := json.Unmarshal(executionRaw, &execution); err != nil {
					t.Fatal(err)
				}
				if err := readJSON(filepath.Join(dir, "replay.json"), &replay); err != nil {
					t.Fatal(err)
				}
				if execution.CasesDigest != contentDigest(batch.raw) || replay.CasesDigest != contentDigest(batch.raw) || replay.ReplayedFrom != contentDigest(executionRaw) {
					t.Fatal("exact written cases or original saved receipt hash was not retained")
				}
			}
		})
	}
}

func TestUUIDCompactCaseProfileCannotAcceptRewrittenInputCaseDigestOrSavedReplay(t *testing.T) {
	for _, test := range []struct{ name, action, field, reason string }{
		{"execute input", "execute", "input", "single input"},
		{"replay input", "replay", "input", "single input"},
		{"execute case digest", "execute", "cases_digest", "workspace or case file binding"},
		{"replay case digest", "replay", "cases_digest", "workspace or case file binding"},
		{"saved receipt link", "replay", "replayed_from_sha256", "saved replay is not bound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, s, identity, workspace := caseProfileIntegrationFixture(t, suite{Package: uuidPackage, Activity: "Validate"}, 2)
			lock := upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}
			planned := caseProfileIntegrationPlan(t, uuidCompactCaseProfile, s, workspace, lock)
			root := t.TempDir()
			mutations := 0
			fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
			fake.mutate = func(action string, receipt map[string]any) {
				if action != test.action {
					return
				}
				mutations++
				if test.field == "input" {
					runtime := receipt["result"].(map[string]any)["runtime"].(map[string]any)
					traces := runtime["traces"].([]any)
					last := traces[len(traces)-1].(map[string]any)
					delivery := last["deliveries"].([]any)[0].(map[string]any)
					delivery["input"] = uuidStandardSample + " "
				} else {
					receipt[test.field] = "sha256:" + strings.Repeat("0", 64)
				}
			}
			ctx := workerTestContext(t)
			verifier := newNativeSuiteVerifier(ctx, "unused", root, lock, nil)
			verifier.caseProfile = uuidCompactCaseProfile
			verifier.execute = caseProfileIntegrationCommand(t, root, s, planned, &fake)
			result, err := verifier.verify(ctx, s, workspace)
			if err == nil || result.Observation.Passed != 0 || result.Observation.Batches != 0 || mutations != 1 {
				t.Fatal("rewritten evidence became a passed compact observation", result, err, mutations)
			}
			if !strings.Contains(err.Error(), test.reason) {
				t.Fatal("tampered receipt was refused for an unrelated reason", test.reason, err)
			}
			dir := filepath.Join(suiteEvidenceDirectory(root, s), "000")
			wantCalls := []string{"execute"}
			if test.action == "replay" {
				wantCalls = append(wantCalls, "replay")
			}
			if len(fake.calls) != 1 || !reflect.DeepEqual(fake.calls[dir], wantCalls) {
				t.Fatal("invalid evidence did not stop at its failing inspected stage", fake.calls)
			}
		})
	}
}

func TestCaseProfileTimingIsDiagnosticAndLegacyProfileFieldsRemainAbsent(t *testing.T) {
	for _, profile := range []nativeCaseProfile{"", uuidCompactCaseProfile} {
		name := string(profile)
		if name == "" {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			raw, s, identity, workspace := caseProfileIntegrationFixture(t, suite{Package: uuidPackage, Activity: "Validate"}, 129)
			lock := upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}
			planned := caseProfileIntegrationPlan(t, profile, s, workspace, lock)
			var evidence [2]map[string][]byte
			var observations [2]nativeSuiteResult
			for mode := range 2 {
				root := timingIntegrationRoot(t)
				var timing *nativeTiming
				var timingPath string
				if mode == 1 {
					timing, timingPath = caseProfileIntegrationTiming(t, root, s, lock, profile)
				}
				fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
				ctx := workerTestContext(t)
				verifier := newNativeSuiteVerifier(ctx, "unused", root, lock, nil)
				verifier.caseProfile, verifier.timing = profile, timing
				verifier.execute = caseProfileIntegrationCommand(t, root, s, planned, &fake)
				span := timing.beginSuite(0, s)
				result, err := verifier.verify(ctx, s, workspace)
				span.finish(result, err, ctx.Err())
				if err != nil {
					t.Fatal(err)
				}
				summary := map[string]any{"schema": "synthetic-case-profile-integration/v1", "observation": result.Observation}
				originalSummary := mustJSON(t, summary)
				profile.decorateSummary(summary)
				if profile == "" {
					if _, exists := summary["case_profile"]; exists || !bytes.Equal(originalSummary, mustJSON(t, summary)) {
						t.Fatal("legacy summary gained a profile field or changed bytes")
					}
				} else if !equalValue(mustJSON(t, summary["case_profile"]), profile.identity()) {
					t.Fatal("compact summary lost its explicit profile identity")
				}
				if err := save(filepath.Join(root, "summary.json"), summary); err != nil {
					t.Fatal(err)
				}
				if err := timing.close(nil, ctx.Err()); err != nil {
					t.Fatal(err)
				}
				if len(fake.calls) != len(planned) || result.Observation.Passed != len(s.Cases) || result.Observation.Batches != len(planned) {
					t.Fatal("timing changed inspected compact/legacy coverage", result, fake.calls)
				}
				for _, calls := range fake.calls {
					if !reflect.DeepEqual(calls, []string{"execute", "replay"}) {
						t.Fatal("timing changed the mandatory execution/replay pair", calls)
					}
				}
				if mode == 1 {
					rows := timingIntegrationRows(t, timingPath)
					header, footer := rows[0], rows[len(rows)-1]
					if footer["status"] != "COMPLETE" || footer["verified_batches"] != float64(len(planned)) || footer["verified_cases"] != float64(len(s.Cases)) {
						t.Fatal("timing completion differs from actual inspected observations", footer)
					}
					if profile == "" {
						if _, exists := header["case_profile"]; exists {
							t.Fatal("legacy timing header gained a profile field")
						}
						if header["boundary_scope"] != "Actual contiguous batch ranges are compared with the original successful suite observation. Exact original boundaries rely on unchanged nativeBatchEnd/loop source and the original full-summary gate." {
							t.Fatal("legacy timing boundary description changed")
						}
					} else if !equalValue(mustJSON(t, header["case_profile"]), profile.identity()) || !strings.Contains(fmt.Sprint(header["boundary_scope"]), "no original UUID batch boundaries") {
						t.Fatal("compact timing lacks distinct identity/boundary limitations")
					}
				}
				observations[mode] = result
				evidence[mode] = timingIntegrationEvidence(t, root, timingPath)
			}
			if !reflect.DeepEqual(observations[0], observations[1]) || !reflect.DeepEqual(evidence[0], evidence[1]) {
				t.Fatal("optional timing changed original observations or any saved case/source/receipt/summary bytes")
			}
		})
	}
}
