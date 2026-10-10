package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type Field struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}
type InputPort struct {
	Port   string          `json:"port"`
	Entity string          `json:"entity_id"`
	Value  json.RawMessage `json:"value"`
	Fields []Field         `json:"fields"`
}
type Delivery struct {
	Activity string          `json:"activity_id"`
	Inputs   []InputPort     `json:"inputs"`
	Actual   json.RawMessage `json:"actual"`
	Expected json.RawMessage `json:"expected"`
	Fields   []Field         `json:"actual_fields"`
	Passed   bool            `json:"passed"`
}
type Trace struct {
	Index      int        `json:"case_index"`
	Deliveries []Delivery `json:"deliveries"`
}
type Receipt struct {
	Schema         string `json:"schema"`
	Decision       string `json:"decision"`
	Error          string `json:"error"`
	ManifestDigest string `json:"manifest_digest"`
	CasesDigest    string `json:"cases_digest"`
	ReplayedFrom   string `json:"replayed_from_sha256"`
	Result         struct {
		Program struct {
			Entry struct {
				Package  string `json:"package_path"`
				Activity string `json:"activity"`
			} `json:"entry"`
			Image struct {
				Packages []struct {
					Path    string `json:"path"`
					Sources []struct {
						Filename string `json:"filename"`
						Digest   string `json:"source_digest"`
					} `json:"sources"`
					Exports []struct {
						Name   string   `json:"name"`
						Kind   string   `json:"kind"`
						ID     string   `json:"id"`
						Inputs []string `json:"input_types"`
						Output string   `json:"output_type"`
					} `json:"exports"`
				}
			} `json:"workspace_image"`
		} `json:"program"`
		Composition struct {
			Stage  string `json:"stage"`
			Source string `json:"source"`
			Digest string `json:"generated_sha256"`
			Steps  []struct {
				Generation struct {
					Report struct {
						Activity string `json:"activity_id"`
						Compiler string `json:"compiler_source_sha"`
					} `json:"report"`
				} `json:"generation"`
			} `json:"steps"`
		} `json:"composition"`
		Runtime struct {
			Schema             string          `json:"schema"`
			Stage              string          `json:"stage"`
			Failure            json.RawMessage `json:"failure"`
			Producer           string          `json:"producer_source_sha"`
			Go                 string          `json:"go_version"`
			Digest             string          `json:"generated_sha256"`
			Total              int             `json:"finite_total"`
			Passed             int             `json:"finite_passed"`
			ModelCalls         *int            `json:"model_calls"`
			ProjectionReplayed bool            `json:"projection_replayed"`
			RuntimeReplayed    bool            `json:"runtime_replayed"`
			Traces             []Trace         `json:"traces"`
		} `json:"runtime"`
		Replay *struct {
			ModelCalls *int `json:"model_calls"`
		} `json:"replay"`
	} `json:"result"`
}
type Case struct {
	Inputs   map[string]any     `json:"inputs"`
	Expected map[string]Outcome `json:"expected"`
}
type Suite struct {
	Schema string `json:"schema"`
	Cases  []Case `json:"cases"`
}

