// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
// Independent expected-value construction for the original binding policy.
// Collection outcomes are independently authored finite literal anchors; this is not a universal collection oracle.
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

const entry = "gooo/tool-origin/explicit-availability:AssessDefinitionAvailability"
const profile = "gooo/mcp-tool-definition-json/v1"

type Binding struct {
	ContextID           string   `json:"context_id"`
	AliasContextID      string   `json:"alias_context_id"`
	CollectionContextID string   `json:"collection_context_id"`
	BeforeDefinitions   []string `json:"before_definition_ids"`
	AfterDefinitions    []string `json:"after_definition_ids"`
	BeforeProfiles      []string `json:"before_definition_profiles"`
	AfterProfiles       []string `json:"after_definition_profiles"`
	BeforeCapabilities  []string `json:"before_capability_ids"`
	AfterCapabilities   []string `json:"after_capability_ids"`
	BeforePolicy        string   `json:"before_policy_revision"`
	AfterPolicy         string   `json:"after_policy_revision"`
}
type Availability struct {
	BeforeRowPageIndexes []int    `json:"before_row_page_indexes"`
	AfterRowPageIndexes  []int    `json:"after_row_page_indexes"`
	ContextID            string   `json:"context_id"`
	AliasContextID       string   `json:"alias_context_id"`
	CollectionContextID  string   `json:"collection_context_id"`
	DefinitionContextID  string   `json:"definition_context_id"`
	FieldStates          []string `json:"field_states"`
	FieldValues          []string `json:"field_values"`
	BeforeCatalogIndex   int      `json:"before_catalog_index"`
	AfterCatalogIndex    int      `json:"after_catalog_index"`
	CaptureBeginIndexes  []int    `json:"capture_begin_indexes"`
	CaptureEndIndexes    []int    `json:"capture_end_indexes"`
	PageContextIndexes   []int    `json:"page_context_indexes"`
	BeforePolicyIndex    int      `json:"before_policy_index"`
	AfterPolicyIndex     int      `json:"after_policy_index"`
}
type Request struct {
	Availability Availability `json:"availability"`

	Label          string                     `json:"label"`
	Origin         map[string]json.RawMessage `json:"origin"`
	Collection     map[string]json.RawMessage `json:"collection"`
	Parent         map[string]json.RawMessage `json:"parent_literal"`
	Binding        Binding                    `json:"binding"`
	WantState      string                     `json:"anchor_state"`
	WantDefinition string                     `json:"anchor_definition_relation"`
}
type Requests struct {
	Schema string    `json:"schema"`
	Cases  []Request `json:"cases"`
}

func text(m map[string]json.RawMessage, key string) string {
	var value string
	if err := json.Unmarshal(m[key], &value); err != nil {
		panic("invalid fixture Text:" + key)
	}
	return value
}
func integer(m map[string]json.RawMessage, key string) int64 {
	var value int64
	if err := json.Unmarshal(m[key], &value); err != nil {
		panic("invalid fixture integer:" + key)
	}
	return value
}
func boolean(m map[string]json.RawMessage, key string) bool {
	var value bool
	if err := json.Unmarshal(m[key], &value); err != nil {
		panic("invalid fixture boolean:" + key)
	}
	return value
}
func length(m map[string]json.RawMessage, key string) int {
	var value []string
	if err := json.Unmarshal(m[key], &value); err != nil || value == nil {
		panic("invalid fixture collection:" + key)
	}
	return len(value)
}
func relation(left, right string) string {
	if left == "" || right == "" {
		return "UNASSESSED"
	}
	if left == right {
		return "SAME_SUPPLIED"
	}
	return "DIFFERENT_SUPPLIED"
}
func digestSyntax(s string) bool {
	tail, ok := strings.CutPrefix(s, "sha256:")
	if !ok || len(tail) != 64 || tail != strings.ToLower(tail) {
		return false
	}
	_, err := hex.DecodeString(tail)
	return err == nil
}
func expected(c Request) map[string]any {
	b, p := c.Binding, originParent(c)
	before, after := length(c.Origin, "before_aliases"), length(c.Origin, "after_aliases")
	invalid := !validAvailability(c) || b.ContextID == "" || b.AliasContextID != text(c.Origin, "context_id") || b.CollectionContextID != text(c.Collection, "context_id")
	for _, pair := range [][2]int{{len(b.BeforeDefinitions), before}, {len(b.BeforeProfiles), before}, {len(b.BeforeCapabilities), before}, {len(b.AfterDefinitions), after}, {len(b.AfterProfiles), after}, {len(b.AfterCapabilities), after}} {
		invalid = invalid || pair[0] != pair[1]
	}
	r := map[string]any{
		"context_id":            c.Availability.ContextID,
		"origin_context_id":     text(p, "origin_context_id"),
		"collection_context_id": text(p, "context_id"),
		"origin_state":          text(p, "origin_state"), "origin_reason": text(p, "origin_reason"),
		"collection_state": text(p, "state"), "collection_reason": text(p, "reason"),
		"before_index": integer(p, "origin_before_index"), "after_index": integer(p, "origin_after_index"),
		"collection_first_problem_code": text(p, "first_problem_code"),
		"definition_relation":           "UNASSESSED", "definition_profile_relation": "UNASSESSED", "capability_relation": "UNASSESSED",
		"authorization_relation": text(p, "authorization_relation"), "policy_relation": "UNASSESSED",
		"before_definition_id": "", "after_definition_id": "", "before_definition_profile": "", "after_definition_profile": "",
		"before_capability_id": "", "after_capability_id": "", "before_policy_revision": b.BeforePolicy, "after_policy_revision": b.AfterPolicy,
		"state": "UNASSESSED", "reason": "SUPPLIED_BINDINGS_NOT_ASSESSED", "next_action": "COMPLETE_BINDING_OBSERVATION",
		"declared_bindings_match": false, "behavior_equivalence_proven": false, "execution_authorized": false, "live_effect_performed": false,
		"contains_synthetic_fixture": boolean(p, "contains_synthetic_fixture"), "remote_atomicity": text(p, "remote_atomicity"),
	}
	comparable := false
	if !invalid && text(p, "origin_state") == "UNCHANGED" {
		bi, ai := integer(p, "origin_before_index"), integer(p, "origin_after_index")
		if bi < 0 || ai < 0 || bi >= int64(before) || ai >= int64(after) {
			panic("inconsistent parent fixture index")
		}
		bd, ad, bp, ap, bc, ac := b.BeforeDefinitions[bi], b.AfterDefinitions[ai], b.BeforeProfiles[bi], b.AfterProfiles[ai], b.BeforeCapabilities[bi], b.AfterCapabilities[ai]
		r["before_definition_id"], r["after_definition_id"] = bd, ad
		r["before_definition_profile"], r["after_definition_profile"] = bp, ap
		r["before_capability_id"], r["after_capability_id"] = bc, ac
		r["definition_profile_relation"] = relation(bp, ap)
		comparable = bp == profile && ap == profile
		if comparable {
			for _, s := range []string{bd, ad, bc, ac} {
				invalid = invalid || s != "" && !digestSyntax(s)
			}
			if !invalid {
				r["definition_relation"] = relation(bd, ad)
				r["capability_relation"] = relation(bc, ac)
			}
		}
		if c.Availability.FieldStates[c.Availability.BeforePolicyIndex] == "SUPPLIED" && c.Availability.FieldStates[c.Availability.AfterPolicyIndex] == "SUPPLIED" {
			r["policy_relation"] = relation(b.BeforePolicy, b.AfterPolicy)
		}
	}
	// Declared priority table, independent of the Gooo source's branch layout.
	// A known difference must not disappear behind an unrelated missing field.
	type resolution struct {
		applies               bool
		state, reason, action string
	}
	choices := []resolution{
		{invalid, "INVALID_INPUT", "INVALID_OR_UNBOUND_DEFINITION_COLUMNS", "FIX_BOUND_INPUT"},
		{text(p, "origin_state") == "ORIGIN_UNOBSERVED" && (text(p, "state") == "RECORDED_CONTEXT_COMPLETE" || text(p, "state") == "COLLECTION_CONTEXT_UNKNOWN"), "UNASSESSED", "ORIGIN_CONTEXT_NOT_OBSERVED", "OBTAIN_ORIGIN_OBSERVATION"},
		{text(p, "origin_state") != "UNCHANGED" && text(p, "origin_state") != "ORIGIN_UNOBSERVED", "ORIGIN_NOT_UNCHANGED", "ORIGIN_BINDING_REQUIRES_REEVALUATION", "REEVALUATE_ORIGIN"},
		{text(p, "state") == "AUTH_CONTEXT_CHANGED", "REEVALUATE_AUTHORIZATION", "RECORDED_AUTHORIZATION_CONTEXT_CHANGED", "REEVALUATE_POLICY"},
		{text(p, "state") != "RECORDED_CONTEXT_COMPLETE", "UNASSESSED", "COLLECTION_CONTEXT_NOT_ESTABLISHED", text(p, "next_action")},
		{r["definition_relation"] == "DIFFERENT_SUPPLIED", "REEVALUATE_DEFINITION", "SUPPLIED_TOOL_DEFINITION_CHANGED", "REEVALUATE_POLICY"},
		{r["capability_relation"] == "DIFFERENT_SUPPLIED", "REEVALUATE_CAPABILITY", "SUPPLIED_CAPABILITY_CLAIM_CHANGED", "REEVALUATE_POLICY"},
		{r["policy_relation"] == "DIFFERENT_SUPPLIED", "REEVALUATE_POLICY", "SUPPLIED_POLICY_REVISION_CHANGED", "REEVALUATE_POLICY"},
		{!comparable, "UNASSESSED", "DEFINITION_PROFILE_NOT_COMPARABLE", "COMPLETE_BINDING_OBSERVATION"},
		{r["definition_relation"] == "UNASSESSED" || r["capability_relation"] == "UNASSESSED" || r["policy_relation"] == "UNASSESSED", "UNASSESSED", "DEFINITION_CAPABILITY_OR_POLICY_NOT_OBSERVED", "COMPLETE_BINDING_OBSERVATION"},
		{true, "BINDINGS_MATCH", "SAME_SUPPLIED_BINDINGS_ONLY", "CONTINUE_HOST_VALIDATION"},
	}
	for _, choice := range choices {
		if choice.applies {
			r["state"], r["reason"], r["next_action"] = choice.state, choice.reason, choice.action
			break
		}
	}
	if invalid {
		r["definition_relation"], r["capability_relation"], r["policy_relation"] = "UNASSESSED", "UNASSESSED", "UNASSESSED"
	}
	r["declared_bindings_match"] = r["state"] == "BINDINGS_MATCH"
	if len(r) != 32 {
		panic("result ABI")
	}
	return r
}

func texts(m map[string]json.RawMessage, key string) []string {
	var a []string
	if json.Unmarshal(m[key], &a) != nil {
		panic(key)
	}
	return a
}
func ints(m map[string]json.RawMessage, key string) []int {
	var a []int
	if json.Unmarshal(m[key], &a) != nil {
		panic(key)
	}
	return a
}
func validAvailability(c Request) bool {
	a, b, v := c.Availability, c.Binding, c.Collection
	sides, pages := texts(v, "capture_sides"), ints(v, "page_capture_indexes")
	if a.ContextID == "" || a.AliasContextID != text(c.Origin, "context_id") || a.CollectionContextID != text(v, "context_id") || a.DefinitionContextID != b.ContextID || len(a.FieldStates) != len(a.FieldValues) || len(a.FieldStates) > 1444 || len(a.CaptureBeginIndexes) != len(sides) || len(a.CaptureEndIndexes) != len(sides) || len(a.PageContextIndexes) != len(pages) {
		return false
	}
	if len(a.BeforeRowPageIndexes) != length(c.Origin, "before_aliases") || len(a.AfterRowPageIndexes) != length(c.Origin, "after_aliases") {
		return false
	}
	for n, indexes := range [][]int{a.BeforeRowPageIndexes, a.AfterRowPageIndexes} {
		side := "BEFORE"
		if n == 1 {
			side = "AFTER"
		}
		for _, p := range indexes {
			if p < 0 || p >= len(pages) {
				return false
			}
			capture := pages[p]
			if capture < 0 || capture >= len(sides) || sides[capture] != side {
				return false
			}
		}
	}
	values := []string{}
	starts, ends := make([]int, len(sides)), make([]int, len(sides))
	positions := make([]int, len(pages))
	before, after := -1, -1
	for _, side := range []string{"BEFORE", "AFTER"} {
		key := "before_catalog_authorization_context_id"
		if side == "BEFORE" {
			before = len(values)
		} else {
			after = len(values)
			key = "after_catalog_authorization_context_id"
		}
		values = append(values, text(v, key))
		for n, s := range sides {
			if s != side {
				continue
			}
			for _, phase := range []string{"begin", "end"} {
				index := len(values)
				if phase == "begin" {
					starts[n] = index
				} else {
					ends[n] = index
				}
				for _, col := range []string{"bindings", "auth_contexts", "config_revisions", "protocols"} {
					rows := texts(v, "capture_"+phase+"_"+col)
					if len(rows) != len(sides) {
						return false
					}
					values = append(values, rows[n])
				}
				values = append(values, "<optional timestamp>")
			}
			for p, capture := range pages {
				if capture != n {
					continue
				}
				positions[p] = len(values)
				for _, col := range []string{"bindings", "auth_contexts", "config_revisions", "protocols"} {
					rows := texts(v, "page_"+col)
					if len(rows) != len(pages) {
						return false
					}
					values = append(values, rows[p])
				}
				values = append(values, "<optional timestamp>")
			}
		}
	}
	bp := len(values)
	values = append(values, b.BeforePolicy, b.AfterPolicy)
	if a.BeforeCatalogIndex != before || a.AfterCatalogIndex != after || a.BeforePolicyIndex != bp || a.AfterPolicyIndex != bp+1 || len(values) != len(a.FieldValues) {
		return false
	}
	for n := range starts {
		if starts[n] != a.CaptureBeginIndexes[n] || ends[n] != a.CaptureEndIndexes[n] {
			return false
		}
	}
	for n := range positions {
		if positions[n] != a.PageContextIndexes[n] {
			return false
		}
	}
	for n, want := range values {
		state, raw := a.FieldStates[n], a.FieldValues[n]
		if state != "SUPPLIED" && state != "UNOBSERVED" || state == "SUPPLIED" && raw == "" || len(raw) > 1024 {
			return false
		}
		if want != "<optional timestamp>" && raw != want {
			return false
		}
	}
	return true
}

