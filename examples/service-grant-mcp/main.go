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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "assess_service_grant"

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := mcp.NewServer(&mcp.Implementation{Name: "gooo-service-grant", Version: "0.1.0-dev"},
		&mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{},
			Instructions: "Assess supplied facts with the pinned Gooo generation. One required request per change. ELIGIBLE, REFUSED and INVALID_INPUT are decisions; isError indicates a tool failure. This server applies no grant, authenticates no authority and contacts no service."})
	closed := false
	server.AddTool(&mcp.Tool{
		Name:        toolName,
		Title:       "Check a service-grant narrowing",
		Description: "Assess whether a candidate service pattern is a supported subset of the previous pattern while preserving one required request. Returns the supplied facts, seven typed decision fields and immutable Gooo source/generation bindings. Supplied facts are not authenticated and no grant or API operation is applied.",
		InputSchema: factsSchema(), OutputSchema: proofSchema(),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, assess)
	if err := server.Run(ctx, &mcp.StdioTransport{MaxLineLength: 64 << 10}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
