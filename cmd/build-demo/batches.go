package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const maxCaseBytes = 32 * 1024
const maxCaseCount = 128

type caseDocument struct {
	Schema string            `json:"schema"`
	Cases  []json.RawMessage `json:"cases"`
}

type batchEvidence struct {
	Cases        int    `json:"cases"`
	CaseFile     string `json:"case_file"`
	CaseSHA      string `json:"case_sha256"`
	Execution    string `json:"execution"`
	ExecutionSHA string `json:"execution_sha256"`
	Replay       string `json:"replay"`
	ReplaySHA    string `json:"replay_sha256"`
}

type browserEvidence struct {
	Schema         string          `json:"schema"`
	SourceCases    string          `json:"source_cases"`
	SourceCasesSHA string          `json:"source_cases_sha256"`
	GeneratedSHA   string          `json:"generated_sha256"`
	Cases          int             `json:"cases"`
	Batches        []batchEvidence `json:"batches"`
}

// Preserve every case in order, including exact JSON integers. The limit is
// measured on the bytes actually supplied to the compiler, with their newline.
func splitBrowserCases(raw []byte) ([][]byte, error) {
	var source caseDocument
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	if source.Schema != "gooo/body-composition-cases/v1" || len(source.Cases) == 0 {
		return nil, fmt.Errorf("invalid or empty browser case document")
	}
	encode := func(rows []json.RawMessage) ([]byte, error) {
		b, err := json.MarshalIndent(caseDocument{source.Schema, rows}, "", "  ")
		return append(b, '\n'), err
	}
	var batches [][]byte
	var rows []json.RawMessage
	var current []byte
	for _, row := range source.Cases {
		next, err := encode(append(append([]json.RawMessage(nil), rows...), row))
		if err != nil {
			return nil, err
		}
		if len(rows) == maxCaseCount || len(next) > maxCaseBytes {
			if len(rows) == 0 {
				return nil, fmt.Errorf("one browser case exceeds %d bytes", maxCaseBytes)
			}
			batches = append(batches, current)
			rows = nil
			next, err = encode([]json.RawMessage{row})
			if err != nil {
				return nil, err
			}
			if len(next) > maxCaseBytes {
				return nil, fmt.Errorf("one browser case exceeds %d bytes", maxCaseBytes)
			}
		}
		rows = append(rows, row)
		current = next
	}
	return append(batches, current), nil
}

func executeBrowserBatches(ctx context.Context, site string, lock lockfile, bindings map[string]string) (string, browserEvidence, error) {
	index := browserEvidence{Schema: "gooo/browser-build-evidence/v1", SourceCases: casesPath}
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		return "", index, err
	}
	index.SourceCasesSHA = digest(raw)
	if err := write(filepath.Join(site, casesPath), raw); err != nil {
		return "", index, err
	}
	batches, err := splitBrowserCases(raw)
	if err != nil {
		return "", index, err
	}
	var generated string
	for i, batch := range batches {
		prefix := fmt.Sprintf("evidence/%03d/", i)
		item := batchEvidence{CaseFile: prefix + "cases.json", CaseSHA: digest(batch), Execution: prefix + "execution.json", Replay: prefix + "replay.json"}
		var cases caseDocument
		if err := json.Unmarshal(batch, &cases); err != nil {
			return "", index, err
		}
		item.Cases = len(cases.Cases)
		caseFile := filepath.Join(site, item.CaseFile)
		if err := write(caseFile, batch); err != nil {
			return "", index, err
		}
		execution, err := command(ctx, nil, "./.gooo", "package", "execute", "--json", "--cases", caseFile, manifest)
		if err != nil {
			return "", index, err
		}
		if err := write(filepath.Join(site, item.Execution), execution); err != nil {
			return "", index, err
		}
		if current, err := os.ReadFile(caseFile); err != nil || !bytes.Equal(current, batch) {
			return "", index, fmt.Errorf("browser case file changed during execution")
		}
		source, err := inspectBatchReceipt(execution, lock, bindings, "", caseFile)
		if err != nil {
			return "", index, err
		}
		if i != 0 && source != generated {
			return "", index, fmt.Errorf("browser batches generated different Go")
		}
		generated = source
		item.ExecutionSHA = digest(execution)
		replay, err := command(ctx, nil, "./.gooo", "package", "replay", "--json", "--receipt", filepath.Join(site, item.Execution), "--cases", caseFile, manifest)
		if err != nil {
			return "", index, err
		}
		if err := write(filepath.Join(site, item.Replay), replay); err != nil {
			return "", index, err
		}
		if current, err := os.ReadFile(caseFile); err != nil || !bytes.Equal(current, batch) {
			return "", index, fmt.Errorf("browser case file changed during replay")
		}
		replayed, err := inspectBatchReceipt(replay, lock, bindings, "sha256:"+item.ExecutionSHA, caseFile)
		if err != nil {
			return "", index, err
		}
		if replayed != generated {
			return "", index, fmt.Errorf("replay changed generated Go")
		}
		item.ReplaySHA = digest(replay)
		if i == 0 {
			// Keep the existing download URLs as clearly identified first-batch aliases.
			if err := write(filepath.Join(site, "execution.json"), execution); err != nil {
				return "", index, err
			}
			if err := write(filepath.Join(site, "replay.json"), replay); err != nil {
				return "", index, err
			}
		}
		index.Cases += item.Cases
		index.Batches = append(index.Batches, item)
	}
	index.GeneratedSHA = digest([]byte(generated))
	indexRaw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", index, err
	}
	if err := write(filepath.Join(site, "evidence-index.json"), append(indexRaw, '\n')); err != nil {
		return "", index, err
	}
	return generated, index, nil
}