// Independently evaluate the alias facts using a name index and observable
// tuple components. The expected collection verdict remains an authored anchor.
func originParent(c Request) map[string]json.RawMessage {
	p := map[string]json.RawMessage{}
	for k, v := range c.Parent {
		p[k] = v
	}
	put := func(k string, v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		p[k] = raw
	}
	o, a := c.Origin, c.Availability
	target := text(o, "target_alias")
	sides := [2]string{"before", "after"}
	indexes := [2]int{-1, -1}
	invalid := text(o, "context_id") == "" || target == ""
	duplicate := false
	for n, side := range sides {
		aliases, names, servers := texts(o, side+"_aliases"), texts(o, side+"_names"), texts(o, side+"_servers")
		if len(aliases) != len(names) || len(names) != len(servers) {
			invalid = true
			continue
		}
		seen := map[string]bool{}
		for i, alias := range aliases {
			if alias == "" || names[i] == "" || servers[i] == "" {
				invalid = true
			}
			if seen[alias] {
				duplicate = true
			}
			seen[alias] = true
			if alias == target {
				indexes[n] = i
			}
		}
	}
	state, reason := "ABSENT", "ALIAS_NOT_IN_SUPPLIED_CATALOGS"
	switch {
	case invalid:
		state, reason = "INVALID_INPUT", "INVALID_CATALOG_COLUMNS"
		indexes = [2]int{-1, -1}
	case duplicate:
		state, reason = "AMBIGUOUS", "DUPLICATE_EXPOSED_NAME"
		indexes = [2]int{-1, -1}
	case indexes[0] < 0 && indexes[1] >= 0:
		state, reason = "ADDED", "ALIAS_ADDED_TO_SUPPLIED_CATALOG"
	case indexes[0] >= 0 && indexes[1] < 0:
		state, reason = "REMOVED", "ALIAS_REMOVED_FROM_SUPPLIED_CATALOG"
	case indexes[0] >= 0 && indexes[1] >= 0:
		state, reason = "UNCHANGED", "SAME_DECLARED_ORIGIN"
		if texts(o, "before_servers")[indexes[0]] != texts(o, "after_servers")[indexes[1]] || texts(o, "before_names")[indexes[0]] != texts(o, "after_names")[indexes[1]] {
			state, reason = "ORIGIN_CHANGED", "DIFFERENT_DECLARED_ORIGIN"
		}
	}
	if !validAvailability(c) {
		state, reason = "INVALID_INPUT", "INVALID_OR_UNBOUND_ORIGIN_AVAILABILITY"
	} else if state == "UNCHANGED" || state == "ORIGIN_CHANGED" {
		bp, ap := a.BeforeRowPageIndexes[indexes[0]], a.AfterRowPageIndexes[indexes[1]]
		pages := ints(c.Collection, "page_capture_indexes")
		bc, ac := pages[bp], pages[ap]
		known := texts(o, "before_names")[indexes[0]] != texts(o, "after_names")[indexes[1]]
		for _, key := range []string{"capture_host_ids", "capture_connection_ids"} {
			values := texts(c.Collection, key)
			known = known || values[bc] != values[ac]
		}
		if known {
			state, reason = "ORIGIN_CHANGED", "DIFFERENT_SUPPLIED_ORIGIN_COMPONENT"
		} else if a.FieldStates[a.PageContextIndexes[bp]] != "SUPPLIED" || a.FieldStates[a.PageContextIndexes[ap]] != "SUPPLIED" {
			state, reason = "ORIGIN_UNOBSERVED", "BACKEND_BINDING_UNOBSERVED"
		}
	}
	if text(c.Parent, "origin_state") != state || text(c.Parent, "origin_reason") != reason || integer(c.Parent, "origin_before_index") != int64(indexes[0]) || integer(c.Parent, "origin_after_index") != int64(indexes[1]) {
		panic("independent origin anchor: " + c.Label)
	}
	put("origin_state", state)
	put("origin_reason", reason)
	put("origin_before_index", indexes[0])
	put("origin_after_index", indexes[1])
	return p
}

