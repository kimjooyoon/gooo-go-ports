package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
)

func TestDiagnosticModePreservesSuccessfulPrograms(t *testing.T) {
	input := []byte(syntheticFacts)
	for _, mode := range []string{"baseline", "compare", "replay"} {
		var ordinary, structured, stderr bytes.Buffer
		if status := execute([]string{"--mode", mode}, bytes.NewReader(input), &ordinary, &stderr); status != 0 || stderr.Len() != 0 {
			t.Fatal(mode, status, stderr.String())
		}
		if status := execute([]string{"--diagnostics=json", "--mode", mode}, bytes.NewReader(input), &structured, &stderr); status != 0 || stderr.Len() != 0 || !bytes.Equal(ordinary.Bytes(), structured.Bytes()) {
			t.Fatal("diagnostics changed a successful program", mode, status, stderr.String())
		}
		input = append([]byte(nil), ordinary.Bytes()...)
	}
	report, err := revision.DecodeComparisonReport(input)
	if err != nil || report.Summary.Classification != "NEWLY_DENIED" {
		t.Fatal("normal domain denial became a refusal", report.Summary, err)
	}
}

func TestDiagnosticCodesFollowRealRefusalCauses(t *testing.T) {
	var baseline, compared bytes.Buffer
	if err := run([]string{"--mode=baseline"}, strings.NewReader(syntheticFacts), &baseline); err != nil {
		t.Fatal(err)
	}
	request, err := revision.DecodeCompareRequest(baseline.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := run(nil, bytes.NewReader(baseline.Bytes()), &compared); err != nil {
		t.Fatal(err)
	}
	report, err := revision.DecodeComparisonReport(compared.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	profile := request
	profile.CandidateProfile = "unsupported"
	generation := request
	generation.Before.Bindings.GenerationSHA256 = "sha256:" + strings.Repeat("0", 64)
	decision := request
	decision.Before.Decision.Allowed = !decision.Before.Decision.Allowed
	report.Summary.ChangedCount = 1
	cases := []struct {
		name, mode, code, stage string
		input                   any
	}{
		{"profile", "compare", "PROFILE_MISMATCH", "check_profiles", profile},
		{"generation", "compare", "GENERATION_MISMATCH", "verify_original", generation},
		{"saved-decision", "compare", "SAVED_DECISION_MISMATCH", "verify_original", decision},
		{"report", "replay", "REPORT_MISMATCH", "verify_comparison", report},
		{"input", "baseline", "INPUT_INVALID", "decode_facts", map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if status := execute([]string{"--diagnostics=json", "--mode", tc.mode}, bytes.NewReader(raw), &stdout, &stderr); status != 1 || stdout.Len() != 0 {
				t.Fatal("refusal emitted a result or success", status, stdout.String())
			}
			checkDiagnostic(t, stderr.Bytes(), tc.code, tc.stage)
		})
	}
}

func TestDiagnosticOptionUsesOnlyParsedFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		json bool
	}{
		{"selected-before-error", []string{"--diagnostics=json", "--unknown"}, true},
		{"unseen-after-error", []string{"--unknown", "--diagnostics=json"}, false},
		{"invalid-format", []string{"--diagnostics=invalid"}, false},
		{"missing-value", []string{"--diagnostics=json", "--input"}, true},
		{"help", []string{"--diagnostics=json", "-h"}, true},
		{"input-value-is-not-option", []string{"--input", "--diagnostics=json"}, false},
		{"after-double-dash", []string{"--", "--diagnostics=json"}, false},
		{"after-positional", []string{"positional", "--diagnostics=json"}, false},
		{"last-text", []string{"--diagnostics=json", "--diagnostics=text"}, false},
		{"last-json", []string{"--diagnostics=text", "--diagnostics=json"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := execute(tc.args, strings.NewReader("{}"), &stdout, &stderr); status != 1 || stdout.Len() != 0 {
				t.Fatal(status, stdout.String())
			}
			if tc.json {
				code, stage := "ARGUMENT_INVALID", "parse_arguments"
				if tc.name == "last-json" {
					code, stage = "INPUT_INVALID", "decode_request"
				}
				checkDiagnostic(t, stderr.Bytes(), code, stage)
			} else if !strings.HasPrefix(stderr.String(), "API revision comparison refused: ") {
				t.Fatal("default text contract changed", stderr.String())
			}
		})
	}
}

func TestOutputAndDiagnosticFailuresPreservePrimaryCause(t *testing.T) {
	outputErr := errors.New("synthetic output failure")
	for _, cause := range []error{outputErr, nil} {
		stdout := &failingWriter{cause: cause}
		err := run([]string{"--diagnostics=json", "--mode=baseline"}, strings.NewReader(syntheticFacts), stdout)
		want := cause
		if want == nil {
			want = io.ErrShortWrite
		}
		if !errors.Is(err, want) || stdout.calls != 1 || stdout.Len() != 7 || json.Valid(stdout.Bytes()) {
			t.Fatal("partial output or primary failure changed", err, stdout.calls, stdout.String())
		}
		var diagnostic bytes.Buffer
		if !errors.Is(writeRefusal(&diagnostic, err), want) {
			t.Fatal("diagnostic replaced the primary error")
		}
		checkDiagnostic(t, diagnostic.Bytes(), "OUTPUT_IO", "write_stdout")
		deliveryErr := errors.New("synthetic diagnostic failure")
		stderr := &failingWriter{cause: deliveryErr}
		joined := writeRefusal(stderr, err)
		if !errors.Is(joined, want) || !errors.Is(joined, deliveryErr) || stderr.calls != 1 || stdout.calls != 1 {
			t.Fatal("failed diagnostics lost a cause or repeated output", joined)
		}
	}
	var stdout bytes.Buffer
	stderr := &failingWriter{cause: errors.New("stderr unavailable")}
	if status := execute([]string{"--diagnostics=json"}, strings.NewReader("{}"), &stdout, stderr); status != 1 || stdout.Len() != 0 || stderr.calls != 1 {
		t.Fatal("diagnostic delivery failure changed the refusal", status)
	}
}

func TestBothDiagnosticFormatsPreserveShortWrite(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		err := run([]string{"--diagnostics=" + format}, strings.NewReader("{}"), io.Discard)
		stderr := &failingWriter{}
		joined := writeRefusal(stderr, err)
		if !errors.Is(joined, err) || !errors.Is(joined, io.ErrShortWrite) || stderr.calls != 1 || stderr.Len() != 7 {
			t.Fatal("short diagnostics lost the original cause or partial output", format, joined)
		}
	}
}

type failingWriter struct {
	bytes.Buffer
	cause error
	calls int
}

func (writer *failingWriter) Write(raw []byte) (int, error) {
	writer.calls++
	n, _ := writer.Buffer.Write(raw[:min(7, len(raw))])
	return n, writer.cause
}

func checkDiagnostic(t *testing.T, raw []byte, code, stage string) {
	t.Helper()
	var record struct {
		Schema      string       `json:"schema"`
		Command     string       `json:"command"`
		Status      string       `json:"status"`
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &record); err != nil || record.Schema != "gooo/api-policy-revision-diagnostic/v1" || record.Command != "compare" || record.Status != "REFUSED" || len(record.Diagnostics) != 1 {
		t.Fatalf("not one versioned refusal: %s (%v)", raw, err)
	}
	actual := record.Diagnostics[0]
	if actual.Code != code || actual.Stage != stage || actual.Severity != "error" || actual.Message == "" {
		t.Fatalf("wrong refusal: %+v, want %s/%s", actual, code, stage)
	}
}
