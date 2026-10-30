package main

import (
	"fmt"
	"path/filepath"
	"reflect"
)

type nativeExecutionProfile string

const nativeExecutionSession276 nativeExecutionProfile = "experimental-session-276-v1"
const nativeExecutionSession276Gzip nativeExecutionProfile = "experimental-session-276-gzip-v1"

func parseNativeExecutionProfile(value string) (nativeExecutionProfile, error) {
	switch value {
	case "", "stateless":
		return "", nil
	case string(nativeExecutionSession276):
		return nativeExecutionSession276, nil
	case string(nativeExecutionSession276Gzip):
		return nativeExecutionSession276Gzip, nil
	default:
		return "", fmt.Errorf("unknown --execution-profile %q", value)
	}
}

// This profile changes transport and receipt producer only. Other execution
// contracts need their own compatibility evidence before accepting sessions.
func (p nativeExecutionProfile) validate(compiler, out, browser, timing string, cases nativeCaseProfile, workspace nativeWorkspaceProfile, shardRequested bool) error {
	if p == "" {
		return nil
	}
	if out == "" || browser != "" || shardRequested || timing != "" || cases != "" || workspace != "" {
		return fmt.Errorf("experimental session requires --out, legacy case/workspace profiles, and no browser, CI shard or --timing-out mode")
	}
	if !filepath.IsAbs(compiler) {
		return fmt.Errorf("experimental session requires an explicit absolute --compiler path")
	}
	if !nativeSessionPlatformSupported() {
		return fmt.Errorf("experimental session is supported only on Linux and Darwin")
	}
	return nil
}

func (p nativeExecutionProfile) decorateSummary(summary map[string]any, lock upstream) {
	if p == "" {
		return
	}
	summary["schema"] = "gooo/go-ports-session-differential/v1"
	summary["execution_profile"] = string(p)
	summary["compiler_revision"] = nativeSessionCompilerRevision
	summary["oracle_lock_compiler_revision"] = lock.CompilerRevision
	summary["session_observations"] = "session-observations.json"
	summary["session_source_identity"] = "session-source.json"
	if p == nativeExecutionSession276Gzip {
		summary["receipt_storage"] = map[string]any{"encoding": "gzip", "logical_identity": "exact decoded original JSON bytes including LF", "stored_identity": "actual gzip blob bytes; separate stored_sha256 and stored_bytes"}
	}
	summary["scope"] = "Finite original legacy-corpus differential observations with the explicitly selected compiler276 session producer. Original Go oracle sources, inputs, batch boundaries, fresh execute and own-parent saved replay remain required. Runtime/v2 metadata is unsigned consistency evidence; per-request process wait resources are not observed. This does not qualify default205, compact/dependency profiles, CI shard readers, general compatibility, performance or production safety."
}

// Reuse the original source/catalog/binary hash reader without creating a
// per-command timing ledger. The optional profile's extra readers are bound
// separately; they are not silently inserted into the historical293 roster.
func nativeSessionCLISourceIdentity(cfg nativeTimingConfig) (nativeTimingSnapshot, error) {
	snapshot, err := nativeTimingIdentity(cfg)
	if err != nil {
		return snapshot, err
	}
	for _, pattern := range []string{"cmd/verify/native_session*.go", "cmd/verify/native_execution_profile*.go"} {
		paths, err := filepath.Glob(filepath.Join(cfg.RepositoryRoot, pattern))
		if err != nil || len(paths) == 0 {
			return snapshot, fmt.Errorf("missing session profile source inventory: %s: %v", pattern, err)
		}
		for _, path := range paths {
			name, err := filepath.Rel(cfg.RepositoryRoot, path)
			if err != nil {
				return snapshot, err
			}
			name = filepath.ToSlash(name)
			path, err = nativeTimingSourcePath(cfg.RepositoryRoot, name)
			if err != nil {
				return snapshot, err
			}
			observed, err := nativeTimingHashFile(path)
			if err != nil {
				return snapshot, err
			}
			snapshot.Sources[name] = observed
		}
	}
	return snapshot, nil
}

func checkNativeSessionCLISourceIdentity(cfg nativeTimingConfig, original nativeTimingSnapshot, out string) error {
	current, err := nativeSessionCLISourceIdentity(cfg)
	unchanged := err == nil && reflect.DeepEqual(original, current)
	record := map[string]any{"schema": "gooo/session-source-identity-final/v1", "unchanged": unchanged, "observed": current}
	if err != nil {
		record["error"] = err.Error()
	}
	if saveErr := save(out, record); saveErr != nil {
		return saveErr
	}
	if !unchanged {
		return fmt.Errorf("session source/catalog/binary identity changed during verification: %v", err)
	}
	return nil
}
