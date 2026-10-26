package refreshkernel

//gooo:generated:start id="gooo://tool-origin/catalog-refresh/v1/candidate-status" kind="entity"
type GoooRecordd196916f92c736be8812e1d76be69830ae3d223467a69b8bacbf56fc067115c7 struct {
	GoooField6dc8158b713f4ec3d3837b09dba4d9c1d7f1ca7f3642b2198c7439730df178a3 bool   `json:"valid"`
	GoooFielde4706b13762f4ee54ce61f86e4ef2ed3240fc31188377574cfe842116d3657c7 string `json:"reason"`
	GoooFieldfddc45acf98987625a7a3c2c940214a6de821d838b9b3373dba9a88801df4301 string `json:"coverage"`
	GoooField1f56632c2933eb427cb3125c76e6e8315306f5c8a48505fbf145612b540cee75 string `json:"coverage_reason"`
	GoooFieldf30488cac7bcc77e2d1f686a4d2de9fa4b1e67f400eee3ab7b8149d4824b10a9 int64  `json:"problem_page_index"`
	GoooField1000719346fcc298a058b86f4ceb68045eb700246d75db4f69db34f1fb24779c int64  `json:"problem_row_index"`
	GoooFielda7b34bfc627ca9489e7813f74b13a7e35d24f954795a1434a7b77e474d969dbe int64  `json:"page_count"`
	GoooField8d2bfa1c4241d03116984ab4c575f24e2e8472e419657b917ecc590b30619e46 int64  `json:"row_count"`
}

//gooo:generated:end id="gooo://tool-origin/catalog-refresh/v1/candidate-status" kind="entity"
//gooo:generated:start id="gooo://tool-origin/catalog-refresh/v1/decision" kind="entity"
type GoooRecord28074af32c494ad342d70e45501a0c17b0c62f719ad9da77fcde3f3088799c50 struct {
	GoooFieldb45035c5f3b4e0a9686f86459937ece2736fc071a51dc0065bbcdb2bcefe398c string `json:"state"`
	GoooFieldc92b7078f1ec6cc9b58599b99701de59f0fa388811f7b494c9c5522029b49593 string `json:"reason"`
	GoooField7b9d4dcd0b27b82c86fc48e93a1f0900d875d77c18e5b0d65aa4077c593e9e8c string `json:"candidate_id"`
	GoooField708627bb92d1ef761947bc20377aac92c35a6536ae582b2659e09589a24d5697 string `json:"attempt_id"`
	GoooField3aea6aba8cd184045f198f75b54d0d15314e1fb1a53231da23d74b0f43ba7b09 string `json:"expected_registry_version"`
	GoooField5074c42524f53e2dac53ad0c61b9543080126815e9bc542df03876270d0a7c63 string `json:"expected_scope_epoch"`
	GoooField802f48cd68ca3485ce95339087832c18805c59c64682d9d00b6fd40bafa17549 string `json:"expected_invalidation_epoch"`
	GoooField9110271f4a91166349f4c9cf3555530dd0f0200ac1a4b7f4f21f709c2cb2ef39 string `json:"expected_latest_attempt"`
	GoooField2f61fbc77037c29c92303006ae11d691513d5a7c49c01ad43dbb5eb6367cf085 int64  `json:"candidate_page_count"`
	GoooFieldfe1990f28e7244cc96f211b271fad67c56cb55be3cc337625ef8365f0a466fc1 int64  `json:"candidate_row_count"`
	GoooField2c100031e164b512ac9485da087d04099df96884fef6bf00b37f2d3c56d991f8 string `json:"coverage"`
	GoooField395b3b208cb79b60ecb6bd56ad0f8d3a068846577440bd634606bcff665f2d30 string `json:"coverage_reason"`
	GoooField3175d659156ca82699ecbec1c633f0f1c6022145a4fb92ea3c303a289564a25e int64  `json:"problem_page_index"`
	GoooField2341c752c64f8cfada74879c3f25f86fd6be66864f141d65611c3dc0bbea4b52 int64  `json:"problem_row_index"`
	GoooField8c6614baf1d7033c59d1f68f35960cb761af8beebc042a44ff8fb669778dab80 string `json:"decision_scope"`
	GoooField22313d6369eb2ae8355e2d98cd0bc85acfe29b2bf2a98ea8ceff1e3b2d5f5b38 bool   `json:"execution_authorized"`
	GoooFieldad322230141f558ea6be8e58b5e99538b2241e852d55ec63de63bf4780dab835 bool   `json:"live_effect_performed"`
	GoooFielddfdee191dd26bdd2a741c1fadceeaf805530e827c12dbedc67432257379ce25b string `json:"remote_atomicity"`
}

