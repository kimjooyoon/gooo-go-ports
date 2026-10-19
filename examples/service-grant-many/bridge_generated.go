// Code generated from compiler-owned typed-plan identities; DO NOT EDIT.
package servicegrantmany

// EvaluateColumns returns the Gooo decision on ordered caller-supplied facts.
func EvaluateColumns(input RequestColumns) Decision {
	native := GoooRecord7201cd023e75bfb95e776ef109d81255061cbcb83c9d60f0777ebf90f26b68bf{
		GoooField2da56a6c64b7fa99c74ec0edbff173ac72722c028f47396343ff7d8195f30f03: input.Before,
		GoooField39fd9cae00230e526199c131c2f320dd9209cc92ea789156b4b15ceea0f95251: input.After,
		GoooFieldf24f614c675e868faab41da4399c35e962b3476af4b13dd176493ee11d951fc0: input.RequiredTypes,
		GoooField12ec6d0b7e86f8f336fb3de209ceff8ccd17ee8ccd7cf8a017d172704139afd9: input.RequiredNames,
	}
	actual := GoooComposedActivity0(native)
	return Decision{
		Status:                    actual.GoooFielde17671758b8989367f712ab10dc2d43b771a3d8bbdcc929b4cdf3da276636d3b,
		Reason:                    actual.GoooFieldd132de59112b956418d37cbc65d8c214e6f60595e3ca6d7b5bb6e4263d9e3a27,
		InvalidField:              actual.GoooField9c305cfca0a209a0b885c1f7a5a47b548d60a2bd2439cc3a676dc569246584e1,
		InvalidIndex:              actual.GoooFielde0a069823b3754d4abd610b77c2703fb4f9f4b6c4c598de9f9ed3de001dd519e,
		FirstCounterexample:       actual.GoooField04678d8d144f2fd436965c853ce87d75ca776bfb204397c13a89d3e399c753d8,
		RequestCount:              actual.GoooFieldb448ee2b919a060532c8c5201943f00d449084a7ea4fa58767a49c958399c2eb,
		CheckedCount:              actual.GoooFieldf2456700384378821972f0ab5088b371516a8b0486c65c9e4401d7be13968d04,
		AllPreserved:              actual.GoooField8aed4c7c938458d56e85c0d57db3ce04e55bd1cb7bd5d26c7a04ab6b89fbc45f,
		SupportedSubset:           actual.GoooField79032c17601e93f3f5476b925e3dc9e336a82cb14fd38d13640f060e0fcec169,
		SubsetRuleID:              actual.GoooField2df87d407856d2d2c7ebbdf38cbaef5288e18499aca504fe83ef79a8a9df5d09,
		CounterexampleBeforeMatch: actual.GoooFieldd98984f36f8370dd819801c630db15dad3b38a27cc5c14d7121008f7526abf60,
		CounterexampleAfterMatch:  actual.GoooField580bd5f185f5febade822e8d272088d589f6dad6a0dcfedfd978483c97ca252a,
	}
}
