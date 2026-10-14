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
//gooo:generated:start id="gooo://api-policy-revision/change-summary/v1" kind="entity"
type GoooRecord160d75b903ca4d9fb4bc16516d2193362785dd22e8bd755642e2fca4f0cb677b struct {
	GoooFieldd65699a430c669973eebac8e88b227b292b3e4b77fb668c4934a4ecf7b836062 string `json:"classification"`
	GoooFieldbeb8d0481f18e7bd3c576ce2e857fd168fe75fb75a5940cfd84f14f9e145f690 int64  `json:"changed_count"`
}

//gooo:generated:end id="gooo://api-policy-revision/change-summary/v1" kind="entity"
//gooo:generated:start id="gooo://api-policy-revision/difference-flags/v1" kind="entity"
type GoooRecordb632d83a3b587b104c85621d6098352c0b9656ff10ea47bbf2d308ddbb100653 struct {
	GoooField1d8a8b68f317d4bdf8b20d97740a1456e30ec1b8098ca9adb398642646e965ef bool `json:"allowed"`
	GoooFieldc9610cdb7b007a9d774c82e75e64e42f155ecdc1b8c786ebd2c22d6c35a8ac3c bool `json:"reason"`
	GoooFieldad82e725d8b2edc02b617b81bfb90c3eeb87ccf94010a431c64b95f542c9bebb bool `json:"authenticated"`
	GoooFielda3105bca1e8852e8d1c57d3f047d87771f1ff8c3e4a2795a8ec654b0ac5813fe bool `json:"tenant_present"`
	GoooField9df5a8b598fe34fe21adf53a5e9eafa78232b0e9ae6e2aceda3f17c81b8d64d0 bool `json:"tenant_matched"`
	GoooFieldc21394f11c6b9024c46cc98a582bac3314b1b0994297b7d863b4476930d68354 bool `json:"action_present"`
	GoooField1c231f97ec2907d13d36327bfb4e351a586f8af781ec6106f7c9cd360b26854f bool `json:"action_matched"`
	GoooField74f94493ad3ea3fc6b89d781b4aac71de7fd9ddda0293122e5bb8fd90a023f94 bool `json:"access_allowed"`
	GoooField7a09d07439d4744cc3d5cf361c096844ee59c3a6003e490529d0dd7d0ec83f2a bool `json:"access_error"`
	GoooFieldac0b642cdf4c4da7af724b3ef312310320811cbc8f7c2606b58ce294cd94b49e bool `json:"access_result"`
	GoooField8daaa1194f8f74234dcf9f57862df3279eabd7034a0f2c09d7584f69d3f03524 bool `json:"group_matched"`
	GoooFielde987121cd36c8a090a1463190b8ed922e421c85adeee4241e6ab854f37ca7974 bool `json:"principal_matched"`
	GoooFieldf55d1805bd38d2e655b4efb5e7711b5c2661607e81e14af8d9c6f5c1d15d22ad bool `json:"group_missing"`
	GoooField31503713a87f5b6aa8350bd2e3238b00460da643d795c5fcb752e7bab5b3c9df bool `json:"principal_defaulted"`
	GoooField73e1d3786f6a5a55a8473f5d0c289a2b65fd7e87eafcbbfdce93bc99d06fa3ca bool `json:"effective_principal"`
}

