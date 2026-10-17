package revisionpolicy

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func marshalTest(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// This is only a lexical transport fixture. Its invented bindings/decision are
// not asserted to be a valid native proof or a factual event.
func transportFixture() CompareRequest {
	return CompareRequest{
		BeforeProfile: OriginalProfile, CandidateProfile: CandidateProfile,
		Before: original.APIProof{
			Facts: original.APIFacts{
				Facts:         original.Facts{GroupPresent: true, GroupValue: "admin", PrincipalPresent: true, PrincipalValue: "synthetic:user"},
				Authenticated: true, CallerTenant: "tenant-a", ResourceTenant: "tenant-a", RequestedAction: "write", AllowedAction: "write",
			},
			Bindings: original.Bindings{CompilerSourceSHA: strings.Repeat("a", 40), GenerationSHA256: "sha256:" + strings.Repeat("b", 64), Assets: map[string]string{"source.gooo": "sha256:" + strings.Repeat("c", 64)}},
		},
	}
}

func transportReport() ComparisonReport {
	r := transportFixture()
	return ComparisonReport{
		Schema: ComparisonSchema, Status: ComparisonStatus, Scope: ComparisonScope,
		BeforeProfile: r.BeforeProfile, CandidateProfile: r.CandidateProfile,
		Before: r.Before, After: r.Before, Summary: Summary{Classification: "UNCHANGED"}, ClassifierBindings: r.Before.Bindings,
	}
}

func TestRequestStrictNestedTransport(t *testing.T) {
	request := transportFixture()
	raw := marshalTest(t, request)
	candidateField := append([]byte(`"candidate_profile":`), marshalTest(t, request.CandidateProfile)...)
	if bytes.Count(raw, candidateField) != 1 {
		t.Fatal("transport fixture must contain one candidate profile field")
	}
	decoded, err := DecodeCompareRequest(raw)
	if err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatalf("typed transport roundtrip: %v", err)
	}
	malformed := [][]byte{
		[]byte("null"), []byte("[]"), append(append([]byte(nil), raw...), []byte(" {}")...),
		bytes.Replace(raw, []byte(`"before_profile":"cel-api-original-v1"`), []byte(`"before_profile":null`), 1),
		bytes.Replace(raw, candidateField, []byte(`"candidate_profile":17`), 1),
		bytes.Replace(raw, []byte(`"before":{`), []byte(`"unexpected":0,"before":{`), 1),
		bytes.Replace(raw, []byte(`"before":{`), []byte(`"before":null,"discard":{`), 1),
		bytes.Replace(raw, []byte(`"before_profile":`), []byte(`"before_profile":"cel-api-original-v1","before_profile":`), 1),
		bytes.Replace(raw, []byte(`"before_profile":`), []byte(`"\u0062efore_profile":"cel-api-original-v1","before_profile":`), 1),
		bytes.Replace(raw, []byte(`"group_present":true`), []byte(`"group_present":false,"group_present":true`), 1),
		bytes.Replace(raw, []byte(`"group_present":true`), []byte(`"\u0067roup_present":false,"group_present":true`), 1),
		bytes.Replace(raw, []byte(`"group_present":true`), []byte(`"group_present":1`), 1),
		bytes.Replace(raw, []byte(`"group_value":"admin"`), []byte(`"group_value":"\ud800"`), 1),
		bytes.Replace(raw, []byte(`"group_value":"admin"`), []byte(`"group_value":"\udc00"`), 1),
		bytes.Replace(raw, []byte(`"group_value":"admin"`), []byte{'"', 'g', 'r', 'o', 'u', 'p', '_', 'v', 'a', 'l', 'u', 'e', '"', ':', '"', 0xff, '"'}, 1),
		bytes.Replace(raw, []byte(`"allowed":false`), []byte(`"allowed":null`), 1),
		bytes.Replace(raw, []byte(`"reason":""`), []byte(`"reason":false`), 1),
		bytes.Replace(raw, []byte(`"assets":{`), []byte(`"assets":null,"extra":{`), 1),
		bytes.Repeat([]byte(" "), MaxInputBytes+1),
	}
	for index, input := range malformed {
		if _, err := DecodeCompareRequest(input); err == nil {
			t.Fatalf("malformed request %d accepted", index)
		}
	}
	for _, id := range []string{"", "cel-api-original-v0", CandidateProfile, OriginalProfile + " "} {
		current := request
		current.BeforeProfile = id
		if _, err := DecodeCompareRequest(marshalTest(t, current)); err == nil {
			t.Fatalf("unknown/mixed original profile %q accepted", id)
		}
	}
	for _, id := range []string{"", "cel-api-readonly-v0", OriginalProfile, CandidateProfile + " "} {
		current := request
		current.CandidateProfile = id
		if _, err := DecodeCompareRequest(marshalTest(t, current)); err == nil {
			t.Fatalf("unknown/mixed candidate profile %q accepted", id)
		}
	}
}

