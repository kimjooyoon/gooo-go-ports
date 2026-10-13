package servicepolicy

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestSourceBindingsReturnedMapsAreIndependent(t *testing.T) {
	want, err := SourceBindings()
	if err != nil {
		t.Fatal(err)
	}
	want.Assets = maps.Clone(want.Assets)
	matcher := MatcherFacts{Pattern: "mcp://*", ReqType: "mcp", ReqName: "weather"}
	impact := ImpactFacts{Before: "*", After: "mcp://weather", ReqType: "mcp", ReqName: "weather"}
	refinement := RefinementFacts{Before: "*", After: "mcp://weather"}
	update := GrantUpdateFacts{Before: "*", After: "mcp://weather", RequiredType: "mcp", RequiredName: "weather"}
	for _, tc := range []struct {
		name string
		read func() (Bindings, error)
	}{
		{"source", SourceBindings},
		{"matcher", func() (Bindings, error) { p, err := EvaluateMatcher(matcher); return p.Bindings, err }},
		{"impact", func() (Bindings, error) { p, err := EvaluateImpact(impact); return p.Bindings, err }},
		{"refinement", func() (Bindings, error) { p, err := EvaluateRefinement(refinement); return p.Bindings, err }},
		{"checked-matcher", func() (Bindings, error) { p, err := EvaluateMatcherChecked(matcher); return p.Bindings, err }},
		{"checked-impact", func() (Bindings, error) { p, err := EvaluateImpactChecked(impact); return p.Bindings, err }},
		{"checked-refinement", func() (Bindings, error) { p, err := EvaluateRefinementChecked(refinement); return p.Bindings, err }},
		{"grant-update", func() (Bindings, error) { p, err := EvaluateGrantUpdate(update); return p.Bindings, err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous, err := SourceBindings()
			if err != nil {
				t.Fatal(err)
			}
			previousProof, err := EvaluateGrantUpdate(update)
			if err != nil {
				t.Fatal(err)
			}
			owned, err := tc.read()
			if err != nil || !reflect.DeepEqual(owned, want) {
				t.Fatalf("unexpected original bindings: %+v, %v", owned, err)
			}
			owned.Assets["matcher.gooo"] = "sha256:caller-modified"
			delete(owned.Assets, "model.go")
			owned.Assets["caller-added"] = "sha256:untrusted"
			if !reflect.DeepEqual(previous, want) || !reflect.DeepEqual(previousProof.Bindings, want) {
				t.Fatal("caller map mutation changed an earlier binding or proof")
			}
			fresh, err := SourceBindings()
			if err != nil || !reflect.DeepEqual(fresh, want) {
				t.Fatalf("caller map mutation contaminated SourceBindings: %+v, %v", fresh, err)
			}
			again, err := tc.read()
			if err != nil || !reflect.DeepEqual(again, want) {
				t.Fatalf("caller map mutation contaminated a later proof: %+v, %v", again, err)
			}
		})
	}
}

func TestSourceBindingsConcurrentCallerMapMutation(t *testing.T) {
	want, err := SourceBindings()
	if err != nil {
		t.Fatal(err)
	}
	want.Assets = maps.Clone(want.Assets)
	const workers = 16
	start := make(chan struct{})
	failures := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for iteration := range 8 {
				bindings, err := SourceBindings()
				if err != nil || !reflect.DeepEqual(bindings, want) {
					failures <- fmt.Errorf("worker %d source bindings differ: %v", worker, err)
					return
				}
				facts := GrantUpdateFacts{Before: "*", After: "mcp://weather", RequiredType: "mcp", RequiredName: "weather"}
				status := "ELIGIBLE"
				if (worker+iteration)%2 != 0 {
					facts.Before, facts.After = "mcp://weather", "*"
					status = "REFUSED"
				}
				proof, err := EvaluateGrantUpdate(facts)
				if err != nil || proof.Facts != facts || proof.Decision.Status != status || !reflect.DeepEqual(proof.Bindings, want) {
					failures <- fmt.Errorf("worker %d evaluation differs: %+v, %v", worker, proof, err)
					return
				}
				bindings.Assets["matcher.gooo"] = "source-caller-change"
				delete(bindings.Assets, "model.go")
				bindings.Assets[fmt.Sprintf("source-worker-%d", worker)] = "added"
				if !reflect.DeepEqual(proof.Bindings, want) {
					failures <- fmt.Errorf("worker %d source map aliases proof", worker)
					return
				}
				proof.Bindings.Assets["matcher.gooo"] = "proof-caller-change"
				delete(proof.Bindings.Assets, "model.go")
				proof.Bindings.Assets[fmt.Sprintf("proof-worker-%d", worker)] = "added"
			}
		}()
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	fresh, err := SourceBindings()
	if err != nil || !reflect.DeepEqual(fresh, want) {
		t.Fatalf("concurrent caller mutation contaminated the cached binding: %+v, %v", fresh, err)
	}
}

