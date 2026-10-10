package main

import (
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"strconv"
)

const geometryUnaryBatchSize = 64
const geometryBinaryBatchSize = 32

type geometryObservations struct {
	Suites                                     []suite
	PortReferenceCases, ConsumerReferenceCases int
}

type rectangleCoordinates [4]int64

var rectangleNames = [4]string{"min_x", "min_y", "max_x", "max_y"}

func rectangleRecord(r rectangleCoordinates) map[string]any {
	return map[string]any{"min_x": r[0], "min_y": r[1], "max_x": r[2], "max_y": r[3]}
}

func readRectangle(value any) (rectangleCoordinates, error) {
	var result rectangleCoordinates
	fields, ok := value.(map[string]any)
	if !ok || len(fields) != len(rectangleNames) {
		return result, fmt.Errorf("Rectangle requires exactly four named int64 fields")
	}
	for i, name := range rectangleNames {
		n, ok := fields[name].(int64)
		if !ok {
			return result, fmt.Errorf("Rectangle field %s must preserve an exact int64", name)
		}
		result[i] = n
	}
	return result, nil
}

func geometryCorpus() (geometryObservations, error) {
	result := geometryObservations{}
	for _, op := range []struct {
		pkg, activity string
		arity         int
		inputs        [][]any
		oracle        func([]image.Rectangle) any
	}{
		{"go/image", "RectangleEmpty", 1, geometryUnaryInputs(), func(r []image.Rectangle) any { return r[0].Empty() }},
		{"go/image", "RectangleOverlaps", 2, geometryBinaryInputs(), func(r []image.Rectangle) any { return r[0].Overlaps(r[1]) }},
		{"go/image", "RectangleIntersect", 2, geometryBinaryInputs(), func(r []image.Rectangle) any { return imageRectangleRecord(r[0].Intersect(r[1])) }},
		{"go/image", "RectangleUnion", 2, geometryBinaryInputs(), func(r []image.Rectangle) any { return imageRectangleRecord(r[0].Union(r[1])) }},
		{"example/crop-planner", "CropPlan", 2, geometryBinaryInputs(), cropPlanOracle},
	} {
		s, err := geometrySuite(op.pkg, op.activity, op.arity, op.inputs, op.oracle)
		if err != nil {
			return geometryObservations{}, err
		}
		s.Domain = fmt.Sprintf("%d Rectangle record(s), each with four original signed int64 coordinates on 64-bit Go; %d finite independent image.Rectangle method comparisons cover a small grid, empty and noncanonical shapes, touching edges, containment, int64 and precision boundaries and fixed samples; no normalization, coordinate narrowing, domain-rejection extension, pixel processing or exhaustive-domain claim", op.arity, len(s.Cases))
		if op.pkg == "go/image" {
			result.PortReferenceCases += len(s.Cases)
		} else {
			result.ConsumerReferenceCases += len(s.Cases)
		}
		result.Suites = append(result.Suites, s)
	}
	return result, nil
}

func geometrySuite(pkg, activity string, arity int, inputs [][]any, oracle func([]image.Rectangle) any) (suite, error) {
	if strconv.IntSize != 64 || (arity != 1 && arity != 2) || len(inputs) == 0 {
		return suite{}, fmt.Errorf("geometry requires 64-bit Go and a nonempty one/two-record roster")
	}
	seen := map[[8]int64]bool{}
	// Validate every original record before constructing any Go int or invoking
	// the oracle. Empty and noncanonical rectangles are valid original inputs.
	for _, input := range inputs {
		if len(input) != arity {
			return suite{}, fmt.Errorf("%s requires %d Rectangle records", activity, arity)
		}
		var key [8]int64
		for i, value := range input {
			r, err := readRectangle(value)
			if err != nil {
				return suite{}, fmt.Errorf("%s input%d: %w", activity, i, err)
			}
			copy(key[4*i:], r[:])
		}
		if seen[key] {
			return suite{}, fmt.Errorf("%s duplicate original Rectangle tuple", activity)
		}
		seen[key] = true
	}
	result := suite{Package: pkg, Activity: activity}
	for _, input := range inputs {
		original := make([]any, arity)
		rectangles := make([]image.Rectangle, arity)
		for i, value := range input {
			r, _ := readRectangle(value)
			original[i] = rectangleRecord(r)
			// Rect and Canon would silently normalize original reversed bounds.
			rectangles[i] = image.Rectangle{Min: image.Point{X: int(r[0]), Y: int(r[1])}, Max: image.Point{X: int(r[2]), Y: int(r[3])}}
		}
		result.Cases = append(result.Cases, testCase{Inputs: original, Expected: oracle(rectangles)})
	}
	return result, nil
}

