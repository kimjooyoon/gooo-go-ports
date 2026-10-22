package grantselection

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	many "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
)

func sample() Input {
	return Input{Before: "*", RequiredRequests: []Request{{Type: "mcp", Name: "weather"}, {Type: "mcp", Name: "calculator"}},
		Candidates: []Candidate{{"weather-only", "mcp://weather", 0}, {"all-mcp", "mcp://*", 10}}}
}

func TestSelectPreservesAllRequiredRequestsAndCompleteProofs(t *testing.T) {
	input := sample()
	r, err := Assess(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Facts, input) || r.Selection.State != "PROPOSED" || r.Selection.SelectedCandidateID != "all-mcp" || r.Selection.SelectedIndex != 1 || r.Selection.SelectedPriority != 10 || r.Selection.ExecutionAuthorized {
		t.Fatalf("unexpected choice: %+v", r.Selection)
	}
	if len(r.Assessments) != 2 || r.Assessments[0].Proof.Decision.Status != "REFUSED" || r.Assessments[0].Proof.Decision.FirstCounterexample != 1 || r.Assessments[1].Proof.Decision.Status != "ELIGIBLE" {
		t.Fatalf("complete assessments missing: %+v", r.Assessments)
	}
	for i, a := range r.Assessments {
		want := many.Assess(many.Input{Before: input.Before, After: input.Candidates[i].After, RequiredRequests: input.RequiredRequests})
		if !reflect.DeepEqual(a.Proof, want) || a.CandidateID != input.Candidates[i].ID {
			t.Fatalf("candidate%d proof/facts changed", i)
		}
		raw, err := json.Marshal(a.Proof)
		if err != nil {
			t.Fatal(err)
		}
		if a.ProofSHA256 != digest(raw) {
			t.Fatal("proof digest identifies different bytes")
		}
	}
	if r.ContextID != r.Selection.ContextID || r.Selection.SelectedAssessmentID != r.Assessments[1].ProofSHA256 {
		t.Fatal("selection lost context or original proof identity")
	}
}

func TestSourceOwnsCandidateAdmissionBeforeFanOut(t *testing.T) {
	for _, name := range []string{"empty", "one", "33", "duplicate", "negative", "long-id"} {
		t.Run(name, func(t *testing.T) {
			input := sample()
			switch name {
			case "empty":
				input.Candidates = []Candidate{}
			case "one":
				input.Candidates = input.Candidates[:1]
			case "33":
				input.Candidates = make([]Candidate, 33)
				for i := range input.Candidates {
					input.Candidates[i] = Candidate{fmt.Sprint(i), "mcp://*", 0}
				}
			case "duplicate":
				input.Candidates[1].ID = input.Candidates[0].ID
			case "negative":
				input.Candidates[1].Preference = -1
			case "long-id":
				input.Candidates[1].ID = strings.Repeat("한", 43)
			}
			calls := 0
			r, err := assess(input, func(in many.Input) many.Result { calls++; return many.Assess(in) })
			if err != nil || calls != 0 || r.Selection.State != "INVALID_INPUT" || r.Selection.SelectedIndex != -1 || len(r.Assessments) != 0 || r.Selection.ExecutionAuthorized {
				t.Fatalf("source admission bypassed: calls%d result%+v error%v", calls, r.Selection, err)
			}
			if !reflect.DeepEqual(r.Facts, input) {
				t.Fatal("invalid input was repaired")
			}
		})
	}
}

func TestSuppliedPreferenceTieAndInt64Boundary(t *testing.T) {
	input := sample()
	input.Candidates = []Candidate{{"first", "mcp://*", math.MaxInt64}, {"second", "mcp://*", math.MaxInt64}}
	r, err := Assess(input)
	if err != nil || r.Selection.SelectedCandidateID != "first" || r.Selection.SelectedPriority != math.MaxInt64 {
		t.Fatalf("stable tie changed: %+v %v", r.Selection, err)
	}
	input.Candidates[1].Preference = 0
	r, err = Assess(input)
	if err != nil || r.Selection.SelectedCandidateID != "second" || r.Selection.SelectedPriority != 0 {
		t.Fatalf("int64 preference overflowed: %+v %v", r.Selection, err)
	}
}

func TestAllRefusedAndInvalidDomainRemainOriginalDecisions(t *testing.T) {
	input := sample()
	input.Candidates[1].After = "http://*"
	r, err := Assess(input)
	if err != nil || r.Selection.State != "REFUSED" || r.Selection.EligibleCount != 0 || r.Selection.SelectedIndex != -1 || len(r.Assessments) != 2 {
		t.Fatalf("unexpected no-choice: %+v %v", r, err)
	}
	input.Candidates[0].After = "mcp://weather/path"
	input.Candidates[1].After = "mcp://*"
	r, err = Assess(input)
	if err != nil || r.Selection.SelectedCandidateID != "all-mcp" || r.Assessments[0].Proof.Decision.Status != "INVALID_INPUT" || r.Assessments[0].Proof.Facts.After != "mcp://weather/path" {
		t.Fatalf("unsupported pattern was repaired or lost: %+v %v", r, err)
	}
}

