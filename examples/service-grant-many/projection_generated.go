package servicegrantmany

//gooo:generated:start id="gooo://service-policy/checked-refinement/decision/v1" kind="entity"
type GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64 struct {
	GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704 string `json:"domain_status"`
	GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899 string `json:"invalid_field"`
	GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38 string `json:"status"`
	GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c string `json:"rule_id"`
	GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f bool   `json:"supported_subset"`
	GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073 bool   `json:"has_witness"`
	GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea string `json:"witness_type"`
	GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9 string `json:"witness_name"`
	GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5 bool   `json:"before_match"`
	GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068 bool   `json:"after_match"`
}

//gooo:generated:end id="gooo://service-policy/checked-refinement/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/multi-required/decision/v1" kind="entity"
type GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6 struct {
	GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b string `json:"status"`
	GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27 string `json:"reason"`
	GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1 string `json:"invalid_field"`
	GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e int64  `json:"invalid_index"`
	GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8 int64  `json:"first_counterexample"`
	GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb int64  `json:"request_count"`
	GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04 int64  `json:"checked_count"`
	GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f bool   `json:"all_preserved"`
	GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169 bool   `json:"supported_subset"`
	GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09 string `json:"subset_rule_id"`
	GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60 bool   `json:"counterexample_before_match"`
	GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a bool   `json:"counterexample_after_match"`
}

//gooo:generated:end id="gooo://service-policy/multi-required/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/multi-required/input/v1" kind="entity"
type GoooRecord7201cd023e75bfb95e776ef109d81255061cbcb83c9d60f0777ebf90f26b68bf struct {
	GoooField2da56a6c64b7fa99c74ec0edbff173ac72722c028f47396343ff7d8195f30f03 string   `json:"before"`
	GoooField39fd9cae00230e526199c131c2f320dd9209cc92ea789156b4b15ceea0f95251 string   `json:"after"`
	GoooFieldf24f614c675e868faab41da4399c35e962b3476af4b13dd176493ee11d951fc0 []string `json:"required_types"`
	GoooField12ec6d0b7e86f8f336fb3de209ceff8ccd17ee8ccd7cf8a017d172704139afd9 []string `json:"required_names"`
}

//gooo:generated:end id="gooo://service-policy/multi-required/input/v1" kind="entity"
//gooo:generated:start id="gooo://service-refinement/decision/v1" kind="entity"
type GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367 struct {
	GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354 string `json:"status"`
	GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454 string `json:"rule_id"`
	GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711 bool   `json:"supported_subset"`
	GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d bool   `json:"has_witness"`
	GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd string `json:"witness_type"`
	GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6 string `json:"witness_name"`
	GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc bool   `json:"before_match"`
	GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef bool   `json:"after_match"`
}

//gooo:generated:end id="gooo://service-refinement/decision/v1" kind="entity"

