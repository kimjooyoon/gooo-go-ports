package main

import (
	"fmt"
	"image"
	"math"
)

const containmentPortBatchSize = geometryBinaryBatchSize
const fitsCropBatchSize = geometryBinaryBatchSize
const fitsCropPackage = "example/fits-crop"

type containmentObservations struct {
	Suites                                        []suite
	PortReferenceCases, ConsumerReferenceCases    int
	BaselineReferenceInputs, AddedReferenceInputs int
}

type containmentNamedPair struct {
	Name                string
	Receiver, Container rectangleCoordinates
}

// Coordinates are supplied directly. No Rect/Canon, coordinate subtraction,
// width calculation, float conversion or input narrowing builds these cases.
func containmentNamedPairs() []containmentNamedPair {
	bounds := rectangleCoordinates{10, 20, 30, 40}
	full := rectangleCoordinates{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64}
	precisionBounds := rectangleCoordinates{9007199254740993, -9007199254740997, 9007199254741003, -9007199254740987}
	return []containmentNamedPair{
		{"equal", bounds, bounds},
		{"strictly-contained", rectangleCoordinates{11, 21, 29, 39}, bounds},
		{"shared-minimum", rectangleCoordinates{10, 20, 20, 30}, bounds},
		{"shared-maximum", rectangleCoordinates{20, 30, 30, 40}, bounds},
		{"cross-min-x", rectangleCoordinates{9, 21, 20, 30}, bounds},
		{"cross-min-y", rectangleCoordinates{11, 19, 20, 30}, bounds},
		{"cross-max-x", rectangleCoordinates{20, 21, 31, 30}, bounds},
		{"cross-max-y", rectangleCoordinates{11, 30, 20, 41}, bounds},
		{"touch-left", rectangleCoordinates{0, 21, 10, 30}, bounds},
		{"touch-top", rectangleCoordinates{11, 10, 20, 20}, bounds},
		{"touch-right", rectangleCoordinates{30, 21, 35, 30}, bounds},
		{"touch-bottom", rectangleCoordinates{11, 40, 20, 45}, bounds},
		{"touch-corner", rectangleCoordinates{30, 40, 35, 45}, bounds},
		{"disjoint", rectangleCoordinates{50, 60, 70, 80}, bounds},
		{"zero-width", rectangleCoordinates{15, 21, 15, 30}, bounds},
		{"zero-height", rectangleCoordinates{11, 25, 20, 25}, bounds},
		{"reversed-x", rectangleCoordinates{20, 21, 11, 30}, bounds},
		{"reversed-y", rectangleCoordinates{11, 30, 20, 21}, bounds},
		{"reversed-both", rectangleCoordinates{20, 30, 11, 21}, bounds},
		{"empty-outside", rectangleCoordinates{88, 99, 88, 99}, bounds},
		{"nonempty-in-empty", rectangleCoordinates{11, 21, 20, 30}, rectangleCoordinates{15, 25, 15, 25}},
		{"empty-in-empty", rectangleCoordinates{99, 88, 99, 88}, rectangleCoordinates{15, 25, 15, 25}},
		{"empty-in-reversed", rectangleCoordinates{99, 88, 99, 88}, rectangleCoordinates{30, 40, 10, 20}},
		{"nonempty-in-reversed", rectangleCoordinates{11, 21, 20, 30}, rectangleCoordinates{30, 40, 10, 20}},
		{"full-int64-equal", full, full},
		{"min-int64-unit", rectangleCoordinates{math.MinInt64, math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 1}, full},
		{"max-int64-unit", rectangleCoordinates{math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64}, full},
		{"reversed-int64", rectangleCoordinates{math.MaxInt64, math.MaxInt64, math.MinInt64, math.MinInt64}, full},
		{"precision-contained", rectangleCoordinates{9007199254740994, -9007199254740996, 9007199254741002, -9007199254740988}, precisionBounds},
		{"precision-cross-min-x", rectangleCoordinates{9007199254740992, -9007199254740996, 9007199254741002, -9007199254740988}, precisionBounds},
		{"precision-cross-min-y", rectangleCoordinates{9007199254740994, -9007199254740998, 9007199254741002, -9007199254740988}, precisionBounds},
		{"precision-cross-max-x", rectangleCoordinates{9007199254740994, -9007199254740996, 9007199254741004, -9007199254740988}, precisionBounds},
		{"precision-cross-max-y", rectangleCoordinates{9007199254740994, -9007199254740996, 9007199254741002, -9007199254740986}, precisionBounds},
	}
}

