package servicepolicy

import (
	"errors"
	"regexp"
	"strings"
)

const MaxComponentBytes = 128

var scheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
var name = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// This validates transport syntax only. It computes no service match or impact.
func validType(value string) bool {
	return len(value) <= MaxComponentBytes && (value == "" || scheme.MatchString(value))
}
func validName(value string) bool {
	return len(value) <= MaxComponentBytes && (value == "" || name.MatchString(value))
}
func validPattern(value string) bool {
	if len(value) > MaxComponentBytes {
		return false
	}
	if value == "" || value == "*" {
		return true
	}
	typ, part, found := strings.Cut(value, "://")
	if !found || !scheme.MatchString(typ) || part == "" {
		return false
	}
	if part == "*" {
		return true
	}
	if strings.HasPrefix(part, "*.") {
		part = part[2:]
	} else if strings.HasSuffix(part, ".*") {
		part = part[:len(part)-2]
	}
	return name.MatchString(part)
}

func ValidateMatcher(f MatcherFacts) error {
	if !validPattern(f.Pattern) || !validType(f.ReqType) || !validName(f.ReqName) {
		return errors.New("outside canonical ASCII service-pattern domain (128 bytes per component)")
	}
	return nil
}

func ValidateImpact(f ImpactFacts) error {
	if err := ValidateMatcher(MatcherFacts{Pattern: f.Before, ReqType: f.ReqType, ReqName: f.ReqName}); err != nil {
		return err
	}
	return ValidateMatcher(MatcherFacts{Pattern: f.After, ReqType: f.ReqType, ReqName: f.ReqName})
}

// ValidateRefinement validates both raw patterns before Gooo takes any shortcut.
func ValidateRefinement(f RefinementFacts) error {
	if !validPattern(f.Before) || !validPattern(f.After) {
		return errors.New("outside canonical ASCII service-pattern domain (128 bytes per component)")
	}
	return nil
}

// This checks the typed output contract, not semantic grant inclusion or matches.
func validateRefinementDecision(d RefinementDecision) error {
	invalid := errors.New("inconsistent Gooo refinement decision")
	noWitness := !d.HasWitness && d.WitnessType == "" && d.WitnessName == "" && !d.BeforeMatch && !d.AfterMatch
	switch d.Status {
	case "SUPPORTED_SUBSET":
		if !d.SupportedSubset || !noWitness {
			return invalid
		}
		switch d.RuleID {
		case "CANDIDATE_EMPTY", "IDENTICAL_PATTERNS", "OLD_GLOBAL", "OLD_TYPED_ALL", "EXACT_CANDIDATE_ACCEPTED_BY_OLD", "PREFIX_HIERARCHY", "SUFFIX_HIERARCHY":
			return nil
		}
	case "WIDENING_WITNESS":
		if d.SupportedSubset || !d.HasWitness || d.RuleID != "CHECKED_CANDIDATE_REQUEST" || d.BeforeMatch || !d.AfterMatch || d.WitnessType == "" || d.WitnessName == "" || !validType(d.WitnessType) || !validName(d.WitnessName) {
			return invalid
		}
		return nil
	case "UNPROVEN":
		if !d.SupportedSubset && noWitness && d.RuleID == "" {
			return nil
		}
	}
	return invalid
}
