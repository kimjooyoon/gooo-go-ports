// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
// Independent expected-value construction for the original binding policy.
// Parent observations are immutable literal fixtures, not a second origin oracle.
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

const entry = "gooo/tool-origin/definition-binding:AssessDefinitionBinding"
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
type Request struct {
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
	b, p := c.Binding, c.Parent
	before, after := length(c.Origin, "before_aliases"), length(c.Origin, "after_aliases")
	invalid := b.ContextID == "" || b.AliasContextID != text(c.Origin, "context_id") || b.CollectionContextID != text(c.Collection, "context_id")
	for _, pair := range [][2]int{{len(b.BeforeDefinitions), before}, {len(b.BeforeProfiles), before}, {len(b.BeforeCapabilities), before}, {len(b.AfterDefinitions), after}, {len(b.AfterProfiles), after}, {len(b.AfterCapabilities), after}} {
		invalid = invalid || pair[0] != pair[1]
	}
	r := map[string]any{
		"context_id":            b.ContextID,
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
		r["policy_relation"] = relation(b.BeforePolicy, b.AfterPolicy)
	}
	// Declared priority table, independent of the Gooo source's branch layout.
	// A known difference must not disappear behind an unrelated missing field.
	type resolution struct {
		applies               bool
		state, reason, action string
	}
	choices := []resolution{
		{invalid, "INVALID_INPUT", "INVALID_OR_UNBOUND_DEFINITION_COLUMNS", "FIX_BOUND_INPUT"},
		{text(p, "origin_state") != "UNCHANGED", "ORIGIN_NOT_UNCHANGED", "ORIGIN_BINDING_REQUIRES_REEVALUATION", "REEVALUATE_ORIGIN"},
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
	if err := dec.Decode(&requests); err != nil || requests.Schema != "gooo/definition-binding-reference-inputs/v1" || len(requests.Cases) == 0 || len(requests.Cases) > 128 {
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
		cases = append(cases, map[string]any{"inputs": map[string]any{entry + ".input0": c.Origin, entry + ".input1": c.Collection, entry + ".input2": c.Binding}, "expected": map[string]any{entry: value}})
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
