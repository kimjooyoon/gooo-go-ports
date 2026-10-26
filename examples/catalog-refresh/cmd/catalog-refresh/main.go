// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
// catalog-refresh evaluates supplied saved pages in a new in-memory registry.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"unicode/utf8"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	host "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/host"
	recorded "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

const usage = `Evaluate one whole-catalog refresh using generated Gooo rules.

Usage: catalog-refresh < saved-refresh.json > report.json
       catalog-refresh --help

Input is exactly one JSON object, at most 1 MiB, with these required fields:
  previous_catalog_base64: canonical padded base64 of the previous stored bytes
  collector_state: SUCCESS, ERROR, TIMEOUT, or CANCELLED (explicitly supplied)
  pages: ordered [{"request_json":"...","response_json":"..."}] saved frames

An empty previous byte string is "". Original page strings are preserved.
The command accepts at most 16 pages, 256 rows, and 64 KiB per saved frame.
Malformed input is rejected; incomplete or failed collection retains old bytes.
Successful complete collection can replace the catalog, including with no tools.

Exit 0 writes one JSON report containing the actual local receipt and stored_catalog.
stored_catalog.bytes is base64, including when the retained bytes are not JSON.
Exit 1 reports input, execution or I/O failure as JSON on stderr. Invalid arguments
exit 2. RETAINED is a completed result, not a process failure or permission to use
old tools. Preserve input, stderr, exit status and the report together.

Each invocation creates a new in-process registry; tokens do not persist across
invocations. Collector state is a caller assertion, not verified collection.
This command does not write catalog files, connect to MCP servers, collect pages,
call tools or models, establish current authorization, or perform a durable swap.
`

type refreshInput struct {
	previous  []byte
	pages     []refresh.SnapshotPage
	collector string
}

type report struct {
	Schema              string             `json:"schema"`
	Scope               string             `json:"scope"`
	InputSHA256         string             `json:"input_sha256"`
	CollectorState      string             `json:"collector_state"`
	CollectorStateBasis string             `json:"collector_state_basis"`
	Receipt             refresh.Receipt    `json:"receipt"`
	StoredCatalog       host.StoredCatalog `json:"stored_catalog"`
}

type problem struct {
	State string `json:"state"`
	Stage string `json:"stage"`
	Code  string `json:"code"`
}

func (p *problem) Error() string { return p.Code }
func invalid(code string) error  { return &problem{"INVALID_INPUT", "decode_input", code} }

// Decode the exact outer fields without the case-insensitive struct matching or
// last-key-wins behavior of json.Unmarshal. The shared projector then validates
// the untouched pages JSON before its strings are decoded, including duplicate
// keys, raw UTF-8, unpaired surrogate escapes, frame identity and resource bounds.
func decodeInput(raw []byte) (refreshInput, error) {
	if len(raw) > recorded.MaxInputBytes || !utf8.Valid(raw) || !json.Valid(raw) {
		return refreshInput{}, invalid("JSON_SIZE_OR_ENCODING")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if start, err := d.Token(); err != nil || start != json.Delim('{') {
		return refreshInput{}, invalid("EXACT_OBJECT_REQUIRED")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return refreshInput{}, invalid("INVALID_KEY")
		}
		if key != "previous_catalog_base64" && key != "collector_state" && key != "pages" {
			return refreshInput{}, invalid("UNKNOWN_FIELD")
		}
		if _, duplicate := fields[key]; duplicate {
			return refreshInput{}, invalid("DUPLICATE_FIELD")
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return refreshInput{}, invalid("INVALID_FIELD")
		}
		fields[key] = value
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') || len(fields) != 3 {
		return refreshInput{}, invalid("MISSING_FIELD")
	}
	if _, err := d.Token(); err != io.EOF {
		return refreshInput{}, invalid("TRAILING_JSON")
	}
	var encoded, collector string
	if len(fields["previous_catalog_base64"]) == 0 || fields["previous_catalog_base64"][0] != '"' || json.Unmarshal(fields["previous_catalog_base64"], &encoded) != nil {
		return refreshInput{}, invalid("BASE64_STRING_REQUIRED")
	}
	previous, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.StdEncoding.EncodeToString(previous) != encoded {
		return refreshInput{}, invalid("CANONICAL_BASE64_REQUIRED")
	}
	if json.Unmarshal(fields["collector_state"], &collector) != nil || (collector != "SUCCESS" && collector != "ERROR" && collector != "TIMEOUT" && collector != "CANCELLED") {
		return refreshInput{}, invalid("EXPLICIT_COLLECTOR_STATE_REQUIRED")
	}
	projectorInput := append([]byte(`{"target_name":"","before":{"pages":[]},"after":{"pages":`), fields["pages"]...)
	projectorInput = append(projectorInput, []byte(`}}`)...)
	if _, err := recorded.ProjectDefinitionSnapshotInputsJSON(projectorInput); err != nil {
		return refreshInput{}, err
	}
	var pages []refresh.SnapshotPage
	if json.Unmarshal(fields["pages"], &pages) != nil {
		return refreshInput{}, invalid("INVALID_PAGES")
	}
	return refreshInput{previous, pages, collector}, nil
}

func diagnostic(out io.Writer, err error) int {
	var local *problem
	var shared *recorded.Problem
	switch {
	case errors.As(err, &local):
		_ = json.NewEncoder(out).Encode(local)
	case errors.As(err, &shared):
		_ = json.NewEncoder(out).Encode(shared)
	default:
		_ = json.NewEncoder(out).Encode(problem{"UNKNOWN", "command", "IO_OR_EXECUTION_FAILURE"})
	}
	return 1
}

func emit(raw []byte, out, diagnostics io.Writer) int {
	n, err := out.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	return 0
}

func runCommand(args []string, input io.Reader, out, diagnostics io.Writer) int {
	if len(args) != 0 {
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			return emit([]byte(usage), out, diagnostics)
		}
		diagnostic(diagnostics, &problem{"INVALID_ARGUMENT", "parse_arguments", "UNSUPPORTED_ARGUMENTS"})
		return 2
	}
	raw, err := io.ReadAll(io.LimitReader(input, recorded.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	parsed, err := decodeInput(raw)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	registry, err := refresh.NewRegistry(parsed.previous)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	attempt, err := registry.Begin(context.Background())
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	receipt := registry.Refresh(attempt, parsed.pages, parsed.collector)
	sum := sha256.Sum256(raw)
	result := report{"gooo/local-catalog-refresh-report/v1", "SINGLE_IN_PROCESS_REFRESH", "sha256:" + hex.EncodeToString(sum[:]), parsed.collector, "CALLER_SUPPLIED", receipt, registry.Snapshot()}
	encoded, err := json.Marshal(result)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	if len(encoded) > recorded.MaxReportBytes {
		return diagnostic(diagnostics, &problem{"UNKNOWN", "encode_result", "OUTPUT_LIMIT_EXCEEDED"})
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}

func main() { os.Exit(runCommand(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
