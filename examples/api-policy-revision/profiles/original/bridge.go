package nativepolicy

import "encoding/json"

func EvaluateScalar(f Facts) (Proof, error) {
	b, err := SourceBindings()
	if err != nil {
		return Proof{}, err
	}
	if err := validateTexts(f.GroupValue, f.PrincipalValue); err != nil {
		return Proof{}, err
	}
	native := nativeScalar(f)
	raw, err := json.Marshal(native)
	if err != nil {
		return Proof{}, err
	}
	var decision Decision
	if err := json.Unmarshal(raw, &decision); err != nil {
		return Proof{}, err
	}
	return Proof{Facts: f, Decision: decision, Bindings: b}, nil
}

func EvaluateAPI(f APIFacts) (APIProof, error) {
	b, err := SourceBindings()
	if err != nil {
		return APIProof{}, err
	}
	if err := validateTexts(f.GroupValue, f.PrincipalValue, f.CallerTenant, f.ResourceTenant, f.RequestedAction, f.AllowedAction); err != nil {
		return APIProof{}, err
	}
	native := nativeAPI(f)
	raw, err := json.Marshal(native)
	if err != nil {
		return APIProof{}, err
	}
	var decision APIDecision
	if err := json.Unmarshal(raw, &decision); err != nil {
		return APIProof{}, err
	}
	return APIProof{Facts: f, Decision: decision, Bindings: b}, nil
}
