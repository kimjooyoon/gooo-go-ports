package grantselection

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestJSONInputHashAndNoCallerVerificationFields(t *testing.T) {
	raw, err := json.MarshalIndent(sample(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	doc, err := AssessJSON(raw)
	if err != nil || doc.InputSHA256 != digest(raw) || doc.Result.Selection.SelectedCandidateID != "all-mcp" {
		t.Fatalf("JSON assessment changed: %+v %v", doc, err)
	}
	for _, field := range []string{"bindings_verified", "statuses", "phase", "assessment_ids", "context_ids"} {
		bad := append([]byte(`{"`+field+`":true,`), raw[1:]...)
		if got, err := AssessJSON(bad); err == nil || !reflect.DeepEqual(got, Document{}) {
			t.Fatalf("external field admitted: %s", field)
		}
	}
}

func TestLossyOrLateMalformedInputHasNoDocument(t *testing.T) {
	raw, _ := json.Marshal(sample())
	text := string(raw)
	cases := []string{
		strings.Replace(text, `"id":"all-mcp"`, `"id":"all-mcp","id":"different"`, 1),
		strings.Replace(text, `"all-mcp"`, `null`, 1),
		strings.Replace(text, `"all-mcp"`, `"\ud800"`, 1),
		strings.Replace(text, `"preference":10`, `"preference":null`, 1),
		strings.Replace(text, `"preference":10`, `"preference":1.0`, 1),
		strings.Replace(text, `"preference":10`, `"preference":9223372036854775808`, 1),
		strings.Replace(text, `"before":"*"`, `"before":"*","before":"mcp://*"`, 1),
		strings.Replace(text, `"required_requests":[`, `"required_requests":null,"extra":[`, 1),
		text + ` {}`, strings.Repeat(" ", MaxInputBytes+1), string([]byte{0xff}),
	}
	for i, bad := range cases {
		if bad == text {
			t.Fatalf("control%d did not mutate the input", i)
		}
		if got, err := AssessJSON([]byte(bad)); err == nil || !reflect.DeepEqual(got, Document{}) {
			t.Fatalf("control%d leaked a partial document: %+v %v", i, got, err)
		}
	}
}

func TestUnicodePairAndSourceInvalidPatternArePreserved(t *testing.T) {
	raw, _ := json.Marshal(sample())
	text := strings.Replace(string(raw), `"weather-only"`, `"\ud83d\ude80"`, 1)
	text = strings.Replace(text, `"mcp://weather"`, `"mcp://weather/path"`, 1)
	doc, err := AssessJSON([]byte(text))
	if err != nil || doc.Result.Facts.Candidates[0].ID != "🚀" || doc.Result.Assessments[0].Proof.Facts.After != "mcp://weather/path" || doc.Result.Assessments[0].Proof.Decision.Status != "INVALID_INPUT" {
		t.Fatalf("valid Unicode or represented invalid domain changed: %+v %v", doc, err)
	}
}
