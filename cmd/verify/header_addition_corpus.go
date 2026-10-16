package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/headeradditioninput"
	"golang.org/x/net/http/httpguts"
)

const headerAdditionCorpusFile = "examples/diagnostic-header-addition/corpus.json"
const headerAdditionCorpusSHA = "6a2c688b2af7fb2d032d794b6a2cbab4d42aae3969a054001965ef559c1b970f"

type headerAdditionSpecification struct {
	Label          string
	Input          headeradditioninput.Request
	Status, Reason string
	PayloadReason  int64
	Eligible       bool
}

// Validate the complete frozen population before calling the Go oracle. The
// four partial specification fields are assertions, never a native expected
// record or a retained private result. The oracle computes all sixteen fields.
func headerAdditionInputs(raw []byte) ([]headerAdditionSpecification, error) {
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, err
	}
	value, err := decode(raw)
	if err != nil {
		return nil, err
	}
	top, err := decimalPayloadObject(value, "schema", "status", "case_count", "cases")
	if err != nil {
		return nil, err
	}
	count, err := decimalPayloadInteger(top["case_count"])
	if err != nil || count != 50 || top["schema"] != "gooo/diagnostic-header-addition-design-cases/v2" || top["status"] != "SPECIFICATION_ONLY_NOT_EXECUTED" {
		return nil, fmt.Errorf("header addition frozen specification identity")
	}
	rows, ok := top["cases"].([]any)
	if !ok || len(rows) != 50 {
		return nil, fmt.Errorf("header addition exact50 cases required")
	}
	out := make([]headerAdditionSpecification, 0, len(rows))
	seen := map[string]bool{}
	for _, value := range rows {
		row, err := decimalPayloadObject(value, "label", "input", "expected")
		if err != nil {
			return nil, err
		}
		label, ok := row["label"].(string)
		if !ok || label == "" || seen[label] {
			return nil, fmt.Errorf("header addition unique nonempty labels required")
		}
		inputRaw, err := json.Marshal(row["input"])
		if err != nil {
			return nil, err
		}
		input, err := headeradditioninput.Decode(inputRaw)
		if err != nil {
			return nil, fmt.Errorf("header addition input %s: %w", label, err)
		}
		partial, err := decimalPayloadObject(row["expected"], "status", "reason", "payload_reason", "eligible")
		if err != nil {
			return nil, err
		}
		status, sok := partial["status"].(string)
		reason, rok := partial["reason"].(string)
		eligible, bok := partial["eligible"].(bool)
		payloadReason, err := decimalPayloadInteger(partial["payload_reason"])
		if !sok || !rok || !bok || err != nil {
			return nil, fmt.Errorf("header addition exact partial specification types")
		}
		out = append(out, headerAdditionSpecification{label, input, status, reason, payloadReason, eligible})
		seen[label] = true
	}
	return out, nil
}

func headerAdditionFacts(f headeradditioninput.Facts) map[string]any {
	return map[string]any{
		"content_length_occurrences": f.ContentLengthOccurrences, "observed_body_bytes": f.ObservedBodyBytes,
		"maximum_payload_bytes": f.MaximumPayloadBytes, "transfer_encoding_present": f.TransferEncodingPresent,
		"body_observation_bound_to_request": f.BodyObservationBoundToRequest, "current_access": f.CurrentAccess,
		"candidate_name_occurrences": f.CandidateNameOccurrences, "header_set_bound_to_request": f.HeaderSetBoundToRequest,
		"policy_revision_current": f.PolicyRevisionCurrent, "allowed_header_name": f.AllowedHeaderName,
		"expected_request_revision": f.ExpectedRequestRevision, "observed_request_revision": f.ObservedRequestRevision,
	}
}

func headerAdditionArguments(in headeradditioninput.Request) []any {
	return []any{in.Name, in.Value, in.DeclaredLength, headerAdditionFacts(in.Facts)}
}

