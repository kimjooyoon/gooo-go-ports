package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestPathAliasCatalogPreservesAllEarlier68WorkspaceClosures(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	prior := frozen
	prior.Catalog.Packages = nil
	for _, pkg := range frozen.Catalog.Packages {
		if pkg.Path != pathCleanPackage && pkg.Path != pathAliasPackage {
			prior.Catalog.Packages = append(prior.Catalog.Packages, pkg)
		}
	}
	// Workspace selection needs entry identities, not another generation of the
	// original160266-case oracle populations.
	old := []suite{}
	for key := range nativeActivityContracts() {
		parts := strings.SplitN(key, ":", 2)
		s := suite{Package: parts[0], Activity: parts[1]}
		if !isPathAliasSuite(s) && !isHeaderSuite(s) && !isHeaderAdditionSuite(s) && !isCanonicalHeaderSuite(s) && !isHTTPConditionalSuite(s) {
			old = append(old, s)
		}
	}
	if len(old) != 68 {
		t.Fatal("earlier contract roster changed", len(old))
	}
	for _, s := range old {
		before, err := prior.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		after, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("new catalog changed an earlier workspace/source identity", s.Package, s.Activity)
		}
		for _, name := range []string{"ports/path/path.gooo", "ports/path/clean.gooo", "examples/path-alias-facts/main.gooo"} {
			if _, ok := after.Sources[name]; ok {
				t.Fatal("new path source leaked into earlier workspace", name)
			}
		}
	}
}

func TestPathAliasWorkspacesPreserveAcceptedSourcesAndExactClosures(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	wantHashes := map[string]string{
		"ports/path/path.gooo":                "sha256:593caad2ac3b443d353775a9f4e168282af1ab70181bcbc483c4b379c9ee3de3",
		"ports/path/clean.gooo":               "sha256:8430fc094b000fbd25b9d081408a9e8cdb59fca942f7a5541e7695c0291b790d",
		"examples/path-alias-facts/main.gooo": "sha256:5b0c8436bf95c923730af71a316d3897839d734c53c0223fffad5b555488182e",
	}
	for _, entry := range []suite{{Package: pathCleanPackage, Activity: pathCleanActivity}, {Package: pathAliasPackage, Activity: pathAliasActivity}} {
		w, err := frozen.workspaceForSuite(entry)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"ports/path/path.gooo", "ports/path/clean.gooo"}
		if entry.Package == pathAliasPackage {
			want = append(want, "examples/path-alias-facts/main.gooo")
		}
		if !reflect.DeepEqual(w.Names, want) || len(w.Sources) != len(want) || len(w.Identities) != len(want) {
			t.Fatal("new path source closure differs", w.Names)
		}
		for _, name := range want {
			if contentDigest(w.Sources[name]) != wantHashes[name] {
				t.Fatal("accepted candidate bytes changed", name)
			}
		}
	}
	// The positive selection above must work before these intended closure refusals.
	for _, mutate := range []func(*frozenVerificationCatalog){
		func(f *frozenVerificationCatalog) { delete(f.Sources, "ports/path/clean.gooo") },
		func(f *frozenVerificationCatalog) { f.Sources["ports/path/clean.gooo"] = []byte("altered") },
	} {
		f, err := freezeVerificationCatalog("../../verification-catalog.json")
		if err != nil {
			t.Fatal(err)
		}
		mutate(&f)
		if _, err := f.workspaceForSuite(suite{Package: pathCleanPackage, Activity: pathCleanActivity}); err == nil {
			t.Fatal("unbound path source accepted")
		}
	}
}

// Independently construct a worst-width translated case document. This verifies
// real JSON key expansion and unchanged values, rather than restating the bound.
func pathAliasTranslatedDocument(s suite, maxKey string) any {
	rows := []any{}
	for _, c := range s.Cases {
		inputs := map[string]any{}
		for i, value := range c.Inputs {
			key := maxKey
			if len(c.Inputs) > 1 {
				key += ".input" + strconv.Itoa(i)
			}
			inputs[key] = value
		}
		rows = append(rows, map[string]any{"inputs": inputs, "expected": map[string]any{maxKey: c.Expected}})
	}
	return map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": rows}
}