//gooo:generated:end id="gooo://api-policy-revision/difference-flags/v1" kind="entity"
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
//gooo:generated:start id="gooo://api-policy-revision/change-summary" kind="activity"
func GoooPackage3ActivityChangeSummary(input0 GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f, input1 GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f) GoooRecord160d75b903ca4d9fb4bc16516d2193362785dd22e8bd755642e2fca4f0cb677b {

	var differences = GoooPackage2ActivityDifferenceFlags(input0, input1)
	var changedCount int64 = 0
	if differences.GoooField1d8a8b68f317d4bdf8b20d97740a1456e30ec1b8098ca9adb398642646e965ef {
		changedCount = changedCount + 1
	}
	if differences.GoooFieldc9610cdb7b007a9d774c82e75e64e42f155ecdc1b8c786ebd2c22d6c35a8ac3c {
		changedCount = changedCount + 1
	}
	if differences.GoooFieldad82e725d8b2edc02b617b81bfb90c3eeb87ccf94010a431c64b95f542c9bebb {
		changedCount = changedCount + 1
	}
	if differences.GoooFielda3105bca1e8852e8d1c57d3f047d87771f1ff8c3e4a2795a8ec654b0ac5813fe {
		changedCount = changedCount + 1
	}
	if differences.GoooField9df5a8b598fe34fe21adf53a5e9eafa78232b0e9ae6e2aceda3f17c81b8d64d0 {
		changedCount = changedCount + 1
	}
	if differences.GoooFieldc21394f11c6b9024c46cc98a582bac3314b1b0994297b7d863b4476930d68354 {
		changedCount = changedCount + 1
	}
	if differences.GoooField1c231f97ec2907d13d36327bfb4e351a586f8af781ec6106f7c9cd360b26854f {
		changedCount = changedCount + 1
	}
	if differences.GoooField74f94493ad3ea3fc6b89d781b4aac71de7fd9ddda0293122e5bb8fd90a023f94 {
		changedCount = changedCount + 1
	}
	if differences.GoooField7a09d07439d4744cc3d5cf361c096844ee59c3a6003e490529d0dd7d0ec83f2a {
		changedCount = changedCount + 1
	}
	if differences.GoooFieldac0b642cdf4c4da7af724b3ef312310320811cbc8f7c2606b58ce294cd94b49e {
		changedCount = changedCount + 1
	}
	if differences.GoooField8daaa1194f8f74234dcf9f57862df3279eabd7034a0f2c09d7584f69d3f03524 {
		changedCount = changedCount + 1
	}
	if differences.GoooFielde987121cd36c8a090a1463190b8ed922e421c85adeee4241e6ab854f37ca7974 {
		changedCount = changedCount + 1
	}
	if differences.GoooFieldf55d1805bd38d2e655b4efb5e7711b5c2661607e81e14af8d9c6f5c1d15d22ad {
		changedCount = changedCount + 1
	}
	if differences.GoooField31503713a87f5b6aa8350bd2e3238b00460da643d795c5fcb752e7bab5b3c9df {
		changedCount = changedCount + 1
	}
	if differences.GoooField73e1d3786f6a5a55a8473f5d0c289a2b65fd7e87eafcbbfdce93bc99d06fa3ca {
		changedCount = changedCount + 1
	}
	var classification = "UNCHANGED"
	if !input0.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c && input1.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c {
		classification = "NEWLY_ALLOWED"
	} else {
		if input0.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c && !input1.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c {
			classification = "NEWLY_DENIED"
		} else {
			if differences.GoooFieldc9610cdb7b007a9d774c82e75e64e42f155ecdc1b8c786ebd2c22d6c35a8ac3c {
				classification = "REASON_CHANGED"
			} else {
				if changedCount > 0 {
					classification = "OBSERVATIONS_CHANGED"
				}
			}
		}
	}
	return GoooRecord160d75b903ca4d9fb4bc16516d2193362785dd22e8bd755642e2fca4f0cb677b{GoooFieldd65699a430c669973eebac8e88b227b292b3e4b77fb668c4934a4ecf7b836062: classification, GoooFieldbeb8d0481f18e7bd3c576ce2e857fd168fe75fb75a5940cfd84f14f9e145f690: changedCount}

}

