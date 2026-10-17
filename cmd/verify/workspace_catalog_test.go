package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestVerificationCatalogSelectsPortsAndOnlyTheEntryConsumer(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.Catalog.Packages) != 56 || len(frozen.Sources) != 67 {
		t.Fatal("complete reviewed catalog roster changed")
	}
	if !reflect.DeepEqual(frozen.Catalog.Packages[54], sourcePackage{Path: "gooo/autobot/ai-lines", Sources: []string{"ports/autobot/ai_lines.gooo"}}) {
		t.Fatal("reviewed AutoBot adaptation must be exactly one separate package")
	}
	if !reflect.DeepEqual(frozen.Catalog.Packages[48], sourcePackage{Path: headerAdditionPackage, Sources: []string{headerAdditionSource}}) {
		t.Fatal("reviewed composition must be exactly one appended app package")
	}
	allIdentities, err := snapshotSourceIdentities(frozen.Catalog.Packages, frozen.Sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range allNativeSuites(t) {
		selected, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(s.Package, s.Activity, err)
		}
		if selected.Manifest.Schema != "gooo/package-workspace-manifest/v1" || selected.Manifest.Entry.Package != s.Package || selected.Manifest.Entry.Activity != s.Activity {
			t.Fatal("execution manifest or entry differs")
		}
		if _, exists := selected.Sources["ports/cel/scalar_access.gooo"]; exists {
			t.Fatal("bounded CEL rule leaked into an unrelated legacy workspace")
		}
		if _, exists := selected.Sources["examples/cel-api-policy/main.gooo"]; exists {
			t.Fatal("bounded CEL consumer leaked into an unrelated legacy workspace")
		}
		if _, exists := selected.Sources["ports/autobot/ai_lines.gooo"]; exists {
			t.Fatal("bounded Python adaptation leaked into a Go compatibility suite")
		}
		for _, name := range []string{"ports/agentmesh/service_pattern.gooo", "examples/agentmesh-service-policy/impact.gooo", "examples/agentmesh-service-policy/refinement.gooo", "examples/agentmesh-service-policy/domain.gooo", "examples/agentmesh-service-policy/checked_matcher.gooo", "examples/agentmesh-service-policy/checked_impact.gooo", "examples/agentmesh-service-policy/checked_refinement.gooo", "examples/agentmesh-service-policy/conditional_apply.gooo"} {
			if _, exists := selected.Sources[name]; exists {
				t.Fatal("bounded AgentMesh source leaked into an unrelated legacy workspace", name)
			}
		}
		var serialized map[string]json.RawMessage
		if err := json.Unmarshal(mustJSON(t, selected.Manifest), &serialized); err != nil || len(serialized) != 3 || serialized["schema"] == nil || serialized["entry"] == nil || serialized["packages"] == nil {
			t.Fatal("execution manifest does not use the exact lowercase JSON keys", err)
		}
		ports, consumers := 0, 0
		contract, _ := contractFor(s)
		image := workspaceImage{}
		for _, pkg := range selected.Manifest.Packages {
			if strings.HasPrefix(pkg.Path, "go/") {
				ports++
			} else {
				consumers++
				if pkg.Path != s.Package {
					t.Fatal("unrelated consumer leaked into execution", pkg.Path, s.Package)
				}
			}
			p := workspacePackage{Path: pkg.Path}
			for _, name := range pkg.Sources {
				if !reflect.DeepEqual(selected.Sources[name], frozen.Sources[name]) || selected.Identities[name] != allIdentities[name] {
					t.Fatal("selected source bytes or source owner/digest changed", name)
				}
				p.Sources = append(p.Sources, workspaceSource{Filename: name, Digest: selected.Identities[name].Digest})
			}
			if pkg.Path == s.Package {
				p.Exports = []workspaceExport{{Name: s.Activity, Kind: "activity", ID: contract.ID, InputTypes: contract.InputEntities, OutputType: whitespaceResultContract(s).Entity}}
			}
			image.Packages = append(image.Packages, p)
		}
		wantConsumers := 1
		if strings.HasPrefix(s.Package, "go/") {
			wantConsumers = 0
		}
		wantPorts := 10
		if isLiteralTextSuite(s) || isLiteralCutSuite(s) {
			wantPorts = 1
		}
		if isUUIDSuite(s) {
			wantPorts = 2
		}
		if isUUIDParseSuite(s) {
			wantPorts = 3
			if !reflect.DeepEqual(selected.Names, []string{"ports/google/uuid/quote.gooo", "ports/google/uuid/validate.gooo", "ports/google/uuid/text/parse_canonical.gooo"}) {
				t.Fatal("parser workspace lacks exact quote/validation-helper/Text closure", selected.Names)
			}
		} else if _, exists := selected.Sources["ports/google/uuid/text/parse_canonical.gooo"]; exists {
			t.Fatal("new parser source leaked into a prior workspace")
		}
		if ports != wantPorts || consumers != wantConsumers {
			t.Fatal("selected package roster differs", ports, consumers)
		}
		if err := inspectWorkspace(image, s, contract, selected.Identities); err != nil {
			t.Fatal("selected workspace no longer binds exact frozen sources", err)
		}
		if err := inspectWorkspace(image, s, contract, allIdentities); err == nil {
			t.Fatal("accepted the full catalog as the smaller execution roster")
		}
		image.Packages[0].Sources[0].Digest = contentDigest([]byte("altered"))
		if err := inspectWorkspace(image, s, contract, selected.Identities); err == nil {
			t.Fatal("accepted altered selected-source digest")
		}
	}
}

