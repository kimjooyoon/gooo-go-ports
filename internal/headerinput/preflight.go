// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// Unicode-escape scanning adapts cmd/check-redelivery-policy/wire.go from
// immutable gooo-go-ports 807adb4fc9784aaf252fa1e2a43950cd0f730ff3.
// Package headerinput validates one external request before any Gooo call.
// It does not decide whether a header is valid or compute HeaderFacts.
package headerinput

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

const MaxTextBytes = 1024
const MaxRawBytes = 16 << 10

var (
	ErrRawTooLarge       = errors.New("HEADER_TRANSPORT_RAW_TOO_LARGE")
	ErrInvalidUTF8       = errors.New("HEADER_TRANSPORT_INVALID_UTF8")
	ErrMalformedJSON     = errors.New("HEADER_TRANSPORT_MALFORMED_JSON")
	ErrUnpairedSurrogate = errors.New("HEADER_TRANSPORT_UNPAIRED_SURROGATE")
	ErrObjectRequired    = errors.New("HEADER_TRANSPORT_OBJECT_REQUIRED")
	ErrUnknownField      = errors.New("HEADER_TRANSPORT_UNKNOWN_FIELD")
	ErrDuplicateField    = errors.New("HEADER_TRANSPORT_DUPLICATE_FIELD")
	ErrTextRequired      = errors.New("HEADER_TRANSPORT_TEXT_REQUIRED")
	ErrMissingField      = errors.New("HEADER_TRANSPORT_MISSING_FIELD")
	ErrTextTooLarge      = errors.New("HEADER_TRANSPORT_TEXT_TOO_LARGE")
)

// Request contains decoded Text, not raw JSON spelling or diagnostic results.
// External "name" maps to Name and Report.input0; external "value" maps to
// ValueText and Report.input1. The unchanged Gooo result calls its echo "value".
// Report's current Gooo declaration has two unnamed Text inputs; ValueText is
// only a host identifier. No named port or Gooo source is renamed here.
type Request struct {
	Name      string
	ValueText string
}

// Prepare is the host handoff boundary. A refusal returns no trusted document
// and does not call invoke. A successful callback receives its own byte copy;
// it may launch Gooo, but this package neither launches it nor attests a result.
// The command-line adapter passes nil and emits the document on stdout only.
func Prepare(reader io.Reader, invoke func([]byte) error) ([]byte, error) {
	return preparePair(reader, invoke, "name", "value", "app/header-field-facts:Report")
}

// PrepareIfMatch validates exactly two selected caller-supplied Texts before
// binding. It does not select headers, validate their grammar or decide a result.
func PrepareIfMatch(reader io.Reader, invoke func([]byte) error) ([]byte, error) {
	return preparePair(reader, invoke, "if_match", "current_etag", "go/net/http/precondition:IfMatchCondition")
}

func preparePair(reader io.Reader, invoke func([]byte) error, first, second, entry string) ([]byte, error) {
	request, err := readPair(reader, first, second)
	if err != nil {
		return nil, err
	}
	// The pinned839 DecodeCompositionInputs wire contract has top-level inputs,
	// not a finite-cases wrapper: internal/bodyexecution/composition_observation.go
	// at83928fa4538245abe33b08549da47f8cbdc50ed2, SHA256 cbf56139bad4b9197a0c8219e240e13941e4bcd28830588af84c6f8f4b20e6bf.
	document := struct {
		Schema string              `json:"schema"`
		Inputs []map[string]string `json:"inputs"`
	}{Schema: "gooo/body-composition-inputs/v1", Inputs: []map[string]string{
		{entry + ".input0": request.Name, entry + ".input1": request.ValueText},
	}}
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode trusted header inputs: %w", err)
	}
	raw = append(raw, '\n')
	if invoke != nil {
		if err := invoke(append([]byte(nil), raw...)); err != nil {
			return nil, fmt.Errorf("header input handoff: %w", err)
		}
	}
	return raw, nil
}

