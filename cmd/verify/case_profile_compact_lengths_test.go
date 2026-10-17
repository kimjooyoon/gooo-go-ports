package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Retain the original full-prefix Go encoder as an independent wire oracle.
// These small formatter fixtures do not supply native semantic expectations.
func uuidCompactFullEncoderReference(s suite, start int) (nativePreparedCaseBatch, error) {
	low, high, best := start+1, min(start+128, len(s.Cases)), start
	var selected []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		raw, err := json.Marshal(caseDocument(batch))
		if err != nil {
			return nativePreparedCaseBatch{}, fmt.Errorf("serialize uuid-compact-v1: %w", err)
		}
		raw = append(raw, '\n')
		if len(raw) <= 30000 && len(raw)-1 <= 32768 {
			best, selected, low = end, raw, end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one original UUID case exceeds uuid-compact-v1 wire budget")
	}
	return nativePreparedCaseBatch{End: best, Raw: selected}, nil
}

func uuidCompactLengthFixture(count int, input string) suite {
	s := suite{Package: uuidPackage, Activity: "Validate"}
	for i := 0; i < count; i++ {
		s.Cases = append(s.Cases, testCase{
			Inputs:   []any{input},
			Expected: map[string]any{"value": int64(i)},
		})
	}
	return s
}

func uuidCompactLengthPolicy(t *testing.T, s suite) nativeCaseBatchPolicy {
	t.Helper()
	p, err := uuidCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func requireUUIDCompactSameResult(t *testing.T, got nativePreparedCaseBatch, err error, want nativePreparedCaseBatch, wantErr error) {
	t.Helper()
	if (err == nil) != (wantErr == nil) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
		t.Fatalf("full Go encoder error changed: got %v, want %v", err, wantErr)
	}
	if got.End != want.End || !bytes.Equal(got.Raw, want.Raw) || (got.Raw == nil) != (want.Raw == nil) {
		t.Fatalf("full Go encoder interval/raw changed: end %d/%d, bytes %d/%d", got.End, want.End, len(got.Raw), len(want.Raw))
	}
}

func TestUUIDCompactRowLengthsExactBuiltinWire(t *testing.T) {
	s := uuidCompactLengthFixture(129, "<>&\x00\r\n\t\"\\\u2028\u2029가🙂")
	for i := range s.Cases {
		s.Cases[i].Expected = map[string]any{
			"nested": []any{nil, false, true, int64(math.MinInt64), uint64(math.MaxUint64),
				float32(1.5), math.Copysign(0, -1), json.Number("1e+03"),
				map[string]any{"text": string([]byte{'a', 0xff, 'b'}), "empty": []any{}}},
			"ordinal": int64(i),
		}
	}
	before, err := json.Marshal(s.Cases)
	if err != nil {
		t.Fatal(err)
	}
	p := uuidCompactLengthPolicy(t, s)
	for start := 0; start < len(s.Cases); {
		want, wantErr := uuidCompactFullEncoderReference(s, start)
		got, err := p.prepare(s, start)
		requireUUIDCompactSameResult(t, got, err, want, wantErr)
		if err != nil || got.End <= start {
			t.Fatal("builtin fixture unexpectedly refused", err)
		}
		start = got.End
	}
	after, err := json.Marshal(s.Cases)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("sizing changed the original input/order/expectations", err)
	}
}

func TestUUIDCompactRowLengthsLFRowLimitAndMutation(t *testing.T) {
	s := uuidCompactLengthFixture(1, "x")
	record := s.Cases[0].Expected.(map[string]any)
	record["value"] = ""
	base, err := json.Marshal(caseDocument(s))
	if err != nil {
		t.Fatal(err)
	}
	p := uuidCompactLengthPolicy(t, s)
	for _, size := range []int{29999, 30000, 30001} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			record["value"] = strings.Repeat("x", size-len(base)-1)
			want, wantErr := uuidCompactFullEncoderReference(s, 0)
			got, err := p.prepare(s, 0)
			requireUUIDCompactSameResult(t, got, err, want, wantErr)
			if (size <= 30000) != (err == nil) || (err == nil && len(got.Raw) != size) {
				t.Fatal("LF-inclusive boundary changed", size, len(got.Raw), err)
			}
		})
	}
	// Reuse the receiver after mutating the same map: no length survives a call.
	record["value"] = ""
	first, err := p.prepare(s, 0)
	if err != nil {
		t.Fatal(err)
	}
	record["value"] = strings.Repeat("\x00", 6000)
	if _, err := p.prepare(s, 0); err == nil {
		t.Fatal("sizing reused a stale previous-call length")
	}
	record["value"] = ""
	restored, err := p.prepare(s, 0)
	if err != nil || !bytes.Equal(first.Raw, restored.Raw) {
		t.Fatal("restored value did not restore exact wire bytes", err)
	}
	rows := uuidCompactLengthFixture(129, "")
	p = uuidCompactLengthPolicy(t, rows)
	for _, span := range [][2]int{{0, 128}, {128, 129}} {
		want, wantErr := uuidCompactFullEncoderReference(rows, span[0])
		got, err := p.prepare(rows, span[0])
		requireUUIDCompactSameResult(t, got, err, want, wantErr)
		if err != nil || got.End != span[1] {
			t.Fatal("128-row ceiling or final original row changed", span, got.End, err)
		}
	}
}