// This enforces the ABI's typed domain, not eligibility. In particular, negative
// counts, exhausted revisions, control bytes and false bindings reach Gooo.
func validateHeaderAdditionInputs(inputs []any) error {
	if len(inputs) != 4 {
		return fmt.Errorf("header addition four inputs required")
	}
	for _, value := range inputs[:3] {
		if _, err := decimalPayloadText(value); err != nil {
			return err
		}
	}
	facts, err := decimalPayloadObject(inputs[3], "content_length_occurrences", "observed_body_bytes", "maximum_payload_bytes", "transfer_encoding_present", "body_observation_bound_to_request", "current_access", "candidate_name_occurrences", "header_set_bound_to_request", "policy_revision_current", "allowed_header_name", "expected_request_revision", "observed_request_revision")
	if err != nil {
		return err
	}
	for _, name := range []string{"content_length_occurrences", "observed_body_bytes", "maximum_payload_bytes", "candidate_name_occurrences", "expected_request_revision", "observed_request_revision"} {
		if _, ok := facts[name].(int64); !ok {
			return fmt.Errorf("header addition exact int64 field %s required", name)
		}
	}
	for _, name := range []string{"transfer_encoding_present", "body_observation_bound_to_request", "current_access", "header_set_bound_to_request", "policy_revision_current"} {
		if _, ok := facts[name].(bool); !ok {
			return fmt.Errorf("header addition exact Boolean field %s required", name)
		}
	}
	_, err = decimalPayloadText(facts["allowed_header_name"])
	return err
}

// The parser result comes from the actual pinned Go API. Ordered refusal
// selection is an independent Go reference for the original Gooo composition;
// it is used only by the verifier, never by the input adapter or a live host.
func headerAdditionPayloadReason(in headeradditioninput.Request) (int64, error) {
	f := in.Facts
	value, err := strconv.ParseInt(in.DeclaredLength, 10, 64)
	kind := int64(0)
	if err != nil {
		switch {
		case errors.Is(err, strconv.ErrSyntax):
			kind = 1
		case errors.Is(err, strconv.ErrRange):
			kind = 2
		default:
			return 0, err
		}
	}
	unsigned := in.DeclaredLength != "" && len(in.DeclaredLength) <= 1024 && strings.IndexFunc(in.DeclaredLength, func(r rune) bool { return r < '0' || r > '9' }) < 0
	conditions := []struct {
		bad    bool
		reason int64
	}{
		{f.ContentLengthOccurrences < 0 || f.ObservedBodyBytes < 0 || f.MaximumPayloadBytes < 0 || (f.ContentLengthOccurrences == 0 && in.DeclaredLength != ""), 1},
		{!f.CurrentAccess, 2}, {!f.BodyObservationBoundToRequest, 3}, {f.TransferEncodingPresent, 4},
		{f.ContentLengthOccurrences == 0, 5}, {f.ContentLengthOccurrences > 1, 6}, {kind == 1, 7}, {kind == 2, 8}, {!unsigned, 9},
		{kind != 0 || value < 0 || f.ObservedBodyBytes < 0 || value != f.ObservedBodyBytes, 10},
		{f.ObservedBodyBytes < 0 || f.MaximumPayloadBytes < 0 || f.ObservedBodyBytes > f.MaximumPayloadBytes, 11},
	}
	for _, c := range conditions {
		if c.bad {
			return c.reason, nil
		}
	}
	return 0, nil
}

