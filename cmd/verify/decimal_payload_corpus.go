package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const decimalPackage = "go/strconv"
const decimalActivity = "ParseIntDecimal64"
const decimalResultEntity = "gooo://go-ports/strconv/decimal-int64-result/v1"
const payloadAdmissionPackage = "app/payload-admission"
const payloadAdmissionActivity = "Report"
const payloadAdmissionResultEntity = "gooo://go-ports/examples/payload-admission/facts/v1"
const decimalParserFixture = "ports/strconv/corpus-256.json"
const payloadAdmissionFixture = "examples/payload-admission/corpus.json"
const decimalParserFixtureSHA = "27a2a638f7de7d9b23c69539234e7bc6c61a8c628dca12c65dda46bb8d6b9c99"
const payloadAdmissionFixtureSHA = "a2573a1a3e2ed19ad3f46b8246d9be510f74568878c786d2165e8489f46c74a8"
const decimalParserExpectedCases = 256
const payloadAdmissionExpectedCases = 101

var payloadAdmissionInputNames = []string{"declared_length", "content_length_occurrences", "observed_body_bytes", "maximum_payload_bytes", "transfer_encoding_present", "body_observation_bound_to_request", "current_access"}

type decimalPayloadObservations struct {
	Suites                                       []suite
	DecimalReferenceCases, PayloadReferenceCases int
}

type decimalPayloadFixtureRow struct {
	Name   string
	Inputs map[string]any
}

type decimalInt64Facts struct {
	Input     string
	Value     int64
	Parsed    bool
	ErrorKind int64
}

type payloadAdmissionInput struct {
	DeclaredLength                                                        string
	ContentLengthOccurrences, ObservedBodyBytes, MaximumPayloadBytes      int64
	TransferEncodingPresent, BodyObservationBoundToRequest, CurrentAccess bool
}

func isDecimalPayloadSuite(s suite) bool {
	return s.Package == decimalPackage && s.Activity == decimalActivity || s.Package == payloadAdmissionPackage && s.Activity == payloadAdmissionActivity
}

// These are input-fixture guards, not claims about the compiler's native JSON
// transport. In particular, encoding/json must not silently replace a fixture's
// malformed UTF-8 or unpaired surrogate before its Text enters the Go oracle.
func decimalPayloadJSON(raw []byte) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return fmt.Errorf("decimal/payload fixture requires valid UTF-8 JSON")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] != '\\' {
				continue
			}
			i++
			if raw[i] != 'u' {
				continue
			}
			unit, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
			if err != nil {
				return err
			}
			i += 4
			if unit >= 0xdc00 && unit <= 0xdfff {
				return fmt.Errorf("unpaired fixture surrogate")
			}
			if unit >= 0xd800 && unit <= 0xdbff {
				if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
					return fmt.Errorf("unpaired fixture surrogate")
				}
				low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("unpaired fixture surrogate")
				}
				i += 6
			}
		}
	}
	return nil
}

func decimalPayloadObject(value any, names ...string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != len(names) {
		return nil, fmt.Errorf("exact decimal/payload fixture object required")
	}
	for _, name := range names {
		if _, exists := object[name]; !exists {
			return nil, fmt.Errorf("missing decimal/payload field %s", name)
		}
	}
	return object, nil
}

func decimalPayloadFixtureRows(raw []byte, schema string, count int) ([]decimalPayloadFixtureRow, error) {
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, err
	}
	decoded, err := decode(raw)
	if err != nil {
		return nil, err
	}
	top, err := decimalPayloadObject(decoded, "schema", "cases")
	if err != nil {
		return nil, err
	}
	if value, ok := top["schema"].(string); !ok || value != schema {
		return nil, fmt.Errorf("decimal/payload fixture schema differs")
	}
	cases, ok := top["cases"].([]any)
	if !ok || count < 1 || len(cases) != count {
		return nil, fmt.Errorf("decimal/payload fixture population differs")
	}
	rows := make([]decimalPayloadFixtureRow, 0, count)
	seen := map[string]bool{}
	for _, value := range cases {
		row, err := decimalPayloadObject(value, "name", "inputs")
		if err != nil {
			return nil, err
		}
		name, ok := row["name"].(string)
		if !ok || name == "" || seen[name] {
			return nil, fmt.Errorf("distinct decimal/payload fixture names required")
		}
		inputs, ok := row["inputs"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("fixture inputs object required")
		}
		rows = append(rows, decimalPayloadFixtureRow{Name: name, Inputs: inputs})
		seen[name] = true
	}
	return rows, nil
}

func decimalPayloadText(value any) (string, error) {
	text, ok := value.(string)
	if !ok || !utf8.ValidString(text) || len(text) > 1024 {
		return "", fmt.Errorf("decoded valid UTF-8 Text<=1024 required")
	}
	return text, nil
}

