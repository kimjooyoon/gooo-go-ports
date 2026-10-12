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
	if !isStringByteSuite(s) && !isUUIDSuite(s) {
		return end, nil
	}
	low, high, best := start+1, end, start
	for low <= high {
		candidate := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:candidate]
		raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
		if err != nil {
			return 0, fmt.Errorf("serialize string byte batch: %w", err)
		}
		if len(raw)+1 <= stringByteMaxBatchBytes {
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
