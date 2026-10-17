// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// This command validates supplied scalar facts. It computes no policy decision
// and neither imports an oracle nor invokes Gooo or another executable.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"
)

const (
	maxRawBytes  = 1 << 20
	maxRows      = 128
	maxTextBytes = 1024
	maxWireBytes = 32 << 10
	entry        = "gooo/autobot/ai-lines:ResolveLine"
)

// prepare accepts one nonempty array of nine-scalar rows, in declared input
// order. None is represented by false presence and canonical empty Text.
// All rows are checked before any encoded document is returned.
func prepare(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("input reader is required")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxRawBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read resolver facts: %w", err)
	}
	if len(raw) > maxRawBytes {
		return nil, fmt.Errorf("resolver facts exceed 1 MiB")
	}
	if !utf8.Valid(raw) || !json.Valid(raw) || !scalarEscapes(raw) {
		return nil, fmt.Errorf("resolver facts require one valid Unicode-scalar JSON document")
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, fmt.Errorf("resolver facts require an array of rows")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 || len(rows) > maxRows {
		return nil, fmt.Errorf("resolver facts require 1..128 rows")
	}
	inputs := make([]map[string]any, 0, len(rows))
	for index, row := range rows {
		var values []json.RawMessage
		if err := json.Unmarshal(row, &values); err != nil || len(values) != 9 {
			return nil, fmt.Errorf("row %d requires exactly nine scalar values", index)
		}
		texts := make(map[int]string, 3)
		bools := make(map[int]bool, 6)
		bound := make(map[string]any, 9)
		for port, value := range values {
			value = bytes.TrimSpace(value)
			key := entry + ".input" + strconv.Itoa(port)
			if port == 0 || port == 2 || port == 5 {
				var text string
				if len(value) == 0 || value[0] != '"' || json.Unmarshal(value, &text) != nil || len(text) > maxTextBytes {
					return nil, fmt.Errorf("row %d input%d requires Text of at most 1024 UTF8 bytes", index, port)
				}
				texts[port], bound[key] = text, text
			} else {
				var flag bool
				switch string(value) {
				case "true":
					flag = true
				case "false":
					flag = false
				default:
					return nil, fmt.Errorf("row %d input%d requires Boolean", index, port)
				}
				bools[port], bound[key] = flag, flag
			}
		}
		if bools[1] {
			switch texts[2] {
			case "autonomous", "draft_then_send", "never":
			default:
				return nil, fmt.Errorf("row %d has an invalid stored line", index)
			}
		} else if texts[2] != "" {
			return nil, fmt.Errorf("row %d absent stored line requires empty Text", index)
		}
		if !bools[4] && texts[5] != "" {
			return nil, fmt.Errorf("row %d absent baseline requires empty Text", index)
		}
		inputs = append(inputs, bound)
	}
	document := struct {
		Schema string           `json:"schema"`
		Inputs []map[string]any `json:"inputs"`
	}{Schema: "gooo/body-composition-inputs/v1", Inputs: inputs}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode resolver inputs: %w", err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) > maxWireBytes {
		return nil, fmt.Errorf("encoded resolver inputs exceed 32768 bytes")
	}
	return encoded, nil
}

// JSON syntax and raw UTF8 are checked first. encoding/json otherwise repairs
// isolated escaped surrogates. Skip ordinary escapes so literal "\\ud800"
// remains valid Text; require an adjacent low escape after each high surrogate.
func scalarEscapes(raw []byte) bool {
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
			return false
		}
		if first >= 0xD800 && first <= 0xDBFF {
			if at+12 > len(raw) || raw[at+6] != '\\' || raw[at+7] != 'u' {
				return false
			}
			second, err := strconv.ParseUint(string(raw[at+8:at+12]), 16, 16)
			if err != nil || second < 0xDC00 || second > 0xDFFF {
				return false
			}
			at += 11
		} else if first >= 0xDC00 && first <= 0xDFFF {
			return false
		} else {
			at += 5
		}
	}
	return true
}

func run(args []string, reader io.Reader, writer io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: autobot-line-input < facts.json > inputs.json")
	}
	if writer == nil {
		return fmt.Errorf("output writer is required")
	}
	raw, err := prepare(reader)
	if err != nil {
		return err
	}
	n, err := writer.Write(raw)
	if err != nil {
		return fmt.Errorf("write resolver inputs: %w", err)
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