func TestUUIDCompactRowLengthsLazyNumericErrors(t *testing.T) {
	for _, bad := range []struct {
		name  string
		value any
	}{{"NaN", math.NaN()}, {"Infinity", math.Inf(1)}, {"invalidNumber", json.Number("not-a-number")}} {
		for _, index := range []int{120, 2} {
			t.Run(fmt.Sprintf("%s/row%d", bad.name, index), func(t *testing.T) {
				s := uuidCompactLengthFixture(128, strings.Repeat("x", 1024))
				s.Cases[index].Expected.(map[string]any)["value"] = bad.value
				p := uuidCompactLengthPolicy(t, s)
				want, wantErr := uuidCompactFullEncoderReference(s, 0)
				got, err := p.prepare(s, 0)
				requireUUIDCompactSameResult(t, got, err, want, wantErr)
				if index == 120 && (err != nil || got.End < 1 || got.End >= 64) {
					t.Fatal("unvisited late error was evaluated eagerly", got.End, err)
				}
				if index == 2 && err == nil {
					t.Fatal("visited malformed number was ignored")
				}
			})
		}
	}
}

type uuidCompactMarshalState struct {
	calls  []int
	failAt int
}

type uuidCompactStatefulJSON struct {
	state *uuidCompactMarshalState
	row   int
}

func (v uuidCompactStatefulJSON) MarshalJSON() ([]byte, error) {
	v.state.calls = append(v.state.calls, v.row)
	if len(v.state.calls) == v.state.failAt {
		return nil, errors.New("fixture marshal failure")
	}
	return json.Marshal(fmt.Sprintf("row%d-call%d", v.row, len(v.state.calls)))
}

func TestUUIDCompactRowLengthsStatefulFallback(t *testing.T) {
	for _, test := range []struct {
		name     string
		failAt   int
		onlyLate bool
	}{{name: "changingBytes"}, {name: "seventhCallFails", failAt: 7}, {name: "unvisitedLate", onlyLate: true}} {
		t.Run(test.name, func(t *testing.T) {
			build := func() (suite, *uuidCompactMarshalState) {
				state := &uuidCompactMarshalState{failAt: test.failAt}
				width := 256
				if test.onlyLate {
					width = 1024
				}
				s := uuidCompactLengthFixture(128, strings.Repeat("x", width))
				for i := range s.Cases {
					if !test.onlyLate || i == 120 {
						s.Cases[i].Expected.(map[string]any)["value"] = uuidCompactStatefulJSON{state: state, row: i}
					}
				}
				return s, state
			}
			original, originalState := build()
			candidate, candidateState := build()
			p := uuidCompactLengthPolicy(t, candidate)
			if len(candidateState.calls) != 0 {
				t.Fatal("policy admission called a custom marshaler")
			}
			want, wantErr := uuidCompactFullEncoderReference(original, 0)
			got, err := p.prepare(candidate, 0)
			requireUUIDCompactSameResult(t, got, err, want, wantErr)
			if !reflect.DeepEqual(candidateState.calls, originalState.calls) {
				t.Fatal("custom marshaler invocation order/count changed", candidateState.calls, originalState.calls)
			}
			if test.onlyLate && len(candidateState.calls) != 0 {
				t.Fatal("unvisited custom marshaler was called")
			}
			if !test.onlyLate && len(candidateState.calls) == 0 {
				t.Fatal("fixture did not exercise the fallback encoder")
			}
		})
	}
}

func TestUUIDCompactRowLengthsOtherFallbackValues(t *testing.T) {
	type namedText string
	deep := any("leaf")
	for i := 0; i < 34; i++ {
		deep = []any{deep}
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, row := range []struct {
		name  string
		value any
	}{{"named", namedText("<>&")}, {"deep", deep}, {"cycle", cycle}, {"unknown", make(chan int)}} {
		t.Run(row.name, func(t *testing.T) {
			s := uuidCompactLengthFixture(1, "x")
			s.Cases[0].Expected.(map[string]any)["value"] = row.value
			p := uuidCompactLengthPolicy(t, s)
			want, wantErr := uuidCompactFullEncoderReference(s, 0)
			got, err := p.prepare(s, 0)
			requireUUIDCompactSameResult(t, got, err, want, wantErr)
		})
	}
}
