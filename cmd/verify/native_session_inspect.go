package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

const nativeSessionCompilerRevision = "276734167edc7f5fef18cbb79004ea9592553317"

// These wire structs preserve the field order and omitempty rules of accepted
// compiler276's bodyexecution.ProcessObservation and ProcessTiming. That exact
// JSON encoding participates in artifact.source_build_sha256. They are recorded
// consistency checks, not authentication of an executable or its producer.
type nativeSessionProcessTiming struct {
	StartNS              int64  `json:"start_ns"`
	WaitNS               int64  `json:"wait_ns"`
	WaitLimitNS          int64  `json:"wait_limit_ns,omitempty"`
	DeadlineRemainingNS  *int64 `json:"deadline_remaining_ns,omitempty"`
	ContextAtStartReturn string `json:"context_at_start_return"`
}

type nativeSessionProcess struct {
	Started          bool                        `json:"started"`
	Completed        bool                        `json:"completed"`
	Canceled         bool                        `json:"canceled"`
	TimedOut         bool                        `json:"timed_out"`
	DiagnosticsBytes int                         `json:"diagnostics_bytes"`
	OutputTruncated  bool                        `json:"output_truncated"`
	ExitCode         *int                        `json:"exit_code"`
	WallNS           int64                       `json:"wall_ns"`
	UserNS           int64                       `json:"user_ns"`
	SystemNS         int64                       `json:"system_ns"`
	PeakRSSBytes     *int64                      `json:"peak_rss_bytes"`
	StdoutSHA256     string                      `json:"stdout_sha256"`
	StderrSHA256     string                      `json:"stderr_sha256"`
	Failure          string                      `json:"failure,omitempty"`
	Diagnostics      []byte                      `json:"diagnostics,omitempty"`
	Timing           *nativeSessionProcessTiming `json:"timing,omitempty"`
	FailurePhase     string                      `json:"failure_phase,omitempty"`
}

type nativeSessionArtifact struct {
	Schema             string               `json:"schema"`
	KeySHA256          string               `json:"key_sha256"`
	Reused             bool                 `json:"reused"`
	ExecutableVerified bool                 `json:"executable_verified"`
	SourceBuild        nativeSessionProcess `json:"source_build"`
	SourceBuildSHA256  string               `json:"source_build_sha256"`
	MissReason         string               `json:"miss_reason,omitempty"`
	WaitNS             int64                `json:"wait_ns"`
}

func inspectSessionWithRuntime(raw []byte, s suite, replay bool, identity expectedIdentity, reported *nativeReportedRuntime) (checked, error) {
	if identity.CompilerRevision != nativeSessionCompilerRevision || identity.GoVersion != "go1.27.3" {
		return checked{}, fmt.Errorf("invalid evidence: session inspection requires explicit accepted compiler276 and Go1.27.3")
	}
	return inspectWithRuntimeProfile(raw, s, replay, identity, reported, "gooo/body-composition-runtime/v2", inspectNativeSessionOwnership)
}

