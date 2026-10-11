// Generate source-owned Unicode simple folding from pinned Go tables.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unicode"
)

const targetSource = "ports/unicode/simple_fold.gooo"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("gen-simple-fold", flag.ContinueOnError)
	check := flags.Bool("check", false, "verify generated source without writing")
	root := flags.String("root", ".", "repository root containing upstream.json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	raw, err := os.ReadFile(filepath.Join(*root, "upstream.json"))
	if err != nil {
		return err
	}
	var pin upstream
	if err := json.Unmarshal(raw, &pin); err != nil {
		return err
	}
	if err := verifyIdentity(pin, runtime.Version(), unicode.Version, runtime.GOROOT()); err != nil {
		return err
	}
	raw, err = os.ReadFile(filepath.Join(runtime.GOROOT(), "src/unicode/tables.go"))
	if err != nil {
		return err
	}
	tables, err := parseTables(raw, unicode.CaseRanges)
	if err != nil {
		return err
	}
	source, err := generate(pin, tables)
	if err != nil {
		return err
	}
	path := filepath.Join(*root, targetSource)
	if *check {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, source) {
			return fmt.Errorf("%s differs; run go run ./cmd/gen-simple-fold", targetSource)
		}
		_, err = fmt.Fprintf(stdout, "current: %s (%d bytes)\n", targetSource, len(source))
		return err
	}
	if err := os.WriteFile(path, source, 0644); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "generated: %s (%d bytes; %d ASCII entries, %d orbit pairs, %d case ranges)\n", targetSource, len(source), len(tables.ascii), len(tables.orbit), len(tables.ranges))
	return err
}