func TestLateBindingFailureAndPanicReturnNoPartialResult(t *testing.T) {
	for _, name := range []string{"facts", "generation", "status", "panic"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			r, err := assess(sample(), func(input many.Input) many.Result {
				calls++
				proof := many.Assess(input)
				if calls == 2 {
					switch name {
					case "facts":
						proof.Facts.After = "different"
					case "generation":
						proof.Bindings.GenerationSHA256 = "sha256:wrong"
					case "status":
						proof.Decision.Status = ""
					case "panic":
						panic("controlled dependency failure")
					}
				}
				return proof
			})
			var problem *Problem
			if calls != 2 || !errors.As(err, &problem) || problem.State != "UNKNOWN" || !reflect.DeepEqual(r, Result{}) {
				t.Fatalf("late failure leaked a decision: calls%d result%+v error%v", calls, r, err)
			}
		})
	}
}

func TestSnapshotsAreIndependentOfCallerAndPriorResultMutation(t *testing.T) {
	input := sample()
	original := snapshot(input)
	calls := 0
	r, err := assess(input, func(in many.Input) many.Result {
		calls++
		if calls == 1 {
			input.RequiredRequests[0].Name = "changed"
			input.Candidates[1].After = "changed"
		}
		return many.Assess(in)
	})
	if err != nil || !reflect.DeepEqual(r.Facts, original) || r.Selection.SelectedCandidateID != "all-mcp" {
		t.Fatalf("caller reuse changed snapshot: %+v %v", r, err)
	}
	r.Bindings.Selector.Sources["ports/service-policy/candidate_selection.gooo"] = "changed"
	r.Assessments[0].Proof.Bindings.Sources["new"] = "changed"
	again, err := Assess(original)
	if err != nil || again.Bindings.Selector.Sources["ports/service-policy/candidate_selection.gooo"] == "changed" || again.Assessments[0].Proof.Bindings.Sources["new"] != "" {
		t.Fatal("returned maps alias future assessments")
	}
}

func TestPolicyRequestBoundaryIsNotFilteredByGo(t *testing.T) {
	input := sample()
	input.RequiredRequests = make([]Request, 257)
	for i := range input.RequiredRequests {
		input.RequiredRequests[i] = Request{Type: "mcp", Name: "weather"}
	}
	r, err := Assess(input)
	if err != nil || r.Selection.State != "REFUSED" || len(r.Assessments) != 2 {
		t.Fatalf("257 was lost: %+v %v", r.Selection, err)
	}
	for _, a := range r.Assessments {
		if a.Proof.Decision.Status != "INVALID_INPUT" || a.Proof.Decision.Reason != "REQUIRED_REQUEST_LIMIT_EXCEEDED" || a.Proof.Decision.RequestCount != 257 || len(a.Proof.Facts.RequiredNames) != 257 {
			t.Fatal("Gooo policy limit was replaced by a Go filter")
		}
	}
}

func TestMaximum32CandidatesAnd256Requirements(t *testing.T) {
	input := sample()
	input.RequiredRequests = make([]Request, 256)
	input.Candidates = make([]Candidate, 32)
	for i := range input.RequiredRequests {
		input.RequiredRequests[i] = Request{Type: "mcp", Name: "weather"}
	}
	for i := range input.Candidates {
		input.Candidates[i] = Candidate{fmt.Sprintf("candidate-%02d", i), "mcp://*", int64(31 - i)}
	}
	r, err := Assess(input)
	if err != nil || len(r.Assessments) != 32 || r.Selection.SelectedIndex != 31 || r.Selection.EligibleCount != 32 || r.Selection.ExecutionAuthorized {
		t.Fatalf("bounded work failed: %+v %v", r.Selection, err)
	}
	for _, a := range r.Assessments {
		if a.Proof.Decision.CheckedCount != 256 || len(a.Proof.Facts.RequiredNames) != 256 {
			t.Fatal("duplicate positions were shortened")
		}
	}
}

func TestInvalidTextIsTransportFailureBeforeEvaluation(t *testing.T) {
	for _, text := range []string{string([]byte{0xff}), strings.Repeat("x", 1025)} {
		input := sample()
		input.Before = text
		calls := 0
		r, err := assess(input, func(in many.Input) many.Result { calls++; return many.Assess(in) })
		var problem *Problem
		if !errors.As(err, &problem) || problem.State != "INVALID_TRANSPORT" || calls != 0 || !reflect.DeepEqual(r, Result{}) {
			t.Fatalf("transport failure became a decision: %+v %v", r, err)
		}
	}
}

func TestConcurrentAssessmentsHaveNoSharedMutableState(t *testing.T) {
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			r, err := Assess(sample())
			if err != nil || r.Selection.SelectedCandidateID != "all-mcp" {
				t.Errorf("concurrent result changed: %+v %v", r.Selection, err)
			}
		}()
	}
	group.Wait()
}
