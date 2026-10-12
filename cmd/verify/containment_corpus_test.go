package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func containmentRawRectangle(r rectangleCoordinates) image.Rectangle {
	return image.Rectangle{Min: image.Point{X: int(r[0]), Y: int(r[1])}, Max: image.Point{X: int(r[2]), Y: int(r[3])}}
}

func TestContainmentOriginalGoEmptyPolicyAndNamedSemantics(t *testing.T) {
	wants := map[string][2]bool{
		"equal": {true, true}, "strictly-contained": {true, true}, "shared-minimum": {true, true}, "shared-maximum": {true, true},
		"cross-min-x": {false, false}, "cross-min-y": {false, false}, "cross-max-x": {false, false}, "cross-max-y": {false, false},
		"touch-left": {false, false}, "touch-top": {false, false}, "touch-right": {false, false}, "touch-bottom": {false, false}, "touch-corner": {false, false}, "disjoint": {false, false},
		"zero-width": {true, false}, "zero-height": {true, false}, "reversed-x": {true, false}, "reversed-y": {true, false}, "reversed-both": {true, false}, "empty-outside": {true, false},
		"nonempty-in-empty": {false, false}, "empty-in-empty": {true, false}, "empty-in-reversed": {true, false}, "nonempty-in-reversed": {false, false},
		"full-int64-equal": {true, true}, "min-int64-unit": {true, true}, "max-int64-unit": {true, true}, "reversed-int64": {true, false},
		"precision-contained": {true, true}, "precision-cross-min-x": {false, false}, "precision-cross-min-y": {false, false}, "precision-cross-max-x": {false, false}, "precision-cross-max-y": {false, false},
	}
	for _, pair := range containmentNamedPairs() {
		want, ok := wants[pair.Name]
		if !ok {
			t.Fatal("anchor lacks independent expected observation", pair.Name)
		}
		receiver, container := containmentRawRectangle(pair.Receiver), containmentRawRectangle(pair.Container)
		if got := rectangleInOracle([]image.Rectangle{receiver, container}); got != want[0] {
			t.Fatal(pair.Name, "In", got, want[0])
		}
		if got := fitsCropOracle([]image.Rectangle{container, receiver}); got != want[1] {
			t.Fatal(pair.Name, "FitsCrop", got, want[1])
		}
		delete(wants, pair.Name)
	}
	if len(wants) != 0 {
		t.Fatal("required semantic anchor missing", wants)
	}
}

