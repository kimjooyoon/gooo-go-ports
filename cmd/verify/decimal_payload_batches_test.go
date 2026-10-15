package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Independent translation-shaped struct, rather than checking only the bound
// formula against itself. The pinned compiler uses these JSON properties.
func translatedDecimalPayloadBytes(t *testing.T, s suite, lowered string) int {
	t.Helper()
	type row struct {
		Inputs   map[string]any `json:"inputs"`
		Expected map[string]any `json:"expected"`
	}
	d := struct {
		Schema string `json:"schema"`
		Cases  []row  `json:"cases"`
	}{Schema: "gooo/body-composition-cases/v1"}
	for _, c := range s.Cases {
		r := row{Inputs: map[string]any{}, Expected: map[string]any{lowered: c.Expected}}
		for i, v := range c.Inputs {
			key := lowered
			if len(c.Inputs) > 1 {
				key = fmt.Sprintf("%s.input%d", lowered, i)
			}
			r.Inputs[key] = v
		}
		d.Cases = append(d.Cases, r)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

func TestDecimalPayloadPreparedBatchesPreserveInputsAndRespectTranslation(t *testing.T) {
	added, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	totalBatches := 0
	for _, s := range added.Suites {
		w, err := catalog.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		other, err := uuidCompactCaseProfile.policy(s, w, caseProfileTestLock())
		if err != nil || policy != other {
			t.Fatal("UUID profile changed new entries", err)
		}
		var reconstructed []testCase
		batches, maxRaw, maxTranslated := 0, 0, 0
		for start := 0; start < len(s.Cases); {
			prepared, err := policy.prepare(s, start)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.End <= start || prepared.End-start > 128 || len(prepared.Raw) > 30000 {
				t.Fatal("bad batch bounds")
			}
			part := s
			part.Cases = s.Cases[start:prepared.End]
			want, err := json.MarshalIndent(caseDocument(part), "", "  ")
			if err != nil || !bytes.Equal(append(want, '\n'), prepared.Raw) {
				t.Fatal("sizer/writer mismatch", err)
			}
			// Maximal ordinal spelling permitted by the source-byte bound.
			lowered := "GoooPackage" + strings.Repeat("9", len(fmt.Sprint(policy.sourceBytes))) + "Activity" + s.Activity
			translated := translatedDecimalPayloadBytes(t, part, lowered)
			if translated > 32768 {
				t.Fatal("translated document exceeds compiler cap", translated)
			}
			if prepared.End < len(s.Cases) && prepared.End-start < 128 {
				part.Cases = s.Cases[start : prepared.End+1]
				next, _ := json.MarshalIndent(caseDocument(part), "", "  ")
				if len(next)+1 <= 30000 && translatedDecimalPayloadBytes(t, part, lowered) <= 32768 {
					t.Fatal("unnecessarily small batch")
				}
			}
			reconstructed = append(reconstructed, s.Cases[start:prepared.End]...)
			batches++
			maxRaw = max(maxRaw, len(prepared.Raw))
			maxTranslated = max(maxTranslated, translated)
			start = prepared.End
		}
		if !reflect.DeepEqual(reconstructed, s.Cases) {
			t.Fatal("cases lost, changed or reordered")
		}
		totalBatches += batches
		t.Logf("%s:%s cases=%d batches=%d max_raw=%d max_translated=%d", s.Package, s.Activity, len(s.Cases), batches, maxRaw, maxTranslated)
	}
	t.Logf("new cases=%d batches=%d; original66 remain159909/2109", added.DecimalReferenceCases+added.PayloadReferenceCases, totalBatches)
}

func TestDecimalPayloadBatchRejectsUnboundedSourceAndOversizedRow(t *testing.T) {
	s := suite{Package: payloadAdmissionPackage, Activity: payloadAdmissionActivity, Cases: []testCase{{Inputs: []any{strings.Repeat("\x00", 1024), int64(1), int64(0), int64(0), false, true, true}, Expected: map[string]any{"input": strings.Repeat("\x00", 1024)}}}}
	w := caseProfileTestWorkspace(s)
	policy, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.prepare(s, 0); err != nil {
		t.Fatal("valid escaped boundary rejected", err)
	}
	s.Cases[0].Inputs[0] = strings.Repeat("\x00", 30000)
	if _, err := policy.prepare(s, 0); err == nil {
		t.Fatal("oversized row accepted")
	}
	for _, start := range []int{-1, 1} {
		if _, err := policy.prepare(s, start); err == nil {
			t.Fatal("invalid offset accepted")
		}
	}
	w.Sources[w.Names[0]] = nil
	if _, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock()); err == nil {
		t.Fatal("empty source accepted")
	}
	if _, err := nativeCaseProfile("").policy(s, caseProfileTestWorkspace(s), upstream{}); err == nil {
		t.Fatal("unpinned lowering accepted")
	}
}
