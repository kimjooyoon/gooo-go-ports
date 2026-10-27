// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
// A local MCP peer exercises the real SDK and generated Gooo decisions.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/sdk-catalog-refresh"
)

func serve(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	page := 0
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			return err
		}
		if len(req.ID) == 0 {
			continue
		}
		var result string
		switch req.Method {
		case "initialize":
			result = `{"protocolVersion":"2025-11-25","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"local-synthetic-peer","version":"1"}}`
		case "tools/list":
			results := []string{
				`{"tools":[{"name":"lookup","inputSchema":{"type":"object"}}]}`,
				`{"tools":[{"name":"filtered","inputSchema":{"type":"object","properties":{"items":{"type":"array","x-mcp-header":"Items"}}}}]}`,
				`{"tools":[]}`,
			}
			if page >= len(results) {
				return errors.New("unexpected extra tools/list")
			}
			result = results[page]
			page++
		default:
			return fmt.Errorf("unexpected method %s", req.Method)
		}
		if _, err := fmt.Fprintf(conn, "{\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":%s}\n", req.ID, result); err != nil {
			return err
		}
	}
}

func run(ctx context.Context, output io.Writer) (first error) {
	local, remote := net.Pipe()
	done := make(chan error, 1)
	go func() { defer remote.Close(); done <- serve(remote) }()
	defer func() {
		_ = local.Close()
		_ = remote.Close()
		select {
		case err := <-done:
			if first == nil && err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				first = err
			}
		case <-time.After(time.Second):
			first = errors.Join(first, errors.New("local peer did not stop"))
		}
	}()
	client, err := refresh.Connect(ctx, local, []byte("previous opaque catalog"))
	if err != nil {
		return err
	}
	defer client.Close()
	for _, name := range []string{"complete-list", "SDK-filtered-row", "complete-empty"} {
		report, err := client.Refresh(ctx)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(output).Encode(struct {
			Case   string         `json:"case"`
			Report refresh.Report `json:"report"`
		}{name, report}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: sdk-catalog-refresh-demo")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := run(ctx, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
