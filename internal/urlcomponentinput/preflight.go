// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// Unicode-escape scanning follows internal/headerinput at immutable
// gooo-go-ports 146e07cca2f6dde150e647c8946bd68829adc4b6.
// Package urlcomponentinput validates original Text before a Gooo invocation.
// It does not encode URL components or authorize HTTP effects.
package urlcomponentinput

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

const (
	MaxTextBytes      = 341
	MaxRawBytes       = 16 << 10
	MaxCorpusRawBytes = 1 << 20
	MaxRows           = 128
	Entry             = "app/url-context-facts:Report"
	Schema            = "gooo/body-composition-inputs/v1"
)

var (
	ErrRawTooLarge       = errors.New("URL_COMPONENT_TRANSPORT_RAW_TOO_LARGE")
	ErrInvalidUTF8       = errors.New("URL_COMPONENT_TRANSPORT_INVALID_UTF8")
	ErrMalformedJSON     = errors.New("URL_COMPONENT_TRANSPORT_MALFORMED_JSON")
	ErrUnpairedSurrogate = errors.New("URL_COMPONENT_TRANSPORT_UNPAIRED_SURROGATE")
	ErrObjectRequired    = errors.New("URL_COMPONENT_TRANSPORT_OBJECT_REQUIRED")
	ErrUnknownField      = errors.New("URL_COMPONENT_TRANSPORT_UNKNOWN_FIELD")
	ErrDuplicateField    = errors.New("URL_COMPONENT_TRANSPORT_DUPLICATE_FIELD")
	ErrTextRequired      = errors.New("URL_COMPONENT_TRANSPORT_TEXT_REQUIRED")
	ErrMissingField      = errors.New("URL_COMPONENT_TRANSPORT_MISSING_FIELD")
	ErrTextTooLarge      = errors.New("URL_COMPONENT_TRANSPORT_TEXT_TOO_LARGE")
	ErrInputSchema       = errors.New("URL_COMPONENT_INPUT_SCHEMA")
	ErrInputRows         = errors.New("URL_COMPONENT_INPUT_ROW_BOUND")
	ErrInputField        = errors.New("URL_COMPONENT_INPUT_FIELD")
)

type Request struct {
	Text string
}

// Prepare emits an input-only document, never expected outputs. A refusal calls
// no handoff and returns no document. The optional callback gets its own copy.
func Prepare(reader io.Reader, invoke func([]byte) error) ([]byte, error) {
	request, err := Read(reader)
	if err != nil {
		return nil, err
	}
	document := struct {
		Schema string              `json:"schema"`
		Inputs []map[string]string `json:"inputs"`
	}{Schema: Schema, Inputs: []map[string]string{{Entry: request.Text}}}
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode URL component inputs: %w", err)
	}
	raw = append(raw, '\n')
	if invoke != nil {
		if err := invoke(append([]byte(nil), raw...)); err != nil {
			return nil, fmt.Errorf("URL component input handoff: %w", err)
		}
	}
	return raw, nil
}

func Read(reader io.Reader) (Request, error) {
	raw, err := readBounded(reader, MaxRawBytes)
	if err != nil {
		return Request{}, err
	}
	return Decode(raw)
}

// Decode accepts exactly one object with one decoded key, text. It preserves
// spaces, NUL, percent spellings and all other admitted Text bytes unchanged.
func Decode(raw []byte) (Request, error) {
	value, err := strictJSON(raw, MaxRawBytes)
	if err != nil {
		return Request{}, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Request{}, ErrObjectRequired
	}
	for name := range object {
		if name != "text" {
			return Request{}, ErrUnknownField
		}
	}
	value, present := object["text"]
	if !present {
		return Request{}, ErrMissingField
	}
	text, err := checkedText(value)
	if err != nil {
		return Request{}, err
	}
	return Request{Text: text}, nil
}

// ReadCorpus accepts only the public Report's input-only schema and 1..128
// original Text rows. This is an oracle input bound, not native batch admission.
func ReadCorpus(reader io.Reader) ([]string, error) {
	raw, err := readBounded(reader, MaxCorpusRawBytes)
	if err != nil {
		return nil, err
	}
	value, err := strictJSON(raw, MaxCorpusRawBytes)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok || len(object) != 2 || object["schema"] != Schema {
		return nil, ErrInputSchema
	}
	rows, ok := object["inputs"].([]any)
	if !ok || len(rows) < 1 || len(rows) > MaxRows {
		return nil, ErrInputRows
	}
	texts := make([]string, 0, len(rows))
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 1 {
			return nil, ErrInputField
		}
		value, present := row[Entry]
		if !present {
			return nil, ErrInputField
		}
		text, err := checkedText(value)
		if err != nil {
			return nil, err
		}
		texts = append(texts, text)
	}
	return texts, nil
}

func checkedText(value any) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", ErrTextRequired
	}
	if !utf8.ValidString(text) {
		return "", ErrInvalidUTF8
	}
	if len(text) > MaxTextBytes {
		return "", ErrTextTooLarge
	}
	return text, nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("URL component input reader is required")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read URL component input: %w", err)
	}
	if int64(len(raw)) > limit {
		return nil, ErrRawTooLarge
	}
	return raw, nil
}

func strictJSON(raw []byte, limit int) (any, error) {
	if len(raw) > limit {
		return nil, ErrRawTooLarge
	}
	if !utf8.Valid(raw) {
		return nil, ErrInvalidUTF8
	}
	if !json.Valid(raw) {
		return nil, ErrMalformedJSON
	}
	if err := rejectUnpairedSurrogates(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := jsonValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrMalformedJSON
	}
	return value, nil
}

// Only shallow fixed request/corpus shapes are admitted. A finite recursion
// bound also prevents deeply nested refused values from growing the host stack.
func jsonValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 8 {
		return nil, ErrMalformedJSON
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, ErrMalformedJSON
	}
	switch token {
	case json.Delim('{'):
		object := map[string]any{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return nil, ErrMalformedJSON
			}
			if _, present := object[name]; present {
				return nil, ErrDuplicateField
			}
			value, err := jsonValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
			return nil, ErrMalformedJSON
		}
		return object, nil
	case json.Delim('['):
		values := []any{}
		for decoder.More() {
			value, err := jsonValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
			return nil, ErrMalformedJSON
		}
		return values, nil
	default:
		return token, nil
	}
}

// Syntax is already checked. Skip ordinary escapes so literal \\ud800 stays
// text; only an adjacent escaped high/low surrogate pair represents a scalar.
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
