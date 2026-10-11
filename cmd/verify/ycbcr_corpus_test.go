package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestYCbCrCorpusCoverageAndBatchBudget(t *testing.T) {
	added, err := ycbcrCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Suites) != 3 || added.PortReferenceCases != 2160 || added.ConsumerReferenceCases != 924 || added.PortRejectionCases != 48 || added.ConsumerRejectionCases != 24 {
		t.Fatal("unexpected YCbCr suite/rejection counts")
	}
	again, err := ycbcrCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("nondeterministic YCbCr corpus", err)
	}
	for index, s := range added.Suites {
		seen := map[string]bool{}
		for _, c := range s.Cases {
			key := string(mustJSON(t, c.Inputs))
			if seen[key] {
				t.Fatal("duplicate tuple", key)
			}
			seen[key] = true
		}
		for n := int64(0); n < 256; n++ {
			input := []any{n, n, n}
			if index == 1 {
				input = []any{n, int64(128), int64(128)}
			}
			if !seen[string(mustJSON(t, input))] {
				t.Fatalf("%s missing gray %d", s.Activity, n)
			}
		}
		rejection := rejectedYCbCr()
		if index == 1 {
			rejection = rejectedRGB()
		} else if index == 2 {
			rejection = rejectedYCbCrInspection()
		}
		for _, n := range colorRejectedInputs() {
			for position := 0; position < 3; position++ {
				input := []any{int64(12), int64(34), int64(56)}
				input[position] = n
				checkConsumerCase(t, s, input, rejection)
			}
		}
		size := ycbcrPortBatchSize
		if index == 2 {
			size = ycbcrInspectorBatchSize
		}
		maxBytes, batches := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatalf("%s exceeds case-document budget: %d, %v", s.Activity, len(raw)+1, err)
			}
			var doc struct {
				Cases []struct {
					Inputs   map[string]json.RawMessage `json:"inputs"`
					Expected map[string]json.RawMessage `json:"expected"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Cases) != len(batch.Cases) {
				t.Fatal("serialized case roster changed", err)
			}
			key := s.Package + ":" + s.Activity
			for i, c := range batch.Cases {
				if len(doc.Cases[i].Inputs) != 3 || len(doc.Cases[i].Expected) != 1 || !equalValue(doc.Cases[i].Expected[key], c.Expected) {
					t.Fatal("serialized YCbCr record changed")
				}
				for position, n := range c.Inputs {
					if !equalValue(doc.Cases[i].Inputs[fmt.Sprintf("%s.input%d", key, position)], n) {
						t.Fatal("serialized original input lost int64 precision")
					}
				}
			}
			maxBytes = max(maxBytes, len(raw)+1)
			batches++
		}
		t.Logf("%s:%s cases=%d batches=%d maxbytes=%d", s.Package, s.Activity, len(s.Cases), batches, maxBytes)
	}
	t.Logf("YCbCr reference ports=%d consumer=%d rejection ports=%d consumer=%d", added.PortReferenceCases, added.ConsumerReferenceCases, added.PortRejectionCases, added.ConsumerRejectionCases)
}

func TestYCbCrManualRoundingSaturationAndRoundTrip(t *testing.T) {
	added, err := ycbcrCorpus()
	if err != nil {
		t.Fatal(err)
	}
	// Literal expectations make channel swaps, rounding and saturation errors
	// visible independently of the shared oracle wiring.
	for _, row := range []struct{ input, want [3]int64 }{
		{[3]int64{0, 0, 0}, [3]int64{0, 128, 128}},
		{[3]int64{255, 255, 255}, [3]int64{255, 128, 128}},
		{[3]int64{255, 0, 0}, [3]int64{76, 85, 255}},
		{[3]int64{0, 255, 0}, [3]int64{150, 44, 21}},
		{[3]int64{0, 0, 255}, [3]int64{29, 255, 107}},
		{[3]int64{1, 0, 0}, [3]int64{0, 128, 129}},
		{[3]int64{0, 1, 0}, [3]int64{1, 128, 128}},
		{[3]int64{0, 0, 1}, [3]int64{0, 129, 128}},
	} {
		checkConsumerCase(t, added.Suites[0], []any{row.input[0], row.input[1], row.input[2]}, map[string]any{"y": row.want[0], "cb": row.want[1], "cr": row.want[2]})
	}
	for _, row := range []struct{ input, want [3]int64 }{
		{[3]int64{0, 128, 128}, [3]int64{0, 0, 0}},
		{[3]int64{255, 128, 128}, [3]int64{255, 255, 255}},
		{[3]int64{0, 0, 0}, [3]int64{0, 135, 0}},
		{[3]int64{255, 255, 255}, [3]int64{255, 121, 255}},
	} {
		checkConsumerCase(t, added.Suites[1], []any{row.input[0], row.input[1], row.input[2]}, map[string]any{"red": row.want[0], "green": row.want[1], "blue": row.want[2]})
	}
	for _, c := range added.Suites[2].Cases {
		v := c.Expected.(map[string]any)
		if v["in_byte_domain"] != true {
			continue
		}
		exact := true
		for i, field := range []string{"red", "green", "blue"} {
			delta := int64(v[field].(uint8)) - c.Inputs[i].(int64)
			if v[field+"_delta"] != delta {
				t.Fatal("delta direction or integer width changed")
			}
			exact = exact && delta == 0
		}
		if v["exact"] != exact {
			t.Fatal("round-trip exactness disagrees with differences")
		}
	}
}

func TestYCbCrPortableExampleInputsAndExpectedValues(t *testing.T) {
	manual := []struct {
		input, ycbcr, rgb, delta [3]int64
		exact                    bool
	}{
		{[3]int64{0, 0, 0}, [3]int64{0, 128, 128}, [3]int64{0, 0, 0}, [3]int64{0, 0, 0}, true},
		{[3]int64{255, 255, 255}, [3]int64{255, 128, 128}, [3]int64{255, 255, 255}, [3]int64{0, 0, 0}, true},
		{[3]int64{255, 0, 0}, [3]int64{76, 85, 255}, [3]int64{254, 0, 0}, [3]int64{-1, 0, 0}, false},
		{[3]int64{0, 255, 0}, [3]int64{150, 44, 21}, [3]int64{0, 255, 1}, [3]int64{0, 0, 1}, false},
		{[3]int64{0, 0, 255}, [3]int64{29, 255, 107}, [3]int64{0, 0, 254}, [3]int64{0, 0, -1}, false},
		{[3]int64{128, 128, 128}, [3]int64{128, 128, 128}, [3]int64{128, 128, 128}, [3]int64{0, 0, 0}, true},
		{[3]int64{128, 64, 32}, [3]int64{79, 101, 163}, [3]int64{128, 63, 31}, [3]int64{0, -1, -1}, false},
		{[3]int64{142, 83, 202}, [3]int64{114, 178, 148}, [3]int64{142, 82, 203}, [3]int64{0, -1, 1}, false},
		{[3]int64{202, 142, 83}, [3]int64{153, 88, 163}, [3]int64{202, 142, 82}, [3]int64{0, 0, -1}, false},
	}
	added, err := ycbcrCorpus()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../examples/ycbcr-inspector/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema string                       `json:"schema"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Schema != "gooo/body-composition-inputs/v1" || len(doc.Inputs) != 12 {
		t.Fatal("portable example input roster changed", err)
	}
	inputs := make([][3]int64, 0, 12)
	for _, row := range manual {
		inputs = append(inputs, row.input)
		checkConsumerCase(t, added.Suites[2], []any{row.input[0], row.input[1], row.input[2]}, map[string]any{
			"in_byte_domain": true, "y": row.ycbcr[0], "cb": row.ycbcr[1], "cr": row.ycbcr[2],
			"red": row.rgb[0], "green": row.rgb[1], "blue": row.rgb[2],
			"red_delta": row.delta[0], "green_delta": row.delta[1], "blue_delta": row.delta[2], "exact": row.exact,
		})
	}
	inputs = append(inputs, [3]int64{-1, 0, 0}, [3]int64{0, 256, 0}, [3]int64{0, 0, math.MaxInt64})
	for i, row := range inputs {
		if len(doc.Inputs[i]) != 3 {
			t.Fatal("portable input arity changed")
		}
		for position, n := range row {
			if !equalValue(doc.Inputs[i][fmt.Sprintf("app/ycbcr-inspector:InspectRGB.input%d", position)], n) {
				t.Fatal("portable input changed or lost precision", i, position)
			}
		}
		if i >= len(manual) {
			calls := 0
			_, err := byteTupleSuite("app/ycbcr-inspector", "InspectRGB", 3, [][]any{{row[0], row[1], row[2]}}, func([]uint8) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatal("out-of-domain portable sample reached Go conversion")
			}
		}
	}
}

func TestYCbCrSaturationBoundaryCoverage(t *testing.T) {
	seen := map[[3]int64]bool{}
	for _, row := range ycbcrInputs() {
		seen[[3]int64{row[0].(int64), row[1].(int64), row[2].(int64)}] = true
	}
	transitions := 0
	for _, cb := range []uint8{0, 1, 127, 128, 129, 254, 255} {
		for _, cr := range []uint8{0, 1, 127, 128, 129, 254, 255} {
			for y := 1; y < 256; y++ {
				r0, g0, b0 := color.YCbCrToRGB(uint8(y-1), cb, cr)
				r1, g1, b1 := color.YCbCrToRGB(uint8(y), cb, cr)
				for _, pair := range [][2]uint8{{r0, r1}, {g0, g1}, {b0, b1}} {
					if !(pair[0] == 0 && pair[1] > 0 || pair[0] < 255 && pair[1] == 255) {
						continue
					}
					transitions++
					for _, n := range []int{y - 1, y, min(y+1, 255)} {
						if !seen[[3]int64{int64(n), int64(cb), int64(cr)}] {
							t.Fatal("missing saturation neighbor", n, cb, cr)
						}
					}
				}
			}
		}
	}
	if transitions == 0 {
		t.Fatal("no saturation transitions checked")
	}
}

func ycbcrLegacyCorpus(t *testing.T) []suite {
	t.Helper()
	base := simpleFoldLegacyCorpus(t)
	folding, err := simpleFoldCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(base, folding.Suites...)
}

func TestYCbCrPreservesExisting31014Cases(t *testing.T) {
	before := ycbcrLegacyCorpus(t)
	count := 0
	for _, s := range before {
		count += len(s.Cases)
	}
	if count != 31014 {
		t.Fatal("legacy case count changed", count)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, before)))
	// Frozen from the unchanged 5bb78836 legacy generators before YCbCr integration.
	const frozen = "31470989904a37c42442fb7670c4552f30dedf6e0c24d1841c77b97740a17fa4"
	if hash != frozen {
		t.Fatalf("existing 31014-case corpus changed: %s", hash)
	}
	if _, err := ycbcrCorpus(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, ycbcrLegacyCorpus(t)) {
		t.Fatal("YCbCr construction mutated the existing corpus")
	}
}