//gooo:generated:start id="gooo://service-policy/multi-required/assess/v1" kind="activity"
func GoooComposedActivity0(input GoooRecord7201cd023e75bfb95e776ef109d81255061cbcb83c9d60f0777ebf90f26b68bf) GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6 {

	var count = int64(len(input.GoooFieldf24f614c675e868faab41da4399c35e962b3476af4b13dd176493ee11d951fc0))
	if !GoooComposedActivity0Call2(input.GoooField2da56a6c64b7fa99c74ec0edbff173ac72722c028f47396343ff7d8195f30f03) {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "INVALID_CANONICAL_INPUT", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "before", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: 0, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}
	if !GoooComposedActivity0Call2(input.GoooField39fd9cae00230e526199c131c2f320dd9209cc92ea789156b4b15ceea0f95251) {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "INVALID_CANONICAL_INPUT", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "after", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: 0, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}
	if count != int64(len(input.GoooField12ec6d0b7e86f8f336fb3de209ceff8ccd17ee8ccd7cf8a017d172704139afd9)) {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "COLUMN_LENGTH_MISMATCH", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "required_names", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: 0, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}
	if count == 0 {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "EMPTY_REQUIRED_REQUESTS", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "required_types", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: 0, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: 0, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}

	if count > 256 {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "REQUIRED_REQUEST_LIMIT_EXCEEDED", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "required_types", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: 0, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}
	var index int64 = 0
	var invalid int64 = -1
	var invalidField = ""
	var invalidReason = ""
	for _, typ := range input.GoooFieldf24f614c675e868faab41da4399c35e962b3476af4b13dd176493ee11d951fc0 {
		var name = input.GoooField12ec6d0b7e86f8f336fb3de209ceff8ccd17ee8ccd7cf8a017d172704139afd9[index]
		var validType = GoooComposedActivity0Call0(typ)
		var validName = GoooComposedActivity0Call1(name)
		if invalid == -1 {
			if !validType || typ == "" {
				invalid = index
				invalidField = "required_types"
				invalidReason = "INVALID_CANONICAL_INPUT"
				if typ == "" {
					invalidReason = "EMPTY_REQUIRED_REQUEST"
				}
			} else {
				if !validName || name == "" {
					invalid = index
					invalidField = "required_names"
					invalidReason = "INVALID_CANONICAL_INPUT"
					if name == "" {
						invalidReason = "EMPTY_REQUIRED_REQUEST"
					}
				}
			}
		}
		index = index + 1
	}
	if invalid != -1 {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "INVALID_INPUT", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: invalidReason, GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: invalidField, GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: invalid, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: index, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}
	}
	var first int64 = -1
	var firstBefore = false
	var firstAfter = false
	var failureReason = ""
	var position int64 = 0
	for _, typ := range input.GoooFieldf24f614c675e868faab41da4399c35e962b3476af4b13dd176493ee11d951fc0 {
		var name = input.GoooField12ec6d0b7e86f8f336fb3de209ceff8ccd17ee8ccd7cf8a017d172704139afd9[position]
		var before = GoooComposedActivity0Call3(input.GoooField2da56a6c64b7fa99c74ec0edbff173ac72722c028f47396343ff7d8195f30f03, typ, name)
		var after = GoooComposedActivity0Call3(input.GoooField39fd9cae00230e526199c131c2f320dd9209cc92ea789156b4b15ceea0f95251, typ, name)
		if first == -1 && (!before || !after) {
			first = position
			firstBefore = before
			firstAfter = after
			failureReason = "REQUIRED_REQUEST_REMOVED"
			if !before {
				failureReason = "CURRENT_REQUIRED_REQUEST_DENIED"
			}
		}
		position = position + 1
	}
	var preserved = first == -1
	var grant = GoooComposedActivity0Call5(input.GoooField2da56a6c64b7fa99c74ec0edbff173ac72722c028f47396343ff7d8195f30f03, input.GoooField39fd9cae00230e526199c131c2f320dd9209cc92ea789156b4b15ceea0f95251)
	if grant.GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704 != "VALID_INPUT" || grant.GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899 != "" {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "REFUSED", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "DEPENDENCY_RESULT_INVALID", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: first, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: index, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: preserved, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: firstBefore, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: firstAfter}
	}
	if !grant.GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f {
		var reason = "REFINEMENT_UNPROVEN"
		if grant.GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38 == "WIDENING_WITNESS" {
			reason = "WIDENING_WITNESS"
		}
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "REFUSED", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: reason, GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: first, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: index, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: preserved, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: false, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: "", GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: firstBefore, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: firstAfter}
	}
	if !preserved {
		return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "REFUSED", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: failureReason, GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: first, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: index, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: false, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: true, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c, GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: firstBefore, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: firstAfter}
	}
	return GoooRecord78411208777237ed5b6b24d0ea7b8c07211293e8000f2ece2d19fc5dc05f3db6{GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b: "ELIGIBLE", GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27: "SUBSET_AND_ALL_REQUIRED_REQUESTS_PRESERVED", GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1: "", GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e: -1, GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8: -1, GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb: count, GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04: index, GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f: true, GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169: true, GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09: grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c, GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60: false, GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a: false}

}
func GoooComposedActivity0Call0(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if n == 0 {
		return true
	}
	var first = int64(input[0])
	if !((first >= 65 && first <= 90) || (first >= 97 && first <= 122)) {
		return false
	}
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		if offset < n {
			var b = int64(input[offset])
			var alpha = (b >= 65 && b <= 90) || (b >= 97 && b <= 122)
			var digit = b >= 48 && b <= 57
			if !alpha && !digit && b != 43 && b != 45 && b != 46 {
				return false
			}
		}
	}
	return true

}
func GoooComposedActivity0Call1(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if n == 0 {
		return true
	}
	var previousDot = true
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		if offset < n {
			var b = int64(input[offset])
			if b == 46 {
				if previousDot {
					return false
				}
				previousDot = true
			} else {
				var alpha = (b >= 65 && b <= 90) || (b >= 97 && b <= 122)
				var digit = b >= 48 && b <= 57
				if !alpha && !digit && b != 45 && b != 95 {
					return false
				}
				previousDot = false
			}
		}
	}
	return !previousDot

}
func GoooComposedActivity0Call2(input string) bool {

	var n = int64(len(input))
	if n > 128 {
		return false
	}
	if input == "" || input == "*" {
		return true
	}
	var split int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= n {
			if input[offset:end] == "://" {
				if split == -1 {
					split = offset
				}
			}
		}
	}
	if split <= 0 {
		return false
	}
	var typ = input[0:split]
	if !GoooComposedActivity0Call0(typ) {
		return false
	}
	var start int64 = split + 3
	var part = input[start:n]
	var size = int64(len(part))
	if size == 0 {
		return false
	}
	if part == "*" {
		return true
	}
	if size >= 2 {
		if part[0:2] == "*." {
			part = part[2:size]
		} else {
			var tail = size - 2
			if part[tail:size] == ".*" {
				part = part[0:tail]
			}
		}
	}
	if part == "" {
		return false
	}
	return GoooComposedActivity0Call1(part)

}
func GoooComposedActivity0Call3(input0 string, input1 string, input2 string) bool {

	if input0 == "" || input1 == "" || input2 == "" {
		return false
	}
	if input0 == "*" {
		return true
	}
	var split int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= int64(len(input0)) {
			if input0[offset:end] == "://" {
				if split == -1 {
					split = offset
				}
			}
		}
	}
	if split < 0 {
		return false
	}
	var nameStart int64 = split + 3
	var patternType = input0[0:split]
	var patternName = input0[nameStart:int64(len(input0))]
	if patternType != input1 || patternName == "" {
		return false
	}
	if patternName == "*" {
		return true
	}
	var patternLength = int64(len(patternName))
	var requestLength = int64(len(input2))
	if patternLength >= 2 {
		if patternName[0:2] == "*." {
			var suffix = patternName[1:patternLength]
			var suffixLength = int64(len(suffix))
			if requestLength < suffixLength {
				return false
			}
			var suffixStart = requestLength - suffixLength
			return input2[suffixStart:requestLength] == suffix
		}
		var tailStart = patternLength - 2
		if patternName[tailStart:patternLength] == ".*" {
			var prefixEnd = patternLength - 1
			var prefix = patternName[0:prefixEnd]
			if requestLength < prefixEnd {
				return false
			}
			return input2[0:prefixEnd] == prefix
		}
	}
	return input2 == patternName

}
func GoooComposedActivity0Call4(input0 string, input1 string) GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367 {

	var rule = ""
	if input1 == "" {
		rule = "CANDIDATE_EMPTY"
	} else {
		if input0 == input1 {
			rule = "IDENTICAL_PATTERNS"
		} else {
			if input0 == "*" {
				rule = "OLD_GLOBAL"
			}
		}
	}
	var oldType = ""
	var candidateType = ""
	var oldBase = ""
	var candidateBase = ""
	var oldKind = "NONE"
	var candidateKind = "NONE"
	var oldSplit int64 = -1
	var candidateSplit int64 = -1
	var offset int64 = 0
	for offset = 0; offset < 128; offset = offset + 1 {
		var end int64 = offset + 3
		if end <= int64(len(input0)) {
			if input0[offset:end] == "://" {
				if oldSplit == -1 {
					oldSplit = offset
				}
			}
		}
		if end <= int64(len(input1)) {
			if input1[offset:end] == "://" {
				if candidateSplit == -1 {
					candidateSplit = offset
				}
			}
		}
	}
	if oldSplit > 0 {
		oldType = input0[0:oldSplit]
		var start int64 = oldSplit + 3
		oldBase = input0[start:int64(len(input0))]
		oldKind = "EXACT"
		if oldBase == "*" {
			oldKind = "TYPED_ALL"
		} else {
			var n = int64(len(oldBase))
			if n >= 2 {
				if oldBase[0:2] == "*." {
					oldKind = "SUFFIX"
					oldBase = oldBase[2:n]
				} else {
					var tail = n - 2
					if oldBase[tail:n] == ".*" {
						oldKind = "PREFIX"
						oldBase = oldBase[0:tail]
					}
				}
			}
		}
	}
	if candidateSplit > 0 {
		candidateType = input1[0:candidateSplit]
		var start int64 = candidateSplit + 3
		candidateBase = input1[start:int64(len(input1))]
		candidateKind = "EXACT"
		if candidateBase == "*" {
			candidateKind = "TYPED_ALL"
		} else {
			var n = int64(len(candidateBase))
			if n >= 2 {
				if candidateBase[0:2] == "*." {
					candidateKind = "SUFFIX"
					candidateBase = candidateBase[2:n]
				} else {
					var tail = n - 2
					if candidateBase[tail:n] == ".*" {
						candidateKind = "PREFIX"
						candidateBase = candidateBase[0:tail]
					}
				}
			}
		}
	}
	if rule == "" {
		if oldKind == "TYPED_ALL" && candidateType == oldType {
			rule = "OLD_TYPED_ALL"
		}
	}
	if rule == "" {
		if candidateKind == "EXACT" {
			if GoooComposedActivity0Call3(input0, candidateType, candidateBase) {
				rule = "EXACT_CANDIDATE_ACCEPTED_BY_OLD"
			}
		}
	}
	if rule == "" && oldType == candidateType {
		if oldKind == "PREFIX" && candidateKind == "PREFIX" {
			if oldBase == candidateBase {
				rule = "PREFIX_HIERARCHY"
			} else {
				var end = int64(len(oldBase)) + 1
				if int64(len(candidateBase)) >= end {
					if candidateBase[0:end] == oldBase+"." {
						rule = "PREFIX_HIERARCHY"
					}
				}
			}
		}
		if oldKind == "SUFFIX" && candidateKind == "SUFFIX" {
			if oldBase == candidateBase {
				rule = "SUFFIX_HIERARCHY"
			} else {
				var n = int64(len(candidateBase))
				var size = int64(len(oldBase)) + 1
				if n >= size {
					var start = n - size
					if candidateBase[start:n] == "."+oldBase {
						rule = "SUFFIX_HIERARCHY"
					}
				}
			}
		}
	}
	var status = "UNPROVEN"
	var supported = false
	if rule != "" {
		status = "SUPPORTED_SUBSET"
		supported = true
	}
	var witnessed = false
	var witnessType = ""
	var witnessName = ""
	var witnessBefore = false
	var witnessAfter = false
	if rule == "" {
		var firstType = candidateType
		var secondType = candidateType
		var firstName = ""
		var secondName = ""
		if input1 == "*" {
			firstType = "a"
			secondType = "b"
			firstName = "x"
			secondName = "x"
		}
		if candidateKind == "TYPED_ALL" {
			firstName = "x"
			secondName = "y"
		}
		if candidateKind == "EXACT" {
			firstName = candidateBase
		}
		if candidateKind == "PREFIX" {
			firstName = candidateBase + ".x"
			secondName = candidateBase + ".y"
		}
		if candidateKind == "SUFFIX" {
			firstName = "x." + candidateBase
			secondName = "y." + candidateBase
		}
		if firstName != "" {
			var before = GoooComposedActivity0Call3(input0, firstType, firstName)
			var after = GoooComposedActivity0Call3(input1, firstType, firstName)
			if after && !before {
				witnessed = true
				witnessType = firstType
				witnessName = firstName
				witnessBefore = before
				witnessAfter = after
			}
		}
		if !witnessed && secondName != "" {
			var before = GoooComposedActivity0Call3(input0, secondType, secondName)
			var after = GoooComposedActivity0Call3(input1, secondType, secondName)
			if after && !before {
				witnessed = true
				witnessType = secondType
				witnessName = secondName
				witnessBefore = before
				witnessAfter = after
			}
		}
		if witnessed {
			status = "WIDENING_WITNESS"
			rule = "CHECKED_CANDIDATE_REQUEST"
		}
	}
	return GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367{GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354: status, GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454: rule, GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711: supported, GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d: witnessed, GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd: witnessType, GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6: witnessName, GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc: witnessBefore, GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef: witnessAfter}

}
func GoooComposedActivity0Call5(input0 string, input1 string) GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64 {

	if !GoooComposedActivity0Call2(input0) {
		return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "INVALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "before", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: "INVALID_INPUT", GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: "", GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: false, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: false, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: "", GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: "", GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: false, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: false}
	}
	if !GoooComposedActivity0Call2(input1) {
		return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "INVALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "after", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: "INVALID_INPUT", GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: "", GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: false, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: false, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: "", GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: "", GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: false, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: false}
	}
	var result = GoooComposedActivity0Call4(input0, input1)
	return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "VALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: result.GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354, GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: result.GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454, GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: result.GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: result.GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: result.GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd, GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: result.GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6, GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: result.GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: result.GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef}

}

//gooo:generated:end id="gooo://service-policy/multi-required/assess/v1" kind="activity"
