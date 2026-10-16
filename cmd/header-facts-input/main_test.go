package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRequestPreflightProducesOnlyTrustedOriginalTextInputs(t *testing.T) {
	for _, request := range []string{
		`{"name":"","value":""}`,
		`{"name":"bad name:","value":"a\r\nb"}`,
		`{"value":"\u0000가\t","\u006eame":"A가"}`,
		`{"name":"\\ud800","value":"\ud83d\ude00"}`,
	} {
		var out bytes.Buffer
		called := 0
		var handed []byte
		err := run(strings.NewReader(request), &out, func(raw []byte) error {
			called++
			handed = append([]byte(nil), raw...)
			for i := range raw {
				raw[i] = 'x'
			}
			return nil
		})
		if err != nil || called != 1 || !bytes.Equal(out.Bytes(), handed) {
			t.Fatalf("handoff: %v called=%d", err, called)
		}
		var original map[string]string
		if err := json.Unmarshal([]byte(request), &original); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Schema string              `json:"schema"`
			Inputs []map[string]string `json:"inputs"`
		}
		decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Schema != "gooo/body-composition-inputs/v1" || len(got.Inputs) != 1 || len(got.Inputs[0]) != 2 || got.Inputs[0]["app/header-field-facts:Report.input0"] != original["name"] || got.Inputs[0]["app/header-field-facts:Report.input1"] != original["value"] || out.Bytes()[out.Len()-1] != '\n' {
			t.Fatalf("changed input-only document: %s", out.Bytes())
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &top); err != nil || len(top) != 2 || top["schema"] == nil || top["inputs"] == nil || top["cases"] != nil || top["expected"] != nil {
			t.Fatalf("input-only wire keys differ from pinned decoder: %s", out.Bytes())
		}
	}
}

func TestRefusalWritesNothingAndNeverCallsOptionalHandoff(t *testing.T) {
	requests := [][]byte{
		[]byte(`{"name":"x","value":`),
		[]byte(`{"name":"x","value":"ok"}{}`),
		[]byte(`{"name":"x","\u006eame":"y","value":"ok"}`),
		[]byte(`{"name":"x","value":"ok","value":"ok"}`),
		[]byte(`{"name":"x"}`), []byte(`{"name":null,"value":"ok"}`),
		[]byte(`{"name":"x","value":false}`), []byte(`{"name":"x","value":[]}`),
		[]byte(`{"name":"x","valueText":"ok"}`), []byte(`{"name":"x","value":"ok","name_valid":true}`),
		[]byte(`{"name":"\ud800","value":"ok"}`), []byte(`{"name":"x","value":"\udc00"}`),
		[]byte(`{"name":"` + strings.Repeat("a", 1025) + `","value":"ok"}`),
		[]byte(`{"name":"x","value":"` + strings.Repeat("é", 512) + `a"}`),
		[]byte(`{"name":"x","value":"ok"}` + strings.Repeat(" ", 16<<10)),
		append(append([]byte(`{"name":"x","value":"`), 0xff), []byte(`"}`)...),
	}
	for _, raw := range requests {
		var out bytes.Buffer
		called := 0
		if err := run(bytes.NewReader(raw), &out, func([]byte) error { called++; return nil }); err == nil || out.Len() != 0 || called != 0 {
			t.Fatalf("refusal leaked document/callback: err=%v bytes=%d calls=%d", err, out.Len(), called)
		}
	}
}

func TestHandoffAndOutputFailuresRemainErrors(t *testing.T) {
	failure := errors.New("handoff refused")
	var out bytes.Buffer
	if err := run(strings.NewReader(`{"name":"x","value":"ok"}`), &out, func([]byte) error { return failure }); !errors.Is(err, failure) || out.Len() != 0 {
		t.Fatalf("callback failure: %v %q", err, out.Bytes())
	}
	if err := run(strings.NewReader(`{"name":"x","value":"ok"}`), nil, nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := run(strings.NewReader(`{"name":"x","value":"ok"}`), shortWriter{}, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short output: %v", err)
	}
	if err := run(strings.NewReader(`{"name":"x","value":"ok"}`), failedWriter{failure}, nil); !errors.Is(err, failure) {
		t.Fatalf("failed output: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }
