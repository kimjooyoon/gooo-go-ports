// Copyright (c) 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
// Go transports facts and binds internally evaluated proofs. Gooo owns choices.
package grantselection

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection/internal/selector"
	many "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
)

type Request = many.Request

type Candidate struct {
	ID         string `json:"id"`
	After      string `json:"after"`
	Preference int64  `json:"preference"`
}

// Input exposes proposals and supplied requirements, never proof-status flags.
// Lower nonnegative Preference wins among eligible candidates; ties keep order.
type Input struct {
	Before           string      `json:"before"`
	RequiredRequests []Request   `json:"required_requests"`
	Candidates       []Candidate `json:"candidates"`
}

type Choice = selector.Decision

type Assessment struct {
	CandidateID        string      `json:"candidate_id"`
	SuppliedPreference int64       `json:"supplied_preference"`
	ProofSHA256        string      `json:"proof_sha256"`
	Proof              many.Result `json:"proof"`
}

type KernelBindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	GoVersion         string            `json:"go_version"`
	GenerationSHA256  string            `json:"generation_sha256"`
	Sources           map[string]string `json:"sources"`
	GeneratedAssets   map[string]string `json:"generated_assets"`
}

type Bindings struct {
	Selector KernelBindings `json:"selector"`
	Policy   many.Bindings  `json:"policy"`
}

type Result struct {
	Facts       Input        `json:"facts"`
	ContextID   string       `json:"context_id"`
	Assessments []Assessment `json:"assessments"`
	Selection   Choice       `json:"selection"`
	Bindings    Bindings     `json:"bindings"`
}

// Problem is a host transport/binding failure, not a fabricated Gooo decision.
type Problem struct {
	State string `json:"state"`
	Stage string `json:"stage"`
	Code  string `json:"code"`
	Index int    `json:"index"`
	Field string `json:"field,omitempty"`
}

func (p *Problem) Error() string { return p.State + ": " + p.Stage + ": " + p.Code }

//go:embed kernel-generation.json
var kernelGeneration []byte

//go:embed policy-generation.json
var policyGeneration []byte

func digest(raw []byte) string {
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}

func sourceBindings() (Bindings, error) {
	var kernel struct {
		Compiler string            `json:"compiler_source_sha"`
		Go       string            `json:"go_version"`
		Sources  map[string]string `json:"source"`
		Assets   map[string]string `json:"generated_assets"`
	}
	var policy struct {
		Compiler string            `json:"compiler_source_sha"`
		Sources  map[string]string `json:"public_sources"`
		Assets   map[string]string `json:"assets"`
	}
	if json.Unmarshal(kernelGeneration, &kernel) != nil || json.Unmarshal(policyGeneration, &policy) != nil {
		return Bindings{}, &Problem{State: "UNKNOWN", Stage: "bind_generation", Code: "GENERATION_UNREADABLE", Index: -1}
	}
	const pin = "205896a413e5dea92366309c35ae85e542d055f6"
	if kernel.Compiler != pin || kernel.Go != "go1.27.3" || policy.Compiler != pin || len(kernel.Sources) != 1 || len(kernel.Assets) != 3 || len(policy.Sources) != 5 || len(policy.Assets) != 30 {
		return Bindings{}, &Problem{State: "UNKNOWN", Stage: "bind_generation", Code: "GENERATION_PROFILE_MISMATCH", Index: -1}
	}
	return Bindings{
		Selector: KernelBindings{kernel.Compiler, kernel.Go, digest(kernelGeneration), kernel.Sources, kernel.Assets},
		Policy:   many.Bindings{CompilerSourceSHA: policy.Compiler, GenerationSHA256: digest(policyGeneration), Sources: policy.Sources, Assets: policy.Assets},
	}, nil
}

func snapshot(input Input) Input {
	out := input
	if input.RequiredRequests != nil {
		out.RequiredRequests = append([]Request{}, input.RequiredRequests...)
	}
	if input.Candidates != nil {
		out.Candidates = append([]Candidate{}, input.Candidates...)
	}
	return out
}

