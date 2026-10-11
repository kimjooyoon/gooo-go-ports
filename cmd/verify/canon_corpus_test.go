package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestCanonCorpusOriginalInputsIdempotenceAndBatchBudget(t *testing.T) {
	added, err := canonCorpus()
	if err != nil {
		t.Fatal(err)
	}
	again, err := canonCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("nondeterministic Canon corpus", err)
	}
	if len(added.Suites) != 2 {
		t.Fatal("Canon suite roster")
	}
	for i, s := range added.Suites {
		size := geometryUnaryBatchSize
		if i == 1 {
			size = geometryBinaryBatchSize
		}
		seen := map[string]testCase{}
		for _, row := range s.Cases {
			key := string(mustJSON(t, row.Inputs))
			if _, exists := seen[key]; exists {
				t.Fatal("duplicate original tuple")
			}
			seen[key] = row
		}
		original := geometryUnaryInputs()
		if i == 1 {
			original = geometryBinaryInputs()
		}
		for _, input := range original {
			if _, exists := seen[string(mustJSON(t, input))]; !exists {
				t.Fatal("discarded original noncanonical tuple")
			}
		}
		if i == 0 {
			for _, row := range s.Cases {
				canonical := []any{row.Expected}
				second, exists := seen[string(mustJSON(t, canonical))]
				if !exists || !reflect.DeepEqual(row.Expected, second.Expected) {
					t.Fatal("missing or unequal idempotence observation")
				}
			}
		}
		batches, largest := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatal("native cases budget", len(raw)+1, err)
			}
			var doc struct {
				Cases []struct{ Inputs, Expected map[string]json.RawMessage }
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			for j, row := range doc.Cases {
				entry := s.Package + ":" + s.Activity
				for k, v := range batch.Cases[j].Inputs {
					key := entry
					if i == 1 {
						key = fmt.Sprintf("%s.input%d", entry, k)
					}
					if !equalValue(row.Inputs[key], v) {
						t.Fatal("serialized original int64 input changed")
					}
				}
				if !equalValue(row.Expected[entry], batch.Cases[j].Expected) {
					t.Fatal("serialized oracle result changed")
				}
			}
			batches++
			largest = max(largest, len(raw)+1)
		}
		t.Logf("%s cases=%d batches=%d maxbytes=%d", s.Activity, len(s.Cases), batches, largest)
	}
}

func TestCanonLiteralEdgesAndDirectionIndependentSelection(t *testing.T) {
	added, err := canonCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ input, want rectangleCoordinates }{
		{rectangleCoordinates{1, 1, -1, -1}, rectangleCoordinates{-1, -1, 1, 1}},
		{rectangleCoordinates{1, -1, -1, 1}, rectangleCoordinates{-1, -1, 1, 1}},
		{rectangleCoordinates{-1, 1, 1, -1}, rectangleCoordinates{-1, -1, 1, 1}},
		{rectangleCoordinates{1, 1, 1, 1}, rectangleCoordinates{1, 1, 1, 1}},
		{rectangleCoordinates{math.MaxInt64, math.MaxInt64, math.MinInt64, math.MinInt64}, rectangleCoordinates{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64}},
	} {
		checkConsumerCase(t, added.Suites[0], []any{rectangleRecord(row.input)}, rectangleRecord(row.want))
	}
	pairs := dragCropNamedPairs()
	for i, pair := range pairs {
		var rect rectangleCoordinates
		hasPixels := false
		switch i {
		case 0, 1, 2, 3:
			rect = rectangleCoordinates{100, 100, 200, 200}
			hasPixels = true
		case 4:
			rect = rectangleCoordinates{0, 0, 640, 480}
			hasPixels = true
		case 11:
			rect = rectangleCoordinates{-8, -20, 5, 4}
			hasPixels = true
		case 12:
			rect = rectangleCoordinates{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64}
			hasPixels = true
		case 13:
			rect = rectangleCoordinates{math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64}
			hasPixels = true
		case 14:
			rect = rectangleCoordinates{math.MinInt64, math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 1}
			hasPixels = true
		case 15:
			const p = int64(9007199254740993)
			rect = rectangleCoordinates{p + 1, -p + 1, p + 3, -p + 3}
			hasPixels = true
		}
		want := rectangleRecord(rect)
		want["has_pixels"] = hasPixels
		checkConsumerCase(t, added.Suites[1], []any{rectangleRecord(pair[0]), rectangleRecord(pair[1])}, want)
	}
}

func canonLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	base := ycbcrLegacyCorpus(t)
	ycbcr, err := ycbcrCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(base, ycbcr.Suites...)
}

func TestCanonPreservesExisting34170Cases(t *testing.T) {
	before := canonLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	const frozen = "b51f8442275b066656ce713f7ea4ce1bc0f72651349112bb40f6b65d44e28cf4"
	if count != 34170 || fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before))) != frozen {
		t.Fatal("existing 34170-case corpus changed")
	}
	if _, err := canonCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, canonLegacyCorpus(t)) {
		t.Fatal("Canon construction mutated old inputs/oracles")
	}
}

func TestCanonDocumentedDragInputsMatchGoOracle(t *testing.T) {
	raw, err := os.ReadFile("../../examples/crop-planner/drag_inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Schema string                        `json:"schema"`
		Inputs []map[string]map[string]int64 `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != "gooo/body-composition-inputs/v1" || len(document.Inputs) != 12 {
		t.Fatal("documented fixture roster")
	}
	hi, lo := int64(math.MaxInt64), int64(math.MinInt64)
	want := []rectangleCoordinates{{100, 100, 200, 200}, {100, 100, 200, 200}, {100, 100, 200, 200}, {100, 100, 200, 200}, {0, 40, 200, 480}, {}, {}, {}, {-8, -30, 5, 4}, {hi - 1, hi - 1, hi, hi}, {lo, lo, lo + 1, lo + 1}, {9007199254740993, -9007199254740992, 9007199254740994, -9007199254740991}}
	for i, row := range document.Inputs {
		if len(row) != 2 {
			t.Fatal("input arity")
		}
		r := make([]image.Rectangle, 2)
		for j := range r {
			value, exists := row[fmt.Sprintf("example/crop-planner:DragCropPlan.input%d", j)]
			if !exists || len(value) != 4 {
				t.Fatal("input record shape")
			}
			for _, name := range rectangleNames {
				if _, exists := value[name]; !exists {
					t.Fatal("missing coordinate")
				}
			}
			r[j] = image.Rectangle{Min: image.Point{X: int(value["min_x"]), Y: int(value["min_y"])}, Max: image.Point{X: int(value["max_x"]), Y: int(value["max_y"])}}
		}
		expected := rectangleRecord(want[i])
		expected["has_pixels"] = (i < 5 || i > 7)
		if !reflect.DeepEqual(dragCropOracle(r), expected) {
			t.Fatalf("documented row %d differs from original Go", i+1)
		}
	}
}
