package snapshotkernel

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

//gooo:generated:start id="gooo://tool-origin/definition-snapshot/v1/compare" kind="activity"
func GoooComposedActivity0(input0 GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25, input1 GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25, input2 GoooRecord4657ec7bd1268ab0c3c0e0c605b3ad32555eed4612703867937f41a7e32492ce) GoooRecord28d4e59537922d347cdeecb8ddf43183852c2c7bf6bf73fabff02468d4fcf78d {

	var before = GoooComposedActivity0Call2(input0, input2)
	var after = GoooComposedActivity0Call2(input1, input2)
	var bi = before.GoooField0ced5d5b1e45be76dde72062f976b5d3761ddbee9d002f6d4b46882df429b63d
	var ai = after.GoooField0ced5d5b1e45be76dde72062f976b5d3761ddbee9d002f6d4b46882df429b63d
	if before.GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 >= 0 || after.GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 >= 0 {
		bi = -1
		ai = -1
	}
	var beforeDefinition = ""
	var afterDefinition = ""
	var beforeProfile = ""
	var afterProfile = ""
	var beforeAnnotation = ""
	var afterAnnotation = ""
	if before.GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 && bi >= 0 {
		beforeDefinition = input0.GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42[bi]
		beforeProfile = input0.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472[bi]
		beforeAnnotation = input0.GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10[bi]
	}
	if after.GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 && ai >= 0 {
		afterDefinition = input1.GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42[ai]
		afterProfile = input1.GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472[ai]
		afterAnnotation = input1.GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10[ai]
	}
	var presence = "UNASSESSED"
	var definitionRelation = "UNASSESSED"
	var annotationRelation = "UNASSESSED"
	var profileRelation = "UNASSESSED"
	var state = "UNASSESSED"
	var reason = "TARGET_OBSERVATION_INCOMPLETE"
	var action = "OBTAIN_DEFINITION_OBSERVATION"
	var supported = "gooo/mcp-tool-definition-json/v1"
	var selectionValid = input2.GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732 != "" && input2.GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3 != "" && GoooComposedActivity0Call0(input2.GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732) && GoooComposedActivity0Call0(input2.GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3)
	if !selectionValid {
		state = "INVALID_INPUT"
		reason = "INVALID_SELECTION_TEXT"
		action = "FIX_BOUND_INPUT"
	} else if !before.GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 || !after.GoooField37758706b299aff2d8400170d36fa6e3e845b7e14f99bf3f762b2d0892416d88 {
		state = "INVALID_INPUT"
		reason = "INVALID_SNAPSHOT_FACTS"
		action = "FIX_BOUND_INPUT"
	} else if before.GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 >= 0 || after.GoooField9dcd12cf12e44ebf00e036b128fcb259fdb12b7ae58c1215a846819f91f3a0d2 >= 0 {
		state = "AMBIGUOUS_NAME"
		reason = "DUPLICATE_OBSERVED_NAME"
		action = "RESOLVE_OBSERVATION_AMBIGUITY"
	} else {
		if bi >= 0 && ai >= 0 {
			presence = "OBSERVED_BOTH"
			profileRelation = "DIFFERENT_OBSERVED"
			if beforeProfile == afterProfile {
				profileRelation = "SAME_OBSERVED"
			}
			if beforeProfile != supported || afterProfile != supported {
				reason = "DEFINITION_PROFILE_NOT_COMPARABLE"
				action = "USE_SUPPORTED_DEFINITION_PROFILE"
			} else {
				definitionRelation = "DIFFERENT_OBSERVED"
				annotationRelation = "DIFFERENT_OBSERVED"
				if beforeDefinition == afterDefinition {
					definitionRelation = "SAME_OBSERVED"
				}
				if beforeAnnotation == afterAnnotation {
					annotationRelation = "SAME_OBSERVED"
				}
				if definitionRelation == "DIFFERENT_OBSERVED" {
					state = "OBSERVED_DEFINITION_CHANGED"
					reason = "SELECTED_DESCRIPTOR_BYTES_DIFFER"
					action = "REVIEW_DEFINITION_CHANGE"
				} else if annotationRelation == "DIFFERENT_OBSERVED" {
					state = "OBSERVED_ANNOTATIONS_CHANGED"
					reason = "SELECTED_ANNOTATION_CLAIMS_DIFFER"
					action = "REVIEW_ANNOTATION_CHANGE"
				} else {
					state = "OBSERVED_DEFINITION_SAME"
					reason = "SELECTED_DESCRIPTOR_AND_ANNOTATIONS_SAME"
					action = "RETAIN_OBSERVATION"
				}
			}
		} else if bi < 0 && ai >= 0 && before.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b == "COMPLETE" {
			presence = "OBSERVED_ADDED"
			state = "OBSERVED_NAME_ADDED"
			reason = "ABSENT_FROM_COMPLETE_BEFORE_PAGES"
			action = "REVIEW_OBSERVED_ADDITION"
		} else if bi >= 0 && ai < 0 && after.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b == "COMPLETE" {
			presence = "OBSERVED_REMOVED"
			state = "OBSERVED_NAME_REMOVED"
			reason = "ABSENT_FROM_COMPLETE_AFTER_PAGES"
			action = "REVIEW_OBSERVED_REMOVAL"
		} else if bi < 0 && ai < 0 && before.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b == "COMPLETE" && after.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b == "COMPLETE" {
			presence = "ABSENT_BOTH_COMPLETE"
			state = "OBSERVED_NAME_ABSENT"
			reason = "ABSENT_FROM_BOTH_COMPLETE_PAGE_SETS"
			action = "RETAIN_OBSERVATION"
		}
	}
	return GoooRecord28d4e59537922d347cdeecb8ddf43183852c2c7bf6bf73fabff02468d4fcf78d{
		GoooFieldbcc80922b7849199853015833e146f8dc0ce58017e282da4c30d3a50a5c5ca0b: input2.GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732, GoooField18273142b6d46141e51852fde687f320069b853369752f530aef18f1a8fd7be2: input2.GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3, GoooField8b81d4a2dc6c2b3b8666b512ccd9f48956d6f9adebd2bd25f45f677b6631b579: state, GoooField00ccf611ba4fe589dd50637cbc50e02f15aff0f1a80b0a49e34337425ca368ff: reason, GoooFieldcb323392c9bbdc0405511ad798b37a4fa69c343da09ffeeaca078020299b9ee5: action,
		GoooField63a6c9a50699cb5b2f8b90c32a7c30bfc634007831dcba9396407c9c7041086e: "SAVED_ROWS_ONLY",
		GoooFieldacaa986a1441b7b3d9eefb633756a841ceda72ad445c439eef7c663467e9ff73: before.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b, GoooField18971056017d1594a1f6bf8e3ddbd1ab92294ffd7bf1eb077d7d9088aafdfa95: after.GoooField9d4f0dcf7dac8a748ac7137d76faa9396176aa843fb065d77870724df327e59b,
		GoooField4830c19362bc88157643f9b0c4daf8530e8d8455ad5e57775cea109a4b28344a: before.GoooField072f8daaf7adbdd4b9af984e5d14416083ab85446ad85b999d20b7248a890783, GoooField3e947e1e651c834ec6356353878d07a79c1eb3b63a08df4127f0da338c6575fb: after.GoooField072f8daaf7adbdd4b9af984e5d14416083ab85446ad85b999d20b7248a890783,
		GoooField89c0f766d0884fd8c9077074fff7c8146e9a3d8421bc15445897903e4abdbd14: before.GoooField3fa7c2b2229d5959dc86ec343f4bc355c1e7e8e705dd7381348fa3e190e2b051, GoooField6a183f2475886028f735482e251d71a655b916980638bc84f12758e99165e1fe: after.GoooField3fa7c2b2229d5959dc86ec343f4bc355c1e7e8e705dd7381348fa3e190e2b051,
		GoooField7b82058191bf48fd3b1ef0db995b066a24615b53545f9ca74edd4ba8c3b1ac03: before.GoooFieldc4c39d3b9f23380f78b98cd111e97a89bc48aa414ef32e5f1f2d38d37acdf4cb, GoooField0a4e010c728fb215a8b94ebbc7444769ed48e5483d343687bf9bee07a93b961e: after.GoooFieldc4c39d3b9f23380f78b98cd111e97a89bc48aa414ef32e5f1f2d38d37acdf4cb,
		GoooFielde0272827e9a9df708ee952803f9c48d8b866be39839893f7d3c5aae6b994c839: bi, GoooFielda3200f5f87e40bdd5f94042a4fe0b44c9a6599ea48095eb8c5ad5d83bb122b82: ai,
		GoooFieldb790b38a8e8f8b5cb1d09c033c8e0c237d22466bf56b8f03739ce08101716398: beforeDefinition, GoooFielddf16b2b85e5bc6d5b97843dfe7e0bd29487b2c986540e84559356b2b3cd4fb7b: afterDefinition,
		GoooField62d7b7c4076a16a72acb16e33b97e15ec9fcfebc04d9b685afcb6d8c53ec4e61: beforeProfile, GoooField76dc4066ffc1c2702d1b089af2d1a17aa00994e9ac5ab9ae4cc959cda5e3ab49: afterProfile,
		GoooFielddbe0e973b2904a82c77bdb5cad1b540188e7d5231104934fce61a1e950be375e: beforeAnnotation, GoooField82b90b3b3e5e3560fcde62a9413cf744249081a63e1632f6c93391977ee2bab4: afterAnnotation,
		GoooFielda0b75a9c8930e5edb9cbe78219558b56f4c8854fd49b16f51a45b15fa428ce45: presence, GoooField40949c72acd2e5c5a5e34f542c9a61abdf1907455a14ccfcc9a0bf6ea0e9b3c5: definitionRelation,
		GoooField2e0007a926bd286bb13a26f2265750e8d226fe28bd8906bd3b6246ec7022ac5c: annotationRelation, GoooField944a3407c3251dc729bcca65fff1e05d8142c0335df1104e99c87dd186d7c854: profileRelation,
		GoooField9026e0ed0ae780de9f33eaab5a16c5c5c6c26c726a0d7c1268fe3241ca720db7: "NOT_ASSESSED", GoooField92db0f9a71f85b70f9f898dca9e98da7a691092873ba6c69da8bd3dcd120217e: "NOT_ASSESSED",
		GoooField3c159e4a8770047450814557ead5470d359916f4180021675076a085b1f13d65: false, GoooField259e273dad1ee97dce0bb044f85cc9f98f3008a044541c505b597e0890975e87: false, GoooField17ab191a48ab90750695d456c8974a742bbd4a108738925e765e00e1438a6126: false,
		GoooFieldbb5ae9586c9064bef93d42d789b9c5159ee44bb382714f913cb1f57a639dad6d: "UNKNOWN",
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

//gooo:generated:end id="gooo://tool-origin/definition-snapshot/v1/compare" kind="activity"
