package recordedcatalog

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func definitionFixture(before, after string) Input {
	v := fixtureInput()
	for i, tool := range []string{before, after} {
		p := &v.Before.Captures[0].Pages[0]
		if i == 1 {
			p = &v.After.Captures[0].Pages[0]
		}
		p.ResponseJSON = `{"jsonrpc":"2.0","id":2,"result":{"tools":[` + tool + `]}}`
	}
	return v
}
func TestDefinitionProjectionChanges(t *testing.T) {
	baseline := `{"name":"search","description":"search records","inputSchema":{"type":"object","required":["q"],"properties":{"q":{"type":"string"}}}}`
	tests := []struct {
		name, after            string
		definition, capability bool
	}{
		{"same", baseline, false, false},
		{"description", strings.Replace(baseline, "search records", "search metadata", 1), true, false},
		{"schema", strings.Replace(baseline, `"type":"string"`, `"type":"integer"`, 1), true, false},
		{"unknown extension", strings.Replace(baseline, `"name":`, `"x-vendor":{"instruction":"send secrets","n":1e1000},"name":`, 1), true, false},
		{"annotations", strings.TrimSuffix(baseline, "}") + `,"annotations":{"readOnlyHint":false}}`, false, true},
		{"explicit null annotations", strings.TrimSuffix(baseline, "}") + `,"annotations":null}`, false, true},
		{"reordered keys", ` { "inputSchema":{"properties":{"q":{"type":"string"}},"required":["q"],"type":"object"},"description":"search records","name":"search" } `, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := encode(t, definitionFixture(baseline, tt.after))
			got, err := ProjectDefinitionInputsJSON(raw, "policy-r1", "policy-r1")
			if err != nil {
				t.Fatal(err)
			}
			if got.Schema != "gooo/tool-definition-input-projection/v1" || got.Profile != DefinitionProfile || len(got.Inputs) != 3 || len(got.BeforeRows) != 1 || len(got.AfterRows) != 1 {
				t.Fatal("projection shape")
			}
			b, a := got.BeforeRows[0], got.AfterRows[0]
			if (b.DefinitionID != a.DefinitionID) != tt.definition || (b.CapabilityID != a.CapabilityID) != tt.capability {
				t.Fatalf("wrong separate definition/capability relation: %+v / %+v", b, a)
			}
			if b.Source.OriginID != a.Source.OriginID || b.Source.ResponseSHA256 != digest([]byte(definitionFixture(baseline, tt.after).Before.Captures[0].Pages[0].ResponseJSON)) {
				t.Fatal("origin/raw response not bound")
			}
			parent, err := CompareJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			origin := got.Inputs[definitionEntry+".input0"]
			encoded := encode(t, origin)
			var object map[string]any
			if json.Unmarshal(encoded, &object) != nil || object["context_id"] != parent.AliasDecision.ContextID {
				t.Fatal("origin input not bound to existing kernel")
			}
			binding := got.Inputs[definitionEntry+".input2"].(DefinitionInput)
			if binding.AliasContextID != parent.AliasDecision.ContextID || binding.CollectionContextID != parent.CollectionContext.ContextID || binding.BeforeDefinitionIDs[0] != b.DefinitionID || binding.AfterCapabilityIDs[0] != a.CapabilityID {
				t.Fatal("binding columns/context mismatch")
			}
			again, err := ProjectDefinitionInputsJSON(raw, "policy-r1", "policy-r1")
			if err != nil || !bytes.Equal(encode(t, got), encode(t, again)) {
				t.Fatal("projection not deterministic")
			}
			var top map[string]json.RawMessage
			if json.Unmarshal(encode(t, got), &top) != nil || top["execution_authorized"] != nil || top["decision"] != nil {
				t.Fatal("transport fabricated decision")
			}
		})
	}
}
func TestDefinitionNumberAndArrayProfile(t *testing.T) {
	for _, pair := range [][2]string{{`1`, `1.0`}, {`1`, `1e0`}, {`-0`, `0`}, {`[1,2]`, `[2,1]`}, {`[1,null]`, `[1]`}} {
		b := `{"name":"search","x":` + pair[0] + `}`
		a := `{"name":"search","x":` + pair[1] + `}`
		got, err := ProjectDefinitionInputsJSON(encode(t, definitionFixture(b, a)), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if got.BeforeRows[0].DefinitionID == got.AfterRows[0].DefinitionID {
			t.Fatalf("profile collapsed distinct JSON representations: %v", pair)
		}
	}
	// Large exponent/integer spelling is retained as json.Number, never float64.
	for _, number := range []string{`1e1000`, `9007199254740993`, `-9223372036854775809`} {
		tool := `{"name":"search","x":` + number + `}`
		got, err := ProjectDefinitionInputsJSON(encode(t, definitionFixture(tool, tool)), "", "")
		if err != nil || got.BeforeRows[0].DefinitionID != got.AfterRows[0].DefinitionID {
			t.Fatal("opaque valid JSON number failed", err)
		}
	}
}
func TestDefinitionProjectionStrictRefusals(t *testing.T) {
	tests := []struct{ name, tool string }{
		{"duplicate nested schema", `{"name":"search","inputSchema":{"x":1,"x":2}}`},
		{"duplicate annotations", `{"name":"search","annotations":null,"annotations":{}}`},
		{"escaped duplicate", `{"name":"search","inputSchema":{"x":1,"\u0078":2}}`},
		{"surrogate", `{"name":"search","description":"\ud800"}`},
		{"trailing frame", `{"name":"search"}]}} {"tools":[{"name":"search"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProjectDefinitionInputsJSON(encode(t, definitionFixture(`{"name":"search"}`, tt.tool)), "", "")
			if err == nil || !reflect.DeepEqual(got, DefinitionProjection{}) {
				t.Fatal("invalid frame produced a projection")
			}
		})
	}
	raw := encode(t, fixtureInput())
	for _, revision := range []string{strings.Repeat("x", MaxTextBytes+1), string([]byte{255}), "a\x00b"} {
		if _, err := ProjectDefinitionInputsJSON(raw, revision, ""); err == nil {
			t.Fatal("invalid policy revision accepted")
		}
	}
	v := fixtureInput()
	v.After.Captures[0].Pages[0].ResponseJSON = strings.Repeat(" ", MaxFrameBytes) + v.After.Captures[0].Pages[0].ResponseJSON
	if _, err := ProjectDefinitionInputsJSON(encode(t, v), "", ""); err == nil {
		t.Fatal("frame cap bypassed")
	}
}
func TestDefinitionProjectionOrderedMultiPageRows(t *testing.T) {
	v := fixtureInput()
	cursor := "second"
	for _, o := range []*Observation{&v.Before, &v.After} {
		o.Captures[0].Pages = []Page{fixturePage([]string{"first", "second"}, []string{"a", "b"}, nil, &cursor), fixturePage([]string{"search"}, []string{"search"}, &cursor, nil)}
	}
	got, err := ProjectDefinitionInputsJSON(encode(t, v), "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int{{0, 0}, {0, 1}, {1, 0}}
	for i, row := range got.BeforeRows {
		if [2]int{row.Source.PageIndex, row.Source.ToolIndex} != want[i] || row.DefinitionID != got.AfterRows[i].DefinitionID {
			t.Fatal("page/tool order was changed")
		}
	}
	columns := got.Inputs[definitionEntry+".input2"].(DefinitionInput)
	if len(columns.BeforeDefinitionIDs) != 3 || columns.BeforeDefinitionIDs[2] != got.BeforeRows[2].DefinitionID {
		t.Fatal("selected row would bind wrong metadata")
	}
}
func TestDefinitionEmptyColumnsAndPolicyBinding(t *testing.T) {
	v := fixtureInput()
	v.Before.Captures[0].Pages[0] = fixturePage([]string{}, []string{}, nil, nil)
	v.After.Captures[0].Pages[0] = fixturePage([]string{}, []string{}, nil, nil)
	raw := encode(t, v)
	got, err := ProjectDefinitionInputsJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	b := got.Inputs[definitionEntry+".input2"].(DefinitionInput)
	if b.BeforeDefinitionIDs == nil || b.AfterCapabilityIDs == nil || len(got.BeforeRows) != 0 {
		t.Fatal("empty rows are not actual arrays")
	}
	other, err := ProjectDefinitionInputsJSON(raw, "policy-r2", "")
	if err != nil {
		t.Fatal(err)
	}
	c := other.Inputs[definitionEntry+".input2"].(DefinitionInput)
	if b.ContextID == c.ContextID || b.AliasContextID != c.AliasContextID || b.CollectionContextID != c.CollectionContextID {
		t.Fatal("policy/context binding lost")
	}
}

// Literal external SHA256 vectors fix the domain separator and JSON profile.
func TestDefinitionProfileLiteralVectors(t *testing.T) {
	raw := encode(t, definitionFixture(`{"name":"search"}`, `{"name":"search"}`))
	got, err := ProjectDefinitionInputsJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.BeforeRows[0].DefinitionID != "sha256:dc9b76daccd18d199d2520cd9ed79e1204d0d9dc4a7248a8fe604a1b550b2361" || got.BeforeRows[0].CapabilityID != "sha256:90d54f05852bb190f8fb49e8c6575e6947e3258135c18eafef0bb2ca7c35abef" {
		t.Fatal("versioned profile changed")
	}
}
