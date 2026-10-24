// Lossless transport helpers adapted from the pinned service-grant-many input.go.
// BSD-3-Clause; see LICENSE and transport-source-provenance.json.
package aliasorigin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

const MaxInputBytes = 1 << 20
const MaxReportBytes = 4 << 20

var invalidJSON = errors.New("expected exact lossless alias-origin facts")

func object(raw []byte, keys ...string) (map[string]json.RawMessage, error) {
	if len(raw) > MaxInputBytes || !utf8.Valid(raw) {
		return nil, invalidJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, invalidJSON
	}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	values := map[string]json.RawMessage{}
	for decoder.More() {
		start := decoder.InputOffset()
		token, err := decoder.Token()
		key, ok := token.(string)
		keyRaw := bytes.TrimSpace(raw[start:decoder.InputOffset()])
		keyRaw = bytes.TrimSpace(bytes.TrimPrefix(keyRaw, []byte(",")))
		if err != nil || !ok || !validJSONString(keyRaw) || !allowed[key] {
			return nil, invalidJSON
		}
		if _, seen := values[key]; seen {
			return nil, invalidJSON
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, invalidJSON
		}
		values[key] = value
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') || len(values) != len(keys) {
		return nil, invalidJSON
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, invalidJSON
	}
	return values, nil
}

func text(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if !validJSONString(raw) {
		return "", invalidJSON
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", invalidJSON
	}
	return value, nil
}

func array(raw json.RawMessage) ([]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, invalidJSON
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil, invalidJSON
	}
	return values, nil
}

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
