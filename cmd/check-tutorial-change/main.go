package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-go-ports/internal/tutorialchange"
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check-tutorial-change", flag.ContinueOnError)
	flags.SetOutput(stderr)
	before := flags.String("before", "", "original tutorial execution JSON")
	after := flags.String("after", "", "edited tutorial execution JSON")
	replay := flags.String("replay", "", "fresh replay of the edited execution JSON")
	build := flags.String("compiler-build", "", "separately captured compiler version --build --json")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *before == "" || *after == "" || *replay == "" || *build == "" {
		fmt.Fprintln(stderr, "require --before, --after, --replay and --compiler-build; no positional arguments")
		return 2
	}
	var records [4][]byte
	for i, name := range []string{*before, *after, *replay, *build} {
		limit := tutorialchange.MaxReceiptBytes
		if i == 3 {
			limit = tutorialchange.MaxCompilerBuildBytes
		}
		raw, err := readRecord(name, limit)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		records[i] = raw
	}
	summary, err := tutorialchange.Check(records[0], records[1], records[2], records[3])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err = stdout.Write(append(raw, '\n')); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func readRecord(name string, limit int) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(limit) {
		return nil, fmt.Errorf("%s: regular file within %d bytes required", name, limit)
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		return nil, fmt.Errorf("%s: file grew beyond size cap", name)
	}
	return raw, nil
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
