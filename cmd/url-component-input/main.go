// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// This command validates original Text; it computes no URL facts or API effect.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-go-ports/internal/urlcomponentinput"
)

func run(reader io.Reader, writer io.Writer, invoke func([]byte) error) error {
	if writer == nil {
		return fmt.Errorf("URL component input output writer is required")
	}
	raw, err := urlcomponentinput.Prepare(reader, invoke)
	if err != nil {
		return err
	}
	n, err := writer.Write(raw)
	if err != nil {
		return fmt.Errorf("write URL component inputs: %w", err)
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: url-component-input < request.json > inputs.json")
		os.Exit(2)
	}
	if err := run(os.Stdin, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
