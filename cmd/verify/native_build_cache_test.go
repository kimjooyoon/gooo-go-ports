package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNativeBuildCacheRunOwnership(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	first, err := newNativeBuildCache(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := newNativeBuildCache(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.close(); err != nil {
			t.Error(err)
		}
	})
	if first.dir == second.dir || !filepath.IsAbs(first.dir) || first.owner.Mode().Perm() != 0700 {
		t.Fatal("cache is not private and unique to its run")
	}
	// A changed path must not allow reuse or cleanup of somebody else's files.
	if err := os.Rename(first.dir, first.dir+"-owned"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(first.dir, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(first.dir, "keep")
	if err := os.WriteFile(sentinel, []byte("another owner"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := first.directory(""); err == nil {
		t.Fatal("reused a replaced cache directory")
	}
	if err := first.close(); err == nil {
		t.Fatal("removed a replaced cache directory")
	}
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "another owner" {
		t.Fatal("cleanup touched unrelated files", err)
	}
}

func TestNativeBuildCacheRejectsUploadedLocations(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, name := range []string{"evidence", "out"} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "out"), "out-alias"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"evidence", "out", "out-alias"} {
		t.Run(name, func(t *testing.T) {
			parent := filepath.Join(root, name)
			t.Setenv("TMPDIR", parent)
			if cache, err := newNativeBuildCache(filepath.Join(root, "evidence")); err == nil {
				_ = cache.close()
				t.Fatal("allowed a build cache inside uploaded evidence")
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatal("rejected cache was left in uploaded evidence", err)
			}
		})
	}
}

func TestNativeBuildCacheKeepsExecutionAndResultsFresh(t *testing.T) {
	root := t.TempDir()
	cache, err := newNativeBuildCache(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cache.close(); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("GOCACHE", "/must-not-use-caller-cache")
	t.Setenv("GOCACHEPROG", "/must-not-use-caller-cache-program")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	text := simpleFoldSyntheticSource("", "x")
	digest := contentDigest([]byte(text))
	for _, name := range []string{"first", "second"} {
		dir := filepath.Join(root, name)
		got, err := checkSimpleFoldExhaustive(ctx, dir, mustJSON(t, simpleFoldFixture(text)), digest, cache)
		if err != nil || !reflect.DeepEqual(got, expectedSimpleFoldExhaustive(digest)) {
			t.Fatal("same source did not execute with a fresh report", err)
		}
		for _, file := range []string{"native/generated.go", "native-test.txt", "exhaustive.json"} {
			if raw, err := os.ReadFile(filepath.Join(dir, file)); err != nil || len(raw) == 0 {
				t.Fatal("missing independent inputs or execution results", file, err)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "cache")); !os.IsNotExist(err) {
			t.Fatal("shared compilation cache appeared in evidence", err)
		}
	}
	firstReport := filepath.Join(root, "first", "exhaustive.json")
	before, err := os.ReadFile(firstReport)
	if err != nil {
		t.Fatal(err)
	}
	// The cache is warm, but new source must still reach a late oracle mismatch.
	changed := simpleFoldSyntheticSource("case 1114111: return 0\n", "x")
	failed := filepath.Join(root, "changed")
	_, err = checkSimpleFoldExhaustive(ctx, failed, mustJSON(t, simpleFoldFixture(changed)), contentDigest([]byte(changed)), cache)
	if err == nil || !strings.Contains(err.Error(), "SimpleFold(1114111)") {
		t.Fatal("warm cache hid a changed function's final-range mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(failed, "exhaustive.json")); !os.IsNotExist(err) {
		t.Fatal("failed execution reused a successful report", err)
	}
	if after, err := os.ReadFile(firstReport); err != nil || string(before) != string(after) {
		t.Fatal("later execution changed the earlier result", err)
	}
}
