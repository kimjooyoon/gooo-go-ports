// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/observationkernel"
)

// ObservationProfile maps exact legacy text fields into explicit source documents.
// Empty legacy text is interpreted by Gooo, never replaced by an invented ID.
const ObservationProfile = "gooo/recorded-observation-fields/v1"

// ObservationFieldInput and ObservationDiagnostic use the source-owned typed plan.
type ObservationFieldInput = observationkernel.FieldInput
type ObservationDiagnostic = observationkernel.Result

//go:embed observation-generation.json
var observationGeneration []byte

// SuppliedPolicyText records normalized API argument values, not whether an
// original command-line option existed or a historical policy was observed.
type SuppliedPolicyText struct {
	BeforeRevision string `json:"before_revision"`
	AfterRevision  string `json:"after_revision"`
}

type ObservationProjection struct {
	Schema      string `json:"schema"`
	Profile     string `json:"profile"`
	InputSHA256 string `json:"input_sha256"`
	FactsSHA256 string `json:"facts_sha256"`
	// Paths address either the original recorded input or the supplied policy
	// document {before_revision, after_revision}, as declared by each source field.
	SuppliedPolicy SuppliedPolicyText      `json:"supplied_policy"`
	Inputs         []ObservationFieldInput `json:"inputs"`
}

type DefinitionDiagnosticsReport struct {
	Schema                string                  `json:"schema"`
	Comparison            DefinitionReport        `json:"comparison"`
	ObservationProjection ObservationProjection   `json:"observation_projection"`
	Observations          []ObservationDiagnostic `json:"observations"`
	ObservationBindings   json.RawMessage         `json:"observation_bindings"`
}

