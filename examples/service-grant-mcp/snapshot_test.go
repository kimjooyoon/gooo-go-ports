// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	catalog "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func snapshotInput(t *testing.T) catalog.DefinitionSnapshotInput {
	t.Helper()
	page := func(id string) catalog.SnapshotPage {
		return catalog.SnapshotPage{
			RequestJSON:  `{"jsonrpc":"2.0","id":` + id + `,"method":"tools/list","params":{}}`,
			ResponseJSON: `{"jsonrpc":"2.0","id":` + id + `,"result":{"tools":[{"name":"search","description":"Find saved documents","inputSchema":{"type":"object"},"outputSchema":{"type":"string"}}]}}`,
		}
	}
	return catalog.DefinitionSnapshotInput{TargetName: "search", Before: catalog.SnapshotObservation{Pages: []catalog.SnapshotPage{page("1")}}, After: catalog.SnapshotObservation{Pages: []catalog.SnapshotPage{page("2")}}}
}

func snapshotProblem(t *testing.T, result *mcp.CallToolResult, err error) catalog.Problem {
	t.Helper()
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil || len(result.Content) != 1 {
		t.Fatalf("failure exposed a report: %+v %v", result, err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("non-text error")
	}
	var problem catalog.Problem
	if err := json.Unmarshal([]byte(text.Text), &problem); err != nil {
		t.Fatal(err)
	}
	return problem
}

func TestSnapshotProfilesAreOptInAndComposable(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		var args []string
		want := serverMode{manyRequired: bits&1 != 0, aliasOriginChange: bits&2 != 0, definitionSnapshot: bits&4 != 0}
		if want.manyRequired {
			args = append(args, "--many-required")
		}
		if want.aliasOriginChange {
			args = append(args, "--alias-origin-change")
		}
		if want.definitionSnapshot {
			args = append(args, "--definition-snapshot")
		}
		got, err := parseMode(args)
		if err != nil || got != want {
			t.Fatalf("profile %d: %+v %v", bits, got, err)
		}
		if want.definitionSnapshot && frameMax(got) != catalog.MaxInputBytes+defaultFrameMax {
			t.Fatal("snapshot frame allowance")
		}
	}
	for _, args := range [][]string{{"--definition-snapshot", "--definition-snapshot"}, {"--definition-snapshot=false"}, {"--definition-snapshot", ""}, {"--definition-snapshot", "--bad"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("accepted invalid flags %q", args)
		}
	}
}

// The declaration-derived schema must agree with the pinned generated ABI;
// production discovery never derives the contract by reflecting Go structs.
func TestSnapshotDeclaredOutputMatchesGeneratedABI(t *testing.T) {
	shape := snapshotOutputSchema()["properties"].(map[string]any)["decision"].(map[string]any)
	properties := shape["properties"].(map[string]any)
	required := shape["required"].([]any)
	abi := reflect.TypeOf(catalog.DefinitionSnapshotReport{}.Decision)
	if len(properties) != abi.NumField() || len(required) != abi.NumField() {
		t.Fatal("Gooo declaration and generated ABI field counts differ")
	}
	for i := 0; i < abi.NumField(); i++ {
		field := abi.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		types := map[reflect.Kind]string{
			reflect.String: "string", reflect.Bool: "boolean", reflect.Int: "integer",
			reflect.Int8: "integer", reflect.Int16: "integer", reflect.Int32: "integer", reflect.Int64: "integer",
		}
		want, ok := types[field.Type.Kind()]
		if !ok || required[i] != name || properties[name].(map[string]any)["type"] != want {
			t.Fatalf("declaration-derived field differs at %s", name)
		}
	}
	if strings.Contains(string(aliasJSON(t, snapshotInputSchema())), "maxLength") {
		t.Fatal("UTF-8 byte limits became JSON Schema character counts")
	}
}

// Transport equivalence to the separately verified public generated-Gooo API,
// not a second policy oracle or a new native-language execution claim.
func TestSnapshotCompleteReportsAndSourceText(t *testing.T) {
	base := snapshotInput(t)
	changed := snapshotInput(t)
	changed.After.Pages[0].ResponseJSON = strings.Replace(changed.After.Pages[0].ResponseJSON, `"outputSchema":{"type":"string"}`, `"outputSchema":{"type":"number"}`, 1)
	missing := snapshotInput(t)
	missing.After.Pages = []catalog.SnapshotPage{}
	for state, input := range map[string]catalog.DefinitionSnapshotInput{"OBSERVED_DEFINITION_SAME": base, "OBSERVED_DEFINITION_CHANGED": changed, "UNASSESSED": missing} {
		t.Run(state, func(t *testing.T) {
			raw := append([]byte(" \n"), append(aliasJSON(t, input), ' ')...)
			want, err := catalog.CompareDefinitionSnapshotJSON(raw)
			if err != nil || want.Decision.State != state {
				t.Fatalf("reference %v %v", want.Decision, err)
			}
			result, err := compareSnapshot(context.Background(), aliasCall(raw))
			if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
				t.Fatalf("tool failed %+v %v", result, err)
			}
			got, ok := result.StructuredContent.(catalog.DefinitionSnapshotReport)
			if !ok || !reflect.DeepEqual(got, want) {
				t.Fatal("complete public report changed")
			}
			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok || text.Text != string(aliasJSON(t, want)) {
				t.Fatal("text and structured report differ")
			}
			if got.Projection.InputJSON != string(raw) {
				t.Fatal("received argument bytes were rewritten")
			}
		})
	}
}

