// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/snapshotkernel"
)

const snapshotEntry = "gooo/tool-origin/definition-snapshot:CompareDefinitionSnapshot"

type DefinitionSnapshotDecision = snapshotkernel.Decision

//go:embed snapshot-generation.json
var snapshotGeneration []byte

type DefinitionSnapshotReport struct {
	Schema     string                       `json:"schema"`
	Projection DefinitionSnapshotProjection `json:"projection"`
	Inputs     map[string]any               `json:"inputs"`
	Decision   DefinitionSnapshotDecision   `json:"decision"`
	Bindings   json.RawMessage              `json:"bindings"`
}

// snapshotRowBound checks output references, not comparison policy. Gooo owns
// selection, page coverage, ambiguity and the observed relation.
func snapshotRowBound(side SnapshotProjectionSide, index int64, name, definition, profile, annotation string) bool {
	if index == -1 {
		return definition == "" && profile == "" && annotation == ""
	}
	if index < 0 || index >= int64(len(side.Rows)) {
		return false
	}
	row := side.Rows[index]
	return row.OriginalName == name && row.DefinitionID == definition &&
		row.AnnotationID == annotation && side.Fields.DefinitionProfiles[index] == profile
}

// CompareDefinitionSnapshotJSON compares one original tool name in saved
// JSON-RPC pages using Gooo's generated activity. It collects no live data and
// supplies no missing host, alias, authorization or capture-time observation.
func CompareDefinitionSnapshotJSON(raw []byte) (report DefinitionSnapshotReport, returned error) {
	defer func() {
		if recover() != nil {
			report = DefinitionSnapshotReport{}
			returned = &Problem{State: "UNKNOWN", Stage: "evaluate_definition_snapshot", Code: "KERNEL_FAILURE"}
		}
	}()
	projected, err := ProjectDefinitionSnapshotInputsJSON(raw)
	if err != nil {
		return DefinitionSnapshotReport{}, err
	}
	before := snapshotkernel.SnapshotFacts(projected.Before.Fields)
	after := snapshotkernel.SnapshotFacts(projected.After.Fields)
	selection := snapshotkernel.Selection{ContextID: projected.ContextID, TargetName: projected.TargetName}
	decision := snapshotkernel.Evaluate(before, after, selection)
	if decision.ContextID != projected.ContextID || decision.TargetName != projected.TargetName ||
		decision.ComparisonScope != "SAVED_ROWS_ONLY" || decision.OriginRelation != "NOT_ASSESSED" ||
		decision.AuthorizationRelation != "NOT_ASSESSED" || decision.BehaviorEquivalenceProven ||
		decision.ExecutionAuthorized || decision.LiveEffectPerformed || decision.RemoteAtomicity != "UNKNOWN" ||
		!snapshotRowBound(projected.Before, decision.BeforeIndex, decision.TargetName, decision.BeforeDefinitionID, decision.BeforeDefinitionProfile, decision.BeforeAnnotationID) ||
		!snapshotRowBound(projected.After, decision.AfterIndex, decision.TargetName, decision.AfterDefinitionID, decision.AfterDefinitionProfile, decision.AfterAnnotationID) {
		return DefinitionSnapshotReport{}, &Problem{State: "UNKNOWN", Stage: "bind_definition_snapshot_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
	}
	report = DefinitionSnapshotReport{Schema: "gooo/definition-snapshot-report/v1",
		Projection: projected, Decision: decision, Bindings: bytes.Clone(snapshotGeneration),
		Inputs: map[string]any{snapshotEntry + ".input0": before, snapshotEntry + ".input1": after, snapshotEntry + ".input2": selection}}
	encoded, err := json.Marshal(report)
	if err != nil {
		return DefinitionSnapshotReport{}, err
	}
	if len(encoded) > MaxReportBytes {
		return DefinitionSnapshotReport{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return report, nil
}
