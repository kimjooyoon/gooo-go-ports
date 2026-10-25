// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
// Independent expected-value oracle for finite saved-definition facts.
// It does not execute Gooo or import the generated implementation.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const entry = "gooo/tool-origin/definition-snapshot:CompareDefinitionSnapshot"
const profile = "gooo/mcp-tool-definition-json/v1"

type Facts struct {
	RequestKinds  []string `json:"page_request_cursor_kinds"`
	RequestValues []string `json:"page_request_cursor_values"`
	NextKinds     []string `json:"page_next_cursor_kinds"`
	NextValues    []string `json:"page_next_cursor_values"`
	Names         []string `json:"original_names"`
	Definitions   []string `json:"definition_ids"`
	Profiles      []string `json:"definition_profiles"`
	Annotations   []string `json:"annotation_ids"`
	RowPages      []int64  `json:"row_page_indexes"`
}

type Selection struct {
	ContextID  string `json:"context_id"`
	TargetName string `json:"target_name"`
}

type Request struct {
	Label     string                     `json:"label"`
	Before    Facts                      `json:"before"`
	After     Facts                      `json:"after"`
	Selection Selection                  `json:"selection"`
	Anchors   map[string]json.RawMessage `json:"anchors"`
}

type Requests struct {
	Schema string    `json:"schema"`
	Cases  []Request `json:"cases"`
}

type observation struct {
	valid                           bool
	coverage, coverageReason        string
	problemPage, problemRow         int
	firstDuplicate, secondDuplicate int
	selected                        int
}

func boundedText(s string) bool {
	return len(s) <= 1024 && !strings.ContainsRune(s, 0)
}

func validDigest(s string) bool {
	tail, ok := strings.CutPrefix(s, "sha256:")
	if !ok || len(tail) != 64 || tail != strings.ToLower(tail) {
		return false
	}
	_, err := hex.DecodeString(tail)
	return err == nil
}

func inspect(f Facts, target string) observation {
	r := observation{true, "COMPLETE", "OBSERVED_PAGE_CHAIN_COMPLETE", -1, -1, -1, -1, -1}
	fail := func(reason string, page, row int) observation {
		r.valid = false
		r.coverage, r.coverageReason = "INVALID", reason
		r.problemPage, r.problemRow = page, row
		return r
	}
	pages, rows := len(f.RequestKinds), len(f.Names)
	// Shape precedence is specified independently as a table of obligations.
	shape := []struct {
		bad    bool
		reason string
	}{
		{pages > 16 || rows > 256, "SNAPSHOT_LIMIT"},
		{len(f.RequestValues) != pages || len(f.NextKinds) != pages || len(f.NextValues) != pages, "PAGE_COLUMN_LENGTH"},
		{len(f.Definitions) != rows || len(f.Profiles) != rows || len(f.Annotations) != rows || len(f.RowPages) != rows, "ROW_COLUMN_LENGTH"},
	}
	for _, rule := range shape {
		if rule.bad {
			return fail(rule.reason, -1, -1)
		}
	}
	requestKinds := map[string]bool{"ABSENT": true, "STRING": true}
	nextKinds := map[string]bool{"ABSENT": true, "NULL": true, "STRING": true}
	for i := range pages {
		kind, next := f.RequestKinds[i], f.NextKinds[i]
		bad := !requestKinds[kind] || !nextKinds[next]
		bad = bad || kind != "STRING" && f.RequestValues[i] != ""
		bad = bad || next != "STRING" && f.NextValues[i] != ""
		bad = bad || !boundedText(f.RequestValues[i]) || !boundedText(f.NextValues[i])
		if bad {
			return fail("INVALID_CURSOR_TAG_OR_VALUE", i, -1)
		}
	}
	for i, name := range f.Names {
		bad := name == "" || !boundedText(name) || f.Profiles[i] == "" || !boundedText(f.Profiles[i])
		bad = bad || !validDigest(f.Definitions[i]) || !validDigest(f.Annotations[i])
		bad = bad || f.RowPages[i] < 0 || f.RowPages[i] >= int64(pages)
		bad = bad || i > 0 && f.RowPages[i] < f.RowPages[i-1]
		if bad {
			return fail("INVALID_ROW_FACT", -1, i)
		}
	}
	firstByName := make(map[string]int, rows)
	for i, name := range f.Names {
		if previous, exists := firstByName[name]; exists {
			if r.firstDuplicate < 0 {
				r.firstDuplicate, r.secondDuplicate = previous, i
			}
		} else {
			firstByName[name] = i
		}
	}
	if r.secondDuplicate >= 0 {
		r.problemRow = r.secondDuplicate
	}
	if index, present := firstByName[target]; present {
		r.selected = index
	}
	// Finite page coverage is separate from observed-row selection and identity.
	if pages == 0 {
		r.coverage, r.coverageReason = "INCOMPLETE", "NO_SAVED_PAGES"
		return r
	}
	links := make([]string, pages)
	if f.RequestKinds[0] != "ABSENT" {
		links[0] = "MISSING_INITIAL_PAGE"
	}
	for i := 1; i < pages; i++ {
		switch {
		case f.NextKinds[i-1] != "STRING":
			links[i] = "PAGE_AFTER_TERMINAL"
		case f.RequestKinds[i] != "STRING" || f.RequestValues[i] != f.NextValues[i-1]:
			links[i] = "CURSOR_LINK_MISMATCH"
		}
	}
	for i, reason := range links {
		if reason != "" {
			r.coverage, r.coverageReason, r.problemPage = "INCOMPLETE", reason, i
			return r
		}
	}
	if f.NextKinds[pages-1] == "STRING" {
		r.coverage, r.coverageReason, r.problemPage = "INCOMPLETE", "MISSING_TERMINAL_PAGE", pages-1
	}
	return r
}

