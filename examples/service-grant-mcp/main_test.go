package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

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
