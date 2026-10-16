package main

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func headerAdditionFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := decimalPayloadReadFixture("../..", headerAdditionCorpusFile, headerAdditionCorpusSHA)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestHeaderAdditionFrozen50FreshSixteenFieldRecords(t *testing.T) {
	raw := headerAdditionFixture(t)
	population, err := headerAdditionInputs(raw)
	if err != nil {
		t.Fatal(err)
	}
	suites, err := headerAdditionCorpus("../..")
	if err != nil || len(suites) != 1 || len(population) != 50 || len(suites[0].Cases) != 50 || suites[0].Package != headerAdditionPackage || suites[0].Activity != headerAdditionActivity {
		t.Fatal("exact one fifty-case suite", err)
	}
	wantNames := []string{"candidate_name", "candidate_value", "status", "reason", "payload_eligible", "payload_reason", "header_name_valid", "header_value_valid", "name_invalid_offset", "name_invalid_octet", "value_invalid_offset", "value_invalid_octet", "value_has_obs_text", "value_edge_whitespace", "request_revision", "eligible"}
	for i, spec := range population {
		row := suites[0].Cases[i]
		if !reflect.DeepEqual(row.Inputs, headerAdditionArguments(spec.Input)) || validateHeaderAdditionInputs(row.Inputs) != nil {
			t.Fatal("input order/value/type changed", spec.Label)
		}
		record, ok := row.Expected.(map[string]any)
		if !ok || len(record) != 16 {
			t.Fatal("full16 expected record missing", spec.Label)
		}
		for _, name := range wantNames {
			if _, exists := record[name]; !exists {
				t.Fatal("missing expected field", name)
			}
		}
		for _, name := range []string{"payload_reason", "name_invalid_offset", "name_invalid_octet", "value_invalid_offset", "value_invalid_octet", "request_revision"} {
			if _, ok := record[name].(int64); !ok {
				t.Fatal("expected integer lost int64", name)
			}
		}
		for _, name := range []string{"payload_eligible", "header_name_valid", "header_value_valid", "value_has_obs_text", "value_edge_whitespace", "eligible"} {
			if _, ok := record[name].(bool); !ok {
				t.Fatal("expected Boolean changed", name)
			}
		}
		if record["candidate_name"] != spec.Input.Name || record["candidate_value"] != spec.Input.Value || record["request_revision"] != spec.Input.Facts.ObservedRequestRevision || record["status"] != spec.Status || record["reason"] != spec.Reason || record["payload_reason"] != spec.PayloadReason || record["eligible"] != spec.Eligible {
			t.Fatal("echo/partial specification mismatch", spec.Label)
		}
	}
	// Values here are independent contract witnesses, not copied native outputs.
	for _, witness := range []struct {
		at     int
		fields map[string]any
	}{
		{8, map[string]any{"header_name_valid": false, "name_invalid_offset": int64(-1), "name_invalid_octet": int64(-1)}},
		{9, map[string]any{"name_invalid_offset": int64(3), "name_invalid_octet": int64(195)}},
		{10, map[string]any{"value_invalid_offset": int64(2), "value_invalid_octet": int64(13)}},
		{11, map[string]any{"value_invalid_offset": int64(2), "value_invalid_octet": int64(0)}},
		{12, map[string]any{"value_invalid_offset": int64(2), "value_invalid_octet": int64(127)}},
		{16, map[string]any{"header_value_valid": true, "value_has_obs_text": true}},
		{43, map[string]any{"value_invalid_offset": int64(1), "value_invalid_octet": int64(13), "value_has_obs_text": true, "value_edge_whitespace": true}},
		{45, map[string]any{"status": "INVALID_INPUT", "eligible": false}},
		{46, map[string]any{"request_revision": int64(math.MaxInt64), "status": "INVALID_INPUT", "eligible": false}},
		{47, map[string]any{"request_revision": int64(math.MaxInt64), "status": "INVALID_INPUT", "eligible": false}},
		{48, map[string]any{"request_revision": int64(math.MaxInt64 - 1), "status": "ELIGIBLE", "eligible": true}},
		{49, map[string]any{"request_revision": int64(math.MaxInt64 - 1), "reason": "REQUEST_REVISION_CHANGED", "eligible": false}},
	} {
		got := suites[0].Cases[witness.at].Expected.(map[string]any)
		for name, want := range witness.fields {
			if got[name] != want {
				t.Fatal(population[witness.at].Label, name, got[name], want)
			}
		}
	}
}

