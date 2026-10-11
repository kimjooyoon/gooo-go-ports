package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func verifyFailureProbe(out string) error {
	read := func(name string) []byte { raw, _ := os.ReadFile(filepath.Join(out, name)); return raw }
	raw := read("missing-context.json")
	if err := checkJSON(raw, 16<<20, 64); err != nil {
		return err
	}
	var r struct {
		Schema   string          `json:"schema"`
		Decision string          `json:"decision"`
		Error    string          `json:"error"`
		Manifest string          `json:"manifest_digest"`
		Cases    string          `json:"cases_digest"`
		Result   json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	if string(read("missing-context.exit.txt")) != "1" || r.Schema != "gooo/workspace-body-execution-receipt/v1" || r.Decision != "FAIL_CLOSED" || !strings.Contains(r.Error, `requires field "now"`) || len(r.Result) != 0 || r.Manifest != digest(read("gooo.workspace.json")) || r.Cases != digest(read("missing-context-cases.json")) {
		return errors.New("compiler failure evidence not bound")
	}
	return nil
}
func writeChecksums(out string) error {
	files, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	hashes := map[string]string{}
	for _, f := range files {
		if f.IsDir() || f.Name() == "checksums.json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(out, f.Name()))
		if err != nil {
			return err
		}
		hashes[f.Name()] = digest(raw)
	}
	return save(out, "checksums.json", append(marshal(hashes), '\n'))
}
func verifySaved(out string) error {
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(out, name)) }
	raw, err := read("checksums.json")
	if err != nil {
		return err
	}
	var hashes map[string]string
	if err = json.Unmarshal(raw, &hashes); err != nil {
		return err
	}
	files, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	if len(files) != len(hashes)+1 {
		return errors.New("saved file inventory differs")
	}
	for name, want := range hashes {
		if filepath.Base(name) != name {
			return errors.New("nonlocal evidence path")
		}
		raw, err := read(name)
		if err != nil {
			return err
		}
		if digest(raw) != want {
			return fmt.Errorf("saved file changed: %s", name)
		}
	}
	identity, err := read("compiler.json")
	if err != nil {
		return err
	}
	if err = checkIdentity(identity); err != nil {
		return err
	}
	all, valid, suite, err := loadFixtures(filepath.Join(out, "fixtures.json"))
	if err != nil {
		return err
	}
	cases, err := read("cases.json")
	if err != nil {
		return err
	}
	if !bytes.Equal(cases, append(marshal(suite), '\n')) {
		return errors.New("cases do not preserve fixtures and independent Go reference")
	}
	actual, err := verifyReceipts(out, valid)
	if err != nil {
		return err
	}
	rows := []CaseResult{}
	index, dispatches := 0, 0
	for _, f := range all {
		row := CaseResult{ID: f.ID, InputDigest: digest(f.raw())}
		_, inputErr := decodeEnvelope(f.raw())
		if inputErr != nil {
			row.Outcome = Outcome{false, "reject", "invalid_input"}
			row.Error = inputErr.Error()
		} else {
			row.Outcome = actual[index]
			index++
			row.SpyCalls = boolInt(row.Outcome.Allowed)
		}
		if row.Outcome != f.Expected.Outcome || row.SpyCalls != f.Expected.SpyCalls {
			return fmt.Errorf("manual observation mismatch: %s", f.ID)
		}
		dispatches += row.SpyCalls
		rows = append(rows, row)
	}
	observed, err := read("observations.json")
	if err != nil {
		return err
	}
	if !exact(observed, rows) {
		return errors.New("saved spy observation mismatch")
	}
	if err = verifyFailureProbe(out); err != nil {
		return err
	}
	summary, err := read("summary.json")
	if err != nil {
		return err
	}
	var report map[string]json.RawMessage
	if err = json.Unmarshal(summary, &report); err != nil {
		return err
	}
	for key, want := range map[string]any{"status": "PASS", "compiler_revision": compilerRevision, "fixtures": 49, "gooo_reference_cases": 31, "manual_expected_cases": 49, "host_input_rejections": 18, "mock_dispatches": dispatches, "replay_dispatches": 0, "actual_compiler_failure_probes": 1, "compiler_failure_dispatches": 0, "external_api_calls": 0, "real_exports": 0, "performance_measured": false, "synthetic_context": true} {
		if !exact(report[key], want) {
			return fmt.Errorf("saved summary mismatch: %s", key)
		}
	}
	fmt.Printf("Saved evidence verified: %s (no native execution, model call, or mock dispatch)\n", out)
	return nil
}
