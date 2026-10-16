// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// This command validates a request; it does not compute or execute IfMatchResult.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-go-ports/internal/headerinput"
)

func run(reader io.Reader, writer io.Writer, invoke func([]byte) error) error {
	if writer == nil {
		return fmt.Errorf("header input output writer is required")
	}
	raw, err := headerinput.PrepareIfMatch(reader, invoke)
	if err != nil {
		return err
	}
	n, err := writer.Write(raw)
	if err != nil {
		return fmt.Errorf("write trusted header inputs: %w", err)
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: if-match-input < request.json > inputs.json")
		os.Exit(2)
	}
	if err := run(os.Stdin, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