func TestYCbCrReceiptBindingRejectsTampering(t *testing.T) {
	added, err := ycbcrCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		// Use precision-sensitive rejected inputs: preserving their original
		// values is independent of the all-minus-one output convention.
		s.Cases = s.Cases[len(s.Cases)-1:]
		raw, one, identity := contractFixture(t, s)
		mutations := map[string]func(*envelope){
			"wrong output entity": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = integerEntity },
			"wrong input entity":  func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[2].Entity = "gooo://std/text" },
			"narrowed byte alias": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[2].Value = mustJSON(t, 1) },
			"rounded wide integer": func(e *envelope) {
				e.Result.Runtime.Traces[0].Deliveries[0].Inputs[2].Value = mustJSON(t, int64(9007199254740992))
			},
			"missing fields":         func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil },
			"changed field identity": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].ID += "-wrong" },
			"changed field name":     func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Name = "unknown" },
			"changed field value":    func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[1].Value = mustJSON(t, 0) },
			"swapped fields":         func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[1], f[2] = f[2], f[1] },
			"correlated output": func(e *envelope) {
				d := &e.Result.Runtime.Traces[0].Deliveries[0]
				v := map[string]any{}
				for k, n := range one.Cases[0].Expected.(map[string]any) {
					v[k] = n
				}
				f := &d.ActualFields[1]
				v[f.Name] = 0
				f.Value = mustJSON(t, 0)
				d.Actual, d.Expected = mustJSON(t, v), mustJSON(t, v)
			},
		}
		for name, mutate := range mutations {
			t.Run(s.Package+":"+s.Activity+"/"+name, func(t *testing.T) {
				var e envelope
				if err := json.Unmarshal(raw, &e); err != nil {
					t.Fatal(err)
				}
				mutate(&e)
				if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
					t.Fatal("accepted corrupt YCbCr receipt")
				}
			})
		}
	}
}

func TestYCbCrOracleInputRosterRejectsBeforeConversion(t *testing.T) {
	valid := []any{int64(0), int64(128), int64(255)}
	for _, bad := range []any{int64(-1), int64(256), int64(math.MinInt64), int64(math.MaxInt64), int64(4294967296), int64(9007199254740993), uint8(1), float64(1), json.Number("1"), "1", nil} {
		for position := 0; position < 3; position++ {
			row := append([]any(nil), valid...)
			row[position] = bad
			calls := 0
			_, err := byteTupleSuite("go/image/color", "RGBToYCbCr", 3, [][]any{valid, row}, func([]uint8) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatal("invalid original tuple reached oracle", row, err, calls)
			}
		}
	}
}