func digest(raw []byte) string { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }
func marshal(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
func exact(raw []byte, expected any) bool {
	var left, right any
	for _, item := range []struct {
		raw   []byte
		value *any
	}{{raw, &left}, {marshal(expected), &right}} {
		d := json.NewDecoder(bytes.NewReader(item.raw))
		d.UseNumber()
		if d.Decode(item.value) != nil {
			return false
		}
	}
	return reflect.DeepEqual(left, right)
}
func fields(kind string, names []string, value any) []Field {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(marshal(value), &m)
	out := make([]Field, len(names))
	for i, name := range names {
		out[i] = Field{entityPrefix + kind + "/" + strings.ReplaceAll(name, "_", "-"), name, m[name]}
	}
	return out
}
func originalPorts(e Envelope) []InputPort {
	return []InputPort{
		{"input0", entityPrefix + "request", marshal(e.Request), fields("request", []string{"action", "tenant", "item_count"}, e.Request)},
		{"input1", entityPrefix + "context", marshal(e.Context), fields("context", []string{"authenticated", "tenant", "allowed_action", "item_limit", "now", "expires_at"}, e.Context)},
		{"input2", entityPrefix + "choice", marshal(e.Choice), fields("choice", []string{"status", "choice"}, e.Choice)},
	}
}
func inspectReceipt(raw, manifest, source, cases []byte, valid []Fixture, replayOf []byte) ([]Outcome, error) {
	if err := checkJSON(raw, 16<<20, 64); err != nil {
		return nil, err
	}
	var r Receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	reject := func(s string) ([]Outcome, error) { return nil, fmt.Errorf("receipt rejected: %s", s) }
	if r.Schema != "gooo/workspace-body-execution-receipt/v1" || r.Decision != "PASS" || r.Error != "" {
		return reject("execution status")
	}
	if r.ManifestDigest != digest(manifest) || r.CasesDigest != digest(cases) {
		return reject("original input/manifest binding")
	}
	if len(replayOf) > 0 && (r.ReplayedFrom != digest(replayOf) || r.Result.Replay == nil || r.Result.Replay.ModelCalls == nil || *r.Result.Replay.ModelCalls != 0) {
		return reject("saved replay origin")
	}
	if len(replayOf) == 0 && (r.ReplayedFrom != "" || r.Result.Replay != nil) {
		return reject("fresh execution contains replay metadata")
	}
	p := r.Result.Program
	if p.Entry.Package != "pilot/policy" || p.Entry.Activity != "Admit" || len(p.Image.Packages) != 1 {
		return reject("entry identity")
	}
	pkg := p.Image.Packages[0]
	if pkg.Path != "pilot/policy" || len(pkg.Sources) != 1 || pkg.Sources[0].Filename != "policy.gooo" || pkg.Sources[0].Digest != digest(marshal(string(source))) {
		return reject("source bytes")
	}
	found := 0
	for _, e := range pkg.Exports {
		if e.Name == "Admit" {
			found++
			if e.Kind != "activity" || e.ID != activityID || !reflect.DeepEqual(e.Inputs, []string{entityPrefix + "request", entityPrefix + "context", entityPrefix + "choice"}) || e.Output != entityPrefix+"outcome" {
				return reject("entry typed signature")
			}
		}
	}
	if found != 1 {
		return reject("entry export")
	}
	c, t := r.Result.Composition, r.Result.Runtime
	if c.Stage != "COMPLETE" || t.Stage != "COMPLETE" || t.Schema != "gooo/body-composition-runtime/v1" || len(t.Failure) > 0 && string(t.Failure) != "null" {
		return reject("runtime completion")
	}
	if t.Producer != compilerRevision || !strings.HasPrefix(t.Go, "go version go1.27.2 ") {
		return reject("producer identity")
	}
	if c.Source == "" || c.Digest != digest([]byte(c.Source)) || t.Digest != c.Digest {
		return reject("generated source binding")
	}
	if len(c.Steps) != 1 || c.Steps[0].Generation.Report.Activity != activityID || c.Steps[0].Generation.Report.Compiler != compilerRevision {
		return reject("generation binding")
	}
	if t.Total != len(valid) || t.Passed != len(valid) || t.ModelCalls == nil || *t.ModelCalls != 0 || !t.ProjectionReplayed || !t.RuntimeReplayed || len(t.Traces) != len(valid) {
		return reject("finite observations")
	}
	out := make([]Outcome, len(valid))
	for i, f := range valid {
		e, err := decodeEnvelope(f.raw())
		if err != nil {
			return nil, err
		}
		trace := t.Traces[i]
		if trace.Index != i || len(trace.Deliveries) != 1 {
			return reject("trace order")
		}
		d := trace.Deliveries[0]
		if d.Activity != activityID || !exact(marshal(d.Inputs), originalPorts(e)) {
			return reject("original typed input values/fields/order")
		}
		oracle := reference(e)
		if oracle != f.Expected.Outcome {
			return reject("Go reference disagrees with manual expectation")
		}
		if !d.Passed || !exact(d.Expected, oracle) || !exact(d.Actual, oracle) {
			return reject("actual vs reference/manual expectation")
		}
		if !exact(marshal(d.Fields), fields("outcome", []string{"allowed", "disposition", "reason"}, oracle)) {
			return reject("actual field identities/types")
		}
		if err := json.Unmarshal(d.Actual, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