// observationFields projects every bounded source field in traversal order.
// It does not decide which values are missing, required, equal or changed.
func observationFields(facts Input, inputHash, factsHash, beforePolicy, afterPolicy string) (ObservationProjection, error) {
	p := ObservationProjection{Schema: "gooo/recorded-observation-input-projection/v1", Profile: ObservationProfile, InputSHA256: inputHash, FactsSHA256: factsHash, SuppliedPolicy: SuppliedPolicyText{beforePolicy, afterPolicy}, Inputs: []ObservationFieldInput{}}
	add := func(document, path, kind, basis, value, scope, expectedDocument, expectedPath, expectedValue string) error {
		field := ObservationFieldInput{SourceDocument: document, FieldPath: path, FieldKind: kind, RecordingBasis: basis, ValueState: "LEGACY_TEXT", Value: value, ComparisonScope: scope, ExpectedSourceDocument: expectedDocument, ExpectedFieldPath: expectedPath, ExpectedValueState: "LEGACY_TEXT", ExpectedValue: expectedValue}
		if scope == "NONE" {
			field.ExpectedValueState = "NOT_APPLICABLE"
		}
		raw, err := json.Marshal(struct {
			Profile     string
			InputSHA256 string
			Field       ObservationFieldInput
		}{ObservationProfile, inputHash, field})
		if err != nil {
			return err
		}
		field.ContextID = "gooo/observation-field/v1/" + digest(raw)
		p.Inputs = append(p.Inputs, field)
		return nil
	}
	addRaw := func(path, kind, basis, value, scope, expectedPath, expectedValue string) error {
		expectedDocument := "RECORDED_INPUT"
		if scope == "NONE" {
			expectedDocument = "NOT_APPLICABLE"
		}
		return add("RECORDED_INPUT", path, kind, basis, value, scope, expectedDocument, expectedPath, expectedValue)
	}
	for n, o := range []Observation{facts.Before, facts.After} {
		side := "before"
		if n == 1 {
			side = "after"
		}
		scope, expectedPath, expectedValue := "NONE", "", ""
		if n == 1 {
			scope = "BETWEEN_CATALOGS"
			expectedPath = "/before/catalog_authorization_context_id"
			expectedValue = facts.Before.CatalogAuthorizationContextID
		}
		if err := addRaw("/"+side+"/catalog_authorization_context_id", "OPAQUE_CONTEXT_ID", "HOST_DECLARATION", o.CatalogAuthorizationContextID, scope, expectedPath, expectedValue); err != nil {
			return ObservationProjection{}, err
		}
		for captureIndex, c := range o.Captures {
			prefix := fmt.Sprintf("/%s/captures/%d", side, captureIndex)
			contexts := []struct {
				path    string
				value   Context
				compare bool
			}{{prefix + "/begin", c.Begin, false}, {prefix + "/end", c.End, true}}
			for pageIndex, page := range c.Pages {
				contexts = append(contexts, struct {
					path    string
					value   Context
					compare bool
				}{fmt.Sprintf("%s/pages/%d/context", prefix, pageIndex), page.Context, true})
			}
			for _, item := range contexts {
				fields := []struct{ key, kind, value, begin string }{
					{"backend_binding_revision", "BACKEND_BINDING_REVISION", item.value.BackendBindingRevision, c.Begin.BackendBindingRevision},
					{"authorization_context_id", "OPAQUE_CONTEXT_ID", item.value.AuthorizationContextID, c.Begin.AuthorizationContextID},
					{"relevant_configuration_revision", "CONFIGURATION_REVISION", item.value.RelevantConfigurationRevision, c.Begin.RelevantConfigurationRevision},
					{"protocol_version", "PROTOCOL_VERSION", item.value.ProtocolVersion, c.Begin.ProtocolVersion},
					{"captured_at", "CAPTURE_TIMESTAMP", item.value.CapturedAt, ""},
				}
				for _, f := range fields {
					scope, expectedPath, expectedValue := "NONE", "", ""
					// Timestamp values are facts, not an equality/stability predicate.
					if item.compare && f.key != "captured_at" {
						scope = "SAME_CAPTURE"
						expectedPath = prefix + "/begin/" + f.key
						expectedValue = f.begin
					}
					if err := addRaw(item.path+"/"+f.key, f.kind, c.ContextRecordingBasis, f.value, scope, expectedPath, expectedValue); err != nil {
						return ObservationProjection{}, err
					}
				}
			}
		}
	}
	for n, value := range []string{beforePolicy, afterPolicy} {
		path, scope, expectedPath, expectedDocument, expectedValue := "/before_revision", "NONE", "", "NOT_APPLICABLE", ""
		if n == 1 {
			path = "/after_revision"
			scope = "BETWEEN_CATALOGS"
			expectedPath = "/before_revision"
			expectedDocument = "SUPPLIED_POLICY"
			expectedValue = beforePolicy
		}
		if err := add("SUPPLIED_POLICY", path, "POLICY_REVISION", "HOST_DECLARATION", value, scope, expectedDocument, expectedPath, expectedValue); err != nil {
			return ObservationProjection{}, err
		}
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return ObservationProjection{}, err
	}
	if len(raw) > MaxReportBytes {
		return ObservationProjection{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return p, nil
}

// ProjectObservationInputsJSON retains all source fields, even those that are
// present or optional. Gooo owns availability, comparison and next-action rules.
func ProjectObservationInputsJSON(raw []byte, beforePolicy, afterPolicy string) (ObservationProjection, error) {
	// Reuse the definition projection's existing strict policy-text validation.
	definition, err := ProjectDefinitionInputsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return ObservationProjection{}, err
	}
	parsed, err := projectJSON(raw)
	if err != nil {
		return ObservationProjection{}, err
	}
	if definition.InputSHA256 != digest(raw) || definition.FactsSHA256 != parsed.FactsSHA256 {
		return ObservationProjection{}, &Problem{State: "UNKNOWN", Stage: "bind_observation_inputs", Code: "SOURCE_CONTEXT_MISMATCH"}
	}
	return observationFields(parsed.Facts, definition.InputSHA256, parsed.FactsSHA256, beforePolicy, afterPolicy)
}

// CompareDefinitionDiagnosticsJSON adds a versioned diagnostic report without
// changing the legacy comparison or granting authority. Keep the original input
// file with this report; hashes alone cannot reconstruct its saved frames.
func CompareDefinitionDiagnosticsJSON(raw []byte, beforePolicy, afterPolicy string) (result DefinitionDiagnosticsReport, returned error) {
	defer func() {
		if recover() != nil {
			result = DefinitionDiagnosticsReport{}
			returned = &Problem{State: "UNKNOWN", Stage: "evaluate_observation", Code: "OBSERVATION_KERNEL_FAILURE"}
		}
	}()
	comparison, err := CompareDefinitionJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return DefinitionDiagnosticsReport{}, err
	}
	projected, err := ProjectObservationInputsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return DefinitionDiagnosticsReport{}, err
	}
	if comparison.Projection.InputSHA256 != projected.InputSHA256 || comparison.Projection.FactsSHA256 != projected.FactsSHA256 {
		return DefinitionDiagnosticsReport{}, &Problem{State: "UNKNOWN", Stage: "bind_observation_inputs", Code: "SOURCE_CONTEXT_MISMATCH"}
	}
	observations := make([]ObservationDiagnostic, 0, len(projected.Inputs))
	for _, input := range projected.Inputs {
		actual := observationkernel.Evaluate(input)
		if actual.ContextID != input.ContextID || actual.SourceDocument != input.SourceDocument || actual.ExpectedSourceDocument != input.ExpectedSourceDocument || actual.FieldPath != input.FieldPath || actual.FieldKind != input.FieldKind || actual.RecordingBasis != input.RecordingBasis || actual.Value != input.Value || actual.ComparisonScope != input.ComparisonScope || actual.ExpectedValue != input.ExpectedValue || actual.ExpectedFieldPath != input.ExpectedFieldPath || actual.ProducerAuthenticated || actual.ExecutionAuthorized || actual.LiveEffectPerformed || actual.BehaviorEquivalenceProven {
			return DefinitionDiagnosticsReport{}, &Problem{State: "UNKNOWN", Stage: "bind_observation_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
		}
		observations = append(observations, actual)
	}
	result = DefinitionDiagnosticsReport{Schema: "gooo/definition-observation-diagnostics/v1", Comparison: comparison, ObservationProjection: projected, Observations: observations, ObservationBindings: bytes.Clone(observationGeneration)}
	encoded, err := json.Marshal(result)
	if err != nil {
		return DefinitionDiagnosticsReport{}, err
	}
	if len(encoded) > MaxReportBytes {
		return DefinitionDiagnosticsReport{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return result, nil
}
