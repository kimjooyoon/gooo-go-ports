// A synthetic local consumer demonstration; no network or model calls.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	"os"
)

func run() error {
	r, err := refresh.NewRegistry([]byte("previous stored catalog"))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	for _, scenario := range []struct{ name, tools, next string }{
		{"partial-retains-existing", `[{"name":"lookup","inputSchema":{"type":"object"}}]`, `,"nextCursor":"remaining"`},
		{"complete-replaces-existing", `[{"name":"lookup","inputSchema":{"type":"object"}}]`, ""},
		{"terminal-empty-replaces-with-empty", "[]", ""},
	} {
		a, err := r.Begin(context.Background())
		if err != nil {
			return err
		}
		receipt := r.Refresh(a, []refresh.SnapshotPage{{RequestJSON: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, ResponseJSON: `{"jsonrpc":"2.0","id":1,"result":{"tools":` + scenario.tools + scenario.next + `}}`}}, "SUCCESS")
		if err := encoder.Encode(struct {
			Scenario string          `json:"scenario"`
			Receipt  refresh.Receipt `json:"receipt"`
		}{scenario.name, receipt}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
