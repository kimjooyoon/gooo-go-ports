// Command service-grant-mcp transports facts to the immutable Gooo evaluator.
// No policy decision is implemented here and no grant or service is changed.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
	aliasorigin "github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change"
	many "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "assess_service_grant"

const (
	manyToolName    = "assess_service_grant_change"
	defaultFrameMax = 64 << 10
	// The friendly arguments object has its own1MiB decoder limit. Reserve
	// another64KiB for JSON-RPC framing/metadata only in the explicit profile.
	manyFrameMax       = many.MaxBatchBytes + defaultFrameMax
	manyCompilerPin    = "205896a413e5dea92366309c35ae85e542d055f6"
	manyGenerationPin  = "sha256:86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164"
	aliasToolName      = "gooo_compare_alias_origin"
	aliasGenerationPin = "sha256:b0c87b7865697c4b6433ef54e23defd7267ed8ea4da562cc8e1af24e283e7b06"
	aliasFrameMax      = aliasorigin.MaxInputBytes + defaultFrameMax
	// A report is bounded separately at 4 MiB. Its JSON is carried both as
	// structuredContent and as escaped text. This distinct frame budget also
	// includes JSON-RPC IDs, SDK metadata, batches and the final newline.
	// Oversized frames become explicit tool errors before any bytes are written.
	aliasWireMax = (16 << 20) + defaultFrameMax
)

func closedObject(properties map[string]any, required []string) map[string]any {
	return map[string]any{"type": "object", "properties": properties,
		"required": required, "additionalProperties": false}
}

func describedText(description, example string) map[string]any {
	return map[string]any{"type": "string", "description": description, "examples": []string{example}}
}

func grantPatternFields() map[string]any {
	return map[string]any{
		"before": describedText("Previous service-grant pattern, preserved exactly. Examples: * (all services), mcp://* (all MCP services), or mcp://weather (one service). Supported patterns use ASCII text of at most 128 bytes; paths, queries and fragments are outside this domain.", "*"),
		"after":  describedText("Proposed replacement service-grant pattern, using the same syntax as before. It must be a supported subset and preserve the supplied required request(s) for ELIGIBLE. Do not normalize or repair an unsupported pattern to force eligibility.", "mcp://*"),
	}
}

func requestTypeSchema() map[string]any {
	return describedText("Exact required service type, for example mcp or http; not a full URI. The supported domain is ASCII, at most 128 bytes, starting with a letter when nonempty, followed by letters, digits, +, - or .; Gooo decides domain validity.", "mcp")
}

func requestNameSchema() map[string]any {
	return describedText("Exact required service name, for example weather, not an individual tool name or a path such as weather/forecast. Supported nonempty names contain ASCII letters, digits, - and _ in nonempty dot-separated labels, at most 128 bytes. Preserve the supplied spelling; Gooo decides domain validity.", "weather")
}

func factsSchema() map[string]any {
	properties := grantPatternFields()
	properties["required_type"] = requestTypeSchema()
	properties["required_name"] = requestNameSchema()
	return closedObject(properties, []string{"before", "after", "required_type", "required_name"})
}

func proofSchema() map[string]any {
	decision := map[string]any{}
	for _, name := range []string{"status", "reason", "invalid_field", "subset_rule_id"} {
		decision[name] = map[string]any{"type": "string"}
	}
	decision["status"] = map[string]any{"type": "string", "enum": []string{"ELIGIBLE", "REFUSED", "INVALID_INPUT"}}
	for _, name := range []string{"required_observed", "required_before_match", "required_after_match"} {
		decision[name] = map[string]any{"type": "boolean"}
	}
	bindings := closedObject(map[string]any{
		"compiler_source_sha": map[string]any{"type": "string"},
		"generation_sha256":   map[string]any{"type": "string"},
		"generation_origin":   map[string]any{"type": "string"},
		"assets":              map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
	}, []string{"compiler_source_sha", "generation_sha256", "generation_origin", "assets"})
	return closedObject(map[string]any{
		"facts":    factsSchema(),
		"decision": closedObject(decision, []string{"status", "reason", "invalid_field", "required_observed", "required_before_match", "required_after_match", "subset_rule_id"}),
		"bindings": bindings,
	}, []string{"facts", "decision", "bindings"})
}

func toolError(err error) (*mcp.CallToolResult, error) {
	result := new(mcp.CallToolResult)
	result.SetError(err)
	return result, nil
}

func assess(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return toolError(err)
	}
	if request == nil || request.Params == nil {
		return toolError(fmt.Errorf("INVALID_FACTS: four exact JSON Text fields are required"))
	}
	// Keep raw arguments: the strict transport decoder rejects duplicate keys,
	// missing/extra fields, null, wrong types and unpaired UTF-16 surrogates.
	facts, err := servicepolicy.DecodeGrantUpdate(request.Params.Arguments)
	if err != nil {
		return toolError(fmt.Errorf("INVALID_FACTS: %w", err))
	}
	proof, err := servicepolicy.EvaluateGrantUpdate(facts)
	if err != nil {
		return toolError(fmt.Errorf("EVALUATION_FAILED: %w", err))
	}
	raw, err := json.Marshal(proof)
	if err != nil {
		return toolError(fmt.Errorf("ENCODING_FAILED: %w", err))
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: proof,
	}, nil
}

func manyFactsSchema() map[string]any {
	request := closedObject(map[string]any{"type": requestTypeSchema(), "name": requestNameSchema()}, []string{"type", "name"})
	// No maxItems/minItems/domain restrictions: Gooo owns the256/257 and
	// empty-list decisions. DecodeInput owns the separate JSON byte limit.
	properties := grantPatternFields()
	properties["required_requests"] = map[string]any{"type": "array", "items": request,
		"description": "Required service requests to preserve in their original order, including duplicates. Each item has exact type and name fields. Do not sort or deduplicate. Empty lists and lists above the supported 256-request policy limit return Gooo decisions; malformed JSON or an arguments object exceeding 1 MiB is a tool error."}
	return closedObject(properties, []string{"before", "after", "required_requests"})
}

func manyProofSchema() map[string]any {
	text := map[string]any{"type": "string"}
	texts := map[string]any{"type": "array", "items": text}
	facts := closedObject(map[string]any{
		"before": text, "after": text, "required_types": texts, "required_names": texts,
	}, []string{"before", "after", "required_types", "required_names"})
	decision := map[string]any{}
	for _, name := range []string{"status", "reason", "invalid_field", "subset_rule_id"} {
		decision[name] = text
	}
	decision["status"] = map[string]any{"type": "string", "enum": []string{"ELIGIBLE", "REFUSED", "INVALID_INPUT"}}
	for _, name := range []string{"invalid_index", "first_counterexample", "request_count", "checked_count"} {
		decision[name] = map[string]any{"type": "integer"}
	}
	for _, name := range []string{"all_preserved", "supported_subset", "counterexample_before_match", "counterexample_after_match"} {
		decision[name] = map[string]any{"type": "boolean"}
	}
	stringMap := map[string]any{"type": "object", "additionalProperties": text}
	bindings := closedObject(map[string]any{
		"compiler_source_sha": text, "generation_sha256": text, "sources": stringMap, "assets": stringMap,
	}, []string{"compiler_source_sha", "generation_sha256", "sources", "assets"})
	return closedObject(map[string]any{
		"facts":    facts,
		"decision": closedObject(decision, []string{"status", "reason", "invalid_field", "invalid_index", "first_counterexample", "request_count", "checked_count", "all_preserved", "supported_subset", "subset_rule_id", "counterexample_before_match", "counterexample_after_match"}),
		"bindings": bindings,
	}, []string{"facts", "decision", "bindings"})
}

