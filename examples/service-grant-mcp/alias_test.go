package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	aliasorigin "github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const aliasChangedInput = ` {"target_alias":"docs_search","before":[{"server":"docs","original_name":"search","exposed_name":"docs_search"}],"after":[{"server":"issues","original_name":"query","exposed_name":"docs_search"}]} `

func aliasCall(raw []byte) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}}
}

func aliasJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func aliasProblem(t *testing.T, result *mcp.CallToolResult, err error) aliasorigin.Problem {
	t.Helper()
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil || len(result.Content) != 1 {
		t.Fatalf("tool failure exposed a report: result=%+v err=%v", result, err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("diagnostic is not text")
	}
	var problem aliasorigin.Problem
	if err := json.Unmarshal([]byte(text.Text), &problem); err != nil {
		t.Fatal(err)
	}
	return problem
}

func TestAliasProfileIsExplicitAndComposable(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		mode  serverMode
		frame int
	}{
		{nil, serverMode{}, defaultFrameMax},
		{[]string{"--many-required"}, serverMode{manyRequired: true}, manyFrameMax},
		{[]string{"--alias-origin-change"}, serverMode{aliasOriginChange: true}, aliasFrameMax},
		{[]string{"--many-required", "--alias-origin-change"}, serverMode{true, true}, aliasFrameMax},
		{[]string{"--alias-origin-change", "--many-required"}, serverMode{true, true}, aliasFrameMax},
	} {
		mode, err := parseMode(tc.args)
		if err != nil || mode != tc.mode || frameMax(mode) != tc.frame {
			t.Fatalf("wrong profile: %v %v %v", tc.args, mode, err)
		}
	}
	for _, args := range [][]string{{"--alias-origin-change", "--alias-origin-change"}, {"--alias-origin-change=false"}, {"--many-required", "--alias-origin-change", "--many-required"}, {"--alias-origin-change", "extra"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("accepted unsupported arguments: %v", args)
		}
	}
	if defaultFrameMax != 64<<10 || manyFrameMax != (1<<20)+(64<<10) || aliasFrameMax != (1<<20)+(64<<10) {
		t.Fatal("legacy frame caps changed or alias arguments lost their framing allowance")
	}
}

// This checks transport equivalence to the independently verified public
// module; it is deliberately not another alias-comparison implementation.
func TestAliasTransportPreservesCompletePublicReports(t *testing.T) {
	base := aliasorigin.Input{TargetAlias: "docs_search", Before: []aliasorigin.Entry{{Server: "docs", OriginalName: "search", ExposedName: "docs_search"}}, After: []aliasorigin.Entry{{Server: "issues", OriginalName: "query", ExposedName: "docs_search"}}}
	cases := map[string]aliasorigin.Input{"ORIGIN_CHANGED": base}
	same := base
	same.After = append([]aliasorigin.Entry(nil), base.Before...)
	cases["UNCHANGED"] = same
	removed := base
	removed.After = []aliasorigin.Entry{}
	cases["REMOVED"] = removed
	added := base
	added.Before = []aliasorigin.Entry{}
	cases["ADDED"] = added
	absent := base
	absent.TargetAlias = "absent"
	cases["ABSENT"] = absent
	invalid := base
	invalid.TargetAlias = ""
	cases["INVALID_INPUT"] = invalid
	ambiguous := base
	ambiguous.After = append(append([]aliasorigin.Entry(nil), base.After...), aliasorigin.Entry{Server: "status", OriginalName: "check", ExposedName: "health"}, aliasorigin.Entry{Server: "metrics", OriginalName: "status", ExposedName: "health"})
	cases["AMBIGUOUS"] = ambiguous
	for state, input := range cases {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			raw := append([]byte(" \n"), append(aliasJSON(t, input), ' ')...)
			want, err := aliasorigin.CompareJSON(raw)
			if err != nil || want.Decision.State != state {
				t.Fatalf("public comparison failed: %v %v", want.Decision, err)
			}
			for range 3 {
				result, err := compareAlias(context.Background(), aliasCall(raw))
				if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
					t.Fatalf("comparison failed: %+v %v", result, err)
				}
				got, ok := result.StructuredContent.(aliasorigin.Report)
				if !ok || !reflect.DeepEqual(got, want) {
					t.Fatal("complete typed report or received-byte digest changed")
				}
				text, ok := result.Content[0].(*mcp.TextContent)
				if !ok || text.Text != string(aliasJSON(t, want)) {
					t.Fatal("text report differs from the complete public result")
				}
			}
		})
	}
}

