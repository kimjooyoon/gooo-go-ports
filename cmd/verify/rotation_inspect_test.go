package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestRotationReceiptKeepsOriginalSignedRotation(t *testing.T) {
	for _, k := range []int64{math.MinInt64, math.MaxInt64, -9007199254740993, 9007199254740993} {
		for _, consumer := range []bool{false, true} {
			raw, _, identity := fixture(t)
			var original envelope
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			s := suite{Package: "go/math/bits", Activity: "RotateLeft8"}
			want := inspectRotationOracle(128, int(k)).(map[string]any)["rotated"]
			if consumer {
				s.Package, s.Activity = "app/byte-rotation", "Inspect"
				want = inspectRotationOracle(128, int(k))
			}
			s.Cases = []testCase{{Inputs: []any{int64(128), k}, Expected: want}}
			contract, err := contractFor(s)
			if err != nil {
				t.Fatal(err)
			}
			identity.Sources = fixtureSources(t, s)
			original.Result.Program.Entry.Package, original.Result.Program.Entry.Activity = s.Package, s.Activity
			original.Result.Program.Image = fixtureWorkspace(t, s, identity.Sources)
			original.Result.Composition.Steps[0].Generation.Report.ID = contract.ID
			d := &original.Result.Runtime.Traces[0].Deliveries[0]
			d.ID = contract.ID
			d.Inputs[0].Value, d.Inputs[1].Value = mustJSON(t, int64(128)), mustJSON(t, k)
			d.Actual, d.Expected = mustJSON(t, want), mustJSON(t, want)
			valid := mustJSON(t, original)
			if _, err := inspect(valid, s, false, identity); err != nil {
				t.Fatalf("valid exact rotation %d consumer=%t: %v", k, consumer, err)
			}
			for name, changed := range map[string]json.RawMessage{
				// All four counts differ from their normalization, even when
				// the rotation result itself would be exactly the same.
				"normalized": mustJSON(t, int64(uint64(k)&7)),
				"rounded":    mustJSON(t, float64(k)),
				"string":     mustJSON(t, string(mustJSON(t, k))),
			} {
				var e envelope
				if err := json.Unmarshal(valid, &e); err != nil {
					t.Fatal(err)
				}
				e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = changed
				// MinInt64 is exactly representable by float64; its JSON
				// encoder still prints a different integer. Verify that the
				// mutation is distinct instead of assuming precision loss.
				if equalValue(changed, k) {
					t.Fatalf("mutation %s did not change original input %d", name, k)
				}
				if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
					t.Fatalf("accepted %s rotation despite original %d", name, k)
				}
			}
			if consumer {
				var e envelope
				if err := json.Unmarshal(valid, &e); err != nil {
					t.Fatal(err)
				}
				v := inspectRotationOracle(128, int(k)).(map[string]any)
				v["restored"] = 0
				e.Result.Runtime.Traces[0].Deliveries[0].Actual = mustJSON(t, v)
				e.Result.Runtime.Traces[0].Deliveries[0].Expected = mustJSON(t, v)
				if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
					t.Fatal("accepted changed consumer result and expectation")
				}
			}
		}
	}
}
