package nativepolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"
)

const MaxTextBytes = 65536

func validateTexts(values ...string) error {
	for _, value := range values {
		if len(value) > MaxTextBytes || !utf8.ValidString(value) {
			return errors.New("raw fact text outside valid UTF-8 65536-byte domain")
		}
	}
	return nil
}

// DecodeScalar accepts exactly four raw facts, never a host comparison result.
func DecodeScalar(raw []byte) (Facts, error) {
	if err := validateObject(raw, false); err != nil {
		return Facts{}, err
	}
	var out Facts
	err := json.Unmarshal(raw, &out)
	return out, err
}

// DecodeAPI accepts a synthetic server fact record. The boolean is not proof
// of authentication; the actual trusted provider is outside this prototype.
func DecodeAPI(raw []byte) (APIFacts, error) {
	if err := validateObject(raw, true); err != nil {
		return APIFacts{}, err
	}
	var out APIFacts
	err := json.Unmarshal(raw, &out)
	return out, err
}

func validateObject(raw []byte, api bool) error {
	invalid := errors.New("expected exact typed bounded raw fact object")
	if !utf8.Valid(raw) || len(raw) > 9*(MaxTextBytes*6+32) {
		return invalid
	}
	fields := map[string]bool{"group_present": true, "group_value": false, "principal_present": true, "principal_value": false}
	if api {
		fields["authenticated"] = true
		for _, name := range []string{"caller_tenant", "resource_tenant", "requested_action", "allowed_action"} {
			fields[name] = false
		}
	}
	keys := make([]string, 0, len(fields))
	for name := range fields {
		keys = append(keys, name)
	}
	object, err := strictObject(raw, keys, len(keys))
	if err != nil {
		return invalid
	}
	for key, value := range object {
		if fields[key] {
			if !bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false")) {
				return invalid
			}
		} else if _, err := strictString(value, MaxTextBytes); err != nil {
			return invalid
		}
	}
	return nil
}

// encoding/json otherwise replaces malformed surrogate escapes with U+FFFD.
// Refuse that lossy transport before exact Gooo text relations are evaluated.
func validUnicodeEscapes(raw []byte) bool {
	for i := 1; i < len(raw)-1; {
		if raw[i] != '\\' {
			i++
			continue
		}
		if i+1 >= len(raw)-1 {
			return false
		}
		if raw[i+1] != 'u' {
			i += 2
			continue
		}
		if i+6 > len(raw)-1 {
			return false
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
			i += 12
			continue
		}
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		i += 6
	}
	return true
}
