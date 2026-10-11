// Package jsonkeys checks decoded object key uniqueness without rewriting JSON.
package jsonkeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// RejectDuplicates compares decoded key names within each object. It
// leaves case, Unicode normalization, unknown fields, and original bytes alone.
func RejectDuplicates(raw []byte) error {
	// Retain encoding/json's syntax and nesting-depth limits before recursion.
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := inspectValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func inspectValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("invalid JSON object key")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := inspectValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := inspectValue(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = d.Token() // Syntax was checked above, including the closing delimiter.
	return err
}
