package main

import (
	"fmt"
	"image/color"
	"math/rand/v2"
)

const ycbcrPortBatchSize = 64
const ycbcrInspectorBatchSize = 32

type ycbcrObservations struct {
	Suites                 []suite
	PortReferenceCases     int
	PortRejectionCases     int
	ConsumerReferenceCases int
	ConsumerRejectionCases int
}

func ycbcrCorpus() (ycbcrObservations, error) {
	result := ycbcrObservations{}
	rgb := ycbcrRGBInputs()
	for _, operation := range []struct {
		pkg, activity string
		inputs        [][]any
		oracle        func([]uint8) any
		rejected      any
	}{
		{"go/image/color", "RGBToYCbCr", rgb, rgbToYCbCrOracle, rejectedYCbCr()},
		{"go/image/color", "YCbCrToRGB", ycbcrInputs(), ycbcrToRGBOracle, rejectedRGB()},
		{"app/ycbcr-inspector", "InspectRGB", rgb, inspectYCbCrRGBOracle, rejectedYCbCrInspection()},
	} {
		// The shared tuple validator checks the complete int64 roster before
		// ANY uint8 conversion or call to the independent standard library.
		s, err := byteTupleSuite(operation.pkg, operation.activity, 3, operation.inputs, operation.oracle)
		if err != nil {
			return ycbcrObservations{}, err
		}
		references := len(s.Cases)
		for _, n := range colorRejectedInputs() {
			for position := 0; position < 3; position++ {
				input := []any{int64(12), int64(34), int64(56)}
				input[position] = n
				s.Cases = append(s.Cases, testCase{Inputs: input, Expected: operation.rejected})
			}
		}
		rejections := len(s.Cases) - references
		s.Domain = fmt.Sprintf("uint8 triples; %d finite Go-reference cases (boundary grid, gray ramp, rounding/saturation neighbors, fixed PCG samples); %d separate out-of-byte rejection cases preserving original int64 inputs", references, rejections)
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

func ycbcrRGBInputs() [][]any {
	add, inputs := colorInputSet()
	// Named primaries expose chroma rounding and upper saturation. The
	// asymmetric sample makes channel swaps and lossy round trips visible.
	for _, row := range [][3]int64{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {128, 64, 32}, {142, 83, 202}, {202, 142, 83}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		add(row[:]...)
	}
	edges := []int64{0, 1, 2, 127, 128, 129, 254, 255}
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
	// Adjacent channels around neutral gray exercise positive and negative
	// chroma contributions without copying the conversion arithmetic.
	for _, n := range []int64{2, 7, 16, 63, 127, 128, 129, 253} {
		for position := 0; position < 3; position++ {
			for _, delta := range []int64{-1, 1} {
				row := [3]int64{n, n, n}
				row[position] += delta
				add(row[:]...)
			}
		}
	}
	rng := rand.New(rand.NewPCG(109, 163))
	for i := 0; i < 128; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

func ycbcrInputs() [][]any {
	add, inputs := colorInputSet()
	edges := []int64{0, 1, 2, 127, 128, 129, 254, 255}
	for _, y := range edges {
		for _, cb := range edges {
			for _, cr := range edges {
				add(y, cb, cr)
			}
		}
	}
	for y := int64(0); y < 256; y++ {
		add(y, 128, 128)
	}
	// Discover the first nonzero and first saturated value of every output
	// channel for selected chroma pairs using Go itself. Retain both sides
	// and the next neighbor. This is finite boundary selection, not copied
	// Gooo arithmetic and not an exhaustive assertion about all chroma pairs.
	for _, cb := range []uint8{0, 1, 127, 128, 129, 254, 255} {
		for _, cr := range []uint8{0, 1, 127, 128, 129, 254, 255} {
			pr, pg, pb := color.YCbCrToRGB(0, cb, cr)
			previous := [3]uint8{pr, pg, pb}
			for y := int64(1); y < 256; y++ {
				r, g, b := color.YCbCrToRGB(uint8(y), cb, cr)
				current := [3]uint8{r, g, b}
				for channel, n := range current {
					if (previous[channel] == 0 && n > 0) || (previous[channel] < 255 && n == 255) {
						for _, neighbor := range []int64{y - 1, y, min(y+1, 255)} {
							add(neighbor, int64(cb), int64(cr))
						}
					}
				}
				previous = current
			}
		}
	}
	rng := rand.New(rand.NewPCG(127, 181))
	for i := 0; i < 128; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

// Expected outputs come from the pinned Go standard library, never the ports.
func rgbToYCbCrOracle(input []uint8) any {
	y, cb, cr := color.RGBToYCbCr(input[0], input[1], input[2])
	return map[string]any{"y": y, "cb": cb, "cr": cr}
}

func ycbcrToRGBOracle(input []uint8) any {
	r, g, b := color.YCbCrToRGB(input[0], input[1], input[2])
	return map[string]any{"red": r, "green": g, "blue": b}
}

func inspectYCbCrRGBOracle(input []uint8) any {
	y, cb, cr := color.RGBToYCbCr(input[0], input[1], input[2])
	r, g, b := color.YCbCrToRGB(y, cb, cr)
	return map[string]any{"in_byte_domain": true, "y": y, "cb": cb, "cr": cr,
		"red": r, "green": g, "blue": b,
		"red_delta": int64(r) - int64(input[0]), "green_delta": int64(g) - int64(input[1]), "blue_delta": int64(b) - int64(input[2]),
		"exact": r == input[0] && g == input[1] && b == input[2]}
}

func rejectedYCbCr() any {
	return map[string]any{"y": -1, "cb": -1, "cr": -1}
}

func rejectedYCbCrInspection() any {
	return map[string]any{"in_byte_domain": false, "y": -1, "cb": -1, "cr": -1,
		"red": -1, "green": -1, "blue": -1, "red_delta": -1, "green_delta": -1, "blue_delta": -1, "exact": false}
}
