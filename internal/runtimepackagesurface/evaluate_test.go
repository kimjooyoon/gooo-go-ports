package runtimepackagesurface

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The frozen expectations are independent of the generated projection. Read the
// original corpus so a second copied fixture cannot silently drift.
func TestFrozenIndependentAssessments(t *testing.T) {
	raw, err := os.ReadFile("../../examples/runtime-package-surface/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Inputs   map[string]Observation `json:"inputs"`
			Expected map[string]Assessment  `json:"expected"`
		} `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != "gooo/body-composition-cases/v1" || len(fixture.Cases) != 14 {
		t.Fatal("the complete frozen fourteen-case corpus is required")
	}
	const entry = "example/runtime-package-surface:Inspect"
	for index, row := range fixture.Cases {
		input, hasInput := row.Inputs[entry]
		expected, hasExpected := row.Expected[entry]
		if !hasInput || !hasExpected || len(row.Inputs) != 1 || len(row.Expected) != 1 {
			t.Fatalf("case %d: unexpected entry", index)
		}
		actual := Evaluate(input)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("case %d: whole assessment differs: got %#v, want %#v", index, actual, expected)
		}
	}
}
