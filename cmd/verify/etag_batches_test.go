package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func etagOriginal76(t *testing.T) []suite {
	t.Helper()
	all := canonicalHeaderOriginal74(t)
	canonical, err := canonicalHeaderCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	return append(all, canonical...)
}

func TestETagPreservesOriginal76WorkspaceAndProfilePolicies(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	prior := frozen
	prior.Catalog.Packages = nil
	for _, pkg := range frozen.Catalog.Packages {
		if pkg.Path != etagPortPackage && pkg.Path != etagReportPackage {
			prior.Catalog.Packages = append(prior.Catalog.Packages, pkg)
		}
	}
	old, cases := etagOriginal76(t), 0
	for _, s := range old {
		cases += len(s.Cases)
		for _, workspaceProfile := range []nativeWorkspaceProfile{"", dependencyClosureWorkspaceProfile} {
			before, err := workspaceProfile.workspaceForSuite(prior, s)
			if err != nil {
				t.Fatal(s.Package, s.Activity, err)
			}
			after, err := workspaceProfile.workspaceForSuite(frozen, s)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("ETag changed original workspace", workspaceProfile, s.Package, s.Activity, err)
			}
			for _, name := range []string{etagPortSource, etagReportSource} {
				if _, ok := after.Sources[name]; ok {
					t.Fatal("ETag source leaked into original76", name)
				}
			}
			for _, profile := range []nativeCaseProfile{"", uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
				a, err := profile.policy(s, before, caseProfileTestLock())
				if err != nil {
					t.Fatal(err)
				}
				b, err := profile.policy(s, after, caseProfileTestLock())
				if err != nil || a != b {
					t.Fatal("original profile batch policy changed", workspaceProfile, profile, s.Package, s.Activity, err)
				}
			}
		}
	}
	if len(old) != 76 || cases != 173335 {
		t.Fatal("original suite/case prefix changed", len(old), cases)
	}
	// Equal frozen workspaces, original cases and policies leave the original
	// partitioner inputs unchanged. This does not execute the seven exhaustive
	// dispatches or establish a new native/replay acceptance.
}

