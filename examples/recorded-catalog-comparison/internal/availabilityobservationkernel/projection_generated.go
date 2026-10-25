package availabilityobservationkernel

//gooo:generated:start id="gooo://tool-origin/explicit-observation/v1/input" kind="entity"
type GoooRecord98c1b4aa9a2f785ded823164623491519fd67a877252e0aab36d3bbecf9355b9 struct {
	GoooField3ec71ad7fee5e7e7e1ce5a75eee69b4124f157380b22b28d476a8c4882eab561 string `json:"context_id"`
	GoooField2cca1da4f8a8518ad3eaa56122ccc7de85fe559081efbaac3e560bce0b9e222f string `json:"field_path"`
	GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 string `json:"field_kind"`
	GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68 string `json:"recording_basis"`
	GoooField1c9fdfbef345d5eb403dcf95cce7cf3c6f1e8662ab40056b3ba0b501cb6aa4dc string `json:"value_state"`
	GoooField1ef8a334b09f27a9cef57a656690e1688266018b520e0dd0420be1a01f141fb5 string `json:"value"`
	GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e string `json:"comparison_scope"`
	GoooField1b72f57a0c469a353577bada7294fe54152ad32bcc2b3ecf58cfaa49d7e27aec string `json:"expected_value_state"`
	GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96 string `json:"expected_value"`
	GoooField19a1301426d2eb8f653f39cce965331efdf38ac8ea03d5ed8dabc4c84363efa1 string `json:"expected_field_path"`
	GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 string `json:"source_document"`
	GoooField4e13fc40a19e8ec8048d5efb30351c44f3664a82a814da9e70ee9d7d2c67d835 string `json:"expected_source_document"`
}

//gooo:generated:end id="gooo://tool-origin/explicit-observation/v1/input" kind="entity"
//gooo:generated:start id="gooo://tool-origin/explicit-observation/v1/result" kind="entity"
type GoooRecorddc40cf68406fcb42a1b49a531ee2f19b51cefcf00a8625277678dde8470e8a10 struct {
	GoooField1b844b2decfdaa93683d333e71773eec84bbdde29aa117c38ac8b503bc390f2f string `json:"context_id"`
	GoooField4166f898cd3406fa145b9ea9e97c1e5f9783eda380ec7befd22817a6aa5ae5e1 string `json:"field_path"`
	GoooField378edd8fa496808748c1d7c73b04a6bdf6696522739bb88c0c554fd72edeb98e string `json:"field_kind"`
	GoooField74a5946d33221f7900026b7c5fba1c158549f6ab7308d08d1380ac9738f06c9a string `json:"recording_basis"`
	GoooField9e1b5c6129e300e832041504598cf1468337a41d34ea9c7fc05abc1900e26a47 string `json:"value_state"`
	GoooField65da2ef592c23f19cd6dd0c82d19635ef8390ccdb3fe290860080c8dcb829587 string `json:"value"`
	GoooField18d0db6f8654b06fab523a110daa00196748446a3129912b87c168dd86793e03 string `json:"comparison_scope"`
	GoooFieldff469995a55ade207881d85810dc7fedc9f3281f647a649851c4be7588699266 string `json:"expected_value_state"`
	GoooFieldc6f03aac0ebcd55dc7d2049241e2c0251036c1ab425c62d307b0ed1793fd585c string `json:"expected_value"`
	GoooField15627d410c81289df487b64d419ce2eec5b847dbac2868ef7ab47f3fed13b841 string `json:"expected_field_path"`
	GoooField1e5228620e1ad319272b4b74eb41958522f9a82ebdc4d4be78594d0e3bb2b8e6 string `json:"source_document"`
	GoooField1dfa619bacb50b32482d085501ce40769ec56ad7e65e8465502e6e597afb2f3c string `json:"expected_source_document"`
	GoooField9680fb127de7c245e0ffc802179f98a922d3bf0cb706ef9844819219b4a5e7c2 string `json:"state"`
	GoooFielddc2d5e54ad65712b51620ccb596a8be2bd7527728192df671433233e3edd9638 string `json:"reason"`
	GoooField5da4b1b8ddedcdb75cf3cbcc74d42f5a2d8d321614a9c692a4178127860dec0f string `json:"next_action"`
	GoooField8daf6211258485a7fd4080c86a6e3b41dea9cb314996d51e391287d4f13a8cca bool   `json:"required_for_context"`
	GoooFieldea3b3589f4114f0b6a6e0775cce2dd98bd31fa6f3ef1d3792e8f4424b21cce2c bool   `json:"known_change"`
	GoooField44bd7dea34e33d51a1a339470cb5c49ad2f18eb3f1f8cc59fcd36b5631bf10c6 bool   `json:"producer_authenticated"`
	GoooField0a6d445c0344b0bf1d5956586d437663e6389a394b8f318cd05a6a82f1a46865 bool   `json:"behavior_equivalence_proven"`
	GoooField3ec173536c8dd22192d3fa61ae6e44aad0313d27a7d9eb53a2d869a3db541076 bool   `json:"execution_authorized"`
	GoooField971e537d41d60ea975eeb11cacb59c4827ee963211bb7d271ca5893b8edc463c bool   `json:"live_effect_performed"`
}

