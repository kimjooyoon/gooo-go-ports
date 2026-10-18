// Independent policy oracle imports authenticated, unmodified x/mod/semver.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"unicode/utf8"

	"golang.org/x/mod/semver"
)

type Input struct {
	Name      string  `json:"name"`
	Current   *string `json:"current"`
	Candidate *string `json:"candidate"`
	Minimum   *string `json:"minimum,omitempty"`
}
type UpgradeFacts struct {
	Current        string `json:"current"`
	Candidate      string `json:"candidate"`
	CurrentValid   bool   `json:"current_valid"`
	CandidateValid bool   `json:"candidate_valid"`
	Order          int    `json:"candidate_vs_current"`
	Newer          bool   `json:"precedence_newer"`
	Eligible       bool   `json:"eligible"`
	Reason         string `json:"reason"`
	RawChanged     bool   `json:"raw_changed"`
}
type MinimumFacts struct {
	Current        string `json:"current"`
	Candidate      string `json:"candidate"`
	Minimum        string `json:"minimum"`
	CurrentValid   bool   `json:"current_valid"`
	CandidateValid bool   `json:"candidate_valid"`
	MinimumValid   bool   `json:"minimum_valid"`
	CurrentOrder   int    `json:"candidate_vs_current"`
	MinimumOrder   int    `json:"candidate_vs_minimum"`
	Newer          bool   `json:"precedence_newer"`
	MeetsMinimum   bool   `json:"meets_minimum"`
	Eligible       bool   `json:"eligible"`
	Reason         string `json:"reason"`
	RawChanged     bool   `json:"raw_changed"`
}

func upgrade(current, candidate string) UpgradeFacts {
	cv, nv := semver.IsValid(current), semver.IsValid(candidate)
	order := semver.Compare(candidate, current)
	newer := order > 0
	eligible := cv && nv && newer
	reason := "eligible"
	if !cv {
		reason = "invalid_current"
	} else if !nv {
		reason = "invalid_candidate"
	} else if !newer {
		reason = "not_newer"
	}
	return UpgradeFacts{current, candidate, cv, nv, order, newer, eligible, reason, current != candidate}
}

func minimum(current, candidate, floor string) MinimumFacts {
	cv, nv, mv := semver.IsValid(current), semver.IsValid(candidate), semver.IsValid(floor)
	currentOrder, floorOrder := semver.Compare(candidate, current), semver.Compare(candidate, floor)
	newer, meets := currentOrder > 0, floorOrder >= 0
	eligible := cv && nv && mv && newer && meets
	reason := "eligible"
	if !cv {
		reason = "invalid_current"
	} else if !nv {
		reason = "invalid_candidate"
	} else if !mv {
		reason = "invalid_minimum"
	} else if !newer {
		reason = "not_newer"
	} else if !meets {
		reason = "below_minimum"
	}
	return MinimumFacts{current, candidate, floor, cv, nv, mv, currentOrder, floorOrder, newer, meets, eligible, reason, current != candidate}
}

func main() {
	policy := flag.String("policy", "upgrade", "upgrade or minimum policy")
	flag.Parse()
	fail := func(message any) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
	if (*policy != "upgrade" && *policy != "minimum") || flag.NArg() != 0 {
		fail("require --policy upgrade or minimum, without positional arguments")
	}
	var rows []Input
	d := json.NewDecoder(os.Stdin)
	d.DisallowUnknownFields()
	if err := d.Decode(&rows); err != nil {
		fail(err)
	}
	out := make([]any, 0, len(rows))
	for _, in := range rows {
		values := []*string{in.Current, in.Candidate}
		if *policy == "minimum" {
			values = append(values, in.Minimum)
		} else if in.Minimum != nil {
			fail("minimum is unsupported in upgrade input")
		}
		for _, value := range values {
			if value == nil || len(*value) > 1024 || !utf8.ValidString(*value) {
				fail("required decoded valid UTF-8 Text 0..1024 bytes")
			}
		}
		if *policy == "minimum" {
			out = append(out, minimum(*in.Current, *in.Candidate, *in.Minimum))
		} else {
			out = append(out, upgrade(*in.Current, *in.Candidate))
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
}