func TestHeaderAdditionLateMalformedOrSpecMismatchReturnsNoPartialCases(t *testing.T) {
	raw := headerAdditionFixture(t)
	for _, kind := range []string{"missing", "extra", "duplicate-label", "null", "float-int", "large-int", "wrong-bool", "large-text", "unknown-fact", "missing-fact", "partial-type", "partial-mismatch"} {
		value, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		top := value.(map[string]any)
		rows := top["cases"].([]any)
		last := rows[49].(map[string]any)
		input := last["input"].(map[string]any)
		facts := input["facts"].(map[string]any)
		switch kind {
		case "missing":
			delete(last, "label")
		case "extra":
			last["actual"] = true
		case "duplicate-label":
			last["label"] = rows[0].(map[string]any)["label"]
		case "null":
			input["name"] = nil
		case "float-int":
			facts["observed_request_revision"] = json.Number("1e0")
		case "large-int":
			facts["observed_request_revision"] = json.Number("9223372036854775808")
		case "wrong-bool":
			facts["current_access"] = 1
		case "large-text":
			input["value"] = strings.Repeat("x", 1025)
		case "unknown-fact":
			facts["eligible"] = true
		case "missing-fact":
			delete(facts, "current_access")
		case "partial-type":
			last["expected"].(map[string]any)["eligible"] = 0
		case "partial-mismatch":
			last["expected"].(map[string]any)["eligible"] = true
		}
		altered, err := json.Marshal(top)
		if err != nil {
			t.Fatal(err)
		}
		cases, err := headerAdditionCases(altered)
		if err == nil || cases != nil {
			t.Fatal("late malformed/spec mismatch returned cases", kind)
		}
	}
	for _, invalid := range [][]byte{append(append([]byte{}, raw...), []byte(`{}`)...), []byte(`{"schema":"x","schema":"x"}`), []byte(`{"name":"\ud800"}`), {0xff}} {
		if out, err := headerAdditionCases(invalid); err == nil || out != nil {
			t.Fatal("bad JSON returned cases")
		}
	}
	// The public file loader separately refuses changed bytes before parsing.
	dir := t.TempDir()
	path := dir + "/examples/diagnostic-header-addition"
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+"/corpus.json", append(append([]byte{}, raw...), '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := headerAdditionCorpus(dir); err == nil || out != nil {
		t.Fatal("changed frozen source accepted")
	}
}

func TestHeaderAdditionABIDomainKeepsSemanticRefusalsAndExactIntegers(t *testing.T) {
	population, err := headerAdditionInputs(headerAdditionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range population {
		if err := validateHeaderAdditionInputs(headerAdditionArguments(spec.Input)); err != nil {
			t.Fatal("in-domain semantic refusal rejected", spec.Label, err)
		}
	}
	for _, change := range []func([]any){
		func(in []any) { in[0] = []byte("x") },
		func(in []any) { in[1] = strings.Repeat("x", 1025) },
		func(in []any) { in[2] = string([]byte{0xff}) },
		func(in []any) { in[3].(map[string]any)["observed_request_revision"] = float64(1) },
		func(in []any) { in[3].(map[string]any)["observed_request_revision"] = json.Number("1") },
		func(in []any) { in[3].(map[string]any)["current_access"] = int64(1) },
		func(in []any) { in[3].(map[string]any)["allowed_header_name"] = nil },
		func(in []any) { delete(in[3].(map[string]any), "current_access") },
		func(in []any) { in[3].(map[string]any)["extra"] = false },
	} {
		in := headerAdditionArguments(population[0].Input)
		change(in)
		if validateHeaderAdditionInputs(in) == nil {
			t.Fatal("wrong typed input admitted", in)
		}
	}
	if validateHeaderAdditionInputs(nil) == nil || validateHeaderAdditionInputs(headerAdditionArguments(population[0].Input)[:3]) == nil {
		t.Fatal("wrong arity")
	}
	// Out-of-domain input cannot be supplied directly to the fresh reference.
	in := population[0].Input
	in.Value = strings.Repeat("x", 1025)
	if out, err := headerAdditionReference(in); err == nil || out != nil {
		t.Fatal("reference widened bounded domain")
	}
}
