package main

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestStringByteReceiptBindsOriginalTextNeedleAndByteOffsets(t *testing.T) {
	for _, needle := range []int64{169, 0, -1, 256, 9007199254740993, math.MinInt64, math.MaxInt64} {
		for _, consumer := range []bool{false, true} {
			s := suite{Package: stringBytePackage, Activity: "IndexByte"}
			text := "\x00é🙂"
			want := any(-1)
			if needle >= 0 && needle <= 255 {
				want = strings.IndexByte(text, byte(needle))
			}
			if consumer {
				s.Package, s.Activity = stringByteConsumerPackage, "Search"
				if needle >= 0 && needle <= 255 {
					want = stringByteSearchOracle(text, byte(needle))
				} else {
					want = map[string]any{"offset": -1, "found": false, "in_byte_domain": false, "byte_length": len(text)}
				}
			}
			s.Cases = []testCase{{Inputs: []any{text, needle}, Expected: want}}
			raw, one, identity := contractFixture(t, s)
			mutations := map[string]func(*envelope){
				"trimmed NUL Text": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, "é🙂") },
				"normalized Text": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, "\x00e\u0301🙂")
				},
				"non-Text input":      func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, 0) },
				"Text entity changed": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Entity = integerEntity },
				"Text record metadata": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Fields = []recordFieldValue{{"other", "other", mustJSON(t, text)}}
				},
				"string needle": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, string(mustJSON(t, needle)))
				},
				"changed source": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("other source"))
				},
				"rewritten output and expectation": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Actual, d.Expected = mustJSON(t, "wrong"), mustJSON(t, "wrong")
				},
			}
			if needle < 0 || needle > 255 {
				mutations["narrowed byte needle"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, int64(byte(needle)))
				}
			}
			if !equalValue(mustJSON(t, float64(needle)), needle) {
				mutations["rounded needle"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, float64(needle))
				}
			}
			if consumer {
				mutations["missing result fields"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil }
				mutations["changed field identity"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID += "-wrong" }
				mutations["swapped field order"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] }
				mutations["wrong record output entity"] = func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity }
			} else {
				mutations["scalar output has record fields"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"other", "other", mustJSON(t, -1)}}
				}
			}
			if needle == 169 {
				// The matching continuation byte is at byte offset 2 in
				// NUL+é+🙂. A rune-index implementation would report 1.
				mutations["correlated rune-position output"] = func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					wrong := any(1)
					if consumer {
						wrong = map[string]any{"offset": 1, "found": true, "in_byte_domain": true, "byte_length": 3}
						d.ActualFields[0].Value = mustJSON(t, 1)
						d.ActualFields[3].Value = mustJSON(t, 3)
					}
					d.Actual, d.Expected = mustJSON(t, wrong), mustJSON(t, wrong)
				}
			}
			for name, mutate := range mutations {
				t.Run(s.Package+"/"+name, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					mutate(&e)
					if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
						t.Fatal("accepted rewritten Text, needle, output or source")
					}
				})
			}
		}
	}
}

func TestStringByteReviewedOutputFieldTypesAndOrder(t *testing.T) {
	s := suite{Package: stringByteConsumerPackage, Activity: "Search"}
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../" + contract.Source)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, `entity SearchResult id "`+stringByteResultEntity+`" fields {`)
	if start < 0 {
		t.Fatal("reviewed result entity missing")
	}
	block := source[start:]
	end := strings.Index(block, "}")
	if end < 0 {
		t.Fatal("result record block missing")
	}
	block = block[:end]
	previous := 0
	fields := whitespaceResultContract(s).Fields
	for _, field := range fields {
		kind := "integer"
		if field.Name == "found" || field.Name == "in_byte_domain" {
			kind = "boolean"
		}
		declaration := `field ` + field.Name + ` id "` + field.ID + `" type ` + kind + ` required one`
		index := strings.Index(block, declaration)
		if index < previous {
			t.Fatal("reviewed record field identity/type/order differs", field)
		}
		previous = index + len(declaration)
	}
	if strings.Count(block, "field ") != len(fields) {
		t.Fatal("unexpected record field roster")
	}
}
