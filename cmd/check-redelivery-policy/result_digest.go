package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"
)

var resultProfileJSON []byte

const expectedResultProfileBytes = 202475
const expectedResultProfileSHA256 = "46862aabf1934c1d2a39ad830c4a6cd05d2369c5014fea231d87b7861f4d3862"

// Main loads the shared public profile once before observing receipts. Failed
// loads clear prior state; concurrent reloads are outside this single-run CLI.
func loadResultProfile(path string) error {
	resultProfileJSON = nil
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("load pinned result profile: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != expectedResultProfileBytes {
		return fmt.Errorf("pinned result profile size or regular-file kind differs")
	}
	raw, err := io.ReadAll(io.LimitReader(file, expectedResultProfileBytes+1))
	if err != nil {
		return err
	}
	if len(raw) != expectedResultProfileBytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != expectedResultProfileSHA256 {
		return fmt.Errorf("pinned result profile bytes/hash differ")
	}
	if compilerRevision != "205896a413e5dea92366309c35ae85e542d055f6" {
		return fmt.Errorf("invalid pinned result format profile")
	}
	var profile resultProfile
	if err := json.Unmarshal(raw, &profile); err != nil || profile.Compiler != compilerRevision {
		return fmt.Errorf("invalid pinned result format profile")
	}
	resultProfileJSON = raw
	return nil
}

type resultProfile struct {
	Compiler string                  `json:"compiler_revision"`
	Nodes    map[string]profileShape `json:"nodes"`
}
type profileShape struct {
	Kinds      []string       `json:"kinds"`
	Fields     []profileField `json:"fields"`
	NumberType string         `json:"number_type,omitempty"`
}
type profileField struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional,omitempty"`
	OmitZero bool   `json:"omit_zero,omitempty"`
}

var rawResultValues = map[string]bool{
	"$.runtime.traces.*.deliveries.*.input":                 true,
	"$.runtime.traces.*.deliveries.*.inputs.*.value":        true,
	"$.runtime.traces.*.deliveries.*.actual":                true,
	"$.runtime.traces.*.deliveries.*.actual_fields.*.value": true,
}
var genericResultMaps = map[string]bool{
	"$.composition.steps.*.generation.report.completeness_receipt.scope":         true,
	"$.composition.steps.*.generation.report.completeness_receipt.status_counts": true,
}

// The explicitly selected compiler source hashes json.Marshal(prior typed Result).
// This deliberately supports only the canonical native CLI result profile for
// this frozen workflow. Typed field order, scalar spelling, optional fields and
// shapes must match the pinned profile; unsupported reserializations fail closed.
// The two generic maps are decoded with UseNumber and recursively key-sorted.
// RawMessage values retain order, number spelling and pre-existing escapes.
// Finally json.Marshal(RawMessage) applies the default Go HTML escaping. Plain
// json.Compact, marshaling the whole result as a map, or hashing the envelope
// cannot substitute for the pinned typed-result digest.
func pinnedResultDigest(receipt []byte) (string, error) {
	encoded, err := pinnedResultBytes(receipt)
	if err != nil {
		return "", err
	}
	return digest(encoded), nil
}

func pinnedResultBytes(receipt []byte) ([]byte, error) {
	if len(resultProfileJSON) == 0 {
		return nil, fmt.Errorf("pinned result profile is not loaded")
	}
	if len(resultProfileJSON) != 202475 || digest(resultProfileJSON) != "sha256:46862aabf1934c1d2a39ad830c4a6cd05d2369c5014fea231d87b7861f4d3862" || compilerRevision != "205896a413e5dea92366309c35ae85e542d055f6" {
		return nil, fmt.Errorf("current result profile raw identity differs")
	}
	return pinnedResultBytesWithProfile(receipt, resultProfileJSON, compilerRevision)
}

// Explicit archival fixture entry; no input-based fallback or global reload.
func legacy839ResultBytes(receipt, legacyProfile []byte) ([]byte, error) {
	if len(legacyProfile) != 58326 || digest(legacyProfile) != "sha256:9eafadab81a6c7fe33a4f59777e3776cc27f31c7962e472f3828e77867142d44" {
		return nil, fmt.Errorf("historical result profile raw identity differs")
	}
	return pinnedResultBytesWithProfile(receipt, legacyProfile, "83928fa4538245abe33b08549da47f8cbdc50ed2")
}

