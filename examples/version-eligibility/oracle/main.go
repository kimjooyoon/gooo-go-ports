// Independent policy oracle imports authenticated, unmodified x/mod/semver.
package main

import (
	"bytes"
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
	Version   *string `json:"version,omitempty"`
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

type StableFacts struct {
	Current         string `json:"current"`
	Candidate       string `json:"candidate"`
	Minimum         string `json:"minimum"`
	CurrentValid    bool   `json:"current_valid"`
	CandidateValid  bool   `json:"candidate_valid"`
	MinimumValid    bool   `json:"minimum_valid"`
	CurrentOrder    int    `json:"candidate_vs_current"`
	MinimumOrder    int    `json:"candidate_vs_minimum"`
	Newer           bool   `json:"precedence_newer"`
	MeetsMinimum    bool   `json:"meets_minimum"`
	Eligible        bool   `json:"eligible"`
	Reason          string `json:"reason"`
	SameMajor       bool   `json:"same_major"`
	CandidateStable bool   `json:"candidate_stable"`
	PolicyEligible  bool   `json:"policy_eligible"`
	PolicyReason    string `json:"policy_reason"`
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

// Keep the minimum policy unchanged; omit its redundant raw_changed in this
// separate 16-field result. The additional facts
// call the pinned original APIs, not a copied Gooo parser or output fixture.
func stable(current, candidate, floor string) StableFacts {
	baseline := minimum(current, candidate, floor)
	sameMajor := baseline.CurrentValid && baseline.CandidateValid && semver.Major(current) == semver.Major(candidate)
	candidateStable := baseline.CandidateValid && semver.Prerelease(candidate) == ""
	policyEligible := baseline.Eligible && sameMajor && candidateStable
	policyReason := baseline.Reason
	if baseline.Eligible {
		if !sameMajor {
			policyReason = "different_major"
		} else if !candidateStable {
			policyReason = "prerelease"
		} else {
			policyReason = "eligible"
		}
	}
	return StableFacts{baseline.Current, baseline.Candidate, baseline.Minimum, baseline.CurrentValid, baseline.CandidateValid, baseline.MinimumValid, baseline.CurrentOrder, baseline.MinimumOrder, baseline.Newer, baseline.MeetsMinimum, baseline.Eligible, baseline.Reason, sameMajor, candidateStable, policyEligible, policyReason}
}

func main() {
	policy := flag.String("policy", "upgrade", "upgrade, minimum, stable, major or prerelease")
	flag.Parse()
	fail := func(message any) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
	projection := *policy == "major" || *policy == "prerelease"
	if (*policy != "upgrade" && *policy != "minimum" && *policy != "stable" && !projection) || flag.NArg() != 0 {
		fail("require --policy upgrade, minimum, stable, major or prerelease, without positional arguments")
	}
	var rows []json.RawMessage
	d := json.NewDecoder(os.Stdin)
	if err := d.Decode(&rows); err != nil {
		fail(err)
	}
	out := make([]any, 0, len(rows))
	projected := make([]string, 0, len(rows))
	for _, raw := range rows {
		var in Input
		row := json.NewDecoder(bytes.NewReader(raw))
		row.DisallowUnknownFields()
		if err := row.Decode(&in); err != nil {
			fail(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			fail(err)
		}
		if projection {
			if len(fields) != 2 || fields["name"] == nil || fields["version"] == nil {
				fail("projection input requires exactly name and version")
			}
			var name *string
			if err := json.Unmarshal(fields["name"], &name); err != nil || name == nil {
				fail("projection name requires a string")
			}
			if in.Version == nil || len(*in.Version) > 1024 || !utf8.ValidString(*in.Version) {
				fail("required decoded valid UTF-8 Text 0..1024 bytes")
			}
			if *policy == "major" {
				projected = append(projected, semver.Major(*in.Version))
			} else {
				projected = append(projected, semver.Prerelease(*in.Version))
			}
			continue
		}
		if _, present := fields["version"]; present {
			fail("version is unsupported in policy input")
		}
		values := []*string{in.Current, in.Candidate}
		if *policy == "minimum" || *policy == "stable" {
			values = append(values, in.Minimum)
		} else if in.Minimum != nil {
			fail("minimum is unsupported in upgrade input")
		}
		for _, value := range values {
			if value == nil || len(*value) > 1024 || !utf8.ValidString(*value) {
				fail("required decoded valid UTF-8 Text 0..1024 bytes")
			}
		}
		if *policy == "stable" {
			out = append(out, stable(*in.Current, *in.Candidate, *in.Minimum))
		} else if *policy == "minimum" {
			out = append(out, minimum(*in.Current, *in.Candidate, *in.Minimum))
		} else {
			out = append(out, upgrade(*in.Current, *in.Candidate))
		}
	}
	if projection {
		if err := json.NewEncoder(os.Stdout).Encode(projected); err != nil {
			fail(err)
		}
		return
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
}
