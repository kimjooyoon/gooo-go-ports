package headeradditioninput

import (
	"bytes"
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

func baseRequest() Request {
	return Request{Name: "x-request-id", Value: "req-123", DeclaredLength: "2", Facts: Facts{
		ContentLengthOccurrences: 1, ObservedBodyBytes: 2, MaximumPayloadBytes: 2,
		BodyObservationBoundToRequest: true, CurrentAccess: true,
		HeaderSetBoundToRequest: true, PolicyRevisionCurrent: true,
		AllowedHeaderName: "x-request-id", ExpectedRequestRevision: 1, ObservedRequestRevision: 1,
	}}
}

func wire(t *testing.T, request Request) []byte {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func changed(t *testing.T, original []byte, facts bool, key string, replacement []byte) []byte {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(original, &top); err != nil {
		t.Fatal(err)
	}
	object := top
	if facts {
		object = nil
		if err := json.Unmarshal(top["facts"], &object); err != nil {
			t.Fatal(err)
		}
	}
	if replacement == nil {
		delete(object, key)
	} else {
		object[key] = replacement
	}
	if facts {
		raw, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		top["facts"] = raw
	}
	raw, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func expectRefusal(t *testing.T, raw []byte, want error) {
	t.Helper()
	got, err := Decode(raw)
	if !errors.Is(err, want) || got != (Request{}) {
		t.Fatalf("got %#v,%v; want zero,%v", got, err, want)
	}
	if trusted, err := Prepare(bytes.NewReader(raw)); err == nil || trusted != nil {
		t.Fatalf("refusal emitted trusted document: %q,%v", trusted, err)
	}
}

func TestEveryFieldRequiredAndExactObjectShape(t *testing.T) {
	raw := wire(t, baseRequest())
	for _, key := range []string{"name", "value", "declared_length", "facts"} {
		t.Run("missing-top-"+key, func(t *testing.T) { expectRefusal(t, changed(t, raw, false, key, nil), ErrMissingField) })
	}
	typeOfFacts := reflect.TypeOf(Facts{})
	for i := 0; i < typeOfFacts.NumField(); i++ {
		key := typeOfFacts.Field(i).Tag.Get("json")
		t.Run("missing-fact-"+key, func(t *testing.T) { expectRefusal(t, changed(t, raw, true, key, nil), ErrMissingField) })
	}
	for _, facts := range []bool{false, true} {
		for _, key := range []string{"extra", "Name", "expected", "eligible", "input3"} {
			expectRefusal(t, changed(t, raw, facts, key, []byte("true")), ErrUnknownField)
		}
	}
	for _, bad := range []string{"null", "[]", `"facts"`, "false", "1"} {
		expectRefusal(t, changed(t, raw, false, "facts", []byte(bad)), ErrObjectRequired)
	}
	for _, bad := range []string{"null", "[]", `"request"`, "false", "1"} {
		expectRefusal(t, []byte(bad), ErrObjectRequired)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	duplicates := []string{
		strings.Replace(string(raw), `"name":`, `"name":"first","name":`, 1),
		strings.Replace(string(raw), `"name":`, `"name":"first","na\u006de":`, 1),
		strings.Replace(string(raw), `"facts":`, `"facts":`+string(top["facts"])+`,"f\u0061cts":`, 1),
		strings.Replace(string(raw), `"current_access":true`, `"current_access":false,"current_access":true`, 1),
		strings.Replace(string(raw), `"current_access":true`, `"current_access":false,"current_\u0061ccess":true`, 1),
	}
	for _, duplicate := range duplicates {
		expectRefusal(t, []byte(duplicate), ErrDuplicateField)
	}
}

func TestExactScalarTypesAndSigned64LexicalDomain(t *testing.T) {
	raw := wire(t, baseRequest())
	typ := reflect.TypeOf(Facts{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		key := f.Tag.Get("json")
		if f.Type.Kind() == reflect.Int64 {
			for _, valid := range []struct {
				spelling string
				value    int64
			}{
				{"-9223372036854775808", -9223372036854775808}, {"9223372036854775807", 9223372036854775807},
				{"9223372036854775806", 9223372036854775806}, {"9007199254740993", 9007199254740993},
				{"-1", -1}, {"0", 0}, {"-0", 0},
			} {
				got, err := Decode(changed(t, raw, true, key, []byte(valid.spelling)))
				if err != nil || reflect.ValueOf(got.Facts).Field(i).Int() != valid.value {
					t.Fatalf("%s %s: %#v,%v", key, valid.spelling, got, err)
				}
			}
			for _, invalid := range []string{"9223372036854775808", "-9223372036854775809", "1.0", "1e0", "1E0", "1e9999", `"1"`, "true", "null", "[]", "{}"} {
				expectRefusal(t, changed(t, raw, true, key, []byte(invalid)), ErrIntegerRequired)
			}
		} else if f.Type.Kind() == reflect.Bool {
			for _, invalid := range []string{"0", "1", `"false"`, "null", "[]", "{}"} {
				expectRefusal(t, changed(t, raw, true, key, []byte(invalid)), ErrBooleanRequired)
			}
		}
	}
	for _, path := range []struct {
		facts bool
		key   string
	}{{false, "name"}, {false, "value"}, {false, "declared_length"}, {true, "allowed_header_name"}} {
		for _, invalid := range []string{"null", "true", "0", "[]", "{}"} {
			expectRefusal(t, changed(t, raw, path.facts, path.key, []byte(invalid)), ErrTextRequired)
		}
	}
	zero := Request{}
	if got, err := Decode(wire(t, zero)); err != nil || got != zero {
		t.Fatalf("explicit false/zero facts must be present, not missing: %#v,%v", got, err)
	}
}

func TestEachTextHasDecodedUTF8ByteBoundAndStrictUnicode(t *testing.T) {
	raw := wire(t, baseRequest())
	readText := func(r Request, key string) string {
		switch key {
		case "name":
			return r.Name
		case "value":
			return r.Value
		case "declared_length":
			return r.DeclaredLength
		default:
			return r.Facts.AllowedHeaderName
		}
	}
	for _, path := range []struct {
		facts bool
		key   string
	}{{false, "name"}, {false, "value"}, {false, "declared_length"}, {true, "allowed_header_name"}} {
		for _, valid := range []string{"", strings.Repeat("a", 1024), strings.Repeat("é", 512), strings.Repeat("😀", 256), "\x00\r\n\t", "\ufffd", `\ud800`} {
			encoded, _ := json.Marshal(valid)
			if got, err := Decode(changed(t, raw, path.facts, path.key, encoded)); err != nil || readText(got, path.key) != valid {
				t.Fatalf("valid Text changed at %s len%d: %v", path.key, len(valid), err)
			}
		}
		for _, invalid := range []string{strings.Repeat("a", 1025), strings.Repeat("é", 512) + "a", strings.Repeat("😀", 256) + "a"} {
			encoded, _ := json.Marshal(invalid)
			expectRefusal(t, changed(t, raw, path.facts, path.key, encoded), ErrTextTooLarge)
		}
		for _, invalid := range []string{`"\ud800"`, `"\udc00"`, `"\ud800\ud800"`, `"\ud800 \udc00"`, `"\ud800\\udc00"`, `"\ud83d\ude00\udc00"`} {
			expectRefusal(t, changed(t, raw, path.facts, path.key, []byte(invalid)), ErrUnpairedSurrogate)
		}
		for _, valid := range []struct{ wire, decoded string }{{`"\ud83d\ude00"`, "😀"}, {`"\uD800\uDC00"`, "\U00010000"}, {`"\\ud800"`, `\ud800`}, {`"\ufffd"`, "\ufffd"}} {
			if got, err := Decode(changed(t, raw, path.facts, path.key, []byte(valid.wire))); err != nil || readText(got, path.key) != valid.decoded {
				t.Fatal("escaped Text changed", path.key, valid.wire, err)
			}
		}
	}
	// Four fully escaped1024-byte Texts require >16KiB but fit the explicit32KiB
	// request limit. Transport must not accidentally inherit the header helper cap.
	for _, path := range []struct {
		facts bool
		key   string
	}{{false, "name"}, {false, "value"}, {false, "declared_length"}, {true, "allowed_header_name"}} {
		raw = changed(t, raw, path.facts, path.key, []byte(`"`+strings.Repeat(`\u0000`, 1024)+`"`))
	}
	if len(raw) <= 16<<10 || len(raw) > MaxRawBytes {
		t.Fatal("escaped witness size", len(raw))
	}
	got, err := Decode(raw)
	if err != nil || got.Name != strings.Repeat("\x00", 1024) || got.Value != got.Name || got.DeclaredLength != got.Name || got.Facts.AllowedHeaderName != got.Name {
		t.Fatal("four fully escaped Texts", err)
	}
	for _, invalid := range [][]byte{{0xff}, {0xc0, 0xaf}, {0xed, 0xa0, 0x80}, {0xc2}, {0xf4, 0x90, 0x80, 0x80}} {
		bad := bytes.Replace(wire(t, baseRequest()), []byte(`"req-123"`), append(append([]byte{'"'}, invalid...), '"'), 1)
		expectRefusal(t, bad, ErrInvalidUTF8)
	}
}

type countedReader struct {
	reader   io.Reader
	consumed int
}

func (r *countedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.consumed += n
	return n, err
}

type errorReader struct {
	raw []byte
	err error
}

func (r *errorReader) Read(p []byte) (int, error) {
	n := copy(p, r.raw)
	r.raw = r.raw[n:]
	return n, r.err
}

func TestBoundedReadMalformedTrailingAndErrorsRefuse(t *testing.T) {
	raw := wire(t, baseRequest())
	exact := append(append([]byte{}, raw...), bytes.Repeat([]byte{' '}, MaxRawBytes-len(raw))...)
	if got, err := Read(bytes.NewReader(exact)); err != nil || got != baseRequest() {
		t.Fatal("exact raw bound", got, err)
	}
	reader := &countedReader{reader: bytes.NewReader(append(exact, bytes.Repeat([]byte{' '}, 1000)...))}
	if got, err := Read(reader); !errors.Is(err, ErrRawTooLarge) || got != (Request{}) || reader.consumed != MaxRawBytes+1 {
		t.Fatal("bounded reader", got, err, reader.consumed)
	}
	for _, bad := range [][]byte{nil, append(append([]byte{}, raw...), []byte(`{}`)...), append(append([]byte{}, raw...), []byte(`true`)...), []byte(`{"name":`), append([]byte{0xef, 0xbb, 0xbf}, raw...), bytes.Replace(raw, []byte(`"name"`), []byte(`"\ud800"`), 1)} {
		if got, err := Decode(bad); err == nil || got != (Request{}) {
			t.Fatal("malformed admitted", got, err)
		}
		if doc, err := Prepare(bytes.NewReader(bad)); err == nil || doc != nil {
			t.Fatal("malformed emitted", err)
		}
	}
	failure := errors.New("incomplete host read")
	for _, content := range [][]byte{nil, raw} {
		if got, err := Read(&errorReader{raw: content, err: failure}); !errors.Is(err, failure) || got != (Request{}) {
			t.Fatal("read error swallowed", got, err)
		}
	}
	if _, err := Read(nil); err == nil {
		t.Fatal("nil reader")
	}
}

func fixture(t *testing.T, path, digest string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != digest {
		t.Fatal("immutable fixture changed:", path)
	}
	return raw
}

func generic(t *testing.T, raw []byte) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

// The existing accepted input-only example fixes the decoder envelope. This
// checks source fixture shape rather than borrowing any retained native result.
func TestAll50FrozenInputsUseExistingInputOnlyWireShape(t *testing.T) {
	var existing struct {
		Schema string                       `json:"schema"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	raw := fixture(t, "../../examples/header-field-facts/inputs.json", "4c0a1eb41d2b49ed0d852dfefc743bf20965ff6dce8c64753386200608f9b2b1")
	if err := json.Unmarshal(raw, &existing); err != nil || existing.Schema != "gooo/body-composition-inputs/v1" || len(existing.Inputs) != 3 {
		t.Fatal("existing input-only fixture envelope", err)
	}
	var spec struct {
		Cases []struct {
			Label string  `json:"label"`
			Input Request `json:"input"`
		} `json:"cases"`
	}
	raw = fixture(t, "../../examples/diagnostic-header-addition/corpus.json", "6a2c688b2af7fb2d032d794b6a2cbab4d42aae3969a054001965ef559c1b970f")
	if err := json.Unmarshal(raw, &spec); err != nil || len(spec.Cases) != 50 {
		t.Fatal("frozen50 source population", err)
	}
	for _, c := range spec.Cases {
		got, err := Decode(wire(t, c.Input))
		if err != nil || got != c.Input {
			t.Fatalf("input changed at %s: %#v,%v", c.Label, got, err)
		}
		doc, err := Prepare(bytes.NewReader(wire(t, c.Input)))
		if err != nil {
			t.Fatal(c.Label, err)
		}
		row := map[string]any{Entry + ".input0": c.Input.Name, Entry + ".input1": c.Input.Value, Entry + ".input2": c.Input.DeclaredLength, Entry + ".input3": c.Input.Facts}
		expected, err := json.Marshal(map[string]any{"schema": existing.Schema, "inputs": []any{row}})
		if err != nil || !reflect.DeepEqual(generic(t, doc), generic(t, expected)) {
			t.Fatalf("wrong exact4-input/12-fact wire at %s: %s (%v)", c.Label, doc, err)
		}
		if bytes.Contains(doc, []byte(`"expected":`)) || bytes.Contains(doc, []byte(`"cases":`)) {
			t.Fatal("adapter emitted results or wrong wrapper")
		}
	}
}
