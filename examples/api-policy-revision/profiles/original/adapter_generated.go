// Code generated from actual lowered entry identities; DO NOT EDIT.
package nativepolicy

import (
	api "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy/internal/api"
	scalar "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy/internal/scalar"
)

func nativeScalar(f Facts) any {
	return scalar.GoooPackage0ActivityScalarAccess(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue)
}
func nativeAPI(f APIFacts) any {
	return api.GoooPackage1ActivityAPIAccess(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue, f.Authenticated, f.CallerTenant, f.ResourceTenant, f.RequestedAction, f.AllowedAction)
}
