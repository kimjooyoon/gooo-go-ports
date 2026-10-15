package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func caseProfileTestLock() upstream {
	return upstream{CompilerRevision: uuidCompactCompilerRevision, GoVersion: "go1.27.2"}
}

func caseProfileTestWorkspace(s suite) suiteWorkspace {
	const name = "ports/profile-test.gooo"
	w := suiteWorkspace{Names: []string{name}, Sources: map[string][]byte{name: bytes.Repeat([]byte("x"), 128)}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	w.Manifest.Packages = []sourcePackage{{Path: s.Package, Sources: []string{name}}}
	return w
}

func caseProfileTestSuite(texts ...string) suite {
	s := suite{Package: uuidPackage, Activity: "Validate"}
	for _, text := range texts {
		s.Cases = append(s.Cases, testCase{Inputs: []any{text}, Expected: uuidValidationOracle(text)})
	}
	return s
}

func TestNativeCaseProfileParserAndUnknownBeforeAnyOutput(t *testing.T) {
	for _, value := range []string{"", "legacy"} {
		got, err := parseNativeCaseProfile(value, false)
		if err != nil || got != "" {
			t.Fatal("legacy alias did not preserve zero profile", value, got, err)
		}
	}
	if got, err := parseNativeCaseProfile("uuid-compact-v1", false); err != nil || got != uuidCompactCaseProfile {
		t.Fatal("explicit profile refused", got, err)
	}
	if _, err := parseNativeCaseProfile("uuid-compact-v1", true); err == nil {
		t.Fatal("compact profile silently affected browser fixtures")
	}
	for _, value := range []string{"UUID-COMPACT-V1", " uuid-compact-v1", "uuid-compact-v2", "unknown"} {
		if _, err := parseNativeCaseProfile(value, false); err == nil {
			t.Fatal("unknown profile accepted", value)
		}
	}
	// No upstream.json/compiler exists here: another error means unrelated
	// I/O happened before profile validation. Do not run this test in parallel.
	root := t.TempDir()
	t.Chdir(root)
	target := filepath.Join(root, "uncreated-parent", "evidence")
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	flag.CommandLine = flag.NewFlagSet("verify-case-profile-test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(&bytes.Buffer{})
	os.Args = []string{"verify", "--out", target, "--case-profile", "unknown"}
	if err := run(); err == nil || !strings.Contains(err.Error(), "unknown --case-profile") {
		t.Fatal("profile was not refused before upstream/output work", err)
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatal("unknown profile created output parent", err)
	}
}

func TestNativeCaseProfileLegacyWriterBytesAndMetadataAbsence(t *testing.T) {
	s := caseProfileTestSuite("<>&", "\x00\r\n", "가🙂")
	policy, err := nativeCaseProfile("").policy(s, suiteWorkspace{}, upstream{})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := policy.prepare(s, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cases.json")
	if err := writeNativeCaseBatch(path, s, prepared); err != nil {
		t.Fatal(err)
	}
	want, err := json.MarshalIndent(caseDocument(s), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, append(want, '\n')) {
		t.Fatal("default exact indented wire bytes changed", err)
	}
	for _, decorate := range []func(map[string]any){nativeCaseProfile("").decorateSummary, nativeCaseProfile("").decorateTimingHeader} {
		document := map[string]any{"scope": "legacy text", "boundary_scope": "legacy boundaries", "count": int64(2337)}
		before, _ := json.Marshal(document)
		decorate(document)
		after, _ := json.Marshal(document)
		if !bytes.Equal(before, after) {
			t.Fatal("legacy metadata/optional-field absence changed", string(after))
		}
	}
}

// checkNativeCaseProfileUUIDPreparedBatch retains every original wire/value
// assertion while the full66 test owns the only compact partition traversal.
func checkNativeCaseProfileUUIDPreparedBatch(t *testing.T, s suite, start int, prepared nativePreparedCaseBatch) {
	t.Helper()
	if prepared.End <= start || prepared.End > len(s.Cases) || prepared.End-start > 128 || len(prepared.Raw) == 0 || len(prepared.Raw) > 30000 ||
		len(prepared.Raw)-1 > 32768 || prepared.Raw[len(prepared.Raw)-1] != '\n' {
		t.Fatal("compact interval is outside the declared budget")
	}
	batch := s
	batch.Cases = s.Cases[start:prepared.End]
	exactRaw, err := json.Marshal(caseDocument(batch))
	if err != nil || !bytes.Equal(prepared.Raw, append(exactRaw, '\n')) {
		t.Fatal("sizer/writer byte identity changed", err)
	}
	var decoded struct {
		Schema string
		Cases  []struct {
			Inputs   map[string]json.RawMessage
			Expected map[string]json.RawMessage
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(prepared.Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || len(decoded.Cases) != len(batch.Cases) ||
		decoded.Schema != "gooo/body-composition-cases/v1" {
		t.Fatal("strict compact document shape", err)
	}
	key := s.Package + ":" + s.Activity
	for i, row := range decoded.Cases {
		input, _ := json.Marshal(batch.Cases[i].Inputs[0])
		expected, _ := json.Marshal(batch.Cases[i].Expected)
		if len(row.Inputs) != 1 || len(row.Expected) != 1 ||
			!bytes.Equal(row.Inputs[key], input) || !bytes.Equal(row.Expected[key], expected) {
			t.Fatal("input/order/expected value changed", key, start+i)
		}
	}
	if prepared.End < len(s.Cases) && prepared.End-start < 128 {
		next := s
		next.Cases = s.Cases[start : prepared.End+1]
		raw, _ := json.Marshal(caseDocument(next))
		if len(raw)+1 <= 30000 {
			t.Fatal("selected prefix is not largest fitting prefix")
		}
	}
}

func TestNativeCaseProfileCompactDuplicateEscapesAndRefusals(t *testing.T) {
	s := caseProfileTestSuite("", "\x00\r\n\"\\<>&", "가🙂", "duplicate", "duplicate", strings.Repeat("a", 1024))
	original, _ := json.Marshal(s.Cases)
	policy, err := uuidCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := policy.prepare(s, 0)
	if err != nil || batch.End != len(s.Cases) {
		t.Fatal("valid boundaries/duplicates rejected", err)
	}
	after, _ := json.Marshal(s.Cases)
	if !bytes.Equal(original, after) {
		t.Fatal("policy normalized or mutated original cases")
	}
	for _, bad := range [][]any{nil, {int64(1)}, {"a", "b"}, {string([]byte{0xff})}, {strings.Repeat("a", 1025)}} {
		broken := s
		broken.Cases = append([]testCase(nil), s.Cases...)
		broken.Cases[len(broken.Cases)-1].Inputs = bad
		if _, err := uuidCompactCaseProfile.policy(broken, caseProfileTestWorkspace(broken), caseProfileTestLock()); err == nil {
			t.Fatal("bad late input passed admission", bad)
		}
	}
	for _, start := range []int{-1, len(s.Cases)} {
		if _, err := policy.prepare(s, start); err == nil {
			t.Fatal("invalid interval accepted")
		}
	}
	tooLarge := caseProfileTestSuite("x")
	tooLarge.Cases[0].Expected.(map[string]any)["error"] = strings.Repeat("x", 30000)
	policy, err = uuidCompactCaseProfile.policy(tooLarge, caseProfileTestWorkspace(tooLarge), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.prepare(tooLarge, 0); err == nil {
		t.Fatal("one oversize case silently split/truncated/fell back")
	}
	lock := caseProfileTestLock()
	lock.CompilerRevision = strings.Repeat("a", 40)
	if _, err := uuidCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), lock); err == nil {
		t.Fatal("unproved compiler lowering accepted")
	}
	w := caseProfileTestWorkspace(s)
	w.Sources[w.Names[0]] = make([]byte, uuidCompactMaxSourceBytes+1)
	if _, err := uuidCompactCaseProfile.policy(s, w, caseProfileTestLock()); err == nil {
		t.Fatal("source-byte proof budget ignored")
	}
	inspector := s
	inspector.Package, inspector.Activity = uuidConsumerPackage, "Inspect"
	for i := range inspector.Cases {
		inspector.Cases[i].Expected = uuidInspectorOracle(inspector.Cases[i].Inputs[0].(string))
	}
	w = caseProfileTestWorkspace(inspector)
	w.Sources[w.Names[0]] = make([]byte, uuidCompactMaxSourceBytes)
	if _, err := uuidCompactCaseProfile.policy(inspector, w, caseProfileTestLock()); err == nil {
		t.Fatal("potential lowered-key expansion accepted")
	}
}

func TestNativeCaseProfileCompactMetadataDoesNotInheritLegacyBounds(t *testing.T) {
	summary := map[string]any{
		"scope":             "The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged.",
		"uuid_parse_scope":  "All prior 56 suites, 108835 cases, 1416 batch boundaries and source rosters remain unchanged.",
		"containment_scope": "Original 57 suites, 130702 case documents and 1741 batch boundaries remain unchanged.",
	}
	uuidCompactCaseProfile.decorateSummary(summary)
	header := map[string]any{"boundary_scope": "old original boundaries", "expected_boundaries_recomputed": false}
	uuidCompactCaseProfile.decorateTimingHeader(header)
	if !reflect.DeepEqual(summary["case_profile"], header["case_profile"]) {
		t.Fatal("summary/timing profile identities differ")
	}
	for _, key := range []string{"scope", "uuid_parse_scope", "containment_scope"} {
		value := summary[key].(string)
		if strings.Contains(value, "batch boundaries are unchanged") || strings.Contains(value, "batch boundaries remain unchanged") {
			t.Fatal("compact profile inherited historical boundary claim", key)
		}
	}
	if header["expected_boundaries_recomputed"] != false ||
		!strings.Contains(header["boundary_scope"].(string), "no original UUID batch boundaries") {
		t.Fatal("recorder implies recomputation or old partition preservation")
	}
}

func TestNativeCaseProfileCompactLFInclusiveBudgetAnd128RowCeiling(t *testing.T) {
	// A synthetic oversized expected Text isolates serialization budgeting; it
	// is not a valid native-domain or native-capacity acceptance claim.
	s := caseProfileTestSuite("x")
	record := s.Cases[0].Expected.(map[string]any)
	record["error"] = ""
	base, err := json.Marshal(caseDocument(s))
	if err != nil {
		t.Fatal(err)
	}
	record["error"] = strings.Repeat("x", uuidCompactMaxRawBytes-len(base)-1)
	policy, err := uuidCompactCaseProfile.policy(s, caseProfileTestWorkspace(s), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := policy.prepare(s, 0)
	if err != nil || len(prepared.Raw) != 30000 {
		t.Fatal("exact30000 including LF was not admitted", len(prepared.Raw), err)
	}
	record["error"] = record["error"].(string) + "x"
	if _, err := policy.prepare(s, 0); err == nil {
		t.Fatal("30001 bytes including LF were admitted")
	}
	rows := repeatNativeCase(caseProfileTestSuite(""), 129)
	policy, err = uuidCompactCaseProfile.policy(rows, caseProfileTestWorkspace(rows), caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	first, err := policy.prepare(rows, 0)
	if err != nil || first.End != 128 {
		t.Fatal("128-row native ceiling changed", first.End, err)
	}
	last, err := policy.prepare(rows, first.End)
	if err != nil || last.End != 129 {
		t.Fatal("final original case was lost", last.End, err)
	}
}

func TestNativeCaseProfileUUIDCompactFull66PartitionObservation(t *testing.T) {
	suites := allNativeSuites(t)
	catalog, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil || len(suites) != 66 {
		t.Fatal("full immutable5faa suite/catalog roster", len(suites), err)
	}
	cases, legacyBatches, compactBatches, affected := 0, 0, 0, 0
	uuidCases, uuidBatches := 0, 0
	uuidEntries := map[string]bool{}
	for index, s := range suites {
		legacy, err := nativeCaseProfile("").policy(s, suiteWorkspace{}, upstream{})
		if err != nil {
			t.Fatal(err)
		}
		compactOther, err := uuidCompactCaseProfile.policy(s, suiteWorkspace{}, caseProfileTestLock())
		if uuidCompactTarget(s) {
			if err == nil {
				t.Fatal("UUID compact policy accepted an empty workspace")
			}
		} else if err != nil {
			t.Fatal("non-UUID profile changed admission", s.Package, s.Activity, err)
		}
		workspace, err := catalog.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := uuidCompactCaseProfile.policy(s, workspace, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		// Only identical value receivers and the same untouched suite/start may
		// share one prepare result. No source, case, assertion or native PASS is cached.
		if !uuidCompactTarget(s) && (legacy != compactOther || compactOther != policy) {
			t.Fatal("non-UUID policy changed with workspace selection or legacy profile", s.Package, s.Activity)
		}
		prior, current := 0, 0
		for start := 0; start < len(s.Cases); prior++ {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil || end <= start {
				t.Fatal("legacy interval failure", err)
			}
			got, err := legacy.prepare(s, start)
			if err != nil || got.End != end || got.Raw != nil {
				t.Fatal("zero policy changed legacy boundary/writer", s.Package, s.Activity, start, got.End, end, err)
			}
			if !uuidCompactTarget(s) {
				// prepare is pure for these equal value receivers and identical
				// suite/start arguments. Retain the separate direct-vs-legacy check above.
				other := got
				if err != nil || other.End != end || other.Raw != nil {
					t.Fatal("opt-in changed a non-UUID case/writer", s.Package, s.Activity, start, err)
				}
				current++
			}
			start = end
		}
		if uuidCompactTarget(s) {
			affected++
			key := s.Package + ":" + s.Activity
			if uuidEntries[key] {
				t.Fatal("UUID scope contains a repeated entry", key)
			}
			uuidEntries[key] = true
			for start := 0; start < len(s.Cases); current++ {
				prepared, err := policy.prepare(s, start)
				if err != nil || prepared.End <= start {
					t.Fatal("new policy interval failure", err)
				}
				checkNativeCaseProfileUUIDPreparedBatch(t, s, start, prepared)
				start = prepared.End
			}
			uuidCases, uuidBatches = uuidCases+len(s.Cases), uuidBatches+current
			t.Logf("new-policy partition %s:%s cases=%d batches=%d", s.Package, s.Activity, len(s.Cases), current)
		} else if prior != current {
			t.Fatal("non-UUID partitions changed", index, prior, current)
		}
		cases, legacyBatches, compactBatches = cases+len(s.Cases), legacyBatches+prior, compactBatches+current
		t.Logf("profile partition index=%d entry=%s:%s cases=%d legacy_batches=%d compact_batches=%d",
			index, s.Package, s.Activity, len(s.Cases), prior, current)
	}
	if uuidCases != 65357 || affected != 3 || len(uuidEntries) != 3 {
		t.Fatal("original UUID population or scope changed", uuidCases, affected, len(uuidEntries))
	}
	// These batch totals come from the original accepted PR89 profile. They
	// are assertions for this test candidate, not a new observed/native PASS.
	if uuidBatches != 631 || compactBatches != 2109 {
		t.Fatal("accepted compact partition counts changed", uuidBatches, compactBatches)
	}
	if cases != 159909 || legacyBatches != 2337 || affected != 3 {
		t.Fatal("original population/baseline or explicit UUID-only scope changed", cases, legacyBatches, affected)
	}
	// Root must freeze these actual new-policy observations before any separate
	// workflow/audit amendment. They are not native or full-CI acceptance.
	t.Logf("source-only proposed profile=%s suites=%d cases=%d legacy_batches=%d proposed_batches=%d proposed_timing_rows=%d native_calls=0 saved_replays=0",
		uuidCompactCaseProfile, len(suites), cases, legacyBatches, compactBatches, 2+2*len(suites)+compactBatches)
}
