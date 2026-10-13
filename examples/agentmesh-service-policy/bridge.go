package servicepolicy

import (
	"encoding/json"
	"errors"
)

func EvaluateMatcher(f MatcherFacts) (MatcherProof, error) {
	b, err := SourceBindings()
	if err != nil {
		return MatcherProof{}, err
	}
	if err := ValidateMatcher(f); err != nil {
		return MatcherProof{}, err
	}
	return MatcherProof{Facts: f, Matched: nativeMatcher(f), Bindings: b}, nil
}

func EvaluateImpact(f ImpactFacts) (ImpactProof, error) {
	b, err := SourceBindings()
	if err != nil {
		return ImpactProof{}, err
	}
	if err := ValidateImpact(f); err != nil {
		return ImpactProof{}, err
	}
	encoded, err := json.Marshal(nativeImpact(f))
	if err != nil {
		return ImpactProof{}, err
	}
	var d ImpactDecision
	if err := json.Unmarshal(encoded, &d); err != nil {
		return ImpactProof{}, err
	}
	return ImpactProof{Facts: f, Decision: d, Bindings: b}, nil
}

func EvaluateRefinement(f RefinementFacts) (RefinementProof, error) {
	b, err := SourceBindings()
	if err != nil {
		return RefinementProof{}, err
	}
	if err := ValidateRefinement(f); err != nil {
		return RefinementProof{}, err
	}
	encoded, err := json.Marshal(nativeRefinement(f))
	if err != nil {
		return RefinementProof{}, err
	}
	// Require every field, including the false/empty no-witness sentinel fields.
	var fields map[string]json.RawMessage
	if json.Unmarshal(encoded, &fields) != nil || len(fields) != 8 {
		return RefinementProof{}, errors.New("invalid Gooo refinement record shape")
	}
	for _, key := range []string{"status", "rule_id", "supported_subset", "has_witness", "witness_type", "witness_name", "before_match", "after_match"} {
		if _, ok := fields[key]; !ok || string(fields[key]) == "null" {
			return RefinementProof{}, errors.New("invalid Gooo refinement record field")
		}
	}
	var d RefinementDecision
	if err := json.Unmarshal(encoded, &d); err != nil {
		return RefinementProof{}, err
	}
	if err := validateRefinementDecision(d); err != nil {
		return RefinementProof{}, err
	}
	return RefinementProof{Facts: f, Decision: d, Bindings: b}, nil
}
