package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	many "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exercise simultaneous assessments against retained native outputs rather
// than a second policy implementation or a list of selected result fields.
func TestConcurrentNativeProofs(t *testing.T) {
	for _, index := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			input, err := os.ReadFile(fmt.Sprintf("fixtures/control-%d.input", index))
			if err != nil {
				t.Fatal(err)
			}
			output, err := os.ReadFile(fmt.Sprintf("fixtures/control-%d.stdout", index))
			if err != nil {
				t.Fatal(err)
			}
			var facts, expected []json.RawMessage
			if json.Unmarshal(input, &facts) != nil || len(facts) != 1 || json.Unmarshal(output, &expected) != nil || len(expected) != 1 {
				t.Fatal("native fixture must contain one input and one complete proof")
			}
			var want any
			if err := json.Unmarshal(expected[0], &want); err != nil {
				t.Fatal(err)
			}
			for range 4 {
				result, err := assess(context.Background(), &mcp.CallToolRequest{
					Params: &mcp.CallToolParamsRaw{Arguments: facts[0]},
				})
				if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
					t.Fatalf("assessment failed: result=%+v err=%v", result, err)
				}
				raw, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				var got any
				if json.Unmarshal(raw, &got) != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("structured result differs from the complete native proof")
				}
				text, ok := result.Content[0].(*mcp.TextContent)
				if !ok || json.Unmarshal([]byte(text.Text), &got) != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("text result differs from the complete native proof")
				}
			}
		})
	}
}

func TestInvalidFactsReturnNoDecision(t *testing.T) {
	for name, raw := range map[string]string{
		"duplicate":  `{"before":"*","after":"*","required_type":"mcp","required_name":"weather","before":"http://*"}`,
		"missing":    `{"before":"*","after":"*","required_type":"mcp"}`,
		"extra":      `{"before":"*","after":"*","required_type":"mcp","required_name":"weather","decision":"ELIGIBLE"}`,
		"null":       `{"before":"*","after":"*","required_type":"mcp","required_name":null}`,
		"number":     `{"before":"*","after":"*","required_type":"mcp","required_name":1}`,
		"surrogate":  `{"before":"*","after":"*","required_type":"mcp","required_name":"\ud800"}`,
		"array":      `[]`,
		"empty":      "",
		"over-limit": strings.Repeat(" ", 4097),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := assess(context.Background(), &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)},
			})
			if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
				t.Fatalf("invalid facts returned a decision: result=%+v err=%v", result, err)
			}
		})
	}
}

func TestCancelledAssessmentReturnsNoDecision(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := assess(ctx, nil)
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatalf("cancelled request returned a decision: result=%+v err=%v", result, err)
	}
}

// These draft tests are transport tests against the separately pinned public
// many module. They are not a replacement independent policy oracle.
func manyResult(t *testing.T, raw []byte) many.Result {
	t.Helper()
	result, err := assessMany(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)},
	})
	if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("many assessment failed: result=%+v err=%v", result, err)
	}
	proof, ok := result.StructuredContent.(many.Result)
	if !ok {
		t.Fatal("many structured content lost the typed full Result")
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != string(encoded) {
		t.Fatal("many text and complete structured proof differ")
	}
	if proof.Bindings.CompilerSourceSHA != manyCompilerPin || proof.Bindings.GenerationSHA256 != manyGenerationPin || len(proof.Bindings.Assets) != 30 || len(proof.Bindings.Sources) != 5 {
		t.Fatal("many generation was mixed with the old839/89 profile")
	}
	return proof
}

func TestManyProfileIsExplicit(t *testing.T) {
	legacy, err := parseMode(nil)
	if err != nil || legacy != (serverMode{}) || frameMax(legacy) != 64<<10 {
		t.Fatal("default no-argument profile changed")
	}
	enabled, err := parseMode([]string{"--many-required"})
	if err != nil || !enabled.manyRequired || enabled.aliasOriginChange || frameMax(enabled) != many.MaxBatchBytes+(64<<10) {
		t.Fatal("many profile did not reserve bounded JSON-RPC overhead")
	}
	for _, args := range [][]string{{"--many-required", "--many-required"}, {"--many-required", "extra"}, {"--many-required=false"}, {"--compact-proof-v1"}, {"extra"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("unexpected arguments accepted: %q", args)
		}
	}
}

func TestManySchemasDoNotPreemptGoooPolicy(t *testing.T) {
	input := manyFactsSchema()
	if input["additionalProperties"] != false {
		t.Fatal("many input is not closed")
	}
	properties := input["properties"].(map[string]any)
	if len(properties) != 3 {
		t.Fatal("many input must contain only friendly transport fields")
	}
	requests := properties["required_requests"].(map[string]any)
	if _, exists := requests["maxItems"]; exists {
		t.Fatal("transport must not refuse257 ahead of Gooo")
	}
	if _, exists := requests["minItems"]; exists {
		t.Fatal("empty-list invalidity belongs to Gooo")
	}
	pair := requests["items"].(map[string]any)
	if pair["additionalProperties"] != false || len(pair["properties"].(map[string]any)) != 2 {
		t.Fatal("required request is not an exact type/name pair")
	}
	output := manyProofSchema()["properties"].(map[string]any)
	decision := output["decision"].(map[string]any)
	fields := decision["properties"].(map[string]any)
	if decision["additionalProperties"] != false || len(fields) != 12 || len(decision["required"].([]string)) != 12 {
		t.Fatal("many decision must retain all12 typed fields")
	}
	for _, name := range []string{"invalid_index", "first_counterexample", "request_count", "checked_count"} {
		if fields[name].(map[string]any)["type"] != "integer" {
			t.Fatal("many integer field changed: " + name)
		}
	}
	for _, name := range []string{"all_preserved", "supported_subset", "counterexample_before_match", "counterexample_after_match"} {
		if fields[name].(map[string]any)["type"] != "boolean" {
			t.Fatal("many Boolean field changed: " + name)
		}
	}
}

