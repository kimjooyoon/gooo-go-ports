package servicepolicy

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
)

const CompilerPin = "83928fa4538245abe33b08549da47f8cbdc50ed2"

// These are consistency checks in a trusted executable, not signature/issuer or
// later host-binary attestations. The source pin preserves exact Google bytes.
//
//go:embed matcher.gooo impact.gooo *-workspace.json source-pin.json compiler.json generation.json receipts/* internal/matcher/generated.go internal/impact/generated.go upstream/LICENSE upstream/api/*.txt evidence/oracle-extracted.go.txt evidence/extraction.json evidence/oracle-go.mod.txt evidence/oracle-command.go.txt evidence/observations.json matcher-cases.json impact-cases.json model.go domain.go input.go bridge.go bindings.go adapter_generated.go cmd/evaluate/main.go generate.py verification_checks.py verify.py evidence/repository-adaptation.json go.mod
var assets embed.FS

var boundNames = []string{
	"matcher.gooo", "impact.gooo", "matcher-workspace.json", "impact-workspace.json", "source-pin.json", "compiler.json",
	"upstream/LICENSE", "upstream/api/tar.go.txt", "upstream/api/network.go.txt", "upstream/api/tar_test.go.txt",
	"evidence/oracle-extracted.go.txt", "evidence/extraction.json", "evidence/oracle-go.mod.txt", "evidence/oracle-command.go.txt", "evidence/observations.json", "matcher-cases.json", "impact-cases.json",
	"receipts/matcher-program.json", "receipts/matcher-lowered.gooo", "receipts/matcher-body-codegen.json", "receipts/matcher-seed-cases.json", "internal/matcher/generated.go",
	"receipts/impact-program.json", "receipts/impact-lowered.gooo", "receipts/impact-body-codegen.json", "receipts/impact-seed-cases.json", "internal/impact/generated.go",
	"model.go", "domain.go", "input.go", "bridge.go", "bindings.go", "adapter_generated.go", "cmd/evaluate/main.go", "generate.py", "verification_checks.py", "verify.py", "evidence/repository-adaptation.json", "go.mod",
}

type generation struct {
	Schema        string            `json:"schema"`
	Origin        string            `json:"origin"`
	Assets        map[string]string `json:"assets"`
	PublicSources map[string]string `json:"public_sources"`
}
type programRecord struct {
	Schema string `json:"schema"`
	Entry  struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
		Lowered  string `json:"lowered_name"`
	} `json:"entry"`
	Lowered string `json:"lowered_gooo_source"`
	Image   struct {
		Entry struct {
			Package  string   `json:"package_path"`
			Source   string   `json:"source"`
			Activity string   `json:"activity"`
			Inputs   []string `json:"inputs"`
			Output   string   `json:"output"`
		} `json:"entry"`
		Packages []struct {
			Path    string `json:"path"`
			Sources []struct {
				Filename string `json:"filename"`
				Digest   string `json:"source_digest"`
			} `json:"sources"`
			Exports []struct {
				Name   string   `json:"name"`
				Kind   string   `json:"kind"`
				ID     string   `json:"id"`
				Inputs []string `json:"input_types"`
				Output string   `json:"output_type"`
			} `json:"exports"`
		} `json:"packages"`
	} `json:"workspace_image"`
	PureCalls struct {
		Sites []struct {
			Caller string `json:"caller"`
			Callee string `json:"callee"`
		} `json:"sites"`
	} `json:"pure_calls"`
}

func SourceBindings() (Bindings, error) {
	files := map[string][]byte{}
	for _, name := range boundNames {
		raw, err := assets.ReadFile(name)
		if err != nil {
			return Bindings{}, err
		}
		files[name] = raw
	}
	manifest, err := assets.ReadFile("generation.json")
	if err != nil {
		return Bindings{}, err
	}
	return checkBindings(files, manifest)
}

