package nativepolicy

import "errors"

var (
	ErrAPIGenerationMismatch = errors.New("saved API proof does not match this bound source generation")
	ErrAPIDecisionMismatch   = errors.New("saved API decision does not match re-evaluated facts")
)

// APIReplay is historical consistency under the same bound source generation.
// Its decision is an observation of supplied saved facts, never authorization
// for a current operation. Bindings identify sources and their generation
// producer; they do not identify the host binary, factual issuer or history.
type APIReplay struct {
	Schema                 string      `json:"schema"`
	Status                 string      `json:"status"`
	Scope                  string      `json:"scope"`
	VerifiedDecisionFields int         `json:"verified_decision_fields"`
	Decision               APIDecision `json:"decision"`
	Bindings               Bindings    `json:"bindings"`
}

// VerifyAPIProof first validates the current embedded source generation, then
// requires its complete exact bindings and re-evaluates all fifteen decision
// fields through EvaluateAPI. It performs no current-fact lookup or effect.
// Unsigned consistent synthetic facts and decisions are accepted: consistency
// does not prove that a factual event occurred or authenticate its issuer.
func VerifyAPIProof(saved APIProof) (APIReplay, error) {
	current, err := SourceBindings()
	if err != nil {
		return APIReplay{}, err
	}
	if !sameBindings(saved.Bindings, current) {
		return APIReplay{}, ErrAPIGenerationMismatch
	}
	actual, err := EvaluateAPI(saved.Facts)
	if err != nil {
		return APIReplay{}, err
	}
	if saved.Decision != actual.Decision {
		return APIReplay{}, ErrAPIDecisionMismatch
	}
	return APIReplay{
		Schema: "gooo/api-proof-replay/v1", Status: "HISTORICALLY_CONSISTENT",
		Scope: "same_bound_source_generation", VerifiedDecisionFields: 15,
		Decision: actual.Decision, Bindings: current,
	}, nil
}

func sameBindings(a, b Bindings) bool {
	if a.CompilerSourceSHA != b.CompilerSourceSHA || a.GenerationSHA256 != b.GenerationSHA256 || len(a.Assets) != len(b.Assets) {
		return false
	}
	for name, expected := range b.Assets {
		if actual, exists := a.Assets[name]; !exists || actual != expected {
			return false
		}
	}
	return true
}
