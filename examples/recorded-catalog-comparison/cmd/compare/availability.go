// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"encoding/json"
	"io"
	"os"

	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func policyAndAvailabilityArguments(args []string, allowAvailability bool) (map[string]string, bool) {
	if len(args)%2 != 0 || len(args) > 6 {
		return nil, false
	}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if key != "--before-policy" && key != "--after-policy" && !(allowAvailability && key == "--availability") {
			return nil, false
		}
		if _, duplicate := values[key]; duplicate {
			return nil, false
		}
		if key == "--availability" && args[i+1] == "" {
			return nil, false
		}
		values[key] = args[i+1]
	}
	return values, true
}

func runAvailabilityTemplate(input io.Reader, out, diagnostics io.Writer, before, after string) int {
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	doc, err := origin.AvailabilityTemplateJSON(raw, before, after)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}

func runExplicitAvailability(input io.Reader, out, diagnostics io.Writer, path, before, after string) int {
	// This is a bounded local saved file, not another stream or a live collector.
	info, err := os.Stat(path)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	if !info.Mode().IsRegular() || info.Size() > origin.MaxInputBytes {
		return diagnostic(diagnostics, &origin.Problem{State: "INVALID_TRANSPORT", Stage: "read_availability", Code: "AVAILABILITY_FILE_REQUIRED"})
	}
	f, err := os.Open(path)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	availability, readErr := io.ReadAll(io.LimitReader(f, origin.MaxInputBytes+1))
	closeErr := f.Close()
	if readErr != nil {
		return diagnostic(diagnostics, readErr)
	}
	if closeErr != nil {
		return diagnostic(diagnostics, closeErr)
	}
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	report, err := origin.CompareExplicitObservationJSON(raw, availability, before, after)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}
