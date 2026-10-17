// Copyright 2025-2026 mrveiss
// SPDX-License-Identifier: Apache-2.0
// Independent Go reference for resolve_line and LineResolution.reason in
// mrveiss/AutoBot-AI, revision a9c47e296907f1e5f681c38c05146cc8d3c2e7a7,
// autobot_shared/ai_lines.py. The original source, LICENSE and NOTICE are in
// upstream/. No Gooo candidate source or output is used as an oracle.
//
// Future invocation: pinned-go run /absolute/path/oracle.go < inputs.json
// Input is an array of nine-scalar arrays, not the corpus envelope. The corpus
// has 192 branch-state combinations plus 12 text edges; this is not exhaustive
// coverage of Text. This reference neither reads a store nor authenticates facts.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"unicode/utf8"
)

const (
	maxInputBytes  = 1 << 20
	maxOutputBytes = 4 << 20
	maxRows        = 4096
	maxTextBytes   = 1024
)

type facts struct {
	key             string
	storedPresent   bool
	storedValue     string
	mutating        bool
	baselinePresent bool
	baselineValue   string
	role            bool
	unavailable     bool
	scopeMissing    bool
}

type resolution struct {
	Key           string `json:"key"`
	Line          string `json:"line"`
	Source        string `json:"source"`
	StoredPresent bool   `json:"stored_present"`
	StoredValue   string `json:"stored_value"`
	FloorPresent  bool   `json:"floor_present"`
	FloorValue    string `json:"floor_value"`
	Reason        string `json:"reason"`
}

var invalid = errors.New("invalid input")

// This is an independently written priority decision over supplied facts. It
// preserves None versus empty Text through explicit presence flags.
func resolve(f facts) resolution {
	r := resolution{Key: f.key}
	switch {
	case f.mutating && f.scopeMissing:
		r.Line, r.Source = "never", "no_verified_company"
	case f.unavailable:
		r.Line, r.Source = "never", "store_unavailable"
	case f.storedPresent && f.storedValue == "never":
		r.Line, r.Source = "never", "stored"
		r.StoredPresent, r.StoredValue = true, "never"
	default:
		r.StoredPresent, r.StoredValue = f.storedPresent, f.storedValue
		r.FloorPresent, r.FloorValue = f.baselinePresent, f.baselineValue
		if f.storedPresent {
			r.Line, r.Source = f.storedValue, "stored"
		} else if f.mutating {
			r.Line, r.Source = "draft_then_send", "default"
		} else {
			r.Line, r.Source = "autonomous", "unmanaged"
		}
		if r.Line == "autonomous" {
			switch {
			case f.baselinePresent && f.baselineValue != "":
				r.Line, r.Source = "draft_then_send", "baseline_floor"
			case f.role:
				r.Line, r.Source = "draft_then_send", "role_floor"
				r.FloorPresent, r.FloorValue = false, ""
			}
		}
	}
	switch r.Source {
	case "no_verified_company":
		r.Reason = "no verified company or user scopes this action, so it cannot be allowed"
	case "store_unavailable":
		r.Reason = "the line store could not be read, so the action fails closed"
	case "baseline_floor":
		r.Reason = "approval baseline category '" + r.FloorValue + "' can never be autonomous"
	case "role_floor":
		r.Reason = "the agent role's policy requires approval for this risk"
	case "default":
		r.Reason = "no line is set for this action; an unset line counts as line 2"
	case "unmanaged":
		r.Reason = "no line is set and the action does not change anything"
	case "stored":
		switch r.Line {
		case "autonomous":
			r.Reason = "line 1 is set for this action"
		case "draft_then_send":
			r.Reason = "line 2 is set for this action"
		case "never":
			r.Reason = "line 3 is set for this action"
		}
	}
	return r
}

func parseFacts(raw json.RawMessage) (facts, error) {
	var f facts
	var a []json.RawMessage
	if err := json.Unmarshal(raw, &a); err != nil || len(a) != 9 {
		return f, invalid
	}
	texts := map[int]*string{0: &f.key, 2: &f.storedValue, 5: &f.baselineValue}
	for i, dst := range texts {
		v := bytes.TrimSpace(a[i])
		if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, dst) != nil ||
			!utf8.ValidString(*dst) || len(*dst) > maxTextBytes {
			return facts{}, invalid
		}
	}
	bools := map[int]*bool{1: &f.storedPresent, 3: &f.mutating, 4: &f.baselinePresent,
		6: &f.role, 7: &f.unavailable, 8: &f.scopeMissing}
	for i, dst := range bools {
		switch string(bytes.TrimSpace(a[i])) {
		case "true":
			*dst = true
		case "false":
			*dst = false
		default:
			return facts{}, invalid
		}
	}
	if f.storedPresent {
		switch f.storedValue {
		case "autonomous", "draft_then_send", "never":
		default:
			return facts{}, invalid
		}
	} else if f.storedValue != "" {
		return facts{}, invalid
	}
	if !f.baselinePresent && f.baselineValue != "" {
		return facts{}, invalid
	}
	return f, nil
}

func hex4(b []byte) (uint16, bool) {
	if len(b) < 4 {
		return 0, false
	}
	var n uint16
	for _, c := range b[:4] {
		n <<= 4
		switch {
		case c >= '0' && c <= '9':
			n += uint16(c - '0')
		case c >= 'a' && c <= 'f':
			n += uint16(c-'a') + 10
		case c >= 'A' && c <= 'F':
			n += uint16(c-'A') + 10
		default:
			return 0, false
		}
	}
	return n, true
}

// encoding/json replaces malformed UTF-8 and unpaired escaped surrogates. Reject
// them before decoding so a malformed Text never becomes a different valid Text.
func scalarJSON(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	inside := false
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			return false
		}
		if b[i] != 'u' {
			continue
		}
		u, ok := hex4(b[i+1:])
		if !ok {
			return false
		}
		i += 4
		if u >= 0xdc00 && u <= 0xdfff {
			return false
		}
		if u >= 0xd800 && u <= 0xdbff {
			if len(b)-(i+1) < 6 || b[i+1] != '\\' || b[i+2] != 'u' {
				return false
			}
			lo, valid := hex4(b[i+3:])
			if !valid || lo < 0xdc00 || lo > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return !inside
}

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > maxOutputBytes-b.Len() {
		return 0, invalid
	}
	return b.Buffer.Write(p)
}

func evaluate(data []byte) ([]byte, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '[' || !scalarJSON(data) {
		return nil, invalid
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil || len(rows) > maxRows {
		return nil, invalid
	}
	answers := make([]resolution, 0, len(rows))
	for _, raw := range rows {
		f, err := parseFacts(raw)
		if err != nil {
			return nil, err
		}
		answers = append(answers, resolve(f))
	}
	var buf cappedBuffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(answers); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	if len(os.Args) != 1 {
		_, _ = io.WriteString(os.Stderr, "INVALID_INPUT\n")
		os.Exit(2)
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxInputBytes+1))
	if err != nil || len(data) > maxInputBytes {
		_, _ = io.WriteString(os.Stderr, "INVALID_INPUT\n")
		os.Exit(2)
	}
	output, err := evaluate(data)
	if err != nil {
		_, _ = io.WriteString(os.Stderr, "INVALID_INPUT\n")
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(output); err != nil {
		_, _ = io.WriteString(os.Stderr, "OUTPUT_FAILED\n")
		os.Exit(2)
	}
}
