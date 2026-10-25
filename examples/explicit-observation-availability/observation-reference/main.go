// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
// Independent finite expectations; the installable consumer never imports this.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Input struct {
	ContextID              string `json:"context_id"`
	FieldPath              string `json:"field_path"`
	FieldKind              string `json:"field_kind"`
	RecordingBasis         string `json:"recording_basis"`
	ValueState             string `json:"value_state"`
	Value                  string `json:"value"`
	ComparisonScope        string `json:"comparison_scope"`
	ExpectedValueState     string `json:"expected_value_state"`
	ExpectedValue          string `json:"expected_value"`
	ExpectedFieldPath      string `json:"expected_field_path"`
	SourceDocument         string `json:"source_document"`
	ExpectedSourceDocument string `json:"expected_source_document"`
}
type Result struct {
	ContextID                 string `json:"context_id"`
	FieldPath                 string `json:"field_path"`
	FieldKind                 string `json:"field_kind"`
	RecordingBasis            string `json:"recording_basis"`
	ValueState                string `json:"value_state"`
	Value                     string `json:"value"`
	ComparisonScope           string `json:"comparison_scope"`
	ExpectedValueState        string `json:"expected_value_state"`
	ExpectedValue             string `json:"expected_value"`
	ExpectedFieldPath         string `json:"expected_field_path"`
	SourceDocument            string `json:"source_document"`
	ExpectedSourceDocument    string `json:"expected_source_document"`
	State                     string `json:"state"`
	Reason                    string `json:"reason"`
	NextAction                string `json:"next_action"`
	RequiredForContext        bool   `json:"required_for_context"`
	KnownChange               bool   `json:"known_change"`
	ProducerAuthenticated     bool   `json:"producer_authenticated"`
	BehaviorEquivalenceProven bool   `json:"behavior_equivalence_proven"`
	ExecutionAuthorized       bool   `json:"execution_authorized"`
	LiveEffectPerformed       bool   `json:"live_effect_performed"`
}

