package jsonkeys

import (
	"bytes"
	"strings"
	"testing"
)

func TestRejectDuplicates(t *testing.T) {
	bad := []string{
		`{"x":0,"x":1}`,
		`{"x":1,"x":1}`,
		`{"x":0,"\u0078":1}`,
		`{"é":0,"\u00e9":1}`,
		`{"𝄞":0,"\ud834\udd1e":1}`,
		`{"":0,"":1}`,
		`{"outer":{"x":0,"x":1}}`,
		`{"outer":[{"x":0,"x":1}]}`,
		`[{"x":{"a":1},"x":{"b":2}}]`,
		`{"x":null,"x":[]}`,
	}
	for _, raw := range bad {
		t.Run(raw, func(t *testing.T) {
			if err := RejectDuplicates([]byte(raw)); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
				t.Fatalf("expected duplicate-key rejection, got %v", err)
			}
		})
	}
}

func TestUniqueJSONPreservesExistingPolicyAndBytes(t *testing.T) {
	good := []string{
		`null`, `true`, `false`, `0`, `-0`, `1e10000`, `"text"`, `[]`, `{}`,
		`{"x":null}`, `{"x":1,"X":2}`, `{"é":1,"e\u0301":2}`,
		`[{"x":1},{"x":2}]`, `{"a":{"x":1},"b":{"x":2}}`,
		`{"text":"{\"x\":1,\"x\":2}"}`, `{"x":[1,1,null,null]}`,
		`{"unknown":{"nested":[9223372036854775807,-9223372036854775808,9007199254740993]}}`,
		" \n {\"x\": 1}\t\r\n",
	}
	for _, raw := range good {
		t.Run(raw, func(t *testing.T) {
			data := []byte(raw)
			before := bytes.Clone(data)
			if err := RejectDuplicates(data); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, before) {
				t.Fatal("rewrote original JSON bytes")
			}
		})
	}
}

func TestRejectInvalidOrTrailingJSON(t *testing.T) {
	bad := []string{"", " ", `{`, `[`, `{"x":}`, `{"x":1,}`, `[1,]`, `{"x" 1}`, `{"x":1} {}`, `null true`, `01`}
	for _, raw := range bad {
		if err := RejectDuplicates([]byte(raw)); err == nil {
			t.Errorf("accepted invalid or trailing JSON: %q", raw)
		}
	}
	// Match encoding/json's existing maximum-depth rejection before recursion.
	tooDeep := strings.Repeat("[", 10001) + "0" + strings.Repeat("]", 10001)
	if err := RejectDuplicates([]byte(tooDeep)); err == nil {
		t.Fatal("accepted JSON beyond encoding/json's nesting limit")
	}
}
