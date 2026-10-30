//go:build linux || darwin

// Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../../LICENSE.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

const engineSHA = "276734167edc7f5fef18cbb79004ea9592553317"
const sourceSHA = "a4ceee554b2e6473fb8bb40f2a97cf56babc276e37c086413d6033fea76da764"
const capturesSHA = "78a7e137230b26ad35d2a6b64c77b51e8dd10bb272d6a9ebd35a475f77c3babc"
const entryID = "urn:declared-tests:assess"
const helperID = "urn:declared-tests:known-outcome"

type fileRef struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
	SHA   string `json:"sha256"`
}
type contextRecord struct {
	ID           string             `json:"id"`
	Operation    string             `json:"operation"`
	ProfileRoot  string             `json:"profile_root"`
	Source       fileRef            `json:"source"`
	Manifest     fileRef            `json:"manifest"`
	Input        fileRef            `json:"input"`
	Request      fileRef            `json:"request"`
	Parent       *fileRef           `json:"saved_replay_parent"`
	Go           fileRef            `json:"Go"`
	GoVersion    string             `json:"runtime_go_version"`
	Capture      string             `json:"original_capture"`
	CaptureFiles map[string]fileRef `json:"original_capture_files"`
}

func require(ok bool, message string) {
	if !ok {
		panic(fmt.Errorf("%s", message))
	}
}
func hash(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func reference(path string) fileRef {
	path, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return fileRef{path, len(b), hash(b)}
}
func bound(ref fileRef, maximum int) []byte {
	require(filepath.IsAbs(ref.Path) && ref.Bytes > 0 && ref.Bytes <= maximum, "saved file byte/path bound differs")
	info, err := os.Stat(ref.Path)
	if err != nil {
		panic(err)
	}
	require(info.Size() == int64(ref.Bytes), "saved file bytes differ")
	b, err := os.ReadFile(ref.Path)
	if err != nil {
		panic(err)
	}
	require(hash(b) == ref.SHA, "saved file bytes differ")
	return b
}

// encoding/json normally accepts duplicate names; reject them at every object depth.
func jsonValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		result := map[string]any{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return nil, e
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("JSON key differs")
			}
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("duplicate JSON field: %s", name)
			}
			value, e := jsonValue(d)
			if e != nil {
				return nil, e
			}
			result[name] = value
		}
		_, err = d.Token()
		return result, err
	case '[':
		result := []any{}
		for d.More() {
			value, e := jsonValue(d)
			if e != nil {
				return nil, e
			}
			result = append(result, value)
		}
		_, err = d.Token()
		return result, err
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}
func parse(raw []byte) any {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := jsonValue(d)
	if err != nil {
		panic(err)
	}
	_, err = d.Token()
	require(err == io.EOF, "trailing JSON data")
	return v
}
func object(v any) map[string]any {
	m, ok := v.(map[string]any)
	require(ok, "object missing or malformed")
	return m
}
func list(v any) []any { a, ok := v.([]any); require(ok, "array missing or malformed"); return a }
func one(v any) map[string]any {
	a := list(v)
	require(len(a) == 1, "expected exactly one record")
	return object(a[0])
}
func text(v any) string { s, ok := v.(string); require(ok, "string missing or malformed"); return s }
func count(v any) int {
	n, ok := v.(json.Number)
	require(ok, "integer missing or malformed")
	i, e := n.Int64()
	require(e == nil && i >= 0 && i <= 1000000, "integer bound differs")
	return int(i)
}
func is(v any, w any) bool { return reflect.DeepEqual(v, w) }
func load(path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	parse(b)
	if err = json.Unmarshal(b, v); err != nil {
		panic(err)
	}
}
func display(frameRef fileRef, c contextRecord) (view map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			view = nil
			err = fmt.Errorf("display refused: %v", r)
		}
	}()
	f := object(parse(bound(frameRef, 8<<20)))
	require(f["schema"] == "gooo/workspace-session-response/v1" && f["id"] == c.ID, "session schema/id differs")
	require(count(f["exit_code"]) == 0 && f["error"] == nil && f["closed"] != true, "session command failed")
	r := object(f["receipt"])
	require(r["schema"] == "gooo/workspace-body-execution-receipt/v1" && r["error"] == nil, "receipt schema/error differs")
	source := bound(c.Source, 8<<20)
	require(len(source) == 5977 && hash(source) == sourceSHA, "Assess source differs")
	manifest := object(parse(bound(c.Manifest, 8<<20)))
	require(is(manifest["entry"], map[string]any{"package_path": "openfga/declaredtests", "activity": "Assess"}), "manifest entry differs")
	input := one(object(parse(bound(c.Input, 8<<20)))["cases"])
	want := object(object(input["inputs"])["openfga/declaredtests:Assess"])
	require(r["manifest_digest"] == "sha256:"+c.Manifest.SHA && r["cases_digest"] == "sha256:"+c.Input.SHA && r["manifest"] == c.Manifest.Path, "manifest/input digest/path differs")
	result := object(r["result"])
	require(result["schema"] == "gooo/workspace-body-execution/v1", "result schema differs")
	program := object(result["program"])
	require(program["schema"] == "gooo/workspace-body-program/v1", "program schema differs")
	pentry := object(program["entry"])
	require(pentry["package_path"] == "openfga/declaredtests" && pentry["activity"] == "Assess", "program entry differs")
	origins := list(program["activity_origins"])
	require(len(origins) == 2, "source origin count differs")
	ids := map[string]bool{}
	for _, v := range origins {
		origin := object(v)
		id := text(origin["lowered_activity_id"])
		ids[id] = true
		located := object(origin["input_source"])
		decl := object(located["declaration"])
		require(located["activity_id"] == id && located["package_path"] == "openfga/declaredtests" && located["filename"] == "core.source.gooo" && located["source_sha256"] == "sha256:"+sourceSHA && decl["source_sha256"] == "sha256:"+sourceSHA && decl["stable_id"] == id, "activity source binding differs")
	}
	require(ids[entryID] && ids[helperID], "source IDs differ")
	runtime := object(result["runtime"])
	require(runtime["schema"] == "gooo/body-composition-runtime/v2" && runtime["stage"] == "COMPLETE", "runtime schema/stage differs")
	bound(c.Go, 64<<20)
	require(runtime["producer_source_sha"] == engineSHA && runtime["go_version"] == c.GoVersion && runtime["go_tool_sha256"] == "sha256:"+c.Go.SHA, "accepted producer identity differs")
	require(count(runtime["model_calls"]) == 0, "model calls differ")
	delivery := one(one(runtime["traces"])["deliveries"])
	require(delivery["activity_id"] == entryID && is(delivery["input"], want), "Assess input echo differs")
	actual := object(delivery["actual"])
	strings := []string{"trial_id", "source_revision", "state", "process_outcome", "observation_trial_id", "observation_source_revision"}
	ints := []string{"declared", "observed", "matched"}
	bools := []string{"argv_captured", "failure_observed", "expectation_observed"}
	require(len(actual) == len(strings)+len(ints)+len(bools), "program value fields differ")
	for _, k := range strings {
		text(actual[k])
	}
	for _, k := range ints {
		require(count(actual[k]) <= 256, "program count differs")
	}
	for _, k := range bools {
		_, ok := actual[k].(bool)
		require(ok, "program boolean differs")
	}
	states := map[string]bool{"INVALID_PLAN": true, "INCOMPLETE_OBSERVATION": true, "INVALID_OBSERVATION": true, "NO_TESTS_OBSERVED": true, "TEST_SET_MISMATCH": true, "PROCESS_MISMATCH": true, "MATCHED_EXPECTATION": true}
	require(states[text(actual["state"])], "program state differs")
	require(actual["process_outcome"] == "MATCH" || actual["process_outcome"] == "MISMATCH" || actual["process_outcome"] == "UNASSESSED", "process outcome differs")
	plan, observed := object(want["plan"]), object(want["observation"])
	require(actual["trial_id"] == plan["trial_id"] && actual["source_revision"] == plan["source_revision"] && actual["observation_trial_id"] == observed["trial_id"] && actual["observation_source_revision"] == observed["source_revision"] && actual["argv_captured"] == observed["argv_captured"], "program source/input pointers differ")
	passed, total := count(runtime["finite_passed"]), count(runtime["finite_total"])
	require(total == 1 && passed <= total, "finite denominator differs")
	decision := "PROGRESS"
	if passed == total {
		decision = "PASS"
	}
	require(r["decision"] == decision, "finite decision differs")
	if c.Operation == "execute" {
		require(c.Parent == nil && r["replayed_from_sha256"] == nil, "execute parent differs")
	} else {
		require(c.Operation == "replay" && c.Parent != nil, "operation/parent differs")
		bound(*c.Parent, 8<<20)
		require(r["replayed_from_sha256"] == "sha256:"+c.Parent.SHA, "saved first parent differs")
	}
	request := object(parse(bound(c.Request, 8<<20)))
	require(request["schema"] == "gooo/workspace-session-request/v1" && request["id"] == f["id"] && request["operation"] == c.Operation, "caller request identity differs")
	args := []any{"--go", c.Go.Path, "--cases", c.Input.Path, c.Manifest.Path}
	if c.Operation == "replay" {
		args = append([]any{"--receipt", c.Parent.Path}, args...)
	}
	require(is(request["arguments"], args), "caller request current-file arguments differ")
	capturePath := filepath.Join(c.ProfileRoot, "captures", "manifest.json")
	captureRaw, e := os.ReadFile(capturePath)
	if e != nil {
		panic(e)
	}
	require(hash(captureRaw) == capturesSHA, "capture manifest differs")
	captures := object(object(parse(captureRaw))["captures"])
	require(c.Capture == "red" || c.Capture == "green", "capture label differs")
	capture := object(captures[c.Capture])
	binding := object(capture["source_binding"])
	sourceID := "capture-source:sha256:" + text(binding["sha256"])
	require(plan["source_revision"] == sourceID && observed["source_revision"] == sourceID, "capture source ID differs")
	require(len(c.CaptureFiles) == 3, "capture component roster differs")
	for _, key := range []string{"stdout", "process", "source_binding"} {
		spec := object(capture[key])
		expected := fileRef{filepath.Join(c.ProfileRoot, "captures", text(spec["path"])), count(spec["bytes"]), text(spec["sha256"])}
		require(c.CaptureFiles[key] == expected, "capture component differs")
		bound(expected, 8<<20)
	}
	return map[string]any{"presentation": "Captured-test Assess display; finite local file-binding checks.", "id": c.ID, "program_value": actual, "assessment_execution": map[string]any{"receipt_decision": decision, "matched_expected_result_cases": passed, "result_cases": total, "command_exit_code": 0}, "domain_scope": map[string]any{"selected_named_nodes": 4, "selected_leaf_nodes": 3, "whole58_Go_leaf_tests_assessed": false}, "evidence": map[string]any{"caller_saved_full_frame": frameRef, "program_value_pointer": "/receipt/result/runtime/traces/0/deliveries/0/actual", "original_Gooo_source": c.Source, "original_Gooo_source_commit": engineSHA, "stable_activity_ids": []string{entryID, helperID}, "current_input": c.Input, "workspace_manifest": c.Manifest, "caller_request": c.Request, "saved_replay_parent": c.Parent, "original_host_capture": c.Capture, "original_capture_files": c.CaptureFiles, "original_capture_manifest": reference(capturePath), "accepted_engine_source": engineSHA, "Go_tool": c.Go}, "authority": "Diagnostic display only; no effect or CI authority. No general cryptographic receipt verification."}, nil
}