func TestAliasTransportErrorsReturnNoReport(t *testing.T) {
	for name, raw := range map[string][]byte{
		"duplicate":  []byte(`{"target_alias":"a","target_alias":"b","before":[],"after":[]}`),
		"null":       []byte(`{"target_alias":"a","before":null,"after":[]}`),
		"unknown":    []byte(`{"target_alias":"a","before":[],"after":[],"decision":"UNCHANGED"}`),
		"surrogate":  []byte(`{"target_alias":"\ud800","before":[],"after":[]}`),
		"encoding":   {0xff},
		"over-limit": bytes.Repeat([]byte(" "), aliasorigin.MaxInputBytes+1),
		"text-bytes": aliasJSON(t, aliasorigin.Input{TargetAlias: strings.Repeat("é", 513), Before: []aliasorigin.Entry{}, After: []aliasorigin.Entry{}}),
		"collection": aliasJSON(t, aliasorigin.Input{TargetAlias: "a", Before: make([]aliasorigin.Entry, 257), After: []aliasorigin.Entry{}}),
	} {
		t.Run(name, func(t *testing.T) {
			_, want := aliasorigin.CompareJSON(raw)
			result, err := compareAlias(context.Background(), aliasCall(raw))
			problem := aliasProblem(t, result, err)
			if want == nil || problem.Error() != want.Error() {
				t.Fatalf("public diagnostic changed: got %v want %v", problem, want)
			}
		})
	}
	result, err := compareAlias(context.Background(), nil)
	if problem := aliasProblem(t, result, err); problem.Code != "INVALID_JSON_FACTS" {
		t.Fatal(problem)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = compareAlias(ctx, aliasCall([]byte(aliasChangedInput)))
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatal("cancelled comparison returned a report")
	}
}

func TestAliasOneMiBArgumentsAndFourMiBReportAreSeparate(t *testing.T) {
	raw := []byte(aliasChangedInput)
	raw = append(raw, bytes.Repeat([]byte(" "), aliasorigin.MaxInputBytes-len(raw))...)
	result, err := compareAlias(context.Background(), aliasCall(raw))
	if err != nil || result.IsError {
		t.Fatal("exact 1 MiB arguments were rejected", err)
	}

	input := aliasorigin.Input{TargetAlias: "missing", Before: []aliasorigin.Entry{}, After: []aliasorigin.Entry{}}
	for i := 0; i < 180; i++ {
		row := aliasorigin.Entry{Server: strings.Repeat("<", 1024), OriginalName: strings.Repeat("<", 1024), ExposedName: fmt.Sprintf("alias-%d", i)}
		input.Before = append(input.Before, row)
		input.After = append(input.After, row)
	}
	// Literal HTML characters fit the raw input limit but expand sixfold in
	// the report's Go JSON serialization. This is valid public domain input.
	raw = bytes.ReplaceAll(aliasJSON(t, input), []byte(`\u003c`), []byte("<"))
	if len(raw) > aliasorigin.MaxInputBytes {
		t.Fatal("fixture exceeds raw input bound")
	}
	proof, err := aliasorigin.CompareJSON(raw)
	if err != nil || len(aliasJSON(t, proof)) <= aliasorigin.MaxReportBytes {
		t.Fatal("fixture did not reach the independent output bound", err)
	}
	result, err = compareAlias(context.Background(), aliasCall(raw))
	if problem := aliasProblem(t, result, err); problem.Code != "OUTPUT_LIMIT_EXCEEDED" {
		t.Fatal(problem)
	}
	proof.Bindings = append(proof.Bindings, ' ')
	result, err = aliasResult(proof)
	if problem := aliasProblem(t, result, err); problem.Code != "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH" {
		t.Fatal("altered bindings were accepted", problem)
	}
}