func imageRectangleRecord(r image.Rectangle) map[string]any {
	return rectangleRecord(rectangleCoordinates{int64(r.Min.X), int64(r.Min.Y), int64(r.Max.X), int64(r.Max.Y)})
}

func cropPlanOracle(r []image.Rectangle) any {
	clipped := r[0].Intersect(r[1])
	result := imageRectangleRecord(clipped)
	result["has_pixels"] = !clipped.Empty()
	return result
}

func geometryGrid(values []int64) []rectangleCoordinates {
	var result []rectangleCoordinates
	for _, x0 := range values {
		for _, y0 := range values {
			for _, x1 := range values {
				for _, y1 := range values {
					result = append(result, rectangleCoordinates{x0, y0, x1, y1})
				}
			}
		}
	}
	return result
}

func geometryBoundaryValues() []int64 {
	return []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -4294967296, -2147483649,
		2147483648, 4294967296, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
}

func geometryExtremeRectangles() []rectangleCoordinates {
	return []rectangleCoordinates{
		{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64},
		{math.MinInt64, math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 1},
		{math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64},
		{math.MaxInt64, math.MaxInt64, math.MinInt64, math.MinInt64},
	}
}

func geometryNamedPairs() [][2]rectangleCoordinates {
	a := rectangleCoordinates{0, 0, 4, 4}
	return [][2]rectangleCoordinates{
		{a, {1, 1, 3, 3}}, {a, {2, 2, 6, 6}}, {a, {4, 0, 6, 4}},
		{a, {0, 4, 4, 6}}, {a, {4, 4, 6, 6}}, {a, {5, 0, 7, 4}},
		{a, {3, 0, 4, 4}}, {{9, 8, 2, 1}, {-7, -6, -7, 4}},
	}
}

func geometryUnaryInputs() [][]any {
	var inputs [][]any
	seen := map[rectangleCoordinates]bool{}
	add := func(r rectangleCoordinates) {
		if !seen[r] {
			seen[r] = true
			inputs = append(inputs, []any{rectangleRecord(r)})
		}
	}
	for _, r := range geometryGrid([]int64{-1, 0, 1}) {
		add(r)
	}
	for i := 0; i < 4; i++ {
		for _, value := range geometryBoundaryValues() {
			r := rectangleCoordinates{3, 5, 11, 13}
			r[i] = value
			add(r)
		}
	}
	for _, r := range geometryExtremeRectangles() {
		add(r)
	}
	rng := rand.New(rand.NewPCG(317, 331))
	for target := len(inputs) + 128; len(inputs) < target; {
		add(rectangleCoordinates{int64(rng.Uint64()), int64(rng.Uint64()), int64(rng.Uint64()), int64(rng.Uint64())})
	}
	return inputs
}

func geometryBinaryInputs() [][]any {
	var inputs [][]any
	seen := map[[2]rectangleCoordinates]bool{}
	add := func(a, b rectangleCoordinates) {
		key := [2]rectangleCoordinates{a, b}
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, []any{rectangleRecord(a), rectangleRecord(b)})
		}
	}
	grid := geometryGrid([]int64{0, 1})
	for _, a := range grid {
		for _, b := range grid {
			add(a, b)
		}
	}
	for _, pair := range geometryNamedPairs() {
		add(pair[0], pair[1])
		add(pair[1], pair[0])
	}
	for i := 0; i < 8; i++ {
		for _, value := range geometryBoundaryValues() {
			pair := [2]rectangleCoordinates{{3, 5, 11, 13}, {7, 9, 17, 19}}
			pair[i/4][i%4] = value
			add(pair[0], pair[1])
		}
	}
	extremes := geometryExtremeRectangles()
	for _, r := range extremes[1:] {
		add(extremes[0], r)
		add(r, extremes[0])
	}
	rng := rand.New(rand.NewPCG(347, 367))
	for target := len(inputs) + 128; len(inputs) < target; {
		var pair [2]rectangleCoordinates
		for i := range pair {
			for j := range pair[i] {
				pair[i][j] = int64(rng.Uint64())
			}
		}
		add(pair[0], pair[1])
	}
	return inputs
}
