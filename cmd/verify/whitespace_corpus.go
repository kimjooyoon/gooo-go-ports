package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const whitespacePortBatchSize = 128
const whitespaceInspectorBatchSize = 64

type whitespaceObservations struct {
	Suites                                         []suite
	PortReferenceCases, PortRejectionCases         int
	ConsumerReferenceCases, ConsumerRejectionCases int
}

func whitespaceCorpus() (whitespaceObservations, error) {
	result := whitespaceObservations{}
	inputs := whitespaceInputs()
	for _, op := range []struct {
		pkg, activity string
		oracle        func(rune) any
		rejected      any
	}{
		{"go/unicode", "IsSpace", func(r rune) any { return unicode.IsSpace(r) }, false},
		{"app/whitespace-inspector", "Inspect", whitespaceInspectionOracle, rejectedWhitespaceInspection()},
	} {
		s, err := whitespaceSuite(op.pkg, op.activity, inputs, op.oracle)
		if err != nil {
			return whitespaceObservations{}, err
		}
		for _, n := range whitespaceRejectedInputs() {
			s.Cases = append(s.Cases, testCase{Inputs: []any{n}, Expected: op.rejected})
		}
		s.Domain = fmt.Sprintf("signed int32 rune; %d Go-reference observations include all 256 Latin-1 values, all 25 pinned White_Space members and adjacent boundaries, nonspaces, invalid Unicode, int32 endpoints and fixed samples; %d original int64 outside-int32 inputs test an explicit rejection extension without narrowing", len(inputs), len(whitespaceRejectedInputs()))
		if op.pkg == "go/unicode" {
			result.PortReferenceCases = len(inputs)
			result.PortRejectionCases = len(whitespaceRejectedInputs())
		} else {
			result.ConsumerReferenceCases = len(inputs)
			result.ConsumerRejectionCases = len(whitespaceRejectedInputs())
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func whitespaceSuite(pkg, activity string, inputs [][]any, oracle func(rune) any) (suite, error) {
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty whitespace corpus")
	}
	seen := map[int64]bool{}
	// Check the entire roster before converting any original input to rune.
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

// Explicit members are coverage targets, not the oracle implementation.
func whitespaceMembers() []int64 {
	return []int64{9, 10, 11, 12, 13, 32, 133, 160, 5760, 8192, 8193, 8194, 8195, 8196, 8197, 8198, 8199, 8200, 8201, 8202, 8232, 8233, 8239, 8287, 12288}
}

func whitespaceBoundaryInputs() []int64 {
	return []int64{math.MinInt32, math.MinInt32 + 1, -65527, -65504, -65403, -1, 0,
		0x001c, 0x001d, 0x001e, 0x001f, 0x180e, 0x200b, 0x200c, 0x200d, 0x200e, 0x200f, 0x2060, 0x2800, 0xfeff,
		0x10009, 0x10020, 0x10085, 0xd7ff, 0xd800, 0xdbff, 0xdc00, 0xdfff, 0xe000,
		0xfffd, 0xffff, 0x10000, 0x10ffff, 0x110000, math.MaxInt32 - 1, math.MaxInt32}
}

func whitespaceInputs() [][]any {
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
	for _, n := range whitespaceMembers() {
		add(n - 1)
		add(n)
		add(n + 1)
	}
	for _, n := range whitespaceBoundaryInputs() {
		add(n)
	}
	rng := rand.New(rand.NewPCG(229, 263))
	for i := 0; i < 128; i++ {
		add(int64(int32(rng.Uint32())))
	}
	for i := 0; i < 64; i++ {
		add(int64(rng.Uint32N(0x110000)))
	}
	return inputs
}

func whitespaceRejectedInputs() []int64 {
	return []int64{math.MinInt64, -9007199254740993, -4294967264, math.MinInt32 - 1, math.MaxInt32 + 1, 4294967328, 9007199254740993, math.MaxInt64}
}

func whitespaceInspectionOracle(r rune) any {
	space := unicode.IsSpace(r)
	return map[string]any{"in_rune_domain": true, "valid_scalar": utf8.ValidRune(r), "is_go_space": space,
		"is_ascii_space": space && r >= 0 && r <= 127, "utf8_bytes": utf8.RuneLen(r), "utf16_units": utf16.RuneLen(r)}
}

func rejectedWhitespaceInspection() any {
	return map[string]any{"in_rune_domain": false, "valid_scalar": false, "is_go_space": false, "is_ascii_space": false, "utf8_bytes": -1, "utf16_units": -1}
}
