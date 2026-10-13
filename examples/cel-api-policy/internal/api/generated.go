// Code generated from compiler-owned workspace lowering; DO NOT EDIT.
package gooo_workspace

//gooo:generated:start id="gooo://api-example/decision/v1" kind="entity"
type GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f struct {
	GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c bool   `json:"allowed"`
	GoooField9070b1876901c9fa3b251685ae4e93389798cfc93968bd778d4ac018175ecece string `json:"reason"`
	GoooField9196b47bc3ddf9c447479de09ab0a173f7429c384de6fd22abae0e3706ae790d bool   `json:"authenticated"`
	GoooField61ff71662beb719376cd734469194d3694d1bbaedde5ab50bcd5590e96d4c0ea bool   `json:"tenant_present"`
	GoooFielda52a2b58f144e7f3e39c7aa6f3fa4c474fe71b17c86170d406926f5d15838aa6 bool   `json:"tenant_matched"`
	GoooFieldc767880d124ed03ee076c8e5bce66122c771a7c36516db669a1d4242b447c716 bool   `json:"action_present"`
	GoooField2273ef110c1d5be56ac4721a14b55392b34e57989a26c2fb0e8edef6d1a15faa bool   `json:"action_matched"`
	GoooField7990f2c3352eb47f9f44c8bb9ddef3b3e63bddf3d9b58f7b3757e698598bd813 bool   `json:"access_allowed"`
	GoooField8bcf16a4750a95a3558de5b82b9d54c33a03c0bb42a107a22450963a6790b2ef bool   `json:"access_error"`
	GoooFielde70e95a986cd4c031e9b573f90ba8d8a07cf87ff4a9246a65f0af1e2468dcd9f string `json:"access_result"`
	GoooField9bfb03bbd4ad885d88cffc24db8807c2b0205626172ad53645d2e0786bb34ebb bool   `json:"group_matched"`
	GoooField5ced9a7a2a2120077996c77030e20ac27e5c8936dd8bb70c79eb37d55dc61a2e bool   `json:"principal_matched"`
	GoooField1a43d0816df198736d9ea1d4a9681bcfed6ffa3432eab0b050609ba2e02d625b bool   `json:"group_missing"`
	GoooFielddcf21a43f49bc529874437e69e41600c5f4064b0d8f8dc18a06bcd535184649a bool   `json:"principal_defaulted"`
	GoooFieldca6917f1263ec85d1d6424d56fbbaa6b6f1dad686999a222aff26796372e28ba string `json:"effective_principal"`
}

//gooo:generated:end id="gooo://api-example/decision/v1" kind="entity"
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
//gooo:generated:start id="gooo://api-example/api-access" kind="activity"
func GoooPackage1ActivityAPIAccess(input0 bool, input1 string, input2 bool, input3 string, input4 bool, input5 string, input6 string, input7 string, input8 string) GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f {

	var access = GoooPackage0ActivityScalarAccess(input0, input1, input2, input3)
	var tenantPresent = input5 != "" && input6 != ""
	var tenantMatched = input5 == input6
	var actionPresent = input7 != "" && input8 != ""
	var actionMatched = input7 == input8
	var allowed = input4 && tenantPresent && tenantMatched && actionPresent && actionMatched && access.GoooFieldcb8cc465aec016a10f2d3636efad04cb149513e0a42056a241af55fa6ade1479
	var reason = "ALLOW"
	if !input4 {
		reason = "NOT_AUTHENTICATED"
	} else {
		if !tenantPresent {
			reason = "TENANT_UNSPECIFIED"
		} else {
			if !tenantMatched {
				reason = "TENANT_MISMATCH"
			} else {
				if !actionPresent {
					reason = "ACTION_UNSPECIFIED"
				} else {
					if !actionMatched {
						reason = "ACTION_MISMATCH"
					} else {
						if access.GoooField6f3ac4da1bf03eedfa43087abe57f103959c8d333d8916764fbef6a466a9c64d {
							reason = "ACCESS_ERROR"
						} else {
							if !access.GoooFieldcb8cc465aec016a10f2d3636efad04cb149513e0a42056a241af55fa6ade1479 {
								reason = "ACCESS_DENIED"
							}
						}
					}
				}
			}
		}
	}
	return GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f{GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c: allowed, GoooField9070b1876901c9fa3b251685ae4e93389798cfc93968bd778d4ac018175ecece: reason, GoooField9196b47bc3ddf9c447479de09ab0a173f7429c384de6fd22abae0e3706ae790d: input4, GoooField61ff71662beb719376cd734469194d3694d1bbaedde5ab50bcd5590e96d4c0ea: tenantPresent, GoooFielda52a2b58f144e7f3e39c7aa6f3fa4c474fe71b17c86170d406926f5d15838aa6: tenantMatched, GoooFieldc767880d124ed03ee076c8e5bce66122c771a7c36516db669a1d4242b447c716: actionPresent, GoooField2273ef110c1d5be56ac4721a14b55392b34e57989a26c2fb0e8edef6d1a15faa: actionMatched, GoooField7990f2c3352eb47f9f44c8bb9ddef3b3e63bddf3d9b58f7b3757e698598bd813: access.GoooFieldcb8cc465aec016a10f2d3636efad04cb149513e0a42056a241af55fa6ade1479, GoooField8bcf16a4750a95a3558de5b82b9d54c33a03c0bb42a107a22450963a6790b2ef: access.GoooField6f3ac4da1bf03eedfa43087abe57f103959c8d333d8916764fbef6a466a9c64d, GoooFielde70e95a986cd4c031e9b573f90ba8d8a07cf87ff4a9246a65f0af1e2468dcd9f: access.GoooField84ba8c45ca57451c46aa6b86e9a09e3c1d3d8f08c6d524838b57d34e1dacd58d, GoooField9bfb03bbd4ad885d88cffc24db8807c2b0205626172ad53645d2e0786bb34ebb: access.GoooFieldc3d5715a4d7c6103b3ac4baeccf89ab946ed6fffb351ef59466246623cbb4f0d, GoooField5ced9a7a2a2120077996c77030e20ac27e5c8936dd8bb70c79eb37d55dc61a2e: access.GoooField5885535990bf010baef728121786667125f0e673eac71335c150808a2b2dfa97, GoooField1a43d0816df198736d9ea1d4a9681bcfed6ffa3432eab0b050609ba2e02d625b: access.GoooField42198dda594e75b7f6d845f8e073f6749a15da67541e837217a7c7dda43171b8, GoooFielddcf21a43f49bc529874437e69e41600c5f4064b0d8f8dc18a06bcd535184649a: access.GoooField27f50d14ea751a7fd26c1463651b945394f683d0f16eecc662ef2bce4f6f2336, GoooFieldca6917f1263ec85d1d6424d56fbbaa6b6f1dad686999a222aff26796372e28ba: access.GoooFieldba009cd340b1608ab7fe33112b1411124eaa03ed1e5f1d44682af87fd2b03a43}

}

//gooo:generated:end id="gooo://api-example/api-access" kind="activity"
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
