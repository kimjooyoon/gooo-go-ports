package servicepolicy

import "errors"

func decodeGrantUpdateDecision(value any) (GrantUpdateDecision, error) {
	var d GrantUpdateDecision
	keys := []string{"status", "reason", "invalid_field", "required_observed", "required_before_match", "required_after_match", "subset_rule_id"}
	if err := decodeDecision(value, keys, &d); err != nil {
		return GrantUpdateDecision{}, err
	}
	if err := d.Validate(); err != nil {
		return GrantUpdateDecision{}, err
	}
	return d, nil
}

func knownGrantUpdateRule(rule string) bool {
	switch rule {
	case "CANDIDATE_EMPTY", "IDENTICAL_PATTERNS", "OLD_GLOBAL", "OLD_TYPED_ALL", "EXACT_CANDIDATE_ACCEPTED_BY_OLD", "PREFIX_HIERARCHY", "SUFFIX_HIERARCHY":
		return true
	}
	return false
}

// Validate checks the closed result schema and sentinel invariants. It does not
// evaluate patterns, prove eligibility, authenticate a writer or authorize a write.
func (d GrantUpdateDecision) Validate() error {
	bad := errors.New("unknown or inconsistent generated Gooo decision")
	switch d.Status {
	case "ELIGIBLE":
		if d.Reason == "SUBSET_AND_REQUIRED_REQUEST_PRESERVED" && d.InvalidField == "" && d.RequiredObserved && d.RequiredBeforeMatch && d.RequiredAfterMatch && knownGrantUpdateRule(d.SubsetRuleID) && d.SubsetRuleID != "CANDIDATE_EMPTY" {
			return nil
		}
	case "INVALID_INPUT":
		if d.RequiredObserved || d.RequiredBeforeMatch || d.RequiredAfterMatch || d.SubsetRuleID != "" {
			return bad
		}
		if d.Reason == "EMPTY_REQUIRED_REQUEST" && (d.InvalidField == "required_type" || d.InvalidField == "required_name") {
			return nil
		}
		if d.Reason == "INVALID_CANONICAL_INPUT" {
			switch d.InvalidField {
			case "before", "after", "required_type", "required_name":
				return nil
			}
		}
	case "REFUSED":
		if d.InvalidField != "" {
			return bad
		}
		switch d.Reason {
		case "DEPENDENCY_RESULT_INVALID":
			if d.SubsetRuleID == "" && ((!d.RequiredObserved && !d.RequiredBeforeMatch && !d.RequiredAfterMatch) || (d.RequiredObserved && d.RequiredBeforeMatch)) {
				return nil
			}
		case "CURRENT_REQUIRED_REQUEST_DENIED":
			if d.RequiredObserved && !d.RequiredBeforeMatch && d.SubsetRuleID == "" {
				return nil
			}
		case "WIDENING_WITNESS", "REFINEMENT_UNPROVEN":
			if d.RequiredObserved && d.RequiredBeforeMatch && d.SubsetRuleID == "" {
				return nil
			}
		case "REQUIRED_REQUEST_REMOVED":
			if d.RequiredObserved && d.RequiredBeforeMatch && !d.RequiredAfterMatch && knownGrantUpdateRule(d.SubsetRuleID) {
				return nil
			}
		}
	}
	return bad
}
