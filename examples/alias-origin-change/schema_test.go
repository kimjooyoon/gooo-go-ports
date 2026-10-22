// Copyright (c) 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package aliasorigin

import (
	"encoding/json"
	"github.com/google/jsonschema-go/jsonschema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resolveSchema(t *testing.T, name string) *jsonschema.Resolved {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("schemas", name+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	r, err := s.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func schemaInstance(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestSchemasCoverAllActualAPIDecisionStatesWithoutAuthority(t *testing.T) {
	input, output := resolveSchema(t, "input"), resolveSchema(t, "output")
	base := sample()
	same := sample()
	same.After = append([]Entry(nil), same.Before...)
	removed := sample()
	removed.After = []Entry{}
	added := sample()
	added.Before = []Entry{}
	absent := Input{TargetAlias: "missing", Before: []Entry{}, After: []Entry{}}
	bad := sample()
	bad.TargetAlias = ""
	duplicate := sample()
	duplicate.After = append(duplicate.After, duplicate.After[0])
	for _, tc := range []struct {
		in    Input
		state string
	}{{base, "ORIGIN_CHANGED"}, {same, "UNCHANGED"}, {removed, "REMOVED"}, {added, "ADDED"}, {absent, "ABSENT"}, {bad, "INVALID_INPUT"}, {duplicate, "AMBIGUOUS"}} {
		t.Run(tc.state, func(t *testing.T) {
			if err := input.Validate(schemaInstance(t, tc.in)); err != nil {
				t.Fatal(err)
			}
			report, err := Compare(tc.in)
			if err != nil || report.Decision.State != tc.state {
				t.Fatal("wrong actual state", report.Decision, err)
			}
			if err := output.Validate(schemaInstance(t, report)); err != nil {
				t.Fatal(err)
			}
			altered := schemaInstance(t, report).(map[string]any)
			altered["decision"].(map[string]any)["execution_authorized"] = true
			if output.Validate(altered) == nil {
				t.Fatal("schema accepted authority")
			}
		})
	}
}
func TestSchemasPreserveByteParserAndCorrespondenceBoundaries(t *testing.T) {
	input, output := resolveSchema(t, "input"), resolveSchema(t, "output")
	in := sample()
	in.After[0].OriginalName = strings.Repeat("😀", 300)
	if err := input.Validate(schemaInstance(t, in)); err != nil {
		t.Fatal("schema counts Unicode characters, not native UTF8 bytes", err)
	}
	if _, err := Compare(in); err == nil {
		t.Fatal("native1024byte transport bound lost")
	}
	report, err := Compare(sample())
	if err != nil {
		t.Fatal(err)
	}
	shape := schemaInstance(t, report).(map[string]any)
	shape["facts_sha256"] = "sha256:" + strings.Repeat("0", 64)
	if err := output.Validate(shape); err != nil {
		t.Fatal("shape schema should not claim hash/fact authentication", err)
	}
	wire := schemaInstance(t, sample()).(map[string]any)
	wire["execution_authorized"] = true
	if input.Validate(wire) == nil {
		t.Fatal("caller authority allowed")
	}
}
func TestDiagnosticSchemaCoversActualFailureAndRejectsWrongStage(t *testing.T) {
	diagnostic := resolveSchema(t, "diagnostic")
	_, err := CompareJSON([]byte(`{"target_alias":"x","before":null,"after":[]}`))
	p, ok := err.(*Problem)
	if !ok {
		t.Fatal("missing typed native transport diagnostic", err)
	}
	if err := diagnostic.Validate(schemaInstance(t, p)); err != nil {
		t.Fatal(err)
	}
	changed := *p
	changed.Stage = "evaluate"
	if diagnostic.Validate(schemaInstance(t, &changed)) == nil {
		t.Fatal("accepted wrong state/stage/code combination")
	}
}