//gooo:generated:end id="gooo://api-policy-revision/change-summary" kind="activity"
//gooo:generated:start id="gooo://api-policy-revision/difference-flags" kind="activity"
func GoooPackage2ActivityDifferenceFlags(input0 GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f, input1 GoooRecord81490f6f92835ce73a5380fe540edb6ab513336bc5a5f3ff4565b272cba47d8f) GoooRecordb632d83a3b587b104c85621d6098352c0b9656ff10ea47bbf2d308ddbb100653 {

	return GoooRecordb632d83a3b587b104c85621d6098352c0b9656ff10ea47bbf2d308ddbb100653{GoooField1d8a8b68f317d4bdf8b20d97740a1456e30ec1b8098ca9adb398642646e965ef: input0.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c != input1.GoooFieldb4d1c305b959d8795a5b8471b887de42c3208b1f4b972449d08762db866aeb3c, GoooFieldc9610cdb7b007a9d774c82e75e64e42f155ecdc1b8c786ebd2c22d6c35a8ac3c: input0.GoooField9070b1876901c9fa3b251685ae4e93389798cfc93968bd778d4ac018175ecece != input1.GoooField9070b1876901c9fa3b251685ae4e93389798cfc93968bd778d4ac018175ecece, GoooFieldad82e725d8b2edc02b617b81bfb90c3eeb87ccf94010a431c64b95f542c9bebb: input0.GoooField9196b47bc3ddf9c447479de09ab0a173f7429c384de6fd22abae0e3706ae790d != input1.GoooField9196b47bc3ddf9c447479de09ab0a173f7429c384de6fd22abae0e3706ae790d, GoooFielda3105bca1e8852e8d1c57d3f047d87771f1ff8c3e4a2795a8ec654b0ac5813fe: input0.GoooField61ff71662beb719376cd734469194d3694d1bbaedde5ab50bcd5590e96d4c0ea != input1.GoooField61ff71662beb719376cd734469194d3694d1bbaedde5ab50bcd5590e96d4c0ea, GoooField9df5a8b598fe34fe21adf53a5e9eafa78232b0e9ae6e2aceda3f17c81b8d64d0: input0.GoooFielda52a2b58f144e7f3e39c7aa6f3fa4c474fe71b17c86170d406926f5d15838aa6 != input1.GoooFielda52a2b58f144e7f3e39c7aa6f3fa4c474fe71b17c86170d406926f5d15838aa6, GoooFieldc21394f11c6b9024c46cc98a582bac3314b1b0994297b7d863b4476930d68354: input0.GoooFieldc767880d124ed03ee076c8e5bce66122c771a7c36516db669a1d4242b447c716 != input1.GoooFieldc767880d124ed03ee076c8e5bce66122c771a7c36516db669a1d4242b447c716, GoooField1c231f97ec2907d13d36327bfb4e351a586f8af781ec6106f7c9cd360b26854f: input0.GoooField2273ef110c1d5be56ac4721a14b55392b34e57989a26c2fb0e8edef6d1a15faa != input1.GoooField2273ef110c1d5be56ac4721a14b55392b34e57989a26c2fb0e8edef6d1a15faa, GoooField74f94493ad3ea3fc6b89d781b4aac71de7fd9ddda0293122e5bb8fd90a023f94: input0.GoooField7990f2c3352eb47f9f44c8bb9ddef3b3e63bddf3d9b58f7b3757e698598bd813 != input1.GoooField7990f2c3352eb47f9f44c8bb9ddef3b3e63bddf3d9b58f7b3757e698598bd813, GoooField7a09d07439d4744cc3d5cf361c096844ee59c3a6003e490529d0dd7d0ec83f2a: input0.GoooField8bcf16a4750a95a3558de5b82b9d54c33a03c0bb42a107a22450963a6790b2ef != input1.GoooField8bcf16a4750a95a3558de5b82b9d54c33a03c0bb42a107a22450963a6790b2ef, GoooFieldac0b642cdf4c4da7af724b3ef312310320811cbc8f7c2606b58ce294cd94b49e: input0.GoooFielde70e95a986cd4c031e9b573f90ba8d8a07cf87ff4a9246a65f0af1e2468dcd9f != input1.GoooFielde70e95a986cd4c031e9b573f90ba8d8a07cf87ff4a9246a65f0af1e2468dcd9f, GoooField8daaa1194f8f74234dcf9f57862df3279eabd7034a0f2c09d7584f69d3f03524: input0.GoooField9bfb03bbd4ad885d88cffc24db8807c2b0205626172ad53645d2e0786bb34ebb != input1.GoooField9bfb03bbd4ad885d88cffc24db8807c2b0205626172ad53645d2e0786bb34ebb, GoooFielde987121cd36c8a090a1463190b8ed922e421c85adeee4241e6ab854f37ca7974: input0.GoooField5ced9a7a2a2120077996c77030e20ac27e5c8936dd8bb70c79eb37d55dc61a2e != input1.GoooField5ced9a7a2a2120077996c77030e20ac27e5c8936dd8bb70c79eb37d55dc61a2e, GoooFieldf55d1805bd38d2e655b4efb5e7711b5c2661607e81e14af8d9c6f5c1d15d22ad: input0.GoooField1a43d0816df198736d9ea1d4a9681bcfed6ffa3432eab0b050609ba2e02d625b != input1.GoooField1a43d0816df198736d9ea1d4a9681bcfed6ffa3432eab0b050609ba2e02d625b, GoooField31503713a87f5b6aa8350bd2e3238b00460da643d795c5fcb752e7bab5b3c9df: input0.GoooFielddcf21a43f49bc529874437e69e41600c5f4064b0d8f8dc18a06bcd535184649a != input1.GoooFielddcf21a43f49bc529874437e69e41600c5f4064b0d8f8dc18a06bcd535184649a, GoooField73e1d3786f6a5a55a8473f5d0c289a2b65fd7e87eafcbbfdce93bc99d06fa3ca: input0.GoooFieldca6917f1263ec85d1d6424d56fbbaa6b6f1dad686999a222aff26796372e28ba != input1.GoooFieldca6917f1263ec85d1d6424d56fbbaa6b6f1dad686999a222aff26796372e28ba}

}

//gooo:generated:end id="gooo://api-policy-revision/difference-flags" kind="activity"
