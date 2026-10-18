package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/internal/urlcomponentinput"
)

func TestCommandPreservesTrustedInputOnlyHandoff(t *testing.T) {
	var output bytes.Buffer
	var handed []byte
	if err := run(strings.NewReader(`{"text":" a+b/%20\u0000한 "}`), &output, func(raw []byte) error {
		handed = bytes.Clone(raw)
		clear(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), handed) {
		t.Fatal("callback mutated the emitted input document")
	}
	texts, err := urlcomponentinput.ReadCorpus(&output)
	if err != nil || !reflect.DeepEqual(texts, []string{" a+b/%20\x00한 "}) {
		t.Fatalf("original Text changed: %q %v", texts, err)
	}
}

func TestCommandRefusalNeverWritesOrInvokes(t *testing.T) {
	for _, request := range []string{
		`{"text":null}`, `{"text":"x","text":"y"}`, `{"text":"\ud800"}`,
		`{"text":"x"}{}`, `{"text":"` + strings.Repeat("한", 114) + `"}`,
	} {
		var output bytes.Buffer
		called := false
		if err := run(strings.NewReader(request), &output, func([]byte) error { called = true; return nil }); err == nil || output.Len() != 0 || called {
			t.Fatalf("refusal leaked: %v %q called=%t", err, output.Bytes(), called)
		}
	}
}

func TestCommandIOFailuresStayErrors(t *testing.T) {
	failure := errors.New("handoff refused")
	var output bytes.Buffer
	if err := run(strings.NewReader(`{"text":"ok"}`), &output, func([]byte) error { return failure }); !errors.Is(err, failure) || output.Len() != 0 {
		t.Fatalf("handoff failure: %v %q", err, output.Bytes())
	}
	if err := run(strings.NewReader(`{"text":"ok"}`), nil, nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := run(strings.NewReader(`{"text":"ok"}`), shortWriter{}, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short output: %v", err)
	}
	if err := run(strings.NewReader(`{"text":"ok"}`), failedWriter{failure}, nil); !errors.Is(err, failure) {
		t.Fatalf("failed output: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(raw []byte) (int, error) { return len(raw) - 1, nil }

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }
