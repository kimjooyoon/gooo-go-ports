package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const dpMaxInt64 = int64(1<<63 - 1)
const dpMinInt64 = int64(-1 << 63)

func TestDecimalPayloadOriginalGoErrorPrecedenceAndSaturation(t *testing.T) {
	for _, row := range []struct {
		name, text  string
		value, kind int64
	}{
		{"empty", "", 0, 1},
		{"zero", "0", 0, 0},
		{"negative-zero", "-0", 0, 0},
		{"explicit-plus", "+1", 1, 0},
		{"sign-only", "+", 0, 1},
		{"non-ascii-digit", "１", 0, 1},
		{"maximum", "9223372036854775807", dpMaxInt64, 0},
		{"minimum", "-9223372036854775808", dpMinInt64, 0},
		{"maximum-plus-one", "9223372036854775808", dpMaxInt64, 2},
		{"minimum-minus-one", "-9223372036854775809", dpMinInt64, 2},
		{"signed-overflow-before-invalid", "9223372036854775808x", 0, 1},
		{"uint64-limit-before-invalid", "18446744073709551615x", 0, 1},
		{"unsigned-overflow-before-invalid", "18446744073709551616x", dpMaxInt64, 2},
		{"negative-unsigned-overflow-before-invalid", "-18446744073709551616x", dpMinInt64, 2},
		{"zero-padded-1024-maximum", strings.Repeat("0", 1005) + "9223372036854775807", dpMaxInt64, 0},
		{"1024-final-invalid", strings.Repeat("0", 1023) + "x", 0, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := decimalInt64Reference(row.text)
			want := decimalInt64Facts{Input: row.text, Value: row.value, Parsed: row.kind == 0, ErrorKind: row.kind}
			if err != nil || got != want {
				t.Fatalf("original Go returned value/error facts: got=%+v want=%+v err=%v", got, want, err)
			}
			record := decimalInt64Record(got)
			if len(record) != 4 || !reflect.DeepEqual(record["input"], row.text) || !reflect.DeepEqual(record["value"], row.value) || !reflect.DeepEqual(record["error_kind"], row.kind) || !reflect.DeepEqual(record["parsed"], row.kind == 0) {
				t.Fatal("parser record lost an original value or exact scalar type", record)
			}
		})
	}
	for _, text := range []string{strings.Repeat("0", 1025), string([]byte{0xff})} {
		if _, err := decimalInt64Reference(text); err == nil {
			t.Fatal("out-of-domain Text reached a compatible Go expectation")
		}
	}
}

func dpInput(text string, observed, maximum int64) payloadAdmissionInput {
	return payloadAdmissionInput{DeclaredLength: text, ContentLengthOccurrences: 1, ObservedBodyBytes: observed, MaximumPayloadBytes: maximum, BodyObservationBoundToRequest: true, CurrentAccess: true}
}

