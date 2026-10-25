package definitionkernel

//gooo:generated:start id="gooo://tool-origin/definition-binding/v1/input" kind="entity"
type GoooRecordb012d7ff14e3f1aafd160f4618549815bacf3a72de707e856fc582c05ce63633 struct {
	GoooFielddffb0d9c99a177e97081856bde6ff8aba519d83851f584da97a39b6d10bc6b64 string   `json:"context_id"`
	GoooField4a040797d0dc1c8c73be21832f5044acb789916fc30634be414dc77e866f89a3 string   `json:"alias_context_id"`
	GoooFieldbbf4118486629c5858a6c16d6cdfe354c5ba0da4725dea18a51b0694ce3a8ca7 string   `json:"collection_context_id"`
	GoooFielde508df087fed7e2114acffa4644a6f5a8354a5b8795fcda027bdf5bef16e6e25 []string `json:"before_definition_ids"`
	GoooField679db5638e7516f0651309847b7eae8e4f2105ecf49820624096f080040f1a9e []string `json:"after_definition_ids"`
	GoooField6546d37842d28b2b883459e51402341c9d3983f37e17fed69d04f59789b3f17a []string `json:"before_definition_profiles"`
	GoooField4f177805a0111b1f94862b96e45102f482e4c609f27d319b0d3c022d3eda7722 []string `json:"after_definition_profiles"`
	GoooFieldc04d90981596caf8ecd7ac185159c963eafecb93fbdb614f331eb482c5856ce7 []string `json:"before_capability_ids"`
	GoooFielde09fac6641fe463f2034fc949e3f1b834ae96bb87064421eda5623fbd669a8c8 []string `json:"after_capability_ids"`
	GoooFielde8c04553264ef06f247683cc0b5b0806f0f3d79b896b21b9f805f3299023a4cf string   `json:"before_policy_revision"`
	GoooFieldeda438bbcda65225bb331d9676abdb21cacf7ad0bc7d777a315ac4bb77d6cec2 string   `json:"after_policy_revision"`
}

//gooo:generated:end id="gooo://tool-origin/definition-binding/v1/input" kind="entity"
//gooo:generated:start id="gooo://tool-origin/definition-binding/v1/result" kind="entity"
type GoooRecorde850682663826a57b801017e71179c46fcf778407bbdaa4d91e181f35fd9566c struct {
	GoooField3e8344c828080d05509ccae691e2679888337debb63c79d5d952a78df5be435c string `json:"context_id"`
	GoooFielddadb8bec311e8cc220df5f05a613f0cf1a385c94175d32be1a2770969270ec04 string `json:"origin_context_id"`
	GoooFieldd1f19447d3fa701c1671ea38f76e8a75ab114d77ca7a4752f9f0c75d12af00b4 string `json:"collection_context_id"`
	GoooField40c37d648d894360192076b7cbbadae071f4c8a096bf0868d050ca1dad5a680b string `json:"origin_state"`
	GoooFieldd88ae7cf4ca18f299c810cb81a16cfa908a2d411682d0718f77073407eb9c276 string `json:"origin_reason"`
	GoooFieldfa3b334a646702521460a06556881e5740715f9886ada34560c7729853843598 string `json:"collection_state"`
	GoooField75c6b28aed721730a5bb98c77bd1be78c34c20b5fc331b22dc764b87198eaedb string `json:"collection_reason"`
	GoooFieldf016688d9f62cb4bba3f8938fc1d68be9157949da0ae5660ed24bc88f3595037 int64  `json:"before_index"`
	GoooField4794366c09b6e0dd22239a0870c5ab574201c16b2394c8caf003c695a434529d int64  `json:"after_index"`
	GoooFielde08c9b681c42cc184d057a504ddfe26f8c7d549f936d7b46ad453bd8be1530e1 string `json:"collection_first_problem_code"`
	GoooFieldc240e43a4309c1b6244c0086e03d179813b35239ed9f60172bb9c10cfc29d664 string `json:"definition_relation"`
	GoooFielde8e0375a071d7f3330f89b6f1f1530af674225a19d4b3f25bbb7ab160aa8f4b1 string `json:"definition_profile_relation"`
	GoooField50e198e9819d0520239a2a4fe68f965ac6e2be7b631e2fddfab2589d1dcd096f string `json:"capability_relation"`
	GoooField95fc988a55e0e2d357aa2713a36065b21c961dae27a20016a2cbc0df44cb2157 string `json:"authorization_relation"`
	GoooField6088657cc00835dab667b54aacfcbb582aa70a0bf674b337aa4237159b58b6b5 string `json:"policy_relation"`
	GoooFieldb0feaf12f92f2d7fbd8b27dcbac1dff3a1b171584a802fbbab7bc74394c975d6 string `json:"before_definition_id"`
	GoooFielda0b4c083415c69709761fb58e2ab9f519e48194bb983d30bb2d5071ca5fb90a5 string `json:"after_definition_id"`
	GoooField6de54242e2089835bdbd6a93f255cdd11345970c96a94c0cb7c5116cf7955103 string `json:"before_definition_profile"`
	GoooFielda7051e64d27f3e52aa6064efb498d6bd80351be99ead88b1553da1254923e3be string `json:"after_definition_profile"`
	GoooField6de71529dd215103adc3c534804eb46eb1572a18a8ad591dbef068e9d93f6ad8 string `json:"before_capability_id"`
	GoooField3dedc07522eb3bcf5056f95580514abcc6ee1b0064a1db0d22734a410982b347 string `json:"after_capability_id"`
	GoooField84088f66e4d707bcee599bd18d563386ed8ae8c78b567462a77da35e5a13354c string `json:"before_policy_revision"`
	GoooField22f66e6d5f5b6f312376f95fbd54af8be38c05b7035524e88a92a51339172548 string `json:"after_policy_revision"`
	GoooField9d633c471165a6437dcb25bc8ce160c7159cee2593bc736ef145f9f99c040680 string `json:"state"`
	GoooFieldf80ada518a9793b77927e9e89d623e83af40a28284f8acb50a335bf76e7bc2db string `json:"reason"`
	GoooFielda822e777dfdbbd9b93d26d42b467286821e70057fdbe7de67196c570bfef9e9f string `json:"next_action"`
	GoooFieldd7086a5c8879fc94864722bb800b920c4ce1d70212a77f0adbd1ea050014df61 bool   `json:"declared_bindings_match"`
	GoooFieldc2020d8241f0f19c3ffb0528074adc46c4a95486b88779d1634fa074aa511c46 bool   `json:"behavior_equivalence_proven"`
	GoooField6ae444a93a7cb2b70e05c669cef17675c658676e471108f9d43b0e3535cf7d71 bool   `json:"execution_authorized"`
	GoooField4868b45e2b24e4f899e9c4763ecedaf2beea8c1e936651953a96d55a4a78dcec bool   `json:"live_effect_performed"`
	GoooFieldefd6184e484efa8bdb4cc9789769360936f5e63331a49faf49d160a3aa00d079 bool   `json:"contains_synthetic_fixture"`
	GoooFielde1904d75aca8c711898aaf87bdc606812c35ded98056e53334f64c24648c97a9 string `json:"remote_atomicity"`
}

