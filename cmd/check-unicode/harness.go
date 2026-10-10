package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

//go:embed exhaustive_test.go.txt
var harnessTemplate string

type exhaustiveReport struct {
	Schema                               string `json:"schema"`
	GoVersion                            string `json:"go_version"`
	UnicodeVersion                       string `json:"unicode_version"`
	UnicodeInputs                        int    `json:"unicode_range_inputs"`
	PortComparisons                      int    `json:"exhaustive_port_comparisons"`
	ConsumerComparisons                  int    `json:"exhaustive_consumer_field_comparisons"`
	OutsideInt32RangeInputs              int    `json:"outside_unicode_range_int32_samples"`
	OutsideInt32RangePortComparisons     int    `json:"outside_unicode_range_port_comparisons"`
	OutsideInt32RangeConsumerComparisons int    `json:"outside_unicode_range_consumer_field_comparisons"`
	ExtensionInputs                      int    `json:"outside_int32_extension_samples"`
	ExtensionPortComparisons             int    `json:"extension_port_observations"`
	ExtensionConsumerComparisons         int    `json:"extension_consumer_field_observations"`
	Mismatches                           int    `json:"mismatches"`
}

func makeHarness(names generatedNames) ([]byte, error) {
	t, err := template.New("native").Parse(harnessTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, names); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func validateExhaustive(r exhaustiveReport) error {
	want := exhaustiveReport{"gooo/unicode-exhaustive-native/v1", "go1.27.2", "17.0.0", 1114112, 2228224, 4456448, 8, 16, 32, 8, 16, 32, 0}
	if r != want {
		return fmt.Errorf("native exhaustive report counters or identity differ: %+v", r)
	}
	return nil
}
