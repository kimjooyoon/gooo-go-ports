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