func TestAliasWireLimitCountsTheCompleteFrame(t *testing.T) {
	result, err := compareAlias(context.Background(), aliasCall([]byte(aliasChangedInput)))
	if err != nil || result.IsError {
		t.Fatal(err)
	}
	frame := aliasJSON(t, map[string]any{"jsonrpc": "2.0", "id": "id\"\\\u2028", "result": result})
	atLimit := append(append([]byte(nil), frame...), bytes.Repeat([]byte(" "), aliasWireMax-len(frame)-1)...)
	atLimit = append(atLimit, '\n')
	var out bytes.Buffer
	w := aliasWireWriter{out: &out}
	if n, err := w.Write(atLimit); err != nil || n != len(atLimit) || !bytes.Equal(out.Bytes(), atLimit) {
		t.Fatal("complete frame at the wire limit changed", n, err)
	}
	out.Reset()
	over := append(append([]byte(nil), atLimit[:len(atLimit)-1]...), ' ', '\n')
	if n, err := w.Write(over); err != nil || n != len(over) {
		t.Fatal("oversized frame did not become a complete tool error", n, err)
	}
	if out.Len() >= aliasWireMax || bytes.Contains(out.Bytes(), []byte("structuredContent")) || bytes.Contains(out.Bytes(), []byte("ORIGIN_CHANGED")) {
		t.Fatal("oversized proof leaked instead of a small diagnostic")
	}
	var response struct {
		ID     string `json:"id"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if json.Unmarshal(out.Bytes(), &response) != nil || response.ID != "id\"\\\u2028" || !response.Result.IsError || len(response.Result.Content) != 1 {
		t.Fatal("error lost the JSON-RPC identity")
	}
	var problem aliasorigin.Problem
	if json.Unmarshal([]byte(response.Result.Content[0].Text), &problem) != nil || problem.Code != "MCP_WIRE_LIMIT_EXCEEDED" {
		t.Fatal("missing explicit wire diagnostic")
	}
}

func TestAliasWireFailurePreservesMixedBatchAndMetadata(t *testing.T) {
	result, err := compareAlias(context.Background(), aliasCall([]byte(aliasChangedInput)))
	if err != nil || result.IsError {
		t.Fatal(err)
	}
	var alias map[string]any
	if json.Unmarshal(aliasJSON(t, result), &alias) != nil {
		t.Fatal("bad result")
	}
	alias["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{"name": "gooo-service-grant", "version": "0.1.0-dev"}}
	alias["resultType"] = "complete"
	legacy := json.RawMessage(`{"jsonrpc":"2.0","id":2,"result":{"structuredContent":{"decision":{"status":"ELIGIBLE"}}}}`)
	batch := aliasJSON(t, []any{map[string]any{"jsonrpc": "2.0", "id": 1, "result": alias}, legacy})
	changed, err := aliasWireFailure(batch)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]json.RawMessage
	if json.Unmarshal(changed, &got) != nil || len(got) != 2 {
		t.Fatal("batch was lost")
	}
	var old, want any
	if json.Unmarshal(aliasJSON(t, got[1]), &old) != nil || json.Unmarshal(legacy, &want) != nil || !reflect.DeepEqual(old, want) {
		t.Fatal("another tool's result changed")
	}
	var next map[string]json.RawMessage
	if json.Unmarshal(got[0]["result"], &next) != nil || !bytes.Equal(next["_meta"], aliasJSON(t, alias["_meta"])) {
		t.Fatal("SDK metadata changed")
	}
	if !bytes.Equal(next["resultType"], []byte(`"complete"`)) {
		t.Fatal("current-protocol completion marker changed")
	}
	if _, ok := next["structuredContent"]; ok {
		t.Fatal("partial proof remained")
	}
	var out bytes.Buffer
	w := aliasWireWriter{out: &out}
	unknown := append([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), bytes.Repeat([]byte(" "), aliasWireMax)...)
	if _, err := w.Write(unknown); err == nil || out.Len() != 0 {
		t.Fatal("unrecognized oversized output was partly written")
	}
}

type aliasShortWriter struct{}

func (aliasShortWriter) Write(raw []byte) (int, error) { return len(raw) - 1, nil }
func TestAliasWireWriterDetectsShortWrites(t *testing.T) {
	w := aliasWireWriter{out: aliasShortWriter{}}
	if _, err := w.Write([]byte("{}\n")); err != io.ErrShortWrite {
		t.Fatal("short output write was silently accepted", err)
	}
}
