package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUUIDReferenceValidatesWholeRosterBeforeCallingOracle(t *testing.T) {
	valid := uuidStandardSample
	bad := []struct {
		name string
		rows [][]any
	}{
		{"empty", nil}, {"missing", [][]any{{}}}, {"arity", [][]any{{valid, valid}}},
		{"integer", [][]any{{int64(36)}}}, {"bytes", [][]any{{[]byte(valid)}}},
		{"invalid UTF8", [][]any{{string([]byte{0xff})}}},
		{"oversize", [][]any{{strings.Repeat("a", 1025)}}},
		{"late invalid", [][]any{{valid}, {string([]byte{0xff})}}},
		{"late oversize", [][]any{{valid}, {strings.Repeat("a", 1025)}}},
		{"duplicate", [][]any{{valid}, {valid}}},
	}
	for _, row := range bad {
		t.Run(row.name, func(t *testing.T) {
			calls := 0
			_, err := uuidReferenceSuite(uuidPackage, "Validate", row.rows, func(string) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatalf("invalid roster reached oracle: error=%v calls=%d", err, calls)
			}
		})
	}
	calls := 0
	boundary := strings.Repeat("a", 1024)
	s, err := uuidReferenceSuite(uuidPackage, "Validate", [][]any{{boundary}}, func(text string) any {
		calls++
		if text != boundary {
			t.Fatal("input was narrowed")
		}
		return uuidValidationOracle(text)
	})
	if err != nil || calls != 1 || len(s.Cases) != 1 {
		t.Fatalf("boundary rejected: %v %d", err, calls)
	}
}

func TestUUIDOracleLiteralErrorsClassesAndPrecedence(t *testing.T) {
	for _, row := range []struct {
		text, message string
		length        bool
	}{
		{uuidStandardSample, "", false}, {strings.ToUpper(uuidStandardSample), "", false},
		{"00000000-0000-0000-0000-000000000000", "", false},
		{"", "invalid UUID length: 0", true}, {"123", "invalid UUID length: 3", true},
		{strings.Repeat("x", 1024), "invalid UUID length: 1024", true},
		{"[g0112233-4455-6677-8899-aabbccddeeff}", "invalid bracketed UUID format", false},
		{"urn:guid:g0112233-4455-6677-8899-aabbccddeeff", `invalid urn prefix: "urn:guid:"`, false},
		{"g0112233-4455-6677-8899-aabbccddeeff", "invalid UUID format", false},
		{"0011223304455-6677-8899-aabbccddeeff", "invalid UUID format", false},
		{"00112233445566778899aabbccddeefg", "invalid UUID format", false},
		{"abcdefgh\u00e9" + strings.Repeat("a", 35), `invalid urn prefix: "abcdefgh\xc3"`, false},
		{"abcdefgh\uac00" + strings.Repeat("a", 34), `invalid urn prefix: "abcdefgh\xea"`, false},
		{"abcdef\U0001f642" + strings.Repeat("a", 35), `invalid urn prefix: "abcdef\xf0\x9f\x99"`, false},
		{"u\x00n:uuid:" + uuidStandardSample, `invalid urn prefix: "u\x00n:uuid:"`, false},
		{"u\"n:uuid:" + uuidStandardSample, `invalid urn prefix: "u\"n:uuid:"`, false},
	} {
		want := map[string]any{"valid": row.message == "", "error": row.message, "invalid_length": row.length}
		if got := uuidValidationOracle(row.text); !reflect.DeepEqual(got, want) {
			t.Fatalf("literal %q: got %#v want %#v", row.text, got, want)
		}
		consumer := uuidInspectorOracle(row.text).(map[string]any)
		if consumer["byte_length"] != int64(len(row.text)) || len(consumer) != 4 {
			t.Fatal("original UTF8 byte length not retained")
		}
	}
}

func TestUUIDCorpusCoversEveryRequestedFiniteAxis(t *testing.T) {
	observed, err := uuidCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.Suites) != 2 || observed.PortReferenceCases != observed.ConsumerReferenceCases {
		t.Fatal("reference roster differs")
	}
	port, consumer := observed.Suites[0], observed.Suites[1]
	if port.Package != uuidPackage || port.Activity != "Validate" || consumer.Package != uuidConsumerPackage || consumer.Activity != "Inspect" {
		t.Fatal("suite entry mismatch")
	}
	seen := map[string]bool{}
	for i, row := range port.Cases {
		text, err := readUUIDInput(row.Inputs)
		if err != nil || !utf8.ValidString(text) || len(text) > 1024 || seen[text] {
			t.Fatalf("bad original input at %d: %v", i, err)
		}
		seen[text] = true
		if !reflect.DeepEqual(row.Inputs, consumer.Cases[i].Inputs) || !reflect.DeepEqual(row.Expected, uuidValidationOracle(text)) || !reflect.DeepEqual(consumer.Cases[i].Expected, uuidInspectorOracle(text)) {
			t.Fatal("oracle or input divergence")
		}
	}
	require := func(text string) {
		t.Helper()
		if !seen[text] {
			t.Fatalf("required finite input omitted: %q", text)
		}
	}
	if len(uuidUpstreamInputs()) != 9 || len(uuidPilotInputs()) != 40 {
		t.Fatal("original anchors changed")
	}
	for _, text := range uuidUpstreamInputs() {
		require(text)
	}
	for _, row := range uuidPilotInputs() {
		require(row.Text)
	}
	prefixes := map[string]bool{}
	for _, text := range uuidURNCaseVariants() {
		require(text)
		prefixes[text[:9]] = true
		if !uuidValidationOracle(text).(map[string]any)["valid"].(bool) {
			t.Fatal("ASCII case-insensitive URN rejected")
		}
	}
	if len(prefixes) != 128 {
		t.Fatal("all seven-letter ASCII URN case choices required")
	}
	for _, original := range uuidForms(uuidStandardSample) {
		for position := range original {
			for value := 0; value < 128; value++ {
				raw := []byte(original)
				raw[position] = byte(value)
				require(string(raw))
			}
		}
	}
	for length := 0; length <= 1024; length++ {
		require(strings.Repeat("x", length))
	}
	for _, text := range uuidUnicodeInputs() {
		require(text)
	}
	for _, text := range uuidDeterministicInputs() {
		require(text)
	}
	if len(port.Cases) < 20000 {
		t.Fatal("wide finite mutation roster unexpectedly narrowed")
	}
	second, err := uuidCorpus()
	if err != nil || !reflect.DeepEqual(observed, second) {
		t.Fatal("nondeterministic corpus")
	}
}

func TestUUIDActualSerializedBatchesRespectRowsAndBytes(t *testing.T) {
	observed, err := uuidCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range observed.Suites {
		batches := 0
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, 128)
			if err != nil || end <= start || end-start > 128 {
				t.Fatalf("batch bounds %d:%d: %v", start, end, err)
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			raw := mustJSONIndentedUUID(t, caseDocument(batch))
			if len(raw)+1 > 30000 {
				t.Fatalf("actual UUID JSON exceeds byte cap: %d", len(raw)+1)
			}
			start = end
			batches++
		}
		t.Logf("%s:%s %d reference cases, %d actual serialized batches", s.Package, s.Activity, len(s.Cases), batches)
	}
}

func mustJSONIndentedUUID(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
