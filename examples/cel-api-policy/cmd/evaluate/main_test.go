package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	nativepolicy "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func TestCLIUsesNativeResultAndValidatesWholeBatch(t *testing.T) {
	raw := `[{"group_present":false,"group_value":"admin","principal_present":true,"principal_value":"user:me@acme.co"}]`
	var out bytes.Buffer
	if err := run("scalar", 0, strings.NewReader(raw), &out); err != nil {
		t.Fatal(err)
	}
	var results []nativepolicy.Proof
	if json.Unmarshal(out.Bytes(), &results) != nil || len(results) != 1 || !results[0].Decision.Allowed || !results[0].Decision.GroupMissing || results[0].Decision.EvaluationError {
		t.Fatal("native missing-group suppression result unavailable")
	}
	for _, bad := range []string{`[]`, `null`, strings.TrimSuffix(raw, "]") + `,{}]`, strings.TrimSuffix(raw, "]") + `] {}`, `[{"allowed":true}]`} {
		out.Reset()
		if err := run("scalar", 0, strings.NewReader(bad), &out); err == nil {
			t.Fatalf("invalid batch accepted: %s", bad)
		}
		if out.Len() != 0 {
			t.Fatal("partial batch result emitted")
		}
	}
	if err := run("invalid", 0, strings.NewReader(raw), &out); err == nil {
		t.Fatal("invalid mode accepted")
	}
}
