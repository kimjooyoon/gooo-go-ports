// Independent Go reference and strict transport boundary only. Candidate
// behavior is authored in ports/path/path.gooo, never supplied by this process.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"unicode/utf8"
)

const goVersion = "go1.27.3"
const pathSourceSHA = "0a5024c364387eeea9ab5fe0b285f0d62355e8cd181cdbddfdfa227f6916d94c"

type facts struct {
	Text           string `json:"text"`
	Ext            string `json:"ext"`
	IsAbs          bool   `json:"is_abs"`
	TextByteLength int64  `json:"text_byte_length"`
	ExtByteLength  int64  `json:"ext_byte_length"`
	HasExtension   bool   `json:"has_extension"`
}
type manualRow struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected facts  `json:"expected"`
}
type manualContract struct {
	Schema string      `json:"schema"`
	Status string      `json:"status"`
	Cases  []manualRow `json:"cases"`
}

func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }

func reference(text string) facts {
	extension := path.Ext(text)
	return facts{Text: text, Ext: extension, IsAbs: path.IsAbs(text),
		TextByteLength: int64(len(text)), ExtByteLength: int64(len(extension)), HasExtension: extension != ""}
}

func exhaustiveInputs() []string {
	inputs := make([]string, 0, 9841)
	alphabet := []byte{'a', '.', '/'}
	for length := 0; length <= 8; length++ {
		count := 1
		for j := 0; j < length; j++ {
			count *= 3
		}
		for code := 0; code < count; code++ {
			value, rest := make([]byte, length), code
			for j := length - 1; j >= 0; j-- {
				value[j] = alphabet[rest%3]
				rest /= 3
			}
			inputs = append(inputs, string(value))
		}
	}
	return inputs
}

func originalGoIdentity() error {
	if runtime.Version() != goVersion {
		return fmt.Errorf("require %s, got %s", goVersion, runtime.Version())
	}
	raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src/path/path.go"))
	if err != nil {
		return err
	}
	if digest(raw) != "sha256:"+pathSourceSHA {
		return fmt.Errorf("pinned original Go path source changed")
	}
	return nil
}

func run() error {
	manual := flag.String("manual", "manual-contract.json", "frozen24 manual rows")
	corpus := flag.String("corpus", "manual", "manual or exhaustive; exhaustive is a future separate9841-tuple run")
	checkInput := flag.String("check-input", "", "strictly validate one native input document without executing Gooo")
	entry := flag.String("entry", "app/path-facts:Report", "single native input key")
	out := flag.String("out", "", "new output file or stdout; refuses overwrite")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional argument")
	}
	if err := originalGoIdentity(); err != nil {
		return err
	}
	if *checkInput != "" {
		if *out != "" || *corpus != "manual" {
			return fmt.Errorf("input check cannot write oracle output")
		}
		raw, err := os.ReadFile(*checkInput)
		if err != nil {
			return err
		}
		values, err := nativeInputs(raw, *entry)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "STRICT_TRANSPORT_ACCEPTED_NO_GOOO_EXECUTION", "inputs": len(values)})
	}
	var rows []manualRow
	manualSHA := ""
	switch *corpus {
	case "manual":
		raw, err := os.ReadFile(*manual)
		if err != nil {
			return err
		}
		if _, err := strictJSON(raw); err != nil {
			return err
		}
		var frozen manualContract
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&frozen); err != nil {
			return err
		}
		if frozen.Schema != "gooo/path-facts-manual-contract/v1" || frozen.Status != "FROZEN_MANUAL24_UNEXECUTED" || len(frozen.Cases) != 24 {
			return fmt.Errorf("manual24 contract differs")
		}
		for i, row := range frozen.Cases {
			if row.Name == "" || !utf8.ValidString(row.Input) || len(row.Input) > 1024 || !reflect.DeepEqual(reference(row.Input), row.Expected) {
				return fmt.Errorf("independent original Go differs from frozen manual row %d", i)
			}
		}
		rows, manualSHA = frozen.Cases, digest(raw)
	case "exhaustive":
		inputs := exhaustiveInputs()
		if len(inputs) != 9841 {
			return fmt.Errorf("future exhaustive corpus size changed")
		}
		for i, text := range inputs {
			rows = append(rows, manualRow{fmt.Sprintf("exhaustive-%d", i), text, reference(text)})
		}
	default:
		return fmt.Errorf("unknown corpus")
	}
	artifact := map[string]any{"schema": "gooo/path-facts-original-go-oracle/v1", "status": "ORIGINAL_GO_EXPECTATIONS_ONLY_NO_GOOO_EXECUTION",
		"go_version": goVersion, "go_path_source_sha256": pathSourceSHA, "corpus": *corpus,
		"manual_contract_sha256": manualSHA, "cases": len(rows), "results": rows}
	f := os.Stdout
	if *out != "" {
		var err error
		f, err = os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
	}
	if err := json.NewEncoder(f).Encode(artifact); err != nil {
		if f != os.Stdout {
			f.Close()
		}
		return err
	}
	if f != os.Stdout {
		return f.Close()
	}
	return nil
}

