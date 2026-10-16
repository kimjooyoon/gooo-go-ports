// Copyright 2026 Gooo Go Ports contributors. All rights reserved.
// Use of this source code is governed by the repository BSD-style LICENSE.
// JSON token and surrogate scanning adapt gooo-go-ports internal/headerinput
// at 3fb196f2f3c756ebb366f55f8280e134dbbe70da, under its BSD-style LICENSE.
// Package headeradditioninput checks wire structure. It does not authenticate facts,
// compute policy, authorize a change or launch Gooo.
package headeradditioninput

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
const MaxRawBytes = 32 << 10
const Entry = "app/diagnostic-header-addition:AssessAddition"

var (
	ErrRawTooLarge       = errors.New("ADDITION_TRANSPORT_RAW_TOO_LARGE")
	ErrInvalidUTF8       = errors.New("ADDITION_TRANSPORT_INVALID_UTF8")
	ErrMalformedJSON     = errors.New("ADDITION_TRANSPORT_MALFORMED_JSON")
	ErrUnpairedSurrogate = errors.New("ADDITION_TRANSPORT_UNPAIRED_SURROGATE")
	ErrObjectRequired    = errors.New("ADDITION_TRANSPORT_OBJECT_REQUIRED")
	ErrUnknownField      = errors.New("ADDITION_TRANSPORT_UNKNOWN_FIELD")
	ErrDuplicateField    = errors.New("ADDITION_TRANSPORT_DUPLICATE_FIELD")
	ErrMissingField      = errors.New("ADDITION_TRANSPORT_MISSING_FIELD")
	ErrTextRequired      = errors.New("ADDITION_TRANSPORT_TEXT_REQUIRED")
	ErrTextTooLarge      = errors.New("ADDITION_TRANSPORT_TEXT_TOO_LARGE")
	ErrIntegerRequired   = errors.New("ADDITION_TRANSPORT_INT64_REQUIRED")
	ErrBooleanRequired   = errors.New("ADDITION_TRANSPORT_BOOLEAN_REQUIRED")
)

// Facts must be assembled by a trusted host from its own request and policy
// snapshot. Decoding an untrusted person's claims does not make them true.
// Every field is mandatory; Decode checks presence before returning any value.
type Facts struct {
	ContentLengthOccurrences      int64  `json:"content_length_occurrences"`
	ObservedBodyBytes             int64  `json:"observed_body_bytes"`
	MaximumPayloadBytes           int64  `json:"maximum_payload_bytes"`
	TransferEncodingPresent       bool   `json:"transfer_encoding_present"`
	BodyObservationBoundToRequest bool   `json:"body_observation_bound_to_request"`
	CurrentAccess                 bool   `json:"current_access"`
	CandidateNameOccurrences      int64  `json:"candidate_name_occurrences"`
	HeaderSetBoundToRequest       bool   `json:"header_set_bound_to_request"`
	PolicyRevisionCurrent         bool   `json:"policy_revision_current"`
	AllowedHeaderName             string `json:"allowed_header_name"`
	ExpectedRequestRevision       int64  `json:"expected_request_revision"`
	ObservedRequestRevision       int64  `json:"observed_request_revision"`
}

type Request struct {
	Name           string `json:"name"`
	Value          string `json:"value"`
	DeclaredLength string `json:"declared_length"`
	Facts          Facts  `json:"facts"`
}

// Read consumes at most MaxRawBytes+1 bytes. Even a complete JSON object is
// refused if its reader also reports an error. No partial Request is returned.
func Read(reader io.Reader) (Request, error) {
	if reader == nil {
		return Request{}, fmt.Errorf("read addition request: nil reader")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, MaxRawBytes+1))
	if err != nil {
		return Request{}, fmt.Errorf("read addition request: %w", err)
	}
	return Decode(raw)
}

