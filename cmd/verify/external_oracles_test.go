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
	if err := verifyExternalOracles("../..", lock.ExternalSources); err != nil {
		t.Fatal(err)
	}
	for _, sources := range [][]externalSource{nil, lock.ExternalSources[:1], append(append([]externalSource{}, lock.ExternalSources...), lock.ExternalSources[1])} {
		if err := verifyExternalOracles("../..", sources); err == nil {
			t.Fatal("accepted missing or duplicated module")
		}
	}
	unknown := append([]externalSource{}, lock.ExternalSources...)
	unknown[1].Module = "unreviewed"
	if err := verifyExternalOracles("../..", unknown); err == nil {
		t.Fatal("accepted unknown module")
	}
	original := externalSourcesForModule(lock.ExternalSources, "github.com/google/uuid")[0]
	for _, field := range []string{"Module", "Repository", "Revision", "Version", "ModuleH1", "Source", "SourceSHA", "OracleCopy", "License", "LicenseSHA", "Target"} {
		mutated := original
		reflect.ValueOf(&mutated).Elem().FieldByName(field).SetString("wrong")
		if err := verifyUUIDOracle("../..", []externalSource{mutated}); err == nil {
			t.Fatal("accepted changed UUID metadata", field)
		}
	}
	for _, change := range []func(*externalSource){
		func(s *externalSource) { s.Symbols = []string{"Parse"} },
		func(s *externalSource) { s.SupportingSources = nil },
		func(s *externalSource) { s.SupportingSHA = map[string]string{"util.go": "wrong"} },
		func(s *externalSource) { s.SupportingTargets = nil },
	} {
		mutated := original
		change(&mutated)
		if err := verifyUUIDOracle("../..", []externalSource{mutated}); err == nil {
			t.Fatal("accepted changed UUID support roster")
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
