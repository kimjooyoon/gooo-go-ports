package revisionpolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

var errInputSchema = errors.New("expected exact typed bounded API revision object")

var decisionFields = []string{
	"allowed", "reason", "authenticated", "tenant_present", "tenant_matched",
	"action_present", "action_matched", "access_allowed", "access_error", "access_result",
	"group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal",
}

func DecodeCompareRequest(raw []byte) (CompareRequest, error) {
	root, err := strictObject(raw, []string{"before_profile", "candidate_profile", "before"})
	if err != nil {
		return CompareRequest{}, err
	}
	beforeID, err := strictString(root["before_profile"], 64)
	if err != nil {
		return CompareRequest{}, err
	}
	candidateID, err := strictString(root["candidate_profile"], 64)
	if err != nil {
		return CompareRequest{}, err
	}
	if err := checkProfiles(beforeID, candidateID); err != nil {
		return CompareRequest{}, err
	}
	before, err := original.DecodeAPIProof(root["before"])
	if err != nil {
		return CompareRequest{}, fmt.Errorf("before: %w", err)
	}
	return CompareRequest{BeforeProfile: beforeID, CandidateProfile: candidateID, Before: before}, nil
}

// DecodeComparisonReport validates every nested transport object before callers
// can replay it. Decoding a typed record is separate from VerifyComparison.
func DecodeComparisonReport(raw []byte) (ComparisonReport, error) {
	root, err := strictObjectWithin(raw, []string{
		"schema", "status", "scope", "before_profile", "candidate_profile", "before", "after",
		"differences", "summary", "classifier_bindings",
	}, MaxReportBytes)
	if err != nil {
		return ComparisonReport{}, err
	}
	for _, name := range []string{"schema", "status", "scope", "before_profile", "candidate_profile"} {
		if _, err := strictString(root[name], 128); err != nil {
			return ComparisonReport{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	before, err := original.DecodeAPIProof(root["before"])
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("before: %w", err)
	}
	after, err := original.DecodeAPIProof(root["after"])
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("after: %w", err)
	}
	differences, err := decodeDifferences(root["differences"])
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("differences: %w", err)
	}
	summary, err := decodeSummary(root["summary"])
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("summary: %w", err)
	}
	// Reuse the original exact binding decoder through a strictly typed carrier.
	// No generation is selected or verified from the supplied carrier bindings.
	carrier, err := json.Marshal(struct {
		Facts    original.APIFacts    `json:"facts"`
		Decision original.APIDecision `json:"decision"`
		Bindings json.RawMessage      `json:"bindings"`
	}{before.Facts, before.Decision, root["classifier_bindings"]})
	if err != nil {
		return ComparisonReport{}, err
	}
	classifier, err := original.DecodeAPIProof(carrier)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("classifier bindings: %w", err)
	}
	var report ComparisonReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return ComparisonReport{}, err
	}
	if err := checkProfiles(report.BeforeProfile, report.CandidateProfile); err != nil {
		return ComparisonReport{}, err
	}
	if report.Schema != ComparisonSchema || report.Status != ComparisonStatus || report.Scope != ComparisonScope {
		return ComparisonReport{}, errInputSchema
	}
	report.Before, report.After = before, after
	report.Differences, report.Summary = differences, summary
	report.ClassifierBindings = classifier.Bindings
	return report, nil
}

func decodeDifferences(raw []byte) (DifferenceFlags, error) {
	fields, err := strictObject(raw, decisionFields)
	if err != nil {
		return DifferenceFlags{}, err
	}
	for _, value := range fields {
		if !bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false")) {
			return DifferenceFlags{}, errInputSchema
		}
	}
	var flags DifferenceFlags
	if err := json.Unmarshal(raw, &flags); err != nil {
		return DifferenceFlags{}, err
	}
	return flags, nil
}

func decodeSummary(raw []byte) (Summary, error) {
	fields, err := strictObject(raw, []string{"classification", "changed_count"})
	if err != nil {
		return Summary{}, err
	}
	classification, err := strictString(fields["classification"], 64)
	if err != nil {
		return Summary{}, err
	}
	switch classification {
	case "NEWLY_ALLOWED", "NEWLY_DENIED", "REASON_CHANGED", "OBSERVATIONS_CHANGED", "UNCHANGED":
	default:
		return Summary{}, errInputSchema
	}
	var count *int
	if json.Unmarshal(fields["changed_count"], &count) != nil || count == nil || *count < 0 || *count > 15 {
		return Summary{}, errInputSchema
	}
	return Summary{Classification: classification, ChangedCount: *count}, nil
}

func strictString(raw []byte, maxBytes int) (string, error) {
	var text string
	if len(raw) == 0 || raw[0] != '"' || !validUnicodeEscapes(raw) ||
		json.Unmarshal(raw, &text) != nil || len(text) > maxBytes || !utf8.ValidString(text) {
		return "", errInputSchema
	}
	return text, nil
}

// This lexical check preserves the original transport's refusal of lossy
// isolated surrogate decoding, including escaped object keys.
func validUnicodeEscapes(raw []byte) bool {
	for i := 1; i < len(raw)-1; {
		if raw[i] != '\\' {
			i++
			continue
		}
		if i+1 >= len(raw)-1 {
			return false
		}
		if raw[i+1] != 'u' {
			i += 2
			continue
		}
		if i+6 > len(raw)-1 {
			return false
		}
		value, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+12 > len(raw)-1 || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 12
			continue
		}
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func strictObject(raw []byte, keys []string) (map[string]json.RawMessage, error) {
	return strictObjectWithin(raw, keys, MaxInputBytes)
}

func strictObjectWithin(raw []byte, keys []string, maxBytes int) (map[string]json.RawMessage, error) {
	if len(raw) > maxBytes || !utf8.Valid(raw) {
		return nil, errInputSchema
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errInputSchema
	}
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	values := make(map[string]json.RawMessage, len(keys))
	for decoder.More() {
		before := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			return nil, errInputSchema
		}
		key, ok := token.(string)
		keyRaw := bytes.TrimSpace(raw[before:decoder.InputOffset()])
		keyRaw = bytes.TrimSpace(bytes.TrimPrefix(keyRaw, []byte(",")))
		if _, err := strictString(keyRaw, 512); err != nil || !ok || !allowed[key] {
			return nil, errInputSchema
		}
		if _, duplicate := values[key]; duplicate || len(values) >= len(keys) {
			return nil, errInputSchema
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errInputSchema
		}
		values[key] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(values) != len(keys) {
		return nil, errInputSchema
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInputSchema
	}
	return values, nil
}