func TestManyTransportReturnsTheCompleteGeneratedAPIResult(t *testing.T) {
	for name, raw := range map[string]string{
		"two-preserved":        `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}`,
		"second-lost":          `{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}`,
		"domain-invalid-later": `{"before":"*","after":"http://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"weather/forecast"}]}`,
		"empty-is-decision":    `{"before":"*","after":"mcp://*","required_requests":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input, err := many.DecodeInput([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			want := many.Assess(input)
			got := manyResult(t, []byte(raw))
			if !reflect.DeepEqual(got, want) {
				t.Fatal("MCP changed facts, a decision field or complete source bindings")
			}
			if name == "domain-invalid-later" && (got.Decision.Status != "INVALID_INPUT" || got.Decision.InvalidIndex != 1) {
				t.Fatal("represented invalidity became a transport failure")
			}
		})
	}
}

func TestManyRequestOrderAndDuplicatesArePreserved(t *testing.T) {
	proof := manyResult(t, []byte(`{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}`))
	wantFacts := many.RequestColumns{Before: "*", After: "mcp://weather", RequiredTypes: []string{"mcp", "mcp", "mcp"}, RequiredNames: []string{"weather", "weather", "calculator"}}
	wantDecision := many.Decision{Status: "REFUSED", Reason: "REQUIRED_REQUEST_REMOVED", InvalidField: "", InvalidIndex: -1, FirstCounterexample: 2, RequestCount: 3, CheckedCount: 3, AllPreserved: false, SupportedSubset: true, SubsetRuleID: "OLD_GLOBAL", CounterexampleBeforeMatch: true, CounterexampleAfterMatch: false}
	if !reflect.DeepEqual(proof.Facts, wantFacts) || proof.Decision != wantDecision {
		t.Fatal("order/duplicate-sensitive original corpus case changed")
	}
}

func TestMany257RequestsReachTheGeneratedGoooGuard(t *testing.T) {
	input := many.Input{Before: "*", After: "mcp://*", RequiredRequests: make([]many.Request, 257)}
	for i := range input.RequiredRequests {
		input.RequiredRequests[i] = many.Request{Type: "mcp", Name: "weather"}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	proof := manyResult(t, raw)
	want := many.Decision{Status: "INVALID_INPUT", Reason: "REQUIRED_REQUEST_LIMIT_EXCEEDED", InvalidField: "required_types", InvalidIndex: -1, FirstCounterexample: -1, RequestCount: 257, CheckedCount: 0, AllPreserved: false, SupportedSubset: false, SubsetRuleID: "", CounterexampleBeforeMatch: false, CounterexampleAfterMatch: false}
	if proof.Decision != want || len(proof.Facts.RequiredNames) != 257 || len(proof.Facts.RequiredTypes) != 257 {
		t.Fatal("257 was dropped or refused before Gooo returned its complete decision")
	}
}

func TestManyStructuralErrorsReturnNoPartialProof(t *testing.T) {
	for name, raw := range map[string]string{
		"missing":                 `{"before":"*","after":"mcp://*"}`,
		"extra":                   `{"before":"*","after":"mcp://*","required_requests":[],"decision":"ELIGIBLE"}`,
		"decoded-duplicate":       `{"before":"*","b\u0065fore":"*","after":"mcp://*","required_requests":[]}`,
		"null-list":               `{"before":"*","after":"mcp://*","required_requests":null}`,
		"late-bad-type":           `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":1}]}`,
		"late-extra":              `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator","extra":true}]}`,
		"late-duplicate":          `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator","name":"weather"}]}`,
		"lone-surrogate":          `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"\ud800"}]}`,
		"raw-utf8":                "{\"before\":\"*\",\"after\":\"mcp://*\",\"required_requests\":[{\"type\":\"mcp\",\"name\":\"\xff\"}]}",
		"array-instead-of-object": `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := assessMany(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)}})
			if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
				t.Fatalf("structural error returned a partial proof: result=%+v err=%v", result, err)
			}
		})
	}
}

func TestManyArgumentByteLimitUsesOtherwiseValidJSON(t *testing.T) {
	base := `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"}]}`
	exact := []byte("{" + strings.Repeat(" ", many.MaxBatchBytes-len(base)) + base[1:])
	if len(exact) != many.MaxBatchBytes || manyResult(t, exact).Decision.Status != "ELIGIBLE" {
		t.Fatal("otherwise valid exact1MiB arguments were refused")
	}
	result, err := assessMany(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: append(exact, ' ')}})
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatal("otherwise valid1MiB+1 arguments did not fail structurally")
	}
}

type cancelAfterChecks struct {
	context.Context
	calls int
	after int
}

func (ctx *cancelAfterChecks) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.after {
		return context.Canceled
	}
	return nil
}

func TestManyCancellationReturnsNoDecision(t *testing.T) {
	for _, boundary := range []int{1, 2, 3, 4} {
		ctx := &cancelAfterChecks{Context: context.Background(), after: boundary}
		request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"}]}`)}}
		result, err := assessMany(ctx, request)
		if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
			t.Fatalf("cancellation at explicit check%d returned a decision", boundary)
		}
	}
	result, err := assessMany(context.Background(), nil)
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatal("nil request returned a proof")
	}
}
