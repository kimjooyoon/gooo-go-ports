package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

type unreadDescriptionInput struct{ reads int }

func (r *unreadDescriptionInput) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("describe must not read stdin")
}

func TestCLIDescribeWithoutInputOrExternalTools(t *testing.T) {
	t.Setenv("PATH", "")
	for _, option := range []string{"-describe", "--describe"} {
		input := &unreadDescriptionInput{}
		var output, stderr bytes.Buffer
		if code := execute([]string{option}, input, &output, &stderr); code != 0 || stderr.Len() != 0 || input.reads != 0 {
			t.Fatalf("%s: exit=%d reads=%d stderr=%s", option, code, input.reads, stderr.String())
		}
		if bytes.Count(output.Bytes(), []byte("\n")) != 1 || output.Bytes()[output.Len()-1] != '\n' {
			t.Fatal("description must be one complete JSON record with LF")
		}
		var got struct {
			Schema string `json:"schema"`
			Status string `json:"status"`
			Scope  string `json:"scope"`
			Modes  []struct {
				Mode         string `json:"mode"`
				InputRecord  string `json:"input_record_type"`
				OutputRecord string `json:"output_record_type"`
				InputBytes   int    `json:"input_bytes"`
				OutputBytes  int    `json:"batch_output_bytes_including_LF"`
			} `json:"modes"`
			Batch           describedBatch               `json:"batch_items"`
			Facts           []describedField             `json:"required_raw_fact_fields"`
			Decisions       []describedField             `json:"decision_fields"`
			Profiles        revision.EmbeddedDescription `json:"embedded_profiles"`
			Executed        bool                         `json:"policy_executed"`
			Authenticated   bool                         `json:"producer_authenticated"`
			Accepted        bool                         `json:"native_acceptance"`
			InstalledEngine bool                         `json:"installed_engine_checked"`
			Bundle          bool                         `json:"bundle_archive_checked"`
		}
		if err := json.Unmarshal(output.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Schema != "gooo/api-policy-revision-description/v1" || got.Status != "DECLARED_CAPABILITIES" || got.Scope != "embedded_bindings_checked_introspection_only" || got.Executed || got.Authenticated || got.Accepted || got.InstalledEngine || got.Bundle {
			t.Fatalf("description scope changed: %+v", got)
		}
		if got.Batch != (describedBatch{1, 128}) || len(got.Modes) != 3 || len(got.Facts) != 9 || len(got.Decisions) != 15 {
			t.Fatal("description changed the supported batch or record domain")
		}
		if original.MaxTextBytes != 65536 {
			t.Fatal("raw Text domain changed; review the description contract")
		}
		inputTypes := []string{"APIFacts", "CompareRequest", "ComparisonReport"}
		outputTypes := []string{"CompareRequest", "ComparisonReport", "ComparisonReport"}
		for i, mode := range []string{"baseline", "compare", "replay"} {
			wantInput, wantOutput := 4<<20, 16<<20
			if mode == "baseline" {
				wantOutput = 4 << 20
			}
			if mode == "replay" {
				wantInput = 16 << 20
			}
			if got.Modes[i].Mode != mode || got.Modes[i].InputRecord != inputTypes[i] || got.Modes[i].OutputRecord != outputTypes[i] || got.Modes[i].InputBytes != wantInput || got.Modes[i].OutputBytes != wantOutput {
				t.Fatalf("wrong %s limits: %+v", mode, got.Modes[i])
			}
		}
		booleanNames := map[string]bool{"group_present": true, "principal_present": true, "authenticated": true}
		wantFacts := []string{"group_present", "group_value", "principal_present", "principal_value", "authenticated", "caller_tenant", "resource_tenant", "requested_action", "allowed_action"}
		for i, field := range got.Facts {
			if field.Name != wantFacts[i] {
				t.Fatalf("wrong field: %+v", field)
			}
			if booleanNames[field.Name] {
				if field.Type != "boolean" || field.MaxUTF8Bytes != 0 {
					t.Fatalf("wrong boolean: %+v", field)
				}
			} else if field.Type != "string" || field.MaxUTF8Bytes != original.MaxTextBytes {
				t.Fatalf("wrong raw Text domain: %+v", field)
			}
		}
		bindings, err := revision.RevisionBindings()
		if err != nil {
			t.Fatal(err)
		}
		frozen, err := original.SourceBindings()
		if err != nil {
			t.Fatal(err)
		}
		if got.Profiles.OriginalProfile != revision.OriginalProfile || got.Profiles.CandidateProfile != revision.CandidateProfile || !reflect.DeepEqual(got.Profiles.Revision, bindings) || !reflect.DeepEqual(got.Profiles.Original, frozen) || len(bindings.Assets) != 38 || len(frozen.Assets) != 28 {
			t.Fatal("description is not bound to the actual embedded profiles")
		}
		var compiler map[string]any
		if json.Unmarshal(got.Profiles.Compiler, &compiler) != nil || compiler["compiler_source_sha"] != revision.CompilerPin || compiler["native_go_required"] != "go1.27.2" {
			t.Fatal("description changed the embedded generation compiler pin")
		}
	}
}

func TestCLIDescribeRejectsExplicitEvaluationFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--describe", "--mode=compare"}, {"--mode=baseline", "-describe"},
		{"--describe", "--input=/file-must-not-open"}, {"--input=", "--describe"},
		{"--describe", "--batch"}, {"--describe", "--batch=false"},
		{"--describe=false", "--mode=compare"},
	} {
		input := &unreadDescriptionInput{}
		var stdout, stderr bytes.Buffer
		code := execute(append([]string{"--diagnostics=json"}, args...), input, &stdout, &stderr)
		var diagnostic struct {
			Status      string `json:"status"`
			Diagnostics []struct {
				Code  string `json:"code"`
				Stage string `json:"stage"`
			} `json:"diagnostics"`
		}
		if code != 1 || stdout.Len() != 0 || input.reads != 0 || json.Unmarshal(stderr.Bytes(), &diagnostic) != nil || diagnostic.Status != "REFUSED" || len(diagnostic.Diagnostics) != 1 || diagnostic.Diagnostics[0].Code != "ARGUMENT_INVALID" || diagnostic.Diagnostics[0].Stage != "parse_arguments" {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s reads=%d", args, code, stdout.String(), stderr.String(), input.reads)
		}
	}
}

type shortDescriptionWriter struct{}

func (shortDescriptionWriter) Write(raw []byte) (int, error) { return len(raw) - 1, nil }

func TestCLIDescribeKeepsOutputFailureNonzero(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute([]string{"--diagnostics=json", "--describe"}, &unreadDescriptionInput{}, shortDescriptionWriter{}, &stderr); code != 1 || !bytes.Contains(stderr.Bytes(), []byte(`"code":"OUTPUT_IO"`)) {
		t.Fatalf("short description write: exit=%d stderr=%s", code, stderr.String())
	}
	var output bytes.Buffer
	if err := run([]string{"--describe"}, &unreadDescriptionInput{}, &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var document any
	if decoder.Decode(&document) != nil || decoder.Decode(&document) != io.EOF {
		t.Fatal("description is not exactly one JSON value")
	}
}
