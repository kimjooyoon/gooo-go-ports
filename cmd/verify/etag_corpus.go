package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"unicode/utf8"
)

const etagCorpusFile = "examples/strong-validator-facts/corpus.json"
const etagCorpusSHA = "ce9015cc12bc37e785761f4ee6907a77603082cd69b4d8edc80a634ff4478345"
const etagManualFile = "examples/strong-validator-facts/manual247.json"
const etagManualSHA = "850a4380fd3f4b35e3f4d58ea33609e6536ce464cb938645057b22abeb7d8ec4"
const etagScanCases = 192
const etagStrongCases = 31
const etagFactsCases = 24
const etagTotalCases = etagScanCases + etagStrongCases + etagFactsCases

// Every raw occurrence is retained, including the two repeated scanner inputs.
// Different activities' equal input strings are not interchangeable histories.
type etagInputRow struct {
	ID, Kind string
	Inputs   []string
}

type etagManualRow struct {
	etagInputRow
	Expected any
}

func etagKindAt(at int) string {
	if at < etagScanCases {
		return "scanner"
	}
	if at < etagScanCases+etagStrongCases {
		return "strong_match"
	}
	return "facts"
}

func etagOriginalScannerIDs() []string {
	return []string{"original-go-1-weak", "original-go-2-strong", "original-go-3-list-prefix", "original-go-4-empty", "original-go-5-weak-prefix-only", "original-go-6-unclosed-weak", "original-go-7-lowercase-weak", "original-go-8-interior-space"}
}

// This parser performs no Go oracle or native call. The existing JSON helper
// refuses invalid UTF8/unpaired surrogates; decode also refuses duplicate keys.
func etagFixture(raw []byte, manual bool) ([]etagInputRow, []any, error) {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return nil, nil, fmt.Errorf("ETag bounded fixture required")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, nil, err
	}
	value, err := decode(raw)
	if err != nil {
		return nil, nil, err
	}
	names, schema := []string{"schema", "cases"}, "gooo/etag-input-corpus/v1"
	if manual {
		names, schema = []string{"schema", "basis", "domain", "cases"}, "gooo/etag-manual-anchors/v1"
	}
	top, err := decimalPayloadObject(value, names...)
	if err != nil || top["schema"] != schema {
		return nil, nil, fmt.Errorf("ETag fixture schema/fields")
	}
	if manual {
		for _, name := range []string{"basis", "domain"} {
			text, ok := top[name].(string)
			if !ok || text == "" || !utf8.ValidString(text) {
				return nil, nil, fmt.Errorf("ETag anchor metadata")
			}
		}
	}
	cases, ok := top["cases"].([]any)
	if !ok || len(cases) != etagTotalCases {
		return nil, nil, fmt.Errorf("ETag full247 ordered occurrences required")
	}
	rows, expected := make([]etagInputRow, 0, etagTotalCases), make([]any, 0, etagTotalCases)
	ids := map[string]bool{}
	unique := map[string]map[string]bool{"scanner": {}, "strong_match": {}, "facts": {}}
	duplicates := map[string]int{}
	for at, value := range cases {
		fields := []string{"id", "kind", "inputs"}
		if manual {
			fields = append(fields, "expected", "purpose")
		}
		object, err := decimalPayloadObject(value, fields...)
		if err != nil {
			return nil, nil, err
		}
		id, idOK := object["id"].(string)
		kind, kindOK := object["kind"].(string)
		if !idOK || id == "" || len(id) > 128 || !utf8.ValidString(id) || !kindOK || kind != etagKindAt(at) || ids[kind+"\x00"+id] {
			return nil, nil, fmt.Errorf("ETag exact kind/order and distinct occurrence ID at%d", at)
		}
		if at < 8 && id != etagOriginalScannerIDs()[at] {
			return nil, nil, fmt.Errorf("original Go scanner8 prefix IDs changed")
		}
		args, ok := object["inputs"].([]any)
		arity := 1
		if kind == "strong_match" {
			arity = 2
		}
		if !ok || len(args) != arity {
			return nil, nil, fmt.Errorf("ETag exact Text arity at%d", at)
		}
		inputs := make([]string, arity)
		for port, value := range args {
			inputs[port], err = decimalPayloadText(value)
			if err != nil {
				return nil, nil, fmt.Errorf("ETag row%d/input%d: %w", at, port, err)
			}
		}
		tuple, err := json.Marshal(inputs)
		if err != nil {
			return nil, nil, err
		}
		if unique[kind][string(tuple)] {
			duplicates[kind]++
		}
		unique[kind][string(tuple)] = true
		ids[kind+"\x00"+id] = true
		rows = append(rows, etagInputRow{id, kind, inputs})
		if manual {
			purpose, ok := object["purpose"].(string)
			if !ok || purpose == "" || !utf8.ValidString(purpose) {
				return nil, nil, fmt.Errorf("ETag anchor purpose")
			}
			literal, err := etagLiteral(kind, object["expected"])
			if err != nil {
				return nil, nil, err
			}
			expected = append(expected, literal)
		}
	}
	if len(unique["scanner"]) != 190 || duplicates["scanner"] != 2 || len(unique["strong_match"]) != 31 || duplicates["strong_match"] != 0 || len(unique["facts"]) != 24 || duplicates["facts"] != 0 {
		return nil, nil, fmt.Errorf("ETag original190/31/24 unique input tuples and2/0/0 duplicate occurrences required")
	}
	return rows, expected, nil
}

