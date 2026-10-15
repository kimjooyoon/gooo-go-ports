package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Share compiled packages between correctness fixtures in this test process.
// Every fixture still executes with fresh source, temporary files and reports.
// The nil-cache and dedicated cache ownership tests keep their own paths.
var correctnessNativeBuildCache *nativeBuildCache

func TestMain(m *testing.M) {
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Fprintln(os.Stderr, "native correctness build cache root:", err)
		os.Exit(1)
	}
	cache, err := newNativeBuildCache(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "native correctness build cache:", err)
		os.Exit(1)
	}
	correctnessNativeBuildCache = cache
	code := m.Run()
	if err := cache.close(); err != nil {
		fmt.Fprintln(os.Stderr, "native correctness build cache cleanup:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func nativeCorrectnessCache(t *testing.T) *nativeBuildCache {
	t.Helper()
	if correctnessNativeBuildCache == nil {
		t.Fatal("native correctness build cache is unavailable")
	}
	if _, err := correctnessNativeBuildCache.directory(""); err != nil {
		t.Fatal("native correctness build cache:", err)
	}
	return correctnessNativeBuildCache
}
