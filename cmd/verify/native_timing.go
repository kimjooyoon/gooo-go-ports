package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

const nativeTimingSchema = "gooo/native-verification-timing/v1"
const nativeTimingMaxBytes = 8 << 20

type nativeTimingConfig struct {
	Output, EvidenceRoot, RepositoryRoot, Compiler string
	Workers                                        int
	Suites                                         []suite
	Lock                                           upstream
	Catalog                                        frozenVerificationCatalog
	CompilerIdentity                               []byte
}

type nativeTimingFile struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type nativeTimingSnapshot struct {
	Sources  map[string]nativeTimingFile `json:"source_files"`
	Binaries map[string]nativeTimingFile `json:"binaries"`
}

// This is the reviewed 23-file CI roster, followed by the two runtime additions.
// Hash current bytes; never import a private preparation file or an old receipt.
var nativeTimingSourceNames = []string{
	".github/workflows/ci.yml", "tools/check-ci-scope-workflow_test.py", "tools/classify-ci-scope.py",
	"cmd/build-demo/main.go", "cmd/build-demo/api_policy.go", "cmd/build-demo/api_policy_test.go",
	"tools/api-policy-browser-smoke.mjs", "tools/api-policy-browser-bridge.go.txt", "tools/api-policy-app.test.cjs",
	"site/api-policy.html", "site/api-policy.js", "site/api-policy-worker.js", "site/api-policy.css",
	"cmd/verify/native_correctness_cache_test.go", "cmd/verify/simple_fold_exhaustive_test.go",
	"cmd/verify/ycbcr_exhaustive_test.go", "cmd/verify/case_exhaustive_test.go", "README.md",
	"cmd/verify/main.go", "cmd/verify/suite_workers.go", "cmd/verify/suite_workers_test.go",
	"cmd/verify/native_build_cache.go", "cmd/verify/native_build_cache_test.go",
	"cmd/verify/native_suite.go", "cmd/verify/native_timing.go",
	"cmd/verify/literal_cut_corpus.go", "cmd/verify/literal_cut_contracts.go",
	"cmd/verify/contracts.go", "cmd/verify/whitespace_contracts.go", "cmd/verify/inspect.go",
	"cmd/verify/string_byte_batches.go", "cmd/verify/workspace_catalog.go",
	"cmd/verify/literal_cut_corpus_test.go", "cmd/verify/literal_cut_integration_test.go",
	"cmd/verify/testdata/literal-cut-original62.json", "cmd/verify/contracts_test.go", "cmd/verify/workspace_catalog_test.go",
	"tools/check-literal-text-inputs.py", "tools/check-literal-text-inputs_test.py",
	"literal-cut-report.workspace.json", "examples/literal-cut-report/inputs.json",
	"examples/literal-cut-report/manual-contract.json", "examples/literal-cut-report/README.md", "CONTRIBUTING.md",
}

var nativeTimingPhaseNames = [...]string{
	"prepare", "execute", "inspect_execution", "replay", "inspect_replay", "receipt_binding", "exhaustive",
}

type nativeTiming struct {
	mu              sync.Mutex
	cfg             nativeTimingConfig
	started         time.Time
	before          nativeTimingSnapshot
	snapshot        func(nativeTimingConfig) (nativeTimingSnapshot, error)
	file            *os.File
	root            *os.Root
	rootInfo        os.FileInfo
	fileInfo        os.FileInfo
	expectedSources map[string]sourceIdentity
	writer          *bufio.Writer
	err             error
	errorCount      int
	writeFailed     bool
	eventCount      int
	encodedBytes    int
	closed          bool
	aborted         bool
	suites          []*nativeSuiteTiming
	byEntry         map[string]*nativeSuiteTiming
}

type nativeSuiteTiming struct {
	owner                 *nativeTiming
	index, cases          int
	entry, pkg, activity  string
	started, finished     time.Time
	begun, done, verified bool
	batches               []*nativeBatchTiming
	observation           observation
	status                string
	sourceSet             map[string]sourceIdentity
	sourceSetSHA256       string
}

