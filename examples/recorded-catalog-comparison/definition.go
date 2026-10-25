// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/definitionkernel"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/kernel"
)

// DefinitionProfile names this exact JSON projection, not semantic equivalence.
// Go1.27.3 encoding/json sorts object keys and retains json.Number spellings;
// array order, unknown descriptor fields and absent/null differences are kept.
const DefinitionProfile = "gooo/mcp-tool-definition-json/v1"
const definitionEntry = "gooo/tool-origin/definition-binding:AssessDefinitionBinding"

// DefinitionInput and DefinitionDecision follow the compiler-owned typed plan.
type DefinitionInput = definitionkernel.DefinitionInput
type DefinitionDecision = definitionkernel.Decision

//go:embed definition-generation.json
var definitionGeneration []byte

type DefinitionReport struct {
	Projection DefinitionProjection `json:"projection"`
	Decision   DefinitionDecision   `json:"decision"`
	Bindings   json.RawMessage      `json:"bindings"`
}

type DefinitionRow struct {
	Source       RowSource `json:"source"`
	DefinitionID string    `json:"definition_id"`
	CapabilityID string    `json:"capability_id"`
}

type DefinitionProjection struct {
	Schema      string          `json:"schema"`
	Profile     string          `json:"profile"`
	InputSHA256 string          `json:"input_sha256"`
	FactsSHA256 string          `json:"facts_sha256"`
	Inputs      map[string]any  `json:"inputs"`
	BeforeRows  []DefinitionRow `json:"before_rows"`
	AfterRows   []DefinitionRow `json:"after_rows"`
	// CollectionBindings covers the unchanged imported origin/context kernel.
	// This transport projection is not a generation or execution receipt for the
	// new Gooo definition activity and does not include a decision/authorization.
	CollectionBindings json.RawMessage `json:"collection_bindings"`
}

// definitionRows sees only frames already accepted by the shared strict
// transport. Metadata remains opaque data; it cannot cause a tool invocation.
func definitionRows(o Observation, sources []RowSource) ([]DefinitionRow, error) {
	rows := make([]DefinitionRow, 0, len(sources))
	type pageID struct{ capture, page int }
	decoded := map[pageID][]map[string]any{}
	for _, source := range sources {
		key := pageID{source.CaptureIndex, source.PageIndex}
		tools, ok := decoded[key]
		if !ok {
			p := o.Captures[key.capture].Pages[key.page]
			if source.ResponseSHA256 != digest([]byte(p.ResponseJSON)) {
				return nil, invalid("DEFINITION_RESPONSE_BINDING")
			}
			var err error
			tools, err = definitionTools(p.ResponseJSON)
			if err != nil {
				return nil, err
			}
			decoded[key] = tools
		}
		if source.ToolIndex < 0 || source.ToolIndex >= len(tools) {
			return nil, invalid("DEFINITION_ROW_BINDING")
		}
		definitionID, capabilityID, err := definitionIDs(tools[source.ToolIndex])
		if err != nil {
			return nil, err
		}
		rows = append(rows, DefinitionRow{Source: source, DefinitionID: definitionID, CapabilityID: capabilityID})
	}
	return rows, nil
}

// definitionTools reads the exact protocol keys already checked by the strict
// transport. Struct decoding would also match Result/Tools case-insensitively
// and could bind a valid original name to an unrelated shadow definition.
func definitionTools(responseJSON string) ([]map[string]any, error) {
	envelope, err := jsonObject([]byte(responseJSON))
	if err != nil {
		return nil, invalid("DEFINITION_FRAME_DECODE")
	}
	result, err := jsonObject(envelope["result"])
	if err != nil {
		return nil, invalid("DEFINITION_FRAME_DECODE")
	}
	var tools []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(result["tools"]))
	decoder.UseNumber()
	if decoder.Decode(&tools) != nil {
		return nil, invalid("DEFINITION_FRAME_DECODE")
	}
	return tools, nil
}

// definitionIDs is the shared exact v1 descriptor projection. Callers first
// validate the original frame and decode numbers with json.Decoder.UseNumber.
func definitionIDs(tool map[string]any) (string, string, error) {
	descriptor := make(map[string]any, len(tool))
	for k, v := range tool {
		if k != "annotations" {
			descriptor[k] = v
		}
	}
	annotations, present := tool["annotations"]
	// Presence is explicit; absent annotations and explicit null do not collapse.
	claims := map[string]any{"present": present}
	if present {
		claims["value"] = annotations
	}
	definition, err := json.Marshal(descriptor)
	if err != nil {
		return "", "", invalid("DEFINITION_ENCODING")
	}
	capability, err := json.Marshal(claims)
	if err != nil {
		return "", "", invalid("DEFINITION_ENCODING")
	}
	return digest(append([]byte(DefinitionProfile+"\x00definition\x00"), definition...)), digest(append([]byte(DefinitionProfile+"\x00annotations\x00"), capability...)), nil
}

