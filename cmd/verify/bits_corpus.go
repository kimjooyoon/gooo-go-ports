package main

import (
	"fmt"
	"math/bits"
)

func mathBitsCorpus() ([]suite, error) {
	inputs := make([]int64, 256)
	for i := range inputs {
		inputs[i] = int64(i)
	}
	operations := []struct {
		name   string
		oracle func(uint8) any
	}{
		{"Len8", func(n uint8) any { return bits.Len8(n) }},
		{"LeadingZeros8", func(n uint8) any { return bits.LeadingZeros8(n) }},
		{"TrailingZeros8", func(n uint8) any { return bits.TrailingZeros8(n) }},
		{"OnesCount8", func(n uint8) any { return bits.OnesCount8(n) }},
		{"Reverse8", func(n uint8) any { return bits.Reverse8(n) }},
	}
	var all []suite
	for _, operation := range operations {
		s, err := byteSuite("go/math/bits", operation.name,
			"all 256 uint8 values exactly once; out-of-domain Gooo inputs are excluded",
			inputs, true, operation.oracle)
		if err != nil {
			return nil, err
		}
		all = append(all, s)
	}
	consumer, err := byteSuite("app/byte-summary", "ByteSummary",
		"importing consumer; uint8 inputs 0,1,127,128,170,255",
		[]int64{0, 1, 127, 128, 170, 255}, false, func(n uint8) any {
			return map[string]any{"length": bits.Len8(n), "ones": bits.OnesCount8(n), "reversed": bits.Reverse8(n)}
		})
	if err != nil {
		return nil, err
	}
	return append(all, consumer), nil
}

func byteSuite(pkg, activity, domain string, inputs []int64, exhaustive bool, oracle func(uint8) any) (suite, error) {
	s := suite{Package: pkg, Activity: activity, Domain: domain}
	// Validate the complete corpus before any uint8 conversion or oracle call.
	if err := validateByteInputs(inputs, exhaustive); err != nil {
		return suite{}, fmt.Errorf("%s:%s: %w", pkg, activity, err)
	}
	for _, n := range inputs {
		s.Cases = append(s.Cases, testCase{Inputs: []any{n}, Expected: oracle(uint8(n))})
	}
	return s, nil
}

func validateByteInputs(inputs []int64, exhaustive bool) error {
	if len(inputs) == 0 {
		return fmt.Errorf("byte corpus is empty")
	}
	seen := [256]bool{}
	for _, n := range inputs {
		if n < 0 || n > 255 {
			return fmt.Errorf("input %d is outside the uint8 domain; conversion would change it", n)
		}
		if seen[n] {
			return fmt.Errorf("duplicate byte input %d", n)
		}
		seen[n] = true
	}
	if exhaustive {
		for n, present := range seen {
			if !present {
				return fmt.Errorf("exhaustive byte corpus is missing input %d", n)
			}
		}
	}
	return nil
}
