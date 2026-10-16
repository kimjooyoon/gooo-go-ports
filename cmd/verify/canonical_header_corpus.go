package main

import (
	"fmt"
	"net/textproto"
	"unicode/utf8"
)

const canonicalHeaderCorpusFile = "examples/canonical-header-key/corpus.json"
const canonicalHeaderCorpusSHA = "e5deb0c69d9b7ae99cfb6ed91ed6a57449d616e45cb04fb38f5ed3dbbe128aa1"
const canonicalHeaderManualFile = "examples/canonical-header-key/manual20.json"
const canonicalHeaderManualSHA = "753b8ed8beda42912a1fef7c376482189eb92652a8a9e0d7bda806c1bd6ffb6e"
const canonicalHeaderRawCases = 1059
const canonicalHeaderUniqueKeys = 1054

type canonicalHeaderInputRow struct{ ID, Key string }
type canonicalHeaderManualRow struct{ ID, Key, Expected string }

type canonicalHeaderPartition struct {
	Name             string `json:"name"`
	StartInclusive   int    `json:"start_inclusive"`
	EndExclusive     int    `json:"end_exclusive"`
	RawRows          int    `json:"raw_rows"`
	UniqueKeys       int    `json:"unique_keys"`
	WithinDuplicates int    `json:"within_partition_duplicate_occurrences"`
	CrossDuplicates  int    `json:"cross_partition_duplicate_occurrences"`
}

type canonicalHeaderInputPopulation struct {
	Rows                                          []canonicalHeaderInputRow
	Partitions                                    []canonicalHeaderPartition
	UniqueKeys, WithinDuplicates, CrossDuplicates int
}

func canonicalHeaderManualIDs() []string {
	return []string{"empty", "canonical", "all-lower", "all-upper", "mixed", "dash-runs", "token-punctuation", "punctuation-first", "digit-first", "punctuation-after-dash", "token-non-dash-separators", "invalid-colon-suffix", "invalid-space", "invalid-tab", "invalid-control", "invalid-nul", "invalid-crlf", "invalid-nonascii", "edge1024-token", "edge1024-unicode-invalid"}
}

// These half-open spans identify every raw occurrence, including repeated keys.
// They describe the frozen input corpus, not native batch boundaries.
func canonicalHeaderPartitions() []canonicalHeaderPartition {
	names := []string{"manual", "ascii-single", "ascii-initial", "ascii-middle-lower", "ascii-middle-dash", "ascii-terminal-lower", "ascii-terminal-dash", "named", "unicode-boundaries", "byte-length-edges", "seed-token", "seed-ascii", "seed-unicode"}
	counts := []int{20, 128, 128, 128, 128, 128, 128, 33, 36, 10, 64, 64, 64}
	var result []canonicalHeaderPartition
	start := 0
	for at, name := range names {
		result = append(result, canonicalHeaderPartition{Name: name, StartInclusive: start, EndExclusive: start + counts[at], RawRows: counts[at]})
		start += counts[at]
	}
	return result
}

func canonicalHeaderOccurrenceID(part canonicalHeaderPartition, index int) string {
	if part.Name == "manual" {
		return canonicalHeaderManualIDs()[index]
	}
	if part.Name == "named" {
		return fmt.Sprintf("named/%02d", index)
	}
	return fmt.Sprintf("%s/%03d", part.Name, index)
}

// Decode and validate the whole finite population before any Go oracle call.
// Raw fixture SHA fixes values/order; IDs and accounting also reject accidental
// deduplication, padding or a partial population. Duplicate key strings remain.
func canonicalHeaderInputs(raw []byte) (canonicalHeaderInputPopulation, error) {
	empty := canonicalHeaderInputPopulation{}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return empty, fmt.Errorf("canonical header input byte bound")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return empty, err
	}
	v, err := decode(raw)
	if err != nil {
		return empty, err
	}
	top, err := decimalPayloadObject(v, "schema", "cases")
	if err != nil || top["schema"] != "gooo/canonical-header-key-corpus/v1" {
		return empty, fmt.Errorf("canonical header corpus schema/fields")
	}
	rows, ok := top["cases"].([]any)
	if !ok || len(rows) != canonicalHeaderRawCases {
		return empty, fmt.Errorf("canonical header full1059 occurrences required")
	}
	out := canonicalHeaderInputPopulation{Partitions: canonicalHeaderPartitions()}
	ids, global := map[string]bool{}, map[string]bool{}
	for at := range out.Partitions {
		part := &out.Partitions[at]
		local := map[string]bool{}
		for index := part.StartInclusive; index < part.EndExclusive; index++ {
			object, err := decimalPayloadObject(rows[index], "id", "key")
			if err != nil {
				return empty, err
			}
			id, ok := object["id"].(string)
			if !ok || id == "" || len(id) > 128 || !utf8.ValidString(id) || ids[id] || id != canonicalHeaderOccurrenceID(*part, index-part.StartInclusive) {
				return empty, fmt.Errorf("canonical header exact ordered occurrence ID at%d", index)
			}
			key, err := decimalPayloadText(object["key"])
			if err != nil {
				return empty, err
			}
			ids[id] = true
			if local[key] {
				part.WithinDuplicates++
			} else if global[key] {
				part.CrossDuplicates++
			}
			local[key], global[key] = true, true
			out.Rows = append(out.Rows, canonicalHeaderInputRow{id, key})
		}
		part.UniqueKeys = len(local)
		wantCross := []int{0, 0, 0, 0, 0, 0, 0, 2, 1, 0, 0, 0, 2}
		if part.WithinDuplicates != 0 || part.CrossDuplicates != wantCross[at] {
			return empty, fmt.Errorf("canonical header partition duplicate accounting: %s", part.Name)
		}
		out.WithinDuplicates += part.WithinDuplicates
		out.CrossDuplicates += part.CrossDuplicates
	}
	out.UniqueKeys = len(global)
	if out.UniqueKeys != canonicalHeaderUniqueKeys || out.WithinDuplicates != 0 || out.CrossDuplicates != 5 || len(out.Rows)-out.UniqueKeys != out.WithinDuplicates+out.CrossDuplicates {
		return empty, fmt.Errorf("canonical header exact1054 unique/zero within/five cross duplicates required")
	}
	return out, nil
}

