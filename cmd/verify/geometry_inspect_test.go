package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

func rectangleFixtureFields(t *testing.T, entity string, value any) []recordFieldValue {
	t.Helper()
	if entity != rectangleEntity {
		return nil
	}
	r, err := readRectangle(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields []recordFieldValue
	for i, field := range rectangleFieldContracts(rectangleEntity) {
		fields = append(fields, recordFieldValue{field.ID, field.Name, mustJSON(t, r[i])})
	}
	return fields
}

func TestGeometryReceiptBindsEveryOriginalRecordAndField(t *testing.T) {
	corpus, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := canonCorpus()
	if err != nil {
		t.Fatal(err)
	}
	corpus.Suites = append(corpus.Suites, canonical.Suites...)
	for _, s := range corpus.Suites {
		// Use distinct, nonzero coordinates outside floating-point and int32 precision.
		// Union additionally exercises Go's asymmetric both-empty return value.
		for _, index := range []int{len(s.Cases) - 1, 0} {
			if index == 0 {
				for i, row := range s.Cases {
					r, _ := readRectangle(row.Inputs[0])
					if (len(row.Inputs) == 1 && r == geometryExtremeRectangles()[0]) || r == (rectangleCoordinates{9, 8, 2, 1}) {
						index = i
						break
					}
				}
			}
			s.Cases = append([]testCase{s.Cases[index]}, s.Cases...)
			raw, one, identity := contractFixture(t, s)
			changes := map[string]func(*envelope){
				"wrong output type": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity },
				"correlated output lie": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Actual, d.Expected = mustJSON(t, "wrong"), mustJSON(t, "wrong")
				},
			}
			arity := len(one.Cases[0].Inputs)
			for inputIndex := range arity {
				original, _ := readRectangle(one.Cases[0].Inputs[inputIndex])
				prefix := fmt.Sprintf("input%d/", inputIndex)
				changeFields := func(change func([]recordFieldValue) []recordFieldValue) func(*envelope) {
					return func(e *envelope) {
						d := &e.Result.Runtime.Traces[0].Deliveries[0]
						if arity == 1 {
							d.InputFields = change(d.InputFields)
						} else {
							d.Inputs[inputIndex].Fields = change(d.Inputs[inputIndex].Fields)
						}
					}
				}
				changes[prefix+"missing fields"] = changeFields(func([]recordFieldValue) []recordFieldValue { return nil })
				changes[prefix+"duplicate field"] = changeFields(func(f []recordFieldValue) []recordFieldValue { return append(f, f[0]) })
				changes[prefix+"swapped fields"] = changeFields(func(f []recordFieldValue) []recordFieldValue { f[0], f[1] = f[1], f[0]; return f })
				if original[0] > original[2] || original[1] > original[3] {
					changes[prefix+"silently normalized bounds"] = func(e *envelope) {
						normalized := rectangleCoordinates{min(original[0], original[2]), min(original[1], original[3]), max(original[0], original[2]), max(original[1], original[3])}
						value := rectangleRecord(normalized)
						fields := rectangleFixtureFields(t, rectangleEntity, value)
						d := &e.Result.Runtime.Traces[0].Deliveries[0]
						if arity == 1 {
							d.Input, d.InputFields = mustJSON(t, value), fields
						} else {
							d.Inputs[inputIndex].Value, d.Inputs[inputIndex].Fields = mustJSON(t, value), fields
						}
					}
				}
				for fieldIndex, name := range rectangleNames {
					changes[prefix+name+"/wrong ID"] = changeFields(func(f []recordFieldValue) []recordFieldValue { f[fieldIndex].ID = "gooo://untrusted/field"; return f })
					changes[prefix+name+"/wrong name"] = changeFields(func(f []recordFieldValue) []recordFieldValue { f[fieldIndex].Name = "untrusted"; return f })
					changes[prefix+name+"/wrong value"] = changeFields(func(f []recordFieldValue) []recordFieldValue { f[fieldIndex].Value = mustJSON(t, "0"); return f })
					mutated := original
					mutated[fieldIndex] ^= 1
					changes[prefix+name+"/correlated original rewrite"] = func(e *envelope) {
						d := &e.Result.Runtime.Traces[0].Deliveries[0]
						value := rectangleRecord(mutated)
						fields := rectangleFixtureFields(t, rectangleEntity, value)
						if arity == 1 {
							d.Input, d.InputFields = mustJSON(t, value), fields
						} else {
							d.Inputs[inputIndex].Value, d.Inputs[inputIndex].Fields = mustJSON(t, value), fields
						}
					}
					for label, rewritten := range map[string]any{"int32 narrowing": int64(int32(original[fieldIndex])), "floating-point rounding": float64(original[fieldIndex])} {
						if equalValue(mustJSON(t, rewritten), original[fieldIndex]) {
							continue
						}
						changes[prefix+name+"/"+label] = func(e *envelope) {
							d := &e.Result.Runtime.Traces[0].Deliveries[0]
							value := rectangleRecord(original)
							value[name] = rewritten
							if arity == 1 {
								d.Input = mustJSON(t, value)
								d.InputFields[fieldIndex].Value = mustJSON(t, rewritten)
							} else {
								d.Inputs[inputIndex].Value = mustJSON(t, value)
								d.Inputs[inputIndex].Fields[fieldIndex].Value = mustJSON(t, rewritten)
							}
						}
					}
				}
			}
			if arity == 2 {
				changes["swapped input records retaining ports"] = func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Inputs[0].Value, d.Inputs[1].Value = d.Inputs[1].Value, d.Inputs[0].Value
					d.Inputs[0].Fields, d.Inputs[1].Fields = d.Inputs[1].Fields, d.Inputs[0].Fields
				}
				changes["stray unary fields"] = func(e *envelope) { d := &e.Result.Runtime.Traces[0].Deliveries[0]; d.InputFields = d.Inputs[0].Fields }
			}
			result := geometryResultContract(one)
			if len(result.Fields) > 0 {
				if one.Activity == "RectangleUnion" || one.Activity == "CropPlan" || one.Activity == "DragCropPlan" {
					changes["correlated raw result and field rewrite"] = func(e *envelope) {
						wrong := rectangleRecord(rectangleCoordinates{})
						if one.Activity == "CropPlan" || one.Activity == "DragCropPlan" {
							for name, value := range one.Cases[0].Expected.(map[string]any) {
								wrong[name] = value
							}
							wrong["has_pixels"] = !wrong["has_pixels"].(bool)
						}
						d := &e.Result.Runtime.Traces[0].Deliveries[0]
						d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
						for i, field := range result.Fields {
							d.ActualFields[i].Value = mustJSON(t, wrong[field.Name])
						}
					}
				}
				changes["missing result fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
				changes["swapped result fields"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] }
				changes["extra result field"] = func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.ActualFields = append(d.ActualFields, d.ActualFields[0])
				}
				for i, field := range result.Fields {
					changes[field.Name+"/wrong result ID"] = func(e *envelope) {
						e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[i].ID = "gooo://untrusted/field"
					}
					changes[field.Name+"/wrong result name"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[i].Name = "untrusted" }
					changes[field.Name+"/wrong result field value"] = func(e *envelope) {
						e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[i].Value = mustJSON(t, "wrong")
					}
				}
			}
			for name, change := range changes {
				t.Run(fmt.Sprintf("%s/%d/%s", one.Activity, index, name), func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					change(&e)
					if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
						t.Fatal("accepted changed Rectangle evidence")
					}
				})
			}
		}
	}
}

