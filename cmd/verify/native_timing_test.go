package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func nativeTimingTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func nativeTimingTestConfig(t *testing.T, evidenceRoot string, suites []suite) nativeTimingConfig {
	t.Helper()
	evidence, err := filepath.EvalSymlinks(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	repo := nativeTimingTestRoot(t)
	write := nativeTimingFixtureWriter(t, repo)
	for _, name := range nativeTimingSourceNames {
		write(name, []byte("fixture source: "+name+"\n"))
	}
	port := "ports/timing.gooo"
	portBytes := []byte("package go/example/timing\n")
	write(port, portBytes)
	catalog := verificationCatalog{Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "go/example/timing", Sources: []string{port}}}}
	sources := map[string][]byte{port: portBytes}
	compilerRevision := strings.Repeat("a", 40)
	for _, s := range suites {
		contract, err := contractFor(s)
		if err != nil {
			continue // Recorder-only fixtures deliberately have no native contract.
		}
		if _, exists := sources[contract.Source]; exists {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../..", contract.Source))
		if err != nil {
			t.Fatal(err)
		}
		sources[contract.Source] = raw
		write(contract.Source, raw)
		catalog.Packages = append(catalog.Packages, sourcePackage{Path: s.Package, Sources: []string{contract.Source}})
		_, _, originalIdentity := fixture(t)
		compilerRevision = originalIdentity.CompilerRevision
	}
	catalogBytes, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	write("verification-catalog.json", catalogBytes)
	lock := upstream{GoVersion: runtime.Version(), CompilerVersion: "timing-fixture", CompilerRevision: compilerRevision, SourceSHA: map[string]string{"src/example.go": strings.Repeat("b", 64)}}
	lockBytes, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	write("upstream.json", lockBytes)
	write("fixture-compiler", []byte("this fixture is hashed, never executed\n"))
	identity, err := json.Marshal(map[string]any{"version": lock.CompilerVersion, "compiler_source_sha": lock.CompilerRevision, "go_version": lock.GoVersion, "native_go_required": lock.GoVersion, "vcs_modified": "false"})
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for name, raw := range sources {
		hashes[name] = nativeTimingBytes(raw).SHA256
	}
	return nativeTimingConfig{Output: filepath.Join(evidence, "native-timing.jsonl"), EvidenceRoot: evidence, RepositoryRoot: repo, Compiler: filepath.Join(repo, "fixture-compiler"), Workers: 2, Suites: suites, Lock: lock,
		Catalog: frozenVerificationCatalog{Catalog: catalog, Raw: catalogBytes, Sources: sources, Hashes: hashes}, CompilerIdentity: identity}
}

