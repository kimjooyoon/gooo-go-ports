package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"unicode"
)

const simpleFoldPortBatchSize = 128
const simpleFoldConsumerBatchSize = 64

type simpleFoldObservations struct {
	Suites                                         []suite
	PortReferenceCases, PortExtensionCases         int
	ConsumerReferenceCases, ConsumerRejectionCases int
}

func simpleFoldCorpus() (simpleFoldObservations, error) {
	if unicode.Version != "17.0.0" {
		return simpleFoldObservations{}, fmt.Errorf("SimpleFold corpus requires Unicode 17.0.0, got %s", unicode.Version)
	}
	cycles, changed, maximum, err := simpleFoldCycles()
	if err != nil || maximum != 4 {
		return simpleFoldObservations{}, fmt.Errorf("pinned SimpleFold cycle bound changed: maximum=%d: %v", maximum, err)
	}
	port, err := simpleFoldSuite("go/unicode", "SimpleFold", 1, simpleFoldPortInputs(changed), func(r []rune) any {
		return int64(unicode.SimpleFold(r[0]))
	})
	if err != nil {
		return simpleFoldObservations{}, err
	}
	consumer, err := simpleFoldSuite("app/simple-fold", "Compare", 2, simpleFoldConsumerInputs(cycles), simpleFoldComparisonOracle)
	if err != nil {
		return simpleFoldObservations{}, err
	}
	result := simpleFoldObservations{PortReferenceCases: len(port.Cases), ConsumerReferenceCases: len(consumer.Cases)}
	for _, n := range simpleFoldExtendedInputs() {
		port.Cases = append(port.Cases, testCase{Inputs: []any{n}, Expected: n})
		result.PortExtensionCases++
		for _, pair := range [][2]int64{{n, 'A'}, {'a', n}, {n, n}} {
			consumer.Cases = append(consumer.Cases, testCase{Inputs: []any{pair[0], pair[1]}, Expected: simpleFoldRejectedComparison(pair[0], pair[1])})
			result.ConsumerRejectionCases++
		}
	}
	consumer.Cases = append(consumer.Cases, testCase{Inputs: []any{int64(math.MaxInt64), int64(math.MinInt64)}, Expected: simpleFoldRejectedComparison(math.MaxInt64, math.MinInt64)})
	result.ConsumerRejectionCases++
	port.Domain = fmt.Sprintf("original signed int32 rune; %d finite Go-reference observations include every changing code point, all Latin-1 values, every CaseRanges boundary, invalid Unicode and fixed samples; %d original outside-int32 inputs observe a separate identity extension without narrowing", result.PortReferenceCases, result.PortExtensionCases)
	consumer.Domain = fmt.Sprintf("two original signed int32 runes; %d finite Go-reference observations include all ordered pairs in every 3/4-member fold cycle, named 1/2-member cycles, false pairs, invalid Unicode and fixed samples; %d outside-int32 pairs return false/false/original-left/original-right as a separate rejection extension; not whole-string, locale, normalization or full case folding", result.ConsumerReferenceCases, result.ConsumerRejectionCases)
	result.Suites = []suite{port, consumer}
	return result, nil
}

// This is Go-only coverage selection and an exhaustive bound check, not a
// million-case native corpus or a claim of exhaustive Gooo equivalence.
func simpleFoldCycles() ([][]rune, []int64, int, error) {
	var cycles [][]rune
	var changed []int64
	seen := map[rune]bool{}
	maximum := 1
	for r := rune(0); r <= unicode.MaxRune; r++ {
		next := unicode.SimpleFold(r)
		if next == r {
			continue
		}
		changed = append(changed, int64(r))
		if seen[r] {
			continue
		}
		var cycle []rune
		local := map[rune]bool{}
		for member := r; ; member = unicode.SimpleFold(member) {
			if local[member] {
				if member != r {
					return nil, nil, 0, fmt.Errorf("fold orbit from U+%04X does not return to its origin", r)
				}
				break
			}
			if member < 0 || member > unicode.MaxRune || seen[member] || len(cycle) >= 4 {
				return nil, nil, 0, fmt.Errorf("unexpected fold cycle from U+%04X through U+%04X", r, member)
			}
			local[member], seen[member] = true, true
			cycle = append(cycle, member)
		}
		maximum = max(maximum, len(cycle))
		cycles = append(cycles, cycle)
	}
	return cycles, changed, maximum, nil
}

func simpleFoldSuite(pkg, activity string, arity int, inputs [][]any, oracle func([]rune) any) (suite, error) {
	if len(inputs) == 0 || (arity != 1 && arity != 2) {
		return suite{}, fmt.Errorf("SimpleFold requires a nonempty one/two-rune roster")
	}
	seen := map[[2]int64]bool{}
	// Check the complete original roster before any rune conversion or oracle call.
	for _, input := range inputs {
		if len(input) != arity {
			return suite{}, fmt.Errorf("%s requires %d original rune inputs", activity, arity)
		}
		var key [2]int64
		for i, value := range input {
			n, ok := value.(int64)
			if !ok || n < math.MinInt32 || n > math.MaxInt32 {
				return suite{}, fmt.Errorf("%s input%d must be an exact int64 in the int32 rune domain", activity, i)
			}
			key[i] = n
		}
		if seen[key] {
			return suite{}, fmt.Errorf("%s duplicate original input tuple", activity)
		}
		seen[key] = true
	}
	s := suite{Package: pkg, Activity: activity}
	for _, input := range inputs {
		runes := make([]rune, arity)
		for i, value := range input {
			runes[i] = rune(value.(int64))
		}
		s.Cases = append(s.Cases, testCase{Inputs: append([]any(nil), input...), Expected: oracle(runes)})
	}
	return s, nil
}

