// Package main transports exact browser text to the existing checked APIs.
// It does not apply a policy or establish authority for any supplied fact.
package main

import (
	"encoding/json"
	"errors"
	"unicode/utf16"

	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
)

const maxFieldUTF16Units = 4096

func fieldCount(mode string) int {
	switch mode {
	case "match":
		return 3
	case "impact":
		return 4
	case "refine":
		return 2
	default:
		return 0
	}
}

// Validate the original code units before decoding. syscall/js's direct string
// conversion would replace an unpaired surrogate, changing the supplied fact.
func exactUTF16(units []uint16) (string, error) {
	if len(units) > maxFieldUTF16Units {
		return "", errors.New("each field must contain at most 4096 UTF-16 code units")
	}
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u <= 0xdbff {
			if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", errors.New("unpaired UTF-16 surrogate is not exact Text")
			}
			i++
		} else if u >= 0xdc00 && u <= 0xdfff {
			return "", errors.New("unpaired UTF-16 surrogate is not exact Text")
		}
	}
	return string(utf16.Decode(units)), nil
}

func failureJSON(message string) string {
	raw, _ := json.Marshal(struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}{Error: message})
	return string(raw)
}

func evaluateJSON(mode string, fields [][]uint16) string {
	n := fieldCount(mode)
	if n == 0 || len(fields) != n {
		return failureJSON("unknown mode or incorrect field count")
	}
	values := make([]string, n)
	for i, units := range fields {
		value, err := exactUTF16(units)
		if err != nil {
			return failureJSON(err.Error())
		}
		values[i] = value
	}
	var proof any
	var err error
	switch mode {
	case "match":
		proof, err = servicepolicy.EvaluateMatcherChecked(servicepolicy.MatcherFacts{Pattern: values[0], ReqType: values[1], ReqName: values[2]})
	case "impact":
		proof, err = servicepolicy.EvaluateImpactChecked(servicepolicy.ImpactFacts{Before: values[0], After: values[1], ReqType: values[2], ReqName: values[3]})
	case "refine":
		proof, err = servicepolicy.EvaluateRefinementChecked(servicepolicy.RefinementFacts{Before: values[0], After: values[1]})
	}
	if err != nil {
		return failureJSON(err.Error())
	}
	raw, err := json.Marshal(struct {
		OK    bool `json:"ok"`
		Proof any  `json:"proof"`
	}{OK: true, Proof: proof})
	if err != nil {
		return failureJSON("cannot encode the checked native proof")
	}
	return string(raw)
}

// This is the native Bindings object, without an extra successful envelope.
func bindingsJSON() string {
	bindings, err := servicepolicy.SourceBindings()
	if err != nil {
		return failureJSON(err.Error())
	}
	raw, err := json.Marshal(bindings)
	if err != nil {
		return failureJSON("cannot encode the native source bindings")
	}
	return string(raw)
}