func legacy839ResultDigest(receipt, legacyProfile []byte) (string, error) {
	encoded, err := legacy839ResultBytes(receipt, legacyProfile)
	if err != nil {
		return "", err
	}
	return digest(encoded), nil
}

func pinnedResultBytesWithProfile(receipt, profileJSON []byte, profileCompiler string) ([]byte, error) {
	if len(receipt) == 0 || len(receipt) > 32<<20 || !utf8.Valid(receipt) {
		return nil, fmt.Errorf("invalid pinned receipt size or UTF-8")
	}
	limits := json.NewDecoder(bytes.NewReader(receipt))
	limits.UseNumber()
	if err := pinnedJSONDepth(limits, 0); err != nil {
		return nil, err
	}
	if _, err := decode(receipt); err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(receipt, &envelope); err != nil {
		return nil, err
	}
	if len(envelope) != 6 {
		return nil, fmt.Errorf("unsupported original-receipt envelope shape")
	}
	for _, key := range []string{"schema", "decision", "manifest", "manifest_digest", "inputs_digest", "result"} {
		if _, ok := envelope[key]; !ok {
			return nil, fmt.Errorf("unsupported original-receipt envelope field")
		}
		if key != "result" {
			var value string
			if len(envelope[key]) == 0 || envelope[key][0] != '"' || json.Unmarshal(envelope[key], &value) != nil {
				return nil, fmt.Errorf("unsupported original-receipt envelope scalar")
			}
		}
	}
	var profile resultProfile
	if err := json.Unmarshal(profileJSON, &profile); err != nil || profile.Compiler != profileCompiler {
		return nil, fmt.Errorf("invalid pinned result format profile")
	}
	var producer struct {
		Runtime struct {
			Source string `json:"producer_source_sha"`
		} `json:"runtime"`
	}
	if json.Unmarshal(envelope["result"], &producer) != nil || producer.Runtime.Source != profileCompiler {
		return nil, fmt.Errorf("receipt producer differs from explicitly selected result profile")
	}
	raw, ok := envelope["result"]
	if !ok {
		return nil, fmt.Errorf("missing original result")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	normalized, err := normalizeResultValueForProfile(d, raw, "$", profile.Nodes, 0, profileCompiler == "205896a413e5dea92366309c35ae85e542d055f6")
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing original-result data")
	}
	return json.Marshal(json.RawMessage(normalized))
}

func normalizeResultValue(d *json.Decoder, input []byte, path string, profile map[string]profileShape, depth int) ([]byte, error) {
	// Preserve the explicitly archival/default normalization behavior.
	return normalizeResultValueForProfile(d, input, path, profile, depth, false)
}

