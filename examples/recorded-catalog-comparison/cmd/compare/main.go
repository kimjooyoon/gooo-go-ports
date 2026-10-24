// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"encoding/json"
	"errors"
	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
	"io"
	"os"
)

func run(input io.Reader, out, diagnostics io.Writer) int {
	fail := func(err error) int {
		var problem *origin.Problem
		if !errors.As(err, &problem) {
			problem = &origin.Problem{State: "UNKNOWN", Stage: "command", Code: "IO_FAILURE"}
		}
		_ = json.NewEncoder(diagnostics).Encode(problem)
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return fail(err)
	}
	report, err := origin.CompareJSON(raw)
	if err != nil {
		return fail(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return fail(err)
	}
	if len(encoded) > origin.MaxReportBytes {
		return fail(&origin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	if _, err := out.Write(append(encoded, '\n')); err != nil {
		return fail(err)
	}
	return 0
}
func main() { os.Exit(run(os.Stdin, os.Stdout, os.Stderr)) }
