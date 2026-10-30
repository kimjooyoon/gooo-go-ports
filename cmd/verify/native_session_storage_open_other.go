//go:build !linux && !darwin

package main

import "os"

// The session transport is unsupported here. This portable ordinary-file
// helper retains cross-platform compilation and the common fstat/identity
// checks; Unix nonblocking/no-follow guarantees are not claimed on these OSes.
func openNativeSessionEvidenceFile(path string) (*os.File, error) {
	return os.Open(path)
}
