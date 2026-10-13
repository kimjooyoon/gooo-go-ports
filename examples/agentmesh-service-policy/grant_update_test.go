package servicepolicy

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGrantUpdateNativeMatchesIndependentWorkspaceCases(t *testing.T) {
	data, err := os.ReadFile("grant-update-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]string              `json:"inputs"`
			Expected map[string]GrantUpdateDecision `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	const key = "gooo/service-policy/conditional-apply:AssessServiceGrantUpdate"
	if len(corpus.Cases) != 21 {
		t.Fatalf("expected21independent scenarios, got%d", len(corpus.Cases))
	}
	for i, c := range corpus.Cases {
		f := GrantUpdateFacts{c.Inputs[key+".input0"], c.Inputs[key+".input1"], c.Inputs[key+".input2"], c.Inputs[key+".input3"]}
		got, err := EvaluateGrantUpdate(f)
		if err != nil || got.Facts != f || got.Decision != c.Expected[key] {
			t.Errorf("case%d: got%+v err%v want%+v", i, got, err, c.Expected[key])
		}
	}
}

func TestGrantUpdateInvalidGeneratedResultSchema(t *testing.T) {
	valid := GrantUpdateDecision{"ELIGIBLE", "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", "", true, true, true, "OLD_GLOBAL"}
	for name, edit := range map[string]func(map[string]any){
		"missing":             func(v map[string]any) { delete(v, "required_observed") },
		"extra":               func(v map[string]any) { v["extra"] = true },
		"null_boolean":        func(v map[string]any) { v["required_before_match"] = nil },
		"string_boolean":      func(v map[string]any) { v["required_before_match"] = "true" },
		"unknown_status":      func(v map[string]any) { v["status"] = "APPROVED" },
		"unknown_reason":      func(v map[string]any) { v["reason"] = "ALLOW" },
		"unobserved_eligible": func(v map[string]any) { v["required_observed"] = false },
		"required_removed":    func(v map[string]any) { v["required_after_match"] = false },
		"unknown_rule":        func(v map[string]any) { v["subset_rule_id"] = "GUESS" },
		"empty_rule_eligible": func(v map[string]any) { v["subset_rule_id"] = "CANDIDATE_EMPTY" },
	} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(valid)
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			edit(v)
			if _, err := decodeGrantUpdateDecision(v); err == nil {
				t.Fatal("malformed generated result accepted")
			}
		})
	}
}

func TestGrantUpdateInvalidUTF8NeverRepairedIntoFacts(t *testing.T) {
	_, err := EvaluateGrantUpdate(GrantUpdateFacts{Before: "*", After: "mcp://a", RequiredType: "mcp", RequiredName: string([]byte{0xff})})
	if err == nil {
		t.Fatal("lossy UTF8 transport accepted")
	}
}

// These mutations rehash modified assets coherently. They must fail semantic
// source/entry/record binding checks, rather than only the outer asset hash.
func TestGrantUpdateRehashedBoundaryBindingsRefuseSubstitution(t *testing.T) {
	for _, mutation := range []string{"workspace-to-raw", "source", "impact-to-raw", "refinement-to-raw", "entry", "field-id", "field-type", "producer", "projection", "host-asset-omission"} {
		t.Run(mutation, func(t *testing.T) {
			files := map[string][]byte{}
			for _, name := range boundNames {
				raw, err := assets.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				files[name] = raw
			}
			manifest, err := assets.ReadFile("generation.json")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := checkBindings(files, manifest); err != nil {
				t.Fatal("unmodified fixture must pass", err)
			}
			var g generation
			if err := json.Unmarshal(manifest, &g); err != nil {
				t.Fatal(err)
			}
			target := "receipts/conditional-apply-program.json"
			switch mutation {
			case "workspace-to-raw":
				target = "conditional-apply-workspace.json"
				files[target] = append([]byte(nil), files["impact-raw-workspace.json"]...)
				g.PublicSources["service-grant-apply.workspace.json"] = digest(files[target])
			case "source":
				target = "conditional_apply.gooo"
				files[target] = append(append([]byte(nil), files[target]...), '\n')
				g.PublicSources["examples/agentmesh-service-policy/conditional_apply.gooo"] = digest(files[target])
			case "projection":
				target = "internal/conditional-apply/generated.go"
				files[target] = append(append([]byte(nil), files[target]...), '\n')
			case "host-asset-omission":
				target = "apply/store.go"
				delete(files, target)
				delete(g.Assets, target)
			default:
				if mutation == "field-id" || mutation == "field-type" || mutation == "producer" {
					target = "receipts/conditional-apply-body-codegen.json"
				}
				var doc map[string]any
				if err := json.Unmarshal(files[target], &doc); err != nil {
					t.Fatal(err)
				}
				obj := func(v any) map[string]any { return v.(map[string]any) }
				array := func(v any) []any { return v.([]any) }
				changed := false
				switch mutation {
				case "entry":
					obj(doc["entry"])["activity"] = "CompareServiceGrantChecked"
					changed = true
				case "impact-to-raw", "refinement-to-raw":
					old, replacement := "gooo/service-policy/checked-impact:CompareServiceGrantChecked", "app/service-grant-impact:CompareServiceGrant"
					if mutation == "refinement-to-raw" {
						old, replacement = "gooo/service-policy/checked-refinement:AnalyzeServiceGrantChecked", "app/service-grant-refinement:AnalyzeServiceGrant"
					}
					for _, value := range array(obj(doc["pure_calls"])["sites"]) {
						site := obj(value)
						if site["caller"] == "gooo/service-policy/conditional-apply:AssessServiceGrantUpdate" && site["callee"] == old {
							site["callee"] = replacement
							changed = true
						}
					}
				case "producer":
					obj(doc["report"])["compiler_source_sha"] = "untrusted"
					changed = true
				case "field-id", "field-type":
					for _, value := range array(obj(doc["report"])["record_types"]) {
						record := obj(value)
						if record["name"] != "GrantUpdateDecision" {
							continue
						}
						for _, value := range array(record["fields"]) {
							field := obj(value)
							if field["name"] != "required_observed" {
								continue
							}
							if mutation == "field-id" {
								field["id"] = "gooo://service-policy/conditional-apply/required_before_match"
							} else {
								field["type_id"] = "urn:gooo:type:string"
							}
							changed = true
						}
					}
				}
				if !changed {
					t.Fatal("expected mutation target missing")
				}
				files[target], err = json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mutation != "host-asset-omission" {
				g.Assets[target] = digest(files[target])
			}
			changed, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := checkBindings(files, changed); err == nil {
				t.Fatal("coherently rehashed conditional-apply substitution accepted")
			}
		})
	}
}
