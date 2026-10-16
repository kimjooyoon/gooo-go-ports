// This parses host-owned facts; it does not
// authenticate client assertions, compute policy, launch Gooo or apply a change.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-go-ports/internal/headeradditioninput"
)

func run(reader io.Reader, writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("addition input output writer required")
	}
	raw, err := headeradditioninput.Prepare(reader)
	if err != nil {
		return err
	}
	n, err := writer.Write(raw)
	if err != nil {
		return fmt.Errorf("write addition inputs: %w", err)
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: header-addition-input < host-owned-request.json > inputs.json")
		os.Exit(2)
	}
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
