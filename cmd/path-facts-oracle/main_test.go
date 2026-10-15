package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManual24IndependentGoAndInputOrder(t *testing.T) {
	raw, err := os.ReadFile("../check-path-facts/testdata/manual-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strictJSON(raw); err != nil {
		t.Fatal(err)
	}
	var frozen manualContract
	if err := json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen.Cases) != 24 {
		t.Fatal("manual24 changed")
	}
	names := map[string]bool{}
	for _, row := range frozen.Cases {
		if row.Name == "" || names[row.Name] || !reflect.DeepEqual(reference(row.Input), row.Expected) {
			t.Fatal("manual originalGo mismatch", row.Name)
		}
		names[row.Name] = true
	}
	for _, entry := range []string{"go/path:Ext", "go/path:IsAbs", "app/path-facts:Report"} {
		inputs := make([]map[string]string, 0, 24)
		for _, row := range frozen.Cases {
			inputs = append(inputs, map[string]string{entry: row.Input})
		}
		raw, err := json.Marshal(map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": inputs})
		if err != nil {
			t.Fatal(err)
		}
		got, err := nativeInputs(raw, entry)
		if err != nil || len(got) != 24 {
			t.Fatal(entry, err)
		}
		for i, value := range got {
			if value != frozen.Cases[i].Input {
				t.Fatal("original input/order changed", entry, i)
			}
		}
	}
}

// These are exactly the accepted private15 raw transport controls, copied as
// source fixtures. They validate this strict boundary, not bareCLI behavior.
func TestFrozen15TransportControls(t *testing.T) {
	controls := []struct{ name, sha, reason string }{
		{"text-1025", "e05f16ec060cb56b21ecb87424a2d01a05a35e68a3018e43fa27b748bd060e97", "TEXT_BYTE_BOUND"},
		{"raw-invalid-utf8", "cd38e4d98315356abb65f0a9b3ba09225870f243ce239cf1ae4e7157ff729f05", "INVALID_RAW_UTF8"},
		{"unpaired-high-surrogate", "88bd5c0251c36b242f8a80a54c6fefde567d7c6b5bf768c58b94501329f42aec", "INVALID_UNICODE_SCALAR"},
		{"unpaired-low-surrogate", "32397456260c5e9479c51f99a644fd7ca4bf988dfba1ebff3b8a84174c1cbbee", "INVALID_UNICODE_SCALAR"},
		{"null", "f0564c04f5064a992ef572937495c35905a9c75e4ecd2e2129d893b3ba691418", "TEXT_TYPE"},
		{"boolean", "dba09572c9b153528567ff3bc7fe138c53d362d984bec1a8798783e89ebf10b4", "TEXT_TYPE"},
		{"integer", "c83a97ed1c5077c1c70576be60aa47fd4e841d297b9f51c2121911d15cc4b939", "TEXT_TYPE"},
		{"array", "004860f4c0366ec29394a03aa93a90b6f43a9850d051b4c21a42e07251323748", "TEXT_TYPE"},
		{"object", "0d00f47261d2ea0e84a824233adaef881a39a845d6ab650e341dfb3a82d7303c", "TEXT_TYPE"},
		{"missing", "7162f99cebe3a302f974ed59a4485fa32ff6ac9795373d424da7cb10da035df2", "REQUIRED_INPUT"},
		{"duplicate-input", "efb662d5f952f38293f8b97782053b8ca00a3b2cf486f291a4221d0ccb6c90b8", "DUPLICATE_KEY"},
		{"escaped-duplicate-input", "37696b0245e5f340438ae992bc14c886cdbe9a94e0ee054ac50c6b962fc346d5", "DUPLICATE_KEY"},
		{"unknown-input", "f14bfcaccb2f8b5d15c90b74337f8cea83c76877b86bdab901661dddc2bada20", "UNEXPECTED_INPUT"},
		{"malformed", "1dc11ba8cbc1767cb968d3f12368e6cae2f312440ae463e8472967b512b7b653", "MALFORMED_JSON"},
		{"trailing-json", "a805b2c2df6b9b0aead94395b2481b04d798ae2a0bfb0057b9cbf81636115963", "TRAILING_JSON"},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "negative", control.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if digest(raw) != "sha256:"+control.sha {
				t.Fatal("frozen bytes changed")
			}
			if _, err := nativeInputs(raw, "app/path-facts:Report"); err == nil || err.Error() != control.reason {
				t.Fatal("wrong strict refusal", err)
			}
		})
	}
	for _, raw := range []string{
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/path-facts:Report":"\ud83d\ude42.x"}]}`,
		`{"schema":"gooo/body-composition-inputs/v1","inputs":[{"app/path-facts:Report":"\\ud800.x"}]}`,
	} {
		if _, err := nativeInputs([]byte(raw), "app/path-facts:Report"); err != nil {
			t.Fatal("valid surrogate pair/literal escaped slash refused", err)
		}
	}
}

func TestFutureExhaustiveCorpusOrderAndSize(t *testing.T) {
	inputs := exhaustiveInputs()
	if len(inputs) != 9841 || inputs[0] != "" || inputs[1] != "a" || inputs[2] != "." || inputs[3] != "/" || inputs[4] != "aa" || inputs[len(inputs)-1] != "////////" {
		t.Fatal("fixed corpus changed")
	}
	seen := map[string]bool{}
	counts := map[int]int{}
	for _, value := range inputs {
		if seen[value] {
			t.Fatal("duplicate")
		}
		seen[value] = true
		counts[len(value)]++
	}
	want := 1
	for length := 0; length <= 8; length++ {
		if counts[length] != want {
			t.Fatal("length denominator", length)
		}
		want *= 3
	}
}
