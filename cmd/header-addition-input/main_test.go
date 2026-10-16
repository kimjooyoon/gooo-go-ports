package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

const valid = `{"name":"x-request-id","value":"a\r\nb","declared_length":"-2","facts":{"content_length_occurrences":-1,"observed_body_bytes":0,"maximum_payload_bytes":0,"transfer_encoding_present":false,"body_observation_bound_to_request":false,"current_access":false,"candidate_name_occurrences":0,"header_set_bound_to_request":false,"policy_revision_current":false,"allowed_header_name":"authorization","expected_request_revision":9223372036854775807,"observed_request_revision":-9223372036854775808}}`

func TestSemanticRefusalsStillProduceInputDocument(t *testing.T) {
	var out bytes.Buffer
	if err := run(strings.NewReader(valid), &out); err != nil || out.Len() == 0 {
		t.Fatal("semantic input was not handed through", err)
	}
	for _, forbidden := range []string{`"cases":`, `"expected":`, `"eligible":`, `"status":`} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatal("adapter fabricated policy output", forbidden)
		}
	}
	if !strings.Contains(out.String(), `"schema":"gooo/body-composition-inputs/v1"`) || !strings.Contains(out.String(), `"inputs":[{`) || !strings.Contains(out.String(), `"observed_request_revision":-9223372036854775808`) || out.Bytes()[out.Len()-1] != '\n' {
		t.Fatal("wrong native document", out.String())
	}
}

func TestTransportRefusalNeverWritesPartialDocument(t *testing.T) {
	for _, request := range []string{`{}`, valid + `{}`, strings.Replace(valid, `"current_access":false`, `"current_access":null`, 1), strings.Replace(valid, `"observed_body_bytes":0`, `"observed_body_bytes":1e0`, 1), strings.Replace(valid, `"name":"x-request-id"`, `"name":"\ud800"`, 1)} {
		var out bytes.Buffer
		if err := run(strings.NewReader(request), &out); err == nil || out.Len() != 0 {
			t.Fatal("refusal emitted input bytes", err, out.String())
		}
	}
}

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestOutputErrorsRemainErrors(t *testing.T) {
	if err := run(strings.NewReader(valid), nil); err == nil {
		t.Fatal("nil output")
	}
	if err := run(strings.NewReader(valid), shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	failure := errors.New("disk full")
	if err := run(strings.NewReader(valid), failedWriter{failure}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