//gooo:generated:end id="gooo://tool-origin/definition-binding/v1/result" kind="entity"
//gooo:generated:start id="gooo://tool-origin/recorded-catalog/v1/input" kind="entity"
type GoooRecorde9a2c608bd57c725bc756fe66f68f2407910973fe5391c93fdd4d7ad43edd1a3 struct {
	GoooField1f17ccb002e4ab134fff7f78e56e479a421e2baeb938a2e8ff02acef74a0864f string   `json:"context_id"`
	GoooFieldc05ad3dd3a6d25ad68b7a59dab7a6512961fca0d8e37d6a13835a7967fcf3281 string   `json:"alias_context_id"`
	GoooFieldd7d6194681376b9df506f52d2a3391beb46d3ee3708b04201ffdcfe4b41e433c string   `json:"before_catalog_authorization_context_id"`
	GoooField505da28ceeb71a548fe610ffe699be0f3c61068c325bccaebaaeac4fe7fd8a3f string   `json:"after_catalog_authorization_context_id"`
	GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9 []string `json:"declared_sides"`
	GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd []string `json:"declared_host_ids"`
	GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29 []string `json:"declared_connection_ids"`
	GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f []string `json:"capture_sides"`
	GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395 []string `json:"capture_host_ids"`
	GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b []string `json:"capture_connection_ids"`
	GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0 []string `json:"capture_recording_bases"`
	GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb []string `json:"capture_evidence_kinds"`
	GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3 []string `json:"capture_watch_coverages"`
	GoooFielde97c146c5f3b6d5cfd44ba482dba123b232a0de4297f1370f5278f3a08f98380 []string `json:"capture_remote_atomicities"`
	GoooFieldfcc990dd181cc3038110126b349fdcd444abe34593b11c6aba5cbc16336f700d []string `json:"capture_begin_bindings"`
	GoooField45733addc9cb65fbddbfc376d6abc9a47a76a7886536db99cbb1c1fc77cfa72d []string `json:"capture_end_bindings"`
	GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f []string `json:"capture_begin_auth_contexts"`
	GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726 []string `json:"capture_end_auth_contexts"`
	GoooField2d15a2ff2e7598d651442f42a26ce28b57ad4e024633727c14a5d8ca496df31f []string `json:"capture_begin_config_revisions"`
	GoooFieldda2dd52494a43c9e409ea904715ae0a7ad29f03d34e0aa38d7dd96ec51471f78 []string `json:"capture_end_config_revisions"`
	GoooFieldbde4255daed43f9499fee96e6e8e360c2d7004679bae37fc68ed20714e43414d []string `json:"capture_begin_protocols"`
	GoooField3b2bebd67139f53b88d71c330ab27cc0fefef2de74c210c7a6a9e7f6d5c02caf []string `json:"capture_end_protocols"`
	GoooFieldfa7abe9325a28ea644e860fcfc818d668cd559be36fd94587a82e60a969894d0 []bool   `json:"capture_relevant_change_observed"`
	GoooField8b9de0aee82e1beeeda3e157dc4b7bbf377f1c523048f5b2ece15572e3b50872 []int64  `json:"page_capture_indexes"`
	GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5 []string `json:"page_request_cursor_kinds"`
	GoooField6d59473999bc855a5f11e2102f04dc92272d390213dc7c4e25ef55470bf99489 []string `json:"page_request_cursor_values"`
	GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f []string `json:"page_next_cursor_kinds"`
	GoooFieldafda383193e738c3c19bc9a49361d319ef33dad8017c1ec3e130253ff6306be2 []string `json:"page_next_cursor_values"`
	GoooField79c241d258a35a0b8cc84130750730aa9a55cb5460b84856814ce307dca4e0a0 []string `json:"page_bindings"`
	GoooFielddcee9b84c5b08be7d7152be4ede578ec076160f0ab2e6e76ffdc659047e00de1 []string `json:"page_auth_contexts"`
	GoooField5fc11073284805be697146fe7cb776a1321e6194535674f6c58c76be4c185c4e []string `json:"page_config_revisions"`
	GoooFieldfe4b54b52ee4ed272b1cc435df15713dd13d2d544ba28638a72cf8f44a5238da []string `json:"page_protocols"`
}

//gooo:generated:end id="gooo://tool-origin/recorded-catalog/v1/input" kind="entity"
//gooo:generated:start id="gooo://tool-origin/recorded-catalog/v1/result" kind="entity"
type GoooRecordaa12040aa48df656a2672ae2e8b3cc715b06749610492e678c3c0517d3230a64 struct {
	GoooField6eb89ea461a7030e08b98a61049763d85e84d78215ef8cf5e156e8f8d3ddfa66 string   `json:"origin_context_id"`
	GoooField1db058eff73cda088b309c5d1912f1ba73f7016260f4834d8af0684e4588b952 string   `json:"origin_state"`
	GoooField26edd9863bab28c16b1fd1848bb6a4ca71b8750dd00fcbbbc636e74ff9a87a88 string   `json:"origin_reason"`
	GoooField03155b7da8f7eaf403d6c6b7946d60c458396df1952e7d8f56208c615b431f51 int64    `json:"origin_before_index"`
	GoooFieldb8c0c4da7c6f7d5ab81a1cd411b40bd018a40d7a8db4858a30aa3106259c9c38 int64    `json:"origin_after_index"`
	GoooFieldc1a3c4f7b45f56395490cd67dd6a2b9b0483bc98f803d2c0bb1332e2e59ce0b0 string   `json:"origin_conflict_catalog"`
	GoooField7a1e391829bdda16b2c19dbe676cb282046984f7434c534c0d6ca1fd23a3afe3 int64    `json:"origin_conflict_first_index"`
	GoooField43db54858eabe65af5e13659a7953235fbf05896e6df912b21321862497fac96 int64    `json:"origin_conflict_second_index"`
	GoooField32f6bc7303d8456046bdacb58befc2547300db078f3595ec4885ca6a3c721f3e bool     `json:"origin_execution_authorized"`
	GoooFieldd71176a3f8df05d3c0434c75d3c919f13e0b8e3f5b6f4fb3598e70aac2ab55fb string   `json:"context_id"`
	GoooField665125756c2097c40ccb79245b8a1b5d8fe0a6ca26670f19a225715ae9086124 string   `json:"state"`
	GoooField7a9f53964bd85a48d5ccebe411e7bc77883aefb5119c7e26a1d1d508238a831f string   `json:"reason"`
	GoooFieldc0d4c69b1047ae1466879a90149fa0c12e77842d0f10ea0a61313ba8ebe314d6 string   `json:"next_action"`
	GoooFieldd452a118b3d2ea8e0e21d9e9f775899270c367de03f7fb2b2ba951fc43f07909 string   `json:"before_pagination_state"`
	GoooField5abbbf1411716e7ead9235828126d7a0e07deab269de94ef24443a3ad995c51e string   `json:"after_pagination_state"`
	GoooFielda605e189393eedcab3bc677a03639b400103791762d186dd9320895d288e34cc string   `json:"before_configuration_state"`
	GoooFieldf07ce42cdfe6734c7fa15bb19fbf1bfda690d98bd247c84f20ed16504d8b619e string   `json:"after_configuration_state"`
	GoooField08bdafe0584fba0157dcfe1cef5dd7afd97f92529d8cf974cd24b288b30e286d string   `json:"authorization_relation"`
	GoooField99673d3ef58227d44dd56592aaeb606c054cb4acc788ba9e0ac51e035cc63bc5 string   `json:"recording_basis_state"`
	GoooField31656675b6a4fff619236e1c94e7d6a839e9775cf9666a5eea2f5abcb8d55d88 string   `json:"remote_atomicity"`
	GoooFieldfd5ae7ce8a649b10fb188e62e267d80872a4ca8b0ff7c4a3bed54e10b67d43c0 string   `json:"first_problem_side"`
	GoooField3ee033f213c1a2f3dcd5481044589e20f747b75f7edb5daf476b9de3067ce200 string   `json:"first_problem_code"`
	GoooField420181d04dd732ea0c9dc07e036321ed519233a1ed3a856cfef0c5a275328154 int64    `json:"first_problem_capture_index"`
	GoooField982ca34c3f93f04ad90cdf54deee6ac6db12e16b238f715cf810d6452ed3b955 int64    `json:"first_problem_page_index"`
	GoooField7ccfb570dd10abf6ee76352f9de9d400303b73a5b5b45ca2d93c772734595d17 int64    `json:"first_problem_declared_index"`
	GoooFieldecbfc449c7ecc8fe5f35ef316b6ca2fd62950ff0961040fa4a0675ebe722ea5c bool     `json:"contains_synthetic_fixture"`
	GoooFielda82336ed4220901bd38aa04b9239e2cb8b4e2dc87663caa2b5cc73e2af7dad0d bool     `json:"live_removal_proven"`
	GoooField06c83d01d0ffd64e55950b630ff3636a61b2948d8106f502df6dc8f1e09abef3 bool     `json:"execution_authorized"`
	GoooField0e33d0a2d397a1136fdc6eaa7b4f5a526f9378d5c9e0078bed382dbf29ad57ab []string `json:"capture_pagination_states"`
	GoooFieldc1155be6e393234ef746a4b5ba4aed9d7cefb58f1f1f8ece2fb6c300e839707b []string `json:"capture_configuration_states"`
	GoooField9dc8ba8f865b6f2a9d7baac265e44d126a59fcd8eebcc4f80aed98da4252289b []string `json:"capture_recording_bases"`
	GoooField7a74937253eb539f6ba1728a333a65d0006f46c29bebcd917270e285637c83a0 []string `json:"capture_evidence_kinds"`
}

//gooo:generated:end id="gooo://tool-origin/recorded-catalog/v1/result" kind="entity"
//gooo:generated:start id="gooo://tool-origin/v1/input" kind="entity"
type GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a struct {
	GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f string   `json:"context_id"`
	GoooFielda4c9dc33a4f1a4e2640f75b5249d0e467a38d272580c2c9dbcae5326bb590364 string   `json:"target_alias"`
	GoooField160b2f010a78a17180bbbf497218e1108cb818d720b5017c6d4e3f48598fc955 []string `json:"before_servers"`
	GoooField428983b2ba3f869abdae0e3edee6d55b476b3202ce7bf877814496181332a0d6 []string `json:"before_names"`
	GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1 []string `json:"before_aliases"`
	GoooField69649810acac2a0e7327b35cc0935b6fc56c95b826735a6ce301a0fd510dd062 []string `json:"after_servers"`
	GoooField4ffb11f0f796f2b3b81a3f979723322625c0df704433b993fa55370336bd135e []string `json:"after_names"`
	GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98 []string `json:"after_aliases"`
}

//gooo:generated:end id="gooo://tool-origin/v1/input" kind="entity"
//gooo:generated:start id="gooo://tool-origin/v1/result" kind="entity"
type GoooRecorde0ad1c493684691a6d21d05f83c27094f343f3bee137cb4d596f1809dd0be330 struct {
	GoooField7f24f8dc1d6f723e8d49830371f3dc7684d77020312f44feb4721b166a743a7d string `json:"context_id"`
	GoooFieldfb8728181cfded348b301ec43994868412a36d6024eb0feea4857b5e30d8236b string `json:"state"`
	GoooField2f4939454fb06b7f88e1ecc70be0e7ad808274d896dffc1b3fbcb71f75b2ccc5 string `json:"reason"`
	GoooField91bcf6a6e5f244af7b578b292812f2b398122ef299c631fe28231e073aa6c87d int64  `json:"before_index"`
	GoooField655bd17dbb2ec6d2ff98a7187dbdbb593d37752eb5400e5c17e3bf96f62fe4cd int64  `json:"after_index"`
	GoooField2b8efaee9b09888bd39b5c8c164a4d3157ff3b174ae63ec97ae95d2d56b97fd8 string `json:"conflict_catalog"`
	GoooFieldd2d2867afb14dd78731b64142c6345b137cca6dbb7ebad9b367b567e04fd99ef int64  `json:"conflict_first_index"`
	GoooFieldbfacd31df3560849d89122a41fc040f3d329e0614805bfcd1eccbe0894c74b28 int64  `json:"conflict_second_index"`
	GoooFielda660e93ccc2a5d204d69a64193e6825a84e5740ac34e409f2b6f0a9e64ce41f4 bool   `json:"execution_authorized"`
}

//gooo:generated:end id="gooo://tool-origin/v1/result" kind="entity"