func expected(c Request) map[string]any {
	b, a := inspect(c.Before, c.Selection.TargetName), inspect(c.After, c.Selection.TargetName)
	// An ambiguous page set cannot silently select even an otherwise unique row.
	if b.firstDuplicate >= 0 || a.firstDuplicate >= 0 {
		b.selected, a.selected = -1, -1
	}
	r := map[string]any{
		"context_id": c.Selection.ContextID, "target_name": c.Selection.TargetName,
		"state": "UNASSESSED", "reason": "TARGET_OBSERVATION_INCOMPLETE", "next_action": "OBTAIN_DEFINITION_OBSERVATION",
		"comparison_scope":     "SAVED_ROWS_ONLY",
		"before_page_coverage": b.coverage, "after_page_coverage": a.coverage,
		"before_coverage_reason": b.coverageReason, "after_coverage_reason": a.coverageReason,
		"before_problem_page_index": b.problemPage, "after_problem_page_index": a.problemPage,
		"before_problem_row_index": b.problemRow, "after_problem_row_index": a.problemRow,
		"before_index": b.selected, "after_index": a.selected,
		"before_definition_id": "", "after_definition_id": "",
		"before_definition_profile": "", "after_definition_profile": "",
		"before_annotation_id": "", "after_annotation_id": "",
		"observed_presence_relation": "UNASSESSED", "definition_relation": "UNASSESSED",
		"annotation_relation": "UNASSESSED", "profile_relation": "UNASSESSED",
		"origin_relation": "NOT_ASSESSED", "authorization_relation": "NOT_ASSESSED",
		"behavior_equivalence_proven": false, "execution_authorized": false, "live_effect_performed": false,
		"remote_atomicity": "UNKNOWN",
	}
	if len(r) != 32 {
		panic("result ABI")
	}
	for _, side := range []struct {
		name   string
		facts  Facts
		status observation
	}{{"before", c.Before, b}, {"after", c.After, a}} {
		if side.status.valid && side.status.selected >= 0 {
			i := side.status.selected
			r[side.name+"_definition_id"] = side.facts.Definitions[i]
			r[side.name+"_definition_profile"] = side.facts.Profiles[i]
			r[side.name+"_annotation_id"] = side.facts.Annotations[i]
		}
	}
	validSelection := c.Selection.ContextID != "" && c.Selection.TargetName != "" && boundedText(c.Selection.ContextID) && boundedText(c.Selection.TargetName)
	admission := []struct {
		applies               bool
		state, reason, action string
	}{
		{!validSelection, "INVALID_INPUT", "INVALID_SELECTION_TEXT", "FIX_BOUND_INPUT"},
		{!b.valid || !a.valid, "INVALID_INPUT", "INVALID_SNAPSHOT_FACTS", "FIX_BOUND_INPUT"},
		{b.firstDuplicate >= 0 || a.firstDuplicate >= 0, "AMBIGUOUS_NAME", "DUPLICATE_OBSERVED_NAME", "RESOLVE_OBSERVATION_AMBIGUITY"},
	}
	for _, rule := range admission {
		if rule.applies {
			r["state"], r["reason"], r["next_action"] = rule.state, rule.reason, rule.action
			return r
		}
	}
	both := b.selected >= 0 && a.selected >= 0
	if both {
		r["observed_presence_relation"] = "OBSERVED_BOTH"
		r["profile_relation"] = "DIFFERENT_OBSERVED"
		if r["before_definition_profile"] == r["after_definition_profile"] {
			r["profile_relation"] = "SAME_OBSERVED"
		}
		if r["before_definition_profile"] != profile || r["after_definition_profile"] != profile {
			r["reason"], r["next_action"] = "DEFINITION_PROFILE_NOT_COMPARABLE", "USE_SUPPORTED_DEFINITION_PROFILE"
			return r
		}
		for _, field := range []string{"definition", "annotation"} {
			r[field+"_relation"] = "DIFFERENT_OBSERVED"
			if r["before_"+field+"_id"] == r["after_"+field+"_id"] {
				r[field+"_relation"] = "SAME_OBSERVED"
			}
		}
	}
	choices := []struct {
		applies                         bool
		state, reason, action, presence string
	}{
		{both && r["definition_relation"] == "DIFFERENT_OBSERVED", "OBSERVED_DEFINITION_CHANGED", "SELECTED_DESCRIPTOR_BYTES_DIFFER", "REVIEW_DEFINITION_CHANGE", "OBSERVED_BOTH"},
		{both && r["annotation_relation"] == "DIFFERENT_OBSERVED", "OBSERVED_ANNOTATIONS_CHANGED", "SELECTED_ANNOTATION_CLAIMS_DIFFER", "REVIEW_ANNOTATION_CHANGE", "OBSERVED_BOTH"},
		{both, "OBSERVED_DEFINITION_SAME", "SELECTED_DESCRIPTOR_AND_ANNOTATIONS_SAME", "RETAIN_OBSERVATION", "OBSERVED_BOTH"},
		{b.selected < 0 && a.selected >= 0 && b.coverage == "COMPLETE", "OBSERVED_NAME_ADDED", "ABSENT_FROM_COMPLETE_BEFORE_PAGES", "REVIEW_OBSERVED_ADDITION", "OBSERVED_ADDED"},
		{b.selected >= 0 && a.selected < 0 && a.coverage == "COMPLETE", "OBSERVED_NAME_REMOVED", "ABSENT_FROM_COMPLETE_AFTER_PAGES", "REVIEW_OBSERVED_REMOVAL", "OBSERVED_REMOVED"},
		{b.selected < 0 && a.selected < 0 && b.coverage == "COMPLETE" && a.coverage == "COMPLETE", "OBSERVED_NAME_ABSENT", "ABSENT_FROM_BOTH_COMPLETE_PAGE_SETS", "RETAIN_OBSERVATION", "ABSENT_BOTH_COMPLETE"},
	}
	for _, rule := range choices {
		if rule.applies {
			r["state"], r["reason"], r["next_action"], r["observed_presence_relation"] = rule.state, rule.reason, rule.action, rule.presence
			break
		}
	}
	if len(r) != 32 {
		panic("result ABI")
	}
	return r
}

