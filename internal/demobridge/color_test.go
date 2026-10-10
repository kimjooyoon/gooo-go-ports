package demobridge

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRunColorChannelsRemainSeparateAndOrdered(t *testing.T) {
	for _, test := range []struct {
		name   string
		inputs []string
		kind   string
		want   map[string]any
	}{
		{"color.RGBToCMYK", []string{"0", "128", "255"}, "cmyk", map[string]any{"cyan": "1", "magenta": "127", "yellow": "128", "black": "255"}},
		{"color.CMYKToRGB", []string{"0", "1", "128", "255"}, "rgb", map[string]any{"red": "1", "green": "127", "blue": "128"}},
	} {
		calls := 0
		original := append([]string(nil), test.inputs...)
		r := readResponse(t, Run(test.name, test.inputs, func(name string, a, b, c, d int64) any {
			calls++
			if name != test.name {
				t.Fatal("operation changed")
			}
			values := []int64{a, b, c, d}
			for i, value := range values {
				if i < len(test.inputs) && strconv.FormatInt(value, 10) != test.inputs[i] || i >= len(test.inputs) && value != 0 {
					t.Fatalf("input %d changed: %v -> %v", i, test.inputs, values)
				}
			}
			return testReply{First: 1, Second: 127, Third: 128, Fourth: 255}
		}))
		if !r.OK || calls != 1 || !reflect.DeepEqual(r.Inputs, original) || !reflect.DeepEqual(test.inputs, original) || r.Output == nil || r.Output.Type != test.kind || !reflect.DeepEqual(r.Output.Value, test.want) {
			t.Fatalf("channel input/output changed: %+v", r)
		}
	}
}

func TestRunColorRejectsEveryBadChannelBeforeCompute(t *testing.T) {
	invalid := []string{"", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "0x10", "NaN", "１２", "-1", "256", "-256", "257", "4294967296", "9007199254740993", "-9223372036854775808", "9223372036854775807", "9223372036854775808", "-9223372036854775809"}
	for name, arity := range map[string]int{"color.RGBToCMYK": 3, "color.CMYKToRGB": 4} {
		for position := 0; position < arity; position++ {
			for _, raw := range invalid {
				inputs := []string{"12", "34", "56", "78"}[:arity]
				inputs[position] = raw
				rejectBeforeCompute(t, name, inputs)
			}
		}
		for length := 0; length <= 6; length++ {
			if length == arity {
				continue
			}
			inputs := make([]string, length)
			for i := range inputs {
				inputs[i] = "0"
			}
			rejectBeforeCompute(t, name, inputs)
		}
	}
}

func TestRunValidatesAllSixReplyFields(t *testing.T) {
	valid := `{"text":"","flag":false,"first":0,"second":0,"third":0,"fourth":0}`
	for _, field := range []string{"text", "flag", "first", "second", "third", "fourth"} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(valid), &fields); err != nil {
			t.Fatal(err)
		}
		delete(fields, field)
		missing, _ := json.Marshal(fields)
		duplicate := strings.TrimSuffix(valid, "}") + `,"` + field + `":0}`
		for _, raw := range []string{string(missing), duplicate} {
			r := readResponse(t, Run("color.CMYKToRGB", []string{"0", "0", "0", "0"}, func(string, int64, int64, int64, int64) any {
				return json.RawMessage(raw)
			}))
			if r.OK || r.Error == "" || r.Output != nil {
				t.Fatalf("accepted missing/duplicate %s: %+v", field, r)
			}
		}
	}
	for _, field := range []string{"first", "second", "third", "fourth"} {
		for _, raw := range []string{`"0"`, `null`, `{}`, `[]`, `1e2`, `1.0`, `-0`, `9223372036854775808`, `-9223372036854775809`} {
			bad := strings.Replace(valid, `"`+field+`":0`, `"`+field+`":`+raw, 1)
			r := readResponse(t, Run("http.StatusText", []string{"0", "0"}, func(string, int64, int64, int64, int64) any {
				return json.RawMessage(bad)
			}))
			if r.OK || r.Error == "" || r.Output != nil {
				t.Fatalf("accepted malformed numeric field %s=%s: %+v", field, raw, r)
			}
		}
	}
}

func TestRunRejectsOutOfDomainColorReplyChannels(t *testing.T) {
	for name, channels := range map[string]int{"color.RGBToCMYK": 4, "color.CMYKToRGB": 3} {
		inputs := []string{"0", "0", "0"}
		if name == "color.CMYKToRGB" {
			inputs = append(inputs, "0")
		}
		for position := 0; position < channels; position++ {
			for _, value := range []int64{-1, 256, -9223372036854775808, 9223372036854775807} {
				values := [4]int64{0, 1, 128, 255}
				values[position] = value
				calls := 0
				r := readResponse(t, Run(name, inputs, func(string, int64, int64, int64, int64) any {
					calls++
					return testReply{First: values[0], Second: values[1], Third: values[2], Fourth: values[3]}
				}))
				if r.OK || r.Error == "" || r.Output != nil || calls != 1 {
					t.Fatalf("accepted invalid result channel %d=%d: %+v", position, value, r)
				}
			}
		}
	}
}
