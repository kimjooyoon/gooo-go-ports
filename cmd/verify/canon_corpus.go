package main

import (
	"fmt"
	"image"
	"math"
)

// Canon observations are separate from the frozen geometry corpus. No existing
// method or CropPlan input is normalized or replaced while adding these suites.
func canonCorpus() (geometryObservations, error) {
	result := geometryObservations{}
	inputs := geometryUnaryInputs()
	canonOracle := func(r []image.Rectangle) any { return imageRectangleRecord(r[0].Canon()) }
	initial, err := geometrySuite("go/image", "RectangleCanon", 1, inputs, canonOracle)
	if err != nil {
		return result, err
	}
	seen := map[rectangleCoordinates]bool{}
	for _, row := range initial.Cases {
		r, _ := readRectangle(row.Inputs[0])
		seen[r] = true
	}
	// Retain original noncanonical inputs, then add any absent canonical results
	// as independent inputs. This observes Canon(Canon(r)) == Canon(r) finitely.
	for _, row := range initial.Cases {
		r, err := readRectangle(row.Expected)
		if err != nil {
			return result, err
		}
		if !seen[r] {
			seen[r] = true
			inputs = append(inputs, []any{rectangleRecord(r)})
		}
	}
	canon, err := geometrySuite("go/image", "RectangleCanon", 1, inputs, canonOracle)
	if err != nil {
		return result, err
	}
	canon.Domain = fmt.Sprintf("one Rectangle with four original signed int64 coordinates on 64-bit Go; %d finite Go Rectangle.Canon comparisons include all {-1,0,1} coordinate tuples, each-coordinate int64/precision boundaries, fixed samples and each observed canonical result; equal bounds remain degenerate at their original location; comparisons and swaps only, no narrowing, arithmetic, extension or exhaustive-domain claim", len(canon.Cases))
	drag, err := geometrySuite("example/crop-planner", "DragCropPlan", 2, dragCropInputs(), dragCropOracle)
	if err != nil {
		return result, err
	}
	drag.Domain = fmt.Sprintf("two original signed-int64 Rectangle records on 64-bit Go; %d finite Go comparisons of bounds.Intersect(drag.Canon()) and !Empty; only the second record is canonicalized, preserving the original bounds and the unchanged CropPlan contract; includes each drag direction, clipping, empty/touching bounds, extrema and precision boundaries; coordinates only, no image pixels or exhaustive-domain claim", len(drag.Cases))
	result.Suites = []suite{canon, drag}
	result.PortReferenceCases, result.ConsumerReferenceCases = len(canon.Cases), len(drag.Cases)
	return result, nil
}

func dragCropOracle(r []image.Rectangle) any {
	clipped := r[0].Intersect(r[1].Canon())
	result := imageRectangleRecord(clipped)
	result["has_pixels"] = !clipped.Empty()
	return result
}

func dragCropNamedPairs() [][2]rectangleCoordinates {
	bounds := rectangleCoordinates{0, 0, 640, 480}
	const p = int64(9007199254740993)
	return [][2]rectangleCoordinates{
		{bounds, {100, 100, 200, 200}}, {bounds, {200, 100, 100, 200}},
		{bounds, {100, 200, 200, 100}}, {bounds, {200, 200, 100, 100}},
		{bounds, {800, 700, -20, -30}}, {bounds, {640, 0, 700, 100}},
		{bounds, {650, 100, 640, 0}}, {bounds, {100, 50, 100, 200}},
		{bounds, {50, 100, 200, 100}}, {bounds, {100, 100, 100, 100}},
		{{10, 10, 0, 0}, {1, 1, 9, 9}}, {{-20, -20, 10, 10}, {5, 4, -8, -30}},
		{{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64}, {math.MaxInt64, math.MaxInt64, math.MinInt64, math.MinInt64}},
		{{math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64}, {math.MaxInt64, math.MaxInt64, math.MaxInt64 - 1, math.MaxInt64 - 1}},
		{{math.MinInt64, math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 1}, {math.MinInt64 + 1, math.MinInt64 + 1, math.MinInt64, math.MinInt64}},
		{{p, -p, p + 4, -p + 4}, {p + 3, -p + 3, p + 1, -p + 1}},
	}
}

func dragCropInputs() [][]any {
	inputs := geometryBinaryInputs()
	seen := map[[8]int64]bool{}
	for _, input := range inputs {
		var key [8]int64
		for i, value := range input {
			r, _ := readRectangle(value)
			copy(key[i*4:], r[:])
		}
		seen[key] = true
	}
	for _, pair := range dragCropNamedPairs() {
		var key [8]int64
		copy(key[:4], pair[0][:])
		copy(key[4:], pair[1][:])
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, []any{rectangleRecord(pair[0]), rectangleRecord(pair[1])})
		}
	}
	return inputs
}
