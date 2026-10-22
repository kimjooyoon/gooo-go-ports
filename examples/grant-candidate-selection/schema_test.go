// Copyright (c) 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package grantselection_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	selection "github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection"
)

func schemaFor(t *testing.T, name string) *jsonschema.Resolved {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("schemas", name+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func instance(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func originalInput(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("example-input.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSchemasCoverActualPublicResultsAndRefuseEffectAuthority(t *testing.T) {
	input, output := schemaFor(t, "input"), schemaFor(t, "output")
	raw := originalInput(t)
	for _, tc := range []struct{ name, old, new, state string }{
		{"proposed", "", "", "PROPOSED"},
		{"refused", "mcp://*", "http://*", "REFUSED"},
		{"source-owned-invalid", "\"preference\": 10", "\"preference\": -1", "INVALID_INPUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := raw
			if tc.old != "" {
				data = []byte(strings.Replace(string(raw), tc.old, tc.new, 1))
			}
			var facts any
			if err := json.Unmarshal(data, &facts); err != nil {
				t.Fatal(err)
			}
			if err := input.Validate(facts); err != nil {
				t.Fatal(err)
			}
			document, err := selection.AssessJSON(data)
			if err != nil {
				t.Fatal(err)
			}
			if document.Result.Selection.State != tc.state || document.Result.Selection.ExecutionAuthorized {
				t.Fatalf("unexpected source result: %+v", document.Result.Selection)
			}
			value := instance(t, document)
			if err := output.Validate(value); err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)["result"].(map[string]any)
			choice := result["selection"].(map[string]any)
			choice["execution_authorized"] = true
			if err := output.Validate(value); err == nil {
				t.Fatal("schema accepted effect authority")
			}
		})
	}
}

func TestSchemasDoNotReplaceNativeParsingOrSourceCorrespondence(t *testing.T) {
	input, output := schemaFor(t, "input"), schemaFor(t, "output")
	raw := originalInput(t)
	// JSON Schema sees a numeric integer; native decoding also checks its token.
	decimal := []byte(strings.Replace(string(raw), "\"preference\": 10", "\"preference\": 10.0", 1))
	var facts any
	if err := json.Unmarshal(decimal, &facts); err != nil {
		t.Fatal(err)
	}
	if err := input.Validate(facts); err != nil {
		t.Fatal(err)
	}
	if _, err := selection.AssessJSON(decimal); err == nil {
		t.Fatal("native parser accepted decimal int64 spelling")
	}
	// The input shape leaves source admission to Gooo rather than prefiltering it.
	one, err := selection.DecodeInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	one.Candidates = one.Candidates[:1]
	if err := input.Validate(instance(t, one)); err != nil {
		t.Fatal(err)
	}
	result, err := selection.Assess(one)
	if err != nil || result.Selection.State != "INVALID_INPUT" || len(result.Assessments) != 0 {
		t.Fatalf("Gooo admission result: %+v %v", result, err)
	}
	// A well-shaped invented hash passes structure: it is not a verified proof.
	document, err := selection.AssessJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	changed := instance(t, document)
	changed.(map[string]any)["result"].(map[string]any)["selection"].(map[string]any)["selected_assessment_id"] = "sha256:" + strings.Repeat("0", 64)
	if err := output.Validate(changed); err != nil {
		t.Fatalf("structural schema incorrectly claims correspondence: %v", err)
	}
}

func TestSchemasCoverActualDiagnosticsAndRejectCallerVerification(t *testing.T) {
	input, diagnostic := schemaFor(t, "input"), schemaFor(t, "diagnostic")
	raw := originalInput(t)
	extra := []byte(strings.Replace(string(raw), "\"preference\": 10", "\"preference\": 10, \"bindings_verified\": true", 1))
	var supplied any
	if err := json.Unmarshal(extra, &supplied); err != nil {
		t.Fatal(err)
	}
	if err := input.Validate(supplied); err == nil {
		t.Fatal("schema accepted caller verification")
	}
	_, err := selection.AssessJSON(extra)
	var problem *selection.Problem
	if !errors.As(err, &problem) || problem.State != "INVALID_TRANSPORT" {
		t.Fatalf("unexpected actual diagnostic: %v", err)
	}
	if err := diagnostic.Validate(instance(t, problem)); err != nil {
		t.Fatal(err)
	}
	values, err := selection.DecodeInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	values.Before = string([]byte{255})
	_, err = selection.Assess(values)
	if !errors.As(err, &problem) || problem.Stage != "admit_text" || problem.Code != "INVALID_UTF8" {
		t.Fatalf("unexpected text diagnostic: %v", err)
	}
	if err := diagnostic.Validate(instance(t, problem)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []selection.Problem{
		{State: "REFUSED", Stage: "bind_assessment", Code: "ASSESSMENT_BINDING_MISMATCH", Index: 1},
		{State: "UNKNOWN", Stage: "bind_assessment", Code: "INVALID_JSON_FACTS", Index: 1},
	} {
		if err := diagnostic.Validate(instance(t, bad)); err == nil {
			t.Fatalf("schema accepted mismatched diagnostic: %+v", bad)
		}
	}
}
