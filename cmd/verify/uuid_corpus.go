// Copyright 2016 Google Inc. All rights reserved.
// Upstream TestValidate input examples use the BSD-style license retained in
// third_party/google-uuid/LICENSE. Other corpus construction is locally authored.
package main

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type uuidObservations struct {
	Suites                                     []suite
	PortReferenceCases, ConsumerReferenceCases int
}
type uuidNamedInput struct{ Name, Text string }

// The entire Go oracle domain is validated before any reference call. Text is
// original, valid UTF-8 and at most 1024 bytes; there is no normalization or trim.
func readUUIDInput(row []any) (string, error) {
	if len(row) != 1 {
		return "", fmt.Errorf("UUID validation requires exactly one Text input")
	}
	text, ok := row[0].(string)
	if !ok || !utf8.ValidString(text) || len(text) > 1024 {
		return "", fmt.Errorf("UUID validation requires valid UTF-8 Text of at most 1024 bytes")
	}
	return text, nil
}
func uuidReferenceSuite(pkg, activity string, inputs [][]any, oracle func(string) any) (suite, error) {
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty UUID reference corpus")
	}
	seen := map[string]bool{}
	for _, row := range inputs {
		text, err := readUUIDInput(row)
		if err != nil {
			return suite{}, err
		}
		if seen[text] {
			return suite{}, fmt.Errorf("duplicate UUID reference Text")
		}
		seen[text] = true
	}
	result := suite{Package: pkg, Activity: activity}
	for _, row := range inputs {
		text, _ := readUUIDInput(row)
		result.Cases = append(result.Cases, testCase{Inputs: []any{text}, Expected: oracle(text)})
	}
	return result, nil
}

// These call the original local google/uuid v1.6.0 module directly. Failed
// format and length results remain reference cases, not Gooo domain extensions.
func uuidValidationOracle(text string) any {
	err := uuid.Validate(text)
	message := ""
	if err != nil {
		message = err.Error()
	}
	return map[string]any{"valid": err == nil, "error": message, "invalid_length": uuid.IsInvalidLengthError(err)}
}
func uuidInspectorOracle(text string) any {
	result := uuidValidationOracle(text).(map[string]any)
	result["byte_length"] = int64(len(text))
	return result
}

// All nine original TestValidate inputs from google/uuid v1.6.0 uuid_test.go.
func uuidUpstreamInputs() []string {
	return []string{
		"123e4567-e89b-12d3-a456-426655440000",
		"urn:uuid:123e4567-e89b-12d3-a456-426655440000",
		"{123e4567-e89b-12d3-a456-426655440000}",
		"123e4567e89b12d3a456426655440000", "invalid-uuid", "123",
		"urn:test:123e4567-e89b-12d3-a456-426655440000",
		"[123e4567-e89b-12d3-a456-426655440000]",
		"12345678gabc1234abcd1234abcd1234",
	}
}