//gooo:generated:end id="gooo://tool-origin/catalog-refresh/v1/decision" kind="entity"
//gooo:generated:start id="gooo://tool-origin/catalog-refresh/v1/refresh-evidence" kind="entity"
type GoooRecordfe399ddb2fb6a2479413e08a7769d15865f1d8c6ba3c6bca970dd9cf95ea2179 struct {
	GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36 string   `json:"attempt_id"`
	GoooField95a4b108299fa581c6eda3e145abc6b5c99e345ee414eff8ce35fd0dc07eefb0 string   `json:"candidate_id"`
	GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 string   `json:"collector_state"`
	GoooField2faca12ae84814dbc7b0d4a860d383224ebb55e05108acbc78e3846cfe4b554a string   `json:"binding_state"`
	GoooField75427d3f80eda63dff9f2598dc9e6cb23292a5830db20e24b66c1e167458da7b string   `json:"validator_profile"`
	GoooFielde606eae7de091e2541083e4c687abb7356680399fb2d923ecf44e203542e6e64 []string `json:"row_definition_ids"`
	GoooField11174a0b4e54b048d0175f0224ce613ce33de6916bde6303e8e07263313554b4 []string `json:"row_validation_states"`
}

//gooo:generated:end id="gooo://tool-origin/catalog-refresh/v1/refresh-evidence" kind="entity"
//gooo:generated:start id="gooo://tool-origin/catalog-refresh/v1/refresh-state" kind="entity"
type GoooRecord75875bd2679c233875f4e28a2bfddb4b63f7972b06acd425b097eaf651a6c602 struct {
	GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41 string `json:"expected_registry_version"`
	GoooFieldb8bad7d576d3f3ef1bbfa52cb78155d8cd1a8bdb3a05ba832c9b3a8054b2239b string `json:"current_registry_version"`
	GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95 string `json:"expected_scope_epoch"`
	GoooField4d5f142fa7504fb39112a9b2e005b16b346ea42f4e99ec25956a17670b23fff2 string `json:"current_scope_epoch"`
	GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08 string `json:"expected_invalidation_epoch"`
	GoooFieldc4c614252daec3c11b2fc0f5cbfd15664b26d5d2a2084c9903de0f3ebc0c2646 string `json:"current_invalidation_epoch"`
	GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f string `json:"expected_latest_attempt"`
	GoooFieldfcafdac1622119746746a4d9c00163b335723fdfeda92d86b9482aca9dc1a84c string `json:"current_latest_attempt"`
}

//gooo:generated:end id="gooo://tool-origin/catalog-refresh/v1/refresh-state" kind="entity"
//gooo:generated:start id="gooo://tool-origin/definition-snapshot/v1/facts" kind="entity"
type GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25 struct {
	GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270 []string `json:"page_request_cursor_kinds"`
	GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393 []string `json:"page_request_cursor_values"`
	GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824 []string `json:"page_next_cursor_kinds"`
	GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55 []string `json:"page_next_cursor_values"`
	GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048 []string `json:"original_names"`
	GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42 []string `json:"definition_ids"`
	GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472 []string `json:"definition_profiles"`
	GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10 []string `json:"annotation_ids"`
	GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049 []int64  `json:"row_page_indexes"`
}