func TestGeometrySavedReplayRetainsInputsAndReceiptBinding(t *testing.T) {
	corpus, err := geometryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range corpus.Suites {
		s.Cases = s.Cases[len(s.Cases)-1:]
		raw, one, identity := contractFixture(t, s)
		original, err := inspect(raw, one, false, identity)
		if err != nil {
			t.Fatal(err)
		}
		var e envelope
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		zero := 0
		e.Result.Replay = &struct {
			Calls *int `json:"model_calls"`
		}{&zero}
		e.ReplayedFrom = contentDigest(raw)
		replayed, err := inspect(mustJSON(t, e), one, true, identity)
		if err != nil {
			t.Fatal(err)
		}
		if err := inspectReplayBinding(original, replayed, contentDigest(raw)); err != nil {
			t.Fatal(err)
		}
		for name, changed := range map[string]checked{
			"unrelated receipt":   {replayed.Generated, contentDigest([]byte("another receipt"))},
			"regenerated program": {contentDigest([]byte("another program")), replayed.ReplayedFrom},
		} {
			if err := inspectReplayBinding(original, changed, contentDigest(raw)); err == nil {
				t.Fatal("accepted", name)
			}
		}
		e.Result.Replay.Calls = nil
		if _, err := inspect(mustJSON(t, e), one, true, identity); err == nil {
			t.Fatal("accepted absent replay model calls")
		}
		oneCall := 1
		e.Result.Replay.Calls = &oneCall
		if _, err := inspect(mustJSON(t, e), one, true, identity); err == nil {
			t.Fatal("accepted new replay model call")
		}
		e.Result.Replay.Calls = &zero
		d := &e.Result.Runtime.Traces[0].Deliveries[0]
		if len(one.Cases[0].Inputs) == 1 {
			d.InputFields[0].Value = mustJSON(t, int64(math.MinInt64))
		} else {
			d.Inputs[1].Fields[0].Value = mustJSON(t, int64(math.MinInt64))
		}
		if _, err := inspect(mustJSON(t, e), one, true, identity); err == nil {
			t.Fatal("saved replay skipped original record fields")
		}
	}
}

func TestGeometryFieldContractsMatchReviewedSources(t *testing.T) {
	for _, record := range []struct {
		path, entity string
		fields       []recordFieldContract
	}{
		{"ports/image/geometry.gooo", rectangleEntity, rectangleFieldContracts(rectangleEntity)},
		{"examples/crop-planner/crop_planner.gooo", cropPlanEntity, geometryResultContract(suite{Package: "example/crop-planner", Activity: "CropPlan"}).Fields},
	} {
		raw, err := os.ReadFile("../../" + record.path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		if !strings.Contains(source, `id "`+record.entity+`" fields {`) {
			t.Fatal("missing record entity", record.path)
		}
		previous := -1
		for _, field := range record.fields {
			kind := "integer"
			if field.Name == "has_pixels" {
				kind = "boolean"
			}
			declaration := `field ` + field.Name + ` id "` + field.ID + `" type ` + kind + ` required one`
			index := strings.Index(source, declaration)
			if index <= previous {
				t.Fatal("record field declaration/order differs", record.path, field)
			}
			previous = index
		}
	}
}
