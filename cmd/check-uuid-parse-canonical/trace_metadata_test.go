package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTraceMetadataChecksSourceDeclaredFiniteShapes(t *testing.T) {
	rows := []row{{"00112233-4455-6677-8899-aabbccddeeff"}}
	raw, id := fixture(t, rows)
	value, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	document := object(value)
	trace := object(array(object(object(document["result"])["runtime"])["traces"])[0])
	metadata, err := decode([]byte(`{
      "field_local_bridge":{"locals":[{"other_start":0,"other_end":1,"other_drops":0,"start_clock":0,"completion":1}],"fields":[],"local_clocks":[1],"field_clocks":null,"completions":1},
      "field_writes":[{"site_index":0,"frame":0,"use_start":0,"use_end":1,"uses_dropped":0,"completed":true,"parent_assignment":-1,"completion":1}],
      "field_uses":[{"site_index":0,"frame":0,"assignment":-1,"completion_clock":0,"reason":"UNINITIALIZED"}],
      "field_frames":1,"conditions_observed":true,"conditions":[{"site_index":0,"value":true}],
      "local_writes":[{"site_index":0,"frame":0,"read_start":0,"read_end":0,"use_start":0,"use_end":1,"reads_dropped":0,"uses_dropped":0,"completed":true}],
      "local_uses":[{"site_index":0,"frame":0,"assignment":0}],"local_frames":1,"reads_observed":true
    }`))
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range object(metadata) {
		trace[name] = v
	}
	positive := mustJSON(t, document)
	if _, err := inspect(positive, rows, id, ""); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"unknown top field":               func(v map[string]any) { v["untrusted"] = true },
		"unsupported read payload":        func(v map[string]any) { v["reads"] = []any{} },
		"count Boolean":                   func(v map[string]any) { v["field_frames"] = true },
		"negative count":                  func(v map[string]any) { v["local_frames"] = json.Number("-1") },
		"decimal count":                   func(v map[string]any) { v["local_frames"] = json.Number("1.0") },
		"counter overflow":                func(v map[string]any) { v["local_frames"] = json.Number("65537") },
		"Boolean number":                  func(v map[string]any) { v["reads_observed"] = json.Number("1") },
		"record array replaced by object": func(v map[string]any) { v["conditions"] = map[string]any{} },
		"array overflow":                  func(v map[string]any) { v["conditions"] = make([]any, traceMetadataLimit+1) },
		"condition Boolean number":        func(v map[string]any) { object(array(v["conditions"])[0])["value"] = json.Number("1") },
		"unknown local write field":       func(v map[string]any) { object(array(v["local_writes"])[0])["extra"] = false },
		"field write decimal index":       func(v map[string]any) { object(array(v["field_writes"])[0])["site_index"] = json.Number("0.0") },
		"missing field write completion":  func(v map[string]any) { delete(object(array(v["field_writes"])[0]), "completion") },
		"integer reason":                  func(v map[string]any) { object(array(v["local_uses"])[0])["reason"] = json.Number("0") },
		"oversized reason":                func(v map[string]any) { object(array(v["field_uses"])[0])["reason"] = strings.Repeat("x", 1025) },
		"unknown bridge field":            func(v map[string]any) { object(v["field_local_bridge"])["extra"] = nil },
		"bridge decimal clock":            func(v map[string]any) { object(v["field_local_bridge"])["local_clocks"] = []any{json.Number("1.0")} },
		"bridge missing window field": func(v map[string]any) {
			delete(object(array(object(v["field_local_bridge"])["locals"])[0]), "other_end")
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			v, err := decode(positive)
			if err != nil {
				t.Fatal(err)
			}
			doc := object(v)
			trace := object(array(object(object(doc["result"])["runtime"])["traces"])[0])
			mutate(trace)
			if _, err := inspect(mustJSON(t, doc), rows, id, ""); err == nil {
				t.Fatal("accepted unsupported trace diagnostic")
			}
		})
	}
}