//gooo:generated:end id="gooo://tool-origin/definition-snapshot/v1/facts" kind="entity"
//gooo:generated:start id="gooo://tool-origin/definition-snapshot/v1/result" kind="entity"
type GoooRecord28d4e59537922d347cdeecb8ddf43183852c2c7bf6bf73fabff02468d4fcf78d struct {
	GoooFieldbcc80922b7849199853015833e146f8dc0ce58017e282da4c30d3a50a5c5ca0b string `json:"context_id"`
	GoooField18273142b6d46141e51852fde687f320069b853369752f530aef18f1a8fd7be2 string `json:"target_name"`
	GoooField8b81d4a2dc6c2b3b8666b512ccd9f48956d6f9adebd2bd25f45f677b6631b579 string `json:"state"`
	GoooField00ccf611ba4fe589dd50637cbc50e02f15aff0f1a80b0a49e34337425ca368ff string `json:"reason"`
	GoooFieldcb323392c9bbdc0405511ad798b37a4fa69c343da09ffeeaca078020299b9ee5 string `json:"next_action"`
	GoooField63a6c9a50699cb5b2f8b90c32a7c30bfc634007831dcba9396407c9c7041086e string `json:"comparison_scope"`
	GoooFieldacaa986a1441b7b3d9eefb633756a841ceda72ad445c439eef7c663467e9ff73 string `json:"before_page_coverage"`
	GoooField18971056017d1594a1f6bf8e3ddbd1ab92294ffd7bf1eb077d7d9088aafdfa95 string `json:"after_page_coverage"`
	GoooField4830c19362bc88157643f9b0c4daf8530e8d8455ad5e57775cea109a4b28344a string `json:"before_coverage_reason"`
	GoooField3e947e1e651c834ec6356353878d07a79c1eb3b63a08df4127f0da338c6575fb string `json:"after_coverage_reason"`
	GoooField89c0f766d0884fd8c9077074fff7c8146e9a3d8421bc15445897903e4abdbd14 int64  `json:"before_problem_page_index"`
	GoooField6a183f2475886028f735482e251d71a655b916980638bc84f12758e99165e1fe int64  `json:"after_problem_page_index"`
	GoooField7b82058191bf48fd3b1ef0db995b066a24615b53545f9ca74edd4ba8c3b1ac03 int64  `json:"before_problem_row_index"`
	GoooField0a4e010c728fb215a8b94ebbc7444769ed48e5483d343687bf9bee07a93b961e int64  `json:"after_problem_row_index"`
	GoooFielde0272827e9a9df708ee952803f9c48d8b866be39839893f7d3c5aae6b994c839 int64  `json:"before_index"`
	GoooFielda3200f5f87e40bdd5f94042a4fe0b44c9a6599ea48095eb8c5ad5d83bb122b82 int64  `json:"after_index"`
	GoooFieldb790b38a8e8f8b5cb1d09c033c8e0c237d22466bf56b8f03739ce08101716398 string `json:"before_definition_id"`
	GoooFielddf16b2b85e5bc6d5b97843dfe7e0bd29487b2c986540e84559356b2b3cd4fb7b string `json:"after_definition_id"`
	GoooField62d7b7c4076a16a72acb16e33b97e15ec9fcfebc04d9b685afcb6d8c53ec4e61 string `json:"before_definition_profile"`
	GoooField76dc4066ffc1c2702d1b089af2d1a17aa00994e9ac5ab9ae4cc959cda5e3ab49 string `json:"after_definition_profile"`
	GoooFielddbe0e973b2904a82c77bdb5cad1b540188e7d5231104934fce61a1e950be375e string `json:"before_annotation_id"`
	GoooField82b90b3b3e5e3560fcde62a9413cf744249081a63e1632f6c93391977ee2bab4 string `json:"after_annotation_id"`
	GoooFielda0b75a9c8930e5edb9cbe78219558b56f4c8854fd49b16f51a45b15fa428ce45 string `json:"observed_presence_relation"`
	GoooField40949c72acd2e5c5a5e34f542c9a61abdf1907455a14ccfcc9a0bf6ea0e9b3c5 string `json:"definition_relation"`
	GoooField2e0007a926bd286bb13a26f2265750e8d226fe28bd8906bd3b6246ec7022ac5c string `json:"annotation_relation"`
	GoooField944a3407c3251dc729bcca65fff1e05d8142c0335df1104e99c87dd186d7c854 string `json:"profile_relation"`
	GoooField9026e0ed0ae780de9f33eaab5a16c5c5c6c26c726a0d7c1268fe3241ca720db7 string `json:"origin_relation"`
	GoooField92db0f9a71f85b70f9f898dca9e98da7a691092873ba6c69da8bd3dcd120217e string `json:"authorization_relation"`
	GoooField3c159e4a8770047450814557ead5470d359916f4180021675076a085b1f13d65 bool   `json:"behavior_equivalence_proven"`
	GoooField259e273dad1ee97dce0bb044f85cc9f98f3008a044541c505b597e0890975e87 bool   `json:"execution_authorized"`
	GoooField17ab191a48ab90750695d456c8974a742bbd4a108738925e765e00e1438a6126 bool   `json:"live_effect_performed"`
	GoooFieldbb5ae9586c9064bef93d42d789b9c5159ee44bb382714f913cb1f57a639dad6d string `json:"remote_atomicity"`
}

//gooo:generated:end id="gooo://tool-origin/definition-snapshot/v1/result" kind="entity"
//gooo:generated:start id="gooo://tool-origin/definition-snapshot/v1/selection" kind="entity"
type GoooRecord4657ec7bd1268ab0c3c0e0c605b3ad32555eed4612703867937f41a7e32492ce struct {
	GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732 string `json:"context_id"`
	GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3 string `json:"target_name"`
}

