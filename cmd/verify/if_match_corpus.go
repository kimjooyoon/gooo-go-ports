package main

import (
	"fmt"
	"reflect"
	"unicode/utf8"
)

const ifMatchCorpusFile = "ports/net/http/if-match-corpus.json"
const ifMatchCorpusSHA = "8569d7ddfa9b9cda385d3f648d9bad9ef514705a9353c5773a0a64182e774581"
const ifMatchCases = 284

type ifMatchInputRow struct {
	ID, Category string
	Inputs       [2]string
}

// This input-only parser shares the existing strict fixture JSON path. It does
// not trim/repair, remove repeated occurrences or manufacture expected outputs.
func ifMatchInputs(raw []byte) ([]ifMatchInputRow, error) {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return nil, fmt.Errorf("IfMatch bounded input corpus required")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, err
	}
	value, err := decode(raw)
	if err != nil {
		return nil, err
	}
	top, err := decimalPayloadObject(value, "schema", "input_order", "rows")
	if err != nil || top["schema"] != "gooo/if-match-input-corpus/v1" || !reflect.DeepEqual(top["input_order"], []any{"selectedIfMatchHeader", "currentETag"}) {
		return nil, fmt.Errorf("IfMatch exact schema/operand order")
	}
	values, ok := top["rows"].([]any)
	if !ok || len(values) != ifMatchCases {
		return nil, fmt.Errorf("IfMatch full284 ordered occurrences required")
	}
	prefix := []string{"upstream-ifmatch_matches", "upstream-ifmatch_star", "upstream-ifmatch_failed", "upstream-ifmatch_fails_on_weak_etag"}
	rows := make([]ifMatchInputRow, 0, ifMatchCases)
	seen := map[string]bool{}
	for at, value := range values {
		object, err := decimalPayloadObject(value, "id", "inputs", "category")
		if err != nil {
			return nil, err
		}
		id, idOK := object["id"].(string)
		category, catOK := object["category"].(string)
		if !idOK || id == "" || len(id) > 128 || !utf8.ValidString(id) || seen[id] || !catOK || category == "" || !utf8.ValidString(category) || at < 4 && id != prefix[at] {
			return nil, fmt.Errorf("IfMatch distinct occurrence IDs/original prefix/category at%d", at)
		}
		inputs, ok := object["inputs"].([]any)
		if !ok || len(inputs) != 2 {
			return nil, fmt.Errorf("IfMatch exact two Text operands at%d", at)
		}
		row := ifMatchInputRow{ID: id, Category: category}
		for port, value := range inputs {
			text, err := decimalPayloadText(value)
			if err != nil {
				return nil, fmt.Errorf("IfMatch row%d/input%d: %w", at, port, err)
			}
			row.Inputs[port] = text
		}
		seen[id] = true
		rows = append(rows, row)
	}
	return rows, nil
}

func ifMatchCorpus(root string) ([]suite, error) {
	raw, err := decimalPayloadReadFixture(root, ifMatchCorpusFile, ifMatchCorpusSHA)
	if err != nil {
		return nil, err
	}
	rows, err := ifMatchInputs(raw)
	if err != nil {
		return nil, err
	}
	s := suite{Package: ifMatchPackage, Activity: ifMatchActivity, Domain: ifMatchScope}
	for _, row := range rows {
		s.Cases = append(s.Cases, testCase{Inputs: []any{row.Inputs[0], row.Inputs[1]}, Expected: ifMatchReference(row.Inputs[0], row.Inputs[1])})
	}
	return []suite{s}, nil
}