// ProjectDefinitionInputsJSON derives the three source-bound inputs for the
// Gooo definition activity from saved tools/list frames. Policy revisions are
// explicit host declarations (empty means unobserved). This function only
// projects data: Gooo owns origin/context/definition/capability/policy decisions.
func ProjectDefinitionInputsJSON(raw []byte, beforePolicy, afterPolicy string) (result DefinitionProjection, returned error) {
	defer func() {
		if recover() != nil {
			result = DefinitionProjection{}
			returned = &Problem{State: "UNKNOWN", Stage: "project_definition", Code: "DEFINITION_PROJECTION_FAILURE"}
		}
	}()
	for _, revision := range []string{beforePolicy, afterPolicy} {
		if len(revision) > MaxTextBytes || !utf8.ValidString(revision) || strings.ContainsRune(revision, 0) {
			return DefinitionProjection{}, invalid("POLICY_REVISION_TEXT")
		}
	}
	p, err := projectJSON(raw)
	if err != nil {
		return DefinitionProjection{}, err
	}
	before, err := definitionRows(p.Facts.Before, p.Projection.BeforeRows)
	if err != nil {
		return DefinitionProjection{}, err
	}
	after, err := definitionRows(p.Facts.After, p.Projection.AfterRows)
	if err != nil {
		return DefinitionProjection{}, err
	}
	binding := DefinitionInput{AliasContextID: p.Origin.ContextID, CollectionContextID: p.Collection.ContextID, BeforeDefinitionIDs: []string{}, AfterDefinitionIDs: []string{}, BeforeDefinitionProfiles: []string{}, AfterDefinitionProfiles: []string{}, BeforeCapabilityIDs: []string{}, AfterCapabilityIDs: []string{}, BeforePolicyRevision: beforePolicy, AfterPolicyRevision: afterPolicy}
	for side, rows := range [][]DefinitionRow{before, after} {
		for _, row := range rows {
			if side == 0 {
				binding.BeforeDefinitionIDs = append(binding.BeforeDefinitionIDs, row.DefinitionID)
				binding.BeforeDefinitionProfiles = append(binding.BeforeDefinitionProfiles, DefinitionProfile)
				binding.BeforeCapabilityIDs = append(binding.BeforeCapabilityIDs, row.CapabilityID)
			} else {
				binding.AfterDefinitionIDs = append(binding.AfterDefinitionIDs, row.DefinitionID)
				binding.AfterDefinitionProfiles = append(binding.AfterDefinitionProfiles, DefinitionProfile)
				binding.AfterCapabilityIDs = append(binding.AfterCapabilityIDs, row.CapabilityID)
			}
		}
	}
	bound, err := json.Marshal(struct {
		Profile     string
		InputSHA256 string
		Columns     DefinitionInput
	}{DefinitionProfile, digest(raw), binding})
	if err != nil {
		return DefinitionProjection{}, invalid("DEFINITION_ENCODING")
	}
	binding.ContextID = "gooo/definition-binding/v1/" + digest(bound)
	result = DefinitionProjection{Schema: "gooo/tool-definition-input-projection/v1", Profile: DefinitionProfile, InputSHA256: digest(raw), FactsSHA256: p.FactsSHA256, Inputs: map[string]any{definitionEntry + ".input0": p.Origin, definitionEntry + ".input1": p.Collection, definitionEntry + ".input2": binding}, BeforeRows: before, AfterRows: after, CollectionBindings: bytes.Clone(generation)}
	encoded, err := json.Marshal(result)
	if err != nil {
		return DefinitionProjection{}, err
	}
	if len(encoded) > MaxReportBytes {
		return DefinitionProjection{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return result, nil
}

// CompareDefinitionJSON evaluates the original Gooo policy compiled into this
// module. The transport performs no model request, shell command or live call.
func CompareDefinitionJSON(raw []byte, beforePolicy, afterPolicy string) (report DefinitionReport, returned error) {
	defer func() {
		if recover() != nil {
			report = DefinitionReport{}
			returned = &Problem{State: "UNKNOWN", Stage: "evaluate_definition", Code: "KERNEL_FAILURE"}
		}
	}()
	projected, err := ProjectDefinitionInputsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return DefinitionReport{}, err
	}
	origin := projected.Inputs[definitionEntry+".input0"].(kernel.OriginInput)
	collection := projected.Inputs[definitionEntry+".input1"].(kernel.CollectionInput)
	binding := projected.Inputs[definitionEntry+".input2"].(DefinitionInput)
	decision := definitionkernel.Evaluate(definitionkernel.OriginInput(origin), definitionkernel.CollectionInput(collection), binding)
	if decision.ContextID != binding.ContextID || decision.OriginContextID != binding.AliasContextID || decision.CollectionContextID != binding.CollectionContextID || decision.BehaviorEquivalenceProven || decision.ExecutionAuthorized || decision.LiveEffectPerformed || decision.RemoteAtomicity != "UNKNOWN" {
		return DefinitionReport{}, &Problem{State: "UNKNOWN", Stage: "bind_definition_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
	}
	report = DefinitionReport{Projection: projected, Decision: decision, Bindings: bytes.Clone(definitionGeneration)}
	encoded, err := json.Marshal(report)
	if err != nil {
		return DefinitionReport{}, err
	}
	if len(encoded) > MaxReportBytes {
		return DefinitionReport{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return report, nil
}
