package main

import (
	"encoding/json"
	"fmt"
)

const stringByteMaxBatchBytes = 30000

func isStringByteSuite(s suite) bool {
	return (s.Package == stringBytePackage && s.Activity == "IndexByte") ||
		(s.Package == stringByteConsumerPackage && s.Activity == "Search")
}

// Text suites adapt to serialized bytes. Existing suites retain their exact
// boundaries. Match save's MarshalIndent and final LF,
// rather than estimating from Text length or compact JSON.
func nativeBatchEnd(s suite, start, rowLimit int) (int, error) {
	if start < 0 || start >= len(s.Cases) || rowLimit < 1 || rowLimit > 128 {
		return 0, fmt.Errorf("invalid native batch start or row limit")
	}
	end := min(start+rowLimit, len(s.Cases))
	if !isStringByteSuite(s) && !isUUIDSuite(s) && !isLiteralTextSuite(s) && !isLiteralCutSuite(s) && !isLastIndexSuite(s) {
		return end, nil
	}
	// Unknown/named marshalers retain the original full-document evaluations.
	// The memo is local to this call; no oracle, source or mutable corpus cache.
	measure := func(candidate int) (int, error) {
		batch := s
		batch.Cases = s.Cases[start:candidate]
		raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
		return len(raw) + 1, err
	}
	pure := true
	for _, row := range s.Cases[start:end] {
		for _, input := range row.Inputs {
			pure = pure && nativeBatchPureJSONValue(input, 0)
		}
		pure = pure && nativeBatchPureJSONValue(row.Expected, 0)
	}
	if pure {
		batch := s
		batch.Cases = s.Cases[start:end]
		document := caseDocument(batch).(map[string]any)
		rows := document["cases"].([]any)
		const indent = "  "
		rowPrefix := indent + indent // top-level cases array contains row objects.
		// Measure the real schema/shape with one inert object. Remove only its
		// encoded bytes; preserve envelope, first-row indent and final LF.
		document["cases"] = []any{map[string]any{}}
		envelope, err := json.MarshalIndent(document, "", indent)
		if err != nil {
			return 0, fmt.Errorf("serialize string byte batch: %w", err)
		}
		marker, err := json.MarshalIndent(map[string]any{}, rowPrefix, indent)
		if err != nil {
			return 0, fmt.Errorf("serialize string byte batch: %w", err)
		}
		overhead := len(envelope) + 1 - len(marker)
		separator := len(",\n" + rowPrefix)
		prefix := []int{0}
		measure = func(candidate int) (int, error) {
			count := candidate - start // always >=1 under the original probes.
			for len(prefix) <= count {
				row, err := json.MarshalIndent(rows[len(prefix)-1], rowPrefix, indent)
				if err != nil {
					return 0, err
				}
				prefix = append(prefix, prefix[len(prefix)-1]+len(row))
			}
			return overhead + prefix[count] + (count-1)*separator, nil
		}
	}
	low, high, best := start+1, end, start
	for low <= high {
		candidate := low + (high-low)/2
		size, err := measure(candidate)
		if err != nil {
			return 0, fmt.Errorf("serialize string byte batch: %w", err)
		}
		if size <= stringByteMaxBatchBytes {
			best, low = candidate, candidate+1
		} else {
			high = candidate - 1
		}
	}
	if best == start {
		return 0, fmt.Errorf("one string byte case exceeds %d serialized bytes", stringByteMaxBatchBytes)
	}
	return best, nil
}

// Only exact builtin JSON values are memoized. Named/custom marshalers, cycles
// and deep containers fall back to the original encoder/probe behavior.
func nativeBatchPureJSONValue(value any, depth int) bool {
	if depth > 32 {
		return false
	}
	switch v := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	case []any:
		for _, child := range v {
			if !nativeBatchPureJSONValue(child, depth+1) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, child := range v {
			if !nativeBatchPureJSONValue(child, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
