package main

import (
	"encoding/json"
	"testing"
)

func TestContainmentBooleanReceiptsRejectRewrittenValuesInputsAndIdentity(t *testing.T) {
	corpus, err := containmentCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range corpus.Suites {
		// Distinct original records include exact values beyond2^53 and int32.
		pair := containmentNamedPairs()[len(containmentNamedPairs())-4]
		rows := [][]any{{rectangleRecord(pair.Receiver), rectangleRecord(pair.Container)}}
		oracle := rectangleInOracle
		if s.Package == fitsCropPackage {
			oracle = fitsCropOracle
		}
		s, err = geometrySuite(s.Package, s.Activity, 2, rows, oracle)
		if err != nil {
			t.Fatal(err)
		}
		raw, one, identity := contractFixture(t, s)
		if result := geometryResultContract(s); result.Entity != "gooo://std/boolean" || len(result.Fields) != 0 {
			t.Fatal("trusted Boolean result contract missing")
		}
		changes := map[string]func(*envelope){
			"Boolean as number": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				d.Actual, d.Expected = mustJSON(t, 0), mustJSON(t, 0)
			},
			"coordinated Boolean lie": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				wrong := !one.Cases[0].Expected.(bool)
				d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
			},
			"stray output record field": func(e *envelope) {
				e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"other", "other", mustJSON(t, true)}}
			},
			"output entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity },
			"source digest": func(e *envelope) {
				e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("changed"))
			},
			"source owner": func(e *envelope) { e.Result.Program.Image.Packages[0].Path = "untrusted" },
			"entry":        func(e *envelope) { e.Result.Program.Entry.Activity = "Other" },
			"producer":     func(e *envelope) { e.Result.Runtime.Producer = "changed" },
			"all activity IDs": func(e *envelope) {
				e.Result.Program.Image.Packages[0].Exports[0].ID = "wrong"
				e.Result.Composition.Steps[0].Generation.Report.ID = "wrong"
				e.Result.Runtime.Traces[0].Deliveries[0].ID = "wrong"
			},
			"swapped records keeping ports": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				d.Inputs[0].Value, d.Inputs[1].Value = d.Inputs[1].Value, d.Inputs[0].Value
				d.Inputs[0].Fields, d.Inputs[1].Fields = d.Inputs[1].Fields, d.Inputs[0].Fields
			},
		}
		for position := 0; position < 2; position++ {
			original, err := readRectangle(one.Cases[0].Inputs[position])
			if err != nil {
				t.Fatal(err)
			}
			label := string(rune('0'+position)) + "/"
			changes[label+"input entity"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Entity = integerEntity }
			changes[label+"missing fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Fields = nil }
			changes[label+"field order"] = func(e *envelope) {
				fields := e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Fields
				fields[0], fields[1] = fields[1], fields[0]
			}
			for field, name := range rectangleNames {
				changes[label+name+"/field ID"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Fields[field].ID = "wrong"
				}
				changes[label+name+"/field name"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Fields[field].Name = "wrong"
				}
				changes[label+name+"/integer as string"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Fields[field].Value = mustJSON(t, "0")
				}
				if !equalValue(mustJSON(t, float64(original[field])), original[field]) {
					changes[label+name+"/float rounding"] = func(e *envelope) {
						d := &e.Result.Runtime.Traces[0].Deliveries[0]
						value := rectangleRecord(original)
						value[name] = float64(original[field])
						d.Inputs[position].Value = mustJSON(t, value)
						d.Inputs[position].Fields[field].Value = mustJSON(t, float64(original[field]))
					}
				}
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
					t.Fatal("accepted rewritten containment evidence")
				}
			})
		}
	}
}

func TestContainmentSavedReplayBindsOriginalReceiptProgramAndFields(t *testing.T) {
	corpus, err := containmentCorpus()
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
		if err = json.Unmarshal(raw, &e); err != nil {
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
		if err = inspectReplayBinding(original, replayed, contentDigest(raw)); err != nil {
			t.Fatal(err)
		}
		for _, changed := range []checked{{replayed.Generated, contentDigest([]byte("other receipt"))}, {contentDigest([]byte("other program")), replayed.ReplayedFrom}} {
			if inspectReplayBinding(original, changed, contentDigest(raw)) == nil {
				t.Fatal("accepted unbound replay")
			}
		}
		e.Result.Replay.Calls = nil
		if _, err = inspect(mustJSON(t, e), one, true, identity); err == nil {
			t.Fatal("missing replay model counter accepted")
		}
		e.Result.Replay.Calls = &zero
		e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Fields[0].ID = "wrong"
		if _, err = inspect(mustJSON(t, e), one, true, identity); err == nil {
			t.Fatal("saved replay skipped record field identity")
		}
	}
}
