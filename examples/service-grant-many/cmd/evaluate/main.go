package main

import (
	"encoding/json"
	"flag"
	"fmt"
	policy "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
	"io"
	"os"
)

func run(in io.Reader, out, errout io.Writer, columns bool) int {
	raw, err := io.ReadAll(io.LimitReader(in, policy.MaxBatchBytes+1))
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	rows, err := policy.DecodeBatch(raw)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	facts := make([]policy.RequestColumns, len(rows))
	for i, row := range rows {
		if columns {
			facts[i], err = policy.DecodeColumns(row)
		} else {
			var input policy.Input
			input, err = policy.DecodeInput(row)
			facts[i] = input.Columns()
		}
		if err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
	}
	results := make([]policy.Result, len(facts))
	for i, input := range facts {
		results[i] = policy.AssessColumns(input)
	}
	if err = json.NewEncoder(out).Encode(results); err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	return 0
}
func main() {
	columns := flag.Bool("columns", false, "read the exact four-field ordered-column representation")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "no positional arguments accepted")
		os.Exit(2)
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, *columns))
}
