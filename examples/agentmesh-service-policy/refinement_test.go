package servicepolicy

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Fixtures follow the reviewed six-family language matrix. verify.py separately
// executes the exact pinned upstream matcher on both sides of every real witness.
func TestSinglePatternRefinementMatrixAndWitnesses(t *testing.T) {
	raw, err := os.ReadFile("refinement-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string `json:"name"`
		RefinementFacts
		BeforeFamily string             `json:"before_family"`
		AfterFamily  string             `json:"after_family"`
		Coverage     []string           `json:"coverage"`
		Want         RefinementDecision `json:"want"`
	}
	if json.Unmarshal(raw, &cases) != nil || len(cases) != 101 {
		t.Fatal("refinement corpus count")
	}
	cells := map[string]bool{}
	rules := map[string]bool{}
	counts := map[string]int{}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			p, err := EvaluateRefinement(c.RefinementFacts)
			if err != nil || p.Facts != c.RefinementFacts || p.Decision != c.Want {
				t.Fatalf("refinement output differs: %+v %v", p, err)
			}
			if err := validateRefinementDecision(p.Decision); err != nil {
				t.Fatal(err)
			}
			if len(p.Bindings.Assets) == 0 || p.Bindings.GenerationOrigin != "fresh_pinned_generation" {
				t.Fatal("missing generation evidence")
			}
			counts[p.Decision.Status]++
			if p.Decision.SupportedSubset {
				rules[p.Decision.RuleID] = true
			}
			for _, tag := range c.Coverage {
				if tag == "matrix" {
					cells[c.BeforeFamily+"/"+c.AfterFamily] = true
				}
			}
		})
	}
	if len(cells) != 36 || len(rules) != 7 || counts["SUPPORTED_SUBSET"] != 38 || counts["WIDENING_WITNESS"] != 63 || counts["UNPROVEN"] != 0 {
		t.Fatalf("coverage changed: cells=%d rules=%d counts=%v", len(cells), len(rules), counts)
	}
}

func TestRefinementValidatesBothPatternsBeforeShortcuts(t *testing.T) {
	invalid := []string{"mcp://a/path", "mcp://a?q=x", "mcp://a#x", "mcp:a", "mcp://a*b", "mcp://*.*", "mcp://*.a.*", "mcp://a..b", "mcp://.a", "mcp://a.", "mcp://user@a", "mcp://a:443", "mcp://", "mcp://é", "mcp://a\x00b", "a://" + strings.Repeat("x", 125), strings.Repeat("a", 125) + "://*"}
	for _, bad := range invalid {
		for _, good := range []string{"", "*", "mcp://a.*"} {
			for _, f := range []RefinementFacts{{Before: bad, After: good}, {Before: good, After: bad}, {Before: bad, After: bad}} {
				if err := ValidateRefinement(f); err == nil {
					t.Fatalf("invalid refinement input accepted: %#v", f)
				}
				if p, err := EvaluateRefinement(f); err == nil || p.Decision.SupportedSubset || p.Decision.HasWitness {
					t.Fatalf("shortcut bypassed invalid other pattern: %#v %+v %v", f, p, err)
				}
			}
		}
	}
	for _, value := range []string{"a://" + strings.Repeat("x", 124), "a://" + strings.Repeat("x", 122) + ".*", "a://*." + strings.Repeat("x", 122), strings.Repeat("a", 124) + "://*"} {
		if len(value) != 128 {
			t.Fatal("wrong boundary fixture")
		}
		if _, err := EvaluateRefinement(RefinementFacts{Before: value, After: value}); err != nil {
			t.Fatal("128-byte pattern refused", err)
		}
	}
}

func TestRefinementOutputContradictionsRefused(t *testing.T) {
	valid := []RefinementDecision{
		{Status: "SUPPORTED_SUBSET", RuleID: "CANDIDATE_EMPTY", SupportedSubset: true},
		{Status: "WIDENING_WITNESS", RuleID: "CHECKED_CANDIDATE_REQUEST", HasWitness: true, WitnessType: "mcp", WitnessName: "a", AfterMatch: true},
		{Status: "UNPROVEN"},
	}
	for _, d := range valid {
		if err := validateRefinementDecision(d); err != nil {
			t.Fatal("valid output refused", err)
		}
	}
	for _, rule := range []string{"CANDIDATE_EMPTY", "IDENTICAL_PATTERNS", "OLD_GLOBAL", "OLD_TYPED_ALL", "EXACT_CANDIDATE_ACCEPTED_BY_OLD", "PREFIX_HIERARCHY", "SUFFIX_HIERARCHY"} {
		d := valid[0]
		d.RuleID = rule
		if err := validateRefinementDecision(d); err != nil {
			t.Fatal(rule, err)
		}
	}
	for i, base := range valid {
		mutations := []struct {
			name   string
			change func(*RefinementDecision)
		}{
			{"status", func(d *RefinementDecision) { d.Status = "UNKNOWN" }},
			{"rule", func(d *RefinementDecision) { d.RuleID = "HOST_COMPUTED" }},
			{"supported_subset", func(d *RefinementDecision) { d.SupportedSubset = !d.SupportedSubset }},
			{"has_witness", func(d *RefinementDecision) { d.HasWitness = !d.HasWitness }},
			{"before_match", func(d *RefinementDecision) { d.BeforeMatch = true }},
			{"after_match", func(d *RefinementDecision) { d.AfterMatch = !d.AfterMatch }},
			{"witness_type", func(d *RefinementDecision) {
				if d.HasWitness {
					d.WitnessType = ""
				} else {
					d.WitnessType = "mcp"
				}
			}},
			{"witness_name", func(d *RefinementDecision) {
				if d.HasWitness {
					d.WitnessName = ""
				} else {
					d.WitnessName = "a"
				}
			}},
		}
		for _, m := range mutations {
			t.Run(base.Status+"/"+m.name, func(t *testing.T) {
				d := base
				m.change(&d)
				if err := validateRefinementDecision(d); err == nil {
					t.Fatalf("contradictory output accepted %d %+v", i, d)
				}
			})
		}
	}
	for _, value := range []string{"", "é", "a/b", "a\x00b", strings.Repeat("a", 129)} {
		for _, field := range []string{"type", "name"} {
			d := valid[1]
			if field == "type" {
				d.WitnessType = value
			} else {
				d.WitnessName = value
			}
			if err := validateRefinementDecision(d); err == nil {
				t.Fatalf("bad witness component accepted: %s", field)
			}
		}
	}
	d := valid[1]
	d.WitnessType = strings.Repeat("A", 128)
	d.WitnessName = strings.Repeat("n", 128)
	if err := validateRefinementDecision(d); err != nil {
		t.Fatal("independent 128-byte witness component bounds refused", err)
	}
}
