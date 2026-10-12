package main

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type uuidParseObservations struct {
	Suite                                                                           suite
	ReferenceCases, LegacyReferenceInputs, ParserAnchorInputs, AddedReferenceInputs int
}

// These immutable parser-specific inputs retain the 12 first-use observations
// and 124 separately observed boundary inputs. No private file or old receipt
// is read at runtime. The new expectations always call original Go below.
//
//go:embed testdata/uuid-parse-anchors.json
var uuidParseAnchorJSON []byte

type uuidParseAnchor struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Text     string `json:"input"`
}

func uuidParseAnchors() ([]uuidParseAnchor, error) {
	var anchors []uuidParseAnchor
	if err := json.Unmarshal(uuidParseAnchorJSON, &anchors); err != nil {
		return nil, err
	}
	if len(anchors) == 0 {
		return nil, fmt.Errorf("empty UUID parser anchors")
	}
	seen := map[string]bool{}
	for _, anchor := range anchors {
		if anchor.Name == "" || anchor.Category == "" || seen[anchor.Name] {
			return nil, fmt.Errorf("missing or repeated UUID parser anchor identity")
		}
		if _, err := readUUIDInput([]any{anchor.Text}); err != nil {
			return nil, err
		}
		seen[anchor.Name] = true
	}
	return anchors, nil
}

// Call the original byte-preserved google/uuid v1.6.0 Parse and String methods.
// UUID.String() remains meaningful when Parse returns an error: raw32 commits a
// failed xtob byte, while standard-form parsing retains only preceding bytes.
func uuidParseCanonicalOracle(text string) any {
	value, err := uuid.Parse(text)
	message := ""
	if err != nil {
		message = err.Error()
	}
	return map[string]any{"parsed": err == nil, "canonical_uuid": value.String(), "error": message, "invalid_length": uuid.IsInvalidLengthError(err)}
}

func uuidParseCorpus() (uuidParseObservations, error) {
	legacy := uuidCorpusInputs()
	anchors, err := uuidParseAnchors()
	if err != nil {
		return uuidParseObservations{}, err
	}
	inputs := make([][]any, 0, len(legacy)+len(anchors))
	seen := map[string]bool{}
	for _, row := range legacy {
		text, err := readUUIDInput(row)
		if err != nil {
			return uuidParseObservations{}, err
		}
		if seen[text] {
			return uuidParseObservations{}, fmt.Errorf("duplicate original UUID corpus input")
		}
		seen[text] = true
		inputs = append(inputs, []any{text})
	}
	for _, anchor := range anchors {
		if !seen[anchor.Text] {
			seen[anchor.Text] = true
			inputs = append(inputs, []any{anchor.Text})
		}
	}
	parsed, err := uuidReferenceSuite(uuidParsePackage, "ParseCanonical", inputs, uuidParseCanonicalOracle)
	if err != nil {
		return uuidParseObservations{}, err
	}
	added := len(inputs) - len(legacy)
	parsed.Domain = fmt.Sprintf("%d finite decoded valid-UTF8 Text inputs<=1024 bytes; original google/uuid v1.6.0 Parse + returned UUID.String + exact error + IsInvalidLengthError mapped to a required record. The original %d UUID validation inputs retain their order; %d parser-specific anchors contribute %d new inputs after exact Text deduplication. Observations include raw32 failed-byte assignment, standard-form partial returns, unrestricted38-byte outer delimiters, hyphen priority, URN quoted prefixes, byte9 rune cuts, lengths and varied byte patterns. No binary UUID method set, arbitrary-byte ParseBytes, exhaustive-string-domain or whole-module compatibility claim.", len(inputs), len(legacy), len(anchors), added)
	return uuidParseObservations{Suite: parsed, ReferenceCases: len(parsed.Cases), LegacyReferenceInputs: len(legacy), ParserAnchorInputs: len(anchors), AddedReferenceInputs: added}, nil
}