func simpleFoldPortInputs(changed []int64) [][]any {
	var result [][]any
	seen := map[int64]bool{}
	add := func(n int64) {
		if !seen[n] {
			seen[n] = true
			result = append(result, []any{n})
		}
	}
	for n := int64(0); n <= 255; n++ {
		add(n)
	}
	for _, n := range changed {
		add(n)
	}
	for _, cr := range unicode.CaseRanges {
		for _, n := range []int64{int64(cr.Lo) - 1, int64(cr.Lo), int64(cr.Lo) + 1, int64(cr.Hi) - 1, int64(cr.Hi), int64(cr.Hi) + 1} {
			add(n)
		}
	}
	for _, n := range simpleFoldRuneBoundaries() {
		add(n)
	}
	rng := rand.New(rand.NewPCG(379, 383))
	for i := 0; i < 64; i++ {
		add(int64(int32(rng.Uint32())))
		add(int64(rng.Uint32N(0x110000)))
	}
	return result
}

func simpleFoldConsumerInputs(cycles [][]rune) [][]any {
	var result [][]any
	seen := map[[2]int64]bool{}
	add := func(left, right int64) {
		key := [2]int64{left, right}
		if !seen[key] {
			seen[key] = true
			result = append(result, []any{left, right})
		}
	}
	for _, cycle := range cycles {
		if len(cycle) > 2 {
			for _, left := range cycle {
				for _, right := range cycle {
					add(int64(left), int64(right))
				}
			}
		}
	}
	// Named scripts and the self-orbit overrides for dotted/dotless I. All
	// expectations still come from the original Go functions after validation.
	for _, pair := range [][2]int64{
		{'A', 'a'}, {'K', 0x212a}, {'s', 0x17f}, {0x3a3, 0x3c2}, {0x398, 0x3f4}, {0x345, 0x1fbe},
		{0x13b0, 0xab80}, {0x10400, 0x10428}, {0x130, 'I'}, {0x131, 'i'},
		{0xdf, 0x1e9e}, {0xdf, 's'}, {0xe9, 'e'}, {' ', ' '}, {'A', 'B'},
		{0x100, 0x101}, {0x139, 0x13a}, {0x1c4, 0x1c5}, {0x1c5, 0x1c6},
		{0x1c90, 0x10d0}, {0x1e900, 0x1e922}, {'0', 'O'}, {0x212a, 0x212b},
	} {
		add(pair[0], pair[1])
		add(pair[1], pair[0])
	}
	for _, n := range simpleFoldRuneBoundaries() {
		add(n, n)
		add(n, 'A')
		add('a', n)
	}
	add(-1, math.MaxInt32)
	add(0xd800, 0xd801)
	rng := rand.New(rand.NewPCG(389, 397))
	for i := 0; i < 64; i++ {
		add(int64(int32(rng.Uint32())), int64(int32(rng.Uint32())))
		add(int64(rng.Uint32N(0x110000)), int64(rng.Uint32N(0x110000)))
	}
	return result
}

func simpleFoldRuneBoundaries() []int64 {
	return []int64{math.MinInt32, math.MinInt32 + 1, -65471, -65429, -1, 0,
		0x7f, 0x80, 0x130, 0x131, 0xd7ff, 0xd800, 0xdbff, 0xdc00, 0xdfff, 0xe000,
		0xfffd, 0xffff, 0x10000, 0x10ffff, 0x110000, 0x110041, math.MaxInt32 - 1, math.MaxInt32}
}

func simpleFoldExtendedInputs() []int64 {
	return []int64{math.MinInt64, -9007199254740993, -9007199254740992, -4294967231, math.MinInt32 - 1,
		math.MaxInt32 + 1, 4294967361, 4294967403, 9007199254740992, 9007199254740993, math.MaxInt64}
}

func simpleFoldComparisonOracle(input []rune) any {
	left, right := input[0], input[1]
	equivalent := left == right
	// Follow Go's actual orbit until it closes, independently of the consumer's
	// three unrolled successor comparisons and the generated port's tables.
	for member := unicode.SimpleFold(left); member != left; member = unicode.SimpleFold(member) {
		equivalent = equivalent || member == right
	}
	return map[string]any{"in_rune_domain": true, "equivalent": equivalent,
		"left_next": int64(unicode.SimpleFold(left)), "right_next": int64(unicode.SimpleFold(right))}
}

func simpleFoldRejectedComparison(left, right int64) any {
	return map[string]any{"in_rune_domain": false, "equivalent": false, "left_next": left, "right_next": right}
}
