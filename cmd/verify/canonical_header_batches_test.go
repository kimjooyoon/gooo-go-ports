package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func canonicalHeaderOriginal74(t *testing.T) []suite {
	t.Helper()
	all := allNativeSuites(t)
	decimal, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	all = append(all, decimal.Suites...)
	paths, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	all = append(all, paths...)
	headers, err := headerCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	all = append(all, headers...)
	addition, err := headerAdditionCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	all = append(all, addition...)
	return all
}
func TestCanonicalHeaderPreservesAllOriginal74WorkspaceAndProfilePolicies(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	prior := frozen
	prior.Catalog.Packages = nil
	for _, pkg := range frozen.Catalog.Packages {
		if pkg.Path != canonicalHeaderPackage && pkg.Path != canonicalHeaderReportPackage {
			prior.Catalog.Packages = append(prior.Catalog.Packages, pkg)
		}
	}
	old := canonicalHeaderOriginal74(t)
	cases := 0
	for _, s := range old {
		cases += len(s.Cases)
		before, err := prior.workspaceForSuite(s)
		if err != nil {
			t.Fatal(s.Package, s.Activity, err)
		}
		after, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(s.Package, s.Activity, err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("new canonical package changed original workspace", s.Package, s.Activity)
		}
		for _, name := range []string{canonicalHeaderSource, canonicalHeaderReportSource} {
			if _, ok := after.Sources[name]; ok {
				t.Fatal("canonical source leaked into original74", name)
			}
		}
		for _, profile := range []nativeCaseProfile{"", uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
			a, err := profile.policy(s, before, caseProfileTestLock())
			if err != nil {
				t.Fatal(err)
			}
			b, err := profile.policy(s, after, caseProfileTestLock())
			if err != nil || a != b {
				t.Fatal("original profile batch policy changed", profile, s.Package, s.Activity, err)
			}
		}
	}
	if len(old) != 74 || cases != 171217 {
		t.Fatal("original suite/case prefix changed", len(old), cases)
	}
	// Seven exhaustive dispatches stay in the unchanged native_suite.go and
	// are not suppressed or inferred from these source-only policy controls.
}

func TestCanonicalHeaderActualGoBudgetsPreserveEveryOriginalRow(t *testing.T) {
	suites, err := canonicalHeaderCorpus("../..")
	if err != nil || len(suites) != 2 {
		t.Fatal("canonical scalar+Report suites", err)
	}
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range suites {
		w, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(w.Manifest.Packages, canonicalHeaderPackages(s)) {
			t.Fatal("exact source/import closure")
		}
		policy, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range []nativeCaseProfile{uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
			other, err := profile.policy(s, w, caseProfileTestLock())
			if err != nil || other != policy {
				t.Fatal("canonical policy differs across profiles", profile, err)
			}
		}
		covered := []testCase{}
		batches, maxRaw, maxTranslated := 0, 0, 0
		for start := 0; start < len(s.Cases); {
			prepared, err := policy.prepare(s, start)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.End <= start || prepared.End > len(s.Cases) || prepared.End-start > 128 {
				t.Fatal("contiguous bounded range")
			}
			batch := s
			batch.Cases = s.Cases[start:prepared.End]
			pretty, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || !bytes.Equal(prepared.Raw, append(pretty, '\n')) || !equalValue(prepared.Raw, caseDocument(batch)) {
				t.Fatal("actual Go bytes or original values changed", err)
			}
			// Independent translated document uses an ordinal with the proven
			// maximum decimal width, not a guessed or private LoweredName.
			maxName := "GoooPackage" + strconv.Itoa(policy.sourceBytes) + "Activity" + s.Activity
			translated, err := json.Marshal(pathAliasTranslatedDocument(batch, maxName))
			if err != nil || len(prepared.Raw) > 30000 || len(translated) > 32768 {
				t.Fatal("actual Go raw/translated bounds", err)
			}
			maxRaw, maxTranslated = max(maxRaw, len(prepared.Raw)), max(maxTranslated, len(translated))
			covered = append(covered, batch.Cases...)
			start = prepared.End
			batches++
		}
		if !reflect.DeepEqual(covered, s.Cases) || len(covered) != canonicalHeaderRawCases || batches < 1 || batches >= len(s.Cases) {
			t.Fatal("all1059 original occurrences, generic non-singleton grouping", len(covered), batches)
		}
		t.Logf("canonical-header actual-Go package=%s activity=%s cases=%d batches=%d max_raw=%d max_translated_upper=%d native_calls=0 saved_replays=0", s.Package, s.Activity, len(covered), batches, maxRaw, maxTranslated)
		for _, start := range []int{-1, len(s.Cases)} {
			if _, err := policy.prepare(s, start); err == nil {
				t.Fatal("invalid start accepted", start)
			}
		}
		oversized := s
		oversized.Cases = []testCase{{Inputs: []any{"x"}, Expected: strings.Repeat("x", 31000)}}
		if _, err := policy.prepare(oversized, 0); err == nil {
			t.Fatal("oversized single occurrence accepted or dropped")
		}
		for _, input := range [][]any{nil, {"a", "b"}, {int64(1)}, {nil}, {string([]byte{0xff})}, {strings.Repeat("a", 1025)}} {
			changed := s
			changed.Cases = []testCase{{Inputs: input, Expected: ""}}
			if _, err := policy.prepare(changed, 0); err == nil {
				t.Fatal("invalid Text precondition accepted", input)
			}
		}
		fresh, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		delete(fresh.Sources, canonicalHeaderSource)
		if _, err := canonicalHeaderBatchPolicy(s, fresh, caseProfileTestLock()); err == nil {
			t.Fatal("missing canonical source accepted")
		}
		fresh, err = frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		fresh.Manifest.Packages[0], fresh.Manifest.Packages[1] = fresh.Manifest.Packages[1], fresh.Manifest.Packages[0]
		if _, err := canonicalHeaderBatchPolicy(s, fresh, caseProfileTestLock()); err == nil {
			t.Fatal("wrong package order accepted")
		}
	}
}
