package urlcomponentinput

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestOriginalTextSurvivesStrictHandoff(t *testing.T) {
	for _, input := range []struct{ name, raw, text string }{
		{"empty", `{"text":""}`, ""},
		{"preserved syntax", `{"text":"  a+b/%20  "}`, "  a+b/%20  "},
		{"decoded key", `{"te\u0078t":"\u0000\n\u007f"}`, "\x00\n\x7f"},
		{"Unicode", `{"text":"é/한글🚀e\u0301"}`, "é/한글🚀e\u0301"},
		{"surrogate pair", `{"text":"\ud83d\uDE80"}`, "🚀"},
		{"scalar extremes", `{"text":"\ud800\udc00\udbff\udfff"}`, "\U00010000\U0010ffff"},
		{"literal escape", `{"text":"\\ud800"}`, `\ud800`},
		{"replacement scalar", `{"text":"\ufffd"}`, "\ufffd"},
		{"341 ASCII bytes", `{"text":"` + strings.Repeat("/", 341) + `"}`, strings.Repeat("/", 341)},
		{"341 Unicode bytes", `{"text":"` + strings.Repeat("한", 113) + "é" + `"}`, strings.Repeat("한", 113) + "é"},
	} {
		t.Run(input.name, func(t *testing.T) {
			var handed []byte
			calls := 0
			raw, err := Prepare(strings.NewReader(input.raw), func(value []byte) error {
				calls++
				handed = bytes.Clone(value)
				clear(value)
				return nil
			})
			if err != nil || calls != 1 || !bytes.Equal(raw, handed) || raw[len(raw)-1] != '\n' {
				t.Fatalf("handoff changed: err=%v calls=%d", err, calls)
			}
			var document map[string]json.RawMessage
			if err := json.Unmarshal(raw, &document); err != nil || len(document) != 2 || document["cases"] != nil || document["expected"] != nil {
				t.Fatalf("not an input-only document: %s (%v)", raw, err)
			}
			texts, err := ReadCorpus(bytes.NewReader(raw))
			if err != nil || !reflect.DeepEqual(texts, []string{input.text}) {
				t.Fatalf("original Text changed: %q (%v)", texts, err)
			}
		})
	}
}

func TestRefusedRequestsHaveNoDocumentOrHandoff(t *testing.T) {
	for _, input := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"malformed", []byte(`{"text":`), ErrMalformedJSON},
		{"trailing object", []byte(`{"text":"ok"}{}`), ErrMalformedJSON},
		{"array", []byte(`[]`), ErrObjectRequired},
		{"null root", []byte(`null`), ErrObjectRequired},
		{"missing", []byte(`{}`), ErrMissingField},
		{"unknown", []byte(`{"text":"x","path_segment":"x"}`), ErrUnknownField},
		{"duplicate", []byte(`{"text":"x","text":"y"}`), ErrDuplicateField},
		{"escaped duplicate", []byte(`{"text":"x","te\u0078t":"x"}`), ErrDuplicateField},
		{"null", []byte(`{"text":null}`), ErrTextRequired},
		{"number", []byte(`{"text":12}`), ErrTextRequired},
		{"Boolean", []byte(`{"text":false}`), ErrTextRequired},
		{"object value", []byte(`{"text":{}}`), ErrTextRequired},
		{"array value", []byte(`{"text":[]}`), ErrTextRequired},
		{"raw invalid UTF8", append(append([]byte(`{"text":"`), 0xff), []byte(`"}`)...), ErrInvalidUTF8},
		{"high surrogate", []byte(`{"text":"\ud800"}`), ErrUnpairedSurrogate},
		{"low surrogate", []byte(`{"text":"\udc00"}`), ErrUnpairedSurrogate},
		{"separated pair", []byte(`{"text":"\ud800 \udc00"}`), ErrUnpairedSurrogate},
		{"surrogate key", []byte(`{"\ud800":"x"}`), ErrUnpairedSurrogate},
		{"342 ASCII bytes", []byte(`{"text":"` + strings.Repeat("a", 342) + `"}`), ErrTextTooLarge},
		{"342 Unicode bytes", []byte(`{"text":"` + strings.Repeat("한", 114) + `"}`), ErrTextTooLarge},
		{"raw bound", []byte(`{"text":""}` + strings.Repeat(" ", MaxRawBytes)), ErrRawTooLarge},
		{"deep refused value", []byte(`{"text":` + strings.Repeat("[", 10) + `0` + strings.Repeat("]", 10) + `}`), ErrMalformedJSON},
	} {
		t.Run(input.name, func(t *testing.T) {
			calls := 0
			raw, err := Prepare(bytes.NewReader(input.raw), func([]byte) error { calls++; return nil })
			if !errors.Is(err, input.want) || raw != nil || calls != 0 {
				t.Fatalf("refusal: err=%v want=%v raw=%q calls=%d", err, input.want, raw, calls)
			}
		})
	}
}

