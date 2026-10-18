package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func TestCIShardNativeComparisonPreservesReportsAcrossPartialIndentation(t *testing.T) {
	plan := ciShardTestPlan(t, 3, "go/unicode:SimpleFold", "go/image/color:RGBToYCbCr", "go/image/color:YCbCrToRGB", "go/unicode:To", "go/unicode:ToUpper", "go/unicode:ToLower", "go/unicode:ToTitle")
	for _, original := range ciShardTestPartials(t, plan) {
		raw, err := json.MarshalIndent(original, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		var partial ciShardPartial
		if err := decodeCIShardJSON(raw, &partial); err != nil {
			t.Fatal(err)
		}
		for i, record := range original.Suites {
			if !equalCIShardSuiteRecords(record, partial.Suites[i]) {
				t.Fatal("valid typed reports changed meaning after JSON indentation")
			}
			mutated := ciShardTestClone(t, partial.Suites[i])
			mutated.Batches[0].End++
			if equalCIShardSuiteRecords(record, mutated) {
				t.Fatal("changed original boundary accepted")
			}
			mutated = ciShardTestClone(t, partial.Suites[i])
			mutated.Exhaustive[0].Report = json.RawMessage(`{"mismatches":1}`)
			if equalCIShardSuiteRecords(record, mutated) {
				t.Fatal("changed exhaustive value accepted")
			}
		}
	}
}

func TestCIShardNativeFlagsRejectBeforeSourceOrOutput(t *testing.T) {
	oldFlags, oldArgs := flag.CommandLine, os.Args
	defer func() { flag.CommandLine, os.Args = oldFlags, oldArgs }()
	oldDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	if err := os.Chdir(empty); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDirectory)
	for _, args := range [][]string{
		{"--ci-shard-execute-out="}, {"--ci-shard-verify-out="},
		{"--ci-shard-reference-out="}, {"--ci-shard-reference-out=index.json"},
		{"--ci-shard-reference-out=index.json", "--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0"},
		{"--ci-shard-reference-out=same", "--ci-shard-verify-out=same", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-partial=unused", "--ci-shard-evidence=unused"},
		{"--ci-shard-native-compiler=./.gooo"}, {"--ci-shard-native-compiler="},
		{"--ci-shard-evidence="}, {"--ci-shard-evidence=unused"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-verify-out=other", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0", "--ci-shard-partial=unused"},
		{"--ci-shard-verify-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-partial=unused"},
		{"--ci-shard-verify-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-partial=unused", "--ci-shard-evidence=unused", "--ci-shard-index=0"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0", "--out=old-full"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0", "--workers=3"},
		{"--ci-shard-execute-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0", "--ci-shard-count=3"},
		{"--ci-shard-verify-out=fresh", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-partial=unused", "--ci-shard-evidence=unused", "--ci-shard-native-compiler=./.gooo"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("verify", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = append([]string{"verify"}, args...)
			err := run()
			if err == nil || strings.Contains(err.Error(), "open ") || strings.Contains(err.Error(), "upstream.json") {
				t.Fatal("invalid native flags reached filesystem or full native work", err)
			}
			entries, err := os.ReadDir(empty)
			if err != nil || len(entries) != 0 {
				t.Fatal("invalid flags created output", err)
			}
		})
	}
}

func TestCIShardNativeOptionsSeparateExecutionFromVerification(t *testing.T) {
	base := ciShardOptions{Context: "context.json", Plan: "plan.json", Index: 0}
	if err := (ciShardNativeOptions{ExecuteOut: "new", Compiler: "compiler"}).validate(base, false); err != nil {
		t.Fatal(err)
	}
	base.Index = -1
	base.Partials = ciShardPaths{"partial0.json", "partial1.json"}
	if err := (ciShardNativeOptions{VerifyOut: "new.json", Evidence: ciShardPaths{"evidence0", "evidence1"}}).validate(base, false); err != nil {
		t.Fatal(err)
	}
	for _, n := range []ciShardNativeOptions{
		{VerifyOut: "new.json", Evidence: ciShardPaths{"one"}},
		{VerifyOut: "new.json", Evidence: ciShardPaths{"one", "two", "extra"}},
		{VerifyOut: "new.json", Evidence: ciShardPaths{"one", "two"}, CompilerRequested: true},
	} {
		if err := n.validate(base, false); err == nil {
			t.Fatal("mixed native modes accepted")
		}
	}
}
