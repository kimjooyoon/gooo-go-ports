package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestObservationPreservesOrderSpellingAndDuplicates(t *testing.T) {
	raw := `{"origin_ref":"fixture://opaque origin","import_paths":["Example.org/X ","한글/경로","Example.org/X ","x\r", "\ud83d\ude00"]}`
	got, err := readObservation(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Example.org/X ", "한글/경로", "Example.org/X ", "x\r", "😀"}
	if !reflect.DeepEqual(got.ImportPaths, want) {
		t.Fatalf("observation changed: %#v", got.ImportPaths)
	}
	var out, diagnostics bytes.Buffer
	if code := run(nil, strings.NewReader(raw), &out, &diagnostics); code != 0 {
		t.Fatalf("code %d: %s", code, &diagnostics)
	}
	var wire struct {
		Status            string   `json:"status"`
		ObservedPaths     []string `json:"observed_paths"`
		ObservationOrigin string   `json:"observation_origin"`
	}
	if err := json.Unmarshal(out.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire.ObservedPaths, want) || wire.ObservationOrigin != got.OriginRef || wire.Status == "OBSERVED_EXACT" {
		t.Fatalf("lost observation or suppressed Gooo refusal: %s", &out)
	}
}

func TestRejectsMalformedObservationWithoutOutput(t *testing.T) {
	base := `{"origin_ref":"","import_paths":[]}`
	cases := []string{
		`null`, `{}`, `{"origin_ref":null,"import_paths":[]}`,
		`{"origin_ref":"","import_paths":null}`,
		`{"origin_ref":"","import_paths":[null]}`,
		`{"origin_ref":"","origin_ref":"","import_paths":[]}`,
		`{"origin_ref":"","import_paths":[],"declared_paths":[]}`,
		base + base,
		`{"origin_ref":"","import_paths":["\ud800"]}`,
		`{"origin_ref":"\udc00","import_paths":[]}`,
		`{"origin_ref":"\ud800x","import_paths":[]}`,
		string([]byte{'{', '"', 0xff, '"', ':', '0', '}'}),
		strings.Repeat(" ", maxInputBytes+1),
	}
	paths := make([]string, maxPaths+1)
	for i := range paths {
		paths[i] = "example.org/x"
	}
	many, _ := json.Marshal(map[string]any{"origin_ref": "", "import_paths": paths})
	cases = append(cases, string(many))
	longText, _ := json.Marshal(map[string]any{"origin_ref": strings.Repeat("é", maxTextBytes/2+1), "import_paths": []string{}})
	cases = append(cases, string(longText))
	for i, raw := range cases {
		var out, diagnostics bytes.Buffer
		if code := run(nil, strings.NewReader(raw), &out, &diagnostics); code != 2 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Errorf("case %d: code=%d out=%q errors=%q", i, code, &out, &diagnostics)
		}
	}
}

func TestEmptyObservationReachesGooo(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := run(nil, strings.NewReader(`{"origin_ref":"","import_paths":[]}`), &out, &diagnostics); code != 0 {
		t.Fatalf("code %d: %s", code, &diagnostics)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ORIGIN_UNOBSERVED" {
		t.Fatalf("host replaced Gooo result: %s", &out)
	}
}

type failedIO struct{}

func (failedIO) Read([]byte) (int, error)  { return 0, errors.New("read failed") }
func (failedIO) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestIOAndArgumentsFail(t *testing.T) {
	if code := run(nil, failedIO{}, io.Discard, io.Discard); code != 2 {
		t.Fatal(code)
	}
	if code := run(nil, strings.NewReader(`{"origin_ref":"","import_paths":[]}`), failedIO{}, io.Discard); code != 1 {
		t.Fatal(code)
	}
	if code := run([]string{"unexpected"}, strings.NewReader(""), io.Discard, io.Discard); code != 2 {
		t.Fatal(code)
	}
}

// Every frozen native case also crosses the CLI boundary unchanged, including
// opaque fixture origins, duplicate paths and trailing-space observations.
func TestFrozenNativeCasesThroughCLI(t *testing.T) {
	raw, err := os.ReadFile("../../examples/runtime-package-surface/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]json.RawMessage `json:"inputs"`
			Expected map[string]json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 14 {
		t.Fatalf("unexpected frozen corpus: %d", len(corpus.Cases))
	}
	const entry = "example/runtime-package-surface:Inspect"
	for i, fixture := range corpus.Cases {
		var out, diagnostics bytes.Buffer
		if code := run(nil, bytes.NewReader(fixture.Inputs[entry]), &out, &diagnostics); code != 0 {
			t.Fatalf("case %d: code %d: %s", i, code, &diagnostics)
		}
		var actual, expected any
		if err := json.Unmarshal(out.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(fixture.Expected[entry], &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("case %d differs from frozen native expectation: %s", i, &out)
		}
	}
}
