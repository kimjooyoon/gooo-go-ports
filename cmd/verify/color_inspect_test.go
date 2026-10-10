package main

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func TestColorReceiptInputAndRecordTampering(t *testing.T) {
	for _, arity := range []int{3, 4} {
		t.Run(fmt.Sprintf("arity%d", arity), func(t *testing.T) {
			raw, _, identity := fixture(t)
			var original envelope
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			s := suite{Package: "app/color-inspector", Activity: "InspectRGB"}
			inputs := []any{int64(12), int64(math.MaxInt64), int64(9007199254740993)}
			want := rejectedColorInspection()
			if arity == 4 {
				s.Package, s.Activity = "go/image/color", "CMYKToRGB"
				inputs, want = []any{int64(12), int64(34), int64(math.MaxInt64), int64(9007199254740993)}, rejectedRGB()
			}
			s.Cases = []testCase{{Inputs: inputs, Expected: want}}
			original.Result.Program.Entry.Package = s.Package
			original.Result.Program.Entry.Activity = s.Activity
			contract, err := contractFor(s)
			if err != nil {
				t.Fatal(err)
			}
			identity.Sources = fixtureSources(t, s)
			original.Result.Program.Image = fixtureWorkspace(t, s, identity.Sources)
			id := contract.ID
			original.Result.Composition.Steps[0].Generation.Report.ID = id
			delivery := &original.Result.Runtime.Traces[0].Deliveries[0]
			delivery.ID = id
			for len(delivery.Inputs) < arity {
				delivery.Inputs = append(delivery.Inputs, delivery.Inputs[0])
			}
			for i, input := range inputs {
				delivery.Inputs[i].Port = fmt.Sprintf("input%d", i)
				delivery.Inputs[i].Value = mustJSON(t, input)
			}
			delivery.Actual, delivery.Expected = mustJSON(t, want), mustJSON(t, want)
			valid := mustJSON(t, original)
			if _, err := inspect(valid, s, false, identity); err != nil {
				t.Fatalf("valid %d-input record: %v", arity, err)
			}
			mutations := map[string]func(*envelope){
				"missing input": func(e *envelope) { d := &e.Result.Runtime.Traces[0].Deliveries[0]; d.Inputs = d.Inputs[:arity-1] },
				"extra input": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Inputs = append(d.Inputs, d.Inputs[0])
				},
				"last inputs swapped": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Inputs[arity-2], d.Inputs[arity-1] = d.Inputs[arity-1], d.Inputs[arity-2]
				},
				"wrong last port": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[arity-1].Port = "input0" },
				"rounded wide input": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[arity-1].Value = json.RawMessage(`9007199254740992`)
				},
				"string input": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[arity-1].Value = json.RawMessage(`"9007199254740993"`)
				},
				"missing record field": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					var v map[string]any
					_ = json.Unmarshal(d.Actual, &v)
					delete(v, "red")
					d.Actual = mustJSON(t, v)
				},
				"extra record field": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					var v map[string]any
					_ = json.Unmarshal(d.Actual, &v)
					v["extra"] = 0
					d.Actual = mustJSON(t, v)
				},
				"wrong record type": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					var v map[string]any
					_ = json.Unmarshal(d.Actual, &v)
					v["red"] = "-1"
					d.Actual = mustJSON(t, v)
				},
				"wrong record value and expectation": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					var v map[string]any
					_ = json.Unmarshal(d.Actual, &v)
					v["red"] = 0
					d.Actual, d.Expected = mustJSON(t, v), mustJSON(t, v)
				},
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(valid, &e); err != nil {
						t.Fatal(err)
					}
					mutate(&e)
					if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
						t.Fatal("accepted changed input or record shape/value")
					}
				})
			}
		})
	}
}

func TestRecordComparisonKeepsExactIntegers(t *testing.T) {
	if equalValue(json.RawMessage(`{"red":9007199254740993}`), map[string]any{"red": int64(9007199254740992)}) {
		t.Fatal("rounded integer inside record")
	}
	if !equalValue(json.RawMessage(`{"red":9223372036854775807,"blue":-9223372036854775808}`), map[string]any{"red": int64(math.MaxInt64), "blue": int64(math.MinInt64)}) {
		t.Fatal("lost int64 endpoints inside record")
	}
}
