// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func savedAvailability(t *testing.T, raw []byte) string {
	t.Helper()
	doc, err := origin.AvailabilityTemplateJSON(raw, "p", "p")
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Fields {
		doc.Fields[i].ValueState = "SUPPLIED"
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "availability.json")
	if err = os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExplicitAvailabilityFlagsRefuseBeforeStdin(t *testing.T) {
	for _, args := range [][]string{
		{"--availability", "a"}, {"--definitions", "--availability", "a"},
		{"--definitions", "--diagnostics", "--availability"},
		{"--definitions", "--diagnostics", "--availability", ""},
		{"--definitions", "--diagnostics", "--availability", "a", "--availability", "b"},
		{"--availability-template", "--availability", "a"},
		{"--availability-template", "--before-policy"},
		{"--availability-template", "--before-policy", "p", "--before-policy", "q"},
	} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 {
			t.Fatal(args, code, &diagnostics)
		}
	}
}

func TestAvailabilityTemplateAndSavedFileActuallyReachExplicitDecision(t *testing.T) {
	raw := origin.ExampleInputJSON()
	var out, diagnostics bytes.Buffer
	if code := runCommand([]string{"--availability-template", "--before-policy", "p", "--after-policy", "p"}, bytes.NewReader(raw), &out, &diagnostics); code != 0 {
		t.Fatal(code, &diagnostics)
	}
	var template origin.AvailabilityDocument
	if err := json.Unmarshal(out.Bytes(), &template); err != nil {
		t.Fatal(err)
	}
	if template.Schema != origin.AvailabilitySchema || len(template.Fields) == 0 {
		t.Fatal(template)
	}
	for _, field := range template.Fields {
		if field.ValueState != "" {
			t.Fatal("template silently supplied a field", field)
		}
	}
	path := savedAvailability(t, raw)
	args := []string{"--definitions", "--diagnostics", "--availability", path, "--before-policy", "p", "--after-policy", "p"}
	out.Reset()
	diagnostics.Reset()
	if code := runCommand(args, bytes.NewReader(raw), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatal(code, &diagnostics)
	}
	var report origin.ExplicitObservationReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != "gooo/explicit-definition-observation-report/v1" || report.Decision.State != "ORIGIN_NOT_UNCHANGED" || report.Decision.ExecutionAuthorized || report.Projection.AvailabilitySHA256 == "" {
		t.Fatal(report.Decision)
	}
	// A harmless whitespace change is still a different bound original input.
	out.Reset()
	diagnostics.Reset()
	if code := runCommand(args, bytes.NewReader(append(raw, ' ')), &out, &diagnostics); code != 1 || out.Len() != 0 {
		t.Fatal("source mismatch produced a partial report", code)
	}
	var problem origin.Problem
	if json.Unmarshal(diagnostics.Bytes(), &problem) != nil || problem.Code != "AVAILABILITY_SOURCE_BINDING" {
		t.Fatal(&diagnostics)
	}
}

func TestAvailabilityFileFailureDoesNotReadInputOrProduceReport(t *testing.T) {
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		var out, diagnostics bytes.Buffer
		args := []string{"--definitions", "--diagnostics", "--availability", path}
		if code := runCommand(args, unreadableInput{}, &out, &diagnostics); code != 1 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal(code, &diagnostics)
		}
	}
}

func TestExplicitAvailabilityIOFailuresAreNotSuccess(t *testing.T) {
	raw := origin.ExampleInputJSON()
	path := savedAvailability(t, raw)
	for _, args := range [][]string{
		{"--availability-template", "--before-policy", "p", "--after-policy", "p"},
		{"--definitions", "--diagnostics", "--availability", path, "--before-policy", "p", "--after-policy", "p"},
	} {
		for _, short := range []bool{false, true} {
			var diagnostics bytes.Buffer
			if code := runCommand(args, bytes.NewReader(raw), failingOutput{short: short}, &diagnostics); code != 1 {
				t.Fatal(args, code)
			}
		}
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, errorInput{}, &out, &diagnostics); code != 1 || out.Len() != 0 {
			t.Fatal(args, code)
		}
	}
}