func decimalPayloadInteger(value any) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("signed64 JSON integer required")
	}
	result, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("signed64 JSON integer required: %w", err)
	}
	return result, nil
}

func decimalFixtureInputs(rows []decimalPayloadFixtureRow) ([]string, error) {
	inputs := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		object, err := decimalPayloadObject(row.Inputs, "text")
		if err != nil {
			return nil, err
		}
		text, err := decimalPayloadText(object["text"])
		if err != nil {
			return nil, err
		}
		if seen[text] {
			return nil, fmt.Errorf("duplicate decimal fixture input")
		}
		seen[text] = true
		inputs = append(inputs, text)
	}
	return inputs, nil
}

func payloadFixtureInputs(rows []decimalPayloadFixtureRow) ([]payloadAdmissionInput, error) {
	inputs := make([]payloadAdmissionInput, 0, len(rows))
	seen := map[payloadAdmissionInput]bool{}
	for _, row := range rows {
		object, err := decimalPayloadObject(row.Inputs, payloadAdmissionInputNames...)
		if err != nil {
			return nil, err
		}
		var in payloadAdmissionInput
		in.DeclaredLength, err = decimalPayloadText(object["declared_length"])
		if err != nil {
			return nil, err
		}
		for _, target := range []struct {
			name  string
			value *int64
		}{
			{"content_length_occurrences", &in.ContentLengthOccurrences}, {"observed_body_bytes", &in.ObservedBodyBytes}, {"maximum_payload_bytes", &in.MaximumPayloadBytes},
		} {
			*target.value, err = decimalPayloadInteger(object[target.name])
			if err != nil {
				return nil, err
			}
		}
		for _, target := range []struct {
			name  string
			value *bool
		}{
			{"transfer_encoding_present", &in.TransferEncodingPresent}, {"body_observation_bound_to_request", &in.BodyObservationBoundToRequest}, {"current_access", &in.CurrentAccess},
		} {
			value, ok := object[target.name].(bool)
			if !ok {
				return nil, fmt.Errorf("exact Boolean field %s required", target.name)
			}
			*target.value = value
		}
		if seen[in] {
			return nil, fmt.Errorf("duplicate payload fixture tuple")
		}
		seen[in] = true
		inputs = append(inputs, in)
	}
	return inputs, nil
}

// Expected parser values come only from the actual original Go API. Preserve
// its returned int64 even on ErrRange; no candidate conversion/cutoff is copied.
func decimalInt64Reference(text string) (decimalInt64Facts, error) {
	if _, err := decimalPayloadText(text); err != nil {
		return decimalInt64Facts{}, err
	}
	value, err := strconv.ParseInt(text, 10, 64)
	facts := decimalInt64Facts{Input: text, Value: value, Parsed: err == nil}
	if err != nil {
		syntax, outOfRange := errors.Is(err, strconv.ErrSyntax), errors.Is(err, strconv.ErrRange)
		if syntax == outOfRange {
			return decimalInt64Facts{}, fmt.Errorf("unexpected ParseInt error")
		}
		if syntax {
			facts.ErrorKind = 1
		} else {
			facts.ErrorKind = 2
		}
	}
	return facts, nil
}

func decimalInt64Record(facts decimalInt64Facts) map[string]any {
	return map[string]any{"input": facts.Input, "value": facts.Value, "parsed": facts.Parsed, "error_kind": facts.ErrorKind}
}

