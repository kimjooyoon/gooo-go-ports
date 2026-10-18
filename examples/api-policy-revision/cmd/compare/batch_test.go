package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func batchTestJSON(t *testing.T, value any, escapeHTML bool) []byte {
	t.Helper()
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(escapeHTML)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func batchTestFacts(t *testing.T) original.APIFacts {
	t.Helper()
	facts, err := original.DecodeAPI([]byte(syntheticFacts))
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func batchTestFailure(t *testing.T, mode string, raw []byte, code, stage string, index int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if status := execute([]string{"--diagnostics=json", "--batch", "--mode", mode}, bytes.NewReader(raw), &stdout, &stderr); status != 1 || stdout.Len() != 0 {
		t.Fatalf("batch refusal emitted a result: status %d, output %d bytes", status, stdout.Len())
	}
	checkDiagnostic(t, stderr.Bytes(), code, stage)
	var record struct {
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	actual := record.Diagnostics[0].ItemIndex
	if index < 0 && actual != nil || index >= 0 && (actual == nil || *actual != index) {
		t.Fatalf("wrong item index %v, want %d", actual, index)
	}
}

func TestBatchMatchesCompleteSingleRecordsOrderAndDuplicates(t *testing.T) {
	write := batchTestFacts(t)
	read := write
	read.RequestedAction, read.AllowedAction = "read", "read"
	denied := read
	denied.Authenticated = false
	rows := []json.RawMessage{}
	for _, facts := range []original.APIFacts{read, write, denied, read} {
		rows = append(rows, batchTestJSON(t, facts, true))
	}
	var comparison []byte
	for _, mode := range []string{"baseline", "compare", "replay"} {
		expected := make([]json.RawMessage, len(rows))
		for i, row := range rows {
			var single, stderr bytes.Buffer
			if status := execute([]string{"--diagnostics=json", "--mode", mode}, bytes.NewReader(row), &single, &stderr); status != 0 || stderr.Len() != 0 {
				t.Fatalf("single %s item%d: status %d, %s", mode, i, status, stderr.String())
			}
			expected[i] = append([]byte(nil), single.Bytes()...)
		}
		var output, stderr bytes.Buffer
		if status := execute([]string{"--diagnostics=json", "--batch", "--mode", mode}, bytes.NewReader(batchTestJSON(t, rows, true)), &output, &stderr); status != 0 || stderr.Len() != 0 {
			t.Fatalf("batch %s: status %d, %s", mode, status, stderr.String())
		}
		if !bytes.Equal(output.Bytes(), batchTestJSON(t, expected, true)) || bytes.Count(output.Bytes(), []byte("\n")) != 1 {
			t.Fatal(mode, "changed complete records, ordering, duplicates or final LF")
		}
		if mode == "compare" {
			comparison = append([]byte(nil), output.Bytes()...)
			var reports []revision.ComparisonReport
			if err := json.Unmarshal(comparison, &reports); err != nil {
				t.Fatal(err)
			}
			for i, want := range []string{"UNCHANGED", "NEWLY_DENIED", "UNCHANGED", "UNCHANGED"} {
				if reports[i].Summary.Classification != want {
					t.Fatal("mixed policy decisions/order changed", i, reports[i].Summary)
				}
			}
		}
		if mode == "replay" && !bytes.Equal(comparison, output.Bytes()) {
			t.Fatal("complete saved batch replay bytes changed")
		}
		rows = expected
	}
}

func TestBatchLateMalformedOrTamperedItemPreservesRefusalCause(t *testing.T) {
	facts := batchTestFacts(t)
	request, err := revision.Baseline(facts)
	if err != nil {
		t.Fatal(err)
	}
	report, err := revision.Compare(request)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := strings.Replace(syntheticFacts, `"group_present":true`, `"group_present":true,"\u0067roup_present":true`, 1)
	for _, bad := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`null`), json.RawMessage(duplicate)} {
		batchTestFailure(t, "baseline", batchTestJSON(t, []json.RawMessage{batchTestJSON(t, facts, true), bad}, true), "INPUT_INVALID", "decode_facts", 1)
	}
	altered := request
	altered.Before.Decision.Allowed = !altered.Before.Decision.Allowed
	batchTestFailure(t, "compare", batchTestJSON(t, []revision.CompareRequest{request, altered}, true), "SAVED_DECISION_MISMATCH", "verify_original", 1)
	changed := report
	changed.Differences.Reason = !changed.Differences.Reason
	raw := batchTestJSON(t, []revision.ComparisonReport{report, changed}, true)
	batchTestFailure(t, "replay", raw, "REPORT_MISMATCH", "verify_comparison", 1)
	if err := run([]string{"--batch", "--mode=replay"}, bytes.NewReader(raw), &bytes.Buffer{}); !errors.Is(err, revision.ErrComparisonMismatch) {
		t.Fatal("batch diagnostic replaced the original policy cause", err)
	}
}

func TestBatchOuterArrayCountAndInputLimits(t *testing.T) {
	tooMany := make([]json.RawMessage, 129)
	for i := range tooMany {
		tooMany[i] = json.RawMessage(syntheticFacts)
	}
	for _, mode := range []string{"baseline", "compare", "replay"} {
		for _, raw := range [][]byte{[]byte(`[]`), []byte(`null`), []byte(`{}`), []byte(`[{}] {}`), batchTestJSON(t, tooMany, true)} {
			batchTestFailure(t, mode, raw, "INPUT_INVALID", "decode_batch", -1)
		}
		limit := revision.MaxInputBytes
		if mode == "replay" {
			limit = revision.MaxReportBytes
		}
		batchTestFailure(t, mode, bytes.Repeat([]byte(" "), limit+1), "INPUT_LIMIT_EXCEEDED", "read_input", -1)
	}
	var output bytes.Buffer
	if err := run([]string{"--batch", "--mode=baseline"}, bytes.NewReader(batchTestJSON(t, tooMany[:128], true)), &output); err != nil {
		t.Fatal("128 valid items refused", err)
	}
	var requests []revision.CompareRequest
	if err := json.Unmarshal(output.Bytes(), &requests); err != nil || len(requests) != 128 {
		t.Fatal("maximum-count batch did not retain every item", err)
	}
}

func TestBatchZeroIndexAndSingleDiagnosticShape(t *testing.T) {
	batchTestFailure(t, "baseline", []byte(`[{}]`), "INPUT_INVALID", "decode_facts", 0)
	var stdout, stderr bytes.Buffer
	if status := execute([]string{"--diagnostics=json", "--mode=baseline"}, strings.NewReader(`{}`), &stdout, &stderr); status != 1 || stdout.Len() != 0 {
		t.Fatal("single refusal contract changed")
	}
	var record struct {
		Diagnostics []map[string]json.RawMessage `json:"diagnostics"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &record); err != nil || len(record.Diagnostics) != 1 || len(record.Diagnostics[0]) != 4 {
		t.Fatal("single diagnostic acquired batch fields", err)
	}
	if _, exists := record.Diagnostics[0]["item_index"]; exists {
		t.Fatal("single diagnostic contains item_index")
	}
}

func TestBatchOutputLimitsIncludeTheCompleteEncodedArray(t *testing.T) {
	facts := batchTestFacts(t)
	facts.GroupValue = strings.Repeat("x", 32000)
	many := make([]original.APIFacts, 128)
	for i := range many {
		many[i] = facts
	}
	raw := batchTestJSON(t, many, true)
	if len(raw) > revision.MaxInputBytes {
		t.Fatal("baseline output-limit fixture exceeds the input bound")
	}
	checkLimit := func(mode string, raw []byte) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if status := execute([]string{"--diagnostics=json", "--batch", "--mode", mode}, bytes.NewReader(raw), &stdout, &stderr); status != 1 || stdout.Len() != 0 {
			t.Fatal(mode, "oversized output was emitted")
		}
		checkDiagnostic(t, stderr.Bytes(), "OUTPUT_LIMIT_EXCEEDED", "check_output_limit")
	}
	checkLimit("baseline", raw)
	text := strings.Repeat("<", original.MaxTextBytes)
	large := original.APIFacts{Facts: original.Facts{GroupValue: text, PrincipalValue: text},
		CallerTenant: text, ResourceTenant: text, RequestedAction: text, AllowedAction: text}
	request, err := revision.Baseline(large)
	if err != nil {
		t.Fatal(err)
	}
	report, err := revision.Compare(request)
	if err != nil {
		t.Fatal(err)
	}
	requests := []revision.CompareRequest{request, request, request, request}
	reports := []revision.ComparisonReport{report, report, report, report}
	compactRequests, compactReports := batchTestJSON(t, requests, false), batchTestJSON(t, reports, false)
	if len(compactRequests) > revision.MaxInputBytes || len(compactReports) > revision.MaxReportBytes || len(batchTestJSON(t, reports, true)) <= revision.MaxReportBytes {
		t.Fatal("HTML-escaped output-limit fixture does not distinguish input/output budgets")
	}
	checkLimit("compare", compactRequests)
	checkLimit("replay", compactReports)
}
