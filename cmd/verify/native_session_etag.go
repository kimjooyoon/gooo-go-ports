package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SourceCheck uses the existing exact compiler276 process encoding; SourceOutput
// is []byte and therefore participates in its bound Go JSON array as base64.
type nativeSessionToolchainReference struct {
	Schema            string               `json:"schema"`
	KeySHA256         string               `json:"key_sha256"`
	GoToolSHA256      string               `json:"go_tool_sha256"`
	GoVersion         string               `json:"go_version"`
	Reused            bool                 `json:"reused"`
	BytesVerified     bool                 `json:"bytes_verified"`
	BuildInfoVerified bool                 `json:"build_info_verified"`
	SourceCheck       nativeSessionProcess `json:"source_check"`
	SourceOutput      []byte               `json:"source_output"`
	SourceCheckSHA256 string               `json:"source_check_sha256"`
}

func inspectNativeSessionETagABI(raw []byte, s suite, identity expectedIdentity) error {
	if identity.CompilerRevision != nativeSessionCompilerRevision || identity.GoVersion != nativeSessionProfileGoVersion {
		return fmt.Errorf("session HTTP ABI requires explicit compiler276 and Go1.27.3")
	}
	return inspectETagABIWithProcesses(raw, s, identity, inspectNativeSessionETagProcesses)
}

func inspectNativeSessionETagProcesses(raw []byte) error {
	// This validates artifact ownership, its historical source build, current
	// build/reuse accounting and both fresh runs. The shared profile validator
	// also remains in its original later position after all semantic checks.
	if err := inspectNativeSessionOwnership(raw); err != nil {
		return fmt.Errorf("session HTTP ownership: %w", err)
	}
	var receipt struct {
		Result struct {
			Runtime struct {
				Schema             string          `json:"schema"`
				Producer           string          `json:"producer_source_sha"`
				GoVersion          string          `json:"go_version"`
				GoToolSelection    string          `json:"go_tool_selection"`
				GoToolSHA256       string          `json:"go_tool_sha256"`
				Toolchain          json.RawMessage `json:"toolchain"`
				ToolchainReference json.RawMessage `json:"toolchain_reference"`
			} `json:"runtime"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return err
	}
	runtime := receipt.Result.Runtime
	version := strings.Fields(runtime.GoVersion)
	if runtime.Schema != "gooo/body-composition-runtime/v2" || runtime.Producer != nativeSessionCompilerRevision ||
		runtime.GoToolSelection != "explicit" || !validDigest(runtime.GoToolSHA256) ||
		len(version) != 4 || version[0] != "go" || version[1] != "version" || version[2] != nativeSessionProfileGoVersion || !strings.Contains(version[3], "/") {
		return fmt.Errorf("session HTTP runtime/toolchain profile differs")
	}
	var reference nativeSessionToolchainReference
	fields, err := nativeSessionObject(runtime.ToolchainReference, &reference,
		[]string{"schema", "key_sha256", "go_tool_sha256", "go_version", "reused", "bytes_verified", "build_info_verified", "source_check", "source_output", "source_check_sha256"}, nil)
	if err != nil {
		return fmt.Errorf("session HTTP toolchain reference: %w", err)
	}
	sourceCheck, err := decodeNativeSessionProcess(fields["source_check"])
	if err != nil {
		return fmt.Errorf("session HTTP historical toolchain check: %w", err)
	}
	if reference.Schema != "gooo/owned-go-toolchain/v1" || !validDigest(reference.KeySHA256) ||
		reference.GoToolSHA256 != runtime.GoToolSHA256 || reference.GoVersion != runtime.GoVersion ||
		!reference.BytesVerified || !reference.BuildInfoVerified || !validDigest(reference.SourceCheckSHA256) {
		return fmt.Errorf("session HTTP bound toolchain identity differs")
	}
	if err := successfulNativeSessionProcess(sourceCheck); err != nil {
		return fmt.Errorf("session HTTP historical toolchain completion: %w", err)
	}
	if sourceCheck.WallNS <= 0 {
		return fmt.Errorf("session HTTP historical toolchain check requires positive wall time")
	}
	if len(reference.SourceOutput) == 0 || len(reference.SourceOutput) > 256 ||
		!bytes.Equal(bytes.TrimSpace(reference.SourceOutput), []byte(runtime.GoVersion)) ||
		sourceCheck.StdoutSHA256 != contentDigest(reference.SourceOutput) {
		return fmt.Errorf("session HTTP original toolchain output/version binding differs")
	}
	// The environment-derived key is a valid bound digest, not rederived here.
	bound, err := json.Marshal([]any{reference.KeySHA256, reference.GoToolSHA256, reference.GoVersion, sourceCheck, reference.SourceOutput})
	if err != nil || contentDigest(bound) != reference.SourceCheckSHA256 {
		return fmt.Errorf("session HTTP historical toolchain digest differs")
	}
	current, err := decodeNativeSessionProcess(runtime.Toolchain)
	if err != nil {
		return fmt.Errorf("session HTTP current toolchain check: %w", err)
	}
	// Toolchain and artifact reuse are independent. No current work is invented
	// from a historical check; a fresh check must match its bound observation.
	if reference.Reused {
		if !reflect.DeepEqual(current, nativeSessionProcess{}) {
			return fmt.Errorf("reused session HTTP toolchain invents current work")
		}
	} else if !reflect.DeepEqual(current, sourceCheck) {
		return fmt.Errorf("fresh session HTTP toolchain differs from its source check")
	}
	return nil
}