// Frozen original inputs of the private 40-anchor consumer pilot. No private
// file or old receipt is read at runtime and no old result is relabelled.
func uuidPilotInputs() []uuidNamedInput {
	return []uuidNamedInput{
		{"standard", "00112233-4455-6677-8899-aabbccddeeff"},
		{"standard-upper", "00112233-4455-6677-8899-AABBCCDDEEFF"},
		{"all-zero", "00000000-0000-0000-0000-000000000000"},
		{"all-f", "ffffffff-ffff-ffff-ffff-ffffffffffff"},
		{"braced", "{00112233-4455-6677-8899-aabbccddeeff}"},
		{"braced-upper", "{00112233-4455-6677-8899-AABBCCDDEEFF}"},
		{"urn", "urn:uuid:00112233-4455-6677-8899-aabbccddeeff"},
		{"urn-mixed", "URN:uUiD:00112233-4455-6677-8899-AABBCCDDEEFF"},
		{"raw", "00112233445566778899aabbccddeeff"},
		{"raw-upper", "00112233445566778899AABBCCDDEEFF"},
		{"wrong-opening-brace", "[00112233-4455-6677-8899-aabbccddeeff}"},
		{"wrong-closing-brace", "{00112233-4455-6677-8899-aabbccddeeff]"},
		{"bracket-priority-over-hex", "[g0112233-4455-6677-8899-aabbccddeeff}"},
		{"wrong-hyphen-8", "0011223304455-6677-8899-aabbccddeeff"},
		{"wrong-hyphen-13", "00112233-445506677-8899-aabbccddeeff"},
		{"wrong-hyphen-18", "00112233-4455-667708899-aabbccddeeff"},
		{"wrong-hyphen-23", "00112233-4455-6677-88990aabbccddeeff"},
		{"bad-standard-hex", "g0112233-4455-6677-8899-aabbccddeeff"},
		{"bad-raw-hex", "00112233445566778899aabbccddeefg"},
		{"bad-braced-hex", "{g0112233-4455-6677-8899-aabbccddeeff}"},
		{"NUL-raw", "00112233\u000045566778899aabbccddeeff"},
		{"prefix-priority-over-hex", "urn:guid:g0112233-4455-6677-8899-aabbccddeeff"},
		{"quoted-prefix", "u\"n:uuid:00112233-4455-6677-8899-aabbccddeeff"},
		{"backslash-prefix", "ur\\:uuid:00112233-4455-6677-8899-aabbccddeeff"},
		{"NUL-prefix", "u\u0000n:uuid:00112233-4455-6677-8899-aabbccddeeff"},
		{"all-short-escapes-prefix", "\"\\\u0007\b\f\n\r\t\u000b00112233-4455-6677-8899-aabbccddeeff"},
		{"printable-unicode-prefix", "\u00e9abcdefg00112233-4455-6677-8899-aabbccddeeff"},
		{"nonprint-unicode-prefix", "\u0085\u00ad\ufeffab00112233-4455-6677-8899-aabbccddeeff"},
		{"printable-emoji-prefix", "\U0001f642abcde00112233-4455-6677-8899-aabbccddeeff"},
		{"broken3-rune-prefix", "abcdefgh\uac00aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"broken4-rune-prefix2", "abcdefg\U0001f642aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"broken4-rune-prefix3", "abcdef\U0001f642aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"broken2-rune-prefix", "abcdefgh\u00e9aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-0", ""},
		{"invalid-length-31", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-33", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-37", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-44", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-1023", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid-length-1024", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
}

const uuidStandardSample = "00112233-4455-6677-8899-aabbccddeeff"

func uuidForms(standard string) []string {
	return []string{standard, "urn:uuid:" + standard, "{" + standard + "}", strings.ReplaceAll(standard, "-", "")}
}
func uuidURNCaseVariants() []string {
	var values []string
	positions := []int{0, 1, 2, 4, 5, 6, 7}
	for mask := 0; mask < 128; mask++ {
		prefix := []byte("urn:uuid:")
		for bit, offset := range positions {
			if mask&(1<<bit) != 0 {
				prefix[offset] -= 'a' - 'A'
			}
		}
		values = append(values, string(prefix)+uuidStandardSample)
	}
	return values
}
func uuidUnicodeInputs() []string {
	values := []string{"\u00e9abcdefg" + uuidStandardSample, "\U0001f642abcde" + uuidStandardSample,
		"\u0085\u00ad\ufeffab" + uuidStandardSample, "\"\\\a\b\f\n\r\t\v" + uuidStandardSample}
	// Prefix quoting sees only the first nine bytes. Each complete input stays
	// valid UTF-8 even where that nine-byte prefix ends inside a rune.
	for _, r := range []string{"\u00e9", "\uac00", "\U0001f642", "\u00a0", "\u0085", "\u00ad", "\u200b", "\ufeff", "\u2028", "\u2029", "\ufffd", "\u4e2d"} {
		for start := 0; start < 9; start++ {
			values = append(values, strings.Repeat("a", start)+r+strings.Repeat("a", 45-start-len(r)))
		}
	}
	return values
}
func uuidDeterministicInputs() []string {
	var values []string
	state := uint32(0x5a17b39d)
	next := func() uint32 { state = state*1664525 + 1013904223; return state }
	for sample := 0; sample < 256; sample++ {
		bytes := make([]byte, 16)
		for i := range bytes {
			bytes[i] = byte(next() >> 24)
		}
		raw := hex.EncodeToString(bytes)
		standard := raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
		if sample%2 != 0 {
			standard = strings.ToUpper(standard)
		}
		values = append(values, uuidForms(standard)...)
		// Bounded ASCII samples cover varied byte lengths and controls. Force
		// invalidity in accepted format lengths without converting the input.
		length := int(next() % 1025)
		text := make([]byte, length)
		for i := range text {
			text[i] = byte(next()>>24) & 127
		}
		if length > 0 {
			text[0] = '!'
		}
		values = append(values, string(text))
	}
	return values
}
func uuidCorpusInputs() [][]any {
	var rows [][]any
	seen := map[string]bool{}
	add := func(text string) {
		if !seen[text] {
			seen[text] = true
			rows = append(rows, []any{text})
		}
	}
	for _, text := range uuidUpstreamInputs() {
		add(text)
	}
	for _, input := range uuidPilotInputs() {
		add(input.Text)
	}
	for _, text := range uuidURNCaseVariants() {
		add(text)
	}
	for _, original := range uuidForms(uuidStandardSample) {
		for position := 0; position < len(original); position++ {
			for value := 0; value < 128; value++ {
				mutated := []byte(original)
				mutated[position] = byte(value)
				add(string(mutated))
			}
		}
	}
	for length := 0; length <= 1024; length++ {
		add(strings.Repeat("x", length))
	}
	for _, text := range uuidUnicodeInputs() {
		add(text)
	}
	for _, text := range uuidDeterministicInputs() {
		add(text)
	}
	return rows
}
func uuidCorpus() (uuidObservations, error) {
	inputs := uuidCorpusInputs()
	port, err := uuidReferenceSuite(uuidPackage, "Validate", inputs, uuidValidationOracle)
	if err != nil {
		return uuidObservations{}, err
	}
	consumer, err := uuidReferenceSuite(uuidConsumerPackage, "Inspect", inputs, uuidInspectorOracle)
	if err != nil {
		return uuidObservations{}, err
	}
	domain := fmt.Sprintf("%d finite original valid-UTF8 Text values<=1024bytes; original google/uuid v1.6.0 Validate+IsInvalidLengthError; all128 ASCIIURNcase variants, ASCII0..127 at every position of four valid forms, every byte length0..1024, quoted/broken-rune first-nine-byte prefixes and fixed deterministic samples; no arbitrary-Go-byte-string or exhaustive-string-domain claim", len(inputs))
	port.Domain, consumer.Domain = domain, domain+"; consumer preserves original byte length"
	return uuidObservations{Suites: []suite{port, consumer}, PortReferenceCases: len(port.Cases), ConsumerReferenceCases: len(consumer.Cases)}, nil
}
