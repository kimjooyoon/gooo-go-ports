package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/internal/urlcomponentinput"
)

func TestIndependentOriginalGoAtRealContextBoundaries(t *testing.T) {
	if err := originalGoIdentity(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []facts{
		{"", "", "", 0, 0, 0, false, false, false, -1},
		{"a b", "a%20b", "a+b", 3, 5, 3, true, true, true, -1},
		{"a+b", "a+b", "a%2Bb", 3, 3, 5, false, true, true, -1},
		{"$&+:=@", "$&+:=@", "%24%26%2B%3A%3D%40", 6, 6, 18, false, true, true, -1},
		{"%20", "%2520", "%2520", 3, 5, 5, true, true, false, 0},
		{"é/한", "%C3%A9%2F%ED%95%9C", "%C3%A9%2F%ED%95%9C", 6, 18, 18, true, true, false, -1},
		{"\x00\n\x7f", "%00%0A%7F", "%00%0A%7F", 3, 9, 9, true, true, false, -1},
		{"한%", "%ED%95%9C%25", "%ED%95%9C%25", 4, 12, 12, true, true, false, 3},
		{strings.Repeat("/", 341), strings.Repeat("%2F", 341), strings.Repeat("%2F", 341), 341, 1023, 1023, true, true, false, -1},
	} {
		if got := reference(want.Original); got != want {
			t.Fatalf("original Go differs at %q: got=%+v want=%+v", want.Original, got, want)
		}
	}
}

func TestAdapterToOracleRetainsOriginalInputsAndTenTypedFields(t *testing.T) {
	request := `{"text":"  a+b/%20\u0000한  "}`
	raw, err := urlcomponentinput.Prepare(strings.NewReader(request), nil)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "inputs.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"--inputs", input}, &output); err != nil {
		t.Fatal(err)
	}
	var got []facts
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil || len(got) != 1 {
		t.Fatalf("oracle document: %v %s", err, output.Bytes())
	}
	if got[0].Original != "  a+b/%20\x00한  " || got[0] != reference(got[0].Original) {
		t.Fatalf("pipeline original changed: %+v", got)
	}
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &fields); err != nil || len(fields[0]) != 10 || output.Bytes()[output.Len()-1] != '\n' {
		t.Fatalf("ten-field array contract: %v %s", err, output.Bytes())
	}
}

func TestOracleRefusesWholeCorpusBeforeWriting(t *testing.T) {
	base := `{"schema":"` + urlcomponentinput.Schema + `","inputs":[{"` + urlcomponentinput.Entry + `":"ok"}]}`
	for _, raw := range []string{
		`{}`, base + `{}`,
		strings.Replace(base, `"ok"`, `"\udc00"`, 1),
		strings.Replace(base, `"ok"`, `"`+strings.Repeat("a", 342)+`"`, 1),
		strings.Replace(base, `"ok"`, `null`, 1),
		strings.Replace(base, `:Report`, `:Report.input0`, 1),
	} {
		input := filepath.Join(t.TempDir(), "inputs.json")
		if err := os.WriteFile(input, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := run([]string{"--inputs", input}, &output); err == nil || output.Len() != 0 {
			t.Fatalf("oracle refusal leaked: %v %s", err, output.Bytes())
		}
	}
}

func TestOracleArgumentsAndOutputFailures(t *testing.T) {
	for _, args := range [][]string{nil, {"--inputs", ""}, {"--unknown"}, {"--inputs", "x", "extra"}} {
		var output bytes.Buffer
		if err := run(args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("arguments accepted: %v (%v)", args, err)
		}
	}
	var output bytes.Buffer
	if err := run([]string{"--inputs", filepath.Join(t.TempDir(), "missing")}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("missing input accepted: %v", err)
	}
	if err := run(nil, nil); err == nil {
		t.Fatal("nil writer accepted")
	}
	raw, err := urlcomponentinput.Prepare(strings.NewReader(`{"text":""}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "inputs.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--inputs", input}, shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short output: %v", err)
	}
	failure := errors.New("output refused")
	if err := run([]string{"--inputs", input}, failedWriter{failure}); !errors.Is(err, failure) {
		t.Fatalf("output failure lost: %v", err)
	}
	if !reflect.DeepEqual(reference("abc-_.~"), facts{"abc-_.~", "abc-_.~", "abc-_.~", 7, 7, 7, false, false, false, -1}) {
		t.Fatal("unreserved identity changed")
	}
}

type shortWriter struct{}

func (shortWriter) Write(raw []byte) (int, error) { return len(raw) - 1, nil }

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }
