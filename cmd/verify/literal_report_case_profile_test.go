package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Called only for Report during the existing single full66 traversal. Keep
// every original Text byte, both input keys, record value and order explicit.
func checkLiteralReportCompactBatch(t *testing.T, s suite, start int, prepared nativePreparedCaseBatch, policy nativeCaseBatchPolicy) {
	t.Helper()
	if prepared.End <= start || prepared.End > len(s.Cases) || prepared.End-start > 128 || len(prepared.Raw) == 0 || len(prepared.Raw) > 30000 || len(prepared.Raw)-1 > 32768 || prepared.Raw[len(prepared.Raw)-1] != '\n' {
		t.Fatal("literal Report compact bounds")
	}
	part := s
	part.Cases = s.Cases[start:prepared.End]
	want, err := json.Marshal(caseDocument(part))
	if err != nil || !bytes.Equal(prepared.Raw, append(want, '\n')) {
		t.Fatal("literal Report sole writer bytes", err)
	}
	// Reuse the existing independent translation-shaped struct encoder.
	lowered := "GoooPackage" + strings.Repeat("9", len(fmt.Sprint(policy.sourceBytes))) + "Activity" + s.Activity
	translated := translatedDecimalPayloadBytes(t, part, lowered)
	if translated > len(prepared.Raw)-1 || translated > 32768 {
		t.Fatal("source-bound nonexpansion proof differs", translated, len(prepared.Raw)-1)
	}
	var wire struct {
		Schema string
		Cases  []struct{ Inputs, Expected map[string]json.RawMessage }
	}
	decoder := json.NewDecoder(bytes.NewReader(prepared.Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil || wire.Schema != "gooo/body-composition-cases/v1" || len(wire.Cases) != len(part.Cases) {
		t.Fatal("literal Report wire shape", err)
	}
	key := s.Package + ":" + s.Activity
	for i, row := range wire.Cases {
		if len(row.Inputs) != 2 || len(row.Expected) != 1 {
			t.Fatal("literal Report wire key counts")
		}
		for j, value := range part.Cases[i].Inputs {
			token, err := json.Marshal(value)
			if err != nil || !bytes.Equal(row.Inputs[fmt.Sprintf("%s.input%d", key, j)], token) {
				t.Fatal("original Text/input order changed", start+i, j, err)
			}
		}
		token, err := json.Marshal(part.Cases[i].Expected)
		if err != nil || !bytes.Equal(row.Expected[key], token) {
			t.Fatal("original independent record changed", start+i, err)
		}
	}
	if prepared.End < len(s.Cases) && prepared.End-start < 128 {
		part.Cases = s.Cases[start : prepared.End+1]
		next, err := json.Marshal(caseDocument(part))
		if err != nil || len(next)+1 <= 30000 {
			t.Fatal("literal Report prefix not largest fitting", err)
		}
	}
}

func literalReportProfileTestSuite() suite {
	s := suite{Package: literalTextConsumerPackage, Activity: "Report"}
	// Deliberate duplicate pairs verify that the wire policy never deduplicates.
	for _, pair := range []literalTextPair{{"", ""}, {"\x00\r\n\"\\<>&\u2028\u2029", "\\"}, {"가🙂", "🙂"}, {"same", "a"}, {"same", "a"}, {strings.Repeat("🙂", 256), ""}} {
		s.Cases = append(s.Cases, testCase{Inputs: []any{pair.Text, pair.Needle}, Expected: literalTextReportOracle(pair.Text, pair.Needle)})
	}
	return s
}

func TestLiteralReportCompactProfileExplicitScopeAndOldPolicies(t *testing.T) {
	profile, err := parseNativeCaseProfile("uuid-and-literal-report-compact-v1", false)
	if err != nil || profile != uuidLiteralReportCompactCaseProfile {
		t.Fatal("new profile not explicit", profile, err)
	}
	if _, err := parseNativeCaseProfile(string(profile), true); err == nil {
		t.Fatal("new native profile accepted for browser fixtures")
	}
	for _, name := range []string{"uuid-and-literal-report-compact-v2", " uuid-and-literal-report-compact-v1", "UUID-AND-LITERAL-REPORT-COMPACT-V1"} {
		if _, err := parseNativeCaseProfile(name, false); err == nil {
			t.Fatal("unknown profile alias accepted", name)
		}
	}
	s := literalReportProfileTestSuite()
	for _, old := range []nativeCaseProfile{"", uuidCompactCaseProfile} {
		policy, err := old.policy(s, suiteWorkspace{}, caseProfileTestLock())
		if err != nil || policy != (nativeCaseBatchPolicy{}) {
			t.Fatal("old profile acquired Report compact behavior", old, policy, err)
		}
	}
	for _, entry := range []suite{{Package: literalTextPackage, Activity: "Index"}, {Package: literalTextPackage, Activity: "Count"}, {Package: literalTextConsumerPackage, Activity: "Other"}} {
		policy, err := profile.policy(entry, suiteWorkspace{}, caseProfileTestLock())
		if err != nil || policy != (nativeCaseBatchPolicy{}) {
			t.Fatal("new profile expanded beyond Report/UUID", entry, err)
		}
	}
	for _, entry := range []suite{caseProfileTestSuite(uuidStandardSample), {Package: uuidConsumerPackage, Activity: "Inspect", Cases: []testCase{{Inputs: []any{uuidStandardSample}, Expected: uuidInspectorOracle(uuidStandardSample)}}}, {Package: uuidParsePackage, Activity: "ParseCanonical", Cases: []testCase{{Inputs: []any{uuidStandardSample}, Expected: uuidParseCanonicalOracle(uuidStandardSample)}}}} {
		w := caseProfileTestWorkspace(entry)
		prior, err := uuidCompactCaseProfile.policy(entry, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		next, err := profile.policy(entry, w, caseProfileTestLock())
		if err != nil || prior != next {
			t.Fatal("UUID policy receiver changed", err)
		}
		a, err := prior.prepare(entry, 0)
		if err != nil {
			t.Fatal(err)
		}
		b, err := next.prepare(entry, 0)
		if err != nil || a.End != b.End || !bytes.Equal(a.Raw, b.Raw) {
			t.Fatal("UUID writer/partition changed", err)
		}
	}
	// Do not regenerate the357-case corpus here. Existing focused decimal
	// tests own it; representative receivers prove this dispatch is untouched.
	for _, entry := range []suite{{Package: decimalPackage, Activity: decimalActivity, Cases: []testCase{{Inputs: []any{"0"}, Expected: map[string]any{"input": "0", "value": int64(0), "parsed": true, "error_kind": int64(0)}}}}, {Package: payloadAdmissionPackage, Activity: payloadAdmissionActivity, Cases: []testCase{{Inputs: []any{"0", int64(1), int64(0), int64(0), false, true, true}, Expected: map[string]any{"reason": int64(0)}}}}} {
		w := caseProfileTestWorkspace(entry)
		prior, err := uuidCompactCaseProfile.policy(entry, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		next, err := profile.policy(entry, w, caseProfileTestLock())
		if err != nil || prior != next {
			t.Fatal("decimal/payload policy receiver changed", err)
		}
		a, err := prior.prepare(entry, 0)
		if err != nil {
			t.Fatal(err)
		}
		b, err := next.prepare(entry, 0)
		if err != nil || a.End != b.End || !bytes.Equal(a.Raw, b.Raw) {
			t.Fatal("decimal/payload wire/partition changed", err)
		}
	}
}

func TestLiteralReportCompactAdmissionWriterAndSourceBound(t *testing.T) {
	s := literalReportProfileTestSuite()
	before, err := json.Marshal(s.Cases)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := uuidLiteralReportCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := policy.prepare(s, 0)
	if err != nil || batch.End != len(s.Cases) {
		t.Fatal("valid pair/duplicate refusal", err)
	}
	checkLiteralReportCompactBatch(t, s, 0, batch, policy)
	path := filepath.Join(t.TempDir(), "cases.json")
	if err := writeNativeCaseBatch(path, s, batch); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, batch.Raw) {
		t.Fatal("writer reserialized sized bytes", err)
	}
	after, err := json.Marshal(s.Cases)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("policy mutated/normalized/deduplicated cases", err)
	}
	for _, bad := range [][]any{nil, {"one"}, {"a", "b", "extra"}, {"a", true}, {string([]byte{0xff}), ""}, {"", strings.Repeat("a", 1025)}} {
		changed := s
		changed.Cases = append([]testCase(nil), s.Cases...)
		changed.Cases[len(changed.Cases)-1].Inputs = bad
		if _, err := uuidLiteralReportCompactCaseProfile.policy(changed, caseProfileTestWorkspace(changed), caseProfileTestLock()); err == nil {
			t.Fatal("bad final pair admitted", bad)
		}
	}
	changed := s
	changed.Cases = append([]testCase(nil), s.Cases...)
	changed.Cases[0].Expected = "not a record"
	if _, err := uuidLiteralReportCompactCaseProfile.policy(changed, caseProfileTestWorkspace(changed), caseProfileTestLock()); err == nil {
		t.Fatal("wrong expected kind admitted")
	}
	for _, total := range []int{99999, 100000} {
		w := caseProfileTestWorkspace(s)
		w.Sources[w.Names[0]] = bytes.Repeat([]byte("x"), total)
		_, err := uuidLiteralReportCompactCaseProfile.policy(s, w, caseProfileTestLock())
		if (total == 99999 && err != nil) || (total == 100000 && err == nil) {
			t.Fatal("ordinal digit nonexpansion boundary", total, err)
		}
	}
	w := caseProfileTestWorkspace(s)
	w.Sources[w.Names[0]] = nil
	if _, err := uuidLiteralReportCompactCaseProfile.policy(s, w, caseProfileTestLock()); err == nil {
		t.Fatal("empty source proof admitted")
	}
	lock := caseProfileTestLock()
	lock.CompilerRevision = strings.Repeat("a", 40)
	if _, err := uuidLiteralReportCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), lock); err == nil {
		t.Fatal("unpinned compiler accepted")
	}
	for _, start := range []int{-1, len(s.Cases)} {
		if _, err := policy.prepare(s, start); err == nil {
			t.Fatal("invalid start accepted")
		}
	}
}

func TestLiteralReportCompactLFInclusiveCapAndRowCeiling(t *testing.T) {
	s := suite{Package: literalTextConsumerPackage, Activity: "Report", Cases: []testCase{{Inputs: []any{"", ""}, Expected: map[string]any{"synthetic": ""}}}}
	base, err := json.Marshal(caseDocument(s))
	if err != nil {
		t.Fatal(err)
	}
	s.Cases[0].Expected.(map[string]any)["synthetic"] = strings.Repeat("x", 30000-len(base)-1)
	policy, err := uuidLiteralReportCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := policy.prepare(s, 0)
	if err != nil || len(batch.Raw) != 30000 {
		t.Fatal("exact30000 inclLF refused", len(batch.Raw), err)
	}
	s.Cases[0].Expected.(map[string]any)["synthetic"] = s.Cases[0].Expected.(map[string]any)["synthetic"].(string) + "x"
	if _, err := policy.prepare(s, 0); err == nil {
		t.Fatal("30001 inclLF accepted")
	}
	// Synthetic empty expected records isolate the row ceiling from the byte
	// ceiling; they are not valid Report/native semantic acceptance.
	rows := literalReportProfileTestSuite()
	rows.Cases[0].Expected = map[string]any{}
	s = repeatNativeCase(rows, 129)
	policy, err = uuidLiteralReportCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	first, err := policy.prepare(s, 0)
	if err != nil || first.End != 128 {
		t.Fatal("128row ceiling differs", first.End, err)
	}
	last, err := policy.prepare(s, first.End)
	if err != nil || last.End != 129 {
		t.Fatal("last original row lost", last.End, err)
	}
}

