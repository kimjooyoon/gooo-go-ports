package observationkernel

//gooo:generated:start id="gooo://tool-origin/observation-diagnostics/v1/input" kind="entity"
type GoooRecord4f6aa7b0d4067d57ca456699c9d09e485974d221721e78ba6ac67c762ef08b69 struct {
	GoooField014f778d13bc409a5a57089da83afec1d1ae4690e6431e8531fcc74a953680c1 string `json:"context_id"`
	GoooField17b5d1ac1cc906e60b3127adaff64f3b06ab0ab7d8c937760d2ef4c24a927d03 string `json:"field_path"`
	GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd string `json:"field_kind"`
	GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946 string `json:"recording_basis"`
	GoooFieldbb0c0f4329caf256a1c64567af7928cee509e0692cad3570a8cb704dc8526c38 string `json:"value_state"`
	GoooField8f41e2ff6261b012bd0f725f124ce2f40d775bcfce646acc8916c51fb40d745b string `json:"value"`
	GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b string `json:"comparison_scope"`
	GoooField532790890bdee75ee85c846493c26b71950f5cca96e707c3caae20256fda31e7 string `json:"expected_value_state"`
	GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6 string `json:"expected_value"`
	GoooField29c7f5aa364bbc11713a83681a985669162d8b35486ce0bef4dc1faed708b1ae string `json:"expected_field_path"`
	GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c string `json:"source_document"`
	GoooFielda5f2ae61121cf107ff9f694e45dd5069fe0001638e796666c95fb79b61e79f4b string `json:"expected_source_document"`
}

//gooo:generated:end id="gooo://tool-origin/observation-diagnostics/v1/input" kind="entity"
//gooo:generated:start id="gooo://tool-origin/observation-diagnostics/v1/result" kind="entity"
type GoooRecord28191be2c472e10f5bb45b0ce1b3259a31a95735c89b57a7f51fef75e5e493cd struct {
	GoooField1b0fca2b957b1b3b716b52e01676e7e8dc246dbfb128ca20abffec7111a0a6fd string `json:"context_id"`
	GoooFieldf96642d75919cdded6a50c8885656e676ec0da3e5a9d91b5f9dd9dc87ad38c33 string `json:"field_path"`
	GoooField68aaa75515b088f1ecbcec9492a5a45e964e6ea5b8853723bc097d6f7c0936db string `json:"field_kind"`
	GoooFielda1f1a9cc64dd6fc55632c48009848c5d552aff76214e8fbf7986c19c8b21c8fa string `json:"recording_basis"`
	GoooField6024ca95c6af3a184ecaa35b6be370cde821de7909a18c0474c52e758e67a0bd string `json:"value_state"`
	GoooFieldf249f1b63fc58ebef5543cf1fd274ff106914c823119ed2b18f32f7a17a914ac string `json:"value"`
	GoooFieldb32ed5e9287950d1fff4d3849f753b50b07334c5af507f12f082c538ffafd26c string `json:"comparison_scope"`
	GoooField0365f3b240a5479e4f09e07d961c7f69ad94c26b87b2f2023bc7b32872fd8408 string `json:"expected_value_state"`
	GoooField1f292c05c39d83f91e9bbda3581a62566e041eab068bd6c47b6a894fdc027df3 string `json:"expected_value"`
	GoooField5f26a3e2ece43f1f93851a81baba5caedf461321b51ba0b0c3394a34d63759f5 string `json:"expected_field_path"`
	GoooFielda8b5a818f9b1f834759b462e2d196177674863465e7e0798c7dba195819d0824 string `json:"source_document"`
	GoooField8465d53214b46828516766961b4c505de46451e40de548f46f3b5746a4d131e9 string `json:"expected_source_document"`
	GoooFielde3dc5a0c2d59944f9fe75a5535cca4c20ea0fd160643f3618f0b8185f77f2a5e string `json:"state"`
	GoooField1753e0b1b96ed45b171d72536423249234758639a36c1e77c213171116dbc2e0 string `json:"reason"`
	GoooFieldbcbd27d6c2512e04170cd1f61c42486b6bf5f7338a2a9b5560fefb031b2367a0 string `json:"next_action"`
	GoooFieldb891a1470198b021407314dfeb9ccddae4e354ef77112de6e8c7c6d6e67bb6ba bool   `json:"required_for_context"`
	GoooField210b585b3439a20ed71ccfc4749bacb0102a793a81ac0103b9cb2ff896cf7d5b bool   `json:"known_change"`
	GoooField495c95b7cffda5c1d9ce794fc274f65bbc6a1cfc9104348d07eabbf142ce77fd bool   `json:"producer_authenticated"`
	GoooField3760a129caa3664113ae94824dc7e2a35a1a34c29fd9d2176db264d05f3dea32 bool   `json:"behavior_equivalence_proven"`
	GoooField23b254ffff5abcb7c31e47d0443bc603750dfd6d2a2555aa31165f3e16d806a8 bool   `json:"execution_authorized"`
	GoooFieldc0a41c50f8aed1e48dba4321e5b4be2a6627ab27c99aec193e2a0ab146fcf7f4 bool   `json:"live_effect_performed"`
}