func TestETagGoWireBudgetsRetainAll247OrderedOccurrences(t *testing.T) {
	suites, err := etagCorpus("../..")
	if err != nil || len(suites) != 3 {
		t.Fatal("ETag three original suites required", err)
	}
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := []string{etagPortPackage + ":" + etagScanActivity, etagPortPackage + ":" + etagStrongActivity, etagReportPackage + ":" + etagReportActivity}
	wantRows := []int{192, 31, 24}
	total := 0
	for index, s := range suites {
		if s.Package+":"+s.Activity != wantEntries[index] || len(s.Cases) != wantRows[index] {
			t.Fatal("ETag finite population or suite order changed", index)
		}
		w, err := frozen.workspaceForSuite(s)
		if err != nil || !reflect.DeepEqual(w.Manifest.Packages, etagPackages(s)) {
			t.Fatal("ETag exact original manifest closure", err)
		}
		policy, err := nativeCaseProfile("").policy(s, w, caseProfileTestLock())
		if err != nil || !policy.etag {
			t.Fatal("ETag routing/policy missing", err)
		}
		for _, profile := range []nativeCaseProfile{uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
			other, err := profile.policy(s, w, caseProfileTestLock())
			if err != nil || other != policy {
				t.Fatal("ETag policy differs across case profiles", profile, err)
			}
		}
		closure, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s)
		if err != nil || !reflect.DeepEqual(w, closure) {
			t.Fatal("ETag exact closure differs under dependency profile", err)
		}
		covered := []testCase{}
		batches, maxRaw, maxTranslated := 0, 0, 0
		for start := 0; start < len(s.Cases); {
			prepared, err := policy.prepare(s, start)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.End <= start || prepared.End > len(s.Cases) || prepared.End-start > 128 {
				t.Fatal("ETag contiguous bounded prefix differs")
			}
			batch := s
			batch.Cases = s.Cases[start:prepared.End]
			pretty, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || !bytes.Equal(prepared.Raw, append(pretty, '\n')) || !equalValue(prepared.Raw, caseDocument(batch)) {
				t.Fatal("ETag actual Go wire bytes or original values differ", err)
			}
			maxName := "GoooPackage" + strconv.Itoa(policy.sourceBytes) + "Activity" + s.Activity
			translated, err := json.Marshal(pathAliasTranslatedDocument(batch, maxName))
			if err != nil || len(prepared.Raw) > 30000 || len(translated) > 32768 {
				t.Fatal("ETag actual Go raw/translated wire bound", err)
			}
			if etagBatchArity(s) == 2 {
				assertETagTwoInputKeys(t, batch, prepared.Raw)
			}
			// The next original row may not be omitted merely to make a batch
			// smaller. The selected prefix is maximal under the stated bound.
			if prepared.End < len(s.Cases) && prepared.End-start < 128 {
				next := s
				next.Cases = s.Cases[start : prepared.End+1]
				raw, err := json.MarshalIndent(caseDocument(next), "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				compact, err := json.Marshal(caseDocument(next))
				if err != nil {
					t.Fatal(err)
				}
				growth := max(0, policy.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
				upper := len(compact) + (etagBatchArity(s)+1)*len(next.Cases)*growth
				if len(raw)+1 <= 30000 && upper <= 32768 {
					t.Fatal("ETag nonmaximal prefix silently selected")
				}
			}
			maxRaw, maxTranslated = max(maxRaw, len(prepared.Raw)), max(maxTranslated, len(translated))
			covered = append(covered, batch.Cases...)
			start = prepared.End
			batches++
		}
		if !reflect.DeepEqual(covered, s.Cases) || len(covered) != wantRows[index] || batches < 1 {
			t.Fatal("ETag original occurrences omitted, duplicated or reordered", index)
		}
		total += len(covered)
		t.Logf("ETag actual-Go package=%s activity=%s rows=%d batches=%d max_raw=%d max_translated_upper=%d native_calls=0 saved_replays=0", s.Package, s.Activity, len(covered), batches, maxRaw, maxTranslated)
	}
	if total != 247 {
		t.Fatal("ETag full finite row population changed", total)
	}
}

func assertETagTwoInputKeys(t *testing.T, s suite, raw []byte) {
	t.Helper()
	var document struct {
		Cases []struct {
			Inputs map[string]json.RawMessage `json:"inputs"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &document); err != nil || len(document.Cases) != len(s.Cases) {
		t.Fatal("ETag two-input wire document", err)
	}
	base := s.Package + ":" + s.Activity
	for index, row := range document.Cases {
		if len(row.Inputs) != 2 {
			t.Fatal("ETag two original operands merged")
		}
		for operand := 0; operand < 2; operand++ {
			value, ok := row.Inputs[base+".input"+strconv.Itoa(operand)]
			if !ok || !equalValue(value, s.Cases[index].Inputs[operand]) {
				t.Fatal("ETag original operand binding differs", index, operand)
			}
		}
	}
}

func TestETagBatchesRefuseWrongDomainPinsClosureAndOversizedRows(t *testing.T) {
	suites, err := etagCorpus("../..")
	if err != nil || len(suites) != 3 {
		t.Fatal(err)
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
		policy, err := etagBatchPolicy(s, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		for _, start := range []int{-1, len(s.Cases)} {
			if _, err := policy.prepareETag(s, start); err == nil {
				t.Fatal("ETag invalid start accepted", start)
			}
		}
		valid := make([]any, etagBatchArity(s))
		for i := range valid {
			valid[i] = ""
		}
		for operand := range valid {
			for _, invalid := range []any{int64(1), nil, string([]byte{0xff}), strings.Repeat("a", 1025)} {
				values := append([]any(nil), valid...)
				values[operand] = invalid
				changed := s
				changed.Cases = []testCase{{Inputs: values, Expected: false}}
				if _, err := etagBatchPolicy(changed, w, caseProfileTestLock()); err == nil {
					t.Fatal("ETag invalid operand accepted", operand)
				}
			}
		}
		for _, values := range [][]any{nil, valid[:len(valid)-1], append(append([]any(nil), valid...), "extra")} {
			changed := s
			changed.Cases = []testCase{{Inputs: values, Expected: false}}
			if _, err := policy.prepareETag(changed, 0); err == nil {
				t.Fatal("ETag wrong original input arity accepted")
			}
		}
		changed := s
		changed.Cases = []testCase{{Inputs: valid, Expected: strings.Repeat("x", 31000)}}
		if _, err := policy.prepareETag(changed, 0); err == nil {
			t.Fatal("ETag oversized one-row case accepted or dropped")
		}
		for _, lock := range []upstream{{CompilerRevision: "wrong", GoVersion: "go1.27.3"}, {CompilerRevision: uuidCompactCompilerRevision, GoVersion: "go1.27.1"}} {
			if _, err := etagBatchPolicy(s, w, lock); err == nil {
				t.Fatal("ETag changed tool pin accepted")
			}
		}
		fresh, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		delete(fresh.Sources, etagPortSource)
		if _, err := etagBatchPolicy(s, fresh, caseProfileTestLock()); err == nil {
			t.Fatal("ETag missing source accepted")
		}
		fresh, err = frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		fresh.Sources[etagPortSource] = append(bytes.Clone(fresh.Sources[etagPortSource]), '\n')
		if _, err := etagBatchPolicy(s, fresh, caseProfileTestLock()); err == nil {
			t.Fatal("ETag altered source identity accepted")
		}
		if etagReportConsumer(s) {
			fresh, err = frozen.workspaceForSuite(s)
			if err != nil {
				t.Fatal(err)
			}
			fresh.Manifest.Packages[0], fresh.Manifest.Packages[1] = fresh.Manifest.Packages[1], fresh.Manifest.Packages[0]
			if _, err := etagBatchPolicy(s, fresh, caseProfileTestLock()); err == nil {
				t.Fatal("ETag original manifest order changed")
			}
		}
	}
}

func TestETagWorkspaceRejectsWrongCatalogClosureAndImports(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	report := suite{Package: etagReportPackage, Activity: etagReportActivity}
	// Copy only the mutable catalog/source containers for these negative
	// controls; the reviewed frozen snapshot must not be changed by a test.
	copyCatalog := func() frozenVerificationCatalog {
		copy := frozen
		copy.Catalog.Packages = nil
		for _, pkg := range frozen.Catalog.Packages {
			copy.Catalog.Packages = append(copy.Catalog.Packages, sourcePackage{Path: pkg.Path, Sources: append([]string(nil), pkg.Sources...)})
		}
		copy.Sources, copy.Hashes = map[string][]byte{}, map[string]string{}
		for name, raw := range frozen.Sources {
			copy.Sources[name], copy.Hashes[name] = bytes.Clone(raw), frozen.Hashes[name]
		}
		return copy
	}
	for _, control := range []string{"missing-package", "duplicate-package", "extra-source", "missing-source", "changed-source", "unexpected-import", "wrong-import-order"} {
		t.Run(control, func(t *testing.T) {
			changed := copyCatalog()
			for index, pkg := range changed.Catalog.Packages {
				if pkg.Path != etagPortPackage {
					continue
				}
				switch control {
				case "missing-package":
					changed.Catalog.Packages = append(changed.Catalog.Packages[:index], changed.Catalog.Packages[index+1:]...)
				case "duplicate-package":
					changed.Catalog.Packages = append(changed.Catalog.Packages, pkg)
				case "extra-source":
					changed.Catalog.Packages[index].Sources = append(changed.Catalog.Packages[index].Sources, headerPortSource)
				}
				break
			}
			switch control {
			case "missing-source":
				delete(changed.Sources, etagPortSource)
			case "changed-source":
				changed.Sources[etagPortSource] = append(changed.Sources[etagPortSource], '\n')
			case "unexpected-import":
				changed.Sources[etagPortSource] = append(changed.Sources[etagPortSource], []byte("\nimport extra \"go/net/http\"\n")...)
				changed.Hashes[etagPortSource] = strings.TrimPrefix(contentDigest(changed.Sources[etagPortSource]), "sha256:")
			case "wrong-import-order":
				old := "import etags \"" + etagPortPackage + "\"\nimport headers \"" + headerPortPackage + "\""
				replacement := "import headers \"" + headerPortPackage + "\"\nimport etags \"" + etagPortPackage + "\""
				raw := changed.Sources[etagReportSource]
				if bytes.Count(raw, []byte(old)) != 1 {
					t.Fatal("original Report import fixture differs")
				}
				changed.Sources[etagReportSource] = bytes.Replace(raw, []byte(old), []byte(replacement), 1)
				changed.Hashes[etagReportSource] = strings.TrimPrefix(contentDigest(changed.Sources[etagReportSource]), "sha256:")
			}
			if _, err := changed.etagWorkspace(report); err == nil {
				t.Fatal("ETag invalid frozen closure accepted", control)
			}
		})
	}
}
