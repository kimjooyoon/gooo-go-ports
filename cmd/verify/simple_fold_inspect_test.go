package main

import (
	"encoding/json"
	"testing"
)

func TestSimpleFoldReceiptsBindIntegerOutputsAndComparisonFields(t *testing.T) {
	added, err := simpleFoldCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		// The final row exercises the original int64 extension input. Its
		// expected value must not be rounded, narrowed, or changed with the receipt.
		s.Cases = s.Cases[len(s.Cases)-1:]
		raw, one, identity := contractFixture(t, s)
		changes := map[string]func(*envelope){
			"wrong output entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = "gooo://std/boolean" },
			"changed actual and expectation": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				d.Actual, d.Expected = mustJSON(t, 0), mustJSON(t, 0)
			},
		}
		if s.Package == "app/simple-fold" {
			changes["missing fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
			changes["wrong successor field identity"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[2].ID += "-wrong" }
			changes["string successor"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[2].Value = mustJSON(t, "0") }
			changes["swapped successor fields"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[2], f[3] = f[3], f[2] }
			changes["rewritten left input"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, 0) }
			changes["rewritten right input"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, 0) }
		} else {
			changes["scalar with fields"] = func(e *envelope) {
				e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"untrusted", "other", mustJSON(t, 0)}}
			}
			changes["rewritten input"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, 0) }
		}
		for name, change := range changes {
			t.Run(s.Activity+"/"+name, func(t *testing.T) {
				var e envelope
				if err := json.Unmarshal(raw, &e); err != nil {
					t.Fatal(err)
				}
				change(&e)
				if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
					t.Fatal("accepted corrupt SimpleFold receipt")
				}
			})
		}
	}
}
