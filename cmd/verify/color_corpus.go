package main

import (
	"fmt"
	"image/color"
	"math"
	"math/rand/v2"
)

const colorPortBatchSize = 64
const colorInspectorBatchSize = 32

type colorObservations struct {
	Suites                 []suite
	PortReferenceCases     int
	PortRejectionCases     int
	ConsumerReferenceCases int
	ConsumerRejectionCases int
}

func colorCorpus() (colorObservations, error) {
	result := colorObservations{}
	rgb := rgbColorInputs()
	for _, operation := range []struct {
		pkg, activity string
		arity         int
		inputs        [][]any
		oracle        func([]uint8) any
		rejected      any
	}{
		{"go/image/color", "RGBToCMYK", 3, rgb, rgbToCMYKOracle, rejectedCMYK()},
		{"go/image/color", "CMYKToRGB", 4, cmykColorInputs(), cmykToRGBOracle, rejectedRGB()},
		{"app/color-inspector", "InspectRGB", 3, rgb, inspectRGBOracle, rejectedColorInspection()},
	} {
		s, err := byteTupleSuite(operation.pkg, operation.activity, operation.arity, operation.inputs, operation.oracle)
		if err != nil {
			return colorObservations{}, err
		}
		references := len(s.Cases)
		// These retain original int64 values, including wrap/precision traps.
		// They check the Gooo extension contract, never Go equivalence.
		for _, n := range colorRejectedInputs() {
			for position := 0; position < operation.arity; position++ {
				input := []any{int64(12), int64(34), int64(56), int64(78)}[:operation.arity]
				input[position] = n
				s.Cases = append(s.Cases, testCase{Inputs: input, Expected: operation.rejected})
			}
		}
		rejections := len(s.Cases) - references
		s.Domain = fmt.Sprintf("uint8 tuples; %d finite Go-reference cases (boundary grid, gray ramp, division edges, fixed PCG samples); %d explicit out-of-byte rejection cases without narrowing", references, rejections)
		if operation.pkg == "go/image/color" {
			result.PortReferenceCases += references
			result.PortRejectionCases += rejections
		} else {
			result.ConsumerReferenceCases += references
			result.ConsumerRejectionCases += rejections
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func colorRejectedInputs() []int64 {
	return []int64{-1, 256, math.MinInt64, math.MaxInt64, -256, 257, 4294967296, 9007199254740993}
}

// Validate the WHOLE corpus before converting even the first input to uint8.
// Invalid arity, type, range or duplicate tuples must never reach the oracle.
func byteTupleSuite(pkg, activity string, arity int, inputs [][]any, oracle func([]uint8) any) (suite, error) {
	if (arity != 3 && arity != 4) || len(inputs) == 0 {
		return suite{}, fmt.Errorf("invalid color corpus shape")
	}
	seen := map[[4]int64]bool{}
	for _, input := range inputs {
		if len(input) != arity {
			return suite{}, fmt.Errorf("%s: expected %d byte inputs", activity, arity)
		}
		var key [4]int64
		for i, value := range input {
			n, ok := value.(int64)
			if !ok || n < 0 || n > 255 {
				return suite{}, fmt.Errorf("%s: input %v is not an int64 in the uint8 domain", activity, value)
			}
			key[i] = n
		}
		if seen[key] {
			return suite{}, fmt.Errorf("%s: duplicate color input %v", activity, input)
		}
		seen[key] = true
	}
	s := suite{Package: pkg, Activity: activity}
	for _, input := range inputs {
		channels := make([]uint8, arity)
		for i, value := range input {
			channels[i] = uint8(value.(int64))
		}
		s.Cases = append(s.Cases, testCase{Inputs: append([]any(nil), input...), Expected: oracle(channels)})
	}
	return s, nil
}

// Set construction removes overlap among named boundaries, grid, and samples.
// It does not replace byteTupleSuite's independent input validation.
func colorInputSet() (func(...int64), *[][]any) {
	var inputs [][]any
	seen := map[string]bool{}
	return func(values ...int64) {
		key := fmt.Sprint(values)
		if seen[key] {
			return
		}
		seen[key] = true
		row := make([]any, len(values))
		for i, v := range values {
			row[i] = v
		}
		inputs = append(inputs, row)
	}, &inputs
}

func rgbColorInputs() [][]any {
	add, inputs := colorInputSet()
	// Include the documented quantization example independently of the seed.
	add(142, 83, 202)
	edges := []int64{0, 1, 127, 128, 254, 255}
	for _, r := range edges {
		for _, g := range edges {
			for _, b := range edges {
				add(r, g, b)
			}
		}
	}
	for n := int64(0); n < 256; n++ {
		add(n, n, n)
	}
	// Max-channel ties and values adjacent to divisibility boundaries, with
	// the maximum in each of the three argument positions.
	for _, w := range []int64{2, 3, 7, 16, 63, 127, 128, 129, 254, 255} {
		add(w, w-1, 1)
		add(1, w, w-1)
		add(w-1, 1, w)
		add(w, w, 1)
		add(w, 1, w)
		add(1, w, w)
	}
	rng := rand.New(rand.NewPCG(47, 83))
	for i := 0; i < 128; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

func cmykColorInputs() [][]any {
	add, inputs := colorInputSet()
	edges := []int64{0, 1, 254, 255}
	for _, c := range edges {
		for _, m := range edges {
			for _, y := range edges {
				for _, k := range edges {
					add(c, m, y, k)
				}
			}
		}
	}
	for n := int64(0); n < 256; n++ {
		add(0, 0, 0, n)
	}
	for _, n := range []int64{2, 3, 7, 16, 63, 127, 128, 129, 253} {
		add(n, n-1, 1, 255-n)
		add(1, n, n-1, 255-n)
		add(n-1, 1, n, 255-n)
	}
	rng := rand.New(rand.NewPCG(59, 101))
	for i := 0; i < 128; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

// The oracle calls the pinned Go library; no conversion formula is copied.
func rgbToCMYKOracle(input []uint8) any {
	c, m, y, k := color.RGBToCMYK(input[0], input[1], input[2])
	return map[string]any{"cyan": c, "magenta": m, "yellow": y, "black": k}
}

func cmykToRGBOracle(input []uint8) any {
	r, g, b := color.CMYKToRGB(input[0], input[1], input[2], input[3])
	return map[string]any{"red": r, "green": g, "blue": b}
}

func inspectRGBOracle(input []uint8) any {
	c, m, y, k := color.RGBToCMYK(input[0], input[1], input[2])
	r, g, b := color.CMYKToRGB(c, m, y, k)
	return map[string]any{"in_byte_domain": true, "cyan": c, "magenta": m, "yellow": y, "black": k,
		"red": r, "green": g, "blue": b,
		"red_delta": int64(r) - int64(input[0]), "green_delta": int64(g) - int64(input[1]), "blue_delta": int64(b) - int64(input[2]),
		"exact": r == input[0] && g == input[1] && b == input[2]}
}

func rejectedCMYK() any {
	return map[string]any{"cyan": -1, "magenta": -1, "yellow": -1, "black": -1}
}

func rejectedRGB() any {
	return map[string]any{"red": -1, "green": -1, "blue": -1}
}

func rejectedColorInspection() any {
	return map[string]any{"in_byte_domain": false, "cyan": -1, "magenta": -1, "yellow": -1, "black": -1,
		"red": -1, "green": -1, "blue": -1, "red_delta": -1, "green_delta": -1, "blue_delta": -1, "exact": false}
}
