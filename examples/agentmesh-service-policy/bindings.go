package servicepolicy

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"reflect"
	"regexp"
	"strings"
)

const CompilerPin = "83928fa4538245abe33b08549da47f8cbdc50ed2"

// These are consistency checks in a trusted executable, not signature/issuer or
// later host-binary attestations. The source pin preserves exact Google bytes.
//
//go:embed *.gooo *-workspace.json source-pin.json compiler.json generation.json receipts/* internal/*/generated.go upstream/LICENSE upstream/api/*.txt evidence/oracle-extracted.go.txt evidence/extraction.json evidence/oracle-go.mod.txt evidence/oracle-command.go.txt evidence/observations.json *-cases.json domain-reference.json model.go domain.go input.go bridge.go bindings.go adapter_generated.go cmd/evaluate/main.go generate.py verification_checks.py verify.py evidence/repository-adaptation.json go.mod
var assets embed.FS

var boundNames = []string{
	"matcher.gooo",
	"impact.gooo",
	"refinement.gooo",
	"domain.gooo",
	"checked_matcher.gooo",
	"checked_impact.gooo",
	"checked_refinement.gooo",
	"source-pin.json",
	"compiler.json",
	"upstream/LICENSE",
	"upstream/api/tar.go.txt",
	"upstream/api/network.go.txt",
	"upstream/api/tar_test.go.txt",
	"evidence/oracle-extracted.go.txt",
	"evidence/extraction.json",
	"evidence/oracle-go.mod.txt",
	"evidence/oracle-command.go.txt",
	"evidence/observations.json",
	"evidence/repository-adaptation.json",
	"matcher-cases.json",
	"impact-cases.json",
	"refinement-cases.json",
	"checked-cases.json",
	"domain-cases.json",
	"domain-reference.json",
	"matcher-raw-workspace.json",
	"impact-raw-workspace.json",
	"refinement-raw-workspace.json",
	"matcher-workspace.json",
	"impact-workspace.json",
	"refinement-workspace.json",
	"receipts/matcher-program.json",
	"receipts/matcher-lowered.gooo",
	"receipts/matcher-body-codegen.json",
	"receipts/matcher-seed-cases.json",
	"internal/matcher/generated.go",
	"receipts/impact-program.json",
	"receipts/impact-lowered.gooo",
	"receipts/impact-body-codegen.json",
	"receipts/impact-seed-cases.json",
	"internal/impact/generated.go",
	"receipts/refinement-program.json",
	"receipts/refinement-lowered.gooo",
	"receipts/refinement-body-codegen.json",
	"receipts/refinement-seed-cases.json",
	"internal/refinement/generated.go",
	"receipts/checked-matcher-program.json",
	"receipts/checked-matcher-lowered.gooo",
	"receipts/checked-matcher-body-codegen.json",
	"receipts/checked-matcher-seed-cases.json",
	"internal/checked-matcher/generated.go",
	"receipts/checked-impact-program.json",
	"receipts/checked-impact-lowered.gooo",
	"receipts/checked-impact-body-codegen.json",
	"receipts/checked-impact-seed-cases.json",
	"internal/checked-impact/generated.go",
	"receipts/checked-refinement-program.json",
	"receipts/checked-refinement-lowered.gooo",
	"receipts/checked-refinement-body-codegen.json",
	"receipts/checked-refinement-seed-cases.json",
	"internal/checked-refinement/generated.go",
	"model.go",
	"domain.go",
	"input.go",
	"bridge.go",
	"bindings.go",
	"adapter_generated.go",
	"cmd/evaluate/main.go",
	"generate.py",
	"verification_checks.py",
	"verify.py",
	"go.mod",
}

