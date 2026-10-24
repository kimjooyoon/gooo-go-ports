// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import _ "embed"

//go:embed example-input.json
var exampleInput []byte

// ExampleInputJSON returns a fresh copy of the synthetic saved-catalog example.
// It is a fixture, not a new observation of a running MCP server.
func ExampleInputJSON() []byte { return append([]byte(nil), exampleInput...) }
