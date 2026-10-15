package main

// These field IDs and their order come from the reviewed Gooo declarations,
// never from a returned receipt or from the Go oracle's map iteration order.
func literalCutResultContract(s suite) resultContract {
	switch s.Package + ":" + s.Activity {
	case literalCutPackage + ":Cut":
		const e = cutResultEntity
		return resultContract{Entity: e, Fields: []recordFieldContract{
			{"before", e + "/before"}, {"after", e + "/after"}, {"found", e + "/found"},
		}}
	case literalCutPackage + ":CutPrefix", literalCutPackage + ":CutSuffix":
		const e = cutAffixResultEntity
		return resultContract{Entity: e, Fields: []recordFieldContract{
			{"text", e + "/text"}, {"found", e + "/found"},
		}}
	case literalCutConsumerPackage + ":Report":
		const e = literalCutResultEntity
		return resultContract{Entity: e, Fields: []recordFieldContract{
			{"text", e + "/text"}, {"needle", e + "/needle"},
			{"cut_before", e + "/cut-before"}, {"cut_after", e + "/cut-after"}, {"cut_found", e + "/cut-found"},
			{"prefix_after", e + "/prefix-after"}, {"prefix_found", e + "/prefix-found"},
			{"suffix_before", e + "/suffix-before"}, {"suffix_found", e + "/suffix-found"},
			{"text_byte_length", e + "/text-byte-length"}, {"needle_byte_length", e + "/needle-byte-length"},
		}}
	}
	return resultContract{}
}
