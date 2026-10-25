// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"reflect"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/availabilitykernel"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/availabilityobservationkernel"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/kernel"
)

const AvailabilitySchema = "gooo/explicit-observation-availability/v1"
const ExplicitObservationProfile = "gooo/recorded-explicit-observation-fields/v1"
const availabilityEntry = "gooo/tool-origin/explicit-availability:AssessDefinitionAvailability"

// AvailabilityField declares presence separately from the original opaque text.
// Gooo owns declaration validity and all consequences for comparison.
type AvailabilityField struct {
	SourceDocument string `json:"source_document"`
	FieldPath      string `json:"field_path"`
	ValueState     string `json:"value_state"`
}

// AvailabilityDocument binds a complete field roster to one exact saved input
// and the normalized policy arguments. It never replaces the original bytes.
type AvailabilityDocument struct {
	Schema       string              `json:"schema"`
	InputSHA256  string              `json:"input_sha256"`
	PolicySHA256 string              `json:"policy_sha256"`
	Fields       []AvailabilityField `json:"fields"`
}

type ExplicitAvailabilityInput = availabilitykernel.AvailabilityInput
type ExplicitDefinitionDecision = availabilitykernel.Decision

type ExplicitObservationProjection struct {
	Schema             string                `json:"schema"`
	InputSHA256        string                `json:"input_sha256"`
	PolicySHA256       string                `json:"policy_sha256"`
	AvailabilitySHA256 string                `json:"availability_sha256"`
	SourceProjection   DefinitionProjection  `json:"source_projection"`
	FieldProjection    ObservationProjection `json:"field_projection"`
	Inputs             map[string]any        `json:"inputs"`
}

type ExplicitObservationReport struct {
	Schema              string                        `json:"schema"`
	Projection          ExplicitObservationProjection `json:"projection"`
	Decision            ExplicitDefinitionDecision    `json:"decision"`
	Observations        []ObservationDiagnostic       `json:"observations"`
	Bindings            json.RawMessage               `json:"bindings"`
	ObservationBindings json.RawMessage               `json:"observation_bindings"`
}

//go:embed availability-generation.json
var availabilityGeneration []byte

//go:embed availability-observation-generation.json
var availabilityObservationGeneration []byte

func policyTextDigest(beforePolicy, afterPolicy string) (string, error) {
	raw, err := json.Marshal(SuppliedPolicyText{beforePolicy, afterPolicy})
	if err != nil {
		return "", invalid("POLICY_ENCODING")
	}
	return digest(raw), nil
}

// AvailabilityTemplateJSON returns a source-bound complete roster with blank
// states. The caller must choose SUPPLIED or UNOBSERVED for every row. It does
// not infer availability from an empty value or a spelling such as UNKNOWN.
func AvailabilityTemplateJSON(raw []byte, beforePolicy, afterPolicy string) (AvailabilityDocument, error) {
	p, err := ProjectObservationInputsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return AvailabilityDocument{}, err
	}
	policyHash, err := policyTextDigest(beforePolicy, afterPolicy)
	if err != nil {
		return AvailabilityDocument{}, err
	}
	doc := AvailabilityDocument{Schema: AvailabilitySchema, InputSHA256: p.InputSHA256, PolicySHA256: policyHash, Fields: []AvailabilityField{}}
	for _, field := range p.Inputs {
		doc.Fields = append(doc.Fields, AvailabilityField{SourceDocument: field.SourceDocument, FieldPath: field.FieldPath})
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return AvailabilityDocument{}, invalid("AVAILABILITY_ENCODING")
	}
	if len(raw)+len(encoded) > MaxInputBytes {
		return AvailabilityDocument{}, invalid("INPUT_LIMIT")
	}
	return doc, nil
}

func fieldKey(document, path string) string { return document + "\x00" + path }

