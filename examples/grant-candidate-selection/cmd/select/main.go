package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	selection "github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection"
)

func run(input io.Reader, output, diagnostics io.Writer) int {
	fail := func(err error) int {
		var problem *selection.Problem
		if !errors.As(err, &problem) {
			problem = &selection.Problem{State: "UNKNOWN", Stage: "command", Code: "IO_FAILURE", Index: -1}
		}
		_ = json.NewEncoder(diagnostics).Encode(problem)
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(input, selection.MaxInputBytes+1))
	if err != nil {
		return fail(err)
	}
	document, err := selection.AssessJSON(raw)
	if err != nil {
		return fail(err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return fail(err)
	}
	if len(encoded) > selection.MaxReportBytes {
		return fail(&selection.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED", Index: -1})
	}
	if _, err := output.Write(append(encoded, '\n')); err != nil {
		return fail(err)
	}
	return 0
}

func main() { os.Exit(run(os.Stdin, os.Stdout, os.Stderr)) }