func nativeTimingFixtureWriter(t *testing.T, repo string) func(string, []byte) {
	t.Helper()
	expected, err := nativeExpectedVerificationContract()
	if err != nil {
		t.Fatal(err)
	}
	modes := nativeVerificationSourceModes(expected)
	return func(name string, raw []byte) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if name == nativeVerificationContractPath {
			raw = nativeVerificationContractJSON
		}
		mode := os.FileMode(0644)
		if modes[name] == "100755" {
			mode = 0755
		}
		if err := os.WriteFile(path, raw, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
}

// Reuse production source/pin checks; only the expensive host binary observer is
// replaced by a small file. The production binary observer is tested separately.
func nativeTimingTestSnapshot(cfg nativeTimingConfig) (nativeTimingSnapshot, error) {
	value, err := nativeTimingSourceIdentity(cfg)
	if err != nil {
		return value, err
	}
	file, err := nativeTimingHashFile(cfg.Compiler)
	if err != nil {
		return value, err
	}
	for _, name := range []string{"verifier", "go", "compiler"} {
		value.Binaries[name] = file
	}
	return value, nil
}

func TestNativeTimingLedgerRetainsSyntheticCatalogAndRejectsDiskImageDrift(t *testing.T) {
	cfg := nativeTimingTestConfig(t, nativeTimingTestRoot(t), []suite{nativeTimingTestSuite("Check", 1)})
	observed, err := nativeTimingSourceIdentity(cfg)
	if err != nil {
		t.Fatal("synthetic dynamic catalog was blocked", err)
	}
	if _, ok := observed.Sources["ports/timing.gooo"]; !ok {
		t.Fatal("dynamic catalog source was dropped")
	}
	path := filepath.Join(cfg.RepositoryRoot, nativeVerificationContractPath)
	if err := os.WriteFile(path, append(append([]byte{}, nativeVerificationContractJSON...), '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeTimingSourceIdentity(cfg); err == nil {
		t.Fatal("changed on-disk ledger accepted")
	}
}

func TestNativeTimingRejectsKnownExecutableModeDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Git executable mode requires caller Git check on Windows")
	}
	cfg := nativeTimingTestConfig(t, nativeTimingTestRoot(t), []suite{nativeTimingTestSuite("Check", 1)})
	if _, err := nativeTimingSourceIdentity(cfg); err != nil {
		t.Fatal(err)
	}
	expected, err := nativeExpectedVerificationContract()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range expected.Sources {
		if source.Mode != "100755" {
			continue
		}
		path := filepath.Join(cfg.RepositoryRoot, source.Path)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := nativeTimingSourceIdentity(cfg); err == nil {
			t.Fatal("known executable source bitflip accepted")
		}
		return
	}
	t.Fatal("executable fixture source missing")
}

// Shared with the separately owned production integration tests.
func timingRecorderForTest(t *testing.T, evidenceRoot string, suites []suite) (*nativeTiming, string) {
	t.Helper()
	cfg := nativeTimingTestConfig(t, evidenceRoot, suites)
	recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recorder.close(errors.New("test cleanup before explicit completion"), nil) })
	return recorder, cfg.Output
}

func nativeTimingTestSuite(activity string, cases int) suite {
	s := suite{Package: "example/timing", Activity: activity, Domain: "finite fixture"}
	for i := 0; i < cases; i++ {
		s.Cases = append(s.Cases, testCase{Inputs: []any{int64(i)}, Expected: int64(i)})
	}
	return s
}

func nativeTimingTestBinding(t *testing.T, recorder *nativeTiming) expectedIdentity {
	t.Helper()
	sources, err := snapshotSourceIdentities(recorder.cfg.Catalog.Catalog.Packages, recorder.cfg.Catalog.Sources)
	if err != nil {
		t.Fatal(err)
	}
	return expectedIdentity{CompilerRevision: recorder.cfg.Lock.CompilerRevision, GoVersion: recorder.cfg.Lock.GoVersion,
		ManifestDigest: contentDigest([]byte("actual workspace fixture")), CasesDigest: contentDigest([]byte("actual cases fixture")), Sources: sources}
}

func nativeTimingTestBatch(t *testing.T, recorder *nativeTiming, s suite, index, start, end int) *nativeBatchTiming {
	t.Helper()
	bt := recorder.beginBatch(s, index, start, end)
	if bt == nil {
		t.Fatal("valid batch could not start")
	}
	for _, name := range nativeTimingPhaseNames[1:6] {
		bt.phase(name)
	}
	bt.bind(nativeTimingTestBinding(t, recorder), contentDigest([]byte("actual execution receipt fixture")))
	return bt
}

func nativeTimingTestObservation(s suite, batches int) nativeSuiteResult {
	return nativeSuiteResult{Observation: observation{Package: s.Package, Activity: s.Activity, Domain: s.Domain, Cases: len(s.Cases), Passed: len(s.Cases), Batches: batches, ReplayEqual: true}}
}

func nativeTimingTestEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 4<<20)
	var events []map[string]any
	for scan.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal("invalid event JSON", err)
		}
		if row["schema"] != nativeTimingSchema || row["seq"] != float64(len(events)) {
			t.Fatal("event schema/sequence is not complete", row)
		}
		events = append(events, row)
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestNativeTimingPreservesIndicesAndCountsWithoutRecomputingBoundaries(t *testing.T) {
	a, b := nativeTimingTestSuite("First", 3), nativeTimingTestSuite("Second", 1)
	recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{a, b})
	bState := recorder.beginSuite(1, b)
	aState := recorder.beginSuite(0, a)
	nativeTimingTestBatch(t, recorder, a, 0, 0, 1).finish(nil, nil, true)
	nativeTimingTestBatch(t, recorder, b, 0, 0, 1).finish(nil, nil, true)
	bState.finish(nativeTimingTestObservation(b, 1), nil, nil)
	nativeTimingTestBatch(t, recorder, a, 1, 1, 3).finish(nil, nil, true)
	aState.finish(nativeTimingTestObservation(a, 2), nil, nil)
	if err := recorder.close(nil, nil); err != nil {
		t.Fatal(err)
	}
	events := nativeTimingTestEvents(t, path)
	header, footer := events[0], events[len(events)-1]
	if header["kind"] != "header" || header["expected_boundaries_recomputed"] != false || header["configured_workers"] != float64(2) {
		t.Fatal("incorrect header contract", header)
	}
	if header["suite_order"] != "catalog" || !reflect.DeepEqual(header["planned_dispatch_indices"], []any{float64(0), float64(1)}) {
		t.Fatal("default dispatch plan differs from original catalog indices", header)
	}
	encoded, _ := json.Marshal(header)
	if bytes.Contains(encoded, []byte(recorder.cfg.RepositoryRoot)) || bytes.Contains(encoded, []byte(recorder.cfg.EvidenceRoot)) {
		t.Fatal("private absolute paths leaked into the public header")
	}
	if footer["kind"] != "run_end" || footer["status"] != "COMPLETE" || footer["successful_suites"] != float64(2) || footer["verified_batches"] != float64(3) || footer["verified_cases"] != float64(4) || footer["event_count"] != float64(len(events)) || len(events) != 9 {
		t.Fatal("complete footer does not match independent counts", footer)
	}
	var ended []float64
	for _, event := range events {
		if event["kind"] == "suite_end" {
			ended = append(ended, event["suite_index"].(float64))
		}
		if event["kind"] == "batch_end" {
			if _, repeated := event["checked_identity"]; repeated {
				t.Fatal("batch events repeated the whole source set")
			}
			if _, repeated := event["source_set"]; repeated || event["source_set_sha256"] == "" {
				t.Fatal("batch did not use a compact source-set binding")
			}
			phases := event["phases"].([]any)
			if len(phases) != 7 || phases[6].(map[string]any)["observed"] != false {
				t.Fatal("optional exhaustive phase was invented")
			}
			var elapsed float64
			for _, phase := range phases {
				elapsed += phase.(map[string]any)["duration_ns"].(float64)
			}
			if elapsed != event["duration_ns"] || elapsed < 0 {
				t.Fatal("monotonic phase durations do not partition the batch interval")
			}
		}
	}
	if !reflect.DeepEqual(ended, []float64{1, 0}) {
		t.Fatal("out-of-order completion lost original suite indices", ended)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("output is not private", err)
	}
	if err := recorder.close(nil, nil); err != nil || len(nativeTimingTestEvents(t, path)) != len(events) {
		t.Fatal("closing twice rewrote the finite observation", err)
	}
}

