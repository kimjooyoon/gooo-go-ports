package servicepolicy

import (
	"encoding/json"
	"os"
	"testing"
)

// These are retained prior real-upstream observations. verify.py must freshly
// execute the extracted oracle before public acceptance of this integration.
func TestRetainedIndependentMatcherObservations(t *testing.T) {
	raw, err := os.ReadFile("oracle/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		ReqType string `json:"req_type"`
		ReqName string `json:"req_name"`
		Matched bool   `json:"matched"`
	}
	if json.Unmarshal(raw, &rows) != nil || len(rows) != 63 {
		t.Fatal("retained independent matcher corpus")
	}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			p, err := EvaluateMatcher(MatcherFacts{Pattern: row.Pattern, ReqType: row.ReqType, ReqName: row.ReqName})
			if err != nil || p.Matched != row.Matched {
				t.Fatalf("native wrapper observation differs: %+v %v", p, err)
			}
		})
	}
}

func TestOriginalThreeFieldRequestImpact(t *testing.T) {
	raw, err := os.ReadFile("impact-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name string `json:"name"`
		ImpactFacts
		Want ImpactDecision `json:"want"`
	}
	if json.Unmarshal(raw, &rows) != nil || len(rows) != 16 {
		t.Fatal("original impact corpus")
	}
	states := map[string]bool{}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			p, err := EvaluateImpact(row.ImpactFacts)
			if err != nil || p.Decision != row.Want {
				t.Fatalf("original request impact differs: %+v %v", p, err)
			}
			states[p.Decision.Change] = true
		})
	}
	if len(states) != 4 {
		t.Fatal("missing fourstate request impacts")
	}
}

func TestEveryBoundAssetTamperRejected(t *testing.T) {
	manifest, err := assets.ReadFile("generation.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range boundNames {
		t.Run(changed, func(t *testing.T) {
			files := map[string][]byte{}
			for _, name := range boundNames {
				files[name], err = assets.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
			}
			files[changed] = append(append([]byte{}, files[changed]...), '\n')
			if _, err := checkBindings(files, manifest); err == nil {
				t.Fatal("bound asset tamper accepted")
			}
		})
	}
}

func TestRehashedSourceProjectionAndGenerationTamperRejected(t *testing.T) {
	manifest, _ := assets.ReadFile("generation.json")
	var g generation
	json.Unmarshal(manifest, &g)
	for _, changed := range []string{"matcher.gooo", "impact.gooo", "receipts/matcher-lowered.gooo", "internal/impact/generated.go", "adapter_generated.go", "evidence/repository-adaptation.json"} {
		t.Run(changed, func(t *testing.T) {
			files := map[string][]byte{}
			for _, name := range boundNames {
				files[name], _ = assets.ReadFile(name)
			}
			files[changed] = append(append([]byte{}, files[changed]...), '\n')
			if changed == "evidence/repository-adaptation.json" {
				var metadata map[string]any
				json.Unmarshal(files[changed], &metadata)
				metadata["target"] = "ports/agentmesh/forged.gooo"
				files[changed], _ = json.Marshal(metadata)
			}
			copy := g
			copy.Assets = map[string]string{}
			for k, v := range g.Assets {
				copy.Assets[k] = v
			}
			copy.Assets[changed] = digest(files[changed])
			bad, _ := json.Marshal(copy)
			if _, err := checkBindings(files, bad); err == nil {
				t.Fatal("rehashed source/projection mismatch accepted")
			}
		})
	}
	files := map[string][]byte{}
	for _, name := range boundNames {
		files[name], _ = assets.ReadFile(name)
	}
	g.Assets["model.go"] = "sha256:tampered"
	bad, _ := json.Marshal(g)
	if _, err := checkBindings(files, bad); err == nil {
		t.Fatal("generation digest tamper accepted")
	}
}
