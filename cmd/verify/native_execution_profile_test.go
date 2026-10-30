package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These refusals must happen before loading a repository or invoking any
// producer, so run in a directory with neither upstream.json nor compiler.
func TestNativeSessionCLIRejectsMixedModesBeforeIO(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	target := filepath.Join(root, "uncreated-parent", "evidence")
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	base := []string{"--execution-profile", string(nativeExecutionSession276), "--compiler", filepath.Join(root, "absent-compiler"), "--out", target}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown", []string{"--execution-profile", "unknown", "--out", target}, "unknown --execution-profile"},
		{"relative_compiler", []string{"--execution-profile", string(nativeExecutionSession276), "--out", target}, "explicit absolute --compiler"},
		{"missing_out", []string{"--execution-profile", string(nativeExecutionSession276), "--compiler", "/absent-compiler"}, "session requires --out"},
		{"browser", append(append([]string{}, base...), "--browser-fixtures", "browser.json"), "session requires --out"},
		{"timing", append(append([]string{}, base...), "--timing-out", filepath.Join(target, "timing.jsonl")), "session requires --out"},
		{"compact", append(append([]string{}, base...), "--case-profile", "uuid-compact-v1"), "session requires --out"},
		{"dependency_closure", append(append([]string{}, base...), "--workspace-profile", "dependency-closure-v1"), "session requires --out"},
		{"shard", append(append([]string{}, base...), "--ci-shard-plan-out", "plan.json"), "cannot be used with CI shard"},
		{"shard_explicit_stateless", []string{"--execution-profile", "stateless", "--ci-shard-plan-out", "plan.json"}, "cannot"},
		{"gzip_shard", []string{"--execution-profile", string(nativeExecutionSession276Gzip), "--ci-shard-plan-out", "plan.json"}, "cannot be used with CI shard"},
		{"gzip_compact", []string{"--execution-profile", string(nativeExecutionSession276Gzip), "--compiler", filepath.Join(root, "absent-compiler"), "--out", target, "--case-profile", "uuid-compact-v1"}, "session requires --out"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("session-early-error", flag.ContinueOnError)
			flag.CommandLine.SetOutput(&bytes.Buffer{})
			os.Args = append([]string{"verify"}, test.args...)
			err := run()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected early refusal %q, got %v", test.want, err)
			}
			if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
				t.Fatal("refused mode created an evidence parent", err)
			}
			for _, name := range []string{"browser.json", "plan.json"} {
				if _, err := os.Stat(name); !os.IsNotExist(err) {
					t.Fatal("refused mode created output", name, err)
				}
			}
		})
	}
}
