package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestNRGBACorpusCoverageBudgetAndBaseline(t *testing.T) {
	added, err := nrgbaCorpus()
	if err != nil {
		t.Fatal(err)
	}
	again, err := nrgbaCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("nondeterministic NRGBA corpus", err)
	}
	if len(added.Suites) != 2 || added.PortReferenceCases != 2696 || added.ConsumerReferenceCases != 2158 || added.PortRejectionCases != 32 || added.ConsumerRejectionCases != 104 {
		t.Fatal("NRGBA suite/rejection roster changed")
	}
	total, batches, maximum := 0, 0, 0
	for index, s := range added.Suites {
		seen := map[string]bool{}
		for _, c := range s.Cases {
			key := string(mustJSON(t, c.Inputs))
			if seen[key] {
				t.Fatal("duplicate NRGBA case", key)
			}
			seen[key] = true
		}
		for n := int64(0); n < 256; n++ {
			alpha := []any{nrgbaRecord(255, 1, 128, n)}
			if index == 1 {
				alpha = append(alpha, nrgbaRecord(17, 83, 202))
			}
			if !seen[string(mustJSON(t, alpha))] {
				t.Fatal("missing alpha ramp", s.Activity, n)
			}
			for position := 0; position < 3; position++ {
				fg := [4]int64{17, 83, 202, 128}
				fg[position] = n
				row := []any{nrgbaRecord(fg[:]...)}
				if index == 1 {
					row = append(row, nrgbaRecord(202, 83, 17))
				}
				if !seen[string(mustJSON(t, row))] {
					t.Fatal("missing independent foreground ramp", s.Activity, position, n)
				}
				if index == 1 {
					bg := [3]int64{202, 83, 17}
					bg[position] = n
					row = []any{nrgbaRecord(17, 83, 202, 127), nrgbaRecord(bg[:]...)}
					if !seen[string(mustJSON(t, row))] {
						t.Fatal("missing independent background ramp", position, n)
					}
				}
			}
		}
		for _, input := range nrgbaRejectedRecords(index == 1) {
			want := rejectedNRGBA()
			if index == 1 {
				want = rejectedNRGBAMatte()
			}
			checkConsumerCase(t, s, input, want)
		}
		size := nrgbaPortBatchSize
		if index == 1 {
			size = nrgbaConsumerBatchSize
		}
		maxBytes, count := 0, 0
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			if len(raw) > 32<<10 {
				t.Fatal("actual NRGBA saved batch exceeds 32KiB", s.Activity, len(raw))
			}
			var doc struct {
				Cases []struct {
					Inputs   map[string]json.RawMessage `json:"inputs"`
					Expected map[string]json.RawMessage `json:"expected"`
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Cases) != len(batch.Cases) {
				t.Fatal("saved NRGBA roster", err)
			}
			key := s.Package + ":" + s.Activity
			for i, c := range batch.Cases {
				if len(doc.Cases[i].Inputs) != len(c.Inputs) || len(doc.Cases[i].Expected) != 1 || !equalValue(doc.Cases[i].Expected[key], c.Expected) {
					t.Fatal("saved NRGBA case shape")
				}
				for j, input := range c.Inputs {
					name := key
					if len(c.Inputs) > 1 {
						name = fmt.Sprintf("%s.input%d", key, j)
					}
					if !equalValue(doc.Cases[i].Inputs[name], input) {
						t.Fatal("original record lost int64 precision")
					}
				}
			}
			var exact any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&exact); err != nil {
				t.Fatal(err)
			}
			roundtrip, err := json.MarshalIndent(exact, "", "  ")
			if err != nil || !bytes.Equal(append(roundtrip, '\n'), raw) {
				t.Fatal("saved NRGBA numeric tokens changed")
			}
			maxBytes = max(maxBytes, len(raw))
			count++
		}
		total += len(s.Cases)
		batches += count
		maximum = max(maximum, maxBytes)
		t.Logf("%s:%s cases=%d batches=%d maximum_bytes=%d", s.Package, s.Activity, len(s.Cases), count, maxBytes)
	}
	if total > 5000 {
		t.Fatal("finite NRGBA workload exceeds reviewed 5000-case budget", total)
	}
	all := allNativeSuites(t)
	var documents []any
	oldCases := 0
	if len(all) != 57 {
		t.Fatal("native suite roster")
	}
	for _, s := range all[:50] {
		documents = append(documents, caseDocument(s))
		oldCases += len(s.Cases)
	}
	// Frozen from main 47b47fe before adding either NRGBA suite.
	if oldCases != 57251 || contentDigest(mustJSON(t, documents)) != "sha256:195000573f21678fd7511c1f67b076b089dd654e1dc9888afd3dc67ef4fe0c67" {
		t.Fatal("existing 50 suites / 57251 cases changed")
	}
	t.Logf("NRGBA reference ports=%d consumer=%d rejection ports=%d consumer=%d added_cases=%d added_batches=%d maxbytes=%d aggregate_cases=%d aggregate_batches=%d", added.PortReferenceCases, added.ConsumerReferenceCases, added.PortRejectionCases, added.ConsumerRejectionCases, total, batches, maximum, 57251+total, 719+batches)
}

