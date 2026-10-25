// Code generated from compiler-owned record/field identities; DO NOT EDIT.
package runtimepackagesurface

func Evaluate(input Observation) Assessment {
	native := GoooRecord0d8c19ab5f34a1a9ba068ee5a9c2824812be0131a1901904c711675bb7a57c6a{
		GoooField00b8d5090334be4e64baac634f42cafc0982183f09eb4423590d1df35161f236: input.OriginRef,
		GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314: input.ImportPaths,
	}
	actual := GoooComposedActivity0(native)
	return Assessment{
		Status:               actual.GoooFielde0569a41080f6be27c724234db35b2e0ddaa768ecffdd83f6a9600dae513054a,
		ObservationOrigin:    actual.GoooFieldce3661ab6a0d747b227aa8f2d94e352d7a8c877b8560bbbf0978bebca863961d,
		DeclaredPaths:        actual.GoooField66ecd7de70125daddd236872bea8837c5b2ad06921b229e3f870ad06364b15f2,
		ObservedPaths:        actual.GoooField403e560f053ca4b4c83f5b8fa5b2dc06903cdf5a948dbf673c55db79c3c0a966,
		MissingCount:         actual.GoooField6426480fdd193f28b440299b1fd9a0a0305719d7ccf70c5cbb0b0c5bf50edbb2,
		UndeclaredCount:      actual.GoooField7f4697caa2416ae41bbaebbc0880025422a474f132aa4676e3c164502cc3f09b,
		DuplicateObservation: actual.GoooField35f06f6ed7bda307c2758805af8ea14aaf9d719ccc55e4c953edeaeb68be275d,
	}
}
