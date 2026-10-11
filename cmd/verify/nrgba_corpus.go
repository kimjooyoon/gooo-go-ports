package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
)

const nrgbaPortBatchSize = 64
const nrgbaConsumerBatchSize = 32

var nrgbaChannelNames = []string{"red", "green", "blue", "alpha"}
var nrgbaRGBNames = []string{"red", "green", "blue"}

type nrgbaObservations struct {
	Suites                                         []suite
	PortReferenceCases, PortRejectionCases         int
	ConsumerReferenceCases, ConsumerRejectionCases int
}

func nrgbaRecord(values ...int64) map[string]any {
	result := map[string]any{}
	for i, value := range values {
		result[nrgbaChannelNames[i]] = value
	}
	return result
}

// Read exact signed integers before testing the byte domain or narrowing. The
// same reader binds rejection receipts, whose original fields may be any int64.
func readNRGBARecord(value any, names []string) ([]int64, error) {
	fields, ok := value.(map[string]any)
	if !ok || len(fields) != len(names) {
		return nil, fmt.Errorf("color record requires exactly %d named int64 fields", len(names))
	}
	result := make([]int64, len(names))
	for i, name := range names {
		n, ok := fields[name].(int64)
		if !ok {
			return nil, fmt.Errorf("color field %s must preserve an exact int64", name)
		}
		result[i] = n
	}
	return result, nil
}

// Validate the complete roster before any uint8 conversion or oracle call.
func nrgbaRecordSuite(pkg, activity string, consumer bool, inputs [][]any, oracle func([]uint8) (any, error)) (suite, error) {
	arity := 1
	if consumer {
		arity = 2
	}
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty NRGBA corpus")
	}
	seen := map[[7]int64]bool{}
	for _, row := range inputs {
		if len(row) != arity {
			return suite{}, fmt.Errorf("%s requires %d records", activity, arity)
		}
		var key [7]int64
		for position, value := range row {
			names, offset := nrgbaChannelNames, 0
			if position == 1 {
				names, offset = nrgbaRGBNames, 4
			}
			fields, err := readNRGBARecord(value, names)
			if err != nil {
				return suite{}, err
			}
			for i, n := range fields {
				if n < 0 || n > 255 {
					return suite{}, fmt.Errorf("%s original field %s is outside uint8: %d", activity, names[i], n)
				}
				key[offset+i] = n
			}
		}
		if seen[key] {
			return suite{}, fmt.Errorf("duplicate NRGBA tuple")
		}
		seen[key] = true
	}
	result := suite{Package: pkg, Activity: activity}
	for _, row := range inputs {
		var channels []uint8
		original := make([]any, len(row))
		for position, value := range row {
			names := nrgbaChannelNames
			if position == 1 {
				names = nrgbaRGBNames
			}
			fields, _ := readNRGBARecord(value, names)
			original[position] = nrgbaRecord(fields...)
			for _, n := range fields {
				channels = append(channels, uint8(n))
			}
		}
		want, err := oracle(channels)
		if err != nil {
			return suite{}, fmt.Errorf("%s oracle: %w", activity, err)
		}
		result.Cases = append(result.Cases, testCase{Inputs: original, Expected: want})
	}
	return result, nil
}

