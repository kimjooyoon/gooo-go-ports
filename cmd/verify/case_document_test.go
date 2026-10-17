package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// Preserve the original formatter from 4112de2d86125ab03c0c2e1f8371688d83f70253.
// Other full-prefix references call caseDocument and would inherit a formatter
// regression. This map writer is independent of the candidate's row type.
func originalMapCaseDocumentReference(s suite) any {
	rows := []any{}
	key := s.Package + ":" + s.Activity
	for _, c := range s.Cases {
		inputs := map[string]any{}
		for i, v := range c.Inputs {
			k := key
			if len(c.Inputs) > 1 {
				k = fmt.Sprintf("%s.input%d", key, i)
			}
			inputs[k] = v
		}
		rows = append(rows, map[string]any{"inputs": inputs, "expected": map[string]any{key: c.Expected}})
	}
	return map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": rows}
}

type caseDocumentWireFormat struct {
	name   string
	encode func(any) ([]byte, error)
}

func caseDocumentWireFormats() []caseDocumentWireFormat {
	return []caseDocumentWireFormat{
		{"compact", json.Marshal},
		{"pretty", func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }},
		{"prefix-and-tabs", func(v any) ([]byte, error) { return json.MarshalIndent(v, ">>", "\t") }},
		{"empty-indent", func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "") }},
	}
}

func requireCaseDocumentEncoding(t *testing.T, got []byte, err error, want []byte, wantErr error) {
	t.Helper()
	if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
		t.Fatalf("original JSON bytes/nilness changed:\n got %q\nwant %q", got, want)
	}
	if (err == nil) != (wantErr == nil) || reflect.TypeOf(err) != reflect.TypeOf(wantErr) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
		t.Fatalf("original JSON error changed: got %T: %v, want %T: %v", err, err, wantErr, wantErr)
	}
	var gotMarshaler, wantMarshaler *json.MarshalerError
	if errors.As(err, &gotMarshaler) != errors.As(wantErr, &wantMarshaler) {
		t.Fatal("original marshaler error chain changed")
	}
	if gotMarshaler != nil && (gotMarshaler.Type != wantMarshaler.Type || reflect.TypeOf(gotMarshaler.Err) != reflect.TypeOf(wantMarshaler.Err) || fmt.Sprint(gotMarshaler.Err) != fmt.Sprint(wantMarshaler.Err)) {
		t.Fatal("original marshaler value type or wrapped error changed", gotMarshaler, wantMarshaler)
	}
}

func caseDocumentBuiltinFixture() suite {
	var nilInteger *int
	return suite{
		Package:  "go/example<>&",
		Activity: "Inspect\x00가🙂",
		Cases: []testCase{
			{Inputs: nil, Expected: nil},
			{Inputs: []any{}, Expected: nilInteger},
			{Inputs: []any{"<>&\x00\r\n\t\"\\\u2028\u2029가🙂" + string([]byte{0xff})}, Expected: map[string]any{"empty": []any{}, "null": []any(nil), "map": map[string]any{}}},
			{Inputs: []any{int64(math.MinInt64), uint64(math.MaxUint64)}, Expected: []any{nil, false, true, float32(1.5), math.Copysign(0, -1), json.Number("1e+03"), map[string]any{"nested": []any{int64(-1), "text"}}}},
			{Inputs: []any{nil, "one", int64(2), true, false, float64(5.5), int64(6), int64(7), int64(8), int64(9), "ten", "eleven"}, Expected: int64(math.MaxInt64)},
		},
	}
}

func TestCaseDocumentOriginalBuiltinWire(t *testing.T) {
	for _, test := range []struct {
		name string
		s    suite
	}{
		{"nil-cases", suite{Package: "go/example", Activity: "Inspect"}},
		{"empty-cases", suite{Package: "go/example", Activity: "Inspect", Cases: []testCase{}}},
		{"mixed-arity-and-values", caseDocumentBuiltinFixture()},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, err := json.Marshal(test.s.Cases)
			if err != nil {
				t.Fatal(err)
			}
			for _, format := range caseDocumentWireFormats() {
				t.Run(format.name, func(t *testing.T) {
					want, wantErr := format.encode(originalMapCaseDocumentReference(test.s))
					got, err := format.encode(caseDocument(test.s))
					requireCaseDocumentEncoding(t, got, err, want, wantErr)
					if err != nil {
						t.Fatal(err)
					}
				})
			}
			after, err := json.Marshal(test.s.Cases)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("formatter mutated original cases", err)
			}
			if len(test.s.Cases) == 0 {
				raw, err := json.Marshal(caseDocument(test.s))
				if err != nil || string(raw) != `{"cases":[],"schema":"gooo/body-composition-cases/v1"}` {
					t.Fatal("empty cases must retain an array", string(raw), err)
				}
			}
		})
	}
}

