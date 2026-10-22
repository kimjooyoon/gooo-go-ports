// Code generated from compiler-owned record/field identities; DO NOT EDIT.
package selector

func Evaluate(input Input) Decision {
	native := GoooRecord42b864fa50cb6ccd0a85ce0b0357134798e893f0448cd3a7d70d00892d3bf0b0{
		GoooField3e36a565fb10eda4b413ec4503dc96b47cca3efdd15609bdcf6b20b64ba414f4: input.Phase,
		GoooFieldca17fb3c3cdf4c28ad7f9d3f50ee22bfe932f1ff7f8cf0443411001a8874b0f7: input.ContextID,
		GoooField05be2b7d3bbbf03fd071448f485ed992800d02331a6d849d3b916080b7fb3010: input.CandidateIDs,
		GoooField5c4dab7bebe2a63bde47995f43d06b46b085ef9863325780448f69fc1ab60671: input.AssessmentIDs,
		GoooFieldfa3fe6437a3accaf6087c122911deee785361c04c0b12f827fa03881dececcd2: input.ContextIDs,
		GoooFielda91703fa4739c7aab0f4b195f4078ddd2a04e89cca320d124be2ac6db2d1e644: input.Statuses,
		GoooFieldfa9ce851f769a68f131c052345591290f4e9341e3691d956196ef6420f79799e: input.Priorities,
		GoooField2561eb0e807a6d03b849c06baa0b3ca772c9bcd60a301be7e4394784408b0f00: input.BindingsVerified,
	}
	actual := GoooComposedActivity0(native)
	return Decision{
		ContextID:            actual.GoooFieldd222a9375fa35992acd9018e436b4381d432910dd91d670bcb8bc50e8d0dd497,
		State:                actual.GoooField97fdae3df3f9c4985d8e2a61f75113b9e95b9d8b36302eeb9b89cf05cf37b3f9,
		Reason:               actual.GoooField9047500c625a50d55bf4d6bf58ea60fb47fa0995f15cf8f090cd530391995a75,
		SelectedCandidateID:  actual.GoooFieldbfeccd1f811c9942230a2c03914267fcb6c19520dc11928c1f723ede73fe6b66,
		SelectedAssessmentID: actual.GoooField0e984ad6c2532a7829736502fe16b8a7743e0dbacef8d260d14588ea97362942,
		SelectedIndex:        actual.GoooFieldf04dad0bb349e4bedcba1364abbeb7b2b3b99159ca6ac9b0dcccd9b6e3f738a7,
		SelectedPriority:     actual.GoooFieldf3daab021a38c3134c4e1016cf17b8c498ed1a75acf9857d43084adc8b37c4d2,
		CandidateCount:       actual.GoooFieldcf3497e87cc0b98cd10f2a98e08e05fd36b98d5f5f87d044704e80fb5a63a064,
		EligibleCount:        actual.GoooFieldee0d738147efaff9ab159657b556523f31dc457eacb0631d262f73586a32d6eb,
		InvalidIndex:         actual.GoooField73584955536b2b8c1accda24e675c6ba325174e05c5984ca4b1fee386de4ac50,
		UnknownIndex:         actual.GoooField3513a6d70f27f8f71c0fa35d83b08eafa1b3ceffe03831ccd6fbaa53872c4805,
		ExecutionAuthorized:  actual.GoooFielde7f3a9e13a5d0d063988817e06f09368954e8b1d17b26a58e2c2a26b7fc665ff,
	}
}
