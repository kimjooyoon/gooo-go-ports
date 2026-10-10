package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMathBitsCorpusExhaustiveFunctionsAndConsumer(t *testing.T) {
	all, err := mathBitsCorpus()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Len8", "LeadingZeros8", "TrailingZeros8", "OnesCount8", "Reverse8"}
	landmarks := map[int64][]int{
		0: {0, 8, 8, 0, 0}, 1: {1, 7, 0, 1, 128}, 127: {7, 1, 0, 7, 254},
		128: {8, 0, 7, 1, 1}, 170: {8, 0, 1, 4, 85}, 255: {8, 0, 0, 8, 255},
	}
	if len(all) != len(names)+1 {
		t.Fatalf("expected five functions and one consumer, got %d", len(all))
	}
	for i, name := range names {
		s := all[i]
		if s.Package != "go/math/bits" || s.Activity != name || len(s.Cases) != 256 {
			t.Fatalf("incomplete function roster for %s: %s:%s (%d cases)", name, s.Package, s.Activity, len(s.Cases))
		}
		seen := [256]bool{}
		for _, c := range s.Cases {
			if len(c.Inputs) != 1 {
				t.Fatalf("%s needs exactly one input", name)
			}
			n, ok := c.Inputs[0].(int64)
			if !ok || n < 0 || n > 255 || seen[n] {
				t.Fatalf("%s has a duplicate or out-of-domain input: %v", name, c.Inputs)
			}
			seen[n] = true
			if want, ok := landmarks[n]; ok && !equalValue(mustJSON(t, c.Expected), want[i]) {
				t.Fatalf("%s(%d) = %v, want %d", name, n, c.Expected, want[i])
			}
		}
	}
	consumer := all[len(names)]
	inputs := []int64{0, 1, 127, 128, 170, 255}
	if consumer.Package != "app/byte-summary" || consumer.Activity != "ByteSummary" || len(consumer.Cases) != len(inputs) {
		t.Fatal("wrong importing consumer roster")
	}
	for i, c := range consumer.Cases {
		if !reflect.DeepEqual(c.Inputs, []any{inputs[i]}) {
			t.Fatalf("wrong consumer input at %d: %v", i, c.Inputs)
		}
		values := landmarks[inputs[i]]
		want := map[string]int{"length": values[0], "ones": values[3], "reversed": values[4]}
		if !equalValue(mustJSON(t, c.Expected), want) {
			t.Fatalf("wrong independent consumer expectation: %v, want %v", c.Expected, want)
		}
	}
}

func TestByteCorpusRejectsInvalidInputsBeforeConversion(t *testing.T) {
	complete := make([]int64, 256)
	for i := range complete {
		complete[i] = int64(i)
	}
	duplicate := append([]int64(nil), complete...)
	duplicate[255] = 0
	tests := []struct {
		name, reason string
		inputs       []int64
		exhaustive   bool
	}{
		{"empty", "empty", nil, true},
		{"missing byte", "missing input 255", complete[:255], true},
		{"duplicate byte", "duplicate byte input 0", duplicate, true},
		{"negative wraps to 255", "outside the uint8 domain", []int64{0, -1}, false},
		{"256 wraps to zero", "outside the uint8 domain", []int64{0, 256}, false},
		{"257 wraps to one", "outside the uint8 domain", []int64{0, 257}, false},
		{"int64 minimum", "outside the uint8 domain", []int64{0, math.MinInt64}, false},
		{"int64 maximum", "outside the uint8 domain", []int64{0, math.MaxInt64}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			_, err := byteSuite("go/math/bits", "test", "uint8", test.inputs, test.exhaustive, func(uint8) any {
				called = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), test.reason) || called {
				t.Fatalf("invalid corpus reached the oracle: called=%v err=%v", called, err)
			}
		})
	}
	var observed []uint8
	s, err := byteSuite("app/byte-summary", "test", "uint8 boundaries", []int64{0, 255}, false, func(n uint8) any {
		observed = append(observed, n)
		return n
	})
	if err != nil || len(s.Cases) != 2 || !reflect.DeepEqual(observed, []uint8{0, 255}) {
		t.Fatalf("valid endpoints changed at the oracle boundary: %v, %v", observed, err)
	}
}
