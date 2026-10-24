// Code generated from compiler-owned record/field identities; DO NOT EDIT.
package kernel

func Evaluate(input Input) Decision {
	native := GoooRecord8cfc2951857a013943ea032087d799c5a9bcd1055b79b8acac49a9d718e2f82a{
		GoooField745624ba329caed765a6591a971cebe37c88dd1650d854f4b043177defc6f44f: input.ContextID,
		GoooFielda4c9dc33a4f1a4e2640f75b5249d0e467a38d272580c2c9dbcae5326bb590364: input.TargetAlias,
		GoooField160b2f010a78a17180bbbf497218e1108cb818d720b5017c6d4e3f48598fc955: input.BeforeServers,
		GoooField428983b2ba3f869abdae0e3edee6d55b476b3202ce7bf877814496181332a0d6: input.BeforeNames,
		GoooField4d466e645d22421206d6e1e1138d3d142a0bd7d2d314f6110a0c6ce6807bf6b1: input.BeforeAliases,
		GoooField69649810acac2a0e7327b35cc0935b6fc56c95b826735a6ce301a0fd510dd062: input.AfterServers,
		GoooField4ffb11f0f796f2b3b81a3f979723322625c0df704433b993fa55370336bd135e: input.AfterNames,
		GoooField472fa813b06642e419154c84c94484b3ab38f34dad9d53e1d1154ae090b50e98: input.AfterAliases,
	}
	actual := GoooComposedActivity0(native)
	return Decision{
		ContextID:           actual.GoooField7f24f8dc1d6f723e8d49830371f3dc7684d77020312f44feb4721b166a743a7d,
		State:               actual.GoooFieldfb8728181cfded348b301ec43994868412a36d6024eb0feea4857b5e30d8236b,
		Reason:              actual.GoooField2f4939454fb06b7f88e1ecc70be0e7ad808274d896dffc1b3fbcb71f75b2ccc5,
		BeforeIndex:         actual.GoooField91bcf6a6e5f244af7b578b292812f2b398122ef299c631fe28231e073aa6c87d,
		AfterIndex:          actual.GoooField655bd17dbb2ec6d2ff98a7187dbdbb593d37752eb5400e5c17e3bf96f62fe4cd,
		ConflictCatalog:     actual.GoooField2b8efaee9b09888bd39b5c8c164a4d3157ff3b174ae63ec97ae95d2d56b97fd8,
		ConflictFirstIndex:  actual.GoooFieldd2d2867afb14dd78731b64142c6345b137cca6dbb7ebad9b367b567e04fd99ef,
		ConflictSecondIndex: actual.GoooFieldbfacd31df3560849d89122a41fc040f3d329e0614805bfcd1eccbe0894c74b28,
		ExecutionAuthorized: actual.GoooFielda660e93ccc2a5d204d69a64193e6825a84e5740ac34e409f2b6f0a9e64ce41f4,
	}
}
