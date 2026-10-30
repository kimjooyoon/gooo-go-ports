package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const nativeSessionProfileGoVersion = "go1.27.3"
const nativeSessionProfileProbeLimit = 64 << 10
const nativeSessionCLIExecutionProfile = "experimental-session-276-v1"

func validateNativeSessionCompilerIdentity(raw []byte) error {
	if len(raw) == 0 || len(raw) > nativeSessionProfileProbeLimit {
		return fmt.Errorf("bounded session compiler build identity required")
	}
	value, err := decode(raw)
	if err != nil {
		return fmt.Errorf("session compiler build identity: %w", err)
	}
	build, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("session compiler build identity must be an object")
	}
	for key, expected := range map[string]string{
		"schema": "gooo/build-identity/v1", "compiler_source_sha": nativeSessionCompilerRevision,
		"vcs_revision": nativeSessionCompilerRevision, "source_status": "CLEAN_VCS", "vcs_modified": "false",
		"go_version": nativeSessionProfileGoVersion, "native_go_required": nativeSessionProfileGoVersion,
	} {
		if build[key] != expected {
			return fmt.Errorf("session compiler build identity differs: %s", key)
		}
	}
	// Module version/tag context is descriptive; immutable source is the gate.
	return nil
}

type nativeSessionCLIOwner interface {
	Close() error
	Summary() map[string]any
}

type nativeSessionCLIFactory func(context.Context, string, string, string, upstream, *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error)

type nativeSessionCLISuiteRecord struct {
	Package           string         `json:"package"`
	Activity          string         `json:"activity"`
	OwnerRoot         string         `json:"owner_root"`
	Cases             int            `json:"cases"`
	Attempted         bool           `json:"attempted"`
	Verified          bool           `json:"verified"`
	State             string         `json:"state"`
	Observation       *observation   `json:"observation,omitempty"`
	VerificationError string         `json:"verification_error,omitempty"`
	CloseError        string         `json:"close_error,omitempty"`
	EvidenceError     string         `json:"evidence_error,omitempty"`
	LifecyclePath     string         `json:"lifecycle_path,omitempty"`
	LifecycleSHA256   string         `json:"lifecycle_sha256,omitempty"`
	Lifecycle         map[string]any `json:"lifecycle,omitempty"`
}

// A runner owns finite per-suite records, not one shared session. Each callback
// closes and joins its own owner before Verify returns. The caller keeps the
// independent Go cache alive until all callbacks have joined.
type nativeSessionSuiteRunner struct {
	commands               context.Context
	compiler, goTool, root string
	lock                   upstream
	cache                  *nativeBuildCache
	newProfile             nativeSessionCLIFactory
	mu                     sync.Mutex
	active                 int
	sealed                 bool
	allowed                map[string]bool
	records                map[string]nativeSessionCLISuiteRecord
}

// Constructor validation performs no compiler, Go or session process launch.
func newNativeSessionSuiteRunner(ctx context.Context, compiler, goTool, runRoot string, lock upstream, cache *nativeBuildCache) (*nativeSessionSuiteRunner, error) {
	if !nativeSessionPlatformSupported() {
		return nil, fmt.Errorf("experimental native sessions require Linux or macOS")
	}
	if ctx == nil {
		return nil, fmt.Errorf("session runner requires a parent command context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, path := range []string{compiler, goTool, runRoot} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil, fmt.Errorf("session runner requires clean absolute compiler, Go and run-root paths")
		}
	}
	if goTool != etagGoToolPath() || runtime.Version() != nativeSessionProfileGoVersion || lock.GoVersion != nativeSessionProfileGoVersion || lock.CompilerRevision != uuidCompactCompilerRevision {
		return nil, fmt.Errorf("session runner requires the original205 oracle lock and exact running Go1.27.3 tool path")
	}
	info, err := os.Lstat(runRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("session runner requires an existing caller-owned run root")
	}
	for _, path := range []string{compiler, goTool} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("session runner executable is not a regular file: %s", path)
		}
	}
	expected, err := nativeExpectedVerificationContract()
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, s := range expected.Suites {
		allowed[s.Package+":"+s.Activity] = true
	}
	return &nativeSessionSuiteRunner{commands: ctx, compiler: compiler, goTool: goTool, root: runRoot, lock: lock, cache: cache,
		newProfile: newNativeSessionCLIProfile, allowed: allowed, records: map[string]nativeSessionCLISuiteRecord{}}, nil
}

