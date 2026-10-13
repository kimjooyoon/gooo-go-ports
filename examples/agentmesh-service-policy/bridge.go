package servicepolicy

import "encoding/json"

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
