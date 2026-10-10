package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnicodeEscapesCannotAliasTenant(t *testing.T) {
	all, _, _, err := loadFixtures("fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	base := string(all[0].Input)
	for _, escape := range []string{`\ud800`, `\udfff`, `\ud800x`, `\ud800\ud800`} {
		raw := []byte(strings.ReplaceAll(base, `"alpha"`, `"`+escape+`"`))
		if _, err := decodeEnvelope(raw); err == nil {
			t.Fatalf("invalid escape accepted: %s", escape)
		}
	}
	for _, escape := range []string{`\ud83d\ude00`, `\\ud800`, `\ufffd`} {
		e, err := decodeEnvelope([]byte(strings.ReplaceAll(base, `"alpha"`, `"`+escape+`"`)))
		if err != nil || !reference(e).Allowed {
			t.Fatalf("valid explicit tenant %s rejected: %v", escape, err)
		}
	}
}

func TestDispatchRequiresConsistentVerifiedActual(t *testing.T) {
	for _, c := range []struct {
		actual Outcome
		err    error
		calls  int
	}{
		{Outcome{true, "allow", "policy_satisfied"}, nil, 1},
		{Outcome{false, "deny", "limit_exceeded"}, nil, 0},
		{Outcome{false, "review", "model_review"}, nil, 0},
		{Outcome{true, "allow", "policy_satisfied"}, errors.New("verification failed"), 0},
		{Outcome{true, "deny", "model_denied"}, nil, 0},
		{Outcome{false, "allow", "policy_satisfied"}, nil, 0},
		{Outcome{true, "allow", ""}, nil, 0},
	} {
		s := spy{}
		_ = dispatch(c.actual, c.err, &s)
		if s.calls != c.calls {
			t.Fatalf("unexpected calls for %#v: %d", c, s.calls)
		}
	}
}

// Uses a saved successful run when supplied. Mutations preserve the rest of the
// real receipt; they must fail before any mock dispatch.
func TestReceiptBindingsRejectTampering(t *testing.T) {
	out := os.Getenv("PILOT_EVIDENCE")
	if out == "" {
		t.Skip("set PILOT_EVIDENCE to a successful output directory")
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	_, valid, _, err := loadFixtures(filepath.Join(out, "fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw := read("execution.json")
	manifest, source, cases := read("gooo.workspace.json"), read("policy.gooo"), read("cases.json")
	if _, err := inspectReceipt(raw, manifest, source, cases, valid, nil); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*Receipt){
		"input value": func(r *Receipt) {
			r.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = json.RawMessage(`{"action":"export_records","tenant":"alpha","item_count":9007199254740993}`)
		},
		"field bool/int confusion": func(r *Receipt) {
			r.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Fields[0].Value = json.RawMessage(`1`)
		},
		"input field ID": func(r *Receipt) { r.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Fields[0].ID += "wrong" },
		"input order": func(r *Receipt) {
			d := &r.Result.Runtime.Traces[0].Deliveries[0]
			d.Inputs[0], d.Inputs[1] = d.Inputs[1], d.Inputs[0]
		},
		"actual": func(r *Receipt) {
			r.Result.Runtime.Traces[0].Deliveries[0].Actual = json.RawMessage(`{"allowed":false,"disposition":"deny","reason":"model_denied"}`)
		},
		"output field ID":          func(r *Receipt) { r.Result.Runtime.Traces[0].Deliveries[0].Fields[0].ID += "wrong" },
		"source":                   func(r *Receipt) { r.Result.Program.Image.Packages[0].Sources[0].Digest = "wrong" },
		"producer":                 func(r *Receipt) { r.Result.Runtime.Producer = "wrong" },
		"generated source":         func(r *Receipt) { r.Result.Composition.Source += "\n// changed" },
		"null runtime model calls": func(r *Receipt) { r.Result.Runtime.ModelCalls = nil },
		"fresh replay metadata":    func(r *Receipt) { r.ReplayedFrom = "sha256:unexpected" },
		"cases binding":            func(r *Receipt) { r.CasesDigest = "wrong" },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			var r Receipt
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			change(&r)
			_, err := inspectReceipt(marshal(r), manifest, source, cases, valid, nil)
			if err == nil {
				t.Fatal("tamper accepted")
			}
			s := spy{}
			_ = dispatch(Outcome{true, "allow", "policy_satisfied"}, err, &s)
			if s.calls != 0 {
				t.Fatal("tamper dispatched")
			}
		})
	}
	replay := read("replay.json")
	var r Receipt
	if err := json.Unmarshal(replay, &r); err != nil {
		t.Fatal(err)
	}
	r.Result.Replay.ModelCalls = nil
	if _, err := inspectReceipt(marshal(r), manifest, source, cases, valid, raw); err == nil {
		t.Fatal("null replay model calls accepted")
	}
	for _, receipt := range []struct{ raw, parent []byte }{{raw, nil}, {replay, raw}} {
		missing := strings.ReplaceAll(string(receipt.raw), `"model_calls":0,`, "")
		missing = strings.ReplaceAll(missing, `,"model_calls":0`, "")
		if missing == string(receipt.raw) {
			t.Fatal("missing model_calls fixture did not mutate")
		}
		if _, err := inspectReceipt([]byte(missing), manifest, source, cases, valid, receipt.parent); err == nil {
			t.Fatal("missing model calls accepted")
		}
	}
	if _, err := inspectReceipt(replay, manifest, source, cases, valid, append(raw, ' ')); err == nil {
		t.Fatal("wrong replay predecessor accepted")
	}
	duplicate := strings.Replace(string(raw), `"decision":"PASS"`, `"decision":"FAIL_CLOSED","decision":"PASS"`, 1)
	if duplicate == string(raw) {
		t.Fatal("duplicate fixture did not mutate")
	}
	if _, err := inspectReceipt([]byte(duplicate), manifest, source, cases, valid, nil); err == nil {
		t.Fatal("duplicate receipt key accepted")
	}
}
