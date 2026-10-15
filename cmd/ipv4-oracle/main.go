// Independent finite reference: valid classifications use actual Go netip methods.
// This program does not read or evaluate the Gooo implementation.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"unicode/utf8"
)

const (
	pinnedGo     = "go1.27.2"
	corpusBytes  = 43919
	corpusSHA256 = "7fb8c9d44e8fa77807e9b5580ab8aa975a8775486d5dfb6daaef9be2472b4b08"
	corpusSchema = "gooo/ipv4-classification-corpus/v1"
	outputSchema = "gooo/ipv4-independent-oracle/v1"
	caseCount    = 202
)

type octets struct {
	A int64 `json:"a"`
	B int64 `json:"b"`
	C int64 `json:"c"`
	D int64 `json:"d"`
}

type wireOctets struct {
	A *int64 `json:"a"`
	B *int64 `json:"b"`
	C *int64 `json:"c"`
	D *int64 `json:"d"`
}

type inputCase struct {
	Index  *int       `json:"index"`
	Name   string     `json:"name"`
	Cohort string     `json:"cohort"`
	Inputs wireOctets `json:"inputs"`
}

type corpus struct {
	Schema string      `json:"schema"`
	Cases  []inputCase `json:"cases"`
}

// No omitempty: false flags and the four original signed int64s remain present.
// Global unicast is a classification, not reachability or authorization.
type report struct {
	A                  int64 `json:"a"`
	B                  int64 `json:"b"`
	C                  int64 `json:"c"`
	D                  int64 `json:"d"`
	Valid              bool  `json:"valid"`
	IsPrivate          bool  `json:"is_private"`
	IsLoopback         bool  `json:"is_loopback"`
	IsUnspecified      bool  `json:"is_unspecified"`
	IsMulticast        bool  `json:"is_multicast"`
	IsLinkLocalUnicast bool  `json:"is_link_local_unicast"`
	IsGlobalUnicast    bool  `json:"is_global_unicast"`
}

type expectedCase struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Cohort   string `json:"cohort"`
	Inputs   octets `json:"inputs"`
	Expected report `json:"expected"`
}

type output struct {
	Schema                string         `json:"schema"`
	GoVersion             string         `json:"go_version"`
	CorpusBytes           int            `json:"corpus_bytes"`
	CorpusSHA256          string         `json:"corpus_sha256"`
	CaseCount             int            `json:"case_count"`
	RequiredReportFields  int            `json:"required_report_fields"`
	OriginalEchoes        int            `json:"original_echoes_per_report"`
	ValidStdlibCases      int            `json:"valid_stdlib_cases"`
	InvalidExtensionCases int            `json:"invalid_extension_cases"`
	Cases                 []expectedCase `json:"cases"`
	Scope                 string         `json:"scope"`
}

func classify(in octets) report {
	r := report{A: in.A, B: in.B, C: in.C, D: in.D}
	// This is the explicit Gooo integer-domain extension, not invalid Addr{}.
	// Validate every original int64 before any byte conversion or arithmetic.
	for _, value := range [4]int64{in.A, in.B, in.C, in.D} {
		if value < 0 || value > 255 {
			return r
		}
	}
	addr := netip.AddrFrom4([4]byte{byte(in.A), byte(in.B), byte(in.C), byte(in.D)})
	r.Valid = true
	r.IsPrivate = addr.IsPrivate()
	r.IsLoopback = addr.IsLoopback()
	r.IsUnspecified = addr.IsUnspecified()
	r.IsMulticast = addr.IsMulticast()
	r.IsLinkLocalUnicast = addr.IsLinkLocalUnicast()
	r.IsGlobalUnicast = addr.IsGlobalUnicast()
	return r
}

// Reject duplicate decoded keys before typed decoding; encoding/json alone
// otherwise accepts a repeated field. Numbers are never first read as float64.
func scanJSON(d *json.Decoder, depth int) error {
	if depth > 16 {
		return fmt.Errorf("corpus JSON nesting exceeds 16")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid corpus JSON key")
			}
			seen[key] = true
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("invalid corpus JSON object end")
		}
	case '[':
		for d.More() {
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("invalid corpus JSON array end")
		}
	default:
		return fmt.Errorf("unexpected corpus JSON delimiter")
	}
	return nil
}

