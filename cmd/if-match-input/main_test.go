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
		`{"if_match":"","current_etag":""}`,
		`{"if_match":"bad name:","current_etag":"a\r\nb"}`,
		`{"current_etag":"\u0000가\t","if_\u006datch":"A가"}`,
		`{"if_match":"\\ud800","current_etag":"\ud83d\ude00"}`,
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
		if got.Schema != "gooo/body-composition-inputs/v1" || len(got.Inputs) != 1 || len(got.Inputs[0]) != 2 || got.Inputs[0]["go/net/http/precondition:IfMatchCondition.input0"] != original["if_match"] || got.Inputs[0]["go/net/http/precondition:IfMatchCondition.input1"] != original["current_etag"] || out.Bytes()[out.Len()-1] != '\n' {
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
		[]byte(`{"if_match":"x","current_etag":`),
		[]byte(`{"if_match":"x","current_etag":"ok"}{}`),
		[]byte(`{"if_match":"x","if_\u006datch":"y","current_etag":"ok"}`),
		[]byte(`{"if_match":"x","current_etag":"ok","current_etag":"ok"}`),
		[]byte(`{"if_match":"x"}`), []byte(`{"if_match":null,"current_etag":"ok"}`),
		[]byte(`{"if_match":"x","current_etag":false}`), []byte(`{"if_match":"x","current_etag":[]}`),
		[]byte(`{"if_match":"x","valueText":"ok"}`), []byte(`{"if_match":"x","current_etag":"ok","condition":true}`),
		[]byte(`{"if_match":"\ud800","current_etag":"ok"}`), []byte(`{"if_match":"x","current_etag":"\udc00"}`),
		[]byte(`{"if_match":"` + strings.Repeat("a", 1025) + `","current_etag":"ok"}`),
		[]byte(`{"if_match":"x","current_etag":"` + strings.Repeat("é", 512) + `a"}`),
		[]byte(`{"if_match":"x","current_etag":"ok"}` + strings.Repeat(" ", 16<<10)),
		append(append([]byte(`{"if_match":"x","current_etag":"`), 0xff), []byte(`"}`)...),
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
	if err := run(strings.NewReader(`{"if_match":"x","current_etag":"ok"}`), &out, func([]byte) error { return failure }); !errors.Is(err, failure) || out.Len() != 0 {
		t.Fatalf("callback failure: %v %q", err, out.Bytes())
	}
	if err := run(strings.NewReader(`{"if_match":"x","current_etag":"ok"}`), nil, nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := run(strings.NewReader(`{"if_match":"x","current_etag":"ok"}`), shortWriter{}, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short output: %v", err)
	}
	if err := run(strings.NewReader(`{"if_match":"x","current_etag":"ok"}`), failedWriter{failure}, nil); !errors.Is(err, failure) {
		t.Fatalf("failed output: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }
