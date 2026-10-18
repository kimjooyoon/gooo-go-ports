// Copyright 2026 Gooo Go Ports contributors.
// Original contract-derived reference, BSD-3-Clause; see repository LICENSE.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"unicode/utf8"
)

type Input struct {
	Header                string `json:"header"`
	HeaderPresent         bool   `json:"header_present"`
	ElapsedSeconds        int64  `json:"elapsed_seconds"`
	MaxWaitSeconds        int64  `json:"max_wait_seconds"`
	RequestBindingCurrent bool   `json:"request_binding_current"`
	GuidanceApplicable    bool   `json:"guidance_applicable"`
}

type Result struct {
	Input
	ParsedValue         int64  `json:"parsed_value"`
	ParseErrorKind      int64  `json:"parse_error_kind"`
	DeltaSecondsGrammar bool   `json:"delta_seconds_grammar"`
	DelayInPolicy       bool   `json:"delay_in_policy"`
	CooldownActive      bool   `json:"cooldown_active"`
	RemainingSeconds    int64  `json:"remaining_seconds"`
	State               string `json:"state"`
	Reason              string `json:"reason"`
}

var digits = regexp.MustCompile(`^[0-9]+$`)

func assess(in Input) Result {
	value, err := strconv.ParseInt(in.Header, 10, 64)
	kind := int64(0)
	if errors.Is(err, strconv.ErrSyntax) {
		kind = 1
	} else if errors.Is(err, strconv.ErrRange) {
		kind = 2
	} else if err != nil {
		panic("unexpected standard library parse error")
	}
	valid := in.ElapsedSeconds >= 0 && in.MaxWaitSeconds >= 0 && in.MaxWaitSeconds <= 86400
	grammar := digits.MatchString(in.Header)
	out := Result{Input: in, ParsedValue: value, ParseErrorKind: kind,
		DeltaSecondsGrammar: grammar, DelayInPolicy: valid && err == nil && grammar && value >= 0 && value <= in.MaxWaitSeconds,
		RemainingSeconds: -1, State: "UNKNOWN", Reason: "GUIDANCE_UNCONFIRMED"}
	switch {
	case !valid:
		out.State, out.Reason = "DOMAIN_REFUSED", "INPUT_DOMAIN_INVALID"
	case !in.RequestBindingCurrent:
		out.Reason = "REQUEST_BINDING_NOT_CURRENT"
	case !in.GuidanceApplicable:
		out.Reason = "GUIDANCE_NOT_APPLICABLE"
	case !in.HeaderPresent:
		out.Reason = "HEADER_NOT_CONFIRMED"
	case kind == 1:
		out.Reason = "DELAY_SYNTAX_INVALID"
	case kind == 2:
		out.Reason = "DELAY_RANGE_ERROR"
	case err != nil:
		out.Reason = "PARSER_CONTRACT_UNSUPPORTED"
	case !grammar:
		out.Reason = "DELTA_SECONDS_GRAMMAR"
	case value < 0 || value > in.MaxWaitSeconds:
		out.Reason = "WAIT_POLICY_EXCEEDED"
	case in.ElapsedSeconds < value:
		out.State, out.Reason = "WAIT", "COOLDOWN_ACTIVE"
		out.CooldownActive, out.RemainingSeconds = true, value-in.ElapsedSeconds
	default:
		out.State, out.Reason = "READY_TO_REASSESS", "COOLDOWN_ELAPSED"
		out.RemainingSeconds = 0
	}
	return out
}

type Witness struct {
	Name  string `json:"name"`
	Input Input  `json:"input"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: reference WITNESSES.json")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var witnesses []Witness
	if err := decoder.Decode(&witnesses); err != nil {
		panic(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		panic("expected one JSON document")
	}
	rows := make([]any, 0, len(witnesses))
	for index, witness := range witnesses {
		if !utf8.ValidString(witness.Input.Header) || len(witness.Input.Header) > 64 {
			panic("witness header outside declared transport domain")
		}
		rows = append(rows, struct {
			Index  int    `json:"index"`
			Name   string `json:"name"`
			Input  Input  `json:"input"`
			Result Result `json:"result"`
		}{index, witness.Name, witness.Input, assess(witness.Input)})
	}
	if err := json.NewEncoder(os.Stdout).Encode(struct {
		Schema string `json:"schema"`
		Rows   []any  `json:"rows"`
	}{"gooo/retry-after-facts-independent-reference/v1", rows}); err != nil {
		panic(err)
	}
}
