// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../../LICENSE.
// Independent finite oracle: page objects, name maps, and admission obligations.
// This program neither imports generated Gooo nor performs a registry update.
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

const entry = "gooo/tool-origin/catalog-refresh:DecideCatalogRefresh"
const definitionProfile = "gooo/mcp-tool-definition-json/v1"
const validatorProfile = "gooo/mcp-tool-shape/v1"

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
type Evidence struct {
	Attempt     string   `json:"attempt_id"`
	Candidate   string   `json:"candidate_id"`
	Collector   string   `json:"collector_state"`
	Binding     string   `json:"binding_state"`
	Validator   string   `json:"validator_profile"`
	Definitions []string `json:"row_definition_ids"`
	Outcomes    []string `json:"row_validation_states"`
}
type State struct {
	ExpectedRegistry     string `json:"expected_registry_version"`
	CurrentRegistry      string `json:"current_registry_version"`
	ExpectedScope        string `json:"expected_scope_epoch"`
	CurrentScope         string `json:"current_scope_epoch"`
	ExpectedInvalidation string `json:"expected_invalidation_epoch"`
	CurrentInvalidation  string `json:"current_invalidation_epoch"`
	ExpectedAttempt      string `json:"expected_latest_attempt"`
	CurrentAttempt       string `json:"current_latest_attempt"`
}
type Request struct {
	Label    string                     `json:"label"`
	Snapshot Facts                      `json:"snapshot"`
	Evidence Evidence                   `json:"evidence"`
	State    State                      `json:"state"`
	Anchors  map[string]json.RawMessage `json:"anchors"`
}
type Requests struct {
	Schema string    `json:"schema"`
	Cases  []Request `json:"cases"`
}
type cursor struct{ kind, value string }
type page struct{ request, next cursor }
type row struct {
	name, definition, profile, annotation string
	page                                  int64
}
type observation struct {
	valid                    bool
	coverage, coverageReason string
	page, row, duplicate     int
}

func bounded(s string) bool { return len(s) <= 1024 && !strings.ContainsRune(s, 0) }
func token(s string) bool   { return s != "" && bounded(s) }
func digest(s string) bool {
	tail, ok := strings.CutPrefix(s, "sha256:")
	if !ok || len(tail) != 64 || tail != strings.ToLower(tail) {
		return false
	}
	_, err := hex.DecodeString(tail)
	return err == nil
}
func inspect(f Facts) observation {
	o := observation{true, "COMPLETE", "OBSERVED_PAGE_CHAIN_COMPLETE", -1, -1, -1}
	invalid := func(reason string, p, r int) observation {
		o.valid = false
		o.coverage = "INVALID"
		o.coverageReason = reason
		o.page = p
		o.row = r
		return o
	}
	np, nr := len(f.RequestKinds), len(f.Names)
	shape := []struct {
		bad    bool
		reason string
	}{
		{np > 16 || nr > 256, "SNAPSHOT_LIMIT"},
		{len(f.RequestValues) != np || len(f.NextKinds) != np || len(f.NextValues) != np, "PAGE_COLUMN_LENGTH"},
		{len(f.Definitions) != nr || len(f.Profiles) != nr || len(f.Annotations) != nr || len(f.RowPages) != nr, "ROW_COLUMN_LENGTH"},
	}
	for _, check := range shape {
		if check.bad {
			return invalid(check.reason, -1, -1)
		}
	}
	pages := make([]page, np)
	allowedRequest := map[string]bool{"ABSENT": true, "STRING": true}
	allowedNext := map[string]bool{"ABSENT": true, "NULL": true, "STRING": true}
	for i := range pages {
		pages[i] = page{cursor{f.RequestKinds[i], f.RequestValues[i]}, cursor{f.NextKinds[i], f.NextValues[i]}}
		p := pages[i]
		if !allowedRequest[p.request.kind] || !allowedNext[p.next.kind] || !bounded(p.request.value) || !bounded(p.next.value) || p.request.kind != "STRING" && p.request.value != "" || p.next.kind != "STRING" && p.next.value != "" {
			return invalid("INVALID_CURSOR_TAG_OR_VALUE", i, -1)
		}
	}
	rows := make([]row, nr)
	for i := range rows {
		rows[i] = row{f.Names[i], f.Definitions[i], f.Profiles[i], f.Annotations[i], f.RowPages[i]}
		r := rows[i]
		if !token(r.name) || !token(r.profile) || !digest(r.definition) || !digest(r.annotation) || r.page < 0 || r.page >= int64(np) || i > 0 && r.page < rows[i-1].page {
			return invalid("INVALID_ROW_FACT", -1, i)
		}
	}
	names := map[string]int{}
	for i, r := range rows {
		if _, ok := names[r.name]; ok {
			if o.duplicate < 0 {
				o.duplicate = i
				o.row = i
			}
		} else {
			names[r.name] = i
		}
	}
	incomplete := func(reason string, p int) observation {
		o.coverage = "INCOMPLETE"
		o.coverageReason = reason
		o.page = p
		return o
	}
	if len(pages) == 0 {
		return incomplete("NO_SAVED_PAGES", -1)
	}
	// A page-chain transition is an obligation on adjacent page objects.
	transitions := []struct {
		bad    bool
		reason string
		page   int
	}{{pages[0].request.kind != "ABSENT", "MISSING_INITIAL_PAGE", 0}}
	for i := 1; i < len(pages); i++ {
		previous, current := pages[i-1], pages[i]
		transitions = append(transitions,
			struct {
				bad    bool
				reason string
				page   int
			}{previous.next.kind != "STRING", "PAGE_AFTER_TERMINAL", i},
			struct {
				bad    bool
				reason string
				page   int
			}{current.request.kind != "STRING" || current.request.value != previous.next.value, "CURSOR_LINK_MISMATCH", i})
	}
	transitions = append(transitions, struct {
		bad    bool
		reason string
		page   int
	}{pages[len(pages)-1].next.kind == "STRING", "MISSING_TERMINAL_PAGE", len(pages) - 1})
	for _, transition := range transitions {
		if transition.bad {
			return incomplete(transition.reason, transition.page)
		}
	}
	return o
}