func pointer(p string) bool {
	if len(p) == 0 || len(p) > 512 || !strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return false
	}
	return !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(p, "~1", ""), "~0", ""), "~")
}
func availability(declaration, value string) string {
	if declaration == "UNOBSERVED" {
		return "UNOBSERVED"
	}
	if declaration == "SUPPLIED" && value != "" {
		return "SUPPLIED"
	}
	return "INVALID"
}
func assess(i Input) Result {
	r := Result{ContextID: i.ContextID, FieldPath: i.FieldPath, FieldKind: i.FieldKind, RecordingBasis: i.RecordingBasis, ValueState: availability(i.ValueState, i.Value), Value: i.Value, ComparisonScope: i.ComparisonScope, ExpectedValueState: "NOT_APPLICABLE", ExpectedValue: i.ExpectedValue, ExpectedFieldPath: i.ExpectedFieldPath, SourceDocument: i.SourceDocument, ExpectedSourceDocument: i.ExpectedSourceDocument}
	var recognized bool
	switch i.FieldKind {
	case "OPAQUE_CONTEXT_ID", "BACKEND_BINDING_REVISION", "CONFIGURATION_REVISION", "PROTOCOL_VERSION":
		r.RequiredForContext = true
		recognized = true
	case "CAPTURE_TIMESTAMP", "POLICY_REVISION":
		recognized = true
	}
	invalid := i.ContextID == "" || !pointer(i.FieldPath) || !recognized || r.ValueState == "INVALID" || len(i.Value) > 1024 || len(i.ExpectedValue) > 1024
	invalid = invalid || (i.SourceDocument != "RECORDED_INPUT" && i.SourceDocument != "SUPPLIED_POLICY") || (i.FieldKind == "POLICY_REVISION" && i.SourceDocument != "SUPPLIED_POLICY") || (i.FieldKind != "POLICY_REVISION" && i.SourceDocument != "RECORDED_INPUT")
	switch i.RecordingBasis {
	case "CAPTURE_TIME", "RETROSPECTIVE", "UNKNOWN", "HOST_DECLARATION":
	default:
		invalid = true
	}
	switch i.ComparisonScope {
	case "NONE":
		invalid = invalid || i.ExpectedSourceDocument != "NOT_APPLICABLE" || i.ExpectedValueState != "NOT_APPLICABLE" || i.ExpectedValue != "" || i.ExpectedFieldPath != ""
	default:
		r.ExpectedValueState = availability(i.ExpectedValueState, i.ExpectedValue)
		invalid = invalid || i.ExpectedSourceDocument != i.SourceDocument || (i.ComparisonScope != "SAME_CAPTURE" && i.ComparisonScope != "BETWEEN_CATALOGS") || r.ExpectedValueState == "INVALID" || !pointer(i.ExpectedFieldPath) || i.ExpectedFieldPath == i.FieldPath
	}
	if invalid {
		r.State = "INVALID_DECLARATION"
		r.Reason = "INVALID_OBSERVATION_FIELD_DECLARATION"
		r.NextAction = "FIX_DECLARATION"
		return r
	}
	if r.ValueState == "UNOBSERVED" {
		if r.RequiredForContext {
			r.State = "UNASSESSED"
			r.Reason = "REQUIRED_OBSERVATION_UNAVAILABLE"
			r.NextAction = "OBTAIN_OBSERVATION"
		} else {
			r.State = "OPTIONAL_UNOBSERVED"
			r.Reason = "OPTIONAL_OBSERVATION_UNAVAILABLE"
			r.NextAction = "PRESERVE_UNOBSERVED"
		}
		return r
	}
	if i.ComparisonScope == "NONE" {
		r.State = "SUPPLIED_ONLY"
		r.Reason = "SUPPLIED_TEXT_NOT_AUTHENTICATED"
		r.NextAction = "CONTINUE_DOMAIN_VALIDATION"
		return r
	}
	if r.ExpectedValueState == "UNOBSERVED" {
		r.State = "UNASSESSED"
		r.Reason = "COMPARISON_OBSERVATION_UNAVAILABLE"
		r.NextAction = "OBTAIN_COMPARISON_OBSERVATION"
		return r
	}
	if i.Value == i.ExpectedValue {
		r.State = "SAME_SUPPLIED"
		r.Reason = "SAME_SUPPLIED_TEXT_NOT_AUTHENTICATED"
		r.NextAction = "CONTINUE_DOMAIN_VALIDATION"
		return r
	}
	r.State = "KNOWN_MISMATCH"
	r.Reason = "DIFFERENT_SUPPLIED_CONTEXT_VALUES"
	r.NextAction = "REEVALUATE_CONTEXT_CHANGE"
	r.KnownChange = true
	if i.ComparisonScope == "SAME_CAPTURE" {
		r.NextAction = "RECOLLECT_CONTEXT"
	}
	return r
}
func main() {
	inputPath := flag.String("input", "", "finite request JSON")
	flag.Parse()
	if *inputPath == "" || flag.NArg() != 0 {
		panic("exact input file required")
	}
	raw, err := os.ReadFile(*inputPath)
	if err != nil {
		panic(err)
	}
	var requests struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Label        string `json:"label"`
			Input        Input  `json:"input"`
			AnchorState  string `json:"anchor_state"`
			AnchorAction string `json:"anchor_action"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &requests); err != nil {
		panic(err)
	}
	if requests.Schema != "gooo/explicit-observation-reference-inputs/v1" || len(requests.Cases) == 0 {
		panic("finite reference requests required")
	}
	const entry = "gooo/tool-origin/explicit-observation:AssessObservationField"
	cases := make([]any, 0, len(requests.Cases))
	seen := map[string]bool{}
	for _, c := range requests.Cases {
		if c.Label == "" || seen[c.Label] {
			panic("unique label required")
		}
		seen[c.Label] = true
		expected := assess(c.Input)
		if c.AnchorState != "" && expected.State != c.AnchorState {
			panic(fmt.Sprintf("%s: state %s != anchor %s", c.Label, expected.State, c.AnchorState))
		}
		if c.AnchorAction != "" && expected.NextAction != c.AnchorAction {
			panic(fmt.Sprintf("%s: action %s != anchor %s", c.Label, expected.NextAction, c.AnchorAction))
		}
		cases = append(cases, map[string]any{"inputs": map[string]Input{entry: c.Input}, "expected": map[string]Result{entry: expected}})
	}
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": cases}); err != nil {
		panic(err)
	}
}
