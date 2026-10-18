package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureArgs(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "..", "internal", "tutorialchange", "testdata")
	return []string{"--before", filepath.Join(dir, "before.json"), "--after", filepath.Join(dir, "after.json"), "--replay", filepath.Join(dir, "replay.json"), "--compiler-build", filepath.Join(dir, "compiler-build.json")}
}

func TestCLIEmitsOnlySuccessfulSummary(t *testing.T) {
	var out, err bytes.Buffer
	if code := run(fixtureArgs(t), &out, &err); code != 0 || err.Len() != 0 || !strings.Contains(out.String(), `"status": "CHECKED_FINITE_OBSERVATIONS"`) {
		t.Fatalf("code/output: %d %s %s", code, &out, &err)
	}
}

func TestCLIRefusalLeavesStdoutEmpty(t *testing.T) {
	args := fixtureArgs(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"decision":"PASS"}`), 0600); err != nil {
		t.Fatal(err)
	}
	args[1] = bad
	var out, err bytes.Buffer
	if code := run(args, &out, &err); code == 0 || out.Len() != 0 || err.Len() == 0 {
		t.Fatalf("code/output: %d %s %s", code, &out, &err)
	}
}

func TestCLIMissingFlagAndNonregularInput(t *testing.T) {
	var out, err bytes.Buffer
	if code := run(nil, &out, &err); code != 2 || out.Len() != 0 {
		t.Fatalf("missing flags: %d", code)
	}
	args := fixtureArgs(t)
	args[1] = t.TempDir()
	out.Reset()
	err.Reset()
	if code := run(args, &out, &err); code == 0 || out.Len() != 0 || !strings.Contains(err.String(), "regular file") {
		t.Fatalf("directory: %d %s %s", code, &out, &err)
	}
}