// Current205 RawMessage omitempty tests encoded byte length, not the decoded
// JSON zero value. Only the existing exact RawMessage path roster gets this
// rule; pointer/scalar/slice omissions and all historical839 paths stay strict.
func normalizeResultValueForProfile(d *json.Decoder, input []byte, path string, profile map[string]profileShape, depth int, current205 bool) ([]byte, error) {
	if depth > 64 {
		return nil, fmt.Errorf("original result exceeds pinned depth limit")
	}
	if rawResultValues[path] {
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		return json.Marshal(raw)
	}
	if genericResultMaps[path] {
		var value any
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		m, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported pinned generic-map shape at %s", path)
		}
		if path == "$.composition.steps.*.generation.report.completeness_receipt.status_counts" {
			for _, count := range m {
				n, ok := count.(json.Number)
				if !ok {
					return nil, fmt.Errorf("noninteger status count")
				}
				if _, err := n.Int64(); err != nil {
					return nil, fmt.Errorf("invalid pinned status count: %w", err)
				}
				if n.String() == "-0" {
					return nil, fmt.Errorf("noncanonical pinned status count")
				}
			}
		}
		return json.Marshal(m)
	}
	shape, known := profile[path]
	if !known {
		return nil, fmt.Errorf("unsupported pinned result field/shape at %s", path)
	}
	start := d.InputOffset()
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	raw := scalarToken(input[start:d.InputOffset()])
	checkKind := func(kind string) error {
		for _, allowed := range shape.Kinds {
			if allowed == kind {
				return nil
			}
		}
		return fmt.Errorf("unsupported pinned result kind at %s", path)
	}
	if delimiter, ok := token.(json.Delim); ok {
		var out bytes.Buffer
		out.WriteByte(byte(delimiter))
		switch delimiter {
		case '{':
			if err := checkKind("object"); err != nil {
				return nil, err
			}
			var fields []string
			nextField := 0
			for d.More() {
				start := d.InputOffset()
				key, err := d.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok || !bytes.Equal(scalarToken(input[start:d.InputOffset()]), cliString(name)) {
					return nil, fmt.Errorf("noncanonical typed key at %s", path)
				}
				if len(fields) > 0 {
					out.WriteByte(',')
				}
				fields = append(fields, name)
				for nextField < len(shape.Fields) && shape.Fields[nextField].Name != name && shape.Fields[nextField].Optional {
					nextField++
				}
				if nextField == len(shape.Fields) || shape.Fields[nextField].Name != name {
					return nil, fmt.Errorf("unsupported typed field order/optional shape at %s", path)
				}
				field := shape.Fields[nextField]
				nextField++
				keyRaw, _ := json.Marshal(name)
				out.Write(keyRaw)
				out.WriteByte(':')
				value, err := normalizeResultValueForProfile(d, input, path+"."+name, profile, depth+1, current205)
				if err != nil {
					return nil, err
				}
				rawMessage := current205 && rawResultValues[path+"."+name]
				if field.Optional && !rawMessage && (bytes.Equal(value, []byte("null")) || field.OmitZero && canonicalEmpty(value)) {
					return nil, fmt.Errorf("noncanonical omitempty value at %s.%s", path, name)
				}
				out.Write(value)
			}
			for _, field := range shape.Fields[nextField:] {
				if !field.Optional {
					return nil, fmt.Errorf("missing typed field at %s.%s", path, field.Name)
				}
			}
		case '[':
			if err := checkKind("array"); err != nil {
				return nil, err
			}
			first := true
			for d.More() {
				if !first {
					out.WriteByte(',')
				}
				first = false
				value, err := normalizeResultValueForProfile(d, input, path+".*", profile, depth+1, current205)
				if err != nil {
					return nil, err
				}
				out.Write(value)
			}
		default:
			return nil, fmt.Errorf("unexpected pinned result delimiter")
		}
		closing, err := d.Token()
		if err != nil {
			return nil, err
		}
		out.WriteByte(byte(closing.(json.Delim)))
		return out.Bytes(), nil
	}
	var kind string
	switch value := token.(type) {
	case string:
		kind = "string"
		if !bytes.Equal(raw, cliString(value)) {
			return nil, fmt.Errorf("noncanonical typed string at %s", path)
		}
	case json.Number:
		kind = "integer"
		if shape.NumberType == "float64" {
			kind = "number"
			number, err := strconv.ParseFloat(value.String(), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid typed float64 at %s", path)
			}
			canonical, err := json.Marshal(number)
			if err != nil || !bytes.Equal(raw, canonical) {
				return nil, fmt.Errorf("noncanonical typed float64 at %s", path)
			}
			break
		}
		if _, err := strconv.ParseInt(value.String(), 10, 64); err != nil {
			return nil, fmt.Errorf("unsupported typed numeric spelling/range at %s", path)
		}
		if value.String() == "-0" {
			return nil, fmt.Errorf("noncanonical typed zero at %s", path)
		}
	case bool:
		kind = "boolean"
	case nil:
		kind = "null"
	default:
		return nil, fmt.Errorf("unexpected pinned result scalar")
	}
	if err := checkKind(kind); err != nil {
		return nil, err
	}
	return raw, nil
}

func canonicalEmpty(raw []byte) bool {
	for _, empty := range []string{`""`, "0", "-0", "false", "[]", "{}"} {
		if bytes.Equal(raw, []byte(empty)) {
			return true
		}
	}
	return false
}

// Scan the entire envelope, including opaque maps and RawMessage subtrees, with
// the same depth convention as the pinned decoder. Those subtrees are otherwise
// consumed in one Decode call by the serializer below.
func pinnedJSONDepth(d *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("pinned receipt exceeds 64 nested levels")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	for d.More() {
		if delimiter == '{' {
			if _, err := d.Token(); err != nil {
				return err
			}
		}
		if err := pinnedJSONDepth(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func scalarToken(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && (raw[0] == ':' || raw[0] == ',') {
		raw = bytes.TrimSpace(raw[1:])
	}
	return raw
}

func cliString(s string) []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s)
	return bytes.TrimSuffix(out.Bytes(), []byte{'\n'})
}
