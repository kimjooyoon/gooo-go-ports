// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/snapshotkernel"
)

func TestSnapshotGeneratedKernelIndependentCases(t *testing.T) {
	raw, err := os.ReadFile("snapshot-kernel-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Inputs   map[string]json.RawMessage            `json:"inputs"`
			Expected map[string]DefinitionSnapshotDecision `json:"expected"`
		} `json:"cases"`
	}
	if json.Unmarshal(raw, &corpus) != nil || corpus.Schema != "gooo/body-composition-cases/v1" || len(corpus.Cases) != 89 {
		t.Fatal("89 independent reference cases required")
	}
	for i, c := range corpus.Cases {
		var before, after snapshotkernel.SnapshotFacts
		var selection snapshotkernel.Selection
		if json.Unmarshal(c.Inputs[snapshotEntry+".input0"], &before) != nil || json.Unmarshal(c.Inputs[snapshotEntry+".input1"], &after) != nil || json.Unmarshal(c.Inputs[snapshotEntry+".input2"], &selection) != nil {
			t.Fatal("invalid typed fixture", i)
		}
		actual := snapshotkernel.Evaluate(before, after, selection)
		if !reflect.DeepEqual(actual, c.Expected[snapshotEntry]) {
			t.Fatalf("case %d differs from independent reference", i)
		}
	}
}

func TestSnapshotPublicComparisonScopes(t *testing.T) {
	tool := `{"name":"lookup","value":1}`
	for _, c := range []struct{ name, after, state string }{
		{"same", tool, "OBSERVED_DEFINITION_SAME"},
		{"changed", `{"name":"lookup","value":2}`, "OBSERVED_DEFINITION_CHANGED"},
		{"annotations", `{"name":"lookup","value":1,"annotations":null}`, "OBSERVED_ANNOTATIONS_CHANGED"},
		{"duplicate", tool + "," + tool, "AMBIGUOUS_NAME"},
	} {
		t.Run(c.name, func(t *testing.T) {
			input := snapshotFixture(tool, c.after)
			raw := encode(t, input)
			report, err := CompareDefinitionSnapshotJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			d := report.Decision
			if d.State != c.state || d.ContextID != report.Projection.ContextID || d.TargetName != "lookup" || report.Projection.InputJSON != string(raw) || len(report.Inputs) != 3 {
				t.Fatal("wrong decision or source binding", d)
			}
			if d.ComparisonScope != "SAVED_ROWS_ONLY" || d.OriginRelation != "NOT_ASSESSED" || d.AuthorizationRelation != "NOT_ASSESSED" || d.ExecutionAuthorized || d.BehaviorEquivalenceProven || d.LiveEffectPerformed || d.RemoteAtomicity != "UNKNOWN" {
				t.Fatal("comparison acquired unobserved authority", d)
			}
			if c.name == "duplicate" && (d.BeforeIndex != -1 || d.AfterIndex != -1 || d.BeforeDefinitionID != "" || d.AfterDefinitionID != "" || d.AfterProblemRowIndex != 1) {
				t.Fatal("duplicate observation selected a row", d)
			}
		})
	}
	input := snapshotFixture(tool, tool)
	for _, side := range []*SnapshotObservation{&input.Before, &input.After} {
		side.Pages[0].ResponseJSON = strings.Replace(side.Pages[0].ResponseJSON, "]}", `],"nextCursor":"remaining"}`, 1)
	}
	report, err := CompareDefinitionSnapshotJSON(encode(t, input))
	if err != nil || report.Decision.State != "OBSERVED_DEFINITION_SAME" || report.Decision.BeforePageCoverage != "INCOMPLETE" || report.Decision.AfterPageCoverage != "INCOMPLETE" {
		t.Fatal("partial observations hid a comparable selected row or acquired complete coverage", report.Decision, err)
	}
}

func TestSnapshotMissingPagesDoNotProveAbsence(t *testing.T) {
	input := snapshotFixture(`{"name":"lookup"}`, "")
	report, err := CompareDefinitionSnapshotJSON(encode(t, input))
	if err != nil || report.Decision.State != "OBSERVED_NAME_REMOVED" {
		t.Fatal(report.Decision, err)
	}
	input.After.Pages = []SnapshotPage{}
	report, err = CompareDefinitionSnapshotJSON(encode(t, input))
	if err != nil || report.Decision.State != "UNASSESSED" || report.Decision.AfterPageCoverage != "INCOMPLETE" {
		t.Fatal("no page was treated as a complete empty page", report.Decision, err)
	}
}

func TestSnapshotExactProtocolKeysCannotHideChange(t *testing.T) {
	input := snapshotFixture(`{"name":"lookup","value":1}`, `{"name":"lookup","value":2}`)
	for _, side := range []*SnapshotObservation{&input.Before, &input.After} {
		p := &side.Pages[0]
		p.ResponseJSON = strings.TrimSpace(p.ResponseJSON)
		p.ResponseJSON = strings.TrimSuffix(p.ResponseJSON, "}") + `,"Result":{"tools":[{"name":"other","value":0}]}}`
	}
	report, err := CompareDefinitionSnapshotJSON(encode(t, input))
	if err != nil || report.Decision.State != "OBSERVED_DEFINITION_CHANGED" || report.Decision.DefinitionRelation != "DIFFERENT_OBSERVED" {
		t.Fatal("shadow hid exact protocol definition change", report.Decision, err)
	}
}
