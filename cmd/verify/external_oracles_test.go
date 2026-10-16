package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"testing"
)

func TestExternalOracleRosterAndUUIDMetadata(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.ExternalSources) != 4 {
		t.Fatal("expected one fixed, two UUID and one httpguts adaptation rows")
	}
	if err := verifyExternalOracles("../..", lock.ExternalSources); err != nil {
		t.Fatal(err)
	}
	fixed := externalSourcesForModule(lock.ExternalSources, "golang.org/x/image")
	uuidSources := externalSourcesForModule(lock.ExternalSources, "github.com/google/uuid")
	netSources := externalSourcesForModule(lock.ExternalSources, "golang.org/x/net")
	if len(fixed) != 1 || len(uuidSources) != 2 || len(netSources) != 1 {
		t.Fatal("module/target roster differs")
	}
	for _, sources := range [][]externalSource{
		nil, fixed, uuidSources,
		{fixed[0], uuidSources[0]}, {fixed[0], uuidSources[1]},
		{fixed[0], fixed[0], uuidSources[0]},
		{uuidSources[0], uuidSources[1], uuidSources[0]},
		{fixed[0], uuidSources[0], uuidSources[0], netSources[0]},
		{fixed[0], fixed[0], uuidSources[0], netSources[0]},
		append(append([]externalSource{}, lock.ExternalSources...), uuidSources[0]),
	} {
		if err := verifyExternalOracles("../..", sources); err == nil {
			t.Fatal("accepted missing/duplicated module or target")
		}
	}
	unknown := append([]externalSource{}, lock.ExternalSources...)
	unknown[1].Module = "unreviewed"
	if err := verifyExternalOracles("../..", unknown); err == nil {
		t.Fatal("accepted unknown module")
	}
	for _, sources := range [][]externalSource{nil, uuidSources[:1], {uuidSources[0], uuidSources[0]}, {uuidSources[1], uuidSources[1]}} {
		if err := verifyUUIDOracle("../..", sources); err == nil {
			t.Fatal("accepted missing or duplicated UUID target")
		}
	}
	for index, original := range uuidSources {
		for _, field := range []string{"Module", "Repository", "Revision", "Version", "ModuleH1", "Source", "SourceSHA", "OracleCopy", "License", "LicenseSHA", "Target"} {
			t.Run(original.Target+"/"+field, func(t *testing.T) {
				mutated := append([]externalSource{}, uuidSources...)
				reflect.ValueOf(&mutated[index]).Elem().FieldByName(field).SetString("wrong")
				if err := verifyUUIDOracle("../..", mutated); err == nil {
					t.Fatal("accepted changed UUID metadata", field)
				}
			})
		}
		for _, change := range []func(*externalSource){
			func(s *externalSource) { s.Symbols = []string{"Parse"} },
			func(s *externalSource) { s.SupportingSources = nil },
			func(s *externalSource) { s.SupportingSources = []string{"util.go", "uuid.go"} },
			func(s *externalSource) { s.SupportingSHA = map[string]string{"util.go": "wrong"} },
			func(s *externalSource) {
				s.SupportingSHA = map[string]string{"util.go": uuidUtilSHA, "other": uuidSourceSHA}
			},
			func(s *externalSource) { s.SupportingTargets = nil },
			func(s *externalSource) {
				s.SupportingTargets = []string{"ports/google/uuid/quote.gooo", "ports/google/uuid/text/parse_canonical.gooo"}
			},
		} {
			mutated := append([]externalSource{}, uuidSources...)
			change(&mutated[index])
			if err := verifyUUIDOracle("../..", mutated); err == nil {
				t.Fatal("accepted changed UUID support roster", original.Target)
			}
		}
	}
	// Row order is irrelevant; the two exact target contracts remain required.
	if err := verifyUUIDOracle("../..", []externalSource{uuidSources[1], uuidSources[0]}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPGutsExactMetadataAndSourceReferences(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	sources := externalSourcesForModule(lock.ExternalSources, "golang.org/x/net")
	if err := verifyHTTPGutsOracle("../..", sources); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]externalSource{nil, {sources[0], sources[0]}} {
		if err := verifyHTTPGutsOracle("../..", bad); err == nil {
			t.Fatal("accepted missing/duplicate net target")
		}
	}
	for _, field := range []string{"Module", "Repository", "Revision", "Version", "ModuleH1", "Source", "SourceSHA", "OracleCopy", "License", "LicenseSHA", "Target"} {
		mutated := append([]externalSource(nil), sources...)
		reflect.ValueOf(&mutated[0]).Elem().FieldByName(field).SetString("wrong")
		if err := verifyHTTPGutsOracle("../..", mutated); err == nil {
			t.Fatal("accepted changed httpguts metadata", field)
		}
	}
	for _, change := range []func(*externalSource){
		func(s *externalSource) { s.Symbols = []string{"ValidHeaderFieldName"} },
		func(s *externalSource) { s.Symbols = []string{"ValidHeaderFieldValue", "ValidHeaderFieldName"} },
		func(s *externalSource) { s.SupportingSources = nil },
		func(s *externalSource) {
			s.SupportingSHA = map[string]string{"http/httpguts/guts.go": "wrong", "http/httpguts/httplex_test.go": httpgutsTestSHA}
		},
		func(s *externalSource) { s.SupportingTargets = []string{"unreviewed.gooo"} },
	} {
		mutated := append([]externalSource(nil), sources...)
		change(&mutated[0])
		if err := verifyHTTPGutsOracle("../..", mutated); err == nil {
			t.Fatal("accepted changed httpguts support/symbol roster")
		}
	}
	files := []string{"http/httpguts/guts.go.txt", "http/httpguts/httplex.go.txt", "http/httpguts/httplex_test.go.txt", "LICENSE"}
	for _, altered := range files {
		t.Run(altered, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range files {
				raw, err := os.ReadFile(filepath.Join("../../third_party/x-net", name))
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "third_party/x-net", name)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, raw, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := verifyHTTPGutsOracle(root, sources); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "third_party/x-net", altered)
			raw, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, append(raw, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			if err := verifyHTTPGutsOracle(root, sources); err == nil {
				t.Fatal("accepted changed upstream witness")
			}
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := verifyHTTPGutsOracle(root, sources); err == nil {
				t.Fatal("accepted missing upstream witness")
			}
		})
	}
}

