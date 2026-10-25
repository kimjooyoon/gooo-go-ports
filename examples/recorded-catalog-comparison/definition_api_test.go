package recordedcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/definitionkernel"
)

func TestDefinitionGeneratedKernelIndependentCases(t *testing.T) {
	raw, err := os.ReadFile("definition-kernel-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Inputs   map[string]json.RawMessage    `json:"inputs"`
			Expected map[string]DefinitionDecision `json:"expected"`
		} `json:"cases"`
	}
	if json.Unmarshal(raw, &corpus) != nil || corpus.Schema != "gooo/body-composition-cases/v1" || len(corpus.Cases) != 60 {
		t.Fatal("60 independent reference cases required")
	}
	for i, c := range corpus.Cases {
		var origin definitionkernel.OriginInput
		var collection definitionkernel.CollectionInput
		var binding DefinitionInput
		if json.Unmarshal(c.Inputs[definitionEntry+".input0"], &origin) != nil || json.Unmarshal(c.Inputs[definitionEntry+".input1"], &collection) != nil || json.Unmarshal(c.Inputs[definitionEntry+".input2"], &binding) != nil {
			t.Fatal("typed fixture", i)
		}
		actual := definitionkernel.Evaluate(origin, collection, binding)
		if !reflect.DeepEqual(actual, c.Expected[definitionEntry]) {
			t.Fatalf("case%d: generated result differs from independent Go reference", i)
		}
	}
}

func TestDefinitionPublicConsumersAndExport(t *testing.T) {
	raw, err := os.ReadFile("definition-consumer-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Schema string `json:"schema"`
		Kind   string `json:"evidence_kind"`
		Cases  []struct {
			Label    string `json:"label"`
			Input    Input  `json:"input"`
			Before   string `json:"before_policy_revision"`
			After    string `json:"after_policy_revision"`
			State    string `json:"expected_state"`
			Relation string `json:"expected_definition_relation"`
		} `json:"cases"`
	}
	if json.Unmarshal(raw, &corpus) != nil || corpus.Schema != "gooo/definition-binding-consumer-cases/v1" || corpus.Kind != "SYNTHETIC_FIXTURE" || len(corpus.Cases) != 12 {
		t.Fatal("12 labelled saved-format synthetic consumers required")
	}
	records := []map[string]any{}
	seen := map[string]bool{}
	for _, c := range corpus.Cases {
		if c.Label == "" || seen[c.Label] {
			t.Fatal("unique case label required")
		}
		seen[c.Label] = true
		input := encode(t, c.Input)
		report, err := CompareDefinitionJSON(input, c.Before, c.After)
		if err != nil {
			t.Fatal(c.Label, err)
		}
		decision := report.Decision
		if decision.State != c.State || decision.DefinitionRelation != c.Relation || decision.ExecutionAuthorized || decision.LiveEffectPerformed || decision.BehaviorEquivalenceProven || decision.RemoteAtomicity != "UNKNOWN" || !decision.ContainsSyntheticFixture {
			t.Fatal("consumer state/authority", c.Label, decision)
		}
		binding := report.Projection.Inputs[definitionEntry+".input2"].(DefinitionInput)
		if decision.ContextID != binding.ContextID || decision.OriginContextID != binding.AliasContextID || decision.CollectionContextID != binding.CollectionContextID {
			t.Fatal("consumer context binding")
		}
		records = append(records, map[string]any{"label": c.Label, "input": c.Input, "before_policy_revision": c.Before, "after_policy_revision": c.After, "expected_state": c.State, "expected_definition_relation": c.Relation, "raw_input_json": string(input), "report": report})
	}
	if target := os.Getenv("GOOO_DEFINITION_EXPORT"); target != "" {
		abs, err := filepath.Abs(target)
		if err != nil || abs != target {
			t.Fatal("absolute evidence export required")
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Dir(filepath.Dir(cwd))
		rel, err := filepath.Rel(root, abs)
		if err != nil || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("evidence export must be outside checkout")
		}
		f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(append(encode(t, records), '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
}
