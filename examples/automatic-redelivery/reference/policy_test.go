package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestManualPrecedenceAndSignedEchoes(t *testing.T) {
	reasons := []string{"INPUT_DOMAIN_INVALID", "CURRENT_ACCESS_REFUSED", "REQUEST_BINDING_CHANGED", "OPERATION_ALREADY_APPLIED", "PROVIDER_RETRY_VETO", "ATTEMPT_BUDGET_EXHAUSTED", "BODY_NOT_REPLAYABLE", "PROVIDER_GUIDANCE_UNVERIFIED", "POSITIVE_NONAPPLICATION_EVIDENCE", "SERVER_REPEAT_CONTRACT_UNVERIFIED", "VERIFIED_REPEAT_CONTRACT", "DEDUP_KEY_NOT_BOUND", "VERIFIED_SERVER_DEDUP", "INPUT_DOMAIN_INVALID", "INPUT_DOMAIN_INVALID", "POSITIVE_NONAPPLICATION_EVIDENCE"}
	actions := []string{"STOP", "STOP", "STOP", "LOOKUP_RECORDED_RESULT", "STOP", "STOP", "STOP", "REFRESH_OUTCOME", "REDELIVER_ELIGIBLE", "REFRESH_OUTCOME", "REDELIVER_ELIGIBLE", "REFRESH_OUTCOME", "REDELIVER_ELIGIBLE", "STOP", "STOP", "REDELIVER_ELIGIBLE"}
	dedup := []bool{false, true, true, false, true, true, true, false, false, true, false, true, true, false, false, false}
	for i, w := range ManualWitnesses() {
		t.Run(w.Name, func(t *testing.T) {
			got, err := Evaluate(w.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			if got.Input != w.Inputs || got.Reason != reasons[i] || got.Action != actions[i] {
				t.Fatalf("unexpected literal witness: %+v", got)
			}
			want := w.Inputs.AttemptsRemaining
			if actions[i] == "REDELIVER_ELIGIBLE" {
				want--
			}
			if got.AttemptsIfClaimed != want {
				t.Fatal("attempt echo/decrement")
			}
			if got.DedupRequired != dedup[i] {
				t.Fatal("literal dedup requirement", i, got)
			}
			if got.EligibleToClaim != (actions[i] == "REDELIVER_ELIGIBLE") {
				t.Fatal("eligibility")
			}
			if i == 3 && (got.EligibleToClaim || got.AttemptsIfClaimed != 0) {
				t.Fatal("lookup must not dispatch/claim")
			}
			if (i == 0 || i == 13 || i == 14) && (got.DomainStatus != "INVALID_INPUT" || got.DedupRequired) {
				t.Fatal("invalid extremes/enum must not satisfy dedup")
			}
		})
	}
}

func TestFullIndependentPartitionAndFieldShape(t *testing.T) {
	actions, reasons := map[string]int{}, map[string]int{}
	count := 0
	err := ForEachValid(func(index int, in Input) error {
		if index != count {
			t.Fatal("ordered index")
		}
		count++
		got, err := Evaluate(in)
		if err != nil {
			return err
		}
		if got.DomainStatus != "VALID_INPUT" || got.Input != in {
			t.Fatal("valid/echo")
		}
		actions[got.Action]++
		reasons[got.Reason]++
		raw, err := json.Marshal(got)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		if len(fields) != 15 {
			t.Fatal("all15 fields must survive false/zero")
		}
		for _, name := range []string{"body_replayable", "idempotency_key_present", "server_repeat_contract_verified", "request_binding_current", "current_access_allowed", "eligible_to_claim", "dedup_required", "attempts_remaining", "attempts_if_claimed"} {
			if _, ok := fields[name]; !ok {
				t.Fatal("required zero field", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 12288 || !reflect.DeepEqual(actions, map[string]int{"STOP": 10944, "LOOKUP_RECORDED_RESULT": 768, "REDELIVER_ELIGIBLE": 304, "REFRESH_OUTCOME": 272}) {
		t.Fatal(count, actions)
	}
	expected := map[string]int{"CURRENT_ACCESS_REFUSED": 6144, "REQUEST_BINDING_CHANGED": 3072, "OPERATION_ALREADY_APPLIED": 768, "PROVIDER_RETRY_VETO": 576, "ATTEMPT_BUDGET_EXHAUSTED": 576, "BODY_NOT_REPLAYABLE": 576, "PROVIDER_GUIDANCE_UNVERIFIED": 192, "POSITIVE_NONAPPLICATION_EVIDENCE": 256, "SERVER_REPEAT_CONTRACT_UNVERIFIED": 64, "VERIFIED_REPEAT_CONTRACT": 32, "DEDUP_KEY_NOT_BOUND": 16, "VERIFIED_SERVER_DEDUP": 16}
	if !reflect.DeepEqual(reasons, expected) {
		t.Fatal(reasons)
	}
}

func TestStrictWireAndUnicodeBoundary(t *testing.T) {
	base := ManualWitnesses()[12].Inputs
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseInput(raw)
	if err != nil || parsed != base {
		t.Fatal(parsed, err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for name := range fields {
		for _, bad := range []string{"null", "[]", "{}"} {
			changed := map[string]json.RawMessage{}
			for k, v := range fields {
				changed[k] = v
			}
			changed[name] = json.RawMessage(bad)
			b, _ := json.Marshal(changed)
			if _, err := ParseInput(b); err == nil {
				t.Fatal("wrong kind", name, bad)
			}
		}
		missing := map[string]json.RawMessage{}
		for k, v := range fields {
			if k != name {
				missing[k] = v
			}
		}
		b, _ := json.Marshal(missing)
		if _, err := ParseInput(b); err == nil {
			t.Fatal("missing", name)
		}
	}
	replace := func(name, value string) []byte {
		changed := map[string]json.RawMessage{}
		for k, v := range fields {
			changed[k] = v
		}
		changed[name] = json.RawMessage(value)
		b, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, name := range []string{"body_replayable", "idempotency_key_present", "server_repeat_contract_verified", "request_binding_current", "current_access_allowed"} {
		for _, value := range []string{"0", "1", "\"false\""} {
			if _, err := ParseInput(replace(name, value)); err == nil {
				t.Fatal("boolean coercion", name, value)
			}
		}
	}
	for _, value := range []string{"1.0", "1e0", "true", "\"1\"", "9223372036854775808", "-9223372036854775809"} {
		if _, err := ParseInput(replace("attempts_remaining", value)); err == nil {
			t.Fatal("integer token/range", value)
		}
	}
	for _, suffix := range []string{`,"method":"POST"}`, `,"\u006dethod":"POST"}`, `,"extra":true}`} {
		b := append(append([]byte{}, raw[:len(raw)-1]...), []byte(suffix)...)
		if _, err := ParseInput(b); err == nil {
			t.Fatal("duplicate/unknown", suffix)
		}
	}
	for _, b := range [][]byte{append(append([]byte{}, raw...), []byte(" {}")...), []byte(`{"method":"\ud800"}`), []byte(`{"method":"\udc00"}`), []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, []byte(`{`)} {
		if _, err := ParseInput(b); err == nil {
			t.Fatal("bad unicode/JSON accepted")
		}
	}
	// A valid surrogate pair decodes to a valid non-enum scalar and is echoed.
	b := replace("method", `"\ud83d\ude00"`)
	parsed, err = ParseInput(b)
	if err != nil || parsed.Method != "😀" {
		t.Fatal(parsed, err)
	}
	report, err := Evaluate(parsed)
	if err != nil || report.DomainStatus != "INVALID_INPUT" || report.Method != "😀" {
		t.Fatal(report, err)
	}
	for _, size := range []int{1024, 1025} {
		s := ""
		for i := 0; i < size; i++ {
			s += "x"
		}
		v, _ := json.Marshal(s)
		_, err := ParseInput(replace("method", string(v)))
		if (err == nil) != (size == 1024) {
			t.Fatal("text byte boundary", size, err)
		}
	}
	for _, runes := range []int{256, 257} {
		text := ""
		for i := 0; i < runes; i++ {
			text += "😀"
		}
		value, _ := json.Marshal(text)
		_, err := ParseInput(replace("method", string(value)))
		if (err == nil) != (runes == 256) {
			t.Fatal("UTF8 byte boundary", runes, err)
		}
	}
	for _, value := range []string{"-9223372036854775808", "9223372036854775807"} {
		parsed, err := ParseInput(replace("attempts_remaining", value))
		if err != nil {
			t.Fatal(err)
		}
		report, err := Evaluate(parsed)
		if err != nil || report.DomainStatus != "INVALID_INPUT" || report.AttemptsIfClaimed != parsed.AttemptsRemaining {
			t.Fatal("int64 exact invalid echo", report, err)
		}
	}
}