func nativeInputs(raw []byte, entry string) ([]string, error) {
	if entry != "go/path:Ext" && entry != "go/path:IsAbs" && entry != "app/path-facts:Report" {
		return nil, fmt.Errorf("UNKNOWN_ENTRY")
	}
	v, err := strictJSON(raw)
	if err != nil {
		return nil, err
	}
	root, ok := v.(map[string]any)
	if !ok || len(root) != 2 || root["schema"] != "gooo/body-composition-inputs/v1" {
		return nil, fmt.Errorf("INPUT_SCHEMA")
	}
	rows, ok := root["inputs"].([]any)
	if !ok || len(rows) == 0 || len(rows) > 128 {
		return nil, fmt.Errorf("INPUT_ROW_BOUND")
	}
	values := make([]string, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("INPUT_OBJECT")
		}
		if _, present := row[entry]; !present {
			return nil, fmt.Errorf("REQUIRED_INPUT")
		}
		if len(row) != 1 {
			return nil, fmt.Errorf("UNEXPECTED_INPUT")
		}
		text, ok := row[entry].(string)
		if !ok {
			return nil, fmt.Errorf("TEXT_TYPE")
		}
		if !utf8.ValidString(text) {
			return nil, fmt.Errorf("INVALID_RAW_UTF8")
		}
		if len(text) > 1024 {
			return nil, fmt.Errorf("TEXT_BYTE_BOUND")
		}
		values = append(values, text)
	}
	return values, nil
}

func strictJSON(raw []byte) (any, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("INVALID_RAW_UTF8")
	}
	if err := validEscapedUnicode(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := strictValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("TRAILING_JSON")
	}
	return v, nil
}

func strictValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON_DEPTH")
	}
	token, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("MALFORMED_JSON")
	}
	delim, container := token.(json.Delim)
	if !container {
		return token, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return nil, fmt.Errorf("MALFORMED_JSON")
			}
			if _, exists := m[name]; exists {
				return nil, fmt.Errorf("DUPLICATE_KEY")
			}
			value, err := strictValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[name] = value
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("MALFORMED_JSON")
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			value, err := strictValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, value)
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("MALFORMED_JSON")
		}
		return a, nil
	default:
		return nil, fmt.Errorf("MALFORMED_JSON")
	}
}

func validEscapedUnicode(raw []byte) error {
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return fmt.Errorf("MALFORMED_JSON")
		}
		if raw[i] != 'u' {
			continue
		}
		value, ok := hexQuad(raw, i+1)
		if !ok {
			return fmt.Errorf("MALFORMED_JSON")
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return fmt.Errorf("INVALID_UNICODE_SCALAR")
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return fmt.Errorf("INVALID_UNICODE_SCALAR")
			}
			low, ok := hexQuad(raw, i+3)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("INVALID_UNICODE_SCALAR")
			}
			i += 6
		}
	}
	return nil
}

func hexQuad(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for _, c := range raw[start : start+4] {
		value <<= 4
		switch {
		case c >= '0' && c <= '9':
			value += uint16(c - '0')
		case c >= 'a' && c <= 'f':
			value += uint16(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			value += uint16(c - 'A' + 10)
		default:
			return 0, false
		}
	}
	return value, true
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