func TestLiteralTextWorkspacesUseOnlyFrozenSearchLiteralAndOptionalReport(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	const search = "ports/strings/search.gooo"
	const literal = "ports/strings/literal.gooo"
	const report = "examples/literal-text-report/main.gooo"
	// These raw hashes come from the independently frozen native pilot, not
	// from the catalog selection under test.
	wantHashes := map[string]string{
		search:  "sha256:39e0371cb71ee050dfc6d96c92bca2e5b61efc22927f70f3c15506e1599baf01",
		literal: "sha256:4aebcc74d3c2ad965be5881095cf3afc80bd7e0c85fbe0b5848900bd25e425b5",
		report:  "sha256:96d4c80d00b2627a2787c3fa7a9af68de91e8239cd456f10c6ff69dbeb87f77d",
	}
	for _, s := range []suite{{Package: "go/strings", Activity: "Index"}, {Package: "go/strings", Activity: "Count"}, {Package: "app/literal-text-report", Activity: "Report"}} {
		t.Run(s.Activity, func(t *testing.T) {
			selected, err := frozen.workspaceForSuite(s)
			if err != nil {
				t.Fatal(err)
			}
			wantPackages := []sourcePackage{{Path: "go/strings", Sources: []string{search, literal}}}
			wantNames := []string{search, literal}
			if s.Activity == "Report" {
				wantPackages = append(wantPackages, sourcePackage{Path: s.Package, Sources: []string{report}})
				wantNames = append(wantNames, report)
			}
			if !reflect.DeepEqual(selected.Manifest.Packages, wantPackages) || !reflect.DeepEqual(selected.Names, wantNames) || len(selected.Sources) != len(wantNames) || len(selected.Identities) != len(wantNames) {
				t.Fatal("literal workspace source closure differs", selected.Manifest.Packages, selected.Names)
			}
			for _, name := range wantNames {
				if contentDigest(selected.Sources[name]) != wantHashes[name] {
					t.Fatal("literal workspace differs from the frozen pilot", name)
				}
			}
		})
	}
	for _, s := range []suite{{Package: "go/strings", Activity: "IndexByte"}, {Package: "go/strings", Activity: "Report"}, {Package: "app/literal-text-report", Activity: "Count"}, {Package: "other", Activity: "Index"}} {
		if isLiteralTextSuite(s) {
			t.Fatal("literal selector matched a different identity", s)
		}
	}
}

