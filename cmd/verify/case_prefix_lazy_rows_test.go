package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Serialization fixtures only: these add no native semantic cases.
type lazyRowTrialFixture struct {
	name  string
	s     suite
	start int
}

func lazyRowTrialFixtures(compact bool) []lazyRowTrialFixture {
	var fixtures []lazyRowTrialFixture
	for _, f := range []struct {
		name            string
		n, start, width int
	}{
		{"all-fit-1", 1, 0, 1},
		{"all-fit-8", 8, 0, 8},
		{"all-fit-128", 128, 0, 1},
		{"shrinking-128", 128, 0, 1024},
		{"nonzero-start-128", 128, 7, 1024},
	} {
		s := suite{Package: stringBytePackage, Activity: "IndexByte"}
		if compact {
			s.Package, s.Activity = uuidPackage, "Validate"
		}
		text := strings.Repeat("x", f.width)
		for i := 0; i < f.start+f.n; i++ {
			c := testCase{Inputs: []any{text, int64(i)}, Expected: int64(-1)}
			if compact {
				c = testCase{Inputs: []any{text}, Expected: map[string]any{"value": int64(i), "ok": true}}
			}
			s.Cases = append(s.Cases, c)
		}
		fixtures = append(fixtures, lazyRowTrialFixture{f.name, s, f.start})
	}
	return fixtures
}

// Independent original-map full-document binary probes. No candidate row helper.
func lazyRowTrialReference(s suite, start int, compact bool) (nativePreparedCaseBatch, error) {
	low, high, best := start+1, min(start+128, len(s.Cases)), start
	var selected []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		var raw []byte
		var err error
		if compact {
			raw, err = json.Marshal(originalMapCaseDocumentReference(batch))
		} else {
			raw, err = json.MarshalIndent(originalMapCaseDocumentReference(batch), "", "  ")
		}
		if err != nil {
			if compact {
				return nativePreparedCaseBatch{}, fmt.Errorf("serialize uuid-compact-v1: %w", err)
			}
			return nativePreparedCaseBatch{}, fmt.Errorf("serialize string byte batch: %w", err)
		}
		raw = append(raw, '\n')
		if len(raw) <= 30000 && (!compact || len(raw)-1 <= 32768) {
			best, selected, low = end, raw, end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		if compact {
			return nativePreparedCaseBatch{}, fmt.Errorf("one original UUID case exceeds uuid-compact-v1 wire budget")
		}
		return nativePreparedCaseBatch{}, fmt.Errorf("one string byte case exceeds %d serialized bytes", stringByteMaxBatchBytes)
	}
	return nativePreparedCaseBatch{End: best, Raw: selected}, nil
}

func lazyRowTrialActual(s suite, start int, compact bool, p nativeCaseBatchPolicy) (nativePreparedCaseBatch, error) {
	if compact {
		return p.prepare(s, start)
	}
	end, err := nativeBatchEnd(s, start, 128)
	if err != nil {
		return nativePreparedCaseBatch{}, err
	}
	batch := s
	batch.Cases = s.Cases[start:end]
	raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
	if err != nil {
		return nativePreparedCaseBatch{}, err
	}
	return nativePreparedCaseBatch{End: end, Raw: append(raw, '\n')}, nil
}