type nativeTimingPhase struct {
	Name       string `json:"name"`
	Observed   bool   `json:"observed"`
	DurationNS int64  `json:"duration_ns"`
}

type nativeBatchTiming struct {
	owner                 *nativeTiming
	suite                 *nativeSuiteTiming
	index, start, end     int
	started, phaseStarted time.Time
	phaseIndex            int
	phases                [7]nativeTimingPhase
	done, verified, bound bool
	identity              expectedIdentity
	receipt               string
	status                string
}

// Validation opens nothing. Require a direct child and reject every existing
// symlink component of the evidence path, including the evidence directory.
func validateNativeTimingOutput(output, evidenceRoot string) error {
	if output == "" || evidenceRoot == "" || filepath.Clean(output) != output || filepath.Clean(evidenceRoot) != evidenceRoot {
		return fmt.Errorf("native timing requires clean nonempty paths")
	}
	root, err := filepath.Abs(evidenceRoot)
	if err != nil {
		return err
	}
	out, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if filepath.Dir(out) != root || out == root {
		return fmt.Errorf("native timing output must be a direct child of the evidence directory")
	}
	if filepath.Ext(out) != ".jsonl" {
		return fmt.Errorf("native timing output requires a .jsonl name distinct from original semantic evidence")
	}
	for path := root; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("native timing evidence path: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("native timing evidence path is not a real directory")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	if _, err := os.Lstat(out); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("native timing output must not exist")
	}
	return nil
}

func newNativeTiming(cfg nativeTimingConfig) (*nativeTiming, error) {
	return newNativeTimingWithSnapshot(cfg, nativeTimingIdentity)
}

