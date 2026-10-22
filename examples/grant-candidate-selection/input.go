package grantselection

import (
	"bytes"
	"encoding/json"
)

// DecodeInput preserves decoded values and order; it refuses lossy structure.
func DecodeInput(raw []byte) (Input, error) {
	var out Input
	fields, err := object(raw, "before", "required_requests", "candidates")
	if err != nil {
		return out, err
	}
	if out.Before, err = text(fields["before"]); err != nil {
		return Input{}, err
	}
	requests, err := array(fields["required_requests"])
	if err != nil {
		return Input{}, err
	}
	out.RequiredRequests = make([]Request, len(requests))
	for i, row := range requests {
		values, err := object(row, "type", "name")
		if err != nil {
			return Input{}, err
		}
		if out.RequiredRequests[i].Type, err = text(values["type"]); err != nil {
			return Input{}, err
		}
		if out.RequiredRequests[i].Name, err = text(values["name"]); err != nil {
			return Input{}, err
		}
	}
	candidates, err := array(fields["candidates"])
	if err != nil {
		return Input{}, err
	}
	out.Candidates = make([]Candidate, len(candidates))
	for i, row := range candidates {
		values, err := object(row, "id", "after", "preference")
		if err != nil {
			return Input{}, err
		}
		if out.Candidates[i].ID, err = text(values["id"]); err != nil {
			return Input{}, err
		}
		if out.Candidates[i].After, err = text(values["after"]); err != nil {
			return Input{}, err
		}
		value := bytes.TrimSpace(values["preference"])
		if bytes.Equal(value, []byte("null")) || json.Unmarshal(value, &out.Candidates[i].Preference) != nil {
			return Input{}, invalidJSON
		}
	}
	return out, nil
}

type Document struct {
	InputSHA256 string `json:"input_sha256"`
	Result      Result `json:"result"`
}

// AssessJSON also identifies the exact received JSON bytes, not reserialization.
func AssessJSON(raw []byte) (Document, error) {
	input, err := DecodeInput(raw)
	if err != nil {
		return Document{}, &Problem{State: "INVALID_TRANSPORT", Stage: "decode_input", Code: "INVALID_JSON_FACTS", Index: -1}
	}
	result, err := Assess(input)
	if err != nil {
		return Document{}, err
	}
	return Document{InputSHA256: digest(raw), Result: result}, nil
}