// These anchors are also published as the standalone example inputs.
func containmentExamplePairs() []containmentNamedPair {
	return []containmentNamedPair{
		{"example/contained", rectangleCoordinates{2, 2, 8, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/empty_receiver_empty_container", rectangleCoordinates{20, 20, 20, 21}, rectangleCoordinates{0, 0, 0, 0}},
		{"example/cross_left", rectangleCoordinates{-1, 2, 8, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/cross_top", rectangleCoordinates{2, -1, 8, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/cross_right", rectangleCoordinates{2, 2, 11, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/cross_bottom", rectangleCoordinates{2, 2, 8, 11}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/touching_disjoint_right", rectangleCoordinates{10, 2, 11, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/zero_width_outside", rectangleCoordinates{42, 2, 42, 8}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/zero_height_outside", rectangleCoordinates{2, 42, 8, 42}, rectangleCoordinates{0, 0, 10, 10}},
		{"example/reversed_x_extremes", rectangleCoordinates{9223372036854775807, 0, -9223372036854775808, 1}, rectangleCoordinates{-9223372036854775808, -9223372036854775808, 9223372036854775807, 9223372036854775807}},
		{"example/reversed_y_extremes", rectangleCoordinates{0, 9223372036854775807, 1, -9223372036854775808}, rectangleCoordinates{-9223372036854775808, -9223372036854775808, 9223372036854775807, 9223372036854775807}},
		{"example/nonempty_receiver_reversed_extreme_container", rectangleCoordinates{1, 1, 2, 2}, rectangleCoordinates{9223372036854775807, 9223372036854775807, -9223372036854775808, -9223372036854775808}},
		{"example/equal_full_int64_bounds", rectangleCoordinates{-9223372036854775808, -9223372036854775808, 9223372036854775807, 9223372036854775807}, rectangleCoordinates{-9223372036854775808, -9223372036854775808, 9223372036854775807, 9223372036854775807}},
		{"example/above_2pow53_one_unit_left_cross", rectangleCoordinates{9007199254740992, -9007199254740992, 9007199254740996, -9007199254740991}, rectangleCoordinates{9007199254740993, -9007199254740993, 9007199254740996, -9007199254740988}},
	}
}

func containmentInputs() [][]any {
	var inputs [][]any
	seen := map[[2]rectangleCoordinates]bool{}
	add := func(a, b rectangleCoordinates) {
		key := [2]rectangleCoordinates{a, b}
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, []any{rectangleRecord(a), rectangleRecord(b)})
		}
	}
	// Preserve the original 486 tuple prefix exactly, including its fixed PCG
	// samples and each-coordinate int64 and +/-2^53 boundaries.
	for _, input := range geometryBinaryInputs() {
		a, err := readRectangle(input[0])
		if err != nil {
			panic(err)
		}
		b, err := readRectangle(input[1])
		if err != nil {
			panic(err)
		}
		add(a, b)
	}
	for _, pair := range append(containmentNamedPairs(), containmentExamplePairs()...) {
		add(pair.Receiver, pair.Container)
		add(pair.Container, pair.Receiver)
	}
	return inputs
}

func rectangleInOracle(r []image.Rectangle) any { return r[0].In(r[1]) }
func fitsCropOracle(r []image.Rectangle) any    { return !r[1].Empty() && r[1].In(r[0]) }

func containmentCorpus() (containmentObservations, error) {
	inputs := containmentInputs()
	port, err := geometrySuite("go/image", "RectangleIn", 2, inputs, rectangleInOracle)
	if err != nil {
		return containmentObservations{}, err
	}
	consumer, err := geometrySuite(fitsCropPackage, "FitsCrop", 2, inputs, fitsCropOracle)
	if err != nil {
		return containmentObservations{}, err
	}
	baseline := len(geometryBinaryInputs())
	port.Domain = fmt.Sprintf("%d finite original pairs of four-field signed-int64 Rectangle records on 64-bit Go; receiver input0, container input1; independent image.Rectangle.In oracle. Empty/reversed receivers are inside any container; equal maximum edges are allowed; nonempty receivers do not fit empty containers. Original %d geometry tuples remain a prefix, with named equal/contained/crossed/touching/disjoint/empty/reversed/extreme/precision pairs added in both directions and exact tuple deduplication. No coordinate arithmetic, normalization, narrowing, domain extension, image/GIF processing or exhaustive pair-domain claim.", len(inputs), baseline)
	consumer.Domain = fmt.Sprintf("%d finite original Rectangle pairs; bounds input0, requested input1. Independent original image.Rectangle.Empty and In calls return true only for a nonempty requested rectangle wholly inside bounds. This is a separate no-clipping acceptance policy: port empty-receiver true is not consumer acceptance. Raw coordinates and existing CropPlan/DragCropPlan behavior are unchanged; no pixel processing or exhaustive pair-domain claim.", len(inputs))
	return containmentObservations{Suites: []suite{port, consumer}, PortReferenceCases: len(port.Cases), ConsumerReferenceCases: len(consumer.Cases), BaselineReferenceInputs: baseline, AddedReferenceInputs: len(inputs) - baseline}, nil
}
