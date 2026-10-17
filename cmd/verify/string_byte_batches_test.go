package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStringByteActualSerializedBatchesKeepRowsAndBytesBounded(t *testing.T) {
	added, err := stringByteCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range added.Suites {
		var reconstructed []testCase
		batches, maximum := 0, 0
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil {
				t.Fatal(err)
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			path := filepath.Join(t.TempDir(), "cases.json")
			if err := save(path, caseDocument(batch)); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil || len(raw) > stringByteMaxBatchBytes || len(raw) == 0 || raw[len(raw)-1] != '\n' || end-start > 128 || end <= start {
				t.Fatal("actual saved JSON or row limit", len(raw), start, end, err)
			}
			// Byte-based shrinking must take the largest prefix that fits,
			// using the full 128-row budget whenever serialized bytes allow.
			if end < len(s.Cases) && end-start < 128 {
				batch.Cases = s.Cases[start : end+1]
				next, err := json.MarshalIndent(caseDocument(batch), "", "  ")
				if err != nil || len(next)+1 <= stringByteMaxBatchBytes {
					t.Fatal("batch unnecessarily narrowed", err)
				}
			}
			if start == 0 && end <= 16 {
				t.Fatal("short inputs unnecessarily forced into 16-row fixed batches")
			}
			reconstructed = append(reconstructed, s.Cases[start:end]...)
			maximum = max(maximum, len(raw))
			batches++
			start = end
		}
		if !reflect.DeepEqual(reconstructed, s.Cases) {
			t.Fatal("adaptive batching changed, dropped or reordered a case")
		}
		t.Logf("%s:%s cases=%d batches=%d maximum_serialized_bytes=%d", s.Package, s.Activity, len(s.Cases), batches, maximum)
	}
}

func TestStringByteBatchBoundCountsEscapesIndentationAndFinalLF(t *testing.T) {
	s := suite{Package: stringBytePackage, Activity: "IndexByte"}
	for i := 0; i < 128; i++ {
		s.Cases = append(s.Cases, testCase{Inputs: []any{strings.Repeat("\x00", 1024), int64(i)}, Expected: -1})
	}
	end, err := nativeBatchEnd(s, 0, 128)
	if err != nil || end < 1 || end >= 16 {
		t.Fatal("escaped 1024-byte Text did not shrink the serialized batch", end, err)
	}
	batch := s
	batch.Cases = s.Cases[:end]
	raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
	if err != nil || len(raw)+1 > stringByteMaxBatchBytes || !bytes.Contains(raw, []byte(`\u0000`)) {
		t.Fatal("escaped JSON byte accounting", len(raw), err)
	}
	// An unrelated legacy suite keeps its old row limit even with the same
	// artificially large inputs. The new cap must never rebatch old suites.
	s.Package, s.Activity = "go/net/http", "StatusText"
	if legacyEnd, err := nativeBatchEnd(s, 0, 128); err != nil || legacyEnd != 128 {
		t.Fatal("byte bound leaked into a legacy suite", legacyEnd, err)
	}
	s.Package, s.Activity = stringBytePackage, "IndexByte"
	s.Cases = []testCase{{Inputs: []any{strings.Repeat("x", 30000), int64(0)}, Expected: -1}}
	if _, err := nativeBatchEnd(s, 0, 128); err == nil {
		t.Fatal("accepted one case too large to serialize safely")
	}
	for _, boundary := range [][2]int{{-1, 128}, {1, 128}, {0, 0}, {0, 129}} {
		if _, err := nativeBatchEnd(s, boundary[0], boundary[1]); err == nil {
			t.Fatal("accepted invalid start or row limit", boundary)
		}
	}
}

func TestStringByteAdditionKeepsLegacy62241CasesAnd833BatchBoundaries(t *testing.T) {
	all := original57NativeSuites(t)
	if len(all) != 57 || !isStringByteSuite(all[52]) || !isStringByteSuite(all[53]) {
		t.Fatal("new suites must be appended after the original 52")
	}
	oldCases, oldBatches := 0, 0
	for _, s := range all[:52] {
		limit := nativeBatchSize(s)
		for start := 0; start < len(s.Cases); start += limit {
			end, err := nativeBatchEnd(s, start, limit)
			if err != nil || end != min(start+limit, len(s.Cases)) {
				t.Fatal("legacy batch boundary changed", s.Package, s.Activity, start, end, err)
			}
			oldBatches++
		}
		oldCases += len(s.Cases)
	}
	if oldCases != 62241 || oldBatches != 833 {
		t.Fatal("prior native corpus or row-based batch roster changed", oldCases, oldBatches)
	}
	newCases := len(all[52].Cases) + len(all[53].Cases)
	if newCases != 3104 || oldCases+newCases != 65345 {
		t.Fatal("derived new/aggregate case counts differ", newCases)
	}
	ports, activities := 0, 0
	programs := map[string]bool{}
	for _, s := range all[:54] {
		if strings.HasPrefix(s.Package, "go/") {
			ports++
		} else {
			programs[s.Package] = true
			activities++
		}
	}
	if ports != 37 || len(programs) != 15 || activities != 17 {
		t.Fatal("derived port/program/activity roster differs", ports, len(programs), activities)
	}
}

