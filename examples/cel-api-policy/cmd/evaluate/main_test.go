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

func TestReplayCLIHistoricalResultAndTransactionalFailure(t *testing.T) {
	f := nativepolicy.APIFacts{Facts: nativepolicy.Facts{GroupPresent: true, GroupValue: "admin", PrincipalPresent: true, PrincipalValue: "synthetic:user"}, Authenticated: true, CallerTenant: "tenant", ResourceTenant: "tenant", RequestedAction: "read", AllowedAction: "read"}
	p, err := nativepolicy.EvaluateAPI(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	batch := "[" + string(raw) + "]"
	var out bytes.Buffer
	if err := run("replay-api", 0, strings.NewReader(batch), &out); err != nil {
		t.Fatal(err)
	}
	var replay []nativepolicy.APIReplay
	if json.Unmarshal(out.Bytes(), &replay) != nil || len(replay) != 1 || replay[0].Status != "HISTORICALLY_CONSISTENT" || replay[0].Decision != p.Decision {
		t.Fatal("historical CLI result changed")
	}
	p.Decision.Reason = "tampered"
	badDecision, _ := json.Marshal(p)
	p.Decision.Reason = "ALLOW"
	p.Bindings.GenerationSHA256 = "sha256:wrong"
	badBinding, _ := json.Marshal(p)
	for label, bad := range map[string]string{
		"later_schema":   "[" + string(raw) + ",{}]",
		"later_decision": "[" + string(raw) + "," + string(badDecision) + "]",
		"later_binding":  "[" + string(raw) + "," + string(badBinding) + "]",
		"trailing":       batch + " {}", "empty": "[]", "null": "null",
		"over4MiB": strings.Repeat(" ", nativepolicy.MaxBatchBytes+1),
		"over1024": "[" + strings.TrimSuffix(strings.Repeat(`{}`+",", nativepolicy.MaxBatchCount+1), ",") + "]",
	} {
		t.Run(label, func(t *testing.T) {
			out.Reset()
			if err := run("replay-api", 0, strings.NewReader(bad), &out); err == nil {
				t.Fatal("bad saved batch accepted")
			}
			if out.Len() != 0 {
				t.Fatal("partial historical batch emitted")
			}
		})
	}
}