func TestReadBoundsAndErrorsBeforeHandoff(t *testing.T) {
	valid := `{"text":""}`
	exact := valid + strings.Repeat(" ", MaxRawBytes-len(valid))
	if got, err := Read(strings.NewReader(exact)); err != nil || got != (Request{}) {
		t.Fatalf("exact raw bound refused: %#v %v", got, err)
	}
	reader := &countingReader{reader: strings.NewReader(exact + " " + strings.Repeat("x", 100))}
	if got, err := Read(reader); !errors.Is(err, ErrRawTooLarge) || got != (Request{}) || reader.consumed != MaxRawBytes+1 {
		t.Fatalf("read not bounded: got=%#v err=%v consumed=%d", got, err, reader.consumed)
	}
	failure := errors.New("read failed")
	if raw, err := Prepare(failingReader{failure}, func([]byte) error { t.Fatal("handoff after read failure"); return nil }); !errors.Is(err, failure) || raw != nil {
		t.Fatalf("read failure lost: %q %v", raw, err)
	}
	if raw, err := Prepare(strings.NewReader(valid), func([]byte) error { return failure }); !errors.Is(err, failure) || raw != nil {
		t.Fatalf("handoff failure lost: %q %v", raw, err)
	}
	if _, err := Read(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
}

func TestCorpusIsExactPublicEntryAndRetainsAllRows(t *testing.T) {
	texts := []string{"", " a+b/%20 ", "\x00", strings.Repeat("/", MaxTextBytes)}
	rows := make([]map[string]string, len(texts))
	for i, text := range texts {
		rows[i] = map[string]string{Entry: text}
	}
	raw, err := json.Marshal(map[string]any{"schema": Schema, "inputs": rows})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadCorpus(bytes.NewReader(raw)); err != nil || !reflect.DeepEqual(got, texts) {
		t.Fatalf("corpus identity: %q %v", got, err)
	}
	rows = make([]map[string]string, MaxRows)
	for i := range rows {
		rows[i] = map[string]string{Entry: ""}
	}
	raw, _ = json.Marshal(map[string]any{"schema": Schema, "inputs": rows})
	if got, err := ReadCorpus(bytes.NewReader(raw)); err != nil || len(got) != MaxRows {
		t.Fatalf("128 original rows: %d %v", len(got), err)
	}
	rows = append(rows, map[string]string{Entry: ""})
	raw, _ = json.Marshal(map[string]any{"schema": Schema, "inputs": rows})
	if got, err := ReadCorpus(bytes.NewReader(raw)); !errors.Is(err, ErrInputRows) || got != nil {
		t.Fatalf("129 rows accepted: %q %v", got, err)
	}
}

func TestCorpusRefusesAmbiguousOrOutOfDomainInputs(t *testing.T) {
	good := `{"schema":"` + Schema + `","inputs":[{"` + Entry + `":"ok"}]}`
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"schema":"wrong","inputs":[]}`,
		`{"schema":"` + Schema + `","inputs":[],"extra":true}`,
		`{"schema":"` + Schema + `","schema":"` + Schema + `","inputs":[]}`,
		`{"schema":"` + Schema + `","inputs":[]}`,
		`{"schema":"` + Schema + `","inputs":null}`,
		`{"schema":"` + Schema + `","inputs":[{}]}`,
		`{"schema":"` + Schema + `","inputs":[{"app/url-context:Report":"x"}]}`,
		`{"schema":"` + Schema + `","inputs":[{"` + Entry + `":null}]}`,
		`{"schema":"` + Schema + `","inputs":[{"` + Entry + `":"x","app/url-context-facts:Repor\u0074":"x"}]}`,
		strings.Replace(good, `"ok"`, `"\ud800"`, 1),
		strings.Replace(good, `"ok"`, `"`+strings.Repeat("x", 342)+`"`, 1),
		good + `{}`,
	} {
		if got, err := ReadCorpus(strings.NewReader(raw)); err == nil || got != nil {
			t.Fatalf("invalid corpus accepted: %s => %q %v", raw, got, err)
		}
	}
	reader := &countingReader{reader: strings.NewReader(good + strings.Repeat(" ", MaxCorpusRawBytes))}
	if _, err := ReadCorpus(reader); !errors.Is(err, ErrRawTooLarge) || reader.consumed != MaxCorpusRawBytes+1 {
		t.Fatalf("corpus read bound: %d %v", reader.consumed, err)
	}
}

type countingReader struct {
	reader   io.Reader
	consumed int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.consumed += n
	return n, err
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }
