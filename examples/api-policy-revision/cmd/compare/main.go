package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "API revision comparison refused:", err)
		os.Exit(1)
	}
}

// run emits a single complete JSON record only after all input, generation and
// native decision checks succeed. It has no dispatcher or compiler invocation.
func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "compare", "baseline, compare, or replay")
	inputPath := flags.String("input", "", "read one JSON object from this path; otherwise stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional argument")
	}
	if *mode != "baseline" && *mode != "compare" && *mode != "replay" {
		return errors.New("mode must be baseline, compare, or replay")
	}
	reader := stdin
	if *inputPath != "" {
		file, err := os.Open(*inputPath)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	maxBytes := revision.MaxInputBytes
	if *mode == "replay" {
		maxBytes = revision.MaxReportBytes
	}
	raw, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
	if err != nil {
		return err
	}
	if len(raw) > maxBytes {
		return fmt.Errorf("input exceeds %d MiB", maxBytes>>20)
	}
	var result any
	switch *mode {
	case "baseline":
		facts, err := original.DecodeAPI(raw)
		if err != nil {
			return err
		}
		result, err = revision.Baseline(facts)
		if err != nil {
			return err
		}
	case "compare":
		request, err := revision.DecodeCompareRequest(raw)
		if err != nil {
			return err
		}
		result, err = revision.Compare(request)
		if err != nil {
			return err
		}
	case "replay":
		report, err := revision.DecodeComparisonReport(raw)
		if err != nil {
			return err
		}
		result, err = revision.VerifyComparison(report)
		if err != nil {
			return err
		}
	}
	output, err := json.Marshal(result)
	if err != nil {
		return err
	}
	output = append(output, '\n')
	written, err := stdout.Write(output)
	if err != nil {
		return err
	}
	if written != len(output) {
		return io.ErrShortWrite
	}
	return nil
}