type refusal struct {
	reason string
	row    int
}

func candidate(f Facts, e Evidence, o observation) refusal {
	switch {
	case e.Validator != validatorProfile:
		return refusal{"UNSUPPORTED_VALIDATOR_PROFILE", -1}
	case !o.valid:
		return refusal{"INVALID_SNAPSHOT", -1}
	case o.coverage != "COMPLETE":
		return refusal{"INCOMPLETE_CATALOG", -1}
	case o.duplicate >= 0:
		return refusal{"DUPLICATE_NAME", o.duplicate}
	}
	// All obligations of a stage precede every obligation of the next stage.
	for i, p := range f.Profiles {
		if p != definitionProfile {
			return refusal{"UNSUPPORTED_DEFINITION_PROFILE", i}
		}
	}
	if len(e.Definitions) != len(f.Names) || len(e.Outcomes) != len(f.Names) {
		return refusal{"ROW_VALIDATION_LENGTH", -1}
	}
	for i, d := range e.Definitions {
		if d != f.Definitions[i] {
			return refusal{"ROW_VALIDATION_BINDING_MISMATCH", i}
		}
	}
	outcomes := map[string]string{"ACCEPTED": "", "REJECTED": "ROW_REJECTED", "UNKNOWN": "ROW_UNASSESSED"}
	for i, outcome := range e.Outcomes {
		reason, known := outcomes[outcome]
		if !known {
			reason = "INVALID_ROW_VALIDATION_STATE"
		}
		if reason != "" {
			return refusal{reason, i}
		}
	}
	return refusal{"", -1}
}
func expected(c Request) map[string]any {
	f, e, s := c.Snapshot, c.Evidence, c.State
	o := inspect(f)
	r := map[string]any{
		"state": "RETAIN_STORED_CATALOG", "reason": "COMPLETE_CANDIDATE",
		"candidate_id": e.Candidate, "attempt_id": e.Attempt,
		"expected_registry_version": s.ExpectedRegistry, "expected_scope_epoch": s.ExpectedScope,
		"expected_invalidation_epoch": s.ExpectedInvalidation, "expected_latest_attempt": s.ExpectedAttempt,
		"candidate_page_count": len(f.RequestKinds), "candidate_row_count": len(f.Names),
		"coverage": o.coverage, "coverage_reason": o.coverageReason, "problem_page_index": o.page, "problem_row_index": o.row,
		"decision_scope": "WHOLE_CANDIDATE_LOCAL_REPLACEMENT_PRECHECK", "execution_authorized": false, "live_effect_performed": false, "remote_atomicity": "UNKNOWN",
	}
	if len(r) != 18 {
		panic("decision ABI")
	}
	collectors := map[string]string{"SUCCESS": "", "ERROR": "COLLECTOR_ERROR", "TIMEOUT": "COLLECTOR_TIMEOUT", "CANCELLED": "COLLECTOR_CANCELLED"}
	collectorReason, validCollector := collectors[e.Collector]
	validEvidence := token(e.Attempt) && digest(e.Candidate) && token(e.Validator) && validCollector && (e.Binding == "BOUND" || e.Binding == "UNBOUND")
	validState := true
	for _, v := range []string{s.ExpectedRegistry, s.CurrentRegistry, s.ExpectedScope, s.CurrentScope, s.ExpectedInvalidation, s.CurrentInvalidation, s.ExpectedAttempt, s.CurrentAttempt} {
		validState = validState && token(v)
	}
	stage := candidate(f, e, o)
	if o.row < 0 && stage.row >= 0 {
		r["problem_row_index"] = stage.row
	}
	// Ordered guard table is the independent admission model. False guards pass.
	guards := []struct {
		bad    bool
		reason string
	}{
		{!validEvidence, "INVALID_REFRESH_EVIDENCE"},
		{e.Binding != "BOUND", "UNBOUND_CANDIDATE"},
		{!validState, "INVALID_REFRESH_STATE"},
		{s.ExpectedAttempt != e.Attempt, "ATTEMPT_BINDING_MISMATCH"},
		{collectorReason != "", collectorReason},
		{stage.reason != "", stage.reason},
		{s.ExpectedRegistry != s.CurrentRegistry, "REGISTRY_CHANGED"},
		{s.ExpectedScope != s.CurrentScope, "SCOPE_CHANGED"},
		{s.ExpectedInvalidation != s.CurrentInvalidation, "INVALIDATED"},
		{s.ExpectedAttempt != s.CurrentAttempt, "SUPERSEDED_ATTEMPT"},
	}
	for _, guard := range guards {
		if guard.bad {
			r["reason"] = guard.reason
			return r
		}
	}
	r["state"] = "CANDIDATE_ELIGIBLE"
	return r
}
func render(raw []byte) ([]byte, error) {
	if len(raw) > 4<<20 {
		return nil, fmt.Errorf("input cap")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var requests Requests
	if err := decoder.Decode(&requests); err != nil {
		return nil, err
	}
	if requests.Schema != "gooo/catalog-refresh-reference-inputs/v1" || len(requests.Cases) == 0 || len(requests.Cases) > 256 {
		return nil, fmt.Errorf("invalid finite fixture")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("extra JSON value")
	}
	cases := make([]any, 0, len(requests.Cases))
	seen := map[string]bool{}
	for _, c := range requests.Cases {
		if c.Label == "" || seen[c.Label] || len(c.Anchors) < 4 {
			return nil, fmt.Errorf("invalid fixture label/anchors")
		}
		seen[c.Label] = true
		value := expected(c)
		for key, anchor := range c.Anchors {
			actual, exists := value[key]
			encoded, err := json.Marshal(actual)
			if err != nil || !exists || !bytes.Equal(encoded, bytes.TrimSpace(anchor)) {
				return nil, fmt.Errorf("literal anchor %s:%s: got %s want %s", c.Label, key, encoded, anchor)
			}
		}
		cases = append(cases, map[string]any{"inputs": map[string]any{entry + ".input0": c.Snapshot, entry + ".input1": c.Evidence, entry + ".input2": c.State}, "expected": map[string]any{entry: value}})
	}
	out, err := json.MarshalIndent(map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": cases}, "", "  ")
	return append(out, '\n'), err
}
func main() {
	input := flag.String("input", "", "finite fixture JSON (stdin if omitted)")
	flag.Parse()
	var source io.Reader = os.Stdin
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		source = f
	}
	raw, err := io.ReadAll(io.LimitReader(source, (4<<20)+1))
	if err == nil {
		raw, err = render(raw)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "reference:", err)
		os.Exit(1)
	}
	if n, err := os.Stdout.Write(raw); err != nil || n != len(raw) {
		os.Exit(1)
	}
}
