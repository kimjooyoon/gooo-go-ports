package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"reflect"
	"testing"
)

func TestGeometryCoverageAndExactSerialization(t *testing.T) {
	added, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 5 || added.PortReferenceCases != 1711 || added.ConsumerReferenceCases != 486 {
		t.Fatalf("geometry coverage changed: %+v", added)
	}
	for i, s := range added.Suites {
		want, size := 486, geometryBinaryBatchSize
		if i == 0 {
			want, size = 253, geometryUnaryBatchSize
		}
		if len(s.Cases) != want {
			t.Fatalf("%s: got %d cases, want %d", s.Activity, len(s.Cases), want)
		}
		seen := map[[8]int64]bool{}
		for _, c := range s.Cases {
			var key [8]int64
			for j, input := range c.Inputs {
				r, err := readRectangle(input)
				if err != nil {
					t.Fatal(err)
				}
				copy(key[4*j:], r[:])
			}
			if seen[key] {
				t.Fatal("duplicate original tuple")
			}
			seen[key] = true
		}
		require := func(a, b rectangleCoordinates) {
			t.Helper()
			var key [8]int64
			copy(key[:], a[:])
			copy(key[4:], b[:])
			if !seen[key] {
				t.Fatalf("%s missing original tuple %v", s.Activity, key)
			}
		}
		if i == 0 {
			for _, a := range geometryGrid([]int64{-1, 0, 1}) {
				require(a, rectangleCoordinates{})
			}
			for _, a := range geometryExtremeRectangles() {
				require(a, rectangleCoordinates{})
			}
		} else {
			for _, a := range geometryGrid([]int64{0, 1}) {
				for _, b := range geometryGrid([]int64{0, 1}) {
					require(a, b)
				}
			}
			for _, pair := range geometryNamedPairs() {
				require(pair[0], pair[1])
				require(pair[1], pair[0])
			}
			for j, c := range s.Cases {
				if !reflect.DeepEqual(c.Inputs, added.Suites[1].Cases[j].Inputs) {
					t.Fatal("binary methods or consumer changed original roster")
				}
			}
		}
		arity := len(s.Cases[0].Inputs)
		for position := 0; position < arity*4; position++ {
			for _, n := range geometryBoundaryValues() {
				pair := [2]rectangleCoordinates{{3, 5, 11, 13}, {7, 9, 17, 19}}
				if arity == 1 {
					pair[1] = rectangleCoordinates{}
				}
				pair[position/4][position%4] = n
				require(pair[0], pair[1])
			}
		}
		batches, maxBytes := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 || len(batch.Cases) > 128 {
				t.Fatalf("geometry batch exceeds native limit: %d %v", len(raw)+1, err)
			}
			var doc struct {
				Cases []struct{ Inputs, Expected map[string]json.RawMessage }
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Cases) != len(batch.Cases) {
				t.Fatal("serialized case count")
			}
			for j, row := range doc.Cases {
				if len(row.Inputs) != arity || len(row.Expected) != 1 {
					t.Fatal("serialized input/result roster")
				}
				key := s.Package + ":" + s.Activity
				for k, value := range batch.Cases[j].Inputs {
					name := key
					if arity > 1 {
						name = fmt.Sprintf("%s.input%d", key, k)
					}
					if !equalValue(row.Inputs[name], value) {
						t.Fatal("serialized original Rectangle changed")
					}
				}
				if !equalValue(row.Expected[key], batch.Cases[j].Expected) {
					t.Fatal("serialized oracle result changed")
				}
			}
			batches++
			maxBytes = max(maxBytes, len(raw)+1)
		}
		t.Logf("%s:%s cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maxBytes)
	}
	again, err := geometryCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("nondeterministic geometry corpus", err)
	}
}

