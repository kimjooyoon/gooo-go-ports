// Copyright 2026 Gooo Go Ports contributors.
// Use of this source code is governed by the BSD-style license in LICENSE.
// Independent original-Go reference only; it never executes Gooo or HTTP.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kimjooyoon/gooo-go-ports/internal/urlcomponentinput"
)

type facts struct {
	Original              string `json:"original"`
	PathSegment           string `json:"path_segment"`
	QueryValue            string `json:"query_value"`
	InputBytes            int64  `json:"input_bytes"`
	PathBytes             int64  `json:"path_bytes"`
	QueryBytes            int64  `json:"query_bytes"`
	PathChanged           bool   `json:"path_changed"`
	QueryChanged          bool   `json:"query_changed"`
	ContextsDiffer        bool   `json:"contexts_differ"`
	OriginalPercentOffset int64  `json:"original_percent_offset"`
}

func reference(text string) facts {
	path, query := url.PathEscape(text), url.QueryEscape(text)
	return facts{text, path, query, int64(len(text)), int64(len(path)), int64(len(query)), path != text, query != text, path != query, int64(strings.IndexByte(text, '%'))}
}

func originalGoIdentity() error {
	if runtime.Version() != "go1.27.2" {
		return fmt.Errorf("URL component oracle requires go1.27.2, got %s", runtime.Version())
	}
	for _, source := range []struct{ path, sha string }{
		{"url.go", "44c33f88526abf3b5955d6c97297be5c9d228e741b33e52c940dcac5a496e091"},
		{"encoding_table.go", "274e5ad203d6c38c41fe435eb9e5fc65ae1a38e98b44daa9b6355ba0d3fdac09"},
		{"gen_encoding_table.go", "cd0f5020e4a5aa9741411afb4ab1bf73b9e9f40cd950e2bb9b369a773fdd9033"},
	} {
		raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src", "net", "url", source.path))
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != source.sha {
			return fmt.Errorf("pinned original Go net/url source changed: %s", source.path)
		}
	}
	return nil
}

func run(args []string, writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("URL component oracle output writer is required")
	}
	flags := flag.NewFlagSet("url-component-oracle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("inputs", "", "strict public input-only corpus file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: url-component-oracle --inputs inputs.json")
	}
	if err := originalGoIdentity(); err != nil {
		return err
	}
	file, err := os.Open(*input)
	if err != nil {
		return err
	}
	texts, readErr := urlcomponentinput.ReadCorpus(file)
	closeErr := file.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	results := make([]facts, len(texts))
	for index, text := range texts {
		results[index] = reference(text)
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	n, err := writer.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
