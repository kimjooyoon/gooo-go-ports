package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

func wire(code string) error { return fmt.Errorf("WIRE_REFUSED: %s", code) }
func hex4(raw []byte) (uint64, error) {
	if len(raw) != 4 {
		return 0, wire("MALFORMED_JSON")
	}
	v, err := strconv.ParseUint(string(raw), 16, 16)
	if err != nil {
		return 0, wire("MALFORMED_JSON")
	}
	return v, nil
}

// Inspect source escapes before encoding/json can replace an unpaired surrogate.
// JSON syntax itself is subsequently checked by the standard decoder.
func unicodeScalars(raw []byte) error {
	if !utf8.Valid(raw) {
		return wire("INVALID_RAW_UTF8")
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
			return wire("MALFORMED_JSON")
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(raw) {
			return wire("MALFORMED_JSON")
		}
		first, err := hex4(raw[i+2 : i+6])
		if err != nil {
			return err
		}
		if first >= 0xD800 && first <= 0xDBFF {
			if i+12 > len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return wire("INVALID_UNICODE_SCALAR")
			}
			second, err := hex4(raw[i+8 : i+12])
			if err != nil {
				return err
			}
			if second < 0xDC00 || second > 0xDFFF {
				return wire("INVALID_UNICODE_SCALAR")
			}
			i += 11
		} else if first >= 0xDC00 && first <= 0xDFFF {
			return wire("INVALID_UNICODE_SCALAR")
		} else {
			i += 5
		}
	}
	return nil
}

func ParseInput(raw []byte) (Input, error) {
	var in Input
	if err := unicodeScalars(raw); err != nil {
		return in, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return in, wire("OBJECT_REQUIRED")
	}
	names := map[string]bool{"method": true, "delivery_state": true, "body_replayable": true, "idempotency_key_present": true, "server_repeat_contract_verified": true, "request_binding_current": true, "current_access_allowed": true, "attempts_remaining": true, "provider_retry_guidance": true}
	seen := map[string]bool{}
	text := func(value []byte) (string, error) {
		if len(value) == 0 || value[0] != '"' {
			return "", wire("TEXT_TYPE")
		}
		var s string
		if json.Unmarshal(value, &s) != nil {
			return "", wire("MALFORMED_JSON")
		}
		if !utf8.ValidString(s) {
			return "", wire("INVALID_UNICODE_SCALAR")
		}
		if len(s) > 1024 {
			return "", wire("TEXT_BYTE_BOUND")
		}
		return s, nil
	}
	boolean := func(value []byte) (bool, error) {
		if bytes.Equal(value, []byte("true")) {
			return true, nil
		}
		if bytes.Equal(value, []byte("false")) {
			return false, nil
		}
		return false, wire("BOOLEAN_TYPE")
	}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return in, wire("MALFORMED_JSON")
		}
		name, ok := token.(string)
		if !ok {
			return in, wire("MALFORMED_JSON")
		}
		if seen[name] {
			return in, wire("DUPLICATE_FIELD")
		}
		seen[name] = true
		if !names[name] {
			return in, wire("UNKNOWN_FIELD")
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return in, wire("MALFORMED_JSON")
		}
		value = bytes.TrimSpace(value)
		switch name {
		case "method":
			in.Method, err = text(value)
		case "delivery_state":
			in.DeliveryState, err = text(value)
		case "provider_retry_guidance":
			in.ProviderRetryGuidance, err = text(value)
		case "body_replayable":
			in.BodyReplayable, err = boolean(value)
		case "idempotency_key_present":
			in.IdempotencyKeyPresent, err = boolean(value)
		case "server_repeat_contract_verified":
			in.ServerRepeatContractVerified, err = boolean(value)
		case "request_binding_current":
			in.RequestBindingCurrent, err = boolean(value)
		case "current_access_allowed":
			in.CurrentAccessAllowed, err = boolean(value)
		case "attempts_remaining":
			// ParseInt alone accepts a leading plus; JSON grammar plus this token check
			// excludes decimals/exponents and keeps the complete signed-int64 range.
			if len(value) == 0 {
				return in, wire("INTEGER_TOKEN")
			}
			start := 0
			if value[0] == '-' {
				start = 1
			}
			if start == len(value) {
				return in, wire("INTEGER_TOKEN")
			}
			for _, b := range value[start:] {
				if b < '0' || b > '9' {
					return in, wire("INTEGER_TOKEN")
				}
			}
			in.AttemptsRemaining, err = strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				err = wire("INTEGER_RANGE")
			}
		}
		if err != nil {
			return in, err
		}
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return in, wire("MALFORMED_JSON")
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return in, wire("TRAILING_JSON")
	}
	if len(seen) != 9 {
		return in, wire("MISSING_FIELD")
	}
	return in, nil
}
