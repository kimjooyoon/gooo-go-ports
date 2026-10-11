package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func nativeInputFixtureFields(t *testing.T, entity string, value any) []recordFieldValue {
	t.Helper()
	if entity != nrgbaEntity && entity != nrgbaRGBEntity {
		return rectangleFixtureFields(t, entity, value)
	}
	names := nrgbaChannelNames
	if entity == nrgbaRGBEntity {
		names = nrgbaRGBNames
	}
	values, err := readNRGBARecord(value, names)
	if err != nil {
		t.Fatal(err)
	}
	var result []recordFieldValue
	for i, field := range nrgbaFieldContracts(entity) {
		result = append(result, recordFieldValue{field.ID, field.Name, mustJSON(t, values[i])})
	}
	return result
}

func TestNRGBAReceiptBindsExactRecordsAndOutputs(t *testing.T) {
	added, err := nrgbaCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		// Last input retains the precision-sensitive 2^53+1 boundary. Both
		// value objects and per-field metadata must preserve the original.
		s.Cases = s.Cases[len(s.Cases)-1:]
		raw, one, identity := contractFixture(t, s)
		changes := map[string]func(*envelope){
			"wrong output entity":   func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity },
			"missing output fields": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil },
			"output field ID":       func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID += "-wrong" },
			"output field order":    func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] },
			"correlated wrong output": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				v := map[string]any{}
				for k, n := range one.Cases[0].Expected.(map[string]any) {
					v[k] = n
				}
				f := &d.ActualFields[len(d.ActualFields)-1]
				v[f.Name] = 0
				f.Value = mustJSON(t, 0)
				d.Actual, d.Expected = mustJSON(t, v), mustJSON(t, v)
			},
		}
		for position := range one.Cases[0].Inputs {
			for label, mutate := range map[string]func([]recordFieldValue) []recordFieldValue{
				"missing fields": func(f []recordFieldValue) []recordFieldValue { return nil },
				"field ID":       func(f []recordFieldValue) []recordFieldValue { f[0].ID += "-wrong"; return f },
				"field name":     func(f []recordFieldValue) []recordFieldValue { f[0].Name = "unknown"; return f },
				"field order":    func(f []recordFieldValue) []recordFieldValue { f[0], f[1] = f[1], f[0]; return f },
				"field value":    func(f []recordFieldValue) []recordFieldValue { f[len(f)-1].Value = mustJSON(t, int64(1)); return f },
			} {
				t.Run(s.Activity+"/input"+string(rune('0'+position))+"/"+label, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					if len(one.Cases[0].Inputs) == 1 {
						d.InputFields = mutate(d.InputFields)
					} else {
						d.Inputs[position].Fields = mutate(d.Inputs[position].Fields)
					}
					if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
						t.Fatal("accepted corrupt original field metadata")
					}
				})
			}
			for _, rounded := range []int64{1, 9007199254740992} {
				t.Run(s.Activity+"/correlated original rewrite", func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					v := map[string]any{}
					for k, n := range one.Cases[0].Inputs[position].(map[string]any) {
						v[k] = n
					}
					names := nrgbaChannelNames
					if position == 1 {
						names = nrgbaRGBNames
					}
					v[names[len(names)-1]] = rounded
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					contract, _ := contractFor(one)
					fields := nativeInputFixtureFields(t, contract.InputEntities[position], v)
					if len(one.Cases[0].Inputs) == 1 {
						d.Input, d.InputFields = mustJSON(t, v), fields
					} else {
						d.Inputs[position].Value, d.Inputs[position].Fields = mustJSON(t, v), fields
					}
					if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
						t.Fatal("accepted rewritten original value and agreeing metadata")
					}
				})
			}
		}
		for name, change := range changes {
			t.Run(s.Activity+"/"+name, func(t *testing.T) {
				var e envelope
				if err := json.Unmarshal(raw, &e); err != nil {
					t.Fatal(err)
				}
				change(&e)
				if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
					t.Fatal("accepted corrupt NRGBA receipt")
				}
			})
		}
	}
}

func TestNRGBAReviewedRecordFieldDeclarations(t *testing.T) {
	for _, record := range []struct {
		path, entity string
		fields       []recordFieldContract
	}{
		{"ports/image/nrgba.gooo", nrgbaEntity, nrgbaFieldContracts(nrgbaEntity)},
		{"ports/image/nrgba.gooo", nrgbaRGBA16Entity, nrgbaFieldContracts(nrgbaRGBA16Entity)},
		{"ports/image/color.gooo", nrgbaRGBEntity, nrgbaFieldContracts(nrgbaRGBEntity)},
		{"examples/alpha-matte/main.gooo", nrgbaMatteEntity, nrgbaResultContract(suite{Package: "app/alpha-matte", Activity: "MattePixel"}).Fields},
	} {
		raw, err := os.ReadFile("../../" + record.path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		start := strings.Index(source, `id "`+record.entity+`" fields {`)
		if start < 0 {
			t.Fatal("record declaration", record.entity)
		}
		source = source[start:]
		end := strings.Index(source, "}")
		if end < 0 {
			t.Fatal("record block")
		}
		source = source[:end]
		previous := 0
		for _, field := range record.fields {
			kind := "integer"
			if field.Name == "in_byte_domain" {
				kind = "boolean"
			}
			declaration := `field ` + field.Name + ` id "` + field.ID + `" type ` + kind + ` required one`
			index := strings.Index(source, declaration)
			if index < previous {
				t.Fatal("reviewed field order/type/identity", record.entity, field.Name)
			}
			previous = index + len(declaration)
		}
		if strings.Count(source, "field ") != len(record.fields) {
			t.Fatal("unexpected record fields")
		}
	}
}