//gooo:generated:end id="gooo://tool-origin/observation-diagnostics/v1/result" kind="entity"

//gooo:generated:start id="gooo://tool-origin/observation-diagnostics/v1/assess" kind="activity"
func GoooComposedActivity0(input GoooRecord4f6aa7b0d4067d57ca456699c9d09e485974d221721e78ba6ac67c762ef08b69) GoooRecord28191be2c472e10f5bb45b0ce1b3259a31a95735c89b57a7f51fef75e5e493cd {

	var required = input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "OPAQUE_CONTEXT_ID" || input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "BACKEND_BINDING_REVISION" || input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "CONFIGURATION_REVISION" || input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "PROTOCOL_VERSION"
	var optional = input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "CAPTURE_TIMESTAMP" || input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "POLICY_REVISION"
	var availability = GoooComposedActivity0Call0(input.GoooFieldbb0c0f4329caf256a1c64567af7928cee509e0692cad3570a8cb704dc8526c38, input.GoooField8f41e2ff6261b012bd0f725f124ce2f40d775bcfce646acc8916c51fb40d745b)
	var expected = "NOT_APPLICABLE"
	var invalid = input.GoooField014f778d13bc409a5a57089da83afec1d1ae4690e6431e8531fcc74a953680c1 == "" || !GoooComposedActivity0Call1(input.GoooField17b5d1ac1cc906e60b3127adaff64f3b06ab0ab7d8c937760d2ef4c24a927d03) || (!required && !optional) || availability == "INVALID"
	if input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c != "RECORDED_INPUT" && input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c != "SUPPLIED_POLICY" {
		invalid = true
	}
	if input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd == "POLICY_REVISION" {
		if input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c != "SUPPLIED_POLICY" {
			invalid = true
		}
	} else {
		if input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c != "RECORDED_INPUT" {
			invalid = true
		}
	}
	if len(input.GoooField8f41e2ff6261b012bd0f725f124ce2f40d775bcfce646acc8916c51fb40d745b) > 1024 || len(input.GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6) > 1024 {
		invalid = true
	}
	if input.GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946 != "CAPTURE_TIME" && input.GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946 != "RETROSPECTIVE" && input.GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946 != "UNKNOWN" && input.GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946 != "HOST_DECLARATION" {
		invalid = true
	}
	if input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b == "NONE" {
		if input.GoooFielda5f2ae61121cf107ff9f694e45dd5069fe0001638e796666c95fb79b61e79f4b != "NOT_APPLICABLE" || input.GoooField532790890bdee75ee85c846493c26b71950f5cca96e707c3caae20256fda31e7 != "NOT_APPLICABLE" || input.GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6 != "" || input.GoooField29c7f5aa364bbc11713a83681a985669162d8b35486ce0bef4dc1faed708b1ae != "" {
			invalid = true
		}
	} else {
		if input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b != "SAME_CAPTURE" && input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b != "BETWEEN_CATALOGS" {
			invalid = true
		}
		expected = GoooComposedActivity0Call0(input.GoooField532790890bdee75ee85c846493c26b71950f5cca96e707c3caae20256fda31e7, input.GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6)
		if input.GoooFielda5f2ae61121cf107ff9f694e45dd5069fe0001638e796666c95fb79b61e79f4b != input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c || expected == "INVALID" || !GoooComposedActivity0Call1(input.GoooField29c7f5aa364bbc11713a83681a985669162d8b35486ce0bef4dc1faed708b1ae) || input.GoooField29c7f5aa364bbc11713a83681a985669162d8b35486ce0bef4dc1faed708b1ae == input.GoooField17b5d1ac1cc906e60b3127adaff64f3b06ab0ab7d8c937760d2ef4c24a927d03 {
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
		if input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b != "NONE" {
			if expected == "UNOBSERVED" {
				state = "UNASSESSED"
				reason = "COMPARISON_OBSERVATION_UNAVAILABLE"
				action = "OBTAIN_COMPARISON_OBSERVATION"
			} else {
				state = "SAME_SUPPLIED"
				reason = "SAME_SUPPLIED_TEXT_NOT_AUTHENTICATED"
				if input.GoooField8f41e2ff6261b012bd0f725f124ce2f40d775bcfce646acc8916c51fb40d745b != input.GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6 {
					state = "KNOWN_MISMATCH"
					reason = "DIFFERENT_SUPPLIED_CONTEXT_VALUES"
					action = "REEVALUATE_CONTEXT_CHANGE"
					changed = true
					if input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b == "SAME_CAPTURE" {
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
	return GoooRecord28191be2c472e10f5bb45b0ce1b3259a31a95735c89b57a7f51fef75e5e493cd{
		GoooField1b0fca2b957b1b3b716b52e01676e7e8dc246dbfb128ca20abffec7111a0a6fd: input.GoooField014f778d13bc409a5a57089da83afec1d1ae4690e6431e8531fcc74a953680c1,
		GoooFieldf96642d75919cdded6a50c8885656e676ec0da3e5a9d91b5f9dd9dc87ad38c33: input.GoooField17b5d1ac1cc906e60b3127adaff64f3b06ab0ab7d8c937760d2ef4c24a927d03,
		GoooField68aaa75515b088f1ecbcec9492a5a45e964e6ea5b8853723bc097d6f7c0936db: input.GoooFieldeb82e99b695c4e9ad1140645d4a8785b9a1f7fdd2cf62a6e75b86ee8197a77bd,
		GoooFielda1f1a9cc64dd6fc55632c48009848c5d552aff76214e8fbf7986c19c8b21c8fa: input.GoooField4f48611102a5a2d45035ff9731d4c25277c9109147849ea82061fbd2aca3b946,
		GoooField6024ca95c6af3a184ecaa35b6be370cde821de7909a18c0474c52e758e67a0bd: availability,
		GoooFieldf249f1b63fc58ebef5543cf1fd274ff106914c823119ed2b18f32f7a17a914ac: input.GoooField8f41e2ff6261b012bd0f725f124ce2f40d775bcfce646acc8916c51fb40d745b,
		GoooFieldb32ed5e9287950d1fff4d3849f753b50b07334c5af507f12f082c538ffafd26c: input.GoooField80b07ab575712f83ec1d47d06992ed2944d26ceb7536abf5d199e69c6035c44b,
		GoooField0365f3b240a5479e4f09e07d961c7f69ad94c26b87b2f2023bc7b32872fd8408: expected,
		GoooField1f292c05c39d83f91e9bbda3581a62566e041eab068bd6c47b6a894fdc027df3: input.GoooField5f14501d0c7db7cb2f660f0448039b9f05ec0905cc3a1a6f832bf5efc2c5b8d6,
		GoooField5f26a3e2ece43f1f93851a81baba5caedf461321b51ba0b0c3394a34d63759f5: input.GoooField29c7f5aa364bbc11713a83681a985669162d8b35486ce0bef4dc1faed708b1ae,
		GoooFielda8b5a818f9b1f834759b462e2d196177674863465e7e0798c7dba195819d0824: input.GoooFieldb16edb89368f2b33b2fa040e5118f589abc8cda08918a3b83bb6d7e204ea997c,
		GoooField8465d53214b46828516766961b4c505de46451e40de548f46f3b5746a4d131e9: input.GoooFielda5f2ae61121cf107ff9f694e45dd5069fe0001638e796666c95fb79b61e79f4b,
		GoooFielde3dc5a0c2d59944f9fe75a5535cca4c20ea0fd160643f3618f0b8185f77f2a5e: state,
		GoooField1753e0b1b96ed45b171d72536423249234758639a36c1e77c213171116dbc2e0: reason,
		GoooFieldbcbd27d6c2512e04170cd1f61c42486b6bf5f7338a2a9b5560fefb031b2367a0: action,
		GoooFieldb891a1470198b021407314dfeb9ccddae4e354ef77112de6e8c7c6d6e67bb6ba: required,
		GoooField210b585b3439a20ed71ccfc4749bacb0102a793a81ac0103b9cb2ff896cf7d5b: changed,
		GoooField495c95b7cffda5c1d9ce794fc274f65bbc6a1cfc9104348d07eabbf142ce77fd: false,
		GoooField3760a129caa3664113ae94824dc7e2a35a1a34c29fd9d2176db264d05f3dea32: false,
		GoooField23b254ffff5abcb7c31e47d0443bc603750dfd6d2a2555aa31165f3e16d806a8: false,
		GoooFieldc0a41c50f8aed1e48dba4321e5b4be2a6627ab27c99aec193e2a0ab146fcf7f4: false,
	}

}
func GoooComposedActivity0Call0(input0 string, input1 string) string {

	if input0 == "UNOBSERVED" {
		if input1 == "" {
			return "UNOBSERVED"
		}
		return "INVALID"
	}
	if input0 == "SUPPLIED" {
		if input1 != "" {
			return "SUPPLIED"
		}
		return "INVALID"
	}
	if input0 == "LEGACY_TEXT" {
		if input1 == "" {
			return "UNOBSERVED"
		}
		return "SUPPLIED"
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

//gooo:generated:end id="gooo://tool-origin/observation-diagnostics/v1/assess" kind="activity"
