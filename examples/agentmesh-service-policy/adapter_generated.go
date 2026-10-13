// Code generated from pinned compiler-owned entry identities; DO NOT EDIT.
package servicepolicy

import (
	impact "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/impact"
	matcher "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/matcher"
)

func nativeMatcher(f MatcherFacts) bool {
	return matcher.GoooPackage0ActivityMatchServicePattern(f.Pattern, f.ReqType, f.ReqName)
}
func nativeImpact(f ImpactFacts) any {
	return impact.GoooPackage1ActivityCompareServiceGrant(f.Before, f.After, f.ReqType, f.ReqName)
}