// ProjectExplicitObservationInputsJSON validates byte/policy/pointer bindings
// and projects structural indices. No raw text is normalized or substituted.
func ProjectExplicitObservationInputsJSON(raw, availabilityRaw []byte, beforePolicy, afterPolicy string) (result ExplicitObservationProjection, returned error) {
	defer func() {
		if recover() != nil {
			result = ExplicitObservationProjection{}
			returned = &Problem{State: "UNKNOWN", Stage: "project_availability", Code: "AVAILABILITY_PROJECTION_FAILURE"}
		}
	}()
	if len(raw)+len(availabilityRaw) > MaxInputBytes {
		return ExplicitObservationProjection{}, invalid("INPUT_LIMIT")
	}
	if err := strictJSON(availabilityRaw, MaxInputBytes); err != nil {
		return ExplicitObservationProjection{}, err
	}
	if err := exactShape(availabilityRaw, reflect.TypeFor[AvailabilityDocument]()); err != nil {
		return ExplicitObservationProjection{}, err
	}
	var doc AvailabilityDocument
	if err := json.Unmarshal(availabilityRaw, &doc); err != nil {
		return ExplicitObservationProjection{}, invalid("INVALID_AVAILABILITY_DOCUMENT")
	}
	if doc.Schema != AvailabilitySchema {
		return ExplicitObservationProjection{}, invalid("UNSUPPORTED_AVAILABILITY_PROFILE")
	}
	definition, err := ProjectDefinitionInputsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return ExplicitObservationProjection{}, err
	}
	policyHash, err := policyTextDigest(beforePolicy, afterPolicy)
	if err != nil {
		return ExplicitObservationProjection{}, err
	}
	if doc.InputSHA256 != definition.InputSHA256 || doc.PolicySHA256 != policyHash {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_SOURCE_BINDING")
	}
	parsed, err := projectJSON(raw)
	if err != nil {
		return ExplicitObservationProjection{}, err
	}
	if parsed.FactsSHA256 != definition.FactsSHA256 {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_FACTS_BINDING")
	}
	fields, err := observationFields(parsed.Facts, definition.InputSHA256, definition.FactsSHA256, beforePolicy, afterPolicy)
	if err != nil {
		return ExplicitObservationProjection{}, err
	}
	if len(doc.Fields) != len(fields.Inputs) {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_ROSTER_REQUIRED")
	}
	declared := make(map[string]string, len(doc.Fields))
	for _, field := range doc.Fields {
		if len(field.ValueState) > MaxTextBytes {
			return ExplicitObservationProjection{}, invalid("TEXT_LIMIT")
		}
		key := fieldKey(field.SourceDocument, field.FieldPath)
		if _, duplicate := declared[key]; duplicate {
			return ExplicitObservationProjection{}, invalid("DUPLICATE_AVAILABILITY_FIELD")
		}
		declared[key] = field.ValueState
	}
	availabilityHash := digest(availabilityRaw)
	for i := range fields.Inputs {
		field := &fields.Inputs[i]
		state, ok := declared[fieldKey(field.SourceDocument, field.FieldPath)]
		if !ok {
			return ExplicitObservationProjection{}, invalid("AVAILABILITY_ROSTER_REQUIRED")
		}
		field.ValueState = state
		if field.ComparisonScope != "NONE" {
			expected, exists := declared[fieldKey(field.ExpectedSourceDocument, field.ExpectedFieldPath)]
			if !exists {
				return ExplicitObservationProjection{}, invalid("AVAILABILITY_COMPARISON_BINDING")
			}
			field.ExpectedValueState = expected
		}
		bound, err := json.Marshal(struct {
			Profile, AvailabilitySHA256 string
			Field                       ObservationFieldInput
		}{ExplicitObservationProfile, availabilityHash, *field})
		if err != nil {
			return ExplicitObservationProjection{}, invalid("AVAILABILITY_ENCODING")
		}
		field.ContextID = "gooo/explicit-observation-field/v1/" + digest(bound)
	}
	fields.Schema, fields.Profile = "gooo/explicit-observation-input-projection/v1", ExplicitObservationProfile
	origin := definition.Inputs[definitionEntry+".input0"].(kernel.OriginInput)
	collection := definition.Inputs[definitionEntry+".input1"].(kernel.CollectionInput)
	binding := definition.Inputs[definitionEntry+".input2"].(DefinitionInput)
	a := ExplicitAvailabilityInput{AliasContextID: origin.ContextID, CollectionContextID: collection.ContextID, DefinitionContextID: binding.ContextID, FieldStates: []string{}, FieldValues: []string{}, CaptureBeginIndexes: []int64{}, CaptureEndIndexes: []int64{}, PageContextIndexes: []int64{}, BeforeRowPageIndexes: []int64{}, AfterRowPageIndexes: []int64{}}
	for _, field := range fields.Inputs {
		a.FieldStates = append(a.FieldStates, field.ValueState)
		a.FieldValues = append(a.FieldValues, field.Value)
	}
	var offset int64
	type pageAddress struct{ side, capture, page int }
	pageIndexes := map[pageAddress]int64{}
	for side, observation := range []Observation{parsed.Facts.Before, parsed.Facts.After} {
		if side == 0 {
			a.BeforeCatalogIndex = offset
		} else {
			a.AfterCatalogIndex = offset
		}
		offset++
		for ci, capture := range observation.Captures {
			a.CaptureBeginIndexes = append(a.CaptureBeginIndexes, offset)
			offset += 5
			a.CaptureEndIndexes = append(a.CaptureEndIndexes, offset)
			offset += 5
			for pi := range capture.Pages {
				pageIndexes[pageAddress{side, ci, pi}] = int64(len(a.PageContextIndexes))
				a.PageContextIndexes = append(a.PageContextIndexes, offset)
				offset += 5
			}
		}
	}
	// Only source location is projected here. The original origin tuple/hash and
	// field values stay intact; Gooo decides whether a backend was observed.
	for side, rows := range [][]DefinitionRow{definition.BeforeRows, definition.AfterRows} {
		for _, row := range rows {
			pageIndex, exists := pageIndexes[pageAddress{side, row.Source.CaptureIndex, row.Source.PageIndex}]
			if !exists {
				return ExplicitObservationProjection{}, invalid("AVAILABILITY_ROW_PAGE_BINDING")
			}
			if side == 0 {
				a.BeforeRowPageIndexes = append(a.BeforeRowPageIndexes, pageIndex)
			} else {
				a.AfterRowPageIndexes = append(a.AfterRowPageIndexes, pageIndex)
			}
		}
	}
	a.BeforePolicyIndex, a.AfterPolicyIndex = offset, offset+1
	if offset+2 != int64(len(fields.Inputs)) {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_COLUMN_BINDING")
	}
	bound, err := json.Marshal(struct {
		Profile, InputSHA256, PolicySHA256, AvailabilitySHA256 string
		Columns                                                ExplicitAvailabilityInput
	}{AvailabilitySchema, definition.InputSHA256, policyHash, availabilityHash, a})
	if err != nil {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_ENCODING")
	}
	a.ContextID = "gooo/explicit-observation-availability/v1/" + digest(bound)
	result = ExplicitObservationProjection{Schema: "gooo/explicit-definition-input-projection/v1", InputSHA256: definition.InputSHA256, PolicySHA256: policyHash, AvailabilitySHA256: availabilityHash, SourceProjection: definition, FieldProjection: fields, Inputs: map[string]any{availabilityEntry + ".input0": availabilitykernel.OriginInput(origin), availabilityEntry + ".input1": availabilitykernel.CollectionInput(collection), availabilityEntry + ".input2": availabilitykernel.DefinitionInput(binding), availabilityEntry + ".input3": a}}
	encoded, err := json.Marshal(result)
	if err != nil {
		return ExplicitObservationProjection{}, invalid("AVAILABILITY_ENCODING")
	}
	if len(encoded) > MaxReportBytes {
		return ExplicitObservationProjection{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return result, nil
}

// CompareExplicitObservationJSON evaluates the opt-in availability-aware Gooo
// activities. The returned decision is separate from every legacy v1 report.
func CompareExplicitObservationJSON(raw, availabilityRaw []byte, beforePolicy, afterPolicy string) (result ExplicitObservationReport, returned error) {
	defer func() {
		if recover() != nil {
			result = ExplicitObservationReport{}
			returned = &Problem{State: "UNKNOWN", Stage: "evaluate_availability", Code: "AVAILABILITY_KERNEL_FAILURE"}
		}
	}()
	p, err := ProjectExplicitObservationInputsJSON(raw, availabilityRaw, beforePolicy, afterPolicy)
	if err != nil {
		return ExplicitObservationReport{}, err
	}
	a := p.Inputs[availabilityEntry+".input3"].(ExplicitAvailabilityInput)
	decision := availabilitykernel.Evaluate(p.Inputs[availabilityEntry+".input0"].(availabilitykernel.OriginInput), p.Inputs[availabilityEntry+".input1"].(availabilitykernel.CollectionInput), p.Inputs[availabilityEntry+".input2"].(availabilitykernel.DefinitionInput), a)
	if decision.ContextID != a.ContextID || decision.OriginContextID != a.AliasContextID || decision.CollectionContextID != a.CollectionContextID || decision.BehaviorEquivalenceProven || decision.ExecutionAuthorized || decision.LiveEffectPerformed || decision.RemoteAtomicity != "UNKNOWN" {
		return ExplicitObservationReport{}, &Problem{State: "UNKNOWN", Stage: "bind_availability_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
	}
	observations := make([]ObservationDiagnostic, 0, len(p.FieldProjection.Inputs))
	for _, input := range p.FieldProjection.Inputs {
		actual := availabilityobservationkernel.Evaluate(availabilityobservationkernel.FieldInput(input))
		if actual.ContextID != input.ContextID || actual.SourceDocument != input.SourceDocument || actual.ExpectedSourceDocument != input.ExpectedSourceDocument || actual.FieldPath != input.FieldPath || actual.FieldKind != input.FieldKind || actual.RecordingBasis != input.RecordingBasis || actual.Value != input.Value || actual.ComparisonScope != input.ComparisonScope || actual.ExpectedValue != input.ExpectedValue || actual.ExpectedFieldPath != input.ExpectedFieldPath || actual.ProducerAuthenticated || actual.ExecutionAuthorized || actual.LiveEffectPerformed || actual.BehaviorEquivalenceProven {
			return ExplicitObservationReport{}, &Problem{State: "UNKNOWN", Stage: "bind_availability_observation", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
		}
		observations = append(observations, ObservationDiagnostic(actual))
	}
	result = ExplicitObservationReport{Schema: "gooo/explicit-definition-observation-report/v1", Projection: p, Decision: decision, Observations: observations, Bindings: bytes.Clone(availabilityGeneration), ObservationBindings: bytes.Clone(availabilityObservationGeneration)}
	encoded, err := json.Marshal(result)
	if err != nil {
		return ExplicitObservationReport{}, invalid("AVAILABILITY_ENCODING")
	}
	if len(encoded) > MaxReportBytes {
		return ExplicitObservationReport{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return result, nil
}
