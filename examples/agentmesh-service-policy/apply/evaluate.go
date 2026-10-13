package apply

import (
	"strings"

	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
)

// Facts contain the trusted current grant and required request, plus a candidate.
// Gooo alone interprets their policy semantics.
type Facts = servicepolicy.GrantUpdateFacts

// Decision is Gooo's seven-field eligibility result, not a committed transition.
type Decision = servicepolicy.GrantUpdateDecision

type evaluationProof = servicepolicy.GrantUpdateProof

func sourceGeneration() (string, error) {
	bindings, err := servicepolicy.SourceBindings()
	if err != nil {
		return "", err
	}
	return bindings.GenerationSHA256, nil
}

func validGeneration(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, ch := range value[len("sha256:"):] {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}

func evaluate(f Facts) (evaluationProof, error) {
	return servicepolicy.EvaluateGrantUpdate(f)
}