func TestLiteralTextWorkspaceRejectsMissingExtraReorderedAndAlteredSources(t *testing.T) {
	const search = "ports/strings/search.gooo"
	const literal = "ports/strings/literal.gooo"
	const report = "examples/literal-text-report/main.gooo"
	for name, mutate := range map[string]func(*frozenVerificationCatalog){
		"absent literal":      func(f *frozenVerificationCatalog) { delete(f.Sources, literal) },
		"absent declaration":  func(f *frozenVerificationCatalog) { delete(f.Sources, search) },
		"absent Report":       func(f *frozenVerificationCatalog) { delete(f.Sources, report) },
		"altered literal":     func(f *frozenVerificationCatalog) { f.Sources[literal][0] ^= 1 },
		"altered declaration": func(f *frozenVerificationCatalog) { f.Sources[search][0] ^= 1 },
		"altered Report":      func(f *frozenVerificationCatalog) { f.Sources[report][0] ^= 1 },
		"extra port source": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == "go/strings" {
					f.Catalog.Packages[i].Sources = append(f.Catalog.Packages[i].Sources, "ports/time/duration.gooo")
				}
			}
		},
		"extra consumer source": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == "app/literal-text-report" {
					f.Catalog.Packages[i].Sources = append(f.Catalog.Packages[i].Sources, "examples/response.gooo")
				}
			}
		},
		"reordered port sources": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == "go/strings" {
					f.Catalog.Packages[i].Sources = []string{literal, search}
				}
			}
		},
		"unrelated consumer dependency": func(f *frozenVerificationCatalog) {
			f.Sources[report] = append(f.Sources[report], []byte("\nimport other \"app/response\"\n")...)
			f.Hashes[report] = strings.TrimPrefix(contentDigest(f.Sources[report]), "sha256:")
		},
	} {
		t.Run(name, func(t *testing.T) {
			frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
			if err != nil {
				t.Fatal(err)
			}
			mutate(&frozen)
			if _, err := frozen.workspaceForSuite(suite{Package: "app/literal-text-report", Activity: "Report"}); err == nil {
				t.Fatal("accepted incomplete, altered, or unrelated literal closure")
			}
		})
	}
}

func TestLiteralTextSourcesAreExcludedBeforeLegacyHashSelection(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy := []suite{
		{Package: "go/net/http", Activity: "StatusText"},
		{Package: "go/strings", Activity: "IndexByte"},
		{Package: uuidPackage, Activity: "Validate"},
		{Package: uuidParsePackage, Activity: "ParseCanonical"},
	}
	before := make([]suiteWorkspace, len(legacy))
	for i, s := range legacy {
		before[i], err = frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
	}
	// The old suites must not copy or hash either unrelated new source.
	// Initial catalog freezing still checks every file before selection.
	frozen.Sources["ports/strings/literal.gooo"][0] ^= 1
	delete(frozen.Sources, "examples/literal-text-report/main.gooo")
	for i, s := range legacy {
		after, err := frozen.workspaceForSuite(s)
		if err != nil || !reflect.DeepEqual(before[i], after) {
			t.Fatal("new literal source affected a prior workspace", s, err)
		}
	}
}

