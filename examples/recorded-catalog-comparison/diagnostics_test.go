// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"encoding/json"
	"errors"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/observationkernel"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func diagnosticFixture(t *testing.T) Input {
	t.Helper()
	var in Input
	if err := json.Unmarshal(ExampleInputJSON(), &in); err != nil {
		t.Fatal(err)
	}
	in.Before.DeclaredConnections = in.Before.DeclaredConnections[:1]
	in.Before.Captures = in.Before.Captures[:1]
	raw, err := json.Marshal(in.Before)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &in.After); err != nil {
		t.Fatal(err)
	}
	return in
}
func encodedDiagnosticFixture(t *testing.T, in Input) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func missingDiagnosticFixture(t *testing.T) Input {
	t.Helper()
	in := diagnosticFixture(t)
	for _, side := range []*Observation{&in.Before, &in.After} {
		side.CatalogAuthorizationContextID = ""
		for n := range side.Captures {
			c := &side.Captures[n]
			c.ContextRecordingBasis = "RETROSPECTIVE"
			c.ConfigurationWatch.Coverage = "UNKNOWN"
			contexts := []*Context{&c.Begin, &c.End}
			for j := range c.Pages {
				contexts = append(contexts, &c.Pages[j].Context)
			}
			for _, v := range contexts {
				v.BackendBindingRevision = ""
				v.AuthorizationContextID = ""
				v.RelevantConfigurationRevision = ""
				v.CapturedAt = ""
			}
		}
	}
	return in
}
func TestObservationGeneratedFiniteCases(t *testing.T) {
	raw, err := os.ReadFile("observation-kernel-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]ObservationFieldInput `json:"inputs"`
			Expected map[string]ObservationDiagnostic `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 51 {
		t.Fatal(len(corpus.Cases))
	}
	const entry = "gooo/tool-origin/observation-diagnostics:AssessObservationField"
	for n, c := range corpus.Cases {
		got := observationkernel.Evaluate(c.Inputs[entry])
		want := c.Expected[entry]
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case%d:\n%+v\n!=\n%+v", n, got, want)
		}
	}
}
func TestDiagnosticMissingPathsAndUnchangedComparison(t *testing.T) {
	raw := encodedDiagnosticFixture(t, missingDiagnosticFixture(t))
	report, err := CompareDefinitionDiagnosticsJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	original, err := CompareDefinitionJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Comparison, original) {
		t.Fatal("diagnostics changed original comparison")
	}
	if report.Comparison.Decision.State != "UNASSESSED" || report.Comparison.Decision.CollectionReason != "INVALID_COLLECTION_COLUMNS" {
		t.Fatal(report.Comparison.Decision)
	}
	if len(report.Observations) != 34 || len(report.ObservationProjection.Inputs) != 34 {
		t.Fatal("all source observations must be retained")
	}
	expected := map[string]bool{}
	for _, side := range []string{"before", "after"} {
		expected["/"+side+"/catalog_authorization_context_id"] = true
		for _, prefix := range []string{"begin", "end", "pages/0/context"} {
			for _, key := range []string{"backend_binding_revision", "authorization_context_id", "relevant_configuration_revision"} {
				expected["/"+side+"/captures/0/"+prefix+"/"+key] = true
			}
		}
	}
	optional, protocol := 0, 0
	for _, got := range report.Observations {
		if got.ProducerAuthenticated || got.BehaviorEquivalenceProven || got.ExecutionAuthorized || got.LiveEffectPerformed {
			t.Fatal("diagnostic granted authority")
		}
		if got.RequiredForContext && got.ValueState == "UNOBSERVED" {
			if got.SourceDocument != "RECORDED_INPUT" || !expected[got.FieldPath] || got.State != "UNASSESSED" || got.NextAction != "OBTAIN_OBSERVATION" {
				t.Fatal(got)
			}
			delete(expected, got.FieldPath)
		}
		if got.FieldKind == "CAPTURE_TIMESTAMP" || got.FieldKind == "POLICY_REVISION" {
			optional++
			if got.RequiredForContext || got.State != "OPTIONAL_UNOBSERVED" || got.NextAction != "PRESERVE_UNOBSERVED" {
				t.Fatal(got)
			}
		}
		if got.FieldKind == "PROTOCOL_VERSION" {
			protocol++
			if got.ValueState != "SUPPLIED" {
				t.Fatal(got)
			}
		}
	}
	if len(expected) != 0 || optional != 8 || protocol != 6 {
		t.Fatal(expected, optional, protocol)
	}
	again, err := CompareDefinitionDiagnosticsJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report, again) {
		t.Fatal("same saved input changed report")
	}
}
func TestKnownChangeRemainsVisibleAlongsideMissingObservation(t *testing.T) {
	in := diagnosticFixture(t)
	in.Before.CatalogAuthorizationContextID = ""
	in.After.Captures[0].Pages[0].Context.BackendBindingRevision = "known-other-backend"
	report, err := CompareDefinitionDiagnosticsJSON(encodedDiagnosticFixture(t, in), "fixture-policy", "fixture-policy")
	if err != nil {
		t.Fatal(err)
	}
	missing, changed := false, false
	for _, d := range report.Observations {
		if d.FieldPath == "/before/catalog_authorization_context_id" {
			missing = d.State == "UNASSESSED" && d.NextAction == "OBTAIN_OBSERVATION"
		}
		if d.FieldPath == "/after/captures/0/pages/0/context/backend_binding_revision" {
			changed = d.State == "KNOWN_MISMATCH" && d.KnownChange && d.NextAction == "RECOLLECT_CONTEXT" && d.ExpectedFieldPath == "/after/captures/0/begin/backend_binding_revision"
		}
	}
	if !missing || !changed || report.Comparison.Decision.ExecutionAuthorized {
		t.Fatal(missing, changed, report.Comparison.Decision)
	}
}
func TestTimestampChangesAreFactsNotContextEqualityFailures(t *testing.T) {
	in := diagnosticFixture(t)
	in.After.Captures[0].Begin.CapturedAt = "time1"
	in.After.Captures[0].End.CapturedAt = "time2"
	in.After.Captures[0].Pages[0].Context.CapturedAt = "time3"
	report, err := CompareDefinitionDiagnosticsJSON(encodedDiagnosticFixture(t, in), "fixture-policy", "fixture-policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Observations {
		if d.FieldKind == "CAPTURE_TIMESTAMP" && (d.State != "SUPPLIED_ONLY" || d.ComparisonScope != "NONE" || d.RequiredForContext) {
			t.Fatal(d)
		}
	}
	if report.Comparison.Decision.State != "BINDINGS_MATCH" {
		t.Fatal(report.Comparison.Decision)
	}
}
func TestDiagnosticMalformedInputAndPolicyBoundaries(t *testing.T) {
	var shape map[string]any
	if err := json.Unmarshal(encodedDiagnosticFixture(t, diagnosticFixture(t)), &shape); err != nil {
		t.Fatal(err)
	}
	delete(shape["before"].(map[string]any), "catalog_authorization_context_id")
	raw, err := json.Marshal(shape)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CompareDefinitionDiagnosticsJSON(raw, "", "")
	var problem *Problem
	if !errors.As(err, &problem) || problem.State != "INVALID_TRANSPORT" || problem.Code != "EXACT_OBJECT_REQUIRED" || got.Schema != "" {
		t.Fatal(got, err)
	}
	if _, err = CompareDefinitionDiagnosticsJSON(ExampleInputJSON(), strings.Repeat("x", MaxTextBytes+1), ""); err == nil {
		t.Fatal("oversize supplied policy accepted")
	}
}
func TestAllBoundedObservationFieldsAreProjected(t *testing.T) {
	in := diagnosticFixture(t)
	for _, side := range []*Observation{&in.Before, &in.After} {
		template := side.Captures[0]
		side.Captures = nil
		for n := 0; n < MaxConnections; n++ {
			c := template
			c.Pages = make([]Page, MaxPages)
			for j := range c.Pages {
				c.Pages[j].Context = template.Begin
			}
			side.Captures = append(side.Captures, c)
		}
	}
	// Structural mapping only; these fabricated pages are not valid capture evidence.
	p, err := observationFields(in, "synthetic-input-hash", "synthetic-facts-hash", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Inputs) != 1444 {
		t.Fatal(len(p.Inputs))
	}
}
func TestObservationConsumerExport(t *testing.T) {
	type record struct {
		Label        string                      `json:"label"`
		RawInputJSON string                      `json:"raw_input_json"`
		BeforePolicy string                      `json:"before_policy_revision"`
		AfterPolicy  string                      `json:"after_policy_revision"`
		Report       DefinitionDiagnosticsReport `json:"report"`
	}
	baseline := diagnosticFixture(t)
	missing := missingDiagnosticFixture(t)
	changed := diagnosticFixture(t)
	changed.Before.CatalogAuthorizationContextID = ""
	changed.After.Captures[0].Pages[0].Context.BackendBindingRevision = "known-other-backend"
	fixtures := []struct {
		label         string
		input         Input
		before, after string
	}{{"supplied-baseline", baseline, "fixture-policy", "fixture-policy"}, {"twenty-missing-required-eight-optional", missing, "", ""}, {"known-change-and-unobserved", changed, "fixture-policy", "fixture-policy"}, {"policy-change", baseline, "fixture-policy-v1", "fixture-policy-v2"}}
	records := make([]record, 0, len(fixtures))
	for _, f := range fixtures {
		raw := encodedDiagnosticFixture(t, f.input)
		report, err := CompareDefinitionDiagnosticsJSON(raw, f.before, f.after)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record{f.label, string(raw), f.before, f.after, report})
	}
	if target := os.Getenv("GOOO_OBSERVATION_EXPORT"); target != "" {
		abs, err := filepath.Abs(target)
		if err != nil || abs != target {
			t.Fatal("absolute evidence export required")
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Dir(filepath.Dir(cwd))
		rel, err := filepath.Rel(root, abs)
		if err != nil || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("evidence export must be outside checkout")
		}
		raw, err := json.Marshal(records)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(append(raw, '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
}