func TestDecimalPayloadManualReasonsPreserveParserFactsAndInputEchoes(t *testing.T) {
	invalidMetadata := dpInput("0", dpMinInt64, 0)
	invalidMetadata.CurrentAccess, invalidMetadata.BodyObservationBoundToRequest, invalidMetadata.TransferEncodingPresent = false, false, true
	deniedRange := dpInput("9223372036854775808", dpMaxInt64, dpMaxInt64)
	deniedRange.CurrentAccess, deniedRange.BodyObservationBoundToRequest, deniedRange.TransferEncodingPresent = false, false, true
	unbound := dpInput("1", 1, 1)
	unbound.BodyObservationBoundToRequest, unbound.TransferEncodingPresent, unbound.ContentLengthOccurrences = false, true, 2
	transfer := dpInput("1", 1, 1)
	transfer.TransferEncodingPresent, transfer.ContentLengthOccurrences = true, 2
	missing := dpInput("", 0, 0)
	missing.ContentLengthOccurrences = 0
	repeated := dpInput("1", 1, 1)
	repeated.ContentLengthOccurrences = 2
	inconsistentMissing := dpInput("1", 0, 0)
	inconsistentMissing.ContentLengthOccurrences = 0
	negativeLimit := dpInput("1", 1, -1)
	negativeCount := dpInput("+", 0, 0)
	negativeCount.ContentLengthOccurrences = -1
	type facts struct {
		kind, value                         int64
		unsigned, matches, budget, eligible bool
		reason                              int64
	}
	for _, row := range []struct {
		name string
		in   payloadAdmissionInput
		want facts
	}{
		{"empty-body-with-zero-limit", dpInput("0", 0, 0), facts{0, 0, true, true, true, true, 0}},
		{"above-float-exactness", dpInput("9007199254740993", 9007199254740993, 9007199254740993), facts{0, 9007199254740993, true, true, true, true, 0}},
		{"metadata-before-all-other-refusals", invalidMetadata, facts{0, 0, true, false, false, false, 1}},
		{"access-before-binding-retains-range", deniedRange, facts{2, dpMaxInt64, true, false, true, false, 2}},
		{"binding-before-transfer-and-count", unbound, facts{0, 1, true, true, true, false, 3}},
		{"transfer-before-count", transfer, facts{0, 1, true, true, true, false, 4}},
		{"missing-retains-syntax", missing, facts{1, 0, false, false, true, false, 5}},
		{"duplicate-even-when-values-agree", repeated, facts{0, 1, true, true, true, false, 6}},
		{"comma-syntax", dpInput("1,1", 1, 1), facts{1, 0, false, false, true, false, 7}},
		{"range-saturation-never-length-match", dpInput("9223372036854775808", dpMaxInt64, dpMaxInt64), facts{2, dpMaxInt64, true, false, true, false, 8}},
		{"plus-parses-with-noncanonical-spelling", dpInput("+1", 1, 1), facts{0, 1, false, true, true, false, 9}},
		{"minus-zero-parses-with-noncanonical-spelling", dpInput("-0", 0, 0), facts{0, 0, false, true, true, false, 9}},
		{"mismatch-before-budget", dpInput("2", 1, 0), facts{0, 2, true, false, false, false, 10}},
		{"matching-body-exceeds-limit", dpInput("2", 2, 1), facts{0, 2, true, true, false, false, 11}},
		{"zero-padding-retains-1024-original-bytes", dpInput(strings.Repeat("0", 1023)+"1", 1, 1), facts{0, 1, true, true, true, true, 0}},
		{"count-zero-with-nonempty-value", inconsistentMissing, facts{0, 1, true, false, true, false, 1}},
		{"negative-limit-is-policy-input", negativeLimit, facts{0, 1, true, true, false, false, 1}},
		{"negative-count-retains-syntax", negativeCount, facts{1, 0, false, false, true, false, 1}},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := payloadAdmissionReference(row.in)
			if err != nil || len(got) != 14 {
				t.Fatal("exact payload record", err, got)
			}
			arguments := payloadAdmissionArguments(row.in)
			for i, key := range payloadAdmissionInputNames {
				if !reflect.DeepEqual(got[key], arguments[i]) {
					t.Fatal("original typed input echo changed", key, got[key], arguments[i])
				}
			}
			want := map[string]any{"parse_error_kind": row.want.kind, "parsed_value": row.want.value, "unsigned_decimal": row.want.unsigned, "length_matches": row.want.matches, "within_budget": row.want.budget, "eligible": row.want.eligible, "reason": row.want.reason}
			for key, value := range want {
				if !reflect.DeepEqual(got[key], value) {
					t.Fatal("manual reason/derived fact or exact type differs", key, got[key], value)
				}
			}
		})
	}
}

func dpFixture(t *testing.T, schema string, rows []any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"schema": schema, "cases": rows})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func dpParserRow(name string, value any) any {
	return map[string]any{"name": name, "inputs": map[string]any{"text": value}}
}