func TestGeometryPreservesNoncanonicalAndEmptyResults(t *testing.T) {
	added, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	zero := rectangleCoordinates{}
	a := rectangleCoordinates{0, 0, 4, 4}
	for _, example := range []struct {
		r, s, intersect, union rectangleCoordinates
		overlaps               bool
	}{
		{a, rectangleCoordinates{1, 1, 3, 3}, rectangleCoordinates{1, 1, 3, 3}, a, true},
		{a, rectangleCoordinates{2, 2, 6, 6}, rectangleCoordinates{2, 2, 4, 4}, rectangleCoordinates{0, 0, 6, 6}, true},
		{a, rectangleCoordinates{4, 0, 6, 4}, zero, rectangleCoordinates{0, 0, 6, 4}, false},
		{a, rectangleCoordinates{0, 4, 4, 6}, zero, rectangleCoordinates{0, 0, 4, 6}, false},
		{a, rectangleCoordinates{4, 4, 6, 6}, zero, rectangleCoordinates{0, 0, 6, 6}, false},
		{a, rectangleCoordinates{5, 0, 7, 4}, zero, rectangleCoordinates{0, 0, 7, 4}, false},
		{rectangleCoordinates{9, 8, 2, 1}, rectangleCoordinates{-7, -6, -7, 4}, zero, rectangleCoordinates{-7, -6, -7, 4}, false},
		{rectangleCoordinates{-7, -6, -7, 4}, rectangleCoordinates{9, 8, 2, 1}, zero, rectangleCoordinates{9, 8, 2, 1}, false},
	} {
		input := []any{rectangleRecord(example.r), rectangleRecord(example.s)}
		checkConsumerCase(t, added.Suites[1], input, example.overlaps)
		checkConsumerCase(t, added.Suites[2], input, rectangleRecord(example.intersect))
		checkConsumerCase(t, added.Suites[3], input, rectangleRecord(example.union))
		crop := rectangleRecord(example.intersect)
		crop["has_pixels"] = example.overlaps
		checkConsumerCase(t, added.Suites[4], input, crop)
	}
	extremes := geometryExtremeRectangles()
	for i, r := range extremes {
		checkConsumerCase(t, added.Suites[0], []any{rectangleRecord(r)}, i == 3)
	}
	for _, r := range extremes[1:3] {
		for _, input := range [][]any{{rectangleRecord(extremes[0]), rectangleRecord(r)}, {rectangleRecord(r), rectangleRecord(extremes[0])}} {
			checkConsumerCase(t, added.Suites[1], input, true)
			checkConsumerCase(t, added.Suites[2], input, rectangleRecord(r))
			checkConsumerCase(t, added.Suites[3], input, rectangleRecord(extremes[0]))
		}
	}
}

func TestGeometryValidatesEveryRecordBeforeOracle(t *testing.T) {
	valid := rectangleRecord(rectangleCoordinates{3, 5, 11, 13})
	bad := []any{nil, []int64{3, 5, 11, 13}, map[string]int64{"min_x": 3, "min_y": 5, "max_x": 11, "max_y": 13}, "rectangle"}
	for _, name := range rectangleNames {
		missing := rectangleRecord(rectangleCoordinates{3, 5, 11, 13})
		delete(missing, name)
		bad = append(bad, missing)
		for _, value := range []any{nil, int(0), int32(0), uint64(math.MaxUint64), float64(9007199254740993), 0.5, "3", json.Number("3"), json.Number("9223372036854775808"), json.Number("-9223372036854775809")} {
			changed := rectangleRecord(rectangleCoordinates{3, 5, 11, 13})
			changed[name] = value
			bad = append(bad, changed)
		}
	}
	extra := rectangleRecord(rectangleCoordinates{3, 5, 11, 13})
	extra["other"] = int64(0)
	bad = append(bad, extra)
	for _, arity := range []int{1, 2} {
		for position := 0; position < arity; position++ {
			for _, value := range bad {
				first := []any{rectangleRecord(rectangleCoordinates{0, 0, 1, 1}), rectangleRecord(rectangleCoordinates{1, 1, 2, 2})}[:arity]
				second := []any{valid, valid}[:arity]
				second[position] = value
				calls := 0
				_, err := geometrySuite("test", "test", arity, [][]any{first, second}, func([]image.Rectangle) any { calls++; return nil })
				if err == nil || calls != 0 {
					t.Fatalf("bad record reached oracle: arity=%d position=%d value=%v calls=%d", arity, position, value, calls)
				}
			}
		}
	}
	for _, input := range [][][]any{nil, {{valid}, {valid}}, {{valid}, {}}, {{valid}, {valid, valid}}} {
		calls := 0
		_, err := geometrySuite("test", "test", 1, input, func([]image.Rectangle) any { calls++; return nil })
		if err == nil || calls != 0 {
			t.Fatal("invalid roster reached oracle")
		}
	}
	r := geometryExtremeRectangles()[0]
	input := rectangleRecord(r)
	s, err := geometrySuite("test", "test", 1, [][]any{{input}}, func(rs []image.Rectangle) any {
		if !reflect.DeepEqual(imageRectangleRecord(rs[0]), rectangleRecord(r)) {
			t.Fatal("original int64 coordinates narrowed")
		}
		return rs[0].Empty()
	})
	if err != nil {
		t.Fatal(err)
	}
	input["min_x"] = int64(0)
	if !reflect.DeepEqual(s.Cases[0].Inputs[0], rectangleRecord(r)) {
		t.Fatal("saved record aliases caller storage")
	}
}

func geometryLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	legacy := classificationLegacyCorpus(t)
	classification, err := classificationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(legacy, classification.Suites...)
}

func TestGeometryPreservesExisting24770Cases(t *testing.T) {
	before := geometryLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	if count != 24770 {
		t.Fatalf("legacy count changed: %d", count)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from the unchanged corpus before adding geometry.
	const existing = "aaf0d12f5e695d1cdcf9609985817b22debead5f0b14ab3414cc8859884a5fc1"
	if hash != existing {
		t.Fatalf("legacy 24770-case corpus changed: %s", hash)
	}
	if _, err := geometryCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, geometryLegacyCorpus(t)) {
		t.Fatal("geometry construction mutated legacy corpus")
	}
}