func transport(input Input) error {
	check := func(value, field string) error {
		code := ""
		if !utf8.ValidString(value) {
			code = "INVALID_UTF8"
		} else if len(value) > 1024 {
			code = "TEXT_LIMIT_EXCEEDED"
		}
		if code != "" {
			return &Problem{State: "INVALID_TRANSPORT", Stage: "admit_text", Code: code, Index: -1, Field: field}
		}
		return nil
	}
	if err := check(input.Before, "before"); err != nil {
		return err
	}
	for i, r := range input.RequiredRequests {
		if err := check(r.Type, fmt.Sprintf("required_requests[%d].type", i)); err != nil {
			return err
		}
		if err := check(r.Name, fmt.Sprintf("required_requests[%d].name", i)); err != nil {
			return err
		}
	}
	for i, c := range input.Candidates {
		if err := check(c.ID, fmt.Sprintf("candidates[%d].id", i)); err != nil {
			return err
		}
		if err := check(c.After, fmt.Sprintf("candidates[%d].after", i)); err != nil {
			return err
		}
	}
	return nil
}

// Assess evaluates every admitted proposal through the existing generated Gooo
// policy, then calls the new generated Gooo selector. It applies no change.
func Assess(input Input) (Result, error) { return assess(input, many.Assess) }

// The evaluation seam is private: external callers cannot inject proof flags.
func assess(input Input, evaluate func(many.Input) many.Result) (result Result, err error) {
	defer func() {
		if recover() != nil {
			result = Result{}
			err = &Problem{State: "UNKNOWN", Stage: "evaluate_policy", Code: "DEPENDENCY_PANIC", Index: -1}
		}
	}()
	facts := snapshot(input)
	if err := transport(facts); err != nil {
		return Result{}, err
	}
	bindings, err := sourceBindings()
	if err != nil {
		return Result{}, err
	}
	contextBytes, err := json.Marshal(struct {
		Before   string    `json:"before"`
		Required []Request `json:"required_requests"`
		Policy   string    `json:"policy_generation_sha256"`
		Selector string    `json:"selector_generation_sha256"`
	}{facts.Before, facts.RequiredRequests, bindings.Policy.GenerationSHA256, bindings.Selector.GenerationSHA256})
	if err != nil {
		return Result{}, err
	}
	columns := selector.Input{Phase: "ADMIT", ContextID: digest(contextBytes),
		CandidateIDs: make([]string, len(facts.Candidates)), Priorities: make([]int64, len(facts.Candidates))}
	for i, c := range facts.Candidates {
		columns.CandidateIDs[i], columns.Priorities[i] = c.ID, c.Preference
	}
	out := Result{Facts: facts, ContextID: columns.ContextID, Assessments: []Assessment{}, Bindings: bindings}
	out.Selection = selector.Evaluate(columns)
	if out.Selection.State != "ADMITTED" {
		return out, nil
	}
	if !reflect.DeepEqual(many.SourceBindings(), bindings.Policy) {
		return Result{}, &Problem{State: "UNKNOWN", Stage: "bind_generation", Code: "POLICY_BINDINGS_MISMATCH", Index: -1}
	}
	columns.Phase = "SELECT"
	for i, c := range facts.Candidates {
		request := many.Input{Before: facts.Before, After: c.After, RequiredRequests: facts.RequiredRequests}
		proof := evaluate(request)
		if !reflect.DeepEqual(proof.Facts, request.Columns()) || !reflect.DeepEqual(proof.Bindings, bindings.Policy) {
			return Result{}, &Problem{State: "UNKNOWN", Stage: "bind_assessment", Code: "ASSESSMENT_BINDING_MISMATCH", Index: i}
		}
		switch proof.Decision.Status {
		case "ELIGIBLE", "REFUSED", "INVALID_INPUT":
		default:
			return Result{}, &Problem{State: "UNKNOWN", Stage: "bind_assessment", Code: "ASSESSMENT_RESULT_INVALID", Index: i}
		}
		encoded, err := json.Marshal(proof)
		if err != nil {
			return Result{}, err
		}
		identity := digest(encoded)
		out.Assessments = append(out.Assessments, Assessment{c.ID, c.Preference, identity, proof})
		columns.AssessmentIDs = append(columns.AssessmentIDs, identity)
		columns.ContextIDs = append(columns.ContextIDs, columns.ContextID)
		columns.Statuses = append(columns.Statuses, proof.Decision.Status)
		columns.BindingsVerified = append(columns.BindingsVerified, true)
	}
	out.Selection = selector.Evaluate(columns)
	return out, nil
}
