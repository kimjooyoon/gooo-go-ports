// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import "testing"

// Case variants are distinct JSON keys. They must never replace the exact
// protocol members whose tool names and source locations the transport read.
func TestDefinitionProjectionUsesExactProtocolKeys(t *testing.T) {
	before := `{"name":"search","value":1,"annotations":{"readOnlyHint":true}}`
	after := `{"name":"search","value":2,"annotations":{"readOnlyHint":false}}`
	shadow := `{"name":"unrelated","value":99,"annotations":null}`
	clean, err := ProjectDefinitionInputsJSON(encode(t, definitionFixture(before, after)), "", "")
	if err != nil {
		t.Fatal(err)
	}
	variants := []struct {
		name  string
		frame func(string) string
	}{
		{"Result after result", func(tool string) string {
			return `{"jsonrpc":"2.0","id":2,"result":{"tools":[` + tool + `]},"Result":{"tools":[` + shadow + `]}}`
		}},
		{"Result before result", func(tool string) string {
			return `{"jsonrpc":"2.0","id":2,"Result":{"tools":[` + shadow + `]},"result":{"tools":[` + tool + `]}}`
		}},
		{"Tools after tools", func(tool string) string {
			return `{"jsonrpc":"2.0","id":2,"result":{"tools":[` + tool + `],"Tools":[` + shadow + `]}}`
		}},
		{"Tools before tools", func(tool string) string {
			return `{"jsonrpc":"2.0","id":2,"result":{"Tools":[` + shadow + `],"tools":[` + tool + `]}}`
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			input := definitionFixture(before, after)
			input.Before.Captures[0].Pages[0].ResponseJSON = variant.frame(before)
			input.After.Captures[0].Pages[0].ResponseJSON = variant.frame(after)
			got, err := ProjectDefinitionInputsJSON(encode(t, input), "", "")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.BeforeRows) != 1 || len(got.AfterRows) != 1 ||
				got.BeforeRows[0].DefinitionID != clean.BeforeRows[0].DefinitionID ||
				got.AfterRows[0].DefinitionID != clean.AfterRows[0].DefinitionID ||
				got.BeforeRows[0].CapabilityID != clean.BeforeRows[0].CapabilityID ||
				got.AfterRows[0].CapabilityID != clean.AfterRows[0].CapabilityID ||
				got.BeforeRows[0].DefinitionID == got.AfterRows[0].DefinitionID {
				t.Fatal("case-variant protocol keys changed the selected definition or hid its change")
			}
			if got.BeforeRows[0].Source.ResponseSHA256 != digest([]byte(input.Before.Captures[0].Pages[0].ResponseJSON)) {
				t.Fatal("exact original response binding lost")
			}
		})
	}
}