// Injection is internal to this package's recorder tests, not a runtime option.
func newNativeTimingWithSnapshot(cfg nativeTimingConfig, snapshot func(nativeTimingConfig) (nativeTimingSnapshot, error)) (*nativeTiming, error) {
	if err := validateNativeTimingOutput(cfg.Output, cfg.EvidenceRoot); err != nil {
		return nil, err
	}
	for _, path := range []*string{&cfg.Output, &cfg.EvidenceRoot, &cfg.RepositoryRoot, &cfg.Compiler} {
		absolute, err := filepath.Abs(*path)
		if err != nil {
			return nil, err
		}
		*path = absolute
	}
	if err := validateNativeSuiteWorkers(cfg.Workers); err != nil {
		return nil, err
	}
	if snapshot == nil || len(cfg.Suites) == 0 {
		return nil, fmt.Errorf("native timing requires an identity observer and nonempty suite roster")
	}
	if err := nativeTimingCompilerIdentity(cfg); err != nil {
		return nil, err
	}
	t := &nativeTiming{cfg: cfg, started: time.Now(), snapshot: snapshot, byEntry: map[string]*nativeSuiteTiming{}}
	for index, s := range cfg.Suites {
		key := s.Package + ":" + s.Activity
		if s.Package == "" || s.Activity == "" || len(s.Cases) == 0 || t.byEntry[key] != nil {
			return nil, fmt.Errorf("native timing requires unique nonempty suites")
		}
		st := &nativeSuiteTiming{owner: t, index: index, cases: len(s.Cases), entry: key, pkg: s.Package, activity: s.Activity}
		t.suites = append(t.suites, st)
		t.byEntry[key] = st
	}
	// Preserve the frozen inputs against caller mutation without serializing the
	// case corpus again. Actual batch boundaries come only from the verifier.
	t.cfg.Suites = nil
	t.cfg.CompilerIdentity = append([]byte(nil), cfg.CompilerIdentity...)
	lockBytes, err := json.Marshal(cfg.Lock)
	if err != nil {
		return nil, err
	}
	t.cfg.Lock = upstream{}
	if err := json.Unmarshal(lockBytes, &t.cfg.Lock); err != nil {
		return nil, err
	}
	t.cfg.Catalog.Raw = append([]byte(nil), cfg.Catalog.Raw...)
	t.cfg.Catalog.Sources, t.cfg.Catalog.Hashes = map[string][]byte{}, map[string]string{}
	for name, raw := range cfg.Catalog.Sources {
		t.cfg.Catalog.Sources[name] = append([]byte(nil), raw...)
	}
	for name, digest := range cfg.Catalog.Hashes {
		t.cfg.Catalog.Hashes[name] = digest
	}
	t.cfg.Catalog.Catalog = verificationCatalog{}
	if err := json.Unmarshal(t.cfg.Catalog.Raw, &t.cfg.Catalog.Catalog); err != nil {
		return nil, fmt.Errorf("native timing frozen catalog: %w", err)
	}
	t.expectedSources, err = snapshotSourceIdentities(t.cfg.Catalog.Catalog.Packages, t.cfg.Catalog.Sources)
	if err != nil {
		return nil, err
	}
	t.before, err = snapshot(t.cfg)
	if err != nil {
		return nil, err
	}
	if err := validateNativeTimingOutput(cfg.Output, cfg.EvidenceRoot); err != nil {
		return nil, err
	}
	rootPath, _ := filepath.Abs(cfg.EvidenceRoot)
	rootInfo, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}
	t.rootInfo = rootInfo
	t.root, err = os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	openedRoot, err := t.root.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRoot) {
		_ = t.root.Close()
		return nil, fmt.Errorf("native timing evidence directory changed while opening")
	}
	t.file, err = t.root.OpenFile(filepath.Base(cfg.Output), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		_ = t.root.Close()
		return nil, err
	}
	t.fileInfo, err = t.file.Stat()
	if err != nil {
		_ = t.file.Close()
		_ = t.root.Close()
		return nil, err
	}
	t.writer = bufio.NewWriterSize(t.file, 16384)
	planned := make([]map[string]any, 0, len(t.suites))
	total := 0
	for _, st := range t.suites {
		planned = append(planned, map[string]any{"index": st.index, "package": st.pkg, "activity": st.activity, "cases": st.cases})
		total += st.cases
	}
	t.emitLocked(map[string]any{
		"kind": "header", "started_at_utc": t.started.UTC().Format(time.RFC3339Nano),
		"configured_workers": cfg.Workers, "reported_num_cpu": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0),
		"memory_limit_bytes": nil, "cpu_quota": nil, "go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"max_output_bytes": nativeTimingMaxBytes,
		"identity":         t.before, "upstream_pins": t.cfg.Lock, "compiler_identity": nativeTimingBytes(t.cfg.CompilerIdentity),
		"compiler_revision": cfg.Lock.CompilerRevision, "compiler_version": cfg.Lock.CompilerVersion,
		"catalog": nativeTimingBytes(t.cfg.Catalog.Raw), "catalog_source_sha256": t.cfg.Catalog.Hashes,
		"planned_suites": planned, "expected_suites": len(t.suites), "expected_cases": total, "expected_boundaries_recomputed": false,
		"boundary_scope":     "Actual contiguous batch ranges are compared with the original successful suite observation. Exact original boundaries rely on unchanged nativeBatchEnd/loop source and the original full-summary gate.",
		"source_set_scope":   "Each suite_end stores its checked source set once. Batch events bind that exact map by SHA-256; source inputs and the complete frozen catalog are hashed in the header and rechecked at close.",
		"timing_scope":       "Suite and batch elapsed intervals include recorder lock/event overhead; enqueue, slot wait, dispatch, CPU time and resource contention are not measured. Batch selection happens before beginBatch and is unmeasured. Run duration includes identity observations and ends before footer finalization. UTC is for correlation; durations use Go monotonic time.",
		"finalization_scope": "A footer is diagnostic evidence only; successful close return and the original native verification/summary gates are also required. No effect authorization or performance causality is asserted.",
	})
	if t.err != nil {
		_ = t.file.Close()
		_ = t.root.Close()
		return nil, t.err
	}
	return t, nil
}

