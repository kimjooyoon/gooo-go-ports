// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
// Independent reference for the original scoped diagnostic policy.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Facts struct {
	Surface      string `json:"surface_id"`
	Rule         string `json:"rule_id"`
	Complete     bool   `json:"inventory_complete"`
	Known        bool   `json:"backend_known"`
	CanEnforce   bool   `json:"backend_can_enforce"`
	Before       int64  `json:"prepolicy_matches"`
	Remaining    int64  `json:"remaining_matches"`
	Earlier      int64  `json:"earlier_denied_matches"`
	Construction int64  `json:"construction_denied_matches"`
}

type Result struct {
	Facts
	Status string `json:"status"`
	Reason string `json:"reason"`
	Warn   bool   `json:"warn_unmatched"`
}

func classify(f Facts) Result {
	r := Result{Facts: f}
	valid := f.Surface != "" && f.Rule != "" && (f.Known || !f.CanEnforce)
	for _, count := range []int64{f.Before, f.Remaining, f.Earlier, f.Construction} {
		valid = valid && count >= 0 && count <= 4096
	}
	// Add only after boundedness is established, including hostile integer inputs.
	if valid {
		valid = f.Before == f.Remaining+f.Earlier+f.Construction
	}
	switch {
	case !valid:
		r.Status, r.Reason = "INVALID_FACTS", "SCOPE_OR_MATCH_PARTITION_INVALID"
	case !f.Known:
		r.Status, r.Reason = "UNRESOLVED_BACKEND", "BACKEND_ENFORCEMENT_CONTRACT_UNKNOWN"
	case !f.CanEnforce:
		r.Status, r.Reason = "OUTSIDE_ENFORCEABLE_SURFACE", "BACKEND_CANNOT_ENFORCE_THIS_RULE"
	case !f.Complete:
		r.Status, r.Reason = "UNRESOLVED_INVENTORY", "COMPLETE_PREPOLICY_INVENTORY_REQUIRED"
	case f.Before == 0:
		r.Status, r.Reason, r.Warn = "UNMATCHED_DENY", "NO_MATCH_IN_COMPLETE_ENFORCEABLE_SURFACE", true
	case f.Remaining == 0:
		r.Status, r.Reason = "MATCHES_PRIOR_DENIALS", "MATCHES_ALREADY_ACCOUNTED_BY_PRIOR_DENIALS"
	default:
		r.Status, r.Reason = "MATCHES_CURRENT_SURFACE", "RULE_MATCHES_AVAILABLE_TOOLS_NO_EFFECT_CLAIMED"
	}
	return r
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "expected one canonical facts path")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var rows []Facts
	if err := decoder.Decode(&rows); err != nil {
		panic(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		panic("extra JSON value")
	}
	results := make([]Result, 0, len(rows))
	for _, row := range rows {
		results = append(results, classify(row))
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}
