//go:build !js || !wasm

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "the service-policy browser bridge requires GOOS=js GOARCH=wasm")
	os.Exit(1)
}
