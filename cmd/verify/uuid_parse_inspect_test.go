package main

import (
	"encoding/json"
	"testing"
)

func TestUUIDParseReceiptRejectsPartialValueAndTypedFieldTampering(t *testing.T) {
	text := "00112233-4455-6677-8899-aabbccddee0g"
	want := uuidParseCanonicalOracle(text)
	s := suite{Package: uuidParsePackage, Activity: "ParseCanonical", Cases: []testCase{{Inputs: []any{text}, Expected: want}}}
	raw, one, identity := contractFixture(t, s)
	if _, err := inspect(raw, one, false, identity); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*envelope){
		"input normalized": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, text+" ") },
		"Text has record fields": func(e *envelope) {
			e.Result.Runtime.Traces[0].Deliveries[0].InputFields = []recordFieldValue{{"other", "other", mustJSON(t, text)}}
		},
		"result entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = textEntity },
		"source hash": func(e *envelope) {
			e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("altered"))
		},
		"source owner":   func(e *envelope) { e.Result.Program.Image.Packages[0].Path = uuidPackage },
		"producer":       func(e *envelope) { e.Result.Runtime.Producer = "altered" },
		"missing fields": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil },
		"extra field": func(e *envelope) {
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.ActualFields = append(d.ActualFields, recordFieldValue{"other", "other", mustJSON(t, 0)})
		},
		"field ID":          func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].ID += "-wrong" },
		"field name":        func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Name = "other" },
		"field order":       func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] },
		"parsed integer":    func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].Value = mustJSON(t, 0) },
		"canonical Boolean": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Value = mustJSON(t, false) },
		"error Boolean":     func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[2].Value = mustJSON(t, false) },
		"length integer":    func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[3].Value = mustJSON(t, 0) },
	}
	for field, wrong := range map[string]any{"parsed": true, "canonical_uuid": "00000000-0000-0000-0000-000000000000", "error": "", "invalid_length": true} {
		mutations["coordinated "+field] = func(e *envelope) {
			result := map[string]any{}
			for key, value := range want.(map[string]any) {
				result[key] = value
			}
			result[field] = wrong
			d := &e.Result.Runtime.Traces[0].Deliveries[0]
			d.Actual, d.Expected = mustJSON(t, result), mustJSON(t, result)
			for i := range d.ActualFields {
				if d.ActualFields[i].Name == field {
					d.ActualFields[i].Value = mustJSON(t, wrong)
				}
			}
		}
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var receipt envelope
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			mutate(&receipt)
			if _, err := inspect(mustJSON(t, receipt), one, false, identity); err == nil {
				t.Fatal("accepted tampered parser evidence")
			}
		})
	}
}

func TestUUIDParseSavedReplayBindsExactParentAndProgram(t *testing.T) {
	s := suite{Package: uuidParsePackage, Activity: "ParseCanonical", Cases: []testCase{{Inputs: []any{uuidStandardSample}, Expected: uuidParseCanonicalOracle(uuidStandardSample)}}}
	raw, one, identity := contractFixture(t, s)
	original, err := inspect(raw, one, false, identity)
	if err != nil {
		t.Fatal(err)
	}
	var receipt envelope
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	parent := contentDigest(raw)
	receipt.ReplayedFrom = parent
	receipt.Result.Replay = &struct {
		Calls *int `json:"model_calls"`
	}{Calls: new(int)}
	replayed, err := inspect(mustJSON(t, receipt), one, true, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectReplayBinding(original, replayed, parent); err != nil {
		t.Fatal(err)
	}
	if err := inspectReplayBinding(original, replayed, contentDigest([]byte("other parent"))); err == nil {
		t.Fatal("unrelated parent accepted")
	}
	replayed.Generated = contentDigest([]byte("other generated program"))
	if err := inspectReplayBinding(original, replayed, parent); err == nil {
		t.Fatal("altered generated program accepted")
	}
	receipt.Result.Replay.Calls = nil
	if _, err := inspect(mustJSON(t, receipt), one, true, identity); err == nil {
		t.Fatal("missing replay model counter accepted")
	}
}