func etagInputs(raw []byte) ([]etagInputRow, error) {
	rows, _, err := etagFixture(raw, false)
	return rows, err
}

func etagLiteral(kind string, value any) (any, error) {
	if kind == "strong_match" {
		result, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("ETag raw strong literal must be Boolean")
		}
		return result, nil
	}
	names := []string{"etag", "remain"}
	if kind == "facts" {
		names = etagFactsNames
	}
	object, err := decimalPayloadObject(value, names...)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for at, name := range names {
		if kind == "facts" && at >= 3 && at <= 8 {
			v, ok := object[name].(bool)
			if !ok {
				return nil, fmt.Errorf("ETag Boolean literal field %s", name)
			}
			out[name] = v
		} else if kind == "facts" && at >= 9 && at <= 11 {
			v, err := decimalPayloadInteger(object[name])
			if err != nil || v < 0 || v > 1024 {
				return nil, fmt.Errorf("ETag byte-count integer literal field %s", name)
			}
			out[name] = v
		} else {
			v, err := decimalPayloadText(object[name])
			if err != nil {
				return nil, fmt.Errorf("ETag Text literal field %s: %w", name, err)
			}
			out[name] = v
		}
	}
	return out, nil
}

func etagManualInputs(raw []byte) ([]etagManualRow, error) {
	rows, expected, err := etagFixture(raw, true)
	if err != nil {
		return nil, err
	}
	out := make([]etagManualRow, len(rows))
	for at, row := range rows {
		out[at] = etagManualRow{row, expected[at]}
	}
	return out, nil
}

// Literal anchors check the independent Go implementation, never substitute for
// its values. Compare all247 ordered inputs and727 concrete scalar fields.
func etagCheckManual(rows []etagInputRow, anchors []etagManualRow) error {
	if len(rows) != etagTotalCases || len(anchors) != etagTotalCases {
		return fmt.Errorf("ETag input/anchor population")
	}
	for at, anchor := range anchors {
		if !reflect.DeepEqual(rows[at], anchor.etagInputRow) {
			return fmt.Errorf("ETag ordered original input/ID differs at%d", at)
		}
		if !reflect.DeepEqual(etagReference(rows[at]), anchor.Expected) {
			return fmt.Errorf("independent Go/ETag literal disagreement at%d/%s", at, anchor.ID)
		}
	}
	return nil
}

func etagCorpus(root string) ([]suite, error) {
	raw, err := decimalPayloadReadFixture(root, etagCorpusFile, etagCorpusSHA)
	if err != nil {
		return nil, err
	}
	rows, err := etagInputs(raw)
	if err != nil {
		return nil, err
	}
	manualRaw, err := decimalPayloadReadFixture(root, etagManualFile, etagManualSHA)
	if err != nil {
		return nil, err
	}
	anchors, err := etagManualInputs(manualRaw)
	if err != nil {
		return nil, err
	}
	if err = etagCheckManual(rows, anchors); err != nil {
		return nil, err
	}
	suites := []suite{
		{Package: etagPortPackage, Activity: etagScanActivity, Domain: "Original Go scanETag: unchanged decoded validUTF8 Text0..1024bytes; exact etag/remain record, all192 ordered input occurrences including2 duplicates. No arbitrary-byte strings, list/fullIfMatch or effects."},
		{Package: etagPortPackage, Activity: etagStrongActivity, Domain: "Original raw etagStrongMatch on two independently bounded Text0..1024byte operands;31 ordered pairs, including grammar-invalid strings. No scanner/header normalization or fullIfMatch."},
		{Package: etagReportPackage, Activity: etagReportActivity, Domain: "Original13 local StrongValidatorFacts from original scan/rawheader/parsed-tag match;24 unchanged manual inputs. No approval needed to compute, no current-resource authority, write permission or effects."},
	}
	for _, row := range rows {
		at := 0
		if row.Kind == "strong_match" {
			at = 1
		} else if row.Kind == "facts" {
			at = 2
		}
		args := make([]any, len(row.Inputs))
		for port, text := range row.Inputs {
			args[port] = text
		}
		suites[at].Cases = append(suites[at].Cases, testCase{Inputs: args, Expected: etagReference(row)})
	}
	return suites, nil
}