func TestHTTPGutsBuildDependencyGate(t *testing.T) {
	fixture := func() *debug.BuildInfo {
		return &debug.BuildInfo{Deps: []*debug.Module{
			{Path: "golang.org/x/net", Version: httpgutsNetVersion, Sum: httpgutsNetModuleH1},
			{Path: "golang.org/x/text", Version: httpgutsTextVersion, Sum: httpgutsTextModuleH1},
			{Path: "github.com/google/uuid", Version: "v1.6.0", Replace: &debug.Module{Path: "./third_party/google-uuid"}},
		}}
	}
	if err := verifyHTTPGutsBuildDependencies(fixture()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*debug.BuildInfo{nil, {}, {Deps: fixture().Deps[:1]}} {
		if err := verifyHTTPGutsBuildDependencies(bad); err == nil {
			t.Fatal("accepted missing linked module")
		}
	}
	for index := 0; index < 2; index++ {
		for _, field := range []string{"Path", "Version", "Sum"} {
			bad := fixture()
			reflect.ValueOf(bad.Deps[index]).Elem().FieldByName(field).SetString("wrong")
			if err := verifyHTTPGutsBuildDependencies(bad); err == nil {
				t.Fatal("accepted changed linked module", index, field)
			}
		}
		bad := fixture()
		bad.Deps[index].Replace = &debug.Module{Path: bad.Deps[index].Path, Version: bad.Deps[index].Version, Sum: bad.Deps[index].Sum}
		if err := verifyHTTPGutsBuildDependencies(bad); err == nil {
			t.Fatal("accepted replaced linked module")
		}
		bad = fixture()
		bad.Deps = append(bad.Deps, bad.Deps[index])
		if err := verifyHTTPGutsBuildDependencies(bad); err == nil {
			t.Fatal("accepted duplicate linked module")
		}
	}
}

func TestUUIDOriginalModuleHashRejectsChangedAndMissingFiles(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	sources := externalSourcesForModule(lock.ExternalSources, "github.com/google/uuid")
	for _, altered := range []string{"uuid.go", "util.go", "LICENSE", "go.mod", "version4.go", "uuid_test.go"} {
		t.Run(altered, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range uuidOriginalFiles {
				raw, err := os.ReadFile(filepath.Join("../../third_party/google-uuid", name))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "third_party/google-uuid", name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := verifyUUIDOracle(root, sources); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "third_party/google-uuid", altered)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := f.WriteString("\n")
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatal(writeErr, closeErr)
			}
			if err := verifyUUIDOracle(root, sources); err == nil {
				t.Fatal("accepted modified original archive file")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := verifyUUIDOracle(root, sources); err == nil {
				t.Fatal("accepted missing original archive file")
			}
		})
	}
}
