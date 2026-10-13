package servicepolicy

import "errors"

const MaxComponentBytes = 128

// Grammar belongs to Gooo. These adapters call the pinned compiler's generated
// predicate bodies, including ASCII and independent 128-byte bounds.
func validType(value string) bool    { return nativeValidType(value) }
func validName(value string) bool    { return nativeValidName(value) }
func validPattern(value string) bool { return nativeValidPattern(value) }

func firstInvalidMatcher(f MatcherFacts) string {
	if !validPattern(f.Pattern) {
		return "pattern"
	}
	if !validType(f.ReqType) {
		return "req_type"
	}
	if !validName(f.ReqName) {
		return "req_name"
	}
	return ""
}

func firstInvalidImpact(f ImpactFacts) string {
	if !validPattern(f.Before) {
		return "before"
	}
	if !validPattern(f.After) {
		return "after"
	}
	if !validType(f.ReqType) {
		return "req_type"
	}
	if !validName(f.ReqName) {
		return "req_name"
	}
	return ""
}

func firstInvalidRefinement(f RefinementFacts) string {
	if !validPattern(f.Before) {
		return "before"
	}
	if !validPattern(f.After) {
		return "after"
	}
	return ""
}

func invalidInput(field string) error {
	return errors.New("outside canonical ASCII service-pattern domain (128 bytes per component): " + field)
}

func ValidateMatcher(f MatcherFacts) error {
	if field := firstInvalidMatcher(f); field != "" {
		return invalidInput(field)
	}
	return nil
}

func ValidateImpact(f ImpactFacts) error {
	if field := firstInvalidImpact(f); field != "" {
		return invalidInput(field)
	}
	return nil
}

// ValidateRefinement delegates syntax to Gooo without running the analyzer.
func ValidateRefinement(f RefinementFacts) error {
	if field := firstInvalidRefinement(f); field != "" {
		return invalidInput(field)
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

func validateDomainDecision(status, field, expectedInvalid string) error {
	if expectedInvalid == "" && status == "VALID_INPUT" && field == "" {
		return nil
	}
	if expectedInvalid != "" && status == "INVALID_INPUT" && field == expectedInvalid {
		return nil
	}
	return errors.New("inconsistent Gooo input-domain decision")
}

func validateCheckedMatcherDecision(f MatcherFacts, d CheckedMatcherDecision) error {
	if err := validateDomainDecision(d.DomainStatus, d.InvalidField, firstInvalidMatcher(f)); err != nil {
		return err
	}
	if d.DomainStatus == "INVALID_INPUT" && d.Matched {
		return errors.New("invalid Gooo matcher refusal sentinel")
	}
	return nil
}

func validateCheckedImpactDecision(f ImpactFacts, d CheckedImpactDecision) error {
	if err := validateDomainDecision(d.DomainStatus, d.InvalidField, firstInvalidImpact(f)); err != nil {
		return err
	}
	if d.DomainStatus == "INVALID_INPUT" {
		if d.BeforeMatch || d.AfterMatch || d.Change != "INVALID_INPUT" {
			return errors.New("invalid Gooo impact refusal sentinel")
		}
		return nil
	}
	want := "UNCHANGED_DENY"
	if d.BeforeMatch && d.AfterMatch {
		want = "UNCHANGED_ALLOW"
	} else if d.BeforeMatch {
		want = "REVOKED"
	} else if d.AfterMatch {
		want = "WIDENED"
	}
	if d.Change != want {
		return errors.New("inconsistent Gooo impact decision")
	}
	return nil
}

func validateCheckedRefinementDecision(f RefinementFacts, d CheckedRefinementDecision) error {
	if err := validateDomainDecision(d.DomainStatus, d.InvalidField, firstInvalidRefinement(f)); err != nil {
		return err
	}
	if d.DomainStatus == "INVALID_INPUT" {
		if d.RefinementDecision != (RefinementDecision{Status: "INVALID_INPUT"}) {
			return errors.New("invalid Gooo refinement refusal sentinel")
		}
		return nil
	}
	return validateRefinementDecision(d.RefinementDecision)
}
