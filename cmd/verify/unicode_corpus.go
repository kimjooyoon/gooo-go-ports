package main

import (
	"fmt"
	"math"
	"reflect"
	"unicode/utf16"
	"unicode/utf8"
)

type inspectorCorpus struct {
	Suites         []suite
	ReferenceCases int
	RejectionCases int
}

// Record expectations need more bytes than the legacy scalar outputs. Keep
// these batches below the pinned runtime's 32 KiB case-document limit.
const unicodeInspectorBatchSize = 64

// Reuse the legacy input corpus without changing its inputs or expectations.
// The new app's expectations are independent calls to the pinned Go libraries.
func unicodeInspectorCorpus(base []suite) (inspectorCorpus, error) {
	result := inspectorCorpus{}
	for _, operation := range []struct {
		pkg, source, activity string
		arity                 int
		extra                 [][]any
		oracle                func([]rune) any
		rejected              any
	}{
		{"go/unicode/utf8", "ValidRune", "InspectScalar", 1,
			[][]any{{int64(65)}, {int64(128512)}}, inspectScalarOracle, rejectedScalar()},
		{"go/unicode/utf16", "DecodeRune", "InspectPair", 2,
			[][]any{{int64(65533), int64(65533)}, {int64(55357), int64(56832)},
				{int64(0x1D800), int64(0xDC00)}, {int64(0xD800), int64(0x1DC00)},
				{int64(math.MinInt32), int64(0xDC00)}, {int64(0xD800), int64(math.MinInt32)}},
			inspectPairOracle, rejectedPair()},
	} {
		var inputs [][]any
		matches := 0
		for _, s := range base {
			if s.Package == operation.pkg && s.Activity == operation.source {
				matches++
				for _, c := range s.Cases {
					inputs = append(inputs, append([]any(nil), c.Inputs...))
				}
			}
		}
		if matches != 1 || len(inputs) == 0 {
			return inspectorCorpus{}, fmt.Errorf("expected one nonempty source corpus for %s:%s", operation.pkg, operation.source)
		}
		for _, extra := range operation.extra {
			found := false
			for _, input := range inputs {
				found = found || reflect.DeepEqual(input, extra)
			}
			if !found {
				inputs = append(inputs, extra)
			}
		}
		s, err := runeConsumerSuite(operation.activity, operation.arity, inputs, operation.oracle)
		if err != nil {
			return inspectorCorpus{}, err
		}
		result.ReferenceCases += len(s.Cases)
		// These test the Gooo consumer's explicit rejection contract, not Go
		// equivalence. Keep the original int64 inputs; never narrow or wrap them.
		for _, n := range []int64{math.MinInt64, math.MinInt32 - 1, math.MaxInt32 + 1, math.MaxInt64} {
			for position := 0; position < operation.arity; position++ {
				input := []any{int64(0xD800), int64(0xDC00)}[:operation.arity]
				input[position] = n
				s.Cases = append(s.Cases, testCase{Inputs: input, Expected: operation.rejected})
				result.RejectionCases++
			}
		}
		s.Domain = fmt.Sprintf("importing consumer; %d int32 Go-reference cases plus %d explicit out-of-int32 rejection cases (no narrowing)", len(inputs), len(s.Cases)-len(inputs))
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func runeConsumerSuite(activity string, arity int, inputs [][]any, oracle func([]rune) any) (suite, error) {
	if (arity != 1 && arity != 2) || len(inputs) == 0 {
		return suite{}, fmt.Errorf("invalid rune corpus shape")
	}
	// Validate the entire corpus before converting anything or calling Go.
	for _, input := range inputs {
		if len(input) != arity {
			return suite{}, fmt.Errorf("%s: expected %d rune inputs", activity, arity)
		}
		for _, value := range input {
			n, ok := value.(int64)
			if !ok || n < math.MinInt32 || n > math.MaxInt32 {
				return suite{}, fmt.Errorf("%s: input %v is not an int64 in the int32 rune domain", activity, value)
			}
		}
	}
	s := suite{Package: "app/unicode-inspector", Activity: activity}
	for _, input := range inputs {
		runes := make([]rune, arity)
		for i, value := range input {
			runes[i] = rune(value.(int64))
		}
		s.Cases = append(s.Cases, testCase{Inputs: append([]any(nil), input...), Expected: oracle(runes)})
	}
	return s, nil
}

func inspectScalarOracle(input []rune) any {
	r := input[0]
	h, l := utf16.EncodeRune(r)
	return map[string]any{
		"in_rune_domain": true, "valid_scalar": utf8.ValidRune(r), "is_surrogate": utf16.IsSurrogate(r),
		"utf8_bytes": utf8.RuneLen(r), "utf16_units": utf16.RuneLen(r),
		"needs_pair": utf16.RuneLen(r) == 2, "high": h, "low": l,
	}
}

func inspectPairOracle(input []rune) any {
	r := utf16.DecodeRune(input[0], input[1])
	// A valid surrogate pair always decodes above the BMP, never to RuneError.
	// This deliberately does not copy the Gooo high/low range expression.
	return map[string]any{
		"in_rune_domain": true, "valid_pair": r != utf8.RuneError, "decoded_scalar": r,
		"decoded_utf8_bytes": utf8.RuneLen(r), "decoded_utf16_units": utf16.RuneLen(r),
	}
}

func rejectedScalar() any {
	return map[string]any{"in_rune_domain": false, "valid_scalar": false, "is_surrogate": false,
		"utf8_bytes": -1, "utf16_units": -1, "needs_pair": false, "high": -1, "low": -1}
}

func rejectedPair() any {
	return map[string]any{"in_rune_domain": false, "valid_pair": false, "decoded_scalar": -1,
		"decoded_utf8_bytes": -1, "decoded_utf16_units": -1}
}
