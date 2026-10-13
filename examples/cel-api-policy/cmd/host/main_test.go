package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	nativepolicy "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func TestHistoricalAllowedAndDeniedNeverCallCurrentProviderOrEffect(t *testing.T) {
	control := &application{current: syntheticFacts(true), state: 42}
	control.currentFacts()
	control.dispatch()
	if control.providerCalls != 1 || control.effectCalls != 1 || control.state != 43 {
		t.Fatal("host spies inactive")
	}
	for _, allowed := range []bool{true, false} {
		proof, err := nativepolicy.EvaluateAPI(syntheticFacts(allowed))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(proof)
		// Current facts deliberately contradict the saved allow/deny observation.
		app := &application{current: syntheticFacts(!allowed), state: 42}
		report, err := verifyHistory(raw, app)
		if err != nil {
			t.Fatal(err)
		}
		if report.Historical.Decision.Allowed != allowed || report.ProviderCalls != 0 || report.EffectCalls != 0 || report.StateBefore != 42 || report.StateAfter != 42 || app.providerCalls != 0 || app.effectCalls != 0 || app.state != 42 {
			t.Fatalf("replay accessed current provider or dispatched: %+v", report)
		}
	}
}

func TestHostPrivateSaveReloadAndExistingFileRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, decision := range []string{"allow", "deny"} {
		path := filepath.Join(dir, decision+".json")
		var saved, verified bytes.Buffer
		if err := run("save", path, decision, &saved); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("proof was not created privately")
		}
		if err := run("verify", path, "allow", &verified); err != nil {
			t.Fatal(err)
		}
		var saveReport, verifyReport hostReport
		if json.Unmarshal(saved.Bytes(), &saveReport) != nil || json.Unmarshal(verified.Bytes(), &verifyReport) != nil {
			t.Fatal("host output")
		}
		if !reflectBindings(saveReport.Bindings, verifyReport.Bindings) || verifyReport.Historical.Decision.Allowed != (decision == "allow") {
			t.Fatal("reload changed source generation or observation")
		}
		before, _ := os.ReadFile(path)
		if err := run("save", path, decision, &saved); err == nil {
			t.Fatal("existing caller file overwritten")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("existing proof changed")
		}
	}
	public := filepath.Join(dir, "existing-public.json")
	if err := os.WriteFile(public, []byte("private data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveNewProof(public, []byte("replacement")); err == nil {
		t.Fatal("existing permissive file overwritten")
	}
	if _, err := readPrivateProof(public); err == nil {
		t.Fatal("permissive saved file replayed")
	}
	data, _ := os.ReadFile(public)
	if string(data) != "private data" {
		t.Fatal("existing permissive file modified")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join(dir, "allow.json"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivateProof(link); err == nil {
		t.Fatal("symlink proof accepted")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := saveNewProof(filepath.Join(dir, "public-parent.json"), []byte("data")); err == nil {
		t.Fatal("public parent accepted")
	}
}

func reflectBindings(a, b nativepolicy.Bindings) bool {
	if a.CompilerSourceSHA != b.CompilerSourceSHA || a.GenerationSHA256 != b.GenerationSHA256 || len(a.Assets) != len(b.Assets) {
		return false
	}
	for name, value := range a.Assets {
		if b.Assets[name] != value {
			return false
		}
	}
	return true
}