func TestNativeTimingFreezesDispatchPlanAndRejectsMismatchBeforeOutput(t *testing.T) {
	suites := []suite{nativeTimingTestSuite("Small", 1), nativeTimingTestSuite("Large", 3), nativeTimingTestSuite("Tied", 3)}
	cfg := nativeTimingTestConfig(t, nativeTimingTestRoot(t), suites)
	cfg.SuiteOrder = nativeSuiteOrderLargestCorpusFirst
	cfg.DispatchOrder = []int{1, 2, 0}
	recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recorder.close(errors.New("test cleanup before explicit completion"), nil) })
	cfg.DispatchOrder[0] = 0
	cfg.Suites[0], cfg.Suites[1] = cfg.Suites[1], cfg.Suites[0]
	if !reflect.DeepEqual(recorder.cfg.DispatchOrder, []int{1, 2, 0}) {
		t.Fatal("caller mutation changed frozen dispatch plan", recorder.cfg.DispatchOrder)
	}
	if err := recorder.close(errors.New("fixture deliberately not executed"), nil); err == nil {
		t.Fatal("unexecuted dispatch fixture became a success")
	}
	header := nativeTimingTestEvents(t, cfg.Output)[0]
	if header["suite_order"] != "largest-corpus-first" || !reflect.DeepEqual(header["planned_dispatch_indices"], []any{float64(1), float64(2), float64(0)}) {
		t.Fatal("timing header did not retain selected plan", header)
	}
	for _, plan := range [][]int{{0, 1, 2}, {1, 1, 0}, {1, 2}, {1, 2, 3}} {
		broken := nativeTimingTestConfig(t, nativeTimingTestRoot(t), []suite{nativeTimingTestSuite("Small", 1), nativeTimingTestSuite("Large", 3), nativeTimingTestSuite("Tied", 3)})
		broken.SuiteOrder, broken.DispatchOrder = nativeSuiteOrderLargestCorpusFirst, plan
		if value, err := newNativeTimingWithSnapshot(broken, nativeTimingTestSnapshot); err == nil || value != nil || !strings.Contains(err.Error(), "dispatch plan differs") {
			t.Fatal("mismatched dispatch plan accepted", plan, err)
		}
		if _, err := os.Stat(broken.Output); !os.IsNotExist(err) {
			t.Fatal("refused dispatch plan created timing output", err)
		}
	}
}

func TestNativeTimingCannotAcceptGapOverlapDuplicateOrWrongOriginalSummary(t *testing.T) {
	for _, mode := range []string{"gap", "overlap", "duplicate_suite", "duplicate_batch", "duplicate_suite_finish", "duplicate_batch_finish", "missing_batch", "wrong_batches", "wrong_passed", "wrong_identity"} {
		t.Run(mode, func(t *testing.T) {
			s := nativeTimingTestSuite("Check", 3)
			recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
			st := recorder.beginSuite(0, s)
			if mode == "duplicate_suite" {
				recorder.beginSuite(0, s)
			}
			first := nativeTimingTestBatch(t, recorder, s, 0, 0, 1)
			first.finish(nil, nil, true)
			if mode == "duplicate_batch_finish" {
				first.finish(nil, nil, true)
			}
			start := 1
			if mode == "gap" {
				start = 2
			} else if mode == "overlap" {
				start = 0
			}
			if mode == "duplicate_batch" {
				recorder.beginBatch(s, 0, 0, 1)
			}
			if mode != "missing_batch" {
				bt := recorder.beginBatch(s, 1, start, 3)
				if bt != nil {
					for _, name := range nativeTimingPhaseNames[1:6] {
						bt.phase(name)
					}
					identity := nativeTimingTestBinding(t, recorder)
					if mode == "wrong_identity" {
						for name, source := range identity.Sources {
							source.Digest = contentDigest([]byte("different source"))
							identity.Sources[name] = source
						}
					}
					bt.bind(identity, contentDigest([]byte("execution")))
					bt.finish(nil, nil, true)
				}
			}
			result := nativeTimingTestObservation(s, 2)
			if mode == "wrong_batches" {
				result.Observation.Batches++
			} else if mode == "wrong_passed" {
				result.Observation.Passed--
			}
			st.finish(result, nil, nil)
			if mode == "duplicate_suite_finish" {
				st.finish(result, nil, nil)
			}
			if err := recorder.close(nil, nil); err == nil {
				t.Fatal("incomplete observation accepted")
			}
			events := nativeTimingTestEvents(t, path)
			footer := events[len(events)-1]
			if footer["kind"] != "run_end" || footer["complete"] != false || footer["status"] == "COMPLETE" {
				t.Fatal("invalid run produced complete footer", footer)
			}
		})
	}
}

