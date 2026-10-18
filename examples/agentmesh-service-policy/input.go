package servicepolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

const MaxBatchBytes = 1024 * 1024
const MaxBatchCount = 1024

func decodeFacts(raw []byte, keys []string) error {
	invalid := errors.New("expected exact JSON Text service facts")
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
		if !validJSONString(keyRaw) || !ok || !allowed[key] || seen[key] {
			return invalid
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || !validJSONString(value) {
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

// Structural decoding must preserve exact Text. ASCII grammar and 128-byte
// component limits are evaluated by Gooo after decoding, not by this transport.
func validJSONString(raw []byte) bool {
	var text string
	if len(raw) < 2 || raw[0] != '"' || !utf8.Valid(raw) || json.Unmarshal(raw, &text) != nil {
		return false
	}
	// encoding/json accepts unpaired surrogate escapes by replacing them. Reject
	// that lossy conversion while allowing genuine Unicode and surrogate pairs.
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		value, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+12 > len(raw)-1 || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else if value >= 0xdc00 && value <= 0xdfff {
			return false
		} else {
			i += 5
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

// DecodeGrantUpdate checks the transport only. Represented domain refusals are
// returned as GrantUpdateDecision values by the generated Gooo activity.
func DecodeGrantUpdate(raw []byte) (GrantUpdateFacts, error) {
	if err := decodeFacts(raw, []string{"before", "after", "required_type", "required_name"}); err != nil {
		return GrantUpdateFacts{}, err
	}
	var f GrantUpdateFacts
	if err := json.Unmarshal(raw, &f); err != nil {
		return GrantUpdateFacts{}, err
	}
	return f, nil
}
