// Copyright 2026 Google LLC. Apache-2.0; original full files and license in ../upstream.
package oracle

import (
	"regexp"
	"strings"
)

var rfc3986URIRegex = regexp.MustCompile(`^(([^:/?#]+):)?(//([^/?#]*))?([^?#]*)(\?([^#]*))?(#(.*))?`)

func ParseServiceTarget(target string) (svcType, svcName string) {
	if target == "*" {
		return "*", "*"
	}

	if idx := strings.Index(target, "://"); idx >= 0 && !strings.Contains(target[:idx], ":") {
		matches := rfc3986URIRegex.FindStringSubmatch(target)
		if len(matches) < 6 {
			return "", target
		}
		scheme := matches[2]
		hasAuthority := matches[3] != ""
		authority := matches[4]
		path := matches[5]

		if scheme == "" || !hasAuthority {
			return "", target
		}
		name := authority
		if path != "" {
			name = authority + path
		}
		return scheme, name
	}

	parts := strings.SplitN(target, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", target
}

func MatchServicePattern(pattern, reqType, reqName string) bool {
	if reqType == "" || reqName == "" || pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	pType, pName := ParseServiceTarget(pattern)
	if pType == "" || pType != reqType || pName == "" {
		return false
	}
	if pName == "*" {
		return true
	}
	if strings.HasPrefix(pName, "*.") && !strings.HasSuffix(pName, ".*") {
		return strings.HasSuffix(reqName, pName[1:])
	}
	if strings.HasSuffix(pName, ".*") && !strings.HasPrefix(pName, "*.") {
		return strings.HasPrefix(reqName, pName[:len(pName)-1])
	}
	return reqName == pName
}
