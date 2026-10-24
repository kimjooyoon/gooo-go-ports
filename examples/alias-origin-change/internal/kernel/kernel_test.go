package kernel

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestGeneratedKernelAgainstAllOriginal111IndependentOracleCases(t *testing.T) {
	raw, err := os.ReadFile("../../kernel.cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Inputs   map[string]Input    `json:"inputs"`
			Expected map[string]Decision `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 111 {
		t.Fatal("original scope changed")
	}
	for i, c := range doc.Cases {
		got := Evaluate(c.Inputs["gooo/tool-origin:CompareAliasOrigin"])
		want := c.Expected["gooo/tool-origin:CompareAliasOrigin"]
		if !reflect.DeepEqual(got, want) || got.ExecutionAuthorized {
			t.Fatalf("case%d differs: got%+v want%+v", i, got, want)
		}
	}
}