func main() {
	input := flag.String("input", "", "trusted finite request fixture")
	flag.Parse()
	if flag.NArg() != 0 || *input == "" {
		fmt.Fprintln(os.Stderr, "reference: one input fixture required")
		os.Exit(2)
	}
	f, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reference: cannot read input")
		os.Exit(1)
	}
	raw, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	f.Close()
	if err != nil || len(raw) > 4<<20 {
		fmt.Fprintln(os.Stderr, "reference: input cap")
		os.Exit(1)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var requests Requests
	if err := dec.Decode(&requests); err != nil || requests.Schema != "gooo/explicit-availability-reference-inputs/v1" || len(requests.Cases) == 0 || len(requests.Cases) > 128 {
		fmt.Fprintln(os.Stderr, "reference: invalid finite fixture")
		os.Exit(1)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		fmt.Fprintln(os.Stderr, "reference: extra JSON value")
		os.Exit(1)
	}
	cases := make([]any, 0, len(requests.Cases))
	seen := map[string]bool{}
	for _, c := range requests.Cases {
		if c.Label == "" || seen[c.Label] {
			panic("duplicate or empty case label")
		}
		seen[c.Label] = true
		value := expected(c)
		if value["state"] != c.WantState || value["definition_relation"] != c.WantDefinition {
			panic("independent literal anchor: " + c.Label)
		}
		cases = append(cases, map[string]any{"inputs": map[string]any{entry + ".input0": c.Origin, entry + ".input1": c.Collection, entry + ".input2": c.Binding, entry + ".input3": c.Availability}, "expected": map[string]any{entry: value}})
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
