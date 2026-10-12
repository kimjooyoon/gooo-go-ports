package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUUIDResultContractsAndSourceFieldTypesAreIndependent(t *testing.T) {
	for _, row := range []struct {
		s                      suite
		entity                 string
		names, suffixes, kinds []string
	}{
		{suite{Package: uuidPackage, Activity: "Validate"}, uuidResultEntity,
			[]string{"valid", "error", "invalid_length"}, []string{"valid", "error", "invalid-length"}, []string{"boolean", "string", "boolean"}},
		{suite{Package: uuidConsumerPackage, Activity: "Inspect"}, uuidReportEntity,
			[]string{"valid", "error", "invalid_length", "byte_length"}, []string{"valid", "error", "invalid-length", "byte-length"}, []string{"boolean", "string", "boolean", "integer"}},
	} {
		if !isUUIDSuite(row.s) {
			t.Fatal("UUID suite missing")
		}
		result := uuidResultContract(row.s)
		if result.Entity != row.entity || len(result.Fields) != len(row.names) {
			t.Fatal("trusted result contract differs")
		}
		contract, err := contractFor(row.s)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(contract.InputEntities, []string{textEntity}) {
			t.Fatal("UUID original input is not Text")
		}
		raw, err := os.ReadFile("../../" + contract.Source)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		start := strings.Index(source, `id "`+row.entity+`" fields {`)
		if start < 0 {
			t.Fatal("reviewed result entity absent")
		}
		block := source[start:]
		end := strings.Index(block, "}")
		if end < 0 {
			t.Fatal("record block missing")
		}
		block = block[:end]
		previous := 0
		for i, name := range row.names {
			want := recordFieldContract{name, row.entity + "/" + row.suffixes[i]}
			if result.Fields[i] != want {
				t.Fatal("contract field ID/name/order differs")
			}
			declaration := `field ` + name + ` id "` + want.ID + `" type ` + row.kinds[i] + ` required one`
			position := strings.Index(block, declaration)
			if position < previous {
				t.Fatal("source field type/order differs", want)
			}
			previous = position + len(declaration)
		}
		if strings.Count(block, "field ") != len(row.names) {
			t.Fatal("unexpected source field")
		}
	}
	for _, s := range []suite{{Package: uuidPackage, Activity: "Parse"}, {Package: uuidConsumerPackage, Activity: "Validate"}, {Package: "other", Activity: "Inspect"}} {
		if isUUIDSuite(s) || uuidResultContract(s).Entity != "" {
			t.Fatal("unrelated suite accepted as UUID")
		}
	}
}

// Synthetic receipts exercise the checker against correlated changes; they are
// never counted as generated native executions or rewritten historical evidence.
func TestUUIDReceiptRejectsTextErrorClassAndFieldTampering(t *testing.T) {
	for _, text := range []string{uuidStandardSample, "", "[g0112233-4455-6677-8899-aabbccddeeff}", "abcdefgh\u00e9" + strings.Repeat("a", 35), strings.Repeat("x", 1024)} {
		for _, s := range []suite{{Package: uuidPackage, Activity: "Validate"}, {Package: uuidConsumerPackage, Activity: "Inspect"}} {
			want := uuidValidationOracle(text)
			if s.Package == uuidConsumerPackage {
				want = uuidInspectorOracle(text)
			}
			s.Cases = []testCase{{Inputs: []any{text}, Expected: want}}
			raw, one, identity := contractFixture(t, s)
			mutations := map[string]func(*envelope){
				"original Text changed":  func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, text+" ") },
				"integer replacing Text": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, int64(len(text))) },
				"Text scalar has record fields": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].InputFields = []recordFieldValue{{"other", "other", mustJSON(t, text)}}
				},
				"source changed": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("changed"))
				},
				"producer changed":      func(e *envelope) { e.Result.Runtime.Producer = "changed" },
				"result entity changed": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = textEntity },
				"missing fields":        func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil },
				"field identity":        func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID += "-wrong" },
				"field name":            func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].Name = "other" },
				"field order":           func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] },
				"Boolean as integer":    func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].Value = mustJSON(t, 1) },
				"error as Boolean":      func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Value = mustJSON(t, false) },
				"extra field": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.ActualFields = append(d.ActualFields, recordFieldValue{"other", "other", mustJSON(t, 0)})
				},
				"coordinated error class": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					wrong := map[string]any{}
					for k, value := range want.(map[string]any) {
						wrong[k] = value
					}
					wrong["invalid_length"] = !wrong["invalid_length"].(bool)
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
					d.ActualFields[2].Value = mustJSON(t, wrong["invalid_length"])
				},
				"coordinated error message": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					wrong := map[string]any{}
					for k, value := range want.(map[string]any) {
						wrong[k] = value
					}
					wrong["error"] = "wrong error"
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
					d.ActualFields[1].Value = mustJSON(t, wrong["error"])
				},
			}
			if s.Package == uuidConsumerPackage && utf8.RuneCountInString(text) != len(text) {
				mutations["rune count as byte length"] = func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					wrong := uuidInspectorOracle(text).(map[string]any)
					wrong["byte_length"] = int64(utf8.RuneCountInString(text))
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
					d.ActualFields[3].Value = mustJSON(t, wrong["byte_length"])
				}
			}
			for name, mutate := range mutations {
				t.Run(s.Package+"/"+name, func(t *testing.T) {
					var receipt envelope
					if err := json.Unmarshal(raw, &receipt); err != nil {
						t.Fatal(err)
					}
					mutate(&receipt)
					if _, err := inspect(mustJSON(t, receipt), one, false, identity); err == nil {
						t.Fatal("accepted rewritten UUID evidence")
					}
				})
			}
		}
	}
}

func TestUUIDSyntheticSavedReplayBindsExactParentAndProgram(t *testing.T) {
	for _, s := range []suite{{Package: uuidPackage, Activity: "Validate"}, {Package: uuidConsumerPackage, Activity: "Inspect"}} {
		want := uuidValidationOracle(uuidStandardSample)
		if s.Package == uuidConsumerPackage {
			want = uuidInspectorOracle(uuidStandardSample)
		}
		s.Cases = []testCase{{Inputs: []any{uuidStandardSample}, Expected: want}}
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
		if err := inspectReplayBinding(original, replayed, contentDigest([]byte("other receipt"))); err == nil {
			t.Fatal("wrong saved parent accepted")
		}
		receipt.Result.Replay.Calls = nil
		if _, err := inspect(mustJSON(t, receipt), one, true, identity); err == nil {
			t.Fatal("missing replay model counter accepted")
		}
	}
}
