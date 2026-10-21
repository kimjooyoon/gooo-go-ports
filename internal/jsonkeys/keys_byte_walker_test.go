package jsonkeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
)

// This test-only oracle retains the original Decoder.Token implementation.
func originalRejectDuplicates(raw []byte) error {
	// Retain encoding/json's syntax and nesting-depth limits before recursion.
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := originalInspectValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func originalInspectValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("invalid JSON object key")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := originalInspectValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := originalInspectValue(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = d.Token() // Syntax was checked above, including the closing delimiter.
	return err
}

func sameError(t *testing.T, raw []byte) {
	t.Helper()
	before := bytes.Clone(raw)
	want, got := originalRejectDuplicates(raw), RejectDuplicates(raw)
	if fmt.Sprint(want) != fmt.Sprint(got) {
		t.Fatalf("oracle=%v candidate=%v raw=%q", want, got, raw)
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("caller bytes changed")
	}
}

func TestByteWalkerDecodedKeyDifferential(t *testing.T) {
	pairs := [][2]string{
		{`x`, `\u0078`}, {`/`, `\/`}, {`\b`, `\u0008`}, {`\f`, `\u000c`},
		{`\n`, `\u000a`}, {`\r`, `\u000d`}, {`\t`, `\u0009`}, {`\"`, `\u0022`},
		{`\\`, `\u005c`}, {`\u0000`, `\u0000`}, {`\ud800`, `�`}, {`\udfff`, `\ufffd`},
		{`\ud800\udc00`, `𐀀`}, {string([]byte{0xff}), string([]byte{0xfe})},
		{string([]byte{0xed, 0xa0, 0x80}), `���`},
	}
	for i, p := range pairs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			raw := []byte(`{"` + p[0] + `":0,"` + p[1] + `":1}`)
			if originalRejectDuplicates(raw) == nil {
				t.Fatalf("fixture should duplicate: %q", raw)
			}
			sameError(t, raw)
		})
	}
	for _, raw := range []string{
		`{"x":{"a":0,"a":1},"x":2}`, `{"x":0,"x":{"a":0,"a":1}}`,
		`{"x":0,"x":1} trailing`, `{"x":0,"x":1,"later":"\q"}`, `{"x":0,"x":1,}`,
		`{"x":0,"\\u0078":1}`, `{"diagnostic":{"ignored":[{"a":1,"\u0061":2}]}}`,
		`{"a":0,"b":1,"c":2,"d":3,"e":4,"f":5,"g":6,"h":7,"i":8,"\u0061":9}`,
		`{"huge":1e100000000000000000000,"integer":1234567890123456789012345678901234567890}`,
		`{"x":["\\\"}[{,:",true,false,null,-0,1.234e-56],"y":{"x":1}}`,
	} {
		sameError(t, []byte(raw))
	}
}

func TestByteWalkerDepthAndInvocationIsolation(t *testing.T) {
	for _, n := range []int{9999, 10000, 10001} {
		sameError(t, []byte(strings.Repeat("[", n)+"0"+strings.Repeat("]", n)))
		sameError(t, []byte(strings.Repeat(`{"x":`, n)+"0"+strings.Repeat("}", n)))
	}
	raw := []byte(`{"a":0,"b":1}`)
	sameError(t, raw)
	raw[8] = 'a'
	sameError(t, raw)
	if RejectDuplicates(raw) == nil {
		t.Fatal("new duplicate was cached away")
	}
	raw[8] = 'b'
	sameError(t, raw)
}

func generatedDocument(r *rand.Rand, depth int) []byte {
	if depth == 0 || r.Intn(3) == 0 {
		scalar := []string{`null`, `true`, `false`, `-0`, `1e10000`, `0.125`, `"\\\"},[:"`, `"é한😀"`, `"\ud800"`}
		return []byte(scalar[r.Intn(len(scalar))])
	}
	if r.Intn(2) == 0 {
		var out bytes.Buffer
		out.WriteByte('[')
		for i, n := 0, r.Intn(7); i < n; i++ {
			if i > 0 {
				out.WriteByte(',')
			}
			out.Write(generatedDocument(r, depth-1))
		}
		out.WriteByte(']')
		return out.Bytes()
	}
	keys := []string{`a`, `b`, `\u0061`, `é`, `\u00e9`, `�`, `\ud800`, ``, `\u0000`, `\\u0061`}
	var out bytes.Buffer
	out.WriteByte('{')
	for i, n := 0, r.Intn(13); i < n; i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(&out, `"%s":`, keys[r.Intn(len(keys))])
		out.Write(generatedDocument(r, depth-1))
	}
	out.WriteByte('}')
	return out.Bytes()
}

func TestByteWalkerGeneratedDifferential(t *testing.T) {
	r := rand.New(rand.NewSource(20261021))
	for i := 0; i < 4000; i++ {
		raw := generatedDocument(r, 4)
		if !json.Valid(raw) {
			t.Fatalf("invalid generated fixture %q", raw)
		}
		sameError(t, raw)
		if i%4 == 0 {
			sameError(t, append(bytes.Clone(raw), []byte(" false")...))
		}
		if i%4 == 1 && len(raw) > 1 {
			sameError(t, raw[:len(raw)-1])
		}
	}
}