func TestCaseDocumentStructuralConsumersAndInputKeys(t *testing.T) {
	s := caseDocumentBuiltinFixture()
	document, ok := caseDocument(s).(map[string]any)
	if !ok {
		t.Fatal("batch sizers require the original top-level map")
	}
	rows, ok := document["cases"].([]any)
	if !ok || len(rows) != len(s.Cases) {
		t.Fatal("batch sizers require cases as []any")
	}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Cases []struct {
			Inputs map[string]json.RawMessage `json:"inputs"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	key := s.Package + ":" + s.Activity
	for i, c := range s.Cases {
		inputs := decoded.Cases[i].Inputs
		if inputs == nil || len(inputs) != len(c.Inputs) {
			t.Fatal("nil/empty inputs or mixed row arity changed", i, inputs)
		}
		for input := range c.Inputs {
			name := key
			if len(c.Inputs) > 1 {
				name = fmt.Sprintf("%s.input%d", key, input)
			}
			if _, ok := inputs[name]; !ok {
				t.Fatal("original input key missing", i, name)
			}
		}
	}
}

func TestCaseDocumentOriginalJSONErrors(t *testing.T) {
	for _, test := range []struct {
		name  string
		value func() any
	}{
		{"NaN", func() any { return math.NaN() }},
		{"positive-infinity", func() any { return math.Inf(1) }},
		{"negative-infinity", func() any { return math.Inf(-1) }},
		{"invalid-number", func() any { return json.Number("not-a-number") }},
		{"channel", func() any { return make(chan int) }},
		{"function", func() any { return func() {} }},
		{"cycle", func() any { v := map[string]any{}; v["self"] = v; return v }},
	} {
		for _, location := range []string{"expected", "input"} {
			t.Run(test.name+"/"+location, func(t *testing.T) {
				build := func() suite {
					s := suite{Package: "go/example", Activity: "Inspect", Cases: []testCase{{Inputs: []any{"input"}, Expected: "expected"}}}
					if location == "expected" {
						s.Cases[0].Expected = map[string]any{"nested": test.value()}
					} else {
						s.Cases[0].Inputs[0] = []any{test.value()}
					}
					return s
				}
				for _, format := range caseDocumentWireFormats() {
					t.Run(format.name, func(t *testing.T) {
						want, wantErr := format.encode(originalMapCaseDocumentReference(build()))
						got, err := format.encode(caseDocument(build()))
						requireCaseDocumentEncoding(t, got, err, want, wantErr)
						if err == nil {
							t.Fatal("unsupported JSON value was admitted")
						}
					})
				}
			})
		}
	}
}

type caseDocumentMarshalState struct {
	calls     []string
	failAt    int
	invalidAt int
	failure   error
}

func (s *caseDocumentMarshalState) marshal(name string) ([]byte, error) {
	s.calls = append(s.calls, name)
	if len(s.calls) == s.failAt {
		return nil, s.failure
	}
	if len(s.calls) == s.invalidAt {
		return []byte("not-json"), nil
	}
	return json.Marshal(fmt.Sprintf("%s/call%d", name, len(s.calls)))
}

type caseDocumentPointerJSON struct {
	state *caseDocumentMarshalState
	name  string
}

func (v *caseDocumentPointerJSON) MarshalJSON() ([]byte, error) {
	if v == nil {
		return []byte(`"nil-method-called"`), nil
	}
	return v.state.marshal(v.name)
}

type caseDocumentValueJSON struct {
	state *caseDocumentMarshalState
	name  string
}

func (v caseDocumentValueJSON) MarshalJSON() ([]byte, error) {
	return v.state.marshal(v.name)
}

func TestCaseDocumentOriginalCustomMarshalOrderAndErrors(t *testing.T) {
	fullOrder := []string{"r0.expected", "r0.in0", "r0.in1", "r0.in10", "r0.in11", "r0.in2", "r0.in3", "r0.in4", "r0.in5", "r0.in6", "r0.in7", "r0.in8", "r0.in9", "r1.expected.a", "r1.expected.b", "r1.in0"}
	failure := errors.New("case document marshal sentinel")
	for _, test := range []struct {
		name      string
		failAt    int
		invalidAt int
	}{
		{name: "changing-values"},
		{name: "first-expected-fails", failAt: 1},
		{name: "first-input-fails", failAt: 2},
		{name: "lexical-input10-fails", failAt: 4},
		{name: "second-row-expected-fails", failAt: 14},
		{name: "second-row-input-fails", failAt: 16},
		{name: "invalid-custom-JSON", invalidAt: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			build := func() (suite, *caseDocumentMarshalState) {
				state := &caseDocumentMarshalState{failAt: test.failAt, invalidAt: test.invalidAt, failure: failure}
				pointer := func(name string) any { return &caseDocumentPointerJSON{state: state, name: name} }
				inputs := []any{}
				for i := range 12 {
					inputs = append(inputs, pointer(fmt.Sprintf("r0.in%d", i)))
				}
				var nilValue *caseDocumentPointerJSON
				return suite{Package: "go/example", Activity: "Inspect", Cases: []testCase{
					{Inputs: inputs, Expected: caseDocumentValueJSON{state: state, name: "r0.expected"}},
					{Inputs: []any{pointer("r1.in0")}, Expected: map[string]any{"b": pointer("r1.expected.b"), "a": pointer("r1.expected.a")}},
					{Inputs: []any{nilValue}, Expected: nilValue},
				}}, state
			}
			for _, format := range caseDocumentWireFormats() {
				t.Run(format.name, func(t *testing.T) {
					original, originalState := build()
					candidate, candidateState := build()
					oldDocument := originalMapCaseDocumentReference(original)
					newDocument := caseDocument(candidate)
					if len(originalState.calls) != 0 || len(candidateState.calls) != 0 {
						t.Fatal("document construction invoked a marshaler")
					}
					want, wantErr := format.encode(oldDocument)
					got, err := format.encode(newDocument)
					requireCaseDocumentEncoding(t, got, err, want, wantErr)
					count := len(fullOrder)
					if test.failAt > 0 {
						count = test.failAt
					} else if test.invalidAt > 0 {
						count = test.invalidAt
					}
					if !reflect.DeepEqual(originalState.calls, fullOrder[:count]) || !reflect.DeepEqual(candidateState.calls, originalState.calls) {
						t.Fatal("original method order/count changed", originalState.calls, candidateState.calls)
					}
					if test.failAt > 0 && (!errors.Is(wantErr, failure) || !errors.Is(err, failure)) {
						t.Fatal("custom error unwrap chain lost the original sentinel", err, wantErr)
					}
					if test.failAt+test.invalidAt == 0 {
						if err != nil || bytes.Contains(got, []byte("nil-method-called")) {
							t.Fatal("success or typed-nil marshaler behavior changed", err)
						}
					} else if err == nil {
						t.Fatal("original marshaler failure was ignored")
					}
				})
			}
		})
	}
}

var caseDocumentBenchmarkSink any
var caseDocumentWireBenchmarkSink []byte

func BenchmarkCaseDocument(b *testing.B) {
	// This small fixed fixture exercises mixed arity and nested values. It does
	// not construct the verification corpus or invoke the native/compiler path.
	s := caseDocumentBuiltinFixture()
	for _, writer := range []struct {
		name  string
		build func(suite) any
	}{{"original-map", originalMapCaseDocumentReference}, {"presized-map", caseDocument}} {
		b.Run("build/"+writer.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				caseDocumentBenchmarkSink = writer.build(s)
			}
		})
		for _, format := range caseDocumentWireFormats()[:2] {
			b.Run(format.name+"/"+writer.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					raw, err := format.encode(writer.build(s))
					if err != nil {
						b.Fatal(err)
					}
					caseDocumentWireBenchmarkSink = raw
				}
			})
		}
	}
}