func TestContainmentCoverageOriginalInputsAndSerializedBatches(t *testing.T) {
	added, err := containmentCorpus()
	if err != nil {
		t.Fatal(err)
	}
	baseline := geometryBinaryInputs()
	if len(added.Suites) != 2 || added.BaselineReferenceInputs != len(baseline) || len(baseline) != 486 {
		t.Fatal("baseline or two-suite scope changed")
	}
	if added.PortReferenceCases != len(added.Suites[0].Cases) || added.ConsumerReferenceCases != len(added.Suites[1].Cases) || added.AddedReferenceInputs != added.PortReferenceCases-len(baseline) {
		t.Fatal("derived reference counters differ")
	}
	if added.Suites[0].Package != "go/image" || added.Suites[0].Activity != "RectangleIn" || added.Suites[1].Package != fitsCropPackage || added.Suites[1].Activity != "FitsCrop" {
		t.Fatal("new suite identities or order")
	}
	newCases, newBatches := 0, 0
	for index, s := range added.Suites {
		seen := map[[2]rectangleCoordinates]bool{}
		for i, row := range s.Cases {
			if i < len(baseline) && !reflect.DeepEqual(row.Inputs, baseline[i]) {
				t.Fatal("original486 prefix changed", s.Activity, i)
			}
			var key [2]rectangleCoordinates
			for j, input := range row.Inputs {
				key[j], err = readRectangle(input)
				if err != nil {
					t.Fatal(err)
				}
			}
			if seen[key] {
				t.Fatal("duplicate original tuple", key)
			}
			seen[key] = true
			r := []image.Rectangle{containmentRawRectangle(key[0]), containmentRawRectangle(key[1])}
			want := rectangleInOracle(r)
			if index == 1 {
				want = fitsCropOracle(r)
			}
			if _, ok := row.Expected.(bool); !ok || !reflect.DeepEqual(row.Expected, want) {
				t.Fatal("non-Boolean or non-original Go expectation")
			}
		}
		require := func(a, b rectangleCoordinates) {
			t.Helper()
			if !seen[[2]rectangleCoordinates{a, b}] {
				t.Fatal("missing original tuple", s.Activity, a, b)
			}
		}
		for _, pair := range append(containmentNamedPairs(), containmentExamplePairs()...) {
			require(pair.Receiver, pair.Container)
			require(pair.Container, pair.Receiver)
		}
		for position := 0; position < 8; position++ {
			for _, value := range geometryBoundaryValues() {
				pair := [2]rectangleCoordinates{{3, 5, 11, 13}, {7, 9, 17, 19}}
				pair[position/4][position%4] = value
				require(pair[0], pair[1])
			}
		}
		limit := nativeBatchSize(s)
		wantedLimit := containmentPortBatchSize
		if index == 1 {
			wantedLimit = fitsCropBatchSize
		}
		if limit != wantedLimit {
			t.Fatal("containment allocator row limit", limit, wantedLimit)
		}
		batches, maximum := 0, 0
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, limit)
			if err != nil {
				t.Fatal(err)
			}
			if end != min(start+limit, len(s.Cases)) {
				t.Fatal("existing geometry binary record boundary changed")
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatal("actual record batch exceeds serialized byte cap", len(raw)+1, err)
			}
			var document struct {
				Cases []struct{ Inputs, Expected map[string]json.RawMessage }
			}
			if err = json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			key := s.Package + ":" + s.Activity
			for i, row := range document.Cases {
				if len(row.Inputs) != 2 || len(row.Expected) != 1 {
					t.Fatal("serialized record/result roster")
				}
				for j, input := range batch.Cases[i].Inputs {
					name := key + ".input0"
					if j == 1 {
						name = key + ".input1"
					}
					if !equalValue(row.Inputs[name], input) {
						t.Fatal("serialization changed raw original int64 record")
					}
				}
				if !equalValue(row.Expected[key], batch.Cases[i].Expected) {
					t.Fatal("serialized Boolean expectation changed")
				}
			}
			maximum = max(maximum, len(raw)+1)
			batches++
			start = end
		}
		newCases += len(s.Cases)
		newBatches += batches
		t.Logf("%s:%s cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maximum)
	}
	all := allNativeSuites(t)
	ports, consumerActivities := 0, 0
	programs := map[string]bool{}
	for _, s := range all {
		if strings.HasPrefix(s.Package, "go/") {
			ports++
		} else {
			programs[s.Package] = true
			consumerActivities++
		}
	}
	t.Logf("derived aggregate: suites=%d cases=%d batches=%d ports=%d consumer_programs=%d consumer_activities=%d", len(all), 130702+newCases, 1741+newBatches, ports, len(programs), consumerActivities)
}

func TestContainmentRejectsMalformedRecordsBeforeOriginalGoCalls(t *testing.T) {
	a, b := rectangleRecord(rectangleCoordinates{1, 2, 3, 4}), rectangleRecord(rectangleCoordinates{0, 0, 5, 6})
	malformed := []any{nil, "rectangle", map[string]any{"min_x": int64(0)}, map[string]any{"min_x": int64(0), "min_y": int64(0), "max_x": int64(1), "max_y": int64(1), "extra": int64(1)}}
	for _, name := range rectangleNames {
		for _, bad := range []any{nil, false, true, "0", 1.5, float64(9007199254740993), uint64(math.MaxInt64) + 1} {
			r := rectangleRecord(rectangleCoordinates{0, 0, 1, 1})
			r[name] = bad
			malformed = append(malformed, r)
		}
	}
	for _, operation := range []struct{ pkg, activity string }{{"go/image", "RectangleIn"}, {fitsCropPackage, "FitsCrop"}} {
		for position := 0; position < 2; position++ {
			for _, bad := range malformed {
				row := []any{a, b}
				row[position] = bad
				calls := 0
				_, err := geometrySuite(operation.pkg, operation.activity, 2, [][]any{{a, b}, row}, func([]image.Rectangle) any { calls++; return true })
				if err == nil || calls != 0 {
					t.Fatal("invalid record reached original oracle", operation.activity, position, calls)
				}
			}
		}
		calls := 0
		if _, err := geometrySuite(operation.pkg, operation.activity, 2, [][]any{{a, b}, {a, b}}, func([]image.Rectangle) any { calls++; return true }); err == nil || calls != 0 {
			t.Fatal("duplicate tuple reached oracle")
		}
	}
}

