// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	recorded "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func inputJSON(t *testing.T, previous []byte, state string, pages []refresh.SnapshotPage) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"previous_catalog_base64": base64.StdEncoding.EncodeToString(previous), "collector_state": state, "pages": pages})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func page(id, cursor, next, tools string) refresh.SnapshotPage {
	params := `{}`
	if cursor != "" {
		params = `{"cursor":"` + cursor + `"}`
	}
	nextField := ""
	if next != "" {
		nextField = `,"nextCursor":"` + next + `"`
	}
	return refresh.SnapshotPage{RequestJSON: `{"jsonrpc":"2.0","id":` + id + `,"method":"tools/list","params":` + params + `}`, ResponseJSON: `{"jsonrpc":"2.0","id":` + id + `,"result":{"tools":` + tools + nextField + `}}`}
}

func execute(t *testing.T, raw []byte) report {
	t.Helper()
	original := bytes.Clone(raw)
	var out, diagnostics bytes.Buffer
	if code := runCommand(nil, bytes.NewReader(raw), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, diagnostics.String(), out.String())
	}
	if !bytes.Equal(raw, original) {
		t.Fatal("caller input bytes mutated")
	}
	var result report
	d := json.NewDecoder(&out)
	d.DisallowUnknownFields()
	if err := d.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatalf("stdout has trailing data: %v", err)
	}
	sum := sha256.Sum256(raw)
	if result.InputSHA256 != "sha256:"+hex.EncodeToString(sum[:]) || result.Scope != "SINGLE_IN_PROCESS_REFRESH" || result.CollectorStateBasis != "CALLER_SUPPLIED" {
		t.Fatalf("incorrect report binding/scope: %+v", result)
	}
	storedSum := sha256.Sum256(result.StoredCatalog.Bytes)
	if result.StoredCatalog.SHA256 != "sha256:"+hex.EncodeToString(storedSum[:]) || result.Receipt.AfterSHA256 != result.StoredCatalog.SHA256 || result.Receipt.AfterVersion != result.StoredCatalog.RegistryVersion {
		t.Fatal("stored bytes, receipt and returned hash/version disagree")
	}
	if result.Receipt.Decision != nil && (result.Receipt.Decision.ExecutionAuthorized || result.Receipt.Decision.LiveEffectPerformed || result.Receipt.Decision.RemoteAtomicity != "UNKNOWN") {
		t.Fatal("unexpected authority claim")
	}
	return result
}

func TestOwnPagesPreserveOutputSchemaChange(t *testing.T) {
	beforeTool := `[{"name":"customer_inventory_v17","inputSchema":{"type":"object"},"outputSchema":{"type":"object","properties":{"remaining":{"type":"integer"}}}}]`
	afterTool := `[{"name":"customer_inventory_v17","inputSchema":{"type":"object"},"outputSchema":{"type":"object","properties":{"remaining":{"type":"string"}}}}]`
	old := []byte(`{"pages":[{"request_json":"old saved request","response_json":"old saved response"}]}`)
	before := execute(t, inputJSON(t, old, "SUCCESS", []refresh.SnapshotPage{page("731", "", "", beforeTool)}))
	pages := []refresh.SnapshotPage{page("812", "", "second", afterTool), page("813", "second", "", `[{"name":"customer_audit_v8","inputSchema":{"type":"object"}}]`)}
	after := execute(t, inputJSON(t, before.StoredCatalog.Bytes, "SUCCESS", pages))
	if after.Receipt.State != "APPLIED" || after.Receipt.Decision == nil || after.Receipt.Decision.CandidateRowCount != 2 || after.Receipt.Decision.CandidatePageCount != 2 {
		t.Fatalf("own two-page candidate not applied: %+v", after.Receipt)
	}
	if after.Receipt.BeforeSHA256 != before.StoredCatalog.SHA256 || after.Receipt.AfterSHA256 == before.StoredCatalog.SHA256 {
		t.Fatal("old bytes or changed outputSchema lost")
	}
	var actual recorded.SnapshotObservation
	if err := json.Unmarshal(after.StoredCatalog.Bytes, &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Pages) != 2 || actual.Pages[0] != pages[0] || actual.Pages[1] != pages[1] {
		t.Fatal("original supplied frames changed or disappeared")
	}
	if !strings.Contains(actual.Pages[0].ResponseJSON, `"remaining":{"type":"string"}`) {
		t.Fatal("own output schema not retained")
	}
}

func TestIncompleteOrFailedCollectionRetainsExactBinaryPrevious(t *testing.T) {
	previous := []byte{0xff, 0x00, '\r', '\n', 0x80, 'x'}
	partial := []refresh.SnapshotPage{page("41", "", "later", `[{"name":"private_resource_z","inputSchema":{"type":"object"}}]`)}
	for _, state := range []string{"SUCCESS", "ERROR", "TIMEOUT", "CANCELLED"} {
		t.Run(state, func(t *testing.T) {
			r := execute(t, inputJSON(t, previous, state, partial))
			if r.Receipt.State != "RETAINED" || r.Receipt.Decision == nil || r.Receipt.Decision.State != "RETAIN_STORED_CATALOG" || !bytes.Equal(r.StoredCatalog.Bytes, previous) || r.Receipt.BeforeVersion != r.Receipt.AfterVersion || r.Receipt.BeforeSHA256 != r.Receipt.AfterSHA256 || r.CollectorState != state {
				t.Fatalf("old state not retained: %+v", r)
			}
		})
	}
	// A complete page still cannot erase an explicitly observed collection failure.
	r := execute(t, inputJSON(t, previous, "ERROR", []refresh.SnapshotPage{page("42", "", "", "[]")}))
	if r.Receipt.State != "RETAINED" || !bytes.Equal(r.StoredCatalog.Bytes, previous) {
		t.Fatal("explicit collection failure replaced storage")
	}
}

