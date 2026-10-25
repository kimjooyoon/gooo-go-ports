// Code generated from compiler-owned record/field identities; DO NOT EDIT.
package snapshotkernel

func Evaluate(before SnapshotFacts, after SnapshotFacts, selection Selection) Decision {
	nativeBefore := GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25{
		GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270: before.PageRequestCursorKinds,
		GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393: before.PageRequestCursorValues,
		GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824: before.PageNextCursorKinds,
		GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55: before.PageNextCursorValues,
		GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048: before.OriginalNames,
		GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42: before.DefinitionIDs,
		GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472: before.DefinitionProfiles,
		GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10: before.AnnotationIDs,
		GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049: before.RowPageIndexes,
	}
	nativeAfter := GoooRecordf6d51016fa7de3331028b610422621c7c7977deb80765c1821c2dc864cab1c25{
		GoooFieldbe8063979f6935ea47943184fcdb87d2093357afadfcf776b55fb1b2578bb270: after.PageRequestCursorKinds,
		GoooField636f698fa6ff16af46e10ed9594cb506290eb8de8b69d51cd1676a05f4e9c393: after.PageRequestCursorValues,
		GoooFielddfb7ad0ec41208eb132b082e2495adc488e3245d349d4448ae30de6fb68f5824: after.PageNextCursorKinds,
		GoooField647de629afc997732ab83faeec1e9eb35462b99dce8250a17a19373df518ef55: after.PageNextCursorValues,
		GoooFieldc6f860f6c988feb567dcc06b5daa67c87b509fbd7b5a7419569bbed9d9034048: after.OriginalNames,
		GoooFieldfa00f60a62f58f48ae3789debde1f927d37e2b98d96cbcb17fc444f08d523d42: after.DefinitionIDs,
		GoooFielde8855895f76108341377793b070fd3898420a8238d015c24c7fb02429a07c472: after.DefinitionProfiles,
		GoooFieldd2be5b9b91a3dc20849199863c12437606d0e983ef1de21b1c71032b072ccc10: after.AnnotationIDs,
		GoooField5fb3ef968eccfbb7dba4dd167ab87d2cc31a7386804cd808e8e0fb58cc056049: after.RowPageIndexes,
	}
	nativeSelection := GoooRecord4657ec7bd1268ab0c3c0e0c605b3ad32555eed4612703867937f41a7e32492ce{
		GoooFielda771c03f295b0253851902413e0a9baa218d5281cd064e15088e769fea770732: selection.ContextID,
		GoooFieldb9a2febfbd3aa741a86d2a9099d36aa336264d2b348815977c4d50aa3ac80cb3: selection.TargetName,
	}
	actual := GoooComposedActivity0(nativeBefore, nativeAfter, nativeSelection)
	return Decision{
		ContextID:                 actual.GoooFieldbcc80922b7849199853015833e146f8dc0ce58017e282da4c30d3a50a5c5ca0b,
		TargetName:                actual.GoooField18273142b6d46141e51852fde687f320069b853369752f530aef18f1a8fd7be2,
		State:                     actual.GoooField8b81d4a2dc6c2b3b8666b512ccd9f48956d6f9adebd2bd25f45f677b6631b579,
		Reason:                    actual.GoooField00ccf611ba4fe589dd50637cbc50e02f15aff0f1a80b0a49e34337425ca368ff,
		NextAction:                actual.GoooFieldcb323392c9bbdc0405511ad798b37a4fa69c343da09ffeeaca078020299b9ee5,
		ComparisonScope:           actual.GoooField63a6c9a50699cb5b2f8b90c32a7c30bfc634007831dcba9396407c9c7041086e,
		BeforePageCoverage:        actual.GoooFieldacaa986a1441b7b3d9eefb633756a841ceda72ad445c439eef7c663467e9ff73,
		AfterPageCoverage:         actual.GoooField18971056017d1594a1f6bf8e3ddbd1ab92294ffd7bf1eb077d7d9088aafdfa95,
		BeforeCoverageReason:      actual.GoooField4830c19362bc88157643f9b0c4daf8530e8d8455ad5e57775cea109a4b28344a,
		AfterCoverageReason:       actual.GoooField3e947e1e651c834ec6356353878d07a79c1eb3b63a08df4127f0da338c6575fb,
		BeforeProblemPageIndex:    actual.GoooField89c0f766d0884fd8c9077074fff7c8146e9a3d8421bc15445897903e4abdbd14,
		AfterProblemPageIndex:     actual.GoooField6a183f2475886028f735482e251d71a655b916980638bc84f12758e99165e1fe,
		BeforeProblemRowIndex:     actual.GoooField7b82058191bf48fd3b1ef0db995b066a24615b53545f9ca74edd4ba8c3b1ac03,
		AfterProblemRowIndex:      actual.GoooField0a4e010c728fb215a8b94ebbc7444769ed48e5483d343687bf9bee07a93b961e,
		BeforeIndex:               actual.GoooFielde0272827e9a9df708ee952803f9c48d8b866be39839893f7d3c5aae6b994c839,
		AfterIndex:                actual.GoooFielda3200f5f87e40bdd5f94042a4fe0b44c9a6599ea48095eb8c5ad5d83bb122b82,
		BeforeDefinitionID:        actual.GoooFieldb790b38a8e8f8b5cb1d09c033c8e0c237d22466bf56b8f03739ce08101716398,
		AfterDefinitionID:         actual.GoooFielddf16b2b85e5bc6d5b97843dfe7e0bd29487b2c986540e84559356b2b3cd4fb7b,
		BeforeDefinitionProfile:   actual.GoooField62d7b7c4076a16a72acb16e33b97e15ec9fcfebc04d9b685afcb6d8c53ec4e61,
		AfterDefinitionProfile:    actual.GoooField76dc4066ffc1c2702d1b089af2d1a17aa00994e9ac5ab9ae4cc959cda5e3ab49,
		BeforeAnnotationID:        actual.GoooFielddbe0e973b2904a82c77bdb5cad1b540188e7d5231104934fce61a1e950be375e,
		AfterAnnotationID:         actual.GoooField82b90b3b3e5e3560fcde62a9413cf744249081a63e1632f6c93391977ee2bab4,
		ObservedPresenceRelation:  actual.GoooFielda0b75a9c8930e5edb9cbe78219558b56f4c8854fd49b16f51a45b15fa428ce45,
		DefinitionRelation:        actual.GoooField40949c72acd2e5c5a5e34f542c9a61abdf1907455a14ccfcc9a0bf6ea0e9b3c5,
		AnnotationRelation:        actual.GoooField2e0007a926bd286bb13a26f2265750e8d226fe28bd8906bd3b6246ec7022ac5c,
		ProfileRelation:           actual.GoooField944a3407c3251dc729bcca65fff1e05d8142c0335df1104e99c87dd186d7c854,
		OriginRelation:            actual.GoooField9026e0ed0ae780de9f33eaab5a16c5c5c6c26c726a0d7c1268fe3241ca720db7,
		AuthorizationRelation:     actual.GoooField92db0f9a71f85b70f9f898dca9e98da7a691092873ba6c69da8bd3dcd120217e,
		BehaviorEquivalenceProven: actual.GoooField3c159e4a8770047450814557ead5470d359916f4180021675076a085b1f13d65,
		ExecutionAuthorized:       actual.GoooField259e273dad1ee97dce0bb044f85cc9f98f3008a044541c505b597e0890975e87,
		LiveEffectPerformed:       actual.GoooField17ab191a48ab90750695d456c8974a742bbd4a108738925e765e00e1438a6126,
		RemoteAtomicity:           actual.GoooFieldbb5ae9586c9064bef93d42d789b9c5159ee44bb382714f913cb1f57a639dad6d,
	}
}
