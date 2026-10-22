package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ciShardStoredReceiptLimit = 8 << 20
const ciShardDecodedReceiptLimit = 64 << 20

type ciShardCompressedBuffer struct{ bytes.Buffer }

func (b *ciShardCompressedBuffer) Write(p []byte) (int, error) {
	if len(p) > ciShardStoredReceiptLimit-b.Len() {
		return 0, fmt.Errorf("compressed shard receipt exceeds storage bound")
	}
	return b.Buffer.Write(p)
}

func decodeCIShardGzip(raw []byte, limit int64) ([]byte, error) {
	if len(raw) == 0 || len(raw) > ciShardStoredReceiptLimit || limit <= 0 || limit > ciShardDecodedReceiptLimit {
		return nil, fmt.Errorf("bounded compressed shard receipt required")
	}
	input := bytes.NewReader(raw)
	source, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}
	source.Multistream(false)
	decoded, readErr := io.ReadAll(io.LimitReader(source, limit+1))
	err = errors.Join(readErr, source.Close())
	if err != nil {
		return nil, err
	}
	if int64(len(decoded)) > limit || len(decoded) == 0 || input.Len() != 0 {
		return nil, fmt.Errorf("oversized, empty, trailing or multiple-member gzip receipt")
	}
	return decoded, nil
}

// Only the experimental shard invokes this after complete batch validation.
// Write and independently decode both bounded gzip files before deleting either
// original. Any failure keeps the originals and is a failed producer operation.
func compactCIShardReceipts(directory string) error {
	reader, err := newCIShardEvidenceReader(directory)
	if err != nil {
		return err
	}
	names := []string{"execution.json", "replay.json"}
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(directory, name+".gz")); !os.IsNotExist(err) {
			return fmt.Errorf("fresh compressed receipt required: %s", name)
		}
	}
	for _, name := range names {
		raw, err := reader.read(name, ciShardDecodedReceiptLimit)
		if err != nil {
			return err
		}
		var packed ciShardCompressedBuffer
		writer, err := gzip.NewWriterLevel(&packed, gzip.BestSpeed)
		if err != nil {
			return err
		}
		_, writeErr := writer.Write(raw)
		err = errors.Join(writeErr, writer.Close())
		if err != nil {
			return err
		}
		path := filepath.Join(directory, name+".gz")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr = file.Write(packed.Bytes())
		err = errors.Join(writeErr, file.Sync(), file.Close())
		if err != nil {
			return err
		}
		saved, err := readCIShardFile(path, ciShardStoredReceiptLimit)
		if err != nil {
			return err
		}
		decoded, err := decodeCIShardGzip(saved, ciShardDecodedReceiptLimit)
		if err != nil {
			return err
		}
		if !bytes.Equal(decoded, raw) {
			return fmt.Errorf("compressed receipt did not preserve original JSON bytes")
		}
	}
	if err := reader.finish(); err != nil {
		return err
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return err
		}
	}
	return nil
}

func ciShardReceiptLogicalName(name string) (string, error) {
	plain := strings.TrimSuffix(name, ".gz")
	base := filepath.Base(plain)
	if !filepath.IsLocal(plain) || filepath.Clean(plain) != plain || (base != "execution.json" && base != "replay.json") {
		return "", fmt.Errorf("canonical shard receipt path required")
	}
	return plain, nil
}

func (r *ciShardEvidenceReader) receiptPath(name string) (string, error) {
	plain, err := ciShardReceiptLogicalName(name)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(plain)
	if parent == "." {
		parent = ""
	}
	dir, err := r.directory(parent)
	if err != nil {
		return "", err
	}
	selected := ""
	for _, candidate := range []string{plain, plain + ".gz"} {
		_, err := os.Lstat(filepath.Join(dir, filepath.Base(candidate)))
		if err == nil {
			if selected != "" {
				return "", fmt.Errorf("ambiguous plain and gzip shard receipt")
			}
			selected = candidate
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if selected == "" {
		return "", fmt.Errorf("missing shard receipt: %s", plain)
	}
	// An index naming a compressed file must not silently select a plain one.
	if strings.HasSuffix(name, ".gz") && selected != name {
		return "", fmt.Errorf("compressed receipt path changed")
	}
	return selected, nil
}

// The snapshot reader pins actual stored bytes. Callers inspect and hash the
// exact decoded JSON, preserving the original saved-replay parent identity.
func (r *ciShardEvidenceReader) receipt(name string) (ciShardReferenceFile, []byte, error) {
	var zero ciShardReferenceFile
	path, err := r.receiptPath(name)
	if err != nil {
		return zero, nil, err
	}
	compressed := strings.HasSuffix(path, ".gz")
	limit := int64(ciShardDecodedReceiptLimit)
	if compressed {
		limit = ciShardStoredReceiptLimit
	}
	stored, err := r.read(path, limit)
	if err != nil {
		return zero, nil, err
	}
	raw := stored
	if compressed {
		raw, err = decodeCIShardGzip(stored, ciShardDecodedReceiptLimit)
		if err != nil {
			return zero, nil, err
		}
	}
	if again, err := r.receiptPath(name); err != nil || again != path {
		return zero, nil, fmt.Errorf("shard receipt representation changed: %v", err)
	}
	identity := ciShardReferenceFile{Path: path, Bytes: int64(len(raw)), SHA256: contentDigest(raw)}
	if compressed {
		identity.Encoding = "gzip"
		identity.StoredBytes = int64(len(stored))
		identity.StoredSHA256 = contentDigest(stored)
	}
	return identity, raw, nil
}
