package headerinput

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The authoritative pinned839 decoder uses schema+inputs, with qualified keys
// directly in each row. Bind Prepare to both the documented HeaderFacts input
// fixture and the already existing PathAliasFacts fixture. This is a wire-shape
// regression check; it does not replace the actual Gooo CLI integration test.
func TestPrepareMatchesSourceBoundDocumentedInputFixtures(t *testing.T) {
	for _, fixture := range []struct{ path, entry, sha string }{
		{"../../examples/header-field-facts/inputs.json", "app/header-field-facts:Report", "4c0a1eb41d2b49ed0d852dfefc743bf20965ff6dce8c64753386200608f9b2b1"},
		{"../../examples/path-alias-facts/inputs.json", "app/path-alias-facts:Report", "a57199be50b89d26affaa0993ee2f236777f848d796c2991ea5bd5734f7a4367"},
	} {
		raw, err := os.ReadFile(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != fixture.sha {
			t.Fatal("documented input source fixture changed", fixture.path)
		}
		var documented struct {
			Schema string              `json:"schema"`
			Inputs []map[string]string `json:"inputs"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&documented); err != nil || documented.Schema != "gooo/body-composition-inputs/v1" || len(documented.Inputs) == 0 {
			t.Fatalf("documented input fixture: %v", err)
		}
		for _, row := range documented.Inputs {
			name, hasName := row[fixture.entry+".input0"]
			value, hasValue := row[fixture.entry+".input1"]
			if len(row) != 2 || !hasName || !hasValue {
				t.Fatal("documented qualified input row changed")
			}
			prepared, err := Prepare(strings.NewReader(string(wire(t, Request{name, value}))), nil)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Schema string              `json:"schema"`
				Inputs []map[string]string `json:"inputs"`
			}
			decoder := json.NewDecoder(strings.NewReader(string(prepared)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			want := []map[string]string{{"app/header-field-facts:Report.input0": name, "app/header-field-facts:Report.input1": value}}
			if got.Schema != documented.Schema || !reflect.DeepEqual(got.Inputs, want) {
				t.Fatalf("Prepare disagrees with documented native inputs: %s", prepared)
			}
		}
	}
}

func wire(t *testing.T, request Request) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"name": request.Name, "value": request.ValueText})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDecodeAcceptsTextWithoutHeaderSemantics(t *testing.T) {
	tests := []struct {
		name, raw string
		want      Request
	}{
		{"empty Texts", `{"name":"","value":""}`, Request{}},
		{"ordinary", `{"name":"X-Test","value":"ok"}`, Request{"X-Test", "ok"}},
		{"reversed keys and whitespace", " \n {\"value\": \"ok\",\"name\":\"x\"} \t\r", Request{"x", "ok"}},
		{"decoded escaped keys", `{"\u006eame":"x","valu\u0065":"ok"}`, Request{"x", "ok"}},
		{"header-invalid Text", `{"name":"bad name:","value":"a\r\nb"}`, Request{"bad name:", "a\r\nb"}},
		{"escaped control Text", `{"name":"\u0000","value":"\u001f\u007f"}`, Request{"\x00", "\x1f\x7f"}},
		{"unchanged edge whitespace", `{"name":" X ","value":"\t value \t"}`, Request{" X ", "\t value \t"}},
		{"paired surrogate", `{"name":"\ud83d\ude00","value":"\uD83D\uDE00"}`, Request{"😀", "😀"}},
		{"surrogate extremes", `{"name":"\ud800\udc00","value":"\udbff\udfff"}`, Request{"\U00010000", "\U0010ffff"}},
		{"literal escaped backslash", `{"name":"\\ud800","value":"\\udc00"}`, Request{`\ud800`, `\udc00`}},
		{"escaped quote and backslash", `{"name":"\"","value":"\\\""}`, Request{"\"", "\\\""}},
		{"intentional replacement character", `{"name":"�","value":"\ufffd"}`, Request{"�", "�"}},
		{"nonASCII and no normalization", `{"name":"A가","value":"éé"}`, Request{"A가", "éé"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Decode([]byte(test.raw))
			if err != nil || got != test.want {
				t.Fatalf("got %#v,%v; want %#v", got, err, test.want)
			}
		})
	}
}

func TestDecodedBoundIsUTF8BytesOnBothFields(t *testing.T) {
	for _, side := range []string{"name", "value"} {
		for _, text := range []string{
			strings.Repeat("a", 1023), strings.Repeat("a", 1024),
			strings.Repeat("é", 512), strings.Repeat("가", 341) + "a", strings.Repeat("😀", 256),
		} {
			request := Request{"x", "ok"}
			if side == "name" {
				request.Name = text
			} else {
				request.ValueText = text
			}
			got, err := Decode(wire(t, request))
			if err != nil || got != request {
				t.Fatalf("accepted %s boundary %d bytes: %#v,%v", side, len(text), got, err)
			}
		}
		for _, text := range []string{strings.Repeat("a", 1025), strings.Repeat("é", 512) + "a", strings.Repeat("😀", 256) + "a"} {
			request := Request{"x", "ok"}
			if side == "name" {
				request.Name = text
			} else {
				request.ValueText = text
			}
			got, err := Decode(wire(t, request))
			if !errors.Is(err, ErrTextTooLarge) || got != (Request{}) {
				t.Fatalf("refused %s %d bytes: %#v,%v", side, len(text), got, err)
			}
		}
	}
	// Both1024-byte strings may expand sixfold on the wire without becoming
	// oversized decoded Text; even fully escaped decoded member names are valid.
	raw := []byte(`{"\u006e\u0061\u006d\u0065":"` + strings.Repeat(`\u0000`, 1024) + `","\u0076\u0061\u006c\u0075\u0065":"` + strings.Repeat(`\u001f`, 1024) + `"}`)
	got, err := Decode(raw)
	if err != nil || got.Name != strings.Repeat("\x00", 1024) || got.ValueText != strings.Repeat("\x1f", 1024) {
		t.Fatalf("sixfold JSON escaping: %#v,%v", got, err)
	}
}

func TestDecodeRefusesMalformedShapeDuplicatesAndSurrogates(t *testing.T) {
	tests := []struct {
		name, raw string
		want      error
	}{
		{"empty", ``, ErrMalformedJSON},
		{"truncated", `{"name":"x","value":`, ErrMalformedJSON},
		{"trailing object", `{"name":"x","value":"ok"}{}`, ErrMalformedJSON},
		{"trailing scalar", `{"name":"x","value":"ok"} true`, ErrMalformedJSON},
		{"trailing garbage", `{"name":"x","value":"ok"}x`, ErrMalformedJSON},
		{"comment", `{"name":"x","value":"ok"}/*x*/`, ErrMalformedJSON},
		{"trailing comma", `{"name":"x","value":"ok",}`, ErrMalformedJSON},
		{"bad escape", `{"name":"\x41","value":"ok"}`, ErrMalformedJSON},
		{"short unicode escape", `{"name":"\u123","value":"ok"}`, ErrMalformedJSON},
		{"invalid hex", `{"name":"\uZZZZ","value":"ok"}`, ErrMalformedJSON},
		{"unescaped newline", "{\"name\":\"a\nb\",\"value\":\"ok\"}", ErrMalformedJSON},
		{"array", `[]`, ErrObjectRequired},
		{"null object", `null`, ErrObjectRequired},
		{"scalar object", `"text"`, ErrObjectRequired},
		{"missing both", `{}`, ErrMissingField},
		{"missing value", `{"name":"x"}`, ErrMissingField},
		{"missing name", `{"value":"ok"}`, ErrMissingField},
		{"extra validity is not request", `{"name":"x","value":"ok","name_valid":true}`, ErrUnknownField},
		{"extra diagnostic is not request", `{"name":"x","value":"ok","name_invalid_offset":-1}`, ErrUnknownField},
		{"case mismatch", `{"Name":"x","value":"ok"}`, ErrUnknownField},
		{"host identifier is not wire key", `{"name":"x","valueText":"ok"}`, ErrUnknownField},
		{"native ports are not wire keys", `{"input0":"x","input1":"ok"}`, ErrUnknownField},
		{"corpus envelope is not request", `{"schema":"gooo/header-native-inputs/v1","names":[],"values":[],"pairs":[]}`, ErrUnknownField},
		{"duplicate name", `{"name":"x","name":"x","value":"ok"}`, ErrDuplicateField},
		{"duplicate value", `{"name":"x","value":"ok","value":"ok"}`, ErrDuplicateField},
		{"escaped duplicate name", `{"name":"x","\u006eame":"y","value":"ok"}`, ErrDuplicateField},
		{"escaped duplicate value", `{"name":"x","value":"ok","valu\u0065":"bad"}`, ErrDuplicateField},
		{"lone high", `{"name":"\ud800","value":"ok"}`, ErrUnpairedSurrogate},
		{"lone low", `{"name":"x","value":"\udfff"}`, ErrUnpairedSurrogate},
		{"high followed by high", `{"name":"\ud800\udbff","value":"ok"}`, ErrUnpairedSurrogate},
		{"reversed pair", `{"name":"\udc00\ud800","value":"ok"}`, ErrUnpairedSurrogate},
		{"high followed by ASCII escape", `{"name":"\ud800\u0041","value":"ok"}`, ErrUnpairedSurrogate},
		{"nonadjacent surrogate pair", `{"name":"\ud800 \udc00","value":"ok"}`, ErrUnpairedSurrogate},
		{"high then escaped backslash", `{"name":"\ud800\\udc00","value":"ok"}`, ErrUnpairedSurrogate},
		{"paired then lone low", `{"name":"x","value":"\ud83d\ude00\udc00"}`, ErrUnpairedSurrogate},
		{"surrogate member name", `{"\ud800":"x","value":"ok"}`, ErrUnpairedSurrogate},
	}
	for _, side := range []string{"name", "value"} {
		other := "value"
		if side == "value" {
			other = "name"
		}
		for _, value := range []string{"null", "false", "true", "0", "1.25", "1e9999", "[]", "{}"} {
			tests = append(tests, struct {
				name, raw string
				want      error
			}{side + " type " + value, fmt.Sprintf(`{%q:%s,%q:"ok"}`, side, value, other), ErrTextRequired})
		}
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Decode([]byte(test.raw))
			if !errors.Is(err, test.want) || got != (Request{}) {
				t.Fatalf("got %#v,%v; want zero,%v", got, err, test.want)
			}
		})
	}
}

func TestInvalidRawUTF8NeverBecomesReplacementText(t *testing.T) {
	for _, invalid := range [][]byte{{0xff}, {0xc0, 0xaf}, {0xed, 0xa0, 0x80}, {0xc2}, {0xf4, 0x90, 0x80, 0x80}} {
		raw := append([]byte(`{"name":"x","value":"`), invalid...)
		raw = append(raw, []byte(`"}`)...)
		got, err := Decode(raw)
		if !errors.Is(err, ErrInvalidUTF8) || got != (Request{}) {
			t.Fatalf("invalid raw bytes %x: %#v,%v", invalid, got, err)
		}
	}
}

type countedReader struct {
	consumed int
	reader   io.Reader
}

func (r *countedReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.consumed += n
	return n, err
}

type failedReader struct{ err error }
type completeThenErrorReader struct {
	raw []byte
	err error
}

func (r *completeThenErrorReader) Read(buffer []byte) (int, error) {
	n := copy(buffer, r.raw)
	r.raw = r.raw[n:]
	return n, r.err
}
func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

func TestRawCapReadLimitAndReadErrors(t *testing.T) {
	base := `{"name":"x","value":"ok"}`
	exact := base + strings.Repeat(" ", MaxRawBytes-len(base))
	for _, read := range []bool{false, true} {
		var got Request
		var err error
		if read {
			got, err = Read(strings.NewReader(exact))
		} else {
			got, err = Decode([]byte(exact))
		}
		if err != nil || got != (Request{"x", "ok"}) {
			t.Fatalf("exact raw cap read=%v: %#v,%v", read, got, err)
		}
	}
	oversized := &countedReader{reader: strings.NewReader(exact + strings.Repeat(" ", 1000))}
	got, err := Read(oversized)
	if !errors.Is(err, ErrRawTooLarge) || got != (Request{}) || oversized.consumed != MaxRawBytes+1 {
		t.Fatalf("bounded read: %#v,%v,%d", got, err, oversized.consumed)
	}
	if got, err := Decode([]byte(exact + " ")); !errors.Is(err, ErrRawTooLarge) || got != (Request{}) {
		t.Fatalf("raw cap+1: %#v,%v", got, err)
	}
	failure := errors.New("reader failed")
	if got, err := Read(failedReader{failure}); !errors.Is(err, failure) || got != (Request{}) {
		t.Fatalf("read failure: %#v,%v", got, err)
	}
	if got, err := Read(&completeThenErrorReader{[]byte(base), failure}); !errors.Is(err, failure) || got != (Request{}) {
		t.Fatalf("read error despite complete valid payload: %#v,%v", got, err)
	}
	if got, err := Read(nil); err == nil || got != (Request{}) {
		t.Fatalf("nil reader: %#v,%v", got, err)
	}
}

func TestAllFrozenReportPairsPreserveDecodedTexts(t *testing.T) {
	// The repository corpus is a trusted test fixture, not the external wire request.
	path := "../../examples/header-field-facts/corpus.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "16d88045a78fe43f239f7b22ad12635a852529abb74733aa82b5073fdd892053" {
		t.Fatal("frozen fixture SHA changed")
	}
	var corpus struct {
		Schema string   `json:"schema"`
		Names  []string `json:"names"`
		Values []string `json:"values"`
		Pairs  []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Schema != "gooo/header-native-inputs/v1" || len(corpus.Names) != 232 || len(corpus.Values) != 232 || len(corpus.Pairs) != 476 {
		t.Fatal("frozen corpus roster changed")
	}
	for index, pair := range corpus.Pairs {
		want := Request{pair.Name, pair.Value}
		got, err := Decode(wire(t, want))
		if err != nil || got != want {
			t.Fatalf("pair%d: %#v,%v; want %#v", index, got, err, want)
		}
	}
	t.Logf("frozen report pairs decoded without changing either Text: %d", len(corpus.Pairs))
}