func TestTerminalEmptyReplacesNonemptyButMissingPagesRetain(t *testing.T) {
	previous := []byte("nonempty original catalog")
	missing := execute(t, inputJSON(t, previous, "SUCCESS", []refresh.SnapshotPage{}))
	if missing.Receipt.State != "RETAINED" || !bytes.Equal(missing.StoredCatalog.Bytes, previous) {
		t.Fatal("missing pages were treated as empty success")
	}
	empty := page("981", "", "", "[]")
	r := execute(t, inputJSON(t, previous, "SUCCESS", []refresh.SnapshotPage{empty}))
	if r.Receipt.State != "APPLIED" || r.Receipt.Decision == nil || r.Receipt.Decision.CandidateRowCount != 0 || r.Receipt.BeforeVersion == r.Receipt.AfterVersion || bytes.Equal(r.StoredCatalog.Bytes, previous) {
		t.Fatalf("complete empty catalog not applied: %+v", r)
	}
	var stored recorded.SnapshotObservation
	if err := json.Unmarshal(r.StoredCatalog.Bytes, &stored); err != nil || len(stored.Pages) != 1 || stored.Pages[0] != empty {
		t.Fatalf("lost explicit empty terminal page: %v %+v", err, stored)
	}
}

func TestMalformedInputsProduceNoResult(t *testing.T) {
	valid := string(inputJSON(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")}))
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(valid), &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"previous_catalog_base64", "collector_state", "pages"} {
		copyFields := map[string]json.RawMessage{}
		for k, v := range fields {
			if k != key {
				copyFields[k] = v
			}
		}
		b, _ := json.Marshal(copyFields)
		assertInvalid(t, "missing "+key, b)
	}
	cases := map[string][]byte{
		"trailing":            []byte(valid + `{}`),
		"unknown":             []byte(strings.Replace(valid, `{`, `{"extra":true,`, 1)),
		"duplicate":           []byte(strings.Replace(valid, `{`, `{"collector_state":"ERROR",`, 1)),
		"case variation":      []byte(strings.Replace(valid, `"pages"`, `"Pages"`, 1)),
		"null pages":          []byte(`{"previous_catalog_base64":"","collector_state":"SUCCESS","pages":null}`),
		"null previous":       []byte(`{"previous_catalog_base64":null,"collector_state":"SUCCESS","pages":[]}`),
		"invalid base64":      []byte(`{"previous_catalog_base64":"@@","collector_state":"SUCCESS","pages":[]}`),
		"noncanonical base64": []byte(`{"previous_catalog_base64":"eA==\n","collector_state":"SUCCESS","pages":[]}`),
		"unknown collector":   []byte(strings.Replace(valid, `"SUCCESS"`, `"UNKNOWN"`, 1)),
		"unpaired surrogate":  []byte(`{"previous_catalog_base64":"","collector_state":"SUCCESS","pages":[{"request_json":"\ud800","response_json":"{}"}]}`),
		"nested duplicate":    []byte(`{"previous_catalog_base64":"","collector_state":"SUCCESS","pages":[{"request_json":"{}","request_json":"{}","response_json":"{}"}]}`),
		"wrong frame":         inputJSON(t, nil, "SUCCESS", []refresh.SnapshotPage{{RequestJSON: "{}", ResponseJSON: "{}"}}),
		"oversized":           []byte(strings.Repeat(" ", recorded.MaxInputBytes+1)),
		"invalid UTF8":        append([]byte(valid), 0xff),
		"oversized frame":     inputJSON(t, nil, "SUCCESS", []refresh.SnapshotPage{{RequestJSON: strings.Repeat("x", recorded.MaxFrameBytes+1), ResponseJSON: "{}"}}),
	}
	for name, raw := range cases {
		assertInvalid(t, name, raw)
	}
}

func assertInvalid(t *testing.T, name string, raw []byte) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		var out, diagnostics bytes.Buffer
		if code := runCommand(nil, bytes.NewReader(raw), &out, &diagnostics); code != 1 || out.Len() != 0 || !json.Valid(diagnostics.Bytes()) {
			t.Fatalf("invalid input yielded code=%d out=%q stderr=%q", code, out.String(), diagnostics.String())
		}
	})
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestIOAndArgumentsDoNotClaimSuccess(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if runCommand(nil, brokenReader{}, &out, &diagnostics) != 1 || out.Len() != 0 {
		t.Fatal("read error succeeded")
	}
	raw := inputJSON(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")})
	for _, w := range []io.Writer{shortWriter{}, brokenWriter{}} {
		diagnostics.Reset()
		if runCommand(nil, bytes.NewReader(raw), w, &diagnostics) != 1 || !json.Valid(diagnostics.Bytes()) {
			t.Fatal("output failure succeeded")
		}
	}
	for _, args := range [][]string{{"--help", "--unknown"}, {"--collector-state", "SUCCESS"}} {
		out.Reset()
		diagnostics.Reset()
		if runCommand(args, brokenReader{}, &out, &diagnostics) != 2 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "UNSUPPORTED_ARGUMENTS") {
			t.Fatal("invalid arguments read input or succeeded")
		}
	}
	out.Reset()
	diagnostics.Reset()
	if runCommand([]string{"--help"}, brokenReader{}, &out, &diagnostics) != 0 || !strings.Contains(out.String(), "previous_catalog_base64") || diagnostics.Len() != 0 {
		t.Fatal("help reads input or failed")
	}
}
