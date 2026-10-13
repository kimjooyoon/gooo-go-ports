package nativepolicy

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"strings"
)

const CompilerPin = "83928fa4538245abe33b08549da47f8cbdc50ed2"

// These hashes detect inconsistency in a trusted executable, not issuer trust.
// Canonical mirrors are compared with the original importing workspace by
// generate.py --check. The original imports are lowered by actual seed execution.
//
//go:embed canonical/* receipts/* internal/scalar/generated.go internal/api/generated.go compiler.json adapter_generated.go model.go bridge.go bindings.go input.go cmd/evaluate/main.go generate.py generation.json
var assets embed.FS

var boundNames = []string{
	"canonical/scalar_access.gooo", "canonical/main.gooo", "canonical/cel-scalar.workspace.json", "canonical/cel-api-policy.workspace.json",
	"compiler.json", "adapter_generated.go", "model.go", "bridge.go", "bindings.go", "input.go", "cmd/evaluate/main.go", "generate.py", "canonical/source-pin.json",
	"receipts/scalar-seed-inputs.json", "receipts/scalar-execution.json", "receipts/scalar-replay.json", "receipts/scalar-lowered.gooo", "receipts/scalar-body-codegen.json", "internal/scalar/generated.go",
	"receipts/api-seed-inputs.json", "receipts/api-execution.json", "receipts/api-replay.json", "receipts/api-lowered.gooo", "receipts/api-body-codegen.json", "internal/api/generated.go",
}

type generation struct {
	Schema string            `json:"schema"`
	Assets map[string]string `json:"assets"`
}

type programRecord struct {
	Schema string `json:"schema"`
	Entry  struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
		Lowered  string `json:"lowered_name"`
	} `json:"entry"`
	LoweredSource string `json:"lowered_gooo_source"`
	Image         struct {
		Entry struct {
			Package  string   `json:"package_path"`
			Source   string   `json:"source"`
			Activity string   `json:"activity"`
			Inputs   []string `json:"inputs"`
			Output   string   `json:"output"`
		} `json:"entry"`
		Packages []struct {
			Path    string `json:"path"`
			Exports []struct {
				Name   string   `json:"name"`
				Kind   string   `json:"kind"`
				ID     string   `json:"id"`
				Inputs []string `json:"input_types"`
				Output string   `json:"output_type"`
			} `json:"exports"`
			Sources []struct {
				Filename string `json:"filename"`
				Digest   string `json:"source_digest"`
			} `json:"sources"`
		} `json:"packages"`
	} `json:"workspace_image"`
	PureCalls struct {
		Activities []struct {
			Source     string `json:"source"`
			BodyDigest string `json:"source_body_sha256"`
		} `json:"activities"`
		Sites []struct {
			Caller string `json:"caller"`
			Callee string `json:"callee"`
		} `json:"sites"`
	} `json:"pure_calls"`
}

type executionRecord struct {
	Schema         string `json:"schema"`
	Decision       string `json:"decision"`
	ManifestDigest string `json:"manifest_digest"`
	InputsDigest   string `json:"inputs_digest"`
	Result         struct {
		Program programRecord `json:"program"`
		Runtime struct {
			Producer         string `json:"producer_source_sha"`
			GoVersion        string `json:"go_version"`
			ProjectionReplay bool   `json:"projection_replayed"`
			RuntimeReplay    bool   `json:"runtime_replayed"`
			Runs             []struct {
				Started   bool `json:"started"`
				Completed bool `json:"completed"`
				Exit      int  `json:"exit_code"`
				TimedOut  bool `json:"timed_out"`
				Truncated bool `json:"output_truncated"`
			} `json:"runs"`
		} `json:"runtime"`
	} `json:"result"`
}

