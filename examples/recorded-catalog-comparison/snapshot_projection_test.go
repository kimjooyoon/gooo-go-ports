// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// Synthetic protocol controls; they are not historical host observations.
func snapshotPageFixture(id, tool string) SnapshotPage {
	return SnapshotPage{
		RequestJSON:  "{\"jsonrpc\":\"2.0\",\"id\":" + id + ",\"method\":\"tools/list\"}\n",
		ResponseJSON: "{ \"jsonrpc\" : \"2.0\", \"id\" : " + id + ", \"result\" : {\"tools\":[" + tool + "]} }\r\n",
	}
}

func snapshotFixture(before, after string) DefinitionSnapshotInput {
	return DefinitionSnapshotInput{TargetName: "lookup",
		Before: SnapshotObservation{Pages: []SnapshotPage{snapshotPageFixture(`"old"`, before)}},
		After:  SnapshotObservation{Pages: []SnapshotPage{snapshotPageFixture(`"new"`, after)}}}
}

func TestSnapshotProjectionWithoutHostMapping(t *testing.T) {
	tool := `{"name":"lookup","inputSchema":{"type":"object"},"annotations":{"readOnlyHint":true}}`
	input := snapshotFixture(tool, tool)
	raw := encode(t, input)
	got, err := ProjectDefinitionSnapshotInputsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.InputJSON != string(raw) || got.InputSHA256 != digest(raw) || got.TargetName != "lookup" {
		t.Fatal("original input was not retained")
	}
	b, a := got.Before, got.After
	if b.Rows[0].DefinitionID != a.Rows[0].DefinitionID || b.Rows[0].AnnotationID != a.Rows[0].AnnotationID {
		t.Fatal("request IDs or framing leaked into definition identity")
	}
	if b.Pages[0].ResponseSHA256 == a.Pages[0].ResponseSHA256 || b.Pages[0].ResponseSHA256 != digest([]byte(input.Before.Pages[0].ResponseJSON)) {
		t.Fatal("different original payloads lost their distinct byte bindings")
	}
	if b.Rows[0].Source.PageIndex != 0 || b.Rows[0].Source.ToolIndex != 0 || b.Fields.RowPageIndexes[0] != 0 {
		t.Fatal("ordered source locations lost")
	}
	var projected map[string]json.RawMessage
	if json.Unmarshal(encode(t, got), &projected) != nil || projected["decision"] != nil || projected["execution_authorized"] != nil {
		t.Fatal("transport projection acquired a decision")
	}
	var source map[string]json.RawMessage
	if json.Unmarshal(encode(t, b.Rows[0].Source), &source) != nil || len(source) != 3 || source["origin_id"] != nil {
		t.Fatal("snapshot projection invented host origin")
	}
}

func TestSnapshotDefinitionProfileRetainsJSONDistinctions(t *testing.T) {
	for _, pair := range [][2]string{{`1`, `1.0`}, {`1`, `1e0`}, {`-0`, `0`}, {`[1,2]`, `[2,1]`}} {
		b, a := `{"name":"lookup","value":`+pair[0]+`}`, `{"name":"lookup","value":`+pair[1]+`}`
		got, err := ProjectDefinitionSnapshotInputsJSON(encode(t, snapshotFixture(b, a)))
		if err != nil || got.Before.Rows[0].DefinitionID == got.After.Rows[0].DefinitionID {
			t.Fatalf("numeric spelling or array order collapsed: %v %v", pair, err)
		}
	}
	for _, number := range []string{`1e1000`, `9007199254740993`} {
		tool := `{"name":"lookup","value":` + number + `}`
		got, err := ProjectDefinitionSnapshotInputsJSON(encode(t, snapshotFixture(tool, tool)))
		if err != nil || got.Before.Rows[0].DefinitionID != got.After.Rows[0].DefinitionID {
			t.Fatal("opaque number was converted through floating point", err)
		}
	}
	got, err := ProjectDefinitionSnapshotInputsJSON(encode(t, snapshotFixture(`{"name":"lookup"}`, `{"name":"lookup","annotations":null}`)))
	if err != nil || got.Before.Rows[0].DefinitionID != got.After.Rows[0].DefinitionID || got.Before.Rows[0].AnnotationID == got.After.Rows[0].AnnotationID {
		t.Fatal("absent/null annotations collapsed into descriptor identity", err)
	}
}