type generation struct {
	Schema        string            `json:"schema"`
	Origin        string            `json:"origin"`
	Assets        map[string]string `json:"assets"`
	PublicSources map[string]string `json:"public_sources"`
}
type programRecord struct {
	Activities []struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
		Lowered  string `json:"lowered_name"`
	} `json:"activities"`
	Schema string `json:"schema"`
	Entry  struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
		Lowered  string `json:"lowered_name"`
	} `json:"entry"`
	Lowered string `json:"lowered_gooo_source"`
	Image   struct {
		Schema    string   `json:"schema"`
		InitOrder []string `json:"init_order"`
		Entry     struct {
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
		Activities []struct {
			Activity struct {
				Package string `json:"package_path"`
				Name    string `json:"activity"`
				Lowered string `json:"lowered_name"`
			} `json:"activity"`
			Source  string `json:"source"`
			BodySHA string `json:"source_body_sha256"`
		} `json:"activities"`
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
		"ports/agentmesh/service_pattern.gooo":                      "matcher.gooo",
		"examples/agentmesh-service-policy/impact.gooo":             "impact.gooo",
		"examples/agentmesh-service-policy/refinement.gooo":         "refinement.gooo",
		"examples/agentmesh-service-policy/domain.gooo":             "domain.gooo",
		"examples/agentmesh-service-policy/checked_matcher.gooo":    "checked_matcher.gooo",
		"examples/agentmesh-service-policy/checked_impact.gooo":     "checked_impact.gooo",
		"examples/agentmesh-service-policy/checked_refinement.gooo": "checked_refinement.gooo",
		"ports/agentmesh/LICENSE-APACHE-2.0":                        "upstream/LICENSE",
		"agentmesh-matcher-raw.workspace.json":                      "matcher-raw-workspace.json",
		"service-grant-impact-raw.workspace.json":                   "impact-raw-workspace.json",
		"service-grant-refinement-raw.workspace.json":               "refinement-raw-workspace.json",
		"agentmesh-matcher.workspace.json":                          "matcher-workspace.json",
		"service-grant-impact.workspace.json":                       "impact-workspace.json",
		"service-grant-refinement.workspace.json":                   "refinement-workspace.json",
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
		"canonical_matcher":            "ports/agentmesh/service_pattern.gooo",
		"matcher_mirror":               "matcher.gooo",
		"matcher_workspace":            "agentmesh-matcher-raw.workspace.json",
		"impact_source":                "examples/agentmesh-service-policy/impact.gooo",
		"impact_workspace":             "service-grant-impact-raw.workspace.json",
		"checked_matcher_workspace":    "agentmesh-matcher.workspace.json",
		"checked_impact_workspace":     "service-grant-impact.workspace.json",
		"checked_refinement_workspace": "service-grant-refinement.workspace.json",
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

	var contracts struct {
		Packages map[string]packageContract `json:"packages"`
		Records  map[string]recordContract  `json:"records"`
		Routes   map[string]routeContract   `json:"routes"`
	}
	if json.Unmarshal([]byte(bindingContracts), &contracts) != nil {
		return Bindings{}, invalid
	}
	entries, validators := map[string]string{}, map[string]string{}
	for prefix, config := range contracts.Routes {
		expectedManifest := map[string]any{"schema": "gooo/package-workspace-manifest/v1", "entry": map[string]string{"package_path": config.Package, "activity": config.Activity}}
		mp := []map[string]any{}
		for _, name := range config.ManifestPackages {
			mp = append(mp, map[string]any{"path": name, "sources": []string{contracts.Packages[name].Source}})
		}
		expectedManifest["packages"] = mp
		encodedManifest, _ := json.Marshal(expectedManifest)
		if !equalJSON(encodedManifest, files[config.Workspace]) {
			return Bindings{}, invalid
		}
		var p programRecord
		if json.Unmarshal(files["receipts/"+prefix+"-program.json"], &p) != nil || p.Schema != "gooo/workspace-body-program/v1" || p.Entry.Package != config.Package || p.Entry.Activity != config.Activity || p.Entry.Lowered == "" || !bytes.Equal([]byte(p.Lowered), files["receipts/"+prefix+"-lowered.gooo"]) {
			return Bindings{}, invalid
		}
		entries[prefix] = p.Entry.Lowered
		if len(p.Activities) != 1 || p.Activities[0].Package != p.Entry.Package || p.Activities[0].Activity != p.Entry.Activity || p.Activities[0].Lowered != p.Entry.Lowered {
			return Bindings{}, invalid
		}
		ie := p.Image.Entry
		if p.Image.Schema != "gooo/package-runtime-image/v1" || !reflect.DeepEqual(p.Image.InitOrder, config.Packages) || len(p.Image.Packages) != len(config.Packages) || ie.Package != config.Package || ie.Activity != config.Activity || ie.Source != contracts.Packages[config.Package].Source || ie.Output != config.Output || !textInputs(ie.Inputs, config.N, "Text") {
			return Bindings{}, invalid
		}
		allowed := map[string]bool{}
		for _, name := range config.Packages {
			allowed[name] = true
		}
		seenPackages := map[string]bool{}
		for _, pkg := range p.Image.Packages {
			spec, known := contracts.Packages[pkg.Path]
			if !known || !allowed[pkg.Path] || seenPackages[pkg.Path] || len(pkg.Sources) != 1 || pkg.Sources[0].Filename != spec.Source || pkg.Sources[0].Digest != sourceDigest(files[spec.Mirror]) || len(pkg.Exports) != len(spec.Entities)+len(spec.Activities) {
				return Bindings{}, invalid
			}
			seenPackages[pkg.Path] = true
			seenExports := map[string]bool{}
			for _, e := range pkg.Exports {
				if seenExports[e.Name] {
					return Bindings{}, invalid
				}
				seenExports[e.Name] = true
				switch e.Kind {
				case "entity":
					expected, ok := spec.Entities[e.Name]
					if !ok || e.ID != expected || len(e.Inputs) != 0 || e.Output != "" {
						return Bindings{}, invalid
					}
				case "activity":
					a, ok := spec.Activities[e.Name]
					if !ok || e.ID != a.ID || !textInputs(e.Inputs, a.N, "gooo://std/text") || e.Output != a.Output {
						return Bindings{}, invalid
					}
				default:
					return Bindings{}, invalid
				}
			}
		}
		expectedEdges, actualEdges := map[[2]string]int{}, map[[2]string]int{}
		for _, edge := range config.Edges {
			expectedEdges[edge]++
		}
		for _, edge := range p.PureCalls.Sites {
			actualEdges[[2]string{edge.Caller, edge.Callee}]++
		}
		if !reflect.DeepEqual(expectedEdges, actualEdges) {
			return Bindings{}, invalid
		}
		expectedActivities := map[[2]string]bool{}
		for _, pkg := range config.Packages {
			for name := range contracts.Packages[pkg].Activities {
				expectedActivities[[2]string{pkg, name}] = true
			}
		}
		lowered := map[[2]string]string{}
		if prefix == "matcher" {
			if len(p.PureCalls.Activities) != 0 || p.Entry.Lowered != "GoooPackage0ActivityMatchServicePattern" {
				return Bindings{}, invalid
			}
			lowered[[2]string{config.Package, config.Activity}] = p.Entry.Lowered
		}
		seenLowered := map[string]bool{}
		for _, a := range p.PureCalls.Activities {
			key := [2]string{a.Activity.Package, a.Activity.Name}
			spec := contracts.Packages[a.Activity.Package]
			if !expectedActivities[key] || lowered[key] != "" || !loweredName.MatchString(a.Activity.Lowered) || seenLowered[a.Activity.Lowered] || a.Source != spec.Source {
				return Bindings{}, invalid
			}
			body, ok := activityBody(files[spec.Mirror], a.Activity.Name)
			if !ok || a.BodySHA != digest(body) {
				return Bindings{}, invalid
			}
			lowered[key] = a.Activity.Lowered
			seenLowered[a.Activity.Lowered] = true
			if prefix == "checked-matcher" && a.Activity.Package == "gooo/service-domain" {
				validators[a.Activity.Name] = a.Activity.Lowered
			}
		}
		if len(lowered) != len(expectedActivities) || lowered[[2]string{config.Package, config.Activity}] != p.Entry.Lowered {
			return Bindings{}, invalid
		}
		var body bodyRecord
		if json.Unmarshal(files["receipts/"+prefix+"-body-codegen.json"], &body) != nil {
			return Bindings{}, invalid
		}
		r := body.Report
		input := "(" + strings.TrimSuffix(strings.Repeat("string,", config.N), ",") + ")"
		if r.Schema != "gooo/body-codegen-report/v3" || r.Decision != "PASS" || r.Activity != p.Entry.Lowered || r.ID != config.ID || r.Input != input || r.Output != config.GoOutput || r.Compiler != CompilerPin || r.Source != digest([]byte(p.Lowered)) || body.Source == "" || r.Generated != digest([]byte(body.Source)) || r.Replay != r.Generated || !r.Typecheck || !r.Deterministic || r.Writes != 0 {
			return Bindings{}, invalid
		}
		projected, err := format.Source([]byte(body.Source))
		if err != nil || !bytes.Equal(projected, files["internal/"+prefix+"/generated.go"]) {
			return Bindings{}, invalid
		}
		allowedRecords, seenRecords := map[string]bool{}, map[string]bool{}
		for _, name := range config.Records {
			allowedRecords[name] = true
		}
		if len(r.Records) != len(allowedRecords) {
			return Bindings{}, invalid
		}
		for _, rec := range r.Records {
			spec, known := contracts.Records[rec.Name]
			if !known || !allowedRecords[rec.Name] || seenRecords[rec.Name] || rec.ID != spec.ID || len(rec.Fields) != len(spec.Fields) {
				return Bindings{}, invalid
			}
			seenRecords[rec.Name] = true
			for i, f := range rec.Fields {
				expected := spec.Fields[i]
				if f.Name != expected[0] || f.ID != expected[1] || f.Type != "urn:gooo:type:"+expected[2] {
					return Bindings{}, invalid
				}
			}
		}
	}
	adapter, err := format.Source([]byte(adapterSource(entries, validators)))
	if err != nil || !bytes.Equal(adapter, files["adapter_generated.go"]) {
		return Bindings{}, invalid
	}
	return Bindings{CompilerSourceSHA: CompilerPin, GenerationSHA256: digest(manifest), GenerationOrigin: g.Origin, Assets: g.Assets}, nil
}

type activityContract struct {
	ID     string `json:"id"`
	N      int    `json:"n"`
	Output string `json:"output"`
}
type packageContract struct {
	Source     string                      `json:"source"`
	Mirror     string                      `json:"mirror"`
	Entities   map[string]string           `json:"entities"`
	Activities map[string]activityContract `json:"activities"`
}
type recordContract struct {
	ID     string      `json:"id"`
	Fields [][3]string `json:"fields"`
}
type routeContract struct {
	Package          string      `json:"package"`
	Activity         string      `json:"activity"`
	ID               string      `json:"id"`
	N                int         `json:"n"`
	Output           string      `json:"output"`
	GoOutput         string      `json:"go_output"`
	Packages         []string    `json:"packages"`
	ManifestPackages []string    `json:"manifest_packages"`
	Workspace        string      `json:"workspace"`
	Edges            [][2]string `json:"edges"`
	Records          []string    `json:"records"`
}
type bodyRecord struct {
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

var loweredName = regexp.MustCompile(`^GoooPackage[0-9]+Activity[A-Za-z0-9]+$`)

func textInputs(actual []string, n int, want string) bool {
	if len(actual) != n {
		return false
	}
	for _, s := range actual {
		if s != want {
			return false
		}
	}
	return true
}
func equalJSON(a, b []byte) bool {
	var av, bv any
	return json.Unmarshal(a, &av) == nil && json.Unmarshal(b, &bv) == nil && reflect.DeepEqual(av, bv)
}
func activityBody(source []byte, name string) ([]byte, bool) {
	parts := strings.SplitN(string(source), "activity "+name+"(", 2)
	if len(parts) != 2 {
		return nil, false
	}
	body := strings.SplitN(parts[1], "`", 3)
	if len(body) != 3 {
		return nil, false
	}
	return []byte(body[1]), true
}
func adapterSource(entries, validators map[string]string) string {
	prefixes := []string{"matcher", "impact", "refinement", "checked-matcher", "checked-impact", "checked-refinement"}
	var b strings.Builder
	b.WriteString("// Code generated from pinned compiler-owned entry identities; DO NOT EDIT.\npackage servicepolicy\nimport (\n")
	for _, prefix := range prefixes {
		fmt.Fprintf(&b, " %s %q\n", strings.ReplaceAll(prefix, "-", ""), "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/internal/"+prefix)
	}
	b.WriteString(")\n")
	for _, prefix := range prefixes {
		kind := strings.TrimPrefix(prefix, "checked-")
		title := strings.ToUpper(kind[:1]) + kind[1:]
		suffix, out := "", "any"
		if strings.HasPrefix(prefix, "checked-") {
			suffix = "Checked"
		}
		if prefix == "matcher" {
			out = "bool"
		}
		args := map[string]string{"matcher": "f.Pattern,f.ReqType,f.ReqName", "impact": "f.Before,f.After,f.ReqType,f.ReqName", "refinement": "f.Before,f.After"}[kind]
		fmt.Fprintf(&b, "func native%s%s(f %sFacts) %s { return %s.%s(%s) }\n", title, suffix, title, out, strings.ReplaceAll(prefix, "-", ""), entries[prefix], args)
	}
	for _, kind := range []string{"Type", "Name", "Pattern"} {
		fmt.Fprintf(&b, "func nativeValid%s(value string) bool { return checkedmatcher.%s(value) }\n", kind, validators["ValidateService"+kind])
	}
	return b.String()
}
func digest(raw []byte) string       { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func sourceDigest(raw []byte) string { encoded, _ := json.Marshal(string(raw)); return digest(encoded) }

// Fixed contract tables bind checked defaults and explicitly named raw compatibility.
const bindingContracts = `{
  "packages": {
    "app/service-grant-impact": {
      "activities": {
        "CompareServiceGrant": {
          "id": "gooo://service-impact/compare",
          "n": 4,
          "output": "gooo://service-impact/decision/v1"
        }
      },
      "entities": {
        "GrantImpact": "gooo://service-impact/decision/v1",
        "Text": "gooo://std/text"
      },
      "mirror": "impact.gooo",
      "source": "examples/agentmesh-service-policy/impact.gooo"
    },
    "app/service-grant-refinement": {
      "activities": {
        "AnalyzeServiceGrant": {
          "id": "gooo://service-refinement/analyze",
          "n": 2,
          "output": "gooo://service-refinement/decision/v1"
        }
      },
      "entities": {
        "GrantRefinement": "gooo://service-refinement/decision/v1",
        "Text": "gooo://std/text"
      },
      "mirror": "refinement.gooo",
      "source": "examples/agentmesh-service-policy/refinement.gooo"
    },
    "go/google/agentmesh/api": {
      "activities": {
        "MatchServicePattern": {
          "id": "gooo://agentmesh-prototype/match-service-pattern",
          "n": 3,
          "output": "gooo://std/boolean"
        }
      },
      "entities": {
        "Boolean": "gooo://std/boolean",
        "Text": "gooo://std/text"
      },
      "mirror": "matcher.gooo",
      "source": "ports/agentmesh/service_pattern.gooo"
    },
    "gooo/service-domain": {
      "activities": {
        "ValidateServiceName": {
          "id": "gooo://service-domain/name",
          "n": 1,
          "output": "gooo://std/boolean"
        },
        "ValidateServicePattern": {
          "id": "gooo://service-domain/pattern",
          "n": 1,
          "output": "gooo://std/boolean"
        },
        "ValidateServiceType": {
          "id": "gooo://service-domain/type",
          "n": 1,
          "output": "gooo://std/boolean"
        }
      },
      "entities": {
        "Boolean": "gooo://std/boolean",
        "Text": "gooo://std/text"
      },
      "mirror": "domain.gooo",
      "source": "examples/agentmesh-service-policy/domain.gooo"
    },
    "gooo/service-policy/checked-impact": {
      "activities": {
        "CompareServiceGrantChecked": {
          "id": "gooo://service-policy/checked-impact/evaluate/v1",
          "n": 4,
          "output": "gooo://service-policy/checked-impact/decision/v1"
        }
      },
      "entities": {
        "CheckedImpactDecision": "gooo://service-policy/checked-impact/decision/v1",
        "Text": "gooo://std/text"
      },
      "mirror": "checked_impact.gooo",
      "source": "examples/agentmesh-service-policy/checked_impact.gooo"
    },
    "gooo/service-policy/checked-matcher": {
      "activities": {
        "MatchServicePatternChecked": {
          "id": "gooo://service-policy/checked-matcher/evaluate/v1",
          "n": 3,
          "output": "gooo://service-policy/checked-matcher/decision/v1"
        }
      },
      "entities": {
        "CheckedMatcherDecision": "gooo://service-policy/checked-matcher/decision/v1",
        "Text": "gooo://std/text"
      },
      "mirror": "checked_matcher.gooo",
      "source": "examples/agentmesh-service-policy/checked_matcher.gooo"
    },
    "gooo/service-policy/checked-refinement": {
      "activities": {
        "AnalyzeServiceGrantChecked": {
          "id": "gooo://service-policy/checked-refinement/evaluate/v1",
          "n": 2,
          "output": "gooo://service-policy/checked-refinement/decision/v1"
        }
      },
      "entities": {
        "CheckedRefinementDecision": "gooo://service-policy/checked-refinement/decision/v1",
        "Text": "gooo://std/text"
      },
      "mirror": "checked_refinement.gooo",
      "source": "examples/agentmesh-service-policy/checked_refinement.gooo"
    }
  },
  "records": {
    "CheckedImpactDecision": {
      "fields": [
        [
          "domain_status",
          "gooo://service-policy/checked-impact/domain_status",
          "string"
        ],
        [
          "invalid_field",
          "gooo://service-policy/checked-impact/invalid_field",
          "string"
        ],
        [
          "before_match",
          "gooo://service-policy/checked-impact/before_match",
          "boolean"
        ],
        [
          "after_match",
          "gooo://service-policy/checked-impact/after_match",
          "boolean"
        ],
        [
          "change",
          "gooo://service-policy/checked-impact/change",
          "string"
        ]
      ],
      "id": "gooo://service-policy/checked-impact/decision/v1"
    },
    "CheckedMatcherDecision": {
      "fields": [
        [
          "domain_status",
          "gooo://service-policy/checked-matcher/domain_status",
          "string"
        ],
        [
          "invalid_field",
          "gooo://service-policy/checked-matcher/invalid_field",
          "string"
        ],
        [
          "matched",
          "gooo://service-policy/checked-matcher/matched",
          "boolean"
        ]
      ],
      "id": "gooo://service-policy/checked-matcher/decision/v1"
    },
    "CheckedRefinementDecision": {
      "fields": [
        [
          "domain_status",
          "gooo://service-policy/checked-refinement/domain_status",
          "string"
        ],
        [
          "invalid_field",
          "gooo://service-policy/checked-refinement/invalid_field",
          "string"
        ],
        [
          "status",
          "gooo://service-policy/checked-refinement/status",
          "string"
        ],
        [
          "rule_id",
          "gooo://service-policy/checked-refinement/rule_id",
          "string"
        ],
        [
          "supported_subset",
          "gooo://service-policy/checked-refinement/supported_subset",
          "boolean"
        ],
        [
          "has_witness",
          "gooo://service-policy/checked-refinement/has_witness",
          "boolean"
        ],
        [
          "witness_type",
          "gooo://service-policy/checked-refinement/witness_type",
          "string"
        ],
        [
          "witness_name",
          "gooo://service-policy/checked-refinement/witness_name",
          "string"
        ],
        [
          "before_match",
          "gooo://service-policy/checked-refinement/before_match",
          "boolean"
        ],
        [
          "after_match",
          "gooo://service-policy/checked-refinement/after_match",
          "boolean"
        ]
      ],
      "id": "gooo://service-policy/checked-refinement/decision/v1"
    },
    "GrantImpact": {
      "fields": [
        [
          "before_match",
          "gooo://service-impact/before",
          "boolean"
        ],
        [
          "after_match",
          "gooo://service-impact/after",
          "boolean"
        ],
        [
          "change",
          "gooo://service-impact/change",
          "string"
        ]
      ],
      "id": "gooo://service-impact/decision/v1"
    },
    "GrantRefinement": {
      "fields": [
        [
          "status",
          "gooo://service-refinement/status",
          "string"
        ],
        [
          "rule_id",
          "gooo://service-refinement/rule_id",
          "string"
        ],
        [
          "supported_subset",
          "gooo://service-refinement/supported_subset",
          "boolean"
        ],
        [
          "has_witness",
          "gooo://service-refinement/has_witness",
          "boolean"
        ],
        [
          "witness_type",
          "gooo://service-refinement/witness_type",
          "string"
        ],
        [
          "witness_name",
          "gooo://service-refinement/witness_name",
          "string"
        ],
        [
          "before_match",
          "gooo://service-refinement/before_match",
          "boolean"
        ],
        [
          "after_match",
          "gooo://service-refinement/after_match",
          "boolean"
        ]
      ],
      "id": "gooo://service-refinement/decision/v1"
    }
  },
  "routes": {
    "checked-impact": {
      "activity": "CompareServiceGrantChecked",
      "canonical_workspace": "service-grant-impact.workspace.json",
      "edges": [
        [
          "gooo/service-policy/checked-impact:CompareServiceGrantChecked",
          "gooo/service-domain:ValidateServicePattern"
        ],
        [
          "gooo/service-policy/checked-impact:CompareServiceGrantChecked",
          "gooo/service-domain:ValidateServicePattern"
        ],
        [
          "gooo/service-policy/checked-impact:CompareServiceGrantChecked",
          "gooo/service-domain:ValidateServiceType"
        ],
        [
          "gooo/service-policy/checked-impact:CompareServiceGrantChecked",
          "gooo/service-domain:ValidateServiceName"
        ],
        [
          "gooo/service-policy/checked-impact:CompareServiceGrantChecked",
          "app/service-grant-impact:CompareServiceGrant"
        ],
        [
          "app/service-grant-impact:CompareServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-impact:CompareServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceType"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceName"
        ]
      ],
      "fields": [
        "before",
        "after",
        "req_type",
        "req_name"
      ],
      "go_output": "CheckedImpactDecision",
      "id": "gooo://service-policy/checked-impact/evaluate/v1",
      "manifest_packages": [
        "gooo/service-domain",
        "go/google/agentmesh/api",
        "app/service-grant-impact",
        "gooo/service-policy/checked-impact"
      ],
      "n": 4,
      "output": "CheckedImpactDecision",
      "package": "gooo/service-policy/checked-impact",
      "packages": [
        "go/google/agentmesh/api",
        "app/service-grant-impact",
        "gooo/service-domain",
        "gooo/service-policy/checked-impact"
      ],
      "records": [
        "GrantImpact",
        "CheckedImpactDecision"
      ],
      "workspace": "impact-workspace.json"
    },
    "checked-matcher": {
      "activity": "MatchServicePatternChecked",
      "canonical_workspace": "agentmesh-matcher.workspace.json",
      "edges": [
        [
          "gooo/service-policy/checked-matcher:MatchServicePatternChecked",
          "gooo/service-domain:ValidateServicePattern"
        ],
        [
          "gooo/service-policy/checked-matcher:MatchServicePatternChecked",
          "gooo/service-domain:ValidateServiceType"
        ],
        [
          "gooo/service-policy/checked-matcher:MatchServicePatternChecked",
          "gooo/service-domain:ValidateServiceName"
        ],
        [
          "gooo/service-policy/checked-matcher:MatchServicePatternChecked",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceType"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceName"
        ]
      ],
      "fields": [
        "pattern",
        "req_type",
        "req_name"
      ],
      "go_output": "CheckedMatcherDecision",
      "id": "gooo://service-policy/checked-matcher/evaluate/v1",
      "manifest_packages": [
        "gooo/service-domain",
        "go/google/agentmesh/api",
        "gooo/service-policy/checked-matcher"
      ],
      "n": 3,
      "output": "CheckedMatcherDecision",
      "package": "gooo/service-policy/checked-matcher",
      "packages": [
        "go/google/agentmesh/api",
        "gooo/service-domain",
        "gooo/service-policy/checked-matcher"
      ],
      "records": [
        "CheckedMatcherDecision"
      ],
      "workspace": "matcher-workspace.json"
    },
    "checked-refinement": {
      "activity": "AnalyzeServiceGrantChecked",
      "canonical_workspace": "service-grant-refinement.workspace.json",
      "edges": [
        [
          "gooo/service-policy/checked-refinement:AnalyzeServiceGrantChecked",
          "gooo/service-domain:ValidateServicePattern"
        ],
        [
          "gooo/service-policy/checked-refinement:AnalyzeServiceGrantChecked",
          "gooo/service-domain:ValidateServicePattern"
        ],
        [
          "gooo/service-policy/checked-refinement:AnalyzeServiceGrantChecked",
          "app/service-grant-refinement:AnalyzeServiceGrant"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceType"
        ],
        [
          "gooo/service-domain:ValidateServicePattern",
          "gooo/service-domain:ValidateServiceName"
        ]
      ],
      "fields": [
        "before",
        "after"
      ],
      "go_output": "CheckedRefinementDecision",
      "id": "gooo://service-policy/checked-refinement/evaluate/v1",
      "manifest_packages": [
        "gooo/service-domain",
        "go/google/agentmesh/api",
        "app/service-grant-refinement",
        "gooo/service-policy/checked-refinement"
      ],
      "n": 2,
      "output": "CheckedRefinementDecision",
      "package": "gooo/service-policy/checked-refinement",
      "packages": [
        "go/google/agentmesh/api",
        "app/service-grant-refinement",
        "gooo/service-domain",
        "gooo/service-policy/checked-refinement"
      ],
      "records": [
        "GrantRefinement",
        "CheckedRefinementDecision"
      ],
      "workspace": "refinement-workspace.json"
    },
    "impact": {
      "activity": "CompareServiceGrant",
      "canonical_workspace": "service-grant-impact-raw.workspace.json",
      "edges": [
        [
          "app/service-grant-impact:CompareServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-impact:CompareServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ]
      ],
      "fields": [],
      "go_output": "GrantImpact",
      "id": "gooo://service-impact/compare",
      "manifest_packages": [
        "go/google/agentmesh/api",
        "app/service-grant-impact"
      ],
      "n": 4,
      "output": "GrantImpact",
      "package": "app/service-grant-impact",
      "packages": [
        "go/google/agentmesh/api",
        "app/service-grant-impact"
      ],
      "records": [
        "GrantImpact"
      ],
      "workspace": "impact-raw-workspace.json"
    },
    "matcher": {
      "activity": "MatchServicePattern",
      "canonical_workspace": "agentmesh-matcher-raw.workspace.json",
      "edges": [],
      "fields": [],
      "go_output": "bool",
      "id": "gooo://agentmesh-prototype/match-service-pattern",
      "manifest_packages": [
        "go/google/agentmesh/api"
      ],
      "n": 3,
      "output": "Boolean",
      "package": "go/google/agentmesh/api",
      "packages": [
        "go/google/agentmesh/api"
      ],
      "records": [],
      "workspace": "matcher-raw-workspace.json"
    },
    "refinement": {
      "activity": "AnalyzeServiceGrant",
      "canonical_workspace": "service-grant-refinement-raw.workspace.json",
      "edges": [
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ],
        [
          "app/service-grant-refinement:AnalyzeServiceGrant",
          "go/google/agentmesh/api:MatchServicePattern"
        ]
      ],
      "fields": [],
      "go_output": "GrantRefinement",
      "id": "gooo://service-refinement/analyze",
      "manifest_packages": [
        "go/google/agentmesh/api",
        "app/service-grant-refinement"
      ],
      "n": 2,
      "output": "GrantRefinement",
      "package": "app/service-grant-refinement",
      "packages": [
        "go/google/agentmesh/api",
        "app/service-grant-refinement"
      ],
      "records": [
        "GrantRefinement"
      ],
      "workspace": "refinement-raw-workspace.json"
    }
  }
}`
