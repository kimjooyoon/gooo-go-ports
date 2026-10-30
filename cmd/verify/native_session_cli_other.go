//go:build !linux && !darwin

package main

import (
	"context"
	"fmt"
)

func nativeSessionPlatformSupported() bool { return false }

func newNativeSessionCLIProfile(context.Context, string, string, string, upstream, *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
	return nativeSuiteVerifier{}, nil, fmt.Errorf("experimental native sessions require Linux or macOS")
}

func newNativeSessionCLIGzipProfile(context.Context, string, string, string, upstream, *nativeBuildCache) (nativeSuiteVerifier, nativeSessionCLIOwner, error) {
	return nativeSuiteVerifier{}, nil, fmt.Errorf("experimental native sessions require Linux or macOS")
}

func readNativeSessionCLICompilerIdentity(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("experimental native sessions require Linux or macOS")
}
