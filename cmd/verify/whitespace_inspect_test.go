package main

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestWhitespaceReceiptKeepsOriginalRuneAndRecordBindings(t *testing.T) {
	for _, n := range []int64{160, math.MinInt32, math.MaxInt32, -4294967264, 4294967328, -9007199254740993, 9007199254740993, math.MinInt64, math.MaxInt64} {
		for _, consumer := range []bool{false, true} {
			raw, _, identity := fixture(t)
			var original envelope
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			s := suite{Package: "go/unicode", Activity: "IsSpace"}
			want := any(false)
			if n == 160 {
				want = true
			}
			if consumer {
				s.Package, s.Activity = "app/whitespace-inspector", "Inspect"
				want = rejectedWhitespaceInspection()
				if n >= math.MinInt32 && n <= math.MaxInt32 {
					want = whitespaceInspectionOracle(rune(n))
				}
			}
			s.Cases = []testCase{{Inputs: []any{n}, Expected: want}}
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
			d.Input = mustJSON(t, n)
			d.Inputs = nil
			d.Actual = mustJSON(t, want)
			d.Expected = mustJSON(t, want)
			for _, field := range whitespaceResultContract(s).Fields {
				d.ActualFields = append(d.ActualFields, recordFieldValue{field.ID, field.Name, mustJSON(t, want.(map[string]any)[field.Name])})
			}
			valid := mustJSON(t, original)
			if _, err := inspect(valid, s, false, identity); err != nil {
				t.Fatalf("valid original %d consumer=%t: %v", n, consumer, err)
			}
			changes := map[string]func(*envelope){
				"wrong output type": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = "gooo://std/integer" },
				"wrong source binding": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("different"))
				},
				"wrong entry id": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ID = "gooo://other/inspect" },
				"string input": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, string(mustJSON(t, n)))
				},
				"rewritten actual and expectation": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Actual = mustJSON(t, "wrong")
					e.Result.Runtime.Traces[0].Deliveries[0].Expected = mustJSON(t, "wrong")
				},
			}
			if n < math.MinInt32 || n > math.MaxInt32 {
				changes["narrowed rune"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, int64(int32(n))) }
			}
			if !equalValue(mustJSON(t, float64(n)), n) {
				changes["rounded input"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Input = mustJSON(t, float64(n)) }
			}
			if consumer {
				changes["missing record fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
				changes["wrong record field id"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID = "gooo://other/field" }
				changes["wrong record field name"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].Name = "other" }
				changes["wrong record field value"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].Value = mustJSON(t, 0) }
				changes["swapped record fields"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] }
				changes["duplicate record field"] = func(e *envelope) {
					f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields
					e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = append(f, f[0])
				}
			} else {
				changes["scalar with record fields"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"unexpected", "unexpected", mustJSON(t, false)}}
				}
			}
			for name, change := range changes {
				var e envelope
				if err := json.Unmarshal(valid, &e); err != nil {
					t.Fatal(err)
				}
				change(&e)
				if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
					t.Fatalf("accepted %s n=%d consumer=%t", name, n, consumer)
				}
			}
		}
	}
}

func TestWhitespaceResultContractsMatchReviewedSource(t *testing.T) {
	for _, s := range []suite{{Package: "go/unicode", Activity: "IsSpace"}, {Package: "app/whitespace-inspector", Activity: "Inspect"}} {
		contract, err := contractFor(s)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../../" + contract.Source)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		result := whitespaceResultContract(s)
		if !strings.Contains(source, `id "`+result.Entity+`"`) {
			t.Fatal("missing reviewed output entity")
		}
		for i, field := range result.Fields {
			kind := "boolean"
			if i >= 4 {
				kind = "integer"
			}
			if !strings.Contains(source, `field `+field.Name+` id "`+field.ID+`" type `+kind+` required one`) {
				t.Fatal("record field contract differs from reviewed source", field)
			}
		}
	}
}