// Read bounds bytes before decoding. Any read/decode error must return before
// launching Gooo; an error is not a false validity value or HeaderFacts result.
func Read(reader io.Reader) (Request, error) {
	return readPair(reader, "name", "value")
}

func readPair(reader io.Reader, first, second string) (Request, error) {
	if reader == nil {
		return Request{}, fmt.Errorf("read header request: nil reader")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, MaxRawBytes+1))
	if err != nil {
		return Request{}, fmt.Errorf("read header request: %w", err)
	}
	return decodePair(raw, first, second)
}

// Decode accepts exactly one object with exactly the decoded keys name/value.
// Empty strings and JSON-escaped NUL/CR/LF are valid transport Texts, regardless
// of the later header predicates. Every refusal returns a zero Request.
func Decode(raw []byte) (Request, error) {
	return decodePair(raw, "name", "value")
}

// Both fixed callers share the same original JSON/Unicode/duplicate validator.
// The key pair selects a transport, not a new semantic or canonical ABI.
func decodePair(raw []byte, first, second string) (Request, error) {
	if len(raw) > MaxRawBytes {
		return Request{}, ErrRawTooLarge
	}
	if !utf8.Valid(raw) {
		return Request{}, ErrInvalidUTF8
	}
	if !json.Valid(raw) {
		return Request{}, ErrMalformedJSON
	}
	// encoding/json can replace lone escaped surrogates with U+FFFD. Inspect
	// original string escapes before decoding; an intentional U+FFFD is valid.
	if err := rejectUnpairedSurrogates(raw); err != nil {
		return Request{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return Request{}, ErrMalformedJSON
	}
	if token != json.Delim('{') {
		return Request{}, ErrObjectRequired
	}
	var request Request
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Request{}, ErrMalformedJSON
		}
		key, ok := token.(string)
		if !ok {
			return Request{}, ErrMalformedJSON
		}
		if key != first && key != second {
			return Request{}, ErrUnknownField
		}
		if seen[key] {
			return Request{}, ErrDuplicateField
		}
		seen[key] = true
		token, err = decoder.Token()
		if err != nil {
			return Request{}, ErrMalformedJSON
		}
		text, ok := token.(string)
		if !ok {
			return Request{}, ErrTextRequired
		}
		if !utf8.ValidString(text) {
			return Request{}, ErrInvalidUTF8
		}
		if len(text) > MaxTextBytes {
			return Request{}, ErrTextTooLarge
		}
		if key == first {
			request.Name = text
		} else {
			request.ValueText = text
		}
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return Request{}, ErrMalformedJSON
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Request{}, ErrMalformedJSON
	}
	if !seen[first] || !seen[second] {
		return Request{}, ErrMissingField
	}
	return request, nil
}

// Input is already valid JSON/UTF8. Skip every ordinary escaped byte so that
// an escaped backslash followed by uD800 remains literal text. A high surrogate
// requires an immediately adjacent low-surrogate escape; a lone low is refused.
func rejectUnpairedSurrogates(raw []byte) error {
	inside := false
	for at := 0; at < len(raw); at++ {
		if raw[at] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[at] != '\\' {
			continue
		}
		if raw[at+1] != 'u' {
			at++
			continue
		}
		first, err := strconv.ParseUint(string(raw[at+2:at+6]), 16, 16)
		if err != nil {
			return ErrMalformedJSON
		}
		if first >= 0xD800 && first <= 0xDBFF {
			if at+12 > len(raw) || raw[at+6] != '\\' || raw[at+7] != 'u' {
				return ErrUnpairedSurrogate
			}
			second, err := strconv.ParseUint(string(raw[at+8:at+12]), 16, 16)
			if err != nil || second < 0xDC00 || second > 0xDFFF {
				return ErrUnpairedSurrogate
			}
			at += 11
		} else if first >= 0xDC00 && first <= 0xDFFF {
			return ErrUnpairedSurrogate
		} else {
			at += 5
		}
	}
	return nil
}
