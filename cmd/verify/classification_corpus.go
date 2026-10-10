package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"unicode"
)

const classificationPortBatchSize = 128
const classificationConsumerBatchSize = 64

type classificationObservations struct {
	Suites                                         []suite
	PortReferenceCases, PortRejectionCases         int
	ConsumerReferenceCases, ConsumerRejectionCases int
}

func classificationCorpus() (classificationObservations, error) {
	if unicode.Version != "17.0.0" {
		return classificationObservations{}, fmt.Errorf("classification corpus requires Unicode 17.0.0, got %s", unicode.Version)
	}
	result := classificationObservations{}
	letters, digits := classificationInputs(unicode.Letter), classificationInputs(unicode.Digit)
	for _, op := range []struct {
		pkg, activity string
		inputs        [][]any
		oracle        func(rune) any
		rejected      any
	}{
		{"go/unicode", "IsLetter", letters, func(r rune) any { return unicode.IsLetter(r) }, false},
		{"go/unicode", "IsDigit", digits, func(r rune) any { return unicode.IsDigit(r) }, false},
		{"app/identifier-rune", "Classify", classificationUnion(letters, digits), identifierRuneOracle, rejectedIdentifierRune()},
	} {
		s, err := classificationSuite(op.pkg, op.activity, op.inputs, op.oracle)
		if err != nil {
			return classificationObservations{}, err
		}
		for _, n := range classificationRejectedInputs() {
			s.Cases = append(s.Cases, testCase{Inputs: []any{n}, Expected: op.rejected})
		}
		s.Domain = fmt.Sprintf("signed int32 rune; %d finite Go-reference observations include all Latin-1 inputs, relevant Unicode 17.0.0 RangeTable boundaries, stride members and holes, named Letter/Nd distinctions, invalid Unicode, narrowing aliases and fixed samples; %d original int64 outside-int32 inputs test a separate false rejection extension without narrowing; no whole-identifier validation", len(op.inputs), len(classificationRejectedInputs()))
		if op.pkg == "go/unicode" {
			result.PortReferenceCases += len(op.inputs)
			result.PortRejectionCases += len(classificationRejectedInputs())
		} else {
			result.ConsumerReferenceCases += len(op.inputs)
			result.ConsumerRejectionCases += len(classificationRejectedInputs())
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func classificationSuite(pkg, activity string, inputs [][]any, oracle func(rune) any) (suite, error) {
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty classification corpus")
	}
	seen := map[int64]bool{}
	// Validate the complete original roster before narrowing or calling Go.
	for _, input := range inputs {
		if len(input) != 1 {
			return suite{}, fmt.Errorf("%s: expected one rune", activity)
		}
		n, ok := input[0].(int64)
		if !ok || n < math.MinInt32 || n > math.MaxInt32 || seen[n] {
			return suite{}, fmt.Errorf("%s: require distinct exact int64 inputs in the int32 domain", activity)
		}
		seen[n] = true
	}
	s := suite{Package: pkg, Activity: activity}
	for _, input := range inputs {
		s.Cases = append(s.Cases, testCase{Inputs: append([]any(nil), input...), Expected: oracle(rune(input[0].(int64)))})
	}
	return s, nil
}

// Tables select coverage targets only. Expected results always come from the
// original unicode.IsLetter/IsDigit calls, never this range traversal.
func classificationInputs(table *unicode.RangeTable) [][]any {
	var inputs [][]any
	seen := map[int64]bool{}
	add := func(n int64) {
		if !seen[n] {
			seen[n] = true
			inputs = append(inputs, []any{n})
		}
	}
	for n := int64(0); n <= 255; n++ {
		add(n)
	}
	addRange := func(lo, hi, stride int64) {
		for _, n := range []int64{lo - 1, lo, lo + 1, hi - 1, hi, hi + 1} {
			add(n)
		}
		if lo+stride <= hi {
			add(lo + stride)
		}
		middle := lo + ((hi-lo)/stride/2)*stride
		add(middle)
		if stride > 1 && middle+1 < hi {
			add(middle + 1)
		}
	}
	for _, r := range table.R16 {
		addRange(int64(r.Lo), int64(r.Hi), int64(r.Stride))
	}
	for _, r := range table.R32 {
		addRange(int64(r.Lo), int64(r.Hi), int64(r.Stride))
	}
	for _, n := range classificationBoundaryInputs() {
		add(n)
	}
	// The same samples in both properties keep their union compact and stable.
	rng := rand.New(rand.NewPCG(277, 307))
	for i := 0; i < 64; i++ {
		add(int64(int32(rng.Uint32())))
	}
	for i := 0; i < 64; i++ {
		add(int64(rng.Uint32N(0x110000)))
	}
	return inputs
}

func classificationBoundaryInputs() []int64 {
	return []int64{
		math.MinInt32, math.MinInt32 + 1, -65488, -65471, -65441, -1, 0,
		'_', 'A', 'z', '0', '9', 0xaa, 0xb2, 0xbd, 0x301, 0x660, 0x905, 0x967, 0xe50,
		0x200c, 0x200d, 0x216b, 0x2460, 0x4e2d, 0xac00, 0xff10,
		0x10400, 0x104a0, 0x11f50, 0x1d7d8, 0x1e4f0, 0x1f600, 0x20000,
		0xd7ff, 0xd800, 0xdbff, 0xdc00, 0xdfff, 0xe000, 0xfffd, 0xffff, 0x10000,
		0x10ffff, 0x110000, 0x110030, 0x110041, 0x11005f, math.MaxInt32 - 1, math.MaxInt32,
	}
}

func classificationUnion(groups ...[][]any) [][]any {
	var result [][]any
	seen := map[int64]bool{}
	for _, inputs := range groups {
		for _, input := range inputs {
			n := input[0].(int64)
			if !seen[n] {
				seen[n] = true
				result = append(result, append([]any(nil), input...))
			}
		}
	}
	return result
}

func classificationRejectedInputs() []int64 {
	return []int64{
		math.MinInt64, -9007199254740993, math.MinInt32 - 1,
		-4294967248, -4294967231, -4294967201,
		math.MaxInt32 + 1, 4294967344, 4294967361, 4294967391,
		9007199254740993, math.MaxInt64,
	}
}

func identifierRuneOracle(r rune) any {
	start := r == '_' || unicode.IsLetter(r)
	digit := unicode.IsDigit(r)
	return map[string]any{"in_rune_domain": true, "can_start": start, "can_continue": start || digit, "is_decimal_digit": digit}
}

func rejectedIdentifierRune() any {
	return map[string]any{"in_rune_domain": false, "can_start": false, "can_continue": false, "is_decimal_digit": false}
}