//gooo:generated:start id="gooo://tool-origin/definition-binding/v1/assess" kind="activity"
func GoooComposedActivity0(input0 GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a, input1 GoooRecorde9a2c608bd57c725bc756fe66f68f2407910973fe5391c93fdd4d7ad43edd1a3, input2 GoooRecordb012d7ff14e3f1aafd160f4618549815bacf3a72de707e856fc582c05ce63633) GoooRecorde850682663826a57b801017e71179c46fcf778407bbdaa4d91e181f35fd9566c {

	var collection = GoooComposedActivity0Call1(input0, input1)
	var beforeCount int64 = int64(len(input0.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1))
	var afterCount int64 = int64(len(input0.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98))
	var invalid = input2.GoooFielddffb0d9c99a177e97081856bde6ff8aba519d83851f584da97a39b6d10bc6b64 == "" || input2.GoooField4a040797d0dc1c8c73be21832f5044acb789916fc30634be414dc77e866f89a3 != input0.GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f || input2.GoooFieldbbf4118486629c5858a6c16d6cdfe354c5ba0da4725dea18a51b0694ce3a8ca7 != input1.GoooField1f17ccb002e4ab134fff7f78e56e479a421e2baeb938a2e8ff02acef74a0864f
	if int64(len(input2.GoooFielde508df087fed7e2114acffa4644a6f5a8354a5b8795fcda027bdf5bef16e6e25)) != beforeCount || int64(len(input2.GoooField6546d37842d28b2b883459e51402341c9d3983f37e17fed69d04f59789b3f17a)) != beforeCount || int64(len(input2.GoooFieldc04d90981596caf8ecd7ac185159c963eafecb93fbdb614f331eb482c5856ce7)) != beforeCount {
		invalid = true
	}
	if int64(len(input2.GoooField679db5638e7516f0651309847b7eae8e4f2105ecf49820624096f080040f1a9e)) != afterCount || int64(len(input2.GoooField4f177805a0111b1f94862b96e45102f482e4c609f27d319b0d3c022d3eda7722)) != afterCount || int64(len(input2.GoooFielde09fac6641fe463f2034fc949e3f1b834ae96bb87064421eda5623fbd669a8c8)) != afterCount {
		invalid = true
	}
	var beforeDefinition = ""
	var afterDefinition = ""
	var beforeProfile = ""
	var afterProfile = ""
	var beforeCapability = ""
	var afterCapability = ""
	var definitionRelation = "UNASSESSED"
	var profileRelation = "UNASSESSED"
	var capabilityRelation = "UNASSESSED"
	var policyRelation = "UNASSESSED"
	var supportedProfile = "gooo/mcp-tool-definition-json/v1"
	if !invalid && collection.GoooField1db058eff73cda088b309c5d1912f1ba73f7016260f4834d8af0684e4588b952 == "UNCHANGED" {
		var beforeIndex = collection.GoooField03155b7da8f7eaf403d6c6b7946d60c458396df1952e7d8f56208c615b431f51
		var afterIndex = collection.GoooFieldb8c0c4da7c6f7d5ab81a1cd411b40bd018a40d7a8db4858a30aa3106259c9c38
		beforeDefinition = input2.GoooFielde508df087fed7e2114acffa4644a6f5a8354a5b8795fcda027bdf5bef16e6e25[beforeIndex]
		afterDefinition = input2.GoooField679db5638e7516f0651309847b7eae8e4f2105ecf49820624096f080040f1a9e[afterIndex]
		beforeProfile = input2.GoooField6546d37842d28b2b883459e51402341c9d3983f37e17fed69d04f59789b3f17a[beforeIndex]
		afterProfile = input2.GoooField4f177805a0111b1f94862b96e45102f482e4c609f27d319b0d3c022d3eda7722[afterIndex]
		beforeCapability = input2.GoooFieldc04d90981596caf8ecd7ac185159c963eafecb93fbdb614f331eb482c5856ce7[beforeIndex]
		afterCapability = input2.GoooFielde09fac6641fe463f2034fc949e3f1b834ae96bb87064421eda5623fbd669a8c8[afterIndex]
		if beforeProfile != "" && afterProfile != "" {
			profileRelation = "DIFFERENT_SUPPLIED"
			if beforeProfile == afterProfile {
				profileRelation = "SAME_SUPPLIED"
			}
		}
		if beforeProfile == supportedProfile && afterProfile == supportedProfile {
			if beforeDefinition != "" && !GoooComposedActivity0Call2(beforeDefinition) {
				invalid = true
			}
			if afterDefinition != "" && !GoooComposedActivity0Call2(afterDefinition) {
				invalid = true
			}
			if beforeCapability != "" && !GoooComposedActivity0Call2(beforeCapability) {
				invalid = true
			}
			if afterCapability != "" && !GoooComposedActivity0Call2(afterCapability) {
				invalid = true
			}
			if !invalid && beforeDefinition != "" && afterDefinition != "" {
				definitionRelation = "DIFFERENT_SUPPLIED"
				if beforeDefinition == afterDefinition {
					definitionRelation = "SAME_SUPPLIED"
				}
			}
			if !invalid && beforeCapability != "" && afterCapability != "" {
				capabilityRelation = "DIFFERENT_SUPPLIED"
				if beforeCapability == afterCapability {
					capabilityRelation = "SAME_SUPPLIED"
				}
			}
		}
		if input2.GoooFielde8c04553264ef06f247683cc0b5b0806f0f3d79b896b21b9f805f3299023a4cf != "" && input2.GoooFieldeda438bbcda65225bb331d9676abdb21cacf7ad0bc7d777a315ac4bb77d6cec2 != "" {
			policyRelation = "DIFFERENT_SUPPLIED"
			if input2.GoooFielde8c04553264ef06f247683cc0b5b0806f0f3d79b896b21b9f805f3299023a4cf == input2.GoooFieldeda438bbcda65225bb331d9676abdb21cacf7ad0bc7d777a315ac4bb77d6cec2 {
				policyRelation = "SAME_SUPPLIED"
			}
		}
	}
	var state = "UNASSESSED"
	var reason = "SUPPLIED_BINDINGS_NOT_ASSESSED"
	var action = "COMPLETE_BINDING_OBSERVATION"
	var matches = false
	if invalid {
		state = "INVALID_INPUT"
		reason = "INVALID_OR_UNBOUND_DEFINITION_COLUMNS"
		action = "FIX_BOUND_INPUT"
		definitionRelation = "UNASSESSED"
		capabilityRelation = "UNASSESSED"
		policyRelation = "UNASSESSED"
	} else if collection.GoooField1db058eff73cda088b309c5d1912f1ba73f7016260f4834d8af0684e4588b952 != "UNCHANGED" {
		state = "ORIGIN_NOT_UNCHANGED"
		reason = "ORIGIN_BINDING_REQUIRES_REEVALUATION"
		action = "REEVALUATE_ORIGIN"
	} else if collection.GoooField665125756c2097c40ccb79245b8a1b5d8fe0a6ca26670f19a225715ae9086124 == "AUTH_CONTEXT_CHANGED" {
		state = "REEVALUATE_AUTHORIZATION"
		reason = "RECORDED_AUTHORIZATION_CONTEXT_CHANGED"
		action = "REEVALUATE_POLICY"
	} else if collection.GoooField665125756c2097c40ccb79245b8a1b5d8fe0a6ca26670f19a225715ae9086124 != "RECORDED_CONTEXT_COMPLETE" {
		reason = "COLLECTION_CONTEXT_NOT_ESTABLISHED"
		action = collection.GoooFieldc0d4c69b1047ae1466879a90149fa0c12e77842d0f10ea0a61313ba8ebe314d6
	} else if definitionRelation == "DIFFERENT_SUPPLIED" {
		state = "REEVALUATE_DEFINITION"
		reason = "SUPPLIED_TOOL_DEFINITION_CHANGED"
		action = "REEVALUATE_POLICY"
	} else if capabilityRelation == "DIFFERENT_SUPPLIED" {
		state = "REEVALUATE_CAPABILITY"
		reason = "SUPPLIED_CAPABILITY_CLAIM_CHANGED"
		action = "REEVALUATE_POLICY"
	} else if policyRelation == "DIFFERENT_SUPPLIED" {
		state = "REEVALUATE_POLICY"
		reason = "SUPPLIED_POLICY_REVISION_CHANGED"
		action = "REEVALUATE_POLICY"
	} else if beforeProfile != supportedProfile || afterProfile != supportedProfile {
		reason = "DEFINITION_PROFILE_NOT_COMPARABLE"
	} else if definitionRelation == "UNASSESSED" || capabilityRelation == "UNASSESSED" || policyRelation == "UNASSESSED" {
		reason = "DEFINITION_CAPABILITY_OR_POLICY_NOT_OBSERVED"
	} else {
		state = "BINDINGS_MATCH"
		reason = "SAME_SUPPLIED_BINDINGS_ONLY"
		action = "CONTINUE_HOST_VALIDATION"
		matches = true
	}
	return GoooRecorde850682663826a57b801017e71179c46fcf778407bbdaa4d91e181f35fd9566c{
		GoooField3e8344c828080d05509ccae691e2679888337debb63c79d5d952a78df5be435c: input2.GoooFielddffb0d9c99a177e97081856bde6ff8aba519d83851f584da97a39b6d10bc6b64, GoooFielddadb8bec311e8cc220df5f05a613f0cf1a385c94175d32be1a2770969270ec04: collection.GoooField6eb89ea461a7030e08b98a61049763d85e84d78215ef8cf5e156e8f8d3ddfa66, GoooFieldd1f19447d3fa701c1671ea38f76e8a75ab114d77ca7a4752f9f0c75d12af00b4: collection.GoooFieldd71176a3f8df05d3c0434c75d3c919f13e0b8e3f5b6f4fb3598e70aac2ab55fb,
		GoooField40c37d648d894360192076b7cbbadae071f4c8a096bf0868d050ca1dad5a680b: collection.GoooField1db058eff73cda088b309c5d1912f1ba73f7016260f4834d8af0684e4588b952, GoooFieldd88ae7cf4ca18f299c810cb81a16cfa908a2d411682d0718f77073407eb9c276: collection.GoooField26edd9863bab28c16b1fd1848bb6a4ca71b8750dd00fcbbbc636e74ff9a87a88,
		GoooFieldfa3b334a646702521460a06556881e5740715f9886ada34560c7729853843598: collection.GoooField665125756c2097c40ccb79245b8a1b5d8fe0a6ca26670f19a225715ae9086124, GoooField75c6b28aed721730a5bb98c77bd1be78c34c20b5fc331b22dc764b87198eaedb: collection.GoooField7a9f53964bd85a48d5ccebe411e7bc77883aefb5119c7e26a1d1d508238a831f,
		GoooFieldf016688d9f62cb4bba3f8938fc1d68be9157949da0ae5660ed24bc88f3595037: collection.GoooField03155b7da8f7eaf403d6c6b7946d60c458396df1952e7d8f56208c615b431f51, GoooField4794366c09b6e0dd22239a0870c5ab574201c16b2394c8caf003c695a434529d: collection.GoooFieldb8c0c4da7c6f7d5ab81a1cd411b40bd018a40d7a8db4858a30aa3106259c9c38,
		GoooFielde08c9b681c42cc184d057a504ddfe26f8c7d549f936d7b46ad453bd8be1530e1: collection.GoooField3ee033f213c1a2f3dcd5481044589e20f747b75f7edb5daf476b9de3067ce200,
		GoooFieldc240e43a4309c1b6244c0086e03d179813b35239ed9f60172bb9c10cfc29d664: definitionRelation, GoooFielde8e0375a071d7f3330f89b6f1f1530af674225a19d4b3f25bbb7ab160aa8f4b1: profileRelation, GoooField50e198e9819d0520239a2a4fe68f965ac6e2be7b631e2fddfab2589d1dcd096f: capabilityRelation,
		GoooField95fc988a55e0e2d357aa2713a36065b21c961dae27a20016a2cbc0df44cb2157: collection.GoooField08bdafe0584fba0157dcfe1cef5dd7afd97f92529d8cf974cd24b288b30e286d, GoooField6088657cc00835dab667b54aacfcbb582aa70a0bf674b337aa4237159b58b6b5: policyRelation,
		GoooFieldb0feaf12f92f2d7fbd8b27dcbac1dff3a1b171584a802fbbab7bc74394c975d6: beforeDefinition, GoooFielda0b4c083415c69709761fb58e2ab9f519e48194bb983d30bb2d5071ca5fb90a5: afterDefinition, GoooField6de54242e2089835bdbd6a93f255cdd11345970c96a94c0cb7c5116cf7955103: beforeProfile, GoooFielda7051e64d27f3e52aa6064efb498d6bd80351be99ead88b1553da1254923e3be: afterProfile,
		GoooField6de71529dd215103adc3c534804eb46eb1572a18a8ad591dbef068e9d93f6ad8: beforeCapability, GoooField3dedc07522eb3bcf5056f95580514abcc6ee1b0064a1db0d22734a410982b347: afterCapability, GoooField84088f66e4d707bcee599bd18d563386ed8ae8c78b567462a77da35e5a13354c: input2.GoooFielde8c04553264ef06f247683cc0b5b0806f0f3d79b896b21b9f805f3299023a4cf, GoooField22f66e6d5f5b6f312376f95fbd54af8be38c05b7035524e88a92a51339172548: input2.GoooFieldeda438bbcda65225bb331d9676abdb21cacf7ad0bc7d777a315ac4bb77d6cec2,
		GoooField9d633c471165a6437dcb25bc8ce160c7159cee2593bc736ef145f9f99c040680: state, GoooFieldf80ada518a9793b77927e9e89d623e83af40a28284f8acb50a335bf76e7bc2db: reason, GoooFielda822e777dfdbbd9b93d26d42b467286821e70057fdbe7de67196c570bfef9e9f: action, GoooFieldd7086a5c8879fc94864722bb800b920c4ce1d70212a77f0adbd1ea050014df61: matches,
		GoooFieldc2020d8241f0f19c3ffb0528074adc46c4a95486b88779d1634fa074aa511c46: false, GoooField6ae444a93a7cb2b70e05c669cef17675c658676e471108f9d43b0e3535cf7d71: false, GoooField4868b45e2b24e4f899e9c4763ecedaf2beea8c1e936651953a96d55a4a78dcec: false,
		GoooFieldefd6184e484efa8bdb4cc9789769360936f5e63331a49faf49d160a3aa00d079: collection.GoooFieldecbfc449c7ecc8fe5f35ef316b6ca2fd62950ff0961040fa4a0675ebe722ea5c, GoooFielde1904d75aca8c711898aaf87bdc606812c35ded98056e53334f64c24648c97a9: collection.GoooField31656675b6a4fff619236e1c94e7d6a839e9775cf9666a5eea2f5abcb8d55d88,
	}

}
func GoooComposedActivity0Call0(input GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a) GoooRecorde0ad1c493684691a6d21d05f83c27094f343f3bee137cb4d596f1809dd0be330 {

	var invalid = input.GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f == "" || input.GoooFielda4c9dc33a4f1a4e2640f75b5249d0e467a38d272580c2c9dbcae5326bb590364 == ""
	var beforeIndex int64 = -1
	var afterIndex int64 = -1
	var conflictCatalog = ""
	var first int64 = -1
	var second int64 = -1
	if len(input.GoooField160b2f010a78a17180bbbf497218e1108cb818d720b5017c6d4e3f48598fc955) != len(input.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1) || len(input.GoooField428983b2ba3f869abdae0e3edee6d55b476b3202ce7bf877814496181332a0d6) != len(input.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1) {
		invalid = true
	}
	if len(input.GoooField69649810acac2a0e7327b35cc0935b6fc56c95b826735a6ce301a0fd510dd062) != len(input.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98) || len(input.GoooField4ffb11f0f796f2b3b81a3f979723322625c0df704433b993fa55370336bd135e) != len(input.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98) {
		invalid = true
	}
	if !invalid {
		var index int64 = 0
		for _, alias := range input.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1 {
			if alias == "" || input.GoooField160b2f010a78a17180bbbf497218e1108cb818d720b5017c6d4e3f48598fc955[index] == "" || input.GoooField428983b2ba3f869abdae0e3edee6d55b476b3202ce7bf877814496181332a0d6[index] == "" {
				invalid = true
			}
			index = index + 1
		}
	}
	if !invalid {
		var index int64 = 0
		for _, alias := range input.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98 {
			if alias == "" || input.GoooField69649810acac2a0e7327b35cc0935b6fc56c95b826735a6ce301a0fd510dd062[index] == "" || input.GoooField4ffb11f0f796f2b3b81a3f979723322625c0df704433b993fa55370336bd135e[index] == "" {
				invalid = true
			}
			index = index + 1
		}
	}
	if !invalid {
		var index int64 = 0
		for _, alias := range input.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1 {
			var prior int64 = 0
			for _, previous := range input.GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1 {
				if conflictCatalog == "" && prior < index && previous == alias {
					conflictCatalog = "BEFORE"
					first = prior
					second = index
				}
				prior = prior + 1
			}
			if alias == input.GoooFielda4c9dc33a4f1a4e2640f75b5249d0e467a38d272580c2c9dbcae5326bb590364 {
				beforeIndex = index
			}
			index = index + 1
		}
	}
	if !invalid {
		var index int64 = 0
		for _, alias := range input.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98 {
			var prior int64 = 0
			for _, previous := range input.GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98 {
				if conflictCatalog == "" && prior < index && previous == alias {
					conflictCatalog = "AFTER"
					first = prior
					second = index
				}
				prior = prior + 1
			}
			if alias == input.GoooFielda4c9dc33a4f1a4e2640f75b5249d0e467a38d272580c2c9dbcae5326bb590364 {
				afterIndex = index
			}
			index = index + 1
		}
	}
	var state = "ABSENT"
	var reason = "ALIAS_NOT_IN_SUPPLIED_CATALOGS"
	if invalid {
		state = "INVALID_INPUT"
		reason = "INVALID_CATALOG_COLUMNS"
		beforeIndex = -1
		afterIndex = -1
	} else if conflictCatalog != "" {
		state = "AMBIGUOUS"
		reason = "DUPLICATE_EXPOSED_NAME"
		beforeIndex = -1
		afterIndex = -1
	} else if beforeIndex >= 0 && afterIndex >= 0 {
		if input.GoooField160b2f010a78a17180bbbf497218e1108cb818d720b5017c6d4e3f48598fc955[beforeIndex] == input.GoooField69649810acac2a0e7327b35cc0935b6fc56c95b826735a6ce301a0fd510dd062[afterIndex] && input.GoooField428983b2ba3f869abdae0e3edee6d55b476b3202ce7bf877814496181332a0d6[beforeIndex] == input.GoooField4ffb11f0f796f2b3b81a3f979723322625c0df704433b993fa55370336bd135e[afterIndex] {
			state = "UNCHANGED"
			reason = "SAME_DECLARED_ORIGIN"
		} else {
			state = "ORIGIN_CHANGED"
			reason = "DIFFERENT_DECLARED_ORIGIN"
		}
	} else if beforeIndex >= 0 {
		state = "REMOVED"
		reason = "ALIAS_REMOVED_FROM_SUPPLIED_CATALOG"
	} else if afterIndex >= 0 {
		state = "ADDED"
		reason = "ALIAS_ADDED_TO_SUPPLIED_CATALOG"
	}
	return GoooRecorde0ad1c493684691a6d21d05f83c27094f343f3bee137cb4d596f1809dd0be330{GoooField7f24f8dc1d6f723e8d49830371f3dc7684d77020312f44feb4721b166a743a7d: input.GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f, GoooFieldfb8728181cfded348b301ec43994868412a36d6024eb0feea4857b5e30d8236b: state, GoooField2f4939454fb06b7f88e1ecc70be0e7ad808274d896dffc1b3fbcb71f75b2ccc5: reason, GoooField91bcf6a6e5f244af7b578b292812f2b398122ef299c631fe28231e073aa6c87d: beforeIndex, GoooField655bd17dbb2ec6d2ff98a7187dbdbb593d37752eb5400e5c17e3bf96f62fe4cd: afterIndex, GoooField2b8efaee9b09888bd39b5c8c164a4d3157ff3b174ae63ec97ae95d2d56b97fd8: conflictCatalog, GoooFieldd2d2867afb14dd78731b64142c6345b137cca6dbb7ebad9b367b567e04fd99ef: first, GoooFieldbfacd31df3560849d89122a41fc040f3d329e0614805bfcd1eccbe0894c74b28: second, GoooFielda660e93ccc2a5d204d69a64193e6825a84e5740ac34e409f2b6f0a9e64ce41f4: false}

}
func GoooComposedActivity0Call1(input0 GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a, input1 GoooRecorde9a2c608bd57c725bc756fe66f68f2407910973fe5391c93fdd4d7ad43edd1a3) GoooRecordaa12040aa48df656a2672ae2e8b3cc715b06749610492e678c3c0517d3230a64 {

	var core = GoooComposedActivity0Call0(input0)
	var count int64 = int64(len(input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f))
	var declaredCount int64 = int64(len(input1.GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9))
	var pageCount int64 = int64(len(input1.GoooField8b9de0aee82e1beeeda3e157dc4b7bbf377f1c523048f5b2ece15572e3b50872))
	var invalid = input1.GoooField1f17ccb002e4ab134fff7f78e56e479a421e2baeb938a2e8ff02acef74a0864f == "" || input1.GoooFieldc05ad3dd3a6d25ad68b7a59dab7a6512961fca0d8e37d6a13835a7967fcf3281 == "" || input1.GoooFieldc05ad3dd3a6d25ad68b7a59dab7a6512961fca0d8e37d6a13835a7967fcf3281 != input0.GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f || input1.GoooFieldd7d6194681376b9df506f52d2a3391beb46d3ee3708b04201ffdcfe4b41e433c == "" || input1.GoooField505da28ceeb71a548fe610ffe699be0f3c61068c325bccaebaaeac4fe7fd8a3f == ""
	if count > 16 || declaredCount > 16 || pageCount > 256 {
		invalid = true
	}
	if int64(len(input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd)) != declaredCount {
		invalid = true
	}
	if int64(len(input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29)) != declaredCount {
		invalid = true
	}
	if int64(len(input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFielde97c146c5f3b6d5cfd44ba482dba123b232a0de4297f1370f5278f3a08f98380)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldfcc990dd181cc3038110126b349fdcd444abe34593b11c6aba5cbc16336f700d)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField45733addc9cb65fbddbfc376d6abc9a47a76a7886536db99cbb1c1fc77cfa72d)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField2d15a2ff2e7598d651442f42a26ce28b57ad4e024633727c14a5d8ca496df31f)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldda2dd52494a43c9e409ea904715ae0a7ad29f03d34e0aa38d7dd96ec51471f78)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldbde4255daed43f9499fee96e6e8e360c2d7004679bae37fc68ed20714e43414d)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField3b2bebd67139f53b88d71c330ab27cc0fefef2de74c210c7a6a9e7f6d5c02caf)) != count {
		invalid = true
	}
	if int64(len(input1.GoooFieldfa7abe9325a28ea644e860fcfc818d668cd559be36fd94587a82e60a969894d0)) != count {
		invalid = true
	}
	if int64(len(input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooField6d59473999bc855a5f11e2102f04dc92272d390213dc7c4e25ef55470bf99489)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooFieldafda383193e738c3c19bc9a49361d319ef33dad8017c1ec3e130253ff6306be2)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooField79c241d258a35a0b8cc84130750730aa9a55cb5460b84856814ce307dca4e0a0)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooFielddcee9b84c5b08be7d7152be4ede578ec076160f0ab2e6e76ffdc659047e00de1)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooField5fc11073284805be697146fe7cb776a1321e6194535674f6c58c76be4c185c4e)) != pageCount {
		invalid = true
	}
	if int64(len(input1.GoooFieldfe4b54b52ee4ed272b1cc435df15713dd13d2d544ba28638a72cf8f44a5238da)) != pageCount {
		invalid = true
	}
	var basisState = "CAPTURE_TIME_ONLY"
	var synthetic = false
	for _, basis := range input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0 {
		if basis == "RETROSPECTIVE" && basisState != "UNKNOWN_PRESENT" {
			basisState = "RETROSPECTIVE_PRESENT"
		}
		if basis == "UNKNOWN" {
			basisState = "UNKNOWN_PRESENT"
		}
	}
	for _, kind := range input1.GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb {
		if kind == "SYNTHETIC_FIXTURE" {
			synthetic = true
		}
	}
	var declaredBefore int64 = 0
	var declaredAfter int64 = 0
	var capturedBefore int64 = 0
	var capturedAfter int64 = 0
	if !invalid {
		var d int64 = 0
		for _, side := range input1.GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9 {
			if side != "BEFORE" && side != "AFTER" {
				invalid = true
			}
			if input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd[d] == "" || input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29[d] == "" {
				invalid = true
			}
			if side == "BEFORE" {
				declaredBefore = declaredBefore + 1
			} else {
				declaredAfter = declaredAfter + 1
			}
			var prior int64 = 0
			for _, earlier := range input1.GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9 {
				if prior < d && earlier == side && input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd[prior] == input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd[d] && input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29[prior] == input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29[d] {
					invalid = true
				}
				prior = prior + 1
			}
			d = d + 1
		}
		var c int64 = 0
		for _, side := range input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f {
			if side != "BEFORE" && side != "AFTER" {
				invalid = true
			}
			if side == "BEFORE" {
				capturedBefore = capturedBefore + 1
			} else {
				capturedAfter = capturedAfter + 1
			}
			if input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[c] == "" || input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[c] == "" {
				invalid = true
			}
			if input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0[c] != "CAPTURE_TIME" && input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0[c] != "RETROSPECTIVE" && input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0[c] != "UNKNOWN" {
				invalid = true
			}
			if input1.GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb[c] != "SYNTHETIC_FIXTURE" && input1.GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb[c] != "RECORDED_OBSERVATION" {
				invalid = true
			}
			if input1.GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3[c] != "CONTINUOUS" && input1.GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3[c] != "ENDPOINTS_ONLY" && input1.GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3[c] != "UNKNOWN" {
				invalid = true
			}
			if input1.GoooFielde97c146c5f3b6d5cfd44ba482dba123b232a0de4297f1370f5278f3a08f98380[c] != "UNKNOWN" {
				invalid = true
			}
			if input1.GoooFieldfcc990dd181cc3038110126b349fdcd444abe34593b11c6aba5cbc16336f700d[c] == "" {
				invalid = true
			}
			if input1.GoooField45733addc9cb65fbddbfc376d6abc9a47a76a7886536db99cbb1c1fc77cfa72d[c] == "" {
				invalid = true
			}
			if input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f[c] == "" {
				invalid = true
			}
			if input1.GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726[c] == "" {
				invalid = true
			}
			if input1.GoooField2d15a2ff2e7598d651442f42a26ce28b57ad4e024633727c14a5d8ca496df31f[c] == "" {
				invalid = true
			}
			if input1.GoooFieldda2dd52494a43c9e409ea904715ae0a7ad29f03d34e0aa38d7dd96ec51471f78[c] == "" {
				invalid = true
			}
			if input1.GoooFieldbde4255daed43f9499fee96e6e8e360c2d7004679bae37fc68ed20714e43414d[c] == "" {
				invalid = true
			}
			if input1.GoooField3b2bebd67139f53b88d71c330ab27cc0fefef2de74c210c7a6a9e7f6d5c02caf[c] == "" {
				invalid = true
			}
			var declared = false
			var declarationIndex int64 = 0
			for _, declaredSide := range input1.GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9 {
				if declaredSide == side && input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd[declarationIndex] == input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[c] && input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29[declarationIndex] == input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[c] {
					declared = true
				}
				declarationIndex = declarationIndex + 1
			}
			if !declared {
				invalid = true
			}
			var prior int64 = 0
			for _, earlier := range input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f {
				if prior < c && earlier == side && input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[prior] == input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[c] && input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[prior] == input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[c] {
					invalid = true
				}
				prior = prior + 1
			}
			c = c + 1
		}
		if declaredBefore > 8 || declaredAfter > 8 || capturedBefore > 8 || capturedAfter > 8 {
			invalid = true
		}
		var p int64 = 0
		var previousCapture int64 = -1
		for _, capture := range input1.GoooField8b9de0aee82e1beeeda3e157dc4b7bbf377f1c523048f5b2ece15572e3b50872 {
			if capture < 0 || capture >= int64(count) || capture < previousCapture {
				invalid = true
			}
			previousCapture = capture
			if input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5[p] != "ABSENT" && input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5[p] != "STRING" {
				invalid = true
			}
			if input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5[p] == "ABSENT" && input1.GoooField6d59473999bc855a5f11e2102f04dc92272d390213dc7c4e25ef55470bf99489[p] != "" {
				invalid = true
			}
			if input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f[p] != "ABSENT" && input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f[p] != "NULL" && input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f[p] != "STRING" {
				invalid = true
			}
			if input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f[p] != "STRING" && input1.GoooFieldafda383193e738c3c19bc9a49361d319ef33dad8017c1ec3e130253ff6306be2[p] != "" {
				invalid = true
			}
			if input1.GoooField79c241d258a35a0b8cc84130750730aa9a55cb5460b84856814ce307dca4e0a0[p] == "" || input1.GoooFielddcee9b84c5b08be7d7152be4ede578ec076160f0ab2e6e76ffdc659047e00de1[p] == "" || input1.GoooField5fc11073284805be697146fe7cb776a1321e6194535674f6c58c76be4c185c4e[p] == "" || input1.GoooFieldfe4b54b52ee4ed272b1cc435df15713dd13d2d544ba28638a72cf8f44a5238da[p] == "" {
				invalid = true
			}
			p = p + 1
		}
	}
	var beforePages = "COMPLETE"
	var afterPages = "COMPLETE"
	var beforeConfig = "RECORDED_STABLE"
	var afterConfig = "RECORDED_STABLE"
	var authorization = "SAME_RECORDED"
	var pageState0 = "INVALID"
	var configState0 = "UNKNOWN"
	var pageState1 = "INVALID"
	var configState1 = "UNKNOWN"
	var pageState2 = "INVALID"
	var configState2 = "UNKNOWN"
	var pageState3 = "INVALID"
	var configState3 = "UNKNOWN"
	var pageState4 = "INVALID"
	var configState4 = "UNKNOWN"
	var pageState5 = "INVALID"
	var configState5 = "UNKNOWN"
	var pageState6 = "INVALID"
	var configState6 = "UNKNOWN"
	var pageState7 = "INVALID"
	var configState7 = "UNKNOWN"
	var pageState8 = "INVALID"
	var configState8 = "UNKNOWN"
	var pageState9 = "INVALID"
	var configState9 = "UNKNOWN"
	var pageState10 = "INVALID"
	var configState10 = "UNKNOWN"
	var pageState11 = "INVALID"
	var configState11 = "UNKNOWN"
	var pageState12 = "INVALID"
	var configState12 = "UNKNOWN"
	var pageState13 = "INVALID"
	var configState13 = "UNKNOWN"
	var pageState14 = "INVALID"
	var configState14 = "UNKNOWN"
	var pageState15 = "INVALID"
	var configState15 = "UNKNOWN"
	var incomplete = false
	var unstable = false
	var unknown = false
	var incompleteSide = ""
	var incompleteCapture int64 = -1
	var incompletePage int64 = -1
	var incompleteDeclared int64 = -1
	var incompleteCode = ""
	var unstableSide = ""
	var unstableCapture int64 = -1
	var unstablePage int64 = -1
	var unknownSide = ""
	var unknownCapture int64 = -1
	var unknownCode = ""
	var authCapture int64 = -1
	var authSide = ""
	if input1.GoooFieldd7d6194681376b9df506f52d2a3391beb46d3ee3708b04201ffdcfe4b41e433c != input1.GoooField505da28ceeb71a548fe610ffe699be0f3c61068c325bccaebaaeac4fe7fd8a3f {
		authorization = "CHANGED"
	}
	if !invalid {
		var d int64 = 0
		for _, side := range input1.GoooField70023faf8fc5886a80f194ae55cfd6e001c979fcc70cd52303630c65239e03f9 {
			var found = false
			var c int64 = 0
			for _, capturedSide := range input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f {
				if capturedSide == side && input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[c] == input1.GoooFieldf829417845cbbc308c9e6bdf03137939281e6126a7700eb065a20a81b64194dd[d] && input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[c] == input1.GoooField02f80411084e09f4bc05878c8c3d750f1abb6e1c8c38031b787dd7f696a76b29[d] {
					found = true
				}
				c = c + 1
			}
			if !found {
				if !incomplete {
					incompleteSide = side
					incompleteDeclared = d
					incompleteCode = "MISSING_DECLARED_CAPTURE"
				}
				incomplete = true
				if side == "BEFORE" {
					beforePages = "INCOMPLETE"
				} else {
					afterPages = "INCOMPLETE"
				}
			}
			d = d + 1
		}
		var c int64 = 0
		for _, side := range input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f {
			var pageStatus = "COMPLETE"
			var configStatus = "RECORDED_STABLE"
			var capturedPages int64 = 0
			var previousKind = ""
			var previousCursor = ""
			var lastPage int64 = -1
			var badPage int64 = -1
			var pageCode = ""
			var changedPage int64 = -1
			var changed = input1.GoooFieldfa7abe9325a28ea644e860fcfc818d668cd559be36fd94587a82e60a969894d0[c] || input1.GoooFieldfcc990dd181cc3038110126b349fdcd444abe34593b11c6aba5cbc16336f700d[c] != input1.GoooField45733addc9cb65fbddbfc376d6abc9a47a76a7886536db99cbb1c1fc77cfa72d[c] || input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f[c] != input1.GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726[c] || input1.GoooField2d15a2ff2e7598d651442f42a26ce28b57ad4e024633727c14a5d8ca496df31f[c] != input1.GoooFieldda2dd52494a43c9e409ea904715ae0a7ad29f03d34e0aa38d7dd96ec51471f78[c] || input1.GoooFieldbde4255daed43f9499fee96e6e8e360c2d7004679bae37fc68ed20714e43414d[c] != input1.GoooField3b2bebd67139f53b88d71c330ab27cc0fefef2de74c210c7a6a9e7f6d5c02caf[c]
			var p int64 = 0
			for _, capture := range input1.GoooField8b9de0aee82e1beeeda3e157dc4b7bbf377f1c523048f5b2ece15572e3b50872 {
				if capture == c {
					var broken = false
					var code = ""
					if capturedPages == 0 {
						if input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5[p] != "ABSENT" {
							broken = true
							code = "MISSING_INITIAL_PAGE"
						}
					} else {
						if previousKind != "STRING" || input1.GoooField0c89786f42b5a53eac12b46b56cc8818aacb786b8656b6715a2283886038bea5[p] != "STRING" || input1.GoooField6d59473999bc855a5f11e2102f04dc92272d390213dc7c4e25ef55470bf99489[p] != previousCursor {
							broken = true
							code = "CURSOR_LINK_MISMATCH"
						}
					}
					if broken {
						if pageStatus == "COMPLETE" {
							badPage = p
							pageCode = code
						}
						pageStatus = "INCOMPLETE"
					}
					if input1.GoooField79c241d258a35a0b8cc84130750730aa9a55cb5460b84856814ce307dca4e0a0[p] != input1.GoooFieldfcc990dd181cc3038110126b349fdcd444abe34593b11c6aba5cbc16336f700d[c] || input1.GoooFielddcee9b84c5b08be7d7152be4ede578ec076160f0ab2e6e76ffdc659047e00de1[p] != input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f[c] || input1.GoooField5fc11073284805be697146fe7cb776a1321e6194535674f6c58c76be4c185c4e[p] != input1.GoooField2d15a2ff2e7598d651442f42a26ce28b57ad4e024633727c14a5d8ca496df31f[c] || input1.GoooFieldfe4b54b52ee4ed272b1cc435df15713dd13d2d544ba28638a72cf8f44a5238da[p] != input1.GoooFieldbde4255daed43f9499fee96e6e8e360c2d7004679bae37fc68ed20714e43414d[c] {
						changed = true
						if changedPage == -1 {
							changedPage = p
						}
					}
					previousKind = input1.GoooFieldd07e402b1e467849c56643ee97dff6b36fc0c0b658edff237dd0ab09874ef10f[p]
					previousCursor = input1.GoooFieldafda383193e738c3c19bc9a49361d319ef33dad8017c1ec3e130253ff6306be2[p]
					capturedPages = capturedPages + 1
					lastPage = p
				}
				p = p + 1
			}
			if capturedPages > 16 {
				invalid = true
			}
			if capturedPages == 0 {
				pageStatus = "INCOMPLETE"
				pageCode = "NO_SAVED_PAGES"
			}
			if capturedPages > 0 && previousKind == "STRING" {
				if pageStatus == "COMPLETE" {
					badPage = lastPage
					pageCode = "MISSING_TERMINAL_PAGE"
				}
				pageStatus = "INCOMPLETE"
			}
			if changed {
				configStatus = "CHANGED"
			} else if input1.GoooField031862fce310523b7efcb5b8ed5da49298cdbb422f3762467cb0bf79b1a34de3[c] != "CONTINUOUS" {
				configStatus = "UNKNOWN"
			}
			if c == 0 {
				pageState0 = pageStatus
				configState0 = configStatus
			}
			if c == 1 {
				pageState1 = pageStatus
				configState1 = configStatus
			}
			if c == 2 {
				pageState2 = pageStatus
				configState2 = configStatus
			}
			if c == 3 {
				pageState3 = pageStatus
				configState3 = configStatus
			}
			if c == 4 {
				pageState4 = pageStatus
				configState4 = configStatus
			}
			if c == 5 {
				pageState5 = pageStatus
				configState5 = configStatus
			}
			if c == 6 {
				pageState6 = pageStatus
				configState6 = configStatus
			}
			if c == 7 {
				pageState7 = pageStatus
				configState7 = configStatus
			}
			if c == 8 {
				pageState8 = pageStatus
				configState8 = configStatus
			}
			if c == 9 {
				pageState9 = pageStatus
				configState9 = configStatus
			}
			if c == 10 {
				pageState10 = pageStatus
				configState10 = configStatus
			}
			if c == 11 {
				pageState11 = pageStatus
				configState11 = configStatus
			}
			if c == 12 {
				pageState12 = pageStatus
				configState12 = configStatus
			}
			if c == 13 {
				pageState13 = pageStatus
				configState13 = configStatus
			}
			if c == 14 {
				pageState14 = pageStatus
				configState14 = configStatus
			}
			if c == 15 {
				pageState15 = pageStatus
				configState15 = configStatus
			}
			if pageStatus == "INCOMPLETE" {
				if !incomplete {
					incompleteSide = side
					incompleteCapture = c
					incompletePage = badPage
					incompleteCode = pageCode
				}
				incomplete = true
				if side == "BEFORE" {
					beforePages = "INCOMPLETE"
				} else {
					afterPages = "INCOMPLETE"
				}
			}
			if configStatus == "CHANGED" {
				if !unstable {
					unstableSide = side
					unstableCapture = c
					unstablePage = changedPage
				}
				unstable = true
				if side == "BEFORE" {
					beforeConfig = "CHANGED"
				} else {
					afterConfig = "CHANGED"
				}
			} else if configStatus == "UNKNOWN" {
				if !unknown {
					unknownSide = side
					unknownCapture = c
					unknownCode = "CONFIGURATION_WATCH_UNKNOWN"
				}
				unknown = true
				if side == "BEFORE" && beforeConfig != "CHANGED" {
					beforeConfig = "UNKNOWN"
				}
				if side == "AFTER" && afterConfig != "CHANGED" {
					afterConfig = "UNKNOWN"
				}
			}
			if input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0[c] != "CAPTURE_TIME" {
				if !unknown {
					unknownSide = side
					unknownCapture = c
					unknownCode = "CONTEXT_NOT_RECORDED_AT_CAPTURE"
				}
				unknown = true
			}
			if side == "BEFORE" {
				var other int64 = 0
				for _, otherSide := range input1.GoooFieldc8f0bafffb7055d0bbb70e7d0c016f2cbd94b9de6fbb49e552c1e927b0f2945f {
					if otherSide == "AFTER" && input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[c] == input1.GoooFieldcf53ea3018bd08700151a870be686c0a11de7516ef836ee04d33b731141e4395[other] && input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[c] == input1.GoooField90782cc0e7bbaab3b736ad52fbc8466704ee530346a9ac7441e73028d2c9249b[other] && (input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f[c] != input1.GoooFieldfdc08141012ada1601deda01a71a894a2fce3bcd0fe6a20be65343c524c4ed9f[other] || input1.GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726[c] != input1.GoooField92de4dde1bc56d94ec4466e082b732a171c321e16f1819e13e841d77a59a1726[other]) {
						authorization = "CHANGED"
						if authCapture == -1 {
							authCapture = other
							authSide = "AFTER"
						}
					}
					other = other + 1
				}
			}
			c = c + 1
		}
	}
	var state = "RECORDED_CONTEXT_COMPLETE"
	var reason = "RECORDED_SCOPE_PAGES_AND_CONTEXT_SATISFIED"
	var action = "FOLLOW_ALIAS_RESULT"
	var problemSide = ""
	var problemCapture int64 = -1
	var problemPage int64 = -1
	var problemDeclared int64 = -1
	var problemCode = ""
	if invalid {
		state = "INVALID_CONTEXT"
		reason = "INVALID_COLLECTION_COLUMNS"
		action = "FIX_CAPTURE_RECORD"
		problemCode = reason
		beforePages = "INCOMPLETE"
		afterPages = "INCOMPLETE"
		beforeConfig = "UNKNOWN"
		afterConfig = "UNKNOWN"
		authorization = "UNKNOWN"
		pageState0 = "INVALID"
		configState0 = "UNKNOWN"
		pageState1 = "INVALID"
		configState1 = "UNKNOWN"
		pageState2 = "INVALID"
		configState2 = "UNKNOWN"
		pageState3 = "INVALID"
		configState3 = "UNKNOWN"
		pageState4 = "INVALID"
		configState4 = "UNKNOWN"
		pageState5 = "INVALID"
		configState5 = "UNKNOWN"
		pageState6 = "INVALID"
		configState6 = "UNKNOWN"
		pageState7 = "INVALID"
		configState7 = "UNKNOWN"
		pageState8 = "INVALID"
		configState8 = "UNKNOWN"
		pageState9 = "INVALID"
		configState9 = "UNKNOWN"
		pageState10 = "INVALID"
		configState10 = "UNKNOWN"
		pageState11 = "INVALID"
		configState11 = "UNKNOWN"
		pageState12 = "INVALID"
		configState12 = "UNKNOWN"
		pageState13 = "INVALID"
		configState13 = "UNKNOWN"
		pageState14 = "INVALID"
		configState14 = "UNKNOWN"
		pageState15 = "INVALID"
		configState15 = "UNKNOWN"
	} else if incomplete {
		state = "INCOMPLETE_COLLECTION"
		reason = incompleteCode
		action = "RECAPTURE_DECLARED_SCOPE"
		problemSide = incompleteSide
		problemCapture = incompleteCapture
		problemPage = incompletePage
		problemDeclared = incompleteDeclared
		problemCode = reason
	} else if unstable {
		state = "UNSTABLE_CONFIGURATION"
		reason = "RECORDED_CONTEXT_CHANGED_DURING_CAPTURE"
		action = "RECAPTURE_STABLE_CONTEXT"
		problemSide = unstableSide
		problemCapture = unstableCapture
		problemPage = unstablePage
		problemCode = reason
	} else if unknown {
		state = "COLLECTION_CONTEXT_UNKNOWN"
		reason = unknownCode
		action = "RECORD_CAPTURE_TIME_CONTEXT"
		problemSide = unknownSide
		problemCapture = unknownCapture
		problemCode = reason
	} else if authorization == "CHANGED" {
		state = "AUTH_CONTEXT_CHANGED"
		reason = "DECLARED_AUTHORIZATION_CONTEXT_CHANGED"
		action = "RECONCILE_AUTH_SCOPE"
		problemSide = authSide
		problemCapture = authCapture
		problemCode = reason
	}
	var resultCount int64 = count
	if resultCount > 16 {
		resultCount = 0
	}
	var paginationStates = []string{pageState0, pageState1, pageState2, pageState3, pageState4, pageState5, pageState6, pageState7, pageState8, pageState9, pageState10, pageState11, pageState12, pageState13, pageState14, pageState15}[:resultCount]
	var configurationStates = []string{configState0, configState1, configState2, configState3, configState4, configState5, configState6, configState7, configState8, configState9, configState10, configState11, configState12, configState13, configState14, configState15}[:resultCount]
	return GoooRecordaa12040aa48df656a2672ae2e8b3cc715b06749610492e678c3c0517d3230a64{
		GoooField6eb89ea461a7030e08b98a61049763d85e84d78215ef8cf5e156e8f8d3ddfa66: core.GoooField7f24f8dc1d6f723e8d49830371f3dc7684d77020312f44feb4721b166a743a7d,
		GoooField1db058eff73cda088b309c5d1912f1ba73f7016260f4834d8af0684e4588b952: core.GoooFieldfb8728181cfded348b301ec43994868412a36d6024eb0feea4857b5e30d8236b,
		GoooField26edd9863bab28c16b1fd1848bb6a4ca71b8750dd00fcbbbc636e74ff9a87a88: core.GoooField2f4939454fb06b7f88e1ecc70be0e7ad808274d896dffc1b3fbcb71f75b2ccc5,
		GoooField03155b7da8f7eaf403d6c6b7946d60c458396df1952e7d8f56208c615b431f51: core.GoooField91bcf6a6e5f244af7b578b292812f2b398122ef299c631fe28231e073aa6c87d,
		GoooFieldb8c0c4da7c6f7d5ab81a1cd411b40bd018a40d7a8db4858a30aa3106259c9c38: core.GoooField655bd17dbb2ec6d2ff98a7187dbdbb593d37752eb5400e5c17e3bf96f62fe4cd,
		GoooFieldc1a3c4f7b45f56395490cd67dd6a2b9b0483bc98f803d2c0bb1332e2e59ce0b0: core.GoooField2b8efaee9b09888bd39b5c8c164a4d3157ff3b174ae63ec97ae95d2d56b97fd8,
		GoooField7a1e391829bdda16b2c19dbe676cb282046984f7434c534c0d6ca1fd23a3afe3: core.GoooFieldd2d2867afb14dd78731b64142c6345b137cca6dbb7ebad9b367b567e04fd99ef,
		GoooField43db54858eabe65af5e13659a7953235fbf05896e6df912b21321862497fac96: core.GoooFieldbfacd31df3560849d89122a41fc040f3d329e0614805bfcd1eccbe0894c74b28,
		GoooField32f6bc7303d8456046bdacb58befc2547300db078f3595ec4885ca6a3c721f3e: core.GoooFielda660e93ccc2a5d204d69a64193e6825a84e5740ac34e409f2b6f0a9e64ce41f4,
		GoooFieldd71176a3f8df05d3c0434c75d3c919f13e0b8e3f5b6f4fb3598e70aac2ab55fb: input1.GoooField1f17ccb002e4ab134fff7f78e56e479a421e2baeb938a2e8ff02acef74a0864f,
		GoooField665125756c2097c40ccb79245b8a1b5d8fe0a6ca26670f19a225715ae9086124: state,
		GoooField7a9f53964bd85a48d5ccebe411e7bc77883aefb5119c7e26a1d1d508238a831f: reason,
		GoooFieldc0d4c69b1047ae1466879a90149fa0c12e77842d0f10ea0a61313ba8ebe314d6: action,
		GoooFieldd452a118b3d2ea8e0e21d9e9f775899270c367de03f7fb2b2ba951fc43f07909: beforePages,
		GoooField5abbbf1411716e7ead9235828126d7a0e07deab269de94ef24443a3ad995c51e: afterPages,
		GoooFielda605e189393eedcab3bc677a03639b400103791762d186dd9320895d288e34cc: beforeConfig,
		GoooFieldf07ce42cdfe6734c7fa15bb19fbf1bfda690d98bd247c84f20ed16504d8b619e: afterConfig,
		GoooField08bdafe0584fba0157dcfe1cef5dd7afd97f92529d8cf974cd24b288b30e286d: authorization,
		GoooField99673d3ef58227d44dd56592aaeb606c054cb4acc788ba9e0ac51e035cc63bc5: basisState,
		GoooField31656675b6a4fff619236e1c94e7d6a839e9775cf9666a5eea2f5abcb8d55d88: "UNKNOWN",
		GoooFieldfd5ae7ce8a649b10fb188e62e267d80872a4ca8b0ff7c4a3bed54e10b67d43c0: problemSide,
		GoooField3ee033f213c1a2f3dcd5481044589e20f747b75f7edb5daf476b9de3067ce200: problemCode,
		GoooField420181d04dd732ea0c9dc07e036321ed519233a1ed3a856cfef0c5a275328154: problemCapture,
		GoooField982ca34c3f93f04ad90cdf54deee6ac6db12e16b238f715cf810d6452ed3b955: problemPage,
		GoooField7ccfb570dd10abf6ee76352f9de9d400303b73a5b5b45ca2d93c772734595d17: problemDeclared,
		GoooFieldecbfc449c7ecc8fe5f35ef316b6ca2fd62950ff0961040fa4a0675ebe722ea5c: synthetic,
		GoooFielda82336ed4220901bd38aa04b9239e2cb8b4e2dc87663caa2b5cc73e2af7dad0d: false,
		GoooField06c83d01d0ffd64e55950b630ff3636a61b2948d8106f502df6dc8f1e09abef3: false,
		GoooField0e33d0a2d397a1136fdc6eaa7b4f5a526f9378d5c9e0078bed382dbf29ad57ab: paginationStates,
		GoooFieldc1155be6e393234ef746a4b5ba4aed9d7cefb58f1f1f8ece2fb6c300e839707b: configurationStates,
		GoooField9dc8ba8f865b6f2a9d7baac265e44d126a59fcd8eebcc4f80aed98da4252289b: input1.GoooField39d77fcb67dd96ff6969a903e663db8dd2a955a6ca9bb9d724a6803b22e9cac0,
		GoooField7a74937253eb539f6ba1728a333a65d0006f46c29bebcd917270e285637c83a0: input1.GoooFieldbde1eac2aee2eea08a0ae8abdfc689c913d402300dd7188a056484022d7d74cb,
	}

}
func GoooComposedActivity0Call2(input string) bool {

	if len(input) != 71 {
		return false
	}
	if input[0:7] != "sha256:" {
		return false
	}
	var offset int64 = 0
	for offset = 0; offset < 71; offset = offset + 1 {
		if offset >= 7 {
			var octet = int64(input[offset])
			if !((octet >= 48 && octet <= 57) || (octet >= 97 && octet <= 102)) {
				return false
			}
		}
	}
	return true

}

//gooo:generated:end id="gooo://tool-origin/definition-binding/v1/assess" kind="activity"