func writeCatalogFixture(t *testing.T, dir string, catalog verificationCatalog, sources map[string]string) string {
	t.Helper()
	for name, source := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "verification-catalog.json")
	if err := os.WriteFile(path, mustJSON(t, catalog), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVerificationCatalogFreezesFilesBeforeSelection(t *testing.T) {
	const port = "ports/image/ycbcr.gooo"
	const consumer = "examples/ycbcr-inspector/main.gooo"
	dir := t.TempDir()
	catalog := verificationCatalog{Schema: verificationCatalogSchema, Packages: []sourcePackage{
		{Path: "go/image/color", Sources: []string{port}},
		{Path: "app/ycbcr-inspector", Sources: []string{consumer}},
	}}
	path := writeCatalogFixture(t, dir, catalog, map[string]string{port: "package color\n", consumer: "package consumer\nimport color \"go/image/color\"\n"})
	frozen, err := freezeVerificationCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	s := suite{Package: "app/ycbcr-inspector", Activity: "InspectRGB"}
	before, err := frozen.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, port), []byte("changed on disk"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, consumer)); err != nil {
		t.Fatal(err)
	}
	after, err := frozen.workspaceForSuite(s)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("suite reread changed/deleted source files", err)
	}
	before.Sources[port][0] = 'X'
	before.Manifest.Packages[0].Sources[0] = "other"
	again, err := frozen.workspaceForSuite(s)
	if err != nil || !reflect.DeepEqual(again, after) {
		t.Fatal("one suite mutated another suite or the frozen catalog", err)
	}
	frozen.Sources[port][0] = 'X'
	if _, err := frozen.workspaceForSuite(s); err == nil {
		t.Fatal("accepted mutated frozen bytes with the original hash")
	}
}

func TestVerificationCatalogRejectsMissingEntryAndDependency(t *testing.T) {
	const consumer = "examples/ycbcr-inspector/main.gooo"
	catalog := verificationCatalog{Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "app/ycbcr-inspector", Sources: []string{consumer}}}}
	path := writeCatalogFixture(t, t.TempDir(), catalog, map[string]string{consumer: "package consumer\nimport color \"go/image/color\"\n"})
	frozen, err := freezeVerificationCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frozen.workspaceForSuite(suite{Package: "app/ycbcr-inspector", Activity: "InspectRGB"}); err == nil || !strings.Contains(err.Error(), "imports absent package go/image/color") {
		t.Fatal("missing consumer dependency was not rejected before execution", err)
	}
	if _, err := frozen.workspaceForSuite(suite{Package: "app/color-inspector", Activity: "InspectRGB"}); err == nil {
		t.Fatal("accepted missing entry package")
	}
	frozen.Catalog.Packages = append(frozen.Catalog.Packages, sourcePackage{Path: "app/color-inspector", Sources: []string{"examples/color-inspector/main.gooo"}})
	frozen.Sources[consumer] = []byte("import other \"app/color-inspector\"\n")
	frozen.Hashes[consumer] = strings.TrimPrefix(contentDigest(frozen.Sources[consumer]), "sha256:")
	if _, err := frozen.workspaceForSuite(suite{Package: "app/ycbcr-inspector", Activity: "InspectRGB"}); err == nil || !strings.Contains(err.Error(), "imports absent package app/color-inspector") {
		t.Fatal("silently included an unrelated consumer dependency", err)
	}
}

func TestVerificationCatalogRejectsBadSchemaAndRoster(t *testing.T) {
	for name, catalog := range map[string]verificationCatalog{
		"compiler manifest schema": {Schema: "gooo/package-workspace-manifest/v1", Packages: []sourcePackage{{Path: "go/test", Sources: []string{"source.gooo"}}}},
		"empty":                    {Schema: verificationCatalogSchema},
		"duplicate package":        {Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "go/test", Sources: []string{"source.gooo"}}, {Path: "go/test", Sources: []string{"other.gooo"}}}},
		"duplicate source":         {Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "go/test", Sources: []string{"source.gooo", "source.gooo"}}}},
		"missing source":           {Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "go/test", Sources: []string{"missing.gooo"}}}},
		"traversal":                {Schema: verificationCatalogSchema, Packages: []sourcePackage{{Path: "go/test", Sources: []string{"../source.gooo"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCatalogFixture(t, t.TempDir(), catalog, map[string]string{"source.gooo": "package example", "other.gooo": "package example"})
			if _, err := freezeVerificationCatalog(path); err == nil {
				t.Fatal("accepted invalid catalog")
			}
		})
	}
}
