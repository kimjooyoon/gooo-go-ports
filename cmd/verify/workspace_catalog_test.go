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
	if len(frozen.Catalog.Packages) != 25 || len(frozen.Sources) != 32 {
		t.Fatal("complete reviewed catalog roster changed")
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
		if ports != 10 || consumers != wantConsumers {
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
