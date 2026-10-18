package main

import (
	"bytes"
	"encoding/json"
	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
	"strings"
	"testing"
)

func TestCLIUsesGeneratedGoooAndWholeBatchRefusal(t *testing.T) {
	good := `{"before":"mcp://a","after":"*","req_type":"mcp","req_name":"b"}`
	var out bytes.Buffer
	if err := run("impact", 0, strings.NewReader("["+good+"]"), &out); err != nil {
		t.Fatal(err)
	}
	var result []servicepolicy.ImpactProof
	if json.Unmarshal(out.Bytes(), &result) != nil || len(result) != 1 || result[0].Decision.BeforeMatch || !result[0].Decision.AfterMatch || result[0].Decision.Change != "WIDENED" {
		t.Fatal("Gooo request impact differs")
	}
	for _, bad := range []string{"[]", "null", "[" + good + ",{}]", "[" + good + ", " + strings.Replace(good, `"after":"*"`, `"after":"mcp://a/path"`, 1) + "]", "[" + good + "] {}", strings.Repeat(" ", servicepolicy.MaxBatchBytes+1), "[" + strings.TrimSuffix(strings.Repeat(good+",", servicepolicy.MaxBatchCount+1), ",") + "]"} {
		out.Reset()
		if err := run("impact", 0, strings.NewReader(bad), &out); err == nil {
			t.Fatal("bad batch accepted")
		}
		if out.Len() != 0 {
			t.Fatal("partial result output")
		}
	}
	if err := run("invalid", 0, strings.NewReader("["+good+"]"), &out); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestCLICountAndByteBoundaries(t *testing.T) {
	row := `{"pattern":"","req_type":"","req_name":""}`
	var out bytes.Buffer
	batch := "[" + strings.TrimSuffix(strings.Repeat(row+",", servicepolicy.MaxBatchCount), ",") + "]"
	if len(batch) > servicepolicy.MaxBatchBytes {
		t.Fatal("count probe unexpectedly outside byte bound")
	}
	if err := run("matcher", 0, strings.NewReader(batch), &out); err != nil {
		t.Fatal(err)
	}
	var results []servicepolicy.MatcherProof
	if json.Unmarshal(out.Bytes(), &results) != nil || len(results) != servicepolicy.MaxBatchCount {
		t.Fatal("1024case boundary changed")
	}
	for _, p := range results {
		if p.Matched {
			t.Fatal("empty inputs unexpectedly matched")
		}
	}
	out.Reset()
	single := "[" + row + "]"
	padded := single + strings.Repeat(" ", servicepolicy.MaxBatchBytes-len(single))
	if err := run("matcher", 0, strings.NewReader(padded), &out); err != nil {
		t.Fatal("1MiB boundary rejected", err)
	}
}

func TestCLIRefinementAndWholeBatchRefusal(t *testing.T) {
	good := `{"before":"mcp://*.x","after":"mcp://a.*"}`
	var out bytes.Buffer
	if err := run("refinement", 0, strings.NewReader("["+good+"]"), &out); err != nil {
		t.Fatal(err)
	}
	var result []servicepolicy.RefinementProof
	if json.Unmarshal(out.Bytes(), &result) != nil || len(result) != 1 {
		t.Fatal("refinement result shape")
	}
	d := result[0].Decision
	if d.Status != "WIDENING_WITNESS" || d.RuleID != "CHECKED_CANDIDATE_REQUEST" || d.SupportedSubset || !d.HasWitness || d.WitnessType != "mcp" || d.WitnessName != "a.y" || d.BeforeMatch || !d.AfterMatch {
		t.Fatalf("Gooo second witness differs: %+v", d)
	}
	for _, bad := range []string{"[]", "null", "[" + good + ",{}]", "[" + good + `,{"before":"mcp://*.a.*","after":""}]`, "[" + good + `,{"before":"*","after":"mcp://a/path"}]`, "[" + good + `] {}`, strings.Repeat(" ", servicepolicy.MaxBatchBytes+1), "[" + strings.TrimSuffix(strings.Repeat(good+",", servicepolicy.MaxBatchCount+1), ",") + "]"} {
		out.Reset()
		if err := run("refinement", 0, strings.NewReader(bad), &out); err == nil {
			t.Fatal("invalid refinement batch accepted")
		}
		if out.Len() != 0 {
			t.Fatal("partial refinement output escaped whole-batch refusal")
		}
	}
}

func TestCLIGrantUpdateDecisionsAndOrder(t *testing.T) {
	facts := []servicepolicy.GrantUpdateFacts{
		{Before: "*", After: "mcp://*", RequiredType: "mcp", RequiredName: "weather"},
		{Before: "mcp://*.x", After: "mcp://a.*", RequiredType: "mcp", RequiredName: "a.x"},
		{Before: "*", After: "", RequiredType: "mcp", RequiredName: "weather"},
	}
	want := []servicepolicy.GrantUpdateDecision{
		{Status: "ELIGIBLE", Reason: "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true, SubsetRuleID: "OLD_GLOBAL"},
		{Status: "REFUSED", Reason: "WIDENING_WITNESS", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true},
		{Status: "REFUSED", Reason: "REQUIRED_REQUEST_REMOVED", RequiredObserved: true, RequiredBeforeMatch: true, SubsetRuleID: "CANDIDATE_EMPTY"},
	}
	raw, _ := json.Marshal(facts)
	var out bytes.Buffer
	if err := run("grant-update", 0, bytes.NewReader(raw), &out); err != nil {
		t.Fatal("policy refusal must be a successful evaluation", err)
	}
	var got []servicepolicy.GrantUpdateProof
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != len(want) {
		t.Fatal("grant-update result shape", err)
	}
	for i, p := range got {
		if p.Facts != facts[i] || p.Decision != want[i] || p.Bindings.CompilerSourceSHA != servicepolicy.CompilerPin || p.Bindings.GenerationSHA256 == "" || len(p.Bindings.Assets) == 0 {
			t.Fatalf("case%d facts, Gooo decision or source bindings differ: %+v", i, p)
		}
	}
}

func TestCLIGrantUpdateDomainOutcomesAreNotTransportErrors(t *testing.T) {
	for _, value := range []string{strings.Repeat("x", 129), "é", "a/path", "a\x00b", ""} {
		f := servicepolicy.GrantUpdateFacts{Before: "*", After: "*", RequiredType: "mcp", RequiredName: value}
		raw, _ := json.Marshal([]servicepolicy.GrantUpdateFacts{f})
		var out bytes.Buffer
		if err := run("grant-update", 0, bytes.NewReader(raw), &out); err != nil {
			t.Fatal("represented invalid domain must return a Gooo decision", err)
		}
		var got []servicepolicy.GrantUpdateProof
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 1 {
			t.Fatal("invalid-domain proof shape", err)
		}
		want := servicepolicy.GrantUpdateDecision{Status: "INVALID_INPUT", Reason: "INVALID_CANONICAL_INPUT", InvalidField: "required_name"}
		if value == "" {
			want.Reason = "EMPTY_REQUIRED_REQUEST"
		}
		if got[0].Facts != f || got[0].Decision != want {
			t.Fatalf("domain refusal changed: %+v", got[0])
		}
	}
}

func TestCLIGrantUpdateWholeBatchStructuralRefusal(t *testing.T) {
	good := `{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather"}`
	for _, later := range []string{`{}`, `{"before":"*","after":"*","required_type":"mcp","required_name":null}`, strings.TrimSuffix(good, "}") + `,"before":"*"}`, strings.Replace(good, `"required_name":"weather"`, `"required_name":"\ud800"`, 1), strings.TrimSuffix(good, "}") + `,"decision":"ELIGIBLE"}`} {
		var out bytes.Buffer
		if err := run("grant-update", 0, strings.NewReader("["+good+","+later+"]"), &out); err == nil || out.Len() != 0 {
			t.Fatal("bad later item must fail without any stdout", err, out.Len())
		}
	}
	for _, raw := range []string{"[]", "null", "[" + good + "] {}", strings.Repeat(" ", servicepolicy.MaxBatchBytes+1), "[" + strings.TrimSuffix(strings.Repeat(good+",", servicepolicy.MaxBatchCount+1), ",") + "]"} {
		var out bytes.Buffer
		if err := run("grant-update", 0, strings.NewReader(raw), &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid batch accepted or partially emitted", err)
		}
	}
}
