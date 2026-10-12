package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Synthetic receipts exercise the existing finite verifier contract only.
// They do not add native observations or the input-only checker's stronger
// inner-result/generation/typed-plan coverage to this finite path.
func TestLiteralTextFiniteReceiptPreservesOriginalInputsAndResults(t *testing.T) {
	for _, pair := range []literalTextPair{{"가x가", "x"}, {"ababa", "aba"}, {"é🙂\x00", ""}, {"e\u0301", "é"}} {
		for _, entry := range []struct{ pkg, activity string }{{literalTextPackage, "Index"}, {literalTextPackage, "Count"}, {literalTextConsumerPackage, "Report"}} {
			var want any
			switch entry.activity {
			case "Index":
				want = strings.Index(pair.Text, pair.Needle)
			case "Count":
				want = strings.Count(pair.Text, pair.Needle)
			case "Report":
				want = literalTextReportOracle(pair.Text, pair.Needle)
			}
			s := suite{Package: entry.pkg, Activity: entry.activity, Cases: []testCase{{Inputs: []any{pair.Text, pair.Needle}, Expected: want}}}
			raw, one, id := contractFixture(t, s)
			mutations := map[string]func(*envelope){
				"swapped input values": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Inputs[0].Value, d.Inputs[1].Value = d.Inputs[1].Value, d.Inputs[0].Value
				},
				"input ports": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Port = "input1" },
				"normalized text": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, "other normalized input")
				},
				"normalized needle": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, "other normalized phrase")
				},
				"non-Text":    func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, 0) },
				"Text entity": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Entity = integerEntity },
				"text record metadata": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Fields = []recordFieldValue{{"wrong", "wrong", mustJSON(t, pair.Text)}}
				},
				"needle record metadata": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Fields = []recordFieldValue{{"wrong", "wrong", mustJSON(t, pair.Needle)}}
				},
				"entry": func(e *envelope) { e.Result.Program.Entry.Activity = "IndexByte" },
				"source": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("wrong source"))
				},
				"export":        func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].ID += "-wrong" },
				"output entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = textEntity },
				"compiler":      func(e *envelope) { e.Result.Runtime.Producer = strings.Repeat("f", 40) },
				"toolchain":     func(e *envelope) { e.Result.Runtime.GoVersion = "go version go1.0 linux/amd64" },
				"case digest":   func(e *envelope) { e.CasesDigest = contentDigest([]byte("other input file")) },
				"correlated wrong result": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Actual, d.Expected = mustJSON(t, "wrong"), mustJSON(t, "wrong")
				},
			}
			if entry.activity == "Report" {
				mutations["missing record fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
				mutations["field ID"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID += "-wrong" }
				mutations["field order"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] }
				mutations["field type"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[2].Value = mustJSON(t, "0") }
				mutations["rune offset or overlapping correlated count"] = func(e *envelope) {
					wrong := literalTextReportOracle(pair.Text, pair.Needle).(map[string]any)
					wrongOffset := strings.Index(pair.Text, pair.Needle) + 1
					wrongCount := strings.Count(pair.Text, pair.Needle) + 1
					if pair.Text == "가x가" && pair.Needle == "x" {
						wrongOffset = 1
					}
					if pair.Needle == "" {
						wrongCount = len(pair.Text) + 1
					}
					wrong["first_byte_offset"] = wrongOffset
					wrong["non_overlapping_count"] = wrongCount
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
					d.ActualFields[2].Value = mustJSON(t, wrongOffset)
					d.ActualFields[4].Value = mustJSON(t, wrongCount)
				}
			} else {
				mutations["correlated rune/overlap/byte-count result"] = func(e *envelope) {
					wrong := want.(int) + 1
					if entry.activity == "Index" && pair.Text == "가x가" && pair.Needle == "x" {
						wrong = 1
					}
					if entry.activity == "Count" && pair.Needle == "" {
						wrong = len(pair.Text) + 1
					}
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
				}
				mutations["scalar result record metadata"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"wrong", "wrong", mustJSON(t, 0)}}
				}
			}
			for name, change := range mutations {
				t.Run(entry.activity+"/"+name, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					change(&e)
					if _, err := inspect(mustJSON(t, e), one, false, id); err == nil {
						t.Fatal("altered finite evidence accepted")
					}
				})
			}
			original, err := inspect(raw, one, false, id)
			if err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(original, checked{Generated: original.Generated, ReplayedFrom: contentDigest([]byte("another receipt"))}, contentDigest(raw)); err == nil {
				t.Fatal("stale saved replay accepted")
			}
		}
	}
}

func TestLiteralTextReviewedReportFieldTypesIdentitiesAndOrder(t *testing.T) {
	s := suite{Package: literalTextConsumerPackage, Activity: "Report"}
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../" + contract.Source)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, `entity LiteralTextReport id "`+literalTextResultEntity+`" fields {`)
	if start < 0 {
		t.Fatal("missing reviewed report entity")
	}
	block := source[start:]
	end := strings.Index(block, "}")
	if end < 0 {
		t.Fatal("missing report declaration block")
	}
	block = block[:end]
	previous := 0
	fields := whitespaceResultContract(s).Fields
	for _, f := range fields {
		kind := "integer"
		if f.Name == "text" || f.Name == "needle" {
			kind = "string"
		}
		if f.Name == "found" {
			kind = "boolean"
		}
		declaration := `field ` + f.Name + ` id "` + f.ID + `" type ` + kind + ` required one`
		at := strings.Index(block, declaration)
		if at < previous {
			t.Fatal("reviewed record field identity/type/order", f)
		}
		previous = at + len(declaration)
	}
	if strings.Count(block, "field ") != 7 || len(fields) != 7 {
		t.Fatal("exact report field roster")
	}
}