func nativeTimingCompilerIdentity(cfg nativeTimingConfig) error {
	var identity struct {
		Version, GoVersion, NativeGo string
		Revision, Modified           string
	}
	var raw struct {
		Version  string `json:"version"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
		Revision string `json:"compiler_source_sha"`
		Modified string `json:"vcs_modified"`
	}
	if err := json.Unmarshal(cfg.CompilerIdentity, &raw); err != nil {
		return fmt.Errorf("native timing compiler identity: %w", err)
	}
	identity.Version, identity.GoVersion, identity.NativeGo, identity.Revision, identity.Modified = raw.Version, raw.Go, raw.Native, raw.Revision, raw.Modified
	if cfg.Lock.GoVersion != runtime.Version() || identity.Version != cfg.Lock.CompilerVersion || identity.Revision != cfg.Lock.CompilerRevision || identity.Modified != "false" || identity.GoVersion != cfg.Lock.GoVersion || identity.NativeGo != cfg.Lock.GoVersion {
		return fmt.Errorf("native timing compiler identity differs from checked pins")
	}
	return nil
}

func nativeTimingBytes(raw []byte) nativeTimingFile {
	sum := sha256.Sum256(raw)
	return nativeTimingFile{Bytes: int64(len(raw)), SHA256: hex.EncodeToString(sum[:])}
}

func nativeTimingHashFile(path string) (nativeTimingFile, error) {
	before, err := os.Lstat(path)
	if err != nil || before == nil || !before.Mode().IsRegular() {
		return nativeTimingFile{}, fmt.Errorf("native timing identity requires a regular file: %s", filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nativeTimingFile{}, err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		_ = f.Close()
		return nativeTimingFile{}, fmt.Errorf("native timing identity file changed while opening")
	}
	h := sha256.New()
	n, readErr := io.Copy(h, f)
	after, statErr := f.Stat()
	closeErr := f.Close()
	if readErr != nil || statErr != nil || closeErr != nil || n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nativeTimingFile{}, fmt.Errorf("native timing identity file changed or could not be read")
	}
	return nativeTimingFile{Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// Reject in-repository aliases as well as final-component symlinks. This does
// not claim an atomic filesystem snapshot; the complete hashes are checked again.
func nativeTimingSourcePath(root, name string) (string, error) {
	if !filepath.IsLocal(name) || filepath.Clean(name) != name {
		return "", fmt.Errorf("native timing source path is not local")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("native timing repository root is not a real directory")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		return "", fmt.Errorf("native timing repository path contains an alias")
	}
	path := root
	parts := strings.Split(name, string(filepath.Separator))
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) {
			return "", fmt.Errorf("native timing source contains an alias or missing component")
		}
	}
	return path, nil
}

func nativeTimingSourceIdentity(cfg nativeTimingConfig) (nativeTimingSnapshot, error) {
	snapshot := nativeTimingSnapshot{Sources: map[string]nativeTimingFile{}, Binaries: map[string]nativeTimingFile{}}
	names := append([]string(nil), nativeTimingSourceNames...)
	names = append(names, "upstream.json", "verification-catalog.json")
	for name := range cfg.Catalog.Hashes {
		names = append(names, name)
	}
	for _, name := range names {
		path, err := nativeTimingSourcePath(cfg.RepositoryRoot, name)
		if err != nil {
			return snapshot, err
		}
		observed, err := nativeTimingHashFile(path)
		if err != nil {
			return snapshot, err
		}
		snapshot.Sources[name] = observed
	}
	upstreamBytes, err := os.ReadFile(filepath.Join(cfg.RepositoryRoot, "upstream.json"))
	if err != nil {
		return snapshot, err
	}
	var lock upstream
	if err := json.Unmarshal(upstreamBytes, &lock); err != nil || !reflect.DeepEqual(lock, cfg.Lock) {
		return snapshot, fmt.Errorf("native timing upstream file differs from checked pins")
	}
	catalog, err := os.ReadFile(filepath.Join(cfg.RepositoryRoot, "verification-catalog.json"))
	if err != nil || !reflect.DeepEqual(catalog, cfg.Catalog.Raw) {
		return snapshot, fmt.Errorf("native timing catalog differs from frozen bytes")
	}
	if len(cfg.Catalog.Hashes) == 0 || len(cfg.Catalog.Hashes) != len(cfg.Catalog.Sources) {
		return snapshot, fmt.Errorf("native timing requires a complete frozen source map")
	}
	for name, digest := range cfg.Catalog.Hashes {
		frozen, ok := cfg.Catalog.Sources[name]
		if !ok || nativeTimingBytes(frozen).SHA256 != digest || snapshot.Sources[name] != nativeTimingBytes(frozen) {
			return snapshot, fmt.Errorf("native timing catalog source differs from frozen bytes")
		}
	}
	return snapshot, nil
}

func nativeTimingIdentity(cfg nativeTimingConfig) (nativeTimingSnapshot, error) {
	snapshot, err := nativeTimingSourceIdentity(cfg)
	if err != nil {
		return snapshot, err
	}
	self, err := os.Executable()
	if err != nil {
		return snapshot, err
	}
	for key, path := range map[string]string{"verifier": self, "go": filepath.Join(runtime.GOROOT(), "bin", "go"), "compiler": cfg.Compiler} {
		observed, err := nativeTimingHashFile(path)
		if err != nil {
			return snapshot, err
		}
		snapshot.Binaries[key] = observed
	}
	return snapshot, nil
}

func (t *nativeTiming) failLocked(err error) {
	if err != nil {
		t.errorCount++
		if t.err == nil {
			t.err = err
		}
	}
}

func (t *nativeTiming) emitLocked(event map[string]any) {
	if t.writeFailed {
		return
	}
	event["schema"] = nativeTimingSchema
	event["seq"] = t.eventCount
	raw, err := json.Marshal(event)
	if err != nil {
		t.writeFailed = true
		t.failLocked(err)
		return
	}
	if len(raw)+1 > nativeTimingMaxBytes-t.encodedBytes {
		t.writeFailed = true
		t.failLocked(fmt.Errorf("native timing output exceeds the 8 MiB bound"))
		return
	}
	raw = append(raw, '\n')
	if _, err := t.writer.Write(raw); err != nil {
		t.writeFailed = true
		t.failLocked(err)
		return
	}
	if err := t.writer.Flush(); err != nil {
		t.writeFailed = true
		t.failLocked(err)
		return
	}
	t.eventCount++
	t.encodedBytes += len(raw)
}

func (t *nativeTiming) beginSuite(index int, s suite) *nativeSuiteTiming {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || index < 0 || index >= len(t.suites) {
		t.failLocked(fmt.Errorf("native timing suite start is outside the original roster"))
		return nil
	}
	st := t.suites[index]
	if st.begun || st.entry != s.Package+":"+s.Activity || st.cases != len(s.Cases) {
		t.failLocked(fmt.Errorf("native timing suite start duplicates or differs from original index"))
		return nil
	}
	st.begun, st.started = true, time.Now()
	t.emitLocked(map[string]any{"kind": "suite_begin", "suite_index": index, "package": st.pkg, "activity": st.activity, "started_at_utc": st.started.UTC().Format(time.RFC3339Nano), "run_offset_ns": st.started.Sub(t.started).Nanoseconds()})
	return st
}

func nativeTimingEndStatus(returnedErr, contextErr error, verified bool) string {
	if contextErr != nil {
		if errors.Is(contextErr, context.DeadlineExceeded) {
			return "DEADLINE"
		}
		return "CANCELED"
	}
	if returnedErr != nil {
		return "FAILED"
	}
	if !verified {
		return "INCOMPLETE"
	}
	return "VERIFIED"
}

func (st *nativeSuiteTiming) finish(result nativeSuiteResult, returnedErr, contextErr error) {
	if st == nil {
		return
	}
	finished := time.Now()
	t := st.owner
	t.mu.Lock()
	defer t.mu.Unlock()
	if st.done {
		t.failLocked(fmt.Errorf("native timing suite finished more than once"))
		return
	}
	st.finished, st.done, st.observation = finished, true, result.Observation
	valid := st.begun && len(st.batches) > 0
	cases := 0
	for _, bt := range st.batches {
		valid = valid && bt.done && bt.verified
		cases += bt.end - bt.start
	}
	r := result.Observation
	valid = valid && cases == st.cases && r.Package == st.pkg && r.Activity == st.activity && r.Cases == st.cases && r.Passed == st.cases && r.Batches == len(st.batches) && r.ReplayEqual
	st.verified = valid && returnedErr == nil && contextErr == nil
	if !valid && returnedErr == nil && contextErr == nil {
		t.failLocked(fmt.Errorf("native timing original suite observation or batch coverage is incomplete"))
	}
	st.status = nativeTimingEndStatus(returnedErr, contextErr, st.verified)
	t.emitLocked(map[string]any{"kind": "suite_end", "suite_index": st.index, "package": st.pkg, "activity": st.activity,
		"started_at_utc": st.started.UTC().Format(time.RFC3339Nano), "finished_at_utc": st.finished.UTC().Format(time.RFC3339Nano), "duration_ns": st.finished.Sub(st.started).Nanoseconds(),
		"status": st.status, "returned_error": returnedErr != nil, "context_error": contextErr != nil,
		"observed_batches": len(st.batches), "covered_cases": cases, "original_observation": r,
		"source_set": st.sourceSet, "source_set_sha256": st.sourceSetSHA256})
}

func (t *nativeTiming) beginBatch(s suite, batchIndex, start, end int) *nativeBatchTiming {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.byEntry[s.Package+":"+s.Activity]
	if t.closed || st == nil || !st.begun || st.done || st.cases != len(s.Cases) || batchIndex != len(st.batches) || start < 0 || end <= start || end > st.cases {
		t.failLocked(fmt.Errorf("native timing batch is duplicate, out of order or outside its suite"))
		return nil
	}
	previousEnd := 0
	if len(st.batches) > 0 {
		previous := st.batches[len(st.batches)-1]
		previousEnd = previous.end
		if !previous.done || !previous.verified {
			t.failLocked(fmt.Errorf("native timing batch follows an unfinished or unverified batch"))
			return nil
		}
	}
	if start != previousEnd {
		t.failLocked(fmt.Errorf("native timing batch coverage has a gap or overlap"))
		return nil
	}
	now := time.Now()
	bt := &nativeBatchTiming{owner: t, suite: st, index: batchIndex, start: start, end: end, started: now, phaseStarted: now}
	for i, name := range nativeTimingPhaseNames {
		bt.phases[i].Name = name
	}
	bt.phases[0].Observed = true
	st.batches = append(st.batches, bt)
	return bt
}

func (bt *nativeBatchTiming) phase(name string) {
	if bt == nil {
		return
	}
	now := time.Now()
	t := bt.owner
	t.mu.Lock()
	defer t.mu.Unlock()
	if bt.done {
		t.failLocked(fmt.Errorf("native timing phase follows a terminal batch"))
		return
	}
	if name == "prepare" && bt.phaseIndex == 0 {
		return
	}
	next := bt.phaseIndex + 1
	if next >= len(bt.phases) || bt.phases[next].Name != name {
		t.failLocked(fmt.Errorf("native timing phase order differs from the verifier"))
		return
	}
	bt.phases[bt.phaseIndex].DurationNS += now.Sub(bt.phaseStarted).Nanoseconds()
	bt.phaseIndex, bt.phaseStarted, bt.phases[next].Observed = next, now, true
}

func nativeTimingValidDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
		return false
	}
	raw, err := hex.DecodeString(value[7:])
	return err == nil && len(raw) == sha256.Size && strings.ToLower(value) == value
}

func (bt *nativeBatchTiming) bind(identity expectedIdentity, receiptSHA string) {
	if bt == nil {
		return
	}
	t := bt.owner
	t.mu.Lock()
	defer t.mu.Unlock()
	if bt.done || bt.bound || identity.CompilerRevision != t.cfg.Lock.CompilerRevision || identity.GoVersion != t.cfg.Lock.GoVersion || !nativeTimingValidDigest(identity.ManifestDigest) || !nativeTimingValidDigest(identity.CasesDigest) || !nativeTimingValidDigest(receiptSHA) || len(identity.Sources) == 0 {
		t.failLocked(fmt.Errorf("native timing batch has incomplete or repeated checked receipt bindings"))
		return
	}
	for name, source := range identity.Sources {
		if source.Package == "" || !nativeTimingValidDigest(source.Digest) || t.expectedSources[name] != source {
			t.failLocked(fmt.Errorf("native timing batch source binding is incomplete"))
			return
		}
	}
	if bt.suite.sourceSet == nil {
		bt.suite.sourceSet = map[string]sourceIdentity{}
		for name, source := range identity.Sources {
			bt.suite.sourceSet[name] = source
		}
		raw, err := json.Marshal(bt.suite.sourceSet)
		if err != nil {
			t.failLocked(err)
			return
		}
		bt.suite.sourceSetSHA256 = contentDigest(raw)
	} else if !reflect.DeepEqual(bt.suite.sourceSet, identity.Sources) {
		t.failLocked(fmt.Errorf("native timing suite source set changed between batches"))
		return
	}
	bt.identity = identity
	bt.identity.Sources = nil
	bt.bound, bt.receipt = true, receiptSHA
}

func (bt *nativeBatchTiming) finish(returnedErr, contextErr error, verified bool) {
	if bt == nil {
		return
	}
	now := time.Now()
	t := bt.owner
	t.mu.Lock()
	defer t.mu.Unlock()
	if bt.done {
		t.failLocked(fmt.Errorf("native timing batch finished more than once"))
		return
	}
	bt.phases[bt.phaseIndex].DurationNS += now.Sub(bt.phaseStarted).Nanoseconds()
	bt.done = true
	exhaustive := bt.index == 0 && ((bt.suite.pkg == "go/unicode" && (bt.suite.activity == "SimpleFold" || bt.suite.activity == "To" || bt.suite.activity == "ToUpper" || bt.suite.activity == "ToLower" || bt.suite.activity == "ToTitle")) || (bt.suite.pkg == "go/image/color" && (bt.suite.activity == "RGBToYCbCr" || bt.suite.activity == "YCbCrToRGB")))
	last := 5
	if exhaustive {
		last = 6
	}
	valid := bt.bound && bt.phaseIndex == last
	bt.verified = verified && valid && returnedErr == nil && contextErr == nil
	if verified && !valid && returnedErr == nil && contextErr == nil {
		t.failLocked(fmt.Errorf("native timing verified batch lacks all checked phases or bindings"))
	}
	bt.status = nativeTimingEndStatus(returnedErr, contextErr, bt.verified)
	t.emitLocked(map[string]any{"kind": "batch_end", "suite_index": bt.suite.index, "package": bt.suite.pkg, "activity": bt.suite.activity,
		"batch_index": bt.index, "start": bt.start, "end": bt.end, "cases": bt.end - bt.start,
		"started_at_utc": bt.started.UTC().Format(time.RFC3339Nano), "finished_at_utc": now.UTC().Format(time.RFC3339Nano), "duration_ns": now.Sub(bt.started).Nanoseconds(),
		"status": bt.status, "verified": bt.verified, "returned_error": returnedErr != nil, "context_error": contextErr != nil,
		"phases":            bt.phases,
		"compiler_revision": bt.identity.CompilerRevision, "go_version": bt.identity.GoVersion,
		"manifest_digest": bt.identity.ManifestDigest, "cases_digest": bt.identity.CasesDigest,
		"source_set_sha256": bt.suite.sourceSetSHA256, "original_receipt_sha256": bt.receipt})
}

func (t *nativeTiming) abort() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.aborted = true
	}
}

func (t *nativeTiming) close(returnedErr, contextErr error) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return t.err
	}
	t.closed = true
	currentRoot, rootErr := os.Lstat(t.cfg.EvidenceRoot)
	currentFile, fileErr := os.Lstat(t.cfg.Output)
	if rootErr != nil || fileErr != nil || !os.SameFile(t.rootInfo, currentRoot) || !os.SameFile(t.fileInfo, currentFile) {
		t.failLocked(fmt.Errorf("native timing output ownership or directory changed"))
	}
	after, snapshotErr := t.snapshot(t.cfg)
	unchanged := snapshotErr == nil && reflect.DeepEqual(t.before, after)
	if !unchanged {
		t.failLocked(fmt.Errorf("native timing source or executable identity changed during observation"))
	}
	complete := !t.aborted && returnedErr == nil && contextErr == nil && unchanged
	started, finished, verified, batches, verifiedBatches, cases := 0, 0, 0, 0, 0, 0
	failedSuites, canceledSuites, failedBatches, incompleteBatches := 0, 0, 0, 0
	notStarted := []int{}
	for _, st := range t.suites {
		if st.begun {
			started++
		} else {
			notStarted = append(notStarted, st.index)
		}
		if st.done {
			finished++
		}
		if st.verified {
			verified++
		}
		if st.status == "FAILED" {
			failedSuites++
		}
		if st.status == "CANCELED" || st.status == "DEADLINE" {
			canceledSuites++
		}
		complete = complete && st.begun && st.done && st.verified
		for _, bt := range st.batches {
			batches++
			if bt.verified {
				verifiedBatches++
				cases += bt.end - bt.start
			} else if bt.status == "FAILED" {
				failedBatches++
			} else {
				incompleteBatches++
			}
		}
	}
	complete = complete && t.err == nil
	status := nativeTimingEndStatus(returnedErr, contextErr, complete)
	if complete {
		status = "COMPLETE"
	}
	if t.err != nil {
		status = "TELEMETRY_ERROR"
	}
	// A known write error leaves a partial log; validation errors can still have
	// an explicit unsuccessful footer. Successful close return remains mandatory.
	if !t.writeFailed {
		t.emitLocked(map[string]any{"kind": "run_end", "status": status, "complete": complete, "aborted": t.aborted,
			"finished_at_utc": time.Now().UTC().Format(time.RFC3339Nano), "duration_ns": time.Since(t.started).Nanoseconds(),
			"expected_suites": len(t.suites), "successful_suites": verified, "failed_suites": failedSuites, "canceled_suites": canceledSuites,
			"not_started_suites": len(notStarted), "missing_suite_finishes": started - finished, "not_started_suite_indices": notStarted,
			"observed_batches": batches, "verified_batches": verifiedBatches, "verified_cases": cases,
			"failed_batches": failedBatches, "incomplete_batches": incompleteBatches, "logger_errors": t.errorCount,
			"source_bindings_unchanged": unchanged, "identity_after": after, "event_count": t.eventCount + 1, "successful_close_return_required": true})
	}
	t.failLocked(t.writer.Flush())
	t.failLocked(t.file.Sync())
	t.failLocked(t.file.Close())
	t.failLocked(t.root.Close())
	if !complete && t.err == nil {
		t.err = fmt.Errorf("native timing observation is incomplete")
	}
	return t.err
}
