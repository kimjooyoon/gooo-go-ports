// Execute exact copied upstream functions; never used by the native adapter.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	oracle "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/oracle"
	"os"
	"runtime"
)

func main() {
	cases := flag.String("cases", "../matcher-cases.json", "retained source-pinned matcher corpus")
	impactCases := flag.String("impact-cases", "../impact-cases.json", "original impact facts")
	flag.Parse()
	if flag.NArg() != 0 {
		panic("unexpected arguments")
	}
	raw, err := os.ReadFile(*cases)
	if err != nil {
		panic(err)
	}
	var rows []struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		ReqType string `json:"req_type"`
		ReqName string `json:"req_name"`
		Want    *bool  `json:"want"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		panic(err)
	}
	output := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		actual := oracle.MatchServicePattern(row.Pattern, row.ReqType, row.ReqName)
		if row.Want != nil && actual != *row.Want {
			panic(fmt.Sprintf("upstream expectation differs: %s", row.Name))
		}
		output = append(output, map[string]any{"name": row.Name, "pattern": row.Pattern, "req_type": row.ReqType, "req_name": row.ReqName, "matched": actual})
	}
	raw, err = os.ReadFile(*impactCases)
	if err != nil {
		panic(err)
	}
	var impacts []struct {
		Name    string `json:"name"`
		Before  string `json:"before"`
		After   string `json:"after"`
		ReqType string `json:"req_type"`
		ReqName string `json:"req_name"`
	}
	if err := json.Unmarshal(raw, &impacts); err != nil {
		panic(err)
	}
	impactMatches := make([]map[string]any, 0, len(impacts))
	for _, r := range impacts {
		impactMatches = append(impactMatches, map[string]any{"name": r.Name, "before_match": oracle.MatchServicePattern(r.Before, r.ReqType, r.ReqName), "after_match": oracle.MatchServicePattern(r.After, r.ReqType, r.ReqName)})
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "gooo/agentmesh-matcher-oracle/v1", "go_version": runtime.Version(), "source_revision": "9fd1a41a15174df0aaef601c942410c2116bff91", "observations": output, "impact_matches": impactMatches}); err != nil {
		panic(err)
	}
}