func TestNRGBAManualPremultiplicationAndMatteAnchors(t *testing.T) {
	for _, row := range []struct {
		input [4]uint8
		want  [4]int64
	}{
		{[4]uint8{255, 0, 0, 128}, [4]int64{32896, 0, 0, 32896}},
		{[4]uint8{12, 34, 56, 0}, [4]int64{0, 0, 0, 0}},
		{[4]uint8{12, 34, 56, 255}, [4]int64{3084, 8738, 14392, 65535}},
		{[4]uint8{255, 1, 128, 1}, [4]int64{257, 1, 129, 257}},
		{[4]uint8{1, 2, 3, 127}, [4]int64{127, 255, 383, 32639}},
	} {
		got, err := nrgbaRGBAOracle(row.input[:])
		if err != nil || !equalValue(mustJSON(t, got), nrgbaRecord(row.want[:]...)) {
			t.Fatal("manual premultiplication anchor", row, got, err)
		}
	}
	for _, row := range []struct {
		input         [7]uint8
		rgb           [3]int64
		premultiplied [4]int64
	}{
		{[7]uint8{255, 0, 0, 128, 255, 255, 255}, [3]int64{255, 127, 127}, [4]int64{32896, 0, 0, 32896}},
		{[7]uint8{255, 0, 0, 128, 0, 0, 255}, [3]int64{128, 0, 127}, [4]int64{32896, 0, 0, 32896}},
		{[7]uint8{12, 34, 56, 0, 17, 83, 202}, [3]int64{17, 83, 202}, [4]int64{0, 0, 0, 0}},
		{[7]uint8{12, 34, 56, 255, 17, 83, 202}, [3]int64{12, 34, 56}, [4]int64{3084, 8738, 14392, 65535}},
		{[7]uint8{1, 0, 0, 128, 0, 0, 0}, [3]int64{0, 0, 0}, [4]int64{129, 0, 0, 32896}},
	} {
		got, err := nrgbaMatteOracle(row.input[:])
		want := map[string]any{"in_byte_domain": true, "red": row.rgb[0], "green": row.rgb[1], "blue": row.rgb[2], "premultiplied_red": row.premultiplied[0], "premultiplied_green": row.premultiplied[1], "premultiplied_blue": row.premultiplied[2], "premultiplied_alpha": row.premultiplied[3]}
		if err != nil || !equalValue(mustJSON(t, got), want) {
			t.Fatal("manual draw.Over anchor", row, got, err)
		}
	}
}