// Decode requires one exact object with four top-level keys and all twelve
// facts. It validates transport only. Negative counts, false binding flags,
// invalid header bytes and exhausted revisions remain valid policy inputs.
func Decode(raw []byte) (Request, error) {
	if len(raw) > MaxRawBytes {
		return Request{}, ErrRawTooLarge
	}
	if !utf8.Valid(raw) {
		return Request{}, ErrInvalidUTF8
	}
	if !json.Valid(raw) {
		return Request{}, ErrMalformedJSON
	}
	if err := rejectUnpairedSurrogates(raw); err != nil {
		return Request{}, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var r Request
	err := object(d, map[string]func() error{
		"name":            func() error { return text(d, &r.Name) },
		"value":           func() error { return text(d, &r.Value) },
		"declared_length": func() error { return text(d, &r.DeclaredLength) },
		"facts": func() error {
			return object(d, map[string]func() error{
				"content_length_occurrences":        func() error { return integer(d, &r.Facts.ContentLengthOccurrences) },
				"observed_body_bytes":               func() error { return integer(d, &r.Facts.ObservedBodyBytes) },
				"maximum_payload_bytes":             func() error { return integer(d, &r.Facts.MaximumPayloadBytes) },
				"transfer_encoding_present":         func() error { return boolean(d, &r.Facts.TransferEncodingPresent) },
				"body_observation_bound_to_request": func() error { return boolean(d, &r.Facts.BodyObservationBoundToRequest) },
				"current_access":                    func() error { return boolean(d, &r.Facts.CurrentAccess) },
				"candidate_name_occurrences":        func() error { return integer(d, &r.Facts.CandidateNameOccurrences) },
				"header_set_bound_to_request":       func() error { return boolean(d, &r.Facts.HeaderSetBoundToRequest) },
				"policy_revision_current":           func() error { return boolean(d, &r.Facts.PolicyRevisionCurrent) },
				"allowed_header_name":               func() error { return text(d, &r.Facts.AllowedHeaderName) },
				"expected_request_revision":         func() error { return integer(d, &r.Facts.ExpectedRequestRevision) },
				"observed_request_revision":         func() error { return integer(d, &r.Facts.ObservedRequestRevision) },
			})
		},
	})
	if err != nil {
		return Request{}, err
	}
	if _, err := d.Token(); err != io.EOF {
		return Request{}, ErrMalformedJSON
	}
	return r, nil
}

func object(d *json.Decoder, fields map[string]func() error) error {
	token, err := d.Token()
	if err != nil {
		return ErrMalformedJSON
	}
	if token != json.Delim('{') {
		return ErrObjectRequired
	}
	seen := make(map[string]bool, len(fields))
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return ErrMalformedJSON
		}
		key, ok := token.(string)
		if !ok {
			return ErrMalformedJSON
		}
		decode, known := fields[key]
		if !known {
			return ErrUnknownField
		}
		if seen[key] {
			return ErrDuplicateField
		}
		seen[key] = true
		if err := decode(); err != nil {
			return err
		}
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return ErrMalformedJSON
	}
	if len(seen) != len(fields) {
		return ErrMissingField
	}
	return nil
}

func text(d *json.Decoder, target *string) error {
	token, err := d.Token()
	if err != nil {
		return ErrMalformedJSON
	}
	s, ok := token.(string)
	if !ok {
		return ErrTextRequired
	}
	if !utf8.ValidString(s) {
		return ErrInvalidUTF8
	}
	if len(s) > MaxTextBytes {
		return ErrTextTooLarge
	}
	*target = s
	return nil
}

func integer(d *json.Decoder, target *int64) error {
	token, err := d.Token()
	if err != nil {
		return ErrMalformedJSON
	}
	n, ok := token.(json.Number)
	if !ok {
		return ErrIntegerRequired
	}
	// ParseInt rejects fractions/exponents and preserves all signed64 values
	// without a float64 conversion. JSON-valid -0 denotes integer zero.
	v, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil {
		return ErrIntegerRequired
	}
	*target = v
	return nil
}

func boolean(d *json.Decoder, target *bool) error {
	token, err := d.Token()
	if err != nil {
		return ErrMalformedJSON
	}
	b, ok := token.(bool)
	if !ok {
		return ErrBooleanRequired
	}
	*target = b
	return nil
}

// Prepare emits only actual pinned839 input wire shape. It does not produce
// expected results, replace absent facts or compute an eligibility decision.
func Prepare(reader io.Reader) ([]byte, error) {
	r, err := Read(reader)
	if err != nil {
		return nil, err
	}
	doc := struct {
		Schema string           `json:"schema"`
		Inputs []map[string]any `json:"inputs"`
	}{"gooo/body-composition-inputs/v1", []map[string]any{{
		Entry + ".input0": r.Name, Entry + ".input1": r.Value,
		Entry + ".input2": r.DeclaredLength, Entry + ".input3": r.Facts,
	}}}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode addition inputs: %w", err)
	}
	return append(raw, '\n'), nil
}

// Input has already passed json.Valid and utf8.Valid. Scan original escapes
// before encoding/json can replace an unpaired surrogate with U+FFFD.
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