func payloadAdmissionReference(in payloadAdmissionInput) (map[string]any, error) {
	parsed, err := decimalInt64Reference(in.DeclaredLength)
	if err != nil {
		return nil, err
	}
	metadataValid := in.ContentLengthOccurrences >= 0 && in.ObservedBodyBytes >= 0 && in.MaximumPayloadBytes >= 0 && !(in.ContentLengthOccurrences == 0 && in.DeclaredLength != "")
	parserConsistent := parsed.Input == in.DeclaredLength && parsed.ErrorKind >= 0 && parsed.ErrorKind <= 2 && parsed.Parsed == (parsed.ErrorKind == 0)
	unsigned := in.DeclaredLength != "" && strings.Trim(in.DeclaredLength, "0123456789") == ""
	matches := parsed.Parsed && parsed.ErrorKind == 0 && parsed.Value >= 0 && in.ObservedBodyBytes >= 0 && parsed.Value == in.ObservedBodyBytes
	budget := in.ObservedBodyBytes >= 0 && in.MaximumPayloadBytes >= 0 && in.ObservedBodyBytes <= in.MaximumPayloadBytes
	reason := int64(0)
	for _, refusal := range []struct {
		code    int64
		matches bool
	}{
		{1, !metadataValid}, {2, !in.CurrentAccess}, {3, !in.BodyObservationBoundToRequest}, {4, in.TransferEncodingPresent},
		{5, in.ContentLengthOccurrences == 0}, {6, in.ContentLengthOccurrences > 1}, {12, !parserConsistent},
		{7, parsed.ErrorKind == 1}, {8, parsed.ErrorKind == 2}, {9, !unsigned}, {10, !matches}, {11, !budget},
	} {
		if refusal.matches {
			reason = refusal.code
			break
		}
	}
	eligible := metadataValid && in.CurrentAccess && in.BodyObservationBoundToRequest && !in.TransferEncodingPresent && in.ContentLengthOccurrences == 1 && parserConsistent && parsed.Parsed && parsed.ErrorKind == 0 && unsigned && matches && budget
	if eligible != (reason == 0) {
		return nil, fmt.Errorf("payload reason/eligibility invariant")
	}
	return map[string]any{
		"declared_length": in.DeclaredLength, "content_length_occurrences": in.ContentLengthOccurrences, "observed_body_bytes": in.ObservedBodyBytes, "maximum_payload_bytes": in.MaximumPayloadBytes,
		"transfer_encoding_present": in.TransferEncodingPresent, "body_observation_bound_to_request": in.BodyObservationBoundToRequest, "current_access": in.CurrentAccess,
		"parse_error_kind": parsed.ErrorKind, "parsed_value": parsed.Value, "unsigned_decimal": unsigned, "length_matches": matches, "within_budget": budget, "eligible": eligible, "reason": reason,
	}, nil
}

func payloadAdmissionArguments(in payloadAdmissionInput) []any {
	return []any{in.DeclaredLength, in.ContentLengthOccurrences, in.ObservedBodyBytes, in.MaximumPayloadBytes, in.TransferEncodingPresent, in.BodyObservationBoundToRequest, in.CurrentAccess}
}

func decimalPayloadReadFixture(root, name, want string) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(name))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4<<20 {
		return nil, fmt.Errorf("bounded regular decimal/payload fixture required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if int64(len(raw)) != info.Size() || hex.EncodeToString(sum[:]) != want {
		return nil, fmt.Errorf("decimal/payload frozen fixture bytes differ: %s", name)
	}
	return raw, nil
}

func decimalPayloadCorpus(root string) (decimalPayloadObservations, error) {
	parserRaw, err := decimalPayloadReadFixture(root, decimalParserFixture, decimalParserFixtureSHA)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	payloadRaw, err := decimalPayloadReadFixture(root, payloadAdmissionFixture, payloadAdmissionFixtureSHA)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	parserRows, err := decimalPayloadFixtureRows(parserRaw, "gooo/strconv-decimal-inputs/v1", decimalParserExpectedCases)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	payloadRows, err := decimalPayloadFixtureRows(payloadRaw, "gooo/payload-admission-inputs/v1", payloadAdmissionExpectedCases)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	// Validate both complete populations before the first Go oracle call.
	texts, err := decimalFixtureInputs(parserRows)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	payloads, err := payloadFixtureInputs(payloadRows)
	if err != nil {
		return decimalPayloadObservations{}, err
	}
	parser := suite{Package: decimalPackage, Activity: decimalActivity, Domain: "Exact finite input-only256 decoded validUTF8 Text<=1024; original pinned Go strconv.ParseInt(s,10,64) returned int64, errors.Is syntax/range, originalText echo. No arbitrary-byte/unsupported>1024/whole strconv claim."}
	consumer := suite{Package: payloadAdmissionPackage, Activity: payloadAdmissionActivity, Domain: "Original pure14field/7input payload eligibility facts over the frozen finite context tuples; independent actual ParseInt and policy. Host observations/current access/binding are asserted facts. No HTTP framing, authorization enforcement, effects, security guarantee or perrequest human approval."}
	for _, text := range texts {
		facts, err := decimalInt64Reference(text)
		if err != nil {
			return decimalPayloadObservations{}, err
		}
		parser.Cases = append(parser.Cases, testCase{Inputs: []any{text}, Expected: decimalInt64Record(facts)})
	}
	for _, in := range payloads {
		expected, err := payloadAdmissionReference(in)
		if err != nil {
			return decimalPayloadObservations{}, err
		}
		consumer.Cases = append(consumer.Cases, testCase{Inputs: payloadAdmissionArguments(in), Expected: expected})
	}
	return decimalPayloadObservations{Suites: []suite{parser, consumer}, DecimalReferenceCases: len(parser.Cases), PayloadReferenceCases: len(consumer.Cases)}, nil
}
