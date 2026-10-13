package servicepolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// Existing APIs retain their valid-input proof shapes while executing the
// checked Gooo entrypoints. A domain refusal remains an error in these APIs.
func EvaluateMatcher(f MatcherFacts) (MatcherProof, error) {
	p, err := EvaluateMatcherChecked(f)
	if err != nil {
		return MatcherProof{}, err
	}
	if p.Decision.DomainStatus != "VALID_INPUT" {
		return MatcherProof{}, invalidInput(p.Decision.InvalidField)
	}
	return MatcherProof{Facts: f, Matched: p.Decision.Matched, Bindings: p.Bindings}, nil
}

func EvaluateImpact(f ImpactFacts) (ImpactProof, error) {
	p, err := EvaluateImpactChecked(f)
	if err != nil {
		return ImpactProof{}, err
	}
	if p.Decision.DomainStatus != "VALID_INPUT" {
		return ImpactProof{}, invalidInput(p.Decision.InvalidField)
	}
	d := ImpactDecision{BeforeMatch: p.Decision.BeforeMatch, AfterMatch: p.Decision.AfterMatch, Change: p.Decision.Change}
	return ImpactProof{Facts: f, Decision: d, Bindings: p.Bindings}, nil
}

func EvaluateRefinement(f RefinementFacts) (RefinementProof, error) {
	p, err := EvaluateRefinementChecked(f)
	if err != nil {
		return RefinementProof{}, err
	}
	if p.Decision.DomainStatus != "VALID_INPUT" {
		return RefinementProof{}, invalidInput(p.Decision.InvalidField)
	}
	return RefinementProof{Facts: f, Decision: p.Decision.RefinementDecision, Bindings: p.Bindings}, nil
}

// EvaluateMatcherChecked returns Gooo's explicit input-domain result. Invalid
// grammar is represented as INVALID_INPUT; transport and binding failures error.
func EvaluateMatcherChecked(f MatcherFacts) (CheckedMatcherProof, error) {
	if err := exactNativeText(f.Pattern, f.ReqType, f.ReqName); err != nil {
		return CheckedMatcherProof{}, err
	}
	b, err := SourceBindings()
	if err != nil {
		return CheckedMatcherProof{}, err
	}
	var d CheckedMatcherDecision
	if err := decodeDecision(nativeMatcherChecked(f), []string{"domain_status", "invalid_field", "matched"}, &d); err != nil {
		return CheckedMatcherProof{}, err
	}
	if err := validateCheckedMatcherDecision(f, d); err != nil {
		return CheckedMatcherProof{}, err
	}
	return CheckedMatcherProof{Facts: f, Decision: d, Bindings: b}, nil
}

func EvaluateImpactChecked(f ImpactFacts) (CheckedImpactProof, error) {
	if err := exactNativeText(f.Before, f.After, f.ReqType, f.ReqName); err != nil {
		return CheckedImpactProof{}, err
	}
	b, err := SourceBindings()
	if err != nil {
		return CheckedImpactProof{}, err
	}
	var d CheckedImpactDecision
	if err := decodeDecision(nativeImpactChecked(f), []string{"domain_status", "invalid_field", "before_match", "after_match", "change"}, &d); err != nil {
		return CheckedImpactProof{}, err
	}
	if err := validateCheckedImpactDecision(f, d); err != nil {
		return CheckedImpactProof{}, err
	}
	return CheckedImpactProof{Facts: f, Decision: d, Bindings: b}, nil
}

func EvaluateRefinementChecked(f RefinementFacts) (CheckedRefinementProof, error) {
	if err := exactNativeText(f.Before, f.After); err != nil {
		return CheckedRefinementProof{}, err
	}
	b, err := SourceBindings()
	if err != nil {
		return CheckedRefinementProof{}, err
	}
	var d CheckedRefinementDecision
	keys := []string{"domain_status", "invalid_field", "status", "rule_id", "supported_subset", "has_witness", "witness_type", "witness_name", "before_match", "after_match"}
	if err := decodeDecision(nativeRefinementChecked(f), keys, &d); err != nil {
		return CheckedRefinementProof{}, err
	}
	if err := validateCheckedRefinementDecision(f, d); err != nil {
		return CheckedRefinementProof{}, err
	}
	return CheckedRefinementProof{Facts: f, Decision: d, Bindings: b}, nil
}

func exactNativeText(values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return errors.New("facts must preserve exact UTF-8 Text")
		}
	}
	return nil
}

// Require every field, including false/empty sentinels, before decoding into a
// typed struct. No unknown, duplicate, null or defaulted field is accepted.
func decodeDecision(value any, keys []string, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	invalid := errors.New("invalid Gooo checked record shape or field")
	d := json.NewDecoder(bytes.NewReader(encoded))
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
		token, err := d.Token()
		if err != nil {
			return invalid
		}
		key, ok := token.(string)
		if !ok || !allowed[key] || seen[key] {
			return invalid
		}
		seen[key] = true
		var field json.RawMessage
		if d.Decode(&field) != nil || bytes.Equal(bytes.TrimSpace(field), []byte("null")) {
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
	return json.Unmarshal(encoded, target)
}