func assessMany(ctx context.Context, request *mcp.CallToolRequest) (response *mcp.CallToolResult, rpcErr error) {
	if err := ctx.Err(); err != nil {
		return toolError(err)
	}
	if request == nil || request.Params == nil {
		return toolError(fmt.Errorf("INVALID_FACTS: before, after and exact ordered required_requests are required"))
	}
	// Preserve raw arguments so the public decoder catches duplicate fields,
	// bad later requests, invalid UTF-8 and unpaired surrogates losslessly.
	input, err := many.DecodeInput(request.Params.Arguments)
	if err != nil {
		return toolError(fmt.Errorf("INVALID_FACTS: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return toolError(err)
	}
	// No sort, deduplication, policy limit, matching or aggregation lives here.
	// A generated evaluation panic is an execution failure, never a decision.
	defer func() {
		if failure := recover(); failure != nil {
			response, rpcErr = toolError(fmt.Errorf("EVALUATION_FAILED: many-required assessment: %v", failure))
		}
	}()
	proof := many.Assess(input)
	if err := ctx.Err(); err != nil {
		return toolError(err)
	}
	if proof.Bindings.CompilerSourceSHA != manyCompilerPin || proof.Bindings.GenerationSHA256 != manyGenerationPin || len(proof.Bindings.Assets) != 30 || len(proof.Bindings.Sources) != 5 {
		return toolError(fmt.Errorf("EVALUATION_FAILED: unexpected many-required source bindings"))
	}
	raw, err := json.Marshal(proof)
	if err != nil {
		return toolError(fmt.Errorf("ENCODING_FAILED: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return toolError(err)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: proof,
	}, nil
}

func aliasError(err error) (*mcp.CallToolResult, error) {
	var problem *aliasorigin.Problem
	if errors.As(err, &problem) {
		raw, marshalErr := json.Marshal(problem)
		if marshalErr != nil {
			return toolError(marshalErr)
		}
		return toolError(errors.New(string(raw)))
	}
	return toolError(err)
}

func aliasResult(report aliasorigin.Report) (*mcp.CallToolResult, error) {
	if fmt.Sprintf("sha256:%x", sha256.Sum256(report.Bindings)) != aliasGenerationPin || report.Decision.ExecutionAuthorized {
		return aliasError(&aliasorigin.Problem{State: "UNKNOWN", Stage: "bind_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"})
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return aliasError(err)
	}
	if len(raw) > aliasorigin.MaxReportBytes {
		return aliasError(&aliasorigin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: report,
	}, nil
}

func compareAlias(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return aliasError(err)
	}
	if request == nil || request.Params == nil {
		return aliasError(&aliasorigin.Problem{State: "INVALID_TRANSPORT", Stage: "decode_json", Code: "INVALID_JSON_FACTS"})
	}
	// Preserve the received bytes: CompareJSON owns lossless transport,
	// snapshotting, hashing and the generated Gooo decision. No alias matching,
	// normalization or catalog repair is implemented by this adapter.
	report, err := aliasorigin.CompareJSON(request.Params.Arguments)
	if err != nil {
		return aliasError(err)
	}
	if err := ctx.Err(); err != nil {
		return aliasError(err)
	}
	return aliasResult(report)
}

// The SDK sends a complete serialized JSON-RPC frame in each Write. Keeping
// this check at its writer boundary includes both result representations, all
// escaping, metadata and batch framing. Existing profiles use StdioTransport
// directly; only the explicit alias profile installs this writer.
type aliasWireWriter struct {
	out io.Writer
}

func (w *aliasWireWriter) Write(data []byte) (int, error) {
	encoded := data
	if len(data) > aliasWireMax {
		var err error
		encoded, err = aliasWireFailure(data)
		if err != nil {
			return 0, err
		}
		if len(encoded) > aliasWireMax {
			return 0, fmt.Errorf("MCP_WIRE_LIMIT_EXCEEDED: diagnostic frame also exceeds limit")
		}
	}
	n, err := w.out.Write(encoded)
	if err == nil && n != len(encoded) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return 0, err
	}
	// The entire input message has been consumed, even when replaced by its
	// bounded error response. Never leak a prefix of the oversized proof.
	return len(data), nil
}

func (w *aliasWireWriter) Close() error { return nil }

func aliasWireFailure(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	var frames []json.RawMessage
	batch := len(trimmed) > 0 && trimmed[0] == '['
	if batch {
		if err := json.Unmarshal(trimmed, &frames); err != nil {
			return nil, err
		}
	} else {
		frames = []json.RawMessage{trimmed}
	}
	changed := false
	for i, raw := range frames {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		var result map[string]json.RawMessage
		if json.Unmarshal(envelope["result"], &result) != nil {
			continue // Preserve existing protocol errors and notifications.
		}
		var proof struct {
			Bindings struct {
				Schema string `json:"schema"`
			} `json:"bindings"`
		}
		if json.Unmarshal(result["structuredContent"], &proof) != nil || proof.Bindings.Schema != "gooo/alias-origin-generation/v1" {
			continue // Never rewrite another tool's proof in a mixed batch.
		}
		replacement, _ := aliasError(&aliasorigin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "MCP_WIRE_LIMIT_EXCEEDED"})
		replacementJSON, err := json.Marshal(replacement)
		if err != nil {
			return nil, err
		}
		var replacementFields map[string]json.RawMessage
		if err := json.Unmarshal(replacementJSON, &replacementFields); err != nil {
			return nil, err
		}
		for _, key := range []string{"_meta", "resultType"} {
			if value, exists := result[key]; exists {
				replacementFields[key] = value
			}
		}
		envelope["result"], err = json.Marshal(replacementFields)
		if err != nil {
			return nil, err
		}
		frames[i], err = json.Marshal(envelope)
		if err != nil {
			return nil, err
		}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("MCP_WIRE_LIMIT_EXCEEDED: no alias result can be replaced")
	}
	if !batch {
		return append(frames[0], '\n'), nil
	}
	encoded, err := json.Marshal(frames)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

type serverMode struct {
	manyRequired      bool
	aliasOriginChange bool
}

func parseMode(args []string) (serverMode, error) {
	var mode serverMode
	for _, arg := range args {
		switch {
		case arg == "--many-required" && !mode.manyRequired:
			mode.manyRequired = true
		case arg == "--alias-origin-change" && !mode.aliasOriginChange:
			mode.aliasOriginChange = true
		default:
			return serverMode{}, fmt.Errorf("usage: gooo-service-grant-mcp [--many-required] [--alias-origin-change]")
		}
	}
	return mode, nil
}

func frameMax(mode serverMode) int {
	if mode.aliasOriginChange {
		return aliasFrameMax
	}
	if mode.manyRequired {
		return manyFrameMax
	}
	return defaultFrameMax
}

func newServer(mode serverMode) *mcp.Server {
	instructions := "Assess supplied facts with the pinned Gooo generation. One required request per change. ELIGIBLE, REFUSED and INVALID_INPUT are decisions; isError indicates a tool failure. This server applies no grant, authenticates no authority and contacts no service."
	if mode.manyRequired {
		instructions += " Use assess_service_grant for one required service request, or assess_service_grant_change for an ordered list. Preserve the complete returned facts, decision and source bindings. Neither tool applies a change."
	}
	if mode.aliasOriginChange {
		instructions += " Use gooo_compare_alias_origin to compare one alias across supplied before/after tool catalogs. Inspect decision.state and the original row indexes; for ORIGIN_CHANGED review the changed origin, and for AMBIGUOUS repair the duplicate exposed names before comparing fresh facts. The result proves neither live server identity nor host routing or authority. execution_authorized is always false; retain the complete report."
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "gooo-service-grant", Version: "0.1.0-dev"},
		&mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{},
			Instructions: instructions})
	closed := false
	server.AddTool(&mcp.Tool{
		Name:        toolName,
		Title:       "Check a service-grant narrowing",
		Description: "Use when considering a narrower service grant that must preserve one required service request. Assesses whether after is a supported subset of before and still matches required_type and required_name. For an ordered request list, use assess_service_grant_change when available. Domain-invalid facts return INVALID_INPUT; malformed arguments return a tool error with no decision. Supplied facts are not authenticated; no grant or API operation is applied.",
		InputSchema: factsSchema(), OutputSchema: proofSchema(),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, assess)
	if mode.manyRequired {
		server.AddTool(&mcp.Tool{
			Name:        manyToolName,
			Title:       "Preserve an ordered service-request list",
			Description: "Use when one proposed service-grant narrowing must preserve an ordered list of required service requests, including duplicates. For one request with flat required_type/required_name fields, use assess_service_grant. Domain-invalid facts and request-limit excess return INVALID_INPUT decisions; malformed arguments return a tool error with no decision. Supplied facts are not authenticated; no grant or service operation is applied.",
			InputSchema: manyFactsSchema(), OutputSchema: manyProofSchema(),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
		}, assessMany)
	}
	if mode.aliasOriginChange {
		server.AddTool(&mcp.Tool{
			Name: aliasToolName, Title: "Compare a tool alias's declared origin",
			Description: "Use before a tool-catalog update when an agent-selected exposed name may point to a different server or original function. Supply one target_alias and exact before/after rows with server, original_name and exposed_name. Returns UNCHANGED, ORIGIN_CHANGED, ADDED, REMOVED, ABSENT, AMBIGUOUS or INVALID_INPUT; inspect the original zero-based indexes. Any duplicate exposed name anywhere in either catalog is AMBIGUOUS. Empty domain text returns INVALID_INPUT; malformed JSON or transport limits return a tool error without a report. Preserve supplied spelling and order. These are supplied snapshots, not proof of live server identity, SDK routing, completeness or authority. No tool is invoked and execution_authorized is always false.",
			InputSchema: aliasSchema(aliasInputSchemaJSON), OutputSchema: aliasSchema(aliasOutputSchemaJSON),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
		}, compareAlias)
	}
	return server
}

func main() {
	mode, err := parseMode(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := newServer(mode)
	var transport mcp.Transport = &mcp.StdioTransport{MaxLineLength: frameMax(mode)}
	if mode.aliasOriginChange {
		transport = &mcp.IOTransport{Reader: os.Stdin, Writer: &aliasWireWriter{out: os.Stdout}, MaxLineLength: frameMax(mode)}
	}
	if err := server.Run(ctx, transport); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// These are verbatim schema snapshots from the pinned public alias module,
// not a second specification. verify_alias.py compares discovery with that
// module's schemas. Keeping them in the executable needs no runtime checkout.
func aliasSchema(raw string) map[string]any {
	var schema map[string]any
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		panic(err)
	}
	return schema
}

const aliasInputSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/kimjooyoon/gooo-go-ports/alias-origin-change/input.schema.json",
  "type": "object",
  "properties": {
    "target_alias": {
      "type": "string",
      "maxLength": 1024
    },
    "before": {
      "type": "array",
      "maxItems": 256,
      "items": {
        "type": "object",
        "properties": {
          "server": {
            "type": "string",
            "maxLength": 1024
          },
          "original_name": {
            "type": "string",
            "maxLength": 1024
          },
          "exposed_name": {
            "type": "string",
            "maxLength": 1024
          }
        },
        "required": [
          "server",
          "original_name",
          "exposed_name"
        ],
        "additionalProperties": false
      }
    },
    "after": {
      "type": "array",
      "maxItems": 256,
      "items": {
        "type": "object",
        "properties": {
          "server": {
            "type": "string",
            "maxLength": 1024
          },
          "original_name": {
            "type": "string",
            "maxLength": 1024
          },
          "exposed_name": {
            "type": "string",
            "maxLength": 1024
          }
        },
        "required": [
          "server",
          "original_name",
          "exposed_name"
        ],
        "additionalProperties": false
      }
    }
  },
  "required": [
    "target_alias",
    "before",
    "after"
  ],
  "additionalProperties": false
}
`

const aliasOutputSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/kimjooyoon/gooo-go-ports/alias-origin-change/output.schema.json",
  "type": "object",
  "properties": {
    "facts": {
      "type": "object",
      "properties": {
        "target_alias": {
          "type": "string",
          "maxLength": 1024
        },
        "before": {
          "type": "array",
          "maxItems": 256,
          "items": {
            "type": "object",
            "properties": {
              "server": {
                "type": "string",
                "maxLength": 1024
              },
              "original_name": {
                "type": "string",
                "maxLength": 1024
              },
              "exposed_name": {
                "type": "string",
                "maxLength": 1024
              }
            },
            "required": [
              "server",
              "original_name",
              "exposed_name"
            ],
            "additionalProperties": false
          }
        },
        "after": {
          "type": "array",
          "maxItems": 256,
          "items": {
            "type": "object",
            "properties": {
              "server": {
                "type": "string",
                "maxLength": 1024
              },
              "original_name": {
                "type": "string",
                "maxLength": 1024
              },
              "exposed_name": {
                "type": "string",
                "maxLength": 1024
              }
            },
            "required": [
              "server",
              "original_name",
              "exposed_name"
            ],
            "additionalProperties": false
          }
        }
      },
      "required": [
        "target_alias",
        "before",
        "after"
      ],
      "additionalProperties": false
    },
    "facts_sha256": {
      "type": "string",
      "pattern": "^sha256:[0-9a-f]{64}$"
    },
    "input_sha256": {
      "type": "string",
      "pattern": "^sha256:[0-9a-f]{64}$"
    },
    "decision": {
      "type": "object",
      "properties": {
        "context_id": {
          "type": "string",
          "pattern": "^gooo/alias-origin/v1/sha256:[0-9a-f]{64}$"
        },
        "state": {
          "enum": [
            "UNCHANGED",
            "ORIGIN_CHANGED",
            "REMOVED",
            "ADDED",
            "ABSENT",
            "INVALID_INPUT",
            "AMBIGUOUS"
          ]
        },
        "reason": {
          "enum": [
            "SAME_DECLARED_ORIGIN",
            "DIFFERENT_DECLARED_ORIGIN",
            "ALIAS_REMOVED_FROM_SUPPLIED_CATALOG",
            "ALIAS_ADDED_TO_SUPPLIED_CATALOG",
            "ALIAS_NOT_IN_SUPPLIED_CATALOGS",
            "INVALID_CATALOG_COLUMNS",
            "DUPLICATE_EXPOSED_NAME"
          ]
        },
        "before_index": {
          "type": "integer",
          "minimum": -1,
          "maximum": 255
        },
        "after_index": {
          "type": "integer",
          "minimum": -1,
          "maximum": 255
        },
        "conflict_catalog": {
          "enum": [
            "",
            "BEFORE",
            "AFTER"
          ]
        },
        "conflict_first_index": {
          "type": "integer",
          "minimum": -1,
          "maximum": 255
        },
        "conflict_second_index": {
          "type": "integer",
          "minimum": -1,
          "maximum": 255
        },
        "execution_authorized": {
          "const": false
        }
      },
      "required": [
        "context_id",
        "state",
        "reason",
        "before_index",
        "after_index",
        "conflict_catalog",
        "conflict_first_index",
        "conflict_second_index",
        "execution_authorized"
      ],
      "additionalProperties": false,
      "oneOf": [
        {
          "properties": {
            "state": {
              "const": "UNCHANGED"
            },
            "reason": {
              "const": "SAME_DECLARED_ORIGIN"
            },
            "before_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "after_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "ORIGIN_CHANGED"
            },
            "reason": {
              "const": "DIFFERENT_DECLARED_ORIGIN"
            },
            "before_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "after_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "REMOVED"
            },
            "reason": {
              "const": "ALIAS_REMOVED_FROM_SUPPLIED_CATALOG"
            },
            "before_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "after_index": {
              "const": -1
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "ADDED"
            },
            "reason": {
              "const": "ALIAS_ADDED_TO_SUPPLIED_CATALOG"
            },
            "before_index": {
              "const": -1
            },
            "after_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "ABSENT"
            },
            "reason": {
              "const": "ALIAS_NOT_IN_SUPPLIED_CATALOGS"
            },
            "before_index": {
              "const": -1
            },
            "after_index": {
              "const": -1
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "INVALID_INPUT"
            },
            "reason": {
              "const": "INVALID_CATALOG_COLUMNS"
            },
            "before_index": {
              "const": -1
            },
            "after_index": {
              "const": -1
            },
            "conflict_catalog": {
              "const": ""
            },
            "conflict_first_index": {
              "const": -1
            },
            "conflict_second_index": {
              "const": -1
            }
          }
        },
        {
          "properties": {
            "state": {
              "const": "AMBIGUOUS"
            },
            "reason": {
              "const": "DUPLICATE_EXPOSED_NAME"
            },
            "before_index": {
              "const": -1
            },
            "after_index": {
              "const": -1
            },
            "conflict_catalog": {
              "enum": [
                "BEFORE",
                "AFTER"
              ]
            },
            "conflict_first_index": {
              "type": "integer",
              "minimum": 0,
              "maximum": 254
            },
            "conflict_second_index": {
              "type": "integer",
              "minimum": 1,
              "maximum": 255
            }
          }
        }
      ]
    },
    "bindings": {
      "type": "object",
      "properties": {
        "schema": {
          "const": "gooo/alias-origin-generation/v1"
        },
        "compiler_source_sha": {
          "type": "string",
          "pattern": "^[0-9a-f]{40}$"
        },
        "go_version": {
          "type": "string",
          "pattern": "^go1\\.[0-9]+\\.[0-9]+$"
        },
        "source": {
          "type": "object",
          "properties": {
            "ports/tool-origin/alias_origin.gooo": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            }
          },
          "required": [
            "ports/tool-origin/alias_origin.gooo"
          ],
          "additionalProperties": false
        },
        "workspace_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "cases_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "generator_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "selected_upstream_pins_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "schema_validation_spec_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "typed_plan_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "raw_projection_sha256": {
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        },
        "projection_transform": {
          "const": "Replace first package token main with kernel; all other bytes unchanged."
        },
        "generated_assets": {
          "type": "object",
          "properties": {
            "internal/kernel/projection_generated.go": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            },
            "internal/kernel/types_generated.go": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            },
            "internal/kernel/bridge_generated.go": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            },
            "go.mod": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            },
            "go.sum": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            },
            "reference/go.mod": {
              "type": "string",
              "pattern": "^sha256:[0-9a-f]{64}$"
            }
          },
          "required": [
            "internal/kernel/projection_generated.go",
            "internal/kernel/types_generated.go",
            "internal/kernel/bridge_generated.go",
            "go.mod",
            "go.sum",
            "reference/go.mod"
          ],
          "additionalProperties": false
        },
        "scope": {
          "type": "string"
        }
      },
      "required": [
        "schema",
        "compiler_source_sha",
        "go_version",
        "source",
        "workspace_sha256",
        "cases_sha256",
        "generator_sha256",
        "selected_upstream_pins_sha256",
        "schema_validation_spec_sha256",
        "typed_plan_sha256",
        "raw_projection_sha256",
        "projection_transform",
        "generated_assets",
        "scope"
      ],
      "additionalProperties": false
    }
  },
  "required": [
    "facts",
    "facts_sha256",
    "decision",
    "bindings"
  ],
  "additionalProperties": false
}
`
