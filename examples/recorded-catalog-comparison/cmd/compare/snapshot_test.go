// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"bytes"
	"encoding/json"
	"testing"

	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func snapshotCommandInput(t *testing.T) []byte {
	t.Helper()
	p := origin.SnapshotPage{RequestJSON: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, ResponseJSON: `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"lookup"}]}}`}
	raw, err := json.Marshal(origin.DefinitionSnapshotInput{TargetName: "lookup", Before: origin.SnapshotObservation{Pages: []origin.SnapshotPage{p}}, After: origin.SnapshotObservation{Pages: []origin.SnapshotPage{p}}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSnapshotCommandRejectsHostOptionsBeforeInput(t *testing.T) {
	for _, args := range [][]string{{"--definition-snapshot", "--before-policy", "x"}, {"--definition-snapshot", "--diagnostics"}, {"--definition-snapshot", "--availability", "a.json"}, {"--definitions", "--definition-snapshot"}, {"--definition-snapshot", "extra"}} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 {
			t.Fatal("unsupported combined mode consumed input or succeeded", args, code)
		}
	}
}

func TestSnapshotCommandPreservesInputAndUnknownAuthority(t *testing.T) {
	raw := snapshotCommandInput(t)
	var out, diagnostics bytes.Buffer
	if code := runCommand([]string{"--definition-snapshot"}, bytes.NewReader(raw), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatal(code, diagnostics.String())
	}
	var report origin.DefinitionSnapshotReport
	if json.Unmarshal(out.Bytes(), &report) != nil || report.Projection.InputJSON != string(raw) || report.Decision.State != "OBSERVED_DEFINITION_SAME" || report.Decision.OriginRelation != "NOT_ASSESSED" || report.Decision.AuthorizationRelation != "NOT_ASSESSED" || report.Decision.ExecutionAuthorized {
		t.Fatal("wrong selected snapshot or invented authority")
	}
}

func TestSnapshotCommandIOFailuresDoNotSucceed(t *testing.T) {
	for _, short := range []bool{false, true} {
		var diagnostics bytes.Buffer
		if code := runCommand([]string{"--definition-snapshot"}, bytes.NewReader(snapshotCommandInput(t)), failingOutput{short: short}, &diagnostics); code != 1 {
			t.Fatal("failed write succeeded", code)
		}
		var problem origin.Problem
		if json.Unmarshal(diagnostics.Bytes(), &problem) != nil || problem.Code != "IO_FAILURE" {
			t.Fatal("missing I/O diagnostic", diagnostics.String())
		}
	}
	var out, diagnostics bytes.Buffer
	if code := runCommand([]string{"--definition-snapshot"}, errorInput{}, &out, &diagnostics); code != 1 || out.Len() != 0 {
		t.Fatal("failed read emitted a successful report", code)
	}
}
