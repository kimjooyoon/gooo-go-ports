package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A cache belongs to one verifier run. Only compiled packages are shared;
// evidence, generated inputs and subprocess temporary files remain per suite.
type nativeBuildCache struct {
	dir   string
	owner os.FileInfo
}

func newNativeBuildCache(evidence string) (*nativeBuildCache, error) {
	created, err := os.MkdirTemp("", "gooo-native-build-cache-")
	if err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(created)
	if err != nil {
		_ = os.RemoveAll(created)
		return nil, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		_ = os.RemoveAll(created)
		return nil, err
	}
	// TMPDIR may be supplied by the caller. A fresh private cache is still
	// required to stay outside both the evidence and CI's uploaded out/ tree.
	for _, excluded := range []string{evidence, "out"} {
		root, err := filepath.Abs(excluded)
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		} else if !os.IsNotExist(err) {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("native build cache must be outside %s", root)
		}
	}
	owner, err := os.Lstat(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return &nativeBuildCache{dir: dir, owner: owner}, nil
}

func (c *nativeBuildCache) directory(fallback string) (string, error) {
	if c == nil {
		return fallback, nil
	}
	current, err := os.Lstat(c.dir)
	if err != nil {
		return "", err
	}
	if !current.IsDir() || c.owner == nil || !os.SameFile(c.owner, current) {
		return "", fmt.Errorf("native build cache ownership changed")
	}
	return c.dir, nil
}

func (c *nativeBuildCache) close() error {
	if _, err := c.directory(""); err != nil {
		return err
	}
	return os.RemoveAll(c.dir)
}