func (r *nativeSessionSuiteRunner) Verify(ctx context.Context, s suite, workspace suiteWorkspace) (result nativeSuiteResult, returnedErr error) {
	if ctx == nil {
		return result, fmt.Errorf("session suite requires a callback context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	key := s.Package + ":" + s.Activity
	ownerRoot := suiteEvidenceDirectory(r.root, s)
	record := nativeSessionCLISuiteRecord{Package: s.Package, Activity: s.Activity, OwnerRoot: ownerRoot, Cases: len(s.Cases), Attempted: true, State: "STARTED"}
	r.mu.Lock()
	if r.sealed {
		r.mu.Unlock()
		return result, fmt.Errorf("session runner summary is already sealed")
	}
	if !r.allowed[key] || len(s.Cases) == 0 {
		r.mu.Unlock()
		return result, fmt.Errorf("session runner requires a nonempty original catalog suite")
	}
	if _, exists := r.records[ownerRoot]; exists {
		r.mu.Unlock()
		return result, fmt.Errorf("session suite owner root was already attempted")
	}
	r.records[ownerRoot] = record
	r.active++
	r.mu.Unlock()
	completed := false
	defer func() {
		if !completed {
			record.State = "INTERRUPTED_CALLBACK_INCOMPLETE"
		} else if returnedErr != nil {
			record.State = "FAILED_OBSERVATION_PRESERVED"
		} else {
			record.State, record.Verified = "SUITE_VERIFIED_OWNER_CLOSED", true
		}
		r.mu.Lock()
		r.records[ownerRoot] = record
		r.active--
		r.mu.Unlock()
	}()
	result, returnedErr = r.verifyOwned(ctx, s, workspace, &record)
	completed = true
	return result, returnedErr
}

func (r *nativeSessionSuiteRunner) verifyOwned(ctx context.Context, s suite, workspace suiteWorkspace, record *nativeSessionCLISuiteRecord) (result nativeSuiteResult, returnedErr error) {
	if err := ctx.Err(); err != nil {
		record.VerificationError = nativeSessionCLIDiagnostic(err)
		return result, err
	}
	if err := os.Mkdir(record.OwnerRoot, 0755); err != nil {
		record.VerificationError = nativeSessionCLIDiagnostic(err)
		return result, err
	}
	verifier, owner, err := r.newProfile(r.commands, r.compiler, r.goTool, record.OwnerRoot, r.lock, r.cache)
	if err != nil {
		record.VerificationError = nativeSessionCLIDiagnostic(err)
		return result, err
	}
	if owner == nil {
		return result, fmt.Errorf("session profile returned no lifecycle owner")
	}
	defer func() {
		closeErr := owner.Close()
		summary := owner.Summary()
		if closeErr == nil {
			closeErr = nativeSessionCLICleanClose(summary)
		}
		record.CloseError = nativeSessionCLIDiagnostic(closeErr)
		path := filepath.Join(record.OwnerRoot, "session-owner.json")
		raw, marshalErr := json.MarshalIndent(summary, "", "  ")
		var evidenceErr error
		if marshalErr == nil {
			raw = append(raw, '\n')
			evidenceErr = nativeSessionCLIWriteNew(path, raw)
			if evidenceErr == nil {
				record.LifecyclePath, record.LifecycleSHA256 = path, contentDigest(raw)
			}
		} else {
			evidenceErr = marshalErr
		}
		record.Lifecycle = nativeSessionCLILifecycle(summary)
		record.EvidenceError = nativeSessionCLIDiagnostic(evidenceErr)
		returnedErr = errors.Join(returnedErr, closeErr, evidenceErr)
	}()
	// The transport owns only this suite subtree. The verifier retains the
	// original run root for both suite/NNN batches and exhaustive helper paths.
	verifier.root = r.root
	result, returnedErr = verifier.verify(ctx, s, workspace)
	record.VerificationError = nativeSessionCLIDiagnostic(returnedErr)
	if result.Observation.Package != "" {
		observed := result.Observation
		record.Observation = &observed
	}
	return result, returnedErr
}

func (r *nativeSessionSuiteRunner) WriteSummary(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) != r.root {
		return fmt.Errorf("session summary must be a new direct child of the run root")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != 0 {
		return fmt.Errorf("session summary requires every suite owner callback to join")
	}
	if r.sealed {
		return fmt.Errorf("session runner summary is already sealed")
	}
	keys := make([]string, 0, len(r.records))
	for key := range r.records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]nativeSessionCLISuiteRecord, 0, len(keys))
	verified := 0
	for _, key := range keys {
		record := r.records[key]
		records = append(records, record)
		if record.Verified {
			verified++
		}
	}
	document := map[string]any{"schema": "gooo/native-session-cli-summary/v1", "execution_profile": nativeSessionCLIExecutionProfile,
		"compiler_revision": nativeSessionCompilerRevision, "oracle_lock_compiler_revision": r.lock.CompilerRevision,
		"go_version": r.lock.GoVersion, "attempted_suites": len(records), "verified_suites": verified, "failed_suites": len(records) - verified,
		"owners": records, "full82_accepted": false, "producer_authenticated": false,
		"per_request_wait_resources": map[string]any{"status": "NOT_OBSERVED", "user_cpu_ns": nil, "system_cpu_ns": nil, "peak_rss_bytes": nil},
		"scope":                      "Finite original-suite observations and actual per-suite Close/Wait diagnostics; whole-session waited CPU is not per-request Rusage. Not full82, shard, CI or default-profile acceptance."}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if err := nativeSessionCLIWriteNew(path, append(raw, '\n')); err != nil {
		return err
	}
	r.sealed = true
	return nil
}