func TestNRGBARecordRosterRejectsBeforeAnyConversion(t *testing.T) {
	for _, consumer := range []bool{false, true} {
		valid := []any{nrgbaRecord(0, 1, 254, 255)}
		if consumer {
			valid = append(valid, nrgbaRecord(12, 34, 56))
		}
		check := func(bad []any) {
			t.Helper()
			calls := 0
			_, err := nrgbaRecordSuite("test", "record", consumer, [][]any{valid, bad}, func([]uint8) (any, error) { calls++; return nil, nil })
			if err == nil || calls != 0 {
				t.Fatal("invalid original roster reached byte conversion/oracle", bad, calls, err)
			}
		}
		check(nil)
		check([]any{})
		check(append(append([]any(nil), valid...), nil))
		check(valid)
		for position := range valid {
			names := nrgbaChannelNames
			if position == 1 {
				names = nrgbaRGBNames
			}
			for _, field := range names {
				for _, bad := range []any{int64(-1), int64(256), int64(math.MinInt64), int64(math.MaxInt64), int64(4294967296), int64(9007199254740993), uint8(1), float64(1), json.Number("1"), "1", nil} {
					row := append([]any(nil), valid...)
					value := map[string]any{}
					for k, v := range valid[position].(map[string]any) {
						value[k] = v
					}
					value[field] = bad
					row[position] = value
					check(row)
				}
				row := append([]any(nil), valid...)
				value := map[string]any{}
				for k, v := range valid[position].(map[string]any) {
					value[k] = v
				}
				delete(value, field)
				value["unknown"] = int64(1)
				row[position] = value
				check(row)
			}
			for _, bad := range []any{nil, []int64{0, 1, 254, 255}, map[string]int64{"red": 0, "green": 1, "blue": 254, "alpha": 255}} {
				row := append([]any(nil), valid...)
				row[position] = bad
				check(row)
			}
		}
		calls := 0
		got, err := nrgbaRecordSuite("test", "record", consumer, [][]any{valid}, func(channels []uint8) (any, error) {
			calls++
			want := []uint8{0, 1, 254, 255}
			if consumer {
				want = append(want, 12, 34, 56)
			}
			if !reflect.DeepEqual(channels, want) {
				t.Fatal("record channel order", channels)
			}
			return nil, nil
		})
		if err != nil || calls != 1 {
			t.Fatal("valid endpoint tuple", err, calls)
		}
		valid[0].(map[string]any)["red"] = int64(99)
		if got.Cases[0].Inputs[0].(map[string]any)["red"] != int64(0) {
			t.Fatal("oracle input record aliases caller map")
		}
	}
}

func TestNRGBAPortableInputsRemainExactRecords(t *testing.T) {
	raw, err := os.ReadFile("../../examples/alpha-matte/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema string                       `json:"schema"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Schema != "gooo/body-composition-inputs/v1" || len(doc.Inputs) != 24 {
		t.Fatal("portable input document", err)
	}
	refs, rejections := 0, 0
	for _, input := range doc.Inputs {
		if len(input) != 2 {
			t.Fatal("portable input arity")
		}
		var originals []any
		var channels []uint8
		inDomain := true
		for position := 0; position < 2; position++ {
			var record map[string]int64
			if err := json.Unmarshal(input[fmt.Sprintf("app/alpha-matte:MattePixel.input%d", position)], &record); err != nil {
				t.Fatal("portable original precision", err)
			}
			names := nrgbaChannelNames
			if position == 1 {
				names = nrgbaRGBNames
			}
			if len(record) != len(names) {
				t.Fatal("portable original record arity")
			}
			values := map[string]any{}
			for _, name := range names {
				n, ok := record[name]
				if !ok {
					t.Fatal("portable field missing", name)
				}
				values[name] = n
				inDomain = inDomain && n >= 0 && n <= 255
			}
			originals = append(originals, values)
		}
		if !inDomain {
			rejections++
			calls := 0
			_, err := nrgbaRecordSuite("app/alpha-matte", "MattePixel", true, [][]any{originals}, func([]uint8) (any, error) { calls++; return nil, nil })
			if err == nil || calls != 0 {
				t.Fatal("portable rejected input reached Go narrowing")
			}
			continue
		}
		refs++
		for position, record := range originals {
			names := nrgbaChannelNames
			if position == 1 {
				names = nrgbaRGBNames
			}
			for _, name := range names {
				channels = append(channels, uint8(record.(map[string]any)[name].(int64)))
			}
		}
		if _, err := nrgbaMatteOracle(channels); err != nil {
			t.Fatal(err)
		}
	}
	if refs != 18 || rejections != 6 {
		t.Fatal("portable reference/rejection counts", refs, rejections)
	}
}
