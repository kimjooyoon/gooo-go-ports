// evaluate runs a synthetic batch; it does not authenticate or call an API.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	nativepolicy "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func main() {
	mode := flag.String("mode", "scalar", "scalar or original-api; JSON array on stdin")
	flag.Parse()
	if err := run(*mode, flag.NArg(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode string, extra int, input io.Reader, output io.Writer) error {
	if extra != 0 || (mode != "scalar" && mode != "original-api") {
		return fmt.Errorf("invalid evaluator mode or arguments")
	}
	if _, err := nativepolicy.SourceBindings(); err != nil {
		return err
	}
	const maxBatch = 4 * 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(input, maxBatch+1))
	if err != nil {
		return err
	}
	if len(raw) > maxBatch {
		return fmt.Errorf("batch exceeds 4 MiB")
	}
	var batch []json.RawMessage
	if json.Unmarshal(raw, &batch) != nil || len(batch) == 0 || len(batch) > 1024 {
		return fmt.Errorf("expected 1..1024 fact objects")
	}
	if mode == "scalar" {
		facts := make([]nativepolicy.Facts, len(batch))
		for i, row := range batch {
			facts[i], err = nativepolicy.DecodeScalar(row)
			if err != nil {
				return fmt.Errorf("case %d: %w", i, err)
			}
		}
		proofs := make([]nativepolicy.Proof, len(batch))
		for i, row := range facts {
			proofs[i], err = nativepolicy.EvaluateScalar(row)
			if err != nil {
				return err
			}
		}
		return json.NewEncoder(output).Encode(proofs)
	}
	facts := make([]nativepolicy.APIFacts, len(batch))
	for i, row := range batch {
		facts[i], err = nativepolicy.DecodeAPI(row)
		if err != nil {
			return fmt.Errorf("case %d: %w", i, err)
		}
	}
	proofs := make([]nativepolicy.APIProof, len(batch))
	for i, row := range facts {
		proofs[i], err = nativepolicy.EvaluateAPI(row)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(output).Encode(proofs)
}
