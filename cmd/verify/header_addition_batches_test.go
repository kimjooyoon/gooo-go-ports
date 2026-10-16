package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestHeaderAdditionPreservesAllEarlier73Workspaces(t *testing.T) {
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	prior := f
	prior.Catalog.Packages = nil
	for _, p := range f.Catalog.Packages {
		if p.Path != headerAdditionPackage {
			prior.Catalog.Packages = append(prior.Catalog.Packages, p)
		}
	}
	seen := 0
	for key := range nativeActivityContracts() {
		parts := strings.SplitN(key, ":", 2)
		s := suite{Package: parts[0], Activity: parts[1]}
		if isHeaderAdditionSuite(s) {
			continue
		}
		seen++
		before, e := prior.workspaceForSuite(s)
		if e != nil {
			t.Fatal(key, e)
		}
		after, e := f.workspaceForSuite(s)
		if e != nil {
			t.Fatal(key, e)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("new composition changed earlier workspace", key)
		}
		if _, exists := after.Sources[headerAdditionSource]; exists {
			t.Fatal("addition source leaked into old suite", key)
		}
	}
	if seen != 73 {
		t.Fatal("original73 contract population changed", seen)
	}
}

func TestHeaderAdditionFixedEightPreservesAll50AndActualGoWireBudgets(t *testing.T) {
	suites, err := headerAdditionCorpus("../..")
	if err != nil || len(suites) != 1 {
		t.Fatal("one addition suite", err)
	}
	s := suites[0]
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Names) != 5 || !reflect.DeepEqual(w.Manifest.Packages, headerAdditionPackages()) {
		t.Fatal("whole-package closure", w.Names)
	}
	p, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []nativeCaseProfile{uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
		other, e := profile.policy(s, w, caseProfileTestLock())
		if e != nil || other != p {
			t.Fatal("existing profile changed new fixed-eight policy", e)
		}
	}
	covered, batches, maxRaw, maxTranslated := []testCase{}, 0, 0, 0
	for start := 0; start < len(s.Cases); batches++ {
		part, e := p.prepare(s, start)
		if e != nil {
			t.Fatal(e)
		}
		if part.End != min(start+8, 50) {
			t.Fatal("fixed8 boundary changed", start, part.End)
		}
		batch := s
		batch.Cases = s.Cases[start:part.End]
		pretty, e := json.MarshalIndent(caseDocument(batch), "", "  ")
		if e != nil || !bytes.Equal(part.Raw, append(pretty, '\n')) || !equalValue(part.Raw, caseDocument(batch)) {
			t.Fatal("original rows/types/expectations changed", e)
		}
		maxName := "GoooPackage" + strconv.Itoa(p.sourceBytes) + "Activity" + s.Activity
		translated, e := json.Marshal(pathAliasTranslatedDocument(batch, maxName))
		if e != nil || len(part.Raw) > 30000 || len(translated) > 32768 {
			t.Fatal("actual Go wire bound", len(part.Raw), len(translated), e)
		}
		maxRaw, maxTranslated = max(maxRaw, len(part.Raw)), max(maxTranslated, len(translated))
		covered = append(covered, batch.Cases...)
		start = part.End
	}
	if len(covered) != 50 || batches != 7 || !reflect.DeepEqual(covered, s.Cases) {
		t.Fatal("omitted/reordered original50", len(covered), batches)
	}
	t.Logf("addition actual-Go source partition cases=50 batches=7 max_raw=%d max_translated=%d native_calls=0 saved_replays=0", maxRaw, maxTranslated)
	for _, start := range []int{-1, 1, 7, 49, 50} {
		if _, e := p.prepare(s, start); e == nil {
			t.Fatal("non-boundary accepted", start)
		}
	}
	// A fixed batch must refuse a wire-budget violation, never resize or drop a
	// row merely because it has a large serialized independent expectation.
	oversized := s
	oversized.Cases = append([]testCase(nil), s.Cases...)
	oversized.Cases[0].Expected = map[string]any{"candidate_value": strings.Repeat("x", 31000)}
	if _, e := p.prepare(oversized, 0); e == nil {
		t.Fatal("oversized fixed batch accepted")
	}
	for _, mutate := range []func(*suiteWorkspace){func(w *suiteWorkspace) { delete(w.Sources, headerAdditionSource) }, func(w *suiteWorkspace) {
		w.Manifest.Packages[0], w.Manifest.Packages[1] = w.Manifest.Packages[1], w.Manifest.Packages[0]
	}} {
		fresh, e := f.workspaceForSuite(s)
		if e != nil {
			t.Fatal(e)
		}
		mutate(&fresh)
		if _, e := headerAdditionBatchPolicy(s, fresh, caseProfileTestLock()); e == nil {
			t.Fatal("broken selected closure accepted")
		}
	}
}
