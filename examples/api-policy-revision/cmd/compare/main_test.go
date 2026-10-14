package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

const syntheticFacts = `{"group_present":true,"group_value":"admin","principal_present":true,"principal_value":"synthetic:user","authenticated":true,"caller_tenant":"tenant-a","resource_tenant":"tenant-a","requested_action":"write","allowed_action":"write"}`

func TestCLICompleteBaselineCompareReplay(t *testing.T) {
	var baseline bytes.Buffer
	if err := run([]string{"-mode", "baseline"}, bytes.NewBufferString(syntheticFacts), &baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := revision.DecodeCompareRequest(baseline.Bytes()); err != nil {
		t.Fatal(err)
	}
	var compared bytes.Buffer
	if err := run([]string{"-mode", "compare"}, &baseline, &compared); err != nil {
		t.Fatal(err)
	}
	report, err := revision.DecodeComparisonReport(compared.Bytes())
	if err != nil || report.Summary != (revision.Summary{Classification: "NEWLY_DENIED", ChangedCount: 2}) {
		t.Fatalf("comparison: %v", err)
	}
	var replay bytes.Buffer
	if err := run([]string{"-mode", "replay"}, bytes.NewReader(compared.Bytes()), &replay); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compared.Bytes(), replay.Bytes()) || bytes.Count(replay.Bytes(), []byte("\n")) != 1 {
		t.Fatal("CLI did not emit exactly one matching complete result")
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, compared.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var fromFile bytes.Buffer
	if err := run([]string{"-mode", "replay", "-input", path}, bytes.NewBufferString("invalid unused stdin"), &fromFile); err != nil || !bytes.Equal(fromFile.Bytes(), replay.Bytes()) {
		t.Fatalf("explicit input-file replay: %v", err)
	}
}

func TestCLILargestValidEscapedFactsRoundTrip(t *testing.T) {
	text := strings.Repeat("<", original.MaxTextBytes)
	facts := original.APIFacts{
		Facts: original.Facts{
			GroupPresent:     false,
			GroupValue:       text,
			PrincipalPresent: false,
			PrincipalValue:   text,
		},
		Authenticated:   false,
		CallerTenant:    text,
		ResourceTenant:  text,
		RequestedAction: text,
		AllowedAction:   text,
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > revision.MaxInputBytes {
		t.Fatal("valid maximum-length facts unexpectedly exceed the request bound")
	}
	var baseline bytes.Buffer
	if err := run([]string{"-mode", "baseline"}, bytes.NewReader(raw), &baseline); err != nil {
		if baseline.Len() != 0 {
			t.Fatal("failed baseline emitted partial JSON")
		}
		t.Fatal(err)
	}
	if baseline.Len() > revision.MaxInputBytes {
		t.Fatal("maximum-length facts produced a request above its input bound")
	}
	var compared bytes.Buffer
	if err := run([]string{"-mode", "compare"}, &baseline, &compared); err != nil {
		if compared.Len() != 0 {
			t.Fatal("failed comparison emitted partial JSON")
		}
		t.Fatal(err)
	}
	if compared.Len() <= revision.MaxInputBytes {
		t.Fatal("escaped before/after facts did not exercise the larger report bound")
	}
	var report revision.ComparisonReport
	if err := json.Unmarshal(compared.Bytes(), &report); err != nil {
		t.Fatalf("comparison did not emit one complete JSON report: %v", err)
	}
	if report.Before.Facts != facts || report.After.Facts != facts {
		t.Fatal("comparison changed maximum-length inactive raw facts")
	}
	var replay bytes.Buffer
	if err := run([]string{"-mode", "replay"}, bytes.NewReader(compared.Bytes()), &replay); err != nil {
		if replay.Len() != 0 {
			t.Fatal("failed replay emitted partial JSON")
		}
		t.Fatalf("the CLI rejected its own complete valid report: %v", err)
	}
	if !bytes.Equal(compared.Bytes(), replay.Bytes()) || bytes.Count(replay.Bytes(), []byte("\n")) != 1 {
		t.Fatal("maximum-length facts did not replay as exactly one identical complete report")
	}
}

func TestCLIFailureEmitsNoPartialJSON(t *testing.T) {
	for _, mode := range []string{"baseline", "compare", "replay", "unknown"} {
		for _, input := range []string{"", "null", "{}", "{} {}", syntheticFacts + " []"} {
			var output bytes.Buffer
			if err := run([]string{"-mode", mode}, bytes.NewBufferString(input), &output); err == nil || output.Len() != 0 {
				t.Fatalf("%s malformed input emitted a partial result", mode)
			}
		}
	}
	var baseline, compared bytes.Buffer
	if err := run([]string{"-mode", "baseline"}, bytes.NewBufferString(syntheticFacts), &baseline); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-mode", "compare"}, &baseline, &compared); err != nil {
		t.Fatal(err)
	}
	var report revision.ComparisonReport
	if err := json.Unmarshal(compared.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	report.Differences.EffectivePrincipal = !report.Differences.EffectivePrincipal
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"-mode", "replay"}, bytes.NewReader(raw), &output); err == nil || output.Len() != 0 {
		t.Fatal("late mismatched classifier result emitted partial JSON")
	}
	for _, mode := range []string{"baseline", "compare"} {
		output.Reset()
		if err := run([]string{"-mode", mode}, bytes.NewReader(bytes.Repeat([]byte("x"), revision.MaxInputBytes+1)), &output); err == nil || output.Len() != 0 {
			t.Fatalf("%s oversized request emitted JSON", mode)
		}
	}
	oversizedReport := bytes.Repeat([]byte("x"), revision.MaxReportBytes+1)
	output.Reset()
	if err := run([]string{"-mode", "replay"}, bytes.NewReader(oversizedReport), &output); err == nil || err.Error() != "input exceeds 16 MiB" || output.Len() != 0 {
		t.Fatal("oversized replay did not refuse at the report limit without JSON")
	}
	if _, err := revision.DecodeComparisonReport(oversizedReport); err == nil {
		t.Fatal("report decoder accepted input above the report limit")
	}
}
