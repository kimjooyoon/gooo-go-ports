package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func run() error {
	contract := flag.String("contract", "", "exact frozen contract file; SHA-256 must match compiled binding")
	input := flag.String("input", "", "one exact nine-named-fact JSON object")
	manual := flag.Bool("manual", false, "emit sixteen independent expected witnesses")
	full := flag.Bool("full", false, "stream all12288 expectations; not candidate execution")
	flag.Parse()
	modes := 0
	if *input != "" {
		modes++
	}
	if *manual {
		modes++
	}
	if *full {
		modes++
	}
	if modes != 1 || *contract == "" || flag.NArg() != 0 {
		return fmt.Errorf("require --contract and exactly one of --input/--manual/--full")
	}
	raw, err := os.ReadFile(*contract)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != ContractSHA256 {
		return fmt.Errorf("frozen contract hash differs")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if *input != "" {
		raw, err = os.ReadFile(*input)
		if err != nil {
			return err
		}
		in, err := ParseInput(raw)
		if err != nil {
			return err
		}
		report, err := Evaluate(in)
		if err != nil {
			return err
		}
		return encoder.Encode(report)
	}
	type row struct {
		Index    int    `json:"index"`
		Name     string `json:"name,omitempty"`
		Inputs   Input  `json:"inputs"`
		Expected Report `json:"expected"`
	}
	if *manual {
		rows := make([]row, 0, 16)
		for i, witness := range ManualWitnesses() {
			report, err := Evaluate(witness.Inputs)
			if err != nil {
				return err
			}
			rows = append(rows, row{i, witness.Name, witness.Inputs, report})
		}
		return encoder.Encode(struct {
			Schema   string `json:"schema"`
			Contract string `json:"contract_sha256"`
			Cases    []row  `json:"cases"`
		}{"gooo/redelivery-independent-manual-oracle/v1", ContractSHA256, rows})
	}
	if err := encoder.Encode(map[string]any{"schema": "gooo/redelivery-independent-full-oracle/v1", "kind": "header", "contract_sha256": ContractSHA256, "expected_cases": 12288, "scope": "ORACLE_EXPECTATIONS_ONLY_NOT_CANDIDATE_PASS"}); err != nil {
		return err
	}
	actions, reasons := map[string]int{}, map[string]int{}
	err = ForEachValid(func(i int, in Input) error {
		report, err := Evaluate(in)
		if err != nil {
			return err
		}
		actions[report.Action]++
		reasons[report.Reason]++
		return encoder.Encode(row{Index: i, Inputs: in, Expected: report})
	})
	if err != nil {
		return err
	}
	return encoder.Encode(map[string]any{"kind": "footer", "emitted_cases": 12288, "actions": actions, "reasons": reasons, "candidate_executed": false})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
