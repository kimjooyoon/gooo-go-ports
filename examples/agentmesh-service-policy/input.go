package servicepolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const MaxBatchBytes = 1024 * 1024
const MaxBatchCount = 1024

func decodeFacts(raw []byte, keys []string) error {
	invalid := errors.New("expected exact canonical ASCII service facts")
	if len(raw) > 4096 || !utf8.Valid(raw) {
		return invalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return invalid
	}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	seen := map[string]bool{}
	for d.More() {
		start := d.InputOffset()
		keyToken, err := d.Token()
		if err != nil {
			return invalid
		}
		key, ok := keyToken.(string)
		keyRaw := bytes.TrimSpace(raw[start:d.InputOffset()])
		keyRaw = bytes.TrimSpace(bytes.TrimPrefix(keyRaw, []byte(",")))
		if !validASCIIJSONString(keyRaw) || !ok || !allowed[key] || seen[key] {
			return invalid
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || !validASCIIJSONString(value) {
			return invalid
		}
		var text string
		if json.Unmarshal(value, &text) != nil || len(text) > MaxComponentBytes {
			return invalid
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(seen) != len(allowed) {
		return invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return invalid
	}
	return nil
}

func validASCIIJSONString(raw []byte) bool {
	var text string
	if len(raw) < 2 || raw[0] != '"' || json.Unmarshal(raw, &text) != nil {
		return false
	}
	// The actual domain is printable ASCII. All surrogate escapes and replacement
	// characters are outside it, including encoding/json's lossy replacement.
	for _, c := range []byte(text) {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func DecodeMatcher(raw []byte) (MatcherFacts, error) {
	if err := decodeFacts(raw, []string{"pattern", "req_type", "req_name"}); err != nil {
		return MatcherFacts{}, err
	}
	var f MatcherFacts
	if err := json.Unmarshal(raw, &f); err != nil {
		return MatcherFacts{}, err
	}
	return f, ValidateMatcher(f)
}
func DecodeImpact(raw []byte) (ImpactFacts, error) {
	if err := decodeFacts(raw, []string{"before", "after", "req_type", "req_name"}); err != nil {
		return ImpactFacts{}, err
	}
	var f ImpactFacts
	if err := json.Unmarshal(raw, &f); err != nil {
		return ImpactFacts{}, err
	}
	return f, ValidateImpact(f)
}

func DecodeRefinement(raw []byte) (RefinementFacts, error) {
	if err := decodeFacts(raw, []string{"before", "after"}); err != nil {
		return RefinementFacts{}, err
	}
	var f RefinementFacts
	if err := json.Unmarshal(raw, &f); err != nil {
		return RefinementFacts{}, err
	}
	return f, ValidateRefinement(f)
}
