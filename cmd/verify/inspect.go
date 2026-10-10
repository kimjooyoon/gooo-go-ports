package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type expectedIdentity struct {
	CompilerRevision, GoVersion, ManifestDigest, CasesDigest string
	Sources                                                  map[string]sourceIdentity
}

type envelope struct {
	ManifestDigest string `json:"manifest_digest"`
	CasesDigest    string `json:"cases_digest"`
	Schema         string `json:"schema"`
	Decision       string `json:"decision"`
	Error          string `json:"error"`
	ReplayedFrom   string `json:"replayed_from_sha256"`
	Result         struct {
		Program struct {
			Image workspaceImage `json:"workspace_image"`
			Entry struct {
				Package  string `json:"package_path"`
				Activity string `json:"activity"`
			} `json:"entry"`
		} `json:"program"`
		Replay *struct {
			Calls *int `json:"model_calls"`
		} `json:"replay"`
		Composition struct {
			SHA    string `json:"generated_sha256"`
			Source string `json:"source"`
			Steps  []struct {
				Generation struct {
					Report struct {
						ID string `json:"activity_id"`
					} `json:"report"`
				} `json:"generation"`
			} `json:"steps"`
		} `json:"composition"`
		Runtime struct {
			Schema     string          `json:"schema"`
			Stage      string          `json:"stage"`
			Failure    json.RawMessage `json:"failure"`
			Producer   string          `json:"producer_source_sha"`
			GoVersion  string          `json:"go_version"`
			Generated  string          `json:"generated_sha256"`
			Calls      *int            `json:"model_calls"`
			Passed     *int            `json:"finite_passed"`
			Total      *int            `json:"finite_total"`
			Projection bool            `json:"projection_replayed"`
			Replayed   bool            `json:"runtime_replayed"`
			Traces     []struct {
				Index      int `json:"case_index"`
				Deliveries []struct {
					ID          string             `json:"activity_id"`
					Input       json.RawMessage    `json:"input,omitempty"`
					InputFields []recordFieldValue `json:"input_fields,omitempty"`
					Inputs      []struct {
						Port   string             `json:"port"`
						Entity string             `json:"entity_id"`
						Value  json.RawMessage    `json:"value"`
						Fields []recordFieldValue `json:"fields,omitempty"`
					} `json:"inputs,omitempty"`
					Actual       json.RawMessage    `json:"actual"`
					ActualFields []recordFieldValue `json:"actual_fields,omitempty"`
					Expected     json.RawMessage    `json:"expected"`
					Passed       *bool              `json:"passed"`
				} `json:"deliveries"`
			} `json:"traces"`
		} `json:"runtime"`
	} `json:"result"`
}
type checked struct{ Generated, ReplayedFrom string }

func inspectReplayBinding(original, replay checked, receiptDigest string) error {
	if !validDigest(receiptDigest) || replay.Generated != original.Generated || replay.ReplayedFrom != receiptDigest {
		return fmt.Errorf("saved replay is not bound to the same receipt and generated program")
	}
	return nil
}

func inspect(raw []byte, s suite, replay bool, identity expectedIdentity) (checked, error) {
	contract, err := contractFor(s)
	if err != nil {
		return checked{}, err
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return checked{}, err
	}
	fail := func(reason string) (checked, error) { return checked{}, fmt.Errorf("invalid evidence: %s", reason) }
	r := e.Result.Runtime
	c := e.Result.Composition
	version := strings.Fields(r.GoVersion)
	if e.ManifestDigest != identity.ManifestDigest || e.CasesDigest != identity.CasesDigest {
		return fail("workspace or case file binding")
	}
	if r.Schema != "gooo/body-composition-runtime/v1" || r.Stage != "COMPLETE" || (len(r.Failure) > 0 && string(r.Failure) != "null") || r.Producer != identity.CompilerRevision || len(version) != 4 || version[0] != "go" || version[1] != "version" || version[2] != identity.GoVersion || r.Generated != c.SHA {
		return fail("runtime producer, toolchain or generated program")
	}
	if e.Schema != "gooo/workspace-body-execution-receipt/v1" || e.Decision != "PASS" || e.Error != "" {
		return fail("schema, decision or compiler error")
	}
	if e.Result.Program.Entry.Package != s.Package || e.Result.Program.Entry.Activity != s.Activity {
		return fail("entry")
	}
	if err := inspectWorkspace(e.Result.Program.Image, s, contract, identity.Sources); err != nil {
		return fail(err.Error())
	}
	if r.Calls == nil || *r.Calls != 0 || r.Passed == nil || *r.Passed != len(s.Cases) || r.Total == nil || *r.Total != len(s.Cases) || !r.Projection || !r.Replayed {
		return fail("runtime counters or replay")
	}
	if !validDigest(c.SHA) || c.Source == "" || contentDigest([]byte(c.Source)) != c.SHA || len(c.Steps) != 1 || c.Steps[0].Generation.Report.ID != contract.ID || len(r.Traces) != len(s.Cases) {
		return fail("generated program or trace roster")
	}
	if replay {
		if e.Result.Replay == nil || e.Result.Replay.Calls == nil || *e.Result.Replay.Calls != 0 || !validDigest(e.ReplayedFrom) {
			return fail("saved replay metadata")
		}
	} else if e.Result.Replay != nil || e.ReplayedFrom != "" {
		return fail("unexpected saved replay")
	}
	for i, t := range r.Traces {
		if t.Index != i || len(t.Deliveries) != 1 {
			return fail("case index or delivery roster")
		}
		d := t.Deliveries[0]
		row := s.Cases[i]
		if d.ID != contract.ID || len(row.Inputs) != len(contract.InputEntities) {
			return fail("activity identity")
		}
		if len(row.Inputs) == 1 {
			if len(d.Inputs) != 0 || !equalValue(d.Input, row.Inputs[0]) {
				return fail("single input")
			}
			if err := inspectRectangleInputFields(contract.InputEntities[0], d.InputFields, row.Inputs[0]); err != nil {
				return fail(err.Error())
			}
		} else {
			if len(d.Input) != 0 || len(d.InputFields) != 0 || len(d.Inputs) != len(row.Inputs) {
				return fail("input arity")
			}
			for j, in := range d.Inputs {
				if in.Port != fmt.Sprintf("input%d", j) || in.Entity != contract.InputEntities[j] || !equalValue(in.Value, row.Inputs[j]) {
					return fail("input order or value")
				}
				if err := inspectRectangleInputFields(contract.InputEntities[j], in.Fields, row.Inputs[j]); err != nil {
					return fail(err.Error())
				}
			}
		}
		if !equalValue(d.Expected, row.Expected) || !equalValue(d.Actual, row.Expected) || d.Passed == nil || !*d.Passed {
			return fail(fmt.Sprintf("Go oracle mismatch at row %d: got %s; want %v", i, d.Actual, row.Expected))
		}
		if err := inspectWhitespaceFields(s, d.ActualFields, row.Expected); err != nil {
			return fail(err.Error())
		}
	}
	return checked{c.SHA, e.ReplayedFrom}, nil
}
func equalValue(raw json.RawMessage, want any) bool {
	encoded, err := json.Marshal(want)
	if err != nil {
		return false
	}
	left, err := decode(raw)
	if err != nil {
		return false
	}
	right, err := decode(encoded)
	return err == nil && reflect.DeepEqual(left, right)
}
func decode(raw []byte) (any, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return v, nil
}

func validDigest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}