// The shared inspector already rejected duplicate keys and checked current
// source, input, finite results, entry, producer, generated code and replay.
func inspectNativeSessionOwnership(raw []byte) error {
	var root map[string]json.RawMessage
	var result, runtime map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || json.Unmarshal(root["result"], &result) != nil || json.Unmarshal(result["runtime"], &runtime) != nil {
		return fmt.Errorf("session runtime object is missing")
	}
	for _, key := range []string{"artifact", "executable_sha256", "build", "runs"} {
		if value, ok := runtime[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("session runtime lacks %s", key)
		}
	}
	var artifact nativeSessionArtifact
	fields, err := nativeSessionObject(runtime["artifact"], &artifact,
		[]string{"schema", "key_sha256", "reused", "executable_verified", "source_build", "source_build_sha256", "wait_ns"}, []string{"miss_reason"})
	if err != nil {
		return fmt.Errorf("session artifact: %w", err)
	}
	sourceBuild, err := decodeNativeSessionProcess(fields["source_build"])
	if err != nil {
		return fmt.Errorf("session source build: %w", err)
	}
	var executable string
	if json.Unmarshal(runtime["executable_sha256"], &executable) != nil || !validDigest(executable) ||
		artifact.Schema != "gooo/owned-native-artifact/v1" || !validDigest(artifact.KeySHA256) ||
		!artifact.ExecutableVerified || artifact.WaitNS < 0 || !validDigest(artifact.SourceBuildSHA256) {
		return fmt.Errorf("session artifact identity or executable binding differs")
	}
	if err := successfulNativeSessionProcess(sourceBuild); err != nil {
		return fmt.Errorf("session historical source build: %w", err)
	}
	bound, err := json.Marshal([]any{artifact.KeySHA256, executable, sourceBuild})
	if err != nil || contentDigest(bound) != artifact.SourceBuildSHA256 {
		return fmt.Errorf("session source build digest differs")
	}
	build, err := decodeNativeSessionProcess(runtime["build"])
	if err != nil {
		return fmt.Errorf("session current build: %w", err)
	}
	if artifact.Reused {
		// accepted276 initialCompositionRuntime leaves Build zero; the cache-hit
		// return in compositionExecutableFor precedes buildCompositionExecutable.
		if artifact.MissReason != "" || !reflect.DeepEqual(build, nativeSessionProcess{}) {
			return fmt.Errorf("reused session artifact invents current build work or a miss")
		}
	} else {
		switch artifact.MissReason {
		case "empty", "key_changed", "executable_changed_or_missing", "tool_dependencies_unbound":
		default:
			return fmt.Errorf("fresh session artifact lacks a recorded supported miss")
		}
		// On a miss accepted276 records copyProcess(r.Build) as SourceBuild.
		if !reflect.DeepEqual(build, sourceBuild) {
			return fmt.Errorf("fresh session artifact differs from its current build")
		}
	}
	var runs []json.RawMessage
	if json.Unmarshal(runtime["runs"], &runs) != nil || len(runs) != 2 {
		return fmt.Errorf("session request lacks two current native runs")
	}
	for index, rawRun := range runs {
		run, err := decodeNativeSessionProcess(rawRun)
		if err == nil {
			err = successfulNativeSessionProcess(run)
		}
		if err != nil {
			return fmt.Errorf("session current native run%d: %w", index+1, err)
		}
	}
	return nil
}

func successfulNativeSessionProcess(p nativeSessionProcess) error {
	// accepted276 processWithOutputLimit rejects any stderr bytes, even exit0:
	// it marks Completed=false, records diagnostics and returns an OUTPUT error.
	if !p.Started || !p.Completed || p.ExitCode == nil || *p.ExitCode != 0 || p.Canceled || p.TimedOut ||
		p.OutputTruncated || p.DiagnosticsBytes != 0 || len(p.Diagnostics) != 0 || p.Failure != "" || p.FailurePhase != "" ||
		p.WallNS < 0 || p.UserNS < 0 || p.SystemNS < 0 || p.PeakRSSBytes != nil && *p.PeakRSSBytes < 0 ||
		!validDigest(p.StdoutSHA256) || p.StderrSHA256 != contentDigest(nil) || p.Timing == nil {
		return fmt.Errorf("successful started/completed native process observation is required")
	}
	t := p.Timing
	if t.StartNS < 0 || t.WaitNS < 0 || t.WaitLimitNS < 0 || t.ContextAtStartReturn != "ACTIVE" ||
		t.StartNS > p.WallNS || t.WaitNS != p.WallNS-t.StartNS {
		return fmt.Errorf("native process timing is inconsistent")
	}
	return nil
}

func decodeNativeSessionProcess(raw []byte) (nativeSessionProcess, error) {
	var p nativeSessionProcess
	fields, err := nativeSessionObject(raw, &p,
		[]string{"started", "completed", "canceled", "timed_out", "diagnostics_bytes", "output_truncated", "exit_code", "wall_ns", "user_ns", "system_ns", "peak_rss_bytes", "stdout_sha256", "stderr_sha256"},
		[]string{"failure", "diagnostics", "timing", "failure_phase"})
	if err != nil {
		return p, err
	}
	if p.Timing != nil {
		_, err = nativeSessionObject(fields["timing"], p.Timing,
			[]string{"start_ns", "wait_ns", "context_at_start_return"}, []string{"wait_limit_ns", "deadline_remaining_ns"})
	}
	return p, err
}

// Only the small, pinned ownership/process objects use this exact-key decoder.
// Unknown or case-folded fields cannot disappear from a build's hashed form.
func nativeSessionObject(raw []byte, target any, required, optional []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("object required")
	}
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		allowed[key] = true
		value, ok := fields[key]
		if !ok || key != "exit_code" && key != "peak_rss_bytes" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("missing or null field %s", key)
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range fields {
		if !allowed[key] {
			return nil, fmt.Errorf("unsupported field %s", key)
		}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, err
	}
	return fields, nil
}