func TestContainmentStandalone14LiteralObservationsMatchOriginalGo(t *testing.T) {
	raw, err := os.ReadFile("../../examples/fits-crop/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decode(raw); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Schema string                       `json:"schema"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	anchors := containmentExamplePairs()
	wants := [][3]bool{
		{false, true, true}, {true, true, false}, {false, false, false}, {false, false, false},
		{false, false, false}, {false, false, false}, {false, false, false}, {true, true, false},
		{true, true, false}, {true, true, false}, {true, true, false}, {false, false, false},
		{false, true, true}, {false, false, false},
	}
	if document.Schema != "gooo/body-composition-inputs/v1" || len(document.Inputs) != 14 || len(anchors) != 14 || len(wants) != 14 {
		t.Fatal("standalone fixture scope changed")
	}
	readOriginal := func(raw json.RawMessage) rectangleCoordinates {
		t.Helper()
		value, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		fields, ok := value.(map[string]any)
		if !ok || len(fields) != 4 {
			t.Fatal("four raw record fields required")
		}
		var result rectangleCoordinates
		for i, name := range rectangleNames {
			number, ok := fields[name].(json.Number)
			if !ok {
				t.Fatal("original decimal integer required", name)
			}
			result[i], err = strconv.ParseInt(number.String(), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		if !equalValue(raw, rectangleRecord(result)) {
			t.Fatal("raw int64 serialization changed")
		}
		return result
	}
	selected := map[[2]rectangleCoordinates]bool{}
	for _, row := range containmentInputs() {
		a, _ := readRectangle(row[0])
		b, _ := readRectangle(row[1])
		selected[[2]rectangleCoordinates{a, b}] = true
	}
	for i, input := range document.Inputs {
		if len(input) != 2 {
			t.Fatal("standalone arity changed")
		}
		bounds := readOriginal(input[fitsCropPackage+":FitsCrop.input0"])
		requested := readOriginal(input[fitsCropPackage+":FitsCrop.input1"])
		if requested != anchors[i].Receiver || bounds != anchors[i].Container {
			t.Fatal("example and registered corpus anchor diverged", i)
		}
		if !selected[[2]rectangleCoordinates{requested, bounds}] || !selected[[2]rectangleCoordinates{bounds, requested}] {
			t.Fatal("standalone input missing from both tuple directions")
		}
		r, b := containmentRawRectangle(requested), containmentRawRectangle(bounds)
		actual := [3]bool{r.Empty(), r.In(b), fitsCropOracle([]image.Rectangle{b, r}).(bool)}
		if actual != wants[i] {
			t.Fatal("manual literal expectations disagree with original Go", i, actual, wants[i])
		}
		t.Logf("fixture%d %s: empty=%t In=%t FitsCrop=%t", i, anchors[i].Name, actual[0], actual[1], actual[2])
	}
}

func TestContainmentPreservesAllOriginal57CaseBytesAndBatchBoundaries(t *testing.T) {
	legacy := original57NativeSuites(t)
	all := allNativeSuites(t)
	if len(legacy) != 57 || len(all) != len(legacy)+2 || !reflect.DeepEqual(all[:len(legacy)], legacy) {
		t.Fatal("new suites must append after the original57 without changing indices")
	}
	var documents []any
	type batchRecord struct {
		Package  string `json:"package"`
		Activity string `json:"activity"`
		Start    int    `json:"start"`
		End      int    `json:"end"`
		Rows     int    `json:"rows"`
		Bytes    int    `json:"serialized_bytes"`
		SHA      string `json:"serialized_sha256"`
	}
	var batches []batchRecord
	hash := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	cases := 0
	for _, s := range legacy {
		documents = append(documents, caseDocument(s))
		cases += len(s.Cases)
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil {
				t.Fatal(err)
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			batches = append(batches, batchRecord{s.Package, s.Activity, start, end, end - start, len(raw), hash(raw)})
			start = end
		}
	}
	serialized, err := json.Marshal(documents)
	if err != nil {
		t.Fatal(err)
	}
	if cases != 130702 || hash(serialized) != "de0f08d3b4d6569fa6546833f192a117b18c763eee6fc137d2d98397ea27434e" {
		t.Fatal("original57 input/expectation bytes changed")
	}
	serialized, err = json.Marshal(batches)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1741 || hash(serialized) != "deca7076df082d6abcdb1cbde0bff5976912070a27d07839f035a5d99119fb97" {
		t.Fatal("original1741 batch boundaries/serialized bytes changed")
	}
	geometry, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(geometry.Suites) != 5 || geometry.PortReferenceCases != 1711 || geometry.ConsumerReferenceCases != 486 {
		t.Fatal("existing geometry work changed")
	}
}
