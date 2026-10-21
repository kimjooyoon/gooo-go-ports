package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const etagBudgetChild = "GOOO_ETAG_BUDGET_TEST_CHILD"

// Controlled children exercise the real runner and pipes without a compiler,
// policy execution, network access or a shell command.
func TestMain(m *testing.M) {
	switch os.Getenv(etagBudgetChild) {
	case "stdout":
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), (1<<20)+1))
		os.Exit(0)
	case "stderr":
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("x"), (256<<10)+1))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestETagReceiptBudgetOnlyForExplicitCompilerExecuteJSON(t *testing.T) {
	compiler := "/fixed/gooo"
	for _, c := range []struct {
		name, binary string
		args         []string
		want         int
	}{
		{"baseline receipt", compiler, []string{"package", "execute", "--json", "--cases", "cases.json"}, 8 << 20},
		{"mutation receipt", compiler, []string{"package", "execute", "--json", "--cases", "same-cases.json"}, 8 << 20},
		{"identity", compiler, []string{"version", "--build", "--json"}, 1 << 20},
		{"different executable", "/fixed/other", []string{"package", "execute", "--json"}, 1 << 20},
		{"non-JSON", compiler, []string{"package", "execute"}, 1 << 20},
		{"different operation", compiler, []string{"package", "replay", "--json"}, 1 << 20},
		{"ordinary Go command", "/fixed/go", []string{"env", "GOROOT"}, 1 << 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := commandStdoutLimit(c.binary, compiler, c.args); got != c.want {
				t.Fatalf("budget %d, want %d", got, c.want)
			}
		})
	}
	if maxCommandStderr != 256<<10 {
		t.Fatal("stderr budget changed")
	}
}

func TestETagStreamExactCapThenOneByte(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "stream-")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &stream{file: file, limit: 4, cancel: cancel}
	for _, p := range [][]byte{[]byte("ab"), []byte("cd")} {
		if n, err := s.Write(p); n != 2 || err != nil {
			t.Fatal(n, err)
		}
	}
	if ctx.Err() != nil || s.capped {
		t.Fatal("exact cap was cancelled")
	}
	if n, err := s.Write([]byte("e")); n != 0 || err == nil || ctx.Err() == nil || !s.capped {
		t.Fatal("one byte over did not stop", n, err)
	}
	raw, err := os.ReadFile(file.Name())
	if err != nil || string(raw) != "abcd" || s.data.String() != "abcd" {
		t.Fatal("original bounded prefix lost", string(raw), err)
	}
}

func TestETagStreamOversizedChunkRetainsOnlyBoundedPrefix(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "stream-")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &stream{file: file, limit: 4, cancel: cancel}
	n, err := s.Write([]byte("abcde"))
	if n != 4 || err == nil || ctx.Err() == nil || !s.capped {
		t.Fatal("oversized chunk not stopped", n, err)
	}
	raw, err := os.ReadFile(file.Name())
	if err != nil || string(raw) != "abcd" || s.data.String() != "abcd" {
		t.Fatal("oversized original prefix changed", string(raw), err)
	}
}

func TestETagRunnerScopesStreamsAndClosesControlledChildren(t *testing.T) {
	for _, c := range []struct {
		name, mode                           string
		args                                 []string
		accepted, stdoutCapped, stderrCapped bool
	}{
		{"receipt", "stdout", []string{"package", "execute", "--json"}, true, false, false},
		{"ordinary", "stdout", []string{"version", "--build", "--json"}, false, true, false},
		{"diagnostics", "stderr", []string{"package", "execute", "--json"}, false, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(etagBudgetChild, c.mode)
			dir := t.TempDir()
			r := &runner{ctx: context.Background(), out: dir, root: dir, compiler: os.Args[0]}
			var got process
			var problem any
			func() { defer func() { problem = recover() }(); got = r.run(c.name, os.Args[0], dir, c.args...) }()
			if c.accepted {
				if problem != nil || got.Exit != 0 || got.Error != "" || len(got.Raw) != (1<<20)+1 {
					t.Fatal("receipt output not retained", problem, len(got.Raw))
				}
			}
			if !c.accepted && (problem == nil || !strings.Contains(fmt.Sprint(problem), "bounded command/process failure")) {
				t.Fatal("oversized stream was accepted", problem)
			}
			raw, err := os.ReadFile(filepath.Join(dir, c.name, "process.json"))
			if err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if err = json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			if record["own_group_gone"] != true || record["group_cleanup_error"] != "" || record["stdout_capped"] != c.stdoutCapped || record["stderr_capped"] != c.stderrCapped {
				t.Fatal("child group/stream closure", record)
			}
			if record["stderr_cap"] != float64(256<<10) {
				t.Fatal("diagnostic budget widened")
			}
			if c.name == "receipt" && record["stdout_cap"] != float64(8<<20) {
				t.Fatal("receipt budget not used by runner")
			}
			if c.name == "ordinary" && record["stdout_cap"] != float64(1<<20) {
				t.Fatal("ordinary stdout budget widened")
			}
		})
	}
}
