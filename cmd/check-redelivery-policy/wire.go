package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

func wireRefused(code string) error { return fmt.Errorf("WIRE_REFUSED: %s", code) }
func wireHex(raw []byte) (uint64, error) {
	if len(raw) != 4 {
		return 0, wireRefused("MALFORMED_JSON")
	}
	v, err := strconv.ParseUint(string(raw), 16, 16)
	if err != nil {
		return 0, wireRefused("MALFORMED_JSON")
	}
	return v, nil
}
func wireUnicode(raw []byte) error {
	if !utf8.Valid(raw) {
		return wireRefused("INVALID_RAW_UTF8")
	}
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			return wireRefused("MALFORMED_JSON")
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(raw) {
			return wireRefused("MALFORMED_JSON")
		}
		first, err := wireHex(raw[i+2 : i+6])
		if err != nil {
			return err
		}
		if first >= 0xD800 && first <= 0xDBFF {
			if i+12 > len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return wireRefused("INVALID_UNICODE_SCALAR")
			}
			second, err := wireHex(raw[i+8 : i+12])
			if err != nil {
				return err
			}
			if second < 0xDC00 || second > 0xDFFF {
				return wireRefused("INVALID_UNICODE_SCALAR")
			}
			i += 11
		} else if first >= 0xDC00 && first <= 0xDFFF {
			return wireRefused("INVALID_UNICODE_SCALAR")
		} else {
			i += 5
		}
	}
	return nil
}

// Inspect original tokens before any qualified-port translation or compiler
// launch. The walker preserves integer spelling and refuses decoded duplicates.
func wireJSON(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return nil, wireRefused("MALFORMED_JSON")
	}
	if err := wireUnicode(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(int) (any, error)
	visit = func(depth int) (any, error) {
		if depth > 64 {
			return nil, wireRefused("MALFORMED_JSON")
		}
		token, err := d.Token()
		if err != nil {
			return nil, wireRefused("MALFORMED_JSON")
		}
		switch token {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				token, err := d.Token()
				if err != nil {
					return nil, wireRefused("MALFORMED_JSON")
				}
				key, ok := token.(string)
				if !ok {
					return nil, wireRefused("MALFORMED_JSON")
				}
				if _, exists := m[key]; exists {
					return nil, wireRefused("DUPLICATE_FIELD")
				}
				v, err := visit(depth + 1)
				if err != nil {
					return nil, err
				}
				m[key] = v
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, wireRefused("MALFORMED_JSON")
			}
			return m, nil
		case json.Delim('['):
			a := []any{}
			for d.More() {
				v, err := visit(depth + 1)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, wireRefused("MALFORMED_JSON")
			}
			return a, nil
		case json.Delim('}'), json.Delim(']'):
			return nil, wireRefused("MALFORMED_JSON")
		default:
			return token, nil
		}
	}
	value, err := visit(0)
	if err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, wireRefused("TRAILING_JSON")
	}
	return value, nil
}
func checkWireScalar(v any, index int) error {
	switch fieldTypes[index] {
	case "string":
		s, ok := v.(string)
		if !ok {
			return wireRefused("TEXT_TYPE")
		}
		if !utf8.ValidString(s) {
			return wireRefused("INVALID_UNICODE_SCALAR")
		}
		if len(s) > 1024 {
			return wireRefused("TEXT_BYTE_BOUND")
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return wireRefused("BOOLEAN_TYPE")
		}
	case "int64":
		n, ok := v.(json.Number)
		if !ok {
			return wireRefused("INTEGER_TOKEN")
		}
		raw := n.String()
		start := 0
		if len(raw) > 0 && raw[0] == '-' {
			start = 1
		}
		if start == len(raw) {
			return wireRefused("INTEGER_TOKEN")
		}
		for _, b := range []byte(raw[start:]) {
			if b < '0' || b > '9' {
				return wireRefused("INTEGER_TOKEN")
			}
		}
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return wireRefused("INTEGER_RANGE")
		}
	}
	return nil
}
func checkNativeWire(raw []byte) ([]map[string]any, error) {
	v, err := wireJSON(raw)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, wireRefused("OBJECT_REQUIRED")
	}
	for name := range m {
		if name != "schema" && name != "inputs" {
			return nil, wireRefused("UNKNOWN_FIELD")
		}
	}
	if !keys(m, "schema", "inputs") {
		return nil, wireRefused("MISSING_FIELD")
	}
	if m["schema"] != "gooo/body-composition-inputs/v1" {
		return nil, wireRefused("MALFORMED_JSON")
	}
	inputs, ok := m["inputs"].([]any)
	if !ok || len(inputs) < 1 || len(inputs) > 8 {
		return nil, wireRefused("MALFORMED_JSON")
	}
	rows := make([]map[string]any, len(inputs))
	allowed := map[string]int{}
	for i := 0; i < 9; i++ {
		allowed[appPackage+":"+activityName+".input"+strconv.Itoa(i)] = i
	}
	for i, v := range inputs {
		input, ok := v.(map[string]any)
		if !ok {
			return nil, wireRefused("OBJECT_REQUIRED")
		}
		for name := range input {
			if _, known := allowed[name]; !known {
				return nil, wireRefused("UNKNOWN_FIELD")
			}
		}
		if len(input) != 9 {
			return nil, wireRefused("MISSING_FIELD")
		}
		row := map[string]any{}
		for j := 0; j < 9; j++ {
			key := appPackage + ":" + activityName + ".input" + strconv.Itoa(j)
			v, exists := input[key]
			if !exists {
				return nil, wireRefused("MISSING_FIELD")
			}
			if err := checkWireScalar(v, j); err != nil {
				return nil, err
			}
			row[fieldNames[j]] = v
		}
		rows[i] = row
	}
	return rows, nil
}
