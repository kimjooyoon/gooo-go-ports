package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type nativeSessionEvidenceStorage string

const nativeSessionEvidencePlain nativeSessionEvidenceStorage = ""
const nativeSessionEvidenceGzip nativeSessionEvidenceStorage = "gzip"
const nativeSessionEvidenceDecodedLimit = 16 << 20

// Incompressible16MiB frames may exceed their raw size slightly. Storage must
// not silently lower the existing native wire bound to the shard's8MiB cap.
const nativeSessionEvidenceStoredLimit = 17 << 20

// Bytes/SHA256 identify exact decoded original bytes, including their LF.
// Optional stored fields identify the actual gzip blob, separately. Plain
// references retain their original three-field JSON bytes and hex SHA spelling.
type nativeSessionFileReference struct {
	Path         string `json:"path"`
	Bytes        int    `json:"bytes"`
	SHA256       string `json:"sha256"`
	Encoding     string `json:"encoding,omitempty"`
	StoredBytes  int64  `json:"stored_bytes,omitempty"`
	StoredSHA256 string `json:"stored_sha256,omitempty"`
}

func nativeSessionReference(path string, raw []byte) nativeSessionFileReference {
	sum := sha256.Sum256(raw)
	return nativeSessionFileReference{Path: path, Bytes: len(raw), SHA256: hex.EncodeToString(sum[:])}
}

func (p nativeSessionEvidenceStorage) validate() error {
	if p != nativeSessionEvidencePlain && p != nativeSessionEvidenceGzip {
		return fmt.Errorf("unsupported native session evidence storage %q", p)
	}
	return nil
}

type nativeSessionPackedBuffer struct{ bytes.Buffer }

func (b *nativeSessionPackedBuffer) Write(raw []byte) (int, error) {
	if len(raw) > nativeSessionEvidenceStoredLimit-b.Len() {
		return 0, fmt.Errorf("compressed native session evidence exceeds storage bound")
	}
	return b.Buffer.Write(raw)
}

// Same bounded single-member/CRC/trailing/expansion rules as decodeCIShardGzip,
// with storage limits belonging to this separate16MiB session wire profile.
func decodeNativeSessionGzip(stored []byte, limit int64) ([]byte, error) {
	if len(stored) == 0 || len(stored) > nativeSessionEvidenceStoredLimit || limit <= 0 || limit > nativeSessionEvidenceDecodedLimit {
		return nil, fmt.Errorf("bounded compressed native session evidence required")
	}
	input := bytes.NewReader(stored)
	source, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}
	source.Multistream(false)
	raw, readErr := io.ReadAll(io.LimitReader(source, limit+1))
	err = errors.Join(readErr, source.Close())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || int64(len(raw)) > limit || input.Len() != 0 {
		return nil, fmt.Errorf("oversized, empty, trailing or multiple-member native session gzip")
	}
	return raw, nil
}

func nativeSessionWritePacked(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	return errors.Join(writeErr, file.Sync(), file.Close())
}

func writeNativeSessionStoredEvidence(path string, raw []byte, profile nativeSessionEvidenceStorage) (nativeSessionFileReference, error) {
	var zero nativeSessionFileReference
	if err := profile.validate(); err != nil {
		return zero, err
	}
	if len(raw) == 0 || len(raw) > nativeSessionEvidenceDecodedLimit {
		return zero, fmt.Errorf("bounded nonempty original native session bytes required")
	}
	if profile == nativeSessionEvidencePlain {
		if err := nativeSessionCLIWriteNew(path, raw); err != nil {
			return zero, err
		}
		return nativeSessionReference(path, raw), nil
	}
	var packed nativeSessionPackedBuffer
	writer, err := gzip.NewWriterLevel(&packed, gzip.BestSpeed)
	if err != nil {
		return zero, err
	}
	_, writeErr := writer.Write(raw)
	if err := errors.Join(writeErr, writer.Close()); err != nil {
		return zero, err
	}
	path += ".gz"
	if err := nativeSessionWritePacked(path, packed.Bytes()); err != nil {
		return zero, err
	}
	// Verify actual persisted bytes, not just the writer's in-memory buffer.
	stored, err := readNativeSessionEvidenceFile(path, nativeSessionEvidenceStoredLimit)
	if err != nil {
		return zero, err
	}
	decoded, err := decodeNativeSessionGzip(stored, int64(len(raw)))
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(raw, decoded) {
		return zero, fmt.Errorf("native session gzip changed original bytes")
	}
	ref := nativeSessionReference(path, raw)
	ref.Encoding, ref.StoredBytes = "gzip", int64(len(stored))
	ref.StoredSHA256 = nativeSessionReference(path, stored).SHA256
	return ref, nil
}

func nativeSessionLogicalPath(ref nativeSessionFileReference) (string, error) {
	if !filepath.IsAbs(ref.Path) || filepath.Clean(ref.Path) != ref.Path {
		return "", fmt.Errorf("canonical absolute native session evidence path required")
	}
	switch ref.Encoding {
	case "":
		if strings.HasSuffix(ref.Path, ".gz") || ref.StoredBytes != 0 || ref.StoredSHA256 != "" {
			return "", fmt.Errorf("plain native session reference has compressed metadata")
		}
		return ref.Path, nil
	case "gzip":
		if !strings.HasSuffix(ref.Path, ".gz") || ref.StoredBytes <= 0 || ref.StoredBytes > nativeSessionEvidenceStoredLimit || ref.StoredSHA256 == "" {
			return "", fmt.Errorf("bounded gzip native session reference required")
		}
		return strings.TrimSuffix(ref.Path, ".gz"), nil
	default:
		return "", fmt.Errorf("unsupported native session reference encoding")
	}
}

// Reading a supplied reference never learns a new identity from the filesystem.
func readNativeSessionStoredEvidence(ref nativeSessionFileReference) ([]byte, error) {
	if _, err := nativeSessionLogicalPath(ref); err != nil {
		return nil, err
	}
	if ref.Bytes <= 0 || ref.Bytes > nativeSessionEvidenceDecodedLimit {
		return nil, fmt.Errorf("bounded original native session reference required")
	}
	limit := int64(nativeSessionEvidenceDecodedLimit)
	if ref.Encoding == "gzip" {
		limit = nativeSessionEvidenceStoredLimit
	}
	stored, err := readNativeSessionEvidenceFile(ref.Path, limit)
	if err != nil {
		return nil, err
	}
	raw := stored
	if ref.Encoding == "gzip" {
		if int64(len(stored)) != ref.StoredBytes || nativeSessionReference(ref.Path, stored).SHA256 != ref.StoredSHA256 {
			return nil, fmt.Errorf("native session stored gzip bytes differ from reference")
		}
		raw, err = decodeNativeSessionGzip(stored, int64(ref.Bytes))
		if err != nil {
			return nil, err
		}
	}
	if len(raw) != ref.Bytes || nativeSessionReference(ref.Path, raw).SHA256 != ref.SHA256 {
		return nil, fmt.Errorf("native session original decoded bytes differ from reference")
	}
	return raw, nil
}