// This is a derivation from the unchanged monotone search and observed result,
// not instrumentation of producer probes or of map construction.
func lazyRowTrialDerivedMaxProbe(start, limit, selectedEnd int) int {
	low, high, maximum := start+1, limit, start
	for low <= high {
		mid := low + (high-low)/2
		maximum = max(maximum, mid)
		if mid <= selectedEnd {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return maximum - start
}

var lazyRowTrialEndSink int
var lazyRowTrialBatchSink nativePreparedCaseBatch

func requireLazyRowFixture(tb testing.TB, f lazyRowTrialFixture, compact bool, mode string) nativeCaseBatchPolicy {
	tb.Helper()
	p := nativeCaseBatchPolicy{}
	if compact {
		var err error
		p, err = uuidCompactCaseProfile.policy(f.s, caseProfileTestWorkspace(f.s), caseProfileTestLock())
		if err != nil {
			tb.Fatal(err)
		}
	}
	want, wantErr := lazyRowTrialReference(f.s, f.start, compact)
	actual, err := lazyRowTrialActual(f.s, f.start, compact, p)
	if wantErr != nil || err != nil || actual.End != want.End || !bytes.Equal(actual.Raw, want.Raw) {
		tb.Fatalf("independent full-writer mismatch end=%d/%d err=%v/%v", actual.End, want.End, err, wantErr)
	}
	limit := min(f.start+128, len(f.s.Cases))
	nextBytes := -1
	if actual.End < limit {
		next := f.s
		next.Cases = f.s.Cases[f.start : actual.End+1]
		var raw []byte
		if compact {
			raw, err = json.Marshal(originalMapCaseDocumentReference(next))
		} else {
			raw, err = json.MarshalIndent(originalMapCaseDocumentReference(next), "", "  ")
		}
		nextBytes = len(raw) + 1
		if err != nil || (nextBytes <= 30000 && (!compact || nextBytes-1 <= 32768)) {
			tb.Fatal("next original row still fits or failed encoding", nextBytes, err)
		}
	}
	if strings.HasPrefix(f.name, "all-fit-") && actual.End != limit {
		tb.Fatal("all-fit fixture unexpectedly shrank")
	}
	if !strings.HasPrefix(f.name, "all-fit-") && actual.End >= limit {
		tb.Fatal("shrinking fixture did not shrink")
	}
	tb.Logf("LAZY_FIXTURE mode=%s name=%s start=%d N=%d observed_selected_end=%d selected_rows=%d actual_wire_bytes=%d actual_wire_sha256=%x next_wire_bytes=%d derived_max_probed_M=%d producer_probe_instrumented=false", mode, f.name, f.start, limit-f.start, actual.End, actual.End-f.start, len(actual.Raw), sha256.Sum256(actual.Raw), nextBytes, lazyRowTrialDerivedMaxProbe(f.start, limit, actual.End))
	return p
}

func BenchmarkLazyCaseRowPrefix(b *testing.B) {
	for _, compact := range []bool{false, true} {
		mode := "pretty-nativeBatchEnd"
		if compact {
			mode = "uuid-compact-prepare"
		}
		for _, f := range lazyRowTrialFixtures(compact) {
			b.Run(mode+"/"+f.name, func(b *testing.B) {
				p := requireLazyRowFixture(b, f, compact, mode)
				b.ReportAllocs()
				for b.Loop() {
					if compact {
						batch, err := p.prepare(f.s, f.start)
						if err != nil {
							b.Fatal(err)
						}
						lazyRowTrialBatchSink = batch
					} else {
						end, err := nativeBatchEnd(f.s, f.start, 128)
						if err != nil {
							b.Fatal(err)
						}
						lazyRowTrialEndSink = end
					}
				}
			})
		}
	}
}

func TestLazyCaseRowProbeLateNumericErrors(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, bad := range []struct {
			name  string
			value any
		}{
			{"NaN", math.NaN()}, {"invalidNumber", json.Number("not-a-number")},
		} {
			for _, index := range []int{50, 120} {
				t.Run(fmt.Sprintf("compact=%t/%s/row%d", compact, bad.name, index), func(t *testing.T) {
					f := lazyRowTrialFixtures(compact)[3]
					if compact {
						f.s.Cases[index].Expected.(map[string]any)["value"] = bad.value
					} else {
						f.s.Cases[index].Expected = bad.value
					}
					p := nativeCaseBatchPolicy{}
					if compact {
						var err error
						p, err = uuidCompactCaseProfile.policy(f.s, caseProfileTestWorkspace(f.s), caseProfileTestLock())
						if err != nil {
							t.Fatal(err)
						}
					}
					want, wantErr := lazyRowTrialReference(f.s, 0, compact)
					actual, err := lazyRowTrialActual(f.s, 0, compact, p)
					if actual.End != want.End || !bytes.Equal(actual.Raw, want.Raw) {
						t.Fatal("malformed numeric result differs from independent full writer")
					}
					for got, expected := err, wantErr; got != nil || expected != nil; got, expected = errors.Unwrap(got), errors.Unwrap(expected) {
						if reflect.TypeOf(got) != reflect.TypeOf(expected) || fmt.Sprint(got) != fmt.Sprint(expected) {
							t.Fatalf("numeric error/wrapped path changed %T:%v versus %T:%v", got, got, expected, expected)
						}
					}
					if (index == 50) != (err != nil) {
						t.Fatal("included late error hidden, or unvisited row evaluated eagerly", err)
					}
				})
			}
		}
	}
}

// This runs under ordinary go test even when benchmarks are not requested.
func TestLazyCaseRowPrefixIndependentWireAndMaximality(t *testing.T) {
	for _, compact := range []bool{false, true} {
		mode := "pretty-nativeBatchEnd"
		if compact {
			mode = "uuid-compact-prepare"
		}
		for _, f := range lazyRowTrialFixtures(compact) {
			t.Run(mode+"/"+f.name, func(t *testing.T) {
				before, err := json.Marshal(f.s.Cases)
				if err != nil {
					t.Fatal(err)
				}
				requireLazyRowFixture(t, f, compact, mode)
				after, err := json.Marshal(f.s.Cases)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("prefix sizing changed original inputs, expectations or order", err)
				}
			})
		}
	}
}