func TestNativeTimingFailureCancellationPanicAndNotStartedAreNotSuccess(t *testing.T) {
	for _, mode := range []string{"failure", "cancel", "deadline", "panic", "never_started"} {
		t.Run(mode, func(t *testing.T) {
			s, later := nativeTimingTestSuite("Check", 1), nativeTimingTestSuite("Later", 1)
			recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s, later})
			var returnedErr, contextErr error
			if mode != "never_started" {
				st := recorder.beginSuite(0, s)
				bt := recorder.beginBatch(s, 0, 0, 1)
				if mode == "failure" {
					returnedErr = errors.New("last original comparison failed")
				} else if mode == "cancel" {
					returnedErr, contextErr = context.Canceled, context.Canceled
				} else if mode == "deadline" {
					returnedErr, contextErr = context.DeadlineExceeded, context.DeadlineExceeded
				}
				bt.finish(returnedErr, contextErr, false)
				if mode == "panic" {
					recorder.abort()
				} else {
					st.finish(nativeSuiteResult{}, returnedErr, contextErr)
				}
			}
			if err := recorder.close(returnedErr, contextErr); err == nil {
				t.Fatal("partial/cancelled run accepted")
			}
			events := nativeTimingTestEvents(t, path)
			footer := events[len(events)-1]
			if footer["status"] == "COMPLETE" || footer["successful_suites"] != float64(0) || footer["verified_batches"] != float64(0) || footer["not_started_suites"].(float64) < 1 {
				t.Fatal("failed or never-started work was invented as success", footer)
			}
			if mode == "panic" && (footer["aborted"] != true || footer["missing_suite_finishes"] != float64(1)) {
				t.Fatal("panic defer with nil errors hid an unfinished suite", footer)
			}
		})
	}
}

func TestNativeTimingExhaustiveAndBindingPhasesRemainRequired(t *testing.T) {
	for _, mode := range []string{"missing_exhaustive", "missing_binding", "phase_reordered", "extra_exhaustive"} {
		t.Run(mode, func(t *testing.T) {
			s := nativeTimingTestSuite("Check", 1)
			if mode == "missing_exhaustive" {
				s.Package, s.Activity = "go/unicode", "SimpleFold"
			}
			recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
			st := recorder.beginSuite(0, s)
			bt := recorder.beginBatch(s, 0, 0, 1)
			if mode == "phase_reordered" {
				bt.phase("replay")
			}
			for _, name := range nativeTimingPhaseNames[1:6] {
				bt.phase(name)
			}
			if mode != "missing_binding" {
				bt.bind(nativeTimingTestBinding(t, recorder), contentDigest([]byte("execution")))
			}
			if mode == "extra_exhaustive" {
				bt.phase("exhaustive")
			}
			bt.finish(nil, nil, true)
			st.finish(nativeTimingTestObservation(s, 1), nil, nil)
			if err := recorder.close(nil, nil); err == nil {
				t.Fatal("unverified phase path accepted")
			}
			events := nativeTimingTestEvents(t, path)
			if events[len(events)-1]["status"] == "COMPLETE" {
				t.Fatal("phase/binding error produced COMPLETE")
			}
		})
	}
}

