// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime/debug"
	"testing"

	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

type unreadableInput struct{}

func (unreadableInput) Read([]byte) (int, error) { panic("discovery or invalid arguments read stdin") }

func TestDiscoveryDoesNotConsumeInput(t *testing.T) {
	for _, option := range []string{"--help", "-h", "--example", "--version"} {
		t.Run(option, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			if code := runCommand([]string{option}, unreadableInput{}, &out, &diagnostics); code != 0 || diagnostics.Len() != 0 || out.Len() == 0 {
				t.Fatalf("discovery failed: code=%d stderr=%s", code, &diagnostics)
			}
			if option == "--example" {
				original, err := os.ReadFile("../../example-input.json")
				if err != nil || !bytes.Equal(out.Bytes(), original) {
					t.Fatal("embedded example differs from public source fixture", err)
				}
			}
			if option == "--version" {
				var version commandVersion
				if json.Unmarshal(out.Bytes(), &version) != nil || version.Schema != "gooo/recorded-catalog-command-build/v1" {
					t.Fatal("version is not build-description JSON")
				}
			}
		})
	}
}

func TestUnsupportedAndCombinedArgumentsRefuseBeforeInput(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"--input", "file.json"}, {"--help", "--version"}, {"--example", "extra"}, {"--example=false"}, {"--"}, {"--diagnostics"}, {"--definitions", "--diagnostics", "--diagnostics"}, {"--definitions", "--diagnostics", "--before-policy"}, {"--definitions", "--before-policy", "x", "--diagnostics"}} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 {
			t.Fatalf("args %q not refused without stdout: code=%d", args, code)
		}
		var problem origin.Problem
		if json.Unmarshal(diagnostics.Bytes(), &problem) != nil || problem.State != "INVALID_ARGUMENT" || problem.Stage != "parse_arguments" || problem.Code != "UNSUPPORTED_ARGUMENTS" {
			t.Fatalf("missing argument diagnostic for %q: %s", args, &diagnostics)
		}
	}
}

func TestEmbeddedExampleIsIndependentAndRunsAsSeparateComparison(t *testing.T) {
	first := origin.ExampleInputJSON()
	first[0] = '!'
	var sample, report, diagnostics bytes.Buffer
	if runCommand([]string{"--example"}, unreadableInput{}, &sample, &diagnostics) != 0 {
		t.Fatal("a previous caller corrupted the embedded example")
	}
	if code := runCommand(nil, &sample, &report, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("example comparison failed: code=%d stderr=%s", code, &diagnostics)
	}
	var got origin.Report
	if err := json.Unmarshal(report.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.AliasDecision.State != "ORIGIN_CHANGED" || got.CollectionContext.State != "RECORDED_CONTEXT_COMPLETE" || !got.CollectionContext.ContainsSyntheticFixture || got.CollectionContext.RemoteAtomicity != "UNKNOWN" || got.CollectionContext.ExecutionAuthorized || got.AliasDecision.ExecutionAuthorized || got.CollectionContext.LiveRemovalProven {
		t.Fatal("example lost its decisions or uncertainty/authority boundaries")
	}
}

func TestBuildDescriptionPreservesUnknownDevelopmentAndReplacement(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {GoVersion: "ignored"}} {
		v := versionInfo(info, false)
		if v.BuildInfoAvailable || v.MainModule != nil || v.GoVersion != "" || v.PackagePath != "" || len(v.VCS) != 0 {
			t.Fatal("missing build metadata acquired an invented identity")
		}
	}
	info := &debug.BuildInfo{GoVersion: "go1.27.3", Path: "example.test/compare", Main: debug.Module{Path: "example.test", Version: "(devel)", Replace: &debug.Module{Path: "./local"}}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "observed-revision"}, {Key: "vcs.modified", Value: "true"}, {Key: "-ldflags", Value: "private unrelated build option"}}}
	v := versionInfo(info, true)
	if !v.BuildInfoAvailable || v.MainModule.Version != "(devel)" || v.MainModule.Sum != "" || v.MainModule.Replace.Path != "./local" || v.VCS["vcs.modified"] != "true" || v.VCS["vcs.revision"] != "observed-revision" || len(v.VCS) != 2 {
		t.Fatal("build description hid development/replacement status or leaked unrelated settings")
	}
}

type failingOutput struct{ short bool }

func (w failingOutput) Write([]byte) (int, error) {
	if w.short {
		return 0, nil
	}
	return 0, errors.New("output unavailable")
}