//gooo:generated:end id="gooo://tool-origin/definition-snapshot/v1/selection" kind="entity"
//gooo:generated:start id="gooo://tool-origin/definition-snapshot/v1/snapshot-status" kind="entity"
type GoooRecord044863e9cd68763c389b45568e4fa7e779e353542fff064d279a9839930aa88a struct {
	GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 bool   `json:"valid"`
	GoooField16e8e5b7d1615d961ebf1f82dd7089e4fa2749b16b4697adeb7f1cdb46735158 string `json:"shape_reason"`
	GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b string `json:"coverage"`
	GoooField072f8daaf7adbdd4b9af984e5d14416083ab85446ad85b999d20b7248a890783 string `json:"coverage_reason"`
	GoooField3fa7c2b2229d5959dc86ec343f4bc355c1e7e8e705dd7381348fa3e190e2b051 int64  `json:"problem_page_index"`
	GoooFieldc4c39d3b9f23380f78b98cd111e97a89bc48aa414ef32e5f1f2d38d37acdf4cb int64  `json:"problem_row_index"`
	GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 int64  `json:"duplicate_first_index"`
	GoooFieldccda8c74a1156edd058e35a776c0707edc23035296419317ec4307549d72e96f int64  `json:"duplicate_second_index"`
	GoooField0ced5d5b1e45be76dde72062f976b5d3761ddbee9d002f6d4b46882df429b63d int64  `json:"selected_index"`
}

//gooo:generated:end id="gooo://tool-origin/definition-snapshot/v1/snapshot-status" kind="entity"

