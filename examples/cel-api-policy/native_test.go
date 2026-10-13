package nativepolicy

import (
	"encoding/json"
	"os"
	"testing"
)

// Expected records come from independently executed CEL, not a Go copy of
// the Gooo decision body. Every output observation is compared.
func TestIndependentCELObservations(t *testing.T) {
	raw, err := os.ReadFile("oracle/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Observations []struct {
			ID       string   `json:"id"`
			Facts    Facts    `json:"facts"`
			Observed Decision `json:"observed"`
		} `json:"observations"`
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Observations) != 52 {
		t.Fatalf("unexpected independent corpus count %d", len(corpus.Observations))
	}
	for _, row := range corpus.Observations {
		t.Run(row.ID, func(t *testing.T) {
			proof, err := EvaluateScalar(row.Facts)
			if err != nil {
				t.Fatal(err)
			}
			if proof.Decision != row.Observed {
				t.Fatalf("native decision differs: got %+v want %+v", proof.Decision, row.Observed)
			}
		})
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
				raw, err := assets.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				files[name] = raw
			}
			files[changed] = append(append([]byte{}, files[changed]...), '\n')
			if _, err := checkBindings(files, manifest); err == nil {
				t.Fatal("changed embedded asset was accepted")
			}
		})
	}
}

func TestManifestTamperRejected(t *testing.T) {
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
	var record generation
	if json.Unmarshal(manifest, &record) != nil {
		t.Fatal("manifest")
	}
	record.Assets["canonical/scalar_access.gooo"] = "sha256:0000"
	bad, _ := json.Marshal(record)
	if _, err := checkBindings(files, bad); err == nil {
		t.Fatal("changed source digest accepted")
	}
}

// These tuples were authored independently against the ORIGINAL API contract.
// They are not CEL observations of authentication, tenancy or API effects.
func TestIndependentOriginalAPIContract(t *testing.T) {
	raw, err := os.ReadFile("oracle/api-composition-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		ID       string      `json:"id"`
		Facts    APIFacts    `json:"facts"`
		Expected APIDecision `json:"expected"`
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus) != 32 {
		t.Fatalf("unexpected original contract count %d", len(corpus))
	}
	for _, row := range corpus {
		t.Run(row.ID, func(t *testing.T) {
			proof, err := EvaluateAPI(row.Facts)
			if err != nil {
				t.Fatal(err)
			}
			if proof.Decision != row.Expected {
				t.Fatalf("native original decision differs: got %+v want %+v", proof.Decision, row.Expected)
			}
		})
	}
}

// Rehashing a changed canonical source cannot make it match the compiler's
// saved importing-program source roster. This exercises the relational check
// beyond the simpler manifest asset-hash checks above.
func TestRehashedCanonicalSourceTamperRejected(t *testing.T) {
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
	var record generation
	if json.Unmarshal(manifest, &record) != nil {
		t.Fatal("manifest")
	}
	for _, changed := range []string{"canonical/scalar_access.gooo", "canonical/main.gooo", "receipts/api-lowered.gooo", "internal/api/generated.go"} {
		t.Run(changed, func(t *testing.T) {
			original := files[changed]
			files[changed] = append(append([]byte{}, original...), '\n')
			record.Assets[changed] = digest(files[changed])
			bad, _ := json.Marshal(record)
			if _, err := checkBindings(files, bad); err == nil {
				t.Fatal("rehashed source/projection inconsistency accepted")
			}
			files[changed] = original
			record.Assets[changed] = digest(original)
		})
	}
}
