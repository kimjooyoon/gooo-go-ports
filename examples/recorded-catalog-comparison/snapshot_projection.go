// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"encoding/json"
	"reflect"
)

// SnapshotPage holds already extracted JSON-RPC payloads. An HTTP/SSE carrier
// and its extraction provenance belong with the input; this adapter does not
// guess a transport, collect a catalog or reconstruct a host registry.
type SnapshotPage struct {
	RequestJSON  string `json:"request_json"`
	ResponseJSON string `json:"response_json"`
}

type SnapshotObservation struct {
	Pages []SnapshotPage `json:"pages"`
}

type DefinitionSnapshotInput struct {
	TargetName string              `json:"target_name"`
	Before     SnapshotObservation `json:"before"`
	After      SnapshotObservation `json:"after"`
}

// SnapshotFields are transport-derived columns, not a Go implementation of
// coverage, identity or definition comparison rules.
type SnapshotFields struct {
	PageRequestCursorKinds  []string `json:"page_request_cursor_kinds"`
	PageRequestCursorValues []string `json:"page_request_cursor_values"`
	PageNextCursorKinds     []string `json:"page_next_cursor_kinds"`
	PageNextCursorValues    []string `json:"page_next_cursor_values"`
	OriginalNames           []string `json:"original_names"`
	DefinitionIDs           []string `json:"definition_ids"`
	DefinitionProfiles      []string `json:"definition_profiles"`
	AnnotationIDs           []string `json:"annotation_ids"`
	RowPageIndexes          []int64  `json:"row_page_indexes"`
}

type SnapshotPageSource struct {
	PageIndex      int             `json:"page_index"`
	RequestSHA256  string          `json:"request_sha256"`
	ResponseSHA256 string          `json:"response_sha256"`
	RequestID      json.RawMessage `json:"request_id"`
	ResponseID     json.RawMessage `json:"response_id"`
	RequestCursor  Cursor          `json:"request_cursor"`
	NextCursor     Cursor          `json:"next_cursor"`
}

type SnapshotRowSource struct {
	PageIndex      int    `json:"page_index"`
	ToolIndex      int    `json:"tool_index"`
	ResponseSHA256 string `json:"response_sha256"`
}

type SnapshotDefinitionRow struct {
	Source       SnapshotRowSource `json:"source"`
	OriginalName string            `json:"original_name"`
	DefinitionID string            `json:"definition_id"`
	AnnotationID string            `json:"annotation_id"`
}

type SnapshotProjectionSide struct {
	Fields SnapshotFields          `json:"fields"`
	Pages  []SnapshotPageSource    `json:"pages"`
	Rows   []SnapshotDefinitionRow `json:"rows"`
}

// DefinitionSnapshotProjection retains exact input bytes and row/page source
// bindings. It makes no decision or assertion about host origin or permission.
type DefinitionSnapshotProjection struct {
	Schema      string                 `json:"schema"`
	Profile     string                 `json:"profile"`
	ContextID   string                 `json:"context_id"`
	InputSHA256 string                 `json:"input_sha256"`
	InputJSON   string                 `json:"input_json"`
	TargetName  string                 `json:"target_name"`
	Before      SnapshotProjectionSide `json:"before"`
	After       SnapshotProjectionSide `json:"after"`
}

func emptySnapshotSide() SnapshotProjectionSide {
	return SnapshotProjectionSide{
		Fields: SnapshotFields{
			PageRequestCursorKinds: []string{}, PageRequestCursorValues: []string{},
			PageNextCursorKinds: []string{}, PageNextCursorValues: []string{},
			OriginalNames: []string{}, DefinitionIDs: []string{},
			DefinitionProfiles: []string{}, AnnotationIDs: []string{}, RowPageIndexes: []int64{},
		},
		Pages: []SnapshotPageSource{}, Rows: []SnapshotDefinitionRow{},
	}
}

