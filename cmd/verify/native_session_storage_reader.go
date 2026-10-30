package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// The opener is platform-specific. Unix refuses symlink following and cannot
// block on a FIFO substituted after the regular-file lstat. Every opened file
// is rechecked before reading, and closed on success or any refusal.
func readNativeSessionEvidenceFile(path string, limit int64) ([]byte, error) {
	return readNativeSessionEvidenceFileWithOpen(path, limit, openNativeSessionEvidenceFile)
}

func readNativeSessionEvidenceFileWithOpen(path string, limit int64, opener func(string) (*os.File, error)) (raw []byte, returnedErr error) {
	if limit <= 0 || limit > nativeSessionEvidenceStoredLimit || opener == nil {
		return nil, fmt.Errorf("bounded native session evidence read required")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("bounded regular native session evidence required")
	}
	file, err := opener(path)
	if file != nil {
		defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	}
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("native session evidence opener returned no file")
	}
	opened, err := file.Stat()
	if err != nil || !nativeSessionSameFileState(before, opened) {
		return nil, fmt.Errorf("native session evidence changed before read: %v", err)
	}
	raw, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	openedAfter, err := file.Stat()
	if err != nil || !nativeSessionSameFileState(before, openedAfter) {
		return nil, fmt.Errorf("opened native session evidence changed during read: %v", err)
	}
	after, err := os.Lstat(path)
	if err != nil || !nativeSessionSameFileState(before, after) || int64(len(raw)) != before.Size() || int64(len(raw)) > limit {
		return nil, fmt.Errorf("native session evidence path/length changed during read: %v", err)
	}
	return raw, nil
}

func nativeSessionSameFileState(before, after os.FileInfo) bool {
	return before != nil && after != nil && before.Mode().IsRegular() && after.Mode().IsRegular() &&
		os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

// Reuse only the shard reader's directory pins and snapshot maps. Its original
// read/finish methods retain their existing behavior and are never invoked by
// this session-specific wrapper. Hash checks pin exact bytes across re-reads.
type nativeSessionEvidenceReader struct {
	*ciShardEvidenceReader
	readFile func(string, int64) ([]byte, error)
}

func newNativeSessionEvidenceReader(root string) (*nativeSessionEvidenceReader, error) {
	base, err := newCIShardEvidenceReader(root)
	if err != nil {
		return nil, err
	}
	return &nativeSessionEvidenceReader{ciShardEvidenceReader: base, readFile: readNativeSessionEvidenceFile}, nil
}

func (r *nativeSessionEvidenceReader) read(name string, limit int64) ([]byte, error) {
	if !filepath.IsLocal(name) || filepath.Clean(name) != name {
		return nil, fmt.Errorf("native session evidence path escapes snapshot")
	}
	parent := filepath.Dir(name)
	if parent == "." {
		parent = ""
	}
	dir, err := r.directory(parent)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, filepath.Base(name))
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	raw, err := r.readFile(path, limit)
	if err != nil {
		return nil, err
	}
	if _, err := r.directory(parent); err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !nativeSessionSameFileState(before, after) {
		return nil, fmt.Errorf("native session evidence snapshot changed: %v", err)
	}
	digest := contentDigest(raw)
	if prior, exists := r.files[name]; exists && (!nativeSessionSameFileState(prior.info, after) || prior.digest != digest) {
		return nil, fmt.Errorf("native session evidence changed after inspection: %s", name)
	}
	if _, exists := r.files[name]; !exists {
		r.order = append(r.order, name)
	}
	r.files[name] = ciShardEvidenceFile{info: after, digest: digest}
	return raw, nil
}

func (r *nativeSessionEvidenceReader) finish() error {
	for _, name := range r.order {
		if _, err := r.read(name, r.files[name].info.Size()); err != nil {
			return err
		}
	}
	return nil
}
