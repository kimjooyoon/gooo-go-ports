// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	recorded "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func planProblem(code string) error { return &problem{"INVALID_INPUT", "file_plan", code} }

// A file plan is a snapshot of explicitly named local files, not a filesystem
// transaction or evidence of how the collector obtained those files.
func readPlanFile(path string, limit int64) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, planProblem("FILE_READ_FAILED")
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return nil, planProblem("BOUNDED_REGULAR_FILE_REQUIRED")
	}
	// NONBLOCK prevents a FIFO swapped in after Stat from blocking the open.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, planProblem("FILE_READ_FAILED")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !samePlanFile(before, opened) {
		return nil, planProblem("FILE_CHANGED_OR_NOT_REGULAR")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	if readErr != nil || int64(len(raw)) > limit {
		return nil, planProblem("FILE_READ_OR_SIZE_FAILED")
	}
	after, err := f.Stat()
	if err != nil || !samePlanFile(opened, after) || int64(len(raw)) != after.Size() {
		return nil, planProblem("FILE_CHANGED_DURING_READ")
	}
	current, err := os.Stat(path)
	if err != nil || !samePlanFile(after, current) {
		return nil, planProblem("FILE_PATH_CHANGED_DURING_READ")
	}
	if err := f.Close(); err != nil {
		return nil, planProblem("FILE_CLOSE_FAILED")
	}
	return raw, nil
}

func samePlanFile(a, b os.FileInfo) bool {
	return b.Mode().IsRegular() && os.SameFile(a, b) && a.Size() == b.Size() &&
		a.Mode() == b.Mode() && a.ModTime().Equal(b.ModTime())
}

// Decode object keys explicitly: encoding/json struct matching is case
// insensitive and ordinary map decoding silently replaces duplicate keys.
func exactPlanObject(raw []byte, allowed ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, planProblem("EXACT_PLAN_OBJECT_REQUIRED")
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, planProblem("INVALID_PLAN_KEY")
		}
		known := false
		for _, name := range allowed {
			known = known || key == name
		}
		if !known {
			return nil, planProblem("UNKNOWN_PLAN_FIELD")
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, planProblem("DUPLICATE_PLAN_FIELD")
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, planProblem("INVALID_PLAN_FIELD")
		}
		fields[key] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(fields) != len(allowed) {
		return nil, planProblem("MISSING_PLAN_FIELD")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, planProblem("TRAILING_PLAN_JSON")
	}
	return fields, nil
}

func planString(raw []byte) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || !json.Valid(raw) {
		return "", planProblem("PLAN_STRING_REQUIRED")
	}
	// Unmarshal replaces unmatched UTF-16 surrogate escapes. Refuse them before
	// decoding so a path cannot silently name a different file.
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		unit := hexUnit(raw[i+1 : i+5])
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return "", planProblem("UNPAIRED_PLAN_SURROGATE")
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return "", planProblem("UNPAIRED_PLAN_SURROGATE")
			}
			low := hexUnit(raw[i+3 : i+7])
			if low < 0xdc00 || low > 0xdfff {
				return "", planProblem("UNPAIRED_PLAN_SURROGATE")
			}
			i += 6
		}
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", planProblem("PLAN_STRING_REQUIRED")
	}
	return value, nil
}

func hexUnit(raw []byte) uint16 {
	var result uint16
	for _, c := range raw {
		result <<= 4
		switch {
		case c >= '0' && c <= '9':
			result += uint16(c - '0')
		case c >= 'a' && c <= 'f':
			result += uint16(c-'a') + 10
		default:
			result += uint16(c-'A') + 10
		}
	}
	return result
}

func planPath(base string, raw []byte) (string, error) {
	name, err := planString(raw)
	if err != nil {
		return "", err
	}
	if name == "" || strings.ContainsRune(name, 0) {
		return "", planProblem("NONEMPTY_FILE_PATH_REQUIRED")
	}
	if filepath.IsAbs(name) {
		return name, nil
	}
	return filepath.Join(base, name), nil
}

func assembleFilePlan(name string) ([]byte, string, error) {
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, "", planProblem("PLAN_PATH_INVALID")
	}
	raw, err := readPlanFile(path, recorded.MaxInputBytes)
	if err != nil {
		return nil, "", err
	}
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return nil, "", planProblem("PLAN_JSON_SIZE_OR_ENCODING")
	}
	fields, err := exactPlanObject(raw, "previous_catalog_file", "collector_state", "pages")
	if err != nil {
		return nil, "", err
	}
	collector, err := planString(fields["collector_state"])
	if err != nil {
		return nil, "", err
	}
	if collector != "SUCCESS" && collector != "ERROR" && collector != "TIMEOUT" && collector != "CANCELLED" {
		return nil, "", planProblem("EXPLICIT_COLLECTOR_STATE_REQUIRED")
	}
	var pageFields []json.RawMessage
	if len(fields["pages"]) == 0 || fields["pages"][0] != '[' || json.Unmarshal(fields["pages"], &pageFields) != nil || len(pageFields) > recorded.MaxPages {
		return nil, "", planProblem("BOUNDED_PLAN_PAGES_REQUIRED")
	}
	previousPath, err := planPath(filepath.Dir(path), fields["previous_catalog_file"])
	if err != nil {
		return nil, "", err
	}
	previous, err := readPlanFile(previousPath, recorded.MaxInputBytes)
	if err != nil {
		return nil, "", err
	}
	pages := make([]refresh.SnapshotPage, 0, len(pageFields))
	for _, pageRaw := range pageFields {
		page, err := exactPlanObject(pageRaw, "request_file", "response_file")
		if err != nil {
			return nil, "", err
		}
		values := [2]string{}
		for i, key := range []string{"request_file", "response_file"} {
			framePath, err := planPath(filepath.Dir(path), page[key])
			if err != nil {
				return nil, "", err
			}
			frame, err := readPlanFile(framePath, recorded.MaxFrameBytes)
			if err != nil {
				return nil, "", err
			}
			if !utf8.Valid(frame) {
				return nil, "", planProblem("INVALID_FRAME_UTF8")
			}
			values[i] = string(frame)
		}
		pages = append(pages, refresh.SnapshotPage{RequestJSON: values[0], ResponseJSON: values[1]})
	}
	assembled, err := json.Marshal(struct {
		Previous  string                 `json:"previous_catalog_base64"`
		Collector string                 `json:"collector_state"`
		Pages     []refresh.SnapshotPage `json:"pages"`
	}{base64.StdEncoding.EncodeToString(previous), collector, pages})
	if err != nil {
		return nil, "", planProblem("PLAN_ASSEMBLY_FAILED")
	}
	if len(assembled) > recorded.MaxInputBytes {
		return nil, "", planProblem("ASSEMBLED_INPUT_TOO_LARGE")
	}
	// runCommand applies decodeInput to this exact envelope before creating any
	// registry. In particular, malformed inner JSON-RPC is never repaired here.
	sum := sha256.Sum256(raw)
	return assembled, "sha256:" + hex.EncodeToString(sum[:]), nil
}
