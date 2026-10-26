// Code generated from compiler-owned record/field identities; DO NOT EDIT.
package refreshkernel

func Evaluate(snapshot SnapshotFacts, evidence RefreshEvidence, state RefreshState) RefreshDecision {
	nativeSnapshot := GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25{
		GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270: snapshot.PageRequestCursorKinds,
		GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393: snapshot.PageRequestCursorValues,
		GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824: snapshot.PageNextCursorKinds,
		GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55: snapshot.PageNextCursorValues,
		GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048: snapshot.OriginalNames,
		GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42: snapshot.DefinitionIDs,
		GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472: snapshot.DefinitionProfiles,
		GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10: snapshot.AnnotationIDs,
		GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049: snapshot.RowPageIndexes,
	}
	nativeEvidence := GoooRecordfe399ddb2fb6a2479413e08a7769d15865f1d8c6ba3c6bca970dd9cf95ea2179{
		GoooField1258229c496acf55993d81d095bac9007dac482fce417df59df5114e3446bf36: evidence.AttemptID,
		GoooField95a4b108299fa581c6eda3e145abc6b5c99e345ee414eff8ce35fd0dc07eefb0: evidence.CandidateID,
		GoooField6601a2dbb88c99a442963b209be0be93c06c924fe917993d30efc1d7f9204c86: evidence.CollectorState,
		GoooField2faca12ae84814dbc7b0d4a860d383224ebb55e05108acbc78e3846cfe4b554a: evidence.BindingState,
		GoooField75427d3f80eda63dff9f2598dc9e6cb23292a5830db20e24b66c1e167458da7b: evidence.ValidatorProfile,
		GoooFielde606eae7de091e2541083e4c687abb7356680399fb2d923ecf44e203542e6e64: evidence.RowDefinitionIDs,
		GoooField11174a0b4e54b048d0175f0224ce613ce33de6916bde6303e8e07263313554b4: evidence.RowValidationStates,
	}
	nativeState := GoooRecord75875bd2679c233875f4e28a2bfddb4b63f7972b06acd425b097eaf651a6c602{
		GoooField1032ac3f4c0b85e3248c106ac39303e520ac2094cc2fe3305e421fb2a2f8ef41: state.ExpectedRegistryVersion,
		GoooFieldb8bad7d576d3f3ef1bbfa52cb78155d8cd1a8bdb3a05ba832c9b3a8054b2239b: state.CurrentRegistryVersion,
		GoooField177dbe5908a51ba3c34e3635df1e93bdab2526acddc276745aadac009d8eef95: state.ExpectedScopeEpoch,
		GoooField4d5f142fa7504fb39112a9b2e005b16b346ea42f4e99ec25956a17670b23fff2: state.CurrentScopeEpoch,
		GoooField0e04a6599cae4dc65474220b7f113889b82fec31a90a2e926c42668cc586dd08: state.ExpectedInvalidationEpoch,
		GoooFieldc4c614252daec3c11b2fc0f5cbfd15664b26d5d2a2084c9903de0f3ebc0c2646: state.CurrentInvalidationEpoch,
		GoooField43d7d754f8c2b40daef769ed8f8e3a600f11b8427f921b84c9299b1c3b02978f: state.ExpectedLatestAttempt,
		GoooFieldfcafdac1622119746746a4d9c00163b335723fdfeda92d86b9482aca9dc1a84c: state.CurrentLatestAttempt,
	}
	actual := GoooComposedActivity0(nativeSnapshot, nativeEvidence, nativeState)
	return RefreshDecision{
		State:                     actual.GoooFieldb45035c5f3b4e0a9686f86459937ece2736fc071a51dc0065bbcdb2bcefe398c,
		Reason:                    actual.GoooFieldc92b7078f1ec6cc9b58599b99701de59f0fa388811f7b494c9c5522029b49593,
		CandidateID:               actual.GoooField7b9d4dcd0b27b82c86fc48e93a1f0900d875d77c18e5b0d65aa4077c593e9e8c,
		AttemptID:                 actual.GoooField708627bb92d1ef761947bc20377aac92c35a6536ae582b2659e09589a24d5697,
		ExpectedRegistryVersion:   actual.GoooField3aea6aba8cd184045f198f75b54d0d15314e1fb1a53231da23d74b0f43ba7b09,
		ExpectedScopeEpoch:        actual.GoooField5074c42524f53e2dac53ad0c61b9543080126815e9bc542df03876270d0a7c63,
		ExpectedInvalidationEpoch: actual.GoooField802f48cd68ca3485ce95339087832c18805c59c64682d9d00b6fd40bafa17549,
		ExpectedLatestAttempt:     actual.GoooField9110271f4a91166349f4c9cf3555530dd0f0200ac1a4b7f4f21f709c2cb2ef39,
		CandidatePageCount:        actual.GoooField2f61fbc77037c29c92303006ae11d691513d5a7c49c01ad43dbb5eb6367cf085,
		CandidateRowCount:         actual.GoooFieldfe1990f28e7244cc96f211b271fad67c56cb55be3cc337625ef8365f0a466fc1,
		Coverage:                  actual.GoooField2c100031e164b512ac9485da087d04099df96884fef6bf00b37f2d3c56d991f8,
		CoverageReason:            actual.GoooField395b3b208cb79b60ecb6bd56ad0f8d3a068846577440bd634606bcff665f2d30,
		ProblemPageIndex:          actual.GoooField3175d659156ca82699ecbec1c633f0f1c6022145a4fb92ea3c303a289564a25e,
		ProblemRowIndex:           actual.GoooField2341c752c64f8cfada74879c3f25f86fd6be66864f141d65611c3dc0bbea4b52,
		DecisionScope:             actual.GoooField8c6614baf1d7033c59d1f68f35960cb761af8beebc042a44ff8fb669778dab80,
		ExecutionAuthorized:       actual.GoooField22313d6369eb2ae8355e2d98cd0bc85acfe29b2bf2a98ea8ceff1e3b2d5f5b38,
		LiveEffectPerformed:       actual.GoooFieldad322230141f558ea6be8e58b5e99538b2241e852d55ec63de63bf4780dab835,
		RemoteAtomicity:           actual.GoooFielddfdee191dd26bdd2a741c1fadceeaf805530e827c12dbedc67432257379ce25b,
	}
}
