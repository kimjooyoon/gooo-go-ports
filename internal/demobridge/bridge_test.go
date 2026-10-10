package demobridge

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"testing"
)

type testReply struct {
	Text   string `json:"text"`
	First  int64  `json:"first"`
	Flag   bool   `json:"flag"`
	Second int64  `json:"second"`
	Third  int64  `json:"third"`
	Fourth int64  `json:"fourth"`
}

func readResponse(t *testing.T, raw string) response {
	t.Helper()
	var r response
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err, raw)
	}
	return r
}

func TestRunPreservesInt64AtBothBoundaries(t *testing.T) {
	for _, n := range []int64{math.MinInt64, -9007199254740993, 9007199254740992, 9007199254740993, math.MaxInt64} {
		a := strconv.FormatInt(n, 10)
		calls := 0
		r := readResponse(t, Run("http.StatusText", []string{a, "0"}, func(op string, first, second, third, fourth int64) any {
			calls++
			if op != "http.StatusText" || first != n || second != 0 || third != 0 || fourth != 0 {
				t.Fatalf("input changed: %s %d %d %d %d", op, first, second, third, fourth)
			}
			return testReply{Text: strconv.FormatInt(first, 10)}
		}))
		if !r.OK || calls != 1 || r.Operation != "http.StatusText" || len(r.Inputs) != 2 || r.Inputs[0] != a || r.Inputs[1] != "0" || r.Output == nil || r.Output.Type != "text" || r.Output.Value != a {
			t.Fatalf("lost exact input: %+v, calls=%d", r, calls)
		}
		r = readResponse(t, Run("utf16.DecodeRune", []string{"0", "0"}, func(string, int64, int64, int64, int64) any {
			return testReply{First: n}
		}))
		if !r.OK || r.Output == nil || r.Output.Type != "integer" || r.Output.Value != a {
			t.Fatalf("lost exact numeric reply: %+v", r)
		}
	}
}

func TestRunRejectsInvalidInputsWithoutComputation(t *testing.T) {
	tests := [][3]string{{"missing", "0", "0"}, {"http.StatusText", "0", "1"}, {"utf16.DecodeRune", "0", "2147483648"}}
	for _, raw := range []string{"", " ", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "0x10", "NaN", "１２", "9223372036854775808", "-9223372036854775809"} {
		tests = append(tests, [3]string{"http.StatusText", raw, "0"}, [3]string{"utf16.DecodeRune", "0", raw})
	}
	for _, op := range []string{"utf8.ValidRune", "utf8.RuneLen", "utf16.IsSurrogate", "utf16.RuneLen", "utf16.DecodeRune", "utf16.EncodeRune"} {
		for _, raw := range []string{"-2147483649", "2147483648"} {
			tests = append(tests, [3]string{op, raw, "0"})
		}
	}
	for _, op := range []string{"utf8.RuneStart", "bits.Len8", "bits.LeadingZeros8", "bits.TrailingZeros8", "bits.OnesCount8", "bits.Reverse8"} {
		for _, raw := range []string{"-1", "256", "9223372036854775807"} {
			tests = append(tests, [3]string{op, raw, "0"})
		}
	}
	for _, test := range tests {
		calls := 0
		r := readResponse(t, Run(test[0], []string{test[1], test[2]}, func(string, int64, int64, int64, int64) any { calls++; return testReply{} }))
		if r.OK || r.Error == "" || r.Output != nil || calls != 0 {
			t.Fatalf("invalid input reached computation: %v, calls=%d, result=%+v", test, calls, r)
		}
	}
}

