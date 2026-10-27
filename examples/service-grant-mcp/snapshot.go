// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	catalog "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const snapshotToolName = "gooo_compare_saved_tool_definitions"
const snapshotGenerationPin = "sha256:9b54075941cbc495f4f4128999269c7f6d6d32bf2217dd73df46105e266c2e82"
const snapshotGenerationSchema = "gooo/definition-snapshot-generation/v1"
const snapshotReportSchema = "gooo/definition-snapshot-report/v1"

func snapshotInputSchema() map[string]any {
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	page := closedObject(map[string]any{
		"request_json":  text("Exact saved JSON-RPC tools/list request text; preserve its ID and cursor. The decoder enforces at most 64 KiB of UTF-8 bytes. Do not parse and rewrite the string."),
		"response_json": text("Exact saved successful JSON-RPC tools/list response text; preserve descriptor fields, number spelling, row order and nextCursor. The decoder enforces at most 64 KiB of UTF-8 bytes."),
	}, []string{"request_json", "response_json"})
	observation := closedObject(map[string]any{
		"pages": map[string]any{"type": "array", "items": page, "maxItems": 16,
			"description": "Saved pages in observed order. Use an empty array if no pages were saved; do not create an empty successful response or remove a continuation cursor to claim completion."},
	}, []string{"pages"})
	return closedObject(map[string]any{
		"target_name": text("Original tool name in the saved server descriptors, not a host alias. The decoder enforces at most 1024 UTF-8 bytes. Empty names remain domain input for Gooo."),
		"before":      observation, "after": observation,
	}, []string{"target_name", "before", "after"})
}

// This transport schema is derived from the Gooo package interface at build
// preparation time. Its source/inspection profile is retained beside it.
//
//go:embed schemas/snapshot-output.schema.json
var snapshotOutputSchemaJSON []byte

func snapshotOutputSchema() map[string]any {
	var result map[string]any
	if err := json.Unmarshal(snapshotOutputSchemaJSON, &result); err != nil {
		panic(err)
	}
	return result
}

func snapshotError(err error) (*mcp.CallToolResult, error) {
	var problem *catalog.Problem
	if errors.As(err, &problem) {
		raw, e := json.Marshal(problem)
		if e != nil {
			return toolError(e)
		}
		return toolError(errors.New(string(raw)))
	}
	return toolError(err)
}

func snapshotResult(report catalog.DefinitionSnapshotReport) (*mcp.CallToolResult, error) {
	d := report.Decision
	if fmt.Sprintf("sha256:%x", sha256.Sum256(report.Bindings)) != snapshotGenerationPin ||
		report.Schema != snapshotReportSchema || d.ComparisonScope != "SAVED_ROWS_ONLY" ||
		d.OriginRelation != "NOT_ASSESSED" || d.AuthorizationRelation != "NOT_ASSESSED" ||
		d.ExecutionAuthorized || d.BehaviorEquivalenceProven || d.LiveEffectPerformed || d.RemoteAtomicity != "UNKNOWN" {
		return snapshotError(&catalog.Problem{State: "UNKNOWN", Stage: "bind_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"})
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return snapshotError(err)
	}
	if len(raw) > catalog.MaxReportBytes {
		return snapshotError(&catalog.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: report}, nil
}

func compareSnapshot(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return snapshotError(err)
	}
	if request == nil || request.Params == nil {
		return snapshotError(&catalog.Problem{State: "INVALID_TRANSPORT", Stage: "decode_json", Code: "INVALID_JSON_FACTS"})
	}
	// No re-marshalling of arguments, alias inference, collection or policy here.
	report, err := catalog.CompareDefinitionSnapshotJSON(request.Params.Arguments)
	if err != nil {
		return snapshotError(err)
	}
	if err := ctx.Err(); err != nil {
		return snapshotError(err)
	}
	return snapshotResult(report)
}

type snapshotWireWriter struct {
	out   io.Writer
	alias bool
}

func (w *snapshotWireWriter) Close() error { return nil }
func (w *snapshotWireWriter) Write(data []byte) (int, error) {
	return boundedToolWireWrite(w.out, data, func(data []byte) ([]byte, error) {
		return replaceToolWireResults(data, "saved-definition", func(raw json.RawMessage) (*mcp.CallToolResult, error) {
			var proof struct {
				Schema   string `json:"schema"`
				Bindings struct {
					Schema string `json:"schema"`
				} `json:"bindings"`
			}
			if json.Unmarshal(raw, &proof) == nil && proof.Schema == snapshotReportSchema && proof.Bindings.Schema == snapshotGenerationSchema {
				return snapshotError(&catalog.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "MCP_WIRE_LIMIT_EXCEEDED"})
			}
			if w.alias {
				return aliasWireResultFailure(raw)
			}
			return nil, nil
		})
	})
}