// These formatter-only records use the actual full Go encoder as the oracle;
// they are not new native semantic cases or a second length formula.
func TestStringByteRowLengthExactWireEdges(t *testing.T) {
	makeRows := func(count int, text string, record bool) suite {
		s := suite{Package: stringBytePackage, Activity: "IndexByte"}
		for i := 0; i < count; i++ {
			var expected any = int64(-1)
			if record {
				expected = map[string]any{"ok": true, "nested": map[string]any{"text": text, "items": []any{false, int64(-9223372036854775807), "<>&\u2028\u2029🙂"}}}
			}
			s.Cases = append(s.Cases, testCase{Inputs: []any{text, int64(i)}, Expected: expected})
		}
		return s
	}
	for _, row := range []struct {
		name string
		s    suite
	}{
		{"escaped1024", makeRows(8, strings.Repeat("\x00", 1024), false)},
		{"HTMLUnicodeControls", makeRows(8, "<>&\x00\r\n\t\"\\\u2028\u2029🙂", false)},
		{"nestedRecords", makeRows(8, strings.Repeat("a", 128), true)},
		{"all128", makeRows(128, "x", false)},
	} {
		t.Run(row.name, func(t *testing.T) {
			before, err := json.Marshal(row.s.Cases)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			for end := 1; end <= len(row.s.Cases); end++ {
				part := row.s
				part.Cases = row.s.Cases[:end]
				raw, err := json.MarshalIndent(caseDocument(part), "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if len(raw)+1 <= stringByteMaxBatchBytes {
					want = end
				}
			}
			got, err := nativeBatchEnd(row.s, 0, 128)
			if err != nil || got != want || want < 1 {
				t.Fatal("full Go wire largest-prefix mismatch", got, want, err)
			}
			after, err := json.Marshal(row.s.Cases)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("sizing mutated original rows", err)
			}
		})
	}
}

func TestStringByteRowLengthFinalLFAndMutationBetweenCalls(t *testing.T) {
	s := suite{Package: stringBytePackage, Activity: "IndexByte", Cases: []testCase{{Inputs: []any{"x", int64(0)}, Expected: ""}}}
	base, err := json.MarshalIndent(caseDocument(s), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	padding := stringByteMaxBatchBytes - len(base)
	if padding < 1 {
		t.Fatal("fixture envelope unexpectedly oversized")
	}
	s.Cases[0].Expected = strings.Repeat("x", padding)
	withoutLF, err := json.MarshalIndent(caseDocument(s), "", "  ")
	if err != nil || len(withoutLF) != stringByteMaxBatchBytes {
		t.Fatal("exact LF boundary fixture", err)
	}
	if _, err := nativeBatchEnd(s, 0, 128); err == nil {
		t.Fatal("omitted final LF")
	}
	s.Cases[0].Expected = strings.Repeat("x", padding-1)
	if end, err := nativeBatchEnd(s, 0, 128); err != nil || end != 1 {
		t.Fatal("exact30000 including LF refused", end, err)
	}
	s.Cases[0].Expected = strings.Repeat("\x00", 6000)
	if _, err := nativeBatchEnd(s, 0, 128); err == nil {
		t.Fatal("reused stale previous-call length")
	}
	s.Cases[0].Expected = ""
	if end, err := nativeBatchEnd(s, 0, 128); err != nil || end != 1 {
		t.Fatal("restored row refused", end, err)
	}
	for _, b := range [][2]int{{-1, 128}, {1, 128}, {0, 0}, {0, 129}} {
		if _, err := nativeBatchEnd(s, b[0], b[1]); err == nil {
			t.Fatal("invalid bounds accepted", b)
		}
	}
}

func TestStringByteRowLengthLateJSONErrorKeepsProbeRefusal(t *testing.T) {
	s := suite{Package: stringBytePackage, Activity: "IndexByte"}
	for i := 0; i < 128; i++ {
		s.Cases = append(s.Cases, testCase{Inputs: []any{strings.Repeat("x", 1024), int64(i)}, Expected: int64(-1)})
	}
	// The first64-row probe is oversized. A malformed row beyond it must not
	// be eagerly serialized; a malformed row inside it must still refuse.
	s.Cases[120].Expected = math.NaN()
	end, err := nativeBatchEnd(s, 0, 128)
	if err != nil || end < 1 || end >= 64 {
		t.Fatal("unvisited late error changed admission", end, err)
	}
	part := s
	part.Cases = s.Cases[:end]
	raw, err := json.MarshalIndent(caseDocument(part), "", "  ")
	if err != nil || len(raw)+1 > stringByteMaxBatchBytes {
		t.Fatal("returned prefix exceeds actual wire", err)
	}
	part.Cases = s.Cases[:end+1]
	next, err := json.MarshalIndent(caseDocument(part), "", "  ")
	if err != nil || len(next)+1 <= stringByteMaxBatchBytes {
		t.Fatal("not largest actual wire prefix", err)
	}
	s.Cases[2].Expected = math.NaN()
	if _, err := nativeBatchEnd(s, 0, 128); err == nil {
		t.Fatal("visited malformed row ignored")
	}
}