//gooo:generated:start id="gooo://tool-origin/catalog-refresh/v1/decide" kind="activity"
func GoooComposedActivity0(input0 GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25, input1 GoooRecordfe399ddb2fb6a2479413e08a7769d15865f1d8c6ba3c6bca970dd9cf95ea2179, input2 GoooRecord75875bd2679c233875f4e28a2bfddb4b63f7972b06acd425b097eaf651a6c602) GoooRecord28074af32c494ad342d70e45501a0c17b0c62f719ad9da77fcde3f3088799c50 {

	var candidate = GoooComposedActivity0Call3(input0, input1)
	var reason = "COMPLETE_CANDIDATE"
	if !GoooComposedActivity0Call4(input1) {
		reason = "INVALID_REFRESH_EVIDENCE"
	} else if input1.GoooField2faca12ae84814dbc7b0d4a860d383224ebb55e05108acbc78e3846cfe4b554a != "BOUND" {
		reason = "UNBOUND_CANDIDATE"
	} else if !GoooComposedActivity0Call5(input2) {
		reason = "INVALID_REFRESH_STATE"
	} else if input2.GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f != input1.GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36 {
		reason = "ATTEMPT_BINDING_MISMATCH"
	} else if input1.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 == "ERROR" {
		reason = "COLLECTOR_ERROR"
	} else if input1.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 == "TIMEOUT" {
		reason = "COLLECTOR_TIMEOUT"
	} else if input1.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 == "CANCELLED" {
		reason = "COLLECTOR_CANCELLED"
	} else if !candidate.GoooField6dc8158b713f4ec3d3837b09dba4d9c1d7f1ca7f3642b2198c7439730df178a3 {
		reason = candidate.GoooFielde4706b13762f4ee54ce61f86e4ef2ed3240fc31188377574cfe842116d3657c7
	} else if input2.GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41 != input2.GoooFieldb8bad7d576d3f3ef1bbfa52cb78155d8cd1a8bdb3a05ba832c9b3a8054b2239b {
		reason = "REGISTRY_CHANGED"
	} else if input2.GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95 != input2.GoooField4d5f142fa7504fb39112a9b2e005b16b346ea42f4e99ec25956a17670b23fff2 {
		reason = "SCOPE_CHANGED"
	} else if input2.GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08 != input2.GoooFieldc4c614252daec3c11b2fc0f5cbfd15664b26d5d2a2084c9903de0f3ebc0c2646 {
		reason = "INVALIDATED"
	} else if input2.GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f != input2.GoooFieldfcafdac1622119746746a4d9c00163b335723fdfeda92d86b9482aca9dc1a84c {
		reason = "SUPERSEDED_ATTEMPT"
	}
	var state = "RETAIN_STORED_CATALOG"
	if reason == "COMPLETE_CANDIDATE" {
		state = "CANDIDATE_ELIGIBLE"
	}
	return GoooRecord28074af32c494ad342d70e45501a0c17b0c62f719ad9da77fcde3f3088799c50{
		GoooFieldb45035c5f3b4e0a9686f86459937ece2736fc071a51dc0065bbcdb2bcefe398c: state, GoooFieldc92b7078f1ec6cc9b58599b99701de59f0fa388811f7b494c9c5522029b49593: reason, GoooField7b9d4dcd0b27b82c86fc48e93a1f0900d875d77c18e5b0d65aa4077c593e9e8c: input1.GoooField95a4b108299fa581c6eda3e145abc6b5c99e345ee414eff8ce35fd0dc07eefb0, GoooField708627bb92d1ef761947bc20377aac92c35a6536ae582b2659e09589a24d5697: input1.GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36,
		GoooField3aea6aba8cd184045f198f75b54d0d15314e1fb1a53231da23d74b0f43ba7b09: input2.GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41,
		GoooField5074c42524f53e2dac53ad0c61b9543080126815e9bc542df03876270d0a7c63: input2.GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95,
		GoooField802f48cd68ca3485ce95339087832c18805c59c64682d9d00b6fd40bafa17549: input2.GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08,
		GoooField9110271f4a91166349f4c9cf3555530dd0f0200ac1a4b7f4f21f709c2cb2ef39: input2.GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f,
		GoooField2f61fbc77037c29c92303006ae11d691513d5a7c49c01ad43dbb5eb6367cf085: candidate.GoooFielda7b34bfc627ca9489e7813f74b13a7e35d24f954795a1434a7b77e474d969dbe, GoooFieldfe1990f28e7244cc96f211b271fad67c56cb55be3cc337625ef8365f0a466fc1: candidate.GoooField8d2bfa1c4241d03116984ab4c575f24e2e8472e419657b917ecc590b30619e46,
		GoooField2c100031e164b512ac9485da087d04099df96884fef6bf00b37f2d3c56d991f8: candidate.GoooFieldfddc45acf98987625a7a3c2c940214a6de821d838b9b3373dba9a88801df4301, GoooField395b3b208cb79b60ecb6bd56ad0f8d3a068846577440bd634606bcff665f2d30: candidate.GoooField1f56632c2933eb427cb3125c76e6e8315306f5c8a48505fbf145612b540cee75,
		GoooField3175d659156ca82699ecbec1c633f0f1c6022145a4fb92ea3c303a289564a25e: candidate.GoooFieldf30488cac7bcc77e2d1f686a4d2de9fa4b1e67f400eee3ab7b8149d4824b10a9, GoooField2341c752c64f8cfada74879c3f25f86fd6be66864f141d65611c3dc0bbea4b52: candidate.GoooField1000719346fcc298a058b86f4ceb68045eb700246d75db4f69db34f1fb24779c,
		GoooField8c6614baf1d7033c59d1f68f35960cb761af8beebc042a44ff8fb669778dab80: "WHOLE_CANDIDATE_LOCAL_REPLACEMENT_PRECHECK",
		GoooField22313d6369eb2ae8355e2d98cd0bc85acfe29b2bf2a98ea8ceff1e3b2d5f5b38: false, GoooFieldad322230141f558ea6be8e58b5e99538b2241e852d55ec63de63bf4780dab835: false, GoooFielddfdee191dd26bdd2a741c1fadceeaf805530e827c12dbedc67432257379ce25b: "UNKNOWN",
	}

}
func GoooComposedActivity0Call0(input string) bool {

	if len(input) > 1024 {
		return false
	}
	var i int64 = 0
	for i = 0; i < 1024; i = i + 1 {
		if i < int64(len(input)) {
			if int64(input[i]) == 0 {
				return false
			}
		}
	}
	return true

}
func GoooComposedActivity0Call1(input string) bool {

	if len(input) != 71 {
		return false
	}
	if input[0:7] != "sha256:" {
		return false
	}
	var i int64 = 0
	for i = 0; i < 71; i = i + 1 {
		if i >= 7 {
			var octet = int64(input[i])
			if !((octet >= 48 && octet <= 57) || (octet >= 97 && octet <= 102)) {
				return false
			}
		}
	}
	return true

}
func GoooComposedActivity0Call2(input0 GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25, input1 GoooRecord4657ec7bd1268ab0c3c0e0c605b3ad32555eed4612703867937f41a7e32492ce) GoooRecord044863e9cd68763c389b45568e4fa7e779e353542fff064d279a9839930aa88a {

	var pages int64 = int64(len(input0.GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270))
	var rows int64 = int64(len(input0.GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048))
	var valid = true
	var shapeReason = ""
	var coverage = "COMPLETE"
	var coverageReason = "OBSERVED_PAGE_CHAIN_COMPLETE"
	var problemPage int64 = -1
	var problemRow int64 = -1
	var duplicateFirst int64 = -1
	var duplicateSecond int64 = -1
	var selected int64 = -1
	if pages > 16 || rows > 256 {
		valid = false
		shapeReason = "SNAPSHOT_LIMIT"
	}
	if int64(len(input0.GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393)) != pages || int64(len(input0.GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824)) != pages || int64(len(input0.GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55)) != pages {
		if valid {
			shapeReason = "PAGE_COLUMN_LENGTH"
		}
		valid = false
	}
	if int64(len(input0.GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42)) != rows || int64(len(input0.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472)) != rows || int64(len(input0.GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10)) != rows || int64(len(input0.GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049)) != rows {
		if valid {
			shapeReason = "ROW_COLUMN_LENGTH"
		}
		valid = false
	}
	if valid {
		var p int64 = 0
		for _, requestKind := range input0.GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270 {
			var nextKind = input0.GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824[p]
			var requestValue = input0.GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393[p]
			var nextValue = input0.GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55[p]
			var bad = (requestKind != "ABSENT" && requestKind != "STRING") || (nextKind != "ABSENT" && nextKind != "NULL" && nextKind != "STRING")
			if requestKind != "STRING" && requestValue != "" {
				bad = true
			}
			if nextKind != "STRING" && nextValue != "" {
				bad = true
			}
			if !GoooComposedActivity0Call0(requestValue) || !GoooComposedActivity0Call0(nextValue) {
				bad = true
			}
			if bad && valid {
				valid = false
				shapeReason = "INVALID_CURSOR_TAG_OR_VALUE"
				problemPage = p
			}
			p = p + 1
		}
	}
	if valid {
		var r int64 = 0
		var previousPage int64 = -1
		for _, name := range input0.GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048 {
			var bad = name == "" || !GoooComposedActivity0Call0(name)
			if !GoooComposedActivity0Call1(input0.GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42[r]) || !GoooComposedActivity0Call1(input0.GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10[r]) {
				bad = true
			}
			if input0.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472[r] == "" || !GoooComposedActivity0Call0(input0.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472[r]) {
				bad = true
			}
			var page = input0.GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049[r]
			if page < 0 || page >= int64(pages) || page < previousPage {
				bad = true
			}
			if bad && valid {
				valid = false
				shapeReason = "INVALID_ROW_FACT"
				problemRow = r
			}
			previousPage = page
			r = r + 1
		}
	}
	if valid {
		var r int64 = 0
		for _, name := range input0.GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048 {
			if name == input1.GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3 && selected == -1 {
				selected = r
			}
			var earlier int64 = 0
			for _, prior := range input0.GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048 {
				if earlier < r && prior == name && duplicateFirst == -1 {
					duplicateFirst = earlier
					duplicateSecond = r
					problemRow = r
				}
				earlier = earlier + 1
			}
			r = r + 1
		}
		if pages == 0 {
			coverage = "INCOMPLETE"
			coverageReason = "NO_SAVED_PAGES"
		} else {
			var p int64 = 0
			for _, requestKind := range input0.GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270 {
				var badLink = false
				var linkReason = ""
				if p == 0 {
					if requestKind != "ABSENT" {
						badLink = true
						linkReason = "MISSING_INITIAL_PAGE"
					}
				} else {
					if input0.GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824[p-1] != "STRING" {
						badLink = true
						linkReason = "PAGE_AFTER_TERMINAL"
					} else if requestKind != "STRING" || input0.GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393[p] != input0.GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55[p-1] {
						badLink = true
						linkReason = "CURSOR_LINK_MISMATCH"
					}
				}
				if badLink && coverage == "COMPLETE" {
					coverage = "INCOMPLETE"
					coverageReason = linkReason
					problemPage = p
				}
				p = p + 1
			}
			if input0.GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824[pages-1] == "STRING" && coverage == "COMPLETE" {
				coverage = "INCOMPLETE"
				coverageReason = "MISSING_TERMINAL_PAGE"
				problemPage = int64(pages - 1)
			}
		}
	} else {
		coverage = "INVALID"
		coverageReason = shapeReason
	}
	return GoooRecord044863e9cd68763c389b45568e4fa7e779e353542fff064d279a9839930aa88a{
		GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88: valid, GoooField16e8e5b7d1615d961ebf1f82dd7089e4fa2749b16b4697adeb7f1cdb46735158: shapeReason, GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b: coverage, GoooField072f8daaf7adbdd4b9af984e5d14416083ab85446ad85b999d20b7248a890783: coverageReason,
		GoooField3fa7c2b2229d5959dc86ec343f4bc355c1e7e8e705dd7381348fa3e190e2b051: problemPage, GoooFieldc4c39d3b9f23380f78b98cd111e97a89bc48aa414ef32e5f1f2d38d37acdf4cb: problemRow,
		GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2: duplicateFirst, GoooFieldccda8c74a1156edd058e35a776c0707edc23035296419317ec4307549d72e96f: duplicateSecond, GoooField0ced5d5b1e45be76dde72062f976b5d3761ddbee9d002f6d4b46882df429b63d: selected,
	}

}
func GoooComposedActivity0Call3(input0 GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25, input1 GoooRecordfe399ddb2fb6a2479413e08a7769d15865f1d8c6ba3c6bca970dd9cf95ea2179) GoooRecordd196916f92c736be8812e1d76be69830ae3d223467a69b8bacbf56fc067115c7 {

	var selection = GoooRecord4657ec7bd1268ab0c3c0e0c605b3ad32555eed4612703867937f41a7e32492ce{GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732: "", GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3: ""}
	var inspected = GoooComposedActivity0Call2(input0, selection)
	var pages int64 = int64(len(input0.GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270))
	var rows int64 = int64(len(input0.GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048))
	var reason = "COMPLETE_CANDIDATE"
	var problemRow = inspected.GoooFieldc4c39d3b9f23380f78b98cd111e97a89bc48aa414ef32e5f1f2d38d37acdf4cb
	if input1.GoooField75427d3f80eda63dff9f2598dc9e6cb23292a5830db20e24b66c1e167458da7b != "gooo/mcp-tool-shape/v1" {
		reason = "UNSUPPORTED_VALIDATOR_PROFILE"
	} else if !inspected.GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 {
		reason = "INVALID_SNAPSHOT"
	} else if inspected.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b != "COMPLETE" {
		reason = "INCOMPLETE_CATALOG"
	} else if inspected.GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 >= 0 {
		reason = "DUPLICATE_NAME"
	}
	if reason == "COMPLETE_CANDIDATE" {
		var r int64 = 0
		for _, profile := range input0.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472 {
			if profile != "gooo/mcp-tool-definition-json/v1" && reason == "COMPLETE_CANDIDATE" {
				reason = "UNSUPPORTED_DEFINITION_PROFILE"
				if problemRow == -1 {
					problemRow = r
				}
			}
			r = r + 1
		}
	}
	if reason == "COMPLETE_CANDIDATE" {
		if int64(len(input1.GoooFielde606eae7de091e2541083e4c687abb7356680399fb2d923ecf44e203542e6e64)) != rows || int64(len(input1.GoooField11174a0b4e54b048d0175f0224ce613ce33de6916bde6303e8e07263313554b4)) != rows {
			reason = "ROW_VALIDATION_LENGTH"
		}
	}

	if reason == "COMPLETE_CANDIDATE" {
		var r int64 = 0
		for _, definition := range input0.GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42 {
			if input1.GoooFielde606eae7de091e2541083e4c687abb7356680399fb2d923ecf44e203542e6e64[r] != definition && reason == "COMPLETE_CANDIDATE" {
				reason = "ROW_VALIDATION_BINDING_MISMATCH"
				if problemRow == -1 {
					problemRow = r
				}
			}
			r = r + 1
		}
	}
	if reason == "COMPLETE_CANDIDATE" {
		var r int64 = 0
		for _, outcome := range input1.GoooField11174a0b4e54b048d0175f0224ce613ce33de6916bde6303e8e07263313554b4 {
			if outcome != "ACCEPTED" && reason == "COMPLETE_CANDIDATE" {
				if outcome == "REJECTED" {
					reason = "ROW_REJECTED"
				} else if outcome == "UNKNOWN" {
					reason = "ROW_UNASSESSED"
				} else {
					reason = "INVALID_ROW_VALIDATION_STATE"
				}
				if problemRow == -1 {
					problemRow = r
				}
			}
			r = r + 1
		}
	}
	return GoooRecordd196916f92c736be8812e1d76be69830ae3d223467a69b8bacbf56fc067115c7{
		GoooField6dc8158b713f4ec3d3837b09dba4d9c1d7f1ca7f3642b2198c7439730df178a3: reason == "COMPLETE_CANDIDATE", GoooFielde4706b13762f4ee54ce61f86e4ef2ed3240fc31188377574cfe842116d3657c7: reason,
		GoooFieldfddc45acf98987625a7a3c2c940214a6de821d838b9b3373dba9a88801df4301: inspected.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b, GoooField1f56632c2933eb427cb3125c76e6e8315306f5c8a48505fbf145612b540cee75: inspected.GoooField072f8daaf7adbdd4b9af984e5d14416083ab85446ad85b999d20b7248a890783,
		GoooFieldf30488cac7bcc77e2d1f686a4d2de9fa4b1e67f400eee3ab7b8149d4824b10a9: inspected.GoooField3fa7c2b2229d5959dc86ec343f4bc355c1e7e8e705dd7381348fa3e190e2b051, GoooField1000719346fcc298a058b86f4ceb68045eb700246d75db4f69db34f1fb24779c: problemRow,
		GoooFielda7b34bfc627ca9489e7813f74b13a7e35d24f954795a1434a7b77e474d969dbe: int64(pages), GoooField8d2bfa1c4241d03116984ab4c575f24e2e8472e419657b917ecc590b30619e46: int64(rows),
	}

}
func GoooComposedActivity0Call4(input GoooRecordfe399ddb2fb6a2479413e08a7769d15865f1d8c6ba3c6bca970dd9cf95ea2179) bool {

	if input.GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36 == "" || !GoooComposedActivity0Call0(input.GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36) {
		return false
	}
	if !GoooComposedActivity0Call1(input.GoooField95a4b108299fa581c6eda3e145abc6b5c99e345ee414eff8ce35fd0dc07eefb0) {
		return false
	}
	if input.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 != "SUCCESS" && input.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 != "ERROR" && input.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 != "TIMEOUT" && input.GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86 != "CANCELLED" {
		return false
	}
	if input.GoooField2faca12ae84814dbc7b0d4a860d383224ebb55e05108acbc78e3846cfe4b554a != "BOUND" && input.GoooField2faca12ae84814dbc7b0d4a860d383224ebb55e05108acbc78e3846cfe4b554a != "UNBOUND" {
		return false
	}
	if input.GoooField75427d3f80eda63dff9f2598dc9e6cb23292a5830db20e24b66c1e167458da7b == "" || !GoooComposedActivity0Call0(input.GoooField75427d3f80eda63dff9f2598dc9e6cb23292a5830db20e24b66c1e167458da7b) {
		return false
	}
	return true

}
func GoooComposedActivity0Call5(input GoooRecord75875bd2679c233875f4e28a2bfddb4b63f7972b06acd425b097eaf651a6c602) bool {

	if input.GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41 == "" || !GoooComposedActivity0Call0(input.GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41) {
		return false
	}
	if input.GoooFieldb8bad7d576d3f3ef1bbfa52cb78155d8cd1a8bdb3a05ba832c9b3a8054b2239b == "" || !GoooComposedActivity0Call0(input.GoooFieldb8bad7d576d3f3ef1bbfa52cb78155d8cd1a8bdb3a05ba832c9b3a8054b2239b) {
		return false
	}
	if input.GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95 == "" || !GoooComposedActivity0Call0(input.GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95) {
		return false
	}
	if input.GoooField4d5f142fa7504fb39112a9b2e005b16b346ea42f4e99ec25956a17670b23fff2 == "" || !GoooComposedActivity0Call0(input.GoooField4d5f142fa7504fb39112a9b2e005b16b346ea42f4e99ec25956a17670b23fff2) {
		return false
	}
	if input.GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08 == "" || !GoooComposedActivity0Call0(input.GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08) {
		return false
	}
	if input.GoooFieldc4c614252daec3c11b2fc0f5cbfd15664b26d5d2a2084c9903de0f3ebc0c2646 == "" || !GoooComposedActivity0Call0(input.GoooFieldc4c614252daec3c11b2fc0f5cbfd15664b26d5d2a2084c9903de0f3ebc0c2646) {
		return false
	}
	if input.GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f == "" || !GoooComposedActivity0Call0(input.GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f) {
		return false
	}
	if input.GoooFieldfcafdac1622119746746a4d9c00163b335723fdfeda92d86b9482aca9dc1a84c == "" || !GoooComposedActivity0Call0(input.GoooFieldfcafdac1622119746746a4d9c00163b335723fdfeda92d86b9482aca9dc1a84c) {
		return false
	}
	return true

}

//gooo:generated:end id="gooo://tool-origin/catalog-refresh/v1/decide" kind="activity"