func headerAdditionReference(in headeradditioninput.Request) (map[string]any, error) {
	if err := validateHeaderAdditionInputs(headerAdditionArguments(in)); err != nil {
		return nil, err
	}
	pr, err := headerAdditionPayloadReason(in)
	if err != nil {
		return nil, err
	}
	nv, vv := httpguts.ValidHeaderFieldName(in.Name), httpguts.ValidHeaderFieldValue(in.Value)
	no, nb, vo, vb := int64(-1), int64(-1), int64(-1), int64(-1)
	if !nv && in.Name != "" {
		for end := 1; end <= len(in.Name); end++ {
			if !httpguts.ValidHeaderFieldName(in.Name[:end]) {
				no, nb = int64(end-1), int64(in.Name[end-1])
				break
			}
		}
	}
	if !vv {
		for at := 0; at < len(in.Value); at++ {
			if !httpguts.ValidHeaderFieldValue(in.Value[at : at+1]) {
				vo, vb = int64(at), int64(in.Value[at])
				break
			}
		}
	}
	obs := utf8.RuneCountInString(in.Value) < len(in.Value)
	edge := strings.Trim(in.Value, " \t") != in.Value
	f := in.Facts
	guards := []struct {
		bad            bool
		status, reason string
	}{
		{f.CandidateNameOccurrences < 0 || f.ExpectedRequestRevision <= 0 || f.ObservedRequestRevision <= 0 || f.ExpectedRequestRevision == math.MaxInt64 || f.ObservedRequestRevision == math.MaxInt64, "INVALID_INPUT", "REQUEST_METADATA_INVALID"},
		{f.AllowedHeaderName != "x-request-id" && f.AllowedHeaderName != "x-correlation-id", "INVALID_INPUT", "ROUTE_HEADER_NOT_SUPPORTED"},
		{!f.HeaderSetBoundToRequest, "REFUSED", "HEADER_CONTEXT_UNBOUND"}, {!f.PolicyRevisionCurrent, "REFUSED", "POLICY_REVISION_UNBOUND"},
		{f.ExpectedRequestRevision != f.ObservedRequestRevision, "REFUSED", "REQUEST_REVISION_CHANGED"}, {pr != 0, "REFUSED", "PAYLOAD_REFUSED"},
		{!nv, "REFUSED", "HEADER_NAME_INVALID"}, {in.Name != f.AllowedHeaderName, "REFUSED", "CANDIDATE_NAME_NOT_ALLOWED"},
		{f.CandidateNameOccurrences != 0, "REFUSED", "FIELD_ALREADY_PRESENT"}, {!vv, "REFUSED", "HEADER_VALUE_INVALID"},
		{obs, "REFUSED", "VALUE_NON_ASCII"}, {edge, "REFUSED", "VALUE_EDGE_WHITESPACE"},
		{len(in.Value) == 0 || len(in.Value) > 128, "REFUSED", "VALUE_SIZE_NOT_ALLOWED"},
	}
	status, reason := "ELIGIBLE", "ROUTE_DIAGNOSTIC_FIELD_AND_PAYLOAD_ACCEPTED"
	for _, g := range guards {
		if g.bad {
			status, reason = g.status, g.reason
			break
		}
	}
	return map[string]any{
		"candidate_name": in.Name, "candidate_value": in.Value, "status": status, "reason": reason,
		"payload_eligible": pr == 0, "payload_reason": pr, "header_name_valid": nv, "header_value_valid": vv,
		"name_invalid_offset": no, "name_invalid_octet": nb, "value_invalid_offset": vo, "value_invalid_octet": vb,
		"value_has_obs_text": obs, "value_edge_whitespace": edge, "request_revision": f.ObservedRequestRevision, "eligible": status == "ELIGIBLE",
	}, nil
}

func headerAdditionCases(raw []byte) ([]testCase, error) {
	population, err := headerAdditionInputs(raw)
	if err != nil {
		return nil, err
	}
	out := make([]testCase, 0, len(population))
	for _, spec := range population {
		expected, err := headerAdditionReference(spec.Input)
		if err != nil {
			return nil, err
		}
		if expected["status"] != spec.Status || expected["reason"] != spec.Reason || expected["payload_reason"] != spec.PayloadReason || expected["eligible"] != spec.Eligible {
			return nil, fmt.Errorf("header addition independent reference disagrees with specification %s", spec.Label)
		}
		out = append(out, testCase{Inputs: headerAdditionArguments(spec.Input), Expected: expected})
	}
	return out, nil
}

func headerAdditionCorpus(root string) ([]suite, error) {
	raw, err := decimalPayloadReadFixture(root, headerAdditionCorpusFile, headerAdditionCorpusSHA)
	if err != nil {
		return nil, err
	}
	cases, err := headerAdditionCases(raw)
	if err != nil {
		return nil, err
	}
	return []suite{{Package: headerAdditionPackage, Activity: headerAdditionActivity,
		Domain: "Original pure16-field diagnostic-header addition plan; exact50 ordered frozen witnesses; 3 Text + exact12 trusted facts. Decoded valid UTF8 Text<=1024 and signed64 metadata. Fresh pinned httpguts/strconv Go oracle. No effect, authority proof, HTTP framing or production-safety claim.", Cases: cases}}, nil
}
