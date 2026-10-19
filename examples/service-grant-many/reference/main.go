// Independent reference only: never imported by the Gooo program.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	upstream "gooo.servicegrantmany.reference/upstream"
)

type Facts struct {
	Before string   `json:"before"`
	After  string   `json:"after"`
	Types  []string `json:"required_types"`
	Names  []string `json:"required_names"`
}
type Decision struct {
	Status       string `json:"status"`
	Reason       string `json:"reason"`
	InvalidField string `json:"invalid_field"`
	InvalidIndex int    `json:"invalid_index"`
	First        int    `json:"first_counterexample"`
	Count        int    `json:"request_count"`
	Checked      int    `json:"checked_count"`
	All          bool   `json:"all_preserved"`
	Subset       bool   `json:"supported_subset"`
	Rule         string `json:"subset_rule_id"`
	FirstBefore  bool   `json:"counterexample_before_match"`
	FirstAfter   bool   `json:"counterexample_after_match"`
}
type Case struct {
	Name     string   `json:"name"`
	Facts    Facts    `json:"facts"`
	Decision Decision `json:"decision"`
}

var typeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
var nameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)

func typ(s string) bool  { return len(s) <= 128 && (s == "" || typeRE.MatchString(s)) }
func name(s string) bool { return len(s) <= 128 && (s == "" || nameRE.MatchString(s)) }
func pattern(s string) bool {
	if len(s) > 128 {
		return false
	}
	if s == "" || s == "*" {
		return true
	}
	a, b, ok := strings.Cut(s, "://")
	if !ok || a == "" || !typ(a) || b == "" {
		return false
	}
	if b == "*" {
		return true
	}
	if strings.HasPrefix(b, "*.") {
		b = b[2:]
	} else if strings.HasSuffix(b, ".*") {
		b = b[:len(b)-2]
	}
	return b != "" && name(b)
}
func kind(s string) (string, string, string) {
	a, b, ok := strings.Cut(s, "://")
	if !ok {
		return "", "", "NONE"
	}
	if b == "*" {
		return a, b, "ALL"
	}
	if strings.HasPrefix(b, "*.") {
		return a, b[2:], "SUFFIX"
	}
	if strings.HasSuffix(b, ".*") {
		return a, b[:len(b)-2], "PREFIX"
	}
	return a, b, "EXACT"
}
func refinement(old, next string) (string, string) {
	if next == "" {
		return "CANDIDATE_EMPTY", ""
	}
	if old == next {
		return "IDENTICAL_PATTERNS", ""
	}
	if old == "*" {
		return "OLD_GLOBAL", ""
	}
	ot, ob, ok := kind(old)
	nt, nb, nk := kind(next)
	if ok == "ALL" && nt == ot {
		return "OLD_TYPED_ALL", ""
	}
	if nk == "EXACT" && upstream.MatchServicePattern(old, nt, nb) {
		return "EXACT_CANDIDATE_ACCEPTED_BY_OLD", ""
	}
	if ot == nt && ok == nk {
		if ok == "PREFIX" && (ob == nb || strings.HasPrefix(nb, ob+".")) {
			return "PREFIX_HIERARCHY", ""
		}
		if ok == "SUFFIX" && (ob == nb || strings.HasSuffix(nb, "."+ob)) {
			return "SUFFIX_HIERARCHY", ""
		}
	}
	types := []string{nt, nt}
	names := []string{"", ""}
	switch nk {
	case "ALL":
		names = []string{"x", "y"}
	case "EXACT":
		names[0] = nb
	case "PREFIX":
		names = []string{nb + ".x", nb + ".y"}
	case "SUFFIX":
		names = []string{"x." + nb, "y." + nb}
	}
	if next == "*" {
		types = []string{"a", "b"}
		names = []string{"x", "x"}
	}
	for i, n := range names {
		if n != "" && !upstream.MatchServicePattern(old, types[i], n) && upstream.MatchServicePattern(next, types[i], n) {
			return "", "WIDENING_WITNESS"
		}
	}
	return "", "REFINEMENT_UNPROVEN"
}
func assess(f Facts) Decision {
	d := Decision{Status: "INVALID_INPUT", InvalidIndex: -1, First: -1, Count: len(f.Types)}
	for _, v := range []struct{ field, value string }{{"before", f.Before}, {"after", f.After}} {
		if !pattern(v.value) {
			d.InvalidField = v.field
			d.Reason = "INVALID_CANONICAL_INPUT"
			return d
		}
	}
	if len(f.Types) != len(f.Names) {
		d.InvalidField = "required_names"
		d.Reason = "COLUMN_LENGTH_MISMATCH"
		return d
	}
	if len(f.Types) == 0 {
		d.InvalidField = "required_types"
		d.Reason = "EMPTY_REQUIRED_REQUESTS"
		return d
	}
	if len(f.Types) > 256 {
		d.InvalidField = "required_types"
		d.Reason = "REQUIRED_REQUEST_LIMIT_EXCEEDED"
		return d
	}
	for i, t := range f.Types {
		d.Checked++
		for _, v := range []struct {
			field, value string
			valid        bool
		}{{"required_types", t, typ(t)}, {"required_names", f.Names[i], name(f.Names[i])}} {
			if d.InvalidIndex < 0 && (!v.valid || v.value == "") {
				d.InvalidIndex = i
				d.InvalidField = v.field
				d.Reason = "INVALID_CANONICAL_INPUT"
				if v.value == "" {
					d.Reason = "EMPTY_REQUIRED_REQUEST"
				}
			}
		}
	}
	if d.InvalidIndex >= 0 {
		return d
	}
	d.Status = "REFUSED"
	d.All = true
	for i, t := range f.Types {
		b := upstream.MatchServicePattern(f.Before, t, f.Names[i])
		a := upstream.MatchServicePattern(f.After, t, f.Names[i])
		if d.First < 0 && (!b || !a) {
			d.First = i
			d.FirstBefore = b
			d.FirstAfter = a
			d.All = false
			d.Reason = "REQUIRED_REQUEST_REMOVED"
			if !b {
				d.Reason = "CURRENT_REQUIRED_REQUEST_DENIED"
			}
		}
	}
	rule, refusal := refinement(f.Before, f.After)
	if rule == "" {
		d.Reason = refusal
		return d
	}
	d.Subset = true
	d.Rule = rule
	if !d.All {
		return d
	}
	d.Status = "ELIGIBLE"
	d.Reason = "SUBSET_AND_ALL_REQUIRED_REQUESTS_PRESERVED"
	return d
}
func main() {
	b, err := os.ReadFile("../reference-cases.json")
	if err != nil {
		panic(err)
	}
	var cases []Case
	if err = json.Unmarshal(b, &cases); err != nil {
		panic(err)
	}
	if len(cases) > 17 {
		panic("finite case limit")
	}
	for i := range cases {
		if len(cases[i].Facts.Types) > 257 || len(cases[i].Facts.Names) > 257 {
			panic("collection bound")
		}
		cases[i].Decision = assess(cases[i].Facts)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err = enc.Encode(cases); err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "independent Go reference only; not a Gooo production fallback")
}
