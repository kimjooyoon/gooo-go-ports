package main

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"strconv"
)

const rotationPortBatchSize = 128
const rotationInspectorBatchSize = 64

type rotationObservations struct {
	Suites                 []suite
	PortReferenceCases     int
	PortRejectionCases     int
	ConsumerReferenceCases int
	ConsumerRejectionCases int
}

func rotationCorpus() (rotationObservations, error) {
	result := rotationObservations{}
	for _, operation := range []struct {
		pkg, activity string
		inputs        [][]any
		oracle        func(uint8, int) any
		rejected      any
	}{
		{"go/math/bits", "RotateLeft8", rotationPortInputs(), func(x uint8, k int) any { return bits.RotateLeft8(x, k) }, -1},
		{"app/byte-rotation", "Inspect", rotationInspectorInputs(), inspectRotationOracle, rejectedRotationInspection()},
	} {
		s, err := rotationSuite(operation.pkg, operation.activity, operation.inputs, operation.oracle)
		if err != nil {
			return rotationObservations{}, err
		}
		references := len(s.Cases)
		// Invalid byte values are an explicit Gooo extension, never narrowed
		// to uint8 or sent to the Go oracle. Rotation inputs stay unchanged.
		for _, x := range rotationRejectedBytes() {
			for _, k := range []int64{0, -1, math.MinInt64, math.MaxInt64} {
				s.Cases = append(s.Cases, testCase{Inputs: []any{x, k}, Expected: operation.rejected})
			}
		}
		rejections := len(s.Cases) - references
		if operation.pkg == "go/math/bits" {
			s.Domain = fmt.Sprintf("uint8 x and signed 64-bit Go int k; %d Go-reference cases, including all 2048 pairs x=0..255 and k=0..7, named negative/large/int64 boundary rotations and fixed PCG samples; %d explicit out-of-byte rejection cases; rotations serialized unchanged", references, rejections)
			result.PortReferenceCases = references
			result.PortRejectionCases = rejections
		} else {
			s.Domain = fmt.Sprintf("importing consumer; %d finite Go-reference cases covering every byte at k=1, named byte/rotation boundaries and fixed PCG samples; %d explicit out-of-byte rejection cases; original rotations retained", references, rejections)
			result.ConsumerReferenceCases = references
			result.ConsumerRejectionCases = rejections
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

// Validate the complete input roster before any byte/int conversion or oracle
// call. A 32-bit Go int cannot preserve the signed int64 rotation domain.
func rotationSuite(pkg, activity string, inputs [][]any, oracle func(uint8, int) any) (suite, error) {
	if strconv.IntSize != 64 {
		return suite{}, fmt.Errorf("%s: RotateLeft8 oracle requires 64-bit Go int", activity)
	}
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("%s: empty rotation corpus", activity)
	}
	seen := map[[2]int64]bool{}
	for _, input := range inputs {
		if len(input) != 2 {
			return suite{}, fmt.Errorf("%s: expected byte and rotation inputs", activity)
		}
		x, byteOK := input[0].(int64)
		k, rotationOK := input[1].(int64)
		if !byteOK || x < 0 || x > 255 || !rotationOK {
			return suite{}, fmt.Errorf("%s: require int64 byte 0..255 and int64 rotation: %v", activity, input)
		}
		key := [2]int64{x, k}
		if seen[key] {
			return suite{}, fmt.Errorf("%s: duplicate rotation input %v", activity, input)
		}
		seen[key] = true
	}
	s := suite{Package: pkg, Activity: activity}
	for _, input := range inputs {
		x, k := input[0].(int64), input[1].(int64)
		s.Cases = append(s.Cases, testCase{Inputs: append([]any(nil), input...), Expected: oracle(uint8(x), int(k))})
	}
	return s, nil
}

func rotationBoundaryCounts() []int64 {
	return []int64{-33, -32, -31, -17, -16, -15, -9, -8, -7, -2, -1,
		8, 9, 15, 16, 17, 31, 32, 33,
		math.MinInt32, math.MaxInt32, -4294967296, 4294967296,
		-9007199254740993, -9007199254740992, -9007199254740991,
		9007199254740991, 9007199254740992, 9007199254740993,
		math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 7, math.MinInt64 + 8,
		math.MaxInt64 - 8, math.MaxInt64 - 7, math.MaxInt64 - 1, math.MaxInt64}
}

func rotationBoundaryBytes() []int64 { return []int64{0, 1, 2, 85, 127, 128, 170, 255} }

func rotationRejectedBytes() []int64 {
	return []int64{-1, 256, -256, 257, math.MinInt64, math.MaxInt64, -9007199254740993, 9007199254740993, 4294967296}
}

func rotationInputSet() (func(int64, int64), *[][]any) {
	var inputs [][]any
	seen := map[[2]int64]bool{}
	return func(x, k int64) {
		key := [2]int64{x, k}
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, []any{x, k})
		}
	}, &inputs
}

func rotationPortInputs() [][]any {
	add, inputs := rotationInputSet()
	for x := int64(0); x < 256; x++ {
		for k := int64(0); k < 8; k++ {
			add(x, k)
		}
	}
	for _, x := range rotationBoundaryBytes() {
		for _, k := range rotationBoundaryCounts() {
			add(x, k)
		}
	}
	rng := rand.New(rand.NewPCG(73, 131))
	for i := 0; i < 128; i++ {
		// Uint64 spans every bit pattern; conversion interprets it as a
		// signed sample. The resulting int64 itself is the oracle input.
		add(rng.Int64N(256), int64(rng.Uint64()))
	}
	return *inputs
}

func rotationInspectorInputs() [][]any {
	add, inputs := rotationInputSet()
	for x := int64(0); x < 256; x++ {
		add(x, 1)
	}
	for _, x := range rotationBoundaryBytes() {
		for k := int64(0); k < 8; k++ {
			add(x, k)
		}
		for _, k := range rotationBoundaryCounts() {
			add(x, k)
		}
	}
	rng := rand.New(rand.NewPCG(89, 149))
	for i := 0; i < 64; i++ {
		add(rng.Int64N(256), int64(rng.Uint64()))
	}
	return *inputs
}

// The independent consumer oracle uses Go's rotation and population-count
// functions. Its normalization uses Go's bit-mask rule, not port arithmetic.
func inspectRotationOracle(x uint8, k int) any {
	normalized := int(uint(k) & 7)
	rotated := bits.RotateLeft8(x, k)
	return map[string]any{
		"in_byte_domain": true, "normalized_rotation": normalized,
		"rotated": rotated, "restored": bits.RotateLeft8(rotated, 8-normalized),
		"ones_before": bits.OnesCount8(x), "ones_after": bits.OnesCount8(rotated),
	}
}

func rejectedRotationInspection() any {
	return map[string]any{"in_byte_domain": false, "normalized_rotation": -1,
		"rotated": -1, "restored": -1, "ones_before": -1, "ones_after": -1}
}