//gooo:generated:end id="gooo://tool-origin/explicit-observation/v1/result" kind="entity"

//gooo:generated:start id="gooo://tool-origin/explicit-observation/v1/assess" kind="activity"
func GoooComposedActivity0(input GoooRecord98c1b4aa9a2f785ded823164623491519fd67a877252e0aab36d3bbecf9355b9) GoooRecorddc40cf68406fcb42a1b49a531ee2f19b51cefcf00a8625277678dde8470e8a10 {

	var required = input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "OPAQUE_CONTEXT_ID" || input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "BACKEND_BINDING_REVISION" || input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "CONFIGURATION_REVISION" || input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "PROTOCOL_VERSION"
	var optional = input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "CAPTURE_TIMESTAMP" || input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "POLICY_REVISION"
	var availability = GoooComposedActivity0Call0(input.GoooField1c9fdfbef345d5eb403dcf95cce7cf3c6f1e8662ab40056b3ba0b501cb6aa4dc, input.GoooField1ef8a334b09f27a9cef57a656690e1688266018b520e0dd0420be1a01f141fb5)
	var expected = "NOT_APPLICABLE"
	var invalid = input.GoooField3ec71ad7fee5e7e7e1ce5a75eee69b4124f157380b22b28d476a8c4882eab561 == "" || !GoooComposedActivity0Call1(input.GoooField2cca1da4f8a8518ad3eaa56122ccc7de85fe559081efbaac3e560bce0b9e222f) || (!required && !optional) || availability == "INVALID"
	if input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 != "RECORDED_INPUT" && input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 != "SUPPLIED_POLICY" {
		invalid = true
	}
	if input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767 == "POLICY_REVISION" {
		if input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 != "SUPPLIED_POLICY" {
			invalid = true
		}
	} else {
		if input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 != "RECORDED_INPUT" {
			invalid = true
		}
	}
	if len(input.GoooField1ef8a334b09f27a9cef57a656690e1688266018b520e0dd0420be1a01f141fb5) > 1024 || len(input.GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96) > 1024 {
		invalid = true
	}
	if input.GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68 != "CAPTURE_TIME" && input.GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68 != "RETROSPECTIVE" && input.GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68 != "UNKNOWN" && input.GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68 != "HOST_DECLARATION" {
		invalid = true
	}
	if input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e == "NONE" {
		if input.GoooField4e13fc40a19e8ec8048d5efb30351c44f3664a82a814da9e70ee9d7d2c67d835 != "NOT_APPLICABLE" || input.GoooField1b72f57a0c469a353577bada7294fe54152ad32bcc2b3ecf58cfaa49d7e27aec != "NOT_APPLICABLE" || input.GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96 != "" || input.GoooField19a1301426d2eb8f653f39cce965331efdf38ac8ea03d5ed8dabc4c84363efa1 != "" {
			invalid = true
		}
	} else {
		if input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e != "SAME_CAPTURE" && input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e != "BETWEEN_CATALOGS" {
			invalid = true
		}
		expected = GoooComposedActivity0Call0(input.GoooField1b72f57a0c469a353577bada7294fe54152ad32bcc2b3ecf58cfaa49d7e27aec, input.GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96)
		if input.GoooField4e13fc40a19e8ec8048d5efb30351c44f3664a82a814da9e70ee9d7d2c67d835 != input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5 || expected == "INVALID" || !GoooComposedActivity0Call1(input.GoooField19a1301426d2eb8f653f39cce965331efdf38ac8ea03d5ed8dabc4c84363efa1) || input.GoooField19a1301426d2eb8f653f39cce965331efdf38ac8ea03d5ed8dabc4c84363efa1 == input.GoooField2cca1da4f8a8518ad3eaa56122ccc7de85fe559081efbaac3e560bce0b9e222f {
			invalid = true
		}
	}
	var state = "SUPPLIED_ONLY"
	var reason = "SUPPLIED_TEXT_NOT_AUTHENTICATED"
	var action = "CONTINUE_DOMAIN_VALIDATION"
	var changed = false
	if availability == "UNOBSERVED" {
		state = "UNASSESSED"
		reason = "REQUIRED_OBSERVATION_UNAVAILABLE"
		action = "OBTAIN_OBSERVATION"
		if optional {
			state = "OPTIONAL_UNOBSERVED"
			reason = "OPTIONAL_OBSERVATION_UNAVAILABLE"
			action = "PRESERVE_UNOBSERVED"
		}
	} else {
		if input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e != "NONE" {
			if expected == "UNOBSERVED" {
				state = "UNASSESSED"
				reason = "COMPARISON_OBSERVATION_UNAVAILABLE"
				action = "OBTAIN_COMPARISON_OBSERVATION"
			} else {
				state = "SAME_SUPPLIED"
				reason = "SAME_SUPPLIED_TEXT_NOT_AUTHENTICATED"
				if input.GoooField1ef8a334b09f27a9cef57a656690e1688266018b520e0dd0420be1a01f141fb5 != input.GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96 {
					state = "KNOWN_MISMATCH"
					reason = "DIFFERENT_SUPPLIED_CONTEXT_VALUES"
					action = "REEVALUATE_CONTEXT_CHANGE"
					changed = true
					if input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e == "SAME_CAPTURE" {
						action = "RECOLLECT_CONTEXT"
					}
				}
			}
		}
	}
	if invalid {
		state = "INVALID_DECLARATION"
		reason = "INVALID_OBSERVATION_FIELD_DECLARATION"
		action = "FIX_DECLARATION"
		changed = false
	}
	return GoooRecorddc40cf68406fcb42a1b49a531ee2f19b51cefcf00a8625277678dde8470e8a10{
		GoooField1b844b2decfdaa93683d333e71773eec84bbdde29aa117c38ac8b503bc390f2f: input.GoooField3ec71ad7fee5e7e7e1ce5a75eee69b4124f157380b22b28d476a8c4882eab561,
		GoooField4166f898cd3406fa145b9ea9e97c1e5f9783eda380ec7befd22817a6aa5ae5e1: input.GoooField2cca1da4f8a8518ad3eaa56122ccc7de85fe559081efbaac3e560bce0b9e222f,
		GoooField378edd8fa496808748c1d7c73b04a6bdf6696522739bb88c0c554fd72edeb98e: input.GoooField48b762732785303ee7bcb519845acc1e46e4ea5b8fafff51f3b8beb3d8545767,
		GoooField74a5946d33221f7900026b7c5fba1c158549f6ab7308d08d1380ac9738f06c9a: input.GoooField221ccea07e828c5784e5340b87643cdc46e4fa916074e247ae7bf489fca72d68,
		GoooField9e1b5c6129e300e832041504598cf1468337a41d34ea9c7fc05abc1900e26a47: availability,
		GoooField65da2ef592c23f19cd6dd0c82d19635ef8390ccdb3fe290860080c8dcb829587: input.GoooField1ef8a334b09f27a9cef57a656690e1688266018b520e0dd0420be1a01f141fb5,
		GoooField18d0db6f8654b06fab523a110daa00196748446a3129912b87c168dd86793e03: input.GoooField94e6bcb05bba476db16ccc0d9e304ff5e067b8a43c4733bf0c79819b36fabe5e,
		GoooFieldff469995a55ade207881d85810dc7fedc9f3281f647a649851c4be7588699266: expected,
		GoooFieldc6f03aac0ebcd55dc7d2049241e2c0251036c1ab425c62d307b0ed1793fd585c: input.GoooFieldd3f8edac0c7ff0117a5e2052923317accb599544c67f0b1d886bbc0936f50f96,
		GoooField15627d410c81289df487b64d419ce2eec5b847dbac2868ef7ab47f3fed13b841: input.GoooField19a1301426d2eb8f653f39cce965331efdf38ac8ea03d5ed8dabc4c84363efa1,
		GoooField1e5228620e1ad319272b4b74eb41958522f9a82ebdc4d4be78594d0e3bb2b8e6: input.GoooField4bab18a5eefd7ce30212df40d3faead3ff12d7711095e421008377d55dc070c5,
		GoooField1dfa619bacb50b32482d085501ce40769ec56ad7e65e8465502e6e597afb2f3c: input.GoooField4e13fc40a19e8ec8048d5efb30351c44f3664a82a814da9e70ee9d7d2c67d835,
		GoooField9680fb127de7c245e0ffc802179f98a922d3bf0cb706ef9844819219b4a5e7c2: state,
		GoooFielddc2d5e54ad65712b51620ccb596a8be2bd7527728192df671433233e3edd9638: reason,
		GoooField5da4b1b8ddedcdb75cf3cbcc74d42f5a2d8d321614a9c692a4178127860dec0f: action,
		GoooField8daf6211258485a7fd4080c86a6e3b41dea9cb314996d51e391287d4f13a8cca: required,
		GoooFieldea3b3589f4114f0b6a6e0775cce2dd98bd31fa6f3ef1d3792e8f4424b21cce2c: changed,
		GoooField44bd7dea34e33d51a1a339470cb5c49ad2f18eb3f1f8cc59fcd36b5631bf10c6: false,
		GoooField0a6d445c0344b0bf1d5956586d437663e6389a394b8f318cd05a6a82f1a46865: false,
		GoooField3ec173536c8dd22192d3fa61ae6e44aad0313d27a7d9eb53a2d869a3db541076: false,
		GoooField971e537d41d60ea975eeb11cacb59c4827ee963211bb7d271ca5893b8edc463c: false,
	}

}
func GoooComposedActivity0Call0(input0 string, input1 string) string {

	if input0 == "UNOBSERVED" {
		return "UNOBSERVED"
	}
	if input0 == "SUPPLIED" {
		if input1 != "" {
			return "SUPPLIED"
		}
		return "INVALID"
	}
	return "INVALID"

}
func GoooComposedActivity0Call1(input string) bool {

	var size int64 = int64(len(input))
	if size == 0 || size > 512 {
		return false
	}
	if input[0] != 47 {
		return false
	}
	var valid = true
	var escaped = false
	for offset := int64(0); offset < 512; offset = offset + 1 {
		if offset < int64(size) {
			var code = input[offset]
			if code == 0 {
				valid = false
			}
			if escaped {
				if code != 48 && code != 49 {
					valid = false
				}
				escaped = false
			} else {
				if code == 126 {
					escaped = true
				}
			}
		}
	}
	return valid && !escaped

}

//gooo:generated:end id="gooo://tool-origin/explicit-observation/v1/assess" kind="activity"
