package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
)

type receipt struct {
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
					Sources []struct {
						Filename string `json:"filename"`
						Digest   string `json:"source_digest"`
					} `json:"sources"`
				} `json:"packages"`
			} `json:"workspace_image"`
		} `json:"program"`
		Composition struct {
			Stage  string `json:"stage"`
			Source string `json:"source"`
			Digest string `json:"generated_sha256"`
		} `json:"composition"`
		Replay *struct {
			ModelCalls *int `json:"model_calls"`
		} `json:"replay"`
		Runtime struct {
			Stage      string          `json:"stage"`
			Failure    json.RawMessage `json:"failure"`
			Producer   string          `json:"producer_source_sha"`
			Go         string          `json:"go_version"`
			Digest     string          `json:"generated_sha256"`
			Calls      *int            `json:"model_calls"`
			Passed     *int            `json:"finite_passed"`
			Total      *int            `json:"finite_total"`
			Projection bool            `json:"projection_replayed"`
			Replayed   bool            `json:"runtime_replayed"`
			Traces     []struct {
				Index      int `json:"case_index"`
				Deliveries []struct {
					ID     string `json:"activity_id"`
					Inputs []struct {
						Port  string          `json:"port"`
						Value json.RawMessage `json:"value"`
					} `json:"inputs"`
					Actual   json.RawMessage `json:"actual"`
					Expected json.RawMessage `json:"expected"`
					Passed   bool            `json:"passed"`
				} `json:"deliveries"`
			} `json:"traces"`
		} `json:"runtime"`
	} `json:"result"`
}

func inspectReceipt(raw []byte, lock lockfile, sources map[string]string, replayedFrom string) (string, error) {
	var e receipt
	if err := json.Unmarshal(raw, &e); err != nil {
		return "", err
	}
	fail := func(reason string) (string, error) {
		return "", fmt.Errorf("invalid browser build evidence: %s", reason)
	}
	manifestRaw, err := os.ReadFile(manifest)
	if err != nil {
		return "", err
	}
	casesRaw, err := os.ReadFile(casesPath)
	if err != nil {
		return "", err
	}
	var fixture struct {
		Cases []struct {
			Inputs   map[string]json.RawMessage `json:"inputs"`
			Expected map[string]json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(casesRaw, &fixture); err != nil {
		return "", err
	}
	if len(fixture.Cases) == 0 || e.Schema != "gooo/workspace-body-execution-receipt/v1" || e.Decision != "PASS" || e.Error != "" {
		return fail("schema, decision or empty cases")
	}
	if e.ManifestDigest != "sha256:"+digest(manifestRaw) || e.CasesDigest != "sha256:"+digest(casesRaw) {
		return fail("manifest or case binding")
	}
	p := e.Result.Program
	c := e.Result.Composition
	r := e.Result.Runtime
	if p.Entry.Package != "app/browser-demo" || p.Entry.Activity != "Dispatch" {
		return fail("entry")
	}
	seen := map[string]string{}
	for _, pkg := range p.Image.Packages {
		for _, source := range pkg.Sources {
			if _, exists := seen[source.Filename]; exists {
				return fail("duplicate source")
			}
			if !strings.HasPrefix(source.Digest, "sha256:") {
				return fail("source digest format")
			}
			seen[source.Filename] = strings.TrimPrefix(source.Digest, "sha256:")
		}
	}
	if !reflect.DeepEqual(seen, sources) {
		return fail("source binding")
	}
	if c.Stage != "COMPLETE" || c.Source == "" || c.Digest != "sha256:"+digest([]byte(c.Source)) || r.Digest != c.Digest {
		return fail("generated source digest")
	}
	goVersion := strings.Fields(r.Go)
	if r.Stage != "COMPLETE" || (len(r.Failure) > 0 && string(r.Failure) != "null") || r.Producer != lock.CompilerRevision || len(goVersion) != 4 || goVersion[0] != "go" || goVersion[1] != "version" || goVersion[2] != lock.GoVersion {
		return fail("runtime identity")
	}
	if r.Calls == nil || *r.Calls != 0 || r.Total == nil || *r.Total != len(fixture.Cases) || r.Passed == nil || *r.Passed != len(fixture.Cases) || !r.Projection || !r.Replayed || len(r.Traces) != len(fixture.Cases) {
		return fail("runtime counters or replay")
	}
	if e.ReplayedFrom != replayedFrom {
		return fail("original receipt binding")
	}
	if replayedFrom != "" {
		if e.Result.Replay == nil || e.Result.Replay.ModelCalls == nil || *e.Result.Replay.ModelCalls != 0 {
			return fail("saved replay model calls")
		}
	} else if e.Result.Replay != nil {
		return fail("unexpected saved replay")
	}
	for i, t := range r.Traces {
		if t.Index != i || len(t.Deliveries) != 1 {
			return fail("trace roster")
		}
		d := t.Deliveries[0]
		row := fixture.Cases[i]
		if d.ID != "gooo://go-ports/examples/browser-demo/dispatch" || len(d.Inputs) != 3 || !d.Passed {
			return fail("delivery identity")
		}
		for j, in := range d.Inputs {
			port := fmt.Sprintf("input%d", j)
			if in.Port != port || !sameJSON(in.Value, row.Inputs["app/browser-demo:Dispatch."+port]) {
				return fail("input value or order")
			}
		}
		want := row.Expected["app/browser-demo:Dispatch"]
		if !sameJSON(d.Actual, want) || !sameJSON(d.Expected, want) {
			return fail("output differs from supplied case")
		}
	}
	return c.Source, nil
}

func sameJSON(a, b []byte) bool {
	decode := func(raw []byte) (any, error) {
		var x any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&x); err != nil {
			return nil, err
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("trailing JSON data")
		}
		return x, nil
	}
	x, err := decode(a)
	if err != nil {
		return false
	}
	y, err := decode(b)
	return err == nil && reflect.DeepEqual(x, y)
}
