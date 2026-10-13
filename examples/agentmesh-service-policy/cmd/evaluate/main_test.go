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
