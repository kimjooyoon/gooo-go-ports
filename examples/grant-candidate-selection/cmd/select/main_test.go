package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	selection "github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection"
)

const facts = `{"before":"*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}],"candidates":[{"id":"weather-only","after":"mcp://weather","preference":0},{"id":"all-mcp","after":"mcp://*","preference":10}]}`

func TestCommandCompleteResultAndRepresentedRefusal(t *testing.T) {
	for _, after := range []string{"mcp://*", "http://*"} {
		input := strings.Replace(facts, `"after":"mcp://*"`, `"after":"`+after+`"`, 1)
		var stdout, stderr bytes.Buffer
		if code := run(strings.NewReader(input), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("command failed%d %s", code, stderr.String())
		}
		var doc map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		result := doc["result"].(map[string]any)
		choice := result["selection"].(map[string]any)
		want := "PROPOSED"
		if after == "http://*" {
			want = "REFUSED"
		}
		if choice["state"] != want || choice["execution_authorized"] != false || len(result["assessments"].([]any)) != 2 {
			t.Fatalf("incomplete result: %s", stdout.String())
		}
	}
}

func TestExactInputBudgetAndOversizedReportHaveNoPartialOutput(t *testing.T) {
	maximum := "{" + strings.Repeat(" ", selection.MaxInputBytes-len(facts)) + facts[1:]
	if len(maximum) != selection.MaxInputBytes {
		t.Fatal("wrong boundary fixture")
	}
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader(maximum), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exact input budget refused: %d %s", code, stderr.String())
	}
	input := selection.Input{Before: "*", RequiredRequests: make([]selection.Request, 256), Candidates: make([]selection.Candidate, 32)}
	for i := range input.RequiredRequests {
		input.RequiredRequests[i] = selection.Request{Type: strings.Repeat("x", 512), Name: strings.Repeat("y", 512)}
	}
	for i := range input.Candidates {
		input.Candidates[i] = selection.Candidate{ID: strings.Repeat("x", i+1), After: "mcp://*", Preference: 0}
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) >= selection.MaxInputBytes {
		t.Fatal("report fixture does not fit input budget")
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(bytes.NewReader(raw), &stdout, &stderr); code != 1 || stdout.Len() != 0 {
		t.Fatalf("oversized report leaked: %d bytes%d", code, stdout.Len())
	}
	var problem map[string]any
	if json.Unmarshal(stderr.Bytes(), &problem) != nil || problem["code"] != "OUTPUT_LIMIT_EXCEEDED" || problem["stage"] != "encode_result" {
		t.Fatalf("wrong report budget diagnostic: %s", stderr.String())
	}
}

func TestLateMalformedOrOversizedCommandEmitsNoDecision(t *testing.T) {
	for _, input := range []string{
		strings.Replace(facts, `"id":"all-mcp"`, `"id":"all-mcp","bindings_verified":true`, 1),
		facts + ` {}`, strings.Repeat(" ", (1<<20)+1),
	} {
		var stdout, stderr bytes.Buffer
		if code := run(strings.NewReader(input), &stdout, &stderr); code != 1 || stdout.Len() != 0 {
			t.Fatalf("invalid command leaked result: %d %s", code, stdout.String())
		}
		var problem map[string]any
		if json.Unmarshal(stderr.Bytes(), &problem) != nil || problem["state"] != "INVALID_TRANSPORT" || problem["stage"] != "decode_input" {
			t.Fatalf("invalid diagnostic: %s", stderr.String())
		}
	}
}
