package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Synthetic parser fixture: no compiler execution or external decision is claimed.
func failureProbeFixture(t *testing.T) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	manifest, cases := []byte("manifest bytes\n"), []byte("case bytes\n")
	raw := marshal(map[string]any{
		"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "FAIL_CLOSED",
		"error": `Context requires field "now"`, "manifest_digest": digest(manifest), "cases_digest": digest(cases),
	})
	for name, data := range map[string][]byte{
		"gooo.workspace.json": manifest, "missing-context-cases.json": cases,
		"missing-context.exit.txt": []byte("1"), "missing-context.json": raw,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, raw
}

func TestFailureProbeKeepsBoundReceiptBytes(t *testing.T) {
	dir, raw := failureProbeFixture(t)
	// Unique unknown fields retain their existing accepted policy, including exact integers.
	raw = append([]byte(`{"extra":[{"n":9223372036854775807},{"n":9007199254740993}],`), raw[1:]...)
	path := filepath.Join(dir, "missing-context.json")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFailureProbe(dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("verification changed receipt bytes", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "missing-context.exit.txt"), []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFailureProbe(dir); err == nil {
		t.Fatal("accepted a successful process as a failure observation")
	}
}

func TestFailureProbeRejectsDuplicateReceiptKeys(t *testing.T) {
	for name, members := range map[string]string{
		"conflicting decision": `"decision":"PASS"`,
		"same decision":        `"decision":"FAIL_CLOSED"`,
		"escaped decision":     `"\u0064ecision":"PASS"`,
		"nested unknown":       `"extra":{"value":false,"value":true}`,
		"array object":         `"extra":[{"value":0,"\u0076alue":1}]`,
	} {
		t.Run(name, func(t *testing.T) {
			dir, raw := failureProbeFixture(t)
			mutated := append([]byte("{"+members+","), raw[1:]...)
			path := filepath.Join(dir, "missing-context.json")
			if err := os.WriteFile(path, mutated, 0644); err != nil {
				t.Fatal(err)
			}
			if err := verifyFailureProbe(dir); err == nil {
				t.Fatal("accepted duplicate receipt key")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(mutated, after) {
				t.Fatal("rejection changed receipt bytes", err)
			}
		})
	}
}
