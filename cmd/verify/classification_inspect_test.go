package main

import (
	"encoding/json"
	"testing"
)

func TestClassificationReceiptsBindOutputAndFields(t *testing.T) {
	added, err := classificationCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		raw, one, identity := contractFixture(t, s)
		changes := map[string]func(*envelope){
			"wrong output entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity },
			"changed actual and expectation": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				d.Actual, d.Expected = mustJSON(t, "false"), mustJSON(t, "false")
			},
		}
		if s.Package == "app/identifier-rune" {
			changes["missing fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
			changes["changed field identity"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].ID += "-wrong" }
			changes["changed field value"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Value = mustJSON(t, 1) }
			changes["swapped fields"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[1], f[2] = f[2], f[1] }
		} else {
			changes["scalar with fields"] = func(e *envelope) {
				e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"untrusted", "other", mustJSON(t, false)}}
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
					t.Fatal("accepted corrupt classification receipt")
				}
			})
		}
	}
}