func projectSnapshotSide(observation SnapshotObservation, side string) (SnapshotProjectionSide, error) {
	result := emptySnapshotSide()
	if len(observation.Pages) > MaxPages {
		return SnapshotProjectionSide{}, invalid("PAGE_LIMIT")
	}
	for pi, page := range observation.Pages {
		decoded, err := decodeSavedPage(page.RequestJSON, page.ResponseJSON, 0, false)
		if err != nil {
			if problem, ok := err.(*Problem); ok {
				problem.Side, problem.PageIndex = side, &pi
				problem.RequestSHA256 = digest([]byte(page.RequestJSON))
				problem.ResponseSHA256 = digest([]byte(page.ResponseJSON))
			}
			return SnapshotProjectionSide{}, err
		}
		if len(result.Rows)+len(decoded.Names) > MaxEntries {
			return SnapshotProjectionSide{}, invalid("ROW_LIMIT")
		}
		tools, err := definitionTools(page.ResponseJSON)
		if err != nil {
			return SnapshotProjectionSide{}, err
		}
		if len(tools) != len(decoded.Names) {
			return SnapshotProjectionSide{}, invalid("DEFINITION_ROW_BINDING")
		}
		source := decoded.Source
		result.Pages = append(result.Pages, SnapshotPageSource{PageIndex: pi,
			RequestSHA256: source.RequestSHA256, ResponseSHA256: source.ResponseSHA256,
			RequestID: source.RequestID, ResponseID: source.ResponseID,
			RequestCursor: source.RequestCursor, NextCursor: source.NextCursor})
		result.Fields.PageRequestCursorKinds = append(result.Fields.PageRequestCursorKinds, source.RequestCursor.Tag)
		result.Fields.PageRequestCursorValues = append(result.Fields.PageRequestCursorValues, source.RequestCursor.Value)
		result.Fields.PageNextCursorKinds = append(result.Fields.PageNextCursorKinds, source.NextCursor.Tag)
		result.Fields.PageNextCursorValues = append(result.Fields.PageNextCursorValues, source.NextCursor.Value)
		for ti, tool := range tools {
			definitionID, annotationID, err := definitionIDs(tool)
			if err != nil {
				return SnapshotProjectionSide{}, err
			}
			result.Rows = append(result.Rows, SnapshotDefinitionRow{
				Source:       SnapshotRowSource{PageIndex: pi, ToolIndex: ti, ResponseSHA256: source.ResponseSHA256},
				OriginalName: decoded.Names[ti], DefinitionID: definitionID, AnnotationID: annotationID})
			result.Fields.OriginalNames = append(result.Fields.OriginalNames, decoded.Names[ti])
			result.Fields.DefinitionIDs = append(result.Fields.DefinitionIDs, definitionID)
			result.Fields.DefinitionProfiles = append(result.Fields.DefinitionProfiles, DefinitionProfile)
			result.Fields.AnnotationIDs = append(result.Fields.AnnotationIDs, annotationID)
			result.Fields.RowPageIndexes = append(result.Fields.RowPageIndexes, int64(pi))
		}
	}
	return result, nil
}

// ProjectDefinitionSnapshotInputsJSON accepts a selected original tool name
// and saved pages only. It deliberately does not construct host IDs, aliases,
// capture timestamps, configuration revisions or availability declarations.
func ProjectDefinitionSnapshotInputsJSON(raw []byte) (result DefinitionSnapshotProjection, returned error) {
	defer func() {
		if recover() != nil {
			result = DefinitionSnapshotProjection{}
			returned = &Problem{State: "UNKNOWN", Stage: "project_definition_snapshot", Code: "SNAPSHOT_PROJECTION_FAILURE"}
		}
	}()
	if err := strictJSON(raw, MaxInputBytes); err != nil {
		return result, err
	}
	if err := exactShape(raw, reflect.TypeFor[DefinitionSnapshotInput]()); err != nil {
		return result, err
	}
	var input DefinitionSnapshotInput
	if json.Unmarshal(raw, &input) != nil {
		return result, invalid("INVALID_FACTS")
	}
	if len(input.TargetName) > MaxTextBytes {
		return result, invalid("TEXT_LIMIT")
	}
	before, err := projectSnapshotSide(input.Before, "BEFORE")
	if err != nil {
		return result, err
	}
	after, err := projectSnapshotSide(input.After, "AFTER")
	if err != nil {
		return result, err
	}
	result = DefinitionSnapshotProjection{Schema: "gooo/definition-snapshot-input-projection/v1",
		Profile: DefinitionProfile, ContextID: "gooo/definition-snapshot/v1/" + digest(raw),
		InputSHA256: digest(raw), InputJSON: string(raw), TargetName: input.TargetName,
		Before: before, After: after}
	encoded, err := json.Marshal(result)
	if err != nil {
		return DefinitionSnapshotProjection{}, err
	}
	if len(encoded) > MaxReportBytes {
		return DefinitionSnapshotProjection{}, invalid("OUTPUT_LIMIT_EXCEEDED")
	}
	return result, nil
}