func nrgbaCorpus() (nrgbaObservations, error) {
	result := nrgbaObservations{}
	for _, operation := range []struct {
		pkg, activity string
		consumer      bool
		inputs        [][]any
		oracle        func([]uint8) (any, error)
	}{
		{"go/image/color", "NRGBAToRGBA", false, nrgbaPortInputs(), nrgbaRGBAOracle},
		{"app/alpha-matte", "MattePixel", true, nrgbaMatteInputs(), nrgbaMatteOracle},
	} {
		s, err := nrgbaRecordSuite(operation.pkg, operation.activity, operation.consumer, operation.inputs, operation.oracle)
		if err != nil {
			return nrgbaObservations{}, err
		}
		references := len(s.Cases)
		for _, row := range nrgbaRejectedRecords(operation.consumer) {
			rejected := rejectedNRGBA()
			if operation.consumer {
				rejected = rejectedNRGBAMatte()
			}
			s.Cases = append(s.Cases, testCase{Inputs: row, Expected: rejected})
		}
		rejections := len(s.Cases) - references
		s.Domain = fmt.Sprintf("NRGBA record with four original int64 channels in 0..255; %d finite pinned color.NRGBA.RGBA reference cases; %d separately counted out-of-byte rejection cases preserving every original field. Boundary grid, complete selected alpha/channel ramps, rounding neighbors, hidden RGB above alpha and fixed PCG tuples; no exhaustive four-byte-domain claim", references, rejections)
		if operation.consumer {
			s.Domain = fmt.Sprintf("NRGBA foreground and opaque RGB background records, seven original int64 channels in 0..255; %d finite reference cases use actual 1x1 image/draw.Draw with draw.Over, no mask, checking optimized and generic image paths for agreement, plus color.NRGBA.RGBA diagnostics; %d separate out-of-byte rejections before any narrowing, including alpha-zero/opaque shortcuts. Final alpha is implicitly 255. No whole-image, linear-light, color-management or exhaustive seven-byte-domain claim", references, rejections)
			result.ConsumerReferenceCases, result.ConsumerRejectionCases = references, rejections
		} else {
			result.PortReferenceCases, result.PortRejectionCases = references, rejections
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func nrgbaInputSet(consumer bool) (func(...int64), *[][]any) {
	add, tuples := colorInputSet()
	var records [][]any
	return func(values ...int64) {
		before := len(*tuples)
		add(values...)
		if before == len(*tuples) {
			return
		}
		row := []any{nrgbaRecord(values[:4]...)}
		if consumer {
			row = append(row, nrgbaRecord(values[4:]...))
		}
		records = append(records, row)
	}, &records
}

func nrgbaPortInputs() [][]any {
	add, inputs := nrgbaInputSet(false)
	// Named anchors precede the finite grid so independent manual expectations
	// also exercise transparent hidden RGB, opaque endpoints and half alpha.
	for _, row := range [][4]int64{{255, 0, 0, 128}, {12, 34, 56, 0}, {12, 34, 56, 255}, {255, 1, 128, 1}, {1, 2, 3, 127}, {254, 127, 1, 254}} {
		add(row[:]...)
	}
	edges := []int64{0, 1, 127, 128, 254, 255}
	for _, a := range edges {
		for _, r := range edges {
			for _, g := range edges {
				for _, b := range edges {
					add(r, g, b, a)
				}
			}
		}
	}
	for n := int64(0); n < 256; n++ {
		add(255, 1, 128, n)
		add(12, 34, 56, n)
		for position := 0; position < 3; position++ {
			row := [4]int64{17, 83, 202, 128}
			row[position] = n
			add(row[:]...)
		}
	}
	// Adjacent channel/alpha values expose integer truncation without copying
	// the port formula into its oracle.
	for _, a := range []int64{1, 2, 126, 127, 128, 129, 253, 254} {
		for _, n := range []int64{1, 2, 126, 127, 128, 129, 253, 254} {
			add(n-1, n, n+1, a)
		}
	}
	rng := rand.New(rand.NewPCG(211, 307))
	for i := 0; i < 64; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

func nrgbaMatteInputs() [][]any {
	add, inputs := nrgbaInputSet(true)
	for _, row := range [][7]int64{{255, 0, 0, 128, 255, 255, 255}, {12, 34, 56, 0, 17, 83, 202}, {12, 34, 56, 255, 17, 83, 202}, {255, 1, 128, 1, 0, 0, 0}, {1, 2, 3, 127, 254, 127, 1}} {
		add(row[:]...)
	}
	foregrounds := [][3]int64{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {1, 127, 254}, {254, 128, 1}, {17, 83, 202}}
	backgrounds := [][3]int64{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 127, 255}, {202, 83, 17}}
	for _, a := range []int64{0, 1, 127, 128, 254, 255} {
		for _, fg := range foregrounds {
			for _, bg := range backgrounds {
				add(fg[0], fg[1], fg[2], a, bg[0], bg[1], bg[2])
			}
		}
	}
	for n := int64(0); n < 256; n++ {
		add(255, 1, 128, n, 17, 83, 202)
		for position := 0; position < 3; position++ {
			fg := [7]int64{17, 83, 202, 128, 202, 83, 17}
			fg[position] = n
			add(fg[:]...)
			bg := [7]int64{17, 83, 202, 127, 202, 83, 17}
			bg[position+4] = n
			add(bg[:]...)
		}
	}
	for _, a := range []int64{1, 2, 126, 127, 128, 129, 253, 254} {
		for _, n := range []int64{1, 2, 126, 127, 128, 129, 253, 254} {
			add(n-1, n, n+1, a, 255-n, n, 0)
		}
	}
	rng := rand.New(rand.NewPCG(223, 331))
	for i := 0; i < 64; i++ {
		add(rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256), rng.Int64N(256))
	}
	return *inputs
}

func nrgbaRejectedRecords(consumer bool) [][]any {
	add, inputs := nrgbaInputSet(consumer)
	channels := 4
	if consumer {
		channels = 7
	}
	for _, n := range colorRejectedInputs() {
		for position := 0; position < channels; position++ {
			row := []int64{12, 34, 56, 78, 17, 83, 202}[:channels]
			row[position] = n
			add(row...)
		}
		if consumer {
			// Neither transparent source nor opaque source permits skipping the
			// contract's validation of any original foreground/background field.
			for position := 0; position < 3; position++ {
				row := []int64{12, 34, 56, 0, 17, 83, 202}
				row[position] = n
				add(row...)
				row = []int64{12, 34, 56, 255, 17, 83, 202}
				row[position+4] = n
				add(row...)
			}
		}
	}
	return *inputs
}

func nrgbaRGBAOracle(input []uint8) (any, error) {
	r, g, b, a := (color.NRGBA{R: input[0], G: input[1], B: input[2], A: input[3]}).RGBA()
	return map[string]any{"red": r, "green": g, "blue": b, "alpha": a}, nil
}

// Wrappers intentionally expose only image.Image/draw.Image methods, avoiding
// draw's concrete NRGBA/RGBA and RGBA64 fast paths in the independent second run.
type nrgbaGenericSource struct{ image.Image }
type nrgbaGenericDestination struct{ draw.Image }

func nrgbaDrawOracle(input []uint8) (color.RGBA, error) {
	bounds := image.Rect(0, 0, 1, 1)
	src := image.NewNRGBA(bounds)
	src.SetNRGBA(0, 0, color.NRGBA{R: input[0], G: input[1], B: input[2], A: input[3]})
	background := color.RGBA{R: input[4], G: input[5], B: input[6], A: 255}
	fast, generic := image.NewRGBA(bounds), image.NewRGBA(bounds)
	fast.SetRGBA(0, 0, background)
	generic.SetRGBA(0, 0, background)
	draw.Draw(fast, bounds, src, image.Point{}, draw.Over)
	draw.Draw(nrgbaGenericDestination{generic}, bounds, nrgbaGenericSource{src}, image.Point{}, draw.Over)
	got, want := fast.RGBAAt(0, 0), generic.RGBAAt(0, 0)
	if got != want || got.A != 255 {
		return color.RGBA{}, fmt.Errorf("draw fast/generic or opaque-alpha disagreement for %v: %+v vs %+v", input, got, want)
	}
	return got, nil
}

func nrgbaMatteOracle(input []uint8) (any, error) {
	pixel, err := nrgbaDrawOracle(input)
	if err != nil {
		return nil, err
	}
	r, g, b, a := (color.NRGBA{R: input[0], G: input[1], B: input[2], A: input[3]}).RGBA()
	return map[string]any{"in_byte_domain": true, "premultiplied_red": r, "premultiplied_green": g, "premultiplied_blue": b, "premultiplied_alpha": a, "red": pixel.R, "green": pixel.G, "blue": pixel.B}, nil
}
func rejectedNRGBA() map[string]any {
	return map[string]any{"red": -1, "green": -1, "blue": -1, "alpha": -1}
}
func rejectedNRGBAMatte() map[string]any {
	return map[string]any{"in_byte_domain": false, "premultiplied_red": -1, "premultiplied_green": -1, "premultiplied_blue": -1, "premultiplied_alpha": -1, "red": -1, "green": -1, "blue": -1}
}
