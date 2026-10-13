// Code generated from compiler-owned workspace lowering; DO NOT EDIT.
package gooo_workspace

//gooo:generated:start id="gooo://cel-example/access-decision/v1" kind="entity"
type GoooRecord3e8b96dec6aae778872339a77f4b0383eb3a480edd460b3a8367cd9514678de9 struct {
	GoooFieldcb8cc465aec016a10f2d3636efad04cb149513e0a42056a241af55fa6ade1479 bool   `json:"allowed"`
	GoooField6f3ac4da1bf03eedfa43087abe57f103959c8d333d8916764fbef6a466a9c64d bool   `json:"evaluation_error"`
	GoooField84ba8c45ca57451c46aa6b86e9a09e3c1d3d8f08c6d524838b57d34e1dacd58d string `json:"result"`
	GoooFieldc3d5715a4d7c6103b3ac4baeccf89ab946ed6fffb351ef59466246623cbb4f0d bool   `json:"group_matched"`
	GoooField5885535990bf010baef728121786667125f0e673eac71335c150808a2b2dfa97 bool   `json:"principal_matched"`
	GoooField42198dda594e75b7f6d845f8e073f6749a15da67541e837217a7c7dda43171b8 bool   `json:"group_missing"`
	GoooField27f50d14ea751a7fd26c1463651b945394f683d0f16eecc662ef2bce4f6f2336 bool   `json:"principal_defaulted"`
	GoooFieldba009cd340b1608ab7fe33112b1411124eaa03ed1e5f1d44682af87fd2b03a43 string `json:"effective_principal"`
}

//gooo:generated:end id="gooo://cel-example/access-decision/v1" kind="entity"
//gooo:generated:start id="gooo://cel-example/scalar-access" kind="activity"
func GoooPackage0ActivityScalarAccess(input0 bool, input1 string, input2 bool, input3 string) GoooRecord3e8b96dec6aae778872339a77f4b0383eb3a480edd460b3a8367cd9514678de9 {

	var effectivePrincipal = input3
	if !input2 {
		effectivePrincipal = ""
	}
	var groupMatched = input0 && input1 == "admin"
	var principalMatched = effectivePrincipal == "user:me@acme.co"
	var allowed = groupMatched || principalMatched
	var evaluationError = !input0 && !principalMatched
	var result = "DENY"
	if evaluationError {
		result = "ERROR_NO_SUCH_KEY_GROUP"
	}
	if allowed {
		result = "ALLOW"
	}
	return GoooRecord3e8b96dec6aae778872339a77f4b0383eb3a480edd460b3a8367cd9514678de9{GoooFieldcb8cc465aec016a10f2d3636efad04cb149513e0a42056a241af55fa6ade1479: allowed, GoooField6f3ac4da1bf03eedfa43087abe57f103959c8d333d8916764fbef6a466a9c64d: evaluationError, GoooField84ba8c45ca57451c46aa6b86e9a09e3c1d3d8f08c6d524838b57d34e1dacd58d: result, GoooFieldc3d5715a4d7c6103b3ac4baeccf89ab946ed6fffb351ef59466246623cbb4f0d: groupMatched, GoooField5885535990bf010baef728121786667125f0e673eac71335c150808a2b2dfa97: principalMatched, GoooField42198dda594e75b7f6d845f8e073f6749a15da67541e837217a7c7dda43171b8: !input0, GoooField27f50d14ea751a7fd26c1463651b945394f683d0f16eecc662ef2bce4f6f2336: !input2, GoooFieldba009cd340b1608ab7fe33112b1411124eaa03ed1e5f1d44682af87fd2b03a43: effectivePrincipal}

}

//gooo:generated:end id="gooo://cel-example/scalar-access" kind="activity"
