package kernel

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

//gooo:generated:start id="gooo://tool-origin/v1/compare" kind="activity"
func GoooComposedActivity0(input GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a) GoooRecorde0ad1c493684691a6d21d05f83c27094f343f3bee137cb4d596f1809dd0be330 {

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

//gooo:generated:end id="gooo://tool-origin/v1/compare" kind="activity"
