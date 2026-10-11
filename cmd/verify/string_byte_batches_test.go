package main

import (
	"bytes"
	"encoding/json"
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
	all := allNativeSuites(t)
	if len(all) != 54 || !isStringByteSuite(all[52]) || !isStringByteSuite(all[53]) {
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
	for _, s := range all {
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
