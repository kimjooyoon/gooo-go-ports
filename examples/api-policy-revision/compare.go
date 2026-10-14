package revisionpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

var (
	ErrProfileMismatch    = errors.New("comparison requires the exact original and readonly profiles")
	ErrComparisonMismatch = errors.New("saved comparison differs from complete native re-evaluation")
)

func checkProfiles(before, candidate string) error {
	if before != OriginalProfile || candidate != CandidateProfile {
		return ErrProfileMismatch
	}
	return nil
}

// Baseline creates a new original proof from supplied facts. It never repairs a
// saved proof, establishes the truth of these facts, or executes an API action.
func Baseline(facts original.APIFacts) (CompareRequest, error) {
	proof, err := original.EvaluateAPI(facts)
	if err != nil {
		return CompareRequest{}, fmt.Errorf("original baseline: %w", err)
	}
	return CompareRequest{BeforeProfile: OriginalProfile, CandidateProfile: CandidateProfile, Before: proof}, nil
}

// Compare first verifies the original sealed proof, then evaluates the candidate
// and both classifiers through generated Gooo. Go contains no policy, field
// comparison, classification or changed-count fallback.
func Compare(request CompareRequest) (ComparisonReport, error) {
	if err := checkProfiles(request.BeforeProfile, request.CandidateProfile); err != nil {
		return ComparisonReport{}, err
	}
	if _, err := original.VerifyAPIProof(request.Before); err != nil {
		return ComparisonReport{}, fmt.Errorf("original proof: %w", err)
	}
	bindings, err := RevisionBindings()
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("revision bindings: %w", err)
	}
	decision, err := json.Marshal(nativeCandidate(request.Before.Facts))
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("candidate output: %w", err)
	}
	after, err := decodeGeneratedProof(request.Before.Facts, decision, bindings)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("candidate output: %w", err)
	}
	flags, err := nativeDifferences(request.Before.Decision, after.Decision)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("difference classifier: %w", err)
	}
	flagsJSON, err := json.Marshal(flags)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("difference output: %w", err)
	}
	differences, err := decodeDifferences(flagsJSON)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("difference output: %w", err)
	}
	summaryValue, err := nativeSummary(request.Before.Decision, after.Decision)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("summary classifier: %w", err)
	}
	summaryJSON, err := json.Marshal(summaryValue)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("summary output: %w", err)
	}
	summary, err := decodeSummary(summaryJSON)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("summary output: %w", err)
	}
	return ComparisonReport{
		Schema: ComparisonSchema, Status: ComparisonStatus, Scope: ComparisonScope,
		BeforeProfile: OriginalProfile, CandidateProfile: CandidateProfile,
		Before: request.Before, After: after, Differences: differences, Summary: summary,
		ClassifierBindings: bindings,
	}, nil
}

// VerifyComparison recomputes the original proof, candidate and every classifier
// field before comparing the entire typed report, including all bindings. A
// consistently fabricated unsigned input may still be internally consistent;
// replay supplies neither current facts nor permission to execute an effect.
func VerifyComparison(saved ComparisonReport) (ComparisonReport, error) {
	actual, err := Compare(CompareRequest{
		BeforeProfile: saved.BeforeProfile, CandidateProfile: saved.CandidateProfile, Before: saved.Before,
	})
	if err != nil {
		return ComparisonReport{}, err
	}
	if !reflect.DeepEqual(saved, actual) {
		return ComparisonReport{}, ErrComparisonMismatch
	}
	return actual, nil
}

func decodeGeneratedProof(facts original.APIFacts, decision json.RawMessage, bindings original.Bindings) (original.APIProof, error) {
	// The unchanged original decoder validates all fifteen output fields and
	// nested fact/binding types. It does not verify the candidate as the original.
	raw, err := json.Marshal(struct {
		Facts    original.APIFacts `json:"facts"`
		Decision json.RawMessage   `json:"decision"`
		Bindings original.Bindings `json:"bindings"`
	}{facts, decision, bindings})
	if err != nil {
		return original.APIProof{}, err
	}
	return original.DecodeAPIProof(raw)
}