func TestTransportPreservesPairedEscapesAndInactiveRawText(t *testing.T) {
	request := transportFixture()
	request.Before.Facts.GroupPresent = false
	request.Before.Facts.GroupValue = "\U0001f600\x00\r\n"
	request.Before.Facts.PrincipalValue = "user:me@acme.co"
	raw := marshalTest(t, request)
	raw = bytes.Replace(raw, []byte("\U0001f600"), []byte(`\ud83d\ude00`), 1)
	actual, err := DecodeCompareRequest(raw)
	if err != nil || actual.Before.Facts != request.Before.Facts {
		t.Fatalf("raw facts altered: %v", err)
	}
	request.Before.Facts.GroupValue = strings.Repeat("x", original.MaxTextBytes+1)
	if _, err := DecodeCompareRequest(marshalTest(t, request)); err == nil {
		t.Fatal("oversized inactive placeholder accepted")
	}
}

func TestAllGeneratedFlagAndSummaryFieldsAreRequired(t *testing.T) {
	flags := DifferenceFlags{Allowed: true, Reason: true}
	raw := marshalTest(t, flags)
	actual, err := decodeDifferences(raw)
	if err != nil || actual != flags {
		t.Fatalf("generated flags lost or changed: %+v %v", actual, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	for _, name := range decisionFields {
		for _, replacement := range []json.RawMessage{nil, []byte("null"), []byte("0"), []byte(`"false"`)} {
			current := make(map[string]json.RawMessage, len(object))
			for key, value := range object {
				current[key] = value
			}
			if replacement == nil {
				delete(current, name)
			} else {
				current[name] = replacement
			}
			if _, err := decodeDifferences(marshalTest(t, current)); err == nil {
				t.Fatalf("flag %s missing/wrong type accepted", name)
			}
		}
	}
	for _, input := range []string{
		`{}`, `{"classification":"UNCHANGED"}`, `{"classification":"UNCHANGED","changed_count":null}`,
		`{"classification":"UNCHANGED","changed_count":1.0}`, `{"classification":"UNCHANGED","changed_count":1e0}`,
		`{"classification":"UNCHANGED","changed_count":-1}`, `{"classification":"UNCHANGED","changed_count":16}`,
		`{"classification":"unknown","changed_count":0}`, `{"classification":"UNCHANGED","changed_count":0,"extra":true}`,
		`{"classification":"UNCHANGED","changed_count":0,"changed_count":0}`,
	} {
		if _, err := decodeSummary([]byte(input)); err == nil {
			t.Fatalf("malformed summary accepted: %s", input)
		}
	}
}

func TestReportNestedTransportRejectsMalformedLateFields(t *testing.T) {
	report := transportReport()
	raw := marshalTest(t, report)
	decoded, err := DecodeComparisonReport(raw)
	if err != nil || !reflect.DeepEqual(decoded, report) {
		t.Fatalf("report transport roundtrip: %v", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	for name := range root {
		for _, missing := range []bool{false, true} {
			current := make(map[string]json.RawMessage, len(root))
			for key, value := range root {
				current[key] = value
			}
			if missing {
				delete(current, name)
			} else {
				current[name] = []byte("null")
			}
			if _, err := DecodeComparisonReport(marshalTest(t, current)); err == nil {
				t.Fatalf("null/missing report field %s accepted", name)
			}
		}
	}
	for _, malformed := range [][]byte{
		append(append([]byte(nil), raw...), []byte(" false")...),
		bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"duplicate","\u0073chema":`), 1),
		bytes.Replace(raw, []byte(`"effective_principal":false`), []byte(`"effective_principal":0`), 1),
		bytes.Replace(raw, []byte(`"changed_count":0`), []byte(`"changed_count":true`), 1),
		bytes.Replace(raw, []byte(`"classifier_bindings":{`), []byte(`"unexpected":0,"classifier_bindings":{`), 1),
		bytes.Replace(raw, []byte(`"source.gooo":`), []byte(`"source.gooo":"duplicate","\u0073ource.gooo":`), -1),
	} {
		if _, err := DecodeComparisonReport(malformed); err == nil {
			t.Fatal("malformed nested report accepted")
		}
	}
}