func decodeCorpus(raw []byte) (corpus, error) {
	var value corpus
	if len(raw) == 0 || len(raw) > corpusBytes || !utf8.Valid(raw) {
		return value, fmt.Errorf("require bounded UTF8 corpus JSON")
	}
	scanner := json.NewDecoder(bytes.NewReader(raw))
	scanner.UseNumber()
	if err := scanJSON(scanner, 0); err != nil {
		return value, err
	}
	if _, err := scanner.Token(); err != io.EOF {
		return value, fmt.Errorf("require exactly one corpus JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	if value.Schema != corpusSchema || value.Cases == nil {
		return value, fmt.Errorf("require corpus schema and case array")
	}
	for i, row := range value.Cases {
		v := row.Inputs
		if row.Index == nil || v.A == nil || v.B == nil || v.C == nil || v.D == nil {
			return value, fmt.Errorf("case %d: require index and four present int64 inputs", i)
		}
	}
	return value, nil
}

func readCorpus(path string) (corpus, error) {
	var value corpus
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || canonical != path {
		return value, fmt.Errorf("require an absolute canonical corpus path")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != corpusBytes {
		return value, fmt.Errorf("require frozen regular corpus with exactly %d bytes", corpusBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return value, err
	}
	info, statErr := f.Stat()
	if statErr != nil || !os.SameFile(before, info) {
		f.Close()
		return value, fmt.Errorf("corpus identity changed before read")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, corpusBytes+1))
	closeErr := f.Close()
	if readErr != nil {
		return value, readErr
	}
	if closeErr != nil {
		return value, closeErr
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return value, fmt.Errorf("corpus identity changed during read")
	}
	if len(raw) != corpusBytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != corpusSHA256 {
		return value, fmt.Errorf("frozen corpus bytes/SHA256 differ")
	}
	return decodeCorpus(raw)
}

func evaluateCorpus(inputs corpus, goVersion string) (output, error) {
	result := output{Schema: outputSchema, GoVersion: goVersion, CorpusBytes: corpusBytes,
		CorpusSHA256: corpusSHA256, CaseCount: caseCount, RequiredReportFields: 11, OriginalEchoes: 4,
		Cases: make([]expectedCase, 0, caseCount),
		Scope: "Actual netip.AddrFrom4 and six Go methods for48 valid tuples;154 invalid int64 tuples use a separate unchanged-echo/false extension. Original64 prefix is retained. No Gooo/native/replay, exhaustive IPv4, reachability, authorization or production-safety claim."}
	if inputs.Schema != corpusSchema || len(inputs.Cases) != caseCount {
		return result, fmt.Errorf("require frozen 202-case corpus schema")
	}
	names := make(map[string]bool, caseCount)
	tuples := make(map[octets]bool, caseCount)
	for index, row := range inputs.Cases {
		cohort := "invalid"
		if index < 32 {
			cohort = "boundary"
		} else if index >= 48 && index < 64 {
			cohort = "seeded"
		}
		v := row.Inputs
		if row.Index == nil || *row.Index != index || row.Name == "" || names[row.Name] || row.Cohort != cohort || v.A == nil || v.B == nil || v.C == nil || v.D == nil {
			return result, fmt.Errorf("case %d: frozen index/name/cohort/inputs differ", index)
		}
		names[row.Name] = true
		original := octets{A: *v.A, B: *v.B, C: *v.C, D: *v.D}
		if tuples[original] {
			return result, fmt.Errorf("case %d: duplicate input tuple", index)
		}
		tuples[original] = true
		expected := classify(original)
		if expected.Valid != (cohort != "invalid") {
			return result, fmt.Errorf("case %d: integer-domain cohort differs", index)
		}
		if expected.Valid {
			result.ValidStdlibCases++
		} else {
			result.InvalidExtensionCases++
		}
		result.Cases = append(result.Cases, expectedCase{Index: index, Name: row.Name, Cohort: cohort, Inputs: original, Expected: expected})
	}
	if result.ValidStdlibCases != 48 || result.InvalidExtensionCases != 154 {
		return result, fmt.Errorf("frozen valid/extension population differs")
	}
	return result, nil
}

func run(args []string, out io.Writer, goVersion string) error {
	if len(args) >= 2 && args[0] == "--profile" && args[1] == "prefix-rule-pair" {
		return runPrefixRulePair(args[2:], out, goVersion)
	}
	flags := flag.NewFlagSet("ipv4-independent-oracle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	corpusPath := flags.String("corpus", "", "absolute path of frozen public202 input-only corpus")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *corpusPath == "" || flags.NArg() != 0 {
		return fmt.Errorf("require --corpus and no positional arguments")
	}
	if goVersion != pinnedGo {
		return fmt.Errorf("require %s; actual %s", pinnedGo, goVersion)
	}
	inputs, err := readCorpus(*corpusPath)
	if err != nil {
		return err
	}
	result, err := evaluateCorpus(inputs, goVersion)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, runtime.Version()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