func checkBindings(files map[string][]byte, manifest []byte) (Bindings, error) {
	invalid := errors.New("Agent Mesh service-pattern source bindings invalid")
	var g generation
	if json.Unmarshal(manifest, &g) != nil || g.Schema != "gooo/agentmesh-native-generation/v1" || g.Origin != "fresh_pinned_generation" || len(g.Assets) != len(boundNames) || len(files) != len(boundNames) {
		return Bindings{}, invalid
	}
	for _, name := range boundNames {
		if len(files[name]) == 0 || g.Assets[name] != digest(files[name]) {
			return Bindings{}, invalid
		}
	}
	publicSources := map[string]string{
		"ports/agentmesh/service_pattern.gooo":          "matcher.gooo",
		"ports/agentmesh/LICENSE-APACHE-2.0":            "upstream/LICENSE",
		"examples/agentmesh-service-policy/impact.gooo": "impact.gooo",
		"agentmesh-matcher.workspace.json":              "matcher-workspace.json",
		"service-grant-impact.workspace.json":           "impact-workspace.json",
	}
	if len(g.PublicSources) != len(publicSources) {
		return Bindings{}, invalid
	}
	for canonical, mirror := range publicSources {
		if g.PublicSources[canonical] != digest(files[mirror]) {
			return Bindings{}, invalid
		}
	}
	var compiler struct {
		Schema   string `json:"schema"`
		Source   string `json:"compiler_source_sha"`
		Revision string `json:"vcs_revision"`
		Status   string `json:"source_status"`
		Modified string `json:"vcs_modified"`
		Go       string `json:"go_version"`
		NativeGo string `json:"native_go_required"`
	}
	if json.Unmarshal(files["compiler.json"], &compiler) != nil || compiler.Schema != "gooo/build-identity/v1" || compiler.Source != CompilerPin || compiler.Revision != CompilerPin || compiler.Status != "CLEAN_VCS" || compiler.Modified != "false" || compiler.Go != "go1.27.2" || compiler.NativeGo != "go1.27.2" {
		return Bindings{}, invalid
	}
	var pin struct {
		Schema   string `json:"schema"`
		Revision string `json:"revision"`
		Archives []struct {
			Filename string `json:"filename"`
			SHA      string `json:"sha256"`
			Bytes    int    `json:"bytes"`
		} `json:"archive_files"`
		Components []struct {
			Filename string `json:"filename"`
			SHA      string `json:"sha256"`
			Bytes    int    `json:"bytes"`
		} `json:"canonical_components"`
		Placement map[string]string `json:"public_placement"`
	}
	if json.Unmarshal(files["source-pin.json"], &pin) != nil || pin.Schema != "gooo/agentmesh-source-pin/v1" || pin.Revision != "9fd1a41a15174df0aaef601c942410c2116bff91" || len(pin.Archives) != 4 || len(pin.Components) != 4 {
		return Bindings{}, invalid
	}
	for _, f := range append(pin.Archives, pin.Components...) {
		raw, ok := files[f.Filename]
		if !ok || len(raw) != f.Bytes || digest(raw) != "sha256:"+f.SHA {
			return Bindings{}, invalid
		}
	}
	expectedPlacement := map[string]string{
		"canonical_matcher": "ports/agentmesh/service_pattern.gooo",
		"matcher_mirror":    "matcher.gooo",
		"matcher_workspace": "agentmesh-matcher.workspace.json",
		"impact_source":     "examples/agentmesh-service-policy/impact.gooo",
		"impact_workspace":  "service-grant-impact.workspace.json",
	}
	if len(pin.Placement) != len(expectedPlacement) {
		return Bindings{}, invalid
	}
	for key, value := range expectedPlacement {
		if pin.Placement[key] != value {
			return Bindings{}, invalid
		}
	}
	var adaptation struct {
		ID         string `json:"id"`
		Revision   string `json:"revision"`
		Target     string `json:"target"`
		TargetSHA  string `json:"target_sha256"`
		LicenseSHA string `json:"license_sha256"`
	}
	if json.Unmarshal(files["evidence/repository-adaptation.json"], &adaptation) != nil ||
		adaptation.ID != "agentmesh-service-pattern/v1" || adaptation.Revision != pin.Revision ||
		adaptation.Target != expectedPlacement["canonical_matcher"] || "sha256:"+adaptation.TargetSHA != digest(files["matcher.gooo"]) ||
		"sha256:"+adaptation.LicenseSHA != digest(files["upstream/LICENSE"]) {
		return Bindings{}, invalid
	}
	entries := map[string]string{}
	for _, config := range []struct {
		Prefix, Package, Activity, ID, Input, Output string
		N                                            int
	}{
		{"matcher", "go/google/agentmesh/api", "MatchServicePattern", "gooo://agentmesh-prototype/match-service-pattern", "(string,string,string)", "bool", 3},
		{"impact", "app/service-grant-impact", "CompareServiceGrant", "gooo://service-impact/compare", "(string,string,string,string)", "GrantImpact", 4},
	} {
		var p programRecord
		if json.Unmarshal(files["receipts/"+config.Prefix+"-program.json"], &p) != nil || p.Schema != "gooo/workspace-body-program/v1" || p.Entry.Package != config.Package || p.Entry.Activity != config.Activity || p.Entry.Lowered == "" || !bytes.Equal([]byte(p.Lowered), files["receipts/"+config.Prefix+"-lowered.gooo"]) {
			return Bindings{}, invalid
		}
		entries[config.Prefix] = p.Entry.Lowered
		ie := p.Image.Entry
		expectedOutput := "Boolean"
		if config.Prefix == "impact" {
			expectedOutput = "GrantImpact"
		}
		if ie.Package != config.Package || ie.Source != map[string]string{"matcher": "ports/agentmesh/service_pattern.gooo", "impact": "examples/agentmesh-service-policy/impact.gooo"}[config.Prefix] || ie.Activity != config.Activity || ie.Output != expectedOutput || len(ie.Inputs) != config.N {
			return Bindings{}, invalid
		}
		for _, typ := range ie.Inputs {
			if typ != "Text" {
				return Bindings{}, invalid
			}
		}
		expectedSources := map[string]string{"ports/agentmesh/service_pattern.gooo": "matcher.gooo"}
		if config.Prefix == "impact" {
			expectedSources["examples/agentmesh-service-policy/impact.gooo"] = "impact.gooo"
		}
		seen := map[string]bool{}
		for _, pkg := range p.Image.Packages {
			if pkg.Path != "go/google/agentmesh/api" && (config.Prefix != "impact" || pkg.Path != "app/service-grant-impact") {
				return Bindings{}, invalid
			}
			for _, s := range pkg.Sources {
				if expectedSources[s.Filename] == "" || seen[s.Filename] || s.Digest != sourceDigest(files[expectedSources[s.Filename]]) {
					return Bindings{}, invalid
				}
				seen[s.Filename] = true
			}
			n := 3
			activity, id, output := "MatchServicePattern", "gooo://agentmesh-prototype/match-service-pattern", "gooo://std/boolean"
			if pkg.Path == "app/service-grant-impact" {
				n = 4
				activity, id, output = "CompareServiceGrant", "gooo://service-impact/compare", "gooo://service-impact/decision/v1"
			}
			activities := 0
			for _, e := range pkg.Exports {
				if e.Kind == "activity" {
					activities++
					if e.Name != activity || e.ID != id || len(e.Inputs) != n || e.Output != output {
						return Bindings{}, invalid
					}
					for _, typ := range e.Inputs {
						if typ != "gooo://std/text" {
							return Bindings{}, invalid
						}
					}
				}
			}
			if activities != 1 {
				return Bindings{}, invalid
			}
		}
		if len(seen) != len(expectedSources) {
			return Bindings{}, invalid
		}
		if config.Prefix == "impact" {
			if len(p.PureCalls.Sites) != 2 {
				return Bindings{}, invalid
			}
			for _, s := range p.PureCalls.Sites {
				if s.Caller != "app/service-grant-impact:CompareServiceGrant" || s.Callee != "go/google/agentmesh/api:MatchServicePattern" {
					return Bindings{}, invalid
				}
			}
		}
		var body struct {
			Source string `json:"source"`
			Report struct {
				Schema        string `json:"schema"`
				Decision      string `json:"decision"`
				Activity      string `json:"activity"`
				ID            string `json:"activity_id"`
				Input         string `json:"input_type"`
				Output        string `json:"output_type"`
				Compiler      string `json:"compiler_source_sha"`
				Source        string `json:"source_digest"`
				Generated     string `json:"generated_digest"`
				Replay        string `json:"replay_digest"`
				Typecheck     bool   `json:"typecheck_passed"`
				Deterministic bool   `json:"deterministic_replay"`
				Writes        int    `json:"repository_writes"`
				Records       []struct {
					Name   string `json:"name"`
					ID     string `json:"id"`
					Fields []struct {
						Name string `json:"name"`
						ID   string `json:"id"`
						Type string `json:"type_id"`
					} `json:"fields"`
				} `json:"record_types"`
			} `json:"report"`
		}
		if json.Unmarshal(files["receipts/"+config.Prefix+"-body-codegen.json"], &body) != nil {
			return Bindings{}, invalid
		}
		r := body.Report
		if r.Schema != "gooo/body-codegen-report/v3" || r.Decision != "PASS" || r.Activity != p.Entry.Lowered || r.ID != config.ID || r.Input != config.Input || r.Output != config.Output || r.Compiler != CompilerPin || r.Source != digest([]byte(p.Lowered)) || body.Source == "" || r.Generated != digest([]byte(body.Source)) || r.Replay != r.Generated || !r.Typecheck || !r.Deterministic || r.Writes != 0 {
			return Bindings{}, invalid
		}
		projected, err := format.Source([]byte(body.Source))
		if err != nil || !bytes.Equal(projected, files["internal/"+config.Prefix+"/generated.go"]) {
			return Bindings{}, invalid
		}
		if config.Prefix == "matcher" {
			if len(r.Records) != 0 {
				return Bindings{}, invalid
			}
		} else {
			if len(r.Records) != 1 || r.Records[0].Name != "GrantImpact" || r.Records[0].ID != "gooo://service-impact/decision/v1" || len(r.Records[0].Fields) != 3 {
				return Bindings{}, invalid
			}
			for i, f := range r.Records[0].Fields {
				names := []string{"before_match", "after_match", "change"}
				ids := []string{"before", "after", "change"}
				typ := "urn:gooo:type:boolean"
				if i == 2 {
					typ = "urn:gooo:type:string"
				}
				if f.Name != names[i] || f.ID != "gooo://service-impact/"+ids[i] || f.Type != typ {
					return Bindings{}, invalid
				}
			}
		}
	}
	adapter, err := format.Source([]byte(adapterSource(entries["matcher"], entries["impact"])))
	if err != nil || !bytes.Equal(adapter, files["adapter_generated.go"]) {
		return Bindings{}, invalid
	}
	return Bindings{CompilerSourceSHA: CompilerPin, GenerationSHA256: digest(manifest), GenerationOrigin: g.Origin, Assets: g.Assets}, nil
}

func adapterSource(matcher, impact string) string {
	return fmt.Sprintf(`// Code generated from pinned compiler-owned entry identities; DO NOT EDIT.
package servicepolicy
import (
 matcher "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/matcher"
 impact "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/impact"
)
func nativeMatcher(f MatcherFacts) bool { return matcher.%s(f.Pattern,f.ReqType,f.ReqName) }
func nativeImpact(f ImpactFacts) any { return impact.%s(f.Before,f.After,f.ReqType,f.ReqName) }
`, matcher, impact)
}
func digest(raw []byte) string       { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func sourceDigest(raw []byte) string { encoded, _ := json.Marshal(string(raw)); return digest(encoded) }
