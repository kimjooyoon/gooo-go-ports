package revisionpolicy

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func principalCases(t *testing.T) []map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("testdata/principal-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []map[string]json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 7 {
		t.Fatal("principal expectation population differs", err, len(fixture.Cases))
	}
	names := map[string]bool{}
	for _, row := range fixture.Cases {
		var name string
		principalDecode(t, row, "name", &name)
		if name == "" || names[name] {
			t.Fatal("principal expectation name differs", name)
		}
		names[name] = true
		for _, role := range []string{"original_expected", "candidate_expected"} {
			var fields map[string]json.RawMessage
			principalDecode(t, row, role, &fields)
			if len(fields) != 15 {
				t.Fatal("principal decision must specify all fifteen fields", name, role, len(fields))
			}
			shape := reflect.TypeOf(original.APIDecision{})
			for i := 0; i < shape.NumField(); i++ {
				key := shape.Field(i).Tag.Get("json")
				if _, ok := fields[key]; !ok {
					t.Fatal("principal decision field absent", name, role, key)
				}
			}
		}
	}
	return fixture.Cases
}

func principalDecode(t *testing.T, row map[string]json.RawMessage, key string, value any) {
	t.Helper()
	raw, ok := row[key]
	if !ok {
		t.Fatal("principal expectation field absent", key)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		t.Fatal("principal expectation field invalid", key, err)
	}
}

func TestExplicitPrincipalAllFieldsAndSavedComparison(t *testing.T) {
	for _, row := range principalCases(t) {
		var name string
		principalDecode(t, row, "name", &name)
		t.Run(name, func(t *testing.T) {
			var facts original.APIFacts
			var before, after original.APIDecision
			var comparison map[string]json.RawMessage
			var summary Summary
			var differences DifferenceFlags
			principalDecode(t, row, "facts", &facts)
			principalDecode(t, row, "original_expected", &before)
			principalDecode(t, row, "candidate_expected", &after)
			principalDecode(t, row, "comparison_expected", &comparison)
			principalDecode(t, row, "comparison_expected", &summary)
			principalDecode(t, comparison, "differences", &differences)
			report := nativeReport(t, facts)
			if report.BeforeProfile != OriginalProfile || report.CandidateProfile != CandidateProfile ||
				report.Before.Facts != facts || report.After.Facts != facts ||
				report.Before.Decision != before || report.After.Decision != after ||
				report.Differences != differences || report.Summary != summary {
				t.Fatalf("literal complete principal report differs: %+v", report)
			}
			decoded, err := DecodeComparisonReport(marshalTest(t, report))
			if err != nil {
				t.Fatal(err)
			}
			verified, err := VerifyComparison(decoded)
			if err != nil || !reflect.DeepEqual(verified, report) {
				t.Fatal("principal saved comparison changed", err)
			}
			_, originErr := original.VerifyAPIProof(report.After)
			if !errors.Is(originErr, original.ErrAPIGenerationMismatch) {
				t.Fatal("candidate observation was accepted as the original generation", originErr)
			}
		})
	}
}

func TestPriorReadonlyProfileRefusesRebinding(t *testing.T) {
	request, err := Baseline(allowedFacts("read"))
	if err != nil {
		t.Fatal(err)
	}
	request.CandidateProfile = "cel-api-readonly-v1"
	if _, err := Compare(request); !errors.Is(err, ErrProfileMismatch) {
		t.Fatal("prior readonly request was rebound to a new profile", err)
	}
	report := nativeReport(t, allowedFacts("read"))
	report.CandidateProfile = "cel-api-readonly-v1"
	if _, err := VerifyComparison(report); !errors.Is(err, ErrProfileMismatch) {
		t.Fatal("prior readonly saved report was rebound to a new profile", err)
	}
}