func canonicalHeaderManualInputs(raw []byte) ([]canonicalHeaderManualRow, error) {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return nil, fmt.Errorf("manual20 byte bound")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return nil, err
	}
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	top, err := decimalPayloadObject(v, "schema", "basis", "domain", "cases")
	if err != nil || top["schema"] != "gooo/canonical-header-key-manual/v1" {
		return nil, fmt.Errorf("canonical manual20 schema/fields")
	}
	for _, name := range []string{"basis", "domain"} {
		value, ok := top[name].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("canonical manual metadata")
		}
	}
	rows, ok := top["cases"].([]any)
	if !ok || len(rows) != 20 {
		return nil, fmt.Errorf("exact20 fixed manual anchors required")
	}
	var out []canonicalHeaderManualRow
	for at, value := range rows {
		row, err := decimalPayloadObject(value, "id", "key", "expected", "purpose")
		if err != nil {
			return nil, err
		}
		id, ok := row["id"].(string)
		if !ok || id != canonicalHeaderManualIDs()[at] {
			return nil, fmt.Errorf("exact ordered manual ID")
		}
		key, err := decimalPayloadText(row["key"])
		if err != nil {
			return nil, err
		}
		expected, err := decimalPayloadText(row["expected"])
		if err != nil {
			return nil, err
		}
		purpose, ok := row["purpose"].(string)
		if !ok || purpose == "" {
			return nil, fmt.Errorf("manual purpose required")
		}
		out = append(out, canonicalHeaderManualRow{id, key, expected})
	}
	return out, nil
}

func canonicalHeaderCheckManual(in canonicalHeaderInputPopulation, anchors []canonicalHeaderManualRow) error {
	if len(in.Rows) != canonicalHeaderRawCases || len(anchors) != 20 {
		return fmt.Errorf("manual/corpus populations")
	}
	for at, row := range anchors {
		if in.Rows[at].ID != row.ID || in.Rows[at].Key != row.Key {
			return fmt.Errorf("original manual20 prefix changed")
		}
	}
	for _, row := range anchors {
		if textproto.CanonicalMIMEHeaderKey(row.Key) != row.Expected {
			return fmt.Errorf("independent Go/manual canonical disagreement: %s", row.ID)
		}
	}
	return nil
}

// Only canonical_key is the original textproto API return. The other fields
// are original diagnostic composition, using the existing independent x/net
// header oracle for validity/witnesses. No candidate or saved actual supplies it.
func canonicalHeaderReference(key string) map[string]any {
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	header := headerReference(key, "")
	return map[string]any{"key": key, "canonical_key": canonical, "changed": canonical != key, "name_valid": header["name_valid"], "input_bytes": int64(len(key)), "canonical_bytes": int64(len(canonical)), "name_invalid_offset": header["name_invalid_offset"], "name_invalid_octet": header["name_invalid_octet"]}
}

func canonicalHeaderCorpus(root string) ([]suite, error) {
	raw, err := decimalPayloadReadFixture(root, canonicalHeaderCorpusFile, canonicalHeaderCorpusSHA)
	if err != nil {
		return nil, err
	}
	in, err := canonicalHeaderInputs(raw)
	if err != nil {
		return nil, err
	}
	manualRaw, err := decimalPayloadReadFixture(root, canonicalHeaderManualFile, canonicalHeaderManualSHA)
	if err != nil {
		return nil, err
	}
	anchors, err := canonicalHeaderManualInputs(manualRaw)
	if err != nil {
		return nil, err
	}
	if err := canonicalHeaderCheckManual(in, anchors); err != nil {
		return nil, err
	}
	scalar := suite{Package: canonicalHeaderPackage, Activity: canonicalHeaderActivity, Domain: "Decoded validUTF8 Text0..1024bytes including invalid header names; actual Go CanonicalMIMEHeaderKey content only. Every1059 original occurrence retained; no arbitrary-byte/unbounded Go string, normalization or performance claim."}
	report := suite{Package: canonicalHeaderReportPackage, Activity: canonicalHeaderReportActivity, Domain: "Original8 diagnostics retain exact key/canonical Text, changed/validity, UTF8 byte lengths and first invalid offset/octet. No per-call approval needed to compute; no HTTP parsing, authorization or effect."}
	for _, row := range in.Rows {
		scalar.Cases = append(scalar.Cases, testCase{Inputs: []any{row.Key}, Expected: textproto.CanonicalMIMEHeaderKey(row.Key)})
		report.Cases = append(report.Cases, testCase{Inputs: []any{row.Key}, Expected: canonicalHeaderReference(row.Key)})
	}
	return []suite{scalar, report}, nil
}
