//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Called only by the verifier's existing post-inspection/exhaustive/own-parent
// hook. Execute/replay themselves always save and consume original plain bytes.
func (c *nativeSessionClient) compactBatch(directory string) error {
	if c.storage != nativeSessionEvidenceGzip {
		return fmt.Errorf("session receipt compaction requires immutable gzip profile")
	}
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-c.ctx.Done():
		return c.ctx.Err()
	case <-c.done:
		return c.failure()
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	names := []string{"execution.json", "replay.json"}
	indices := make([]int, len(names))
	originals := make([]nativeSessionFileReference, len(names))
	for i, name := range names {
		path := filepath.Join(directory, name)
		if err := c.ownedPath(path, true); err != nil {
			return err
		}
		if _, err := os.Lstat(path + ".gz"); !os.IsNotExist(err) {
			return fmt.Errorf("fresh compressed session receipt required: %s", name)
		}
		index := -1
		c.mu.Lock()
		for n, observation := range c.observations {
			if observation.Receipt != nil && observation.Receipt.Path == path {
				if index != -1 || observation.Error != "" || observation.Operation != []string{"execute", "replay"}[i] {
					c.mu.Unlock()
					return fmt.Errorf("ambiguous or failed session receipt reference")
				}
				index = n
				originals[i] = *observation.Receipt
			}
		}
		c.mu.Unlock()
		if index == -1 || originals[i].Encoding != "" || originals[i].StoredBytes != 0 || originals[i].StoredSHA256 != "" {
			return fmt.Errorf("original inspected plain session receipt required")
		}
		indices[i] = index
	}
	reader, err := newNativeSessionEvidenceReader(directory)
	if err != nil {
		return err
	}
	compressed := make([]nativeSessionFileReference, len(names))
	for i, name := range names {
		raw, err := reader.read(name, nativeSessionEvidenceDecodedLimit)
		if err != nil {
			return err
		}
		if nativeSessionReference(originals[i].Path, raw) != originals[i] {
			return fmt.Errorf("original session receipt changed before compaction")
		}
		compressed[i], err = writeNativeSessionStoredEvidence(originals[i].Path, raw, nativeSessionEvidenceGzip)
		if err != nil {
			return err
		}
		stored, err := reader.read(name+".gz", nativeSessionEvidenceStoredLimit)
		if err != nil || int64(len(stored)) != compressed[i].StoredBytes || nativeSessionReference(compressed[i].Path, stored).SHA256 != compressed[i].StoredSHA256 {
			return fmt.Errorf("persisted gzip session receipt changed: %v", err)
		}
	}
	// Pin both complete originals and both persisted gzip blobs before removing
	// either original. An earlier failure leaves both original files unchanged.
	if err := reader.finish(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.closed || c.ctx.Err() != nil {
		return fmt.Errorf("session closed or canceled before receipt compaction commit")
	}
	for i := range names {
		if current := c.observations[indices[i]].Receipt; current == nil || *current != originals[i] {
			return fmt.Errorf("session receipt reference changed during compaction")
		}
	}
	// References already point to independently verified lossless copies before
	// deletion. Even a later filesystem removal failure preserves usable proof.
	for i := range names {
		ref := compressed[i]
		c.observations[indices[i]].Receipt = &ref
	}
	for _, name := range names {
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err != nil || !os.SameFile(reader.files[name].info, info) || info.Size() != reader.files[name].info.Size() || !info.ModTime().Equal(reader.files[name].info.ModTime()) {
			return fmt.Errorf("original session receipt changed before removal")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