func nativeSessionCLICleanClose(summary map[string]any) error {
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
		fields["closed"] != true || fields["waited"] != true || fields["streams_joined"] != true || fields["process_group_remaining"] != false ||
		fields["close_request_sent"] != true || fields["close_response_received"] != true || fields["exit_code"] != json.Number("0") {
		return fmt.Errorf("session owner close, wait, streams or process-group observation is incomplete")
	}
	entries, ok := fields["owned_entries_after_close"].([]any)
	if !ok || len(entries) != 0 {
		return fmt.Errorf("session owner retained native temporary entries")
	}
	return nil
}

func nativeSessionCLILifecycle(summary map[string]any) map[string]any {
	fields := map[string]any{}
	for _, key := range []string{"schema", "pid", "error", "closed", "waited", "streams_joined", "process_group_remaining", "exit_code", "owned_entries_after_close", "close_request_sent", "close_response_received", "waited_process_resources_available", "waited_process_user_ns", "waited_process_system_ns", "resource_scope"} {
		fields[key] = summary[key]
	}
	if failure := summary["error"]; failure != nil {
		if text, ok := failure.(string); ok {
			fields["error"] = nativeSessionCLIDiagnostic(errors.New(text))
		} else {
			fields["error"] = "invalid non-string session lifecycle error"
		}
	}
	return fields
}

func nativeSessionCLIDiagnostic(err error) string {
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
	return text
}

func nativeSessionCLIWriteNew(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	return errors.Join(writeErr, file.Close())
}
