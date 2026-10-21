// Package jsonkeys checks decoded object key uniqueness without rewriting JSON.
package jsonkeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// RejectDuplicates compares decoded key names within each object. It
// leaves case, Unicode normalization, unknown fields, and original bytes alone.
func RejectDuplicates(raw []byte) error {
	// Keep validation first: malformed JSON takes precedence over any earlier
	// duplicate, and encoding/json retains ownership of grammar and depth limits.
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON")
	}
	w := keyWalker{raw: raw}
	if err := w.value(); err != nil {
		return err
	}
	w.space()
	if w.pos != len(raw) {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

type keyWalker struct {
	raw []byte
	pos int
}

func (w *keyWalker) space() {
	for w.pos < len(w.raw) {
		switch w.raw[w.pos] {
		case ' ', '\t', '\n', '\r':
			w.pos++
		default:
			return
		}
	}
}

// quoted advances over a string already accepted by json.Valid. Values need
// no decoding; keys use the escaped result to choose the decoded-key slow path.
func (w *keyWalker) quoted() (start, end int, escaped bool) {
	start = w.pos
	w.pos++
	for {
		switch w.raw[w.pos] {
		case '"':
			w.pos++
			return start, w.pos, escaped
		case '\\':
			escaped = true
			w.pos += 2
		default:
			w.pos++
		}
	}
}

func (w *keyWalker) value() error {
	w.space()
	switch w.raw[w.pos] {
	case '{':
		return w.object()
	case '[':
		w.pos++
		w.space()
		if w.raw[w.pos] == ']' {
			w.pos++
			return nil
		}
		for {
			if err := w.value(); err != nil {
				return err
			}
			w.space()
			if w.raw[w.pos] == ']' {
				w.pos++
				return nil
			}
			w.pos++ // comma, guaranteed by json.Valid
		}
	case '"':
		w.quoted()
	case 't':
		w.pos += 4
	case 'f':
		w.pos += 5
	case 'n':
		w.pos += 4
	default:
		// Numeric tokens are not converted: UseNumber previously accepted every
		// valid literal, including values outside floating-point or integer ranges.
		for w.pos < len(w.raw) {
			switch w.raw[w.pos] {
			case ',', '}', ']', ' ', '\t', '\n', '\r':
				return nil
			default:
				w.pos++
			}
		}
	}
	return nil
}

func (w *keyWalker) object() error {
	w.pos++
	w.space()
	if w.raw[w.pos] == '}' {
		w.pos++
		return nil
	}
	// Most receipt objects have a small fixed roster. Large objects promote to
	// a map, keeping worst-case key lookup bounded without shared mutable state.
	var small [8][]byte
	count := 0
	var large map[string]struct{}
	for {
		w.space()
		start, end, escaped := w.quoted()
		key := w.raw[start+1 : end-1]
		if escaped || !utf8.Valid(key) {
			var decoded string
			if err := json.Unmarshal(w.raw[start:end], &decoded); err != nil {
				return err
			}
			key = []byte(decoded)
		}
		duplicate := false
		if large != nil {
			_, duplicate = large[string(key)]
		} else {
			for _, previous := range small[:count] {
				if bytes.Equal(previous, key) {
					duplicate = true
					break
				}
			}
		}
		if duplicate {
			return fmt.Errorf("duplicate JSON key %q", string(key))
		}
		if large != nil {
			large[string(key)] = struct{}{}
		} else if count < len(small) {
			small[count] = key
			count++
		} else {
			large = make(map[string]struct{}, 2*len(small))
			for _, previous := range small {
				large[string(previous)] = struct{}{}
			}
			large[string(key)] = struct{}{}
		}
		w.space()
		w.pos++ // colon, guaranteed by json.Valid
		if err := w.value(); err != nil {
			return err
		}
		w.space()
		if w.raw[w.pos] == '}' {
			w.pos++
			return nil
		}
		w.pos++ // comma, guaranteed by json.Valid
	}
}
