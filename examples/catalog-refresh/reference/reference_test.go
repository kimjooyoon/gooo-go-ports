// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func completeRequest() Request {
	d := "sha256:" + strings.Repeat("a", 64)
	return Request{
		Label:    "unit",
		Snapshot: Facts{RequestKinds: []string{"ABSENT", "STRING"}, RequestValues: []string{"", "page:2"}, NextKinds: []string{"STRING", "ABSENT"}, NextValues: []string{"page:2", ""}, Names: []string{"lookup", "write"}, Definitions: []string{d, d}, Profiles: []string{definitionProfile, definitionProfile}, Annotations: []string{d, d}, RowPages: []int64{0, 1}},
		Evidence: Evidence{Attempt: "attempt:7", Candidate: d, Collector: "SUCCESS", Binding: "BOUND", Validator: validatorProfile, Definitions: []string{d, d}, Outcomes: []string{"ACCEPTED", "ACCEPTED"}},
		State:    State{ExpectedRegistry: "registry:3", CurrentRegistry: "registry:3", ExpectedScope: "scope:2", CurrentScope: "scope:2", ExpectedInvalidation: "invalidation:4", CurrentInvalidation: "invalidation:4", ExpectedAttempt: "attempt:7", CurrentAttempt: "attempt:7"},
	}
}
func assertFields(t *testing.T, actual, want map[string]any) {
	t.Helper()
	for key, value := range want {
		if !reflect.DeepEqual(actual[key], value) {
			t.Errorf("%s = %#v, want %#v", key, actual[key], value)
		}
	}
}
func TestCompleteDecisionABIAndNoEffects(t *testing.T) {
	c := completeRequest()
	got := expected(c)
	want := map[string]any{
		"state": "CANDIDATE_ELIGIBLE", "reason": "COMPLETE_CANDIDATE", "candidate_id": c.Evidence.Candidate, "attempt_id": "attempt:7",
		"expected_registry_version": "registry:3", "expected_scope_epoch": "scope:2", "expected_invalidation_epoch": "invalidation:4", "expected_latest_attempt": "attempt:7",
		"candidate_page_count": 2, "candidate_row_count": 2, "coverage": "COMPLETE", "coverage_reason": "OBSERVED_PAGE_CHAIN_COMPLETE",
		"problem_page_index": -1, "problem_row_index": -1, "decision_scope": "WHOLE_CANDIDATE_LOCAL_REPLACEMENT_PRECHECK", "execution_authorized": false, "live_effect_performed": false, "remote_atomicity": "UNKNOWN",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decision ABI differs: got %#v", got)
	}
}
func TestEveryRowParticipatesInAdmission(t *testing.T) {
	for _, name := range []string{"lookup", "write"} {
		t.Run(name, func(t *testing.T) {
			c := completeRequest()
			index := 0
			if name == "write" {
				index = 1
			}
			c.Evidence.Outcomes[index] = "REJECTED"
			assertFields(t, expected(c), map[string]any{"state": "RETAIN_STORED_CATALOG", "reason": "ROW_REJECTED", "problem_row_index": index})
			c.Evidence.Outcomes[index] = "ACCEPTED"
			c.Snapshot.Profiles[index] = "unknown/profile"
			assertFields(t, expected(c), map[string]any{"reason": "UNSUPPORTED_DEFINITION_PROFILE", "problem_row_index": index})
			c.Snapshot.Profiles[index] = definitionProfile
			c.Snapshot.Annotations[index] = "not-a-digest"
			assertFields(t, expected(c), map[string]any{"reason": "INVALID_SNAPSHOT", "problem_row_index": index, "coverage": "INVALID"})
		})
	}
}
func TestAllRowBindingsPrecedeAllOutcomes(t *testing.T) {
	c := completeRequest()
	c.Evidence.Outcomes[0] = "REJECTED"
	c.Evidence.Definitions[1] = "sha256:" + strings.Repeat("b", 64)
	assertFields(t, expected(c), map[string]any{"reason": "ROW_VALIDATION_BINDING_MISMATCH", "problem_row_index": 1})
}
func TestIndependentHostTokensAndPrecedence(t *testing.T) {
	rows := []struct {
		name, reason string
		change       func(*State)
	}{
		{"registry", "REGISTRY_CHANGED", func(s *State) { s.CurrentRegistry = "new" }},
		{"scope", "SCOPE_CHANGED", func(s *State) { s.CurrentScope = "new" }},
		{"invalidation", "INVALIDATED", func(s *State) { s.CurrentInvalidation = "new" }},
		{"attempt", "SUPERSEDED_ATTEMPT", func(s *State) { s.CurrentAttempt = "new" }},
	}
	for _, test := range rows {
		t.Run(test.name, func(t *testing.T) {
			c := completeRequest()
			test.change(&c.State)
			assertFields(t, expected(c), map[string]any{"reason": test.reason, "state": "RETAIN_STORED_CATALOG"})
		})
	}
	c := completeRequest()
	for _, test := range rows {
		test.change(&c.State)
	}
	assertFields(t, expected(c), map[string]any{"reason": "REGISTRY_CHANGED"})
	c.State.CurrentRegistry = c.State.ExpectedRegistry
	assertFields(t, expected(c), map[string]any{"reason": "SCOPE_CHANGED"})
	c.State.CurrentScope = c.State.ExpectedScope
	assertFields(t, expected(c), map[string]any{"reason": "INVALIDATED"})
}
func TestEmptyTerminalIsDifferentFromNoPages(t *testing.T) {
	c := completeRequest()
	c.Snapshot = Facts{RequestKinds: []string{"ABSENT"}, RequestValues: []string{""}, NextKinds: []string{"NULL"}, NextValues: []string{""}}
	c.Evidence.Definitions = nil
	c.Evidence.Outcomes = nil
	assertFields(t, expected(c), map[string]any{"reason": "COMPLETE_CANDIDATE", "candidate_row_count": 0, "candidate_page_count": 1})
	c.Snapshot = Facts{}
	assertFields(t, expected(c), map[string]any{"reason": "INCOMPLETE_CATALOG", "coverage_reason": "NO_SAVED_PAGES", "candidate_page_count": 0})
}
func TestCollectorFailureDoesNotBecomeCompleteCandidate(t *testing.T) {
	for collector, reason := range map[string]string{"ERROR": "COLLECTOR_ERROR", "TIMEOUT": "COLLECTOR_TIMEOUT", "CANCELLED": "COLLECTOR_CANCELLED"} {
		t.Run(collector, func(t *testing.T) {
			c := completeRequest()
			c.Snapshot.NextKinds[1] = "STRING"
			c.Snapshot.NextValues[1] = "more"
			c.Evidence.Collector = collector
			assertFields(t, expected(c), map[string]any{"reason": reason, "coverage": "INCOMPLETE", "coverage_reason": "MISSING_TERMINAL_PAGE", "problem_page_index": 1})
		})
	}
}
func TestMetadataRefusalRetainsCandidateDiagnostics(t *testing.T) {
	c := completeRequest()
	c.Evidence.Binding = "UNBOUND"
	c.Snapshot.Profiles[1] = "unknown/profile"
	assertFields(t, expected(c), map[string]any{"reason": "UNBOUND_CANDIDATE", "problem_row_index": 1})
	c.Snapshot.Names[1] = c.Snapshot.Names[0]
	c.Snapshot.NextKinds[1] = "STRING"
	c.Snapshot.NextValues[1] = "more"
	assertFields(t, expected(c), map[string]any{"reason": "UNBOUND_CANDIDATE", "problem_page_index": 1, "problem_row_index": 1, "coverage": "INCOMPLETE"})
}
func TestUTF8ByteBoundariesAndDigestDomain(t *testing.T) {
	if !token(strings.Repeat("é", 512)) || token(strings.Repeat("é", 513)) || token("nul\x00token") || token("") {
		t.Fatal("token domain is bytes, nonempty and NUL-free")
	}
	for _, bad := range []string{"sha256:" + strings.Repeat("A", 64), "sha256:" + strings.Repeat("g", 64), "sha256:ab", strings.Repeat("a", 64)} {
		if digest(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	if !digest("sha256:" + strings.Repeat("0", 64)) {
		t.Fatal("valid digest refused")
	}
}
func TestSavedCorpusLiteralAnchorsAndExactGeneratedCases(t *testing.T) {
	raw, err := os.ReadFile("../reference-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := render(raw)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile("../kernel.cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, rendered) {
		t.Fatal("saved kernel.cases differ from independent oracle")
	}
	var request Requests
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	if len(request.Cases) < 100 {
		t.Fatal("finite coverage controls unexpectedly removed")
	}
	reasons := map[string]bool{}
	for _, c := range request.Cases {
		v := expected(c)
		reasons[v["reason"].(string)] = true
		if v["execution_authorized"] != false || v["live_effect_performed"] != false || v["remote_atomicity"] != "UNKNOWN" {
			t.Fatal("effect boundary")
		}
	}
	for _, reason := range []string{"INVALID_REFRESH_EVIDENCE", "UNBOUND_CANDIDATE", "INVALID_REFRESH_STATE", "ATTEMPT_BINDING_MISMATCH", "COLLECTOR_ERROR", "COLLECTOR_TIMEOUT", "COLLECTOR_CANCELLED", "UNSUPPORTED_VALIDATOR_PROFILE", "INVALID_SNAPSHOT", "INCOMPLETE_CATALOG", "DUPLICATE_NAME", "UNSUPPORTED_DEFINITION_PROFILE", "ROW_VALIDATION_LENGTH", "ROW_VALIDATION_BINDING_MISMATCH", "ROW_REJECTED", "ROW_UNASSESSED", "INVALID_ROW_VALIDATION_STATE", "REGISTRY_CHANGED", "SCOPE_CHANGED", "INVALIDATED", "SUPERSEDED_ATTEMPT", "COMPLETE_CANDIDATE"} {
		if !reasons[reason] {
			t.Errorf("missing reason %s", reason)
		}
	}
}
func TestTypedBoundaryCasesArePreservedAndExplained(t *testing.T) {
	raw, err := os.ReadFile("../reference-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var request Requests
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("../input-boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	var boundaries struct {
		Reference  int `json:"reference_cases"`
		Native     int `json:"native_cases"`
		Rejections []struct {
			Index int    `json:"reference_case_index"`
			Label string `json:"label"`
			Stage string `json:"stage"`
			Code  string `json:"code"`
			Size  int    `json:"observed_size"`
			Limit int    `json:"limit"`
		} `json:"rejections"`
	}
	if err := json.Unmarshal(raw, &boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries.Reference != len(request.Cases) || boundaries.Native+len(boundaries.Rejections) != boundaries.Reference || len(boundaries.Rejections) < 9 {
		t.Fatal("boundary counts")
	}
	seen := map[int]bool{}
	for _, rejection := range boundaries.Rejections {
		if rejection.Index < 0 || rejection.Index >= len(request.Cases) || seen[rejection.Index] {
			t.Fatal("boundary index")
		}
		seen[rejection.Index] = true
		c := request.Cases[rejection.Index]
		if c.Label != rejection.Label || rejection.Stage != "INPUT_VALIDATION" || rejection.Code != "INVALID_INPUT_VALUE" || rejection.Size <= rejection.Limit {
			t.Fatal("boundary explanation")
		}
		if expected(c)["state"] != "RETAIN_STORED_CATALOG" {
			t.Fatal("oversize domain-reference expectation was dropped")
		}
	}
}
func TestFixtureEnvelopeRejectsAmbiguity(t *testing.T) {
	raw, err := os.ReadFile("../reference-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var request Requests
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	request.Cases = request.Cases[:1]
	base, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"trailing":     append(append([]byte(nil), base...), []byte(" {}")...),
		"unknown":      bytes.Replace(base, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
		"wrong-schema": bytes.Replace(base, []byte("gooo/catalog-refresh-reference-inputs/v1"), []byte("other/v1"), 1),
		"too-large":    bytes.Repeat([]byte(" "), (4<<20)+1),
	}
	for label, input := range tests {
		t.Run(label, func(t *testing.T) {
			if _, err := render(input); err == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
	request.Cases = append(request.Cases, request.Cases[0])
	duplicate, _ := json.Marshal(request)
	if _, err := render(duplicate); err == nil {
		t.Fatal("duplicate labels accepted")
	}
	request.Cases = request.Cases[:1]
	request.Cases[0].Anchors["reason"] = json.RawMessage(`"WRONG"`)
	wrong, _ := json.Marshal(request)
	if _, err := render(wrong); err == nil {
		t.Fatal("incorrect literal anchor accepted")
	}
}