func TestSourceBindingCacheDoesNotCacheGrantDecisions(t *testing.T) {
	for round := range 2 {
		for _, tc := range []struct {
			name  string
			facts GrantUpdateFacts
			want  GrantUpdateDecision
		}{
			{"eligible", GrantUpdateFacts{"*", "mcp://weather", "mcp", "weather"}, GrantUpdateDecision{Status: "ELIGIBLE", Reason: "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true, SubsetRuleID: "OLD_GLOBAL"}},
			{"widening", GrantUpdateFacts{"mcp://weather", "mcp://*", "mcp", "weather"}, GrantUpdateDecision{Status: "REFUSED", Reason: "WIDENING_WITNESS", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true}},
			{"required-removed", GrantUpdateFacts{"*", "mcp://calculator", "mcp", "weather"}, GrantUpdateDecision{Status: "REFUSED", Reason: "REQUIRED_REQUEST_REMOVED", RequiredObserved: true, RequiredBeforeMatch: true, SubsetRuleID: "OLD_GLOBAL"}},
			{"invalid-unicode", GrantUpdateFacts{"*", "mcp://weather", "mcp", "\u2603"}, GrantUpdateDecision{Status: "INVALID_INPUT", Reason: "INVALID_CANONICAL_INPUT", InvalidField: "required_name"}},
			{"empty-required", GrantUpdateFacts{"*", "mcp://weather", "mcp", ""}, GrantUpdateDecision{Status: "INVALID_INPUT", Reason: "EMPTY_REQUIRED_REQUEST", InvalidField: "required_name"}},
		} {
			t.Run(fmt.Sprintf("%d/%s", round, tc.name), func(t *testing.T) {
				proof, err := EvaluateGrantUpdate(tc.facts)
				if err != nil || proof.Facts != tc.facts || proof.Decision != tc.want {
					t.Fatalf("input-specific result changed: %+v, %v; want %+v", proof, err, tc.want)
				}
			})
		}
	}
}

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
	for _, changed := range []string{"matcher.gooo", "impact.gooo", "receipts/matcher-lowered.gooo", "internal/impact/generated.go", "adapter_generated.go", "evidence/repository-adaptation.json", "refinement.gooo", "receipts/refinement-lowered.gooo", "internal/refinement/generated.go", "refinement-workspace.json"} {
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

func TestRefinementRehashedProgramIdentityTamperRejected(t *testing.T) {
	for _, name := range []string{"unknown_empty_entity", "producer", "entry_activity", "source_digest", "source_package", "duplicate_package", "init_order", "pure_call_target", "pure_call_count", "record_entity", "record_name", "field_id", "field_type", "field_order"} {
		t.Run(name, func(t *testing.T) {
			files := map[string][]byte{}
			for _, f := range boundNames {
				var err error
				files[f], err = assets.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
			}
			target := "receipts/refinement-program.json"
			if name == "producer" || name == "record_entity" || name == "record_name" || name == "field_id" || name == "field_type" || name == "field_order" {
				target = "receipts/refinement-body-codegen.json"
			}
			var doc map[string]any
			if err := json.Unmarshal(files[target], &doc); err != nil {
				t.Fatal(err)
			}
			obj := func(x any) map[string]any { return x.(map[string]any) }
			array := func(x any) []any { return x.([]any) }
			if target == "receipts/refinement-program.json" {
				image := obj(doc["workspace_image"])
				pkgs := array(image["packages"])
				var app map[string]any
				for _, p := range pkgs {
					if obj(p)["path"] == "app/service-grant-refinement" {
						app = obj(p)
					}
				}
				if app == nil {
					t.Fatal("missing refinement package")
				}
				switch name {
				case "unknown_empty_entity":
					found := false
					for _, e := range array(app["exports"]) {
						v := obj(e)
						if v["kind"] == "entity" && v["name"] == "Text" {
							v["name"] = "Unknown"
							v["id"] = ""
							found = true
						}
					}
					if !found {
						t.Fatal("missing Text entity")
					}
				case "entry_activity":
					obj(doc["entry"])["activity"] = "Other"
				case "source_digest":
					obj(array(app["sources"])[0])["source_digest"] = "sha256:changed"
				case "source_package":
					app["path"] = "go/google/agentmesh/api"
				case "duplicate_package":
					image["packages"] = append(pkgs, pkgs[0])
				case "init_order":
					order := array(image["init_order"])
					order[0], order[1] = order[1], order[0]
				case "pure_call_target":
					sites := array(obj(doc["pure_calls"])["sites"])
					obj(sites[0])["callee"] = "host:Matcher"
				case "pure_call_count":
					pc := obj(doc["pure_calls"])
					pc["sites"] = array(pc["sites"])[1:]
				}
			} else {
				report := obj(doc["report"])
				if name == "producer" {
					report["compiler_source_sha"] = "wrong"
				} else {
					record := obj(array(report["record_types"])[0])
					fields := array(record["fields"])
					switch name {
					case "record_entity":
						record["id"] = "wrong"
					case "record_name":
						record["name"] = "Other"
					case "field_id":
						obj(fields[7])["id"] = "wrong"
					case "field_type":
						obj(fields[7])["type_id"] = "urn:gooo:type:string"
					case "field_order":
						fields[0], fields[1] = fields[1], fields[0]
					}
				}
			}
			files[target], _ = json.Marshal(doc)
			manifest, _ := assets.ReadFile("generation.json")
			var g generation
			if err := json.Unmarshal(manifest, &g); err != nil {
				t.Fatal(err)
			}
			g.Assets[target] = digest(files[target])
			changed, _ := json.Marshal(g)
			if _, err := checkBindings(files, changed); err == nil {
				t.Fatal("rehashed metadata contradiction accepted")
			}
		})
	}
}
