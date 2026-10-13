package gooo_workspace

//gooo:generated:start id="gooo://service-impact/decision/v1" kind="entity"
type GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153 struct {
	GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d bool   `json:"before_match"`
	GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219 bool   `json:"after_match"`
	GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d string `json:"change"`
}

//gooo:generated:end id="gooo://service-impact/decision/v1" kind="entity"
//gooo:generated:start id="gooo://service-policy/checked-impact/decision/v1" kind="entity"
type GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e struct {
	GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650 string `json:"domain_status"`
	GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8 string `json:"invalid_field"`
	GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf bool   `json:"before_match"`
	GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac bool   `json:"after_match"`
	GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea string `json:"change"`
}

//gooo:generated:end id="gooo://service-policy/checked-impact/decision/v1" kind="entity"
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
//gooo:generated:start id="gooo://service-policy/conditional-apply/decision/v1" kind="entity"
type GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a struct {
	GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c string `json:"status"`
	GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e string `json:"reason"`
	GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f string `json:"invalid_field"`
	GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81 bool   `json:"required_observed"`
	GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04 bool   `json:"required_before_match"`
	GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32 bool   `json:"required_after_match"`
	GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38 string `json:"subset_rule_id"`
}

//gooo:generated:end id="gooo://service-policy/conditional-apply/decision/v1" kind="entity"
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
//gooo:generated:start id="gooo://service-policy/conditional-apply/assess/v1" kind="activity"
func GoooPackage8ActivityAssessServiceGrantUpdate(input0 string, input1 string, input2 string, input3 string) GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a {

	var required = GoooPackage6ActivityCompareServiceGrantChecked(input0, input1, input2, input3)
	if required.GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650 == "INVALID_INPUT" {
		var bad = required.GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8
		var knownField = bad == "before" || bad == "after" || bad == "req_type" || bad == "req_name"
		if bad == "req_type" {
			bad = "required_type"
		}
		if bad == "req_name" {
			bad = "required_name"
		}
		if knownField && !required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf && !required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac && required.GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea == "INVALID_INPUT" {
			return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "INVALID_INPUT", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "INVALID_CANONICAL_INPUT", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: bad, GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
		}
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "DEPENDENCY_RESULT_INVALID", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if required.GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650 != "VALID_INPUT" || required.GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8 != "" {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "DEPENDENCY_RESULT_INVALID", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	var impactConsistent = (required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf && required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac && required.GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea == "UNCHANGED_ALLOW") || (!required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf && !required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac && required.GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea == "UNCHANGED_DENY") || (!required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf && required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac && required.GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea == "WIDENED") || (required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf && !required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac && required.GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea == "REVOKED")
	if !impactConsistent {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "DEPENDENCY_RESULT_INVALID", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if input2 == "" {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "INVALID_INPUT", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "EMPTY_REQUIRED_REQUEST", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "required_type", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if input3 == "" {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "INVALID_INPUT", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "EMPTY_REQUIRED_REQUEST", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "required_name", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: false, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: false, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: false, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if !required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "CURRENT_REQUIRED_REQUEST_DENIED", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	var grant = GoooPackage7ActivityAnalyzeServiceGrantChecked(input0, input1)
	if grant.GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704 != "VALID_INPUT" || grant.GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899 != "" {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "DEPENDENCY_RESULT_INVALID", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if grant.GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38 == "WIDENING_WITNESS" && !grant.GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f && grant.GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073 && grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "CHECKED_CANDIDATE_REQUEST" && grant.GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea != "" && grant.GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9 != "" && !grant.GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5 && grant.GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068 {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "WIDENING_WITNESS", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if grant.GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38 == "UNPROVEN" && !grant.GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f && !grant.GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073 && grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "" && grant.GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea == "" && grant.GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9 == "" && !grant.GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5 && !grant.GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068 {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "REFINEMENT_UNPROVEN", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	var knownRule = grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "CANDIDATE_EMPTY" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "IDENTICAL_PATTERNS" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "OLD_GLOBAL" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "OLD_TYPED_ALL" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "EXACT_CANDIDATE_ACCEPTED_BY_OLD" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "PREFIX_HIERARCHY" || grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c == "SUFFIX_HIERARCHY"
	var subsetConsistent = grant.GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38 == "SUPPORTED_SUBSET" && grant.GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f && !grant.GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073 && knownRule && grant.GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea == "" && grant.GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9 == "" && !grant.GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5 && !grant.GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068
	if !subsetConsistent {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "DEPENDENCY_RESULT_INVALID", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: ""}
	}
	if !required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac {
		return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "REFUSED", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "REQUIRED_REQUEST_REMOVED", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c}
	}
	return GoooRecord51ad4f38abe9c897f8535c5a89e9d4815654c44043b479d76857e7c0b112773a{GoooField397a9a71181c6837ce76c17bb755a874146f2fedb1d954183e1be570fa333f4c: "ELIGIBLE", GoooField818a53d020075bed26d058c6dfd90ca90ba62d9bceffbbe2c34831970bc39a7e: "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", GoooFieldc50c719148d72ed3692dcff66719cf9eb811ca9512603c20fa68766c17ecaa8f: "", GoooFieldcd852cceb25b591c0fdaeec7665c7e2827abf3c7071d00174b5d0dd760d6fb81: true, GoooField32aa5e7241a6c98298df0ddd377519d8cb5f57d74a41027013f030de8bb9ff04: required.GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf, GoooField03db507c412ad10359fb927108ef495bd34ac64ad3fa31396a57badec5342c32: required.GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac, GoooFieldf062617b96762071c7326c8ddb919c608bf1810227a2fca57ed804d580c51a38: grant.GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c}

}

//gooo:generated:end id="gooo://service-policy/conditional-apply/assess/v1" kind="activity"
//gooo:generated:start id="gooo://service-domain/type" kind="activity"
func GoooPackage3ActivityValidateServiceType(input string) bool {

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

//gooo:generated:end id="gooo://service-domain/type" kind="activity"
//gooo:generated:start id="gooo://service-domain/name" kind="activity"
func GoooPackage4ActivityValidateServiceName(input string) bool {

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

//gooo:generated:end id="gooo://service-domain/name" kind="activity"
//gooo:generated:start id="gooo://service-domain/pattern" kind="activity"
func GoooPackage5ActivityValidateServicePattern(input string) bool {

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
	if !GoooPackage3ActivityValidateServiceType(typ) {
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
	return GoooPackage4ActivityValidateServiceName(part)

}

//gooo:generated:end id="gooo://service-domain/pattern" kind="activity"
//gooo:generated:start id="gooo://agentmesh-prototype/match-service-pattern" kind="activity"
func GoooPackage0ActivityMatchServicePattern(input0 string, input1 string, input2 string) bool {

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

//gooo:generated:end id="gooo://agentmesh-prototype/match-service-pattern" kind="activity"
//gooo:generated:start id="gooo://service-impact/compare" kind="activity"
func GoooPackage1ActivityCompareServiceGrant(input0 string, input1 string, input2 string, input3 string) GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153 {

	var before = GoooPackage0ActivityMatchServicePattern(input0, input2, input3)
	var after = GoooPackage0ActivityMatchServicePattern(input1, input2, input3)
	var change = "UNCHANGED_DENY"
	if before {
		if after {
			change = "UNCHANGED_ALLOW"
		} else {
			change = "REVOKED"
		}
	} else {
		if after {
			change = "WIDENED"
		}
	}
	return GoooRecord8caadab4cd27247527aeb5d57230dc24cb20b32dd532144849f72efd0ed67153{GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d: before, GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219: after, GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d: change}

}

//gooo:generated:end id="gooo://service-impact/compare" kind="activity"
//gooo:generated:start id="gooo://service-policy/checked-impact/evaluate/v1" kind="activity"
func GoooPackage6ActivityCompareServiceGrantChecked(input0 string, input1 string, input2 string, input3 string) GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e {

	if !GoooPackage5ActivityValidateServicePattern(input0) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "before", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage5ActivityValidateServicePattern(input1) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "after", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage3ActivityValidateServiceType(input2) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "req_type", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	if !GoooPackage4ActivityValidateServiceName(input3) {
		return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "INVALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "req_name", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: false, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: false, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: "INVALID_INPUT"}
	}
	var result = GoooPackage1ActivityCompareServiceGrant(input0, input1, input2, input3)
	return GoooRecorda03acd3ba02b82f739d609e25239e3eb36532cb33623ba448b028923604c461e{GoooFieldb68bddcca03807b9d3b7351580885098ab19653ee50128d38ae96d5bac276650: "VALID_INPUT", GoooFieldc673629c5c89a0b38f8954ebea6f889f2292d582cce3fb1be7194fab70ecfad8: "", GoooFieldcb040b8a819e1010951cd9754fd976778e836fe898076d235b2fb3d3080e02cf: result.GoooFielddbb72b1dd94b1481e9a1dbe9ea75cb83fd7073af74987154593d2dad6adb649d, GoooField9c2e81f8161f0bc825eaa4d4f0f01ecd432178cb7e7db6eee17331a96f8176ac: result.GoooFielda820b495c790dce4367677d8cf1a3fd0e7cff0db75cf63a37389d5c91923c219, GoooField134fcdefde3b0f35a4f54e2c8857d7f11114b89fdac8465fea0b5a08df5295ea: result.GoooField9e32e23e3055b6c63e2d262d3924843e69f7c0c733913e3c8ee8d8f843929c4d}

}

//gooo:generated:end id="gooo://service-policy/checked-impact/evaluate/v1" kind="activity"
//gooo:generated:start id="gooo://service-refinement/analyze" kind="activity"
func GoooPackage2ActivityAnalyzeServiceGrant(input0 string, input1 string) GoooRecord702bf03e1b9b659de2cca8f66c90d35af72f6f992a24d89cf90ce7c269907367 {

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
			if GoooPackage0ActivityMatchServicePattern(input0, candidateType, candidateBase) {
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
			var before = GoooPackage0ActivityMatchServicePattern(input0, firstType, firstName)
			var after = GoooPackage0ActivityMatchServicePattern(input1, firstType, firstName)
			if after && !before {
				witnessed = true
				witnessType = firstType
				witnessName = firstName
				witnessBefore = before
				witnessAfter = after
			}
		}
		if !witnessed && secondName != "" {
			var before = GoooPackage0ActivityMatchServicePattern(input0, secondType, secondName)
			var after = GoooPackage0ActivityMatchServicePattern(input1, secondType, secondName)
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

//gooo:generated:end id="gooo://service-refinement/analyze" kind="activity"
//gooo:generated:start id="gooo://service-policy/checked-refinement/evaluate/v1" kind="activity"
func GoooPackage7ActivityAnalyzeServiceGrantChecked(input0 string, input1 string) GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64 {

	if !GoooPackage5ActivityValidateServicePattern(input0) {
		return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "INVALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "before", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: "INVALID_INPUT", GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: "", GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: false, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: false, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: "", GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: "", GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: false, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: false}
	}
	if !GoooPackage5ActivityValidateServicePattern(input1) {
		return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "INVALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "after", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: "INVALID_INPUT", GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: "", GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: false, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: false, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: "", GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: "", GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: false, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: false}
	}
	var result = GoooPackage2ActivityAnalyzeServiceGrant(input0, input1)
	return GoooRecord9d003d83477863c3d498fa44b96246cb2d9b8db54ef2d72d163a5bd796f6af64{GoooFieldb9bc7288b80e18695cd2ba3cd55ae0886e7f6db29565e5b9dfd8fa7ba1ad8704: "VALID_INPUT", GoooField2c45937c0e03d4504ca27156845c813e37b78a54e17cd46b8460249084d2f899: "", GoooFielde65f454fd5b6723b32117f7bd8595ef15deb2d3eebfdaf2990058ce0b7c7fb38: result.GoooFieldaf6991124a762ff89da9eae9eccd222db33bb2ddf56b92993d7bd2cf5c9ad354, GoooField4e7c9ae8fe957fe6a0f012ce48c75ca7dd8168843318af2eb56479b01164968c: result.GoooField24698bac43894a5573e5dadbf8c2e52cf44a1615484c51a4d0e675600a8d0454, GoooFieldcc2afc6f81cbe3844a0eae06e83134f93fe219a1547297d8d152bd31d8b2369f: result.GoooFielda09c15e873f28daf8cbd9c93c478c1480486831d7c55656c7d5491fbf2c52711, GoooField43c536994304b4d2b07bf7504a15a5fe8b4d87f3221a7b083d5d517a10087073: result.GoooFieldbba889b4d1fcabf4151fd8dad7f727eb777e045df9c39fff7dc72f4401ec003d, GoooField08b3874cbd90106dd8fd54eb7828cb158f21fde72f0c388c1f3567550ab6f5ea: result.GoooField02e0207ce9dd381ba57b20243ee685352c92be98dc5d7fe8d6e33525e5a3dfbd, GoooField58132c044e2ce842fe1b7255b9eb5238249a9ee43c0873e29554292a38d770e9: result.GoooFieldd602c0b83fa88a27ce104dff11a826ceeeb694a927fe062f415d97ce859b53d6, GoooField05ce321efb8e356a9db821a11acd7e12a8359eddf23b82dd2e55c4d05f0968b5: result.GoooFieldabdccd9e95eef4d357adccc51e4976e882b3d7a22d6b8701951a951f3eb4c4cc, GoooFielde9bbaf701f1e838fa2bc680ae2e9fd09ad51b00acf4953e05bf3d81388d10068: result.GoooField25664eeb9a2c3098e71e67b9fbfd995fdeecbd8e091db678ed6d4b66defab7ef}

}

//gooo:generated:end id="gooo://service-policy/checked-refinement/evaluate/v1" kind="activity"
