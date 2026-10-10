package cropbridge

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRunPreservesOriginalRecordsAndExactOutput(t *testing.T) {
	inputs := []string{"-9223372036854775808", "9007199254740993", "9223372036854775807", "-9007199254740993", "9", "8", "7", "6"}
	wantInputs := append([]string(nil), inputs...)
	calls := 0
	raw := Run(inputs, func(a, b Rectangle) any {
		calls++
		if a != (Rectangle{-1 << 63, 9007199254740993, 1<<63 - 1, -9007199254740993}) || b != (Rectangle{9, 8, 7, 6}) {
			t.Fatal("input records narrowed, reordered or normalized", a, b)
		}
		inputs[0] = "0"
		return json.RawMessage(`{"min_x":-9223372036854775808,"min_y":9007199254740993,"max_x":9223372036854775807,"max_y":-9007199254740993,"has_pixels":false}`)
	})
	var got response
	if err := json.Unmarshal([]byte(raw), &got); err != nil || !got.OK || got.Error != "" || calls != 1 || !reflect.DeepEqual(got.Inputs, wantInputs) {
		t.Fatal(raw, err, calls)
	}
	if *got.Output != (Output{wantInputs[0], wantInputs[1], wantInputs[2], wantInputs[3], false}) {
		t.Fatal("output lost exact integer values", got.Output)
	}
}

func TestInvalidInputsNeverCallCompiledProgram(t *testing.T) {
	bad := []string{"", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "NaN", "0x10", "9223372036854775808", "-9223372036854775809"}
	for i := range 8 {
		for _, value := range bad {
			inputs := []string{"0", "0", "10", "10", "1", "1", "9", "9"}
			inputs[i] = value
			raw := Run(inputs, func(Rectangle, Rectangle) any { t.Fatal("invalid input reached compute"); return nil })
			if !strings.Contains(raw, `"ok":false`) {
				t.Fatal(raw)
			}
		}
	}
	for _, n := range []int{0, 7, 9} {
		if raw := Run(make([]string, n), func(Rectangle, Rectangle) any { t.Fatal("invalid arity reached compute"); return nil }); !strings.Contains(raw, `"ok":false`) {
			t.Fatal(raw)
		}
	}
}

func TestMalformedOutputAndPanicFailClosed(t *testing.T) {
	valid := `{"min_x":1,"min_y":2,"max_x":3,"max_y":4,"has_pixels":true}`
	bad := []string{"null", `[]`, valid + `{}`, strings.Replace(valid, `"min_x":1`, `"min_x":1,"min_x":1`, 1), strings.Replace(valid, `"min_x":1,`, "", 1), strings.Replace(valid, `"min_x":1`, `"extra":1`, 1)}
	for _, value := range []string{`null`, `true`, `"1"`, `1.0`, `1e0`, `9223372036854775808`, `-9223372036854775809`} {
		bad = append(bad, strings.Replace(valid, `"min_x":1`, `"min_x":`+value, 1))
	}
	bad = append(bad, strings.Replace(valid, `"has_pixels":true`, `"has_pixels":1`, 1))
	inputs := []string{"0", "0", "10", "10", "1", "1", "9", "9"}
	for _, raw := range bad {
		if _, err := DecodeOutput([]byte(raw)); err == nil {
			t.Fatal("accepted invalid record", raw)
		}
		if got := Run(inputs, func(Rectangle, Rectangle) any { return json.RawMessage(raw) }); !strings.Contains(got, `"ok":false`) {
			t.Fatal(got)
		}
	}
	if got := Run(inputs, nil); !strings.Contains(got, `"ok":false`) {
		t.Fatal(got)
	}
	if got := Run(inputs, func(Rectangle, Rectangle) any { panic("failed") }); !strings.Contains(got, `"ok":false`) {
		t.Fatal(got)
	}
}