func TestLiteralReportCompactMetadataIsNewAndOldIdentityUnchanged(t *testing.T) {
	oldBefore, err := json.Marshal(uuidCompactCaseProfile.identity())
	if err != nil {
		t.Fatal(err)
	}
	summary := map[string]any{"scope": "The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged."}
	header := map[string]any{"expected_boundaries_recomputed": false}
	uuidLiteralReportCompactCaseProfile.decorateSummary(summary)
	uuidLiteralReportCompactCaseProfile.decorateTimingHeader(header)
	identity := uuidLiteralReportCompactCaseProfile.identity()
	if identity["name"] != string(uuidLiteralReportCompactCaseProfile) || identity["basis_profile"] != string(uuidCompactCaseProfile) || !reflect.DeepEqual(identity["entries"], []string{uuidPackage + ":Validate", uuidConsumerPackage + ":Inspect", uuidParsePackage + ":ParseCanonical", literalTextConsumerPackage + ":Report"}) {
		t.Fatal("new profile identity/scope", identity)
	}
	if !reflect.DeepEqual(summary["case_profile"], identity) || !reflect.DeepEqual(header["case_profile"], identity) || summary["literal_report_compact_scope"] == nil || header["expected_boundaries_recomputed"] != false {
		t.Fatal("new metadata missing/recorder claim changed")
	}
	if strings.Contains(summary["scope"].(string), "Only UUID3") || !strings.Contains(header["boundary_scope"].(string), "No prior profile PASS") {
		t.Fatal("historical profile boundary/PASS inherited")
	}
	oldAfter, err := json.Marshal(uuidCompactCaseProfile.identity())
	if err != nil || !bytes.Equal(oldBefore, oldAfter) {
		t.Fatal("new identity mutated old UUID metadata", err)
	}
	legacy := map[string]any{"scope": "original", "boundary_scope": "original"}
	prior, _ := json.Marshal(legacy)
	nativeCaseProfile("").decorateSummary(legacy)
	nativeCaseProfile("").decorateTimingHeader(legacy)
	after, _ := json.Marshal(legacy)
	if !bytes.Equal(prior, after) {
		t.Fatal("legacy metadata changed")
	}
}
