// Code generated from pinned compiler-owned entry identities; DO NOT EDIT.
package servicepolicy

import (
	checkedimpact "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/checked-impact"
	checkedmatcher "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/checked-matcher"
	checkedrefinement "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/checked-refinement"
	impact "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/impact"
	matcher "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/matcher"
	refinement "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/refinement"
)

func nativeMatcher(f MatcherFacts) bool {
	return matcher.GoooPackage0ActivityMatchServicePattern(f.Pattern, f.ReqType, f.ReqName)
}
func nativeImpact(f ImpactFacts) any {
	return impact.GoooPackage1ActivityCompareServiceGrant(f.Before, f.After, f.ReqType, f.ReqName)
}
func nativeRefinement(f RefinementFacts) any {
	return refinement.GoooPackage1ActivityAnalyzeServiceGrant(f.Before, f.After)
}
func nativeMatcherChecked(f MatcherFacts) any {
	return checkedmatcher.GoooPackage4ActivityMatchServicePatternChecked(f.Pattern, f.ReqType, f.ReqName)
}
func nativeImpactChecked(f ImpactFacts) any {
	return checkedimpact.GoooPackage5ActivityCompareServiceGrantChecked(f.Before, f.After, f.ReqType, f.ReqName)
}
func nativeRefinementChecked(f RefinementFacts) any {
	return checkedrefinement.GoooPackage5ActivityAnalyzeServiceGrantChecked(f.Before, f.After)
}
func nativeValidType(value string) bool {
	return checkedmatcher.GoooPackage1ActivityValidateServiceType(value)
}
func nativeValidName(value string) bool {
	return checkedmatcher.GoooPackage2ActivityValidateServiceName(value)
}
func nativeValidPattern(value string) bool {
	return checkedmatcher.GoooPackage3ActivityValidateServicePattern(value)
}