func TestRunAllLegacyOperationsAndOutputKinds(t *testing.T) {
	tests := []struct{ name, a, b, kind string }{
		{"http.StatusText", "404", "0", "text"},
		{"utf8.ValidRune", "-2147483648", "0", "boolean"},
		{"utf8.RuneLen", "2147483647", "0", "integer"},
		{"utf8.RuneStart", "255", "0", "boolean"},
		{"utf16.IsSurrogate", "55296", "0", "boolean"},
		{"utf16.RuneLen", "128512", "0", "integer"},
		{"utf16.DecodeRune", "-2147483648", "2147483647", "integer"},
		{"utf16.EncodeRune", "128512", "0", "pair"},
		{"bits.Len8", "0", "0", "integer"},
		{"bits.LeadingZeros8", "1", "0", "integer"},
		{"bits.TrailingZeros8", "128", "0", "integer"},
		{"bits.OnesCount8", "170", "0", "integer"},
		{"bits.Reverse8", "255", "0", "integer"},
	}
	for _, test := range tests {
		calls := 0
		r := readResponse(t, Run(test.name, []string{test.a, test.b}, func(name string, a, b, c, d int64) any {
			calls++
			if name != test.name || strconv.FormatInt(a, 10) != test.a || strconv.FormatInt(b, 10) != test.b || c != 0 || d != 0 {
				t.Fatal("legacy argument order or padding changed")
			}
			return testReply{Text: "Not Found", First: 55357, Flag: true, Second: 56832}
		}))
		if !r.OK || calls != 1 || r.Output == nil || r.Output.Type != test.kind {
			t.Fatalf("%s: %+v", test.name, r)
		}
		if test.kind == "boolean" && r.Output.Value != true {
			t.Fatalf("boolean became a different type: %+v", r.Output)
		}
		if test.kind == "pair" {
			pair, ok := r.Output.Value.(map[string]any)
			if !ok || len(pair) != 2 || pair["high"] != "55357" || pair["low"] != "56832" {
				t.Fatalf("pair lost string-valued integers: %+v", r.Output)
			}
		}
		for _, inputs := range [][]string{nil, {}, {test.a}, {test.a, test.b, "0"}, {test.a, test.b, "0", "0"}} {
			rejectBeforeCompute(t, test.name, inputs)
		}
		if test.name != "utf16.DecodeRune" {
			r = readResponse(t, Run(test.name, []string{test.a, "1"}, func(string, int64, int64, int64, int64) any { t.Fatal("unexpected second input"); return nil }))
			if r.OK {
				t.Fatal("accepted unused nonzero input")
			}
		}
	}
}

func TestRunRejectsMalformedCallbackReplies(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"text":"","first":0,"flag":false,"second":0}`,
		`{"text":"","first":0,"flag":false,"second":0,"third":0,"fourth":0,"extra":1}`,
		`{"text":"","first":0,"flag":false,"second":0,"third":0,"fourth":0,"first":1}`,
		`{"text":null,"first":0,"flag":false,"second":0,"third":0,"fourth":0}`,
		`{"text":"","first":"1","flag":false,"second":0,"third":0,"fourth":0}`,
		`{"text":"","first":1.0,"flag":false,"second":0,"third":0,"fourth":0}`,
		`{"text":"","first":9223372036854775808,"flag":false,"second":0,"third":0,"fourth":0}`,
		`{"text":"","first":0,"flag":0,"second":0,"third":0,"fourth":0}`,
		`{"text":"","first":0,"flag":false,"second":{},"third":0,"fourth":0}`,
		`{"text":"","first":0,"flag":false,"second":0,"third":0,"fourth":0} {}`,
	} {
		calls := 0
		r := readResponse(t, Run("http.StatusText", []string{"404", "0"}, func(string, int64, int64, int64, int64) any { calls++; return json.RawMessage(raw) }))
		if r.OK || r.Error == "" || r.Output != nil || calls != 1 {
			t.Fatalf("accepted malformed callback: %s, %+v", raw, r)
		}
	}
	for _, compute := range []func(string, int64, int64, int64, int64) any{nil, func(string, int64, int64, int64, int64) any { panic("failed") }, func(string, int64, int64, int64, int64) any { return make(chan int) }} {
		if r := readResponse(t, Run("http.StatusText", []string{"404", "0"}, compute)); r.OK || r.Error == "" {
			t.Fatalf("unavailable computation accepted: %+v", r)
		}
	}
}

func rejectBeforeCompute(t *testing.T, name string, inputs []string) {
	t.Helper()
	calls := 0
	r := readResponse(t, Run(name, inputs, func(string, int64, int64, int64, int64) any {
		calls++
		return testReply{}
	}))
	if r.OK || r.Error == "" || r.Output != nil || calls != 0 {
		t.Fatalf("invalid input reached computation: %s %v, calls=%d, result=%+v", name, inputs, calls, r)
	}
}

func TestRunSnapshotsInputsBeforeCallback(t *testing.T) {
	inputs := []string{"12", "34", "56"}
	r := readResponse(t, Run("color.RGBToCMYK", inputs, func(_ string, a, b, c, d int64) any {
		inputs[0] = "999"
		if a != 12 || b != 34 || c != 56 || d != 0 {
			t.Fatal("callback inputs changed")
		}
		return testReply{}
	}))
	if !r.OK || !reflect.DeepEqual(r.Inputs, []string{"12", "34", "56"}) {
		t.Fatalf("response no longer describes the executed inputs: %+v", r)
	}
}
