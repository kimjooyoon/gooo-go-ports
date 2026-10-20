// Command service-grant-mcp transports facts to the immutable Gooo evaluator.
// No policy decision is implemented here and no grant or service is changed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
	many "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "assess_service_grant"

const (
	manyToolName    = "assess_service_grant_change"
	defaultFrameMax = 64 << 10
	// The friendly arguments object has its own1MiB decoder limit. Reserve
	// another64KiB for JSON-RPC framing/metadata only in the explicit profile.
	manyFrameMax      = many.MaxBatchBytes + defaultFrameMax
	manyCompilerPin   = "205896a413e5dea92366309c35ae85e542d055f6"
	manyGenerationPin = "sha256:86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164"
)

func closedObject(properties map[string]any, required []string) map[string]any {
	return map[string]any{"type": "object", "properties": properties,
		"required": required, "additionalProperties": false}
}

func factsSchema() map[string]any {
	properties := map[string]any{}
	for _, name := range []string{"before", "after", "required_type", "required_name"} {
		properties[name] = map[string]any{"type": "string"}
	}
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
	text := map[string]any{"type": "string"}
	request := closedObject(map[string]any{"type": text, "name": text}, []string{"type", "name"})
	// No maxItems/minItems/domain restrictions: Gooo owns the256/257 and
	// empty-list decisions. DecodeInput owns the separate JSON byte limit.
	return closedObject(map[string]any{
		"before": text, "after": text,
		"required_requests": map[string]any{"type": "array", "items": request},
	}, []string{"before", "after", "required_requests"})
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

func parseMode(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == "--many-required" {
		return true, nil
	}
	return false, fmt.Errorf("usage: gooo-service-grant-mcp [--many-required]")
}

func frameMax(enableMany bool) int {
	if enableMany {
		return manyFrameMax
	}
	return defaultFrameMax
}

func newServer(enableMany bool) *mcp.Server {
	instructions := "Assess supplied facts with the pinned Gooo generation. One required request per change. ELIGIBLE, REFUSED and INVALID_INPUT are decisions; isError indicates a tool failure. This server applies no grant, authenticates no authority and contacts no service."
	if enableMany {
		instructions += " The optional assess_service_grant_change tool checks one change against an ordered list of required requests using a separate de99 generation. Its full facts are ordered columns, with12 typed decision fields and separate30-asset source bindings. It applies no change."
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "gooo-service-grant", Version: "0.1.0-dev"},
		&mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{},
			Instructions: instructions})
	closed := false
	server.AddTool(&mcp.Tool{
		Name:        toolName,
		Title:       "Check a service-grant narrowing",
		Description: "Assess whether a candidate service pattern is a supported subset of the previous pattern while preserving one required request. Returns the supplied facts, seven typed decision fields and immutable Gooo source/generation bindings. Supplied facts are not authenticated and no grant or API operation is applied.",
		InputSchema: factsSchema(), OutputSchema: proofSchema(),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, assess)
	if enableMany {
		server.AddTool(&mcp.Tool{
			Name:        manyToolName,
			Title:       "Preserve an ordered service-request list",
			Description: "Assess one grant change while preserving every supplied required request in original order, including duplicates. Input has before, after and a required_requests array of exact type/name pairs. The separate Gooo many-required generation returns complete ordered-column facts,12 typed decision fields and its own30-asset bindings. Domain invalidity and the Gooo257-request refusal are decisions, not tool errors. Supplied facts are not authenticated and no grant or service operation is applied.",
			InputSchema: manyFactsSchema(), OutputSchema: manyProofSchema(),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
		}, assessMany)
	}
	return server
}

func main() {
	enableMany, err := parseMode(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := newServer(enableMany)
	if err := server.Run(ctx, &mcp.StdioTransport{MaxLineLength: frameMax(enableMany)}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
