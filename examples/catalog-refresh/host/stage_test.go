// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package catalogrefresh

import (
	"context"
	"strings"
	"testing"

	recordedcatalog "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func stageDescriptor(t *testing.T, descriptor string) *Candidate {
	t.Helper()
	r := newMechanicsRegistry(t, mechanicsEligible)
	a, err := r.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page := syntheticPage("")
	page.ResponseJSON = `{"jsonrpc":"2.0","id":1,"result":{"tools":[` + descriptor + `]}}`
	candidate, err := a.Stage([]SnapshotPage{page}, "SUCCESS")
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func TestStagingShapeProfileObservesAllRowsWithoutInterpretingSchema(t *testing.T) {
	for _, item := range []struct{ name, descriptor, state string }{
		{"required shape", `{"name":"x","inputSchema":{"type":"object"}}`, "ACCEPTED"},
		{"missing exact schema", `{"name":"x","InputSchema":{"type":"object"}}`, "REJECTED"},
		{"null schema", `{"name":"x","inputSchema":null}`, "REJECTED"},
		{"wrong schema type", `{"name":"x","inputSchema":{"type":"array"}}`, "REJECTED"},
		{"case variant type", `{"name":"x","inputSchema":{"Type":"object"}}`, "REJECTED"},
		{"optional output", `{"name":"x","inputSchema":{"type":"object"},"outputSchema":{"type":"object"}}`, "ACCEPTED"},
		{"bad output", `{"name":"x","inputSchema":{"type":"object"},"outputSchema":false}`, "REJECTED"},
		{"bad title", `{"name":"x","inputSchema":{"type":"object"},"title":null}`, "REJECTED"},
		{"long description", `{"name":"x","inputSchema":{"type":"object"},"description":"` + strings.Repeat("a", 1025) + `"}`, "REJECTED"},
		{"null annotations", `{"name":"x","inputSchema":{"type":"object"},"annotations":null}`, "REJECTED"},
		{"boolean hints", `{"name":"x","inputSchema":{"type":"object"},"annotations":{"readOnlyHint":true,"destructiveHint":false}}`, "ACCEPTED"},
		{"nonboolean hint", `{"name":"x","inputSchema":{"type":"object"},"annotations":{"readOnlyHint":"true"}}`, "REJECTED"},
		{"unknown uninterpreted", `{"name":"x","inputSchema":{"type":"object","not-a-supported-keyword":[17]},"unknown":false,"annotations":{"unknownHint":[1,2]}}`, "ACCEPTED"},
	} {
		t.Run(item.name, func(t *testing.T) {
			candidate := stageDescriptor(t, item.descriptor)
			facts, evidence := candidate.Inputs()
			if len(evidence.RowValidationStates) != 1 || evidence.RowValidationStates[0] != item.state || evidence.RowDefinitionIDs[0] != facts.DefinitionIDs[0] {
				t.Fatalf("incorrect shape/binding observation: %+v", evidence)
			}
		})
	}
	// An invalid second row is retained with its own bound outcome, never omitted
	// merely because an earlier selected-name descriptor would be unchanged.
	candidate := stageDescriptor(t, `{"name":"first","inputSchema":{"type":"object"}},{"name":"second","inputSchema":false}`)
	facts, evidence := candidate.Inputs()
	if len(facts.OriginalNames) != 2 || len(evidence.RowValidationStates) != 2 || evidence.RowValidationStates[0] != "ACCEPTED" || evidence.RowValidationStates[1] != "REJECTED" || evidence.RowDefinitionIDs[1] != facts.DefinitionIDs[1] {
		t.Fatal("nonselected rejected row was omitted")
	}
}

func TestStagingExactProtocolMembersAndOpaqueValuesRemainBound(t *testing.T) {
	clean := stageDescriptor(t, `{"name":"x","inputSchema":{"type":"object"},"opaque":1}`)
	changed := stageDescriptor(t, `{"name":"x","inputSchema":{"type":"object"},"opaque":2}`)
	f0, _ := clean.Inputs()
	f1, _ := changed.Inputs()
	if f0.DefinitionIDs[0] == f1.DefinitionIDs[0] {
		t.Fatal("opaque descriptor field disappeared from identity")
	}
	r := newMechanicsRegistry(t, mechanicsEligible)
	a, err := r.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page := syntheticPage("x")
	page.ResponseJSON = `{"jsonrpc":"2.0","id":1,"Result":{"Tools":[{"name":"shadow"}]},"result":{"Tools":[{"name":"shadow"}],"tools":[{"name":"x","inputSchema":{"type":"object"},"InputSchema":false}]}}`
	candidate, err := a.Stage([]SnapshotPage{page}, "SUCCESS")
	if err != nil {
		t.Fatal(err)
	}
	facts, evidence := candidate.Inputs()
	if len(facts.OriginalNames) != 1 || facts.OriginalNames[0] != "x" || evidence.RowValidationStates[0] != "ACCEPTED" {
		t.Fatal("case-variant keys replaced exact protocol members")
	}
	annotationA := stageDescriptor(t, `{"name":"x","inputSchema":{"type":"object"},"annotations":{"unknown":1}}`)
	annotationB := stageDescriptor(t, `{"name":"x","inputSchema":{"type":"object"},"annotations":{"unknown":2}}`)
	fa, _ := annotationA.Inputs()
	fb, _ := annotationB.Inputs()
	if fa.AnnotationIDs[0] == fb.AnnotationIDs[0] || annotationA.ID() == annotationB.ID() {
		t.Fatal("unknown annotation was not retained in binding")
	}
}

func TestStagingRefusesAmbiguousOrOversizedSavedBytes(t *testing.T) {
	for _, response := range []string{
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"x","name":"y"}]}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"x","inputSchema":{"type":"object","type":"array"}}]}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"x","opaque":"\ud800"}]}}`,
		strings.Repeat(" ", 65537),
	} {
		r := newMechanicsRegistry(t, mechanicsEligible)
		a, err := r.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		page := syntheticPage("x")
		page.ResponseJSON = response
		before := r.Snapshot()
		requireRetained(t, r.Refresh(a, []SnapshotPage{page}, "SUCCESS"), "STAGING_FAILED")
		requireUnchanged(t, before, r.Snapshot())
	}
}

func TestMechanicsRecentReceiptJournalHasBoundedMemory(t *testing.T) {
	r := newMechanicsRegistry(t, mechanicsEligible)
	for range 300 {
		r.Commit(nil)
	}
	receipts := r.Receipts()
	if len(receipts) != 256 || r.DroppedReceipts() != 44 || receipts[0].Sequence != 45 || receipts[255].Sequence != 300 {
		t.Fatalf("unbounded or incorrect journal: len=%d dropped=%d", len(receipts), r.DroppedReceipts())
	}
}

func TestStagingRejectsInvalidUTF8BeforeOuterJSONCanReplaceBytes(t *testing.T) {
	for _, side := range []string{"request", "response"} {
		t.Run(side, func(t *testing.T) {
			called := false
			r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
				called = true
				return mechanicsEligible(f, e, s)
			})
			a, err := r.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			page := syntheticPage("x")
			// Each frame has otherwise valid JSON syntax. Only the original
			// byte inside a JSON string is invalid; normalizing it would hide
			// the failure and let the synthetic successful response proceed.
			if side == "request" {
				page.RequestJSON = "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"_meta\":{\"note\":\"\xff\"}}"
			} else {
				page.ResponseJSON = "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"x\",\"inputSchema\":{\"type\":\"object\"},\"description\":\"\xff\"}]}}"
			}
			before := r.Snapshot()
			receipt := r.Refresh(a, []SnapshotPage{page}, "SUCCESS")
			requireRetained(t, receipt, "STAGING_FAILED")
			requireUnchanged(t, before, r.Snapshot())
			if called || receipt.Decision != nil || receipt.Error != "saved frame contains invalid UTF-8" {
				t.Fatalf("malformed original bytes reached evaluator or wrong refusal: %+v", receipt)
			}
		})
	}
}

func TestStagingBoundsRawFramesBeforeUTF8AndOuterJSON(t *testing.T) {
	for _, side := range []string{"request", "response"} {
		t.Run(side, func(t *testing.T) {
			calls := 0
			r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
				calls++
				return mechanicsEligible(f, e, s)
			})
			a, err := r.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			page := syntheticPage("x")
			frame := &page.RequestJSON
			if side == "response" {
				frame = &page.ResponseJSON
			}
			// Whitespace padding preserves valid frame syntax; the boundary
			// alone determines whether staging can reach the strict projector.
			*frame += strings.Repeat(" ", recordedcatalog.MaxFrameBytes-len(*frame))
			if _, err := a.Stage([]SnapshotPage{page}, "SUCCESS"); err != nil {
				t.Fatalf("exact 64 KiB frame unexpectedly rejected: %v", err)
			}
			*frame += " "
			if len(*frame) != recordedcatalog.MaxFrameBytes+1 {
				t.Fatal("wrong test boundary")
			}
			before := r.Snapshot()
			for _, invalidUTF8 := range []bool{false, true} {
				if invalidUTF8 {
					*frame = "\xff" + (*frame)[1:]
				}
				receipt := r.Refresh(a, []SnapshotPage{page}, "SUCCESS")
				requireRetained(t, receipt, "STAGING_FAILED")
				requireUnchanged(t, before, r.Snapshot())
				if calls != 0 || receipt.Decision != nil || receipt.Error != "saved frame exceeds byte limit" {
					t.Fatalf("oversized raw frame reached evaluator or later validation: %+v", receipt)
				}
			}
		})
	}
}
