package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExternalOracleRosterAndUUIDMetadata(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.ExternalSources) != 3 {
		t.Fatal("expected one fixed and two UUID adaptation rows")
	}
	if err := verifyExternalOracles("../..", lock.ExternalSources); err != nil {
		t.Fatal(err)
	}
	fixed := externalSourcesForModule(lock.ExternalSources, "golang.org/x/image")
	uuidSources := externalSourcesForModule(lock.ExternalSources, "github.com/google/uuid")
	if len(fixed) != 1 || len(uuidSources) != 2 {
		t.Fatal("module/target roster differs")
	}
	for _, sources := range [][]externalSource{
		nil, fixed, uuidSources,
		{fixed[0], uuidSources[0]}, {fixed[0], uuidSources[1]},
		{fixed[0], fixed[0], uuidSources[0]},
		{uuidSources[0], uuidSources[1], uuidSources[0]},
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
