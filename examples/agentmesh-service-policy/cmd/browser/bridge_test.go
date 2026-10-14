package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
)

func units(values ...string) [][]uint16 {
	fields := make([][]uint16, len(values))
	for i, value := range values {
		fields[i] = utf16.Encode([]rune(value))
	}
	return fields
}

func TestExactUTF16PreservesTextAndRefusesLoss(t *testing.T) {
	for _, text := range []string{"", "\x00\r\n", "가", "🙂", "e\u0301", "\ufffd", strings.Repeat("x", maxFieldUTF16Units)} {
		got, err := exactUTF16(utf16.Encode([]rune(text)))
		if err != nil || got != text {
			t.Fatalf("exact text lost: %q, %v", got, err)
		}
	}
	for _, input := range [][]uint16{{0xd800}, {0xdc00}, {0xd800, 'x'}, {'x', 0xdfff}, {0xd800, 0xd800, 0xdc00}, make([]uint16, maxFieldUTF16Units+1)} {
		if _, err := exactUTF16(input); err == nil {
			t.Fatal("accepted lossy or oversized transport")
		}
	}
}

func TestBrowserProofIsExactCheckedAPIResult(t *testing.T) {
	matcher, err := servicepolicy.EvaluateMatcherChecked(servicepolicy.MatcherFacts{Pattern: "*", ReqType: "mcp", ReqName: "a"})
	if err != nil {
		t.Fatal(err)
	}
	impact, err := servicepolicy.EvaluateImpactChecked(servicepolicy.ImpactFacts{Before: "*", After: "mcp://a", ReqType: "mcp", ReqName: "b"})
	if err != nil {
		t.Fatal(err)
	}
	refinement, err := servicepolicy.EvaluateRefinementChecked(servicepolicy.RefinementFacts{Before: "mcp://*.x", After: "mcp://a.*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		mode   string
		fields [][]uint16
		proof  any
	}{
		{"match", units("*", "mcp", "a"), matcher},
		{"impact", units("*", "mcp://a", "mcp", "b"), impact},
		{"refine", units("mcp://*.x", "mcp://a.*"), refinement},
	} {
		want, err := json.Marshal(struct {
			OK    bool `json:"ok"`
			Proof any  `json:"proof"`
		}{true, row.proof})
		if err != nil || evaluateJSON(row.mode, row.fields) != string(want) {
			t.Fatalf("%s changed the checked proof", row.mode)
		}
	}
	nativeBindings, err := servicepolicy.SourceBindings()
	if err != nil {
		t.Fatal(err)
	}
	var bound servicepolicy.Bindings
	if json.Unmarshal([]byte(bindingsJSON()), &bound) != nil || !reflect.DeepEqual(bound, nativeBindings) {
		t.Fatal("browser source bindings differ from the native API")
	}
}

func TestBrowserDomainRefusalComesFromCheckedGooo(t *testing.T) {
	for _, name := range []string{"가", "🙂", strings.Repeat("a", 129)} {
		var result struct {
			OK    bool                              `json:"ok"`
			Proof servicepolicy.CheckedMatcherProof `json:"proof"`
		}
		if err := json.Unmarshal([]byte(evaluateJSON("match", units("*", "mcp", name))), &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Proof.Facts.ReqName != name || result.Proof.Decision.DomainStatus != "INVALID_INPUT" || result.Proof.Decision.InvalidField != "req_name" || result.Proof.Decision.Matched {
			t.Fatalf("valid Unicode or overlong domain text did not reach Gooo: %+v", result)
		}
	}
	for _, row := range []struct {
		mode   string
		fields [][]uint16
	}{
		{"unknown", units("*", "mcp", "x")}, {"match", units("*", "mcp")},
		{"refine", units("*", "*", "extra")}, {"match", [][]uint16{{'*'}, {'m'}, {0xd800}}},
		{"match", [][]uint16{{'*'}, {'m'}, make([]uint16, maxFieldUTF16Units+1)}},
	} {
		var result map[string]any
		if json.Unmarshal([]byte(evaluateJSON(row.mode, row.fields)), &result) != nil || len(result) != 2 || result["ok"] != false || result["error"] == "" || result["proof"] != nil {
			t.Fatal("transport refusal leaked or invented a proof", result)
		}
	}
}
