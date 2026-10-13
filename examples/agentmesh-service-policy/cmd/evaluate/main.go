// evaluate transports strict synthetic service facts to generated Gooo.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	servicepolicy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
	"io"
	"os"
)

func main() {
	mode := flag.String("mode", "matcher", "matcher or impact; JSON array on stdin")
	flag.Parse()
	if err := run(*mode, flag.NArg(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode string, extra int, input io.Reader, output io.Writer) error {
	if extra != 0 || (mode != "matcher" && mode != "impact") {
		return fmt.Errorf("invalid evaluator mode or arguments")
	}
	raw, err := io.ReadAll(io.LimitReader(input, servicepolicy.MaxBatchBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > servicepolicy.MaxBatchBytes {
		return fmt.Errorf("batch exceeds 1 MiB")
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 || len(rows) > servicepolicy.MaxBatchCount {
		return fmt.Errorf("expected1..1024 exact fact objects")
	}
	if mode == "matcher" {
		facts := make([]servicepolicy.MatcherFacts, len(rows))
		for i, row := range rows {
			facts[i], err = servicepolicy.DecodeMatcher(row)
			if err != nil {
				return fmt.Errorf("case%d: %w", i, err)
			}
		}
		proofs := make([]servicepolicy.MatcherProof, len(rows))
		for i, f := range facts {
			proofs[i], err = servicepolicy.EvaluateMatcher(f)
			if err != nil {
				return err
			}
		}
		return json.NewEncoder(output).Encode(proofs)
	}
	facts := make([]servicepolicy.ImpactFacts, len(rows))
	for i, row := range rows {
		facts[i], err = servicepolicy.DecodeImpact(row)
		if err != nil {
			return fmt.Errorf("case%d: %w", i, err)
		}
	}
	proofs := make([]servicepolicy.ImpactProof, len(rows))
	for i, f := range facts {
		proofs[i], err = servicepolicy.EvaluateImpact(f)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(output).Encode(proofs)
}
