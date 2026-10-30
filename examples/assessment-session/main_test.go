//go:build linux || darwin

// Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../../LICENSE.
package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var records = flag.String("records", os.Getenv("GOOO_ASSESSMENT_RECORDS"), "persistent output from one explicit experimental276 sample")

func readContext(root, name string) contextRecord {
	var c contextRecord
	load(filepath.Join(root, name+"-context.json"), &c)
	return c
}
func runtimeOf(f map[string]any) map[string]any {
	return object(object(object(f["receipt"])["result"])["runtime"])
}
func valueOf(f map[string]any) map[string]any {
	return object(one(one(runtimeOf(f)["traces"])["deliveries"])["actual"])
}
func TestSavedAssessmentDisplay(t *testing.T) {
	if *records == "" {
		t.Skip("No learning records supplied. Run the example once, then set GOOO_ASSESSMENT_RECORDS or -records for offline checks.")
	}
	root, e := filepath.Abs(*records)
	if e != nil {
		t.Fatal(e)
	}
	refuse := func(t *testing.T, frame fileRef, c contextRecord) {
		t.Helper()
		if _, err := display(frame, c); err == nil {
			t.Fatal("inconsistent saved evidence acquired a display")
		}
	}
	frameMutation := func(t *testing.T, mutate func(map[string]any)) {
		t.Helper()
		c := readContext(root, "green-correct")
		raw, e := os.ReadFile(filepath.Join(root, "green-correct-response.jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		f := object(parse(raw))
		mutate(f)
		path := filepath.Join(t.TempDir(), "frame.jsonl")
		save(path, f)
		refuse(t, reference(path), c)
	}
	captureMutation := func(t *testing.T, keys []string, label bool) {
		t.Helper()
		red, green := readContext(root, "red-correct"), readContext(root, "green-correct")
		if label {
			red.Capture = "green"
		}
		for _, key := range keys {
			red.CaptureFiles[key] = green.CaptureFiles[key]
		}
		refuse(t, reference(filepath.Join(root, "red-correct-response.jsonl")), red)
	}
	requestMutation := func(t *testing.T, name string, mutate func(map[string]any)) {
		t.Helper()
		c := readContext(root, name)
		r := object(parse(bound(c.Request, 8<<20)))
		mutate(r)
		path := filepath.Join(t.TempDir(), "request.jsonl")
		save(path, r)
		c.Request = reference(path)
		refuse(t, reference(filepath.Join(root, name+"-response.jsonl")), c)
	}
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{"actual_red_matched_wrong_expected", func(t *testing.T) {
			views := map[string]map[string]any{}
			for _, name := range []string{"red-correct", "red-intentional", "green-correct", "wrong-expected"} {
				v, e := display(reference(filepath.Join(root, name+"-response.jsonl")), readContext(root, name))
				if e != nil {
					t.Fatal(e)
				}
				views[name] = v
			}
			first := reference(filepath.Join(root, "first-receipt.json"))
			for _, name := range []string{"red-intentional", "green-correct", "wrong-expected"} {
				parent, ok := object(views[name]["evidence"])["saved_replay_parent"].(*fileRef)
				if !ok || parent == nil || *parent != first {
					t.Fatal("replays do not all bind the same caller-saved first receipt")
				}
			}
			if object(views["red-correct"]["program_value"])["state"] != "TEST_SET_MISMATCH" || object(views["red-correct"]["assessment_execution"])["receipt_decision"] != "PASS" {
				t.Fatal("outer/domain distinction lost")
			}
			if object(views["red-intentional"]["program_value"])["failure_observed"] != true || object(views["green-correct"]["program_value"])["failure_observed"] != false {
				t.Fatal("failure distinction lost")
			}
			if !is(views["wrong-expected"]["program_value"], views["green-correct"]["program_value"]) || object(views["wrong-expected"]["assessment_execution"])["receipt_decision"] != "PROGRESS" || object(views["wrong-expected"]["assessment_execution"])["matched_expected_result_cases"] != 0 {
				t.Fatal("wrong expected value became success")
			}
		}},
		{"missing_value", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) { delete(one(one(runtimeOf(f)["traces"])["deliveries"]), "actual") })
		}},
		{"null_value", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) { one(one(runtimeOf(f)["traces"])["deliveries"])["actual"] = nil })
		}},
		{"missing_field", func(t *testing.T) { frameMutation(t, func(f map[string]any) { delete(valueOf(f), "state") }) }},
		{"boolean_not_count", func(t *testing.T) { frameMutation(t, func(f map[string]any) { valueOf(f)["matched"] = true }) }},
		{"response_schema", func(t *testing.T) { frameMutation(t, func(f map[string]any) { f["schema"] = "wrong" }) }},
		{"runtime_schema", func(t *testing.T) { frameMutation(t, func(f map[string]any) { runtimeOf(f)["schema"] = "wrong" }) }},
		{"source_origin", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) {
				program := object(object(object(f["receipt"])["result"])["program"])
				object(object(list(program["activity_origins"])[0])["input_source"])["source_sha256"] = "sha256:wrong"
			})
		}},
		{"input_digest", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) { object(f["receipt"])["cases_digest"] = "sha256:wrong" })
		}},
		{"host_source_pointer", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) { valueOf(f)["source_revision"] = "another-host-source" })
		}},
		{"other_compiler", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) {
				runtimeOf(f)["producer_source_sha"] = "205896a413e5dea92366309c35ae85e542d055f6"
			})
		}},
		{"parent_digest", func(t *testing.T) {
			frameMutation(t, func(f map[string]any) { object(f["receipt"])["replayed_from_sha256"] = "sha256:wrong" })
		}},
		{"saved_bytes", func(t *testing.T) {
			raw, e := os.ReadFile(filepath.Join(root, "green-correct-response.jsonl"))
			if e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(t.TempDir(), "frame")
			saveRaw(p, raw)
			ref := reference(p)
			if e = os.WriteFile(p, append(raw, ' '), 0600); e != nil {
				t.Fatal(e)
			}
			refuse(t, ref, readContext(root, "green-correct"))
		}},
		{"duplicate_outer_key", func(t *testing.T) {
			raw, e := os.ReadFile(filepath.Join(root, "green-correct-response.jsonl"))
			if e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(t.TempDir(), "frame")
			saveRaw(p, append([]byte(`{"schema":"gooo/workspace-session-response/v1",`), raw[1:]...))
			refuse(t, reference(p), readContext(root, "green-correct"))
		}},
		{"whole_capture", func(t *testing.T) { captureMutation(t, []string{"stdout", "process", "source_binding"}, true) }},
		{"stdout_only", func(t *testing.T) { captureMutation(t, []string{"stdout"}, false) }},
		{"process_only", func(t *testing.T) { captureMutation(t, []string{"process"}, false) }},
		{"stdout_process", func(t *testing.T) { captureMutation(t, []string{"stdout", "process"}, false) }},
		{"binding_only", func(t *testing.T) { captureMutation(t, []string{"source_binding"}, false) }},
		{"capture_label", func(t *testing.T) { captureMutation(t, nil, true) }},
		{"other_request", func(t *testing.T) {
			red, green := readContext(root, "red-correct"), readContext(root, "green-correct")
			red.Request = green.Request
			refuse(t, reference(filepath.Join(root, "red-correct-response.jsonl")), red)
		}},
		{"request_schema", func(t *testing.T) {
			requestMutation(t, "red-correct", func(r map[string]any) { r["schema"] = "wrong" })
		}},
		{"request_operation", func(t *testing.T) {
			requestMutation(t, "red-correct", func(r map[string]any) { r["operation"] = "replay" })
		}},
		{"duplicate_cases_flag", func(t *testing.T) {
			requestMutation(t, "red-correct", func(r map[string]any) {
				r["arguments"] = append(list(r["arguments"]), "--cases", filepath.Join(root, "profile/cases-red-correct.json"))
			})
		}},
		{"request_input", func(t *testing.T) {
			requestMutation(t, "red-correct", func(r map[string]any) {
				list(r["arguments"])[3] = filepath.Join(root, "profile/cases-green-correct.json")
			})
		}},
		{"request_parent", func(t *testing.T) {
			requestMutation(t, "green-correct", func(r map[string]any) { list(r["arguments"])[1] = filepath.Join(root, "green-correct-response.jsonl") })
		}},
		{"request_Go", func(t *testing.T) {
			requestMutation(t, "red-correct", func(r map[string]any) { list(r["arguments"])[1] = "/another/go" })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
	if len(tests) != 27 {
		t.Fatal("offline control denominator changed")
	}
}
