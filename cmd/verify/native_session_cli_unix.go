//go:build linux || darwin

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func nativeSessionPlatformSupported() bool { return true }

func newNativeSessionCLIProfile(ctx context.Context, compiler, goTool, root string, lock upstream, cache *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
	verifier, client, err := newExperimentalNativeSessionVerifier(ctx, compiler, goTool, root, lock, cache)
	if err != nil {
		return nativeSuiteVerifier{}, nil, err
	}
	return verifier, client, nil
}

// The top-level build identity retains the same raw compiler.json role as the
// default CLI. Probe process metadata is separate, and is saved before checks.
func readNativeSessionCLICompilerIdentity(ctx context.Context, compiler, evidenceRoot string) ([]byte, error) {
	if ctx == nil || !filepath.IsAbs(compiler) || !filepath.IsAbs(evidenceRoot) {
		return nil, errors.New("session compiler identity requires context and absolute paths")
	}
	raw, probeErr := nativeSessionProfileProbe(ctx, evidenceRoot, "compiler-probe", compiler, "version", "--build", "--json")
	if probeErr != nil {
		// The bounded probe preserves consumed stdout even on timeout/refusal.
		preserved, readErr := os.ReadFile(filepath.Join(evidenceRoot, "compiler-probe.stdout"))
		if readErr != nil {
			return nil, errors.Join(probeErr, readErr)
		}
		raw = preserved
	}
	if err := nativeSessionCLIWriteNew(filepath.Join(evidenceRoot, "compiler.json"), raw); err != nil {
		return nil, errors.Join(probeErr, err)
	}
	if probeErr != nil {
		return nil, probeErr
	}
	if err := validateNativeSessionCompilerIdentity(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