func TestDecimalPayloadFixtureJSONPreservesUnicodeAndRejectsMalformedTransport(t *testing.T) {
	const prefix = `{"schema":"gooo/strconv-decimal-inputs/v1","cases":[{"name":"one","inputs":{"text":`
	const suffix = `}}]}`
	for _, token := range []string{`"\ud83d\ude42"`, `"\\ud800"`, `"�"`, `""`, `"\u0000"`, `"\"\\"`} {
		raw := []byte(prefix + token + suffix)
		rows, err := decimalPayloadFixtureRows(raw, "gooo/strconv-decimal-inputs/v1", 1)
		if err != nil {
			t.Fatal("valid escaped/literal Text refused", token, err)
		}
		if _, err := decimalFixtureInputs(rows); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range [][]byte{
		[]byte(prefix + `"\ud800"` + suffix), []byte(prefix + `"\udc00"` + suffix),
		[]byte(prefix + `"\ud800\u0041"` + suffix), []byte(prefix + `"\ud800\ud800"` + suffix),
		append(append([]byte(prefix+`"`), 0xff), []byte(`"`+suffix)...),
		[]byte(`{"schema":"gooo/strconv-decimal-inputs/v1","schema":"gooo/strconv-decimal-inputs/v1","cases":[]}`),
		[]byte(`{"schema":"gooo/strconv-decimal-inputs/v1","cases":[{"name":"one","name":"two","inputs":{"text":"1"}}]}`),
		[]byte(`{"schema":"gooo/strconv-decimal-inputs/v1","cases":[{"name":"one","inputs":{"text":"1","te\u0078t":"2"}}]}`),
		[]byte(prefix + `"0"` + suffix + `{}`),
		[]byte(`{"schema":"gooo/strconv-decimal-inputs/v1","cases":null}`),
		[]byte(`{"schema":"gooo/strconv-decimal-inputs/v1","cases":[],"unknown":true}`),
		[]byte(`{"schema":"wrong","cases":[]}`),
	} {
		if rows, err := decimalPayloadFixtureRows(raw, "gooo/strconv-decimal-inputs/v1", 1); err == nil || rows != nil {
			t.Fatal("malformed/duplicate/unknown fixture admitted", string(raw))
		}
	}
}

func TestDecimalPayloadFixtureKindsAndEntirePopulationAdmission(t *testing.T) {
	for _, value := range []any{nil, true, false, float64(1), int64(1), []any{}, map[string]any{}, string([]byte{0xff}), strings.Repeat("0", 1025)} {
		rows := []decimalPayloadFixtureRow{{Name: "valid", Inputs: map[string]any{"text": "0"}}, {Name: "late-invalid", Inputs: map[string]any{"text": value}}}
		if got, err := decimalFixtureInputs(rows); err == nil || got != nil {
			t.Fatal("late invalid parser row yielded partial admitted inputs", value)
		}
	}
	for _, rows := range [][]any{
		{dpParserRow("same", "0"), dpParserRow("same", "1")},
		{map[string]any{"name": "x", "inputs": map[string]any{"text": "0"}, "unknown": true}},
		{map[string]any{"name": "x"}}, {map[string]any{"name": "", "inputs": map[string]any{"text": "0"}}},
	} {
		if got, err := decimalPayloadFixtureRows(dpFixture(t, "gooo/strconv-decimal-inputs/v1", rows), "gooo/strconv-decimal-inputs/v1", len(rows)); err == nil || got != nil {
			t.Fatal("wrong row shape/name admitted")
		}
	}
	duplicateText := []decimalPayloadFixtureRow{{Name: "one", Inputs: map[string]any{"text": "0"}}, {Name: "two", Inputs: map[string]any{"text": "0"}}}
	if _, err := decimalFixtureInputs(duplicateText); err == nil {
		t.Fatal("duplicate original parser tuple admitted")
	}
	for _, value := range []any{nil, true, "1", float64(1), int64(1), json.Number("1.0"), json.Number("1e0"), json.Number("9223372036854775808"), json.Number("-9223372036854775809")} {
		if _, err := decimalPayloadInteger(value); err == nil {
			t.Fatal("non-int64 JSON token admitted", value)
		}
	}
	for text, want := range map[string]int64{"-9223372036854775808": dpMinInt64, "9223372036854775807": dpMaxInt64, "0": 0, "-0": 0} {
		if got, err := decimalPayloadInteger(json.Number(text)); err != nil || got != want {
			t.Fatal("legal exact integer rejected", text, got, err)
		}
	}
	base := map[string]any{"declared_length": "0", "content_length_occurrences": json.Number("1"), "observed_body_bytes": json.Number("0"), "maximum_payload_bytes": json.Number("0"), "transfer_encoding_present": false, "body_observation_bound_to_request": true, "current_access": true}
	if got, err := payloadFixtureInputs([]decimalPayloadFixtureRow{{Name: "valid", Inputs: base}}); err != nil || len(got) != 1 {
		t.Fatal("positive exact-kind base invalid", err)
	}
	for _, key := range payloadAdmissionInputNames {
		bad := map[string]any{}
		for k, v := range base {
			bad[k] = v
		}
		bad[key] = nil
		rows := []decimalPayloadFixtureRow{{Name: "valid", Inputs: base}, {Name: "late-invalid", Inputs: bad}}
		if got, err := payloadFixtureInputs(rows); err == nil || got != nil {
			t.Fatal("null input or partial roster admitted", key)
		}
		delete(bad, key)
		if _, err := payloadFixtureInputs([]decimalPayloadFixtureRow{{Name: "missing", Inputs: bad}}); err == nil {
			t.Fatal("missing input admitted", key)
		}
	}
	for _, value := range []any{json.Number("0"), json.Number("1"), "false", nil} {
		bad := map[string]any{}
		for k, v := range base {
			bad[k] = v
		}
		bad["current_access"] = value
		if _, err := payloadFixtureInputs([]decimalPayloadFixtureRow{{Name: "wrong-bool", Inputs: bad}}); err == nil {
			t.Fatal("non-Boolean fact admitted", value)
		}
	}
	if _, err := payloadFixtureInputs([]decimalPayloadFixtureRow{{Name: "one", Inputs: base}, {Name: "two", Inputs: base}}); err == nil {
		t.Fatal("duplicate complete context tuple admitted")
	}
}

func TestDecimalPayloadPublicFinitePopulationsAndManual17Prefix(t *testing.T) {
	got, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Suites) != 2 || got.DecimalReferenceCases != 256 || got.PayloadReferenceCases != 101 {
		t.Fatal("frozen two-suite population differs", got.DecimalReferenceCases, got.PayloadReferenceCases)
	}
	parser, policy := got.Suites[0], got.Suites[1]
	if parser.Package != decimalPackage || parser.Activity != decimalActivity || policy.Package != payloadAdmissionPackage || policy.Activity != payloadAdmissionActivity || !isDecimalPayloadSuite(parser) || !isDecimalPayloadSuite(policy) {
		t.Fatal("suite identity/order differs")
	}
	for _, s := range []suite{{Package: decimalPackage, Activity: "Other"}, {Package: "other", Activity: decimalActivity}, {Package: payloadAdmissionPackage, Activity: "Other"}} {
		if isDecimalPayloadSuite(s) {
			t.Fatal("unrelated suite opted into new corpus")
		}
	}
	parseKinds, reasons := map[int64]bool{}, map[int64]bool{}
	for _, row := range parser.Cases {
		if len(row.Inputs) != 1 {
			t.Fatal("unary original Text missing")
		}
		record, ok := row.Expected.(map[string]any)
		if !ok || len(record) != 4 {
			t.Fatal("parser record shape")
		}
		if !reflect.DeepEqual(record["input"], row.Inputs[0]) {
			t.Fatal("parser input echo")
		}
		if _, ok := record["value"].(int64); !ok {
			t.Fatal("parser Integer lost")
		}
		if _, ok := record["parsed"].(bool); !ok {
			t.Fatal("parser Boolean lost")
		}
		kind, ok := record["error_kind"].(int64)
		if !ok {
			t.Fatal("parser error Integer lost")
		}
		parseKinds[kind] = true
	}
	for _, row := range policy.Cases {
		if len(row.Inputs) != 7 {
			t.Fatal("original context tuple missing")
		}
		record, ok := row.Expected.(map[string]any)
		if !ok || len(record) != 14 {
			t.Fatal("payload record shape")
		}
		for i, key := range payloadAdmissionInputNames {
			if !reflect.DeepEqual(record[key], row.Inputs[i]) {
				t.Fatal("payload original echo changed", key)
			}
		}
		for _, key := range []string{"parse_error_kind", "parsed_value", "reason"} {
			if _, ok := record[key].(int64); !ok {
				t.Fatal("payload Integer lost", key)
			}
		}
		for _, key := range []string{"unsigned_decimal", "length_matches", "within_budget", "eligible"} {
			if _, ok := record[key].(bool); !ok {
				t.Fatal("payload Boolean lost", key)
			}
		}
		reasons[record["reason"].(int64)] = true
	}
	if !reflect.DeepEqual(parseKinds, map[int64]bool{0: true, 1: true, 2: true}) {
		t.Fatal("parser success/syntax/range coverage changed", parseKinds)
	}
	for reason := int64(0); reason <= 11; reason++ {
		if !reasons[reason] {
			t.Fatal("reachable policy reason missing", reason)
		}
	}
	if reasons[12] {
		t.Fatal("actual Go oracle must not invent unsupported parser facts")
	}
	raw, err := decimalPayloadReadFixture("../..", "examples/payload-admission/manual17.json", "fc54b8c739824dc74ab3850201c8bb3f448e8a06a8406630b02963c522eeda8b")
	if err != nil {
		t.Fatal(err)
	}
	manualRows, err := decimalPayloadFixtureRows(raw, "gooo/payload-admission-inputs/v1", 17)
	if err != nil {
		t.Fatal(err)
	}
	manual, err := payloadFixtureInputs(manualRows)
	if err != nil {
		t.Fatal(err)
	}
	for i, in := range manual {
		if !reflect.DeepEqual(policy.Cases[i].Inputs, payloadAdmissionArguments(in)) {
			t.Fatal("original17 prefix changed", i)
		}
	}
	last := manual[16]
	if manualRows[16].Name != "zero-occurrences-with-nonempty-value" || last.DeclaredLength != "1" || last.ContentLengthOccurrences != 0 || last.ObservedBodyBytes != 0 || last.MaximumPayloadBytes != 0 || last.TransferEncodingPresent || !last.BodyObservationBoundToRequest || !last.CurrentAccess {
		t.Fatal("count0/nonempty independent witness changed")
	}
}

func TestDecimalPayloadFixtureSourceHashAndRegularFileGuard(t *testing.T) {
	root := t.TempDir()
	name := "fixture.json"
	if err := os.WriteFile(filepath.Join(root, name), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := decimalPayloadReadFixture(root, name, decimalParserFixtureSHA); err == nil {
		t.Fatal("changed fixture bytes accepted")
	}
	if err := os.Symlink(name, filepath.Join(root, "alias.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := decimalPayloadReadFixture(root, "alias.json", decimalParserFixtureSHA); err == nil {
		t.Fatal("symlink fixture accepted")
	}
}
