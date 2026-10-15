package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
)

// Pinned workspace lowering preserves the JSON values and replaces the entry
// key base in each input and expected value. Bound its ordinal by the total
// selected source bytes, rather than guessing the actual lowered symbol.
func decimalPayloadBatchPolicy(s suite, selected suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if !isDecimalPayloadSuite(s) || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.2" {
		return nativeCaseBatchPolicy{}, fmt.Errorf("decimal/payload batching requires pinned compiler839 and Go1.27.2")
	}
	if selected.Manifest.Schema != "gooo/package-workspace-manifest/v1" || selected.Manifest.Entry.Package != s.Package || selected.Manifest.Entry.Activity != s.Activity || len(selected.Names) == 0 || len(selected.Names) != len(selected.Sources) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("decimal/payload batching requires exact selected source closure")
	}
	seen, total := map[string]bool{}, 0
	for _, name := range selected.Names {
		raw, ok := selected.Sources[name]
		if !filepath.IsLocal(name) || seen[name] || !ok || len(raw) == 0 || len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("decimal/payload source closure is missing, repeated or exceeds10MiB")
		}
		seen[name], total = true, total+len(raw)
	}
	lowerMax := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	return nativeCaseBatchPolicy{decimalPayload: true, sourceBytes: total, loweredKeyBytesUpperBound: lowerMax}, nil
}

func (p nativeCaseBatchPolicy) prepareDecimalPayload(s suite, start int) (nativePreparedCaseBatch, error) {
	if !isDecimalPayloadSuite(s) || start < 0 || start >= len(s.Cases) || p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes || p.loweredKeyBytesUpperBound != len("GoooPackage")+len(strconv.Itoa(p.sourceBytes))+len("Activity")+len(s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("invalid decimal/payload batch or lowering bound")
	}
	arity := 1
	if s.Package == payloadAdmissionPackage {
		arity = 7
	}
	growth := max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
	low, high, best := start+1, min(start+128, len(s.Cases)), start
	var chosen []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		for _, c := range batch.Cases {
			if len(c.Inputs) != arity {
				return nativePreparedCaseBatch{}, fmt.Errorf("decimal/payload input arity differs")
			}
		}
		document := caseDocument(batch)
		raw, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		compact, err := json.Marshal(document)
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		translatedUpper := len(compact) + (arity+1)*(end-start)*growth
		if len(raw)+1 <= 30000 && translatedUpper <= 32768 {
			best, chosen, low = end, append(raw, '\n'), end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one decimal/payload case exceeds wire or translated budget")
	}
	return nativePreparedCaseBatch{End: best, Raw: chosen}, nil
}