func TestSnapshotSharesExistingProjectionProfile(t *testing.T) {
	tool := `{"name":"lookup","value":1e0,"annotations":{"readOnlyHint":true}}`
	got, err := ProjectDefinitionSnapshotInputsJSON(encode(t, snapshotFixture(tool, tool)))
	if err != nil {
		t.Fatal(err)
	}
	existing, err := ProjectDefinitionInputsJSON(encode(t, definitionFixture(tool, tool)), "", "")
	if err != nil || got.Before.Rows[0].DefinitionID != existing.BeforeRows[0].DefinitionID || got.Before.Rows[0].AnnotationID != existing.BeforeRows[0].CapabilityID {
		t.Fatal("new path silently changed the shared definition profile", err)
	}
}

func TestSnapshotPreservesCursorAbsenceNullEmptyAndDuplicates(t *testing.T) {
	page := snapshotPageFixture(`1`, `{"name":"lookup"},{"name":"lookup"}`)
	for _, next := range []string{"", `,"nextCursor":null`, `,"nextCursor":""`} {
		p := page
		p.ResponseJSON = `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"lookup"},{"name":"lookup"}]` + next + `}}`
		got, err := ProjectDefinitionSnapshotInputsJSON(encode(t, DefinitionSnapshotInput{TargetName: "lookup", Before: SnapshotObservation{Pages: []SnapshotPage{p}}, After: SnapshotObservation{Pages: []SnapshotPage{}}}))
		if err != nil || len(got.Before.Rows) != 2 || len(got.After.Pages) != 0 || got.After.Fields.OriginalNames == nil {
			t.Fatal("observed rows deduplicated or absent pages fabricated", err)
		}
		want := map[string]string{"": "ABSENT", `,"nextCursor":null`: "NULL", `,"nextCursor":""`: "STRING"}[next]
		if got.Before.Pages[0].NextCursor.Tag != want {
			t.Fatalf("cursor kind collapsed: %s", next)
		}
	}
}

func TestSnapshotRetainsExistingExposureErrorPrecedence(t *testing.T) {
	page := snapshotPageFixture(`1`, `{"name":12}`)
	_, oldErr := decodePage(Page{RequestJSON: page.RequestJSON, ResponseJSON: page.ResponseJSON, DeclaredExposedNames: []string{}})
	_, newErr := decodeSavedPage(page.RequestJSON, page.ResponseJSON, 0, false)
	var oldProblem, newProblem *Problem
	if !errors.As(oldErr, &oldProblem) || oldProblem.Code != "EXPOSURE_COUNT_MISMATCH" || !errors.As(newErr, &newProblem) || newProblem.Code != "RPC_STRING_REQUIRED" {
		t.Fatalf("old host precedence changed or snapshot invented aliases: %v %v", oldErr, newErr)
	}
}

func TestSnapshotStrictFailuresPreserveInput(t *testing.T) {
	input := snapshotFixture(`{"name":"lookup"}`, `{"name":"lookup"}`)
	cases := []struct {
		name string
		edit func(*DefinitionSnapshotInput)
		code string
	}{
		{"duplicate descriptor key", func(x *DefinitionSnapshotInput) {
			x.Before.Pages[0].ResponseJSON = `{"jsonrpc":"2.0","id":"old","result":{"tools":[{"name":"lookup","name":"other"}]}}`
		}, "DUPLICATE_OR_INVALID_JSON_KEY"},
		{"typed ID mismatch", func(x *DefinitionSnapshotInput) {
			x.Before.Pages[0].RequestJSON = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
		}, "RPC_ID_MISMATCH"},
		{"null request cursor", func(x *DefinitionSnapshotInput) {
			x.Before.Pages[0].RequestJSON = `{"jsonrpc":"2.0","id":"old","method":"tools/list","params":{"cursor":null}}`
		}, "NULL_REQUEST_CURSOR"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			x := snapshotFixture(`{"name":"lookup"}`, `{"name":"lookup"}`)
			test.edit(&x)
			raw := encode(t, x)
			copy := bytes.Clone(raw)
			_, err := ProjectDefinitionSnapshotInputsJSON(raw)
			var problem *Problem
			if !errors.As(err, &problem) || problem.Code != test.code || !bytes.Equal(raw, copy) {
				t.Fatalf("wrong failure or changed input: %v", err)
			}
		})
	}
	raw := encode(t, input)
	raw = append([]byte(`{"host_id":"invented",`), raw[1:]...)
	if _, err := ProjectDefinitionSnapshotInputsJSON(raw); err == nil {
		t.Fatal("new input accepted an undeclared host field")
	}
}
