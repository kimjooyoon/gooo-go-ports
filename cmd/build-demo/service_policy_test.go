package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func servicePolicyTestDocuments(t *testing.T) (map[string][]byte, lockfile) {
	t.Helper()
	documents := map[string][]byte{}
	read := func(name string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		documents[name] = raw
	}
	read(servicePolicyModule + "/generation.json")
	var generation servicePolicyGeneration
	if err := servicePolicyDecode(documents[servicePolicyModule+"/generation.json"], &generation); err != nil {
		t.Fatal(err)
	}
	for name := range generation.Assets {
		read(servicePolicyModule + "/" + name)
	}
	for name := range generation.PublicSources {
		read(name)
	}
	for _, name := range servicePolicySourceFiles {
		read(servicePolicyModule + "/" + name)
	}
	read("upstream.json")
	var lock lockfile
	if json.Unmarshal(documents["upstream.json"], &lock) != nil {
		t.Fatal("invalid repository pin")
	}
	return documents, lock
}

func TestServicePolicyBuildRequiresBoundSourceAndCompiler(t *testing.T) {
	documents, lock := servicePolicyTestDocuments(t)
	bound, err := servicePolicyCheckDocuments(documents, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(bound.Assets) != 89 || bound.CompilerSourceSHA != lock.CompilerRevision || bound.GenerationSHA256 != "sha256:"+digest(documents[servicePolicyModule+"/generation.json"]) || bound.GenerationOrigin != "fresh_pinned_generation" {
		t.Fatal("incomplete expected native SourceBindings")
	}
	for _, path := range []string{
		servicePolicyModule + "/internal/checked-refinement/generated.go",
		servicePolicyModule + "/bridge.go",
		servicePolicyModule + "/upstream/api/tar.go.txt",
		servicePolicyModule + "/compiler.json",
		"ports/agentmesh/service_pattern.gooo",
		"agentmesh-matcher.workspace.json",
	} {
		mutated := maps.Clone(documents)
		mutated[path] = append(append([]byte{}, documents[path]...), '\n')
		if _, err := servicePolicyCheckDocuments(mutated, lock); err == nil {
			t.Fatal("accepted source drift", path)
		}
		delete(mutated, path)
		if _, err := servicePolicyCheckDocuments(mutated, lock); err == nil {
			t.Fatal("accepted missing source", path)
		}
	}
	for _, changed := range []lockfile{
		{CompilerRevision: strings.Repeat("0", 40), CompilerVersion: lock.CompilerVersion, GoVersion: lock.GoVersion},
		{CompilerRevision: lock.CompilerRevision, CompilerVersion: "different", GoVersion: lock.GoVersion},
		{CompilerRevision: lock.CompilerRevision, CompilerVersion: lock.CompilerVersion, GoVersion: "go1.26.0"},
	} {
		if _, err := servicePolicyCheckDocuments(documents, changed); err == nil {
			t.Fatal("accepted changed compiler/toolchain identity")
		}
	}
}

func TestServicePolicyBuildRejectsMalformedGenerationAndPaths(t *testing.T) {
	documents, lock := servicePolicyTestDocuments(t)
	path := servicePolicyModule + "/generation.json"
	original := documents[path]
	for _, mutate := range []func(map[string]any){
		func(g map[string]any) { g["extra"] = true },
		func(g map[string]any) { g["origin"] = "saved_receipt_replay" },
		func(g map[string]any) { g["schema"] = "unknown" },
		func(g map[string]any) { delete(g["assets"].(map[string]any), "bridge.go") },
		func(g map[string]any) {
			assets := g["assets"].(map[string]any)
			assets["../escape"] = assets["bridge.go"]
			delete(assets, "bridge.go")
		},
		func(g map[string]any) {
			delete(g["public_sources"].(map[string]any), "agentmesh-matcher.workspace.json")
		},
	} {
		var g map[string]any
		if json.Unmarshal(original, &g) != nil {
			t.Fatal("invalid baseline")
		}
		mutate(g)
		changed := maps.Clone(documents)
		changed[path], _ = json.Marshal(g)
		if _, err := servicePolicyCheckDocuments(changed, lock); err == nil {
			t.Fatal("accepted malformed generation")
		}
	}
	for _, raw := range []string{`{"schema":"first","schema":"second"}`, `{} {}`, `{"assets":null,"assets":{}}`} {
		var generation servicePolicyGeneration
		if servicePolicyDecode([]byte(raw), &generation) == nil {
			t.Fatal("accepted ambiguous generation JSON")
		}
	}
	for _, name := range []string{".", "..", "../x", "/absolute", "a/../b", "a//b", `a\b`, ""} {
		if servicePolicyLocalPath(name) {
			t.Fatal("accepted ambiguous asset path", name)
		}
	}
}

func TestServicePolicyDisplayRosterAndBrowserImportBoundary(t *testing.T) {
	documents, _ := servicePolicyTestDocuments(t)
	seen := map[string]bool{}
	if len(servicePolicySourceFiles) != 32 {
		t.Fatal("published source roster changed")
	}
	for _, name := range servicePolicySourceFiles {
		if seen[name] || !servicePolicyLocalPath(name) || len(documents[servicePolicyModule+"/"+name]) == 0 {
			t.Fatal("missing, duplicate or non-local display source", name)
		}
		seen[name] = true
		if strings.HasPrefix(name, "apply/") || strings.HasPrefix(name, "oracle/") {
			t.Fatal("unrelated execution source added to browser display", name)
		}
	}
	imports := map[string]bool{}
	for _, name := range []string{"bridge.go", "main_js.go"} {
		source := documents[servicePolicyModule+"/cmd/browser/"+name]
		parsed, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(path, ".") {
				imports[path] = true
			}
		}
	}
	if len(imports) != 1 || !imports["github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"] {
		t.Fatal("browser must import only the native checked module, never an oracle or application host", imports)
	}
}