func TestPathAliasPartitionsPreserveEveryOrderedCaseAndActualWireBounds(t *testing.T) {
	suites, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	totalCases, totalBatches := 0, 0
	for _, s := range suites {
		w, err := catalog.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		p, err := uuidLiteralReportCompactCaseProfile.policy(s, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range []nativeCaseProfile{"", uuidCompactCaseProfile} {
			other, err := profile.policy(s, w, caseProfileTestLock())
			if err != nil || other != p {
				t.Fatal("unrelated case profile changed new path policy", err)
			}
		}
		batches, maxRaw, maxTranslated := 0, 0, 0
		for start := 0; start < len(s.Cases); batches++ {
			prepared, err := p.prepare(s, start)
			if err != nil || prepared.End <= start || prepared.End > len(s.Cases) || prepared.End-start > 128 || len(prepared.Raw) > 30000 {
				t.Fatal("invalid path partition", start, prepared.End, err)
			}
			part := s
			part.Cases = s.Cases[start:prepared.End]
			pretty, err := json.MarshalIndent(caseDocument(part), "", "  ")
			if err != nil || !bytes.Equal(prepared.Raw, append(pretty, '\n')) {
				t.Fatal("writer changed original cases/expected values", err)
			}
			maxName := "GoooPackage" + strconv.Itoa(p.sourceBytes) + "Activity" + s.Activity
			translated, err := json.Marshal(pathAliasTranslatedDocument(part, maxName))
			if err != nil || len(translated) > 32768 {
				t.Fatal("actual worst-width translated document exceeds bound", len(translated), err)
			}
			maxRaw, maxTranslated = max(maxRaw, len(prepared.Raw)), max(maxTranslated, len(translated))
			start = prepared.End
		}
		t.Logf("path-source-partition entry=%s:%s cases=%d batches=%d max_raw=%d max_translated=%d native_calls=0 saved_replays=0", s.Package, s.Activity, len(s.Cases), batches, maxRaw, maxTranslated)
		totalCases, totalBatches = totalCases+len(s.Cases), totalBatches+batches
	}
	if len(suites) != 2 || totalCases != 9961 {
		t.Fatal("path population changed")
	}
	t.Logf("proposed-full70 suites=70 cases=170227 batches=%d timing_rows=%d source_partition_only=true native_calls=0 saved_replays=0", 2072+totalBatches, 2214+totalBatches)
}

func TestPathAliasBatchRejectsWrongEntryPinsSourceArityAndOversizedRow(t *testing.T) {
	s := suite{Package: pathAliasPackage, Activity: pathAliasActivity, Cases: []testCase{{Inputs: []any{"a/..", "."}, Expected: map[string]any{"left": "a/..", "right": "."}}}}
	w := caseProfileTestWorkspace(s)
	p, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.prepare(s, 0); err != nil {
		t.Fatal("valid baseline refused", err)
	}
	for _, start := range []int{-1, 1} {
		if _, err := p.prepare(s, start); err == nil {
			t.Fatal("bad interval accepted")
		}
	}
	for _, bad := range []suite{{Package: pathCleanPackage, Activity: "Report"}, {Package: pathAliasPackage, Activity: "Clean"}} {
		if _, err := pathAliasBatchPolicy(bad, w, caseProfileTestLock()); err == nil {
			t.Fatal("different entry accepted")
		}
	}
	if _, err := pathAliasBatchPolicy(s, w, upstream{}); err == nil {
		t.Fatal("unpinned lowering accepted")
	}
	delete(w.Sources, w.Names[0])
	if _, err := pathAliasBatchPolicy(s, w, caseProfileTestLock()); err == nil {
		t.Fatal("missing source accepted")
	}
	s.Cases[0].Inputs = []any{"a"}
	if _, err := p.prepare(s, 0); err == nil {
		t.Fatal("wrong input arity accepted")
	}
	s.Cases[0].Inputs = []any{strings.Repeat("\x00", 30000), "a"}
	if _, err := p.prepare(s, 0); err == nil {
		t.Fatal("oversized single row accepted")
	}
}