func TestDiscoveryAndComparisonOutputFailuresAreNotSuccess(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"--example"}, {"--version"}, {"--definitions", "--diagnostics"}} {
		for _, short := range []bool{false, true} {
			var diagnostics bytes.Buffer
			if code := runCommand(args, bytes.NewReader(origin.ExampleInputJSON()), failingOutput{short: short}, &diagnostics); code != 1 {
				t.Fatalf("output failure reported success: args=%q short=%v", args, short)
			}
			var problem origin.Problem
			if json.Unmarshal(diagnostics.Bytes(), &problem) != nil || problem.Code != "IO_FAILURE" {
				t.Fatalf("output failure lost diagnostic: %s", &diagnostics)
			}
		}
	}
}

type errorInput struct{}

func (errorInput) Read([]byte) (int, error) { return 0, errors.New("input unavailable") }

func TestInputAndTransportFailuresStillRefuseWithoutReport(t *testing.T) {
	for _, input := range []io.Reader{io.MultiReader(bytes.NewBufferString("{"), errorInput{}), bytes.NewBufferString(`{"target_alias":"a","target_alias":"b"}`)} {
		var out, diagnostics bytes.Buffer
		if runCommand(nil, input, &out, &diagnostics) != 1 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal("input failure produced a report or lost diagnostic")
		}
	}
}

func TestDefinitionArgumentsAndOutput(t *testing.T) {
	for _, args := range [][]string{{"--definitions", "--before-policy"}, {"--definitions", "--before-policy", "r1", "--before-policy", "r2"}, {"--definitions", "--unknown", "x"}, {"--definitions", "--help"}, {"--definitions", "--after-policy", "a", "--before-policy", "b", "extra"}} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal("invalid definition arguments consumed input or produced result", args)
		}
	}
	raw, err := os.ReadFile("../../definition-consumer-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input  json.RawMessage `json:"input"`
			Before string          `json:"before_policy_revision"`
			After  string          `json:"after_policy_revision"`
			State  string          `json:"expected_state"`
		} `json:"cases"`
	}
	if json.Unmarshal(raw, &corpus) != nil || len(corpus.Cases) != 12 {
		t.Fatal("consumer fixture")
	}
	for _, c := range corpus.Cases {
		var out, diagnostics bytes.Buffer
		args := []string{"--definitions", "--before-policy", c.Before, "--after-policy", c.After}
		if code := runCommand(args, bytes.NewReader(c.Input), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
			t.Fatal("definition CLI", code, diagnostics.String())
		}
		var report origin.DefinitionReport
		if json.Unmarshal(out.Bytes(), &report) != nil || report.Decision.State != c.State || report.Decision.ExecutionAuthorized || report.Decision.LiveEffectPerformed || report.Decision.BehaviorEquivalenceProven {
			t.Fatal("definition CLI decision")
		}
	}
	for _, input := range []io.Reader{errorInput{}, bytes.NewBufferString(`{"target_alias":"x","target_alias":"y"}`)} {
		var out, diagnostics bytes.Buffer
		if runCommand([]string{"--definitions"}, input, &out, &diagnostics) != 1 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal("definition input failure lost")
		}
	}
	for _, short := range []bool{false, true} {
		var diagnostics bytes.Buffer
		if runCommand([]string{"--definitions"}, bytes.NewReader(corpus.Cases[0].Input), failingOutput{short: short}, &diagnostics) != 1 || diagnostics.Len() == 0 {
			t.Fatal("definition output failure lost")
		}
	}
}

func TestDefinitionDiagnosticCommandMatchesAPI(t *testing.T) {
	for _, spec := range []struct {
		args          []string
		before, after string
	}{
		{[]string{"--definitions", "--diagnostics"}, "", ""},
		{[]string{"--definitions", "--diagnostics", "--before-policy", "fixture-old", "--after-policy", "fixture-new"}, "fixture-old", "fixture-new"},
		{[]string{"--definitions", "--diagnostics", "--after-policy", "fixture-new"}, "", "fixture-new"},
	} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(spec.args, bytes.NewReader(origin.ExampleInputJSON()), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
			t.Fatal(code, &diagnostics)
		}
		report, err := origin.CompareDefinitionDiagnosticsJSON(origin.ExampleInputJSON(), spec.before, spec.after)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), append(expected, '\n')) {
			t.Fatal("command differs from source-owned API report")
		}
	}
}
func TestDiagnosticTransportFailureHasNoPartialReport(t *testing.T) {
	for _, input := range []io.Reader{bytes.NewBufferString("{"), errorInput{}} {
		var out, diagnostics bytes.Buffer
		if code := runCommand([]string{"--definitions", "--diagnostics"}, input, &out, &diagnostics); code != 1 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal(code, &out, &diagnostics)
		}
	}
}
