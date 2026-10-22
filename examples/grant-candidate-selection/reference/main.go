// Independent finite reference: validate rows, filter, then stable-sort options.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type request struct {
	Phase       string   `json:"phase"`
	Context     string   `json:"context_id"`
	IDs         []string `json:"candidate_ids"`
	Assessments []string `json:"assessment_ids"`
	Contexts    []string `json:"context_ids"`
	Statuses    []string `json:"statuses"`
	Priorities  []int64  `json:"priorities"`
	Bindings    []bool   `json:"bindings_verified"`
}

type candidate struct {
	index      int
	preference int64
}

func evaluate(r request) map[string]any {
	n := len(r.IDs)
	out := map[string]any{"context_id": r.Context, "state": "REFUSED", "reason": "NO_ELIGIBLE_ASSESSMENT",
		"selected_candidate_id": "", "selected_assessment_id": "", "selected_index": -1,
		"selected_priority": int64(0), "candidate_count": n, "eligible_count": 0,
		"invalid_index": -1, "unknown_index": -1, "execution_authorized": false}
	invalid := func() map[string]any {
		out["state"], out["reason"] = "INVALID_INPUT", "INVALID_CANDIDATE_COLUMNS"
		return out
	}
	if r.Context == "" || n < 2 || n > 32 || len(r.Priorities) != n || (r.Phase != "ADMIT" && r.Phase != "SELECT") {
		return invalid()
	}
	seen := map[string]bool{}
	for i, id := range r.IDs {
		if out["invalid_index"] == -1 && (id == "" || len(id) > 128 || r.Priorities[i] < 0 || seen[id]) {
			out["invalid_index"] = i
		}
		seen[id] = true
	}
	if out["invalid_index"] != -1 {
		return invalid()
	}
	if r.Phase == "ADMIT" {
		out["state"], out["reason"] = "ADMITTED", "CANDIDATE_COLUMNS_ADMITTED"
		return out
	}
	if len(r.Assessments) != n || len(r.Contexts) != n || len(r.Statuses) != n || len(r.Bindings) != n {
		return invalid()
	}
	options := []candidate{}
	validStatuses := map[string]bool{"ELIGIBLE": true, "REFUSED": true, "INVALID_INPUT": true}
	for i := range r.IDs {
		if out["invalid_index"] == -1 && (r.Assessments[i] == "" || !validStatuses[r.Statuses[i]]) {
			out["invalid_index"] = i
		}
		if out["unknown_index"] == -1 && (!r.Bindings[i] || r.Contexts[i] != r.Context) {
			out["unknown_index"] = i
		}
		if r.Statuses[i] == "ELIGIBLE" {
			options = append(options, candidate{i, r.Priorities[i]})
		}
	}
	if out["invalid_index"] != -1 {
		return invalid()
	}
	if out["unknown_index"] != -1 {
		out["state"], out["reason"] = "UNKNOWN", "UNBOUND_OR_DIFFERENT_CONTEXT"
		return out
	}
	if len(options) != 0 {
		sort.SliceStable(options, func(i, j int) bool { return options[i].preference < options[j].preference })
		first := options[0].index
		out["state"], out["reason"] = "PROPOSED", "LOWEST_SUPPLIED_PRIORITY_STABLE_INPUT_ORDER"
		out["selected_candidate_id"], out["selected_assessment_id"] = r.IDs[first], r.Assessments[first]
		out["selected_index"], out["selected_priority"], out["eligible_count"] = first, r.Priorities[first], len(options)
	}
	return out
}

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var rows []request
	if err := json.Unmarshal(raw, &rows); err != nil {
		panic(err)
	}
	cases := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cases = append(cases, map[string]any{
			"inputs":   map[string]any{"gooo/service-policy/candidate-selection:SelectCandidate": row},
			"expected": map[string]any{"gooo/service-policy/candidate-selection:SelectCandidate": evaluate(row)}})
	}
	encoded, err := json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": cases}, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
