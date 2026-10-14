// Code generated from actual lowered entry identities; DO NOT EDIT.
package revisionpolicy

import (
	"encoding/json"
	candidate "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision/internal/candidate"
	flags "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision/internal/flags"
	summary "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision/internal/summary"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func nativeCandidate(f original.APIFacts) any {
	return candidate.GoooPackage2ActivityReadOnlyAPI(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue, f.Authenticated, f.CallerTenant, f.ResourceTenant, f.RequestedAction, f.AllowedAction)
}
func nativeDifferences(before, after original.APIDecision) (any, error) {
	var a, b flags.GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f
	first, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(first, &a); err != nil {
		return nil, err
	}
	second, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(second, &b); err != nil {
		return nil, err
	}
	return flags.GoooPackage2ActivityDifferenceFlags(a, b), nil
}
func nativeSummary(before, after original.APIDecision) (any, error) {
	var a, b summary.GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f
	first, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(first, &a); err != nil {
		return nil, err
	}
	second, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(second, &b); err != nil {
		return nil, err
	}
	return summary.GoooPackage3ActivityChangeSummary(a, b), nil
}
