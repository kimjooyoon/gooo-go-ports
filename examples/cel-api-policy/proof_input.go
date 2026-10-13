package nativepolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	MaxBatchBytes     = 4 * 1024 * 1024
	MaxBatchCount     = 1024
	maxAssetCount     = 64
	maxAssetNameBytes = 512
)

var errSavedSchema = errors.New("expected exact typed bounded saved API proof")

// DecodeAPIProof preserves the saved facts, including ignored raw placeholders.
// Every nested object is exact; encoding/json's duplicate-key overwrite and
// malformed-Unicode replacement behavior are refused before decoding.
func DecodeAPIProof(raw []byte) (APIProof, error) {
	root, err := strictObject(raw, []string{"facts", "decision", "bindings"}, 3)
	if err != nil {
		return APIProof{}, err
	}
	facts, err := DecodeAPI(root["facts"])
	if err != nil {
		return APIProof{}, fmt.Errorf("facts: %w", err)
	}
	decisionFields := []string{"allowed", "reason", "authenticated", "tenant_present", "tenant_matched", "action_present", "action_matched", "access_allowed", "access_error", "access_result", "group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal"}
	d, err := strictObject(root["decision"], decisionFields, 15)
	if err != nil {
		return APIProof{}, fmt.Errorf("decision: %w", err)
	}
	for name, value := range d {
		if name == "reason" || name == "access_result" || name == "effective_principal" {
			if _, err := strictString(value, MaxTextBytes); err != nil {
				return APIProof{}, fmt.Errorf("decision %s: %w", name, err)
			}
		} else if !bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false")) {
			return APIProof{}, fmt.Errorf("decision %s: %w", name, errSavedSchema)
		}
	}
	b, err := strictObject(root["bindings"], []string{"compiler_source_sha", "assets", "generation_sha256"}, 3)
	if err != nil {
		return APIProof{}, fmt.Errorf("bindings: %w", err)
	}
	for _, name := range []string{"compiler_source_sha", "generation_sha256"} {
		if _, err := strictString(b[name], MaxTextBytes); err != nil {
			return APIProof{}, fmt.Errorf("bindings %s: %w", name, err)
		}
	}
	assets, err := strictObject(b["assets"], nil, maxAssetCount)
	if err != nil || len(assets) == 0 {
		return APIProof{}, fmt.Errorf("binding assets: %w", errSavedSchema)
	}
	for name, value := range assets {
		if name == "" || len(name) > maxAssetNameBytes {
			return APIProof{}, errSavedSchema
		}
		if _, err := strictString(value, MaxTextBytes); err != nil {
			return APIProof{}, fmt.Errorf("asset %s: %w", name, err)
		}
	}
	var saved APIProof
	if err := json.Unmarshal(raw, &saved); err != nil {
		return APIProof{}, err
	}
	saved.Facts = facts
	return saved, nil
}

// DecodeAPIProofBatch validates every member before callers emit any result.
func DecodeAPIProofBatch(raw []byte) ([]APIProof, error) {
	if len(raw) > MaxBatchBytes || !utf8.Valid(raw) {
		return nil, errSavedSchema
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 || len(rows) > MaxBatchCount {
		return nil, errSavedSchema
	}
	proofs := make([]APIProof, len(rows))
	for i, row := range rows {
		proof, err := DecodeAPIProof(row)
		if err != nil {
			return nil, fmt.Errorf("saved case %d: %w", i, err)
		}
		proofs[i] = proof
	}
	return proofs, nil
}

func strictString(raw []byte, maxBytes int) (string, error) {
	var text string
	if len(raw) == 0 || raw[0] != '"' || !validUnicodeEscapes(raw) || json.Unmarshal(raw, &text) != nil || len(text) > maxBytes || !utf8.ValidString(text) {
		return "", errSavedSchema
	}
	return text, nil
}

// nil keys denotes a bounded string-key map; all other objects require exactly
// the named fields. Duplicate keys are detected after escape decoding.
func strictObject(raw []byte, keys []string, maxFields int) (map[string]json.RawMessage, error) {
	if len(raw) > MaxBatchBytes || !utf8.Valid(raw) {
		return nil, errSavedSchema
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errSavedSchema
	}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	values := map[string]json.RawMessage{}
	for d.More() {
		before := d.InputOffset()
		token, err := d.Token()
		if err != nil {
			return nil, errSavedSchema
		}
		key, ok := token.(string)
		keyRaw := bytes.TrimSpace(raw[before:d.InputOffset()])
		keyRaw = bytes.TrimSpace(bytes.TrimPrefix(keyRaw, []byte(",")))
		if _, err := strictString(keyRaw, maxAssetNameBytes); err != nil {
			return nil, errSavedSchema
		}
		if !ok || (keys != nil && !allowed[key]) {
			return nil, errSavedSchema
		}
		if _, duplicate := values[key]; duplicate || len(values) >= maxFields {
			return nil, errSavedSchema
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, errSavedSchema
		}
		values[key] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || (keys != nil && len(values) != len(keys)) {
		return nil, errSavedSchema
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errSavedSchema
	}
	return values, nil
}