func SourceBindings() (Bindings, error) {
	files := make(map[string][]byte, len(boundNames))
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
	invalid := errors.New("imported CEL policy source bindings invalid")
	var record generation
	if json.Unmarshal(manifest, &record) != nil || record.Schema != "gooo/cel-public-generation/v1" || len(record.Assets) != len(boundNames) || len(files) != len(boundNames) {
		return Bindings{}, invalid
	}
	for _, name := range boundNames {
		raw, ok := files[name]
		if !ok || len(raw) == 0 || record.Assets[name] != digest(raw) {
			return Bindings{}, invalid
		}
	}
	var compiler struct {
		Schema    string `json:"schema"`
		Source    string `json:"compiler_source_sha"`
		Revision  string `json:"vcs_revision"`
		Status    string `json:"source_status"`
		Modified  string `json:"vcs_modified"`
		GoVersion string `json:"go_version"`
		NativeGo  string `json:"native_go_required"`
	}
	if json.Unmarshal(files["compiler.json"], &compiler) != nil || compiler.Schema != "gooo/build-identity/v1" || compiler.Source != CompilerPin || compiler.Revision != CompilerPin || compiler.Status != "CLEAN_VCS" || compiler.Modified != "false" || compiler.GoVersion != "go1.27.2" || compiler.NativeGo != "go1.27.2" {
		return Bindings{}, invalid
	}
	for _, c := range []struct{ prefix, workspace, pkg, activity, id, input, output string }{
		{"scalar", "cel-scalar.workspace.json", "go/cel.dev/cel-go/codelab", "ScalarAccess", "gooo://cel-example/scalar-access", "(bool,string,bool,string)", "AccessDecision"},
		{"api", "cel-api-policy.workspace.json", "app/cel-api-policy", "APIAccess", "gooo://api-example/api-access", "(bool,string,bool,string,bool,string,string,string,string)", "ApiDecision"},
	} {
		var execution, replay executionRecord
		if json.Unmarshal(files["receipts/"+c.prefix+"-execution.json"], &execution) != nil || json.Unmarshal(files["receipts/"+c.prefix+"-replay.json"], &replay) != nil {
			return Bindings{}, invalid
		}
		p := execution.Result.Program
		pRaw, _ := json.Marshal(p)
		rpRaw, _ := json.Marshal(replay.Result.Program)
		if !bytes.Equal(pRaw, rpRaw) || !bytes.Equal([]byte(p.LoweredSource), files["receipts/"+c.prefix+"-lowered.gooo"]) {
			return Bindings{}, invalid
		}
		for _, e := range []executionRecord{execution, replay} {
			if e.Schema != "gooo/workspace-body-execution-receipt/v1" || (e.Decision != "OBSERVED" && e.Decision != "PASS") || e.ManifestDigest != digest(files["canonical/"+c.workspace]) || e.InputsDigest != digest(files["receipts/"+c.prefix+"-seed-inputs.json"]) || e.Result.Runtime.Producer != CompilerPin || !strings.HasPrefix(e.Result.Runtime.GoVersion, "go version go1.27.2 ") || !e.Result.Runtime.ProjectionReplay || !e.Result.Runtime.RuntimeReplay || len(e.Result.Runtime.Runs) == 0 {
				return Bindings{}, invalid
			}
			for _, r := range e.Result.Runtime.Runs {
				if !r.Started || !r.Completed || r.Exit != 0 || r.TimedOut || r.Truncated {
					return Bindings{}, invalid
				}
			}
		}
		if p.Schema != "gooo/workspace-body-program/v1" || p.Entry.Package != c.pkg || p.Entry.Activity != c.activity || p.Entry.Lowered == "" {
			return Bindings{}, invalid
		}
		sourceName := "ports/cel/scalar_access.gooo"
		inputNames := []string{"Boolean", "Text", "Boolean", "Text"}
		if c.prefix == "api" {
			sourceName = "examples/cel-api-policy/main.gooo"
			inputNames = append(inputNames, "Boolean", "Text", "Text", "Text", "Text")
		}
		ie := p.Image.Entry
		if ie.Package != c.pkg || ie.Source != sourceName || ie.Activity != c.activity || ie.Output != c.output || !sameStrings(ie.Inputs, inputNames) {
			return Bindings{}, invalid
		}
		for _, pkg := range p.Image.Packages {
			name, id, output := "ScalarAccess", "gooo://cel-example/scalar-access", "gooo://cel-example/access-decision/v1"
			inputs := []string{"gooo://std/boolean", "gooo://std/text", "gooo://std/boolean", "gooo://std/text"}
			if pkg.Path == "app/cel-api-policy" && c.prefix == "api" {
				name, id, output = "APIAccess", "gooo://api-example/api-access", "gooo://api-example/decision/v1"
				inputs = append(inputs, "gooo://std/boolean", "gooo://std/text", "gooo://std/text", "gooo://std/text", "gooo://std/text")
			} else if pkg.Path != "go/cel.dev/cel-go/codelab" {
				return Bindings{}, invalid
			}
			found := 0
			for _, e := range pkg.Exports {
				if e.Kind == "activity" {
					found++
					if e.Name != name || e.ID != id || e.Output != output || !sameStrings(e.Inputs, inputs) {
						return Bindings{}, invalid
					}
				}
			}
			if found != 1 {
				return Bindings{}, invalid
			}
		}
		expected := map[string]string{"ports/cel/scalar_access.gooo": "canonical/scalar_access.gooo"}
		if c.prefix == "api" {
			expected["examples/cel-api-policy/main.gooo"] = "canonical/main.gooo"
		}
		seen := map[string]bool{}
		for _, pkg := range p.Image.Packages {
			for _, s := range pkg.Sources {
				mirror, ok := expected[s.Filename]
				if !ok || seen[s.Filename] || s.Digest != sourceDigest(files[mirror]) {
					return Bindings{}, invalid
				}
				seen[s.Filename] = true
			}
		}
		if len(seen) != len(expected) {
			return Bindings{}, invalid
		}
		if c.prefix == "api" {
			if len(p.PureCalls.Sites) != 1 || p.PureCalls.Sites[0].Caller != "app/cel-api-policy:APIAccess" || p.PureCalls.Sites[0].Callee != "go/cel.dev/cel-go/codelab:ScalarAccess" || len(p.PureCalls.Activities) != 2 {
				return Bindings{}, invalid
			}
			bodies := map[string]bool{}
			for _, a := range p.PureCalls.Activities {
				mirror, ok := expected[a.Source]
				if !ok || bodies[a.Source] {
					return Bindings{}, invalid
				}
				bodies[a.Source] = true
				source := string(files[mirror])
				start := strings.Index(source, "computes `")
				end := strings.LastIndex(source, "`")
				if start < 0 || end < start+10 || a.BodyDigest != digest([]byte(source[start+10:end])) {
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
		if json.Unmarshal(files["receipts/"+c.prefix+"-body-codegen.json"], &body) != nil {
			return Bindings{}, invalid
		}
		r := body.Report
		if body.Source == "" || r.Schema != "gooo/body-codegen-report/v3" || r.Decision != "PASS" || r.Activity != p.Entry.Lowered || r.ID != c.id || r.Input != c.input || r.Output != c.output || r.Compiler != CompilerPin || r.Source != digest([]byte(p.LoweredSource)) || r.Generated != digest([]byte(body.Source)) || r.Replay != r.Generated || !r.Typecheck || !r.Deterministic || r.Writes != 0 {
			return Bindings{}, invalid
		}
		expectedRecords := 1
		if c.prefix == "api" {
			expectedRecords = 2
		}
		if len(r.Records) != expectedRecords {
			return Bindings{}, invalid
		}
		foundRecords := map[string]bool{}
		for _, rec := range r.Records {
			base := "gooo://cel-example/access-decision/"
			id := "gooo://cel-example/access-decision/v1"
			names := []string{"allowed", "evaluation_error", "result", "group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal"}
			stringsSet := map[string]bool{"result": true, "effective_principal": true}
			if rec.Name == "ApiDecision" && c.prefix == "api" {
				base, id = "gooo://api-example/decision/", "gooo://api-example/decision/v1"
				names = []string{"allowed", "reason", "authenticated", "tenant_present", "tenant_matched", "action_present", "action_matched", "access_allowed", "access_error", "access_result", "group_matched", "principal_matched", "group_missing", "principal_defaulted", "effective_principal"}
				stringsSet = map[string]bool{"reason": true, "access_result": true, "effective_principal": true}
			} else if rec.Name != "AccessDecision" {
				return Bindings{}, invalid
			}
			if foundRecords[rec.Name] || rec.ID != id || len(rec.Fields) != len(names) {
				return Bindings{}, invalid
			}
			foundRecords[rec.Name] = true
			for i, f := range rec.Fields {
				typ := "urn:gooo:type:boolean"
				if stringsSet[names[i]] {
					typ = "urn:gooo:type:string"
				}
				if f.Name != names[i] || f.ID != base+strings.ReplaceAll(names[i], "_", "-") || f.Type != typ {
					return Bindings{}, invalid
				}
			}
		}
		projection, err := format.Source([]byte("// Code generated from compiler-owned workspace lowering; DO NOT EDIT.\n" + body.Source))
		if err != nil || !bytes.Equal(projection, files["internal/"+c.prefix+"/generated.go"]) {
			return Bindings{}, invalid
		}
	}
	return Bindings{CompilerSourceSHA: CompilerPin, Assets: record.Assets, GenerationSHA256: digest(manifest)}, nil
}
func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }

// Workspace image hashes the JSON-encoded source string, not its raw bytes.
func sourceDigest(raw []byte) string { encoded, _ := json.Marshal(string(raw)); return digest(encoded) }

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