func TestSnapshotInputFailuresReturnNoDecision(t *testing.T) {
	for name, raw := range map[string][]byte{
		"duplicate":    []byte(`{"target_name":"x","target_name":"y","before":{"pages":[]},"after":{"pages":[]}}`),
		"null-pages":   []byte(`{"target_name":"x","before":{"pages":null},"after":{"pages":[]}}`),
		"unknown":      []byte(`{"target_name":"x","before":{"pages":[]},"after":{"pages":[]},"decision":"SAME"}`),
		"surrogate":    []byte(`{"target_name":"\ud800","before":{"pages":[]},"after":{"pages":[]}}`),
		"invalid-UTF8": {0xff}, "over-limit": bytes.Repeat([]byte(" "), catalog.MaxInputBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, want := catalog.CompareDefinitionSnapshotJSON(raw)
			result, err := compareSnapshot(context.Background(), aliasCall(raw))
			got := snapshotProblem(t, result, err)
			if want == nil || got.Error() != want.Error() {
				t.Fatalf("public diagnostic changed: got%v want%v", got, want)
			}
		})
	}
	result, err := compareSnapshot(context.Background(), nil)
	if snapshotProblem(t, result, err).Code != "INVALID_JSON_FACTS" {
		t.Fatal("missing request")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = compareSnapshot(ctx, aliasCall(aliasJSON(t, snapshotInput(t))))
	if err != nil || result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatal("cancelled comparison exposed a report")
	}
}

func TestSnapshotGenerationAndAuthorityAreBound(t *testing.T) {
	raw := aliasJSON(t, snapshotInput(t))
	base, err := catalog.CompareDefinitionSnapshotJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*catalog.DefinitionSnapshotReport){
		"bindings":      func(p *catalog.DefinitionSnapshotReport) { p.Bindings = json.RawMessage(`{}`) },
		"schema":        func(p *catalog.DefinitionSnapshotReport) { p.Schema = "other" },
		"origin":        func(p *catalog.DefinitionSnapshotReport) { p.Decision.OriginRelation = "SAME" },
		"authorization": func(p *catalog.DefinitionSnapshotReport) { p.Decision.AuthorizationRelation = "SAME" },
		"execution":     func(p *catalog.DefinitionSnapshotReport) { p.Decision.ExecutionAuthorized = true },
		"behavior":      func(p *catalog.DefinitionSnapshotReport) { p.Decision.BehaviorEquivalenceProven = true },
		"effect":        func(p *catalog.DefinitionSnapshotReport) { p.Decision.LiveEffectPerformed = true },
		"atomicity":     func(p *catalog.DefinitionSnapshotReport) { p.Decision.RemoteAtomicity = "PROVEN" },
	} {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			r, e := snapshotResult(p)
			if snapshotProblem(t, r, e).Code != "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH" {
				t.Fatal("unbound proof accepted")
			}
		})
	}
}

func TestSnapshotWireLimitPreservesOtherResultsAndMetadata(t *testing.T) {
	report, err := catalog.CompareDefinitionSnapshotJSON(aliasJSON(t, snapshotInput(t)))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := snapshotResult(report)
	var resultFields map[string]any
	if json.Unmarshal(aliasJSON(t, result), &resultFields) != nil {
		t.Fatal("result marshal")
	}
	resultFields["_meta"] = map[string]any{"kept": true}
	resultFields["resultType"] = "complete"
	// Artificial oversized wire frame exercises the final SDK-writer boundary.
	// It is not claimed to be an accepted 4MiB public comparison report.
	resultFields["content"] = []any{map[string]any{"type": "text", "text": strings.Repeat("x", aliasWireMax)}}
	other := map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"structuredContent": map[string]any{"keep": true}}}
	raw := append(aliasJSON(t, []any{map[string]any{"jsonrpc": "2.0", "id": 1, "result": resultFields}, other}), '\n')
	var out bytes.Buffer
	writer := snapshotWireWriter{out: &out}
	n, err := writer.Write(raw)
	if err != nil || n != len(raw) || out.Len() >= aliasWireMax {
		t.Fatalf("writer %d %v", n, err)
	}
	var frames []map[string]json.RawMessage
	if json.Unmarshal(out.Bytes(), &frames) != nil || len(frames) != 2 {
		t.Fatal("batch lost")
	}
	var replaced map[string]json.RawMessage
	if json.Unmarshal(frames[0]["result"], &replaced) != nil {
		t.Fatal("replacement")
	}
	var toolResult mcp.CallToolResult
	if json.Unmarshal(frames[0]["result"], &toolResult) != nil || !toolResult.IsError {
		t.Fatal("no bounded tool failure")
	}
	if string(replaced["_meta"]) != `{"kept":true}` || string(replaced["resultType"]) != `"complete"` || !bytes.Equal(frames[1]["result"], aliasJSON(t, other["result"])) {
		t.Fatal("metadata or other result changed")
	}
	out.Reset()
	unknown := append(aliasJSON(t, map[string]any{"result": map[string]any{"content": strings.Repeat("x", aliasWireMax)}}), '\n')
	n, err = writer.Write(unknown)
	if err == nil || n != 0 || out.Len() != 0 {
		t.Fatal("partial oversized unknown frame escaped")
	}
}

type snapshotShortWriter struct{}

func (snapshotShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestSnapshotShortWriteIsFailure(t *testing.T) {
	w := snapshotWireWriter{out: snapshotShortWriter{}}
	if n, e := w.Write([]byte("{}\n")); n != 0 || e != io.ErrShortWrite {
		t.Fatalf("%d %v", n, e)
	}
}