func TestNativeTimingOutputCannotReplaceOrEscapeEvidence(t *testing.T) {
	root := nativeTimingTestRoot(t)
	outside := nativeTimingTestRoot(t)
	sentinel := filepath.Join(outside, "keep")
	if err := os.WriteFile(sentinel, []byte("unrelated evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(root, "linked.jsonl")); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "existing.jsonl")
	if err := os.WriteFile(existing, []byte("old observation"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{sentinel, root}, {existing, root}, {filepath.Join(root, "linked.jsonl"), root}, {filepath.Join(root, "alias", "new.jsonl"), filepath.Join(root, "alias")}, {root + "/x/../new.jsonl", root}, {filepath.Join(root, "missing", "new.jsonl"), filepath.Join(root, "missing")}} {
		if err := validateNativeTimingOutput(row[0], row[1]); err == nil {
			t.Fatal("unsafe output path accepted", row)
		}
	}
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "unrelated evidence" {
		t.Fatal("path validation touched unrelated content", err)
	}
	if raw, err := os.ReadFile(existing); err != nil || string(raw) != "old observation" {
		t.Fatal("existing output was overwritten", err)
	}
	cfg := nativeTimingTestConfig(t, root, []suite{nativeTimingTestSuite("Check", 1)})
	reserved := cfg
	reserved.Output = filepath.Join(root, "summary.json")
	if recorder, err := newNativeTimingWithSnapshot(reserved, nativeTimingTestSnapshot); err == nil || recorder != nil {
		t.Fatal("timing opened the original future semantic summary path")
	}
	if _, err := os.Lstat(reserved.Output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reserved semantic summary was created by telemetry", err)
	}
	if err := validateNativeTimingOutput(cfg.Output, cfg.EvidenceRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Output, []byte("created after early validation"), 0600); err != nil {
		t.Fatal(err)
	}
	if recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot); err == nil || recorder != nil {
		t.Fatal("constructor did not revalidate existing output")
	}
}

func TestNativeTimingProductionIdentityDetectsEqualLengthContentAndPinChanges(t *testing.T) {
	cfg := nativeTimingTestConfig(t, nativeTimingTestRoot(t), []suite{nativeTimingTestSuite("Check", 1)})
	before, err := nativeTimingIdentity(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"verifier", "go", "compiler"} {
		if before.Binaries[key].Bytes <= 0 || len(before.Binaries[key].SHA256) != 64 {
			t.Fatal("actual executable bytes were not hashed", key)
		}
	}
	path := filepath.Join(cfg.RepositoryRoot, "README.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	after, err := nativeTimingIdentity(cfg)
	if err != nil || before.Sources["README.md"].Bytes != after.Sources["README.md"].Bytes || before.Sources["README.md"].SHA256 == after.Sources["README.md"].SHA256 {
		t.Fatal("same-size source mutation was hidden", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepositoryRoot, "ports/timing.gooo"), []byte("a different frozen source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeTimingSourceIdentity(cfg); err == nil {
		t.Fatal("changed frozen catalog source accepted")
	}
}

func TestNativeTimingChangedSourceOrOutputOwnerCannotComplete(t *testing.T) {
	for _, mode := range []string{"source", "replaced_file", "replaced_root"} {
		t.Run(mode, func(t *testing.T) {
			s := nativeTimingTestSuite("Check", 1)
			recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
			st := recorder.beginSuite(0, s)
			nativeTimingTestBatch(t, recorder, s, 0, 0, 1).finish(nil, nil, true)
			st.finish(nativeTimingTestObservation(s, 1), nil, nil)
			if mode == "source" {
				if err := os.WriteFile(filepath.Join(recorder.cfg.RepositoryRoot, "README.md"), []byte("changed source"), 0644); err != nil {
					t.Fatal(err)
				}
			} else if mode == "replaced_file" {
				if err := os.Rename(path, path+"-owned"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("other owner"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(recorder.cfg.EvidenceRoot, recorder.cfg.EvidenceRoot+"-owned"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(recorder.cfg.EvidenceRoot, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("other owner"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := recorder.close(nil, nil); err == nil {
				t.Fatal("changed identity completed")
			}
			if mode != "source" {
				if raw, err := os.ReadFile(path); err != nil || string(raw) != "other owner" {
					t.Fatal("cleanup changed another owner's output", err)
				}
			}
		})
	}
}

type nativeTimingFailWriter struct{}

func (nativeTimingFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected write failure")
}

func TestNativeTimingIOFailureIsReturnedAndOwnedFileAlwaysCloses(t *testing.T) {
	s := nativeTimingTestSuite("Check", 1)
	recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
	recorder.writer = bufio.NewWriter(nativeTimingFailWriter{})
	recorder.beginSuite(0, s)
	if err := recorder.close(nil, nil); err == nil {
		t.Fatal("write error was hidden")
	}
	if _, err := recorder.file.Write([]byte("must be closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatal("owned file was not closed after an observer error", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Contains(raw, []byte(`"status":"COMPLETE"`)) {
		t.Fatal("write failure left an accepted completion", err)
	}
}

func TestNativeTimingSourceParentAliasIsRefused(t *testing.T) {
	cfg := nativeTimingTestConfig(t, nativeTimingTestRoot(t), []suite{nativeTimingTestSuite("Check", 1)})
	original := filepath.Join(cfg.RepositoryRoot, "tools")
	outside := filepath.Join(nativeTimingTestRoot(t), "tools")
	if err := os.Rename(original, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, original); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(outside, "api-policy-browser-smoke.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nativeTimingSourceIdentity(cfg); err == nil {
		t.Fatal("a parent symlink escaped the repository source boundary")
	}
	if recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot); err == nil || recorder != nil {
		t.Fatal("constructor accepted source bytes reached through an external parent alias")
	}
	after, err := os.ReadFile(filepath.Join(outside, "api-policy-browser-smoke.mjs"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("source alias rejection touched external content", err)
	}
}

func TestNativeTimingByteLimitPreventsAcceptedPartialOutput(t *testing.T) {
	s := nativeTimingTestSuite("Check", 1)
	recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
	// Inject an exhausted allowance without allocating a redundant 8 MiB fixture.
	recorder.encodedBytes = nativeTimingMaxBytes - 1
	st := recorder.beginSuite(0, s)
	nativeTimingTestBatch(t, recorder, s, 0, 0, 1).finish(nil, nil, true)
	st.finish(nativeTimingTestObservation(s, 1), nil, nil)
	if err := recorder.close(nil, nil); err == nil {
		t.Fatal("a full original observation hid an exhausted telemetry output allowance")
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > nativeTimingMaxBytes || bytes.Contains(raw, []byte(`"status":"COMPLETE"`)) {
		t.Fatal("bounded partial output was mislabeled complete", err)
	}
}

func TestNativeTimingConcurrentSuitesHaveOneOrderedEventStream(t *testing.T) {
	suites := []suite{}
	for i := 0; i < 4; i++ {
		suites = append(suites, nativeTimingTestSuite(fmt.Sprint(i), 2))
	}
	recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), suites)
	var joined sync.WaitGroup
	for index, s := range suites {
		joined.Add(1)
		go func(index int, s suite) {
			defer joined.Done()
			st := recorder.beginSuite(index, s)
			for batch := 0; batch < 2; batch++ {
				nativeTimingTestBatch(t, recorder, s, batch, batch, batch+1).finish(nil, nil, true)
			}
			st.finish(nativeTimingTestObservation(s, 2), nil, nil)
		}(index, s)
	}
	joined.Wait()
	if err := recorder.close(nil, nil); err != nil {
		t.Fatal(err)
	}
	events := nativeTimingTestEvents(t, path)
	if len(events) != 18 || events[len(events)-1]["successful_suites"] != float64(4) {
		t.Fatal("concurrent event writes were lost or interleaved")
	}
}

func TestNativeTimingDisabledHasNoObserverEffects(t *testing.T) {
	var recorder *nativeTiming
	var st *nativeSuiteTiming
	var bt *nativeBatchTiming
	if recorder.beginSuite(0, suite{}) != nil || recorder.beginBatch(suite{}, 0, 0, 1) != nil || recorder.close(nil, nil) != nil {
		t.Fatal("disabled recorder changed control flow")
	}
	recorder.abort()
	st.finish(nativeSuiteResult{}, nil, nil)
	bt.phase("execute")
	bt.bind(expectedIdentity{}, "")
	bt.finish(nil, nil, false)
}
