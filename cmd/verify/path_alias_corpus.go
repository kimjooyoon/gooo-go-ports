package main

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
)

const pathCleanPackage = "go/path"
const pathCleanActivity = "Clean"
const pathAliasPackage = "app/path-alias-facts"
const pathAliasActivity = "Report"
const pathAliasResultEntity = "gooo://go-ports/examples/path-alias-facts/result/v1"
const pathCleanExpectedCases = 9841
const pathAliasExpectedCases = 120
const pathCleanFixture = "ports/path/clean-corpus-9841.json"
const pathAliasFixture = "examples/path-alias-facts/corpus-120.json"
const pathAliasAnchorFixture = "examples/path-alias-facts/anchor-pairs-24.json"
const pathCleanFixtureSHA = "7ba203221673fe804c20d6f68e054fb909c528f50490e4b23aac4c395636b033"
const pathAliasFixtureSHA = "31999e8ce4fa75665f21180dd5cbb2ede3bf51179fefb1fc23c2eba59610029e"
const pathAliasAnchorSHA = "f935a4caa43e304e7826b0b66a210bee4afc27352863940b74bd21a88d3e4c05"

type pathAliasPair struct{ Name, Left, Right string }

func isPathAliasSuite(s suite) bool {
	return s.Package == pathCleanPackage && s.Activity == pathCleanActivity || s.Package == pathAliasPackage && s.Activity == pathAliasActivity
}

// Reuse the existing strict raw-Unicode/duplicate/trailing JSON guards. These
// validate input fixtures, not arbitrary native CLI transport or path safety.
func pathCleanInputs(raw []byte) ([]string, error) {
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, err
	}
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	top, err := decimalPayloadObject(v, "schema", "alphabet", "maximum_length", "order", "texts")
	if err != nil {
		return nil, err
	}
	if top["schema"] != "gooo/path-clean-inputs/v1" || top["order"] != "length ascending, then Cartesian product in declared alphabet order" || top["maximum_length"] != json.Number("8") || !decimalPayloadSame(top["alphabet"], []string{".", "/", "a"}) {
		return nil, fmt.Errorf("path Clean exact finite alphabet/order/length required")
	}
	values, ok := top["texts"].([]any)
	if !ok || len(values) != pathCleanExpectedCases {
		return nil, fmt.Errorf("path Clean requires9841 input Texts")
	}
	inputs := make([]string, 0, pathCleanExpectedCases)
	frontier := []string{""}
	for size := 0; size <= 8; size++ {
		for _, expectedInput := range frontier {
			text, err := decimalPayloadText(values[len(inputs)])
			if err != nil || text != expectedInput {
				return nil, fmt.Errorf("path Clean original shortlex input differs at%d", len(inputs))
			}
			inputs = append(inputs, text)
		}
		if size < 8 {
			next := make([]string, 0, 3*len(frontier))
			for _, prefix := range frontier {
				for _, suffix := range []string{".", "/", "a"} {
					next = append(next, prefix+suffix)
				}
			}
			frontier = next
		}
	}
	return inputs, nil
}

func pathAliasInputs(raw []byte, count int) ([]pathAliasPair, error) {
	rows, err := decimalPayloadFixtureRows(raw, "gooo/path-alias-inputs/v1", count)
	if err != nil {
		return nil, err
	}
	if count < 1 || count > pathAliasExpectedCases {
		return nil, fmt.Errorf("bounded path pair count required")
	}
	inputs := make([]pathAliasPair, 0, count)
	seen := map[[2]string]bool{}
	for _, row := range rows {
		object, err := decimalPayloadObject(row.Inputs, "left", "right")
		if err != nil {
			return nil, err
		}
		left, err := decimalPayloadText(object["left"])
		if err != nil {
			return nil, err
		}
		right, err := decimalPayloadText(object["right"])
		if err != nil {
			return nil, err
		}
		key := [2]string{left, right}
		if seen[key] {
			return nil, fmt.Errorf("duplicate original path pair")
		}
		seen[key] = true
		inputs = append(inputs, pathAliasPair{row.Name, left, right})
	}
	return inputs, nil
}

// Only the actual pinned standard-library APIs compute clean/absolute facts.
// Other fields are this original composition's relations on those Go results.
func pathAliasReference(left, right string) (map[string]any, error) {
	if _, err := decimalPayloadText(left); err != nil {
		return nil, err
	}
	if _, err := decimalPayloadText(right); err != nil {
		return nil, err
	}
	lc, rc := path.Clean(left), path.Clean(right)
	exact, same := left == right, lc == rc
	return map[string]any{
		"left": left, "right": right, "left_clean": lc, "right_clean": rc,
		"left_is_abs": path.IsAbs(left), "right_is_abs": path.IsAbs(right),
		"left_changed": lc != left, "right_changed": rc != right,
		"exact_same": exact, "same_clean": same, "lexical_alias": same && !exact,
	}, nil
}

func pathAliasCorpus(root string) ([]suite, error) {
	cleanRaw, err := decimalPayloadReadFixture(root, pathCleanFixture, pathCleanFixtureSHA)
	if err != nil {
		return nil, err
	}
	pairRaw, err := decimalPayloadReadFixture(root, pathAliasFixture, pathAliasFixtureSHA)
	if err != nil {
		return nil, err
	}
	anchorRaw, err := decimalPayloadReadFixture(root, pathAliasAnchorFixture, pathAliasAnchorSHA)
	if err != nil {
		return nil, err
	}
	texts, err := pathCleanInputs(cleanRaw)
	if err != nil {
		return nil, err
	}
	pairs, err := pathAliasInputs(pairRaw, pathAliasExpectedCases)
	if err != nil {
		return nil, err
	}
	anchors, err := pathAliasInputs(anchorRaw, 24)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(pairs[:24], anchors) {
		return nil, fmt.Errorf("original24 path pair prefix changed")
	}
	for i := 0; i < 32; i++ {
		s := texts[(47+307*i)%pathCleanExpectedCases]
		for j, right := range []string{s + "/.", s + "/a/..", "./" + s} {
			row := pairs[24+3*i+j]
			if row.Left != s || row.Right != right {
				return nil, fmt.Errorf("input-only path pair derivation changed")
			}
		}
	}
	// All three complete input populations are validated before the first oracle
	// call. No retained output, private proof path or candidate formula is read.
	clean := suite{Package: pathCleanPackage, Activity: pathCleanActivity, Domain: "Finite9841 unique shortlex Texts over ./a, length0..8, actual pinned Go path.Clean. Adaptation accepts decoded validUTF8 Text<=1024; this corpus is not the whole domain, arbitrary bytes, filepath, URL decoding or filesystem/authorization behavior."}
	report := suite{Package: pathAliasPackage, Activity: pathAliasActivity, Domain: "Original11 typed lexical alias facts for120 fixed original Text pairs, original24 anchors preserved. Actual path.Clean/IsAbs expectations retain both raw inputs. No router/filesystem/application identity, security, authorization, effects or fullUTF8domain claim."}
	for _, text := range texts {
		clean.Cases = append(clean.Cases, testCase{Inputs: []any{text}, Expected: path.Clean(text)})
	}
	for _, row := range pairs {
		expected, err := pathAliasReference(row.Left, row.Right)
		if err != nil {
			return nil, err
		}
		report.Cases = append(report.Cases, testCase{Inputs: []any{row.Left, row.Right}, Expected: expected})
	}
	return []suite{clean, report}, nil
}