func main() {
	input := flag.String("input", "", "trusted finite synthetic fixture; empty reads stdin")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "reference: unexpected positional argument")
		os.Exit(2)
	}
	var source io.Reader = os.Stdin
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, "reference: cannot read fixture")
			os.Exit(1)
		}
		defer f.Close()
		source = f
	}
	raw, err := io.ReadAll(io.LimitReader(source, (4<<20)+1))
	if err != nil || len(raw) > 4<<20 {
		fmt.Fprintln(os.Stderr, "reference: input cap")
		os.Exit(1)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var requests Requests
	if err := d.Decode(&requests); err != nil || requests.Schema != "gooo/definition-snapshot-reference-inputs/v1" || len(requests.Cases) == 0 || len(requests.Cases) > 128 {
		fmt.Fprintln(os.Stderr, "reference: invalid finite fixture")
		os.Exit(1)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		fmt.Fprintln(os.Stderr, "reference: extra JSON value")
		os.Exit(1)
	}
	cases := make([]any, 0, len(requests.Cases))
	seen := map[string]bool{}
	for _, c := range requests.Cases {
		if c.Label == "" || seen[c.Label] || len(c.Anchors) < 4 {
			panic("invalid fixture label/anchors")
		}
		seen[c.Label] = true
		value := expected(c)
		for key, anchor := range c.Anchors {
			actual, exists := value[key]
			encoded, err := json.Marshal(actual)
			if err != nil || !exists || !bytes.Equal(encoded, bytes.TrimSpace(anchor)) {
				panic("independent literal anchor: " + c.Label + ":" + key)
			}
		}
		for _, key := range []string{"behavior_equivalence_proven", "execution_authorized", "live_effect_performed"} {
			if value[key] != false {
				panic("proof/effect boundary")
			}
		}
		if value["origin_relation"] != "NOT_ASSESSED" || value["authorization_relation"] != "NOT_ASSESSED" || value["remote_atomicity"] != "UNKNOWN" {
			panic("authority boundary")
		}
		cases = append(cases, map[string]any{"inputs": map[string]any{entry + ".input0": c.Before, entry + ".input1": c.After, entry + ".input2": c.Selection}, "expected": map[string]any{entry: value}})
	}
	raw, err = json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": cases}, "", "  ")
	if err != nil {
		panic(err)
	}
	raw = append(raw, '\n')
	if n, err := os.Stdout.Write(raw); err != nil || n != len(raw) {
		os.Exit(1)
	}
}
